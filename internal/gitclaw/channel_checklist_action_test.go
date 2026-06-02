package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelChecklistCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-checklist-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 684,
			"title": "GitClaw telegram thread chat-checklist-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-checklist-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 68401,
			"body": "@gitclaw /channels checklist --checklist-id checklist-1 --message-id inbound-684 --notify-message-id notify-684\nTitle: Launch channel checklist\nItems:\n- Confirm deployment with CHANNEL_CHECKLIST_ITEM_SECRET\n- Notify the release room\nNotes:\nVisible checklist note with CHANNEL_CHECKLIST_NOTE_SECRET.",
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
			Number: 684,
			Title:  "GitClaw telegram thread chat-checklist-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{684: {{
			ID: 68400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-checklist-123",
				MessageID: "inbound-684",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_CHECKLIST_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{684: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel checklist action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one checklist issue: %#v", len(github.Issues), github.Issues)
	}
	checklist := github.Issues[1]
	if !HasChannelChecklistMarker(checklist.Body) || !strings.Contains(checklist.Body, `checklist_id="checklist-1"`) {
		t.Fatalf("checklist issue missing channel-checklist marker:\n%s", checklist.Body)
	}
	for _, want := range []string{
		"GitClaw channel checklist",
		"checklist_id: checklist-1",
		"item_count: 2",
		"source_channel: telegram",
		"source_issue: #684",
		"source_message_id_sha256_12:",
		"checklist_mode: github-issue-checklist",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Checklist",
		"- [ ] Confirm deployment with CHANNEL_CHECKLIST_ITEM_SECRET",
		"- [ ] Notify the release room",
		"## Title",
		"Launch channel checklist",
		"## Notes",
		"Visible checklist note with CHANNEL_CHECKLIST_NOTE_SECRET.",
	} {
		if !strings.Contains(checklist.Body, want) {
			t.Fatalf("checklist issue missing %q:\n%s", want, checklist.Body)
		}
	}
	if strings.Contains(checklist.Body, "chat-checklist-123") || strings.Contains(checklist.Body, "inbound-684") || strings.Contains(checklist.Body, "CHANNEL_CHECKLIST_INGEST_SECRET") {
		t.Fatalf("checklist issue leaked provider IDs or channel body:\n%s", checklist.Body)
	}
	if !hasLabel(github.IssueLabels[checklist.Number], "gitclaw") {
		t.Fatalf("checklist issue missing gitclaw trigger label: %#v", github.IssueLabels[checklist.Number])
	}

	sourceComments := github.CommentsByIssue[684]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-684"`,
		"GitClaw channel checklist captured.",
		"Checklist: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch channel checklist",
		"Items: 2",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("checklist notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_CHECKLIST_ITEM_SECRET") || strings.Contains(outbound, "CHANNEL_CHECKLIST_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_CHECKLIST_INGEST_SECRET") {
		t.Fatalf("checklist notification leaked items, notes, or channel body:\n%s", outbound)
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Checklist Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels checklist`",
		"channel_checklist_status: `captured`",
		"checklist_issue: `#101`",
		"checklist_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#684`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"checklist_item_count: `2`",
		"checklist_mode: `github-issue-checklist`",
		"repository_mutation_performed: `false`",
		"raw_checklist_id_included: `false`",
		"raw_checklist_title_included: `false`",
		"raw_checklist_items_included: `false`",
		"raw_checklist_notes_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_checklist_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel checklist receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHECKLIST_INGEST_SECRET", "CHANNEL_CHECKLIST_ITEM_SECRET", "CHANNEL_CHECKLIST_NOTE_SECRET", "Launch channel checklist", "checklist-1", "chat-checklist-123", "inbound-684", "notify-684", "Notify the release room"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel checklist receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 684,
			"title": "GitClaw telegram thread chat-checklist-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-checklist-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 68402,
			"body": "@gitclaw /channels checklist --checklist-id checklist-1 --message-id inbound-684 --notify-message-id notify-684\nTitle: Launch channel checklist\nItems:\n- Do not leak duplicate token CHANNEL_CHECKLIST_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate checklist created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[684]); got != 4 {
		t.Fatalf("duplicate checklist posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[684])
	}
	duplicateReceipt := github.CommentsByIssue[684][3].Body
	for _, want := range []string{
		"channel_checklist_status: `duplicate`",
		"checklist_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate checklist receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_CHECKLIST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate checklist receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelChecklistActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 41, Title: "Channel checklist"},
		Comment: &Comment{
			ID: 4101,
			Body: `@gitclaw /channel todo-list --route team-demo --list-id Release.List --message-id source-1 --notify-message-id notify-1
Title: Make channel messages spawn GitHub-native checklists
Checklist:
1. Confirm metadata-only source receipts.
2. Keep Slack/Telegram lightweight.
Notes:
Let GitHub become the durable checklist surface.`,
		},
	}
	req, err := BuildChannelChecklistActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelChecklistActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "todo-list" || req.Options.Route != "team-demo" || req.Options.ChecklistID != "release-list" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel checklist parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native checklists" || !strings.Contains(req.Options.Items, "Confirm metadata-only") || !strings.Contains(req.Options.Notes, "durable checklist") {
		t.Fatalf("unexpected checklist sections: %#v", req)
	}
	if req.ItemCount != 2 || req.TargetFromIssue || req.AutoChecklistID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ItemsSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route checklist hashes/counts: %#v", req)
	}
}

func TestIsChannelChecklistActionFieldsKeepsAliasesSeparate(t *testing.T) {
	if isChannelChecklistActionFields([]string{"/channels", "task"}) {
		t.Fatalf("task should remain a task alias, not a checklist alias")
	}
	if !isChannelChecklistActionFields([]string{"/channels", "checklist"}) {
		t.Fatalf("checklist should be accepted as a checklist alias")
	}
	if !isChannelChecklistActionFields([]string{"/channels", "todo-list"}) {
		t.Fatalf("todo-list should be accepted as a checklist alias")
	}
}
