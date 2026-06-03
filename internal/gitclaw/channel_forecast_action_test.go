package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelForecastCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-forecast-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-forecast-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-forecast-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels forecast --forecast-id forecast-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness forecast\nPrediction:\nVisible prediction token CHANNEL_FORECAST_PREDICTION_SECRET.\nEvidence:\nVisible evidence token CHANNEL_FORECAST_EVIDENCE_SECRET.\nResolution:\nVisible resolution token CHANNEL_FORECAST_RESOLUTION_SECRET.\nDue:\nVisible due token CHANNEL_FORECAST_DUE_SECRET.",
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
			Title:  "GitClaw telegram thread chat-forecast-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-forecast-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_FORECAST_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel forecast action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one forecast issue: %#v", len(github.Issues), github.Issues)
	}
	forecast := github.Issues[1]
	if !HasChannelForecastMarker(forecast.Body) || !strings.Contains(forecast.Body, `forecast_id="forecast-1"`) {
		t.Fatalf("forecast issue missing channel-forecast marker:\n%s", forecast.Body)
	}
	for _, want := range []string{
		"GitClaw channel forecast",
		"forecast_id: forecast-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"forecast_mode: github-issue-forecast",
		"scheduled_workflow_created: false",
		"reminder_created: false",
		"wager_created: false",
		"money_or_points_tracked: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness forecast",
		"## Prediction",
		"Visible prediction token CHANNEL_FORECAST_PREDICTION_SECRET.",
		"## Evidence",
		"Visible evidence token CHANNEL_FORECAST_EVIDENCE_SECRET.",
		"## Resolution",
		"Visible resolution token CHANNEL_FORECAST_RESOLUTION_SECRET.",
		"## Due / Review",
		"Visible due token CHANNEL_FORECAST_DUE_SECRET.",
	} {
		if !strings.Contains(forecast.Body, want) {
			t.Fatalf("forecast issue missing %q:\n%s", want, forecast.Body)
		}
	}
	if strings.Contains(forecast.Body, "chat-forecast-123") || strings.Contains(forecast.Body, "inbound-384") || strings.Contains(forecast.Body, "CHANNEL_FORECAST_INGEST_SECRET") {
		t.Fatalf("forecast issue leaked provider IDs or channel body:\n%s", forecast.Body)
	}
	if !hasLabel(github.IssueLabels[forecast.Number], "gitclaw") {
		t.Fatalf("forecast issue missing gitclaw trigger label: %#v", github.IssueLabels[forecast.Number])
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
		"GitClaw channel forecast recorded.",
		"Forecast: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness forecast",
		"Due: Visible due token CHANNEL_FORECAST_DUE_SECRET.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("forecast notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORECAST_PREDICTION_SECRET", "CHANNEL_FORECAST_EVIDENCE_SECRET", "CHANNEL_FORECAST_RESOLUTION_SECRET", "CHANNEL_FORECAST_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("forecast notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Forecast Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels forecast`",
		"channel_forecast_status: `recorded`",
		"forecast_issue: `#101`",
		"forecast_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_forecast_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_forecast_title_included: `false`",
		"raw_forecast_prediction_included: `false`",
		"raw_forecast_evidence_included: `false`",
		"raw_forecast_resolution_included: `false`",
		"raw_forecast_due_included: `false`",
		"raw_channel_message_body_included: `false`",
		"scheduled_workflow_created: `false`",
		"reminder_created: `false`",
		"wager_created: `false`",
		"money_or_points_tracked: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_forecast_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel forecast receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_FORECAST_INGEST_SECRET", "CHANNEL_FORECAST_PREDICTION_SECRET", "CHANNEL_FORECAST_EVIDENCE_SECRET", "CHANNEL_FORECAST_RESOLUTION_SECRET", "CHANNEL_FORECAST_DUE_SECRET", "Launch readiness forecast", "forecast-1", "chat-forecast-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel forecast receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-forecast-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-forecast-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels forecast --forecast-id forecast-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness forecast\nPrediction:\nDo not leak duplicate token CHANNEL_FORECAST_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate forecast created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate forecast posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_forecast_status: `duplicate`",
		"forecast_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate forecast receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_FORECAST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate forecast receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelForecastActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel forecast"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel prediction --route team-demo --forecast-id Weekly.Forecast --message-id source-1 --notify-message-id notify-1
Forecast: The channel reached launch readiness
Prediction:
- Design is stable.
Evidence:
- Release checklist moved late.
Resolution:
- Follow-up moves to GitHub.
Due:
- Review tomorrow.`,
		},
	}
	req, err := BuildChannelForecastActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelForecastActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "prediction" || req.Options.Route != "team-demo" || req.Options.ForecastID != "weekly-forecast" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel forecast parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.Prediction, "Design is stable") || !strings.Contains(req.Options.Evidence, "Release checklist") || !strings.Contains(req.Options.Resolution, "Follow-up moves") || !strings.Contains(req.Options.Due, "Review tomorrow") {
		t.Fatalf("unexpected forecast sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoForecastID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.PredictionSHA == "" || req.EvidenceSHA == "" || req.ResolutionSHA == "" || req.DueSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route forecast hashes: %#v", req)
	}
}

func TestIsChannelForecastActionFieldsKeepsPlaybookAndDigestAliasesSeparate(t *testing.T) {
	if isChannelForecastActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a forecast alias")
	}
	if isChannelForecastActionFields([]string{"/channels", "runbook"}) {
		t.Fatalf("runbook should remain a playbook alias, not a forecast alias")
	}
	if isChannelForecastActionFields([]string{"/channels", "challenge"}) {
		t.Fatalf("challenge should remain a quest alias, not a forecast alias")
	}
	if !isChannelForecastActionFields([]string{"/channels", "prediction"}) {
		t.Fatalf("prediction should be accepted as a forecast alias")
	}
}
