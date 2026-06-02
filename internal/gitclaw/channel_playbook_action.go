package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPlaybookOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	PlaybookID        string
	Title             string
	Steps             string
	Checks            string
	Rollback          string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelPlaybookResult struct {
	PlaybookIssueNumber int
	PlaybookIssueURL    string
	PlaybookCreated     bool
	PlaybookDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelPlaybookActionRequest struct {
	Options             ChannelPlaybookOptions
	Command             string
	Subcommand          string
	AutoPlaybookID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	StepsSHA            string
	StepsBytes          int
	StepsLines          int
	ChecksSHA           string
	ChecksBytes         int
	ChecksLines         int
	RollbackSHA         string
	RollbackBytes       int
	RollbackLines       int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelPlaybookActionRequest(ev Event, cfg Config) bool {
	return isChannelPlaybookActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPlaybookActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "playbook", "runbook", "procedure", "recipe", "sop", "checklist", "standard-operating-procedure":
		return true
	default:
		return false
	}
}

func BuildChannelPlaybookActionRequest(ev Event, cfg Config) (ChannelPlaybookActionRequest, error) {
	fields, trailing, ok := channelPlaybookActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPlaybookActionRequest{}, fmt.Errorf("missing channel playbook command")
	}
	req := ChannelPlaybookActionRequest{
		Options: ChannelPlaybookOptions{
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
				return ChannelPlaybookActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--playbook-id", "--runbook-id", "--procedure-id", "--recipe-id", "--sop-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PlaybookID = cleanChannelPlaybookID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPlaybookActionRequest{}, fmt.Errorf("unknown channel playbook argument %q", field)
			}
			if req.Options.PlaybookID == "" {
				req.Options.PlaybookID = cleanChannelPlaybookID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelPlaybookActionRequest{}, fmt.Errorf("unexpected channel playbook argument %q", field)
		}
	}
	if err := applyChannelPlaybookIssueTarget(ev, &req); err != nil {
		return ChannelPlaybookActionRequest{}, err
	}
	title, steps, checks, rollback := parseChannelPlaybookSections(trailing, ev)
	req.Options.Title = title
	req.Options.Steps = steps
	req.Options.Checks = checks
	req.Options.Rollback = rollback
	if strings.TrimSpace(req.Options.PlaybookID) == "" {
		req.Options.PlaybookID = autoChannelPlaybookID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, steps, checks, rollback)
		req.AutoPlaybookID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPlaybookNotifyMessageID(ev, req.Options.PlaybookID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPlaybookOptions(req.Options)
	if err := validateChannelPlaybookActionRequestOptions(req.Options); err != nil {
		return ChannelPlaybookActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.StepsSHA = shortDocumentHash(req.Options.Steps)
	req.StepsBytes = len(req.Options.Steps)
	req.StepsLines = lineCount(req.Options.Steps)
	req.ChecksSHA = shortDocumentHash(req.Options.Checks)
	req.ChecksBytes = len(req.Options.Checks)
	req.ChecksLines = lineCount(req.Options.Checks)
	req.RollbackSHA = shortDocumentHash(req.Options.Rollback)
	req.RollbackBytes = len(req.Options.Rollback)
	req.RollbackLines = lineCount(req.Options.Rollback)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelPlaybookNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelPlaybook(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPlaybookOptions) (ChannelPlaybookResult, error) {
	opts = normalizeChannelPlaybookOptions(opts)
	var err error
	opts, err = applyChannelPlaybookRoute(cfg, opts)
	if err != nil {
		return ChannelPlaybookResult{}, err
	}
	if err := validateChannelPlaybookOptions(opts); err != nil {
		return ChannelPlaybookResult{}, err
	}
	playbookIssue, created, duplicate, err := findOrCreateChannelPlaybookIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelPlaybookResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelPlaybookNotificationBody(opts, playbookIssue.Number, issueURL(opts.Repo, playbookIssue.Number)),
	})
	if err != nil {
		return ChannelPlaybookResult{}, fmt.Errorf("queue channel playbook notification: %w", err)
	}
	return ChannelPlaybookResult{
		PlaybookIssueNumber: playbookIssue.Number,
		PlaybookIssueURL:    issueURL(opts.Repo, playbookIssue.Number),
		PlaybookCreated:     created,
		PlaybookDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelPlaybookActionReport(ev Event, req ChannelPlaybookActionRequest, result ChannelPlaybookResult) string {
	status := "recorded"
	switch {
	case result.PlaybookDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.PlaybookDuplicate:
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
	b.WriteString("## GitClaw Channel Playbook Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_playbook_status: `%s`\n", status)
	fmt.Fprintf(&b, "- playbook_issue: `#%d`\n", result.PlaybookIssueNumber)
	fmt.Fprintf(&b, "- playbook_issue_url: `%s`\n", result.PlaybookIssueURL)
	fmt.Fprintf(&b, "- playbook_issue_created: `%t`\n", result.PlaybookCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.PlaybookDuplicate)
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
	fmt.Fprintf(&b, "- playbook_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.PlaybookID))
	fmt.Fprintf(&b, "- playbook_id_auto: `%t`\n", req.AutoPlaybookID)
	fmt.Fprintf(&b, "- playbook_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- playbook_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- playbook_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- playbook_steps_sha256_12: `%s`\n", req.StepsSHA)
	fmt.Fprintf(&b, "- playbook_steps_bytes: `%d`\n", req.StepsBytes)
	fmt.Fprintf(&b, "- playbook_steps_lines: `%d`\n", req.StepsLines)
	fmt.Fprintf(&b, "- playbook_checks_sha256_12: `%s`\n", req.ChecksSHA)
	fmt.Fprintf(&b, "- playbook_checks_bytes: `%d`\n", req.ChecksBytes)
	fmt.Fprintf(&b, "- playbook_checks_lines: `%d`\n", req.ChecksLines)
	fmt.Fprintf(&b, "- playbook_rollback_sha256_12: `%s`\n", req.RollbackSHA)
	fmt.Fprintf(&b, "- playbook_rollback_bytes: `%d`\n", req.RollbackBytes)
	fmt.Fprintf(&b, "- playbook_rollback_lines: `%d`\n", req.RollbackLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_playbook_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_playbook_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_playbook_steps_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_playbook_checks_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_playbook_rollback_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_playbook_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel playbook as a durable GitHub issue, then queued a provider-facing link back to the original thread. The playbook issue contains the human-readable title, steps, checks, and rollback guidance; this source receipt keeps provider IDs, playbook IDs, section text, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the playbook-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent playbook links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate playbook issues are suppressed by `playbook_id`; duplicate playbook-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the playbook issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelPlaybookIssueBody(opts ChannelPlaybookOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-playbook playbook_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.PlaybookID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel playbook.\n\n")
	fmt.Fprintf(&b, "- playbook_id: %s\n", opts.PlaybookID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- playbook_mode: github-issue-playbook\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Steps) != "" {
		b.WriteString("\n\n## Steps\n\n")
		b.WriteString(strings.TrimSpace(opts.Steps))
	}
	if strings.TrimSpace(opts.Checks) != "" {
		b.WriteString("\n\n## Checks\n\n")
		b.WriteString(strings.TrimSpace(opts.Checks))
	}
	if strings.TrimSpace(opts.Rollback) != "" {
		b.WriteString("\n\n## Rollback\n\n")
		b.WriteString(strings.TrimSpace(opts.Rollback))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel playbook.")
	return strings.TrimSpace(b.String())
}

func channelPlaybookActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPlaybookActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPlaybookIssueTarget(ev Event, req *ChannelPlaybookActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel playbook requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelPlaybookSections(trailing string, ev Event) (string, string, string, string) {
	lines := cleanChannelPlaybookTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel playbook from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", ""
	}
	playbook := channelPlaybookParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				playbook.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelPlaybookSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				playbook.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			playbook.Title = trimmed
			continue
		}
		if current == "" {
			current = "steps"
		}
		playbook.append(current, line)
	}
	return strings.TrimSpace(playbook.Title), strings.TrimSpace(playbook.Steps), strings.TrimSpace(playbook.Checks), strings.TrimSpace(playbook.Rollback)
}

type channelPlaybookParsedSections struct {
	Title    string
	Steps    string
	Checks   string
	Rollback string
}

func (sections *channelPlaybookParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelPlaybookParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "steps":
		sections.Steps = appendChannelPlaybookSectionLine(sections.Steps, value)
	case "checks":
		sections.Checks = appendChannelPlaybookSectionLine(sections.Checks, value)
	case "rollback":
		sections.Rollback = appendChannelPlaybookSectionLine(sections.Rollback, value)
	}
}

func appendChannelPlaybookSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelPlaybookSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelPlaybookHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "steps":
		return "steps", strings.TrimSpace(value), true
	case "checks":
		return "checks", strings.TrimSpace(value), true
	case "rollback":
		return "rollback", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelPlaybookHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "playbook", "runbook", "procedure", "recipe", "sop", "goal", "purpose", "intent", "when to use":
		return "title"
	case "steps", "procedure steps", "how", "how to", "checklist":
		return "steps"
	case "checks", "verify", "verification", "validation", "done when", "acceptance":
		return "checks"
	case "rollback", "backout", "revert", "undo", "escape hatch":
		return "rollback"
	default:
		return ""
	}
}

func cleanChannelPlaybookTrailingLines(trailing string) []string {
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

func normalizeChannelPlaybookOptions(opts ChannelPlaybookOptions) ChannelPlaybookOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.PlaybookID = cleanChannelPlaybookID(opts.PlaybookID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Steps = strings.TrimSpace(opts.Steps)
	opts.Checks = strings.TrimSpace(opts.Checks)
	opts.Rollback = strings.TrimSpace(opts.Rollback)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelPlaybookRoute(cfg Config, opts ChannelPlaybookOptions) (ChannelPlaybookOptions, error) {
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

func validateChannelPlaybookOptions(opts ChannelPlaybookOptions) error {
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
	if opts.PlaybookID == "" {
		return fmt.Errorf("missing playbook id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing playbook source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing playbook title")
	}
	return nil
}

func validateChannelPlaybookActionRequestOptions(opts ChannelPlaybookOptions) error {
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
	if opts.PlaybookID == "" {
		return fmt.Errorf("missing playbook id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing playbook source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing playbook title")
	}
	return nil
}

func findOrCreateChannelPlaybookIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPlaybookOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel playbook issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelPlaybookMatches(issue.Body, opts.PlaybookID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelPlaybookIssueTitle(opts), RenderChannelPlaybookIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel playbook issue: %w", err)
	}
	return issue, true, false, nil
}

func channelPlaybookIssueTitle(opts ChannelPlaybookOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.PlaybookID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel playbook: " + title
}

func channelPlaybookMatches(body, playbookID string) bool {
	return HasChannelPlaybookMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`playbook_id="%s"`, escapeMarkerValue(cleanChannelPlaybookID(playbookID))))
}

func cleanChannelPlaybookID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelPlaybookID(ev Event, channel, threadID, sourceMessageID, title, steps, checks, rollback string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, steps, checks, rollback}, "|")
	return fmt.Sprintf("playbook-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPlaybookNotifyMessageID(ev Event, playbookID string) string {
	seed := strings.Join([]string{eventID(ev), playbookID}, "|")
	return fmt.Sprintf("gitclaw-channel-playbook-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelPlaybookNotificationBody(opts ChannelPlaybookOptions, playbookIssueNumber int, playbookIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel playbook recorded.\n\n")
	if playbookIssueNumber > 0 {
		fmt.Fprintf(&b, "Playbook: #%d\n", playbookIssueNumber)
	}
	if playbookIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", playbookIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
