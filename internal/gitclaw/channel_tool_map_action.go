package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolMapOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MapID             string
	RequestedTool     string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolMapResult struct {
	Notification                 ChannelSendResult
	RouteName                    string
	RouteHash                    string
	Channel                      string
	ThreadHash                   string
	MessageHash                  string
	NotifyHash                   string
	MapIDHash                    string
	RequestedToolHash            string
	NoteHash                     string
	BodyHash                     string
	StepHash                     string
	SnapshotHash                 string
	StepCount                    int
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

type ChannelToolMapActionRequest struct {
	Options                      ChannelToolMapOptions
	Command                      string
	Subcommand                   string
	AutoSourceMessageID          bool
	AutoNotifyMessageID          bool
	AutoMapID                    bool
	TargetFromIssue              bool
	NoteSource                   string
	RequestedRouteHash           string
	RequestedThreadHash          string
	RequestedMsgHash             string
	NotifyMessageHash            string
	MapIDHash                    string
	RequestedToolSHA             string
	RequestedToolBytes           int
	RequestedToolTerms           int
	NoteSHA                      string
	NoteBytes                    int
	NoteLines                    int
	StepSHA                      string
	SnapshotSHA                  string
	StepCount                    int
	NotificationBodySHA          string
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

type channelToolMapSnapshot struct {
	ToolStatus   channelToolStatusSnapshot
	StepCount    int
	StepHash     string
	SnapshotHash string
}

type channelToolMapStep struct {
	Command string
	Reason  string
}

func IsChannelToolMapActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelToolMapActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelToolMapActionFields(fields)
}

func isChannelToolMapActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "tool-map", "tools-map", "tool-path", "tools-path", "tool-flow", "tools-flow", "tool-safety", "tools-safety", "safe-tool", "tool-runbook":
		return true
	default:
		return false
	}
}

func BuildChannelToolMapActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolMapActionRequest, error) {
	fields, trailing, ok := channelToolMapActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolMapActionRequest{}, fmt.Errorf("missing channel tool map command")
	}
	req := ChannelToolMapActionRequest{
		Options: ChannelToolMapOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--map-id", "--tool-map-id", "--runbook-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MapID = cleanChannelToolMapID(fields[i+1])
			i++
		case "--tool", "-t":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("--tool requires a value")
			}
			req.Options.RequestedTool = cleanToolLookupName(fields[i+1])
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolMapActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolMapActionRequest{}, fmt.Errorf("unknown channel tool map argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelToolMapIssueTargetIfPresent(ev, &req)
	if err := applyChannelToolMapPositionals(&req, positional); err != nil {
		return ChannelToolMapActionRequest{}, err
	}
	if err := applyChannelToolMapIssueTarget(ev, &req); err != nil {
		return ChannelToolMapActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelToolMapTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelToolMapSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MapID) == "" {
		req.Options.MapID = autoChannelToolMapID(ev, req.Options)
		req.AutoMapID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolMapNotifyMessageID(ev, req.Options.MapID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolMapOptions(req.Options)
	if err := validateChannelToolMapActionRequestOptions(req.Options); err != nil {
		return ChannelToolMapActionRequest{}, err
	}
	snapshot := buildChannelToolMapSnapshot(cfg, repoContext, req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MapIDHash = shortDocumentHash(req.Options.MapID)
	req.RequestedToolSHA = shortDocumentHash(req.Options.RequestedTool)
	req.RequestedToolBytes = len(req.Options.RequestedTool)
	req.RequestedToolTerms = len(memorySearchTerms(req.Options.RequestedTool))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelToolMapNotificationBody(req.Options, cfg, repoContext))
	req.applySnapshot(snapshot)
	return req, nil
}

func RunChannelToolMap(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolMapOptions, repoContext RepoContext) (ChannelToolMapResult, error) {
	opts = normalizeChannelToolMapOptions(opts)
	var err error
	opts, err = applyChannelToolMapRoute(cfg, opts)
	if err != nil {
		return ChannelToolMapResult{}, err
	}
	if err := validateChannelToolMapOptions(opts); err != nil {
		return ChannelToolMapResult{}, err
	}
	body := renderChannelToolMapNotificationBody(opts, cfg, repoContext)
	snapshot := buildChannelToolMapSnapshot(cfg, repoContext, opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelToolMapResult{}, fmt.Errorf("queue channel tool map notification: %w", err)
	}
	result := ChannelToolMapResult{
		Notification:      notification,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
		MapIDHash:         shortDocumentHash(opts.MapID),
		RequestedToolHash: shortDocumentHash(opts.RequestedTool),
		NoteHash:          shortDocumentHash(opts.Note),
		BodyHash:          shortDocumentHash(body),
		StepHash:          snapshot.StepHash,
		SnapshotHash:      snapshot.SnapshotHash,
		StepCount:         snapshot.StepCount,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelToolMapActionReport(ev Event, req ChannelToolMapActionRequest, result ChannelToolMapResult) string {
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
	mapIDHash := firstNonEmpty(result.MapIDHash, req.MapIDHash)
	requestedToolHash := firstNonEmpty(result.RequestedToolHash, req.RequestedToolSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	stepHash := firstNonEmpty(result.StepHash, req.StepSHA)
	snapshotHash := firstNonEmpty(result.SnapshotHash, req.SnapshotSHA)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Tool Map Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_map_mode: `%s`\n", "provider-facing-tool-safety-sequence")
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
	fmt.Fprintf(&b, "- tool_map_id_sha256_12: `%s`\n", noneIfEmpty(mapIDHash))
	fmt.Fprintf(&b, "- tool_map_id_auto: `%t`\n", req.AutoMapID)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", noneIfEmpty(requestedToolHash))
	fmt.Fprintf(&b, "- requested_tool_bytes: `%d`\n", req.RequestedToolBytes)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- tool_map_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- tool_map_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- tool_map_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- tool_map_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- tool_map_step_count: `%d`\n", nonzeroOrReq(result.StepCount, req.StepCount))
	fmt.Fprintf(&b, "- tool_map_step_sha256_12: `%s`\n", noneIfEmpty(stepHash))
	fmt.Fprintf(&b, "- tool_map_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
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
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_server_launch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- toolset_activation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_run_request_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_map_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_tool_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_map_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_map_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_map_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing tool map on the canonical channel issue. This is a safe sequence card for Slack or Telegram users who want to move from tool discovery to reviewed tool workflows: it reports compact tool snapshot metadata and points at status, search, info, approval-plan, rehearsal, and request-run commands, but it does not execute tools, launch MCP servers, activate toolsets, create approval issues, create rehearsal issues, create run-request issues, call a model, mutate workflows, mutate the repository, or call provider APIs. The source receipt keeps thread ids, message ids, map ids, requested tool names, notes, step text, raw schemas, raw inputs, raw outputs, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read tool-map cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool-map cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool-map cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelToolMapActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolMapActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolMapIssueTarget(ev Event, req *ChannelToolMapActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool map requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelToolMapIssueTargetIfPresent(ev Event, req *ChannelToolMapActionRequest) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
}

func applyChannelToolMapPositionals(req *ChannelToolMapActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for i, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !req.TargetFromIssue && req.Options.Route == "" && req.Options.Channel == "" && req.Options.RequestedTool == "" && len(positional)-i > 1 {
			req.Options.Route = value
			continue
		}
		if req.Options.RequestedTool == "" {
			req.Options.RequestedTool = cleanToolLookupName(value)
			continue
		}
		if req.Options.MapID == "" {
			req.Options.MapID = cleanChannelToolMapID(value)
			continue
		}
		return fmt.Errorf("unexpected channel tool map argument %q", value)
	}
	return nil
}

func normalizeChannelToolMapOptions(opts ChannelToolMapOptions) ChannelToolMapOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MapID = cleanChannelToolMapID(opts.MapID)
	opts.RequestedTool = cleanToolLookupName(opts.RequestedTool)
	opts.Note = cleanChannelToolMapNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToolMapRoute(cfg Config, opts ChannelToolMapOptions) (ChannelToolMapOptions, error) {
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
		Body:      "GitClaw channel tool map.",
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

func validateChannelToolMapOptions(opts ChannelToolMapOptions) error {
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
	if opts.MapID == "" {
		return fmt.Errorf("missing tool map id")
	}
	if opts.RequestedTool == "" {
		return fmt.Errorf("missing requested tool")
	}
	return nil
}

func validateChannelToolMapActionRequestOptions(opts ChannelToolMapOptions) error {
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
	if opts.MapID == "" {
		return fmt.Errorf("missing tool map id")
	}
	if opts.RequestedTool == "" {
		return fmt.Errorf("missing requested tool")
	}
	return nil
}

func cleanChannelToolMapID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelToolMapNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelToolMapTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelToolMapTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelToolMapNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelToolMapTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func autoChannelToolMapSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-tool-map-source-%s", eventID(ev))
}

func autoChannelToolMapID(ev Event, opts ChannelToolMapOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedTool, opts.Note}, "|")
	return fmt.Sprintf("tool-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolMapNotifyMessageID(ev Event, mapID string) string {
	seed := strings.Join([]string{eventID(ev), mapID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-map-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelToolMapNotificationBody(opts ChannelToolMapOptions, cfg Config, repoContext RepoContext) string {
	opts = normalizeChannelToolMapOptions(opts)
	snapshot := buildChannelToolMapSnapshot(cfg, repoContext, opts)
	steps := channelToolMapStepsForTool(opts.RequestedTool, snapshot.ToolStatus)
	var b strings.Builder
	b.WriteString("GitClaw channel tool map.\n\n")
	fmt.Fprintf(&b, "Requested tool: %s\n", opts.RequestedTool)
	fmt.Fprintf(&b, "Available tools: %d\n", snapshot.ToolStatus.AvailableTools)
	fmt.Fprintf(&b, "Enabled tools: %d\n", snapshot.ToolStatus.EnabledTools)
	fmt.Fprintf(&b, "Disabled tools: %d\n", snapshot.ToolStatus.DisabledTools)
	fmt.Fprintf(&b, "Allowlist blocked tools: %d\n", snapshot.ToolStatus.AllowlistBlockedTools)
	fmt.Fprintf(&b, "Read-only contracts: %d\n", snapshot.ToolStatus.ReadOnlyContracts)
	fmt.Fprintf(&b, "Metadata-only contracts: %d\n", snapshot.ToolStatus.MetadataOnlyContracts)
	fmt.Fprintf(&b, "Mutating contracts: %d\n", snapshot.ToolStatus.MutatingContracts)
	fmt.Fprintf(&b, "Validation status: %s\n", snapshot.ToolStatus.ToolValidationStatus)
	fmt.Fprintf(&b, "Risk status: %s\n", snapshot.ToolStatus.ToolRiskStatus)
	fmt.Fprintf(&b, "Prompt-visible entries: %d\n", snapshot.ToolStatus.PromptVisibleEntries)
	b.WriteString("\nTool sequence:\n")
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. `%s` - %s\n", i+1, step.Command, step.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Tool map hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Tool step hash: %s\n", snapshot.StepHash)
	b.WriteString("\nMap source: current GitHub Actions checkout tool metadata.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Shell execution: not performed by this action.\n")
	b.WriteString("MCP server launch: not performed by this action.\n")
	b.WriteString("Toolset activation: not performed by this action.\n")
	b.WriteString("Approval issue creation: not performed by this action.\n")
	b.WriteString("Rehearsal issue creation: not performed by this action.\n")
	b.WriteString("Tool-run request issue creation: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func buildChannelToolMapSnapshot(cfg Config, repoContext RepoContext, opts ChannelToolMapOptions) channelToolMapSnapshot {
	toolStatus := buildChannelToolStatusSnapshot(cfg, repoContext)
	steps := channelToolMapStepsForTool(opts.RequestedTool, toolStatus)
	snapshot := channelToolMapSnapshot{
		ToolStatus: toolStatus,
		StepCount:  len(steps),
		StepHash:   shortDocumentHash(channelToolMapStepManifest(steps)),
	}
	snapshot.SnapshotHash = shortDocumentHash(channelToolMapSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelToolMapStepsForTool(tool string, snapshot channelToolStatusSnapshot) []channelToolMapStep {
	tool = cleanToolLookupName(tool)
	if tool == "" {
		tool = "<tool>"
	}
	return []channelToolMapStep{
		{Command: "/channels tools --message-id <id> --notify-message-id <id>", Reason: fmt.Sprintf("confirm current tool availability first (%d enabled tools)", snapshot.EnabledTools)},
		{Command: fmt.Sprintf("/channels tool-search %s --message-id <id> --notify-message-id <id>", tool), Reason: "find nearby capabilities without exposing raw schemas"},
		{Command: fmt.Sprintf("/channels tool-info %s --message-id <id> --notify-message-id <id>", tool), Reason: "inspect the focused tool card before any reviewed workflow"},
		{Command: fmt.Sprintf("/channels approval-plan %s --id <approval-plan-id> --message-id <id> --notify-message-id <id>", tool), Reason: "open a dry-run approval review issue without granting approval"},
		{Command: fmt.Sprintf("/channels rehearse-tool %s --id <rehearsal-id> --message-id <id> --notify-message-id <id>", tool), Reason: "open a rehearsal issue for model-backed conversation around the boundary"},
		{Command: fmt.Sprintf("/channels request-run %s --id <request-id> --message-id <id> --notify-message-id <id>", tool), Reason: "open a reviewed run request issue when execution should be considered"},
	}
}

func channelToolMapStepManifest(steps []channelToolMapStep) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, strings.Join([]string{step.Command, step.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}

func channelToolMapSnapshotManifest(opts ChannelToolMapOptions, snapshot channelToolMapSnapshot) string {
	toolStatus := snapshot.ToolStatus
	return fmt.Sprintf(
		"tool=%s\nstatus=%d/%d/%d/%d/%d/%d/%d\noutputs=%d/%d/%d\nsnapshot=%d/%d/%d/%d/%d/%d\nvalidation=%s/%d/%d\nrisk=%s/%d/%d/%d\nruntime=%t/%t/%t/%t\nsteps=%d/%s\nnote=%s",
		shortDocumentHash(cleanToolLookupName(opts.RequestedTool)),
		toolStatus.AvailableTools,
		toolStatus.EnabledTools,
		toolStatus.DisabledTools,
		toolStatus.AllowlistBlockedTools,
		toolStatus.ReadOnlyContracts,
		toolStatus.MetadataOnlyContracts,
		toolStatus.MutatingContracts,
		toolStatus.ActiveToolOutputs,
		toolStatus.KnownToolOutputs,
		toolStatus.UnknownToolOutputs,
		toolStatus.ToolsetsScanned,
		toolStatus.MCPSpecsScanned,
		toolStatus.SnapshotEntries,
		toolStatus.CatalogEntries,
		toolStatus.PromptVisibleEntries,
		toolStatus.ToolGuidanceFiles,
		toolStatus.ToolValidationStatus,
		toolStatus.ToolValidationErrors,
		toolStatus.ToolValidationWarnings,
		toolStatus.ToolRiskStatus,
		toolStatus.ToolRiskFindings,
		toolStatus.HighRiskFindings,
		toolStatus.WarningRiskFindings,
		toolStatus.DynamicMCPDiscoveryAllowed,
		toolStatus.MCPServerLaunchAllowed,
		toolStatus.ToolsetActivationSupported,
		toolStatus.ModelCallableStructuredTools,
		snapshot.StepCount,
		snapshot.StepHash,
		shortDocumentHash(opts.Note),
	)
}

func (r *ChannelToolMapActionRequest) applySnapshot(snapshot channelToolMapSnapshot) {
	r.AvailableTools = snapshot.ToolStatus.AvailableTools
	r.EnabledTools = snapshot.ToolStatus.EnabledTools
	r.DisabledTools = snapshot.ToolStatus.DisabledTools
	r.AllowlistBlockedTools = snapshot.ToolStatus.AllowlistBlockedTools
	r.ReadOnlyContracts = snapshot.ToolStatus.ReadOnlyContracts
	r.MetadataOnlyContracts = snapshot.ToolStatus.MetadataOnlyContracts
	r.MutatingContracts = snapshot.ToolStatus.MutatingContracts
	r.ActiveToolOutputs = snapshot.ToolStatus.ActiveToolOutputs
	r.KnownToolOutputs = snapshot.ToolStatus.KnownToolOutputs
	r.UnknownToolOutputs = snapshot.ToolStatus.UnknownToolOutputs
	r.ToolsetsScanned = snapshot.ToolStatus.ToolsetsScanned
	r.MCPSpecsScanned = snapshot.ToolStatus.MCPSpecsScanned
	r.SnapshotEntries = snapshot.ToolStatus.SnapshotEntries
	r.CatalogEntries = snapshot.ToolStatus.CatalogEntries
	r.PromptVisibleEntries = snapshot.ToolStatus.PromptVisibleEntries
	r.ToolGuidanceFiles = snapshot.ToolStatus.ToolGuidanceFiles
	r.EnabledToolNamesHash = hashStringList(snapshot.ToolStatus.EnabledToolNames)
	r.PromptVisibleToolHash = hashStringList(snapshot.ToolStatus.PromptVisibleToolNames)
	r.ToolOutputManifestHash = hashStringOrNone(snapshot.ToolStatus.ToolOutputManifest)
	r.ToolSnapshotHash = snapshot.ToolStatus.ToolSnapshotHash
	r.ToolValidationStatus = snapshot.ToolStatus.ToolValidationStatus
	r.ToolValidationErrors = snapshot.ToolStatus.ToolValidationErrors
	r.ToolValidationWarnings = snapshot.ToolStatus.ToolValidationWarnings
	r.ToolRiskStatus = snapshot.ToolStatus.ToolRiskStatus
	r.ToolRiskFindings = snapshot.ToolStatus.ToolRiskFindings
	r.HighRiskFindings = snapshot.ToolStatus.HighRiskFindings
	r.WarningRiskFindings = snapshot.ToolStatus.WarningRiskFindings
	r.DynamicMCPDiscoveryAllowed = snapshot.ToolStatus.DynamicMCPDiscoveryAllowed
	r.MCPServerLaunchAllowed = snapshot.ToolStatus.MCPServerLaunchAllowed
	r.ToolsetActivationSupported = snapshot.ToolStatus.ToolsetActivationSupported
	r.ModelCallableStructuredTools = snapshot.ToolStatus.ModelCallableStructuredTools
	r.StepCount = snapshot.StepCount
	r.StepSHA = snapshot.StepHash
	r.SnapshotSHA = snapshot.SnapshotHash
}

func (r *ChannelToolMapResult) applySnapshot(snapshot channelToolMapSnapshot) {
	r.AvailableTools = snapshot.ToolStatus.AvailableTools
	r.EnabledTools = snapshot.ToolStatus.EnabledTools
	r.DisabledTools = snapshot.ToolStatus.DisabledTools
	r.AllowlistBlockedTools = snapshot.ToolStatus.AllowlistBlockedTools
	r.ReadOnlyContracts = snapshot.ToolStatus.ReadOnlyContracts
	r.MetadataOnlyContracts = snapshot.ToolStatus.MetadataOnlyContracts
	r.MutatingContracts = snapshot.ToolStatus.MutatingContracts
	r.ActiveToolOutputs = snapshot.ToolStatus.ActiveToolOutputs
	r.KnownToolOutputs = snapshot.ToolStatus.KnownToolOutputs
	r.UnknownToolOutputs = snapshot.ToolStatus.UnknownToolOutputs
	r.ToolsetsScanned = snapshot.ToolStatus.ToolsetsScanned
	r.MCPSpecsScanned = snapshot.ToolStatus.MCPSpecsScanned
	r.SnapshotEntries = snapshot.ToolStatus.SnapshotEntries
	r.CatalogEntries = snapshot.ToolStatus.CatalogEntries
	r.PromptVisibleEntries = snapshot.ToolStatus.PromptVisibleEntries
	r.ToolGuidanceFiles = snapshot.ToolStatus.ToolGuidanceFiles
	r.EnabledToolNamesHash = hashStringList(snapshot.ToolStatus.EnabledToolNames)
	r.PromptVisibleToolHash = hashStringList(snapshot.ToolStatus.PromptVisibleToolNames)
	r.ToolOutputManifestHash = hashStringOrNone(snapshot.ToolStatus.ToolOutputManifest)
	r.ToolSnapshotHash = snapshot.ToolStatus.ToolSnapshotHash
	r.ToolValidationStatus = snapshot.ToolStatus.ToolValidationStatus
	r.ToolValidationErrors = snapshot.ToolStatus.ToolValidationErrors
	r.ToolValidationWarnings = snapshot.ToolStatus.ToolValidationWarnings
	r.ToolRiskStatus = snapshot.ToolStatus.ToolRiskStatus
	r.ToolRiskFindings = snapshot.ToolStatus.ToolRiskFindings
	r.HighRiskFindings = snapshot.ToolStatus.HighRiskFindings
	r.WarningRiskFindings = snapshot.ToolStatus.WarningRiskFindings
	r.DynamicMCPDiscoveryAllowed = snapshot.ToolStatus.DynamicMCPDiscoveryAllowed
	r.MCPServerLaunchAllowed = snapshot.ToolStatus.MCPServerLaunchAllowed
	r.ToolsetActivationSupported = snapshot.ToolStatus.ToolsetActivationSupported
	r.ModelCallableStructuredTools = snapshot.ToolStatus.ModelCallableStructuredTools
	r.StepCount = snapshot.StepCount
}
