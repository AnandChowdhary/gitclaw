package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelBingoOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	BingoID           string
	Theme             string
	GridSize          int
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBingoResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	BingoIDHash  string
	ThemeHash    string
	BoardHash    string
	NoteHash     string
	BodyHash     string
	CellCount    int
	GridSize     int
}

type ChannelBingoActionRequest struct {
	Options             ChannelBingoOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoBingoID         bool
	TargetFromIssue     bool
	ThemeSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	BingoIDHash         string
	ThemeSHA            string
	ThemeBytes          int
	BoardSHA            string
	BoardCellCount      int
	GridSize            int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	NotificationBodySHA string
}

func IsChannelBingoActionRequest(ev Event, cfg Config) bool {
	return isChannelBingoActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBingoActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "bingo", "channel-bingo", "icebreaker-bingo", "game-card":
		return true
	default:
		return false
	}
}

func BuildChannelBingoActionRequest(ev Event, cfg Config) (ChannelBingoActionRequest, error) {
	fields, trailing, ok := channelBingoActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBingoActionRequest{}, fmt.Errorf("missing channel bingo command")
	}
	req := ChannelBingoActionRequest{
		Options: ChannelBingoOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             "fun",
			GridSize:          3,
		},
		Command:     strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:  strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		ThemeSource: "default",
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
				return ChannelBingoActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--bingo-id", "--card-id", "--game-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.BingoID = cleanChannelBingoID(fields[i+1])
			i++
		case "--theme", "--lane", "--topic":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			req.ThemeSource = "flag"
			i++
		case "--size", "--grid-size":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			size, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelBingoActionRequest{}, fmt.Errorf("%s must be 3 or 4", field)
			}
			req.Options.GridSize = size
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBingoActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBingoActionRequest{}, fmt.Errorf("unknown channel bingo argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelBingoIssueTargetIfPresent(ev, &req)
	if err := applyChannelBingoPositionals(&req, positional); err != nil {
		return ChannelBingoActionRequest{}, err
	}
	if err := applyChannelBingoIssueTarget(ev, &req); err != nil {
		return ChannelBingoActionRequest{}, err
	}
	if note := parseChannelBingoTrailingNote(trailing); note != "" && req.Options.Note == "" {
		req.Options.Note = note
		req.NoteSource = "trailing-note"
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBingoSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.BingoID) == "" {
		req.Options.BingoID = autoChannelBingoID(ev, req.Options)
		req.AutoBingoID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBingoNotifyMessageID(ev, req.Options.BingoID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBingoOptions(req.Options)
	if err := validateChannelBingoActionRequestOptions(req.Options); err != nil {
		return ChannelBingoActionRequest{}, err
	}
	board := channelBingoBoard(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.BingoIDHash = shortDocumentHash(req.Options.BingoID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.BoardSHA = shortDocumentHash(strings.Join(board, "\n"))
	req.BoardCellCount = len(board)
	req.GridSize = req.Options.GridSize
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelBingoNotificationBody(req.Options))
	return req, nil
}

func RunChannelBingo(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBingoOptions) (ChannelBingoResult, error) {
	opts = normalizeChannelBingoOptions(opts)
	var err error
	opts, err = applyChannelBingoRoute(cfg, opts)
	if err != nil {
		return ChannelBingoResult{}, err
	}
	if err := validateChannelBingoOptions(opts); err != nil {
		return ChannelBingoResult{}, err
	}
	board := channelBingoBoard(opts)
	body := renderChannelBingoNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelBingoResult{}, fmt.Errorf("queue channel bingo notification: %w", err)
	}
	return ChannelBingoResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		BingoIDHash:  shortDocumentHash(opts.BingoID),
		ThemeHash:    shortDocumentHash(opts.Theme),
		BoardHash:    shortDocumentHash(strings.Join(board, "\n")),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
		CellCount:    len(board),
		GridSize:     opts.GridSize,
	}, nil
}

func RenderChannelBingoActionReport(ev Event, req ChannelBingoActionRequest, result ChannelBingoResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
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
	bingoIDHash := result.BingoIDHash
	if bingoIDHash == "" {
		bingoIDHash = req.BingoIDHash
	}
	themeHash := result.ThemeHash
	if themeHash == "" {
		themeHash = req.ThemeSHA
	}
	boardHash := result.BoardHash
	if boardHash == "" {
		boardHash = req.BoardSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	cellCount := result.CellCount
	if cellCount == 0 {
		cellCount = req.BoardCellCount
	}
	gridSize := result.GridSize
	if gridSize == 0 {
		gridSize = req.GridSize
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Bingo Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_bingo_status: `%s`\n", status)
	fmt.Fprintf(&b, "- bingo_mode: `%s`\n", "provider-facing-deterministic-mini-game")
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
	fmt.Fprintf(&b, "- bingo_id_sha256_12: `%s`\n", noneIfEmpty(bingoIDHash))
	fmt.Fprintf(&b, "- bingo_id_auto: `%t`\n", req.AutoBingoID)
	fmt.Fprintf(&b, "- bingo_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- bingo_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- bingo_theme_source: `%s`\n", noneIfEmpty(req.ThemeSource))
	fmt.Fprintf(&b, "- bingo_grid_size: `%d`\n", gridSize)
	fmt.Fprintf(&b, "- bingo_cell_count: `%d`\n", cellCount)
	fmt.Fprintf(&b, "- bingo_board_sha256_12: `%s`\n", noneIfEmpty(boardHash))
	fmt.Fprintf(&b, "- bingo_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- bingo_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- bingo_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- bingo_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- game_state_persisted: `%t`\n", false)
	fmt.Fprintf(&b, "- score_tracking_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bingo_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bingo_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bingo_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bingo_board_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_bingo_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel bingo card on the canonical channel issue. The readable board is delivered through the channel outbox, while this source receipt keeps thread ids, message ids, bingo ids, themes, notes, card cells, and channel bodies out of band. The action does not call a model, use external randomness, persist game state, track scores, mutate repository files, edit workflows, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read bingo cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent bingo cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate bingo cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelBingoActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBingoActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBingoIssueTarget(ev Event, req *ChannelBingoActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel bingo requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelBingoIssueTargetIfPresent(ev Event, req *ChannelBingoActionRequest) {
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

func applyChannelBingoPositionals(req *ChannelBingoActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.ThemeSource == "default" {
				req.Options.Theme = value
				req.ThemeSource = "positional-theme"
				continue
			}
			return fmt.Errorf("unexpected channel bingo argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.ThemeSource == "default" {
			req.Options.Theme = value
			req.ThemeSource = "positional-theme"
			continue
		}
		return fmt.Errorf("unexpected channel bingo argument %q", value)
	}
	return nil
}

func normalizeChannelBingoOptions(opts ChannelBingoOptions) ChannelBingoOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.BingoID = cleanChannelBingoID(opts.BingoID)
	opts.Theme = cleanChannelBingoTheme(opts.Theme)
	opts.Note = cleanChannelBingoNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.GridSize == 0 {
		opts.GridSize = 3
	}
	return opts
}

func applyChannelBingoRoute(cfg Config, opts ChannelBingoOptions) (ChannelBingoOptions, error) {
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
		Body:      "GitClaw channel bingo.",
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

func validateChannelBingoOptions(opts ChannelBingoOptions) error {
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
	if opts.BingoID == "" {
		return fmt.Errorf("missing bingo id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing bingo theme")
	}
	if opts.GridSize != 3 && opts.GridSize != 4 {
		return fmt.Errorf("channel bingo grid size must be 3 or 4")
	}
	return nil
}

func validateChannelBingoActionRequestOptions(opts ChannelBingoOptions) error {
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
	if opts.BingoID == "" {
		return fmt.Errorf("missing bingo id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing bingo theme")
	}
	if opts.GridSize != 3 && opts.GridSize != 4 {
		return fmt.Errorf("channel bingo grid size must be 3 or 4")
	}
	return nil
}

func cleanChannelBingoID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelBingoTheme(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "fun"
	}
	if len(value) > 48 {
		value = strings.Trim(value[:48], "-")
	}
	return value
}

func cleanChannelBingoNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelBingoTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") || strings.HasPrefix(lower, "prompt:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelBingoNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelBingoSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-bingo-source-%s", eventID(ev))
}

func autoChannelBingoID(ev Event, opts ChannelBingoOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, strconv.Itoa(opts.GridSize), opts.Note}, "|")
	return fmt.Sprintf("bingo-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBingoNotifyMessageID(ev Event, bingoID string) string {
	seed := strings.Join([]string{eventID(ev), bingoID}, "|")
	return fmt.Sprintf("gitclaw-channel-bingo-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelBingoBoard(opts ChannelBingoOptions) []string {
	deck := channelBingoDeck(opts.Theme)
	need := opts.GridSize * opts.GridSize
	if need <= 0 {
		need = 9
	}
	if len(deck) == 0 {
		return nil
	}
	seed := strings.Join([]string{opts.BingoID, opts.Theme, opts.SourceMessageID, opts.NotifyMessageID, strconv.Itoa(opts.GridSize)}, "|")
	offset := intFromHexPrefix(shortDocumentHash(seed)) % len(deck)
	cells := make([]string, 0, need)
	for i := 0; len(cells) < need; i++ {
		candidate := deck[(offset+i)%len(deck)]
		if containsString(cells, candidate) {
			continue
		}
		cells = append(cells, candidate)
	}
	if opts.GridSize%2 == 1 && len(cells) > 0 {
		cells[len(cells)/2] = "Free: keep it on GitHub"
	}
	return cells
}

func channelBingoDeck(theme string) []string {
	switch cleanChannelBingoTheme(theme) {
	case "release", "launch", "ship":
		return []string{
			"Green CI", "Rollback note named", "Owner tagged", "Docs touched",
			"Risk called out", "User impact clear", "Migration path checked", "Metric chosen",
			"Flag plan ready", "Reviewer unblocked", "Changelog line drafted", "Smoke test passed",
			"Dependency noted", "Fallback message ready", "Rollout window named", "Support note linked",
			"Screenshot attached", "Follow-up issue linked", "Acceptance criteria checked", "Known gap owned",
		}
	case "triage", "support", "inbox":
		return []string{
			"Repro steps found", "Priority named", "Owner tagged", "Duplicate checked",
			"Logs linked", "Customer impact clear", "Next action chosen", "Label fixed",
			"Blocking question asked", "Timeline noted", "Workaround found", "Regression suspected",
			"Scope narrowed", "Related issue linked", "Severity reviewed", "Assumption written",
			"Needs design", "Needs data", "Needs deploy", "Ready to close",
		}
	case "pairing", "focus", "standup":
		return []string{
			"One tiny next step", "Question answered", "Branch found", "Test named",
			"Diff explained", "Risk named", "Decision parked", "Owner clear",
			"Hidden dependency found", "Context linked", "Review requested", "Demo path found",
			"Scope cut", "Follow-up created", "Timer started", "Break taken",
			"Assumption checked", "Command pasted", "Result verified", "Handoff ready",
		}
	default:
		return []string{
			"Tiny win logged", "Mystery clarified", "Idea captured", "Question parked",
			"Next step named", "Context linked", "Assumption checked", "Risk noticed",
			"One thing deleted", "Note made readable", "Thread rescued", "Decision named",
			"Tool found", "Docs breadcrumb added", "Owner found", "Scope made smaller",
			"Example created", "Follow-up queued", "Confusing bit renamed", "Something got less weird",
		}
	}
}

func renderChannelBingoNotificationBody(opts ChannelBingoOptions) string {
	board := channelBingoBoard(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel bingo.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	fmt.Fprintf(&b, "Grid: %dx%d\n", opts.GridSize, opts.GridSize)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
	}
	b.WriteByte('\n')
	for i, cell := range board {
		fmt.Fprintf(&b, "- [ ] %s\n", cell)
		if opts.GridSize > 0 && (i+1)%opts.GridSize == 0 && i+1 < len(board) {
			b.WriteByte('\n')
		}
	}
	fmt.Fprintf(&b, "\nBingo hash: %s\n", shortDocumentHash(strings.Join(board, "\n")))
	fmt.Fprintf(&b, "Theme hash: %s\n", shortDocumentHash(opts.Theme))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nBingo source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used by this action.\n")
	b.WriteString("Game state: not persisted by this action.\n")
	b.WriteString("Score tracking: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func intFromHexPrefix(value string) int {
	value = strings.TrimSpace(value)
	if len(value) > 8 {
		value = value[:8]
	}
	n, err := strconv.ParseInt(value, 16, 32)
	if err != nil || n < 0 {
		return 0
	}
	return int(n)
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
