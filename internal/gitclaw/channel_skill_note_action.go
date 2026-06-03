package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSkillNoteOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	NoteID            string
	SkillName         string
	Title             string
	Lesson            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSkillNoteResult struct {
	SkillNoteIssueNumber int
	SkillNoteIssueURL    string
	SkillNoteCreated     bool
	SkillNoteDuplicate   bool
	Notification         ChannelSendResult
	RouteName            string
	RouteHash            string
	Channel              string
	ThreadHash           string
	MessageHash          string
	NotifyHash           string
}

type ChannelSkillNoteActionRequest struct {
	Options             ChannelSkillNoteOptions
	Command             string
	Subcommand          string
	AutoNoteID          bool
	AutoSkillName       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	SkillNameSHA        string
	SkillNameBytes      int
	SkillNameLines      int
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	LessonSHA           string
	LessonBytes         int
	LessonLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelSkillNoteActionRequest(ev Event, cfg Config) bool {
	return isChannelSkillNoteActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSkillNoteActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "skill-note", "skill-lesson", "lesson", "skill-tip", "capture-skill-note":
		return true
	default:
		return false
	}
}

func BuildChannelSkillNoteActionRequest(ev Event, cfg Config) (ChannelSkillNoteActionRequest, error) {
	fields, trailing, ok := channelSkillNoteActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSkillNoteActionRequest{}, fmt.Errorf("missing channel skill note command")
	}
	req := ChannelSkillNoteActionRequest{
		Options: ChannelSkillNoteOptions{
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
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--note-id", "--skill-note-id", "--lesson-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NoteID = cleanChannelSkillNoteID(fields[i+1])
			i++
		case "--skill", "--skill-name":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SkillName = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillNoteActionRequest{}, fmt.Errorf("unknown channel skill note argument %q", field)
			}
			if req.Options.NoteID == "" {
				req.Options.NoteID = cleanChannelSkillNoteID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelSkillNoteActionRequest{}, fmt.Errorf("unexpected channel skill note argument %q", field)
		}
	}
	if err := applyChannelSkillNoteIssueTarget(ev, &req); err != nil {
		return ChannelSkillNoteActionRequest{}, err
	}
	bodySkillName, title, lesson := parseChannelSkillNoteBody(trailing, ev)
	if strings.TrimSpace(req.Options.SkillName) == "" {
		req.Options.SkillName = bodySkillName
	}
	if strings.TrimSpace(req.Options.SkillName) == "" {
		req.Options.SkillName = "unspecified-channel-skill"
		req.AutoSkillName = true
	}
	req.Options.Title = title
	req.Options.Lesson = lesson
	if strings.TrimSpace(req.Options.NoteID) == "" {
		req.Options.NoteID = autoChannelSkillNoteID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.SkillName, title, lesson)
		req.AutoNoteID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillNoteNotifyMessageID(ev, req.Options.NoteID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillNoteOptions(req.Options)
	if err := validateChannelSkillNoteActionRequestOptions(req.Options); err != nil {
		return ChannelSkillNoteActionRequest{}, err
	}
	req.SkillNameSHA = shortDocumentHash(req.Options.SkillName)
	req.SkillNameBytes = len(req.Options.SkillName)
	req.SkillNameLines = lineCount(req.Options.SkillName)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.LessonSHA = shortDocumentHash(req.Options.Lesson)
	req.LessonBytes = len(req.Options.Lesson)
	req.LessonLines = lineCount(req.Options.Lesson)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelSkillNoteNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelSkillNote(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSkillNoteOptions) (ChannelSkillNoteResult, error) {
	opts = normalizeChannelSkillNoteOptions(opts)
	var err error
	opts, err = applyChannelSkillNoteRoute(cfg, opts)
	if err != nil {
		return ChannelSkillNoteResult{}, err
	}
	if err := validateChannelSkillNoteOptions(opts); err != nil {
		return ChannelSkillNoteResult{}, err
	}
	noteIssue, created, duplicate, err := findOrCreateChannelSkillNoteIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelSkillNoteResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelSkillNoteNotificationBody(opts, noteIssue.Number, issueURL(opts.Repo, noteIssue.Number)),
	})
	if err != nil {
		return ChannelSkillNoteResult{}, fmt.Errorf("queue channel skill note notification: %w", err)
	}
	return ChannelSkillNoteResult{
		SkillNoteIssueNumber: noteIssue.Number,
		SkillNoteIssueURL:    issueURL(opts.Repo, noteIssue.Number),
		SkillNoteCreated:     created,
		SkillNoteDuplicate:   duplicate,
		Notification:         notification,
		RouteName:            opts.Route,
		RouteHash:            channelRouteHash(opts.Route),
		Channel:              opts.Channel,
		ThreadHash:           shortDocumentHash(opts.ThreadID),
		MessageHash:          shortDocumentHash(opts.SourceMessageID),
		NotifyHash:           shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelSkillNoteActionReport(ev Event, req ChannelSkillNoteActionRequest, result ChannelSkillNoteResult) string {
	status := "captured"
	switch {
	case result.SkillNoteDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.SkillNoteDuplicate:
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
	b.WriteString("## GitClaw Channel Skill Note Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_note_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_note_issue: `#%d`\n", result.SkillNoteIssueNumber)
	fmt.Fprintf(&b, "- skill_note_issue_url: `%s`\n", result.SkillNoteIssueURL)
	fmt.Fprintf(&b, "- skill_note_issue_created: `%t`\n", result.SkillNoteCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.SkillNoteDuplicate)
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
	fmt.Fprintf(&b, "- skill_note_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.NoteID))
	fmt.Fprintf(&b, "- skill_note_id_auto: `%t`\n", req.AutoNoteID)
	fmt.Fprintf(&b, "- skill_name_sha256_12: `%s`\n", req.SkillNameSHA)
	fmt.Fprintf(&b, "- skill_name_bytes: `%d`\n", req.SkillNameBytes)
	fmt.Fprintf(&b, "- skill_name_lines: `%d`\n", req.SkillNameLines)
	fmt.Fprintf(&b, "- skill_name_auto: `%t`\n", req.AutoSkillName)
	fmt.Fprintf(&b, "- skill_note_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- skill_note_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- skill_note_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- skill_note_lesson_sha256_12: `%s`\n", req.LessonSHA)
	fmt.Fprintf(&b, "- skill_note_lesson_bytes: `%d`\n", req.LessonBytes)
	fmt.Fprintf(&b, "- skill_note_lesson_lines: `%d`\n", req.LessonLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_skill_note_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_note_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_note_lesson_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_note_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin skill note as a durable GitHub issue, then queued a provider-facing skill-note link back to the mirrored thread. The skill-note issue contains the human-readable skill, title, and lesson; this source receipt keeps provider IDs, note IDs, skill names, lessons, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the skill-note notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent skill-note links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate skill-note issues are suppressed by `note_id`; duplicate skill-note notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the skill-note issue with the `gitclaw` label; skill installation remains an explicit reviewed follow-up\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSkillNoteIssueBody(opts ChannelSkillNoteOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-skill-note note_id=\"%s\" channel=\"%s\" skill_name_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.NoteID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.SkillName), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel skill note.\n\n")
	fmt.Fprintf(&b, "- note_id: %s\n", opts.NoteID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- skill_note_mode: github-issue-skill-note\n")
	fmt.Fprintf(&b, "- skill_install_performed: false\n")
	fmt.Fprintf(&b, "- memory_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Skill\n\n")
	b.WriteString(strings.TrimSpace(opts.SkillName))
	b.WriteString("\n\n## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Lesson) != "" {
		b.WriteString("\n\n## Lesson\n\n")
		b.WriteString(strings.TrimSpace(opts.Lesson))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the channel skill note without installing a skill, mutating memory, or changing repository files automatically.")
	return strings.TrimSpace(b.String())
}

func channelSkillNoteActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSkillNoteActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSkillNoteIssueTarget(ev Event, req *ChannelSkillNoteActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill note requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelSkillNoteBody(trailing string, ev Event) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel skill note from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return "", defaultTitle, ""
	}
	var skillName string
	var title string
	var lessonLines []string
	mode := ""
	for _, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "skill:"):
			if skillName == "" {
				skillName = strings.TrimSpace(trimmed[len("skill:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "skill name:"):
			if skillName == "" {
				skillName = strings.TrimSpace(trimmed[len("skill name:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "title:"):
			title = strings.TrimSpace(trimmed[len("title:"):])
			mode = ""
		case strings.HasPrefix(lower, "lesson:"):
			rest := strings.TrimSpace(trimmed[len("lesson:"):])
			if rest != "" {
				lessonLines = append(lessonLines, rest)
			}
			mode = "lesson"
		case strings.HasPrefix(lower, "note:"):
			rest := strings.TrimSpace(trimmed[len("note:"):])
			if rest != "" {
				lessonLines = append(lessonLines, rest)
			}
			mode = "lesson"
		case strings.HasPrefix(lower, "context:"), strings.HasPrefix(lower, "notes:"), strings.HasPrefix(lower, "source:"):
			rest := trimmed
			if idx := strings.Index(rest, ":"); idx >= 0 {
				rest = strings.TrimSpace(rest[idx+1:])
			}
			if rest != "" {
				lessonLines = append(lessonLines, rest)
			}
			mode = "lesson"
		default:
			if mode == "lesson" {
				lessonLines = append(lessonLines, line)
				continue
			}
			if title == "" {
				title = trimmed
				mode = ""
				continue
			}
			lessonLines = append(lessonLines, line)
		}
	}
	if title == "" {
		title = defaultTitle
	}
	lesson := stripChannelSkillNoteLessonHeader(strings.TrimSpace(strings.Join(lessonLines, "\n")))
	return skillName, title, lesson
}

func stripChannelSkillNoteLessonHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"lesson:", "note:", "context:", "notes:", "source:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

func normalizeChannelSkillNoteOptions(opts ChannelSkillNoteOptions) ChannelSkillNoteOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.NoteID = cleanChannelSkillNoteID(opts.NoteID)
	opts.SkillName = strings.TrimSpace(opts.SkillName)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Lesson = strings.TrimSpace(opts.Lesson)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSkillNoteRoute(cfg Config, opts ChannelSkillNoteOptions) (ChannelSkillNoteOptions, error) {
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

func validateChannelSkillNoteOptions(opts ChannelSkillNoteOptions) error {
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
		return fmt.Errorf("missing skill note id")
	}
	if opts.SkillName == "" {
		return fmt.Errorf("missing skill name")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing skill note source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing skill note title")
	}
	return nil
}

func validateChannelSkillNoteActionRequestOptions(opts ChannelSkillNoteOptions) error {
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
		return fmt.Errorf("missing skill note id")
	}
	if opts.SkillName == "" {
		return fmt.Errorf("missing skill name")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing skill note source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing skill note title")
	}
	return nil
}

func findOrCreateChannelSkillNoteIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSkillNoteOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel skill note issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelSkillNoteMatches(issue.Body, opts.NoteID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelSkillNoteIssueTitle(opts), RenderChannelSkillNoteIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel skill note issue: %w", err)
	}
	return issue, true, false, nil
}

func channelSkillNoteIssueTitle(opts ChannelSkillNoteOptions) string {
	skillName := strings.TrimSpace(opts.SkillName)
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	switch {
	case skillName != "" && title != "":
		title = skillName + ": " + title
	case skillName != "":
		title = skillName
	case title == "":
		title = opts.NoteID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel skill note: " + title
}

func channelSkillNoteMatches(body, noteID string) bool {
	return HasChannelSkillNoteMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`note_id="%s"`, escapeMarkerValue(cleanChannelSkillNoteID(noteID))))
}

func cleanChannelSkillNoteID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelSkillNoteID(ev Event, channel, threadID, sourceMessageID, skillName, title, lesson string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, skillName, title, lesson}, "|")
	return fmt.Sprintf("skill-note-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSkillNoteNotifyMessageID(ev Event, noteID string) string {
	seed := strings.Join([]string{eventID(ev), noteID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-note-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelSkillNoteNotificationBody(opts ChannelSkillNoteOptions, noteIssueNumber int, noteIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill note captured.\n\n")
	if noteIssueNumber > 0 {
		fmt.Fprintf(&b, "Skill note: #%d\n", noteIssueNumber)
	}
	if noteIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", noteIssueURL)
	}
	fmt.Fprintf(&b, "Skill: %s\n", strings.TrimSpace(opts.SkillName))
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue reviewing it in the linked GitHub issue. Installing or updating skills remains a separate reviewed step.")
	return strings.TrimSpace(b.String())
}
