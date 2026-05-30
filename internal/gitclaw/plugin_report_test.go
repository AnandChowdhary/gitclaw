package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const pluginPolicyTestBody = `# Plugins

PLUGIN_POLICY_BODY_SECRET
`

const pluginSpecTestBody = `---
name: github-models-provider
kind: provider
source: repo-reviewed
activation: metadata-only
capabilities:
  - model:github-models
  - tool:search_files
optional_capabilities:
  - mcp:github
requires_approval: true
---

# GitHub Models Provider

PLUGIN_SPEC_BODY_SECRET
`

func TestRenderPluginReportAuditsPluginsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/plugins/github-models-provider.md", pluginSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 117,
			"title": "@gitclaw /plugins",
			"body": "Hidden plugins report body token: PLUGINS_REPORT_BODY_SECRET.",
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
	for _, want := range []string{
		"GitClaw Plugins Report",
		"Generated without a model call",
		"plugins_status: `ok`",
		"plugin_policy_path: `.gitclaw/PLUGINS.md`",
		"plugin_policy_present: `true`",
		"plugin_policy_loaded_for_model: `true`",
		"plugin_specs_dir: `.gitclaw/plugins`",
		"plugin_specs: `1`",
		"plugin_specs_with_frontmatter: `1`",
		"plugin_capabilities: `2`",
		"plugin_optional_capabilities: `1`",
		"plugin_specs_requiring_approval: `1`",
		"plugin_specs_metadata_only: `1`",
		"plugin_package_files_present: `0`",
		"plugin_install_supported: `false`",
		"plugin_execution_supported: `false`",
		"plugin_execution_allowed: `false`",
		"mcp_connection_allowed: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_plugin_bodies_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Plugin Specs",
		"name=`github-models-provider`",
		"path=`.gitclaw/plugins/github-models-provider.md`",
		"frontmatter=`true`",
		"kind=`provider`",
		"source=`repo-reviewed`",
		"activation=`metadata-only`",
		"capabilities=`2`",
		"optional_capabilities=`1`",
		"requires_approval=`true`",
		"### Runtime Boundary",
		"plugin packages are not installed",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("plugin report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"PLUGIN_POLICY_BODY_SECRET", "PLUGIN_SPEC_BODY_SECRET", "PLUGINS_REPORT_BODY_SECRET", "GitHub Models Provider"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("plugin report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestPluginsListCommandReportsPlugins(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/plugins/github-models-provider.md", pluginSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"plugins", "list"}); err != nil {
			t.Fatalf("plugins list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Plugins Report", "scope: `local-cli`", "plugins_status: `ok`", "plugin_policy_loaded_for_model: `true`", "plugin_specs: `1`", "plugin_capabilities: `2`", "plugin_execution_allowed: `false`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("plugins list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "PLUGIN_POLICY_BODY_SECRET") || strings.Contains(output, "PLUGIN_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("plugins list leaked body or issue metadata:\n%s", output)
	}
}

func TestHandlePluginsCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/plugins/github-models-provider.md", pluginSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 118,
			"title": "@gitclaw /plugin",
			"body": "Hidden plugins handler token: PLUGINS_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{118: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic plugins command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Plugins Report", "Generated without a model call", "model=\"gitclaw/plugins\"", "plugins_status: `ok`", "plugin_policy_loaded_for_model: `true`", "plugin_specs: `1`", "plugin_execution_allowed: `false`", "raw_plugin_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("plugins handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"PLUGINS_HANDLER_BODY_SECRET", "PLUGIN_POLICY_BODY_SECRET", "PLUGIN_SPEC_BODY_SECRET", "GitHub Models Provider"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("plugins handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[118], "gitclaw:done") || hasLabel(github.IssueLabels[118], "gitclaw:running") || hasLabel(github.IssueLabels[118], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[118])
	}
}

func TestLoadRepoContextIncludesPluginPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, pluginPolicyPath, pluginPolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == pluginPolicyPath {
			found = true
			if !strings.Contains(doc.Body, "PLUGIN_POLICY_BODY_SECRET") {
				t.Fatalf("plugin policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("plugin policy file was not loaded into context: %#v", ctx.Documents)
	}
}
