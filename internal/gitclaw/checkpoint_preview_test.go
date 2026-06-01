package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRollbackDiffCommandReportsPreviewWithoutRawDiffs(t *testing.T) {
	root := initCheckpointPreviewRepo(t)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"rollback", "diff", "HEAD~1"}); err != nil {
			t.Fatalf("rollback diff returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Rollback Preview Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"rollback_preview_status: `ok`",
		"preview_strategy: `git-diff-stat-inspect-only`",
		"rollback_mode: `preview-only`",
		"target_ref: `HEAD~1`",
		"target_commit: `",
		"head_commit: `",
		"comparison_range_sha256_12: `",
		"git_available: `true`",
		"git_repository: `true`",
		"branch: `main`",
		"worktree_clean: `true`",
		"changed_files: `2`",
		"preview_files_returned: `2`",
		"preview_file_limit: `20`",
		"added_files: `1`",
		"modified_files: `1`",
		"total_insertions: `2`",
		"total_deletions: `1`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"path_names_included: `false`",
		"path_hashes_included: `true`",
		"restore_operations_enabled: `false`",
		"git_reset_allowed: `false`",
		"checkout_mutation_allowed: `false`",
		"llm_e2e_required_after_rollback_preview_change: `true`",
		"### Preview Summary",
		"kind=`rollback-preview`",
		"### Changed File Metadata",
		"status=`A`",
		"status=`M`",
		"path_sha256_12=`",
		"raw_path_included=`false`",
		"### Preview Gates",
		"rollback_preview_gate=`ok`",
		"restore_gate=`disabled-preview-only`",
		"raw_diff_gate=`numstat-name-status-and-path-hashes-only`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("rollback diff output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{
		"- repository:",
		"- issue:",
		"PREVIEW_FILE_SECRET",
		"PREVIEW_NEW_SECRET",
		"PREVIEW_COMMIT_SECRET",
		"tracked.txt",
		"added.go",
		"second ",
		"package preview",
	} {
		if strings.Contains(output, notWant) {
			t.Fatalf("rollback diff output leaked %q:\n%s", notWant, output)
		}
	}
}

func TestHandleRollbackDiffCommandPostsPreviewWithoutLLM(t *testing.T) {
	root := initCheckpointPreviewRepo(t)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 135,
			"title": "@gitclaw /rollback diff HEAD~1",
			"body": "Hidden rollback preview handler token: ROLLBACK_PREVIEW_HANDLER_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{135: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic rollback preview command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Rollback Preview Report",
		"Generated without a model call",
		"model=\"gitclaw/checkpoints\"",
		"repository: `owner/repo`",
		"issue: `#135`",
		"requested_checkpoints_command: `preview`",
		"rollback_preview_status: `ok`",
		"target_ref: `HEAD~1`",
		"changed_files: `2`",
		"path_names_included: `false`",
		"raw_diffs_included: `false`",
		"restore_operations_enabled: `false`",
		"llm_e2e_required_after_rollback_preview_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rollback preview handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"ROLLBACK_PREVIEW_HANDLER_SECRET", "PREVIEW_FILE_SECRET", "PREVIEW_NEW_SECRET", "PREVIEW_COMMIT_SECRET", "tracked.txt", "added.go"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("rollback preview handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[135], "gitclaw:done") || hasLabel(github.IssueLabels[135], "gitclaw:running") || hasLabel(github.IssueLabels[135], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[135])
	}
}

func initCheckpointPreviewRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, "tracked.txt", "first\n")
	runCheckpointTestGit(t, root, "add", "tracked.txt")
	runCheckpointTestGit(t, root, "commit", "-m", "initial preview")
	writeTestFile(t, root, "tracked.txt", "second PREVIEW_FILE_SECRET\n")
	writeTestFile(t, root, "added.go", "package preview // PREVIEW_NEW_SECRET\n")
	runCheckpointTestGit(t, root, "add", "tracked.txt", "added.go")
	runCheckpointTestGit(t, root, "commit", "-m", "second PREVIEW_COMMIT_SECRET")
	return root
}
