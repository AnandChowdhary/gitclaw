package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelBundleProposalOptions struct {
	Repo              string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	BundleID          string
	Name              string
	Purpose           string
	Skills            string
	Instruction       string
	Policy            string
	Notes             string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelBundleProposalResult struct {
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

type ChannelBundleProposalActionRequest struct {
	Options             ChannelBundleProposalOptions
	Command             string
	Subcommand          string
	AutoBundleID        bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	SkillCount          int
	NameSHA             string
	NameBytes           int
	NameLines           int
	PurposeSHA          string
	PurposeBytes        int
	PurposeLines        int
	SkillsSHA           string
	SkillsBytes         int
	SkillsLines         int
	InstructionSHA      string
	InstructionBytes    int
	InstructionLines    int
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

func IsChannelBundleProposalActionRequest(ev Event, cfg Config) bool {
	return isChannelBundleProposalActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBundleProposalActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "propose-bundle", "bundle-proposal", "skill-bundle-proposal", "propose-skill-bundle", "skill-bundle", "bundle":
		return true
	default:
		return false
	}
}

func BuildChannelBundleProposalActionRequest(ev Event, cfg Config) (ChannelBundleProposalActionRequest, error) {
	fields, trailing, ok := channelBundleProposalActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBundleProposalActionRequest{}, fmt.Errorf("missing channel bundle proposal command")
	}
	req := ChannelBundleProposalActionRequest{
		Options: ChannelBundleProposalOptions{
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
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--bundle-id", "--proposal-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.BundleID = cleanChannelBundleProposalID(fields[i+1])
			i++
		case "--name", "--title":
			if i+1 >= len(fields) {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Name = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBundleProposalActionRequest{}, fmt.Errorf("unknown channel bundle proposal argument %q", field)
			}
			if req.Options.BundleID == "" {
				req.Options.BundleID = cleanChannelBundleProposalID(field)
				continue
			}
			if req.Options.Name == "" {
				req.Options.Name = field
				continue
			}
			return ChannelBundleProposalActionRequest{}, fmt.Errorf("unexpected channel bundle proposal argument %q", field)
		}
	}
	if err := applyChannelBundleProposalIssueTarget(ev, &req); err != nil {
		return ChannelBundleProposalActionRequest{}, err
	}
	name, purpose, skills, instruction, policy, notes := parseChannelBundleProposalSections(trailing, ev)
	if strings.TrimSpace(req.Options.Name) == "" {
		req.Options.Name = name
	}
	req.Options.Purpose = purpose
	req.Options.Skills = skills
	req.Options.Instruction = instruction
	req.Options.Policy = policy
	req.Options.Notes = notes
	if strings.TrimSpace(req.Options.BundleID) == "" {
		req.Options.BundleID = autoChannelBundleProposalID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, req.Options.Name, skills, instruction, policy, notes)
		req.AutoBundleID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBundleProposalNotifyMessageID(ev, req.Options.BundleID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBundleProposalOptions(req.Options)
	if err := validateChannelBundleProposalOptions(req.Options); err != nil {
		return ChannelBundleProposalActionRequest{}, err
	}
	req.SkillCount = len(channelBundleProposalSkillLines(req.Options.Skills))
	req.NameSHA = shortDocumentHash(req.Options.Name)
	req.NameBytes = len(req.Options.Name)
	req.NameLines = lineCount(req.Options.Name)
	req.PurposeSHA = shortDocumentHash(req.Options.Purpose)
	req.PurposeBytes = len(req.Options.Purpose)
	req.PurposeLines = lineCount(req.Options.Purpose)
	req.SkillsSHA = shortDocumentHash(req.Options.Skills)
	req.SkillsBytes = len(req.Options.Skills)
	req.SkillsLines = lineCount(req.Options.Skills)
	req.InstructionSHA = shortDocumentHash(req.Options.Instruction)
	req.InstructionBytes = len(req.Options.Instruction)
	req.InstructionLines = lineCount(req.Options.Instruction)
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
	req.NotificationBodySHA = shortDocumentHash(renderChannelBundleProposalNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelBundleProposal(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBundleProposalOptions) (ChannelBundleProposalResult, error) {
	opts = normalizeChannelBundleProposalOptions(opts)
	if err := validateChannelBundleProposalOptions(opts); err != nil {
		return ChannelBundleProposalResult{}, err
	}
	proposalIssue, created, duplicate, err := findOrCreateChannelBundleProposalIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelBundleProposalResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelBundleProposalNotificationBody(opts, proposalIssue.Number, issueURL(opts.Repo, proposalIssue.Number)),
	})
	if err != nil {
		return ChannelBundleProposalResult{}, fmt.Errorf("queue channel bundle proposal notification: %w", err)
	}
	return ChannelBundleProposalResult{
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

func RenderChannelBundleProposalActionReport(ev Event, req ChannelBundleProposalActionRequest, result ChannelBundleProposalResult) string {
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
	b.WriteString("## GitClaw Channel Bundle Proposal Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_bundle_proposal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- bundle_proposal_issue: `#%d`\n", result.ProposalIssueNumber)
	fmt.Fprintf(&b, "- bundle_proposal_issue_url: `%s`\n", result.ProposalIssueURL)
	fmt.Fprintf(&b, "- bundle_proposal_issue_created: `%t`\n", result.ProposalCreated)
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
	fmt.Fprintf(&b, "- bundle_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.BundleID))
	fmt.Fprintf(&b, "- bundle_id_auto: `%t`\n", req.AutoBundleID)
	fmt.Fprintf(&b, "- skill_count: `%d`\n", req.SkillCount)
	fmt.Fprintf(&b, "- bundle_name_sha256_12: `%s`\n", req.NameSHA)
	fmt.Fprintf(&b, "- bundle_name_bytes: `%d`\n", req.NameBytes)
	fmt.Fprintf(&b, "- bundle_name_lines: `%d`\n", req.NameLines)
	fmt.Fprintf(&b, "- bundle_purpose_sha256_12: `%s`\n", req.PurposeSHA)
	fmt.Fprintf(&b, "- bundle_purpose_bytes: `%d`\n", req.PurposeBytes)
	fmt.Fprintf(&b, "- bundle_purpose_lines: `%d`\n", req.PurposeLines)
	fmt.Fprintf(&b, "- bundle_skills_sha256_12: `%s`\n", req.SkillsSHA)
	fmt.Fprintf(&b, "- bundle_skills_bytes: `%d`\n", req.SkillsBytes)
	fmt.Fprintf(&b, "- bundle_skills_lines: `%d`\n", req.SkillsLines)
	fmt.Fprintf(&b, "- bundle_instruction_sha256_12: `%s`\n", req.InstructionSHA)
	fmt.Fprintf(&b, "- bundle_instruction_bytes: `%d`\n", req.InstructionBytes)
	fmt.Fprintf(&b, "- bundle_instruction_lines: `%d`\n", req.InstructionLines)
	fmt.Fprintf(&b, "- bundle_policy_sha256_12: `%s`\n", req.PolicySHA)
	fmt.Fprintf(&b, "- bundle_policy_bytes: `%d`\n", req.PolicyBytes)
	fmt.Fprintf(&b, "- bundle_policy_lines: `%d`\n", req.PolicyLines)
	fmt.Fprintf(&b, "- bundle_notes_sha256_12: `%s`\n", req.NotesSHA)
	fmt.Fprintf(&b, "- bundle_notes_bytes: `%d`\n", req.NotesBytes)
	fmt.Fprintf(&b, "- bundle_notes_lines: `%d`\n", req.NotesLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "github-issue-to-git-reviewed-skill-bundle")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- bundle_yaml_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_purpose_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_skills_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_instruction_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_policy_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bundle_notes_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_bundle_proposal_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing a channel-origin skill-bundle proposal, then queued a provider-facing proposal link back to the mirrored thread. This action does not call a model, install skills, enable bundles, write bundle YAML, copy raw channel bodies, or mutate the repository.\n\n")
	b.WriteString("### Proposal Review Path\n")
	fmt.Fprintf(&b, "- continue on bundle proposal issue: `#%d`\n", result.ProposalIssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up on the proposal issue to refine or test the reviewed skill bundle\n")
	b.WriteString("- provider delivery remains delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelBundleProposalIssueBody(opts ChannelBundleProposalOptions) string {
	skills := channelBundleProposalSkillLines(opts.Skills)
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-bundle-proposal bundle_id=\"%s\" channel=\"%s\" skills_sha256_12=\"%s\" instruction_sha256_12=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.BundleID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.Skills), shortDocumentHash(opts.Instruction), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel skill bundle proposal.\n\n")
	fmt.Fprintf(&b, "- bundle_id: %s\n", opts.BundleID)
	fmt.Fprintf(&b, "- bundle_name: %s\n", opts.Name)
	fmt.Fprintf(&b, "- skill_count: %d\n", len(skills))
	fmt.Fprintf(&b, "- instruction_lines: %d\n", lineCount(opts.Instruction))
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- proposal_store: github-issue-to-git-reviewed-skill-bundle\n")
	fmt.Fprintf(&b, "- review_pr_required: true\n")
	fmt.Fprintf(&b, "- bundle_enabled: false\n")
	fmt.Fprintf(&b, "- skill_install_performed: false\n")
	fmt.Fprintf(&b, "- bundle_yaml_write_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Purpose\n\n")
	b.WriteString(strings.TrimSpace(opts.Purpose))
	b.WriteString("\n\n## Skills\n\n")
	b.WriteString(strings.TrimSpace(opts.Skills))
	if strings.TrimSpace(opts.Instruction) != "" {
		b.WriteString("\n\n## Bundle Instruction\n\n")
		b.WriteString(strings.TrimSpace(opts.Instruction))
	}
	if strings.TrimSpace(opts.Policy) != "" {
		b.WriteString("\n\n## Policy\n\n")
		b.WriteString(strings.TrimSpace(opts.Policy))
	}
	if strings.TrimSpace(opts.Notes) != "" {
		b.WriteString("\n\n## Notes\n\n")
		b.WriteString(strings.TrimSpace(opts.Notes))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for deciding whether this bundle should become `.gitclaw/skill-bundles/<name>.yaml`, documentation, a prompt pack, a proactive workflow, or a test fixture. Activation requires normal GitHub review.")
	return strings.TrimSpace(b.String())
}

func channelBundleProposalActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBundleProposalActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBundleProposalIssueTarget(ev Event, req *ChannelBundleProposalActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel bundle proposal requires a gitclaw:channel-thread issue or an explicit channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelBundleProposalSections(trailing string, ev Event) (string, string, string, string, string, string) {
	lines := cleanChannelToolsetProposalTrailingLines(trailing)
	sections := channelBundleProposalParsedSections{
		Name:    fmt.Sprintf("Channel bundle proposal from issue #%d", ev.Issue.Number),
		Purpose: "Review whether this channel-origin skill bundle should become a GitClaw task profile.",
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
		section, value, ok := parseChannelBundleProposalSectionHeader(trimmed)
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
			current = "skills"
		}
		sections.append(current, line)
	}
	return strings.TrimSpace(sections.Name), strings.TrimSpace(sections.Purpose), strings.TrimSpace(sections.Skills), strings.TrimSpace(sections.Instruction), strings.TrimSpace(sections.Policy), strings.TrimSpace(sections.Notes)
}

type channelBundleProposalParsedSections struct {
	Name        string
	Purpose     string
	Skills      string
	Instruction string
	Policy      string
	Notes       string
}

func (sections *channelBundleProposalParsedSections) setOrAppend(section, value string) {
	if sections == nil {
		return
	}
	switch section {
	case "name":
		sections.Name = strings.TrimSpace(value)
	case "purpose":
		sections.Purpose = appendChannelToolsetProposalSectionLine(sections.Purpose, value)
	case "skills":
		sections.Skills = appendChannelToolsetProposalSectionLine(sections.Skills, value)
	case "instruction":
		sections.Instruction = appendChannelToolsetProposalSectionLine(sections.Instruction, value)
	case "policy":
		sections.Policy = appendChannelToolsetProposalSectionLine(sections.Policy, value)
	case "notes":
		sections.Notes = appendChannelToolsetProposalSectionLine(sections.Notes, value)
	}
}

func (sections *channelBundleProposalParsedSections) append(section, value string) {
	if sections == nil {
		return
	}
	value = strings.TrimRight(value, " \t\r")
	switch section {
	case "purpose":
		sections.Purpose = appendChannelToolsetProposalSectionLine(sections.Purpose, value)
	case "skills":
		sections.Skills = appendChannelToolsetProposalSectionLine(sections.Skills, value)
	case "instruction":
		sections.Instruction = appendChannelToolsetProposalSectionLine(sections.Instruction, value)
	case "policy":
		sections.Policy = appendChannelToolsetProposalSectionLine(sections.Policy, value)
	case "notes":
		sections.Notes = appendChannelToolsetProposalSectionLine(sections.Notes, value)
	}
}

func parseChannelBundleProposalSectionHeader(line string) (string, string, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	switch normalizeChannelBundleProposalHeader(name) {
	case "name":
		return "name", strings.TrimSpace(value), true
	case "purpose":
		return "purpose", strings.TrimSpace(value), true
	case "skills":
		return "skills", strings.TrimSpace(value), true
	case "instruction":
		return "instruction", strings.TrimSpace(value), true
	case "policy":
		return "policy", strings.TrimSpace(value), true
	case "notes":
		return "notes", strings.TrimSpace(value), true
	default:
		return "", "", false
	}
}

func normalizeChannelBundleProposalHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	header = strings.Join(strings.Fields(header), " ")
	switch header {
	case "name", "title", "bundle", "bundle name", "skill bundle", "summary":
		return "name"
	case "purpose", "why", "goal", "goals", "use case", "use cases", "description":
		return "purpose"
	case "skills", "skill", "skill refs", "skill references", "members":
		return "skills"
	case "instruction", "instructions", "bundle instruction", "guidance", "procedure", "workflow":
		return "instruction"
	case "policy", "gates", "guardrails", "approval", "approval policy", "risk":
		return "policy"
	case "notes", "context", "details":
		return "notes"
	default:
		return ""
	}
}

func normalizeChannelBundleProposalOptions(opts ChannelBundleProposalOptions) ChannelBundleProposalOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.BundleID = cleanChannelBundleProposalID(opts.BundleID)
	opts.Name = strings.TrimSpace(opts.Name)
	opts.Purpose = strings.TrimSpace(opts.Purpose)
	opts.Skills = strings.TrimSpace(opts.Skills)
	opts.Instruction = strings.TrimSpace(opts.Instruction)
	opts.Policy = strings.TrimSpace(opts.Policy)
	opts.Notes = strings.TrimSpace(opts.Notes)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelBundleProposalOptions(opts ChannelBundleProposalOptions) error {
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
	if opts.BundleID == "" {
		return fmt.Errorf("missing bundle proposal id")
	}
	if !skillNamePattern.MatchString(opts.BundleID) {
		return fmt.Errorf("invalid bundle proposal id %q", opts.BundleID)
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing source issue")
	}
	if opts.Name == "" {
		return fmt.Errorf("missing bundle proposal name")
	}
	if opts.Purpose == "" {
		return fmt.Errorf("missing bundle proposal purpose")
	}
	if len(channelBundleProposalSkillLines(opts.Skills)) == 0 {
		return fmt.Errorf("missing proposed bundle skills")
	}
	return nil
}

func findOrCreateChannelBundleProposalIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelBundleProposalOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel bundle proposal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelBundleProposalMatches(issue.Body, opts.BundleID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelBundleProposalIssueTitle(opts), RenderChannelBundleProposalIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel bundle proposal issue: %w", err)
	}
	return issue, true, false, nil
}

func channelBundleProposalIssueTitle(opts ChannelBundleProposalOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Name), "\n", " ")
	if title == "" {
		title = opts.BundleID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel bundle proposal: " + title
}

func channelBundleProposalMatches(body, bundleID string) bool {
	return HasChannelBundleProposalMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`bundle_id="%s"`, escapeMarkerValue(cleanChannelBundleProposalID(bundleID))))
}

func cleanChannelBundleProposalID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelBundleProposalID(ev Event, channel, threadID, sourceMessageID, name, skills, instruction, policy, notes string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, name, skills, instruction, policy, notes}, "|")
	return fmt.Sprintf("bundle-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelBundleProposalNotifyMessageID(ev Event, bundleID string) string {
	seed := strings.Join([]string{eventID(ev), bundleID}, "|")
	return fmt.Sprintf("gitclaw-channel-bundle-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelBundleProposalNotificationBody(opts ChannelBundleProposalOptions, proposalIssueNumber int, proposalIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill bundle proposal captured.\n\n")
	if proposalIssueNumber > 0 {
		fmt.Fprintf(&b, "Bundle proposal issue: #%d\n", proposalIssueNumber)
	}
	if proposalIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", proposalIssueURL)
	}
	fmt.Fprintf(&b, "Name: %s\n", strings.TrimSpace(opts.Name))
	fmt.Fprintf(&b, "Skills: %d\n", len(channelBundleProposalSkillLines(opts.Skills)))
	fmt.Fprintf(&b, "Review PR required: %t\n", true)
	fmt.Fprintf(&b, "Bundle enabled: %t\n", false)
	b.WriteString("\nContinue tracking it in the linked GitHub issue. This notification did not install skills, call a model, enable a bundle, write bundle YAML, or mutate the repository.")
	return strings.TrimSpace(b.String())
}

func channelBundleProposalSkillLines(skills string) []string {
	lines := strings.Split(strings.TrimSpace(skills), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		skill := cleanChannelToolsetProposalToolLine(line)
		if skill == "" {
			continue
		}
		cleaned = append(cleaned, skill)
	}
	return cleaned
}
