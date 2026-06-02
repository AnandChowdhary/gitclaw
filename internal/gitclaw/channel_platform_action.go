package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPlatformOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	Provider        string
	AdapterState    string
	Reason          string
	Home            string
	Author          string
}

type ChannelPlatformResult struct {
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
	ProviderHash    string
	StateHash       string
	ReasonHash      string
	HomeHash        string
	BodyHash        string
	IngestPresent   bool
	GatewayPresent  bool
	OutboxPresent   bool
	DispatchRuntime bool
}

type ChannelPlatformActionRequest struct {
	Options             ChannelPlatformOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ProviderHash        string
	StateHash           string
	ReasonHash          string
	ReasonBytes         int
	ReasonLines         int
	HomeHash            string
	NotificationBodySHA string
}

func IsChannelPlatformActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelPlatformActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelPlatformActionFields(fields)
}

func isChannelPlatformActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "platform", "platforms", "adapter", "adapter-status", "gateway-status", "bridge-status":
		return true
	default:
		return false
	}
}

func BuildChannelPlatformActionRequest(ev Event, cfg Config) (ChannelPlatformActionRequest, error) {
	fields, trailing, ok := channelPlatformActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPlatformActionRequest{}, fmt.Errorf("missing channel platform command")
	}
	req := ChannelPlatformActionRequest{
		Options: ChannelPlatformOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--provider", "--platform", "--adapter":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Provider = fields[i+1]
			i++
		case "--state", "--status", "--adapter-state":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.AdapterState = fields[i+1]
			i++
		case "--reason":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("--reason requires a value")
			}
			req.Options.Reason = fields[i+1]
			i++
		case "--home", "--home-channel", "--set-home":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Home = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPlatformActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPlatformActionRequest{}, fmt.Errorf("unknown channel platform argument %q", field)
			}
			if req.Options.Provider == "" {
				req.Options.Provider = field
				continue
			}
			return ChannelPlatformActionRequest{}, fmt.Errorf("unexpected channel platform argument %q", field)
		}
	}
	if err := applyChannelPlatformIssueTarget(ev, &req); err != nil {
		return ChannelPlatformActionRequest{}, err
	}
	trailingReason := parseChannelPlatformTrailingReason(trailing)
	if strings.TrimSpace(req.Options.Reason) == "" {
		req.Options.Reason = trailingReason
	}
	if strings.TrimSpace(req.Options.Provider) == "" {
		req.Options.Provider = req.Options.Channel
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelPlatformSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPlatformNotifyMessageID(ev, req.Options.Provider)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPlatformOptions(req.Options)
	if err := validateChannelPlatformActionRequestOptions(req.Options); err != nil {
		return ChannelPlatformActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ProviderHash = shortDocumentHash(req.Options.Provider)
	req.StateHash = shortDocumentHash(req.Options.AdapterState)
	req.ReasonHash = optionalChannelPlatformHash(req.Options.Reason)
	req.ReasonBytes = len(req.Options.Reason)
	req.ReasonLines = lineCount(req.Options.Reason)
	req.HomeHash = optionalChannelPlatformHash(req.Options.Home)
	req.NotificationBodySHA = shortDocumentHash(renderChannelPlatformNotificationBody(req.Options, inspectChannelSurface(cfg.Workdir)))
	return req, nil
}

func RunChannelPlatform(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPlatformOptions) (ChannelPlatformResult, error) {
	opts = normalizeChannelPlatformOptions(opts)
	var err error
	opts, err = applyChannelPlatformRoute(cfg, opts)
	if err != nil {
		return ChannelPlatformResult{}, err
	}
	if err := validateChannelPlatformOptions(opts); err != nil {
		return ChannelPlatformResult{}, err
	}
	surface := inspectChannelSurface(cfg.Workdir)
	body := renderChannelPlatformNotificationBody(opts, surface)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelPlatformResult{}, fmt.Errorf("queue channel platform notification: %w", err)
	}
	return ChannelPlatformResult{
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
		ProviderHash:    shortDocumentHash(opts.Provider),
		StateHash:       shortDocumentHash(opts.AdapterState),
		ReasonHash:      optionalChannelPlatformHash(opts.Reason),
		HomeHash:        optionalChannelPlatformHash(opts.Home),
		BodyHash:        shortDocumentHash(body),
		IngestPresent:   surface.IngestWorkflow.Present,
		GatewayPresent:  surface.GatewayWorkflow.Present,
		OutboxPresent:   surface.OutboxWorkflow.Present,
		DispatchRuntime: surface.IngestWorkflow.WorkflowDispatch && surface.OutboxWorkflow.WorkflowDispatch,
	}, nil
}

func RenderChannelPlatformActionReport(ev Event, req ChannelPlatformActionRequest, result ChannelPlatformResult) string {
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
	providerHash := result.ProviderHash
	if providerHash == "" {
		providerHash = req.ProviderHash
	}
	stateHash := result.StateHash
	if stateHash == "" {
		stateHash = req.StateHash
	}
	reasonHash := result.ReasonHash
	if reasonHash == "" {
		reasonHash = req.ReasonHash
	}
	homeHash := result.HomeHash
	if homeHash == "" {
		homeHash = req.HomeHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Platform Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_platform_status: `%s`\n", status)
	fmt.Fprintf(&b, "- platform_snapshot_mode: `%s`\n", "provider-facing-status")
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
	fmt.Fprintf(&b, "- provider_sha256_12: `%s`\n", noneIfEmpty(providerHash))
	fmt.Fprintf(&b, "- adapter_state_sha256_12: `%s`\n", noneIfEmpty(stateHash))
	fmt.Fprintf(&b, "- reason_sha256_12: `%s`\n", noneIfEmpty(reasonHash))
	fmt.Fprintf(&b, "- reason_bytes: `%d`\n", req.ReasonBytes)
	fmt.Fprintf(&b, "- reason_lines: `%d`\n", req.ReasonLines)
	fmt.Fprintf(&b, "- home_sha256_12: `%s`\n", noneIfEmpty(homeHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- ingest_workflow_present: `%t`\n", result.IngestPresent)
	fmt.Fprintf(&b, "- gateway_workflow_present: `%t`\n", result.GatewayPresent)
	fmt.Fprintf(&b, "- outbox_workflow_present: `%t`\n", result.OutboxPresent)
	fmt.Fprintf(&b, "- workflow_dispatch_runtime: `%t`\n", result.DispatchRuntime)
	fmt.Fprintf(&b, "- platform_pause_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- platform_resume_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- adapter_state_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- breaker_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- home_channel_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reason_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_home_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_platform_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing platform status snapshot on the canonical channel issue. This is the GitHub-native version of a messaging-platform status command: it reports the reviewed bridge shape and adapter state claim, but it does not pause or resume adapters, mutate breaker state, change a home channel, call provider APIs, start a gateway, or call a model. The source receipt keeps reasons, home selectors, thread ids, message ids, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the platform-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent platform-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate platform-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `gitclaw channel-gateway --channel <provider> --account-id <account> --renew` for the reviewed workflow-dispatch gateway lease path\n")
	return strings.TrimSpace(b.String())
}

func channelPlatformActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPlatformActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPlatformIssueTarget(ev Event, req *ChannelPlatformActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel platform requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelPlatformTrailingReason(trailing string) string {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	var reasonLines []string
	section := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
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
		default:
			if section == "reason" {
				reasonLines = append(reasonLines, line)
			} else {
				reasonLines = append(reasonLines, line)
				section = "reason"
			}
		}
	}
	return strings.TrimSpace(strings.Join(reasonLines, "\n"))
}

func normalizeChannelPlatformOptions(opts ChannelPlatformOptions) ChannelPlatformOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.Provider = cleanChannelProviderName(opts.Provider)
	if opts.Provider == "" {
		opts.Provider = opts.Channel
	}
	opts.AdapterState = cleanChannelReaction(opts.AdapterState)
	if opts.AdapterState == "" {
		opts.AdapterState = "unknown"
	}
	opts.Reason = strings.TrimSpace(opts.Reason)
	opts.Home = strings.TrimSpace(opts.Home)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelPlatformRoute(cfg Config, opts ChannelPlatformOptions) (ChannelPlatformOptions, error) {
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
		Body:      opts.Provider,
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

func validateChannelPlatformOptions(opts ChannelPlatformOptions) error {
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
	if opts.Provider == "" {
		return fmt.Errorf("missing platform provider")
	}
	if opts.AdapterState == "" {
		return fmt.Errorf("missing adapter state")
	}
	return nil
}

func validateChannelPlatformActionRequestOptions(opts ChannelPlatformOptions) error {
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
	if opts.Provider == "" {
		return fmt.Errorf("missing platform provider")
	}
	if opts.AdapterState == "" {
		return fmt.Errorf("missing adapter state")
	}
	return nil
}

func autoChannelPlatformSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-platform-source-%s", eventID(ev))
}

func autoChannelPlatformNotifyMessageID(ev Event, provider string) string {
	seed := strings.Join([]string{eventID(ev), provider}, "|")
	return fmt.Sprintf("gitclaw-channel-platform-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func optionalChannelPlatformHash(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return shortDocumentHash(value)
}

func renderChannelPlatformNotificationBody(opts ChannelPlatformOptions, surface channelSurface) string {
	info, ok := lookupChannelProvider(opts.Provider)
	var b strings.Builder
	b.WriteString("GitClaw channel platform status.\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", opts.Provider)
	fmt.Fprintf(&b, "Adapter state: %s\n", opts.AdapterState)
	b.WriteString("Gateway runtime: GitHub Actions workflow_dispatch\n")
	b.WriteString("State storage: gitclaw:channel-state issue\n")
	b.WriteString("Outbox: gitclaw channel-outbox + channel-delivery\n")
	if ok {
		fmt.Fprintf(&b, "Ingress: %s\n", info.IngressStrategy)
		fmt.Fprintf(&b, "Outbound: %s\n", info.OutboundDelivery)
	} else {
		b.WriteString("Ingress: custom provider contract\n")
		b.WriteString("Outbound: external sender then channel-delivery receipt\n")
	}
	fmt.Fprintf(&b, "Workflow surface: ingest=%t gateway=%t outbox=%t\n", surface.IngestWorkflow.Present, surface.GatewayWorkflow.Present, surface.OutboxWorkflow.Present)
	b.WriteString("\nLive adapter inspection: not performed by this action.\n")
	b.WriteString("Pause/resume: not performed by this action.\n")
	b.WriteString("Home channel: not changed by this action.")
	return strings.TrimSpace(b.String())
}
