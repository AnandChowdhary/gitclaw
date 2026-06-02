package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBoardCardOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	BoardCardID       string
	Lane              string
	Owner             string
	Title             string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBoardCardResult struct {
	BoardCardIssueNumber int
	BoardCardIssueURL    string
	BoardCardCreated     bool
	BoardCardDuplicate   bool
	Notification         ChannelSendResult
	RouteName            string
	RouteHash            string
	Channel              string
	ThreadHash           string
	MessageHash          string
	NotifyHash           string
}

type ChannelBoardCardActionRequest struct {
	Options             ChannelBoardCardOptions
	Command             string
	Subcommand          string
	AutoBoardCardID     bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	LaneSHA             string
	LaneBytes           int
	LaneLines           int
	OwnerSHA            string
	OwnerBytes          int
	OwnerLines          int
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelBoardCardActionRequest(ev Event, cfg Config) bool {
	return isChannelBoardCardActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBoardCardActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "card", "board-card", "kanban", "kanban-card", "lane-card", "work-card", "queue-card":
		return true
	default:
		return false
	}
}

func BuildChannelBoardCardActionRequest(ev Event, cfg Config) (ChannelBoardCardActionRequest, error) {
	fields, trailing, ok := channelBoardCardActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBoardCardActionRequest{}, fmt.Errorf("missing channel board card command")
	}
	req := ChannelBoardCardActionRequest{
		Options: ChannelBoardCardOptions{
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
				return ChannelBoardCardActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--card-id", "--board-card-id", "--kanban-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.BoardCardID = cleanChannelBoardCardID(fields[i+1])
			i++
		case "--lane", "--column", "--status":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Lane = fields[i+1]
			i++
		case "--owner", "--assignee", "--responsible":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Owner = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBoardCardActionRequest{}, fmt.Errorf("unknown channel board card argument %q", field)
			}
			if req.Options.BoardCardID == "" {
				req.Options.BoardCardID = cleanChannelBoardCardID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBoardCardActionRequest{}, fmt.Errorf("unexpected channel board card argument %q", field)
		}
	}
	if err := applyChannelBoardCardIssueTarget(ev, &req); err != nil {
		return ChannelBoardCardActionRequest{}, err
	}
	lane, owner, title, notes := parseChannelBoardCardSections(trailing, ev)
	if strings.TrimSpace(req.Options.Lane) == "" {
		req.Options.Lane = lane
	}
	if strings.TrimSpace(req.Options.Owner) == "" {
		req.Options.Owner = owner
	}
	req.Options.Title = title
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.BoardCardID) == "" {
		req.Options.BoardCardID = autoChannelBoardCardID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Lane, req.Options.Owner, title, notes)
		req.AutoBoardCardID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBoardCardNotifyMessageID(ev, req.Options.BoardCardID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBoardCardOptions(req.Options)
	if err := validateChannelBoardCardActionRequestOptions(req.Options); err != nil {
		return ChannelBoardCardActionRequest{}, err
	}
	req.LaneSHA = shortDocumentHash(req.Options.Lane)
	req.LaneBytes = len(req.Options.Lane)
	req.LaneLines = lineCount(req.Options.Lane)
	req.OwnerSHA = shortDocumentHash(req.Options.Owner)
	req.OwnerBytes = len(req.Options.Owner)
	req.OwnerLines = lineCount(req.Options.Owner)
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelBoardCardNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelBoardCard(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBoardCardOptions) (ChannelBoardCardResult, error) {
	opts = normalizeChannelBoardCardOptions(opts)
	var err error
	opts, err = applyChannelBoardCardRoute(cfg, opts)
	if err != nil {
		return ChannelBoardCardResult{}, err
	}
	if err := validateChannelBoardCardOptions(opts); err != nil {
		return ChannelBoardCardResult{}, err
	}
	cardIssue, created, duplicate, err := findOrCreateChannelBoardCardIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelBoardCardResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelBoardCardNotificationBody(opts, cardIssue.Number, issueURL(opts.Repo, cardIssue.Number)),
	})
	if err != nil {
		return ChannelBoardCardResult{}, fmt.Errorf("queue channel board card notification: %w", err)
	}
	return ChannelBoardCardResult{
		BoardCardIssueNumber: cardIssue.Number,
		BoardCardIssueURL:    issueURL(opts.Repo, cardIssue.Number),
		BoardCardCreated:     created,
		BoardCardDuplicate:   duplicate,
		Notification:         notification,
		RouteName:            opts.Route,
		RouteHash:            channelRouteHash(opts.Route),
		Channel:              opts.Channel,
		ThreadHash:           shortDocumentHash(opts.ThreadID),
		MessageHash:          shortDocumentHash(opts.SourceMessageID),
		NotifyHash:           shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelBoardCardActionReport(ev Event, req ChannelBoardCardActionRequest, result ChannelBoardCardResult) string {
	status := "captured"
	switch {
	case result.BoardCardDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.BoardCardDuplicate:
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
	b.WriteString("## GitClaw Channel Board Card Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_board_card_status: `%s`\n", status)
	fmt.Fprintf(&b, "- board_card_issue: `#%d`\n", result.BoardCardIssueNumber)
	fmt.Fprintf(&b, "- board_card_issue_url: `%s`\n", result.BoardCardIssueURL)
	fmt.Fprintf(&b, "- board_card_issue_created: `%t`\n", result.BoardCardCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.BoardCardDuplicate)
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
	fmt.Fprintf(&b, "- board_card_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.BoardCardID))
	fmt.Fprintf(&b, "- board_card_id_auto: `%t`\n", req.AutoBoardCardID)
	fmt.Fprintf(&b, "- board_card_lane_sha256_12: `%s`\n", req.LaneSHA)
	fmt.Fprintf(&b, "- board_card_lane_bytes: `%d`\n", req.LaneBytes)
	fmt.Fprintf(&b, "- board_card_lane_lines: `%d`\n", req.LaneLines)
	fmt.Fprintf(&b, "- board_card_owner_sha256_12: `%s`\n", req.OwnerSHA)
	fmt.Fprintf(&b, "- board_card_owner_bytes: `%d`\n", req.OwnerBytes)
	fmt.Fprintf(&b, "- board_card_owner_lines: `%d`\n", req.OwnerLines)
	fmt.Fprintf(&b, "- board_card_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- board_card_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- board_card_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- board_card_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- board_card_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- board_card_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- board_card_mode: `%s`\n", "github-issue-board-card")
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_board_card_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_board_card_lane_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_board_card_owner_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_board_card_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_board_card_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_board_card_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin board card as a durable GitHub issue, then queued a provider-facing board-card link back to the mirrored thread. The board-card issue contains the human-readable lane, owner, title, and notes; this source receipt keeps provider IDs, board-card IDs, lane, owner, titles, notes, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the board-card link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent board-card links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate board-card issues are suppressed by `board_card_id`; duplicate board-card link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the board-card issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelBoardCardIssueBody(opts ChannelBoardCardOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-board-card board_card_id=\"%s\" channel=\"%s\" lane_sha256_12=\"%s\" owner_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.BoardCardID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Lane), shortDocumentHash(opts.Owner), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel board card.\n\n")
	fmt.Fprintf(&b, "- board_card_id: %s\n", opts.BoardCardID)
	fmt.Fprintf(&b, "- lane: %s\n", opts.Lane)
	if strings.TrimSpace(opts.Owner) != "" {
		fmt.Fprintf(&b, "- owner: %s\n", opts.Owner)
	}
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- board_card_mode: github-issue-board-card\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Card\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	b.WriteString("\n\n## Lane\n\n")
	b.WriteString(strings.TrimSpace(opts.Lane))
	if strings.TrimSpace(opts.Owner) != "" {
		b.WriteString("\n\n## Owner\n\n")
		b.WriteString(strings.TrimSpace(opts.Owner))
	}
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for tracking this channel-created board card. Any status changes, task conversion, skill work, or proactive follow-up should happen through normal GitHub conversation.")
	return strings.TrimSpace(b.String())
}

func channelBoardCardActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBoardCardActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBoardCardIssueTarget(ev Event, req *ChannelBoardCardActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel board card requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelBoardCardSections(trailing string, ev Event) (string, string, string, string) {
	lines := cleanChannelBoardCardTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel board card from issue #%d", ev.Issue.Number)
	card := channelBoardCardParsedSections{Lane: "triage", Title: defaultTitle}
	if len(lines) == 0 {
		return card.Lane, card.Owner, card.Title, card.Notes
	}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				card.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelBoardCardSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				card.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			card.Title = trimmed
			continue
		}
		if current == "" {
			current = "notes"
		}
		card.append(current, line)
	}
	return strings.TrimSpace(card.Lane), strings.TrimSpace(card.Owner), strings.TrimSpace(card.Title), strings.TrimSpace(card.Notes)
}

type channelBoardCardParsedSections struct {
	Lane  string
	Owner string
	Title string
	Notes string
}

func (sections *channelBoardCardParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	switch section {
	case "lane":
		sections.Lane = strings.TrimSpace(value)
	case "owner":
		sections.Owner = strings.TrimSpace(value)
	case "title":
		sections.Title = strings.TrimSpace(value)
	default:
		sections.append(section, value)
	}
}

func (sections *channelBoardCardParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	if section == "notes" {
		sections.Notes = appendChannelBoardCardSectionLine(sections.Notes, value)
	}
}

func appendChannelBoardCardSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelBoardCardSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelBoardCardHeader(name) {
	case "lane":
		return "lane", strings.TrimSpace(value), true
	case "owner":
		return "owner", strings.TrimSpace(value), true
	case "title":
		return "title", strings.TrimSpace(value), true
	case "notes":
		return "notes", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelBoardCardHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "lane", "column", "status", "stage":
		return "lane"
	case "owner", "assignee", "responsible", "lead":
		return "owner"
	case "title", "card", "board card", "task", "work item":
		return "title"
	case "notes", "context", "details", "description", "acceptance", "acceptance criteria", "why":
		return "notes"
	default:
		return ""
	}
}

func cleanChannelBoardCardTrailingLines(trailing string) []string {
	rawLines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned
}

func normalizeChannelBoardCardOptions(opts ChannelBoardCardOptions) ChannelBoardCardOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.BoardCardID = cleanChannelBoardCardID(opts.BoardCardID)
	opts.Lane = cleanChannelBoardCardLane(opts.Lane)
	if opts.Lane == "" {
		opts.Lane = "triage"
	}
	opts.Owner = strings.TrimSpace(opts.Owner)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBoardCardRoute(cfg Config, opts ChannelBoardCardOptions) (ChannelBoardCardOptions, error) {
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

func validateChannelBoardCardOptions(opts ChannelBoardCardOptions) error {
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
	if opts.BoardCardID == "" {
		return fmt.Errorf("missing board card id")
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing board card lane")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing board card source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing board card title")
	}
	return nil
}

func validateChannelBoardCardActionRequestOptions(opts ChannelBoardCardOptions) error {
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
	if opts.BoardCardID == "" {
		return fmt.Errorf("missing board card id")
	}
	if opts.Lane == "" {
		return fmt.Errorf("missing board card lane")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing board card source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing board card title")
	}
	return nil
}

func findOrCreateChannelBoardCardIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBoardCardOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel board card issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelBoardCardMatches(issue.Body, opts.BoardCardID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelBoardCardIssueTitle(opts), RenderChannelBoardCardIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel board card issue: %w", err)
	}
	return issue, true, false, nil
}

func channelBoardCardIssueTitle(opts ChannelBoardCardOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.BoardCardID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel board card: " + title
}

func channelBoardCardMatches(body, boardCardID string) bool {
	return HasChannelBoardCardMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`board_card_id="%s"`, escapeMarkerValue(cleanChannelBoardCardID(boardCardID))))
}

func cleanChannelBoardCardID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelBoardCardLane(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Join(strings.Fields(value), "-")
	return cleanChannelHuddleID(value)
}

func autoChannelBoardCardID(ev Event, channel, threadID, sourceMessageID, lane, owner, title, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, lane, owner, title, notes}, "|")
	return fmt.Sprintf("board-card-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBoardCardNotifyMessageID(ev Event, boardCardID string) string {
	seed := strings.Join([]string{eventID(ev), boardCardID}, "|")
	return fmt.Sprintf("gitclaw-channel-board-card-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelBoardCardNotificationBody(opts ChannelBoardCardOptions, cardIssueNumber int, cardIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel board card captured.\n\n")
	if cardIssueNumber > 0 {
		fmt.Fprintf(&b, "Board card: #%d\n", cardIssueNumber)
	}
	if cardIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", cardIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	fmt.Fprintf(&b, "Lane: %s\n", strings.TrimSpace(opts.Lane))
	if strings.TrimSpace(opts.Owner) != "" {
		fmt.Fprintf(&b, "Owner: %s\n", strings.TrimSpace(opts.Owner))
	}
	b.WriteString("\nContinue tracking it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
