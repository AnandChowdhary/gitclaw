package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelSessionSearchOptions struct {
	Repo              string
	Route             string
	Channel           string
	ThreadID          string
	SourceMessageID   string
	NotifyMessageID   string
	SearchID          string
	Query             string
	MaxResults        int
	Author            string
	SourceIssueNumber int
	SourceCommentID   int64
}

type ChannelSessionSearchResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	SearchIDHash        string
	QueryHash           string
	Search              SessionSearchReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSessionSearchActionRequest struct {
	Options             ChannelSessionSearchOptions
	Search              SessionSearchReport
	Command             string
	Subcommand          string
	AutoSourceMessageID bool
	AutoNotifyMessageID bool
	AutoSearchID        bool
	TargetFromIssue     bool
	QuerySource         string
	RequestedRouteHash  string
	RequestedThreadHash string
	RequestedMsgHash    string
	NotifyMessageHash   string
	SearchIDHash        string
	QuerySHA            string
	QueryBytes          int
	QueryTerms          int
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

func IsChannelSessionSearchActionRequest(ev Event, cfg Config) bool {
	return isChannelSessionSearchActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSessionSearchActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSessionSearchSubcommand(fields[1]) {
	case "session-search", "search-session", "thread-search", "search-thread", "recall-session", "conversation-search", "recall":
		return true
	default:
		return false
	}
}

func BuildChannelSessionSearchActionRequest(ev Event, cfg Config, transcript []TranscriptMessage) (ChannelSessionSearchActionRequest, error) {
	fields, trailing, ok := channelSessionSearchActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSessionSearchActionRequest{}, fmt.Errorf("missing channel session search command")
	}
	req := ChannelSessionSearchActionRequest{
		Options: ChannelSessionSearchOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxResults:        defaultSessionSearchMaxResults,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSessionSearchSubcommand(fields[1]),
	}
	if ev.Comment != nil {
		req.Options.SourceCommentID = ev.Comment.ID
	}
	var queryParts []string
	for i := 2; i < len(fields); i++ {
		field := fields[i]
		switch field {
		case "--route", "-r":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--search-id", "--recall-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SearchID = cleanChannelSessionSearchID(fields[i+1])
			i++
		case "--query", "-q":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			queryParts = append(queryParts, fields[i+1])
			req.QuerySource = "flag"
			i++
		case "--max-results", "--limit":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxResults, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 25", field)
			}
			req.Options.MaxResults = maxResults
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSessionSearchActionRequest{}, fmt.Errorf("unknown channel session search argument %q", field)
			}
			queryParts = append(queryParts, field)
			if req.QuerySource == "" {
				req.QuerySource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = cleanChannelSessionSearchQuery(strings.Join(queryParts, " "))
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = parseChannelSessionSearchTrailingQuery(trailing)
		if req.Options.Query != "" {
			req.QuerySource = "trailing-query"
		}
	}
	if err := applyChannelSessionSearchIssueTarget(ev, &req); err != nil {
		return ChannelSessionSearchActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSessionSearchSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SearchID) == "" {
		req.Options.SearchID = autoChannelSessionSearchID(ev, req.Options)
		req.AutoSearchID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSessionSearchNotifyMessageID(ev, req.Options.SearchID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSessionSearchOptions(req.Options)
	if err := validateChannelSessionSearchActionRequestOptions(req.Options); err != nil {
		return ChannelSessionSearchActionRequest{}, err
	}
	req.Search = BuildSessionSearchReport(transcript, req.Options.Query, req.Options.MaxResults)
	req.QueryTerms = req.Search.QueryTerms
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.SearchIDHash = shortDocumentHash(req.Options.SearchID)
	req.QuerySHA = req.Search.QueryHash
	req.QueryBytes = len(req.Options.Query)
	notificationBody := RenderChannelSessionSearchNotificationBody(req.Options, req.Search)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSessionSearch(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSessionSearchActionRequest) (ChannelSessionSearchResult, error) {
	opts := normalizeChannelSessionSearchOptions(req.Options)
	var err error
	opts, err = applyChannelSessionSearchRoute(cfg, opts)
	if err != nil {
		return ChannelSessionSearchResult{}, err
	}
	if err := validateChannelSessionSearchOptions(opts); err != nil {
		return ChannelSessionSearchResult{}, err
	}
	search := req.Search
	if search.QueryHash == "" {
		search = BuildSessionSearchReport(nil, opts.Query, opts.MaxResults)
	}
	body := RenderChannelSessionSearchNotificationBody(opts, search)
	notification, err := RunChannelSend(ctx, cfg, github, ChannelSendOptions{
		Repo:      opts.Repo,
		Route:     opts.Route,
		Channel:   opts.Channel,
		ThreadID:  opts.ThreadID,
		MessageID: opts.NotifyMessageID,
		Author:    opts.Author,
		Body:      body,
	})
	if err != nil {
		return ChannelSessionSearchResult{}, fmt.Errorf("queue channel session search notification: %w", err)
	}
	return ChannelSessionSearchResult{
		Notification:        notification,
		RouteName:           opts.Route,
		RouteHash:           channelRouteHash(opts.Route),
		Channel:             opts.Channel,
		ThreadHash:          shortDocumentHash(opts.ThreadID),
		MessageHash:         shortDocumentHash(opts.SourceMessageID),
		NotifyHash:          shortDocumentHash(opts.NotifyMessageID),
		SearchIDHash:        shortDocumentHash(opts.SearchID),
		QueryHash:           search.QueryHash,
		Search:              search,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelSessionSearchActionReport(ev Event, req ChannelSessionSearchActionRequest, result ChannelSessionSearchResult) string {
	status := "queued"
	if result.Notification.Duplicate {
		status = "duplicate"
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
	searchIDHash := result.SearchIDHash
	if searchIDHash == "" {
		searchIDHash = req.SearchIDHash
	}
	queryHash := result.QueryHash
	if queryHash == "" {
		queryHash = req.QuerySHA
	}
	notificationBodySHA := result.NotificationBodySHA
	if notificationBodySHA == "" {
		notificationBodySHA = req.NotificationBodySHA
	}
	notificationBytes := result.NotificationBytes
	if notificationBytes == 0 {
		notificationBytes = req.NotificationBytes
	}
	notificationLines := result.NotificationLines
	if notificationLines == 0 {
		notificationLines = req.NotificationLines
	}
	search := result.Search
	if search.QueryHash == "" {
		search = req.Search
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Session Search Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_session_search_status: `%s`\n", status)
	fmt.Fprintf(&b, "- session_search_status: `%s`\n", search.SearchStatus)
	fmt.Fprintf(&b, "- search_mode: `%s`\n", "github-issue-transcript-local-lexical")
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
	fmt.Fprintf(&b, "- source_message_id_auto: `%t`\n", req.AutoSourceMessageID)
	fmt.Fprintf(&b, "- notify_message_id_sha256_12: `%s`\n", noneIfEmpty(notifyHash))
	fmt.Fprintf(&b, "- notify_message_id_auto: `%t`\n", req.AutoNotifyMessageID)
	fmt.Fprintf(&b, "- search_id_sha256_12: `%s`\n", noneIfEmpty(searchIDHash))
	fmt.Fprintf(&b, "- search_id_auto: `%t`\n", req.AutoSearchID)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", noneIfEmpty(queryHash))
	fmt.Fprintf(&b, "- query_terms: `%d`\n", search.QueryTerms)
	fmt.Fprintf(&b, "- query_bytes: `%d`\n", req.QueryBytes)
	fmt.Fprintf(&b, "- query_source: `%s`\n", noneIfEmpty(req.QuerySource))
	fmt.Fprintf(&b, "- max_results: `%d`\n", search.MaxResults)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", search.TranscriptMessages)
	fmt.Fprintf(&b, "- matched_messages: `%d`\n", search.MatchedMessages)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", search.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", search.ResultsReturned)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_query_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_search_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_session_search_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw searched the current GitHub-backed channel transcript and queued a provider-facing recall result with only hashes, indexes, counts, and delivery metadata. The action does not call a model, execute tools, mutate repository files, call provider APIs, or print raw channel bodies.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read session-search recall results with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent recall results with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate recall notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSessionSearchNotificationBody(opts ChannelSessionSearchOptions, report SessionSearchReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel session search\n\n")
	fmt.Fprintf(&b, "Search status: %s\n", report.SearchStatus)
	fmt.Fprintf(&b, "Query hash: %s\n", report.QueryHash)
	fmt.Fprintf(&b, "Query terms: %d\n", report.QueryTerms)
	fmt.Fprintf(&b, "Max results: %d\n", report.MaxResults)
	fmt.Fprintf(&b, "Transcript messages: %d\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "Matched messages: %d\n", report.MatchedMessages)
	fmt.Fprintf(&b, "Matched lines: %d\n", report.MatchedLines)
	fmt.Fprintf(&b, "Results returned: %d\n", report.ResultsReturned)
	fmt.Fprintf(&b, "Search id hash: %s\n", shortDocumentHash(opts.SearchID))
	b.WriteString("\nResults:\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- message=%02d role=%s source=%s trusted=%t edited=%t line=%d score=%d matched_terms=%d message_bytes=%d message_lines=%d message_sha256_12=%s line_sha256_12=%s\n",
				result.MessageIndex,
				result.Role,
				result.Source,
				result.Trusted,
				result.Edited,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.MessageBytes,
				result.MessageLines,
				result.MessageSHA,
				result.LineSHA,
			)
		}
	}
	b.WriteString("\nRaw channel bodies, issue bodies, comment bodies, assistant replies, prompts, tool outputs, and raw search queries are not included. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSessionSearchActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSessionSearchActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSessionSearchIssueTarget(ev Event, req *ChannelSessionSearchActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel session search requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSessionSearchOptions(opts ChannelSessionSearchOptions) ChannelSessionSearchOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SearchID = cleanChannelSessionSearchID(opts.SearchID)
	opts.Query = cleanChannelSessionSearchQuery(opts.Query)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxResults <= 0 {
		opts.MaxResults = defaultSessionSearchMaxResults
	}
	return opts
}

func applyChannelSessionSearchRoute(cfg Config, opts ChannelSessionSearchOptions) (ChannelSessionSearchOptions, error) {
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
		Body:      "GitClaw channel session search.",
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

func validateChannelSessionSearchOptions(opts ChannelSessionSearchOptions) error {
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
	if opts.SearchID == "" {
		return fmt.Errorf("missing session search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid session search id %q", opts.SearchID)
	}
	if cleanChannelSessionSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing session search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel session search max results must be between 1 and 25")
	}
	return nil
}

func validateChannelSessionSearchActionRequestOptions(opts ChannelSessionSearchOptions) error {
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
	if opts.SearchID == "" {
		return fmt.Errorf("missing session search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid session search id %q", opts.SearchID)
	}
	if cleanChannelSessionSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing session search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel session search max results must be between 1 and 25")
	}
	return nil
}

func cleanChannelSessionSearchSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSessionSearchID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSessionSearchQuery(value string) string {
	value = cleanMemorySearchQuery(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		value = strings.TrimSpace(value[:300])
	}
	return value
}

func parseChannelSessionSearchTrailingQuery(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "query:") || strings.HasPrefix(lower, "search:") || strings.HasPrefix(lower, "recall:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelSessionSearchQuery(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelSessionSearchSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-session-search-source-%s", eventID(ev))
}

func autoChannelSessionSearchID(ev Event, opts ChannelSessionSearchOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Query}, "|")
	return cleanChannelSessionSearchID(fmt.Sprintf("session-search-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSessionSearchNotifyMessageID(ev Event, searchID string) string {
	seed := strings.Join([]string{eventID(ev), searchID}, "|")
	return fmt.Sprintf("gitclaw-channel-session-search-%s-%s", eventID(ev), shortDocumentHash(seed))
}
