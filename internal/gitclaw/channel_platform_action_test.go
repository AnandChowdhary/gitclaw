package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPlatformQueuesStatusWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-platform-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 825,
			"title": "GitClaw telegram thread chat-platform-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-platform-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 82501,
			"body": "@gitclaw /channels platform telegram --state running --message-id platform-inbound-825 --notify-message-id platform-notify-825 --home private-home-NOECHO_CHANNEL_PLATFORM_HOME\nReason:\nOperator reason with CHANNEL_PLATFORM_REASON_SECRET.",
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
			Number: 825,
			Title:  "GitClaw telegram thread chat-platform-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{825: {{
			ID: 82500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-platform-123",
				MessageID: "platform-inbound-825",
				Author:    "telegram",
				Body:      "Original mirrored platform command with CHANNEL_PLATFORM_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{825: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel platform action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("platform should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[825]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="platform-notify-825"`,
		"GitClaw channel platform status.",
		"Provider: telegram",
		"Adapter state: running",
		"Gateway runtime: GitHub Actions workflow_dispatch",
		"State storage: gitclaw:channel-state issue",
		"Outbox: gitclaw channel-outbox + channel-delivery",
		"Live adapter inspection: not performed by this action.",
		"Pause/resume: not performed by this action.",
		"Home channel: not changed by this action.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("platform notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PLATFORM_REASON_SECRET", "CHANNEL_PLATFORM_INGEST_SECRET", "private-home-NOECHO_CHANNEL_PLATFORM_HOME"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("platform notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Platform Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels platform`",
		"channel_platform_status: `queued`",
		"platform_snapshot_mode: `provider-facing-status`",
		"notification_target_issue: `#825`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"platform_pause_performed: `false`",
		"platform_resume_performed: `false`",
		"adapter_state_mutation_performed: `false`",
		"breaker_mutation_performed: `false`",
		"home_channel_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_reason_included: `false`",
		"raw_home_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_platform_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel platform receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PLATFORM_INGEST_SECRET", "CHANNEL_PLATFORM_REASON_SECRET", "private-home-NOECHO_CHANNEL_PLATFORM_HOME", "chat-platform-123", "platform-inbound-825", "platform-notify-825"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel platform receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 825,
			"title": "GitClaw telegram thread chat-platform-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-platform-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 82502,
			"body": "@gitclaw /channels platform telegram --state running --message-id platform-inbound-825 --notify-message-id platform-notify-825\nReason:\nDo not leak duplicate token CHANNEL_PLATFORM_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate platform created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[825]); got != 4 {
		t.Fatalf("duplicate platform posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[825])
	}
	duplicateReceipt := github.CommentsByIssue[825][3].Body
	for _, want := range []string{
		"channel_platform_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"platform_pause_performed: `false`",
		"platform_resume_performed: `false`",
		"home_channel_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate platform receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PLATFORM_DUPLICATE_SECRET", "chat-platform-123", "platform-inbound-825", "platform-notify-825"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate platform receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelPlatformActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 32, Title: "Channel platform"},
		Comment: &Comment{
			ID: 3201,
			Body: `@gitclaw /channel adapter-status --route team-demo --provider tg --state paused-by-breaker --message-id source-1 --notify-message-id notify-1 --home team-home
Reason:
Circuit breaker cooled down after webhook noise.`,
		},
	}
	req, err := BuildChannelPlatformActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPlatformActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "adapter-status" || req.Options.Route != "team-demo" || req.Options.Provider != "telegram" || req.Options.AdapterState != "paused-by-breaker" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.Home != "team-home" {
		t.Fatalf("unexpected channel platform parsing: %#v", req)
	}
	if !strings.Contains(req.Options.Reason, "Circuit breaker") {
		t.Fatalf("unexpected platform reason: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.ProviderHash == "" || req.StateHash == "" || req.ReasonHash == "" || req.HomeHash == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route platform hashes: %#v", req)
	}
}
