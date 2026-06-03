package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolResultOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ResultID          string
	ToolName          string
	Status            string
	ExitCode          string
	RecordedAt        string
	Summary           string
	Details           string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolResultResult struct {
	ResultIssueNumber int
	ResultIssueURL    string
	ResultCreated     bool
	ResultDuplicate   bool
	Notification      ChannelSendResult
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	MessageHash       string
	NotifyHash        string
}

type ChannelToolResultActionRequest struct {
	Options             ChannelToolResultOptions
	Command             string
	Subcommand          string
	AutoResultID        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	ToolSHA             string
	ToolBytes           int
	StatusSHA           string
	StatusBytes         int
	ExitCodeSHA         string
	RecordedAtSHA       string
	RecordedAtBytes     int
	RecordedAtLines     int
	SummarySHA          string
	SummaryBytes        int
	SummaryLines        int
	DetailsSHA          string
	DetailsBytes        int
	DetailsLines        int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelToolResultActionRequest(ev Event, cfg Config) bool {
	return isChannelToolResultActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolResultActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "tool-result", "tool-output", "tool-receipt", "tool-note", "run-result", "result-note":
		return true
	default:
		return false
	}
}

func BuildChannelToolResultActionRequest(ev Event, cfg Config) (ChannelToolResultActionRequest, error) {
	fields, trailing, ok := channelToolResultActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolResultActionRequest{}, fmt.Errorf("missing channel tool-result command")
	}
	req := ChannelToolResultActionRequest{
		Options: ChannelToolResultOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
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
				return ChannelToolResultActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--result-id", "--tool-result-id", "--tool-run-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ResultID = cleanChannelToolResultID(fields[i+1])
			i++
		case "--tool", "--tool-name", "-t":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ToolName = cleanToolLookupName(fields[i+1])
			i++
		case "--status", "--outcome":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Status = fields[i+1]
			i++
		case "--exit-code", "--code":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ExitCode = fields[i+1]
			i++
		case "--recorded-at", "--time", "--timestamp", "--date":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RecordedAt = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolResultActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolResultActionRequest{}, fmt.Errorf("unknown channel tool-result argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	if err := applyChannelToolResultPositionals(&req, positional); err != nil {
		return ChannelToolResultActionRequest{}, err
	}
	if err := applyChannelToolResultIssueTarget(ev, &req); err != nil {
		return ChannelToolResultActionRequest{}, err
	}
	parsed := parseChannelToolResultBody(trailing, ev)
	if strings.TrimSpace(req.Options.RecordedAt) == "" {
		req.Options.RecordedAt = parsed.RecordedAt
	}
	if strings.TrimSpace(req.Options.ToolName) == "" {
		req.Options.ToolName = parsed.ToolName
	}
	if strings.TrimSpace(req.Options.Status) == "" {
		req.Options.Status = parsed.Status
	}
	if strings.TrimSpace(req.Options.ExitCode) == "" {
		req.Options.ExitCode = parsed.ExitCode
	}
	req.Options.Summary = parsed.Summary
	req.Options.Details = parsed.Details
	if strings.TrimSpace(req.Options.ResultID) == "" {
		req.Options.ResultID = autoChannelToolResultID(ev, req.Options)
		req.AutoResultID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolResultNotifyMessageID(ev, req.Options.ResultID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolResultOptions(req.Options)
	if err := validateChannelToolResultActionRequestOptions(req.Options); err != nil {
		return ChannelToolResultActionRequest{}, err
	}
	req.ToolSHA = shortDocumentHash(req.Options.ToolName)
	req.ToolBytes = len(req.Options.ToolName)
	req.StatusSHA = shortDocumentHash(req.Options.Status)
	req.StatusBytes = len(req.Options.Status)
	req.ExitCodeSHA = shortDocumentHash(req.Options.ExitCode)
	req.RecordedAtSHA = shortDocumentHash(req.Options.RecordedAt)
	req.RecordedAtBytes = len(req.Options.RecordedAt)
	req.RecordedAtLines = lineCount(req.Options.RecordedAt)
	req.SummarySHA = shortDocumentHash(req.Options.Summary)
	req.SummaryBytes = len(req.Options.Summary)
	req.SummaryLines = lineCount(req.Options.Summary)
	req.DetailsSHA = shortDocumentHash(req.Options.Details)
	req.DetailsBytes = len(req.Options.Details)
	req.DetailsLines = lineCount(req.Options.Details)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelToolResultNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelToolResult(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolResultOptions) (ChannelToolResultResult, error) {
	opts = normalizeChannelToolResultOptions(opts)
	var err error
	opts, err = applyChannelToolResultRoute(cfg, opts)
	if err != nil {
		return ChannelToolResultResult{}, err
	}
	if err := validateChannelToolResultOptions(opts); err != nil {
		return ChannelToolResultResult{}, err
	}
	resultIssue, created, duplicate, err := findOrCreateChannelToolResultIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelToolResultResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelToolResultNotificationBody(opts, resultIssue.Number, issueURL(opts.Repo, resultIssue.Number)),
	})
	if err != nil {
		return ChannelToolResultResult{}, fmt.Errorf("queue channel tool-result notification: %w", err)
	}
	return ChannelToolResultResult{
		ResultIssueNumber: resultIssue.Number,
		ResultIssueURL:    issueURL(opts.Repo, resultIssue.Number),
		ResultCreated:     created,
		ResultDuplicate:   duplicate,
		Notification:      notification,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelToolResultActionReport(ev Event, req ChannelToolResultActionRequest, result ChannelToolResultResult) string {
	status := "recorded"
	switch {
	case result.ResultDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ResultDuplicate:
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
	b.WriteString("## GitClaw Channel Tool Result Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_result_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_result_issue: `#%d`\n", result.ResultIssueNumber)
	fmt.Fprintf(&b, "- tool_result_issue_url: `%s`\n", result.ResultIssueURL)
	fmt.Fprintf(&b, "- tool_result_issue_created: `%t`\n", result.ResultCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ResultDuplicate)
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
	fmt.Fprintf(&b, "- tool_result_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ResultID))
	fmt.Fprintf(&b, "- tool_result_id_auto: `%t`\n", req.AutoResultID)
	fmt.Fprintf(&b, "- tool_name_sha256_12: `%s`\n", req.ToolSHA)
	fmt.Fprintf(&b, "- tool_name_bytes: `%d`\n", req.ToolBytes)
	fmt.Fprintf(&b, "- tool_status_sha256_12: `%s`\n", req.StatusSHA)
	fmt.Fprintf(&b, "- tool_status_bytes: `%d`\n", req.StatusBytes)
	fmt.Fprintf(&b, "- tool_exit_code_sha256_12: `%s`\n", req.ExitCodeSHA)
	fmt.Fprintf(&b, "- recorded_at_sha256_12: `%s`\n", req.RecordedAtSHA)
	fmt.Fprintf(&b, "- recorded_at_bytes: `%d`\n", req.RecordedAtBytes)
	fmt.Fprintf(&b, "- recorded_at_lines: `%d`\n", req.RecordedAtLines)
	fmt.Fprintf(&b, "- tool_result_summary_sha256_12: `%s`\n", req.SummarySHA)
	fmt.Fprintf(&b, "- tool_result_summary_bytes: `%d`\n", req.SummaryBytes)
	fmt.Fprintf(&b, "- tool_result_summary_lines: `%d`\n", req.SummaryLines)
	fmt.Fprintf(&b, "- tool_result_details_sha256_12: `%s`\n", req.DetailsSHA)
	fmt.Fprintf(&b, "- tool_result_details_bytes: `%d`\n", req.DetailsBytes)
	fmt.Fprintf(&b, "- tool_result_details_lines: `%d`\n", req.DetailsLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_tool_result_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_status_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_exit_code_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_recorded_at_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_result_summary_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_result_details_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_result_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a channel-origin tool result as a durable GitHub issue, then queued a provider-facing link back to the original thread. This is a tool gateway receipt, not a tool executor: the source receipt keeps provider IDs, result IDs, tool names, statuses, output details, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the tool-result link with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool-result links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool-result issues are suppressed by `result_id`; duplicate tool-result notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the tool-result issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolResultIssueBody(opts ChannelToolResultOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-tool-result result_id=\"%s\" channel=\"%s\" tool_sha256_12=\"%s\" status_sha256_12=\"%s\" recorded_at_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ResultID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ToolName), shortDocumentHash(opts.Status), shortDocumentHash(opts.RecordedAt), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel tool result.\n\n")
	fmt.Fprintf(&b, "- result_id: %s\n", opts.ResultID)
	fmt.Fprintf(&b, "- tool_name: %s\n", opts.ToolName)
	fmt.Fprintf(&b, "- status: %s\n", opts.Status)
	if opts.ExitCode != "" {
		fmt.Fprintf(&b, "- exit_code: %s\n", opts.ExitCode)
	}
	fmt.Fprintf(&b, "- recorded_at: %s\n", opts.RecordedAt)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- tool_result_mode: github-issue-tool-result\n")
	fmt.Fprintf(&b, "- tool_execution_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Tool\n\n")
	b.WriteString(strings.TrimSpace(opts.ToolName))
	b.WriteString("\n\n## Status\n\n")
	b.WriteString(strings.TrimSpace(opts.Status))
	if strings.TrimSpace(opts.ExitCode) != "" {
		b.WriteString("\n\nExit code: ")
		b.WriteString(strings.TrimSpace(opts.ExitCode))
	}
	b.WriteString("\n\n## Recorded At\n\n")
	b.WriteString(strings.TrimSpace(opts.RecordedAt))
	b.WriteString("\n\n## Summary\n\n")
	b.WriteString(strings.TrimSpace(opts.Summary))
	if strings.TrimSpace(opts.Details) != "" {
		b.WriteString("\n\n## Result Details\n\n")
		b.WriteString(strings.TrimSpace(opts.Details))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the externally observed tool result. GitClaw did not execute the tool for this channel action.")
	return strings.TrimSpace(b.String())
}

func channelToolResultActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolResultActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolResultPositionals(req *ChannelToolResultActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		switch {
		case req.Options.ToolName == "":
			req.Options.ToolName = cleanToolLookupName(value)
		case req.Options.ResultID == "":
			req.Options.ResultID = cleanChannelToolResultID(value)
		case req.Options.Route == "" && req.Options.Channel == "":
			req.Options.Route = value
		default:
			return fmt.Errorf("unexpected channel tool-result argument %q", value)
		}
	}
	return nil
}

func applyChannelToolResultIssueTarget(ev Event, req *ChannelToolResultActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool-result requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

type channelToolResultParsedBody struct {
	ToolName   string
	Status     string
	ExitCode   string
	RecordedAt string
	Summary    string
	Details    string
}

func parseChannelToolResultBody(trailing string, ev Event) channelToolResultParsedBody {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	parsed := channelToolResultParsedBody{}
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "tool:"):
			if parsed.ToolName == "" {
				parsed.ToolName = cleanToolLookupName(trimmed[len("tool:"):])
			}
			continue
		case strings.HasPrefix(lower, "tool name:"):
			if parsed.ToolName == "" {
				parsed.ToolName = cleanToolLookupName(trimmed[len("tool name:"):])
			}
			continue
		case strings.HasPrefix(lower, "status:"):
			if parsed.Status == "" {
				parsed.Status = strings.TrimSpace(trimmed[len("status:"):])
			}
			continue
		case strings.HasPrefix(lower, "outcome:"):
			if parsed.Status == "" {
				parsed.Status = strings.TrimSpace(trimmed[len("outcome:"):])
			}
			continue
		case strings.HasPrefix(lower, "exit code:"):
			if parsed.ExitCode == "" {
				parsed.ExitCode = strings.TrimSpace(trimmed[len("exit code:"):])
			}
			continue
		case strings.HasPrefix(lower, "recorded at:"):
			if parsed.RecordedAt == "" {
				parsed.RecordedAt = strings.TrimSpace(trimmed[len("recorded at:"):])
			}
			continue
		case strings.HasPrefix(lower, "time:"):
			if parsed.RecordedAt == "" {
				parsed.RecordedAt = strings.TrimSpace(trimmed[len("time:"):])
			}
			continue
		case strings.HasPrefix(lower, "timestamp:"):
			if parsed.RecordedAt == "" {
				parsed.RecordedAt = strings.TrimSpace(trimmed[len("timestamp:"):])
			}
			continue
		case strings.HasPrefix(lower, "date:"):
			if parsed.RecordedAt == "" {
				parsed.RecordedAt = strings.TrimSpace(trimmed[len("date:"):])
			}
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultSummary := fmt.Sprintf("Tool result from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		parsed.Summary = defaultSummary
		return parsed
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var detailLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "summary:"):
		parsed.Summary = strings.TrimSpace(first[len("summary:"):])
		detailLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "title:"):
		parsed.Summary = strings.TrimSpace(first[len("title:"):])
		detailLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "tool result:"):
		parsed.Summary = strings.TrimSpace(first[len("tool result:"):])
		detailLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "result:"), strings.HasPrefix(lowerFirst, "output:"), strings.HasPrefix(lowerFirst, "details:"), strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"):
		parsed.Summary = defaultSummary
		detailLines = cleaned
	default:
		parsed.Summary = first
		detailLines = cleaned[1:]
	}
	if parsed.Summary == "" {
		parsed.Summary = defaultSummary
	}
	details := strings.TrimSpace(strings.Join(detailLines, "\n"))
	detailsLower := strings.ToLower(strings.TrimSpace(details))
	switch {
	case strings.HasPrefix(detailsLower, "result:"):
		details = strings.TrimSpace(strings.TrimSpace(details)[len("result:"):])
	case strings.HasPrefix(detailsLower, "output:"):
		details = strings.TrimSpace(strings.TrimSpace(details)[len("output:"):])
	case strings.HasPrefix(detailsLower, "details:"):
		details = strings.TrimSpace(strings.TrimSpace(details)[len("details:"):])
	case strings.HasPrefix(detailsLower, "notes:"):
		details = strings.TrimSpace(strings.TrimSpace(details)[len("notes:"):])
	case strings.HasPrefix(detailsLower, "context:"):
		details = strings.TrimSpace(strings.TrimSpace(details)[len("context:"):])
	}
	parsed.Details = details
	return parsed
}

func normalizeChannelToolResultOptions(opts ChannelToolResultOptions) ChannelToolResultOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ResultID = cleanChannelToolResultID(opts.ResultID)
	opts.ToolName = cleanToolLookupName(opts.ToolName)
	opts.Status = strings.ToLower(strings.TrimSpace(opts.Status))
	opts.ExitCode = strings.TrimSpace(opts.ExitCode)
	opts.RecordedAt = strings.TrimSpace(opts.RecordedAt)
	opts.Summary = strings.TrimSpace(opts.Summary)
	opts.Details = strings.TrimSpace(opts.Details)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToolResultRoute(cfg Config, opts ChannelToolResultOptions) (ChannelToolResultOptions, error) {
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
		Body:      opts.Summary,
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

func validateChannelToolResultOptions(opts ChannelToolResultOptions) error {
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
	if opts.ResultID == "" {
		return fmt.Errorf("missing tool-result id")
	}
	if opts.ToolName == "" {
		return fmt.Errorf("missing tool name")
	}
	if opts.Status == "" {
		return fmt.Errorf("missing tool-result status")
	}
	if opts.RecordedAt == "" {
		return fmt.Errorf("missing tool-result recorded timestamp")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing tool-result source issue")
	}
	if opts.Summary == "" {
		return fmt.Errorf("missing tool-result summary")
	}
	return nil
}

func validateChannelToolResultActionRequestOptions(opts ChannelToolResultOptions) error {
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
	if opts.ResultID == "" {
		return fmt.Errorf("missing tool-result id")
	}
	if opts.ToolName == "" {
		return fmt.Errorf("missing tool name")
	}
	if opts.Status == "" {
		return fmt.Errorf("missing tool-result status")
	}
	if opts.RecordedAt == "" {
		return fmt.Errorf("missing tool-result recorded timestamp")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing tool-result source issue")
	}
	if opts.Summary == "" {
		return fmt.Errorf("missing tool-result summary")
	}
	return nil
}

func findOrCreateChannelToolResultIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolResultOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel tool-result issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelToolResultMatches(issue.Body, opts.ResultID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelToolResultIssueTitle(opts), RenderChannelToolResultIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel tool-result issue: %w", err)
	}
	return issue, true, false, nil
}

func channelToolResultIssueTitle(opts ChannelToolResultOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Summary), "\n", " ")
	if title == "" {
		title = opts.ResultID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel tool result: " + title
}

func channelToolResultMatches(body, resultID string) bool {
	return HasChannelToolResultMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`result_id="%s"`, escapeMarkerValue(cleanChannelToolResultID(resultID))))
}

func cleanChannelToolResultID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelToolResultID(ev Event, opts ChannelToolResultOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.ToolName, opts.Status, opts.RecordedAt, opts.Summary, opts.Details}, "|")
	return fmt.Sprintf("tool-result-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolResultNotifyMessageID(ev Event, resultID string) string {
	seed := strings.Join([]string{eventID(ev), resultID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-result-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelToolResultNotificationBody(opts ChannelToolResultOptions, resultIssueNumber int, resultIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool result recorded.\n\n")
	if resultIssueNumber > 0 {
		fmt.Fprintf(&b, "Tool result: #%d\n", resultIssueNumber)
	}
	if resultIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", resultIssueURL)
	}
	fmt.Fprintf(&b, "Tool: %s\n", strings.TrimSpace(opts.ToolName))
	fmt.Fprintf(&b, "Status: %s\n", strings.TrimSpace(opts.Status))
	fmt.Fprintf(&b, "Summary: %s\n", strings.TrimSpace(opts.Summary))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
