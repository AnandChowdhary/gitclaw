package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolRehearsalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RehearsalID       string
	RequestedTool     string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolRehearsalResult struct {
	Rehearsal           ToolRehearsalIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	RehearsalHash       string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelToolRehearsalActionRequest struct {
	Options             ChannelToolRehearsalOptions
	Rehearsal           ToolRehearsalIssueRequest
	Command             string
	Subcommand          string
	AutoRehearsalID     bool
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

func IsChannelToolRehearsalActionRequest(ev Event, cfg Config) bool {
	return isChannelToolRehearsalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolRehearsalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse-tool", "tool-rehearse", "tool-rehearsal", "try-tool", "tool-trial", "practice-tool", "tool-lab", "tool-sandbox":
		return true
	default:
		return false
	}
}

func BuildChannelToolRehearsalActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolRehearsalActionRequest, error) {
	fields, ok := channelToolRehearsalActionFields(ev, cfg)
	if !ok {
		return ChannelToolRehearsalActionRequest{}, fmt.Errorf("missing channel tool rehearsal command")
	}
	req := ChannelToolRehearsalActionRequest{
		Options: ChannelToolRehearsalOptions{
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
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--tool", "-t":
			if i+1 >= len(fields) {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("--tool requires a value")
			}
			req.Options.RequestedTool = cleanToolLookupName(fields[i+1])
			i++
		case "--id", "--rehearsal-id":
			if i+1 >= len(fields) {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RehearsalID = cleanToolRehearsalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolRehearsalActionRequest{}, fmt.Errorf("unknown channel tool rehearsal argument %q", field)
			}
			if req.Options.RequestedTool == "" {
				req.Options.RequestedTool = cleanToolLookupName(field)
				continue
			}
			if req.Options.RehearsalID == "" {
				req.Options.RehearsalID = cleanToolRehearsalID(field)
				continue
			}
			return ChannelToolRehearsalActionRequest{}, fmt.Errorf("unexpected channel tool rehearsal argument %q", field)
		}
	}
	if err := applyChannelToolRehearsalIssueTarget(ev, &req); err != nil {
		return ChannelToolRehearsalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RehearsalID) == "" {
		req.Options.RehearsalID = autoChannelToolRehearsalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.RequestedTool)
		req.AutoRehearsalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolRehearsalNotifyMessageID(ev, req.Options.RehearsalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolRehearsalOptions(req.Options)
	if err := validateChannelToolRehearsalOptions(req.Options); err != nil {
		return ChannelToolRehearsalActionRequest{}, err
	}
	rehearsal, err := buildToolRehearsalIssueRequestFromChannel(ev, repoContext, req.Options)
	if err != nil {
		return ChannelToolRehearsalActionRequest{}, err
	}
	req.Rehearsal = rehearsal
	req.RequestedToolSHA = rehearsal.RequestedToolSHA
	req.RequestedToolTerms = rehearsal.RequestedToolTerms
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelToolRehearsalNotificationBody(req.Options, ToolRehearsalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, rehearsal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelToolRehearsal(ctx context.Context, cfg Config, github interface {
	ToolRehearsalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelToolRehearsalActionRequest) (ChannelToolRehearsalResult, error) {
	rehearsalResult, err := RunToolRehearsalIssue(ctx, cfg, github, req.Rehearsal)
	if err != nil {
		return ChannelToolRehearsalResult{}, err
	}
	notificationBody := RenderChannelToolRehearsalNotificationBody(req.Options, rehearsalResult, req.Rehearsal)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelToolRehearsalResult{}, fmt.Errorf("queue channel tool rehearsal notification: %w", err)
	}
	return ChannelToolRehearsalResult{
		Rehearsal:           rehearsalResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		RehearsalHash:       shortDocumentHash(req.Options.RehearsalID),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelToolRehearsalActionReport(ev Event, req ChannelToolRehearsalActionRequest, result ChannelToolRehearsalResult) string {
	status := "created"
	switch {
	case result.Rehearsal.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.Rehearsal.Duplicate:
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
	b.WriteString("## GitClaw Channel Tool Rehearsal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Rehearsal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Rehearsal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.Rehearsal.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.Rehearsal.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Rehearsal.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Rehearsal.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- rehearsal_id_sha256_12: `%s`\n", result.RehearsalHash)
	fmt.Fprintf(&b, "- rehearsal_id_auto: `%t`\n", req.AutoRehearsalID)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", req.RequestedToolSHA)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- normalized_tool: `%s`\n", valueOrNone(req.Rehearsal.NormalizedTool))
	fmt.Fprintf(&b, "- matched_tool: `%s`\n", valueOrNone(req.Rehearsal.MatchedTool))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", req.Rehearsal.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", req.Rehearsal.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", req.Rehearsal.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", req.Rehearsal.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", req.Rehearsal.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", req.Rehearsal.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode: `%s`\n", valueOrNone(req.Rehearsal.ToolMode))
	fmt.Fprintf(&b, "- tool_trigger_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.Rehearsal.ToolTrigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", req.Rehearsal.MutatingContract)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", req.Rehearsal.ValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", req.Rehearsal.ValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", req.Rehearsal.ValidationWarnings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_inputs_generated: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_run_request_created: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rehearsal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_tool_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Rehearsal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Rehearsal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Rehearsal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_rehearsal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing a reviewed tool contract from a mirrored channel thread, then queued a provider-facing rehearsal link back to that thread. This action does not call a model, execute a tool, generate tool inputs, create a run request, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.Rehearsal.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the rehearsal issue to exercise prompt-visible tool behavior\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolRehearsalNotificationBody(opts ChannelToolRehearsalOptions, result ToolRehearsalIssueResult, rehearsal ToolRehearsalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool rehearsal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Rehearsal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Normalized tool: %s\n", valueOrNone(rehearsal.NormalizedTool))
	fmt.Fprintf(&b, "Matched tool: %s\n", valueOrNone(rehearsal.MatchedTool))
	fmt.Fprintf(&b, "Tool enabled: %t\n", rehearsal.ToolEnabled)
	fmt.Fprintf(&b, "Tool mode: %s\n", valueOrNone(rehearsal.ToolMode))
	fmt.Fprintf(&b, "Mutating contract: %t\n", rehearsal.MutatingContract)
	b.WriteString("\nContinue in the linked GitHub issue to rehearse the tool contract with a normal model-backed conversation. This notification did not execute a model, tool, shell command, or repository mutation.")
	return strings.TrimSpace(b.String())
}

func channelToolRehearsalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelToolRehearsalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelToolRehearsalIssueTarget(ev Event, req *ChannelToolRehearsalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool rehearsal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelToolRehearsalOptions(opts ChannelToolRehearsalOptions) ChannelToolRehearsalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RehearsalID = cleanToolRehearsalID(opts.RehearsalID)
	opts.RequestedTool = cleanToolLookupName(opts.RequestedTool)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelToolRehearsalOptions(opts ChannelToolRehearsalOptions) error {
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
	if opts.RehearsalID == "" {
		return fmt.Errorf("missing tool rehearsal id")
	}
	if !skillNamePattern.MatchString(opts.RehearsalID) {
		return fmt.Errorf("invalid tool rehearsal id %q", opts.RehearsalID)
	}
	if opts.RequestedTool == "" {
		return fmt.Errorf("missing requested tool")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildToolRehearsalIssueRequestFromChannel(ev Event, repoContext RepoContext, opts ChannelToolRehearsalOptions) (ToolRehearsalIssueRequest, error) {
	requestedTool := cleanToolLookupName(opts.RequestedTool)
	if requestedTool == "" {
		return ToolRehearsalIssueRequest{}, fmt.Errorf("missing tool rehearsal target")
	}
	normalized := normalizeToolLookupName(requestedTool)
	matches := matchingToolContracts(toolReportContracts, requestedTool)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	sourceText := activeRequestText(ev)
	req := ToolRehearsalIssueRequest{
		Repo:                 ev.Repo,
		Command:              "/tools",
		Subcommand:           "rehearse",
		RehearsalID:          opts.RehearsalID,
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
	return req, nil
}

func autoChannelToolRehearsalID(ev Event, channel, threadID, sourceMessageID, requestedTool string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, requestedTool}, "|")
	return cleanToolRehearsalID(fmt.Sprintf("tool-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelToolRehearsalNotifyMessageID(ev Event, rehearsalID string) string {
	seed := strings.Join([]string{eventID(ev), rehearsalID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
