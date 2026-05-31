package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const nodePolicyTestBody = `# Nodes

NODE_POLICY_BODY_SECRET
`

const nodeSpecTestBody = `---
name: github-actions-runner
role: primary-runtime
runtime: github-actions
mode: ephemeral-job
capabilities:
  - issue-event
  - workflow-dispatch
  - scheduled-run
requires_approval: true
---

# GitHub Actions Runner

This runner does not start node hosts or pair devices.
NODE_SPEC_BODY_SECRET
`

func TestRenderNodeReportAuditsNodeSpecsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 123,
			"title": "@gitclaw /nodes",
			"body": "Hidden nodes report body token: NODES_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderNodeReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Nodes Report",
		"Generated without a model call",
		"nodes_status: `ok`",
		"node_policy_path: `.gitclaw/NODES.md`",
		"node_policy_present: `true`",
		"node_policy_loaded_for_model: `true`",
		"node_specs_dir: `.gitclaw/nodes`",
		"node_specs: `1`",
		"node_specs_with_frontmatter: `1`",
		"node_roles: `1`",
		"node_capabilities_declared: `3`",
		"node_specs_requiring_approval: `1`",
		"node_specs_ephemeral_jobs: `1`",
		"active_node_runtime: `github-actions-ephemeral-job`",
		"node_inventory_source: `git-reviewed-metadata`",
		"gateway_websocket_required: `false`",
		"headless_node_host_supported: `false`",
		"node_pairing_supported: `false`",
		"node_rpc_supported: `false`",
		"node_command_invocation_supported: `false`",
		"remote_node_exec_supported: `false`",
		"browser_proxy_supported: `false`",
		"media_device_capabilities_supported: `false`",
		"long_running_node_service_supported: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_node_bodies_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Node Policy",
		"`.gitclaw/NODES.md` loaded=`true` source=`contextDocumentPaths`",
		"### Node Specs",
		"name=`github-actions-runner`",
		"path=`.gitclaw/nodes/github-actions-runner.md`",
		"frontmatter=`true`",
		"role=`primary-runtime`",
		"runtime=`github-actions`",
		"mode=`ephemeral-job`",
		"capabilities=`3`",
		"requires_approval=`true`",
		"### Runtime Boundary",
		"GitHub Actions jobs are the only active execution nodes in v1",
		"future remote-node execution requires reviewed workflows",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("node report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"NODE_POLICY_BODY_SECRET", "NODE_SPEC_BODY_SECRET", "NODES_REPORT_BODY_SECRET", "GitHub Actions Runner"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("node report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestRenderNodeCatalogReportShowsCommandAndLayerSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /nodes catalog",
			"body": "Hidden nodes catalog body token: NODES_CATALOG_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderNodeReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Nodes Catalog Report",
		"Generated without a model call",
		"requested_nodes_command: `catalog`",
		"nodes_command_status: `ok`",
		"issue_side_execution: `github_actions_node_metadata`",
		"nodes_catalog_status: `ok`",
		"catalog_strategy: `compact-github-actions-node-discovery`",
		"node_model: `github-actions-ephemeral-node-metadata`",
		"node_scope: `repository-execution-node`",
		"node_policy_path: `.gitclaw/NODES.md`",
		"node_policy_present: `true`",
		"node_policy_loaded_for_model: `true`",
		"node_specs_dir: `.gitclaw/nodes`",
		"node_specs: `1`",
		"node_specs_with_frontmatter: `1`",
		"node_roles: `1`",
		"node_capabilities_declared: `3`",
		"node_specs_requiring_approval: `1`",
		"node_specs_ephemeral_jobs: `1`",
		"active_node_runtime: `github-actions-ephemeral-job`",
		"node_inventory_source: `git-reviewed-metadata`",
		"catalog_entries: `4`",
		"node_layers: `8`",
		"gateway_process_supported: `false`",
		"headless_node_host_supported: `false`",
		"node_pairing_supported: `false`",
		"node_rpc_supported: `false`",
		"remote_node_exec_supported: `false`",
		"browser_proxy_supported: `false`",
		"media_device_capabilities_supported: `false`",
		"cross_node_routing_supported: `false`",
		"raw_node_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_nodes_catalog_change: `true`",
		"### Catalog Entries",
		"command=`catalog` issue_intent=`@gitclaw /nodes catalog` local_command=`gitclaw nodes catalog` execution=`metadata-only` gate=`body-free-output`",
		"command=`list` issue_intent=`@gitclaw /nodes` local_command=`gitclaw nodes list`",
		"command=`risk` issue_intent=`@gitclaw /nodes risk` local_command=`gitclaw nodes risk`",
		"### Node Layers",
		"layer=`policy` store=`.gitclaw/NODES.md`",
		"layer=`specs` store=`.gitclaw/nodes/*.md`",
		"layer=`runtime` store=`GitHub Actions workflow`",
		"layer=`wake` store=`issues/schedule/workflow_dispatch`",
		"layer=`capabilities` store=`node spec capability names`",
		"layer=`remote-exec` store=`unsupported in v1`",
		"### Catalog Gates",
		"node_policy_gate=`repo-reviewed-policy-file`",
		"runtime_gate=`github-actions-ephemeral-job-only`",
		"gateway_gate=`disabled-no-websocket-gateway-v1`",
		"remote_exec_gate=`disabled-no-remote-node-exec-v1`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("node catalog report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"NODE_POLICY_BODY_SECRET", "NODE_SPEC_BODY_SECRET", "NODES_CATALOG_BODY_SECRET", "GitHub Actions Runner"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("node catalog report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestNodesListCommandReportsNodes(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"nodes", "list"}); err != nil {
			t.Fatalf("nodes list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Nodes Report", "scope: `local-cli`", "nodes_status: `ok`", "node_policy_loaded_for_model: `true`", "node_specs: `1`", "node_specs_ephemeral_jobs: `1`", "remote_node_exec_supported: `false`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("nodes list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "NODE_POLICY_BODY_SECRET") || strings.Contains(output, "NODE_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("nodes list leaked body or issue metadata:\n%s", output)
	}
}

func TestNodesCatalogCommandReportsSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"nodes", "catalog"}); err != nil {
			t.Fatalf("nodes catalog returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Nodes Catalog Report", "scope: `local-cli`", "nodes_catalog_status: `ok`", "catalog_entries: `4`", "node_layers: `8`", "command=`catalog` issue_intent=`@gitclaw /nodes catalog` local_command=`gitclaw nodes catalog`", "layer=`capabilities` store=`node spec capability names`", "raw_node_bodies_included: `false`", "model_e2e_gate=`required`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("nodes catalog output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "NODE_POLICY_BODY_SECRET") || strings.Contains(output, "NODE_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("nodes catalog leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleNodesCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 124,
			"title": "@gitclaw /node",
			"body": "Hidden nodes handler token: NODES_HANDLER_BODY_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic nodes command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Nodes Report", "Generated without a model call", "model=\"gitclaw/nodes\"", "nodes_status: `ok`", "node_policy_loaded_for_model: `true`", "node_specs: `1`", "active_node_runtime: `github-actions-ephemeral-job`", "raw_node_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("nodes handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"NODES_HANDLER_BODY_SECRET", "NODE_POLICY_BODY_SECRET", "NODE_SPEC_BODY_SECRET", "GitHub Actions Runner"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("nodes handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[124], "gitclaw:done") || hasLabel(github.IssueLabels[124], "gitclaw:running") || hasLabel(github.IssueLabels[124], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[124])
	}
}

func TestHandleNodesCatalogCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /node capabilities",
			"body": "Hidden nodes catalog handler token: NODES_CATALOG_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{126: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic nodes catalog command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Nodes Catalog Report", "Generated without a model call", "model=\"gitclaw/nodes\"", "requested_nodes_command: `catalog`", "nodes_catalog_status: `ok`", "catalog_entries: `4`", "node_layers: `8`", "command=`catalog` issue_intent=`@gitclaw /nodes catalog`", "layer=`policy` store=`.gitclaw/NODES.md`", "raw_node_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("nodes catalog handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"NODES_CATALOG_HANDLER_BODY_SECRET", "NODE_POLICY_BODY_SECRET", "NODE_SPEC_BODY_SECRET", "GitHub Actions Runner"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("nodes catalog handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}

func TestLoadRepoContextIncludesNodePolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == nodePolicyPath {
			found = true
			if !strings.Contains(doc.Body, "NODE_POLICY_BODY_SECRET") {
				t.Fatalf("node policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("node policy file was not loaded into context: %#v", ctx.Documents)
	}
}
