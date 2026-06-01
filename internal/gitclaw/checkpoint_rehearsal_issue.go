package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const checkpointRehearsalIssueMarker = "gitclaw:checkpoint-rehearsal-issue"

type CheckpointRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type CheckpointRehearsalIssueRequest struct {
	Repo                  string
	Command               string
	Subcommand            string
	RehearsalID           string
	TargetRef             string
	TargetRefSHA          string
	TargetAllowed         bool
	CheckpointStatus      string
	GitAvailable          bool
	GitRepository         bool
	Branch                string
	HeadCommit            string
	CommitsAvailable      int
	WorktreeClean         bool
	StagedChanges         int
	UnstagedChanges       int
	UntrackedFiles        int
	BackupBranch          string
	BackupBranchLocalRef  bool
	PreviewStatus         string
	TargetCommit          string
	ComparisonRangeSHA    string
	ChangedFiles          int
	PreviewFilesReturned  int
	RestoreMode           string
	SourceIssueNumber     int
	SourceCommentID       int64
	SourceSHA             string
	SourceBytes           int
	SourceLines           int
	SourceKind            string
	CheckpointStatusCmd   string
	CheckpointPreviewCmd  string
	CheckpointRiskCmd     string
	RollbackDiffCmd       string
	RollbackRiskCmd       string
	PreviewErrorReason    string
	CheckpointErrorReason string
}

type CheckpointRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsCheckpointRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isCheckpointRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isCheckpointRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/checkpoints" && fields[0] != "/checkpoint" && fields[0] != "/rollback" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse", "rehearsal", "practice", "drill", "rollback-rehearsal":
		return true
	default:
		return false
	}
}

func BuildCheckpointRehearsalIssueRequest(ev Event, cfg Config) (CheckpointRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isCheckpointRehearsalIssueFields(fields) {
		return CheckpointRehearsalIssueRequest{}, fmt.Errorf("missing checkpoint rehearsal issue command")
	}
	sourceText := activeRequestText(ev)
	targetRef, rehearsalID, err := parseCheckpointRehearsalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return CheckpointRehearsalIssueRequest{}, err
	}
	targetRef = normalizeCheckpointPreviewTarget(targetRef)
	if !checkpointPreviewTargetAllowed(targetRef) {
		return CheckpointRehearsalIssueRequest{}, fmt.Errorf("unsafe checkpoint target ref")
	}
	if rehearsalID == "" {
		rehearsalID = cleanCheckpointRehearsalID("checkpoint-rehearsal-" + shortDocumentHash(sourceText))
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return CheckpointRehearsalIssueRequest{}, fmt.Errorf("invalid checkpoint rehearsal id %q", rehearsalID)
	}
	checkpoint := BuildCheckpointReport(cfg.Workdir)
	preview := BuildCheckpointPreviewReport(cfg.Workdir, targetRef)
	sourceKind := "issue"
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceKind = "comment"
		sourceCommentID = ev.Comment.ID
	}
	return CheckpointRehearsalIssueRequest{
		Repo:                  ev.Repo,
		Command:               fields[0],
		Subcommand:            strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RehearsalID:           rehearsalID,
		TargetRef:             targetRef,
		TargetRefSHA:          shortDocumentHash(targetRef),
		TargetAllowed:         true,
		CheckpointStatus:      checkpoint.Status,
		GitAvailable:          checkpoint.GitAvailable,
		GitRepository:         checkpoint.GitRepository,
		Branch:                checkpoint.Branch,
		HeadCommit:            checkpoint.HeadShortSHA,
		CommitsAvailable:      checkpoint.CommitsAvailable,
		WorktreeClean:         checkpoint.WorktreeClean,
		StagedChanges:         checkpoint.StagedChanges,
		UnstagedChanges:       checkpoint.UnstagedChanges,
		UntrackedFiles:        checkpoint.UntrackedFiles,
		BackupBranch:          checkpoint.BackupBranch,
		BackupBranchLocalRef:  checkpoint.BackupBranchLocalRef,
		PreviewStatus:         preview.Status,
		TargetCommit:          preview.TargetCommit,
		ComparisonRangeSHA:    preview.ComparisonRangeSHA,
		ChangedFiles:          preview.ChangedFiles,
		PreviewFilesReturned:  preview.FilesReturned,
		RestoreMode:           "rehearsal-only",
		SourceIssueNumber:     ev.Issue.Number,
		SourceCommentID:       sourceCommentID,
		SourceSHA:             shortDocumentHash(sourceText),
		SourceBytes:           len(sourceText),
		SourceLines:           lineCount(sourceText),
		SourceKind:            sourceKind,
		CheckpointStatusCmd:   "gitclaw checkpoints status",
		CheckpointPreviewCmd:  fmt.Sprintf("gitclaw checkpoints preview %s", targetRef),
		CheckpointRiskCmd:     "gitclaw checkpoints risk",
		RollbackDiffCmd:       fmt.Sprintf("gitclaw rollback diff %s", targetRef),
		RollbackRiskCmd:       "gitclaw rollback risk",
		PreviewErrorReason:    preview.ErrorReason,
		CheckpointErrorReason: checkpoint.ErrorReason,
	}, nil
}

func RunCheckpointRehearsalIssue(ctx context.Context, cfg Config, github CheckpointRehearsalIssueGitHubClient, req CheckpointRehearsalIssueRequest) (CheckpointRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return CheckpointRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return CheckpointRehearsalIssueResult{}, fmt.Errorf("missing checkpoint rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return CheckpointRehearsalIssueResult{}, fmt.Errorf("list checkpoint rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if checkpointRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return CheckpointRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, checkpointRehearsalIssueTitle(req), RenderCheckpointRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return CheckpointRehearsalIssueResult{}, fmt.Errorf("create checkpoint rehearsal issue: %w", err)
	}
	return CheckpointRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderCheckpointRehearsalIssueBody(req CheckpointRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" target_ref_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", checkpointRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), escapeMarkerValue(req.TargetRefSHA), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw checkpoint rollback rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- target_ref: %s\n", req.TargetRef)
	fmt.Fprintf(&b, "- target_ref_sha256_12: %s\n", req.TargetRefSHA)
	fmt.Fprintf(&b, "- target_allowed: %t\n", req.TargetAllowed)
	fmt.Fprintf(&b, "- checkpoint_status: %s\n", req.CheckpointStatus)
	fmt.Fprintf(&b, "- rollback_preview_status: %s\n", req.PreviewStatus)
	fmt.Fprintf(&b, "- branch: %s\n", valueOrNone(req.Branch))
	fmt.Fprintf(&b, "- head_commit: %s\n", valueOrNone(req.HeadCommit))
	fmt.Fprintf(&b, "- target_commit: %s\n", valueOrNone(req.TargetCommit))
	fmt.Fprintf(&b, "- comparison_range_sha256_12: %s\n", valueOrNone(req.ComparisonRangeSHA))
	fmt.Fprintf(&b, "- commits_available: %d\n", req.CommitsAvailable)
	fmt.Fprintf(&b, "- changed_files: %d\n", req.ChangedFiles)
	fmt.Fprintf(&b, "- preview_files_returned: %d\n", req.PreviewFilesReturned)
	fmt.Fprintf(&b, "- worktree_clean: %t\n", req.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: %d\n", req.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: %d\n", req.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: %d\n", req.UntrackedFiles)
	fmt.Fprintf(&b, "- backup_branch: %s\n", req.BackupBranch)
	fmt.Fprintf(&b, "- backup_branch_local_ref: %t\n", req.BackupBranchLocalRef)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	b.WriteString("- rehearsal_mode: rollback-conversation\n")
	b.WriteString("- restore_mode: rehearsal-only\n")
	b.WriteString("- rollback_mode: inspect-only\n")
	b.WriteString("- repository_mutation_allowed: false\n")
	b.WriteString("- git_reset_allowed: false\n")
	b.WriteString("- git_clean_allowed: false\n")
	b.WriteString("- checkout_mutation_allowed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_diffs_included: false\n")
	b.WriteString("- raw_file_bodies_included: false\n")
	if req.CheckpointErrorReason != "" {
		fmt.Fprintf(&b, "- checkpoint_error_reason: %s\n", req.CheckpointErrorReason)
	}
	if req.PreviewErrorReason != "" {
		fmt.Fprintf(&b, "- preview_error_reason: %s\n", req.PreviewErrorReason)
	}
	b.WriteString("\nUse this issue to rehearse a rollback from git metadata without mutating the repository. Keep any real rollback as a reviewed branch or pull request after inspecting preview output and backup manifests.\n\n")
	b.WriteString("### Suggested Dry-Run Commands\n")
	fmt.Fprintf(&b, "- `%s`\n", req.CheckpointStatusCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.CheckpointPreviewCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.CheckpointRiskCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.RollbackDiffCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.RollbackRiskCmd)
	return strings.TrimSpace(b.String())
}

func RenderCheckpointRehearsalIssueActionReport(ev Event, req CheckpointRehearsalIssueRequest, result CheckpointRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Checkpoint Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_checkpoints_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- checkpoint_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
	fmt.Fprintf(&b, "- target_ref_sha256_12: `%s`\n", req.TargetRefSHA)
	fmt.Fprintf(&b, "- target_allowed: `%t`\n", req.TargetAllowed)
	fmt.Fprintf(&b, "- checkpoint_status: `%s`\n", req.CheckpointStatus)
	fmt.Fprintf(&b, "- rollback_preview_status: `%s`\n", req.PreviewStatus)
	fmt.Fprintf(&b, "- git_available: `%t`\n", req.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", req.GitRepository)
	fmt.Fprintf(&b, "- branch_sha256_12: `%s`\n", shortDocumentHash(req.Branch))
	fmt.Fprintf(&b, "- head_commit: `%s`\n", req.HeadCommit)
	fmt.Fprintf(&b, "- target_commit: `%s`\n", req.TargetCommit)
	fmt.Fprintf(&b, "- comparison_range_sha256_12: `%s`\n", req.ComparisonRangeSHA)
	fmt.Fprintf(&b, "- commits_available: `%d`\n", req.CommitsAvailable)
	fmt.Fprintf(&b, "- changed_files: `%d`\n", req.ChangedFiles)
	fmt.Fprintf(&b, "- preview_files_returned: `%d`\n", req.PreviewFilesReturned)
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", req.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: `%d`\n", req.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: `%d`\n", req.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", req.UntrackedFiles)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", req.BackupBranch)
	fmt.Fprintf(&b, "- backup_branch_local_ref: `%t`\n", req.BackupBranchLocalRef)
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", req.RestoreMode)
	fmt.Fprintf(&b, "- rollback_mode: `%s`\n", "inspect-only")
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_reset_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_clean_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- checkout_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_ref_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_checkpoint_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	if req.CheckpointErrorReason != "" {
		fmt.Fprintf(&b, "- checkpoint_error_reason: `%s`\n", req.CheckpointErrorReason)
	}
	if req.PreviewErrorReason != "" {
		fmt.Fprintf(&b, "- preview_error_reason: `%s`\n", req.PreviewErrorReason)
	}
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing a rollback from git checkpoint metadata. The action does not print raw diffs, print file bodies, reset/clean/checkout files, mutate the repository, or call a model; continue on the rehearsal issue to discuss the dry-run plan.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- run checkpoint status, rollback preview, and checkpoint risk commands before any reviewed recovery branch\n")
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseCheckpointRehearsalIssueArgs(args []string, sourceText string) (string, string, error) {
	targetRef := defaultCheckpointPreviewTarget
	targetSet := false
	rehearsalID := ""
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--id":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--id requires a value")
			}
			rehearsalID = cleanCheckpointRehearsalID(args[i])
		case "--target", "--to", "--ref":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("%s requires a value", field)
			}
			targetRef = args[i]
			targetSet = true
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown checkpoint rehearsal argument %q", field)
			}
			if !targetSet {
				targetRef = field
				targetSet = true
			}
		}
	}
	if rehearsalID == "" {
		rehearsalID = cleanCheckpointRehearsalID("checkpoint-rehearsal-" + shortDocumentHash(sourceText))
	}
	return targetRef, rehearsalID, nil
}

func cleanCheckpointRehearsalID(value string) string {
	return cleanSkillRehearsalID(value)
}

func checkpointRehearsalIssueTitle(req CheckpointRehearsalIssueRequest) string {
	title := "GitClaw checkpoint rehearsal"
	if req.RehearsalID != "" {
		title += ": " + req.RehearsalID
	}
	return title
}

func checkpointRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+checkpointRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanCheckpointRehearsalID(rehearsalID))))
}
