package gitclaw

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultCheckpointPreviewTarget = "HEAD~1"
	maxCheckpointPreviewFiles      = 20
)

type CheckpointPreviewReport struct {
	Status                                   string
	PreviewStrategy                          string
	RollbackMode                             string
	GitAvailable                             bool
	GitRepository                            bool
	WorktreeRoot                             string
	Branch                                   string
	HeadCommit                               string
	TargetRef                                string
	TargetCommit                             string
	ComparisonRangeSHA                       string
	WorktreeClean                            bool
	StagedChanges                            int
	UnstagedChanges                          int
	UntrackedFiles                           int
	ChangedFiles                             int
	FilesReturned                            int
	FileLimit                                int
	AddedFiles                               int
	ModifiedFiles                            int
	DeletedFiles                             int
	RenamedFiles                             int
	CopiedFiles                              int
	BinaryFiles                              int
	TotalInsertions                          int
	TotalDeletions                           int
	RawDiffsIncluded                         bool
	RawFileBodiesIncluded                    bool
	RawIssueBodiesIncluded                   bool
	RawCommentBodiesIncluded                 bool
	RawPromptBodiesIncluded                  bool
	RawToolOutputsIncluded                   bool
	CredentialValuesIncluded                 bool
	PathNamesIncluded                        bool
	PathHashesIncluded                       bool
	RestoreOperationsEnabled                 bool
	GitResetAllowed                          bool
	GitCleanAllowed                          bool
	CheckoutMutationAllowed                  bool
	PreRestoreSnapshotRequired               bool
	BackupManifestRequiredForRestore         bool
	LLME2ERequiredAfterRollbackPreviewChange bool
	Files                                    []CheckpointPreviewFile
	ErrorReason                              string
}

type CheckpointPreviewFile struct {
	Index        int
	Status       string
	PathSHA      string
	OldPathSHA   string
	Extension    string
	Additions    int
	Deletions    int
	Binary       bool
	RawPathShown bool
}

func RenderCheckpointPreviewReport(ev Event, cfg Config, target string) string {
	return renderCheckpointPreviewReport(ev, BuildCheckpointPreviewReport(cfg.Workdir, target), true)
}

func RenderCheckpointPreviewCLIReport(root, target string) string {
	return renderCheckpointPreviewReport(Event{}, BuildCheckpointPreviewReport(root, target), false)
}

func BuildCheckpointPreviewReport(root, target string) CheckpointPreviewReport {
	if root == "" {
		root = "."
	}
	target = normalizeCheckpointPreviewTarget(target)
	report := CheckpointPreviewReport{
		Status:                                   "unavailable",
		PreviewStrategy:                          "git-diff-stat-inspect-only",
		RollbackMode:                             "preview-only",
		WorktreeRoot:                             ".",
		TargetRef:                                target,
		FileLimit:                                maxCheckpointPreviewFiles,
		RawDiffsIncluded:                         false,
		RawFileBodiesIncluded:                    false,
		RawIssueBodiesIncluded:                   false,
		RawCommentBodiesIncluded:                 false,
		RawPromptBodiesIncluded:                  false,
		RawToolOutputsIncluded:                   false,
		CredentialValuesIncluded:                 false,
		PathNamesIncluded:                        false,
		PathHashesIncluded:                       true,
		RestoreOperationsEnabled:                 false,
		GitResetAllowed:                          false,
		GitCleanAllowed:                          false,
		CheckoutMutationAllowed:                  false,
		PreRestoreSnapshotRequired:               true,
		BackupManifestRequiredForRestore:         true,
		LLME2ERequiredAfterRollbackPreviewChange: true,
	}
	if !checkpointPreviewTargetAllowed(target) {
		report.ErrorReason = "unsafe_target_ref"
		return report
	}
	base := BuildCheckpointReport(root)
	report.GitAvailable = base.GitAvailable
	report.GitRepository = base.GitRepository
	report.WorktreeRoot = base.Root
	report.Branch = base.Branch
	report.HeadCommit = base.HeadShortSHA
	report.WorktreeClean = base.WorktreeClean
	report.StagedChanges = base.StagedChanges
	report.UnstagedChanges = base.UnstagedChanges
	report.UntrackedFiles = base.UntrackedFiles
	if !base.GitAvailable {
		report.ErrorReason = "git_not_found"
		return report
	}
	if !base.GitRepository {
		report.ErrorReason = "not_git_repository"
		return report
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		report.ErrorReason = "workdir_abs_failed"
		return report
	}
	targetCommit, err := runCheckpointGit(absRoot, "rev-parse", "--verify", target+"^{commit}")
	if err != nil || strings.TrimSpace(targetCommit) == "" {
		report.ErrorReason = "target_ref_unavailable"
		return report
	}
	report.TargetCommit = strings.TrimSpace(targetCommit)
	if short, err := runCheckpointGit(absRoot, "rev-parse", "--short=12", report.TargetCommit); err == nil {
		report.TargetCommit = strings.TrimSpace(short)
	}
	if report.HeadCommit == "" {
		if short, err := runCheckpointGit(absRoot, "rev-parse", "--short=12", "HEAD"); err == nil {
			report.HeadCommit = strings.TrimSpace(short)
		}
	}
	report.ComparisonRangeSHA = shortDocumentHash(target + "..HEAD")

	numstat, err := runCheckpointGit(absRoot, "diff", "--numstat", "--find-renames", target, "HEAD")
	if err != nil {
		report.ErrorReason = "git_diff_numstat_failed"
		return report
	}
	nameStatus, err := runCheckpointGit(absRoot, "diff", "--name-status", "--find-renames", target, "HEAD")
	if err != nil {
		report.ErrorReason = "git_diff_name_status_failed"
		return report
	}
	files := parseCheckpointPreviewFiles(numstat, nameStatus)
	report.ChangedFiles = len(files)
	for _, file := range files {
		if file.Binary {
			report.BinaryFiles++
		} else {
			report.TotalInsertions += file.Additions
			report.TotalDeletions += file.Deletions
		}
		switch checkpointPreviewStatusFamily(file.Status) {
		case "A":
			report.AddedFiles++
		case "M":
			report.ModifiedFiles++
		case "D":
			report.DeletedFiles++
		case "R":
			report.RenamedFiles++
		case "C":
			report.CopiedFiles++
		}
	}
	if len(files) > maxCheckpointPreviewFiles {
		files = files[:maxCheckpointPreviewFiles]
	}
	report.Files = files
	report.FilesReturned = len(files)
	report.Status = "ok"
	if !report.WorktreeClean {
		report.Status = "warn"
	}
	return report
}

func renderCheckpointPreviewReport(ev Event, report CheckpointPreviewReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Rollback Preview Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_checkpoints_command: `%s`\n", "preview")
		fmt.Fprintf(&b, "- checkpoints_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_git_diff_stat")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- rollback_preview_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- preview_strategy: `%s`\n", report.PreviewStrategy)
	fmt.Fprintf(&b, "- rollback_mode: `%s`\n", report.RollbackMode)
	fmt.Fprintf(&b, "- target_ref: `%s`\n", inlineCode(report.TargetRef))
	fmt.Fprintf(&b, "- target_commit: `%s`\n", report.TargetCommit)
	fmt.Fprintf(&b, "- head_commit: `%s`\n", report.HeadCommit)
	fmt.Fprintf(&b, "- comparison_range_sha256_12: `%s`\n", report.ComparisonRangeSHA)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", report.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", inlineCode(report.WorktreeRoot))
	fmt.Fprintf(&b, "- branch: `%s`\n", inlineCode(report.Branch))
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", report.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: `%d`\n", report.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: `%d`\n", report.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", report.UntrackedFiles)
	fmt.Fprintf(&b, "- changed_files: `%d`\n", report.ChangedFiles)
	fmt.Fprintf(&b, "- preview_files_returned: `%d`\n", report.FilesReturned)
	fmt.Fprintf(&b, "- preview_file_limit: `%d`\n", report.FileLimit)
	fmt.Fprintf(&b, "- added_files: `%d`\n", report.AddedFiles)
	fmt.Fprintf(&b, "- modified_files: `%d`\n", report.ModifiedFiles)
	fmt.Fprintf(&b, "- deleted_files: `%d`\n", report.DeletedFiles)
	fmt.Fprintf(&b, "- renamed_files: `%d`\n", report.RenamedFiles)
	fmt.Fprintf(&b, "- copied_files: `%d`\n", report.CopiedFiles)
	fmt.Fprintf(&b, "- binary_files: `%d`\n", report.BinaryFiles)
	fmt.Fprintf(&b, "- total_insertions: `%d`\n", report.TotalInsertions)
	fmt.Fprintf(&b, "- total_deletions: `%d`\n", report.TotalDeletions)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", report.RawDiffsIncluded)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", report.RawFileBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(&b, "- path_names_included: `%t`\n", report.PathNamesIncluded)
	fmt.Fprintf(&b, "- path_hashes_included: `%t`\n", report.PathHashesIncluded)
	fmt.Fprintf(&b, "- restore_operations_enabled: `%t`\n", report.RestoreOperationsEnabled)
	fmt.Fprintf(&b, "- git_reset_allowed: `%t`\n", report.GitResetAllowed)
	fmt.Fprintf(&b, "- git_clean_allowed: `%t`\n", report.GitCleanAllowed)
	fmt.Fprintf(&b, "- checkout_mutation_allowed: `%t`\n", report.CheckoutMutationAllowed)
	fmt.Fprintf(&b, "- pre_restore_snapshot_required: `%t`\n", report.PreRestoreSnapshotRequired)
	fmt.Fprintf(&b, "- backup_manifest_required_for_restore: `%t`\n", report.BackupManifestRequiredForRestore)
	fmt.Fprintf(&b, "- llm_e2e_required_after_rollback_preview_change: `%t`\n", report.LLME2ERequiredAfterRollbackPreviewChange)
	if report.ErrorReason != "" {
		fmt.Fprintf(&b, "- error_reason: `%s`\n", report.ErrorReason)
	}
	b.WriteByte('\n')
	b.WriteString("This preview is GitClaw's inspect-only adaptation of Hermes `/rollback diff`: it compares a target git ref to `HEAD` and reports diff counts plus path hashes, not raw patches or file bodies. It never restores, resets, cleans, checks out, or mutates files.\n\n")

	b.WriteString("### Preview Summary\n")
	fmt.Fprintf(&b, "- kind=`rollback-preview` target_ref=`%s` target_commit=`%s` head_commit=`%s` comparison_range_sha256_12=`%s` changed_files=`%d` insertions=`%d` deletions=`%d` binary_files=`%d` raw_diff_included=`false` mutation_allowed=`false`\n",
		inlineCode(report.TargetRef),
		report.TargetCommit,
		report.HeadCommit,
		report.ComparisonRangeSHA,
		report.ChangedFiles,
		report.TotalInsertions,
		report.TotalDeletions,
		report.BinaryFiles,
	)

	b.WriteString("\n### Changed File Metadata\n")
	if len(report.Files) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, file := range report.Files {
			fmt.Fprintf(&b, "- index=`%d` status=`%s` path_sha256_12=`%s` old_path_sha256_12=`%s` extension=`%s` additions=`%d` deletions=`%d` binary=`%t` raw_path_included=`%t`\n",
				file.Index,
				file.Status,
				file.PathSHA,
				inlineCodeOrNone(file.OldPathSHA),
				inlineCode(file.Extension),
				file.Additions,
				file.Deletions,
				file.Binary,
				file.RawPathShown,
			)
		}
	}

	b.WriteString("\n### Preview Gates\n")
	fmt.Fprintf(&b, "- rollback_preview_gate=`%s`\n", report.Status)
	b.WriteString("- target_ref_gate=`resolved-before-preview`\n")
	b.WriteString("- worktree_gate=`clean-required-before-future-restore`\n")
	b.WriteString("- pre_restore_snapshot_gate=`required-before-restore`\n")
	b.WriteString("- backup_manifest_gate=`required-before-restore`\n")
	b.WriteString("- restore_gate=`disabled-preview-only`\n")
	b.WriteString("- destructive_git_gate=`reset-clean-checkout-disabled`\n")
	b.WriteString("- raw_diff_gate=`numstat-name-status-and-path-hashes-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func parseCheckpointPreviewFiles(numstat, nameStatus string) []CheckpointPreviewFile {
	numLines := nonEmptyLines(numstat)
	statusLines := nonEmptyLines(nameStatus)
	maxLen := len(numLines)
	if len(statusLines) > maxLen {
		maxLen = len(statusLines)
	}
	files := make([]CheckpointPreviewFile, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		file := CheckpointPreviewFile{Index: i + 1, Status: "unknown", Extension: "none"}
		var path string
		if i < len(statusLines) {
			status, newPath, oldPath := parseCheckpointPreviewNameStatus(statusLines[i])
			file.Status = status
			path = newPath
			if oldPath != "" {
				file.OldPathSHA = shortDocumentHash(oldPath)
			}
		}
		if i < len(numLines) {
			additions, deletions, binary, numPath := parseCheckpointPreviewNumstat(numLines[i])
			file.Additions = additions
			file.Deletions = deletions
			file.Binary = binary
			if path == "" {
				path = numPath
			}
		}
		if path != "" {
			file.PathSHA = shortDocumentHash(path)
			file.Extension = checkpointPreviewExtension(path)
		}
		files = append(files, file)
	}
	return files
}

func parseCheckpointPreviewNameStatus(line string) (status, newPath, oldPath string) {
	parts := strings.Split(line, "\t")
	if len(parts) == 0 {
		return "unknown", "", ""
	}
	status = parts[0]
	if len(parts) >= 3 && (strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")) {
		return status, parts[2], parts[1]
	}
	if len(parts) >= 2 {
		return status, parts[1], ""
	}
	return status, "", ""
}

func parseCheckpointPreviewNumstat(line string) (additions, deletions int, binary bool, path string) {
	parts := strings.Split(line, "\t")
	if len(parts) < 3 {
		return 0, 0, false, ""
	}
	if parts[0] == "-" || parts[1] == "-" {
		binary = true
	} else {
		additions, _ = strconv.Atoi(parts[0])
		deletions, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 4 {
		path = parts[len(parts)-1]
	} else {
		path = parts[2]
	}
	return additions, deletions, binary, path
}

func requestedCheckpointPreviewTarget(ev Event, cfg Config) string {
	target, _ := checkpointPreviewTargetFromFields(activeSlashCommandFields(ev, cfg))
	return target
}

func isCheckpointPreviewRequest(ev Event, cfg Config) bool {
	_, ok := checkpointPreviewTargetFromFields(activeSlashCommandFields(ev, cfg))
	return ok
}

func checkpointPreviewTargetFromFields(fields []string) (string, bool) {
	if len(fields) < 2 {
		return "", false
	}
	command := fields[0]
	if command != "/checkpoints" && command != "/checkpoint" && command != "/rollback" {
		return "", false
	}
	switch strings.ToLower(fields[1]) {
	case "preview", "diff", "plan":
		if len(fields) >= 4 && (fields[2] == "--to" || fields[2] == "--ref" || fields[2] == "target") {
			return fields[3], true
		}
		if len(fields) >= 3 {
			return fields[2], true
		}
		return "", true
	default:
		return "", false
	}
}

func normalizeCheckpointPreviewTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return defaultCheckpointPreviewTarget
	}
	return target
}

func checkpointPreviewTargetAllowed(target string) bool {
	if target == "" || strings.HasPrefix(target, "-") {
		return false
	}
	if strings.ContainsAny(target, "\x00\r\n\t ") {
		return false
	}
	return true
}

func checkpointPreviewStatusFamily(status string) string {
	if status == "" {
		return ""
	}
	switch status[0] {
	case 'A', 'M', 'D', 'R', 'C':
		return string(status[0])
	default:
		return "M"
	}
}

func checkpointPreviewExtension(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return "none"
	}
	return ext
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
