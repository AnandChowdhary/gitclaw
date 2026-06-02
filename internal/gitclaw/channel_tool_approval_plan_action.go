package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolApprovalPlanOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ApprovalPlanID    string
	RequestedTool     string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolApprovalPlanResult struct {
	ApprovalPlan        ToolApprovalPlanIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	ApprovalPlanHash    string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelToolApprovalPlanActionRequest struct {
	Options             ChannelToolApprovalPlanOptions
	ApprovalPlan        ToolApprovalPlanIssueRequest
	Command             string
	Subcommand          string
	AutoApprovalPlanID  bool
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

func IsChannelToolApprovalPlanActionRequest(ev Event, cfg Config) bool {
	return isChannelToolApprovalPlanActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolApprovalPlanActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "approval-plan", "tool-approval", "approve-plan", "approval-gate", "tool-gate", "approve-tool":
		return true
	default:
		return false
	}
}

func BuildChannelToolApprovalPlanActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolApprovalPlanActionRequest, error) {
	fields, ok := channelToolApprovalPlanActionFields(ev, cfg)
	if !ok {
		return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("missing channel tool approval plan command")
	}
	req := ChannelToolApprovalPlanActionRequest{
		Options: ChannelToolApprovalPlanOptions{
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
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--tool", "-t":
			if i+1 >= len(fields) {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("--tool requires a value")
			}
			req.Options.RequestedTool = cleanToolLookupName(fields[i+1])
			i++
		case "--id", "--approval-plan-id", "--plan-id":
			if i+1 >= len(fields) {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ApprovalPlanID = cleanToolApprovalPlanID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("unknown channel tool approval plan argument %q", field)
			}
			if req.Options.RequestedTool == "" {
				req.Options.RequestedTool = cleanToolLookupName(field)
				continue
			}
			if req.Options.ApprovalPlanID == "" {
				req.Options.ApprovalPlanID = cleanToolApprovalPlanID(field)
				continue
			}
			return ChannelToolApprovalPlanActionRequest{}, fmt.Errorf("unexpected channel tool approval plan argument %q", field)
		}
	}
	if err := applyChannelToolApprovalPlanIssueTarget(ev, &req); err != nil {
		return ChannelToolApprovalPlanActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.ApprovalPlanID) == "" {
		req.Options.ApprovalPlanID = autoChannelToolApprovalPlanID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.RequestedTool)
		req.AutoApprovalPlanID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolApprovalPlanNotifyMessageID(ev, req.Options.ApprovalPlanID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolApprovalPlanOptions(req.Options)
	if err := validateChannelToolApprovalPlanOptions(req.Options); err != nil {
		return ChannelToolApprovalPlanActionRequest{}, err
	}
	approvalPlan, err := BuildToolApprovalPlanIssueRequest(ev, cfg, repoContext, req.Options.RequestedTool, req.Options.ApprovalPlanID, "channel_comment")
	if err != nil {
		return ChannelToolApprovalPlanActionRequest{}, err
	}
	req.ApprovalPlan = approvalPlan
	req.RequestedToolSHA = approvalPlan.RequestedToolSHA
	req.RequestedToolTerms = approvalPlan.RequestedToolTerms
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelToolApprovalPlanNotificationBody(req.Options, ToolApprovalPlanIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, approvalPlan)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelToolApprovalPlan(ctx context.Context, cfg Config, repoContext RepoContext, github interface {
	ToolApprovalPlanIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelToolApprovalPlanActionRequest) (ChannelToolApprovalPlanResult, error) {
	approvalPlanResult, err := RunToolApprovalPlanIssue(ctx, cfg, github, req.ApprovalPlan, repoContext)
	if err != nil {
		return ChannelToolApprovalPlanResult{}, err
	}
	notificationBody := RenderChannelToolApprovalPlanNotificationBody(req.Options, approvalPlanResult, req.ApprovalPlan)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelToolApprovalPlanResult{}, fmt.Errorf("queue channel tool approval plan notification: %w", err)
	}
	return ChannelToolApprovalPlanResult{
		ApprovalPlan:        approvalPlanResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		ApprovalPlanHash:    shortDocumentHash(req.Options.ApprovalPlanID),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelToolApprovalPlanActionReport(ev Event, req ChannelToolApprovalPlanActionRequest, result ChannelToolApprovalPlanResult) string {
	status := "created"
	switch {
	case result.ApprovalPlan.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ApprovalPlan.Duplicate:
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Tool Approval Plan Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.ApprovalPlan.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.ApprovalPlan.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_approval_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- approval_plan_issue: `#%d`\n", result.ApprovalPlan.IssueNumber)
	fmt.Fprintf(&b, "- approval_plan_issue_url: `%s`\n", result.ApprovalPlan.IssueURL)
	fmt.Fprintf(&b, "- approval_plan_issue_created: `%t`\n", result.ApprovalPlan.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ApprovalPlan.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- approval_plan_id_sha256_12: `%s`\n", result.ApprovalPlanHash)
	fmt.Fprintf(&b, "- approval_plan_id_auto: `%t`\n", req.AutoApprovalPlanID)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", req.RequestedToolSHA)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", req.ApprovalPlan.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", req.ApprovalPlan.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", req.ApprovalPlan.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", req.ApprovalPlan.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", req.ApprovalPlan.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", req.ApprovalPlan.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.ApprovalPlan.ToolMode))
	fmt.Fprintf(&b, "- tool_trigger_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.ApprovalPlan.ToolTrigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", req.ApprovalPlan.MutatingContract)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", req.ApprovalPlan.ApprovalRequired)
	fmt.Fprintf(&b, "- approval_decision: `%s`\n", req.ApprovalPlan.ApprovalDecision)
	fmt.Fprintf(&b, "- run_allowed_now: `%t`\n", req.ApprovalPlan.RunAllowedNow)
	fmt.Fprintf(&b, "- approval_plan_status: `%s`\n", req.ApprovalPlan.PlanStatus)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", req.ApprovalPlan.ValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", req.ApprovalPlan.ValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", req.ApprovalPlan.ValidationWarnings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- approval_mode: `%s`\n", "github-issue-dry-run")
	fmt.Fprintf(&b, "- approval_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_granted: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_approval_plan_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_tool_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_approval_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.ApprovalPlan.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.ApprovalPlan.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.ApprovalPlan.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_approval_plan_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a tool approval dry-run from a mirrored channel thread, then queued a provider-facing approval-plan link back to that thread. This action does not approve, call a model, execute tools, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Approval Review Path\n")
	fmt.Fprintf(&b, "- continue on approval plan issue: `#%d`\n", result.ApprovalPlan.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the approval issue to exercise prompt-visible tool behavior\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolApprovalPlanNotificationBody(opts ChannelToolApprovalPlanOptions, result ToolApprovalPlanIssueResult, approvalPlan ToolApprovalPlanIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool approval plan\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Approval plan issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Normalized tool: %s\n", valueOrNone(approvalPlan.NormalizedTool))
	fmt.Fprintf(&b, "Matched tool: %s\n", valueOrNone(approvalPlan.MatchedTool))
	fmt.Fprintf(&b, "Tool enabled: %t\n", approvalPlan.ToolEnabled)
	fmt.Fprintf(&b, "Tool mode: %s\n", valueOrNone(approvalPlan.ToolMode))
	fmt.Fprintf(&b, "Approval required: %t\n", approvalPlan.ApprovalRequired)
	fmt.Fprintf(&b, "Run allowed now: %t\n", approvalPlan.RunAllowedNow)
	b.WriteString("\nContinue in the linked GitHub issue to review the dry-run approval gates with a normal model-backed conversation. This notification did not approve, execute a model, tool, shell command, or repository mutation.")
	return strings.TrimSpace(b.String())
}

func channelToolApprovalPlanActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelToolApprovalPlanActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelToolApprovalPlanIssueTarget(ev Event, req *ChannelToolApprovalPlanActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool approval plan requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelToolApprovalPlanOptions(opts ChannelToolApprovalPlanOptions) ChannelToolApprovalPlanOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ApprovalPlanID = cleanToolApprovalPlanID(opts.ApprovalPlanID)
	opts.RequestedTool = cleanToolLookupName(opts.RequestedTool)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelToolApprovalPlanOptions(opts ChannelToolApprovalPlanOptions) error {
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
	if opts.ApprovalPlanID == "" {
		return fmt.Errorf("missing tool approval plan id")
	}
	if !skillNamePattern.MatchString(opts.ApprovalPlanID) {
		return fmt.Errorf("invalid tool approval plan id %q", opts.ApprovalPlanID)
	}
	if opts.RequestedTool == "" {
		return fmt.Errorf("missing requested tool")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func autoChannelToolApprovalPlanID(ev Event, channel, threadID, sourceMessageID, requestedTool string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, requestedTool}, "|")
	return cleanToolApprovalPlanID(fmt.Sprintf("tool-approval-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelToolApprovalPlanNotifyMessageID(ev Event, approvalPlanID string) string {
	seed := strings.Join([]string{eventID(ev), approvalPlanID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-approval-%s-%s", eventID(ev), shortDocumentHash(seed))
}
