package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelForecastOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ForecastID        string
	Title             string
	Prediction        string
	Evidence          string
	Resolution        string
	Due               string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelForecastResult struct {
	ForecastIssueNumber int
	ForecastIssueURL    string
	ForecastCreated     bool
	ForecastDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelForecastActionRequest struct {
	Options             ChannelForecastOptions
	Command             string
	Subcommand          string
	AutoForecastID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	PredictionSHA       string
	PredictionBytes     int
	PredictionLines     int
	EvidenceSHA         string
	EvidenceBytes       int
	EvidenceLines       int
	ResolutionSHA       string
	ResolutionBytes     int
	ResolutionLines     int
	DueSHA              string
	DueBytes            int
	DueLines            int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelForecastActionRequest(ev Event, cfg Config) bool {
	return isChannelForecastActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelForecastActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "forecast", "prediction", "predict", "bet", "call", "hunch":
		return true
	default:
		return false
	}
}

func BuildChannelForecastActionRequest(ev Event, cfg Config) (ChannelForecastActionRequest, error) {
	fields, trailing, ok := channelForecastActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelForecastActionRequest{}, fmt.Errorf("missing channel forecast command")
	}
	req := ChannelForecastActionRequest{
		Options: ChannelForecastOptions{
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
				return ChannelForecastActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelForecastActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelForecastActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelForecastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelForecastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--forecast-id", "--prediction-id", "--bet-id", "--call-id", "--id":
			if i+1 >= len(fields) {
				return ChannelForecastActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ForecastID = cleanChannelForecastID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelForecastActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelForecastActionRequest{}, fmt.Errorf("unknown channel forecast argument %q", field)
			}
			if req.Options.ForecastID == "" {
				req.Options.ForecastID = cleanChannelForecastID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelForecastActionRequest{}, fmt.Errorf("unexpected channel forecast argument %q", field)
		}
	}
	if err := applyChannelForecastIssueTarget(ev, &req); err != nil {
		return ChannelForecastActionRequest{}, err
	}
	title, prediction, evidence, resolution, due := parseChannelForecastSections(trailing, ev)
	req.Options.Title = title
	req.Options.Prediction = prediction
	req.Options.Evidence = evidence
	req.Options.Resolution = resolution
	req.Options.Due = due
	if strings.TrimSpace(req.Options.ForecastID) == "" {
		req.Options.ForecastID = autoChannelForecastID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, prediction, evidence, resolution, due)
		req.AutoForecastID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelForecastNotifyMessageID(ev, req.Options.ForecastID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelForecastOptions(req.Options)
	if err := validateChannelForecastActionRequestOptions(req.Options); err != nil {
		return ChannelForecastActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.PredictionSHA = shortDocumentHash(req.Options.Prediction)
	req.PredictionBytes = len(req.Options.Prediction)
	req.PredictionLines = lineCount(req.Options.Prediction)
	req.EvidenceSHA = shortDocumentHash(req.Options.Evidence)
	req.EvidenceBytes = len(req.Options.Evidence)
	req.EvidenceLines = lineCount(req.Options.Evidence)
	req.ResolutionSHA = shortDocumentHash(req.Options.Resolution)
	req.ResolutionBytes = len(req.Options.Resolution)
	req.ResolutionLines = lineCount(req.Options.Resolution)
	req.DueSHA = shortDocumentHash(req.Options.Due)
	req.DueBytes = len(req.Options.Due)
	req.DueLines = lineCount(req.Options.Due)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelForecastNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelForecast(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelForecastOptions) (ChannelForecastResult, error) {
	opts = normalizeChannelForecastOptions(opts)
	var err error
	opts, err = applyChannelForecastRoute(cfg, opts)
	if err != nil {
		return ChannelForecastResult{}, err
	}
	if err := validateChannelForecastOptions(opts); err != nil {
		return ChannelForecastResult{}, err
	}
	forecastIssue, created, duplicate, err := findOrCreateChannelForecastIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelForecastResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelForecastNotificationBody(opts, forecastIssue.Number, issueURL(opts.Repo, forecastIssue.Number)),
	})
	if err != nil {
		return ChannelForecastResult{}, fmt.Errorf("queue channel forecast notification: %w", err)
	}
	return ChannelForecastResult{
		ForecastIssueNumber: forecastIssue.Number,
		ForecastIssueURL:    issueURL(opts.Repo, forecastIssue.Number),
		ForecastCreated:     created,
		ForecastDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelForecastActionReport(ev Event, req ChannelForecastActionRequest, result ChannelForecastResult) string {
	status := "recorded"
	switch {
	case result.ForecastDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ForecastDuplicate:
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
	b.WriteString("## GitClaw Channel Forecast Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_forecast_status: `%s`\n", status)
	fmt.Fprintf(&b, "- forecast_issue: `#%d`\n", result.ForecastIssueNumber)
	fmt.Fprintf(&b, "- forecast_issue_url: `%s`\n", result.ForecastIssueURL)
	fmt.Fprintf(&b, "- forecast_issue_created: `%t`\n", result.ForecastCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ForecastDuplicate)
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
	fmt.Fprintf(&b, "- forecast_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ForecastID))
	fmt.Fprintf(&b, "- forecast_id_auto: `%t`\n", req.AutoForecastID)
	fmt.Fprintf(&b, "- forecast_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- forecast_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- forecast_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- forecast_prediction_sha256_12: `%s`\n", req.PredictionSHA)
	fmt.Fprintf(&b, "- forecast_prediction_bytes: `%d`\n", req.PredictionBytes)
	fmt.Fprintf(&b, "- forecast_prediction_lines: `%d`\n", req.PredictionLines)
	fmt.Fprintf(&b, "- forecast_evidence_sha256_12: `%s`\n", req.EvidenceSHA)
	fmt.Fprintf(&b, "- forecast_evidence_bytes: `%d`\n", req.EvidenceBytes)
	fmt.Fprintf(&b, "- forecast_evidence_lines: `%d`\n", req.EvidenceLines)
	fmt.Fprintf(&b, "- forecast_resolution_sha256_12: `%s`\n", req.ResolutionSHA)
	fmt.Fprintf(&b, "- forecast_resolution_bytes: `%d`\n", req.ResolutionBytes)
	fmt.Fprintf(&b, "- forecast_resolution_lines: `%d`\n", req.ResolutionLines)
	fmt.Fprintf(&b, "- forecast_due_sha256_12: `%s`\n", req.DueSHA)
	fmt.Fprintf(&b, "- forecast_due_bytes: `%d`\n", req.DueBytes)
	fmt.Fprintf(&b, "- forecast_due_lines: `%d`\n", req.DueLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_forecast_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_forecast_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_forecast_prediction_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_forecast_evidence_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_forecast_resolution_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_forecast_due_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- wager_created: `%t`\n", false)
	fmt.Fprintf(&b, "- money_or_points_tracked: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_forecast_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel forecast as a durable GitHub issue, then queued a provider-facing link back to the original thread. The forecast issue contains the human-readable title, prediction, evidence, resolution, and due/review timing; this source receipt keeps provider IDs, forecast IDs, section text, and channel message bodies out of band. No schedule, reminder, betting market, money/points tracking, provider delivery, or repository mutation is created by this action.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the forecast-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent forecast links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate forecast issues are suppressed by `forecast_id`; duplicate forecast-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the forecast issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelForecastIssueBody(opts ChannelForecastOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-forecast forecast_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ForecastID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel forecast.\n\n")
	fmt.Fprintf(&b, "- forecast_id: %s\n", opts.ForecastID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- forecast_mode: github-issue-forecast\n")
	fmt.Fprintf(&b, "- scheduled_workflow_created: false\n")
	fmt.Fprintf(&b, "- reminder_created: false\n")
	fmt.Fprintf(&b, "- wager_created: false\n")
	fmt.Fprintf(&b, "- money_or_points_tracked: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Prediction) != "" {
		b.WriteString("\n\n## Prediction\n\n")
		b.WriteString(strings.TrimSpace(opts.Prediction))
	}
	if strings.TrimSpace(opts.Evidence) != "" {
		b.WriteString("\n\n## Evidence\n\n")
		b.WriteString(strings.TrimSpace(opts.Evidence))
	}
	if strings.TrimSpace(opts.Resolution) != "" {
		b.WriteString("\n\n## Resolution\n\n")
		b.WriteString(strings.TrimSpace(opts.Resolution))
	}
	if strings.TrimSpace(opts.Due) != "" {
		b.WriteString("\n\n## Due / Review\n\n")
		b.WriteString(strings.TrimSpace(opts.Due))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel forecast. Close it when the forecast has resolved, or promote follow-up work through normal reviewed GitHub issues.")
	return strings.TrimSpace(b.String())
}

func channelForecastActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelForecastActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelForecastIssueTarget(ev Event, req *ChannelForecastActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel forecast requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelForecastSections(trailing string, ev Event) (string, string, string, string, string) {
	lines := cleanChannelForecastTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel forecast from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", "", ""
	}
	forecast := channelForecastParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				forecast.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelForecastSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				forecast.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			forecast.Title = trimmed
			continue
		}
		if current == "" {
			current = "prediction"
		}
		forecast.append(current, line)
	}
	return strings.TrimSpace(forecast.Title), strings.TrimSpace(forecast.Prediction), strings.TrimSpace(forecast.Evidence), strings.TrimSpace(forecast.Resolution), strings.TrimSpace(forecast.Due)
}

type channelForecastParsedSections struct {
	Title      string
	Prediction string
	Evidence   string
	Resolution string
	Due        string
}

func (sections *channelForecastParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelForecastParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "prediction":
		sections.Prediction = appendChannelForecastSectionLine(sections.Prediction, value)
	case "evidence":
		sections.Evidence = appendChannelForecastSectionLine(sections.Evidence, value)
	case "resolution":
		sections.Resolution = appendChannelForecastSectionLine(sections.Resolution, value)
	case "due":
		sections.Due = appendChannelForecastSectionLine(sections.Due, value)
	}
}

func appendChannelForecastSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelForecastSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelForecastHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "prediction":
		return "prediction", strings.TrimSpace(value), true
	case "evidence":
		return "evidence", strings.TrimSpace(value), true
	case "resolution":
		return "resolution", strings.TrimSpace(value), true
	case "due":
		return "due", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelForecastHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "forecast", "prediction title", "bet", "call", "name":
		return "title"
	case "prediction", "predict", "claim", "expected", "what", "hunch":
		return "prediction"
	case "evidence", "reasoning", "rationale", "why", "signals", "because":
		return "evidence"
	case "resolution", "resolution criteria", "resolve", "resolved when", "outcome", "check":
		return "resolution"
	case "due", "review", "by", "deadline", "resolve by", "revisit", "date":
		return "due"
	default:
		return ""
	}
}

func cleanChannelForecastTrailingLines(trailing string) []string {
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

func normalizeChannelForecastOptions(opts ChannelForecastOptions) ChannelForecastOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ForecastID = cleanChannelForecastID(opts.ForecastID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Prediction = strings.TrimSpace(opts.Prediction)
	opts.Evidence = strings.TrimSpace(opts.Evidence)
	opts.Resolution = strings.TrimSpace(opts.Resolution)
	opts.Due = strings.TrimSpace(opts.Due)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelForecastRoute(cfg Config, opts ChannelForecastOptions) (ChannelForecastOptions, error) {
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

func validateChannelForecastOptions(opts ChannelForecastOptions) error {
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
	if opts.ForecastID == "" {
		return fmt.Errorf("missing forecast id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing forecast source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing forecast title")
	}
	return nil
}

func validateChannelForecastActionRequestOptions(opts ChannelForecastOptions) error {
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
	if opts.ForecastID == "" {
		return fmt.Errorf("missing forecast id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing forecast source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing forecast title")
	}
	return nil
}

func findOrCreateChannelForecastIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelForecastOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel forecast issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelForecastMatches(issue.Body, opts.ForecastID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelForecastIssueTitle(opts), RenderChannelForecastIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel forecast issue: %w", err)
	}
	return issue, true, false, nil
}

func channelForecastIssueTitle(opts ChannelForecastOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.ForecastID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel forecast: " + title
}

func channelForecastMatches(body, forecastID string) bool {
	return HasChannelForecastMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`forecast_id="%s"`, escapeMarkerValue(cleanChannelForecastID(forecastID))))
}

func cleanChannelForecastID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelForecastID(ev Event, channel, threadID, sourceMessageID, title, prediction, evidence, resolution, due string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, prediction, evidence, resolution, due}, "|")
	return fmt.Sprintf("forecast-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelForecastNotifyMessageID(ev Event, forecastID string) string {
	seed := strings.Join([]string{eventID(ev), forecastID}, "|")
	return fmt.Sprintf("gitclaw-channel-forecast-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelForecastNotificationBody(opts ChannelForecastOptions, forecastIssueNumber int, forecastIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel forecast recorded.\n\n")
	if forecastIssueNumber > 0 {
		fmt.Fprintf(&b, "Forecast: #%d\n", forecastIssueNumber)
	}
	if forecastIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", forecastIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Due) != "" {
		fmt.Fprintf(&b, "Due: %s\n", strings.TrimSpace(opts.Due))
	}
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
