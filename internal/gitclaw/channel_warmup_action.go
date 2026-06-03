package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelWarmupOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	WarmupID          string
	Theme             string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelWarmupResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	WarmupIDHash string
	ThemeHash    string
	NoteHash     string
	BodyHash     string
	PromptCount  int
}

type ChannelWarmupActionRequest struct {
	Options             ChannelWarmupOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoWarmupID        bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	WarmupIDHash        string
	ThemeSHA            string
	ThemeBytes          int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	PromptCount         int
	NotificationBodySHA string
}

func IsChannelWarmupActionRequest(ev Event, cfg Config) bool {
	return isChannelWarmupActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelWarmupActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "warmup", "warmups", "set-warmup", "thread-warmup", "starter", "starters", "icebreaker", "icebreakers", "kickoff", "prompt-card", "conversation-starter", "question-card":
		return true
	default:
		return false
	}
}

func BuildChannelWarmupActionRequest(ev Event, cfg Config) (ChannelWarmupActionRequest, error) {
	fields, trailing, ok := channelWarmupActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelWarmupActionRequest{}, fmt.Errorf("missing channel warmup command")
	}
	req := ChannelWarmupActionRequest{
		Options: ChannelWarmupOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Theme:             "focus",
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
				return ChannelWarmupActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--warmup-id", "--starter-id", "--icebreaker-id", "--prompt-card-id", "--id":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.WarmupID = cleanChannelWarmupID(fields[i+1])
			i++
		case "--theme", "--warmup", "--starter", "--for":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Theme = fields[i+1]
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelWarmupActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelWarmupActionRequest{}, fmt.Errorf("unknown channel warmup argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelWarmupIssueTargetIfPresent(ev, &req)
	if err := applyChannelWarmupPositionals(&req, positional); err != nil {
		return ChannelWarmupActionRequest{}, err
	}
	if err := applyChannelWarmupIssueTarget(ev, &req); err != nil {
		return ChannelWarmupActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelWarmupTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelWarmupSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.WarmupID) == "" {
		req.Options.WarmupID = autoChannelWarmupID(ev, req.Options)
		req.AutoWarmupID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelWarmupNotifyMessageID(ev, req.Options.WarmupID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelWarmupOptions(req.Options)
	if err := validateChannelWarmupActionRequestOptions(req.Options); err != nil {
		return ChannelWarmupActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.WarmupIDHash = shortDocumentHash(req.Options.WarmupID)
	req.ThemeSHA = shortDocumentHash(req.Options.Theme)
	req.ThemeBytes = len(req.Options.Theme)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.PromptCount = len(channelWarmupPromptsForTheme(req.Options.Theme))
	req.NotificationBodySHA = shortDocumentHash(renderChannelWarmupNotificationBody(req.Options))
	return req, nil
}

func RunChannelWarmup(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelWarmupOptions) (ChannelWarmupResult, error) {
	opts = normalizeChannelWarmupOptions(opts)
	var err error
	opts, err = applyChannelWarmupRoute(cfg, opts)
	if err != nil {
		return ChannelWarmupResult{}, err
	}
	if err := validateChannelWarmupOptions(opts); err != nil {
		return ChannelWarmupResult{}, err
	}
	body := renderChannelWarmupNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelWarmupResult{}, fmt.Errorf("queue channel warmup notification: %w", err)
	}
	return ChannelWarmupResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		WarmupIDHash: shortDocumentHash(opts.WarmupID),
		ThemeHash:    shortDocumentHash(opts.Theme),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
		PromptCount:  len(channelWarmupPromptsForTheme(opts.Theme)),
	}, nil
}

func RenderChannelWarmupActionReport(ev Event, req ChannelWarmupActionRequest, result ChannelWarmupResult) string {
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
	warmupIDHash := result.WarmupIDHash
	if warmupIDHash == "" {
		warmupIDHash = req.WarmupIDHash
	}
	themeHash := result.ThemeHash
	if themeHash == "" {
		themeHash = req.ThemeSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	promptCount := result.PromptCount
	if promptCount == 0 {
		promptCount = req.PromptCount
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Warmup Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_warmup_status: `%s`\n", status)
	fmt.Fprintf(&b, "- warmup_card_mode: `%s`\n", "structured-channel-warmup")
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
	fmt.Fprintf(&b, "- warmup_id_sha256_12: `%s`\n", noneIfEmpty(warmupIDHash))
	fmt.Fprintf(&b, "- warmup_id_auto: `%t`\n", req.AutoWarmupID)
	fmt.Fprintf(&b, "- warmup_theme_sha256_12: `%s`\n", noneIfEmpty(themeHash))
	fmt.Fprintf(&b, "- warmup_theme_bytes: `%d`\n", req.ThemeBytes)
	fmt.Fprintf(&b, "- warmup_prompt_count: `%d`\n", promptCount)
	fmt.Fprintf(&b, "- warmup_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- warmup_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- warmup_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- warmup_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- command_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_payload_read: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_body_read: `%t`\n", false)
	fmt.Fprintf(&b, "- warmup_persistence_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- schedule_created: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_warmup_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_warmup_theme_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_warmup_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_warmup_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_warmup_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel warmup card on the canonical channel issue. This gives the chat thread deterministic conversation starters selected by the caller while keeping command execution, skill installs, tool execution, backup payload reads, soul body reads, provider API calls, model calls, provider delivery, workflow edits, policy changes, schedules, durable warmup persistence, and repository mutations out of this action. The source receipt keeps thread ids, message ids, warmup ids, warmup themes, notes, prompt text, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read warmup updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent warmup updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate warmup updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelWarmupActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelWarmupActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelWarmupIssueTarget(ev Event, req *ChannelWarmupActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel warmup requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelWarmupIssueTargetIfPresent(ev Event, req *ChannelWarmupActionRequest) {
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

func applyChannelWarmupPositionals(req *ChannelWarmupActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Theme == "" || req.Options.Theme == "focus" {
				req.Options.Theme = value
				continue
			}
			return fmt.Errorf("unexpected channel warmup argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Theme == "" || req.Options.Theme == "focus" {
			req.Options.Theme = value
			continue
		}
		return fmt.Errorf("unexpected channel warmup argument %q", value)
	}
	return nil
}

func normalizeChannelWarmupOptions(opts ChannelWarmupOptions) ChannelWarmupOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.WarmupID = cleanChannelWarmupID(opts.WarmupID)
	opts.Theme = cleanChannelWarmupTheme(opts.Theme)
	opts.Note = cleanChannelWarmupNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelWarmupRoute(cfg Config, opts ChannelWarmupOptions) (ChannelWarmupOptions, error) {
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
		Body:      "GitClaw channel warmup.",
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

func validateChannelWarmupOptions(opts ChannelWarmupOptions) error {
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
	if opts.WarmupID == "" {
		return fmt.Errorf("missing warmup id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing warmup theme")
	}
	if len(channelWarmupPromptsForTheme(opts.Theme)) == 0 {
		return fmt.Errorf("unsupported warmup theme %q", opts.Theme)
	}
	return nil
}

func validateChannelWarmupActionRequestOptions(opts ChannelWarmupOptions) error {
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
	if opts.WarmupID == "" {
		return fmt.Errorf("missing warmup id")
	}
	if opts.Theme == "" {
		return fmt.Errorf("missing warmup theme")
	}
	if len(channelWarmupPromptsForTheme(opts.Theme)) == 0 {
		return fmt.Errorf("unsupported warmup theme %q", opts.Theme)
	}
	return nil
}

func cleanChannelWarmupID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelWarmupTheme(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	switch value {
	case "", "focus", "focused", "default", "work", "deep-work":
		return "focus"
	case "pair", "pairing", "collab", "collaboration", "co-work", "cowork":
		return "pairing"
	case "triage", "inbox", "sort", "sorting":
		return "triage"
	case "design", "product", "ux", "ui":
		return "design"
	case "launch", "release", "ship", "shipping":
		return "launch"
	case "retro", "retrospective", "review", "after-action", "lessons":
		return "retro"
	case "tool", "tools", "tool-review", "review-tools", "approval", "approvals":
		return "tools"
	case "soul", "souls", "soul-review", "identity", "authority", "context":
		return "soul"
	case "backup", "backups", "backup-review", "recovery", "restore", "rollback":
		return "backups"
	case "fun", "play", "light", "social":
		return "fun"
	default:
		return ""
	}
}

func cleanChannelWarmupNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelWarmupTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelWarmupNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelWarmupSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-warmup-source-%s", eventID(ev))
}

func autoChannelWarmupID(ev Event, opts ChannelWarmupOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Theme, opts.Note}, "|")
	return fmt.Sprintf("warmup-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelWarmupNotifyMessageID(ev Event, warmupID string) string {
	seed := strings.Join([]string{eventID(ev), warmupID}, "|")
	return fmt.Sprintf("gitclaw-channel-warmup-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelWarmupNotificationBody(opts ChannelWarmupOptions) string {
	prompts := channelWarmupPromptsForTheme(opts.Theme)
	var b strings.Builder
	b.WriteString("GitClaw channel warmup.\n\n")
	fmt.Fprintf(&b, "Theme: %s\n", opts.Theme)
	fmt.Fprintf(&b, "Frame: %s\n", channelWarmupFrame(opts.Theme))
	b.WriteString("Conversation starters:\n")
	for _, prompt := range prompts {
		fmt.Fprintf(&b, "- %s\n", prompt)
	}
	if opts.Note != "" {
		fmt.Fprintf(&b, "\nNote: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "\nWarmup hash: %s\n", shortDocumentHash(opts.Theme+"|"+strings.Join(prompts, "|")))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("Warmup persistence: advisory only; no durable channel state changed.\n")
	b.WriteString("\nWarmup source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Command execution: not performed by this action.\n")
	b.WriteString("Skill install: not performed by this action.\n")
	b.WriteString("Tool execution: not performed by this action.\n")
	b.WriteString("Backup payload read: not performed by this action.\n")
	b.WriteString("Soul body read: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Policy mutation: not performed by this action.\n")
	b.WriteString("Schedule creation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelWarmupFrame(theme string) string {
	switch cleanChannelWarmupTheme(theme) {
	case "focus":
		return "Turn a broad chat thread into one crisp next exchange."
	case "pairing":
		return "Help people enter the thread with roles, constraints, and a useful first question."
	case "triage":
		return "Sort loose inputs before converting anything into GitHub work."
	case "design":
		return "Invite concrete product judgment before implementation momentum takes over."
	case "launch":
		return "Surface readiness, blockers, and owner clarity before a release moves."
	case "retro":
		return "Make reflection specific enough to preserve as issues or follow-up work."
	case "tools":
		return "Turn tool energy into reviewed requests before execution."
	case "soul":
		return "Discuss high-authority context without writing soul or memory files."
	case "backups":
		return "Review recovery confidence without reading payloads or restoring files."
	case "fun":
		return "Make the thread easier to enter without losing the GitHub-native audit trail."
	default:
		return "Keep the thread explicit and review-first."
	}
}

func channelWarmupPromptsForTheme(theme string) []string {
	switch cleanChannelWarmupTheme(theme) {
	case "focus":
		return []string{
			"What is the one decision this thread should make next?",
			"What context would make the next reply ten minutes faster?",
			"What should become a GitHub issue if we stop here?",
		}
	case "pairing":
		return []string{
			"Who should answer first, and who should only review?",
			"What constraint should everyone keep in view?",
			"What would make this worth turning into a huddle or room?",
		}
	case "triage":
		return []string{
			"What is a task, what is an open loop, and what is just context?",
			"Which item needs an owner before the thread grows?",
			"What can be safely ignored for now?",
		}
	case "design":
		return []string{
			"What user moment should this improve?",
			"What tradeoff are we willing to make visible?",
			"What screenshot, sketch, or example would settle the next choice?",
		}
	case "launch":
		return []string{
			"What has to be true before this ships?",
			"What is the smallest rollback or recovery signal we need?",
			"Who owns the next visible status update?",
		}
	case "retro":
		return []string{
			"What surprised us in this thread?",
			"What should we repeat next time?",
			"What should become a durable note, checklist, or playbook?",
		}
	case "tools":
		return []string{
			"What decision needs a reviewed tool run, and what evidence would make it safe?",
			"Which tool result would change the next step?",
			"What must stay human-reviewed before execution?",
		}
	case "soul":
		return []string{
			"What high-authority context is relevant here?",
			"What should be discussed without writing SOUL or memory yet?",
			"What would make this safe to promote later?",
		}
	case "backups":
		return []string{
			"What recovery assumption should we verify first?",
			"What backup metadata would be enough before reading payloads?",
			"What restore request would need human review?",
		}
	case "fun":
		return []string{
			"What tiny win should we notice before moving on?",
			"What playful option would still leave a useful GitHub breadcrumb?",
			"What would make this thread easier for the next person to join?",
		}
	default:
		return nil
	}
}
