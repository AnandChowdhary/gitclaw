package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelInsightOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	InsightID         string
	Title             string
	Observation       string
	Evidence          string
	Recommendation    string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelInsightResult struct {
	InsightIssueNumber int
	InsightIssueURL    string
	InsightCreated     bool
	InsightDuplicate   bool
	Notification       ChannelSendResult
	RouteName          string
	RouteHash          string
	Channel            string
	ThreadHash         string
	MessageHash        string
	NotifyHash         string
}

type ChannelInsightActionRequest struct {
	Options             ChannelInsightOptions
	Command             string
	Subcommand          string
	AutoInsightID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ObservationSHA      string
	ObservationBytes    int
	ObservationLines    int
	EvidenceSHA         string
	EvidenceBytes       int
	EvidenceLines       int
	RecommendationSHA   string
	RecommendationBytes int
	RecommendationLines int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelInsightActionRequest(ev Event, cfg Config) bool {
	return isChannelInsightActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelInsightActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "insight", "observation", "finding", "learning", "takeaway", "signal", "lesson-learned":
		return true
	default:
		return false
	}
}

func BuildChannelInsightActionRequest(ev Event, cfg Config) (ChannelInsightActionRequest, error) {
	fields, trailing, ok := channelInsightActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelInsightActionRequest{}, fmt.Errorf("missing channel insight command")
	}
	req := ChannelInsightActionRequest{
		Options: ChannelInsightOptions{
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
				return ChannelInsightActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelInsightActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelInsightActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelInsightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelInsightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--insight-id", "--observation-id", "--finding-id", "--learning-id", "--takeaway-id", "--signal-id", "--id":
			if i+1 >= len(fields) {
				return ChannelInsightActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.InsightID = cleanChannelInsightID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelInsightActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelInsightActionRequest{}, fmt.Errorf("unknown channel insight argument %q", field)
			}
			if req.Options.InsightID == "" {
				req.Options.InsightID = cleanChannelInsightID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelInsightActionRequest{}, fmt.Errorf("unexpected channel insight argument %q", field)
		}
	}
	if err := applyChannelInsightIssueTarget(ev, &req); err != nil {
		return ChannelInsightActionRequest{}, err
	}
	title, observation, evidence, recommendation := parseChannelInsightSections(trailing, ev)
	req.Options.Title = title
	req.Options.Observation = observation
	req.Options.Evidence = evidence
	req.Options.Recommendation = recommendation
	if strings.TrimSpace(req.Options.InsightID) == "" {
		req.Options.InsightID = autoChannelInsightID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, observation, evidence, recommendation)
		req.AutoInsightID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelInsightNotifyMessageID(ev, req.Options.InsightID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelInsightOptions(req.Options)
	if err := validateChannelInsightActionRequestOptions(req.Options); err != nil {
		return ChannelInsightActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ObservationSHA = shortDocumentHash(req.Options.Observation)
	req.ObservationBytes = len(req.Options.Observation)
	req.ObservationLines = lineCount(req.Options.Observation)
	req.EvidenceSHA = shortDocumentHash(req.Options.Evidence)
	req.EvidenceBytes = len(req.Options.Evidence)
	req.EvidenceLines = lineCount(req.Options.Evidence)
	req.RecommendationSHA = shortDocumentHash(req.Options.Recommendation)
	req.RecommendationBytes = len(req.Options.Recommendation)
	req.RecommendationLines = lineCount(req.Options.Recommendation)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelInsightNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelInsight(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelInsightOptions) (ChannelInsightResult, error) {
	opts = normalizeChannelInsightOptions(opts)
	var err error
	opts, err = applyChannelInsightRoute(cfg, opts)
	if err != nil {
		return ChannelInsightResult{}, err
	}
	if err := validateChannelInsightOptions(opts); err != nil {
		return ChannelInsightResult{}, err
	}
	insightIssue, created, duplicate, err := findOrCreateChannelInsightIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelInsightResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelInsightNotificationBody(opts, insightIssue.Number, issueURL(opts.Repo, insightIssue.Number)),
	})
	if err != nil {
		return ChannelInsightResult{}, fmt.Errorf("queue channel insight notification: %w", err)
	}
	return ChannelInsightResult{
		InsightIssueNumber: insightIssue.Number,
		InsightIssueURL:    issueURL(opts.Repo, insightIssue.Number),
		InsightCreated:     created,
		InsightDuplicate:   duplicate,
		Notification:       notification,
		RouteName:          opts.Route,
		RouteHash:          channelRouteHash(opts.Route),
		Channel:            opts.Channel,
		ThreadHash:         shortDocumentHash(opts.ThreadID),
		MessageHash:        shortDocumentHash(opts.SourceMessageID),
		NotifyHash:         shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelInsightActionReport(ev Event, req ChannelInsightActionRequest, result ChannelInsightResult) string {
	status := "recorded"
	switch {
	case result.InsightDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.InsightDuplicate:
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
	b.WriteString("## GitClaw Channel Insight Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_insight_status: `%s`\n", status)
	fmt.Fprintf(&b, "- insight_issue: `#%d`\n", result.InsightIssueNumber)
	fmt.Fprintf(&b, "- insight_issue_url: `%s`\n", result.InsightIssueURL)
	fmt.Fprintf(&b, "- insight_issue_created: `%t`\n", result.InsightCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.InsightDuplicate)
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
	fmt.Fprintf(&b, "- insight_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.InsightID))
	fmt.Fprintf(&b, "- insight_id_auto: `%t`\n", req.AutoInsightID)
	fmt.Fprintf(&b, "- insight_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- insight_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- insight_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- insight_observation_sha256_12: `%s`\n", req.ObservationSHA)
	fmt.Fprintf(&b, "- insight_observation_bytes: `%d`\n", req.ObservationBytes)
	fmt.Fprintf(&b, "- insight_observation_lines: `%d`\n", req.ObservationLines)
	fmt.Fprintf(&b, "- insight_evidence_sha256_12: `%s`\n", req.EvidenceSHA)
	fmt.Fprintf(&b, "- insight_evidence_bytes: `%d`\n", req.EvidenceBytes)
	fmt.Fprintf(&b, "- insight_evidence_lines: `%d`\n", req.EvidenceLines)
	fmt.Fprintf(&b, "- insight_recommendation_sha256_12: `%s`\n", req.RecommendationSHA)
	fmt.Fprintf(&b, "- insight_recommendation_bytes: `%d`\n", req.RecommendationBytes)
	fmt.Fprintf(&b, "- insight_recommendation_lines: `%d`\n", req.RecommendationLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_insight_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_insight_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_insight_observation_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_insight_evidence_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_insight_recommendation_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_insight_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel insight as a durable GitHub issue, then queued a provider-facing link back to the original thread. The insight issue contains the human-readable title, observation, evidence, and recommendation; this source receipt keeps provider IDs, insight IDs, section text, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the insight-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent insight links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate insight issues are suppressed by `insight_id`; duplicate insight-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the insight issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelInsightIssueBody(opts ChannelInsightOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-insight insight_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.InsightID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel insight.\n\n")
	fmt.Fprintf(&b, "- insight_id: %s\n", opts.InsightID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- insight_mode: github-issue-insight\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Observation) != "" {
		b.WriteString("\n\n## Observation\n\n")
		b.WriteString(strings.TrimSpace(opts.Observation))
	}
	if strings.TrimSpace(opts.Evidence) != "" {
		b.WriteString("\n\n## Evidence\n\n")
		b.WriteString(strings.TrimSpace(opts.Evidence))
	}
	if strings.TrimSpace(opts.Recommendation) != "" {
		b.WriteString("\n\n## Recommendation\n\n")
		b.WriteString(strings.TrimSpace(opts.Recommendation))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel insight.")
	return strings.TrimSpace(b.String())
}

func channelInsightActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelInsightActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelInsightIssueTarget(ev Event, req *ChannelInsightActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel insight requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelInsightSections(trailing string, ev Event) (string, string, string, string) {
	lines := cleanChannelInsightTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel insight from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", ""
	}
	insight := channelInsightParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				insight.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelInsightSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				insight.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			insight.Title = trimmed
			continue
		}
		if current == "" {
			current = "observation"
		}
		insight.append(current, line)
	}
	return strings.TrimSpace(insight.Title), strings.TrimSpace(insight.Observation), strings.TrimSpace(insight.Evidence), strings.TrimSpace(insight.Recommendation)
}

type channelInsightParsedSections struct {
	Title          string
	Observation    string
	Evidence       string
	Recommendation string
}

func (sections *channelInsightParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelInsightParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "observation":
		sections.Observation = appendChannelInsightSectionLine(sections.Observation, value)
	case "evidence":
		sections.Evidence = appendChannelInsightSectionLine(sections.Evidence, value)
	case "recommendation":
		sections.Recommendation = appendChannelInsightSectionLine(sections.Recommendation, value)
	}
}

func appendChannelInsightSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelInsightSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelInsightHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "observation":
		return "observation", strings.TrimSpace(value), true
	case "evidence":
		return "evidence", strings.TrimSpace(value), true
	case "recommendation":
		return "recommendation", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelInsightHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "insight", "finding", "learning", "takeaway", "signal", "lesson learned":
		return "title"
	case "observation", "observed", "what happened", "pattern":
		return "observation"
	case "evidence", "source", "examples", "example", "supporting evidence":
		return "evidence"
	case "recommendation", "recommended action", "next", "next step", "apply":
		return "recommendation"
	default:
		return ""
	}
}

func cleanChannelInsightTrailingLines(trailing string) []string {
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

func normalizeChannelInsightOptions(opts ChannelInsightOptions) ChannelInsightOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.InsightID = cleanChannelInsightID(opts.InsightID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Observation = strings.TrimSpace(opts.Observation)
	opts.Evidence = strings.TrimSpace(opts.Evidence)
	opts.Recommendation = strings.TrimSpace(opts.Recommendation)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelInsightRoute(cfg Config, opts ChannelInsightOptions) (ChannelInsightOptions, error) {
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

func validateChannelInsightOptions(opts ChannelInsightOptions) error {
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
	if opts.InsightID == "" {
		return fmt.Errorf("missing insight id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing insight source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing insight title")
	}
	return nil
}

func validateChannelInsightActionRequestOptions(opts ChannelInsightOptions) error {
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
	if opts.InsightID == "" {
		return fmt.Errorf("missing insight id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing insight source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing insight title")
	}
	return nil
}

func findOrCreateChannelInsightIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelInsightOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel insight issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelInsightMatches(issue.Body, opts.InsightID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelInsightIssueTitle(opts), RenderChannelInsightIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel insight issue: %w", err)
	}
	return issue, true, false, nil
}

func channelInsightIssueTitle(opts ChannelInsightOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.InsightID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel insight: " + title
}

func channelInsightMatches(body, insightID string) bool {
	return HasChannelInsightMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`insight_id="%s"`, escapeMarkerValue(cleanChannelInsightID(insightID))))
}

func cleanChannelInsightID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelInsightID(ev Event, channel, threadID, sourceMessageID, title, observation, evidence, recommendation string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, observation, evidence, recommendation}, "|")
	return fmt.Sprintf("insight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelInsightNotifyMessageID(ev Event, insightID string) string {
	seed := strings.Join([]string{eventID(ev), insightID}, "|")
	return fmt.Sprintf("gitclaw-channel-insight-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelInsightNotificationBody(opts ChannelInsightOptions, insightIssueNumber int, insightIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel insight recorded.\n\n")
	if insightIssueNumber > 0 {
		fmt.Fprintf(&b, "Insight: #%d\n", insightIssueNumber)
	}
	if insightIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", insightIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
