package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelAttachmentOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	AttachmentID      string
	Filename          string
	MediaType         string
	Bytes             int64
	FileSHA256        string
	SourceURL         string
	Caption           string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelAttachmentResult struct {
	AttachmentIssueNumber int
	AttachmentIssueURL    string
	AttachmentCreated     bool
	AttachmentDuplicate   bool
	Notification          ChannelSendResult
	RouteName             string
	RouteHash             string
	Channel               string
	ThreadHash            string
	MessageHash           string
	NotifyHash            string
}

type ChannelAttachmentActionRequest struct {
	Options             ChannelAttachmentOptions
	Command             string
	Subcommand          string
	AutoAttachmentID    bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	FilenameSHA         string
	FilenameBytes       int
	FilenameLines       int
	MediaTypeSHA        string
	FileChecksumSHA     string
	SourceURLSHA        string
	CaptionSHA          string
	CaptionBytes        int
	CaptionLines        int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelAttachmentActionRequest(ev Event, cfg Config) bool {
	return isChannelAttachmentActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelAttachmentActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "attachment", "attach", "file", "media", "upload", "document":
		return true
	default:
		return false
	}
}

func BuildChannelAttachmentActionRequest(ev Event, cfg Config) (ChannelAttachmentActionRequest, error) {
	fields, trailing, ok := channelAttachmentActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelAttachmentActionRequest{}, fmt.Errorf("missing channel attachment command")
	}
	req := ChannelAttachmentActionRequest{
		Options: ChannelAttachmentOptions{
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
				return ChannelAttachmentActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--attachment-id", "--file-id", "--media-id", "--document-id", "--id":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.AttachmentID = cleanChannelAttachmentID(fields[i+1])
			i++
		case "--filename", "--file-name", "--name":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Filename = fields[i+1]
			i++
		case "--media-type", "--mime-type", "--content-type", "--type":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MediaType = fields[i+1]
			i++
		case "--bytes", "--size", "--size-bytes":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			bytes, err := parseChannelAttachmentBytes(fields[i+1])
			if err != nil {
				return ChannelAttachmentActionRequest{}, err
			}
			req.Options.Bytes = bytes
			i++
		case "--sha256", "--file-sha256", "--checksum":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.FileSHA256 = fields[i+1]
			i++
		case "--source-url", "--url":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceURL = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelAttachmentActionRequest{}, fmt.Errorf("unknown channel attachment argument %q", field)
			}
			if req.Options.AttachmentID == "" {
				req.Options.AttachmentID = cleanChannelAttachmentID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelAttachmentActionRequest{}, fmt.Errorf("unexpected channel attachment argument %q", field)
		}
	}
	if err := applyChannelAttachmentIssueTarget(ev, &req); err != nil {
		return ChannelAttachmentActionRequest{}, err
	}
	req.Options.Caption = parseChannelAttachmentCaption(trailing)
	if strings.TrimSpace(req.Options.AttachmentID) == "" {
		req.Options.AttachmentID = autoChannelAttachmentID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Filename, req.Options.FileSHA256)
		req.AutoAttachmentID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelAttachmentNotifyMessageID(ev, req.Options.AttachmentID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelAttachmentOptions(req.Options)
	if err := validateChannelAttachmentActionRequestOptions(req.Options); err != nil {
		return ChannelAttachmentActionRequest{}, err
	}
	req.FilenameSHA = shortDocumentHash(req.Options.Filename)
	req.FilenameBytes = len(req.Options.Filename)
	req.FilenameLines = lineCount(req.Options.Filename)
	req.MediaTypeSHA = shortDocumentHash(req.Options.MediaType)
	req.FileChecksumSHA = optionalChannelAttachmentHash(req.Options.FileSHA256)
	req.SourceURLSHA = optionalChannelAttachmentHash(req.Options.SourceURL)
	req.CaptionSHA = shortDocumentHash(req.Options.Caption)
	req.CaptionBytes = len(req.Options.Caption)
	req.CaptionLines = lineCount(req.Options.Caption)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelAttachmentNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelAttachment(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelAttachmentOptions) (ChannelAttachmentResult, error) {
	opts = normalizeChannelAttachmentOptions(opts)
	var err error
	opts, err = applyChannelAttachmentRoute(cfg, opts)
	if err != nil {
		return ChannelAttachmentResult{}, err
	}
	if err := validateChannelAttachmentOptions(opts); err != nil {
		return ChannelAttachmentResult{}, err
	}
	attachmentIssue, created, duplicate, err := findOrCreateChannelAttachmentIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelAttachmentResult{}, err
	}
	notify := ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelAttachmentNotificationBody(opts, attachmentIssue.Number, issueURL(opts.Repo, attachmentIssue.Number)),
	}
	notification, err := RunChannelSend(ctx, cfg, github, notify)
	if err != nil {
		return ChannelAttachmentResult{}, fmt.Errorf("queue channel attachment notification: %w", err)
	}
	return ChannelAttachmentResult{
		AttachmentIssueNumber: attachmentIssue.Number,
		AttachmentIssueURL:    issueURL(opts.Repo, attachmentIssue.Number),
		AttachmentCreated:     created,
		AttachmentDuplicate:   duplicate,
		Notification:          notification,
		RouteName:             opts.Route,
		RouteHash:             channelRouteHash(opts.Route),
		Channel:               opts.Channel,
		ThreadHash:            shortDocumentHash(opts.ThreadID),
		MessageHash:           shortDocumentHash(opts.SourceMessageID),
		NotifyHash:            shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelAttachmentActionReport(ev Event, req ChannelAttachmentActionRequest, result ChannelAttachmentResult) string {
	status := "recorded"
	switch {
	case result.AttachmentDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.AttachmentDuplicate:
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
	b.WriteString("## GitClaw Channel Attachment Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_attachment_status: `%s`\n", status)
	fmt.Fprintf(&b, "- attachment_issue: `#%d`\n", result.AttachmentIssueNumber)
	fmt.Fprintf(&b, "- attachment_issue_url: `%s`\n", result.AttachmentIssueURL)
	fmt.Fprintf(&b, "- attachment_issue_created: `%t`\n", result.AttachmentCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.AttachmentDuplicate)
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
	fmt.Fprintf(&b, "- attachment_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.AttachmentID))
	fmt.Fprintf(&b, "- attachment_id_auto: `%t`\n", req.AutoAttachmentID)
	fmt.Fprintf(&b, "- attachment_filename_sha256_12: `%s`\n", req.FilenameSHA)
	fmt.Fprintf(&b, "- attachment_filename_bytes: `%d`\n", req.FilenameBytes)
	fmt.Fprintf(&b, "- attachment_filename_lines: `%d`\n", req.FilenameLines)
	fmt.Fprintf(&b, "- attachment_media_type_sha256_12: `%s`\n", req.MediaTypeSHA)
	fmt.Fprintf(&b, "- attachment_bytes: `%d`\n", req.Options.Bytes)
	fmt.Fprintf(&b, "- file_checksum_sha256_12: `%s`\n", noneIfEmpty(req.FileChecksumSHA))
	fmt.Fprintf(&b, "- source_url_sha256_12: `%s`\n", noneIfEmpty(req.SourceURLSHA))
	fmt.Fprintf(&b, "- attachment_caption_sha256_12: `%s`\n", req.CaptionSHA)
	fmt.Fprintf(&b, "- attachment_caption_bytes: `%d`\n", req.CaptionBytes)
	fmt.Fprintf(&b, "- attachment_caption_lines: `%d`\n", req.CaptionLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- attachment_mode: `%s`\n", "github-issue-attachment-metadata")
	fmt.Fprintf(&b, "- attachment_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- provider_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_attachment_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_attachment_filename_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_attachment_caption_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_url_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_attachment_bytes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_attachment_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded channel-origin attachment metadata as a durable GitHub issue, then queued a provider-facing link back to the original thread. The attachment issue contains readable metadata; this source receipt keeps provider IDs, attachment IDs, filenames, captions, source URLs, file bytes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the attachment-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent attachment links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate attachment issues are suppressed by `attachment_id`; duplicate attachment-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the attachment issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelAttachmentIssueBody(opts ChannelAttachmentOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-attachment attachment_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" filename_sha256_12=\"%s\" file_checksum_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.AttachmentID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), shortDocumentHash(opts.Filename), optionalChannelAttachmentHash(opts.FileSHA256), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel attachment.\n\n")
	fmt.Fprintf(&b, "- attachment_id: %s\n", opts.AttachmentID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- filename: %s\n", opts.Filename)
	fmt.Fprintf(&b, "- filename_sha256_12: %s\n", shortDocumentHash(opts.Filename))
	fmt.Fprintf(&b, "- media_type: %s\n", opts.MediaType)
	fmt.Fprintf(&b, "- attachment_bytes: %d\n", opts.Bytes)
	fmt.Fprintf(&b, "- file_sha256: %s\n", noneIfEmpty(opts.FileSHA256))
	fmt.Fprintf(&b, "- source_url_sha256_12: %s\n", noneIfEmpty(optionalChannelAttachmentHash(opts.SourceURL)))
	fmt.Fprintf(&b, "- attachment_mode: github-issue-attachment-metadata\n")
	fmt.Fprintf(&b, "- attachment_bytes_included: false\n")
	fmt.Fprintf(&b, "- provider_fetch_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_url_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n")
	fmt.Fprintf(&b, "- raw_attachment_bytes_included: false\n\n")
	if strings.TrimSpace(opts.Caption) != "" {
		b.WriteString("## Caption\n\n")
		b.WriteString(strings.TrimSpace(opts.Caption))
		b.WriteString("\n\n")
	}
	b.WriteString("Use this issue as the durable GitHub home for the channel-origin attachment metadata. The file bytes were not fetched or copied by this action.")
	return strings.TrimSpace(b.String())
}

func channelAttachmentActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelAttachmentActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelAttachmentIssueTarget(ev Event, req *ChannelAttachmentActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel attachment requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelAttachmentCaption(trailing string) string {
	text := strings.TrimSpace(trailing)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, prefix := range []string{"caption:", "notes:", "description:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(text[len(prefix):])
		}
	}
	return text
}

func normalizeChannelAttachmentOptions(opts ChannelAttachmentOptions) ChannelAttachmentOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.AttachmentID = cleanChannelAttachmentID(opts.AttachmentID)
	opts.Filename = cleanChannelAttachmentFilename(opts.Filename)
	opts.MediaType = strings.ToLower(strings.TrimSpace(opts.MediaType))
	if opts.MediaType == "" {
		opts.MediaType = "application/octet-stream"
	}
	opts.FileSHA256 = cleanChannelAttachmentChecksum(opts.FileSHA256)
	opts.SourceURL = strings.TrimSpace(opts.SourceURL)
	opts.Caption = strings.TrimSpace(opts.Caption)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelAttachmentRoute(cfg Config, opts ChannelAttachmentOptions) (ChannelAttachmentOptions, error) {
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
		Body:      opts.Filename,
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

func validateChannelAttachmentOptions(opts ChannelAttachmentOptions) error {
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
	if opts.AttachmentID == "" {
		return fmt.Errorf("missing attachment id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing attachment source issue")
	}
	if opts.Filename == "" {
		return fmt.Errorf("missing attachment filename")
	}
	if opts.MediaType == "" {
		return fmt.Errorf("missing attachment media type")
	}
	if opts.Bytes < 0 {
		return fmt.Errorf("attachment bytes must be non-negative")
	}
	return nil
}

func validateChannelAttachmentActionRequestOptions(opts ChannelAttachmentOptions) error {
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
	if opts.AttachmentID == "" {
		return fmt.Errorf("missing attachment id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing attachment source issue")
	}
	if opts.Filename == "" {
		return fmt.Errorf("missing attachment filename")
	}
	if opts.MediaType == "" {
		return fmt.Errorf("missing attachment media type")
	}
	if opts.Bytes < 0 {
		return fmt.Errorf("attachment bytes must be non-negative")
	}
	return nil
}

func findOrCreateChannelAttachmentIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelAttachmentOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel attachment issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelAttachmentMatches(issue.Body, opts.AttachmentID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelAttachmentIssueTitle(opts), RenderChannelAttachmentIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel attachment issue: %w", err)
	}
	return issue, true, false, nil
}

func channelAttachmentIssueTitle(opts ChannelAttachmentOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Filename), "\n", " ")
	if title == "" {
		title = opts.AttachmentID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel attachment: " + title
}

func channelAttachmentMatches(body, attachmentID string) bool {
	return HasChannelAttachmentMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`attachment_id="%s"`, escapeMarkerValue(cleanChannelAttachmentID(attachmentID))))
}

func cleanChannelAttachmentID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelAttachmentFilename(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func cleanChannelAttachmentChecksum(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	return value
}

func optionalChannelAttachmentHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return shortDocumentHash(value)
}

func parseChannelAttachmentBytes(value string) (int64, error) {
	bytes, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("attachment bytes must be an integer: %w", err)
	}
	if bytes < 0 {
		return 0, fmt.Errorf("attachment bytes must be non-negative")
	}
	return bytes, nil
}

func autoChannelAttachmentID(ev Event, channel, threadID, sourceMessageID, filename, fileSHA string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, filename, fileSHA}, "|")
	return fmt.Sprintf("attachment-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelAttachmentNotifyMessageID(ev Event, attachmentID string) string {
	seed := strings.Join([]string{eventID(ev), attachmentID}, "|")
	return fmt.Sprintf("gitclaw-channel-attachment-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelAttachmentNotificationBody(opts ChannelAttachmentOptions, attachmentIssueNumber int, attachmentIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel attachment recorded.\n\n")
	if attachmentIssueNumber > 0 {
		fmt.Fprintf(&b, "Attachment: #%d\n", attachmentIssueNumber)
	}
	if attachmentIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", attachmentIssueURL)
	}
	fmt.Fprintf(&b, "Filename: %s\n", strings.TrimSpace(opts.Filename))
	fmt.Fprintf(&b, "Media type: %s\n", strings.TrimSpace(opts.MediaType))
	fmt.Fprintf(&b, "Size: %d bytes\n", opts.Bytes)
	b.WriteString("\nRecorded in the linked GitHub issue without fetching or copying file bytes.")
	return strings.TrimSpace(b.String())
}
