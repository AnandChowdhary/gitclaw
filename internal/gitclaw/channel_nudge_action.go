package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelNudgeOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	NudgeID           string
	Target            string
	Tone              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelNudgeResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	NudgeIDHash  string
	TargetHash   string
	ToneHash     string
	NoteHash     string
	BodyHash     string
}

type ChannelNudgeActionRequest struct {
	Options             ChannelNudgeOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoNudgeID         bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NudgeIDHash         string
	TargetSHA           string
	TargetBytes         int
	ToneSHA             string
	ToneBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	NotificationBodySHA string
}

func IsChannelNudgeActionRequest(ev Event, cfg Config) bool {
	return isChannelNudgeActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelNudgeActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "nudge", "tap", "bump", "heads-up", "headsup", "attention":
		return true
	default:
		return false
	}
}

func BuildChannelNudgeActionRequest(ev Event, cfg Config) (ChannelNudgeActionRequest, error) {
	fields, trailing, ok := channelNudgeActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelNudgeActionRequest{}, fmt.Errorf("missing channel nudge command")
	}
	req := ChannelNudgeActionRequest{
		Options: ChannelNudgeOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Tone:              "gentle",
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
				return ChannelNudgeActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--nudge-id", "--tap-id", "--bump-id", "--id":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NudgeID = cleanChannelNudgeID(fields[i+1])
			i++
		case "--to", "--target", "--recipient":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Target = fields[i+1]
			i++
		case "--tone", "--urgency":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Tone = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelNudgeActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelNudgeActionRequest{}, fmt.Errorf("unknown channel nudge argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelNudgeIssueTargetIfPresent(ev, &req)
	if err := applyChannelNudgePositionals(&req, positional); err != nil {
		return ChannelNudgeActionRequest{}, err
	}
	if err := applyChannelNudgeIssueTarget(ev, &req); err != nil {
		return ChannelNudgeActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelNudgeTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.Target) == "" {
		req.Options.Target = defaultChannelNudgeTargetForSubcommand(req.Subcommand)
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelNudgeSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.NudgeID) == "" {
		req.Options.NudgeID = autoChannelNudgeID(ev, req.Options)
		req.AutoNudgeID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelNudgeNotifyMessageID(ev, req.Options.NudgeID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelNudgeOptions(req.Options)
	if err := validateChannelNudgeActionRequestOptions(req.Options); err != nil {
		return ChannelNudgeActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NudgeIDHash = shortDocumentHash(req.Options.NudgeID)
	req.TargetSHA = shortDocumentHash(req.Options.Target)
	req.TargetBytes = len(req.Options.Target)
	req.ToneSHA = shortDocumentHash(req.Options.Tone)
	req.ToneBytes = len(req.Options.Tone)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelNudgeNotificationBody(req.Options))
	return req, nil
}

func RunChannelNudge(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelNudgeOptions) (ChannelNudgeResult, error) {
	opts = normalizeChannelNudgeOptions(opts)
	var err error
	opts, err = applyChannelNudgeRoute(cfg, opts)
	if err != nil {
		return ChannelNudgeResult{}, err
	}
	if err := validateChannelNudgeOptions(opts); err != nil {
		return ChannelNudgeResult{}, err
	}
	body := renderChannelNudgeNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelNudgeResult{}, fmt.Errorf("queue channel nudge notification: %w", err)
	}
	return ChannelNudgeResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		NudgeIDHash:  shortDocumentHash(opts.NudgeID),
		TargetHash:   shortDocumentHash(opts.Target),
		ToneHash:     shortDocumentHash(opts.Tone),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
	}, nil
}

func RenderChannelNudgeActionReport(ev Event, req ChannelNudgeActionRequest, result ChannelNudgeResult) string {
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
	nudgeIDHash := result.NudgeIDHash
	if nudgeIDHash == "" {
		nudgeIDHash = req.NudgeIDHash
	}
	targetHash := result.TargetHash
	if targetHash == "" {
		targetHash = req.TargetSHA
	}
	toneHash := result.ToneHash
	if toneHash == "" {
		toneHash = req.ToneSHA
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
	b.WriteString("## GitClaw Channel Nudge Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_nudge_status: `%s`\n", status)
	fmt.Fprintf(&b, "- nudge_mode: `%s`\n", "structured-channel-nudge")
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
	fmt.Fprintf(&b, "- nudge_id_sha256_12: `%s`\n", noneIfEmpty(nudgeIDHash))
	fmt.Fprintf(&b, "- nudge_id_auto: `%t`\n", req.AutoNudgeID)
	fmt.Fprintf(&b, "- nudge_target_sha256_12: `%s`\n", noneIfEmpty(targetHash))
	fmt.Fprintf(&b, "- nudge_target_bytes: `%d`\n", req.TargetBytes)
	fmt.Fprintf(&b, "- nudge_tone_sha256_12: `%s`\n", noneIfEmpty(toneHash))
	fmt.Fprintf(&b, "- nudge_tone_bytes: `%d`\n", req.ToneBytes)
	fmt.Fprintf(&b, "- nudge_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- nudge_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- nudge_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- nudge_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- task_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- watch_created: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_nudge_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_nudge_target_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_nudge_tone_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_nudge_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_nudge_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel nudge on the canonical channel issue. This keeps channel conversations useful in the small: a person can ask for attention without creating a task, reminder, watch, scheduled workflow, provider API call, model call, or repository mutation. The source receipt keeps thread ids, message ids, nudge ids, targets, tones, notes, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read nudge updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent nudge updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate nudge updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelNudgeActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelNudgeActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelNudgeIssueTarget(ev Event, req *ChannelNudgeActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel nudge requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelNudgeIssueTargetIfPresent(ev Event, req *ChannelNudgeActionRequest) {
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

func applyChannelNudgePositionals(req *ChannelNudgeActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Target == "" {
				req.Options.Target = value
				continue
			}
			return fmt.Errorf("unexpected channel nudge argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Target == "" {
			req.Options.Target = value
			continue
		}
		return fmt.Errorf("unexpected channel nudge argument %q", value)
	}
	return nil
}

func normalizeChannelNudgeOptions(opts ChannelNudgeOptions) ChannelNudgeOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.NudgeID = cleanChannelNudgeID(opts.NudgeID)
	opts.Target = cleanChannelNudgeTarget(opts.Target)
	opts.Tone = cleanChannelNudgeTone(opts.Tone)
	opts.Note = cleanChannelNudgeNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelNudgeRoute(cfg Config, opts ChannelNudgeOptions) (ChannelNudgeOptions, error) {
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
		Body:      "GitClaw channel nudge.",
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

func validateChannelNudgeOptions(opts ChannelNudgeOptions) error {
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
	if opts.NudgeID == "" {
		return fmt.Errorf("missing nudge id")
	}
	if opts.Target == "" {
		return fmt.Errorf("missing nudge target")
	}
	if opts.Tone == "" {
		return fmt.Errorf("missing nudge tone")
	}
	return nil
}

func validateChannelNudgeActionRequestOptions(opts ChannelNudgeOptions) error {
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
	if opts.NudgeID == "" {
		return fmt.Errorf("missing nudge id")
	}
	if opts.Target == "" {
		return fmt.Errorf("missing nudge target")
	}
	if opts.Tone == "" {
		return fmt.Errorf("missing nudge tone")
	}
	return nil
}

func cleanChannelNudgeID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelNudgeTarget(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 80 {
		value = strings.TrimSpace(value[:80])
	}
	return value
}

func cleanChannelNudgeTone(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "gentle", "normal", "urgent":
		return value
	case "soft", "low":
		return "gentle"
	case "medium", "regular":
		return "normal"
	case "high", "now":
		return "urgent"
	default:
		return ""
	}
}

func cleanChannelNudgeNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelNudgeTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelNudgeNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func defaultChannelNudgeTargetForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "heads-up", "headsup", "attention":
		return "current-thread"
	default:
		return ""
	}
}

func autoChannelNudgeSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-nudge-source-%s", eventID(ev))
}

func autoChannelNudgeID(ev Event, opts ChannelNudgeOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Target, opts.Tone, opts.Note}, "|")
	return fmt.Sprintf("nudge-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelNudgeNotifyMessageID(ev Event, nudgeID string) string {
	seed := strings.Join([]string{eventID(ev), nudgeID}, "|")
	return fmt.Sprintf("gitclaw-channel-nudge-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelNudgeNotificationBody(opts ChannelNudgeOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel nudge.\n\n")
	fmt.Fprintf(&b, "Target: %s\n", opts.Target)
	fmt.Fprintf(&b, "Tone: %s\n", opts.Tone)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "Target hash: %s\n", shortDocumentHash(opts.Target))
	fmt.Fprintf(&b, "Tone hash: %s\n", shortDocumentHash(opts.Tone))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nNudge source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Task issue: not created by this action.\n")
	b.WriteString("Reminder: not created by this action.\n")
	b.WriteString("Watch: not created by this action.\n")
	b.WriteString("Scheduled workflow: not created by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
