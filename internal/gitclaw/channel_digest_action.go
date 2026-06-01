package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelDigestOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	DigestID          string
	Summary           string
	Highlights        string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelDigestResult struct {
	DigestIssueNumber int
	DigestIssueURL    string
	DigestCreated     bool
	DigestDuplicate   bool
	Notification      ChannelSendResult
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	MessageHash       string
	NotifyHash        string
}

type ChannelDigestActionRequest struct {
	Options             ChannelDigestOptions
	Command             string
	Subcommand          string
	AutoDigestID        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
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

func IsChannelDigestActionRequest(ev Event, cfg Config) bool {
	return isChannelDigestActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelDigestActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "digest", "brief", "recap", "summary", "summarize":
		return true
	default:
		return false
	}
}

func BuildChannelDigestActionRequest(ev Event, cfg Config) (ChannelDigestActionRequest, error) {
	fields, trailing, ok := channelDigestActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelDigestActionRequest{}, fmt.Errorf("missing channel digest command")
	}
	req := ChannelDigestActionRequest{
		Options: ChannelDigestOptions{
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
				return ChannelDigestActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelDigestActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelDigestActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelDigestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelDigestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--digest-id", "--brief-id", "--recap-id", "--summary-id", "--id":
			if i+1 >= len(fields) {
				return ChannelDigestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DigestID = cleanChannelDigestID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelDigestActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelDigestActionRequest{}, fmt.Errorf("unknown channel digest argument %q", field)
			}
			if req.Options.DigestID == "" {
				req.Options.DigestID = cleanChannelDigestID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelDigestActionRequest{}, fmt.Errorf("unexpected channel digest argument %q", field)
		}
	}
	if err := applyChannelDigestIssueTarget(ev, &req); err != nil {
		return ChannelDigestActionRequest{}, err
	}
	summary, highlights := parseChannelDigestSummaryHighlights(trailing, ev)
	req.Options.Summary = summary
	req.Options.Highlights = highlights
	if strings.TrimSpace(req.Options.DigestID) == "" {
		req.Options.DigestID = autoChannelDigestID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, summary, highlights)
		req.AutoDigestID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelDigestNotifyMessageID(ev, req.Options.DigestID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelDigestOptions(req.Options)
	if err := validateChannelDigestActionRequestOptions(req.Options); err != nil {
		return ChannelDigestActionRequest{}, err
	}
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelDigestNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelDigest(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDigestOptions) (ChannelDigestResult, error) {
	opts = normalizeChannelDigestOptions(opts)
	var err error
	opts, err = applyChannelDigestRoute(cfg, opts)
	if err != nil {
		return ChannelDigestResult{}, err
	}
	if err := validateChannelDigestOptions(opts); err != nil {
		return ChannelDigestResult{}, err
	}
	digestIssue, created, duplicate, err := findOrCreateChannelDigestIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelDigestResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelDigestNotificationBody(opts, digestIssue.Number, issueURL(opts.Repo, digestIssue.Number)),
	})
	if err != nil {
		return ChannelDigestResult{}, fmt.Errorf("queue channel digest notification: %w", err)
	}
	return ChannelDigestResult{
		DigestIssueNumber: digestIssue.Number,
		DigestIssueURL:    issueURL(opts.Repo, digestIssue.Number),
		DigestCreated:     created,
		DigestDuplicate:   duplicate,
		Notification:      notification,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelDigestActionReport(ev Event, req ChannelDigestActionRequest, result ChannelDigestResult) string {
	status := "recorded"
	switch {
	case result.DigestDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.DigestDuplicate:
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
	b.WriteString("## GitClaw Channel Digest Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_digest_status: `%s`\n", status)
	fmt.Fprintf(&b, "- digest_issue: `#%d`\n", result.DigestIssueNumber)
	fmt.Fprintf(&b, "- digest_issue_url: `%s`\n", result.DigestIssueURL)
	fmt.Fprintf(&b, "- digest_issue_created: `%t`\n", result.DigestCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.DigestDuplicate)
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
	fmt.Fprintf(&b, "- digest_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.DigestID))
	fmt.Fprintf(&b, "- digest_id_auto: `%t`\n", req.AutoDigestID)
	fmt.Fprintf(&b, "- digest_summary_sha256_12: `%s`\n", req.SummarySHA)
	fmt.Fprintf(&b, "- digest_summary_bytes: `%d`\n", req.SummaryBytes)
	fmt.Fprintf(&b, "- digest_summary_lines: `%d`\n", req.SummaryLines)
	fmt.Fprintf(&b, "- digest_highlights_sha256_12: `%s`\n", req.HighlightsSHA)
	fmt.Fprintf(&b, "- digest_highlights_bytes: `%d`\n", req.HighlightsBytes)
	fmt.Fprintf(&b, "- digest_highlights_lines: `%d`\n", req.HighlightsLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_digest_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_digest_summary_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_digest_highlights_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_digest_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel digest as a durable GitHub issue, then queued a provider-facing link back to the original thread. The digest issue contains the human-readable summary and highlights; this source receipt keeps provider IDs, digest IDs, summaries, highlights, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the digest-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent digest links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate digest issues are suppressed by `digest_id`; duplicate digest-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the digest issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelDigestIssueBody(opts ChannelDigestOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-digest digest_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.DigestID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel digest.\n\n")
	fmt.Fprintf(&b, "- digest_id: %s\n", opts.DigestID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- digest_mode: github-issue-digest\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Summary\n\n")
	b.WriteString(strings.TrimSpace(opts.Summary))
	if strings.TrimSpace(opts.Highlights) != "" {
		b.WriteString("\n\n## Highlights\n\n")
		b.WriteString(strings.TrimSpace(opts.Highlights))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel digest.")
	return strings.TrimSpace(b.String())
}

func channelDigestActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelDigestActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelDigestIssueTarget(ev Event, req *ChannelDigestActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel digest requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelDigestSummaryHighlights(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultSummary := fmt.Sprintf("Channel digest from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultSummary, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var summary string
	var highlightLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "summary:"):
		summary = strings.TrimSpace(first[len("summary:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "digest:"):
		summary = strings.TrimSpace(first[len("digest:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "title:"):
		summary = strings.TrimSpace(first[len("title:"):])
		highlightLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "highlights:"), strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"):
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
	case strings.HasPrefix(highlightsLower, "highlights:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("highlights:"):])
	case strings.HasPrefix(highlightsLower, "notes:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("notes:"):])
	case strings.HasPrefix(highlightsLower, "context:"):
		highlights = strings.TrimSpace(strings.TrimSpace(highlights)[len("context:"):])
	}
	return summary, highlights
}

func normalizeChannelDigestOptions(opts ChannelDigestOptions) ChannelDigestOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.DigestID = cleanChannelDigestID(opts.DigestID)
	opts.Summary = strings.TrimSpace(opts.Summary)
	opts.Highlights = strings.TrimSpace(opts.Highlights)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelDigestRoute(cfg Config, opts ChannelDigestOptions) (ChannelDigestOptions, error) {
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

func validateChannelDigestOptions(opts ChannelDigestOptions) error {
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
	if opts.DigestID == "" {
		return fmt.Errorf("missing digest id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing digest source issue")
	}
	if opts.Summary == "" {
		return fmt.Errorf("missing digest summary")
	}
	return nil
}

func validateChannelDigestActionRequestOptions(opts ChannelDigestOptions) error {
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
	if opts.DigestID == "" {
		return fmt.Errorf("missing digest id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing digest source issue")
	}
	if opts.Summary == "" {
		return fmt.Errorf("missing digest summary")
	}
	return nil
}

func findOrCreateChannelDigestIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDigestOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel digest issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelDigestMatches(issue.Body, opts.DigestID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelDigestIssueTitle(opts), RenderChannelDigestIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel digest issue: %w", err)
	}
	return issue, true, false, nil
}

func channelDigestIssueTitle(opts ChannelDigestOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Summary), "\n", " ")
	if title == "" {
		title = opts.DigestID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel digest: " + title
}

func channelDigestMatches(body, digestID string) bool {
	return HasChannelDigestMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`digest_id="%s"`, escapeMarkerValue(cleanChannelDigestID(digestID))))
}

func cleanChannelDigestID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelDigestID(ev Event, channel, threadID, sourceMessageID, summary, highlights string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, summary, highlights}, "|")
	return fmt.Sprintf("digest-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelDigestNotifyMessageID(ev Event, digestID string) string {
	seed := strings.Join([]string{eventID(ev), digestID}, "|")
	return fmt.Sprintf("gitclaw-channel-digest-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelDigestNotificationBody(opts ChannelDigestOptions, digestIssueNumber int, digestIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel digest recorded.\n\n")
	if digestIssueNumber > 0 {
		fmt.Fprintf(&b, "Digest: #%d\n", digestIssueNumber)
	}
	if digestIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", digestIssueURL)
	}
	fmt.Fprintf(&b, "Summary: %s\n", strings.TrimSpace(opts.Summary))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
