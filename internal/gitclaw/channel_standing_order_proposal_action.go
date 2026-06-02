package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelStandingOrderProposalOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ProposalID        string
	Cadence           string
	Title             string
	ProposalBody      string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelStandingOrderProposalResult struct {
	ProposalIssueNumber int
	ProposalIssueURL    string
	ProposalCreated     bool
	ProposalDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelStandingOrderProposalActionRequest struct {
	Options             ChannelStandingOrderProposalOptions
	Command             string
	Subcommand          string
	AutoProposalID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	CadenceSHA          string
	CadenceBytes        int
	CadenceLines        int
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ProposalBodySHA     string
	ProposalBodyBytes   int
	ProposalBodyLines   int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelStandingOrderProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelStandingOrderProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelStandingOrderProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-order", "order-propose", "order-proposal", "standing-order", "standing-order-proposal", "propose-standing-order", "propose-orders":
		return true
	default:
		return false
	}
}

func BuildChannelStandingOrderProposalActionRequest(ev Event, cfg Config) (ChannelStandingOrderProposalActionRequest, error) {
	fields, trailing, ok := channelStandingOrderProposalActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("missing channel standing-order proposal command")
	}
	req := ChannelStandingOrderProposalActionRequest{
		Options: ChannelStandingOrderProposalOptions{
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
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--id", "--proposal-id", "--order-id", "--standing-order-id":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProposalID = cleanChannelStandingOrderProposalID(fields[i+1])
			i++
		case "--cadence", "--frequency", "--interval", "--every", "--trigger":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Cadence = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("unknown channel standing-order proposal argument %q", field)
			}
			if req.Options.ProposalID == "" {
				req.Options.ProposalID = cleanChannelStandingOrderProposalID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelStandingOrderProposalActionRequest{}, fmt.Errorf("unexpected channel standing-order proposal argument %q", field)
		}
	}
	if err := applyChannelStandingOrderProposalIssueTarget(ev, &req); err != nil {
		return ChannelStandingOrderProposalActionRequest{}, err
	}
	title, proposalBody := parseChannelStandingOrderProposalText(trailing, ev)
	req.Options.Title = title
	req.Options.ProposalBody = proposalBody
	if strings.TrimSpace(req.Options.ProposalID) == "" {
		req.Options.ProposalID = autoChannelStandingOrderProposalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, proposalBody)
		req.AutoProposalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelStandingOrderProposalNotifyMessageID(ev, req.Options.ProposalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelStandingOrderProposalOptions(req.Options)
	if err := validateChannelStandingOrderProposalActionRequestOptions(req.Options); err != nil {
		return ChannelStandingOrderProposalActionRequest{}, err
	}
	req.CadenceSHA = shortDocumentHash(req.Options.Cadence)
	req.CadenceBytes = len(req.Options.Cadence)
	req.CadenceLines = lineCount(req.Options.Cadence)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ProposalBodySHA = shortDocumentHash(req.Options.ProposalBody)
	req.ProposalBodyBytes = len(req.Options.ProposalBody)
	req.ProposalBodyLines = lineCount(req.Options.ProposalBody)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(RenderChannelStandingOrderProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelStandingOrderProposal(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelStandingOrderProposalOptions) (ChannelStandingOrderProposalResult, error) {
	opts = normalizeChannelStandingOrderProposalOptions(opts)
	var err error
	opts, err = applyChannelStandingOrderProposalRoute(cfg, opts)
	if err != nil {
		return ChannelStandingOrderProposalResult{}, err
	}
	if err := validateChannelStandingOrderProposalOptions(opts); err != nil {
		return ChannelStandingOrderProposalResult{}, err
	}
	proposalIssue, created, duplicate, err := findOrCreateChannelStandingOrderProposalIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelStandingOrderProposalResult{}, err
	}
	notificationBody := RenderChannelStandingOrderProposalNotificationBody(opts, proposalIssue.Number, issueURL(opts.Repo, proposalIssue.Number))
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelStandingOrderProposalResult{}, fmt.Errorf("queue channel standing-order proposal notification: %w", err)
	}
	return ChannelStandingOrderProposalResult{
		ProposalIssueNumber: proposalIssue.Number,
		ProposalIssueURL:    issueURL(opts.Repo, proposalIssue.Number),
		ProposalCreated:     created,
		ProposalDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelStandingOrderProposalActionReport(ev Event, req ChannelStandingOrderProposalActionRequest, result ChannelStandingOrderProposalResult) string {
	status := "created"
	switch {
	case result.ProposalDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ProposalDuplicate:
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
	notificationBodySHA := result.NotificationBodySHA
	if notificationBodySHA == "" {
		notificationBodySHA = req.NotificationBodySHA
	}
	notificationBytes := result.NotificationBytes
	if notificationBytes == 0 {
		notificationBytes = len(RenderChannelStandingOrderProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	}
	notificationLines := result.NotificationLines
	if notificationLines == 0 {
		notificationLines = lineCount(RenderChannelStandingOrderProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Standing Order Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Options.SourceCommentID)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_standing_order_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- standing_order_proposal_issue: `#%d`\n", result.ProposalIssueNumber)
	fmt.Fprintf(&b, "- standing_order_proposal_issue_url: `%s`\n", result.ProposalIssueURL)
	fmt.Fprintf(&b, "- standing_order_proposal_issue_created: `%t`\n", result.ProposalCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ProposalDuplicate)
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
	fmt.Fprintf(&b, "- proposal_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ProposalID))
	fmt.Fprintf(&b, "- proposal_id_auto: `%t`\n", req.AutoProposalID)
	fmt.Fprintf(&b, "- proposal_cadence_sha256_12: `%s`\n", req.CadenceSHA)
	fmt.Fprintf(&b, "- proposal_cadence_bytes: `%d`\n", req.CadenceBytes)
	fmt.Fprintf(&b, "- proposal_cadence_lines: `%d`\n", req.CadenceLines)
	fmt.Fprintf(&b, "- proposal_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- proposal_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- proposal_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- proposal_body_sha256_12: `%s`\n", req.ProposalBodySHA)
	fmt.Fprintf(&b, "- proposal_body_bytes: `%d`\n", req.ProposalBodyBytes)
	fmt.Fprintf(&b, "- proposal_body_lines: `%d`\n", req.ProposalBodyLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- target_path_sha256_12: `%s`\n", shortDocumentHash(standingOrdersPath))
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-standing-orders-file")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- standing_order_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- schedule_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_cadence_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_standing_order_proposal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub issue for a standing-order proposal from a mirrored channel thread, then queued a provider-facing proposal link back to that thread. The proposal issue is only a review surface; this action does not call a model, edit standing orders, create schedules, or mutate the repository.\n\n")
	b.WriteString("### Proposal Review Path\n")
	fmt.Fprintf(&b, "- continue on proposal issue: `#%d`\n", result.ProposalIssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the proposal issue to discuss scope, triggers, approval gates, escalation, and cron enforcement\n")
	b.WriteString("- apply accepted proposals through a normal branch and PR that edits `.gitclaw/STANDING_ORDERS.md` and any scheduled workflow prompts\n")
	b.WriteString("- duplicate proposal issues are suppressed by `proposal_id`; duplicate notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelStandingOrderProposalIssueBody(opts ChannelStandingOrderProposalOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-standing-order-proposal proposal_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ProposalID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel standing-order proposal.\n\n")
	fmt.Fprintf(&b, "- proposal_id: %s\n", opts.ProposalID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- cadence: %s\n", opts.Cadence)
	fmt.Fprintf(&b, "- target_path: %s\n", standingOrdersPath)
	b.WriteString("- proposal_mode: github-issue-review\n")
	b.WriteString("- review_pr_required: true\n")
	b.WriteString("- standing_order_file_written: false\n")
	b.WriteString("- schedule_created: false\n")
	b.WriteString("- repository_mutation_performed: false\n")
	b.WriteString("- provider_delivery_performed: false\n")
	b.WriteString("- raw_thread_id_included: false\n")
	b.WriteString("- raw_source_message_id_included: false\n")
	b.WriteString("- raw_channel_message_body_included: false\n")
	b.WriteString("- proposal_body_included: true\n\n")
	b.WriteString("## Proposed Standing Order\n\n")
	fmt.Fprintf(&b, "### %s\n\n", strings.TrimSpace(opts.Title))
	b.WriteString(strings.TrimSpace(opts.ProposalBody))
	b.WriteString("\n\n## Review Checklist\n\n")
	b.WriteString("- scope is narrow and explicit\n")
	b.WriteString("- trigger or cadence is defined\n")
	b.WriteString("- approval gate is explicit\n")
	b.WriteString("- escalation rule says when to stop and ask\n")
	b.WriteString("- accepted changes happen through a normal PR\n")
	b.WriteString("- any schedule is implemented through reviewed GitHub Actions workflow changes\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelStandingOrderProposalNotificationBody(opts ChannelStandingOrderProposalOptions, proposalIssueNumber int, proposalIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel standing-order proposal created.\n\n")
	if proposalIssueNumber > 0 {
		fmt.Fprintf(&b, "Proposal issue: #%d\n", proposalIssueNumber)
	}
	if proposalIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", proposalIssueURL)
	}
	fmt.Fprintf(&b, "Proposal ID: %s\n", opts.ProposalID)
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	fmt.Fprintf(&b, "Cadence: %s\n", strings.TrimSpace(opts.Cadence))
	b.WriteString("Review PR required: true\n")
	b.WriteString("Standing orders changed: false\n")
	b.WriteString("Schedule created: false\n\n")
	b.WriteString("Continue in the linked GitHub issue to review scope, trigger, approval gate, escalation, and any scheduled workflow enforcement.")
	return strings.TrimSpace(b.String())
}

func channelStandingOrderProposalActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelStandingOrderProposalActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelStandingOrderProposalIssueTarget(ev Event, req *ChannelStandingOrderProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel standing-order proposal requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelStandingOrderProposalText(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Standing order proposal from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	title := ""
	proposalLines := cleaned
	for _, prefix := range []string{"title:", "subject:", "order:", "standing order:"} {
		if strings.HasPrefix(lowerFirst, prefix) {
			title = strings.TrimSpace(first[len(prefix):])
			proposalLines = cleaned[1:]
			break
		}
	}
	if title == "" {
		if strings.HasPrefix(lowerFirst, "## program:") {
			title = strings.TrimSpace(first[len("## program:"):])
		} else if strings.HasPrefix(lowerFirst, "## standing order:") {
			title = strings.TrimSpace(first[len("## standing order:"):])
		} else {
			title = first
		}
	}
	if strings.TrimSpace(title) == "" {
		title = defaultTitle
	}
	proposalBody := strings.TrimSpace(strings.Join(proposalLines, "\n"))
	if proposalBody == "" {
		proposalBody = strings.TrimSpace(title)
	}
	return title, proposalBody
}

func normalizeChannelStandingOrderProposalOptions(opts ChannelStandingOrderProposalOptions) ChannelStandingOrderProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ProposalID = cleanChannelStandingOrderProposalID(opts.ProposalID)
	opts.Cadence = strings.TrimSpace(opts.Cadence)
	if opts.Cadence == "" {
		opts.Cadence = "reviewed-schedule-required"
	}
	opts.Title = strings.TrimSpace(opts.Title)
	opts.ProposalBody = strings.TrimSpace(opts.ProposalBody)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelStandingOrderProposalRoute(cfg Config, opts ChannelStandingOrderProposalOptions) (ChannelStandingOrderProposalOptions, error) {
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

func validateChannelStandingOrderProposalOptions(opts ChannelStandingOrderProposalOptions) error {
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
	if opts.ProposalID == "" {
		return fmt.Errorf("missing standing-order proposal id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing standing-order proposal source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing standing-order proposal title")
	}
	if opts.ProposalBody == "" {
		return fmt.Errorf("missing standing-order proposal body")
	}
	return nil
}

func validateChannelStandingOrderProposalActionRequestOptions(opts ChannelStandingOrderProposalOptions) error {
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
	if opts.ProposalID == "" {
		return fmt.Errorf("missing standing-order proposal id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing standing-order proposal source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing standing-order proposal title")
	}
	if opts.ProposalBody == "" {
		return fmt.Errorf("missing standing-order proposal body")
	}
	return nil
}

func findOrCreateChannelStandingOrderProposalIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelStandingOrderProposalOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel standing-order proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelStandingOrderProposalMatches(issue.Body, opts.ProposalID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelStandingOrderProposalIssueTitle(opts), RenderChannelStandingOrderProposalIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel standing-order proposal issue: %w", err)
	}
	return issue, true, false, nil
}

func channelStandingOrderProposalIssueTitle(opts ChannelStandingOrderProposalOptions) string {
	return "GitClaw standing order proposal: " + opts.ProposalID
}

func channelStandingOrderProposalMatches(body, proposalID string) bool {
	return HasChannelStandingOrderProposalMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`proposal_id="%s"`, escapeMarkerValue(cleanChannelStandingOrderProposalID(proposalID))))
}

func cleanChannelStandingOrderProposalID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelStandingOrderProposalID(ev Event, channel, threadID, sourceMessageID, title, proposalBody string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, proposalBody}, "|")
	return fmt.Sprintf("standing-order-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelStandingOrderProposalNotifyMessageID(ev Event, proposalID string) string {
	seed := strings.Join([]string{eventID(ev), proposalID}, "|")
	return fmt.Sprintf("gitclaw-channel-standing-order-%s-%s", eventID(ev), shortDocumentHash(seed))
}
