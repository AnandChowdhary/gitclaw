package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelPollOptions struct {
	Repo              string
	PollID            string
	SourceIssueNumber int
	SourceCommentID   int64
	Question          string
	Options           []string
	Routes            []string
	MessageID         string
	Author            string
}

type ChannelPollResult struct {
	PollIssueNumber int
	PollIssueURL    string
	PollCreated     bool
	Broadcast       ChannelBroadcastResult
}

type ChannelPollActionRequest struct {
	Options           ChannelPollOptions
	Command           string
	Subcommand        string
	AutoPollID        bool
	AutoMessageID     bool
	QuestionSHA       string
	QuestionBytes     int
	QuestionLines     int
	OptionsSHA        string
	OptionCount       int
	PollSource        string
	RoutesSHA         string
	RouteCount        int
	OutboundBodySHA   string
	OutboundBodyBytes int
	OutboundBodyLines int
}

func IsChannelPollActionRequest(ev Event, cfg Config) bool {
	return isChannelPollActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelPollActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "poll", "vote", "ballot", "ask":
		return true
	default:
		return false
	}
}

func BuildChannelPollActionRequest(ev Event, cfg Config) (ChannelPollActionRequest, error) {
	fields, trailingBody, ok := channelPollActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelPollActionRequest{}, fmt.Errorf("missing channel poll command")
	}
	req := ChannelPollActionRequest{
		Options: ChannelPollOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		PollSource: "trailing-lines",
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var inlineOptions []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelPollActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--routes":
			if i+1 >= len(fields) {
				return ChannelPollActionRequest{}, fmt.Errorf("--routes requires a value")
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(fields[i+1])...)
			i++
		case "--poll-id", "--id":
			if i+1 >= len(fields) {
				return ChannelPollActionRequest{}, fmt.Errorf("--poll-id requires a value")
			}
			req.Options.PollID = cleanChannelPollID(fields[i+1])
			i++
		case "--message-id":
			if i+1 >= len(fields) {
				return ChannelPollActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelPollActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--option", "--choice":
			if i+1 >= len(fields) {
				return ChannelPollActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			inlineOptions = append(inlineOptions, fields[i+1])
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelPollActionRequest{}, fmt.Errorf("unknown channel poll argument %q", field)
			}
			req.Options.Routes = append(req.Options.Routes, splitChannelBroadcastRoutes(field)...)
		}
	}

	req.Options.Routes = normalizeChannelBroadcastRoutes(req.Options.Routes)
	if len(req.Options.Routes) == 0 {
		return ChannelPollActionRequest{}, fmt.Errorf("missing poll routes")
	}
	question, options := parseChannelPollQuestionOptions(trailingBody, ev, inlineOptions)
	req.Options.Question = question
	req.Options.Options = options
	if req.Options.PollID == "" {
		req.Options.PollID = autoChannelPollID(ev, req.Options.Routes, question, options)
		req.AutoPollID = true
	}
	if req.Options.MessageID == "" {
		req.Options.MessageID = autoChannelPollMessageID(ev, req.Options.PollID, req.Options.Routes)
		req.AutoMessageID = true
	}
	if err := validateChannelPollOptions(req.Options); err != nil {
		return ChannelPollActionRequest{}, err
	}
	outboundPreview := renderChannelPollOutboundBody(req.Options, 0, issueURL(ev.Repo, 0))
	req.QuestionSHA = shortDocumentHash(question)
	req.QuestionBytes = len(question)
	req.QuestionLines = lineCount(question)
	req.OptionsSHA = shortDocumentHash(strings.Join(options, "\n"))
	req.OptionCount = len(options)
	req.RoutesSHA = channelBroadcastRoutesHash(req.Options.Routes)
	req.RouteCount = len(req.Options.Routes)
	req.OutboundBodySHA = shortDocumentHash(outboundPreview)
	req.OutboundBodyBytes = len(outboundPreview)
	req.OutboundBodyLines = lineCount(outboundPreview)
	return req, nil
}

func RunChannelPoll(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPollOptions) (ChannelPollResult, error) {
	opts = normalizeChannelPollOptions(opts)
	if err := validateChannelPollOptions(opts); err != nil {
		return ChannelPollResult{}, err
	}
	poll, created, err := findOrCreateChannelPollIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelPollResult{}, err
	}
	broadcastOpts := ChannelBroadcastOptions{
		Repo:      opts.Repo,
		Routes:    opts.Routes,
		MessageID: opts.MessageID,
		Author:    opts.Author,
		Body:      renderChannelPollOutboundBody(opts, poll.Number, issueURL(opts.Repo, poll.Number)),
	}
	broadcast, err := RunChannelBroadcast(ctx, cfg, github, broadcastOpts)
	if err != nil {
		return ChannelPollResult{}, err
	}
	return ChannelPollResult{
		PollIssueNumber: poll.Number,
		PollIssueURL:    issueURL(opts.Repo, poll.Number),
		PollCreated:     created,
		Broadcast:       broadcast,
	}, nil
}

func RenderChannelPollActionReport(ev Event, req ChannelPollActionRequest, result ChannelPollResult) string {
	status := "queued"
	switch {
	case !result.PollCreated && result.Broadcast.Queued == 0 && result.Broadcast.Duplicates > 0:
		status = "duplicate"
	case result.Broadcast.Queued > 0 && result.Broadcast.Duplicates > 0:
		status = "partially-queued"
	}
	outboundBody := renderChannelPollOutboundBody(req.Options, result.PollIssueNumber, result.PollIssueURL)
	outboundBodySHA := shortDocumentHash(outboundBody)
	outboundBodyBytes := len(outboundBody)
	outboundBodyLines := lineCount(outboundBody)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Poll Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_poll_status: `%s`\n", status)
	fmt.Fprintf(&b, "- poll_issue: `#%d`\n", result.PollIssueNumber)
	fmt.Fprintf(&b, "- poll_issue_url: `%s`\n", result.PollIssueURL)
	fmt.Fprintf(&b, "- poll_issue_created: `%t`\n", result.PollCreated)
	fmt.Fprintf(&b, "- poll_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.PollID))
	fmt.Fprintf(&b, "- poll_id_auto: `%t`\n", req.AutoPollID)
	fmt.Fprintf(&b, "- poll_question_sha256_12: `%s`\n", req.QuestionSHA)
	fmt.Fprintf(&b, "- poll_question_bytes: `%d`\n", req.QuestionBytes)
	fmt.Fprintf(&b, "- poll_question_lines: `%d`\n", req.QuestionLines)
	fmt.Fprintf(&b, "- poll_options_sha256_12: `%s`\n", req.OptionsSHA)
	fmt.Fprintf(&b, "- poll_options: `%d`\n", req.OptionCount)
	fmt.Fprintf(&b, "- poll_source: `%s`\n", req.PollSource)
	fmt.Fprintf(&b, "- poll_routes: `%d`\n", req.RouteCount)
	fmt.Fprintf(&b, "- poll_invites_queued: `%d`\n", result.Broadcast.Queued)
	fmt.Fprintf(&b, "- poll_invite_duplicates: `%d`\n", result.Broadcast.Duplicates)
	fmt.Fprintf(&b, "- target_issues_created: `%d`\n", result.Broadcast.Created)
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", req.RoutesSHA)
	fmt.Fprintf(&b, "- message_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.MessageID))
	fmt.Fprintf(&b, "- message_id_auto: `%t`\n", req.AutoMessageID)
	fmt.Fprintf(&b, "- outbound_body_sha256_12: `%s`\n", outboundBodySHA)
	fmt.Fprintf(&b, "- outbound_body_bytes: `%d`\n", outboundBodyBytes)
	fmt.Fprintf(&b, "- outbound_body_lines: `%d`\n", outboundBodyLines)
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_poll_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_question_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_options_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outbound_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_poll_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw created or reused a GitHub poll issue, then queued one provider-facing poll invitation per reviewed route. The poll issue contains the human-readable question and options; this source receipt keeps route names, poll IDs, question text, option text, thread IDs, message IDs, and outbound bodies out of band.\n\n")
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
	b.WriteString("- provider gateways read pending poll invites with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent poll invites with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate poll invites are suppressed independently for each route by `channel + message_id`\n")
	return strings.TrimSpace(b.String())
}

func findOrCreateChannelPollIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelPollOptions) (Issue, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, fmt.Errorf("list channel poll issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelPollMatches(issue.Body, opts.PollID) {
			return issue, false, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelPollIssueTitle(opts), RenderChannelPollIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, fmt.Errorf("create channel poll issue: %w", err)
	}
	return issue, true, nil
}

func RenderChannelPollIssueBody(opts ChannelPollOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-poll poll_id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" routes_sha256_12=\"%s\" options_sha256_12=\"%s\" -->\n", escapeMarkerValue(opts.PollID), opts.SourceIssueNumber, opts.SourceCommentID, escapeMarkerValue(channelBroadcastRoutesHash(opts.Routes)), escapeMarkerValue(shortDocumentHash(strings.Join(normalizeChannelPollChoices(opts.Options), "\n"))))
	b.WriteString("GitClaw channel poll.\n\n")
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: `%s`\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- poll_id_sha256_12: `%s`\n", shortDocumentHash(opts.PollID))
	fmt.Fprintf(&b, "- routes_sha256_12: `%s`\n", channelBroadcastRoutesHash(opts.Routes))
	fmt.Fprintf(&b, "- route_count: `%d`\n", len(normalizeChannelBroadcastRoutes(opts.Routes)))
	fmt.Fprintf(&b, "- option_count: `%d`\n", len(normalizeChannelPollChoices(opts.Options)))
	fmt.Fprintf(&b, "- raw_route_names_included: `%t`\n\n", false)
	b.WriteString("## Question\n\n")
	b.WriteString(strings.TrimSpace(opts.Question))
	b.WriteString("\n\n## Options\n\n")
	for i, option := range normalizeChannelPollChoices(opts.Options) {
		fmt.Fprintf(&b, "%d. %s\n", i+1, option)
	}
	b.WriteString("\nVote by commenting with the option number or option text. Participants can continue here from GitHub, or through a mirrored channel thread invited into this poll.")
	return strings.TrimSpace(b.String())
}

func channelPollActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelPollActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func renderChannelPollOutboundBody(opts ChannelPollOptions, issueNumber int, url string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel poll\n\n")
	if issueNumber > 0 {
		fmt.Fprintf(&b, "Poll: #%d\n", issueNumber)
	}
	fmt.Fprintf(&b, "URL: %s\n", url)
	fmt.Fprintf(&b, "Source: #%d\n", opts.SourceIssueNumber)
	b.WriteString("\nQuestion:\n")
	b.WriteString(strings.TrimSpace(opts.Question))
	b.WriteString("\n\nOptions:\n")
	for i, option := range normalizeChannelPollChoices(opts.Options) {
		fmt.Fprintf(&b, "%d. %s\n", i+1, option)
	}
	b.WriteString("\nVote in the linked GitHub poll or continue through the mirrored channel thread.")
	return strings.TrimSpace(b.String())
}

func parseChannelPollQuestionOptions(trailing string, ev Event, inlineOptions []string) (string, []string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	defaultQuestion := fmt.Sprintf("Poll for issue #%d", ev.Issue.Number)
	var questionLines []string
	var options []string
	inOptions := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "question:"):
			inOptions = false
			value := strings.TrimSpace(trimmed[len("question:"):])
			if value != "" {
				questionLines = append(questionLines, value)
			}
		case strings.HasPrefix(lower, "options:"), strings.HasPrefix(lower, "choices:"):
			inOptions = true
		case inOptions:
			options = append(options, cleanChannelPollChoice(trimPollOptionBullet(trimmed)))
		case len(questionLines) == 0:
			questionLines = append(questionLines, strings.TrimPrefix(trimmed, "Question:"))
		default:
			questionLines = append(questionLines, trimmed)
		}
	}
	for _, option := range inlineOptions {
		options = append(options, cleanChannelPollChoice(option))
	}
	question := strings.TrimSpace(strings.Join(questionLines, "\n"))
	if question == "" {
		question = defaultQuestion
	}
	return question, normalizeChannelPollChoices(options)
}

func trimPollOptionBullet(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "-")
	value = strings.TrimPrefix(value, "*")
	value = strings.TrimSpace(value)
	for i, r := range value {
		if r < '0' || r > '9' {
			if i > 0 && (r == '.' || r == ')') {
				return strings.TrimSpace(value[i+1:])
			}
			break
		}
	}
	return value
}

func normalizeChannelPollOptions(opts ChannelPollOptions) ChannelPollOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.PollID = cleanChannelPollID(opts.PollID)
	opts.Question = strings.TrimSpace(opts.Question)
	opts.Options = normalizeChannelPollChoices(opts.Options)
	opts.Routes = normalizeChannelBroadcastRoutes(opts.Routes)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func normalizeChannelPollChoices(options []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(options))
	for _, option := range options {
		option = cleanChannelPollChoice(option)
		if option == "" {
			continue
		}
		key := strings.ToLower(option)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, option)
	}
	return normalized
}

func cleanChannelPollChoice(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n`\"'")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 160 {
		value = strings.TrimSpace(value[:160])
	}
	return value
}

func validateChannelPollOptions(opts ChannelPollOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.PollID == "" {
		return fmt.Errorf("missing poll id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing poll source issue")
	}
	if strings.TrimSpace(opts.Question) == "" {
		return fmt.Errorf("missing poll question")
	}
	if len(normalizeChannelBroadcastRoutes(opts.Routes)) == 0 {
		return fmt.Errorf("missing poll routes")
	}
	if strings.TrimSpace(opts.MessageID) == "" {
		return fmt.Errorf("missing poll message id")
	}
	choices := normalizeChannelPollChoices(opts.Options)
	if len(choices) < 2 {
		return fmt.Errorf("channel poll requires at least two options")
	}
	if len(choices) > 10 {
		return fmt.Errorf("channel poll supports at most ten options")
	}
	return nil
}

func channelPollIssueTitle(opts ChannelPollOptions) string {
	question := strings.TrimSpace(opts.Question)
	if question == "" {
		question = fmt.Sprintf("issue #%d", opts.SourceIssueNumber)
	}
	question = strings.ReplaceAll(question, "\n", " ")
	if len(question) > 80 {
		question = strings.TrimSpace(question[:80])
	}
	return "GitClaw channel poll: " + question
}

func channelPollMatches(body, pollID string) bool {
	return HasChannelPollMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`poll_id="%s"`, escapeMarkerValue(cleanChannelPollID(pollID))))
}

func cleanChannelPollID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelPollID(ev Event, routes []string, question string, options []string) string {
	seed := strings.Join([]string{eventID(ev), strings.Join(normalizeChannelBroadcastRoutes(routes), ","), question, strings.Join(normalizeChannelPollChoices(options), "\n")}, "|")
	return fmt.Sprintf("poll-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelPollMessageID(ev Event, pollID string, routes []string) string {
	seed := strings.Join([]string{eventID(ev), pollID, strings.Join(normalizeChannelBroadcastRoutes(routes), ",")}, "|")
	return fmt.Sprintf("gitclaw-poll-%s-%s", eventID(ev), shortDocumentHash(seed))
}
