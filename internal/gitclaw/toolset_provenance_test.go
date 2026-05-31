package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeToolsetProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/toolsets/repo-read.yaml", `name: repo-read
description: TOOLSET_PROVENANCE_DESCRIPTION_SECRET
mode: read-only
tools:
  - gitclaw.list_files
  - gitclaw.search_files
  - gitclaw.read_file
instruction: |
  TOOLSET_PROVENANCE_INSTRUCTION_SECRET
  Prefer bounded repository search.
`)
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "test@example.com")
	runTestGit(t, root, "config", "user.name", "Test User")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add toolset provenance fixture TOOLSET_COMMIT_SUBJECT_SECRET")
}

func TestToolsetsProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeToolsetProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "toolsets", "provenance"}); err != nil {
			t.Fatalf("tools toolsets provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Toolsets Provenance Report",
		"scope: `local-cli`",
		"toolset_provenance_status: `ok`",
		"provenance_scope: `repo-local-toolset-git-history`",
		"toolset_store_status: `ok`",
		"toolset_store_path: `.gitclaw/toolsets`",
		"toolset_files: `1`",
		"scanned_toolsets: `1`",
		"toolset_tool_refs: `3`",
		"resolved_tool_refs: `3`",
		"unknown_tool_refs: `0`",
		"disabled_tool_refs: `0`",
		"allowlist_blocked_tool_refs: `0`",
		"toolsets_with_instruction: `1`",
		"toolsets_with_risk_findings: `0`",
		"git_tracked_toolsets: `1`",
		"untracked_toolsets: `0`",
		"working_tree_dirty_toolsets: `0`",
		"toolsets_with_commits: `1`",
		"toolsets_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"toolset_activation_supported: `false`",
		"repository_mutation_allowed: `false`",
		"shell_execution_allowed: `false`",
		"network_tool_execution_allowed: `false`",
		"raw_toolset_bodies_included: `false`",
		"raw_toolset_instructions_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_toolset_provenance_change: `true`",
		"### Toolset Provenance Cards",
		"toolset_name=`repo-read` path=`.gitclaw/toolsets/repo-read.yaml`",
		"mode=`read-only`",
		"tools=`gitclaw.list_files, gitclaw.read_file, gitclaw.search_files`",
		"resolved_tools=`gitclaw.list_files, gitclaw.read_file, gitclaw.search_files`",
		"unknown_tools=`none`",
		"instruction=`true`",
		"description=`true`",
		"parse_error=`false`",
		"risk_findings=`0`",
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
		"activation_gate=`disabled`",
		"mutation_gate=`disabled`",
		"shell_execution_gate=`disabled`",
		"network_execution_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("toolset provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"TOOLSET_PROVENANCE_DESCRIPTION_SECRET",
		"TOOLSET_PROVENANCE_INSTRUCTION_SECRET",
		"Prefer bounded repository search",
		"TOOLSET_COMMIT_SUBJECT_SECRET",
		"test@example.com",
		"Test User",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("toolset provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderToolsReportRoutesToolsetProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeToolsetProvenanceGitFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /tools toolsets provenance"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 170,
			"title": "@gitclaw /tools toolsets provenance",
			"body": "Hidden toolset provenance body token: TOOLSET_PROVENANCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderToolsReport(ev, cfg, repoContext)
	for _, want := range []string{
		"GitClaw Toolsets Provenance Report",
		"repository: `owner/repo`",
		"issue: `#170`",
		"toolset_provenance_status: `ok`",
		"toolset_files: `1`",
		"git_tracked_toolsets: `1`",
		"toolsets_with_commits: `1`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("toolset provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"TOOLSET_PROVENANCE_ROUTE_BODY_SECRET", "TOOLSET_PROVENANCE_DESCRIPTION_SECRET", "TOOLSET_PROVENANCE_INSTRUCTION_SECRET", "TOOLSET_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("toolset provenance routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestHandleToolsetsProvenanceCommandPostsGitHistoryReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeToolsetProvenanceGitFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 171,
			"title": "@gitclaw /tools toolsets history",
			"body": "Hidden toolset provenance body token: TOOLSET_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{171: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic toolset provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Toolsets Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/tools\"",
		"repository: `owner/repo`",
		"issue: `#171`",
		"toolset_provenance_status: `ok`",
		"toolset_files: `1`",
		"git_tracked_toolsets: `1`",
		"working_tree_dirty_toolsets: `0`",
		"toolsets_with_commits: `1`",
		"raw_toolset_bodies_included: `false`",
		"raw_toolset_instructions_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_toolset_provenance_change: `true`",
		"toolset_name=`repo-read` path=`.gitclaw/toolsets/repo-read.yaml`",
		"git_history_gate=`pass`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("toolset provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOLSET_PROVENANCE_HANDLER_BODY_SECRET", "TOOLSET_PROVENANCE_DESCRIPTION_SECRET", "TOOLSET_PROVENANCE_INSTRUCTION_SECRET", "TOOLSET_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("toolset provenance report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[171], "gitclaw:done") || hasLabel(github.IssueLabels[171], "gitclaw:running") || hasLabel(github.IssueLabels[171], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[171])
	}
}
