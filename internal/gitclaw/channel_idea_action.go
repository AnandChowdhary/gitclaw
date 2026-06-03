package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelIdeaOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	IdeaID            string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelIdeaResult struct {
	IdeaIssueNumber int
	IdeaIssueURL    string
	IdeaCreated     bool
	IdeaDuplicate   bool
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
}

type ChannelIdeaActionRequest struct {
	Options             ChannelIdeaOptions
	Command             string
	Subcommand          string
	AutoIdeaID          bool
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

func IsChannelIdeaActionRequest(ev Event, cfg Config) bool {
	return isChannelIdeaActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelIdeaActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "idea", "ideate", "concept", "pitch":
		return true
	default:
		return false
	}
}

func BuildChannelIdeaActionRequest(ev Event, cfg Config) (ChannelIdeaActionRequest, error) {
	fields, trailing, ok := channelIdeaActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelIdeaActionRequest{}, fmt.Errorf("missing channel idea command")
	}
	req := ChannelIdeaActionRequest{
		Options: ChannelIdeaOptions{
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
				return ChannelIdeaActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelIdeaActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelIdeaActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelIdeaActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelIdeaActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--idea-id", "--brainstorm-id", "--concept-id", "--pitch-id", "--id":
			if i+1 >= len(fields) {
				return ChannelIdeaActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.IdeaID = cleanChannelIdeaID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelIdeaActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelIdeaActionRequest{}, fmt.Errorf("unknown channel idea argument %q", field)
			}
			if req.Options.IdeaID == "" {
				req.Options.IdeaID = cleanChannelIdeaID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelIdeaActionRequest{}, fmt.Errorf("unexpected channel idea argument %q", field)
		}
	}
	if err := applyChannelIdeaIssueTarget(ev, &req); err != nil {
		return ChannelIdeaActionRequest{}, err
	}
	title, notes := parseChannelIdeaTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.IdeaID) == "" {
		req.Options.IdeaID = autoChannelIdeaID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, notes)
		req.AutoIdeaID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelIdeaNotifyMessageID(ev, req.Options.IdeaID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelIdeaOptions(req.Options)
	if err := validateChannelIdeaActionRequestOptions(req.Options); err != nil {
		return ChannelIdeaActionRequest{}, err
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelIdeaNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelIdea(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelIdeaOptions) (ChannelIdeaResult, error) {
	opts = normalizeChannelIdeaOptions(opts)
	var err error
	opts, err = applyChannelIdeaRoute(cfg, opts)
	if err != nil {
		return ChannelIdeaResult{}, err
	}
	if err := validateChannelIdeaOptions(opts); err != nil {
		return ChannelIdeaResult{}, err
	}
	ideaIssue, created, duplicate, err := findOrCreateChannelIdeaIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelIdeaResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelIdeaNotificationBody(opts, ideaIssue.Number, issueURL(opts.Repo, ideaIssue.Number)),
	})
	if err != nil {
		return ChannelIdeaResult{}, fmt.Errorf("queue channel idea notification: %w", err)
	}
	return ChannelIdeaResult{
		IdeaIssueNumber: ideaIssue.Number,
		IdeaIssueURL:    issueURL(opts.Repo, ideaIssue.Number),
		IdeaCreated:     created,
		IdeaDuplicate:   duplicate,
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelIdeaActionReport(ev Event, req ChannelIdeaActionRequest, result ChannelIdeaResult) string {
	status := "captured"
	switch {
	case result.IdeaDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.IdeaDuplicate:
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
	b.WriteString("## GitClaw Channel Idea Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_idea_status: `%s`\n", status)
	fmt.Fprintf(&b, "- idea_issue: `#%d`\n", result.IdeaIssueNumber)
	fmt.Fprintf(&b, "- idea_issue_url: `%s`\n", result.IdeaIssueURL)
	fmt.Fprintf(&b, "- idea_issue_created: `%t`\n", result.IdeaCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.IdeaDuplicate)
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
	fmt.Fprintf(&b, "- idea_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.IdeaID))
	fmt.Fprintf(&b, "- idea_id_auto: `%t`\n", req.AutoIdeaID)
	fmt.Fprintf(&b, "- idea_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- idea_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- idea_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- idea_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- idea_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- idea_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_idea_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_idea_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_idea_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_idea_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin idea as a durable GitHub issue, then queued a provider-facing idea link back to the mirrored thread. The idea issue contains the human-readable title and notes; this source receipt keeps provider IDs, idea IDs, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the idea-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent idea links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate idea issues are suppressed by `idea_id`; duplicate idea-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the idea issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelIdeaIssueBody(opts ChannelIdeaOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-idea idea_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.IdeaID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel idea.\n\n")
	fmt.Fprintf(&b, "- idea_id: %s\n", opts.IdeaID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- idea_mode: github-issue-idea\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Idea\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for shaping the channel idea into a task, skill, memory, tool request, or proactive workflow.")
	return strings.TrimSpace(b.String())
}

func channelIdeaActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelIdeaActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelIdeaIssueTarget(ev Event, req *ChannelIdeaActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel idea requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelIdeaTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel idea from issue #%d", ev.Issue.Number)
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
	case strings.HasPrefix(lowerFirst, "idea:"):
		title = strings.TrimSpace(first[len("idea:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "pitch:"):
		title = strings.TrimSpace(first[len("pitch:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"), strings.HasPrefix(lowerFirst, "why:"):
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
	case strings.HasPrefix(notesLower, "why:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("why:"):])
	}
	return title, notes
}

func normalizeChannelIdeaOptions(opts ChannelIdeaOptions) ChannelIdeaOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.IdeaID = cleanChannelIdeaID(opts.IdeaID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelIdeaRoute(cfg Config, opts ChannelIdeaOptions) (ChannelIdeaOptions, error) {
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

func validateChannelIdeaOptions(opts ChannelIdeaOptions) error {
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
	if opts.IdeaID == "" {
		return fmt.Errorf("missing idea id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing idea source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing idea title")
	}
	return nil
}

func validateChannelIdeaActionRequestOptions(opts ChannelIdeaOptions) error {
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
	if opts.IdeaID == "" {
		return fmt.Errorf("missing idea id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing idea source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing idea title")
	}
	return nil
}

func findOrCreateChannelIdeaIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelIdeaOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel idea issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelIdeaMatches(issue.Body, opts.IdeaID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelIdeaIssueTitle(opts), RenderChannelIdeaIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel idea issue: %w", err)
	}
	return issue, true, false, nil
}

func channelIdeaIssueTitle(opts ChannelIdeaOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.IdeaID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel idea: " + title
}

func channelIdeaMatches(body, ideaID string) bool {
	return HasChannelIdeaMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`idea_id="%s"`, escapeMarkerValue(cleanChannelIdeaID(ideaID))))
}

func cleanChannelIdeaID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelIdeaID(ev Event, channel, threadID, sourceMessageID, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, notes}, "|")
	return fmt.Sprintf("idea-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelIdeaNotifyMessageID(ev Event, ideaID string) string {
	seed := strings.Join([]string{eventID(ev), ideaID}, "|")
	return fmt.Sprintf("gitclaw-channel-idea-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelIdeaNotificationBody(opts ChannelIdeaOptions, ideaIssueNumber int, ideaIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel idea captured.\n\n")
	if ideaIssueNumber > 0 {
		fmt.Fprintf(&b, "Idea: #%d\n", ideaIssueNumber)
	}
	if ideaIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", ideaIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue shaping it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
