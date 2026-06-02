package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRsvpResponseOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	RsvpID            string
	ResponseID        string
	SourceMessageID   string
	NotifyMessageID   string
	Response          string
	Responder         string
	Note              string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelRsvpResponseResult struct {
	RsvpIssueNumber   int
	RsvpIssueURL      string
	ResponseCommentID int64
	ResponseDuplicate bool
	Notification      ChannelSendResult
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	ResponseIDHash    string
	ResponseHash      string
	ResponderHash     string
	NoteHash          string
	SourceMessageHash string
	NotifyHash        string
	NotificationSHA   string
}

type ChannelRsvpResponseActionRequest struct {
	Options               ChannelRsvpResponseOptions
	Command               string
	Subcommand            string
	AutoResponseID        bool
	AutoNotifyMessageID   bool
	TargetFromIssue       bool
	ResponseSHA           string
	ResponderSHA          string
	ResponderBytes        int
	ResponderLines        int
	NoteSHA               string
	NoteBytes             int
	NoteLines             int
	RequestedRouteHash    string
	RequestedThreadHash   string
	RequestedMsgHash      string
	NotifyMessageHash     string
	NotificationBodySHA   string
	RsvpResponseBodySHA   string
	RsvpResponseBodyBytes int
	RsvpResponseBodyLines int
}

func IsChannelRsvpResponseActionRequest(ev Event, cfg Config) bool {
	return isChannelRsvpResponseActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRsvpResponseActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rsvp-response", "rsvp-reply", "respond", "attendance-response":
		return true
	default:
		return false
	}
}

func BuildChannelRsvpResponseActionRequest(ev Event, cfg Config) (ChannelRsvpResponseActionRequest, error) {
	fields, trailing, ok := channelRsvpResponseActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRsvpResponseActionRequest{}, fmt.Errorf("missing channel rsvp response command")
	}
	req := ChannelRsvpResponseActionRequest{
		Options: ChannelRsvpResponseOptions{
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
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--rsvp-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RsvpID = cleanChannelRsvpID(fields[i+1])
			i++
		case "--response-id", "--reply-id":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ResponseID = cleanChannelRsvpResponseID(fields[i+1])
			i++
		case "--message-id", "--source-message-id":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			if req.Options.ResponseID == "" {
				req.Options.ResponseID = cleanChannelRsvpResponseID(fields[i+1])
			}
			i++
		case "--notify-message-id", "--ack-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--response", "--answer", "--status":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			response, ok := normalizeChannelRsvpResponseValue(fields[i+1])
			if !ok {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("unsupported rsvp response %q", fields[i+1])
			}
			req.Options.Response = response
			i++
		case "--responder", "--name":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Responder = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRsvpResponseActionRequest{}, fmt.Errorf("unknown channel rsvp response argument %q", field)
			}
			if response, ok := normalizeChannelRsvpResponseValue(field); ok && req.Options.Response == "" {
				req.Options.Response = response
				continue
			}
			if req.Options.RsvpID == "" {
				req.Options.RsvpID = cleanChannelRsvpID(field)
				continue
			}
			return ChannelRsvpResponseActionRequest{}, fmt.Errorf("unexpected channel rsvp response argument %q", field)
		}
	}
	response, responder, note := parseChannelRsvpResponseText(trailing)
	if req.Options.Response == "" {
		req.Options.Response = response
	}
	if req.Options.Responder == "" {
		req.Options.Responder = responder
	}
	req.Options.Note = note
	if err := applyChannelRsvpResponseIssueTarget(ev, &req); err != nil {
		return ChannelRsvpResponseActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.ResponseID) == "" {
		req.Options.ResponseID = autoChannelRsvpResponseID(ev, req.Options.RsvpID, req.Options.Channel, req.Options.ThreadID, req.Options.Response, req.Options.Responder, req.Options.Note)
		req.AutoResponseID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelRsvpResponseNotifyMessageID(req.Options.RsvpID, req.Options.ResponseID, req.Options.Channel, req.Options.ThreadID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelRsvpResponseOptions(req.Options)
	if err := validateChannelRsvpResponseActionRequestOptions(req.Options); err != nil {
		return ChannelRsvpResponseActionRequest{}, err
	}
	responseBody := RenderChannelRsvpResponseComment(req.Options)
	notificationBody := renderChannelRsvpResponseNotificationBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.ResponseSHA = shortDocumentHash(req.Options.Response)
	req.ResponderSHA = shortDocumentHash(req.Options.Responder)
	req.ResponderBytes = len(req.Options.Responder)
	req.ResponderLines = lineCount(req.Options.Responder)
	req.NoteSHA = shortDocumentHash(req.Options.Note)
	req.NoteBytes = len(req.Options.Note)
	req.NoteLines = lineCount(req.Options.Note)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.RsvpResponseBodySHA = shortDocumentHash(responseBody)
	req.RsvpResponseBodyBytes = len(responseBody)
	req.RsvpResponseBodyLines = lineCount(responseBody)
	return req, nil
}

func RunChannelRsvpResponse(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRsvpResponseOptions) (ChannelRsvpResponseResult, error) {
	opts = normalizeChannelRsvpResponseOptions(opts)
	var err error
	opts, err = applyChannelRsvpResponseRoute(cfg, opts)
	if err != nil {
		return ChannelRsvpResponseResult{}, err
	}
	if err := validateChannelRsvpResponseOptions(opts); err != nil {
		return ChannelRsvpResponseResult{}, err
	}
	rsvp, err := findChannelRsvpIssue(ctx, cfg, github, opts.Repo, opts.RsvpID)
	if err != nil {
		return ChannelRsvpResponseResult{}, err
	}
	responseBody := RenderChannelRsvpResponseComment(opts)
	responseDuplicate := false
	var responseCommentID int64
	comments, err := github.ListIssueComments(ctx, opts.Repo, rsvp.Number)
	if err != nil {
		return ChannelRsvpResponseResult{}, fmt.Errorf("list channel rsvp response comments: %w", err)
	}
	for _, comment := range comments {
		if channelRsvpResponseMatches(comment.Body, opts.RsvpID, opts.ResponseID) {
			responseDuplicate = true
			responseCommentID = comment.ID
			break
		}
	}
	if !responseDuplicate {
		posted, err := github.PostIssueComment(ctx, opts.Repo, rsvp.Number, responseBody)
		if err != nil {
			return ChannelRsvpResponseResult{}, fmt.Errorf("post channel rsvp response: %w", err)
		}
		responseCommentID = posted.ID
	}
	notificationBody := renderChannelRsvpResponseNotificationBody(opts, rsvp.Number, issueURL(opts.Repo, rsvp.Number))
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      notificationBody,
	})
	if err != nil {
		return ChannelRsvpResponseResult{}, fmt.Errorf("queue channel rsvp response notification: %w", err)
	}
	return ChannelRsvpResponseResult{
		RsvpIssueNumber:   rsvp.Number,
		RsvpIssueURL:      issueURL(opts.Repo, rsvp.Number),
		ResponseCommentID: responseCommentID,
		ResponseDuplicate: responseDuplicate,
		Notification:      notification,
		RouteName:         notification.RouteName,
		RouteHash:         notification.RouteHash,
		Channel:           notification.Channel,
		ThreadHash:        notification.ThreadHash,
		ResponseIDHash:    shortDocumentHash(opts.ResponseID),
		ResponseHash:      shortDocumentHash(opts.Response),
		ResponderHash:     shortDocumentHash(opts.Responder),
		NoteHash:          shortDocumentHash(opts.Note),
		SourceMessageHash: shortDocumentHash(opts.SourceMessageID),
		NotifyHash:        shortDocumentHash(opts.NotifyMessageID),
		NotificationSHA:   shortDocumentHash(notificationBody),
	}, nil
}

func RenderChannelRsvpResponseActionReport(ev Event, req ChannelRsvpResponseActionRequest, result ChannelRsvpResponseResult) string {
	status := "recorded"
	switch {
	case result.ResponseDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ResponseDuplicate:
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
	sourceMessageHash := result.SourceMessageHash
	if sourceMessageHash == "" {
		sourceMessageHash = req.RequestedMsgHash
	}
	notifyHash := result.NotifyHash
	if notifyHash == "" {
		notifyHash = req.NotifyMessageHash
	}
	responseHash := result.ResponseHash
	if responseHash == "" {
		responseHash = req.ResponseSHA
	}
	responderHash := result.ResponderHash
	if responderHash == "" {
		responderHash = req.ResponderSHA
	}
	noteHash := result.NoteHash
	if noteHash == "" {
		noteHash = req.NoteSHA
	}
	notificationBodyHash := result.NotificationSHA
	if notificationBodyHash == "" {
		notificationBodyHash = req.NotificationBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel RSVP Response Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_rsvp_response_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rsvp_issue: `#%d`\n", result.RsvpIssueNumber)
	fmt.Fprintf(&b, "- rsvp_issue_url: `%s`\n", result.RsvpIssueURL)
	fmt.Fprintf(&b, "- response_comment_id: `%d`\n", result.ResponseCommentID)
	fmt.Fprintf(&b, "- response_recorded: `%t`\n", !result.ResponseDuplicate)
	fmt.Fprintf(&b, "- response_duplicate_suppressed: `%t`\n", result.ResponseDuplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- rsvp_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.RsvpID))
	fmt.Fprintf(&b, "- response_id_sha256_12: `%s`\n", noneIfEmpty(result.ResponseIDHash))
	fmt.Fprintf(&b, "- response_id_auto: `%t`\n", req.AutoResponseID)
	fmt.Fprintf(&b, "- response_sha256_12: `%s`\n", noneIfEmpty(responseHash))
	fmt.Fprintf(&b, "- responder_sha256_12: `%s`\n", noneIfEmpty(responderHash))
	fmt.Fprintf(&b, "- responder_bytes: `%d`\n", req.ResponderBytes)
	fmt.Fprintf(&b, "- responder_lines: `%d`\n", req.ResponderLines)
	fmt.Fprintf(&b, "- note_sha256_12: `%s`\n", noneIfEmpty(noteHash))
	fmt.Fprintf(&b, "- note_bytes: `%d`\n", req.NoteBytes)
	fmt.Fprintf(&b, "- note_lines: `%d`\n", req.NoteLines)
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(sourceMessageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- rsvp_response_body_sha256_12: `%s`\n", req.RsvpResponseBodySHA)
	fmt.Fprintf(&b, "- rsvp_response_body_bytes: `%d`\n", req.RsvpResponseBodyBytes)
	fmt.Fprintf(&b, "- rsvp_response_body_lines: `%d`\n", req.RsvpResponseBodyLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodyHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_rsvp_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_response_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_response_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_responder_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_note_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_rsvp_response_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a channel-origin RSVP response on the GitHub RSVP issue, then queued a provider-facing acknowledgement back to the mirrored channel thread. The RSVP issue contains the human-readable response; this source receipt keeps RSVP IDs, response IDs, participant text, notes, thread IDs, message IDs, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the RSVP response acknowledgement with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent acknowledgements with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate RSVP response records are suppressed by `rsvp_id + response_id`; duplicate acknowledgements are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the RSVP issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelRsvpResponseComment(opts ChannelRsvpResponseOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-rsvp-response rsvp_id=\"%s\" response_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" channel=\"%s\" response_sha256_12=\"%s\" responder_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" note_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.RsvpID), escapeMarkerValue(opts.ResponseID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Response), shortDocumentHash(opts.Responder), shortDocumentHash(opts.SourceMessageID), shortDocumentHash(opts.Note))
	b.WriteString("GitClaw channel RSVP response.\n\n")
	fmt.Fprintf(&b, "- response: %s\n", opts.Response)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- response_id_sha256_12: %s\n", shortDocumentHash(opts.ResponseID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n\n")
	if strings.TrimSpace(opts.Responder) != "" {
		b.WriteString("## Participant\n\n")
		b.WriteString(strings.TrimSpace(opts.Responder))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(opts.Note) != "" {
		b.WriteString("## Note\n\n")
		b.WriteString(strings.TrimSpace(opts.Note))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func channelRsvpResponseActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRsvpResponseActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelRsvpResponseIssueTarget(ev Event, req *ChannelRsvpResponseActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel rsvp response requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelRsvpResponseText(trailing string) (string, string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	var response string
	var responder string
	var note []string
	inNote := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inNote && len(note) > 0 {
				note = append(note, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "response:"):
			inNote = false
			if normalized, ok := normalizeChannelRsvpResponseValue(strings.TrimSpace(trimmed[len("response:"):])); ok {
				response = normalized
			}
		case strings.HasPrefix(lower, "answer:"):
			inNote = false
			if normalized, ok := normalizeChannelRsvpResponseValue(strings.TrimSpace(trimmed[len("answer:"):])); ok {
				response = normalized
			}
		case strings.HasPrefix(lower, "rsvp:"):
			inNote = false
			if normalized, ok := normalizeChannelRsvpResponseValue(strings.TrimSpace(trimmed[len("rsvp:"):])); ok {
				response = normalized
			}
		case strings.HasPrefix(lower, "responder:"):
			inNote = false
			responder = strings.TrimSpace(trimmed[len("responder:"):])
		case strings.HasPrefix(lower, "participant:"):
			inNote = false
			responder = strings.TrimSpace(trimmed[len("participant:"):])
		case strings.HasPrefix(lower, "name:"):
			inNote = false
			responder = strings.TrimSpace(trimmed[len("name:"):])
		case strings.HasPrefix(lower, "note:"):
			inNote = true
			value := strings.TrimSpace(trimmed[len("note:"):])
			if value != "" {
				note = append(note, value)
			}
		case strings.HasPrefix(lower, "notes:"):
			inNote = true
			value := strings.TrimSpace(trimmed[len("notes:"):])
			if value != "" {
				note = append(note, value)
			}
		case inNote:
			note = append(note, line)
		case response == "":
			if normalized, ok := normalizeChannelRsvpResponseValue(trimmed); ok {
				response = normalized
				continue
			}
			note = append(note, line)
		default:
			note = append(note, line)
		}
	}
	return response, strings.TrimSpace(responder), strings.TrimSpace(strings.Join(note, "\n"))
}

func normalizeChannelRsvpResponseOptions(opts ChannelRsvpResponseOptions) ChannelRsvpResponseOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.RsvpID = cleanChannelRsvpID(opts.RsvpID)
	opts.ResponseID = cleanChannelRsvpResponseID(opts.ResponseID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	if response, ok := normalizeChannelRsvpResponseValue(opts.Response); ok {
		opts.Response = response
	} else {
		opts.Response = strings.TrimSpace(opts.Response)
	}
	opts.Responder = strings.TrimSpace(opts.Responder)
	opts.Note = strings.TrimSpace(opts.Note)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelRsvpResponseRoute(cfg Config, opts ChannelRsvpResponseOptions) (ChannelRsvpResponseOptions, error) {
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
		Body:      opts.Response,
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

func validateChannelRsvpResponseActionRequestOptions(opts ChannelRsvpResponseOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("missing channel route or channel thread target")
	}
	if opts.RsvpID == "" {
		return fmt.Errorf("missing rsvp id")
	}
	if opts.ResponseID == "" {
		return fmt.Errorf("missing rsvp response id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing rsvp response source issue")
	}
	if _, ok := normalizeChannelRsvpResponseValue(opts.Response); !ok {
		return fmt.Errorf("missing or unsupported rsvp response")
	}
	return nil
}

func validateChannelRsvpResponseOptions(opts ChannelRsvpResponseOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.RsvpID == "" {
		return fmt.Errorf("missing rsvp id")
	}
	if opts.ResponseID == "" {
		return fmt.Errorf("missing rsvp response id")
	}
	if opts.NotifyMessageID == "" {
		return fmt.Errorf("missing notification message id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing rsvp response source issue")
	}
	if _, ok := normalizeChannelRsvpResponseValue(opts.Response); !ok {
		return fmt.Errorf("missing or unsupported rsvp response")
	}
	return nil
}

func findChannelRsvpIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, repo, rsvpID string) (Issue, error) {
	issues, err := github.ListOpenIssues(ctx, repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, fmt.Errorf("list channel rsvp issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelRsvpMatches(issue.Body, rsvpID) {
			return issue, nil
		}
	}
	return Issue{}, fmt.Errorf("channel rsvp %q not found", cleanChannelRsvpID(rsvpID))
}

func channelRsvpResponseMatches(body, rsvpID, responseID string) bool {
	return HasChannelRsvpResponseMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`rsvp_id="%s"`, escapeMarkerValue(cleanChannelRsvpID(rsvpID)))) &&
		strings.Contains(body, fmt.Sprintf(`response_id="%s"`, escapeMarkerValue(cleanChannelRsvpResponseID(responseID))))
}

func cleanChannelRsvpResponseID(value string) string {
	return cleanChannelHuddleID(value)
}

func normalizeChannelRsvpResponseValue(value string) (string, bool) {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "yes", "y", "going", "attending", "in", "accept", "accepted":
		return "yes", true
	case "no", "n", "not-going", "decline", "declined", "out", "cannot", "cant", "can't":
		return "no", true
	case "maybe", "m", "tentative", "unsure":
		return "maybe", true
	default:
		return "", false
	}
}

func autoChannelRsvpResponseID(ev Event, rsvpID, channel, threadID, response, responder, note string) string {
	seed := strings.Join([]string{eventID(ev), rsvpID, channel, threadID, response, responder, note}, "|")
	return fmt.Sprintf("rsvp-response-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRsvpResponseNotifyMessageID(rsvpID, responseID, channel, threadID string) string {
	seed := strings.Join([]string{rsvpID, responseID, channel, threadID}, "|")
	return fmt.Sprintf("gitclaw-rsvp-response-%s", shortDocumentHash(seed))
}

func renderChannelRsvpResponseNotificationBody(opts ChannelRsvpResponseOptions, rsvpIssueNumber int, rsvpIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw RSVP response recorded.\n\n")
	if rsvpIssueNumber > 0 {
		fmt.Fprintf(&b, "RSVP: #%d\n", rsvpIssueNumber)
	}
	if rsvpIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", rsvpIssueURL)
	}
	fmt.Fprintf(&b, "Response: %s\n", opts.Response)
	if strings.TrimSpace(opts.Responder) != "" {
		fmt.Fprintf(&b, "Participant: %s\n", strings.TrimSpace(opts.Responder))
	}
	b.WriteString("\nRecorded in the linked GitHub RSVP issue.")
	return strings.TrimSpace(b.String())
}
