package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderSandboxReportShowsExecutionBoundaryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSandboxTestWorkflow(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext := RepoContext{
		Documents: []ContextDocument{
			{Path: ".gitclaw/TOOLS.md", Body: "SANDBOX_TOOLS_SECRET"},
		},
		SkillSummaries: []SkillSummary{
			{Name: "repo-reader", Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Enabled: true, RequiredBins: []string{"rg"}, MissingBins: []string{"rg"}},
		},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: ".", Output: "README.md\nSANDBOX_TOOL_OUTPUT_SECRET"},
		},
		AllowedTools:  cfg.AllowedTools,
		DisabledTools: cfg.DisabledTools,
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 131,
			"title": "@gitclaw /sandbox",
			"body": "Hidden sandbox body token: SANDBOX_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderSandboxReport(ev, cfg, repoContext)
	for _, want := range []string{
		"GitClaw Sandbox Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#131`",
		"event_kind: `issue_opened`",
		"event_name: `issues`",
		"event_action: `opened`",
		"active_command: `/sandbox`",
		"sandbox_status: `locked_down`",
		"runtime_boundary: `github-actions-ephemeral-runner`",
		"sandbox_backend: `github-actions`",
		"host_exec_policy: `deny`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"write_mode: `read-only`",
		"approval_mode: `not_applicable_no_exec_tool`",
		"approval_store: `not_configured`",
		"elevated_mode_available: `false`",
		"skill_cli_auto_allow: `false`",
		"inline_eval_policy: `not_applicable_no_exec_tool`",
		"network_egress_policy: `github-actions-default`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"read_only_tool_contracts: `3`",
		"metadata_only_tool_contracts: `2`",
		"mutating_tool_contracts: `0`",
		"active_tool_outputs: `1`",
		"tool_validation_status: `ok`",
		"workflow_permission_status: `ok`",
		"workflow_present: `true`",
		"unexpected_write_permissions: `0`",
		"backup_write_permission_scope: `backup-job-only`",
		"skill_required_bins: `1`",
		"skill_missing_bins: `1`",
		"raw_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_workflow_included: `false`",
		"### Execution Boundary",
		"shell_tool=`absent`",
		"file_write_tool=`absent`",
		"pull_request_tool=`absent`",
		"### Tool Contracts",
		"name=`gitclaw.list_files` mode=`read-only` mutating=`false` enabled=`true`",
		"name=`gitclaw.policy` mode=`metadata-only` mutating=`false` enabled=`true`",
		"### Active Tool Outputs",
		"name=`gitclaw.list_files` input_sha256_12=`",
		"output_sha256_12=`",
		"### Workflow Permission Boundary",
		"job=`handle` present=`true`",
		"models:read",
		"job=`backup` present=`true`",
		"contents:write",
		"### Sandbox Notes",
		"host exec is denied because no shell/exec tool is exposed in GitClaw v1",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("sandbox report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"SANDBOX_BODY_SECRET",
		"SANDBOX_TOOLS_SECRET",
		"SANDBOX_TOOL_OUTPUT_SECRET",
		"Hidden sandbox body token",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("sandbox report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestIsSandboxReportRequestAcceptsAliases(t *testing.T) {
	for _, command := range []string{"/sandbox", "/sandboxes", "/exec-policy"} {
		ev, err := ParseEvent("issues", []byte(`{
			"action": "opened",
			"repository": {"full_name": "owner/repo", "default_branch": "main"},
			"issue": {
				"number": 132,
				"title": "@gitclaw `+command+`",
				"body": "",
				"author_association": "MEMBER",
				"user": {"login": "alice", "type": "User"},
				"labels": [{"name": "gitclaw"}]
			},
			"sender": {"login": "alice", "type": "User"}
		}`))
		if err != nil {
			t.Fatalf("ParseEvent(%s) returned error: %v", command, err)
		}
		if !IsSandboxReportRequest(ev, DefaultConfig()) {
			t.Fatalf("%s should be accepted as a sandbox report command", command)
		}
	}
}

func TestRenderSandboxRiskReportShowsBoundaryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSandboxTestWorkflow(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext := RepoContext{
		Documents: []ContextDocument{
			{Path: ".gitclaw/TOOLS.md", Body: "SANDBOX_RISK_TOOLS_SECRET"},
		},
		SkillSummaries: []SkillSummary{
			{Name: "repo-reader", Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Enabled: true, RequiredBins: []string{"rg"}},
		},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: ".", Output: "README.md\nSANDBOX_RISK_TOOL_OUTPUT_SECRET"},
		},
		AllowedTools:  cfg.AllowedTools,
		DisabledTools: cfg.DisabledTools,
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 133,
			"title": "@gitclaw /sandbox risk",
			"body": "Hidden sandbox risk body token: SANDBOX_RISK_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderSandboxRiskReport(ev, cfg, repoContext, true)
	for _, want := range []string{
		"GitClaw Sandbox Risk Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#133`",
		"event_kind: `issue_opened`",
		"event_name: `issues`",
		"event_action: `opened`",
		"active_command: `/sandbox`",
		"sandbox_risk_status: `ok`",
		"verification_scope: `github_actions_sandbox_boundary`",
		"runtime_boundary: `github-actions-ephemeral-runner`",
		"sandbox_backend: `github-actions`",
		"host_exec_policy: `deny`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"write_mode: `read-only`",
		"approval_mode: `not_applicable_no_exec_tool`",
		"approval_store: `not_configured`",
		"elevated_mode_available: `false`",
		"skill_cli_auto_allow: `false`",
		"inline_eval_policy: `not_applicable_no_exec_tool`",
		"network_egress_policy: `github-actions-default`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"read_only_tool_contracts: `3`",
		"metadata_only_tool_contracts: `2`",
		"mutating_tool_contracts: `0`",
		"active_tool_outputs: `1`",
		"tool_validation_status: `ok`",
		"tool_validation_errors: `0`",
		"tool_validation_warnings: `0`",
		"workflow_permission_status: `ok`",
		"workflow_present: `true`",
		"unexpected_write_permissions: `0`",
		"backup_write_permission_scope: `backup-job-only`",
		"backup_concurrency_group: `true`",
		"backup_concurrency_cancel_safe: `true`",
		"skill_required_bins: `1`",
		"skill_missing_bins: `0`",
		"surfaces_with_risk_findings: `0`",
		"sandbox_risk_findings: `0`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"secrets_included: `false`",
		"llm_e2e_required_after_sandbox_risk_change: `true`",
		"### Runtime Boundary Risk Card",
		"kind=`runtime-boundary` runtime_boundary=`github-actions-ephemeral-runner`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Tool Contract Risk Card",
		"kind=`tool-contract` available_tools=`5`",
		"tool_validation_status=`ok`",
		"### Workflow Permission Risk Card",
		"kind=`workflow-permission` workflow_present=`true`",
		"backup_concurrency_cancel_safe=`true`",
		"### Skill Runtime Risk Card",
		"kind=`skill-runtime` skill_cli_auto_allow=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("sandbox risk report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"SANDBOX_RISK_BODY_SECRET",
		"SANDBOX_RISK_TOOLS_SECRET",
		"SANDBOX_RISK_TOOL_OUTPUT_SECRET",
		"Hidden sandbox risk body token",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("sandbox risk report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSandboxReportRoutesRiskCommand(t *testing.T) {
	root := t.TempDir()
	writeSandboxTestWorkflow(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext := RepoContext{
		Documents:     []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "SANDBOX_ROUTE_TOOLS_SECRET"}},
		AllowedTools:  cfg.AllowedTools,
		DisabledTools: cfg.DisabledTools,
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 134,
			"title": "@gitclaw /sandbox risk",
			"body": "Hidden sandbox route body token: SANDBOX_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderSandboxReport(ev, cfg, repoContext)
	for _, want := range []string{"GitClaw Sandbox Risk Report", "sandbox_risk_status: `ok`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("sandbox risk route missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "SANDBOX_ROUTE_BODY_SECRET") || strings.Contains(report, "SANDBOX_ROUTE_TOOLS_SECRET") {
		t.Fatalf("sandbox risk route leaked body:\n%s", report)
	}
}

func writeSandboxTestWorkflow(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", `name: GitClaw
jobs:
  preflight:
    permissions:
      contents: read
      issues: read
  handle:
    permissions:
      contents: read
      issues: write
      models: read
  backup:
    concurrency:
      group: gitclaw-backups-owner/repo
      cancel-in-progress: false
    permissions:
      contents: write
      issues: read
`)
}
