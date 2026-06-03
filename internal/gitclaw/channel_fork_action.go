package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelForkOptions struct {
	Repo              string
	Channel           string
	SourceThreadID    string
	TargetThreadID    string
	SourceMessageID   string
	NotifyMessageID   string
	ForkID            string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelForkResult struct {
	ForkIssueNumber int
	ForkIssueURL    string
	ForkCreated     bool
	ForkDuplicate   bool
	Notification    ChannelSendResult
	Channel         string
	SourceThreadSHA string
	TargetThreadSHA string
	MessageHash     string
	NotifyHash      string
}

type ChannelForkActionRequest struct {
	Options             ChannelForkOptions
	Command             string
	Subcommand          string
	AutoForkID          bool
	AutoTargetThreadID  bool
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

func IsChannelForkActionRequest(ev Event, cfg Config) bool {
	return isChannelForkActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelForkActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "fork", "branch", "split", "thread-fork", "fork-thread", "branch-thread", "split-thread":
		return true
	default:
		return false
	}
}

func BuildChannelForkActionRequest(ev Event, cfg Config) (ChannelForkActionRequest, error) {
	fields, trailing, ok := channelForkActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelForkActionRequest{}, fmt.Errorf("missing channel fork command")
	}
	req := ChannelForkActionRequest{
		Options: ChannelForkOptions{
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
				return ChannelForkActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--source-thread-id", "--source-thread", "--from-thread", "--from-thread-id":
			if i+1 >= len(fields) {
				return ChannelForkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceThreadID = fields[i+1]
			i++
		case "--new-thread-id", "--target-thread-id", "--to-thread-id", "--to-thread", "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelForkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelForkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelForkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--fork-id", "--branch-id", "--split-id", "--id":
			if i+1 >= len(fields) {
				return ChannelForkActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ForkID = cleanChannelForkID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelForkActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelForkActionRequest{}, fmt.Errorf("unknown channel fork argument %q", field)
			}
			if req.Options.ForkID == "" {
				req.Options.ForkID = cleanChannelForkID(field)
				continue
			}
			return ChannelForkActionRequest{}, fmt.Errorf("unexpected channel fork argument %q", field)
		}
	}
	if err := applyChannelForkIssueTarget(ev, &req); err != nil {
		return ChannelForkActionRequest{}, err
	}
	title, notes := parseChannelForkTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.TargetThreadID) == "" {
		req.Options.TargetThreadID = autoChannelForkTargetThreadID(ev, req.Options.Channel, req.Options.SourceThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoTargetThreadID = true
	}
	if strings.TrimSpace(req.Options.ForkID) == "" {
		req.Options.ForkID = autoChannelForkID(ev, req.Options.Channel, req.Options.SourceThreadID, req.Options.TargetThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoForkID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelForkNotifyMessageID(ev, req.Options.ForkID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelForkOptions(req.Options)
	if err := validateChannelForkOptions(req.Options); err != nil {
		return ChannelForkActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	notificationBody := renderChannelForkNotificationBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	return req, nil
}

func RunChannelFork(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelForkOptions) (ChannelForkResult, error) {
	opts = normalizeChannelForkOptions(opts)
	if err := validateChannelForkOptions(opts); err != nil {
		return ChannelForkResult{}, err
	}
	forkIssue, created, duplicate, err := findOrCreateChannelForkIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelForkResult{}, err
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, forkIssue.Number, []string{cfg.TriggerLabel, cfg.ChannelLabel}); err != nil {
		return ChannelForkResult{}, fmt.Errorf("label channel fork issue: %w", err)
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.SourceThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelForkNotificationBody(opts, forkIssue.Number, issueURL(opts.Repo, forkIssue.Number)),
	})
	if err != nil {
		return ChannelForkResult{}, fmt.Errorf("queue channel fork notification: %w", err)
	}
	return ChannelForkResult{
		ForkIssueNumber: forkIssue.Number,
		ForkIssueURL:    issueURL(opts.Repo, forkIssue.Number),
		ForkCreated:     created,
		ForkDuplicate:   duplicate,
		Notification:    notification,
		Channel:         opts.Channel,
		SourceThreadSHA: shortDocumentHash(opts.SourceThreadID),
		TargetThreadSHA: shortDocumentHash(opts.TargetThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelForkActionReport(ev Event, req ChannelForkActionRequest, result ChannelForkResult) string {
	status := "created"
	switch {
	case result.ForkDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ForkDuplicate:
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
	b.WriteString("## GitClaw Channel Fork Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_fork_status: `%s`\n", status)
	fmt.Fprintf(&b, "- fork_issue: `#%d`\n", result.ForkIssueNumber)
	fmt.Fprintf(&b, "- fork_issue_url: `%s`\n", result.ForkIssueURL)
	fmt.Fprintf(&b, "- fork_issue_created: `%t`\n", result.ForkCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ForkDuplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- source_thread_id_sha256_12: `%s`\n", sourceThreadSHA)
	fmt.Fprintf(&b, "- target_thread_id_sha256_12: `%s`\n", targetThreadSHA)
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", messageHash)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", notifyHash)
	fmt.Fprintf(&b, "- fork_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ForkID))
	fmt.Fprintf(&b, "- fork_id_auto: `%t`\n", req.AutoForkID)
	fmt.Fprintf(&b, "- target_thread_id_auto: `%t`\n", req.AutoTargetThreadID)
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- fork_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- fork_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- fork_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- fork_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- fork_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- fork_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_fork_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fork_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fork_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_fork_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a new GitHub-backed channel thread, then queued a provider-facing fork acknowledgement back to the source thread. The fork issue is a normal channel thread where conversation can continue; this source receipt keeps provider thread IDs, message IDs, fork IDs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the fork acknowledgement with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent fork acknowledgements with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate fork issues are suppressed by channel and target thread id; duplicate notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelForkThreadBody(opts ChannelForkOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-thread channel=\"%s\" thread_id=\"%s\" -->\n", escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.TargetThreadID))
	fmt.Fprintf(&b, "<!-- gitclaw:channel-fork fork_id=\"%s\" channel=\"%s\" source_thread_id_sha256_12=\"%s\" target_thread_id=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ForkID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.SourceThreadID), escapeMarkerValue(opts.TargetThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw forked channel thread.\n\n")
	fmt.Fprintf(&b, "- channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- thread_id: %s\n", opts.TargetThreadID)
	fmt.Fprintf(&b, "- fork_id: %s\n", opts.ForkID)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- source_thread_id_sha256_12: %s\n", shortDocumentHash(opts.SourceThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- fork_mode: github-issue-channel-thread-fork\n")
	fmt.Fprintf(&b, "- model_call_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_source_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Fork\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nMessages for this forked channel thread can continue in this issue as normal GitClaw comments.")
	return strings.TrimSpace(b.String())
}

func channelForkActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelForkActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelForkIssueTarget(ev Event, req *ChannelForkActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" && strings.TrimSpace(req.Options.SourceThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel fork requires a gitclaw:channel-thread issue or explicit channel/source thread")
	}
	if req.Options.Channel == "" {
		req.Options.Channel = channel
	}
	if req.Options.SourceThreadID == "" {
		req.Options.SourceThreadID = threadID
	}
	req.TargetFromIssue = true
	return nil
}

func parseChannelForkTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel fork from issue #%d", ev.Issue.Number)
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
	case strings.HasPrefix(lowerFirst, "fork:"):
		title = strings.TrimSpace(first[len("fork:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "topic:"):
		title = strings.TrimSpace(first[len("topic:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"):
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
	}
	return title, notes
}

func normalizeChannelForkOptions(opts ChannelForkOptions) ChannelForkOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.SourceThreadID = strings.TrimSpace(opts.SourceThreadID)
	opts.TargetThreadID = strings.TrimSpace(opts.TargetThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ForkID = cleanChannelForkID(opts.ForkID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelForkOptions(opts ChannelForkOptions) error {
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
		return fmt.Errorf("target thread id must differ from source thread id")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.ForkID == "" {
		return fmt.Errorf("missing fork id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing fork source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing fork title")
	}
	return nil
}

func findOrCreateChannelForkIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelForkOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, nil, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel fork issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelThreadMatches(issue.Body, opts.Channel, opts.TargetThreadID) {
			return issue, false, true, nil
		}
	}
	title := fmt.Sprintf("GitClaw %s thread %s", opts.Channel, opts.TargetThreadID)
	issue, err := github.CreateIssue(ctx, opts.Repo, title, RenderChannelForkThreadBody(opts), []string{cfg.TriggerLabel, cfg.ChannelLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel fork issue: %w", err)
	}
	return issue, true, false, nil
}

func channelForkMatches(body, forkID string) bool {
	return HasChannelForkMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`fork_id="%s"`, escapeMarkerValue(cleanChannelForkID(forkID))))
}

func cleanChannelForkID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelForkID(ev Event, channel, sourceThreadID, targetThreadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, sourceThreadID, targetThreadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("fork-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelForkTargetThreadID(ev Event, channel, sourceThreadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, sourceThreadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("gitclaw-fork-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelForkNotifyMessageID(ev Event, forkID string) string {
	seed := strings.Join([]string{eventID(ev), forkID}, "|")
	return fmt.Sprintf("gitclaw-channel-fork-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelForkNotificationBody(opts ChannelForkOptions, forkIssueNumber int, forkIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel thread forked.\n\n")
	if forkIssueNumber > 0 {
		fmt.Fprintf(&b, "Fork: #%d\n", forkIssueNumber)
	}
	if forkIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", forkIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue in the linked GitHub channel thread.")
	return strings.TrimSpace(b.String())
}
