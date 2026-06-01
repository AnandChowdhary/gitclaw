package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSkillRehearsalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RehearsalID       string
	RequestedSkill    string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSkillRehearsalResult struct {
	Rehearsal     SkillRehearsalIssueResult
	Notification  ChannelSendResult
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	RehearsalHash string
}

type ChannelSkillRehearsalActionRequest struct {
	Options             ChannelSkillRehearsalOptions
	Rehearsal           SkillRehearsalIssueRequest
	Command             string
	Subcommand          string
	AutoRehearsalID     bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	RequestedSkillSHA   string
	RequestedSkillTerms int
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSkillRehearsalActionRequest(ev Event, cfg Config) bool {
	return isChannelSkillRehearsalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSkillRehearsalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse-skill", "skill-rehearse", "skill-rehearsal", "try-skill", "skill-trial", "practice-skill":
		return true
	default:
		return false
	}
}

func BuildChannelSkillRehearsalActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillRehearsalActionRequest, error) {
	fields, ok := channelSkillRehearsalActionFields(ev, cfg)
	if !ok {
		return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("missing channel skill rehearsal command")
	}
	req := ChannelSkillRehearsalActionRequest{
		Options: ChannelSkillRehearsalOptions{
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
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--skill", "-s":
			if i+1 >= len(fields) {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("--skill requires a value")
			}
			req.Options.RequestedSkill = cleanSkillLookupName(fields[i+1])
			i++
		case "--id", "--rehearsal-id":
			if i+1 >= len(fields) {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RehearsalID = cleanSkillRehearsalID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("unknown channel skill rehearsal argument %q", field)
			}
			if req.Options.RequestedSkill == "" {
				req.Options.RequestedSkill = cleanSkillLookupName(field)
				continue
			}
			if req.Options.RehearsalID == "" {
				req.Options.RehearsalID = cleanSkillRehearsalID(field)
				continue
			}
			return ChannelSkillRehearsalActionRequest{}, fmt.Errorf("unexpected channel skill rehearsal argument %q", field)
		}
	}
	if err := applyChannelSkillRehearsalIssueTarget(ev, &req); err != nil {
		return ChannelSkillRehearsalActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.RehearsalID) == "" {
		req.Options.RehearsalID = autoChannelSkillRehearsalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.RequestedSkill)
		req.AutoRehearsalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillRehearsalNotifyMessageID(ev, req.Options.RehearsalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillRehearsalOptions(req.Options)
	if err := validateChannelSkillRehearsalOptions(req.Options); err != nil {
		return ChannelSkillRehearsalActionRequest{}, err
	}
	rehearsal, err := buildSkillRehearsalIssueRequestFromChannel(ev, repoContext, req.Options)
	if err != nil {
		return ChannelSkillRehearsalActionRequest{}, err
	}
	req.Rehearsal = rehearsal
	req.RequestedSkillSHA = shortDocumentHash(req.Options.RequestedSkill)
	req.RequestedSkillTerms = len(memorySearchTerms(req.Options.RequestedSkill))
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	notificationBody := RenderChannelSkillRehearsalNotificationBody(req.Options, SkillRehearsalIssueResult{
		IssueNumber: 0,
		IssueURL:    issueURL(ev.Repo, 0),
	}, rehearsal)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSkillRehearsal(ctx context.Context, cfg Config, github interface {
	SkillRehearsalIssueGitHubClient
	ChannelSendGitHubClient
}, req ChannelSkillRehearsalActionRequest) (ChannelSkillRehearsalResult, error) {
	rehearsalResult, err := RunSkillRehearsalIssue(ctx, cfg, github, req.Rehearsal)
	if err != nil {
		return ChannelSkillRehearsalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      req.Options.Repo,
		Channel:   req.Options.Channel,
		ThreadID:  req.Options.ThreadID,
		MessageID: req.Options.NotifyMessageID,
		Author:    req.Options.Author,
		Body:      RenderChannelSkillRehearsalNotificationBody(req.Options, rehearsalResult, req.Rehearsal),
	})
	if err != nil {
		return ChannelSkillRehearsalResult{}, fmt.Errorf("queue channel skill rehearsal notification: %w", err)
	}
	return ChannelSkillRehearsalResult{
		Rehearsal:     rehearsalResult,
		Notification:  notification,
		Channel:       req.Options.Channel,
		ThreadHash:    shortDocumentHash(req.Options.ThreadID),
		MessageHash:   shortDocumentHash(req.Options.SourceMessageID),
		NotifyHash:    shortDocumentHash(req.Options.NotifyMessageID),
		RehearsalHash: shortDocumentHash(req.Options.RehearsalID),
	}, nil
}

func RenderChannelSkillRehearsalActionReport(ev Event, req ChannelSkillRehearsalActionRequest, result ChannelSkillRehearsalResult) string {
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Skill Rehearsal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.Rehearsal.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: `%s`\n", req.Rehearsal.SourceKind)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_rehearsal_status: `%s`\n", status)
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
	fmt.Fprintf(&b, "- requested_skill_sha256_12: `%s`\n", req.RequestedSkillSHA)
	fmt.Fprintf(&b, "- requested_skill_terms: `%d`\n", req.RequestedSkillTerms)
	fmt.Fprintf(&b, "- requested_skill: `%s`\n", inlineCode(req.Rehearsal.RequestedSkill))
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", req.Rehearsal.MatchedSkillCount)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", req.Rehearsal.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_matches: `%d`\n", req.Rehearsal.EnabledMatches)
	fmt.Fprintf(&b, "- disabled_matches: `%d`\n", req.Rehearsal.DisabledMatches)
	fmt.Fprintf(&b, "- allowlist_blocked_matches: `%d`\n", req.Rehearsal.AllowlistBlocked)
	fmt.Fprintf(&b, "- missing_env: `%d`\n", req.Rehearsal.MissingEnv)
	fmt.Fprintf(&b, "- missing_bins: `%d`\n", req.Rehearsal.MissingBins)
	fmt.Fprintf(&b, "- selected_matches_this_turn: `%d`\n", req.Rehearsal.SelectedMatches)
	fmt.Fprintf(&b, "- skill_validation_status: `%s`\n", req.Rehearsal.SkillValidation.Status)
	fmt.Fprintf(&b, "- skill_validation_errors: `%d`\n", req.Rehearsal.SkillValidation.Errors)
	fmt.Fprintf(&b, "- skill_validation_warnings: `%d`\n", req.Rehearsal.SkillValidation.Warnings)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", req.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", req.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rehearsal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.Rehearsal.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.Rehearsal.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.Rehearsal.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_rehearsal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for trying a reviewed skill from a mirrored channel thread, then queued a provider-facing rehearsal link back to that thread. This action does not call a model, install skills, edit `SKILL.md`, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.Rehearsal.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the rehearsal issue to exercise prompt-visible skill behavior\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSkillRehearsalNotificationBody(opts ChannelSkillRehearsalOptions, result SkillRehearsalIssueResult, rehearsal SkillRehearsalIssueRequest) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill rehearsal\n\n")
	if result.IssueNumber > 0 {
		fmt.Fprintf(&b, "Rehearsal issue: #%d\n", result.IssueNumber)
	}
	if result.IssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", result.IssueURL)
	}
	fmt.Fprintf(&b, "Requested skill: %s\n", valueOrNone(rehearsal.RequestedSkill))
	fmt.Fprintf(&b, "Matched skills: %d\n", rehearsal.MatchedSkillCount)
	fmt.Fprintf(&b, "Enabled matches: %d\n", rehearsal.EnabledMatches)
	b.WriteString("\nContinue in the linked GitHub issue to rehearse this skill with a normal model-backed conversation. This notification did not execute a model, install a skill, edit `SKILL.md`, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelSkillRehearsalActionFields(ev Event, cfg Config) ([]string, bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelSkillRehearsalActionFields(fields) {
		return nil, false
	}
	return fields, true
}

func applyChannelSkillRehearsalIssueTarget(ev Event, req *ChannelSkillRehearsalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill rehearsal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSkillRehearsalOptions(opts ChannelSkillRehearsalOptions) ChannelSkillRehearsalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RehearsalID = cleanSkillRehearsalID(opts.RehearsalID)
	opts.RequestedSkill = strings.ToLower(cleanSkillLookupName(opts.RequestedSkill))
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelSkillRehearsalOptions(opts ChannelSkillRehearsalOptions) error {
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
		return fmt.Errorf("missing skill rehearsal id")
	}
	if !skillNamePattern.MatchString(opts.RehearsalID) {
		return fmt.Errorf("invalid skill rehearsal id %q", opts.RehearsalID)
	}
	if opts.RequestedSkill == "" || !skillNamePattern.MatchString(opts.RequestedSkill) {
		return fmt.Errorf("invalid channel skill rehearsal name %q", opts.RequestedSkill)
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	return nil
}

func buildSkillRehearsalIssueRequestFromChannel(ev Event, repoContext RepoContext, opts ChannelSkillRehearsalOptions) (SkillRehearsalIssueRequest, error) {
	requestedSkill := strings.ToLower(cleanSkillLookupName(opts.RequestedSkill))
	if requestedSkill == "" || !skillNamePattern.MatchString(requestedSkill) {
		return SkillRehearsalIssueRequest{}, fmt.Errorf("invalid skill rehearsal name %q", opts.RequestedSkill)
	}
	matches := matchingSkillSummaries(repoContext.SkillSummaries, requestedSkill)
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	sourceText := activeRequestText(ev)
	req := SkillRehearsalIssueRequest{
		Repo:              ev.Repo,
		Command:           "/skills",
		Subcommand:        "rehearse",
		RehearsalID:       opts.RehearsalID,
		RequestedSkill:    requestedSkill,
		MatchedSkills:     append([]SkillSummary(nil), matches...),
		MatchedSkillCount: len(matches),
		AvailableSkills:   availableSkillCount(repoContext),
		SkillValidation:   validation,
		SourceIssueNumber: opts.SourceIssueNumber,
		SourceCommentID:   opts.SourceCommentID,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "channel_comment",
	}
	for _, skill := range matches {
		if skillIsEnabled(skill) {
			req.EnabledMatches++
		}
		if skill.DisabledByConfig {
			req.DisabledMatches++
		}
		if skill.BlockedByAllowlist {
			req.AllowlistBlocked++
		}
		req.MissingEnv += len(skill.MissingEnv)
		req.MissingBins += len(skill.MissingBins)
		if skillSelectedForTurn(repoContext, skill) {
			req.SelectedMatches++
		}
	}
	return req, nil
}

func autoChannelSkillRehearsalID(ev Event, channel, threadID, sourceMessageID, requestedSkill string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, requestedSkill}, "|")
	return fmt.Sprintf("skill-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSkillRehearsalNotifyMessageID(ev Event, rehearsalID string) string {
	seed := strings.Join([]string{eventID(ev), rehearsalID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-rehearsal-%s-%s", eventID(ev), shortDocumentHash(seed))
}
