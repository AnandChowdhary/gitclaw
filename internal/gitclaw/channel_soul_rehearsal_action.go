package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSoulRehearsalOptions struct {
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

type ChannelSoulRehearsalResult struct {
	Rehearsal           SoulRehearsalIssueResult
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

type ChannelSoulRehearsalActionRequest struct {
	Options              ChannelSoulRehearsalOptions
	Rehearsal            SoulRehearsalIssueRequest
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

func IsChannelSoulRehearsalActionRequest(ev Event, cfg Config) bool {
	return isChannelSoulRehearsalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoulRehearsalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse-soul", "soul-rehearse", "soul-rehearsal", "try-soul", "voice-test", "tone-test", "practice-soul":
		return true
	default:
		return false
	}
}

func BuildChannelSoulRehearsalActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulRehearsalActionRequest, error) {
	fields, ok := channelSoulRehearsalActionFields(ev, cfg)
	if !ok {
		return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("missing channel soul rehearsal command")
	}
	req := ChannelSoulRehearsalActionRequest{
		Options: ChannelSoulRehearsalOptions{
			Repo:              ev.Repo,
			Target:            "soul",
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
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--target", "--path":
			if i+1 >= len(fields) {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Target = cleanSoulInfoPath(fields[i+1])
			targetSet = true
			i++
		case "--id", "--rehearsal-id":
			if i+1 >= len(fields) {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RehearsalID = cleanSoulRehearsalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("unknown channel soul rehearsal argument %q", field)
			}
			if !targetSet {
				req.Options.Target = cleanSoulInfoPath(field)
				targetSet = true
				continue
			}
			if req.Options.RehearsalID == "" {
				req.Options.RehearsalID = cleanSoulRehearsalID(field)
				continue
			}
			return ChannelSoulRehearsalActionRequest{}, fmt.Errorf("unexpected channel soul rehearsal argument %q", field)
		}
	}
	if err := applyChannelSoulRehearsalIssueTarget(ev, &req); err != nil {
		return ChannelSoulRehearsalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RehearsalID) == "" {
		req.Options.RehearsalID = autoChannelSoulRehearsalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Target)
		req.AutoRehearsalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulRehearsalNotifyMessageID(ev, req.Options.RehearsalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulRehearsalOptions(req.Options)
	if err := validateChannelSoulRehearsalOptions(req.Options); err != nil {
		return ChannelSoulRehearsalActionRequest{}, err
	}
	rehearsal, err := buildSoulRehearsalIssueRequestFromChannel(ev, cfg, repoContext, req.Options)
	if err != nil {
		return ChannelSoulRehearsalActionRequest{}, err
	}
	req.Rehearsal = rehearsal
	req.RequestedTargetSHA = shortDocumentHash(req.Options.Target)
	req.RequestedTargetTerms = len(memorySearchTerms(req.Options.Target))
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelSoulRehearsalNotificationBody(req.Options, SoulRehearsalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, rehearsal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSoulRehearsal(ctx context.Context, cfg Config, github interface {
	SoulRehearsalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelSoulRehearsalActionRequest) (ChannelSoulRehearsalResult, error) {
	rehearsalResult, err := RunSoulRehearsalIssue(ctx, cfg, github, req.Rehearsal)
	if err != nil {
		return ChannelSoulRehearsalResult{}, err
	}
	notificationBody := RenderChannelSoulRehearsalNotificationBody(req.Options, rehearsalResult, req.Rehearsal)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelSoulRehearsalResult{}, fmt.Errorf("queue channel soul rehearsal notification: %w", err)
	}
	return ChannelSoulRehearsalResult{
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

func RenderChannelSoulRehearsalActionReport(ev Event, req ChannelSoulRehearsalActionRequest, result ChannelSoulRehearsalResult) string {
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
	b.WriteString("## GitClaw Channel Soul Rehearsal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Rehearsal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Rehearsal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_rehearsal_status: `%s`\n", status)
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
	fmt.Fprintf(&b, "- requested_target: `%s`\n", inlineCode(req.Rehearsal.RequestedTarget))
	fmt.Fprintf(&b, "- normalized_soul_path: `%s`\n", req.Rehearsal.TargetPath)
	fmt.Fprintf(&b, "- target_category: `%s`\n", req.Rehearsal.TargetCategory)
	fmt.Fprintf(&b, "- target_present: `%t`\n", req.Rehearsal.TargetPresent)
	fmt.Fprintf(&b, "- target_required: `%t`\n", req.Rehearsal.TargetRequired)
	fmt.Fprintf(&b, "- target_canonical: `%t`\n", req.Rehearsal.TargetCanonical)
	fmt.Fprintf(&b, "- target_loaded_for_this_turn: `%t`\n", req.Rehearsal.TargetLoaded)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", valueOrNone(req.Rehearsal.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: `%d`\n", req.Rehearsal.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: `%d`\n", req.Rehearsal.TargetLines)
	fmt.Fprintf(&b, "- soul_validation_status: `%s`\n", req.Rehearsal.ValidationStatus)
	fmt.Fprintf(&b, "- soul_validation_errors: `%d`\n", req.Rehearsal.ValidationErrors)
	fmt.Fprintf(&b, "- soul_validation_warnings: `%d`\n", req.Rehearsal.ValidationWarnings)
	fmt.Fprintf(&b, "- soul_risk_status: `%s`\n", req.Rehearsal.RiskStatus)
	fmt.Fprintf(&b, "- soul_risk_findings: `%d`\n", req.Rehearsal.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", req.Rehearsal.HighRiskFindings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- context_target_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- candidate_soul_generation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rehearsal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_soul_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Rehearsal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Rehearsal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Rehearsal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_rehearsal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for rehearsing high-authority context from a mirrored channel thread, then queued a provider-facing rehearsal link back to that thread. This action does not call a model, generate candidate soul text, write `.gitclaw/` files, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.Rehearsal.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the rehearsal issue to exercise current prompt-visible behavior\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSoulRehearsalNotificationBody(opts ChannelSoulRehearsalOptions, result SoulRehearsalIssueResult, rehearsal SoulRehearsalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul rehearsal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Rehearsal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Target: %s\n", valueOrNone(rehearsal.TargetPath))
	fmt.Fprintf(&b, "Validation: %s\n", rehearsal.ValidationStatus)
	fmt.Fprintf(&b, "Risk: %s\n", rehearsal.RiskStatus)
	b.WriteString("\nContinue in the linked GitHub issue to rehearse the current high-authority context with a normal model-backed conversation. This notification did not execute a model, generate candidate soul text, edit `.gitclaw/` files, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelSoulRehearsalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelSoulRehearsalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelSoulRehearsalIssueTarget(ev Event, req *ChannelSoulRehearsalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul rehearsal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSoulRehearsalOptions(opts ChannelSoulRehearsalOptions) ChannelSoulRehearsalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RehearsalID = cleanSoulRehearsalID(opts.RehearsalID)
	opts.Target = cleanSoulInfoPath(opts.Target)
	if opts.Target == "" {
		opts.Target = "soul"
	}
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelSoulRehearsalOptions(opts ChannelSoulRehearsalOptions) error {
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
		return fmt.Errorf("missing soul rehearsal id")
	}
	if !skillNamePattern.MatchString(opts.RehearsalID) {
		return fmt.Errorf("invalid soul rehearsal id %q", opts.RehearsalID)
	}
	if opts.Target == "" {
		return fmt.Errorf("missing soul rehearsal target")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildSoulRehearsalIssueRequestFromChannel(ev Event, cfg Config, repoContext RepoContext, opts ChannelSoulRehearsalOptions) (SoulRehearsalIssueRequest, error) {
	targetPath := normalizeSoulInfoPath(opts.Target, cfg, repoContext)
	if targetPath == "" || !soulInfoAllowedPath(targetPath) {
		return SoulRehearsalIssueRequest{}, fmt.Errorf("unsupported soul rehearsal target %q", opts.Target)
	}
	match, ok := soulInfoMatch(cfg.Workdir, repoContext, targetPath)
	if !ok {
		return SoulRehearsalIssueRequest{}, fmt.Errorf("target metadata unavailable for %q", targetPath)
	}
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	sourceText := activeRequestText(ev)
	return SoulRehearsalIssueRequest{
		Repo:               ev.Repo,
		Command:            "/soul",
		Subcommand:         "rehearse",
		RehearsalID:        opts.RehearsalID,
		RequestedTarget:    cleanSoulInfoPath(opts.Target),
		TargetPath:         targetPath,
		TargetCategory:     match.Category,
		TargetPresent:      match.Present,
		TargetRequired:     match.Required,
		TargetCanonical:    match.Canonical,
		TargetLoaded:       match.LoadedForThisTurn,
		TargetSHA:          match.SHA,
		TargetBytes:        match.Bytes,
		TargetLines:        match.Lines,
		ValidationStatus:   validation.Status,
		ValidationErrors:   validation.Errors,
		ValidationWarnings: validation.Warnings,
		RiskStatus:         risk.Status,
		RiskFindings:       len(risk.Findings),
		HighRiskFindings:   risk.HighRiskFindings,
		SourceIssueNumber:  opts.SourceIssueNumber,
		SourceCommentID:    opts.SourceCommentID,
		SourceSHA:          shortDocumentHash(sourceText),
		SourceBytes:        len(sourceText),
		SourceLines:        lineCount(sourceText),
		SourceKind:         "channel_comment",
	}, nil
}

func autoChannelSoulRehearsalID(ev Event, channel, threadID, sourceMessageID, target string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, target}, "|")
	return fmt.Sprintf("soul-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSoulRehearsalNotifyMessageID(ev Event, rehearsalID string) string {
	seed := strings.Join([]string{eventID(ev), rehearsalID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
