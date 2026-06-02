package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelAvailabilityQueuesPresenceCardWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-availability-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 918,
			"title": "GitClaw telegram thread chat-availability-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-availability-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91801,
			"body": "@gitclaw /channels availability --message-id availability-inbound-918 --notify-message-id availability-notify-918 --availability-id Availability.Secret.918 --state available\nDo not include this command hidden token in the receipt: CHANNEL_AVAILABILITY_COMMAND_SECRET.",
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
			Number: 918,
			Title:  "GitClaw telegram thread chat-availability-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{918: {{
			ID: 91800,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-availability-123",
				MessageID: "availability-inbound-918",
				Author:    "telegram",
				Body:      "Original mirrored availability command with CHANNEL_AVAILABILITY_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{918: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel availability action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("availability action should not create artifact issues: %#v", github.Issues)
	}
	sourceComments := github.CommentsByIssue[918]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}

	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="availability-notify-918"`,
		"GitClaw channel availability",
		"State: available",
		"Bridge runtime: GitHub Actions workflow_dispatch",
		"Canonical surface: GitHub issue thread",
		"Inbound path: channel-ingest workflow",
		"Outbound path: channel-outbox + channel-delivery",
		"Provider socket health: not probed by this action",
		"Session rows used as liveness: false",
		"Provider API call: not performed by this action",
		"Model call: not performed by this action",
		"Repository mutation: not performed by this action",
		"Workflow mutation: not performed by this action",
		"Safe follow-up commands:",
		"@gitclaw /channels status",
		"@gitclaw /channels model --message-id <id>",
		"@gitclaw /channels tools --message-id <id>",
		"@gitclaw /channels session-search <query> --message-id <id>",
		"@gitclaw /channels reminder --message-id <id>",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("availability notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_AVAILABILITY_COMMAND_SECRET", "CHANNEL_AVAILABILITY_INGEST_SECRET", "Availability.Secret.918"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("availability notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Availability Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels availability`",
		"channel_availability_status: `queued`",
		"availability_snapshot_mode: `provider-facing-presence-card`",
		"availability_state: `available`",
		"availability_state_sha256_12: `",
		"notification_target_issue: `#918`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"availability_id_sha256_12: `",
		"availability_id_auto: `false`",
		"bridge_runtime: `github-actions-workflow-dispatch`",
		"canonical_surface: `github-issue-thread`",
		"inbound_strategy: `channel-ingest workflow_dispatch`",
		"outbound_strategy: `channel-outbox + channel-delivery`",
		"session_rows_used_as_liveness: `false`",
		"provider_socket_probe_performed: `false`",
		"provider_api_call_performed: `false`",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"workflow_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_availability_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_channel_availability_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("availability receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_AVAILABILITY_COMMAND_SECRET", "CHANNEL_AVAILABILITY_INGEST_SECRET", "chat-availability-123", "availability-inbound-918", "availability-notify-918", "Availability.Secret.918"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("availability receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 918,
			"title": "GitClaw telegram thread chat-availability-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-availability-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91802,
			"body": "@gitclaw /channels online --message-id availability-inbound-918 --notify-message-id availability-notify-918 --availability-id Availability.Secret.918 --state available\nDo not leak duplicate token CHANNEL_AVAILABILITY_DUPLICATE_SECRET.",
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
	if got := len(github.CommentsByIssue[918]); got != 4 {
		t.Fatalf("duplicate availability posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[918])
	}
	duplicateReceipt := github.CommentsByIssue[918][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels online`",
		"channel_availability_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"workflow_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate availability receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_AVAILABILITY_DUPLICATE_SECRET", "chat-availability-123", "availability-inbound-918", "availability-notify-918", "Availability.Secret.918"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate availability receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelAvailabilityActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel availability"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel beacon --route team-demo --message-id source-1 --notify-message-id notify-1 --availability-id Availability.One --state Away.Now`,
		},
	}
	req, err := BuildChannelAvailabilityActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelAvailabilityActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "beacon" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.AvailabilityID != "availability-one" || req.Options.State != "away-now" {
		t.Fatalf("unexpected channel availability parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoAvailabilityID || req.RequestedRouteHash == "" || req.AvailabilityIDHash == "" || req.StateHash == "" || req.NotificationBodySHA == "" || req.NotificationBytes == 0 || req.NotificationLines == 0 {
		t.Fatalf("expected explicit route availability hashes: %#v", req)
	}
	if !IsChannelAvailabilityActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel beacon alias to be recognized")
	}
}
