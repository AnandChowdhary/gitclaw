package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPactCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-pact-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-pact-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-pact-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels pact --pact-id pact-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness pact\nParticipants:\nVisible participants token CHANNEL_PACT_PARTICIPANTS_SECRET.\nAgreement:\nVisible agreement token CHANNEL_PACT_AGREEMENT_SECRET.\nScope:\nVisible scope token CHANNEL_PACT_SCOPE_SECRET.\nRevisit:\nVisible revisit token CHANNEL_PACT_REVISIT_SECRET.",
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
			Title:  "GitClaw telegram thread chat-pact-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-pact-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_PACT_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel pact action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one pact issue: %#v", len(github.Issues), github.Issues)
	}
	pact := github.Issues[1]
	if !HasChannelPactMarker(pact.Body) || !strings.Contains(pact.Body, `pact_id="pact-1"`) {
		t.Fatalf("pact issue missing channel-pact marker:\n%s", pact.Body)
	}
	for _, want := range []string{
		"GitClaw channel pact",
		"pact_id: pact-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"pact_mode: github-issue-pact",
		"scheduled_workflow_created: false",
		"reminder_created: false",
		"standing_order_created: false",
		"soul_write_performed: false",
		"memory_write_performed: false",
		"policy_mutation_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness pact",
		"## Participants",
		"Visible participants token CHANNEL_PACT_PARTICIPANTS_SECRET.",
		"## Agreement",
		"Visible agreement token CHANNEL_PACT_AGREEMENT_SECRET.",
		"## Scope",
		"Visible scope token CHANNEL_PACT_SCOPE_SECRET.",
		"## Revisit",
		"Visible revisit token CHANNEL_PACT_REVISIT_SECRET.",
	} {
		if !strings.Contains(pact.Body, want) {
			t.Fatalf("pact issue missing %q:\n%s", want, pact.Body)
		}
	}
	if strings.Contains(pact.Body, "chat-pact-123") || strings.Contains(pact.Body, "inbound-384") || strings.Contains(pact.Body, "CHANNEL_PACT_INGEST_SECRET") {
		t.Fatalf("pact issue leaked provider IDs or channel body:\n%s", pact.Body)
	}
	if !hasLabel(github.IssueLabels[pact.Number], "gitclaw") {
		t.Fatalf("pact issue missing gitclaw trigger label: %#v", github.IssueLabels[pact.Number])
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
		"GitClaw channel pact recorded.",
		"Pact: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness pact",
		"Participants: Visible participants token CHANNEL_PACT_PARTICIPANTS_SECRET.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("pact notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PACT_AGREEMENT_SECRET", "CHANNEL_PACT_SCOPE_SECRET", "CHANNEL_PACT_REVISIT_SECRET", "CHANNEL_PACT_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("pact notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Pact Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels pact`",
		"channel_pact_status: `recorded`",
		"pact_issue: `#101`",
		"pact_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_pact_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_pact_title_included: `false`",
		"raw_pact_participants_included: `false`",
		"raw_pact_agreement_included: `false`",
		"raw_pact_scope_included: `false`",
		"raw_pact_revisit_included: `false`",
		"raw_channel_message_body_included: `false`",
		"scheduled_workflow_created: `false`",
		"reminder_created: `false`",
		"standing_order_created: `false`",
		"soul_write_performed: `false`",
		"memory_write_performed: `false`",
		"policy_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_pact_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel pact receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PACT_INGEST_SECRET", "CHANNEL_PACT_PARTICIPANTS_SECRET", "CHANNEL_PACT_AGREEMENT_SECRET", "CHANNEL_PACT_SCOPE_SECRET", "CHANNEL_PACT_REVISIT_SECRET", "Launch readiness pact", "pact-1", "chat-pact-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel pact receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-pact-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-pact-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels pact --pact-id pact-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness pact\nScope:\nDo not leak duplicate token CHANNEL_PACT_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate pact created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate pact posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_pact_status: `duplicate`",
		"pact_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate pact receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_PACT_DUPLICATE_SECRET") {
		t.Fatalf("duplicate pact receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelPactActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel pact"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel agreement --route team-demo --pact-id Weekly.Pact --message-id source-1 --notify-message-id notify-1
Pact: The channel reached launch readiness
Participants:
- Release and design leads.
Agreement:
- We pause launch talk until the checklist has an owner.
Scope:
- Launch readiness threads only.
Revisit:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelPactActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPactActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "agreement" || req.Options.Route != "team-demo" || req.Options.PactID != "weekly-pact" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel pact parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Participants, "Release and design leads") || !strings.Contains(req.Options.Agreement, "pause launch talk") || !strings.Contains(req.Options.Scope, "Launch readiness") || !strings.Contains(req.Options.Revisit, "Follow-up moves") {
		t.Fatalf("unexpected pact sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoPactID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ParticipantsSHA == "" || req.AgreementSHA == "" || req.ScopeSHA == "" || req.RevisitSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route pact hashes: %#v", req)
	}
}

func TestIsChannelPactActionFieldsKeepsPlaybookAndDigestAliasesSeparate(t *testing.T) {
	if isChannelPactActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a pact alias")
	}
	if isChannelPactActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should remain a playbook alias, not a pact alias")
	}
	if !isChannelPactActionFields([]string{"/channels", "agreement"}) {
		t.Fatalf("agreement should be accepted as a pact alias")
	}
}
