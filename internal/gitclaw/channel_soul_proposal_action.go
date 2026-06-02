package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSoulProposalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ProposalID        string
	Target            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSoulProposalResult struct {
	Proposal            SoulProposalIssueResult
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	ProposalHash        string
	TargetHash          string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSoulProposalActionRequest struct {
	Options              ChannelSoulProposalOptions
	Proposal             SoulProposalIssueRequest
	Command              string
	Subcommand           string
	AutoProposalID       bool
	AutoNotifyMessageID  bool
	TargetFromIssue      bool
	RequestedTargetSHA   string
	RequestedTargetTerms int
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	ProposalIDSHA        string
	TargetPathSHA        string
	NotificationBodySHA  string
	NotificationBytes    int
	NotificationLines    int
}

func IsChannelSoulProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelSoulProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoulProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-soul", "soul-propose", "soul-proposal", "edit-soul", "change-soul", "soul-change":
		return true
	default:
		return false
	}
}

func BuildChannelSoulProposalActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulProposalActionRequest, error) {
	fields, ok := channelSoulProposalActionFields(ev, cfg)
	if !ok {
		return ChannelSoulProposalActionRequest{}, fmt.Errorf("missing channel soul proposal command")
	}
	req := ChannelSoulProposalActionRequest{
		Options: ChannelSoulProposalOptions{
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
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--target", "--path":
			if i+1 >= len(fields) {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Target = cleanSoulInfoPath(fields[i+1])
			targetSet = true
			i++
		case "--id", "--proposal-id", "--soul-proposal-id":
			if i+1 >= len(fields) {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProposalID = cleanSoulProposalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulProposalActionRequest{}, fmt.Errorf("unknown channel soul proposal argument %q", field)
			}
			if !targetSet && channelSoulProposalLooksLikeTarget(field) {
				req.Options.Target = cleanSoulInfoPath(field)
				targetSet = true
				continue
			}
			if req.Options.ProposalID == "" {
				req.Options.ProposalID = cleanSoulProposalID(field)
				continue
			}
			if !targetSet {
				req.Options.Target = cleanSoulInfoPath(field)
				targetSet = true
				continue
			}
			return ChannelSoulProposalActionRequest{}, fmt.Errorf("unexpected channel soul proposal argument %q", field)
		}
	}
	if err := applyChannelSoulProposalIssueTarget(ev, &req); err != nil {
		return ChannelSoulProposalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.ProposalID) == "" {
		req.Options.ProposalID = autoChannelSoulProposalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Target)
		req.AutoProposalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulProposalNotifyMessageID(ev, req.Options.ProposalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulProposalOptions(req.Options)
	if err := validateChannelSoulProposalOptions(req.Options); err != nil {
		return ChannelSoulProposalActionRequest{}, err
	}
	proposal, err := buildSoulProposalIssueRequestFromChannel(ev, cfg, repoContext, req.Options)
	if err != nil {
		return ChannelSoulProposalActionRequest{}, err
	}
	req.Proposal = proposal
	req.ProposalIDSHA = shortDocumentHash(proposal.ProposalID)
	req.RequestedTargetSHA = shortDocumentHash(req.Options.Target)
	req.RequestedTargetTerms = len(memorySearchTerms(req.Options.Target))
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.TargetPathSHA = shortDocumentHash(proposal.TargetPath)
	notificationBody := RenderChannelSoulProposalNotificationBody(req.Options, SoulProposalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, proposal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSoulProposal(ctx context.Context, cfg Config, github interface {
	SoulProposalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelSoulProposalActionRequest) (ChannelSoulProposalResult, error) {
	proposalResult, err := RunSoulProposalIssue(ctx, github, req.Proposal)
	if err != nil {
		return ChannelSoulProposalResult{}, err
	}
	notificationBody := RenderChannelSoulProposalNotificationBody(req.Options, proposalResult, req.Proposal)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelSoulProposalResult{}, fmt.Errorf("queue channel soul proposal notification: %w", err)
	}
	return ChannelSoulProposalResult{
		Proposal:            proposalResult,
		Notification:        notification,
		Channel:             req.Options.Channel,
		ThreadHash:          shortDocumentHash(req.Options.ThreadID),
		MessageHash:         shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:          shortDocumentHash(req.Options.NotifyMessageID),
		ProposalHash:        shortDocumentHash(req.Proposal.ProposalID),
		TargetHash:          shortDocumentHash(req.Proposal.TargetPath),
		NotificationBodySHA: shortDocumentHash(notificationBody),
		NotificationBytes:   len(notificationBody),
		NotificationLines:   lineCount(notificationBody),
	}, nil
}

func RenderChannelSoulProposalActionReport(ev Event, req ChannelSoulProposalActionRequest, result ChannelSoulProposalResult) string {
	status := "created"
	switch {
	case result.Proposal.Duplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.Proposal.Duplicate:
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
	proposalHash := result.ProposalHash
	if proposalHash == "" {
		proposalHash = req.ProposalIDSHA
	}
	targetHash := result.TargetHash
	if targetHash == "" {
		targetHash = req.TargetPathSHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Soul Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Proposal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Proposal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_proposal_issue: `#%d`\n", result.Proposal.IssueNumber)
	fmt.Fprintf(&b, "- soul_proposal_issue_url: `%s`\n", result.Proposal.IssueURL)
	fmt.Fprintf(&b, "- soul_proposal_issue_created: `%t`\n", result.Proposal.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Proposal.Duplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- soul_proposal_id_sha256_12: `%s`\n", noneIfEmpty(proposalHash))
	fmt.Fprintf(&b, "- soul_proposal_id_auto: `%t`\n", req.AutoProposalID)
	fmt.Fprintf(&b, "- requested_target_sha256_12: `%s`\n", req.RequestedTargetSHA)
	fmt.Fprintf(&b, "- requested_target_terms: `%d`\n", req.RequestedTargetTerms)
	fmt.Fprintf(&b, "- target_path_sha256_12: `%s`\n", noneIfEmpty(targetHash))
	fmt.Fprintf(&b, "- target_category: `%s`\n", req.Proposal.TargetCategory)
	fmt.Fprintf(&b, "- target_present: `%t`\n", req.Proposal.TargetPresent)
	fmt.Fprintf(&b, "- target_required: `%t`\n", req.Proposal.TargetRequired)
	fmt.Fprintf(&b, "- target_canonical: `%t`\n", req.Proposal.TargetCanonical)
	fmt.Fprintf(&b, "- target_loaded_for_this_turn: `%t`\n", req.Proposal.TargetLoaded)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", valueOrNone(req.Proposal.TargetSHA))
	fmt.Fprintf(&b, "- target_bytes: `%d`\n", req.Proposal.TargetBytes)
	fmt.Fprintf(&b, "- target_lines: `%d`\n", req.Proposal.TargetLines)
	fmt.Fprintf(&b, "- soul_validation_status: `%s`\n", req.Proposal.ValidationStatus)
	fmt.Fprintf(&b, "- soul_validation_errors: `%d`\n", req.Proposal.ValidationErrors)
	fmt.Fprintf(&b, "- soul_validation_warnings: `%d`\n", req.Proposal.ValidationWarnings)
	fmt.Fprintf(&b, "- soul_risk_status: `%s`\n", req.Proposal.RiskStatus)
	fmt.Fprintf(&b, "- soul_risk_findings: `%d`\n", req.Proposal.RiskFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", req.Proposal.HighRiskFindings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", notificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-soul-file")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- candidate_soul_generation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_proposal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_existing_soul_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_candidate_soul_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Proposal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Proposal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Proposal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_proposal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for a high-authority context proposal from a mirrored channel thread, then queued a provider-facing proposal link back to that thread. This action does not call a model, generate candidate soul text, write `.gitclaw/` files, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Proposal Review Path\n")
	fmt.Fprintf(&b, "- continue on soul proposal issue: `#%d`\n", result.Proposal.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the proposal issue to discuss the reviewed context change\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSoulProposalNotificationBody(opts ChannelSoulProposalOptions, result SoulProposalIssueResult, proposal SoulProposalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul proposal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Proposal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Proposal id: %s\n", valueOrNone(proposal.ProposalID))
	fmt.Fprintf(&b, "Target path: %s\n", valueOrNone(proposal.TargetPath))
	fmt.Fprintf(&b, "Target category: %s\n", valueOrNone(proposal.TargetCategory))
	fmt.Fprintf(&b, "Validation: %s\n", proposal.ValidationStatus)
	fmt.Fprintf(&b, "Risk: %s\n", proposal.RiskStatus)
	fmt.Fprintf(&b, "Review PR required: %t\n", true)
	fmt.Fprintf(&b, "Soul file written: %t\n", false)
	b.WriteString("\nContinue in the linked GitHub issue to review the high-authority context proposal with a normal model-backed conversation. This notification did not execute a model, generate candidate soul text, edit `.gitclaw/` files, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelSoulProposalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelSoulProposalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelSoulProposalIssueTarget(ev Event, req *ChannelSoulProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul proposal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSoulProposalOptions(opts ChannelSoulProposalOptions) ChannelSoulProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ProposalID = cleanSoulProposalID(opts.ProposalID)
	opts.Target = cleanSoulInfoPath(opts.Target)
	if opts.Target == "" {
		opts.Target = "soul"
	}
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelSoulProposalOptions(opts ChannelSoulProposalOptions) error {
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
	if opts.ProposalID == "" {
		return fmt.Errorf("missing soul proposal id")
	}
	if !skillNamePattern.MatchString(opts.ProposalID) {
		return fmt.Errorf("invalid soul proposal id %q", opts.ProposalID)
	}
	if opts.Target == "" {
		return fmt.Errorf("missing soul proposal target")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildSoulProposalIssueRequestFromChannel(ev Event, cfg Config, repoContext RepoContext, opts ChannelSoulProposalOptions) (SoulProposalIssueRequest, error) {
	targetPath := normalizeSoulInfoPath(opts.Target, cfg, repoContext)
	if targetPath == "" || !soulInfoAllowedPath(targetPath) {
		return SoulProposalIssueRequest{}, fmt.Errorf("unsupported soul proposal target %q", opts.Target)
	}
	match, ok := soulInfoMatch(cfg.Workdir, repoContext, targetPath)
	if !ok {
		return SoulProposalIssueRequest{}, fmt.Errorf("target metadata unavailable for %q", targetPath)
	}
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	sourceText := activeRequestText(ev)
	return SoulProposalIssueRequest{
		Repo:               ev.Repo,
		Command:            "/soul",
		Subcommand:         "propose",
		ProposalID:         opts.ProposalID,
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

func channelSoulProposalLooksLikeTarget(value string) bool {
	cleaned := cleanSoulInfoPath(value)
	switch strings.ToLower(cleaned) {
	case "soul", "soul.md", "identity", "identity.md", "user", "user.md", "tools", "tools.md", "memory", "memory.md", "heartbeat", "heartbeat.md":
		return true
	default:
		return strings.Contains(cleaned, "/") || strings.HasPrefix(cleaned, ".gitclaw")
	}
}

func autoChannelSoulProposalID(ev Event, channel, threadID, sourceMessageID, target string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, target}, "|")
	return fmt.Sprintf("soul-proposal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSoulProposalNotifyMessageID(ev Event, proposalID string) string {
	seed := strings.Join([]string{eventID(ev), proposalID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-proposal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
