package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelQuickRepliesOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ReplyID           string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelQuickRepliesResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	ReplyIDHash  string
	LaneHash     string
	NoteHash     string
	OptionsHash  string
	BodyHash     string
	OptionCount  int
}

type ChannelQuickRepliesActionRequest struct {
	Options             ChannelQuickRepliesOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoReplyID         bool
	TargetFromIssue     bool
	LaneSource          string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ReplyIDHash         string
	LaneSHA             string
	LaneBytes           int
	LaneTerms           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	OptionsSHA          string
	OptionCount         int
	NotificationBodySHA string
}

type channelQuickReplyOption struct {
	Label   string
	Command string
	Reason  string
}

func IsChannelQuickRepliesActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelQuickRepliesActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelQuickRepliesActionFields(fields)
}

func isChannelQuickRepliesActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelQuickRepliesSubcommand(fields[1]) {
	case "quick-replies", "quick-reply", "reply-options", "reply-chips", "chips", "suggest-replies", "suggested-replies", "reply-suggestions", "next-replies":
		return true
	default:
		return false
	}
}

func BuildChannelQuickRepliesActionRequest(ev Event, cfg Config) (ChannelQuickRepliesActionRequest, error) {
	fields, trailing, ok := channelQuickRepliesActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelQuickRepliesActionRequest{}, fmt.Errorf("missing channel quick replies command")
	}
	req := ChannelQuickRepliesActionRequest{
		Options: ChannelQuickRepliesOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Lane:              "general",
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelQuickRepliesSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--reply-id", "--quick-reply-id", "--quick-replies-id", "--chips-id", "--id":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ReplyID = cleanChannelQuickRepliesID(fields[i+1])
			i++
		case "--lane", "--focus", "--for":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			req.LaneSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelQuickRepliesActionRequest{}, fmt.Errorf("unknown channel quick replies argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelQuickRepliesIssueTargetIfPresent(ev, &req)
	if err := applyChannelQuickRepliesPositionals(&req, positional); err != nil {
		return ChannelQuickRepliesActionRequest{}, err
	}
	if req.LaneSource == "" {
		req.LaneSource = "default"
	}
	if err := applyChannelQuickRepliesIssueTarget(ev, &req); err != nil {
		return ChannelQuickRepliesActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelQuickRepliesTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelQuickRepliesSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ReplyID) == "" {
		req.Options.ReplyID = autoChannelQuickRepliesID(ev, req.Options)
		req.AutoReplyID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelQuickRepliesNotifyMessageID(ev, req.Options.ReplyID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelQuickRepliesOptions(req.Options)
	if err := validateChannelQuickRepliesActionRequestOptions(req.Options); err != nil {
		return ChannelQuickRepliesActionRequest{}, err
	}
	options := channelQuickRepliesForLane(req.Options.Lane)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ReplyIDHash = shortDocumentHash(req.Options.ReplyID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.LaneTerms = len(memorySearchTerms(req.Options.Lane))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.OptionsSHA = shortDocumentHash(channelQuickRepliesManifest(options))
	req.OptionCount = len(options)
	req.NotificationBodySHA = shortDocumentHash(renderChannelQuickRepliesNotificationBody(req.Options))
	return req, nil
}

func RunChannelQuickReplies(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelQuickRepliesOptions) (ChannelQuickRepliesResult, error) {
	opts = normalizeChannelQuickRepliesOptions(opts)
	var err error
	opts, err = applyChannelQuickRepliesRoute(cfg, opts)
	if err != nil {
		return ChannelQuickRepliesResult{}, err
	}
	if err := validateChannelQuickRepliesOptions(opts); err != nil {
		return ChannelQuickRepliesResult{}, err
	}
	options := channelQuickRepliesForLane(opts.Lane)
	body := renderChannelQuickRepliesNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelQuickRepliesResult{}, fmt.Errorf("queue channel quick replies notification: %w", err)
	}
	return ChannelQuickRepliesResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		ReplyIDHash:  shortDocumentHash(opts.ReplyID),
		LaneHash:     shortDocumentHash(opts.Lane),
		NoteHash:     shortDocumentHash(opts.Note),
		OptionsHash:  shortDocumentHash(channelQuickRepliesManifest(options)),
		BodyHash:     shortDocumentHash(body),
		OptionCount:  len(options),
	}, nil
}

func RenderChannelQuickRepliesActionReport(ev Event, req ChannelQuickRepliesActionRequest, result ChannelQuickRepliesResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := firstNonEmpty(result.Channel, req.Options.Channel)
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	replyIDHash := firstNonEmpty(result.ReplyIDHash, req.ReplyIDHash)
	laneHash := firstNonEmpty(result.LaneHash, req.LaneSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	optionsHash := firstNonEmpty(result.OptionsHash, req.OptionsSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	optionCount := result.OptionCount
	if optionCount == 0 {
		optionCount = req.OptionCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Quick Replies Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_quick_replies_status: `%s`\n", status)
	fmt.Fprintf(&b, "- quick_replies_mode: `%s`\n", "provider-facing-reply-chips")
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
	fmt.Fprintf(&b, "- source_message_id_auto: `%t`\n", req.AutoSourceMessageID)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- quick_replies_id_sha256_12: `%s`\n", noneIfEmpty(replyIDHash))
	fmt.Fprintf(&b, "- quick_replies_id_auto: `%t`\n", req.AutoReplyID)
	fmt.Fprintf(&b, "- quick_replies_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- quick_replies_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- quick_replies_lane_terms: `%d`\n", req.LaneTerms)
	fmt.Fprintf(&b, "- quick_replies_lane_source: `%s`\n", noneIfEmpty(req.LaneSource))
	fmt.Fprintf(&b, "- quick_replies_option_count: `%d`\n", optionCount)
	fmt.Fprintf(&b, "- quick_replies_options_sha256_12: `%s`\n", noneIfEmpty(optionsHash))
	fmt.Fprintf(&b, "- quick_replies_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- quick_replies_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- quick_replies_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- quick_replies_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- artifact_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- task_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quick_replies_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quick_replies_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quick_replies_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quick_replies_options_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_quick_replies_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued provider-facing quick replies on the canonical channel issue. This turns a Slack or Telegram thread into a small next-action surface: users can copy a suggested command, while this action itself does not execute commands, create artifacts, create tasks or reminders, install skills, execute tools, call a model, call provider APIs, edit workflows, mutate the repository, or deliver through provider APIs. The source receipt keeps thread ids, message ids, quick-reply ids, lanes, notes, option text, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read quick replies with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent quick replies with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate quick replies are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelQuickRepliesActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelQuickRepliesActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelQuickRepliesIssueTarget(ev Event, req *ChannelQuickRepliesActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel quick replies requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelQuickRepliesIssueTargetIfPresent(ev Event, req *ChannelQuickRepliesActionRequest) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
}

func applyChannelQuickRepliesPositionals(req *ChannelQuickRepliesActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Lane == "" || req.Options.Lane == "general" {
				req.Options.Lane = value
				req.LaneSource = "positional"
				continue
			}
			return fmt.Errorf("unexpected channel quick replies argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == "general" {
			req.Options.Lane = value
			req.LaneSource = "positional"
			continue
		}
		return fmt.Errorf("unexpected channel quick replies argument %q", value)
	}
	return nil
}

func normalizeChannelQuickRepliesOptions(opts ChannelQuickRepliesOptions) ChannelQuickRepliesOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ReplyID = cleanChannelQuickRepliesID(opts.ReplyID)
	opts.Lane = cleanChannelQuickRepliesLane(opts.Lane)
	opts.Note = cleanChannelQuickRepliesNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Lane == "" {
		opts.Lane = "general"
	}
	return opts
}

func applyChannelQuickRepliesRoute(cfg Config, opts ChannelQuickRepliesOptions) (ChannelQuickRepliesOptions, error) {
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
		Body:      "GitClaw channel quick replies.",
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

func validateChannelQuickRepliesOptions(opts ChannelQuickRepliesOptions) error {
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
	if opts.ReplyID == "" {
		return fmt.Errorf("missing quick replies id")
	}
	if len(channelQuickRepliesForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported quick replies lane %q", opts.Lane)
	}
	return nil
}

func validateChannelQuickRepliesActionRequestOptions(opts ChannelQuickRepliesOptions) error {
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
	if opts.ReplyID == "" {
		return fmt.Errorf("missing quick replies id")
	}
	if len(channelQuickRepliesForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported quick replies lane %q", opts.Lane)
	}
	return nil
}

func cleanChannelQuickRepliesSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelQuickRepliesID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelQuickRepliesLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "general", "default", "all", "core":
		return "general"
	case "handoff", "handoffs", "session", "sessions":
		return "handoff"
	case "skill", "skills":
		return "skills"
	case "tool", "tools", "tool-review", "review":
		return "tools"
	case "fun", "play", "presence":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelQuickRepliesNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelQuickRepliesTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelQuickRepliesNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelQuickRepliesSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-quick-replies-source-%s", eventID(ev))
}

func autoChannelQuickRepliesID(ev Event, opts ChannelQuickRepliesOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return cleanChannelQuickRepliesID(fmt.Sprintf("quick-replies-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelQuickRepliesNotifyMessageID(ev Event, replyID string) string {
	seed := strings.Join([]string{eventID(ev), replyID}, "|")
	return fmt.Sprintf("gitclaw-channel-quick-replies-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelQuickRepliesNotificationBody(opts ChannelQuickRepliesOptions) string {
	opts = normalizeChannelQuickRepliesOptions(opts)
	options := channelQuickRepliesForLane(opts.Lane)
	var b strings.Builder
	b.WriteString("GitClaw channel quick replies.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	b.WriteString("Reply chips:\n")
	for i, option := range options {
		fmt.Fprintf(&b, "%d. %s - `%s`\n", i+1, option.Label, option.Command)
		fmt.Fprintf(&b, "   %s\n", option.Reason)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Quick replies hash: %s\n", shortDocumentHash(channelQuickRepliesManifest(options)))
	b.WriteString("\nQuick replies source: GitHub channel action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Artifact issue creation: not performed by this action.\n")
	b.WriteString("Task/reminder creation: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelQuickRepliesForLane(lane string) []channelQuickReplyOption {
	switch cleanChannelQuickRepliesLane(lane) {
	case "handoff":
		return []channelQuickReplyOption{
			{Label: "Pulse", Command: "/channels room-pulse handoff --pulse-id <id> --message-id <id> --notify-message-id <id>", Reason: "check whether the room is ready for handoff"},
			{Label: "Handoff", Command: "/channels handoff --id <handoff-id> --message-id <id> --notify-message-id <id>", Reason: "open a session handoff from this channel thread"},
			{Label: "Nudge", Command: "/channels nudge release-captain --nudge-id <id> --message-id <id> --notify-message-id <id> --tone gentle", Reason: "ask the next human to look without creating a task"},
			{Label: "Mood", Command: "/channels mood focused --message-id <id> --notify-message-id <id> --intensity 4", Reason: "mark the room posture without invoking a model"},
		}
	case "skills":
		return []channelQuickReplyOption{
			{Label: "Skills", Command: "/channels skills --message-id <id> --notify-message-id <id>", Reason: "show the current skill surface"},
			{Label: "Skill map", Command: "/channels skill-map repo-reader --map-id <id> --message-id <id> --notify-message-id <id>", Reason: "show the safe skill path before proposing work"},
			{Label: "Propose", Command: "/channels propose-skill weekly-review --message-id <id> --notify-message-id <id>", Reason: "open a reviewed skill proposal issue"},
			{Label: "Note", Command: "/channels skill-note --skill repo-reader --note-id <id> --message-id <id> --notify-message-id <id>", Reason: "capture a channel-origin skill lesson"},
		}
	case "tools":
		return []channelQuickReplyOption{
			{Label: "Tools", Command: "/channels tools --message-id <id> --notify-message-id <id>", Reason: "show available tool contracts"},
			{Label: "Tool map", Command: "/channels tool-map search_files --map-id <id> --message-id <id> --notify-message-id <id>", Reason: "show the safe tool path before execution"},
			{Label: "Approval", Command: "/channels approval-plan search_files --id <id> --message-id <id> --notify-message-id <id>", Reason: "open a reviewed tool approval plan"},
			{Label: "Run request", Command: "/channels request-run search_files --id <id> --message-id <id> --notify-message-id <id>", Reason: "open a reviewed tool-run request"},
		}
	case "fun":
		return []channelQuickReplyOption{
			{Label: "Vibe", Command: "/channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>", Reason: "ask for a playful check-in"},
			{Label: "Spark", Command: "/channels spark --spark-id <id> --message-id <id> --notify-message-id <id>", Reason: "queue a bounded conversation starter"},
			{Label: "Sticker", Command: "/channels sticker confetti --sticker-id <id> --message-id <id> --notify-message-id <id>", Reason: "add a provider-facing flourish without media upload"},
			{Label: "Haiku", Command: "/channels haiku launch --haiku-id <id> --message-id <id> --notify-message-id <id>", Reason: "send a deterministic tiny poem"},
		}
	default:
		return []channelQuickReplyOption{
			{Label: "Pulse", Command: "/channels room-pulse general --pulse-id <id> --message-id <id> --notify-message-id <id>", Reason: "check the thread state"},
			{Label: "Reply", Command: "/channels reply --message-id <id>", Reason: "send a short provider-facing reply from this issue"},
			{Label: "Nudge", Command: "/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>", Reason: "ask for attention without creating a task"},
			{Label: "Palette", Command: "/channels palette all --palette-id <id> --message-id <id> --notify-message-id <id>", Reason: "open the broader channel command launcher"},
		}
	}
}

func channelQuickRepliesManifest(options []channelQuickReplyOption) string {
	lines := make([]string, 0, len(options))
	for _, option := range options {
		lines = append(lines, strings.Join([]string{option.Label, option.Command, option.Reason}, "|"))
	}
	return strings.Join(lines, "\n")
}
