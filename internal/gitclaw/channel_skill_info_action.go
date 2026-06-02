package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSkillInfoOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	InfoID            string
	RequestedSkill    string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSkillInfoReport struct {
	RequestedSkillHash string
	NormalizedHash     string
	InfoStatus         string
	AvailableSkills    int
	EnabledSkills      int
	MatchedSkills      int
	ValidationStatus   string
	ValidationErrors   int
	ValidationWarnings int
	RawBodiesIncluded  bool
	Skills             []SkillSummary
}

type ChannelSkillInfoResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	InfoIDHash          string
	RequestedSkillHash  string
	NormalizedSkillHash string
	Info                ChannelSkillInfoReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSkillInfoActionRequest struct {
	Options               ChannelSkillInfoOptions
	Info                  ChannelSkillInfoReport
	Command               string
	Subcommand            string
	AutoSourceMessageID   bool
	AutoNotifyMessageID   bool
	AutoInfoID            bool
	TargetFromIssue       bool
	SkillSource           string
	RequestedRouteHash    string
	RequestedThreadHash   string
	RequestedMsgHash      string
	NotifyMessageHash     string
	InfoIDHash            string
	RequestedSkillHash    string
	NormalizedSkillHash   string
	RequestedSkillBytes   int
	MatchedSkillNamesHash string
	MatchedSkillPathsHash string
	SkillInfoIndexHash    string
	NotificationBodySHA   string
	NotificationBytes     int
	NotificationLines     int
}

func IsChannelSkillInfoActionRequest(ev Event, cfg Config) bool {
	return isChannelSkillInfoActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSkillInfoActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSkillInfoSubcommand(fields[1]) {
	case "skill-info", "skills-info", "skill-describe", "describe-skill", "skill-card", "skill-capability-info", "skill-capability-describe":
		return true
	default:
		return false
	}
}

func BuildChannelSkillInfoActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillInfoActionRequest, error) {
	fields, trailing, ok := channelSkillInfoActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSkillInfoActionRequest{}, fmt.Errorf("missing channel skill info command")
	}
	req := ChannelSkillInfoActionRequest{
		Options: ChannelSkillInfoOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSkillInfoSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var skillParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--info-id", "--skill-info-id", "--skill-capability-info-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.InfoID = cleanChannelSkillInfoID(fields[i+1])
			i++
		case "--skill", "--name":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			skillParts = append(skillParts, fields[i+1])
			req.SkillSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillInfoActionRequest{}, fmt.Errorf("unknown channel skill info argument %q", field)
			}
			skillParts = append(skillParts, field)
			if req.SkillSource == "" {
				req.SkillSource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.RequestedSkill) == "" {
		req.Options.RequestedSkill = cleanChannelSkillInfoName(strings.Join(skillParts, " "))
	}
	if strings.TrimSpace(req.Options.RequestedSkill) == "" {
		req.Options.RequestedSkill = parseChannelSkillInfoTrailingSkill(trailing)
		if req.Options.RequestedSkill != "" {
			req.SkillSource = "trailing-skill"
		}
	}
	if err := applyChannelSkillInfoIssueTarget(ev, &req); err != nil {
		return ChannelSkillInfoActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSkillInfoSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.InfoID) == "" {
		req.Options.InfoID = autoChannelSkillInfoID(ev, req.Options)
		req.AutoInfoID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillInfoNotifyMessageID(ev, req.Options.InfoID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillInfoOptions(req.Options)
	if err := validateChannelSkillInfoActionRequestOptions(req.Options); err != nil {
		return ChannelSkillInfoActionRequest{}, err
	}
	req.Info = BuildChannelSkillInfoReport(repoContext, req.Options.RequestedSkill)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.InfoIDHash = shortDocumentHash(req.Options.InfoID)
	req.RequestedSkillHash = req.Info.RequestedSkillHash
	req.NormalizedSkillHash = req.Info.NormalizedHash
	req.RequestedSkillBytes = len(req.Options.RequestedSkill)
	req.MatchedSkillNamesHash = hashStringList(channelSkillInfoNames(req.Info.Skills))
	req.MatchedSkillPathsHash = hashStringList(channelSkillInfoPaths(req.Info.Skills))
	req.SkillInfoIndexHash = hashStringOrNone(channelSkillInfoIndex(req.Info, repoContext))
	notificationBody := RenderChannelSkillInfoNotificationBody(req.Options, req.Info, repoContext)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelSkillInfoReport(repoContext RepoContext, requestedSkill string) ChannelSkillInfoReport {
	requestedSkill = cleanChannelSkillInfoName(requestedSkill)
	normalized := strings.ToLower(cleanSkillLookupName(requestedSkill))
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	report := ChannelSkillInfoReport{
		RequestedSkillHash: shortDocumentHash(requestedSkill),
		NormalizedHash:     shortDocumentHash(normalized),
		InfoStatus:         "ok",
		AvailableSkills:    availableSkillCount(repoContext),
		EnabledSkills:      enabledSkillCount(repoContext.SkillSummaries),
		ValidationStatus:   validation.Status,
		ValidationErrors:   validation.Errors,
		ValidationWarnings: validation.Warnings,
		RawBodiesIncluded:  false,
	}
	if requestedSkill == "" || normalized == "" {
		report.InfoStatus = "missing_skill"
		return report
	}
	report.Skills = matchingSkillSummaries(repoContext.SkillSummaries, requestedSkill)
	report.MatchedSkills = len(report.Skills)
	if len(report.Skills) == 0 {
		report.InfoStatus = "not_found"
		return report
	}
	if len(report.Skills) > 1 {
		report.InfoStatus = "ambiguous"
	}
	return report
}

func RunChannelSkillInfo(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSkillInfoActionRequest, repoContext RepoContext) (ChannelSkillInfoResult, error) {
	opts := normalizeChannelSkillInfoOptions(req.Options)
	var err error
	opts, err = applyChannelSkillInfoRoute(cfg, opts)
	if err != nil {
		return ChannelSkillInfoResult{}, err
	}
	if err := validateChannelSkillInfoOptions(opts); err != nil {
		return ChannelSkillInfoResult{}, err
	}
	info := req.Info
	if info.RequestedSkillHash == "" {
		info = BuildChannelSkillInfoReport(repoContext, opts.RequestedSkill)
	}
	body := RenderChannelSkillInfoNotificationBody(opts, info, repoContext)
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
		return ChannelSkillInfoResult{}, fmt.Errorf("queue channel skill info notification: %w", err)
	}
	return ChannelSkillInfoResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		InfoIDHash:          shortDocumentHash(opts.InfoID),
		RequestedSkillHash:  info.RequestedSkillHash,
		NormalizedSkillHash: info.NormalizedHash,
		Info:                info,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelSkillInfoActionReport(ev Event, req ChannelSkillInfoActionRequest, result ChannelSkillInfoResult) string {
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
	infoIDHash := result.InfoIDHash
	if infoIDHash == "" {
		infoIDHash = req.InfoIDHash
	}
	requestedSkillHash := result.RequestedSkillHash
	if requestedSkillHash == "" {
		requestedSkillHash = req.RequestedSkillHash
	}
	normalizedSkillHash := result.NormalizedSkillHash
	if normalizedSkillHash == "" {
		normalizedSkillHash = req.NormalizedSkillHash
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
	info := result.Info
	if info.RequestedSkillHash == "" {
		info = req.Info
	}
	matchedNamesHash := hashStringList(channelSkillInfoNames(info.Skills))
	if matchedNamesHash == "" {
		matchedNamesHash = req.MatchedSkillNamesHash
	}
	matchedPathsHash := hashStringList(channelSkillInfoPaths(info.Skills))
	if matchedPathsHash == "" {
		matchedPathsHash = req.MatchedSkillPathsHash
	}
	infoIndexHash := req.SkillInfoIndexHash
	if infoIndexHash == "" {
		infoIndexHash = hashStringOrNone(channelSkillInfoIndex(info, RepoContext{}))
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Skill Info Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_info_status: `%s`\n", info.InfoStatus)
	fmt.Fprintf(&b, "- info_mode: `%s`\n", "repo-local-skill-metadata-card")
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
	fmt.Fprintf(&b, "- skill_info_id_sha256_12: `%s`\n", noneIfEmpty(infoIDHash))
	fmt.Fprintf(&b, "- skill_info_id_auto: `%t`\n", req.AutoInfoID)
	fmt.Fprintf(&b, "- requested_skill_sha256_12: `%s`\n", noneIfEmpty(requestedSkillHash))
	fmt.Fprintf(&b, "- normalized_skill_sha256_12: `%s`\n", noneIfEmpty(normalizedSkillHash))
	fmt.Fprintf(&b, "- requested_skill_bytes: `%d`\n", req.RequestedSkillBytes)
	fmt.Fprintf(&b, "- skill_source: `%s`\n", noneIfEmpty(req.SkillSource))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", info.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", info.EnabledSkills)
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", info.MatchedSkills)
	fmt.Fprintf(&b, "- validation_status: `%s`\n", info.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", info.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", info.ValidationWarnings)
	fmt.Fprintf(&b, "- matched_skill_names_sha256_12: `%s`\n", noneIfEmpty(matchedNamesHash))
	fmt.Fprintf(&b, "- matched_skill_paths_sha256_12: `%s`\n", noneIfEmpty(matchedPathsHash))
	fmt.Fprintf(&b, "- skill_info_index_sha256_12: `%s`\n", noneIfEmpty(infoIndexHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
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
	fmt.Fprintf(&b, "- raw_requested_skill_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_info_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_info_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing focused skill card from repo-local skill metadata. The source receipt keeps raw skill names, paths, descriptions, bodies, ids, and channel bodies out of band. The action does not call a model, install or update skills, contact registries, run installers, mutate repository files, execute tools, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read skill-info cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent skill-info cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate skill-info notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSkillInfoNotificationBody(opts ChannelSkillInfoOptions, report ChannelSkillInfoReport, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill info\n\n")
	fmt.Fprintf(&b, "Skill info status: %s\n", report.InfoStatus)
	fmt.Fprintf(&b, "Requested skill hash: %s\n", report.RequestedSkillHash)
	fmt.Fprintf(&b, "Normalized skill hash: %s\n", report.NormalizedHash)
	fmt.Fprintf(&b, "Available skills: %d\n", report.AvailableSkills)
	fmt.Fprintf(&b, "Enabled skills: %d\n", report.EnabledSkills)
	fmt.Fprintf(&b, "Matched skills: %d\n", report.MatchedSkills)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarnings)
	fmt.Fprintf(&b, "Skill info id hash: %s\n", shortDocumentHash(opts.InfoID))
	b.WriteString("\nSkills:\n")
	if len(report.Skills) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range report.Skills {
			fmt.Fprintf(&b, "- skill_name=%s path=%s folder=%s enabled=%t disabled_by_config=%t blocked_by_allowlist=%t selected_for_this_turn=%t always=%t frontmatter=%t description_present=%t bytes=%d lines=%d sha256_12=%s requires_env=%d requires_bins=%d missing_env=%d missing_bins=%d\n",
				skill.Name,
				skill.Path,
				skillFolderName(skill.Path),
				skillIsEnabled(skill),
				skill.DisabledByConfig,
				skill.BlockedByAllowlist,
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
		}
	}
	b.WriteString("\nRaw skill bodies, skill descriptions, channel bodies, issue bodies, comment bodies, prompts, tool outputs, and raw requested skill text are not included. Skill install: not performed by this action. Skill update: not performed by this action. Registry contact: not performed by this action. Installer scripts: not run by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSkillInfoActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSkillInfoActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSkillInfoIssueTarget(ev Event, req *ChannelSkillInfoActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill info requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSkillInfoOptions(opts ChannelSkillInfoOptions) ChannelSkillInfoOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.InfoID = cleanChannelSkillInfoID(opts.InfoID)
	opts.RequestedSkill = cleanChannelSkillInfoName(opts.RequestedSkill)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSkillInfoRoute(cfg Config, opts ChannelSkillInfoOptions) (ChannelSkillInfoOptions, error) {
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
		Body:      "GitClaw channel skill info.",
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

func validateChannelSkillInfoOptions(opts ChannelSkillInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing skill info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid skill info id %q", opts.InfoID)
	}
	if cleanChannelSkillInfoName(opts.RequestedSkill) == "" {
		return fmt.Errorf("missing requested skill")
	}
	return nil
}

func validateChannelSkillInfoActionRequestOptions(opts ChannelSkillInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing skill info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid skill info id %q", opts.InfoID)
	}
	if cleanChannelSkillInfoName(opts.RequestedSkill) == "" {
		return fmt.Errorf("missing requested skill")
	}
	return nil
}

func cleanChannelSkillInfoSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSkillInfoID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSkillInfoName(value string) string {
	value = cleanSkillLookupName(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func parseChannelSkillInfoTrailingSkill(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "skill:") || strings.HasPrefix(lower, "name:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelSkillInfoName(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelSkillInfoSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-skill-info-source-%s", eventID(ev))
}

func autoChannelSkillInfoID(ev Event, opts ChannelSkillInfoOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedSkill}, "|")
	return cleanChannelSkillInfoID(fmt.Sprintf("skill-info-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSkillInfoNotifyMessageID(ev Event, infoID string) string {
	seed := strings.Join([]string{eventID(ev), infoID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-info-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelSkillInfoNames(skills []SkillSummary) []string {
	var names []string
	for _, skill := range skills {
		if strings.TrimSpace(skill.Name) != "" {
			names = append(names, skill.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelSkillInfoPaths(skills []SkillSummary) []string {
	var paths []string
	for _, skill := range skills {
		if strings.TrimSpace(skill.Path) != "" {
			paths = append(paths, skill.Path)
		}
	}
	return uniqueSortedStrings(paths)
}

func channelSkillInfoIndex(report ChannelSkillInfoReport, repoContext RepoContext) string {
	var lines []string
	for _, skill := range report.Skills {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%t|%t|%t|%t|%t|%t|%d|%d|%d|%d|%s",
			skill.Name,
			skill.Path,
			skill.SHA,
			skillIsEnabled(skill),
			skill.DisabledByConfig,
			skill.BlockedByAllowlist,
			skillSelectedForTurn(repoContext, skill),
			skill.Always,
			skill.FrontmatterPresent,
			len(skill.RequiredEnv),
			len(skill.RequiredBins),
			len(skill.MissingEnv),
			len(skill.MissingBins),
			report.InfoStatus,
		))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}
