package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelDecisionCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-decision-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 283,
			"title": "GitClaw telegram thread chat-decision-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-decision-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 28301,
			"body": "@gitclaw /channels decision --decision-id decision-1 --message-id inbound-283 --notify-message-id notify-283\nDecision: Use the GitHub-native inbox for launch approvals\nRationale:\nVisible decision rationale with CHANNEL_DECISION_RATIONALE_SECRET.",
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
			Number: 283,
			Title:  "GitClaw telegram thread chat-decision-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{283: {{
			ID: 28300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-decision-123",
				MessageID: "inbound-283",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_DECISION_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{283: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel decision action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one decision issue: %#v", len(github.Issues), github.Issues)
	}
	decision := github.Issues[1]
	if !HasChannelDecisionMarker(decision.Body) || !strings.Contains(decision.Body, `decision_id="decision-1"`) {
		t.Fatalf("decision issue missing channel-decision marker:\n%s", decision.Body)
	}
	for _, want := range []string{
		"GitClaw channel decision",
		"decision_id: decision-1",
		"source_channel: telegram",
		"source_issue: #283",
		"source_message_id_sha256_12:",
		"decision_mode: github-issue-decision",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Use the GitHub-native inbox for launch approvals",
		"Visible decision rationale with CHANNEL_DECISION_RATIONALE_SECRET.",
	} {
		if !strings.Contains(decision.Body, want) {
			t.Fatalf("decision issue missing %q:\n%s", want, decision.Body)
		}
	}
	if strings.Contains(decision.Body, "chat-decision-123") || strings.Contains(decision.Body, "inbound-283") || strings.Contains(decision.Body, "CHANNEL_DECISION_INGEST_SECRET") {
		t.Fatalf("decision issue leaked provider IDs or channel body:\n%s", decision.Body)
	}
	if !hasLabel(github.IssueLabels[decision.Number], "gitclaw") {
		t.Fatalf("decision issue missing gitclaw trigger label: %#v", github.IssueLabels[decision.Number])
	}

	sourceComments := github.CommentsByIssue[283]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-283"`,
		"GitClaw channel decision recorded.",
		"Decision: #101",
		"https://github.com/owner/repo/issues/101",
		"Summary: Use the GitHub-native inbox for launch approvals",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("decision notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_DECISION_RATIONALE_SECRET") || strings.Contains(outbound, "CHANNEL_DECISION_INGEST_SECRET") {
		t.Fatalf("decision notification leaked rationale or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Decision Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels decision`",
		"channel_decision_status: `recorded`",
		"decision_issue: `#101`",
		"decision_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#283`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_decision_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_decision_text_included: `false`",
		"raw_rationale_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_decision_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel decision receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_DECISION_INGEST_SECRET", "CHANNEL_DECISION_RATIONALE_SECRET", "Use the GitHub-native inbox", "decision-1", "chat-decision-123", "inbound-283", "notify-283"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel decision receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 283,
			"title": "GitClaw telegram thread chat-decision-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-decision-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 28302,
			"body": "@gitclaw /channels decision --decision-id decision-1 --message-id inbound-283 --notify-message-id notify-283\nDecision: Use the GitHub-native inbox for launch approvals\nRationale:\nDo not leak duplicate token CHANNEL_DECISION_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate decision created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[283]); got != 4 {
		t.Fatalf("duplicate decision posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[283])
	}
	duplicateReceipt := github.CommentsByIssue[283][3].Body
	for _, want := range []string{
		"channel_decision_status: `duplicate`",
		"decision_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate decision receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_DECISION_DUPLICATE_SECRET") {
		t.Fatalf("duplicate decision receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelDecisionActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel decision"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel decide --route team-demo --decision-id Design.Decision --message-id source-1 --notify-message-id notify-1
Decision: Keep the GitHub issue as the canonical decision log
Rationale:
It is reviewable and durable.`,
		},
	}
	req, err := BuildChannelDecisionActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelDecisionActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "decide" || req.Options.Route != "team-demo" || req.Options.DecisionID != "design-decision" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel decision parsing: %#v", req)
	}
	if req.Options.Decision != "Keep the GitHub issue as the canonical decision log" || !strings.Contains(req.Options.Rationale, "reviewable and durable") {
		t.Fatalf("unexpected decision/rationale: %#v", req)
	}
	if req.TargetFromIssue || req.AutoDecisionID || req.AutoNotifyMessageID || req.DecisionSHA == "" || req.RationaleSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route decision hashes: %#v", req)
	}
}
