package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelMadLibsOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MadLibsID         string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelMadLibsResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	MadLibsHash  string
	ThemeHash    string
	NoteHash     string
	TemplateHash string
	BlankHash    string
	PromptHash   string
	SeedHash     string
	BodyHash     string
	BlankCount   int
	DeckSize     int
	CardIndex    int
}

type ChannelMadLibsActionRequest struct {
	Options             ChannelMadLibsOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoMadLibsID       bool
	TargetFromIssue     bool
	ThemeSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	MadLibsIDHash       string
	ThemeSHA            string
	ThemeBytes          int
	ThemeTerms          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	TemplateSHA         string
	BlankSHA            string
	PromptSHA           string
	SeedSHA             string
	BlankCount          int
	DeckSize            int
	CardIndex           int
	NotificationBodySHA string
}

type ChannelMadLibsCard struct {
	Template string
	Blanks   []ChannelMadLibsBlank
	Prompt   string
}

type ChannelMadLibsBlank struct {
	Label string
	Hint  string
}

type ChannelMadLibsPick struct {
	Card     ChannelMadLibsCard
	SeedSHA  string
	Index    int
	DeckSize int
}

func IsChannelMadLibsActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelMadLibsActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelMadLibsActionFields(fields)
}

func isChannelMadLibsActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelMadLibsSubcommand(fields[1]) {
	case "mad-libs", "mad-lib", "madlibs", "madlib", "fill-in", "fillin", "fill-blanks", "blanks", "word-game":
		return true
	default:
		return false
	}
}

func BuildChannelMadLibsActionRequest(ev Event, cfg Config) (ChannelMadLibsActionRequest, error) {
	fields, trailing, ok := channelMadLibsActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelMadLibsActionRequest{}, fmt.Errorf("missing channel mad libs command")
	}
	req := ChannelMadLibsActionRequest{
		Options: ChannelMadLibsOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             defaultChannelMadLibsThemeForSubcommand(fields[1]),
		},
		Command:     strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:  cleanChannelMadLibsSubcommand(fields[1]),
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
				return ChannelMadLibsActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--mad-libs-id", "--mad-lib-id", "--fill-in-id", "--blank-id", "--word-game-id", "--id":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MadLibsID = cleanChannelMadLibsID(fields[i+1])
			i++
		case "--theme", "--lane", "--for", "--mode":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			req.ThemeSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMadLibsActionRequest{}, fmt.Errorf("unknown channel mad libs argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelMadLibsIssueTargetIfPresent(ev, &req)
	if err := applyChannelMadLibsPositionals(&req, positional); err != nil {
		return ChannelMadLibsActionRequest{}, err
	}
	if err := applyChannelMadLibsIssueTarget(ev, &req); err != nil {
		return ChannelMadLibsActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelMadLibsTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelMadLibsSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MadLibsID) == "" {
		req.Options.MadLibsID = autoChannelMadLibsID(ev, req.Options)
		req.AutoMadLibsID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMadLibsNotifyMessageID(ev, req.Options.MadLibsID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMadLibsOptions(req.Options)
	if err := validateChannelMadLibsActionRequestOptions(req.Options); err != nil {
		return ChannelMadLibsActionRequest{}, err
	}
	pick := BuildChannelMadLibsPick(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MadLibsIDHash = shortDocumentHash(req.Options.MadLibsID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.ThemeTerms = len(memorySearchTerms(req.Options.Theme))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.TemplateSHA = shortDocumentHash(pick.Card.Template)
	req.BlankSHA = shortDocumentHash(channelMadLibsBlankText(pick.Card.Blanks))
	req.PromptSHA = shortDocumentHash(pick.Card.Prompt)
	req.SeedSHA = pick.SeedSHA
	req.BlankCount = len(pick.Card.Blanks)
	req.DeckSize = pick.DeckSize
	req.CardIndex = pick.Index
	req.NotificationBodySHA = shortDocumentHash(renderChannelMadLibsNotificationBody(req.Options))
	return req, nil
}

func RunChannelMadLibs(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMadLibsOptions) (ChannelMadLibsResult, error) {
	opts = normalizeChannelMadLibsOptions(opts)
	var err error
	opts, err = applyChannelMadLibsRoute(cfg, opts)
	if err != nil {
		return ChannelMadLibsResult{}, err
	}
	if err := validateChannelMadLibsOptions(opts); err != nil {
		return ChannelMadLibsResult{}, err
	}
	pick := BuildChannelMadLibsPick(opts)
	body := renderChannelMadLibsNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelMadLibsResult{}, fmt.Errorf("queue channel mad libs notification: %w", err)
	}
	return ChannelMadLibsResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		MadLibsHash:  shortDocumentHash(opts.MadLibsID),
		ThemeHash:    shortDocumentHash(opts.Theme),
		NoteHash:     shortDocumentHash(opts.Note),
		TemplateHash: shortDocumentHash(pick.Card.Template),
		BlankHash:    shortDocumentHash(channelMadLibsBlankText(pick.Card.Blanks)),
		PromptHash:   shortDocumentHash(pick.Card.Prompt),
		SeedHash:     pick.SeedSHA,
		BodyHash:     shortDocumentHash(body),
		BlankCount:   len(pick.Card.Blanks),
		DeckSize:     pick.DeckSize,
		CardIndex:    pick.Index,
	}, nil
}

func RenderChannelMadLibsActionReport(ev Event, req ChannelMadLibsActionRequest, result ChannelMadLibsResult) string {
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
	madLibsHash := firstNonEmpty(result.MadLibsHash, req.MadLibsIDHash)
	themeHash := firstNonEmpty(result.ThemeHash, req.ThemeSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	templateHash := firstNonEmpty(result.TemplateHash, req.TemplateSHA)
	blankHash := firstNonEmpty(result.BlankHash, req.BlankSHA)
	promptHash := firstNonEmpty(result.PromptHash, req.PromptSHA)
	seedHash := firstNonEmpty(result.SeedHash, req.SeedSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	blankCount := result.BlankCount
	if blankCount == 0 {
		blankCount = req.BlankCount
	}
	deckSize := result.DeckSize
	if deckSize == 0 {
		deckSize = req.DeckSize
	}
	cardIndex := result.CardIndex
	if result.DeckSize == 0 {
		cardIndex = req.CardIndex
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Mad Libs Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_mad_libs_status: `%s`\n", status)
	fmt.Fprintf(&b, "- mad_libs_mode: `%s`\n", "deterministic-channel-fill-in-card")
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
	fmt.Fprintf(&b, "- mad_libs_id_sha256_12: `%s`\n", noneIfEmpty(madLibsHash))
	fmt.Fprintf(&b, "- mad_libs_id_auto: `%t`\n", req.AutoMadLibsID)
	fmt.Fprintf(&b, "- mad_libs_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- mad_libs_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- mad_libs_theme_terms: `%d`\n", req.ThemeTerms)
	fmt.Fprintf(&b, "- mad_libs_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- mad_libs_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- mad_libs_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- mad_libs_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- mad_libs_template_sha256_12: `%s`\n", noneIfEmpty(templateHash))
	fmt.Fprintf(&b, "- mad_libs_blank_bank_sha256_12: `%s`\n", noneIfEmpty(blankHash))
	fmt.Fprintf(&b, "- mad_libs_prompt_sha256_12: `%s`\n", noneIfEmpty(promptHash))
	fmt.Fprintf(&b, "- mad_libs_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- mad_libs_blank_count: `%d`\n", blankCount)
	fmt.Fprintf(&b, "- mad_libs_deck_size: `%d`\n", deckSize)
	fmt.Fprintf(&b, "- selected_card_index: `%d`\n", cardIndex)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- dynamic_text_generation_performed: `%t`\n", false)
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
	fmt.Fprintf(&b, "- raw_mad_libs_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mad_libs_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mad_libs_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mad_libs_template_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mad_libs_blank_bank_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mad_libs_prompt_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_mad_libs_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel mad-libs fill-in card on the canonical channel issue. This is a tiny chat-native word game: Slack or Telegram can get one bounded template and blank bank, while the source receipt keeps thread ids, message ids, mad-libs ids, themes, notes, templates, blanks, prompts, and channel bodies out of band. The action does not call a model, generate text dynamically, use external randomness, persist game state, track scores, mutate repository files, edit workflows, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read mad-libs cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent mad-libs cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate mad-libs cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelMadLibsActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelMadLibsActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelMadLibsIssueTarget(ev Event, req *ChannelMadLibsActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel mad libs requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelMadLibsIssueTargetIfPresent(ev Event, req *ChannelMadLibsActionRequest) {
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

func applyChannelMadLibsPositionals(req *ChannelMadLibsActionRequest, positional []string) error {
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
				req.ThemeSource = "positional"
				continue
			}
			return fmt.Errorf("unexpected channel mad libs argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Theme == "" || req.Options.Theme == "general" || req.Options.Theme == "fun" {
			req.Options.Theme = value
			req.ThemeSource = "positional"
			continue
		}
		return fmt.Errorf("unexpected channel mad libs argument %q", value)
	}
	return nil
}

func normalizeChannelMadLibsOptions(opts ChannelMadLibsOptions) ChannelMadLibsOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MadLibsID = cleanChannelMadLibsID(opts.MadLibsID)
	opts.Theme = cleanChannelMadLibsTheme(opts.Theme)
	opts.Note = cleanChannelMadLibsNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelMadLibsRoute(cfg Config, opts ChannelMadLibsOptions) (ChannelMadLibsOptions, error) {
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
		Body:      "GitClaw channel mad libs.",
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

func validateChannelMadLibsOptions(opts ChannelMadLibsOptions) error {
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
	if opts.MadLibsID == "" {
		return fmt.Errorf("missing mad libs id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing mad libs theme")
	}
	return nil
}

func validateChannelMadLibsActionRequestOptions(opts ChannelMadLibsOptions) error {
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
	if opts.MadLibsID == "" {
		return fmt.Errorf("missing mad libs id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing mad libs theme")
	}
	return nil
}

func cleanChannelMadLibsSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(value, " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "fillin":
		return "fill-in"
	case "madlib":
		return "mad-lib"
	case "madlibs":
		return "mad-libs"
	default:
		return strings.Trim(value, "-")
	}
}

func cleanChannelMadLibsID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelMadLibsTheme(value string) string {
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

func cleanChannelMadLibsNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelMadLibsTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelMadLibsTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"note:", "context:", "because:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelMadLibsNote(trimmed[idx+1:])
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelMadLibsTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelMadLibsThemeForSubcommand(subcommand string) string {
	switch cleanChannelMadLibsSubcommand(subcommand) {
	case "word-game", "fill-in", "fill-blanks", "blanks":
		return "fun"
	default:
		return "general"
	}
}

func autoChannelMadLibsSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-mad-libs-source-%s", eventID(ev))
}

func autoChannelMadLibsID(ev Event, opts ChannelMadLibsOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return fmt.Sprintf("mad-libs-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMadLibsNotifyMessageID(ev Event, madLibsID string) string {
	seed := strings.Join([]string{eventID(ev), madLibsID}, "|")
	return fmt.Sprintf("gitclaw-channel-mad-libs-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func BuildChannelMadLibsPick(opts ChannelMadLibsOptions) ChannelMadLibsPick {
	opts = normalizeChannelMadLibsOptions(opts)
	deck := channelMadLibsDeckForTheme(opts.Theme)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.MadLibsID, opts.Theme, opts.Note}, "|")
	index := deterministicChannelChooseIndex(seed, len(deck))
	card := deck[index]
	return ChannelMadLibsPick{Card: card, SeedSHA: shortDocumentHash(seed), Index: index, DeckSize: len(deck)}
}

func renderChannelMadLibsNotificationBody(opts ChannelMadLibsOptions) string {
	opts = normalizeChannelMadLibsOptions(opts)
	pick := BuildChannelMadLibsPick(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel mad libs.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	b.WriteString("Fill-in card:\n")
	fmt.Fprintf(&b, "Template: %s\n", pick.Card.Template)
	b.WriteString("Blanks:\n")
	for i, blank := range pick.Card.Blanks {
		if blank.Hint == "" {
			fmt.Fprintf(&b, "%d. %s\n", i+1, blank.Label)
			continue
		}
		fmt.Fprintf(&b, "%d. %s - %s\n", i+1, blank.Label, blank.Hint)
	}
	fmt.Fprintf(&b, "Prompt: %s\n", pick.Card.Prompt)
	fmt.Fprintf(&b, "Mad libs hash: %s\n", shortDocumentHash(pick.Card.Template+"\n"+channelMadLibsBlankText(pick.Card.Blanks)+"\n"+pick.Card.Prompt))
	fmt.Fprintf(&b, "Seed hash: %s\n", pick.SeedSHA)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nMad libs source: deterministic GitHub channel action seed.\n")
	b.WriteString("Mad libs deck: bounded static GitClaw fill-in deck.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Dynamic text generation: not performed by this action.\n")
	b.WriteString("External randomness: not used.\n")
	b.WriteString("Game state: not persisted by this action.\n")
	b.WriteString("Score tracking: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelMadLibsBlankText(blanks []ChannelMadLibsBlank) string {
	var lines []string
	for _, blank := range blanks {
		if blank.Hint == "" {
			lines = append(lines, blank.Label)
			continue
		}
		lines = append(lines, blank.Label+": "+blank.Hint)
	}
	return strings.Join(lines, "\n")
}

func channelMadLibsDeckForTheme(theme string) []ChannelMadLibsCard {
	switch cleanChannelMadLibsTheme(theme) {
	case "launch", "ship", "release":
		return []ChannelMadLibsCard{
			{Template: "The [adjective] release captain pressed [button] after the [artifact] said [phrase].", Blanks: []ChannelMadLibsBlank{{"adjective", "a mood"}, {"button", "a tiny action"}, {"artifact", "a release object"}, {"phrase", "three words"}}, Prompt: "Reply with adjective, button, artifact, and phrase."},
			{Template: "Before shipping, the team gave the [system] a [snack] and asked [person] to check [risk].", Blanks: []ChannelMadLibsBlank{{"system", "thing being shipped"}, {"snack", "object"}, {"person", "role"}, {"risk", "one risk"}}, Prompt: "Reply with system, snack, person, and risk."},
			{Template: "A [signal] turned green, so the room wrote a [document] and kept one [backup] nearby.", Blanks: []ChannelMadLibsBlank{{"signal", "status cue"}, {"document", "artifact"}, {"backup", "fallback"}}, Prompt: "Reply with signal, document, and backup."},
		}
	case "tools", "tool":
		return []ChannelMadLibsCard{
			{Template: "The tool named [tool] promised to [verb], but first it needed a [boundary] and a [receipt].", Blanks: []ChannelMadLibsBlank{{"tool", "safe tool name"}, {"verb", "read-only action"}, {"boundary", "limit"}, {"receipt", "proof"}}, Prompt: "Reply with tool, verb, boundary, and receipt."},
			{Template: "A [schema] walked into chat carrying [input] and asked for [approval] before touching [output].", Blanks: []ChannelMadLibsBlank{{"schema", "contract"}, {"input", "small input"}, {"approval", "gate"}, {"output", "result"}}, Prompt: "Reply with schema, input, approval, and output."},
			{Template: "The safest tool run is [adjective], [adjective], and leaves a [noun] behind.", Blanks: []ChannelMadLibsBlank{{"adjective", "quality"}, {"adjective", "second quality"}, {"noun", "audit object"}}, Prompt: "Reply with two qualities and one audit object."},
		}
	case "research", "map":
		return []ChannelMadLibsCard{
			{Template: "The research map found [source], rejected [pattern], and kept [lesson] for later.", Blanks: []ChannelMadLibsBlank{{"source", "reference"}, {"pattern", "anti-pattern"}, {"lesson", "small lesson"}}, Prompt: "Reply with source, pattern, and lesson."},
			{Template: "Open the [document], underline [claim], and turn [question] into the next [command].", Blanks: []ChannelMadLibsBlank{{"document", "source"}, {"claim", "fact"}, {"question", "open question"}, {"command", "GitClaw command"}}, Prompt: "Reply with document, claim, question, and command."},
			{Template: "A careful note says [thing] works only when [condition] and never when [failure].", Blanks: []ChannelMadLibsBlank{{"thing", "idea"}, {"condition", "constraint"}, {"failure", "bad assumption"}}, Prompt: "Reply with thing, condition, and failure."},
		}
	case "soul", "memory":
		return []ChannelMadLibsCard{
			{Template: "The thread remembered [lesson] because [moment], then packed it into [container].", Blanks: []ChannelMadLibsBlank{{"lesson", "short lesson"}, {"moment", "why it matters"}, {"container", "durable place"}}, Prompt: "Reply with lesson, moment, and container."},
			{Template: "A high-authority note said [rule], so the channel chose [behavior] instead of [shortcut].", Blanks: []ChannelMadLibsBlank{{"rule", "principle"}, {"behavior", "next behavior"}, {"shortcut", "temptation"}}, Prompt: "Reply with rule, behavior, and shortcut."},
			{Template: "Memory should be [adjective], [adjective], and useful when [future-event] happens.", Blanks: []ChannelMadLibsBlank{{"adjective", "quality"}, {"adjective", "second quality"}, {"future-event", "future trigger"}}, Prompt: "Reply with two qualities and one future trigger."},
		}
	case "backups", "backup", "restore":
		return []ChannelMadLibsCard{
			{Template: "The backup trail followed [hash] through [archive] until it found [anchor].", Blanks: []ChannelMadLibsBlank{{"hash", "short proof"}, {"archive", "backup place"}, {"anchor", "issue or run"}}, Prompt: "Reply with hash, archive, and anchor."},
			{Template: "Before restore, the team rehearsed [step], checked [signal], and left [note].", Blanks: []ChannelMadLibsBlank{{"step", "safe step"}, {"signal", "freshness clue"}, {"note", "handoff"}}, Prompt: "Reply with step, signal, and note."},
			{Template: "A boring recovery needs [map], [permission], and one [test].", Blanks: []ChannelMadLibsBlank{{"map", "route back"}, {"permission", "approval"}, {"test", "verification"}}, Prompt: "Reply with map, permission, and test."},
		}
	case "fun", "play":
		return []ChannelMadLibsCard{
			{Template: "Today the channel found a [adjective] [artifact] and decided to [verb] before the [deadline].", Blanks: []ChannelMadLibsBlank{{"adjective", "mood"}, {"artifact", "object"}, {"verb", "action"}, {"deadline", "time cue"}}, Prompt: "Reply with adjective, artifact, verb, and deadline."},
			{Template: "A tiny [role] opened a [container] and discovered the next command was [command].", Blanks: []ChannelMadLibsBlank{{"role", "role or persona"}, {"container", "object"}, {"command", "GitClaw channel command"}}, Prompt: "Reply with role, container, and command."},
			{Template: "The room traded [snack] for [tool] and promised to keep the receipt [adjective].", Blanks: []ChannelMadLibsBlank{{"snack", "object"}, {"tool", "capability"}, {"adjective", "quality"}}, Prompt: "Reply with snack, tool, and adjective."},
		}
	default:
		return []ChannelMadLibsCard{
			{Template: "A [channel] message became a [github-object] and left a [proof] for the next person.", Blanks: []ChannelMadLibsBlank{{"channel", "provider"}, {"github-object", "issue artifact"}, {"proof", "metadata trail"}}, Prompt: "Reply with provider, GitHub object, and proof."},
			{Template: "The bridge carried [message] to [place], then queued [reply] without needing [server].", Blanks: []ChannelMadLibsBlank{{"message", "small message"}, {"place", "GitHub place"}, {"reply", "outbound card"}, {"server", "runtime"}}, Prompt: "Reply with message, place, reply, and server."},
			{Template: "GitHub remembered [context], the channel kept [vibe], and the next move was [command].", Blanks: []ChannelMadLibsBlank{{"context", "useful context"}, {"vibe", "tone"}, {"command", "next command"}}, Prompt: "Reply with context, vibe, and command."},
		}
	}
}
