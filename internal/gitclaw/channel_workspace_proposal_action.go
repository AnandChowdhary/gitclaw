package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelWorkspaceProposalOptions struct {
	Repo                string
	Route               string
	Channel             string
	ThreadID            string
	SourceMessageID     string
	NotifyMessageID     string
	WorkspaceProposalID string
	Title               string
	TargetPath          string
	Proposal            string
	Rationale           string
	Author              string
	SourceIssueNumber   int
	SourceCommentID     int64
}

type ChannelWorkspaceProposalResult struct {
	WorkspaceProposalIssueNumber int
	WorkspaceProposalIssueURL    string
	WorkspaceProposalCreated     bool
	WorkspaceProposalDuplicate   bool
	Notification                 ChannelSendResult
	RouteName                    string
	RouteHash                    string
	Channel                      string
	ThreadHash                   string
	MessageHash                  string
	NotifyHash                   string
}

type ChannelWorkspaceProposalActionRequest struct {
	Options                 ChannelWorkspaceProposalOptions
	Command                 string
	Subcommand              string
	AutoWorkspaceProposalID bool
	AutoNotifyMessageID     bool
	TargetFromIssue         bool
	TitleSHA                string
	TitleBytes              int
	TitleLines              int
	TargetPathSHA           string
	TargetPathBytes         int
	TargetPathLines         int
	ProposalSHA             string
	ProposalBytes           int
	ProposalLines           int
	RationaleSHA            string
	RationaleBytes          int
	RationaleLines          int
	RequestedRouteHash      string
	RequestedThreadHash     string
	RequestedMsgHash        string
	NotifyMessageHash       string
	NotificationBodySHA     string
}

func IsChannelWorkspaceProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelWorkspaceProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelWorkspaceProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-workspace", "workspace-proposal", "workspace-context", "context-proposal", "propose-context", "workspace-note", "workspace":
		return true
	default:
		return false
	}
}

func BuildChannelWorkspaceProposalActionRequest(ev Event, cfg Config) (ChannelWorkspaceProposalActionRequest, error) {
	fields, trailing, ok := channelWorkspaceProposalActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("missing channel workspace proposal command")
	}
	req := ChannelWorkspaceProposalActionRequest{
		Options: ChannelWorkspaceProposalOptions{
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
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--workspace-id", "--workspace-proposal-id", "--proposal-id", "--context-id", "--id":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.WorkspaceProposalID = cleanChannelWorkspaceProposalID(fields[i+1])
			i++
		case "--target", "--target-path", "--workspace-path", "--path":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetPath = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("unknown channel workspace proposal argument %q", field)
			}
			if req.Options.WorkspaceProposalID == "" {
				req.Options.WorkspaceProposalID = cleanChannelWorkspaceProposalID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelWorkspaceProposalActionRequest{}, fmt.Errorf("unexpected channel workspace proposal argument %q", field)
		}
	}
	if err := applyChannelWorkspaceProposalIssueTarget(ev, &req); err != nil {
		return ChannelWorkspaceProposalActionRequest{}, err
	}
	title, targetPath, proposal, rationale := parseChannelWorkspaceProposalSections(trailing, ev)
	req.Options.Title = title
	if strings.TrimSpace(req.Options.TargetPath) == "" {
		req.Options.TargetPath = targetPath
	}
	req.Options.Proposal = proposal
	req.Options.Rationale = rationale
	if strings.TrimSpace(req.Options.WorkspaceProposalID) == "" {
		req.Options.WorkspaceProposalID = autoChannelWorkspaceProposalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, title, req.Options.TargetPath, proposal, rationale)
		req.AutoWorkspaceProposalID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelWorkspaceProposalNotifyMessageID(ev, req.Options.WorkspaceProposalID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelWorkspaceProposalOptions(req.Options)
	if err := validateChannelWorkspaceProposalActionRequestOptions(req.Options); err != nil {
		return ChannelWorkspaceProposalActionRequest{}, err
	}
	req.TitleSHA = shortDocumentHash(req.Options.Title)
	req.TitleBytes = len(req.Options.Title)
	req.TitleLines = lineCount(req.Options.Title)
	req.TargetPathSHA = shortDocumentHash(req.Options.TargetPath)
	req.TargetPathBytes = len(req.Options.TargetPath)
	req.TargetPathLines = lineCount(req.Options.TargetPath)
	req.ProposalSHA = shortDocumentHash(req.Options.Proposal)
	req.ProposalBytes = len(req.Options.Proposal)
	req.ProposalLines = lineCount(req.Options.Proposal)
	req.RationaleSHA = shortDocumentHash(req.Options.Rationale)
	req.RationaleBytes = len(req.Options.Rationale)
	req.RationaleLines = lineCount(req.Options.Rationale)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelWorkspaceProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelWorkspaceProposal(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelWorkspaceProposalOptions) (ChannelWorkspaceProposalResult, error) {
	opts = normalizeChannelWorkspaceProposalOptions(opts)
	var err error
	opts, err = applyChannelWorkspaceProposalRoute(cfg, opts)
	if err != nil {
		return ChannelWorkspaceProposalResult{}, err
	}
	if err := validateChannelWorkspaceProposalOptions(opts); err != nil {
		return ChannelWorkspaceProposalResult{}, err
	}
	workspaceProposalIssue, created, duplicate, err := findOrCreateChannelWorkspaceProposalIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelWorkspaceProposalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelWorkspaceProposalNotificationBody(opts, workspaceProposalIssue.Number, issueURL(opts.Repo, workspaceProposalIssue.Number)),
	})
	if err != nil {
		return ChannelWorkspaceProposalResult{}, fmt.Errorf("queue channel workspace proposal notification: %w", err)
	}
	return ChannelWorkspaceProposalResult{
		WorkspaceProposalIssueNumber: workspaceProposalIssue.Number,
		WorkspaceProposalIssueURL:    issueURL(opts.Repo, workspaceProposalIssue.Number),
		WorkspaceProposalCreated:     created,
		WorkspaceProposalDuplicate:   duplicate,
		Notification:                 notification,
		RouteName:                    opts.Route,
		RouteHash:                    channelRouteHash(opts.Route),
		Channel:                      opts.Channel,
		ThreadHash:                   shortDocumentHash(opts.ThreadID),
		MessageHash:                  shortDocumentHash(opts.SourceMessageID),
		NotifyHash:                   shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelWorkspaceProposalActionReport(ev Event, req ChannelWorkspaceProposalActionRequest, result ChannelWorkspaceProposalResult) string {
	status := "recorded"
	switch {
	case result.WorkspaceProposalDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.WorkspaceProposalDuplicate:
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
	b.WriteString("## GitClaw Channel Workspace Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_workspace_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- workspace_proposal_issue: `#%d`\n", result.WorkspaceProposalIssueNumber)
	fmt.Fprintf(&b, "- workspace_proposal_issue_url: `%s`\n", result.WorkspaceProposalIssueURL)
	fmt.Fprintf(&b, "- workspace_proposal_issue_created: `%t`\n", result.WorkspaceProposalCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.WorkspaceProposalDuplicate)
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
	fmt.Fprintf(&b, "- workspace_proposal_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.WorkspaceProposalID))
	fmt.Fprintf(&b, "- workspace_proposal_id_auto: `%t`\n", req.AutoWorkspaceProposalID)
	fmt.Fprintf(&b, "- workspace_proposal_title_sha256_12: `%s`\n", req.TitleSHA)
	fmt.Fprintf(&b, "- workspace_proposal_title_bytes: `%d`\n", req.TitleBytes)
	fmt.Fprintf(&b, "- workspace_proposal_title_lines: `%d`\n", req.TitleLines)
	fmt.Fprintf(&b, "- workspace_proposal_target_path_sha256_12: `%s`\n", req.TargetPathSHA)
	fmt.Fprintf(&b, "- workspace_proposal_target_path_bytes: `%d`\n", req.TargetPathBytes)
	fmt.Fprintf(&b, "- workspace_proposal_target_path_lines: `%d`\n", req.TargetPathLines)
	fmt.Fprintf(&b, "- workspace_proposal_proposal_sha256_12: `%s`\n", req.ProposalSHA)
	fmt.Fprintf(&b, "- workspace_proposal_proposal_bytes: `%d`\n", req.ProposalBytes)
	fmt.Fprintf(&b, "- workspace_proposal_proposal_lines: `%d`\n", req.ProposalLines)
	fmt.Fprintf(&b, "- workspace_proposal_rationale_sha256_12: `%s`\n", req.RationaleSHA)
	fmt.Fprintf(&b, "- workspace_proposal_rationale_bytes: `%d`\n", req.RationaleBytes)
	fmt.Fprintf(&b, "- workspace_proposal_rationale_lines: `%d`\n", req.RationaleLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- workspace_proposal_mode: `%s`\n", "github-issue-workspace-proposal")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- workspace_file_written: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workspace_proposal_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workspace_proposal_title_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workspace_proposal_target_path_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workspace_proposal_proposal_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workspace_proposal_rationale_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_workspace_proposal_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel workspace proposal as a durable GitHub issue, then queued a provider-facing link back to the original thread. The workspace proposal issue contains the human-readable title, target path, proposal, and rationale; this source receipt keeps provider IDs, workspace proposal IDs, section text, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the workspace-proposal link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent workspace-proposal links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate workspace proposal issues are suppressed by `workspace_proposal_id`; duplicate workspace-proposal link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the workspace proposal issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelWorkspaceProposalIssueBody(opts ChannelWorkspaceProposalOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-workspace-proposal workspace_proposal_id=\"%s\" channel=\"%s\" target_path_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.WorkspaceProposalID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.TargetPath), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel workspace proposal.\n\n")
	fmt.Fprintf(&b, "- workspace_proposal_id: %s\n", opts.WorkspaceProposalID)
	fmt.Fprintf(&b, "- target_path: %s\n", opts.TargetPath)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- workspace_proposal_mode: github-issue-workspace-proposal\n")
	fmt.Fprintf(&b, "- review_pr_required: true\n")
	fmt.Fprintf(&b, "- workspace_file_written: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Title\n\n")
	b.WriteString(strings.TrimSpace(opts.Title))
	if strings.TrimSpace(opts.TargetPath) != "" {
		b.WriteString("\n\n## Target Path\n\n")
		b.WriteString(strings.TrimSpace(opts.TargetPath))
	}
	if strings.TrimSpace(opts.Proposal) != "" {
		b.WriteString("\n\n## Proposal\n\n")
		b.WriteString(strings.TrimSpace(opts.Proposal))
	}
	if strings.TrimSpace(opts.Rationale) != "" {
		b.WriteString("\n\n## Rationale\n\n")
		b.WriteString(strings.TrimSpace(opts.Rationale))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the channel workspace proposal. Actual workspace file edits require a normal reviewed GitHub follow-up.")
	return strings.TrimSpace(b.String())
}

func channelWorkspaceProposalActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelWorkspaceProposalActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelWorkspaceProposalIssueTarget(ev Event, req *ChannelWorkspaceProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel workspace proposal requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelWorkspaceProposalSections(trailing string, ev Event) (string, string, string, string) {
	lines := cleanChannelWorkspaceProposalTrailingLines(trailing)
	defaultTitle := fmt.Sprintf("Channel workspace proposal from issue #%d", ev.Issue.Number)
	if len(lines) == 0 {
		return defaultTitle, "", "", ""
	}
	workspaceProposal := channelWorkspaceProposalParsedSections{Title: defaultTitle}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				workspaceProposal.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelWorkspaceProposalSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				workspaceProposal.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			workspaceProposal.Title = trimmed
			continue
		}
		if current == "" {
			current = "proposal"
		}
		workspaceProposal.append(current, line)
	}
	return strings.TrimSpace(workspaceProposal.Title), strings.TrimSpace(workspaceProposal.TargetPath), strings.TrimSpace(workspaceProposal.Proposal), strings.TrimSpace(workspaceProposal.Rationale)
}

type channelWorkspaceProposalParsedSections struct {
	Title      string
	TargetPath string
	Proposal   string
	Rationale  string
}

func (sections *channelWorkspaceProposalParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	if section == "title" {
		sections.Title = strings.TrimSpace(value)
		return
	}
	sections.append(section, value)
}

func (sections *channelWorkspaceProposalParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "target_path":
		sections.TargetPath = appendChannelWorkspaceProposalSectionLine(sections.TargetPath, value)
	case "proposal":
		sections.Proposal = appendChannelWorkspaceProposalSectionLine(sections.Proposal, value)
	case "rationale":
		sections.Rationale = appendChannelWorkspaceProposalSectionLine(sections.Rationale, value)
	}
}

func appendChannelWorkspaceProposalSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelWorkspaceProposalSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelWorkspaceProposalHeader(name) {
	case "title":
		return "title", strings.TrimSpace(value), true
	case "target_path":
		return "target_path", strings.TrimSpace(value), true
	case "proposal":
		return "proposal", strings.TrimSpace(value), true
	case "rationale":
		return "rationale", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelWorkspaceProposalHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "title", "workspace", "workspace proposal", "context proposal", "proposal title":
		return "title"
	case "target", "target path", "workspace path", "path", "file":
		return "target_path"
	case "proposal", "context", "workspace context", "spec", "body", "notes":
		return "proposal"
	case "rationale", "why", "reason", "evidence":
		return "rationale"
	default:
		return ""
	}
}

func cleanChannelWorkspaceProposalTrailingLines(trailing string) []string {
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

func normalizeChannelWorkspaceProposalOptions(opts ChannelWorkspaceProposalOptions) ChannelWorkspaceProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.WorkspaceProposalID = cleanChannelWorkspaceProposalID(opts.WorkspaceProposalID)
	opts.Title = strings.TrimSpace(opts.Title)
	opts.TargetPath = cleanChannelWorkspaceProposalTargetPath(opts.TargetPath, opts.WorkspaceProposalID)
	opts.Proposal = strings.TrimSpace(opts.Proposal)
	opts.Rationale = strings.TrimSpace(opts.Rationale)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelWorkspaceProposalRoute(cfg Config, opts ChannelWorkspaceProposalOptions) (ChannelWorkspaceProposalOptions, error) {
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

func validateChannelWorkspaceProposalOptions(opts ChannelWorkspaceProposalOptions) error {
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
	if opts.WorkspaceProposalID == "" {
		return fmt.Errorf("missing workspace proposal id")
	}
	if !skillNamePattern.MatchString(opts.WorkspaceProposalID) {
		return fmt.Errorf("invalid workspace proposal id %q", opts.WorkspaceProposalID)
	}
	if err := validateChannelWorkspaceProposalTargetPath(opts.TargetPath); err != nil {
		return err
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing workspace proposal source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing workspace proposal title")
	}
	if opts.Proposal == "" {
		return fmt.Errorf("missing workspace proposal body")
	}
	return nil
}

func validateChannelWorkspaceProposalActionRequestOptions(opts ChannelWorkspaceProposalOptions) error {
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
	if opts.WorkspaceProposalID == "" {
		return fmt.Errorf("missing workspace proposal id")
	}
	if !skillNamePattern.MatchString(opts.WorkspaceProposalID) {
		return fmt.Errorf("invalid workspace proposal id %q", opts.WorkspaceProposalID)
	}
	if err := validateChannelWorkspaceProposalTargetPath(opts.TargetPath); err != nil {
		return err
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing workspace proposal source issue")
	}
	if opts.Title == "" {
		return fmt.Errorf("missing workspace proposal title")
	}
	if opts.Proposal == "" {
		return fmt.Errorf("missing workspace proposal body")
	}
	return nil
}

func findOrCreateChannelWorkspaceProposalIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelWorkspaceProposalOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel workspace proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelWorkspaceProposalMatches(issue.Body, opts.WorkspaceProposalID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelWorkspaceProposalIssueTitle(opts), RenderChannelWorkspaceProposalIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel workspace proposal issue: %w", err)
	}
	return issue, true, false, nil
}

func channelWorkspaceProposalIssueTitle(opts ChannelWorkspaceProposalOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Title), "\n", " ")
	if title == "" {
		title = opts.WorkspaceProposalID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel workspace proposal: " + title
}

func channelWorkspaceProposalMatches(body, workspaceProposalID string) bool {
	return HasChannelWorkspaceProposalMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`workspace_proposal_id="%s"`, escapeMarkerValue(cleanChannelWorkspaceProposalID(workspaceProposalID))))
}

func cleanChannelWorkspaceProposalID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelWorkspaceProposalTargetPath(value, proposalID string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "./")
	if value == "" && strings.TrimSpace(proposalID) != "" {
		value = ".gitclaw/workspaces/" + cleanChannelWorkspaceProposalID(proposalID) + ".md"
	}
	return value
}

func validateChannelWorkspaceProposalTargetPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("missing workspace proposal target path")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("workspace proposal target path cannot contain '..'")
	}
	if !strings.HasPrefix(path, ".gitclaw/workspaces/") {
		return fmt.Errorf("workspace proposal target path must be under .gitclaw/workspaces")
	}
	if !strings.HasSuffix(path, ".md") {
		return fmt.Errorf("workspace proposal target path must be a markdown file")
	}
	return nil
}

func autoChannelWorkspaceProposalID(ev Event, channel, threadID, sourceMessageID, title, targetPath, proposal, rationale string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, title, targetPath, proposal, rationale}, "|")
	return fmt.Sprintf("workspace-proposal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelWorkspaceProposalNotifyMessageID(ev Event, workspaceProposalID string) string {
	seed := strings.Join([]string{eventID(ev), workspaceProposalID}, "|")
	return fmt.Sprintf("gitclaw-channel-workspace-proposal-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelWorkspaceProposalNotificationBody(opts ChannelWorkspaceProposalOptions, workspaceProposalIssueNumber int, workspaceProposalIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel workspace proposal recorded.\n\n")
	if workspaceProposalIssueNumber > 0 {
		fmt.Fprintf(&b, "Workspace proposal: #%d\n", workspaceProposalIssueNumber)
	}
	if workspaceProposalIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", workspaceProposalIssueURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(opts.Title))
	fmt.Fprintf(&b, "Target path: %s\n", strings.TrimSpace(opts.TargetPath))
	b.WriteString("\nRecorded in the linked GitHub issue. No workspace file was written.")
	return strings.TrimSpace(b.String())
}
