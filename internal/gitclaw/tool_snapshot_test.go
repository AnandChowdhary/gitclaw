package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeToolSnapshotFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_SNAPSHOT_BODY_TOKEN: use read-only metadata.")
	writeTestFile(t, root, ".gitclaw/toolsets/repo-read.yaml", `name: repo-read
description: Read-only repository context tools.
mode: read-only
tools:
  - gitclaw.list_files
  - gitclaw.search_files
instruction: |
  TOOLSET_SNAPSHOT_INSTRUCTION_TOKEN should stay hidden.
`)
	writeTestFile(t, root, ".gitclaw/mcp/github-read.yaml", `name: github-read
description: Metadata-only GitHub reads.
transport: stdio
source: github-mcp-read
activation: metadata-only
tool_allowlist:
  - issues.read
tool_denylist:
  - issues.write
resources_enabled: false
prompts_enabled: false
`)
}

func TestRenderToolSnapshotReportFingerprintsToolSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeToolSnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext := RepoContext{
		Documents: []ContextDocument{
			{Path: ".gitclaw/TOOLS.md", Body: "TOOLS_SNAPSHOT_BODY_TOKEN: use read-only metadata."},
		},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.search_files", Input: "TOOL_SNAPSHOT_INPUT_TOKEN", Output: "TOOL_SNAPSHOT_OUTPUT_TOKEN"},
		},
	}
	body := RenderToolSnapshotCLIReport(cfg, repoContext)
	for _, want := range []string{
		"GitClaw Tools Snapshot Report",
		"scope: `local-cli`",
		"tool_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-tool-snapshot-v1`",
		"snapshot_scope: `deterministic-tools-toolsets-mcp-outputs`",
		"snapshot_sha256_12:",
		"snapshot_entries: `9`",
		"catalog_entries: `7`",
		"builtin_contract_entries: `5`",
		"toolset_profile_entries: `1`",
		"mcp_tool_entries: `1`",
		"guidance_entries: `1`",
		"active_output_entries: `1`",
		"prompt_visible_entries: `7`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"active_tool_outputs: `1`",
		"known_tool_outputs: `1`",
		"unknown_tool_outputs: `0`",
		"toolsets_scanned: `1`",
		"mcp_specs_scanned: `1`",
		"registry_contact_allowed: `false`",
		"dynamic_mcp_discovery_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_supported: `false`",
		"model_callable_structured_tools: `false`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_toolset_bodies_included: `false`",
		"raw_toolset_instructions_included: `false`",
		"raw_mcp_bodies_included: `false`",
		"raw_mcp_command_args_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_tool_inputs_included: `false`",
		"llm_e2e_required_after_tool_snapshot_change: `true`",
		"tool_validation_status: `ok`",
		"tool_risk_status: `ok`",
		"### Snapshot Entries",
		"kind=`builtin-contract` name=`gitclaw.search_files`",
		"kind=`toolset-profile` name=`repo-read`",
		"kind=`mcp-tool` name=`github-read/issues.read`",
		"kind=`guidance` name=`.gitclaw/TOOLS.md`",
		"kind=`active-output` name=`gitclaw.search_files`",
		"prompt_visible=`true`",
		"sha256_12=",
		"input_sha256_12=",
		"output_sha256_12=",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Snapshot Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"registry_gate=`disabled`",
		"dynamic_mcp_discovery_gate=`disabled`",
		"mcp_runtime_gate=`disabled`",
		"toolset_activation_gate=`disabled`",
		"structured_tool_gate=`disabled`",
		"shell_execution_gate=`disabled`",
		"mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools snapshot report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOLS_SNAPSHOT_BODY_TOKEN", "TOOLSET_SNAPSHOT_INSTRUCTION_TOKEN", "TOOL_SNAPSHOT_INPUT_TOKEN", "TOOL_SNAPSHOT_OUTPUT_TOKEN"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools snapshot report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderToolsReportRoutesSnapshotWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeToolSnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "tools snapshot"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 162,
			Title:  "@gitclaw /tools snapshot",
			Body:   "Hidden tools snapshot issue token: TOOL_SNAPSHOT_ROUTE_BODY_SECRET.",
		},
	}
	body := RenderToolsReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Tools Snapshot Report",
		"repository: `owner/repo`",
		"issue: `#162`",
		"tool_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-tool-snapshot-v1`",
		"snapshot_sha256_12:",
		"issue_title_sha256_12:",
		"kind=`toolset-profile` name=`repo-read`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_tool_snapshot_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools snapshot route report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOLS_SNAPSHOT_BODY_TOKEN", "TOOLSET_SNAPSHOT_INSTRUCTION_TOKEN", "TOOL_SNAPSHOT_ROUTE_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools snapshot route report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestToolsSnapshotCommandReportsCompositeFingerprintWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeToolSnapshotFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "snapshot"}); err != nil {
			t.Fatalf("tools snapshot returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tools Snapshot Report", "scope: `local-cli`", "tool_snapshot_status: `ok`", "snapshot_version: `gitclaw-tool-snapshot-v1`", "snapshot_entries:", "catalog_entries:", "builtin_contract_entries: `5`", "toolset_profile_entries: `1`", "mcp_tool_entries: `1`", "guidance_entries: `1`", "raw_tool_outputs_included: `false`", "llm_e2e_required_after_tool_snapshot_change: `true`", "### Snapshot Entries", "kind=`builtin-contract` name=`gitclaw.list_files`", "kind=`toolset-profile` name=`repo-read`", "kind=`mcp-tool` name=`github-read/issues.read`", "### Snapshot Gates", "snapshot_hash_gate=`composite-sha256_12`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools snapshot output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"TOOLS_SNAPSHOT_BODY_TOKEN", "TOOLSET_SNAPSHOT_INSTRUCTION_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("tools snapshot leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleToolsSnapshotCommandPostsCompositeFingerprintWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeToolSnapshotFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 163,
			"title": "@gitclaw /tools snapshot",
			"body": "Hidden tools snapshot body token: TOOL_SNAPSHOT_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{163: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tools snapshot command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Tools Snapshot Report", "Generated without a model call", "model=\"gitclaw/tools\"", "repository: `owner/repo`", "issue: `#163`", "tool_snapshot_status: `ok`", "snapshot_version: `gitclaw-tool-snapshot-v1`", "snapshot_sha256_12:", "builtin_contract_entries: `5`", "toolset_profile_entries: `1`", "mcp_tool_entries: `1`", "guidance_entries: `1`", "raw_tool_outputs_included: `false`", "llm_e2e_required_after_tool_snapshot_change: `true`", "issue_title_sha256_12:", "### Snapshot Entries", "kind=`toolset-profile` name=`repo-read`", "kind=`mcp-tool` name=`github-read/issues.read`", "### Snapshot Gates", "validation_gate=`pass`", "risk_gate=`pass`", "snapshot_hash_gate=`composite-sha256_12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools snapshot handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOLS_SNAPSHOT_BODY_TOKEN", "TOOLSET_SNAPSHOT_INSTRUCTION_TOKEN", "TOOL_SNAPSHOT_HANDLER_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools snapshot handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[163], "gitclaw:done") || hasLabel(github.IssueLabels[163], "gitclaw:running") || hasLabel(github.IssueLabels[163], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[163])
	}
}
