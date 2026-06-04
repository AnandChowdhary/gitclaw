package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

type ChannelToolSpotlightOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	SpotlightID       string
	Mode              string
	Focus             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolSpotlightReport struct {
	SpotlightStatus  string
	FocusHash        string
	FocusTerms       int
	AvailableTools   int
	EnabledTools     int
	EligibleTools    int
	MatchedTools     int
	CandidateTools   int
	ActiveOutputs    int
	SelectedIndex    int
	SelectedTool     toolContract
	SelectedEnabled  bool
	SelectedDisabled bool
	SelectedBlocked  bool
	SelectionSeedSHA string
	SelectionSHA     string
	ValidationStatus string
	ValidationErrors int
	ValidationWarns  int
}

type ChannelToolSpotlightResult struct {
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
	SelectedNameHash    string
	SelectedModeHash    string
	SelectedTriggerHash string
	SelectionSeedHash   string
	SelectionHash       string
	Report              ChannelToolSpotlightReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelToolSpotlightActionRequest struct {
	Options             ChannelToolSpotlightOptions
	Report              ChannelToolSpotlightReport
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
	ModeHash            string
	ModeBytes           int
	FocusSHA            string
	FocusBytes          int
	FocusTerms          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	SelectedNameHash    string
	SelectedModeHash    string
	SelectedTriggerHash string
	SelectionSeedSHA    string
	SelectionSHA        string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelToolSpotlightActionRequest(ev Event, cfg Config) bool {
	return isChannelToolSpotlightActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolSpotlightActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelToolSpotlightSubcommand(fields[1]) {
	case "tool-spotlight", "tools-spotlight", "spotlight-tool", "tool-pick", "tool-draw", "tool-capability-spotlight", "tool-capability-draw", "tool-drill", "tools-drill", "drill-tool", "tool-warmup", "tool-contract-drill", "capability-drill":
		return true
	default:
		return false
	}
}

func BuildChannelToolSpotlightActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolSpotlightActionRequest, error) {
	fields, trailing, ok := channelToolSpotlightActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolSpotlightActionRequest{}, fmt.Errorf("missing channel tool spotlight command")
	}
	req := ChannelToolSpotlightActionRequest{
		Options: ChannelToolSpotlightOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Mode:              channelToolSpotlightModeForSubcommand(fields[1]),
			Focus:             defaultChannelToolSpotlightFocus(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelToolSpotlightSubcommand(fields[1]),
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
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--spotlight-id", "--tool-spotlight-id", "--tool-pick-id", "--tool-draw-id", "--drill-id", "--tool-drill-id", "--warmup-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SpotlightID = cleanChannelToolSpotlightID(fields[i+1])
			i++
		case "--focus", "--tool", "--query", "--for":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolSpotlightActionRequest{}, fmt.Errorf("unknown channel tool spotlight argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelToolSpotlightIssueTargetIfPresent(ev, &req)
	if err := applyChannelToolSpotlightPositionals(&req, positional); err != nil {
		return ChannelToolSpotlightActionRequest{}, err
	}
	if err := applyChannelToolSpotlightIssueTarget(ev, &req); err != nil {
		return ChannelToolSpotlightActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelToolSpotlightTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelToolSpotlightSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SpotlightID) == "" {
		req.Options.SpotlightID = autoChannelToolSpotlightID(ev, req.Options)
		req.AutoSpotlightID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolSpotlightNotifyMessageID(ev, req.Options.SpotlightID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolSpotlightOptions(req.Options)
	if err := validateChannelToolSpotlightActionRequestOptions(req.Options); err != nil {
		return ChannelToolSpotlightActionRequest{}, err
	}
	req.Report = BuildChannelToolSpotlightReport(repoContext, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.SpotlightIDHash = shortDocumentHash(req.Options.SpotlightID)
	req.ModeHash = shortDocumentHash(req.Options.Mode)
	req.ModeBytes = len(req.Options.Mode)
	req.FocusSHA = req.Report.FocusHash
	req.FocusBytes = len(req.Options.Focus)
	req.FocusTerms = req.Report.FocusTerms
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.SelectedNameHash = shortDocumentHash(req.Report.SelectedTool.Name)
	req.SelectedModeHash = shortDocumentHash(req.Report.SelectedTool.Mode)
	req.SelectedTriggerHash = shortDocumentHash(req.Report.SelectedTool.Trigger)
	req.SelectionSeedSHA = req.Report.SelectionSeedSHA
	req.SelectionSHA = req.Report.SelectionSHA
	notificationBody := renderChannelToolSpotlightNotificationBody(req.Options, req.Report)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelToolSpotlightReport(repoContext RepoContext, opts ChannelToolSpotlightOptions) ChannelToolSpotlightReport {
	focus := cleanChannelToolSpotlightFocus(opts.Focus)
	validation := ValidateTools(repoContext)
	report := ChannelToolSpotlightReport{
		SpotlightStatus:  "ok",
		FocusHash:        shortDocumentHash(focus),
		FocusTerms:       len(memorySearchTerms(focus)),
		AvailableTools:   len(toolReportContracts),
		EnabledTools:     enabledToolCount(repoContext),
		EligibleTools:    channelToolSpotlightEligibleCount(repoContext),
		ActiveOutputs:    len(repoContext.ToolOutputs),
		ValidationStatus: validation.Status,
		ValidationErrors: validation.Errors,
		ValidationWarns:  validation.Warnings,
	}
	candidates := channelToolSpotlightCandidates(repoContext, focus)
	if focus != "" && focus != "general" {
		report.MatchedTools = len(channelToolSpotlightMatchingContracts(repoContext, focus))
		if len(candidates) == 0 {
			candidates = channelToolSpotlightCandidates(repoContext, "")
			if len(candidates) > 0 {
				report.SpotlightStatus = "fallback"
			}
		}
	}
	if report.MatchedTools == 0 && (focus == "" || focus == "general") {
		report.MatchedTools = len(candidates)
	}
	report.CandidateTools = len(candidates)
	seed := channelToolSpotlightSeed(opts, focus)
	report.SelectionSeedSHA = shortDocumentHash(seed)
	if len(candidates) == 0 {
		report.SpotlightStatus = "no_eligible_tools"
		report.SelectedIndex = -1
		report.SelectionSHA = "none"
		return report
	}
	idx := deterministicChannelToolSpotlightIndex(seed, len(candidates))
	selected := candidates[idx]
	enabled, disabled, blocked := toolEnabledInRepoContext(selected.Name, repoContext)
	report.SelectedIndex = idx
	report.SelectedTool = selected
	report.SelectedEnabled = enabled
	report.SelectedDisabled = disabled
	report.SelectedBlocked = blocked
	report.SelectionSHA = shortDocumentHash(strings.Join([]string{
		selected.Name,
		selected.Mode,
		selected.Trigger,
		focus,
		fmt.Sprintf("%d", idx),
	}, "|"))
	return report
}

func RunChannelToolSpotlight(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelToolSpotlightActionRequest, repoContext RepoContext) (ChannelToolSpotlightResult, error) {
	opts := normalizeChannelToolSpotlightOptions(req.Options)
	var err error
	opts, err = applyChannelToolSpotlightRoute(cfg, opts)
	if err != nil {
		return ChannelToolSpotlightResult{}, err
	}
	if err := validateChannelToolSpotlightOptions(opts); err != nil {
		return ChannelToolSpotlightResult{}, err
	}
	report := req.Report
	if report.SpotlightStatus == "" {
		report = BuildChannelToolSpotlightReport(repoContext, opts)
	}
	body := renderChannelToolSpotlightNotificationBody(opts, report)
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
		return ChannelToolSpotlightResult{}, fmt.Errorf("queue channel tool spotlight notification: %w", err)
	}
	return ChannelToolSpotlightResult{
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
		SelectedNameHash:    shortDocumentHash(report.SelectedTool.Name),
		SelectedModeHash:    shortDocumentHash(report.SelectedTool.Mode),
		SelectedTriggerHash: shortDocumentHash(report.SelectedTool.Trigger),
		SelectionSeedHash:   report.SelectionSeedSHA,
		SelectionHash:       report.SelectionSHA,
		Report:              report,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelToolSpotlightActionReport(ev Event, req ChannelToolSpotlightActionRequest, result ChannelToolSpotlightResult) string {
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
	selectedNameHash := firstNonEmpty(result.SelectedNameHash, req.SelectedNameHash)
	selectedModeHash := firstNonEmpty(result.SelectedModeHash, req.SelectedModeHash)
	selectedTriggerHash := firstNonEmpty(result.SelectedTriggerHash, req.SelectedTriggerHash)
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
	if channelToolSpotlightMode(req.Options.Mode) == "drill" {
		b.WriteString("## GitClaw Channel Tool Drill Action\n\n")
	} else {
		b.WriteString("## GitClaw Channel Tool Spotlight Action\n\n")
	}
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_spotlight_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_spotlight_status: `%s`\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "- spotlight_mode: `%s`\n", "deterministic-tool-contract-draw")
	fmt.Fprintf(&b, "- tool_card_mode: `%s`\n", channelToolSpotlightReportMode(req.Options.Mode))
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
	fmt.Fprintf(&b, "- tool_spotlight_id_sha256_12: `%s`\n", noneIfEmpty(spotlightIDHash))
	fmt.Fprintf(&b, "- tool_spotlight_id_auto: `%t`\n", req.AutoSpotlightID)
	fmt.Fprintf(&b, "- tool_card_mode_sha256_12: `%s`\n", noneIfEmpty(req.ModeHash))
	fmt.Fprintf(&b, "- tool_card_mode_bytes: `%d`\n", req.ModeBytes)
	fmt.Fprintf(&b, "- spotlight_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- spotlight_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- spotlight_focus_terms: `%d`\n", report.FocusTerms)
	fmt.Fprintf(&b, "- spotlight_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- spotlight_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- spotlight_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- spotlight_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- spotlight_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", report.EnabledTools)
	fmt.Fprintf(&b, "- eligible_tools: `%d`\n", report.EligibleTools)
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", report.MatchedTools)
	fmt.Fprintf(&b, "- candidate_tools: `%d`\n", report.CandidateTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "- selected_index: `%d`\n", report.SelectedIndex)
	fmt.Fprintf(&b, "- selected_tool_name_sha256_12: `%s`\n", noneIfEmpty(selectedNameHash))
	fmt.Fprintf(&b, "- selected_tool_mode_sha256_12: `%s`\n", noneIfEmpty(selectedModeHash))
	fmt.Fprintf(&b, "- selected_tool_trigger_sha256_12: `%s`\n", noneIfEmpty(selectedTriggerHash))
	fmt.Fprintf(&b, "- selected_tool_enabled: `%t`\n", report.SelectedEnabled)
	fmt.Fprintf(&b, "- selected_tool_disabled_by_config: `%t`\n", report.SelectedDisabled)
	fmt.Fprintf(&b, "- selected_tool_blocked_by_allowlist: `%t`\n", report.SelectedBlocked)
	fmt.Fprintf(&b, "- selection_seed_sha256_12: `%s`\n", noneIfEmpty(selectionSeedHash))
	fmt.Fprintf(&b, "- selection_sha256_12: `%s`\n", noneIfEmpty(selectionHash))
	fmt.Fprintf(&b, "- validation_status: `%s`\n", report.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", report.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", report.ValidationWarns)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- drill_step_count: `%d`\n", channelToolSpotlightDrillStepCount(req.Options.Mode, report))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- deterministic_selection: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_server_launch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- toolset_activation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_spotlight_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_card_mode_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selection_seed_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_triggers_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_spotlight_change: `%t`\n", true)
	if channelToolSpotlightMode(req.Options.Mode) == "drill" {
		fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_drill_change: `%t`\n", true)
	}
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	if channelToolSpotlightMode(req.Options.Mode) == "drill" {
		b.WriteString("GitClaw queued a provider-facing tool drill card from deterministic tool contract metadata. The provider card may name one safe tool and show bounded practice prompts, while the source receipt keeps raw tool names, triggers, schemas, inputs, outputs, ids, focus text, notes, and channel bodies out of band. The action does not call a model, execute tools, run shells, launch MCP servers, activate toolsets, mutate repository files, use external randomness, or call provider APIs.\n\n")
	} else {
		b.WriteString("GitClaw queued a provider-facing tool spotlight card from deterministic tool contract metadata. The provider card may name one safe tool so people can act on it, while the source receipt keeps raw tool names, triggers, schemas, inputs, outputs, ids, focus text, notes, and channel bodies out of band. The action does not call a model, execute tools, run shells, launch MCP servers, activate toolsets, mutate repository files, use external randomness, or call provider APIs.\n\n")
	}
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read tool cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool card notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelToolSpotlightNotificationBody(opts ChannelToolSpotlightOptions, report ChannelToolSpotlightReport) string {
	if channelToolSpotlightMode(opts.Mode) == "drill" {
		return renderChannelToolDrillNotificationBody(opts, report)
	}
	var b strings.Builder
	b.WriteString("GitClaw channel tool spotlight\n\n")
	fmt.Fprintf(&b, "Spotlight status: %s\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Available tools: %d\n", report.AvailableTools)
	fmt.Fprintf(&b, "Enabled tools: %d\n", report.EnabledTools)
	fmt.Fprintf(&b, "Eligible tools: %d\n", report.EligibleTools)
	fmt.Fprintf(&b, "Matched tools: %d\n", report.MatchedTools)
	fmt.Fprintf(&b, "Candidate tools: %d\n", report.CandidateTools)
	fmt.Fprintf(&b, "Active tool outputs: %d\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Selection seed hash: %s\n", report.SelectionSeedSHA)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarns)
	fmt.Fprintf(&b, "Tool spotlight id hash: %s\n", shortDocumentHash(opts.SpotlightID))
	b.WriteString("\nSpotlight:\n")
	if report.SpotlightStatus == "no_eligible_tools" || strings.TrimSpace(report.SelectedTool.Name) == "" {
		b.WriteString("- none\n")
	} else {
		tool := report.SelectedTool
		fmt.Fprintf(&b, "- tool_name=%s mode=%s enabled=%t disabled_by_config=%t blocked_by_allowlist=%t mutating=%t trigger_sha256_12=%s\n",
			tool.Name,
			tool.Mode,
			report.SelectedEnabled,
			report.SelectedDisabled,
			report.SelectedBlocked,
			isMutatingToolContract(tool),
			shortDocumentHash(tool.Trigger),
		)
		b.WriteString("\nTry next:\n")
		fmt.Fprintf(&b, "- @gitclaw /channels tool-info %s --message-id <message> --notify-message-id <message>\n", tool.Name)
		fmt.Fprintf(&b, "- @gitclaw /channels tool-map %s --map-id <id> --message-id <message> --notify-message-id <message>\n", tool.Name)
	}
	b.WriteString("\nRaw tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, raw focus text, raw notes, raw tool triggers, and raw spotlight ids are not included in the source receipt. Tool execution: not performed by this action. Shell execution: not performed by this action. MCP server launch: not performed by this action. Toolset activation: not performed by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func renderChannelToolDrillNotificationBody(opts ChannelToolSpotlightOptions, report ChannelToolSpotlightReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool drill\n\n")
	fmt.Fprintf(&b, "Drill status: %s\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Available tools: %d\n", report.AvailableTools)
	fmt.Fprintf(&b, "Enabled tools: %d\n", report.EnabledTools)
	fmt.Fprintf(&b, "Candidate tools: %d\n", report.CandidateTools)
	fmt.Fprintf(&b, "Active tool outputs: %d\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Tool drill id hash: %s\n", shortDocumentHash(opts.SpotlightID))
	b.WriteString("\nDrill:\n")
	if report.SpotlightStatus == "no_eligible_tools" || strings.TrimSpace(report.SelectedTool.Name) == "" {
		b.WriteString("- No eligible read-only tool contract was available for this drill.\n")
	} else {
		tool := report.SelectedTool
		fmt.Fprintf(&b, "- tool_name=%s mode=%s enabled=%t disabled_by_config=%t blocked_by_allowlist=%t mutating=%t trigger_sha256_12=%s\n",
			tool.Name,
			tool.Mode,
			report.SelectedEnabled,
			report.SelectedDisabled,
			report.SelectedBlocked,
			isMutatingToolContract(tool),
			shortDocumentHash(tool.Trigger),
		)
		b.WriteString("- inspect: name the safest question this tool can answer.\n")
		b.WriteString("- practice: ask GitClaw a normal repo question that should use this tool.\n")
		b.WriteString("- verify: check the assistant marker for prompt-visible tools and tool output count.\n")
		b.WriteString("- next: open a tool rehearsal issue if the answer needs more than one turn.\n")
		b.WriteString("\nTry next:\n")
		fmt.Fprintf(&b, "- @gitclaw /channels tool-info %s --message-id <message> --notify-message-id <message>\n", tool.Name)
		fmt.Fprintf(&b, "- @gitclaw /channels rehearse-tool %s --id <id> --message-id <message>\n", tool.Name)
	}
	b.WriteString("\nRaw tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, raw focus text, raw notes, raw tool triggers, and raw drill ids are not included in the source receipt. Tool execution: not performed by this action. Shell execution: not performed by this action. MCP server launch: not performed by this action. Toolset activation: not performed by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelToolSpotlightActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolSpotlightActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolSpotlightIssueTarget(ev Event, req *ChannelToolSpotlightActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool spotlight requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelToolSpotlightIssueTargetIfPresent(ev Event, req *ChannelToolSpotlightActionRequest) {
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

func applyChannelToolSpotlightPositionals(req *ChannelToolSpotlightActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel tool spotlight argument %q", value)
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
		return fmt.Errorf("unexpected channel tool spotlight argument %q", value)
	}
	return nil
}

func normalizeChannelToolSpotlightOptions(opts ChannelToolSpotlightOptions) ChannelToolSpotlightOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SpotlightID = cleanChannelToolSpotlightID(opts.SpotlightID)
	opts.Mode = channelToolSpotlightMode(opts.Mode)
	opts.Focus = cleanChannelToolSpotlightFocus(opts.Focus)
	opts.Note = cleanChannelToolSpotlightNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToolSpotlightRoute(cfg Config, opts ChannelToolSpotlightOptions) (ChannelToolSpotlightOptions, error) {
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
		Body:      "GitClaw channel tool spotlight.",
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

func validateChannelToolSpotlightOptions(opts ChannelToolSpotlightOptions) error {
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
		return fmt.Errorf("missing tool spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid tool spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func validateChannelToolSpotlightActionRequestOptions(opts ChannelToolSpotlightOptions) error {
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
		return fmt.Errorf("missing tool spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid tool spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func cleanChannelToolSpotlightSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func channelToolSpotlightModeForSubcommand(subcommand string) string {
	switch cleanChannelToolSpotlightSubcommand(subcommand) {
	case "tool-drill", "tools-drill", "drill-tool", "tool-warmup", "tool-contract-drill", "capability-drill":
		return "drill"
	default:
		return "spotlight"
	}
}

func channelToolSpotlightMode(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "drill", "tool-drill", "tools-drill", "tool-warmup", "tool-contract-drill", "capability-drill":
		return "drill"
	default:
		return "spotlight"
	}
}

func channelToolSpotlightReportMode(value string) string {
	if channelToolSpotlightMode(value) == "drill" {
		return "deterministic-tool-contract-drill"
	}
	return "deterministic-tool-contract-spotlight"
}

func channelToolSpotlightDrillStepCount(mode string, report ChannelToolSpotlightReport) int {
	if channelToolSpotlightMode(mode) != "drill" || report.SpotlightStatus == "no_eligible_tools" {
		return 0
	}
	return 4
}

func cleanChannelToolSpotlightID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelToolSpotlightFocus(value string) string {
	value = cleanChannelToolSearchQuery(value)
	if value == "" {
		return "general"
	}
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func cleanChannelToolSpotlightNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelToolSpotlightTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelToolSpotlightTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelToolSpotlightNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelToolSpotlightTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelToolSpotlightFocus(subcommand string) string {
	switch cleanChannelToolSpotlightSubcommand(subcommand) {
	case "tool-capability-spotlight", "tool-capability-draw", "capability-drill":
		return "capability"
	default:
		return "general"
	}
}

func autoChannelToolSpotlightSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-tool-spotlight-source-%s", eventID(ev))
}

func autoChannelToolSpotlightID(ev Event, opts ChannelToolSpotlightOptions) string {
	mode := channelToolSpotlightMode(opts.Mode)
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, mode, opts.Focus, opts.Note}, "|")
	prefix := "tool-spotlight"
	if mode == "drill" {
		prefix = "tool-drill"
	}
	return fmt.Sprintf("%s-%s-%s", prefix, eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolSpotlightNotifyMessageID(ev Event, spotlightID string) string {
	seed := strings.Join([]string{eventID(ev), spotlightID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelToolSpotlightMatchingContracts(repoContext RepoContext, focus string) []toolContract {
	focus = cleanChannelToolSpotlightFocus(focus)
	if focus == "" || focus == "general" {
		return nil
	}
	terms := memorySearchTerms(focus)
	var out []toolContract
	for _, contract := range toolReportContracts {
		score, _ := toolContractSearchScore(contract, focus, terms)
		if score == 0 {
			continue
		}
		out = append(out, contract)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func channelToolSpotlightCandidates(repoContext RepoContext, focus string) []toolContract {
	var source []toolContract
	if focus != "" && focus != "general" {
		source = channelToolSpotlightMatchingContracts(repoContext, focus)
	} else {
		source = toolReportContracts
	}
	var candidates []toolContract
	for _, contract := range source {
		if channelToolSpotlightEligible(repoContext, contract) {
			candidates = append(candidates, contract)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})
	return candidates
}

func channelToolSpotlightEligibleCount(repoContext RepoContext) int {
	count := 0
	for _, contract := range toolReportContracts {
		if channelToolSpotlightEligible(repoContext, contract) {
			count++
		}
	}
	return count
}

func channelToolSpotlightEligible(repoContext RepoContext, contract toolContract) bool {
	enabled, _, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
	return enabled && !blocked && !isMutatingToolContract(contract)
}

func channelToolSpotlightSeed(opts ChannelToolSpotlightOptions, focus string) string {
	return strings.Join([]string{
		opts.Repo,
		opts.Route,
		opts.Channel,
		opts.ThreadID,
		opts.SourceMessageID,
		opts.NotifyMessageID,
		opts.SpotlightID,
		channelToolSpotlightMode(opts.Mode),
		focus,
		opts.Note,
	}, "|")
}

func deterministicChannelToolSpotlightIndex(seed string, size int) int {
	if size <= 0 {
		return -1
	}
	sum := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint64(sum[:8]) % uint64(size))
}
