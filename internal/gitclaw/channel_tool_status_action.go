package gitclaw

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type ChannelToolStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	SourceMessageID string
	NotifyMessageID string
	StatusID        string
	Author          string
}

type ChannelToolStatusResult struct {
	Notification                 ChannelSendResult
	RouteName                    string
	RouteHash                    string
	Channel                      string
	ThreadHash                   string
	MessageHash                  string
	NotifyHash                   string
	StatusIDHash                 string
	BodyHash                     string
	AvailableTools               int
	EnabledTools                 int
	DisabledTools                int
	AllowlistBlockedTools        int
	ReadOnlyContracts            int
	MetadataOnlyContracts        int
	MutatingContracts            int
	ActiveToolOutputs            int
	KnownToolOutputs             int
	UnknownToolOutputs           int
	ToolsetsScanned              int
	MCPSpecsScanned              int
	SnapshotEntries              int
	CatalogEntries               int
	PromptVisibleEntries         int
	ToolGuidanceFiles            int
	EnabledToolNamesHash         string
	PromptVisibleToolHash        string
	ToolOutputManifestHash       string
	ToolSnapshotHash             string
	ToolValidationStatus         string
	ToolValidationErrors         int
	ToolValidationWarnings       int
	ToolRiskStatus               string
	ToolRiskFindings             int
	HighRiskFindings             int
	WarningRiskFindings          int
	DynamicMCPDiscoveryAllowed   bool
	MCPServerLaunchAllowed       bool
	ToolsetActivationSupported   bool
	ModelCallableStructuredTools bool
}

type ChannelToolStatusActionRequest struct {
	Options                      ChannelToolStatusOptions
	Command                      string
	Subcommand                   string
	AutoSourceMessageID          bool
	AutoNotifyMessageID          bool
	AutoStatusID                 bool
	TargetFromIssue              bool
	RequestedRouteHash           string
	RequestedThreadHash          string
	RequestedMsgHash             string
	NotifyMessageHash            string
	StatusIDHash                 string
	AvailableTools               int
	EnabledTools                 int
	DisabledTools                int
	AllowlistBlockedTools        int
	ReadOnlyContracts            int
	MetadataOnlyContracts        int
	MutatingContracts            int
	ActiveToolOutputs            int
	KnownToolOutputs             int
	UnknownToolOutputs           int
	ToolsetsScanned              int
	MCPSpecsScanned              int
	SnapshotEntries              int
	CatalogEntries               int
	PromptVisibleEntries         int
	ToolGuidanceFiles            int
	EnabledToolNamesHash         string
	PromptVisibleToolHash        string
	ToolOutputManifestHash       string
	ToolSnapshotHash             string
	ToolValidationStatus         string
	ToolValidationErrors         int
	ToolValidationWarnings       int
	ToolRiskStatus               string
	ToolRiskFindings             int
	HighRiskFindings             int
	WarningRiskFindings          int
	DynamicMCPDiscoveryAllowed   bool
	MCPServerLaunchAllowed       bool
	ToolsetActivationSupported   bool
	ModelCallableStructuredTools bool
	NotificationBodySHA          string
}

type channelToolStatusSnapshot struct {
	AvailableTools               int
	EnabledTools                 int
	DisabledTools                int
	AllowlistBlockedTools        int
	ReadOnlyContracts            int
	MetadataOnlyContracts        int
	MutatingContracts            int
	ActiveToolOutputs            int
	KnownToolOutputs             int
	UnknownToolOutputs           int
	ToolsetsScanned              int
	MCPSpecsScanned              int
	SnapshotEntries              int
	CatalogEntries               int
	PromptVisibleEntries         int
	ToolGuidanceFiles            int
	EnabledToolNames             []string
	PromptVisibleToolNames       []string
	ToolOutputManifest           string
	ToolSnapshotHash             string
	ToolValidationStatus         string
	ToolValidationErrors         int
	ToolValidationWarnings       int
	ToolRiskStatus               string
	ToolRiskFindings             int
	HighRiskFindings             int
	WarningRiskFindings          int
	DynamicMCPDiscoveryAllowed   bool
	MCPServerLaunchAllowed       bool
	ToolsetActivationSupported   bool
	ModelCallableStructuredTools bool
}

func IsChannelToolStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelToolStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelToolStatusActionFields(fields)
}

func isChannelToolStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "tools", "tool-status", "tools-status", "tool-list", "tools-list", "tool-capabilities", "tool-capability-status":
		return true
	default:
		return false
	}
}

func BuildChannelToolStatusActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolStatusActionRequest, error) {
	fields, _, ok := channelToolStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolStatusActionRequest{}, fmt.Errorf("missing channel tool status command")
	}
	req := ChannelToolStatusActionRequest{
		Options: ChannelToolStatusOptions{
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
				return ChannelToolStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--status-id", "--tool-status-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = cleanChannelToolStatusID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolStatusActionRequest{}, fmt.Errorf("unknown channel tool status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelToolStatusActionRequest{}, fmt.Errorf("unexpected channel tool status argument %q", field)
		}
	}
	if err := applyChannelToolStatusIssueTarget(ev, &req); err != nil {
		return ChannelToolStatusActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelToolStatusSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StatusID) == "" {
		req.Options.StatusID = autoChannelToolStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolStatusNotifyMessageID(ev, req.Options.StatusID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolStatusOptions(req.Options)
	if err := validateChannelToolStatusActionRequestOptions(req.Options); err != nil {
		return ChannelToolStatusActionRequest{}, err
	}
	snapshot := buildChannelToolStatusSnapshot(cfg, repoContext)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.AvailableTools = snapshot.AvailableTools
	req.EnabledTools = snapshot.EnabledTools
	req.DisabledTools = snapshot.DisabledTools
	req.AllowlistBlockedTools = snapshot.AllowlistBlockedTools
	req.ReadOnlyContracts = snapshot.ReadOnlyContracts
	req.MetadataOnlyContracts = snapshot.MetadataOnlyContracts
	req.MutatingContracts = snapshot.MutatingContracts
	req.ActiveToolOutputs = snapshot.ActiveToolOutputs
	req.KnownToolOutputs = snapshot.KnownToolOutputs
	req.UnknownToolOutputs = snapshot.UnknownToolOutputs
	req.ToolsetsScanned = snapshot.ToolsetsScanned
	req.MCPSpecsScanned = snapshot.MCPSpecsScanned
	req.SnapshotEntries = snapshot.SnapshotEntries
	req.CatalogEntries = snapshot.CatalogEntries
	req.PromptVisibleEntries = snapshot.PromptVisibleEntries
	req.ToolGuidanceFiles = snapshot.ToolGuidanceFiles
	req.EnabledToolNamesHash = hashStringList(snapshot.EnabledToolNames)
	req.PromptVisibleToolHash = hashStringList(snapshot.PromptVisibleToolNames)
	req.ToolOutputManifestHash = hashStringOrNone(snapshot.ToolOutputManifest)
	req.ToolSnapshotHash = snapshot.ToolSnapshotHash
	req.ToolValidationStatus = snapshot.ToolValidationStatus
	req.ToolValidationErrors = snapshot.ToolValidationErrors
	req.ToolValidationWarnings = snapshot.ToolValidationWarnings
	req.ToolRiskStatus = snapshot.ToolRiskStatus
	req.ToolRiskFindings = snapshot.ToolRiskFindings
	req.HighRiskFindings = snapshot.HighRiskFindings
	req.WarningRiskFindings = snapshot.WarningRiskFindings
	req.DynamicMCPDiscoveryAllowed = snapshot.DynamicMCPDiscoveryAllowed
	req.MCPServerLaunchAllowed = snapshot.MCPServerLaunchAllowed
	req.ToolsetActivationSupported = snapshot.ToolsetActivationSupported
	req.ModelCallableStructuredTools = snapshot.ModelCallableStructuredTools
	req.NotificationBodySHA = shortDocumentHash(renderChannelToolStatusNotificationBody(req.Options, cfg, repoContext))
	return req, nil
}

func RunChannelToolStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolStatusOptions, repoContext RepoContext) (ChannelToolStatusResult, error) {
	opts = normalizeChannelToolStatusOptions(opts)
	var err error
	opts, err = applyChannelToolStatusRoute(cfg, opts, repoContext)
	if err != nil {
		return ChannelToolStatusResult{}, err
	}
	if err := validateChannelToolStatusOptions(opts); err != nil {
		return ChannelToolStatusResult{}, err
	}
	body := renderChannelToolStatusNotificationBody(opts, cfg, repoContext)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelToolStatusResult{}, fmt.Errorf("queue channel tool status notification: %w", err)
	}
	snapshot := buildChannelToolStatusSnapshot(cfg, repoContext)
	return ChannelToolStatusResult{
		Notification:                 notification,
		RouteName:                    opts.Route,
		RouteHash:                    channelRouteHash(opts.Route),
		Channel:                      opts.Channel,
		ThreadHash:                   shortDocumentHash(opts.ThreadID),
		MessageHash:                  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:                   shortDocumentHash(opts.NotifyMessageID),
		StatusIDHash:                 shortDocumentHash(opts.StatusID),
		BodyHash:                     shortDocumentHash(body),
		AvailableTools:               snapshot.AvailableTools,
		EnabledTools:                 snapshot.EnabledTools,
		DisabledTools:                snapshot.DisabledTools,
		AllowlistBlockedTools:        snapshot.AllowlistBlockedTools,
		ReadOnlyContracts:            snapshot.ReadOnlyContracts,
		MetadataOnlyContracts:        snapshot.MetadataOnlyContracts,
		MutatingContracts:            snapshot.MutatingContracts,
		ActiveToolOutputs:            snapshot.ActiveToolOutputs,
		KnownToolOutputs:             snapshot.KnownToolOutputs,
		UnknownToolOutputs:           snapshot.UnknownToolOutputs,
		ToolsetsScanned:              snapshot.ToolsetsScanned,
		MCPSpecsScanned:              snapshot.MCPSpecsScanned,
		SnapshotEntries:              snapshot.SnapshotEntries,
		CatalogEntries:               snapshot.CatalogEntries,
		PromptVisibleEntries:         snapshot.PromptVisibleEntries,
		ToolGuidanceFiles:            snapshot.ToolGuidanceFiles,
		EnabledToolNamesHash:         hashStringList(snapshot.EnabledToolNames),
		PromptVisibleToolHash:        hashStringList(snapshot.PromptVisibleToolNames),
		ToolOutputManifestHash:       hashStringOrNone(snapshot.ToolOutputManifest),
		ToolSnapshotHash:             snapshot.ToolSnapshotHash,
		ToolValidationStatus:         snapshot.ToolValidationStatus,
		ToolValidationErrors:         snapshot.ToolValidationErrors,
		ToolValidationWarnings:       snapshot.ToolValidationWarnings,
		ToolRiskStatus:               snapshot.ToolRiskStatus,
		ToolRiskFindings:             snapshot.ToolRiskFindings,
		HighRiskFindings:             snapshot.HighRiskFindings,
		WarningRiskFindings:          snapshot.WarningRiskFindings,
		DynamicMCPDiscoveryAllowed:   snapshot.DynamicMCPDiscoveryAllowed,
		MCPServerLaunchAllowed:       snapshot.MCPServerLaunchAllowed,
		ToolsetActivationSupported:   snapshot.ToolsetActivationSupported,
		ModelCallableStructuredTools: snapshot.ModelCallableStructuredTools,
	}, nil
}

func RenderChannelToolStatusActionReport(ev Event, req ChannelToolStatusActionRequest, result ChannelToolStatusResult) string {
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
	statusIDHash := result.StatusIDHash
	if statusIDHash == "" {
		statusIDHash = req.StatusIDHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Tool Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_snapshot_mode: `%s`\n", "provider-facing-tool-status")
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
	fmt.Fprintf(&b, "- tool_status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- tool_status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", nonzeroOrReq(result.AvailableTools, req.AvailableTools))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", nonzeroOrReq(result.EnabledTools, req.EnabledTools))
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", result.DisabledTools)
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", result.AllowlistBlockedTools)
	fmt.Fprintf(&b, "- read_only_contracts: `%d`\n", result.ReadOnlyContracts)
	fmt.Fprintf(&b, "- metadata_only_contracts: `%d`\n", result.MetadataOnlyContracts)
	fmt.Fprintf(&b, "- mutating_contracts: `%d`\n", result.MutatingContracts)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", result.ActiveToolOutputs)
	fmt.Fprintf(&b, "- known_tool_outputs: `%d`\n", result.KnownToolOutputs)
	fmt.Fprintf(&b, "- unknown_tool_outputs: `%d`\n", result.UnknownToolOutputs)
	fmt.Fprintf(&b, "- toolsets_scanned: `%d`\n", result.ToolsetsScanned)
	fmt.Fprintf(&b, "- mcp_specs_scanned: `%d`\n", result.MCPSpecsScanned)
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", result.SnapshotEntries)
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", result.CatalogEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", result.PromptVisibleEntries)
	fmt.Fprintf(&b, "- tool_guidance_files: `%d`\n", result.ToolGuidanceFiles)
	fmt.Fprintf(&b, "- enabled_tool_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.EnabledToolNamesHash, req.EnabledToolNamesHash)))
	fmt.Fprintf(&b, "- prompt_visible_tool_names_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.PromptVisibleToolHash, req.PromptVisibleToolHash)))
	fmt.Fprintf(&b, "- tool_output_manifest_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.ToolOutputManifestHash, req.ToolOutputManifestHash)))
	fmt.Fprintf(&b, "- tool_snapshot_sha256_12: `%s`\n", noneIfEmpty(firstNonEmpty(result.ToolSnapshotHash, req.ToolSnapshotHash)))
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", firstNonEmpty(result.ToolValidationStatus, req.ToolValidationStatus, "unknown"))
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", result.ToolValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", result.ToolValidationWarnings)
	fmt.Fprintf(&b, "- tool_risk_status: `%s`\n", firstNonEmpty(result.ToolRiskStatus, req.ToolRiskStatus, "unknown"))
	fmt.Fprintf(&b, "- tool_risk_findings: `%d`\n", result.ToolRiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", result.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", result.WarningRiskFindings)
	fmt.Fprintf(&b, "- dynamic_mcp_discovery_allowed: `%t`\n", result.DynamicMCPDiscoveryAllowed)
	fmt.Fprintf(&b, "- mcp_server_launch_allowed: `%t`\n", result.MCPServerLaunchAllowed)
	fmt.Fprintf(&b, "- toolset_activation_supported: `%t`\n", result.ToolsetActivationSupported)
	fmt.Fprintf(&b, "- model_callable_structured_tools: `%t`\n", result.ModelCallableStructuredTools)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_server_launch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- toolset_activation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_instructions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mcp_command_args_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing tool status snapshot on the canonical channel issue. This is the GitHub-native channel version of a tools-list command: it reports compact deterministic tool availability, prompt-visible counts, validation and risk totals from the current Actions checkout, but it does not execute tools, launch MCP servers, activate toolsets, call a model, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, status ids, raw schemas, raw tool inputs, raw tool outputs, toolset instructions, MCP command args, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the tool-status notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool-status notifications with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool-status notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- use `/channels request-tool`, `/channels approval-plan`, or `/channels rehearse-tool` when a channel message should become a reviewed tool workflow\n")
	return strings.TrimSpace(b.String())
}

func channelToolStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolStatusIssueTarget(ev Event, req *ChannelToolStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelToolStatusOptions(opts ChannelToolStatusOptions) ChannelToolStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StatusID = cleanChannelToolStatusID(opts.StatusID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToolStatusRoute(cfg Config, opts ChannelToolStatusOptions, repoContext RepoContext) (ChannelToolStatusOptions, error) {
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
		Body:      channelToolStatusListOrNone(channelToolStatusEnabledNames(repoContext)),
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

func validateChannelToolStatusOptions(opts ChannelToolStatusOptions) error {
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
		return fmt.Errorf("missing tool status id")
	}
	return nil
}

func validateChannelToolStatusActionRequestOptions(opts ChannelToolStatusOptions) error {
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
		return fmt.Errorf("missing tool status id")
	}
	return nil
}

func cleanChannelToolStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelToolStatusSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-tool-source-%s", eventID(ev))
}

func autoChannelToolStatusID(ev Event, opts ChannelToolStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID}, "|")
	return fmt.Sprintf("tool-status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolStatusNotifyMessageID(ev Event, statusID string) string {
	seed := strings.Join([]string{eventID(ev), statusID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelToolStatusNotificationBody(opts ChannelToolStatusOptions, cfg Config, repoContext RepoContext) string {
	snapshot := buildChannelToolStatusSnapshot(cfg, repoContext)
	var b strings.Builder
	b.WriteString("GitClaw channel tool status.\n\n")
	fmt.Fprintf(&b, "Available tools: %d\n", snapshot.AvailableTools)
	fmt.Fprintf(&b, "Enabled tools: %d\n", snapshot.EnabledTools)
	fmt.Fprintf(&b, "Disabled tools: %d\n", snapshot.DisabledTools)
	fmt.Fprintf(&b, "Allowlist blocked tools: %d\n", snapshot.AllowlistBlockedTools)
	fmt.Fprintf(&b, "Read-only contracts: %d\n", snapshot.ReadOnlyContracts)
	fmt.Fprintf(&b, "Metadata-only contracts: %d\n", snapshot.MetadataOnlyContracts)
	fmt.Fprintf(&b, "Mutating contracts: %d\n", snapshot.MutatingContracts)
	fmt.Fprintf(&b, "Enabled tool names: %s\n", channelToolStatusListOrNone(snapshot.EnabledToolNames))
	fmt.Fprintf(&b, "Toolsets scanned: %d\n", snapshot.ToolsetsScanned)
	fmt.Fprintf(&b, "MCP specs scanned: %d\n", snapshot.MCPSpecsScanned)
	fmt.Fprintf(&b, "Prompt-visible entries: %d\n", snapshot.PromptVisibleEntries)
	fmt.Fprintf(&b, "Active tool outputs: %d\n", snapshot.ActiveToolOutputs)
	fmt.Fprintf(&b, "Known tool outputs: %d\n", snapshot.KnownToolOutputs)
	fmt.Fprintf(&b, "Unknown tool outputs: %d\n", snapshot.UnknownToolOutputs)
	fmt.Fprintf(&b, "Validation status: %s\n", snapshot.ToolValidationStatus)
	fmt.Fprintf(&b, "Risk status: %s\n", snapshot.ToolRiskStatus)
	b.WriteString("Progressive disclosure: enabled\n")
	b.WriteString("Snapshot source: current GitHub Actions checkout\n")
	b.WriteString("\nRaw tool schemas: not included.\n")
	b.WriteString("Raw tool inputs: not included.\n")
	b.WriteString("Raw tool outputs: not included.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Shell execution: not performed by this action.\n")
	b.WriteString("MCP server launch: not performed by this action.\n")
	b.WriteString("Toolset activation: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelToolStatusSnapshot(cfg Config, repoContext RepoContext) channelToolStatusSnapshot {
	verify := BuildToolVerifyReport(repoContext)
	snapshot := BuildToolSnapshotReport(cfg, repoContext)
	return channelToolStatusSnapshot{
		AvailableTools:               verify.AvailableTools,
		EnabledTools:                 verify.EnabledTools,
		DisabledTools:                verify.DisabledTools,
		AllowlistBlockedTools:        verify.AllowlistBlockedTools,
		ReadOnlyContracts:            verify.ReadOnlyContracts,
		MetadataOnlyContracts:        verify.MetadataOnlyContracts,
		MutatingContracts:            verify.MutatingContracts,
		ActiveToolOutputs:            verify.ActiveOutputs,
		KnownToolOutputs:             verify.KnownToolOutputs,
		UnknownToolOutputs:           verify.UnknownToolOutputs,
		ToolsetsScanned:              snapshot.ToolsetsScanned,
		MCPSpecsScanned:              snapshot.MCPSpecsScanned,
		SnapshotEntries:              snapshot.SnapshotEntries,
		CatalogEntries:               snapshot.CatalogEntries,
		PromptVisibleEntries:         snapshot.PromptVisibleEntries,
		ToolGuidanceFiles:            verify.GuidanceFiles,
		EnabledToolNames:             channelToolStatusEnabledNames(repoContext),
		PromptVisibleToolNames:       channelToolStatusPromptVisibleNames(snapshot),
		ToolOutputManifest:           channelToolStatusOutputManifest(repoContext),
		ToolSnapshotHash:             snapshot.SnapshotSHA,
		ToolValidationStatus:         verify.Validation.Status,
		ToolValidationErrors:         verify.Validation.Errors,
		ToolValidationWarnings:       verify.Validation.Warnings,
		ToolRiskStatus:               verify.Risk.Status,
		ToolRiskFindings:             len(verify.Risk.Findings),
		HighRiskFindings:             verify.Risk.HighRiskFindings,
		WarningRiskFindings:          verify.Risk.WarningRiskFindings,
		DynamicMCPDiscoveryAllowed:   snapshot.DynamicMCPDiscoveryAllowed,
		MCPServerLaunchAllowed:       snapshot.MCPServerLaunchAllowed,
		ToolsetActivationSupported:   snapshot.ToolsetActivationSupported,
		ModelCallableStructuredTools: snapshot.ModelCallableStructuredTools,
	}
}

func channelToolStatusEnabledNames(repoContext RepoContext) []string {
	var names []string
	for _, contract := range toolReportContracts {
		if enabled, _, _ := toolEnabledInRepoContext(contract.Name, repoContext); enabled {
			names = append(names, contract.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelToolStatusPromptVisibleNames(snapshot ToolSnapshotReport) []string {
	var names []string
	for _, card := range snapshot.Cards {
		if card.PromptVisible {
			names = append(names, card.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelToolStatusOutputManifest(repoContext RepoContext) string {
	var lines []string
	for _, output := range repoContext.ToolOutputs {
		lines = append(lines, fmt.Sprintf("%s|%s|%d|%d|%s", output.Name, shortDocumentHash(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output)))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func channelToolStatusListOrNone(values []string) string {
	values = uniqueSortedStrings(values)
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
