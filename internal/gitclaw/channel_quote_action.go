package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelQuoteOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	QuoteID           string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelQuoteResult struct {
	QuoteIssueNumber int
	QuoteIssueURL    string
	QuoteCreated     bool
	QuoteDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelQuoteActionRequest struct {
	Options             ChannelQuoteOptions
	Command             string
	Subcommand          string
	AutoQuoteID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
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

func IsChannelQuoteActionRequest(ev Event, cfg Config) bool {
	return isChannelQuoteActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelQuoteActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "quote", "save-quote", "quotebook", "pullquote", "quote-card", "capture-quote":
		return true
	default:
		return false
	}
}

func BuildChannelQuoteActionRequest(ev Event, cfg Config) (ChannelQuoteActionRequest, error) {
	fields, trailing, ok := channelQuoteActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelQuoteActionRequest{}, fmt.Errorf("missing channel quote command")
	}
	req := ChannelQuoteActionRequest{
		Options: ChannelQuoteOptions{
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
				return ChannelQuoteActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelQuoteActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelQuoteActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelQuoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelQuoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--quote-id", "--saved-quote-id", "--pullquote-id", "--id":
			if i+1 >= len(fields) {
				return ChannelQuoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.QuoteID = cleanChannelQuoteID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelQuoteActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelQuoteActionRequest{}, fmt.Errorf("unknown channel quote argument %q", field)
			}
			if req.Options.QuoteID == "" {
				req.Options.QuoteID = cleanChannelQuoteID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelQuoteActionRequest{}, fmt.Errorf("unexpected channel quote argument %q", field)
		}
	}
	if err := applyChannelQuoteIssueTarget(ev, &req); err != nil {
		return ChannelQuoteActionRequest{}, err
	}
	title, notes := parseChannelQuoteTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.QuoteID) == "" {
		req.Options.QuoteID = autoChannelQuoteID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoQuoteID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelQuoteNotifyMessageID(ev, req.Options.QuoteID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelQuoteOptions(req.Options)
	if err := validateChannelQuoteActionRequestOptions(req.Options); err != nil {
		return ChannelQuoteActionRequest{}, err
	}
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelQuoteNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelQuote(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelQuoteOptions) (ChannelQuoteResult, error) {
	opts = normalizeChannelQuoteOptions(opts)
	var err error
	opts, err = applyChannelQuoteRoute(cfg, opts)
	if err != nil {
		return ChannelQuoteResult{}, err
	}
	if err := validateChannelQuoteOptions(opts); err != nil {
		return ChannelQuoteResult{}, err
	}
	quoteIssue, created, duplicate, err := findOrCreateChannelQuoteIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelQuoteResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelQuoteNotificationBody(opts, quoteIssue.Number, issueURL(opts.Repo, quoteIssue.Number)),
	})
	if err != nil {
		return ChannelQuoteResult{}, fmt.Errorf("queue channel quote notification: %w", err)
	}
	return ChannelQuoteResult{
		QuoteIssueNumber: quoteIssue.Number,
		QuoteIssueURL:    issueURL(opts.Repo, quoteIssue.Number),
		QuoteCreated:     created,
		QuoteDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelQuoteActionReport(ev Event, req ChannelQuoteActionRequest, result ChannelQuoteResult) string {
	status := "captured"
	switch {
	case result.QuoteDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.QuoteDuplicate:
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
	b.WriteString("## GitClaw Channel Quote Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_quote_status: `%s`\n", status)
	fmt.Fprintf(&b, "- quote_issue: `#%d`\n", result.QuoteIssueNumber)
	fmt.Fprintf(&b, "- quote_issue_url: `%s`\n", result.QuoteIssueURL)
	fmt.Fprintf(&b, "- quote_issue_created: `%t`\n", result.QuoteCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.QuoteDuplicate)
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
	fmt.Fprintf(&b, "- quote_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.QuoteID))
	fmt.Fprintf(&b, "- quote_id_auto: `%t`\n", req.AutoQuoteID)
	fmt.Fprintf(&b, "- quote_text_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- quote_text_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- quote_text_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- quote_context_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- quote_context_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- quote_context_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_quote_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quote_text_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quote_context_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_quote_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin quote as a durable GitHub issue, then queued a provider-facing quote link back to the mirrored thread. The quote issue contains the human-readable quote text and context; this source receipt keeps provider IDs, quote IDs, quote text, context, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the quote-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent quote links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate quote issues are suppressed by `quote_id`; duplicate quote-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the quote issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelQuoteIssueBody(opts ChannelQuoteOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-quote quote_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.QuoteID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel quote.\n\n")
	fmt.Fprintf(&b, "- quote_id: %s\n", opts.QuoteID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- quote_mode: github-issue-quote\n")
	fmt.Fprintf(&b, "- memory_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Quote\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Context\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the channel quote without mutating memory or turning it into a task automatically.")
	return strings.TrimSpace(b.String())
}

func channelQuoteActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelQuoteActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelQuoteIssueTarget(ev Event, req *ChannelQuoteActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel quote requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelQuoteTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel quote from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var title string
	var noteLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "quote:"):
		title = strings.TrimSpace(first[len("quote:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "quote text:"):
		title = strings.TrimSpace(first[len("quote text:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "says:"):
		title = strings.TrimSpace(first[len("says:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "title:"):
		title = strings.TrimSpace(first[len("title:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"), strings.HasPrefix(lowerFirst, "source:"), strings.HasPrefix(lowerFirst, "why:"):
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
	case strings.HasPrefix(notesLower, "source:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("source:"):])
	case strings.HasPrefix(notesLower, "why:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("why:"):])
	}
	return title, notes
}

func normalizeChannelQuoteOptions(opts ChannelQuoteOptions) ChannelQuoteOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.QuoteID = cleanChannelQuoteID(opts.QuoteID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelQuoteRoute(cfg Config, opts ChannelQuoteOptions) (ChannelQuoteOptions, error) {
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

func validateChannelQuoteOptions(opts ChannelQuoteOptions) error {
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
	if opts.QuoteID == "" {
		return fmt.Errorf("missing quote id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing quote source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing quote text")
	}
	return nil
}

func validateChannelQuoteActionRequestOptions(opts ChannelQuoteOptions) error {
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
	if opts.QuoteID == "" {
		return fmt.Errorf("missing quote id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing quote source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing quote text")
	}
	return nil
}

func findOrCreateChannelQuoteIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelQuoteOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel quote issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelQuoteMatches(issue.Body, opts.QuoteID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelQuoteIssueTitle(opts), RenderChannelQuoteIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel quote issue: %w", err)
	}
	return issue, true, false, nil
}

func channelQuoteIssueTitle(opts ChannelQuoteOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.QuoteID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel quote: " + title
}

func channelQuoteMatches(body, quoteID string) bool {
	return HasChannelQuoteMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`quote_id="%s"`, escapeMarkerValue(cleanChannelQuoteID(quoteID))))
}

func cleanChannelQuoteID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelQuoteID(ev Event, channel, threadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("quote-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelQuoteNotifyMessageID(ev Event, quoteID string) string {
	seed := strings.Join([]string{eventID(ev), quoteID}, "|")
	return fmt.Sprintf("gitclaw-channel-quote-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelQuoteNotificationBody(opts ChannelQuoteOptions, quoteIssueNumber int, quoteIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel quote captured.\n\n")
	if quoteIssueNumber > 0 {
		fmt.Fprintf(&b, "Quote: #%d\n", quoteIssueNumber)
	}
	if quoteIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", quoteIssueURL)
	}
	fmt.Fprintf(&b, "Quote: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue shaping it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
