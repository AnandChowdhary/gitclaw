package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelQuoteCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-quote-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-quote-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-quote-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels quote --quote-id quote-1 --message-id inbound-484 --notify-message-id notify-484\nQuote: Build a channel-native quote incubator\nNotes:\nVisible quote note with CHANNEL_QUOTE_NOTE_SECRET.",
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
			Title:  "GitClaw telegram thread chat-quote-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-quote-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_QUOTE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel quote action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one quote issue: %#v", len(github.Issues), github.Issues)
	}
	quote := github.Issues[1]
	if !HasChannelQuoteMarker(quote.Body) || !strings.Contains(quote.Body, `quote_id="quote-1"`) {
		t.Fatalf("quote issue missing channel-quote marker:\n%s", quote.Body)
	}
	for _, want := range []string{
		"GitClaw channel quote",
		"quote_id: quote-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"quote_mode: github-issue-quote",
		"memory_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Build a channel-native quote incubator",
		"Visible quote note with CHANNEL_QUOTE_NOTE_SECRET.",
	} {
		if !strings.Contains(quote.Body, want) {
			t.Fatalf("quote issue missing %q:\n%s", want, quote.Body)
		}
	}
	if strings.Contains(quote.Body, "chat-quote-123") || strings.Contains(quote.Body, "inbound-484") || strings.Contains(quote.Body, "CHANNEL_QUOTE_INGEST_SECRET") {
		t.Fatalf("quote issue leaked provider IDs or channel body:\n%s", quote.Body)
	}
	if !hasLabel(github.IssueLabels[quote.Number], "gitclaw") {
		t.Fatalf("quote issue missing gitclaw trigger label: %#v", github.IssueLabels[quote.Number])
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
		"GitClaw channel quote captured.",
		"Quote: #101",
		"https://github.com/owner/repo/issues/101",
		"Quote: Build a channel-native quote incubator",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("quote notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_QUOTE_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_QUOTE_INGEST_SECRET") {
		t.Fatalf("quote notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Quote Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels quote`",
		"channel_quote_status: `captured`",
		"quote_issue: `#101`",
		"quote_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_quote_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_quote_text_included: `false`",
		"raw_quote_context_included: `false`",
		"raw_channel_message_body_included: `false`",
		"memory_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_quote_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel quote receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_QUOTE_INGEST_SECRET", "CHANNEL_QUOTE_NOTE_SECRET", "Build a channel-native", "quote-1", "chat-quote-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel quote receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-quote-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-quote-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels quote --quote-id quote-1 --message-id inbound-484 --notify-message-id notify-484\nQuote: Build a channel-native quote incubator\nNotes:\nDo not leak duplicate token CHANNEL_QUOTE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate quote created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate quote posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_quote_status: `duplicate`",
		"quote_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate quote receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_QUOTE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate quote receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelQuoteActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel quote"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel pullquote --route team-demo --quote-id Roadmap.Spark --message-id source-1 --notify-message-id notify-1
Quote: Make channel messages spawn GitHub-native quote labs
Context:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable shaping surface.`,
		},
	}
	req, err := BuildChannelQuoteActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelQuoteActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "pullquote" || req.Options.Route != "team-demo" || req.Options.QuoteID != "roadmap-spark" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel quote parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native quote labs" || !strings.Contains(req.Options.Notes, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoQuoteID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route quote hashes: %#v", req)
	}
}
