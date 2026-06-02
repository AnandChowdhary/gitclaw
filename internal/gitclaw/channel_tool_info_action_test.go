package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolInfoQueuesFocusedCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelToolInfoFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-info-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 907,
			"title": "GitClaw telegram thread chat-tool-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90701,
			"body": "@gitclaw /channels tool-info read_file --message-id tool-info-inbound-907 --notify-message-id tool-info-notify-907 --tool-info-id Tool.Info.Secret.907\nDo not include this command hidden token in the receipt: CHANNEL_TOOL_INFO_COMMAND_MARKER.",
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
			Number: 907,
			Title:  "GitClaw telegram thread chat-tool-info-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{907: {{
			ID: 90700,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-info-123",
				MessageID: "tool-info-inbound-907",
				Author:    "telegram",
				Body:      "Original mirrored tool info command with CHANNEL_TOOL_INFO_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{907: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool info action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("tool info action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[907]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="tool-info-notify-907"`,
		"GitClaw channel tool info",
		"Tool info status: ok",
		"Requested tool hash: ",
		"Normalized tool hash: ",
		"Available tools: 5",
		"Matched tools: 1",
		"Active outputs for tool: 0",
		"Validation status: ok",
		"Validation errors: 0",
		"Validation warnings: 0",
		"Tool info id hash: ",
		"Contracts:",
		"name=gitclaw.read_file",
		"source=builtin-gitclaw",
		"enabled=true",
		"disabled_by_config=false",
		"blocked_by_allowlist=false",
		"mode=read-only",
		"mutating=false",
		"trigger_sha256_12=",
		"Active outputs:",
		"- none",
		"Raw tool triggers, tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, and raw requested tool text are not included.",
		"Tool execution: not performed by this action.",
		"Shell execution: not performed by this action.",
		"MCP server launch: not performed by this action.",
		"Toolset activation: not performed by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool info notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_INFO_INGEST_MARKER", "CHANNEL_TOOL_INFO_COMMAND_MARKER", "CHANNEL_TOOL_INFO_FILE_SECRET", "Tool.Info.Secret.907", "explicit repository-relative path"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("tool info notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Info Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tool-info`",
		"channel_tool_info_status: `queued`",
		"tool_info_status: `ok`",
		"info_mode: `deterministic-tool-contract-card`",
		"notification_target_issue: `#907`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_info_id_sha256_12: `",
		"tool_info_id_auto: `false`",
		"requested_tool_sha256_12: `",
		"normalized_tool_sha256_12: `",
		"requested_tool_bytes: `9`",
		"tool_source: `positional`",
		"available_tools: `5`",
		"matched_tools: `1`",
		"active_outputs_for_tool: `0`",
		"validation_status: `ok`",
		"validation_errors: `0`",
		"validation_warnings: `0`",
		"matched_tool_names_sha256_12: `",
		"tool_info_index_sha256_12: `",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"tool_execution_allowed: `false`",
		"tool_execution_performed: `false`",
		"shell_execution_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_allowed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_tool_name_included: `false`",
		"raw_tool_trigger_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_tool_info_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"llm_e2e_required_after_channel_tool_info_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool info receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"read_file", "gitclaw.read_file", "explicit repository-relative path", "CHANNEL_TOOL_INFO_INGEST_MARKER", "CHANNEL_TOOL_INFO_COMMAND_MARKER", "CHANNEL_TOOL_INFO_FILE_SECRET", "chat-tool-info-123", "tool-info-inbound-907", "tool-info-notify-907", "Tool.Info.Secret.907"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool info receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 907,
			"title": "GitClaw telegram thread chat-tool-info-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-info-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90702,
			"body": "@gitclaw /channels describe-tool read_file --message-id tool-info-inbound-907 --notify-message-id tool-info-notify-907 --tool-info-id Tool.Info.Secret.907\nDo not include duplicate hidden token CHANNEL_TOOL_INFO_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[907]); got != 4 {
		t.Fatalf("duplicate tool info posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[907])
	}
	duplicateReceipt := github.CommentsByIssue[907][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels describe-tool`",
		"channel_tool_info_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"tool_execution_allowed: `false`",
		"tool_execution_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool info receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"read_file", "gitclaw.read_file", "CHANNEL_TOOL_INFO_DUPLICATE_MARKER", "chat-tool-info-123", "tool-info-inbound-907", "tool-info-notify-907", "Tool.Info.Secret.907"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate tool info receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelToolInfoActionRequestParsesRouteAliasAndTrailingTool(t *testing.T) {
	root := t.TempDir()
	writeChannelToolInfoFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel tool info"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel capability-describe --route team-demo --message-id source-1 --notify-message-id notify-1 --id Tool.Info.One
Tool: read_file`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel capability-describe"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelToolInfoActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolInfoActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "capability-describe" || req.Options.Route != "team-demo" || req.Options.RequestedTool != "read_file" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.InfoID != "tool-info-one" {
		t.Fatalf("unexpected channel tool info parsing: %#v", req)
	}
	if req.ToolSource != "trailing-tool" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoInfoID {
		t.Fatalf("unexpected channel tool info defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.InfoIDHash == "" || req.RequestedToolHash == "" || req.NormalizedToolHash == "" || req.NotificationBodySHA == "" || req.Info.MatchedTools != 1 {
		t.Fatalf("expected route info hashes and match: %#v", req)
	}
	if !IsChannelToolInfoActionRequest(ev, cfg) {
		t.Fatalf("expected channel capability-describe alias to be recognized")
	}
}

func writeChannelToolInfoFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "README.md", "Channel tool info fixture with CHANNEL_TOOL_INFO_FILE_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Reviewed deterministic tool guidance for channel tool info tests.\n")
}
