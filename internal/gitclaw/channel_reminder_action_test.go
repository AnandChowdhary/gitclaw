package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelReminderCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-reminder-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 262,
			"title": "GitClaw telegram thread chat-reminder-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-reminder-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26201,
			"body": "@gitclaw /channels reminder --reminder-id reminder-1 --message-id inbound-262 --notify-message-id notify-262 --at 2099-01-02T03:04:05Z\nTitle: Follow up on channel incident\nNotes:\nVisible reminder notes with CHANNEL_REMINDER_NOTES_SECRET.",
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
			Number: 262,
			Title:  "GitClaw telegram thread chat-reminder-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{262: {{
			ID: 26200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-reminder-123",
				MessageID: "inbound-262",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_REMINDER_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{262: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel reminder action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one reminder issue: %#v", len(github.Issues), github.Issues)
	}
	reminder := github.Issues[1]
	if !HasChannelReminderMarker(reminder.Body) || !strings.Contains(reminder.Body, `reminder_id="reminder-1"`) {
		t.Fatalf("reminder issue missing channel-reminder marker:\n%s", reminder.Body)
	}
	for _, want := range []string{
		"GitClaw channel reminder",
		"reminder_id: reminder-1",
		"not_before: 2099-01-02T03:04:05Z",
		"source_channel: telegram",
		"source_issue: #262",
		"source_message_id_sha256_12:",
		"reminder_mode: github-issue-reminder",
		"wake_strategy: scheduled-github-actions-proactive-check",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Follow up on channel incident",
		"Visible reminder notes with CHANNEL_REMINDER_NOTES_SECRET.",
	} {
		if !strings.Contains(reminder.Body, want) {
			t.Fatalf("reminder issue missing %q:\n%s", want, reminder.Body)
		}
	}
	if strings.Contains(reminder.Body, "chat-reminder-123") || strings.Contains(reminder.Body, "inbound-262") || strings.Contains(reminder.Body, "CHANNEL_REMINDER_INGEST_SECRET") {
		t.Fatalf("reminder issue leaked provider IDs or channel body:\n%s", reminder.Body)
	}
	if !hasLabel(github.IssueLabels[reminder.Number], "gitclaw") {
		t.Fatalf("reminder issue missing gitclaw trigger label: %#v", github.IssueLabels[reminder.Number])
	}

	sourceComments := github.CommentsByIssue[262]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-262"`,
		"GitClaw channel reminder created.",
		"Reminder: #101",
		"https://github.com/owner/repo/issues/101",
		"Due: 2099-01-02T03:04:05Z",
		"Title: Follow up on channel incident",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("reminder notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_REMINDER_NOTES_SECRET") || strings.Contains(outbound, "CHANNEL_REMINDER_INGEST_SECRET") {
		t.Fatalf("reminder notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Reminder Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels reminder`",
		"channel_reminder_status: `created`",
		"reminder_issue: `#101`",
		"reminder_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#262`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_reminder_id_included: `false`",
		"raw_reminder_due_at_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_reminder_title_included: `false`",
		"raw_reminder_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_reminder_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel reminder receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_REMINDER_INGEST_SECRET", "CHANNEL_REMINDER_NOTES_SECRET", "Follow up on channel incident", "reminder-1", "2099-01-02T03:04:05Z", "chat-reminder-123", "inbound-262", "notify-262"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel reminder receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 262,
			"title": "GitClaw telegram thread chat-reminder-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-reminder-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26202,
			"body": "@gitclaw /channels reminder --reminder-id reminder-1 --message-id inbound-262 --notify-message-id notify-262 --at 2099-01-02T03:04:05Z\nTitle: Follow up on channel incident\nNotes:\nDo not leak duplicate token CHANNEL_REMINDER_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate reminder created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[262]); got != 4 {
		t.Fatalf("duplicate reminder posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[262])
	}
	duplicateReceipt := github.CommentsByIssue[262][3].Body
	for _, want := range []string{
		"channel_reminder_status: `duplicate`",
		"reminder_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate reminder receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_REMINDER_DUPLICATE_SECRET") {
		t.Fatalf("duplicate reminder receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelReminderActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 29, Title: "Channel reminder"},
		Comment: &Comment{
			ID: 2901,
			Body: `@gitclaw /channel snooze --route team-demo --reminder-id Design.Reminder --message-id source-1 --notify-message-id notify-1 --at 2026-06-02
Title: Route follow-up reminder
Notes:
Check the provider queue.`,
		},
	}
	req, err := BuildChannelReminderActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelReminderActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "snooze" || req.Options.Route != "team-demo" || req.Options.ReminderID != "design-reminder" || req.Options.DueAt != "2026-06-02T00:00:00Z" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel reminder parsing: %#v", req)
	}
	if req.Options.Title != "Route follow-up reminder" || !strings.Contains(req.Options.Notes, "Check the provider queue.") {
		t.Fatalf("unexpected reminder title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoReminderID || req.AutoNotifyMessageID || req.DueAtSHA == "" || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route reminder hashes: %#v", req)
	}
}
