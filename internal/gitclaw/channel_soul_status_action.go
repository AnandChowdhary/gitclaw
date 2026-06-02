package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSoulStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelSoulStatusResult struct {
	Notification              ChannelSendResult
	RouteName                 string
	RouteHash                 string
	Channel                   string
	ThreadHash                string
	MessageHash               string
	NotifyHash                string
	StatusIDHash              string
	BodyHash                  string
	SoulStatus                string
	SnapshotVersion           string
	SnapshotScope             string
	SnapshotSHA               string
	SnapshotEntries           int
	LoadedSnapshotEntries     int
	RequiredSnapshotEntries   int
	RequiredLoadedEntries     int
	MissingRequiredEntries    int
	OptionalLoadedEntries     int
	MemoryNoteEntries         int
	PromptVisibleEntries      int
	ValidationStatus          string
	ValidationErrors          int
	ValidationWarnings        int
	RequiredFiles             int
	RequiredFilesPresent      int
	RequiredFilesMissing      int
	NoncanonicalMemoryNotes   int
	RiskStatus                string
	ContextDocuments          int
	ScannedDocuments          int
	DocumentsWithRiskFindings int
	RiskFindings              int
	HighRiskFindings          int
	WarningRiskFindings       int
	InfoRiskFindings          int
	RegistryContactAllowed    bool
	ProfileExportAllowed      bool
	SoulWritesAllowed         bool
	RepositoryMutationAllowed bool
	RawBodiesIncluded         bool
	RawDescriptionsIncluded   bool
	RegistryVerification      string
	ProfileExportVerification string
}

type ChannelSoulStatusActionRequest struct {
	Options                   ChannelSoulStatusOptions
	Command                   string
	Subcommand                string
	AutoSourceMessageID       bool
	AutoNotifyMessageID       bool
	AutoStatusID              bool
	TargetFromIssue           bool
	RequestedRouteHash        string
	RequestedThreadHash       string
	RequestedMsgHash          string
	NotifyMessageHash         string
	StatusIDHash              string
	NotificationBodySHA       string
	SoulStatus                string
	SnapshotVersion           string
	SnapshotScope             string
	SnapshotSHA               string
	SnapshotEntries           int
	LoadedSnapshotEntries     int
	RequiredSnapshotEntries   int
	RequiredLoadedEntries     int
	MissingRequiredEntries    int
	OptionalLoadedEntries     int
	MemoryNoteEntries         int
	PromptVisibleEntries      int
	ValidationStatus          string
	ValidationErrors          int
	ValidationWarnings        int
	RequiredFiles             int
	RequiredFilesPresent      int
	RequiredFilesMissing      int
	NoncanonicalMemoryNotes   int
	RiskStatus                string
	ContextDocuments          int
	ScannedDocuments          int
	DocumentsWithRiskFindings int
	RiskFindings              int
	HighRiskFindings          int
	WarningRiskFindings       int
	InfoRiskFindings          int
	RegistryContactAllowed    bool
	ProfileExportAllowed      bool
	SoulWritesAllowed         bool
	RepositoryMutationAllowed bool
	RawBodiesIncluded         bool
	RawDescriptionsIncluded   bool
	RegistryVerification      string
	ProfileExportVerification string
}

type channelSoulStatusSnapshot struct {
	SoulStatus                string
	SnapshotVersion           string
	SnapshotScope             string
	SnapshotSHA               string
	SnapshotEntries           int
	LoadedSnapshotEntries     int
	RequiredSnapshotEntries   int
	RequiredLoadedEntries     int
	MissingRequiredEntries    int
	OptionalLoadedEntries     int
	MemoryNoteEntries         int
	PromptVisibleEntries      int
	ValidationStatus          string
	ValidationErrors          int
	ValidationWarnings        int
	RequiredFiles             int
	RequiredFilesPresent      int
	RequiredFilesMissing      int
	NoncanonicalMemoryNotes   int
	RiskStatus                string
	ContextDocuments          int
	ScannedDocuments          int
	DocumentsWithRiskFindings int
	RiskFindings              int
	HighRiskFindings          int
	WarningRiskFindings       int
	InfoRiskFindings          int
	RegistryContactAllowed    bool
	ProfileExportAllowed      bool
	SoulWritesAllowed         bool
	RepositoryMutationAllowed bool
	RawBodiesIncluded         bool
	RawDescriptionsIncluded   bool
	RegistryVerification      string
	ProfileExportVerification string
}

func IsChannelSoulStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelSoulStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelSoulStatusActionFields(fields)
}

func isChannelSoulStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "soul-status", "soul-snapshot", "soul-health", "agent-soul", "identity-soul", "context-soul", "authority-status":
		return true
	default:
		return false
	}
}

func BuildChannelSoulStatusActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulStatusActionRequest, error) {
	fields, _, ok := channelSoulStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSoulStatusActionRequest{}, fmt.Errorf("missing channel soul status command")
	}
	req := ChannelSoulStatusActionRequest{
		Options:    ChannelSoulStatusOptions{Repo: ev.Repo},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--soul-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelSoulStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulStatusActionRequest{}, fmt.Errorf("unknown channel soul status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelSoulStatusActionRequest{}, fmt.Errorf("unexpected channel soul status argument %q", field)
		}
	}
	if err := applyChannelSoulStatusIssueTarget(ev, &req); err != nil {
		return ChannelSoulStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSoulStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelSoulStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulStatusOptions(req.Options)
	if err := validateChannelSoulStatusActionRequestOptions(req.Options); err != nil {
		return ChannelSoulStatusActionRequest{}, err
	}
	snapshot := buildChannelSoulStatusSnapshot(repoContext)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelSoulStatusNotificationBody(snapshot))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelSoulStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSoulStatusOptions, repoContext RepoContext) (ChannelSoulStatusResult, error) {
	opts = normalizeChannelSoulStatusOptions(opts)
	var err error
	opts, err = applyChannelSoulStatusRoute(cfg, opts)
	if err != nil {
		return ChannelSoulStatusResult{}, err
	}
	if err := validateChannelSoulStatusOptions(opts); err != nil {
		return ChannelSoulStatusResult{}, err
	}
	snapshot := buildChannelSoulStatusSnapshot(repoContext)
	body := renderChannelSoulStatusNotificationBody(snapshot)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSoulStatusResult{}, fmt.Errorf("queue channel soul status notification: %w", err)
	}
	result := ChannelSoulStatusResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash: shortDocumentHash(opts.StatusID),
		BodyHash:     shortDocumentHash(body),
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelSoulStatusActionReport(ev Event, req ChannelSoulStatusActionRequest, result ChannelSoulStatusResult) string {
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
	b.WriteString("## GitClaw Channel Soul Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_snapshot_mode: `%s`\n", "provider-facing-soul-status")
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
	fmt.Fprintf(&b, "- soul_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- soul_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- soul_status: `%s`\n", firstNonEmpty(result.SoulStatus, req.SoulStatus, "unknown"))
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", firstNonEmpty(result.SnapshotVersion, req.SnapshotVersion, soulSnapshotVersion))
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", firstNonEmpty(result.SnapshotScope, req.SnapshotScope, "repo-local-high-authority-context"))
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SnapshotSHA, req.SnapshotSHA)))
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", nonzeroOrReq(result.SnapshotEntries, req.SnapshotEntries))
	fmt.Fprintf(&b, "- loaded_snapshot_entries: `%d`\n", nonzeroOrReq(result.LoadedSnapshotEntries, req.LoadedSnapshotEntries))
	fmt.Fprintf(&b, "- required_snapshot_entries: `%d`\n", nonzeroOrReq(result.RequiredSnapshotEntries, req.RequiredSnapshotEntries))
	fmt.Fprintf(&b, "- required_loaded_entries: `%d`\n", nonzeroOrReq(result.RequiredLoadedEntries, req.RequiredLoadedEntries))
	fmt.Fprintf(&b, "- missing_required_entries: `%d`\n", result.MissingRequiredEntries)
	fmt.Fprintf(&b, "- optional_loaded_entries: `%d`\n", result.OptionalLoadedEntries)
	fmt.Fprintf(&b, "- memory_note_entries: `%d`\n", result.MemoryNoteEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", result.PromptVisibleEntries)
	fmt.Fprintf(&b, "- soul_validation_status: `%s`\n", firstNonEmpty(result.ValidationStatus, req.ValidationStatus, "unknown"))
	fmt.Fprintf(&b, "- soul_validation_errors: `%d`\n", result.ValidationErrors)
	fmt.Fprintf(&b, "- soul_validation_warnings: `%d`\n", result.ValidationWarnings)
	fmt.Fprintf(&b, "- soul_required_files: `%d`\n", result.RequiredFiles)
	fmt.Fprintf(&b, "- soul_required_files_present: `%d`\n", result.RequiredFilesPresent)
	fmt.Fprintf(&b, "- soul_required_files_missing: `%d`\n", result.RequiredFilesMissing)
	fmt.Fprintf(&b, "- soul_noncanonical_memory_notes: `%d`\n", result.NoncanonicalMemoryNotes)
	fmt.Fprintf(&b, "- soul_risk_status: `%s`\n", firstNonEmpty(result.RiskStatus, req.RiskStatus, "unknown"))
	fmt.Fprintf(&b, "- context_documents: `%d`\n", result.ContextDocuments)
	fmt.Fprintf(&b, "- scanned_documents: `%d`\n", result.ScannedDocuments)
	fmt.Fprintf(&b, "- documents_with_risk_findings: `%d`\n", result.DocumentsWithRiskFindings)
	fmt.Fprintf(&b, "- soul_risk_findings: `%d`\n", result.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", result.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", result.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", result.InfoRiskFindings)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", result.RegistryContactAllowed || req.RegistryContactAllowed)
	fmt.Fprintf(&b, "- profile_export_allowed: `%t`\n", result.ProfileExportAllowed || req.ProfileExportAllowed)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", result.SoulWritesAllowed || req.SoulWritesAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", result.RepositoryMutationAllowed || req.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", result.RawBodiesIncluded || req.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_descriptions_included: `%t`\n", result.RawDescriptionsIncluded || req.RawDescriptionsIncluded)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", firstNonEmpty(result.RegistryVerification, req.RegistryVerification, "not_configured"))
	fmt.Fprintf(&b, "- profile_export_verification: `%s`\n", firstNonEmpty(result.ProfileExportVerification, req.ProfileExportVerification, "not_configured"))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- registry_contact_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_file_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_identity_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_user_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_guidance_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_heartbeat_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_session_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing soul status snapshot on the canonical channel issue. This is the GitHub-native channel version of an agent soul health card: it reports the repo-local high-authority context, validation state, risk state, and composite snapshot as counts and hashes, but it does not contact registries, export profiles, write soul files, call a model, mutate repository files, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, soul file paths, raw soul bodies, identity bodies, user bodies, memory bodies, tool guidance, heartbeat bodies, prompts, sessions, backup payloads, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the soul-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent soul-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate soul-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/soul snapshot`, `/soul anchors`, `/soul validate`, or `/soul risk` on GitHub for deeper body-free reports\n")
	return strings.TrimSpace(b.String())
}

func channelSoulStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSoulStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSoulStatusIssueTarget(ev Event, req *ChannelSoulStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSoulStatusOptions(opts ChannelSoulStatusOptions) ChannelSoulStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelSoulStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSoulStatusRoute(cfg Config, opts ChannelSoulStatusOptions) (ChannelSoulStatusOptions, error) {
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
		Body:      "GitClaw channel soul status.",
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

func validateChannelSoulStatusOptions(opts ChannelSoulStatusOptions) error {
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
		return fmt.Errorf("missing soul status id")
	}
	return nil
}

func validateChannelSoulStatusActionRequestOptions(opts ChannelSoulStatusOptions) error {
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
		return fmt.Errorf("missing soul status id")
	}
	return nil
}

func cleanChannelSoulStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelSoulStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-soul-source-%s", eventID(ev))
}

func autoChannelSoulStatusID(ev Event, opts ChannelSoulStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("soul-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSoulStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelSoulStatusNotificationBody(snapshot channelSoulStatusSnapshot) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul status.\n\n")
	fmt.Fprintf(&b, "Soul status: %s\n", snapshot.SoulStatus)
	fmt.Fprintf(&b, "Snapshot version: %s\n", snapshot.SnapshotVersion)
	fmt.Fprintf(&b, "Snapshot scope: %s\n", snapshot.SnapshotScope)
	fmt.Fprintf(&b, "Snapshot hash: %s\n", snapshot.SnapshotSHA)
	fmt.Fprintf(&b, "Snapshot entries: %d\n", snapshot.SnapshotEntries)
	fmt.Fprintf(&b, "Loaded entries: %d\n", snapshot.LoadedSnapshotEntries)
	fmt.Fprintf(&b, "Required entries: %d/%d loaded\n", snapshot.RequiredLoadedEntries, snapshot.RequiredSnapshotEntries)
	fmt.Fprintf(&b, "Missing required entries: %d\n", snapshot.MissingRequiredEntries)
	fmt.Fprintf(&b, "Optional loaded entries: %d\n", snapshot.OptionalLoadedEntries)
	fmt.Fprintf(&b, "Memory note entries: %d\n", snapshot.MemoryNoteEntries)
	fmt.Fprintf(&b, "Prompt-visible entries: %d\n", snapshot.PromptVisibleEntries)
	fmt.Fprintf(&b, "Validation: %s (%d errors, %d warnings)\n", snapshot.ValidationStatus, snapshot.ValidationErrors, snapshot.ValidationWarnings)
	fmt.Fprintf(&b, "Required files: %d/%d present\n", snapshot.RequiredFilesPresent, snapshot.RequiredFiles)
	fmt.Fprintf(&b, "Risk: %s (%d findings, high=%d warning=%d info=%d)\n", snapshot.RiskStatus, snapshot.RiskFindings, snapshot.HighRiskFindings, snapshot.WarningRiskFindings, snapshot.InfoRiskFindings)
	b.WriteString("\nRegistry contact: not performed by this action.\n")
	b.WriteString("Profile export: disabled.\n")
	b.WriteString("Soul writes: disabled.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Raw soul bodies: not included.\n")
	b.WriteString("Raw identity bodies: not included.\n")
	b.WriteString("Raw user bodies: not included.\n")
	b.WriteString("Raw memory bodies: not included.\n")
	b.WriteString("Raw tool guidance bodies: not included.\n")
	b.WriteString("Raw heartbeat bodies: not included.\n")
	b.WriteString("Prompts, sessions, and backup payloads: not included.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelSoulStatusSnapshot(repoContext RepoContext) channelSoulStatusSnapshot {
	report := BuildSoulSnapshotReport(repoContext)
	validation := report.AnchorReport.Validation
	risk := report.AnchorReport.Risk
	return channelSoulStatusSnapshot{
		SoulStatus:                report.Status,
		SnapshotVersion:           report.SnapshotVersion,
		SnapshotScope:             report.SnapshotScope,
		SnapshotSHA:               report.SnapshotSHA,
		SnapshotEntries:           report.SnapshotEntries,
		LoadedSnapshotEntries:     report.LoadedSnapshotEntries,
		RequiredSnapshotEntries:   report.RequiredSnapshotEntries,
		RequiredLoadedEntries:     report.RequiredLoadedEntries,
		MissingRequiredEntries:    report.MissingRequiredEntries,
		OptionalLoadedEntries:     report.OptionalLoadedEntries,
		MemoryNoteEntries:         report.MemoryNoteEntries,
		PromptVisibleEntries:      report.PromptVisibleEntries,
		ValidationStatus:          validation.Status,
		ValidationErrors:          validation.Errors,
		ValidationWarnings:        validation.Warnings,
		RequiredFiles:             validation.RequiredFiles,
		RequiredFilesPresent:      validation.PresentRequiredFiles,
		RequiredFilesMissing:      validation.MissingRequiredFiles,
		NoncanonicalMemoryNotes:   validation.NoncanonicalMemoryNotes,
		RiskStatus:                risk.Status,
		ContextDocuments:          risk.Documents,
		ScannedDocuments:          risk.ScannedDocuments,
		DocumentsWithRiskFindings: risk.DocumentsWithRiskFindings,
		RiskFindings:              len(risk.Findings),
		HighRiskFindings:          risk.HighRiskFindings,
		WarningRiskFindings:       risk.WarningRiskFindings,
		InfoRiskFindings:          risk.InfoRiskFindings,
		RegistryContactAllowed:    report.RegistryContactAllowed,
		ProfileExportAllowed:      report.ProfileExportAllowed,
		SoulWritesAllowed:         report.SoulWritesAllowed,
		RepositoryMutationAllowed: report.RepositoryMutationAllowed,
		RawBodiesIncluded:         report.RawBodiesIncluded,
		RawDescriptionsIncluded:   report.RawDescriptionsIncluded,
		RegistryVerification:      risk.RegistryVerification,
		ProfileExportVerification: risk.ProfileExportVerification,
	}
}

func (r *ChannelSoulStatusActionRequest) applySnapshot(snapshot channelSoulStatusSnapshot) {
	r.SoulStatus = snapshot.SoulStatus
	r.SnapshotVersion = snapshot.SnapshotVersion
	r.SnapshotScope = snapshot.SnapshotScope
	r.SnapshotSHA = snapshot.SnapshotSHA
	r.SnapshotEntries = snapshot.SnapshotEntries
	r.LoadedSnapshotEntries = snapshot.LoadedSnapshotEntries
	r.RequiredSnapshotEntries = snapshot.RequiredSnapshotEntries
	r.RequiredLoadedEntries = snapshot.RequiredLoadedEntries
	r.MissingRequiredEntries = snapshot.MissingRequiredEntries
	r.OptionalLoadedEntries = snapshot.OptionalLoadedEntries
	r.MemoryNoteEntries = snapshot.MemoryNoteEntries
	r.PromptVisibleEntries = snapshot.PromptVisibleEntries
	r.ValidationStatus = snapshot.ValidationStatus
	r.ValidationErrors = snapshot.ValidationErrors
	r.ValidationWarnings = snapshot.ValidationWarnings
	r.RequiredFiles = snapshot.RequiredFiles
	r.RequiredFilesPresent = snapshot.RequiredFilesPresent
	r.RequiredFilesMissing = snapshot.RequiredFilesMissing
	r.NoncanonicalMemoryNotes = snapshot.NoncanonicalMemoryNotes
	r.RiskStatus = snapshot.RiskStatus
	r.ContextDocuments = snapshot.ContextDocuments
	r.ScannedDocuments = snapshot.ScannedDocuments
	r.DocumentsWithRiskFindings = snapshot.DocumentsWithRiskFindings
	r.RiskFindings = snapshot.RiskFindings
	r.HighRiskFindings = snapshot.HighRiskFindings
	r.WarningRiskFindings = snapshot.WarningRiskFindings
	r.InfoRiskFindings = snapshot.InfoRiskFindings
	r.RegistryContactAllowed = snapshot.RegistryContactAllowed
	r.ProfileExportAllowed = snapshot.ProfileExportAllowed
	r.SoulWritesAllowed = snapshot.SoulWritesAllowed
	r.RepositoryMutationAllowed = snapshot.RepositoryMutationAllowed
	r.RawBodiesIncluded = snapshot.RawBodiesIncluded
	r.RawDescriptionsIncluded = snapshot.RawDescriptionsIncluded
	r.RegistryVerification = snapshot.RegistryVerification
	r.ProfileExportVerification = snapshot.ProfileExportVerification
}

func (r *ChannelSoulStatusResult) applySnapshot(snapshot channelSoulStatusSnapshot) {
	r.SoulStatus = snapshot.SoulStatus
	r.SnapshotVersion = snapshot.SnapshotVersion
	r.SnapshotScope = snapshot.SnapshotScope
	r.SnapshotSHA = snapshot.SnapshotSHA
	r.SnapshotEntries = snapshot.SnapshotEntries
	r.LoadedSnapshotEntries = snapshot.LoadedSnapshotEntries
	r.RequiredSnapshotEntries = snapshot.RequiredSnapshotEntries
	r.RequiredLoadedEntries = snapshot.RequiredLoadedEntries
	r.MissingRequiredEntries = snapshot.MissingRequiredEntries
	r.OptionalLoadedEntries = snapshot.OptionalLoadedEntries
	r.MemoryNoteEntries = snapshot.MemoryNoteEntries
	r.PromptVisibleEntries = snapshot.PromptVisibleEntries
	r.ValidationStatus = snapshot.ValidationStatus
	r.ValidationErrors = snapshot.ValidationErrors
	r.ValidationWarnings = snapshot.ValidationWarnings
	r.RequiredFiles = snapshot.RequiredFiles
	r.RequiredFilesPresent = snapshot.RequiredFilesPresent
	r.RequiredFilesMissing = snapshot.RequiredFilesMissing
	r.NoncanonicalMemoryNotes = snapshot.NoncanonicalMemoryNotes
	r.RiskStatus = snapshot.RiskStatus
	r.ContextDocuments = snapshot.ContextDocuments
	r.ScannedDocuments = snapshot.ScannedDocuments
	r.DocumentsWithRiskFindings = snapshot.DocumentsWithRiskFindings
	r.RiskFindings = snapshot.RiskFindings
	r.HighRiskFindings = snapshot.HighRiskFindings
	r.WarningRiskFindings = snapshot.WarningRiskFindings
	r.InfoRiskFindings = snapshot.InfoRiskFindings
	r.RegistryContactAllowed = snapshot.RegistryContactAllowed
	r.ProfileExportAllowed = snapshot.ProfileExportAllowed
	r.SoulWritesAllowed = snapshot.SoulWritesAllowed
	r.RepositoryMutationAllowed = snapshot.RepositoryMutationAllowed
	r.RawBodiesIncluded = snapshot.RawBodiesIncluded
	r.RawDescriptionsIncluded = snapshot.RawDescriptionsIncluded
	r.RegistryVerification = snapshot.RegistryVerification
	r.ProfileExportVerification = snapshot.ProfileExportVerification
}
