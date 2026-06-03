package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelLoreOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	LoreID            string
	Title             string
	Lore              string
	Context           string
	Source            string
	Review            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelLoreResult struct {
	LoreIssueNumber int
	LoreIssueURL    string
	LoreCreated     bool
	LoreDuplicate   bool
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
}

type ChannelLoreActionRequest struct {
	Options             ChannelLoreOptions
	Command             string
	Subcommand          string
	AutoLoreID          bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	LoreSHA             string
	LoreBytes           int
	LoreLines           int
	ContextSHA          string
	ContextBytes        int
	ContextLines        int
	SourceSHA           string
	SourceBytes         int
	SourceLines         int
	ReviewSHA           string
	ReviewBytes         int
	ReviewLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelLoreActionRequest(ev Event, cfg Config) bool {
	return isChannelLoreActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelLoreActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "lore", "context-note", "shared-context", "canon", "thread-lore", "background":
		return true
	default:
		return false
	}
}

func BuildChannelLoreActionRequest(ev Event, cfg Config) (ChannelLoreActionRequest, error) {
	fields, trailing, ok := channelLoreActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelLoreActionRequest{}, fmt.Errorf("missing channel lore command")
	}
	req := ChannelLoreActionRequest{
		Options: ChannelLoreOptions{
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
				return ChannelLoreActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelLoreActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelLoreActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelLoreActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelLoreActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--lore-id", "--context-id", "--canon-id", "--id":
			if i+1 >= len(fields) {
				return ChannelLoreActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.LoreID = cleanChannelLoreID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelLoreActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelLoreActionRequest{}, fmt.Errorf("unknown channel lore argument %q", field)
			}
			if req.Options.LoreID == "" {
				req.Options.LoreID = cleanChannelLoreID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelLoreActionRequest{}, fmt.Errorf("unexpected channel lore argument %q", field)
		}
	}
	if err := applyChannelLoreIssueTarget(ev, &req); err != nil {
		return ChannelLoreActionRequest{}, err
	}
	title, lore, context, source, review := parseChannelLoreSections(trailing, ev)
	req.Options.Title = title
	req.Options.Lore = lore
	req.Options.Context = context
	req.Options.Source = source
	req.Options.Review = review
	if strings.TrimSpace(req.Options.LoreID) == "" {
		req.Options.LoreID = autoChannelLoreID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, lore, context, source, review)
		req.AutoLoreID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelLoreNotifyMessageID(ev, req.Options.LoreID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelLoreOptions(req.Options)
	if err := validateChannelLoreActionRequestOptions(req.Options); err != nil {
		return ChannelLoreActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.LoreSHA = shortDocumentHash(req.Options.Lore)
	req.LoreBytes = len(req.Options.Lore)
	req.LoreLines = lineCount(req.Options.Lore)
	req.ContextSHA = shortDocumentHash(req.Options.Context)
	req.ContextBytes = len(req.Options.Context)
	req.ContextLines = lineCount(req.Options.Context)
	req.SourceSHA = shortDocumentHash(req.Options.Source)
	req.SourceBytes = len(req.Options.Source)
	req.SourceLines = lineCount(req.Options.Source)
	req.ReviewSHA = shortDocumentHash(req.Options.Review)
	req.ReviewBytes = len(req.Options.Review)
	req.ReviewLines = lineCount(req.Options.Review)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelLoreNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelLore(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelLoreOptions) (ChannelLoreResult, error) {
	opts = normalizeChannelLoreOptions(opts)
	var err error
	opts, err = applyChannelLoreRoute(cfg, opts)
	if err != nil {
		return ChannelLoreResult{}, err
	}
	if err := validateChannelLoreOptions(opts); err != nil {
		return ChannelLoreResult{}, err
	}
	loreIssue, created, duplicate, err := findOrCreateChannelLoreIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelLoreResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelLoreNotificationBody(opts, loreIssue.Number, issueURL(opts.Repo, loreIssue.Number)),
	})
	if err != nil {
		return ChannelLoreResult{}, fmt.Errorf("queue channel lore notification: %w", err)
	}
	return ChannelLoreResult{
		LoreIssueNumber: loreIssue.Number,
		LoreIssueURL:    issueURL(opts.Repo, loreIssue.Number),
		LoreCreated:     created,
		LoreDuplicate:   duplicate,
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelLoreActionReport(ev Event, req ChannelLoreActionRequest, result ChannelLoreResult) string {
	status := "recorded"
	switch {
	case result.LoreDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.LoreDuplicate:
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
	b.WriteString("## GitClaw Channel Lore Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_lore_status: `%s`\n", status)
	fmt.Fprintf(&b, "- lore_issue: `#%d`\n", result.LoreIssueNumber)
	fmt.Fprintf(&b, "- lore_issue_url: `%s`\n", result.LoreIssueURL)
	fmt.Fprintf(&b, "- lore_issue_created: `%t`\n", result.LoreCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.LoreDuplicate)
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
	fmt.Fprintf(&b, "- lore_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.LoreID))
	fmt.Fprintf(&b, "- lore_id_auto: `%t`\n", req.AutoLoreID)
	fmt.Fprintf(&b, "- lore_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- lore_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- lore_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- lore_body_sha256_12: `%s`\n", req.LoreSHA)
	fmt.Fprintf(&b, "- lore_body_bytes: `%d`\n", req.LoreBytes)
	fmt.Fprintf(&b, "- lore_body_lines: `%d`\n", req.LoreLines)
	fmt.Fprintf(&b, "- lore_context_sha256_12: `%s`\n", req.ContextSHA)
	fmt.Fprintf(&b, "- lore_context_bytes: `%d`\n", req.ContextBytes)
	fmt.Fprintf(&b, "- lore_context_lines: `%d`\n", req.ContextLines)
	fmt.Fprintf(&b, "- lore_source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- lore_source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- lore_source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- lore_review_sha256_12: `%s`\n", req.ReviewSHA)
	fmt.Fprintf(&b, "- lore_review_bytes: `%d`\n", req.ReviewBytes)
	fmt.Fprintf(&b, "- lore_review_lines: `%d`\n", req.ReviewLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_lore_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_lore_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_lore_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_lore_context_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_lore_source_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_lore_review_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_lore_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded mirrored channel lore as a durable GitHub issue, then queued a provider-facing link back to the original thread. The lore issue contains the human-readable title, lore, context, source, and review timing; this source receipt keeps provider IDs, lore IDs, section text, and channel message bodies out of band. No model call, schedule, reminder, soul write, memory write, policy mutation, skill install, provider delivery, or repository mutation is created by this action.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the lore-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent lore links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate lore issues are suppressed by `lore_id`; duplicate lore-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the lore issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelLoreIssueBody(opts ChannelLoreOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-lore lore_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.LoreID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel lore.\n\n")
	fmt.Fprintf(&b, "- lore_id: %s\n", opts.LoreID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- lore_mode: github-issue-lore\n")
	fmt.Fprintf(&b, "- model_call_performed: false\n")
	fmt.Fprintf(&b, "- scheduled_workflow_created: false\n")
	fmt.Fprintf(&b, "- reminder_created: false\n")
	fmt.Fprintf(&b, "- soul_write_performed: false\n")
	fmt.Fprintf(&b, "- memory_write_performed: false\n")
	fmt.Fprintf(&b, "- policy_mutation_performed: false\n")
	fmt.Fprintf(&b, "- skill_install_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Lore) != "" {
		b.WriteString("\n\n## Lore\n\n")
		b.WriteString(strings.TrimSpace(opts.Lore))
	}
	if strings.TrimSpace(opts.Context) != "" {
		b.WriteString("\n\n## Context\n\n")
		b.WriteString(strings.TrimSpace(opts.Context))
	}
	if strings.TrimSpace(opts.Source) != "" {
		b.WriteString("\n\n## Source\n\n")
		b.WriteString(strings.TrimSpace(opts.Source))
	}
	if strings.TrimSpace(opts.Review) != "" {
		b.WriteString("\n\n## Review\n\n")
		b.WriteString(strings.TrimSpace(opts.Review))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for channel lore. Close it when the lore is stale, superseded, or promoted through normal reviewed GitHub issues.")
	return strings.TrimSpace(b.String())
}

func channelLoreActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelLoreActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelLoreIssueTarget(ev Event, req *ChannelLoreActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel lore requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelLoreSections(trailing string, ev Event) (string, string, string, string, string) {
	lines := cleanChannelLoreTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel lore from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", "", ""
	}
	lore := channelLoreParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				lore.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelLoreSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				lore.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			lore.Title = trimmed
			continue
		}
		if current == "" {
			current = "lore"
		}
		lore.append(current, line)
	}
	return strings.TrimSpace(lore.Title), strings.TrimSpace(lore.Lore), strings.TrimSpace(lore.Context), strings.TrimSpace(lore.Source), strings.TrimSpace(lore.Review)
}

type channelLoreParsedSections struct {
	Title   string
	Lore    string
	Context string
	Source  string
	Review  string
}

func (sections *channelLoreParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelLoreParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "lore":
		sections.Lore = appendChannelLoreSectionLine(sections.Lore, value)
	case "context":
		sections.Context = appendChannelLoreSectionLine(sections.Context, value)
	case "source":
		sections.Source = appendChannelLoreSectionLine(sections.Source, value)
	case "review":
		sections.Review = appendChannelLoreSectionLine(sections.Review, value)
	}
}

func appendChannelLoreSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelLoreSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelLoreHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "lore":
		return "lore", strings.TrimSpace(value), true
	case "context":
		return "context", strings.TrimSpace(value), true
	case "source":
		return "source", strings.TrimSpace(value), true
	case "review":
		return "review", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelLoreHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "lore title", "context title", "heading", "caption", "name":
		return "title"
	case "lore", "context note", "shared context", "canon", "note", "story", "what":
		return "lore"
	case "context", "background", "why", "situation", "because":
		return "context"
	case "source", "origin", "from", "where":
		return "source"
	case "review", "revisit", "stale when", "expiry", "expires", "check":
		return "review"
	default:
		return ""
	}
}

func cleanChannelLoreTrailingLines(trailing string) []string {
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

func normalizeChannelLoreOptions(opts ChannelLoreOptions) ChannelLoreOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.LoreID = cleanChannelLoreID(opts.LoreID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Lore = strings.TrimSpace(opts.Lore)
	opts.Context = strings.TrimSpace(opts.Context)
	opts.Source = strings.TrimSpace(opts.Source)
	opts.Review = strings.TrimSpace(opts.Review)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelLoreRoute(cfg Config, opts ChannelLoreOptions) (ChannelLoreOptions, error) {
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

func validateChannelLoreOptions(opts ChannelLoreOptions) error {
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
	if opts.LoreID == "" {
		return fmt.Errorf("missing lore id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing lore source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing lore title")
	}
	return nil
}

func validateChannelLoreActionRequestOptions(opts ChannelLoreOptions) error {
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
	if opts.LoreID == "" {
		return fmt.Errorf("missing lore id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing lore source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing lore title")
	}
	return nil
}

func findOrCreateChannelLoreIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelLoreOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel lore issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelLoreMatches(issue.Body, opts.LoreID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelLoreIssueTitle(opts), RenderChannelLoreIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel lore issue: %w", err)
	}
	return issue, true, false, nil
}

func channelLoreIssueTitle(opts ChannelLoreOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.LoreID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel lore: " + title
}

func channelLoreMatches(body, loreID string) bool {
	return HasChannelLoreMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`lore_id="%s"`, escapeMarkerValue(cleanChannelLoreID(loreID))))
}

func cleanChannelLoreID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelLoreID(ev Event, channel, threadID, sourceMessageID, title, lore, context, source, review string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, lore, context, source, review}, "|")
	return fmt.Sprintf("lore-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelLoreNotifyMessageID(ev Event, loreID string) string {
	seed := strings.Join([]string{eventID(ev), loreID}, "|")
	return fmt.Sprintf("gitclaw-channel-lore-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelLoreNotificationBody(opts ChannelLoreOptions, loreIssueNumber int, loreIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel lore recorded.\n\n")
	if loreIssueNumber > 0 {
		fmt.Fprintf(&b, "Lore: #%d\n", loreIssueNumber)
	}
	if loreIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", loreIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Review) != "" {
		fmt.Fprintf(&b, "Review: %s\n", strings.TrimSpace(opts.Review))
	}
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
