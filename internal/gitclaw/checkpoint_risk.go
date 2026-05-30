package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type CheckpointRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Name     string
	Field    string
	LineSHA  string
}

type CheckpointRiskReport struct {
	Status                                  string
	VerificationScope                       string
	CheckpointStrategy                      string
	RollbackMode                            string
	GitAvailable                            bool
	GitRepository                           bool
	WorktreeRoot                            string
	Branch                                  string
	HeadCommit                              string
	CommitsAvailable                        int
	RecentCommitsReturned                   int
	WorktreeClean                           bool
	StagedChanges                           int
	UnstagedChanges                         int
	UntrackedFiles                          int
	BackupBranch                            string
	BackupBranchLocalRef                    bool
	RecentCommitLimit                       int
	RawDiffsIncluded                        bool
	RawFileBodiesIncluded                   bool
	RawIssueBodiesIncluded                  bool
	RawCommentBodiesIncluded                bool
	CredentialValuesIncluded                bool
	RestoreOperationsEnabled                bool
	GitResetAllowed                         bool
	GitCleanAllowed                         bool
	CheckoutMutationAllowed                 bool
	ShadowStorePathIncluded                 bool
	PreRestoreSnapshotRequired              bool
	RollbackDiffPreviewRequired             bool
	BackupManifestRequiredForRestore        bool
	SurfacesWithRiskFindings                int
	Findings                                []CheckpointRiskFinding
	HighRiskFindings                        int
	WarningRiskFindings                     int
	InfoRiskFindings                        int
	LLME2ERequiredAfterCheckpointRiskChange bool
}

func RenderCheckpointRiskReport(ev Event, report CheckpointReport) string {
	return renderCheckpointRiskReport(ev, report, true)
}

func renderCheckpointRiskReport(ev Event, checkpoint CheckpointReport, includeIssue bool) string {
	report := BuildCheckpointRiskReport(checkpoint)
	var b strings.Builder
	b.WriteString("## GitClaw Checkpoint Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeCheckpointRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report checks rollback safety using git metadata already present in the checkout. It reports status counts, recent-commit hashes, risk codes, and severities only; it does not print diffs, file bodies, commit subjects, issue bodies, comments, prompts, tool outputs, credentials, or secret values, and it does not restore, reset, clean, or checkout files.\n\n")

	b.WriteString("### Git Checkpoint Risk Card\n")
	writeCheckpointGitRiskCard(&b, report)

	b.WriteString("\n### Worktree Change Risk Card\n")
	writeCheckpointWorktreeRiskCard(&b, report)

	b.WriteString("\n### Backup Branch Risk Card\n")
	writeCheckpointBackupRiskCard(&b, report)

	b.WriteString("\n### Rollback Operation Risk Card\n")
	writeCheckpointOperationRiskCard(&b, report)

	b.WriteString("\n### Recent Commit Risk Cards\n")
	if checkpoint.RecentCommitsReturned == 0 {
		b.WriteString("- kind=`checkpoint-commit` none\n")
	} else {
		for _, commit := range checkpoint.RecentCommits {
			fmt.Fprintf(&b, "- kind=`checkpoint-commit` commit=`%s` date=`%s` subject_sha256_12=`%s` raw_subject_included=`false` raw_diff_included=`false`\n", commit.ShortSHA, commit.Date, commit.SubjectSHA)
		}
	}

	b.WriteString("\n### Current Checkpoint Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-checkpoint-request` current_issue_checkpoint_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-checkpoint-request` scope=`local-cli` current_issue_checkpoint_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeCheckpointRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildCheckpointRiskReport(checkpoint CheckpointReport) CheckpointRiskReport {
	report := CheckpointRiskReport{
		Status:                                  "ok",
		VerificationScope:                       "git_checkpoint_metadata",
		CheckpointStrategy:                      "git-history-plus-backup-branch",
		RollbackMode:                            "inspect-only",
		GitAvailable:                            checkpoint.GitAvailable,
		GitRepository:                           checkpoint.GitRepository,
		WorktreeRoot:                            checkpoint.Root,
		Branch:                                  checkpoint.Branch,
		HeadCommit:                              checkpoint.HeadShortSHA,
		CommitsAvailable:                        checkpoint.CommitsAvailable,
		RecentCommitsReturned:                   checkpoint.RecentCommitsReturned,
		WorktreeClean:                           checkpoint.WorktreeClean,
		StagedChanges:                           checkpoint.StagedChanges,
		UnstagedChanges:                         checkpoint.UnstagedChanges,
		UntrackedFiles:                          checkpoint.UntrackedFiles,
		BackupBranch:                            checkpoint.BackupBranch,
		BackupBranchLocalRef:                    checkpoint.BackupBranchLocalRef,
		RecentCommitLimit:                       maxCheckpointRecentCommits,
		RawDiffsIncluded:                        checkpoint.RawDiffsIncluded,
		RawFileBodiesIncluded:                   checkpoint.RawFileBodiesIncluded,
		RawIssueBodiesIncluded:                  false,
		RawCommentBodiesIncluded:                false,
		CredentialValuesIncluded:                false,
		RestoreOperationsEnabled:                checkpoint.RestoreOperationsEnabled,
		GitResetAllowed:                         false,
		GitCleanAllowed:                         false,
		CheckoutMutationAllowed:                 false,
		ShadowStorePathIncluded:                 false,
		PreRestoreSnapshotRequired:              true,
		RollbackDiffPreviewRequired:             true,
		BackupManifestRequiredForRestore:        true,
		LLME2ERequiredAfterCheckpointRiskChange: true,
	}
	report.Findings = checkpointRiskFindings(checkpoint, report)
	sortCheckpointRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = checkpointRiskSurfaceCount(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func checkpointRiskFindings(checkpoint CheckpointReport, report CheckpointRiskReport) []CheckpointRiskFinding {
	var findings []CheckpointRiskFinding
	if !checkpoint.GitAvailable {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_git_unavailable", "git", "git-checkpoint", "git", "git_available"))
	}
	if checkpoint.GitAvailable && !checkpoint.GitRepository {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_git_repository_missing", "git", "git-checkpoint", "git", "git_repository"))
	}
	if checkpoint.ErrorReason != "" && checkpoint.ErrorReason != "not_git_repository" && checkpoint.ErrorReason != "git_not_found" {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_git_metadata_unavailable", "git", "git-checkpoint", "git", checkpoint.ErrorReason))
	}
	if checkpoint.GitRepository && checkpoint.CommitsAvailable == 0 {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_history_empty", "git", "git-checkpoint", "git", "commits_available"))
	}
	if !checkpoint.WorktreeClean {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_worktree_dirty", "worktree", "checkpoint-worktree", "worktree", "worktree_clean"))
	}
	if checkpoint.StagedChanges > 0 {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_staged_changes_present", "worktree", "checkpoint-worktree", "worktree", "staged_changes"))
	}
	if checkpoint.UnstagedChanges > 0 {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_unstaged_changes_present", "worktree", "checkpoint-worktree", "worktree", "unstaged_changes"))
	}
	if checkpoint.UntrackedFiles > 0 {
		findings = append(findings, checkpointRiskFinding("warning", "checkpoint_untracked_files_present", "worktree", "checkpoint-worktree", "worktree", "untracked_files"))
	}
	if report.RawDiffsIncluded {
		findings = append(findings, checkpointRiskFinding("high", "checkpoint_raw_diffs_included", "body-leakage", "checkpoint-operation", "rollback", "raw_diffs_included"))
	}
	if report.RawFileBodiesIncluded {
		findings = append(findings, checkpointRiskFinding("high", "checkpoint_raw_file_bodies_included", "body-leakage", "checkpoint-operation", "rollback", "raw_file_bodies_included"))
	}
	if report.RestoreOperationsEnabled {
		findings = append(findings, checkpointRiskFinding("high", "checkpoint_restore_enabled", "write-authority", "checkpoint-operation", "rollback", "restore_operations_enabled"))
	}
	if report.GitResetAllowed || report.GitCleanAllowed || report.CheckoutMutationAllowed {
		findings = append(findings, checkpointRiskFinding("high", "checkpoint_destructive_git_allowed", "write-authority", "checkpoint-operation", "rollback", "destructive_git_allowed"))
	}
	return findings
}

func writeCheckpointRiskSummary(b *strings.Builder, report CheckpointRiskReport) {
	fmt.Fprintf(b, "- checkpoint_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- checkpoint_strategy: `%s`\n", report.CheckpointStrategy)
	fmt.Fprintf(b, "- rollback_mode: `%s`\n", report.RollbackMode)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_repository: `%t`\n", report.GitRepository)
	fmt.Fprintf(b, "- worktree_root: `%s`\n", report.WorktreeRoot)
	fmt.Fprintf(b, "- branch: `%s`\n", report.Branch)
	fmt.Fprintf(b, "- head_commit: `%s`\n", report.HeadCommit)
	fmt.Fprintf(b, "- commits_available: `%d`\n", report.CommitsAvailable)
	fmt.Fprintf(b, "- recent_commits_returned: `%d`\n", report.RecentCommitsReturned)
	fmt.Fprintf(b, "- recent_commit_limit: `%d`\n", report.RecentCommitLimit)
	fmt.Fprintf(b, "- worktree_clean: `%t`\n", report.WorktreeClean)
	fmt.Fprintf(b, "- staged_changes: `%d`\n", report.StagedChanges)
	fmt.Fprintf(b, "- unstaged_changes: `%d`\n", report.UnstagedChanges)
	fmt.Fprintf(b, "- untracked_files: `%d`\n", report.UntrackedFiles)
	fmt.Fprintf(b, "- backup_branch: `%s`\n", report.BackupBranch)
	fmt.Fprintf(b, "- backup_branch_local_ref: `%t`\n", report.BackupBranchLocalRef)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- checkpoint_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- raw_diffs_included: `%t`\n", report.RawDiffsIncluded)
	fmt.Fprintf(b, "- raw_file_bodies_included: `%t`\n", report.RawFileBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- restore_operations_enabled: `%t`\n", report.RestoreOperationsEnabled)
	fmt.Fprintf(b, "- git_reset_allowed: `%t`\n", report.GitResetAllowed)
	fmt.Fprintf(b, "- git_clean_allowed: `%t`\n", report.GitCleanAllowed)
	fmt.Fprintf(b, "- checkout_mutation_allowed: `%t`\n", report.CheckoutMutationAllowed)
	fmt.Fprintf(b, "- shadow_store_path_included: `%t`\n", report.ShadowStorePathIncluded)
	fmt.Fprintf(b, "- pre_restore_snapshot_required: `%t`\n", report.PreRestoreSnapshotRequired)
	fmt.Fprintf(b, "- rollback_diff_preview_required: `%t`\n", report.RollbackDiffPreviewRequired)
	fmt.Fprintf(b, "- backup_manifest_required_for_restore: `%t`\n", report.BackupManifestRequiredForRestore)
	fmt.Fprintf(b, "- llm_e2e_required_after_checkpoint_risk_change: `%t`\n", report.LLME2ERequiredAfterCheckpointRiskChange)
}

func writeCheckpointGitRiskCard(b *strings.Builder, report CheckpointRiskReport) {
	findings := checkpointFindingsForKind(report.Findings, "git-checkpoint")
	fmt.Fprintf(b, "- kind=`git-checkpoint` git_available=`%t` git_repository=`%t` root=`%s` branch=`%s` head_commit=`%s` commits_available=`%d` recent_commits_returned=`%d` recent_commit_limit=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", report.GitAvailable, report.GitRepository, inlineCode(report.WorktreeRoot), inlineCode(report.Branch), report.HeadCommit, report.CommitsAvailable, report.RecentCommitsReturned, report.RecentCommitLimit, len(findings), checkpointRiskMaxSeverity(findings), inlineListOrNone(checkpointRiskCodes(findings)))
}

func writeCheckpointWorktreeRiskCard(b *strings.Builder, report CheckpointRiskReport) {
	findings := checkpointFindingsForKind(report.Findings, "checkpoint-worktree")
	fmt.Fprintf(b, "- kind=`checkpoint-worktree` worktree_clean=`%t` staged_changes=`%d` unstaged_changes=`%d` untracked_files=`%d` raw_diffs_included=`%t` raw_file_bodies_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", report.WorktreeClean, report.StagedChanges, report.UnstagedChanges, report.UntrackedFiles, report.RawDiffsIncluded, report.RawFileBodiesIncluded, len(findings), checkpointRiskMaxSeverity(findings), inlineListOrNone(checkpointRiskCodes(findings)))
}

func writeCheckpointBackupRiskCard(b *strings.Builder, report CheckpointRiskReport) {
	fmt.Fprintf(b, "- kind=`checkpoint-backup` backup_branch=`%s` backup_branch_local_ref=`%t` backup_manifest_required_for_restore=`%t` raw_backup_payloads_included=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n", inlineCode(report.BackupBranch), report.BackupBranchLocalRef, report.BackupManifestRequiredForRestore)
}

func writeCheckpointOperationRiskCard(b *strings.Builder, report CheckpointRiskReport) {
	findings := checkpointFindingsForKind(report.Findings, "checkpoint-operation")
	fmt.Fprintf(b, "- kind=`checkpoint-operation` rollback_mode=`%s` restore_operations_enabled=`%t` git_reset_allowed=`%t` git_clean_allowed=`%t` checkout_mutation_allowed=`%t` pre_restore_snapshot_required=`%t` rollback_diff_preview_required=`%t` shadow_store_path_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", inlineCode(report.RollbackMode), report.RestoreOperationsEnabled, report.GitResetAllowed, report.GitCleanAllowed, report.CheckoutMutationAllowed, report.PreRestoreSnapshotRequired, report.RollbackDiffPreviewRequired, report.ShadowStorePathIncluded, len(findings), checkpointRiskMaxSeverity(findings), inlineListOrNone(checkpointRiskCodes(findings)))
}

func writeCheckpointRiskFindings(b *strings.Builder, findings []CheckpointRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` field=`%s` line_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Name, finding.Field, finding.LineSHA)
	}
}

func checkpointRiskFinding(severity, code, category, kind, name, field string) CheckpointRiskFinding {
	return CheckpointRiskFinding{Severity: severity, Code: code, Category: category, Kind: kind, Name: name, Field: field, LineSHA: shortDocumentHash(kind + ":" + name + ":" + field + ":" + code)}
}

func checkpointFindingsForKind(findings []CheckpointRiskFinding, kind string) []CheckpointRiskFinding {
	var selected []CheckpointRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			selected = append(selected, finding)
		}
	}
	return selected
}

func checkpointRiskSurfaceCount(findings []CheckpointRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Name
		if key == "\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func checkpointRiskCodes(findings []CheckpointRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func checkpointRiskMaxSeverity(findings []CheckpointRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if checkpointRiskSeverityRank(finding.Severity) > checkpointRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func checkpointRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortCheckpointRiskFindings(findings []CheckpointRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := checkpointRiskSeverityRank(a.Severity), checkpointRiskSeverityRank(b.Severity); rankA != rankB {
			return rankA > rankB
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Field < b.Field
	})
}

func isCheckpointRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/checkpoints" && command != "/checkpoint" && command != "/rollback" {
		return false
	}
	return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
}
