package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelDecisionOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	DecisionID        string
	Decision          string
	Rationale         string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelDecisionResult struct {
	DecisionIssueNumber int
	DecisionIssueURL    string
	DecisionCreated     bool
	DecisionDuplicate   bool
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
}

type ChannelDecisionActionRequest struct {
	Options             ChannelDecisionOptions
	Command             string
	Subcommand          string
	AutoDecisionID      bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	DecisionSHA         string
	DecisionBytes       int
	DecisionLines       int
	RationaleSHA        string
	RationaleBytes      int
	RationaleLines      int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelDecisionActionRequest(ev Event, cfg Config) bool {
	return isChannelDecisionActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelDecisionActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "decision", "decisions", "decide", "record-decision", "capture-decision":
		return true
	default:
		return false
	}
}

func BuildChannelDecisionActionRequest(ev Event, cfg Config) (ChannelDecisionActionRequest, error) {
	fields, trailing, ok := channelDecisionActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelDecisionActionRequest{}, fmt.Errorf("missing channel decision command")
	}
	req := ChannelDecisionActionRequest{
		Options: ChannelDecisionOptions{
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
				return ChannelDecisionActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelDecisionActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelDecisionActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelDecisionActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelDecisionActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--decision-id", "--decide-id", "--id":
			if i+1 >= len(fields) {
				return ChannelDecisionActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.DecisionID = cleanChannelDecisionID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelDecisionActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelDecisionActionRequest{}, fmt.Errorf("unknown channel decision argument %q", field)
			}
			if req.Options.DecisionID == "" {
				req.Options.DecisionID = cleanChannelDecisionID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelDecisionActionRequest{}, fmt.Errorf("unexpected channel decision argument %q", field)
		}
	}
	if err := applyChannelDecisionIssueTarget(ev, &req); err != nil {
		return ChannelDecisionActionRequest{}, err
	}
	decision, rationale := parseChannelDecisionText(trailing, ev)
	req.Options.Decision = decision
	req.Options.Rationale = rationale
	if strings.TrimSpace(req.Options.DecisionID) == "" {
		req.Options.DecisionID = autoChannelDecisionID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, decision, rationale)
		req.AutoDecisionID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelDecisionNotifyMessageID(ev, req.Options.DecisionID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelDecisionOptions(req.Options)
	if err := validateChannelDecisionActionRequestOptions(req.Options); err != nil {
		return ChannelDecisionActionRequest{}, err
	}
	req.DecisionSHA = shortDocumentHash(req.Options.Decision)
	req.DecisionBytes = len(req.Options.Decision)
	req.DecisionLines = lineCount(req.Options.Decision)
	req.RationaleSHA = shortDocumentHash(req.Options.Rationale)
	req.RationaleBytes = len(req.Options.Rationale)
	req.RationaleLines = lineCount(req.Options.Rationale)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelDecisionNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelDecision(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDecisionOptions) (ChannelDecisionResult, error) {
	opts = normalizeChannelDecisionOptions(opts)
	var err error
	opts, err = applyChannelDecisionRoute(cfg, opts)
	if err != nil {
		return ChannelDecisionResult{}, err
	}
	if err := validateChannelDecisionOptions(opts); err != nil {
		return ChannelDecisionResult{}, err
	}
	decisionIssue, created, duplicate, err := findOrCreateChannelDecisionIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelDecisionResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelDecisionNotificationBody(opts, decisionIssue.Number, issueURL(opts.Repo, decisionIssue.Number)),
	})
	if err != nil {
		return ChannelDecisionResult{}, fmt.Errorf("queue channel decision notification: %w", err)
	}
	return ChannelDecisionResult{
		DecisionIssueNumber: decisionIssue.Number,
		DecisionIssueURL:    issueURL(opts.Repo, decisionIssue.Number),
		DecisionCreated:     created,
		DecisionDuplicate:   duplicate,
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelDecisionActionReport(ev Event, req ChannelDecisionActionRequest, result ChannelDecisionResult) string {
	status := "recorded"
	switch {
	case result.DecisionDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.DecisionDuplicate:
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
	b.WriteString("## GitClaw Channel Decision Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_decision_status: `%s`\n", status)
	fmt.Fprintf(&b, "- decision_issue: `#%d`\n", result.DecisionIssueNumber)
	fmt.Fprintf(&b, "- decision_issue_url: `%s`\n", result.DecisionIssueURL)
	fmt.Fprintf(&b, "- decision_issue_created: `%t`\n", result.DecisionCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.DecisionDuplicate)
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
	fmt.Fprintf(&b, "- decision_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.DecisionID))
	fmt.Fprintf(&b, "- decision_id_auto: `%t`\n", req.AutoDecisionID)
	fmt.Fprintf(&b, "- decision_text_sha256_12: `%s`\n", req.DecisionSHA)
	fmt.Fprintf(&b, "- decision_text_bytes: `%d`\n", req.DecisionBytes)
	fmt.Fprintf(&b, "- decision_text_lines: `%d`\n", req.DecisionLines)
	fmt.Fprintf(&b, "- rationale_sha256_12: `%s`\n", req.RationaleSHA)
	fmt.Fprintf(&b, "- rationale_bytes: `%d`\n", req.RationaleBytes)
	fmt.Fprintf(&b, "- rationale_lines: `%d`\n", req.RationaleLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_decision_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_decision_text_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_rationale_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_decision_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw recorded a mirrored channel decision as a durable GitHub decision issue, then queued a provider-facing link back to the original thread. The decision issue contains the human-readable decision and rationale; this source receipt keeps provider IDs, decision IDs, decision text, rationale, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the decision-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent decision links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate decision issues are suppressed by `decision_id`; duplicate decision-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the decision issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelDecisionIssueBody(opts ChannelDecisionOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-decision decision_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.DecisionID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel decision.\n\n")
	fmt.Fprintf(&b, "- decision_id: %s\n", opts.DecisionID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- decision_mode: github-issue-decision\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Decision\n\n")
	b.WriteString(strings.TrimSpace(opts.Decision))
	if strings.TrimSpace(opts.Rationale) != "" {
		b.WriteString("\n\n## Rationale\n\n")
		b.WriteString(strings.TrimSpace(opts.Rationale))
	}
	b.WriteString("\n\nUse this issue as the durable GitHub home for the recorded channel decision.")
	return strings.TrimSpace(b.String())
}

func channelDecisionActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelDecisionActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelDecisionIssueTarget(ev Event, req *ChannelDecisionActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel decision requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelDecisionText(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultDecision := fmt.Sprintf("Decision from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultDecision, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var decision string
	var rationaleLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "decision:"):
		decision = strings.TrimSpace(first[len("decision:"):])
		rationaleLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "rationale:"), strings.HasPrefix(lowerFirst, "context:"):
		decision = defaultDecision
		rationaleLines = cleaned
	default:
		decision = first
		rationaleLines = cleaned[1:]
	}
	if decision == "" {
		decision = defaultDecision
	}
	rationale := strings.TrimSpace(strings.Join(rationaleLines, "\n"))
	rationaleLower := strings.ToLower(strings.TrimSpace(rationale))
	switch {
	case strings.HasPrefix(rationaleLower, "rationale:"):
		rationale = strings.TrimSpace(strings.TrimSpace(rationale)[len("rationale:"):])
	case strings.HasPrefix(rationaleLower, "context:"):
		rationale = strings.TrimSpace(strings.TrimSpace(rationale)[len("context:"):])
	}
	return decision, rationale
}

func normalizeChannelDecisionOptions(opts ChannelDecisionOptions) ChannelDecisionOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.DecisionID = cleanChannelDecisionID(opts.DecisionID)
	opts.Decision = strings.TrimSpace(opts.Decision)
	opts.Rationale = strings.TrimSpace(opts.Rationale)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelDecisionRoute(cfg Config, opts ChannelDecisionOptions) (ChannelDecisionOptions, error) {
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
		Body:      opts.Decision,
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

func validateChannelDecisionOptions(opts ChannelDecisionOptions) error {
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
	if opts.DecisionID == "" {
		return fmt.Errorf("missing decision id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing decision source issue")
	}
	if opts.Decision == "" {
		return fmt.Errorf("missing decision text")
	}
	return nil
}

func validateChannelDecisionActionRequestOptions(opts ChannelDecisionOptions) error {
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
	if opts.DecisionID == "" {
		return fmt.Errorf("missing decision id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing decision source issue")
	}
	if opts.Decision == "" {
		return fmt.Errorf("missing decision text")
	}
	return nil
}

func findOrCreateChannelDecisionIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelDecisionOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel decision issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelDecisionMatches(issue.Body, opts.DecisionID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelDecisionIssueTitle(opts), RenderChannelDecisionIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel decision issue: %w", err)
	}
	return issue, true, false, nil
}

func channelDecisionIssueTitle(opts ChannelDecisionOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Decision), "\n", " ")
	if title == "" {
		title = opts.DecisionID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel decision: " + title
}

func channelDecisionMatches(body, decisionID string) bool {
	return HasChannelDecisionMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`decision_id="%s"`, escapeMarkerValue(cleanChannelDecisionID(decisionID))))
}

func cleanChannelDecisionID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelDecisionID(ev Event, channel, threadID, sourceMessageID, decision, rationale string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, decision, rationale}, "|")
	return fmt.Sprintf("decision-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelDecisionNotifyMessageID(ev Event, decisionID string) string {
	seed := strings.Join([]string{eventID(ev), decisionID}, "|")
	return fmt.Sprintf("gitclaw-channel-decision-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelDecisionNotificationBody(opts ChannelDecisionOptions, decisionIssueNumber int, decisionIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel decision recorded.\n\n")
	if decisionIssueNumber > 0 {
		fmt.Fprintf(&b, "Decision: #%d\n", decisionIssueNumber)
	}
	if decisionIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", decisionIssueURL)
	}
	fmt.Fprintf(&b, "Summary: %s\n", strings.TrimSpace(opts.Decision))
	b.WriteString("\nRecorded in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
