package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRoomPulseOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	PulseID           string
	Focus             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRoomPulseResult struct {
	Notification      ChannelSendResult
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	MessageHash       string
	NotifyHash        string
	PulseIDHash       string
	FocusHash         string
	NoteHash          string
	BodyHash          string
	SnapshotHash      string
	NextStepHash      string
	TotalComments     int
	ChannelMessages   int
	AssistantTurns    int
	OutboundCards     int
	StatusCards       int
	ActivitySignals   int
	ErrorMarkers      int
	UserCommands      int
	OtherComments     int
	LastObservedKind  string
	PulseState        string
	CommentBodiesRead bool
}

type ChannelRoomPulseActionRequest struct {
	Options             ChannelRoomPulseOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoPulseID         bool
	TargetFromIssue     bool
	FocusSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	PulseIDHash         string
	FocusSHA            string
	FocusBytes          int
	FocusTerms          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	NotificationBodySHA string
	SnapshotHash        string
	NextStepHash        string
	TotalComments       int
	ChannelMessages     int
	AssistantTurns      int
	OutboundCards       int
	StatusCards         int
	ActivitySignals     int
	ErrorMarkers        int
	UserCommands        int
	OtherComments       int
	LastObservedKind    string
	PulseState          string
}

type channelRoomPulseSnapshot struct {
	TotalComments    int
	ChannelMessages  int
	AssistantTurns   int
	OutboundCards    int
	StatusCards      int
	ActivitySignals  int
	ErrorMarkers     int
	UserCommands     int
	OtherComments    int
	LastObservedKind string
	PulseState       string
	NextStep         string
	SnapshotHash     string
	NextStepHash     string
}

func IsChannelRoomPulseActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelRoomPulseActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelRoomPulseActionFields(fields)
}

func isChannelRoomPulseActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelRoomPulseSubcommand(fields[1]) {
	case "room-pulse", "roompulse", "thread-pulse", "threadpulse", "room-check", "thread-check", "room-state", "thread-state", "room-beat", "thread-beat":
		return true
	default:
		return false
	}
}

func BuildChannelRoomPulseActionRequest(ev Event, cfg Config) (ChannelRoomPulseActionRequest, error) {
	fields, trailing, ok := channelRoomPulseActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRoomPulseActionRequest{}, fmt.Errorf("missing channel room pulse command")
	}
	req := ChannelRoomPulseActionRequest{
		Options: ChannelRoomPulseOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Focus:             "general",
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelRoomPulseSubcommand(fields[1]),
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
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--pulse-id", "--room-pulse-id", "--room-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PulseID = cleanChannelRoomPulseID(fields[i+1])
			i++
		case "--focus", "--lane", "--for":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Focus = fields[i+1]
			req.FocusSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRoomPulseActionRequest{}, fmt.Errorf("unknown channel room pulse argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelRoomPulseIssueTargetIfPresent(ev, &req)
	if err := applyChannelRoomPulsePositionals(&req, positional); err != nil {
		return ChannelRoomPulseActionRequest{}, err
	}
	if err := applyChannelRoomPulseIssueTarget(ev, &req); err != nil {
		return ChannelRoomPulseActionRequest{}, err
	}
	if req.FocusSource == "" {
		req.FocusSource = "default"
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelRoomPulseTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelRoomPulseSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.PulseID) == "" {
		req.Options.PulseID = autoChannelRoomPulseID(ev, req.Options)
		req.AutoPulseID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRoomPulseNotifyMessageID(ev, req.Options.PulseID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRoomPulseOptions(req.Options)
	if err := validateChannelRoomPulseActionRequestOptions(req.Options); err != nil {
		return ChannelRoomPulseActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.PulseIDHash = shortDocumentHash(req.Options.PulseID)
	req.FocusSHA = shortDocumentHash(req.Options.Focus)
	req.FocusBytes = len(req.Options.Focus)
	req.FocusTerms = len(memorySearchTerms(req.Options.Focus))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	return req, nil
}

func RunChannelRoomPulse(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelRoomPulseActionRequest) (ChannelRoomPulseResult, error) {
	opts := normalizeChannelRoomPulseOptions(req.Options)
	var err error
	opts, err = applyChannelRoomPulseRoute(cfg, opts)
	if err != nil {
		return ChannelRoomPulseResult{}, err
	}
	if err := validateChannelRoomPulseOptions(opts); err != nil {
		return ChannelRoomPulseResult{}, err
	}
	comments, err := github.ListIssueComments(ctx, opts.Repo, opts.SourceIssueNumber)
	if err != nil {
		return ChannelRoomPulseResult{}, fmt.Errorf("list channel room comments: %w", err)
	}
	snapshot := buildChannelRoomPulseSnapshot(comments, opts)
	body := renderChannelRoomPulseNotificationBody(opts, snapshot)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelRoomPulseResult{}, fmt.Errorf("queue channel room pulse notification: %w", err)
	}
	result := ChannelRoomPulseResult{
		Notification:      notification,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
		PulseIDHash:       shortDocumentHash(opts.PulseID),
		FocusHash:         shortDocumentHash(opts.Focus),
		NoteHash:          shortDocumentHash(opts.Note),
		BodyHash:          shortDocumentHash(body),
		CommentBodiesRead: true,
	}
	result.applySnapshot(snapshot)
	return result, nil
}

func RenderChannelRoomPulseActionReport(ev Event, req ChannelRoomPulseActionRequest, result ChannelRoomPulseResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := firstNonEmpty(result.Channel, req.Options.Channel)
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	pulseIDHash := firstNonEmpty(result.PulseIDHash, req.PulseIDHash)
	focusHash := firstNonEmpty(result.FocusHash, req.FocusSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	snapshotHash := firstNonEmpty(result.SnapshotHash, req.SnapshotHash)
	nextStepHash := firstNonEmpty(result.NextStepHash, req.NextStepHash)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Room Pulse Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_room_pulse_status: `%s`\n", status)
	fmt.Fprintf(&b, "- room_pulse_mode: `%s`\n", "metadata-only-channel-presence")
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
	fmt.Fprintf(&b, "- room_pulse_id_sha256_12: `%s`\n", noneIfEmpty(pulseIDHash))
	fmt.Fprintf(&b, "- room_pulse_id_auto: `%t`\n", req.AutoPulseID)
	fmt.Fprintf(&b, "- room_pulse_focus_sha256_12: `%s`\n", noneIfEmpty(focusHash))
	fmt.Fprintf(&b, "- room_pulse_focus_bytes: `%d`\n", req.FocusBytes)
	fmt.Fprintf(&b, "- room_pulse_focus_terms: `%d`\n", req.FocusTerms)
	fmt.Fprintf(&b, "- room_pulse_focus_source: `%s`\n", noneIfEmpty(req.FocusSource))
	fmt.Fprintf(&b, "- room_pulse_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- room_pulse_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- room_pulse_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- room_pulse_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- room_pulse_state: `%s`\n", firstNonEmpty(result.PulseState, req.PulseState, "unknown"))
	fmt.Fprintf(&b, "- room_pulse_total_comments: `%d`\n", nonzeroOrReq(result.TotalComments, req.TotalComments))
	fmt.Fprintf(&b, "- room_pulse_channel_messages: `%d`\n", result.ChannelMessages)
	fmt.Fprintf(&b, "- room_pulse_assistant_turns: `%d`\n", result.AssistantTurns)
	fmt.Fprintf(&b, "- room_pulse_outbound_cards: `%d`\n", result.OutboundCards)
	fmt.Fprintf(&b, "- room_pulse_status_cards: `%d`\n", result.StatusCards)
	fmt.Fprintf(&b, "- room_pulse_activity_signals: `%d`\n", result.ActivitySignals)
	fmt.Fprintf(&b, "- room_pulse_error_markers: `%d`\n", result.ErrorMarkers)
	fmt.Fprintf(&b, "- room_pulse_user_commands: `%d`\n", result.UserCommands)
	fmt.Fprintf(&b, "- room_pulse_other_comments: `%d`\n", result.OtherComments)
	fmt.Fprintf(&b, "- room_pulse_last_observed_kind: `%s`\n", firstNonEmpty(result.LastObservedKind, req.LastObservedKind, "unknown"))
	fmt.Fprintf(&b, "- room_pulse_snapshot_sha256_12: `%s`\n", noneIfEmpty(snapshotHash))
	fmt.Fprintf(&b, "- room_pulse_next_step_sha256_12: `%s`\n", noneIfEmpty(nextStepHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- comment_body_read_performed: `%t`\n", result.CommentBodiesRead)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- task_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_room_pulse_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_room_pulse_focus_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_room_pulse_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_room_pulse_next_step_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_room_pulse_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing room pulse on the canonical channel issue. This is a chat-native heartbeat for Slack or Telegram threads: it counts safe GitClaw markers, reports whether the room looks active, and suggests a next command, but it does not summarize raw conversation text, call a model, call provider APIs, create tasks or reminders, edit workflows, or mutate the repository. The source receipt keeps thread ids, message ids, pulse ids, focus values, notes, suggested step text, issue bodies, comment bodies, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read room-pulse cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent room-pulse cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate room-pulse cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelRoomPulseActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRoomPulseActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRoomPulseIssueTarget(ev Event, req *ChannelRoomPulseActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel room pulse requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelRoomPulseIssueTargetIfPresent(ev Event, req *ChannelRoomPulseActionRequest) {
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

func applyChannelRoomPulsePositionals(req *ChannelRoomPulseActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Focus == "" || req.Options.Focus == "general" {
				req.Options.Focus = value
				req.FocusSource = "positional"
				continue
			}
			return fmt.Errorf("unexpected channel room pulse argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Focus == "" || req.Options.Focus == "general" {
			req.Options.Focus = value
			req.FocusSource = "positional"
			continue
		}
		return fmt.Errorf("unexpected channel room pulse argument %q", value)
	}
	return nil
}

func normalizeChannelRoomPulseOptions(opts ChannelRoomPulseOptions) ChannelRoomPulseOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.PulseID = cleanChannelRoomPulseID(opts.PulseID)
	opts.Focus = cleanChannelRoomPulseFocus(opts.Focus)
	opts.Note = cleanChannelRoomPulseNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Focus == "" {
		opts.Focus = "general"
	}
	return opts
}

func applyChannelRoomPulseRoute(cfg Config, opts ChannelRoomPulseOptions) (ChannelRoomPulseOptions, error) {
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
		Body:      "GitClaw channel room pulse.",
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

func validateChannelRoomPulseOptions(opts ChannelRoomPulseOptions) error {
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
	if opts.PulseID == "" {
		return fmt.Errorf("missing room pulse id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue number")
	}
	return nil
}

func validateChannelRoomPulseActionRequestOptions(opts ChannelRoomPulseOptions) error {
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
	if opts.PulseID == "" {
		return fmt.Errorf("missing room pulse id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue number")
	}
	return nil
}

func cleanChannelRoomPulseSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelRoomPulseID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelRoomPulseFocus(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if len(value) > 48 {
		value = strings.Trim(value[:48], "-")
	}
	return value
}

func cleanChannelRoomPulseNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelRoomPulseTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelRoomPulseNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelRoomPulseSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-room-pulse-source-%s", eventID(ev))
}

func autoChannelRoomPulseID(ev Event, opts ChannelRoomPulseOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Focus, opts.Note}, "|")
	return cleanChannelRoomPulseID(fmt.Sprintf("room-pulse-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelRoomPulseNotifyMessageID(ev Event, pulseID string) string {
	seed := strings.Join([]string{eventID(ev), pulseID}, "|")
	return fmt.Sprintf("gitclaw-channel-room-pulse-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func buildChannelRoomPulseSnapshot(comments []Comment, opts ChannelRoomPulseOptions) channelRoomPulseSnapshot {
	snapshot := channelRoomPulseSnapshot{TotalComments: len(comments), LastObservedKind: "none"}
	for _, comment := range comments {
		kind := channelRoomPulseCommentKind(comment.Body)
		snapshot.LastObservedKind = kind
		switch kind {
		case "channel-message":
			snapshot.ChannelMessages++
		case "assistant-turn":
			snapshot.AssistantTurns++
		case "channel-outbound":
			snapshot.OutboundCards++
		case "channel-status":
			snapshot.StatusCards++
		case "channel-activity":
			snapshot.ActivitySignals++
		case "error-marker":
			snapshot.ErrorMarkers++
		case "user-command":
			snapshot.UserCommands++
		default:
			snapshot.OtherComments++
		}
	}
	snapshot.PulseState = channelRoomPulseState(snapshot)
	snapshot.NextStep = channelRoomPulseNextStep(snapshot, opts.Focus)
	snapshot.NextStepHash = shortDocumentHash(snapshot.NextStep)
	snapshot.SnapshotHash = shortDocumentHash(channelRoomPulseSnapshotManifest(opts, snapshot))
	return snapshot
}

func channelRoomPulseCommentKind(body string) string {
	switch {
	case strings.Contains(body, "<!-- gitclaw:error"):
		return "error-marker"
	case strings.Contains(body, "<!-- gitclaw:channel-message"):
		return "channel-message"
	case strings.Contains(body, "gitclaw:assistant-turn"):
		return "assistant-turn"
	case strings.Contains(body, "<!-- gitclaw:channel-outbound"):
		return "channel-outbound"
	case strings.Contains(body, "<!-- gitclaw:channel-status"):
		return "channel-status"
	case strings.Contains(body, "<!-- gitclaw:channel-activity"):
		return "channel-activity"
	case strings.Contains(body, "@gitclaw /channel"):
		return "user-command"
	default:
		return "other"
	}
}

func channelRoomPulseState(snapshot channelRoomPulseSnapshot) string {
	switch {
	case snapshot.ErrorMarkers > 0:
		return "needs-attention"
	case snapshot.OutboundCards > 0:
		return "ready-for-delivery"
	case snapshot.UserCommands > 0 && snapshot.AssistantTurns > 0:
		return "active"
	case snapshot.AssistantTurns == 0 && snapshot.ChannelMessages <= 1:
		return "warming-up"
	case snapshot.AssistantTurns >= 3:
		return "moving"
	default:
		return "steady"
	}
}

func channelRoomPulseNextStep(snapshot channelRoomPulseSnapshot, focus string) string {
	if snapshot.ErrorMarkers > 0 {
		return "/channels recovery-map issue --map-id <id> --message-id <id> --notify-message-id <id>"
	}
	switch cleanChannelRoomPulseFocus(focus) {
	case "handoff", "session":
		return "/channels handoff --id <handoff-id> --message-id <id> --notify-message-id <id>"
	case "skills", "skill":
		return "/channels skill-map repo-reader --map-id <id> --message-id <id> --notify-message-id <id>"
	case "tools", "tool-review":
		return "/channels tool-map search_files --map-id <id> --message-id <id> --notify-message-id <id>"
	case "fun", "spark":
		return "/channels spark --spark-id <id> --message-id <id> --notify-message-id <id>"
	default:
		if snapshot.OutboundCards > 0 {
			return "gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>"
		}
		return "/channels nudge <target> --nudge-id <id> --message-id <id> --notify-message-id <id>"
	}
}

func channelRoomPulseSnapshotManifest(opts ChannelRoomPulseOptions, snapshot channelRoomPulseSnapshot) string {
	return fmt.Sprintf(
		"focus=%s\ncounts=%d/%d/%d/%d/%d/%d/%d/%d/%d\nlast=%s\nstate=%s\nnext=%s\nnote=%s",
		shortDocumentHash(cleanChannelRoomPulseFocus(opts.Focus)),
		snapshot.TotalComments,
		snapshot.ChannelMessages,
		snapshot.AssistantTurns,
		snapshot.OutboundCards,
		snapshot.StatusCards,
		snapshot.ActivitySignals,
		snapshot.ErrorMarkers,
		snapshot.UserCommands,
		snapshot.OtherComments,
		snapshot.LastObservedKind,
		snapshot.PulseState,
		snapshot.NextStepHash,
		shortDocumentHash(opts.Note),
	)
}

func renderChannelRoomPulseNotificationBody(opts ChannelRoomPulseOptions, snapshot channelRoomPulseSnapshot) string {
	opts = normalizeChannelRoomPulseOptions(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel room pulse.\n\n")
	fmt.Fprintf(&b, "Pulse: %s\n", snapshot.PulseState)
	fmt.Fprintf(&b, "Focus: %s\n", opts.Focus)
	fmt.Fprintf(&b, "Comments observed: %d\n", snapshot.TotalComments)
	fmt.Fprintf(&b, "Mirrored channel messages: %d\n", snapshot.ChannelMessages)
	fmt.Fprintf(&b, "Assistant turns: %d\n", snapshot.AssistantTurns)
	fmt.Fprintf(&b, "Outbound cards: %d\n", snapshot.OutboundCards)
	fmt.Fprintf(&b, "Status cards: %d\n", snapshot.StatusCards)
	fmt.Fprintf(&b, "Activity signals: %d\n", snapshot.ActivitySignals)
	fmt.Fprintf(&b, "Error markers: %d\n", snapshot.ErrorMarkers)
	fmt.Fprintf(&b, "User commands: %d\n", snapshot.UserCommands)
	fmt.Fprintf(&b, "Other comments: %d\n", snapshot.OtherComments)
	fmt.Fprintf(&b, "Last observed kind: %s\n", snapshot.LastObservedKind)
	fmt.Fprintf(&b, "Suggested next step: `%s`\n", snapshot.NextStep)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	fmt.Fprintf(&b, "Room pulse hash: %s\n", snapshot.SnapshotHash)
	fmt.Fprintf(&b, "Suggested step hash: %s\n", snapshot.NextStepHash)
	b.WriteString("\nPulse source: GitHub channel issue metadata and GitClaw markers.\n")
	b.WriteString("Raw issue/comment bodies: not included.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Task/reminder creation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func (r *ChannelRoomPulseActionRequest) applySnapshot(snapshot channelRoomPulseSnapshot) {
	r.TotalComments = snapshot.TotalComments
	r.ChannelMessages = snapshot.ChannelMessages
	r.AssistantTurns = snapshot.AssistantTurns
	r.OutboundCards = snapshot.OutboundCards
	r.StatusCards = snapshot.StatusCards
	r.ActivitySignals = snapshot.ActivitySignals
	r.ErrorMarkers = snapshot.ErrorMarkers
	r.UserCommands = snapshot.UserCommands
	r.OtherComments = snapshot.OtherComments
	r.LastObservedKind = snapshot.LastObservedKind
	r.PulseState = snapshot.PulseState
	r.SnapshotHash = snapshot.SnapshotHash
	r.NextStepHash = snapshot.NextStepHash
	r.NotificationBodySHA = shortDocumentHash(renderChannelRoomPulseNotificationBody(r.Options, snapshot))
}

func (r *ChannelRoomPulseResult) applySnapshot(snapshot channelRoomPulseSnapshot) {
	r.TotalComments = snapshot.TotalComments
	r.ChannelMessages = snapshot.ChannelMessages
	r.AssistantTurns = snapshot.AssistantTurns
	r.OutboundCards = snapshot.OutboundCards
	r.StatusCards = snapshot.StatusCards
	r.ActivitySignals = snapshot.ActivitySignals
	r.ErrorMarkers = snapshot.ErrorMarkers
	r.UserCommands = snapshot.UserCommands
	r.OtherComments = snapshot.OtherComments
	r.LastObservedKind = snapshot.LastObservedKind
	r.PulseState = snapshot.PulseState
	r.SnapshotHash = snapshot.SnapshotHash
	r.NextStepHash = snapshot.NextStepHash
}
