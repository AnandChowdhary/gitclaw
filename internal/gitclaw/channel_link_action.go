package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelLinkOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	LinkID            string
	LinkURL           string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelLinkResult struct {
	LinkIssueNumber int
	LinkIssueURL    string
	LinkCreated     bool
	LinkDuplicate   bool
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
}

type ChannelLinkActionRequest struct {
	Options             ChannelLinkOptions
	Command             string
	Subcommand          string
	AutoLinkID          bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	LinkURLSHA          string
	LinkURLBytes        int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelLinkActionRequest(ev Event, cfg Config) bool {
	return isChannelLinkActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelLinkActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "link", "url", "link-card", "reference", "ref":
		return true
	default:
		return false
	}
}

func BuildChannelLinkActionRequest(ev Event, cfg Config) (ChannelLinkActionRequest, error) {
	fields, trailing, ok := channelLinkActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelLinkActionRequest{}, fmt.Errorf("missing channel link command")
	}
	req := ChannelLinkActionRequest{
		Options: ChannelLinkOptions{
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
				return ChannelLinkActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--link-id", "--save-id", "--capture-id", "--archive-id", "--id":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.LinkID = cleanChannelLinkID(fields[i+1])
			i++
		case "--url", "--link-url", "--source-url", "--href":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.LinkURL = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelLinkActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelLinkActionRequest{}, fmt.Errorf("unknown channel link argument %q", field)
			}
			if req.Options.LinkID == "" {
				req.Options.LinkID = cleanChannelLinkID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelLinkActionRequest{}, fmt.Errorf("unexpected channel link argument %q", field)
		}
	}
	if err := applyChannelLinkIssueTarget(ev, &req); err != nil {
		return ChannelLinkActionRequest{}, err
	}
	title, notes := parseChannelLinkTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.LinkID) == "" {
		req.Options.LinkID = autoChannelLinkID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.LinkURL, title, notes)
		req.AutoLinkID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelLinkNotifyMessageID(ev, req.Options.LinkID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelLinkOptions(req.Options)
	if err := validateChannelLinkActionRequestOptions(req.Options); err != nil {
		return ChannelLinkActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.LinkURLSHA = shortDocumentHash(req.Options.LinkURL)
	req.LinkURLBytes = len(req.Options.LinkURL)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelLinkNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelLink(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelLinkOptions) (ChannelLinkResult, error) {
	opts = normalizeChannelLinkOptions(opts)
	var err error
	opts, err = applyChannelLinkRoute(cfg, opts)
	if err != nil {
		return ChannelLinkResult{}, err
	}
	if err := validateChannelLinkOptions(opts); err != nil {
		return ChannelLinkResult{}, err
	}
	linkIssue, created, duplicate, err := findOrCreateChannelLinkIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelLinkResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelLinkNotificationBody(opts, linkIssue.Number, issueURL(opts.Repo, linkIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelLinkResult{}, fmt.Errorf("queue channel link notification: %w", err)
	}
	return ChannelLinkResult{
		LinkIssueNumber: linkIssue.Number,
		LinkIssueURL:    issueURL(opts.Repo, linkIssue.Number),
		LinkCreated:     created,
		LinkDuplicate:   duplicate,
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelLinkActionReport(ev Event, req ChannelLinkActionRequest, result ChannelLinkResult) string {
	status := "saved"
	switch {
	case result.LinkDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.LinkDuplicate:
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
	b.WriteString("## GitClaw Channel Link Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_link_status: `%s`\n", status)
	fmt.Fprintf(&b, "- link_issue: `#%d`\n", result.LinkIssueNumber)
	fmt.Fprintf(&b, "- link_issue_url: `%s`\n", result.LinkIssueURL)
	fmt.Fprintf(&b, "- link_issue_created: `%t`\n", result.LinkCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.LinkDuplicate)
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
	fmt.Fprintf(&b, "- link_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.LinkID))
	fmt.Fprintf(&b, "- link_id_auto: `%t`\n", req.AutoLinkID)
	fmt.Fprintf(&b, "- link_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- link_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- link_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- link_url_sha256_12: `%s`\n", req.LinkURLSHA)
	fmt.Fprintf(&b, "- link_url_bytes: `%d`\n", req.LinkURLBytes)
	fmt.Fprintf(&b, "- link_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- link_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- link_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_link_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_link_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_link_url_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_link_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_link_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw saved channel-origin link metadata as a durable GitHub link-card issue, then queued a provider-facing issue link back to the original thread. The link-card issue contains the human-readable saved context and a URL hash; this source receipt keeps provider IDs, link IDs, raw URLs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the link-card notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent link-card links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate link-card issues are suppressed by `link_id`; duplicate link-card notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the link issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelLinkIssueBody(opts ChannelLinkOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-link link_id=\"%s\" channel=\"%s\" link_url_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.LinkID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.LinkURL), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel link card.\n\n")
	fmt.Fprintf(&b, "- link_id: %s\n", opts.LinkID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- link_url_sha256_12: %s\n", shortDocumentHash(opts.LinkURL))
	fmt.Fprintf(&b, "- link_mode: github-issue-link-card\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_link_url_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Link Card\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel-origin link. The URL was not fetched, expanded, or copied by this action.")
	return strings.TrimSpace(b.String())
}

func channelLinkActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelLinkActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelLinkIssueTarget(ev Event, req *ChannelLinkActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel link requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelLinkTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel link from issue #%d", ev.Issue.Number)
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
	case strings.HasPrefix(lowerFirst, "link:"):
		title = strings.TrimSpace(first[len("link:"):])
		noteLines = cleaned[1:]
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
	}
	return title, notes
}

func normalizeChannelLinkOptions(opts ChannelLinkOptions) ChannelLinkOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.LinkID = cleanChannelLinkID(opts.LinkID)
	opts.LinkURL = strings.TrimSpace(opts.LinkURL)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelLinkRoute(cfg Config, opts ChannelLinkOptions) (ChannelLinkOptions, error) {
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

func validateChannelLinkOptions(opts ChannelLinkOptions) error {
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
	if opts.LinkID == "" {
		return fmt.Errorf("missing link id")
	}
	if opts.LinkURL == "" {
		return fmt.Errorf("missing link url")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing link source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing link title")
	}
	return nil
}

func validateChannelLinkActionRequestOptions(opts ChannelLinkOptions) error {
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
	if opts.LinkID == "" {
		return fmt.Errorf("missing link id")
	}
	if opts.LinkURL == "" {
		return fmt.Errorf("missing link url")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing link source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing link title")
	}
	return nil
}

func findOrCreateChannelLinkIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelLinkOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel link issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelLinkMatches(issue.Body, opts.LinkID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelLinkIssueTitle(opts), RenderChannelLinkIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel link issue: %w", err)
	}
	return issue, true, false, nil
}

func channelLinkIssueTitle(opts ChannelLinkOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.LinkID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel link: " + title
}

func channelLinkMatches(body, linkID string) bool {
	return HasChannelLinkMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`link_id="%s"`, escapeMarkerValue(cleanChannelLinkID(linkID))))
}

func cleanChannelLinkID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelLinkID(ev Event, channel, threadID, sourceMessageID, linkURL, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, linkURL, title, notes}, "|")
	return fmt.Sprintf("link-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelLinkNotifyMessageID(ev Event, linkID string) string {
	seed := strings.Join([]string{eventID(ev), linkID}, "|")
	return fmt.Sprintf("gitclaw-channel-link-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelLinkNotificationBody(opts ChannelLinkOptions, linkIssueNumber int, linkIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel link card saved.\n\n")
	if linkIssueNumber > 0 {
		fmt.Fprintf(&b, "Link card: #%d\n", linkIssueNumber)
	}
	if linkIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", linkIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nSaved in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
