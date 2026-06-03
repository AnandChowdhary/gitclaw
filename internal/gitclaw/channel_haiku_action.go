package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelHaikuOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	HaikuID           string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelHaikuResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	HaikuIDHash  string
	ThemeHash    string
	NoteHash     string
	HaikuHash    string
	SeedHash     string
	BodyHash     string
	LineCount    int
}

type ChannelHaikuActionRequest struct {
	Options             ChannelHaikuOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoHaikuID         bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	HaikuIDHash         string
	ThemeSHA            string
	ThemeBytes          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	HaikuSHA            string
	SeedSHA             string
	HaikuLines          int
	NotificationBodySHA string
}

type ChannelHaikuPoem struct {
	Lines   []string
	SeedSHA string
}

func IsChannelHaikuActionRequest(ev Event, cfg Config) bool {
	return isChannelHaikuActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelHaikuActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "haiku", "poem", "poems", "tiny-poem", "micropoem", "micro-poem", "poem-card", "verse":
		return true
	default:
		return false
	}
}

func BuildChannelHaikuActionRequest(ev Event, cfg Config) (ChannelHaikuActionRequest, error) {
	fields, trailing, ok := channelHaikuActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelHaikuActionRequest{}, fmt.Errorf("missing channel haiku command")
	}
	req := ChannelHaikuActionRequest{
		Options: ChannelHaikuOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             defaultChannelHaikuThemeForSubcommand(fields[1]),
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
				return ChannelHaikuActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--haiku-id", "--poem-id", "--verse-id", "--id":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.HaikuID = cleanChannelHaikuID(fields[i+1])
			i++
		case "--theme", "--for", "--about":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelHaikuActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelHaikuActionRequest{}, fmt.Errorf("unknown channel haiku argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelHaikuIssueTargetIfPresent(ev, &req)
	if err := applyChannelHaikuPositionals(&req, positional); err != nil {
		return ChannelHaikuActionRequest{}, err
	}
	if err := applyChannelHaikuIssueTarget(ev, &req); err != nil {
		return ChannelHaikuActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelHaikuTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelHaikuSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.HaikuID) == "" {
		req.Options.HaikuID = autoChannelHaikuID(ev, req.Options)
		req.AutoHaikuID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelHaikuNotifyMessageID(ev, req.Options.HaikuID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelHaikuOptions(req.Options)
	if err := validateChannelHaikuActionRequestOptions(req.Options); err != nil {
		return ChannelHaikuActionRequest{}, err
	}
	poem := BuildChannelHaikuPoem(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.HaikuIDHash = shortDocumentHash(req.Options.HaikuID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.HaikuSHA = shortDocumentHash(channelHaikuPoemText(poem.Lines))
	req.SeedSHA = poem.SeedSHA
	req.HaikuLines = len(poem.Lines)
	req.NotificationBodySHA = shortDocumentHash(renderChannelHaikuNotificationBody(req.Options))
	return req, nil
}

func RunChannelHaiku(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelHaikuOptions) (ChannelHaikuResult, error) {
	opts = normalizeChannelHaikuOptions(opts)
	var err error
	opts, err = applyChannelHaikuRoute(cfg, opts)
	if err != nil {
		return ChannelHaikuResult{}, err
	}
	if err := validateChannelHaikuOptions(opts); err != nil {
		return ChannelHaikuResult{}, err
	}
	poem := BuildChannelHaikuPoem(opts)
	body := renderChannelHaikuNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelHaikuResult{}, fmt.Errorf("queue channel haiku notification: %w", err)
	}
	return ChannelHaikuResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		HaikuIDHash:  shortDocumentHash(opts.HaikuID),
		ThemeHash:    shortDocumentHash(opts.Theme),
		NoteHash:     shortDocumentHash(opts.Note),
		HaikuHash:    shortDocumentHash(channelHaikuPoemText(poem.Lines)),
		SeedHash:     poem.SeedSHA,
		BodyHash:     shortDocumentHash(body),
		LineCount:    len(poem.Lines),
	}, nil
}

func RenderChannelHaikuActionReport(ev Event, req ChannelHaikuActionRequest, result ChannelHaikuResult) string {
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
	haikuIDHash := result.HaikuIDHash
	if haikuIDHash == "" {
		haikuIDHash = req.HaikuIDHash
	}
	themeHash := result.ThemeHash
	if themeHash == "" {
		themeHash = req.ThemeSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	haikuHash := result.HaikuHash
	if haikuHash == "" {
		haikuHash = req.HaikuSHA
	}
	seedHash := result.SeedHash
	if seedHash == "" {
		seedHash = req.SeedSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	lineCount := result.LineCount
	if lineCount == 0 {
		lineCount = req.HaikuLines
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Haiku Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_haiku_status: `%s`\n", status)
	fmt.Fprintf(&b, "- haiku_mode: `%s`\n", "deterministic-channel-poem")
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
	fmt.Fprintf(&b, "- haiku_id_sha256_12: `%s`\n", noneIfEmpty(haikuIDHash))
	fmt.Fprintf(&b, "- haiku_id_auto: `%t`\n", req.AutoHaikuID)
	fmt.Fprintf(&b, "- haiku_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- haiku_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- haiku_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- haiku_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- haiku_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- haiku_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- haiku_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- haiku_sha256_12: `%s`\n", noneIfEmpty(haikuHash))
	fmt.Fprintf(&b, "- haiku_lines: `%d`\n", lineCount)
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
	fmt.Fprintf(&b, "- raw_haiku_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_haiku_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_haiku_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_haiku_lines_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_haiku_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel haiku on the canonical channel issue. This is a tiny chat-native signal: Slack or Telegram can get a bounded, deterministic poem card while the source receipt keeps thread ids, message ids, haiku ids, themes, notes, and poem lines out of band. The action does not call a model, use external randomness, generate media, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read haiku cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent haiku cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate haiku cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelHaikuActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelHaikuActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelHaikuIssueTarget(ev Event, req *ChannelHaikuActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel haiku requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelHaikuIssueTargetIfPresent(ev Event, req *ChannelHaikuActionRequest) {
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

func applyChannelHaikuPositionals(req *ChannelHaikuActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel haiku argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Theme == "" || req.Options.Theme == "general" || req.Options.Theme == "fun" {
			req.Options.Theme = value
			continue
		}
		return fmt.Errorf("unexpected channel haiku argument %q", value)
	}
	return nil
}

func normalizeChannelHaikuOptions(opts ChannelHaikuOptions) ChannelHaikuOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.HaikuID = cleanChannelHaikuID(opts.HaikuID)
	opts.Theme = cleanChannelHaikuTheme(opts.Theme)
	opts.Note = cleanChannelHaikuNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelHaikuRoute(cfg Config, opts ChannelHaikuOptions) (ChannelHaikuOptions, error) {
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
		Body:      "GitClaw channel haiku.",
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

func validateChannelHaikuOptions(opts ChannelHaikuOptions) error {
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
	if opts.HaikuID == "" {
		return fmt.Errorf("missing haiku id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing haiku theme")
	}
	return nil
}

func validateChannelHaikuActionRequestOptions(opts ChannelHaikuOptions) error {
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
	if opts.HaikuID == "" {
		return fmt.Errorf("missing haiku id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing haiku theme")
	}
	return nil
}

func cleanChannelHaikuID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelHaikuTheme(value string) string {
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

func cleanChannelHaikuNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelHaikuTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelHaikuTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelHaikuNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelHaikuTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelHaikuThemeForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "poem", "poems", "tiny-poem", "micropoem", "micro-poem", "poem-card", "verse":
		return "fun"
	default:
		return "general"
	}
}

func autoChannelHaikuSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-haiku-source-%s", eventID(ev))
}

func autoChannelHaikuID(ev Event, opts ChannelHaikuOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return fmt.Sprintf("haiku-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelHaikuNotifyMessageID(ev Event, haikuID string) string {
	seed := strings.Join([]string{eventID(ev), haikuID}, "|")
	return fmt.Sprintf("gitclaw-channel-haiku-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func BuildChannelHaikuPoem(opts ChannelHaikuOptions) ChannelHaikuPoem {
	opts = normalizeChannelHaikuOptions(opts)
	deck := channelHaikuDeckForTheme(opts.Theme)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.HaikuID, opts.Theme, opts.Note}, "|")
	index := deterministicChannelChooseIndex(seed, len(deck))
	lines := append([]string(nil), deck[index]...)
	return ChannelHaikuPoem{Lines: lines, SeedSHA: shortDocumentHash(seed)}
}

func renderChannelHaikuNotificationBody(opts ChannelHaikuOptions) string {
	opts = normalizeChannelHaikuOptions(opts)
	poem := BuildChannelHaikuPoem(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel haiku.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	b.WriteString("Haiku:\n")
	for _, line := range poem.Lines {
		fmt.Fprintf(&b, "%s\n", line)
	}
	fmt.Fprintf(&b, "Haiku hash: %s\n", shortDocumentHash(channelHaikuPoemText(poem.Lines)))
	fmt.Fprintf(&b, "Seed hash: %s\n", poem.SeedSHA)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nPoem source: deterministic GitHub channel action seed.\n")
	b.WriteString("Haiku deck: bounded static GitClaw line deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Media generation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelHaikuPoemText(lines []string) string {
	return strings.Join(lines, "\n")
}

func channelHaikuDeckForTheme(theme string) [][]string {
	switch cleanChannelHaikuTheme(theme) {
	case "launch", "ship", "release":
		return [][]string{
			{"green checks in the branch", "release notes fold into light", "ship with one clear path"},
			{"the tag waits softly", "small diffs breathe under review", "morning ships the thing"},
			{"build logs settle down", "one brave changelog line remains", "release finds the door"},
		}
	case "tools", "tool":
		return [][]string{
			{"quiet tools line up", "one bounded call names the work", "receipts hold the trail"},
			{"schemas catch the rain", "inputs stay small and reviewable", "outputs leave a hash"},
			{"a tool waits in light", "approval keeps the edge clean", "then the work can move"},
		}
	case "soul", "memory":
		return [][]string{
			{"old context hums low", "a small note finds its anchor", "memory stays kind"},
			{"soul files do not shout", "they hold the shape of the work", "quietly enough"},
			{"a durable line", "keeps the next thread from drifting", "home is a checksum"},
		}
	case "backups", "backup", "restore":
		return [][]string{
			{"backup branches sleep", "until the path home is named", "hashes keep watch"},
			{"snapshots under glass", "restore plans wait without haste", "nothing moves unseen"},
			{"old comments folded", "the backup index remembers", "recovery breathes"},
		}
	case "focus", "quiet":
		return [][]string{
			{"one tab remains open", "the noisy road grows smaller", "focus finds a chair"},
			{"quiet issue light", "the next reversible step", "waits under the cursor"},
			{"less motion, more shape", "the thread keeps its narrow beam", "work becomes visible"},
		}
	case "retro", "review":
		return [][]string{
			{"past turns leave footprints", "three notes gather on the wall", "next time starts clearer"},
			{"reviews cool the tea", "one sharp lesson keeps its name", "the sprint exhales"},
			{"what changed in the rain", "what stayed true beneath the logs", "what moves next with care"},
		}
	case "fun", "play":
		return [][]string{
			{"confetti in logs", "a tiny card crosses chat", "the thread grins softly"},
			{"dice beside the build", "oracles wink at the diff", "still the hashes hold"},
			{"small channel magic", "made entirely of comments", "somehow this is fine"},
		}
	default:
		return [][]string{
			{"small signals gather", "a thread finds its next clear shape", "github keeps the trail"},
			{"comments become paths", "actions wake and fold back in", "the issue remembers"},
			{"a quiet command", "crosses the channel boundary", "leaving only hashes"},
		}
	}
}
