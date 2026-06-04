package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelMissionControlOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MissionID         string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelMissionControlResult struct {
	Notification  ChannelSendResult
	RouteName     string
	RouteHash     string
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	MissionIDHash string
	LaneHash      string
	NoteHash      string
	BodyHash      string
	LoopHash      string
	LoopStepCount int
	CommandCount  int
}

type ChannelMissionControlActionRequest struct {
	Options             ChannelMissionControlOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoMissionID       bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	MissionIDHash       string
	LaneSHA             string
	LaneBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	LoopSHA             string
	LoopStepCount       int
	CommandCount        int
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type channelMissionControlStep struct {
	Phase       string
	Surface     string
	Instruction string
	Command     string
}

func IsChannelMissionControlActionRequest(ev Event, cfg Config) bool {
	return isChannelMissionControlActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelMissionControlActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelMissionControlSubcommand(fields[1]) {
	case "mission-control", "control-loop", "ops-loop", "flight-plan", "flight-deck", "operator-loop", "next-loop", "openclaw-loop", "hermes-loop", "review-loop":
		return true
	default:
		return false
	}
}

func BuildChannelMissionControlActionRequest(ev Event, cfg Config) (ChannelMissionControlActionRequest, error) {
	fields, trailing, ok := channelMissionControlActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelMissionControlActionRequest{}, fmt.Errorf("missing channel mission-control command")
	}
	req := ChannelMissionControlActionRequest{
		Options: ChannelMissionControlOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Lane:              defaultChannelMissionControlLaneForSubcommand(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelMissionControlSubcommand(fields[1]),
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
				return ChannelMissionControlActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--mission-id", "--loop-id", "--control-id", "--flight-id", "--id":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MissionID = cleanChannelMissionControlID(fields[i+1])
			i++
		case "--lane", "--scope", "--for", "--theme":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMissionControlActionRequest{}, fmt.Errorf("unknown channel mission-control argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelMissionControlIssueTargetIfPresent(ev, &req)
	if err := applyChannelMissionControlPositionals(&req, positional); err != nil {
		return ChannelMissionControlActionRequest{}, err
	}
	if err := applyChannelMissionControlIssueTarget(ev, &req); err != nil {
		return ChannelMissionControlActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelMissionControlTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelMissionControlSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MissionID) == "" {
		req.Options.MissionID = autoChannelMissionControlID(ev, req.Options)
		req.AutoMissionID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMissionControlNotifyMessageID(ev, req.Options.MissionID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMissionControlOptions(req.Options)
	if err := validateChannelMissionControlActionRequestOptions(req.Options); err != nil {
		return ChannelMissionControlActionRequest{}, err
	}
	steps := channelMissionControlStepsForLane(req.Options.Lane)
	body := renderChannelMissionControlNotificationBody(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MissionIDHash = shortDocumentHash(req.Options.MissionID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.LoopSHA = shortDocumentHash(renderChannelMissionControlLoopFingerprint(steps))
	req.LoopStepCount = len(steps)
	req.CommandCount = len(steps)
	req.NotificationBodySHA = shortDocumentHash(body)
	req.NotificationBytes = len(body)
	req.NotificationLines = lineCount(body)
	return req, nil
}

func RunChannelMissionControl(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMissionControlOptions) (ChannelMissionControlResult, error) {
	opts = normalizeChannelMissionControlOptions(opts)
	var err error
	opts, err = applyChannelMissionControlRoute(cfg, opts)
	if err != nil {
		return ChannelMissionControlResult{}, err
	}
	if err := validateChannelMissionControlOptions(opts); err != nil {
		return ChannelMissionControlResult{}, err
	}
	body := renderChannelMissionControlNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelMissionControlResult{}, fmt.Errorf("queue channel mission-control notification: %w", err)
	}
	steps := channelMissionControlStepsForLane(opts.Lane)
	return ChannelMissionControlResult{
		Notification:  notification,
		RouteName:     opts.Route,
		RouteHash:     channelRouteHash(opts.Route),
		Channel:       opts.Channel,
		ThreadHash:    shortDocumentHash(opts.ThreadID),
		MessageHash:   shortDocumentHash(opts.SourceMessageID),
		NotifyHash:    shortDocumentHash(opts.NotifyMessageID),
		MissionIDHash: shortDocumentHash(opts.MissionID),
		LaneHash:      shortDocumentHash(opts.Lane),
		NoteHash:      shortDocumentHash(opts.Note),
		BodyHash:      shortDocumentHash(body),
		LoopHash:      shortDocumentHash(renderChannelMissionControlLoopFingerprint(steps)),
		LoopStepCount: len(steps),
		CommandCount:  len(steps),
	}, nil
}

func RenderChannelMissionControlActionReport(ev Event, req ChannelMissionControlActionRequest, result ChannelMissionControlResult) string {
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
	missionIDHash := firstNonEmpty(result.MissionIDHash, req.MissionIDHash)
	laneHash := firstNonEmpty(result.LaneHash, req.LaneSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	loopHash := firstNonEmpty(result.LoopHash, req.LoopSHA)
	loopStepCount := result.LoopStepCount
	if loopStepCount == 0 {
		loopStepCount = req.LoopStepCount
	}
	commandCount := result.CommandCount
	if commandCount == 0 {
		commandCount = req.CommandCount
	}
	notificationBytes := req.NotificationBytes
	notificationLines := req.NotificationLines
	var b strings.Builder
	b.WriteString("## GitClaw Channel Mission Control Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_mission_control_status: `%s`\n", status)
	fmt.Fprintf(&b, "- mission_control_card_mode: `%s`\n", "bounded-operating-loop")
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
	fmt.Fprintf(&b, "- mission_id_sha256_12: `%s`\n", noneIfEmpty(missionIDHash))
	fmt.Fprintf(&b, "- mission_id_auto: `%t`\n", req.AutoMissionID)
	fmt.Fprintf(&b, "- mission_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- mission_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- mission_step_count: `%d`\n", loopStepCount)
	fmt.Fprintf(&b, "- mission_command_count: `%d`\n", commandCount)
	fmt.Fprintf(&b, "- mission_step_sha256_12: `%s`\n", noneIfEmpty(loopHash))
	fmt.Fprintf(&b, "- mission_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- mission_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- mission_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- mission_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- dynamic_loop_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- live_source_browse_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- artifact_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- schedule_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mission_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mission_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mission_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mission_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mission_commands_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_mission_control_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing operating-loop card on the canonical channel issue. The provider card can show bounded phases and copyable next commands, while this source receipt keeps raw ids, lane names, notes, loop text, command text, channel bodies, issue bodies, comments, prompts, tool outputs, and provider message ids out of band. The action uses a static deck and does not call a model, execute commands, install skills, execute tools, read backup payloads, read soul bodies, write memory, fetch sources, browse live sources, create artifact issues, call provider APIs, mutate workflows, change policy, create schedules, use external randomness, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read mission-control cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent mission-control cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate mission-control cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelMissionControlActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelMissionControlActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelMissionControlIssueTarget(ev Event, req *ChannelMissionControlActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel mission-control requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelMissionControlIssueTargetIfPresent(ev Event, req *ChannelMissionControlActionRequest) {
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

func applyChannelMissionControlPositionals(req *ChannelMissionControlActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	defaultLane := defaultChannelMissionControlLaneForSubcommand(req.Subcommand)
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Lane == "" || req.Options.Lane == defaultLane {
				req.Options.Lane = value
				continue
			}
			return fmt.Errorf("unexpected channel mission-control argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == defaultLane {
			req.Options.Lane = value
			continue
		}
		return fmt.Errorf("unexpected channel mission-control argument %q", value)
	}
	return nil
}

func normalizeChannelMissionControlOptions(opts ChannelMissionControlOptions) ChannelMissionControlOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MissionID = cleanChannelMissionControlID(opts.MissionID)
	opts.Lane = cleanChannelMissionControlLane(opts.Lane)
	opts.Note = cleanChannelMissionControlNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelMissionControlRoute(cfg Config, opts ChannelMissionControlOptions) (ChannelMissionControlOptions, error) {
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
		Body:      "GitClaw channel mission control.",
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

func validateChannelMissionControlOptions(opts ChannelMissionControlOptions) error {
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
	if opts.MissionID == "" {
		return fmt.Errorf("missing mission-control id")
	}
	if !skillNamePattern.MatchString(opts.MissionID) {
		return fmt.Errorf("invalid mission-control id %q", opts.MissionID)
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing mission-control lane")
	}
	if len(channelMissionControlStepsForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported mission-control lane %q", opts.Lane)
	}
	return nil
}

func validateChannelMissionControlActionRequestOptions(opts ChannelMissionControlOptions) error {
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
	if opts.MissionID == "" {
		return fmt.Errorf("missing mission-control id")
	}
	if !skillNamePattern.MatchString(opts.MissionID) {
		return fmt.Errorf("invalid mission-control id %q", opts.MissionID)
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing mission-control lane")
	}
	if len(channelMissionControlStepsForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported mission-control lane %q", opts.Lane)
	}
	return nil
}

func cleanChannelMissionControlSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.NewReplacer("_", "-", " ", "-").Replace(value)
}

func cleanChannelMissionControlID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelMissionControlLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "all", "general", "capability", "capabilities", "ops", "operator":
		return "all"
	case "research", "openclaw", "hermes", "landscape", "patterns", "pattern":
		return "research"
	case "skill", "skills", "skill-use", "skill-map":
		return "skills"
	case "tool", "tools", "tool-use", "tool-map":
		return "tools"
	case "soul", "souls", "profile", "profiles", "identity", "context", "authority":
		return "soul"
	case "memory", "memories", "durable-memory", "remember":
		return "memory"
	case "backup", "backups", "recovery", "restore", "rollback":
		return "backups"
	case "channel", "channels", "provider", "providers", "slack", "telegram", "gateway", "gateways":
		return "channels"
	case "launch", "release", "ship", "shipping":
		return "launch"
	case "fun", "play", "spark", "sparks", "social":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelMissionControlNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelMissionControlTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelMissionControlTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelMissionControlNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func shouldSkipChannelMissionControlTrailingLine(line string) bool {
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.Contains(lower, "do not include") || strings.Contains(lower, "do not leak") || strings.Contains(lower, "hidden token")
}

func defaultChannelMissionControlLaneForSubcommand(subcommand string) string {
	switch cleanChannelMissionControlSubcommand(subcommand) {
	case "openclaw-loop", "hermes-loop":
		return "research"
	default:
		return "all"
	}
}

func autoChannelMissionControlSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-mission-control-source-%s", eventID(ev))
}

func autoChannelMissionControlID(ev Event, opts ChannelMissionControlOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return fmt.Sprintf("mission-control-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMissionControlNotifyMessageID(ev Event, missionID string) string {
	seed := strings.Join([]string{eventID(ev), missionID}, "|")
	return fmt.Sprintf("gitclaw-channel-mission-control-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelMissionControlNotificationBody(opts ChannelMissionControlOptions) string {
	steps := channelMissionControlStepsForLane(opts.Lane)
	var b strings.Builder
	b.WriteString("GitClaw channel mission control.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	fmt.Fprintf(&b, "Control loop: %s\n", channelMissionControlPosture(opts.Lane))
	b.WriteString("Loop steps:\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s [%s] - %s\n", i+1, step.Phase, step.Surface, step.Instruction)
		fmt.Fprintf(&b, "   Next: `%s`\n", step.Command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nMission hash: %s\n", shortDocumentHash(opts.MissionID))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("Mission persistence: advisory only; no durable channel state changed.\n")
	b.WriteString("\nMission source: bounded GitHub channel action deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Dynamic loop generation: not performed by this action.\n")
	b.WriteString("External randomness: not used by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Memory write: not performed by this action.\n")
	b.WriteString("Source fetch: not performed by this action.\n")
	b.WriteString("Live source browse: not performed by this action.\n")
	b.WriteString("Artifact issue creation: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Policy mutation: not performed by this action.\n")
	b.WriteString("Schedule creation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelMissionControlPosture(lane string) string {
	switch cleanChannelMissionControlLane(lane) {
	case "research":
		return "Turn OpenClaw and Hermes research into one review-first GitClaw move."
	case "skills":
		return "Discover, spotlight, then rehearse before installing or using skills."
	case "tools":
		return "Name the evidence gate before any tool execution."
	case "soul":
		return "Keep high-authority context explicit before writing or promoting it."
	case "memory":
		return "Recall first, propose second, write only after review."
	case "backups":
		return "Verify recovery metadata before reading payloads or restoring files."
	case "channels":
		return "Keep provider chat lively while GitHub stays the source ledger."
	case "launch":
		return "Move from readiness signal to rollback-aware status update."
	case "fun":
		return "Keep momentum playful while preserving a useful audit trail."
	default:
		return "Select one safe surface, gate it, rehearse it, then return to GitHub."
	}
}

func channelMissionControlStepsForLane(lane string) []channelMissionControlStep {
	switch cleanChannelMissionControlLane(lane) {
	case "research":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "research", Instruction: "review the static OpenClaw/Hermes source and pattern catalog", Command: "@gitclaw /research catalog"},
			{Phase: "Gate", Surface: "research", Instruction: "draw one provider-facing source or pattern card", Command: "@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "research", Instruction: "convert the selected pattern into a safe command path", Command: "@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "research", Instruction: "return with a safe next action without executing tools", Command: "@gitclaw /channels compass research --compass-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "skills":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "skills", Instruction: "inspect skill posture without loading bodies", Command: "@gitclaw /channels skills --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "skills", Instruction: "spotlight one skill without installing it", Command: "@gitclaw /channels skill-spotlight repo-reader --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "skills", Instruction: "map safe use before any install or write", Command: "@gitclaw /channels skill-map repo-reader --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "skills", Instruction: "open review only if the thread needs durable work", Command: "@gitclaw /channels propose-skill repo-reader --message-id <id> --notify-message-id <id>"},
		}
	case "tools":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "tools", Instruction: "inspect tool posture without execution", Command: "@gitclaw /channels tools --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "tools", Instruction: "spotlight one safe tool contract", Command: "@gitclaw /channels tool-spotlight search_files --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "tools", Instruction: "map approval before any run", Command: "@gitclaw /channels tool-map search_files --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "tools", Instruction: "request a reviewed tool run only after evidence is clear", Command: "@gitclaw /channels request-run search_files --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "soul":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "soul", Instruction: "inspect high-authority context without raw bodies", Command: "@gitclaw /channels soul-status --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "soul", Instruction: "spotlight one authority surface without writing soul", Command: "@gitclaw /channels soul-spotlight identity --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "soul", Instruction: "review persistent-state risk before promotion", Command: "@gitclaw /channels soul-risk --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "soul", Instruction: "open a proposal only when the change is explicit", Command: "@gitclaw /channels propose-soul --target soul --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "memory":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "memory", Instruction: "inspect durable-memory shape without writes", Command: "@gitclaw /channels memory-status --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "memory", Instruction: "search memory with body-free metadata", Command: "@gitclaw /channels memory-search handoff --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "memory", Instruction: "turn useful recall into a review surface", Command: "@gitclaw /channels propose-memory --memory-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "memory", Instruction: "rehearse before promoting memory", Command: "@gitclaw /channels rehearse-memory --target long-term --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "backups":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "backups", Instruction: "inspect backup status without reading payloads", Command: "@gitclaw /channels backup --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "backups", Instruction: "check continuity and freshness from metadata", Command: "@gitclaw /channels backup-freshness --freshness-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "backups", Instruction: "map a safe recovery sequence without restores", Command: "@gitclaw /channels recovery-map issue --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "backups", Instruction: "open restore review only with human-readable scope", Command: "@gitclaw /channels restore-request --request-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "channels":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "channels", Instruction: "show provider availability without socket probes", Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "channels", Instruction: "offer a compact command menu", Command: "@gitclaw /channels palette fun --palette-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "channels", Instruction: "make the thread easier to enter", Command: "@gitclaw /channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "channels", Instruction: "search the mirrored transcript when context gets fuzzy", Command: "@gitclaw /channels session-search handoff --message-id <id> --notify-message-id <id>"},
		}
	case "launch":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "launch", Instruction: "set the channel posture for launch review", Command: "@gitclaw /channels mode launch --mode-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "launch", Instruction: "ask the thread what must be true before shipping", Command: "@gitclaw /channels warmup launch --warmup-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "launch", Instruction: "surface rollback confidence before release", Command: "@gitclaw /channels recovery-map incident --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "launch", Instruction: "queue a provider-facing attention nudge only after review", Command: "@gitclaw /channels nudge release-captain --nudge-id <id> --message-id <id> --notify-message-id <id> --tone gentle"},
		}
	case "fun":
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "fun", Instruction: "make the thread easy to enter", Command: "@gitclaw /channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "fun", Instruction: "turn a loose idea into one experiment", Command: "@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "fun", Instruction: "roll a bounded prompt-game card", Command: "@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "fun", Instruction: "leave a tiny but searchable GitHub breadcrumb", Command: "@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>"},
		}
	default:
		return []channelMissionControlStep{
			{Phase: "Signal", Surface: "all", Instruction: "see the thread and provider bridge posture", Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>"},
			{Phase: "Gate", Surface: "all", Instruction: "choose a capability surface without executing anything", Command: "@gitclaw /channels constellation all --constellation-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Rehearse", Surface: "all", Instruction: "get a safe next step", Command: "@gitclaw /channels compass all --compass-id <id> --message-id <id> --notify-message-id <id>"},
			{Phase: "Commit", Surface: "all", Instruction: "ask for repo-aware guidance without tool execution", Command: "@gitclaw /channels coach all --coach-id <id> --message-id <id> --notify-message-id <id>"},
		}
	}
}

func renderChannelMissionControlLoopFingerprint(steps []channelMissionControlStep) string {
	var parts []string
	for _, step := range steps {
		parts = append(parts, strings.Join([]string{step.Phase, step.Surface, step.Instruction, step.Command}, "|"))
	}
	return strings.Join(parts, "\n")
}
