package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelWatchOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	WatchID           string
	Cadence           string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelWatchResult struct {
	WatchIssueNumber int
	WatchIssueURL    string
	WatchCreated     bool
	WatchDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelWatchActionRequest struct {
	Options             ChannelWatchOptions
	Command             string
	Subcommand          string
	AutoWatchID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	CadenceSHA          string
	CadenceBytes        int
	CadenceLines        int
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

func IsChannelWatchActionRequest(ev Event, cfg Config) bool {
	return isChannelWatchActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelWatchActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "watch", "monitor", "track", "observe", "watch-topic", "keep-watch", "keepalive":
		return true
	default:
		return false
	}
}

func BuildChannelWatchActionRequest(ev Event, cfg Config) (ChannelWatchActionRequest, error) {
	fields, trailing, ok := channelWatchActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelWatchActionRequest{}, fmt.Errorf("missing channel watch command")
	}
	req := ChannelWatchActionRequest{
		Options: ChannelWatchOptions{
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
				return ChannelWatchActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--watch-id", "--monitor-id", "--tracking-id", "--id":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.WatchID = cleanChannelWatchID(fields[i+1])
			i++
		case "--cadence", "--frequency", "--interval", "--every":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Cadence = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelWatchActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelWatchActionRequest{}, fmt.Errorf("unknown channel watch argument %q", field)
			}
			if req.Options.WatchID == "" {
				req.Options.WatchID = cleanChannelWatchID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelWatchActionRequest{}, fmt.Errorf("unexpected channel watch argument %q", field)
		}
	}
	if err := applyChannelWatchIssueTarget(ev, &req); err != nil {
		return ChannelWatchActionRequest{}, err
	}
	title, notes := parseChannelWatchTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.WatchID) == "" {
		req.Options.WatchID = autoChannelWatchID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoWatchID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelWatchNotifyMessageID(ev, req.Options.WatchID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelWatchOptions(req.Options)
	if err := validateChannelWatchActionRequestOptions(req.Options); err != nil {
		return ChannelWatchActionRequest{}, err
	}
	req.CadenceSHA = shortDocumentHash(req.Options.Cadence)
	req.CadenceBytes = len(req.Options.Cadence)
	req.CadenceLines = lineCount(req.Options.Cadence)
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelWatchNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelWatch(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelWatchOptions) (ChannelWatchResult, error) {
	opts = normalizeChannelWatchOptions(opts)
	var err error
	opts, err = applyChannelWatchRoute(cfg, opts)
	if err != nil {
		return ChannelWatchResult{}, err
	}
	if err := validateChannelWatchOptions(opts); err != nil {
		return ChannelWatchResult{}, err
	}
	watchIssue, created, duplicate, err := findOrCreateChannelWatchIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelWatchResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelWatchNotificationBody(opts, watchIssue.Number, issueURL(opts.Repo, watchIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelWatchResult{}, fmt.Errorf("queue channel watch notification: %w", err)
	}
	return ChannelWatchResult{
		WatchIssueNumber: watchIssue.Number,
		WatchIssueURL:    issueURL(opts.Repo, watchIssue.Number),
		WatchCreated:     created,
		WatchDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelWatchActionReport(ev Event, req ChannelWatchActionRequest, result ChannelWatchResult) string {
	status := "created"
	switch {
	case result.WatchDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.WatchDuplicate:
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
	b.WriteString("## GitClaw Channel Watch Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_watch_status: `%s`\n", status)
	fmt.Fprintf(&b, "- watch_issue: `#%d`\n", result.WatchIssueNumber)
	fmt.Fprintf(&b, "- watch_issue_url: `%s`\n", result.WatchIssueURL)
	fmt.Fprintf(&b, "- watch_issue_created: `%t`\n", result.WatchCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.WatchDuplicate)
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
	fmt.Fprintf(&b, "- watch_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.WatchID))
	fmt.Fprintf(&b, "- watch_id_auto: `%t`\n", req.AutoWatchID)
	fmt.Fprintf(&b, "- watch_cadence_sha256_12: `%s`\n", req.CadenceSHA)
	fmt.Fprintf(&b, "- watch_cadence_bytes: `%d`\n", req.CadenceBytes)
	fmt.Fprintf(&b, "- watch_cadence_lines: `%d`\n", req.CadenceLines)
	fmt.Fprintf(&b, "- watch_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- watch_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- watch_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- watch_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- watch_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- watch_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- proactive_watch_issue: `%t`\n", true)
	fmt.Fprintf(&b, "- watch_scheduler: `%s`\n", "github-actions-scheduled-workflow")
	fmt.Fprintf(&b, "- raw_watch_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_watch_cadence_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_watch_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_watch_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_watch_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub watch issue from a mirrored channel thread, then queued a provider-facing watch link back to that thread. The watch issue is a durable control record for proactive follow-up; scheduled GitHub Actions workflows or normal issue comments can continue the watch without a socket server. This source receipt keeps provider IDs, watch IDs, cadence, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the watch-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent watch links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate watch issues are suppressed by `watch_id`; duplicate watch-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the watch issue with the `gitclaw` label\n")
	b.WriteString("- future scheduled workflows can search `gitclaw:channel-watch` issues and post model-backed check-ins when cadence is due\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelWatchIssueBody(opts ChannelWatchOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-watch watch_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.WatchID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel watch.\n\n")
	fmt.Fprintf(&b, "- watch_id: %s\n", opts.WatchID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- cadence: %s\n", opts.Cadence)
	fmt.Fprintf(&b, "- watch_mode: github-issue-watch\n")
	fmt.Fprintf(&b, "- proactive_schedule_ready: true\n")
	fmt.Fprintf(&b, "- scheduler: github-actions-scheduled-workflow\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Watch\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nContinue the watch in this GitHub issue. Use labels for state, comments for check-ins, and scheduled workflows for proactive scans.")
	return strings.TrimSpace(b.String())
}

func channelWatchActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelWatchActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelWatchIssueTarget(ev Event, req *ChannelWatchActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel watch requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelWatchTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel watch from issue #%d", ev.Issue.Number)
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
	case strings.HasPrefix(lowerFirst, "subject:"):
		title = strings.TrimSpace(first[len("subject:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "watch:"):
		title = strings.TrimSpace(first[len("watch:"):])
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

func normalizeChannelWatchOptions(opts ChannelWatchOptions) ChannelWatchOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.WatchID = cleanChannelWatchID(opts.WatchID)
	opts.Cadence = strings.TrimSpace(opts.Cadence)
	if opts.Cadence == "" {
		opts.Cadence = "manual-follow-up"
	}
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelWatchRoute(cfg Config, opts ChannelWatchOptions) (ChannelWatchOptions, error) {
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

func validateChannelWatchOptions(opts ChannelWatchOptions) error {
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
	if opts.WatchID == "" {
		return fmt.Errorf("missing watch id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing watch source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing watch title")
	}
	return nil
}

func validateChannelWatchActionRequestOptions(opts ChannelWatchOptions) error {
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
	if opts.WatchID == "" {
		return fmt.Errorf("missing watch id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing watch source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing watch title")
	}
	return nil
}

func findOrCreateChannelWatchIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelWatchOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel watch issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelWatchMatches(issue.Body, opts.WatchID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelWatchIssueTitle(opts), RenderChannelWatchIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel watch issue: %w", err)
	}
	return issue, true, false, nil
}

func channelWatchIssueTitle(opts ChannelWatchOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.WatchID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel watch: " + title
}

func channelWatchMatches(body, watchID string) bool {
	return HasChannelWatchMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`watch_id="%s"`, escapeMarkerValue(cleanChannelWatchID(watchID))))
}

func cleanChannelWatchID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelWatchID(ev Event, channel, threadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("watch-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelWatchNotifyMessageID(ev Event, watchID string) string {
	seed := strings.Join([]string{eventID(ev), watchID}, "|")
	return fmt.Sprintf("gitclaw-channel-watch-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelWatchNotificationBody(opts ChannelWatchOptions, watchIssueNumber int, watchIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel watch created.\n\n")
	if watchIssueNumber > 0 {
		fmt.Fprintf(&b, "Watch: #%d\n", watchIssueNumber)
	}
	if watchIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", watchIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	fmt.Fprintf(&b, "Cadence: %s\n", strings.TrimSpace(opts.Cadence))
	b.WriteString("\nContinue the watch in the linked GitHub issue. Scheduled GitHub Actions workflows can use that issue as the durable control record.")
	return strings.TrimSpace(b.String())
}
