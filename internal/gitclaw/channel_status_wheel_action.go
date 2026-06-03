package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelStatusWheelOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	WheelID           string
	Lane              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelStatusWheelResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	WheelIDHash  string
	LaneHash     string
	NoteHash     string
	DeckHash     string
	StatusHash   string
	ActionHash   string
	SeedHash     string
	BodyHash     string
	StatusCount  int
	StatusIndex  int
}

type ChannelStatusWheelActionRequest struct {
	Options             ChannelStatusWheelOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoWheelID         bool
	TargetFromIssue     bool
	LaneSource          string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	WheelIDHash         string
	LaneSHA             string
	LaneBytes           int
	LaneTerms           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	DeckSHA             string
	StatusSHA           string
	ActionSHA           string
	SeedSHA             string
	StatusCount         int
	StatusIndex         int
	NotificationBodySHA string
}

type channelStatusWheelEntry struct {
	Status string
	Action string
}

func IsChannelStatusWheelActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelStatusWheelActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelStatusWheelActionFields(fields)
}

func isChannelStatusWheelActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelStatusWheelSubcommand(fields[1]) {
	case "status-wheel", "statuswheel", "wheel", "spin", "status-spin", "signal-wheel", "signal-spin", "traffic-light", "traffic":
		return true
	default:
		return false
	}
}

func BuildChannelStatusWheelActionRequest(ev Event, cfg Config) (ChannelStatusWheelActionRequest, error) {
	fields, trailing, ok := channelStatusWheelActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelStatusWheelActionRequest{}, fmt.Errorf("missing channel status wheel command")
	}
	req := ChannelStatusWheelActionRequest{
		Options: ChannelStatusWheelOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Lane:              "focus",
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelStatusWheelSubcommand(fields[1]),
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
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--wheel-id", "--status-wheel-id", "--spin-id", "--signal-id", "--id":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.WheelID = cleanChannelStatusWheelID(fields[i+1])
			i++
		case "--lane", "--wheel", "--for", "--mode":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			req.LaneSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelStatusWheelActionRequest{}, fmt.Errorf("unknown channel status wheel argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelStatusWheelIssueTargetIfPresent(ev, &req)
	if err := applyChannelStatusWheelPositionals(&req, positional); err != nil {
		return ChannelStatusWheelActionRequest{}, err
	}
	if req.LaneSource == "" {
		req.LaneSource = "default"
	}
	if err := applyChannelStatusWheelIssueTarget(ev, &req); err != nil {
		return ChannelStatusWheelActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelStatusWheelTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelStatusWheelSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.WheelID) == "" {
		req.Options.WheelID = autoChannelStatusWheelID(ev, req.Options)
		req.AutoWheelID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelStatusWheelNotifyMessageID(ev, req.Options.WheelID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelStatusWheelOptions(req.Options)
	if err := validateChannelStatusWheelActionRequestOptions(req.Options); err != nil {
		return ChannelStatusWheelActionRequest{}, err
	}
	pick := buildChannelStatusWheelPick(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.WheelIDHash = shortDocumentHash(req.Options.WheelID)
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.LaneTerms = len(memorySearchTerms(req.Options.Lane))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.DeckSHA = pick.DeckHash
	req.StatusSHA = shortDocumentHash(pick.Entry.Status)
	req.ActionSHA = shortDocumentHash(pick.Entry.Action)
	req.SeedSHA = pick.SeedHash
	req.StatusCount = pick.Count
	req.StatusIndex = pick.Index
	req.NotificationBodySHA = shortDocumentHash(renderChannelStatusWheelNotificationBody(req.Options))
	return req, nil
}

func RunChannelStatusWheel(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelStatusWheelOptions) (ChannelStatusWheelResult, error) {
	opts = normalizeChannelStatusWheelOptions(opts)
	var err error
	opts, err = applyChannelStatusWheelRoute(cfg, opts)
	if err != nil {
		return ChannelStatusWheelResult{}, err
	}
	if err := validateChannelStatusWheelOptions(opts); err != nil {
		return ChannelStatusWheelResult{}, err
	}
	pick := buildChannelStatusWheelPick(opts)
	body := renderChannelStatusWheelNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelStatusWheelResult{}, fmt.Errorf("queue channel status wheel notification: %w", err)
	}
	return ChannelStatusWheelResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		WheelIDHash:  shortDocumentHash(opts.WheelID),
		LaneHash:     shortDocumentHash(opts.Lane),
		NoteHash:     shortDocumentHash(opts.Note),
		DeckHash:     pick.DeckHash,
		StatusHash:   shortDocumentHash(pick.Entry.Status),
		ActionHash:   shortDocumentHash(pick.Entry.Action),
		SeedHash:     pick.SeedHash,
		BodyHash:     shortDocumentHash(body),
		StatusCount:  pick.Count,
		StatusIndex:  pick.Index,
	}, nil
}

func RenderChannelStatusWheelActionReport(ev Event, req ChannelStatusWheelActionRequest, result ChannelStatusWheelResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := firstNonEmpty(result.Channel, req.Options.Channel)
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	wheelIDHash := firstNonEmpty(result.WheelIDHash, req.WheelIDHash)
	laneHash := firstNonEmpty(result.LaneHash, req.LaneSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	deckHash := firstNonEmpty(result.DeckHash, req.DeckSHA)
	statusHash := firstNonEmpty(result.StatusHash, req.StatusSHA)
	actionHash := firstNonEmpty(result.ActionHash, req.ActionSHA)
	seedHash := firstNonEmpty(result.SeedHash, req.SeedSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	statusCount := result.StatusCount
	if statusCount == 0 {
		statusCount = req.StatusCount
	}
	statusIndex := result.StatusIndex
	if statusIndex == 0 {
		statusIndex = req.StatusIndex
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Status Wheel Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_status_wheel_status: `%s`\n", status)
	fmt.Fprintf(&b, "- status_wheel_mode: `%s`\n", "deterministic-channel-status-wheel")
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
	fmt.Fprintf(&b, "- status_wheel_id_sha256_12: `%s`\n", noneIfEmpty(wheelIDHash))
	fmt.Fprintf(&b, "- status_wheel_id_auto: `%t`\n", req.AutoWheelID)
	fmt.Fprintf(&b, "- status_wheel_lane_sha256_12: `%s`\n", noneIfEmpty(laneHash))
	fmt.Fprintf(&b, "- status_wheel_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- status_wheel_lane_terms: `%d`\n", req.LaneTerms)
	fmt.Fprintf(&b, "- status_wheel_lane_source: `%s`\n", noneIfEmpty(req.LaneSource))
	fmt.Fprintf(&b, "- status_wheel_status_count: `%d`\n", statusCount)
	fmt.Fprintf(&b, "- status_wheel_status_index: `%d`\n", statusIndex)
	fmt.Fprintf(&b, "- status_wheel_deck_sha256_12: `%s`\n", noneIfEmpty(deckHash))
	fmt.Fprintf(&b, "- status_wheel_status_sha256_12: `%s`\n", noneIfEmpty(statusHash))
	fmt.Fprintf(&b, "- status_wheel_action_sha256_12: `%s`\n", noneIfEmpty(actionHash))
	fmt.Fprintf(&b, "- status_wheel_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- status_wheel_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- status_wheel_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- status_wheel_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- status_wheel_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- artifact_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- task_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- status_persistence_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_wheel_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_wheel_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_wheel_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_wheel_deck_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_wheel_status_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_wheel_action_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_status_wheel_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing status wheel card on the canonical channel issue. This gives a Slack or Telegram thread a small deterministic spin for team posture while the source receipt keeps thread ids, message ids, wheel ids, lanes, notes, deck text, selected statuses, selected actions, and channel bodies out of band. The action does not call a model, use external randomness, execute commands, create artifacts/tasks/reminders, install skills, execute tools, call provider APIs, persist status, edit workflows, mutate the repository, or deliver through provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read status wheel cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent status wheel cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate status wheel cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelStatusWheelActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelStatusWheelActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelStatusWheelIssueTarget(ev Event, req *ChannelStatusWheelActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel status wheel requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelStatusWheelIssueTargetIfPresent(ev Event, req *ChannelStatusWheelActionRequest) {
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

func applyChannelStatusWheelPositionals(req *ChannelStatusWheelActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Lane == "" || req.Options.Lane == "focus" {
				req.Options.Lane = value
				req.LaneSource = "positional"
				continue
			}
			return fmt.Errorf("unexpected channel status wheel argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Lane == "" || req.Options.Lane == "focus" {
			req.Options.Lane = value
			req.LaneSource = "positional"
			continue
		}
		return fmt.Errorf("unexpected channel status wheel argument %q", value)
	}
	return nil
}

func normalizeChannelStatusWheelOptions(opts ChannelStatusWheelOptions) ChannelStatusWheelOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.WheelID = cleanChannelStatusWheelID(opts.WheelID)
	opts.Lane = cleanChannelStatusWheelLane(opts.Lane)
	opts.Note = cleanChannelStatusWheelNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Lane == "" {
		opts.Lane = "focus"
	}
	return opts
}

func applyChannelStatusWheelRoute(cfg Config, opts ChannelStatusWheelOptions) (ChannelStatusWheelOptions, error) {
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
		Body:      "GitClaw channel status wheel.",
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

func validateChannelStatusWheelOptions(opts ChannelStatusWheelOptions) error {
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
	if opts.WheelID == "" {
		return fmt.Errorf("missing status wheel id")
	}
	if len(channelStatusWheelDeckForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported status wheel lane %q", opts.Lane)
	}
	return nil
}

func validateChannelStatusWheelActionRequestOptions(opts ChannelStatusWheelOptions) error {
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
	if opts.WheelID == "" {
		return fmt.Errorf("missing status wheel id")
	}
	if len(channelStatusWheelDeckForLane(opts.Lane)) == 0 {
		return fmt.Errorf("unsupported status wheel lane %q", opts.Lane)
	}
	return nil
}

func cleanChannelStatusWheelSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelStatusWheelID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelStatusWheelLane(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "focus", "focused", "default", "work":
		return "focus"
	case "release", "launch", "ship", "shipping":
		return "release"
	case "triage", "inbox", "sort", "sorting":
		return "triage"
	case "tool", "tools", "tool-review", "review":
		return "tools"
	case "soul", "souls", "identity", "authority", "context":
		return "soul"
	case "fun", "play", "presence", "social":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelStatusWheelNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelStatusWheelTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelStatusWheelNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelStatusWheelSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-status-wheel-source-%s", eventID(ev))
}

func autoChannelStatusWheelID(ev Event, opts ChannelStatusWheelOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Lane, opts.Note}, "|")
	return cleanChannelStatusWheelID(fmt.Sprintf("status-wheel-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelStatusWheelNotifyMessageID(ev Event, wheelID string) string {
	seed := strings.Join([]string{eventID(ev), wheelID}, "|")
	return fmt.Sprintf("gitclaw-channel-status-wheel-%s-%s", eventID(ev), shortDocumentHash(seed))
}

type channelStatusWheelPick struct {
	Entry    channelStatusWheelEntry
	DeckHash string
	SeedHash string
	Index    int
	Count    int
}

func buildChannelStatusWheelPick(opts ChannelStatusWheelOptions) channelStatusWheelPick {
	opts = normalizeChannelStatusWheelOptions(opts)
	deck := channelStatusWheelDeckForLane(opts.Lane)
	manifest := channelStatusWheelDeckManifest(deck)
	deckHash := shortDocumentHash(manifest)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.WheelID, opts.Lane, opts.Note, deckHash}, "|")
	index := deterministicChannelChooseIndex(seed, len(deck))
	entry := channelStatusWheelEntry{}
	if len(deck) > 0 {
		entry = deck[index]
	}
	return channelStatusWheelPick{
		Entry:    entry,
		DeckHash: deckHash,
		SeedHash: shortDocumentHash(seed),
		Index:    index + 1,
		Count:    len(deck),
	}
}

func renderChannelStatusWheelNotificationBody(opts ChannelStatusWheelOptions) string {
	opts = normalizeChannelStatusWheelOptions(opts)
	pick := buildChannelStatusWheelPick(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel status wheel.\n\n")
	fmt.Fprintf(&b, "Lane: %s\n", opts.Lane)
	fmt.Fprintf(&b, "Picked: #%d of %d\n", pick.Index, pick.Count)
	fmt.Fprintf(&b, "Status: %s\n", pick.Entry.Status)
	fmt.Fprintf(&b, "Micro-action: %s\n", pick.Entry.Action)
	fmt.Fprintf(&b, "Status hash: %s\n", shortDocumentHash(pick.Entry.Status))
	fmt.Fprintf(&b, "Action hash: %s\n", shortDocumentHash(pick.Entry.Action))
	fmt.Fprintf(&b, "Deck hash: %s\n", pick.DeckHash)
	fmt.Fprintf(&b, "Seed hash: %s\n", pick.SeedHash)
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nSelection source: deterministic GitHub channel action seed.\n")
	b.WriteString("Status persistence: advisory only; no durable channel state changed.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Artifact issue creation: not performed by this action.\n")
	b.WriteString("Task/reminder creation: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelStatusWheelDeckForLane(lane string) []channelStatusWheelEntry {
	switch cleanChannelStatusWheelLane(lane) {
	case "release":
		return []channelStatusWheelEntry{
			{Status: "green - ship the smallest reversible slice", Action: "name the rollback signal before posting the next update"},
			{Status: "yellow - one blocker needs an owner", Action: "ask the thread for the single owner and deadline"},
			{Status: "red - pause the release motion", Action: "turn the blocker into a reviewed GitHub issue"},
			{Status: "blue - evidence missing", Action: "request the screenshot, log, or run URL that would settle it"},
			{Status: "purple - announce carefully", Action: "draft the provider-facing status before changing anything"},
		}
	case "triage":
		return []channelStatusWheelEntry{
			{Status: "green - sort now", Action: "split task, context, and open-loop items in the next reply"},
			{Status: "yellow - unclear owner", Action: "ask who owns the next GitHub artifact"},
			{Status: "red - too broad", Action: "reduce the thread to one decision before creating work"},
			{Status: "blue - needs recall", Action: "use a search command before making a durable issue"},
			{Status: "purple - keep as context", Action: "save only the smallest useful breadcrumb"},
		}
	case "tools":
		return []channelStatusWheelEntry{
			{Status: "green - safe to plan", Action: "open an approval-plan before any tool execution"},
			{Status: "yellow - schema needed", Action: "ask for the exact tool input and expected output"},
			{Status: "red - do not execute", Action: "write a rehearsal issue instead of running the tool"},
			{Status: "blue - result would help", Action: "request the smallest read-only tool result"},
			{Status: "purple - policy unclear", Action: "check tool status before proposing the run"},
		}
	case "soul":
		return []channelStatusWheelEntry{
			{Status: "green - discuss context", Action: "name the relevant high-authority file without editing it"},
			{Status: "yellow - promotion risk", Action: "keep this as a proposal until reviewed"},
			{Status: "red - no soul write", Action: "capture the concern without mutating memory or soul"},
			{Status: "blue - ask for source", Action: "request the exact persistent context path or quote"},
			{Status: "purple - identity-sensitive", Action: "prefer a body-free status or risk card first"},
		}
	case "fun":
		return []channelStatusWheelEntry{
			{Status: "green - playful and useful", Action: "send one tiny celebratory card and keep the breadcrumb"},
			{Status: "yellow - needs focus", Action: "pair the joke with one concrete next step"},
			{Status: "red - too noisy", Action: "switch to a quiet status update"},
			{Status: "blue - invite someone in", Action: "use a warmup prompt instead of more commentary"},
			{Status: "purple - ritual moment", Action: "turn it into a reviewed ritual or pact if it repeats"},
		}
	default:
		return []channelStatusWheelEntry{
			{Status: "green - one crisp next move", Action: "ask the thread for the smallest useful reply"},
			{Status: "yellow - context missing", Action: "request the one fact that would unblock the next step"},
			{Status: "red - stop expanding scope", Action: "convert only the sharpest piece into GitHub work"},
			{Status: "blue - search before deciding", Action: "recover existing repo context before answering"},
			{Status: "purple - leave a breadcrumb", Action: "post the outcome in a durable issue comment"},
		}
	}
}

func channelStatusWheelDeckManifest(deck []channelStatusWheelEntry) string {
	lines := make([]string, 0, len(deck))
	for _, entry := range deck {
		lines = append(lines, strings.Join([]string{entry.Status, entry.Action}, "|"))
	}
	return strings.Join(lines, "\n")
}
