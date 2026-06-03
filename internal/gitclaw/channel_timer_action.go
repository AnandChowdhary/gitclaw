package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelTimerOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	TimerID           string
	Duration          string
	DurationSeconds   int
	Label             string
	Mode              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelTimerResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	TimerIDHash  string
	DurationHash string
	DurationSecs int
	LabelHash    string
	ModeHash     string
	NoteHash     string
	BodyHash     string
}

type ChannelTimerActionRequest struct {
	Options             ChannelTimerOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoTimerID         bool
	TargetFromIssue     bool
	DurationSource      string
	LabelSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	TimerIDHash         string
	DurationSHA         string
	DurationSeconds     int
	LabelSHA            string
	LabelBytes          int
	LabelLines          int
	ModeSHA             string
	ModeBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	NotificationBodySHA string
}

func IsChannelTimerActionRequest(ev Event, cfg Config) bool {
	return isChannelTimerActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelTimerActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "timer", "timebox", "countdown", "focus", "focus-timer", "pomodoro", "break":
		return true
	default:
		return false
	}
}

func BuildChannelTimerActionRequest(ev Event, cfg Config) (ChannelTimerActionRequest, error) {
	fields, trailing, ok := channelTimerActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelTimerActionRequest{}, fmt.Errorf("missing channel timer command")
	}
	req := ChannelTimerActionRequest{
		Options: ChannelTimerOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Duration:          defaultChannelTimerDurationForSubcommand(fields[1]),
			Label:             defaultChannelTimerLabelForSubcommand(fields[1]),
			Mode:              defaultChannelTimerModeForSubcommand(fields[1]),
		},
		Command:        strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:     strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		DurationSource: "default",
		LabelSource:    "default",
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
				return ChannelTimerActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--timer-id", "--timebox-id", "--countdown-id", "--focus-id", "--id":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TimerID = cleanChannelTimerID(fields[i+1])
			i++
		case "--duration", "--for", "--length":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Duration = fields[i+1]
			req.DurationSource = "flag"
			i++
		case "--minutes", "--mins":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Duration = strings.TrimSpace(fields[i+1]) + "m"
			req.DurationSource = "flag"
			i++
		case "--label", "--title", "--name":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Label = fields[i+1]
			req.LabelSource = "flag"
			i++
		case "--mode", "--lane":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Mode = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelTimerActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelTimerActionRequest{}, fmt.Errorf("unknown channel timer argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelTimerIssueTargetIfPresent(ev, &req)
	if err := applyChannelTimerPositionals(&req, positional); err != nil {
		return ChannelTimerActionRequest{}, err
	}
	if err := applyChannelTimerIssueTarget(ev, &req); err != nil {
		return ChannelTimerActionRequest{}, err
	}
	if label := parseChannelTimerTrailingLabel(trailing); label != "" && req.LabelSource == "default" {
		req.Options.Label = label
		req.LabelSource = "trailing-label"
	}
	if note := parseChannelTimerTrailingNote(trailing); note != "" && req.Options.Note == "" {
		req.Options.Note = note
		req.NoteSource = "trailing-note"
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelTimerSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.TimerID) == "" {
		req.Options.TimerID = autoChannelTimerID(ev, req.Options)
		req.AutoTimerID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelTimerNotifyMessageID(ev, req.Options.TimerID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelTimerOptions(req.Options)
	if err := validateChannelTimerActionRequestOptions(req.Options); err != nil {
		return ChannelTimerActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.TimerIDHash = shortDocumentHash(req.Options.TimerID)
	req.DurationSHA = shortDocumentHash(req.Options.Duration)
	req.DurationSeconds = req.Options.DurationSeconds
	req.LabelSHA = shortDocumentHash(req.Options.Label)
	req.LabelBytes = len(req.Options.Label)
	req.LabelLines = lineCount(req.Options.Label)
	req.ModeSHA = shortDocumentHash(req.Options.Mode)
	req.ModeBytes = len(req.Options.Mode)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelTimerNotificationBody(req.Options))
	return req, nil
}

func RunChannelTimer(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelTimerOptions) (ChannelTimerResult, error) {
	opts = normalizeChannelTimerOptions(opts)
	var err error
	opts, err = applyChannelTimerRoute(cfg, opts)
	if err != nil {
		return ChannelTimerResult{}, err
	}
	if err := validateChannelTimerOptions(opts); err != nil {
		return ChannelTimerResult{}, err
	}
	body := renderChannelTimerNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelTimerResult{}, fmt.Errorf("queue channel timer notification: %w", err)
	}
	return ChannelTimerResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		TimerIDHash:  shortDocumentHash(opts.TimerID),
		DurationHash: shortDocumentHash(opts.Duration),
		DurationSecs: opts.DurationSeconds,
		LabelHash:    shortDocumentHash(opts.Label),
		ModeHash:     shortDocumentHash(opts.Mode),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
	}, nil
}

func RenderChannelTimerActionReport(ev Event, req ChannelTimerActionRequest, result ChannelTimerResult) string {
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
	timerIDHash := result.TimerIDHash
	if timerIDHash == "" {
		timerIDHash = req.TimerIDHash
	}
	durationHash := result.DurationHash
	if durationHash == "" {
		durationHash = req.DurationSHA
	}
	durationSeconds := result.DurationSecs
	if durationSeconds == 0 {
		durationSeconds = req.DurationSeconds
	}
	labelHash := result.LabelHash
	if labelHash == "" {
		labelHash = req.LabelSHA
	}
	modeHash := result.ModeHash
	if modeHash == "" {
		modeHash = req.ModeSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Timer Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_timer_status: `%s`\n", status)
	fmt.Fprintf(&b, "- timer_mode: `%s`\n", "provider-facing-timebox-cue")
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
	fmt.Fprintf(&b, "- timer_id_sha256_12: `%s`\n", noneIfEmpty(timerIDHash))
	fmt.Fprintf(&b, "- timer_id_auto: `%t`\n", req.AutoTimerID)
	fmt.Fprintf(&b, "- timer_duration_sha256_12: `%s`\n", noneIfEmpty(durationHash))
	fmt.Fprintf(&b, "- timer_duration_seconds: `%d`\n", durationSeconds)
	fmt.Fprintf(&b, "- timer_duration_source: `%s`\n", noneIfEmpty(req.DurationSource))
	fmt.Fprintf(&b, "- timer_label_sha256_12: `%s`\n", noneIfEmpty(labelHash))
	fmt.Fprintf(&b, "- timer_label_bytes: `%d`\n", req.LabelBytes)
	fmt.Fprintf(&b, "- timer_label_lines: `%d`\n", req.LabelLines)
	fmt.Fprintf(&b, "- timer_label_source: `%s`\n", noneIfEmpty(req.LabelSource))
	fmt.Fprintf(&b, "- timer_mode_sha256_12: `%s`\n", noneIfEmpty(modeHash))
	fmt.Fprintf(&b, "- timer_mode_bytes: `%d`\n", req.ModeBytes)
	fmt.Fprintf(&b, "- timer_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- timer_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- timer_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- timer_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_timer_started: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_timer_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_timer_duration_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_timer_label_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_timer_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_timer_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel timer card on the canonical channel issue. This is a lightweight timebox cue for Slack/Telegram threads: it does not create a reminder issue, schedule a workflow, start a provider timer, call a model, mutate repository files, or call provider APIs. The source receipt keeps thread ids, message ids, timer ids, labels, notes, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read timer cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent timer cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate timer cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelTimerActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelTimerActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelTimerIssueTarget(ev Event, req *ChannelTimerActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel timer requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelTimerIssueTargetIfPresent(ev Event, req *ChannelTimerActionRequest) {
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

func applyChannelTimerPositionals(req *ChannelTimerActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if isChannelTimerDurationLike(value) && req.DurationSource == "default" {
				req.Options.Duration = value
				req.DurationSource = "positional-duration"
				continue
			}
			if req.LabelSource == "default" {
				req.Options.Label = value
				req.LabelSource = "positional-label"
				continue
			}
			return fmt.Errorf("unexpected channel timer argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if isChannelTimerDurationLike(value) && req.DurationSource == "default" {
			req.Options.Duration = value
			req.DurationSource = "positional-duration"
			continue
		}
		if req.LabelSource == "default" {
			req.Options.Label = value
			req.LabelSource = "positional-label"
			continue
		}
		return fmt.Errorf("unexpected channel timer argument %q", value)
	}
	return nil
}

func normalizeChannelTimerOptions(opts ChannelTimerOptions) ChannelTimerOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.TimerID = cleanChannelTimerID(opts.TimerID)
	opts.Duration, opts.DurationSeconds, _ = normalizeChannelTimerDuration(opts.Duration)
	opts.Label = cleanChannelTimerText(opts.Label, 160)
	opts.Mode = cleanChannelTimerMode(opts.Mode)
	opts.Note = cleanChannelTimerText(opts.Note, 240)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelTimerRoute(cfg Config, opts ChannelTimerOptions) (ChannelTimerOptions, error) {
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
		Body:      "GitClaw channel timer.",
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

func validateChannelTimerOptions(opts ChannelTimerOptions) error {
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
	if opts.TimerID == "" {
		return fmt.Errorf("missing timer id")
	}
	if opts.Duration == "" || opts.DurationSeconds <= 0 {
		return fmt.Errorf("missing timer duration")
	}
	if opts.Label == "" {
		return fmt.Errorf("missing timer label")
	}
	if opts.Mode == "" {
		return fmt.Errorf("missing timer mode")
	}
	return nil
}

func validateChannelTimerActionRequestOptions(opts ChannelTimerOptions) error {
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
	if opts.TimerID == "" {
		return fmt.Errorf("missing timer id")
	}
	if opts.Duration == "" || opts.DurationSeconds <= 0 {
		return fmt.Errorf("missing timer duration")
	}
	if opts.Label == "" {
		return fmt.Errorf("missing timer label")
	}
	return nil
}

func cleanChannelTimerID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelTimerText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if maxLen > 0 && len(value) > maxLen {
		value = strings.TrimSpace(value[:maxLen])
	}
	return value
}

func cleanChannelTimerMode(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "focus"
	}
	if len(value) > 32 {
		value = strings.Trim(value[:32], "-")
	}
	return value
}

func isChannelTimerDurationLike(value string) bool {
	_, _, ok := normalizeChannelTimerDuration(value)
	return ok
}

func normalizeChannelTimerDuration(value string) (string, int, bool) {
	cleaned := strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if cleaned == "" {
		return "", 0, false
	}
	digitEnd := 0
	for digitEnd < len(cleaned) && cleaned[digitEnd] >= '0' && cleaned[digitEnd] <= '9' {
		digitEnd++
	}
	if digitEnd == 0 {
		return "", 0, false
	}
	amount, err := strconv.Atoi(cleaned[:digitEnd])
	if err != nil || amount <= 0 {
		return "", 0, false
	}
	unit := cleaned[digitEnd:]
	if unit == "" {
		unit = "m"
	}
	seconds := 0
	switch unit {
	case "s", "sec", "secs", "second", "seconds":
		seconds = amount
	case "m", "min", "mins", "minute", "minutes":
		seconds = amount * 60
	case "h", "hr", "hrs", "hour", "hours":
		seconds = amount * 3600
	default:
		return "", 0, false
	}
	if seconds <= 0 || seconds > 24*60*60 {
		return "", 0, false
	}
	return canonicalChannelTimerDuration(seconds), seconds, true
}

func canonicalChannelTimerDuration(seconds int) string {
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatChannelTimerDuration(seconds int) string {
	switch {
	case seconds%3600 == 0:
		hours := seconds / 3600
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	case seconds%60 == 0:
		minutes := seconds / 60
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	default:
		if seconds == 1 {
			return "1 second"
		}
		return fmt.Sprintf("%d seconds", seconds)
	}
}

func parseChannelTimerTrailingLabel(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelTimerTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"label:", "title:", "timer:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelTimerText(trimmed[idx+1:], 160)
				}
			}
		}
	}
	return ""
}

func parseChannelTimerTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelTimerTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelTimerText(trimmed[idx+1:], 240)
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelTimerTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelTimerDurationForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "break":
		return "5m"
	default:
		return "25m"
	}
}

func defaultChannelTimerLabelForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "break":
		return "Break timer"
	case "pomodoro":
		return "Pomodoro timer"
	case "timebox":
		return "Timebox"
	case "countdown":
		return "Countdown"
	default:
		return "Focus timer"
	}
}

func defaultChannelTimerModeForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "break":
		return "break"
	case "pomodoro":
		return "pomodoro"
	default:
		return "focus"
	}
}

func autoChannelTimerSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-timer-source-%s", eventID(ev))
}

func autoChannelTimerID(ev Event, opts ChannelTimerOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Duration, opts.Label, opts.Mode, opts.Note}, "|")
	return fmt.Sprintf("timer-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelTimerNotifyMessageID(ev Event, timerID string) string {
	seed := strings.Join([]string{eventID(ev), timerID}, "|")
	return fmt.Sprintf("gitclaw-channel-timer-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelTimerNotificationBody(opts ChannelTimerOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel timer.\n\n")
	fmt.Fprintf(&b, "Timer: %s\n", opts.Label)
	fmt.Fprintf(&b, "Duration: %s\n", formatChannelTimerDuration(opts.DurationSeconds))
	fmt.Fprintf(&b, "Mode: %s\n", opts.Mode)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "Timer hash: %s\n", shortDocumentHash(opts.Label))
	fmt.Fprintf(&b, "Duration seconds: %d\n", opts.DurationSeconds)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nTimer source: GitHub channel action.\n")
	b.WriteString("Scheduled reminder: not created by this action.\n")
	b.WriteString("Provider timer: not started by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
