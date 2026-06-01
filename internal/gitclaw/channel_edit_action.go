package gitclaw

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ChannelEditOptions struct {
	Repo            string
	Route           string
	Channel         string
	ThreadID        string
	TargetMessageID string
	EditID          string
	Author          string
	Body            string
}

type ChannelEditResult struct {
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
	EditIDHash        string
	BodyHash          string
}

type ChannelEditActionRequest struct {
	Options                ChannelEditOptions
	Command                string
	Subcommand             string
	AutoEditID             bool
	TargetFromIssue        bool
	EditBodySHA            string
	EditBodyBytes          int
	EditBodyLines          int
	BodySource             string
	RequestedRouteHash     string
	RequestedThreadHash    string
	RequestedTargetMsgHash string
	RequestedEditIDHash    string
}

func IsChannelEditActionRequest(ev Event, cfg Config) bool {
	fields, _, ok := channelEditActionFieldsAndTrailingBody(ev, cfg)
	return ok && isChannelEditActionFields(fields)
}

func isChannelEditActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "edit", "update", "replace":
		return true
	default:
		return false
	}
}

func BuildChannelEditActionRequest(ev Event, cfg Config) (ChannelEditActionRequest, error) {
	fields, trailingBody, ok := channelEditActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelEditActionRequest{}, fmt.Errorf("missing channel edit command")
	}
	req := ChannelEditActionRequest{
		Options: ChannelEditOptions{
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
				return ChannelEditActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelEditActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelEditActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--message", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelEditActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.TargetMessageID = fields[i+1]
			i++
		case "--edit-id", "--update-id", "--id":
			if i+1 >= len(fields) {
				return ChannelEditActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.EditID = fields[i+1]
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelEditActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		case "--body":
			if i+1 >= len(fields) {
				return ChannelEditActionRequest{}, fmt.Errorf("--body requires a value")
			}
			bodyParts = append(bodyParts, fields[i+1:]...)
			i = len(fields)
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelEditActionRequest{}, fmt.Errorf("unknown channel edit argument %q", field)
			}
			if req.Options.Route == "" && req.Options.Channel == "" {
				req.Options.Route = field
				continue
			}
			bodyParts = append(bodyParts, fields[i:]...)
			i = len(fields)
		}
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
	if err := applyChannelEditIssueTarget(ev, &req); err != nil {
		return ChannelEditActionRequest{}, err
	}
	req.Options = normalizeChannelEditOptions(req.Options)
	if req.Options.EditID == "" {
		req.Options.EditID = autoChannelEditID(ev, req.Options)
		req.AutoEditID = true
	}
	req.Options.EditID = cleanChannelEditID(req.Options.EditID)
	if req.Options.EditID == "" {
		return ChannelEditActionRequest{}, fmt.Errorf("missing edit id")
	}
	req.EditBodySHA = shortDocumentHash(req.Options.Body)
	req.EditBodyBytes = len(req.Options.Body)
	req.EditBodyLines = lineCount(req.Options.Body)
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedTargetMsgHash = shortDocumentHash(req.Options.TargetMessageID)
	req.RequestedEditIDHash = shortDocumentHash(req.Options.EditID)
	return req, nil
}

func channelEditActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelEditActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelEditIssueTarget(ev Event, req *ChannelEditActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel edit requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func RunChannelEdit(ctx context.Context, cfg Config, github ChannelSendGitHubClient, opts ChannelEditOptions) (ChannelEditResult, error) {
	opts = normalizeChannelEditOptions(opts)
	var err error
	opts, err = applyChannelEditRoute(cfg, opts)
	if err != nil {
		return ChannelEditResult{}, err
	}
	if err := validateChannelEditOptions(opts); err != nil {
		return ChannelEditResult{}, err
	}

	issue, created, err := findOrCreateChannelIssue(ctx, cfg, github, ChannelIngestOptions{
		Repo:     opts.Repo,
		Channel:  opts.Channel,
		ThreadID: opts.ThreadID,
	})
	if err != nil {
		return ChannelEditResult{}, err
	}

	comments, err := github.ListIssueComments(ctx, opts.Repo, issue.Number)
	if err != nil {
		return ChannelEditResult{}, fmt.Errorf("list channel issue comments: %w", err)
	}
	for _, comment := range comments {
		if channelEditMatches(comment.Body, opts.Channel, opts.EditID) {
			_ = github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel})
			return ChannelEditResult{
				IssueNumber:       issue.Number,
				IssueURL:          issueURL(opts.Repo, issue.Number),
				Created:           created,
				Duplicate:         true,
				RouteName:         opts.Route,
				RouteHash:         channelRouteHash(opts.Route),
				Channel:           opts.Channel,
				ThreadHash:        shortDocumentHash(opts.ThreadID),
				TargetMessageHash: shortDocumentHash(opts.TargetMessageID),
				EditIDHash:        shortDocumentHash(opts.EditID),
				BodyHash:          shortDocumentHash(opts.Body),
			}, nil
		}
	}

	posted, err := github.PostIssueComment(ctx, opts.Repo, issue.Number, RenderChannelEditComment(opts))
	if err != nil {
		return ChannelEditResult{}, fmt.Errorf("post channel edit: %w", err)
	}
	if err := github.AddIssueLabels(ctx, opts.Repo, issue.Number, []string{cfg.ChannelLabel}); err != nil {
		return ChannelEditResult{}, fmt.Errorf("label channel issue: %w", err)
	}
	return ChannelEditResult{
		IssueNumber:       issue.Number,
		IssueURL:          issueURL(opts.Repo, issue.Number),
		CommentID:         posted.ID,
		Created:           created,
		RouteName:         opts.Route,
		RouteHash:         channelRouteHash(opts.Route),
		Channel:           opts.Channel,
		ThreadHash:        shortDocumentHash(opts.ThreadID),
		TargetMessageHash: shortDocumentHash(opts.TargetMessageID),
		EditIDHash:        shortDocumentHash(opts.EditID),
		BodyHash:          shortDocumentHash(opts.Body),
	}, nil
}

func normalizeChannelEditOptions(opts ChannelEditOptions) ChannelEditOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.TargetMessageID = strings.TrimSpace(opts.TargetMessageID)
	opts.EditID = cleanChannelEditID(opts.EditID)
	opts.Author = strings.TrimSpace(opts.Author)
	opts.Body = strings.TrimSpace(opts.Body)
	return opts
}

func applyChannelEditRoute(cfg Config, opts ChannelEditOptions) (ChannelEditOptions, error) {
	if opts.Route == "" {
		return opts, nil
	}
	routeOpts, err := applyChannelSendRoute(cfg, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.EditID,
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

func validateChannelEditOptions(opts ChannelEditOptions) error {
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
	if opts.EditID == "" {
		return fmt.Errorf("missing edit id")
	}
	if opts.Body == "" {
		return fmt.Errorf("missing edit body")
	}
	return nil
}

func RenderChannelEditComment(opts ChannelEditOptions) string {
	author := opts.Author
	if author == "" {
		author = "gitclaw"
	}
	return fmt.Sprintf(`<!-- gitclaw:channel-edit channel="%s" thread_id="%s" target_message_id="%s" edit_id="%s" author="%s" -->
%s`, escapeMarkerValue(opts.Channel), escapeMarkerValue(opts.ThreadID), escapeMarkerValue(opts.TargetMessageID), escapeMarkerValue(opts.EditID), escapeMarkerValue(author), strings.TrimSpace(opts.Body))
}

func channelEditMatches(body, channel, editID string) bool {
	return HasChannelEditMarker(body) &&
		strings.Contains(body, fmt.Sprintf(`channel="%s"`, escapeMarkerValue(channel))) &&
		strings.Contains(body, fmt.Sprintf(`edit_id="%s"`, escapeMarkerValue(editID)))
}

func channelEditMarkerFields(body string) (string, string, string, string) {
	match := channelEditMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", "", "", ""
	}
	return markerAttribute(match[1], "channel"),
		markerAttribute(match[1], "thread_id"),
		markerAttribute(match[1], "target_message_id"),
		markerAttribute(match[1], "edit_id")
}

func RenderChannelEditActionReport(ev Event, req ChannelEditActionRequest, result ChannelEditResult) string {
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
	editIDHash := result.EditIDHash
	if editIDHash == "" {
		editIDHash = req.RequestedEditIDHash
	}
	bodyHash := result.BodyHash
	if bodyHash == "" {
		bodyHash = req.EditBodySHA
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Edit Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_edit_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- target_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- edit_comment_id: `%d`\n", result.CommentID)
	fmt.Fprintf(&b, "- target_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- route_resolved: `%t`\n", result.RouteName != "")
	fmt.Fprintf(&b, "- requested_route_sha256_12: `%s`\n", noneIfEmpty(req.RequestedRouteHash))
	fmt.Fprintf(&b, "- resolved_route_sha256_12: `%s`\n", noneIfEmpty(result.RouteHash))
	fmt.Fprintf(&b, "- channel: `%s`\n", channel)
	fmt.Fprintf(&b, "- thread_id_sha256_12: `%s`\n", noneIfEmpty(threadHash))
	fmt.Fprintf(&b, "- target_message_id_sha256_12: `%s`\n", noneIfEmpty(targetHash))
	fmt.Fprintf(&b, "- edit_id_sha256_12: `%s`\n", noneIfEmpty(editIDHash))
	fmt.Fprintf(&b, "- edit_id_auto: `%t`\n", req.AutoEditID)
	fmt.Fprintf(&b, "- edit_body_sha256_12: `%s`\n", bodyHash)
	fmt.Fprintf(&b, "- edit_body_bytes: `%d`\n", req.EditBodyBytes)
	fmt.Fprintf(&b, "- edit_body_lines: `%d`\n", req.EditBodyLines)
	fmt.Fprintf(&b, "- edit_body_source: `%s`\n", req.BodySource)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- target_issue_is_source: `%t`\n", result.IssueNumber == ev.Issue.Number)
	fmt.Fprintf(&b, "- raw_route_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_target_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_edit_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_edit_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_edit_action_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw queued a structured `gitclaw:channel-edit` comment on the canonical channel issue. Provider tokens, provider APIs, raw thread IDs, raw target message IDs, raw edit IDs, and replacement bodies are not included in this receipt.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read pending edits with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record applied edits with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate edits are suppressed by `channel + edit_id`\n")
	return strings.TrimSpace(b.String())
}

func cleanChannelEditID(value string) string {
	return cleanChannelHuddleID(value)
}

func autoChannelEditID(ev Event, opts ChannelEditOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.TargetMessageID, opts.Body}, "|")
	return fmt.Sprintf("edit-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func writeChannelEditOutputs(result ChannelEditResult) error {
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
	fmt.Fprintf(file, "edit_id_sha256_12=%s\n", result.EditIDHash)
	return nil
}
