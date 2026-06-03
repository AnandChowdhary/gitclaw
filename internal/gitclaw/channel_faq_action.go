package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

type ChannelFAQOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	FAQID             string
	Question          string
	Answer            string
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelFAQResult struct {
	FAQIssueNumber int
	FAQIssueURL    string
	FAQCreated     bool
	FAQDuplicate   bool
	Notification   ChannelSendResult
	RouteName      string
	RouteHash      string
	Channel        string
	ThreadHash     string
	MessageHash    string
	NotifyHash     string
}

type ChannelFAQActionRequest struct {
	Options             ChannelFAQOptions
	Command             string
	Subcommand          string
	AutoFAQID           bool
	AutoNotifyMessageID bool
	TargetFromIssue     bool
	QuestionSHA         string
	QuestionBytes       int
	QuestionLines       int
	AnswerSHA           string
	AnswerBytes         int
	AnswerLines         int
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	NotificationBodySHA string
}

func IsChannelFAQActionRequest(ev Event, cfg Config) bool {
	return isChannelFAQActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelFAQActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "faq", "qna", "qa", "question", "answer", "faq-card", "capture-faq":
		return true
	default:
		return false
	}
}

func BuildChannelFAQActionRequest(ev Event, cfg Config) (ChannelFAQActionRequest, error) {
	fields, trailing, ok := channelFAQActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelFAQActionRequest{}, fmt.Errorf("missing channel faq command")
	}
	req := ChannelFAQActionRequest{
		Options: ChannelFAQOptions{
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
				return ChannelFAQActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelFAQActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelFAQActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelFAQActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelFAQActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--faq-id", "--question-id", "--answer-id", "--id":
			if i+1 >= len(fields) {
				return ChannelFAQActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.FAQID = cleanChannelFAQID(fields[i+1])
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelFAQActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelFAQActionRequest{}, fmt.Errorf("unknown channel faq argument %q", field)
			}
			if req.Options.FAQID == "" {
				req.Options.FAQID = cleanChannelFAQID(field)
				continue
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			return ChannelFAQActionRequest{}, fmt.Errorf("unexpected channel faq argument %q", field)
		}
	}
	if err := applyChannelFAQIssueTarget(ev, &req); err != nil {
		return ChannelFAQActionRequest{}, err
	}
	question, answer := parseChannelFAQQuestionAnswer(trailing, ev)
	req.Options.Question = question
	req.Options.Answer = answer
	if strings.TrimSpace(req.Options.FAQID) == "" {
		req.Options.FAQID = autoChannelFAQID(ev, req.Options.Channel, req.Options.ThreadID, req.Options.SourceMessageID, question, answer)
		req.AutoFAQID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelFAQNotifyMessageID(ev, req.Options.FAQID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelFAQOptions(req.Options)
	if err := validateChannelFAQActionRequestOptions(req.Options); err != nil {
		return ChannelFAQActionRequest{}, err
	}
	req.QuestionSHA = shortDocumentHash(req.Options.Question)
	req.QuestionBytes = len(req.Options.Question)
	req.QuestionLines = lineCount(req.Options.Question)
	req.AnswerSHA = shortDocumentHash(req.Options.Answer)
	req.AnswerBytes = len(req.Options.Answer)
	req.AnswerLines = lineCount(req.Options.Answer)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.NotificationBodySHA = shortDocumentHash(renderChannelFAQNotificationBody(req.Options, 0, issueURL(ev.Repo, 0)))
	return req, nil
}

func RunChannelFAQ(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelFAQOptions) (ChannelFAQResult, error) {
	opts = normalizeChannelFAQOptions(opts)
	var err error
	opts, err = applyChannelFAQRoute(cfg, opts)
	if err != nil {
		return ChannelFAQResult{}, err
	}
	if err := validateChannelFAQOptions(opts); err != nil {
		return ChannelFAQResult{}, err
	}
	faqIssue, created, duplicate, err := findOrCreateChannelFAQIssue(ctx, cfg, github, opts)
	if err != nil {
		return ChannelFAQResult{}, err
	}
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      renderChannelFAQNotificationBody(opts, faqIssue.Number, issueURL(opts.Repo, faqIssue.Number)),
	})
	if err != nil {
		return ChannelFAQResult{}, fmt.Errorf("queue channel faq notification: %w", err)
	}
	return ChannelFAQResult{
		FAQIssueNumber: faqIssue.Number,
		FAQIssueURL:    issueURL(opts.Repo, faqIssue.Number),
		FAQCreated:     created,
		FAQDuplicate:   duplicate,
		Notification:   notification,
		RouteName:      opts.Route,
		RouteHash:      channelRouteHash(opts.Route),
		Channel:        opts.Channel,
		ThreadHash:     shortDocumentHash(opts.ThreadID),
		MessageHash:    shortDocumentHash(opts.SourceMessageID),
		NotifyHash:     shortDocumentHash(opts.NotifyMessageID),
	}, nil
}

func RenderChannelFAQActionReport(ev Event, req ChannelFAQActionRequest, result ChannelFAQResult) string {
	status := "captured"
	switch {
	case result.FAQDuplicate && result.Notification.Duplicate:
		status = "duplicate"
	case result.FAQDuplicate:
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
	b.WriteString("## GitClaw Channel FAQ Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_faq_status: `%s`\n", status)
	fmt.Fprintf(&b, "- faq_issue: `#%d`\n", result.FAQIssueNumber)
	fmt.Fprintf(&b, "- faq_issue_url: `%s`\n", result.FAQIssueURL)
	fmt.Fprintf(&b, "- faq_issue_created: `%t`\n", result.FAQCreated)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.FAQDuplicate)
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
	fmt.Fprintf(&b, "- faq_id_sha256_12: `%s`\n", shortDocumentHash(req.Options.FAQID))
	fmt.Fprintf(&b, "- faq_id_auto: `%t`\n", req.AutoFAQID)
	fmt.Fprintf(&b, "- faq_question_sha256_12: `%s`\n", req.QuestionSHA)
	fmt.Fprintf(&b, "- faq_question_bytes: `%d`\n", req.QuestionBytes)
	fmt.Fprintf(&b, "- faq_question_lines: `%d`\n", req.QuestionLines)
	fmt.Fprintf(&b, "- faq_answer_sha256_12: `%s`\n", req.AnswerSHA)
	fmt.Fprintf(&b, "- faq_answer_bytes: `%d`\n", req.AnswerBytes)
	fmt.Fprintf(&b, "- faq_answer_lines: `%d`\n", req.AnswerLines)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", req.NotificationBodySHA)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- raw_faq_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_faq_question_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_faq_answer_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_faq_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw captured a channel-origin FAQ question as a durable GitHub issue, then queued a provider-facing FAQ link back to the mirrored thread. The FAQ issue contains the human-readable question and answer; this source receipt keeps provider IDs, FAQ IDs, questions, answers, and channel message bodies out of band.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read the FAQ-link notification with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent FAQ links with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate FAQ issues are suppressed by `faq_id`; duplicate FAQ-link notifications are suppressed by `channel + notify_message_id`\n")
	b.WriteString("- normal GitClaw conversation continues on the FAQ issue with the `gitclaw` label\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelFAQIssueBody(opts ChannelFAQOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- gitclaw:channel-faq faq_id=\"%s\" channel=\"%s\" thread_id_sha256_12=\"%s\" source_message_id_sha256_12=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" -->\n", escapeMarkerValue(opts.FAQID), escapeMarkerValue(opts.Channel), shortDocumentHash(opts.ThreadID), shortDocumentHash(opts.SourceMessageID), opts.SourceIssueNumber, opts.SourceCommentID)
	b.WriteString("GitClaw channel FAQ entry.\n\n")
	fmt.Fprintf(&b, "- faq_id: %s\n", opts.FAQID)
	fmt.Fprintf(&b, "- source_channel: %s\n", opts.Channel)
	fmt.Fprintf(&b, "- source_issue: #%d\n", opts.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(opts.Repo, opts.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", opts.SourceCommentID)
	fmt.Fprintf(&b, "- thread_id_sha256_12: %s\n", shortDocumentHash(opts.ThreadID))
	fmt.Fprintf(&b, "- source_message_id_sha256_12: %s\n", shortDocumentHash(opts.SourceMessageID))
	fmt.Fprintf(&b, "- faq_mode: github-issue-faq\n")
	fmt.Fprintf(&b, "- memory_mutation_performed: false\n")
	fmt.Fprintf(&b, "- repository_mutation_performed: false\n")
	fmt.Fprintf(&b, "- provider_delivery_performed: false\n")
	fmt.Fprintf(&b, "- raw_thread_id_included: false\n")
	fmt.Fprintf(&b, "- raw_source_message_id_included: false\n")
	fmt.Fprintf(&b, "- raw_channel_message_body_included: false\n\n")
	b.WriteString("## Question\n\n")
	b.WriteString(strings.TrimSpace(opts.Question))
	if strings.TrimSpace(opts.Answer) != "" {
		b.WriteString("\n\n## Answer\n\n")
		b.WriteString(strings.TrimSpace(opts.Answer))
	}
	b.WriteString("\n\nUse this issue as the reviewable GitHub home for preserving the channel FAQ entry without mutating memory or turning it into a task automatically.")
	return strings.TrimSpace(b.String())
}

func channelFAQActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelFAQActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelFAQIssueTarget(ev Event, req *ChannelFAQActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel faq requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func parseChannelFAQQuestionAnswer(trailing string, ev Event) (string, string) {
	lines := strings.Split(strings.TrimSpace(trailing), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" && len(cleaned) == 0 {
			continue
		}
		cleaned = append(cleaned, line)
	}
	defaultQuestion := fmt.Sprintf("Channel FAQ question from issue #%d", ev.Issue.Number)
	if len(cleaned) == 0 {
		return defaultQuestion, ""
	}
	first := strings.TrimSpace(cleaned[0])
	lowerFirst := strings.ToLower(first)
	var question string
	var answerLines []string
	switch {
	case strings.HasPrefix(lowerFirst, "question:"):
		question = strings.TrimSpace(first[len("question:"):])
		answerLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "faq:"):
		question = strings.TrimSpace(first[len("faq:"):])
		answerLines = cleaned[1:]
	case strings.HasPrefix(lowerFirst, "answer:"):
		question = defaultQuestion
		answerLines = append([]string{strings.TrimSpace(first[len("answer:"):])}, cleaned[1:]...)
	case strings.HasPrefix(lowerFirst, "context:"), strings.HasPrefix(lowerFirst, "notes:"), strings.HasPrefix(lowerFirst, "source:"):
		question = defaultQuestion
		answerLines = cleaned
	default:
		question = first
		answerLines = cleaned[1:]
	}
	if question == "" {
		question = defaultQuestion
	}
	answer := stripChannelFAQAnswerHeader(strings.TrimSpace(strings.Join(answerLines, "\n")))
	return question, answer
}

func stripChannelFAQAnswerHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"answer:", "context:", "notes:", "source:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

func normalizeChannelFAQOptions(opts ChannelFAQOptions) ChannelFAQOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.FAQID = cleanChannelFAQID(opts.FAQID)
	opts.Question = strings.TrimSpace(opts.Question)
	opts.Answer = strings.TrimSpace(opts.Answer)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelFAQRoute(cfg Config, opts ChannelFAQOptions) (ChannelFAQOptions, error) {
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
		Body:      opts.Question,
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

func validateChannelFAQOptions(opts ChannelFAQOptions) error {
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
	if opts.FAQID == "" {
		return fmt.Errorf("missing faq id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing faq source issue")
	}
	if opts.Question == "" {
		return fmt.Errorf("missing faq question")
	}
	return nil
}

func validateChannelFAQActionRequestOptions(opts ChannelFAQOptions) error {
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
	if opts.FAQID == "" {
		return fmt.Errorf("missing faq id")
	}
	if opts.SourceIssueNumber <= 0 {
		return fmt.Errorf("missing faq source issue")
	}
	if opts.Question == "" {
		return fmt.Errorf("missing faq question")
	}
	return nil
}

func findOrCreateChannelFAQIssue(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelFAQOptions) (Issue, bool, bool, error) {
	issues, err := github.ListOpenIssues(ctx, opts.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("list channel faq issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if channelFAQMatches(issue.Body, opts.FAQID) {
			return issue, false, true, nil
		}
	}
	issue, err := github.CreateIssue(ctx, opts.Repo, channelFAQIssueTitle(opts), RenderChannelFAQIssueBody(opts), []string{cfg.TriggerLabel})
	if err != nil {
		return Issue{}, false, false, fmt.Errorf("create channel faq issue: %w", err)
	}
	return issue, true, false, nil
}

func channelFAQIssueTitle(opts ChannelFAQOptions) string {
	title := strings.ReplaceAll(strings.TrimSpace(opts.Question), "\n", " ")
	if title == "" {
		title = opts.FAQID
	}
	if len(title) > 80 {
		title = strings.TrimSpace(title[:80])
	}
	return "GitClaw channel FAQ: " + title
}

func channelFAQMatches(body, faqID string) bool {
	return HasChannelFAQMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`faq_id="%s"`, escapeMarkerValue(cleanChannelFAQID(faqID))))
}

func cleanChannelFAQID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelFAQID(ev Event, channel, threadID, sourceMessageID, question, answer string) string {
	seed := strings.Join([]string{eventID(ev), channel, threadID, sourceMessageID, question, answer}, "|")
	return fmt.Sprintf("faq-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func autoChannelFAQNotifyMessageID(ev Event, faqID string) string {
	seed := strings.Join([]string{eventID(ev), faqID}, "|")
	return fmt.Sprintf("gitclaw-channel-faq-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func renderChannelFAQNotificationBody(opts ChannelFAQOptions, faqIssueNumber int, faqIssueURL string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel FAQ entry captured.\n\n")
	if faqIssueNumber > 0 {
		fmt.Fprintf(&b, "FAQ entry: #%d\n", faqIssueNumber)
	}
	if faqIssueURL != "" {
		fmt.Fprintf(&b, "URL: %s\n", faqIssueURL)
	}
	fmt.Fprintf(&b, "Question: %s\n", strings.TrimSpace(opts.Question))
	b.WriteString("\nContinue reviewing it in the linked GitHub issue.")
	return strings.TrimSpace(b.String())
}
