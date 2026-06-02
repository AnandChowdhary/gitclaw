package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelToolSearchOptions struct {
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

type ChannelToolSearchResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	SearchIDHash        string
	QueryHash           string
	Search              ToolSearchReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelToolSearchActionRequest struct {
	Options              ChannelToolSearchOptions
	Search               ToolSearchReport
	Command              string
	Subcommand           string
	AutoSourceMessageID  bool
	AutoNotifyMessageID  bool
	AutoSearchID         bool
	TargetFromIssue      bool
	QuerySource          string
	RequestedRouteHash   string
	RequestedThreadHash  string
	RequestedMsgHash     string
	NotifyMessageHash    string
	SearchIDHash         string
	QuerySHA             string
	QueryBytes           int
	QueryTerms           int
	MatchedToolNamesHash string
	MatchedToolIndexHash string
	NotificationBodySHA  string
	NotificationBytes    int
	NotificationLines    int
}

func IsChannelToolSearchActionRequest(ev Event, cfg Config) bool {
	return isChannelToolSearchActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelToolSearchActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelToolSearchSubcommand(fields[1]) {
	case "tool-search", "tools-search", "search-tool", "search-tools", "tool-recall", "tool-capability-search", "tool-capabilities-search":
		return true
	default:
		return false
	}
}

func BuildChannelToolSearchActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelToolSearchActionRequest, error) {
	fields, trailing, ok := channelToolSearchActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelToolSearchActionRequest{}, fmt.Errorf("missing channel tool search command")
	}
	req := ChannelToolSearchActionRequest{
		Options: ChannelToolSearchOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxResults:        defaultToolSearchMaxResults,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelToolSearchSubcommand(fields[1]),
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
				return ChannelToolSearchActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--search-id", "--tool-search-id", "--capability-search-id", "--recall-id", "--id":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SearchID = cleanChannelToolSearchID(fields[i+1])
			i++
		case "--query", "-q":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			queryParts = append(queryParts, fields[i+1])
			req.QuerySource = "flag"
			i++
		case "--max-results", "--limit":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxResults, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 25", field)
			}
			req.Options.MaxResults = maxResults
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelToolSearchActionRequest{}, fmt.Errorf("unknown channel tool search argument %q", field)
			}
			queryParts = append(queryParts, field)
			if req.QuerySource == "" {
				req.QuerySource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = cleanChannelToolSearchQuery(strings.Join(queryParts, " "))
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = parseChannelToolSearchTrailingQuery(trailing)
		if req.Options.Query != "" {
			req.QuerySource = "trailing-query"
		}
	}
	if err := applyChannelToolSearchIssueTarget(ev, &req); err != nil {
		return ChannelToolSearchActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelToolSearchSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SearchID) == "" {
		req.Options.SearchID = autoChannelToolSearchID(ev, req.Options)
		req.AutoSearchID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelToolSearchNotifyMessageID(ev, req.Options.SearchID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelToolSearchOptions(req.Options)
	if err := validateChannelToolSearchActionRequestOptions(req.Options); err != nil {
		return ChannelToolSearchActionRequest{}, err
	}
	req.Search = BuildToolSearchReport(repoContext, req.Options.Query, req.Options.MaxResults)
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
	req.MatchedToolNamesHash = hashStringList(channelToolSearchResultNames(req.Search.Results))
	req.MatchedToolIndexHash = hashStringOrNone(channelToolSearchResultIndex(req.Search.Results))
	notificationBody := RenderChannelToolSearchNotificationBody(req.Options, req.Search)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelToolSearch(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelToolSearchActionRequest, repoContext RepoContext) (ChannelToolSearchResult, error) {
	opts := normalizeChannelToolSearchOptions(req.Options)
	var err error
	opts, err = applyChannelToolSearchRoute(cfg, opts)
	if err != nil {
		return ChannelToolSearchResult{}, err
	}
	if err := validateChannelToolSearchOptions(opts); err != nil {
		return ChannelToolSearchResult{}, err
	}
	search := req.Search
	if search.QueryHash == "" {
		search = BuildToolSearchReport(repoContext, opts.Query, opts.MaxResults)
	}
	body := RenderChannelToolSearchNotificationBody(opts, search)
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
		return ChannelToolSearchResult{}, fmt.Errorf("queue channel tool search notification: %w", err)
	}
	return ChannelToolSearchResult{
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

func RenderChannelToolSearchActionReport(ev Event, req ChannelToolSearchActionRequest, result ChannelToolSearchResult) string {
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
	matchedNamesHash := hashStringList(channelToolSearchResultNames(search.Results))
	if matchedNamesHash == "" {
		matchedNamesHash = req.MatchedToolNamesHash
	}
	matchedIndexHash := hashStringOrNone(channelToolSearchResultIndex(search.Results))
	if matchedIndexHash == "" {
		matchedIndexHash = req.MatchedToolIndexHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Tool Search Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_tool_search_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_search_status: `%s`\n", search.SearchStatus)
	fmt.Fprintf(&b, "- search_mode: `%s`\n", "deterministic-tool-contracts-local-lexical")
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
	fmt.Fprintf(&b, "- tool_search_id_sha256_12: `%s`\n", noneIfEmpty(searchIDHash))
	fmt.Fprintf(&b, "- tool_search_id_auto: `%t`\n", req.AutoSearchID)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", noneIfEmpty(queryHash))
	fmt.Fprintf(&b, "- query_terms: `%d`\n", search.QueryTerms)
	fmt.Fprintf(&b, "- query_bytes: `%d`\n", req.QueryBytes)
	fmt.Fprintf(&b, "- query_source: `%s`\n", noneIfEmpty(req.QuerySource))
	fmt.Fprintf(&b, "- max_results: `%d`\n", search.MaxResults)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", search.AvailableTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", search.ActiveOutputs)
	fmt.Fprintf(&b, "- matched_contracts: `%d`\n", search.MatchedContracts)
	fmt.Fprintf(&b, "- matched_outputs: `%d`\n", search.MatchedOutputs)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", search.ResultsReturned)
	fmt.Fprintf(&b, "- matched_tool_names_sha256_12: `%s`\n", noneIfEmpty(matchedNamesHash))
	fmt.Fprintf(&b, "- matched_tool_index_sha256_12: `%s`\n", noneIfEmpty(matchedIndexHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- tool_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_server_launch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- toolset_activation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_query_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_search_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_triggers_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_tool_search_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw searched deterministic tool contracts and active tool-output metadata, then queued a provider-facing capability search result. The source receipt keeps raw tool names, triggers, schemas, inputs, outputs, search queries, and channel bodies out of band. The action does not call a model, execute tools, launch MCP servers, activate toolsets, mutate repository files, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read tool-search results with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent tool-search results with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate tool-search notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelToolSearchNotificationBody(opts ChannelToolSearchOptions, report ToolSearchReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel tool search\n\n")
	fmt.Fprintf(&b, "Search status: %s\n", report.SearchStatus)
	fmt.Fprintf(&b, "Query hash: %s\n", report.QueryHash)
	fmt.Fprintf(&b, "Query terms: %d\n", report.QueryTerms)
	fmt.Fprintf(&b, "Max results: %d\n", report.MaxResults)
	fmt.Fprintf(&b, "Available tools: %d\n", report.AvailableTools)
	fmt.Fprintf(&b, "Active tool outputs: %d\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "Matched contracts: %d\n", report.MatchedContracts)
	fmt.Fprintf(&b, "Matched outputs: %d\n", report.MatchedOutputs)
	fmt.Fprintf(&b, "Results returned: %d\n", report.ResultsReturned)
	fmt.Fprintf(&b, "Search id hash: %s\n", shortDocumentHash(opts.SearchID))
	b.WriteString("\nResults:\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			if result.Kind == "contract" {
				fmt.Fprintf(&b, "- kind=contract name=%s score=%d match_fields=%s enabled=%t disabled_by_config=%t blocked_by_allowlist=%t mode=%s trigger_sha256_12=%s\n",
					result.Name,
					result.Score,
					inlineListOrNone(result.MatchFields),
					result.Enabled,
					result.Disabled,
					result.Blocked,
					result.Mode,
					shortDocumentHash(result.Trigger),
				)
				continue
			}
			fmt.Fprintf(&b, "- kind=active-output name=%s score=%d match_fields=%s input_sha256_12=%s output_bytes=%d output_lines=%d output_sha256_12=%s\n",
				result.Name,
				result.Score,
				inlineListOrNone(result.MatchFields),
				noneIfEmpty(result.InputSHA),
				result.OutputBytes,
				result.OutputLines,
				noneIfEmpty(result.OutputSHA),
			)
		}
	}
	b.WriteString("\nRaw tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, and raw search queries are not included. Tool execution: not performed by this action. Shell execution: not performed by this action. MCP server launch: not performed by this action. Toolset activation: not performed by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelToolSearchActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelToolSearchActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelToolSearchIssueTarget(ev Event, req *ChannelToolSearchActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel tool search requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelToolSearchOptions(opts ChannelToolSearchOptions) ChannelToolSearchOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SearchID = cleanChannelToolSearchID(opts.SearchID)
	opts.Query = cleanChannelToolSearchQuery(opts.Query)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxResults <= 0 {
		opts.MaxResults = defaultToolSearchMaxResults
	}
	return opts
}

func applyChannelToolSearchRoute(cfg Config, opts ChannelToolSearchOptions) (ChannelToolSearchOptions, error) {
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
		Body:      "GitClaw channel tool search.",
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

func validateChannelToolSearchOptions(opts ChannelToolSearchOptions) error {
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
		return fmt.Errorf("missing tool search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid tool search id %q", opts.SearchID)
	}
	if cleanChannelToolSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing tool search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel tool search max results must be between 1 and 25")
	}
	return nil
}

func validateChannelToolSearchActionRequestOptions(opts ChannelToolSearchOptions) error {
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
		return fmt.Errorf("missing tool search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid tool search id %q", opts.SearchID)
	}
	if cleanChannelToolSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing tool search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel tool search max results must be between 1 and 25")
	}
	return nil
}

func cleanChannelToolSearchSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelToolSearchID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelToolSearchQuery(value string) string {
	value = cleanMemorySearchQuery(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		value = strings.TrimSpace(value[:300])
	}
	return value
}

func parseChannelToolSearchTrailingQuery(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "query:") || strings.HasPrefix(lower, "search:") || strings.HasPrefix(lower, "tool:") || strings.HasPrefix(lower, "tools:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelToolSearchQuery(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelToolSearchSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-tool-search-source-%s", eventID(ev))
}

func autoChannelToolSearchID(ev Event, opts ChannelToolSearchOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Query}, "|")
	return cleanChannelToolSearchID(fmt.Sprintf("tool-search-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelToolSearchNotifyMessageID(ev Event, searchID string) string {
	seed := strings.Join([]string{eventID(ev), searchID}, "|")
	return fmt.Sprintf("gitclaw-channel-tool-search-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelToolSearchResultNames(results []ToolSearchResult) []string {
	var names []string
	for _, result := range results {
		if strings.TrimSpace(result.Name) != "" {
			names = append(names, result.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelToolSearchResultIndex(results []ToolSearchResult) string {
	var lines []string
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%d|%s|%s|%s", result.Kind, result.Name, result.Mode, result.Score, inlineList(result.MatchFields), result.InputSHA, result.OutputSHA))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}
