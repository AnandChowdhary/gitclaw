package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelRollcallOptions struct {
	Repo              string
	RollcallID        string
	SourceIssueNumber int
	SourceCommentID   int64
	Prompt            string
	Instructions      string
	Routes            []string
	MessageID         string
	Author            string
}

type ChannelRollcallResult struct {
	RollcallIssueNumber int
	RollcallIssueURL    string
	RollcallCreated     bool
	Broadcast           ChannelBroadcastResult
}

type ChannelRollcallActionRequest struct {
	Options               ChannelRollcallOptions
	Command               string
	Subcommand            string
	AutoRollcallID        bool
	AutoMessageID         bool
	PromptSHA             string
	PromptBytes           int
	PromptLines           int
	InstructionsSHA       string
	InstructionsBytes     int
	InstructionsLines     int
	RollcallSource        string
	RoutesSHA             string
	RouteCount            int
	OutboundBodySHA       string
	OutboundBodyBytes     int
	OutboundBodyLineCount int
}

func IsChannelRollcallActionRequest(ev Event, cfg Config) bool {
	return isChannelRollcallActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelRollcallActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rollcall", "roll-call", "checkin", "check-in", "standup", "attendance":
		return true
	default:
		return false
	}
}

func BuildChannelRollcallActionRequest(ev Event, cfg Config) (ChannelRollcallActionRequest, error) {
	fields, trailingBody, ok := channelRollcallActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelRollcallActionRequest{}, fmt.Errorf("missing channel rollcall command")
	}
	req := ChannelRollcallActionRequest{
		Options: ChannelRollcallOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:        fields[0],
		Subcommand:     strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RollcallSource: "trailing-lines",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelRollcallActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelRollcallActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--rollcall-id", "--id":
			if i+1 >= len(fields) {
				return ChannelRollcallActionRequest{}, fmt.Errorf("--rollcall-id requires a value")
			}
			req.Options.RollcallID = cleanChannelRollcallID(fields[i+1])
			i++
		case "--message-id":
			if i+1 >= len(fields) {
				return ChannelRollcallActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelRollcallActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelRollcallActionRequest{}, fmt.Errorf("unknown channel rollcall argument %q", field)
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
		}
	}

	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if len(req.Options.Routes) == 0 {
		return ChannelRollcallActionRequest{}, fmt.Errorf("missing rollcall routes")
	}
	prompt, instructions := parseChannelRollcallPromptInstructions(trailingBody, ev)
	req.Options.Prompt = prompt
	req.Options.Instructions = instructions
	if req.Options.RollcallID == "" {
		req.Options.RollcallID = autoChannelRollcallID(ev, req.Options.Routes, prompt, instructions)
		req.AutoRollcallID = true
	}
	if req.Options.MessageID == "" {
		req.Options.MessageID = autoChannelRollcallMessageID(ev, req.Options.RollcallID, req.Options.Routes)
		req.AutoMessageID = true
	}
	if err := validateChannelRollcallOptions(req.Options); err != nil {
		return ChannelRollcallActionRequest{}, err
	}
	outboundPreview := renderChannelRollcallOutboundBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.PromptSHA = shortDocumentHash(prompt)
	req.PromptBytes = len(prompt)
	req.PromptLines = lineCount(prompt)
	req.InstructionsSHA = shortDocumentHash(instructions)
	req.InstructionsBytes = len(instructions)
	req.InstructionsLines = lineCount(instructions)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	req.OutboundBodySHA = shortDocumentHash(outboundPreview)
	req.OutboundBodyBytes = len(outboundPreview)
	req.OutboundBodyLineCount = lineCount(outboundPreview)
	return req, nil
}

func RunChannelRollcall(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRollcallOptions) (ChannelRollcallResult, error) {
	opts = normalizeChannelRollcallOptions(opts)
	if err := validateChannelRollcallOptions(opts); err != nil {
		return ChannelRollcallResult{}, err
	}
	rollcall, created, err := findOrCreateChannelRollcallIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelRollcallResult{}, err
	}
	broadcastOpts := ChannelBroadcastOptions{
		Repo:      opts.Repo,
		Routes:    opts.Routes,
		MessageID: opts.MessageID,
		Author:    opts.Author,
		Body:      renderChannelRollcallOutboundBody(opts, rollcall.Number, issueURL(opts.Repo, rollcall.Number)),
	}
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, broadcastOpts)
	if err != nil {
		return ChannelRollcallResult{}, err
	}
	return ChannelRollcallResult{
		RollcallIssueNumber: rollcall.Number,
		RollcallIssueURL:    issueURL(opts.Repo, rollcall.Number),
		RollcallCreated:     created,
		Broadcast:           broadcast,
	}, nil
}

func RenderChannelRollcallActionReport(ev Event, req ChannelRollcallActionRequest, result ChannelRollcallResult) string {
	status := "queued"
	switch {
	case !result.RollcallCreated && result.Broadcast.Queued == 0 && result.Broadcast.Duplicates > 0:
		status = "duplicate"
	case result.Broadcast.Queued > 0 && result.Broadcast.Duplicates > 0:
		status = "partially-queued"
	}
	outboundBody := renderChannelRollcallOutboundBody(req.Options, result.RollcallIssueNumber, result.RollcallIssueURL)
	outboundBodySHA := shortDocumentHash(outboundBody)
	outboundBodyBytes := len(outboundBody)
	outboundBodyLines := lineCount(outboundBody)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Rollcall Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_rollcall_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rollcall_issue: `#%d`\n", result.RollcallIssueNumber)
	fmt.Fprintf(&b, "- rollcall_issue_url: `%s`\n", result.RollcallIssueURL)
	fmt.Fprintf(&b, "- rollcall_issue_created: `%t`\n", result.RollcallCreated)
	fmt.Fprintf(&b, "- rollcall_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.RollcallID))
	fmt.Fprintf(&b, "- rollcall_id_auto: `%t`\n", req.AutoRollcallID)
	fmt.Fprintf(&b, "- rollcall_prompt_sha256_12: `%s`\n", req.PromptSHA)
	fmt.Fprintf(&b, "- rollcall_prompt_bytes: `%d`\n", req.PromptBytes)
	fmt.Fprintf(&b, "- rollcall_prompt_lines: `%d`\n", req.PromptLines)
	fmt.Fprintf(&b, "- rollcall_instructions_sha256_12: `%s`\n", req.InstructionsSHA)
	fmt.Fprintf(&b, "- rollcall_instructions_bytes: `%d`\n", req.InstructionsBytes)
	fmt.Fprintf(&b, "- rollcall_instructions_lines: `%d`\n", req.InstructionsLines)
	fmt.Fprintf(&b, "- rollcall_source: `%s`\n", req.RollcallSource)
	fmt.Fprintf(&b, "- rollcall_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- rollcall_invites_queued: `%d`\n", result.Broadcast.Queued)
	fmt.Fprintf(&b, "- rollcall_invite_duplicates: `%d`\n", result.Broadcast.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Broadcast.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", outboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", outboundBodyBytes)
	fmt.Fprintf(&b, "- outbound_body_lines: `%d`\n", outboundBodyLines)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rollcall_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_instructions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_rollcall_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub rollcall issue, then queued one provider-facing check-in invitation per reviewed route. The rollcall issue contains the human-readable prompt and instructions; this source receipt keeps route names, rollcall IDs, prompt text, instructions, thread IDs, message IDs, and outbound bodies out of band.\n\n")
	b.WriteString("### Destinations\n")
	if len(result.Broadcast.Destinations) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, destination := range result.Broadcast.Destinations {
			fmt.Fprintf(
				&b,
				"- destination=`%02d` target_issue=`#%d` outbound_comment_id=`%d` target_issue_created=`%t` duplicate_suppressed=`%t` route_sha256_12=`%s` channel=`%s` thread_id_sha256_12=`%s` message_id_sha256_12=`%s` body_sha256_12=`%s`\n",
				destination.Index,
				destination.IssueNumber,
				destination.CommentID,
				destination.Created,
				destination.Duplicate,
				noneIfEmpty(destination.RouteHash),
				destination.Channel,
				noneIfEmpty(destination.ThreadHash),
				noneIfEmpty(destination.MessageHash),
				noneIfEmpty(destination.BodyHash),
			)
		}
	}
	b.WriteString("\n### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending rollcall invites with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent rollcall invites with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate rollcall invites are suppressed independently for each route by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}

func findOrCreateChannelRollcallIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelRollcallOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel rollcall issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelRollcallMatches(issue.Body, opts.RollcallID) {
			return issue, false, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelRollcallIssueTitle(opts), RenderChannelRollcallIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel rollcall issue: %w", err)
	}
	return issue, true, nil
}

func RenderChannelRollcallIssueBody(opts ChannelRollcallOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-rollcall rollcall_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" routes_sha256_12=\"%s\" instructions_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.RollcallID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(channelBroadcastRoutesHash(opts.Routes)), escapeMarkerValue(shortDocumentHash(opts.Instructions)))
	b.WriteString("GitClaw channel rollcall.\n\n")
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: `%s`\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- rollcall_id_sha256_12: `%s`\n", shortDocumentHash(opts.RollcallID))
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", channelBroadcastRoutesHash(opts.Routes))
	fmt.Fprintf(&b, "- route_count: `%d`\n", len(normalizeChannelBroadcastRoutes(opts.Routes)))
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n\n", false)
	b.WriteString("## Prompt\n\n")
	b.WriteString(strings.TrimSpace(opts.Prompt))
	b.WriteString("\n")
	if strings.TrimSpace(opts.Instructions) != "" {
		b.WriteString("\n## Instructions\n\n")
		b.WriteString(strings.TrimSpace(opts.Instructions))
		b.WriteString("\n")
	}
	b.WriteString("\nCheck in by commenting here, or by replying through a mirrored channel thread invited into this rollcall.")
	return strings.TrimSpace(b.String())
}

func channelRollcallActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelRollcallActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func renderChannelRollcallOutboundBody(opts ChannelRollcallOptions, issueNumber int, url string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel rollcall\n\n")
	if issueNumber > 0 {
		fmt.Fprintf(&b, "Rollcall: #%d\n", issueNumber)
	}
	fmt.Fprintf(&b, "URL: %s\n", url)
	fmt.Fprintf(&b, "Source: #%d\n", opts.SourceIssueNumber)
	b.WriteString("\nPrompt:\n")
	b.WriteString(strings.TrimSpace(opts.Prompt))
	b.WriteString("\n")
	if strings.TrimSpace(opts.Instructions) != "" {
		b.WriteString("\nInstructions:\n")
		b.WriteString(strings.TrimSpace(opts.Instructions))
		b.WriteString("\n")
	}
	b.WriteString("\nCheck in in the linked GitHub rollcall or continue through the mirrored channel thread.")
	return strings.TrimSpace(b.String())
}

func parseChannelRollcallPromptInstructions(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultPrompt := fmt.Sprintf("Check in for issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultPrompt, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var prompt string
	var instructionLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "prompt:"):
		prompt = strings.TrimSpace(first[len("prompt:"):])
		instructionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "instructions:"), strings.HasPrefix(lowerFirst, "format:"):
		prompt = defaultPrompt
		instructionLines = cleaned
	default:
		prompt = first
		instructionLines = cleaned[1:]
	}
	if prompt == "" {
		prompt = defaultPrompt
	}
	instructions := strings.TrimSpace(strings.Join(instructionLines, "\n"))
	instructionsLower := strings.ToLower(strings.TrimSpace(instructions))
	switch {
	case strings.HasPrefix(instructionsLower, "instructions:"):
		instructions = strings.TrimSpace(strings.TrimSpace(instructions)[len("instructions:"):])
	case strings.HasPrefix(instructionsLower, "format:"):
		instructions = strings.TrimSpace(strings.TrimSpace(instructions)[len("format:"):])
	}
	return prompt, instructions
}

func normalizeChannelRollcallOptions(opts ChannelRollcallOptions) ChannelRollcallOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.RollcallID = cleanChannelRollcallID(opts.RollcallID)
	opts.Prompt = strings.TrimSpace(opts.Prompt)
	opts.Instructions = strings.TrimSpace(opts.Instructions)
	opts.Routes = normalizeChannelBroadcastRoutes(opts.Routes)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func validateChannelRollcallOptions(opts ChannelRollcallOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.RollcallID == "" {
		return fmt.Errorf("missing rollcall id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing rollcall source issue")
	}
	if strings.TrimSpace(opts.Prompt) == "" {
		return fmt.Errorf("missing rollcall prompt")
	}
	if len(normalizeChannelBroadcastRoutes(opts.Routes)) == 0 {
		return fmt.Errorf("missing rollcall routes")
	}
	if strings.TrimSpace(opts.MessageID) == "" {
		return fmt.Errorf("missing rollcall message id")
	}
	return nil
}

func channelRollcallIssueTitle(opts ChannelRollcallOptions) string {
	prompt := strings.TrimSpace(opts.Prompt)
	if prompt == "" {
		prompt = fmt.Sprintf("issue #%d", opts.SourceIssueNumber)
	}
	prompt = strings.ReplaceAll(prompt, "\n", " ")
	if len(prompt) > 80 {
		prompt = strings.TrimSpace(prompt[:80])
	}
	return "GitClaw channel rollcall: " + prompt
}

func channelRollcallMatches(body, rollcallID string) bool {
	return HasChannelRollcallMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`rollcall_id="%s"`, escapeMarkerValue(cleanChannelRollcallID(rollcallID))))
}

func cleanChannelRollcallID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelRollcallID(ev Event, routes []string, prompt, instructions string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(normalizeChannelBroadcastRoutes(routes), ","), prompt, instructions}, "|")
	return fmt.Sprintf("rollcall-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelRollcallMessageID(ev Event, rollcallID string, routes []string) string {
	seed := strings.Join([]string{eventID(ev), rollcallID, strings.Join(normalizeChannelBroadcastRoutes(routes), ",")}, "|")
	return fmt.Sprintf("gitclaw-rollcall-%s-%s", eventID(ev), shortDocumentHash(seed))
}
