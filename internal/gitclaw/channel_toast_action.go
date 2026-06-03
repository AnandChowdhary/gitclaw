package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToastOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ToastID           string
	Title             string
	Reason            string
	Tone              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToastResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	ToastIDHash  string
	TitleHash    string
	ReasonHash   string
	ToneHash     string
	BodyHash     string
}

type ChannelToastActionRequest struct {
	Options             ChannelToastOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoToastID         bool
	TargetFromIssue     bool
	TitleSource         string
	ReasonSource        string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ToastIDHash         string
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ReasonSHA           string
	ReasonBytes         int
	ReasonLines         int
	ToneSHA             string
	ToneBytes           int
	NotificationBodySHA string
}

func IsChannelToastActionRequest(ev Event, cfg Config) bool {
	return isChannelToastActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToastActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "toast", "cheer", "cheers", "salute", "ship-it", "shipit", "high-five", "highfive":
		return true
	default:
		return false
	}
}

func BuildChannelToastActionRequest(ev Event, cfg Config) (ChannelToastActionRequest, error) {
	fields, trailing, ok := channelToastActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToastActionRequest{}, fmt.Errorf("missing channel toast command")
	}
	req := ChannelToastActionRequest{
		Options: ChannelToastOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Tone:              defaultChannelToastToneForSubcommand(fields[1]),
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
				return ChannelToastActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--toast-id", "--cheer-id", "--salute-id", "--ship-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ToastID = cleanChannelToastID(fields[i+1])
			i++
		case "--toast", "--title", "--headline":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Title = fields[i+1]
			req.TitleSource = "flag"
			i++
		case "--reason", "--note", "--because":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Reason = fields[i+1]
			req.ReasonSource = "flag"
			i++
		case "--tone", "--style":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Tone = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToastActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToastActionRequest{}, fmt.Errorf("unknown channel toast argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelToastIssueTargetIfPresent(ev, &req)
	if err := applyChannelToastPositionals(&req, positional); err != nil {
		return ChannelToastActionRequest{}, err
	}
	if err := applyChannelToastIssueTarget(ev, &req); err != nil {
		return ChannelToastActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.Title) == "" {
		req.Options.Title = parseChannelToastTrailingTitle(trailing)
		if req.Options.Title != "" {
			req.TitleSource = "trailing-title"
		}
	}
	if strings.TrimSpace(req.Options.Reason) == "" {
		req.Options.Reason = parseChannelToastTrailingReason(trailing)
		if req.Options.Reason != "" {
			req.ReasonSource = "trailing-reason"
		}
	}
	if strings.TrimSpace(req.Options.Title) == "" {
		req.Options.Title = defaultChannelToastTitleForSubcommand(req.Subcommand)
		req.TitleSource = "default"
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelToastSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ToastID) == "" {
		req.Options.ToastID = autoChannelToastID(ev, req.Options)
		req.AutoToastID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToastNotifyMessageID(ev, req.Options.ToastID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToastOptions(req.Options)
	if err := validateChannelToastActionRequestOptions(req.Options); err != nil {
		return ChannelToastActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ToastIDHash = shortDocumentHash(req.Options.ToastID)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ReasonSHA = shortDocumentHash(req.Options.Reason)
	req.ReasonBytes = len(req.Options.Reason)
	req.ReasonLines = lineCount(req.Options.Reason)
	req.ToneSHA = shortDocumentHash(req.Options.Tone)
	req.ToneBytes = len(req.Options.Tone)
	req.NotificationBodySHA = shortDocumentHash(renderChannelToastNotificationBody(req.Options))
	return req, nil
}

func RunChannelToast(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToastOptions) (ChannelToastResult, error) {
	opts = normalizeChannelToastOptions(opts)
	var err error
	opts, err = applyChannelToastRoute(cfg, opts)
	if err != nil {
		return ChannelToastResult{}, err
	}
	if err := validateChannelToastOptions(opts); err != nil {
		return ChannelToastResult{}, err
	}
	body := renderChannelToastNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelToastResult{}, fmt.Errorf("queue channel toast notification: %w", err)
	}
	return ChannelToastResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		ToastIDHash:  shortDocumentHash(opts.ToastID),
		TitleHash:    shortDocumentHash(opts.Title),
		ReasonHash:   shortDocumentHash(opts.Reason),
		ToneHash:     shortDocumentHash(opts.Tone),
		BodyHash:     shortDocumentHash(body),
	}, nil
}

func RenderChannelToastActionReport(ev Event, req ChannelToastActionRequest, result ChannelToastResult) string {
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
	toastIDHash := result.ToastIDHash
	if toastIDHash == "" {
		toastIDHash = req.ToastIDHash
	}
	titleHash := result.TitleHash
	if titleHash == "" {
		titleHash = req.TitleSHA
	}
	reasonHash := result.ReasonHash
	if reasonHash == "" {
		reasonHash = req.ReasonSHA
	}
	toneHash := result.ToneHash
	if toneHash == "" {
		toneHash = req.ToneSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Toast Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_toast_status: `%s`\n", status)
	fmt.Fprintf(&b, "- toast_mode: `%s`\n", "provider-facing-celebration")
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
	fmt.Fprintf(&b, "- toast_id_sha256_12: `%s`\n", noneIfEmpty(toastIDHash))
	fmt.Fprintf(&b, "- toast_id_auto: `%t`\n", req.AutoToastID)
	fmt.Fprintf(&b, "- toast_title_sha256_12: `%s`\n", noneIfEmpty(titleHash))
	fmt.Fprintf(&b, "- toast_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- toast_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- toast_title_source: `%s`\n", noneIfEmpty(req.TitleSource))
	fmt.Fprintf(&b, "- toast_reason_sha256_12: `%s`\n", noneIfEmpty(reasonHash))
	fmt.Fprintf(&b, "- toast_reason_bytes: `%d`\n", req.ReasonBytes)
	fmt.Fprintf(&b, "- toast_reason_lines: `%d`\n", req.ReasonLines)
	fmt.Fprintf(&b, "- toast_reason_source: `%s`\n", noneIfEmpty(req.ReasonSource))
	fmt.Fprintf(&b, "- toast_tone_sha256_12: `%s`\n", noneIfEmpty(toneHash))
	fmt.Fprintf(&b, "- toast_tone_bytes: `%d`\n", req.ToneBytes)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- kudos_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toast_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toast_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toast_reason_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_toast_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel toast on the canonical channel issue. This is the lightweight celebration lane: it lets a Slack/Telegram thread acknowledge a tiny win without opening a durable kudos issue, while the source receipt keeps thread ids, message ids, toast ids, titles, reasons, and channel bodies out of band. The action does not call a model, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read toast cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent toast cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate toast cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelToastActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToastActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToastIssueTarget(ev Event, req *ChannelToastActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel toast requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelToastIssueTargetIfPresent(ev Event, req *ChannelToastActionRequest) {
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

func applyChannelToastPositionals(req *ChannelToastActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Title == "" {
				req.Options.Title = value
				req.TitleSource = "positional-title"
				continue
			}
			return fmt.Errorf("unexpected channel toast argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Title == "" {
			req.Options.Title = value
			req.TitleSource = "positional-title"
			continue
		}
		return fmt.Errorf("unexpected channel toast argument %q", value)
	}
	return nil
}

func normalizeChannelToastOptions(opts ChannelToastOptions) ChannelToastOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ToastID = cleanChannelToastID(opts.ToastID)
	opts.Title = cleanChannelToastText(opts.Title, 160)
	opts.Reason = cleanChannelToastText(opts.Reason, 240)
	opts.Tone = cleanChannelToastTone(opts.Tone)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToastRoute(cfg Config, opts ChannelToastOptions) (ChannelToastOptions, error) {
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
		Body:      "GitClaw channel toast.",
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

func validateChannelToastOptions(opts ChannelToastOptions) error {
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
	if opts.ToastID == "" {
		return fmt.Errorf("missing toast id")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing toast title")
	}
	if opts.Tone == "" {
		return fmt.Errorf("missing toast tone")
	}
	return nil
}

func validateChannelToastActionRequestOptions(opts ChannelToastOptions) error {
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
	if opts.ToastID == "" {
		return fmt.Errorf("missing toast id")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing toast title")
	}
	if opts.Tone == "" {
		return fmt.Errorf("missing toast tone")
	}
	return nil
}

func cleanChannelToastID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelToastText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if maxLen > 0 && len(value) > maxLen {
		value = strings.TrimSpace(value[:maxLen])
	}
	return value
}

func cleanChannelToastTone(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "bright"
	}
	if len(value) > 32 {
		value = strings.Trim(value[:32], "-")
	}
	return value
}

func parseChannelToastTrailingTitle(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelToastTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"toast:", "title:", "headline:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelToastText(trimmed[idx+1:], 160)
				}
			}
		}
		return cleanChannelToastText(trimmed, 160)
	}
	return ""
}

func parseChannelToastTrailingReason(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelToastTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"reason:", "note:", "because:", "context:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelToastText(trimmed[idx+1:], 240)
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelToastTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelToastToneForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "salute":
		return "warm"
	case "ship-it", "shipit":
		return "victory"
	default:
		return "bright"
	}
}

func defaultChannelToastTitleForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "cheer", "cheers":
		return "Tiny win"
	case "salute":
		return "Worth a salute"
	case "ship-it", "shipit":
		return "Ship it"
	case "high-five", "highfive":
		return "High five"
	default:
		return "A small toast"
	}
}

func autoChannelToastSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-toast-source-%s", eventID(ev))
}

func autoChannelToastID(ev Event, opts ChannelToastOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Title, opts.Reason, opts.Tone}, "|")
	return fmt.Sprintf("toast-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToastNotifyMessageID(ev Event, toastID string) string {
	seed := strings.Join([]string{eventID(ev), toastID}, "|")
	return fmt.Sprintf("gitclaw-channel-toast-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelToastNotificationBody(opts ChannelToastOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel toast.\n\n")
	fmt.Fprintf(&b, "Toast: %s\n", opts.Title)
	fmt.Fprintf(&b, "Tone: %s\n", opts.Tone)
	if opts.Reason != "" {
		fmt.Fprintf(&b, "Reason: %s\n", opts.Reason)
	}
	fmt.Fprintf(&b, "Toast hash: %s\n", shortDocumentHash(opts.Title))
	if opts.Reason != "" {
		fmt.Fprintf(&b, "Reason hash: %s\n", shortDocumentHash(opts.Reason))
	}
	b.WriteString("\nToast source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Kudos issue: not created by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
