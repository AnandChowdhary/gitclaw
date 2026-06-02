package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelToolsetProposalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	ToolsetID         string
	Name              string
	Purpose           string
	Tools             string
	Policy            string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelToolsetProposalResult struct {
	ProposalIssueNumber int
	ProposalIssueURL    string
	ProposalCreated     bool
	ProposalDuplicate   bool
	Notification        ChannelSendResult
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelToolsetProposalActionRequest struct {
	Options             ChannelToolsetProposalOptions
	Command             string
	Subcommand          string
	AutoToolsetID       bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	ToolCount           int
	NameSHA             string
	NameBytes           int
	NameLines           int
	PurposeSHA          string
	PurposeBytes        int
	PurposeLines        int
	ToolsSHA            string
	ToolsBytes          int
	ToolsLines          int
	PolicySHA           string
	PolicyBytes         int
	PolicyLines         int
	NotesSHA            string
	NotesBytes          int
	NotesLines          int
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelToolsetProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelToolsetProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolsetProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-toolset", "toolset-proposal", "toolset", "propose-tools", "tool-bundle", "tools-bundle":
		return true
	default:
		return false
	}
}

func BuildChannelToolsetProposalActionRequest(ev Event, cfg Config) (ChannelToolsetProposalActionRequest, error) {
	fields, trailing, ok := channelToolsetProposalActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolsetProposalActionRequest{}, fmt.Errorf("missing channel toolset proposal command")
	}
	req := ChannelToolsetProposalActionRequest{
		Options: ChannelToolsetProposalOptions{
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
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--toolset-id", "--proposal-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ToolsetID = cleanChannelToolsetProposalID(fields[i+1])
			i++
		case "--name", "--title":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Name = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolsetProposalActionRequest{}, fmt.Errorf("unknown channel toolset proposal argument %q", field)
			}
			if req.Options.ToolsetID == "" {
				req.Options.ToolsetID = cleanChannelToolsetProposalID(field)
				continue
			}
			if req.Options.Name == "" {
				req.Options.Name = field
				continue
			}
			return ChannelToolsetProposalActionRequest{}, fmt.Errorf("unexpected channel toolset proposal argument %q", field)
		}
	}
	if err := applyChannelToolsetProposalIssueTarget(ev, &req); err != nil {
		return ChannelToolsetProposalActionRequest{}, err
	}
	name, purpose, tools, policy, notes := parseChannelToolsetProposalSections(trailing, ev)
	if strings.TrimSpace(req.Options.Name) == "" {
		req.Options.Name = name
	}
	req.Options.Purpose = purpose
	req.Options.Tools = tools
	req.Options.Policy = policy
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.ToolsetID) == "" {
		req.Options.ToolsetID = autoChannelToolsetProposalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Name, tools, policy, notes)
		req.AutoToolsetID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolsetProposalNotifyMessageID(ev, req.Options.ToolsetID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolsetProposalOptions(req.Options)
	if err := validateChannelToolsetProposalOptions(req.Options); err != nil {
		return ChannelToolsetProposalActionRequest{}, err
	}
	req.ToolCount = len(channelToolsetProposalToolLines(req.Options.Tools))
	req.NameSHA = shortDocumentHash(req.Options.Name)
	req.NameBytes = len(req.Options.Name)
	req.NameLines = lineCount(req.Options.Name)
	req.PurposeSHA = shortDocumentHash(req.Options.Purpose)
	req.PurposeBytes = len(req.Options.Purpose)
	req.PurposeLines = lineCount(req.Options.Purpose)
	req.ToolsSHA = shortDocumentHash(req.Options.Tools)
	req.ToolsBytes = len(req.Options.Tools)
	req.ToolsLines = lineCount(req.Options.Tools)
	req.PolicySHA = shortDocumentHash(req.Options.Policy)
	req.PolicyBytes = len(req.Options.Policy)
	req.PolicyLines = lineCount(req.Options.Policy)
	req.NotesSHA = shortDocumentHash(req.Options.Notes)
	req.NotesBytes = len(req.Options.Notes)
	req.NotesLines = lineCount(req.Options.Notes)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelToolsetProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelToolsetProposal(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolsetProposalOptions) (ChannelToolsetProposalResult, error) {
	opts = normalizeChannelToolsetProposalOptions(opts)
	if err := validateChannelToolsetProposalOptions(opts); err != nil {
		return ChannelToolsetProposalResult{}, err
	}
	proposalIssue, created, duplicate, err := findOrCreateChannelToolsetProposalIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelToolsetProposalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelToolsetProposalNotificationBody(opts, proposalIssue.Number, issueURL(opts.Repo, proposalIssue.Number)),
	})
	if err != nil {
		return ChannelToolsetProposalResult{}, fmt.Errorf("queue channel toolset proposal notification: %w", err)
	}
	return ChannelToolsetProposalResult{
		ProposalIssueNumber: proposalIssue.Number,
		ProposalIssueURL:    issueURL(opts.Repo, proposalIssue.Number),
		ProposalCreated:     created,
		ProposalDuplicate:   duplicate,
		Notification:        notification,
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelToolsetProposalActionReport(ev Event, req ChannelToolsetProposalActionRequest, result ChannelToolsetProposalResult) string {
	status := "created"
	switch {
	case result.ProposalDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.ProposalDuplicate:
		status = "existing"
	}
	notificationQueued := result.Notification.CommentID != 0 && !result.Notification.Duplicate
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
	b.WriteString("## GitClaw Channel Toolset Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_toolset_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- toolset_proposal_issue: `#%d`\n", result.ProposalIssueNumber)
	fmt.Fprintf(&b, "- toolset_proposal_issue_url: `%s`\n", result.ProposalIssueURL)
	fmt.Fprintf(&b, "- toolset_proposal_issue_created: `%t`\n", result.ProposalCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.ProposalDuplicate)
	fmt.Fprintf(&b, "- notification_target_issue: `#%d`\n", result.Notification.IssueNumber)
	fmt.Fprintf(&b, "- notification_comment_id: `%d`\n", result.Notification.CommentID)
	fmt.Fprintf(&b, "- notification_queued: `%t`\n", notificationQueued)
	fmt.Fprintf(&b, "- notification_duplicate_suppressed: `%t`\n", result.Notification.Duplicate)
	fmt.Fprintf(&b, "- channel: `%s`\n", result.Channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- toolset_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.ToolsetID))
	fmt.Fprintf(&b, "- toolset_id_auto: `%t`\n", req.AutoToolsetID)
	fmt.Fprintf(&b, "- tool_count: `%d`\n", req.ToolCount)
	fmt.Fprintf(&b, "- toolset_name_sha256_12: `%s`\n", req.NameSHA)
	fmt.Fprintf(&b, "- toolset_name_bytes: `%d`\n", req.NameBytes)
	fmt.Fprintf(&b, "- toolset_name_lines: `%d`\n", req.NameLines)
	fmt.Fprintf(&b, "- toolset_purpose_sha256_12: `%s`\n", req.PurposeSHA)
	fmt.Fprintf(&b, "- toolset_purpose_bytes: `%d`\n", req.PurposeBytes)
	fmt.Fprintf(&b, "- toolset_purpose_lines: `%d`\n", req.PurposeLines)
	fmt.Fprintf(&b, "- toolset_tools_sha256_12: `%s`\n", req.ToolsSHA)
	fmt.Fprintf(&b, "- toolset_tools_bytes: `%d`\n", req.ToolsBytes)
	fmt.Fprintf(&b, "- toolset_tools_lines: `%d`\n", req.ToolsLines)
	fmt.Fprintf(&b, "- toolset_policy_sha256_12: `%s`\n", req.PolicySHA)
	fmt.Fprintf(&b, "- toolset_policy_bytes: `%d`\n", req.PolicyBytes)
	fmt.Fprintf(&b, "- toolset_policy_lines: `%d`\n", req.PolicyLines)
	fmt.Fprintf(&b, "- toolset_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- toolset_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- toolset_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-toolset")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- toolset_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- active_tool_config_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_purpose_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_tools_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_policy_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_toolset_proposal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a channel-origin toolset proposal, then queued a provider-facing proposal link back to the mirrored thread. This action does not call a model, execute tools, enable toolsets, write tool configuration, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Proposal Review Path\n")
	fmt.Fprintf(&b, "- continue on toolset proposal issue: `#%d`\n", result.ProposalIssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the proposal issue to refine the reviewed toolset bundle\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolsetProposalIssueBody(opts ChannelToolsetProposalOptions) string {
	tools := channelToolsetProposalToolLines(opts.Tools)
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-toolset-proposal toolset_id=\"%s\" channel=\"%s\" tools_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.ToolsetID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Tools), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel toolset proposal.\n\n")
	fmt.Fprintf(&b, "- toolset_id: %s\n", opts.ToolsetID)
	fmt.Fprintf(&b, "- toolset_name: %s\n", opts.Name)
	fmt.Fprintf(&b, "- tool_count: %d\n", len(tools))
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- proposal_store: github-issue-to-git-reviewed-toolset\n")
	fmt.Fprintf(&b, "- review_pr_required: true\n")
	fmt.Fprintf(&b, "- toolset_enabled: false\n")
	fmt.Fprintf(&b, "- active_tool_config_write_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Purpose\n\n")
	b.WriteString(strings.TrimSpace(opts.Purpose))
	b.WriteString("\n\n## Proposed Tools\n\n")
	for _, tool := range tools {
		fmt.Fprintf(&b, "- %s\n", tool)
	}
	if strings.TrimSpace(opts.Policy) != "" {
		b.WriteString("\n## Policy\n\n")
		b.WriteString(strings.TrimSpace(opts.Policy))
	}
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for deciding whether this toolset should become config, documentation, a skill, an approval plan, or a proactive workflow. Activation requires normal GitHub review.")
	return strings.TrimSpace(b.String())
}

func channelToolsetProposalActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolsetProposalActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolsetProposalIssueTarget(ev Event, req *ChannelToolsetProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel toolset proposal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelToolsetProposalSections(trailing string, ev Event) (string, string, string, string, string) {
	lines := cleanChannelToolsetProposalTrailingLines(trailing)
	sections := channelToolsetProposalParsedSections{
		Name:    fmt.Sprintf("Channel toolset proposal from issue #%d", ev.Issue.Number),
		Purpose: "Review whether this channel-origin toolset bundle should become a GitClaw tool configuration.",
	}
	current := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != "" {
				sections.append(current, "")
			}
			continue
		}
		section, value, ok := parseChannelToolsetProposalSectionHeader(trimmed)
		if ok {
			current = section
			if value != "" {
				sections.setOrAppend(current, value)
			}
			continue
		}
		if i == 0 && current == "" {
			sections.Name = trimmed
			continue
		}
		if current == "" {
			current = "tools"
		}
		sections.append(current, line)
	}
	return strings.TrimSpace(sections.Name), strings.TrimSpace(sections.Purpose), strings.TrimSpace(sections.Tools), strings.TrimSpace(sections.Policy), strings.TrimSpace(sections.Notes)
}

type channelToolsetProposalParsedSections struct {
	Name    string
	Purpose string
	Tools   string
	Policy  string
	Notes   string
}

func (sections *channelToolsetProposalParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	switch section {
	case "name":
		sections.Name = strings.TrimSpace(value)
	case "purpose":
		sections.Purpose = appendChannelToolsetProposalSectionLine(sections.Purpose, value)
	case "tools":
		sections.Tools = appendChannelToolsetProposalSectionLine(sections.Tools, value)
	case "policy":
		sections.Policy = appendChannelToolsetProposalSectionLine(sections.Policy, value)
	case "notes":
		sections.Notes = appendChannelToolsetProposalSectionLine(sections.Notes, value)
	}
}

func (sections *channelToolsetProposalParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "purpose":
		sections.Purpose = appendChannelToolsetProposalSectionLine(sections.Purpose, value)
	case "tools":
		sections.Tools = appendChannelToolsetProposalSectionLine(sections.Tools, value)
	case "policy":
		sections.Policy = appendChannelToolsetProposalSectionLine(sections.Policy, value)
	case "notes":
		sections.Notes = appendChannelToolsetProposalSectionLine(sections.Notes, value)
	}
}

func appendChannelToolsetProposalSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelToolsetProposalSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelToolsetProposalHeader(name) {
	case "name":
		return "name", strings.TrimSpace(value), true
	case "purpose":
		return "purpose", strings.TrimSpace(value), true
	case "tools":
		return "tools", strings.TrimSpace(value), true
	case "policy":
		return "policy", strings.TrimSpace(value), true
	case "notes":
		return "notes", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelToolsetProposalHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "name", "title", "toolset", "toolset name", "summary":
		return "name"
	case "purpose", "why", "goal", "goals", "use case", "use cases":
		return "purpose"
	case "tools", "tool", "proposed tools", "bundle", "tool bundle", "capabilities":
		return "tools"
	case "policy", "gates", "guardrails", "approval", "approval policy", "risk":
		return "policy"
	case "notes", "context", "details", "description":
		return "notes"
	default:
		return ""
	}
}

func cleanChannelToolsetProposalTrailingLines(trailing string) []string {
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

func normalizeChannelToolsetProposalOptions(opts ChannelToolsetProposalOptions) ChannelToolsetProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.ToolsetID = cleanChannelToolsetProposalID(opts.ToolsetID)
	opts.Name = strings.TrimSpace(opts.Name)
	opts.Purpose = strings.TrimSpace(opts.Purpose)
	opts.Tools = strings.TrimSpace(opts.Tools)
	opts.Policy = strings.TrimSpace(opts.Policy)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelToolsetProposalOptions(opts ChannelToolsetProposalOptions) error {
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
	if opts.ToolsetID == "" {
		return fmt.Errorf("missing toolset proposal id")
	}
	if !skillNamePattern.MatchString(opts.ToolsetID) {
		return fmt.Errorf("invalid toolset proposal id %q", opts.ToolsetID)
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	if opts.Name == "" {
		return fmt.Errorf("missing toolset proposal name")
	}
	if opts.Purpose == "" {
		return fmt.Errorf("missing toolset proposal purpose")
	}
	if len(channelToolsetProposalToolLines(opts.Tools)) == 0 {
		return fmt.Errorf("missing proposed tools")
	}
	return nil
}

func findOrCreateChannelToolsetProposalIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelToolsetProposalOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel toolset proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelToolsetProposalMatches(issue.Body, opts.ToolsetID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelToolsetProposalIssueTitle(opts), RenderChannelToolsetProposalIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel toolset proposal issue: %w", err)
	}
	return issue, true, false, nil
}

func channelToolsetProposalIssueTitle(opts ChannelToolsetProposalOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Name), "\n", " ")
	if title == "" {
		title = opts.ToolsetID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel toolset proposal: " + title
}

func channelToolsetProposalMatches(body, toolsetID string) bool {
	return HasChannelToolsetProposalMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`toolset_id="%s"`, escapeMarkerValue(cleanChannelToolsetProposalID(toolsetID))))
}

func cleanChannelToolsetProposalID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelToolsetProposalID(ev Event, channel, threadID, sourceMessageID, name, tools, policy, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, name, tools, policy, notes}, "|")
	return fmt.Sprintf("toolset-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelToolsetProposalNotifyMessageID(ev Event, toolsetID string) string {
	seed := strings.Join([]string{eventID(ev), toolsetID}, "|")
	return fmt.Sprintf("gitclaw-channel-toolset-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelToolsetProposalNotificationBody(opts ChannelToolsetProposalOptions, proposalIssueNumber int, proposalIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel toolset proposal captured.\n\n")
	if proposalIssueNumber > 0 {
		fmt.Fprintf(&b, "Toolset proposal issue: #%d\n", proposalIssueNumber)
	}
	if proposalIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", proposalIssueURL)
	}
	fmt.Fprintf(&b, "Name: %s\n", strings.TrimSpace(opts.Name))
	fmt.Fprintf(&b, "Tools: %d\n", len(channelToolsetProposalToolLines(opts.Tools)))
	fmt.Fprintf(&b, "Review PR required: %t\n", true)
	fmt.Fprintf(&b, "Toolset enabled: %t\n", false)
	b.WriteString("\nContinue tracking it in the linked GitHub issue. This notification did not enable tools, call a model, execute a tool, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelToolsetProposalToolLines(tools string) []string {
	lines := strings.Split(strings.TrimSpace(tools), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		tool := cleanChannelToolsetProposalToolLine(line)
		if tool == "" {
			continue
		}
		cleaned = append(cleaned, tool)
	}
	return cleaned
}

func cleanChannelToolsetProposalToolLine(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"- [ ]", "- [x]", "- [X]", "* [ ]", "* [x]", "* [X]", "- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(line) && (line[i] == '.' || line[i] == ')') && line[i+1] == ' ' {
		return strings.TrimSpace(line[i+2:])
	}
	return line
}
