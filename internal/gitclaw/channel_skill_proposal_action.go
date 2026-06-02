package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSkillProposalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ProposalName      string
	RequestedAction   string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSkillProposalResult struct {
	Proposal     SkillProposalIssueResult
	Notification ChannelSendResult
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	ProposalHash string
}

type ChannelSkillProposalActionRequest struct {
	Options             ChannelSkillProposalOptions
	Proposal            SkillProposalIssueRequest
	Command             string
	Subcommand          string
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	ProposalNameSHA     string
	ProposalNameTerms   int
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ProposalPathSHA     string
	DestinationPathSHA  string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSkillProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelSkillProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSkillProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-skill", "skill-propose", "skill-proposal", "propose-skill-create", "propose-skill-update":
		return true
	default:
		return false
	}
}

func BuildChannelSkillProposalActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillProposalActionRequest, error) {
	fields, ok := channelSkillProposalActionFields(ev, cfg)
	if !ok {
		return ChannelSkillProposalActionRequest{}, fmt.Errorf("missing channel skill proposal command")
	}
	req := ChannelSkillProposalActionRequest{
		Options: ChannelSkillProposalOptions{
			Repo:              ev.Repo,
			RequestedAction:   "auto",
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	switch req.Subcommand {
	case "propose-skill-create":
		req.Options.RequestedAction = "propose-create"
	case "propose-skill-update":
		req.Options.RequestedAction = "propose-update"
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--name", "--skill", "--target", "--id", "--proposal-id":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ProposalName = fields[i+1]
			i++
		case "--action", "--operation":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RequestedAction = normalizeChannelSkillProposalAction(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillProposalActionRequest{}, fmt.Errorf("unknown channel skill proposal argument %q", field)
			}
			if req.Options.ProposalName == "" {
				req.Options.ProposalName = field
				continue
			}
			return ChannelSkillProposalActionRequest{}, fmt.Errorf("unexpected channel skill proposal argument %q", field)
		}
	}
	if err := applyChannelSkillProposalIssueTarget(ev, &req); err != nil {
		return ChannelSkillProposalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillProposalNotifyMessageID(ev, req.Options.ProposalName)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillProposalOptions(req.Options)
	if err := validateChannelSkillProposalOptions(req.Options); err != nil {
		return ChannelSkillProposalActionRequest{}, err
	}
	proposal, err := buildSkillProposalIssueRequestFromChannel(ev, repoContext, req.Options)
	if err != nil {
		return ChannelSkillProposalActionRequest{}, err
	}
	req.Proposal = proposal
	req.ProposalNameSHA = shortDocumentHash(proposal.Target.Candidate)
	req.ProposalNameTerms = len(memorySearchTerms(proposal.Target.Candidate))
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ProposalPathSHA = shortDocumentHash(proposal.ProposalPath)
	req.DestinationPathSHA = shortDocumentHash(proposal.DestinationPath)
	notificationBody := RenderChannelSkillProposalNotificationBody(req.Options, SkillProposalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, proposal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSkillProposal(ctx context.Context, cfg Config, github interface {
	SkillProposalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelSkillProposalActionRequest) (ChannelSkillProposalResult, error) {
	proposalResult, err := RunSkillProposalIssue(ctx, github, req.Proposal)
	if err != nil {
		return ChannelSkillProposalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      RenderChannelSkillProposalNotificationBody(req.Options, proposalResult, req.Proposal),
	})
	if err != nil {
		return ChannelSkillProposalResult{}, fmt.Errorf("queue channel skill proposal notification: %w", err)
	}
	return ChannelSkillProposalResult{
		Proposal:     proposalResult,
		Notification: notification,
		Channel:      req.Options.Channel,
		ThreadHash:   shortDocumentHash(req.Options.ThreadID),
		MessageHash:  shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:   shortDocumentHash(req.Options.NotifyMessageID),
		ProposalHash: shortDocumentHash(req.Proposal.Target.Candidate),
	}, nil
}

func RenderChannelSkillProposalActionReport(ev Event, req ChannelSkillProposalActionRequest, result ChannelSkillProposalResult) string {
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Skill Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Proposal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Proposal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- proposal_issue: `#%d`\n", result.Proposal.IssueNumber)
	fmt.Fprintf(&b, "- proposal_issue_url: `%s`\n", result.Proposal.IssueURL)
	fmt.Fprintf(&b, "- proposal_issue_created: `%t`\n", result.Proposal.Created)
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
	fmt.Fprintf(&b, "- proposal_name_sha256_12: `%s`\n", result.ProposalHash)
	fmt.Fprintf(&b, "- proposal_name_terms: `%d`\n", req.ProposalNameTerms)
	fmt.Fprintf(&b, "- requested_action: `%s`\n", req.Proposal.RequestedAction)
	fmt.Fprintf(&b, "- planned_proposal_action: `%s`\n", req.Proposal.PlannedAction)
	fmt.Fprintf(&b, "- target_type: `%s`\n", req.Proposal.Target.Type)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", req.Proposal.Target.Hash)
	fmt.Fprintf(&b, "- proposal_path_sha256_12: `%s`\n", req.ProposalPathSHA)
	fmt.Fprintf(&b, "- destination_path_sha256_12: `%s`\n", req.DestinationPathSHA)
	fmt.Fprintf(&b, "- existing_skill_matches: `%d`\n", req.Proposal.ExistingSkillMatches)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", req.Proposal.AvailableSkills)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", req.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", req.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-proposal-file")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_body_generated: `%t`\n", false)
	fmt.Fprintf(&b, "- proposal_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_destination_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Proposal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Proposal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Proposal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_proposal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for a skill proposal from a mirrored channel thread, then queued a provider-facing proposal link back to that thread. This action does not call a model, generate a skill body, write proposal files, edit active skills, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Proposal Review Path\n")
	fmt.Fprintf(&b, "- continue on proposal issue: `#%d`\n", result.Proposal.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the proposal issue to discuss the reviewed skill proposal\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSkillProposalNotificationBody(opts ChannelSkillProposalOptions, result SkillProposalIssueResult, proposal SkillProposalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill proposal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Proposal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Proposal name: %s\n", valueOrNone(proposal.Target.Candidate))
	fmt.Fprintf(&b, "Planned action: %s\n", proposal.PlannedAction)
	fmt.Fprintf(&b, "Proposal path: %s\n", valueOrNone(proposal.ProposalPath))
	fmt.Fprintf(&b, "Destination path: %s\n", valueOrNone(proposal.DestinationPath))
	fmt.Fprintf(&b, "Review PR required: %t\n", true)
	fmt.Fprintf(&b, "Active skill written: %t\n", false)
	b.WriteString("\nContinue in the linked GitHub issue to review the skill proposal with a normal model-backed conversation. This notification did not execute a model, generate a skill body, write proposal files, edit active skills, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelSkillProposalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelSkillProposalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelSkillProposalIssueTarget(ev Event, req *ChannelSkillProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill proposal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSkillProposalOptions(opts ChannelSkillProposalOptions) ChannelSkillProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ProposalName = strings.TrimSpace(opts.ProposalName)
	opts.RequestedAction = normalizeChannelSkillProposalAction(opts.RequestedAction)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelSkillProposalOptions(opts ChannelSkillProposalOptions) error {
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
	target := classifySkillInstallTarget(opts.ProposalName)
	if target.Candidate == "" {
		return fmt.Errorf("missing skill proposal name")
	}
	if !skillNamePattern.MatchString(target.Candidate) {
		return fmt.Errorf("invalid channel skill proposal name %q", target.Candidate)
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildSkillProposalIssueRequestFromChannel(ev Event, repoContext RepoContext, opts ChannelSkillProposalOptions) (SkillProposalIssueRequest, error) {
	target := classifySkillInstallTarget(opts.ProposalName)
	if target.Candidate == "" || !skillNamePattern.MatchString(target.Candidate) {
		return SkillProposalIssueRequest{}, fmt.Errorf("invalid channel skill proposal name %q", opts.ProposalName)
	}
	requestedAction := normalizeChannelSkillProposalAction(opts.RequestedAction)
	matches := matchingInstallPlanSkillSummaries(repoContext.SkillSummaries, target)
	sourceText := activeRequestText(ev)
	return SkillProposalIssueRequest{
		Repo:                 ev.Repo,
		Command:              "/skills",
		Subcommand:           "propose",
		RequestedAction:      requestedAction,
		PlannedAction:        plannedSkillProposalAction(requestedAction, len(matches)),
		Target:               target,
		ProposalPath:         skillProposalPlanPath(target.Candidate),
		DestinationPath:      skillInstallDestinationPath(target.Candidate),
		ExistingSkillMatches: len(matches),
		AvailableSkills:      availableSkillCount(repoContext),
		SourceIssueNumber:    opts.SourceIssueNumber,
		SourceCommentID:      opts.SourceCommentID,
		SourceSHA:            shortDocumentHash(sourceText),
		SourceBytes:          len(sourceText),
		SourceLines:          lineCount(sourceText),
		SourceKind:           "channel_comment",
	}, nil
}

func normalizeChannelSkillProposalAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "create", "new", "propose-create":
		return "propose-create"
	case "update", "edit", "propose-update":
		return "propose-update"
	default:
		return "auto"
	}
}

func autoChannelSkillProposalNotifyMessageID(ev Event, proposalName string) string {
	seed := strings.Join([]string{eventID(ev), proposalName}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-proposal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
