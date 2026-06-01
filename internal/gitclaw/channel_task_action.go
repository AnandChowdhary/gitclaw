package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelTaskOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	TaskID            string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelTaskResult struct {
	TaskIssueNumber int
	TaskIssueURL    string
	TaskCreated     bool
	TaskDuplicate   bool
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
}

type ChannelTaskActionRequest struct {
	Options             ChannelTaskOptions
	Command             string
	Subcommand          string
	AutoTaskID          bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelTaskActionRequest(ev Event, cfg Config) bool {
	return isChannelTaskActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelTaskActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "task", "todo", "ticket", "issue":
		return true
	default:
		return false
	}
}

func BuildChannelTaskActionRequest(ev Event, cfg Config) (ChannelTaskActionRequest, error) {
	fields, trailing, ok := channelTaskActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelTaskActionRequest{}, fmt.Errorf("missing channel task command")
	}
	req := ChannelTaskActionRequest{
		Options: ChannelTaskOptions{
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
				return ChannelTaskActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelTaskActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelTaskActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelTaskActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelTaskActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--task-id", "--ticket-id", "--issue-id", "--id":
			if i+1 >= len(fields) {
				return ChannelTaskActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TaskID = cleanChannelTaskID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelTaskActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelTaskActionRequest{}, fmt.Errorf("unknown channel task argument %q", field)
			}
			if req.Options.TaskID == "" {
				req.Options.TaskID = cleanChannelTaskID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelTaskActionRequest{}, fmt.Errorf("unexpected channel task argument %q", field)
		}
	}
	if err := applyChannelTaskIssueTarget(ev, &req); err != nil {
		return ChannelTaskActionRequest{}, err
	}
	title, notes := parseChannelTaskTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.TaskID) == "" {
		req.Options.TaskID = autoChannelTaskID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoTaskID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelTaskNotifyMessageID(ev, req.Options.TaskID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelTaskOptions(req.Options)
	if err := validateChannelTaskActionRequestOptions(req.Options); err != nil {
		return ChannelTaskActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelTaskNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelTask(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelTaskOptions) (ChannelTaskResult, error) {
	opts = normalizeChannelTaskOptions(opts)
	var err error
	opts, err = applyChannelTaskRoute(cfg, opts)
	if err != nil {
		return ChannelTaskResult{}, err
	}
	if err := validateChannelTaskOptions(opts); err != nil {
		return ChannelTaskResult{}, err
	}
	taskIssue, created, duplicate, err := findOrCreateChannelTaskIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelTaskResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelTaskNotificationBody(opts, taskIssue.Number, issueURL(opts.Repo, taskIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelTaskResult{}, fmt.Errorf("queue channel task notification: %w", err)
	}
	return ChannelTaskResult{
		TaskIssueNumber: taskIssue.Number,
		TaskIssueURL:    issueURL(opts.Repo, taskIssue.Number),
		TaskCreated:     created,
		TaskDuplicate:   duplicate,
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelTaskActionReport(ev Event, req ChannelTaskActionRequest, result ChannelTaskResult) string {
	status := "created"
	switch {
	case result.TaskDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.TaskDuplicate:
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
	b.WriteString("## GitClaw Channel Task Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_task_status: `%s`\n", status)
	fmt.Fprintf(&b, "- task_issue: `#%d`\n", result.TaskIssueNumber)
	fmt.Fprintf(&b, "- task_issue_url: `%s`\n", result.TaskIssueURL)
	fmt.Fprintf(&b, "- task_issue_created: `%t`\n", result.TaskCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.TaskDuplicate)
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
	fmt.Fprintf(&b, "- task_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.TaskID))
	fmt.Fprintf(&b, "- task_id_auto: `%t`\n", req.AutoTaskID)
	fmt.Fprintf(&b, "- task_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- task_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- task_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- task_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- task_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- task_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_task_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_task_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_task_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_task_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub task issue from a mirrored channel thread, then queued a provider-facing task link back to that thread. The task issue contains the human-readable work item; this source receipt keeps provider IDs, task IDs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the task-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent task links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate task issues are suppressed by `task_id`; duplicate task-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the task issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelTaskIssueBody(opts ChannelTaskOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-task task_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.TaskID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel task.\n\n")
	fmt.Fprintf(&b, "- task_id: %s\n", opts.TaskID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- task_mode: github-issue-task\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Task\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nContinue the work in this GitHub issue. Use labels for task state and comments for handoffs.")
	return strings.TrimSpace(b.String())
}

func channelTaskActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelTaskActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelTaskIssueTarget(ev Event, req *ChannelTaskActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel task requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelTaskTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel task from issue #%d", ev.Issue.Number)
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

func normalizeChannelTaskOptions(opts ChannelTaskOptions) ChannelTaskOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.TaskID = cleanChannelTaskID(opts.TaskID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelTaskRoute(cfg Config, opts ChannelTaskOptions) (ChannelTaskOptions, error) {
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

func validateChannelTaskOptions(opts ChannelTaskOptions) error {
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
	if opts.TaskID == "" {
		return fmt.Errorf("missing task id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing task source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing task title")
	}
	return nil
}

func validateChannelTaskActionRequestOptions(opts ChannelTaskOptions) error {
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
	if opts.TaskID == "" {
		return fmt.Errorf("missing task id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing task source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing task title")
	}
	return nil
}

func findOrCreateChannelTaskIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelTaskOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel task issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelTaskMatches(issue.Body, opts.TaskID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelTaskIssueTitle(opts), RenderChannelTaskIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel task issue: %w", err)
	}
	return issue, true, false, nil
}

func channelTaskIssueTitle(opts ChannelTaskOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.TaskID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel task: " + title
}

func channelTaskMatches(body, taskID string) bool {
	return HasChannelTaskMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`task_id="%s"`, escapeMarkerValue(cleanChannelTaskID(taskID))))
}

func cleanChannelTaskID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelTaskID(ev Event, channel, threadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("task-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelTaskNotifyMessageID(ev Event, taskID string) string {
	seed := strings.Join([]string{eventID(ev), taskID}, "|")
	return fmt.Sprintf("gitclaw-channel-task-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelTaskNotificationBody(opts ChannelTaskOptions, taskIssueNumber int, taskIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel task created.\n\n")
	if taskIssueNumber > 0 {
		fmt.Fprintf(&b, "Task: #%d\n", taskIssueNumber)
	}
	if taskIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", taskIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue the work in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
