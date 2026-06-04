package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelArcadeOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ArcadeID          string
	Mode              string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelArcadeResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	ArcadeIDHash string
	ModeHash     string
	NoteHash     string
	BodyHash     string
	MoveHash     string
	MoveCount    int
}

type ChannelArcadeActionRequest struct {
	Options             ChannelArcadeOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoArcadeID        bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	ArcadeIDHash        string
	ModeSHA             string
	ModeBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	MoveSHA             string
	MoveCount           int
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type channelArcadeMove struct {
	Name    string
	State   string
	Signal  string
	Command string
}

func IsChannelArcadeActionRequest(ev Event, cfg Config) bool {
	return isChannelArcadeActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelArcadeActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelArcadeSubcommand(fields[1]) {
	case "arcade", "channel-arcade", "play-menu", "play-card", "game-menu", "mini-game", "mini-arcade", "tiny-game", "prompt-arcade", "fun-menu", "pick-a-move":
		return true
	default:
		return false
	}
}

func BuildChannelArcadeActionRequest(ev Event, cfg Config) (ChannelArcadeActionRequest, error) {
	fields, trailing, ok := channelArcadeActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelArcadeActionRequest{}, fmt.Errorf("missing channel arcade command")
	}
	req := ChannelArcadeActionRequest{
		Options: ChannelArcadeOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Mode:              defaultChannelArcadeModeForSubcommand(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelArcadeSubcommand(fields[1]),
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
				return ChannelArcadeActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--arcade-id", "--game-id", "--play-id", "--card-id", "--move-id", "--id":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ArcadeID = cleanChannelArcadeID(fields[i+1])
			i++
		case "--mode", "--theme", "--lane", "--for":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Mode = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelArcadeActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelArcadeActionRequest{}, fmt.Errorf("unknown channel arcade argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelArcadeIssueTargetIfPresent(ev, &req)
	if err := applyChannelArcadePositionals(&req, positional); err != nil {
		return ChannelArcadeActionRequest{}, err
	}
	if err := applyChannelArcadeIssueTarget(ev, &req); err != nil {
		return ChannelArcadeActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelArcadeTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelArcadeSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.ArcadeID) == "" {
		req.Options.ArcadeID = autoChannelArcadeID(ev, req.Options)
		req.AutoArcadeID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelArcadeNotifyMessageID(ev, req.Options.ArcadeID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelArcadeOptions(req.Options)
	if err := validateChannelArcadeActionRequestOptions(req.Options); err != nil {
		return ChannelArcadeActionRequest{}, err
	}
	moves := channelArcadeMovesForMode(req.Options.Mode)
	body := renderChannelArcadeNotificationBody(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.ArcadeIDHash = shortDocumentHash(req.Options.ArcadeID)
	req.ModeSHA = shortDocumentHash(req.Options.Mode)
	req.ModeBytes = len(req.Options.Mode)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.MoveSHA = shortDocumentHash(renderChannelArcadeMoveFingerprint(moves))
	req.MoveCount = len(moves)
	req.NotificationBodySHA = shortDocumentHash(body)
	req.NotificationBytes = len(body)
	req.NotificationLines = lineCount(body)
	return req, nil
}

func RunChannelArcade(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelArcadeOptions) (ChannelArcadeResult, error) {
	opts = normalizeChannelArcadeOptions(opts)
	var err error
	opts, err = applyChannelArcadeRoute(cfg, opts)
	if err != nil {
		return ChannelArcadeResult{}, err
	}
	if err := validateChannelArcadeOptions(opts); err != nil {
		return ChannelArcadeResult{}, err
	}
	body := renderChannelArcadeNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelArcadeResult{}, fmt.Errorf("queue channel arcade notification: %w", err)
	}
	moves := channelArcadeMovesForMode(opts.Mode)
	return ChannelArcadeResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		ArcadeIDHash: shortDocumentHash(opts.ArcadeID),
		ModeHash:     shortDocumentHash(opts.Mode),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
		MoveHash:     shortDocumentHash(renderChannelArcadeMoveFingerprint(moves)),
		MoveCount:    len(moves),
	}, nil
}

func RenderChannelArcadeActionReport(ev Event, req ChannelArcadeActionRequest, result ChannelArcadeResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	arcadeIDHash := firstNonEmpty(result.ArcadeIDHash, req.ArcadeIDHash)
	modeHash := firstNonEmpty(result.ModeHash, req.ModeSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	moveHash := firstNonEmpty(result.MoveHash, req.MoveSHA)
	moveCount := result.MoveCount
	if moveCount == 0 {
		moveCount = req.MoveCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Arcade Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_arcade_status: `%s`\n", status)
	fmt.Fprintf(&b, "- arcade_card_mode: `%s`\n", "bounded-channel-play-menu")
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
	fmt.Fprintf(&b, "- arcade_id_sha256_12: `%s`\n", noneIfEmpty(arcadeIDHash))
	fmt.Fprintf(&b, "- arcade_id_auto: `%t`\n", req.AutoArcadeID)
	fmt.Fprintf(&b, "- arcade_mode_sha256_12: `%s`\n", noneIfEmpty(modeHash))
	fmt.Fprintf(&b, "- arcade_mode_bytes: `%d`\n", req.ModeBytes)
	fmt.Fprintf(&b, "- arcade_move_count: `%d`\n", moveCount)
	fmt.Fprintf(&b, "- arcade_move_sha256_12: `%s`\n", noneIfEmpty(moveHash))
	fmt.Fprintf(&b, "- arcade_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- arcade_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- arcade_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- arcade_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", req.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", req.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- dynamic_play_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- game_state_persisted: `%t`\n", false)
	fmt.Fprintf(&b, "- score_tracking_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- schedule_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_arcade_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_arcade_mode_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_arcade_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_arcade_moves_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_arcade_commands_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_arcade_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing arcade card on the canonical channel issue. The card can show a bounded play menu and copyable next commands, while this source receipt keeps raw ids, mode names, notes, move text, command text, channel bodies, issue bodies, comments, prompts, tool outputs, and provider message ids out of band. The action uses a static deck and does not call a model, generate play text dynamically, use external randomness, persist game state, track scores, execute commands, install skills, execute tools, read backup payloads, read soul bodies, write memory, call provider APIs, mutate workflows, change policy, create schedules, perform provider delivery, or mutate the repository.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read arcade cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent arcade cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate arcade cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelArcadeActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelArcadeActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelArcadeIssueTarget(ev Event, req *ChannelArcadeActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel arcade requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelArcadeIssueTargetIfPresent(ev Event, req *ChannelArcadeActionRequest) {
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

func applyChannelArcadePositionals(req *ChannelArcadeActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	defaultMode := defaultChannelArcadeModeForSubcommand(req.Subcommand)
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Mode == "" || req.Options.Mode == defaultMode {
				req.Options.Mode = value
				continue
			}
			return fmt.Errorf("unexpected channel arcade argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Mode == "" || req.Options.Mode == defaultMode {
			req.Options.Mode = value
			continue
		}
		return fmt.Errorf("unexpected channel arcade argument %q", value)
	}
	return nil
}

func normalizeChannelArcadeOptions(opts ChannelArcadeOptions) ChannelArcadeOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ArcadeID = cleanChannelArcadeID(opts.ArcadeID)
	opts.Mode = cleanChannelArcadeMode(opts.Mode)
	opts.Note = cleanChannelArcadeNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelArcadeRoute(cfg Config, opts ChannelArcadeOptions) (ChannelArcadeOptions, error) {
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
		Body:      "GitClaw channel arcade.",
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

func validateChannelArcadeOptions(opts ChannelArcadeOptions) error {
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
	if opts.ArcadeID == "" {
		return fmt.Errorf("missing arcade id")
	}
	if !skillNamePattern.MatchString(opts.ArcadeID) {
		return fmt.Errorf("invalid arcade id %q", opts.ArcadeID)
	}
	if opts.Mode == "" {
		return fmt.Errorf("missing arcade mode")
	}
	if len(channelArcadeMovesForMode(opts.Mode)) == 0 {
		return fmt.Errorf("unsupported arcade mode %q", opts.Mode)
	}
	return nil
}

func validateChannelArcadeActionRequestOptions(opts ChannelArcadeOptions) error {
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
	if opts.ArcadeID == "" {
		return fmt.Errorf("missing arcade id")
	}
	if !skillNamePattern.MatchString(opts.ArcadeID) {
		return fmt.Errorf("invalid arcade id %q", opts.ArcadeID)
	}
	if opts.Mode == "" {
		return fmt.Errorf("missing arcade mode")
	}
	if len(channelArcadeMovesForMode(opts.Mode)) == 0 {
		return fmt.Errorf("unsupported arcade mode %q", opts.Mode)
	}
	return nil
}

func cleanChannelArcadeSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.NewReplacer("_", "-", " ", "-").Replace(value)
}

func cleanChannelArcadeID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelArcadeMode(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "fun", "play", "general", "default", "social":
		return "fun"
	case "warmup", "check-in", "checkin", "vibe", "starter", "icebreaker":
		return "warmup"
	case "story", "story-dice", "improv", "riff", "prompt":
		return "story"
	case "launch", "release", "ship", "shipping":
		return "launch"
	case "tools", "tool", "tool-use":
		return "tools"
	case "research", "openclaw", "hermes", "patterns":
		return "research"
	case "soul", "memory", "identity", "context":
		return "soul"
	case "backups", "backup", "recovery", "restore":
		return "backups"
	case "channels", "channel", "provider", "providers", "slack", "telegram":
		return "channels"
	default:
		return ""
	}
}

func cleanChannelArcadeNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelArcadeTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelArcadeTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelArcadeNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func shouldSkipChannelArcadeTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelArcadeModeForSubcommand(subcommand string) string {
	switch cleanChannelArcadeSubcommand(subcommand) {
	case "prompt-arcade":
		return "story"
	default:
		return "fun"
	}
}

func autoChannelArcadeSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-arcade-source-%s", eventID(ev))
}

func autoChannelArcadeID(ev Event, opts ChannelArcadeOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Mode, opts.Note}, "|")
	return fmt.Sprintf("arcade-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelArcadeNotifyMessageID(ev Event, arcadeID string) string {
	seed := strings.Join([]string{eventID(ev), arcadeID}, "|")
	return fmt.Sprintf("gitclaw-channel-arcade-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelArcadeNotificationBody(opts ChannelArcadeOptions) string {
	opts = normalizeChannelArcadeOptions(opts)
	moves := channelArcadeMovesForMode(opts.Mode)
	var b strings.Builder
	b.WriteString("GitClaw channel arcade.\n\n")
	fmt.Fprintf(&b, "Mode: %s\n", opts.Mode)
	fmt.Fprintf(&b, "Frame: %s\n", channelArcadeFrame(opts.Mode))
	b.WriteString("Moves:\n")
	for i, move := range moves {
		fmt.Fprintf(&b, "%d. %s [%s] - %s\n", i+1, move.Name, move.State, move.Signal)
		fmt.Fprintf(&b, "   Try: `%s`\n", move.Command)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nArcade hash: %s\n", shortDocumentHash(opts.ArcadeID))
	fmt.Fprintf(&b, "Move hash: %s\n", shortDocumentHash(renderChannelArcadeMoveFingerprint(moves)))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("Arcade persistence: advisory only; no score or game state changed.\n")
	b.WriteString("\nArcade source: bounded GitHub channel action deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Dynamic play generation: not performed by this action.\n")
	b.WriteString("External randomness: not used by this action.\n")
	b.WriteString("Game-state persistence: not performed by this action.\n")
	b.WriteString("Score tracking: not performed by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Memory write: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Policy mutation: not performed by this action.\n")
	b.WriteString("Schedule creation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelArcadeFrame(mode string) string {
	switch cleanChannelArcadeMode(mode) {
	case "warmup":
		return "Choose one low-ceremony way to make the next reply easier."
	case "story":
		return "Pick one prompt-game move and keep the result in the thread."
	case "launch":
		return "Keep the release room lively while the rollback path stays visible."
	case "tools":
		return "Make tool talk easier without running or approving anything."
	case "research":
		return "Turn OpenClaw/Hermes inspiration into one reviewable next move."
	case "soul":
		return "Make high-authority context feel approachable without editing it."
	case "backups":
		return "Practice recovery talk without reading payloads or restoring files."
	case "channels":
		return "Keep provider chat responsive while GitHub remains the durable ledger."
	default:
		return "Pick one bounded move; GitHub keeps the receipt."
	}
}

func channelArcadeMovesForMode(mode string) []channelArcadeMove {
	cleanedMode := cleanChannelArcadeMode(mode)
	if cleanedMode == "" {
		return nil
	}
	switch cleanedMode {
	case "warmup":
		return []channelArcadeMove{
			{Name: "Vibe check", State: "ready", Signal: "ask an easy entry question", Command: "@gitclaw /channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Quick replies", State: "ready", Signal: "offer reply chips without creating work", Command: "@gitclaw /channels quick-replies handoff --reply-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Spark", State: "ready", Signal: "turn the thread into one small experiment", Command: "@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Room pulse", State: "watch", Signal: "read safe markers before choosing a tone", Command: "@gitclaw /channels room-pulse handoff --pulse-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "story":
		return []channelArcadeMove{
			{Name: "Story dice", State: "ready", Signal: "roll a bounded prompt-card scene", Command: "@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Postcard", State: "ready", Signal: "send a tiny scene without media generation", Command: "@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Soundtrack", State: "ready", Signal: "queue a bounded three-track mood card", Command: "@gitclaw /channels soundtrack fun --soundtrack-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Haiku", State: "watch", Signal: "use a static line deck for a short thread beat", Command: "@gitclaw /channels haiku fun --haiku-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "launch":
		return []channelArcadeMove{
			{Name: "Toast", State: "ready", Signal: "mark the moment without durable kudos", Command: "@gitclaw /channels toast launch-ready --toast-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Timer", State: "ready", Signal: "set a visible timebox cue without scheduling", Command: "@gitclaw /channels timer 25m --timer-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Bingo", State: "ready", Signal: "make the release room scannable and playful", Command: "@gitclaw /channels bingo release --bingo-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Recovery", State: "watch", Signal: "check rollback posture before the bit gets loud", Command: "@gitclaw /channels recovery-map incident --map-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "tools":
		return []channelArcadeMove{
			{Name: "Tool search", State: "ready", Signal: "find a tool without schema dumps", Command: "@gitclaw /channels tool-search search_files --message-id <id> --notify-message-id <id>"},
			{Name: "Tool map", State: "watch", Signal: "sequence a tool request before execution", Command: "@gitclaw /channels tool-map search_files --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Tool spotlight", State: "ready", Signal: "show one safe tool contract", Command: "@gitclaw /channels tool-spotlight search_files --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Approval plan", State: "gated", Signal: "turn execution into a reviewed issue", Command: "@gitclaw /channels approval-plan search_files --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "research":
		return []channelArcadeMove{
			{Name: "Source spotlight", State: "ready", Signal: "review one OpenClaw/Hermes source", Command: "@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Research map", State: "ready", Signal: "translate one pattern into GitClaw shape", Command: "@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Cockpit", State: "watch", Signal: "scan research posture before choosing work", Command: "@gitclaw /channels cockpit research --cockpit-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Mission control", State: "watch", Signal: "keep the next move review-first", Command: "@gitclaw /channels mission-control research --mission-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "soul":
		return []channelArcadeMove{
			{Name: "Soul status", State: "ready", Signal: "show high-authority posture without raw bodies", Command: "@gitclaw /channels soul-status --message-id <id> --notify-message-id <id>"},
			{Name: "Soul spotlight", State: "ready", Signal: "inspect one safe context surface", Command: "@gitclaw /channels soul-spotlight identity --spotlight-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Memory status", State: "watch", Signal: "check durable memory without writes", Command: "@gitclaw /channels memory-status --message-id <id> --notify-message-id <id>"},
			{Name: "Soul proposal", State: "gated", Signal: "turn edits into a review issue", Command: "@gitclaw /channels propose-soul --target soul --id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "backups":
		return []channelArcadeMove{
			{Name: "Backup status", State: "ready", Signal: "show archive posture without payload reads", Command: "@gitclaw /channels backup --message-id <id> --notify-message-id <id>"},
			{Name: "Freshness", State: "watch", Signal: "check backup age from metadata", Command: "@gitclaw /channels backup-freshness --freshness-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Continuity", State: "watch", Signal: "look for gaps before recovery work", Command: "@gitclaw /channels backup-continuity --continuity-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Restore request", State: "gated", Signal: "make restore review explicit", Command: "@gitclaw /channels restore-request --request-id <id> --message-id <id> --notify-message-id <id>"},
		}
	case "channels":
		return []channelArcadeMove{
			{Name: "Availability", State: "ready", Signal: "show bridge posture without socket probes", Command: "@gitclaw /channels availability --message-id <id> --notify-message-id <id>"},
			{Name: "Palette", State: "ready", Signal: "show bounded channel shortcuts", Command: "@gitclaw /channels palette fun --palette-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Dock", State: "watch", Signal: "request route continuity without moving provider state", Command: "@gitclaw /channels dock design-review --dock-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Outbox", State: "gated", Signal: "keep delivery receipts in GitHub", Command: "gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>"},
		}
	default:
		return []channelArcadeMove{
			{Name: "Story dice", State: "ready", Signal: "start a tiny prompt-game card", Command: "@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Spark", State: "ready", Signal: "turn the room toward one experiment", Command: "@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Postcard", State: "ready", Signal: "send a small scene card", Command: "@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>"},
			{Name: "Cockpit", State: "watch", Signal: "switch from play to operator view", Command: "@gitclaw /channels cockpit fun --cockpit-id <id> --message-id <id> --notify-message-id <id>"},
		}
	}
}

func renderChannelArcadeMoveFingerprint(moves []channelArcadeMove) string {
	var parts []string
	for _, move := range moves {
		parts = append(parts, strings.Join([]string{move.Name, move.State, move.Signal, move.Command}, "|"))
	}
	return strings.Join(parts, "\n")
}
