package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelModeQueuesModeWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-mode-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-mode-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mode-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels mode tool-review --message-id mode-inbound-901 --notify-message-id mode-notify-901 --mode-id mode-secret-901\nNote: Use this tiny launcher.\nDo not include this command hidden token in the receipt: CHANNEL_MODE_COMMAND_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 901,
			Title:  "GitClaw telegram thread chat-mode-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-mode-123",
				MessageID: "mode-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored mode command with CHANNEL_MODE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel mode action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("mode action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="mode-notify-901"`,
		"GitClaw channel mode.",
		"Mode: tool-review",
		"Posture: Review tool context, approvals, and run plans before any tool execution.",
		"Suggested next steps:",
		"/channels tools --message-id <id>",
		"/channels tool-search <query> --message-id <id> --notify-message-id <id>",
		"/channels tool-info <tool> --message-id <id> --notify-message-id <id>",
		"/channels approval-plan <tool> --id <id> --message-id <id>",
		"Note: Use this tiny launcher.",
		"Mode hash: ",
		"Note hash: ",
		"Mode persistence: advisory only; no durable channel state changed.",
		"Mode source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Command execution: not performed by this action.",
		"Skill install: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Backup payload read: not performed by this action.",
		"Soul body read: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Policy mutation: not performed by this action.",
		"Schedule creation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("mode notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MODE_INGEST_MARKER", "CHANNEL_MODE_COMMAND_MARKER", "mode-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("mode notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Mode Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels mode`",
		"channel_mode_status: `queued`",
		"mode_card_mode: `structured-channel-mode`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"mode_id_sha256_12: `",
		"mode_id_auto: `false`",
		"mode_name_sha256_12: `",
		"mode_name_bytes: `11`",
		"mode_step_count: `4`",
		"mode_note_sha256_12: `",
		"mode_note_bytes: `23`",
		"mode_note_lines: `1`",
		"mode_note_source: `trailing-note`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"mode_persistence_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_mode_id_included: `false`",
		"raw_mode_name_included: `false`",
		"raw_mode_note_included: `false`",
		"raw_mode_steps_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_mode_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel mode receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MODE_INGEST_MARKER", "CHANNEL_MODE_COMMAND_MARKER", "chat-mode-123", "mode-inbound-901", "mode-notify-901", "mode-secret-901", "tool-review", "Use this tiny launcher.", "/channels tools --message-id"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel mode receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-mode-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-mode-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels posture tool-review --message-id mode-inbound-901 --notify-message-id mode-notify-901 --mode-id mode-secret-901\nNote: Use this tiny launcher.\nDo not leak duplicate token CHANNEL_MODE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate mode created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate mode posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels posture`",
		"channel_mode_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"command_execution_performed: `false`",
		"skill_install_performed: `false`",
		"tool_execution_performed: `false`",
		"backup_payload_read: `false`",
		"soul_body_read: `false`",
		"mode_persistence_performed: `false`",
		"workflow_mutation_performed: `false`",
		"policy_mutation_performed: `false`",
		"schedule_created: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate mode receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MODE_DUPLICATE_MARKER", "chat-mode-123", "mode-inbound-901", "mode-notify-901", "mode-secret-901", "tool-review", "Use this tiny launcher.", "/channels tools --message-id"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate mode receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelModeActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel mode"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel stance --route team-demo --mode soul --message-id source-1 --notify-message-id notify-1 --mode-id Mode.One
Note: Almost there.`,
		},
	}
	req, err := BuildChannelModeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelModeActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "stance" || req.Options.Route != "team-demo" || req.Options.Focus != "soul-review" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ModeID != "mode-one" || req.StepCount != 4 {
		t.Fatalf("unexpected channel mode parsing: %#v", req)
	}
	if req.Options.Note != "Almost there." || req.NoteSource != "trailing-note" {
		t.Fatalf("unexpected channel mode note parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoModeID || req.RequestedRouteHash == "" || req.ModeIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route mode hashes: %#v", req)
	}
}

func TestBuildChannelModeActionRequestParsesPositionalRouteAndFocus(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels thread-mode team-demo backup --message-id source-2 --notify-message-id notify-2 --mode-id Mode.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelModeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelModeActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Focus != "backup-review" || req.StepCount != 4 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel mode parsing: %#v", req)
	}
}
