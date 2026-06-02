package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelAvailabilityOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	AvailabilityID  string
	State           string
	Author          string
}

type ChannelAvailabilityResult struct {
	Notification       ChannelSendResult
	RouteName          string
	RouteHash          string
	Channel            string
	ThreadHash         string
	MessageHash        string
	NotifyHash         string
	AvailabilityIDHash string
	StateHash          string
	BodyHash           string
	BodyBytes          int
	BodyLines          int
}

type ChannelAvailabilityActionRequest struct {
	Options             ChannelAvailabilityOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoAvailabilityID  bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	AvailabilityIDHash  string
	StateHash           string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelAvailabilityActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelAvailabilityActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelAvailabilityActionFields(fields)
}

func isChannelAvailabilityActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelAvailabilitySubcommand(fields[1]) {
	case "availability", "available", "online", "awake", "here", "beacon", "presence-status", "channel-presence":
		return true
	default:
		return false
	}
}

func BuildChannelAvailabilityActionRequest(ev Event, cfg Config) (ChannelAvailabilityActionRequest, error) {
	fields, _, ok := channelAvailabilityActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelAvailabilityActionRequest{}, fmt.Errorf("missing channel availability command")
	}
	req := ChannelAvailabilityActionRequest{
		Options: ChannelAvailabilityOptions{
			Repo: ev.Repo,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelAvailabilitySubcommand(fields[1]),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--availability-id", "--presence-id", "--status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.AvailabilityID = cleanChannelAvailabilityID(fields[i+1])
			i++
		case "--state", "--availability", "--presence":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.State = cleanChannelAvailabilityState(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelAvailabilityActionRequest{}, fmt.Errorf("unknown channel availability argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			if req.Options.State == "" {
				req.Options.State = cleanChannelAvailabilityState(field)
				continue
			}
			return ChannelAvailabilityActionRequest{}, fmt.Errorf("unexpected channel availability argument %q", field)
		}
	}
	if err := applyChannelAvailabilityIssueTarget(ev, &req); err != nil {
		return ChannelAvailabilityActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.State) == "" {
		req.Options.State = defaultChannelAvailabilityState(req.Subcommand)
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelAvailabilitySourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.AvailabilityID) == "" {
		req.Options.AvailabilityID = autoChannelAvailabilityID(ev, req.Options)
		req.AutoAvailabilityID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelAvailabilityNotifyMessageID(ev, req.Options.AvailabilityID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelAvailabilityOptions(req.Options)
	if err := validateChannelAvailabilityActionRequestOptions(req.Options); err != nil {
		return ChannelAvailabilityActionRequest{}, err
	}
	body := renderChannelAvailabilityNotificationBody(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.AvailabilityIDHash = shortDocumentHash(req.Options.AvailabilityID)
	req.StateHash = shortDocumentHash(req.Options.State)
	req.NotificationBodySHA = shortDocumentHash(body)
	req.NotificationBytes = len(body)
	req.NotificationLines = lineCount(body)
	return req, nil
}

func RunChannelAvailability(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelAvailabilityOptions) (ChannelAvailabilityResult, error) {
	opts = normalizeChannelAvailabilityOptions(opts)
	var err error
	opts, err = applyChannelAvailabilityRoute(cfg, opts)
	if err != nil {
		return ChannelAvailabilityResult{}, err
	}
	if err := validateChannelAvailabilityOptions(opts); err != nil {
		return ChannelAvailabilityResult{}, err
	}
	body := renderChannelAvailabilityNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelAvailabilityResult{}, fmt.Errorf("queue channel availability notification: %w", err)
	}
	return ChannelAvailabilityResult{
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
		AvailabilityIDHash: shortDocumentHash(opts.AvailabilityID),
		StateHash:          shortDocumentHash(opts.State),
		BodyHash:           shortDocumentHash(body),
		BodyBytes:          len(body),
		BodyLines:          lineCount(body),
	}, nil
}

func RenderChannelAvailabilityActionReport(ev Event, req ChannelAvailabilityActionRequest, result ChannelAvailabilityResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
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
	availabilityIDHash := result.AvailabilityIDHash
	if availabilityIDHash == "" {
		availabilityIDHash = req.AvailabilityIDHash
	}
	stateHash := result.StateHash
	if stateHash == "" {
		stateHash = req.StateHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	bodyBytes := result.BodyBytes
	if bodyBytes == 0 {
		bodyBytes = req.NotificationBytes
	}
	bodyLines := result.BodyLines
	if bodyLines == 0 {
		bodyLines = req.NotificationLines
	}

	var b strings.Builder
	b.WriteString("## GitClaw Channel Availability Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_availability_status: `%s`\n", status)
	fmt.Fprintf(&b, "- availability_snapshot_mode: `%s`\n", "provider-facing-presence-card")
	fmt.Fprintf(&b, "- availability_state: `%s`\n", req.Options.State)
	fmt.Fprintf(&b, "- availability_state_sha256_12: `%s`\n", stateHash)
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
	fmt.Fprintf(&b, "- source_message_id_auto: `%t`\n", req.AutoSourceMessageID)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- availability_id_sha256_12: `%s`\n", noneIfEmpty(availabilityIDHash))
	fmt.Fprintf(&b, "- availability_id_auto: `%t`\n", req.AutoAvailabilityID)
	fmt.Fprintf(&b, "- bridge_runtime: `%s`\n", "github-actions-workflow-dispatch")
	fmt.Fprintf(&b, "- canonical_surface: `%s`\n", "github-issue-thread")
	fmt.Fprintf(&b, "- inbound_strategy: `%s`\n", "channel-ingest workflow_dispatch")
	fmt.Fprintf(&b, "- outbound_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- session_rows_used_as_liveness: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_socket_probe_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", bodyBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", bodyLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_availability_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_availability_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing availability card on the canonical channel issue. The card is a GitHub-native presence signal: it proves the Actions-backed bridge can receive a channel command and place a reply in the outbox, while avoiding provider socket probes, provider API calls, session-store liveness guesses, model calls, repository mutations, workflow edits, raw channel bodies, prompts, tool outputs, and credentials.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read availability cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent availability cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate availability notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/channels status`, `/channels model`, `/channels tools`, or `/channels session-search` when the channel needs deeper context\n")
	return strings.TrimSpace(b.String())
}

func renderChannelAvailabilityNotificationBody(opts ChannelAvailabilityOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel availability\n\n")
	fmt.Fprintf(&b, "State: %s\n", opts.State)
	b.WriteString("Bridge runtime: GitHub Actions workflow_dispatch\n")
	b.WriteString("Canonical surface: GitHub issue thread\n")
	b.WriteString("Inbound path: channel-ingest workflow\n")
	b.WriteString("Outbound path: channel-outbox + channel-delivery\n")
	b.WriteString("Provider socket health: not probed by this action\n")
	b.WriteString("Session rows used as liveness: false\n")
	b.WriteString("Provider API call: not performed by this action\n")
	b.WriteString("Model call: not performed by this action\n")
	b.WriteString("Repository mutation: not performed by this action\n")
	b.WriteString("Workflow mutation: not performed by this action\n")
	b.WriteString("\nSafe follow-up commands:\n")
	b.WriteString("- @gitclaw /channels status\n")
	b.WriteString("- @gitclaw /channels model --message-id <id>\n")
	b.WriteString("- @gitclaw /channels tools --message-id <id>\n")
	b.WriteString("- @gitclaw /channels session-search <query> --message-id <id>\n")
	b.WriteString("- @gitclaw /channels reminder --message-id <id>\n")
	return strings.TrimSpace(b.String())
}

func channelAvailabilityActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelAvailabilityActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelAvailabilityIssueTarget(ev Event, req *ChannelAvailabilityActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel availability requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelAvailabilityOptions(opts ChannelAvailabilityOptions) ChannelAvailabilityOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.AvailabilityID = cleanChannelAvailabilityID(opts.AvailabilityID)
	opts.State = cleanChannelAvailabilityState(opts.State)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelAvailabilityRoute(cfg Config, opts ChannelAvailabilityOptions) (ChannelAvailabilityOptions, error) {
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
		Body:      "GitClaw channel availability.",
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

func validateChannelAvailabilityOptions(opts ChannelAvailabilityOptions) error {
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
	if opts.AvailabilityID == "" {
		return fmt.Errorf("missing availability id")
	}
	if opts.State == "" {
		return fmt.Errorf("missing availability state")
	}
	return nil
}

func validateChannelAvailabilityActionRequestOptions(opts ChannelAvailabilityOptions) error {
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
	if opts.AvailabilityID == "" {
		return fmt.Errorf("missing availability id")
	}
	if opts.State == "" {
		return fmt.Errorf("missing availability state")
	}
	return nil
}

func cleanChannelAvailabilitySubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelAvailabilityID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelAvailabilityState(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" {
		return ""
	}
	var b strings.Builder
	dash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		if b.Len() >= 40 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func defaultChannelAvailabilityState(subcommand string) string {
	switch cleanChannelAvailabilitySubcommand(subcommand) {
	case "online", "awake", "here", "available", "beacon":
		return "available"
	default:
		return "available"
	}
}

func autoChannelAvailabilitySourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-availability-source-%s", eventID(ev))
}

func autoChannelAvailabilityID(ev Event, opts ChannelAvailabilityOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("availability-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelAvailabilityNotifyMessageID(ev Event, availabilityID string) string {
	seed := strings.Join([]string{eventID(ev), availabilityID}, "|")
	return fmt.Sprintf("gitclaw-channel-availability-%s-%s", eventID(ev), shortDocumentHash(seed))
}
