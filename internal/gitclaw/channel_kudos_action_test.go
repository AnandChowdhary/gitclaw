package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelKudosCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-kudos-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-kudos-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-kudos-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels kudos --kudos-id kudos-1 --message-id inbound-484 --notify-message-id notify-484\nTo: Platform Team\nReason:\nVisible kudos reason with CHANNEL_KUDOS_REASON_SECRET.",
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
			Title:  "GitClaw telegram thread chat-kudos-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-kudos-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_KUDOS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel kudos action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one kudos issue: %#v", len(github.Issues), github.Issues)
	}
	kudos := github.Issues[1]
	if !HasChannelKudosMarker(kudos.Body) || !strings.Contains(kudos.Body, `kudos_id="kudos-1"`) {
		t.Fatalf("kudos issue missing channel-kudos marker:\n%s", kudos.Body)
	}
	for _, want := range []string{
		"GitClaw channel kudos",
		"kudos_id: kudos-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"kudos_mode: github-issue-kudos",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Platform Team",
		"Visible kudos reason with CHANNEL_KUDOS_REASON_SECRET.",
	} {
		if !strings.Contains(kudos.Body, want) {
			t.Fatalf("kudos issue missing %q:\n%s", want, kudos.Body)
		}
	}
	if strings.Contains(kudos.Body, "chat-kudos-123") || strings.Contains(kudos.Body, "inbound-484") || strings.Contains(kudos.Body, "CHANNEL_KUDOS_INGEST_SECRET") {
		t.Fatalf("kudos issue leaked provider IDs or channel body:\n%s", kudos.Body)
	}
	if !hasLabel(github.IssueLabels[kudos.Number], "gitclaw") {
		t.Fatalf("kudos issue missing gitclaw trigger label: %#v", github.IssueLabels[kudos.Number])
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
		"GitClaw channel kudos captured.",
		"Kudos: #101",
		"https://github.com/owner/repo/issues/101",
		"Recipient: Platform Team",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("kudos notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_KUDOS_REASON_SECRET") || strings.Contains(outbound, "CHANNEL_KUDOS_INGEST_SECRET") {
		t.Fatalf("kudos notification leaked reason or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Kudos Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels kudos`",
		"channel_kudos_status: `captured`",
		"kudos_issue: `#101`",
		"kudos_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_kudos_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_kudos_recipient_included: `false`",
		"raw_kudos_reason_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_kudos_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel kudos receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_KUDOS_INGEST_SECRET", "CHANNEL_KUDOS_REASON_SECRET", "Platform Team", "kudos-1", "chat-kudos-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel kudos receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-kudos-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-kudos-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels kudos --kudos-id kudos-1 --message-id inbound-484 --notify-message-id notify-484\nTo: Platform Team\nReason:\nDo not leak duplicate token CHANNEL_KUDOS_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate kudos created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate kudos posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_kudos_status: `duplicate`",
		"kudos_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate kudos receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_KUDOS_DUPLICATE_SECRET") {
		t.Fatalf("duplicate kudos receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelKudosActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel kudos"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel shoutout --route team-demo --kudos-id Roadmap.Spark --message-id source-1 --notify-message-id notify-1
To: Platform Team
Context:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable shaping surface.`,
		},
	}
	req, err := BuildChannelKudosActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelKudosActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "shoutout" || req.Options.Route != "team-demo" || req.Options.KudosID != "roadmap-spark" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel kudos parsing: %#v", req)
	}
	if req.Options.Recipient != "Platform Team" || !strings.Contains(req.Options.Reason, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected recipient/reason: %#v", req)
	}
	if req.TargetFromIssue || req.AutoKudosID || req.AutoNotifyMessageID || req.RecipientSHA == "" || req.ReasonSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route kudos hashes: %#v", req)
	}
}
