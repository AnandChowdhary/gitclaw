package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelStoryDiceOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	StoryDiceID       string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelStoryDiceResult struct {
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
	StoryDiceIDHash string
	ThemeHash       string
	NoteHash        string
	RollHash        string
	SeedHash        string
	BodyHash        string
	DieCount        int
}

type ChannelStoryDiceActionRequest struct {
	Options             ChannelStoryDiceOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoStoryDiceID     bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	StoryDiceIDHash     string
	ThemeSHA            string
	ThemeBytes          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	RollSHA             string
	SeedSHA             string
	StoryDiceCount      int
	NotificationBodySHA string
}

type ChannelStoryDiceRoll struct {
	Dice    []ChannelStoryDie
	SeedSHA string
}

type ChannelStoryDie struct {
	Label string
	Value string
}

type channelStoryDieDeck struct {
	Label  string
	Values []string
}

func IsChannelStoryDiceActionRequest(ev Event, cfg Config) bool {
	return isChannelStoryDiceActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelStoryDiceActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "story-dice", "storydice", "plot-dice", "prompt-dice", "scene-dice", "story-card", "plot-card", "improv", "riff":
		return true
	default:
		return false
	}
}

func BuildChannelStoryDiceActionRequest(ev Event, cfg Config) (ChannelStoryDiceActionRequest, error) {
	fields, trailing, ok := channelStoryDiceActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelStoryDiceActionRequest{}, fmt.Errorf("missing channel story dice command")
	}
	req := ChannelStoryDiceActionRequest{
		Options: ChannelStoryDiceOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             defaultChannelStoryDiceThemeForSubcommand(fields[1]),
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
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
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--story-dice-id", "--storydice-id", "--plot-dice-id", "--prompt-dice-id", "--scene-dice-id", "--riff-id", "--id":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StoryDiceID = cleanChannelStoryDiceID(fields[i+1])
			i++
		case "--theme", "--for", "--about":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelStoryDiceActionRequest{}, fmt.Errorf("unknown channel story dice argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelStoryDiceIssueTargetIfPresent(ev, &req)
	if err := applyChannelStoryDicePositionals(&req, positional); err != nil {
		return ChannelStoryDiceActionRequest{}, err
	}
	if err := applyChannelStoryDiceIssueTarget(ev, &req); err != nil {
		return ChannelStoryDiceActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelStoryDiceTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelStoryDiceSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StoryDiceID) == "" {
		req.Options.StoryDiceID = autoChannelStoryDiceID(ev, req.Options)
		req.AutoStoryDiceID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelStoryDiceNotifyMessageID(ev, req.Options.StoryDiceID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelStoryDiceOptions(req.Options)
	if err := validateChannelStoryDiceActionRequestOptions(req.Options); err != nil {
		return ChannelStoryDiceActionRequest{}, err
	}
	roll := BuildChannelStoryDiceRoll(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StoryDiceIDHash = shortDocumentHash(req.Options.StoryDiceID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.RollSHA = shortDocumentHash(channelStoryDiceRollText(roll.Dice))
	req.SeedSHA = roll.SeedSHA
	req.StoryDiceCount = len(roll.Dice)
	req.NotificationBodySHA = shortDocumentHash(renderChannelStoryDiceNotificationBody(req.Options))
	return req, nil
}

func RunChannelStoryDice(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelStoryDiceOptions) (ChannelStoryDiceResult, error) {
	opts = normalizeChannelStoryDiceOptions(opts)
	var err error
	opts, err = applyChannelStoryDiceRoute(cfg, opts)
	if err != nil {
		return ChannelStoryDiceResult{}, err
	}
	if err := validateChannelStoryDiceOptions(opts); err != nil {
		return ChannelStoryDiceResult{}, err
	}
	roll := BuildChannelStoryDiceRoll(opts)
	body := renderChannelStoryDiceNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelStoryDiceResult{}, fmt.Errorf("queue channel story dice notification: %w", err)
	}
	return ChannelStoryDiceResult{
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
		StoryDiceIDHash: shortDocumentHash(opts.StoryDiceID),
		ThemeHash:       shortDocumentHash(opts.Theme),
		NoteHash:        shortDocumentHash(opts.Note),
		RollHash:        shortDocumentHash(channelStoryDiceRollText(roll.Dice)),
		SeedHash:        roll.SeedSHA,
		BodyHash:        shortDocumentHash(body),
		DieCount:        len(roll.Dice),
	}, nil
}

func RenderChannelStoryDiceActionReport(ev Event, req ChannelStoryDiceActionRequest, result ChannelStoryDiceResult) string {
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
	storyDiceIDHash := result.StoryDiceIDHash
	if storyDiceIDHash == "" {
		storyDiceIDHash = req.StoryDiceIDHash
	}
	themeHash := result.ThemeHash
	if themeHash == "" {
		themeHash = req.ThemeSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	rollHash := result.RollHash
	if rollHash == "" {
		rollHash = req.RollSHA
	}
	seedHash := result.SeedHash
	if seedHash == "" {
		seedHash = req.SeedSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	dieCount := result.DieCount
	if dieCount == 0 {
		dieCount = req.StoryDiceCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Story Dice Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_story_dice_status: `%s`\n", status)
	fmt.Fprintf(&b, "- story_dice_mode: `%s`\n", "deterministic-channel-prompt-dice")
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
	fmt.Fprintf(&b, "- story_dice_id_sha256_12: `%s`\n", noneIfEmpty(storyDiceIDHash))
	fmt.Fprintf(&b, "- story_dice_id_auto: `%t`\n", req.AutoStoryDiceID)
	fmt.Fprintf(&b, "- story_dice_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- story_dice_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- story_dice_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- story_dice_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- story_dice_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- story_dice_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- story_dice_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- story_dice_roll_sha256_12: `%s`\n", noneIfEmpty(rollHash))
	fmt.Fprintf(&b, "- story_dice_count: `%d`\n", dieCount)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_randomness_used: `%t`\n", false)
	fmt.Fprintf(&b, "- media_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_story_dice_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_story_dice_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_story_dice_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_story_dice_roll_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_story_dice_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel story-dice card on the canonical channel issue. This is a tiny chat-native activity: Slack or Telegram can get a bounded, deterministic prompt-dice card while the source receipt keeps thread ids, message ids, story-dice ids, themes, notes, and rolled prompts out of band. The action does not call a model, use external randomness, generate media, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read story-dice cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent story-dice cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate story-dice cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelStoryDiceActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelStoryDiceActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelStoryDiceIssueTarget(ev Event, req *ChannelStoryDiceActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel story dice requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelStoryDiceIssueTargetIfPresent(ev Event, req *ChannelStoryDiceActionRequest) {
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

func applyChannelStoryDicePositionals(req *ChannelStoryDiceActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Theme == "" || req.Options.Theme == "general" || req.Options.Theme == "fun" {
				req.Options.Theme = value
				continue
			}
			return fmt.Errorf("unexpected channel story dice argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Theme == "" || req.Options.Theme == "general" || req.Options.Theme == "fun" {
			req.Options.Theme = value
			continue
		}
		return fmt.Errorf("unexpected channel story dice argument %q", value)
	}
	return nil
}

func normalizeChannelStoryDiceOptions(opts ChannelStoryDiceOptions) ChannelStoryDiceOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StoryDiceID = cleanChannelStoryDiceID(opts.StoryDiceID)
	opts.Theme = cleanChannelStoryDiceTheme(opts.Theme)
	opts.Note = cleanChannelStoryDiceNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelStoryDiceRoute(cfg Config, opts ChannelStoryDiceOptions) (ChannelStoryDiceOptions, error) {
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
		Body:      "GitClaw channel story dice.",
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

func validateChannelStoryDiceOptions(opts ChannelStoryDiceOptions) error {
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
	if opts.StoryDiceID == "" {
		return fmt.Errorf("missing story dice id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing story dice theme")
	}
	return nil
}

func validateChannelStoryDiceActionRequestOptions(opts ChannelStoryDiceOptions) error {
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
	if opts.StoryDiceID == "" {
		return fmt.Errorf("missing story dice id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing story dice theme")
	}
	return nil
}

func cleanChannelStoryDiceID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelStoryDiceTheme(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "general"
	}
	if len(value) > 32 {
		value = strings.Trim(value[:32], "-")
	}
	return value
}

func cleanChannelStoryDiceNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelStoryDiceTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelStoryDiceTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelStoryDiceNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelStoryDiceTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelStoryDiceThemeForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "improv", "riff", "story-card", "plot-card":
		return "fun"
	default:
		return "general"
	}
}

func autoChannelStoryDiceSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-story-dice-source-%s", eventID(ev))
}

func autoChannelStoryDiceID(ev Event, opts ChannelStoryDiceOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return fmt.Sprintf("story-dice-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelStoryDiceNotifyMessageID(ev Event, storyDiceID string) string {
	seed := strings.Join([]string{eventID(ev), storyDiceID}, "|")
	return fmt.Sprintf("gitclaw-channel-story-dice-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func BuildChannelStoryDiceRoll(opts ChannelStoryDiceOptions) ChannelStoryDiceRoll {
	opts = normalizeChannelStoryDiceOptions(opts)
	deck := channelStoryDiceDeckForTheme(opts.Theme)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.StoryDiceID, opts.Theme, opts.Note}, "|")
	dice := make([]ChannelStoryDie, 0, len(deck))
	for _, die := range deck {
		if len(die.Values) == 0 {
			continue
		}
		index := deterministicChannelChooseIndex(seed+"|"+die.Label, len(die.Values))
		dice = append(dice, ChannelStoryDie{Label: die.Label, Value: die.Values[index]})
	}
	return ChannelStoryDiceRoll{Dice: dice, SeedSHA: shortDocumentHash(seed)}
}

func renderChannelStoryDiceNotificationBody(opts ChannelStoryDiceOptions) string {
	opts = normalizeChannelStoryDiceOptions(opts)
	roll := BuildChannelStoryDiceRoll(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel story dice.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	b.WriteString("Dice:\n")
	for i, die := range roll.Dice {
		fmt.Fprintf(&b, "%d. %s: %s\n", i+1, die.Label, die.Value)
	}
	fmt.Fprintf(&b, "Story dice hash: %s\n", shortDocumentHash(channelStoryDiceRollText(roll.Dice)))
	fmt.Fprintf(&b, "Seed hash: %s\n", roll.SeedSHA)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nStory dice source: deterministic GitHub channel action seed.\n")
	b.WriteString("Story dice deck: bounded static GitClaw prompt deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Media generation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelStoryDiceRollText(dice []ChannelStoryDie) string {
	var lines []string
	for _, die := range dice {
		lines = append(lines, die.Label+": "+die.Value)
	}
	return strings.Join(lines, "\n")
}

func channelStoryDiceDeckForTheme(theme string) []channelStoryDieDeck {
	switch cleanChannelStoryDiceTheme(theme) {
	case "launch", "ship", "release":
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"a release window just opened", "a checklist turns green", "a quiet deploy room gathers"}},
			{Label: "Constraint", Values: []string{"only one reversible move is allowed", "the rollback note must fit in one sentence", "the riskiest assumption needs a name"}},
			{Label: "Move", Values: []string{"pick the smallest proof", "ask for the missing owner", "write the handoff before the button"}},
			{Label: "Button", Values: []string{"ship when the receipt is boring", "leave the next operator a map", "make the success path visible"}},
		}
	case "tools", "tool":
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"a tool request arrives from chat", "a schema is almost right", "a dry run finds a safer path"}},
			{Label: "Constraint", Values: []string{"approval must stay explicit", "raw output cannot enter the receipt", "the tool name has to resolve cleanly"}},
			{Label: "Move", Values: []string{"rehearse the call first", "quote the capability boundary", "turn the request into a reviewed plan"}},
			{Label: "Button", Values: []string{"no execution without a visible trail", "small inputs make sharp tools", "the best tool result is easy to audit"}},
		}
	case "soul", "memory":
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"a durable note asks to be remembered", "a context file casts a long shadow", "a past thread offers a useful clue"}},
			{Label: "Constraint", Values: []string{"body text stays out of the receipt", "authority must be reviewed", "memory should be smaller than the moment"}},
			{Label: "Move", Values: []string{"name the source before the lesson", "turn the feeling into one stable rule", "keep only what changes future behavior"}},
			{Label: "Button", Values: []string{"remember less, use it better", "warmth still needs provenance", "the thread can come home"}},
		}
	case "backups", "backup", "restore":
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"the backup branch has a clue", "a restore request pauses at the gate", "an old issue becomes a map"}},
			{Label: "Constraint", Values: []string{"no payload body leaves the archive", "restore stays rehearsal-only", "freshness must be proven before confidence"}},
			{Label: "Move", Values: []string{"search metadata first", "compare hashes before replay", "ask which issue is the anchor"}},
			{Label: "Button", Values: []string{"practice recovery before needing it", "a backup is useful when it can explain itself", "the way back should be boring"}},
		}
	case "retro", "review":
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"the room exhales after a push", "a decision wants a receipt", "one rough edge keeps blinking"}},
			{Label: "Constraint", Values: []string{"blame is not a useful artifact", "only one lesson gets promoted", "the next action must be testable"}},
			{Label: "Move", Values: []string{"write the thing to repeat", "write the thing to stop", "turn the surprise into a guardrail"}},
			{Label: "Button", Values: []string{"keep the lesson close to the work", "the retro ends with a runnable next step", "clarity is the souvenir"}},
		}
	case "fun", "play":
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"a tiny side quest appears", "the thread asks for a plot twist", "a deterministic die lands with style"}},
			{Label: "Constraint", Values: []string{"no magic, only hashes", "the joke still needs a receipt", "the card must fit in chat"}},
			{Label: "Move", Values: []string{"pick the surprising next command", "turn the bit into a task", "invite one human to choose"}},
			{Label: "Button", Values: []string{"fun is better when it is inspectable", "tiny rituals keep channels alive", "the outbox carries the sparkle"}},
		}
	default:
		return []channelStoryDieDeck{
			{Label: "Opening", Values: []string{"the channel thread wakes up", "a GitHub issue becomes the room", "a message crosses the bridge"}},
			{Label: "Constraint", Values: []string{"no server gets to be required", "the source receipt stays body-free", "the provider card carries the visible copy"}},
			{Label: "Move", Values: []string{"queue one small follow-up", "choose the next safe command", "leave a hash trail behind"}},
			{Label: "Button", Values: []string{"conversation continues where it started", "GitHub remembers the path", "the next move is small enough to run"}},
		}
	}
}
