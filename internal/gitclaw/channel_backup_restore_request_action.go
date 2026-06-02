package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type ChannelBackupRestoreRequestOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RequestID         string
	BackupIssueNumber int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupRestoreRequestResult struct {
	RestoreRequest      BackupRestoreRequestIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	RequestHash         string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupRestoreRequestActionRequest struct {
	Options             ChannelBackupRestoreRequestOptions
	RestoreRequest      BackupRestoreRequestIssueRequest
	Command             string
	Subcommand          string
	AutoRequestID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelBackupRestoreRequestActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupRestoreRequestActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupRestoreRequestActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "restore-request", "request-restore", "backup-restore-request", "restore-backup", "backup-restore", "recovery-request", "request-recovery":
		return true
	default:
		return false
	}
}

func BuildChannelBackupRestoreRequestActionRequest(ev Event, cfg Config) (ChannelBackupRestoreRequestActionRequest, error) {
	fields, ok := channelBackupRestoreRequestActionFields(ev, cfg)
	if !ok {
		return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("missing channel backup restore request command")
	}
	req := ChannelBackupRestoreRequestActionRequest{
		Options: ChannelBackupRestoreRequestOptions{
			Repo:              ev.Repo,
			BackupIssueNumber: ev.Issue.Number,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	backupIssueSet := false
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--issue", "--backup-issue", "--source-issue", "-i":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			parsed, ok := parseBackupIssueNumber(fields[i+1])
			if !ok {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("invalid backup issue %q", fields[i+1])
			}
			req.Options.BackupIssueNumber = parsed
			backupIssueSet = true
			i++
		case "--id", "--request-id", "--restore-request-id":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RequestID = cleanBackupRestoreRequestID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("unknown channel backup restore request argument %q", field)
			}
			if parsed, ok := parseBackupIssueNumber(field); ok && !backupIssueSet {
				req.Options.BackupIssueNumber = parsed
				backupIssueSet = true
				continue
			}
			if req.Options.RequestID == "" {
				req.Options.RequestID = cleanBackupRestoreRequestID(field)
				continue
			}
			return ChannelBackupRestoreRequestActionRequest{}, fmt.Errorf("unexpected channel backup restore request argument %q", field)
		}
	}
	if err := applyChannelBackupRestoreRequestIssueTarget(ev, &req); err != nil {
		return ChannelBackupRestoreRequestActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RequestID) == "" {
		req.Options.RequestID = autoChannelBackupRestoreRequestID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.BackupIssueNumber)
		req.AutoRequestID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupRestoreRequestNotifyMessageID(ev, req.Options.RequestID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupRestoreRequestOptions(req.Options)
	if err := validateChannelBackupRestoreRequestOptions(req.Options); err != nil {
		return ChannelBackupRestoreRequestActionRequest{}, err
	}
	restoreRequest := buildBackupRestoreRequestIssueRequestFromChannel(ev, req.Options)
	req.RestoreRequest = restoreRequest
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelBackupRestoreRequestNotificationBody(req.Options, BackupRestoreRequestIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, restoreRequest)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelBackupRestoreRequest(ctx context.Context, cfg Config, github interface {
	BackupRestoreRequestIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelBackupRestoreRequestActionRequest) (ChannelBackupRestoreRequestResult, error) {
	restoreResult, err := RunBackupRestoreRequestIssue(ctx, cfg, github, req.RestoreRequest)
	if err != nil {
		return ChannelBackupRestoreRequestResult{}, err
	}
	notificationBody := RenderChannelBackupRestoreRequestNotificationBody(req.Options, restoreResult, req.RestoreRequest)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelBackupRestoreRequestResult{}, fmt.Errorf("queue channel backup restore request notification: %w", err)
	}
	return ChannelBackupRestoreRequestResult{
		RestoreRequest:      restoreResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		RequestHash:         shortDocumentHash(req.Options.RequestID),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelBackupRestoreRequestActionReport(ev Event, req ChannelBackupRestoreRequestActionRequest, result ChannelBackupRestoreRequestResult) string {
	status := "created"
	switch {
	case result.RestoreRequest.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.RestoreRequest.Duplicate:
		status = "existing"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	threadHash := result.ThreadHash
	if threadHash == "" {
		threadHash = req.RequestedThreadHash
	}
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = req.NotifyMessageHash
	}
	notificationBodySHA := result.NotificationBodySHA
	if notificationBodySHA == "" {
		notificationBodySHA = req.NotificationBodySHA
	}
	notificationBytes := result.NotificationBytes
	if notificationBytes == 0 {
		notificationBytes = req.NotificationBytes
	}
	notificationLines := result.NotificationLines
	if notificationLines == 0 {
		notificationLines = req.NotificationLines
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Restore Request Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.RestoreRequest.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.RestoreRequest.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_restore_request_status: `%s`\n", status)
	fmt.Fprintf(&b, "- restore_request_issue: `#%d`\n", result.RestoreRequest.IssueNumber)
	fmt.Fprintf(&b, "- restore_request_issue_url: `%s`\n", result.RestoreRequest.IssueURL)
	fmt.Fprintf(&b, "- restore_request_issue_created: `%t`\n", result.RestoreRequest.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.RestoreRequest.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- request_id_sha256_12: `%s`\n", result.RequestHash)
	fmt.Fprintf(&b, "- request_id_auto: `%t`\n", req.AutoRequestID)
	fmt.Fprintf(&b, "- backup_issue: `#%d`\n", req.RestoreRequest.BackupIssueNumber)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", req.RestoreRequest.BackupBranch)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", req.RestoreRequest.BackupRoot)
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", shortDocumentHash(req.RestoreRequest.RepoBackupDir))
	fmt.Fprintf(&b, "- issue_backup_path_sha256_12: `%s`\n", shortDocumentHash(req.RestoreRequest.IssueBackupPath))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", shortDocumentHash(req.RestoreRequest.IndexPath))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- restore_request_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", true)
	fmt.Fprintf(&b, "- restore_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run-first")
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_request_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.RestoreRequest.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.RestoreRequest.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.RestoreRequest.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_restore_request_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a channel-origin backup restore request, then queued a provider-facing restore-review link back to that thread. This action does not read raw backup payloads, restore files, replay GitHub API calls, copy raw channel bodies, mutate the repository, or call a model.\n\n")
	b.WriteString("### Restore Review Path\n")
	fmt.Fprintf(&b, "- continue on restore request issue: `#%d`\n", result.RestoreRequest.IssueNumber)
	b.WriteString("- fetch `gitclaw-backups` and run verify/coverage/drill/restore-plan/manifest before any restore\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelBackupRestoreRequestNotificationBody(opts ChannelBackupRestoreRequestOptions, result BackupRestoreRequestIssueResult, restoreRequest BackupRestoreRequestIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup restore request\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Review issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Backup issue: #%d\n", restoreRequest.BackupIssueNumber)
	fmt.Fprintf(&b, "Target repository: %s\n", restoreRequest.TargetRepo)
	fmt.Fprintf(&b, "Backup branch: %s\n", restoreRequest.BackupBranch)
	fmt.Fprintf(&b, "Restore PR required: %t\n", true)
	fmt.Fprintf(&b, "Restore mode: %s\n", "dry-run-first")
	b.WriteString("\nContinue in the linked GitHub issue to review the restore request with normal model-backed conversation and verified backup commands. This notification did not execute a model, read raw backup bodies, restore files, replay GitHub API calls, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelBackupRestoreRequestActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelBackupRestoreRequestActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelBackupRestoreRequestIssueTarget(ev Event, req *ChannelBackupRestoreRequestActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup restore request requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupRestoreRequestOptions(opts ChannelBackupRestoreRequestOptions) ChannelBackupRestoreRequestOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RequestID = cleanBackupRestoreRequestID(opts.RequestID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelBackupRestoreRequestOptions(opts ChannelBackupRestoreRequestOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.RequestID == "" {
		return fmt.Errorf("missing backup restore request id")
	}
	if !skillNamePattern.MatchString(opts.RequestID) {
		return fmt.Errorf("invalid backup restore request id %q", opts.RequestID)
	}
	if opts.BackupIssueNumber <= 0 {
		return fmt.Errorf("invalid backup issue number")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildBackupRestoreRequestIssueRequestFromChannel(ev Event, opts ChannelBackupRestoreRequestOptions) BackupRestoreRequestIssueRequest {
	repo := backupReportRepo(ev.Repo)
	repoDir := filepath.ToSlash(backupRepoDir(defaultBackupRoot, repo))
	issuePath := filepath.ToSlash(issueBackupPath(defaultBackupRoot, repo, opts.BackupIssueNumber))
	sourceText := activeRequestText(ev)
	return BackupRestoreRequestIssueRequest{
		Repo:              ev.Repo,
		Command:           "/backup",
		Subcommand:        "restore-request",
		RequestID:         opts.RequestID,
		BackupIssueNumber: opts.BackupIssueNumber,
		TargetRepo:        repo,
		BackupBranch:      defaultBackupBranch,
		BackupRoot:        defaultBackupRoot,
		RepoBackupDir:     repoDir,
		IssueBackupPath:   issuePath,
		IndexPath:         filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		VerifyCmd:         fmt.Sprintf("gitclaw backup verify --root %s --repo %s", defaultBackupRoot, repo),
		CoverageCmd:       fmt.Sprintf("gitclaw backup coverage --root %s --repo %s --issue %d", defaultBackupRoot, repo, opts.BackupIssueNumber),
		DrillCmd:          fmt.Sprintf("gitclaw backup drill --root %s --repo %s --issue %d", defaultBackupRoot, repo, opts.BackupIssueNumber),
		RestorePlanCmd:    fmt.Sprintf("gitclaw backup restore-plan --root %s --repo %s --target-repo %s --issue %d", defaultBackupRoot, repo, repo, opts.BackupIssueNumber),
		ManifestCmd:       fmt.Sprintf("gitclaw backup manifest --root %s --repo %s --issue %d", defaultBackupRoot, repo, opts.BackupIssueNumber),
		SourceIssueNumber: opts.SourceIssueNumber,
		SourceCommentID:   opts.SourceCommentID,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "channel_comment",
	}
}

func autoChannelBackupRestoreRequestID(ev Event, channel, threadID, sourceMessageID string, backupIssueNumber int) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, fmt.Sprintf("%d", backupIssueNumber)}, "|")
	return fmt.Sprintf("backup-restore-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBackupRestoreRequestNotifyMessageID(ev Event, requestID string) string {
	seed := strings.Join([]string{eventID(ev), requestID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-restore-%s-%s", eventID(ev), shortDocumentHash(seed))
}
