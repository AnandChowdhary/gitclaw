package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelConstellationOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ConstellationID   string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelConstellationResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	ConstellationIDHash string
	LaneHash            string
	NoteHash            string
	BodyHash            string
	StarHash            string
	StarCount           int
	CommandCount        int
}

type ChannelConstellationActionRequest struct {
	Options                 ChannelConstellationOptions
	Command                 string
	Subcommand              string
	AutoSourceMessageID     bool
	AutoNotifyMessageID     bool
	AutoConstellationID     bool
	TargetFromIssue         bool
	NoteSource              string
	RequestedRouteHash      string
	RequestedThreadHash     string
	RequestedMsgHash        string
	NotifyMessageHash       string
	ConstellationIDHash     string
	LaneSHA                 string
	LaneBytes               int
	NoteSHA                 string
	NoteBytes               int
	NoteLines               int
	StarSHA                 string
	StarCount               int
	CommandCount            int
	NotificationBodySHA     string
	NotificationBodyBytes   int
	NotificationBodyLineCnt int
}

type channelConstellationStar struct {
	Name    string
	Surface string
	Signal  string
	Command string
}

func IsChannelConstellationActionRequest(ev Event, cfg Config) bool {
	return isChannelConstellationActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelConstellationActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelConstellationSubcommand(fields[1]) {
	case "constellation", "constellations", "star-map", "stars", "north-star", "capability-constellation", "capability-stars", "research-constellation", "openclaw-constellation", "hermes-constellation":
		return true
	default:
		return false
	}
}

func BuildChannelConstellationActionRequest(ev Event, cfg Config) (ChannelConstellationActionRequest, error) {
	fields, trailing, ok := channelConstellationActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelConstellationActionRequest{}, fmt.Errorf("missing channel constellation command")
	}
	req := ChannelConstellationActionRequest{
		Options: ChannelConstellationOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Lane:              defaultChannelConstellationLaneForSubcommand(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelConstellationSubcommand(fields[1]),
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
				return ChannelConstellationActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--constellation-id", "--star-map-id", "--stars-id", "--north-star-id", "--id":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ConstellationID = cleanChannelConstellationID(fields[i+1])
			i++
		case "--lane", "--scope", "--for", "--theme":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelConstellationActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelConstellationActionRequest{}, fmt.Errorf("unknown channel constellation argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelConstellationIssueTargetIfPresent(ev, &req)
	if err := applyChannelConstellationPositionals(&req, positional); err != nil {
		return ChannelConstellationActionRequest{}, err
	}
	if err := applyChannelConstellationIssueTarget(ev, &req); err != nil {
		return ChannelConstellationActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelConstellationTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelConstellationSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ConstellationID) == "" {
		req.Options.ConstellationID = autoChannelConstellationID(ev, req.Options)
		req.AutoConstellationID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelConstellationNotifyMessageID(ev, req.Options.ConstellationID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelConstellationOptions(req.Options)
	if err := validateChannelConstellationActionRequestOptions(req.Options); err != nil {
		return ChannelConstellationActionRequest{}, err
	}
	stars := channelConstellationStarsForLane(req.Options.Lane)
	body := renderChannelConstellationNotificationBody(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ConstellationIDHash = shortDocumentHash(req.Options.ConstellationID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.StarSHA = shortDocumentHash(renderChannelConstellationStarFingerprint(stars))
	req.StarCount = len(stars)
	req.CommandCount = len(stars)
	req.NotificationBodySHA = shortDocumentHash(body)
	req.NotificationBodyBytes = len(body)
	req.NotificationBodyLineCnt = lineCount(body)
	return req, nil
}

func RunChannelConstellation(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelConstellationOptions) (ChannelConstellationResult, error) {
	opts = normalizeChannelConstellationOptions(opts)
	var err error
	opts, err = applyChannelConstellationRoute(cfg, opts)
	if err != nil {
		return ChannelConstellationResult{}, err
	}
	if err := validateChannelConstellationOptions(opts); err != nil {
		return ChannelConstellationResult{}, err
	}
	body := renderChannelConstellationNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelConstellationResult{}, fmt.Errorf("queue channel constellation notification: %w", err)
	}
	stars := channelConstellationStarsForLane(opts.Lane)
	return ChannelConstellationResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		ConstellationIDHash: shortDocumentHash(opts.ConstellationID),
		LaneHash:            shortDocumentHash(opts.Lane),
		NoteHash:            shortDocumentHash(opts.Note),
		BodyHash:            shortDocumentHash(body),
		StarHash:            shortDocumentHash(renderChannelConstellationStarFingerprint(stars)),
		StarCount:           len(stars),
		CommandCount:        len(stars),
	}, nil
}

func RenderChannelConstellationActionReport(ev Event, req ChannelConstellationActionRequest, result ChannelConstellationResult) string {
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
	constellationIDHash := firstNonEmpty(result.ConstellationIDHash, req.ConstellationIDHash)
	laneHash := firstNonEmpty(result.LaneHash, req.LaneSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	starHash := firstNonEmpty(result.StarHash, req.StarSHA)
	starCount := result.StarCount
	if starCount == 0 {
		starCount = req.StarCount
	}
	commandCount := result.CommandCount
	if commandCount == 0 {
		commandCount = req.CommandCount
	}
	bodyBytes := req.NotificationBodyBytes
	bodyLines := req.NotificationBodyLineCnt
	var b strings.Builder
	b.WriteString("## GitClaw Channel Constellation Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_constellation_status: `%s`\n", status)
	fmt.Fprintf(&b, "- constellation_card_mode: `%s`\n", "bounded-capability-star-map")
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
	fmt.Fprintf(&b, "- constellation_id_sha256_12: `%s`\n", noneIfEmpty(constellationIDHash))
	fmt.Fprintf(&b, "- constellation_id_auto: `%t`\n", req.AutoConstellationID)
	fmt.Fprintf(&b, "- constellation_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- constellation_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- constellation_star_count: `%d`\n", starCount)
	fmt.Fprintf(&b, "- constellation_command_count: `%d`\n", commandCount)
	fmt.Fprintf(&b, "- constellation_star_sha256_12: `%s`\n", noneIfEmpty(starHash))
	fmt.Fprintf(&b, "- constellation_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- constellation_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- constellation_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- constellation_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", bodyBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", bodyLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- dynamic_star_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- live_source_browse_performed: `%t`\n", false)
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
	fmt.Fprintf(&b, "- raw_constellation_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_constellation_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_constellation_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_constellation_stars_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_constellation_commands_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_constellation_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing capability star-map card on the canonical channel issue. The provider card can show bounded labels and copyable next commands, while this source receipt keeps raw ids, lane names, notes, star text, command text, channel bodies, issue bodies, comments, prompts, tool outputs, and provider message ids out of band. The action uses a static deck and does not call a model, execute commands, install skills, execute tools, read backup payloads, read soul bodies, write memory, fetch sources, browse live sources, call provider APIs, mutate workflows, change policy, create schedules, use external randomness, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read constellation cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent constellation cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate constellation cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelConstellationActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelConstellationActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelConstellationIssueTarget(ev Event, req *ChannelConstellationActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel constellation requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelConstellationIssueTargetIfPresent(ev Event, req *ChannelConstellationActionRequest) {
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

func applyChannelConstellationPositionals(req *ChannelConstellationActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	defaultLane := defaultChannelConstellationLaneForSubcommand(req.Subcommand)
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Lane == "" || req.Options.Lane == defaultLane {
				req.Options.Lane = value
				continue
			}
			return fmt.Errorf("unexpected channel constellation argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == defaultLane {
			req.Options.Lane = value
			continue
		}
		return fmt.Errorf("unexpected channel constellation argument %q", value)
	}
	return nil
}

func normalizeChannelConstellationOptions(opts ChannelConstellationOptions) ChannelConstellationOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ConstellationID = cleanChannelConstellationID(opts.ConstellationID)
	opts.Lane = cleanChannelConstellationLane(opts.Lane)
	opts.Note = cleanChannelConstellationNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelConstellationRoute(cfg Config, opts ChannelConstellationOptions) (ChannelConstellationOptions, error) {
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
		Body:      "GitClaw channel constellation.",
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

func validateChannelConstellationOptions(opts ChannelConstellationOptions) error {
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
	if opts.ConstellationID == "" {
		return fmt.Errorf("missing constellation id")
	}
	if !skillNamePattern.MatchString(opts.ConstellationID) {
		return fmt.Errorf("invalid constellation id %q", opts.ConstellationID)
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing constellation lane")
	}
	if len(channelConstellationStarsForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported constellation lane %q", opts.Lane)
	}
	return nil
}

func validateChannelConstellationActionRequestOptions(opts ChannelConstellationOptions) error {
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
	if opts.ConstellationID == "" {
		return fmt.Errorf("missing constellation id")
	}
	if !skillNamePattern.MatchString(opts.ConstellationID) {
		return fmt.Errorf("invalid constellation id %q", opts.ConstellationID)
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing constellation lane")
	}
	if len(channelConstellationStarsForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported constellation lane %q", opts.Lane)
	}
	return nil
}

func cleanChannelConstellationSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.NewReplacer("_", "-", " ", "-").Replace(value)
}

func cleanChannelConstellationID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelConstellationLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "all", "general", "capability", "capabilities", "agent", "agents", "claw", "gitclaw":
		return "all"
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
	case "research", "openclaw", "hermes", "landscape", "patterns", "pattern":
		return "research"
	case "channel", "channels", "provider", "providers", "slack", "telegram", "gateway", "gateways":
		return "channels"
	case "fun", "play", "spark", "sparks", "social":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelConstellationNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelConstellationTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelConstellationTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelConstellationNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func shouldSkipChannelConstellationTrailingLine(line string) bool {
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.Contains(lower, "do not include") || strings.Contains(lower, "do not leak") || strings.Contains(lower, "hidden token")
}

func defaultChannelConstellationLaneForSubcommand(subcommand string) string {
	switch cleanChannelConstellationSubcommand(subcommand) {
	case "research-constellation", "openclaw-constellation", "hermes-constellation":
		return "research"
	default:
		return "all"
	}
}

func autoChannelConstellationSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-constellation-source-%s", eventID(ev))
}

func autoChannelConstellationID(ev Event, opts ChannelConstellationOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return fmt.Sprintf("constellation-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelConstellationNotifyMessageID(ev Event, constellationID string) string {
	seed := strings.Join([]string{eventID(ev), constellationID}, "|")
	return fmt.Sprintf("gitclaw-channel-constellation-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelConstellationNotificationBody(opts ChannelConstellationOptions) string {
	stars := channelConstellationStarsForLane(opts.Lane)
	var b strings.Builder
	b.WriteString("GitClaw channel constellation.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	fmt.Fprintf(&b, "North star: %s\n", channelConstellationNorthStar(opts.Lane))
	b.WriteString("Stars:\n")
	for _, star := range stars {
		fmt.Fprintf(&b, "- %s [%s] - %s\n", star.Name, star.Surface, star.Signal)
		fmt.Fprintf(&b, "  Next: `%s`\n", star.Command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nConstellation hash: %s\n", shortDocumentHash(renderChannelConstellationStarFingerprint(stars)))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("Constellation persistence: advisory only; no durable channel state changed.\n")
	b.WriteString("\nConstellation source: bounded GitHub channel action deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Dynamic star generation: not performed by this action.\n")
	b.WriteString("External randomness: not used by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Memory write: not performed by this action.\n")
	b.WriteString("Source fetch: not performed by this action.\n")
	b.WriteString("Live source browse: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Policy mutation: not performed by this action.\n")
	b.WriteString("Schedule creation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelConstellationNorthStar(lane string) string {
	switch cleanChannelConstellationLane(lane) {
	case "skills":
		return "Discover and rehearse skills before installing or executing anything."
	case "tools":
		return "Keep tool energy behind reviewed evidence gates."
	case "soul":
		return "Treat high-authority context as reviewable and intentionally durable."
	case "memory":
		return "Turn useful recall into reviewable durable memory, not accidental persistence."
	case "backups":
		return "Make recovery confidence visible before restore pressure arrives."
	case "research":
		return "Turn OpenClaw and Hermes patterns into small audited GitClaw moves."
	case "channels":
		return "Keep Slack and Telegram lively while GitHub remains the ledger."
	case "fun":
		return "Make the thread easier to enter without losing the audit trail."
	default:
		return "Route a chat message toward one reviewed GitHub-native capability surface."
	}
}

func channelConstellationStarsForLane(lane string) []channelConstellationStar {
	switch cleanChannelConstellationLane(lane) {
	case "skills":
		return []channelConstellationStar{
			{Name: "Skill status", Surface: "skills", Signal: "see installed and proposed skill readiness", Command: "@gitclaw /channels skills --message-id <id> --notify-message-id <id>"},
			{Name: "Skill spotlight", Surface: "skills", Signal: "focus one skill without reading raw bodies", Command: "@gitclaw /channels skill-spotlight repo-reader --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Skill map", Surface: "skills", Signal: "walk a safe use path before installation or execution", Command: "@gitclaw /channels skill-map repo-reader --map-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "tools":
		return []channelConstellationStar{
			{Name: "Tool status", Surface: "tools", Signal: "inspect tool posture without executing tools", Command: "@gitclaw /channels tools --message-id <id> --notify-message-id <id>"},
			{Name: "Tool spotlight", Surface: "tools", Signal: "focus one built-in tool contract safely", Command: "@gitclaw /channels tool-spotlight search_files --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Tool map", Surface: "tools", Signal: "plan approval before a tool run", Command: "@gitclaw /channels tool-map search_files --map-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "soul":
		return []channelConstellationStar{
			{Name: "Soul status", Surface: "soul", Signal: "snapshot high-authority context without raw bodies", Command: "@gitclaw /channels soul-status --message-id <id> --notify-message-id <id>"},
			{Name: "Soul spotlight", Surface: "soul", Signal: "focus one authority surface without writing soul", Command: "@gitclaw /channels soul-spotlight identity --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Soul risk", Surface: "soul", Signal: "review persistent-state risk before promotion", Command: "@gitclaw /channels soul-risk --message-id <id> --notify-message-id <id>"},
		}
	case "memory":
		return []channelConstellationStar{
			{Name: "Memory status", Surface: "memory", Signal: "inspect durable-memory shape without writing memory", Command: "@gitclaw /channels memory-status --message-id <id> --notify-message-id <id>"},
			{Name: "Memory search", Surface: "memory", Signal: "recall body-free memory matches", Command: "@gitclaw /channels memory-search handoff --message-id <id> --notify-message-id <id>"},
			{Name: "Memory proposal", Surface: "memory", Signal: "turn a useful observation into a review issue", Command: "@gitclaw /channels propose-memory --memory-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "backups":
		return []channelConstellationStar{
			{Name: "Backup status", Surface: "backups", Signal: "inspect backup posture without reading payloads", Command: "@gitclaw /channels backup --message-id <id> --notify-message-id <id>"},
			{Name: "Backup spotlight", Surface: "backups", Signal: "focus backup metadata without restoring files", Command: "@gitclaw /channels backup-spotlight continuity --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Recovery map", Surface: "backups", Signal: "walk a safe recovery sequence without mutation", Command: "@gitclaw /channels recovery-map issue --map-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "research":
		return []channelConstellationStar{
			{Name: "Research catalog", Surface: "research", Signal: "review the OpenClaw and Hermes landscape without source fetches", Command: "@gitclaw /research catalog"},
			{Name: "Research spotlight", Surface: "research", Signal: "draw one source or pattern card", Command: "@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Research map", Surface: "research", Signal: "turn the selected pattern into a safe GitClaw path", Command: "@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "channels":
		return []channelConstellationStar{
			{Name: "Availability", Surface: "channels", Signal: "show provider readiness without probing sockets", Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>"},
			{Name: "Palette", Surface: "channels", Signal: "offer a tiny copyable command menu", Command: "@gitclaw /channels palette fun --palette-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Warmup", Surface: "channels", Signal: "make the thread easier to enter", Command: "@gitclaw /channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "fun":
		return []channelConstellationStar{
			{Name: "Haiku", Surface: "fun", Signal: "send a deterministic tiny poem", Command: "@gitclaw /channels haiku fun --haiku-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Story dice", Surface: "fun", Signal: "roll a bounded prompt-game card", Command: "@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Soundtrack", Surface: "fun", Signal: "queue a three-track thread mood", Command: "@gitclaw /channels soundtrack fun --soundtrack-id <id> --message-id <id> --notify-message-id <id>"},
		}
	default:
		return []channelConstellationStar{
			{Name: "Skills", Surface: "skills", Signal: "discover capability before installation", Command: "@gitclaw /channels skills --message-id <id> --notify-message-id <id>"},
			{Name: "Tools", Surface: "tools", Signal: "plan evidence before execution", Command: "@gitclaw /channels tools --message-id <id> --notify-message-id <id>"},
			{Name: "Soul and backups", Surface: "soul+backups", Signal: "keep authority and recovery reviewable", Command: "@gitclaw /channels compass all --compass-id <id> --message-id <id> --notify-message-id <id>"},
		}
	}
}

func renderChannelConstellationStarFingerprint(stars []channelConstellationStar) string {
	var parts []string
	for _, star := range stars {
		parts = append(parts, strings.Join([]string{star.Name, star.Surface, star.Signal, star.Command}, "|"))
	}
	return strings.Join(parts, "\n")
}
