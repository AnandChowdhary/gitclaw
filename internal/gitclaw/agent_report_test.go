package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const agentPolicyTestBody = `# Agents

AGENT_POLICY_BODY_SECRET
`

const agentSpecTestBody = `---
name: repo-assistant
role: primary
runtime: github-actions
mode: single-assistant
tools:
  - gitclaw.search_files
  - gitclaw.read_file
requires_approval: true
---

# Repo Assistant

This assistant does not spawn subagents.
AGENT_SPEC_BODY_SECRET
`

func TestRenderAgentReportAuditsAgentSpecsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 121,
			"title": "@gitclaw /agents",
			"body": "Hidden agents report body token: AGENTS_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderAgentReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Agents Report",
		"Generated without a model call",
		"agents_status: `ok`",
		"agent_policy_path: `.gitclaw/AGENTS.md`",
		"agent_policy_present: `true`",
		"agent_policy_loaded_for_model: `true`",
		"agent_specs_dir: `.gitclaw/agents`",
		"agent_specs: `1`",
		"agent_specs_with_frontmatter: `1`",
		"agent_roles: `1`",
		"agent_tools_requested: `2`",
		"agent_specs_requiring_approval: `1`",
		"agent_specs_single_assistant: `1`",
		"active_agent_runtime: `github-actions`",
		"multi_agent_routing_supported: `false`",
		"multi_agent_delegation_supported: `false`",
		"subagent_execution_supported: `false`",
		"delegate_task_supported: `false`",
		"remote_agent_process_allowed: `false`",
		"agent_to_agent_messaging_allowed: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_agent_bodies_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Agent Policy",
		"`.gitclaw/AGENTS.md` loaded=`true` source=`contextDocumentPaths`",
		"### Agent Specs",
		"name=`repo-assistant`",
		"path=`.gitclaw/agents/repo-assistant.md`",
		"frontmatter=`true`",
		"role=`primary`",
		"runtime=`github-actions`",
		"mode=`single-assistant`",
		"tools=`2`",
		"requires_approval=`true`",
		"### Runtime Boundary",
		"GitHub Actions is the only active assistant runtime in v1",
		"future multi-agent routing or delegation requires reviewed workflows",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("agent report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "AGENTS_REPORT_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("agent report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestRenderAgentCatalogReportShowsCommandAndLayerSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 123,
			"title": "@gitclaw /agents catalog",
			"body": "Hidden agents catalog body token: AGENTS_CATALOG_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderAgentReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Agents Catalog Report",
		"Generated without a model call",
		"requested_agents_command: `catalog`",
		"agents_command_status: `ok`",
		"issue_side_execution: `github_actions_agent_metadata`",
		"agents_catalog_status: `ok`",
		"catalog_strategy: `compact-single-assistant-agent-discovery`",
		"agent_model: `github-actions-single-repo-assistant`",
		"agent_scope: `repository-issue-assistant`",
		"agent_policy_path: `.gitclaw/AGENTS.md`",
		"agent_policy_present: `true`",
		"agent_policy_loaded_for_model: `true`",
		"agent_specs_dir: `.gitclaw/agents`",
		"agent_specs: `1`",
		"agent_specs_with_frontmatter: `1`",
		"agent_roles: `1`",
		"agent_tools_requested: `2`",
		"agent_specs_requiring_approval: `1`",
		"agent_specs_single_assistant: `1`",
		"active_agent_runtime: `github-actions`",
		"catalog_entries: `5`",
		"agent_layers: `7`",
		"multi_agent_delegation_supported: `false`",
		"subagent_execution_supported: `false`",
		"delegate_task_supported: `false`",
		"shared_agent_memory_supported: `false`",
		"agent_gateway_supported: `false`",
		"raw_agent_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_agents_catalog_change: `true`",
		"### Catalog Entries",
		"command=`catalog` issue_intent=`@gitclaw /agents catalog` local_command=`gitclaw agents catalog` execution=`metadata-only` gate=`body-free-output`",
		"command=`list` issue_intent=`@gitclaw /agents` local_command=`gitclaw agents list`",
		"command=`provenance` issue_intent=`@gitclaw /agents provenance` local_command=`gitclaw agents provenance`",
		"command=`risk` issue_intent=`@gitclaw /agents risk` local_command=`gitclaw agents risk`",
		"### Agent Layers",
		"layer=`policy` store=`.gitclaw/AGENTS.md`",
		"layer=`specs` store=`.gitclaw/agents/*.md`",
		"layer=`runtime` store=`GitHub Actions workflow`",
		"layer=`delegation` store=`unsupported in v1`",
		"### Catalog Gates",
		"agent_policy_gate=`repo-reviewed-policy-file`",
		"delegation_gate=`disabled-single-assistant-v1`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("agent catalog report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "AGENTS_CATALOG_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("agent catalog report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestAgentsListCommandReportsAgents(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"agents", "list"}); err != nil {
			t.Fatalf("agents list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Agents Report", "scope: `local-cli`", "agents_status: `ok`", "agent_policy_loaded_for_model: `true`", "agent_specs: `1`", "agent_specs_single_assistant: `1`", "multi_agent_delegation_supported: `false`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("agents list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "AGENT_POLICY_BODY_SECRET") || strings.Contains(output, "AGENT_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("agents list leaked body or issue metadata:\n%s", output)
	}
}

func TestAgentsCatalogCommandReportsSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"agents", "catalog"}); err != nil {
			t.Fatalf("agents catalog returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Agents Catalog Report", "scope: `local-cli`", "agents_catalog_status: `ok`", "catalog_entries: `5`", "agent_layers: `7`", "command=`catalog` issue_intent=`@gitclaw /agents catalog` local_command=`gitclaw agents catalog`", "command=`provenance` issue_intent=`@gitclaw /agents provenance` local_command=`gitclaw agents provenance`", "layer=`tools` store=`agent spec tool names`", "raw_agent_bodies_included: `false`", "model_e2e_gate=`required`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("agents catalog output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "AGENT_POLICY_BODY_SECRET") || strings.Contains(output, "AGENT_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("agents catalog leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleAgentsCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 122,
			"title": "@gitclaw /agent",
			"body": "Hidden agents handler token: AGENTS_HANDLER_BODY_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic agents command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Agents Report", "Generated without a model call", "model=\"gitclaw/agents\"", "agents_status: `ok`", "agent_policy_loaded_for_model: `true`", "agent_specs: `1`", "active_agent_runtime: `github-actions`", "raw_agent_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("agents handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"AGENTS_HANDLER_BODY_SECRET", "AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("agents handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[122], "gitclaw:done") || hasLabel(github.IssueLabels[122], "gitclaw:running") || hasLabel(github.IssueLabels[122], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[122])
	}
}

func TestHandleAgentsCatalogCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 124,
			"title": "@gitclaw /agent commands",
			"body": "Hidden agents catalog handler token: AGENTS_CATALOG_HANDLER_BODY_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic agents catalog command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Agents Catalog Report", "Generated without a model call", "model=\"gitclaw/agents\"", "requested_agents_command: `catalog`", "agents_catalog_status: `ok`", "catalog_entries: `5`", "agent_layers: `7`", "command=`catalog` issue_intent=`@gitclaw /agents catalog`", "command=`provenance` issue_intent=`@gitclaw /agents provenance`", "layer=`policy` store=`.gitclaw/AGENTS.md`", "raw_agent_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("agents catalog handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"AGENTS_CATALOG_HANDLER_BODY_SECRET", "AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("agents catalog handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[124], "gitclaw:done") || hasLabel(github.IssueLabels[124], "gitclaw:running") || hasLabel(github.IssueLabels[124], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[124])
	}
}

func TestLoadRepoContextIncludesAgentPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, agentPolicyPath, agentPolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == agentPolicyPath {
			found = true
			if !strings.Contains(doc.Body, "AGENT_POLICY_BODY_SECRET") {
				t.Fatalf("agent policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("agent policy file was not loaded into context: %#v", ctx.Documents)
	}
}
