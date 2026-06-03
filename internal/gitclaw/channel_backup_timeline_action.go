package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultChannelBackupTimelineLimit = 5
const maxChannelBackupTimelineLimit = 25

type ChannelBackupTimelineOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	TimelineID        string
	Limit             int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupTimelineResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	TimelineIDHash      string
	Timeline            BackupTimeline
	BackupFetchStatus   string
	BackupRootHash      string
	TimelineErrorKind   string
	TimelineErrorHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupTimelineActionRequest struct {
	Options             ChannelBackupTimelineOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoTimelineID      bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	TimelineIDHash      string
	LimitSource         string
}

func IsChannelBackupTimelineActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupTimelineActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupTimelineActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBackupTimelineSubcommand(fields[1]) {
	case "backup-timeline", "backups-timeline", "backup-history", "backups-history", "timeline-backups", "backup-chronology", "recovery-timeline", "archive-timeline", "archive-history":
		return true
	default:
		return false
	}
}

func BuildChannelBackupTimelineActionRequest(ev Event, cfg Config) (ChannelBackupTimelineActionRequest, error) {
	fields, _, ok := channelBackupTimelineActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupTimelineActionRequest{}, fmt.Errorf("missing channel backup timeline command")
	}
	req := ChannelBackupTimelineActionRequest{
		Options: ChannelBackupTimelineOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Limit:             defaultChannelBackupTimelineLimit,
		},
		Command:     strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:  cleanChannelBackupTimelineSubcommand(fields[1]),
		LimitSource: "default",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--timeline-id", "--backup-timeline-id", "--history-id", "--chronology-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TimelineID = cleanChannelBackupTimelineID(fields[i+1])
			i++
		case "--limit", "--points":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			limit, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil || limit < 1 || limit > maxChannelBackupTimelineLimit {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("%s must be an integer from 1 to %d", field, maxChannelBackupTimelineLimit)
			}
			req.Options.Limit = limit
			req.LimitSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupTimelineActionRequest{}, fmt.Errorf("unknown channel backup timeline argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBackupTimelineActionRequest{}, fmt.Errorf("unexpected channel backup timeline argument %q", field)
		}
	}
	if err := applyChannelBackupTimelineIssueTarget(ev, &req); err != nil {
		return ChannelBackupTimelineActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupTimelineSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.TimelineID) == "" {
		req.Options.TimelineID = autoChannelBackupTimelineID(ev, req.Options)
		req.AutoTimelineID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupTimelineNotifyMessageID(ev, req.Options.TimelineID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupTimelineOptions(req.Options)
	if err := validateChannelBackupTimelineActionRequestOptions(req.Options); err != nil {
		return ChannelBackupTimelineActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.TimelineIDHash = shortDocumentHash(req.Options.TimelineID)
	return req, nil
}

func RunChannelBackupTimeline(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelBackupTimelineActionRequest) (ChannelBackupTimelineResult, error) {
	opts := normalizeChannelBackupTimelineOptions(req.Options)
	var err error
	opts, err = applyChannelBackupTimelineRoute(cfg, opts)
	if err != nil {
		return ChannelBackupTimelineResult{}, err
	}
	if err := validateChannelBackupTimelineOptions(opts); err != nil {
		return ChannelBackupTimelineResult{}, err
	}
	timeline, backupRoot, fetchStatus, timelineErr := loadChannelBackupTimeline(ctx, cfg, opts)
	errorKind := ""
	errorHash := ""
	if timelineErr != nil {
		errorKind = channelBackupTimelineErrorKind(timelineErr)
		errorHash = shortDocumentHash(timelineErr.Error())
	}
	body := renderChannelBackupTimelineNotificationBody(opts, timeline, fetchStatus, errorKind)
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
		return ChannelBackupTimelineResult{}, fmt.Errorf("queue channel backup timeline notification: %w", err)
	}
	return ChannelBackupTimelineResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		TimelineIDHash:      shortDocumentHash(opts.TimelineID),
		Timeline:            timeline,
		BackupFetchStatus:   fetchStatus,
		BackupRootHash:      shortDocumentHash(backupRoot),
		TimelineErrorKind:   errorKind,
		TimelineErrorHash:   errorHash,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelBackupTimelineActionReport(ev Event, req ChannelBackupTimelineActionRequest, result ChannelBackupTimelineResult) string {
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
	timelineIDHash := result.TimelineIDHash
	if timelineIDHash == "" {
		timelineIDHash = req.TimelineIDHash
	}
	timeline := result.Timeline
	if timeline.BackupTimelineStatus == "" {
		timeline = unavailableBackupTimeline(req.Options.Repo, req.Options.Limit, "")
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Timeline Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_timeline_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_timeline_status: `%s`\n", timeline.BackupTimelineStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", timeline.BackupVerifyStatus)
	fmt.Fprintf(&b, "- backup_fetch_status: `%s`\n", noneIfEmpty(result.BackupFetchStatus))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- timeline_mode: `%s`\n", "gitclaw-backups-chronology-card")
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
	fmt.Fprintf(&b, "- timeline_id_sha256_12: `%s`\n", noneIfEmpty(timelineIDHash))
	fmt.Fprintf(&b, "- timeline_id_auto: `%t`\n", req.AutoTimelineID)
	fmt.Fprintf(&b, "- limit: `%d`\n", timeline.Limit)
	fmt.Fprintf(&b, "- limit_source: `%s`\n", noneIfEmpty(req.LimitSource))
	fmt.Fprintf(&b, "- backup_root_sha256_12: `%s`\n", noneIfEmpty(result.BackupRootHash))
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(timeline.RepoDir)))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(timeline.IndexPath)))
	fmt.Fprintf(&b, "- readme_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(timeline.ReadmePath)))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", timeline.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(timeline.IndexGeneratedAt)))
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", timeline.VerificationFailures)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", timeline.IssueCount)
	fmt.Fprintf(&b, "- timeline_points: `%d`\n", timeline.TimelinePoints)
	fmt.Fprintf(&b, "- timeline_order: `%s`\n", noneIfEmpty(timeline.TimelineOrder))
	fmt.Fprintf(&b, "- timeline_window: `%s`\n", noneIfEmpty(timeline.TimelineWindow))
	fmt.Fprintf(&b, "- first_issue_sha256_12: `%s`\n", channelBackupTimelineIssueHash(timeline.FirstIssueNumber))
	fmt.Fprintf(&b, "- first_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(timeline.FirstGeneratedAt)))
	fmt.Fprintf(&b, "- latest_issue_sha256_12: `%s`\n", channelBackupTimelineIssueHash(timeline.LatestIssueNumber))
	fmt.Fprintf(&b, "- latest_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(timeline.LatestGeneratedAt)))
	fmt.Fprintf(&b, "- total_span_seconds: `%d`\n", timeline.TotalSpanSeconds)
	fmt.Fprintf(&b, "- timeline_points_sha256_12: `%s`\n", noneIfEmpty(channelBackupTimelinePointsHash(timeline.Points)))
	fmt.Fprintf(&b, "- timeline_error_kind: `%s`\n", noneIfEmpty(result.TimelineErrorKind))
	fmt.Fprintf(&b, "- timeline_error_sha256_12: `%s`\n", noneIfEmpty(result.TimelineErrorHash))
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
	fmt.Fprintf(&b, "- raw_timeline_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_root_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_timeline_points_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_titles_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_timeline_change: `%t`\n", true)
	fmt.Fprintf(&b, "- source_issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw inspected the local or read-only fetched gitclaw-backups chronology and queued a provider-facing timeline card. It reports issue numbers, backup timestamps, gaps, counts, and hashes in the provider card while this source receipt keeps raw paths, titles, bodies, comments, transcripts, prompts, tool outputs, provider IDs, and timeline IDs out of band. This action does not write the backup branch, restore files, replay GitHub APIs, call a model, call provider APIs, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read backup-timeline cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-timeline cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-timeline notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelBackupTimelineNotificationBody(opts ChannelBackupTimelineOptions, timeline BackupTimeline, fetchStatus, errorKind string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup timeline\n\n")
	fmt.Fprintf(&b, "Backup timeline status: %s\n", timeline.BackupTimelineStatus)
	fmt.Fprintf(&b, "Backup verify status: %s\n", timeline.BackupVerifyStatus)
	fmt.Fprintf(&b, "Backup branch: %s\n", defaultBackupBranch)
	fmt.Fprintf(&b, "Backup fetch status: %s\n", fetchStatus)
	if errorKind != "" {
		fmt.Fprintf(&b, "Timeline error kind: %s\n", errorKind)
	}
	fmt.Fprintf(&b, "Issue count: %d\n", timeline.IssueCount)
	fmt.Fprintf(&b, "Limit: %d\n", timeline.Limit)
	fmt.Fprintf(&b, "Timeline points: %d\n", timeline.TimelinePoints)
	fmt.Fprintf(&b, "Timeline order: %s\n", noneIfEmpty(timeline.TimelineOrder))
	fmt.Fprintf(&b, "Timeline window: %s\n", noneIfEmpty(timeline.TimelineWindow))
	if timeline.TimelinePoints > 0 {
		fmt.Fprintf(&b, "First issue: #%d\n", timeline.FirstIssueNumber)
		fmt.Fprintf(&b, "First generated at: %s\n", timeline.FirstGeneratedAt)
		fmt.Fprintf(&b, "Latest issue: #%d\n", timeline.LatestIssueNumber)
		fmt.Fprintf(&b, "Latest generated at: %s\n", timeline.LatestGeneratedAt)
		fmt.Fprintf(&b, "Total span seconds: %d\n", timeline.TotalSpanSeconds)
	} else {
		b.WriteString("First issue: none\n")
		b.WriteString("Latest issue: none\n")
		b.WriteString("Total span seconds: 0\n")
	}
	fmt.Fprintf(&b, "Timeline id hash: %s\n", shortDocumentHash(opts.TimelineID))
	b.WriteString("\nTimeline points:\n")
	if len(timeline.Points) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, point := range timeline.Points {
			fmt.Fprintf(&b, "- issue=#%d path=%s generated_at=%s event=%s gap_seconds_since_previous=%d payload_bytes=%d payload_sha256_12=%s comments=%d transcript_messages=%d assistant_turn_comments=%d error_comments=%d title_sha256_12=%s\n",
				point.IssueNumber,
				point.Path,
				point.BackupGeneratedAt,
				point.EventName,
				point.GapSecondsSincePrevious,
				point.PayloadBytes,
				point.PayloadSHA,
				point.Comments,
				point.TranscriptMessages,
				point.AssistantTurns,
				point.ErrorComments,
				point.IssueTitleSHA,
			)
		}
	}
	b.WriteString("\nRaw backup payloads, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw timeline ids are not included. Model call: not performed by this action. Repository mutation: not performed by this action. Backup branch write: not performed by this action. Restore: not performed by this action. GitHub API replay: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func loadChannelBackupTimeline(ctx context.Context, cfg Config, opts ChannelBackupTimelineOptions) (BackupTimeline, string, string, error) {
	localRoot := channelBackupSearchLocalRoot(cfg)
	if channelBackupSearchIndexExists(localRoot, opts.Repo) {
		timeline, err := BuildBackupTimeline(localRoot, opts.Repo, opts.Limit)
		if err != nil {
			return unavailableBackupTimeline(opts.Repo, opts.Limit, localRoot), localRoot, "local_error", err
		}
		return timeline, localRoot, "local", nil
	}
	worktree, cleanup, err := fetchChannelBackupSearchWorktree(ctx, cfg)
	if err != nil {
		return unavailableBackupTimeline(opts.Repo, opts.Limit, localRoot), localRoot, "unavailable", err
	}
	defer cleanup()
	fetchedRoot := filepath.Join(worktree, defaultBackupRoot)
	timeline, err := BuildBackupTimeline(fetchedRoot, opts.Repo, opts.Limit)
	if err != nil {
		return unavailableBackupTimeline(opts.Repo, opts.Limit, fetchedRoot), fetchedRoot, "fetched_error", err
	}
	return timeline, fetchedRoot, "fetched", nil
}

func unavailableBackupTimeline(repo string, limit int, root string) BackupTimeline {
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	if limit <= 0 {
		limit = defaultChannelBackupTimelineLimit
	}
	return BackupTimeline{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		Limit:                     limit,
		BackupTimelineStatus:      "unavailable",
		BackupVerifyStatus:        "unavailable",
		TimelineOrder:             "chronological",
		TimelineWindow:            "latest_by_backup_generated_at",
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
}

func channelBackupTimelineErrorKind(err error) string {
	if err == nil {
		return ""
	}
	base := channelBackupSearchErrorKind(err)
	if base != "backup_search_failed" {
		return base
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "limit must be positive"):
		return "backup_timeline_limit_invalid"
	case strings.Contains(text, "parse backup timestamp"):
		return "backup_timeline_timestamp_invalid"
	default:
		return "backup_timeline_failed"
	}
}

func channelBackupTimelineActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupTimelineActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupTimelineIssueTarget(ev Event, req *ChannelBackupTimelineActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup timeline requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupTimelineOptions(opts ChannelBackupTimelineOptions) ChannelBackupTimelineOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.TimelineID = cleanChannelBackupTimelineID(opts.TimelineID)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Limit <= 0 {
		opts.Limit = defaultChannelBackupTimelineLimit
	}
	return opts
}

func applyChannelBackupTimelineRoute(cfg Config, opts ChannelBackupTimelineOptions) (ChannelBackupTimelineOptions, error) {
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
		Body:      "GitClaw channel backup timeline.",
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

func validateChannelBackupTimelineOptions(opts ChannelBackupTimelineOptions) error {
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
	if opts.TimelineID == "" {
		return fmt.Errorf("missing backup timeline id")
	}
	if opts.Limit < 1 || opts.Limit > maxChannelBackupTimelineLimit {
		return fmt.Errorf("backup timeline limit must be from 1 to %d", maxChannelBackupTimelineLimit)
	}
	return nil
}

func validateChannelBackupTimelineActionRequestOptions(opts ChannelBackupTimelineOptions) error {
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
	if opts.TimelineID == "" {
		return fmt.Errorf("missing backup timeline id")
	}
	if opts.Limit < 1 || opts.Limit > maxChannelBackupTimelineLimit {
		return fmt.Errorf("backup timeline limit must be from 1 to %d", maxChannelBackupTimelineLimit)
	}
	return nil
}

func cleanChannelBackupTimelineSubcommand(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
}

func cleanChannelBackupTimelineID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBackupTimelineSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-timeline-source-%s", eventID(ev))
}

func autoChannelBackupTimelineID(ev Event, opts ChannelBackupTimelineOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, strconv.Itoa(opts.Limit)}, "|")
	return cleanChannelBackupTimelineID(fmt.Sprintf("backup-timeline-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelBackupTimelineNotifyMessageID(ev Event, timelineID string) string {
	seed := strings.Join([]string{eventID(ev), timelineID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-timeline-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelBackupTimelineIssueHash(issueNumber int) string {
	if issueNumber <= 0 {
		return "none"
	}
	return shortDocumentHash(strconv.Itoa(issueNumber))
}

func channelBackupTimelinePointsHash(points []BackupTimelinePoint) string {
	if len(points) == 0 {
		return "none"
	}
	lines := make([]string, 0, len(points))
	for _, point := range points {
		lines = append(lines, strings.Join([]string{
			strconv.Itoa(point.IssueNumber),
			point.Path,
			point.BackupGeneratedAt,
			point.EventName,
			strconv.FormatInt(point.GapSecondsSincePrevious, 10),
			strconv.Itoa(point.PayloadBytes),
			point.PayloadSHA,
			strconv.Itoa(point.Comments),
			strconv.Itoa(point.TranscriptMessages),
			strconv.Itoa(point.AssistantTurns),
			strconv.Itoa(point.ErrorComments),
			point.IssueTitleSHA,
		}, "|"))
	}
	return shortDocumentHash(strings.Join(lines, "\n"))
}
