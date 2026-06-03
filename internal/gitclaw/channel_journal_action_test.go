package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelJournalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-journal-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-journal-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-journal-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels journal --journal-id journal-1 --date 2026-06-03 --message-id inbound-384 --notify-message-id notify-384\nSummary: Team settled the launch readiness plan\nEntry:\nVisible journal entry with CHANNEL_JOURNAL_ENTRY_SECRET.",
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
			Number: 384,
			Title:  "GitClaw telegram thread chat-journal-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-journal-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_JOURNAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel journal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one journal issue: %#v", len(github.Issues), github.Issues)
	}
	journal := github.Issues[1]
	if !HasChannelJournalMarker(journal.Body) || !strings.Contains(journal.Body, `journal_id="journal-1"`) {
		t.Fatalf("journal issue missing channel-journal marker:\n%s", journal.Body)
	}
	for _, want := range []string{
		"GitClaw channel journal",
		"journal_id: journal-1",
		"journal_date: 2026-06-03",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"journal_mode: github-issue-journal",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Date",
		"2026-06-03",
		"Team settled the launch readiness plan",
		"## Entry",
		"Visible journal entry with CHANNEL_JOURNAL_ENTRY_SECRET.",
	} {
		if !strings.Contains(journal.Body, want) {
			t.Fatalf("journal issue missing %q:\n%s", want, journal.Body)
		}
	}
	if strings.Contains(journal.Body, "chat-journal-123") || strings.Contains(journal.Body, "inbound-384") || strings.Contains(journal.Body, "CHANNEL_JOURNAL_INGEST_SECRET") {
		t.Fatalf("journal issue leaked provider IDs or channel body:\n%s", journal.Body)
	}
	if !hasLabel(github.IssueLabels[journal.Number], "gitclaw") {
		t.Fatalf("journal issue missing gitclaw trigger label: %#v", github.IssueLabels[journal.Number])
	}

	sourceComments := github.CommentsByIssue[384]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-384"`,
		"GitClaw channel journal recorded.",
		"Journal: #101",
		"https://github.com/owner/repo/issues/101",
		"Date: 2026-06-03",
		"Summary: Team settled the launch readiness plan",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("journal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_JOURNAL_ENTRY_SECRET") || strings.Contains(outbound, "CHANNEL_JOURNAL_INGEST_SECRET") {
		t.Fatalf("journal notification leaked entry details or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Journal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels journal`",
		"channel_journal_status: `recorded`",
		"journal_issue: `#101`",
		"journal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_journal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"journal_date_sha256_12:",
		"journal_date_bytes: `10`",
		"journal_date_lines: `1`",
		"raw_journal_date_included: `false`",
		"raw_journal_summary_included: `false`",
		"raw_journal_highlights_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_journal_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel journal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_JOURNAL_INGEST_SECRET", "CHANNEL_JOURNAL_ENTRY_SECRET", "Team settled the launch", "2026-06-03", "journal-1", "chat-journal-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel journal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-journal-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-journal-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels journal --journal-id journal-1 --date 2026-06-03 --message-id inbound-384 --notify-message-id notify-384\nSummary: Team settled the launch readiness plan\nEntry:\nDo not leak duplicate token CHANNEL_JOURNAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate journal created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate journal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_journal_status: `duplicate`",
		"journal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate journal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_JOURNAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate journal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelJournalActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel journal"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel log-entry --route team-demo --journal-id Weekly.Log --date 2026-06-03 --message-id source-1 --notify-message-id notify-1
Summary: The channel reached launch readiness
Entry:
- Design is stable.
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelJournalActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelJournalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "log-entry" || req.Options.Route != "team-demo" || req.Options.JournalID != "weekly-log" || req.Options.JournalDate != "2026-06-03" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel journal parsing: %#v", req)
	}
	if req.Options.Summary != "The channel reached launch readiness" || !strings.Contains(req.Options.Highlights, "Design is stable") {
		t.Fatalf("unexpected summary/highlights: %#v", req)
	}
	if req.TargetFromIssue || req.AutoJournalID || req.AutoNotifyMessageID || req.DateSHA == "" || req.SummarySHA == "" || req.HighlightsSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route journal hashes: %#v", req)
	}
}
