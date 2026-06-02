package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSessionHandoffOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	HandoffID         string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSessionHandoffResult struct {
	Handoff             SessionHandoffIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	HandoffHash         string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSessionHandoffActionRequest struct {
	Options             ChannelSessionHandoffOptions
	Handoff             SessionHandoffIssueRequest
	Command             string
	Subcommand          string
	AutoHandoffID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSessionHandoffActionRequest(ev Event, cfg Config) bool {
	return isChannelSessionHandoffActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSessionHandoffActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanSessionHandoffCommandName(fields[1]) {
	case "handoff", "session-handoff", "handoff-session", "fork-session", "session-fork", "new-session", "new-issue", "continue-github":
		return true
	default:
		return false
	}
}

func BuildChannelSessionHandoffActionRequest(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) (ChannelSessionHandoffActionRequest, error) {
	fields, ok := channelSessionHandoffActionFields(ev, cfg)
	if !ok {
		return ChannelSessionHandoffActionRequest{}, fmt.Errorf("missing channel session handoff command")
	}
	req := ChannelSessionHandoffActionRequest{
		Options: ChannelSessionHandoffOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: cleanSessionHandoffCommandName(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--id", "--handoff-id":
			if i+1 >= len(fields) {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.HandoffID = cleanSessionHandoffID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSessionHandoffActionRequest{}, fmt.Errorf("unknown channel session handoff argument %q", field)
			}
			if req.Options.HandoffID == "" {
				req.Options.HandoffID = cleanSessionHandoffID(field)
				continue
			}
			return ChannelSessionHandoffActionRequest{}, fmt.Errorf("unexpected channel session handoff argument %q", field)
		}
	}
	if err := applyChannelSessionHandoffIssueTarget(ev, &req); err != nil {
		return ChannelSessionHandoffActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.HandoffID) == "" {
		req.Options.HandoffID = autoChannelSessionHandoffID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID)
		req.AutoHandoffID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSessionHandoffNotifyMessageID(ev, req.Options.HandoffID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSessionHandoffOptions(req.Options)
	if err := validateChannelSessionHandoffOptions(req.Options); err != nil {
		return ChannelSessionHandoffActionRequest{}, err
	}
	handoff, err := buildSessionHandoffIssueRequestFromChannel(ev, cfg, comments, transcript, req.Options)
	if err != nil {
		return ChannelSessionHandoffActionRequest{}, err
	}
	req.Handoff = handoff
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelSessionHandoffNotificationBody(req.Options, SessionHandoffIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, handoff)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSessionHandoff(ctx context.Context, cfg Config, github interface {
	SessionHandoffIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelSessionHandoffActionRequest) (ChannelSessionHandoffResult, error) {
	handoffResult, err := RunSessionHandoffIssue(ctx, cfg, github, req.Handoff)
	if err != nil {
		return ChannelSessionHandoffResult{}, err
	}
	notificationBody := RenderChannelSessionHandoffNotificationBody(req.Options, handoffResult, req.Handoff)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelSessionHandoffResult{}, fmt.Errorf("queue channel session handoff notification: %w", err)
	}
	return ChannelSessionHandoffResult{
		Handoff:             handoffResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		HandoffHash:         shortDocumentHash(req.Options.HandoffID),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelSessionHandoffActionReport(ev Event, req ChannelSessionHandoffActionRequest, result ChannelSessionHandoffResult) string {
	status := "created"
	switch {
	case result.Handoff.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.Handoff.Duplicate:
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
	report := req.Handoff.Resume
	var b strings.Builder
	b.WriteString("## GitClaw Channel Session Handoff Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Handoff.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Handoff.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_session_handoff_status: `%s`\n", status)
	fmt.Fprintf(&b, "- handoff_issue: `#%d`\n", result.Handoff.IssueNumber)
	fmt.Fprintf(&b, "- handoff_issue_url: `%s`\n", result.Handoff.IssueURL)
	fmt.Fprintf(&b, "- handoff_issue_created: `%t`\n", result.Handoff.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Handoff.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- handoff_id_sha256_12: `%s`\n", result.HandoffHash)
	fmt.Fprintf(&b, "- handoff_id_auto: `%t`\n", req.AutoHandoffID)
	fmt.Fprintf(&b, "- handoff_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- handoff_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- source_session_store: `%s`\n", "github-issue-thread")
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(&b, "- usage_total_tokens: `%d`\n", report.UsageTotalTokens)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- next_issue_comment_resumes_handoff: `%t`\n", true)
	fmt.Fprintf(&b, "- source_issue_continuation_supported: `%t`\n", true)
	fmt.Fprintf(&b, "- github_actions_reentry_supported: `%t`\n", true)
	fmt.Fprintf(&b, "- workflow_event: `%s`\n", "issue_comment")
	fmt.Fprintf(&b, "- workflow_dispatch_required: `%t`\n", false)
	fmt.Fprintf(&b, "- server_required: `%t`\n", false)
	fmt.Fprintf(&b, "- socket_required: `%t`\n", false)
	fmt.Fprintf(&b, "- external_session_db_required: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_replay_preferred: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_handoff_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Handoff.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Handoff.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Handoff.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_session_handoff_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a labeled GitHub issue for continuing a mirrored channel session in a new GitHub lane, then queued a provider-facing handoff link back to that channel thread. This action does not copy raw channel bodies, issue bodies, comments, assistant replies, prompts, or tool outputs.\n\n")
	b.WriteString("### Handoff Path\n")
	fmt.Fprintf(&b, "- continue on handoff issue: `#%d`\n", result.Handoff.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the handoff issue to exercise model, skill, tool, and usage telemetry\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSessionHandoffNotificationBody(opts ChannelSessionHandoffOptions, result SessionHandoffIssueResult, handoff SessionHandoffIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel session handoff\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Handoff issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	report := handoff.Resume
	fmt.Fprintf(&b, "Transcript messages: %d\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "Assistant turns: %d\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "Model-backed turns: %d\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "Usage-bearing turns: %d\n", report.UsageBearingAssistantTurns)
	b.WriteString("\nContinue in the linked GitHub issue with a normal `@gitclaw` message. This notification did not execute a model, tool, shell command, or repository mutation.")
	return strings.TrimSpace(b.String())
}

func channelSessionHandoffActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelSessionHandoffActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelSessionHandoffIssueTarget(ev Event, req *ChannelSessionHandoffActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel session handoff requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSessionHandoffOptions(opts ChannelSessionHandoffOptions) ChannelSessionHandoffOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.HandoffID = cleanSessionHandoffID(opts.HandoffID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelSessionHandoffOptions(opts ChannelSessionHandoffOptions) error {
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
	if opts.HandoffID == "" {
		return fmt.Errorf("missing session handoff id")
	}
	if !skillNamePattern.MatchString(opts.HandoffID) {
		return fmt.Errorf("invalid session handoff id %q", opts.HandoffID)
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildSessionHandoffIssueRequestFromChannel(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, opts ChannelSessionHandoffOptions) (SessionHandoffIssueRequest, error) {
	sourceText := activeRequestText(ev)
	resume := BuildSessionResumeReport("issue-thread", "", ev, cfg, comments, transcript)
	resume.ActiveCommand = strings.Join(activeSlashCommandFields(ev, cfg), " ")
	return SessionHandoffIssueRequest{
		Repo:              ev.Repo,
		Command:           "/session",
		Subcommand:        "handoff",
		HandoffID:         opts.HandoffID,
		SourceIssueNumber: opts.SourceIssueNumber,
		SourceCommentID:   opts.SourceCommentID,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "channel_comment",
		Resume:            resume,
	}, nil
}

func autoChannelSessionHandoffID(ev Event, channel, threadID, sourceMessageID string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID}, "|")
	return cleanSessionHandoffID(fmt.Sprintf("session-handoff-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSessionHandoffNotifyMessageID(ev Event, handoffID string) string {
	seed := strings.Join([]string{eventID(ev), handoffID}, "|")
	return fmt.Sprintf("gitclaw-channel-session-handoff-%s-%s", eventID(ev), shortDocumentHash(seed))
}
