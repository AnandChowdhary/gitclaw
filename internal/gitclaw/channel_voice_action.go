package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelVoiceOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	VoiceID           string
	Title             string
	Transcript        string
	DurationSeconds   int
	MediaType         string
	AudioURL          string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelVoiceResult struct {
	VoiceIssueNumber int
	VoiceIssueURL    string
	VoiceCreated     bool
	VoiceDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelVoiceActionRequest struct {
	Options              ChannelVoiceOptions
	Command              string
	Subcommand           string
	AutoVoiceID          bool
	AutoNotifyMessageID  bool
	TargetFromIssue      bool
	TitleSHA             string
	TitleBytes           int
	TitleLines           int
	TranscriptSHA        string
	TranscriptBytes      int
	TranscriptLines      int
	MediaTypeSHA         string
	MediaTypeBytes       int
	AudioURLSHA          string
	AudioURLBytes        int
	RequestedRouteHash   string
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	NotificationBodySHA  string
	DurationSecondsKnown bool
}

func IsChannelVoiceActionRequest(ev Event, cfg Config) bool {
	return isChannelVoiceActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelVoiceActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "voice", "voice-note", "voicenote", "audio", "audio-note", "transcript", "transcribe", "voice-memo", "memo":
		return true
	default:
		return false
	}
}

func BuildChannelVoiceActionRequest(ev Event, cfg Config) (ChannelVoiceActionRequest, error) {
	fields, trailing, ok := channelVoiceActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelVoiceActionRequest{}, fmt.Errorf("missing channel voice command")
	}
	req := ChannelVoiceActionRequest{
		Options: ChannelVoiceOptions{
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
				return ChannelVoiceActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--voice-id", "--audio-id", "--memo-id", "--transcript-id", "--id":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.VoiceID = cleanChannelVoiceID(fields[i+1])
			i++
		case "--duration", "--duration-seconds", "--seconds":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			seconds, err := parseChannelVoiceDurationSeconds(fields[i+1])
			if err != nil {
				return ChannelVoiceActionRequest{}, err
			}
			req.Options.DurationSeconds = seconds
			i++
		case "--media-type", "--mime-type", "--content-type", "--type":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.MediaType = fields[i+1]
			i++
		case "--url", "--audio-url", "--media-url", "--source-url":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.AudioURL = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelVoiceActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelVoiceActionRequest{}, fmt.Errorf("unknown channel voice argument %q", field)
			}
			if req.Options.VoiceID == "" {
				req.Options.VoiceID = cleanChannelVoiceID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelVoiceActionRequest{}, fmt.Errorf("unexpected channel voice argument %q", field)
		}
	}
	if err := applyChannelVoiceIssueTarget(ev, &req); err != nil {
		return ChannelVoiceActionRequest{}, err
	}
	title, transcript := parseChannelVoiceTitleTranscript(trailing, ev)
	req.Options.Title = title
	req.Options.Transcript = transcript
	if strings.TrimSpace(req.Options.VoiceID) == "" {
		req.Options.VoiceID = autoChannelVoiceID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, transcript)
		req.AutoVoiceID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelVoiceNotifyMessageID(ev, req.Options.VoiceID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelVoiceOptions(req.Options)
	if err := validateChannelVoiceActionRequestOptions(req.Options); err != nil {
		return ChannelVoiceActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.TranscriptSHA = shortDocumentHash(req.Options.Transcript)
	req.TranscriptBytes = len(req.Options.Transcript)
	req.TranscriptLines = lineCount(req.Options.Transcript)
	req.MediaTypeSHA = shortDocumentHash(req.Options.MediaType)
	req.MediaTypeBytes = len(req.Options.MediaType)
	req.AudioURLSHA = shortDocumentHash(req.Options.AudioURL)
	req.AudioURLBytes = len(req.Options.AudioURL)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelVoiceNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	req.DurationSecondsKnown = req.Options.DurationSeconds > 0
	return req, nil
}

func RunChannelVoice(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelVoiceOptions) (ChannelVoiceResult, error) {
	opts = normalizeChannelVoiceOptions(opts)
	var err error
	opts, err = applyChannelVoiceRoute(cfg, opts)
	if err != nil {
		return ChannelVoiceResult{}, err
	}
	if err := validateChannelVoiceOptions(opts); err != nil {
		return ChannelVoiceResult{}, err
	}
	voiceIssue, created, duplicate, err := findOrCreateChannelVoiceIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelVoiceResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelVoiceNotificationBody(opts, voiceIssue.Number, issueURL(opts.Repo, voiceIssue.Number)),
	})
	if err != nil {
		return ChannelVoiceResult{}, fmt.Errorf("queue channel voice notification: %w", err)
	}
	return ChannelVoiceResult{
		VoiceIssueNumber: voiceIssue.Number,
		VoiceIssueURL:    issueURL(opts.Repo, voiceIssue.Number),
		VoiceCreated:     created,
		VoiceDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelVoiceActionReport(ev Event, req ChannelVoiceActionRequest, result ChannelVoiceResult) string {
	status := "captured"
	switch {
	case result.VoiceDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.VoiceDuplicate:
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
	b.WriteString("## GitClaw Channel Voice Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_voice_status: `%s`\n", status)
	fmt.Fprintf(&b, "- voice_issue: `#%d`\n", result.VoiceIssueNumber)
	fmt.Fprintf(&b, "- voice_issue_url: `%s`\n", result.VoiceIssueURL)
	fmt.Fprintf(&b, "- voice_issue_created: `%t`\n", result.VoiceCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.VoiceDuplicate)
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
	fmt.Fprintf(&b, "- voice_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.VoiceID))
	fmt.Fprintf(&b, "- voice_id_auto: `%t`\n", req.AutoVoiceID)
	fmt.Fprintf(&b, "- voice_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- voice_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- voice_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- transcript_sha256_12: `%s`\n", req.TranscriptSHA)
	fmt.Fprintf(&b, "- transcript_bytes: `%d`\n", req.TranscriptBytes)
	fmt.Fprintf(&b, "- transcript_lines: `%d`\n", req.TranscriptLines)
	fmt.Fprintf(&b, "- duration_seconds: `%d`\n", req.Options.DurationSeconds)
	fmt.Fprintf(&b, "- duration_seconds_known: `%t`\n", req.DurationSecondsKnown)
	fmt.Fprintf(&b, "- media_type_sha256_12: `%s`\n", req.MediaTypeSHA)
	fmt.Fprintf(&b, "- media_type_bytes: `%d`\n", req.MediaTypeBytes)
	fmt.Fprintf(&b, "- audio_url_sha256_12: `%s`\n", noneIfEmpty(req.AudioURLSHA))
	fmt.Fprintf(&b, "- audio_url_bytes: `%d`\n", req.AudioURLBytes)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_voice_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_voice_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_media_type_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_audio_url_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_voice_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin voice/audio note as a durable GitHub issue, then queued a provider-facing transcript link back to the mirrored thread. The voice issue contains the human-readable transcript; this source receipt keeps provider IDs, audio URLs, media metadata, transcripts, titles, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the voice-note notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent voice-note links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate voice issues are suppressed by `voice_id`; duplicate voice-note notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the voice issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelVoiceIssueBody(opts ChannelVoiceOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-voice voice_id=\"%s\" channel=\"%s\" media_type_sha256_12=\"%s\" audio_url_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.VoiceID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.MediaType), shortDocumentHash(opts.AudioURL), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel voice note.\n\n")
	fmt.Fprintf(&b, "- voice_id: %s\n", opts.VoiceID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- duration_seconds: %d\n", opts.DurationSeconds)
	fmt.Fprintf(&b, "- media_type_sha256_12: %s\n", shortDocumentHash(opts.MediaType))
	fmt.Fprintf(&b, "- audio_url_sha256_12: %s\n", noneIfEmpty(shortDocumentHash(opts.AudioURL)))
	fmt.Fprintf(&b, "- voice_mode: github-issue-transcript\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_audio_url_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Voice Note\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Transcript) != "" {
		b.WriteString("\n\n## Transcript\n\n")
		b.WriteString(strings.TrimSpace(opts.Transcript))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for summarizing, tasking, searching, or following up on the channel voice note.")
	return strings.TrimSpace(b.String())
}

func channelVoiceActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelVoiceActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelVoiceIssueTarget(ev Event, req *ChannelVoiceActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel voice requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelVoiceTitleTranscript(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTitle := fmt.Sprintf("Channel voice note from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTitle, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var title string
	var transcriptLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "title:"):
		title = strings.TrimSpace(first[len("title:"):])
		transcriptLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "voice:"):
		title = strings.TrimSpace(first[len("voice:"):])
		transcriptLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "audio:"):
		title = strings.TrimSpace(first[len("audio:"):])
		transcriptLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "transcript:"), strings.HasPrefix(lowerFirst, "summary:"):
		title = defaultTitle
		transcriptLines = cleaned
	default:
		title = first
		transcriptLines = cleaned[1:]
	}
	if title == "" {
		title = defaultTitle
	}
	transcript := strings.TrimSpace(strings.Join(transcriptLines, "\n"))
	transcriptLower := strings.ToLower(strings.TrimSpace(transcript))
	switch {
	case strings.HasPrefix(transcriptLower, "transcript:"):
		transcript = strings.TrimSpace(strings.TrimSpace(transcript)[len("transcript:"):])
	case strings.HasPrefix(transcriptLower, "summary:"):
		transcript = strings.TrimSpace(strings.TrimSpace(transcript)[len("summary:"):])
	}
	return title, transcript
}

func normalizeChannelVoiceOptions(opts ChannelVoiceOptions) ChannelVoiceOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.VoiceID = cleanChannelVoiceID(opts.VoiceID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Transcript = strings.TrimSpace(opts.Transcript)
	opts.MediaType = strings.ToLower(strings.TrimSpace(opts.MediaType))
	opts.AudioURL = strings.TrimSpace(opts.AudioURL)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.DurationSeconds < 0 {
		opts.DurationSeconds = 0
	}
	return opts
}

func applyChannelVoiceRoute(cfg Config, opts ChannelVoiceOptions) (ChannelVoiceOptions, error) {
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

func validateChannelVoiceOptions(opts ChannelVoiceOptions) error {
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
	if opts.VoiceID == "" {
		return fmt.Errorf("missing voice id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing voice source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing voice title")
	}
	return nil
}

func validateChannelVoiceActionRequestOptions(opts ChannelVoiceOptions) error {
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
	if opts.VoiceID == "" {
		return fmt.Errorf("missing voice id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing voice source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing voice title")
	}
	return nil
}

func findOrCreateChannelVoiceIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelVoiceOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel voice issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelVoiceMatches(issue.Body, opts.VoiceID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelVoiceIssueTitle(opts), RenderChannelVoiceIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel voice issue: %w", err)
	}
	return issue, true, false, nil
}

func channelVoiceIssueTitle(opts ChannelVoiceOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.VoiceID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel voice: " + title
}

func channelVoiceMatches(body, voiceID string) bool {
	return HasChannelVoiceMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`voice_id="%s"`, escapeMarkerValue(cleanChannelVoiceID(voiceID))))
}

func cleanChannelVoiceID(value string) string {
	return cleanChannelHuddleID(value)
}

func parseChannelVoiceDurationSeconds(value string) (int, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "seconds")
	value = strings.TrimSuffix(value, "second")
	value = strings.TrimSuffix(value, "secs")
	value = strings.TrimSuffix(value, "sec")
	value = strings.TrimSuffix(value, "s")
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("duration requires a number of seconds")
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid channel voice duration %q", value)
	}
	return seconds, nil
}

func autoChannelVoiceID(ev Event, channel, threadID, sourceMessageID, title, transcript string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, transcript}, "|")
	return fmt.Sprintf("voice-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelVoiceNotifyMessageID(ev Event, voiceID string) string {
	seed := strings.Join([]string{eventID(ev), voiceID}, "|")
	return fmt.Sprintf("gitclaw-channel-voice-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelVoiceNotificationBody(opts ChannelVoiceOptions, voiceIssueNumber int, voiceIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel voice note captured.\n\n")
	if voiceIssueNumber > 0 {
		fmt.Fprintf(&b, "Voice note: #%d\n", voiceIssueNumber)
	}
	if voiceIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", voiceIssueURL)
	}
	if opts.DurationSeconds > 0 {
		fmt.Fprintf(&b, "Duration: %ds\n", opts.DurationSeconds)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nContinue with the transcript in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
