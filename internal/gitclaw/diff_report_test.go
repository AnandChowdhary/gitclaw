package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const diffPolicyTestBody = `# Diffs

GITCLAW_DIFFS_CONTEXT_V1

DIFF_POLICY_BODY_SECRET
`

const diffSpecTestBody = `---
name: working-tree
kind: git-diff
source: git-worktree
mode: metadata-only
max_files: 200
raw_patch_allowed: false
requires_approval: true
---

# Working Tree Diff

This declarative diff record must not be printed.

DIFF_SPEC_BODY_SECRET
`

func TestRenderDiffReportAuditsGitChangesWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initDirtyDiffRepo(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 127,
			"title": "@gitclaw /diffs",
			"body": "Hidden diffs report body token: DIFFS_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderDiffReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Diffs Report",
		"Generated without a model call",
		"diff_status: `dirty`",
		"diff_policy_path: `.gitclaw/DIFFS.md`",
		"diff_policy_present: `true`",
		"diff_policy_loaded_for_model: `true`",
		"diff_specs_dir: `.gitclaw/diffs`",
		"diff_specs: `1`",
		"diff_specs_with_frontmatter: `1`",
		"diff_specs_requiring_approval: `1`",
		"diff_specs_disallowing_raw_patch: `1`",
		"git_available: `true`",
		"git_repository: `true`",
		"worktree_root: `.`",
		"branch: `main`",
		"worktree_clean: `false`",
		"changed_files: `3`",
		"staged_files: `1`",
		"unstaged_files: `1`",
		"untracked_files: `1`",
		"staged_insertions: `1`",
		"staged_deletions: `0`",
		"unstaged_insertions: `1`",
		"unstaged_deletions: `1`",
		"binary_diff_files: `0`",
		"diff_file_limit: `200`",
		"diff_files_returned: `3`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Diff Policy",
		"`.gitclaw/DIFFS.md` loaded=`true` source=`contextDocumentPaths`",
		"### Diff Specs",
		"name=`working-tree`",
		"path=`.gitclaw/diffs/working-tree.md`",
		"frontmatter=`true`",
		"kind=`git-diff`",
		"source=`git-worktree`",
		"mode=`metadata-only`",
		"max_files=`200`",
		"raw_patch_allowed=`false`",
		"requires_approval=`true`",
		"### Changed Files",
		"path=`staged.txt`",
		"path=`tracked.txt`",
		"path=`untracked.txt`",
		"### Runtime Boundary",
		"`/diffs` is inspect-only",
		"future diff rendering needs reviewed workflows",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("diff report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"DIFF_POLICY_BODY_SECRET", "DIFF_SPEC_BODY_SECRET", "DIFFS_REPORT_BODY_SECRET", "GITCLAW_DIFFS_CONTEXT_V1", "This declarative diff record", "DIFF_TRACKED_SECRET", "DIFF_STAGED_SECRET", "DIFF_UNTRACKED_SECRET"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("diff report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestDiffsSummaryCommandReportsDiffs(t *testing.T) {
	dir := t.TempDir()
	initDirtyDiffRepo(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"diffs", "summary"}); err != nil {
			t.Fatalf("diffs summary returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Diffs Report", "scope: `local-cli`", "diff_status: `dirty`", "diff_policy_loaded_for_model: `true`", "diff_specs: `1`", "changed_files: `3`", "staged_files: `1`", "unstaged_files: `1`", "untracked_files: `1`", "raw_diffs_included: `false`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diffs summary output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "DIFF_POLICY_BODY_SECRET") || strings.Contains(output, "DIFF_SPEC_BODY_SECRET") || strings.Contains(output, "DIFF_TRACKED_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("diffs summary leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleDiffsCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	initDirtyDiffRepo(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 128,
			"title": "@gitclaw /changes",
			"body": "Hidden diffs handler token: DIFFS_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{128: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic diffs command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Diffs Report", "Generated without a model call", "model=\"gitclaw/diffs\"", "diff_status: `dirty`", "diff_policy_loaded_for_model: `true`", "changed_files: `3`", "raw_diffs_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("diffs handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"DIFFS_HANDLER_BODY_SECRET", "DIFF_POLICY_BODY_SECRET", "DIFF_SPEC_BODY_SECRET", "DIFF_TRACKED_SECRET", "This declarative diff record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("diffs handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[128], "gitclaw:done") || hasLabel(github.IssueLabels[128], "gitclaw:running") || hasLabel(github.IssueLabels[128], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[128])
	}
}

func TestLoadRepoContextIncludesDiffPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, diffPolicyPath, diffPolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == diffPolicyPath {
			found = true
			if !strings.Contains(doc.Body, "DIFF_POLICY_BODY_SECRET") {
				t.Fatalf("diff policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("diff policy file was not loaded into context: %#v", ctx.Documents)
	}
}

func initDirtyDiffRepo(t *testing.T, root string) {
	t.Helper()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, "tracked.txt", "clean content\n")
	writeTestFile(t, root, diffPolicyPath, diffPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/diffs/working-tree.md", diffSpecTestBody)
	runCheckpointTestGit(t, root, "add", "tracked.txt", ".gitclaw")
	runCheckpointTestGit(t, root, "commit", "-m", "initial diff fixture")

	writeTestFile(t, root, "tracked.txt", "modified DIFF_TRACKED_SECRET\n")
	writeTestFile(t, root, "staged.txt", "staged DIFF_STAGED_SECRET\n")
	runCheckpointTestGit(t, root, "add", "staged.txt")
	writeTestFile(t, root, "untracked.txt", "untracked DIFF_UNTRACKED_SECRET\n")
}
