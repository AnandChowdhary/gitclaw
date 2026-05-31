package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const safeMCPSpecTestBody = `name: github-read
description: MCP_DESCRIPTION_SECRET should not appear in reports.
transport: stdio
source: repo-reviewed-mcp
activation: metadata-only
tool_allowlist:
  - issues.read
  - contents.read
  - pull_requests.read
tool_denylist:
  - contents.write
  - actions.write
requires_secrets:
  - GITHUB_TOKEN
resources_enabled: false
prompts_enabled: false
`

func writeMCPProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/mcp/github-read.yaml", `name: github-read
description: MCP_PROVENANCE_DESCRIPTION_SECRET
transport: stdio
source: MCP_PROVENANCE_SOURCE_SECRET
activation: metadata-only
tool_allowlist:
  - issues.read
  - contents.read
  - pull_requests.read
tool_denylist:
  - contents.write
  - actions.write
requires_secrets:
  - MCP_PROVENANCE_SECRET_NAME
resources_enabled: false
prompts_enabled: false
`)
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "test@example.com")
	runTestGit(t, root, "config", "user.name", "Test User")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add MCP provenance fixture MCP_COMMIT_SUBJECT_SECRET")
}

func TestRenderMCPReportAuditsSpecsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/mcp/github-read.yaml", safeMCPSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	report := RenderMCPCLIReport(cfg)
	for _, want := range []string{
		"GitClaw MCP Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"mcp_status: `ok`",
		"mcp_specs_dir: `.gitclaw/mcp`",
		"mcp_specs: `1`",
		"parsed_mcp_specs: `1`",
		"mcp_specs_with_command: `0`",
		"mcp_specs_with_url: `0`",
		"mcp_specs_with_tool_allowlist: `1`",
		"mcp_tool_allowlist_refs: `3`",
		"mcp_tool_denylist_refs: `2`",
		"mcp_required_secret_refs: `1`",
		"mcp_env_passthrough_refs: `0`",
		"mcp_specs_with_resources_enabled: `0`",
		"mcp_specs_with_prompts_enabled: `0`",
		"mcp_specs_with_risk_findings: `0`",
		"mcp_risk_findings: `0`",
		"mcp_connection_supported: `false`",
		"mcp_server_launch_allowed: `false`",
		"mcp_tool_exposure_allowed: `false`",
		"dynamic_tool_discovery_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_mcp_bodies_included: `false`",
		"raw_command_args_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_mcp_change: `true`",
		"mcp_name=`github-read`",
		"path=`.gitclaw/mcp/github-read.yaml`",
		"transport=`stdio`",
		"source=`repo-reviewed-mcp`",
		"activation=`metadata-only`",
		"description=`true`",
		"command_present=`false`",
		"url_present=`false`",
		"tool_allowlist=`contents.read, issues.read, pull_requests.read`",
		"tool_denylist=`actions.write, contents.write`",
		"requires_secrets=`GITHUB_TOKEN`",
		"risk_findings=`0`",
		"risk_codes=`none`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("MCP report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"MCP_DESCRIPTION_SECRET", "repository:", "issue:"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("MCP report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestRenderMCPRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/mcp/risky.yaml", `name: risky
description: risky MCP spec.
transport: stdio
source: repo-reviewed-mcp
activation: runtime
command: bash
args:
  - -c
  - echo MCP_ARGS_SECRET
url: https://example.invalid/MCP_URL_SECRET
tool_allowlist:
  - contents.write
env_passthrough:
  - "*"
resources_enabled: true
prompts_enabled: true
notes: this unknown key should be reported by schema validation.
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	report := RenderMCPRiskCLIReport(cfg)
	for _, want := range []string{
		"GitClaw MCP Risk Report",
		"scope: `local-cli`",
		"mcp_status: `high`",
		"mcp_specs: `1`",
		"parsed_mcp_specs: `0`",
		"mcp_specs_with_command: `1`",
		"mcp_specs_with_url: `1`",
		"mcp_specs_with_resources_enabled: `1`",
		"mcp_specs_with_prompts_enabled: `1`",
		"mcp_specs_with_risk_findings: `1`",
		"high_risk_findings: `2`",
		"warning_risk_findings: `6`",
		"mcp_name=`risky`",
		"command_present=`true`",
		"args_count=`2`",
		"url_present=`true`",
		"env_passthrough=`*`",
		"resources_enabled=`true`",
		"prompts_enabled=`true`",
		"risk_max_severity=`high`",
		"code=`mcp_yaml_parse_error`",
		"code=`mcp_activation_not_metadata_only`",
		"code=`mcp_command_declared`",
		"code=`mcp_remote_endpoint_declared`",
		"code=`mcp_resources_enabled`",
		"code=`mcp_prompts_enabled`",
		"code=`mcp_unbounded_env_passthrough`",
		"code=`mcp_mutating_tool_allowlisted`",
		"line_sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("MCP risk report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"MCP_ARGS_SECRET", "MCP_URL_SECRET", "bash\n", "https://example.invalid"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("MCP risk report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestRenderMCPInfoReportFocusesOneSpecWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/mcp/github-read.yaml", safeMCPSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	report := RenderMCPInfoCLIReport(cfg, "github-read")
	for _, want := range []string{
		"GitClaw MCP Info Report",
		"scope: `local-cli`",
		"requested_mcp_sha256_12:",
		"normalized_mcp: `github-read`",
		"mcp_info_status: `ok`",
		"mcp_specs: `1`",
		"matched_mcp_specs: `1`",
		"raw_requested_mcp_included: `false`",
		"raw_mcp_bodies_included: `false`",
		"raw_command_args_included: `false`",
		"mcp_name=`github-read`",
		"risk_findings=`0`",
		"### Risk Findings For Matches",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("MCP info report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "MCP_DESCRIPTION_SECRET") {
		t.Fatalf("MCP info report leaked description body:\n%s", report)
	}
}

func TestPluginsMCPCommandsReportSpecs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/mcp/github-read.yaml", safeMCPSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)

	listOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"plugins", "mcp"}); err != nil {
			t.Fatalf("plugins mcp returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw MCP Report", "scope: `local-cli`", "mcp_status: `ok`", "mcp_specs: `1`"} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("plugins mcp output missing %q:\n%s", want, listOutput)
		}
	}

	infoOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"plugins", "mcp", "info", "github-read"}); err != nil {
			t.Fatalf("plugins mcp info returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw MCP Info Report", "mcp_info_status: `ok`", "matched_mcp_specs: `1`"} {
		if !strings.Contains(infoOutput, want) {
			t.Fatalf("plugins mcp info output missing %q:\n%s", want, infoOutput)
		}
	}
	if strings.Contains(listOutput, "MCP_DESCRIPTION_SECRET") || strings.Contains(infoOutput, "MCP_DESCRIPTION_SECRET") {
		t.Fatalf("MCP CLI leaked spec description:\nlist:\n%s\ninfo:\n%s", listOutput, infoOutput)
	}
}

func TestRenderMCPProvenanceReportsGitHistoryWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeMCPProvenanceGitFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	report := RenderMCPProvenanceCLIReport(cfg)
	for _, want := range []string{
		"GitClaw MCP Provenance Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"mcp_provenance_status: `ok`",
		"provenance_scope: `repo-local-mcp-git-history`",
		"mcp_status: `ok`",
		"mcp_specs_dir: `.gitclaw/mcp`",
		"mcp_specs: `1`",
		"parsed_mcp_specs: `1`",
		"mcp_specs_with_command: `0`",
		"mcp_specs_with_url: `0`",
		"mcp_specs_with_tool_allowlist: `1`",
		"mcp_tool_allowlist_refs: `3`",
		"mcp_tool_denylist_refs: `2`",
		"mcp_required_secret_refs: `1`",
		"mcp_env_passthrough_refs: `0`",
		"mcp_specs_with_resources_enabled: `0`",
		"mcp_specs_with_prompts_enabled: `0`",
		"mcp_specs_with_risk_findings: `0`",
		"mcp_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"git_tracked_mcp_specs: `1`",
		"untracked_mcp_specs: `0`",
		"working_tree_dirty_mcp_specs: `0`",
		"mcp_specs_with_commits: `1`",
		"mcp_specs_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"mcp_connection_supported: `false`",
		"mcp_server_launch_allowed: `false`",
		"mcp_tool_exposure_allowed: `false`",
		"dynamic_tool_discovery_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_mcp_bodies_included: `false`",
		"raw_command_args_included: `false`",
		"raw_urls_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"credential_values_included: `false`",
		"env_values_included: `false`",
		"llm_e2e_required_after_mcp_provenance_change: `true`",
		"### MCP Provenance Cards",
		"mcp_name=`github-read` path=`.gitclaw/mcp/github-read.yaml`",
		"transport=`stdio`",
		"activation=`metadata-only`",
		"description=`true`",
		"source_present=`true`",
		"source_sha256_12=",
		"command_present=`false`",
		"command_sha256_12=`none`",
		"args_count=`0`",
		"args_sha256_12=`none`",
		"url_present=`false`",
		"url_sha256_12=`none`",
		"tool_allowlist=`contents.read, issues.read, pull_requests.read`",
		"tool_denylist=`actions.write, contents.write`",
		"requires_secret_refs=`1`",
		"requires_secrets_sha256_12=",
		"env_passthrough_refs=`0`",
		"env_passthrough_sha256_12=`none`",
		"resources_enabled=`false`",
		"prompts_enabled=`false`",
		"parse_error=`false`",
		"parse_error_sha256_12=`none`",
		"risk_findings=`0`",
		"risk_max_severity=`none`",
		"risk_codes=`none`",
		"git_tracked=`true`",
		"working_tree_dirty=`false`",
		"commit_available=`true`",
		"last_commit_sha256_12=",
		"last_commit_short=",
		"last_commit_date=",
		"subject_sha256_12=",
		"### Provenance Gates",
		"risk_gate=`pass`",
		"git_history_gate=`pass`",
		"connection_gate=`disabled`",
		"server_launch_gate=`disabled`",
		"tool_exposure_gate=`disabled`",
		"dynamic_discovery_gate=`disabled`",
		"mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"raw_command_args_gate=`hash_only`",
		"raw_url_gate=`hash_only`",
		"credential_value_gate=`disabled`",
		"env_value_gate=`disabled`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("MCP provenance report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{
		"MCP_PROVENANCE_DESCRIPTION_SECRET",
		"MCP_PROVENANCE_SOURCE_SECRET",
		"MCP_PROVENANCE_SECRET_NAME",
		"MCP_COMMIT_SUBJECT_SECRET",
		"test@example.com",
		"Test User",
	} {
		if strings.Contains(report, notWant) {
			t.Fatalf("MCP provenance leaked %q:\n%s", notWant, report)
		}
	}
}

func TestPluginsMCPProvenanceCommandReportsSpecs(t *testing.T) {
	dir := t.TempDir()
	writeMCPProvenanceGitFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"plugins", "mcp", "provenance"}); err != nil {
			t.Fatalf("plugins mcp provenance returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw MCP Provenance Report", "scope: `local-cli`", "mcp_provenance_status: `ok`", "git_tracked_mcp_specs: `1`", "mcp_specs_with_commits: `1`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("plugins mcp provenance output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "MCP_PROVENANCE_DESCRIPTION_SECRET") || strings.Contains(output, "MCP_COMMIT_SUBJECT_SECRET") {
		t.Fatalf("MCP provenance CLI leaked secret text:\n%s", output)
	}
}

func TestRenderPluginReportRoutesMCPRiskWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/mcp/github-read.yaml", safeMCPSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 121,
			"title": "@gitclaw /plugins mcp risk",
			"body": "Hidden MCP route token: MCP_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderPluginReport(ev, cfg)
	for _, want := range []string{"GitClaw MCP Risk Report", "repository: `owner/repo`", "issue: `#121`", "mcp_status: `ok`", "issue_title_sha256_12:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("MCP routed report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "MCP_ROUTE_BODY_SECRET") || strings.Contains(report, "MCP_DESCRIPTION_SECRET") {
		t.Fatalf("MCP routed report leaked body text:\n%s", report)
	}
}

func TestRenderPluginReportRoutesMCPProvenanceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeMCPProvenanceGitFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 123,
			"title": "@gitclaw /plugins mcp provenance",
			"body": "Hidden MCP provenance route token: MCP_PROVENANCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderPluginReport(ev, cfg)
	for _, want := range []string{"GitClaw MCP Provenance Report", "repository: `owner/repo`", "issue: `#123`", "mcp_provenance_status: `ok`", "issue_title_sha256_12:", "git_history_gate=`pass`"} {
		if !strings.Contains(report, want) {
			t.Fatalf("MCP provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"MCP_PROVENANCE_ROUTE_BODY_SECRET", "MCP_PROVENANCE_DESCRIPTION_SECRET", "MCP_PROVENANCE_SOURCE_SECRET", "MCP_COMMIT_SUBJECT_SECRET"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("MCP provenance routed report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestHandlePluginsMCPRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/mcp/github-read.yaml", safeMCPSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 122,
			"title": "@gitclaw /plugin mcp risk",
			"body": "Hidden MCP handler token: MCP_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{122: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic MCP report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw MCP Risk Report", "Generated without a model call", "model=\"gitclaw/plugins\"", "mcp_status: `ok`", "mcp_specs: `1`", "raw_mcp_bodies_included: `false`", "llm_e2e_required_after_mcp_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("MCP handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"MCP_HANDLER_BODY_SECRET", "MCP_DESCRIPTION_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("MCP handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[122], "gitclaw:done") || hasLabel(github.IssueLabels[122], "gitclaw:running") || hasLabel(github.IssueLabels[122], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[122])
	}
}

func TestHandlePluginsMCPProvenanceCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeMCPProvenanceGitFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 124,
			"title": "@gitclaw /plugin mcp history",
			"body": "Hidden MCP provenance handler token: MCP_PROVENANCE_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{124: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic MCP provenance report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw MCP Provenance Report", "Generated without a model call", "model=\"gitclaw/plugins\"", "mcp_provenance_status: `ok`", "mcp_specs: `1`", "git_tracked_mcp_specs: `1`", "mcp_specs_with_commits: `1`", "raw_mcp_bodies_included: `false`", "raw_command_args_included: `false`", "raw_urls_included: `false`", "raw_git_subjects_included: `false`", "author_identities_included: `false`", "llm_e2e_required_after_mcp_provenance_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("MCP provenance handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"MCP_PROVENANCE_HANDLER_BODY_SECRET", "MCP_PROVENANCE_DESCRIPTION_SECRET", "MCP_PROVENANCE_SOURCE_SECRET", "MCP_PROVENANCE_SECRET_NAME", "MCP_COMMIT_SUBJECT_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("MCP provenance handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[124], "gitclaw:done") || hasLabel(github.IssueLabels[124], "gitclaw:running") || hasLabel(github.IssueLabels[124], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[124])
	}
}
