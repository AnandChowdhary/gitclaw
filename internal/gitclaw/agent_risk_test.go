package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestAgentsRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"agents", "risk"}); err != nil {
			t.Fatalf("agents risk returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Agent Risk Report", "scope: `local-cli`", "Generated without a model call", "agent_risk_status: `ok`", "verification_scope: `github_actions_agent_metadata`", "agent_policy_present: `true`", "agent_policy_loaded_for_model: `true`", "agent_specs: `1`", "scanned_agent_specs: `1`", "agent_roles: `1`", "agent_tools_requested: `2`", "agent_specs_requiring_approval: `1`", "agent_specs_single_assistant: `1`", "current_issue_agent_request: `false`", "surfaces_with_risk_findings: `0`", "agent_risk_findings: `0`", "active_agent_runtime: `github-actions`", "multi_agent_routing_supported: `false`", "multi_agent_delegation_supported: `false`", "subagent_execution_supported: `false`", "delegate_task_supported: `false`", "remote_agent_process_allowed: `false`", "agent_to_agent_messaging_allowed: `false`", "repository_mutation_allowed: `false`", "raw_agent_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "credential_values_included: `false`", "llm_e2e_required_after_agent_risk_change: `true`", "### Agent Policy Risk Card", "kind=`agent-policy` path=`.gitclaw/AGENTS.md`", "risk_findings=`0`", "risk_codes=`none`", "### Agent Spec Risk Cards", "kind=`agent-spec` name=`repo-assistant` path=`.gitclaw/agents/repo-assistant.md`", "### Current Agent Request Risk Card", "scope=`local-cli` current_issue_agent_request=`false`", "### Risk Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("agents risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("agents risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderAgentRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/risky.md", `---
name: risky
role: orchestrator
runtime: acp
mode: multi-agent
tools:
  - gitclaw.search_files
requires_approval: false
---

api_key=AGENT_RISK_SPEC_SECRET
delegate_task(tasks=["research"])
gateway start
same bot token
retry forever
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	output := RenderAgentRiskCLIReport(cfg)
	for _, want := range []string{"GitClaw Agent Risk Report", "agent_risk_status: `high`", "agent_specs: `1`", "scanned_agent_specs: `1`", "agent_specs_requiring_approval: `0`", "agent_specs_single_assistant: `0`", "surfaces_with_risk_findings: `1`", "agent_risk_findings: `9`", "high_risk_findings: `4`", "warning_risk_findings: `5`", "code=`credential_material_in_agent`", "code=`subagent_delegation_enabled`", "code=`external_agent_process`", "code=`shared_agent_secret_state`", "code=`unbounded_agent_loop`", "code=`agent_runtime_not_github_actions`", "code=`agent_mode_not_single_assistant`", "code=`agent_approval_gate_missing`", "line_sha256_12=", "risk_max_severity=`high`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("agents risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"AGENT_RISK_SPEC_SECRET", "delegate_task(tasks", "gateway start", "same bot token", "retry forever", "api_key="} {
		if strings.Contains(output, notWant) {
			t.Fatalf("agents risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderAgentReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/agents/repo-assistant.md", "api_key=AGENT_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 123,
			"title": "@gitclaw /agents risk",
			"body": "Hidden agents risk body token: AGENT_RISK_BODY_SECRET.",
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
	body := RenderAgentReport(ev, cfg)
	for _, want := range []string{"GitClaw Agent Risk Report", "repository: `owner/repo`", "issue: `#123`", "agent_risk_status: `high`", "code=`credential_material_in_agent`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("agents risk report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "AGENT_RISK_BODY_SECRET") || strings.Contains(body, "AGENT_ROUTE_RISK_SPEC_SECRET") {
		t.Fatalf("agents risk report leaked body token:\n%s", body)
	}
}

func TestHandleAgentsRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 124,
			"title": "@gitclaw /agent risk",
			"body": "Hidden agents risk handler token: AGENTS_RISK_HANDLER_BODY_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic agents risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Agent Risk Report", "Generated without a model call", "model=\"gitclaw/agents\"", "agent_risk_status: `ok`", "verification_scope: `github_actions_agent_metadata`", "raw_agent_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "llm_e2e_required_after_agent_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("agents risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"AGENTS_RISK_HANDLER_BODY_SECRET", "AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("agents risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[124], "gitclaw:done") || hasLabel(github.IssueLabels[124], "gitclaw:running") || hasLabel(github.IssueLabels[124], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[124])
	}
}
