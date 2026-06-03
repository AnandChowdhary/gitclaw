package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelDoneOptions struct {
	Repo              string
	NotifyMessageID   string
	Author            string
	ArtifactIssue     Issue
	SourceIssueNumber int
	SourceCommentID   int64
	ArtifactKind      string
	ArtifactID        string
	Channel           string
}

type ChannelDoneResult struct {
	ArtifactIssueNumber int
	ArtifactIssueURL    string
	ArtifactClosed      bool
	SourceIssueNumber   int
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	ArtifactIDHash      string
	NotifyHash          string
	NotificationBodySHA string
}

type ChannelDoneActionRequest struct {
	Options             ChannelDoneOptions
	Command             string
	Subcommand          string
	AutoNotifyMessageID bool
	RequestedNotifySHA  string
	ArtifactTitleSHA    string
	NotificationBodySHA string
}

type channelDoneArtifactRef struct {
	Kind              string
	ID                string
	Channel           string
	SourceIssueNumber int
	SourceCommentID   int64
}

func IsChannelDoneActionRequest(ev Event, cfg Config) bool {
	return isChannelDoneActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelDoneActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "done", "complete", "completed", "close", "closed", "resolve", "resolved":
		return true
	default:
		return false
	}
}

func BuildChannelDoneActionRequest(ev Event, cfg Config) (ChannelDoneActionRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isChannelDoneActionFields(fields) {
		return ChannelDoneActionRequest{}, fmt.Errorf("missing channel done command")
	}
	ref, err := channelDoneArtifactRefFromBody(ev.Issue.Body)
	if err != nil {
		return ChannelDoneActionRequest{}, err
	}
	req := ChannelDoneActionRequest{
		Options: ChannelDoneOptions{
			Repo:              ev.Repo,
			ArtifactIssue:     ev.Issue,
			SourceIssueNumber: ref.SourceIssueNumber,
			SourceCommentID:   ref.SourceCommentID,
			ArtifactKind:      ref.Kind,
			ArtifactID:        ref.ID,
			Channel:           ref.Channel,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := strings.TrimSpace(fields[i])
		switch field {
		case "--message-id", "--notify-message-id", "--notification-message-id", "--done-message-id", "--completion-message-id":
			if i+1 >= len(fields) {
				return ChannelDoneActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = strings.TrimSpace(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelDoneActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = strings.TrimSpace(fields[i+1])
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelDoneActionRequest{}, fmt.Errorf("unknown channel done argument %q", field)
			}
			if req.Options.NotifyMessageID == "" {
				req.Options.NotifyMessageID = strings.TrimSpace(field)
				continue
			}
			return ChannelDoneActionRequest{}, fmt.Errorf("unexpected channel done argument %q", field)
		}
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelDoneNotifyMessageID(ev, ref)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelDoneOptions(req.Options)
	if err := validateChannelDoneOptions(req.Options); err != nil {
		return ChannelDoneActionRequest{}, err
	}
	req.RequestedNotifySHA = shortDocumentHash(req.Options.NotifyMessageID)
	req.ArtifactTitleSHA = shortDocumentHash(req.Options.ArtifactIssue.Title)
	req.NotificationBodySHA = shortDocumentHash(RenderChannelDoneNotificationBody(req.Options, req.Options.ArtifactIssue.Number, issueURL(req.Options.Repo, req.Options.ArtifactIssue.Number)))
	return req, nil
}

func RunChannelDone(ctx context.Context, cfg Config, github ChannelDoneGitHubClient, opts ChannelDoneOptions) (ChannelDoneResult, error) {
	opts = normalizeChannelDoneOptions(opts)
	if err := validateChannelDoneOptions(opts); err != nil {
		return ChannelDoneResult{}, err
	}
	sourceIssue, err := github.GetIssue(ctx, opts.Repo, opts.SourceIssueNumber)
	if err != nil {
		return ChannelDoneResult{}, fmt.Errorf("get source channel issue: %w", err)
	}
	channel, threadID := channelThreadMarkerFields(sourceIssue.Body)
	if channel == "" || threadID == "" {
		return ChannelDoneResult{}, fmt.Errorf("source issue #%d is not a channel thread", opts.SourceIssueNumber)
	}
	if opts.Channel != "" && channel != opts.Channel {
		return ChannelDoneResult{}, fmt.Errorf("artifact channel %q does not match source channel %q", opts.Channel, channel)
	}
	opts.Channel = channel
	body := RenderChannelDoneNotificationBody(opts, opts.ArtifactIssue.Number, issueURL(opts.Repo, opts.ArtifactIssue.Number))
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   channel,
		ThreadID:  threadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelDoneResult{}, fmt.Errorf("queue channel done notification: %w", err)
	}
	if err := github.CloseIssue(ctx, opts.Repo, opts.ArtifactIssue.Number); err != nil {
		return ChannelDoneResult{}, fmt.Errorf("close channel artifact issue: %w", err)
	}
	return ChannelDoneResult{
		ArtifactIssueNumber: opts.ArtifactIssue.Number,
		ArtifactIssueURL:    issueURL(opts.Repo, opts.ArtifactIssue.Number),
		ArtifactClosed:      true,
		SourceIssueNumber:   opts.SourceIssueNumber,
		Notification:        notification,
		Channel:             channel,
		ThreadHash:          shortDocumentHash(threadID),
		ArtifactIDHash:      shortDocumentHash(opts.ArtifactID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		NotificationBodySHA: shortDocumentHash(body),
	}, nil
}

func RenderChannelDoneActionReport(ev Event, req ChannelDoneActionRequest, result ChannelDoneResult) string {
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	var sourceCommentID int64
	if ev.Comment != nil {
		sourceCommentID = ev.Comment.ID
	}
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := result.ThreadHash
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = req.RequestedNotifySHA
	}
	bodyHash := result.NotificationBodySHA
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Done Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", sourceCommentID)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_artifact_kind: `%s`\n", req.Options.ArtifactKind)
	fmt.Fprintf(&b, "- channel_artifact_issue: `#%d`\n", result.ArtifactIssueNumber)
	fmt.Fprintf(&b, "- channel_artifact_issue_url: `%s`\n", result.ArtifactIssueURL)
	fmt.Fprintf(&b, "- channel_artifact_closed: `%t`\n", result.ArtifactClosed)
	fmt.Fprintf(&b, "- source_channel_issue: `#%d`\n", result.SourceIssueNumber)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- artifact_id_sha256_12: `%s`\n", noneIfEmpty(result.ArtifactIDHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- artifact_title_sha256_12: `%s`\n", req.ArtifactTitleSHA)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- raw_artifact_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_artifact_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_artifact_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_done_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw closed the GitHub-native channel artifact issue and queued a provider-facing acknowledgement back to the original mirrored channel thread. The acknowledgement is delivered later by `gitclaw channel-outbox` and `gitclaw channel-delivery`; this source receipt keeps artifact IDs, thread IDs, message IDs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the done acknowledgement with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent acknowledgements with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate done acknowledgements are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelDoneNotificationBody(opts ChannelDoneOptions, issueNumber int, issueURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "GitClaw channel %s completed\n\n", opts.ArtifactKind)
	fmt.Fprintf(&b, "Artifact issue: #%d %s\n", issueNumber, issueURL)
	fmt.Fprintf(&b, "Kind: %s\n", opts.ArtifactKind)
	b.WriteString("State: closed\n")
	b.WriteString("Provider delivery performed: false\n\n")
	b.WriteString("GitClaw closed the GitHub issue for this channel artifact. Continue in GitHub if more context is needed.")
	return strings.TrimSpace(b.String())
}

func channelDoneArtifactRefFromBody(body string) (channelDoneArtifactRef, error) {
	if ref, ok := channelDoneArtifactRefFromMarker(body, "task", channelTaskMarkerPattern, "task_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "watch", channelWatchMarkerPattern, "watch_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "standing-order-proposal", channelStandingOrderProposalMarkerPattern, "proposal_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "backup-restore-request", backupRestoreRequestIssueMarkerPattern, "id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "checkpoint-rehearsal", checkpointRehearsalIssueMarkerPattern, "id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "clip", channelClipMarkerPattern, "clip_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "open-loop", channelOpenLoopMarkerPattern, "loop_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "attachment", channelAttachmentMarkerPattern, "attachment_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "snippet", channelSnippetMarkerPattern, "snippet_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "decision", channelDecisionMarkerPattern, "decision_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "digest", channelDigestMarkerPattern, "digest_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "journal", channelJournalMarkerPattern, "journal_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "quote", channelQuoteMarkerPattern, "quote_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "glossary", channelGlossaryMarkerPattern, "glossary_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "faq", channelFAQMarkerPattern, "faq_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "idea", channelIdeaMarkerPattern, "idea_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "jam", channelJamMarkerPattern, "jam_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "retro", channelRetroMarkerPattern, "retro_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "playbook", channelPlaybookMarkerPattern, "playbook_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "insight", channelInsightMarkerPattern, "insight_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "workspace-proposal", channelWorkspaceProposalMarkerPattern, "workspace_proposal_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "board-card", channelBoardCardMarkerPattern, "board_card_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "checklist", channelChecklistMarkerPattern, "checklist_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "agenda", channelAgendaMarkerPattern, "agenda_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "tool-result", channelToolResultMarkerPattern, "result_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "toolset-proposal", channelToolsetProposalMarkerPattern, "toolset_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "prompt-proposal", channelPromptProposalMarkerPattern, "prompt_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "bundle-proposal", channelBundleProposalMarkerPattern, "bundle_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "incident", channelIncidentMarkerPattern, "incident_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "voice", channelVoiceMarkerPattern, "voice_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "image", channelImageMarkerPattern, "image_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "link", channelLinkMarkerPattern, "link_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "bookmark", channelBookmarkMarkerPattern, "bookmark_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "fork", channelForkMarkerPattern, "fork_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "merge", channelMergeMarkerPattern, "merge_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "access-request", channelAccessRequestMarkerPattern, "access_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "contact", channelContactMarkerPattern, "contact_id"); ok {
		return ref, nil
	}
	if ref, ok := channelDoneArtifactRefFromMarker(body, "reminder", channelReminderMarkerPattern, "reminder_id"); ok {
		return ref, nil
	}
	return channelDoneArtifactRef{}, fmt.Errorf("channel done requires a gitclaw:channel-task, gitclaw:channel-watch, gitclaw:channel-standing-order-proposal, gitclaw:backup-restore-request-issue, gitclaw:checkpoint-rehearsal-issue, gitclaw:channel-clip, gitclaw:channel-open-loop, gitclaw:channel-attachment, gitclaw:channel-snippet, gitclaw:channel-decision, gitclaw:channel-digest, gitclaw:channel-journal, gitclaw:channel-quote, gitclaw:channel-glossary, gitclaw:channel-faq, gitclaw:channel-idea, gitclaw:channel-jam, gitclaw:channel-retro, gitclaw:channel-playbook, gitclaw:channel-insight, gitclaw:channel-workspace-proposal, gitclaw:channel-board-card, gitclaw:channel-checklist, gitclaw:channel-agenda, gitclaw:channel-tool-result, gitclaw:channel-toolset-proposal, gitclaw:channel-prompt-proposal, gitclaw:channel-bundle-proposal, gitclaw:channel-incident, gitclaw:channel-voice, gitclaw:channel-image, gitclaw:channel-link, gitclaw:channel-bookmark, gitclaw:channel-fork, gitclaw:channel-merge, gitclaw:channel-access-request, gitclaw:channel-contact, or gitclaw:channel-reminder issue")
}

func channelDoneArtifactRefFromMarker(body, kind string, pattern interface{ FindStringSubmatch(string) []string }, idKey string) (channelDoneArtifactRef, bool) {
	match := pattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return channelDoneArtifactRef{}, false
	}
	sourceIssue, _ := strconv.Atoi(markerAttribute(match[1], "source_issue"))
	sourceCommentID, _ := strconv.ParseInt(markerAttribute(match[1], "source_comment_id"), 10, 64)
	return channelDoneArtifactRef{
		Kind:              kind,
		ID:                markerAttribute(match[1], idKey),
		Channel:           markerAttribute(match[1], "channel"),
		SourceIssueNumber: sourceIssue,
		SourceCommentID:   sourceCommentID,
	}, true
}

func normalizeChannelDoneOptions(opts ChannelDoneOptions) ChannelDoneOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.ArtifactKind = strings.ToLower(strings.TrimSpace(opts.ArtifactKind))
	opts.ArtifactID = strings.TrimSpace(opts.ArtifactID)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	return opts
}

func validateChannelDoneOptions(opts ChannelDoneOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.ArtifactIssue.Number <= 0 {
		return fmt.Errorf("missing channel artifact issue")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source channel issue")
	}
	if opts.ArtifactKind == "" {
		return fmt.Errorf("missing channel artifact kind")
	}
	if opts.ArtifactID == "" {
		return fmt.Errorf("missing channel artifact id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing channel done notify message id")
	}
	return nil
}

func autoChannelDoneNotifyMessageID(ev Event, ref channelDoneArtifactRef) string {
	seed := strings.Join([]string{eventID(ev), ref.Kind, ref.ID, strconv.Itoa(ev.Issue.Number)}, "|")
	return "gitclaw-channel-done-" + shortDocumentHash(seed)
}
