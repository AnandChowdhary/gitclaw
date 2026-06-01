package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRoomOptions struct {
	Repo              string
	RoomID            string
	SourceIssueNumber int
	SourceCommentID   int64
	Topic             string
	Notes             string
	Routes            []string
	MessageID         string
	Author            string
}

type ChannelRoomResult struct {
	RoomIssueNumber int
	RoomIssueURL    string
	RoomCreated     bool
	Broadcast       ChannelBroadcastResult
}

type ChannelRoomActionRequest struct {
	Options          ChannelRoomOptions
	Command          string
	Subcommand       string
	AutoRoomID       bool
	AutoMessageID    bool
	TopicSHA         string
	TopicBytes       int
	TopicLines       int
	NotesSHA         string
	NotesBytes       int
	NotesLines       int
	RoomSource       string
	RoutesSHA        string
	RouteCount       int
	OutboundBodySHA  string
	OutboundBodySize int
}

func IsChannelRoomActionRequest(ev Event, cfg Config) bool {
	return isChannelRoomActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRoomActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "room", "space":
		return true
	default:
		return false
	}
}

func BuildChannelRoomActionRequest(ev Event, cfg Config) (ChannelRoomActionRequest, error) {
	fields, trailingNotes, ok := channelRoomActionFieldsAndTrailingNotes(ev, cfg)
	if !ok {
		return ChannelRoomActionRequest{}, fmt.Errorf("missing channel room command")
	}
	req := ChannelRoomActionRequest{
		Options: ChannelRoomOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RoomSource: "trailing-lines",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelRoomActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelRoomActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--room-id", "--huddle-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRoomActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.RoomID = cleanChannelRoomID(fields[i+1])
			i++
		case "--message-id":
			if i+1 >= len(fields) {
				return ChannelRoomActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRoomActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRoomActionRequest{}, fmt.Errorf("unknown channel room argument %q", field)
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
		}
	}

	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if len(req.Options.Routes) == 0 {
		return ChannelRoomActionRequest{}, fmt.Errorf("missing room routes")
	}
	topic, notes := parseChannelRoomTopicNotes(trailingNotes, ev)
	req.Options.Topic = topic
	req.Options.Notes = notes
	if req.Options.RoomID == "" {
		req.Options.RoomID = autoChannelRoomID(ev, req.Options.Routes, topic, notes)
		req.AutoRoomID = true
	}
	if req.Options.MessageID == "" {
		req.Options.MessageID = autoChannelRoomMessageID(ev, req.Options.RoomID, req.Options.Routes)
		req.AutoMessageID = true
	}
	if err := validateChannelRoomOptions(req.Options); err != nil {
		return ChannelRoomActionRequest{}, err
	}
	outboundPreview := renderChannelRoomOutboundBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.TopicSHA = shortDocumentHash(topic)
	req.TopicBytes = len(topic)
	req.TopicLines = lineCount(topic)
	req.NotesSHA = shortDocumentHash(notes)
	req.NotesBytes = len(notes)
	req.NotesLines = lineCount(notes)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	req.OutboundBodySHA = shortDocumentHash(outboundPreview)
	req.OutboundBodySize = len(outboundPreview)
	return req, nil
}

func RunChannelRoom(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRoomOptions) (ChannelRoomResult, error) {
	opts = normalizeChannelRoomOptions(opts)
	if err := validateChannelRoomOptions(opts); err != nil {
		return ChannelRoomResult{}, err
	}
	room, created, err := findOrCreateChannelRoomIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelRoomResult{}, err
	}
	broadcastOpts := ChannelBroadcastOptions{
		Repo:      opts.Repo,
		Routes:    opts.Routes,
		MessageID: opts.MessageID,
		Author:    opts.Author,
		Body:      renderChannelRoomOutboundBody(opts, room.Number, issueURL(opts.Repo, room.Number)),
	}
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, broadcastOpts)
	if err != nil {
		return ChannelRoomResult{}, err
	}
	return ChannelRoomResult{
		RoomIssueNumber: room.Number,
		RoomIssueURL:    issueURL(opts.Repo, room.Number),
		RoomCreated:     created,
		Broadcast:       broadcast,
	}, nil
}

func RenderChannelRoomActionReport(ev Event, req ChannelRoomActionRequest, result ChannelRoomResult) string {
	status := "queued"
	switch {
	case !result.RoomCreated && result.Broadcast.Queued == 0 && result.Broadcast.Duplicates > 0:
		status = "duplicate"
	case result.Broadcast.Queued > 0 && result.Broadcast.Duplicates > 0:
		status = "partially-queued"
	}
	outboundBody := renderChannelRoomOutboundBody(req.Options, result.RoomIssueNumber, result.RoomIssueURL)
	outboundBodySHA := shortDocumentHash(outboundBody)
	outboundBodyBytes := len(outboundBody)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Room Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_room_status: `%s`\n", status)
	fmt.Fprintf(&b, "- room_issue: `#%d`\n", result.RoomIssueNumber)
	fmt.Fprintf(&b, "- room_issue_url: `%s`\n", result.RoomIssueURL)
	fmt.Fprintf(&b, "- room_issue_created: `%t`\n", result.RoomCreated)
	fmt.Fprintf(&b, "- room_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.RoomID))
	fmt.Fprintf(&b, "- room_id_auto: `%t`\n", req.AutoRoomID)
	fmt.Fprintf(&b, "- room_topic_sha256_12: `%s`\n", req.TopicSHA)
	fmt.Fprintf(&b, "- room_topic_bytes: `%d`\n", req.TopicBytes)
	fmt.Fprintf(&b, "- room_topic_lines: `%d`\n", req.TopicLines)
	fmt.Fprintf(&b, "- room_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- room_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- room_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- room_source: `%s`\n", req.RoomSource)
	fmt.Fprintf(&b, "- room_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- room_invites_queued: `%d`\n", result.Broadcast.Queued)
	fmt.Fprintf(&b, "- room_invite_duplicates: `%d`\n", result.Broadcast.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Broadcast.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", outboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", outboundBodyBytes)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_room_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_topic_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_room_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a durable GitHub channel room, then queued one provider-facing invitation per reviewed route. The room issue contains the human-readable topic and notes; this source receipt keeps route names, room IDs, topic text, notes, thread IDs, message IDs, and outbound bodies out of band.\n\n")
	b.WriteString("### Destinations\n")
	if len(result.Broadcast.Destinations) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, destination := range result.Broadcast.Destinations {
			fmt.Fprintf(
				&b,
				"- destination=`%02d` target_issue=`#%d` outbound_comment_id=`%d` target_issue_created=`%t` duplicate_suppressed=`%t` route_sha256_12=`%s` channel=`%s` thread_id_sha256_12=`%s` message_id_sha256_12=`%s` body_sha256_12=`%s`\n",
				destination.Index,
				destination.IssueNumber,
				destination.CommentID,
				destination.Created,
				destination.Duplicate,
				noneIfEmpty(destination.RouteHash),
				destination.Channel,
				noneIfEmpty(destination.ThreadHash),
				noneIfEmpty(destination.MessageHash),
				noneIfEmpty(destination.BodyHash),
			)
		}
	}
	b.WriteString("\n### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending room invites with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent room invites with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate room invites are suppressed independently for each route by `channel + message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the room issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func findOrCreateChannelRoomIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRoomOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel room issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelRoomMatches(issue.Body, opts.RoomID) {
			return issue, false, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelRoomIssueTitle(opts), RenderChannelRoomIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel room issue: %w", err)
	}
	return issue, true, nil
}

func RenderChannelRoomIssueBody(opts ChannelRoomOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-room room_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" routes_sha256_12=\"%s\" notes_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.RoomID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(channelBroadcastRoutesHash(opts.Routes)), escapeMarkerValue(shortDocumentHash(opts.Notes)))
	b.WriteString("GitClaw channel room.\n\n")
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: `%s`\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- room_id_sha256_12: `%s`\n", shortDocumentHash(opts.RoomID))
	fmt.Fprintf(&b, "- room_mode: `%s`\n", "durable-issue-channel")
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", channelBroadcastRoutesHash(opts.Routes))
	fmt.Fprintf(&b, "- route_count: `%d`\n", len(normalizeChannelBroadcastRoutes(opts.Routes)))
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n\n", false)
	b.WriteString("## Topic\n\n")
	b.WriteString(strings.TrimSpace(opts.Topic))
	b.WriteString("\n")
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
		b.WriteString("\n")
	}
	b.WriteString("\nKeep talking here from GitHub, or through a mirrored channel thread invited into this room. Tag `@gitclaw` in this issue when you want the model-backed assistant to help.")
	return strings.TrimSpace(b.String())
}

func channelRoomActionFieldsAndTrailingNotes(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRoomActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func renderChannelRoomOutboundBody(opts ChannelRoomOptions, issueNumber int, url string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel room\n\n")
	if issueNumber > 0 {
		fmt.Fprintf(&b, "Room: #%d\n", issueNumber)
	}
	fmt.Fprintf(&b, "URL: %s\n", url)
	fmt.Fprintf(&b, "Source: #%d\n", opts.SourceIssueNumber)
	b.WriteString("\nTopic:\n")
	b.WriteString(strings.TrimSpace(opts.Topic))
	b.WriteString("\n")
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\nNotes:\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
		b.WriteString("\n")
	}
	b.WriteString("\nKeep talking in the linked GitHub room or through the mirrored channel thread.")
	return strings.TrimSpace(b.String())
}

func parseChannelRoomTopicNotes(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTopic := fmt.Sprintf("Channel room for issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTopic, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var topic string
	var noteLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "topic:"):
		topic = strings.TrimSpace(first[len("topic:"):])
		noteLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "context:"):
		topic = defaultTopic
		noteLines = cleaned
	default:
		topic = first
		noteLines = cleaned[1:]
	}
	if topic == "" {
		topic = defaultTopic
	}
	notes := strings.TrimSpace(strings.Join(noteLines, "\n"))
	notesLower := strings.ToLower(strings.TrimSpace(notes))
	switch {
	case strings.HasPrefix(notesLower, "notes:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("notes:"):])
	case strings.HasPrefix(notesLower, "context:"):
		notes = strings.TrimSpace(strings.TrimSpace(notes)[len("context:"):])
	}
	return topic, notes
}

func normalizeChannelRoomOptions(opts ChannelRoomOptions) ChannelRoomOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.RoomID = cleanChannelRoomID(opts.RoomID)
	opts.Topic = strings.TrimSpace(opts.Topic)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Routes = normalizeChannelBroadcastRoutes(opts.Routes)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelRoomOptions(opts ChannelRoomOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.RoomID == "" {
		return fmt.Errorf("missing room id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing room source issue")
	}
	if strings.TrimSpace(opts.Topic) == "" {
		return fmt.Errorf("missing room topic")
	}
	if len(normalizeChannelBroadcastRoutes(opts.Routes)) == 0 {
		return fmt.Errorf("missing room routes")
	}
	if strings.TrimSpace(opts.MessageID) == "" {
		return fmt.Errorf("missing room message id")
	}
	return nil
}

func channelRoomIssueTitle(opts ChannelRoomOptions) string {
	topic := strings.TrimSpace(opts.Topic)
	if topic == "" {
		topic = fmt.Sprintf("issue #%d", opts.SourceIssueNumber)
	}
	topic = strings.ReplaceAll(topic, "\n", " ")
	if len(topic) > 80 {
		topic = strings.TrimSpace(topic[:80])
	}
	return "GitClaw channel room: " + topic
}

func channelRoomMatches(body, roomID string) bool {
	return HasChannelRoomMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`room_id="%s"`, escapeMarkerValue(cleanChannelRoomID(roomID))))
}

func cleanChannelRoomID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelRoomID(ev Event, routes []string, topic, notes string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(normalizeChannelBroadcastRoutes(routes), ","), topic, notes}, "|")
	return fmt.Sprintf("room-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRoomMessageID(ev Event, roomID string, routes []string) string {
	seed := strings.Join([]string{eventID(ev), roomID, strings.Join(normalizeChannelBroadcastRoutes(routes), ",")}, "|")
	return fmt.Sprintf("gitclaw-room-%s-%s", eventID(ev), shortDocumentHash(seed))
}
