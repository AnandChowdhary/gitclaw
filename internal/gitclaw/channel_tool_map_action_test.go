package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelToolMapQueuesSafeSequenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	gitclawDir := filepath.Join(root, ".gitclaw")
	if err := os.MkdirAll(gitclawDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitclawDir, "TOOLS.md"), []byte("Tool map guidance secret CHANNEL_TOOL_MAP_GUIDANCE_SECRET.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello tool maps\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-map-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-tool-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels tool-map search_files --message-id tool-map-inbound-901 --notify-message-id tool-map-notify-901 --map-id tool-map-secret-901\nNote: Keep tool execution reviewed\nDo not include this command hidden token in the receipt: CHANNEL_TOOL_MAP_COMMAND_SECRET.",
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
			Title:  "GitClaw telegram thread chat-tool-map-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-map-123",
				MessageID: "tool-map-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored tool-map command with CHANNEL_TOOL_MAP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool map action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("tool map should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="tool-map-notify-901"`,
		"GitClaw channel tool map.",
		"Requested tool: search_files",
		"Available tools: 5",
		"Enabled tools: 5",
		"Disabled tools: 0",
		"Allowlist blocked tools: 0",
		"Read-only contracts: 3",
		"Metadata-only contracts: 2",
		"Mutating contracts: 0",
		"Validation status: ok",
		"Risk status: ok",
		"Prompt-visible entries: 7",
		"Tool sequence:",
		"`/channels tools --message-id <id> --notify-message-id <id>`",
		"`/channels tool-search search_files --message-id <id> --notify-message-id <id>`",
		"`/channels tool-info search_files --message-id <id> --notify-message-id <id>`",
		"`/channels approval-plan search_files --id <approval-plan-id> --message-id <id> --notify-message-id <id>`",
		"`/channels rehearse-tool search_files --id <rehearsal-id> --message-id <id> --notify-message-id <id>`",
		"`/channels request-run search_files --id <request-id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep tool execution reviewed",
		"Note hash: ",
		"Tool map hash: ",
		"Tool step hash: ",
		"Map source: current GitHub Actions checkout tool metadata.",
		"Tool execution: not performed by this action.",
		"Shell execution: not performed by this action.",
		"MCP server launch: not performed by this action.",
		"Toolset activation: not performed by this action.",
		"Approval issue creation: not performed by this action.",
		"Rehearsal issue creation: not performed by this action.",
		"Tool-run request issue creation: not performed by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool-map notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_MAP_INGEST_SECRET", "CHANNEL_TOOL_MAP_COMMAND_SECRET", "CHANNEL_TOOL_MAP_GUIDANCE_SECRET", "tool-map-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("tool-map notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Map Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tool-map`",
		"channel_tool_map_status: `queued`",
		"tool_map_mode: `provider-facing-tool-safety-sequence`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_map_id_sha256_12: `",
		"tool_map_id_auto: `false`",
		"requested_tool_sha256_12: `",
		"requested_tool_bytes: `12`",
		"requested_tool_terms: `1`",
		"tool_map_note_sha256_12: `",
		"tool_map_note_bytes: `28`",
		"tool_map_note_lines: `1`",
		"tool_map_note_source: `trailing-note`",
		"tool_map_step_count: `6`",
		"tool_map_step_sha256_12: `",
		"tool_map_snapshot_sha256_12: `",
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
		"tool_risk_status: `ok`",
		"dynamic_mcp_discovery_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_supported: `false`",
		"model_callable_structured_tools: `false`",
		"tool_execution_performed: `false`",
		"shell_execution_performed: `false`",
		"mcp_server_launch_performed: `false`",
		"toolset_activation_performed: `false`",
		"approval_issue_created: `false`",
		"rehearsal_issue_created: `false`",
		"tool_run_request_issue_created: `false`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_tool_map_id_included: `false`",
		"raw_requested_tool_included: `false`",
		"raw_tool_map_note_included: `false`",
		"raw_tool_map_steps_included: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_tool_map_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool map receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_MAP_INGEST_SECRET", "CHANNEL_TOOL_MAP_COMMAND_SECRET", "CHANNEL_TOOL_MAP_GUIDANCE_SECRET", "chat-tool-map-123", "tool-map-inbound-901", "tool-map-notify-901", "tool-map-secret-901", "search_files", "Keep tool execution reviewed"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool map receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-tool-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels tool-path search_files --message-id tool-map-inbound-901 --notify-message-id tool-map-notify-901 --map-id tool-map-secret-901\nNote: Keep tool execution reviewed\nDo not leak duplicate token CHANNEL_TOOL_MAP_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate tool map created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate tool map posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels tool-path`",
		"channel_tool_map_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"tool_execution_performed: `false`",
		"approval_issue_created: `false`",
		"rehearsal_issue_created: `false`",
		"tool_run_request_issue_created: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool map receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_MAP_DUPLICATE_SECRET", "chat-tool-map-123", "tool-map-inbound-901", "tool-map-notify-901", "tool-map-secret-901", "search_files", "Keep tool execution reviewed"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate tool map receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelToolMapActionRequestParsesRouteAlias(t *testing.T) {
	repoContext := RepoContext{}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel tool map"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel tool-runbook --route team-demo --tool search_files --message-id source-1 --notify-message-id notify-1 --map-id map-1 --note reviewed-only`,
		},
	}
	req, err := BuildChannelToolMapActionRequest(ev, DefaultConfig(), repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolMapActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-runbook" || req.Options.Route != "team-demo" || req.Options.RequestedTool != "search_files" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MapID != "map-1" || req.Options.Note != "reviewed-only" {
		t.Fatalf("unexpected channel tool map parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMapID || req.RequestedRouteHash == "" || req.MapIDHash == "" || req.RequestedToolSHA == "" || req.StepSHA == "" || req.SnapshotSHA == "" {
		t.Fatalf("expected explicit route tool-map hashes: %#v", req)
	}
}
