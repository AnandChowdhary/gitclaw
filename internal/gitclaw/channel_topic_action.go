package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelTopicOptions struct {
	Repo     string
	Route    string
	Channel  string
	ThreadID string
	TopicID  string
	Author   string
	Topic    string
}

type ChannelTopicResult struct {
	IssueNumber int
	IssueURL    string
	CommentID   int64
	Created     bool
	Duplicate   bool
	RouteName   string
	RouteHash   string
	Channel     string
	ThreadHash  string
	TopicIDHash string
	TopicHash   string
}

type ChannelTopicActionRequest struct {
	Options             ChannelTopicOptions
	Command             string
	Subcommand          string
	AutoTopicID         bool
	TargetFromIssue     bool
	TopicSHA            string
	TopicBytes          int
	TopicLines          int
	TopicSource         string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedTopicIDSHA string
}

func IsChannelTopicActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelTopicActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelTopicActionFields(fields)
}

func isChannelTopicActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "topic", "thread-topic", "thread-title", "title", "rename", "subject":
		return true
	default:
		return false
	}
}

func BuildChannelTopicActionRequest(ev Event, cfg Config) (ChannelTopicActionRequest, error) {
	fields, trailingBody, ok := channelTopicActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelTopicActionRequest{}, fmt.Errorf("missing channel topic command")
	}
	req := ChannelTopicActionRequest{
		Options: ChannelTopicOptions{
			Repo: ev.Repo,
		},
		Command:     fields[0],
		Subcommand:  strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		TopicSource: "inline",
	}
	var topicParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelTopicActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelTopicActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelTopicActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--topic-id", "--title-id", "--id":
			if i+1 >= len(fields) {
				return ChannelTopicActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TopicID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelTopicActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--topic", "--title", "--subject", "--body":
			if i+1 >= len(fields) {
				return ChannelTopicActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			topicParts = append(topicParts, fields[i+1:]...)
			i = len(fields)
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelTopicActionRequest{}, fmt.Errorf("unknown channel topic argument %q", field)
			}
			topicParts = append(topicParts, fields[i:]...)
			i = len(fields)
		}
	}
	topic := strings.TrimSpace(strings.Join(topicParts, " "))
	trailingBody = strings.TrimSpace(trailingBody)
	if trailingBody != "" {
		if topic != "" {
			topic += "\n" + trailingBody
		} else {
			topic = trailingBody
			req.TopicSource = "trailing-lines"
		}
	}
	req.Options.Topic = topic
	if err := applyChannelTopicIssueTarget(ev, &req); err != nil {
		return ChannelTopicActionRequest{}, err
	}
	req.Options = normalizeChannelTopicOptions(req.Options)
	if req.Options.TopicID == "" {
		req.Options.TopicID = autoChannelTopicID(ev, req.Options)
		req.AutoTopicID = true
	}
	req.Options.TopicID = cleanChannelTopicID(req.Options.TopicID)
	if req.Options.TopicID == "" {
		return ChannelTopicActionRequest{}, fmt.Errorf("missing topic id")
	}
	req.TopicSHA = shortDocumentHash(req.Options.Topic)
	req.TopicBytes = len(req.Options.Topic)
	req.TopicLines = lineCount(req.Options.Topic)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedTopicIDSHA = shortDocumentHash(req.Options.TopicID)
	return req, nil
}

func channelTopicActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelTopicActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelTopicIssueTarget(ev Event, req *ChannelTopicActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel topic requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func RunChannelTopic(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelTopicOptions) (ChannelTopicResult, error) {
	opts = normalizeChannelTopicOptions(opts)
	var err error
	opts, err = applyChannelTopicRoute(cfg, opts)
	if err != nil {
		return ChannelTopicResult{}, err
	}
	if err := validateChannelTopicOptions(opts); err != nil {
		return ChannelTopicResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelTopicResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelTopicResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelTopicMatches(comment.Body, opts.Channel, opts.TopicID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelTopicResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(opts.Repo, issue.Number),
				Created:     created,
				Duplicate:   true,
				RouteName:   opts.Route,
				RouteHash:   channelRouteHash(opts.Route),
				Channel:     opts.Channel,
				ThreadHash:  shortDocumentHash(opts.ThreadID),
				TopicIDHash: shortDocumentHash(opts.TopicID),
				TopicHash:   shortDocumentHash(opts.Topic),
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelTopicComment(opts))
	if err != nil {
		return ChannelTopicResult{}, fmt.Errorf("post channel topic: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelTopicResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelTopicResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(opts.Repo, issue.Number),
		CommentID:   posted.ID,
		Created:     created,
		RouteName:   opts.Route,
		RouteHash:   channelRouteHash(opts.Route),
		Channel:     opts.Channel,
		ThreadHash:  shortDocumentHash(opts.ThreadID),
		TopicIDHash: shortDocumentHash(opts.TopicID),
		TopicHash:   shortDocumentHash(opts.Topic),
	}, nil
}

func normalizeChannelTopicOptions(opts ChannelTopicOptions) ChannelTopicOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.TopicID = cleanChannelTopicID(opts.TopicID)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.Topic = strings.TrimSpace(opts.Topic)
	return opts
}

func applyChannelTopicRoute(cfg Config, opts ChannelTopicOptions) (ChannelTopicOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.TopicID,
		Author:    opts.Author,
		Body:      opts.Topic,
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

func validateChannelTopicOptions(opts ChannelTopicOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.TopicID == "" {
		return fmt.Errorf("missing topic id")
	}
	if opts.Topic == "" {
		return fmt.Errorf("missing channel topic")
	}
	return nil
}

func RenderChannelTopicComment(opts ChannelTopicOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-topic channel="%s" thread_id="%s" topic_id="%s" author="%s" -->
%s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.TopicID), escapeMarkerValue(author), strings.TrimSpace(opts.Topic))
}

func channelTopicMatches(body, channel, topicID string) bool {
	return HasChannelTopicMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`topic_id="%s"`, escapeMarkerValue(topicID)))
}

func channelTopicMarkerFields(body string) (string, string, string) {
	match := channelTopicMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", ""
	}
	return markerAttribute(match[1], "channel"),
		markerAttribute(match[1], "thread_id"),
		markerAttribute(match[1], "topic_id")
}

func RenderChannelTopicActionReport(ev Event, req ChannelTopicActionRequest, result ChannelTopicResult) string {
	status := "queued"
	if result.Duplicate {
		status = "duplicate"
	}
	threadHash := result.ThreadHash
	if threadHash == "" && req.Options.ThreadID != "" {
		threadHash = shortDocumentHash(req.Options.ThreadID)
	}
	channel := result.Channel
	if channel == "" {
		channel = req.Options.Channel
	}
	topicIDHash := result.TopicIDHash
	if topicIDHash == "" {
		topicIDHash = req.RequestedTopicIDSHA
	}
	topicHash := result.TopicHash
	if topicHash == "" {
		topicHash = req.TopicSHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Topic Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_topic_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- topic_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- target_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- topic_id_sha256_12: `%s`\n", noneIfEmpty(topicIDHash))
	fmt.Fprintf(&b, "- topic_id_auto: `%t`\n", req.AutoTopicID)
	fmt.Fprintf(&b, "- topic_sha256_12: `%s`\n", topicHash)
	fmt.Fprintf(&b, "- topic_bytes: `%d`\n", req.TopicBytes)
	fmt.Fprintf(&b, "- topic_lines: `%d`\n", req.TopicLines)
	fmt.Fprintf(&b, "- topic_source: `%s`\n", req.TopicSource)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- target_issue_is_source: `%t`\n", result.IssueNumber == ev.Issue.Number)
	fmt.Fprintf(&b, "- raw_route_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_topic_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_topic_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- github_issue_title_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_topic_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_topic_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a structured `gitclaw:channel-topic` comment on the canonical channel issue. Provider tokens, provider APIs, raw thread IDs, raw topic IDs, raw topic text, GitHub issue-title mutations, repository mutations, prompts, tool outputs, and credentials are not included in this receipt.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending topic updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent topic updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate topic updates are suppressed by `channel + topic_id`\n")
	return strings.TrimSpace(b.String())
}

func cleanChannelTopicID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelTopicID(ev Event, opts ChannelTopicOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.Topic}, "|")
	return fmt.Sprintf("topic-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func writeChannelTopicOutputs(result ChannelTopicResult) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT: %w", err)
	}
	defer file.Close()
	fmt.Fprintf(file, "issue_number=%d\n", result.IssueNumber)
	fmt.Fprintf(file, "issue_url=%s\n", result.IssueURL)
	fmt.Fprintf(file, "comment_id=%d\n", result.CommentID)
	fmt.Fprintf(file, "created=%t\n", result.Created)
	fmt.Fprintf(file, "duplicate=%t\n", result.Duplicate)
	fmt.Fprintf(file, "route_resolved=%t\n", result.RouteName != "")
	fmt.Fprintf(file, "route_sha256_12=%s\n", result.RouteHash)
	fmt.Fprintf(file, "topic_id_sha256_12=%s\n", result.TopicIDHash)
	return nil
}
