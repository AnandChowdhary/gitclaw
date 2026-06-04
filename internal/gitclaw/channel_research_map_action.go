package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelResearchMapOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MapID             string
	Focus             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelResearchMapReport struct {
	MapStatus                string
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
	StepCount                int
	StepHash                 string
	SelectionSeedSHA         string
	SelectionSHA             string
	RawBodiesIncluded        bool
	SourceFetchPerformed     bool
	LiveBrowsePerformed      bool
}

type ChannelResearchMapResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	MapIDHash           string
	FocusHash           string
	NoteHash            string
	SelectedKindHash    string
	SelectedIDHash      string
	SelectedSystemHash  string
	SelectedURLHash     string
	SelectedPatternHash string
	SelectedSurfaceHash string
	SelectedGateHash    string
	StepHash            string
	SelectionSeedHash   string
	SelectionHash       string
	Report              ChannelResearchMapReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelResearchMapActionRequest struct {
	Options             ChannelResearchMapOptions
	Report              ChannelResearchMapReport
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoMapID           bool
	TargetFromIssue     bool
	FocusSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	MapIDHash           string
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
	StepSHA             string
	StepCount           int
	SelectionSeedSHA    string
	SelectionSHA        string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type channelResearchMapStep struct {
	Command string
	Reason  string
}

func IsChannelResearchMapActionRequest(ev Event, cfg Config) bool {
	return isChannelResearchMapActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelResearchMapActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelResearchMapSubcommand(fields[1]) {
	case "research-map", "research-path", "research-runbook", "research-bridge", "landscape-map", "landscape-path", "openclaw-map", "hermes-map", "pattern-map":
		return true
	default:
		return false
	}
}

func BuildChannelResearchMapActionRequest(ev Event, cfg Config) (ChannelResearchMapActionRequest, error) {
	fields, trailing, ok := channelResearchMapActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelResearchMapActionRequest{}, fmt.Errorf("missing channel research map command")
	}
	req := ChannelResearchMapActionRequest{
		Options: ChannelResearchMapOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             defaultChannelResearchMapFocus(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelResearchMapSubcommand(fields[1]),
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
				return ChannelResearchMapActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--map-id", "--research-map-id", "--research-bridge-id", "--bridge-id", "--runbook-id", "--id":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MapID = cleanChannelResearchMapID(fields[i+1])
			i++
		case "--focus", "--query", "--for", "--system", "--topic":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelResearchMapActionRequest{}, fmt.Errorf("unknown channel research map argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelResearchMapIssueTargetIfPresent(ev, &req)
	if err := applyChannelResearchMapPositionals(&req, positional); err != nil {
		return ChannelResearchMapActionRequest{}, err
	}
	if err := applyChannelResearchMapIssueTarget(ev, &req); err != nil {
		return ChannelResearchMapActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelResearchMapTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelResearchMapSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MapID) == "" {
		req.Options.MapID = autoChannelResearchMapID(ev, req.Options)
		req.AutoMapID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelResearchMapNotifyMessageID(ev, req.Options.MapID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelResearchMapOptions(req.Options)
	if err := validateChannelResearchMapActionRequestOptions(req.Options); err != nil {
		return ChannelResearchMapActionRequest{}, err
	}
	req.Report = BuildChannelResearchMapReport(cfg, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MapIDHash = shortDocumentHash(req.Options.MapID)
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
	req.StepSHA = req.Report.StepHash
	req.StepCount = req.Report.StepCount
	req.SelectionSeedSHA = req.Report.SelectionSeedSHA
	req.SelectionSHA = req.Report.SelectionSHA
	notificationBody := renderChannelResearchMapNotificationBody(req.Options, req.Report)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelResearchMapReport(cfg Config, opts ChannelResearchMapOptions) ChannelResearchMapReport {
	focus := cleanChannelResearchMapFocus(opts.Focus)
	surface := inspectResearchSurface(cfg.Workdir)
	report := ChannelResearchMapReport{
		MapStatus:                "ok",
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
				report.MapStatus = "fallback"
			}
		}
	}
	if report.MatchedItems == 0 && (focus == "" || focus == "general") {
		report.MatchedItems = len(candidates)
	}
	report.CandidateItems = len(candidates)
	seed := channelResearchMapSeed(opts, focus)
	report.SelectionSeedSHA = shortDocumentHash(seed)
	if len(candidates) == 0 {
		report.MapStatus = "no_research_items"
		report.SelectedIndex = -1
		report.SelectionSHA = "none"
		report.StepHash = "none"
		return report
	}
	idx := deterministicChannelResearchSpotlightIndex(seed, len(candidates))
	selected := candidates[idx]
	steps := channelResearchMapSteps(focus, selected)
	report.SelectedIndex = idx
	report.SelectedCandidate = selected
	report.StepCount = len(steps)
	report.StepHash = shortDocumentHash(renderChannelResearchMapStepFingerprint(steps))
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
		report.StepHash,
	}, "|"))
	return report
}

func RunChannelResearchMap(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelResearchMapActionRequest) (ChannelResearchMapResult, error) {
	opts := normalizeChannelResearchMapOptions(req.Options)
	var err error
	opts, err = applyChannelResearchMapRoute(cfg, opts)
	if err != nil {
		return ChannelResearchMapResult{}, err
	}
	if err := validateChannelResearchMapOptions(opts); err != nil {
		return ChannelResearchMapResult{}, err
	}
	report := req.Report
	if report.MapStatus == "" {
		report = BuildChannelResearchMapReport(cfg, opts)
	}
	body := renderChannelResearchMapNotificationBody(opts, report)
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
		return ChannelResearchMapResult{}, fmt.Errorf("queue channel research map notification: %w", err)
	}
	selected := report.SelectedCandidate
	return ChannelResearchMapResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		MapIDHash:           shortDocumentHash(opts.MapID),
		FocusHash:           report.FocusHash,
		NoteHash:            shortDocumentHash(opts.Note),
		SelectedKindHash:    shortDocumentHash(selected.Kind),
		SelectedIDHash:      shortDocumentHash(channelResearchSpotlightCandidateID(selected)),
		SelectedSystemHash:  shortDocumentHash(selected.System),
		SelectedURLHash:     shortDocumentHash(selected.URL),
		SelectedPatternHash: shortDocumentHash(selected.Pattern),
		SelectedSurfaceHash: shortDocumentHash(selected.Surface),
		SelectedGateHash:    shortDocumentHash(selected.Gate),
		StepHash:            report.StepHash,
		SelectionSeedHash:   report.SelectionSeedSHA,
		SelectionHash:       report.SelectionSHA,
		Report:              report,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelResearchMapActionReport(ev Event, req ChannelResearchMapActionRequest, result ChannelResearchMapResult) string {
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
	mapIDHash := firstNonEmpty(result.MapIDHash, req.MapIDHash)
	focusHash := firstNonEmpty(result.FocusHash, req.FocusSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	selectedKindHash := firstNonEmpty(result.SelectedKindHash, req.SelectedKindHash)
	selectedIDHash := firstNonEmpty(result.SelectedIDHash, req.SelectedIDHash)
	selectedSystemHash := firstNonEmpty(result.SelectedSystemHash, req.SelectedSystemHash)
	selectedURLHash := firstNonEmpty(result.SelectedURLHash, req.SelectedURLHash)
	selectedPatternHash := firstNonEmpty(result.SelectedPatternHash, req.SelectedPatternHash)
	selectedSurfaceHash := firstNonEmpty(result.SelectedSurfaceHash, req.SelectedSurfaceHash)
	selectedGateHash := firstNonEmpty(result.SelectedGateHash, req.SelectedGateHash)
	stepHash := firstNonEmpty(result.StepHash, req.StepSHA)
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
	if report.MapStatus == "" {
		report = req.Report
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Research Map Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_research_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- research_map_status: `%s`\n", report.MapStatus)
	fmt.Fprintf(&b, "- research_map_mode: `%s`\n", "static-research-catalog-safety-sequence")
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
	fmt.Fprintf(&b, "- research_map_id_sha256_12: `%s`\n", noneIfEmpty(mapIDHash))
	fmt.Fprintf(&b, "- research_map_id_auto: `%t`\n", req.AutoMapID)
	fmt.Fprintf(&b, "- research_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- research_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- research_focus_terms: `%d`\n", report.FocusTerms)
	fmt.Fprintf(&b, "- research_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- research_map_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- research_map_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- research_map_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- research_map_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
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
	fmt.Fprintf(&b, "- research_map_step_count: `%d`\n", report.StepCount)
	fmt.Fprintf(&b, "- research_map_step_sha256_12: `%s`\n", noneIfEmpty(stepHash))
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
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_map_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selection_seed_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_source_ids_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_source_urls_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_patterns_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_surfaces_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_map_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_research_map_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing research map from reviewed static catalog metadata. The provider card may name one source, pattern, or rejected surface and show safe next commands, while this source receipt keeps raw ids, URLs, surfaces, pattern text, focus text, notes, step text, channel bodies, and research bodies out of band. The action does not call a model, execute tools, fetch sources, browse live sources, mutate workflows, mutate repository files, use external randomness, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read research-map cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent research-map cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate research-map cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelResearchMapNotificationBody(opts ChannelResearchMapOptions, report ChannelResearchMapReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel research map\n\n")
	fmt.Fprintf(&b, "Map status: %s\n", report.MapStatus)
	fmt.Fprintf(&b, "Source snapshot date: %s\n", report.SourceSnapshotDate)
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Reviewed sources: %d\n", report.ReviewedSources)
	fmt.Fprintf(&b, "Pattern coverage: %d\n", report.PatternCoverage)
	fmt.Fprintf(&b, "Rejected patterns: %d\n", report.RejectedPatterns)
	fmt.Fprintf(&b, "Matched items: %d\n", report.MatchedItems)
	fmt.Fprintf(&b, "Candidate items: %d\n", report.CandidateItems)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Step count: %d\n", report.StepCount)
	fmt.Fprintf(&b, "Step hash: %s\n", report.StepHash)
	fmt.Fprintf(&b, "Selection seed hash: %s\n", report.SelectionSeedSHA)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Research map id hash: %s\n", shortDocumentHash(opts.MapID))
	b.WriteString("\nSelected research item:\n")
	if report.MapStatus == "no_research_items" || strings.TrimSpace(report.SelectedCandidate.Kind) == "" {
		b.WriteString("- none\n")
	} else {
		writeChannelResearchSpotlightCandidate(&b, report.SelectedCandidate)
	}
	b.WriteString("\nResearch sequence:\n")
	for i, step := range channelResearchMapSteps(cleanChannelResearchMapFocus(opts.Focus), report.SelectedCandidate) {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, step.Command, step.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nNote hash: %s\n", shortDocumentHash(opts.Note))
	fmt.Fprintf(&b, "Map source: reviewed static research catalog snapshot %s.\n", report.SourceSnapshotDate)
	b.WriteString("\nRaw research notes, source bodies, channel bodies, issue bodies, comment bodies, prompts, tool outputs, raw focus text, raw notes, raw map ids, and raw step text are not included in the source receipt. Source fetch: not performed by this action. Live source browse: not performed by this action. Model call: not performed by this action. Tool execution: not performed by this action. Provider API call: not performed by this action. Workflow mutation: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelResearchMapSteps(focus string, candidate channelResearchSpotlightCandidate) []channelResearchMapStep {
	target := cleanChannelResearchMapStepTarget(focus, candidate)
	if target == "" {
		target = "research"
	}
	steps := []channelResearchMapStep{
		{Command: "@gitclaw /research catalog", Reason: "review the body-free source and pattern catalog"},
		{Command: fmt.Sprintf("@gitclaw /channels research-spotlight %s --message-id <id> --notify-message-id <id>", target), Reason: "draw a compact source or pattern card for the thread"},
	}
	if domain := channelResearchMapDomainStep(candidate, target); domain.Command != "" {
		steps = append(steps, domain)
	}
	steps = append(steps,
		channelResearchMapStep{Command: fmt.Sprintf("@gitclaw /channels compass %s --compass-id <id> --message-id <id> --notify-message-id <id>", target), Reason: "turn the selected pattern into safe next-step orientation"},
		channelResearchMapStep{Command: "@gitclaw /channels coach research --coach-id <id> --message-id <id> --notify-message-id <id>", Reason: "ask for a repo-aware next move without executing commands"},
		channelResearchMapStep{Command: "@gitclaw /channels palette research --palette-id <id> --message-id <id> --notify-message-id <id>", Reason: "show a small command palette for continued exploration"},
	)
	return steps
}

func channelResearchMapDomainStep(candidate channelResearchSpotlightCandidate, target string) channelResearchMapStep {
	text := strings.ToLower(strings.Join([]string{
		candidate.Pattern,
		candidate.Surface,
		candidate.Decision,
		candidate.Gate,
		candidate.Upstream,
		candidate.System,
	}, " "))
	switch {
	case strings.Contains(text, "skill"):
		return channelResearchMapStep{Command: fmt.Sprintf("@gitclaw /channels skill-spotlight %s --spotlight-id <id> --message-id <id> --notify-message-id <id>", target), Reason: "inspect the related skill surface without installing skills"}
	case strings.Contains(text, "tool"):
		return channelResearchMapStep{Command: fmt.Sprintf("@gitclaw /channels tool-spotlight %s --spotlight-id <id> --message-id <id> --notify-message-id <id>", target), Reason: "inspect the related tool surface without executing tools"}
	case strings.Contains(text, "soul") || strings.Contains(text, "profile"):
		return channelResearchMapStep{Command: fmt.Sprintf("@gitclaw /channels soul-spotlight %s --spotlight-id <id> --message-id <id> --notify-message-id <id>", target), Reason: "inspect high-authority context without printing raw bodies"}
	case strings.Contains(text, "memory"):
		return channelResearchMapStep{Command: "@gitclaw /channels memory-status --message-id <id> --notify-message-id <id>", Reason: "inspect durable-memory state without writing memory"}
	case strings.Contains(text, "backup") || strings.Contains(text, "durability"):
		return channelResearchMapStep{Command: fmt.Sprintf("@gitclaw /channels backup-spotlight %s --spotlight-id <id> --message-id <id> --notify-message-id <id>", target), Reason: "inspect durable backup metadata without restoring files"}
	case strings.Contains(text, "checkpoint") || strings.Contains(text, "rollback"):
		return channelResearchMapStep{Command: "@gitclaw /channels checkpoint-status --message-id <id> --notify-message-id <id>", Reason: "inspect rollback readiness without reset, clean, checkout, or restore"}
	case strings.Contains(text, "model"):
		return channelResearchMapStep{Command: "@gitclaw /channels model --message-id <id> --notify-message-id <id>", Reason: "inspect model/provider status without switching models"}
	case strings.Contains(text, "channel") || strings.Contains(text, "gateway"):
		return channelResearchMapStep{Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>", Reason: "inspect channel availability without sockets or provider APIs"}
	default:
		return channelResearchMapStep{Command: "@gitclaw /channels status --message-id <id> --notify-message-id <id>", Reason: "check the mirrored channel thread state before acting"}
	}
}

func channelResearchMapStepTarget(focus string, candidate channelResearchSpotlightCandidate) string {
	focus = cleanChannelResearchMapFocus(focus)
	if focus != "" && focus != "general" {
		return focus
	}
	return cleanChannelResearchMapStepTarget("", candidate)
}

func cleanChannelResearchMapStepTarget(focus string, candidate channelResearchSpotlightCandidate) string {
	if focus = cleanChannelResearchMapFocus(focus); focus != "" && focus != "general" {
		return focus
	}
	for _, value := range []string{candidate.Pattern, candidate.System, candidate.Surface, candidate.Decision, candidate.Gate, candidate.Kind} {
		value = cleanChannelResearchMapFocus(value)
		if value != "" && value != "general" {
			return value
		}
	}
	return "research"
}

func renderChannelResearchMapStepFingerprint(steps []channelResearchMapStep) string {
	var parts []string
	for _, step := range steps {
		parts = append(parts, step.Command+"|"+step.Reason)
	}
	return strings.Join(parts, "\n")
}

func channelResearchMapActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelResearchMapActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelResearchMapIssueTarget(ev Event, req *ChannelResearchMapActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel research map requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelResearchMapIssueTargetIfPresent(ev Event, req *ChannelResearchMapActionRequest) {
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

func applyChannelResearchMapPositionals(req *ChannelResearchMapActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	var values []string
	for _, value := range positional {
		value = cleanChannelResearchMapFocus(value)
		if value == "" || value == "general" {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	if req.FocusSource != "" {
		return fmt.Errorf("unexpected channel research map argument %q", values[0])
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

func normalizeChannelResearchMapOptions(opts ChannelResearchMapOptions) ChannelResearchMapOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MapID = cleanChannelResearchMapID(opts.MapID)
	opts.Focus = cleanChannelResearchMapFocus(opts.Focus)
	opts.Note = cleanChannelResearchMapNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelResearchMapRoute(cfg Config, opts ChannelResearchMapOptions) (ChannelResearchMapOptions, error) {
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
		Body:      "GitClaw channel research map.",
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

func validateChannelResearchMapOptions(opts ChannelResearchMapOptions) error {
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
		return fmt.Errorf("missing research map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid research map id %q", opts.MapID)
	}
	return nil
}

func validateChannelResearchMapActionRequestOptions(opts ChannelResearchMapOptions) error {
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
		return fmt.Errorf("missing research map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid research map id %q", opts.MapID)
	}
	return nil
}

func cleanChannelResearchMapSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelResearchMapID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelResearchMapFocus(value string) string {
	return cleanChannelResearchSpotlightFocus(value)
}

func cleanChannelResearchMapNote(value string) string {
	return cleanChannelResearchSpotlightNote(value)
}

func parseChannelResearchMapTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelResearchMapTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelResearchMapNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelResearchMapTrailingLine(line string) bool {
	return shouldSkipChannelResearchSpotlightTrailingLine(line)
}

func defaultChannelResearchMapFocus(subcommand string) string {
	switch cleanChannelResearchMapSubcommand(subcommand) {
	case "openclaw-map":
		return "openclaw"
	case "hermes-map":
		return "hermes"
	case "pattern-map":
		return "pattern"
	default:
		return "general"
	}
}

func autoChannelResearchMapSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-research-map-source-%s", eventID(ev))
}

func autoChannelResearchMapID(ev Event, opts ChannelResearchMapOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("research-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelResearchMapNotifyMessageID(ev Event, mapID string) string {
	seed := strings.Join([]string{eventID(ev), mapID}, "|")
	return fmt.Sprintf("gitclaw-channel-research-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelResearchMapSeed(opts ChannelResearchMapOptions, focus string) string {
	return strings.Join([]string{
		opts.Repo,
		opts.Route,
		opts.Channel,
		opts.ThreadID,
		opts.SourceMessageID,
		opts.NotifyMessageID,
		opts.MapID,
		focus,
		opts.Note,
		researchSnapshotDate,
	}, "|")
}
