package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelReactionOptions struct {
	Repo      string
	Route     string
	Channel   string
	ThreadID  string
	MessageID string
	Reaction  string
	Author    string
}

type ChannelReactionResult struct {
	IssueNumber  int
	IssueURL     string
	CommentID    int64
	Created      bool
	Duplicate    bool
	RouteName    string
	RouteHash    string
	Channel      string
	ThreadHash   string
	MessageHash  string
	ReactionHash string
}

type ChannelReactionActionRequest struct {
	Options             ChannelReactionOptions
	Command             string
	Subcommand          string
	TargetFromIssue     bool
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	ReactionHash        string
}

func IsChannelReactionActionRequest(ev Event, cfg Config) bool {
	return isChannelReactionActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelReactionActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "react", "reaction", "emoji", "ack", "acknowledge", "pin", "star", "bookmark":
		return true
	default:
		return false
	}
}

func BuildChannelReactionActionRequest(ev Event, cfg Config) (ChannelReactionActionRequest, error) {
	fields, ok := channelReactionActionFields(ev, cfg)
	if !ok {
		return ChannelReactionActionRequest{}, fmt.Errorf("missing channel reaction command")
	}
	req := ChannelReactionActionRequest{
		Options: ChannelReactionOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
	}
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelReactionActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelReactionActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelReactionActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--message", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelReactionActionRequest{}, fmt.Errorf("--message-id requires a value")
			}
			req.Options.MessageID = fields[i+1]
			i++
		case "--reaction", "--emoji":
			if i+1 >= len(fields) {
				return ChannelReactionActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Reaction = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelReactionActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelReactionActionRequest{}, fmt.Errorf("unknown channel reaction argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	for _, value := range positional {
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		if req.Options.Reaction == "" {
			req.Options.Reaction = value
			continue
		}
		return ChannelReactionActionRequest{}, fmt.Errorf("unexpected channel reaction argument %q", value)
	}
	if err := applyChannelReactionIssueTarget(ev, &req); err != nil {
		return ChannelReactionActionRequest{}, err
	}
	if req.Options.Reaction == "" {
		req.Options.Reaction = defaultChannelReactionForSubcommand(req.Subcommand)
	}
	req.Options = normalizeChannelReactionOptions(req.Options)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.MessageID)
	req.ReactionHash = shortDocumentHash(req.Options.Reaction)
	return req, nil
}

func defaultChannelReactionForSubcommand(subcommand string) string {
	switch cleanChannelReactionSubcommand(subcommand) {
	case "pin":
		return "pushpin"
	case "star":
		return "star"
	case "bookmark":
		return "bookmark"
	default:
		return ""
	}
}

func cleanChannelReactionSubcommand(value string) string {
	return strings.ToLower(strings.Trim(value, " \t\r\n.,:;!?"))
}

func applyChannelReactionIssueTarget(ev Event, req *ChannelReactionActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel reaction requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func channelReactionActionFields(ev Event, cfg Config) ([]string, bool) {
	for _, line := range strings.Split(activeRequestText(ev), "\n") {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelReactionActionFields(fields) {
			continue
		}
		return fields, true
	}
	return nil, false
}

func RunChannelReaction(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelReactionOptions) (ChannelReactionResult, error) {
	opts = normalizeChannelReactionOptions(opts)
	var err error
	opts, err = applyChannelReactionRoute(cfg, opts)
	if err != nil {
		return ChannelReactionResult{}, err
	}
	if err := validateChannelReactionOptions(opts); err != nil {
		return ChannelReactionResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelReactionResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelReactionResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelReactionMatches(comment.Body, opts.Channel, opts.MessageID, opts.Reaction) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelReactionResult{
				IssueNumber:  issue.Number,
				IssueURL:     issueURL(opts.Repo, issue.Number),
				Created:      created,
				Duplicate:    true,
				RouteName:    opts.Route,
				RouteHash:    channelRouteHash(opts.Route),
				Channel:      opts.Channel,
				ThreadHash:   shortDocumentHash(opts.ThreadID),
				MessageHash:  shortDocumentHash(opts.MessageID),
				ReactionHash: shortDocumentHash(opts.Reaction),
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelReactionComment(opts))
	if err != nil {
		return ChannelReactionResult{}, fmt.Errorf("post channel reaction: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelReactionResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelReactionResult{
		IssueNumber:  issue.Number,
		IssueURL:     issueURL(opts.Repo, issue.Number),
		CommentID:    posted.ID,
		Created:      created,
		RouteName:    opts.Route,
		RouteHash:    channelRouteHash(opts.Route),
		Channel:      opts.Channel,
		ThreadHash:   shortDocumentHash(opts.ThreadID),
		MessageHash:  shortDocumentHash(opts.MessageID),
		ReactionHash: shortDocumentHash(opts.Reaction),
	}, nil
}

func normalizeChannelReactionOptions(opts ChannelReactionOptions) ChannelReactionOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.MessageID = strings.TrimSpace(opts.MessageID)
	opts.Reaction = cleanChannelReaction(opts.Reaction)
	opts.Author = strings.TrimSpace(opts.Author)
	return opts
}

func applyChannelReactionRoute(cfg Config, opts ChannelReactionOptions) (ChannelReactionOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.MessageID,
		Author:    opts.Author,
		Body:      "reaction:" + opts.Reaction,
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

func validateChannelReactionOptions(opts ChannelReactionOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.MessageID == "" {
		return fmt.Errorf("missing target message id")
	}
	if opts.Reaction == "" {
		return fmt.Errorf("missing reaction")
	}
	return nil
}

func RenderChannelReactionComment(opts ChannelReactionOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-reaction channel="%s" thread_id="%s" message_id="%s" reaction="%s" author="%s" -->
GitClaw channel reaction: %s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.MessageID), escapeMarkerValue(opts.Reaction), escapeMarkerValue(author), opts.Reaction)
}

func channelReactionMatches(body, channel, messageID, reaction string) bool {
	return HasChannelReactionMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`message_id="%s"`, escapeMarkerValue(messageID))) &&
		strings.Contains(body, fmt.Sprintf(`reaction="%s"`, escapeMarkerValue(reaction)))
}

func channelReactionMarkerFields(body string) (string, string, string, string) {
	match := channelReactionMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", "", ""
	}
	return markerAttribute(match[1], "channel"),
		markerAttribute(match[1], "thread_id"),
		markerAttribute(match[1], "message_id"),
		markerAttribute(match[1], "reaction")
}

func RenderChannelReactionActionReport(ev Event, req ChannelReactionActionRequest, result ChannelReactionResult) string {
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
	messageHash := result.MessageHash
	if messageHash == "" {
		messageHash = req.RequestedMsgHash
	}
	reactionHash := result.ReactionHash
	if reactionHash == "" {
		reactionHash = req.ReactionHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Reaction Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_reaction_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- reaction_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- target_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- target_message_id_sha256_12: `%s`\n", noneIfEmpty(messageHash))
	fmt.Fprintf(&b, "- reaction_sha256_12: `%s`\n", noneIfEmpty(reactionHash))
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- target_issue_is_source: `%t`\n", result.IssueNumber == ev.Issue.Number)
	fmt.Fprintf(&b, "- raw_route_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_reaction_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_reaction_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a structured `gitclaw:channel-reaction` comment on the canonical channel issue. Provider tokens, provider APIs, raw thread IDs, raw target message IDs, raw reactions, and channel message bodies are not included in this receipt.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending reactions with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent reactions with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate reactions are suppressed by `channel + target_message_id + reaction`\n")
	return strings.TrimSpace(b.String())
}

func cleanChannelReaction(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n`\"'"))
	value = strings.Trim(value, ":")
	value = strings.ReplaceAll(value, " ", "-")
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' || r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeChannelReactionOutputs(result ChannelReactionResult) error {
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
	fmt.Fprintf(file, "reaction_sha256_12=%s\n", result.ReactionHash)
	return nil
}
