package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelJournalOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	JournalID         string
	JournalDate       string
	Summary           string
	Highlights        string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelJournalResult struct {
	JournalIssueNumber int
	JournalIssueURL    string
	JournalCreated     bool
	JournalDuplicate   bool
	Notification       ChannelSendResult
	RouteName          string
	RouteHash          string
	Channel            string
	ThreadHash         string
	MessageHash        string
	NotifyHash         string
}

type ChannelJournalActionRequest struct {
	Options             ChannelJournalOptions
	Command             string
	Subcommand          string
	AutoJournalID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	DateSHA             string
	DateBytes           int
	DateLines           int
	SummarySHA          string
	SummaryBytes        int
	SummaryLines        int
	HighlightsSHA       string
	HighlightsBytes     int
	HighlightsLines     int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelJournalActionRequest(ev Event, cfg Config) bool {
	return isChannelJournalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelJournalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "journal", "log", "log-entry", "daily-log", "daily-note", "note", "field-note":
		return true
	default:
		return false
	}
}

func BuildChannelJournalActionRequest(ev Event, cfg Config) (ChannelJournalActionRequest, error) {
	fields, trailing, ok := channelJournalActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelJournalActionRequest{}, fmt.Errorf("missing channel journal command")
	}
	req := ChannelJournalActionRequest{
		Options: ChannelJournalOptions{
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
				return ChannelJournalActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--journal-id", "--log-id", "--entry-id", "--note-id", "--id":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.JournalID = cleanChannelJournalID(fields[i+1])
			i++
		case "--date", "--journal-date", "--day":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.JournalDate = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelJournalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelJournalActionRequest{}, fmt.Errorf("unknown channel journal argument %q", field)
			}
			if req.Options.JournalID == "" {
				req.Options.JournalID = cleanChannelJournalID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelJournalActionRequest{}, fmt.Errorf("unexpected channel journal argument %q", field)
		}
	}
	if err := applyChannelJournalIssueTarget(ev, &req); err != nil {
		return ChannelJournalActionRequest{}, err
	}
	journalDate, summary, highlights := parseChannelJournalSummaryHighlights(trailing, ev)
	if strings.TrimSpace(req.Options.JournalDate) == "" {
		req.Options.JournalDate = journalDate
	}
	req.Options.Summary = summary
	req.Options.Highlights = highlights
	if strings.TrimSpace(req.Options.JournalID) == "" {
		req.Options.JournalID = autoChannelJournalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.JournalDate, summary, highlights)
		req.AutoJournalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelJournalNotifyMessageID(ev, req.Options.JournalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelJournalOptions(req.Options)
	if err := validateChannelJournalActionRequestOptions(req.Options); err != nil {
		return ChannelJournalActionRequest{}, err
	}
	req.DateSHA = shortDocumentHash(req.Options.JournalDate)
	req.DateBytes = len(req.Options.JournalDate)
	req.DateLines = lineCount(req.Options.JournalDate)
	req.SummarySHA = shortDocumentHash(req.Options.Summary)
	req.SummaryBytes = len(req.Options.Summary)
	req.SummaryLines = lineCount(req.Options.Summary)
	req.HighlightsSHA = shortDocumentHash(req.Options.Highlights)
	req.HighlightsBytes = len(req.Options.Highlights)
	req.HighlightsLines = lineCount(req.Options.Highlights)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelJournalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelJournal(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelJournalOptions) (ChannelJournalResult, error) {
	opts = normalizeChannelJournalOptions(opts)
	var err error
	opts, err = applyChannelJournalRoute(cfg, opts)
	if err != nil {
		return ChannelJournalResult{}, err
	}
	if err := validateChannelJournalOptions(opts); err != nil {
		return ChannelJournalResult{}, err
	}
	journalIssue, created, duplicate, err := findOrCreateChannelJournalIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelJournalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelJournalNotificationBody(opts, journalIssue.Number, issueURL(opts.Repo, journalIssue.Number)),
	})
	if err != nil {
		return ChannelJournalResult{}, fmt.Errorf("queue channel journal notification: %w", err)
	}
	return ChannelJournalResult{
		JournalIssueNumber: journalIssue.Number,
		JournalIssueURL:    issueURL(opts.Repo, journalIssue.Number),
		JournalCreated:     created,
		JournalDuplicate:   duplicate,
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelJournalActionReport(ev Event, req ChannelJournalActionRequest, result ChannelJournalResult) string {
	status := "recorded"
	switch {
	case result.JournalDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.JournalDuplicate:
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
	b.WriteString("## GitClaw Channel Journal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_journal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- journal_issue: `#%d`\n", result.JournalIssueNumber)
	fmt.Fprintf(&b, "- journal_issue_url: `%s`\n", result.JournalIssueURL)
	fmt.Fprintf(&b, "- journal_issue_created: `%t`\n", result.JournalCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.JournalDuplicate)
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
	fmt.Fprintf(&b, "- journal_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.JournalID))
	fmt.Fprintf(&b, "- journal_id_auto: `%t`\n", req.AutoJournalID)
	fmt.Fprintf(&b, "- journal_date_sha256_12: `%s`\n", req.DateSHA)
	fmt.Fprintf(&b, "- journal_date_bytes: `%d`\n", req.DateBytes)
	fmt.Fprintf(&b, "- journal_date_lines: `%d`\n", req.DateLines)
	fmt.Fprintf(&b, "- journal_summary_sha256_12: `%s`\n", req.SummarySHA)
	fmt.Fprintf(&b, "- journal_summary_bytes: `%d`\n", req.SummaryBytes)
	fmt.Fprintf(&b, "- journal_summary_lines: `%d`\n", req.SummaryLines)
	fmt.Fprintf(&b, "- journal_highlights_sha256_12: `%s`\n", req.HighlightsSHA)
	fmt.Fprintf(&b, "- journal_highlights_bytes: `%d`\n", req.HighlightsBytes)
	fmt.Fprintf(&b, "- journal_highlights_lines: `%d`\n", req.HighlightsLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_journal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_journal_date_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_journal_summary_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_journal_highlights_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_journal_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel journal entry as a durable GitHub issue, then queued a provider-facing link back to the original thread. The journal issue contains the human-readable date, summary, and entry details; this source receipt keeps provider IDs, journal IDs, dates, summaries, entry details, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the journal-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent journal links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate journal issues are suppressed by `journal_id`; duplicate journal-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the journal issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelJournalIssueBody(opts ChannelJournalOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-journal journal_id=\"%s\" channel=\"%s\" date_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.JournalID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.JournalDate), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel journal.\n\n")
	fmt.Fprintf(&b, "- journal_id: %s\n", opts.JournalID)
	fmt.Fprintf(&b, "- journal_date: %s\n", opts.JournalDate)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- journal_mode: github-issue-journal\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Date\n\n")
	b.WriteString(strings.TrimSpace(opts.JournalDate))
	b.WriteString("\n\n")
	b.WriteString("## Summary\n\n")
	b.WriteString(strings.TrimSpace(opts.Summary))
	if strings.TrimSpace(opts.Highlights) != "" {
		b.WriteString("\n\n## Entry\n\n")
		b.WriteString(strings.TrimSpace(opts.Highlights))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel journal entry. Promotion into `.gitclaw/MEMORY.md` should happen only through the reviewed memory proposal flow.")
	return strings.TrimSpace(b.String())
}

func channelJournalActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelJournalActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelJournalIssueTarget(ev Event, req *ChannelJournalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel journal requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelJournalSummaryHighlights(trailing string, ev Event) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	var journalDate string
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "date:"):
			if journalDate == "" {
				journalDate = strings.TrimSpace(trimmed[len("date:"):])
			}
			continue
		case strings.HasPrefix(lower, "journal date:"):
			if journalDate == "" {
				journalDate = strings.TrimSpace(trimmed[len("journal date:"):])
			}
			continue
		case strings.HasPrefix(lower, "day:"):
			if journalDate == "" {
				journalDate = strings.TrimSpace(trimmed[len("day:"):])
			}
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultSummary := fmt.Sprintf("Channel journal from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return journalDate, defaultSummary, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var summary string
	var highlightLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "summary:"):
		summary = strings.TrimSpace(first[len("summary:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "journal:"):
		summary = strings.TrimSpace(first[len("journal:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "log:"):
		summary = strings.TrimSpace(first[len("log:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "title:"):
		summary = strings.TrimSpace(first[len("title:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "entry:"), strings.HasPrefix(lowerFirst, "highlights:"), strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "details:"), strings.HasPrefix(lowerFirst, "context:"):
		summary = defaultSummary
		highlightLines = cleaned
	default:
		summary = first
		highlightLines = cleaned[1:]
	}
	if summary == "" {
		summary = defaultSummary
	}
	highlights := strings.TrimSpace(strings.Join(highlightLines, "\n"))
	highlightsLower := strings.ToLower(strings.TrimSpace(highlights))
	switch {
	case strings.HasPrefix(highlightsLower, "entry:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("entry:"):])
	case strings.HasPrefix(highlightsLower, "highlights:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("highlights:"):])
	case strings.HasPrefix(highlightsLower, "notes:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("notes:"):])
	case strings.HasPrefix(highlightsLower, "details:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("details:"):])
	case strings.HasPrefix(highlightsLower, "context:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("context:"):])
	}
	return journalDate, summary, highlights
}

func normalizeChannelJournalOptions(opts ChannelJournalOptions) ChannelJournalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.JournalID = cleanChannelJournalID(opts.JournalID)
	opts.JournalDate = strings.TrimSpace(opts.JournalDate)
	opts.Summary = strings.TrimSpace(opts.Summary)
	opts.Highlights = strings.TrimSpace(opts.Highlights)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelJournalRoute(cfg Config, opts ChannelJournalOptions) (ChannelJournalOptions, error) {
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
		Body:      opts.Summary,
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

func validateChannelJournalOptions(opts ChannelJournalOptions) error {
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
	if opts.JournalID == "" {
		return fmt.Errorf("missing journal id")
	}
	if opts.JournalDate == "" {
		return fmt.Errorf("missing journal date")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing journal source issue")
	}
	if opts.Summary == "" {
		return fmt.Errorf("missing journal summary")
	}
	return nil
}

func validateChannelJournalActionRequestOptions(opts ChannelJournalOptions) error {
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
	if opts.JournalID == "" {
		return fmt.Errorf("missing journal id")
	}
	if opts.JournalDate == "" {
		return fmt.Errorf("missing journal date")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing journal source issue")
	}
	if opts.Summary == "" {
		return fmt.Errorf("missing journal summary")
	}
	return nil
}

func findOrCreateChannelJournalIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelJournalOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel journal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelJournalMatches(issue.Body, opts.JournalID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelJournalIssueTitle(opts), RenderChannelJournalIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel journal issue: %w", err)
	}
	return issue, true, false, nil
}

func channelJournalIssueTitle(opts ChannelJournalOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Summary), "\n", " ")
	if title == "" {
		title = opts.JournalID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel journal: " + title
}

func channelJournalMatches(body, journalID string) bool {
	return HasChannelJournalMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`journal_id="%s"`, escapeMarkerValue(cleanChannelJournalID(journalID))))
}

func cleanChannelJournalID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelJournalID(ev Event, channel, threadID, sourceMessageID, journalDate, summary, highlights string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, journalDate, summary, highlights}, "|")
	return fmt.Sprintf("journal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelJournalNotifyMessageID(ev Event, journalID string) string {
	seed := strings.Join([]string{eventID(ev), journalID}, "|")
	return fmt.Sprintf("gitclaw-channel-journal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelJournalNotificationBody(opts ChannelJournalOptions, journalIssueNumber int, journalIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel journal recorded.\n\n")
	if journalIssueNumber > 0 {
		fmt.Fprintf(&b, "Journal: #%d\n", journalIssueNumber)
	}
	if journalIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", journalIssueURL)
	}
	fmt.Fprintf(&b, "Date: %s\n", strings.TrimSpace(opts.JournalDate))
	fmt.Fprintf(&b, "Summary: %s\n", strings.TrimSpace(opts.Summary))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
