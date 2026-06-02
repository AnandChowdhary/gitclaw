package gitclaw

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type ChannelSkillStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelSkillStatusResult struct {
	Notification              ChannelSendResult
	RouteName                 string
	RouteHash                 string
	Channel                   string
	ThreadHash                string
	MessageHash               string
	NotifyHash                string
	StatusIDHash              string
	BodyHash                  string
	AvailableSkills           int
	EnabledSkills             int
	DisabledSkills            int
	AllowlistBlockedSkills    int
	SelectedSkills            int
	SkillsWithFrontmatter     int
	SkillsWithDescriptions    int
	SkillsMissingRequirements int
	EnabledSkillNamesHash     string
	SkillPathsHash            string
	SelectedSkillPathsHash    string
	SkillIndexHash            string
}

type ChannelSkillStatusActionRequest struct {
	Options                   ChannelSkillStatusOptions
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
	AvailableSkills           int
	EnabledSkills             int
	DisabledSkills            int
	AllowlistBlockedSkills    int
	SelectedSkills            int
	SkillsWithFrontmatter     int
	SkillsWithDescriptions    int
	SkillsMissingRequirements int
	EnabledSkillNamesHash     string
	SkillPathsHash            string
	SelectedSkillPathsHash    string
	SkillIndexHash            string
	NotificationBodySHA       string
}

func IsChannelSkillStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelSkillStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelSkillStatusActionFields(fields)
}

func isChannelSkillStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "skills", "skill-status", "skills-status", "skill-list", "skills-list", "capabilities", "capability-status":
		return true
	default:
		return false
	}
}

func BuildChannelSkillStatusActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillStatusActionRequest, error) {
	fields, _, ok := channelSkillStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSkillStatusActionRequest{}, fmt.Errorf("missing channel skill status command")
	}
	req := ChannelSkillStatusActionRequest{
		Options: ChannelSkillStatusOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--skill-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelSkillStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillStatusActionRequest{}, fmt.Errorf("unknown channel skill status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelSkillStatusActionRequest{}, fmt.Errorf("unexpected channel skill status argument %q", field)
		}
	}
	if err := applyChannelSkillStatusIssueTarget(ev, &req); err != nil {
		return ChannelSkillStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSkillStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelSkillStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillStatusOptions(req.Options)
	if err := validateChannelSkillStatusActionRequestOptions(req.Options); err != nil {
		return ChannelSkillStatusActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.AvailableSkills = availableSkillCount(repoContext)
	req.EnabledSkills = enabledSkillCount(repoContext.SkillSummaries)
	req.DisabledSkills = disabledByConfigCount(repoContext.SkillSummaries)
	req.AllowlistBlockedSkills = blockedByAllowlistCount(repoContext.SkillSummaries)
	req.SelectedSkills = len(repoContext.Skills)
	req.SkillsWithFrontmatter = skillsWithFrontmatter(repoContext.SkillSummaries)
	req.SkillsWithDescriptions = skillsWithDescription(repoContext.SkillSummaries)
	req.SkillsMissingRequirements = missingRequirementSkillCount(repoContext.SkillSummaries)
	req.EnabledSkillNamesHash = hashStringList(channelSkillStatusEnabledNames(repoContext))
	req.SkillPathsHash = hashStringList(channelSkillStatusSkillPaths(repoContext))
	req.SelectedSkillPathsHash = hashStringList(channelSkillStatusSelectedSkillPaths(repoContext))
	req.SkillIndexHash = hashStringOrNone(channelSkillStatusIndex(repoContext))
	req.NotificationBodySHA = shortDocumentHash(renderChannelSkillStatusNotificationBody(req.Options, repoContext))
	return req, nil
}

func RunChannelSkillStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSkillStatusOptions, repoContext RepoContext) (ChannelSkillStatusResult, error) {
	opts = normalizeChannelSkillStatusOptions(opts)
	var err error
	opts, err = applyChannelSkillStatusRoute(cfg, opts, repoContext)
	if err != nil {
		return ChannelSkillStatusResult{}, err
	}
	if err := validateChannelSkillStatusOptions(opts); err != nil {
		return ChannelSkillStatusResult{}, err
	}
	body := renderChannelSkillStatusNotificationBody(opts, repoContext)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSkillStatusResult{}, fmt.Errorf("queue channel skill status notification: %w", err)
	}
	return ChannelSkillStatusResult{
		Notification:              notification,
		RouteName:                 opts.Route,
		RouteHash:                 channelRouteHash(opts.Route),
		Channel:                   opts.Channel,
		ThreadHash:                shortDocumentHash(opts.ThreadID),
		MessageHash:               shortDocumentHash(opts.SourceMessageID),
		NotifyHash:                shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash:              shortDocumentHash(opts.StatusID),
		BodyHash:                  shortDocumentHash(body),
		AvailableSkills:           availableSkillCount(repoContext),
		EnabledSkills:             enabledSkillCount(repoContext.SkillSummaries),
		DisabledSkills:            disabledByConfigCount(repoContext.SkillSummaries),
		AllowlistBlockedSkills:    blockedByAllowlistCount(repoContext.SkillSummaries),
		SelectedSkills:            len(repoContext.Skills),
		SkillsWithFrontmatter:     skillsWithFrontmatter(repoContext.SkillSummaries),
		SkillsWithDescriptions:    skillsWithDescription(repoContext.SkillSummaries),
		SkillsMissingRequirements: missingRequirementSkillCount(repoContext.SkillSummaries),
		EnabledSkillNamesHash:     hashStringList(channelSkillStatusEnabledNames(repoContext)),
		SkillPathsHash:            hashStringList(channelSkillStatusSkillPaths(repoContext)),
		SelectedSkillPathsHash:    hashStringList(channelSkillStatusSelectedSkillPaths(repoContext)),
		SkillIndexHash:            hashStringOrNone(channelSkillStatusIndex(repoContext)),
	}, nil
}

func RenderChannelSkillStatusActionReport(ev Event, req ChannelSkillStatusActionRequest, result ChannelSkillStatusResult) string {
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
	b.WriteString("## GitClaw Channel Skill Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_snapshot_mode: `%s`\n", "provider-facing-skill-status")
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
	fmt.Fprintf(&b, "- skill_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- skill_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", nonzeroOrReq(result.AvailableSkills, req.AvailableSkills))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", nonzeroOrReq(result.EnabledSkills, req.EnabledSkills))
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", result.DisabledSkills)
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", result.AllowlistBlockedSkills)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", result.SelectedSkills)
	fmt.Fprintf(&b, "- skills_with_frontmatter: `%d`\n", result.SkillsWithFrontmatter)
	fmt.Fprintf(&b, "- skills_with_descriptions: `%d`\n", result.SkillsWithDescriptions)
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", result.SkillsMissingRequirements)
	fmt.Fprintf(&b, "- enabled_skill_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.EnabledSkillNamesHash, req.EnabledSkillNamesHash)))
	fmt.Fprintf(&b, "- skill_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SkillPathsHash, req.SkillPathsHash)))
	fmt.Fprintf(&b, "- selected_skill_paths_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SelectedSkillPathsHash, req.SelectedSkillPathsHash)))
	fmt.Fprintf(&b, "- skill_index_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.SkillIndexHash, req.SkillIndexHash)))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing skill status snapshot on the canonical channel issue. This is the GitHub-native channel version of a skills-list command: it reports compact skill availability from the current Actions checkout, but it does not call a model, install skills, update skills, contact registries, run installers, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, raw skill bodies, descriptions, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the skill-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent skill-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate skill-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/channels propose-skill` or `/channels rehearse-skill` when a channel message should become a reviewed skill workflow\n")
	return strings.TrimSpace(b.String())
}

func channelSkillStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSkillStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSkillStatusIssueTarget(ev Event, req *ChannelSkillStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSkillStatusOptions(opts ChannelSkillStatusOptions) ChannelSkillStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelSkillStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSkillStatusRoute(cfg Config, opts ChannelSkillStatusOptions, repoContext RepoContext) (ChannelSkillStatusOptions, error) {
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
		Body:      inlineListOrNone(channelSkillStatusEnabledNames(repoContext)),
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

func validateChannelSkillStatusOptions(opts ChannelSkillStatusOptions) error {
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
		return fmt.Errorf("missing skill status id")
	}
	return nil
}

func validateChannelSkillStatusActionRequestOptions(opts ChannelSkillStatusOptions) error {
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
		return fmt.Errorf("missing skill status id")
	}
	return nil
}

func cleanChannelSkillStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelSkillStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-skill-source-%s", eventID(ev))
}

func autoChannelSkillStatusID(ev Event, opts ChannelSkillStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("skill-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSkillStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelSkillStatusNotificationBody(opts ChannelSkillStatusOptions, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill status.\n\n")
	fmt.Fprintf(&b, "Available skills: %d\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "Enabled skills: %d\n", enabledSkillCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "Disabled skills: %d\n", disabledByConfigCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "Allowlist blocked skills: %d\n", blockedByAllowlistCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "Selected skills for this turn: %d\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "Enabled skill names: %s\n", inlineListOrNone(channelSkillStatusEnabledNames(repoContext)))
	fmt.Fprintf(&b, "Skills with frontmatter: %d\n", skillsWithFrontmatter(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "Skills with descriptions: %d\n", skillsWithDescription(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "Skills missing requirements: %d\n", missingRequirementSkillCount(repoContext.SkillSummaries))
	b.WriteString("Progressive disclosure: enabled\n")
	b.WriteString("Snapshot source: current GitHub Actions checkout\n")
	b.WriteString("\nFull skill bodies: not included.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Skill update: not performed by this action.\n")
	b.WriteString("Registry contact: not performed by this action.\n")
	b.WriteString("Installer scripts: not run by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSkillStatusEnabledNames(repoContext RepoContext) []string {
	var names []string
	for _, skill := range repoContext.SkillSummaries {
		if skillIsEnabled(skill) {
			names = append(names, skill.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelSkillStatusSkillPaths(repoContext RepoContext) []string {
	var paths []string
	for _, skill := range repoContext.SkillSummaries {
		paths = append(paths, skill.Path)
	}
	return uniqueSortedStrings(paths)
}

func channelSkillStatusSelectedSkillPaths(repoContext RepoContext) []string {
	var paths []string
	for _, skill := range repoContext.Skills {
		paths = append(paths, skill.Path)
	}
	return uniqueSortedStrings(paths)
}

func channelSkillStatusIndex(repoContext RepoContext) string {
	var lines []string
	for _, skill := range repoContext.SkillSummaries {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%t", skill.Name, skill.Path, skill.SHA, skillIsEnabled(skill)))
	}
	if len(lines) == 0 {
		for _, output := range repoContext.ToolOutputs {
			if output.Name == "gitclaw.skill_index" {
				return strings.TrimSpace(output.Output)
			}
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nonzeroOrReq(value, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}
