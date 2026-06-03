package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelDockOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	DockID            string
	TargetRoute       string
	Reason            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelDockResult struct {
	DockIssueNumber int
	DockIssueURL    string
	DockCreated     bool
	DockDuplicate   bool
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
}

type ChannelDockActionRequest struct {
	Options             ChannelDockOptions
	Command             string
	Subcommand          string
	AutoDockID          bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TargetRouteSHA      string
	TargetRouteBytes    int
	ReasonSHA           string
	ReasonBytes         int
	ReasonLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelDockActionRequest(ev Event, cfg Config) bool {
	return isChannelDockActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelDockActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "dock", "redock", "route-request", "routing-request", "continue-in", "move-thread", "switch-route":
		return true
	default:
		return false
	}
}

func BuildChannelDockActionRequest(ev Event, cfg Config) (ChannelDockActionRequest, error) {
	fields, trailing, ok := channelDockActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelDockActionRequest{}, fmt.Errorf("missing channel dock command")
	}
	req := ChannelDockActionRequest{
		Options: ChannelDockOptions{
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
				return ChannelDockActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--dock-id", "--route-request-id", "--routing-request-id", "--id":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DockID = cleanChannelDockID(fields[i+1])
			i++
		case "--target-route", "--dock-route", "--to-route", "--destination-route", "--to":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetRoute = cleanChannelRouteName(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelDockActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelDockActionRequest{}, fmt.Errorf("unknown channel dock argument %q", field)
			}
			if req.Options.TargetRoute == "" {
				req.Options.TargetRoute = cleanChannelRouteName(field)
				continue
			}
			if req.Options.DockID == "" {
				req.Options.DockID = cleanChannelDockID(field)
				continue
			}
			return ChannelDockActionRequest{}, fmt.Errorf("unexpected channel dock argument %q", field)
		}
	}
	if err := applyChannelDockIssueTarget(ev, &req); err != nil {
		return ChannelDockActionRequest{}, err
	}
	reason := parseChannelDockReason(trailing)
	req.Options.Reason = reason
	if strings.TrimSpace(req.Options.DockID) == "" {
		req.Options.DockID = autoChannelDockID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.TargetRoute, reason)
		req.AutoDockID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelDockNotifyMessageID(ev, req.Options.DockID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelDockOptions(req.Options)
	if err := validateChannelDockActionRequestOptions(req.Options); err != nil {
		return ChannelDockActionRequest{}, err
	}
	req.TargetRouteSHA = shortDocumentHash(req.Options.TargetRoute)
	req.TargetRouteBytes = len(req.Options.TargetRoute)
	req.ReasonSHA = shortDocumentHash(req.Options.Reason)
	req.ReasonBytes = len(req.Options.Reason)
	req.ReasonLines = lineCount(req.Options.Reason)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelDockNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelDock(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDockOptions) (ChannelDockResult, error) {
	opts = normalizeChannelDockOptions(opts)
	var err error
	opts, err = applyChannelDockRoute(cfg, opts)
	if err != nil {
		return ChannelDockResult{}, err
	}
	if err := validateChannelDockOptions(opts); err != nil {
		return ChannelDockResult{}, err
	}
	dockIssue, created, duplicate, err := findOrCreateChannelDockIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelDockResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelDockNotificationBody(opts, dockIssue.Number, issueURL(opts.Repo, dockIssue.Number)),
	})
	if err != nil {
		return ChannelDockResult{}, fmt.Errorf("queue channel dock notification: %w", err)
	}
	return ChannelDockResult{
		DockIssueNumber: dockIssue.Number,
		DockIssueURL:    issueURL(opts.Repo, dockIssue.Number),
		DockCreated:     created,
		DockDuplicate:   duplicate,
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelDockActionReport(ev Event, req ChannelDockActionRequest, result ChannelDockResult) string {
	status := "captured"
	switch {
	case result.DockDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.DockDuplicate:
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
	b.WriteString("## GitClaw Channel Dock Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_dock_status: `%s`\n", status)
	fmt.Fprintf(&b, "- dock_issue: `#%d`\n", result.DockIssueNumber)
	fmt.Fprintf(&b, "- dock_issue_url: `%s`\n", result.DockIssueURL)
	fmt.Fprintf(&b, "- dock_issue_created: `%t`\n", result.DockCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.DockDuplicate)
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
	fmt.Fprintf(&b, "- dock_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.DockID))
	fmt.Fprintf(&b, "- dock_id_auto: `%t`\n", req.AutoDockID)
	fmt.Fprintf(&b, "- target_route_sha256_12: `%s`\n", req.TargetRouteSHA)
	fmt.Fprintf(&b, "- target_route_bytes: `%d`\n", req.TargetRouteBytes)
	fmt.Fprintf(&b, "- reason_sha256_12: `%s`\n", req.ReasonSHA)
	fmt.Fprintf(&b, "- reason_bytes: `%d`\n", req.ReasonBytes)
	fmt.Fprintf(&b, "- reason_lines: `%d`\n", req.ReasonLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- provider_route_change_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- session_route_persistence_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- routebook_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_dock_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_route_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reason_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_dock_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin dock request as a durable GitHub issue, then queued a provider-facing review link back to the mirrored thread. This action records a requested route-continuity change without moving provider sessions, editing routebooks, mutating workflows, calling provider APIs, persisting mode/session routes, or calling a model. The source receipt keeps provider IDs, dock IDs, target routes, reasons, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the dock-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent dock links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate dock issues are suppressed by `dock_id`; duplicate dock-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the dock issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelDockIssueBody(opts ChannelDockOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-dock dock_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.DockID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel dock.\n\n")
	fmt.Fprintf(&b, "- dock_id: %s\n", opts.DockID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- target_route: %s\n", opts.TargetRoute)
	fmt.Fprintf(&b, "- target_route_sha256_12: %s\n", shortDocumentHash(opts.TargetRoute))
	fmt.Fprintf(&b, "- dock_mode: github-issue-dock-request\n")
	fmt.Fprintf(&b, "- provider_route_change_performed: false\n")
	fmt.Fprintf(&b, "- session_route_persistence_performed: false\n")
	fmt.Fprintf(&b, "- routebook_mutation_performed: false\n")
	fmt.Fprintf(&b, "- workflow_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_api_call_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	if strings.TrimSpace(opts.Reason) != "" {
		b.WriteString("## Reason\n\n")
		b.WriteString(strings.TrimSpace(opts.Reason))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for deciding whether this channel thread should dock into another reviewed route. This action did not edit `.gitclaw/channels/routes.yaml`, mutate workflow files, call provider APIs, or persist a route switch.")
	return strings.TrimSpace(b.String())
}

func channelDockActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelDockActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelDockIssueTarget(ev Event, req *ChannelDockActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel dock requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelDockReason(trailing string) string {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	reasonLines := make([]string, 0, len(lines))
	section := ""
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if section == "reason" && len(reasonLines) > 0 {
				reasonLines = append(reasonLines, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "reason:"):
			remainder := strings.TrimSpace(trimmed[len("reason:"):])
			if remainder != "" {
				reasonLines = append(reasonLines, remainder)
			}
			section = "reason"
		case strings.HasPrefix(lower, "notes:"):
			remainder := strings.TrimSpace(trimmed[len("notes:"):])
			if remainder != "" {
				reasonLines = append(reasonLines, remainder)
			}
			section = "reason"
		case strings.HasPrefix(lower, "context:"):
			remainder := strings.TrimSpace(trimmed[len("context:"):])
			if remainder != "" {
				reasonLines = append(reasonLines, remainder)
			}
			section = "reason"
		default:
			reasonLines = append(reasonLines, line)
			section = "reason"
		}
	}
	return strings.TrimSpace(strings.Join(reasonLines, "\n"))
}

func normalizeChannelDockOptions(opts ChannelDockOptions) ChannelDockOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.DockID = cleanChannelDockID(opts.DockID)
	opts.TargetRoute = cleanChannelRouteName(opts.TargetRoute)
	opts.Reason = strings.TrimSpace(opts.Reason)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelDockRoute(cfg Config, opts ChannelDockOptions) (ChannelDockOptions, error) {
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
		Body:      opts.TargetRoute,
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

func validateChannelDockOptions(opts ChannelDockOptions) error {
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
	if opts.DockID == "" {
		return fmt.Errorf("missing dock id")
	}
	if opts.TargetRoute == "" {
		return fmt.Errorf("missing dock target route")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing dock source issue")
	}
	return nil
}

func validateChannelDockActionRequestOptions(opts ChannelDockOptions) error {
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
	if opts.DockID == "" {
		return fmt.Errorf("missing dock id")
	}
	if opts.TargetRoute == "" {
		return fmt.Errorf("missing dock target route")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing dock source issue")
	}
	return nil
}

func findOrCreateChannelDockIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDockOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel dock issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelDockMatches(issue.Body, opts.DockID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelDockIssueTitle(opts), RenderChannelDockIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel dock issue: %w", err)
	}
	return issue, true, false, nil
}

func channelDockIssueTitle(opts ChannelDockOptions) string {
	title := "dock to " + strings.TrimSpace(opts.TargetRoute)
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel dock: " + title
}

func channelDockMatches(body, dockID string) bool {
	return HasChannelDockMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`dock_id="%s"`, escapeMarkerValue(cleanChannelDockID(dockID))))
}

func cleanChannelDockID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelDockID(ev Event, channel, threadID, sourceMessageID, targetRoute, reason string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, targetRoute, reason}, "|")
	return fmt.Sprintf("dock-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelDockNotifyMessageID(ev Event, dockID string) string {
	seed := strings.Join([]string{eventID(ev), dockID}, "|")
	return fmt.Sprintf("gitclaw-channel-dock-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelDockNotificationBody(opts ChannelDockOptions, dockIssueNumber int, dockIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel dock captured.\n\n")
	if dockIssueNumber > 0 {
		fmt.Fprintf(&b, "Dock: #%d\n", dockIssueNumber)
	}
	if dockIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", dockIssueURL)
	}
	fmt.Fprintf(&b, "Target route: %s\n", strings.TrimSpace(opts.TargetRoute))
	b.WriteString("\nReview the requested route-continuity change in the linked GitHub issue. No provider route change has been performed.")
	return strings.TrimSpace(b.String())
}
