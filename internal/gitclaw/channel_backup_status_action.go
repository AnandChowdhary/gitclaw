package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBackupStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelBackupStatusResult struct {
	Notification                  ChannelSendResult
	RouteName                     string
	RouteHash                     string
	Channel                       string
	ThreadHash                    string
	MessageHash                   string
	NotifyHash                    string
	StatusIDHash                  string
	BodyHash                      string
	BackupBranch                  string
	BackupRoot                    string
	BackupSchemaVersion           int
	RepoBackupDirHash             string
	IndexPathHash                 string
	ReadmePathHash                string
	BackupDocsPathHash            string
	BackupDocsPresent             bool
	BackupDocsBytes               int
	BackupDocsLines               int
	BackupDocsHash                string
	CatalogEntries                int
	FetchedBranchRequiredCommands int
	MetadataOnlyCommands          int
	RawRecoveryCommands           int
	ProviderVisibleActions        int
	CatalogCommandNamesHash       string
	BackupStatusSnapshotHash      string
}

type ChannelBackupStatusActionRequest struct {
	Options                       ChannelBackupStatusOptions
	Command                       string
	Subcommand                    string
	AutoSourceMessageID           bool
	AutoNotifyMessageID           bool
	AutoStatusID                  bool
	TargetFromIssue               bool
	RequestedRouteHash            string
	RequestedThreadHash           string
	RequestedMsgHash              string
	NotifyMessageHash             string
	StatusIDHash                  string
	NotificationBodySHA           string
	BackupBranch                  string
	BackupRoot                    string
	BackupSchemaVersion           int
	RepoBackupDirHash             string
	IndexPathHash                 string
	ReadmePathHash                string
	BackupDocsPathHash            string
	BackupDocsPresent             bool
	BackupDocsBytes               int
	BackupDocsLines               int
	BackupDocsHash                string
	CatalogEntries                int
	FetchedBranchRequiredCommands int
	MetadataOnlyCommands          int
	RawRecoveryCommands           int
	ProviderVisibleActions        int
	CatalogCommandNamesHash       string
	BackupStatusSnapshotHash      string
}

type channelBackupStatusSnapshot struct {
	BackupBranch                  string
	BackupRoot                    string
	BackupSchemaVersion           int
	RepoBackupDirHash             string
	IndexPathHash                 string
	ReadmePathHash                string
	BackupDocsPathHash            string
	BackupDocsPresent             bool
	BackupDocsBytes               int
	BackupDocsLines               int
	BackupDocsHash                string
	CatalogEntries                int
	FetchedBranchRequiredCommands int
	MetadataOnlyCommands          int
	RawRecoveryCommands           int
	ProviderVisibleActions        int
	CatalogCommandNamesHash       string
	BackupStatusSnapshotHash      string
}

func IsChannelBackupStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelBackupStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelBackupStatusActionFields(fields)
}

func isChannelBackupStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "backup", "backups", "backup-status", "backup-health", "backup-summary", "backup-state", "recovery-status", "recovery-health":
		return true
	default:
		return false
	}
}

func BuildChannelBackupStatusActionRequest(ev Event, cfg Config) (ChannelBackupStatusActionRequest, error) {
	fields, _, ok := channelBackupStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupStatusActionRequest{}, fmt.Errorf("missing channel backup status command")
	}
	req := ChannelBackupStatusActionRequest{
		Options: ChannelBackupStatusOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--backup-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelBackupStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupStatusActionRequest{}, fmt.Errorf("unknown channel backup status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBackupStatusActionRequest{}, fmt.Errorf("unexpected channel backup status argument %q", field)
		}
	}
	if err := applyChannelBackupStatusIssueTarget(ev, &req); err != nil {
		return ChannelBackupStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelBackupStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupStatusOptions(req.Options)
	if err := validateChannelBackupStatusActionRequestOptions(req.Options); err != nil {
		return ChannelBackupStatusActionRequest{}, err
	}
	snapshot := buildChannelBackupStatusSnapshot(cfg, req.Options.Repo)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelBackupStatusNotificationBody(req.Options, snapshot))
	req.BackupBranch = snapshot.BackupBranch
	req.BackupRoot = snapshot.BackupRoot
	req.BackupSchemaVersion = snapshot.BackupSchemaVersion
	req.RepoBackupDirHash = snapshot.RepoBackupDirHash
	req.IndexPathHash = snapshot.IndexPathHash
	req.ReadmePathHash = snapshot.ReadmePathHash
	req.BackupDocsPathHash = snapshot.BackupDocsPathHash
	req.BackupDocsPresent = snapshot.BackupDocsPresent
	req.BackupDocsBytes = snapshot.BackupDocsBytes
	req.BackupDocsLines = snapshot.BackupDocsLines
	req.BackupDocsHash = snapshot.BackupDocsHash
	req.CatalogEntries = snapshot.CatalogEntries
	req.FetchedBranchRequiredCommands = snapshot.FetchedBranchRequiredCommands
	req.MetadataOnlyCommands = snapshot.MetadataOnlyCommands
	req.RawRecoveryCommands = snapshot.RawRecoveryCommands
	req.ProviderVisibleActions = snapshot.ProviderVisibleActions
	req.CatalogCommandNamesHash = snapshot.CatalogCommandNamesHash
	req.BackupStatusSnapshotHash = snapshot.BackupStatusSnapshotHash
	return req, nil
}

func RunChannelBackupStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBackupStatusOptions) (ChannelBackupStatusResult, error) {
	opts = normalizeChannelBackupStatusOptions(opts)
	var err error
	opts, err = applyChannelBackupStatusRoute(cfg, opts)
	if err != nil {
		return ChannelBackupStatusResult{}, err
	}
	if err := validateChannelBackupStatusOptions(opts); err != nil {
		return ChannelBackupStatusResult{}, err
	}
	snapshot := buildChannelBackupStatusSnapshot(cfg, opts.Repo)
	body := renderChannelBackupStatusNotificationBody(opts, snapshot)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelBackupStatusResult{}, fmt.Errorf("queue channel backup status notification: %w", err)
	}
	return ChannelBackupStatusResult{
		Notification:                  notification,
		RouteName:                     opts.Route,
		RouteHash:                     channelRouteHash(opts.Route),
		Channel:                       opts.Channel,
		ThreadHash:                    shortDocumentHash(opts.ThreadID),
		MessageHash:                   shortDocumentHash(opts.SourceMessageID),
		NotifyHash:                    shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash:                  shortDocumentHash(opts.StatusID),
		BodyHash:                      shortDocumentHash(body),
		BackupBranch:                  snapshot.BackupBranch,
		BackupRoot:                    snapshot.BackupRoot,
		BackupSchemaVersion:           snapshot.BackupSchemaVersion,
		RepoBackupDirHash:             snapshot.RepoBackupDirHash,
		IndexPathHash:                 snapshot.IndexPathHash,
		ReadmePathHash:                snapshot.ReadmePathHash,
		BackupDocsPathHash:            snapshot.BackupDocsPathHash,
		BackupDocsPresent:             snapshot.BackupDocsPresent,
		BackupDocsBytes:               snapshot.BackupDocsBytes,
		BackupDocsLines:               snapshot.BackupDocsLines,
		BackupDocsHash:                snapshot.BackupDocsHash,
		CatalogEntries:                snapshot.CatalogEntries,
		FetchedBranchRequiredCommands: snapshot.FetchedBranchRequiredCommands,
		MetadataOnlyCommands:          snapshot.MetadataOnlyCommands,
		RawRecoveryCommands:           snapshot.RawRecoveryCommands,
		ProviderVisibleActions:        snapshot.ProviderVisibleActions,
		CatalogCommandNamesHash:       snapshot.CatalogCommandNamesHash,
		BackupStatusSnapshotHash:      snapshot.BackupStatusSnapshotHash,
	}, nil
}

func RenderChannelBackupStatusActionReport(ev Event, req ChannelBackupStatusActionRequest, result ChannelBackupStatusResult) string {
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_snapshot_mode: `%s`\n", "provider-facing-backup-status")
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
	fmt.Fprintf(&b, "- backup_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- backup_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", firstNonEmpty(result.BackupBranch, req.BackupBranch, defaultBackupBranch))
	fmt.Fprintf(&b, "- backup_root: `%s`\n", firstNonEmpty(result.BackupRoot, req.BackupRoot, defaultBackupRoot))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", nonzeroOrReq(result.BackupSchemaVersion, req.BackupSchemaVersion))
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.RepoBackupDirHash, req.RepoBackupDirHash)))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.IndexPathHash, req.IndexPathHash)))
	fmt.Fprintf(&b, "- readme_path_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.ReadmePathHash, req.ReadmePathHash)))
	fmt.Fprintf(&b, "- backup_docs_path_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.BackupDocsPathHash, req.BackupDocsPathHash)))
	fmt.Fprintf(&b, "- backup_docs_present: `%t`\n", result.BackupDocsPresent || req.BackupDocsPresent)
	fmt.Fprintf(&b, "- backup_docs_bytes: `%d`\n", nonzeroOrReq(result.BackupDocsBytes, req.BackupDocsBytes))
	fmt.Fprintf(&b, "- backup_docs_lines: `%d`\n", nonzeroOrReq(result.BackupDocsLines, req.BackupDocsLines))
	fmt.Fprintf(&b, "- backup_docs_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.BackupDocsHash, req.BackupDocsHash)))
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", nonzeroOrReq(result.CatalogEntries, req.CatalogEntries))
	fmt.Fprintf(&b, "- fetched_branch_required_commands: `%d`\n", nonzeroOrReq(result.FetchedBranchRequiredCommands, req.FetchedBranchRequiredCommands))
	fmt.Fprintf(&b, "- metadata_only_commands: `%d`\n", nonzeroOrReq(result.MetadataOnlyCommands, req.MetadataOnlyCommands))
	fmt.Fprintf(&b, "- raw_recovery_commands: `%d`\n", nonzeroOrReq(result.RawRecoveryCommands, req.RawRecoveryCommands))
	fmt.Fprintf(&b, "- provider_visible_backup_actions: `%d`\n", nonzeroOrReq(result.ProviderVisibleActions, req.ProviderVisibleActions))
	fmt.Fprintf(&b, "- catalog_command_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.CatalogCommandNamesHash, req.CatalogCommandNamesHash)))
	fmt.Fprintf(&b, "- backup_status_snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.BackupStatusSnapshotHash, req.BackupStatusSnapshotHash)))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- backup_branch_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_read: `%t`\n", false)
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
	fmt.Fprintf(&b, "- raw_backup_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_repo_backup_dir_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_index_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_readme_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing backup status snapshot on the canonical channel issue. This is the GitHub-native channel version of a backup cockpit: it reports branch, root, schema, catalog, and local backup-doc metadata, but it does not fetch the backup branch, read backup payloads, restore files, write backup state, replay GitHub APIs, call a model, mutate repository files, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, repo backup paths, raw backup payloads, issue bodies, comment bodies, transcripts, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the backup-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/channels rehearse-backup` or `/channels restore-request` when a channel message should become a recovery workflow\n")
	return strings.TrimSpace(b.String())
}

func channelBackupStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupStatusIssueTarget(ev Event, req *ChannelBackupStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupStatusOptions(opts ChannelBackupStatusOptions) ChannelBackupStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelBackupStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBackupStatusRoute(cfg Config, opts ChannelBackupStatusOptions) (ChannelBackupStatusOptions, error) {
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
		Body:      "GitClaw channel backup status.",
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

func validateChannelBackupStatusOptions(opts ChannelBackupStatusOptions) error {
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
		return fmt.Errorf("missing backup status id")
	}
	return nil
}

func validateChannelBackupStatusActionRequestOptions(opts ChannelBackupStatusOptions) error {
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
		return fmt.Errorf("missing backup status id")
	}
	return nil
}

func cleanChannelBackupStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBackupStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-source-%s", eventID(ev))
}

func autoChannelBackupStatusID(ev Event, opts ChannelBackupStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("backup-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBackupStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelBackupStatusNotificationBody(opts ChannelBackupStatusOptions, snapshot channelBackupStatusSnapshot) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup status.\n\n")
	fmt.Fprintf(&b, "Backup branch: %s\n", snapshot.BackupBranch)
	fmt.Fprintf(&b, "Backup root: %s\n", snapshot.BackupRoot)
	fmt.Fprintf(&b, "Schema version: %d\n", snapshot.BackupSchemaVersion)
	fmt.Fprintf(&b, "Catalog commands: %d\n", snapshot.CatalogEntries)
	fmt.Fprintf(&b, "Fetched-branch inspection commands: %d\n", snapshot.FetchedBranchRequiredCommands)
	fmt.Fprintf(&b, "Metadata-only commands: %d\n", snapshot.MetadataOnlyCommands)
	fmt.Fprintf(&b, "Raw recovery commands: %d\n", snapshot.RawRecoveryCommands)
	fmt.Fprintf(&b, "Channel backup actions: %s\n", strings.Join(channelBackupStatusProviderActions(), ", "))
	fmt.Fprintf(&b, "Backup docs: %s\n", channelBackupStatusPresentLabel(snapshot.BackupDocsPresent))
	b.WriteString("Latest backup freshness: requires fetched backup branch\n")
	b.WriteString("\nRaw backup payloads: not read by this action.\n")
	b.WriteString("Backup branch fetch: not performed by this action.\n")
	b.WriteString("Restore: not performed by this action.\n")
	b.WriteString("Backup branch write: not performed by this action.\n")
	b.WriteString("GitHub API replay: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelBackupStatusSnapshot(cfg Config, repo string) channelBackupStatusSnapshot {
	repo = backupReportRepo(repo)
	paths := backupCatalogPaths(defaultBackupRoot, repo, 0)
	entries := backupCatalogEntries(defaultBackupRoot, repo)
	docsPath := defaultBackupRoot + "/README.md"
	docsBody, docsErr := readRepoTextFile(rootOrDot(cfg.Workdir), docsPath, maxContextDocumentBytes)
	snapshot := channelBackupStatusSnapshot{
		BackupBranch:                  defaultBackupBranch,
		BackupRoot:                    defaultBackupRoot,
		BackupSchemaVersion:           1,
		RepoBackupDirHash:             shortDocumentHash(paths.RepoDir),
		IndexPathHash:                 shortDocumentHash(paths.IndexPath),
		ReadmePathHash:                shortDocumentHash(paths.ReadmePath),
		BackupDocsPathHash:            shortDocumentHash(docsPath),
		BackupDocsPresent:             docsErr == nil,
		CatalogEntries:                len(entries),
		FetchedBranchRequiredCommands: countFetchedBackupCatalogEntries(entries),
		MetadataOnlyCommands:          countBackupCatalogEntriesWithExecution(entries, "metadata-only"),
		RawRecoveryCommands:           countBackupCatalogEntriesWithExecution(entries, "explicit-local-raw-recovery"),
		ProviderVisibleActions:        len(channelBackupStatusProviderActions()),
		CatalogCommandNamesHash:       hashStringList(backupCatalogCommandNames(entries)),
	}
	if docsErr == nil {
		snapshot.BackupDocsBytes = len(docsBody)
		snapshot.BackupDocsLines = lineCount(docsBody)
		snapshot.BackupDocsHash = shortDocumentHash(docsBody)
	}
	snapshot.BackupStatusSnapshotHash = channelBackupStatusSnapshotHash(snapshot)
	return snapshot
}

func countBackupCatalogEntriesWithExecution(entries []backupCatalogEntry, execution string) int {
	count := 0
	for _, entry := range entries {
		if entry.Execution == execution {
			count++
		}
	}
	return count
}

func backupCatalogCommandNames(entries []backupCatalogEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func channelBackupStatusProviderActions() []string {
	return []string{"status", "recovery-map", "rehearse-backup", "restore-request"}
}

func channelBackupStatusPresentLabel(present bool) string {
	if present {
		return "present"
	}
	return "missing"
}

func channelBackupStatusSnapshotHash(snapshot channelBackupStatusSnapshot) string {
	parts := []string{
		snapshot.BackupBranch,
		snapshot.BackupRoot,
		fmt.Sprintf("%d", snapshot.BackupSchemaVersion),
		snapshot.RepoBackupDirHash,
		snapshot.IndexPathHash,
		snapshot.ReadmePathHash,
		snapshot.BackupDocsPathHash,
		fmt.Sprintf("%t", snapshot.BackupDocsPresent),
		fmt.Sprintf("%d", snapshot.BackupDocsBytes),
		fmt.Sprintf("%d", snapshot.BackupDocsLines),
		snapshot.BackupDocsHash,
		fmt.Sprintf("%d", snapshot.CatalogEntries),
		fmt.Sprintf("%d", snapshot.FetchedBranchRequiredCommands),
		fmt.Sprintf("%d", snapshot.MetadataOnlyCommands),
		fmt.Sprintf("%d", snapshot.RawRecoveryCommands),
		fmt.Sprintf("%d", snapshot.ProviderVisibleActions),
		snapshot.CatalogCommandNamesHash,
	}
	return shortDocumentHash(strings.Join(parts, "\n"))
}
