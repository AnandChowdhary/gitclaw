package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelMemoryRehearsalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RehearsalID       string
	Target            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelMemoryRehearsalResult struct {
	Rehearsal           MemoryRehearsalIssueResult
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

type ChannelMemoryRehearsalActionRequest struct {
	Options              ChannelMemoryRehearsalOptions
	Rehearsal            MemoryRehearsalIssueRequest
	Command              string
	Subcommand           string
	AutoRehearsalID      bool
	AutoNotifyMessageID  bool
	TargetFromIssue      bool
	RequestedTargetSHA   string
	RequestedTargetTerms int
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	NotificationBodySHA  string
	NotificationBytes    int
	NotificationLines    int
}

func IsChannelMemoryRehearsalActionRequest(ev Event, cfg Config) bool {
	return isChannelMemoryRehearsalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelMemoryRehearsalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse-memory", "memory-rehearse", "memory-rehearsal", "try-memory", "memory-trial", "practice-memory", "recall-test", "memory-test":
		return true
	default:
		return false
	}
}

func BuildChannelMemoryRehearsalActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelMemoryRehearsalActionRequest, error) {
	fields, ok := channelMemoryRehearsalActionFields(ev, cfg)
	if !ok {
		return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("missing channel memory rehearsal command")
	}
	req := ChannelMemoryRehearsalActionRequest{
		Options: ChannelMemoryRehearsalOptions{
			Repo:              ev.Repo,
			Target:            "long-term",
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	targetSet := false
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--target", "--memory-target":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Target = cleanMemoryPromoteTarget(fields[i+1])
			targetSet = true
			i++
		case "--id", "--rehearsal-id":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RehearsalID = cleanMemoryRehearsalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("unknown channel memory rehearsal argument %q", field)
			}
			if !targetSet {
				req.Options.Target = cleanMemoryPromoteTarget(field)
				targetSet = true
				continue
			}
			if req.Options.RehearsalID == "" {
				req.Options.RehearsalID = cleanMemoryRehearsalID(field)
				continue
			}
			return ChannelMemoryRehearsalActionRequest{}, fmt.Errorf("unexpected channel memory rehearsal argument %q", field)
		}
	}
	if err := applyChannelMemoryRehearsalIssueTarget(ev, &req); err != nil {
		return ChannelMemoryRehearsalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RehearsalID) == "" {
		req.Options.RehearsalID = autoChannelMemoryRehearsalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Target)
		req.AutoRehearsalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMemoryRehearsalNotifyMessageID(ev, req.Options.RehearsalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMemoryRehearsalOptions(req.Options)
	if err := validateChannelMemoryRehearsalOptions(req.Options); err != nil {
		return ChannelMemoryRehearsalActionRequest{}, err
	}
	rehearsal, err := buildMemoryRehearsalIssueRequestFromChannel(ev, cfg, repoContext, req.Options)
	if err != nil {
		return ChannelMemoryRehearsalActionRequest{}, err
	}
	req.Rehearsal = rehearsal
	req.RequestedTargetSHA = shortDocumentHash(req.Options.Target)
	req.RequestedTargetTerms = len(memorySearchTerms(req.Options.Target))
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelMemoryRehearsalNotificationBody(req.Options, MemoryRehearsalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, rehearsal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelMemoryRehearsal(ctx context.Context, cfg Config, github interface {
	MemoryRehearsalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelMemoryRehearsalActionRequest) (ChannelMemoryRehearsalResult, error) {
	rehearsalResult, err := RunMemoryRehearsalIssue(ctx, cfg, github, req.Rehearsal)
	if err != nil {
		return ChannelMemoryRehearsalResult{}, err
	}
	notificationBody := RenderChannelMemoryRehearsalNotificationBody(req.Options, rehearsalResult, req.Rehearsal)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelMemoryRehearsalResult{}, fmt.Errorf("queue channel memory rehearsal notification: %w", err)
	}
	return ChannelMemoryRehearsalResult{
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

func RenderChannelMemoryRehearsalActionReport(ev Event, req ChannelMemoryRehearsalActionRequest, result ChannelMemoryRehearsalResult) string {
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
	b.WriteString("## GitClaw Channel Memory Rehearsal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Rehearsal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Rehearsal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_memory_rehearsal_status: `%s`\n", status)
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
	fmt.Fprintf(&b, "- requested_target_sha256_12: `%s`\n", req.RequestedTargetSHA)
	fmt.Fprintf(&b, "- requested_target_terms: `%d`\n", req.RequestedTargetTerms)
	fmt.Fprintf(&b, "- normalized_target_kind: `%s`\n", req.Rehearsal.Target.Kind)
	fmt.Fprintf(&b, "- normalized_target_path: `%s`\n", req.Rehearsal.Target.Path)
	fmt.Fprintf(&b, "- target_present: `%t`\n", req.Rehearsal.TargetPresent)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", valueOrNone(req.Rehearsal.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: `%d`\n", req.Rehearsal.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: `%d`\n", req.Rehearsal.TargetLines)
	fmt.Fprintf(&b, "- memory_budget_bytes: `%d`\n", req.Rehearsal.MemoryBudget)
	fmt.Fprintf(&b, "- memory_budget_remaining_bytes: `%d`\n", req.Rehearsal.RemainingBytes)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", req.Rehearsal.DatedMemoryNotes)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", valueOrNone(req.Rehearsal.LatestMemoryNote))
	fmt.Fprintf(&b, "- memory_validation_status: `%s`\n", req.Rehearsal.ValidationStatus)
	fmt.Fprintf(&b, "- memory_validation_errors: `%d`\n", req.Rehearsal.ValidationErrors)
	fmt.Fprintf(&b, "- memory_validation_warnings: `%d`\n", req.Rehearsal.ValidationWarnings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- candidate_memory_generation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rehearsal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_memory_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Rehearsal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Rehearsal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Rehearsal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_memory_rehearsal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing current memory from a mirrored channel thread, then queued a provider-facing rehearsal link back to that thread. This action does not call a model, generate candidate memory, edit `.gitclaw/` files, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.Rehearsal.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the rehearsal issue to exercise current prompt-visible memory behavior\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelMemoryRehearsalNotificationBody(opts ChannelMemoryRehearsalOptions, result MemoryRehearsalIssueResult, rehearsal MemoryRehearsalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel memory rehearsal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Rehearsal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Target: %s\n", valueOrNone(rehearsal.Target.Path))
	fmt.Fprintf(&b, "Validation: %s\n", rehearsal.ValidationStatus)
	b.WriteString("\nContinue in the linked GitHub issue to rehearse the current memory context with a normal model-backed conversation. This notification did not execute a model, generate candidate memory, edit `.gitclaw/` files, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelMemoryRehearsalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelMemoryRehearsalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelMemoryRehearsalIssueTarget(ev Event, req *ChannelMemoryRehearsalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel memory rehearsal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelMemoryRehearsalOptions(opts ChannelMemoryRehearsalOptions) ChannelMemoryRehearsalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RehearsalID = cleanMemoryRehearsalID(opts.RehearsalID)
	opts.Target = cleanMemoryPromoteTarget(opts.Target)
	if opts.Target == "" {
		opts.Target = "long-term"
	}
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelMemoryRehearsalOptions(opts ChannelMemoryRehearsalOptions) error {
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
		return fmt.Errorf("missing memory rehearsal id")
	}
	if !skillNamePattern.MatchString(opts.RehearsalID) {
		return fmt.Errorf("invalid memory rehearsal id %q", opts.RehearsalID)
	}
	if opts.Target == "" {
		return fmt.Errorf("missing memory rehearsal target")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildMemoryRehearsalIssueRequestFromChannel(ev Event, cfg Config, repoContext RepoContext, opts ChannelMemoryRehearsalOptions) (MemoryRehearsalIssueRequest, error) {
	target := normalizeMemoryPromoteTarget(opts.Target)
	if !target.Supported {
		return MemoryRehearsalIssueRequest{}, fmt.Errorf("unsupported memory rehearsal target %q", target.Requested)
	}
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	targetFile := memoryPromoteTargetFile(surface, target)
	remainingBytes := maxContextDocumentBytes - targetFile.Bytes
	if remainingBytes < 0 {
		remainingBytes = 0
	}
	sourceText := activeRequestText(ev)
	return MemoryRehearsalIssueRequest{
		Repo:               ev.Repo,
		Command:            "/memory",
		Subcommand:         "rehearse",
		RehearsalID:        opts.RehearsalID,
		Target:             target,
		TargetPresent:      targetFile.Present,
		TargetSHA:          targetFile.SHA,
		TargetBytes:        targetFile.Bytes,
		TargetLines:        targetFile.Lines,
		DatedMemoryNotes:   len(surface.DatedNotes),
		LatestMemoryNote:   latestMemoryNotePath(surface.DatedNotes),
		MemoryBudget:       maxContextDocumentBytes,
		RemainingBytes:     remainingBytes,
		ValidationStatus:   validation.Status,
		ValidationErrors:   validation.Errors,
		ValidationWarnings: validation.Warnings,
		SourceIssueNumber:  opts.SourceIssueNumber,
		SourceCommentID:    opts.SourceCommentID,
		SourceSHA:          shortDocumentHash(sourceText),
		SourceBytes:        len(sourceText),
		SourceLines:        lineCount(sourceText),
		SourceKind:         "channel_comment",
	}, nil
}

func autoChannelMemoryRehearsalID(ev Event, channel, threadID, sourceMessageID, target string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, target}, "|")
	return fmt.Sprintf("memory-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMemoryRehearsalNotifyMessageID(ev Event, rehearsalID string) string {
	seed := strings.Join([]string{eventID(ev), rehearsalID}, "|")
	return fmt.Sprintf("gitclaw-channel-memory-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
