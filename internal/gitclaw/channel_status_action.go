package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelStatusOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	TargetMessageID string
	StatusID        string
	State           string
	Author          string
	Body            string
}

type ChannelStatusResult struct {
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
	StatusIDHash      string
	StateHash         string
	BodyHash          string
}

type ChannelStatusActionRequest struct {
	Options                 ChannelStatusOptions
	Command                 string
	Subcommand              string
	AutoStatusID            bool
	TargetFromIssue         bool
	StatusBodySHA           string
	StatusBodyBytes         int
	StatusBodyLines         int
	BodySource              string
	RequestedRouteHash      string
	RequestedThreadHash     string
	RequestedTargetMsgHash  string
	RequestedStatusIDHash   string
	RequestedStatusStateSHA string
}

func IsChannelStatusActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelStatusActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelStatusActionFields(fields)
}

func isChannelStatusActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "status", "progress", "typing":
		return true
	default:
		return false
	}
}

func BuildChannelStatusActionRequest(ev Event, cfg Config) (ChannelStatusActionRequest, error) {
	fields, trailingBody, ok := channelStatusActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelStatusActionRequest{}, fmt.Errorf("missing channel status command")
	}
	req := ChannelStatusActionRequest{
		Options: ChannelStatusOptions{
			Repo: ev.Repo,
		},
		Command:    fields[0],
		Subcommand: strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		BodySource: "default",
	}
	var bodyParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--message", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetMessageID = fields[i+1]
			i++
		case "--status-id", "--update-id", "--id":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.StatusID = fields[i+1]
			i++
		case "--state", "--status":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.State = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--body":
			if i+1 >= len(fields) {
				return ChannelStatusActionRequest{}, fmt.Errorf("--body requires a value")
			}
			bodyParts = append(bodyParts, fields[i+1:]...)
			i = len(fields)
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelStatusActionRequest{}, fmt.Errorf("unknown channel status argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			if req.Options.State == "" {
				req.Options.State = field
				continue
			}
			bodyParts = append(bodyParts, fields[i:]...)
			i = len(fields)
		}
	}
	if req.Options.State == "" {
		req.Options.State = defaultChannelStatusState(req.Subcommand)
	}
	body := strings.TrimSpace(strings.Join(bodyParts, " "))
	trailingBody = strings.TrimSpace(trailingBody)
	if trailingBody != "" {
		if body != "" {
			body += "\n" + trailingBody
		} else {
			body = trailingBody
			req.BodySource = "trailing-lines"
		}
	}
	if strings.TrimSpace(body) != "" && req.BodySource == "default" {
		req.BodySource = "inline"
	}
	req.Options.Body = body
	if err := applyChannelStatusIssueTarget(ev, &req); err != nil {
		return ChannelStatusActionRequest{}, err
	}
	req.Options = normalizeChannelStatusOptions(req.Options)
	if req.Options.Body == "" {
		req.Options.Body = defaultChannelStatusBody(req.Options.State)
		req.BodySource = "default"
	}
	if req.Options.StatusID == "" {
		req.Options.StatusID = autoChannelStatusID(ev, req.Options)
		req.AutoStatusID = true
	}
	req.Options.StatusID = cleanChannelStatusID(req.Options.StatusID)
	if req.Options.StatusID == "" {
		return ChannelStatusActionRequest{}, fmt.Errorf("missing status id")
	}
	req.StatusBodySHA = shortDocumentHash(req.Options.Body)
	req.StatusBodyBytes = len(req.Options.Body)
	req.StatusBodyLines = lineCount(req.Options.Body)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedTargetMsgHash = shortDocumentHash(req.Options.TargetMessageID)
	req.RequestedStatusIDHash = shortDocumentHash(req.Options.StatusID)
	req.RequestedStatusStateSHA = shortDocumentHash(req.Options.State)
	return req, nil
}

func channelStatusActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelStatusActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelStatusIssueTarget(ev Event, req *ChannelStatusActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel status requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func RunChannelStatus(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelStatusOptions) (ChannelStatusResult, error) {
	opts = normalizeChannelStatusOptions(opts)
	var err error
	opts, err = applyChannelStatusRoute(cfg, opts)
	if err != nil {
		return ChannelStatusResult{}, err
	}
	if err := validateChannelStatusOptions(opts); err != nil {
		return ChannelStatusResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelStatusResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelStatusResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelStatusMatches(comment.Body, opts.Channel, opts.StatusID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelStatusResult{
				IssueNumber:       issue.Number,
				IssueURL:          issueURL(opts.Repo, issue.Number),
				Created:           created,
				Duplicate:         true,
				RouteName:         opts.Route,
				RouteHash:         channelRouteHash(opts.Route),
				Channel:           opts.Channel,
				ThreadHash:        shortDocumentHash(opts.ThreadID),
				TargetMessageHash: shortDocumentHash(opts.TargetMessageID),
				StatusIDHash:      shortDocumentHash(opts.StatusID),
				StateHash:         shortDocumentHash(opts.State),
				BodyHash:          shortDocumentHash(opts.Body),
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelStatusComment(opts))
	if err != nil {
		return ChannelStatusResult{}, fmt.Errorf("post channel status: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelStatusResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelStatusResult{
		IssueNumber:       issue.Number,
		IssueURL:          issueURL(opts.Repo, issue.Number),
		CommentID:         posted.ID,
		Created:           created,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		TargetMessageHash: shortDocumentHash(opts.TargetMessageID),
		StatusIDHash:      shortDocumentHash(opts.StatusID),
		StateHash:         shortDocumentHash(opts.State),
		BodyHash:          shortDocumentHash(opts.Body),
	}, nil
}

func normalizeChannelStatusOptions(opts ChannelStatusOptions) ChannelStatusOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.TargetMessageID = strings.TrimSpace(opts.TargetMessageID)
	opts.StatusID = cleanChannelStatusID(opts.StatusID)
	opts.State = cleanChannelStatusState(opts.State)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.Body = strings.TrimSpace(opts.Body)
	return opts
}

func applyChannelStatusRoute(cfg Config, opts ChannelStatusOptions) (ChannelStatusOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.StatusID,
		Author:    opts.Author,
		Body:      opts.Body,
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

func validateChannelStatusOptions(opts ChannelStatusOptions) error {
	if err := validateRepoName(opts.Repo); err != nil {
		return err
	}
	if opts.Channel == "" {
		return fmt.Errorf("missing channel")
	}
	if opts.ThreadID == "" {
		return fmt.Errorf("missing thread id")
	}
	if opts.TargetMessageID == "" {
		return fmt.Errorf("missing target message id")
	}
	if opts.StatusID == "" {
		return fmt.Errorf("missing status id")
	}
	if opts.State == "" {
		return fmt.Errorf("missing status state")
	}
	if opts.Body == "" {
		return fmt.Errorf("missing status body")
	}
	return nil
}

func RenderChannelStatusComment(opts ChannelStatusOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-status channel="%s" thread_id="%s" target_message_id="%s" status_id="%s" state="%s" author="%s" -->
%s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.TargetMessageID), escapeMarkerValue(opts.StatusID), escapeMarkerValue(opts.State), escapeMarkerValue(author), strings.TrimSpace(opts.Body))
}

func channelStatusMatches(body, channel, statusID string) bool {
	return HasChannelStatusMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`status_id="%s"`, escapeMarkerValue(statusID)))
}

func channelStatusMarkerFields(body string) (string, string, string, string, string) {
	match := channelStatusMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", "", "", ""
	}
	return markerAttribute(match[1], "channel"),
		markerAttribute(match[1], "thread_id"),
		markerAttribute(match[1], "target_message_id"),
		markerAttribute(match[1], "status_id"),
		markerAttribute(match[1], "state")
}

func RenderChannelStatusActionReport(ev Event, req ChannelStatusActionRequest, result ChannelStatusResult) string {
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
	targetHash := result.TargetMessageHash
	if targetHash == "" {
		targetHash = req.RequestedTargetMsgHash
	}
	statusIDHash := result.StatusIDHash
	if statusIDHash == "" {
		statusIDHash = req.RequestedStatusIDHash
	}
	stateHash := result.StateHash
	if stateHash == "" {
		stateHash = req.RequestedStatusStateSHA
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.StatusBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Status Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_status_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- status_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- target_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- target_message_id_sha256_12: `%s`\n", noneIfEmpty(targetHash))
	fmt.Fprintf(&b, "- status_id_sha256_12: `%s`\n", noneIfEmpty(statusIDHash))
	fmt.Fprintf(&b, "- status_id_auto: `%t`\n", req.AutoStatusID)
	fmt.Fprintf(&b, "- status_state_sha256_12: `%s`\n", noneIfEmpty(stateHash))
	fmt.Fprintf(&b, "- status_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- status_body_bytes: `%d`\n", req.StatusBodyBytes)
	fmt.Fprintf(&b, "- status_body_lines: `%d`\n", req.StatusBodyLines)
	fmt.Fprintf(&b, "- status_body_source: `%s`\n", req.BodySource)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- target_issue_is_source: `%t`\n", result.IssueNumber == ev.Issue.Number)
	fmt.Fprintf(&b, "- raw_route_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_state_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_status_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_status_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a structured `gitclaw:channel-status` comment on the canonical channel issue. Provider tokens, provider APIs, raw thread IDs, raw target message IDs, raw status IDs, raw status states, and status bodies are not included in this receipt.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending status updates with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent status updates with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate status updates are suppressed by `channel + status_id`\n")
	return strings.TrimSpace(b.String())
}

func defaultChannelStatusState(subcommand string) string {
	switch strings.ToLower(strings.Trim(subcommand, " \t\r\n.,:;!?")) {
	case "typing":
		return "typing"
	default:
		return "working"
	}
}

func defaultChannelStatusBody(state string) string {
	switch cleanChannelStatusState(state) {
	case "queued":
		return "GitClaw queued this request."
	case "typing":
		return "GitClaw is preparing a response."
	case "working":
		return "GitClaw is working on it."
	case "blocked":
		return "GitClaw needs attention before continuing."
	case "done", "complete", "completed":
		return "GitClaw finished this step."
	default:
		return "GitClaw status update."
	}
}

func cleanChannelStatusState(value string) string {
	return cleanChannelReaction(value)
}

func cleanChannelStatusID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelStatusID(ev Event, opts ChannelStatusOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.TargetMessageID, opts.State, opts.Body}, "|")
	return fmt.Sprintf("status-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func writeChannelStatusOutputs(result ChannelStatusResult) error {
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
	fmt.Fprintf(file, "status_id_sha256_12=%s\n", result.StatusIDHash)
	fmt.Fprintf(file, "status_state_sha256_12=%s\n", result.StateHash)
	return nil
}
