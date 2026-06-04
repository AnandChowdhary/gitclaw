package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelCockpitOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	CockpitID         string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelCockpitResult struct {
	Notification  ChannelSendResult
	RouteName     string
	RouteHash     string
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	CockpitIDHash string
	LaneHash      string
	NoteHash      string
	BodyHash      string
	GaugeHash     string
	GaugeCount    int
	CommandCount  int
}

type ChannelCockpitActionRequest struct {
	Options             ChannelCockpitOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoCockpitID       bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	CockpitIDHash       string
	LaneSHA             string
	LaneBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	GaugeSHA            string
	GaugeCount          int
	CommandCount        int
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type channelCockpitGauge struct {
	Name    string
	State   string
	Signal  string
	Command string
}

func IsChannelCockpitActionRequest(ev Event, cfg Config) bool {
	return isChannelCockpitActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelCockpitActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelCockpitSubcommand(fields[1]) {
	case "cockpit", "control-room", "status-board", "dashboard", "dash", "ops-board", "ops-cockpit", "channel-cockpit", "openclaw-cockpit", "hermes-cockpit", "flight-status":
		return true
	default:
		return false
	}
}

func BuildChannelCockpitActionRequest(ev Event, cfg Config) (ChannelCockpitActionRequest, error) {
	fields, trailing, ok := channelCockpitActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelCockpitActionRequest{}, fmt.Errorf("missing channel cockpit command")
	}
	req := ChannelCockpitActionRequest{
		Options: ChannelCockpitOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Lane:              defaultChannelCockpitLaneForSubcommand(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelCockpitSubcommand(fields[1]),
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
				return ChannelCockpitActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--cockpit-id", "--dashboard-id", "--board-id", "--status-board-id", "--id":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.CockpitID = cleanChannelCockpitID(fields[i+1])
			i++
		case "--lane", "--scope", "--for", "--theme":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelCockpitActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelCockpitActionRequest{}, fmt.Errorf("unknown channel cockpit argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelCockpitIssueTargetIfPresent(ev, &req)
	if err := applyChannelCockpitPositionals(&req, positional); err != nil {
		return ChannelCockpitActionRequest{}, err
	}
	if err := applyChannelCockpitIssueTarget(ev, &req); err != nil {
		return ChannelCockpitActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelCockpitTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelCockpitSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.CockpitID) == "" {
		req.Options.CockpitID = autoChannelCockpitID(ev, req.Options)
		req.AutoCockpitID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelCockpitNotifyMessageID(ev, req.Options.CockpitID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelCockpitOptions(req.Options)
	if err := validateChannelCockpitActionRequestOptions(req.Options); err != nil {
		return ChannelCockpitActionRequest{}, err
	}
	gauges := channelCockpitGaugesForLane(req.Options.Lane)
	body := renderChannelCockpitNotificationBody(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.CockpitIDHash = shortDocumentHash(req.Options.CockpitID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.GaugeSHA = shortDocumentHash(renderChannelCockpitGaugeFingerprint(gauges))
	req.GaugeCount = len(gauges)
	req.CommandCount = len(gauges)
	req.NotificationBodySHA = shortDocumentHash(body)
	req.NotificationBytes = len(body)
	req.NotificationLines = lineCount(body)
	return req, nil
}

func RunChannelCockpit(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelCockpitOptions) (ChannelCockpitResult, error) {
	opts = normalizeChannelCockpitOptions(opts)
	var err error
	opts, err = applyChannelCockpitRoute(cfg, opts)
	if err != nil {
		return ChannelCockpitResult{}, err
	}
	if err := validateChannelCockpitOptions(opts); err != nil {
		return ChannelCockpitResult{}, err
	}
	body := renderChannelCockpitNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelCockpitResult{}, fmt.Errorf("queue channel cockpit notification: %w", err)
	}
	gauges := channelCockpitGaugesForLane(opts.Lane)
	return ChannelCockpitResult{
		Notification:  notification,
		RouteName:     opts.Route,
		RouteHash:     channelRouteHash(opts.Route),
		Channel:       opts.Channel,
		ThreadHash:    shortDocumentHash(opts.ThreadID),
		MessageHash:   shortDocumentHash(opts.SourceMessageID),
		NotifyHash:    shortDocumentHash(opts.NotifyMessageID),
		CockpitIDHash: shortDocumentHash(opts.CockpitID),
		LaneHash:      shortDocumentHash(opts.Lane),
		NoteHash:      shortDocumentHash(opts.Note),
		BodyHash:      shortDocumentHash(body),
		GaugeHash:     shortDocumentHash(renderChannelCockpitGaugeFingerprint(gauges)),
		GaugeCount:    len(gauges),
		CommandCount:  len(gauges),
	}, nil
}

func RenderChannelCockpitActionReport(ev Event, req ChannelCockpitActionRequest, result ChannelCockpitResult) string {
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
	cockpitIDHash := firstNonEmpty(result.CockpitIDHash, req.CockpitIDHash)
	laneHash := firstNonEmpty(result.LaneHash, req.LaneSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	gaugeHash := firstNonEmpty(result.GaugeHash, req.GaugeSHA)
	gaugeCount := result.GaugeCount
	if gaugeCount == 0 {
		gaugeCount = req.GaugeCount
	}
	commandCount := result.CommandCount
	if commandCount == 0 {
		commandCount = req.CommandCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Cockpit Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_cockpit_status: `%s`\n", status)
	fmt.Fprintf(&b, "- cockpit_card_mode: `%s`\n", "bounded-provider-status-board")
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
	fmt.Fprintf(&b, "- cockpit_id_sha256_12: `%s`\n", noneIfEmpty(cockpitIDHash))
	fmt.Fprintf(&b, "- cockpit_id_auto: `%t`\n", req.AutoCockpitID)
	fmt.Fprintf(&b, "- cockpit_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- cockpit_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- cockpit_gauge_count: `%d`\n", gaugeCount)
	fmt.Fprintf(&b, "- cockpit_command_count: `%d`\n", commandCount)
	fmt.Fprintf(&b, "- cockpit_gauge_sha256_12: `%s`\n", noneIfEmpty(gaugeHash))
	fmt.Fprintf(&b, "- cockpit_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- cockpit_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- cockpit_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- cockpit_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", req.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", req.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- dynamic_cockpit_generation_performed: `%t`\n", false)
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
	fmt.Fprintf(&b, "- raw_cockpit_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_cockpit_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_cockpit_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_cockpit_gauges_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_cockpit_commands_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_cockpit_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing status-board card on the canonical channel issue. The provider card can show bounded gauges and copyable next commands, while this source receipt keeps raw ids, lane names, notes, gauge text, command text, channel bodies, issue bodies, comments, prompts, tool outputs, and provider message ids out of band. The action uses a static deck and does not call a model, execute commands, install skills, execute tools, read backup payloads, read soul bodies, write memory, fetch sources, browse live sources, create artifact issues, call provider APIs, mutate workflows, change policy, create schedules, use external randomness, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read cockpit cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent cockpit cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate cockpit cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelCockpitActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelCockpitActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelCockpitIssueTarget(ev Event, req *ChannelCockpitActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel cockpit requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelCockpitIssueTargetIfPresent(ev Event, req *ChannelCockpitActionRequest) {
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

func applyChannelCockpitPositionals(req *ChannelCockpitActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	defaultLane := defaultChannelCockpitLaneForSubcommand(req.Subcommand)
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Lane == "" || req.Options.Lane == defaultLane {
				req.Options.Lane = value
				continue
			}
			return fmt.Errorf("unexpected channel cockpit argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == defaultLane {
			req.Options.Lane = value
			continue
		}
		return fmt.Errorf("unexpected channel cockpit argument %q", value)
	}
	return nil
}

func normalizeChannelCockpitOptions(opts ChannelCockpitOptions) ChannelCockpitOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.CockpitID = cleanChannelCockpitID(opts.CockpitID)
	opts.Lane = cleanChannelCockpitLane(opts.Lane)
	opts.Note = cleanChannelCockpitNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelCockpitRoute(cfg Config, opts ChannelCockpitOptions) (ChannelCockpitOptions, error) {
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
		Body:      "GitClaw channel cockpit.",
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

func validateChannelCockpitOptions(opts ChannelCockpitOptions) error {
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
	if opts.CockpitID == "" {
		return fmt.Errorf("missing cockpit id")
	}
	if !skillNamePattern.MatchString(opts.CockpitID) {
		return fmt.Errorf("invalid cockpit id %q", opts.CockpitID)
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing cockpit lane")
	}
	if len(channelCockpitGaugesForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported cockpit lane %q", opts.Lane)
	}
	return nil
}

func validateChannelCockpitActionRequestOptions(opts ChannelCockpitOptions) error {
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
	if opts.CockpitID == "" {
		return fmt.Errorf("missing cockpit id")
	}
	if !skillNamePattern.MatchString(opts.CockpitID) {
		return fmt.Errorf("invalid cockpit id %q", opts.CockpitID)
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing cockpit lane")
	}
	if len(channelCockpitGaugesForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported cockpit lane %q", opts.Lane)
	}
	return nil
}

func cleanChannelCockpitSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.NewReplacer("_", "-", " ", "-").Replace(value)
}

func cleanChannelCockpitID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelCockpitLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "all", "general", "ops", "operator", "agent", "agents", "gitclaw":
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

func cleanChannelCockpitNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelCockpitTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelCockpitTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelCockpitNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func shouldSkipChannelCockpitTrailingLine(line string) bool {
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.Contains(lower, "do not include") || strings.Contains(lower, "do not leak") || strings.Contains(lower, "hidden token")
}

func defaultChannelCockpitLaneForSubcommand(subcommand string) string {
	switch cleanChannelCockpitSubcommand(subcommand) {
	case "openclaw-cockpit", "hermes-cockpit":
		return "research"
	default:
		return "all"
	}
}

func autoChannelCockpitSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-cockpit-source-%s", eventID(ev))
}

func autoChannelCockpitID(ev Event, opts ChannelCockpitOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return fmt.Sprintf("cockpit-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelCockpitNotifyMessageID(ev Event, cockpitID string) string {
	seed := strings.Join([]string{eventID(ev), cockpitID}, "|")
	return fmt.Sprintf("gitclaw-channel-cockpit-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelCockpitNotificationBody(opts ChannelCockpitOptions) string {
	gauges := channelCockpitGaugesForLane(opts.Lane)
	var b strings.Builder
	b.WriteString("GitClaw channel cockpit.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	fmt.Fprintf(&b, "Board: %s\n", channelCockpitBoard(opts.Lane))
	b.WriteString("Gauges:\n")
	for _, gauge := range gauges {
		fmt.Fprintf(&b, "- %s [%s] - %s\n", gauge.Name, gauge.State, gauge.Signal)
		fmt.Fprintf(&b, "  Next: `%s`\n", gauge.Command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nCockpit hash: %s\n", shortDocumentHash(opts.CockpitID))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("Cockpit persistence: advisory only; no durable channel state changed.\n")
	b.WriteString("\nCockpit source: bounded GitHub channel action deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Dynamic cockpit generation: not performed by this action.\n")
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

func channelCockpitBoard(lane string) string {
	switch cleanChannelCockpitLane(lane) {
	case "research":
		return "OpenClaw/Hermes patterns are ready for review-first GitClaw translation."
	case "skills":
		return "Skills are visible as metadata before any install, load, or write."
	case "tools":
		return "Tools stay reviewable before execution, approval, or schema exposure."
	case "soul":
		return "High-authority context stays explicit before proposal or promotion."
	case "memory":
		return "Durable memory is recall-first and write-gated."
	case "backups":
		return "Recovery posture is metadata-first before payload reads or restores."
	case "channels":
		return "Provider chat stays lively while GitHub remains the durable ledger."
	case "launch":
		return "Launch work stays rollback-aware and status-visible."
	case "fun":
		return "Small playful moves keep momentum without creating hidden state."
	default:
		return "Choose one safe surface, inspect it, and keep follow-up in GitHub."
	}
}

func channelCockpitGaugesForLane(lane string) []channelCockpitGauge {
	switch cleanChannelCockpitLane(lane) {
	case "research":
		return []channelCockpitGauge{
			{Name: "Evidence", State: "ready", Signal: "reviewed OpenClaw/Hermes source catalog is the first stop", Command: "@gitclaw /research catalog"},
			{Name: "Pattern", State: "ready", Signal: "draw one source or pattern without live browsing", Command: "@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Path", State: "ready", Signal: "translate the selected pattern into safe GitClaw commands", Command: "@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Loop", State: "watch", Signal: "keep the next move review-first and bounded", Command: "@gitclaw /channels mission-control research --mission-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "skills":
		return []channelCockpitGauge{
			{Name: "Inventory", State: "ready", Signal: "skill status is metadata-only", Command: "@gitclaw /channels skills --message-id <id> --notify-message-id <id>"},
			{Name: "Search", State: "ready", Signal: "skill discovery avoids registry contact and body loads", Command: "@gitclaw /channels skill-search repo-reader --message-id <id> --notify-message-id <id>"},
			{Name: "Map", State: "watch", Signal: "safe skill use is sequenced before installs", Command: "@gitclaw /channels skill-map repo-reader --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Review", State: "gated", Signal: "new skills move through GitHub proposal issues", Command: "@gitclaw /channels propose-skill repo-reader --message-id <id> --notify-message-id <id>"},
		}
	case "tools":
		return []channelCockpitGauge{
			{Name: "Inventory", State: "ready", Signal: "tool status is visible without schema dumps", Command: "@gitclaw /channels tools --message-id <id> --notify-message-id <id>"},
			{Name: "Search", State: "ready", Signal: "tool discovery is body-free and non-executing", Command: "@gitclaw /channels tool-search search_files --message-id <id> --notify-message-id <id>"},
			{Name: "Map", State: "watch", Signal: "tool use is mapped before approval or run requests", Command: "@gitclaw /channels tool-map search_files --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Approval", State: "gated", Signal: "execution stays behind a reviewed request", Command: "@gitclaw /channels request-run search_files --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "soul":
		return []channelCockpitGauge{
			{Name: "Snapshot", State: "ready", Signal: "authority posture is visible without raw bodies", Command: "@gitclaw /channels soul-status --message-id <id> --notify-message-id <id>"},
			{Name: "Spotlight", State: "ready", Signal: "one high-authority surface can be inspected safely", Command: "@gitclaw /channels soul-spotlight identity --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Risk", State: "watch", Signal: "persistent-state risk is checked before edits", Command: "@gitclaw /channels soul-risk --message-id <id> --notify-message-id <id>"},
			{Name: "Proposal", State: "gated", Signal: "soul changes require a GitHub proposal issue", Command: "@gitclaw /channels propose-soul --target soul --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "memory":
		return []channelCockpitGauge{
			{Name: "Snapshot", State: "ready", Signal: "durable-memory posture is visible without writes", Command: "@gitclaw /channels memory-status --message-id <id> --notify-message-id <id>"},
			{Name: "Recall", State: "ready", Signal: "memory search stays body-free", Command: "@gitclaw /channels memory-search handoff --message-id <id> --notify-message-id <id>"},
			{Name: "Proposal", State: "watch", Signal: "candidate memory moves through review", Command: "@gitclaw /channels propose-memory --memory-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Rehearsal", State: "gated", Signal: "promotion is rehearsed before becoming prompt-visible", Command: "@gitclaw /channels rehearse-memory --target long-term --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "backups":
		return []channelCockpitGauge{
			{Name: "Snapshot", State: "ready", Signal: "backup posture is visible without payload reads", Command: "@gitclaw /channels backup --message-id <id> --notify-message-id <id>"},
			{Name: "Freshness", State: "watch", Signal: "latest backup age is checked from metadata", Command: "@gitclaw /channels backup-freshness --freshness-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Continuity", State: "watch", Signal: "archive gaps are inspected before recovery work", Command: "@gitclaw /channels backup-continuity --continuity-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Restore", State: "gated", Signal: "restore review is explicit and dry-run-first", Command: "@gitclaw /channels restore-request --request-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "channels":
		return []channelCockpitGauge{
			{Name: "Presence", State: "ready", Signal: "availability is visible without socket probes", Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>"},
			{Name: "Palette", State: "ready", Signal: "chat-native shortcuts stay bounded", Command: "@gitclaw /channels palette fun --palette-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Room", State: "watch", Signal: "thread pulse reads safe markers, not raw bodies", Command: "@gitclaw /channels room-pulse handoff --pulse-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Outbox", State: "gated", Signal: "provider delivery stays through GitHub receipts", Command: "gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>"},
		}
	case "launch":
		return []channelCockpitGauge{
			{Name: "Mode", State: "ready", Signal: "thread posture is advisory and non-persistent", Command: "@gitclaw /channels mode launch --mode-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Readiness", State: "watch", Signal: "launch questions stay visible before shipping", Command: "@gitclaw /channels warmup launch --warmup-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Recovery", State: "watch", Signal: "rollback path is checked before status nudges", Command: "@gitclaw /channels recovery-map incident --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Signal", State: "gated", Signal: "attention nudges remain explicit provider cards", Command: "@gitclaw /channels nudge release-captain --nudge-id <id> --message-id <id> --notify-message-id <id> --tone gentle"},
		}
	case "fun":
		return []channelCockpitGauge{
			{Name: "Warmup", State: "ready", Signal: "lightweight presence can start the thread", Command: "@gitclaw /channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Spark", State: "ready", Signal: "small ideas stay bounded", Command: "@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Play", State: "ready", Signal: "prompt games do not persist scores or state", Command: "@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Trail", State: "watch", Signal: "useful moments can leave a searchable breadcrumb", Command: "@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>"},
		}
	default:
		return []channelCockpitGauge{
			{Name: "Bridge", State: "ready", Signal: "channel thread and provider routing are visible", Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>"},
			{Name: "Capabilities", State: "ready", Signal: "capability surfaces are visible without execution", Command: "@gitclaw /channels constellation all --constellation-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Control", State: "watch", Signal: "next actions stay review-first", Command: "@gitclaw /channels mission-control all --mission-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Orient", State: "gated", Signal: "ask for a safe next step before doing work", Command: "@gitclaw /channels compass all --compass-id <id> --message-id <id> --notify-message-id <id>"},
		}
	}
}

func renderChannelCockpitGaugeFingerprint(gauges []channelCockpitGauge) string {
	var parts []string
	for _, gauge := range gauges {
		parts = append(parts, strings.Join([]string{gauge.Name, gauge.State, gauge.Signal, gauge.Command}, "|"))
	}
	return strings.Join(parts, "\n")
}
