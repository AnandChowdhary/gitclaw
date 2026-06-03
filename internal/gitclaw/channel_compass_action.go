package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelCompassOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	CompassID         string
	Focus             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelCompassResult struct {
	Notification  ChannelSendResult
	RouteName     string
	RouteHash     string
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	CompassIDHash string
	FocusHash     string
	NoteHash      string
	BodyHash      string
	StepCount     int
}

type ChannelCompassActionRequest struct {
	Options             ChannelCompassOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoCompassID       bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	CompassIDHash       string
	FocusSHA            string
	FocusBytes          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	StepCount           int
	NotificationBodySHA string
}

func IsChannelCompassActionRequest(ev Event, cfg Config) bool {
	return isChannelCompassActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelCompassActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "compass", "orient", "orientation", "navigator", "navigate", "whereami", "map", "wayfinder", "guide":
		return true
	default:
		return false
	}
}

func BuildChannelCompassActionRequest(ev Event, cfg Config) (ChannelCompassActionRequest, error) {
	fields, trailing, ok := channelCompassActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelCompassActionRequest{}, fmt.Errorf("missing channel compass command")
	}
	req := ChannelCompassActionRequest{
		Options: ChannelCompassOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             "all",
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
				return ChannelCompassActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--compass-id", "--orient-id", "--navigator-id", "--map-id", "--id":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.CompassID = cleanChannelCompassID(fields[i+1])
			i++
		case "--focus", "--scope", "--section", "--for":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelCompassActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelCompassActionRequest{}, fmt.Errorf("unknown channel compass argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelCompassIssueTargetIfPresent(ev, &req)
	if err := applyChannelCompassPositionals(&req, positional); err != nil {
		return ChannelCompassActionRequest{}, err
	}
	if err := applyChannelCompassIssueTarget(ev, &req); err != nil {
		return ChannelCompassActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelCompassTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelCompassSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.CompassID) == "" {
		req.Options.CompassID = autoChannelCompassID(ev, req.Options)
		req.AutoCompassID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelCompassNotifyMessageID(ev, req.Options.CompassID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelCompassOptions(req.Options)
	if err := validateChannelCompassActionRequestOptions(req.Options); err != nil {
		return ChannelCompassActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.CompassIDHash = shortDocumentHash(req.Options.CompassID)
	req.FocusSHA = shortDocumentHash(req.Options.Focus)
	req.FocusBytes = len(req.Options.Focus)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.StepCount = len(channelCompassStepsForFocus(req.Options.Focus))
	req.NotificationBodySHA = shortDocumentHash(renderChannelCompassNotificationBody(req.Options))
	return req, nil
}

func RunChannelCompass(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelCompassOptions) (ChannelCompassResult, error) {
	opts = normalizeChannelCompassOptions(opts)
	var err error
	opts, err = applyChannelCompassRoute(cfg, opts)
	if err != nil {
		return ChannelCompassResult{}, err
	}
	if err := validateChannelCompassOptions(opts); err != nil {
		return ChannelCompassResult{}, err
	}
	body := renderChannelCompassNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelCompassResult{}, fmt.Errorf("queue channel compass notification: %w", err)
	}
	return ChannelCompassResult{
		Notification:  notification,
		RouteName:     opts.Route,
		RouteHash:     channelRouteHash(opts.Route),
		Channel:       opts.Channel,
		ThreadHash:    shortDocumentHash(opts.ThreadID),
		MessageHash:   shortDocumentHash(opts.SourceMessageID),
		NotifyHash:    shortDocumentHash(opts.NotifyMessageID),
		CompassIDHash: shortDocumentHash(opts.CompassID),
		FocusHash:     shortDocumentHash(opts.Focus),
		NoteHash:      shortDocumentHash(opts.Note),
		BodyHash:      shortDocumentHash(body),
		StepCount:     len(channelCompassStepsForFocus(opts.Focus)),
	}, nil
}

func RenderChannelCompassActionReport(ev Event, req ChannelCompassActionRequest, result ChannelCompassResult) string {
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
	compassIDHash := result.CompassIDHash
	if compassIDHash == "" {
		compassIDHash = req.CompassIDHash
	}
	focusHash := result.FocusHash
	if focusHash == "" {
		focusHash = req.FocusSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	stepCount := result.StepCount
	if stepCount == 0 {
		stepCount = req.StepCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Compass Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_compass_status: `%s`\n", status)
	fmt.Fprintf(&b, "- compass_mode: `%s`\n", "structured-channel-compass")
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
	fmt.Fprintf(&b, "- compass_id_sha256_12: `%s`\n", noneIfEmpty(compassIDHash))
	fmt.Fprintf(&b, "- compass_id_auto: `%t`\n", req.AutoCompassID)
	fmt.Fprintf(&b, "- compass_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- compass_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- compass_step_count: `%d`\n", stepCount)
	fmt.Fprintf(&b, "- compass_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- compass_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- compass_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- compass_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
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
	fmt.Fprintf(&b, "- raw_compass_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_compass_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_compass_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_compass_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_compass_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel compass on the canonical channel issue. This gives the chat thread a compact orientation card for safe next steps across skills, tools, soul, memory, backups, and lightweight channel signals while keeping command execution, skill installs, tool execution, backup payload reads, soul body reads, provider API calls, model calls, provider delivery, and repository mutations out of this action. The source receipt keeps thread ids, message ids, compass ids, focus values, notes, step text, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read compass updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent compass updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate compass updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelCompassActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelCompassActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelCompassIssueTarget(ev Event, req *ChannelCompassActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel compass requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelCompassIssueTargetIfPresent(ev Event, req *ChannelCompassActionRequest) {
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

func applyChannelCompassPositionals(req *ChannelCompassActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Focus == "" || req.Options.Focus == "all" {
				req.Options.Focus = value
				continue
			}
			return fmt.Errorf("unexpected channel compass argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Focus == "" || req.Options.Focus == "all" {
			req.Options.Focus = value
			continue
		}
		return fmt.Errorf("unexpected channel compass argument %q", value)
	}
	return nil
}

func normalizeChannelCompassOptions(opts ChannelCompassOptions) ChannelCompassOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.CompassID = cleanChannelCompassID(opts.CompassID)
	opts.Focus = cleanChannelCompassFocus(opts.Focus)
	opts.Note = cleanChannelCompassNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelCompassRoute(cfg Config, opts ChannelCompassOptions) (ChannelCompassOptions, error) {
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
		Body:      "GitClaw channel compass.",
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

func validateChannelCompassOptions(opts ChannelCompassOptions) error {
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
	if opts.CompassID == "" {
		return fmt.Errorf("missing compass id")
	}
	if opts.Focus == "" {
		return fmt.Errorf("missing compass focus")
	}
	if len(channelCompassStepsForFocus(opts.Focus)) == 0 {
		return fmt.Errorf("unsupported compass focus %q", opts.Focus)
	}
	return nil
}

func validateChannelCompassActionRequestOptions(opts ChannelCompassOptions) error {
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
	if opts.CompassID == "" {
		return fmt.Errorf("missing compass id")
	}
	if opts.Focus == "" {
		return fmt.Errorf("missing compass focus")
	}
	if len(channelCompassStepsForFocus(opts.Focus)) == 0 {
		return fmt.Errorf("unsupported compass focus %q", opts.Focus)
	}
	return nil
}

func cleanChannelCompassID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelCompassFocus(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "all", "default", "capabilities", "capability", "steps", "command":
		return "all"
	case "core", "status", "ops", "basics", "basic":
		return "core"
	case "skill", "skills", "capability-search":
		return "skills"
	case "tool", "tools", "tooling":
		return "tools"
	case "soul", "souls", "authority", "context":
		return "soul"
	case "memory", "memories", "recall":
		return "memory"
	case "backup", "backups", "recovery", "restore":
		return "backups"
	case "fun", "play", "signals", "presence":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelCompassNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelCompassTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelCompassNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelCompassSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-compass-source-%s", eventID(ev))
}

func autoChannelCompassID(ev Event, opts ChannelCompassOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("compass-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelCompassNotifyMessageID(ev Event, compassID string) string {
	seed := strings.Join([]string{eventID(ev), compassID}, "|")
	return fmt.Sprintf("gitclaw-channel-compass-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelCompassNotificationBody(opts ChannelCompassOptions) string {
	steps := channelCompassStepsForFocus(opts.Focus)
	var b strings.Builder
	b.WriteString("GitClaw channel compass.\n\n")
	fmt.Fprintf(&b, "Focus: %s\n", opts.Focus)
	b.WriteString("Next safe steps:\n")
	for _, command := range steps {
		fmt.Fprintf(&b, "- %s\n", command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nCompass hash: %s\n", shortDocumentHash(opts.Focus+"|"+strings.Join(steps, "|")))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nCompass source: GitHub channel action.\n")
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

func channelCompassStepsForFocus(focus string) []string {
	switch cleanChannelCompassFocus(focus) {
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
	case "memory":
		return []string{
			"/channels memory-status --message-id <id>",
			"/channels memory-search <query> --message-id <id> --notify-message-id <id>",
			"/channels memory-note --note-id <id> --target <target> --message-id <id>",
			"/channels propose-memory --target <target> --id <id> --message-id <id>",
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
			"/channels memory-search <query> --message-id <id> --notify-message-id <id>",
			"/channels backup-search <query> --message-id <id> --notify-message-id <id>",
			"/channels roll --dice 2d6 --message-id <id> --notify-message-id <id>",
			"/channels mood <mood> --message-id <id> --notify-message-id <id>",
			"/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>",
		}
	default:
		return nil
	}
}
