package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolLessonOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	NoteID            string
	ToolName          string
	Title             string
	Lesson            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolLessonResult struct {
	ToolLessonIssueNumber int
	ToolLessonIssueURL    string
	ToolLessonCreated     bool
	ToolLessonDuplicate   bool
	Notification          ChannelSendResult
	RouteName             string
	RouteHash             string
	Channel               string
	ThreadHash            string
	MessageHash           string
	NotifyHash            string
}

type ChannelToolLessonActionRequest struct {
	Options             ChannelToolLessonOptions
	Command             string
	Subcommand          string
	AutoNoteID          bool
	AutoToolName        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	ToolNameSHA         string
	ToolNameBytes       int
	ToolNameLines       int
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

func IsChannelToolLessonActionRequest(ev Event, cfg Config) bool {
	return isChannelToolLessonActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolLessonActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "tool-lesson", "tool-guidance", "tool-tip", "tool-rule", "capture-tool-lesson":
		return true
	default:
		return false
	}
}

func BuildChannelToolLessonActionRequest(ev Event, cfg Config) (ChannelToolLessonActionRequest, error) {
	fields, trailing, ok := channelToolLessonActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolLessonActionRequest{}, fmt.Errorf("missing channel tool lesson command")
	}
	req := ChannelToolLessonActionRequest{
		Options: ChannelToolLessonOptions{
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
				return ChannelToolLessonActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--note-id", "--tool-lesson-id", "--lesson-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NoteID = cleanChannelToolLessonID(fields[i+1])
			i++
		case "--tool", "--tool-name":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ToolName = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolLessonActionRequest{}, fmt.Errorf("unknown channel tool lesson argument %q", field)
			}
			if req.Options.NoteID == "" {
				req.Options.NoteID = cleanChannelToolLessonID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelToolLessonActionRequest{}, fmt.Errorf("unexpected channel tool lesson argument %q", field)
		}
	}
	if err := applyChannelToolLessonIssueTarget(ev, &req); err != nil {
		return ChannelToolLessonActionRequest{}, err
	}
	bodyToolName, title, lesson := parseChannelToolLessonBody(trailing, ev)
	if strings.TrimSpace(req.Options.ToolName) == "" {
		req.Options.ToolName = bodyToolName
	}
	if strings.TrimSpace(req.Options.ToolName) == "" {
		req.Options.ToolName = "unspecified-channel-tool"
		req.AutoToolName = true
	}
	req.Options.Title = title
	req.Options.Lesson = lesson
	if strings.TrimSpace(req.Options.NoteID) == "" {
		req.Options.NoteID = autoChannelToolLessonID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.ToolName, title, lesson)
		req.AutoNoteID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolLessonNotifyMessageID(ev, req.Options.NoteID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolLessonOptions(req.Options)
	if err := validateChannelToolLessonActionRequestOptions(req.Options); err != nil {
		return ChannelToolLessonActionRequest{}, err
	}
	req.ToolNameSHA = shortDocumentHash(req.Options.ToolName)
	req.ToolNameBytes = len(req.Options.ToolName)
	req.ToolNameLines = lineCount(req.Options.ToolName)
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelToolLessonNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelToolLesson(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolLessonOptions) (ChannelToolLessonResult, error) {
	opts = normalizeChannelToolLessonOptions(opts)
	var err error
	opts, err = applyChannelToolLessonRoute(cfg, opts)
	if err != nil {
		return ChannelToolLessonResult{}, err
	}
	if err := validateChannelToolLessonOptions(opts); err != nil {
		return ChannelToolLessonResult{}, err
	}
	noteIssue, created, duplicate, err := findOrCreateChannelToolLessonIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelToolLessonResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelToolLessonNotificationBody(opts, noteIssue.Number, issueURL(opts.Repo, noteIssue.Number)),
	})
	if err != nil {
		return ChannelToolLessonResult{}, fmt.Errorf("queue channel tool lesson notification: %w", err)
	}
	return ChannelToolLessonResult{
		ToolLessonIssueNumber: noteIssue.Number,
		ToolLessonIssueURL:    issueURL(opts.Repo, noteIssue.Number),
		ToolLessonCreated:     created,
		ToolLessonDuplicate:   duplicate,
		Notification:          notification,
		RouteName:             opts.Route,
		RouteHash:             channelRouteHash(opts.Route),
		Channel:               opts.Channel,
		ThreadHash:            shortDocumentHash(opts.ThreadID),
		MessageHash:           shortDocumentHash(opts.SourceMessageID),
		NotifyHash:            shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelToolLessonActionReport(ev Event, req ChannelToolLessonActionRequest, result ChannelToolLessonResult) string {
	status := "captured"
	switch {
	case result.ToolLessonDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ToolLessonDuplicate:
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
	b.WriteString("## GitClaw Channel Tool Lesson Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_lesson_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_lesson_issue: `#%d`\n", result.ToolLessonIssueNumber)
	fmt.Fprintf(&b, "- tool_lesson_issue_url: `%s`\n", result.ToolLessonIssueURL)
	fmt.Fprintf(&b, "- tool_lesson_issue_created: `%t`\n", result.ToolLessonCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ToolLessonDuplicate)
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
	fmt.Fprintf(&b, "- tool_lesson_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.NoteID))
	fmt.Fprintf(&b, "- tool_lesson_id_auto: `%t`\n", req.AutoNoteID)
	fmt.Fprintf(&b, "- tool_name_sha256_12: `%s`\n", req.ToolNameSHA)
	fmt.Fprintf(&b, "- tool_name_bytes: `%d`\n", req.ToolNameBytes)
	fmt.Fprintf(&b, "- tool_name_lines: `%d`\n", req.ToolNameLines)
	fmt.Fprintf(&b, "- tool_name_auto: `%t`\n", req.AutoToolName)
	fmt.Fprintf(&b, "- tool_lesson_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- tool_lesson_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- tool_lesson_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- tool_lesson_text_sha256_12: `%s`\n", req.LessonSHA)
	fmt.Fprintf(&b, "- tool_lesson_text_bytes: `%d`\n", req.LessonBytes)
	fmt.Fprintf(&b, "- tool_lesson_text_lines: `%d`\n", req.LessonLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_tool_lesson_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_lesson_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_lesson_text_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_lesson_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin tool lesson as a durable GitHub issue, then queued a provider-facing tool-lesson link back to the mirrored thread. The tool-lesson issue contains the human-readable tool, title, and lesson; this source receipt keeps provider IDs, note IDs, tool names, lessons, and channel message bodies out of band. It does not execute tools, install tools, update tool policy, or mutate repository state.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the tool-lesson notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool-lesson links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool-lesson issues are suppressed by `note_id`; duplicate tool-lesson notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the tool-lesson issue with the `gitclaw` label; tool execution, tool installation, and tool policy changes remain explicit reviewed follow-ups\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolLessonIssueBody(opts ChannelToolLessonOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-tool-lesson note_id=\"%s\" channel=\"%s\" tool_name_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.NoteID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ToolName), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel tool lesson.\n\n")
	fmt.Fprintf(&b, "- note_id: %s\n", opts.NoteID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- tool_lesson_mode: github-issue-tool-lesson\n")
	fmt.Fprintf(&b, "- tool_execution_performed: false\n")
	fmt.Fprintf(&b, "- tool_install_performed: false\n")
	fmt.Fprintf(&b, "- tool_policy_mutation_performed: false\n")
	fmt.Fprintf(&b, "- memory_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Tool\n\n")
	b.WriteString(strings.TrimSpace(opts.ToolName))
	b.WriteString("\n\n## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Lesson) != "" {
		b.WriteString("\n\n## Lesson\n\n")
		b.WriteString(strings.TrimSpace(opts.Lesson))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the channel tool lesson without executing tools, installing tools, mutating tool policy, mutating memory, or changing repository files automatically.")
	return strings.TrimSpace(b.String())
}

func channelToolLessonActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolLessonActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolLessonIssueTarget(ev Event, req *ChannelToolLessonActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool lesson requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelToolLessonBody(trailing string, ev Event) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel tool lesson from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return "", defaultTitle, ""
	}
	var toolName string
	var title string
	var lessonLines []string
	mode := ""
	for _, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "tool:"):
			if toolName == "" {
				toolName = strings.TrimSpace(trimmed[len("tool:"):])
			}
			mode = ""
		case strings.HasPrefix(lower, "tool name:"):
			if toolName == "" {
				toolName = strings.TrimSpace(trimmed[len("tool name:"):])
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
	lesson := stripChannelToolLessonLessonHeader(strings.TrimSpace(strings.Join(lessonLines, "\n")))
	return toolName, title, lesson
}

func stripChannelToolLessonLessonHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"lesson:", "note:", "context:", "notes:", "source:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

func normalizeChannelToolLessonOptions(opts ChannelToolLessonOptions) ChannelToolLessonOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.NoteID = cleanChannelToolLessonID(opts.NoteID)
	opts.ToolName = cleanToolLookupName(opts.ToolName)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Lesson = strings.TrimSpace(opts.Lesson)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelToolLessonRoute(cfg Config, opts ChannelToolLessonOptions) (ChannelToolLessonOptions, error) {
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

func validateChannelToolLessonOptions(opts ChannelToolLessonOptions) error {
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
		return fmt.Errorf("missing tool lesson id")
	}
	if opts.ToolName == "" {
		return fmt.Errorf("missing tool name")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing tool lesson source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing tool lesson title")
	}
	return nil
}

func validateChannelToolLessonActionRequestOptions(opts ChannelToolLessonOptions) error {
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
		return fmt.Errorf("missing tool lesson id")
	}
	if opts.ToolName == "" {
		return fmt.Errorf("missing tool name")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing tool lesson source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing tool lesson title")
	}
	return nil
}

func findOrCreateChannelToolLessonIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolLessonOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel tool lesson issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelToolLessonMatches(issue.Body, opts.NoteID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelToolLessonIssueTitle(opts), RenderChannelToolLessonIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel tool lesson issue: %w", err)
	}
	return issue, true, false, nil
}

func channelToolLessonIssueTitle(opts ChannelToolLessonOptions) string {
	toolName := strings.TrimSpace(opts.ToolName)
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	switch {
	case toolName != "" && title != "":
		title = toolName + ": " + title
	case toolName != "":
		title = toolName
	case title == "":
		title = opts.NoteID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel tool lesson: " + title
}

func channelToolLessonMatches(body, noteID string) bool {
	return HasChannelToolLessonMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`note_id="%s"`, escapeMarkerValue(cleanChannelToolLessonID(noteID))))
}

func cleanChannelToolLessonID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelToolLessonID(ev Event, channel, threadID, sourceMessageID, toolName, title, lesson string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, toolName, title, lesson}, "|")
	return fmt.Sprintf("tool-lesson-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolLessonNotifyMessageID(ev Event, noteID string) string {
	seed := strings.Join([]string{eventID(ev), noteID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-lesson-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelToolLessonNotificationBody(opts ChannelToolLessonOptions, noteIssueNumber int, noteIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool lesson captured.\n\n")
	if noteIssueNumber > 0 {
		fmt.Fprintf(&b, "Tool lesson: #%d\n", noteIssueNumber)
	}
	if noteIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", noteIssueURL)
	}
	fmt.Fprintf(&b, "Tool: %s\n", strings.TrimSpace(opts.ToolName))
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue reviewing it in the linked GitHub issue. Tool execution, installation, and policy updates remain separate reviewed steps.")
	return strings.TrimSpace(b.String())
}
