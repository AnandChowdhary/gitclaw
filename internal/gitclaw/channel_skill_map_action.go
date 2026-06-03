package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSkillMapOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MapID             string
	RequestedSkill    string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSkillMapResult struct {
	Notification              ChannelSendResult
	RouteName                 string
	RouteHash                 string
	Channel                   string
	ThreadHash                string
	MessageHash               string
	NotifyHash                string
	MapIDHash                 string
	RequestedSkillHash        string
	NoteHash                  string
	BodyHash                  string
	StepHash                  string
	SnapshotHash              string
	StepCount                 int
	AvailableSkills           int
	EnabledSkills             int
	DisabledSkills            int
	AllowlistBlockedSkills    int
	SelectedSkills            int
	SkillsWithFrontmatter     int
	SkillsWithDescriptions    int
	SkillsMissingRequirements int
	ValidationStatus          string
	ValidationErrors          int
	ValidationWarnings        int
	MatchedSkills             int
	EnabledSkillNamesHash     string
	SkillPathsHash            string
	SelectedSkillPathsHash    string
	SkillIndexHash            string
}

type ChannelSkillMapActionRequest struct {
	Options                   ChannelSkillMapOptions
	Command                   string
	Subcommand                string
	AutoSourceMessageID       bool
	AutoNotifyMessageID       bool
	AutoMapID                 bool
	TargetFromIssue           bool
	NoteSource                string
	SkillSource               string
	RequestedRouteHash        string
	RequestedThreadHash       string
	RequestedMsgHash          string
	NotifyMessageHash         string
	MapIDHash                 string
	RequestedSkillHash        string
	NormalizedSkillHash       string
	RequestedSkillBytes       int
	RequestedSkillTerms       int
	NoteSHA                   string
	NoteBytes                 int
	NoteLines                 int
	StepSHA                   string
	SnapshotSHA               string
	StepCount                 int
	NotificationBodySHA       string
	AvailableSkills           int
	EnabledSkills             int
	DisabledSkills            int
	AllowlistBlockedSkills    int
	SelectedSkills            int
	SkillsWithFrontmatter     int
	SkillsWithDescriptions    int
	SkillsMissingRequirements int
	ValidationStatus          string
	ValidationErrors          int
	ValidationWarnings        int
	MatchedSkills             int
	EnabledSkillNamesHash     string
	SkillPathsHash            string
	SelectedSkillPathsHash    string
	SkillIndexHash            string
}

type channelSkillMapSnapshot struct {
	AvailableSkills           int
	EnabledSkills             int
	DisabledSkills            int
	AllowlistBlockedSkills    int
	SelectedSkills            int
	SkillsWithFrontmatter     int
	SkillsWithDescriptions    int
	SkillsMissingRequirements int
	ValidationStatus          string
	ValidationErrors          int
	ValidationWarnings        int
	MatchedSkills             int
	EnabledSkillNames         []string
	SkillPaths                []string
	SelectedSkillPaths        []string
	SkillIndex                string
	StepCount                 int
	StepHash                  string
	SnapshotHash              string
}

type channelSkillMapStep struct {
	Command string
	Reason  string
}

func IsChannelSkillMapActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelSkillMapActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelSkillMapActionFields(fields)
}

func isChannelSkillMapActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSkillMapSubcommand(fields[1]) {
	case "skill-map", "skills-map", "skill-path", "skills-path", "skill-flow", "skills-flow", "skill-runbook", "skills-runbook", "skill-safety", "safe-skill":
		return true
	default:
		return false
	}
}

func BuildChannelSkillMapActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillMapActionRequest, error) {
	fields, trailing, ok := channelSkillMapActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSkillMapActionRequest{}, fmt.Errorf("missing channel skill map command")
	}
	req := ChannelSkillMapActionRequest{
		Options: ChannelSkillMapOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSkillMapSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var skillParts []string
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--map-id", "--skill-map-id", "--runbook-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MapID = cleanChannelSkillMapID(fields[i+1])
			i++
		case "--skill", "--name", "-s":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			skillParts = append(skillParts, fields[i+1])
			req.SkillSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillMapActionRequest{}, fmt.Errorf("unknown channel skill map argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelSkillMapIssueTargetIfPresent(ev, &req)
	if req.SkillSource == "" {
		req.SkillSource = "positional"
	}
	if err := applyChannelSkillMapPositionals(&req, positional, &skillParts); err != nil {
		return ChannelSkillMapActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RequestedSkill) == "" {
		req.Options.RequestedSkill = cleanChannelSkillMapName(strings.Join(skillParts, " "))
	}
	if strings.TrimSpace(req.Options.RequestedSkill) == "" {
		req.Options.RequestedSkill = parseChannelSkillMapTrailingSkill(trailing)
		if req.Options.RequestedSkill != "" {
			req.SkillSource = "trailing-skill"
		}
	}
	if err := applyChannelSkillMapIssueTarget(ev, &req); err != nil {
		return ChannelSkillMapActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelSkillMapTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSkillMapSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MapID) == "" {
		req.Options.MapID = autoChannelSkillMapID(ev, req.Options)
		req.AutoMapID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillMapNotifyMessageID(ev, req.Options.MapID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillMapOptions(req.Options)
	if err := validateChannelSkillMapActionRequestOptions(req.Options); err != nil {
		return ChannelSkillMapActionRequest{}, err
	}
	snapshot := buildChannelSkillMapSnapshot(repoContext, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MapIDHash = shortDocumentHash(req.Options.MapID)
	req.RequestedSkillHash = shortDocumentHash(req.Options.RequestedSkill)
	req.NormalizedSkillHash = shortDocumentHash(strings.ToLower(cleanSkillLookupName(req.Options.RequestedSkill)))
	req.RequestedSkillBytes = len(req.Options.RequestedSkill)
	req.RequestedSkillTerms = len(memorySearchTerms(req.Options.RequestedSkill))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelSkillMapNotificationBody(req.Options, repoContext))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelSkillMap(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSkillMapOptions, repoContext RepoContext) (ChannelSkillMapResult, error) {
	opts = normalizeChannelSkillMapOptions(opts)
	var err error
	opts, err = applyChannelSkillMapRoute(cfg, opts)
	if err != nil {
		return ChannelSkillMapResult{}, err
	}
	if err := validateChannelSkillMapOptions(opts); err != nil {
		return ChannelSkillMapResult{}, err
	}
	body := renderChannelSkillMapNotificationBody(opts, repoContext)
	snapshot := buildChannelSkillMapSnapshot(repoContext, opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSkillMapResult{}, fmt.Errorf("queue channel skill map notification: %w", err)
	}
	result := ChannelSkillMapResult{
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
		MapIDHash:          shortDocumentHash(opts.MapID),
		RequestedSkillHash: shortDocumentHash(opts.RequestedSkill),
		NoteHash:           shortDocumentHash(opts.Note),
		BodyHash:           shortDocumentHash(body),
		StepHash:           snapshot.StepHash,
		SnapshotHash:       snapshot.SnapshotHash,
		StepCount:          snapshot.StepCount,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelSkillMapActionReport(ev Event, req ChannelSkillMapActionRequest, result ChannelSkillMapResult) string {
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
	requestedSkillHash := firstNonEmpty(result.RequestedSkillHash, req.RequestedSkillHash)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	stepHash := firstNonEmpty(result.StepHash, req.StepSHA)
	snapshotHash := firstNonEmpty(result.SnapshotHash, req.SnapshotSHA)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Skill Map Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_map_mode: `%s`\n", "provider-facing-skill-sequence")
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
	fmt.Fprintf(&b, "- skill_map_id_sha256_12: `%s`\n", noneIfEmpty(mapIDHash))
	fmt.Fprintf(&b, "- skill_map_id_auto: `%t`\n", req.AutoMapID)
	fmt.Fprintf(&b, "- requested_skill_sha256_12: `%s`\n", noneIfEmpty(requestedSkillHash))
	fmt.Fprintf(&b, "- normalized_skill_sha256_12: `%s`\n", noneIfEmpty(req.NormalizedSkillHash))
	fmt.Fprintf(&b, "- requested_skill_bytes: `%d`\n", req.RequestedSkillBytes)
	fmt.Fprintf(&b, "- requested_skill_terms: `%d`\n", req.RequestedSkillTerms)
	fmt.Fprintf(&b, "- skill_source: `%s`\n", noneIfEmpty(req.SkillSource))
	fmt.Fprintf(&b, "- skill_map_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- skill_map_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- skill_map_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- skill_map_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- skill_map_step_count: `%d`\n", nonzeroOrReq(result.StepCount, req.StepCount))
	fmt.Fprintf(&b, "- skill_map_step_sha256_12: `%s`\n", noneIfEmpty(stepHash))
	fmt.Fprintf(&b, "- skill_map_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", nonzeroOrReq(result.AvailableSkills, req.AvailableSkills))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", nonzeroOrReq(result.EnabledSkills, req.EnabledSkills))
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", result.DisabledSkills)
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", result.AllowlistBlockedSkills)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", result.SelectedSkills)
	fmt.Fprintf(&b, "- skills_with_frontmatter: `%d`\n", result.SkillsWithFrontmatter)
	fmt.Fprintf(&b, "- skills_with_descriptions: `%d`\n", result.SkillsWithDescriptions)
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", result.SkillsMissingRequirements)
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", result.MatchedSkills)
	fmt.Fprintf(&b, "- validation_status: `%s`\n", firstNonEmpty(result.ValidationStatus, req.ValidationStatus, "unknown"))
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", result.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", result.ValidationWarnings)
	fmt.Fprintf(&b, "- enabled_skill_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.EnabledSkillNamesHash, req.EnabledSkillNamesHash)))
	fmt.Fprintf(&b, "- skill_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SkillPathsHash, req.SkillPathsHash)))
	fmt.Fprintf(&b, "- selected_skill_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSkillPathsHash, req.SelectedSkillPathsHash)))
	fmt.Fprintf(&b, "- skill_index_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SkillIndexHash, req.SkillIndexHash)))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_proposal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_rehearsal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_note_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_map_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_skill_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_map_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_map_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_map_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing skill map on the canonical channel issue. This is a safe sequence card for Slack or Telegram users who want to move from skill discovery to reviewed skill workflows: it reports compact skill catalog metadata and points at status, search, info, proposal, rehearsal, and skill-note commands, but it does not install skills, update skills, contact registries, run installers, create proposal issues, create rehearsal issues, create skill-note issues, call a model, mutate workflows, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, map ids, requested skill names, notes, step text, raw skill names, paths, descriptions, bodies, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read skill-map cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent skill-map cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate skill-map cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelSkillMapActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSkillMapActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSkillMapIssueTarget(ev Event, req *ChannelSkillMapActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill map requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelSkillMapIssueTargetIfPresent(ev Event, req *ChannelSkillMapActionRequest) {
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

func applyChannelSkillMapPositionals(req *ChannelSkillMapActionRequest, positional []string, skillParts *[]string) error {
	if req == nil {
		return nil
	}
	for i, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !req.TargetFromIssue && req.Options.Route == "" && req.Options.Channel == "" && len(*skillParts) == 0 && len(positional)-i > 1 {
			req.Options.Route = value
			continue
		}
		if len(*skillParts) == 0 {
			*skillParts = append(*skillParts, value)
			continue
		}
		if req.Options.MapID == "" {
			req.Options.MapID = cleanChannelSkillMapID(value)
			continue
		}
		return fmt.Errorf("unexpected channel skill map argument %q", value)
	}
	return nil
}

func normalizeChannelSkillMapOptions(opts ChannelSkillMapOptions) ChannelSkillMapOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MapID = cleanChannelSkillMapID(opts.MapID)
	opts.RequestedSkill = cleanChannelSkillMapName(opts.RequestedSkill)
	opts.Note = cleanChannelSkillMapNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSkillMapRoute(cfg Config, opts ChannelSkillMapOptions) (ChannelSkillMapOptions, error) {
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
		Body:      "GitClaw channel skill map.",
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

func validateChannelSkillMapOptions(opts ChannelSkillMapOptions) error {
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
		return fmt.Errorf("missing skill map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid skill map id %q", opts.MapID)
	}
	if opts.RequestedSkill == "" {
		return fmt.Errorf("missing requested skill")
	}
	return nil
}

func validateChannelSkillMapActionRequestOptions(opts ChannelSkillMapOptions) error {
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
		return fmt.Errorf("missing skill map id")
	}
	if !skillNamePattern.MatchString(opts.MapID) {
		return fmt.Errorf("invalid skill map id %q", opts.MapID)
	}
	if opts.RequestedSkill == "" {
		return fmt.Errorf("missing requested skill")
	}
	return nil
}

func cleanChannelSkillMapSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSkillMapID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSkillMapName(value string) string {
	return cleanChannelSkillInfoName(value)
}

func cleanChannelSkillMapNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelSkillMapTrailingSkill(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "skill:") || strings.HasPrefix(lower, "name:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelSkillMapName(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func parseChannelSkillMapTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelSkillMapTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelSkillMapNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelSkillMapTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "skill:") ||
		strings.HasPrefix(lower, "name:") ||
		strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func autoChannelSkillMapSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-skill-map-source-%s", eventID(ev))
}

func autoChannelSkillMapID(ev Event, opts ChannelSkillMapOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedSkill, opts.Note}, "|")
	return cleanChannelSkillMapID(fmt.Sprintf("skill-map-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSkillMapNotifyMessageID(ev Event, mapID string) string {
	seed := strings.Join([]string{eventID(ev), mapID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelSkillMapNotificationBody(opts ChannelSkillMapOptions, repoContext RepoContext) string {
	opts = normalizeChannelSkillMapOptions(opts)
	snapshot := buildChannelSkillMapSnapshot(repoContext, opts)
	steps := channelSkillMapStepsForSkill(opts.RequestedSkill, snapshot)
	var b strings.Builder
	b.WriteString("GitClaw channel skill map.\n\n")
	fmt.Fprintf(&b, "Requested skill: %s\n", opts.RequestedSkill)
	fmt.Fprintf(&b, "Available skills: %d\n", snapshot.AvailableSkills)
	fmt.Fprintf(&b, "Enabled skills: %d\n", snapshot.EnabledSkills)
	fmt.Fprintf(&b, "Disabled skills: %d\n", snapshot.DisabledSkills)
	fmt.Fprintf(&b, "Allowlist blocked skills: %d\n", snapshot.AllowlistBlockedSkills)
	fmt.Fprintf(&b, "Selected skills for this turn: %d\n", snapshot.SelectedSkills)
	fmt.Fprintf(&b, "Matched skills: %d\n", snapshot.MatchedSkills)
	fmt.Fprintf(&b, "Skills with frontmatter: %d\n", snapshot.SkillsWithFrontmatter)
	fmt.Fprintf(&b, "Skills with descriptions: %d\n", snapshot.SkillsWithDescriptions)
	fmt.Fprintf(&b, "Skills missing requirements: %d\n", snapshot.SkillsMissingRequirements)
	fmt.Fprintf(&b, "Validation status: %s\n", snapshot.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", snapshot.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", snapshot.ValidationWarnings)
	b.WriteString("\nSkill sequence:\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, step.Command, step.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Skill map hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Skill step hash: %s\n", snapshot.StepHash)
	b.WriteString("\nMap source: current GitHub Actions checkout skill metadata.\n")
	b.WriteString("Full skill bodies: not included.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Skill update: not performed by this action.\n")
	b.WriteString("Registry contact: not performed by this action.\n")
	b.WriteString("Installer scripts: not run by this action.\n")
	b.WriteString("Skill proposal issue creation: not performed by this action.\n")
	b.WriteString("Skill rehearsal issue creation: not performed by this action.\n")
	b.WriteString("Skill-note issue creation: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelSkillMapSnapshot(repoContext RepoContext, opts ChannelSkillMapOptions) channelSkillMapSnapshot {
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	snapshot := channelSkillMapSnapshot{
		AvailableSkills:           availableSkillCount(repoContext),
		EnabledSkills:             enabledSkillCount(repoContext.SkillSummaries),
		DisabledSkills:            disabledByConfigCount(repoContext.SkillSummaries),
		AllowlistBlockedSkills:    blockedByAllowlistCount(repoContext.SkillSummaries),
		SelectedSkills:            len(repoContext.Skills),
		SkillsWithFrontmatter:     skillsWithFrontmatter(repoContext.SkillSummaries),
		SkillsWithDescriptions:    skillsWithDescription(repoContext.SkillSummaries),
		SkillsMissingRequirements: missingRequirementSkillCount(repoContext.SkillSummaries),
		ValidationStatus:          validation.Status,
		ValidationErrors:          validation.Errors,
		ValidationWarnings:        validation.Warnings,
		MatchedSkills:             len(matchingSkillSummaries(repoContext.SkillSummaries, opts.RequestedSkill)),
		EnabledSkillNames:         channelSkillStatusEnabledNames(repoContext),
		SkillPaths:                channelSkillStatusSkillPaths(repoContext),
		SelectedSkillPaths:        channelSkillStatusSelectedSkillPaths(repoContext),
		SkillIndex:                channelSkillStatusIndex(repoContext),
	}
	steps := channelSkillMapStepsForSkill(opts.RequestedSkill, snapshot)
	snapshot.StepCount = len(steps)
	snapshot.StepHash = shortDocumentHash(channelSkillMapStepManifest(steps))
	snapshot.SnapshotHash = shortDocumentHash(channelSkillMapSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelSkillMapStepsForSkill(skill string, snapshot channelSkillMapSnapshot) []channelSkillMapStep {
	skill = cleanChannelSkillMapName(skill)
	if skill == "" {
		skill = "<skill>"
	}
	return []channelSkillMapStep{
		{Command: "/channels skills --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("confirm current skill availability first (%d enabled skills)", snapshot.EnabledSkills)},
		{Command: fmt.Sprintf("/channels skill-search %s --message-id <id> --notify-message-id <id>", skill), Reason: "find nearby skills without loading full skill bodies"},
		{Command: fmt.Sprintf("/channels skill-info %s --message-id <id> --notify-message-id <id>", skill), Reason: "inspect the focused skill card before opening reviewed work"},
		{Command: fmt.Sprintf("/channels propose-skill %s --message-id <id> --notify-message-id <id>", skill), Reason: "open a reviewed skill proposal issue when a durable change is needed"},
		{Command: fmt.Sprintf("/channels rehearse-skill %s --id <rehearsal-id> --message-id <id> --notify-message-id <id>", skill), Reason: "open a rehearsal issue for model-backed conversation around the skill"},
		{Command: fmt.Sprintf("/channels skill-note --skill %s --note-id <note-id> --message-id <id> --notify-message-id <id>", skill), Reason: "capture a channel-origin lesson without installing or updating skills"},
	}
}

func channelSkillMapStepManifest(steps []channelSkillMapStep) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, strings.Join([]string{step.Command, step.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}

func channelSkillMapSnapshotManifest(opts ChannelSkillMapOptions, snapshot channelSkillMapSnapshot) string {
	return fmt.Sprintf(
		"skill=%s\ncounts=%d/%d/%d/%d/%d/%d/%d/%d/%d\nvalidation=%s/%d/%d\nhashes=%s/%s/%s/%s\nsteps=%d/%s\nnote=%s",
		shortDocumentHash(cleanChannelSkillMapName(opts.RequestedSkill)),
		snapshot.AvailableSkills,
		snapshot.EnabledSkills,
		snapshot.DisabledSkills,
		snapshot.AllowlistBlockedSkills,
		snapshot.SelectedSkills,
		snapshot.SkillsWithFrontmatter,
		snapshot.SkillsWithDescriptions,
		snapshot.SkillsMissingRequirements,
		snapshot.MatchedSkills,
		snapshot.ValidationStatus,
		snapshot.ValidationErrors,
		snapshot.ValidationWarnings,
		hashStringList(snapshot.EnabledSkillNames),
		hashStringList(snapshot.SkillPaths),
		hashStringList(snapshot.SelectedSkillPaths),
		hashStringOrNone(snapshot.SkillIndex),
		snapshot.StepCount,
		snapshot.StepHash,
		shortDocumentHash(opts.Note),
	)
}

func (r *ChannelSkillMapActionRequest) applySnapshot(snapshot channelSkillMapSnapshot) {
	r.AvailableSkills = snapshot.AvailableSkills
	r.EnabledSkills = snapshot.EnabledSkills
	r.DisabledSkills = snapshot.DisabledSkills
	r.AllowlistBlockedSkills = snapshot.AllowlistBlockedSkills
	r.SelectedSkills = snapshot.SelectedSkills
	r.SkillsWithFrontmatter = snapshot.SkillsWithFrontmatter
	r.SkillsWithDescriptions = snapshot.SkillsWithDescriptions
	r.SkillsMissingRequirements = snapshot.SkillsMissingRequirements
	r.ValidationStatus = snapshot.ValidationStatus
	r.ValidationErrors = snapshot.ValidationErrors
	r.ValidationWarnings = snapshot.ValidationWarnings
	r.MatchedSkills = snapshot.MatchedSkills
	r.EnabledSkillNamesHash = hashStringList(snapshot.EnabledSkillNames)
	r.SkillPathsHash = hashStringList(snapshot.SkillPaths)
	r.SelectedSkillPathsHash = hashStringList(snapshot.SelectedSkillPaths)
	r.SkillIndexHash = hashStringOrNone(snapshot.SkillIndex)
	r.StepCount = snapshot.StepCount
	r.StepSHA = snapshot.StepHash
	r.SnapshotSHA = snapshot.SnapshotHash
}

func (r *ChannelSkillMapResult) applySnapshot(snapshot channelSkillMapSnapshot) {
	r.AvailableSkills = snapshot.AvailableSkills
	r.EnabledSkills = snapshot.EnabledSkills
	r.DisabledSkills = snapshot.DisabledSkills
	r.AllowlistBlockedSkills = snapshot.AllowlistBlockedSkills
	r.SelectedSkills = snapshot.SelectedSkills
	r.SkillsWithFrontmatter = snapshot.SkillsWithFrontmatter
	r.SkillsWithDescriptions = snapshot.SkillsWithDescriptions
	r.SkillsMissingRequirements = snapshot.SkillsMissingRequirements
	r.ValidationStatus = snapshot.ValidationStatus
	r.ValidationErrors = snapshot.ValidationErrors
	r.ValidationWarnings = snapshot.ValidationWarnings
	r.MatchedSkills = snapshot.MatchedSkills
	r.EnabledSkillNamesHash = hashStringList(snapshot.EnabledSkillNames)
	r.SkillPathsHash = hashStringList(snapshot.SkillPaths)
	r.SelectedSkillPathsHash = hashStringList(snapshot.SelectedSkillPaths)
	r.SkillIndexHash = hashStringOrNone(snapshot.SkillIndex)
	r.StepCount = snapshot.StepCount
}
