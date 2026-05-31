package gitclaw

import (
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

func runCheckpointTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
