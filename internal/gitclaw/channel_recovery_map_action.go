package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRecoveryMapOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MapID             string
	Scope             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRecoveryMapResult struct {
	Notification                  ChannelSendResult
	RouteName                     string
	RouteHash                     string
	Channel                       string
	ThreadHash                    string
	MessageHash                   string
	NotifyHash                    string
	MapIDHash                     string
	ScopeHash                     string
	NoteHash                      string
	BodyHash                      string
	StepHash                      string
	SnapshotHash                  string
	StepCount                     int
	BackupBranch                  string
	BackupRoot                    string
	BackupSchemaVersion           int
	BackupDocsPresent             bool
	CatalogEntries                int
	FetchedBranchRequiredCommands int
	MetadataOnlyCommands          int
	RawRecoveryCommands           int
	ProviderVisibleActions        int
}

type ChannelRecoveryMapActionRequest struct {
	Options                       ChannelRecoveryMapOptions
	Command                       string
	Subcommand                    string
	AutoSourceMessageID           bool
	AutoNotifyMessageID           bool
	AutoMapID                     bool
	TargetFromIssue               bool
	NoteSource                    string
	RequestedRouteHash            string
	RequestedThreadHash           string
	RequestedMsgHash              string
	NotifyMessageHash             string
	MapIDHash                     string
	ScopeSHA                      string
	ScopeBytes                    int
	NoteSHA                       string
	NoteBytes                     int
	NoteLines                     int
	StepSHA                       string
	SnapshotSHA                   string
	StepCount                     int
	NotificationBodySHA           string
	BackupBranch                  string
	BackupRoot                    string
	BackupSchemaVersion           int
	BackupDocsPresent             bool
	CatalogEntries                int
	FetchedBranchRequiredCommands int
	MetadataOnlyCommands          int
	RawRecoveryCommands           int
	ProviderVisibleActions        int
}

type channelRecoveryMapSnapshot struct {
	BackupBranch                  string
	BackupRoot                    string
	BackupSchemaVersion           int
	BackupDocsPresent             bool
	CatalogEntries                int
	FetchedBranchRequiredCommands int
	MetadataOnlyCommands          int
	RawRecoveryCommands           int
	ProviderVisibleActions        int
	StepCount                     int
	StepHash                      string
	SnapshotHash                  string
}

type channelRecoveryMapStep struct {
	Command string
	Reason  string
}

func IsChannelRecoveryMapActionRequest(ev Event, cfg Config) bool {
	return isChannelRecoveryMapActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRecoveryMapActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "recovery-map", "restore-map", "backup-map", "recovery-path", "restore-path", "backup-path", "recovery-flow", "restore-flow":
		return true
	default:
		return false
	}
}

func BuildChannelRecoveryMapActionRequest(ev Event, cfg Config) (ChannelRecoveryMapActionRequest, error) {
	fields, trailing, ok := channelRecoveryMapActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRecoveryMapActionRequest{}, fmt.Errorf("missing channel recovery map command")
	}
	req := ChannelRecoveryMapActionRequest{
		Options: ChannelRecoveryMapOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Scope:             "issue",
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--map-id", "--recovery-map-id", "--restore-map-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MapID = cleanChannelRecoveryMapID(fields[i+1])
			i++
		case "--scope", "--for":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Scope = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRecoveryMapActionRequest{}, fmt.Errorf("unknown channel recovery map argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelRecoveryMapIssueTargetIfPresent(ev, &req)
	if err := applyChannelRecoveryMapPositionals(&req, positional); err != nil {
		return ChannelRecoveryMapActionRequest{}, err
	}
	if err := applyChannelRecoveryMapIssueTarget(ev, &req); err != nil {
		return ChannelRecoveryMapActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelRecoveryMapTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelRecoveryMapSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MapID) == "" {
		req.Options.MapID = autoChannelRecoveryMapID(ev, req.Options)
		req.AutoMapID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRecoveryMapNotifyMessageID(ev, req.Options.MapID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRecoveryMapOptions(req.Options)
	if err := validateChannelRecoveryMapActionRequestOptions(req.Options); err != nil {
		return ChannelRecoveryMapActionRequest{}, err
	}
	snapshot := buildChannelRecoveryMapSnapshot(cfg, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MapIDHash = shortDocumentHash(req.Options.MapID)
	req.ScopeSHA = shortDocumentHash(req.Options.Scope)
	req.ScopeBytes = len(req.Options.Scope)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelRecoveryMapNotificationBody(req.Options, cfg))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelRecoveryMap(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRecoveryMapOptions) (ChannelRecoveryMapResult, error) {
	opts = normalizeChannelRecoveryMapOptions(opts)
	var err error
	opts, err = applyChannelRecoveryMapRoute(cfg, opts)
	if err != nil {
		return ChannelRecoveryMapResult{}, err
	}
	if err := validateChannelRecoveryMapOptions(opts); err != nil {
		return ChannelRecoveryMapResult{}, err
	}
	body := renderChannelRecoveryMapNotificationBody(opts, cfg)
	snapshot := buildChannelRecoveryMapSnapshot(cfg, opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelRecoveryMapResult{}, fmt.Errorf("queue channel recovery map notification: %w", err)
	}
	result := ChannelRecoveryMapResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		MapIDHash:    shortDocumentHash(opts.MapID),
		ScopeHash:    shortDocumentHash(opts.Scope),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
		StepHash:     snapshot.StepHash,
		SnapshotHash: snapshot.SnapshotHash,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelRecoveryMapActionReport(ev Event, req ChannelRecoveryMapActionRequest, result ChannelRecoveryMapResult) string {
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
	mapIDHash := result.MapIDHash
	if mapIDHash == "" {
		mapIDHash = req.MapIDHash
	}
	scopeHash := result.ScopeHash
	if scopeHash == "" {
		scopeHash = req.ScopeSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	stepHash := result.StepHash
	if stepHash == "" {
		stepHash = req.StepSHA
	}
	snapshotHash := result.SnapshotHash
	if snapshotHash == "" {
		snapshotHash = req.SnapshotSHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Recovery Map Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_recovery_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- recovery_map_mode: `%s`\n", "provider-facing-recovery-sequence")
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
	fmt.Fprintf(&b, "- recovery_map_id_sha256_12: `%s`\n", noneIfEmpty(mapIDHash))
	fmt.Fprintf(&b, "- recovery_map_id_auto: `%t`\n", req.AutoMapID)
	fmt.Fprintf(&b, "- recovery_scope_sha256_12: `%s`\n", noneIfEmpty(scopeHash))
	fmt.Fprintf(&b, "- recovery_scope_bytes: `%d`\n", req.ScopeBytes)
	fmt.Fprintf(&b, "- recovery_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- recovery_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- recovery_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- recovery_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- recovery_step_count: `%d`\n", nonzeroOrReq(result.StepCount, req.StepCount))
	fmt.Fprintf(&b, "- recovery_step_sha256_12: `%s`\n", noneIfEmpty(stepHash))
	fmt.Fprintf(&b, "- recovery_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", firstNonEmpty(result.BackupBranch, req.BackupBranch, defaultBackupBranch))
	fmt.Fprintf(&b, "- backup_root: `%s`\n", firstNonEmpty(result.BackupRoot, req.BackupRoot, defaultBackupRoot))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", nonzeroOrReq(result.BackupSchemaVersion, req.BackupSchemaVersion))
	fmt.Fprintf(&b, "- backup_docs_present: `%t`\n", result.BackupDocsPresent || req.BackupDocsPresent)
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", nonzeroOrReq(result.CatalogEntries, req.CatalogEntries))
	fmt.Fprintf(&b, "- fetched_branch_required_commands: `%d`\n", nonzeroOrReq(result.FetchedBranchRequiredCommands, req.FetchedBranchRequiredCommands))
	fmt.Fprintf(&b, "- metadata_only_commands: `%d`\n", nonzeroOrReq(result.MetadataOnlyCommands, req.MetadataOnlyCommands))
	fmt.Fprintf(&b, "- raw_recovery_commands: `%d`\n", nonzeroOrReq(result.RawRecoveryCommands, req.RawRecoveryCommands))
	fmt.Fprintf(&b, "- provider_visible_backup_actions: `%d`\n", nonzeroOrReq(result.ProviderVisibleActions, req.ProviderVisibleActions))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- backup_branch_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_read: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_replay_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_request_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_recovery_map_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_recovery_scope_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_recovery_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_recovery_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_recovery_map_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing recovery map on the canonical channel issue. This gives Slack or Telegram a safe recovery sequence from repo-local backup catalog metadata while keeping thread ids, message ids, map ids, scopes, notes, step text, issue bodies, comment bodies, backup payloads, and channel bodies out of the source receipt. The action does not fetch the backup branch, read backup payloads, restore files, create rehearsal issues, create restore-request issues, replay GitHub APIs, call a model, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read recovery-map cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent recovery-map cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate recovery-map cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelRecoveryMapActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRecoveryMapActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRecoveryMapIssueTarget(ev Event, req *ChannelRecoveryMapActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel recovery map requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelRecoveryMapIssueTargetIfPresent(ev Event, req *ChannelRecoveryMapActionRequest) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
}

func applyChannelRecoveryMapPositionals(req *ChannelRecoveryMapActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Scope == "" || req.Options.Scope == "issue" {
				req.Options.Scope = value
				continue
			}
			return fmt.Errorf("unexpected channel recovery map argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Scope == "" || req.Options.Scope == "issue" {
			req.Options.Scope = value
			continue
		}
		return fmt.Errorf("unexpected channel recovery map argument %q", value)
	}
	return nil
}

func normalizeChannelRecoveryMapOptions(opts ChannelRecoveryMapOptions) ChannelRecoveryMapOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MapID = cleanChannelRecoveryMapID(opts.MapID)
	opts.Scope = cleanChannelRecoveryMapScope(opts.Scope)
	opts.Note = cleanChannelRecoveryMapNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelRecoveryMapRoute(cfg Config, opts ChannelRecoveryMapOptions) (ChannelRecoveryMapOptions, error) {
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
		Body:      "GitClaw channel recovery map.",
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

func validateChannelRecoveryMapOptions(opts ChannelRecoveryMapOptions) error {
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
	if opts.MapID == "" {
		return fmt.Errorf("missing recovery map id")
	}
	if opts.Scope == "" {
		return fmt.Errorf("missing recovery map scope")
	}
	return nil
}

func validateChannelRecoveryMapActionRequestOptions(opts ChannelRecoveryMapOptions) error {
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
	if opts.MapID == "" {
		return fmt.Errorf("missing recovery map id")
	}
	if opts.Scope == "" {
		return fmt.Errorf("missing recovery map scope")
	}
	return nil
}

func cleanChannelRecoveryMapID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelRecoveryMapScope(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	switch value {
	case "", "current", "source":
		return "issue"
	case "repository":
		return "repo"
	case "thread", "chat":
		return "channel"
	case "incident-response", "outage":
		return "incident"
	default:
		if len(value) > 32 {
			value = strings.Trim(value[:32], "-")
		}
		return value
	}
}

func cleanChannelRecoveryMapNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelRecoveryMapTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelRecoveryMapTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelRecoveryMapNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelRecoveryMapTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func autoChannelRecoveryMapSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-recovery-map-source-%s", eventID(ev))
}

func autoChannelRecoveryMapID(ev Event, opts ChannelRecoveryMapOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Scope, opts.Note}, "|")
	return fmt.Sprintf("recovery-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRecoveryMapNotifyMessageID(ev Event, mapID string) string {
	seed := strings.Join([]string{eventID(ev), mapID}, "|")
	return fmt.Sprintf("gitclaw-channel-recovery-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelRecoveryMapNotificationBody(opts ChannelRecoveryMapOptions, cfg Config) string {
	opts = normalizeChannelRecoveryMapOptions(opts)
	snapshot := buildChannelRecoveryMapSnapshot(cfg, opts)
	steps := channelRecoveryMapStepsForScope(opts.Scope, snapshot)
	var b strings.Builder
	b.WriteString("GitClaw channel recovery map.\n\n")
	fmt.Fprintf(&b, "Scope: %s\n", opts.Scope)
	fmt.Fprintf(&b, "Backup branch: %s\n", snapshot.BackupBranch)
	fmt.Fprintf(&b, "Backup root: %s\n", snapshot.BackupRoot)
	fmt.Fprintf(&b, "Schema version: %d\n", snapshot.BackupSchemaVersion)
	fmt.Fprintf(&b, "Backup docs: %s\n", channelBackupStatusPresentLabel(snapshot.BackupDocsPresent))
	fmt.Fprintf(&b, "Catalog commands: %d\n", snapshot.CatalogEntries)
	fmt.Fprintf(&b, "Fetched-branch inspection commands: %d\n", snapshot.FetchedBranchRequiredCommands)
	b.WriteString("\nRecovery sequence:\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, step.Command, step.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Recovery map hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Recovery step hash: %s\n", snapshot.StepHash)
	b.WriteString("\nMap source: current GitHub Actions checkout backup catalog.\n")
	b.WriteString("Backup branch fetch: not performed by this action.\n")
	b.WriteString("Raw backup payloads: not read by this action.\n")
	b.WriteString("Restore: not performed by this action.\n")
	b.WriteString("Rehearsal issue creation: not performed by this action.\n")
	b.WriteString("Restore-request issue creation: not performed by this action.\n")
	b.WriteString("GitHub API replay: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelRecoveryMapSnapshot(cfg Config, opts ChannelRecoveryMapOptions) channelRecoveryMapSnapshot {
	backup := buildChannelBackupStatusSnapshot(cfg, opts.Repo)
	snapshot := channelRecoveryMapSnapshot{
		BackupBranch:                  backup.BackupBranch,
		BackupRoot:                    backup.BackupRoot,
		BackupSchemaVersion:           backup.BackupSchemaVersion,
		BackupDocsPresent:             backup.BackupDocsPresent,
		CatalogEntries:                backup.CatalogEntries,
		FetchedBranchRequiredCommands: backup.FetchedBranchRequiredCommands,
		MetadataOnlyCommands:          backup.MetadataOnlyCommands,
		RawRecoveryCommands:           backup.RawRecoveryCommands,
		ProviderVisibleActions:        backup.ProviderVisibleActions,
	}
	steps := channelRecoveryMapStepsForScope(opts.Scope, snapshot)
	snapshot.StepCount = len(steps)
	snapshot.StepHash = shortDocumentHash(channelRecoveryMapStepManifest(steps))
	snapshot.SnapshotHash = shortDocumentHash(channelRecoveryMapSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelRecoveryMapStepsForScope(scope string, snapshot channelRecoveryMapSnapshot) []channelRecoveryMapStep {
	scope = cleanChannelRecoveryMapScope(scope)
	inspectReason := "inspect one backup record without restoring files"
	if scope == "repo" {
		inspectReason = "inspect the candidate issue backup before repository-level recovery"
	}
	if scope == "incident" {
		inspectReason = "pin one candidate issue backup before incident recovery review"
	}
	return []channelRecoveryMapStep{
		{Command: "/channels backup --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("confirm backup cockpit metadata first (%d catalog commands)", snapshot.CatalogEntries)},
		{Command: "/channels backup-search <query> --message-id <id> --notify-message-id <id>", Reason: "find candidate backup metadata without reading raw payloads"},
		{Command: "/channels backup-info <issue> --message-id <id> --notify-message-id <id>", Reason: inspectReason},
		{Command: "/channels rehearse-backup --issue <issue> --id <id> --message-id <id>", Reason: "open a reviewed recovery rehearsal issue before any restore"},
		{Command: "/channels restore-request --issue <issue> --id <id> --message-id <id>", Reason: "open an explicit restore request issue when recovery should proceed"},
	}
}

func channelRecoveryMapStepManifest(steps []channelRecoveryMapStep) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, strings.Join([]string{step.Command, step.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}

func channelRecoveryMapSnapshotManifest(opts ChannelRecoveryMapOptions, snapshot channelRecoveryMapSnapshot) string {
	return fmt.Sprintf(
		"scope=%s\nbackup=%s/%s/%d/%t\ncatalog=%d/%d/%d/%d/%d\nsteps=%d/%s",
		cleanChannelRecoveryMapScope(opts.Scope),
		snapshot.BackupBranch,
		snapshot.BackupRoot,
		snapshot.BackupSchemaVersion,
		snapshot.BackupDocsPresent,
		snapshot.CatalogEntries,
		snapshot.FetchedBranchRequiredCommands,
		snapshot.MetadataOnlyCommands,
		snapshot.RawRecoveryCommands,
		snapshot.ProviderVisibleActions,
		snapshot.StepCount,
		snapshot.StepHash,
	)
}

func (r *ChannelRecoveryMapActionRequest) applySnapshot(snapshot channelRecoveryMapSnapshot) {
	r.BackupBranch = snapshot.BackupBranch
	r.BackupRoot = snapshot.BackupRoot
	r.BackupSchemaVersion = snapshot.BackupSchemaVersion
	r.BackupDocsPresent = snapshot.BackupDocsPresent
	r.CatalogEntries = snapshot.CatalogEntries
	r.FetchedBranchRequiredCommands = snapshot.FetchedBranchRequiredCommands
	r.MetadataOnlyCommands = snapshot.MetadataOnlyCommands
	r.RawRecoveryCommands = snapshot.RawRecoveryCommands
	r.ProviderVisibleActions = snapshot.ProviderVisibleActions
	r.StepCount = snapshot.StepCount
	r.StepSHA = snapshot.StepHash
	r.SnapshotSHA = snapshot.SnapshotHash
}

func (r *ChannelRecoveryMapResult) applySnapshot(snapshot channelRecoveryMapSnapshot) {
	r.BackupBranch = snapshot.BackupBranch
	r.BackupRoot = snapshot.BackupRoot
	r.BackupSchemaVersion = snapshot.BackupSchemaVersion
	r.BackupDocsPresent = snapshot.BackupDocsPresent
	r.CatalogEntries = snapshot.CatalogEntries
	r.FetchedBranchRequiredCommands = snapshot.FetchedBranchRequiredCommands
	r.MetadataOnlyCommands = snapshot.MetadataOnlyCommands
	r.RawRecoveryCommands = snapshot.RawRecoveryCommands
	r.ProviderVisibleActions = snapshot.ProviderVisibleActions
	r.StepCount = snapshot.StepCount
}
