package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBookmarkOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	BookmarkID        string
	ReferenceURL      string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBookmarkResult struct {
	BookmarkIssueNumber int
	BookmarkIssueURL    string
	BookmarkCreated     bool
	BookmarkDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelBookmarkActionRequest struct {
	Options             ChannelBookmarkOptions
	Command             string
	Subcommand          string
	AutoBookmarkID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ReferenceURLSHA     string
	ReferenceURLBytes   int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelBookmarkActionRequest(ev Event, cfg Config) bool {
	return isChannelBookmarkActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBookmarkActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "bookmark-message", "save-message", "preserve", "keep", "flag-message":
		return true
	default:
		return false
	}
}

func BuildChannelBookmarkActionRequest(ev Event, cfg Config) (ChannelBookmarkActionRequest, error) {
	fields, trailing, ok := channelBookmarkActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBookmarkActionRequest{}, fmt.Errorf("missing channel bookmark command")
	}
	req := ChannelBookmarkActionRequest{
		Options: ChannelBookmarkOptions{
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
				return ChannelBookmarkActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--bookmark-id", "--save-id", "--capture-id", "--archive-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.BookmarkID = cleanChannelBookmarkID(fields[i+1])
			i++
		case "--url", "--reference-url", "--source-url", "--href":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ReferenceURL = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBookmarkActionRequest{}, fmt.Errorf("unknown channel bookmark argument %q", field)
			}
			if req.Options.BookmarkID == "" {
				req.Options.BookmarkID = cleanChannelBookmarkID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBookmarkActionRequest{}, fmt.Errorf("unexpected channel bookmark argument %q", field)
		}
	}
	if err := applyChannelBookmarkIssueTarget(ev, &req); err != nil {
		return ChannelBookmarkActionRequest{}, err
	}
	title, notes := parseChannelBookmarkTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.BookmarkID) == "" {
		req.Options.BookmarkID = autoChannelBookmarkID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.ReferenceURL, title, notes)
		req.AutoBookmarkID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBookmarkNotifyMessageID(ev, req.Options.BookmarkID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBookmarkOptions(req.Options)
	if err := validateChannelBookmarkActionRequestOptions(req.Options); err != nil {
		return ChannelBookmarkActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ReferenceURLSHA = optionalChannelBookmarkHash(req.Options.ReferenceURL)
	req.ReferenceURLBytes = len(req.Options.ReferenceURL)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelBookmarkNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelBookmark(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBookmarkOptions) (ChannelBookmarkResult, error) {
	opts = normalizeChannelBookmarkOptions(opts)
	var err error
	opts, err = applyChannelBookmarkRoute(cfg, opts)
	if err != nil {
		return ChannelBookmarkResult{}, err
	}
	if err := validateChannelBookmarkOptions(opts); err != nil {
		return ChannelBookmarkResult{}, err
	}
	bookmarkIssue, created, duplicate, err := findOrCreateChannelBookmarkIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelBookmarkResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelBookmarkNotificationBody(opts, bookmarkIssue.Number, issueURL(opts.Repo, bookmarkIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelBookmarkResult{}, fmt.Errorf("queue channel bookmark notification: %w", err)
	}
	return ChannelBookmarkResult{
		BookmarkIssueNumber: bookmarkIssue.Number,
		BookmarkIssueURL:    issueURL(opts.Repo, bookmarkIssue.Number),
		BookmarkCreated:     created,
		BookmarkDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelBookmarkActionReport(ev Event, req ChannelBookmarkActionRequest, result ChannelBookmarkResult) string {
	status := "saved"
	switch {
	case result.BookmarkDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.BookmarkDuplicate:
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
	b.WriteString("## GitClaw Channel Bookmark Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_bookmark_status: `%s`\n", status)
	fmt.Fprintf(&b, "- bookmark_issue: `#%d`\n", result.BookmarkIssueNumber)
	fmt.Fprintf(&b, "- bookmark_issue_url: `%s`\n", result.BookmarkIssueURL)
	fmt.Fprintf(&b, "- bookmark_issue_created: `%t`\n", result.BookmarkCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.BookmarkDuplicate)
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
	fmt.Fprintf(&b, "- bookmark_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.BookmarkID))
	fmt.Fprintf(&b, "- bookmark_id_auto: `%t`\n", req.AutoBookmarkID)
	fmt.Fprintf(&b, "- bookmark_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- bookmark_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- bookmark_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- reference_url_sha256_12: `%s`\n", req.ReferenceURLSHA)
	fmt.Fprintf(&b, "- reference_url_bytes: `%d`\n", req.ReferenceURLBytes)
	fmt.Fprintf(&b, "- reference_url_present: `%t`\n", req.Options.ReferenceURL != "")
	fmt.Fprintf(&b, "- bookmark_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- bookmark_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- bookmark_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_bookmark_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bookmark_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reference_url_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bookmark_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_bookmark_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw saved a channel-origin message pointer as a durable GitHub bookmark issue, then queued a provider-facing acknowledgement back to the original thread. The bookmark issue contains the human-readable saved context and optional reference URL hash; this source receipt keeps provider IDs, bookmark IDs, raw reference URLs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the bookmark acknowledgement with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent bookmark acknowledgements with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate bookmark issues are suppressed by `bookmark_id`; duplicate bookmark notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the bookmark issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelBookmarkIssueBody(opts ChannelBookmarkOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-bookmark bookmark_id=\"%s\" channel=\"%s\" reference_url_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.BookmarkID), escapeMarkerValue(opts.Channel), optionalChannelBookmarkHash(opts.ReferenceURL), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel bookmark.\n\n")
	fmt.Fprintf(&b, "- bookmark_id: %s\n", opts.BookmarkID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- reference_url_sha256_12: %s\n", optionalChannelBookmarkHash(opts.ReferenceURL))
	fmt.Fprintf(&b, "- reference_url_present: %t\n", opts.ReferenceURL != "")
	fmt.Fprintf(&b, "- bookmark_mode: github-issue-message-bookmark\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_reference_url_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Bookmark\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel-origin bookmark. The referenced channel message body was not copied into the receipt or provider acknowledgement.")
	return strings.TrimSpace(b.String())
}

func channelBookmarkActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBookmarkActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBookmarkIssueTarget(ev Event, req *ChannelBookmarkActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel bookmark requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelBookmarkTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel bookmark from issue #%d", ev.Issue.Number)
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
	case strings.HasPrefix(lowerFirst, "bookmark:"):
		title = strings.TrimSpace(first[len("bookmark:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "reason:"):
		title = defaultTitle
		noteLines = cleaned
	case strings.HasPrefix(lowerFirst, "summary:"):
		title = strings.TrimSpace(first[len("summary:"):])
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
	case strings.HasPrefix(notesLower, "reason:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("reason:"):])
	}
	return title, notes
}

func normalizeChannelBookmarkOptions(opts ChannelBookmarkOptions) ChannelBookmarkOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.BookmarkID = cleanChannelBookmarkID(opts.BookmarkID)
	opts.ReferenceURL = strings.TrimSpace(opts.ReferenceURL)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBookmarkRoute(cfg Config, opts ChannelBookmarkOptions) (ChannelBookmarkOptions, error) {
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

func validateChannelBookmarkOptions(opts ChannelBookmarkOptions) error {
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
	if opts.BookmarkID == "" {
		return fmt.Errorf("missing bookmark id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing bookmark source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing bookmark title")
	}
	return nil
}

func validateChannelBookmarkActionRequestOptions(opts ChannelBookmarkOptions) error {
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
	if opts.BookmarkID == "" {
		return fmt.Errorf("missing bookmark id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing bookmark source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing bookmark title")
	}
	return nil
}

func findOrCreateChannelBookmarkIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBookmarkOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel bookmark issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelBookmarkMatches(issue.Body, opts.BookmarkID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelBookmarkIssueTitle(opts), RenderChannelBookmarkIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel bookmark issue: %w", err)
	}
	return issue, true, false, nil
}

func channelBookmarkIssueTitle(opts ChannelBookmarkOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.BookmarkID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel bookmark: " + title
}

func channelBookmarkMatches(body, bookmarkID string) bool {
	return HasChannelBookmarkMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`bookmark_id="%s"`, escapeMarkerValue(cleanChannelBookmarkID(bookmarkID))))
}

func cleanChannelBookmarkID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBookmarkID(ev Event, channel, threadID, sourceMessageID, referenceURL, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, referenceURL, title, notes}, "|")
	return fmt.Sprintf("bookmark-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBookmarkNotifyMessageID(ev Event, bookmarkID string) string {
	seed := strings.Join([]string{eventID(ev), bookmarkID}, "|")
	return fmt.Sprintf("gitclaw-channel-bookmark-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelBookmarkNotificationBody(opts ChannelBookmarkOptions, bookmarkIssueNumber int, bookmarkIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel bookmark saved.\n\n")
	if bookmarkIssueNumber > 0 {
		fmt.Fprintf(&b, "Bookmark: #%d\n", bookmarkIssueNumber)
	}
	if bookmarkIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", bookmarkIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nSaved in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}

func optionalChannelBookmarkHash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return shortDocumentHash(value)
}
