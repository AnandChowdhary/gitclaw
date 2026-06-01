package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

const backupRehearsalIssueMarker = "gitclaw:backup-rehearsal-issue"

type BackupRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type BackupRehearsalIssueRequest struct {
	Repo              string
	Command           string
	Subcommand        string
	RehearsalID       string
	BackupIssueNumber int
	BackupBranch      string
	BackupRoot        string
	RepoBackupDir     string
	IssueBackupPath   string
	IndexPath         string
	RestorePlanCmd    string
	CoverageCmd       string
	DrillCmd          string
	SourceIssueNumber int
	SourceCommentID   int64
	SourceSHA         string
	SourceBytes       int
	SourceLines       int
	SourceKind        string
}

type BackupRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsBackupRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isBackupRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isBackupRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 || (fields[0] != "/backup" && fields[0] != "/backups") {
		return false
	}
	switch cleanBackupCommandName(fields[1]) {
	case "rehearse", "rehearsal", "restore-rehearsal", "recovery", "recover":
		return true
	default:
		return false
	}
}

func BuildBackupRehearsalIssueRequest(ev Event, cfg Config) (BackupRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isBackupRehearsalIssueFields(fields) {
		return BackupRehearsalIssueRequest{}, fmt.Errorf("missing backup rehearsal issue command")
	}
	sourceText := activeRequestText(ev)
	backupIssueNumber, rehearsalID, err := parseBackupRehearsalIssueArgs(fields[2:], ev.Issue.Number, sourceText)
	if err != nil {
		return BackupRehearsalIssueRequest{}, err
	}
	if backupIssueNumber <= 0 {
		return BackupRehearsalIssueRequest{}, fmt.Errorf("invalid backup rehearsal issue number")
	}
	if rehearsalID == "" {
		rehearsalID = cleanBackupRehearsalID(fmt.Sprintf("backup-rehearsal-%d-%s", backupIssueNumber, shortDocumentHash(sourceText)))
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return BackupRehearsalIssueRequest{}, fmt.Errorf("invalid backup rehearsal id %q", rehearsalID)
	}
	repo := backupReportRepo(ev.Repo)
	repoDir := filepath.ToSlash(backupRepoDir(defaultBackupRoot, repo))
	issuePath := filepath.ToSlash(issueBackupPath(defaultBackupRoot, repo, backupIssueNumber))
	req := BackupRehearsalIssueRequest{
		Repo:              ev.Repo,
		Command:           fields[0],
		Subcommand:        cleanBackupCommandName(fields[1]),
		RehearsalID:       rehearsalID,
		BackupIssueNumber: backupIssueNumber,
		BackupBranch:      defaultBackupBranch,
		BackupRoot:        defaultBackupRoot,
		RepoBackupDir:     repoDir,
		IssueBackupPath:   issuePath,
		IndexPath:         filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		RestorePlanCmd:    fmt.Sprintf("gitclaw backup restore-plan --root %s --repo %s --issue %d", defaultBackupRoot, repo, backupIssueNumber),
		CoverageCmd:       fmt.Sprintf("gitclaw backup coverage --root %s --repo %s --issue %d", defaultBackupRoot, repo, backupIssueNumber),
		DrillCmd:          fmt.Sprintf("gitclaw backup drill --root %s --repo %s --issue %d", defaultBackupRoot, repo, backupIssueNumber),
		SourceIssueNumber: ev.Issue.Number,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "issue",
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	return req, nil
}

func RunBackupRehearsalIssue(ctx context.Context, cfg Config, github BackupRehearsalIssueGitHubClient, req BackupRehearsalIssueRequest) (BackupRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return BackupRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return BackupRehearsalIssueResult{}, fmt.Errorf("missing backup rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return BackupRehearsalIssueResult{}, fmt.Errorf("list backup rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if backupRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return BackupRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, backupRehearsalIssueTitle(req), RenderBackupRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return BackupRehearsalIssueResult{}, fmt.Errorf("create backup rehearsal issue: %w", err)
	}
	return BackupRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderBackupRehearsalIssueBody(req BackupRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" backup_issue=\"%d\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", backupRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), req.BackupIssueNumber, req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw backup recovery rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- backup_issue: #%d\n", req.BackupIssueNumber)
	fmt.Fprintf(&b, "- backup_branch: %s\n", req.BackupBranch)
	fmt.Fprintf(&b, "- backup_root: %s\n", req.BackupRoot)
	fmt.Fprintf(&b, "- repo_backup_dir: %s\n", req.RepoBackupDir)
	fmt.Fprintf(&b, "- issue_backup_path: %s\n", req.IssueBackupPath)
	fmt.Fprintf(&b, "- index_path: %s\n", req.IndexPath)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	b.WriteString("- rehearsal_mode: recovery-conversation\n")
	b.WriteString("- restore_mode: dry-run\n")
	b.WriteString("- repository_mutation_allowed: false\n")
	b.WriteString("- backup_branch_write_allowed: false\n")
	b.WriteString("- github_api_replay_allowed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_backup_bodies_included: false\n\n")
	b.WriteString("Use this issue to rehearse recovery from the GitClaw backup branch. Fetch `gitclaw-backups` locally, run the coverage/drill/restore-plan commands below, and keep any real restore as a reviewed branch or separate recovery procedure.\n\n")
	b.WriteString("### Suggested Dry-Run Commands\n")
	fmt.Fprintf(&b, "- `%s`\n", req.CoverageCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.DrillCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.RestorePlanCmd)
	return strings.TrimSpace(b.String())
}

func RenderBackupRehearsalIssueActionReport(ev Event, req BackupRehearsalIssueRequest, result BackupRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Backup Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_backup_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- backup_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
	fmt.Fprintf(&b, "- backup_issue: `#%d`\n", req.BackupIssueNumber)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", req.BackupBranch)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", req.BackupRoot)
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", shortDocumentHash(req.RepoBackupDir))
	fmt.Fprintf(&b, "- issue_backup_path_sha256_12: `%s`\n", shortDocumentHash(req.IssueBackupPath))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", shortDocumentHash(req.IndexPath))
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run")
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing recovery from a backed-up conversation. The action does not read raw backup payloads, restore files, mutate the repository, replay GitHub API calls, or call a model; continue on the rehearsal issue after verifying the backup branch.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- fetch `gitclaw-backups` and run coverage/drill/restore-plan locally against the expected issue backup\n")
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseBackupRehearsalIssueArgs(args []string, defaultIssue int, sourceText string) (int, string, error) {
	issueNumber := defaultIssue
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
				return 0, "", fmt.Errorf("--id requires a value")
			}
			rehearsalID = cleanBackupRehearsalID(args[i])
		case "--issue", "--source-issue", "-i":
			i++
			if i >= len(args) {
				return 0, "", fmt.Errorf("--issue requires a value")
			}
			parsed, ok := parseBackupIssueNumber(args[i])
			if !ok {
				return 0, "", fmt.Errorf("invalid backup issue %q", args[i])
			}
			issueNumber = parsed
		default:
			if strings.HasPrefix(field, "--") {
				return 0, "", fmt.Errorf("unknown backup rehearsal argument %q", field)
			}
			if parsed, ok := parseBackupIssueNumber(field); ok {
				issueNumber = parsed
			}
		}
	}
	if rehearsalID == "" {
		rehearsalID = cleanBackupRehearsalID(fmt.Sprintf("backup-rehearsal-%d-%s", issueNumber, shortDocumentHash(sourceText)))
	}
	return issueNumber, rehearsalID, nil
}

func cleanBackupRehearsalID(value string) string {
	return cleanSkillRehearsalID(value)
}

func backupRehearsalIssueTitle(req BackupRehearsalIssueRequest) string {
	title := fmt.Sprintf("GitClaw backup rehearsal: issue #%d", req.BackupIssueNumber)
	if req.RehearsalID != "" {
		title += " (" + req.RehearsalID + ")"
	}
	return title
}

func backupRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+backupRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanBackupRehearsalID(rehearsalID))))
}
