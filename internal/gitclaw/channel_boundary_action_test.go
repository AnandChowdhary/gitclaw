package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBoundaryCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-boundary-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-boundary-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-boundary-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels boundary --boundary-id boundary-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness boundary\nBoundary:\nVisible boundary token CHANNEL_BOUNDARY_BODY_SECRET.\nScope:\nVisible scope token CHANNEL_BOUNDARY_SCOPE_SECRET.\nReason:\nVisible reason token CHANNEL_BOUNDARY_REASON_SECRET.\nReview:\nVisible review token CHANNEL_BOUNDARY_REVIEW_SECRET.",
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
			Title:  "GitClaw telegram thread chat-boundary-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-boundary-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BOUNDARY_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel boundary action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one boundary issue: %#v", len(github.Issues), github.Issues)
	}
	boundary := github.Issues[1]
	if !HasChannelBoundaryMarker(boundary.Body) || !strings.Contains(boundary.Body, `boundary_id="boundary-1"`) {
		t.Fatalf("boundary issue missing channel-boundary marker:\n%s", boundary.Body)
	}
	for _, want := range []string{
		"GitClaw channel boundary",
		"boundary_id: boundary-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"boundary_mode: github-issue-boundary",
		"model_call_performed: false",
		"scheduled_workflow_created: false",
		"reminder_created: false",
		"allowlist_mutation_performed: false",
		"pairing_code_issued: false",
		"soul_write_performed: false",
		"memory_write_performed: false",
		"policy_mutation_performed: false",
		"skill_install_performed: false",
		"workflow_mutation_performed: false",
		"provider_setting_mutation_performed: false",
		"repository_mutation_performed: false",
		"provider_delivery_performed: false",
		"enforcement_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness boundary",
		"## Boundary",
		"Visible boundary token CHANNEL_BOUNDARY_BODY_SECRET.",
		"## Scope",
		"Visible scope token CHANNEL_BOUNDARY_SCOPE_SECRET.",
		"## Reason",
		"Visible reason token CHANNEL_BOUNDARY_REASON_SECRET.",
		"## Review",
		"Visible review token CHANNEL_BOUNDARY_REVIEW_SECRET.",
	} {
		if !strings.Contains(boundary.Body, want) {
			t.Fatalf("boundary issue missing %q:\n%s", want, boundary.Body)
		}
	}
	if strings.Contains(boundary.Body, "chat-boundary-123") || strings.Contains(boundary.Body, "inbound-384") || strings.Contains(boundary.Body, "CHANNEL_BOUNDARY_INGEST_SECRET") {
		t.Fatalf("boundary issue leaked provider IDs or channel body:\n%s", boundary.Body)
	}
	if !hasLabel(github.IssueLabels[boundary.Number], "gitclaw") {
		t.Fatalf("boundary issue missing gitclaw trigger label: %#v", github.IssueLabels[boundary.Number])
	}

	reasonComments := github.CommentsByIssue[384]
	if len(reasonComments) != 3 {
		t.Fatalf("reason comments = %d, want message + outbound + receipt: %#v", len(reasonComments), reasonComments)
	}
	outbound := reasonComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-384"`,
		"GitClaw channel boundary recorded.",
		"Boundary: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness boundary",
		"Review: Visible review token CHANNEL_BOUNDARY_REVIEW_SECRET.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("boundary notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BOUNDARY_BODY_SECRET", "CHANNEL_BOUNDARY_SCOPE_SECRET", "CHANNEL_BOUNDARY_REASON_SECRET", "CHANNEL_BOUNDARY_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("boundary notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := reasonComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Boundary Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels boundary`",
		"channel_boundary_status: `recorded`",
		"boundary_issue: `#101`",
		"boundary_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_boundary_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_boundary_title_included: `false`",
		"raw_boundary_body_included: `false`",
		"raw_boundary_scope_included: `false`",
		"raw_boundary_reason_included: `false`",
		"raw_boundary_review_included: `false`",
		"raw_channel_message_body_included: `false`",
		"model_call_performed: `false`",
		"scheduled_workflow_created: `false`",
		"reminder_created: `false`",
		"allowlist_mutation_performed: `false`",
		"pairing_code_issued: `false`",
		"soul_write_performed: `false`",
		"memory_write_performed: `false`",
		"policy_mutation_performed: `false`",
		"skill_install_performed: `false`",
		"workflow_mutation_performed: `false`",
		"provider_setting_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"enforcement_performed: `false`",
		"llm_e2e_required_after_channel_boundary_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel boundary receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BOUNDARY_INGEST_SECRET", "CHANNEL_BOUNDARY_BODY_SECRET", "CHANNEL_BOUNDARY_SCOPE_SECRET", "CHANNEL_BOUNDARY_REASON_SECRET", "CHANNEL_BOUNDARY_REVIEW_SECRET", "Launch readiness boundary", "boundary-1", "chat-boundary-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel boundary receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-boundary-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-boundary-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels boundary --boundary-id boundary-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness boundary\nBoundary:\nDo not leak duplicate token CHANNEL_BOUNDARY_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate boundary created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate boundary posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_boundary_status: `duplicate`",
		"boundary_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate boundary receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_BOUNDARY_DUPLICATE_SECRET") {
		t.Fatalf("duplicate boundary receipt leaked reason:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelBoundaryActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel boundary"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel guardrail --route team-demo --boundary-id Weekly.Boundary --message-id source-1 --notify-message-id notify-1
Title: The channel reached launch readiness
Boundary:
- Design is stable.
Scope:
- Release checklist moved late.
Reason:
- Follow-up moves to GitHub.
Review:
- Review tomorrow.`,
		},
	}
	req, err := BuildChannelBoundaryActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBoundaryActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "guardrail" || req.Options.Route != "team-demo" || req.Options.BoundaryID != "weekly-boundary" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel boundary parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Boundary, "Design is stable") || !strings.Contains(req.Options.Scope, "Release checklist") || !strings.Contains(req.Options.Reason, "Follow-up moves") || !strings.Contains(req.Options.Review, "Review tomorrow") {
		t.Fatalf("unexpected boundary sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoBoundaryID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.BoundarySHA == "" || req.ScopeSHA == "" || req.ReasonSHA == "" || req.ReviewSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route boundary hashes: %#v", req)
	}
}

func TestIsChannelBoundaryActionFieldsKeepsPlaybookAndDigestAliasesSeparate(t *testing.T) {
	if isChannelBoundaryActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a boundary alias")
	}
	if isChannelBoundaryActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should remain a playbook alias, not a boundary alias")
	}
	if isChannelBoundaryActionFields([]string{"/channels", "challenge"}) {
		t.Fatalf("challenge should remain a quest alias, not a boundary alias")
	}
	if isChannelBoundaryActionFields([]string{"/channels", "prediction"}) {
		t.Fatalf("prediction should remain a forecast alias, not a boundary alias")
	}
	if isChannelBoundaryActionFields([]string{"/channels", "lore"}) {
		t.Fatalf("lore should remain a lore alias, not a boundary alias")
	}
	if !isChannelBoundaryActionFields([]string{"/channels", "guardrail"}) {
		t.Fatalf("guardrail should be accepted as a boundary alias")
	}
}
