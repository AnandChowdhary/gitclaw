package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelCheckpointRehearsalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RehearsalID       string
	TargetRef         string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelCheckpointRehearsalResult struct {
	Rehearsal           CheckpointRehearsalIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	RehearsalHash       string
	TargetRefHash       string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelCheckpointRehearsalActionRequest struct {
	Options             ChannelCheckpointRehearsalOptions
	Rehearsal           CheckpointRehearsalIssueRequest
	Command             string
	Subcommand          string
	AutoRehearsalID     bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	TargetRefHash       string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelCheckpointRehearsalActionRequest(ev Event, cfg Config) bool {
	return isChannelCheckpointRehearsalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelCheckpointRehearsalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse-checkpoint", "checkpoint-rehearse", "checkpoint-rehearsal", "rehearse-rollback", "rollback-rehearsal", "rollback-drill", "checkpoint-drill", "rollback-lab", "checkpoint-lab":
		return true
	default:
		return false
	}
}

func BuildChannelCheckpointRehearsalActionRequest(ev Event, cfg Config) (ChannelCheckpointRehearsalActionRequest, error) {
	fields, ok := channelCheckpointRehearsalActionFields(ev, cfg)
	if !ok {
		return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("missing channel checkpoint rehearsal command")
	}
	req := ChannelCheckpointRehearsalActionRequest{
		Options: ChannelCheckpointRehearsalOptions{
			Repo:              ev.Repo,
			TargetRef:         defaultCheckpointPreviewTarget,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	targetSet := false
	for i := 2; i < len(fields); i++ {
		field := strings.TrimSpace(fields[i])
		if field == "" {
			continue
		}
		switch field {
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--target", "--to", "--ref":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetRef = fields[i+1]
			targetSet = true
			i++
		case "--id", "--rehearsal-id":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RehearsalID = cleanCheckpointRehearsalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("unknown channel checkpoint rehearsal argument %q", field)
			}
			if !targetSet {
				req.Options.TargetRef = field
				targetSet = true
				continue
			}
			if req.Options.RehearsalID == "" {
				req.Options.RehearsalID = cleanCheckpointRehearsalID(field)
				continue
			}
			return ChannelCheckpointRehearsalActionRequest{}, fmt.Errorf("unexpected channel checkpoint rehearsal argument %q", field)
		}
	}
	if err := applyChannelCheckpointRehearsalIssueTarget(ev, &req); err != nil {
		return ChannelCheckpointRehearsalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RehearsalID) == "" {
		req.Options.RehearsalID = autoChannelCheckpointRehearsalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.TargetRef)
		req.AutoRehearsalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelCheckpointRehearsalNotifyMessageID(ev, req.Options.RehearsalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelCheckpointRehearsalOptions(req.Options)
	if err := validateChannelCheckpointRehearsalOptions(req.Options); err != nil {
		return ChannelCheckpointRehearsalActionRequest{}, err
	}
	rehearsal := buildCheckpointRehearsalIssueRequestFromChannel(ev, cfg, req.Options)
	req.Rehearsal = rehearsal
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.TargetRefHash = shortDocumentHash(req.Options.TargetRef)
	notificationBody := RenderChannelCheckpointRehearsalNotificationBody(req.Options, CheckpointRehearsalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, rehearsal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelCheckpointRehearsal(ctx context.Context, cfg Config, github interface {
	CheckpointRehearsalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelCheckpointRehearsalActionRequest) (ChannelCheckpointRehearsalResult, error) {
	rehearsalResult, err := RunCheckpointRehearsalIssue(ctx, cfg, github, req.Rehearsal)
	if err != nil {
		return ChannelCheckpointRehearsalResult{}, err
	}
	notificationBody := RenderChannelCheckpointRehearsalNotificationBody(req.Options, rehearsalResult, req.Rehearsal)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelCheckpointRehearsalResult{}, fmt.Errorf("queue channel checkpoint rehearsal notification: %w", err)
	}
	return ChannelCheckpointRehearsalResult{
		Rehearsal:           rehearsalResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		RehearsalHash:       shortDocumentHash(req.Options.RehearsalID),
		TargetRefHash:       shortDocumentHash(req.Options.TargetRef),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelCheckpointRehearsalActionReport(ev Event, req ChannelCheckpointRehearsalActionRequest, result ChannelCheckpointRehearsalResult) string {
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
	targetHash := result.TargetRefHash
	if targetHash == "" {
		targetHash = req.TargetRefHash
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
	b.WriteString("## GitClaw Channel Checkpoint Rehearsal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Rehearsal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Rehearsal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_checkpoint_rehearsal_status: `%s`\n", status)
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
	fmt.Fprintf(&b, "- target_ref_sha256_12: `%s`\n", targetHash)
	fmt.Fprintf(&b, "- target_allowed: `%t`\n", req.Rehearsal.TargetAllowed)
	fmt.Fprintf(&b, "- checkpoint_status: `%s`\n", req.Rehearsal.CheckpointStatus)
	fmt.Fprintf(&b, "- rollback_preview_status: `%s`\n", req.Rehearsal.PreviewStatus)
	fmt.Fprintf(&b, "- git_available: `%t`\n", req.Rehearsal.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", req.Rehearsal.GitRepository)
	fmt.Fprintf(&b, "- branch_sha256_12: `%s`\n", shortDocumentHash(req.Rehearsal.Branch))
	fmt.Fprintf(&b, "- head_commit: `%s`\n", req.Rehearsal.HeadCommit)
	fmt.Fprintf(&b, "- target_commit: `%s`\n", req.Rehearsal.TargetCommit)
	fmt.Fprintf(&b, "- comparison_range_sha256_12: `%s`\n", req.Rehearsal.ComparisonRangeSHA)
	fmt.Fprintf(&b, "- commits_available: `%d`\n", req.Rehearsal.CommitsAvailable)
	fmt.Fprintf(&b, "- changed_files: `%d`\n", req.Rehearsal.ChangedFiles)
	fmt.Fprintf(&b, "- preview_files_returned: `%d`\n", req.Rehearsal.PreviewFilesReturned)
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", req.Rehearsal.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: `%d`\n", req.Rehearsal.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: `%d`\n", req.Rehearsal.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", req.Rehearsal.UntrackedFiles)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", req.Rehearsal.BackupBranch)
	fmt.Fprintf(&b, "- backup_branch_local_ref: `%t`\n", req.Rehearsal.BackupBranchLocalRef)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "rollback-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", req.Rehearsal.RestoreMode)
	fmt.Fprintf(&b, "- rollback_mode: `%s`\n", "inspect-only")
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_reset_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_clean_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- checkout_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rehearsal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_ref_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Rehearsal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Rehearsal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Rehearsal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_checkpoint_rehearsal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	if req.Rehearsal.CheckpointErrorReason != "" {
		fmt.Fprintf(&b, "- checkpoint_error_reason: `%s`\n", req.Rehearsal.CheckpointErrorReason)
	}
	if req.Rehearsal.PreviewErrorReason != "" {
		fmt.Fprintf(&b, "- preview_error_reason: `%s`\n", req.Rehearsal.PreviewErrorReason)
	}
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing a checkpoint rollback from a mirrored channel thread, then queued a provider-facing rehearsal link back to that thread. This action does not call a model, print raw diffs, print file bodies, reset/clean/checkout files, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.Rehearsal.IssueNumber)
	b.WriteString("- run checkpoint status, rollback preview, checkpoint risk, and rollback risk before any reviewed recovery branch\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelCheckpointRehearsalNotificationBody(opts ChannelCheckpointRehearsalOptions, result CheckpointRehearsalIssueResult, rehearsal CheckpointRehearsalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel checkpoint rehearsal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Rehearsal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Target ref hash: %s\n", rehearsal.TargetRefSHA)
	fmt.Fprintf(&b, "Rollback mode: %s\n", "inspect-only")
	fmt.Fprintf(&b, "Restore mode: %s\n", rehearsal.RestoreMode)
	fmt.Fprintf(&b, "Changed files: %d\n", rehearsal.ChangedFiles)
	b.WriteString("\nContinue in the linked GitHub issue to rehearse rollback with normal model-backed conversation and dry-run checkpoint commands. This notification did not execute a model, print raw diffs, restore files, run git reset/clean/checkout, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelCheckpointRehearsalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelCheckpointRehearsalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelCheckpointRehearsalIssueTarget(ev Event, req *ChannelCheckpointRehearsalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel checkpoint rehearsal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelCheckpointRehearsalOptions(opts ChannelCheckpointRehearsalOptions) ChannelCheckpointRehearsalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RehearsalID = cleanCheckpointRehearsalID(opts.RehearsalID)
	opts.TargetRef = normalizeCheckpointPreviewTarget(opts.TargetRef)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelCheckpointRehearsalOptions(opts ChannelCheckpointRehearsalOptions) error {
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
		return fmt.Errorf("missing checkpoint rehearsal id")
	}
	if !skillNamePattern.MatchString(opts.RehearsalID) {
		return fmt.Errorf("invalid checkpoint rehearsal id %q", opts.RehearsalID)
	}
	if opts.TargetRef == "" {
		return fmt.Errorf("missing target ref")
	}
	if !checkpointPreviewTargetAllowed(opts.TargetRef) {
		return fmt.Errorf("unsafe checkpoint target ref")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildCheckpointRehearsalIssueRequestFromChannel(ev Event, cfg Config, opts ChannelCheckpointRehearsalOptions) CheckpointRehearsalIssueRequest {
	checkpoint := BuildCheckpointReport(cfg.Workdir)
	preview := BuildCheckpointPreviewReport(cfg.Workdir, opts.TargetRef)
	sourceText := activeRequestText(ev)
	return CheckpointRehearsalIssueRequest{
		Repo:                  ev.Repo,
		Command:               "/checkpoints",
		Subcommand:            "rehearse",
		RehearsalID:           opts.RehearsalID,
		TargetRef:             opts.TargetRef,
		TargetRefSHA:          shortDocumentHash(opts.TargetRef),
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
		SourceIssueNumber:     opts.SourceIssueNumber,
		SourceCommentID:       opts.SourceCommentID,
		SourceSHA:             shortDocumentHash(sourceText),
		SourceBytes:           len(sourceText),
		SourceLines:           lineCount(sourceText),
		SourceKind:            "channel_comment",
		CheckpointStatusCmd:   "gitclaw checkpoints status",
		CheckpointPreviewCmd:  fmt.Sprintf("gitclaw checkpoints preview %s", opts.TargetRef),
		CheckpointRiskCmd:     "gitclaw checkpoints risk",
		RollbackDiffCmd:       fmt.Sprintf("gitclaw rollback diff %s", opts.TargetRef),
		RollbackRiskCmd:       "gitclaw rollback risk",
		PreviewErrorReason:    preview.ErrorReason,
		CheckpointErrorReason: checkpoint.ErrorReason,
	}
}

func autoChannelCheckpointRehearsalID(ev Event, channel, threadID, sourceMessageID, targetRef string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, targetRef}, "|")
	return fmt.Sprintf("checkpoint-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelCheckpointRehearsalNotifyMessageID(ev Event, rehearsalID string) string {
	seed := strings.Join([]string{eventID(ev), rehearsalID}, "|")
	return fmt.Sprintf("gitclaw-channel-checkpoint-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
