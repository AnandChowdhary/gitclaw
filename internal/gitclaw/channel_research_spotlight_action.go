package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

type ChannelResearchSpotlightOptions struct {
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

type ChannelResearchSpotlightReport struct {
	SpotlightStatus          string
	FocusHash                string
	FocusTerms               int
	SourceSnapshotDate       string
	ReviewedSources          int
	PatternCoverage          int
	RejectedPatterns         int
	LocalResearchDocs        int
	LocalResearchDocsPresent int
	ResearchFollowups        int
	MatchedItems             int
	CandidateItems           int
	SelectedIndex            int
	SelectedCandidate        channelResearchSpotlightCandidate
	SelectionSeedSHA         string
	SelectionSHA             string
	RawBodiesIncluded        bool
	SourceFetchPerformed     bool
	LiveBrowsePerformed      bool
}

type channelResearchSpotlightCandidate struct {
	Kind       string
	ID         string
	System     string
	SourceKind string
	URL        string
	Pattern    string
	Decision   string
	Upstream   string
	Surface    string
	Status     string
	Gate       string
}

type ChannelResearchSpotlightResult struct {
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
	SelectedKindHash    string
	SelectedIDHash      string
	SelectedSystemHash  string
	SelectedURLHash     string
	SelectedPatternHash string
	SelectedSurfaceHash string
	SelectedGateHash    string
	SelectionSeedHash   string
	SelectionHash       string
	Report              ChannelResearchSpotlightReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelResearchSpotlightActionRequest struct {
	Options             ChannelResearchSpotlightOptions
	Report              ChannelResearchSpotlightReport
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
	SelectedKindHash    string
	SelectedIDHash      string
	SelectedSystemHash  string
	SelectedURLHash     string
	SelectedPatternHash string
	SelectedSurfaceHash string
	SelectedGateHash    string
	SelectionSeedSHA    string
	SelectionSHA        string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelResearchSpotlightActionRequest(ev Event, cfg Config) bool {
	return isChannelResearchSpotlightActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelResearchSpotlightActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelResearchSpotlightSubcommand(fields[1]) {
	case "research-spotlight", "research-draw", "research-pick", "landscape-spotlight", "landscape-draw", "openclaw-spotlight", "hermes-spotlight", "pattern-spotlight", "pattern-draw":
		return true
	default:
		return false
	}
}

func BuildChannelResearchSpotlightActionRequest(ev Event, cfg Config) (ChannelResearchSpotlightActionRequest, error) {
	fields, trailing, ok := channelResearchSpotlightActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("missing channel research spotlight command")
	}
	req := ChannelResearchSpotlightActionRequest{
		Options: ChannelResearchSpotlightOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             defaultChannelResearchSpotlightFocus(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelResearchSpotlightSubcommand(fields[1]),
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
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--spotlight-id", "--research-spotlight-id", "--research-draw-id", "--research-pick-id", "--id":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SpotlightID = cleanChannelResearchSpotlightID(fields[i+1])
			i++
		case "--focus", "--query", "--for", "--system":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelResearchSpotlightActionRequest{}, fmt.Errorf("unknown channel research spotlight argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelResearchSpotlightIssueTargetIfPresent(ev, &req)
	if err := applyChannelResearchSpotlightPositionals(&req, positional); err != nil {
		return ChannelResearchSpotlightActionRequest{}, err
	}
	if err := applyChannelResearchSpotlightIssueTarget(ev, &req); err != nil {
		return ChannelResearchSpotlightActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelResearchSpotlightTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelResearchSpotlightSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SpotlightID) == "" {
		req.Options.SpotlightID = autoChannelResearchSpotlightID(ev, req.Options)
		req.AutoSpotlightID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelResearchSpotlightNotifyMessageID(ev, req.Options.SpotlightID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelResearchSpotlightOptions(req.Options)
	if err := validateChannelResearchSpotlightActionRequestOptions(req.Options); err != nil {
		return ChannelResearchSpotlightActionRequest{}, err
	}
	req.Report = BuildChannelResearchSpotlightReport(cfg, req.Options)
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
	req.SelectedKindHash = shortDocumentHash(req.Report.SelectedCandidate.Kind)
	req.SelectedIDHash = shortDocumentHash(channelResearchSpotlightCandidateID(req.Report.SelectedCandidate))
	req.SelectedSystemHash = shortDocumentHash(req.Report.SelectedCandidate.System)
	req.SelectedURLHash = shortDocumentHash(req.Report.SelectedCandidate.URL)
	req.SelectedPatternHash = shortDocumentHash(req.Report.SelectedCandidate.Pattern)
	req.SelectedSurfaceHash = shortDocumentHash(req.Report.SelectedCandidate.Surface)
	req.SelectedGateHash = shortDocumentHash(req.Report.SelectedCandidate.Gate)
	req.SelectionSeedSHA = req.Report.SelectionSeedSHA
	req.SelectionSHA = req.Report.SelectionSHA
	notificationBody := renderChannelResearchSpotlightNotificationBody(req.Options, req.Report)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelResearchSpotlightReport(cfg Config, opts ChannelResearchSpotlightOptions) ChannelResearchSpotlightReport {
	focus := cleanChannelResearchSpotlightFocus(opts.Focus)
	surface := inspectResearchSurface(cfg.Workdir)
	report := ChannelResearchSpotlightReport{
		SpotlightStatus:          "ok",
		FocusHash:                shortDocumentHash(focus),
		FocusTerms:               len(memorySearchTerms(focus)),
		SourceSnapshotDate:       researchSnapshotDate,
		ReviewedSources:          len(researchSources()),
		PatternCoverage:          len(researchPatterns()),
		RejectedPatterns:         len(researchRejections()),
		LocalResearchDocs:        len(surface.Documents),
		LocalResearchDocsPresent: surface.DocumentsPresent,
		ResearchFollowups:        surface.Followups,
		RawBodiesIncluded:        false,
		SourceFetchPerformed:     false,
		LiveBrowsePerformed:      false,
	}
	candidates := channelResearchSpotlightCandidates(focus)
	if focus != "" && focus != "general" {
		report.MatchedItems = len(channelResearchSpotlightMatchingCandidates(focus))
		if len(candidates) == 0 {
			candidates = channelResearchSpotlightCandidates("")
			if len(candidates) > 0 {
				report.SpotlightStatus = "fallback"
			}
		}
	}
	if report.MatchedItems == 0 && (focus == "" || focus == "general") {
		report.MatchedItems = len(candidates)
	}
	report.CandidateItems = len(candidates)
	seed := channelResearchSpotlightSeed(opts, focus)
	report.SelectionSeedSHA = shortDocumentHash(seed)
	if len(candidates) == 0 {
		report.SpotlightStatus = "no_research_items"
		report.SelectedIndex = -1
		report.SelectionSHA = "none"
		return report
	}
	idx := deterministicChannelResearchSpotlightIndex(seed, len(candidates))
	selected := candidates[idx]
	report.SelectedIndex = idx
	report.SelectedCandidate = selected
	report.SelectionSHA = shortDocumentHash(strings.Join([]string{
		selected.Kind,
		channelResearchSpotlightCandidateID(selected),
		selected.System,
		selected.URL,
		selected.Pattern,
		selected.Surface,
		selected.Gate,
		focus,
		fmt.Sprintf("%d", idx),
	}, "|"))
	return report
}

func RunChannelResearchSpotlight(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelResearchSpotlightActionRequest) (ChannelResearchSpotlightResult, error) {
	opts := normalizeChannelResearchSpotlightOptions(req.Options)
	var err error
	opts, err = applyChannelResearchSpotlightRoute(cfg, opts)
	if err != nil {
		return ChannelResearchSpotlightResult{}, err
	}
	if err := validateChannelResearchSpotlightOptions(opts); err != nil {
		return ChannelResearchSpotlightResult{}, err
	}
	report := req.Report
	if report.SpotlightStatus == "" {
		report = BuildChannelResearchSpotlightReport(cfg, opts)
	}
	body := renderChannelResearchSpotlightNotificationBody(opts, report)
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
		return ChannelResearchSpotlightResult{}, fmt.Errorf("queue channel research spotlight notification: %w", err)
	}
	selected := report.SelectedCandidate
	return ChannelResearchSpotlightResult{
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
		SelectedKindHash:    shortDocumentHash(selected.Kind),
		SelectedIDHash:      shortDocumentHash(channelResearchSpotlightCandidateID(selected)),
		SelectedSystemHash:  shortDocumentHash(selected.System),
		SelectedURLHash:     shortDocumentHash(selected.URL),
		SelectedPatternHash: shortDocumentHash(selected.Pattern),
		SelectedSurfaceHash: shortDocumentHash(selected.Surface),
		SelectedGateHash:    shortDocumentHash(selected.Gate),
		SelectionSeedHash:   report.SelectionSeedSHA,
		SelectionHash:       report.SelectionSHA,
		Report:              report,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelResearchSpotlightActionReport(ev Event, req ChannelResearchSpotlightActionRequest, result ChannelResearchSpotlightResult) string {
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
	selectedKindHash := firstNonEmpty(result.SelectedKindHash, req.SelectedKindHash)
	selectedIDHash := firstNonEmpty(result.SelectedIDHash, req.SelectedIDHash)
	selectedSystemHash := firstNonEmpty(result.SelectedSystemHash, req.SelectedSystemHash)
	selectedURLHash := firstNonEmpty(result.SelectedURLHash, req.SelectedURLHash)
	selectedPatternHash := firstNonEmpty(result.SelectedPatternHash, req.SelectedPatternHash)
	selectedSurfaceHash := firstNonEmpty(result.SelectedSurfaceHash, req.SelectedSurfaceHash)
	selectedGateHash := firstNonEmpty(result.SelectedGateHash, req.SelectedGateHash)
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
	b.WriteString("## GitClaw Channel Research Spotlight Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_research_spotlight_status: `%s`\n", status)
	fmt.Fprintf(&b, "- research_spotlight_status: `%s`\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "- spotlight_mode: `%s`\n", "static-research-catalog-deterministic-draw")
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
	fmt.Fprintf(&b, "- research_spotlight_id_sha256_12: `%s`\n", noneIfEmpty(spotlightIDHash))
	fmt.Fprintf(&b, "- research_spotlight_id_auto: `%t`\n", req.AutoSpotlightID)
	fmt.Fprintf(&b, "- spotlight_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- spotlight_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- spotlight_focus_terms: `%d`\n", report.FocusTerms)
	fmt.Fprintf(&b, "- spotlight_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- spotlight_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- spotlight_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- spotlight_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- spotlight_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- source_snapshot_date: `%s`\n", report.SourceSnapshotDate)
	fmt.Fprintf(&b, "- reviewed_sources: `%d`\n", report.ReviewedSources)
	fmt.Fprintf(&b, "- pattern_coverage: `%d`\n", report.PatternCoverage)
	fmt.Fprintf(&b, "- rejected_patterns: `%d`\n", report.RejectedPatterns)
	fmt.Fprintf(&b, "- local_research_docs: `%d`\n", report.LocalResearchDocs)
	fmt.Fprintf(&b, "- local_research_docs_present: `%d`\n", report.LocalResearchDocsPresent)
	fmt.Fprintf(&b, "- research_followups_indexed: `%d`\n", report.ResearchFollowups)
	fmt.Fprintf(&b, "- matched_research_items: `%d`\n", report.MatchedItems)
	fmt.Fprintf(&b, "- candidate_research_items: `%d`\n", report.CandidateItems)
	fmt.Fprintf(&b, "- selected_index: `%d`\n", report.SelectedIndex)
	fmt.Fprintf(&b, "- selected_research_kind_sha256_12: `%s`\n", noneIfEmpty(selectedKindHash))
	fmt.Fprintf(&b, "- selected_research_id_sha256_12: `%s`\n", noneIfEmpty(selectedIDHash))
	fmt.Fprintf(&b, "- selected_research_system_sha256_12: `%s`\n", noneIfEmpty(selectedSystemHash))
	fmt.Fprintf(&b, "- selected_research_url_sha256_12: `%s`\n", noneIfEmpty(selectedURLHash))
	fmt.Fprintf(&b, "- selected_research_pattern_sha256_12: `%s`\n", noneIfEmpty(selectedPatternHash))
	fmt.Fprintf(&b, "- selected_research_surface_sha256_12: `%s`\n", noneIfEmpty(selectedSurfaceHash))
	fmt.Fprintf(&b, "- selected_research_gate_sha256_12: `%s`\n", noneIfEmpty(selectedGateHash))
	fmt.Fprintf(&b, "- selection_seed_sha256_12: `%s`\n", noneIfEmpty(selectionSeedHash))
	fmt.Fprintf(&b, "- selection_sha256_12: `%s`\n", noneIfEmpty(selectionHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- deterministic_selection: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_fetch_performed: `%t`\n", report.SourceFetchPerformed)
	fmt.Fprintf(&b, "- live_source_browse_performed: `%t`\n", report.LiveBrowsePerformed)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_spotlight_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selection_seed_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_source_ids_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_source_urls_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_patterns_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_surfaces_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_research_spotlight_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing research spotlight card from reviewed static catalog metadata. The provider card may name one source, pattern, or rejected surface so people can follow up, while this source receipt keeps raw ids, URLs, surfaces, pattern text, focus text, notes, channel bodies, and research bodies out of band. The action does not call a model, execute tools, fetch sources, browse live sources, mutate repository files, use external randomness, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read research-spotlight cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent research-spotlight cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate research-spotlight notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelResearchSpotlightNotificationBody(opts ChannelResearchSpotlightOptions, report ChannelResearchSpotlightReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel research spotlight\n\n")
	fmt.Fprintf(&b, "Spotlight status: %s\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "Source snapshot date: %s\n", report.SourceSnapshotDate)
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Reviewed sources: %d\n", report.ReviewedSources)
	fmt.Fprintf(&b, "Pattern coverage: %d\n", report.PatternCoverage)
	fmt.Fprintf(&b, "Rejected patterns: %d\n", report.RejectedPatterns)
	fmt.Fprintf(&b, "Local research docs: %d\n", report.LocalResearchDocs)
	fmt.Fprintf(&b, "Local research docs present: %d\n", report.LocalResearchDocsPresent)
	fmt.Fprintf(&b, "Research followups indexed: %d\n", report.ResearchFollowups)
	fmt.Fprintf(&b, "Matched items: %d\n", report.MatchedItems)
	fmt.Fprintf(&b, "Candidate items: %d\n", report.CandidateItems)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Selection seed hash: %s\n", report.SelectionSeedSHA)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Research spotlight id hash: %s\n", shortDocumentHash(opts.SpotlightID))
	b.WriteString("\nSpotlight:\n")
	if report.SpotlightStatus == "no_research_items" || strings.TrimSpace(report.SelectedCandidate.Kind) == "" {
		b.WriteString("- none\n")
	} else {
		writeChannelResearchSpotlightCandidate(&b, report.SelectedCandidate)
		b.WriteString("\nTry next:\n")
		b.WriteString("- @gitclaw /research catalog\n")
		if next := channelResearchSpotlightFollowup(report.SelectedCandidate); next != "" {
			fmt.Fprintf(&b, "- %s\n", next)
		}
	}
	b.WriteString("\nRaw research notes, source bodies, channel bodies, issue bodies, comment bodies, prompts, tool outputs, raw focus text, raw notes, and raw spotlight ids are not included in the source receipt. Source fetch: not performed by this action. Live source browse: not performed by this action. Model call: not performed by this action. Tool execution: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func writeChannelResearchSpotlightCandidate(b *strings.Builder, candidate channelResearchSpotlightCandidate) {
	switch candidate.Kind {
	case "source":
		fmt.Fprintf(b, "- kind=source source_id=%s system=%s source_kind=%s url=%s pattern=%q decision=%s\n",
			candidate.ID,
			candidate.System,
			candidate.SourceKind,
			candidate.URL,
			candidate.Pattern,
			candidate.Decision,
		)
	case "pattern":
		fmt.Fprintf(b, "- kind=pattern pattern=%s upstream=%q surface=%q status=%s gate=%s\n",
			candidate.Pattern,
			candidate.Upstream,
			candidate.Surface,
			candidate.Status,
			candidate.Gate,
		)
	case "rejection":
		fmt.Fprintf(b, "- kind=rejection surface=%s upstream=%q decision=%s gate=%s\n",
			candidate.Surface,
			candidate.Upstream,
			candidate.Decision,
			candidate.Gate,
		)
	default:
		fmt.Fprintf(b, "- kind=%s id=%s\n", candidate.Kind, channelResearchSpotlightCandidateID(candidate))
	}
}

func channelResearchSpotlightFollowup(candidate channelResearchSpotlightCandidate) string {
	switch candidate.Kind {
	case "source":
		if candidate.System != "" {
			return fmt.Sprintf("@gitclaw /channels research-spotlight %s --message-id <message> --notify-message-id <message>", candidate.System)
		}
	case "pattern":
		if candidate.Pattern != "" {
			return fmt.Sprintf("@gitclaw /channels research-spotlight %s --message-id <message> --notify-message-id <message>", candidate.Pattern)
		}
	case "rejection":
		if candidate.Surface != "" {
			return fmt.Sprintf("@gitclaw /channels research-spotlight %s --message-id <message> --notify-message-id <message>", candidate.Surface)
		}
	}
	return ""
}

func channelResearchSpotlightActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelResearchSpotlightActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelResearchSpotlightIssueTarget(ev Event, req *ChannelResearchSpotlightActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel research spotlight requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelResearchSpotlightIssueTargetIfPresent(ev Event, req *ChannelResearchSpotlightActionRequest) {
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

func applyChannelResearchSpotlightPositionals(req *ChannelResearchSpotlightActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	var values []string
	for _, value := range positional {
		value = cleanChannelResearchSpotlightFocus(value)
		if value == "" || value == "general" {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	if req.FocusSource != "" {
		return fmt.Errorf("unexpected channel research spotlight argument %q", values[0])
	}
	explicitTarget := req.TargetFromIssue || req.Options.Route != "" || req.Options.Channel != "" || req.Options.ThreadID != ""
	focusValues := values
	if !explicitTarget {
		req.Options.Route = values[0]
		focusValues = values[1:]
	}
	if len(focusValues) == 0 {
		return nil
	}
	joined := strings.Join(focusValues, " ")
	if req.Options.Focus == "" || req.Options.Focus == "general" {
		req.Options.Focus = joined
	} else {
		req.Options.Focus = strings.Join([]string{req.Options.Focus, joined}, " ")
	}
	req.FocusSource = "positional"
	return nil
}

func normalizeChannelResearchSpotlightOptions(opts ChannelResearchSpotlightOptions) ChannelResearchSpotlightOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SpotlightID = cleanChannelResearchSpotlightID(opts.SpotlightID)
	opts.Focus = cleanChannelResearchSpotlightFocus(opts.Focus)
	opts.Note = cleanChannelResearchSpotlightNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelResearchSpotlightRoute(cfg Config, opts ChannelResearchSpotlightOptions) (ChannelResearchSpotlightOptions, error) {
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
		Body:      "GitClaw channel research spotlight.",
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

func validateChannelResearchSpotlightOptions(opts ChannelResearchSpotlightOptions) error {
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
		return fmt.Errorf("missing research spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid research spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func validateChannelResearchSpotlightActionRequestOptions(opts ChannelResearchSpotlightOptions) error {
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
		return fmt.Errorf("missing research spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid research spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func cleanChannelResearchSpotlightSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelResearchSpotlightID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelResearchSpotlightFocus(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "general"
	}
	if len(value) > 160 {
		value = strings.TrimSpace(value[:160])
	}
	return value
}

func cleanChannelResearchSpotlightNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelResearchSpotlightTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelResearchSpotlightTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelResearchSpotlightNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelResearchSpotlightTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelResearchSpotlightFocus(subcommand string) string {
	switch cleanChannelResearchSpotlightSubcommand(subcommand) {
	case "openclaw-spotlight":
		return "openclaw"
	case "hermes-spotlight":
		return "hermes"
	case "pattern-spotlight", "pattern-draw":
		return "pattern"
	default:
		return "general"
	}
}

func autoChannelResearchSpotlightSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-research-spotlight-source-%s", eventID(ev))
}

func autoChannelResearchSpotlightID(ev Event, opts ChannelResearchSpotlightOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("research-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelResearchSpotlightNotifyMessageID(ev Event, spotlightID string) string {
	seed := strings.Join([]string{eventID(ev), spotlightID}, "|")
	return fmt.Sprintf("gitclaw-channel-research-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelResearchSpotlightAllCandidates() []channelResearchSpotlightCandidate {
	var candidates []channelResearchSpotlightCandidate
	for _, source := range researchSources() {
		candidates = append(candidates, channelResearchSpotlightCandidate{
			Kind:       "source",
			ID:         source.ID,
			System:     source.System,
			SourceKind: source.Kind,
			URL:        source.URL,
			Pattern:    source.Pattern,
			Decision:   source.Decision,
		})
	}
	for _, pattern := range researchPatterns() {
		candidates = append(candidates, channelResearchSpotlightCandidate{
			Kind:     "pattern",
			Pattern:  pattern.Name,
			Upstream: pattern.Upstream,
			Surface:  pattern.Surface,
			Status:   pattern.Status,
			Gate:     pattern.Gate,
		})
	}
	for _, rejection := range researchRejections() {
		candidates = append(candidates, channelResearchSpotlightCandidate{
			Kind:     "rejection",
			Upstream: rejection.Upstream,
			Surface:  rejection.Surface,
			Decision: rejection.Decision,
			Gate:     rejection.Gate,
		})
	}
	sortChannelResearchSpotlightCandidates(candidates)
	return candidates
}

func channelResearchSpotlightMatchingCandidates(focus string) []channelResearchSpotlightCandidate {
	focus = cleanChannelResearchSpotlightFocus(focus)
	if focus == "" || focus == "general" {
		return nil
	}
	terms := memorySearchTerms(focus)
	var out []channelResearchSpotlightCandidate
	for _, candidate := range channelResearchSpotlightAllCandidates() {
		score, _ := channelResearchSpotlightCandidateScore(candidate, focus, terms)
		if score == 0 {
			continue
		}
		out = append(out, candidate)
	}
	sortChannelResearchSpotlightCandidates(out)
	return out
}

func channelResearchSpotlightCandidates(focus string) []channelResearchSpotlightCandidate {
	if focus != "" && focus != "general" {
		return channelResearchSpotlightMatchingCandidates(focus)
	}
	return channelResearchSpotlightAllCandidates()
}

func channelResearchSpotlightCandidateScore(candidate channelResearchSpotlightCandidate, query string, terms []string) (int, []string) {
	fields := map[string]string{
		"kind":        candidate.Kind,
		"id":          candidate.ID,
		"system":      candidate.System,
		"source_kind": candidate.SourceKind,
		"url":         candidate.URL,
		"pattern":     candidate.Pattern,
		"decision":    candidate.Decision,
		"upstream":    candidate.Upstream,
		"surface":     candidate.Surface,
		"status":      candidate.Status,
		"gate":        candidate.Gate,
	}
	weights := map[string]int{
		"kind":        20,
		"id":          80,
		"system":      80,
		"source_kind": 40,
		"url":         25,
		"pattern":     70,
		"decision":    50,
		"upstream":    50,
		"surface":     65,
		"status":      25,
		"gate":        50,
	}
	return scoreSearchFields(fields, weights, query, terms)
}

func sortChannelResearchSpotlightCandidates(candidates []channelResearchSpotlightCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if channelResearchSpotlightCandidateID(left) != channelResearchSpotlightCandidateID(right) {
			return channelResearchSpotlightCandidateID(left) < channelResearchSpotlightCandidateID(right)
		}
		return left.Gate < right.Gate
	})
}

func channelResearchSpotlightCandidateID(candidate channelResearchSpotlightCandidate) string {
	if candidate.ID != "" {
		return candidate.ID
	}
	if candidate.Pattern != "" {
		return candidate.Pattern
	}
	if candidate.Surface != "" {
		return candidate.Surface
	}
	return candidate.Kind
}

func channelResearchSpotlightSeed(opts ChannelResearchSpotlightOptions, focus string) string {
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
		researchSnapshotDate,
	}, "|")
}

func deterministicChannelResearchSpotlightIndex(seed string, size int) int {
	if size <= 0 {
		return -1
	}
	sum := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint64(sum[:8]) % uint64(size))
}
