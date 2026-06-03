package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelOpenLoopOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	LoopID            string
	Title             string
	Context           string
	NextStep          string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelOpenLoopResult struct {
	OpenLoopIssueNumber int
	OpenLoopIssueURL    string
	OpenLoopCreated     bool
	OpenLoopDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelOpenLoopActionRequest struct {
	Options             ChannelOpenLoopOptions
	Command             string
	Subcommand          string
	AutoLoopID          bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ContextSHA          string
	ContextBytes        int
	ContextLines        int
	NextStepSHA         string
	NextStepBytes       int
	NextStepLines       int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelOpenLoopActionRequest(ev Event, cfg Config) bool {
	return isChannelOpenLoopActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelOpenLoopActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "open-loop", "loop", "follow-up", "followup", "loose-end", "parking-lot":
		return true
	default:
		return false
	}
}

func BuildChannelOpenLoopActionRequest(ev Event, cfg Config) (ChannelOpenLoopActionRequest, error) {
	fields, trailing, ok := channelOpenLoopActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelOpenLoopActionRequest{}, fmt.Errorf("missing channel open loop command")
	}
	req := ChannelOpenLoopActionRequest{
		Options: ChannelOpenLoopOptions{
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
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--loop-id", "--open-loop-id", "--follow-up-id", "--followup-id", "--loose-end-id", "--id":
			if i+1 >= len(fields) {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.LoopID = cleanChannelLoopID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelOpenLoopActionRequest{}, fmt.Errorf("unknown channel open loop argument %q", field)
			}
			if req.Options.LoopID == "" {
				req.Options.LoopID = cleanChannelLoopID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelOpenLoopActionRequest{}, fmt.Errorf("unexpected channel open loop argument %q", field)
		}
	}
	if err := applyChannelOpenLoopIssueTarget(ev, &req); err != nil {
		return ChannelOpenLoopActionRequest{}, err
	}
	title, contextText, nextStep := parseChannelOpenLoopSections(trailing, ev)
	req.Options.Title = title
	req.Options.Context = contextText
	req.Options.NextStep = nextStep
	if strings.TrimSpace(req.Options.LoopID) == "" {
		req.Options.LoopID = autoChannelLoopID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, contextText, nextStep)
		req.AutoLoopID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelOpenLoopNotifyMessageID(ev, req.Options.LoopID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelOpenLoopOptions(req.Options)
	if err := validateChannelOpenLoopActionRequestOptions(req.Options); err != nil {
		return ChannelOpenLoopActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ContextSHA = shortDocumentHash(req.Options.Context)
	req.ContextBytes = len(req.Options.Context)
	req.ContextLines = lineCount(req.Options.Context)
	req.NextStepSHA = shortDocumentHash(req.Options.NextStep)
	req.NextStepBytes = len(req.Options.NextStep)
	req.NextStepLines = lineCount(req.Options.NextStep)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelOpenLoopNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelOpenLoop(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelOpenLoopOptions) (ChannelOpenLoopResult, error) {
	opts = normalizeChannelOpenLoopOptions(opts)
	var err error
	opts, err = applyChannelOpenLoopRoute(cfg, opts)
	if err != nil {
		return ChannelOpenLoopResult{}, err
	}
	if err := validateChannelOpenLoopOptions(opts); err != nil {
		return ChannelOpenLoopResult{}, err
	}
	openLoopIssue, created, duplicate, err := findOrCreateChannelOpenLoopIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelOpenLoopResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelOpenLoopNotificationBody(opts, openLoopIssue.Number, issueURL(opts.Repo, openLoopIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelOpenLoopResult{}, fmt.Errorf("queue channel open loop notification: %w", err)
	}
	return ChannelOpenLoopResult{
		OpenLoopIssueNumber: openLoopIssue.Number,
		OpenLoopIssueURL:    issueURL(opts.Repo, openLoopIssue.Number),
		OpenLoopCreated:     created,
		OpenLoopDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelOpenLoopActionReport(ev Event, req ChannelOpenLoopActionRequest, result ChannelOpenLoopResult) string {
	status := "saved"
	switch {
	case result.OpenLoopDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.OpenLoopDuplicate:
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
	b.WriteString("## GitClaw Channel Open Loop Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_open_loop_status: `%s`\n", status)
	fmt.Fprintf(&b, "- open_loop_issue: `#%d`\n", result.OpenLoopIssueNumber)
	fmt.Fprintf(&b, "- open_loop_issue_url: `%s`\n", result.OpenLoopIssueURL)
	fmt.Fprintf(&b, "- open_loop_issue_created: `%t`\n", result.OpenLoopCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.OpenLoopDuplicate)
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
	fmt.Fprintf(&b, "- open_loop_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.LoopID))
	fmt.Fprintf(&b, "- open_loop_id_auto: `%t`\n", req.AutoLoopID)
	fmt.Fprintf(&b, "- open_loop_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- open_loop_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- open_loop_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- open_loop_context_sha256_12: `%s`\n", req.ContextSHA)
	fmt.Fprintf(&b, "- open_loop_context_bytes: `%d`\n", req.ContextBytes)
	fmt.Fprintf(&b, "- open_loop_context_lines: `%d`\n", req.ContextLines)
	fmt.Fprintf(&b, "- open_loop_next_step_sha256_12: `%s`\n", req.NextStepSHA)
	fmt.Fprintf(&b, "- open_loop_next_step_bytes: `%d`\n", req.NextStepBytes)
	fmt.Fprintf(&b, "- open_loop_next_step_lines: `%d`\n", req.NextStepLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_open_loop_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_open_loop_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_open_loop_context_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_open_loop_next_step_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_open_loop_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel open loop as a durable GitHub issue, then queued a provider-facing link back to the original thread. The open-loop issue contains the human-readable context and next step; this source receipt keeps provider IDs, loop IDs, titles, context, next steps, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the open-loop link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent open-loop links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate open-loop issues are suppressed by `loop_id`; duplicate open-loop link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the open-loop issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelOpenLoopIssueBody(opts ChannelOpenLoopOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-open-loop loop_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.LoopID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel open loop.\n\n")
	fmt.Fprintf(&b, "- loop_id: %s\n", opts.LoopID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- open_loop_mode: github-issue-open-loop\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Open Loop\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Context) != "" {
		b.WriteString("\n\n## Context\n\n")
		b.WriteString(strings.TrimSpace(opts.Context))
	}
	if strings.TrimSpace(opts.NextStep) != "" {
		b.WriteString("\n\n## Next Step\n\n")
		b.WriteString(strings.TrimSpace(opts.NextStep))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel-origin open loop until it becomes a task, reminder, watch, memory proposal, or resolved note.")
	return strings.TrimSpace(b.String())
}

func channelOpenLoopActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelOpenLoopActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelOpenLoopIssueTarget(ev Event, req *ChannelOpenLoopActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel open loop requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelOpenLoopSections(trailing string, ev Event) (string, string, string) {
	defaultTitle := fmt.Sprintf("Channel open loop from issue #%d", ev.Issue.Number)
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	sections := map[string][]string{}
	current := ""
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t\r")
		if strings.TrimSpace(line) == "" && current == "" {
			continue
		}
		if section, value, ok := parseChannelOpenLoopSectionHeader(line); ok {
			current = section
			if value != "" {
				sections[current] = append(sections[current], value)
			}
			continue
		}
		if current == "" {
			current = "title"
		}
		sections[current] = append(sections[current], line)
	}
	title := strings.TrimSpace(strings.Join(sections["title"], "\n"))
	if title == "" {
		title = defaultTitle
	}
	contextText := strings.TrimSpace(strings.Join(sections["context"], "\n"))
	nextStep := strings.TrimSpace(strings.Join(sections["next_step"], "\n"))
	return title, contextText, nextStep
}

func parseChannelOpenLoopSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(strings.TrimSpace(line), ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelOpenLoopSectionName(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "context":
		return "context", strings.TrimSpace(value), true
	case "next_step":
		return "next_step", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelOpenLoopSectionName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "title", "summary", "topic", "question", "open loop", "open-loop":
		return "title"
	case "context", "notes", "background", "details", "why":
		return "context"
	case "next", "next step", "next-step", "next_step", "follow up", "follow-up", "followup", "action":
		return "next_step"
	default:
		return ""
	}
}

func normalizeChannelOpenLoopOptions(opts ChannelOpenLoopOptions) ChannelOpenLoopOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.LoopID = cleanChannelLoopID(opts.LoopID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Context = strings.TrimSpace(opts.Context)
	opts.NextStep = strings.TrimSpace(opts.NextStep)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelOpenLoopRoute(cfg Config, opts ChannelOpenLoopOptions) (ChannelOpenLoopOptions, error) {
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

func validateChannelOpenLoopOptions(opts ChannelOpenLoopOptions) error {
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
	if opts.LoopID == "" {
		return fmt.Errorf("missing open loop id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing open loop source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing open loop title")
	}
	return nil
}

func validateChannelOpenLoopActionRequestOptions(opts ChannelOpenLoopOptions) error {
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
	if opts.LoopID == "" {
		return fmt.Errorf("missing open loop id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing open loop source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing open loop title")
	}
	return nil
}

func findOrCreateChannelOpenLoopIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelOpenLoopOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel open loop issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelOpenLoopMatches(issue.Body, opts.LoopID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelOpenLoopIssueTitle(opts), RenderChannelOpenLoopIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel open loop issue: %w", err)
	}
	return issue, true, false, nil
}

func channelOpenLoopIssueTitle(opts ChannelOpenLoopOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.LoopID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel open loop: " + title
}

func channelOpenLoopMatches(body, openLoopID string) bool {
	return HasChannelOpenLoopMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`loop_id="%s"`, escapeMarkerValue(cleanChannelLoopID(openLoopID))))
}

func cleanChannelLoopID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelLoopID(ev Event, channel, threadID, sourceMessageID, title, contextText, nextStep string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, contextText, nextStep}, "|")
	return fmt.Sprintf("open-loop-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelOpenLoopNotifyMessageID(ev Event, openLoopID string) string {
	seed := strings.Join([]string{eventID(ev), openLoopID}, "|")
	return fmt.Sprintf("gitclaw-channel-open-loop-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelOpenLoopNotificationBody(opts ChannelOpenLoopOptions, openLoopIssueNumber int, openLoopIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel open loop captured.\n\n")
	if openLoopIssueNumber > 0 {
		fmt.Fprintf(&b, "Open loop: #%d\n", openLoopIssueNumber)
	}
	if openLoopIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", openLoopIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nSaved in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
