package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolRunRequestOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RequestID         string
	RequestedTool     string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolRunRequestResult struct {
	ToolRequest   ToolRunRequestIssueResult
	Notification  ChannelSendResult
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	RequestIDHash string
}

type ChannelToolRunRequestActionRequest struct {
	Options             ChannelToolRunRequestOptions
	ToolRequest         ToolRunRequestIssueRequest
	Command             string
	Subcommand          string
	AutoRequestID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedToolSHA    string
	RequestedToolTerms  int
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelToolRunRequestActionRequest(ev Event, cfg Config) bool {
	return isChannelToolRunRequestActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolRunRequestActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "request-run", "run-request", "tool-run", "request-tool", "tool-request":
		return true
	default:
		return false
	}
}

func BuildChannelToolRunRequestActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolRunRequestActionRequest, error) {
	fields, ok := channelToolRunRequestActionFields(ev, cfg)
	if !ok {
		return ChannelToolRunRequestActionRequest{}, fmt.Errorf("missing channel tool run request command")
	}
	req := ChannelToolRunRequestActionRequest{
		Options: ChannelToolRunRequestOptions{
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
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--tool", "-t":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("--tool requires a value")
			}
			req.Options.RequestedTool = cleanToolLookupName(fields[i+1])
			i++
		case "--id", "--request-id":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RequestID = cleanToolRunRequestID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolRunRequestActionRequest{}, fmt.Errorf("unknown channel tool run request argument %q", field)
			}
			if req.Options.RequestedTool == "" {
				req.Options.RequestedTool = cleanToolLookupName(field)
				continue
			}
			if req.Options.RequestID == "" {
				req.Options.RequestID = cleanToolRunRequestID(field)
				continue
			}
			return ChannelToolRunRequestActionRequest{}, fmt.Errorf("unexpected channel tool run request argument %q", field)
		}
	}
	if err := applyChannelToolRunRequestIssueTarget(ev, &req); err != nil {
		return ChannelToolRunRequestActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RequestID) == "" {
		req.Options.RequestID = autoChannelToolRunRequestID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.RequestedTool)
		req.AutoRequestID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolRunRequestNotifyMessageID(ev, req.Options.RequestID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolRunRequestOptions(req.Options)
	if err := validateChannelToolRunRequestOptions(req.Options); err != nil {
		return ChannelToolRunRequestActionRequest{}, err
	}
	toolReq, err := buildToolRunRequestIssueRequestFromChannel(ev, repoContext, req.Options)
	if err != nil {
		return ChannelToolRunRequestActionRequest{}, err
	}
	req.ToolRequest = toolReq
	req.RequestedToolSHA = toolReq.RequestedToolSHA
	req.RequestedToolTerms = toolReq.RequestedToolTerms
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelToolRunRequestNotificationBody(req.Options, ToolRunRequestIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, toolReq)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelToolRunRequest(ctx context.Context, cfg Config, github interface {
	ToolRunRequestIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelToolRunRequestActionRequest) (ChannelToolRunRequestResult, error) {
	toolResult, err := RunToolRunRequestIssue(ctx, github, req.ToolRequest)
	if err != nil {
		return ChannelToolRunRequestResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      RenderChannelToolRunRequestNotificationBody(req.Options, toolResult, req.ToolRequest),
	})
	if err != nil {
		return ChannelToolRunRequestResult{}, fmt.Errorf("queue channel tool run request notification: %w", err)
	}
	return ChannelToolRunRequestResult{
		ToolRequest:   toolResult,
		Notification:  notification,
		Channel:       req.Options.Channel,
		ThreadHash:    shortDocumentHash(req.Options.ThreadID),
		MessageHash:   shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:    shortDocumentHash(req.Options.NotifyMessageID),
		RequestIDHash: shortDocumentHash(req.Options.RequestID),
	}, nil
}

func RenderChannelToolRunRequestActionReport(ev Event, req ChannelToolRunRequestActionRequest, result ChannelToolRunRequestResult) string {
	status := "created"
	switch {
	case result.ToolRequest.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ToolRequest.Duplicate:
		status = "existing"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
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
	b.WriteString("## GitClaw Channel Tool Run Request Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_run_request_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_run_request_issue: `#%d`\n", result.ToolRequest.IssueNumber)
	fmt.Fprintf(&b, "- tool_run_request_issue_url: `%s`\n", result.ToolRequest.IssueURL)
	fmt.Fprintf(&b, "- tool_run_request_issue_created: `%t`\n", result.ToolRequest.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ToolRequest.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- request_id_sha256_12: `%s`\n", result.RequestIDHash)
	fmt.Fprintf(&b, "- request_id_auto: `%t`\n", req.AutoRequestID)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", req.RequestedToolSHA)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- normalized_tool: `%s`\n", valueOrNone(req.ToolRequest.NormalizedTool))
	fmt.Fprintf(&b, "- matched_tool: `%s`\n", valueOrNone(req.ToolRequest.MatchedTool))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", req.ToolRequest.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", req.ToolRequest.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", req.ToolRequest.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", req.ToolRequest.ToolEnabled)
	fmt.Fprintf(&b, "- tool_mode: `%s`\n", valueOrNone(req.ToolRequest.ToolMode))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", req.ToolRequest.MutatingContract)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", req.ToolRequest.ApprovalRequired)
	fmt.Fprintf(&b, "- run_allowed_now: `%t`\n", req.ToolRequest.RunAllowedNow)
	fmt.Fprintf(&b, "- review_decision: `%s`\n", req.ToolRequest.ReviewDecision)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", req.ToolRequest.ValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", req.ToolRequest.ValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", req.ToolRequest.ValidationWarnings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", req.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", req.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- request_store: `%s`\n", "github-issue-to-reviewed-tool-run")
	fmt.Fprintf(&b, "- review_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_request_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_tool_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_run_request_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a reviewed GitHub tool-run request issue from a mirrored channel thread, then queued a provider-facing review link back to that thread. This action does not call a model, execute a tool, run shell commands, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Review Path\n")
	fmt.Fprintf(&b, "- review request issue: `#%d`\n", result.ToolRequest.IssueNumber)
	b.WriteString("- continue in GitHub to review the request before converting it into any future approved tool-run workflow\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolRunRequestNotificationBody(opts ChannelToolRunRequestOptions, result ToolRunRequestIssueResult, toolReq ToolRunRequestIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool run request\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Review issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Normalized tool: %s\n", valueOrNone(toolReq.NormalizedTool))
	fmt.Fprintf(&b, "Review decision: %s\n", toolReq.ReviewDecision)
	fmt.Fprintf(&b, "Run allowed now: %t\n", toolReq.RunAllowedNow)
	b.WriteString("\nReview the linked GitHub issue before converting this into a model-backed tool turn or future approved workflow. This notification did not execute a model, tool, shell command, or repository mutation.")
	return strings.TrimSpace(b.String())
}

func channelToolRunRequestActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelToolRunRequestActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelToolRunRequestIssueTarget(ev Event, req *ChannelToolRunRequestActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool run request requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelToolRunRequestOptions(opts ChannelToolRunRequestOptions) ChannelToolRunRequestOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RequestID = cleanToolRunRequestID(opts.RequestID)
	opts.RequestedTool = cleanToolLookupName(opts.RequestedTool)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelToolRunRequestOptions(opts ChannelToolRunRequestOptions) error {
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
	if opts.RequestID == "" {
		return fmt.Errorf("missing tool run request id")
	}
	if !skillNamePattern.MatchString(opts.RequestID) {
		return fmt.Errorf("invalid tool run request id %q", opts.RequestID)
	}
	if opts.RequestedTool == "" {
		return fmt.Errorf("missing requested tool")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildToolRunRequestIssueRequestFromChannel(ev Event, repoContext RepoContext, opts ChannelToolRunRequestOptions) (ToolRunRequestIssueRequest, error) {
	requestedTool := cleanToolLookupName(opts.RequestedTool)
	if requestedTool == "" {
		return ToolRunRequestIssueRequest{}, fmt.Errorf("missing tool run request target")
	}
	normalized := normalizeToolLookupName(requestedTool)
	matches := matchingToolContracts(toolReportContracts, requestedTool)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	sourceText := activeRequestText(ev)
	req := ToolRunRequestIssueRequest{
		Repo:                 ev.Repo,
		Command:              "/tools",
		Subcommand:           "request-run",
		RequestID:            opts.RequestID,
		RequestedToolSHA:     shortDocumentHash(requestedTool),
		RequestedToolTerms:   len(memorySearchTerms(requestedTool)),
		NormalizedTool:       normalized,
		MatchedTools:         len(matches),
		ActiveOutputsForTool: len(activeOutputs),
		AvailableTools:       len(toolReportContracts),
		ValidationStatus:     validation.Status,
		ValidationErrors:     validation.Errors,
		ValidationWarnings:   validation.Warnings,
		SourceIssueNumber:    opts.SourceIssueNumber,
		SourceCommentID:      opts.SourceCommentID,
		SourceSHA:            shortDocumentHash(sourceText),
		SourceBytes:          len(sourceText),
		SourceLines:          lineCount(sourceText),
		SourceKind:           "channel_comment",
	}
	if len(matches) == 1 {
		match := matches[0]
		req.MatchedTool = match.Name
		req.ToolMode = match.Mode
		req.ToolTrigger = match.Trigger
		req.MutatingContract = isMutatingToolContract(match)
		req.ToolEnabled, req.DisabledByConfig, req.BlockedByAllowlist = toolEnabledInRepoContext(match.Name, repoContext)
	}
	req.ApprovalRequired = req.MutatingContract
	req.RunAllowedNow = toolRunRequestAllowed(req)
	req.ReviewDecision = toolRunRequestDecision(req)
	return req, nil
}

func autoChannelToolRunRequestID(ev Event, channel, threadID, sourceMessageID, requestedTool string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, requestedTool}, "|")
	return fmt.Sprintf("tool-run-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolRunRequestNotifyMessageID(ev Event, requestID string) string {
	seed := strings.Join([]string{eventID(ev), requestID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-request-%s-%s", eventID(ev), shortDocumentHash(seed))
}
