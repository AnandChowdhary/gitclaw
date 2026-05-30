package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestNodesRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"nodes", "risk"}); err != nil {
			t.Fatalf("nodes risk returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Node Risk Report", "scope: `local-cli`", "Generated without a model call", "node_risk_status: `ok`", "verification_scope: `github_actions_node_metadata`", "node_policy_present: `true`", "node_policy_loaded_for_model: `true`", "node_specs: `1`", "scanned_node_specs: `1`", "node_roles: `1`", "node_capabilities_declared: `3`", "node_specs_requiring_approval: `1`", "node_specs_ephemeral_jobs: `1`", "current_issue_node_request: `false`", "surfaces_with_risk_findings: `0`", "node_risk_findings: `0`", "active_node_runtime: `github-actions-ephemeral-job`", "node_inventory_source: `git-reviewed-metadata`", "gateway_websocket_required: `false`", "headless_node_host_supported: `false`", "node_pairing_supported: `false`", "node_rpc_supported: `false`", "node_command_invocation_supported: `false`", "remote_node_exec_supported: `false`", "browser_proxy_supported: `false`", "media_device_capabilities_supported: `false`", "long_running_node_service_supported: `false`", "repository_mutation_allowed: `false`", "raw_node_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "credential_values_included: `false`", "llm_e2e_required_after_node_risk_change: `true`", "### Node Policy Risk Card", "kind=`node-policy` path=`.gitclaw/NODES.md`", "risk_findings=`0`", "risk_codes=`none`", "### Node Spec Risk Cards", "kind=`node-spec` name=`github-actions-runner` path=`.gitclaw/nodes/github-actions-runner.md`", "### Current Node Request Risk Card", "scope=`local-cli` current_issue_node_request=`false`", "### Risk Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("nodes risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "NODE_POLICY_BODY_SECRET", "NODE_SPEC_BODY_SECRET", "GitHub Actions Runner"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("nodes risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderNodeRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/risky.md", `---
name: risky
role: remote-host
runtime: node-host
mode: paired-device
capabilities:
  - remote-command
requires_approval: false
---

OPENCLAW_GATEWAY_TOKEN=NODE_RISK_SPEC_SECRET
openclaw node run --host 10.0.0.1 --port 18789
node.invoke system.run host=node
autoApproveCidrs: ["0.0.0.0/0"]
browserProxy.enabled: true
camera.capture
hermes -p worker chat -q run
retry forever
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	output := RenderNodeRiskCLIReport(cfg)
	for _, want := range []string{"GitClaw Node Risk Report", "node_risk_status: `high`", "node_specs: `1`", "scanned_node_specs: `1`", "node_specs_requiring_approval: `0`", "node_specs_ephemeral_jobs: `0`", "surfaces_with_risk_findings: `1`", "node_risk_findings: `11`", "high_risk_findings: `4`", "warning_risk_findings: `7`", "code=`credential_material_in_node`", "code=`gateway_websocket_node_host`", "code=`remote_node_exec_enabled`", "code=`node_pairing_autoapprove`", "code=`browser_proxy_enabled`", "code=`media_device_capability`", "code=`external_worker_lane`", "code=`unbounded_node_loop`", "code=`node_runtime_not_github_actions`", "code=`node_mode_not_ephemeral_job`", "code=`node_approval_gate_missing`", "line_sha256_12=", "risk_max_severity=`high`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("nodes risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"NODE_RISK_SPEC_SECRET", "openclaw node run", "system.run", "autoApproveCidrs", "browserProxy.enabled", "camera.capture", "hermes -p", "retry forever", "OPENCLAW_GATEWAY_TOKEN="} {
		if strings.Contains(output, notWant) {
			t.Fatalf("nodes risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderNodeReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, root, ".gitclaw/nodes/github-actions-runner.md", "OPENCLAW_GATEWAY_TOKEN=NODE_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /nodes risk",
			"body": "Hidden nodes risk body token: NODE_RISK_BODY_SECRET.",
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
	body := RenderNodeReport(ev, cfg)
	for _, want := range []string{"GitClaw Node Risk Report", "repository: `owner/repo`", "issue: `#125`", "node_risk_status: `high`", "code=`credential_material_in_node`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("nodes risk report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "NODE_RISK_BODY_SECRET") || strings.Contains(body, "NODE_ROUTE_RISK_SPEC_SECRET") {
		t.Fatalf("nodes risk report leaked body token:\n%s", body)
	}
}

func TestHandleNodesRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, nodePolicyPath, nodePolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/nodes/github-actions-runner.md", nodeSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /node risk",
			"body": "Hidden nodes risk handler token: NODES_RISK_HANDLER_BODY_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic nodes risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Node Risk Report", "Generated without a model call", "model=\"gitclaw/nodes\"", "node_risk_status: `ok`", "verification_scope: `github_actions_node_metadata`", "raw_node_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "llm_e2e_required_after_node_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("nodes risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"NODES_RISK_HANDLER_BODY_SECRET", "NODE_POLICY_BODY_SECRET", "NODE_SPEC_BODY_SECRET", "GitHub Actions Runner"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("nodes risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}
