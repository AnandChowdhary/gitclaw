package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelToolStatusQueuesSnapshotWithoutLLM(t *testing.T) {
	root := t.TempDir()
	gitclawDir := filepath.Join(root, ".gitclaw")
	if err := os.MkdirAll(gitclawDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitclawDir, "TOOLS.md"), []byte("Tool guidance secret CHANNEL_TOOL_STATUS_GUIDANCE_SECRET.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello tools\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 887,
			"title": "GitClaw telegram thread chat-tool-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88701,
			"body": "@gitclaw /channels tools --message-id tool-status-inbound-887 --notify-message-id tool-status-notify-887 --status-id tool-status-secret-887\nDo not include this command hidden token in the receipt: CHANNEL_TOOL_STATUS_COMMAND_SECRET.",
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
			Number: 887,
			Title:  "GitClaw telegram thread chat-tool-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{887: {{
			ID: 88700,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-status-123",
				MessageID: "tool-status-inbound-887",
				Author:    "telegram",
				Body:      "Original mirrored tools command with CHANNEL_TOOL_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{887: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("tool status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[887]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="tool-status-notify-887"`,
		"GitClaw channel tool status.",
		"Available tools: 5",
		"Enabled tools: 5",
		"Disabled tools: 0",
		"Allowlist blocked tools: 0",
		"Read-only contracts: 3",
		"Metadata-only contracts: 2",
		"Mutating contracts: 0",
		"Enabled tool names: gitclaw.list_files, gitclaw.policy, gitclaw.read_file, gitclaw.search_files, gitclaw.skill_index",
		"Toolsets scanned: 0",
		"MCP specs scanned: 0",
		"Prompt-visible entries: 7",
		"Active tool outputs: 1",
		"Known tool outputs: 1",
		"Unknown tool outputs: 0",
		"Validation status: ok",
		"Risk status: ok",
		"Progressive disclosure: enabled",
		"Snapshot source: current GitHub Actions checkout",
		"Raw tool schemas: not included.",
		"Raw tool inputs: not included.",
		"Raw tool outputs: not included.",
		"Tool execution: not performed by this action.",
		"Shell execution: not performed by this action.",
		"MCP server launch: not performed by this action.",
		"Toolset activation: not performed by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_STATUS_INGEST_SECRET", "CHANNEL_TOOL_STATUS_COMMAND_SECRET", "CHANNEL_TOOL_STATUS_GUIDANCE_SECRET", "tool-status-secret-887"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("tool-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tools`",
		"channel_tool_status_status: `queued`",
		"tool_snapshot_mode: `provider-facing-tool-status`",
		"notification_target_issue: `#887`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"read_only_contracts: `3`",
		"metadata_only_contracts: `2`",
		"mutating_contracts: `0`",
		"active_tool_outputs: `1`",
		"known_tool_outputs: `1`",
		"unknown_tool_outputs: `0`",
		"toolsets_scanned: `0`",
		"mcp_specs_scanned: `0`",
		"snapshot_entries: `7`",
		"catalog_entries: `5`",
		"prompt_visible_entries: `7`",
		"tool_guidance_files: `1`",
		"tool_validation_status: `ok`",
		"tool_validation_errors: `0`",
		"tool_validation_warnings: `0`",
		"tool_risk_status: `ok`",
		"tool_risk_findings: `0`",
		"progressive_disclosure_enabled: `true`",
		"tool_execution_performed: `false`",
		"shell_execution_performed: `false`",
		"mcp_server_launch_performed: `false`",
		"toolset_activation_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_tool_status_id_included: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_toolset_instructions_included: `false`",
		"raw_mcp_command_args_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_tool_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_STATUS_INGEST_SECRET", "CHANNEL_TOOL_STATUS_COMMAND_SECRET", "CHANNEL_TOOL_STATUS_GUIDANCE_SECRET", "chat-tool-status-123", "tool-status-inbound-887", "tool-status-notify-887", "tool-status-secret-887"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 887,
			"title": "GitClaw telegram thread chat-tool-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88702,
			"body": "@gitclaw /channels tool-status --message-id tool-status-inbound-887 --notify-message-id tool-status-notify-887 --status-id tool-status-secret-887\nDo not leak duplicate token CHANNEL_TOOL_STATUS_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate tool status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[887]); got != 4 {
		t.Fatalf("duplicate tool status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[887])
	}
	duplicateReceipt := github.CommentsByIssue[887][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels tool-status`",
		"channel_tool_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"tool_execution_performed: `false`",
		"shell_execution_performed: `false`",
		"mcp_server_launch_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_STATUS_DUPLICATE_SECRET", "chat-tool-status-123", "tool-status-inbound-887", "tool-status-notify-887", "tool-status-secret-887"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate tool status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelToolStatusActionRequestParsesRouteAlias(t *testing.T) {
	repoContext := RepoContext{}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel tools"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel tool-capabilities --route team-demo --message-id source-1 --notify-message-id notify-1 --status-id tools-1`,
		},
	}
	req, err := BuildChannelToolStatusActionRequest(ev, DefaultConfig(), repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-capabilities" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "tools-1" {
		t.Fatalf("unexpected channel tool status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.EnabledToolNamesHash == "" || req.PromptVisibleToolHash == "" || req.ToolSnapshotHash == "" {
		t.Fatalf("expected explicit route tool-status hashes: %#v", req)
	}
}
