package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRiddleOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	RiddleID          string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRiddleResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	RiddleIDHash string
	ThemeHash    string
	NoteHash     string
	DeckHash     string
	QuestionHash string
	HintHash     string
	AnswerHash   string
	SeedHash     string
	BodyHash     string
	RiddleCount  int
	RiddleIndex  int
}

type ChannelRiddleActionRequest struct {
	Options             ChannelRiddleOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoRiddleID        bool
	TargetFromIssue     bool
	ThemeSource         string
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	RiddleIDHash        string
	ThemeSHA            string
	ThemeBytes          int
	ThemeTerms          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	DeckSHA             string
	QuestionSHA         string
	HintSHA             string
	AnswerSHA           string
	SeedSHA             string
	RiddleCount         int
	RiddleIndex         int
	NotificationBodySHA string
}

type channelRiddleEntry struct {
	Question string
	Hint     string
	Answer   string
}

type channelRiddlePick struct {
	Entry    channelRiddleEntry
	DeckHash string
	SeedHash string
	Index    int
	Count    int
}

func IsChannelRiddleActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelRiddleActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelRiddleActionFields(fields)
}

func isChannelRiddleActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelRiddleSubcommand(fields[1]) {
	case "riddle", "riddle-card", "thread-riddle", "tiny-riddle", "micro-riddle", "brain-teaser", "teaser", "puzzle-card":
		return true
	default:
		return false
	}
}

func BuildChannelRiddleActionRequest(ev Event, cfg Config) (ChannelRiddleActionRequest, error) {
	fields, trailing, ok := channelRiddleActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRiddleActionRequest{}, fmt.Errorf("missing channel riddle command")
	}
	req := ChannelRiddleActionRequest{
		Options: ChannelRiddleOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             defaultChannelRiddleThemeForSubcommand(fields[1]),
		},
		Command:     strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand:  cleanChannelRiddleSubcommand(fields[1]),
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
				return ChannelRiddleActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--riddle-id", "--puzzle-id", "--teaser-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RiddleID = cleanChannelRiddleID(fields[i+1])
			i++
		case "--theme", "--lane", "--for", "--mode":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			req.ThemeSource = "flag"
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRiddleActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRiddleActionRequest{}, fmt.Errorf("unknown channel riddle argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelRiddleIssueTargetIfPresent(ev, &req)
	if err := applyChannelRiddlePositionals(&req, positional); err != nil {
		return ChannelRiddleActionRequest{}, err
	}
	if err := applyChannelRiddleIssueTarget(ev, &req); err != nil {
		return ChannelRiddleActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelRiddleTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelRiddleSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.RiddleID) == "" {
		req.Options.RiddleID = autoChannelRiddleID(ev, req.Options)
		req.AutoRiddleID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRiddleNotifyMessageID(ev, req.Options.RiddleID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRiddleOptions(req.Options)
	if err := validateChannelRiddleActionRequestOptions(req.Options); err != nil {
		return ChannelRiddleActionRequest{}, err
	}
	pick := buildChannelRiddlePick(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.RiddleIDHash = shortDocumentHash(req.Options.RiddleID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.ThemeTerms = len(memorySearchTerms(req.Options.Theme))
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.DeckSHA = pick.DeckHash
	req.QuestionSHA = shortDocumentHash(pick.Entry.Question)
	req.HintSHA = shortDocumentHash(pick.Entry.Hint)
	req.AnswerSHA = shortDocumentHash(pick.Entry.Answer)
	req.SeedSHA = pick.SeedHash
	req.RiddleCount = pick.Count
	req.RiddleIndex = pick.Index
	req.NotificationBodySHA = shortDocumentHash(renderChannelRiddleNotificationBody(req.Options))
	return req, nil
}

func RunChannelRiddle(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRiddleOptions) (ChannelRiddleResult, error) {
	opts = normalizeChannelRiddleOptions(opts)
	var err error
	opts, err = applyChannelRiddleRoute(cfg, opts)
	if err != nil {
		return ChannelRiddleResult{}, err
	}
	if err := validateChannelRiddleOptions(opts); err != nil {
		return ChannelRiddleResult{}, err
	}
	pick := buildChannelRiddlePick(opts)
	body := renderChannelRiddleNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelRiddleResult{}, fmt.Errorf("queue channel riddle notification: %w", err)
	}
	return ChannelRiddleResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		RiddleIDHash: shortDocumentHash(opts.RiddleID),
		ThemeHash:    shortDocumentHash(opts.Theme),
		NoteHash:     shortDocumentHash(opts.Note),
		DeckHash:     pick.DeckHash,
		QuestionHash: shortDocumentHash(pick.Entry.Question),
		HintHash:     shortDocumentHash(pick.Entry.Hint),
		AnswerHash:   shortDocumentHash(pick.Entry.Answer),
		SeedHash:     pick.SeedHash,
		BodyHash:     shortDocumentHash(body),
		RiddleCount:  pick.Count,
		RiddleIndex:  pick.Index,
	}, nil
}

func RenderChannelRiddleActionReport(ev Event, req ChannelRiddleActionRequest, result ChannelRiddleResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
	channel := firstNonEmpty(result.Channel, req.Options.Channel)
	threadHash := firstNonEmpty(result.ThreadHash, req.RequestedThreadHash)
	messageHash := firstNonEmpty(result.MessageHash, req.RequestedMsgHash)
	notifyHash := firstNonEmpty(result.NotifyHash, req.NotifyMessageHash)
	riddleIDHash := firstNonEmpty(result.RiddleIDHash, req.RiddleIDHash)
	themeHash := firstNonEmpty(result.ThemeHash, req.ThemeSHA)
	noteHash := firstNonEmpty(result.NoteHash, req.NoteSHA)
	deckHash := firstNonEmpty(result.DeckHash, req.DeckSHA)
	questionHash := firstNonEmpty(result.QuestionHash, req.QuestionSHA)
	hintHash := firstNonEmpty(result.HintHash, req.HintSHA)
	answerHash := firstNonEmpty(result.AnswerHash, req.AnswerSHA)
	seedHash := firstNonEmpty(result.SeedHash, req.SeedSHA)
	bodyHash := firstNonEmpty(result.BodyHash, req.NotificationBodySHA)
	riddleCount := result.RiddleCount
	if riddleCount == 0 {
		riddleCount = req.RiddleCount
	}
	riddleIndex := result.RiddleIndex
	if riddleIndex == 0 {
		riddleIndex = req.RiddleIndex
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Riddle Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_riddle_status: `%s`\n", status)
	fmt.Fprintf(&b, "- riddle_mode: `%s`\n", "deterministic-channel-riddle")
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
	fmt.Fprintf(&b, "- riddle_id_sha256_12: `%s`\n", noneIfEmpty(riddleIDHash))
	fmt.Fprintf(&b, "- riddle_id_auto: `%t`\n", req.AutoRiddleID)
	fmt.Fprintf(&b, "- riddle_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- riddle_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- riddle_theme_terms: `%d`\n", req.ThemeTerms)
	fmt.Fprintf(&b, "- riddle_theme_source: `%s`\n", noneIfEmpty(req.ThemeSource))
	fmt.Fprintf(&b, "- riddle_count: `%d`\n", riddleCount)
	fmt.Fprintf(&b, "- riddle_index: `%d`\n", riddleIndex)
	fmt.Fprintf(&b, "- riddle_deck_sha256_12: `%s`\n", noneIfEmpty(deckHash))
	fmt.Fprintf(&b, "- riddle_question_sha256_12: `%s`\n", noneIfEmpty(questionHash))
	fmt.Fprintf(&b, "- riddle_hint_sha256_12: `%s`\n", noneIfEmpty(hintHash))
	fmt.Fprintf(&b, "- riddle_answer_sha256_12: `%s`\n", noneIfEmpty(answerHash))
	fmt.Fprintf(&b, "- riddle_seed_sha256_12: `%s`\n", noneIfEmpty(seedHash))
	fmt.Fprintf(&b, "- riddle_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- riddle_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- riddle_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- riddle_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
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
	fmt.Fprintf(&b, "- raw_riddle_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_riddle_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_riddle_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_riddle_deck_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_riddle_question_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_riddle_hint_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_riddle_answer_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_riddle_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing riddle card on the canonical channel issue. This is a tiny chat-native puzzle for Slack or Telegram threads: it picks one bounded question, hint, and answer from a reviewed static deck, while the source receipt keeps thread ids, message ids, riddle ids, themes, notes, deck text, question text, hint text, answer text, and channel bodies out of band. The action does not call a model, use external randomness, execute commands, create artifacts/tasks/reminders, install skills, execute tools, call provider APIs, edit workflows, mutate the repository, or deliver through provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read riddle cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent riddle cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate riddle cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelRiddleActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRiddleActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRiddleIssueTarget(ev Event, req *ChannelRiddleActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel riddle requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelRiddleIssueTargetIfPresent(ev Event, req *ChannelRiddleActionRequest) {
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

func applyChannelRiddlePositionals(req *ChannelRiddleActionRequest, positional []string) error {
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
			return fmt.Errorf("unexpected channel riddle argument %q", value)
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
		return fmt.Errorf("unexpected channel riddle argument %q", value)
	}
	return nil
}

func normalizeChannelRiddleOptions(opts ChannelRiddleOptions) ChannelRiddleOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.RiddleID = cleanChannelRiddleID(opts.RiddleID)
	opts.Theme = cleanChannelRiddleTheme(opts.Theme)
	opts.Note = cleanChannelRiddleNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Theme == "" {
		opts.Theme = "focus"
	}
	return opts
}

func applyChannelRiddleRoute(cfg Config, opts ChannelRiddleOptions) (ChannelRiddleOptions, error) {
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
		Body:      "GitClaw channel riddle.",
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

func validateChannelRiddleOptions(opts ChannelRiddleOptions) error {
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
	if opts.RiddleID == "" {
		return fmt.Errorf("missing riddle id")
	}
	if len(channelRiddleDeckForTheme(opts.Theme)) == 0 {
		return fmt.Errorf("unsupported riddle theme %q", opts.Theme)
	}
	return nil
}

func validateChannelRiddleActionRequestOptions(opts ChannelRiddleOptions) error {
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
	if opts.RiddleID == "" {
		return fmt.Errorf("missing riddle id")
	}
	if len(channelRiddleDeckForTheme(opts.Theme)) == 0 {
		return fmt.Errorf("unsupported riddle theme %q", opts.Theme)
	}
	return nil
}

func cleanChannelRiddleSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelRiddleID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelRiddleTheme(value string) string {
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

func cleanChannelRiddleNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelRiddleTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelRiddleNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func defaultChannelRiddleThemeForSubcommand(subcommand string) string {
	switch cleanChannelRiddleSubcommand(subcommand) {
	case "tiny-riddle", "micro-riddle", "brain-teaser", "teaser", "puzzle-card":
		return "fun"
	default:
		return "focus"
	}
}

func autoChannelRiddleSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-riddle-source-%s", eventID(ev))
}

func autoChannelRiddleID(ev Event, opts ChannelRiddleOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return cleanChannelRiddleID(fmt.Sprintf("riddle-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelRiddleNotifyMessageID(ev Event, riddleID string) string {
	seed := strings.Join([]string{eventID(ev), riddleID}, "|")
	return fmt.Sprintf("gitclaw-channel-riddle-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func buildChannelRiddlePick(opts ChannelRiddleOptions) channelRiddlePick {
	opts = normalizeChannelRiddleOptions(opts)
	deck := channelRiddleDeckForTheme(opts.Theme)
	manifest := channelRiddleDeckManifest(deck)
	deckHash := shortDocumentHash(manifest)
	seed := strings.Join([]string{opts.Repo, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.NotifyMessageID, opts.RiddleID, opts.Theme, opts.Note, deckHash}, "|")
	index := deterministicChannelChooseIndex(seed, len(deck))
	entry := channelRiddleEntry{}
	if len(deck) > 0 {
		entry = deck[index]
	}
	return channelRiddlePick{
		Entry:    entry,
		DeckHash: deckHash,
		SeedHash: shortDocumentHash(seed),
		Index:    index + 1,
		Count:    len(deck),
	}
}

func renderChannelRiddleNotificationBody(opts ChannelRiddleOptions) string {
	opts = normalizeChannelRiddleOptions(opts)
	pick := buildChannelRiddlePick(opts)
	var b strings.Builder
	b.WriteString("GitClaw channel riddle.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	fmt.Fprintf(&b, "Picked: #%d of %d\n", pick.Index, pick.Count)
	fmt.Fprintf(&b, "Riddle: %s\n", pick.Entry.Question)
	fmt.Fprintf(&b, "Hint: %s\n", pick.Entry.Hint)
	fmt.Fprintf(&b, "Answer: %s\n", pick.Entry.Answer)
	fmt.Fprintf(&b, "Question hash: %s\n", shortDocumentHash(pick.Entry.Question))
	fmt.Fprintf(&b, "Hint hash: %s\n", shortDocumentHash(pick.Entry.Hint))
	fmt.Fprintf(&b, "Answer hash: %s\n", shortDocumentHash(pick.Entry.Answer))
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

func channelRiddleDeckForTheme(theme string) []channelRiddleEntry {
	switch cleanChannelRiddleTheme(theme) {
	case "release":
		return []channelRiddleEntry{
			{Question: "What gets smaller when the release gets safer?", Hint: "Think rollback, not scope theater.", Answer: "The blast radius."},
			{Question: "What travels first but should arrive last?", Hint: "It tells users what changed after the checks pass.", Answer: "The announcement."},
			{Question: "I turn a scary deploy into a reversible walk. What am I?", Hint: "Name it before the merge.", Answer: "A rollback plan."},
			{Question: "What is green, boring, and secretly exciting?", Hint: "It appears before you ship.", Answer: "The CI run."},
			{Question: "What weighs less than a meeting and unblocks more than one?", Hint: "It has a name beside it.", Answer: "A clear owner."},
		}
	case "debug":
		return []channelRiddleEntry{
			{Question: "I appear once, disappear twice, and become real when named. What am I?", Hint: "Give it a stable repro.", Answer: "A flaky bug."},
			{Question: "What is loud until it answers one question?", Hint: "You search it by timestamp.", Answer: "A log line."},
			{Question: "What makes a mystery smaller without fixing it?", Hint: "It usually fits in a test.", Answer: "A reproduction."},
			{Question: "What tells the truth only after you stop assuming?", Hint: "Ask it before editing the code.", Answer: "The failing fixture."},
			{Question: "I protect the fix by naming what must not move. What am I?", Hint: "Say it before touching neighboring code.", Answer: "A boundary."},
		}
	case "care":
		return []channelRiddleEntry{
			{Question: "What lowers the cost of replying without lowering the care?", Hint: "Two paths are enough.", Answer: "Concrete choices."},
			{Question: "What can hold uncertainty without dropping the thread?", Hint: "It is allowed to wait.", Answer: "A pause."},
			{Question: "What gets kinder when it gets more specific?", Hint: "It often comes with receipts.", Answer: "Clarity."},
			{Question: "What is smaller than the worry and useful anyway?", Hint: "Start with one fact.", Answer: "The next answer."},
			{Question: "What should be soft in tone and hard in evidence?", Hint: "It updates the room.", Answer: "A status note."},
		}
	case "fun":
		return []channelRiddleEntry{
			{Question: "What starts as a bit and ends as a breadcrumb?", Hint: "It leaves one useful next step.", Answer: "A good channel card."},
			{Question: "What has no dice, no score, and still feels like play?", Hint: "It nudges the room without keeping state.", Answer: "A tiny prompt game."},
			{Question: "What can be silly and still lower coordination cost?", Hint: "The answer should fit in chat.", Answer: "A shared ritual."},
			{Question: "What opens the door without making anyone enter?", Hint: "It is low-pressure on purpose.", Answer: "An icebreaker."},
			{Question: "What turns a quiet thread into a small useful spark?", Hint: "Ask for one weird but useful move.", Answer: "A riddle."},
		}
	default:
		return []channelRiddleEntry{
			{Question: "What remembers the thread without becoming memory?", Hint: "It is stored where the conversation already lives.", Answer: "A GitHub issue."},
			{Question: "What is safer when reviewed and better when small?", Hint: "It moves the next action along.", Answer: "A channel command."},
			{Question: "What carries context but not raw bodies?", Hint: "It prefers hashes and counts.", Answer: "A body-free receipt."},
			{Question: "What lets chat stay chat while GitHub stays canonical?", Hint: "It queues rather than calls the provider.", Answer: "The channel outbox."},
			{Question: "What makes a serious assistant easier to invite in?", Hint: "It is useful, tiny, and optional.", Answer: "A playful ritual."},
		}
	}
}

func channelRiddleDeckManifest(deck []channelRiddleEntry) string {
	lines := make([]string, 0, len(deck))
	for _, entry := range deck {
		lines = append(lines, strings.Join([]string{entry.Question, entry.Hint, entry.Answer}, "|"))
	}
	return strings.Join(lines, "\n")
}
