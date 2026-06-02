package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelStandingOrderProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-standing-order-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-standing-order-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-standing-order-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26301,
			"body": "@gitclaw /channels propose-order --id order-1 --cadence weekly --message-id inbound-263 --notify-message-id notify-263\nTitle: Weekly metrics review\n## Program: Weekly metrics review\n**Authority:** Compile internal metrics and draft a private summary.\n**Trigger:** Weekly on Friday.\n**Approval gate:** Human approval before external delivery.\n**Escalation:** Ask if source data is missing.\n\nVisible proposal body with CHANNEL_ORDER_PROPOSAL_BODY_SECRET.",
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
			Number: 263,
			Title:  "GitClaw telegram thread chat-standing-order-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{263: {{
			ID: 26300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-standing-order-123",
				MessageID: "inbound-263",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_ORDER_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{263: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel standing-order proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one standing-order proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposal := github.Issues[1]
	if proposal.Title != "GitClaw standing order proposal: order-1" {
		t.Fatalf("unexpected standing-order proposal title: %q", proposal.Title)
	}
	if !HasChannelStandingOrderProposalMarker(proposal.Body) || !strings.Contains(proposal.Body, `proposal_id="order-1"`) {
		t.Fatalf("standing-order proposal issue missing marker:\n%s", proposal.Body)
	}
	for _, want := range []string{
		"GitClaw channel standing-order proposal",
		"proposal_id: order-1",
		"source_channel: telegram",
		"source_issue: #263",
		"source_message_id_sha256_12:",
		"cadence: weekly",
		"target_path: .gitclaw/STANDING_ORDERS.md",
		"proposal_mode: github-issue-review",
		"review_pr_required: true",
		"standing_order_file_written: false",
		"schedule_created: false",
		"repository_mutation_performed: false",
		"proposal_body_included: true",
		"Weekly metrics review",
		"**Authority:** Compile internal metrics",
		"CHANNEL_ORDER_PROPOSAL_BODY_SECRET",
		"accepted changes happen through a normal PR",
	} {
		if !strings.Contains(proposal.Body, want) {
			t.Fatalf("standing-order proposal issue missing %q:\n%s", want, proposal.Body)
		}
	}
	if strings.Contains(proposal.Body, "chat-standing-order-123") || strings.Contains(proposal.Body, "inbound-263") || strings.Contains(proposal.Body, "CHANNEL_ORDER_INGEST_SECRET") {
		t.Fatalf("standing-order proposal issue leaked provider IDs or mirrored body:\n%s", proposal.Body)
	}
	if !hasLabel(github.IssueLabels[proposal.Number], "gitclaw") {
		t.Fatalf("standing-order proposal issue missing gitclaw trigger label: %#v", github.IssueLabels[proposal.Number])
	}

	sourceComments := github.CommentsByIssue[263]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-263"`,
		"GitClaw channel standing-order proposal created.",
		"Proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Proposal ID: order-1",
		"Title: Weekly metrics review",
		"Cadence: weekly",
		"Review PR required: true",
		"Standing orders changed: false",
		"Schedule created: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("standing-order proposal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_ORDER_PROPOSAL_BODY_SECRET") || strings.Contains(outbound, "CHANNEL_ORDER_INGEST_SECRET") {
		t.Fatalf("standing-order proposal notification leaked body or mirrored source:\n%s", outbound)
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Standing Order Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-order`",
		"channel_standing_order_proposal_status: `created`",
		"standing_order_proposal_issue: `#101`",
		"standing_order_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#263`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"proposal_store: `github-issue-to-git-reviewed-standing-orders-file`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"standing_order_file_written: `false`",
		"schedule_created: `false`",
		"repository_mutation_performed: `false`",
		"raw_proposal_id_included: `false`",
		"raw_proposal_cadence_included: `false`",
		"raw_proposal_title_included: `false`",
		"raw_proposal_body_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_standing_order_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("standing-order proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ORDER_INGEST_SECRET", "CHANNEL_ORDER_PROPOSAL_BODY_SECRET", "Weekly metrics review", "order-1", "weekly", "chat-standing-order-123", "inbound-263", "notify-263"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("standing-order proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-standing-order-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-standing-order-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26302,
			"body": "@gitclaw /channels propose-order --id order-1 --cadence weekly --message-id inbound-263 --notify-message-id notify-263\nTitle: Weekly metrics review\nDo not leak duplicate token CHANNEL_ORDER_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate standing-order proposal created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[263]); got != 4 {
		t.Fatalf("duplicate standing-order proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[263])
	}
	duplicateReceipt := github.CommentsByIssue[263][3].Body
	for _, want := range []string{
		"channel_standing_order_proposal_status: `duplicate`",
		"standing_order_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate standing-order proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ORDER_DUPLICATE_SECRET", "order-1", "weekly"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate standing-order proposal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelStandingOrderProposalActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 29, Title: "Channel standing-order proposal"},
		Comment: &Comment{
			ID: 2901,
			Body: `@gitclaw /channel standing-order-proposal --route team-demo --id Compliance.Review --cadence daily --message-id source-1 --notify-message-id notify-1
Title: Compliance digest
## Program: Compliance digest
**Authority:** Draft internal digest only.
**Trigger:** Daily.
**Approval gate:** Human approval before delivery.
**Escalation:** Missing source data.`,
		},
	}
	req, err := BuildChannelStandingOrderProposalActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelStandingOrderProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "standing-order-proposal" || req.Options.Route != "team-demo" || req.Options.ProposalID != "compliance-review" || req.Options.Cadence != "daily" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel standing-order proposal parsing: %#v", req)
	}
	if req.Options.Title != "Compliance digest" || !strings.Contains(req.Options.ProposalBody, "**Authority:** Draft internal digest only.") {
		t.Fatalf("unexpected standing-order proposal title/body: %#v", req)
	}
	if req.TargetFromIssue || req.AutoProposalID || req.AutoNotifyMessageID || req.CadenceSHA == "" || req.TitleSHA == "" || req.ProposalBodySHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route standing-order proposal hashes: %#v", req)
	}
}
