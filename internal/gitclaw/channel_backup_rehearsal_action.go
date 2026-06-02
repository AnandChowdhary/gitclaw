package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type ChannelBackupRehearsalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RehearsalID       string
	BackupIssueNumber int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupRehearsalResult struct {
	Rehearsal           BackupRehearsalIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	RehearsalHash       string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupRehearsalActionRequest struct {
	Options             ChannelBackupRehearsalOptions
	Rehearsal           BackupRehearsalIssueRequest
	Command             string
	Subcommand          string
	AutoRehearsalID     bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelBackupRehearsalActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupRehearsalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupRehearsalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse-backup", "backup-rehearse", "backup-rehearsal", "rehearse-recovery", "recovery-rehearsal", "backup-drill", "recovery-drill":
		return true
	default:
		return false
	}
}

func BuildChannelBackupRehearsalActionRequest(ev Event, cfg Config) (ChannelBackupRehearsalActionRequest, error) {
	fields, ok := channelBackupRehearsalActionFields(ev, cfg)
	if !ok {
		return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("missing channel backup rehearsal command")
	}
	req := ChannelBackupRehearsalActionRequest{
		Options: ChannelBackupRehearsalOptions{
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
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--issue", "--backup-issue", "--source-issue", "-i":
			if i+1 >= len(fields) {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			parsed, ok := parseBackupIssueNumber(fields[i+1])
			if !ok {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("invalid backup issue %q", fields[i+1])
			}
			req.Options.BackupIssueNumber = parsed
			backupIssueSet = true
			i++
		case "--id", "--rehearsal-id":
			if i+1 >= len(fields) {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RehearsalID = cleanBackupRehearsalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("unknown channel backup rehearsal argument %q", field)
			}
			if parsed, ok := parseBackupIssueNumber(field); ok && !backupIssueSet {
				req.Options.BackupIssueNumber = parsed
				backupIssueSet = true
				continue
			}
			if req.Options.RehearsalID == "" {
				req.Options.RehearsalID = cleanBackupRehearsalID(field)
				continue
			}
			return ChannelBackupRehearsalActionRequest{}, fmt.Errorf("unexpected channel backup rehearsal argument %q", field)
		}
	}
	if err := applyChannelBackupRehearsalIssueTarget(ev, &req); err != nil {
		return ChannelBackupRehearsalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RehearsalID) == "" {
		req.Options.RehearsalID = autoChannelBackupRehearsalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.BackupIssueNumber)
		req.AutoRehearsalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupRehearsalNotifyMessageID(ev, req.Options.RehearsalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupRehearsalOptions(req.Options)
	if err := validateChannelBackupRehearsalOptions(req.Options); err != nil {
		return ChannelBackupRehearsalActionRequest{}, err
	}
	rehearsal := buildBackupRehearsalIssueRequestFromChannel(ev, req.Options)
	req.Rehearsal = rehearsal
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelBackupRehearsalNotificationBody(req.Options, BackupRehearsalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, rehearsal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelBackupRehearsal(ctx context.Context, cfg Config, github interface {
	BackupRehearsalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelBackupRehearsalActionRequest) (ChannelBackupRehearsalResult, error) {
	rehearsalResult, err := RunBackupRehearsalIssue(ctx, cfg, github, req.Rehearsal)
	if err != nil {
		return ChannelBackupRehearsalResult{}, err
	}
	notificationBody := RenderChannelBackupRehearsalNotificationBody(req.Options, rehearsalResult, req.Rehearsal)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelBackupRehearsalResult{}, fmt.Errorf("queue channel backup rehearsal notification: %w", err)
	}
	return ChannelBackupRehearsalResult{
		Rehearsal:           rehearsalResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		RehearsalHash:       shortDocumentHash(req.Options.RehearsalID),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelBackupRehearsalActionReport(ev Event, req ChannelBackupRehearsalActionRequest, result ChannelBackupRehearsalResult) string {
	status := "created"
	switch {
	case result.Rehearsal.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.Rehearsal.Duplicate:
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
	b.WriteString("## GitClaw Channel Backup Rehearsal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Rehearsal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Rehearsal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.Rehearsal.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.Rehearsal.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Rehearsal.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Rehearsal.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", result.RehearsalHash)
	fmt.Fprintf(&b, "- rehearsal_id_auto: `%t`\n", req.AutoRehearsalID)
	fmt.Fprintf(&b, "- backup_issue: `#%d`\n", req.Rehearsal.BackupIssueNumber)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", req.Rehearsal.BackupBranch)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", req.Rehearsal.BackupRoot)
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", shortDocumentHash(req.Rehearsal.RepoBackupDir))
	fmt.Fprintf(&b, "- issue_backup_path_sha256_12: `%s`\n", shortDocumentHash(req.Rehearsal.IssueBackupPath))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", shortDocumentHash(req.Rehearsal.IndexPath))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "recovery-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run")
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rehearsal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Rehearsal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Rehearsal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Rehearsal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_rehearsal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing recovery from a channel-origin request, then queued a provider-facing rehearsal link back to that thread. This action does not read raw backup payloads, restore files, replay GitHub API calls, copy raw channel bodies, mutate the repository, or call a model.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.Rehearsal.IssueNumber)
	b.WriteString("- fetch `gitclaw-backups` and run coverage/drill/restore-plan locally against the expected issue backup\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelBackupRehearsalNotificationBody(opts ChannelBackupRehearsalOptions, result BackupRehearsalIssueResult, rehearsal BackupRehearsalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup rehearsal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Rehearsal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Backup issue: #%d\n", rehearsal.BackupIssueNumber)
	fmt.Fprintf(&b, "Backup branch: %s\n", rehearsal.BackupBranch)
	b.WriteString("Restore mode: dry-run\n")
	b.WriteString("\nContinue in the linked GitHub issue to rehearse recovery with normal model-backed conversation and reviewed backup commands. This notification did not execute a model, restore files, replay GitHub API calls, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelBackupRehearsalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelBackupRehearsalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelBackupRehearsalIssueTarget(ev Event, req *ChannelBackupRehearsalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup rehearsal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupRehearsalOptions(opts ChannelBackupRehearsalOptions) ChannelBackupRehearsalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RehearsalID = cleanBackupRehearsalID(opts.RehearsalID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelBackupRehearsalOptions(opts ChannelBackupRehearsalOptions) error {
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
	if opts.RehearsalID == "" {
		return fmt.Errorf("missing backup rehearsal id")
	}
	if !skillNamePattern.MatchString(opts.RehearsalID) {
		return fmt.Errorf("invalid backup rehearsal id %q", opts.RehearsalID)
	}
	if opts.BackupIssueNumber <= 0 {
		return fmt.Errorf("invalid backup issue number")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildBackupRehearsalIssueRequestFromChannel(ev Event, opts ChannelBackupRehearsalOptions) BackupRehearsalIssueRequest {
	repo := backupReportRepo(ev.Repo)
	repoDir := filepath.ToSlash(backupRepoDir(defaultBackupRoot, repo))
	issuePath := filepath.ToSlash(issueBackupPath(defaultBackupRoot, repo, opts.BackupIssueNumber))
	sourceText := activeRequestText(ev)
	return BackupRehearsalIssueRequest{
		Repo:              ev.Repo,
		Command:           "/backup",
		Subcommand:        "rehearse",
		RehearsalID:       opts.RehearsalID,
		BackupIssueNumber: opts.BackupIssueNumber,
		BackupBranch:      defaultBackupBranch,
		BackupRoot:        defaultBackupRoot,
		RepoBackupDir:     repoDir,
		IssueBackupPath:   issuePath,
		IndexPath:         filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		RestorePlanCmd:    fmt.Sprintf("gitclaw backup restore-plan --root %s --repo %s --issue %d", defaultBackupRoot, repo, opts.BackupIssueNumber),
		CoverageCmd:       fmt.Sprintf("gitclaw backup coverage --root %s --repo %s --issue %d", defaultBackupRoot, repo, opts.BackupIssueNumber),
		DrillCmd:          fmt.Sprintf("gitclaw backup drill --root %s --repo %s --issue %d", defaultBackupRoot, repo, opts.BackupIssueNumber),
		SourceIssueNumber: opts.SourceIssueNumber,
		SourceCommentID:   opts.SourceCommentID,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "channel_comment",
	}
}

func autoChannelBackupRehearsalID(ev Event, channel, threadID, sourceMessageID string, backupIssueNumber int) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, fmt.Sprintf("%d", backupIssueNumber)}, "|")
	return fmt.Sprintf("backup-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBackupRehearsalNotifyMessageID(ev Event, rehearsalID string) string {
	seed := strings.Join([]string{eventID(ev), rehearsalID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
