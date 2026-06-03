package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRitualOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RitualID          string
	Title             string
	Cadence           string
	Trigger           string
	Practice          string
	Review            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRitualResult struct {
	RitualIssueNumber int
	RitualIssueURL    string
	RitualCreated     bool
	RitualDuplicate   bool
	Notification      ChannelSendResult
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	MessageHash       string
	NotifyHash        string
}

type ChannelRitualActionRequest struct {
	Options             ChannelRitualOptions
	Command             string
	Subcommand          string
	AutoRitualID        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	CadenceSHA          string
	CadenceBytes        int
	CadenceLines        int
	TriggerSHA          string
	TriggerBytes        int
	TriggerLines        int
	PracticeSHA         string
	PracticeBytes       int
	PracticeLines       int
	ReviewSHA           string
	ReviewBytes         int
	ReviewLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelRitualActionRequest(ev Event, cfg Config) bool {
	return isChannelRitualActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRitualActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "ritual", "practice", "routine", "habit", "cadence":
		return true
	default:
		return false
	}
}

func BuildChannelRitualActionRequest(ev Event, cfg Config) (ChannelRitualActionRequest, error) {
	fields, trailing, ok := channelRitualActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRitualActionRequest{}, fmt.Errorf("missing channel ritual command")
	}
	req := ChannelRitualActionRequest{
		Options: ChannelRitualOptions{
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
				return ChannelRitualActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRitualActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRitualActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelRitualActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRitualActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--ritual-id", "--practice-id", "--routine-id", "--habit-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRitualActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RitualID = cleanChannelRitualID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRitualActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRitualActionRequest{}, fmt.Errorf("unknown channel ritual argument %q", field)
			}
			if req.Options.RitualID == "" {
				req.Options.RitualID = cleanChannelRitualID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelRitualActionRequest{}, fmt.Errorf("unexpected channel ritual argument %q", field)
		}
	}
	if err := applyChannelRitualIssueTarget(ev, &req); err != nil {
		return ChannelRitualActionRequest{}, err
	}
	title, cadence, trigger, practice, review := parseChannelRitualSections(trailing, ev)
	req.Options.Title = title
	req.Options.Cadence = cadence
	req.Options.Trigger = trigger
	req.Options.Practice = practice
	req.Options.Review = review
	if strings.TrimSpace(req.Options.RitualID) == "" {
		req.Options.RitualID = autoChannelRitualID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, cadence, trigger, practice, review)
		req.AutoRitualID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRitualNotifyMessageID(ev, req.Options.RitualID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRitualOptions(req.Options)
	if err := validateChannelRitualActionRequestOptions(req.Options); err != nil {
		return ChannelRitualActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.CadenceSHA = shortDocumentHash(req.Options.Cadence)
	req.CadenceBytes = len(req.Options.Cadence)
	req.CadenceLines = lineCount(req.Options.Cadence)
	req.TriggerSHA = shortDocumentHash(req.Options.Trigger)
	req.TriggerBytes = len(req.Options.Trigger)
	req.TriggerLines = lineCount(req.Options.Trigger)
	req.PracticeSHA = shortDocumentHash(req.Options.Practice)
	req.PracticeBytes = len(req.Options.Practice)
	req.PracticeLines = lineCount(req.Options.Practice)
	req.ReviewSHA = shortDocumentHash(req.Options.Review)
	req.ReviewBytes = len(req.Options.Review)
	req.ReviewLines = lineCount(req.Options.Review)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelRitualNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelRitual(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRitualOptions) (ChannelRitualResult, error) {
	opts = normalizeChannelRitualOptions(opts)
	var err error
	opts, err = applyChannelRitualRoute(cfg, opts)
	if err != nil {
		return ChannelRitualResult{}, err
	}
	if err := validateChannelRitualOptions(opts); err != nil {
		return ChannelRitualResult{}, err
	}
	ritualIssue, created, duplicate, err := findOrCreateChannelRitualIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelRitualResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelRitualNotificationBody(opts, ritualIssue.Number, issueURL(opts.Repo, ritualIssue.Number)),
	})
	if err != nil {
		return ChannelRitualResult{}, fmt.Errorf("queue channel ritual notification: %w", err)
	}
	return ChannelRitualResult{
		RitualIssueNumber: ritualIssue.Number,
		RitualIssueURL:    issueURL(opts.Repo, ritualIssue.Number),
		RitualCreated:     created,
		RitualDuplicate:   duplicate,
		Notification:      notification,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelRitualActionReport(ev Event, req ChannelRitualActionRequest, result ChannelRitualResult) string {
	status := "recorded"
	switch {
	case result.RitualDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.RitualDuplicate:
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
	b.WriteString("## GitClaw Channel Ritual Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_ritual_status: `%s`\n", status)
	fmt.Fprintf(&b, "- ritual_issue: `#%d`\n", result.RitualIssueNumber)
	fmt.Fprintf(&b, "- ritual_issue_url: `%s`\n", result.RitualIssueURL)
	fmt.Fprintf(&b, "- ritual_issue_created: `%t`\n", result.RitualCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.RitualDuplicate)
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
	fmt.Fprintf(&b, "- ritual_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.RitualID))
	fmt.Fprintf(&b, "- ritual_id_auto: `%t`\n", req.AutoRitualID)
	fmt.Fprintf(&b, "- ritual_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- ritual_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- ritual_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- ritual_cadence_sha256_12: `%s`\n", req.CadenceSHA)
	fmt.Fprintf(&b, "- ritual_cadence_bytes: `%d`\n", req.CadenceBytes)
	fmt.Fprintf(&b, "- ritual_cadence_lines: `%d`\n", req.CadenceLines)
	fmt.Fprintf(&b, "- ritual_trigger_sha256_12: `%s`\n", req.TriggerSHA)
	fmt.Fprintf(&b, "- ritual_trigger_bytes: `%d`\n", req.TriggerBytes)
	fmt.Fprintf(&b, "- ritual_trigger_lines: `%d`\n", req.TriggerLines)
	fmt.Fprintf(&b, "- ritual_practice_sha256_12: `%s`\n", req.PracticeSHA)
	fmt.Fprintf(&b, "- ritual_practice_bytes: `%d`\n", req.PracticeBytes)
	fmt.Fprintf(&b, "- ritual_practice_lines: `%d`\n", req.PracticeLines)
	fmt.Fprintf(&b, "- ritual_review_sha256_12: `%s`\n", req.ReviewSHA)
	fmt.Fprintf(&b, "- ritual_review_bytes: `%d`\n", req.ReviewBytes)
	fmt.Fprintf(&b, "- ritual_review_lines: `%d`\n", req.ReviewLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_ritual_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_ritual_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_ritual_cadence_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_ritual_trigger_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_ritual_practice_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_ritual_review_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- standing_order_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_ritual_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel ritual as a durable GitHub issue, then queued a provider-facing link back to the original thread. The ritual issue contains the human-readable title, cadence, trigger, practice, and review notes; this source receipt keeps provider IDs, ritual IDs, section text, and channel message bodies out of band. No schedule, reminder, standing order, workflow edit, or repository mutation is created by this action.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the ritual-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent ritual links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate ritual issues are suppressed by `ritual_id`; duplicate ritual-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the ritual issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelRitualIssueBody(opts ChannelRitualOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-ritual ritual_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.RitualID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel ritual.\n\n")
	fmt.Fprintf(&b, "- ritual_id: %s\n", opts.RitualID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- ritual_mode: github-issue-ritual\n")
	fmt.Fprintf(&b, "- scheduled_workflow_created: false\n")
	fmt.Fprintf(&b, "- reminder_created: false\n")
	fmt.Fprintf(&b, "- standing_order_created: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Cadence) != "" {
		b.WriteString("\n\n## Cadence\n\n")
		b.WriteString(strings.TrimSpace(opts.Cadence))
	}
	if strings.TrimSpace(opts.Trigger) != "" {
		b.WriteString("\n\n## Trigger\n\n")
		b.WriteString(strings.TrimSpace(opts.Trigger))
	}
	if strings.TrimSpace(opts.Practice) != "" {
		b.WriteString("\n\n## Practice\n\n")
		b.WriteString(strings.TrimSpace(opts.Practice))
	}
	if strings.TrimSpace(opts.Review) != "" {
		b.WriteString("\n\n## Review\n\n")
		b.WriteString(strings.TrimSpace(opts.Review))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for deciding whether the channel ritual should become a standing order, proactive workflow, reminder, skill, memory, or be closed.")
	return strings.TrimSpace(b.String())
}

func channelRitualActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRitualActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRitualIssueTarget(ev Event, req *ChannelRitualActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel ritual requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelRitualSections(trailing string, ev Event) (string, string, string, string, string) {
	lines := cleanChannelRitualTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel ritual from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", "", ""
	}
	ritual := channelRitualParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				ritual.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelRitualSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				ritual.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			ritual.Title = trimmed
			continue
		}
		if current == "" {
			current = "practice"
		}
		ritual.append(current, line)
	}
	return strings.TrimSpace(ritual.Title), strings.TrimSpace(ritual.Cadence), strings.TrimSpace(ritual.Trigger), strings.TrimSpace(ritual.Practice), strings.TrimSpace(ritual.Review)
}

type channelRitualParsedSections struct {
	Title    string
	Cadence  string
	Trigger  string
	Practice string
	Review   string
}

func (sections *channelRitualParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelRitualParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "cadence":
		sections.Cadence = appendChannelRitualSectionLine(sections.Cadence, value)
	case "trigger":
		sections.Trigger = appendChannelRitualSectionLine(sections.Trigger, value)
	case "practice":
		sections.Practice = appendChannelRitualSectionLine(sections.Practice, value)
	case "review":
		sections.Review = appendChannelRitualSectionLine(sections.Review, value)
	}
}

func appendChannelRitualSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelRitualSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelRitualHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "cadence":
		return "cadence", strings.TrimSpace(value), true
	case "trigger":
		return "trigger", strings.TrimSpace(value), true
	case "practice":
		return "practice", strings.TrimSpace(value), true
	case "review":
		return "review", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelRitualHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "ritual", "practice name", "routine", "habit", "name":
		return "title"
	case "cadence", "when", "schedule", "frequency", "rhythm":
		return "cadence"
	case "trigger", "start when", "cue", "signal":
		return "trigger"
	case "practice", "steps", "routine steps", "how", "what to do":
		return "practice"
	case "review", "done when", "check", "reflection", "acceptance":
		return "review"
	default:
		return ""
	}
}

func cleanChannelRitualTrailingLines(trailing string) []string {
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

func normalizeChannelRitualOptions(opts ChannelRitualOptions) ChannelRitualOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RitualID = cleanChannelRitualID(opts.RitualID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Cadence = strings.TrimSpace(opts.Cadence)
	opts.Trigger = strings.TrimSpace(opts.Trigger)
	opts.Practice = strings.TrimSpace(opts.Practice)
	opts.Review = strings.TrimSpace(opts.Review)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelRitualRoute(cfg Config, opts ChannelRitualOptions) (ChannelRitualOptions, error) {
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

func validateChannelRitualOptions(opts ChannelRitualOptions) error {
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
	if opts.RitualID == "" {
		return fmt.Errorf("missing ritual id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing ritual source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing ritual title")
	}
	return nil
}

func validateChannelRitualActionRequestOptions(opts ChannelRitualOptions) error {
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
	if opts.RitualID == "" {
		return fmt.Errorf("missing ritual id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing ritual source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing ritual title")
	}
	return nil
}

func findOrCreateChannelRitualIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRitualOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel ritual issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelRitualMatches(issue.Body, opts.RitualID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelRitualIssueTitle(opts), RenderChannelRitualIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel ritual issue: %w", err)
	}
	return issue, true, false, nil
}

func channelRitualIssueTitle(opts ChannelRitualOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.RitualID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel ritual: " + title
}

func channelRitualMatches(body, ritualID string) bool {
	return HasChannelRitualMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`ritual_id="%s"`, escapeMarkerValue(cleanChannelRitualID(ritualID))))
}

func cleanChannelRitualID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelRitualID(ev Event, channel, threadID, sourceMessageID, title, cadence, trigger, practice, review string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, cadence, trigger, practice, review}, "|")
	return fmt.Sprintf("ritual-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRitualNotifyMessageID(ev Event, ritualID string) string {
	seed := strings.Join([]string{eventID(ev), ritualID}, "|")
	return fmt.Sprintf("gitclaw-channel-ritual-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelRitualNotificationBody(opts ChannelRitualOptions, ritualIssueNumber int, ritualIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel ritual recorded.\n\n")
	if ritualIssueNumber > 0 {
		fmt.Fprintf(&b, "Ritual: #%d\n", ritualIssueNumber)
	}
	if ritualIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", ritualIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Cadence) != "" {
		fmt.Fprintf(&b, "Cadence: %s\n", strings.TrimSpace(opts.Cadence))
	}
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
