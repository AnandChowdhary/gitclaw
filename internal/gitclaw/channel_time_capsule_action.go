package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelTimeCapsuleOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	CapsuleID         string
	OpenAfter         string
	Title             string
	Message           string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelTimeCapsuleResult struct {
	CapsuleIssueNumber int
	CapsuleIssueURL    string
	CapsuleCreated     bool
	CapsuleDuplicate   bool
	Notification       ChannelSendResult
	RouteName          string
	RouteHash          string
	Channel            string
	ThreadHash         string
	MessageHash        string
	NotifyHash         string
}

type ChannelTimeCapsuleActionRequest struct {
	Options             ChannelTimeCapsuleOptions
	Command             string
	Subcommand          string
	AutoCapsuleID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	OpenAfterSHA        string
	OpenAfterBytes      int
	OpenAfterLines      int
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	MessageSHA          string
	MessageBytes        int
	MessageLines        int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelTimeCapsuleActionRequest(ev Event, cfg Config) bool {
	return isChannelTimeCapsuleActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelTimeCapsuleActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "time-capsule", "capsule", "future-note", "save-for-later", "open-later":
		return true
	default:
		return false
	}
}

func BuildChannelTimeCapsuleActionRequest(ev Event, cfg Config) (ChannelTimeCapsuleActionRequest, error) {
	fields, trailing, ok := channelTimeCapsuleActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("missing channel time capsule command")
	}
	req := ChannelTimeCapsuleActionRequest{
		Options: ChannelTimeCapsuleOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--time-capsule-id", "--capsule-id", "--future-note-id", "--id":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.CapsuleID = cleanChannelTimeCapsuleID(fields[i+1])
			i++
		case "--open-after", "--open-on", "--not-before", "--date", "--time-capsule-date":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.OpenAfter = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("unknown channel time capsule argument %q", field)
			}
			if req.Options.CapsuleID == "" {
				req.Options.CapsuleID = cleanChannelTimeCapsuleID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelTimeCapsuleActionRequest{}, fmt.Errorf("unexpected channel time capsule argument %q", field)
		}
	}
	if err := applyChannelTimeCapsuleIssueTarget(ev, &req); err != nil {
		return ChannelTimeCapsuleActionRequest{}, err
	}
	openAfter, title, message := parseChannelTimeCapsuleTitleMessage(trailing, ev)
	if strings.TrimSpace(req.Options.OpenAfter) == "" {
		req.Options.OpenAfter = openAfter
	}
	if strings.TrimSpace(req.Options.OpenAfter) == "" {
		req.Options.OpenAfter = "unspecified-future"
	}
	req.Options.Title = title
	req.Options.Message = message
	if strings.TrimSpace(req.Options.CapsuleID) == "" {
		req.Options.CapsuleID = autoChannelTimeCapsuleID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.OpenAfter, title, message)
		req.AutoCapsuleID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelTimeCapsuleNotifyMessageID(ev, req.Options.CapsuleID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelTimeCapsuleOptions(req.Options)
	if err := validateChannelTimeCapsuleActionRequestOptions(req.Options); err != nil {
		return ChannelTimeCapsuleActionRequest{}, err
	}
	req.OpenAfterSHA = shortDocumentHash(req.Options.OpenAfter)
	req.OpenAfterBytes = len(req.Options.OpenAfter)
	req.OpenAfterLines = lineCount(req.Options.OpenAfter)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.MessageSHA = shortDocumentHash(req.Options.Message)
	req.MessageBytes = len(req.Options.Message)
	req.MessageLines = lineCount(req.Options.Message)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelTimeCapsuleNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelTimeCapsule(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelTimeCapsuleOptions) (ChannelTimeCapsuleResult, error) {
	opts = normalizeChannelTimeCapsuleOptions(opts)
	var err error
	opts, err = applyChannelTimeCapsuleRoute(cfg, opts)
	if err != nil {
		return ChannelTimeCapsuleResult{}, err
	}
	if err := validateChannelTimeCapsuleOptions(opts); err != nil {
		return ChannelTimeCapsuleResult{}, err
	}
	capsuleIssue, created, duplicate, err := findOrCreateChannelTimeCapsuleIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelTimeCapsuleResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelTimeCapsuleNotificationBody(opts, capsuleIssue.Number, issueURL(opts.Repo, capsuleIssue.Number)),
	})
	if err != nil {
		return ChannelTimeCapsuleResult{}, fmt.Errorf("queue channel time capsule notification: %w", err)
	}
	return ChannelTimeCapsuleResult{
		CapsuleIssueNumber: capsuleIssue.Number,
		CapsuleIssueURL:    issueURL(opts.Repo, capsuleIssue.Number),
		CapsuleCreated:     created,
		CapsuleDuplicate:   duplicate,
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelTimeCapsuleActionReport(ev Event, req ChannelTimeCapsuleActionRequest, result ChannelTimeCapsuleResult) string {
	status := "recorded"
	switch {
	case result.CapsuleDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.CapsuleDuplicate:
		status = "existing"
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Time Capsule Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_time_capsule_status: `%s`\n", status)
	fmt.Fprintf(&b, "- capsule_issue: `#%d`\n", result.CapsuleIssueNumber)
	fmt.Fprintf(&b, "- capsule_issue_url: `%s`\n", result.CapsuleIssueURL)
	fmt.Fprintf(&b, "- capsule_issue_created: `%t`\n", result.CapsuleCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.CapsuleDuplicate)
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
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- capsule_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.CapsuleID))
	fmt.Fprintf(&b, "- capsule_id_auto: `%t`\n", req.AutoCapsuleID)
	fmt.Fprintf(&b, "- open_after_sha256_12: `%s`\n", req.OpenAfterSHA)
	fmt.Fprintf(&b, "- open_after_bytes: `%d`\n", req.OpenAfterBytes)
	fmt.Fprintf(&b, "- open_after_lines: `%d`\n", req.OpenAfterLines)
	fmt.Fprintf(&b, "- capsule_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- capsule_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- capsule_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- capsule_message_sha256_12: `%s`\n", req.MessageSHA)
	fmt.Fprintf(&b, "- capsule_message_bytes: `%d`\n", req.MessageBytes)
	fmt.Fprintf(&b, "- capsule_message_lines: `%d`\n", req.MessageLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_capsule_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_open_after_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_capsule_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_capsule_message_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_delivery_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_time_capsule_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel time capsule as a durable GitHub issue, then queued a provider-facing link back to the original thread. The capsule issue contains the human-readable open-after hint, title, and message; this source receipt keeps provider IDs, capsule IDs, open-after hints, titles, messages, and channel message bodies out of band. This action does not schedule future delivery, create cron workflows, mutate repository files, or call a model.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the time-capsule-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent time-capsule links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate capsule issues are suppressed by `capsule_id`; duplicate time-capsule-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the capsule issue with the `gitclaw` label; scheduled reminders remain a separate explicit workflow\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelTimeCapsuleIssueBody(opts ChannelTimeCapsuleOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-time-capsule capsule_id=\"%s\" channel=\"%s\" open_after_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.CapsuleID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.OpenAfter), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel time capsule.\n\n")
	fmt.Fprintf(&b, "- capsule_id: %s\n", opts.CapsuleID)
	fmt.Fprintf(&b, "- open_after: %s\n", opts.OpenAfter)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- time_capsule_mode: github-issue-time-capsule\n")
	fmt.Fprintf(&b, "- scheduled_delivery_created: false\n")
	fmt.Fprintf(&b, "- reminder_created: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Open After\n\n")
	b.WriteString(strings.TrimSpace(opts.OpenAfter))
	b.WriteString("\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Message) != "" {
		b.WriteString("\n\n## Message\n\n")
		b.WriteString(strings.TrimSpace(opts.Message))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel time capsule. Opening it later, turning it into a reminder, or promoting it into `.gitclaw/MEMORY.md` should happen only through normal reviewed GitHub conversation.")
	return strings.TrimSpace(b.String())
}

func channelTimeCapsuleActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelTimeCapsuleActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelTimeCapsuleIssueTarget(ev Event, req *ChannelTimeCapsuleActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel time capsule requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelTimeCapsuleTitleMessage(trailing string, ev Event) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	var openAfter string
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "open after:"):
			if openAfter == "" {
				openAfter = strings.TrimSpace(trimmed[len("open after:"):])
			}
			continue
		case strings.HasPrefix(lower, "open on:"):
			if openAfter == "" {
				openAfter = strings.TrimSpace(trimmed[len("open on:"):])
			}
			continue
		case strings.HasPrefix(lower, "not before:"):
			if openAfter == "" {
				openAfter = strings.TrimSpace(trimmed[len("not before:"):])
			}
			continue
		case strings.HasPrefix(lower, "date:"):
			if openAfter == "" {
				openAfter = strings.TrimSpace(trimmed[len("date:"):])
			}
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel time capsule from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return openAfter, defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var title string
	var highlightLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "title:"):
		title = strings.TrimSpace(first[len("title:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "capsule:"):
		title = strings.TrimSpace(first[len("capsule:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "time capsule:"):
		title = strings.TrimSpace(first[len("time capsule:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "message:"), strings.HasPrefix(lowerFirst, "note:"), strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "details:"), strings.HasPrefix(lowerFirst, "context:"):
		title = defaultTitle
		highlightLines = cleaned
	default:
		title = first
		highlightLines = cleaned[1:]
	}
	if title == "" {
		title = defaultTitle
	}
	message := strings.TrimSpace(strings.Join(highlightLines, "\n"))
	messageLower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.HasPrefix(messageLower, "message:"):
		message = strings.TrimSpace(strings.TrimSpace(message)[len("message:"):])
	case strings.HasPrefix(messageLower, "note:"):
		message = strings.TrimSpace(strings.TrimSpace(message)[len("note:"):])
	case strings.HasPrefix(messageLower, "notes:"):
		message = strings.TrimSpace(strings.TrimSpace(message)[len("notes:"):])
	case strings.HasPrefix(messageLower, "details:"):
		message = strings.TrimSpace(strings.TrimSpace(message)[len("details:"):])
	case strings.HasPrefix(messageLower, "context:"):
		message = strings.TrimSpace(strings.TrimSpace(message)[len("context:"):])
	}
	return openAfter, title, message
}

func normalizeChannelTimeCapsuleOptions(opts ChannelTimeCapsuleOptions) ChannelTimeCapsuleOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.CapsuleID = cleanChannelTimeCapsuleID(opts.CapsuleID)
	opts.OpenAfter = strings.TrimSpace(opts.OpenAfter)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Message = strings.TrimSpace(opts.Message)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelTimeCapsuleRoute(cfg Config, opts ChannelTimeCapsuleOptions) (ChannelTimeCapsuleOptions, error) {
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
		Body:      opts.Title,
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

func validateChannelTimeCapsuleOptions(opts ChannelTimeCapsuleOptions) error {
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
	if opts.CapsuleID == "" {
		return fmt.Errorf("missing time capsule id")
	}
	if opts.OpenAfter == "" {
		return fmt.Errorf("missing open after")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing time capsule source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing time capsule title")
	}
	return nil
}

func validateChannelTimeCapsuleActionRequestOptions(opts ChannelTimeCapsuleOptions) error {
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
	if opts.CapsuleID == "" {
		return fmt.Errorf("missing time capsule id")
	}
	if opts.OpenAfter == "" {
		return fmt.Errorf("missing open after")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing time capsule source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing time capsule title")
	}
	return nil
}

func findOrCreateChannelTimeCapsuleIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelTimeCapsuleOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel time capsule issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelTimeCapsuleMatches(issue.Body, opts.CapsuleID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelTimeCapsuleIssueTitle(opts), RenderChannelTimeCapsuleIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel time capsule issue: %w", err)
	}
	return issue, true, false, nil
}

func channelTimeCapsuleIssueTitle(opts ChannelTimeCapsuleOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.CapsuleID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel time capsule: " + title
}

func channelTimeCapsuleMatches(body, capsuleID string) bool {
	return HasChannelTimeCapsuleMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`capsule_id="%s"`, escapeMarkerValue(cleanChannelTimeCapsuleID(capsuleID))))
}

func cleanChannelTimeCapsuleID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelTimeCapsuleID(ev Event, channel, threadID, sourceMessageID, openAfter, title, message string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, openAfter, title, message}, "|")
	return fmt.Sprintf("time-capsule-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelTimeCapsuleNotifyMessageID(ev Event, capsuleID string) string {
	seed := strings.Join([]string{eventID(ev), capsuleID}, "|")
	return fmt.Sprintf("gitclaw-channel-time-capsule-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelTimeCapsuleNotificationBody(opts ChannelTimeCapsuleOptions, capsuleIssueNumber int, capsuleIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel time capsule recorded.\n\n")
	if capsuleIssueNumber > 0 {
		fmt.Fprintf(&b, "Time capsule: #%d\n", capsuleIssueNumber)
	}
	if capsuleIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", capsuleIssueURL)
	}
	fmt.Fprintf(&b, "Open after: %s\n", strings.TrimSpace(opts.OpenAfter))
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
