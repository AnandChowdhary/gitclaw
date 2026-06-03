package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelStickerOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	StickerID         string
	Sticker           string
	Scale             int
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelStickerResult struct {
	Notification  ChannelSendResult
	RouteName     string
	RouteHash     string
	Channel       string
	ThreadHash    string
	MessageHash   string
	NotifyHash    string
	StickerIDHash string
	StickerHash   string
	NoteHash      string
	BodyHash      string
}

type ChannelStickerActionRequest struct {
	Options             ChannelStickerOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoStickerID       bool
	TargetFromIssue     bool
	NoteSource          string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	StickerIDHash       string
	StickerSHA          string
	StickerBytes        int
	NoteSHA             string
	NoteBytes           int
	NoteLines           int
	NotificationBodySHA string
}

func IsChannelStickerActionRequest(ev Event, cfg Config) bool {
	return isChannelStickerActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelStickerActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "sticker", "stamp", "badge", "confetti", "celebrate":
		return true
	default:
		return false
	}
}

func BuildChannelStickerActionRequest(ev Event, cfg Config) (ChannelStickerActionRequest, error) {
	fields, trailing, ok := channelStickerActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelStickerActionRequest{}, fmt.Errorf("missing channel sticker command")
	}
	req := ChannelStickerActionRequest{
		Options: ChannelStickerOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Scale:             3,
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
				return ChannelStickerActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--sticker-id", "--stamp-id", "--badge-id", "--spark-id", "--id":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StickerID = cleanChannelStickerID(fields[i+1])
			i++
		case "--sticker", "--stamp", "--badge", "--name":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Sticker = fields[i+1]
			i++
		case "--scale", "--size", "--level":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			scale, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelStickerActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 5", field)
			}
			req.Options.Scale = scale
			i++
		case "--note":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("--note requires a value")
			}
			req.Options.Note = fields[i+1]
			req.NoteSource = "flag"
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelStickerActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelStickerActionRequest{}, fmt.Errorf("unknown channel sticker argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelStickerIssueTargetIfPresent(ev, &req)
	if err := applyChannelStickerPositionals(&req, positional); err != nil {
		return ChannelStickerActionRequest{}, err
	}
	if err := applyChannelStickerIssueTarget(ev, &req); err != nil {
		return ChannelStickerActionRequest{}, err
	}
	if req.Options.Note == "" {
		req.Options.Note = parseChannelStickerTrailingNote(trailing)
		if req.Options.Note != "" {
			req.NoteSource = "trailing-note"
		}
	}
	if strings.TrimSpace(req.Options.Sticker) == "" {
		req.Options.Sticker = defaultChannelStickerForSubcommand(req.Subcommand)
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelStickerSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.StickerID) == "" {
		req.Options.StickerID = autoChannelStickerID(ev, req.Options)
		req.AutoStickerID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelStickerNotifyMessageID(ev, req.Options.StickerID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelStickerOptions(req.Options)
	if err := validateChannelStickerActionRequestOptions(req.Options); err != nil {
		return ChannelStickerActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.StickerIDHash = shortDocumentHash(req.Options.StickerID)
	req.StickerSHA = shortDocumentHash(req.Options.Sticker)
	req.StickerBytes = len(req.Options.Sticker)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.NotificationBodySHA = shortDocumentHash(renderChannelStickerNotificationBody(req.Options))
	return req, nil
}

func RunChannelSticker(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelStickerOptions) (ChannelStickerResult, error) {
	opts = normalizeChannelStickerOptions(opts)
	var err error
	opts, err = applyChannelStickerRoute(cfg, opts)
	if err != nil {
		return ChannelStickerResult{}, err
	}
	if err := validateChannelStickerOptions(opts); err != nil {
		return ChannelStickerResult{}, err
	}
	body := renderChannelStickerNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelStickerResult{}, fmt.Errorf("queue channel sticker notification: %w", err)
	}
	return ChannelStickerResult{
		Notification:  notification,
		RouteName:     opts.Route,
		RouteHash:     channelRouteHash(opts.Route),
		Channel:       opts.Channel,
		ThreadHash:    shortDocumentHash(opts.ThreadID),
		MessageHash:   shortDocumentHash(opts.SourceMessageID),
		NotifyHash:    shortDocumentHash(opts.NotifyMessageID),
		StickerIDHash: shortDocumentHash(opts.StickerID),
		StickerHash:   shortDocumentHash(opts.Sticker),
		NoteHash:      shortDocumentHash(opts.Note),
		BodyHash:      shortDocumentHash(body),
	}, nil
}

func RenderChannelStickerActionReport(ev Event, req ChannelStickerActionRequest, result ChannelStickerResult) string {
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
	stickerIDHash := result.StickerIDHash
	if stickerIDHash == "" {
		stickerIDHash = req.StickerIDHash
	}
	stickerHash := result.StickerHash
	if stickerHash == "" {
		stickerHash = req.StickerSHA
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
	b.WriteString("## GitClaw Channel Sticker Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_sticker_status: `%s`\n", status)
	fmt.Fprintf(&b, "- sticker_mode: `%s`\n", "structured-channel-sticker")
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
	fmt.Fprintf(&b, "- sticker_id_sha256_12: `%s`\n", noneIfEmpty(stickerIDHash))
	fmt.Fprintf(&b, "- sticker_id_auto: `%t`\n", req.AutoStickerID)
	fmt.Fprintf(&b, "- sticker_sha256_12: `%s`\n", noneIfEmpty(stickerHash))
	fmt.Fprintf(&b, "- sticker_bytes: `%d`\n", req.StickerBytes)
	fmt.Fprintf(&b, "- sticker_scale_level: `%d`\n", req.Options.Scale)
	fmt.Fprintf(&b, "- sticker_note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- sticker_note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- sticker_note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- sticker_note_source: `%s`\n", noneIfEmpty(req.NoteSource))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- image_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- media_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- file_upload_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_sticker_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_sticker_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_sticker_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_sticker_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel sticker on the canonical channel issue. This keeps channel conversations more alive than reports: people can add a compact sticker signal while the source receipt keeps thread ids, message ids, sticker ids, notes, and channel bodies out of band. The action does not call a model, generate images, fetch media, upload files, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read sticker updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent sticker updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate sticker updates are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelStickerActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelStickerActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelStickerIssueTarget(ev Event, req *ChannelStickerActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel sticker requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelStickerIssueTargetIfPresent(ev Event, req *ChannelStickerActionRequest) {
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

func applyChannelStickerPositionals(req *ChannelStickerActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Sticker == "" {
				req.Options.Sticker = value
				continue
			}
			return fmt.Errorf("unexpected channel sticker argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Sticker == "" {
			req.Options.Sticker = value
			continue
		}
		return fmt.Errorf("unexpected channel sticker argument %q", value)
	}
	return nil
}

func normalizeChannelStickerOptions(opts ChannelStickerOptions) ChannelStickerOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.StickerID = cleanChannelStickerID(opts.StickerID)
	opts.Sticker = cleanChannelSticker(opts.Sticker)
	opts.Note = cleanChannelStickerNote(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelStickerRoute(cfg Config, opts ChannelStickerOptions) (ChannelStickerOptions, error) {
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
		Body:      "GitClaw channel sticker.",
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

func validateChannelStickerOptions(opts ChannelStickerOptions) error {
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
	if opts.StickerID == "" {
		return fmt.Errorf("missing sticker id")
	}
	if opts.Sticker == "" {
		return fmt.Errorf("missing sticker")
	}
	if opts.Scale < 1 || opts.Scale > 5 {
		return fmt.Errorf("channel sticker scale must be between 1 and 5")
	}
	return nil
}

func validateChannelStickerActionRequestOptions(opts ChannelStickerOptions) error {
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
	if opts.StickerID == "" {
		return fmt.Errorf("missing sticker id")
	}
	if opts.Sticker == "" {
		return fmt.Errorf("missing sticker")
	}
	if opts.Scale < 1 || opts.Scale > 5 {
		return fmt.Errorf("channel sticker scale must be between 1 and 5")
	}
	return nil
}

func cleanChannelStickerID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSticker(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if len(value) > 48 {
		value = strings.Trim(value[:48], "-")
	}
	return value
}

func cleanChannelStickerNote(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 240 {
		value = strings.TrimSpace(value[:240])
	}
	return value
}

func parseChannelStickerTrailingNote(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "context:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelStickerNote(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func defaultChannelStickerForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "badge":
		return "badge"
	case "stamp":
		return "stamp"
	case "spark":
		return "spark"
	case "confetti", "celebrate":
		return "confetti"
	default:
		return ""
	}
}

func autoChannelStickerSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-sticker-source-%s", eventID(ev))
}

func autoChannelStickerID(ev Event, opts ChannelStickerOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Sticker, strconv.Itoa(opts.Scale), opts.Note}, "|")
	return fmt.Sprintf("sticker-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelStickerNotifyMessageID(ev Event, stickerID string) string {
	seed := strings.Join([]string{eventID(ev), stickerID}, "|")
	return fmt.Sprintf("gitclaw-channel-sticker-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelStickerNotificationBody(opts ChannelStickerOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel sticker.\n\n")
	fmt.Fprintf(&b, "Sticker: %s\n", opts.Sticker)
	fmt.Fprintf(&b, "Scale: %d/5\n", opts.Scale)
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", opts.Note)
	}
	fmt.Fprintf(&b, "Sticker hash: %s\n", shortDocumentHash(opts.Sticker))
	if opts.Note != "" {
		fmt.Fprintf(&b, "Note hash: %s\n", shortDocumentHash(opts.Note))
	}
	b.WriteString("\nSticker source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Image generation: not performed by this action.\n")
	b.WriteString("Media fetch: not performed by this action.\n")
	b.WriteString("File upload: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
