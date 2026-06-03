package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultChannelActivityTTLSeconds = 10
const maxChannelActivityTTLSeconds = 3600

type ChannelActivityOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	TargetMessageID string
	ActivityID      string
	Activity        string
	TTLSeconds      int
	Author          string
}

type ChannelActivityResult struct {
	IssueNumber       int
	IssueURL          string
	CommentID         int64
	Created           bool
	Duplicate         bool
	RouteName         string
	RouteHash         string
	Channel           string
	ThreadHash        string
	TargetMessageHash string
	ActivityIDHash    string
	ActivityHash      string
}

type ChannelActivityActionRequest struct {
	Options                 ChannelActivityOptions
	Command                 string
	Subcommand              string
	AutoActivityID          bool
	TargetFromIssue         bool
	RequestedRouteHash      string
	RequestedThreadHash     string
	RequestedTargetMsgHash  string
	RequestedActivityIDHash string
	ActivityHash            string
}

func IsChannelActivityActionRequest(ev Event, cfg Config) bool {
	return isChannelActivityActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelActivityActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelActivitySubcommand(fields[1]) {
	case "activity", "chat-action", "action", "presence", "typing-action", "recording", "uploading", "thinking", "stop-activity":
		return true
	default:
		return false
	}
}

func BuildChannelActivityActionRequest(ev Event, cfg Config) (ChannelActivityActionRequest, error) {
	fields, ok := channelActivityActionFields(ev, cfg)
	if !ok {
		return ChannelActivityActionRequest{}, fmt.Errorf("missing channel activity command")
	}
	req := ChannelActivityActionRequest{
		Options: ChannelActivityOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: cleanChannelActivitySubcommand(fields[1]),
	}
	if defaultActivity := defaultChannelActivityForSubcommand(req.Subcommand); defaultActivity != "" {
		req.Options.Activity = defaultActivity
	}
	var positional []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--message", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetMessageID = fields[i+1]
			i++
		case "--activity-id", "--event-id", "--id":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.ActivityID = fields[i+1]
			i++
		case "--activity", "--kind", "--state":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.Activity = fields[i+1]
			i++
		case "--ttl-seconds", "--ttl":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			ttl, err := parseChannelActivityTTL(fields[i+1])
			if err != nil {
				return ChannelActivityActionRequest{}, err
			}
			req.Options.TTLSeconds = ttl
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelActivityActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelActivityActionRequest{}, fmt.Errorf("unknown channel activity argument %q", field)
			}
			positional = append(positional, field)
		}
	}
	for _, value := range positional {
		if req.Options.Activity == "" {
			req.Options.Activity = value
			continue
		}
		if req.Options.Route == "" && req.Options.Channel == "" {
			req.Options.Route = value
			continue
		}
		return ChannelActivityActionRequest{}, fmt.Errorf("unexpected channel activity argument %q", value)
	}
	if err := applyChannelActivityIssueTarget(ev, &req); err != nil {
		return ChannelActivityActionRequest{}, err
	}
	req.Options = normalizeChannelActivityOptions(req.Options)
	if req.Options.ActivityID == "" {
		req.Options.ActivityID = autoChannelActivityID(ev, req.Options)
		req.AutoActivityID = true
	}
	req.Options.ActivityID = cleanChannelActivityID(req.Options.ActivityID)
	if req.Options.TTLSeconds == 0 && req.Options.Activity != "idle" && req.Options.Activity != "stopped" {
		req.Options.TTLSeconds = defaultChannelActivityTTLSeconds
	}
	if req.Options.ActivityID == "" {
		return ChannelActivityActionRequest{}, fmt.Errorf("missing activity id")
	}
	if err := validateChannelActivityActionRequestOptions(req.Options); err != nil {
		return ChannelActivityActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedTargetMsgHash = shortDocumentHash(req.Options.TargetMessageID)
	req.RequestedActivityIDHash = shortDocumentHash(req.Options.ActivityID)
	req.ActivityHash = shortDocumentHash(req.Options.Activity)
	return req, nil
}

func channelActivityActionFields(ev Event, cfg Config) ([]string, bool) {
	for _, line := range strings.Split(activeRequestText(ev), "\n") {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelActivityActionFields(fields) {
			continue
		}
		return fields, true
	}
	return nil, false
}

func applyChannelActivityIssueTarget(ev Event, req *ChannelActivityActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel activity requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func RunChannelActivity(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelActivityOptions) (ChannelActivityResult, error) {
	opts = normalizeChannelActivityOptions(opts)
	var err error
	opts, err = applyChannelActivityRoute(cfg, opts)
	if err != nil {
		return ChannelActivityResult{}, err
	}
	if opts.TTLSeconds == 0 && opts.Activity != "idle" && opts.Activity != "stopped" {
		opts.TTLSeconds = defaultChannelActivityTTLSeconds
	}
	if err := validateChannelActivityOptions(opts); err != nil {
		return ChannelActivityResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelActivityResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelActivityResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelActivityMatches(comment.Body, opts.Channel, opts.ActivityID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelActivityResult{
				IssueNumber:       issue.Number,
				IssueURL:          issueURL(opts.Repo, issue.Number),
				Created:           created,
				Duplicate:         true,
				RouteName:         opts.Route,
				RouteHash:         channelRouteHash(opts.Route),
				Channel:           opts.Channel,
				ThreadHash:        shortDocumentHash(opts.ThreadID),
				TargetMessageHash: shortDocumentHash(opts.TargetMessageID),
				ActivityIDHash:    shortDocumentHash(opts.ActivityID),
				ActivityHash:      shortDocumentHash(opts.Activity),
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelActivityComment(opts))
	if err != nil {
		return ChannelActivityResult{}, fmt.Errorf("post channel activity: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelActivityResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelActivityResult{
		IssueNumber:       issue.Number,
		IssueURL:          issueURL(opts.Repo, issue.Number),
		CommentID:         posted.ID,
		Created:           created,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		TargetMessageHash: shortDocumentHash(opts.TargetMessageID),
		ActivityIDHash:    shortDocumentHash(opts.ActivityID),
		ActivityHash:      shortDocumentHash(opts.Activity),
	}, nil
}

func normalizeChannelActivityOptions(opts ChannelActivityOptions) ChannelActivityOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.TargetMessageID = strings.TrimSpace(opts.TargetMessageID)
	opts.ActivityID = cleanChannelActivityID(opts.ActivityID)
	opts.Activity = cleanChannelActivity(opts.Activity)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.TTLSeconds < 0 {
		opts.TTLSeconds = 0
	}
	return opts
}

func applyChannelActivityRoute(cfg Config, opts ChannelActivityOptions) (ChannelActivityOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.ActivityID,
		Author:    opts.Author,
		Body:      "activity:" + opts.Activity,
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

func validateChannelActivityOptions(opts ChannelActivityOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.ActivityID == "" {
		return fmt.Errorf("missing activity id")
	}
	if opts.Activity == "" {
		return fmt.Errorf("missing channel activity")
	}
	if opts.TTLSeconds > maxChannelActivityTTLSeconds {
		return fmt.Errorf("channel activity ttl exceeds %d seconds", maxChannelActivityTTLSeconds)
	}
	return nil
}

func validateChannelActivityActionRequestOptions(opts ChannelActivityOptions) error {
	if opts.Route == "" && (opts.Channel == "" || opts.ThreadID == "") {
		return fmt.Errorf("channel activity requires a route or channel/thread target")
	}
	if opts.Activity == "" {
		return fmt.Errorf("missing channel activity")
	}
	if opts.TTLSeconds > maxChannelActivityTTLSeconds {
		return fmt.Errorf("channel activity ttl exceeds %d seconds", maxChannelActivityTTLSeconds)
	}
	return nil
}

func RenderChannelActivityComment(opts ChannelActivityOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-activity channel="%s" thread_id="%s" target_message_id="%s" activity_id="%s" activity="%s" ttl_seconds="%d" author="%s" -->
GitClaw channel activity: %s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.TargetMessageID), escapeMarkerValue(opts.ActivityID), escapeMarkerValue(opts.Activity), opts.TTLSeconds, escapeMarkerValue(author), opts.Activity)
}

func channelActivityMatches(body, channel, activityID string) bool {
	return HasChannelActivityMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`activity_id="%s"`, escapeMarkerValue(activityID)))
}

func channelActivityMarkerFields(body string) (string, string, string, string, string) {
	match := channelActivityMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", "", "", ""
	}
	return markerAttribute(match[1], "channel"),
		markerAttribute(match[1], "thread_id"),
		markerAttribute(match[1], "target_message_id"),
		markerAttribute(match[1], "activity_id"),
		markerAttribute(match[1], "activity")
}

func RenderChannelActivityActionReport(ev Event, req ChannelActivityActionRequest, result ChannelActivityResult) string {
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
	targetMessageHash := result.TargetMessageHash
	if targetMessageHash == "" {
		targetMessageHash = req.RequestedTargetMsgHash
	}
	activityIDHash := result.ActivityIDHash
	if activityIDHash == "" {
		activityIDHash = req.RequestedActivityIDHash
	}
	activityHash := result.ActivityHash
	if activityHash == "" {
		activityHash = req.ActivityHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Activity Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_activity_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- activity_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- target_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- target_message_id_sha256_12: `%s`\n", noneIfEmpty(targetMessageHash))
	fmt.Fprintf(&b, "- activity_id_sha256_12: `%s`\n", noneIfEmpty(activityIDHash))
	fmt.Fprintf(&b, "- activity_id_auto: `%t`\n", req.AutoActivityID)
	fmt.Fprintf(&b, "- activity_sha256_12: `%s`\n", noneIfEmpty(activityHash))
	fmt.Fprintf(&b, "- ttl_seconds: `%d`\n", req.Options.TTLSeconds)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- target_issue_is_source: `%t`\n", result.IssueNumber == ev.Issue.Number)
	fmt.Fprintf(&b, "- raw_route_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_activity_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_activity_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_activity_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_activity_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a structured `gitclaw:channel-activity` comment on the canonical channel issue. Provider tokens, provider APIs, raw thread IDs, raw target message IDs, raw activity IDs, raw activity names, channel message bodies, prompts, tool outputs, and credentials are not included in this receipt.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending activity signals with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent activity signals with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate activity signals are suppressed by `channel + activity_id`\n")
	return strings.TrimSpace(b.String())
}

func cleanChannelActivitySubcommand(value string) string {
	return strings.ToLower(strings.Trim(value, " \t\r\n.,:;!?"))
}

func defaultChannelActivityForSubcommand(subcommand string) string {
	switch cleanChannelActivitySubcommand(subcommand) {
	case "typing-action":
		return "typing"
	case "recording":
		return "recording"
	case "uploading":
		return "uploading"
	case "thinking":
		return "thinking"
	case "stop-activity":
		return "idle"
	default:
		return ""
	}
}

func cleanChannelActivity(value string) string {
	return cleanChannelReaction(value)
}

func cleanChannelActivityID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelActivityID(ev Event, opts ChannelActivityOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.TargetMessageID, opts.Activity}, "|")
	return fmt.Sprintf("activity-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func parseChannelActivityTTL(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid --ttl-seconds: %q", value)
	}
	if parsed > maxChannelActivityTTLSeconds {
		return 0, fmt.Errorf("channel activity ttl exceeds %d seconds", maxChannelActivityTTLSeconds)
	}
	return parsed, nil
}

func writeChannelActivityOutputs(result ChannelActivityResult) error {
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
	fmt.Fprintf(file, "activity_id_sha256_12=%s\n", result.ActivityIDHash)
	fmt.Fprintf(file, "activity_sha256_12=%s\n", result.ActivityHash)
	return nil
}
