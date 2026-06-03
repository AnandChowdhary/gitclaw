package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPostcardOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	PostcardID        string
	Title             string
	Caption           string
	Tone              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelPostcardResult struct {
	Notification   ChannelSendResult
	RouteName      string
	RouteHash      string
	Channel        string
	ThreadHash     string
	MessageHash    string
	NotifyHash     string
	PostcardIDHash string
	TitleHash      string
	CaptionHash    string
	ToneHash       string
	BodyHash       string
}

type ChannelPostcardActionRequest struct {
	Options             ChannelPostcardOptions
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoPostcardID      bool
	TargetFromIssue     bool
	TitleSource         string
	CaptionSource       string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	PostcardIDHash      string
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	CaptionSHA          string
	CaptionBytes        int
	CaptionLines        int
	ToneSHA             string
	ToneBytes           int
	NotificationBodySHA string
}

func IsChannelPostcardActionRequest(ev Event, cfg Config) bool {
	return isChannelPostcardActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPostcardActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "postcard", "postcards", "scene-card", "field-note", "wish-you-were-here":
		return true
	default:
		return false
	}
}

func BuildChannelPostcardActionRequest(ev Event, cfg Config) (ChannelPostcardActionRequest, error) {
	fields, trailing, ok := channelPostcardActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPostcardActionRequest{}, fmt.Errorf("missing channel postcard command")
	}
	req := ChannelPostcardActionRequest{
		Options: ChannelPostcardOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			Tone:              defaultChannelPostcardToneForSubcommand(fields[1]),
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
				return ChannelPostcardActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--postcard-id", "--scene-id", "--field-note-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PostcardID = cleanChannelPostcardID(fields[i+1])
			i++
		case "--postcard", "--title", "--headline", "--scene", "--place":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Title = fields[i+1]
			req.TitleSource = "flag"
			i++
		case "--caption", "--note", "--because":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Caption = fields[i+1]
			req.CaptionSource = "flag"
			i++
		case "--tone", "--style":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Tone = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPostcardActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPostcardActionRequest{}, fmt.Errorf("unknown channel postcard argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	applyChannelPostcardIssueTargetIfPresent(ev, &req)
	if err := applyChannelPostcardPositionals(&req, positional); err != nil {
		return ChannelPostcardActionRequest{}, err
	}
	if err := applyChannelPostcardIssueTarget(ev, &req); err != nil {
		return ChannelPostcardActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.Title) == "" {
		req.Options.Title = parseChannelPostcardTrailingTitle(trailing)
		if req.Options.Title != "" {
			req.TitleSource = "trailing-title"
		}
	}
	if strings.TrimSpace(req.Options.Caption) == "" {
		req.Options.Caption = parseChannelPostcardTrailingCaption(trailing)
		if req.Options.Caption != "" {
			req.CaptionSource = "trailing-caption"
		}
	}
	if strings.TrimSpace(req.Options.Title) == "" {
		req.Options.Title = defaultChannelPostcardTitleForSubcommand(req.Subcommand)
		req.TitleSource = "default"
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelPostcardSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.PostcardID) == "" {
		req.Options.PostcardID = autoChannelPostcardID(ev, req.Options)
		req.AutoPostcardID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPostcardNotifyMessageID(ev, req.Options.PostcardID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPostcardOptions(req.Options)
	if err := validateChannelPostcardActionRequestOptions(req.Options); err != nil {
		return ChannelPostcardActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.PostcardIDHash = shortDocumentHash(req.Options.PostcardID)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.CaptionSHA = shortDocumentHash(req.Options.Caption)
	req.CaptionBytes = len(req.Options.Caption)
	req.CaptionLines = lineCount(req.Options.Caption)
	req.ToneSHA = shortDocumentHash(req.Options.Tone)
	req.ToneBytes = len(req.Options.Tone)
	req.NotificationBodySHA = shortDocumentHash(renderChannelPostcardNotificationBody(req.Options))
	return req, nil
}

func RunChannelPostcard(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPostcardOptions) (ChannelPostcardResult, error) {
	opts = normalizeChannelPostcardOptions(opts)
	var err error
	opts, err = applyChannelPostcardRoute(cfg, opts)
	if err != nil {
		return ChannelPostcardResult{}, err
	}
	if err := validateChannelPostcardOptions(opts); err != nil {
		return ChannelPostcardResult{}, err
	}
	body := renderChannelPostcardNotificationBody(opts)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelPostcardResult{}, fmt.Errorf("queue channel postcard notification: %w", err)
	}
	return ChannelPostcardResult{
		Notification:   notification,
		RouteName:      opts.Route,
		RouteHash:      channelRouteHash(opts.Route),
		Channel:        opts.Channel,
		ThreadHash:     shortDocumentHash(opts.ThreadID),
		MessageHash:    shortDocumentHash(opts.SourceMessageID),
		NotifyHash:     shortDocumentHash(opts.NotifyMessageID),
		PostcardIDHash: shortDocumentHash(opts.PostcardID),
		TitleHash:      shortDocumentHash(opts.Title),
		CaptionHash:    shortDocumentHash(opts.Caption),
		ToneHash:       shortDocumentHash(opts.Tone),
		BodyHash:       shortDocumentHash(body),
	}, nil
}

func RenderChannelPostcardActionReport(ev Event, req ChannelPostcardActionRequest, result ChannelPostcardResult) string {
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
	postcardIDHash := result.PostcardIDHash
	if postcardIDHash == "" {
		postcardIDHash = req.PostcardIDHash
	}
	titleHash := result.TitleHash
	if titleHash == "" {
		titleHash = req.TitleSHA
	}
	captionHash := result.CaptionHash
	if captionHash == "" {
		captionHash = req.CaptionSHA
	}
	toneHash := result.ToneHash
	if toneHash == "" {
		toneHash = req.ToneSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Postcard Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_postcard_status: `%s`\n", status)
	fmt.Fprintf(&b, "- postcard_mode: `%s`\n", "provider-facing-scene-card")
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
	fmt.Fprintf(&b, "- postcard_id_sha256_12: `%s`\n", noneIfEmpty(postcardIDHash))
	fmt.Fprintf(&b, "- postcard_id_auto: `%t`\n", req.AutoPostcardID)
	fmt.Fprintf(&b, "- postcard_title_sha256_12: `%s`\n", noneIfEmpty(titleHash))
	fmt.Fprintf(&b, "- postcard_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- postcard_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- postcard_title_source: `%s`\n", noneIfEmpty(req.TitleSource))
	fmt.Fprintf(&b, "- postcard_caption_sha256_12: `%s`\n", noneIfEmpty(captionHash))
	fmt.Fprintf(&b, "- postcard_caption_bytes: `%d`\n", req.CaptionBytes)
	fmt.Fprintf(&b, "- postcard_caption_lines: `%d`\n", req.CaptionLines)
	fmt.Fprintf(&b, "- postcard_caption_source: `%s`\n", noneIfEmpty(req.CaptionSource))
	fmt.Fprintf(&b, "- postcard_tone_sha256_12: `%s`\n", noneIfEmpty(toneHash))
	fmt.Fprintf(&b, "- postcard_tone_bytes: `%d`\n", req.ToneBytes)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- image_generation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- media_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_api_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_postcard_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_postcard_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_postcard_caption_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_postcard_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-facing channel postcard on the canonical channel issue. This is a tiny scene-card lane: Slack or Telegram can receive a compact place/caption postcard while the source receipt keeps thread ids, message ids, postcard ids, titles, captions, and channel bodies out of band. The action does not call a model, generate images, fetch media, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read postcard cards with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent postcard cards with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate postcard cards are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func channelPostcardActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPostcardActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPostcardIssueTarget(ev Event, req *ChannelPostcardActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel postcard requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func applyChannelPostcardIssueTargetIfPresent(ev Event, req *ChannelPostcardActionRequest) {
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

func applyChannelPostcardPositionals(req *ChannelPostcardActionRequest, positional []string) error {
	if req == nil {
		return nil
	}
	for _, value := range positional {
		if value == "" {
			continue
		}
		if req.TargetFromIssue {
			if req.Options.Title == "" {
				req.Options.Title = value
				req.TitleSource = "positional-title"
				continue
			}
			return fmt.Errorf("unexpected channel postcard argument %q", value)
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Title == "" {
			req.Options.Title = value
			req.TitleSource = "positional-title"
			continue
		}
		return fmt.Errorf("unexpected channel postcard argument %q", value)
	}
	return nil
}

func normalizeChannelPostcardOptions(opts ChannelPostcardOptions) ChannelPostcardOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.PostcardID = cleanChannelPostcardID(opts.PostcardID)
	opts.Title = cleanChannelPostcardText(opts.Title, 160)
	opts.Caption = cleanChannelPostcardText(opts.Caption, 240)
	opts.Tone = cleanChannelPostcardTone(opts.Tone)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelPostcardRoute(cfg Config, opts ChannelPostcardOptions) (ChannelPostcardOptions, error) {
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
		Body:      "GitClaw channel postcard.",
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

func validateChannelPostcardOptions(opts ChannelPostcardOptions) error {
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
	if opts.PostcardID == "" {
		return fmt.Errorf("missing postcard id")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing postcard title")
	}
	if opts.Tone == "" {
		return fmt.Errorf("missing postcard tone")
	}
	return nil
}

func validateChannelPostcardActionRequestOptions(opts ChannelPostcardOptions) error {
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
	if opts.PostcardID == "" {
		return fmt.Errorf("missing postcard id")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing postcard title")
	}
	if opts.Tone == "" {
		return fmt.Errorf("missing postcard tone")
	}
	return nil
}

func cleanChannelPostcardID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelPostcardText(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if maxLen > 0 && len(value) > maxLen {
		value = strings.TrimSpace(value[:maxLen])
	}
	return value
}

func cleanChannelPostcardTone(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "bright"
	}
	if len(value) > 32 {
		value = strings.Trim(value[:32], "-")
	}
	return value
}

func parseChannelPostcardTrailingTitle(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelPostcardTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"postcard:", "title:", "headline:", "scene:", "place:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelPostcardText(trimmed[idx+1:], 160)
				}
			}
		}
		return cleanChannelPostcardText(trimmed, 160)
	}
	return ""
}

func parseChannelPostcardTrailingCaption(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if shouldSkipChannelPostcardTrailingLine(trimmed) {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, prefix := range []string{"caption:", "note:", "because:", "context:"} {
			if strings.HasPrefix(lower, prefix) {
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return cleanChannelPostcardText(trimmed[idx+1:], 240)
				}
			}
		}
	}
	return ""
}

func shouldSkipChannelPostcardTrailingLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "do not ") ||
		strings.HasPrefix(lower, "hidden ") ||
		strings.Contains(lower, "hidden token")
}

func defaultChannelPostcardToneForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "field-note":
		return "observant"
	case "wish-you-were-here":
		return "warm"
	default:
		return "bright"
	}
}

func defaultChannelPostcardTitleForSubcommand(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "postcards":
		return "Tiny scene"
	case "scene-card":
		return "Scene card"
	case "field-note":
		return "Field note"
	case "wish-you-were-here":
		return "Wish you were here"
	default:
		return "A small postcard"
	}
}

func autoChannelPostcardSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-postcard-source-%s", eventID(ev))
}

func autoChannelPostcardID(ev Event, opts ChannelPostcardOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Title, opts.Caption, opts.Tone}, "|")
	return fmt.Sprintf("postcard-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPostcardNotifyMessageID(ev Event, postcardID string) string {
	seed := strings.Join([]string{eventID(ev), postcardID}, "|")
	return fmt.Sprintf("gitclaw-channel-postcard-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelPostcardNotificationBody(opts ChannelPostcardOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel postcard.\n\n")
	fmt.Fprintf(&b, "Postcard: %s\n", opts.Title)
	fmt.Fprintf(&b, "Tone: %s\n", opts.Tone)
	if opts.Caption != "" {
		fmt.Fprintf(&b, "Caption: %s\n", opts.Caption)
	}
	fmt.Fprintf(&b, "Postcard hash: %s\n", shortDocumentHash(opts.Title))
	if opts.Caption != "" {
		fmt.Fprintf(&b, "Caption hash: %s\n", shortDocumentHash(opts.Caption))
	}
	b.WriteString("\nPostcard source: GitHub channel action.\n")
	b.WriteString("Model call: not performed by this action.\n")
	b.WriteString("Image generation: not performed by this action.\n")
	b.WriteString("Media fetch: not performed by this action.\n")
	b.WriteString("Provider API call: not performed by this action.\n")
	b.WriteString("Workflow mutation: not performed by this action.\n")
	b.WriteString("Repository mutation: not performed by this action.\n")
	b.WriteString("Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}
