package gitclaw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const maxCheckpointRecentCommits = 5

type CheckpointReport struct {
	Status                   string
	GitAvailable             bool
	GitRepository            bool
	Root                     string
	Branch                   string
	HeadSHA                  string
	HeadShortSHA             string
	CommitsAvailable         int
	RecentCommitsReturned    int
	WorktreeClean            bool
	StagedChanges            int
	UnstagedChanges          int
	UntrackedFiles           int
	BackupBranch             string
	BackupBranchLocalRef     bool
	RawDiffsIncluded         bool
	RawFileBodiesIncluded    bool
	RestoreOperationsEnabled bool
	RecentCommits            []CheckpointCommit
	ErrorReason              string
}

type CheckpointCommit struct {
	SHA        string
	ShortSHA   string
	Date       string
	SubjectSHA string
}

func IsCheckpointReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/checkpoints" || command == "/checkpoint" || command == "/rollback"
}

func BuildCheckpointReport(root string) CheckpointReport {
	if root == "" {
		root = "."
	}
	report := CheckpointReport{
		Status:                   "unavailable",
		Root:                     ".",
		BackupBranch:             defaultBackupBranch,
		RawDiffsIncluded:         false,
		RawFileBodiesIncluded:    false,
		RestoreOperationsEnabled: false,
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		report.ErrorReason = "workdir_abs_failed"
		return report
	}
	if info, err := os.Stat(absRoot); err != nil || !info.IsDir() {
		report.ErrorReason = "workdir_not_directory"
		return report
	}
	if _, err := exec.LookPath("git"); err != nil {
		report.ErrorReason = "git_not_found"
		return report
	}
	report.GitAvailable = true

	inside, err := runCheckpointGit(absRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		report.ErrorReason = "not_git_repository"
		return report
	}
	report.GitRepository = true
	report.Root = "."

	if branch, err := runCheckpointGit(absRoot, "branch", "--show-current"); err == nil && strings.TrimSpace(branch) != "" {
		report.Branch = strings.TrimSpace(branch)
	} else {
		report.Branch = "(detached)"
	}
	if head, err := runCheckpointGit(absRoot, "rev-parse", "HEAD"); err == nil {
		report.HeadSHA = strings.TrimSpace(head)
	}
	if short, err := runCheckpointGit(absRoot, "rev-parse", "--short=12", "HEAD"); err == nil {
		report.HeadShortSHA = strings.TrimSpace(short)
	}
	if count, err := runCheckpointGit(absRoot, "rev-list", "--count", "HEAD"); err == nil {
		report.CommitsAvailable, _ = strconv.Atoi(strings.TrimSpace(count))
	}
	if _, err := runCheckpointGit(absRoot, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+defaultBackupBranch); err == nil {
		report.BackupBranchLocalRef = true
	} else if _, err := runCheckpointGit(absRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+defaultBackupBranch); err == nil {
		report.BackupBranchLocalRef = true
	}

	status, err := runCheckpointGit(absRoot, "status", "--porcelain=v1")
	if err != nil {
		report.ErrorReason = "git_status_failed"
		return report
	}
	report.StagedChanges, report.UnstagedChanges, report.UntrackedFiles = checkpointStatusCounts(status)
	report.WorktreeClean = report.StagedChanges == 0 && report.UnstagedChanges == 0 && report.UntrackedFiles == 0
	report.Status = "clean"
	if !report.WorktreeClean {
		report.Status = "dirty"
	}

	report.RecentCommits = checkpointRecentCommits(absRoot, maxCheckpointRecentCommits)
	report.RecentCommitsReturned = len(report.RecentCommits)
	return report
}

func RenderCheckpointReport(ev Event, report CheckpointReport) string {
	if isCheckpointRiskRequest(ev, DefaultConfig()) {
		return RenderCheckpointRiskReport(ev, report)
	}
	return renderCheckpointReport(ev, report, true)
}

func RenderCheckpointCLIReport(report CheckpointReport) string {
	return renderCheckpointReport(Event{}, report, false)
}

func RenderCheckpointRiskCLIReport(report CheckpointReport) string {
	return renderCheckpointRiskReport(Event{}, report, false)
}

func renderCheckpointReport(ev Event, report CheckpointReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Checkpoints Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- checkpoint_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- checkpoint_strategy: `%s`\n", "git-history-plus-backup-branch")
	fmt.Fprintf(&b, "- rollback_mode: `%s`\n", "inspect-only")
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", report.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", report.Root)
	fmt.Fprintf(&b, "- branch: `%s`\n", report.Branch)
	fmt.Fprintf(&b, "- head_commit: `%s`\n", report.HeadShortSHA)
	fmt.Fprintf(&b, "- commits_available: `%d`\n", report.CommitsAvailable)
	fmt.Fprintf(&b, "- recent_commits_returned: `%d`\n", report.RecentCommitsReturned)
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", report.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: `%d`\n", report.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: `%d`\n", report.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", report.UntrackedFiles)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", report.BackupBranch)
	fmt.Fprintf(&b, "- backup_branch_local_ref: `%t`\n", report.BackupBranchLocalRef)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", report.RawDiffsIncluded)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", report.RawFileBodiesIncluded)
	fmt.Fprintf(&b, "- restore_operations_enabled: `%t`\n", report.RestoreOperationsEnabled)
	if report.ErrorReason != "" {
		fmt.Fprintf(&b, "- error_reason: `%s`\n", report.ErrorReason)
	}
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report checks rollback readiness using git metadata already present in the checkout. It does not print diffs, file contents, commit subjects, issue bodies, comments, prompts, or secret values, and it does not restore or reset files.\n\n")

	b.WriteString("### Recent Commits\n")
	if len(report.RecentCommits) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, commit := range report.RecentCommits {
			fmt.Fprintf(&b, "- commit=`%s` date=`%s` subject_sha256_12=`%s`\n", commit.ShortSHA, commit.Date, commit.SubjectSHA)
		}
	}

	b.WriteString("\n### Local Commands\n")
	b.WriteString("- `gitclaw checkpoints status`\n")
	b.WriteString("- `gitclaw checkpoints list`\n")
	b.WriteString("- `gitclaw checkpoints risk`\n")
	b.WriteString("- `gitclaw checkpoints verify`\n")
	b.WriteString("- `gitclaw rollback list`\n")
	b.WriteString("- `gitclaw rollback risk`\n")
	b.WriteString("- restore remains disabled in GitClaw v1; use pull requests, git history, and fetched backup manifests for reviewed recovery\n")
	return strings.TrimSpace(b.String())
}

func checkpointStatusCounts(status string) (staged, unstaged, untracked int) {
	for _, line := range strings.Split(strings.TrimRight(status, "\n"), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "??") {
			untracked++
			continue
		}
		if len(line) < 2 {
			continue
		}
		if line[0] != ' ' {
			staged++
		}
		if line[1] != ' ' {
			unstaged++
		}
	}
	return staged, unstaged, untracked
}

func checkpointRecentCommits(root string, limit int) []CheckpointCommit {
	if limit <= 0 {
		return nil
	}
	format := "%H%x1f%h%x1f%ad%x1f%s"
	out, err := runCheckpointGit(root, "log", fmt.Sprintf("-n%d", limit), "--date=short", "--format="+format)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	var commits []CheckpointCommit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) != 4 {
			continue
		}
		commits = append(commits, CheckpointCommit{
			SHA:        parts[0],
			ShortSHA:   parts[1],
			Date:       parts[2],
			SubjectSHA: shortDocumentHash(parts[3]),
		})
	}
	return commits
}

func runCheckpointGit(root string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(data)))
	}
	return string(data), nil
}
