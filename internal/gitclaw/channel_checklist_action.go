package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelChecklistOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ChecklistID       string
	Title             string
	Items             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelChecklistResult struct {
	ChecklistIssueNumber int
	ChecklistIssueURL    string
	ChecklistCreated     bool
	ChecklistDuplicate   bool
	Notification         ChannelSendResult
	RouteName            string
	RouteHash            string
	Channel              string
	ThreadHash           string
	MessageHash          string
	NotifyHash           string
}

type ChannelChecklistActionRequest struct {
	Options             ChannelChecklistOptions
	Command             string
	Subcommand          string
	AutoChecklistID     bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	ItemCount           int
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ItemsSHA            string
	ItemsBytes          int
	ItemsLines          int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelChecklistActionRequest(ev Event, cfg Config) bool {
	return isChannelChecklistActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelChecklistActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "checklist", "check-list", "todo-list", "todos", "todo", "punch-list", "action-list":
		return true
	default:
		return false
	}
}

func BuildChannelChecklistActionRequest(ev Event, cfg Config) (ChannelChecklistActionRequest, error) {
	fields, trailing, ok := channelChecklistActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelChecklistActionRequest{}, fmt.Errorf("missing channel checklist command")
	}
	req := ChannelChecklistActionRequest{
		Options: ChannelChecklistOptions{
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
				return ChannelChecklistActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelChecklistActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelChecklistActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelChecklistActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelChecklistActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--checklist-id", "--list-id", "--id":
			if i+1 >= len(fields) {
				return ChannelChecklistActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ChecklistID = cleanChannelChecklistID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelChecklistActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelChecklistActionRequest{}, fmt.Errorf("unknown channel checklist argument %q", field)
			}
			if req.Options.ChecklistID == "" {
				req.Options.ChecklistID = cleanChannelChecklistID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelChecklistActionRequest{}, fmt.Errorf("unexpected channel checklist argument %q", field)
		}
	}
	if err := applyChannelChecklistIssueTarget(ev, &req); err != nil {
		return ChannelChecklistActionRequest{}, err
	}
	title, items, notes := parseChannelChecklistSections(trailing, ev)
	req.Options.Title = title
	req.Options.Items = items
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.ChecklistID) == "" {
		req.Options.ChecklistID = autoChannelChecklistID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, items, notes)
		req.AutoChecklistID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelChecklistNotifyMessageID(ev, req.Options.ChecklistID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelChecklistOptions(req.Options)
	if err := validateChannelChecklistActionRequestOptions(req.Options); err != nil {
		return ChannelChecklistActionRequest{}, err
	}
	req.ItemCount = len(channelChecklistItemLines(req.Options.Items))
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ItemsSHA = shortDocumentHash(req.Options.Items)
	req.ItemsBytes = len(req.Options.Items)
	req.ItemsLines = lineCount(req.Options.Items)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelChecklistNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelChecklist(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelChecklistOptions) (ChannelChecklistResult, error) {
	opts = normalizeChannelChecklistOptions(opts)
	var err error
	opts, err = applyChannelChecklistRoute(cfg, opts)
	if err != nil {
		return ChannelChecklistResult{}, err
	}
	if err := validateChannelChecklistOptions(opts); err != nil {
		return ChannelChecklistResult{}, err
	}
	checklistIssue, created, duplicate, err := findOrCreateChannelChecklistIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelChecklistResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelChecklistNotificationBody(opts, checklistIssue.Number, issueURL(opts.Repo, checklistIssue.Number)),
	})
	if err != nil {
		return ChannelChecklistResult{}, fmt.Errorf("queue channel checklist notification: %w", err)
	}
	return ChannelChecklistResult{
		ChecklistIssueNumber: checklistIssue.Number,
		ChecklistIssueURL:    issueURL(opts.Repo, checklistIssue.Number),
		ChecklistCreated:     created,
		ChecklistDuplicate:   duplicate,
		Notification:         notification,
		RouteName:            opts.Route,
		RouteHash:            channelRouteHash(opts.Route),
		Channel:              opts.Channel,
		ThreadHash:           shortDocumentHash(opts.ThreadID),
		MessageHash:          shortDocumentHash(opts.SourceMessageID),
		NotifyHash:           shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelChecklistActionReport(ev Event, req ChannelChecklistActionRequest, result ChannelChecklistResult) string {
	status := "captured"
	switch {
	case result.ChecklistDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ChecklistDuplicate:
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
	b.WriteString("## GitClaw Channel Checklist Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_checklist_status: `%s`\n", status)
	fmt.Fprintf(&b, "- checklist_issue: `#%d`\n", result.ChecklistIssueNumber)
	fmt.Fprintf(&b, "- checklist_issue_url: `%s`\n", result.ChecklistIssueURL)
	fmt.Fprintf(&b, "- checklist_issue_created: `%t`\n", result.ChecklistCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ChecklistDuplicate)
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
	fmt.Fprintf(&b, "- checklist_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ChecklistID))
	fmt.Fprintf(&b, "- checklist_id_auto: `%t`\n", req.AutoChecklistID)
	fmt.Fprintf(&b, "- checklist_item_count: `%d`\n", req.ItemCount)
	fmt.Fprintf(&b, "- checklist_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- checklist_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- checklist_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- checklist_items_sha256_12: `%s`\n", req.ItemsSHA)
	fmt.Fprintf(&b, "- checklist_items_bytes: `%d`\n", req.ItemsBytes)
	fmt.Fprintf(&b, "- checklist_items_lines: `%d`\n", req.ItemsLines)
	fmt.Fprintf(&b, "- checklist_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- checklist_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- checklist_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- checklist_mode: `%s`\n", "github-issue-checklist")
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_checklist_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_checklist_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_checklist_items_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_checklist_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_checklist_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin checklist as a durable GitHub issue, then queued a provider-facing checklist link back to the mirrored thread. The checklist issue contains the human-readable title, items, and notes; this source receipt keeps provider IDs, checklist IDs, titles, items, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the checklist link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent checklist links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate checklist issues are suppressed by `checklist_id`; duplicate checklist link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the checklist issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelChecklistIssueBody(opts ChannelChecklistOptions) string {
	items := channelChecklistItemLines(opts.Items)
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-checklist checklist_id=\"%s\" channel=\"%s\" items_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ChecklistID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Items), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel checklist.\n\n")
	fmt.Fprintf(&b, "- checklist_id: %s\n", opts.ChecklistID)
	fmt.Fprintf(&b, "- item_count: %d\n", len(items))
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- checklist_mode: github-issue-checklist\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Checklist\n\n")
	for _, item := range items {
		fmt.Fprintf(&b, "- [ ] %s\n", item)
	}
	b.WriteString("\n## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for tracking this channel-created checklist. Any item completion, task conversion, skill work, or proactive follow-up should happen through normal GitHub conversation.")
	return strings.TrimSpace(b.String())
}

func channelChecklistActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelChecklistActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelChecklistIssueTarget(ev Event, req *ChannelChecklistActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel checklist requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelChecklistSections(trailing string, ev Event) (string, string, string) {
	lines := cleanChannelChecklistTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel checklist from issue #%d", ev.Issue.Number)
	sections := channelChecklistParsedSections{Title: defaultTitle}
	if len(lines) == 0 {
		return sections.Title, sections.Items, sections.Notes
	}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				sections.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelChecklistSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				sections.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			sections.Title = trimmed
			continue
		}
		if current == "" {
			current = "items"
		}
		sections.append(current, line)
	}
	return strings.TrimSpace(sections.Title), strings.TrimSpace(sections.Items), strings.TrimSpace(sections.Notes)
}

type channelChecklistParsedSections struct {
	Title string
	Items string
	Notes string
}

func (sections *channelChecklistParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	switch section {
	case "title":
		sections.Title = strings.TrimSpace(value)
	case "items":
		sections.Items = appendChannelChecklistSectionLine(sections.Items, value)
	case "notes":
		sections.Notes = appendChannelChecklistSectionLine(sections.Notes, value)
	}
}

func (sections *channelChecklistParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "items":
		sections.Items = appendChannelChecklistSectionLine(sections.Items, value)
	case "notes":
		sections.Notes = appendChannelChecklistSectionLine(sections.Notes, value)
	}
}

func appendChannelChecklistSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelChecklistSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelChecklistHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "items":
		return "items", strings.TrimSpace(value), true
	case "notes":
		return "notes", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelChecklistHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "name", "summary":
		return "title"
	case "items", "item", "checklist", "steps", "step", "todos", "todo", "actions", "action items", "acceptance", "acceptance criteria":
		return "items"
	case "notes", "context", "details", "description", "why":
		return "notes"
	default:
		return ""
	}
}

func cleanChannelChecklistTrailingLines(trailing string) []string {
	rawLines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned
}

func normalizeChannelChecklistOptions(opts ChannelChecklistOptions) ChannelChecklistOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ChecklistID = cleanChannelChecklistID(opts.ChecklistID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Items = strings.TrimSpace(opts.Items)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelChecklistRoute(cfg Config, opts ChannelChecklistOptions) (ChannelChecklistOptions, error) {
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

func validateChannelChecklistOptions(opts ChannelChecklistOptions) error {
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
	if opts.ChecklistID == "" {
		return fmt.Errorf("missing checklist id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing checklist source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing checklist title")
	}
	if len(channelChecklistItemLines(opts.Items)) == 0 {
		return fmt.Errorf("missing checklist items")
	}
	return nil
}

func validateChannelChecklistActionRequestOptions(opts ChannelChecklistOptions) error {
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
	if opts.ChecklistID == "" {
		return fmt.Errorf("missing checklist id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing checklist source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing checklist title")
	}
	if len(channelChecklistItemLines(opts.Items)) == 0 {
		return fmt.Errorf("missing checklist items")
	}
	return nil
}

func findOrCreateChannelChecklistIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelChecklistOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel checklist issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelChecklistMatches(issue.Body, opts.ChecklistID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelChecklistIssueTitle(opts), RenderChannelChecklistIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel checklist issue: %w", err)
	}
	return issue, true, false, nil
}

func channelChecklistIssueTitle(opts ChannelChecklistOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.ChecklistID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel checklist: " + title
}

func channelChecklistMatches(body, checklistID string) bool {
	return HasChannelChecklistMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`checklist_id="%s"`, escapeMarkerValue(cleanChannelChecklistID(checklistID))))
}

func cleanChannelChecklistID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelChecklistID(ev Event, channel, threadID, sourceMessageID, title, items, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, items, notes}, "|")
	return fmt.Sprintf("checklist-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelChecklistNotifyMessageID(ev Event, checklistID string) string {
	seed := strings.Join([]string{eventID(ev), checklistID}, "|")
	return fmt.Sprintf("gitclaw-channel-checklist-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelChecklistNotificationBody(opts ChannelChecklistOptions, checklistIssueNumber int, checklistIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel checklist captured.\n\n")
	if checklistIssueNumber > 0 {
		fmt.Fprintf(&b, "Checklist: #%d\n", checklistIssueNumber)
	}
	if checklistIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", checklistIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	fmt.Fprintf(&b, "Items: %d\n", len(channelChecklistItemLines(opts.Items)))
	b.WriteString("\nContinue tracking it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}

func channelChecklistItemLines(items string) []string {
	lines := strings.Split(strings.TrimSpace(items), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		item := cleanChannelChecklistItemLine(line)
		if item == "" {
			continue
		}
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func cleanChannelChecklistItemLine(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"- [ ]", "- [x]", "- [X]", "* [ ]", "* [x]", "* [X]", "- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && (line[i] == '.' || line[i] == ')') && line[i+1] == ' ' {
		if _, err := strconv.Atoi(line[:i]); err == nil {
			return strings.TrimSpace(line[i+2:])
		}
	}
	return line
}
