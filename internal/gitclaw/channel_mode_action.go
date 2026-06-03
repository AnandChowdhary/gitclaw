package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelModeOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ModeID            string
	Focus             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelModeResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	ModeIDHash   string
	FocusHash    string
	NoteHash     string
	BodyHash     string
	StepCount    int
}

type ChannelModeActionRequest struct {
	Options             ChannelModeOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoModeID          bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ModeIDHash          string
	FocusSHA            string
	FocusBytes          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	StepCount           int
	NotificationBodySHA string
}

func IsChannelModeActionRequest(ev Event, cfg Config) bool {
	return isChannelModeActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelModeActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "mode", "modes", "set-mode", "thread-mode", "stance", "posture":
		return true
	default:
		return false
	}
}

func BuildChannelModeActionRequest(ev Event, cfg Config) (ChannelModeActionRequest, error) {
	fields, trailing, ok := channelModeActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelModeActionRequest{}, fmt.Errorf("missing channel mode command")
	}
	req := ChannelModeActionRequest{
		Options: ChannelModeOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             "focus",
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
				return ChannelModeActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--mode-id", "--stance-id", "--posture-id", "--id":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ModeID = cleanChannelModeID(fields[i+1])
			i++
		case "--mode", "--stance", "--posture", "--state", "--for":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelModeActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelModeActionRequest{}, fmt.Errorf("unknown channel mode argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelModeIssueTargetIfPresent(ev, &req)
	if err := applyChannelModePositionals(&req, positional); err != nil {
		return ChannelModeActionRequest{}, err
	}
	if err := applyChannelModeIssueTarget(ev, &req); err != nil {
		return ChannelModeActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelModeTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelModeSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ModeID) == "" {
		req.Options.ModeID = autoChannelModeID(ev, req.Options)
		req.AutoModeID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelModeNotifyMessageID(ev, req.Options.ModeID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelModeOptions(req.Options)
	if err := validateChannelModeActionRequestOptions(req.Options); err != nil {
		return ChannelModeActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ModeIDHash = shortDocumentHash(req.Options.ModeID)
	req.FocusSHA = shortDocumentHash(req.Options.Focus)
	req.FocusBytes = len(req.Options.Focus)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.StepCount = len(channelModeStepsForFocus(req.Options.Focus))
	req.NotificationBodySHA = shortDocumentHash(renderChannelModeNotificationBody(req.Options))
	return req, nil
}

func RunChannelMode(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelModeOptions) (ChannelModeResult, error) {
	opts = normalizeChannelModeOptions(opts)
	var err error
	opts, err = applyChannelModeRoute(cfg, opts)
	if err != nil {
		return ChannelModeResult{}, err
	}
	if err := validateChannelModeOptions(opts); err != nil {
		return ChannelModeResult{}, err
	}
	body := renderChannelModeNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelModeResult{}, fmt.Errorf("queue channel mode notification: %w", err)
	}
	return ChannelModeResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		ModeIDHash:   shortDocumentHash(opts.ModeID),
		FocusHash:    shortDocumentHash(opts.Focus),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
		StepCount:    len(channelModeStepsForFocus(opts.Focus)),
	}, nil
}

func RenderChannelModeActionReport(ev Event, req ChannelModeActionRequest, result ChannelModeResult) string {
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
	modeIDHash := result.ModeIDHash
	if modeIDHash == "" {
		modeIDHash = req.ModeIDHash
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
	b.WriteString("## GitClaw Channel Mode Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_mode_status: `%s`\n", status)
	fmt.Fprintf(&b, "- mode_card_mode: `%s`\n", "structured-channel-mode")
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
	fmt.Fprintf(&b, "- mode_id_sha256_12: `%s`\n", noneIfEmpty(modeIDHash))
	fmt.Fprintf(&b, "- mode_id_auto: `%t`\n", req.AutoModeID)
	fmt.Fprintf(&b, "- mode_name_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- mode_name_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- mode_step_count: `%d`\n", stepCount)
	fmt.Fprintf(&b, "- mode_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- mode_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- mode_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- mode_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- mode_persistence_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- schedule_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mode_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mode_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mode_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mode_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_mode_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel mode card on the canonical channel issue. This gives the chat thread an advisory posture selected by the caller while keeping command execution, skill installs, tool execution, backup payload reads, soul body reads, provider API calls, model calls, provider delivery, workflow edits, policy changes, schedules, durable mode persistence, and repository mutations out of this action. The source receipt keeps thread ids, message ids, mode ids, mode names, notes, step text, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read mode updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent mode updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate mode updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelModeActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelModeActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelModeIssueTarget(ev Event, req *ChannelModeActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel mode requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelModeIssueTargetIfPresent(ev Event, req *ChannelModeActionRequest) {
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

func applyChannelModePositionals(req *ChannelModeActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Focus == "" || req.Options.Focus == "focus" {
				req.Options.Focus = value
				continue
			}
			return fmt.Errorf("unexpected channel mode argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Focus == "" || req.Options.Focus == "focus" {
			req.Options.Focus = value
			continue
		}
		return fmt.Errorf("unexpected channel mode argument %q", value)
	}
	return nil
}

func normalizeChannelModeOptions(opts ChannelModeOptions) ChannelModeOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ModeID = cleanChannelModeID(opts.ModeID)
	opts.Focus = cleanChannelModeFocus(opts.Focus)
	opts.Note = cleanChannelModeNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelModeRoute(cfg Config, opts ChannelModeOptions) (ChannelModeOptions, error) {
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
		Body:      "GitClaw channel mode.",
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

func validateChannelModeOptions(opts ChannelModeOptions) error {
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
	if opts.ModeID == "" {
		return fmt.Errorf("missing mode id")
	}
	if opts.Focus == "" {
		return fmt.Errorf("missing mode name")
	}
	if len(channelModeStepsForFocus(opts.Focus)) == 0 {
		return fmt.Errorf("unsupported mode name %q", opts.Focus)
	}
	return nil
}

func validateChannelModeActionRequestOptions(opts ChannelModeOptions) error {
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
	if opts.ModeID == "" {
		return fmt.Errorf("missing mode id")
	}
	if opts.Focus == "" {
		return fmt.Errorf("missing mode name")
	}
	if len(channelModeStepsForFocus(opts.Focus)) == 0 {
		return fmt.Errorf("unsupported mode name %q", opts.Focus)
	}
	return nil
}

func cleanChannelModeID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelModeFocus(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "focus", "focused", "deep-work", "default", "work":
		return "focus"
	case "pair", "pairing", "collab", "collaboration", "co-work", "cowork":
		return "pairing"
	case "triage", "inbox", "sort", "sorting", "review":
		return "triage"
	case "recovery", "restore", "incident-recovery", "rollback":
		return "recovery"
	case "tool", "tools", "tool-review", "review-tools", "approval", "approvals":
		return "tool-review"
	case "soul", "souls", "soul-review", "identity", "authority", "context":
		return "soul-review"
	case "backup", "backups", "backup-review", "recovery-review":
		return "backup-review"
	case "quiet", "dnd", "do-not-disturb", "heads-down", "mute":
		return "quiet"
	default:
		return ""
	}
}

func cleanChannelModeNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelModeTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelModeNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelModeSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-mode-source-%s", eventID(ev))
}

func autoChannelModeID(ev Event, opts ChannelModeOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return fmt.Sprintf("mode-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelModeNotifyMessageID(ev Event, modeID string) string {
	seed := strings.Join([]string{eventID(ev), modeID}, "|")
	return fmt.Sprintf("gitclaw-channel-mode-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelModeNotificationBody(opts ChannelModeOptions) string {
	steps := channelModeStepsForFocus(opts.Focus)
	var b strings.Builder
	b.WriteString("GitClaw channel mode.\n\n")
	fmt.Fprintf(&b, "Mode: %s\n", opts.Focus)
	fmt.Fprintf(&b, "Posture: %s\n", channelModePosture(opts.Focus))
	b.WriteString("Suggested next steps:\n")
	for _, command := range steps {
		fmt.Fprintf(&b, "- %s\n", command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nMode hash: %s\n", shortDocumentHash(opts.Focus+"|"+strings.Join(steps, "|")))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("Mode persistence: advisory only; no durable channel state changed.\n")
	b.WriteString("\nMode source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Policy mutation: not performed by this action.\n")
	b.WriteString("Schedule creation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelModePosture(focus string) string {
	switch cleanChannelModeFocus(focus) {
	case "focus":
		return "Keep the thread narrow, concrete, and ready for one reviewed next action."
	case "pairing":
		return "Coordinate humans and reviewed routes before turning conversation into work."
	case "triage":
		return "Sort loose channel inputs into reviewable GitHub tasks, loops, and checklists."
	case "recovery":
		return "Inspect rollback and backup readiness before any restore or destructive action."
	case "tool-review":
		return "Review tool context, approvals, and run plans before any tool execution."
	case "soul-review":
		return "Review high-authority context changes without writing soul or memory files."
	case "backup-review":
		return "Inspect backup metadata and restore requests without reading payloads or restoring files."
	case "quiet":
		return "Minimize channel noise and leave reviewed breadcrumbs for later resumption."
	default:
		return "Keep the thread explicit and review-first."
	}
}

func channelModeStepsForFocus(focus string) []string {
	switch cleanChannelModeFocus(focus) {
	case "focus":
		return []string{
			"/channels topic --topic-id <id>",
			"/channels availability --message-id <id> --notify-message-id <id>",
			"/channels agenda --agenda-id <id> --message-id <id>",
			"/channels checklist --checklist-id <id> --message-id <id>",
		}
	case "pairing":
		return []string{
			"/channels huddle <route-a>,<route-b> --huddle-id <id> --message-id <id>",
			"/channels room <route-a>,<route-b> --room-id <id> --message-id <id>",
			"/channels invite <route-a>,<route-b> --message-id <id>",
			"/channels rollcall <route-a>,<route-b> --rollcall-id <id> --message-id <id>",
		}
	case "triage":
		return []string{
			"/channels task --task-id <id> --message-id <id>",
			"/channels open-loop --loop-id <id> --message-id <id>",
			"/channels checklist --checklist-id <id> --message-id <id>",
			"/channels board-card --card-id <id> --lane <lane> --message-id <id>",
		}
	case "recovery":
		return []string{
			"/channels checkpoint-status --message-id <id> --notify-message-id <id>",
			"/channels backup --message-id <id>",
			"/channels restore-request --id <id> --message-id <id>",
			"/channels rehearse-checkpoint --target HEAD~1 --id <id> --message-id <id>",
		}
	case "tool-review":
		return []string{
			"/channels tools --message-id <id>",
			"/channels tool-search <query> --message-id <id> --notify-message-id <id>",
			"/channels tool-info <tool> --message-id <id> --notify-message-id <id>",
			"/channels approval-plan <tool> --id <id> --message-id <id>",
		}
	case "soul-review":
		return []string{
			"/channels soul-status --message-id <id>",
			"/channels soul-search <query> --message-id <id> --notify-message-id <id>",
			"/channels soul-info <path> --message-id <id> --notify-message-id <id>",
			"/channels propose-soul --target soul --id <id> --message-id <id>",
		}
	case "backup-review":
		return []string{
			"/channels backup --message-id <id>",
			"/channels backup-search <query> --message-id <id> --notify-message-id <id>",
			"/channels backup-info <issue> --message-id <id> --notify-message-id <id>",
			"/channels restore-request --id <id> --message-id <id>",
		}
	case "quiet":
		return []string{
			"/channels availability --message-id <id> --notify-message-id <id>",
			"/channels topic --topic-id <id>",
			"/channels journal --journal-id <id> --date <date> --message-id <id>",
			"/channels done --message-id <id>",
		}
	default:
		return nil
	}
}
