package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelSoundtrackOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	SoundtrackID      string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSoundtrackResult struct {
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
	SoundtrackIDHash string
	ThemeHash        string
	NoteHash         string
	SoundtrackHash   string
	SeedHash         string
	BodyHash         string
	TrackCount       int
}

type ChannelSoundtrackActionRequest struct {
	Options             ChannelSoundtrackOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoSoundtrackID    bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	SoundtrackIDHash    string
	ThemeSHA            string
	ThemeBytes          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	SoundtrackSHA       string
	SeedSHA             string
	SoundtrackTracks    int
	NotificationBodySHA string
}

type ChannelSoundtrackMix struct {
	Tracks  []string
	SeedSHA string
}

func IsChannelSoundtrackActionRequest(ev Event, cfg Config) bool {
	return isChannelSoundtrackActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoundtrackActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "soundtrack", "playlist", "mixtape", "mix", "walkout", "walkout-music", "soundtrack-card", "playlist-card", "mix-card":
		return true
	default:
		return false
	}
}

func BuildChannelSoundtrackActionRequest(ev Event, cfg Config) (ChannelSoundtrackActionRequest, error) {
	fields, trailing, ok := channelSoundtrackActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSoundtrackActionRequest{}, fmt.Errorf("missing channel soundtrack command")
	}
	req := ChannelSoundtrackActionRequest{
		Options: ChannelSoundtrackOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             defaultChannelSoundtrackThemeForSubcommand(fields[1]),
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
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--soundtrack-id", "--playlist-id", "--mixtape-id", "--mix-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SoundtrackID = cleanChannelSoundtrackID(fields[i+1])
			i++
		case "--theme", "--for", "--about":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoundtrackActionRequest{}, fmt.Errorf("unknown channel soundtrack argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelSoundtrackIssueTargetIfPresent(ev, &req)
	if err := applyChannelSoundtrackPositionals(&req, positional); err != nil {
		return ChannelSoundtrackActionRequest{}, err
	}
	if err := applyChannelSoundtrackIssueTarget(ev, &req); err != nil {
		return ChannelSoundtrackActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelSoundtrackTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSoundtrackSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SoundtrackID) == "" {
		req.Options.SoundtrackID = autoChannelSoundtrackID(ev, req.Options)
		req.AutoSoundtrackID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoundtrackNotifyMessageID(ev, req.Options.SoundtrackID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoundtrackOptions(req.Options)
	if err := validateChannelSoundtrackActionRequestOptions(req.Options); err != nil {
		return ChannelSoundtrackActionRequest{}, err
	}
	mix := BuildChannelSoundtrackMix(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.SoundtrackIDHash = shortDocumentHash(req.Options.SoundtrackID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.SoundtrackSHA = shortDocumentHash(channelSoundtrackMixText(mix.Tracks))
	req.SeedSHA = mix.SeedSHA
	req.SoundtrackTracks = len(mix.Tracks)
	req.NotificationBodySHA = shortDocumentHash(renderChannelSoundtrackNotificationBody(req.Options))
	return req, nil
}

func RunChannelSoundtrack(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelSoundtrackOptions) (ChannelSoundtrackResult, error) {
	opts = normalizeChannelSoundtrackOptions(opts)
	var err error
	opts, err = applyChannelSoundtrackRoute(cfg, opts)
	if err != nil {
		return ChannelSoundtrackResult{}, err
	}
	if err := validateChannelSoundtrackOptions(opts); err != nil {
		return ChannelSoundtrackResult{}, err
	}
	mix := BuildChannelSoundtrackMix(opts)
	body := renderChannelSoundtrackNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSoundtrackResult{}, fmt.Errorf("queue channel soundtrack notification: %w", err)
	}
	return ChannelSoundtrackResult{
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
		SoundtrackIDHash: shortDocumentHash(opts.SoundtrackID),
		ThemeHash:        shortDocumentHash(opts.Theme),
		NoteHash:         shortDocumentHash(opts.Note),
		SoundtrackHash:   shortDocumentHash(channelSoundtrackMixText(mix.Tracks)),
		SeedHash:         mix.SeedSHA,
		BodyHash:         shortDocumentHash(body),
		TrackCount:       len(mix.Tracks),
	}, nil
}

func RenderChannelSoundtrackActionReport(ev Event, req ChannelSoundtrackActionRequest, result ChannelSoundtrackResult) string {
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
	soundtrackIDHash := result.SoundtrackIDHash
	if soundtrackIDHash == "" {
		soundtrackIDHash = req.SoundtrackIDHash
	}
	themeHash := result.ThemeHash
	if themeHash == "" {
		themeHash = req.ThemeSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	soundtrackHash := result.SoundtrackHash
	if soundtrackHash == "" {
		soundtrackHash = req.SoundtrackSHA
	}
	seedHash := result.SeedHash
	if seedHash == "" {
		seedHash = req.SeedSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	trackCount := result.TrackCount
	if trackCount == 0 {
		trackCount = req.SoundtrackTracks
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Soundtrack Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soundtrack_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soundtrack_mode: `%s`\n", "deterministic-channel-mix")
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
	fmt.Fprintf(&b, "- soundtrack_id_sha256_12: `%s`\n", noneIfEmpty(soundtrackIDHash))
	fmt.Fprintf(&b, "- soundtrack_id_auto: `%t`\n", req.AutoSoundtrackID)
	fmt.Fprintf(&b, "- soundtrack_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- soundtrack_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- soundtrack_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- soundtrack_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- soundtrack_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- soundtrack_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- soundtrack_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- soundtrack_sha256_12: `%s`\n", noneIfEmpty(soundtrackHash))
	fmt.Fprintf(&b, "- soundtrack_track_count: `%d`\n", trackCount)
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
	fmt.Fprintf(&b, "- raw_soundtrack_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soundtrack_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soundtrack_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soundtrack_tracks_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soundtrack_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel soundtrack on the canonical channel issue. This is a tiny chat-native signal: Slack or Telegram can get a bounded, deterministic mix card while the source receipt keeps thread ids, message ids, soundtrack ids, themes, notes, and mix tracks out of band. The action does not call a model, use external randomness, generate media, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read soundtrack cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent soundtrack cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate soundtrack cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelSoundtrackActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSoundtrackActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSoundtrackIssueTarget(ev Event, req *ChannelSoundtrackActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soundtrack requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelSoundtrackIssueTargetIfPresent(ev Event, req *ChannelSoundtrackActionRequest) {
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

func applyChannelSoundtrackPositionals(req *ChannelSoundtrackActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel soundtrack argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Theme == "" || req.Options.Theme == "general" || req.Options.Theme == "fun" {
			req.Options.Theme = value
			continue
		}
		return fmt.Errorf("unexpected channel soundtrack argument %q", value)
	}
	return nil
}

func normalizeChannelSoundtrackOptions(opts ChannelSoundtrackOptions) ChannelSoundtrackOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SoundtrackID = cleanChannelSoundtrackID(opts.SoundtrackID)
	opts.Theme = cleanChannelSoundtrackTheme(opts.Theme)
	opts.Note = cleanChannelSoundtrackNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelSoundtrackRoute(cfg Config, opts ChannelSoundtrackOptions) (ChannelSoundtrackOptions, error) {
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
		Body:      "GitClaw channel soundtrack.",
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

func validateChannelSoundtrackOptions(opts ChannelSoundtrackOptions) error {
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
	if opts.SoundtrackID == "" {
		return fmt.Errorf("missing soundtrack id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing soundtrack theme")
	}
	return nil
}

func validateChannelSoundtrackActionRequestOptions(opts ChannelSoundtrackOptions) error {
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
	if opts.SoundtrackID == "" {
		return fmt.Errorf("missing soundtrack id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing soundtrack theme")
	}
	return nil
}

func cleanChannelSoundtrackID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSoundtrackTheme(value string) string {
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

func cleanChannelSoundtrackNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelSoundtrackTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelSoundtrackTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelSoundtrackNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelSoundtrackTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelSoundtrackThemeForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "playlist", "mixtape", "mix", "walkout", "walkout-music", "soundtrack-card", "playlist-card", "mix-card":
		return "fun"
	default:
		return "general"
	}
}

func autoChannelSoundtrackSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-soundtrack-source-%s", eventID(ev))
}

func autoChannelSoundtrackID(ev Event, opts ChannelSoundtrackOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return fmt.Sprintf("soundtrack-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelSoundtrackNotifyMessageID(ev Event, soundtrackID string) string {
	seed := strings.Join([]string{eventID(ev), soundtrackID}, "|")
	return fmt.Sprintf("gitclaw-channel-soundtrack-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func BuildChannelSoundtrackMix(opts ChannelSoundtrackOptions) ChannelSoundtrackMix {
	opts = normalizeChannelSoundtrackOptions(opts)
	deck := channelSoundtrackDeckForTheme(opts.Theme)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.SoundtrackID, opts.Theme, opts.Note}, "|")
	index := deterministicChannelChooseIndex(seed, len(deck))
	tracks := append([]string(nil), deck[index]...)
	return ChannelSoundtrackMix{Tracks: tracks, SeedSHA: shortDocumentHash(seed)}
}

func renderChannelSoundtrackNotificationBody(opts ChannelSoundtrackOptions) string {
	opts = normalizeChannelSoundtrackOptions(opts)
	mix := BuildChannelSoundtrackMix(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel soundtrack.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	b.WriteString("Tracks:\n")
	for i, track := range mix.Tracks {
		fmt.Fprintf(&b, "%d. %s\n", i+1, track)
	}
	fmt.Fprintf(&b, "Soundtrack hash: %s\n", shortDocumentHash(channelSoundtrackMixText(mix.Tracks)))
	fmt.Fprintf(&b, "Seed hash: %s\n", mix.SeedSHA)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nMix source: deterministic GitHub channel action seed.\n")
	b.WriteString("Soundtrack deck: bounded static GitClaw track deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Media generation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSoundtrackMixText(tracks []string) string {
	return strings.Join(tracks, "\n")
}

func channelSoundtrackDeckForTheme(theme string) [][]string {
	switch cleanChannelSoundtrackTheme(theme) {
	case "launch", "ship", "release":
		return [][]string{
			{"Green Check Walkout - branch is clean", "Changelog Lights - one path to ship", "Tag Door Opens - release is near"},
			{"Diffstat Pulse - small changes moving", "Review Room Echo - notes settle", "Deploy Window - steady hands"},
			{"Build Log Fadeout - warnings quiet", "Cutover Clock - one brave minute", "Ship It Softly - no drama needed"},
		}
	case "tools", "tool":
		return [][]string{
			{"Schema Bassline - inputs stay small", "Read-Only Riff - outputs leave hashes", "Approval Bridge - the edge stays clean"},
			{"Toolbox Intro - contracts line up", "Context Hook - one bounded call", "Receipt Outro - the trail remains"},
			{"Search Files Groove - find the phrase", "List Files Break - map the room", "Skill Index Coda - choose with care"},
		}
	case "soul", "memory":
		return [][]string{
			{"Anchor Tone - context stays kind", "Memory Loop - what mattered remains", "Checksum Home - the thread returns"},
			{"Soul File Lowlight - quiet authority", "Long-Term Chorus - durable enough", "Recall Fade - no raw bodies"},
			{"Profile Warmth - identity intact", "Context Lamp - find the next step", "Gentle Index - remember less better"},
		}
	case "backups", "backup", "restore":
		return [][]string{
			{"Backup Branch Lullaby - snapshots sleep", "Restore Plan Rise - nothing moves unseen", "Hash Watch - the way home glows"},
			{"Archive Click - comments folded", "Recovery Bass - indexes remember", "Rollback Room - hands off reset"},
			{"Freshness Meter - backups breathe", "Continuity Line - no gap unmarked", "Drill Mode - practice before motion"},
		}
	case "focus", "quiet":
		return [][]string{
			{"One Tab Theme - the road gets small", "Narrow Beam - one reversible step", "Cursor Room - work becomes visible"},
			{"Quiet Mode - fewer moving parts", "Deep Thread - hold the shape", "Done Label - breathe out"},
			{"Inbox Dimmer - signals only", "Focus Chair - sit with the next diff", "Small Win Loop - ship the slice"},
		}
	case "retro", "review":
		return [][]string{
			{"Footprint Loop - what changed", "Tea-Cooled Review - what stayed true", "Next-Time Hook - what moves with care"},
			{"Sprint Exhale - lessons keep names", "Sharp Note - one thing to carry", "Wall Card - make it visible"},
			{"After-Action Hum - no blame more signal", "Decision Echo - receipts remain", "Forward Beat - next turn clearer"},
		}
	case "fun", "play":
		return [][]string{
			{"Comment Arcade - tiny card crossing chat", "Hash Glitter - still reviewable", "Soft Launch Laugh - this counts"},
			{"Dice Beside The Build - deterministic edition", "Oracle Wink - choose the door", "Outbox Boogie - provider gets the beat"},
			{"Mini-Game Break - three moves only", "Thread Grin - comments can dance", "Receipts Hold - fun stays inspectable"},
		}
	default:
		return [][]string{
			{"Issue Room Tone - the thread wakes", "Action Runner Beat - folds back in", "Trail Kept - GitHub remembers"},
			{"Channel Bridge Theme - message to issue", "Outbox Pulse - provider reply queued", "Hash Curtain - raw text stays away"},
			{"Small Signal Intro - shape appears", "Workflow Dispatch - no server required", "Comment Return - conversation continues"},
		}
	}
}
