package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelAgendaCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-agenda-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 684,
			"title": "GitClaw telegram thread chat-agenda-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-agenda-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 68401,
			"body": "@gitclaw /channels agenda --agenda-id agenda-1 --message-id inbound-684 --notify-message-id notify-684\nTitle: Launch channel agenda\nItems:\n- Confirm deployment with CHANNEL_AGENDA_ITEM_SECRET\n- Notify the release room\nNotes:\nVisible agenda note with CHANNEL_AGENDA_NOTE_SECRET.",
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
			Title:  "GitClaw telegram thread chat-agenda-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{684: {{
			ID: 68400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-agenda-123",
				MessageID: "inbound-684",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_AGENDA_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{684: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel agenda action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one agenda issue: %#v", len(github.Issues), github.Issues)
	}
	agenda := github.Issues[1]
	if !HasChannelAgendaMarker(agenda.Body) || !strings.Contains(agenda.Body, `agenda_id="agenda-1"`) {
		t.Fatalf("agenda issue missing channel-agenda marker:\n%s", agenda.Body)
	}
	for _, want := range []string{
		"GitClaw channel agenda",
		"agenda_id: agenda-1",
		"item_count: 2",
		"source_channel: telegram",
		"source_issue: #684",
		"source_message_id_sha256_12:",
		"agenda_mode: github-issue-agenda",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Agenda Items",
		"1. Confirm deployment with CHANNEL_AGENDA_ITEM_SECRET",
		"2. Notify the release room",
		"## Title",
		"Launch channel agenda",
		"## Notes",
		"Visible agenda note with CHANNEL_AGENDA_NOTE_SECRET.",
	} {
		if !strings.Contains(agenda.Body, want) {
			t.Fatalf("agenda issue missing %q:\n%s", want, agenda.Body)
		}
	}
	if strings.Contains(agenda.Body, "chat-agenda-123") || strings.Contains(agenda.Body, "inbound-684") || strings.Contains(agenda.Body, "CHANNEL_AGENDA_INGEST_SECRET") {
		t.Fatalf("agenda issue leaked provider IDs or channel body:\n%s", agenda.Body)
	}
	if !hasLabel(github.IssueLabels[agenda.Number], "gitclaw") {
		t.Fatalf("agenda issue missing gitclaw trigger label: %#v", github.IssueLabels[agenda.Number])
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
		"GitClaw channel agenda captured.",
		"Agenda: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch channel agenda",
		"Items: 2",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("agenda notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_AGENDA_ITEM_SECRET") || strings.Contains(outbound, "CHANNEL_AGENDA_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_AGENDA_INGEST_SECRET") {
		t.Fatalf("agenda notification leaked items, notes, or channel body:\n%s", outbound)
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Agenda Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels agenda`",
		"channel_agenda_status: `captured`",
		"agenda_issue: `#101`",
		"agenda_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#684`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"agenda_item_count: `2`",
		"agenda_mode: `github-issue-agenda`",
		"repository_mutation_performed: `false`",
		"raw_agenda_id_included: `false`",
		"raw_agenda_title_included: `false`",
		"raw_agenda_items_included: `false`",
		"raw_agenda_notes_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_agenda_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel agenda receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_AGENDA_INGEST_SECRET", "CHANNEL_AGENDA_ITEM_SECRET", "CHANNEL_AGENDA_NOTE_SECRET", "Launch channel agenda", "agenda-1", "chat-agenda-123", "inbound-684", "notify-684", "Notify the release room"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel agenda receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 684,
			"title": "GitClaw telegram thread chat-agenda-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-agenda-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 68402,
			"body": "@gitclaw /channels agenda --agenda-id agenda-1 --message-id inbound-684 --notify-message-id notify-684\nTitle: Launch channel agenda\nItems:\n- Do not leak duplicate token CHANNEL_AGENDA_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate agenda created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[684]); got != 4 {
		t.Fatalf("duplicate agenda posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[684])
	}
	duplicateReceipt := github.CommentsByIssue[684][3].Body
	for _, want := range []string{
		"channel_agenda_status: `duplicate`",
		"agenda_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate agenda receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_AGENDA_DUPLICATE_SECRET") {
		t.Fatalf("duplicate agenda receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelAgendaActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 41, Title: "Channel agenda"},
		Comment: &Comment{
			ID: 4101,
			Body: `@gitclaw /channel meeting-agenda --route team-demo --meeting-id Release.List --message-id source-1 --notify-message-id notify-1
Title: Make channel messages spawn GitHub-native agendas
Topics:
1. Confirm metadata-only source receipts.
2. Keep Slack/Telegram lightweight.
Notes:
Let GitHub become the durable agenda surface.`,
		},
	}
	req, err := BuildChannelAgendaActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelAgendaActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "meeting-agenda" || req.Options.Route != "team-demo" || req.Options.AgendaID != "release-list" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel agenda parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native agendas" || !strings.Contains(req.Options.Items, "Confirm metadata-only") || !strings.Contains(req.Options.Notes, "durable agenda") {
		t.Fatalf("unexpected agenda sections: %#v", req)
	}
	if req.ItemCount != 2 || req.TargetFromIssue || req.AutoAgendaID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ItemsSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route agenda hashes/counts: %#v", req)
	}
}

func TestIsChannelAgendaActionFieldsKeepsAliasesSeparate(t *testing.T) {
	if isChannelAgendaActionFields([]string{"/channels", "task"}) {
		t.Fatalf("task should remain a task alias, not an agenda alias")
	}
	if !isChannelAgendaActionFields([]string{"/channels", "agenda"}) {
		t.Fatalf("agenda should be accepted as an agenda alias")
	}
	if !isChannelAgendaActionFields([]string{"/channels", "meeting-agenda"}) {
		t.Fatalf("meeting-agenda should be accepted as an agenda alias")
	}
	if !isChannelAgendaActionFields([]string{"/channels", "talking-points"}) {
		t.Fatalf("talking-points should be accepted as an agenda alias")
	}
}
