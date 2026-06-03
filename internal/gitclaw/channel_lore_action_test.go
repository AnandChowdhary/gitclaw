package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelLoreCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-lore-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-lore-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-lore-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels lore --lore-id lore-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness lore\nLore:\nVisible lore token CHANNEL_LORE_BODY_SECRET.\nContext:\nVisible context token CHANNEL_LORE_CONTEXT_SECRET.\nSource:\nVisible source token CHANNEL_LORE_SOURCE_SECRET.\nReview:\nVisible review token CHANNEL_LORE_REVIEW_SECRET.",
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
			Title:  "GitClaw telegram thread chat-lore-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-lore-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_LORE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel lore action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one lore issue: %#v", len(github.Issues), github.Issues)
	}
	lore := github.Issues[1]
	if !HasChannelLoreMarker(lore.Body) || !strings.Contains(lore.Body, `lore_id="lore-1"`) {
		t.Fatalf("lore issue missing channel-lore marker:\n%s", lore.Body)
	}
	for _, want := range []string{
		"GitClaw channel lore",
		"lore_id: lore-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"lore_mode: github-issue-lore",
		"model_call_performed: false",
		"scheduled_workflow_created: false",
		"reminder_created: false",
		"soul_write_performed: false",
		"memory_write_performed: false",
		"policy_mutation_performed: false",
		"skill_install_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness lore",
		"## Lore",
		"Visible lore token CHANNEL_LORE_BODY_SECRET.",
		"## Context",
		"Visible context token CHANNEL_LORE_CONTEXT_SECRET.",
		"## Source",
		"Visible source token CHANNEL_LORE_SOURCE_SECRET.",
		"## Review",
		"Visible review token CHANNEL_LORE_REVIEW_SECRET.",
	} {
		if !strings.Contains(lore.Body, want) {
			t.Fatalf("lore issue missing %q:\n%s", want, lore.Body)
		}
	}
	if strings.Contains(lore.Body, "chat-lore-123") || strings.Contains(lore.Body, "inbound-384") || strings.Contains(lore.Body, "CHANNEL_LORE_INGEST_SECRET") {
		t.Fatalf("lore issue leaked provider IDs or channel body:\n%s", lore.Body)
	}
	if !hasLabel(github.IssueLabels[lore.Number], "gitclaw") {
		t.Fatalf("lore issue missing gitclaw trigger label: %#v", github.IssueLabels[lore.Number])
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
		"GitClaw channel lore recorded.",
		"Lore: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness lore",
		"Review: Visible review token CHANNEL_LORE_REVIEW_SECRET.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("lore notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_LORE_BODY_SECRET", "CHANNEL_LORE_CONTEXT_SECRET", "CHANNEL_LORE_SOURCE_SECRET", "CHANNEL_LORE_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("lore notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Lore Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels lore`",
		"channel_lore_status: `recorded`",
		"lore_issue: `#101`",
		"lore_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_lore_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_lore_title_included: `false`",
		"raw_lore_body_included: `false`",
		"raw_lore_context_included: `false`",
		"raw_lore_source_included: `false`",
		"raw_lore_review_included: `false`",
		"raw_channel_message_body_included: `false`",
		"model_call_performed: `false`",
		"scheduled_workflow_created: `false`",
		"reminder_created: `false`",
		"soul_write_performed: `false`",
		"memory_write_performed: `false`",
		"policy_mutation_performed: `false`",
		"skill_install_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_lore_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel lore receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_LORE_INGEST_SECRET", "CHANNEL_LORE_BODY_SECRET", "CHANNEL_LORE_CONTEXT_SECRET", "CHANNEL_LORE_SOURCE_SECRET", "CHANNEL_LORE_REVIEW_SECRET", "Launch readiness lore", "lore-1", "chat-lore-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel lore receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-lore-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-lore-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels lore --lore-id lore-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness lore\nLore:\nDo not leak duplicate token CHANNEL_LORE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate lore created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate lore posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_lore_status: `duplicate`",
		"lore_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate lore receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_LORE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate lore receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelLoreActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel lore"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel canon --route team-demo --lore-id Weekly.Lore --message-id source-1 --notify-message-id notify-1
Title: The channel reached launch readiness
Lore:
- Design is stable.
Context:
- Release checklist moved late.
Source:
- Follow-up moves to GitHub.
Review:
- Review tomorrow.`,
		},
	}
	req, err := BuildChannelLoreActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelLoreActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "canon" || req.Options.Route != "team-demo" || req.Options.LoreID != "weekly-lore" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel lore parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Lore, "Design is stable") || !strings.Contains(req.Options.Context, "Release checklist") || !strings.Contains(req.Options.Source, "Follow-up moves") || !strings.Contains(req.Options.Review, "Review tomorrow") {
		t.Fatalf("unexpected lore sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoLoreID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.LoreSHA == "" || req.ContextSHA == "" || req.SourceSHA == "" || req.ReviewSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route lore hashes: %#v", req)
	}
}

func TestIsChannelLoreActionFieldsKeepsPlaybookAndDigestAliasesSeparate(t *testing.T) {
	if isChannelLoreActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a lore alias")
	}
	if isChannelLoreActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should remain a playbook alias, not a lore alias")
	}
	if isChannelLoreActionFields([]string{"/channels", "challenge"}) {
		t.Fatalf("challenge should remain a quest alias, not a lore alias")
	}
	if isChannelLoreActionFields([]string{"/channels", "prediction"}) {
		t.Fatalf("prediction should remain a forecast alias, not a lore alias")
	}
	if !isChannelLoreActionFields([]string{"/channels", "context-note"}) {
		t.Fatalf("context-note should be accepted as a lore alias")
	}
}
