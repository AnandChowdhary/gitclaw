package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPactOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	PactID            string
	Title             string
	Participants      string
	Agreement         string
	Scope             string
	Revisit           string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelPactResult struct {
	PactIssueNumber int
	PactIssueURL    string
	PactCreated     bool
	PactDuplicate   bool
	Notification    ChannelSendResult
	RouteName       string
	RouteHash       string
	Channel         string
	ThreadHash      string
	MessageHash     string
	NotifyHash      string
}

type ChannelPactActionRequest struct {
	Options             ChannelPactOptions
	Command             string
	Subcommand          string
	AutoPactID          bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	ParticipantsSHA     string
	ParticipantsBytes   int
	ParticipantsLines   int
	AgreementSHA        string
	AgreementBytes      int
	AgreementLines      int
	ScopeSHA            string
	ScopeBytes          int
	ScopeLines          int
	RevisitSHA          string
	RevisitBytes        int
	RevisitLines        int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelPactActionRequest(ev Event, cfg Config) bool {
	return isChannelPactActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPactActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "pact", "agreement", "norm", "accord", "working-agreement", "team-agreement":
		return true
	default:
		return false
	}
}

func BuildChannelPactActionRequest(ev Event, cfg Config) (ChannelPactActionRequest, error) {
	fields, trailing, ok := channelPactActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPactActionRequest{}, fmt.Errorf("missing channel pact command")
	}
	req := ChannelPactActionRequest{
		Options: ChannelPactOptions{
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
				return ChannelPactActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelPactActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPactActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelPactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--pact-id", "--agreement-id", "--norm-id", "--accord-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPactActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PactID = cleanChannelPactID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPactActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPactActionRequest{}, fmt.Errorf("unknown channel pact argument %q", field)
			}
			if req.Options.PactID == "" {
				req.Options.PactID = cleanChannelPactID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelPactActionRequest{}, fmt.Errorf("unexpected channel pact argument %q", field)
		}
	}
	if err := applyChannelPactIssueTarget(ev, &req); err != nil {
		return ChannelPactActionRequest{}, err
	}
	title, participants, agreement, scope, revisit := parseChannelPactSections(trailing, ev)
	req.Options.Title = title
	req.Options.Participants = participants
	req.Options.Agreement = agreement
	req.Options.Scope = scope
	req.Options.Revisit = revisit
	if strings.TrimSpace(req.Options.PactID) == "" {
		req.Options.PactID = autoChannelPactID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, participants, agreement, scope, revisit)
		req.AutoPactID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPactNotifyMessageID(ev, req.Options.PactID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPactOptions(req.Options)
	if err := validateChannelPactActionRequestOptions(req.Options); err != nil {
		return ChannelPactActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.ParticipantsSHA = shortDocumentHash(req.Options.Participants)
	req.ParticipantsBytes = len(req.Options.Participants)
	req.ParticipantsLines = lineCount(req.Options.Participants)
	req.AgreementSHA = shortDocumentHash(req.Options.Agreement)
	req.AgreementBytes = len(req.Options.Agreement)
	req.AgreementLines = lineCount(req.Options.Agreement)
	req.ScopeSHA = shortDocumentHash(req.Options.Scope)
	req.ScopeBytes = len(req.Options.Scope)
	req.ScopeLines = lineCount(req.Options.Scope)
	req.RevisitSHA = shortDocumentHash(req.Options.Revisit)
	req.RevisitBytes = len(req.Options.Revisit)
	req.RevisitLines = lineCount(req.Options.Revisit)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelPactNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelPact(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPactOptions) (ChannelPactResult, error) {
	opts = normalizeChannelPactOptions(opts)
	var err error
	opts, err = applyChannelPactRoute(cfg, opts)
	if err != nil {
		return ChannelPactResult{}, err
	}
	if err := validateChannelPactOptions(opts); err != nil {
		return ChannelPactResult{}, err
	}
	pactIssue, created, duplicate, err := findOrCreateChannelPactIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelPactResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelPactNotificationBody(opts, pactIssue.Number, issueURL(opts.Repo, pactIssue.Number)),
	})
	if err != nil {
		return ChannelPactResult{}, fmt.Errorf("queue channel pact notification: %w", err)
	}
	return ChannelPactResult{
		PactIssueNumber: pactIssue.Number,
		PactIssueURL:    issueURL(opts.Repo, pactIssue.Number),
		PactCreated:     created,
		PactDuplicate:   duplicate,
		Notification:    notification,
		RouteName:       opts.Route,
		RouteHash:       channelRouteHash(opts.Route),
		Channel:         opts.Channel,
		ThreadHash:      shortDocumentHash(opts.ThreadID),
		MessageHash:     shortDocumentHash(opts.SourceMessageID),
		NotifyHash:      shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelPactActionReport(ev Event, req ChannelPactActionRequest, result ChannelPactResult) string {
	status := "recorded"
	switch {
	case result.PactDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.PactDuplicate:
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
	b.WriteString("## GitClaw Channel Pact Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_pact_status: `%s`\n", status)
	fmt.Fprintf(&b, "- pact_issue: `#%d`\n", result.PactIssueNumber)
	fmt.Fprintf(&b, "- pact_issue_url: `%s`\n", result.PactIssueURL)
	fmt.Fprintf(&b, "- pact_issue_created: `%t`\n", result.PactCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.PactDuplicate)
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
	fmt.Fprintf(&b, "- pact_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.PactID))
	fmt.Fprintf(&b, "- pact_id_auto: `%t`\n", req.AutoPactID)
	fmt.Fprintf(&b, "- pact_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- pact_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- pact_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- pact_participants_sha256_12: `%s`\n", req.ParticipantsSHA)
	fmt.Fprintf(&b, "- pact_participants_bytes: `%d`\n", req.ParticipantsBytes)
	fmt.Fprintf(&b, "- pact_participants_lines: `%d`\n", req.ParticipantsLines)
	fmt.Fprintf(&b, "- pact_agreement_sha256_12: `%s`\n", req.AgreementSHA)
	fmt.Fprintf(&b, "- pact_agreement_bytes: `%d`\n", req.AgreementBytes)
	fmt.Fprintf(&b, "- pact_agreement_lines: `%d`\n", req.AgreementLines)
	fmt.Fprintf(&b, "- pact_scope_sha256_12: `%s`\n", req.ScopeSHA)
	fmt.Fprintf(&b, "- pact_scope_bytes: `%d`\n", req.ScopeBytes)
	fmt.Fprintf(&b, "- pact_scope_lines: `%d`\n", req.ScopeLines)
	fmt.Fprintf(&b, "- pact_revisit_sha256_12: `%s`\n", req.RevisitSHA)
	fmt.Fprintf(&b, "- pact_revisit_bytes: `%d`\n", req.RevisitBytes)
	fmt.Fprintf(&b, "- pact_revisit_lines: `%d`\n", req.RevisitLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_pact_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_pact_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_pact_participants_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_pact_agreement_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_pact_scope_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_pact_revisit_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- standing_order_created: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_pact_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel pact as a durable GitHub issue, then queued a provider-facing link back to the original thread. The pact issue contains the human-readable title, participants, agreement, scope, and revisit notes; this source receipt keeps provider IDs, pact IDs, section text, and channel message bodies out of band. No schedule, reminder, standing order, SOUL write, memory write, policy mutation, workflow edit, or repository mutation is created by this action.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the pact-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent pact links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate pact issues are suppressed by `pact_id`; duplicate pact-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the pact issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelPactIssueBody(opts ChannelPactOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-pact pact_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.PactID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel pact.\n\n")
	fmt.Fprintf(&b, "- pact_id: %s\n", opts.PactID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- pact_mode: github-issue-pact\n")
	fmt.Fprintf(&b, "- scheduled_workflow_created: false\n")
	fmt.Fprintf(&b, "- reminder_created: false\n")
	fmt.Fprintf(&b, "- standing_order_created: false\n")
	fmt.Fprintf(&b, "- soul_write_performed: false\n")
	fmt.Fprintf(&b, "- memory_write_performed: false\n")
	fmt.Fprintf(&b, "- policy_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Participants) != "" {
		b.WriteString("\n\n## Participants\n\n")
		b.WriteString(strings.TrimSpace(opts.Participants))
	}
	if strings.TrimSpace(opts.Agreement) != "" {
		b.WriteString("\n\n## Agreement\n\n")
		b.WriteString(strings.TrimSpace(opts.Agreement))
	}
	if strings.TrimSpace(opts.Scope) != "" {
		b.WriteString("\n\n## Scope\n\n")
		b.WriteString(strings.TrimSpace(opts.Scope))
	}
	if strings.TrimSpace(opts.Revisit) != "" {
		b.WriteString("\n\n## Revisit\n\n")
		b.WriteString(strings.TrimSpace(opts.Revisit))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for deciding whether the channel pact should become a standing order, soul proposal, memory proposal, policy change, skill, or be closed.")
	return strings.TrimSpace(b.String())
}

func channelPactActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPactActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPactIssueTarget(ev Event, req *ChannelPactActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel pact requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelPactSections(trailing string, ev Event) (string, string, string, string, string) {
	lines := cleanChannelPactTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel pact from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", "", ""
	}
	pact := channelPactParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				pact.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelPactSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				pact.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			pact.Title = trimmed
			continue
		}
		if current == "" {
			current = "agreement"
		}
		pact.append(current, line)
	}
	return strings.TrimSpace(pact.Title), strings.TrimSpace(pact.Participants), strings.TrimSpace(pact.Agreement), strings.TrimSpace(pact.Scope), strings.TrimSpace(pact.Revisit)
}

type channelPactParsedSections struct {
	Title        string
	Participants string
	Agreement    string
	Scope        string
	Revisit      string
}

func (sections *channelPactParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelPactParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "participants":
		sections.Participants = appendChannelPactSectionLine(sections.Participants, value)
	case "agreement":
		sections.Agreement = appendChannelPactSectionLine(sections.Agreement, value)
	case "scope":
		sections.Scope = appendChannelPactSectionLine(sections.Scope, value)
	case "revisit":
		sections.Revisit = appendChannelPactSectionLine(sections.Revisit, value)
	}
}

func appendChannelPactSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelPactSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelPactHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "participants":
		return "participants", strings.TrimSpace(value), true
	case "agreement":
		return "agreement", strings.TrimSpace(value), true
	case "scope":
		return "scope", strings.TrimSpace(value), true
	case "revisit":
		return "revisit", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelPactHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "pact", "agreement title", "accord", "name":
		return "title"
	case "participants", "people", "parties", "who", "owners":
		return "participants"
	case "agreement", "promise", "norm", "terms", "what":
		return "agreement"
	case "scope", "applies to", "boundary", "context", "where":
		return "scope"
	case "revisit", "review", "revisit when", "expires", "sunset", "check":
		return "revisit"
	default:
		return ""
	}
}

func cleanChannelPactTrailingLines(trailing string) []string {
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

func normalizeChannelPactOptions(opts ChannelPactOptions) ChannelPactOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.PactID = cleanChannelPactID(opts.PactID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Participants = strings.TrimSpace(opts.Participants)
	opts.Agreement = strings.TrimSpace(opts.Agreement)
	opts.Scope = strings.TrimSpace(opts.Scope)
	opts.Revisit = strings.TrimSpace(opts.Revisit)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelPactRoute(cfg Config, opts ChannelPactOptions) (ChannelPactOptions, error) {
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

func validateChannelPactOptions(opts ChannelPactOptions) error {
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
	if opts.PactID == "" {
		return fmt.Errorf("missing pact id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing pact source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing pact title")
	}
	return nil
}

func validateChannelPactActionRequestOptions(opts ChannelPactOptions) error {
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
	if opts.PactID == "" {
		return fmt.Errorf("missing pact id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing pact source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing pact title")
	}
	return nil
}

func findOrCreateChannelPactIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPactOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel pact issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelPactMatches(issue.Body, opts.PactID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelPactIssueTitle(opts), RenderChannelPactIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel pact issue: %w", err)
	}
	return issue, true, false, nil
}

func channelPactIssueTitle(opts ChannelPactOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.PactID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel pact: " + title
}

func channelPactMatches(body, pactID string) bool {
	return HasChannelPactMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`pact_id="%s"`, escapeMarkerValue(cleanChannelPactID(pactID))))
}

func cleanChannelPactID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelPactID(ev Event, channel, threadID, sourceMessageID, title, participants, agreement, scope, revisit string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, participants, agreement, scope, revisit}, "|")
	return fmt.Sprintf("pact-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPactNotifyMessageID(ev Event, pactID string) string {
	seed := strings.Join([]string{eventID(ev), pactID}, "|")
	return fmt.Sprintf("gitclaw-channel-pact-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelPactNotificationBody(opts ChannelPactOptions, pactIssueNumber int, pactIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel pact recorded.\n\n")
	if pactIssueNumber > 0 {
		fmt.Fprintf(&b, "Pact: #%d\n", pactIssueNumber)
	}
	if pactIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", pactIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Participants) != "" {
		fmt.Fprintf(&b, "Participants: %s\n", strings.TrimSpace(opts.Participants))
	}
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
