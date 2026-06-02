package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelIncidentOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	IncidentID        string
	Severity          string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelIncidentResult struct {
	IncidentIssueNumber int
	IncidentIssueURL    string
	IncidentCreated     bool
	IncidentDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelIncidentActionRequest struct {
	Options             ChannelIncidentOptions
	Command             string
	Subcommand          string
	AutoIncidentID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	SeveritySHA         string
	SeverityBytes       int
	SeverityLines       int
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

func IsChannelIncidentActionRequest(ev Event, cfg Config) bool {
	return isChannelIncidentActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelIncidentActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "incident", "escalate", "escalation", "outage", "page", "alert", "sev", "triage-incident":
		return true
	default:
		return false
	}
}

func BuildChannelIncidentActionRequest(ev Event, cfg Config) (ChannelIncidentActionRequest, error) {
	fields, trailing, ok := channelIncidentActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelIncidentActionRequest{}, fmt.Errorf("missing channel incident command")
	}
	req := ChannelIncidentActionRequest{
		Options: ChannelIncidentOptions{
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
				return ChannelIncidentActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--incident-id", "--escalation-id", "--outage-id", "--alert-id", "--id":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.IncidentID = cleanChannelIncidentID(fields[i+1])
			i++
		case "--severity", "--sev", "--priority":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Severity = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelIncidentActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelIncidentActionRequest{}, fmt.Errorf("unknown channel incident argument %q", field)
			}
			if req.Options.IncidentID == "" {
				req.Options.IncidentID = cleanChannelIncidentID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelIncidentActionRequest{}, fmt.Errorf("unexpected channel incident argument %q", field)
		}
	}
	if err := applyChannelIncidentIssueTarget(ev, &req); err != nil {
		return ChannelIncidentActionRequest{}, err
	}
	title, notes := parseChannelIncidentTitleNotes(trailing, ev)
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.IncidentID) == "" {
		req.Options.IncidentID = autoChannelIncidentID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Severity, title, notes)
		req.AutoIncidentID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelIncidentNotifyMessageID(ev, req.Options.IncidentID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelIncidentOptions(req.Options)
	if err := validateChannelIncidentActionRequestOptions(req.Options); err != nil {
		return ChannelIncidentActionRequest{}, err
	}
	req.SeveritySHA = shortDocumentHash(req.Options.Severity)
	req.SeverityBytes = len(req.Options.Severity)
	req.SeverityLines = lineCount(req.Options.Severity)
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelIncidentNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelIncident(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelIncidentOptions) (ChannelIncidentResult, error) {
	opts = normalizeChannelIncidentOptions(opts)
	var err error
	opts, err = applyChannelIncidentRoute(cfg, opts)
	if err != nil {
		return ChannelIncidentResult{}, err
	}
	if err := validateChannelIncidentOptions(opts); err != nil {
		return ChannelIncidentResult{}, err
	}
	incidentIssue, created, duplicate, err := findOrCreateChannelIncidentIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelIncidentResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelIncidentNotificationBody(opts, incidentIssue.Number, issueURL(opts.Repo, incidentIssue.Number)),
	})
	if err != nil {
		return ChannelIncidentResult{}, fmt.Errorf("queue channel incident notification: %w", err)
	}
	return ChannelIncidentResult{
		IncidentIssueNumber: incidentIssue.Number,
		IncidentIssueURL:    issueURL(opts.Repo, incidentIssue.Number),
		IncidentCreated:     created,
		IncidentDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelIncidentActionReport(ev Event, req ChannelIncidentActionRequest, result ChannelIncidentResult) string {
	status := "captured"
	switch {
	case result.IncidentDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.IncidentDuplicate:
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
	b.WriteString("## GitClaw Channel Incident Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_incident_status: `%s`\n", status)
	fmt.Fprintf(&b, "- incident_issue: `#%d`\n", result.IncidentIssueNumber)
	fmt.Fprintf(&b, "- incident_issue_url: `%s`\n", result.IncidentIssueURL)
	fmt.Fprintf(&b, "- incident_issue_created: `%t`\n", result.IncidentCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.IncidentDuplicate)
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
	fmt.Fprintf(&b, "- incident_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.IncidentID))
	fmt.Fprintf(&b, "- incident_id_auto: `%t`\n", req.AutoIncidentID)
	fmt.Fprintf(&b, "- incident_severity_sha256_12: `%s`\n", req.SeveritySHA)
	fmt.Fprintf(&b, "- incident_severity_bytes: `%d`\n", req.SeverityBytes)
	fmt.Fprintf(&b, "- incident_severity_lines: `%d`\n", req.SeverityLines)
	fmt.Fprintf(&b, "- incident_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- incident_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- incident_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- incident_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- incident_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- incident_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_incident_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_incident_severity_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_incident_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_incident_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_incident_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin incident as a durable GitHub issue, then queued a provider-facing incident link back to the mirrored thread. The incident issue contains the human-readable severity, title, and notes; this source receipt keeps provider IDs, incident IDs, severity, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the incident-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent incident links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate incident issues are suppressed by `incident_id`; duplicate incident-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the incident issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelIncidentIssueBody(opts ChannelIncidentOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-incident incident_id=\"%s\" channel=\"%s\" severity_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.IncidentID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Severity), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel incident.\n\n")
	fmt.Fprintf(&b, "- incident_id: %s\n", opts.IncidentID)
	fmt.Fprintf(&b, "- severity: %s\n", opts.Severity)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- incident_mode: github-issue-incident\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Incident\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for shaping the channel incident into a task, skill, memory, tool request, or proactive workflow.")
	return strings.TrimSpace(b.String())
}

func channelIncidentActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelIncidentActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelIncidentIssueTarget(ev Event, req *ChannelIncidentActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel incident requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelIncidentTitleNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel incident from issue #%d", ev.Issue.Number)
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
	case strings.HasPrefix(lowerFirst, "incident:"):
		title = strings.TrimSpace(first[len("incident:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "alert:"):
		title = strings.TrimSpace(first[len("alert:"):])
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

func normalizeChannelIncidentOptions(opts ChannelIncidentOptions) ChannelIncidentOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.IncidentID = cleanChannelIncidentID(opts.IncidentID)
	opts.Severity = cleanChannelIncidentSeverity(opts.Severity)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelIncidentRoute(cfg Config, opts ChannelIncidentOptions) (ChannelIncidentOptions, error) {
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

func validateChannelIncidentOptions(opts ChannelIncidentOptions) error {
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
	if opts.IncidentID == "" {
		return fmt.Errorf("missing incident id")
	}
	if opts.Severity == "" {
		return fmt.Errorf("missing incident severity")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing incident source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing incident title")
	}
	return nil
}

func validateChannelIncidentActionRequestOptions(opts ChannelIncidentOptions) error {
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
	if opts.IncidentID == "" {
		return fmt.Errorf("missing incident id")
	}
	if opts.Severity == "" {
		return fmt.Errorf("missing incident severity")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing incident source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing incident title")
	}
	return nil
}

func findOrCreateChannelIncidentIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelIncidentOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel incident issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelIncidentMatches(issue.Body, opts.IncidentID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelIncidentIssueTitle(opts), RenderChannelIncidentIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel incident issue: %w", err)
	}
	return issue, true, false, nil
}

func channelIncidentIssueTitle(opts ChannelIncidentOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.IncidentID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel incident: " + title
}

func channelIncidentMatches(body, incidentID string) bool {
	return HasChannelIncidentMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`incident_id="%s"`, escapeMarkerValue(cleanChannelIncidentID(incidentID))))
}

func cleanChannelIncidentID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelIncidentSeverity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "untriaged"
	}
	value = strings.Trim(value, " \t\r\n.,:;!?")
	value = strings.ReplaceAll(value, "_", "-")
	return cleanChannelHuddleID(value)
}

func autoChannelIncidentID(ev Event, channel, threadID, sourceMessageID, severity, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, severity, title, notes}, "|")
	return fmt.Sprintf("incident-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelIncidentNotifyMessageID(ev Event, incidentID string) string {
	seed := strings.Join([]string{eventID(ev), incidentID}, "|")
	return fmt.Sprintf("gitclaw-channel-incident-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelIncidentNotificationBody(opts ChannelIncidentOptions, incidentIssueNumber int, incidentIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel incident captured.\n\n")
	if incidentIssueNumber > 0 {
		fmt.Fprintf(&b, "Incident: #%d\n", incidentIssueNumber)
	}
	if incidentIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", incidentIssueURL)
	}
	fmt.Fprintf(&b, "Severity: %s\n", strings.TrimSpace(opts.Severity))
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue triage in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
