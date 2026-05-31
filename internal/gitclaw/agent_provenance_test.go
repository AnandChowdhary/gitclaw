package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeAgentProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, agentPolicyPath, agentPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/agents/repo-assistant.md", agentSpecTestBody)
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "agent-provenance@example.invalid")
	runTestGit(t, root, "config", "user.name", "Agent Provenance Secret Author")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add agent provenance fixture AGENT_COMMIT_SUBJECT_SECRET")
}

func TestAgentsProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeAgentProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"agents", "provenance"}); err != nil {
			t.Fatalf("agents provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Agent Provenance Report",
		"scope: `local-cli`",
		"agent_provenance_status: `ok`",
		"provenance_scope: `repo-local-agent-git-history`",
		"agents_status: `ok`",
		"agent_validation_status: `ok`",
		"agent_validation_findings: `0`",
		"agent_risk_status: `ok`",
		"agent_risk_findings: `0`",
		"agent_policy_present: `true`",
		"agent_policy_loaded_for_model: `true`",
		"agent_specs: `1`",
		"agent_specs_with_frontmatter: `1`",
		"agent_roles: `1`",
		"agent_tools_requested: `2`",
		"agent_specs_requiring_approval: `1`",
		"agent_specs_single_assistant: `1`",
		"provenance_surfaces: `2`",
		"repo_local_surfaces: `2`",
		"unknown_source_surfaces: `0`",
		"git_tracked_surfaces: `2`",
		"untracked_surfaces: `0`",
		"working_tree_dirty_surfaces: `0`",
		"surfaces_with_commits: `2`",
		"surfaces_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"active_agent_runtime: `github-actions`",
		"multi_agent_routing_supported: `false`",
		"multi_agent_delegation_supported: `false`",
		"subagent_execution_supported: `false`",
		"delegate_task_supported: `false`",
		"remote_agent_process_allowed: `false`",
		"agent_to_agent_messaging_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_agent_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_agent_provenance_change: `true`",
		"### Agent Provenance Cards",
		"kind=`agent-policy` name=`agents-policy` path=`.gitclaw/AGENTS.md` source=`repo-local`",
		"kind=`agent-spec` name=`repo-assistant` path=`.gitclaw/agents/repo-assistant.md` source=`repo-local`",
		"frontmatter=`true`",
		"role=`primary`",
		"runtime=`github-actions`",
		"mode=`single-assistant`",
		"tools=`2`",
		"requires_approval=`true`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"validation_findings=`0`",
		"git_tracked=`true`",
		"working_tree_dirty=`false`",
		"commit_available=`true`",
		"last_commit_sha256_12=",
		"last_commit_short=",
		"last_commit_date=",
		"subject_sha256_12=",
		"### Provenance Gates",
		"agent_validation_gate=`pass`",
		"risk_gate=`pass`",
		"git_history_gate=`pass`",
		"runtime_gate=`github-actions-only`",
		"delegation_gate=`disabled-single-assistant-v1`",
		"mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("agents provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"AGENT_POLICY_BODY_SECRET",
		"AGENT_SPEC_BODY_SECRET",
		"Repo Assistant",
		"AGENT_COMMIT_SUBJECT_SECRET",
		"agent-provenance@example.invalid",
		"Agent Provenance Secret Author",
		"issue: `#0`",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("agents provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderAgentReportRoutesProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeAgentProvenanceGitFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /agents provenance",
			"body": "Hidden agents provenance body token: AGENTS_PROVENANCE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	body := RenderAgentReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Agent Provenance Report",
		"repository: `owner/repo`",
		"issue: `#125`",
		"agent_provenance_status: `ok`",
		"provenance_scope: `repo-local-agent-git-history`",
		"git_history_gate=`pass`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("agents provenance route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"AGENTS_PROVENANCE_BODY_SECRET", "AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "Repo Assistant"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("agents provenance route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestHandleAgentsProvenanceCommandPostsGitHistoryReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeAgentProvenanceGitFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /agent git-history",
			"body": "Hidden agents provenance handler token: AGENTS_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{126: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic agents provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Agent Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/agents\"",
		"agent_provenance_status: `ok`",
		"agent_risk_status: `ok`",
		"provenance_surfaces: `2`",
		"git_tracked_surfaces: `2`",
		"surfaces_with_commits: `2`",
		"raw_agent_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"git_history_gate=`pass`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("agents provenance handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"AGENTS_PROVENANCE_HANDLER_BODY_SECRET", "AGENT_POLICY_BODY_SECRET", "AGENT_SPEC_BODY_SECRET", "Repo Assistant", "AGENT_COMMIT_SUBJECT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("agents provenance handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}
