package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type ChannelBackupInfoOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	InfoID            string
	IssueNumber       int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupInfoResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	InfoIDHash          string
	IssueNumberHash     string
	Info                BackupInfo
	BackupFetchStatus   string
	BackupRootHash      string
	InfoErrorKind       string
	InfoErrorHash       string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupInfoActionRequest struct {
	Options             ChannelBackupInfoOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoInfoID          bool
	TargetFromIssue     bool
	IssueSource         string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	InfoIDHash          string
	IssueNumberHash     string
}

func IsChannelBackupInfoActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupInfoActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupInfoActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBackupInfoSubcommand(fields[1]) {
	case "backup-info", "backups-info", "backup-describe", "describe-backup", "backup-card", "recovery-info", "archive-info":
		return true
	default:
		return false
	}
}

func BuildChannelBackupInfoActionRequest(ev Event, cfg Config) (ChannelBackupInfoActionRequest, error) {
	fields, trailing, ok := channelBackupInfoActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupInfoActionRequest{}, fmt.Errorf("missing channel backup info command")
	}
	req := ChannelBackupInfoActionRequest{
		Options: ChannelBackupInfoOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			IssueNumber:       ev.Issue.Number,
		},
		Command:     strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:  cleanChannelBackupInfoSubcommand(fields[1]),
		IssueSource: "current-channel-issue",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--info-id", "--backup-info-id", "--archive-info-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.InfoID = cleanChannelBackupInfoID(fields[i+1])
			i++
		case "--issue", "--issue-number", "--target-issue", "--number":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			issueNumber, ok := parseBackupIssueNumber(fields[i+1])
			if !ok {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("invalid backup issue number %q", fields[i+1])
			}
			req.Options.IssueNumber = issueNumber
			req.IssueSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("unknown channel backup info argument %q", field)
			}
			issueNumber, ok := parseBackupIssueNumber(field)
			if !ok {
				return ChannelBackupInfoActionRequest{}, fmt.Errorf("invalid channel backup info issue argument %q", field)
			}
			req.Options.IssueNumber = issueNumber
			req.IssueSource = "positional"
		}
	}
	if req.IssueSource == "current-channel-issue" {
		if issueNumber, ok := parseChannelBackupInfoTrailingIssue(trailing); ok {
			req.Options.IssueNumber = issueNumber
			req.IssueSource = "trailing-issue"
		}
	}
	if err := applyChannelBackupInfoIssueTarget(ev, &req); err != nil {
		return ChannelBackupInfoActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupInfoSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.InfoID) == "" {
		req.Options.InfoID = autoChannelBackupInfoID(ev, req.Options)
		req.AutoInfoID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupInfoNotifyMessageID(ev, req.Options.InfoID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupInfoOptions(req.Options)
	if err := validateChannelBackupInfoActionRequestOptions(req.Options); err != nil {
		return ChannelBackupInfoActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.InfoIDHash = shortDocumentHash(req.Options.InfoID)
	req.IssueNumberHash = shortDocumentHash(strconv.Itoa(req.Options.IssueNumber))
	return req, nil
}

func RunChannelBackupInfo(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelBackupInfoActionRequest) (ChannelBackupInfoResult, error) {
	opts := normalizeChannelBackupInfoOptions(req.Options)
	var err error
	opts, err = applyChannelBackupInfoRoute(cfg, opts)
	if err != nil {
		return ChannelBackupInfoResult{}, err
	}
	if err := validateChannelBackupInfoOptions(opts); err != nil {
		return ChannelBackupInfoResult{}, err
	}
	info, backupRoot, fetchStatus, infoErr := loadChannelBackupInfoReport(ctx, cfg, opts)
	errorKind := ""
	errorHash := ""
	if infoErr != nil {
		errorKind = channelBackupInfoErrorKind(infoErr)
		errorHash = shortDocumentHash(infoErr.Error())
	}
	body := renderChannelBackupInfoNotificationBody(opts, info, fetchStatus, errorKind)
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
		return ChannelBackupInfoResult{}, fmt.Errorf("queue channel backup info notification: %w", err)
	}
	return ChannelBackupInfoResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		InfoIDHash:          shortDocumentHash(opts.InfoID),
		IssueNumberHash:     shortDocumentHash(strconv.Itoa(opts.IssueNumber)),
		Info:                info,
		BackupFetchStatus:   fetchStatus,
		BackupRootHash:      shortDocumentHash(backupRoot),
		InfoErrorKind:       errorKind,
		InfoErrorHash:       errorHash,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelBackupInfoActionReport(ev Event, req ChannelBackupInfoActionRequest, result ChannelBackupInfoResult) string {
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
	infoIDHash := result.InfoIDHash
	if infoIDHash == "" {
		infoIDHash = req.InfoIDHash
	}
	issueHash := result.IssueNumberHash
	if issueHash == "" {
		issueHash = req.IssueNumberHash
	}
	info := result.Info
	if info.BackupInfoStatus == "" {
		info = unavailableBackupInfo(optsRepoOrEvent(req.Options.Repo, ev.Repo), req.Options.IssueNumber, "")
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Info Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_info_status: `%s`\n", info.BackupInfoStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", info.BackupVerifyStatus)
	fmt.Fprintf(&b, "- backup_fetch_status: `%s`\n", noneIfEmpty(result.BackupFetchStatus))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- info_mode: `%s`\n", "gitclaw-backups-single-issue-metadata")
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
	fmt.Fprintf(&b, "- backup_info_id_sha256_12: `%s`\n", noneIfEmpty(infoIDHash))
	fmt.Fprintf(&b, "- backup_info_id_auto: `%t`\n", req.AutoInfoID)
	fmt.Fprintf(&b, "- backup_issue_number_sha256_12: `%s`\n", noneIfEmpty(issueHash))
	fmt.Fprintf(&b, "- backup_issue_source: `%s`\n", noneIfEmpty(req.IssueSource))
	fmt.Fprintf(&b, "- backup_root_sha256_12: `%s`\n", noneIfEmpty(result.BackupRootHash))
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(info.RepoDir)))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(info.IndexPath)))
	fmt.Fprintf(&b, "- readme_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(info.ReadmePath)))
	fmt.Fprintf(&b, "- issue_backup_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(info.IssuePath)))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", info.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(info.IndexGeneratedAt)))
	fmt.Fprintf(&b, "- backup_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(info.BackupGeneratedAt)))
	fmt.Fprintf(&b, "- backup_event_name: `%s`\n", noneIfEmpty(info.EventName))
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", info.VerificationFailures)
	fmt.Fprintf(&b, "- payload_bytes: `%d`\n", info.PayloadBytes)
	fmt.Fprintf(&b, "- payload_sha256_12: `%s`\n", noneIfEmpty(info.PayloadSHA))
	fmt.Fprintf(&b, "- labels: `%d`\n", len(info.Labels))
	fmt.Fprintf(&b, "- label_names_sha256_12: `%s`\n", hashStringList(info.Labels))
	fmt.Fprintf(&b, "- comments: `%d`\n", info.Comments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", info.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", info.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", info.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", info.AssistantTurns)
	fmt.Fprintf(&b, "- error_comments: `%d`\n", info.ErrorComments)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", noneIfEmpty(info.IssueTitleSHA))
	fmt.Fprintf(&b, "- issue_body_sha256_12: `%s`\n", noneIfEmpty(info.IssueBodySHA))
	fmt.Fprintf(&b, "- comment_body_hashes_sha256_12: `%s`\n", hashStringList(info.CommentBodySHAs))
	fmt.Fprintf(&b, "- transcript_body_hashes_sha256_12: `%s`\n", hashStringList(info.TranscriptBodySHAs))
	fmt.Fprintf(&b, "- info_error_kind: `%s`\n", noneIfEmpty(result.InfoErrorKind))
	fmt.Fprintf(&b, "- info_error_sha256_12: `%s`\n", noneIfEmpty(result.InfoErrorHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(result.NotificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", result.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", result.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_backup_issue_number_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_info_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_root_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_label_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_titles_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_info_change: `%t`\n", true)
	fmt.Fprintf(&b, "- source_issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw inspected one fetched gitclaw-backups issue payload and queued provider-facing metadata. This action may fetch the backup branch read-only when the local backup root is absent, but it does not write the backup branch, restore files, replay GitHub APIs, call a model, call provider APIs, or print raw backup payloads.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read backup-info cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-info cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-info notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelBackupInfoNotificationBody(opts ChannelBackupInfoOptions, info BackupInfo, fetchStatus, errorKind string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup info\n\n")
	fmt.Fprintf(&b, "Backup info status: %s\n", info.BackupInfoStatus)
	fmt.Fprintf(&b, "Backup verify status: %s\n", info.BackupVerifyStatus)
	fmt.Fprintf(&b, "Backup branch: %s\n", defaultBackupBranch)
	fmt.Fprintf(&b, "Backup fetch status: %s\n", fetchStatus)
	if errorKind != "" {
		fmt.Fprintf(&b, "Info error kind: %s\n", errorKind)
	}
	fmt.Fprintf(&b, "Issue: #%d\n", info.IssueNumber)
	fmt.Fprintf(&b, "Issue path: %s\n", noneIfEmpty(info.IssuePath))
	fmt.Fprintf(&b, "Backup generated at: %s\n", noneIfEmpty(info.BackupGeneratedAt))
	fmt.Fprintf(&b, "Backup event name: %s\n", noneIfEmpty(info.EventName))
	fmt.Fprintf(&b, "Backup schema version: %d\n", info.SchemaVersion)
	fmt.Fprintf(&b, "Verification failures: %d\n", info.VerificationFailures)
	fmt.Fprintf(&b, "Payload bytes: %d\n", info.PayloadBytes)
	fmt.Fprintf(&b, "Payload hash: %s\n", noneIfEmpty(info.PayloadSHA))
	fmt.Fprintf(&b, "Labels: %d\n", len(info.Labels))
	fmt.Fprintf(&b, "Label names hash: %s\n", hashStringList(info.Labels))
	fmt.Fprintf(&b, "Comments: %d\n", info.Comments)
	fmt.Fprintf(&b, "Transcript messages: %d\n", info.TranscriptMessages)
	fmt.Fprintf(&b, "User messages: %d\n", info.UserMessages)
	fmt.Fprintf(&b, "Assistant messages: %d\n", info.AssistantMessages)
	fmt.Fprintf(&b, "Assistant turn comments: %d\n", info.AssistantTurns)
	fmt.Fprintf(&b, "Error comments: %d\n", info.ErrorComments)
	fmt.Fprintf(&b, "Issue title hash: %s\n", noneIfEmpty(info.IssueTitleSHA))
	fmt.Fprintf(&b, "Issue body hash: %s\n", noneIfEmpty(info.IssueBodySHA))
	fmt.Fprintf(&b, "Comment body hashes: %d\n", len(info.CommentBodySHAs))
	fmt.Fprintf(&b, "Transcript body hashes: %d\n", len(info.TranscriptBodySHAs))
	fmt.Fprintf(&b, "Backup info id hash: %s\n", shortDocumentHash(opts.InfoID))
	b.WriteString("\nRaw backup payloads, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw backup info ids are not included. Model call: not performed by this action. Repository mutation: not performed by this action. Backup branch write: not performed by this action. Restore: not performed by this action. GitHub API replay: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func loadChannelBackupInfoReport(ctx context.Context, cfg Config, opts ChannelBackupInfoOptions) (BackupInfo, string, string, error) {
	localRoot := channelBackupSearchLocalRoot(cfg)
	if channelBackupSearchIndexExists(localRoot, opts.Repo) {
		info, err := BuildBackupInfo(localRoot, opts.Repo, opts.IssueNumber)
		if err != nil {
			return unavailableBackupInfo(opts.Repo, opts.IssueNumber, localRoot), localRoot, "local_error", err
		}
		return info, localRoot, "local", nil
	}
	worktree, cleanup, err := fetchChannelBackupSearchWorktree(ctx, cfg)
	if err != nil {
		return unavailableBackupInfo(opts.Repo, opts.IssueNumber, localRoot), localRoot, "unavailable", err
	}
	defer cleanup()
	fetchedRoot := filepath.Join(worktree, defaultBackupRoot)
	info, err := BuildBackupInfo(fetchedRoot, opts.Repo, opts.IssueNumber)
	if err != nil {
		return unavailableBackupInfo(opts.Repo, opts.IssueNumber, fetchedRoot), fetchedRoot, "fetched_error", err
	}
	return info, fetchedRoot, "fetched", nil
}

func unavailableBackupInfo(repo string, issueNumber int, root string) BackupInfo {
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	return BackupInfo{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		IssueNumber:               issueNumber,
		BackupInfoStatus:          "unavailable",
		BackupVerifyStatus:        "unavailable",
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
}

func channelBackupInfoErrorKind(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "fetch backup branch"):
		return "backup_branch_fetch_failed"
	case strings.Contains(text, "create backup worktree"):
		return "backup_worktree_failed"
	case strings.Contains(text, "read backup index"):
		return "backup_index_unavailable"
	case strings.Contains(text, "parse backup index"):
		return "backup_index_invalid"
	case strings.Contains(text, "backup index repo"):
		return "backup_index_repo_mismatch"
	case strings.Contains(text, "not found in backup index"):
		return "backup_issue_not_found"
	default:
		return "backup_info_failed"
	}
}

func channelBackupInfoActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupInfoActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupInfoIssueTarget(ev Event, req *ChannelBackupInfoActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup info requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupInfoOptions(opts ChannelBackupInfoOptions) ChannelBackupInfoOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.InfoID = cleanChannelBackupInfoID(opts.InfoID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBackupInfoRoute(cfg Config, opts ChannelBackupInfoOptions) (ChannelBackupInfoOptions, error) {
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
		Body:      "GitClaw channel backup info.",
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

func validateChannelBackupInfoOptions(opts ChannelBackupInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing backup info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid backup info id %q", opts.InfoID)
	}
	if opts.IssueNumber <= 0 {
		return fmt.Errorf("missing positive backup issue number")
	}
	return nil
}

func validateChannelBackupInfoActionRequestOptions(opts ChannelBackupInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing backup info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid backup info id %q", opts.InfoID)
	}
	if opts.IssueNumber <= 0 {
		return fmt.Errorf("missing positive backup issue number")
	}
	return nil
}

func cleanChannelBackupInfoSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelBackupInfoID(value string) string {
	return cleanChannelHuddleID(value)
}

func parseChannelBackupInfoTrailingIssue(trailing string) (int, bool) {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "issue:") || strings.HasPrefix(lower, "backup:") || strings.HasPrefix(lower, "target:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return parseBackupIssueNumber(trimmed[idx+1:])
			}
		}
	}
	return 0, false
}

func autoChannelBackupInfoSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-info-source-%s", eventID(ev))
}

func autoChannelBackupInfoID(ev Event, opts ChannelBackupInfoOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, strconv.Itoa(opts.IssueNumber)}, "|")
	return cleanChannelBackupInfoID(fmt.Sprintf("backup-info-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelBackupInfoNotifyMessageID(ev Event, infoID string) string {
	seed := strings.Join([]string{eventID(ev), infoID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-info-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func optsRepoOrEvent(optsRepo, eventRepo string) string {
	if strings.TrimSpace(optsRepo) != "" {
		return optsRepo
	}
	return eventRepo
}
