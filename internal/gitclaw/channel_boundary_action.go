package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBoundaryOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	BoundaryID        string
	Title             string
	Boundary          string
	Scope             string
	Reason            string
	Review            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBoundaryResult struct {
	BoundaryIssueNumber int
	BoundaryIssueURL    string
	BoundaryCreated     bool
	BoundaryDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	SourceMessageHash   string
	NotifyMessageHash   string
}

type ChannelBoundaryActionRequest struct {
	Options             ChannelBoundaryOptions
	Command             string
	Subcommand          string
	AutoBoundaryID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TitleSHA            string
	TitleBytes          int
	TitleLines          int
	BoundarySHA         string
	BoundaryBytes       int
	BoundaryLines       int
	ScopeSHA            string
	ScopeBytes          int
	ScopeLines          int
	ReasonSHA           string
	ReasonBytes         int
	ReasonLines         int
	ReviewSHA           string
	ReviewBytes         int
	ReviewLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelBoundaryActionRequest(ev Event, cfg Config) bool {
	return isChannelBoundaryActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBoundaryActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "boundary", "guardrail", "constraint", "consent", "ground-rule", "house-rule":
		return true
	default:
		return false
	}
}

func BuildChannelBoundaryActionRequest(ev Event, cfg Config) (ChannelBoundaryActionRequest, error) {
	fields, trailing, ok := channelBoundaryActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBoundaryActionRequest{}, fmt.Errorf("missing channel boundary command")
	}
	req := ChannelBoundaryActionRequest{
		Options: ChannelBoundaryOptions{
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
				return ChannelBoundaryActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--boundary-id", "--guardrail-id", "--constraint-id", "--consent-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.BoundaryID = cleanChannelBoundaryID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBoundaryActionRequest{}, fmt.Errorf("unknown channel boundary argument %q", field)
			}
			if req.Options.BoundaryID == "" {
				req.Options.BoundaryID = cleanChannelBoundaryID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelBoundaryActionRequest{}, fmt.Errorf("unexpected channel boundary argument %q", field)
		}
	}
	if err := applyChannelBoundaryIssueTarget(ev, &req); err != nil {
		return ChannelBoundaryActionRequest{}, err
	}
	title, boundary, scope, reason, review := parseChannelBoundarySections(trailing, ev)
	req.Options.Title = title
	req.Options.Boundary = boundary
	req.Options.Scope = scope
	req.Options.Reason = reason
	req.Options.Review = review
	if strings.TrimSpace(req.Options.BoundaryID) == "" {
		req.Options.BoundaryID = autoChannelBoundaryID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, boundary, scope, reason, review)
		req.AutoBoundaryID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBoundaryNotifyMessageID(ev, req.Options.BoundaryID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBoundaryOptions(req.Options)
	if err := validateChannelBoundaryActionRequestOptions(req.Options); err != nil {
		return ChannelBoundaryActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.BoundarySHA = shortDocumentHash(req.Options.Boundary)
	req.BoundaryBytes = len(req.Options.Boundary)
	req.BoundaryLines = lineCount(req.Options.Boundary)
	req.ScopeSHA = shortDocumentHash(req.Options.Scope)
	req.ScopeBytes = len(req.Options.Scope)
	req.ScopeLines = lineCount(req.Options.Scope)
	req.ReasonSHA = shortDocumentHash(req.Options.Reason)
	req.ReasonBytes = len(req.Options.Reason)
	req.ReasonLines = lineCount(req.Options.Reason)
	req.ReviewSHA = shortDocumentHash(req.Options.Review)
	req.ReviewBytes = len(req.Options.Review)
	req.ReviewLines = lineCount(req.Options.Review)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelBoundaryNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelBoundary(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBoundaryOptions) (ChannelBoundaryResult, error) {
	opts = normalizeChannelBoundaryOptions(opts)
	var err error
	opts, err = applyChannelBoundaryRoute(cfg, opts)
	if err != nil {
		return ChannelBoundaryResult{}, err
	}
	if err := validateChannelBoundaryOptions(opts); err != nil {
		return ChannelBoundaryResult{}, err
	}
	boundaryIssue, created, duplicate, err := findOrCreateChannelBoundaryIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelBoundaryResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelBoundaryNotificationBody(opts, boundaryIssue.Number, issueURL(opts.Repo, boundaryIssue.Number)),
	})
	if err != nil {
		return ChannelBoundaryResult{}, fmt.Errorf("queue channel boundary notification: %w", err)
	}
	return ChannelBoundaryResult{
		BoundaryIssueNumber: boundaryIssue.Number,
		BoundaryIssueURL:    issueURL(opts.Repo, boundaryIssue.Number),
		BoundaryCreated:     created,
		BoundaryDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		SourceMessageHash:   shortDocumentHash(opts.SourceMessageID),
		NotifyMessageHash:   shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelBoundaryActionReport(ev Event, req ChannelBoundaryActionRequest, result ChannelBoundaryResult) string {
	status := "recorded"
	switch {
	case result.BoundaryDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.BoundaryDuplicate:
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
	messageHash := result.SourceMessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	notifyHash := result.NotifyMessageHash
	if notifyHash == "" {
		notifyHash = req.NotifyMessageHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Boundary Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_boundary_status: `%s`\n", status)
	fmt.Fprintf(&b, "- boundary_issue: `#%d`\n", result.BoundaryIssueNumber)
	fmt.Fprintf(&b, "- boundary_issue_url: `%s`\n", result.BoundaryIssueURL)
	fmt.Fprintf(&b, "- boundary_issue_created: `%t`\n", result.BoundaryCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.BoundaryDuplicate)
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
	fmt.Fprintf(&b, "- boundary_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.BoundaryID))
	fmt.Fprintf(&b, "- boundary_id_auto: `%t`\n", req.AutoBoundaryID)
	fmt.Fprintf(&b, "- boundary_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- boundary_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- boundary_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- boundary_body_sha256_12: `%s`\n", req.BoundarySHA)
	fmt.Fprintf(&b, "- boundary_body_bytes: `%d`\n", req.BoundaryBytes)
	fmt.Fprintf(&b, "- boundary_body_lines: `%d`\n", req.BoundaryLines)
	fmt.Fprintf(&b, "- boundary_scope_sha256_12: `%s`\n", req.ScopeSHA)
	fmt.Fprintf(&b, "- boundary_scope_bytes: `%d`\n", req.ScopeBytes)
	fmt.Fprintf(&b, "- boundary_scope_lines: `%d`\n", req.ScopeLines)
	fmt.Fprintf(&b, "- boundary_reason_sha256_12: `%s`\n", req.ReasonSHA)
	fmt.Fprintf(&b, "- boundary_reason_bytes: `%d`\n", req.ReasonBytes)
	fmt.Fprintf(&b, "- boundary_reason_lines: `%d`\n", req.ReasonLines)
	fmt.Fprintf(&b, "- boundary_review_sha256_12: `%s`\n", req.ReviewSHA)
	fmt.Fprintf(&b, "- boundary_review_bytes: `%d`\n", req.ReviewBytes)
	fmt.Fprintf(&b, "- boundary_review_lines: `%d`\n", req.ReviewLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_boundary_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_boundary_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_boundary_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_boundary_scope_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_boundary_reason_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_boundary_review_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- scheduled_workflow_created: `%t`\n", false)
	fmt.Fprintf(&b, "- reminder_created: `%t`\n", false)
	fmt.Fprintf(&b, "- allowlist_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- pairing_code_issued: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- policy_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_setting_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- enforcement_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_boundary_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded mirrored channel boundary as a durable GitHub issue, then queued a provider-facing link back to the original thread. The boundary issue contains the human-readable title, boundary, scope, reason, and review timing; this source receipt keeps provider IDs, boundary IDs, section text, and channel message bodies out of band. No model call, schedule, reminder, allowlist mutation, pairing code, soul write, memory write, policy mutation, skill install, workflow mutation, provider-setting mutation, provider delivery, enforcement, or repository mutation is created by this action.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the boundary-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent boundary links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate boundary issues are suppressed by `boundary_id`; duplicate boundary-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the boundary issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelBoundaryIssueBody(opts ChannelBoundaryOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-boundary boundary_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.BoundaryID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel boundary.\n\n")
	fmt.Fprintf(&b, "- boundary_id: %s\n", opts.BoundaryID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- boundary_mode: github-issue-boundary\n")
	fmt.Fprintf(&b, "- model_call_performed: false\n")
	fmt.Fprintf(&b, "- scheduled_workflow_created: false\n")
	fmt.Fprintf(&b, "- reminder_created: false\n")
	fmt.Fprintf(&b, "- allowlist_mutation_performed: false\n")
	fmt.Fprintf(&b, "- pairing_code_issued: false\n")
	fmt.Fprintf(&b, "- soul_write_performed: false\n")
	fmt.Fprintf(&b, "- memory_write_performed: false\n")
	fmt.Fprintf(&b, "- policy_mutation_performed: false\n")
	fmt.Fprintf(&b, "- skill_install_performed: false\n")
	fmt.Fprintf(&b, "- workflow_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_setting_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- enforcement_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Boundary) != "" {
		b.WriteString("\n\n## Boundary\n\n")
		b.WriteString(strings.TrimSpace(opts.Boundary))
	}
	if strings.TrimSpace(opts.Scope) != "" {
		b.WriteString("\n\n## Scope\n\n")
		b.WriteString(strings.TrimSpace(opts.Scope))
	}
	if strings.TrimSpace(opts.Reason) != "" {
		b.WriteString("\n\n## Reason\n\n")
		b.WriteString(strings.TrimSpace(opts.Reason))
	}
	if strings.TrimSpace(opts.Review) != "" {
		b.WriteString("\n\n## Review\n\n")
		b.WriteString(strings.TrimSpace(opts.Review))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for a channel boundary. Close it when the boundary is stale, superseded, or promoted through normal reviewed GitHub issues.")
	return strings.TrimSpace(b.String())
}

func channelBoundaryActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBoundaryActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBoundaryIssueTarget(ev Event, req *ChannelBoundaryActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel boundary requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelBoundarySections(trailing string, ev Event) (string, string, string, string, string) {
	lines := cleanChannelBoundaryTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel boundary from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", "", ""
	}
	boundary := channelBoundaryParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				boundary.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelBoundarySectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				boundary.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			boundary.Title = trimmed
			continue
		}
		if current == "" {
			current = "boundary"
		}
		boundary.append(current, line)
	}
	return strings.TrimSpace(boundary.Title), strings.TrimSpace(boundary.Boundary), strings.TrimSpace(boundary.Scope), strings.TrimSpace(boundary.Reason), strings.TrimSpace(boundary.Review)
}

type channelBoundaryParsedSections struct {
	Title    string
	Boundary string
	Scope    string
	Reason   string
	Review   string
}

func (sections *channelBoundaryParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelBoundaryParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "boundary":
		sections.Boundary = appendChannelBoundarySectionLine(sections.Boundary, value)
	case "scope":
		sections.Scope = appendChannelBoundarySectionLine(sections.Scope, value)
	case "reason":
		sections.Reason = appendChannelBoundarySectionLine(sections.Reason, value)
	case "review":
		sections.Review = appendChannelBoundarySectionLine(sections.Review, value)
	}
}

func appendChannelBoundarySectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelBoundarySectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelBoundaryHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "boundary":
		return "boundary", strings.TrimSpace(value), true
	case "scope":
		return "scope", strings.TrimSpace(value), true
	case "reason":
		return "reason", strings.TrimSpace(value), true
	case "review":
		return "review", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelBoundaryHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "boundary title", "guardrail title", "heading", "caption", "name":
		return "title"
	case "boundary", "guardrail", "constraint", "consent", "rule", "ground rule", "house rule", "what":
		return "boundary"
	case "scope", "applies to", "where", "surface", "context":
		return "scope"
	case "reason", "why", "rationale", "because":
		return "reason"
	case "review", "revisit", "expires", "expiry", "check":
		return "review"
	default:
		return ""
	}
}

func cleanChannelBoundaryTrailingLines(trailing string) []string {
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

func normalizeChannelBoundaryOptions(opts ChannelBoundaryOptions) ChannelBoundaryOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.BoundaryID = cleanChannelBoundaryID(opts.BoundaryID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.Boundary = strings.TrimSpace(opts.Boundary)
	opts.Scope = strings.TrimSpace(opts.Scope)
	opts.Reason = strings.TrimSpace(opts.Reason)
	opts.Review = strings.TrimSpace(opts.Review)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelBoundaryRoute(cfg Config, opts ChannelBoundaryOptions) (ChannelBoundaryOptions, error) {
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

func validateChannelBoundaryOptions(opts ChannelBoundaryOptions) error {
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
	if opts.BoundaryID == "" {
		return fmt.Errorf("missing boundary id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing boundary source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing boundary title")
	}
	return nil
}

func validateChannelBoundaryActionRequestOptions(opts ChannelBoundaryOptions) error {
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
	if opts.BoundaryID == "" {
		return fmt.Errorf("missing boundary id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing boundary source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing boundary title")
	}
	return nil
}

func findOrCreateChannelBoundaryIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBoundaryOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel boundary issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelBoundaryMatches(issue.Body, opts.BoundaryID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelBoundaryIssueTitle(opts), RenderChannelBoundaryIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel boundary issue: %w", err)
	}
	return issue, true, false, nil
}

func channelBoundaryIssueTitle(opts ChannelBoundaryOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.BoundaryID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel boundary: " + title
}

func channelBoundaryMatches(body, boundaryID string) bool {
	return HasChannelBoundaryMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`boundary_id="%s"`, escapeMarkerValue(cleanChannelBoundaryID(boundaryID))))
}

func cleanChannelBoundaryID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBoundaryID(ev Event, channel, threadID, sourceMessageID, title, boundary, scope, reason, review string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, boundary, scope, reason, review}, "|")
	return fmt.Sprintf("boundary-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBoundaryNotifyMessageID(ev Event, boundaryID string) string {
	seed := strings.Join([]string{eventID(ev), boundaryID}, "|")
	return fmt.Sprintf("gitclaw-channel-boundary-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelBoundaryNotificationBody(opts ChannelBoundaryOptions, boundaryIssueNumber int, boundaryIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel boundary recorded.\n\n")
	if boundaryIssueNumber > 0 {
		fmt.Fprintf(&b, "Boundary: #%d\n", boundaryIssueNumber)
	}
	if boundaryIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", boundaryIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.Review) != "" {
		fmt.Fprintf(&b, "Review: %s\n", strings.TrimSpace(opts.Review))
	}
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
