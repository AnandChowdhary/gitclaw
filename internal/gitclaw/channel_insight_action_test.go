package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelInsightCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-insight-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-insight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-insight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels insight --insight-id insight-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness insight\nObservation:\nVisible observation token CHANNEL_INSIGHT_OBSERVATION_SECRET.\nEvidence:\nVisible evidence token CHANNEL_INSIGHT_EVIDENCE_SECRET.\nRecommendation:\nVisible recommendation token CHANNEL_INSIGHT_RECOMMENDATION_SECRET.",
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
			Title:  "GitClaw telegram thread chat-insight-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-insight-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_INSIGHT_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel insight action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one insight issue: %#v", len(github.Issues), github.Issues)
	}
	insight := github.Issues[1]
	if !HasChannelInsightMarker(insight.Body) || !strings.Contains(insight.Body, `insight_id="insight-1"`) {
		t.Fatalf("insight issue missing channel-insight marker:\n%s", insight.Body)
	}
	for _, want := range []string{
		"GitClaw channel insight",
		"insight_id: insight-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"insight_mode: github-issue-insight",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness insight",
		"## Observation",
		"Visible observation token CHANNEL_INSIGHT_OBSERVATION_SECRET.",
		"## Evidence",
		"Visible evidence token CHANNEL_INSIGHT_EVIDENCE_SECRET.",
		"## Recommendation",
		"Visible recommendation token CHANNEL_INSIGHT_RECOMMENDATION_SECRET.",
	} {
		if !strings.Contains(insight.Body, want) {
			t.Fatalf("insight issue missing %q:\n%s", want, insight.Body)
		}
	}
	if strings.Contains(insight.Body, "chat-insight-123") || strings.Contains(insight.Body, "inbound-384") || strings.Contains(insight.Body, "CHANNEL_INSIGHT_INGEST_SECRET") {
		t.Fatalf("insight issue leaked provider IDs or channel body:\n%s", insight.Body)
	}
	if !hasLabel(github.IssueLabels[insight.Number], "gitclaw") {
		t.Fatalf("insight issue missing gitclaw trigger label: %#v", github.IssueLabels[insight.Number])
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
		"GitClaw channel insight recorded.",
		"Insight: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness insight",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("insight notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_INSIGHT_OBSERVATION_SECRET", "CHANNEL_INSIGHT_EVIDENCE_SECRET", "CHANNEL_INSIGHT_RECOMMENDATION_SECRET", "CHANNEL_INSIGHT_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("insight notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Insight Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels insight`",
		"channel_insight_status: `recorded`",
		"insight_issue: `#101`",
		"insight_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_insight_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_insight_title_included: `false`",
		"raw_insight_observation_included: `false`",
		"raw_insight_evidence_included: `false`",
		"raw_insight_recommendation_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_insight_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel insight receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_INSIGHT_INGEST_SECRET", "CHANNEL_INSIGHT_OBSERVATION_SECRET", "CHANNEL_INSIGHT_EVIDENCE_SECRET", "CHANNEL_INSIGHT_RECOMMENDATION_SECRET", "Launch readiness insight", "insight-1", "chat-insight-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel insight receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-insight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-insight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels insight --insight-id insight-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness insight\nObservation:\nDo not leak duplicate token CHANNEL_INSIGHT_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate insight created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate insight posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_insight_status: `duplicate`",
		"insight_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate insight receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_INSIGHT_DUPLICATE_SECRET") {
		t.Fatalf("duplicate insight receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelInsightActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel insight"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel finding --route team-demo --insight-id Weekly.Insight --message-id source-1 --notify-message-id notify-1
Insight: The channel reached launch readiness
Observation:
- Design is stable.
Evidence:
- Release follow-up moved late.
Recommendation:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelInsightActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelInsightActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "finding" || req.Options.Route != "team-demo" || req.Options.InsightID != "weekly-insight" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel insight parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Observation, "Design is stable") || !strings.Contains(req.Options.Evidence, "Release follow-up") || !strings.Contains(req.Options.Recommendation, "Follow-up moves") {
		t.Fatalf("unexpected insight sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoInsightID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ObservationSHA == "" || req.EvidenceSHA == "" || req.RecommendationSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route insight hashes: %#v", req)
	}
}

func TestIsChannelInsightActionFieldsKeepsDigestAliasesSeparate(t *testing.T) {
	if isChannelInsightActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a insight alias")
	}
	if !isChannelInsightActionFields([]string{"/channels", "takeaway"}) {
		t.Fatalf("takeaway should be accepted as an insight alias")
	}
}
