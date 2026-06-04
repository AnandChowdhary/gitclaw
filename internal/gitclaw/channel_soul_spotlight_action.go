package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

type ChannelSoulSpotlightOptions struct {
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

type ChannelSoulSpotlightReport struct {
	SpotlightStatus      string
	FocusHash            string
	FocusTerms           int
	AvailableSoulFiles   int
	PresentSoulFiles     int
	EligibleSoulFiles    int
	MatchedSoulFiles     int
	CandidateSoulFiles   int
	SelectedIndex        int
	SelectedMatch        soulInfoMatchResult
	SelectionSeedSHA     string
	SelectionSHA         string
	ValidationStatus     string
	ValidationErrors     int
	ValidationWarnings   int
	RequiredFiles        int
	PresentRequiredFiles int
	MissingRequiredFiles int
	MemoryNotes          int
	RiskStatus           string
	RiskFindings         int
	HighRiskFindings     int
	WarningRiskFindings  int
	InfoRiskFindings     int
	RawBodiesIncluded    bool
}

type ChannelSoulSpotlightResult struct {
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
	SelectedPathHash    string
	SelectedCategorySHA string
	SelectedSourceSHA   string
	SelectionSeedHash   string
	SelectionHash       string
	Report              ChannelSoulSpotlightReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSoulSpotlightActionRequest struct {
	Options             ChannelSoulSpotlightOptions
	Report              ChannelSoulSpotlightReport
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
	SelectedPathHash    string
	SelectedCategorySHA string
	SelectedSourceSHA   string
	SelectionSeedSHA    string
	SelectionSHA        string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSoulSpotlightActionRequest(ev Event, cfg Config) bool {
	return isChannelSoulSpotlightActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoulSpotlightActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSoulSpotlightSubcommand(fields[1]) {
	case "soul-spotlight", "souls-spotlight", "spotlight-soul", "soul-pick", "soul-draw", "authority-spotlight", "authority-draw", "context-spotlight", "context-draw":
		return true
	default:
		return false
	}
}

func BuildChannelSoulSpotlightActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulSpotlightActionRequest, error) {
	fields, trailing, ok := channelSoulSpotlightActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("missing channel soul spotlight command")
	}
	req := ChannelSoulSpotlightActionRequest{
		Options: ChannelSoulSpotlightOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             defaultChannelSoulSpotlightFocus(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSoulSpotlightSubcommand(fields[1]),
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
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--spotlight-id", "--soul-spotlight-id", "--soul-pick-id", "--soul-draw-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SpotlightID = cleanChannelSoulSpotlightID(fields[i+1])
			i++
		case "--focus", "--soul", "--path", "--query", "--for":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulSpotlightActionRequest{}, fmt.Errorf("unknown channel soul spotlight argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelSoulSpotlightIssueTargetIfPresent(ev, &req)
	if err := applyChannelSoulSpotlightPositionals(&req, positional); err != nil {
		return ChannelSoulSpotlightActionRequest{}, err
	}
	if err := applyChannelSoulSpotlightIssueTarget(ev, &req); err != nil {
		return ChannelSoulSpotlightActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelSoulSpotlightTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSoulSpotlightSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SpotlightID) == "" {
		req.Options.SpotlightID = autoChannelSoulSpotlightID(ev, req.Options)
		req.AutoSpotlightID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulSpotlightNotifyMessageID(ev, req.Options.SpotlightID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulSpotlightOptions(req.Options)
	if err := validateChannelSoulSpotlightActionRequestOptions(req.Options); err != nil {
		return ChannelSoulSpotlightActionRequest{}, err
	}
	req.Report = BuildChannelSoulSpotlightReport(cfg, repoContext, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.SpotlightIDHash = shortDocumentHash(req.Options.SpotlightID)
	req.FocusSHA = req.Report.FocusHash
	req.FocusBytes = len(req.Options.Focus)
	req.FocusTerms = req.Report.FocusTerms
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.SelectedPathHash = shortDocumentHash(req.Report.SelectedMatch.Path)
	req.SelectedCategorySHA = shortDocumentHash(req.Report.SelectedMatch.Category)
	req.SelectedSourceSHA = shortDocumentHash(req.Report.SelectedMatch.Source)
	req.SelectionSeedSHA = req.Report.SelectionSeedSHA
	req.SelectionSHA = req.Report.SelectionSHA
	notificationBody := renderChannelSoulSpotlightNotificationBody(req.Options, req.Report)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelSoulSpotlightReport(cfg Config, repoContext RepoContext, opts ChannelSoulSpotlightOptions) ChannelSoulSpotlightReport {
	focus := cleanChannelSoulSpotlightFocus(opts.Focus)
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	report := ChannelSoulSpotlightReport{
		SpotlightStatus:      "ok",
		FocusHash:            shortDocumentHash(focus),
		FocusTerms:           len(memorySearchTerms(focus)),
		AvailableSoulFiles:   channelSoulSpotlightAvailableCount(cfg, repoContext),
		PresentSoulFiles:     channelSoulSpotlightPresentCount(cfg, repoContext),
		EligibleSoulFiles:    channelSoulSpotlightEligibleCount(cfg, repoContext),
		ValidationStatus:     validation.Status,
		ValidationErrors:     validation.Errors,
		ValidationWarnings:   validation.Warnings,
		RequiredFiles:        validation.RequiredFiles,
		PresentRequiredFiles: validation.PresentRequiredFiles,
		MissingRequiredFiles: validation.MissingRequiredFiles,
		MemoryNotes:          validation.MemoryNotes,
		RiskStatus:           risk.Status,
		RiskFindings:         len(risk.Findings),
		HighRiskFindings:     risk.HighRiskFindings,
		WarningRiskFindings:  risk.WarningRiskFindings,
		InfoRiskFindings:     risk.InfoRiskFindings,
		RawBodiesIncluded:    false,
	}
	candidates := channelSoulSpotlightCandidates(cfg, repoContext, focus)
	if focus != "" && focus != "general" {
		report.MatchedSoulFiles = len(channelSoulSpotlightMatchingMatches(cfg, repoContext, focus))
		if len(candidates) == 0 {
			candidates = channelSoulSpotlightCandidates(cfg, repoContext, "")
			if len(candidates) > 0 {
				report.SpotlightStatus = "fallback"
			}
		}
	}
	if report.MatchedSoulFiles == 0 && (focus == "" || focus == "general") {
		report.MatchedSoulFiles = len(candidates)
	}
	report.CandidateSoulFiles = len(candidates)
	seed := channelSoulSpotlightSeed(opts, focus)
	report.SelectionSeedSHA = shortDocumentHash(seed)
	if len(candidates) == 0 {
		report.SpotlightStatus = "no_eligible_soul_files"
		report.SelectedIndex = -1
		report.SelectionSHA = "none"
		return report
	}
	idx := deterministicChannelSoulSpotlightIndex(seed, len(candidates))
	selected := candidates[idx]
	report.SelectedIndex = idx
	report.SelectedMatch = selected
	report.SelectionSHA = shortDocumentHash(strings.Join([]string{
		selected.Path,
		selected.Category,
		selected.Source,
		selected.SHA,
		focus,
		fmt.Sprintf("%d", idx),
	}, "|"))
	return report
}

func RunChannelSoulSpotlight(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSoulSpotlightActionRequest, repoContext RepoContext) (ChannelSoulSpotlightResult, error) {
	opts := normalizeChannelSoulSpotlightOptions(req.Options)
	var err error
	opts, err = applyChannelSoulSpotlightRoute(cfg, opts)
	if err != nil {
		return ChannelSoulSpotlightResult{}, err
	}
	if err := validateChannelSoulSpotlightOptions(opts); err != nil {
		return ChannelSoulSpotlightResult{}, err
	}
	report := req.Report
	if report.SpotlightStatus == "" {
		report = BuildChannelSoulSpotlightReport(cfg, repoContext, opts)
	}
	body := renderChannelSoulSpotlightNotificationBody(opts, report)
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
		return ChannelSoulSpotlightResult{}, fmt.Errorf("queue channel soul spotlight notification: %w", err)
	}
	return ChannelSoulSpotlightResult{
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
		SelectedPathHash:    shortDocumentHash(report.SelectedMatch.Path),
		SelectedCategorySHA: shortDocumentHash(report.SelectedMatch.Category),
		SelectedSourceSHA:   shortDocumentHash(report.SelectedMatch.Source),
		SelectionSeedHash:   report.SelectionSeedSHA,
		SelectionHash:       report.SelectionSHA,
		Report:              report,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelSoulSpotlightActionReport(ev Event, req ChannelSoulSpotlightActionRequest, result ChannelSoulSpotlightResult) string {
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
	selectedPathHash := firstNonEmpty(result.SelectedPathHash, req.SelectedPathHash)
	selectedCategoryHash := firstNonEmpty(result.SelectedCategorySHA, req.SelectedCategorySHA)
	selectedSourceHash := firstNonEmpty(result.SelectedSourceSHA, req.SelectedSourceSHA)
	selectionSeedHash := firstNonEmpty(result.SelectionSeedHash, req.SelectionSeedSHA)
	selectionHash := firstNonEmpty(result.SelectionHash, req.SelectionSHA)
	notificationBodySHA := firstNonEmpty(result.NotificationBodySHA, req.NotificationBodySHA)
	notificationBytes := result.NotificationBytes
	if notificationBytes == 0 {
		notificationBytes = req.NotificationBytes
	}
	notificationLines := result.NotificationLines
	if notificationLines == 0 {
		notificationLines = req.NotificationLines
	}
	report := result.Report
	if report.SpotlightStatus == "" {
		report = req.Report
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Soul Spotlight Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_spotlight_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_spotlight_status: `%s`\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "- spotlight_mode: `%s`\n", "repo-local-high-authority-deterministic-draw")
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
	fmt.Fprintf(&b, "- soul_spotlight_id_sha256_12: `%s`\n", noneIfEmpty(spotlightIDHash))
	fmt.Fprintf(&b, "- soul_spotlight_id_auto: `%t`\n", req.AutoSpotlightID)
	fmt.Fprintf(&b, "- spotlight_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- spotlight_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- spotlight_focus_terms: `%d`\n", report.FocusTerms)
	fmt.Fprintf(&b, "- spotlight_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- spotlight_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- spotlight_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- spotlight_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- spotlight_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- available_soul_files: `%d`\n", report.AvailableSoulFiles)
	fmt.Fprintf(&b, "- present_soul_files: `%d`\n", report.PresentSoulFiles)
	fmt.Fprintf(&b, "- eligible_soul_files: `%d`\n", report.EligibleSoulFiles)
	fmt.Fprintf(&b, "- matched_soul_files: `%d`\n", report.MatchedSoulFiles)
	fmt.Fprintf(&b, "- candidate_soul_files: `%d`\n", report.CandidateSoulFiles)
	fmt.Fprintf(&b, "- selected_index: `%d`\n", report.SelectedIndex)
	fmt.Fprintf(&b, "- selected_soul_path_sha256_12: `%s`\n", noneIfEmpty(selectedPathHash))
	fmt.Fprintf(&b, "- selected_soul_category_sha256_12: `%s`\n", noneIfEmpty(selectedCategoryHash))
	fmt.Fprintf(&b, "- selected_soul_source_sha256_12: `%s`\n", noneIfEmpty(selectedSourceHash))
	fmt.Fprintf(&b, "- selection_seed_sha256_12: `%s`\n", noneIfEmpty(selectionSeedHash))
	fmt.Fprintf(&b, "- selection_sha256_12: `%s`\n", noneIfEmpty(selectionHash))
	fmt.Fprintf(&b, "- validation_status: `%s`\n", report.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", report.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", report.ValidationWarnings)
	fmt.Fprintf(&b, "- required_files: `%d`\n", report.RequiredFiles)
	fmt.Fprintf(&b, "- present_required_files: `%d`\n", report.PresentRequiredFiles)
	fmt.Fprintf(&b, "- missing_required_files: `%d`\n", report.MissingRequiredFiles)
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", report.MemoryNotes)
	fmt.Fprintf(&b, "- risk_status: `%s`\n", report.RiskStatus)
	fmt.Fprintf(&b, "- risk_findings: `%d`\n", report.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- deterministic_selection: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_spotlight_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selection_seed_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_file_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_identity_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_user_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_guidance_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_heartbeat_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_spotlight_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing high-authority context spotlight card from repo-local soul metadata. The provider card may name one safe context path so people can act on it, while the source receipt keeps raw paths, bodies, ids, focus text, notes, and channel bodies out of band. The action does not call a model, execute tools, mutate repository files, write soul or memory, contact registries, export profiles, use external randomness, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read soul-spotlight cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent soul-spotlight cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate soul-spotlight notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelSoulSpotlightNotificationBody(opts ChannelSoulSpotlightOptions, report ChannelSoulSpotlightReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul spotlight\n\n")
	fmt.Fprintf(&b, "Spotlight status: %s\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Available soul files: %d\n", report.AvailableSoulFiles)
	fmt.Fprintf(&b, "Present soul files: %d\n", report.PresentSoulFiles)
	fmt.Fprintf(&b, "Eligible soul files: %d\n", report.EligibleSoulFiles)
	fmt.Fprintf(&b, "Matched soul files: %d\n", report.MatchedSoulFiles)
	fmt.Fprintf(&b, "Candidate soul files: %d\n", report.CandidateSoulFiles)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Selection seed hash: %s\n", report.SelectionSeedSHA)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarnings)
	fmt.Fprintf(&b, "Risk status: %s\n", report.RiskStatus)
	fmt.Fprintf(&b, "Risk findings: %d\n", report.RiskFindings)
	fmt.Fprintf(&b, "Soul spotlight id hash: %s\n", shortDocumentHash(opts.SpotlightID))
	b.WriteString("\nSpotlight:\n")
	if report.SpotlightStatus == "no_eligible_soul_files" || strings.TrimSpace(report.SelectedMatch.Path) == "" {
		b.WriteString("- none\n")
	} else {
		match := report.SelectedMatch
		fmt.Fprintf(&b, "- path=%s category=%s source=%s present=%t required=%t canonical=%t latest=%t loaded_for_this_turn=%t bytes=%d lines=%d sha256_12=%s at_context_limit=%t\n",
			match.Path,
			match.Category,
			match.Source,
			match.Present,
			match.Required,
			match.Canonical,
			match.Latest,
			match.LoadedForThisTurn,
			match.Bytes,
			match.Lines,
			match.SHA,
			match.AtContextLimit,
		)
		b.WriteString("\nTry next:\n")
		fmt.Fprintf(&b, "- @gitclaw /channels soul-info %s --message-id <message> --notify-message-id <message>\n", match.Path)
		fmt.Fprintf(&b, "- @gitclaw /channels soul-search %s --message-id <message> --notify-message-id <message>\n", match.Category)
	}
	b.WriteString("\nRaw soul, identity, user, memory, tool guidance, heartbeat, channel, issue, comment, prompt, and tool output bodies are not included in the source receipt. Raw focus text, raw notes, raw spotlight ids, and raw selected paths are not included in the source receipt. Model call: not performed by this action. Tool execution: not performed by this action. Soul write: not performed by this action. Memory write: not performed by this action. Registry contact: not performed by this action. Profile export: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSoulSpotlightActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSoulSpotlightActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSoulSpotlightIssueTarget(ev Event, req *ChannelSoulSpotlightActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul spotlight requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelSoulSpotlightIssueTargetIfPresent(ev Event, req *ChannelSoulSpotlightActionRequest) {
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

func applyChannelSoulSpotlightPositionals(req *ChannelSoulSpotlightActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel soul spotlight argument %q", value)
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
		return fmt.Errorf("unexpected channel soul spotlight argument %q", value)
	}
	return nil
}

func normalizeChannelSoulSpotlightOptions(opts ChannelSoulSpotlightOptions) ChannelSoulSpotlightOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SpotlightID = cleanChannelSoulSpotlightID(opts.SpotlightID)
	opts.Focus = cleanChannelSoulSpotlightFocus(opts.Focus)
	opts.Note = cleanChannelSoulSpotlightNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSoulSpotlightRoute(cfg Config, opts ChannelSoulSpotlightOptions) (ChannelSoulSpotlightOptions, error) {
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
		Body:      "GitClaw channel soul spotlight.",
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

func validateChannelSoulSpotlightOptions(opts ChannelSoulSpotlightOptions) error {
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
		return fmt.Errorf("missing soul spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid soul spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func validateChannelSoulSpotlightActionRequestOptions(opts ChannelSoulSpotlightOptions) error {
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
		return fmt.Errorf("missing soul spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid soul spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func cleanChannelSoulSpotlightSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSoulSpotlightID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSoulSpotlightFocus(value string) string {
	value = cleanChannelSoulSearchQuery(value)
	if value == "" {
		return "general"
	}
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func cleanChannelSoulSpotlightNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelSoulSpotlightTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelSoulSpotlightTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelSoulSpotlightNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelSoulSpotlightTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelSoulSpotlightFocus(subcommand string) string {
	switch cleanChannelSoulSpotlightSubcommand(subcommand) {
	case "authority-spotlight", "authority-draw", "context-spotlight", "context-draw":
		return "authority"
	default:
		return "general"
	}
}

func autoChannelSoulSpotlightSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-soul-spotlight-source-%s", eventID(ev))
}

func autoChannelSoulSpotlightID(ev Event, opts ChannelSoulSpotlightOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("soul-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSoulSpotlightNotifyMessageID(ev Event, spotlightID string) string {
	seed := strings.Join([]string{eventID(ev), spotlightID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelSoulSpotlightAvailableCount(cfg Config, repoContext RepoContext) int {
	return len(channelSoulSpotlightAllMatches(cfg, repoContext))
}

func channelSoulSpotlightPresentCount(cfg Config, repoContext RepoContext) int {
	count := 0
	for _, match := range channelSoulSpotlightAllMatches(cfg, repoContext) {
		if match.Present {
			count++
		}
	}
	return count
}

func channelSoulSpotlightEligibleCount(cfg Config, repoContext RepoContext) int {
	return len(channelSoulSpotlightCandidates(cfg, repoContext, ""))
}

func channelSoulSpotlightMatchingMatches(cfg Config, repoContext RepoContext, focus string) []soulInfoMatchResult {
	focus = cleanChannelSoulSpotlightFocus(focus)
	if focus == "" || focus == "general" {
		return nil
	}
	terms := memorySearchTerms(focus)
	var out []soulInfoMatchResult
	for _, match := range channelSoulSpotlightAllMatches(cfg, repoContext) {
		score, _ := channelSoulSpotlightMatchScore(match, focus, terms)
		if score == 0 {
			continue
		}
		out = append(out, match)
	}
	sortChannelSoulSpotlightMatches(out)
	return out
}

func channelSoulSpotlightCandidates(cfg Config, repoContext RepoContext, focus string) []soulInfoMatchResult {
	var source []soulInfoMatchResult
	if focus != "" && focus != "general" {
		source = channelSoulSpotlightMatchingMatches(cfg, repoContext, focus)
	} else {
		source = channelSoulSpotlightAllMatches(cfg, repoContext)
	}
	var candidates []soulInfoMatchResult
	for _, match := range source {
		if channelSoulSpotlightEligible(match) {
			candidates = append(candidates, match)
		}
	}
	sortChannelSoulSpotlightMatches(candidates)
	return candidates
}

func channelSoulSpotlightAllMatches(cfg Config, repoContext RepoContext) []soulInfoMatchResult {
	seen := map[string]bool{}
	var paths []string
	for _, path := range requiredSoulDocumentPaths {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for _, doc := range repoContext.Documents {
		if soulInfoAllowedPath(doc.Path) && !seen[doc.Path] {
			seen[doc.Path] = true
			paths = append(paths, doc.Path)
		}
	}
	var matches []soulInfoMatchResult
	for _, path := range paths {
		match, ok := soulInfoMatch(cfg.Workdir, repoContext, path)
		if ok {
			matches = append(matches, match)
		}
	}
	sortChannelSoulSpotlightMatches(matches)
	return matches
}

func channelSoulSpotlightEligible(match soulInfoMatchResult) bool {
	return match.Present && soulInfoAllowedPath(match.Path)
}

func channelSoulSpotlightMatchScore(match soulInfoMatchResult, query string, terms []string) (int, []string) {
	fields := map[string]string{
		"path":     match.Path,
		"category": match.Category,
		"source":   match.Source,
	}
	weights := map[string]int{
		"path":     80,
		"category": 60,
		"source":   20,
	}
	if match.Required {
		fields["required"] = "required high authority"
		weights["required"] = 30
	}
	if match.Latest {
		fields["latest"] = "latest memory"
		weights["latest"] = 30
	}
	return scoreSearchFields(fields, weights, query, terms)
}

func sortChannelSoulSpotlightMatches(matches []soulInfoMatchResult) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Category < matches[j].Category
	})
}

func channelSoulSpotlightSeed(opts ChannelSoulSpotlightOptions, focus string) string {
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

func deterministicChannelSoulSpotlightIndex(seed string, size int) int {
	if size <= 0 {
		return -1
	}
	sum := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint64(sum[:8]) % uint64(size))
}
