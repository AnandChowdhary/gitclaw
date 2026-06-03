package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSnippetOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	SnippetID         string
	Title             string
	Language          string
	Snippet           string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSnippetResult struct {
	SnippetIssueNumber int
	SnippetIssueURL    string
	SnippetCreated     bool
	SnippetDuplicate   bool
	Notification       ChannelSendResult
	RouteName          string
	RouteHash          string
	Channel            string
	ThreadHash         string
	MessageHash        string
	NotifyHash         string
}

type ChannelSnippetActionRequest struct {
	Options             ChannelSnippetOptions
	Command             string
	Subcommand          string
	AutoSnippetID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	LanguageSHA         string
	SnippetSHA          string
	SnippetBytes        int
	SnippetLines        int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelSnippetActionRequest(ev Event, cfg Config) bool {
	return isChannelSnippetActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSnippetActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "snippet", "code", "paste", "code-block", "code-snippet", "config-snippet", "fragment":
		return true
	default:
		return false
	}
}

func BuildChannelSnippetActionRequest(ev Event, cfg Config) (ChannelSnippetActionRequest, error) {
	fields, trailing, ok := channelSnippetActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSnippetActionRequest{}, fmt.Errorf("missing channel snippet command")
	}
	req := ChannelSnippetActionRequest{
		Options: ChannelSnippetOptions{
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
				return ChannelSnippetActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--snippet-id", "--code-id", "--paste-id", "--fragment-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SnippetID = cleanChannelSnippetID(fields[i+1])
			i++
		case "--language", "--lang", "-l":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Language = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSnippetActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSnippetActionRequest{}, fmt.Errorf("unknown channel snippet argument %q", field)
			}
			if req.Options.SnippetID == "" {
				req.Options.SnippetID = cleanChannelSnippetID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelSnippetActionRequest{}, fmt.Errorf("unexpected channel snippet argument %q", field)
		}
	}
	if err := applyChannelSnippetIssueTarget(ev, &req); err != nil {
		return ChannelSnippetActionRequest{}, err
	}
	title, language, snippet, notes := parseChannelSnippetSections(trailing, ev)
	req.Options.Title = title
	if strings.TrimSpace(req.Options.Language) == "" {
		req.Options.Language = language
	}
	req.Options.Snippet = snippet
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.SnippetID) == "" {
		req.Options.SnippetID = autoChannelSnippetID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, req.Options.Language, snippet, notes)
		req.AutoSnippetID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSnippetNotifyMessageID(ev, req.Options.SnippetID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSnippetOptions(req.Options)
	if err := validateChannelSnippetActionRequestOptions(req.Options); err != nil {
		return ChannelSnippetActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.LanguageSHA = shortDocumentHash(req.Options.Language)
	req.SnippetSHA = shortDocumentHash(req.Options.Snippet)
	req.SnippetBytes = len(req.Options.Snippet)
	req.SnippetLines = lineCount(req.Options.Snippet)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelSnippetNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelSnippet(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSnippetOptions) (ChannelSnippetResult, error) {
	opts = normalizeChannelSnippetOptions(opts)
	var err error
	opts, err = applyChannelSnippetRoute(cfg, opts)
	if err != nil {
		return ChannelSnippetResult{}, err
	}
	if err := validateChannelSnippetOptions(opts); err != nil {
		return ChannelSnippetResult{}, err
	}
	snippetIssue, created, duplicate, err := findOrCreateChannelSnippetIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelSnippetResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelSnippetNotificationBody(opts, snippetIssue.Number, issueURL(opts.Repo, snippetIssue.Number)),
	})
	if err != nil {
		return ChannelSnippetResult{}, fmt.Errorf("queue channel snippet notification: %w", err)
	}
	return ChannelSnippetResult{
		SnippetIssueNumber: snippetIssue.Number,
		SnippetIssueURL:    issueURL(opts.Repo, snippetIssue.Number),
		SnippetCreated:     created,
		SnippetDuplicate:   duplicate,
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelSnippetActionReport(ev Event, req ChannelSnippetActionRequest, result ChannelSnippetResult) string {
	status := "saved"
	switch {
	case result.SnippetDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.SnippetDuplicate:
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
	b.WriteString("## GitClaw Channel Snippet Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_snippet_status: `%s`\n", status)
	fmt.Fprintf(&b, "- snippet_issue: `#%d`\n", result.SnippetIssueNumber)
	fmt.Fprintf(&b, "- snippet_issue_url: `%s`\n", result.SnippetIssueURL)
	fmt.Fprintf(&b, "- snippet_issue_created: `%t`\n", result.SnippetCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.SnippetDuplicate)
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
	fmt.Fprintf(&b, "- snippet_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.SnippetID))
	fmt.Fprintf(&b, "- snippet_id_auto: `%t`\n", req.AutoSnippetID)
	fmt.Fprintf(&b, "- snippet_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- snippet_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- snippet_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- snippet_language_sha256_12: `%s`\n", req.LanguageSHA)
	fmt.Fprintf(&b, "- snippet_body_sha256_12: `%s`\n", req.SnippetSHA)
	fmt.Fprintf(&b, "- snippet_body_bytes: `%d`\n", req.SnippetBytes)
	fmt.Fprintf(&b, "- snippet_body_lines: `%d`\n", req.SnippetLines)
	fmt.Fprintf(&b, "- snippet_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- snippet_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- snippet_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_snippet_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_snippet_language_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_snippet_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_snippet_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_snippet_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_snippet_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw saved an explicitly provided channel code/config snippet as a durable GitHub issue, then queued a provider-facing snippet link back to the mirrored thread. The snippet issue contains the readable code block; this source receipt keeps provider IDs, snippet IDs, language names, code bodies, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the snippet-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent snippet links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate snippet issues are suppressed by `snippet_id`; duplicate snippet-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the snippet issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSnippetIssueBody(opts ChannelSnippetOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-snippet snippet_id=\"%s\" channel=\"%s\" language_sha256_12=\"%s\" snippet_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.SnippetID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Language), shortDocumentHash(opts.Snippet), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel snippet.\n\n")
	fmt.Fprintf(&b, "- snippet_id: %s\n", opts.SnippetID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- language: %s\n", noneIfEmpty(opts.Language))
	fmt.Fprintf(&b, "- snippet_sha256_12: %s\n", shortDocumentHash(opts.Snippet))
	fmt.Fprintf(&b, "- snippet_mode: github-issue-snippet\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	b.WriteString("\n\n## Snippet\n\n")
	fence := channelSnippetFence(opts.Snippet)
	fmt.Fprintf(&b, "%s%s\n%s\n%s", fence, cleanChannelSnippetFenceLanguage(opts.Language), strings.TrimSpace(opts.Snippet), fence)
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel-origin code or config snippet. Any review, refactor, tool request, task conversion, or memory promotion should happen through normal GitHub conversation.")
	return strings.TrimSpace(b.String())
}

func channelSnippetActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSnippetActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSnippetIssueTarget(ev Event, req *ChannelSnippetActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel snippet requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelSnippetSections(trailing string, ev Event) (string, string, string, string) {
	defaultTitle := fmt.Sprintf("Channel snippet from issue #%d", ev.Issue.Number)
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	sections := map[string][]string{}
	current := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		if strings.TrimSpace(line) == "" && current == "" {
			continue
		}
		if section, value, ok := parseChannelSnippetSectionHeader(line); ok {
			current = section
			if value != "" {
				sections[current] = append(sections[current], value)
			}
			continue
		}
		if current == "" {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				current = "snippet"
				sections[current] = append(sections[current], line)
				continue
			}
			current = "title"
			sections[current] = append(sections[current], line)
			continue
		}
		sections[current] = append(sections[current], line)
	}
	title := strings.TrimSpace(strings.Join(sections["title"], "\n"))
	if title == "" {
		title = defaultTitle
	}
	language := strings.TrimSpace(strings.Join(sections["language"], "\n"))
	snippet := strings.TrimSpace(strings.Join(sections["snippet"], "\n"))
	if inferred := inferChannelSnippetFenceLanguage(snippet); language == "" && inferred != "" {
		language = inferred
	}
	snippet = stripChannelSnippetFence(snippet)
	notes := strings.TrimSpace(strings.Join(sections["notes"], "\n"))
	return title, language, snippet, notes
}

func parseChannelSnippetSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelSnippetSectionName(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "language":
		return "language", strings.TrimSpace(value), true
	case "snippet":
		return "snippet", strings.TrimSpace(value), true
	case "notes":
		return "notes", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelSnippetSectionName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "title", "summary", "name":
		return "title"
	case "language", "lang", "syntax", "type":
		return "language"
	case "snippet", "code", "block", "paste", "fragment", "config":
		return "snippet"
	case "notes", "context", "why", "description":
		return "notes"
	default:
		return ""
	}
}

func inferChannelSnippetFenceLanguage(snippet string) string {
	lines := strings.Split(strings.TrimSpace(snippet), "\n")
	if len(lines) == 0 {
		return ""
	}
	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, "```") {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(first, "```")))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func stripChannelSnippetFence(snippet string) string {
	lines := strings.Split(strings.TrimSpace(snippet), "\n")
	if len(lines) < 2 {
		return strings.TrimSpace(snippet)
	}
	first := strings.TrimSpace(lines[0])
	last := strings.TrimSpace(lines[len(lines)-1])
	if strings.HasPrefix(first, "```") && strings.HasPrefix(last, "```") {
		return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
	}
	return strings.TrimSpace(snippet)
}

func normalizeChannelSnippetOptions(opts ChannelSnippetOptions) ChannelSnippetOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SnippetID = cleanChannelSnippetID(opts.SnippetID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Language = cleanChannelSnippetLanguage(opts.Language)
	opts.Snippet = strings.TrimSpace(opts.Snippet)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSnippetRoute(cfg Config, opts ChannelSnippetOptions) (ChannelSnippetOptions, error) {
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

func validateChannelSnippetOptions(opts ChannelSnippetOptions) error {
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
	if opts.SnippetID == "" {
		return fmt.Errorf("missing snippet id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing snippet source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing snippet title")
	}
	if opts.Snippet == "" {
		return fmt.Errorf("missing snippet body")
	}
	return nil
}

func validateChannelSnippetActionRequestOptions(opts ChannelSnippetOptions) error {
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
	if opts.SnippetID == "" {
		return fmt.Errorf("missing snippet id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing snippet source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing snippet title")
	}
	if opts.Snippet == "" {
		return fmt.Errorf("missing snippet body")
	}
	return nil
}

func findOrCreateChannelSnippetIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSnippetOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel snippet issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelSnippetMatches(issue.Body, opts.SnippetID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelSnippetIssueTitle(opts), RenderChannelSnippetIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel snippet issue: %w", err)
	}
	return issue, true, false, nil
}

func channelSnippetIssueTitle(opts ChannelSnippetOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.SnippetID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel snippet: " + title
}

func channelSnippetMatches(body, snippetID string) bool {
	return HasChannelSnippetMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`snippet_id="%s"`, escapeMarkerValue(cleanChannelSnippetID(snippetID))))
}

func cleanChannelSnippetID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSnippetLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, "`")
	value = strings.ReplaceAll(value, " ", "-")
	return cleanChannelHuddleID(value)
}

func cleanChannelSnippetFenceLanguage(value string) string {
	value = cleanChannelSnippetLanguage(value)
	if value == "" {
		return ""
	}
	return value
}

func autoChannelSnippetID(ev Event, channel, threadID, sourceMessageID, title, language, snippet, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, language, snippet, notes}, "|")
	return fmt.Sprintf("snippet-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSnippetNotifyMessageID(ev Event, snippetID string) string {
	seed := strings.Join([]string{eventID(ev), snippetID}, "|")
	return fmt.Sprintf("gitclaw-channel-snippet-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelSnippetNotificationBody(opts ChannelSnippetOptions, snippetIssueNumber int, snippetIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel snippet saved.\n\n")
	if snippetIssueNumber > 0 {
		fmt.Fprintf(&b, "Snippet: #%d\n", snippetIssueNumber)
	}
	if snippetIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", snippetIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if opts.Language != "" {
		fmt.Fprintf(&b, "Language: %s\n", opts.Language)
	}
	b.WriteString("\nSaved in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}

func channelSnippetFence(snippet string) string {
	size := 3
	for strings.Contains(snippet, strings.Repeat("`", size)) {
		size++
	}
	return strings.Repeat("`", size)
}
