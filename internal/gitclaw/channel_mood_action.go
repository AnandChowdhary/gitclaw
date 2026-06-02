package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelMoodOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	MoodID            string
	Mood              string
	Intensity         int
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelMoodResult struct {
	Notification ChannelSendResult
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	NotifyHash   string
	MoodIDHash   string
	MoodHash     string
	NoteHash     string
	BodyHash     string
}

type ChannelMoodActionRequest struct {
	Options             ChannelMoodOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoMoodID          bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	MoodIDHash          string
	MoodSHA             string
	MoodBytes           int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	NotificationBodySHA string
}

func IsChannelMoodActionRequest(ev Event, cfg Config) bool {
	return isChannelMoodActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelMoodActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "mood", "vibe", "pulse", "energy", "presence":
		return true
	default:
		return false
	}
}

func BuildChannelMoodActionRequest(ev Event, cfg Config) (ChannelMoodActionRequest, error) {
	fields, trailing, ok := channelMoodActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelMoodActionRequest{}, fmt.Errorf("missing channel mood command")
	}
	req := ChannelMoodActionRequest{
		Options: ChannelMoodOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Intensity:         3,
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
				return ChannelMoodActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--mood-id", "--vibe-id", "--pulse-id", "--id":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MoodID = cleanChannelMoodID(fields[i+1])
			i++
		case "--mood", "--vibe", "--state", "--presence":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Mood = fields[i+1]
			i++
		case "--intensity", "--energy", "--level":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			intensity, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelMoodActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 5", field)
			}
			req.Options.Intensity = intensity
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelMoodActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelMoodActionRequest{}, fmt.Errorf("unknown channel mood argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelMoodIssueTargetIfPresent(ev, &req)
	if err := applyChannelMoodPositionals(&req, positional); err != nil {
		return ChannelMoodActionRequest{}, err
	}
	if err := applyChannelMoodIssueTarget(ev, &req); err != nil {
		return ChannelMoodActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelMoodTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.Mood) == "" {
		req.Options.Mood = defaultChannelMoodForSubcommand(req.Subcommand)
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelMoodSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.MoodID) == "" {
		req.Options.MoodID = autoChannelMoodID(ev, req.Options)
		req.AutoMoodID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelMoodNotifyMessageID(ev, req.Options.MoodID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelMoodOptions(req.Options)
	if err := validateChannelMoodActionRequestOptions(req.Options); err != nil {
		return ChannelMoodActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.MoodIDHash = shortDocumentHash(req.Options.MoodID)
	req.MoodSHA = shortDocumentHash(req.Options.Mood)
	req.MoodBytes = len(req.Options.Mood)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelMoodNotificationBody(req.Options))
	return req, nil
}

func RunChannelMood(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelMoodOptions) (ChannelMoodResult, error) {
	opts = normalizeChannelMoodOptions(opts)
	var err error
	opts, err = applyChannelMoodRoute(cfg, opts)
	if err != nil {
		return ChannelMoodResult{}, err
	}
	if err := validateChannelMoodOptions(opts); err != nil {
		return ChannelMoodResult{}, err
	}
	body := renderChannelMoodNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelMoodResult{}, fmt.Errorf("queue channel mood notification: %w", err)
	}
	return ChannelMoodResult{
		Notification: notification,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:   shortDocumentHash(opts.NotifyMessageID),
		MoodIDHash:   shortDocumentHash(opts.MoodID),
		MoodHash:     shortDocumentHash(opts.Mood),
		NoteHash:     shortDocumentHash(opts.Note),
		BodyHash:     shortDocumentHash(body),
	}, nil
}

func RenderChannelMoodActionReport(ev Event, req ChannelMoodActionRequest, result ChannelMoodResult) string {
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
	moodIDHash := result.MoodIDHash
	if moodIDHash == "" {
		moodIDHash = req.MoodIDHash
	}
	moodHash := result.MoodHash
	if moodHash == "" {
		moodHash = req.MoodSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Mood Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_mood_status: `%s`\n", status)
	fmt.Fprintf(&b, "- mood_mode: `%s`\n", "structured-channel-presence")
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
	fmt.Fprintf(&b, "- mood_id_sha256_12: `%s`\n", noneIfEmpty(moodIDHash))
	fmt.Fprintf(&b, "- mood_id_auto: `%t`\n", req.AutoMoodID)
	fmt.Fprintf(&b, "- mood_sha256_12: `%s`\n", noneIfEmpty(moodHash))
	fmt.Fprintf(&b, "- mood_bytes: `%d`\n", req.MoodBytes)
	fmt.Fprintf(&b, "- mood_intensity_level: `%d`\n", req.Options.Intensity)
	fmt.Fprintf(&b, "- mood_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- mood_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- mood_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- mood_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mood_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mood_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mood_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_mood_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel mood update on the canonical channel issue. This keeps channel conversations more alive than reports: people can mark a compact presence signal while the source receipt keeps thread ids, message ids, mood ids, notes, and channel bodies out of band. The action does not call a model, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read mood updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent mood updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate mood updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelMoodActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelMoodActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelMoodIssueTarget(ev Event, req *ChannelMoodActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel mood requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelMoodIssueTargetIfPresent(ev Event, req *ChannelMoodActionRequest) {
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

func applyChannelMoodPositionals(req *ChannelMoodActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Mood == "" {
				req.Options.Mood = value
				continue
			}
			return fmt.Errorf("unexpected channel mood argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Mood == "" {
			req.Options.Mood = value
			continue
		}
		return fmt.Errorf("unexpected channel mood argument %q", value)
	}
	return nil
}

func normalizeChannelMoodOptions(opts ChannelMoodOptions) ChannelMoodOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.MoodID = cleanChannelMoodID(opts.MoodID)
	opts.Mood = cleanChannelMood(opts.Mood)
	opts.Note = cleanChannelMoodNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelMoodRoute(cfg Config, opts ChannelMoodOptions) (ChannelMoodOptions, error) {
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
		Body:      "GitClaw channel mood.",
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

func validateChannelMoodOptions(opts ChannelMoodOptions) error {
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
	if opts.MoodID == "" {
		return fmt.Errorf("missing mood id")
	}
	if opts.Mood == "" {
		return fmt.Errorf("missing mood")
	}
	if opts.Intensity < 1 || opts.Intensity > 5 {
		return fmt.Errorf("channel mood intensity must be between 1 and 5")
	}
	return nil
}

func validateChannelMoodActionRequestOptions(opts ChannelMoodOptions) error {
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
	if opts.MoodID == "" {
		return fmt.Errorf("missing mood id")
	}
	if opts.Mood == "" {
		return fmt.Errorf("missing mood")
	}
	if opts.Intensity < 1 || opts.Intensity > 5 {
		return fmt.Errorf("channel mood intensity must be between 1 and 5")
	}
	return nil
}

func cleanChannelMoodID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelMood(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if len(value) > 48 {
		value = strings.Trim(value[:48], "-")
	}
	return value
}

func cleanChannelMoodNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelMoodTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelMoodNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func defaultChannelMoodForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "pulse", "presence":
		return "present"
	case "energy":
		return "steady"
	default:
		return ""
	}
}

func autoChannelMoodSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-mood-source-%s", eventID(ev))
}

func autoChannelMoodID(ev Event, opts ChannelMoodOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Mood, strconv.Itoa(opts.Intensity), opts.Note}, "|")
	return fmt.Sprintf("mood-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelMoodNotifyMessageID(ev Event, moodID string) string {
	seed := strings.Join([]string{eventID(ev), moodID}, "|")
	return fmt.Sprintf("gitclaw-channel-mood-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelMoodNotificationBody(opts ChannelMoodOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel mood.\n\n")
	fmt.Fprintf(&b, "Mood: %s\n", opts.Mood)
	fmt.Fprintf(&b, "Intensity: %d/5\n", opts.Intensity)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "Mood hash: %s\n", shortDocumentHash(opts.Mood))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nPresence source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
