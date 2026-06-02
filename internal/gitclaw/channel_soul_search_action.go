package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ChannelSoulSearchOptions struct {
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

type ChannelSoulSearchResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	SearchIDHash        string
	QueryHash           string
	Search              SoulSearchReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSoulSearchActionRequest struct {
	Options             ChannelSoulSearchOptions
	Search              SoulSearchReport
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

func IsChannelSoulSearchActionRequest(ev Event, cfg Config) bool {
	return isChannelSoulSearchActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSoulSearchActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSoulSearchSubcommand(fields[1]) {
	case "soul-search", "souls-search", "search-soul", "search-souls", "soul-recall", "authority-search", "identity-search", "policy-search", "high-authority-search":
		return true
	default:
		return false
	}
}

func BuildChannelSoulSearchActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSoulSearchActionRequest, error) {
	fields, trailing, ok := channelSoulSearchActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSoulSearchActionRequest{}, fmt.Errorf("missing channel soul search command")
	}
	req := ChannelSoulSearchActionRequest{
		Options: ChannelSoulSearchOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxResults:        defaultSoulSearchMaxResults,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSoulSearchSubcommand(fields[1]),
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
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--search-id", "--soul-search-id", "--authority-search-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SearchID = cleanChannelSoulSearchID(fields[i+1])
			i++
		case "--query", "-q":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			queryParts = append(queryParts, fields[i+1])
			req.QuerySource = "flag"
			i++
		case "--max-results", "--limit":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxResults, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 25", field)
			}
			req.Options.MaxResults = maxResults
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSoulSearchActionRequest{}, fmt.Errorf("unknown channel soul search argument %q", field)
			}
			queryParts = append(queryParts, field)
			if req.QuerySource == "" {
				req.QuerySource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = cleanChannelSoulSearchQuery(strings.Join(queryParts, " "))
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = parseChannelSoulSearchTrailingQuery(trailing)
		if req.Options.Query != "" {
			req.QuerySource = "trailing-query"
		}
	}
	if err := applyChannelSoulSearchIssueTarget(ev, &req); err != nil {
		return ChannelSoulSearchActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSoulSearchSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SearchID) == "" {
		req.Options.SearchID = autoChannelSoulSearchID(ev, req.Options)
		req.AutoSearchID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSoulSearchNotifyMessageID(ev, req.Options.SearchID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSoulSearchOptions(req.Options)
	if err := validateChannelSoulSearchActionRequestOptions(req.Options); err != nil {
		return ChannelSoulSearchActionRequest{}, err
	}
	req.Search = BuildSoulSearchReport(repoContext, req.Options.Query, req.Options.MaxResults)
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
	notificationBody := RenderChannelSoulSearchNotificationBody(req.Options, req.Search)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func RunChannelSoulSearch(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSoulSearchActionRequest, repoContext RepoContext) (ChannelSoulSearchResult, error) {
	opts := normalizeChannelSoulSearchOptions(req.Options)
	var err error
	opts, err = applyChannelSoulSearchRoute(cfg, opts)
	if err != nil {
		return ChannelSoulSearchResult{}, err
	}
	if err := validateChannelSoulSearchOptions(opts); err != nil {
		return ChannelSoulSearchResult{}, err
	}
	search := req.Search
	if search.QueryHash == "" {
		search = BuildSoulSearchReport(repoContext, opts.Query, opts.MaxResults)
	}
	body := RenderChannelSoulSearchNotificationBody(opts, search)
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
		return ChannelSoulSearchResult{}, fmt.Errorf("queue channel soul search notification: %w", err)
	}
	return ChannelSoulSearchResult{
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

func RenderChannelSoulSearchActionReport(ev Event, req ChannelSoulSearchActionRequest, result ChannelSoulSearchResult) string {
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
	b.WriteString("## GitClaw Channel Soul Search Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_soul_search_status: `%s`\n", status)
	fmt.Fprintf(&b, "- soul_search_status: `%s`\n", search.SearchStatus)
	fmt.Fprintf(&b, "- search_mode: `%s`\n", "repo-local-high-authority-lexical")
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
	fmt.Fprintf(&b, "- files_scanned: `%d`\n", search.FilesScanned)
	fmt.Fprintf(&b, "- matched_files: `%d`\n", search.MatchedFiles)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", search.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", search.ResultsReturned)
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_performed: `%t`\n", false)
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
	fmt.Fprintf(&b, "- raw_soul_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_identity_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_user_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_guidance_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_heartbeat_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_soul_file_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_soul_search_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw searched repo-local high-authority context files and queued a provider-facing soul recall result with only paths, categories, line numbers, scores, hashes, counts, and delivery metadata. The action does not call a model, execute tools, mutate repository files, write soul or memory, contact registries, export profiles, call provider APIs, or print raw soul/context bodies.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read soul-search recall results with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent recall results with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate soul-search notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSoulSearchNotificationBody(opts ChannelSoulSearchOptions, report SoulSearchReport) string {
	var b strings.Builder
	b.WriteString("GitClaw channel soul search\n\n")
	fmt.Fprintf(&b, "Search status: %s\n", report.SearchStatus)
	fmt.Fprintf(&b, "Query hash: %s\n", report.QueryHash)
	fmt.Fprintf(&b, "Query terms: %d\n", report.QueryTerms)
	fmt.Fprintf(&b, "Max results: %d\n", report.MaxResults)
	fmt.Fprintf(&b, "Files scanned: %d\n", report.FilesScanned)
	fmt.Fprintf(&b, "Matched files: %d\n", report.MatchedFiles)
	fmt.Fprintf(&b, "Matched lines: %d\n", report.MatchedLines)
	fmt.Fprintf(&b, "Results returned: %d\n", report.ResultsReturned)
	fmt.Fprintf(&b, "Search id hash: %s\n", shortDocumentHash(opts.SearchID))
	b.WriteString("\nResults:\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- path=%s category=%s line=%d score=%d matched_terms=%d file_sha256_12=%s line_sha256_12=%s\n",
				result.Path,
				result.Category,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.FileSHA,
				result.LineSHA,
			)
		}
	}
	b.WriteString("\nRaw soul, identity, user, memory, tool guidance, heartbeat, channel, issue, comment, prompt, and tool output bodies are not included. Raw search queries are not included. Model call: not performed by this action. Soul write: not performed by this action. Registry contact: not performed by this action. Profile export: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSoulSearchActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSoulSearchActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSoulSearchIssueTarget(ev Event, req *ChannelSoulSearchActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel soul search requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSoulSearchOptions(opts ChannelSoulSearchOptions) ChannelSoulSearchOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SearchID = cleanChannelSoulSearchID(opts.SearchID)
	opts.Query = cleanChannelSoulSearchQuery(opts.Query)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxResults <= 0 {
		opts.MaxResults = defaultSoulSearchMaxResults
	}
	return opts
}

func applyChannelSoulSearchRoute(cfg Config, opts ChannelSoulSearchOptions) (ChannelSoulSearchOptions, error) {
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
		Body:      "GitClaw channel soul search.",
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

func validateChannelSoulSearchOptions(opts ChannelSoulSearchOptions) error {
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
		return fmt.Errorf("missing soul search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid soul search id %q", opts.SearchID)
	}
	if cleanChannelSoulSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing soul search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel soul search max results must be between 1 and 25")
	}
	return nil
}

func validateChannelSoulSearchActionRequestOptions(opts ChannelSoulSearchOptions) error {
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
		return fmt.Errorf("missing soul search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid soul search id %q", opts.SearchID)
	}
	if cleanChannelSoulSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing soul search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel soul search max results must be between 1 and 25")
	}
	return nil
}

func cleanChannelSoulSearchSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSoulSearchID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSoulSearchQuery(value string) string {
	value = cleanMemorySearchQuery(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		value = strings.TrimSpace(value[:300])
	}
	return value
}

func parseChannelSoulSearchTrailingQuery(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "query:") || strings.HasPrefix(lower, "search:") || strings.HasPrefix(lower, "recall:") || strings.HasPrefix(lower, "soul:") || strings.HasPrefix(lower, "authority:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelSoulSearchQuery(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelSoulSearchSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-soul-search-source-%s", eventID(ev))
}

func autoChannelSoulSearchID(ev Event, opts ChannelSoulSearchOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Query}, "|")
	return cleanChannelSoulSearchID(fmt.Sprintf("soul-search-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSoulSearchNotifyMessageID(ev Event, searchID string) string {
	seed := strings.Join([]string{eventID(ev), searchID}, "|")
	return fmt.Sprintf("gitclaw-channel-soul-search-%s-%s", eventID(ev), shortDocumentHash(seed))
}
