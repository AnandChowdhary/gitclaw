package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ChannelReminderOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ReminderID        string
	DueAt             string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelReminderResult struct {
	ReminderIssueNumber int
	ReminderIssueURL    string
	ReminderCreated     bool
	ReminderDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelReminderActionRequest struct {
	Options             ChannelReminderOptions
	Command             string
	Subcommand          string
	AutoReminderID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	DueAtSHA            string
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelReminderActionRequest(ev Event, cfg Config) bool {
	return isChannelReminderActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelReminderActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "remind", "reminder", "remind-me", "snooze", "follow-up", "followup":
		return true
	default:
		return false
	}
}

func BuildChannelReminderActionRequest(ev Event, cfg Config) (ChannelReminderActionRequest, error) {
	fields, trailing, ok := channelReminderActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelReminderActionRequest{}, fmt.Errorf("missing channel reminder command")
	}
	req := ChannelReminderActionRequest{
		Options: ChannelReminderOptions{
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
				return ChannelReminderActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--reminder-id", "--remind-id", "--snooze-id", "--id":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ReminderID = cleanChannelReminderID(fields[i+1])
			i++
		case "--at", "--due", "--not-before", "--when":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DueAt = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelReminderActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelReminderActionRequest{}, fmt.Errorf("unknown channel reminder argument %q", field)
			}
			if req.Options.ReminderID == "" {
				req.Options.ReminderID = cleanChannelReminderID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelReminderActionRequest{}, fmt.Errorf("unexpected channel reminder argument %q", field)
		}
	}
	if err := applyChannelReminderIssueTarget(ev, &req); err != nil {
		return ChannelReminderActionRequest{}, err
	}
	title, notes := parseChannelReminderTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.ReminderID) == "" {
		req.Options.ReminderID = autoChannelReminderID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.DueAt, title, notes)
		req.AutoReminderID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelReminderNotifyMessageID(ev, req.Options.ReminderID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelReminderOptions(req.Options)
	if err := validateChannelReminderActionRequestOptions(req.Options); err != nil {
		return ChannelReminderActionRequest{}, err
	}
	req.DueAtSHA = shortDocumentHash(req.Options.DueAt)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelReminderNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelReminder(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelReminderOptions) (ChannelReminderResult, error) {
	opts = normalizeChannelReminderOptions(opts)
	var err error
	opts, err = applyChannelReminderRoute(cfg, opts)
	if err != nil {
		return ChannelReminderResult{}, err
	}
	if err := validateChannelReminderOptions(opts); err != nil {
		return ChannelReminderResult{}, err
	}
	reminderIssue, created, duplicate, err := findOrCreateChannelReminderIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelReminderResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelReminderNotificationBody(opts, reminderIssue.Number, issueURL(opts.Repo, reminderIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelReminderResult{}, fmt.Errorf("queue channel reminder notification: %w", err)
	}
	return ChannelReminderResult{
		ReminderIssueNumber: reminderIssue.Number,
		ReminderIssueURL:    issueURL(opts.Repo, reminderIssue.Number),
		ReminderCreated:     created,
		ReminderDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelReminderActionReport(ev Event, req ChannelReminderActionRequest, result ChannelReminderResult) string {
	status := "created"
	switch {
	case result.ReminderDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ReminderDuplicate:
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
	b.WriteString("## GitClaw Channel Reminder Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_reminder_status: `%s`\n", status)
	fmt.Fprintf(&b, "- reminder_issue: `#%d`\n", result.ReminderIssueNumber)
	fmt.Fprintf(&b, "- reminder_issue_url: `%s`\n", result.ReminderIssueURL)
	fmt.Fprintf(&b, "- reminder_issue_created: `%t`\n", result.ReminderCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ReminderDuplicate)
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
	fmt.Fprintf(&b, "- reminder_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ReminderID))
	fmt.Fprintf(&b, "- reminder_id_auto: `%t`\n", req.AutoReminderID)
	fmt.Fprintf(&b, "- reminder_due_at_sha256_12: `%s`\n", req.DueAtSHA)
	fmt.Fprintf(&b, "- reminder_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- reminder_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- reminder_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- reminder_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- reminder_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- reminder_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_reminder_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reminder_due_at_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reminder_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reminder_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_reminder_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub reminder issue from a mirrored channel thread, then queued a provider-facing reminder link back to that thread. The reminder issue contains the due time and human-readable reminder; this source receipt keeps provider IDs, reminder IDs, due times, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the reminder-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent reminder links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate reminder issues are suppressed by `reminder_id`; duplicate reminder-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the reminder issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelReminderIssueBody(opts ChannelReminderOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-reminder reminder_id=\"%s\" channel=\"%s\" due_at=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ReminderID), escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.DueAt), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel reminder.\n\n")
	fmt.Fprintf(&b, "- reminder_id: %s\n", opts.ReminderID)
	fmt.Fprintf(&b, "- not_before: %s\n", opts.DueAt)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- reminder_mode: github-issue-reminder\n")
	fmt.Fprintf(&b, "- wake_strategy: scheduled-github-actions-proactive-check\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Reminder\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nContinue the reminder in this GitHub issue. Scheduled GitHub Actions can treat the `not_before` field as the due gate and then continue the conversation here.")
	return strings.TrimSpace(b.String())
}

func channelReminderActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelReminderActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelReminderIssueTarget(ev Event, req *ChannelReminderActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel reminder requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelReminderTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel reminder from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var title string
	var noteLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "title:"):
		title = strings.TrimSpace(first[len("title:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"):
		title = defaultTitle
		noteLines = cleaned
	default:
		title = first
		noteLines = cleaned[1:]
	}
	if title == "" {
		title = defaultTitle
	}
	notes := strings.TrimSpace(strings.Join(noteLines, "\n"))
	notesLower := strings.ToLower(strings.TrimSpace(notes))
	switch {
	case strings.HasPrefix(notesLower, "notes:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("notes:"):])
	case strings.HasPrefix(notesLower, "context:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("context:"):])
	}
	return title, notes
}

func normalizeChannelReminderOptions(opts ChannelReminderOptions) ChannelReminderOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ReminderID = cleanChannelReminderID(opts.ReminderID)
	opts.DueAt = normalizeChannelReminderDueAt(opts.DueAt)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelReminderRoute(cfg Config, opts ChannelReminderOptions) (ChannelReminderOptions, error) {
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

func validateChannelReminderOptions(opts ChannelReminderOptions) error {
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
	if opts.ReminderID == "" {
		return fmt.Errorf("missing reminder id")
	}
	if opts.DueAt == "" {
		return fmt.Errorf("missing reminder due time")
	}
	if _, err := parseProactiveNotBefore(opts.DueAt); err != nil {
		return err
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing reminder source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing reminder title")
	}
	return nil
}

func validateChannelReminderActionRequestOptions(opts ChannelReminderOptions) error {
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
	if opts.ReminderID == "" {
		return fmt.Errorf("missing reminder id")
	}
	if opts.DueAt == "" {
		return fmt.Errorf("missing reminder due time")
	}
	if _, err := parseProactiveNotBefore(opts.DueAt); err != nil {
		return err
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing reminder source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing reminder title")
	}
	return nil
}

func findOrCreateChannelReminderIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelReminderOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel reminder issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelReminderMatches(issue.Body, opts.ReminderID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelReminderIssueTitle(opts), RenderChannelReminderIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel reminder issue: %w", err)
	}
	return issue, true, false, nil
}

func channelReminderIssueTitle(opts ChannelReminderOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.ReminderID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel reminder: " + title
}

func channelReminderMatches(body, reminderID string) bool {
	return HasChannelReminderMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`reminder_id="%s"`, escapeMarkerValue(cleanChannelReminderID(reminderID))))
}

func cleanChannelReminderID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelReminderID(ev Event, channel, threadID, sourceMessageID, dueAt, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, dueAt, title, notes}, "|")
	return fmt.Sprintf("reminder-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func normalizeChannelReminderDueAt(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := parseProactiveNotBefore(value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(time.RFC3339)
}

func autoChannelReminderNotifyMessageID(ev Event, reminderID string) string {
	seed := strings.Join([]string{eventID(ev), reminderID}, "|")
	return fmt.Sprintf("gitclaw-channel-reminder-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelReminderNotificationBody(opts ChannelReminderOptions, reminderIssueNumber int, reminderIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel reminder created.\n\n")
	if reminderIssueNumber > 0 {
		fmt.Fprintf(&b, "Reminder: #%d\n", reminderIssueNumber)
	}
	if reminderIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", reminderIssueURL)
	}
	fmt.Fprintf(&b, "Due: %s\n", strings.TrimSpace(opts.DueAt))
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nThe reminder is tracked in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
