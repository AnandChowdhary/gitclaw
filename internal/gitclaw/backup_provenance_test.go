package gitclaw

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildBackupProvenanceReportsGitHistoryWithoutBodies(t *testing.T) {
	workdir := t.TempDir()
	root := filepath.Join(workdir, ".gitclaw", "backups")
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-31T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 42,
			Title:  "@gitclaw provenance title BACKUP_PROVENANCE_TITLE_SECRET",
			Body:   "BACKUP_PROVENANCE_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "BACKUP_PROVENANCE_TRANSCRIPT_SECRET"},
			{Role: "assistant", Body: "BACKUP_PROVENANCE_ASSISTANT_SECRET"},
		},
		Comments: []IssueBackupComment{
			{ID: 12, Body: "BACKUP_PROVENANCE_COMMENT_SECRET"},
		},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 31, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	initCommittedBackupGitRepo(t, workdir)

	report, err := BuildBackupProvenance(root, "owner/repo")
	if err != nil {
		t.Fatalf("BuildBackupProvenance returned error: %v", err)
	}
	if report.BackupProvenanceStatus != "ok" || report.BackupVerifyStatus != "ok" || !report.GitAvailable || !report.GitHistoryAvailable || report.ProvenanceFiles != 3 || report.GitTrackedFiles != 3 || report.FilesWithCommits != 3 {
		t.Fatalf("unexpected backup provenance report: %#v", report)
	}
	body := RenderBackupProvenance(report)
	for _, want := range []string{
		"GitClaw Backup Provenance Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"backup_provenance_status: `ok`",
		"backup_verify_status: `ok`",
		"verification_failures: `0`",
		"expected_backup_branch: `gitclaw-backups`",
		"backup_schema_version: `1`",
		"issue_count: `1`",
		"control_files: `2`",
		"issue_payload_files: `1`",
		"provenance_files: `3`",
		"readable_files: `3`",
		"unreadable_files: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"git_tracked_files: `3`",
		"untracked_files: `0`",
		"working_tree_dirty_files: `0`",
		"files_with_commits: `3`",
		"files_without_commits: `0`",
		"raw_backup_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_backup_provenance_change: `true`",
		"### Provenance Gates",
		"verify_gate=`pass`",
		"git_history_gate=`pass`",
		"mutation_gate=`disabled`",
		"### Provenance Files",
		"kind=`index` issue=none path=`index.json`",
		"kind=`readme` issue=none path=`README.md`",
		"kind=`issue-backup` issue=#42 path=`issues/000042.json`",
		"last_commit_sha256_12=",
		"last_commit_short=",
		"subject_sha256_12=",
		"### Provenance Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("backup provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"BACKUP_PROVENANCE_TITLE_SECRET",
		"BACKUP_PROVENANCE_BODY_SECRET",
		"BACKUP_PROVENANCE_TRANSCRIPT_SECRET",
		"BACKUP_PROVENANCE_ASSISTANT_SECRET",
		"BACKUP_PROVENANCE_COMMENT_SECRET",
		"BACKUP_PROVENANCE_COMMIT_SUBJECT_SECRET",
		"Backup Provenance Secret Author",
		"provenance-secret@example.invalid",
		"@gitclaw provenance title",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("backup provenance report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestBackupProvenanceCommandReportsCommittedBackupTree(t *testing.T) {
	workdir := t.TempDir()
	root := filepath.Join(workdir, ".gitclaw", "backups")
	writeBackupFixture(t, root, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-31T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 7, Title: "@gitclaw provenance cli", Body: "BACKUP_PROVENANCE_CLI_BODY_SECRET"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "BACKUP_PROVENANCE_CLI_TRANSCRIPT_SECRET"}},
		Comments:    []IssueBackupComment{{ID: 13, Body: "BACKUP_PROVENANCE_CLI_COMMENT_SECRET"}},
	})
	if _, err := WriteBackupIndex(root, "owner/repo", time.Date(2026, 5, 31, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	initCommittedBackupGitRepo(t, workdir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"backup", "provenance", "--root", root, "--repo", "owner/repo"}); err != nil {
			t.Fatalf("backup provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Backup Provenance Report",
		"backup_provenance_status: `ok`",
		"backup_verify_status: `ok`",
		"git_available: `true`",
		"git_history_available: `true`",
		"raw_backup_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"kind=`issue-backup` issue=#7 path=`issues/000007.json`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"BACKUP_PROVENANCE_CLI_BODY_SECRET", "BACKUP_PROVENANCE_CLI_TRANSCRIPT_SECRET", "BACKUP_PROVENANCE_CLI_COMMENT_SECRET", "@gitclaw provenance cli"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("backup provenance output leaked %q:\n%s", leaked, output)
		}
	}
}

func initCommittedBackupGitRepo(t *testing.T, workdir string) {
	t.Helper()
	runBackupProvenanceGit(t, workdir, "init")
	runBackupProvenanceGit(t, workdir, "config", "user.name", "Backup Provenance Secret Author")
	runBackupProvenanceGit(t, workdir, "config", "user.email", "provenance-secret@example.invalid")
	runBackupProvenanceGit(t, workdir, "config", "commit.gpgsign", "false")
	runBackupProvenanceGit(t, workdir, "add", ".gitclaw")
	runBackupProvenanceGit(t, workdir, "-c", "commit.gpgsign=false", "commit", "-m", "BACKUP_PROVENANCE_COMMIT_SUBJECT_SECRET")
}

func runBackupProvenanceGit(t *testing.T, workdir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}
