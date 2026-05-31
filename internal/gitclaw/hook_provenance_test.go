package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeHookProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, hookPolicyPath, `# Hooks

HOOK_PROVENANCE_POLICY_BODY_SECRET
`)
	writeTestFile(t, root, ".gitclaw/hooks/repo-snapshot.md", `---
name: repo-snapshot
events:
  - issue:opened
  - message:sent
mode: audit-only
delivery: issue-comment
requires_approval: true
---

# Repo Snapshot Hook

HOOK_PROVENANCE_SPEC_BODY_SECRET
`)
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "test@example.com")
	runTestGit(t, root, "config", "user.name", "Test User")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add hook provenance fixture HOOK_COMMIT_SUBJECT_SECRET")
}

func TestHooksProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeHookProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"hooks", "provenance"}); err != nil {
			t.Fatalf("hooks provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Hook Provenance Report",
		"scope: `local-cli`",
		"hook_provenance_status: `ok`",
		"provenance_scope: `repo-local-hook-git-history`",
		"hooks_status: `ok`",
		"hook_risk_status: `ok`",
		"hook_policy_present: `true`",
		"hook_policy_loaded_for_model: `true`",
		"hook_specs: `1`",
		"hook_specs_with_frontmatter: `1`",
		"hook_events: `2`",
		"hook_specs_requiring_approval: `1`",
		"hook_specs_audit_only: `1`",
		"executable_handlers_present: `0`",
		"provenance_surfaces: `2`",
		"git_tracked_surfaces: `2`",
		"untracked_surfaces: `0`",
		"working_tree_dirty_surfaces: `0`",
		"surfaces_with_commits: `2`",
		"surfaces_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"hook_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_hook_bodies_included: `false`",
		"raw_handler_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_hook_provenance_change: `true`",
		"### Hook Provenance Cards",
		"kind=`hook-policy` name=`hooks-policy` path=`.gitclaw/HOOKS.md`",
		"kind=`hook-spec` name=`repo-snapshot` path=`.gitclaw/hooks/repo-snapshot.md`",
		"frontmatter=`true`",
		"events=`2`",
		"mode=`audit-only`",
		"delivery=`issue-comment`",
		"requires_approval=`true`",
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
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"git_history_gate=`pass`",
		"execution_gate=`disabled`",
		"mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("hook provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"HOOK_PROVENANCE_POLICY_BODY_SECRET",
		"HOOK_PROVENANCE_SPEC_BODY_SECRET",
		"Repo Snapshot Hook",
		"HOOK_COMMIT_SUBJECT_SECRET",
		"test@example.com",
		"Test User",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("hook provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderHookReportRoutesProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeHookProvenanceGitFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 168,
			"title": "@gitclaw /hooks provenance",
			"body": "Hidden hook provenance body token: HOOK_PROVENANCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderHookReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Hook Provenance Report",
		"repository: `owner/repo`",
		"issue: `#168`",
		"hook_provenance_status: `ok`",
		"hook_policy_present: `true`",
		"hook_specs: `1`",
		"git_tracked_surfaces: `2`",
		"surfaces_with_commits: `2`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("hook provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"HOOK_PROVENANCE_ROUTE_BODY_SECRET", "HOOK_PROVENANCE_POLICY_BODY_SECRET", "HOOK_PROVENANCE_SPEC_BODY_SECRET", "HOOK_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("hook provenance routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestHandleHooksProvenanceCommandPostsGitHistoryReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeHookProvenanceGitFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 169,
			"title": "@gitclaw /hooks history",
			"body": "Hidden hook provenance body token: HOOK_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{169: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic hook provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Hook Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/hooks\"",
		"repository: `owner/repo`",
		"issue: `#169`",
		"hook_provenance_status: `ok`",
		"hook_policy_present: `true`",
		"hook_specs: `1`",
		"git_tracked_surfaces: `2`",
		"working_tree_dirty_surfaces: `0`",
		"surfaces_with_commits: `2`",
		"raw_hook_bodies_included: `false`",
		"raw_handler_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_hook_provenance_change: `true`",
		"kind=`hook-policy` name=`hooks-policy` path=`.gitclaw/HOOKS.md`",
		"kind=`hook-spec` name=`repo-snapshot` path=`.gitclaw/hooks/repo-snapshot.md`",
		"git_history_gate=`pass`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("hook provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"HOOK_PROVENANCE_HANDLER_BODY_SECRET", "HOOK_PROVENANCE_POLICY_BODY_SECRET", "HOOK_PROVENANCE_SPEC_BODY_SECRET", "HOOK_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("hook provenance report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[169], "gitclaw:done") || hasLabel(github.IssueLabels[169], "gitclaw:running") || hasLabel(github.IssueLabels[169], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[169])
	}
}
