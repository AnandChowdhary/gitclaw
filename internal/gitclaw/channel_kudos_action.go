package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelKudosOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	KudosID           string
	Recipient         string
	Reason            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelKudosResult struct {
	KudosIssueNumber int
	KudosIssueURL    string
	KudosCreated     bool
	KudosDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelKudosActionRequest struct {
	Options             ChannelKudosOptions
	Command             string
	Subcommand          string
	AutoKudosID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RecipientSHA        string
	RecipientBytes      int
	RecipientLines      int
	ReasonSHA           string
	ReasonBytes         int
	ReasonLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelKudosActionRequest(ev Event, cfg Config) bool {
	return isChannelKudosActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelKudosActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "kudos", "thanks", "thank", "shoutout", "appreciation", "praise":
		return true
	default:
		return false
	}
}

func BuildChannelKudosActionRequest(ev Event, cfg Config) (ChannelKudosActionRequest, error) {
	fields, trailing, ok := channelKudosActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelKudosActionRequest{}, fmt.Errorf("missing channel kudos command")
	}
	req := ChannelKudosActionRequest{
		Options: ChannelKudosOptions{
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
				return ChannelKudosActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--kudos-id", "--thanks-id", "--appreciation-id", "--praise-id", "--id":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.KudosID = cleanChannelKudosID(fields[i+1])
			i++
		case "--to", "--recipient", "--for":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Recipient = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelKudosActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelKudosActionRequest{}, fmt.Errorf("unknown channel kudos argument %q", field)
			}
			if req.Options.KudosID == "" {
				req.Options.KudosID = cleanChannelKudosID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelKudosActionRequest{}, fmt.Errorf("unexpected channel kudos argument %q", field)
		}
	}
	if err := applyChannelKudosIssueTarget(ev, &req); err != nil {
		return ChannelKudosActionRequest{}, err
	}
	recipient, reason := parseChannelKudosRecipientReason(trailing, ev)
	if req.Options.Recipient == "" {
		req.Options.Recipient = recipient
	}
	req.Options.Reason = reason
	if strings.TrimSpace(req.Options.KudosID) == "" {
		req.Options.KudosID = autoChannelKudosID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Recipient, reason)
		req.AutoKudosID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelKudosNotifyMessageID(ev, req.Options.KudosID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelKudosOptions(req.Options)
	if err := validateChannelKudosActionRequestOptions(req.Options); err != nil {
		return ChannelKudosActionRequest{}, err
	}
	req.RecipientSHA = shortDocumentHash(req.Options.Recipient)
	req.RecipientBytes = len(req.Options.Recipient)
	req.RecipientLines = lineCount(req.Options.Recipient)
	req.ReasonSHA = shortDocumentHash(req.Options.Reason)
	req.ReasonBytes = len(req.Options.Reason)
	req.ReasonLines = lineCount(req.Options.Reason)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelKudosNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelKudos(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelKudosOptions) (ChannelKudosResult, error) {
	opts = normalizeChannelKudosOptions(opts)
	var err error
	opts, err = applyChannelKudosRoute(cfg, opts)
	if err != nil {
		return ChannelKudosResult{}, err
	}
	if err := validateChannelKudosOptions(opts); err != nil {
		return ChannelKudosResult{}, err
	}
	kudosIssue, created, duplicate, err := findOrCreateChannelKudosIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelKudosResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelKudosNotificationBody(opts, kudosIssue.Number, issueURL(opts.Repo, kudosIssue.Number)),
	})
	if err != nil {
		return ChannelKudosResult{}, fmt.Errorf("queue channel kudos notification: %w", err)
	}
	return ChannelKudosResult{
		KudosIssueNumber: kudosIssue.Number,
		KudosIssueURL:    issueURL(opts.Repo, kudosIssue.Number),
		KudosCreated:     created,
		KudosDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelKudosActionReport(ev Event, req ChannelKudosActionRequest, result ChannelKudosResult) string {
	status := "captured"
	switch {
	case result.KudosDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.KudosDuplicate:
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
	b.WriteString("## GitClaw Channel Kudos Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_kudos_status: `%s`\n", status)
	fmt.Fprintf(&b, "- kudos_issue: `#%d`\n", result.KudosIssueNumber)
	fmt.Fprintf(&b, "- kudos_issue_url: `%s`\n", result.KudosIssueURL)
	fmt.Fprintf(&b, "- kudos_issue_created: `%t`\n", result.KudosCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.KudosDuplicate)
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
	fmt.Fprintf(&b, "- kudos_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.KudosID))
	fmt.Fprintf(&b, "- kudos_id_auto: `%t`\n", req.AutoKudosID)
	fmt.Fprintf(&b, "- kudos_recipient_sha256_12: `%s`\n", req.RecipientSHA)
	fmt.Fprintf(&b, "- kudos_recipient_bytes: `%d`\n", req.RecipientBytes)
	fmt.Fprintf(&b, "- kudos_recipient_lines: `%d`\n", req.RecipientLines)
	fmt.Fprintf(&b, "- kudos_reason_sha256_12: `%s`\n", req.ReasonSHA)
	fmt.Fprintf(&b, "- kudos_reason_bytes: `%d`\n", req.ReasonBytes)
	fmt.Fprintf(&b, "- kudos_reason_lines: `%d`\n", req.ReasonLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_kudos_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_kudos_recipient_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_kudos_reason_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_kudos_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured channel-origin kudos as a durable GitHub issue, then queued a provider-facing acknowledgement back to the mirrored thread. The kudos issue contains the human-readable recipient and reason; this source receipt keeps provider IDs, kudos IDs, recipients, reasons, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the kudos-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent kudos links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate kudos issues are suppressed by `kudos_id`; duplicate kudos-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the kudos issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelKudosIssueBody(opts ChannelKudosOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-kudos kudos_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.KudosID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel kudos.\n\n")
	fmt.Fprintf(&b, "- kudos_id: %s\n", opts.KudosID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- kudos_mode: github-issue-kudos\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Recipient\n\n")
	b.WriteString(strings.TrimSpace(opts.Recipient))
	if strings.TrimSpace(opts.Reason) != "" {
		b.WriteString("\n\n## Reason\n\n")
		b.WriteString(strings.TrimSpace(opts.Reason))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the acknowledgement, turning it into a task, or promoting a repeated team pattern into memory, skill, or proactive workflow review.")
	return strings.TrimSpace(b.String())
}

func channelKudosActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelKudosActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelKudosIssueTarget(ev Event, req *ChannelKudosActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel kudos requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelKudosRecipientReason(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultRecipient := fmt.Sprintf("Channel kudos from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultRecipient, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var recipient string
	var noteLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "to:"):
		recipient = strings.TrimSpace(first[len("to:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "recipient:"):
		recipient = strings.TrimSpace(first[len("recipient:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "for:"):
		recipient = strings.TrimSpace(first[len("for:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "kudos:"):
		recipient = strings.TrimSpace(first[len("kudos:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "thanks:"):
		recipient = strings.TrimSpace(first[len("thanks:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "reason:"), strings.HasPrefix(lowerFirst, "because:"), strings.HasPrefix(lowerFirst, "context:"), strings.HasPrefix(lowerFirst, "why:"):
		recipient = defaultRecipient
		noteLines = cleaned
	default:
		recipient = first
		noteLines = cleaned[1:]
	}
	if recipient == "" {
		recipient = defaultRecipient
	}
	reason := strings.TrimSpace(strings.Join(noteLines, "\n"))
	reasonLower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.HasPrefix(reasonLower, "reason:"):
		reason = strings.TrimSpace(strings.TrimSpace(reason)[len("reason:"):])
	case strings.HasPrefix(reasonLower, "because:"):
		reason = strings.TrimSpace(strings.TrimSpace(reason)[len("because:"):])
	case strings.HasPrefix(reasonLower, "context:"):
		reason = strings.TrimSpace(strings.TrimSpace(reason)[len("context:"):])
	case strings.HasPrefix(reasonLower, "why:"):
		reason = strings.TrimSpace(strings.TrimSpace(reason)[len("why:"):])
	}
	return recipient, reason
}

func normalizeChannelKudosOptions(opts ChannelKudosOptions) ChannelKudosOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.KudosID = cleanChannelKudosID(opts.KudosID)
	opts.Recipient = strings.TrimSpace(opts.Recipient)
	opts.Reason = strings.TrimSpace(opts.Reason)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelKudosRoute(cfg Config, opts ChannelKudosOptions) (ChannelKudosOptions, error) {
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
		Body:      opts.Recipient,
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

func validateChannelKudosOptions(opts ChannelKudosOptions) error {
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
	if opts.KudosID == "" {
		return fmt.Errorf("missing kudos id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing kudos source issue")
	}
	if opts.Recipient == "" {
		return fmt.Errorf("missing kudos recipient")
	}
	return nil
}

func validateChannelKudosActionRequestOptions(opts ChannelKudosOptions) error {
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
	if opts.KudosID == "" {
		return fmt.Errorf("missing kudos id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing kudos source issue")
	}
	if opts.Recipient == "" {
		return fmt.Errorf("missing kudos recipient")
	}
	return nil
}

func findOrCreateChannelKudosIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelKudosOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel kudos issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelKudosMatches(issue.Body, opts.KudosID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelKudosIssueTitle(opts), RenderChannelKudosIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel kudos issue: %w", err)
	}
	return issue, true, false, nil
}

func channelKudosIssueTitle(opts ChannelKudosOptions) string {
	recipient := strings.ReplaceAll(strings.TrimSpace(opts.Recipient), "\n", " ")
	if recipient == "" {
		recipient = opts.KudosID
	}
	if len(recipient) > 80 {
		recipient = strings.TrimSpace(recipient[:80])
	}
	return "GitClaw channel kudos: " + recipient
}

func channelKudosMatches(body, kudosID string) bool {
	return HasChannelKudosMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`kudos_id="%s"`, escapeMarkerValue(cleanChannelKudosID(kudosID))))
}

func cleanChannelKudosID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelKudosID(ev Event, channel, threadID, sourceMessageID, recipient, reason string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, recipient, reason}, "|")
	return fmt.Sprintf("kudos-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelKudosNotifyMessageID(ev Event, kudosID string) string {
	seed := strings.Join([]string{eventID(ev), kudosID}, "|")
	return fmt.Sprintf("gitclaw-channel-kudos-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelKudosNotificationBody(opts ChannelKudosOptions, kudosIssueNumber int, kudosIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel kudos captured.\n\n")
	if kudosIssueNumber > 0 {
		fmt.Fprintf(&b, "Kudos: #%d\n", kudosIssueNumber)
	}
	if kudosIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", kudosIssueURL)
	}
	fmt.Fprintf(&b, "Recipient: %s\n", strings.TrimSpace(opts.Recipient))
	b.WriteString("\nContinue shaping it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
