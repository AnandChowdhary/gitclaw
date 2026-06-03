package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultChannelBackupContinuityMaxGapHours = 168
const maxChannelBackupContinuityMaxGapHours = 87600

type ChannelBackupContinuityOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ContinuityID      string
	MaxGapHours       int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupContinuityResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	ContinuityIDHash    string
	Continuity          BackupContinuity
	BackupFetchStatus   string
	BackupRootHash      string
	ContinuityErrorKind string
	ContinuityErrorHash string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupContinuityActionRequest struct {
	Options             ChannelBackupContinuityOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoContinuityID    bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ContinuityIDHash    string
	MaxGapSource        string
}

func IsChannelBackupContinuityActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupContinuityActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupContinuityActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBackupContinuitySubcommand(fields[1]) {
	case "backup-continuity", "backups-continuity", "backup-gaps", "backups-gaps", "backup-gap", "archive-continuity", "archive-gaps", "archive-health-gaps", "recovery-continuity":
		return true
	default:
		return false
	}
}

func BuildChannelBackupContinuityActionRequest(ev Event, cfg Config) (ChannelBackupContinuityActionRequest, error) {
	fields, _, ok := channelBackupContinuityActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupContinuityActionRequest{}, fmt.Errorf("missing channel backup continuity command")
	}
	req := ChannelBackupContinuityActionRequest{
		Options: ChannelBackupContinuityOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxGapHours:       defaultChannelBackupContinuityMaxGapHours,
		},
		Command:      strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:   cleanChannelBackupContinuitySubcommand(fields[1]),
		MaxGapSource: "default",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--continuity-id", "--backup-continuity-id", "--gap-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ContinuityID = cleanChannelBackupContinuityID(fields[i+1])
			i++
		case "--max-gap-hours", "--max-hours", "--gap-hours":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxGapHours, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil || maxGapHours < 1 || maxGapHours > maxChannelBackupContinuityMaxGapHours {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("%s must be an integer from 1 to %d", field, maxChannelBackupContinuityMaxGapHours)
			}
			req.Options.MaxGapHours = maxGapHours
			req.MaxGapSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupContinuityActionRequest{}, fmt.Errorf("unknown channel backup continuity argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBackupContinuityActionRequest{}, fmt.Errorf("unexpected channel backup continuity argument %q", field)
		}
	}
	if err := applyChannelBackupContinuityIssueTarget(ev, &req); err != nil {
		return ChannelBackupContinuityActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupContinuitySourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ContinuityID) == "" {
		req.Options.ContinuityID = autoChannelBackupContinuityID(ev, req.Options)
		req.AutoContinuityID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupContinuityNotifyMessageID(ev, req.Options.ContinuityID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupContinuityOptions(req.Options)
	if err := validateChannelBackupContinuityActionRequestOptions(req.Options); err != nil {
		return ChannelBackupContinuityActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ContinuityIDHash = shortDocumentHash(req.Options.ContinuityID)
	return req, nil
}

func RunChannelBackupContinuity(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelBackupContinuityActionRequest) (ChannelBackupContinuityResult, error) {
	opts := normalizeChannelBackupContinuityOptions(req.Options)
	var err error
	opts, err = applyChannelBackupContinuityRoute(cfg, opts)
	if err != nil {
		return ChannelBackupContinuityResult{}, err
	}
	if err := validateChannelBackupContinuityOptions(opts); err != nil {
		return ChannelBackupContinuityResult{}, err
	}
	continuity, backupRoot, fetchStatus, continuityErr := loadChannelBackupContinuity(ctx, cfg, opts)
	errorKind := ""
	errorHash := ""
	if continuityErr != nil {
		errorKind = channelBackupContinuityErrorKind(continuityErr)
		errorHash = shortDocumentHash(continuityErr.Error())
	}
	body := renderChannelBackupContinuityNotificationBody(opts, continuity, fetchStatus, errorKind)
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
		return ChannelBackupContinuityResult{}, fmt.Errorf("queue channel backup continuity notification: %w", err)
	}
	return ChannelBackupContinuityResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		ContinuityIDHash:    shortDocumentHash(opts.ContinuityID),
		Continuity:          continuity,
		BackupFetchStatus:   fetchStatus,
		BackupRootHash:      shortDocumentHash(backupRoot),
		ContinuityErrorKind: errorKind,
		ContinuityErrorHash: errorHash,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelBackupContinuityActionReport(ev Event, req ChannelBackupContinuityActionRequest, result ChannelBackupContinuityResult) string {
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
	continuityIDHash := result.ContinuityIDHash
	if continuityIDHash == "" {
		continuityIDHash = req.ContinuityIDHash
	}
	continuity := result.Continuity
	if continuity.BackupContinuityStatus == "" {
		continuity = unavailableBackupContinuity(req.Options.Repo, req.Options.MaxGapHours, "")
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Continuity Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_continuity_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_continuity_status: `%s`\n", continuity.BackupContinuityStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", continuity.BackupVerifyStatus)
	fmt.Fprintf(&b, "- continuity_gate: `%s`\n", noneIfEmpty(continuity.ContinuityGate))
	fmt.Fprintf(&b, "- backup_fetch_status: `%s`\n", noneIfEmpty(result.BackupFetchStatus))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- continuity_mode: `%s`\n", "gitclaw-backups-continuity-card")
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
	fmt.Fprintf(&b, "- continuity_id_sha256_12: `%s`\n", noneIfEmpty(continuityIDHash))
	fmt.Fprintf(&b, "- continuity_id_auto: `%t`\n", req.AutoContinuityID)
	fmt.Fprintf(&b, "- max_gap_hours: `%d`\n", req.Options.MaxGapHours)
	fmt.Fprintf(&b, "- max_gap_seconds: `%d`\n", continuity.MaxGapSeconds)
	fmt.Fprintf(&b, "- max_gap_source: `%s`\n", noneIfEmpty(req.MaxGapSource))
	fmt.Fprintf(&b, "- backup_root_sha256_12: `%s`\n", noneIfEmpty(result.BackupRootHash))
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.RepoDir)))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.IndexPath)))
	fmt.Fprintf(&b, "- readme_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.ReadmePath)))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", continuity.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.IndexGeneratedAt)))
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", continuity.VerificationFailures)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", continuity.IssueCount)
	fmt.Fprintf(&b, "- points_scanned: `%d`\n", continuity.PointsScanned)
	fmt.Fprintf(&b, "- timeline_order: `%s`\n", noneIfEmpty(continuity.TimelineOrder))
	fmt.Fprintf(&b, "- gaps_over_max: `%d`\n", continuity.GapsOverMax)
	fmt.Fprintf(&b, "- gaps_reported: `%d`\n", continuity.GapsReported)
	fmt.Fprintf(&b, "- first_issue_sha256_12: `%s`\n", channelBackupContinuityIssueHash(continuity.FirstIssueNumber))
	fmt.Fprintf(&b, "- first_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.FirstGeneratedAt)))
	fmt.Fprintf(&b, "- latest_issue_sha256_12: `%s`\n", channelBackupContinuityIssueHash(continuity.LatestIssueNumber))
	fmt.Fprintf(&b, "- latest_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.LatestGeneratedAt)))
	fmt.Fprintf(&b, "- total_span_seconds: `%d`\n", continuity.TotalSpanSeconds)
	fmt.Fprintf(&b, "- longest_gap_seconds: `%d`\n", continuity.LongestGapSeconds)
	fmt.Fprintf(&b, "- longest_gap_from_issue_sha256_12: `%s`\n", channelBackupContinuityIssueHash(continuity.LongestGapFromIssueNumber))
	fmt.Fprintf(&b, "- longest_gap_to_issue_sha256_12: `%s`\n", channelBackupContinuityIssueHash(continuity.LongestGapToIssueNumber))
	fmt.Fprintf(&b, "- longest_gap_from_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.LongestGapFromGeneratedAt)))
	fmt.Fprintf(&b, "- longest_gap_to_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(continuity.LongestGapToGeneratedAt)))
	fmt.Fprintf(&b, "- continuity_gaps_sha256_12: `%s`\n", noneIfEmpty(channelBackupContinuityGapsHash(continuity.Gaps)))
	fmt.Fprintf(&b, "- continuity_error_kind: `%s`\n", noneIfEmpty(result.ContinuityErrorKind))
	fmt.Fprintf(&b, "- continuity_error_sha256_12: `%s`\n", noneIfEmpty(result.ContinuityErrorHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(result.NotificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", result.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", result.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- backup_branch_fetch_performed: `%t`\n", strings.HasPrefix(result.BackupFetchStatus, "fetched"))
	fmt.Fprintf(&b, "- raw_backup_payloads_read: `%t`\n", result.BackupFetchStatus == "local" || result.BackupFetchStatus == "fetched")
	fmt.Fprintf(&b, "- restore_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_continuity_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_root_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_continuity_gaps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_titles_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_continuity_change: `%t`\n", true)
	fmt.Fprintf(&b, "- source_issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw inspected the local or read-only fetched gitclaw-backups continuity gate and queued a provider-facing backup-gap card. It reports chronological coverage, longest gap, threshold breaches, counts, timestamps, and hashes while keeping raw paths, titles, bodies, comments, transcripts, prompts, tool outputs, provider IDs, and continuity IDs out of the source receipt. This action does not write the backup branch, restore files, replay GitHub APIs, call a model, call provider APIs, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read backup-continuity cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-continuity cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-continuity notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelBackupContinuityNotificationBody(opts ChannelBackupContinuityOptions, continuity BackupContinuity, fetchStatus, errorKind string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup continuity\n\n")
	fmt.Fprintf(&b, "Backup continuity status: %s\n", continuity.BackupContinuityStatus)
	fmt.Fprintf(&b, "Backup verify status: %s\n", continuity.BackupVerifyStatus)
	fmt.Fprintf(&b, "Continuity gate: %s\n", noneIfEmpty(continuity.ContinuityGate))
	fmt.Fprintf(&b, "Backup branch: %s\n", defaultBackupBranch)
	fmt.Fprintf(&b, "Backup fetch status: %s\n", fetchStatus)
	if errorKind != "" {
		fmt.Fprintf(&b, "Continuity error kind: %s\n", errorKind)
	}
	fmt.Fprintf(&b, "Issue count: %d\n", continuity.IssueCount)
	fmt.Fprintf(&b, "Points scanned: %d\n", continuity.PointsScanned)
	fmt.Fprintf(&b, "Timeline order: %s\n", noneIfEmpty(continuity.TimelineOrder))
	fmt.Fprintf(&b, "Max gap hours: %d\n", opts.MaxGapHours)
	fmt.Fprintf(&b, "Max gap seconds: %d\n", continuity.MaxGapSeconds)
	fmt.Fprintf(&b, "Gaps over max: %d\n", continuity.GapsOverMax)
	fmt.Fprintf(&b, "Gaps reported: %d\n", continuity.GapsReported)
	if continuity.PointsScanned > 0 {
		fmt.Fprintf(&b, "First issue: #%d\n", continuity.FirstIssueNumber)
		fmt.Fprintf(&b, "First backup generated at: %s\n", continuity.FirstGeneratedAt)
		fmt.Fprintf(&b, "Latest issue: #%d\n", continuity.LatestIssueNumber)
		fmt.Fprintf(&b, "Latest backup generated at: %s\n", continuity.LatestGeneratedAt)
		fmt.Fprintf(&b, "Total span seconds: %d\n", continuity.TotalSpanSeconds)
	} else {
		b.WriteString("First issue: none\n")
		b.WriteString("Latest issue: none\n")
		b.WriteString("Total span seconds: 0\n")
	}
	fmt.Fprintf(&b, "Longest gap seconds: %d\n", continuity.LongestGapSeconds)
	if continuity.LongestGapSeconds > 0 {
		fmt.Fprintf(&b, "Longest gap from issue: #%d\n", continuity.LongestGapFromIssueNumber)
		fmt.Fprintf(&b, "Longest gap to issue: #%d\n", continuity.LongestGapToIssueNumber)
		fmt.Fprintf(&b, "Longest gap from generated at: %s\n", continuity.LongestGapFromGeneratedAt)
		fmt.Fprintf(&b, "Longest gap to generated at: %s\n", continuity.LongestGapToGeneratedAt)
	}
	fmt.Fprintf(&b, "Continuity id hash: %s\n", shortDocumentHash(opts.ContinuityID))
	b.WriteString("\nGaps over threshold:\n")
	if len(continuity.Gaps) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, gap := range continuity.Gaps {
			fmt.Fprintf(&b, "- from_issue=#%d to_issue=#%d from_generated_at=%s to_generated_at=%s gap_seconds=%d from_event_hash=%s to_event_hash=%s from_title_hash=%s to_title_hash=%s from_path_hash=%s to_path_hash=%s\n",
				gap.FromIssueNumber,
				gap.ToIssueNumber,
				gap.FromGeneratedAt,
				gap.ToGeneratedAt,
				gap.GapSeconds,
				shortDocumentHash(gap.FromEventName),
				shortDocumentHash(gap.ToEventName),
				gap.FromIssueTitleSHA,
				gap.ToIssueTitleSHA,
				shortDocumentHash(gap.FromPath),
				shortDocumentHash(gap.ToPath),
			)
		}
	}
	b.WriteString("\nRaw backup payloads, backup paths, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw continuity ids are not included. Model call: not performed by this action. Repository mutation: not performed by this action. Backup branch write: not performed by this action. Restore: not performed by this action. GitHub API replay: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func loadChannelBackupContinuity(ctx context.Context, cfg Config, opts ChannelBackupContinuityOptions) (BackupContinuity, string, string, error) {
	localRoot := channelBackupSearchLocalRoot(cfg)
	maxGap := time.Duration(opts.MaxGapHours) * time.Hour
	if channelBackupSearchIndexExists(localRoot, opts.Repo) {
		continuity, err := BuildBackupContinuity(localRoot, opts.Repo, maxGap)
		if err != nil {
			return unavailableBackupContinuity(opts.Repo, opts.MaxGapHours, localRoot), localRoot, "local_error", err
		}
		return continuity, localRoot, "local", nil
	}
	worktree, cleanup, err := fetchChannelBackupSearchWorktree(ctx, cfg)
	if err != nil {
		return unavailableBackupContinuity(opts.Repo, opts.MaxGapHours, localRoot), localRoot, "unavailable", err
	}
	defer cleanup()
	fetchedRoot := filepath.Join(worktree, defaultBackupRoot)
	continuity, err := BuildBackupContinuity(fetchedRoot, opts.Repo, maxGap)
	if err != nil {
		return unavailableBackupContinuity(opts.Repo, opts.MaxGapHours, fetchedRoot), fetchedRoot, "fetched_error", err
	}
	return continuity, fetchedRoot, "fetched", nil
}

func unavailableBackupContinuity(repo string, maxGapHours int, root string) BackupContinuity {
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	if maxGapHours <= 0 {
		maxGapHours = defaultChannelBackupContinuityMaxGapHours
	}
	return BackupContinuity{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		BackupContinuityStatus:    "unavailable",
		BackupVerifyStatus:        "unavailable",
		ContinuityGate:            "fail",
		TimelineOrder:             "chronological",
		MaxGapSeconds:             int64(maxGapHours) * int64(time.Hour/time.Second),
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
}

func channelBackupContinuityErrorKind(err error) string {
	if err == nil {
		return ""
	}
	base := channelBackupSearchErrorKind(err)
	if base != "backup_search_failed" {
		return base
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "max gap must be positive"):
		return "backup_continuity_max_gap_invalid"
	case strings.Contains(text, "parse backup timestamp"):
		return "backup_continuity_timestamp_invalid"
	case strings.Contains(text, "backup continuity reported"):
		return "backup_continuity_gate_failed"
	default:
		return "backup_continuity_failed"
	}
}

func channelBackupContinuityActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupContinuityActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupContinuityIssueTarget(ev Event, req *ChannelBackupContinuityActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup continuity requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupContinuityOptions(opts ChannelBackupContinuityOptions) ChannelBackupContinuityOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ContinuityID = cleanChannelBackupContinuityID(opts.ContinuityID)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxGapHours <= 0 {
		opts.MaxGapHours = defaultChannelBackupContinuityMaxGapHours
	}
	return opts
}

func applyChannelBackupContinuityRoute(cfg Config, opts ChannelBackupContinuityOptions) (ChannelBackupContinuityOptions, error) {
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
		Body:      "GitClaw channel backup continuity.",
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

func validateChannelBackupContinuityOptions(opts ChannelBackupContinuityOptions) error {
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
	if opts.ContinuityID == "" {
		return fmt.Errorf("missing backup continuity id")
	}
	if opts.MaxGapHours < 1 || opts.MaxGapHours > maxChannelBackupContinuityMaxGapHours {
		return fmt.Errorf("backup continuity max gap hours must be from 1 to %d", maxChannelBackupContinuityMaxGapHours)
	}
	return nil
}

func validateChannelBackupContinuityActionRequestOptions(opts ChannelBackupContinuityOptions) error {
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
	if opts.ContinuityID == "" {
		return fmt.Errorf("missing backup continuity id")
	}
	if opts.MaxGapHours < 1 || opts.MaxGapHours > maxChannelBackupContinuityMaxGapHours {
		return fmt.Errorf("backup continuity max gap hours must be from 1 to %d", maxChannelBackupContinuityMaxGapHours)
	}
	return nil
}

func cleanChannelBackupContinuitySubcommand(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
}

func cleanChannelBackupContinuityID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBackupContinuitySourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-continuity-source-%s", eventID(ev))
}

func autoChannelBackupContinuityID(ev Event, opts ChannelBackupContinuityOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, strconv.Itoa(opts.MaxGapHours)}, "|")
	return cleanChannelBackupContinuityID(fmt.Sprintf("backup-continuity-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelBackupContinuityNotifyMessageID(ev Event, continuityID string) string {
	seed := strings.Join([]string{eventID(ev), continuityID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-continuity-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelBackupContinuityIssueHash(issueNumber int) string {
	if issueNumber <= 0 {
		return "none"
	}
	return shortDocumentHash(strconv.Itoa(issueNumber))
}

func channelBackupContinuityGapsHash(gaps []BackupContinuityGap) string {
	if len(gaps) == 0 {
		return "none"
	}
	lines := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		lines = append(lines, strings.Join([]string{
			strconv.Itoa(gap.FromIssueNumber),
			strconv.Itoa(gap.ToIssueNumber),
			gap.FromPath,
			gap.ToPath,
			gap.FromGeneratedAt,
			gap.ToGeneratedAt,
			gap.FromEventName,
			gap.ToEventName,
			strconv.FormatInt(gap.GapSeconds, 10),
			gap.FromIssueTitleSHA,
			gap.ToIssueTitleSHA,
		}, "|"))
	}
	return shortDocumentHash(strings.Join(lines, "\n"))
}
