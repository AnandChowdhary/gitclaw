package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelModelStatusQueuesRuntimeSnapshotWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-model-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 885,
			"title": "GitClaw telegram thread chat-model-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-model-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88501,
			"body": "@gitclaw /channels model --message-id model-inbound-885 --notify-message-id model-notify-885 --status-id model-status-secret-885\nDo not include this command hidden token in the receipt: CHANNEL_MODEL_STATUS_COMMAND_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.ModelFallbacks = []string{"openai/gpt-4.1-nano"}
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 885,
			Title:  "GitClaw telegram thread chat-model-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{885: {{
			ID: 88500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-model-123",
				MessageID: "model-inbound-885",
				Author:    "telegram",
				Body:      "Original mirrored model-status command with CHANNEL_MODEL_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{885: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel model status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("model status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[885]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="model-notify-885"`,
		"GitClaw channel model status.",
		"Provider: github-models",
		"Model: openai/gpt-5-nano",
		"Fallback models: openai/gpt-4.1-nano",
		"Fallbacks configured: 1",
		"Endpoint host: models.github.ai",
		"Run mode: read-only",
		"Model call: not performed by this action.",
		"Model switch: not performed by this action.",
		"Configuration write: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("model-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MODEL_STATUS_INGEST_SECRET", "CHANNEL_MODEL_STATUS_COMMAND_SECRET", "model-status-secret-885"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("model-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Model Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels model`",
		"channel_model_status_status: `queued`",
		"model_snapshot_mode: `provider-facing-runtime-status`",
		"notification_target_issue: `#885`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"model_provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"fallback_models: `openai/gpt-4.1-nano`",
		"fallback_model_count: `1`",
		"endpoint_host: `models.github.ai`",
		"run_mode: `read-only`",
		"model_call_performed: `false`",
		"model_switch_performed: `false`",
		"model_config_write_performed: `false`",
		"fallback_config_write_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_model_status_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_model_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel model status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MODEL_STATUS_INGEST_SECRET", "CHANNEL_MODEL_STATUS_COMMAND_SECRET", "chat-model-123", "model-inbound-885", "model-notify-885", "model-status-secret-885"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel model status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 885,
			"title": "GitClaw telegram thread chat-model-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-model-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88502,
			"body": "@gitclaw /channels model-status --message-id model-inbound-885 --notify-message-id model-notify-885 --status-id model-status-secret-885\nDo not leak duplicate token CHANNEL_MODEL_STATUS_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate model status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[885]); got != 4 {
		t.Fatalf("duplicate model status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[885])
	}
	duplicateReceipt := github.CommentsByIssue[885][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels model-status`",
		"channel_model_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"model_switch_performed: `false`",
		"model_config_write_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate model status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MODEL_STATUS_DUPLICATE_SECRET", "chat-model-123", "model-inbound-885", "model-notify-885", "model-status-secret-885"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate model status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelModelStatusActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel model"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel runtime-model --route team-demo --message-id source-1 --notify-message-id notify-1 --status-id runtime-1`,
		},
	}
	req, err := BuildChannelModelStatusActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelModelStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "runtime-model" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "runtime-1" {
		t.Fatalf("unexpected channel model status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.ProviderHash == "" || req.ModelHash == "" || req.EndpointHostHash == "" {
		t.Fatalf("expected explicit route model-status hashes: %#v", req)
	}
}
