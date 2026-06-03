package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelJamCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-jam-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-jam-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-jam-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels whiteboard --jam-id jam-1 --message-id inbound-484 --notify-message-id notify-484\nJam: Build a channel-native jam incubator\nNotes:\nVisible jam note with CHANNEL_JAM_NOTE_SECRET.",
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
			Number: 484,
			Title:  "GitClaw telegram thread chat-jam-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-jam-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_JAM_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel jam action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one jam issue: %#v", len(github.Issues), github.Issues)
	}
	jam := github.Issues[1]
	if !HasChannelJamMarker(jam.Body) || !strings.Contains(jam.Body, `jam_id="jam-1"`) {
		t.Fatalf("jam issue missing channel-jam marker:\n%s", jam.Body)
	}
	for _, want := range []string{
		"GitClaw channel jam",
		"jam_id: jam-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"jam_mode: github-issue-jam",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Build a channel-native jam incubator",
		"Visible jam note with CHANNEL_JAM_NOTE_SECRET.",
	} {
		if !strings.Contains(jam.Body, want) {
			t.Fatalf("jam issue missing %q:\n%s", want, jam.Body)
		}
	}
	if strings.Contains(jam.Body, "chat-jam-123") || strings.Contains(jam.Body, "inbound-484") || strings.Contains(jam.Body, "CHANNEL_JAM_INGEST_SECRET") {
		t.Fatalf("jam issue leaked provider IDs or channel body:\n%s", jam.Body)
	}
	if !hasLabel(github.IssueLabels[jam.Number], "gitclaw") {
		t.Fatalf("jam issue missing gitclaw trigger label: %#v", github.IssueLabels[jam.Number])
	}

	sourceComments := github.CommentsByIssue[484]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-484"`,
		"GitClaw channel jam captured.",
		"Jam: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Build a channel-native jam incubator",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("jam notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_JAM_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_JAM_INGEST_SECRET") {
		t.Fatalf("jam notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Jam Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels whiteboard`",
		"channel_jam_status: `captured`",
		"jam_issue: `#101`",
		"jam_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_jam_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_jam_title_included: `false`",
		"raw_jam_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_jam_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel jam receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_JAM_INGEST_SECRET", "CHANNEL_JAM_NOTE_SECRET", "Build a channel-native", "jam-1", "chat-jam-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel jam receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-jam-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-jam-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels whiteboard --jam-id jam-1 --message-id inbound-484 --notify-message-id notify-484\nJam: Build a channel-native jam incubator\nNotes:\nDo not leak duplicate token CHANNEL_JAM_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate jam created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate jam posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_jam_status: `duplicate`",
		"jam_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate jam receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_JAM_DUPLICATE_SECRET") {
		t.Fatalf("duplicate jam receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelJamActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel jam"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel whiteboard --route team-demo --jam-id Roadmap.Jam --message-id source-1 --notify-message-id notify-1
Topic: Make channel messages spawn GitHub-native jam labs
Seeds:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable shaping surface.`,
		},
	}
	req, err := BuildChannelJamActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelJamActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "whiteboard" || req.Options.Route != "team-demo" || req.Options.JamID != "roadmap-jam" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel jam parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native jam labs" || !strings.Contains(req.Options.Notes, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoJamID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route jam hashes: %#v", req)
	}
}
