package gitclaw

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildCheckpointReportReportsDirtyGitStateWithoutBodies(t *testing.T) {
	root := t.TempDir()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, "tracked.txt", "clean content\n")
	runCheckpointTestGit(t, root, "add", "tracked.txt")
	runCheckpointTestGit(t, root, "commit", "-m", "initial CHECKPOINT_COMMIT_SECRET")

	writeTestFile(t, root, "tracked.txt", "dirty CHECKPOINT_WORKTREE_SECRET\n")
	writeTestFile(t, root, "untracked.txt", "new CHECKPOINT_UNTRACKED_SECRET\n")

	report := BuildCheckpointReport(root)
	if report.Status != "dirty" || !report.GitAvailable || !report.GitRepository {
		t.Fatalf("unexpected checkpoint report status: %#v", report)
	}
	if report.Branch != "main" || report.HeadShortSHA == "" || report.CommitsAvailable != 1 {
		t.Fatalf("unexpected git metadata: %#v", report)
	}
	if report.StagedChanges != 0 || report.UnstagedChanges != 1 || report.UntrackedFiles != 1 || report.WorktreeClean {
		t.Fatalf("unexpected worktree counts: %#v", report)
	}
	if len(report.RecentCommits) != 1 || report.RecentCommits[0].SubjectSHA == "" {
		t.Fatalf("expected hashed recent commit subject: %#v", report.RecentCommits)
	}

	rendered := RenderCheckpointCLIReport(report)
	for _, want := range []string{
		"GitClaw Checkpoints Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"checkpoint_status: `dirty`",
		"checkpoint_strategy: `git-history-plus-backup-branch`",
		"rollback_mode: `inspect-only`",
		"git_available: `true`",
		"git_repository: `true`",
		"branch: `main`",
		"worktree_clean: `false`",
		"staged_changes: `0`",
		"unstaged_changes: `1`",
		"untracked_files: `1`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"restore_operations_enabled: `false`",
		"llm_e2e_required_after_checkpoint_report_change: `true`",
		"subject_sha256_12=",
		"gitclaw rollback list",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("checkpoint report missing %q:\n%s", want, rendered)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_WORKTREE_SECRET", "CHECKPOINT_UNTRACKED_SECRET", "dirty ", "new "} {
		if strings.Contains(rendered, notWant) {
			t.Fatalf("checkpoint report leaked %q:\n%s", notWant, rendered)
		}
	}
}

func TestRenderCheckpointCatalogReportShowsCommandAndGateSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 133,
			"title": "@gitclaw /checkpoints catalog",
			"body": "Hidden checkpoint catalog body token: CHECKPOINT_CATALOG_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderCheckpointReportWithConfig(ev, DefaultConfig(), BuildCheckpointReport(root))
	for _, want := range []string{
		"GitClaw Checkpoints Catalog Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#133`",
		"requested_checkpoints_command: `catalog`",
		"checkpoints_command_status: `ok`",
		"checkpoint_catalog_status: `ok`",
		"catalog_strategy: `compact-git-history-rollback-discovery`",
		"checkpoint_strategy: `git-history-plus-backup-branch`",
		"rollback_model: `github-actions-git-metadata-inspect-only`",
		"rollback_mode: `inspect-only`",
		"git_available: `true`",
		"git_repository: `true`",
		"branch: `main`",
		"commits_available: `1`",
		"recent_commits_returned: `1`",
		"recent_commit_limit: `5`",
		"worktree_clean: `true`",
		"catalog_entries: `8`",
		"checkpoint_layers: `7`",
		"restore_operations_enabled: `false`",
		"git_reset_allowed: `false`",
		"git_clean_allowed: `false`",
		"checkout_mutation_allowed: `false`",
		"pre_restore_snapshot_required: `true`",
		"rollback_diff_preview_required: `true`",
		"backup_manifest_required_for_restore: `true`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_checkpoint_catalog_change: `true`",
		"command=`catalog` issue_intent=`@gitclaw /checkpoints catalog` local_command=`gitclaw checkpoints catalog` execution=`metadata-only` gate=`body-free-checkpoint-command-map`",
		"command=`rollback-catalog` issue_intent=`@gitclaw /rollback catalog` local_command=`gitclaw rollback catalog`",
		"command=`rollback-risk` issue_intent=`@gitclaw /rollback risk` local_command=`gitclaw rollback risk`",
		"layer=`git-history` store=`repository .git metadata`",
		"layer=`worktree` store=`git status --porcelain`",
		"layer=`backup-branch` store=`gitclaw-backups`",
		"layer=`recent-commits` store=`git log metadata`",
		"layer=`restore-preview` store=`future rollback diff preview`",
		"layer=`operation-boundary` store=`unsupported restore/reset/clean/checkout`",
		"layer=`payloads` store=`unsupported in reports`",
		"checkpoint_catalog_gate=`ok`",
		"git_metadata_gate=`available-before-report`",
		"worktree_gate=`clean-preferred-dirty-is-warning`",
		"backup_branch_gate=`manifest-required-before-restore`",
		"rollback_preview_gate=`diff-preview-required-before-restore`",
		"restore_gate=`disabled-inspect-only-v1`",
		"destructive_git_gate=`reset-clean-checkout-disabled`",
		"raw_body_gate=`hashes-counts-and-metadata-only`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("checkpoint catalog report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_CATALOG_BODY_SECRET", "Hidden checkpoint catalog", "CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_FILE_SECRET"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("checkpoint catalog report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestCheckpointsCatalogCommandReportsSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"checkpoints", "catalog"}); err != nil {
			t.Fatalf("checkpoints catalog returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Checkpoints Catalog Report",
		"scope: `local-cli`",
		"checkpoint_catalog_status: `ok`",
		"catalog_strategy: `compact-git-history-rollback-discovery`",
		"catalog_entries: `8`",
		"checkpoint_layers: `7`",
		"command=`catalog` issue_intent=`@gitclaw /checkpoints catalog` local_command=`gitclaw checkpoints catalog`",
		"command=`rollback-catalog` issue_intent=`@gitclaw /rollback catalog` local_command=`gitclaw rollback catalog`",
		"layer=`git-history` store=`repository .git metadata`",
		"layer=`operation-boundary` store=`unsupported restore/reset/clean/checkout`",
		"restore_gate=`disabled-inspect-only-v1`",
		"raw_body_gate=`hashes-counts-and-metadata-only`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("checkpoints catalog output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_FILE_SECRET", "initial "} {
		if strings.Contains(output, notWant) {
			t.Fatalf("checkpoints catalog output leaked %q:\n%s", notWant, output)
		}
	}
}

func runCheckpointTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
