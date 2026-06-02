package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolInfoOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	InfoID            string
	RequestedTool     string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolInfoReport struct {
	RequestedToolHash  string
	NormalizedHash     string
	InfoStatus         string
	AvailableTools     int
	MatchedTools       int
	ActiveOutputs      int
	ValidationStatus   string
	ValidationErrors   int
	ValidationWarnings int
	Contracts          []toolContract
	Outputs            []ToolOutput
	EnabledByName      map[string]bool
	DisabledByName     map[string]bool
	BlockedByName      map[string]bool
}

type ChannelToolInfoResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	InfoIDHash          string
	RequestedToolHash   string
	NormalizedToolHash  string
	Info                ChannelToolInfoReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelToolInfoActionRequest struct {
	Options              ChannelToolInfoOptions
	Info                 ChannelToolInfoReport
	Command              string
	Subcommand           string
	AutoSourceMessageID  bool
	AutoNotifyMessageID  bool
	AutoInfoID           bool
	TargetFromIssue      bool
	ToolSource           string
	RequestedRouteHash   string
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	InfoIDHash           string
	RequestedToolHash    string
	NormalizedToolHash   string
	RequestedToolBytes   int
	MatchedToolNamesHash string
	ToolInfoIndexHash    string
	NotificationBodySHA  string
	NotificationBytes    int
	NotificationLines    int
}

func IsChannelToolInfoActionRequest(ev Event, cfg Config) bool {
	return isChannelToolInfoActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolInfoActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelToolInfoSubcommand(fields[1]) {
	case "tool-info", "tools-info", "tool-describe", "describe-tool", "tool-card", "capability-info", "capability-describe":
		return true
	default:
		return false
	}
}

func BuildChannelToolInfoActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolInfoActionRequest, error) {
	fields, trailing, ok := channelToolInfoActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolInfoActionRequest{}, fmt.Errorf("missing channel tool info command")
	}
	req := ChannelToolInfoActionRequest{
		Options: ChannelToolInfoOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelToolInfoSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var toolParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--info-id", "--tool-info-id", "--capability-info-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.InfoID = cleanChannelToolInfoID(fields[i+1])
			i++
		case "--tool", "--name":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			toolParts = append(toolParts, fields[i+1])
			req.ToolSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolInfoActionRequest{}, fmt.Errorf("unknown channel tool info argument %q", field)
			}
			toolParts = append(toolParts, field)
			if req.ToolSource == "" {
				req.ToolSource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.RequestedTool) == "" {
		req.Options.RequestedTool = cleanChannelToolInfoName(strings.Join(toolParts, " "))
	}
	if strings.TrimSpace(req.Options.RequestedTool) == "" {
		req.Options.RequestedTool = parseChannelToolInfoTrailingTool(trailing)
		if req.Options.RequestedTool != "" {
			req.ToolSource = "trailing-tool"
		}
	}
	if err := applyChannelToolInfoIssueTarget(ev, &req); err != nil {
		return ChannelToolInfoActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelToolInfoSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.InfoID) == "" {
		req.Options.InfoID = autoChannelToolInfoID(ev, req.Options)
		req.AutoInfoID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolInfoNotifyMessageID(ev, req.Options.InfoID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolInfoOptions(req.Options)
	if err := validateChannelToolInfoActionRequestOptions(req.Options); err != nil {
		return ChannelToolInfoActionRequest{}, err
	}
	req.Info = BuildChannelToolInfoReport(repoContext, req.Options.RequestedTool)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.InfoIDHash = shortDocumentHash(req.Options.InfoID)
	req.RequestedToolHash = req.Info.RequestedToolHash
	req.NormalizedToolHash = req.Info.NormalizedHash
	req.RequestedToolBytes = len(req.Options.RequestedTool)
	req.MatchedToolNamesHash = hashStringList(channelToolInfoContractNames(req.Info.Contracts))
	req.ToolInfoIndexHash = hashStringOrNone(channelToolInfoIndex(req.Info))
	notificationBody := RenderChannelToolInfoNotificationBody(req.Options, req.Info)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelToolInfoReport(repoContext RepoContext, requestedTool string) ChannelToolInfoReport {
	requestedTool = cleanChannelToolInfoName(requestedTool)
	normalized := normalizeToolLookupName(requestedTool)
	validation := ValidateTools(repoContext)
	report := ChannelToolInfoReport{
		RequestedToolHash:  shortDocumentHash(requestedTool),
		NormalizedHash:     shortDocumentHash(normalized),
		InfoStatus:         "ok",
		AvailableTools:     len(toolReportContracts),
		ValidationStatus:   validation.Status,
		ValidationErrors:   validation.Errors,
		ValidationWarnings: validation.Warnings,
		EnabledByName:      map[string]bool{},
		DisabledByName:     map[string]bool{},
		BlockedByName:      map[string]bool{},
	}
	if requestedTool == "" || normalized == "" {
		report.InfoStatus = "missing_tool"
		return report
	}
	report.Contracts = matchingToolContracts(toolReportContracts, requestedTool)
	report.MatchedTools = len(report.Contracts)
	if len(report.Contracts) == 0 {
		report.InfoStatus = "not_found"
		return report
	}
	if len(report.Contracts) > 1 {
		report.InfoStatus = "ambiguous"
	}
	for _, contract := range report.Contracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		report.EnabledByName[contract.Name] = enabled
		report.DisabledByName[contract.Name] = disabled
		report.BlockedByName[contract.Name] = blocked
	}
	report.Outputs = matchingToolOutputs(repoContext.ToolOutputs, report.Contracts)
	report.ActiveOutputs = len(report.Outputs)
	return report
}

func RunChannelToolInfo(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelToolInfoActionRequest, repoContext RepoContext) (ChannelToolInfoResult, error) {
	opts := normalizeChannelToolInfoOptions(req.Options)
	var err error
	opts, err = applyChannelToolInfoRoute(cfg, opts)
	if err != nil {
		return ChannelToolInfoResult{}, err
	}
	if err := validateChannelToolInfoOptions(opts); err != nil {
		return ChannelToolInfoResult{}, err
	}
	info := req.Info
	if info.RequestedToolHash == "" {
		info = BuildChannelToolInfoReport(repoContext, opts.RequestedTool)
	}
	body := RenderChannelToolInfoNotificationBody(opts, info)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelToolInfoResult{}, fmt.Errorf("queue channel tool info notification: %w", err)
	}
	return ChannelToolInfoResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		InfoIDHash:          shortDocumentHash(opts.InfoID),
		RequestedToolHash:   info.RequestedToolHash,
		NormalizedToolHash:  info.NormalizedHash,
		Info:                info,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelToolInfoActionReport(ev Event, req ChannelToolInfoActionRequest, result ChannelToolInfoResult) string {
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
	infoIDHash := result.InfoIDHash
	if infoIDHash == "" {
		infoIDHash = req.InfoIDHash
	}
	requestedToolHash := result.RequestedToolHash
	if requestedToolHash == "" {
		requestedToolHash = req.RequestedToolHash
	}
	normalizedToolHash := result.NormalizedToolHash
	if normalizedToolHash == "" {
		normalizedToolHash = req.NormalizedToolHash
	}
	notificationBodySHA := result.NotificationBodySHA
	if notificationBodySHA == "" {
		notificationBodySHA = req.NotificationBodySHA
	}
	notificationBytes := result.NotificationBytes
	if notificationBytes == 0 {
		notificationBytes = req.NotificationBytes
	}
	notificationLines := result.NotificationLines
	if notificationLines == 0 {
		notificationLines = req.NotificationLines
	}
	info := result.Info
	if info.RequestedToolHash == "" {
		info = req.Info
	}
	matchedNamesHash := hashStringList(channelToolInfoContractNames(info.Contracts))
	if matchedNamesHash == "" {
		matchedNamesHash = req.MatchedToolNamesHash
	}
	infoIndexHash := hashStringOrNone(channelToolInfoIndex(info))
	if infoIndexHash == "" {
		infoIndexHash = req.ToolInfoIndexHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Tool Info Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_info_status: `%s`\n", info.InfoStatus)
	fmt.Fprintf(&b, "- info_mode: `%s`\n", "deterministic-tool-contract-card")
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
	fmt.Fprintf(&b, "- tool_info_id_sha256_12: `%s`\n", noneIfEmpty(infoIDHash))
	fmt.Fprintf(&b, "- tool_info_id_auto: `%t`\n", req.AutoInfoID)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", noneIfEmpty(requestedToolHash))
	fmt.Fprintf(&b, "- normalized_tool_sha256_12: `%s`\n", noneIfEmpty(normalizedToolHash))
	fmt.Fprintf(&b, "- requested_tool_bytes: `%d`\n", req.RequestedToolBytes)
	fmt.Fprintf(&b, "- tool_source: `%s`\n", noneIfEmpty(req.ToolSource))
	fmt.Fprintf(&b, "- available_tools: `%d`\n", info.AvailableTools)
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", info.MatchedTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", info.ActiveOutputs)
	fmt.Fprintf(&b, "- validation_status: `%s`\n", info.ValidationStatus)
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", info.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", info.ValidationWarnings)
	fmt.Fprintf(&b, "- matched_tool_names_sha256_12: `%s`\n", noneIfEmpty(matchedNamesHash))
	fmt.Fprintf(&b, "- tool_info_index_sha256_12: `%s`\n", noneIfEmpty(infoIndexHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- tool_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_server_launch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- toolset_activation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_trigger_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_info_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_info_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing focused tool card from deterministic tool contract metadata. The source receipt keeps the raw tool name, trigger, schemas, inputs, outputs, ids, and channel bodies out of band. The action does not call a model, execute tools, run shell commands, launch MCP servers, activate toolsets, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read tool-info cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool-info cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool-info notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolInfoNotificationBody(opts ChannelToolInfoOptions, report ChannelToolInfoReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool info\n\n")
	fmt.Fprintf(&b, "Tool info status: %s\n", report.InfoStatus)
	fmt.Fprintf(&b, "Requested tool hash: %s\n", report.RequestedToolHash)
	fmt.Fprintf(&b, "Normalized tool hash: %s\n", report.NormalizedHash)
	fmt.Fprintf(&b, "Available tools: %d\n", report.AvailableTools)
	fmt.Fprintf(&b, "Matched tools: %d\n", report.MatchedTools)
	fmt.Fprintf(&b, "Active outputs for tool: %d\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "Validation status: %s\n", report.ValidationStatus)
	fmt.Fprintf(&b, "Validation errors: %d\n", report.ValidationErrors)
	fmt.Fprintf(&b, "Validation warnings: %d\n", report.ValidationWarnings)
	fmt.Fprintf(&b, "Tool info id hash: %s\n", shortDocumentHash(opts.InfoID))
	b.WriteString("\nContracts:\n")
	if len(report.Contracts) == 0 {
		b.WriteString("- none\n")
	} else {
		activeCounts := map[string]int{}
		for _, output := range report.Outputs {
			activeCounts[output.Name]++
		}
		for _, contract := range report.Contracts {
			fmt.Fprintf(&b, "- name=%s source=builtin-gitclaw enabled=%t disabled_by_config=%t blocked_by_allowlist=%t mode=%s mutating=%t trigger_sha256_12=%s active_outputs=%d\n",
				contract.Name,
				report.EnabledByName[contract.Name],
				report.DisabledByName[contract.Name],
				report.BlockedByName[contract.Name],
				contract.Mode,
				isMutatingToolContract(contract),
				shortDocumentHash(contract.Trigger),
				activeCounts[contract.Name],
			)
		}
	}
	b.WriteString("\nActive outputs:\n")
	if len(report.Outputs) == 0 {
		b.WriteString("- none\n")
	} else {
		contracts := toolContractNameSet()
		for _, output := range report.Outputs {
			fmt.Fprintf(&b, "- name=%s contract_known=%t input_sha256_12=%s output_bytes=%d output_lines=%d output_sha256_12=%s\n",
				output.Name,
				contracts[output.Name],
				shortDocumentHash(output.Input),
				len(output.Output),
				lineCount(output.Output),
				shortDocumentHash(output.Output),
			)
		}
	}
	b.WriteString("\nRaw tool triggers, tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, and raw requested tool text are not included. Tool execution: not performed by this action. Shell execution: not performed by this action. MCP server launch: not performed by this action. Toolset activation: not performed by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelToolInfoActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolInfoActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolInfoIssueTarget(ev Event, req *ChannelToolInfoActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool info requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelToolInfoOptions(opts ChannelToolInfoOptions) ChannelToolInfoOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.InfoID = cleanChannelToolInfoID(opts.InfoID)
	opts.RequestedTool = cleanChannelToolInfoName(opts.RequestedTool)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToolInfoRoute(cfg Config, opts ChannelToolInfoOptions) (ChannelToolInfoOptions, error) {
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
		Body:      "GitClaw channel tool info.",
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

func validateChannelToolInfoOptions(opts ChannelToolInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing tool info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid tool info id %q", opts.InfoID)
	}
	if cleanChannelToolInfoName(opts.RequestedTool) == "" {
		return fmt.Errorf("missing requested tool")
	}
	return nil
}

func validateChannelToolInfoActionRequestOptions(opts ChannelToolInfoOptions) error {
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
	if opts.InfoID == "" {
		return fmt.Errorf("missing tool info id")
	}
	if !skillNamePattern.MatchString(opts.InfoID) {
		return fmt.Errorf("invalid tool info id %q", opts.InfoID)
	}
	if cleanChannelToolInfoName(opts.RequestedTool) == "" {
		return fmt.Errorf("missing requested tool")
	}
	return nil
}

func cleanChannelToolInfoSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelToolInfoID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelToolInfoName(value string) string {
	value = cleanToolLookupName(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func parseChannelToolInfoTrailingTool(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "tool:") || strings.HasPrefix(lower, "name:") || strings.HasPrefix(lower, "capability:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelToolInfoName(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelToolInfoSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-tool-info-source-%s", eventID(ev))
}

func autoChannelToolInfoID(ev Event, opts ChannelToolInfoOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.RequestedTool}, "|")
	return cleanChannelToolInfoID(fmt.Sprintf("tool-info-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelToolInfoNotifyMessageID(ev Event, infoID string) string {
	seed := strings.Join([]string{eventID(ev), infoID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-info-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelToolInfoContractNames(contracts []toolContract) []string {
	var names []string
	for _, contract := range contracts {
		if strings.TrimSpace(contract.Name) != "" {
			names = append(names, contract.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelToolInfoIndex(report ChannelToolInfoReport) string {
	var lines []string
	for _, contract := range report.Contracts {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%t|%t|%t|%t|%s", contract.Name, contract.Mode, shortDocumentHash(contract.Trigger), isMutatingToolContract(contract), report.EnabledByName[contract.Name], report.DisabledByName[contract.Name], report.BlockedByName[contract.Name], report.InfoStatus))
	}
	for _, output := range report.Outputs {
		lines = append(lines, fmt.Sprintf("%s|%s|%d|%d|%s", output.Name, shortDocumentHash(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output)))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}
