package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolSpotlightQueuesDeterministicCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelToolSpotlightFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-spotlight-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 909,
			"title": "GitClaw telegram thread chat-tool-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90901,
			"body": "@gitclaw /channels tool-spotlight search_files --message-id tool-spotlight-inbound-909 --notify-message-id tool-spotlight-notify-909 --spotlight-id Tool.Spotlight.Secret.909\nDo not include this command hidden token in the receipt: CHANNEL_TOOL_SPOTLIGHT_COMMAND_MARKER.",
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
			Number: 909,
			Title:  "GitClaw telegram thread chat-tool-spotlight-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{909: {{
			ID: 90900,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-spotlight-123",
				MessageID: "tool-spotlight-inbound-909",
				Author:    "telegram",
				Body:      "Original mirrored tool spotlight command with CHANNEL_TOOL_SPOTLIGHT_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{909: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool spotlight action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("tool spotlight action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[909]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="tool-spotlight-notify-909"`,
		"GitClaw channel tool spotlight",
		"Spotlight status: ok",
		"Focus hash: ",
		"Focus terms: 1",
		"Available tools: 5",
		"Enabled tools: ",
		"Eligible tools: ",
		"Matched tools: 1",
		"Candidate tools: 1",
		"Active tool outputs: ",
		"Selected index: 0",
		"Selection seed hash: ",
		"Selection hash: ",
		"Validation status: ok",
		"Validation errors: 0",
		"Validation warnings: 0",
		"Tool spotlight id hash: ",
		"Spotlight:",
		"tool_name=gitclaw.search_files",
		"mode=read-only",
		"enabled=true",
		"disabled_by_config=false",
		"blocked_by_allowlist=false",
		"mutating=false",
		"trigger_sha256_12=",
		"Try next:",
		"@gitclaw /channels tool-info gitclaw.search_files",
		"@gitclaw /channels tool-map gitclaw.search_files",
		"Raw tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, raw focus text, raw notes, raw tool triggers, and raw spotlight ids are not included in the source receipt.",
		"Tool execution: not performed by this action.",
		"Shell execution: not performed by this action.",
		"MCP server launch: not performed by this action.",
		"Toolset activation: not performed by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool spotlight notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_SPOTLIGHT_INGEST_MARKER", "CHANNEL_TOOL_SPOTLIGHT_COMMAND_MARKER", "CHANNEL_TOOL_SPOTLIGHT_FILE_SECRET", "Tool.Spotlight.Secret.909", "explicit quoted phrase or identifier"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("tool spotlight notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Spotlight Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tool-spotlight`",
		"channel_tool_spotlight_status: `queued`",
		"tool_spotlight_status: `ok`",
		"spotlight_mode: `deterministic-tool-contract-draw`",
		"notification_target_issue: `#909`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_spotlight_id_sha256_12: `",
		"tool_spotlight_id_auto: `false`",
		"spotlight_focus_sha256_12: `",
		"spotlight_focus_bytes: `12`",
		"spotlight_focus_terms: `1`",
		"spotlight_focus_source: `positional`",
		"spotlight_note_sha256_12: `",
		"spotlight_note_bytes: `0`",
		"available_tools: `5`",
		"enabled_tools: `",
		"eligible_tools: `",
		"matched_tools: `1`",
		"candidate_tools: `1`",
		"active_tool_outputs: `",
		"selected_index: `0`",
		"selected_tool_name_sha256_12: `",
		"selected_tool_mode_sha256_12: `",
		"selected_tool_trigger_sha256_12: `",
		"selected_tool_enabled: `true`",
		"selected_tool_disabled_by_config: `false`",
		"selected_tool_blocked_by_allowlist: `false`",
		"selection_seed_sha256_12: `",
		"selection_sha256_12: `",
		"validation_status: `ok`",
		"validation_errors: `0`",
		"validation_warnings: `0`",
		"notification_body_sha256_12: `",
		"deterministic_selection: `true`",
		"external_randomness_used: `false`",
		"tool_execution_allowed: `false`",
		"tool_execution_performed: `false`",
		"shell_execution_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_allowed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_focus_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_tool_spotlight_id_included: `false`",
		"raw_selection_seed_included: `false`",
		"raw_tool_names_included: `false`",
		"raw_tool_triggers_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"llm_e2e_required_after_channel_tool_spotlight_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool spotlight receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"search_files", "gitclaw.search_files", "explicit quoted phrase or identifier", "CHANNEL_TOOL_SPOTLIGHT_INGEST_MARKER", "CHANNEL_TOOL_SPOTLIGHT_COMMAND_MARKER", "CHANNEL_TOOL_SPOTLIGHT_FILE_SECRET", "chat-tool-spotlight-123", "tool-spotlight-inbound-909", "tool-spotlight-notify-909", "Tool.Spotlight.Secret.909"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool spotlight receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 909,
			"title": "GitClaw telegram thread chat-tool-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90902,
			"body": "@gitclaw /channels tool-draw search_files --message-id tool-spotlight-inbound-909 --notify-message-id tool-spotlight-notify-909 --spotlight-id Tool.Spotlight.Secret.909\nDo not include duplicate hidden token CHANNEL_TOOL_SPOTLIGHT_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[909]); got != 4 {
		t.Fatalf("duplicate tool spotlight posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[909])
	}
	duplicateReceipt := github.CommentsByIssue[909][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels tool-draw`",
		"channel_tool_spotlight_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"tool_execution_allowed: `false`",
		"tool_execution_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool spotlight receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"search_files", "gitclaw.search_files", "CHANNEL_TOOL_SPOTLIGHT_DUPLICATE_MARKER", "chat-tool-spotlight-123", "tool-spotlight-inbound-909", "tool-spotlight-notify-909", "Tool.Spotlight.Secret.909"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate tool spotlight receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelToolSpotlightActionRequestParsesRouteAliasAndTrailingNote(t *testing.T) {
	root := t.TempDir()
	writeChannelToolSpotlightFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel tool spotlight"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel tool-capability-draw --route team-demo --message-id source-1 --notify-message-id notify-1 --id Tool.Spotlight.One --focus search_files
Note: try safe search.`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel tool-capability-draw"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelToolSpotlightActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolSpotlightActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-capability-draw" || req.Options.Route != "team-demo" || req.Options.Focus != "search_files" || req.Options.Note != "try safe search." || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SpotlightID != "tool-spotlight-one" {
		t.Fatalf("unexpected channel tool spotlight parsing: %#v", req)
	}
	if req.FocusSource != "flag" || req.NoteSource != "trailing-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSpotlightID {
		t.Fatalf("unexpected channel tool spotlight defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SpotlightIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" || req.Report.CandidateTools != 1 || req.Report.SelectedIndex != 0 {
		t.Fatalf("expected route spotlight hashes and selected tool: %#v", req)
	}
	if !IsChannelToolSpotlightActionRequest(ev, cfg) {
		t.Fatalf("expected channel tool-capability-draw alias to be recognized")
	}
}

func writeChannelToolSpotlightFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "README.md", "Channel tool spotlight fixture with CHANNEL_TOOL_SPOTLIGHT_FILE_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Reviewed deterministic tool guidance for channel tool spotlight tests.\n")
}
