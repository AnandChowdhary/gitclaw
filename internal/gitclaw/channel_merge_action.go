package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelMergeOptions struct {
	Repo              string
	Channel           string
	SourceThreadID    string
	TargetThreadID    string
	SourceMessageID   string
	NotifyMessageID   string
	MergeID           string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelMergeResult struct {
	MergeIssueNumber int
	MergeIssueURL    string
	MergeCreated     bool
	MergeDuplicate   bool
	Notification     ChannelSendResult
	Channel          string
	SourceThreadSHA  string
	TargetThreadSHA  string
	MessageHash      string
	NotifyHash       string
}

type ChannelMergeActionRequest struct {
	Options             ChannelMergeOptions
	Command             string
	Subcommand          string
	AutoMergeID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	NotificationBodySHA string
}

func IsChannelMergeActionRequest(ev Event, cfg Config) bool {
	return isChannelMergeActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelMergeActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "merge", "merge-thread", "thread-merge", "merge-back", "rejoin", "converge", "join-thread":
		return true
	default:
		return false
	}
}

func BuildChannelMergeActionRequest(ev Event, cfg Config) (ChannelMergeActionRequest, error) {
	fields, trailing, ok := channelMergeActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelMergeActionRequest{}, fmt.Errorf("missing channel merge command")
	}
	req := ChannelMergeActionRequest{
		Options: ChannelMergeOptions{
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
				return ChannelMergeActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--source-thread-id", "--source-thread", "--from-thread", "--from-thread-id":
			if i+1 >= len(fields) {
				return ChannelMergeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceThreadID = fields[i+1]
			i++
		case "--target-thread-id", "--target-thread", "--to-thread-id", "--to-thread", "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMergeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMergeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMergeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--merge-id", "--join-id", "--converge-id", "--id":
			if i+1 >= len(fields) {
				return ChannelMergeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MergeID = cleanChannelMergeID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMergeActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMergeActionRequest{}, fmt.Errorf("unknown channel merge argument %q", field)
			}
			if req.Options.MergeID == "" {
				req.Options.MergeID = cleanChannelMergeID(field)
				continue
			}
			return ChannelMergeActionRequest{}, fmt.Errorf("unexpected channel merge argument %q", field)
		}
	}
	if err := applyChannelMergeIssueTarget(ev, &req); err != nil {
		return ChannelMergeActionRequest{}, err
	}
	title, notes := parseChannelMergeTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.MergeID) == "" {
		req.Options.MergeID = autoChannelMergeID(ev, req.Options.Channel, req.Options.SourceThreadID, req.Options.TargetThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoMergeID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMergeNotifyMessageID(ev, req.Options.MergeID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMergeOptions(req.Options)
	if err := validateChannelMergeOptions(req.Options); err != nil {
		return ChannelMergeActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.NotificationBodySHA = shortDocumentHash(renderChannelMergeNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelMerge(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMergeOptions) (ChannelMergeResult, error) {
	opts = normalizeChannelMergeOptions(opts)
	if err := validateChannelMergeOptions(opts); err != nil {
		return ChannelMergeResult{}, err
	}
	mergeIssue, created, duplicate, err := findOrCreateChannelMergeIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelMergeResult{}, err
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, mergeIssue.Number, []string{cfg.TriggerLabel, cfg.ChannelLabel}); err != nil {
		return ChannelMergeResult{}, fmt.Errorf("label channel merge issue: %w", err)
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.TargetThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelMergeNotificationBody(opts, mergeIssue.Number, issueURL(opts.Repo, mergeIssue.Number)),
	})
	if err != nil {
		return ChannelMergeResult{}, fmt.Errorf("queue channel merge notification: %w", err)
	}
	return ChannelMergeResult{
		MergeIssueNumber: mergeIssue.Number,
		MergeIssueURL:    issueURL(opts.Repo, mergeIssue.Number),
		MergeCreated:     created,
		MergeDuplicate:   duplicate,
		Notification:     notification,
		Channel:          opts.Channel,
		SourceThreadSHA:  shortDocumentHash(opts.SourceThreadID),
		TargetThreadSHA:  shortDocumentHash(opts.TargetThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelMergeActionReport(ev Event, req ChannelMergeActionRequest, result ChannelMergeResult) string {
	status := "recorded"
	switch {
	case result.MergeDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.MergeDuplicate:
		status = "existing"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	sourceThreadSHA := result.SourceThreadSHA
	if sourceThreadSHA == "" {
		sourceThreadSHA = shortDocumentHash(req.Options.SourceThreadID)
	}
	targetThreadSHA := result.TargetThreadSHA
	if targetThreadSHA == "" {
		targetThreadSHA = shortDocumentHash(req.Options.TargetThreadID)
	}
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = shortDocumentHash(req.Options.SourceMessageID)
	}
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = shortDocumentHash(req.Options.NotifyMessageID)
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Merge Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_merge_status: `%s`\n", status)
	fmt.Fprintf(&b, "- merge_issue: `#%d`\n", result.MergeIssueNumber)
	fmt.Fprintf(&b, "- merge_issue_url: `%s`\n", result.MergeIssueURL)
	fmt.Fprintf(&b, "- merge_issue_created: `%t`\n", result.MergeCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.MergeDuplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- source_thread_id_sha256_12: `%s`\n", sourceThreadSHA)
	fmt.Fprintf(&b, "- target_thread_id_sha256_12: `%s`\n", targetThreadSHA)
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", messageHash)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", notifyHash)
	fmt.Fprintf(&b, "- merge_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MergeID))
	fmt.Fprintf(&b, "- merge_id_auto: `%t`\n", req.AutoMergeID)
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- merge_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- merge_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- merge_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- merge_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- merge_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- merge_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_merge_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_merge_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_merge_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_merge_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a channel-thread merge as a durable GitHub issue, then queued a provider-facing merge acknowledgement back to the target thread. The merge issue is the reviewable convergence record; this source receipt keeps provider thread IDs, message IDs, merge IDs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the merge acknowledgement with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent merge acknowledgements with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate merge issues are suppressed by `merge_id`; duplicate notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelMergeIssueBody(opts ChannelMergeOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-merge merge_id=\"%s\" channel=\"%s\" source_thread_id_sha256_12=\"%s\" target_thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.MergeID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.SourceThreadID), shortDocumentHash(opts.TargetThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel thread merge.\n\n")
	fmt.Fprintf(&b, "- merge_id: %s\n", opts.MergeID)
	fmt.Fprintf(&b, "- channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- source_thread_id_sha256_12: %s\n", shortDocumentHash(opts.SourceThreadID))
	fmt.Fprintf(&b, "- target_thread_id_sha256_12: %s\n", shortDocumentHash(opts.TargetThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- merge_mode: github-issue-channel-thread-merge\n")
	fmt.Fprintf(&b, "- model_call_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_source_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_target_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Merge\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub convergence record for the channel-thread merge.")
	return strings.TrimSpace(b.String())
}

func channelMergeActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelMergeActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelMergeIssueTarget(ev Event, req *ChannelMergeActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" && strings.TrimSpace(req.Options.TargetThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel merge requires a gitclaw:channel-thread issue or explicit channel/target thread")
	}
	if req.Options.Channel == "" {
		req.Options.Channel = channel
	}
	if req.Options.TargetThreadID == "" {
		req.Options.TargetThreadID = threadID
	}
	req.TargetFromIssue = true
	return nil
}

func parseChannelMergeTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel merge from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var title string
	var noteLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "title:"):
		title = strings.TrimSpace(first[len("title:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "merge:"):
		title = strings.TrimSpace(first[len("merge:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "topic:"):
		title = strings.TrimSpace(first[len("topic:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"), strings.HasPrefix(lowerFirst, "reason:"):
		title = defaultTitle
		noteLines = cleaned
	default:
		title = first
		noteLines = cleaned[1:]
	}
	if title == "" {
		title = defaultTitle
	}
	notes := strings.TrimSpace(strings.Join(noteLines, "\n"))
	notesLower := strings.ToLower(strings.TrimSpace(notes))
	switch {
	case strings.HasPrefix(notesLower, "notes:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("notes:"):])
	case strings.HasPrefix(notesLower, "context:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("context:"):])
	case strings.HasPrefix(notesLower, "reason:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("reason:"):])
	}
	return title, notes
}

func normalizeChannelMergeOptions(opts ChannelMergeOptions) ChannelMergeOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.SourceThreadID = strings.TrimSpace(opts.SourceThreadID)
	opts.TargetThreadID = strings.TrimSpace(opts.TargetThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MergeID = cleanChannelMergeID(opts.MergeID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelMergeOptions(opts ChannelMergeOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.SourceThreadID == "" {
		return fmt.Errorf("missing source thread id")
	}
	if opts.TargetThreadID == "" {
		return fmt.Errorf("missing target thread id")
	}
	if opts.SourceThreadID == opts.TargetThreadID {
		return fmt.Errorf("source and target thread ids must differ")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.MergeID == "" {
		return fmt.Errorf("missing merge id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing merge source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing merge title")
	}
	return nil
}

func findOrCreateChannelMergeIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMergeOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, nil, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel merge issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelMergeMatches(issue.Body, opts.MergeID) {
			return issue, false, true, nil
		}
	}
	title := fmt.Sprintf("GitClaw channel merge: %s", opts.Title)
	issue, err := github.CreateIssue(ctx, opts.Repo, title, RenderChannelMergeIssueBody(opts), []string{cfg.TriggerLabel, cfg.ChannelLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel merge issue: %w", err)
	}
	return issue, true, false, nil
}

func channelMergeMatches(body, mergeID string) bool {
	return HasChannelMergeMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`merge_id="%s"`, escapeMarkerValue(cleanChannelMergeID(mergeID))))
}

func cleanChannelMergeID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelMergeID(ev Event, channel, sourceThreadID, targetThreadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, sourceThreadID, targetThreadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("merge-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMergeNotifyMessageID(ev Event, mergeID string) string {
	seed := strings.Join([]string{eventID(ev), mergeID}, "|")
	return fmt.Sprintf("gitclaw-channel-merge-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelMergeNotificationBody(opts ChannelMergeOptions, mergeIssueNumber int, mergeIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel thread merge recorded.\n\n")
	if mergeIssueNumber > 0 {
		fmt.Fprintf(&b, "Merge: #%d\n", mergeIssueNumber)
	}
	if mergeIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", mergeIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nReview the linked GitHub merge record.")
	return strings.TrimSpace(b.String())
}
