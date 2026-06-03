package gitclaw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ChannelBrowserStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelBrowserStatusResult struct {
	Notification          ChannelSendResult
	RouteName             string
	RouteHash             string
	Channel               string
	ThreadHash            string
	MessageHash           string
	NotifyHash            string
	StatusIDHash          string
	BodyHash              string
	MCPSpecsScanned       int
	BrowserMCPSpecs       int
	GatewayWorkflow       bool
	OutboxWorkflow        bool
	BrowserBridgeReviewed bool
}

type ChannelBrowserStatusActionRequest struct {
	Options             ChannelBrowserStatusOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoStatusID        bool
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	StatusIDHash        string
	MCPSpecsScanned     int
	BrowserMCPSpecs     int
	GatewayWorkflow     bool
	OutboxWorkflow      bool
	NotificationBodySHA string
}

type channelBrowserStatusSurface struct {
	MCPSpecsScanned int
	BrowserMCPSpecs int
	GatewayWorkflow bool
	OutboxWorkflow  bool
}

func IsChannelBrowserStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelBrowserStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelBrowserStatusActionFields(fields)
}

func isChannelBrowserStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "browser", "browsers", "browser-status", "web-status", "web", "playwright-status", "cdp-status", "browser-tools":
		return true
	default:
		return false
	}
}

func BuildChannelBrowserStatusActionRequest(ev Event, cfg Config) (ChannelBrowserStatusActionRequest, error) {
	fields, _, ok := channelBrowserStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBrowserStatusActionRequest{}, fmt.Errorf("missing channel browser status command")
	}
	req := ChannelBrowserStatusActionRequest{
		Options: ChannelBrowserStatusOptions{
			Repo: ev.Repo,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--browser-status-id", "--browser-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelBrowserStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBrowserStatusActionRequest{}, fmt.Errorf("unknown channel browser status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBrowserStatusActionRequest{}, fmt.Errorf("unexpected channel browser status argument %q", field)
		}
	}
	if err := applyChannelBrowserStatusIssueTarget(ev, &req); err != nil {
		return ChannelBrowserStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBrowserStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelBrowserStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBrowserStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBrowserStatusOptions(req.Options)
	if err := validateChannelBrowserStatusActionRequestOptions(req.Options); err != nil {
		return ChannelBrowserStatusActionRequest{}, err
	}
	surface := inspectChannelBrowserStatusSurface(cfg.Workdir)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.MCPSpecsScanned = surface.MCPSpecsScanned
	req.BrowserMCPSpecs = surface.BrowserMCPSpecs
	req.GatewayWorkflow = surface.GatewayWorkflow
	req.OutboxWorkflow = surface.OutboxWorkflow
	req.NotificationBodySHA = shortDocumentHash(renderChannelBrowserStatusNotificationBody(req.Options, surface))
	return req, nil
}

func RunChannelBrowserStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBrowserStatusOptions) (ChannelBrowserStatusResult, error) {
	opts = normalizeChannelBrowserStatusOptions(opts)
	var err error
	opts, err = applyChannelBrowserStatusRoute(cfg, opts)
	if err != nil {
		return ChannelBrowserStatusResult{}, err
	}
	if err := validateChannelBrowserStatusOptions(opts); err != nil {
		return ChannelBrowserStatusResult{}, err
	}
	surface := inspectChannelBrowserStatusSurface(cfg.Workdir)
	body := renderChannelBrowserStatusNotificationBody(opts, surface)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelBrowserStatusResult{}, fmt.Errorf("queue channel browser status notification: %w", err)
	}
	return ChannelBrowserStatusResult{
		Notification:          notification,
		RouteName:             opts.Route,
		RouteHash:             channelRouteHash(opts.Route),
		Channel:               opts.Channel,
		ThreadHash:            shortDocumentHash(opts.ThreadID),
		MessageHash:           shortDocumentHash(opts.SourceMessageID),
		NotifyHash:            shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash:          shortDocumentHash(opts.StatusID),
		BodyHash:              shortDocumentHash(body),
		MCPSpecsScanned:       surface.MCPSpecsScanned,
		BrowserMCPSpecs:       surface.BrowserMCPSpecs,
		GatewayWorkflow:       surface.GatewayWorkflow,
		OutboxWorkflow:        surface.OutboxWorkflow,
		BrowserBridgeReviewed: surface.BrowserMCPSpecs > 0,
	}, nil
}

func RenderChannelBrowserStatusActionReport(ev Event, req ChannelBrowserStatusActionRequest, result ChannelBrowserStatusResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	statusIDHash := firstNonEmpty(result.StatusIDHash, req.StatusIDHash)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	mcpSpecs := result.MCPSpecsScanned
	if mcpSpecs == 0 {
		mcpSpecs = req.MCPSpecsScanned
	}
	browserSpecs := result.BrowserMCPSpecs
	if browserSpecs == 0 {
		browserSpecs = req.BrowserMCPSpecs
	}
	gatewayPresent := result.GatewayWorkflow || req.GatewayWorkflow
	outboxPresent := result.OutboxWorkflow || req.OutboxWorkflow
	browserBridgeReviewed := result.BrowserBridgeReviewed || browserSpecs > 0

	var b strings.Builder
	b.WriteString("## GitClaw Channel Browser Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_browser_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- browser_status_mode: `%s`\n", "provider-facing-browser-readiness")
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
	fmt.Fprintf(&b, "- browser_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- browser_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- mcp_specs_scanned: `%d`\n", mcpSpecs)
	fmt.Fprintf(&b, "- browser_mcp_specs: `%d`\n", browserSpecs)
	fmt.Fprintf(&b, "- browser_bridge_reviewed: `%t`\n", browserBridgeReviewed)
	fmt.Fprintf(&b, "- channel_gateway_workflow_present: `%t`\n", gatewayPresent)
	fmt.Fprintf(&b, "- channel_outbox_workflow_present: `%t`\n", outboxPresent)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only-status-card")
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- browser_session_opened: `%t`\n", false)
	fmt.Fprintf(&b, "- browser_navigation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- browser_screenshot_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- browser_mcp_server_launch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_browser_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mcp_spec_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_browser_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing browser readiness card on the canonical channel issue. This is a channel-native way to ask whether browser automation is reviewed/configured, while keeping browser sessions, navigation, screenshots, browser MCP server launches, model calls, provider APIs, workflow edits, and repository mutations out of this action. The source receipt keeps thread ids, message ids, status ids, MCP spec bodies, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read browser-status updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent browser-status updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate browser-status updates are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal follow-up comments on the same GitHub issue still take the GitHub Models path and must expose model, tool, skill, prompt-context, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func channelBrowserStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBrowserStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBrowserStatusIssueTarget(ev Event, req *ChannelBrowserStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel browser status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBrowserStatusOptions(opts ChannelBrowserStatusOptions) ChannelBrowserStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelBrowserStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBrowserStatusRoute(cfg Config, opts ChannelBrowserStatusOptions) (ChannelBrowserStatusOptions, error) {
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
		Body:      "GitClaw channel browser status.",
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

func validateChannelBrowserStatusOptions(opts ChannelBrowserStatusOptions) error {
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
	if opts.StatusID == "" {
		return fmt.Errorf("missing browser status id")
	}
	return nil
}

func validateChannelBrowserStatusActionRequestOptions(opts ChannelBrowserStatusOptions) error {
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
	if opts.StatusID == "" {
		return fmt.Errorf("missing browser status id")
	}
	return nil
}

func cleanChannelBrowserStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBrowserStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-browser-source-%s", eventID(ev))
}

func autoChannelBrowserStatusID(ev Event, opts ChannelBrowserStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("browser-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBrowserStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-browser-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func inspectChannelBrowserStatusSurface(root string) channelBrowserStatusSurface {
	var surface channelBrowserStatusSurface
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	mcpDir := filepath.Join(root, ".gitclaw", "mcp")
	if entries, err := os.ReadDir(mcpDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			surface.MCPSpecsScanned++
			if strings.Contains(name, "browser") || strings.Contains(name, "playwright") || strings.Contains(name, "cdp") || strings.Contains(name, "web") {
				surface.BrowserMCPSpecs++
			}
		}
	}
	surface.GatewayWorkflow = fileExists(filepath.Join(root, ".github", "workflows", "gitclaw-channel-gateway.yml"))
	surface.OutboxWorkflow = fileExists(filepath.Join(root, ".github", "workflows", "gitclaw-channel-outbox.yml"))
	return surface
}

func renderChannelBrowserStatusNotificationBody(opts ChannelBrowserStatusOptions, surface channelBrowserStatusSurface) string {
	bridge := "not configured"
	if surface.BrowserMCPSpecs > 0 {
		bridge = "reviewed MCP spec present"
	}
	var b strings.Builder
	b.WriteString("GitClaw channel browser status.\n\n")
	fmt.Fprintf(&b, "Browser bridge: %s\n", bridge)
	fmt.Fprintf(&b, "Browser MCP specs: %d\n", surface.BrowserMCPSpecs)
	fmt.Fprintf(&b, "MCP specs scanned: %d\n", surface.MCPSpecsScanned)
	fmt.Fprintf(&b, "Channel gateway workflow: %s\n", presentWord(surface.GatewayWorkflow))
	fmt.Fprintf(&b, "Channel outbox workflow: %s\n", presentWord(surface.OutboxWorkflow))
	b.WriteString("Run mode: read-only status card\n")
	b.WriteString("\nBrowser session opened: not performed by this action.\n")
	b.WriteString("Browser navigation: not performed by this action.\n")
	b.WriteString("Browser screenshot: not performed by this action.\n")
	b.WriteString("Browser MCP server launch: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func presentWord(value bool) string {
	if value {
		return "present"
	}
	return "missing"
}
