package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelImageOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ImageID           string
	Title             string
	Description       string
	Width             int
	Height            int
	MediaType         string
	SourceURL         string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelImageResult struct {
	ImageIssueNumber int
	ImageIssueURL    string
	ImageCreated     bool
	ImageDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelImageActionRequest struct {
	Options             ChannelImageOptions
	Command             string
	Subcommand          string
	AutoImageID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	DescriptionSHA      string
	DescriptionBytes    int
	DescriptionLines    int
	MediaTypeSHA        string
	MediaTypeBytes      int
	SourceURLSHA        string
	SourceURLBytes      int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
	DimensionsKnown     bool
}

func IsChannelImageActionRequest(ev Event, cfg Config) bool {
	return isChannelImageActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelImageActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "image", "photo", "picture", "screenshot", "screen", "visual", "diagram", "scan", "ocr", "image-note":
		return true
	default:
		return false
	}
}

func BuildChannelImageActionRequest(ev Event, cfg Config) (ChannelImageActionRequest, error) {
	fields, trailing, ok := channelImageActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelImageActionRequest{}, fmt.Errorf("missing channel image command")
	}
	req := ChannelImageActionRequest{
		Options: ChannelImageOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--image-id", "--photo-id", "--picture-id", "--screenshot-id", "--visual-id", "--id":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ImageID = cleanChannelImageID(fields[i+1])
			i++
		case "--width", "-w":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			width, err := parseChannelImageDimension(fields[i+1], "width")
			if err != nil {
				return ChannelImageActionRequest{}, err
			}
			req.Options.Width = width
			i++
		case "--height":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			height, err := parseChannelImageDimension(fields[i+1], "height")
			if err != nil {
				return ChannelImageActionRequest{}, err
			}
			req.Options.Height = height
			i++
		case "--media-type", "--mime-type", "--content-type", "--type":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MediaType = fields[i+1]
			i++
		case "--url", "--image-url", "--media-url", "--source-url":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceURL = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelImageActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelImageActionRequest{}, fmt.Errorf("unknown channel image argument %q", field)
			}
			if req.Options.ImageID == "" {
				req.Options.ImageID = cleanChannelImageID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelImageActionRequest{}, fmt.Errorf("unexpected channel image argument %q", field)
		}
	}
	if err := applyChannelImageIssueTarget(ev, &req); err != nil {
		return ChannelImageActionRequest{}, err
	}
	title, description := parseChannelImageTitleDescription(trailing, ev)
	req.Options.Title = title
	req.Options.Description = description
	if strings.TrimSpace(req.Options.ImageID) == "" {
		req.Options.ImageID = autoChannelImageID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, description)
		req.AutoImageID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelImageNotifyMessageID(ev, req.Options.ImageID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelImageOptions(req.Options)
	if err := validateChannelImageActionRequestOptions(req.Options); err != nil {
		return ChannelImageActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.DescriptionSHA = shortDocumentHash(req.Options.Description)
	req.DescriptionBytes = len(req.Options.Description)
	req.DescriptionLines = lineCount(req.Options.Description)
	req.MediaTypeSHA = shortDocumentHash(req.Options.MediaType)
	req.MediaTypeBytes = len(req.Options.MediaType)
	req.SourceURLSHA = shortDocumentHash(req.Options.SourceURL)
	req.SourceURLBytes = len(req.Options.SourceURL)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelImageNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	req.DimensionsKnown = req.Options.Width > 0 || req.Options.Height > 0
	return req, nil
}

func RunChannelImage(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelImageOptions) (ChannelImageResult, error) {
	opts = normalizeChannelImageOptions(opts)
	var err error
	opts, err = applyChannelImageRoute(cfg, opts)
	if err != nil {
		return ChannelImageResult{}, err
	}
	if err := validateChannelImageOptions(opts); err != nil {
		return ChannelImageResult{}, err
	}
	imageIssue, created, duplicate, err := findOrCreateChannelImageIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelImageResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelImageNotificationBody(opts, imageIssue.Number, issueURL(opts.Repo, imageIssue.Number)),
	})
	if err != nil {
		return ChannelImageResult{}, fmt.Errorf("queue channel image notification: %w", err)
	}
	return ChannelImageResult{
		ImageIssueNumber: imageIssue.Number,
		ImageIssueURL:    issueURL(opts.Repo, imageIssue.Number),
		ImageCreated:     created,
		ImageDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelImageActionReport(ev Event, req ChannelImageActionRequest, result ChannelImageResult) string {
	status := "captured"
	switch {
	case result.ImageDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ImageDuplicate:
		status = "existing"
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
	var b strings.Builder
	b.WriteString("## GitClaw Channel Image Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_image_status: `%s`\n", status)
	fmt.Fprintf(&b, "- image_issue: `#%d`\n", result.ImageIssueNumber)
	fmt.Fprintf(&b, "- image_issue_url: `%s`\n", result.ImageIssueURL)
	fmt.Fprintf(&b, "- image_issue_created: `%t`\n", result.ImageCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ImageDuplicate)
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
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- image_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ImageID))
	fmt.Fprintf(&b, "- image_id_auto: `%t`\n", req.AutoImageID)
	fmt.Fprintf(&b, "- image_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- image_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- image_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- image_description_sha256_12: `%s`\n", req.DescriptionSHA)
	fmt.Fprintf(&b, "- image_description_bytes: `%d`\n", req.DescriptionBytes)
	fmt.Fprintf(&b, "- image_description_lines: `%d`\n", req.DescriptionLines)
	fmt.Fprintf(&b, "- image_width: `%d`\n", req.Options.Width)
	fmt.Fprintf(&b, "- image_height: `%d`\n", req.Options.Height)
	fmt.Fprintf(&b, "- image_dimensions_known: `%t`\n", req.DimensionsKnown)
	fmt.Fprintf(&b, "- media_type_sha256_12: `%s`\n", req.MediaTypeSHA)
	fmt.Fprintf(&b, "- media_type_bytes: `%d`\n", req.MediaTypeBytes)
	fmt.Fprintf(&b, "- source_url_sha256_12: `%s`\n", noneIfEmpty(req.SourceURLSHA))
	fmt.Fprintf(&b, "- source_url_bytes: `%d`\n", req.SourceURLBytes)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_image_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_image_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_image_description_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_media_type_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_url_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_image_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin image/photo as a durable GitHub visual context issue, then queued a provider-facing image link back to the mirrored thread. The image issue contains the human-readable title and description; this source receipt keeps provider IDs, source URLs, media metadata, descriptions, titles, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the image-note notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent image-note links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate image issues are suppressed by `image_id`; duplicate image-note notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the image issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelImageIssueBody(opts ChannelImageOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-image image_id=\"%s\" channel=\"%s\" media_type_sha256_12=\"%s\" source_url_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ImageID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.MediaType), shortDocumentHash(opts.SourceURL), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel image note.\n\n")
	fmt.Fprintf(&b, "- image_id: %s\n", opts.ImageID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- width: %d\n", opts.Width)
	fmt.Fprintf(&b, "- height: %d\n", opts.Height)
	fmt.Fprintf(&b, "- media_type_sha256_12: %s\n", shortDocumentHash(opts.MediaType))
	fmt.Fprintf(&b, "- source_url_sha256_12: %s\n", noneIfEmpty(shortDocumentHash(opts.SourceURL)))
	fmt.Fprintf(&b, "- image_mode: github-issue-visual-context\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_url_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Image Note\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Description) != "" {
		b.WriteString("\n\n## Description\n\n")
		b.WriteString(strings.TrimSpace(opts.Description))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for summarizing, tasking, searching, or following up on the channel image.")
	return strings.TrimSpace(b.String())
}

func channelImageActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelImageActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelImageIssueTarget(ev Event, req *ChannelImageActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel image requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelImageTitleDescription(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel image from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var title string
	var descriptionLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "title:"):
		title = strings.TrimSpace(first[len("title:"):])
		descriptionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "image:"):
		title = strings.TrimSpace(first[len("image:"):])
		descriptionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "photo:"):
		title = strings.TrimSpace(first[len("photo:"):])
		descriptionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "screenshot:"):
		title = strings.TrimSpace(first[len("screenshot:"):])
		descriptionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "description:"), strings.HasPrefix(lowerFirst, "caption:"), strings.HasPrefix(lowerFirst, "ocr:"), strings.HasPrefix(lowerFirst, "alt:"):
		title = defaultTitle
		descriptionLines = cleaned
	default:
		title = first
		descriptionLines = cleaned[1:]
	}
	if title == "" {
		title = defaultTitle
	}
	description := strings.TrimSpace(strings.Join(descriptionLines, "\n"))
	descriptionLower := strings.ToLower(strings.TrimSpace(description))
	switch {
	case strings.HasPrefix(descriptionLower, "description:"):
		description = strings.TrimSpace(strings.TrimSpace(description)[len("description:"):])
	case strings.HasPrefix(descriptionLower, "caption:"):
		description = strings.TrimSpace(strings.TrimSpace(description)[len("caption:"):])
	case strings.HasPrefix(descriptionLower, "ocr:"):
		description = strings.TrimSpace(strings.TrimSpace(description)[len("ocr:"):])
	case strings.HasPrefix(descriptionLower, "alt:"):
		description = strings.TrimSpace(strings.TrimSpace(description)[len("alt:"):])
	}
	return title, description
}

func normalizeChannelImageOptions(opts ChannelImageOptions) ChannelImageOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ImageID = cleanChannelImageID(opts.ImageID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Description = strings.TrimSpace(opts.Description)
	opts.MediaType = strings.ToLower(strings.TrimSpace(opts.MediaType))
	opts.SourceURL = strings.TrimSpace(opts.SourceURL)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.Width < 0 {
		opts.Width = 0
	}
	if opts.Height < 0 {
		opts.Height = 0
	}
	return opts
}

func applyChannelImageRoute(cfg Config, opts ChannelImageOptions) (ChannelImageOptions, error) {
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
		Body:      opts.Title,
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

func validateChannelImageOptions(opts ChannelImageOptions) error {
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
	if opts.ImageID == "" {
		return fmt.Errorf("missing image id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing image source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing image title")
	}
	return nil
}

func validateChannelImageActionRequestOptions(opts ChannelImageOptions) error {
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
	if opts.ImageID == "" {
		return fmt.Errorf("missing image id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing image source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing image title")
	}
	return nil
}

func findOrCreateChannelImageIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelImageOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel image issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelImageMatches(issue.Body, opts.ImageID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelImageIssueTitle(opts), RenderChannelImageIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel image issue: %w", err)
	}
	return issue, true, false, nil
}

func channelImageIssueTitle(opts ChannelImageOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.ImageID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel image: " + title
}

func channelImageMatches(body, imageID string) bool {
	return HasChannelImageMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`image_id="%s"`, escapeMarkerValue(cleanChannelImageID(imageID))))
}

func cleanChannelImageID(value string) string {
	return cleanChannelHuddleID(value)
}

func parseChannelImageDimension(value, name string) (int, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "pixels")
	value = strings.TrimSuffix(value, "pixel")
	value = strings.TrimSuffix(value, "px")
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("%s requires a positive pixel count", name)
	}
	dimension, err := strconv.Atoi(value)
	if err != nil || dimension < 0 {
		return 0, fmt.Errorf("invalid channel image %s %q", name, value)
	}
	return dimension, nil
}

func autoChannelImageID(ev Event, channel, threadID, sourceMessageID, title, description string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, description}, "|")
	return fmt.Sprintf("image-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelImageNotifyMessageID(ev Event, imageID string) string {
	seed := strings.Join([]string{eventID(ev), imageID}, "|")
	return fmt.Sprintf("gitclaw-channel-image-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelImageNotificationBody(opts ChannelImageOptions, imageIssueNumber int, imageIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel image note captured.\n\n")
	if imageIssueNumber > 0 {
		fmt.Fprintf(&b, "Image note: #%d\n", imageIssueNumber)
	}
	if imageIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", imageIssueURL)
	}
	if opts.Width > 0 || opts.Height > 0 {
		fmt.Fprintf(&b, "Dimensions: %dx%d\n", opts.Width, opts.Height)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue with the visual context in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
