package gitclaw

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultChannelBackupFreshnessMaxAgeHours = 24
const maxChannelBackupFreshnessMaxAgeHours = 87600

type ChannelBackupFreshnessOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	FreshnessID       string
	MaxAgeHours       int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupFreshnessResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	FreshnessIDHash     string
	Freshness           BackupFreshness
	BackupFetchStatus   string
	BackupRootHash      string
	FreshnessErrorKind  string
	FreshnessErrorHash  string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupFreshnessActionRequest struct {
	Options             ChannelBackupFreshnessOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoFreshnessID     bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	FreshnessIDHash     string
	MaxAgeSource        string
}

func IsChannelBackupFreshnessActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupFreshnessActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupFreshnessActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBackupFreshnessSubcommand(fields[1]) {
	case "backup-freshness", "backups-freshness", "backup-fresh", "backups-fresh", "fresh-backup", "fresh-backups", "backup-staleness", "archive-freshness", "archive-health":
		return true
	default:
		return false
	}
}

func BuildChannelBackupFreshnessActionRequest(ev Event, cfg Config) (ChannelBackupFreshnessActionRequest, error) {
	fields, _, ok := channelBackupFreshnessActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("missing channel backup freshness command")
	}
	req := ChannelBackupFreshnessActionRequest{
		Options: ChannelBackupFreshnessOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxAgeHours:       defaultChannelBackupFreshnessMaxAgeHours,
		},
		Command:      strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:   cleanChannelBackupFreshnessSubcommand(fields[1]),
		MaxAgeSource: "default",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--freshness-id", "--backup-freshness-id", "--fresh-id", "--staleness-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.FreshnessID = cleanChannelBackupFreshnessID(fields[i+1])
			i++
		case "--max-age-hours", "--max-hours", "--age-hours":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxAgeHours, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil || maxAgeHours < 1 || maxAgeHours > maxChannelBackupFreshnessMaxAgeHours {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("%s must be an integer from 1 to %d", field, maxChannelBackupFreshnessMaxAgeHours)
			}
			req.Options.MaxAgeHours = maxAgeHours
			req.MaxAgeSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("unknown channel backup freshness argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBackupFreshnessActionRequest{}, fmt.Errorf("unexpected channel backup freshness argument %q", field)
		}
	}
	if err := applyChannelBackupFreshnessIssueTarget(ev, &req); err != nil {
		return ChannelBackupFreshnessActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupFreshnessSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.FreshnessID) == "" {
		req.Options.FreshnessID = autoChannelBackupFreshnessID(ev, req.Options)
		req.AutoFreshnessID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupFreshnessNotifyMessageID(ev, req.Options.FreshnessID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupFreshnessOptions(req.Options)
	if err := validateChannelBackupFreshnessActionRequestOptions(req.Options); err != nil {
		return ChannelBackupFreshnessActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.FreshnessIDHash = shortDocumentHash(req.Options.FreshnessID)
	return req, nil
}

func RunChannelBackupFreshness(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelBackupFreshnessActionRequest) (ChannelBackupFreshnessResult, error) {
	opts := normalizeChannelBackupFreshnessOptions(req.Options)
	var err error
	opts, err = applyChannelBackupFreshnessRoute(cfg, opts)
	if err != nil {
		return ChannelBackupFreshnessResult{}, err
	}
	if err := validateChannelBackupFreshnessOptions(opts); err != nil {
		return ChannelBackupFreshnessResult{}, err
	}
	freshness, backupRoot, fetchStatus, freshnessErr := loadChannelBackupFreshness(ctx, cfg, opts)
	errorKind := ""
	errorHash := ""
	if freshnessErr != nil {
		errorKind = channelBackupFreshnessErrorKind(freshnessErr)
		errorHash = shortDocumentHash(freshnessErr.Error())
	}
	body := renderChannelBackupFreshnessNotificationBody(opts, freshness, fetchStatus, errorKind)
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
		return ChannelBackupFreshnessResult{}, fmt.Errorf("queue channel backup freshness notification: %w", err)
	}
	return ChannelBackupFreshnessResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		FreshnessIDHash:     shortDocumentHash(opts.FreshnessID),
		Freshness:           freshness,
		BackupFetchStatus:   fetchStatus,
		BackupRootHash:      shortDocumentHash(backupRoot),
		FreshnessErrorKind:  errorKind,
		FreshnessErrorHash:  errorHash,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelBackupFreshnessActionReport(ev Event, req ChannelBackupFreshnessActionRequest, result ChannelBackupFreshnessResult) string {
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
	freshnessIDHash := result.FreshnessIDHash
	if freshnessIDHash == "" {
		freshnessIDHash = req.FreshnessIDHash
	}
	freshness := result.Freshness
	if freshness.BackupFreshnessStatus == "" {
		freshness = unavailableBackupFreshness(req.Options.Repo, req.Options.MaxAgeHours, "")
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Freshness Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_freshness_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_freshness_status: `%s`\n", freshness.BackupFreshnessStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", freshness.BackupVerifyStatus)
	fmt.Fprintf(&b, "- freshness_gate: `%s`\n", noneIfEmpty(freshness.FreshnessGate))
	fmt.Fprintf(&b, "- backup_fetch_status: `%s`\n", noneIfEmpty(result.BackupFetchStatus))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- freshness_mode: `%s`\n", "gitclaw-backups-freshness-card")
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
	fmt.Fprintf(&b, "- freshness_id_sha256_12: `%s`\n", noneIfEmpty(freshnessIDHash))
	fmt.Fprintf(&b, "- freshness_id_auto: `%t`\n", req.AutoFreshnessID)
	fmt.Fprintf(&b, "- max_age_hours: `%d`\n", req.Options.MaxAgeHours)
	fmt.Fprintf(&b, "- max_age_seconds: `%d`\n", freshness.MaxAgeSeconds)
	fmt.Fprintf(&b, "- max_age_source: `%s`\n", noneIfEmpty(req.MaxAgeSource))
	fmt.Fprintf(&b, "- backup_root_sha256_12: `%s`\n", noneIfEmpty(result.BackupRootHash))
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.RepoDir)))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.IndexPath)))
	fmt.Fprintf(&b, "- readme_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.ReadmePath)))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", freshness.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.IndexGeneratedAt)))
	fmt.Fprintf(&b, "- as_of_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.AsOf)))
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", freshness.VerificationFailures)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", freshness.IssueCount)
	fmt.Fprintf(&b, "- latest_issue_sha256_12: `%s`\n", channelBackupFreshnessIssueHash(freshness.LatestIssueNumber))
	fmt.Fprintf(&b, "- latest_generated_at_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.LatestGeneratedAt)))
	fmt.Fprintf(&b, "- latest_age_seconds: `%d`\n", freshness.LatestAgeSeconds)
	fmt.Fprintf(&b, "- clock_skew_seconds: `%d`\n", freshness.ClockSkewSeconds)
	fmt.Fprintf(&b, "- latest_payload_bytes: `%d`\n", freshness.LatestPayloadBytes)
	fmt.Fprintf(&b, "- latest_payload_sha256_12: `%s`\n", noneIfEmpty(freshness.LatestPayloadSHA))
	fmt.Fprintf(&b, "- latest_event_name_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(freshness.LatestEventName)))
	fmt.Fprintf(&b, "- latest_issue_title_sha256_12: `%s`\n", noneIfEmpty(freshness.LatestIssueTitleSHA))
	fmt.Fprintf(&b, "- freshness_error_kind: `%s`\n", noneIfEmpty(result.FreshnessErrorKind))
	fmt.Fprintf(&b, "- freshness_error_sha256_12: `%s`\n", noneIfEmpty(result.FreshnessErrorHash))
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
	fmt.Fprintf(&b, "- raw_freshness_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_root_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_titles_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_freshness_change: `%t`\n", true)
	fmt.Fprintf(&b, "- source_issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw inspected the local or read-only fetched gitclaw-backups freshness gate and queued a provider-facing freshness card. It reports status, age, counts, timestamps, and hashes while keeping raw paths, titles, bodies, comments, transcripts, prompts, tool outputs, provider IDs, and freshness IDs out of the source receipt. This action does not write the backup branch, restore files, replay GitHub APIs, call a model, call provider APIs, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read backup-freshness cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-freshness cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-freshness notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelBackupFreshnessNotificationBody(opts ChannelBackupFreshnessOptions, freshness BackupFreshness, fetchStatus, errorKind string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup freshness\n\n")
	fmt.Fprintf(&b, "Backup freshness status: %s\n", freshness.BackupFreshnessStatus)
	fmt.Fprintf(&b, "Backup verify status: %s\n", freshness.BackupVerifyStatus)
	fmt.Fprintf(&b, "Freshness gate: %s\n", noneIfEmpty(freshness.FreshnessGate))
	fmt.Fprintf(&b, "Backup branch: %s\n", defaultBackupBranch)
	fmt.Fprintf(&b, "Backup fetch status: %s\n", fetchStatus)
	if errorKind != "" {
		fmt.Fprintf(&b, "Freshness error kind: %s\n", errorKind)
	}
	fmt.Fprintf(&b, "Issue count: %d\n", freshness.IssueCount)
	fmt.Fprintf(&b, "Max age hours: %d\n", opts.MaxAgeHours)
	fmt.Fprintf(&b, "Max age seconds: %d\n", freshness.MaxAgeSeconds)
	if freshness.LatestIssueNumber > 0 {
		fmt.Fprintf(&b, "Latest issue: #%d\n", freshness.LatestIssueNumber)
		fmt.Fprintf(&b, "Latest backup generated at: %s\n", freshness.LatestGeneratedAt)
		fmt.Fprintf(&b, "Latest age seconds: %d\n", freshness.LatestAgeSeconds)
		fmt.Fprintf(&b, "Clock skew seconds: %d\n", freshness.ClockSkewSeconds)
		fmt.Fprintf(&b, "Latest payload bytes: %d\n", freshness.LatestPayloadBytes)
		fmt.Fprintf(&b, "Latest payload sha256_12: %s\n", freshness.LatestPayloadSHA)
		fmt.Fprintf(&b, "Latest event hash: %s\n", shortDocumentHash(freshness.LatestEventName))
		fmt.Fprintf(&b, "Latest issue title hash: %s\n", freshness.LatestIssueTitleSHA)
	} else {
		b.WriteString("Latest issue: none\n")
		b.WriteString("Latest age seconds: 0\n")
		b.WriteString("Clock skew seconds: 0\n")
	}
	fmt.Fprintf(&b, "Freshness id hash: %s\n", shortDocumentHash(opts.FreshnessID))
	b.WriteString("\nRaw backup payloads, backup paths, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw freshness ids are not included. Model call: not performed by this action. Repository mutation: not performed by this action. Backup branch write: not performed by this action. Restore: not performed by this action. GitHub API replay: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func loadChannelBackupFreshness(ctx context.Context, cfg Config, opts ChannelBackupFreshnessOptions) (BackupFreshness, string, string, error) {
	localRoot := channelBackupSearchLocalRoot(cfg)
	maxAge := time.Duration(opts.MaxAgeHours) * time.Hour
	now := time.Now().UTC()
	if channelBackupSearchIndexExists(localRoot, opts.Repo) {
		freshness, err := BuildBackupFreshness(localRoot, opts.Repo, maxAge, now)
		if err != nil {
			return unavailableBackupFreshness(opts.Repo, opts.MaxAgeHours, localRoot), localRoot, "local_error", err
		}
		return freshness, localRoot, "local", nil
	}
	worktree, cleanup, err := fetchChannelBackupSearchWorktree(ctx, cfg)
	if err != nil {
		return unavailableBackupFreshness(opts.Repo, opts.MaxAgeHours, localRoot), localRoot, "unavailable", err
	}
	defer cleanup()
	fetchedRoot := filepath.Join(worktree, defaultBackupRoot)
	freshness, err := BuildBackupFreshness(fetchedRoot, opts.Repo, maxAge, now)
	if err != nil {
		return unavailableBackupFreshness(opts.Repo, opts.MaxAgeHours, fetchedRoot), fetchedRoot, "fetched_error", err
	}
	return freshness, fetchedRoot, "fetched", nil
}

func unavailableBackupFreshness(repo string, maxAgeHours int, root string) BackupFreshness {
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	if maxAgeHours <= 0 {
		maxAgeHours = defaultChannelBackupFreshnessMaxAgeHours
	}
	return BackupFreshness{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		BackupFreshnessStatus:     "unavailable",
		BackupVerifyStatus:        "unavailable",
		FreshnessGate:             "fail",
		MaxAgeSeconds:             int64(maxAgeHours) * int64(time.Hour/time.Second),
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
}

func channelBackupFreshnessErrorKind(err error) string {
	if err == nil {
		return ""
	}
	base := channelBackupSearchErrorKind(err)
	if base != "backup_search_failed" {
		return base
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "parse backup timestamp"):
		return "backup_freshness_timestamp_invalid"
	case strings.Contains(text, "backup freshness reported"):
		return "backup_freshness_gate_failed"
	default:
		return "backup_freshness_failed"
	}
}

func channelBackupFreshnessActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupFreshnessActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupFreshnessIssueTarget(ev Event, req *ChannelBackupFreshnessActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup freshness requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupFreshnessOptions(opts ChannelBackupFreshnessOptions) ChannelBackupFreshnessOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.FreshnessID = cleanChannelBackupFreshnessID(opts.FreshnessID)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxAgeHours <= 0 {
		opts.MaxAgeHours = defaultChannelBackupFreshnessMaxAgeHours
	}
	return opts
}

func applyChannelBackupFreshnessRoute(cfg Config, opts ChannelBackupFreshnessOptions) (ChannelBackupFreshnessOptions, error) {
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
		Body:      "GitClaw channel backup freshness.",
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

func validateChannelBackupFreshnessOptions(opts ChannelBackupFreshnessOptions) error {
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
	if opts.FreshnessID == "" {
		return fmt.Errorf("missing backup freshness id")
	}
	if opts.MaxAgeHours < 1 || opts.MaxAgeHours > maxChannelBackupFreshnessMaxAgeHours {
		return fmt.Errorf("backup freshness max age hours must be from 1 to %d", maxChannelBackupFreshnessMaxAgeHours)
	}
	return nil
}

func validateChannelBackupFreshnessActionRequestOptions(opts ChannelBackupFreshnessOptions) error {
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
	if opts.FreshnessID == "" {
		return fmt.Errorf("missing backup freshness id")
	}
	if opts.MaxAgeHours < 1 || opts.MaxAgeHours > maxChannelBackupFreshnessMaxAgeHours {
		return fmt.Errorf("backup freshness max age hours must be from 1 to %d", maxChannelBackupFreshnessMaxAgeHours)
	}
	return nil
}

func cleanChannelBackupFreshnessSubcommand(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
}

func cleanChannelBackupFreshnessID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBackupFreshnessSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-freshness-source-%s", eventID(ev))
}

func autoChannelBackupFreshnessID(ev Event, opts ChannelBackupFreshnessOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, strconv.Itoa(opts.MaxAgeHours)}, "|")
	return cleanChannelBackupFreshnessID(fmt.Sprintf("backup-freshness-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelBackupFreshnessNotifyMessageID(ev Event, freshnessID string) string {
	seed := strings.Join([]string{eventID(ev), freshnessID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-freshness-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelBackupFreshnessIssueHash(issueNumber int) string {
	if issueNumber <= 0 {
		return "none"
	}
	return shortDocumentHash(strconv.Itoa(issueNumber))
}
