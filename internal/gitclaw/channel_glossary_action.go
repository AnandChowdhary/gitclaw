package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelGlossaryOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	GlossaryID        string
	Term              string
	Definition        string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelGlossaryResult struct {
	GlossaryIssueNumber int
	GlossaryIssueURL    string
	GlossaryCreated     bool
	GlossaryDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelGlossaryActionRequest struct {
	Options             ChannelGlossaryOptions
	Command             string
	Subcommand          string
	AutoGlossaryID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	TermSHA             string
	TermBytes           int
	TermLines           int
	DefinitionSHA       string
	DefinitionBytes     int
	DefinitionLines     int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelGlossaryActionRequest(ev Event, cfg Config) bool {
	return isChannelGlossaryActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelGlossaryActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "glossary", "term", "define", "definition", "term-card", "capture-term":
		return true
	default:
		return false
	}
}

func BuildChannelGlossaryActionRequest(ev Event, cfg Config) (ChannelGlossaryActionRequest, error) {
	fields, trailing, ok := channelGlossaryActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelGlossaryActionRequest{}, fmt.Errorf("missing channel glossary command")
	}
	req := ChannelGlossaryActionRequest{
		Options: ChannelGlossaryOptions{
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
				return ChannelGlossaryActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--glossary-id", "--term-id", "--definition-id", "--id":
			if i+1 >= len(fields) {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.GlossaryID = cleanChannelGlossaryID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelGlossaryActionRequest{}, fmt.Errorf("unknown channel glossary argument %q", field)
			}
			if req.Options.GlossaryID == "" {
				req.Options.GlossaryID = cleanChannelGlossaryID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelGlossaryActionRequest{}, fmt.Errorf("unexpected channel glossary argument %q", field)
		}
	}
	if err := applyChannelGlossaryIssueTarget(ev, &req); err != nil {
		return ChannelGlossaryActionRequest{}, err
	}
	term, definition := parseChannelGlossaryTermDefinition(trailing, ev)
	req.Options.Term = term
	req.Options.Definition = definition
	if strings.TrimSpace(req.Options.GlossaryID) == "" {
		req.Options.GlossaryID = autoChannelGlossaryID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, term, definition)
		req.AutoGlossaryID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelGlossaryNotifyMessageID(ev, req.Options.GlossaryID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelGlossaryOptions(req.Options)
	if err := validateChannelGlossaryActionRequestOptions(req.Options); err != nil {
		return ChannelGlossaryActionRequest{}, err
	}
	req.TermSHA = shortDocumentHash(req.Options.Term)
	req.TermBytes = len(req.Options.Term)
	req.TermLines = lineCount(req.Options.Term)
	req.DefinitionSHA = shortDocumentHash(req.Options.Definition)
	req.DefinitionBytes = len(req.Options.Definition)
	req.DefinitionLines = lineCount(req.Options.Definition)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelGlossaryNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelGlossary(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelGlossaryOptions) (ChannelGlossaryResult, error) {
	opts = normalizeChannelGlossaryOptions(opts)
	var err error
	opts, err = applyChannelGlossaryRoute(cfg, opts)
	if err != nil {
		return ChannelGlossaryResult{}, err
	}
	if err := validateChannelGlossaryOptions(opts); err != nil {
		return ChannelGlossaryResult{}, err
	}
	glossaryIssue, created, duplicate, err := findOrCreateChannelGlossaryIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelGlossaryResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelGlossaryNotificationBody(opts, glossaryIssue.Number, issueURL(opts.Repo, glossaryIssue.Number)),
	})
	if err != nil {
		return ChannelGlossaryResult{}, fmt.Errorf("queue channel glossary notification: %w", err)
	}
	return ChannelGlossaryResult{
		GlossaryIssueNumber: glossaryIssue.Number,
		GlossaryIssueURL:    issueURL(opts.Repo, glossaryIssue.Number),
		GlossaryCreated:     created,
		GlossaryDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelGlossaryActionReport(ev Event, req ChannelGlossaryActionRequest, result ChannelGlossaryResult) string {
	status := "captured"
	switch {
	case result.GlossaryDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.GlossaryDuplicate:
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
	b.WriteString("## GitClaw Channel Glossary Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_glossary_status: `%s`\n", status)
	fmt.Fprintf(&b, "- glossary_issue: `#%d`\n", result.GlossaryIssueNumber)
	fmt.Fprintf(&b, "- glossary_issue_url: `%s`\n", result.GlossaryIssueURL)
	fmt.Fprintf(&b, "- glossary_issue_created: `%t`\n", result.GlossaryCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.GlossaryDuplicate)
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
	fmt.Fprintf(&b, "- glossary_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.GlossaryID))
	fmt.Fprintf(&b, "- glossary_id_auto: `%t`\n", req.AutoGlossaryID)
	fmt.Fprintf(&b, "- glossary_term_sha256_12: `%s`\n", req.TermSHA)
	fmt.Fprintf(&b, "- glossary_term_bytes: `%d`\n", req.TermBytes)
	fmt.Fprintf(&b, "- glossary_term_lines: `%d`\n", req.TermLines)
	fmt.Fprintf(&b, "- glossary_definition_sha256_12: `%s`\n", req.DefinitionSHA)
	fmt.Fprintf(&b, "- glossary_definition_bytes: `%d`\n", req.DefinitionBytes)
	fmt.Fprintf(&b, "- glossary_definition_lines: `%d`\n", req.DefinitionLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_glossary_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_glossary_term_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_glossary_definition_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_glossary_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin glossary term as a durable GitHub issue, then queued a provider-facing glossary link back to the mirrored thread. The glossary issue contains the human-readable term and definition; this source receipt keeps provider IDs, glossary IDs, terms, definitions, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the glossary-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent glossary links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate glossary issues are suppressed by `glossary_id`; duplicate glossary-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the glossary issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelGlossaryIssueBody(opts ChannelGlossaryOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-glossary glossary_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.GlossaryID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel glossary entry.\n\n")
	fmt.Fprintf(&b, "- glossary_id: %s\n", opts.GlossaryID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- glossary_mode: github-issue-glossary\n")
	fmt.Fprintf(&b, "- memory_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Term\n\n")
	b.WriteString(strings.TrimSpace(opts.Term))
	if strings.TrimSpace(opts.Definition) != "" {
		b.WriteString("\n\n## Definition\n\n")
		b.WriteString(strings.TrimSpace(opts.Definition))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the channel glossary entry without mutating memory or turning it into a task automatically.")
	return strings.TrimSpace(b.String())
}

func channelGlossaryActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelGlossaryActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelGlossaryIssueTarget(ev Event, req *ChannelGlossaryActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel glossary requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelGlossaryTermDefinition(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultTerm := fmt.Sprintf("Channel glossary term from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultTerm, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var term string
	var definitionLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "term:"):
		term = strings.TrimSpace(first[len("term:"):])
		definitionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "glossary:"):
		term = strings.TrimSpace(first[len("glossary:"):])
		definitionLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "definition:"):
		term = defaultTerm
		definitionLines = append([]string{strings.TrimSpace(first[len("definition:"):])}, cleaned[1:]...)
	case strings.HasPrefix(lowerFirst, "context:"), strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "source:"):
		term = defaultTerm
		definitionLines = cleaned
	default:
		term = first
		definitionLines = cleaned[1:]
	}
	if term == "" {
		term = defaultTerm
	}
	definition := stripChannelGlossaryDefinitionHeader(strings.TrimSpace(strings.Join(definitionLines, "\n")))
	return term, definition
}

func stripChannelGlossaryDefinitionHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"definition:", "context:", "notes:", "source:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

func normalizeChannelGlossaryOptions(opts ChannelGlossaryOptions) ChannelGlossaryOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.GlossaryID = cleanChannelGlossaryID(opts.GlossaryID)
	opts.Term = strings.TrimSpace(opts.Term)
	opts.Definition = strings.TrimSpace(opts.Definition)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelGlossaryRoute(cfg Config, opts ChannelGlossaryOptions) (ChannelGlossaryOptions, error) {
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
		Body:      opts.Term,
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

func validateChannelGlossaryOptions(opts ChannelGlossaryOptions) error {
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
	if opts.GlossaryID == "" {
		return fmt.Errorf("missing glossary id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing glossary source issue")
	}
	if opts.Term == "" {
		return fmt.Errorf("missing glossary term")
	}
	return nil
}

func validateChannelGlossaryActionRequestOptions(opts ChannelGlossaryOptions) error {
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
	if opts.GlossaryID == "" {
		return fmt.Errorf("missing glossary id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing glossary source issue")
	}
	if opts.Term == "" {
		return fmt.Errorf("missing glossary term")
	}
	return nil
}

func findOrCreateChannelGlossaryIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelGlossaryOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel glossary issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelGlossaryMatches(issue.Body, opts.GlossaryID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelGlossaryIssueTitle(opts), RenderChannelGlossaryIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel glossary issue: %w", err)
	}
	return issue, true, false, nil
}

func channelGlossaryIssueTitle(opts ChannelGlossaryOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Term), "\n", " ")
	if title == "" {
		title = opts.GlossaryID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel glossary: " + title
}

func channelGlossaryMatches(body, glossaryID string) bool {
	return HasChannelGlossaryMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`glossary_id="%s"`, escapeMarkerValue(cleanChannelGlossaryID(glossaryID))))
}

func cleanChannelGlossaryID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelGlossaryID(ev Event, channel, threadID, sourceMessageID, term, definition string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, term, definition}, "|")
	return fmt.Sprintf("glossary-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelGlossaryNotifyMessageID(ev Event, glossaryID string) string {
	seed := strings.Join([]string{eventID(ev), glossaryID}, "|")
	return fmt.Sprintf("gitclaw-channel-glossary-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelGlossaryNotificationBody(opts ChannelGlossaryOptions, glossaryIssueNumber int, glossaryIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel glossary entry captured.\n\n")
	if glossaryIssueNumber > 0 {
		fmt.Fprintf(&b, "Glossary entry: #%d\n", glossaryIssueNumber)
	}
	if glossaryIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", glossaryIssueURL)
	}
	fmt.Fprintf(&b, "Term: %s\n", strings.TrimSpace(opts.Term))
	b.WriteString("\nContinue reviewing it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
