package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelCoachOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	CoachID           string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelCoachResult struct {
	Notification                  ChannelSendResult
	RouteName                     string
	RouteHash                     string
	Channel                       string
	ThreadHash                    string
	MessageHash                   string
	NotifyHash                    string
	CoachIDHash                   string
	LaneHash                      string
	NoteHash                      string
	BodyHash                      string
	AdviceHash                    string
	SnapshotHash                  string
	RecommendationCount           int
	AvailableSkills               int
	EnabledSkills                 int
	SelectedSkills                int
	SkillsMissingRequirements     int
	EnabledTools                  int
	ActiveToolOutputs             int
	ToolValidationStatus          string
	ToolRiskStatus                string
	SoulStatus                    string
	SoulValidationStatus          string
	SoulValidationErrors          int
	SoulValidationWarnings        int
	SoulRiskStatus                string
	SoulRiskFindings              int
	RequiredSoulEntries           int
	RequiredLoadedSoulEntries     int
	PromptVisibleSoulEntries      int
	RepositoryMutationRecommended bool
}

type ChannelCoachActionRequest struct {
	Options                       ChannelCoachOptions
	Command                       string
	Subcommand                    string
	AutoSourceMessageID           bool
	AutoNotifyMessageID           bool
	AutoCoachID                   bool
	TargetFromIssue               bool
	NoteSource                    string
	RequestedRouteHash            string
	RequestedThreadHash           string
	RequestedMsgHash              string
	NotifyMessageHash             string
	CoachIDHash                   string
	LaneSHA                       string
	LaneBytes                     int
	NoteSHA                       string
	NoteBytes                     int
	NoteLines                     int
	AdviceSHA                     string
	SnapshotSHA                   string
	RecommendationCount           int
	NotificationBodySHA           string
	AvailableSkills               int
	EnabledSkills                 int
	SelectedSkills                int
	SkillsMissingRequirements     int
	EnabledTools                  int
	ActiveToolOutputs             int
	ToolValidationStatus          string
	ToolRiskStatus                string
	SoulStatus                    string
	SoulValidationStatus          string
	SoulValidationErrors          int
	SoulValidationWarnings        int
	SoulRiskStatus                string
	SoulRiskFindings              int
	RequiredSoulEntries           int
	RequiredLoadedSoulEntries     int
	PromptVisibleSoulEntries      int
	RepositoryMutationRecommended bool
}

type channelCoachSnapshot struct {
	AvailableSkills               int
	EnabledSkills                 int
	SelectedSkills                int
	SkillsMissingRequirements     int
	EnabledTools                  int
	ActiveToolOutputs             int
	ToolValidationStatus          string
	ToolRiskStatus                string
	SoulStatus                    string
	SoulValidationStatus          string
	SoulValidationErrors          int
	SoulValidationWarnings        int
	SoulRiskStatus                string
	SoulRiskFindings              int
	RequiredSoulEntries           int
	RequiredLoadedSoulEntries     int
	PromptVisibleSoulEntries      int
	RepositoryMutationRecommended bool
	RecommendationCount           int
	AdviceHash                    string
	SnapshotHash                  string
}

type channelCoachRecommendation struct {
	Command string
	Reason  string
}

func IsChannelCoachActionRequest(ev Event, cfg Config) bool {
	return isChannelCoachActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelCoachActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "coach", "coach-me", "next", "next-step", "next-steps", "next-move", "advisor", "advise", "mentor":
		return true
	default:
		return false
	}
}

func BuildChannelCoachActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelCoachActionRequest, error) {
	fields, trailing, ok := channelCoachActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelCoachActionRequest{}, fmt.Errorf("missing channel coach command")
	}
	req := ChannelCoachActionRequest{
		Options: ChannelCoachOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Lane:              "all",
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
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
				return ChannelCoachActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--coach-id", "--advice-id", "--advisor-id", "--id":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.CoachID = cleanChannelCoachID(fields[i+1])
			i++
		case "--lane", "--focus", "--scope", "--for":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelCoachActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelCoachActionRequest{}, fmt.Errorf("unknown channel coach argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelCoachIssueTargetIfPresent(ev, &req)
	if err := applyChannelCoachPositionals(&req, positional); err != nil {
		return ChannelCoachActionRequest{}, err
	}
	if err := applyChannelCoachIssueTarget(ev, &req); err != nil {
		return ChannelCoachActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelCoachTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelCoachSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.CoachID) == "" {
		req.Options.CoachID = autoChannelCoachID(ev, req.Options)
		req.AutoCoachID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelCoachNotifyMessageID(ev, req.Options.CoachID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelCoachOptions(req.Options)
	if err := validateChannelCoachActionRequestOptions(req.Options); err != nil {
		return ChannelCoachActionRequest{}, err
	}
	snapshot := buildChannelCoachSnapshot(cfg, req.Options, repoContext)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.CoachIDHash = shortDocumentHash(req.Options.CoachID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelCoachNotificationBody(req.Options, cfg, repoContext))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelCoach(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelCoachOptions, repoContext RepoContext) (ChannelCoachResult, error) {
	opts = normalizeChannelCoachOptions(opts)
	var err error
	opts, err = applyChannelCoachRoute(cfg, opts)
	if err != nil {
		return ChannelCoachResult{}, err
	}
	if err := validateChannelCoachOptions(opts); err != nil {
		return ChannelCoachResult{}, err
	}
	body := renderChannelCoachNotificationBody(opts, cfg, repoContext)
	snapshot := buildChannelCoachSnapshot(cfg, opts, repoContext)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelCoachResult{}, fmt.Errorf("queue channel coach notification: %w", err)
	}
	result := ChannelCoachResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		CoachIDHash:  shortDocumentHash(opts.CoachID),
		LaneHash:     shortDocumentHash(opts.Lane),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
		AdviceHash:   snapshot.AdviceHash,
		SnapshotHash: snapshot.SnapshotHash,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelCoachActionReport(ev Event, req ChannelCoachActionRequest, result ChannelCoachResult) string {
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
	coachIDHash := result.CoachIDHash
	if coachIDHash == "" {
		coachIDHash = req.CoachIDHash
	}
	laneHash := result.LaneHash
	if laneHash == "" {
		laneHash = req.LaneSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	adviceHash := result.AdviceHash
	if adviceHash == "" {
		adviceHash = req.AdviceSHA
	}
	snapshotHash := result.SnapshotHash
	if snapshotHash == "" {
		snapshotHash = req.SnapshotSHA
	}
	recommendationCount := nonzeroOrReq(result.RecommendationCount, req.RecommendationCount)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Coach Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_coach_status: `%s`\n", status)
	fmt.Fprintf(&b, "- coach_mode: `%s`\n", "repo-aware-channel-coach")
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
	fmt.Fprintf(&b, "- coach_id_sha256_12: `%s`\n", noneIfEmpty(coachIDHash))
	fmt.Fprintf(&b, "- coach_id_auto: `%t`\n", req.AutoCoachID)
	fmt.Fprintf(&b, "- coach_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- coach_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- coach_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- coach_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- coach_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- coach_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- coach_recommendation_count: `%d`\n", recommendationCount)
	fmt.Fprintf(&b, "- coach_advice_sha256_12: `%s`\n", noneIfEmpty(adviceHash))
	fmt.Fprintf(&b, "- coach_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", nonzeroOrReq(result.AvailableSkills, req.AvailableSkills))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", nonzeroOrReq(result.EnabledSkills, req.EnabledSkills))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", nonzeroOrReq(result.SelectedSkills, req.SelectedSkills))
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", nonzeroOrReq(result.SkillsMissingRequirements, req.SkillsMissingRequirements))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", nonzeroOrReq(result.EnabledTools, req.EnabledTools))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", nonzeroOrReq(result.ActiveToolOutputs, req.ActiveToolOutputs))
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", firstNonEmpty(result.ToolValidationStatus, req.ToolValidationStatus, "unknown"))
	fmt.Fprintf(&b, "- tool_risk_status: `%s`\n", firstNonEmpty(result.ToolRiskStatus, req.ToolRiskStatus, "unknown"))
	fmt.Fprintf(&b, "- soul_status: `%s`\n", firstNonEmpty(result.SoulStatus, req.SoulStatus, "unknown"))
	fmt.Fprintf(&b, "- soul_validation_status: `%s`\n", firstNonEmpty(result.SoulValidationStatus, req.SoulValidationStatus, "unknown"))
	fmt.Fprintf(&b, "- soul_validation_errors: `%d`\n", nonzeroOrReq(result.SoulValidationErrors, req.SoulValidationErrors))
	fmt.Fprintf(&b, "- soul_validation_warnings: `%d`\n", nonzeroOrReq(result.SoulValidationWarnings, req.SoulValidationWarnings))
	fmt.Fprintf(&b, "- soul_risk_status: `%s`\n", firstNonEmpty(result.SoulRiskStatus, req.SoulRiskStatus, "unknown"))
	fmt.Fprintf(&b, "- soul_risk_findings: `%d`\n", nonzeroOrReq(result.SoulRiskFindings, req.SoulRiskFindings))
	fmt.Fprintf(&b, "- required_soul_entries_loaded: `%d`\n", nonzeroOrReq(result.RequiredLoadedSoulEntries, req.RequiredLoadedSoulEntries))
	fmt.Fprintf(&b, "- required_soul_entries: `%d`\n", nonzeroOrReq(result.RequiredSoulEntries, req.RequiredSoulEntries))
	fmt.Fprintf(&b, "- prompt_visible_soul_entries: `%d`\n", nonzeroOrReq(result.PromptVisibleSoulEntries, req.PromptVisibleSoulEntries))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_recommended: `%t`\n", req.RepositoryMutationRecommended)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_coach_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_coach_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_coach_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_coach_recommendations_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_coach_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel coach card on the canonical channel issue. This gives Slack or Telegram a repo-aware next-move card from body-free skill, tool, and soul metadata while the source receipt keeps thread ids, message ids, coach ids, lanes, notes, recommendation text, and channel bodies out of band. The action does not call a model, execute commands, install skills, execute tools, read backup payloads, read soul bodies, mutate workflows, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read coach cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent coach cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate coach cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelCoachActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelCoachActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelCoachIssueTarget(ev Event, req *ChannelCoachActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel coach requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelCoachIssueTargetIfPresent(ev Event, req *ChannelCoachActionRequest) {
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

func applyChannelCoachPositionals(req *ChannelCoachActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Lane == "" || req.Options.Lane == "all" {
				req.Options.Lane = value
				continue
			}
			return fmt.Errorf("unexpected channel coach argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == "all" {
			req.Options.Lane = value
			continue
		}
		return fmt.Errorf("unexpected channel coach argument %q", value)
	}
	return nil
}

func normalizeChannelCoachOptions(opts ChannelCoachOptions) ChannelCoachOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.CoachID = cleanChannelCoachID(opts.CoachID)
	opts.Lane = cleanChannelCoachLane(opts.Lane)
	opts.Note = cleanChannelCoachNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelCoachRoute(cfg Config, opts ChannelCoachOptions) (ChannelCoachOptions, error) {
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
		Body:      "GitClaw channel coach.",
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

func validateChannelCoachOptions(opts ChannelCoachOptions) error {
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
	if opts.CoachID == "" {
		return fmt.Errorf("missing coach id")
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing coach lane")
	}
	return nil
}

func validateChannelCoachActionRequestOptions(opts ChannelCoachOptions) error {
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
	if opts.CoachID == "" {
		return fmt.Errorf("missing coach id")
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing coach lane")
	}
	return nil
}

func cleanChannelCoachID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelCoachLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	switch value {
	case "", "general", "everything":
		return "all"
	case "skill", "capability", "capabilities":
		return "skills"
	case "tool", "tooling":
		return "tools"
	case "identity", "context", "authority", "profile":
		return "soul"
	case "memory", "memories":
		return "memory"
	case "backup", "restore", "recovery":
		return "backups"
	case "channel", "chat":
		return "channels"
	case "play":
		return "fun"
	default:
		if len(value) > 32 {
			value = strings.Trim(value[:32], "-")
		}
		return value
	}
}

func cleanChannelCoachNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelCoachTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelCoachTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelCoachNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelCoachTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func autoChannelCoachSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-coach-source-%s", eventID(ev))
}

func autoChannelCoachID(ev Event, opts ChannelCoachOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return fmt.Sprintf("coach-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelCoachNotifyMessageID(ev Event, coachID string) string {
	seed := strings.Join([]string{eventID(ev), coachID}, "|")
	return fmt.Sprintf("gitclaw-channel-coach-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelCoachNotificationBody(opts ChannelCoachOptions, cfg Config, repoContext RepoContext) string {
	opts = normalizeChannelCoachOptions(opts)
	snapshot := buildChannelCoachSnapshot(cfg, opts, repoContext)
	recommendations := channelCoachRecommendationsForLane(opts.Lane, snapshot)
	var b strings.Builder
	b.WriteString("GitClaw channel coach.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	b.WriteString("Signals:\n")
	fmt.Fprintf(&b, "- Skills: available=%d enabled=%d selected=%d missing_requirements=%d\n", snapshot.AvailableSkills, snapshot.EnabledSkills, snapshot.SelectedSkills, snapshot.SkillsMissingRequirements)
	fmt.Fprintf(&b, "- Tools: enabled=%d active_outputs=%d validation=%s risk=%s\n", snapshot.EnabledTools, snapshot.ActiveToolOutputs, snapshot.ToolValidationStatus, snapshot.ToolRiskStatus)
	fmt.Fprintf(&b, "- Soul: status=%s validation=%s risk=%s required_loaded=%d/%d\n", snapshot.SoulStatus, snapshot.SoulValidationStatus, snapshot.SoulRiskStatus, snapshot.RequiredLoadedSoulEntries, snapshot.RequiredSoulEntries)
	b.WriteString("\nRecommended next moves:\n")
	for i, rec := range recommendations {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, rec.Command, rec.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Coach hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Recommendation hash: %s\n", snapshot.AdviceHash)
	b.WriteString("\nCoach source: current GitHub Actions checkout metadata.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelCoachSnapshot(cfg Config, opts ChannelCoachOptions, repoContext RepoContext) channelCoachSnapshot {
	toolSnapshot := buildChannelToolStatusSnapshot(cfg, repoContext)
	soulSnapshot := buildChannelSoulStatusSnapshot(repoContext)
	snapshot := channelCoachSnapshot{
		AvailableSkills:           availableSkillCount(repoContext),
		EnabledSkills:             enabledSkillCount(repoContext.SkillSummaries),
		SelectedSkills:            len(repoContext.Skills),
		SkillsMissingRequirements: missingRequirementSkillCount(repoContext.SkillSummaries),
		EnabledTools:              toolSnapshot.EnabledTools,
		ActiveToolOutputs:         toolSnapshot.ActiveToolOutputs,
		ToolValidationStatus:      firstNonEmpty(toolSnapshot.ToolValidationStatus, "unknown"),
		ToolRiskStatus:            firstNonEmpty(toolSnapshot.ToolRiskStatus, "unknown"),
		SoulStatus:                firstNonEmpty(soulSnapshot.SoulStatus, "unknown"),
		SoulValidationStatus:      firstNonEmpty(soulSnapshot.ValidationStatus, "unknown"),
		SoulValidationErrors:      soulSnapshot.ValidationErrors,
		SoulValidationWarnings:    soulSnapshot.ValidationWarnings,
		SoulRiskStatus:            firstNonEmpty(soulSnapshot.RiskStatus, "unknown"),
		SoulRiskFindings:          soulSnapshot.RiskFindings,
		RequiredSoulEntries:       soulSnapshot.RequiredSnapshotEntries,
		RequiredLoadedSoulEntries: soulSnapshot.RequiredLoadedEntries,
		PromptVisibleSoulEntries:  soulSnapshot.PromptVisibleEntries,
	}
	recommendations := channelCoachRecommendationsForLane(opts.Lane, snapshot)
	snapshot.RecommendationCount = len(recommendations)
	snapshot.AdviceHash = shortDocumentHash(channelCoachRecommendationManifest(recommendations))
	snapshot.SnapshotHash = shortDocumentHash(channelCoachSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelCoachRecommendationsForLane(lane string, snapshot channelCoachSnapshot) []channelCoachRecommendation {
	switch cleanChannelCoachLane(lane) {
	case "skills":
		return []channelCoachRecommendation{
			{Command: "/channels skills --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("see enabled and selected skill counts before choosing a capability (%d enabled)", snapshot.EnabledSkills)},
			{Command: "/channels skill-search <query> --message-id <id> --notify-message-id <id>", Reason: "find candidate skill metadata without loading full skill bodies"},
			{Command: "/channels skill-info <skill> --message-id <id> --notify-message-id <id>", Reason: "inspect one focused skill card before proposing or rehearsing changes"},
			{Command: "/channels propose-skill <name> --message-id <id>", Reason: "open a reviewed skill proposal issue when the next move is durable"},
		}
	case "tools":
		return []channelCoachRecommendation{
			{Command: "/channels tools --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("check tool posture and risk before requesting execution (risk=%s)", snapshot.ToolRiskStatus)},
			{Command: "/channels tool-search <query> --message-id <id> --notify-message-id <id>", Reason: "find a matching tool capability without exposing raw schemas"},
			{Command: "/channels tool-info <tool> --message-id <id> --notify-message-id <id>", Reason: "inspect one tool card before opening approval or run review"},
			{Command: "/channels approval-plan <tool> --id <id> --message-id <id>", Reason: "turn a tool question into a reviewed approval-plan issue"},
		}
	case "soul":
		return []channelCoachRecommendation{
			{Command: "/channels soul-status --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("check high-authority context health before proposing edits (status=%s)", snapshot.SoulStatus)},
			{Command: "/channels soul-risk --message-id <id> --notify-message-id <id>", Reason: "review persistent-state risks without exposing raw soul bodies"},
			{Command: "/channels soul-search <query> --message-id <id> --notify-message-id <id>", Reason: "recall high-authority context metadata without printing raw query or bodies"},
			{Command: "/channels propose-soul --target soul --id <id> --message-id <id>", Reason: "open a reviewed proposal issue for durable soul changes"},
		}
	case "memory":
		return []channelCoachRecommendation{
			{Command: "/channels memory-status --message-id <id> --notify-message-id <id>", Reason: "check durable-memory posture before storing new context"},
			{Command: "/channels memory-search <query> --message-id <id> --notify-message-id <id>", Reason: "recall memory metadata without exposing raw memory bodies"},
			{Command: "/channels memory-note --note-id <id> --target <target> --message-id <id>", Reason: "preserve a channel observation as a reviewed GitHub issue"},
			{Command: "/channels propose-memory --target <target> --id <id> --message-id <id>", Reason: "promote durable memory through an explicit review issue"},
		}
	case "backups":
		return []channelCoachRecommendation{
			{Command: "/channels backup --message-id <id> --notify-message-id <id>", Reason: "check backup readiness without reading backup payloads"},
			{Command: "/channels backup-search <query> --message-id <id> --notify-message-id <id>", Reason: "find backup metadata before asking for restore work"},
			{Command: "/channels backup-info <issue> --message-id <id> --notify-message-id <id>", Reason: "inspect one backup record without restoring files"},
			{Command: "/channels backup-note --note-id <id> --scope <scope> --message-id <id>", Reason: "capture recovery context as a reviewable issue"},
		}
	case "channels":
		return []channelCoachRecommendation{
			{Command: "/channels palette all --message-id <id> --notify-message-id <id>", Reason: "surface the available chat-native affordances"},
			{Command: "/channels compass all --message-id <id> --notify-message-id <id>", Reason: "get safe next steps without executing anything"},
			{Command: "/channels status --message-id <id> --status-id <id> --state working", Reason: "queue a visible progress update for the provider thread"},
			{Command: "/channels open-loop --loop-id <id> --message-id <id>", Reason: "turn unresolved channel context into a durable GitHub issue"},
		}
	case "fun":
		return []channelCoachRecommendation{
			{Command: "/channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>", Reason: "start a lightweight check-in without creating durable state"},
			{Command: "/channels haiku fun --haiku-id <id> --message-id <id> --notify-message-id <id>", Reason: "send a deterministic tiny poem card"},
			{Command: "/channels oracle --choose-id <id> --message-id <id> --notify-message-id <id>", Reason: "use the bounded deterministic picker for a playful prompt"},
			{Command: "/channels choose --message-id <id> --notify-message-id <id>", Reason: "pick among visible options without external randomness"},
		}
	default:
		return []channelCoachRecommendation{
			{Command: "/channels skills --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("start with skill posture (%d enabled, %d selected)", snapshot.EnabledSkills, snapshot.SelectedSkills)},
			{Command: "/channels tools --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("check tool readiness before run requests (validation=%s)", snapshot.ToolValidationStatus)},
			{Command: "/channels soul-status --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("confirm high-authority context health (risk=%s)", snapshot.SoulRiskStatus)},
			{Command: "/channels compass all --message-id <id> --notify-message-id <id>", Reason: "fall back to static safe next steps when the lane is unclear"},
		}
	}
}

func channelCoachRecommendationManifest(recommendations []channelCoachRecommendation) string {
	var lines []string
	for _, rec := range recommendations {
		lines = append(lines, strings.Join([]string{rec.Command, rec.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}

func channelCoachSnapshotManifest(opts ChannelCoachOptions, snapshot channelCoachSnapshot) string {
	return fmt.Sprintf(
		"lane=%s\nskills=%d/%d/%d/%d\ntools=%d/%d/%s/%s\nsoul=%s/%s/%d/%d/%s/%d/%d/%d/%d\nadvice=%s/%d",
		cleanChannelCoachLane(opts.Lane),
		snapshot.AvailableSkills,
		snapshot.EnabledSkills,
		snapshot.SelectedSkills,
		snapshot.SkillsMissingRequirements,
		snapshot.EnabledTools,
		snapshot.ActiveToolOutputs,
		snapshot.ToolValidationStatus,
		snapshot.ToolRiskStatus,
		snapshot.SoulStatus,
		snapshot.SoulValidationStatus,
		snapshot.SoulValidationErrors,
		snapshot.SoulValidationWarnings,
		snapshot.SoulRiskStatus,
		snapshot.SoulRiskFindings,
		snapshot.RequiredLoadedSoulEntries,
		snapshot.RequiredSoulEntries,
		snapshot.PromptVisibleSoulEntries,
		snapshot.AdviceHash,
		snapshot.RecommendationCount,
	)
}

func (r *ChannelCoachActionRequest) applySnapshot(snapshot channelCoachSnapshot) {
	r.AvailableSkills = snapshot.AvailableSkills
	r.EnabledSkills = snapshot.EnabledSkills
	r.SelectedSkills = snapshot.SelectedSkills
	r.SkillsMissingRequirements = snapshot.SkillsMissingRequirements
	r.EnabledTools = snapshot.EnabledTools
	r.ActiveToolOutputs = snapshot.ActiveToolOutputs
	r.ToolValidationStatus = snapshot.ToolValidationStatus
	r.ToolRiskStatus = snapshot.ToolRiskStatus
	r.SoulStatus = snapshot.SoulStatus
	r.SoulValidationStatus = snapshot.SoulValidationStatus
	r.SoulValidationErrors = snapshot.SoulValidationErrors
	r.SoulValidationWarnings = snapshot.SoulValidationWarnings
	r.SoulRiskStatus = snapshot.SoulRiskStatus
	r.SoulRiskFindings = snapshot.SoulRiskFindings
	r.RequiredSoulEntries = snapshot.RequiredSoulEntries
	r.RequiredLoadedSoulEntries = snapshot.RequiredLoadedSoulEntries
	r.PromptVisibleSoulEntries = snapshot.PromptVisibleSoulEntries
	r.RepositoryMutationRecommended = snapshot.RepositoryMutationRecommended
	r.RecommendationCount = snapshot.RecommendationCount
	r.AdviceSHA = snapshot.AdviceHash
	r.SnapshotSHA = snapshot.SnapshotHash
}

func (r *ChannelCoachResult) applySnapshot(snapshot channelCoachSnapshot) {
	r.AvailableSkills = snapshot.AvailableSkills
	r.EnabledSkills = snapshot.EnabledSkills
	r.SelectedSkills = snapshot.SelectedSkills
	r.SkillsMissingRequirements = snapshot.SkillsMissingRequirements
	r.EnabledTools = snapshot.EnabledTools
	r.ActiveToolOutputs = snapshot.ActiveToolOutputs
	r.ToolValidationStatus = snapshot.ToolValidationStatus
	r.ToolRiskStatus = snapshot.ToolRiskStatus
	r.SoulStatus = snapshot.SoulStatus
	r.SoulValidationStatus = snapshot.SoulValidationStatus
	r.SoulValidationErrors = snapshot.SoulValidationErrors
	r.SoulValidationWarnings = snapshot.SoulValidationWarnings
	r.SoulRiskStatus = snapshot.SoulRiskStatus
	r.SoulRiskFindings = snapshot.SoulRiskFindings
	r.RequiredSoulEntries = snapshot.RequiredSoulEntries
	r.RequiredLoadedSoulEntries = snapshot.RequiredLoadedSoulEntries
	r.PromptVisibleSoulEntries = snapshot.PromptVisibleSoulEntries
	r.RepositoryMutationRecommended = snapshot.RepositoryMutationRecommended
	r.RecommendationCount = snapshot.RecommendationCount
}
