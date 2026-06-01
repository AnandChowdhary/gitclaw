package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const toolRunRequestIssueMarker = "gitclaw:tool-run-request-issue"

type ToolRunRequestIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type ToolRunRequestIssueRequest struct {
	Repo                 string
	Command              string
	Subcommand           string
	RequestID            string
	NotifyRoutes         []string
	NotifyRoutesSHA      string
	RequestedToolSHA     string
	RequestedToolTerms   int
	NormalizedTool       string
	MatchedTool          string
	MatchedTools         int
	ActiveOutputsForTool int
	AvailableTools       int
	ToolEnabled          bool
	DisabledByConfig     bool
	BlockedByAllowlist   bool
	ToolMode             string
	ToolTrigger          string
	MutatingContract     bool
	ApprovalRequired     bool
	RunAllowedNow        bool
	ReviewDecision       string
	ValidationStatus     string
	ValidationErrors     int
	ValidationWarnings   int
	SourceIssueNumber    int
	SourceCommentID      int64
	SourceSHA            string
	SourceBytes          int
	SourceLines          int
	SourceKind           string
}

type ToolRunRequestIssueResult struct {
	IssueNumber         int
	IssueURL            string
	Created             bool
	Duplicate           bool
	ChannelNotification ToolRunRequestChannelNotification
}

type ToolRunRequestChannelNotification struct {
	Requested           bool
	Routes              int
	Queued              int
	Duplicates          int
	TargetIssuesCreated int
	MessageSHA          string
	BodySHA             string
	BodyBytes           int
	BodyLines           int
	Destinations        []ChannelBroadcastDestinationResult
}

func IsToolRunRequestIssueRequest(ev Event, cfg Config) bool {
	return isToolRunRequestIssueFields(activeSlashCommandFields(ev, cfg))
}

func isToolRunRequestIssueFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/tools" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "request-run", "run-request", "queue-run", "request", "run-issue":
		return true
	default:
		return false
	}
}

func BuildToolRunRequestIssueRequest(ev Event, cfg Config, repoContext RepoContext) (ToolRunRequestIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isToolRunRequestIssueFields(fields) {
		return ToolRunRequestIssueRequest{}, fmt.Errorf("missing tool run request command")
	}
	sourceText := activeRequestText(ev)
	toolName, requestID, notifyRoutes, err := parseToolRunRequestIssueArgs(fields[2:], sourceText)
	if err != nil {
		return ToolRunRequestIssueRequest{}, err
	}
	requestedTool := cleanToolLookupName(toolName)
	if requestedTool == "" {
		return ToolRunRequestIssueRequest{}, fmt.Errorf("missing tool run request target")
	}
	if requestID == "" {
		requestID = fmt.Sprintf("tool-run-%s", shortDocumentHash(sourceText))
	}
	if !skillNamePattern.MatchString(requestID) {
		return ToolRunRequestIssueRequest{}, fmt.Errorf("invalid tool run request id %q", requestID)
	}

	normalized := normalizeToolLookupName(requestedTool)
	matches := matchingToolContracts(toolReportContracts, requestedTool)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)

	req := ToolRunRequestIssueRequest{
		Repo:                 ev.Repo,
		Command:              fields[0],
		Subcommand:           strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RequestID:            requestID,
		NotifyRoutes:         normalizeChannelBroadcastRoutes(notifyRoutes),
		NotifyRoutesSHA:      channelBroadcastRoutesHash(notifyRoutes),
		RequestedToolSHA:     shortDocumentHash(requestedTool),
		RequestedToolTerms:   len(memorySearchTerms(requestedTool)),
		NormalizedTool:       normalized,
		MatchedTools:         len(matches),
		ActiveOutputsForTool: len(activeOutputs),
		AvailableTools:       len(toolReportContracts),
		ValidationStatus:     validation.Status,
		ValidationErrors:     validation.Errors,
		ValidationWarnings:   validation.Warnings,
		SourceIssueNumber:    ev.Issue.Number,
		SourceSHA:            shortDocumentHash(sourceText),
		SourceBytes:          len(sourceText),
		SourceLines:          lineCount(sourceText),
		SourceKind:           "issue",
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
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

func RunToolRunRequestIssue(ctx context.Context, github ToolRunRequestIssueGitHubClient, req ToolRunRequestIssueRequest) (ToolRunRequestIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return ToolRunRequestIssueResult{}, err
	}
	if req.RequestID == "" {
		return ToolRunRequestIssueResult{}, fmt.Errorf("missing tool run request id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, nil, 300)
	if err != nil {
		return ToolRunRequestIssueResult{}, fmt.Errorf("list tool run request issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if toolRunRequestIssueMatches(issue.Body, req.RequestID) {
			return ToolRunRequestIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	title := fmt.Sprintf("GitClaw tool run request: %s", req.RequestID)
	issue, err := github.CreateIssue(ctx, req.Repo, title, RenderToolRunRequestIssueBody(req), nil)
	if err != nil {
		return ToolRunRequestIssueResult{}, fmt.Errorf("create tool run request issue: %w", err)
	}
	return ToolRunRequestIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RunToolRunRequestChannelNotification(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ToolRunRequestIssueRequest, result ToolRunRequestIssueResult) (ToolRunRequestChannelNotification, error) {
	notification := ToolRunRequestChannelNotification{
		Requested: len(req.NotifyRoutes) > 0,
		Routes:    len(req.NotifyRoutes),
	}
	if len(req.NotifyRoutes) == 0 {
		return notification, nil
	}
	if result.IssueNumber <= 0 {
		return notification, fmt.Errorf("missing tool run request issue for channel notification")
	}
	body := RenderToolRunRequestChannelNotificationBody(req, result)
	messageID := toolRunRequestChannelNotificationMessageID(req)
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, ChannelBroadcastOptions{
		Repo:      req.Repo,
		Routes:    req.NotifyRoutes,
		MessageID: messageID,
		Body:      body,
	})
	if err != nil {
		return notification, err
	}
	notification.Queued = broadcast.Queued
	notification.Duplicates = broadcast.Duplicates
	notification.TargetIssuesCreated = broadcast.Created
	notification.MessageSHA = shortDocumentHash(messageID)
	notification.BodySHA = shortDocumentHash(body)
	notification.BodyBytes = len(body)
	notification.BodyLines = lineCount(body)
	notification.Destinations = broadcast.Destinations
	return notification, nil
}

func RenderToolRunRequestIssueBody(req ToolRunRequestIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" normalized_tool=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", toolRunRequestIssueMarker, escapeMarkerValue(req.RequestID), escapeMarkerValue(req.NormalizedTool), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw tool run request issue.\n\n")
	fmt.Fprintf(&b, "- request_id: %s\n", req.RequestID)
	fmt.Fprintf(&b, "- normalized_tool: %s\n", valueOrNone(req.NormalizedTool))
	fmt.Fprintf(&b, "- matched_tool: %s\n", valueOrNone(req.MatchedTool))
	fmt.Fprintf(&b, "- matched_tools: %d\n", req.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: %d\n", req.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: %d\n", req.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: %t\n", req.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: %t\n", req.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: %t\n", req.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode: %s\n", valueOrNone(req.ToolMode))
	fmt.Fprintf(&b, "- mutating_contract: %t\n", req.MutatingContract)
	fmt.Fprintf(&b, "- approval_required: %t\n", req.ApprovalRequired)
	fmt.Fprintf(&b, "- run_allowed_now: %t\n", req.RunAllowedNow)
	fmt.Fprintf(&b, "- review_decision: %s\n", req.ReviewDecision)
	fmt.Fprintf(&b, "- tool_validation_status: %s\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: %d\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: %d\n", req.SourceLines)
	b.WriteString("- review_required: true\n")
	b.WriteString("- model_call_performed: false\n")
	b.WriteString("- tool_execution_performed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_tool_inputs_included: false\n")
	b.WriteString("- raw_tool_outputs_included: false\n")
	b.WriteString("- repository_mutation_performed: false\n\n")
	b.WriteString("Review this request before converting it into a normal model-backed issue turn or future approved tool-run workflow. GitClaw does not execute the requested tool from this issue.")
	return b.String()
}

func RenderToolRunRequestIssueActionReport(ev Event, req ToolRunRequestIssueRequest, result ToolRunRequestIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Tool Run Request Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_tool_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- tool_run_request_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_run_request_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- tool_run_request_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- tool_run_request_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- tool_run_request_id: `%s`\n", inlineCode(req.RequestID))
	fmt.Fprintf(&b, "- channel_notification_requested: `%t`\n", result.ChannelNotification.Requested)
	fmt.Fprintf(&b, "- channel_notification_routes: `%d`\n", result.ChannelNotification.Routes)
	fmt.Fprintf(&b, "- channel_notification_queued: `%d`\n", result.ChannelNotification.Queued)
	fmt.Fprintf(&b, "- channel_notification_duplicates: `%d`\n", result.ChannelNotification.Duplicates)
	fmt.Fprintf(&b, "- channel_notification_target_issues_created: `%d`\n", result.ChannelNotification.TargetIssuesCreated)
	fmt.Fprintf(&b, "- channel_notification_routes_sha256_12: `%s`\n", noneIfEmpty(req.NotifyRoutesSHA))
	fmt.Fprintf(&b, "- channel_notification_message_id_sha256_12: `%s`\n", noneIfEmpty(result.ChannelNotification.MessageSHA))
	fmt.Fprintf(&b, "- channel_notification_body_sha256_12: `%s`\n", noneIfEmpty(result.ChannelNotification.BodySHA))
	fmt.Fprintf(&b, "- channel_notification_body_bytes: `%d`\n", result.ChannelNotification.BodyBytes)
	fmt.Fprintf(&b, "- channel_notification_body_lines: `%d`\n", result.ChannelNotification.BodyLines)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", req.RequestedToolSHA)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- normalized_tool: `%s`\n", valueOrNone(req.NormalizedTool))
	fmt.Fprintf(&b, "- matched_tool: `%s`\n", valueOrNone(req.MatchedTool))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", req.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", req.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", req.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", req.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", req.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", req.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode: `%s`\n", valueOrNone(req.ToolMode))
	fmt.Fprintf(&b, "- tool_trigger_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.ToolTrigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", req.MutatingContract)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", req.ApprovalRequired)
	fmt.Fprintf(&b, "- run_allowed_now: `%t`\n", req.RunAllowedNow)
	fmt.Fprintf(&b, "- review_decision: `%s`\n", req.ReviewDecision)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", req.ValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", req.ValidationWarnings)
	fmt.Fprintf(&b, "- request_store: `%s`\n", "github-issue-to-reviewed-tool-run")
	fmt.Fprintf(&b, "- review_required: `%t`\n", true)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_routes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_notification_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_run_request_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for a reviewed tool-run request. The request is not execution: no tool is called, no model is called, no raw source text is copied, and no repository mutation is performed.\n\n")
	b.WriteString("### Review Path\n")
	fmt.Fprintf(&b, "- review request issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- if accepted, continue in a normal model-backed issue turn or future approved workflow\n")
	b.WriteString("- run `gitclaw tools verify`, `gitclaw tools risk`, and a live GitHub Models conversation E2E after changing tool behavior\n")
	if result.ChannelNotification.Requested {
		b.WriteString("\n### Channel Notifications\n")
		if len(result.ChannelNotification.Destinations) == 0 {
			b.WriteString("- none\n")
		} else {
			for _, destination := range result.ChannelNotification.Destinations {
				fmt.Fprintf(
					&b,
					"- destination=`%02d` target_issue=`#%d` outbound_comment_id=`%d` target_issue_created=`%t` duplicate_suppressed=`%t` route_sha256_12=`%s` channel=`%s` thread_id_sha256_12=`%s` message_id_sha256_12=`%s` body_sha256_12=`%s`\n",
					destination.Index,
					destination.IssueNumber,
					destination.CommentID,
					destination.Created,
					destination.Duplicate,
					noneIfEmpty(destination.RouteHash),
					destination.Channel,
					noneIfEmpty(destination.ThreadHash),
					noneIfEmpty(destination.MessageHash),
					noneIfEmpty(destination.BodyHash),
				)
			}
		}
		b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	}
	return strings.TrimSpace(b.String())
}

func parseToolRunRequestIssueArgs(args []string, sourceText string) (string, string, []string, error) {
	toolName := ""
	requestID := ""
	var notifyRoutes []string
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--tool", "-t":
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("--tool requires a value")
			}
			toolName = cleanToolLookupName(args[i])
		case "--id":
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("--id requires a value")
			}
			requestID = cleanToolRunRequestID(args[i])
		case "--notify-route", "--notify-routes", "--channel-route", "--channel-routes":
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("%s requires a value", field)
			}
			notifyRoutes = append(notifyRoutes, splitChannelBroadcastRoutes(args[i])...)
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", nil, fmt.Errorf("unknown tool run request argument %q", field)
			}
			if toolName == "" {
				toolName = cleanToolLookupName(field)
			}
		}
	}
	if requestID == "" {
		requestID = fmt.Sprintf("tool-run-%s", shortDocumentHash(sourceText))
	}
	return toolName, requestID, normalizeChannelBroadcastRoutes(notifyRoutes), nil
}

func RenderToolRunRequestChannelNotificationBody(req ToolRunRequestIssueRequest, result ToolRunRequestIssueResult) string {
	var b strings.Builder
	b.WriteString("GitClaw tool run request\n\n")
	fmt.Fprintf(&b, "Review issue: #%d %s\n", result.IssueNumber, result.IssueURL)
	fmt.Fprintf(&b, "Source issue: #%d %s\n", req.SourceIssueNumber, issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "Request id: %s\n", req.RequestID)
	fmt.Fprintf(&b, "Normalized tool: %s\n", valueOrNone(req.NormalizedTool))
	fmt.Fprintf(&b, "Review decision: %s\n", req.ReviewDecision)
	fmt.Fprintf(&b, "Run allowed now: %t\n", req.RunAllowedNow)
	b.WriteString("\nReview the GitHub request issue before converting this into a normal model-backed tool turn or future approved workflow. This notification did not execute a model, tool, shell command, or repository mutation.")
	return strings.TrimSpace(b.String())
}

func toolRunRequestChannelNotificationMessageID(req ToolRunRequestIssueRequest) string {
	return fmt.Sprintf("gitclaw-tool-request-%s", req.RequestID)
}

func cleanToolRunRequestID(id string) string {
	id = strings.ToLower(strings.Trim(strings.TrimSpace(id), " \t\r\n.,:;!?`\"'"))
	id = strings.ReplaceAll(id, "_", "-")
	return id
}

func toolRunRequestAllowed(req ToolRunRequestIssueRequest) bool {
	return req.MatchedTools == 1 &&
		req.ToolEnabled &&
		!req.DisabledByConfig &&
		!req.BlockedByAllowlist &&
		!req.MutatingContract &&
		req.ValidationErrors == 0
}

func toolRunRequestDecision(req ToolRunRequestIssueRequest) string {
	switch {
	case req.NormalizedTool == "":
		return "blocked_tool_missing"
	case req.MatchedTools == 0:
		return "blocked_tool_not_found"
	case req.MatchedTools > 1:
		return "blocked_tool_ambiguous"
	case req.DisabledByConfig:
		return "blocked_disabled_by_config"
	case req.BlockedByAllowlist:
		return "blocked_by_allowlist"
	case req.ValidationErrors > 0:
		return "blocked_tool_validation_errors"
	case req.MutatingContract:
		return "approval_required_future_write_mode"
	case !req.ToolEnabled:
		return "blocked_tool_not_enabled"
	default:
		return "review_required_read_only_tool"
	}
}

func toolRunRequestIssueMatches(body, requestID string) bool {
	return strings.Contains(body, "<!-- "+toolRunRequestIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(requestID)))
}

func toolRunRequestHashOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return shortDocumentHash(value)
}
