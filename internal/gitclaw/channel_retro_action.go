package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRetroOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RetroID           string
	Title             string
	WentWell          string
	RoughEdges        string
	Next              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRetroResult struct {
	RetroIssueNumber int
	RetroIssueURL    string
	RetroCreated     bool
	RetroDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelRetroActionRequest struct {
	Options             ChannelRetroOptions
	Command             string
	Subcommand          string
	AutoRetroID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	WentWellSHA         string
	WentWellBytes       int
	WentWellLines       int
	RoughEdgesSHA       string
	RoughEdgesBytes     int
	RoughEdgesLines     int
	NextSHA             string
	NextBytes           int
	NextLines           int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelRetroActionRequest(ev Event, cfg Config) bool {
	return isChannelRetroActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRetroActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "retro", "retrospective", "review", "after-action", "after-action-review", "aar", "lessons":
		return true
	default:
		return false
	}
}

func BuildChannelRetroActionRequest(ev Event, cfg Config) (ChannelRetroActionRequest, error) {
	fields, trailing, ok := channelRetroActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRetroActionRequest{}, fmt.Errorf("missing channel retro command")
	}
	req := ChannelRetroActionRequest{
		Options: ChannelRetroOptions{
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
				return ChannelRetroActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRetroActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRetroActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelRetroActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRetroActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--retro-id", "--retrospective-id", "--review-id", "--aar-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRetroActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RetroID = cleanChannelRetroID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRetroActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRetroActionRequest{}, fmt.Errorf("unknown channel retro argument %q", field)
			}
			if req.Options.RetroID == "" {
				req.Options.RetroID = cleanChannelRetroID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelRetroActionRequest{}, fmt.Errorf("unexpected channel retro argument %q", field)
		}
	}
	if err := applyChannelRetroIssueTarget(ev, &req); err != nil {
		return ChannelRetroActionRequest{}, err
	}
	title, wentWell, roughEdges, next := parseChannelRetroSections(trailing, ev)
	req.Options.Title = title
	req.Options.WentWell = wentWell
	req.Options.RoughEdges = roughEdges
	req.Options.Next = next
	if strings.TrimSpace(req.Options.RetroID) == "" {
		req.Options.RetroID = autoChannelRetroID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, wentWell, roughEdges, next)
		req.AutoRetroID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRetroNotifyMessageID(ev, req.Options.RetroID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRetroOptions(req.Options)
	if err := validateChannelRetroActionRequestOptions(req.Options); err != nil {
		return ChannelRetroActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.WentWellSHA = shortDocumentHash(req.Options.WentWell)
	req.WentWellBytes = len(req.Options.WentWell)
	req.WentWellLines = lineCount(req.Options.WentWell)
	req.RoughEdgesSHA = shortDocumentHash(req.Options.RoughEdges)
	req.RoughEdgesBytes = len(req.Options.RoughEdges)
	req.RoughEdgesLines = lineCount(req.Options.RoughEdges)
	req.NextSHA = shortDocumentHash(req.Options.Next)
	req.NextBytes = len(req.Options.Next)
	req.NextLines = lineCount(req.Options.Next)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelRetroNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelRetro(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRetroOptions) (ChannelRetroResult, error) {
	opts = normalizeChannelRetroOptions(opts)
	var err error
	opts, err = applyChannelRetroRoute(cfg, opts)
	if err != nil {
		return ChannelRetroResult{}, err
	}
	if err := validateChannelRetroOptions(opts); err != nil {
		return ChannelRetroResult{}, err
	}
	retroIssue, created, duplicate, err := findOrCreateChannelRetroIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelRetroResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelRetroNotificationBody(opts, retroIssue.Number, issueURL(opts.Repo, retroIssue.Number)),
	})
	if err != nil {
		return ChannelRetroResult{}, fmt.Errorf("queue channel retro notification: %w", err)
	}
	return ChannelRetroResult{
		RetroIssueNumber: retroIssue.Number,
		RetroIssueURL:    issueURL(opts.Repo, retroIssue.Number),
		RetroCreated:     created,
		RetroDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelRetroActionReport(ev Event, req ChannelRetroActionRequest, result ChannelRetroResult) string {
	status := "recorded"
	switch {
	case result.RetroDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.RetroDuplicate:
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
	b.WriteString("## GitClaw Channel Retro Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_retro_status: `%s`\n", status)
	fmt.Fprintf(&b, "- retro_issue: `#%d`\n", result.RetroIssueNumber)
	fmt.Fprintf(&b, "- retro_issue_url: `%s`\n", result.RetroIssueURL)
	fmt.Fprintf(&b, "- retro_issue_created: `%t`\n", result.RetroCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.RetroDuplicate)
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
	fmt.Fprintf(&b, "- retro_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.RetroID))
	fmt.Fprintf(&b, "- retro_id_auto: `%t`\n", req.AutoRetroID)
	fmt.Fprintf(&b, "- retro_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- retro_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- retro_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- retro_went_well_sha256_12: `%s`\n", req.WentWellSHA)
	fmt.Fprintf(&b, "- retro_went_well_bytes: `%d`\n", req.WentWellBytes)
	fmt.Fprintf(&b, "- retro_went_well_lines: `%d`\n", req.WentWellLines)
	fmt.Fprintf(&b, "- retro_rough_edges_sha256_12: `%s`\n", req.RoughEdgesSHA)
	fmt.Fprintf(&b, "- retro_rough_edges_bytes: `%d`\n", req.RoughEdgesBytes)
	fmt.Fprintf(&b, "- retro_rough_edges_lines: `%d`\n", req.RoughEdgesLines)
	fmt.Fprintf(&b, "- retro_next_sha256_12: `%s`\n", req.NextSHA)
	fmt.Fprintf(&b, "- retro_next_bytes: `%d`\n", req.NextBytes)
	fmt.Fprintf(&b, "- retro_next_lines: `%d`\n", req.NextLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_retro_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_retro_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_retro_went_well_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_retro_rough_edges_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_retro_next_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_retro_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel retrospective as a durable GitHub issue, then queued a provider-facing link back to the original thread. The retro issue contains the human-readable title, went-well notes, rough edges, and next steps; this source receipt keeps provider IDs, retro IDs, section text, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the retro-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent retro links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate retro issues are suppressed by `retro_id`; duplicate retro-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the retro issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelRetroIssueBody(opts ChannelRetroOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-retro retro_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.RetroID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel retrospective.\n\n")
	fmt.Fprintf(&b, "- retro_id: %s\n", opts.RetroID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- retro_mode: github-issue-retro\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.WentWell) != "" {
		b.WriteString("\n\n## Went Well\n\n")
		b.WriteString(strings.TrimSpace(opts.WentWell))
	}
	if strings.TrimSpace(opts.RoughEdges) != "" {
		b.WriteString("\n\n## Rough Edges\n\n")
		b.WriteString(strings.TrimSpace(opts.RoughEdges))
	}
	if strings.TrimSpace(opts.Next) != "" {
		b.WriteString("\n\n## Next\n\n")
		b.WriteString(strings.TrimSpace(opts.Next))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel retrospective.")
	return strings.TrimSpace(b.String())
}

func channelRetroActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRetroActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRetroIssueTarget(ev Event, req *ChannelRetroActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel retro requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelRetroSections(trailing string, ev Event) (string, string, string, string) {
	lines := cleanTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel retro from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", ""
	}
	retro := channelRetroParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				retro.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelRetroSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				retro.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			retro.Title = trimmed
			continue
		}
		if current == "" {
			current = "next"
		}
		retro.append(current, line)
	}
	return strings.TrimSpace(retro.Title), strings.TrimSpace(retro.WentWell), strings.TrimSpace(retro.RoughEdges), strings.TrimSpace(retro.Next)
}

type channelRetroParsedSections struct {
	Title      string
	WentWell   string
	RoughEdges string
	Next       string
}

func (sections *channelRetroParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelRetroParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "went_well":
		sections.WentWell = appendSectionLine(sections.WentWell, value)
	case "rough_edges":
		sections.RoughEdges = appendSectionLine(sections.RoughEdges, value)
	case "next":
		sections.Next = appendSectionLine(sections.Next, value)
	}
}

func appendSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelRetroSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelRetroHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "went_well":
		return "went_well", strings.TrimSpace(value), true
	case "rough_edges":
		return "rough_edges", strings.TrimSpace(value), true
	case "next":
		return "next", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelRetroHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "retro", "retrospective", "review":
		return "title"
	case "went well", "what went well", "good", "wins":
		return "went_well"
	case "rough edges", "what was hard", "blockers", "needs work":
		return "rough_edges"
	case "next", "next experiment", "actions", "try next":
		return "next"
	default:
		return ""
	}
}

func cleanTrailingLines(trailing string) []string {
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

func normalizeChannelRetroOptions(opts ChannelRetroOptions) ChannelRetroOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RetroID = cleanChannelRetroID(opts.RetroID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.WentWell = strings.TrimSpace(opts.WentWell)
	opts.RoughEdges = strings.TrimSpace(opts.RoughEdges)
	opts.Next = strings.TrimSpace(opts.Next)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelRetroRoute(cfg Config, opts ChannelRetroOptions) (ChannelRetroOptions, error) {
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

func validateChannelRetroOptions(opts ChannelRetroOptions) error {
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
	if opts.RetroID == "" {
		return fmt.Errorf("missing retro id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing retro source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing retro title")
	}
	return nil
}

func validateChannelRetroActionRequestOptions(opts ChannelRetroOptions) error {
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
	if opts.RetroID == "" {
		return fmt.Errorf("missing retro id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing retro source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing retro title")
	}
	return nil
}

func findOrCreateChannelRetroIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRetroOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel retro issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelRetroMatches(issue.Body, opts.RetroID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelRetroIssueTitle(opts), RenderChannelRetroIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel retro issue: %w", err)
	}
	return issue, true, false, nil
}

func channelRetroIssueTitle(opts ChannelRetroOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.RetroID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel retro: " + title
}

func channelRetroMatches(body, retroID string) bool {
	return HasChannelRetroMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`retro_id="%s"`, escapeMarkerValue(cleanChannelRetroID(retroID))))
}

func cleanChannelRetroID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelRetroID(ev Event, channel, threadID, sourceMessageID, title, wentWell, roughEdges, next string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, wentWell, roughEdges, next}, "|")
	return fmt.Sprintf("retro-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRetroNotifyMessageID(ev Event, retroID string) string {
	seed := strings.Join([]string{eventID(ev), retroID}, "|")
	return fmt.Sprintf("gitclaw-channel-retro-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelRetroNotificationBody(opts ChannelRetroOptions, retroIssueNumber int, retroIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel retro recorded.\n\n")
	if retroIssueNumber > 0 {
		fmt.Fprintf(&b, "Retro: #%d\n", retroIssueNumber)
	}
	if retroIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", retroIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
