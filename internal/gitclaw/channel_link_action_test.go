package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelLinkCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-link-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-link-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-link-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26301,
			"body": "@gitclaw /channels link --link-id link-1 --url https://links.example.invalid/launch-secret --message-id inbound-263 --notify-message-id notify-263\nLink: Save channel launch detail\nNotes:\nVisible link notes with CHANNEL_LINK_NOTES_SECRET.",
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
			Title:  "GitClaw telegram thread chat-link-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{263: {{
			ID: 26300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-link-123",
				MessageID: "inbound-263",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_LINK_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{263: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel link action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one link issue: %#v", len(github.Issues), github.Issues)
	}
	link := github.Issues[1]
	if !HasChannelLinkMarker(link.Body) || !strings.Contains(link.Body, `link_id="link-1"`) {
		t.Fatalf("link issue missing channel-link marker:\n%s", link.Body)
	}
	for _, want := range []string{
		"GitClaw channel link card",
		"link_id: link-1",
		"source_channel: telegram",
		"source_issue: #263",
		"source_message_id_sha256_12:",
		"link_url_sha256_12:",
		"link_mode: github-issue-link-card",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"raw_link_url_included: false",
		"Save channel launch detail",
		"Visible link notes with CHANNEL_LINK_NOTES_SECRET.",
	} {
		if !strings.Contains(link.Body, want) {
			t.Fatalf("link issue missing %q:\n%s", want, link.Body)
		}
	}
	if strings.Contains(link.Body, "chat-link-123") || strings.Contains(link.Body, "inbound-263") || strings.Contains(link.Body, "https://links.example.invalid/launch-secret") || strings.Contains(link.Body, "CHANNEL_LINK_INGEST_SECRET") {
		t.Fatalf("link issue leaked provider IDs or channel body:\n%s", link.Body)
	}
	if !hasLabel(github.IssueLabels[link.Number], "gitclaw") {
		t.Fatalf("link issue missing gitclaw trigger label: %#v", github.IssueLabels[link.Number])
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
		"GitClaw channel link card saved.",
		"Link card: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Save channel launch detail",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("link notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_LINK_NOTES_SECRET") || strings.Contains(outbound, "CHANNEL_LINK_INGEST_SECRET") || strings.Contains(outbound, "https://links.example.invalid/launch-secret") {
		t.Fatalf("link notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Link Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels link`",
		"channel_link_status: `saved`",
		"link_issue: `#101`",
		"link_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#263`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_link_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_link_title_included: `false`",
		"raw_link_url_included: `false`",
		"raw_link_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_link_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel link receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_LINK_INGEST_SECRET", "CHANNEL_LINK_NOTES_SECRET", "Save channel launch detail", "link-1", "chat-link-123", "inbound-263", "notify-263", "https://links.example.invalid/launch-secret"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel link receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-link-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-link-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26302,
			"body": "@gitclaw /channels link --link-id link-1 --url https://links.example.invalid/launch-secret --message-id inbound-263 --notify-message-id notify-263\nLink: Save channel launch detail\nNotes:\nDo not leak duplicate token CHANNEL_LINK_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate link created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[263]); got != 4 {
		t.Fatalf("duplicate link posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[263])
	}
	duplicateReceipt := github.CommentsByIssue[263][3].Body
	for _, want := range []string{
		"channel_link_status: `duplicate`",
		"link_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate link receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_LINK_DUPLICATE_SECRET") {
		t.Fatalf("duplicate link receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelLinkActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel link"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel url --route team-demo --link-id Design.Link --url https://links.example.invalid/design-secret --message-id source-1 --notify-message-id notify-1
Link: Route saved context
Context:
Keep this provider thread handy.`,
		},
	}
	req, err := BuildChannelLinkActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelLinkActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "url" || req.Options.Route != "team-demo" || req.Options.LinkID != "design-link" || req.Options.LinkURL != "https://links.example.invalid/design-secret" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel link parsing: %#v", req)
	}
	if req.Options.Title != "Route saved context" || !strings.Contains(req.Options.Notes, "Keep this provider thread handy.") {
		t.Fatalf("unexpected link title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoLinkID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.LinkURLSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route link hashes: %#v", req)
	}
}
