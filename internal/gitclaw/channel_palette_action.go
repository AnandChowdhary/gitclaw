package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPaletteOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	PaletteID         string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelPaletteResult struct {
	Notification  ChannelSendResult
	RouteName     string
	RouteHash     string
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	PaletteIDHash string
	LaneHash      string
	NoteHash      string
	BodyHash      string
	CommandCount  int
}

type ChannelPaletteActionRequest struct {
	Options             ChannelPaletteOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoPaletteID       bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	PaletteIDHash       string
	LaneSHA             string
	LaneBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	CommandCount        int
	NotificationBodySHA string
}

func IsChannelPaletteActionRequest(ev Event, cfg Config) bool {
	return isChannelPaletteActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPaletteActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "palette", "menu", "shortcuts", "shortcut", "command-palette", "launcher", "cheat-sheet", "cheatsheet", "help-card":
		return true
	default:
		return false
	}
}

func BuildChannelPaletteActionRequest(ev Event, cfg Config) (ChannelPaletteActionRequest, error) {
	fields, trailing, ok := channelPaletteActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPaletteActionRequest{}, fmt.Errorf("missing channel palette command")
	}
	req := ChannelPaletteActionRequest{
		Options: ChannelPaletteOptions{
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
				return ChannelPaletteActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--palette-id", "--menu-id", "--shortcut-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PaletteID = cleanChannelPaletteID(fields[i+1])
			i++
		case "--lane", "--scope", "--section", "--for":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPaletteActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPaletteActionRequest{}, fmt.Errorf("unknown channel palette argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelPaletteIssueTargetIfPresent(ev, &req)
	if err := applyChannelPalettePositionals(&req, positional); err != nil {
		return ChannelPaletteActionRequest{}, err
	}
	if err := applyChannelPaletteIssueTarget(ev, &req); err != nil {
		return ChannelPaletteActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelPaletteTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelPaletteSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.PaletteID) == "" {
		req.Options.PaletteID = autoChannelPaletteID(ev, req.Options)
		req.AutoPaletteID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPaletteNotifyMessageID(ev, req.Options.PaletteID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPaletteOptions(req.Options)
	if err := validateChannelPaletteActionRequestOptions(req.Options); err != nil {
		return ChannelPaletteActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.PaletteIDHash = shortDocumentHash(req.Options.PaletteID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.CommandCount = len(channelPaletteCommandsForLane(req.Options.Lane))
	req.NotificationBodySHA = shortDocumentHash(renderChannelPaletteNotificationBody(req.Options))
	return req, nil
}

func RunChannelPalette(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPaletteOptions) (ChannelPaletteResult, error) {
	opts = normalizeChannelPaletteOptions(opts)
	var err error
	opts, err = applyChannelPaletteRoute(cfg, opts)
	if err != nil {
		return ChannelPaletteResult{}, err
	}
	if err := validateChannelPaletteOptions(opts); err != nil {
		return ChannelPaletteResult{}, err
	}
	body := renderChannelPaletteNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelPaletteResult{}, fmt.Errorf("queue channel palette notification: %w", err)
	}
	return ChannelPaletteResult{
		Notification:  notification,
		RouteName:     opts.Route,
		RouteHash:     channelRouteHash(opts.Route),
		Channel:       opts.Channel,
		ThreadHash:    shortDocumentHash(opts.ThreadID),
		MessageHash:   shortDocumentHash(opts.SourceMessageID),
		NotifyHash:    shortDocumentHash(opts.NotifyMessageID),
		PaletteIDHash: shortDocumentHash(opts.PaletteID),
		LaneHash:      shortDocumentHash(opts.Lane),
		NoteHash:      shortDocumentHash(opts.Note),
		BodyHash:      shortDocumentHash(body),
		CommandCount:  len(channelPaletteCommandsForLane(opts.Lane)),
	}, nil
}

func RenderChannelPaletteActionReport(ev Event, req ChannelPaletteActionRequest, result ChannelPaletteResult) string {
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
	paletteIDHash := result.PaletteIDHash
	if paletteIDHash == "" {
		paletteIDHash = req.PaletteIDHash
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
	commandCount := result.CommandCount
	if commandCount == 0 {
		commandCount = req.CommandCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Palette Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_palette_status: `%s`\n", status)
	fmt.Fprintf(&b, "- palette_mode: `%s`\n", "structured-channel-command-palette")
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
	fmt.Fprintf(&b, "- palette_id_sha256_12: `%s`\n", noneIfEmpty(paletteIDHash))
	fmt.Fprintf(&b, "- palette_id_auto: `%t`\n", req.AutoPaletteID)
	fmt.Fprintf(&b, "- palette_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- palette_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- palette_command_count: `%d`\n", commandCount)
	fmt.Fprintf(&b, "- palette_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- palette_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- palette_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- palette_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_palette_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_palette_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_palette_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_palette_commands_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_palette_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing command palette on the canonical channel issue. This gives the chat thread a compact launcher for channel-native work while keeping command execution, skill installs, tool execution, backup payload reads, soul body reads, provider API calls, model calls, provider delivery, and repository mutations out of this action. The source receipt keeps thread ids, message ids, palette ids, palette lanes, notes, command bodies, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read palette updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent palette updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate palette updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelPaletteActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPaletteActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPaletteIssueTarget(ev Event, req *ChannelPaletteActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel palette requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelPaletteIssueTargetIfPresent(ev Event, req *ChannelPaletteActionRequest) {
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

func applyChannelPalettePositionals(req *ChannelPaletteActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel palette argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == "all" {
			req.Options.Lane = value
			continue
		}
		return fmt.Errorf("unexpected channel palette argument %q", value)
	}
	return nil
}

func normalizeChannelPaletteOptions(opts ChannelPaletteOptions) ChannelPaletteOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.PaletteID = cleanChannelPaletteID(opts.PaletteID)
	opts.Lane = cleanChannelPaletteLane(opts.Lane)
	opts.Note = cleanChannelPaletteNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelPaletteRoute(cfg Config, opts ChannelPaletteOptions) (ChannelPaletteOptions, error) {
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
		Body:      "GitClaw channel palette.",
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

func validateChannelPaletteOptions(opts ChannelPaletteOptions) error {
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
	if opts.PaletteID == "" {
		return fmt.Errorf("missing palette id")
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing palette lane")
	}
	if len(channelPaletteCommandsForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported palette lane %q", opts.Lane)
	}
	return nil
}

func validateChannelPaletteActionRequestOptions(opts ChannelPaletteOptions) error {
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
	if opts.PaletteID == "" {
		return fmt.Errorf("missing palette id")
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing palette lane")
	}
	if len(channelPaletteCommandsForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported palette lane %q", opts.Lane)
	}
	return nil
}

func cleanChannelPaletteID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelPaletteLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "all", "default", "capabilities", "capability", "commands", "command":
		return "all"
	case "core", "status", "ops", "basics", "basic":
		return "core"
	case "skill", "skills", "capability-search":
		return "skills"
	case "tool", "tools", "tooling":
		return "tools"
	case "soul", "souls", "authority", "context":
		return "soul"
	case "backup", "backups", "recovery":
		return "backups"
	case "fun", "play", "signals", "presence":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelPaletteNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelPaletteTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelPaletteNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelPaletteSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-palette-source-%s", eventID(ev))
}

func autoChannelPaletteID(ev Event, opts ChannelPaletteOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return fmt.Sprintf("palette-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPaletteNotifyMessageID(ev Event, paletteID string) string {
	seed := strings.Join([]string{eventID(ev), paletteID}, "|")
	return fmt.Sprintf("gitclaw-channel-palette-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelPaletteNotificationBody(opts ChannelPaletteOptions) string {
	commands := channelPaletteCommandsForLane(opts.Lane)
	var b strings.Builder
	b.WriteString("GitClaw channel palette.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	b.WriteString("Shortcuts:\n")
	for _, command := range commands {
		fmt.Fprintf(&b, "- %s\n", command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nPalette hash: %s\n", shortDocumentHash(opts.Lane+"|"+strings.Join(commands, "|")))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nPalette source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelPaletteCommandsForLane(lane string) []string {
	switch cleanChannelPaletteLane(lane) {
	case "core":
		return []string{
			"/channels availability --message-id <id> --notify-message-id <id>",
			"/channels topic --topic-id <id>",
			"/channels activity typing --activity-id <id> --message-id <id>",
			"/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>",
		}
	case "skills":
		return []string{
			"/channels skills --message-id <id>",
			"/channels skill-search <query> --message-id <id> --notify-message-id <id>",
			"/channels skill-info <skill> --message-id <id> --notify-message-id <id>",
			"/channels skill-note --note-id <id> --skill <name> --message-id <id>",
		}
	case "tools":
		return []string{
			"/channels tools --message-id <id>",
			"/channels tool-search <query> --message-id <id> --notify-message-id <id>",
			"/channels tool-info <tool> --message-id <id> --notify-message-id <id>",
			"/channels tool-lesson --note-id <id> --tool <tool> --message-id <id>",
		}
	case "soul":
		return []string{
			"/channels soul-status --message-id <id>",
			"/channels soul-search <query> --message-id <id> --notify-message-id <id>",
			"/channels soul-info <path> --message-id <id> --notify-message-id <id>",
			"/channels soul-note --note-id <id> --area <area> --message-id <id>",
		}
	case "backups":
		return []string{
			"/channels backup --message-id <id>",
			"/channels backup-search <query> --message-id <id> --notify-message-id <id>",
			"/channels backup-info <issue> --message-id <id> --notify-message-id <id>",
			"/channels backup-note --note-id <id> --scope <scope> --message-id <id>",
		}
	case "fun":
		return []string{
			"/channels roll --dice 2d6 --message-id <id> --notify-message-id <id>",
			"/channels choose --message-id <id> --notify-message-id <id>",
			"/channels mood <mood> --message-id <id> --notify-message-id <id>",
			"/channels sticker <sticker> --sticker-id <id> --message-id <id> --notify-message-id <id>",
			"/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>",
		}
	case "all":
		return []string{
			"/channels availability --message-id <id> --notify-message-id <id>",
			"/channels skills --message-id <id>",
			"/channels skill-search <query> --message-id <id> --notify-message-id <id>",
			"/channels tools --message-id <id>",
			"/channels tool-search <query> --message-id <id> --notify-message-id <id>",
			"/channels soul-search <query> --message-id <id> --notify-message-id <id>",
			"/channels backup-search <query> --message-id <id> --notify-message-id <id>",
			"/channels roll --dice 2d6 --message-id <id> --notify-message-id <id>",
			"/channels mood <mood> --message-id <id> --notify-message-id <id>",
			"/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>",
		}
	default:
		return nil
	}
}
