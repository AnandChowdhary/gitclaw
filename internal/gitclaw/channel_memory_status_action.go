package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelMemoryStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelMemoryStatusResult struct {
	Notification                 ChannelSendResult
	RouteName                    string
	RouteHash                    string
	Channel                      string
	ThreadHash                   string
	MessageHash                  string
	NotifyHash                   string
	StatusIDHash                 string
	BodyHash                     string
	MemoryStatus                 string
	SnapshotVersion              string
	SnapshotScope                string
	SnapshotSHA                  string
	SnapshotEntries              int
	LongTermEntries              int
	DatedNoteEntries             int
	MemoryNoteEntries            int
	PromptVisibleEntries         int
	LoadedMemoryEntries          int
	OmittedMemoryEntries         int
	MemoryFiles                  int
	LongTermMemoryPresent        bool
	LongTermMemoryLoaded         bool
	DatedMemoryNotes             int
	CanonicalDatedMemoryNotes    int
	NoncanonicalDatedMemoryNotes int
	LoadedMemoryNotes            int
	MaxLoadedMemoryNotes         int
	FirstMemoryNoteHash          string
	LatestMemoryNoteHash         string
	TimelineSpanDays             int
	LargestGapDays               int
	GapsOverOneDay               int
	ValidationStatus             string
	ValidationErrors             int
	ValidationWarnings           int
	EmptyMemoryFiles             int
	MemoryFilesAtLimit           int
	PotentialSecretFindings      int
	RiskStatus                   string
	ScannedMemoryFiles           int
	MemoryFilesWithRiskFindings  int
	RiskFindings                 int
	HighRiskFindings             int
	WarningRiskFindings          int
	InfoRiskFindings             int
	MemoryWritesAllowed          bool
	ExternalProviderAccessed     bool
	ExternalProviderVerification string
	BackgroundPromotionActive    bool
	BackgroundPromotionReview    string
	RawMemoryBodiesIncluded      bool
	RawIssueBodiesIncluded       bool
	RawCommentBodiesIncluded     bool
	RawPromptBodiesIncluded      bool
	RawSessionBodiesIncluded     bool
	EmbeddingVectorsIncluded     bool
}

type ChannelMemoryStatusActionRequest struct {
	Options                      ChannelMemoryStatusOptions
	Command                      string
	Subcommand                   string
	AutoSourceMessageID          bool
	AutoNotifyMessageID          bool
	AutoStatusID                 bool
	TargetFromIssue              bool
	RequestedRouteHash           string
	RequestedThreadHash          string
	RequestedMsgHash             string
	NotifyMessageHash            string
	StatusIDHash                 string
	NotificationBodySHA          string
	MemoryStatus                 string
	SnapshotVersion              string
	SnapshotScope                string
	SnapshotSHA                  string
	SnapshotEntries              int
	LongTermEntries              int
	DatedNoteEntries             int
	MemoryNoteEntries            int
	PromptVisibleEntries         int
	LoadedMemoryEntries          int
	OmittedMemoryEntries         int
	MemoryFiles                  int
	LongTermMemoryPresent        bool
	LongTermMemoryLoaded         bool
	DatedMemoryNotes             int
	CanonicalDatedMemoryNotes    int
	NoncanonicalDatedMemoryNotes int
	LoadedMemoryNotes            int
	MaxLoadedMemoryNotes         int
	FirstMemoryNoteHash          string
	LatestMemoryNoteHash         string
	TimelineSpanDays             int
	LargestGapDays               int
	GapsOverOneDay               int
	ValidationStatus             string
	ValidationErrors             int
	ValidationWarnings           int
	EmptyMemoryFiles             int
	MemoryFilesAtLimit           int
	PotentialSecretFindings      int
	RiskStatus                   string
	ScannedMemoryFiles           int
	MemoryFilesWithRiskFindings  int
	RiskFindings                 int
	HighRiskFindings             int
	WarningRiskFindings          int
	InfoRiskFindings             int
	MemoryWritesAllowed          bool
	ExternalProviderAccessed     bool
	ExternalProviderVerification string
	BackgroundPromotionActive    bool
	BackgroundPromotionReview    string
	RawMemoryBodiesIncluded      bool
	RawIssueBodiesIncluded       bool
	RawCommentBodiesIncluded     bool
	RawPromptBodiesIncluded      bool
	RawSessionBodiesIncluded     bool
	EmbeddingVectorsIncluded     bool
}

type channelMemoryStatusSnapshot struct {
	MemoryStatus                 string
	SnapshotVersion              string
	SnapshotScope                string
	SnapshotSHA                  string
	SnapshotEntries              int
	LongTermEntries              int
	DatedNoteEntries             int
	MemoryNoteEntries            int
	PromptVisibleEntries         int
	LoadedMemoryEntries          int
	OmittedMemoryEntries         int
	MemoryFiles                  int
	LongTermMemoryPresent        bool
	LongTermMemoryLoaded         bool
	DatedMemoryNotes             int
	CanonicalDatedMemoryNotes    int
	NoncanonicalDatedMemoryNotes int
	LoadedMemoryNotes            int
	MaxLoadedMemoryNotes         int
	FirstMemoryNoteHash          string
	LatestMemoryNoteHash         string
	TimelineSpanDays             int
	LargestGapDays               int
	GapsOverOneDay               int
	ValidationStatus             string
	ValidationErrors             int
	ValidationWarnings           int
	EmptyMemoryFiles             int
	MemoryFilesAtLimit           int
	PotentialSecretFindings      int
	RiskStatus                   string
	ScannedMemoryFiles           int
	MemoryFilesWithRiskFindings  int
	RiskFindings                 int
	HighRiskFindings             int
	WarningRiskFindings          int
	InfoRiskFindings             int
	MemoryWritesAllowed          bool
	ExternalProviderAccessed     bool
	ExternalProviderVerification string
	BackgroundPromotionActive    bool
	BackgroundPromotionReview    string
	RawMemoryBodiesIncluded      bool
	RawIssueBodiesIncluded       bool
	RawCommentBodiesIncluded     bool
	RawPromptBodiesIncluded      bool
	RawSessionBodiesIncluded     bool
	EmbeddingVectorsIncluded     bool
}

func IsChannelMemoryStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelMemoryStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelMemoryStatusActionFields(fields)
}

func isChannelMemoryStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "memory-status", "memory-snapshot", "memory-health", "memory-state", "durable-memory", "recall-status", "context-memory":
		return true
	default:
		return false
	}
}

func BuildChannelMemoryStatusActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelMemoryStatusActionRequest, error) {
	fields, _, ok := channelMemoryStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelMemoryStatusActionRequest{}, fmt.Errorf("missing channel memory status command")
	}
	req := ChannelMemoryStatusActionRequest{
		Options:    ChannelMemoryStatusOptions{Repo: ev.Repo},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--memory-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelMemoryStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMemoryStatusActionRequest{}, fmt.Errorf("unknown channel memory status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelMemoryStatusActionRequest{}, fmt.Errorf("unexpected channel memory status argument %q", field)
		}
	}
	if err := applyChannelMemoryStatusIssueTarget(ev, &req); err != nil {
		return ChannelMemoryStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelMemoryStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelMemoryStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMemoryStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMemoryStatusOptions(req.Options)
	if err := validateChannelMemoryStatusActionRequestOptions(req.Options); err != nil {
		return ChannelMemoryStatusActionRequest{}, err
	}
	snapshot := buildChannelMemoryStatusSnapshot(cfg, repoContext)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelMemoryStatusNotificationBody(snapshot))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelMemoryStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMemoryStatusOptions, repoContext RepoContext) (ChannelMemoryStatusResult, error) {
	opts = normalizeChannelMemoryStatusOptions(opts)
	var err error
	opts, err = applyChannelMemoryStatusRoute(cfg, opts)
	if err != nil {
		return ChannelMemoryStatusResult{}, err
	}
	if err := validateChannelMemoryStatusOptions(opts); err != nil {
		return ChannelMemoryStatusResult{}, err
	}
	snapshot := buildChannelMemoryStatusSnapshot(cfg, repoContext)
	body := renderChannelMemoryStatusNotificationBody(snapshot)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelMemoryStatusResult{}, fmt.Errorf("queue channel memory status notification: %w", err)
	}
	result := ChannelMemoryStatusResult{
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

func RenderChannelMemoryStatusActionReport(ev Event, req ChannelMemoryStatusActionRequest, result ChannelMemoryStatusResult) string {
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
	b.WriteString("## GitClaw Channel Memory Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_memory_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- memory_snapshot_mode: `%s`\n", "provider-facing-memory-status")
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
	fmt.Fprintf(&b, "- memory_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- memory_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- memory_status: `%s`\n", firstNonEmpty(result.MemoryStatus, req.MemoryStatus, "unknown"))
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", firstNonEmpty(result.SnapshotVersion, req.SnapshotVersion, memorySnapshotVersion))
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", firstNonEmpty(result.SnapshotScope, req.SnapshotScope, "repo-local-durable-memory"))
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SnapshotSHA, req.SnapshotSHA)))
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", nonzeroOrReq(result.SnapshotEntries, req.SnapshotEntries))
	fmt.Fprintf(&b, "- long_term_entries: `%d`\n", result.LongTermEntries)
	fmt.Fprintf(&b, "- dated_note_entries: `%d`\n", result.DatedNoteEntries)
	fmt.Fprintf(&b, "- memory_note_entries: `%d`\n", result.MemoryNoteEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", result.PromptVisibleEntries)
	fmt.Fprintf(&b, "- loaded_memory_entries: `%d`\n", result.LoadedMemoryEntries)
	fmt.Fprintf(&b, "- omitted_memory_entries: `%d`\n", result.OmittedMemoryEntries)
	fmt.Fprintf(&b, "- memory_files: `%d`\n", result.MemoryFiles)
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", result.LongTermMemoryPresent || req.LongTermMemoryPresent)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", result.LongTermMemoryLoaded || req.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", result.DatedMemoryNotes)
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", result.CanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", result.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", result.LoadedMemoryNotes)
	fmt.Fprintf(&b, "- max_loaded_memory_notes: `%d`\n", result.MaxLoadedMemoryNotes)
	fmt.Fprintf(&b, "- first_memory_note_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.FirstMemoryNoteHash, req.FirstMemoryNoteHash)))
	fmt.Fprintf(&b, "- latest_memory_note_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.LatestMemoryNoteHash, req.LatestMemoryNoteHash)))
	fmt.Fprintf(&b, "- timeline_span_days: `%d`\n", result.TimelineSpanDays)
	fmt.Fprintf(&b, "- largest_gap_days: `%d`\n", result.LargestGapDays)
	fmt.Fprintf(&b, "- gaps_over_one_day: `%d`\n", result.GapsOverOneDay)
	fmt.Fprintf(&b, "- memory_validation_status: `%s`\n", firstNonEmpty(result.ValidationStatus, req.ValidationStatus, "unknown"))
	fmt.Fprintf(&b, "- memory_validation_errors: `%d`\n", result.ValidationErrors)
	fmt.Fprintf(&b, "- memory_validation_warnings: `%d`\n", result.ValidationWarnings)
	fmt.Fprintf(&b, "- empty_memory_files: `%d`\n", result.EmptyMemoryFiles)
	fmt.Fprintf(&b, "- memory_files_at_limit: `%d`\n", result.MemoryFilesAtLimit)
	fmt.Fprintf(&b, "- potential_secret_findings: `%d`\n", result.PotentialSecretFindings)
	fmt.Fprintf(&b, "- memory_risk_status: `%s`\n", firstNonEmpty(result.RiskStatus, req.RiskStatus, "unknown"))
	fmt.Fprintf(&b, "- scanned_memory_files: `%d`\n", result.ScannedMemoryFiles)
	fmt.Fprintf(&b, "- memory_files_with_risk_findings: `%d`\n", result.MemoryFilesWithRiskFindings)
	fmt.Fprintf(&b, "- memory_risk_findings: `%d`\n", result.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", result.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", result.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", result.InfoRiskFindings)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", result.MemoryWritesAllowed || req.MemoryWritesAllowed)
	fmt.Fprintf(&b, "- external_provider_accessed: `%t`\n", result.ExternalProviderAccessed || req.ExternalProviderAccessed)
	fmt.Fprintf(&b, "- external_provider_verification: `%s`\n", firstNonEmpty(result.ExternalProviderVerification, req.ExternalProviderVerification, "not_configured"))
	fmt.Fprintf(&b, "- background_promotion_active: `%t`\n", result.BackgroundPromotionActive || req.BackgroundPromotionActive)
	fmt.Fprintf(&b, "- background_promotion_review: `%s`\n", firstNonEmpty(result.BackgroundPromotionReview, req.BackgroundPromotionReview, "git_review_required"))
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", result.RawMemoryBodiesIncluded || req.RawMemoryBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", result.RawIssueBodiesIncluded || req.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", result.RawCommentBodiesIncluded || req.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", result.RawPromptBodiesIncluded || req.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_session_bodies_included: `%t`\n", result.RawSessionBodiesIncluded || req.RawSessionBodiesIncluded)
	fmt.Fprintf(&b, "- embedding_vectors_included: `%t`\n", result.EmbeddingVectorsIncluded || req.EmbeddingVectorsIncluded)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- background_promotion_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_provider_access_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- embedding_vector_access_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_file_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_timeline_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_session_bodies_included_in_notification: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_memory_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing memory status snapshot on the canonical channel issue. This is the GitHub-native channel version of a durable-memory health card: it reports the repo-local memory snapshot, validation state, risk state, chronology counts, and disabled write/provider gates as counts and hashes, but it does not write memory, promote memory in the background, access external providers, call a model, mutate repository files, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, memory file paths, raw memory bodies, issue bodies, comments, prompts, sessions, backup payloads, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the memory-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent memory-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate memory-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/memory snapshot`, `/memory timeline`, `/memory validate`, or `/memory risk` on GitHub for deeper body-free reports\n")
	return strings.TrimSpace(b.String())
}

func channelMemoryStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelMemoryStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelMemoryStatusIssueTarget(ev Event, req *ChannelMemoryStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel memory status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelMemoryStatusOptions(opts ChannelMemoryStatusOptions) ChannelMemoryStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelMemoryStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelMemoryStatusRoute(cfg Config, opts ChannelMemoryStatusOptions) (ChannelMemoryStatusOptions, error) {
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
		Body:      "GitClaw channel memory status.",
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

func validateChannelMemoryStatusOptions(opts ChannelMemoryStatusOptions) error {
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
		return fmt.Errorf("missing memory status id")
	}
	return nil
}

func validateChannelMemoryStatusActionRequestOptions(opts ChannelMemoryStatusOptions) error {
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
		return fmt.Errorf("missing memory status id")
	}
	return nil
}

func cleanChannelMemoryStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelMemoryStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-memory-source-%s", eventID(ev))
}

func autoChannelMemoryStatusID(ev Event, opts ChannelMemoryStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("memory-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMemoryStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-memory-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelMemoryStatusNotificationBody(snapshot channelMemoryStatusSnapshot) string {
	var b strings.Builder
	b.WriteString("GitClaw channel memory status.\n\n")
	fmt.Fprintf(&b, "Memory status: %s\n", snapshot.MemoryStatus)
	fmt.Fprintf(&b, "Snapshot version: %s\n", snapshot.SnapshotVersion)
	fmt.Fprintf(&b, "Snapshot scope: %s\n", snapshot.SnapshotScope)
	fmt.Fprintf(&b, "Snapshot hash: %s\n", snapshot.SnapshotSHA)
	fmt.Fprintf(&b, "Snapshot entries: %d\n", snapshot.SnapshotEntries)
	fmt.Fprintf(&b, "Memory files: %d\n", snapshot.MemoryFiles)
	fmt.Fprintf(&b, "Long-term memory: present=%t loaded=%t\n", snapshot.LongTermMemoryPresent, snapshot.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "Dated notes: %d canonical=%d noncanonical=%d loaded=%d max_loaded=%d\n", snapshot.DatedMemoryNotes, snapshot.CanonicalDatedMemoryNotes, snapshot.NoncanonicalDatedMemoryNotes, snapshot.LoadedMemoryNotes, snapshot.MaxLoadedMemoryNotes)
	fmt.Fprintf(&b, "Prompt-visible entries: %d\n", snapshot.PromptVisibleEntries)
	fmt.Fprintf(&b, "Loaded memory entries: %d\n", snapshot.LoadedMemoryEntries)
	fmt.Fprintf(&b, "Omitted memory entries: %d\n", snapshot.OmittedMemoryEntries)
	fmt.Fprintf(&b, "Timeline: span_days=%d largest_gap_days=%d gaps_over_one_day=%d\n", snapshot.TimelineSpanDays, snapshot.LargestGapDays, snapshot.GapsOverOneDay)
	fmt.Fprintf(&b, "Validation: %s (%d errors, %d warnings, secret_findings=%d)\n", snapshot.ValidationStatus, snapshot.ValidationErrors, snapshot.ValidationWarnings, snapshot.PotentialSecretFindings)
	fmt.Fprintf(&b, "Risk: %s (%d findings, high=%d warning=%d info=%d)\n", snapshot.RiskStatus, snapshot.RiskFindings, snapshot.HighRiskFindings, snapshot.WarningRiskFindings, snapshot.InfoRiskFindings)
	b.WriteString("\nMemory writes: disabled.\n")
	b.WriteString("Background promotion: disabled; git review required.\n")
	b.WriteString("External provider access: not configured and not performed.\n")
	b.WriteString("Embedding vectors: not included.\n")
	b.WriteString("Raw memory bodies: not included.\n")
	b.WriteString("Raw issue bodies: not included.\n")
	b.WriteString("Raw comment bodies: not included.\n")
	b.WriteString("Raw prompt bodies: not included.\n")
	b.WriteString("Raw session bodies: not included.\n")
	b.WriteString("Backup payloads: not included.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelMemoryStatusSnapshot(cfg Config, repoContext RepoContext) channelMemoryStatusSnapshot {
	report := BuildMemorySnapshotReport(cfg, repoContext)
	return channelMemoryStatusSnapshot{
		MemoryStatus:                 report.Status,
		SnapshotVersion:              report.SnapshotVersion,
		SnapshotScope:                report.SnapshotScope,
		SnapshotSHA:                  report.SnapshotSHA,
		SnapshotEntries:              report.SnapshotEntries,
		LongTermEntries:              report.LongTermEntries,
		DatedNoteEntries:             report.DatedNoteEntries,
		MemoryNoteEntries:            report.MemoryNoteEntries,
		PromptVisibleEntries:         report.PromptVisibleEntries,
		LoadedMemoryEntries:          report.LoadedMemoryEntries,
		OmittedMemoryEntries:         report.OmittedMemoryEntries,
		MemoryFiles:                  report.MemoryFiles,
		LongTermMemoryPresent:        report.LongTermMemoryPresent,
		LongTermMemoryLoaded:         report.LongTermMemoryLoaded,
		DatedMemoryNotes:             report.DatedMemoryNotes,
		CanonicalDatedMemoryNotes:    report.CanonicalDatedMemoryNotes,
		NoncanonicalDatedMemoryNotes: report.NoncanonicalDatedMemoryNotes,
		LoadedMemoryNotes:            report.LoadedMemoryNotes,
		MaxLoadedMemoryNotes:         report.MaxLoadedMemoryNotes,
		FirstMemoryNoteHash:          shortDocumentHash(report.FirstMemoryNote),
		LatestMemoryNoteHash:         shortDocumentHash(report.LatestMemoryNote),
		TimelineSpanDays:             report.TimelineSpanDays,
		LargestGapDays:               report.LargestGapDays,
		GapsOverOneDay:               report.GapsOverOneDay,
		ValidationStatus:             report.Validation.Status,
		ValidationErrors:             report.Validation.Errors,
		ValidationWarnings:           report.Validation.Warnings,
		EmptyMemoryFiles:             report.Validation.EmptyMemoryFiles,
		MemoryFilesAtLimit:           report.Validation.MemoryFilesAtLimit,
		PotentialSecretFindings:      report.Validation.PotentialSecretFindings,
		RiskStatus:                   report.Risk.Status,
		ScannedMemoryFiles:           report.Risk.ScannedMemoryFiles,
		MemoryFilesWithRiskFindings:  report.Risk.MemoryFilesWithRiskFindings,
		RiskFindings:                 len(report.Risk.Findings),
		HighRiskFindings:             report.Risk.HighRiskFindings,
		WarningRiskFindings:          report.Risk.WarningRiskFindings,
		InfoRiskFindings:             report.Risk.InfoRiskFindings,
		MemoryWritesAllowed:          report.MemoryWritesAllowed,
		ExternalProviderAccessed:     report.ExternalProviderAccessed,
		ExternalProviderVerification: report.Risk.ExternalProviderVerification,
		BackgroundPromotionActive:    report.BackgroundPromotionActive,
		BackgroundPromotionReview:    report.Risk.BackgroundPromotionReview,
		RawMemoryBodiesIncluded:      report.RawMemoryBodiesIncluded,
		RawIssueBodiesIncluded:       report.RawIssueBodiesIncluded,
		RawCommentBodiesIncluded:     report.RawCommentBodiesIncluded,
		RawPromptBodiesIncluded:      report.RawPromptBodiesIncluded,
		RawSessionBodiesIncluded:     report.RawSessionBodiesIncluded,
		EmbeddingVectorsIncluded:     report.EmbeddingVectorsIncluded,
	}
}

func (r *ChannelMemoryStatusActionRequest) applySnapshot(snapshot channelMemoryStatusSnapshot) {
	r.MemoryStatus = snapshot.MemoryStatus
	r.SnapshotVersion = snapshot.SnapshotVersion
	r.SnapshotScope = snapshot.SnapshotScope
	r.SnapshotSHA = snapshot.SnapshotSHA
	r.SnapshotEntries = snapshot.SnapshotEntries
	r.LongTermEntries = snapshot.LongTermEntries
	r.DatedNoteEntries = snapshot.DatedNoteEntries
	r.MemoryNoteEntries = snapshot.MemoryNoteEntries
	r.PromptVisibleEntries = snapshot.PromptVisibleEntries
	r.LoadedMemoryEntries = snapshot.LoadedMemoryEntries
	r.OmittedMemoryEntries = snapshot.OmittedMemoryEntries
	r.MemoryFiles = snapshot.MemoryFiles
	r.LongTermMemoryPresent = snapshot.LongTermMemoryPresent
	r.LongTermMemoryLoaded = snapshot.LongTermMemoryLoaded
	r.DatedMemoryNotes = snapshot.DatedMemoryNotes
	r.CanonicalDatedMemoryNotes = snapshot.CanonicalDatedMemoryNotes
	r.NoncanonicalDatedMemoryNotes = snapshot.NoncanonicalDatedMemoryNotes
	r.LoadedMemoryNotes = snapshot.LoadedMemoryNotes
	r.MaxLoadedMemoryNotes = snapshot.MaxLoadedMemoryNotes
	r.FirstMemoryNoteHash = snapshot.FirstMemoryNoteHash
	r.LatestMemoryNoteHash = snapshot.LatestMemoryNoteHash
	r.TimelineSpanDays = snapshot.TimelineSpanDays
	r.LargestGapDays = snapshot.LargestGapDays
	r.GapsOverOneDay = snapshot.GapsOverOneDay
	r.ValidationStatus = snapshot.ValidationStatus
	r.ValidationErrors = snapshot.ValidationErrors
	r.ValidationWarnings = snapshot.ValidationWarnings
	r.EmptyMemoryFiles = snapshot.EmptyMemoryFiles
	r.MemoryFilesAtLimit = snapshot.MemoryFilesAtLimit
	r.PotentialSecretFindings = snapshot.PotentialSecretFindings
	r.RiskStatus = snapshot.RiskStatus
	r.ScannedMemoryFiles = snapshot.ScannedMemoryFiles
	r.MemoryFilesWithRiskFindings = snapshot.MemoryFilesWithRiskFindings
	r.RiskFindings = snapshot.RiskFindings
	r.HighRiskFindings = snapshot.HighRiskFindings
	r.WarningRiskFindings = snapshot.WarningRiskFindings
	r.InfoRiskFindings = snapshot.InfoRiskFindings
	r.MemoryWritesAllowed = snapshot.MemoryWritesAllowed
	r.ExternalProviderAccessed = snapshot.ExternalProviderAccessed
	r.ExternalProviderVerification = snapshot.ExternalProviderVerification
	r.BackgroundPromotionActive = snapshot.BackgroundPromotionActive
	r.BackgroundPromotionReview = snapshot.BackgroundPromotionReview
	r.RawMemoryBodiesIncluded = snapshot.RawMemoryBodiesIncluded
	r.RawIssueBodiesIncluded = snapshot.RawIssueBodiesIncluded
	r.RawCommentBodiesIncluded = snapshot.RawCommentBodiesIncluded
	r.RawPromptBodiesIncluded = snapshot.RawPromptBodiesIncluded
	r.RawSessionBodiesIncluded = snapshot.RawSessionBodiesIncluded
	r.EmbeddingVectorsIncluded = snapshot.EmbeddingVectorsIncluded
}

func (r *ChannelMemoryStatusResult) applySnapshot(snapshot channelMemoryStatusSnapshot) {
	r.MemoryStatus = snapshot.MemoryStatus
	r.SnapshotVersion = snapshot.SnapshotVersion
	r.SnapshotScope = snapshot.SnapshotScope
	r.SnapshotSHA = snapshot.SnapshotSHA
	r.SnapshotEntries = snapshot.SnapshotEntries
	r.LongTermEntries = snapshot.LongTermEntries
	r.DatedNoteEntries = snapshot.DatedNoteEntries
	r.MemoryNoteEntries = snapshot.MemoryNoteEntries
	r.PromptVisibleEntries = snapshot.PromptVisibleEntries
	r.LoadedMemoryEntries = snapshot.LoadedMemoryEntries
	r.OmittedMemoryEntries = snapshot.OmittedMemoryEntries
	r.MemoryFiles = snapshot.MemoryFiles
	r.LongTermMemoryPresent = snapshot.LongTermMemoryPresent
	r.LongTermMemoryLoaded = snapshot.LongTermMemoryLoaded
	r.DatedMemoryNotes = snapshot.DatedMemoryNotes
	r.CanonicalDatedMemoryNotes = snapshot.CanonicalDatedMemoryNotes
	r.NoncanonicalDatedMemoryNotes = snapshot.NoncanonicalDatedMemoryNotes
	r.LoadedMemoryNotes = snapshot.LoadedMemoryNotes
	r.MaxLoadedMemoryNotes = snapshot.MaxLoadedMemoryNotes
	r.FirstMemoryNoteHash = snapshot.FirstMemoryNoteHash
	r.LatestMemoryNoteHash = snapshot.LatestMemoryNoteHash
	r.TimelineSpanDays = snapshot.TimelineSpanDays
	r.LargestGapDays = snapshot.LargestGapDays
	r.GapsOverOneDay = snapshot.GapsOverOneDay
	r.ValidationStatus = snapshot.ValidationStatus
	r.ValidationErrors = snapshot.ValidationErrors
	r.ValidationWarnings = snapshot.ValidationWarnings
	r.EmptyMemoryFiles = snapshot.EmptyMemoryFiles
	r.MemoryFilesAtLimit = snapshot.MemoryFilesAtLimit
	r.PotentialSecretFindings = snapshot.PotentialSecretFindings
	r.RiskStatus = snapshot.RiskStatus
	r.ScannedMemoryFiles = snapshot.ScannedMemoryFiles
	r.MemoryFilesWithRiskFindings = snapshot.MemoryFilesWithRiskFindings
	r.RiskFindings = snapshot.RiskFindings
	r.HighRiskFindings = snapshot.HighRiskFindings
	r.WarningRiskFindings = snapshot.WarningRiskFindings
	r.InfoRiskFindings = snapshot.InfoRiskFindings
	r.MemoryWritesAllowed = snapshot.MemoryWritesAllowed
	r.ExternalProviderAccessed = snapshot.ExternalProviderAccessed
	r.ExternalProviderVerification = snapshot.ExternalProviderVerification
	r.BackgroundPromotionActive = snapshot.BackgroundPromotionActive
	r.BackgroundPromotionReview = snapshot.BackgroundPromotionReview
	r.RawMemoryBodiesIncluded = snapshot.RawMemoryBodiesIncluded
	r.RawIssueBodiesIncluded = snapshot.RawIssueBodiesIncluded
	r.RawCommentBodiesIncluded = snapshot.RawCommentBodiesIncluded
	r.RawPromptBodiesIncluded = snapshot.RawPromptBodiesIncluded
	r.RawSessionBodiesIncluded = snapshot.RawSessionBodiesIncluded
	r.EmbeddingVectorsIncluded = snapshot.EmbeddingVectorsIncluded
}
