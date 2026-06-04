package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

const defaultChannelBackupSpotlightCandidateLimit = 25

type ChannelBackupSpotlightOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	SpotlightID       string
	Focus             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBackupSpotlightReport struct {
	SpotlightStatus       string
	FocusHash             string
	FocusTerms            int
	BackupVerifyStatus    string
	VerificationFailures  int
	BackupSchemaVersion   int
	IssueCount            int
	SearchStatus          string
	MatchedIssues         int
	MatchedLines          int
	CandidateBackups      int
	SelectedIndex         int
	SelectedBackup        channelBackupSpotlightCandidate
	SelectionSeedSHA      string
	SelectionSHA          string
	RawBodiesIncluded     bool
	BackupPayloadsRead    bool
	BackupPayloadsPrinted bool
}

type channelBackupSpotlightCandidate struct {
	SourceKind         string
	IssueNumber        int
	Path               string
	Source             string
	Role               string
	Trusted            bool
	Line               int
	Score              int
	MatchedTerms       int
	BodyBytes          int
	BodyLines          int
	BodySHA            string
	LineSHA            string
	BackupGeneratedAt  string
	EventName          string
	PayloadBytes       int
	PayloadSHA         string
	Comments           int
	TranscriptMessages int
	IssueTitleSHA      string
}

type ChannelBackupSpotlightResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	SpotlightIDHash     string
	FocusHash           string
	NoteHash            string
	SelectedIssueHash   string
	SelectedPathHash    string
	SelectedSourceHash  string
	SelectedRoleHash    string
	SelectionSeedHash   string
	SelectionHash       string
	Report              ChannelBackupSpotlightReport
	BackupFetchStatus   string
	BackupRootHash      string
	SpotlightErrorKind  string
	SpotlightErrorHash  string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupSpotlightActionRequest struct {
	Options             ChannelBackupSpotlightOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoSpotlightID     bool
	TargetFromIssue     bool
	FocusSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	SpotlightIDHash     string
	FocusSHA            string
	FocusBytes          int
	FocusTerms          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
}

func IsChannelBackupSpotlightActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupSpotlightActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupSpotlightActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBackupSpotlightSubcommand(fields[1]) {
	case "backup-spotlight", "backups-spotlight", "spotlight-backup", "backup-pick", "backup-draw", "recovery-spotlight", "recovery-draw", "archive-spotlight", "archive-draw":
		return true
	default:
		return false
	}
}

func BuildChannelBackupSpotlightActionRequest(ev Event, cfg Config) (ChannelBackupSpotlightActionRequest, error) {
	fields, trailing, ok := channelBackupSpotlightActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("missing channel backup spotlight command")
	}
	req := ChannelBackupSpotlightActionRequest{
		Options: ChannelBackupSpotlightOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             defaultChannelBackupSpotlightFocus(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelBackupSpotlightSubcommand(fields[1]),
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
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--spotlight-id", "--backup-spotlight-id", "--backup-pick-id", "--backup-draw-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SpotlightID = cleanChannelBackupSpotlightID(fields[i+1])
			i++
		case "--focus", "--backup", "--query", "--for":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupSpotlightActionRequest{}, fmt.Errorf("unknown channel backup spotlight argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelBackupSpotlightIssueTargetIfPresent(ev, &req)
	if err := applyChannelBackupSpotlightPositionals(&req, positional); err != nil {
		return ChannelBackupSpotlightActionRequest{}, err
	}
	if err := applyChannelBackupSpotlightIssueTarget(ev, &req); err != nil {
		return ChannelBackupSpotlightActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelBackupSpotlightTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupSpotlightSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SpotlightID) == "" {
		req.Options.SpotlightID = autoChannelBackupSpotlightID(ev, req.Options)
		req.AutoSpotlightID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupSpotlightNotifyMessageID(ev, req.Options.SpotlightID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupSpotlightOptions(req.Options)
	if err := validateChannelBackupSpotlightActionRequestOptions(req.Options); err != nil {
		return ChannelBackupSpotlightActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.SpotlightIDHash = shortDocumentHash(req.Options.SpotlightID)
	req.FocusSHA = shortDocumentHash(req.Options.Focus)
	req.FocusBytes = len(req.Options.Focus)
	req.FocusTerms = len(memorySearchTerms(req.Options.Focus))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	return req, nil
}

func BuildChannelBackupSpotlightReport(root string, opts ChannelBackupSpotlightOptions) (ChannelBackupSpotlightReport, error) {
	opts = normalizeChannelBackupSpotlightOptions(opts)
	if err := validateRepoName(opts.Repo); err != nil {
		return ChannelBackupSpotlightReport{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	focus := cleanChannelBackupSpotlightFocus(opts.Focus)
	repoDir, index, err := readBackupIndex(root, opts.Repo)
	if err != nil {
		return ChannelBackupSpotlightReport{}, err
	}
	verify, err := VerifyBackupTree(root, opts.Repo)
	if err != nil {
		return ChannelBackupSpotlightReport{}, err
	}
	report := ChannelBackupSpotlightReport{
		SpotlightStatus:       "ok",
		FocusHash:             shortDocumentHash(focus),
		FocusTerms:            len(memorySearchTerms(focus)),
		BackupVerifyStatus:    "ok",
		VerificationFailures:  len(verify.VerificationFailures),
		BackupSchemaVersion:   index.Version,
		IssueCount:            len(index.Issues),
		SearchStatus:          "not_requested",
		RawBodiesIncluded:     false,
		BackupPayloadsRead:    true,
		BackupPayloadsPrinted: false,
	}
	if !verify.OK() {
		report.BackupVerifyStatus = "warn"
	}
	var candidates []channelBackupSpotlightCandidate
	if focus != "" && focus != "general" {
		search, err := BuildBackupSearch(root, opts.Repo, focus, defaultChannelBackupSpotlightCandidateLimit)
		if err != nil {
			return ChannelBackupSpotlightReport{}, err
		}
		report.SearchStatus = search.SearchStatus
		report.MatchedIssues = search.MatchedIssues
		report.MatchedLines = search.MatchedLines
		candidates = channelBackupSpotlightCandidatesFromSearch(search)
		if len(candidates) == 0 {
			fallback, err := channelBackupSpotlightTimelineCandidates(root, opts.Repo, defaultChannelBackupSpotlightCandidateLimit)
			if err != nil {
				return ChannelBackupSpotlightReport{}, err
			}
			candidates = fallback
			if len(candidates) > 0 {
				report.SpotlightStatus = "fallback"
			}
		}
	} else {
		fallback, err := channelBackupSpotlightTimelineCandidates(root, opts.Repo, defaultChannelBackupSpotlightCandidateLimit)
		if err != nil {
			return ChannelBackupSpotlightReport{}, err
		}
		candidates = fallback
		report.MatchedIssues = len(candidates)
	}
	report.CandidateBackups = len(candidates)
	seed := channelBackupSpotlightSeed(opts, focus)
	report.SelectionSeedSHA = shortDocumentHash(seed)
	if len(candidates) == 0 {
		report.SpotlightStatus = "no_backups"
		report.SelectedIndex = -1
		report.SelectionSHA = "none"
		return report, nil
	}
	idx := deterministicChannelBackupSpotlightIndex(seed, len(candidates))
	selected := candidates[idx]
	if selected.SourceKind == "search" && selected.PayloadSHA == "" {
		selected = enrichChannelBackupSpotlightCandidate(repoDir, opts.Repo, index, selected)
	}
	report.SelectedIndex = idx
	report.SelectedBackup = selected
	report.SelectionSHA = shortDocumentHash(strings.Join([]string{
		fmt.Sprintf("%d", selected.IssueNumber),
		selected.Path,
		selected.Source,
		selected.Role,
		selected.LineSHA,
		selected.PayloadSHA,
		focus,
		fmt.Sprintf("%d", idx),
	}, "|"))
	return report, nil
}

func RunChannelBackupSpotlight(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelBackupSpotlightActionRequest) (ChannelBackupSpotlightResult, error) {
	opts := normalizeChannelBackupSpotlightOptions(req.Options)
	var err error
	opts, err = applyChannelBackupSpotlightRoute(cfg, opts)
	if err != nil {
		return ChannelBackupSpotlightResult{}, err
	}
	if err := validateChannelBackupSpotlightOptions(opts); err != nil {
		return ChannelBackupSpotlightResult{}, err
	}
	report, backupRoot, fetchStatus, spotlightErr := loadChannelBackupSpotlightReport(ctx, cfg, opts)
	errorKind := ""
	errorHash := ""
	if spotlightErr != nil {
		errorKind = channelBackupSpotlightErrorKind(spotlightErr)
		errorHash = shortDocumentHash(spotlightErr.Error())
	}
	body := renderChannelBackupSpotlightNotificationBody(opts, report, fetchStatus, errorKind)
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
		return ChannelBackupSpotlightResult{}, fmt.Errorf("queue channel backup spotlight notification: %w", err)
	}
	selected := report.SelectedBackup
	return ChannelBackupSpotlightResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		SpotlightIDHash:     shortDocumentHash(opts.SpotlightID),
		FocusHash:           report.FocusHash,
		NoteHash:            shortDocumentHash(opts.Note),
		SelectedIssueHash:   channelBackupSpotlightIssueHash(selected.IssueNumber),
		SelectedPathHash:    shortDocumentHash(selected.Path),
		SelectedSourceHash:  shortDocumentHash(selected.Source),
		SelectedRoleHash:    shortDocumentHash(selected.Role),
		SelectionSeedHash:   report.SelectionSeedSHA,
		SelectionHash:       report.SelectionSHA,
		Report:              report,
		BackupFetchStatus:   fetchStatus,
		BackupRootHash:      shortDocumentHash(backupRoot),
		SpotlightErrorKind:  errorKind,
		SpotlightErrorHash:  errorHash,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelBackupSpotlightActionReport(ev Event, req ChannelBackupSpotlightActionRequest, result ChannelBackupSpotlightResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	spotlightIDHash := firstNonEmpty(result.SpotlightIDHash, req.SpotlightIDHash)
	focusHash := firstNonEmpty(result.FocusHash, req.FocusSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	selectionSeedHash := result.SelectionSeedHash
	selectionHash := result.SelectionHash
	notificationBodySHA := result.NotificationBodySHA
	notificationBytes := result.NotificationBytes
	notificationLines := result.NotificationLines
	report := result.Report
	if report.SpotlightStatus == "" {
		report = unavailableChannelBackupSpotlightReport(req.Options.Repo, req.Options.Focus)
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Spotlight Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_spotlight_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_spotlight_status: `%s`\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", report.BackupVerifyStatus)
	fmt.Fprintf(&b, "- backup_fetch_status: `%s`\n", noneIfEmpty(result.BackupFetchStatus))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- spotlight_mode: `%s`\n", "gitclaw-backups-deterministic-recovery-draw")
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
	fmt.Fprintf(&b, "- backup_spotlight_id_sha256_12: `%s`\n", noneIfEmpty(spotlightIDHash))
	fmt.Fprintf(&b, "- backup_spotlight_id_auto: `%t`\n", req.AutoSpotlightID)
	fmt.Fprintf(&b, "- spotlight_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- spotlight_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- spotlight_focus_terms: `%d`\n", report.FocusTerms)
	fmt.Fprintf(&b, "- spotlight_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- spotlight_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- spotlight_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- spotlight_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- spotlight_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- backup_root_sha256_12: `%s`\n", noneIfEmpty(result.BackupRootHash))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", report.BackupSchemaVersion)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", report.VerificationFailures)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", report.IssueCount)
	fmt.Fprintf(&b, "- search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- matched_issues: `%d`\n", report.MatchedIssues)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", report.MatchedLines)
	fmt.Fprintf(&b, "- candidate_backups: `%d`\n", report.CandidateBackups)
	fmt.Fprintf(&b, "- selected_index: `%d`\n", report.SelectedIndex)
	fmt.Fprintf(&b, "- selected_backup_issue_sha256_12: `%s`\n", noneIfEmpty(result.SelectedIssueHash))
	fmt.Fprintf(&b, "- selected_backup_path_sha256_12: `%s`\n", noneIfEmpty(result.SelectedPathHash))
	fmt.Fprintf(&b, "- selected_backup_source_sha256_12: `%s`\n", noneIfEmpty(result.SelectedSourceHash))
	fmt.Fprintf(&b, "- selected_backup_role_sha256_12: `%s`\n", noneIfEmpty(result.SelectedRoleHash))
	fmt.Fprintf(&b, "- selected_backup_line_sha256_12: `%s`\n", noneIfEmpty(report.SelectedBackup.LineSHA))
	fmt.Fprintf(&b, "- selected_backup_payload_sha256_12: `%s`\n", noneIfEmpty(report.SelectedBackup.PayloadSHA))
	fmt.Fprintf(&b, "- selection_seed_sha256_12: `%s`\n", noneIfEmpty(selectionSeedHash))
	fmt.Fprintf(&b, "- selection_sha256_12: `%s`\n", noneIfEmpty(selectionHash))
	fmt.Fprintf(&b, "- spotlight_error_kind: `%s`\n", noneIfEmpty(result.SpotlightErrorKind))
	fmt.Fprintf(&b, "- spotlight_error_sha256_12: `%s`\n", noneIfEmpty(result.SpotlightErrorHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- deterministic_selection: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_read_performed: `%t`\n", result.BackupFetchStatus == "local" || result.BackupFetchStatus == "fetched")
	fmt.Fprintf(&b, "- backup_branch_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- restore_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_spotlight_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selection_seed_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_root_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_issue_titles_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_spotlight_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing backup spotlight card from the gitclaw-backups archive. The provider card may name one safe backup issue and path so people can inspect it next, while the source receipt keeps raw backup paths, payloads, issue titles, ids, focus text, notes, and channel bodies out of band. The action may read or fetch the backup branch read-only, but it does not write backups, restore files, replay GitHub APIs, call a model, execute tools, mutate repository files, use external randomness, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read backup-spotlight cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-spotlight cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-spotlight notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelBackupSpotlightNotificationBody(opts ChannelBackupSpotlightOptions, report ChannelBackupSpotlightReport, fetchStatus, errorKind string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup spotlight\n\n")
	fmt.Fprintf(&b, "Spotlight status: %s\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "Backup verify status: %s\n", report.BackupVerifyStatus)
	fmt.Fprintf(&b, "Backup branch: %s\n", defaultBackupBranch)
	fmt.Fprintf(&b, "Backup fetch status: %s\n", fetchStatus)
	if errorKind != "" {
		fmt.Fprintf(&b, "Spotlight error kind: %s\n", errorKind)
	}
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Backup schema version: %d\n", report.BackupSchemaVersion)
	fmt.Fprintf(&b, "Issue count: %d\n", report.IssueCount)
	fmt.Fprintf(&b, "Search status: %s\n", report.SearchStatus)
	fmt.Fprintf(&b, "Matched issues: %d\n", report.MatchedIssues)
	fmt.Fprintf(&b, "Matched lines: %d\n", report.MatchedLines)
	fmt.Fprintf(&b, "Candidate backups: %d\n", report.CandidateBackups)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Selection seed hash: %s\n", report.SelectionSeedSHA)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Backup spotlight id hash: %s\n", shortDocumentHash(opts.SpotlightID))
	b.WriteString("\nSpotlight:\n")
	if report.SpotlightStatus == "no_backups" || report.SpotlightStatus == "unavailable" || report.SelectedBackup.IssueNumber == 0 {
		b.WriteString("- none\n")
	} else {
		selected := report.SelectedBackup
		fmt.Fprintf(&b, "- issue=#%d path=%s source=%s source_kind=%s role=%s trusted=%t line=%d score=%d matched_terms=%d body_bytes=%d body_lines=%d body_sha256_12=%s line_sha256_12=%s payload_bytes=%d payload_sha256_12=%s comments=%d transcript_messages=%d title_sha256_12=%s generated_at=%s event=%s\n",
			selected.IssueNumber,
			selected.Path,
			selected.Source,
			selected.SourceKind,
			selected.Role,
			selected.Trusted,
			selected.Line,
			selected.Score,
			selected.MatchedTerms,
			selected.BodyBytes,
			selected.BodyLines,
			selected.BodySHA,
			selected.LineSHA,
			selected.PayloadBytes,
			selected.PayloadSHA,
			selected.Comments,
			selected.TranscriptMessages,
			selected.IssueTitleSHA,
			selected.BackupGeneratedAt,
			selected.EventName,
		)
		b.WriteString("\nTry next:\n")
		fmt.Fprintf(&b, "- @gitclaw /channels backup-info #%d --message-id <message> --notify-message-id <message>\n", selected.IssueNumber)
		if selected.Source != "" {
			fmt.Fprintf(&b, "- @gitclaw /channels backup-search %s --message-id <message> --notify-message-id <message>\n", selected.Source)
		} else {
			b.WriteString("- @gitclaw /channels backup-timeline --message-id <message> --notify-message-id <message>\n")
		}
	}
	b.WriteString("\nRaw backup payloads, backup issue titles, channel bodies, issue bodies, comment bodies, transcript messages, prompts, tool outputs, raw focus text, raw notes, and raw spotlight ids are not included in the source receipt. Model call: not performed by this action. Tool execution: not performed by this action. Repository mutation: not performed by this action. Backup branch write: not performed by this action. Restore mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func loadChannelBackupSpotlightReport(ctx context.Context, cfg Config, opts ChannelBackupSpotlightOptions) (ChannelBackupSpotlightReport, string, string, error) {
	localRoot := channelBackupSearchLocalRoot(cfg)
	if channelBackupSearchIndexExists(localRoot, opts.Repo) {
		report, err := BuildChannelBackupSpotlightReport(localRoot, opts)
		if err != nil {
			return unavailableChannelBackupSpotlightReport(opts.Repo, opts.Focus), localRoot, "local_error", err
		}
		return report, localRoot, "local", nil
	}
	worktree, cleanup, err := fetchChannelBackupSearchWorktree(ctx, cfg)
	if err != nil {
		return unavailableChannelBackupSpotlightReport(opts.Repo, opts.Focus), localRoot, "unavailable", err
	}
	defer cleanup()
	fetchedRoot := filepath.Join(worktree, defaultBackupRoot)
	report, err := BuildChannelBackupSpotlightReport(fetchedRoot, opts)
	if err != nil {
		return unavailableChannelBackupSpotlightReport(opts.Repo, opts.Focus), fetchedRoot, "fetched_error", err
	}
	return report, fetchedRoot, "fetched", nil
}

func unavailableChannelBackupSpotlightReport(repo, focus string) ChannelBackupSpotlightReport {
	focus = cleanChannelBackupSpotlightFocus(focus)
	return ChannelBackupSpotlightReport{
		SpotlightStatus:       "unavailable",
		FocusHash:             shortDocumentHash(focus),
		FocusTerms:            len(memorySearchTerms(focus)),
		BackupVerifyStatus:    "unavailable",
		SearchStatus:          "unavailable",
		SelectedIndex:         -1,
		SelectionSeedSHA:      shortDocumentHash(repo + "|" + focus),
		SelectionSHA:          "none",
		RawBodiesIncluded:     false,
		BackupPayloadsPrinted: false,
	}
}

func channelBackupSpotlightCandidatesFromSearch(search BackupSearchReport) []channelBackupSpotlightCandidate {
	candidates := make([]channelBackupSpotlightCandidate, 0, len(search.Results))
	for _, result := range search.Results {
		candidates = append(candidates, channelBackupSpotlightCandidate{
			SourceKind:        "search",
			IssueNumber:       result.IssueNumber,
			Path:              result.Path,
			Source:            result.Source,
			Role:              result.Role,
			Trusted:           result.Trusted,
			Line:              result.Line,
			Score:             result.Score,
			MatchedTerms:      result.MatchedTerms,
			BodyBytes:         result.BodyBytes,
			BodyLines:         result.BodyLines,
			BodySHA:           result.BodySHA,
			LineSHA:           result.LineSHA,
			BackupGeneratedAt: result.BackupGeneratedAt,
			EventName:         result.EventName,
		})
	}
	return candidates
}

func channelBackupSpotlightTimelineCandidates(root, repo string, limit int) ([]channelBackupSpotlightCandidate, error) {
	timeline, err := BuildBackupTimeline(root, repo, limit)
	if err != nil {
		return nil, err
	}
	candidates := make([]channelBackupSpotlightCandidate, 0, len(timeline.Points))
	for _, point := range timeline.Points {
		candidates = append(candidates, channelBackupSpotlightCandidate{
			SourceKind:         "latest",
			IssueNumber:        point.IssueNumber,
			Path:               point.Path,
			Source:             "backup.timeline",
			Role:               "backup",
			Trusted:            true,
			BackupGeneratedAt:  point.BackupGeneratedAt,
			EventName:          point.EventName,
			PayloadBytes:       point.PayloadBytes,
			PayloadSHA:         point.PayloadSHA,
			Comments:           point.Comments,
			TranscriptMessages: point.TranscriptMessages,
			IssueTitleSHA:      point.IssueTitleSHA,
		})
	}
	return candidates, nil
}

func enrichChannelBackupSpotlightCandidate(repoDir, repo string, index BackupIndex, candidate channelBackupSpotlightCandidate) channelBackupSpotlightCandidate {
	for _, issue := range index.Issues {
		if issue.Number != candidate.IssueNumber {
			continue
		}
		backup, payload, err := manifestPayload(repoDir, repo, issue)
		if err != nil {
			return candidate
		}
		candidate.PayloadBytes = payload.Bytes
		candidate.PayloadSHA = payload.SHA
		candidate.Comments = len(backup.Comments)
		candidate.TranscriptMessages = len(backup.Transcript)
		candidate.IssueTitleSHA = shortDocumentHash(backup.Issue.Title)
		return candidate
	}
	return candidate
}

func channelBackupSpotlightActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupSpotlightActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupSpotlightIssueTarget(ev Event, req *ChannelBackupSpotlightActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup spotlight requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelBackupSpotlightIssueTargetIfPresent(ev Event, req *ChannelBackupSpotlightActionRequest) {
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

func applyChannelBackupSpotlightPositionals(req *ChannelBackupSpotlightActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Focus == "" || req.Options.Focus == "general" {
				req.Options.Focus = value
				if req.FocusSource == "" {
					req.FocusSource = "positional"
				}
				continue
			}
			req.Options.Focus = strings.TrimSpace(req.Options.Focus + " " + value)
			if req.FocusSource == "" {
				req.FocusSource = "positional"
			}
			continue
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Focus == "" || req.Options.Focus == "general" {
			req.Options.Focus = value
			if req.FocusSource == "" {
				req.FocusSource = "positional"
			}
			continue
		}
		req.Options.Focus = strings.TrimSpace(req.Options.Focus + " " + value)
		if req.FocusSource == "" {
			req.FocusSource = "positional"
		}
	}
	return nil
}

func normalizeChannelBackupSpotlightOptions(opts ChannelBackupSpotlightOptions) ChannelBackupSpotlightOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SpotlightID = cleanChannelBackupSpotlightID(opts.SpotlightID)
	opts.Focus = cleanChannelBackupSpotlightFocus(opts.Focus)
	opts.Note = cleanChannelBackupSpotlightNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBackupSpotlightRoute(cfg Config, opts ChannelBackupSpotlightOptions) (ChannelBackupSpotlightOptions, error) {
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
		Body:      "GitClaw channel backup spotlight.",
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

func validateChannelBackupSpotlightOptions(opts ChannelBackupSpotlightOptions) error {
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
	if opts.SpotlightID == "" {
		return fmt.Errorf("missing backup spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid backup spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func validateChannelBackupSpotlightActionRequestOptions(opts ChannelBackupSpotlightOptions) error {
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
	if opts.SpotlightID == "" {
		return fmt.Errorf("missing backup spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid backup spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func cleanChannelBackupSpotlightSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelBackupSpotlightID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelBackupSpotlightFocus(value string) string {
	value = cleanChannelBackupSearchQuery(value)
	if value == "" {
		return "general"
	}
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func cleanChannelBackupSpotlightNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelBackupSpotlightTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelBackupSpotlightTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelBackupSpotlightNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelBackupSpotlightTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelBackupSpotlightFocus(subcommand string) string {
	switch cleanChannelBackupSpotlightSubcommand(subcommand) {
	case "recovery-spotlight", "recovery-draw":
		return "recovery"
	case "archive-spotlight", "archive-draw":
		return "archive"
	default:
		return "general"
	}
}

func autoChannelBackupSpotlightSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-spotlight-source-%s", eventID(ev))
}

func autoChannelBackupSpotlightID(ev Event, opts ChannelBackupSpotlightOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("backup-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBackupSpotlightNotifyMessageID(ev Event, spotlightID string) string {
	seed := strings.Join([]string{eventID(ev), spotlightID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelBackupSpotlightSeed(opts ChannelBackupSpotlightOptions, focus string) string {
	return strings.Join([]string{
		opts.Repo,
		opts.Route,
		opts.Channel,
		opts.ThreadID,
		opts.SourceMessageID,
		opts.NotifyMessageID,
		opts.SpotlightID,
		focus,
		opts.Note,
	}, "|")
}

func deterministicChannelBackupSpotlightIndex(seed string, size int) int {
	if size <= 0 {
		return -1
	}
	sum := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint64(sum[:8]) % uint64(size))
}

func channelBackupSpotlightIssueHash(issueNumber int) string {
	if issueNumber <= 0 {
		return ""
	}
	return shortDocumentHash(fmt.Sprintf("#%d", issueNumber))
}

func channelBackupSpotlightErrorKind(err error) string {
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
	case strings.Contains(text, "backup search"):
		return "backup_search_failed"
	default:
		return "backup_spotlight_failed"
	}
}
