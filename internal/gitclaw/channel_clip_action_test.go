package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelClipCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-clip-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-clip-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-clip-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26301,
			"body": "@gitclaw /channels clip --clip-id clip-1 --message-id inbound-263 --notify-message-id notify-263\nTitle: Save channel launch detail\nNotes:\nVisible clip notes with CHANNEL_CLIP_NOTES_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 263,
			Title:  "GitClaw telegram thread chat-clip-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{263: {{
			ID: 26300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-clip-123",
				MessageID: "inbound-263",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_CLIP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{263: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel clip action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one clip issue: %#v", len(github.Issues), github.Issues)
	}
	clip := github.Issues[1]
	if !HasChannelClipMarker(clip.Body) || !strings.Contains(clip.Body, `clip_id="clip-1"`) {
		t.Fatalf("clip issue missing channel-clip marker:\n%s", clip.Body)
	}
	for _, want := range []string{
		"GitClaw channel clip",
		"clip_id: clip-1",
		"source_channel: telegram",
		"source_issue: #263",
		"source_message_id_sha256_12:",
		"clip_mode: github-issue-clip",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Save channel launch detail",
		"Visible clip notes with CHANNEL_CLIP_NOTES_SECRET.",
	} {
		if !strings.Contains(clip.Body, want) {
			t.Fatalf("clip issue missing %q:\n%s", want, clip.Body)
		}
	}
	if strings.Contains(clip.Body, "chat-clip-123") || strings.Contains(clip.Body, "inbound-263") || strings.Contains(clip.Body, "CHANNEL_CLIP_INGEST_SECRET") {
		t.Fatalf("clip issue leaked provider IDs or channel body:\n%s", clip.Body)
	}
	if !hasLabel(github.IssueLabels[clip.Number], "gitclaw") {
		t.Fatalf("clip issue missing gitclaw trigger label: %#v", github.IssueLabels[clip.Number])
	}

	sourceComments := github.CommentsByIssue[263]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-263"`,
		"GitClaw channel clip saved.",
		"Clip: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Save channel launch detail",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("clip notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_CLIP_NOTES_SECRET") || strings.Contains(outbound, "CHANNEL_CLIP_INGEST_SECRET") {
		t.Fatalf("clip notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Clip Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels clip`",
		"channel_clip_status: `saved`",
		"clip_issue: `#101`",
		"clip_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#263`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_clip_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_clip_title_included: `false`",
		"raw_clip_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_clip_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel clip receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CLIP_INGEST_SECRET", "CHANNEL_CLIP_NOTES_SECRET", "Save channel launch detail", "clip-1", "chat-clip-123", "inbound-263", "notify-263"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel clip receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-clip-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-clip-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26302,
			"body": "@gitclaw /channels clip --clip-id clip-1 --message-id inbound-263 --notify-message-id notify-263\nTitle: Save channel launch detail\nNotes:\nDo not leak duplicate token CHANNEL_CLIP_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate clip created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[263]); got != 4 {
		t.Fatalf("duplicate clip posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[263])
	}
	duplicateReceipt := github.CommentsByIssue[263][3].Body
	for _, want := range []string{
		"channel_clip_status: `duplicate`",
		"clip_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate clip receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_CLIP_DUPLICATE_SECRET") {
		t.Fatalf("duplicate clip receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelClipActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel clip"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel save --route team-demo --clip-id Design.Clip --message-id source-1 --notify-message-id notify-1
Summary: Route saved context
Context:
Keep this provider thread handy.`,
		},
	}
	req, err := BuildChannelClipActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelClipActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "save" || req.Options.Route != "team-demo" || req.Options.ClipID != "design-clip" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel clip parsing: %#v", req)
	}
	if req.Options.Title != "Route saved context" || !strings.Contains(req.Options.Notes, "Keep this provider thread handy.") {
		t.Fatalf("unexpected clip title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoClipID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route clip hashes: %#v", req)
	}
}
