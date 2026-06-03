package gitclaw

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

type ChannelSkillSpotlightOptions struct {
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

type ChannelSkillSpotlightReport struct {
	SpotlightStatus   string
	FocusHash         string
	FocusTerms        int
	AvailableSkills   int
	EnabledSkills     int
	EligibleSkills    int
	MatchedSkills     int
	CandidateSkills   int
	SelectedIndex     int
	SelectedSkill     SkillSummary
	SelectionSeedSHA  string
	SelectionSHA      string
	ValidationStatus  string
	ValidationErrors  int
	ValidationWarns   int
	RawBodiesIncluded bool
}

type ChannelSkillSpotlightResult struct {
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
	SelectedPathHash    string
	SelectedFolderHash  string
	SelectionSeedHash   string
	SelectionHash       string
	Report              ChannelSkillSpotlightReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSkillSpotlightActionRequest struct {
	Options             ChannelSkillSpotlightOptions
	Report              ChannelSkillSpotlightReport
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
	SelectedNameHash    string
	SelectedPathHash    string
	SelectedFolderHash  string
	SelectionSeedSHA    string
	SelectionSHA        string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSkillSpotlightActionRequest(ev Event, cfg Config) bool {
	return isChannelSkillSpotlightActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSkillSpotlightActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSkillSpotlightSubcommand(fields[1]) {
	case "skill-spotlight", "skills-spotlight", "spotlight-skill", "skill-pick", "skill-draw", "capability-spotlight", "capability-draw":
		return true
	default:
		return false
	}
}

func BuildChannelSkillSpotlightActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillSpotlightActionRequest, error) {
	fields, trailing, ok := channelSkillSpotlightActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("missing channel skill spotlight command")
	}
	req := ChannelSkillSpotlightActionRequest{
		Options: ChannelSkillSpotlightOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             defaultChannelSkillSpotlightFocus(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSkillSpotlightSubcommand(fields[1]),
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
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--spotlight-id", "--skill-spotlight-id", "--skill-pick-id", "--skill-draw-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SpotlightID = cleanChannelSkillSpotlightID(fields[i+1])
			i++
		case "--focus", "--skill", "--query", "--for":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillSpotlightActionRequest{}, fmt.Errorf("unknown channel skill spotlight argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelSkillSpotlightIssueTargetIfPresent(ev, &req)
	if err := applyChannelSkillSpotlightPositionals(&req, positional); err != nil {
		return ChannelSkillSpotlightActionRequest{}, err
	}
	if err := applyChannelSkillSpotlightIssueTarget(ev, &req); err != nil {
		return ChannelSkillSpotlightActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelSkillSpotlightTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSkillSpotlightSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SpotlightID) == "" {
		req.Options.SpotlightID = autoChannelSkillSpotlightID(ev, req.Options)
		req.AutoSpotlightID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillSpotlightNotifyMessageID(ev, req.Options.SpotlightID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillSpotlightOptions(req.Options)
	if err := validateChannelSkillSpotlightActionRequestOptions(req.Options); err != nil {
		return ChannelSkillSpotlightActionRequest{}, err
	}
	req.Report = BuildChannelSkillSpotlightReport(repoContext, req.Options)
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
	req.SelectedNameHash = shortDocumentHash(req.Report.SelectedSkill.Name)
	req.SelectedPathHash = shortDocumentHash(req.Report.SelectedSkill.Path)
	req.SelectedFolderHash = shortDocumentHash(skillFolderName(req.Report.SelectedSkill.Path))
	req.SelectionSeedSHA = req.Report.SelectionSeedSHA
	req.SelectionSHA = req.Report.SelectionSHA
	notificationBody := renderChannelSkillSpotlightNotificationBody(req.Options, req.Report, repoContext)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelSkillSpotlightReport(repoContext RepoContext, opts ChannelSkillSpotlightOptions) ChannelSkillSpotlightReport {
	focus := cleanChannelSkillSpotlightFocus(opts.Focus)
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	report := ChannelSkillSpotlightReport{
		SpotlightStatus:   "ok",
		FocusHash:         shortDocumentHash(focus),
		FocusTerms:        len(skillSearchTerms(focus)),
		AvailableSkills:   availableSkillCount(repoContext),
		EnabledSkills:     enabledSkillCount(repoContext.SkillSummaries),
		EligibleSkills:    eligibleSkillCount(repoContext.SkillSummaries),
		ValidationStatus:  validation.Status,
		ValidationErrors:  validation.Errors,
		ValidationWarns:   validation.Warnings,
		RawBodiesIncluded: false,
	}
	candidates := channelSkillSpotlightCandidates(repoContext.SkillSummaries, focus)
	if focus != "" && focus != "general" {
		report.MatchedSkills = len(searchSkillSummaries(repoContext.SkillSummaries, focus))
		if len(candidates) == 0 {
			candidates = channelSkillSpotlightCandidates(repoContext.SkillSummaries, "")
			if len(candidates) > 0 {
				report.SpotlightStatus = "fallback"
			}
		}
	}
	if report.MatchedSkills == 0 && (focus == "" || focus == "general") {
		report.MatchedSkills = len(candidates)
	}
	report.CandidateSkills = len(candidates)
	if len(candidates) == 0 {
		report.SpotlightStatus = "no_eligible_skills"
		report.SelectedIndex = -1
		report.SelectionSeedSHA = shortDocumentHash(channelSkillSpotlightSeed(opts, focus))
		report.SelectionSHA = "none"
		return report
	}
	seed := channelSkillSpotlightSeed(opts, focus)
	idx := deterministicChannelSkillSpotlightIndex(seed, len(candidates))
	selected := candidates[idx]
	report.SelectedIndex = idx
	report.SelectedSkill = selected
	report.SelectionSeedSHA = shortDocumentHash(seed)
	report.SelectionSHA = shortDocumentHash(strings.Join([]string{
		selected.Name,
		selected.Path,
		selected.SHA,
		focus,
		fmt.Sprintf("%d", idx),
	}, "|"))
	return report
}

func RunChannelSkillSpotlight(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSkillSpotlightActionRequest, repoContext RepoContext) (ChannelSkillSpotlightResult, error) {
	opts := normalizeChannelSkillSpotlightOptions(req.Options)
	var err error
	opts, err = applyChannelSkillSpotlightRoute(cfg, opts)
	if err != nil {
		return ChannelSkillSpotlightResult{}, err
	}
	if err := validateChannelSkillSpotlightOptions(opts); err != nil {
		return ChannelSkillSpotlightResult{}, err
	}
	report := req.Report
	if report.SpotlightStatus == "" {
		report = BuildChannelSkillSpotlightReport(repoContext, opts)
	}
	body := renderChannelSkillSpotlightNotificationBody(opts, report, repoContext)
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
		return ChannelSkillSpotlightResult{}, fmt.Errorf("queue channel skill spotlight notification: %w", err)
	}
	return ChannelSkillSpotlightResult{
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
		SelectedNameHash:    shortDocumentHash(report.SelectedSkill.Name),
		SelectedPathHash:    shortDocumentHash(report.SelectedSkill.Path),
		SelectedFolderHash:  shortDocumentHash(skillFolderName(report.SelectedSkill.Path)),
		SelectionSeedHash:   report.SelectionSeedSHA,
		SelectionHash:       report.SelectionSHA,
		Report:              report,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelSkillSpotlightActionReport(ev Event, req ChannelSkillSpotlightActionRequest, result ChannelSkillSpotlightResult) string {
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
	spotlightIDHash := result.SpotlightIDHash
	if spotlightIDHash == "" {
		spotlightIDHash = req.SpotlightIDHash
	}
	focusHash := result.FocusHash
	if focusHash == "" {
		focusHash = req.FocusSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	selectedNameHash := result.SelectedNameHash
	if selectedNameHash == "" {
		selectedNameHash = req.SelectedNameHash
	}
	selectedPathHash := result.SelectedPathHash
	if selectedPathHash == "" {
		selectedPathHash = req.SelectedPathHash
	}
	selectedFolderHash := result.SelectedFolderHash
	if selectedFolderHash == "" {
		selectedFolderHash = req.SelectedFolderHash
	}
	selectionSeedHash := result.SelectionSeedHash
	if selectionSeedHash == "" {
		selectionSeedHash = req.SelectionSeedSHA
	}
	selectionHash := result.SelectionHash
	if selectionHash == "" {
		selectionHash = req.SelectionSHA
	}
	notificationBodySHA := result.NotificationBodySHA
	if notificationBodySHA == "" {
		notificationBodySHA = req.NotificationBodySHA
	}
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
	b.WriteString("## GitClaw Channel Skill Spotlight Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_spotlight_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_spotlight_status: `%s`\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "- spotlight_mode: `%s`\n", "repo-local-skill-deterministic-draw")
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
	fmt.Fprintf(&b, "- skill_spotlight_id_sha256_12: `%s`\n", noneIfEmpty(spotlightIDHash))
	fmt.Fprintf(&b, "- skill_spotlight_id_auto: `%t`\n", req.AutoSpotlightID)
	fmt.Fprintf(&b, "- spotlight_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- spotlight_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- spotlight_focus_terms: `%d`\n", report.FocusTerms)
	fmt.Fprintf(&b, "- spotlight_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- spotlight_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- spotlight_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- spotlight_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- spotlight_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", report.EnabledSkills)
	fmt.Fprintf(&b, "- eligible_skills: `%d`\n", report.EligibleSkills)
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", report.MatchedSkills)
	fmt.Fprintf(&b, "- candidate_skills: `%d`\n", report.CandidateSkills)
	fmt.Fprintf(&b, "- selected_index: `%d`\n", report.SelectedIndex)
	fmt.Fprintf(&b, "- selected_skill_name_sha256_12: `%s`\n", noneIfEmpty(selectedNameHash))
	fmt.Fprintf(&b, "- selected_skill_path_sha256_12: `%s`\n", noneIfEmpty(selectedPathHash))
	fmt.Fprintf(&b, "- selected_skill_folder_sha256_12: `%s`\n", noneIfEmpty(selectedFolderHash))
	fmt.Fprintf(&b, "- selection_seed_sha256_12: `%s`\n", noneIfEmpty(selectionSeedHash))
	fmt.Fprintf(&b, "- selection_sha256_12: `%s`\n", noneIfEmpty(selectionHash))
	fmt.Fprintf(&b, "- validation_status: `%s`\n", report.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", report.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", report.ValidationWarns)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- deterministic_selection: `%t`\n", true)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_spotlight_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_selection_seed_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_spotlight_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing skill spotlight card from repo-local skill metadata. The provider card may name one safe skill so people can act on it, while the source receipt keeps raw skill names, paths, descriptions, bodies, ids, focus text, notes, and channel bodies out of band. The action does not call a model, install or update skills, contact registries, run installers, execute tools, mutate repository files, use external randomness, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read skill-spotlight cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent skill-spotlight cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate skill-spotlight notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelSkillSpotlightNotificationBody(opts ChannelSkillSpotlightOptions, report ChannelSkillSpotlightReport, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill spotlight\n\n")
	fmt.Fprintf(&b, "Spotlight status: %s\n", report.SpotlightStatus)
	fmt.Fprintf(&b, "Focus hash: %s\n", report.FocusHash)
	fmt.Fprintf(&b, "Focus terms: %d\n", report.FocusTerms)
	fmt.Fprintf(&b, "Available skills: %d\n", report.AvailableSkills)
	fmt.Fprintf(&b, "Enabled skills: %d\n", report.EnabledSkills)
	fmt.Fprintf(&b, "Eligible skills: %d\n", report.EligibleSkills)
	fmt.Fprintf(&b, "Matched skills: %d\n", report.MatchedSkills)
	fmt.Fprintf(&b, "Candidate skills: %d\n", report.CandidateSkills)
	fmt.Fprintf(&b, "Selected index: %d\n", report.SelectedIndex)
	fmt.Fprintf(&b, "Selection seed hash: %s\n", report.SelectionSeedSHA)
	fmt.Fprintf(&b, "Selection hash: %s\n", report.SelectionSHA)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarns)
	fmt.Fprintf(&b, "Skill spotlight id hash: %s\n", shortDocumentHash(opts.SpotlightID))
	b.WriteString("\nSpotlight:\n")
	if report.SpotlightStatus == "no_eligible_skills" || strings.TrimSpace(report.SelectedSkill.Path) == "" {
		b.WriteString("- none\n")
	} else {
		skill := report.SelectedSkill
		fmt.Fprintf(&b, "- skill_name=%s path=%s folder=%s enabled=%t selected_for_this_turn=%t always=%t frontmatter=%t description_present=%t bytes=%d lines=%d sha256_12=%s requires_env=%d requires_bins=%d missing_env=%d missing_bins=%d\n",
			skill.Name,
			skill.Path,
			skillFolderName(skill.Path),
			skillIsEnabled(skill),
			skillSelectedForTurn(repoContext, skill),
			skill.Always,
			skill.FrontmatterPresent,
			strings.TrimSpace(skill.Description) != "",
			skill.Bytes,
			skill.Lines,
			skill.SHA,
			len(skill.RequiredEnv),
			len(skill.RequiredBins),
			len(skill.MissingEnv),
			len(skill.MissingBins),
		)
		b.WriteString("\nTry next:\n")
		fmt.Fprintf(&b, "- @gitclaw /channels skill-info %s --message-id <message> --notify-message-id <message>\n", skill.Name)
		fmt.Fprintf(&b, "- @gitclaw /channels skill-map %s --map-id <id> --message-id <message> --notify-message-id <message>\n", skill.Name)
	}
	b.WriteString("\nRaw skill bodies, skill descriptions, channel bodies, issue bodies, comment bodies, prompts, tool outputs, raw focus text, raw notes, and raw spotlight ids are not included in the source receipt. Skill install: not performed by this action. Skill update: not performed by this action. Registry contact: not performed by this action. Installer scripts: not run by this action. Tool execution: not performed by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSkillSpotlightActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSkillSpotlightActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSkillSpotlightIssueTarget(ev Event, req *ChannelSkillSpotlightActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill spotlight requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelSkillSpotlightIssueTargetIfPresent(ev Event, req *ChannelSkillSpotlightActionRequest) {
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

func applyChannelSkillSpotlightPositionals(req *ChannelSkillSpotlightActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel skill spotlight argument %q", value)
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
		return fmt.Errorf("unexpected channel skill spotlight argument %q", value)
	}
	return nil
}

func normalizeChannelSkillSpotlightOptions(opts ChannelSkillSpotlightOptions) ChannelSkillSpotlightOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SpotlightID = cleanChannelSkillSpotlightID(opts.SpotlightID)
	opts.Focus = cleanChannelSkillSpotlightFocus(opts.Focus)
	opts.Note = cleanChannelSkillSpotlightNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSkillSpotlightRoute(cfg Config, opts ChannelSkillSpotlightOptions) (ChannelSkillSpotlightOptions, error) {
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
		Body:      "GitClaw channel skill spotlight.",
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

func validateChannelSkillSpotlightOptions(opts ChannelSkillSpotlightOptions) error {
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
		return fmt.Errorf("missing skill spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid skill spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func validateChannelSkillSpotlightActionRequestOptions(opts ChannelSkillSpotlightOptions) error {
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
		return fmt.Errorf("missing skill spotlight id")
	}
	if !skillNamePattern.MatchString(opts.SpotlightID) {
		return fmt.Errorf("invalid skill spotlight id %q", opts.SpotlightID)
	}
	return nil
}

func cleanChannelSkillSpotlightSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSkillSpotlightID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSkillSpotlightFocus(value string) string {
	value = cleanChannelSkillSearchQuery(value)
	if value == "" {
		return "general"
	}
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func cleanChannelSkillSpotlightNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelSkillSpotlightTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelSkillSpotlightTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelSkillSpotlightNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelSkillSpotlightTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelSkillSpotlightFocus(subcommand string) string {
	switch cleanChannelSkillSpotlightSubcommand(subcommand) {
	case "capability-spotlight", "capability-draw":
		return "capability"
	default:
		return "general"
	}
}

func autoChannelSkillSpotlightSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-skill-spotlight-source-%s", eventID(ev))
}

func autoChannelSkillSpotlightID(ev Event, opts ChannelSkillSpotlightOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("skill-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSkillSpotlightNotifyMessageID(ev Event, spotlightID string) string {
	seed := strings.Join([]string{eventID(ev), spotlightID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-spotlight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelSkillSpotlightCandidates(skills []SkillSummary, focus string) []SkillSummary {
	var candidates []SkillSummary
	if focus != "" && focus != "general" {
		for _, result := range searchSkillSummaries(skills, focus) {
			if skillCatalogEligible(result.Skill) {
				candidates = append(candidates, result.Skill)
			}
		}
		return candidates
	}
	for _, skill := range skills {
		if skillCatalogEligible(skill) {
			candidates = append(candidates, skill)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Name != candidates[j].Name {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].Path < candidates[j].Path
	})
	return candidates
}

func channelSkillSpotlightSeed(opts ChannelSkillSpotlightOptions, focus string) string {
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

func deterministicChannelSkillSpotlightIndex(seed string, size int) int {
	if size <= 0 {
		return -1
	}
	sum := sha256.Sum256([]byte(seed))
	return int(binary.BigEndian.Uint64(sum[:8]) % uint64(size))
}
