package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelQuestOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	QuestID           string
	Title             string
	Objective         string
	FirstMove         string
	WinCondition      string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelQuestResult struct {
	QuestIssueNumber int
	QuestIssueURL    string
	QuestCreated     bool
	QuestDuplicate   bool
	Notification     ChannelSendResult
	RouteName        string
	RouteHash        string
	Channel          string
	ThreadHash       string
	MessageHash      string
	NotifyHash       string
}

type ChannelQuestActionRequest struct {
	Options             ChannelQuestOptions
	Command             string
	Subcommand          string
	AutoQuestID         bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ObjectiveSHA        string
	ObjectiveBytes      int
	ObjectiveLines      int
	FirstMoveSHA        string
	FirstMoveBytes      int
	FirstMoveLines      int
	WinConditionSHA     string
	WinConditionBytes   int
	WinConditionLines   int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelQuestActionRequest(ev Event, cfg Config) bool {
	return isChannelQuestActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelQuestActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "quest", "challenge", "side-quest", "mini-quest", "mission", "experiment":
		return true
	default:
		return false
	}
}

func BuildChannelQuestActionRequest(ev Event, cfg Config) (ChannelQuestActionRequest, error) {
	fields, trailing, ok := channelQuestActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelQuestActionRequest{}, fmt.Errorf("missing channel quest command")
	}
	req := ChannelQuestActionRequest{
		Options: ChannelQuestOptions{
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
				return ChannelQuestActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelQuestActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelQuestActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelQuestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelQuestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--quest-id", "--challenge-id", "--mission-id", "--experiment-id", "--id":
			if i+1 >= len(fields) {
				return ChannelQuestActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.QuestID = cleanChannelQuestID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelQuestActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelQuestActionRequest{}, fmt.Errorf("unknown channel quest argument %q", field)
			}
			if req.Options.QuestID == "" {
				req.Options.QuestID = cleanChannelQuestID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelQuestActionRequest{}, fmt.Errorf("unexpected channel quest argument %q", field)
		}
	}
	if err := applyChannelQuestIssueTarget(ev, &req); err != nil {
		return ChannelQuestActionRequest{}, err
	}
	title, objective, firstMove, winCondition := parseChannelQuestSections(trailing, ev)
	req.Options.Title = title
	req.Options.Objective = objective
	req.Options.FirstMove = firstMove
	req.Options.WinCondition = winCondition
	if strings.TrimSpace(req.Options.QuestID) == "" {
		req.Options.QuestID = autoChannelQuestID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, objective, firstMove, winCondition)
		req.AutoQuestID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelQuestNotifyMessageID(ev, req.Options.QuestID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelQuestOptions(req.Options)
	if err := validateChannelQuestActionRequestOptions(req.Options); err != nil {
		return ChannelQuestActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ObjectiveSHA = shortDocumentHash(req.Options.Objective)
	req.ObjectiveBytes = len(req.Options.Objective)
	req.ObjectiveLines = lineCount(req.Options.Objective)
	req.FirstMoveSHA = shortDocumentHash(req.Options.FirstMove)
	req.FirstMoveBytes = len(req.Options.FirstMove)
	req.FirstMoveLines = lineCount(req.Options.FirstMove)
	req.WinConditionSHA = shortDocumentHash(req.Options.WinCondition)
	req.WinConditionBytes = len(req.Options.WinCondition)
	req.WinConditionLines = lineCount(req.Options.WinCondition)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelQuestNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelQuest(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelQuestOptions) (ChannelQuestResult, error) {
	opts = normalizeChannelQuestOptions(opts)
	var err error
	opts, err = applyChannelQuestRoute(cfg, opts)
	if err != nil {
		return ChannelQuestResult{}, err
	}
	if err := validateChannelQuestOptions(opts); err != nil {
		return ChannelQuestResult{}, err
	}
	questIssue, created, duplicate, err := findOrCreateChannelQuestIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelQuestResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelQuestNotificationBody(opts, questIssue.Number, issueURL(opts.Repo, questIssue.Number)),
	})
	if err != nil {
		return ChannelQuestResult{}, fmt.Errorf("queue channel quest notification: %w", err)
	}
	return ChannelQuestResult{
		QuestIssueNumber: questIssue.Number,
		QuestIssueURL:    issueURL(opts.Repo, questIssue.Number),
		QuestCreated:     created,
		QuestDuplicate:   duplicate,
		Notification:     notification,
		RouteName:        opts.Route,
		RouteHash:        channelRouteHash(opts.Route),
		Channel:          opts.Channel,
		ThreadHash:       shortDocumentHash(opts.ThreadID),
		MessageHash:      shortDocumentHash(opts.SourceMessageID),
		NotifyHash:       shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelQuestActionReport(ev Event, req ChannelQuestActionRequest, result ChannelQuestResult) string {
	status := "recorded"
	switch {
	case result.QuestDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.QuestDuplicate:
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
	b.WriteString("## GitClaw Channel Quest Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_quest_status: `%s`\n", status)
	fmt.Fprintf(&b, "- quest_issue: `#%d`\n", result.QuestIssueNumber)
	fmt.Fprintf(&b, "- quest_issue_url: `%s`\n", result.QuestIssueURL)
	fmt.Fprintf(&b, "- quest_issue_created: `%t`\n", result.QuestCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.QuestDuplicate)
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
	fmt.Fprintf(&b, "- quest_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.QuestID))
	fmt.Fprintf(&b, "- quest_id_auto: `%t`\n", req.AutoQuestID)
	fmt.Fprintf(&b, "- quest_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- quest_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- quest_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- quest_objective_sha256_12: `%s`\n", req.ObjectiveSHA)
	fmt.Fprintf(&b, "- quest_objective_bytes: `%d`\n", req.ObjectiveBytes)
	fmt.Fprintf(&b, "- quest_objective_lines: `%d`\n", req.ObjectiveLines)
	fmt.Fprintf(&b, "- quest_first_move_sha256_12: `%s`\n", req.FirstMoveSHA)
	fmt.Fprintf(&b, "- quest_first_move_bytes: `%d`\n", req.FirstMoveBytes)
	fmt.Fprintf(&b, "- quest_first_move_lines: `%d`\n", req.FirstMoveLines)
	fmt.Fprintf(&b, "- quest_win_condition_sha256_12: `%s`\n", req.WinConditionSHA)
	fmt.Fprintf(&b, "- quest_win_condition_bytes: `%d`\n", req.WinConditionBytes)
	fmt.Fprintf(&b, "- quest_win_condition_lines: `%d`\n", req.WinConditionLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_quest_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quest_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quest_objective_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quest_first_move_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_quest_win_condition_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_quest_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel quest as a durable GitHub issue, then queued a provider-facing link back to the original thread. The quest issue contains the human-readable title, objective, first move, and win condition; this source receipt keeps provider IDs, quest IDs, section text, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the quest-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent quest links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate quest issues are suppressed by `quest_id`; duplicate quest-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the quest issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelQuestIssueBody(opts ChannelQuestOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-quest quest_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.QuestID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel quest.\n\n")
	fmt.Fprintf(&b, "- quest_id: %s\n", opts.QuestID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- quest_mode: github-issue-quest\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Objective) != "" {
		b.WriteString("\n\n## Objective\n\n")
		b.WriteString(strings.TrimSpace(opts.Objective))
	}
	if strings.TrimSpace(opts.FirstMove) != "" {
		b.WriteString("\n\n## First Move\n\n")
		b.WriteString(strings.TrimSpace(opts.FirstMove))
	}
	if strings.TrimSpace(opts.WinCondition) != "" {
		b.WriteString("\n\n## Win Condition\n\n")
		b.WriteString(strings.TrimSpace(opts.WinCondition))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel quest.")
	return strings.TrimSpace(b.String())
}

func channelQuestActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelQuestActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelQuestIssueTarget(ev Event, req *ChannelQuestActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel quest requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelQuestSections(trailing string, ev Event) (string, string, string, string) {
	lines := cleanChannelQuestTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel quest from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", ""
	}
	quest := channelQuestParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				quest.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelQuestSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				quest.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			quest.Title = trimmed
			continue
		}
		if current == "" {
			current = "objective"
		}
		quest.append(current, line)
	}
	return strings.TrimSpace(quest.Title), strings.TrimSpace(quest.Objective), strings.TrimSpace(quest.FirstMove), strings.TrimSpace(quest.WinCondition)
}

type channelQuestParsedSections struct {
	Title        string
	Objective    string
	FirstMove    string
	WinCondition string
}

func (sections *channelQuestParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelQuestParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "objective":
		sections.Objective = appendChannelQuestSectionLine(sections.Objective, value)
	case "first_move":
		sections.FirstMove = appendChannelQuestSectionLine(sections.FirstMove, value)
	case "win_condition":
		sections.WinCondition = appendChannelQuestSectionLine(sections.WinCondition, value)
	}
}

func appendChannelQuestSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelQuestSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelQuestHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "objective":
		return "objective", strings.TrimSpace(value), true
	case "first_move":
		return "first_move", strings.TrimSpace(value), true
	case "win_condition":
		return "win_condition", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelQuestHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "quest", "challenge", "mission", "experiment", "name":
		return "title"
	case "objective", "goal", "purpose", "intent", "why", "constraints", "rules":
		return "objective"
	case "first move", "first step", "next step", "next move", "start", "opening move":
		return "first_move"
	case "win condition", "success", "done when", "complete when", "acceptance", "finish", "reward":
		return "win_condition"
	default:
		return ""
	}
}

func cleanChannelQuestTrailingLines(trailing string) []string {
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

func normalizeChannelQuestOptions(opts ChannelQuestOptions) ChannelQuestOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.QuestID = cleanChannelQuestID(opts.QuestID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Objective = strings.TrimSpace(opts.Objective)
	opts.FirstMove = strings.TrimSpace(opts.FirstMove)
	opts.WinCondition = strings.TrimSpace(opts.WinCondition)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelQuestRoute(cfg Config, opts ChannelQuestOptions) (ChannelQuestOptions, error) {
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

func validateChannelQuestOptions(opts ChannelQuestOptions) error {
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
	if opts.QuestID == "" {
		return fmt.Errorf("missing quest id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing quest source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing quest title")
	}
	return nil
}

func validateChannelQuestActionRequestOptions(opts ChannelQuestOptions) error {
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
	if opts.QuestID == "" {
		return fmt.Errorf("missing quest id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing quest source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing quest title")
	}
	return nil
}

func findOrCreateChannelQuestIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelQuestOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel quest issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelQuestMatches(issue.Body, opts.QuestID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelQuestIssueTitle(opts), RenderChannelQuestIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel quest issue: %w", err)
	}
	return issue, true, false, nil
}

func channelQuestIssueTitle(opts ChannelQuestOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.QuestID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel quest: " + title
}

func channelQuestMatches(body, questID string) bool {
	return HasChannelQuestMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`quest_id="%s"`, escapeMarkerValue(cleanChannelQuestID(questID))))
}

func cleanChannelQuestID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelQuestID(ev Event, channel, threadID, sourceMessageID, title, objective, firstMove, winCondition string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, objective, firstMove, winCondition}, "|")
	return fmt.Sprintf("quest-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelQuestNotifyMessageID(ev Event, questID string) string {
	seed := strings.Join([]string{eventID(ev), questID}, "|")
	return fmt.Sprintf("gitclaw-channel-quest-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelQuestNotificationBody(opts ChannelQuestOptions, questIssueNumber int, questIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel quest recorded.\n\n")
	if questIssueNumber > 0 {
		fmt.Fprintf(&b, "Quest: #%d\n", questIssueNumber)
	}
	if questIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", questIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
