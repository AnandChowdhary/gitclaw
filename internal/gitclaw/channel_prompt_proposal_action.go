package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPromptProposalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	PromptID          string
	Name              string
	Purpose           string
	Prompt            string
	Inputs            string
	Policy            string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelPromptProposalResult struct {
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

type ChannelPromptProposalActionRequest struct {
	Options             ChannelPromptProposalOptions
	Command             string
	Subcommand          string
	AutoPromptID        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	NameSHA             string
	NameBytes           int
	NameLines           int
	PurposeSHA          string
	PurposeBytes        int
	PurposeLines        int
	PromptSHA           string
	PromptBytes         int
	PromptLines         int
	InputsSHA           string
	InputsBytes         int
	InputsLines         int
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

func IsChannelPromptProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelPromptProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPromptProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-prompt", "prompt-proposal", "propose-prompt-pack", "prompt-pack-proposal", "prompt-pack", "prompt":
		return true
	default:
		return false
	}
}

func BuildChannelPromptProposalActionRequest(ev Event, cfg Config) (ChannelPromptProposalActionRequest, error) {
	fields, trailing, ok := channelPromptProposalActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPromptProposalActionRequest{}, fmt.Errorf("missing channel prompt proposal command")
	}
	req := ChannelPromptProposalActionRequest{
		Options: ChannelPromptProposalOptions{
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
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--prompt-id", "--proposal-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.PromptID = cleanChannelPromptProposalID(fields[i+1])
			i++
		case "--name", "--title":
			if i+1 >= len(fields) {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Name = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPromptProposalActionRequest{}, fmt.Errorf("unknown channel prompt proposal argument %q", field)
			}
			if req.Options.PromptID == "" {
				req.Options.PromptID = cleanChannelPromptProposalID(field)
				continue
			}
			if req.Options.Name == "" {
				req.Options.Name = field
				continue
			}
			return ChannelPromptProposalActionRequest{}, fmt.Errorf("unexpected channel prompt proposal argument %q", field)
		}
	}
	if err := applyChannelPromptProposalIssueTarget(ev, &req); err != nil {
		return ChannelPromptProposalActionRequest{}, err
	}
	name, purpose, prompt, inputs, policy, notes := parseChannelPromptProposalSections(trailing, ev)
	if strings.TrimSpace(req.Options.Name) == "" {
		req.Options.Name = name
	}
	req.Options.Purpose = purpose
	req.Options.Prompt = prompt
	req.Options.Inputs = inputs
	req.Options.Policy = policy
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.PromptID) == "" {
		req.Options.PromptID = autoChannelPromptProposalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Name, prompt, policy, notes)
		req.AutoPromptID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelPromptProposalNotifyMessageID(ev, req.Options.PromptID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelPromptProposalOptions(req.Options)
	if err := validateChannelPromptProposalOptions(req.Options); err != nil {
		return ChannelPromptProposalActionRequest{}, err
	}
	req.NameSHA = shortDocumentHash(req.Options.Name)
	req.NameBytes = len(req.Options.Name)
	req.NameLines = lineCount(req.Options.Name)
	req.PurposeSHA = shortDocumentHash(req.Options.Purpose)
	req.PurposeBytes = len(req.Options.Purpose)
	req.PurposeLines = lineCount(req.Options.Purpose)
	req.PromptSHA = shortDocumentHash(req.Options.Prompt)
	req.PromptBytes = len(req.Options.Prompt)
	req.PromptLines = lineCount(req.Options.Prompt)
	req.InputsSHA = shortDocumentHash(req.Options.Inputs)
	req.InputsBytes = len(req.Options.Inputs)
	req.InputsLines = lineCount(req.Options.Inputs)
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelPromptProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelPromptProposal(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPromptProposalOptions) (ChannelPromptProposalResult, error) {
	opts = normalizeChannelPromptProposalOptions(opts)
	if err := validateChannelPromptProposalOptions(opts); err != nil {
		return ChannelPromptProposalResult{}, err
	}
	proposalIssue, created, duplicate, err := findOrCreateChannelPromptProposalIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelPromptProposalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelPromptProposalNotificationBody(opts, proposalIssue.Number, issueURL(opts.Repo, proposalIssue.Number)),
	})
	if err != nil {
		return ChannelPromptProposalResult{}, fmt.Errorf("queue channel prompt proposal notification: %w", err)
	}
	return ChannelPromptProposalResult{
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

func RenderChannelPromptProposalActionReport(ev Event, req ChannelPromptProposalActionRequest, result ChannelPromptProposalResult) string {
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
	b.WriteString("## GitClaw Channel Prompt Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_prompt_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- prompt_proposal_issue: `#%d`\n", result.ProposalIssueNumber)
	fmt.Fprintf(&b, "- prompt_proposal_issue_url: `%s`\n", result.ProposalIssueURL)
	fmt.Fprintf(&b, "- prompt_proposal_issue_created: `%t`\n", result.ProposalCreated)
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
	fmt.Fprintf(&b, "- prompt_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.PromptID))
	fmt.Fprintf(&b, "- prompt_id_auto: `%t`\n", req.AutoPromptID)
	fmt.Fprintf(&b, "- prompt_name_sha256_12: `%s`\n", req.NameSHA)
	fmt.Fprintf(&b, "- prompt_name_bytes: `%d`\n", req.NameBytes)
	fmt.Fprintf(&b, "- prompt_name_lines: `%d`\n", req.NameLines)
	fmt.Fprintf(&b, "- prompt_purpose_sha256_12: `%s`\n", req.PurposeSHA)
	fmt.Fprintf(&b, "- prompt_purpose_bytes: `%d`\n", req.PurposeBytes)
	fmt.Fprintf(&b, "- prompt_purpose_lines: `%d`\n", req.PurposeLines)
	fmt.Fprintf(&b, "- prompt_draft_sha256_12: `%s`\n", req.PromptSHA)
	fmt.Fprintf(&b, "- prompt_draft_bytes: `%d`\n", req.PromptBytes)
	fmt.Fprintf(&b, "- prompt_draft_lines: `%d`\n", req.PromptLines)
	fmt.Fprintf(&b, "- prompt_inputs_sha256_12: `%s`\n", req.InputsSHA)
	fmt.Fprintf(&b, "- prompt_inputs_bytes: `%d`\n", req.InputsBytes)
	fmt.Fprintf(&b, "- prompt_inputs_lines: `%d`\n", req.InputsLines)
	fmt.Fprintf(&b, "- prompt_policy_sha256_12: `%s`\n", req.PolicySHA)
	fmt.Fprintf(&b, "- prompt_policy_bytes: `%d`\n", req.PolicyBytes)
	fmt.Fprintf(&b, "- prompt_policy_lines: `%d`\n", req.PolicyLines)
	fmt.Fprintf(&b, "- prompt_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- prompt_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- prompt_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-prompt-pack")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- prompt_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- prompt_test_run_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- prompt_pack_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_purpose_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_draft_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_policy_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_prompt_proposal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a channel-origin prompt-pack proposal, then queued a provider-facing proposal link back to the mirrored thread. This action does not call a model, run prompt tests, enable prompts, write prompt configuration, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Proposal Review Path\n")
	fmt.Fprintf(&b, "- continue on prompt proposal issue: `#%d`\n", result.ProposalIssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the proposal issue to refine or test the reviewed prompt pack\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelPromptProposalIssueBody(opts ChannelPromptProposalOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-prompt-proposal prompt_id=\"%s\" channel=\"%s\" prompt_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.PromptID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Prompt), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel prompt proposal.\n\n")
	fmt.Fprintf(&b, "- prompt_id: %s\n", opts.PromptID)
	fmt.Fprintf(&b, "- prompt_name: %s\n", opts.Name)
	fmt.Fprintf(&b, "- prompt_draft_bytes: %d\n", len(opts.Prompt))
	fmt.Fprintf(&b, "- prompt_draft_lines: %d\n", lineCount(opts.Prompt))
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- proposal_store: github-issue-to-git-reviewed-prompt-pack\n")
	fmt.Fprintf(&b, "- review_pr_required: true\n")
	fmt.Fprintf(&b, "- prompt_enabled: false\n")
	fmt.Fprintf(&b, "- prompt_pack_write_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Purpose\n\n")
	b.WriteString(strings.TrimSpace(opts.Purpose))
	b.WriteString("\n\n## Prompt Draft\n\n")
	b.WriteString(strings.TrimSpace(opts.Prompt))
	if strings.TrimSpace(opts.Inputs) != "" {
		b.WriteString("\n\n## Inputs\n\n")
		b.WriteString(strings.TrimSpace(opts.Inputs))
	}
	if strings.TrimSpace(opts.Policy) != "" {
		b.WriteString("\n\n## Policy\n\n")
		b.WriteString(strings.TrimSpace(opts.Policy))
	}
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for deciding whether this prompt pack should become documentation, a skill, repo-local prompt guidance, a proactive workflow, or a test fixture. Activation requires normal GitHub review.")
	return strings.TrimSpace(b.String())
}

func channelPromptProposalActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPromptProposalActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelPromptProposalIssueTarget(ev Event, req *ChannelPromptProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel prompt proposal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelPromptProposalSections(trailing string, ev Event) (string, string, string, string, string, string) {
	lines := cleanChannelPromptProposalTrailingLines(trailing)
	sections := channelPromptProposalParsedSections{
		Name:    fmt.Sprintf("Channel prompt proposal from issue #%d", ev.Issue.Number),
		Purpose: "Review whether this channel-origin prompt should become a GitClaw prompt pack.",
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
		section, value, ok := parseChannelPromptProposalSectionHeader(trimmed)
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
			current = "prompt"
		}
		sections.append(current, line)
	}
	return strings.TrimSpace(sections.Name), strings.TrimSpace(sections.Purpose), strings.TrimSpace(sections.Prompt), strings.TrimSpace(sections.Inputs), strings.TrimSpace(sections.Policy), strings.TrimSpace(sections.Notes)
}

type channelPromptProposalParsedSections struct {
	Name    string
	Purpose string
	Prompt  string
	Inputs  string
	Policy  string
	Notes   string
}

func (sections *channelPromptProposalParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	switch section {
	case "name":
		sections.Name = strings.TrimSpace(value)
	case "purpose":
		sections.Purpose = appendChannelPromptProposalSectionLine(sections.Purpose, value)
	case "prompt":
		sections.Prompt = appendChannelPromptProposalSectionLine(sections.Prompt, value)
	case "inputs":
		sections.Inputs = appendChannelPromptProposalSectionLine(sections.Inputs, value)
	case "policy":
		sections.Policy = appendChannelPromptProposalSectionLine(sections.Policy, value)
	case "notes":
		sections.Notes = appendChannelPromptProposalSectionLine(sections.Notes, value)
	}
}

func (sections *channelPromptProposalParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "purpose":
		sections.Purpose = appendChannelPromptProposalSectionLine(sections.Purpose, value)
	case "prompt":
		sections.Prompt = appendChannelPromptProposalSectionLine(sections.Prompt, value)
	case "inputs":
		sections.Inputs = appendChannelPromptProposalSectionLine(sections.Inputs, value)
	case "policy":
		sections.Policy = appendChannelPromptProposalSectionLine(sections.Policy, value)
	case "notes":
		sections.Notes = appendChannelPromptProposalSectionLine(sections.Notes, value)
	}
}

func appendChannelPromptProposalSectionLine(current, line string) string {
	if current == "" {
		return strings.TrimSpace(line)
	}
	if strings.TrimSpace(line) == "" {
		return strings.TrimRight(current, " \t\r\n") + "\n\n"
	}
	return strings.TrimRight(current, " \t\r\n") + "\n" + strings.TrimRight(line, " \t\r")
}

func parseChannelPromptProposalSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelPromptProposalHeader(name) {
	case "name":
		return "name", strings.TrimSpace(value), true
	case "purpose":
		return "purpose", strings.TrimSpace(value), true
	case "prompt":
		return "prompt", strings.TrimSpace(value), true
	case "inputs":
		return "inputs", strings.TrimSpace(value), true
	case "policy":
		return "policy", strings.TrimSpace(value), true
	case "notes":
		return "notes", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelPromptProposalHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "name", "title", "prompt name", "summary":
		return "name"
	case "purpose", "why", "goal", "goals", "use case", "use cases":
		return "purpose"
	case "prompt", "draft", "prompt draft", "system prompt", "instruction", "instructions", "template", "prompt text", "body":
		return "prompt"
	case "inputs", "input", "arguments", "variables", "slots", "parameters":
		return "inputs"
	case "policy", "gates", "guardrails", "approval", "approval policy", "risk":
		return "policy"
	case "notes", "context", "details", "description":
		return "notes"
	default:
		return ""
	}
}

func cleanChannelPromptProposalTrailingLines(trailing string) []string {
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

func normalizeChannelPromptProposalOptions(opts ChannelPromptProposalOptions) ChannelPromptProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.PromptID = cleanChannelPromptProposalID(opts.PromptID)
	opts.Name = strings.TrimSpace(opts.Name)
	opts.Purpose = strings.TrimSpace(opts.Purpose)
	opts.Prompt = strings.TrimSpace(opts.Prompt)
	opts.Inputs = strings.TrimSpace(opts.Inputs)
	opts.Policy = strings.TrimSpace(opts.Policy)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelPromptProposalOptions(opts ChannelPromptProposalOptions) error {
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
	if opts.PromptID == "" {
		return fmt.Errorf("missing prompt proposal id")
	}
	if !skillNamePattern.MatchString(opts.PromptID) {
		return fmt.Errorf("invalid prompt proposal id %q", opts.PromptID)
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	if opts.Name == "" {
		return fmt.Errorf("missing prompt proposal name")
	}
	if opts.Purpose == "" {
		return fmt.Errorf("missing prompt proposal purpose")
	}
	if opts.Prompt == "" {
		return fmt.Errorf("missing prompt draft")
	}
	return nil
}

func findOrCreateChannelPromptProposalIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPromptProposalOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel prompt proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelPromptProposalMatches(issue.Body, opts.PromptID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelPromptProposalIssueTitle(opts), RenderChannelPromptProposalIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel prompt proposal issue: %w", err)
	}
	return issue, true, false, nil
}

func channelPromptProposalIssueTitle(opts ChannelPromptProposalOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Name), "\n", " ")
	if title == "" {
		title = opts.PromptID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel prompt proposal: " + title
}

func channelPromptProposalMatches(body, promptID string) bool {
	return HasChannelPromptProposalMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`prompt_id="%s"`, escapeMarkerValue(cleanChannelPromptProposalID(promptID))))
}

func cleanChannelPromptProposalID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelPromptProposalID(ev Event, channel, threadID, sourceMessageID, name, prompt, policy, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, name, prompt, policy, notes}, "|")
	return fmt.Sprintf("prompt-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPromptProposalNotifyMessageID(ev Event, promptID string) string {
	seed := strings.Join([]string{eventID(ev), promptID}, "|")
	return fmt.Sprintf("gitclaw-channel-prompt-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelPromptProposalNotificationBody(opts ChannelPromptProposalOptions, proposalIssueNumber int, proposalIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel prompt proposal captured.\n\n")
	if proposalIssueNumber > 0 {
		fmt.Fprintf(&b, "Prompt proposal issue: #%d\n", proposalIssueNumber)
	}
	if proposalIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", proposalIssueURL)
	}
	fmt.Fprintf(&b, "Name: %s\n", strings.TrimSpace(opts.Name))
	fmt.Fprintf(&b, "Prompt lines: %d\n", lineCount(opts.Prompt))
	fmt.Fprintf(&b, "Review PR required: %t\n", true)
	fmt.Fprintf(&b, "Prompt enabled: %t\n", false)
	b.WriteString("\nContinue tracking it in the linked GitHub issue. This notification did not enable a prompt, call a model, run a prompt test, write prompt configuration, or mutate the repository.")
	return strings.TrimSpace(b.String())
}
