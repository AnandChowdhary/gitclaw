package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelGlossaryCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-glossary-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-glossary-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-glossary-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels glossary --glossary-id glossary-1 --message-id inbound-484 --notify-message-id notify-484\nTerm: Channel-native glossary\nDefinition:\nVisible glossary definition with CHANNEL_GLOSSARY_DEFINITION_SECRET.",
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
			Title:  "GitClaw telegram thread chat-glossary-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-glossary-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_GLOSSARY_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel glossary action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one glossary issue: %#v", len(github.Issues), github.Issues)
	}
	glossary := github.Issues[1]
	if !HasChannelGlossaryMarker(glossary.Body) || !strings.Contains(glossary.Body, `glossary_id="glossary-1"`) {
		t.Fatalf("glossary issue missing channel-glossary marker:\n%s", glossary.Body)
	}
	for _, want := range []string{
		"GitClaw channel glossary entry",
		"glossary_id: glossary-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"glossary_mode: github-issue-glossary",
		"memory_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Channel-native glossary",
		"Visible glossary definition with CHANNEL_GLOSSARY_DEFINITION_SECRET.",
	} {
		if !strings.Contains(glossary.Body, want) {
			t.Fatalf("glossary issue missing %q:\n%s", want, glossary.Body)
		}
	}
	if strings.Contains(glossary.Body, "chat-glossary-123") || strings.Contains(glossary.Body, "inbound-484") || strings.Contains(glossary.Body, "CHANNEL_GLOSSARY_INGEST_SECRET") {
		t.Fatalf("glossary issue leaked provider IDs or channel body:\n%s", glossary.Body)
	}
	if !hasLabel(github.IssueLabels[glossary.Number], "gitclaw") {
		t.Fatalf("glossary issue missing gitclaw trigger label: %#v", github.IssueLabels[glossary.Number])
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
		"GitClaw channel glossary entry captured.",
		"Glossary entry: #101",
		"https://github.com/owner/repo/issues/101",
		"Term: Channel-native glossary",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("glossary notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_GLOSSARY_DEFINITION_SECRET") || strings.Contains(outbound, "CHANNEL_GLOSSARY_INGEST_SECRET") {
		t.Fatalf("glossary notification leaked definition or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Glossary Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels glossary`",
		"channel_glossary_status: `captured`",
		"glossary_issue: `#101`",
		"glossary_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_glossary_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_glossary_term_included: `false`",
		"raw_glossary_definition_included: `false`",
		"raw_channel_message_body_included: `false`",
		"memory_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_glossary_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel glossary receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_GLOSSARY_INGEST_SECRET", "CHANNEL_GLOSSARY_DEFINITION_SECRET", "Channel-native glossary", "glossary-1", "chat-glossary-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel glossary receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-glossary-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-glossary-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels glossary --glossary-id glossary-1 --message-id inbound-484 --notify-message-id notify-484\nTerm: Channel-native glossary\nDefinition:\nDo not leak duplicate token CHANNEL_GLOSSARY_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate glossary created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate glossary posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_glossary_status: `duplicate`",
		"glossary_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate glossary receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_GLOSSARY_DUPLICATE_SECRET") {
		t.Fatalf("duplicate glossary receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelGlossaryActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel glossary"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel define --route team-demo --term-id Roadmap.Spark --message-id source-1 --notify-message-id notify-1
Term: Channel glossary lab
Definition:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable review surface.`,
		},
	}
	req, err := BuildChannelGlossaryActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelGlossaryActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "define" || req.Options.Route != "team-demo" || req.Options.GlossaryID != "roadmap-spark" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel glossary parsing: %#v", req)
	}
	if req.Options.Term != "Channel glossary lab" || !strings.Contains(req.Options.Definition, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected term/definition: %#v", req)
	}
	if req.TargetFromIssue || req.AutoGlossaryID || req.AutoNotifyMessageID || req.TermSHA == "" || req.DefinitionSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route glossary hashes: %#v", req)
	}
}
