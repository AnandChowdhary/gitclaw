package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelCheckpointStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelCheckpointStatusResult struct {
	Notification         ChannelSendResult
	RouteName            string
	RouteHash            string
	Channel              string
	ThreadHash           string
	MessageHash          string
	NotifyHash           string
	StatusIDHash         string
	BodyHash             string
	BodyBytes            int
	BodyLines            int
	Checkpoint           CheckpointReport
	Risk                 CheckpointRiskReport
	RecentCommitIndexSHA string
	RiskFindingIndexSHA  string
}

type ChannelCheckpointStatusActionRequest struct {
	Options              ChannelCheckpointStatusOptions
	Command              string
	Subcommand           string
	AutoSourceMessageID  bool
	AutoNotifyMessageID  bool
	AutoStatusID         bool
	TargetFromIssue      bool
	RequestedRouteHash   string
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	StatusIDHash         string
	NotificationBodySHA  string
	NotificationBytes    int
	NotificationLines    int
	Checkpoint           CheckpointReport
	Risk                 CheckpointRiskReport
	RecentCommitIndexSHA string
	RiskFindingIndexSHA  string
}

func IsChannelCheckpointStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelCheckpointStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelCheckpointStatusActionFields(fields)
}

func isChannelCheckpointStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelCheckpointStatusSubcommand(fields[1]) {
	case "checkpoint-status", "checkpoints-status", "checkpoint-health", "rollback-status", "rollback-health", "rollback-readiness", "checkpoint-readiness", "checkpoint-state", "rollback-state":
		return true
	default:
		return false
	}
}

func BuildChannelCheckpointStatusActionRequest(ev Event, cfg Config) (ChannelCheckpointStatusActionRequest, error) {
	fields, _, ok := channelCheckpointStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("missing channel checkpoint status command")
	}
	req := ChannelCheckpointStatusActionRequest{
		Options:    ChannelCheckpointStatusOptions{Repo: ev.Repo},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelCheckpointStatusSubcommand(fields[1]),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--checkpoint-status-id", "--rollback-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelCheckpointStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("unknown channel checkpoint status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelCheckpointStatusActionRequest{}, fmt.Errorf("unexpected channel checkpoint status argument %q", field)
		}
	}
	if err := applyChannelCheckpointStatusIssueTarget(ev, &req); err != nil {
		return ChannelCheckpointStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelCheckpointStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelCheckpointStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelCheckpointStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelCheckpointStatusOptions(req.Options)
	if err := validateChannelCheckpointStatusActionRequestOptions(req.Options); err != nil {
		return ChannelCheckpointStatusActionRequest{}, err
	}
	checkpoint := BuildCheckpointReport(cfg.Workdir)
	risk := BuildCheckpointRiskReport(checkpoint)
	notificationBody := RenderChannelCheckpointStatusNotificationBody(req.Options, checkpoint, risk)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	req.Checkpoint = checkpoint
	req.Risk = risk
	req.RecentCommitIndexSHA = hashStringOrNone(channelCheckpointStatusRecentCommitIndex(checkpoint))
	req.RiskFindingIndexSHA = hashStringOrNone(channelCheckpointStatusRiskFindingIndex(risk.Findings))
	return req, nil
}

func RunChannelCheckpointStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelCheckpointStatusActionRequest) (ChannelCheckpointStatusResult, error) {
	opts := normalizeChannelCheckpointStatusOptions(req.Options)
	var err error
	opts, err = applyChannelCheckpointStatusRoute(cfg, opts)
	if err != nil {
		return ChannelCheckpointStatusResult{}, err
	}
	if err := validateChannelCheckpointStatusOptions(opts); err != nil {
		return ChannelCheckpointStatusResult{}, err
	}
	checkpoint := req.Checkpoint
	if checkpoint.Status == "" {
		checkpoint = BuildCheckpointReport(cfg.Workdir)
	}
	risk := req.Risk
	if risk.VerificationScope == "" {
		risk = BuildCheckpointRiskReport(checkpoint)
	}
	body := RenderChannelCheckpointStatusNotificationBody(opts, checkpoint, risk)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelCheckpointStatusResult{}, fmt.Errorf("queue channel checkpoint status notification: %w", err)
	}
	return ChannelCheckpointStatusResult{
		Notification:         notification,
		RouteName:            opts.Route,
		RouteHash:            channelRouteHash(opts.Route),
		Channel:              opts.Channel,
		ThreadHash:           shortDocumentHash(opts.ThreadID),
		MessageHash:          shortDocumentHash(opts.SourceMessageID),
		NotifyHash:           shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash:         shortDocumentHash(opts.StatusID),
		BodyHash:             shortDocumentHash(body),
		BodyBytes:            len(body),
		BodyLines:            lineCount(body),
		Checkpoint:           checkpoint,
		Risk:                 risk,
		RecentCommitIndexSHA: hashStringOrNone(channelCheckpointStatusRecentCommitIndex(checkpoint)),
		RiskFindingIndexSHA:  hashStringOrNone(channelCheckpointStatusRiskFindingIndex(risk.Findings)),
	}, nil
}

func RenderChannelCheckpointStatusActionReport(ev Event, req ChannelCheckpointStatusActionRequest, result ChannelCheckpointStatusResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
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
	statusIDHash := result.StatusIDHash
	if statusIDHash == "" {
		statusIDHash = req.StatusIDHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	bodyBytes := result.BodyBytes
	if bodyBytes == 0 {
		bodyBytes = req.NotificationBytes
	}
	bodyLines := result.BodyLines
	if bodyLines == 0 {
		bodyLines = req.NotificationLines
	}
	checkpoint := result.Checkpoint
	if checkpoint.Status == "" {
		checkpoint = req.Checkpoint
	}
	risk := result.Risk
	if risk.VerificationScope == "" {
		risk = req.Risk
	}
	recentCommitIndexSHA := firstNonEmpty(result.RecentCommitIndexSHA, req.RecentCommitIndexSHA)
	riskFindingIndexSHA := firstNonEmpty(result.RiskFindingIndexSHA, req.RiskFindingIndexSHA)

	var b strings.Builder
	b.WriteString("## GitClaw Channel Checkpoint Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_checkpoint_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- checkpoint_snapshot_mode: `%s`\n", "provider-facing-rollback-readiness")
	fmt.Fprintf(&b, "- checkpoint_status: `%s`\n", checkpoint.Status)
	fmt.Fprintf(&b, "- checkpoint_risk_status: `%s`\n", risk.Status)
	fmt.Fprintf(&b, "- checkpoint_strategy: `%s`\n", firstNonEmpty(risk.CheckpointStrategy, "git-history-plus-backup-branch"))
	fmt.Fprintf(&b, "- rollback_mode: `%s`\n", firstNonEmpty(risk.RollbackMode, "inspect-only"))
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- source_message_id_auto: `%t`\n", req.AutoSourceMessageID)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- checkpoint_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- checkpoint_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- git_available: `%t`\n", checkpoint.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", checkpoint.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", checkpoint.Root)
	fmt.Fprintf(&b, "- branch: `%s`\n", checkpoint.Branch)
	fmt.Fprintf(&b, "- head_commit: `%s`\n", checkpoint.HeadShortSHA)
	fmt.Fprintf(&b, "- commits_available: `%d`\n", checkpoint.CommitsAvailable)
	fmt.Fprintf(&b, "- recent_commits_returned: `%d`\n", checkpoint.RecentCommitsReturned)
	fmt.Fprintf(&b, "- recent_commit_limit: `%d`\n", maxCheckpointRecentCommits)
	fmt.Fprintf(&b, "- recent_commit_index_sha256_12: `%s`\n", noneIfEmpty(recentCommitIndexSHA))
	fmt.Fprintf(&b, "- worktree_clean: `%t`\n", checkpoint.WorktreeClean)
	fmt.Fprintf(&b, "- staged_changes: `%d`\n", checkpoint.StagedChanges)
	fmt.Fprintf(&b, "- unstaged_changes: `%d`\n", checkpoint.UnstagedChanges)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", checkpoint.UntrackedFiles)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", checkpoint.BackupBranch)
	fmt.Fprintf(&b, "- backup_branch_local_ref: `%t`\n", checkpoint.BackupBranchLocalRef)
	fmt.Fprintf(&b, "- surfaces_with_risk_findings: `%d`\n", risk.SurfacesWithRiskFindings)
	fmt.Fprintf(&b, "- checkpoint_risk_findings: `%d`\n", len(risk.Findings))
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", risk.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", risk.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", risk.InfoRiskFindings)
	fmt.Fprintf(&b, "- risk_finding_index_sha256_12: `%s`\n", noneIfEmpty(riskFindingIndexSHA))
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", checkpoint.RawDiffsIncluded)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", checkpoint.RawFileBodiesIncluded)
	fmt.Fprintf(&b, "- restore_operations_enabled: `%t`\n", checkpoint.RestoreOperationsEnabled)
	fmt.Fprintf(&b, "- git_reset_allowed: `%t`\n", risk.GitResetAllowed)
	fmt.Fprintf(&b, "- git_clean_allowed: `%t`\n", risk.GitCleanAllowed)
	fmt.Fprintf(&b, "- checkout_mutation_allowed: `%t`\n", risk.CheckoutMutationAllowed)
	fmt.Fprintf(&b, "- pre_restore_snapshot_required: `%t`\n", risk.PreRestoreSnapshotRequired)
	fmt.Fprintf(&b, "- rollback_diff_preview_required: `%t`\n", risk.RollbackDiffPreviewRequired)
	fmt.Fprintf(&b, "- backup_manifest_required_for_restore: `%t`\n", risk.BackupManifestRequiredForRestore)
	fmt.Fprintf(&b, "- shadow_store_path_included: `%t`\n", risk.ShadowStorePathIncluded)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", bodyBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", bodyLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_reset_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- git_clean_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- checkout_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_diff_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_checkpoint_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_commit_subjects_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_diffs_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_bodies_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_checkpoint_status_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	if checkpoint.ErrorReason != "" {
		fmt.Fprintf(&b, "- checkpoint_error_reason: `%s`\n", checkpoint.ErrorReason)
	}
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing checkpoint and rollback readiness card on the canonical channel issue. The action inspects only git metadata and checkpoint risk gates; it does not generate raw diffs, print file bodies or commit subjects, restore, reset, clean, checkout, mutate repository files, call a model, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, channel bodies, issue bodies, comments, prompts, tool outputs, raw diffs, file bodies, and credential values out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read checkpoint-status cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent checkpoint-status cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate checkpoint-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/channels rehearse-checkpoint --target HEAD~1` when the channel message should become a rollback rehearsal lane\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelCheckpointStatusNotificationBody(opts ChannelCheckpointStatusOptions, checkpoint CheckpointReport, risk CheckpointRiskReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel checkpoint status\n\n")
	fmt.Fprintf(&b, "Checkpoint status: %s\n", checkpoint.Status)
	fmt.Fprintf(&b, "Checkpoint risk status: %s\n", risk.Status)
	fmt.Fprintf(&b, "Checkpoint strategy: %s\n", firstNonEmpty(risk.CheckpointStrategy, "git-history-plus-backup-branch"))
	fmt.Fprintf(&b, "Rollback mode: %s\n", firstNonEmpty(risk.RollbackMode, "inspect-only"))
	fmt.Fprintf(&b, "Git available: %t\n", checkpoint.GitAvailable)
	fmt.Fprintf(&b, "Git repository: %t\n", checkpoint.GitRepository)
	fmt.Fprintf(&b, "Worktree root: %s\n", checkpoint.Root)
	fmt.Fprintf(&b, "Branch: %s\n", checkpoint.Branch)
	fmt.Fprintf(&b, "Head commit: %s\n", checkpoint.HeadShortSHA)
	fmt.Fprintf(&b, "Commits available: %d\n", checkpoint.CommitsAvailable)
	fmt.Fprintf(&b, "Recent commits returned: %d\n", checkpoint.RecentCommitsReturned)
	fmt.Fprintf(&b, "Worktree clean: %t\n", checkpoint.WorktreeClean)
	fmt.Fprintf(&b, "Staged changes: %d\n", checkpoint.StagedChanges)
	fmt.Fprintf(&b, "Unstaged changes: %d\n", checkpoint.UnstagedChanges)
	fmt.Fprintf(&b, "Untracked files: %d\n", checkpoint.UntrackedFiles)
	fmt.Fprintf(&b, "Backup branch: %s\n", checkpoint.BackupBranch)
	fmt.Fprintf(&b, "Backup branch local ref: %t\n", checkpoint.BackupBranchLocalRef)
	fmt.Fprintf(&b, "Surfaces with risk findings: %d\n", risk.SurfacesWithRiskFindings)
	fmt.Fprintf(&b, "Risk findings: %d\n", len(risk.Findings))
	fmt.Fprintf(&b, "High risk findings: %d\n", risk.HighRiskFindings)
	fmt.Fprintf(&b, "Warning risk findings: %d\n", risk.WarningRiskFindings)
	fmt.Fprintf(&b, "Info risk findings: %d\n", risk.InfoRiskFindings)
	fmt.Fprintf(&b, "Status id hash: %s\n", shortDocumentHash(opts.StatusID))
	if checkpoint.ErrorReason != "" {
		fmt.Fprintf(&b, "Checkpoint error reason: %s\n", checkpoint.ErrorReason)
	}
	b.WriteString("\nRecent commits:\n")
	if len(checkpoint.RecentCommits) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, commit := range checkpoint.RecentCommits {
			fmt.Fprintf(&b, "- commit=%s date=%s subject_sha256_12=%s raw_subject_included=false raw_diff_included=false\n", commit.ShortSHA, commit.Date, commit.SubjectSHA)
		}
	}
	b.WriteString("\nRisk findings:\n")
	if len(risk.Findings) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, finding := range risk.Findings {
			fmt.Fprintf(&b, "- severity=%s code=%s category=%s kind=%s field=%s line_sha256_12=%s\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Field, finding.LineSHA)
		}
	}
	b.WriteString("\nSafe follow-up commands:\n")
	b.WriteString("- gitclaw checkpoints status\n")
	b.WriteString("- gitclaw checkpoints preview HEAD~1\n")
	b.WriteString("- gitclaw checkpoints risk\n")
	b.WriteString("- gitclaw rollback diff HEAD~1\n")
	b.WriteString("- @gitclaw /channels rehearse-checkpoint --target HEAD~1 --message-id <id>\n")
	b.WriteString("\nRaw diffs: not generated by this action.\n")
	b.WriteString("Raw file bodies: not included.\n")
	b.WriteString("Raw commit subjects: not included.\n")
	b.WriteString("Restore: not performed by this action.\n")
	b.WriteString("Git reset: not performed by this action.\n")
	b.WriteString("Git clean: not performed by this action.\n")
	b.WriteString("Checkout mutation: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelCheckpointStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelCheckpointStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelCheckpointStatusIssueTarget(ev Event, req *ChannelCheckpointStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel checkpoint status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelCheckpointStatusOptions(opts ChannelCheckpointStatusOptions) ChannelCheckpointStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelCheckpointStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelCheckpointStatusRoute(cfg Config, opts ChannelCheckpointStatusOptions) (ChannelCheckpointStatusOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      "GitClaw channel checkpoint status.",
	})
	if err != nil {
		return opts, err
	}
	opts.Route = routeOpts.Route
	opts.Channel = routeOpts.Channel
	opts.ThreadID = routeOpts.ThreadID
	opts.Author = routeOpts.Author
	return opts, nil
}

func validateChannelCheckpointStatusOptions(opts ChannelCheckpointStatusOptions) error {
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
	if opts.StatusID == "" {
		return fmt.Errorf("missing checkpoint status id")
	}
	return nil
}

func validateChannelCheckpointStatusActionRequestOptions(opts ChannelCheckpointStatusOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.StatusID == "" {
		return fmt.Errorf("missing checkpoint status id")
	}
	return nil
}

func cleanChannelCheckpointStatusSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelCheckpointStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelCheckpointStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-checkpoint-status-source-%s", eventID(ev))
}

func autoChannelCheckpointStatusID(ev Event, opts ChannelCheckpointStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("checkpoint-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelCheckpointStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-checkpoint-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelCheckpointStatusRecentCommitIndex(report CheckpointReport) string {
	var lines []string
	for _, commit := range report.RecentCommits {
		lines = append(lines, fmt.Sprintf("%s|%s|%s", commit.ShortSHA, commit.Date, commit.SubjectSHA))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}

func channelCheckpointStatusRiskFindingIndex(findings []CheckpointRiskFinding) string {
	var lines []string
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%s|%s|%s", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Field, finding.LineSHA))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}
