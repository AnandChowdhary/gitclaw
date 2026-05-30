package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestPluginsRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/plugins/github-models-provider.md", pluginSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"plugins", "risk"}); err != nil {
			t.Fatalf("plugins risk returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Plugin Risk Report", "scope: `local-cli`", "Generated without a model call", "plugin_risk_status: `ok`", "verification_scope: `repo_reviewed_plugin_metadata`", "plugin_policy_present: `true`", "plugin_policy_loaded_for_model: `true`", "plugin_specs: `1`", "scanned_plugin_specs: `1`", "plugin_capabilities: `2`", "plugin_optional_capabilities: `1`", "plugin_specs_requiring_approval: `1`", "plugin_specs_metadata_only: `1`", "package_files_present: `0`", "scanned_package_files: `0`", "surfaces_with_risk_findings: `0`", "plugin_risk_findings: `0`", "plugin_install_supported: `false`", "plugin_execution_supported: `false`", "plugin_execution_allowed: `false`", "mcp_connection_allowed: `false`", "repository_mutation_allowed: `false`", "raw_plugin_bodies_included: `false`", "raw_package_bodies_included: `false`", "credential_values_included: `false`", "llm_e2e_required_after_plugin_risk_change: `true`", "### Plugin Policy Risk Card", "kind=`plugin-policy` path=`.gitclaw/PLUGINS.md`", "risk_findings=`0`", "risk_codes=`none`", "### Plugin Spec Risk Cards", "kind=`plugin-spec` name=`github-models-provider` path=`.gitclaw/plugins/github-models-provider.md`", "### Package Runtime Risk Cards", "kind=`package-runtime` none", "### Risk Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("plugins risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "PLUGIN_POLICY_BODY_SECRET", "PLUGIN_SPEC_BODY_SECRET", "GitHub Models Provider"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("plugins risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderPluginRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/plugins/risky.md", `---
name: risky
kind: mcp
source: repo-reviewed
activation: runtime
capabilities:
  - tool:host-exec
requires_approval: false
---

api_key=PLUGIN_RISK_SPEC_SECRET
npm install risky-plugin
connect to MCP server
retry forever
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	output := RenderPluginRiskCLIReport(cfg)
	for _, want := range []string{"GitClaw Plugin Risk Report", "plugin_risk_status: `high`", "plugin_specs: `1`", "scanned_plugin_specs: `1`", "plugin_specs_requiring_approval: `0`", "plugin_specs_metadata_only: `0`", "surfaces_with_risk_findings: `1`", "plugin_risk_findings: `6`", "high_risk_findings: `3`", "warning_risk_findings: `3`", "code=`credential_material_in_plugin`", "code=`automatic_plugin_install`", "code=`mcp_runtime_connection`", "code=`plugin_activation_not_metadata_only`", "code=`plugin_approval_gate_missing`", "code=`unbounded_plugin_loop`", "line_sha256_12=", "risk_max_severity=`high`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("plugins risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"PLUGIN_RISK_SPEC_SECRET", "npm install risky-plugin", "connect to MCP server", "retry forever", "api_key="} {
		if strings.Contains(output, notWant) {
			t.Fatalf("plugins risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderPluginReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/plugins/github-models-provider.md", "api_key=PLUGIN_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 119,
			"title": "@gitclaw /plugins risk",
			"body": "Hidden plugins risk body token: PLUGIN_RISK_BODY_SECRET.",
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
	cfg.Workdir = root
	body := RenderPluginReport(ev, cfg)
	for _, want := range []string{"GitClaw Plugin Risk Report", "repository: `owner/repo`", "issue: `#119`", "plugin_risk_status: `high`", "code=`credential_material_in_plugin`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("plugins risk report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "PLUGIN_RISK_BODY_SECRET") || strings.Contains(body, "PLUGIN_ROUTE_RISK_SPEC_SECRET") {
		t.Fatalf("plugins risk report leaked body token:\n%s", body)
	}
}

func TestHandlePluginsRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/plugins/github-models-provider.md", pluginSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 120,
			"title": "@gitclaw /plugin risk",
			"body": "Hidden plugins risk handler token: PLUGINS_RISK_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{120: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic plugins risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Plugin Risk Report", "Generated without a model call", "model=\"gitclaw/plugins\"", "plugin_risk_status: `ok`", "verification_scope: `repo_reviewed_plugin_metadata`", "raw_plugin_bodies_included: `false`", "raw_package_bodies_included: `false`", "llm_e2e_required_after_plugin_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("plugins risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"PLUGINS_RISK_HANDLER_BODY_SECRET", "PLUGIN_POLICY_BODY_SECRET", "PLUGIN_SPEC_BODY_SECRET", "GitHub Models Provider"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("plugins risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[120], "gitclaw:done") || hasLabel(github.IssueLabels[120], "gitclaw:running") || hasLabel(github.IssueLabels[120], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[120])
	}
}
