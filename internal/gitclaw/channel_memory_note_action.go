package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelMemoryNoteOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	NoteID            string
	MemoryTarget      string
	Title             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelMemoryNoteResult struct {
	MemoryNoteIssueNumber int
	MemoryNoteIssueURL    string
	MemoryNoteCreated     bool
	MemoryNoteDuplicate   bool
	Notification          ChannelSendResult
	RouteName             string
	RouteHash             string
	Channel               string
	ThreadHash            string
	MessageHash           string
	NotifyHash            string
}

type ChannelMemoryNoteActionRequest struct {
	Options             ChannelMemoryNoteOptions
	Command             string
	Subcommand          string
	AutoNoteID          bool
	AutoMemoryTarget    bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	MemoryTargetSHA     string
	MemoryTargetBytes   int
	MemoryTargetLines   int
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelMemoryNoteActionRequest(ev Event, cfg Config) bool {
	return isChannelMemoryNoteActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelMemoryNoteActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "memory-note", "memory-lesson", "memory-guidance", "memory-observation", "memory-capture-note", "capture-memory-note":
		return true
	default:
		return false
	}
}

func BuildChannelMemoryNoteActionRequest(ev Event, cfg Config) (ChannelMemoryNoteActionRequest, error) {
	fields, trailing, ok := channelMemoryNoteActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelMemoryNoteActionRequest{}, fmt.Errorf("missing channel memory note command")
	}
	req := ChannelMemoryNoteActionRequest{
		Options: ChannelMemoryNoteOptions{
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
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--note-id", "--memory-note-id", "--id":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NoteID = cleanChannelMemoryNoteID(fields[i+1])
			i++
		case "--target", "--domain", "--scope", "--memory-target":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MemoryTarget = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMemoryNoteActionRequest{}, fmt.Errorf("unknown channel memory note argument %q", field)
			}
			if req.Options.NoteID == "" {
				req.Options.NoteID = cleanChannelMemoryNoteID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelMemoryNoteActionRequest{}, fmt.Errorf("unexpected channel memory note argument %q", field)
		}
	}
	if err := applyChannelMemoryNoteIssueTarget(ev, &req); err != nil {
		return ChannelMemoryNoteActionRequest{}, err
	}
	bodyMemoryTarget, title, note := parseChannelMemoryNoteBody(trailing, ev)
	if strings.TrimSpace(req.Options.MemoryTarget) == "" {
		req.Options.MemoryTarget = bodyMemoryTarget
	}
	if strings.TrimSpace(req.Options.MemoryTarget) == "" {
		req.Options.MemoryTarget = "unspecified-memory-target"
		req.AutoMemoryTarget = true
	}
	req.Options.Title = title
	req.Options.Note = note
	if strings.TrimSpace(req.Options.NoteID) == "" {
		req.Options.NoteID = autoChannelMemoryNoteID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.MemoryTarget, title, note)
		req.AutoNoteID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMemoryNoteNotifyMessageID(ev, req.Options.NoteID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMemoryNoteOptions(req.Options)
	if err := validateChannelMemoryNoteActionRequestOptions(req.Options); err != nil {
		return ChannelMemoryNoteActionRequest{}, err
	}
	req.MemoryTargetSHA = shortDocumentHash(req.Options.MemoryTarget)
	req.MemoryTargetBytes = len(req.Options.MemoryTarget)
	req.MemoryTargetLines = lineCount(req.Options.MemoryTarget)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelMemoryNoteNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelMemoryNote(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMemoryNoteOptions) (ChannelMemoryNoteResult, error) {
	opts = normalizeChannelMemoryNoteOptions(opts)
	var err error
	opts, err = applyChannelMemoryNoteRoute(cfg, opts)
	if err != nil {
		return ChannelMemoryNoteResult{}, err
	}
	if err := validateChannelMemoryNoteOptions(opts); err != nil {
		return ChannelMemoryNoteResult{}, err
	}
	noteIssue, created, duplicate, err := findOrCreateChannelMemoryNoteIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelMemoryNoteResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelMemoryNoteNotificationBody(opts, noteIssue.Number, issueURL(opts.Repo, noteIssue.Number)),
	})
	if err != nil {
		return ChannelMemoryNoteResult{}, fmt.Errorf("queue channel memory note notification: %w", err)
	}
	return ChannelMemoryNoteResult{
		MemoryNoteIssueNumber: noteIssue.Number,
		MemoryNoteIssueURL:    issueURL(opts.Repo, noteIssue.Number),
		MemoryNoteCreated:     created,
		MemoryNoteDuplicate:   duplicate,
		Notification:          notification,
		RouteName:             opts.Route,
		RouteHash:             channelRouteHash(opts.Route),
		Channel:               opts.Channel,
		ThreadHash:            shortDocumentHash(opts.ThreadID),
		MessageHash:           shortDocumentHash(opts.SourceMessageID),
		NotifyHash:            shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelMemoryNoteActionReport(ev Event, req ChannelMemoryNoteActionRequest, result ChannelMemoryNoteResult) string {
	status := "captured"
	switch {
	case result.MemoryNoteDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.MemoryNoteDuplicate:
		status = "existing"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
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
	b.WriteString("## GitClaw Channel Memory Note Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_memory_note_status: `%s`\n", status)
	fmt.Fprintf(&b, "- memory_note_issue: `#%d`\n", result.MemoryNoteIssueNumber)
	fmt.Fprintf(&b, "- memory_note_issue_url: `%s`\n", result.MemoryNoteIssueURL)
	fmt.Fprintf(&b, "- memory_note_issue_created: `%t`\n", result.MemoryNoteCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.MemoryNoteDuplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- memory_note_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.NoteID))
	fmt.Fprintf(&b, "- memory_note_id_auto: `%t`\n", req.AutoNoteID)
	fmt.Fprintf(&b, "- memory_target_sha256_12: `%s`\n", req.MemoryTargetSHA)
	fmt.Fprintf(&b, "- memory_target_bytes: `%d`\n", req.MemoryTargetBytes)
	fmt.Fprintf(&b, "- memory_target_lines: `%d`\n", req.MemoryTargetLines)
	fmt.Fprintf(&b, "- memory_target_auto: `%t`\n", req.AutoMemoryTarget)
	fmt.Fprintf(&b, "- memory_note_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- memory_note_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- memory_note_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- memory_note_text_sha256_12: `%s`\n", req.NoteSHA)
	fmt.Fprintf(&b, "- memory_note_text_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- memory_note_text_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_memory_note_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_target_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_note_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_note_text_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_promotion_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_memory_note_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin memory note as a durable GitHub issue, then queued a provider-facing memory-note link back to the mirrored thread. The memory-note issue contains the human-readable target, title, and note; this source receipt keeps provider IDs, note IDs, memory targets, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the memory-note notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent memory-note links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate memory-note issues are suppressed by `note_id`; duplicate memory-note notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the memory-note issue with the `gitclaw` label; memory mutation remains an explicit reviewed follow-up\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelMemoryNoteIssueBody(opts ChannelMemoryNoteOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-memory-note note_id=\"%s\" channel=\"%s\" memory_target_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.NoteID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.MemoryTarget), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel memory note.\n\n")
	fmt.Fprintf(&b, "- note_id: %s\n", opts.NoteID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- memory_note_mode: github-issue-memory-note\n")
	fmt.Fprintf(&b, "- memory_write_performed: false\n")
	fmt.Fprintf(&b, "- memory_promotion_performed: false\n")
	fmt.Fprintf(&b, "- memory_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Target\n\n")
	b.WriteString(strings.TrimSpace(opts.MemoryTarget))
	b.WriteString("\n\n## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Note) != "" {
		b.WriteString("\n\n## Note\n\n")
		b.WriteString(strings.TrimSpace(opts.Note))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the channel memory note without writing `.gitclaw/MEMORY.md`, promoting memory, mutating memory, or changing repository files automatically.")
	return strings.TrimSpace(b.String())
}

func channelMemoryNoteActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelMemoryNoteActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelMemoryNoteIssueTarget(ev Event, req *ChannelMemoryNoteActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel memory note requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelMemoryNoteBody(trailing string, ev Event) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel memory note from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return "", defaultTitle, ""
	}
	var memoryTarget string
	var title string
	var noteLines []string
	mode := ""
	for _, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "target:"):
			if memoryTarget == "" {
				memoryTarget = strings.TrimSpace(trimmed[len("target:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "domain:"):
			if memoryTarget == "" {
				memoryTarget = strings.TrimSpace(trimmed[len("domain:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "scope:"):
			if memoryTarget == "" {
				memoryTarget = strings.TrimSpace(trimmed[len("scope:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "memory target:"):
			if memoryTarget == "" {
				memoryTarget = strings.TrimSpace(trimmed[len("memory target:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "title:"):
			title = strings.TrimSpace(trimmed[len("title:"):])
			mode = ""
		case strings.HasPrefix(lower, "note:"):
			rest := strings.TrimSpace(trimmed[len("note:"):])
			if rest != "" {
				noteLines = append(noteLines, rest)
			}
			mode = "note"
		case strings.HasPrefix(lower, "context:"), strings.HasPrefix(lower, "notes:"), strings.HasPrefix(lower, "source:"):
			rest := trimmed
			if idx := strings.Index(rest, ":"); idx >= 0 {
				rest = strings.TrimSpace(rest[idx+1:])
			}
			if rest != "" {
				noteLines = append(noteLines, rest)
			}
			mode = "note"
		default:
			if mode == "note" {
				noteLines = append(noteLines, line)
				continue
			}
			if title == "" {
				title = trimmed
				mode = ""
				continue
			}
			noteLines = append(noteLines, line)
		}
	}
	if title == "" {
		title = defaultTitle
	}
	note := stripChannelMemoryNoteNoteHeader(strings.TrimSpace(strings.Join(noteLines, "\n")))
	return memoryTarget, title, note
}

func stripChannelMemoryNoteNoteHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"note:", "context:", "notes:", "source:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

func normalizeChannelMemoryNoteOptions(opts ChannelMemoryNoteOptions) ChannelMemoryNoteOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.NoteID = cleanChannelMemoryNoteID(opts.NoteID)
	opts.MemoryTarget = strings.TrimSpace(opts.MemoryTarget)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Note = strings.TrimSpace(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelMemoryNoteRoute(cfg Config, opts ChannelMemoryNoteOptions) (ChannelMemoryNoteOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      opts.Title,
	})
	if err != nil {
		return opts, err
	}
	opts.Route = routeOpts.Route
	opts.Channel = routeOpts.Channel
	opts.ThreadID = routeOpts.ThreadID
	opts.Author = routeOpts.Author
	return opts, nil
}

func validateChannelMemoryNoteOptions(opts ChannelMemoryNoteOptions) error {
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
	if opts.NoteID == "" {
		return fmt.Errorf("missing memory note id")
	}
	if opts.MemoryTarget == "" {
		return fmt.Errorf("missing memory target")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing memory note source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing memory note title")
	}
	return nil
}

func validateChannelMemoryNoteActionRequestOptions(opts ChannelMemoryNoteOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.SourceMessageID == "" {
		return fmt.Errorf("missing source message id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.NoteID == "" {
		return fmt.Errorf("missing memory note id")
	}
	if opts.MemoryTarget == "" {
		return fmt.Errorf("missing memory target")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing memory note source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing memory note title")
	}
	return nil
}

func findOrCreateChannelMemoryNoteIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMemoryNoteOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel memory note issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelMemoryNoteMatches(issue.Body, opts.NoteID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelMemoryNoteIssueTitle(opts), RenderChannelMemoryNoteIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel memory note issue: %w", err)
	}
	return issue, true, false, nil
}

func channelMemoryNoteIssueTitle(opts ChannelMemoryNoteOptions) string {
	memoryTarget := strings.TrimSpace(opts.MemoryTarget)
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	switch {
	case memoryTarget != "" && title != "":
		title = memoryTarget + ": " + title
	case memoryTarget != "":
		title = memoryTarget
	case title == "":
		title = opts.NoteID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel memory note: " + title
}

func channelMemoryNoteMatches(body, noteID string) bool {
	return HasChannelMemoryNoteMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`note_id="%s"`, escapeMarkerValue(cleanChannelMemoryNoteID(noteID))))
}

func cleanChannelMemoryNoteID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelMemoryNoteID(ev Event, channel, threadID, sourceMessageID, memoryTarget, title, note string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, memoryTarget, title, note}, "|")
	return fmt.Sprintf("memory-note-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMemoryNoteNotifyMessageID(ev Event, noteID string) string {
	seed := strings.Join([]string{eventID(ev), noteID}, "|")
	return fmt.Sprintf("gitclaw-channel-memory-note-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelMemoryNoteNotificationBody(opts ChannelMemoryNoteOptions, noteIssueNumber int, noteIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel memory note captured.\n\n")
	if noteIssueNumber > 0 {
		fmt.Fprintf(&b, "Memory note: #%d\n", noteIssueNumber)
	}
	if noteIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", noteIssueURL)
	}
	fmt.Fprintf(&b, "Target: %s\n", strings.TrimSpace(opts.MemoryTarget))
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue reviewing it in the linked GitHub issue. Writing `.gitclaw/MEMORY.md` or promoting durable memory remains a separate reviewed step.")
	return strings.TrimSpace(b.String())
}
