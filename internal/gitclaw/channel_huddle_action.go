package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelHuddleOptions struct {
	Repo              string
	HuddleID          string
	SourceIssueNumber int
	SourceCommentID   int64
	Topic             string
	Agenda            string
	Routes            []string
	MessageID         string
	Author            string
}

type ChannelHuddleResult struct {
	HuddleIssueNumber int
	HuddleIssueURL    string
	HuddleCreated     bool
	Broadcast         ChannelBroadcastResult
}

type ChannelHuddleActionRequest struct {
	Options          ChannelHuddleOptions
	Command          string
	Subcommand       string
	AutoHuddleID     bool
	AutoMessageID    bool
	TopicSHA         string
	TopicBytes       int
	TopicLines       int
	AgendaSHA        string
	AgendaBytes      int
	AgendaLines      int
	AgendaSource     string
	RoutesSHA        string
	RouteCount       int
	OutboundBodySHA  string
	OutboundBodySize int
}

func IsChannelHuddleActionRequest(ev Event, cfg Config) bool {
	return isChannelHuddleActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelHuddleActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "huddle", "room", "jam":
		return true
	default:
		return false
	}
}

func BuildChannelHuddleActionRequest(ev Event, cfg Config) (ChannelHuddleActionRequest, error) {
	fields, trailingAgenda, ok := channelHuddleActionFieldsAndTrailingAgenda(ev, cfg)
	if !ok {
		return ChannelHuddleActionRequest{}, fmt.Errorf("missing channel huddle command")
	}
	req := ChannelHuddleActionRequest{
		Options: ChannelHuddleOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:      fields[0],
		Subcommand:   strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		AgendaSource: "trailing-lines",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelHuddleActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelHuddleActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--huddle-id":
			if i+1 >= len(fields) {
				return ChannelHuddleActionRequest{}, fmt.Errorf("--huddle-id requires a value")
			}
			req.Options.HuddleID = cleanChannelHuddleID(fields[i+1])
			i++
		case "--message-id":
			if i+1 >= len(fields) {
				return ChannelHuddleActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelHuddleActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelHuddleActionRequest{}, fmt.Errorf("unknown channel huddle argument %q", field)
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
		}
	}

	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if len(req.Options.Routes) == 0 {
		return ChannelHuddleActionRequest{}, fmt.Errorf("missing huddle routes")
	}
	topic, agenda := parseChannelHuddleTopicAgenda(trailingAgenda, ev)
	req.Options.Topic = topic
	req.Options.Agenda = agenda
	if req.Options.HuddleID == "" {
		req.Options.HuddleID = autoChannelHuddleID(ev, req.Options.Routes, topic, agenda)
		req.AutoHuddleID = true
	}
	if req.Options.MessageID == "" {
		req.Options.MessageID = autoChannelHuddleMessageID(ev, req.Options.HuddleID, req.Options.Routes)
		req.AutoMessageID = true
	}
	if err := validateChannelHuddleOptions(req.Options); err != nil {
		return ChannelHuddleActionRequest{}, err
	}
	outboundPreview := renderChannelHuddleOutboundBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.TopicSHA = shortDocumentHash(topic)
	req.TopicBytes = len(topic)
	req.TopicLines = lineCount(topic)
	req.AgendaSHA = shortDocumentHash(agenda)
	req.AgendaBytes = len(agenda)
	req.AgendaLines = lineCount(agenda)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	req.OutboundBodySHA = shortDocumentHash(outboundPreview)
	req.OutboundBodySize = len(outboundPreview)
	return req, nil
}

func RunChannelHuddle(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelHuddleOptions) (ChannelHuddleResult, error) {
	opts = normalizeChannelHuddleOptions(opts)
	if err := validateChannelHuddleOptions(opts); err != nil {
		return ChannelHuddleResult{}, err
	}
	huddle, created, err := findOrCreateChannelHuddleIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelHuddleResult{}, err
	}
	broadcastOpts := ChannelBroadcastOptions{
		Repo:      opts.Repo,
		Routes:    opts.Routes,
		MessageID: opts.MessageID,
		Author:    opts.Author,
		Body:      renderChannelHuddleOutboundBody(opts, huddle.Number, issueURL(opts.Repo, huddle.Number)),
	}
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, broadcastOpts)
	if err != nil {
		return ChannelHuddleResult{}, err
	}
	return ChannelHuddleResult{
		HuddleIssueNumber: huddle.Number,
		HuddleIssueURL:    issueURL(opts.Repo, huddle.Number),
		HuddleCreated:     created,
		Broadcast:         broadcast,
	}, nil
}

func RenderChannelHuddleActionReport(ev Event, req ChannelHuddleActionRequest, result ChannelHuddleResult) string {
	status := "queued"
	switch {
	case !result.HuddleCreated && result.Broadcast.Queued == 0 && result.Broadcast.Duplicates > 0:
		status = "duplicate"
	case result.Broadcast.Queued > 0 && result.Broadcast.Duplicates > 0:
		status = "partially-queued"
	}
	outboundBody := renderChannelHuddleOutboundBody(req.Options, result.HuddleIssueNumber, result.HuddleIssueURL)
	outboundBodySHA := shortDocumentHash(outboundBody)
	outboundBodyBytes := len(outboundBody)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Huddle Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_huddle_status: `%s`\n", status)
	fmt.Fprintf(&b, "- huddle_issue: `#%d`\n", result.HuddleIssueNumber)
	fmt.Fprintf(&b, "- huddle_issue_url: `%s`\n", result.HuddleIssueURL)
	fmt.Fprintf(&b, "- huddle_issue_created: `%t`\n", result.HuddleCreated)
	fmt.Fprintf(&b, "- huddle_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.HuddleID))
	fmt.Fprintf(&b, "- huddle_id_auto: `%t`\n", req.AutoHuddleID)
	fmt.Fprintf(&b, "- huddle_topic_sha256_12: `%s`\n", req.TopicSHA)
	fmt.Fprintf(&b, "- huddle_topic_bytes: `%d`\n", req.TopicBytes)
	fmt.Fprintf(&b, "- huddle_topic_lines: `%d`\n", req.TopicLines)
	fmt.Fprintf(&b, "- huddle_agenda_sha256_12: `%s`\n", req.AgendaSHA)
	fmt.Fprintf(&b, "- huddle_agenda_bytes: `%d`\n", req.AgendaBytes)
	fmt.Fprintf(&b, "- huddle_agenda_lines: `%d`\n", req.AgendaLines)
	fmt.Fprintf(&b, "- huddle_agenda_source: `%s`\n", req.AgendaSource)
	fmt.Fprintf(&b, "- huddle_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- huddle_invites_queued: `%d`\n", result.Broadcast.Queued)
	fmt.Fprintf(&b, "- huddle_invite_duplicates: `%d`\n", result.Broadcast.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Broadcast.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", outboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", outboundBodyBytes)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_huddle_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_topic_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_agenda_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_huddle_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub huddle issue, then queued one provider-facing invitation per reviewed route. The huddle issue contains the human-readable topic and agenda; this source receipt keeps route names, huddle IDs, topic text, agenda text, thread IDs, message IDs, and outbound bodies out of band.\n\n")
	b.WriteString("### Destinations\n")
	if len(result.Broadcast.Destinations) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, destination := range result.Broadcast.Destinations {
			fmt.Fprintf(
				&b,
				"- destination=`%02d` target_issue=`#%d` outbound_comment_id=`%d` target_issue_created=`%t` duplicate_suppressed=`%t` route_sha256_12=`%s` channel=`%s` thread_id_sha256_12=`%s` message_id_sha256_12=`%s` body_sha256_12=`%s`\n",
				destination.Index,
				destination.IssueNumber,
				destination.CommentID,
				destination.Created,
				destination.Duplicate,
				noneIfEmpty(destination.RouteHash),
				destination.Channel,
				noneIfEmpty(destination.ThreadHash),
				noneIfEmpty(destination.MessageHash),
				noneIfEmpty(destination.BodyHash),
			)
		}
	}
	b.WriteString("\n### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending huddle invites with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent huddle invites with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate huddle invites are suppressed independently for each route by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}

func findOrCreateChannelHuddleIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelHuddleOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel huddle issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelHuddleMatches(issue.Body, opts.HuddleID) {
			return issue, false, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelHuddleIssueTitle(opts), RenderChannelHuddleIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel huddle issue: %w", err)
	}
	return issue, true, nil
}

func RenderChannelHuddleIssueBody(opts ChannelHuddleOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-huddle huddle_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" routes_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.HuddleID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(channelBroadcastRoutesHash(opts.Routes)))
	b.WriteString("GitClaw channel huddle.\n\n")
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: `%s`\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- huddle_id_sha256_12: `%s`\n", shortDocumentHash(opts.HuddleID))
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", channelBroadcastRoutesHash(opts.Routes))
	fmt.Fprintf(&b, "- route_count: `%d`\n", len(normalizeChannelBroadcastRoutes(opts.Routes)))
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n\n", false)
	b.WriteString("## Topic\n\n")
	b.WriteString(strings.TrimSpace(opts.Topic))
	b.WriteString("\n")
	if strings.TrimSpace(opts.Agenda) != "" {
		b.WriteString("\n## Agenda\n\n")
		b.WriteString(strings.TrimSpace(opts.Agenda))
		b.WriteString("\n")
	}
	b.WriteString("\nParticipants can continue here from GitHub, or through a mirrored channel thread that was invited into this huddle.")
	return strings.TrimSpace(b.String())
}

func channelHuddleActionFieldsAndTrailingAgenda(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelHuddleActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func renderChannelHuddleOutboundBody(opts ChannelHuddleOptions, issueNumber int, url string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel huddle\n\n")
	if issueNumber > 0 {
		fmt.Fprintf(&b, "Huddle: #%d\n", issueNumber)
	}
	fmt.Fprintf(&b, "URL: %s\n", url)
	fmt.Fprintf(&b, "Source: #%d\n", opts.SourceIssueNumber)
	b.WriteString("\nTopic:\n")
	b.WriteString(strings.TrimSpace(opts.Topic))
	b.WriteString("\n")
	if strings.TrimSpace(opts.Agenda) != "" {
		b.WriteString("\nAgenda:\n")
		b.WriteString(strings.TrimSpace(opts.Agenda))
		b.WriteString("\n")
	}
	b.WriteString("\nReply in the linked GitHub huddle or continue through the mirrored channel thread.")
	return strings.TrimSpace(b.String())
}

func parseChannelHuddleTopicAgenda(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTopic := fmt.Sprintf("Channel huddle for issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTopic, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var topic string
	var agendaLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "topic:"):
		topic = strings.TrimSpace(first[len("topic:"):])
		agendaLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "agenda:"):
		topic = defaultTopic
		agendaLines = cleaned
	default:
		topic = first
		agendaLines = cleaned[1:]
	}
	if topic == "" {
		topic = defaultTopic
	}
	agenda := strings.TrimSpace(strings.Join(agendaLines, "\n"))
	agendaLower := strings.ToLower(strings.TrimSpace(agenda))
	if strings.HasPrefix(agendaLower, "agenda:") {
		agenda = strings.TrimSpace(strings.TrimSpace(agenda)[len("agenda:"):])
	}
	return topic, agenda
}

func normalizeChannelHuddleOptions(opts ChannelHuddleOptions) ChannelHuddleOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.HuddleID = cleanChannelHuddleID(opts.HuddleID)
	opts.Topic = strings.TrimSpace(opts.Topic)
	opts.Agenda = strings.TrimSpace(opts.Agenda)
	opts.Routes = normalizeChannelBroadcastRoutes(opts.Routes)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelHuddleOptions(opts ChannelHuddleOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.HuddleID == "" {
		return fmt.Errorf("missing huddle id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing huddle source issue")
	}
	if strings.TrimSpace(opts.Topic) == "" {
		return fmt.Errorf("missing huddle topic")
	}
	if len(normalizeChannelBroadcastRoutes(opts.Routes)) == 0 {
		return fmt.Errorf("missing huddle routes")
	}
	if strings.TrimSpace(opts.MessageID) == "" {
		return fmt.Errorf("missing huddle message id")
	}
	return nil
}

func channelHuddleIssueTitle(opts ChannelHuddleOptions) string {
	topic := strings.TrimSpace(opts.Topic)
	if topic == "" {
		topic = fmt.Sprintf("issue #%d", opts.SourceIssueNumber)
	}
	topic = strings.ReplaceAll(topic, "\n", " ")
	if len(topic) > 80 {
		topic = strings.TrimSpace(topic[:80])
	}
	return "GitClaw channel huddle: " + topic
}

func channelHuddleMatches(body, huddleID string) bool {
	return HasChannelHuddleMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`huddle_id="%s"`, escapeMarkerValue(cleanChannelHuddleID(huddleID))))
}

func cleanChannelHuddleID(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	var b strings.Builder
	dash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == '_' || r == '.':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		if b.Len() >= 80 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func autoChannelHuddleID(ev Event, routes []string, topic, agenda string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(normalizeChannelBroadcastRoutes(routes), ","), topic, agenda}, "|")
	return fmt.Sprintf("huddle-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelHuddleMessageID(ev Event, huddleID string, routes []string) string {
	seed := strings.Join([]string{eventID(ev), huddleID, strings.Join(normalizeChannelBroadcastRoutes(routes), ",")}, "|")
	return fmt.Sprintf("gitclaw-huddle-%s-%s", eventID(ev), shortDocumentHash(seed))
}
