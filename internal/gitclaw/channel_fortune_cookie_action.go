package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelFortuneCookieOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	CookieID          string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelFortuneCookieResult struct {
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
	CookieIDHash    string
	ThemeHash       string
	NoteHash        string
	DeckHash        string
	FortuneHash     string
	PromptHash      string
	LuckyNumberHash string
	SeedHash        string
	BodyHash        string
	FortuneCount    int
	FortuneIndex    int
	LuckyNumber     int
}

type ChannelFortuneCookieActionRequest struct {
	Options             ChannelFortuneCookieOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoCookieID        bool
	TargetFromIssue     bool
	ThemeSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	CookieIDHash        string
	ThemeSHA            string
	ThemeBytes          int
	ThemeTerms          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	DeckSHA             string
	FortuneSHA          string
	PromptSHA           string
	LuckyNumberSHA      string
	SeedSHA             string
	FortuneCount        int
	FortuneIndex        int
	LuckyNumber         int
	NotificationBodySHA string
}

type channelFortuneCookieEntry struct {
	Fortune string
	Prompt  string
}

type channelFortuneCookiePick struct {
	Entry       channelFortuneCookieEntry
	DeckHash    string
	SeedHash    string
	Index       int
	Count       int
	LuckyNumber int
}

func IsChannelFortuneCookieActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelFortuneCookieActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelFortuneCookieActionFields(fields)
}

func isChannelFortuneCookieActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelFortuneCookieSubcommand(fields[1]) {
	case "fortune-cookie", "fortune-cookie-card", "cookie", "thread-cookie", "lucky-cookie", "tiny-fortune", "micro-fortune", "luck":
		return true
	default:
		return false
	}
}

func BuildChannelFortuneCookieActionRequest(ev Event, cfg Config) (ChannelFortuneCookieActionRequest, error) {
	fields, trailing, ok := channelFortuneCookieActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelFortuneCookieActionRequest{}, fmt.Errorf("missing channel fortune cookie command")
	}
	req := ChannelFortuneCookieActionRequest{
		Options: ChannelFortuneCookieOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             defaultChannelFortuneCookieThemeForSubcommand(fields[1]),
		},
		Command:     strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:  cleanChannelFortuneCookieSubcommand(fields[1]),
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
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--cookie-id", "--fortune-cookie-id", "--fortune-id", "--luck-id", "--id":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.CookieID = cleanChannelFortuneCookieID(fields[i+1])
			i++
		case "--theme", "--lane", "--for", "--mode":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			req.ThemeSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelFortuneCookieActionRequest{}, fmt.Errorf("unknown channel fortune cookie argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelFortuneCookieIssueTargetIfPresent(ev, &req)
	if err := applyChannelFortuneCookiePositionals(&req, positional); err != nil {
		return ChannelFortuneCookieActionRequest{}, err
	}
	if err := applyChannelFortuneCookieIssueTarget(ev, &req); err != nil {
		return ChannelFortuneCookieActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelFortuneCookieTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelFortuneCookieSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.CookieID) == "" {
		req.Options.CookieID = autoChannelFortuneCookieID(ev, req.Options)
		req.AutoCookieID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelFortuneCookieNotifyMessageID(ev, req.Options.CookieID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelFortuneCookieOptions(req.Options)
	if err := validateChannelFortuneCookieActionRequestOptions(req.Options); err != nil {
		return ChannelFortuneCookieActionRequest{}, err
	}
	pick := buildChannelFortuneCookiePick(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.CookieIDHash = shortDocumentHash(req.Options.CookieID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.ThemeTerms = len(memorySearchTerms(req.Options.Theme))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.DeckSHA = pick.DeckHash
	req.FortuneSHA = shortDocumentHash(pick.Entry.Fortune)
	req.PromptSHA = shortDocumentHash(pick.Entry.Prompt)
	req.LuckyNumberSHA = shortDocumentHash(fmt.Sprintf("%d", pick.LuckyNumber))
	req.SeedSHA = pick.SeedHash
	req.FortuneCount = pick.Count
	req.FortuneIndex = pick.Index
	req.LuckyNumber = pick.LuckyNumber
	req.NotificationBodySHA = shortDocumentHash(renderChannelFortuneCookieNotificationBody(req.Options))
	return req, nil
}

func RunChannelFortuneCookie(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelFortuneCookieOptions) (ChannelFortuneCookieResult, error) {
	opts = normalizeChannelFortuneCookieOptions(opts)
	var err error
	opts, err = applyChannelFortuneCookieRoute(cfg, opts)
	if err != nil {
		return ChannelFortuneCookieResult{}, err
	}
	if err := validateChannelFortuneCookieOptions(opts); err != nil {
		return ChannelFortuneCookieResult{}, err
	}
	pick := buildChannelFortuneCookiePick(opts)
	body := renderChannelFortuneCookieNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelFortuneCookieResult{}, fmt.Errorf("queue channel fortune cookie notification: %w", err)
	}
	return ChannelFortuneCookieResult{
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
		CookieIDHash:    shortDocumentHash(opts.CookieID),
		ThemeHash:       shortDocumentHash(opts.Theme),
		NoteHash:        shortDocumentHash(opts.Note),
		DeckHash:        pick.DeckHash,
		FortuneHash:     shortDocumentHash(pick.Entry.Fortune),
		PromptHash:      shortDocumentHash(pick.Entry.Prompt),
		LuckyNumberHash: shortDocumentHash(fmt.Sprintf("%d", pick.LuckyNumber)),
		SeedHash:        pick.SeedHash,
		BodyHash:        shortDocumentHash(body),
		FortuneCount:    pick.Count,
		FortuneIndex:    pick.Index,
		LuckyNumber:     pick.LuckyNumber,
	}, nil
}

func RenderChannelFortuneCookieActionReport(ev Event, req ChannelFortuneCookieActionRequest, result ChannelFortuneCookieResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := firstNonEmpty(result.Channel, req.Options.Channel)
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	cookieIDHash := firstNonEmpty(result.CookieIDHash, req.CookieIDHash)
	themeHash := firstNonEmpty(result.ThemeHash, req.ThemeSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	deckHash := firstNonEmpty(result.DeckHash, req.DeckSHA)
	fortuneHash := firstNonEmpty(result.FortuneHash, req.FortuneSHA)
	promptHash := firstNonEmpty(result.PromptHash, req.PromptSHA)
	luckyNumberHash := firstNonEmpty(result.LuckyNumberHash, req.LuckyNumberSHA)
	seedHash := firstNonEmpty(result.SeedHash, req.SeedSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	fortuneCount := result.FortuneCount
	if fortuneCount == 0 {
		fortuneCount = req.FortuneCount
	}
	fortuneIndex := result.FortuneIndex
	if fortuneIndex == 0 {
		fortuneIndex = req.FortuneIndex
	}
	luckyNumber := result.LuckyNumber
	if luckyNumber == 0 {
		luckyNumber = req.LuckyNumber
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Fortune Cookie Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_fortune_cookie_status: `%s`\n", status)
	fmt.Fprintf(&b, "- fortune_cookie_mode: `%s`\n", "deterministic-channel-fortune-cookie")
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
	fmt.Fprintf(&b, "- fortune_cookie_id_sha256_12: `%s`\n", noneIfEmpty(cookieIDHash))
	fmt.Fprintf(&b, "- fortune_cookie_id_auto: `%t`\n", req.AutoCookieID)
	fmt.Fprintf(&b, "- fortune_cookie_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- fortune_cookie_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- fortune_cookie_theme_terms: `%d`\n", req.ThemeTerms)
	fmt.Fprintf(&b, "- fortune_cookie_theme_source: `%s`\n", noneIfEmpty(req.ThemeSource))
	fmt.Fprintf(&b, "- fortune_cookie_count: `%d`\n", fortuneCount)
	fmt.Fprintf(&b, "- fortune_cookie_index: `%d`\n", fortuneIndex)
	fmt.Fprintf(&b, "- fortune_cookie_deck_sha256_12: `%s`\n", noneIfEmpty(deckHash))
	fmt.Fprintf(&b, "- fortune_cookie_text_sha256_12: `%s`\n", noneIfEmpty(fortuneHash))
	fmt.Fprintf(&b, "- fortune_cookie_prompt_sha256_12: `%s`\n", noneIfEmpty(promptHash))
	fmt.Fprintf(&b, "- fortune_cookie_lucky_number: `%d`\n", luckyNumber)
	fmt.Fprintf(&b, "- fortune_cookie_lucky_number_sha256_12: `%s`\n", noneIfEmpty(luckyNumberHash))
	fmt.Fprintf(&b, "- fortune_cookie_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- fortune_cookie_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- fortune_cookie_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- fortune_cookie_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- fortune_cookie_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
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
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fortune_cookie_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fortune_cookie_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fortune_cookie_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fortune_cookie_deck_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fortune_cookie_text_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_fortune_cookie_prompt_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_fortune_cookie_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing fortune cookie card on the canonical channel issue. This is a tiny chat-native ritual for Slack or Telegram threads: it picks one bounded fortune and next prompt from a reviewed static deck, while the source receipt keeps thread ids, message ids, cookie ids, themes, notes, deck text, fortune text, prompt text, and channel bodies out of band. The action does not call a model, use external randomness, execute commands, create artifacts/tasks/reminders, install skills, execute tools, call provider APIs, edit workflows, mutate the repository, or deliver through provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read fortune cookie cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent fortune cookie cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate fortune cookie cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelFortuneCookieActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelFortuneCookieActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelFortuneCookieIssueTarget(ev Event, req *ChannelFortuneCookieActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel fortune cookie requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelFortuneCookieIssueTargetIfPresent(ev Event, req *ChannelFortuneCookieActionRequest) {
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

func applyChannelFortuneCookiePositionals(req *ChannelFortuneCookieActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Theme == "" || req.Options.Theme == "focus" || req.ThemeSource == "default" {
				req.Options.Theme = value
				req.ThemeSource = "positional"
				continue
			}
			return fmt.Errorf("unexpected channel fortune cookie argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Theme == "" || req.Options.Theme == "focus" || req.ThemeSource == "default" {
			req.Options.Theme = value
			req.ThemeSource = "positional"
			continue
		}
		return fmt.Errorf("unexpected channel fortune cookie argument %q", value)
	}
	return nil
}

func normalizeChannelFortuneCookieOptions(opts ChannelFortuneCookieOptions) ChannelFortuneCookieOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.CookieID = cleanChannelFortuneCookieID(opts.CookieID)
	opts.Theme = cleanChannelFortuneCookieTheme(opts.Theme)
	opts.Note = cleanChannelFortuneCookieNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Theme == "" {
		opts.Theme = "focus"
	}
	return opts
}

func applyChannelFortuneCookieRoute(cfg Config, opts ChannelFortuneCookieOptions) (ChannelFortuneCookieOptions, error) {
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
		Body:      "GitClaw channel fortune cookie.",
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

func validateChannelFortuneCookieOptions(opts ChannelFortuneCookieOptions) error {
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
	if opts.CookieID == "" {
		return fmt.Errorf("missing fortune cookie id")
	}
	if len(channelFortuneCookieDeckForTheme(opts.Theme)) == 0 {
		return fmt.Errorf("unsupported fortune cookie theme %q", opts.Theme)
	}
	return nil
}

func validateChannelFortuneCookieActionRequestOptions(opts ChannelFortuneCookieOptions) error {
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
	if opts.CookieID == "" {
		return fmt.Errorf("missing fortune cookie id")
	}
	if len(channelFortuneCookieDeckForTheme(opts.Theme)) == 0 {
		return fmt.Errorf("unsupported fortune cookie theme %q", opts.Theme)
	}
	return nil
}

func cleanChannelFortuneCookieSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelFortuneCookieID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelFortuneCookieTheme(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "focus", "focused", "default", "work":
		return "focus"
	case "release", "launch", "ship", "shipping":
		return "release"
	case "debug", "fix", "bug", "incident":
		return "debug"
	case "care", "soft", "support", "steady":
		return "care"
	case "fun", "play", "spark", "social":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelFortuneCookieNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelFortuneCookieTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelFortuneCookieNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func defaultChannelFortuneCookieThemeForSubcommand(subcommand string) string {
	switch cleanChannelFortuneCookieSubcommand(subcommand) {
	case "luck", "lucky-cookie", "tiny-fortune", "micro-fortune":
		return "fun"
	default:
		return "focus"
	}
}

func autoChannelFortuneCookieSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-fortune-cookie-source-%s", eventID(ev))
}

func autoChannelFortuneCookieID(ev Event, opts ChannelFortuneCookieOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return cleanChannelFortuneCookieID(fmt.Sprintf("fortune-cookie-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelFortuneCookieNotifyMessageID(ev Event, cookieID string) string {
	seed := strings.Join([]string{eventID(ev), cookieID}, "|")
	return fmt.Sprintf("gitclaw-channel-fortune-cookie-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func buildChannelFortuneCookiePick(opts ChannelFortuneCookieOptions) channelFortuneCookiePick {
	opts = normalizeChannelFortuneCookieOptions(opts)
	deck := channelFortuneCookieDeckForTheme(opts.Theme)
	manifest := channelFortuneCookieDeckManifest(deck)
	deckHash := shortDocumentHash(manifest)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.CookieID, opts.Theme, opts.Note, deckHash}, "|")
	index := deterministicChannelChooseIndex(seed, len(deck))
	luckyNumber := deterministicChannelChooseIndex(seed+"|lucky-number", 99) + 1
	entry := channelFortuneCookieEntry{}
	if len(deck) > 0 {
		entry = deck[index]
	}
	return channelFortuneCookiePick{
		Entry:       entry,
		DeckHash:    deckHash,
		SeedHash:    shortDocumentHash(seed),
		Index:       index + 1,
		Count:       len(deck),
		LuckyNumber: luckyNumber,
	}
}

func renderChannelFortuneCookieNotificationBody(opts ChannelFortuneCookieOptions) string {
	opts = normalizeChannelFortuneCookieOptions(opts)
	pick := buildChannelFortuneCookiePick(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel fortune cookie.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	fmt.Fprintf(&b, "Opened: #%d of %d\n", pick.Index, pick.Count)
	fmt.Fprintf(&b, "Fortune: %s\n", pick.Entry.Fortune)
	fmt.Fprintf(&b, "Next prompt: %s\n", pick.Entry.Prompt)
	fmt.Fprintf(&b, "Lucky number: %d\n", pick.LuckyNumber)
	fmt.Fprintf(&b, "Fortune hash: %s\n", shortDocumentHash(pick.Entry.Fortune))
	fmt.Fprintf(&b, "Prompt hash: %s\n", shortDocumentHash(pick.Entry.Prompt))
	fmt.Fprintf(&b, "Deck hash: %s\n", pick.DeckHash)
	fmt.Fprintf(&b, "Seed hash: %s\n", pick.SeedHash)
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nSelection source: deterministic GitHub channel action seed.\n")
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

func channelFortuneCookieDeckForTheme(theme string) []channelFortuneCookieEntry {
	switch cleanChannelFortuneCookieTheme(theme) {
	case "release":
		return []channelFortuneCookieEntry{
			{Fortune: "A reversible ship is still a ship.", Prompt: "Name the rollback signal before the next update."},
			{Fortune: "The small launch with evidence outruns the grand launch with vibes.", Prompt: "Post the one run URL that makes this safe."},
			{Fortune: "Your release wants a quieter checklist.", Prompt: "Ask what must be true before the next deploy."},
			{Fortune: "One crisp owner beats three hopeful agreements.", Prompt: "Assign the next blocker or delete it."},
			{Fortune: "The announcement is part of the product.", Prompt: "Draft the provider-facing status before changing state."},
		}
	case "debug":
		return []channelFortuneCookieEntry{
			{Fortune: "The failing case is trying to be a map.", Prompt: "Write the smallest reproduction in the thread."},
			{Fortune: "Logs become useful when they answer one question.", Prompt: "Ask for the exact timestamp, run URL, or input."},
			{Fortune: "A flaky bug dislikes being named.", Prompt: "Give the failure one stable label before chasing it."},
			{Fortune: "The fastest fix may be a better boundary.", Prompt: "State what code must not be touched."},
			{Fortune: "The diff is innocent until the fixture confesses.", Prompt: "Search the repo before inventing a new path."},
		}
	case "care":
		return []channelFortuneCookieEntry{
			{Fortune: "A steady thread can hold more truth than a loud one.", Prompt: "Ask the gentlest clarifying question."},
			{Fortune: "Useful help starts by lowering the cost of replying.", Prompt: "Offer two concrete choices."},
			{Fortune: "The next answer can be smaller than the worry.", Prompt: "Name one thing that is already known."},
			{Fortune: "Clarity is a kindness with receipts.", Prompt: "Summarize the current state in one sentence."},
			{Fortune: "A pause can be a protocol.", Prompt: "Mark what should wait for a human decision."},
		}
	case "fun":
		return []channelFortuneCookieEntry{
			{Fortune: "The thread contains one tiny door disguised as a joke.", Prompt: "Ask for the strangest useful next move."},
			{Fortune: "A playful card can still leave a breadcrumb.", Prompt: "Turn the bit into one concrete follow-up."},
			{Fortune: "Someone is about to make the good kind of questionable decision.", Prompt: "Use a status wheel before committing to the bit."},
			{Fortune: "The room wants a spark, not a spreadsheet.", Prompt: "Send a vibe-check prompt."},
			{Fortune: "Today rewards the person who names the obvious thing warmly.", Prompt: "Post a toast for the smallest win."},
		}
	default:
		return []channelFortuneCookieEntry{
			{Fortune: "The next useful reply is smaller than it looks.", Prompt: "Ask for the one fact that would unblock the thread."},
			{Fortune: "A durable breadcrumb beats a perfect memory.", Prompt: "Leave the outcome in the GitHub issue."},
			{Fortune: "The thread already knows which question matters.", Prompt: "Restate the decision in plain language."},
			{Fortune: "Momentum likes reviewed surfaces.", Prompt: "Turn the next action into a channel command."},
			{Fortune: "A tiny ritual can make a serious system easier to use.", Prompt: "Pick one low-risk next command."},
		}
	}
}

func channelFortuneCookieDeckManifest(deck []channelFortuneCookieEntry) string {
	lines := make([]string, 0, len(deck))
	for _, entry := range deck {
		lines = append(lines, strings.Join([]string{entry.Fortune, entry.Prompt}, "|"))
	}
	return strings.Join(lines, "\n")
}
