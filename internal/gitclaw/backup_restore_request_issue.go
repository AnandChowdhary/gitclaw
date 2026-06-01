package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

const backupRestoreRequestIssueMarker = "gitclaw:backup-restore-request-issue"

type BackupRestoreRequestIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type BackupRestoreRequestIssueRequest struct {
	Repo              string
	Command           string
	Subcommand        string
	RequestID         string
	NotifyRoutes      []string
	NotifyRoutesSHA   string
	BackupIssueNumber int
	TargetRepo        string
	BackupBranch      string
	BackupRoot        string
	RepoBackupDir     string
	IssueBackupPath   string
	IndexPath         string
	VerifyCmd         string
	CoverageCmd       string
	DrillCmd          string
	RestorePlanCmd    string
	ManifestCmd       string
	SourceIssueNumber int
	SourceCommentID   int64
	SourceSHA         string
	SourceBytes       int
	SourceLines       int
	SourceKind        string
}

type BackupRestoreRequestIssueResult struct {
	IssueNumber         int
	IssueURL            string
	Created             bool
	Duplicate           bool
	ChannelNotification BackupRestoreRequestChannelNotification
}

type BackupRestoreRequestChannelNotification struct {
	Requested           bool
	Routes              int
	Queued              int
	Duplicates          int
	TargetIssuesCreated int
	MessageSHA          string
	BodySHA             string
	BodyBytes           int
	BodyLines           int
	Destinations        []ChannelBroadcastDestinationResult
}

func IsBackupRestoreRequestIssueRequest(ev Event, cfg Config) bool {
	return isBackupRestoreRequestIssueFields(activeSlashCommandFields(ev, cfg))
}

func isBackupRestoreRequestIssueFields(fields []string) bool {
	if len(fields) < 2 || (fields[0] != "/backup" && fields[0] != "/backups") {
		return false
	}
	switch cleanBackupCommandName(fields[1]) {
	case "restore-request", "request-restore", "restore-issue", "recovery-request", "request-recovery":
		return true
	default:
		return false
	}
}

func BuildBackupRestoreRequestIssueRequest(ev Event, cfg Config) (BackupRestoreRequestIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isBackupRestoreRequestIssueFields(fields) {
		return BackupRestoreRequestIssueRequest{}, fmt.Errorf("missing backup restore request issue command")
	}
	sourceText := activeRequestText(ev)
	backupIssueNumber, requestID, notifyRoutes, err := parseBackupRestoreRequestIssueArgs(fields[2:], ev.Issue.Number, sourceText)
	if err != nil {
		return BackupRestoreRequestIssueRequest{}, err
	}
	if backupIssueNumber <= 0 {
		return BackupRestoreRequestIssueRequest{}, fmt.Errorf("invalid backup restore issue number")
	}
	if requestID == "" {
		requestID = cleanBackupRestoreRequestID(fmt.Sprintf("backup-restore-%d-%s", backupIssueNumber, shortDocumentHash(sourceText)))
	}
	if !skillNamePattern.MatchString(requestID) {
		return BackupRestoreRequestIssueRequest{}, fmt.Errorf("invalid backup restore request id %q", requestID)
	}
	repo := backupReportRepo(ev.Repo)
	repoDir := filepath.ToSlash(backupRepoDir(defaultBackupRoot, repo))
	issuePath := filepath.ToSlash(issueBackupPath(defaultBackupRoot, repo, backupIssueNumber))
	req := BackupRestoreRequestIssueRequest{
		Repo:              ev.Repo,
		Command:           fields[0],
		Subcommand:        cleanBackupCommandName(fields[1]),
		RequestID:         requestID,
		NotifyRoutes:      normalizeChannelBroadcastRoutes(notifyRoutes),
		NotifyRoutesSHA:   channelBroadcastRoutesHash(notifyRoutes),
		BackupIssueNumber: backupIssueNumber,
		TargetRepo:        repo,
		BackupBranch:      defaultBackupBranch,
		BackupRoot:        defaultBackupRoot,
		RepoBackupDir:     repoDir,
		IssueBackupPath:   issuePath,
		IndexPath:         filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		VerifyCmd:         fmt.Sprintf("gitclaw backup verify --root %s --repo %s", defaultBackupRoot, repo),
		CoverageCmd:       fmt.Sprintf("gitclaw backup coverage --root %s --repo %s --issue %d", defaultBackupRoot, repo, backupIssueNumber),
		DrillCmd:          fmt.Sprintf("gitclaw backup drill --root %s --repo %s --issue %d", defaultBackupRoot, repo, backupIssueNumber),
		RestorePlanCmd:    fmt.Sprintf("gitclaw backup restore-plan --root %s --repo %s --target-repo %s --issue %d", defaultBackupRoot, repo, repo, backupIssueNumber),
		ManifestCmd:       fmt.Sprintf("gitclaw backup manifest --root %s --repo %s --issue %d", defaultBackupRoot, repo, backupIssueNumber),
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

func RunBackupRestoreRequestIssue(ctx context.Context, cfg Config, github BackupRestoreRequestIssueGitHubClient, req BackupRestoreRequestIssueRequest) (BackupRestoreRequestIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return BackupRestoreRequestIssueResult{}, err
	}
	if req.RequestID == "" {
		return BackupRestoreRequestIssueResult{}, fmt.Errorf("missing backup restore request id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return BackupRestoreRequestIssueResult{}, fmt.Errorf("list backup restore request issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if backupRestoreRequestIssueMatches(issue.Body, req.RequestID) {
			return BackupRestoreRequestIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, backupRestoreRequestIssueTitle(req), RenderBackupRestoreRequestIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return BackupRestoreRequestIssueResult{}, fmt.Errorf("create backup restore request issue: %w", err)
	}
	return BackupRestoreRequestIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RunBackupRestoreRequestChannelNotification(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req BackupRestoreRequestIssueRequest, result BackupRestoreRequestIssueResult) (BackupRestoreRequestChannelNotification, error) {
	notification := BackupRestoreRequestChannelNotification{
		Requested: len(req.NotifyRoutes) > 0,
		Routes:    len(req.NotifyRoutes),
	}
	if len(req.NotifyRoutes) == 0 {
		return notification, nil
	}
	if result.IssueNumber <= 0 {
		return notification, fmt.Errorf("missing backup restore request issue for channel notification")
	}
	body := RenderBackupRestoreRequestChannelNotificationBody(req, result)
	messageID := backupRestoreRequestChannelNotificationMessageID(req)
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, ChannelBroadcastOptions{
		Repo:      req.Repo,
		Routes:    req.NotifyRoutes,
		MessageID: messageID,
		Body:      body,
	})
	if err != nil {
		return notification, err
	}
	notification.Queued = broadcast.Queued
	notification.Duplicates = broadcast.Duplicates
	notification.TargetIssuesCreated = broadcast.Created
	notification.MessageSHA = shortDocumentHash(messageID)
	notification.BodySHA = shortDocumentHash(body)
	notification.BodyBytes = len(body)
	notification.BodyLines = lineCount(body)
	notification.Destinations = broadcast.Destinations
	return notification, nil
}

func RenderBackupRestoreRequestIssueBody(req BackupRestoreRequestIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" backup_issue=\"%d\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", backupRestoreRequestIssueMarker, escapeMarkerValue(req.RequestID), req.BackupIssueNumber, req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw backup restore request issue.\n\n")
	fmt.Fprintf(&b, "- request_id: %s\n", req.RequestID)
	fmt.Fprintf(&b, "- restore_scope: issue-thread\n")
	fmt.Fprintf(&b, "- backup_issue: #%d\n", req.BackupIssueNumber)
	fmt.Fprintf(&b, "- target_repository: %s\n", req.TargetRepo)
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
	fmt.Fprintf(&b, "- approval_required: %t\n", true)
	fmt.Fprintf(&b, "- restore_pr_required: %t\n", true)
	fmt.Fprintf(&b, "- restore_mode: %s\n", "dry-run-first")
	fmt.Fprintf(&b, "- repository_mutation_allowed: %t\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_allowed: %t\n", false)
	fmt.Fprintf(&b, "- github_api_replay_allowed: %t\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: %t\n", false)
	fmt.Fprintf(&b, "- raw_backup_bodies_included: %t\n\n", false)
	b.WriteString("Use this issue to review whether a backed-up conversation should be restored. Fetch `gitclaw-backups`, run the dry-run commands below, and keep any real restore behind an explicit human-approved pull request or separate recovery procedure.\n\n")
	b.WriteString("### Required Dry-Run Commands\n")
	fmt.Fprintf(&b, "- `%s`\n", req.VerifyCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.CoverageCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.DrillCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.RestorePlanCmd)
	fmt.Fprintf(&b, "- `%s`\n", req.ManifestCmd)
	b.WriteString("\n### Approval Gates\n")
	b.WriteString("- confirm the backup branch is fetched from the expected repository\n")
	b.WriteString("- compare restore-plan output against operator intent without printing raw backup bodies here\n")
	b.WriteString("- require a reviewed branch or separately approved recovery run before any restore or replay\n")
	return strings.TrimSpace(b.String())
}

func RenderBackupRestoreRequestIssueActionReport(ev Event, req BackupRestoreRequestIssueRequest, result BackupRestoreRequestIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Backup Restore Request Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_backup_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- backup_restore_request_status: `%s`\n", status)
	fmt.Fprintf(&b, "- restore_request_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- restore_request_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- restore_request_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- request_id_sha256_12: `%s`\n", shortDocumentHash(req.RequestID))
	fmt.Fprintf(&b, "- channel_notification_requested: `%t`\n", result.ChannelNotification.Requested)
	fmt.Fprintf(&b, "- channel_notification_routes: `%d`\n", result.ChannelNotification.Routes)
	fmt.Fprintf(&b, "- channel_notification_queued: `%d`\n", result.ChannelNotification.Queued)
	fmt.Fprintf(&b, "- channel_notification_duplicates: `%d`\n", result.ChannelNotification.Duplicates)
	fmt.Fprintf(&b, "- channel_notification_target_issues_created: `%d`\n", result.ChannelNotification.TargetIssuesCreated)
	fmt.Fprintf(&b, "- channel_notification_routes_sha256_12: `%s`\n", noneIfEmpty(req.NotifyRoutesSHA))
	fmt.Fprintf(&b, "- channel_notification_message_id_sha256_12: `%s`\n", noneIfEmpty(result.ChannelNotification.MessageSHA))
	fmt.Fprintf(&b, "- channel_notification_body_sha256_12: `%s`\n", noneIfEmpty(result.ChannelNotification.BodySHA))
	fmt.Fprintf(&b, "- channel_notification_body_bytes: `%d`\n", result.ChannelNotification.BodyBytes)
	fmt.Fprintf(&b, "- channel_notification_body_lines: `%d`\n", result.ChannelNotification.BodyLines)
	fmt.Fprintf(&b, "- backup_issue: `#%d`\n", req.BackupIssueNumber)
	fmt.Fprintf(&b, "- target_repository: `%s`\n", req.TargetRepo)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", req.BackupBranch)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", req.BackupRoot)
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", shortDocumentHash(req.RepoBackupDir))
	fmt.Fprintf(&b, "- issue_backup_path_sha256_12: `%s`\n", shortDocumentHash(req.IssueBackupPath))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", shortDocumentHash(req.IndexPath))
	fmt.Fprintf(&b, "- restore_request_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", true)
	fmt.Fprintf(&b, "- restore_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run-first")
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_routes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_notification_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_restore_request_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a possible backup restore. The action does not read raw backup payloads, restore files, mutate the repository, write the backup branch, replay GitHub API calls, or call a model; continue on the restore request issue after verifying the backup branch.\n\n")
	b.WriteString("### Restore Request Path\n")
	fmt.Fprintf(&b, "- continue on restore request issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- fetch `gitclaw-backups`, run verify/coverage/drill/restore-plan/manifest, and keep any real restore behind explicit human approval\n")
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	if result.ChannelNotification.Requested {
		b.WriteString("\n### Channel Notifications\n")
		if len(result.ChannelNotification.Destinations) == 0 {
			b.WriteString("- none\n")
		} else {
			for _, destination := range result.ChannelNotification.Destinations {
				fmt.Fprintf(
					&b,
					"- destination=`%02d` target_issue=`#%d` outbound_comment_id=`%d` target_issue_created=`%t` duplicate_suppressed=`%t` route_sha256_12=`%s` channel=`%s` thread_id_sha256_12=`%s` message_id_sha256_12=`%s` body_sha256_12=`%s`\n",
					destination.Index,
					destination.IssueNumber,
					destination.CommentID,
					destination.Created,
					destination.Duplicate,
					noneIfEmpty(destination.RouteHash),
					destination.Channel,
					noneIfEmpty(destination.ThreadHash),
					noneIfEmpty(destination.MessageHash),
					noneIfEmpty(destination.BodyHash),
				)
			}
		}
		b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	}
	return strings.TrimSpace(b.String())
}

func parseBackupRestoreRequestIssueArgs(args []string, defaultIssue int, sourceText string) (int, string, []string, error) {
	issueNumber := defaultIssue
	requestID := ""
	var notifyRoutes []string
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--id", "--request-id":
			i++
			if i >= len(args) {
				return 0, "", nil, fmt.Errorf("%s requires a value", field)
			}
			requestID = cleanBackupRestoreRequestID(args[i])
		case "--issue", "--source-issue", "-i":
			i++
			if i >= len(args) {
				return 0, "", nil, fmt.Errorf("--issue requires a value")
			}
			parsed, ok := parseBackupIssueNumber(args[i])
			if !ok {
				return 0, "", nil, fmt.Errorf("invalid backup issue %q", args[i])
			}
			issueNumber = parsed
		case "--notify-route", "--notify-routes", "--channel-route", "--channel-routes":
			i++
			if i >= len(args) {
				return 0, "", nil, fmt.Errorf("%s requires a value", field)
			}
			notifyRoutes = append(notifyRoutes, splitChannelBroadcastRoutes(args[i])...)
		default:
			if strings.HasPrefix(field, "--") {
				return 0, "", nil, fmt.Errorf("unknown backup restore request argument %q", field)
			}
			if parsed, ok := parseBackupIssueNumber(field); ok {
				issueNumber = parsed
			}
		}
	}
	if requestID == "" {
		requestID = cleanBackupRestoreRequestID(fmt.Sprintf("backup-restore-%d-%s", issueNumber, shortDocumentHash(sourceText)))
	}
	return issueNumber, requestID, normalizeChannelBroadcastRoutes(notifyRoutes), nil
}

func RenderBackupRestoreRequestChannelNotificationBody(req BackupRestoreRequestIssueRequest, result BackupRestoreRequestIssueResult) string {
	var b strings.Builder
	b.WriteString("GitClaw backup restore request\n\n")
	fmt.Fprintf(&b, "Review issue: #%d %s\n", result.IssueNumber, result.IssueURL)
	fmt.Fprintf(&b, "Source issue: #%d %s\n", req.SourceIssueNumber, issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "Request id: %s\n", req.RequestID)
	fmt.Fprintf(&b, "Backup issue: #%d\n", req.BackupIssueNumber)
	fmt.Fprintf(&b, "Target repository: %s\n", req.TargetRepo)
	fmt.Fprintf(&b, "Backup branch: %s\n", req.BackupBranch)
	fmt.Fprintf(&b, "Restore PR required: %t\n", true)
	fmt.Fprintf(&b, "Restore mode: %s\n", "dry-run-first")
	b.WriteString("\nReview the GitHub restore request issue, fetch the backup branch, and run the dry-run restore checks before any human-approved recovery. This notification did not call a model, read raw backup bodies, restore files, replay GitHub API calls, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func backupRestoreRequestChannelNotificationMessageID(req BackupRestoreRequestIssueRequest) string {
	return fmt.Sprintf("gitclaw-backup-restore-request-%s", req.RequestID)
}

func cleanBackupRestoreRequestID(value string) string {
	return cleanBackupRehearsalID(value)
}

func backupRestoreRequestIssueTitle(req BackupRestoreRequestIssueRequest) string {
	return fmt.Sprintf("GitClaw backup restore request: #%d", req.BackupIssueNumber)
}

func backupRestoreRequestIssueMatches(body, requestID string) bool {
	return strings.Contains(body, "<!-- "+backupRestoreRequestIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(requestID)))
}
