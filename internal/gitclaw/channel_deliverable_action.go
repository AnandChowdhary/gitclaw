package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelDeliverableOptions struct {
	Repo          string
	Route         string
	Channel       string
	ThreadID      string
	MessageID     string
	DeliverableID string
	Filename      string
	MediaType     string
	Bytes         int64
	FileSHA256    string
	URL           string
	Caption       string
	Author        string
}

type ChannelDeliverableResult struct {
	IssueNumber       int
	IssueURL          string
	CommentID         int64
	Created           bool
	Duplicate         bool
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	MessageHash       string
	DeliverableIDHash string
	BodyHash          string
}

type ChannelDeliverableActionRequest struct {
	Options                 ChannelDeliverableOptions
	Command                 string
	Subcommand              string
	AutoDeliverableID       bool
	AutoMessageID           bool
	TargetFromIssue         bool
	FilenameSHA             string
	FilenameBytes           int
	FilenameLines           int
	MediaTypeSHA            string
	FileChecksumSHA         string
	URLSHA                  string
	CaptionSHA              string
	CaptionBytes            int
	CaptionLines            int
	RequestedRouteHash      string
	RequestedThreadHash     string
	RequestedMessageHash    string
	RequestedDeliverableSHA string
	DeliverableBodySHA      string
}

func IsChannelDeliverableActionRequest(ev Event, cfg Config) bool {
	return isChannelDeliverableActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelDeliverableActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "deliverable", "deliver", "send-file", "share-file", "deliver-file", "media-deliver", "artifact-deliver":
		return true
	default:
		return false
	}
}

func BuildChannelDeliverableActionRequest(ev Event, cfg Config) (ChannelDeliverableActionRequest, error) {
	fields, trailing, ok := channelDeliverableActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelDeliverableActionRequest{}, fmt.Errorf("missing channel deliverable command")
	}
	req := ChannelDeliverableActionRequest{
		Options: ChannelDeliverableOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--outbound-message-id", "--notify-message-id":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--deliverable-id", "--file-id", "--artifact-id", "--id":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DeliverableID = cleanChannelDeliverableID(fields[i+1])
			i++
		case "--filename", "--file-name", "--name":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Filename = fields[i+1]
			i++
		case "--media-type", "--mime-type", "--content-type", "--type":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MediaType = fields[i+1]
			i++
		case "--bytes", "--size", "--size-bytes":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			bytes, err := parseChannelAttachmentBytes(fields[i+1])
			if err != nil {
				return ChannelDeliverableActionRequest{}, err
			}
			req.Options.Bytes = bytes
			i++
		case "--sha256", "--file-sha256", "--checksum":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.FileSHA256 = fields[i+1]
			i++
		case "--url", "--artifact-url", "--download-url":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.URL = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelDeliverableActionRequest{}, fmt.Errorf("unknown channel deliverable argument %q", field)
			}
			if req.Options.DeliverableID == "" {
				req.Options.DeliverableID = cleanChannelDeliverableID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelDeliverableActionRequest{}, fmt.Errorf("unexpected channel deliverable argument %q", field)
		}
	}
	if err := applyChannelDeliverableIssueTarget(ev, &req); err != nil {
		return ChannelDeliverableActionRequest{}, err
	}
	req.Options.Caption = parseChannelDeliverableCaption(trailing)
	if strings.TrimSpace(req.Options.DeliverableID) == "" {
		req.Options.DeliverableID = autoChannelDeliverableID(ev, req.Options)
		req.AutoDeliverableID = true
	}
	if strings.TrimSpace(req.Options.MessageID) == "" {
		req.Options.MessageID = autoChannelDeliverableMessageID(ev, req.Options.DeliverableID)
		req.AutoMessageID = true
	}
	req.Options = normalizeChannelDeliverableOptions(req.Options)
	if err := validateChannelDeliverableActionRequestOptions(req.Options); err != nil {
		return ChannelDeliverableActionRequest{}, err
	}
	req.FilenameSHA = shortDocumentHash(req.Options.Filename)
	req.FilenameBytes = len(req.Options.Filename)
	req.FilenameLines = lineCount(req.Options.Filename)
	req.MediaTypeSHA = shortDocumentHash(req.Options.MediaType)
	req.FileChecksumSHA = optionalChannelAttachmentHash(req.Options.FileSHA256)
	req.URLSHA = optionalChannelAttachmentHash(req.Options.URL)
	req.CaptionSHA = shortDocumentHash(req.Options.Caption)
	req.CaptionBytes = len(req.Options.Caption)
	req.CaptionLines = lineCount(req.Options.Caption)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMessageHash = shortDocumentHash(req.Options.MessageID)
	req.RequestedDeliverableSHA = shortDocumentHash(req.Options.DeliverableID)
	req.DeliverableBodySHA = shortDocumentHash(RenderChannelDeliverableBody(req.Options))
	return req, nil
}

func RunChannelDeliverable(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDeliverableOptions) (ChannelDeliverableResult, error) {
	opts = normalizeChannelDeliverableOptions(opts)
	var err error
	opts, err = applyChannelDeliverableRoute(cfg, opts)
	if err != nil {
		return ChannelDeliverableResult{}, err
	}
	if err := validateChannelDeliverableOptions(opts); err != nil {
		return ChannelDeliverableResult{}, err
	}
	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelDeliverableResult{}, err
	}
	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelDeliverableResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelDeliverableMatches(comment.Body, opts.Channel, opts.DeliverableID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelDeliverableResult{
				IssueNumber:       issue.Number,
				IssueURL:          issueURL(opts.Repo, issue.Number),
				Created:           created,
				Duplicate:         true,
				RouteName:         opts.Route,
				RouteHash:         channelRouteHash(opts.Route),
				Channel:           opts.Channel,
				ThreadHash:        shortDocumentHash(opts.ThreadID),
				MessageHash:       shortDocumentHash(opts.MessageID),
				DeliverableIDHash: shortDocumentHash(opts.DeliverableID),
				BodyHash:          shortDocumentHash(RenderChannelDeliverableBody(opts)),
			}, nil
		}
	}
	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelDeliverableComment(opts))
	if err != nil {
		return ChannelDeliverableResult{}, fmt.Errorf("post channel deliverable: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelDeliverableResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelDeliverableResult{
		IssueNumber:       issue.Number,
		IssueURL:          issueURL(opts.Repo, issue.Number),
		CommentID:         posted.ID,
		Created:           created,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		MessageHash:       shortDocumentHash(opts.MessageID),
		DeliverableIDHash: shortDocumentHash(opts.DeliverableID),
		BodyHash:          shortDocumentHash(RenderChannelDeliverableBody(opts)),
	}, nil
}

func RenderChannelDeliverableActionReport(ev Event, req ChannelDeliverableActionRequest, result ChannelDeliverableResult) string {
	status := "queued"
	if result.Duplicate {
		status = "duplicate"
	}
	deliverableQueued := result.CommentID != 0 && !result.Duplicate
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
		messageHash = req.RequestedMessageHash
	}
	deliverableHash := result.DeliverableIDHash
	if deliverableHash == "" {
		deliverableHash = req.RequestedDeliverableSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.DeliverableBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Deliverable Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_deliverable_status: `%s`\n", status)
	fmt.Fprintf(&b, "- deliverable_target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- deliverable_target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- deliverable_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- deliverable_queued: `%t`\n", deliverableQueued)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- deliverable_id_sha256_12: `%s`\n", noneIfEmpty(deliverableHash))
	fmt.Fprintf(&b, "- deliverable_id_auto: `%t`\n", req.AutoDeliverableID)
	fmt.Fprintf(&b, "- deliverable_filename_sha256_12: `%s`\n", req.FilenameSHA)
	fmt.Fprintf(&b, "- deliverable_filename_bytes: `%d`\n", req.FilenameBytes)
	fmt.Fprintf(&b, "- deliverable_filename_lines: `%d`\n", req.FilenameLines)
	fmt.Fprintf(&b, "- deliverable_media_type_sha256_12: `%s`\n", req.MediaTypeSHA)
	fmt.Fprintf(&b, "- deliverable_bytes: `%d`\n", req.Options.Bytes)
	fmt.Fprintf(&b, "- file_checksum_sha256_12: `%s`\n", noneIfEmpty(req.FileChecksumSHA))
	fmt.Fprintf(&b, "- deliverable_url_sha256_12: `%s`\n", noneIfEmpty(req.URLSHA))
	fmt.Fprintf(&b, "- deliverable_caption_sha256_12: `%s`\n", req.CaptionSHA)
	fmt.Fprintf(&b, "- deliverable_caption_bytes: `%d`\n", req.CaptionBytes)
	fmt.Fprintf(&b, "- deliverable_caption_lines: `%d`\n", req.CaptionLines)
	fmt.Fprintf(&b, "- deliverable_body_sha256_12: `%s`\n", noneIfEmpty(bodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- deliverable_mode: `%s`\n", "channel-outbox-native-deliverable")
	fmt.Fprintf(&b, "- provider_upload_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_deliverable_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_deliverable_filename_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_deliverable_caption_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_deliverable_url_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_checksum_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox --include-body + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_deliverable_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a provider-native channel deliverable as a structured GitHub comment. A gateway can fetch it through `gitclaw channel-outbox --include-body` and record provider upload/delivery with `gitclaw channel-delivery`; this receipt keeps deliverable IDs, filenames, captions, URLs, checksums, thread IDs, message IDs, and channel bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending deliverables with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --include-body --out <file>`\n")
	b.WriteString("- provider gateways record sent deliverables with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate deliverables are suppressed by `channel + deliverable_id`\n")
	return strings.TrimSpace(b.String())
}

func channelDeliverableActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelDeliverableActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelDeliverableIssueTarget(ev Event, req *ChannelDeliverableActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel deliverable requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelDeliverableCaption(trailing string) string {
	text := strings.TrimSpace(trailing)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, prefix := range []string{"caption:", "message:", "notes:", "description:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(text[len(prefix):])
		}
	}
	return text
}

func normalizeChannelDeliverableOptions(opts ChannelDeliverableOptions) ChannelDeliverableOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.DeliverableID = cleanChannelDeliverableID(opts.DeliverableID)
	opts.Filename = cleanChannelAttachmentFilename(opts.Filename)
	opts.MediaType = strings.ToLower(strings.TrimSpace(opts.MediaType))
	if opts.MediaType == "" {
		opts.MediaType = "application/octet-stream"
	}
	opts.FileSHA256 = cleanChannelAttachmentChecksum(opts.FileSHA256)
	opts.URL = strings.TrimSpace(opts.URL)
	opts.Caption = strings.TrimSpace(opts.Caption)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelDeliverableRoute(cfg Config, opts ChannelDeliverableOptions) (ChannelDeliverableOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.MessageID,
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

func validateChannelDeliverableOptions(opts ChannelDeliverableOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.MessageID == "" {
		return fmt.Errorf("missing deliverable message id")
	}
	if opts.DeliverableID == "" {
		return fmt.Errorf("missing deliverable id")
	}
	if opts.Filename == "" {
		return fmt.Errorf("missing deliverable filename")
	}
	if opts.MediaType == "" {
		return fmt.Errorf("missing deliverable media type")
	}
	if opts.URL == "" {
		return fmt.Errorf("missing deliverable url")
	}
	if opts.Bytes < 0 {
		return fmt.Errorf("deliverable bytes must be non-negative")
	}
	return nil
}

func validateChannelDeliverableActionRequestOptions(opts ChannelDeliverableOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.MessageID == "" {
		return fmt.Errorf("missing deliverable message id")
	}
	if opts.DeliverableID == "" {
		return fmt.Errorf("missing deliverable id")
	}
	if opts.Filename == "" {
		return fmt.Errorf("missing deliverable filename")
	}
	if opts.MediaType == "" {
		return fmt.Errorf("missing deliverable media type")
	}
	if opts.URL == "" {
		return fmt.Errorf("missing deliverable url")
	}
	if opts.Bytes < 0 {
		return fmt.Errorf("deliverable bytes must be non-negative")
	}
	return nil
}

func RenderChannelDeliverableComment(opts ChannelDeliverableOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-deliverable channel="%s" thread_id="%s" message_id="%s" deliverable_id="%s" author="%s" filename_sha256_12="%s" media_type_sha256_12="%s" url_sha256_12="%s" -->
%s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.MessageID), escapeMarkerValue(opts.DeliverableID), escapeMarkerValue(author), shortDocumentHash(opts.Filename), shortDocumentHash(opts.MediaType), optionalChannelAttachmentHash(opts.URL), RenderChannelDeliverableBody(opts))
}

func RenderChannelDeliverableBody(opts ChannelDeliverableOptions) string {
	var b strings.Builder
	b.WriteString("GitClaw channel deliverable queued.\n\n")
	fmt.Fprintf(&b, "Filename: %s\n", opts.Filename)
	fmt.Fprintf(&b, "Media type: %s\n", opts.MediaType)
	fmt.Fprintf(&b, "Size: %d bytes\n", opts.Bytes)
	if opts.FileSHA256 != "" {
		fmt.Fprintf(&b, "SHA-256: %s\n", opts.FileSHA256)
	}
	fmt.Fprintf(&b, "URL: %s\n", opts.URL)
	if opts.Caption != "" {
		fmt.Fprintf(&b, "\nCaption:\n%s\n", opts.Caption)
	}
	b.WriteString("\nProvider upload performed: false\n")
	b.WriteString("Provider delivery performed: false\n")
	return strings.TrimSpace(b.String())
}

func channelDeliverableMatches(body, channel, deliverableID string) bool {
	return HasChannelDeliverableMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`deliverable_id="%s"`, escapeMarkerValue(cleanChannelDeliverableID(deliverableID))))
}

func channelDeliverableMarkerFields(body string) (string, string, string, string) {
	match := channelDeliverableMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", "", ""
	}
	return markerAttribute(match[1], "channel"), markerAttribute(match[1], "thread_id"), markerAttribute(match[1], "message_id"), markerAttribute(match[1], "deliverable_id")
}

func cleanChannelDeliverableID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelDeliverableID(ev Event, opts ChannelDeliverableOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Channel, opts.ThreadID, opts.Filename, opts.URL, opts.FileSHA256}, "|")
	return fmt.Sprintf("deliverable-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelDeliverableMessageID(ev Event, deliverableID string) string {
	seed := strings.Join([]string{eventID(ev), deliverableID}, "|")
	return fmt.Sprintf("gitclaw-channel-deliverable-%s-%s", eventID(ev), shortDocumentHash(seed))
}
