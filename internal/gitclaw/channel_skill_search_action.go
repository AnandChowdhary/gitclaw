package gitclaw

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const defaultChannelSkillSearchMaxResults = 10

type ChannelSkillSearchOptions struct {
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

type ChannelSkillSearchReport struct {
	QueryHash         string
	QueryTerms        int
	SearchStatus      string
	MaxResults        int
	AvailableSkills   int
	EnabledSkills     int
	MatchedSkills     int
	ResultsReturned   int
	RawBodiesIncluded bool
	Results           []SkillSearchResult
}

type ChannelSkillSearchResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	SearchIDHash        string
	QueryHash           string
	Search              ChannelSkillSearchReport
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelSkillSearchActionRequest struct {
	Options               ChannelSkillSearchOptions
	Search                ChannelSkillSearchReport
	Command               string
	Subcommand            string
	AutoSourceMessageID   bool
	AutoNotifyMessageID   bool
	AutoSearchID          bool
	TargetFromIssue       bool
	QuerySource           string
	RequestedRouteHash    string
	RequestedThreadHash   string
	RequestedMsgHash      string
	NotifyMessageHash     string
	SearchIDHash          string
	QuerySHA              string
	QueryBytes            int
	QueryTerms            int
	MatchedSkillNamesHash string
	MatchedSkillPathsHash string
	MatchedSkillIndexHash string
	NotificationBodySHA   string
	NotificationBytes     int
	NotificationLines     int
}

func IsChannelSkillSearchActionRequest(ev Event, cfg Config) bool {
	return isChannelSkillSearchActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelSkillSearchActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelSkillSearchSubcommand(fields[1]) {
	case "skill-search", "skills-search", "search-skill", "search-skills", "skill-recall", "capability-search", "capabilities-search":
		return true
	default:
		return false
	}
}

func BuildChannelSkillSearchActionRequest(ev Event, cfg Config, repoContext RepoContext) (ChannelSkillSearchActionRequest, error) {
	fields, trailing, ok := channelSkillSearchActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelSkillSearchActionRequest{}, fmt.Errorf("missing channel skill search command")
	}
	req := ChannelSkillSearchActionRequest{
		Options: ChannelSkillSearchOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxResults:        defaultChannelSkillSearchMaxResults,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelSkillSearchSubcommand(fields[1]),
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
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--search-id", "--skill-search-id", "--capability-search-id", "--recall-id", "--id":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SearchID = cleanChannelSkillSearchID(fields[i+1])
			i++
		case "--query", "-q":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			queryParts = append(queryParts, fields[i+1])
			req.QuerySource = "flag"
			i++
		case "--max-results", "--limit":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxResults, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 25", field)
			}
			req.Options.MaxResults = maxResults
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelSkillSearchActionRequest{}, fmt.Errorf("unknown channel skill search argument %q", field)
			}
			queryParts = append(queryParts, field)
			if req.QuerySource == "" {
				req.QuerySource = "positional"
			}
		}
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = cleanChannelSkillSearchQuery(strings.Join(queryParts, " "))
	}
	if strings.TrimSpace(req.Options.Query) == "" {
		req.Options.Query = parseChannelSkillSearchTrailingQuery(trailing)
		if req.Options.Query != "" {
			req.QuerySource = "trailing-query"
		}
	}
	if err := applyChannelSkillSearchIssueTarget(ev, &req); err != nil {
		return ChannelSkillSearchActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelSkillSearchSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SearchID) == "" {
		req.Options.SearchID = autoChannelSkillSearchID(ev, req.Options)
		req.AutoSearchID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelSkillSearchNotifyMessageID(ev, req.Options.SearchID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelSkillSearchOptions(req.Options)
	if err := validateChannelSkillSearchActionRequestOptions(req.Options); err != nil {
		return ChannelSkillSearchActionRequest{}, err
	}
	req.Search = BuildChannelSkillSearchReport(repoContext, req.Options.Query, req.Options.MaxResults)
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
	req.MatchedSkillNamesHash = hashStringList(channelSkillSearchResultNames(req.Search.Results))
	req.MatchedSkillPathsHash = hashStringList(channelSkillSearchResultPaths(req.Search.Results))
	req.MatchedSkillIndexHash = hashStringOrNone(channelSkillSearchResultIndex(req.Search.Results))
	notificationBody := RenderChannelSkillSearchNotificationBody(req.Options, req.Search, repoContext)
	req.NotificationBodySHA = shortDocumentHash(notificationBody)
	req.NotificationBytes = len(notificationBody)
	req.NotificationLines = lineCount(notificationBody)
	return req, nil
}

func BuildChannelSkillSearchReport(repoContext RepoContext, query string, maxResults int) ChannelSkillSearchReport {
	query = cleanChannelSkillSearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultChannelSkillSearchMaxResults
	}
	report := ChannelSkillSearchReport{
		QueryHash:         shortDocumentHash(query),
		QueryTerms:        len(skillSearchTerms(query)),
		SearchStatus:      "ok",
		MaxResults:        maxResults,
		AvailableSkills:   availableSkillCount(repoContext),
		EnabledSkills:     enabledSkillCount(repoContext.SkillSummaries),
		RawBodiesIncluded: false,
	}
	if query == "" {
		report.SearchStatus = "no_query"
		return report
	}
	results := searchSkillSummaries(repoContext.SkillSummaries, query)
	report.MatchedSkills = len(results)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	report.Results = results
	report.ResultsReturned = len(results)
	if report.MatchedSkills == 0 {
		report.SearchStatus = "no_matches"
	}
	return report
}

func RunChannelSkillSearch(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelSkillSearchActionRequest, repoContext RepoContext) (ChannelSkillSearchResult, error) {
	opts := normalizeChannelSkillSearchOptions(req.Options)
	var err error
	opts, err = applyChannelSkillSearchRoute(cfg, opts)
	if err != nil {
		return ChannelSkillSearchResult{}, err
	}
	if err := validateChannelSkillSearchOptions(opts); err != nil {
		return ChannelSkillSearchResult{}, err
	}
	search := req.Search
	if search.QueryHash == "" {
		search = BuildChannelSkillSearchReport(repoContext, opts.Query, opts.MaxResults)
	}
	body := RenderChannelSkillSearchNotificationBody(opts, search, repoContext)
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
		return ChannelSkillSearchResult{}, fmt.Errorf("queue channel skill search notification: %w", err)
	}
	return ChannelSkillSearchResult{
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

func RenderChannelSkillSearchActionReport(ev Event, req ChannelSkillSearchActionRequest, result ChannelSkillSearchResult) string {
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
	matchedNamesHash := hashStringList(channelSkillSearchResultNames(search.Results))
	if matchedNamesHash == "" {
		matchedNamesHash = req.MatchedSkillNamesHash
	}
	matchedPathsHash := hashStringList(channelSkillSearchResultPaths(search.Results))
	if matchedPathsHash == "" {
		matchedPathsHash = req.MatchedSkillPathsHash
	}
	matchedIndexHash := hashStringOrNone(channelSkillSearchResultIndex(search.Results))
	if matchedIndexHash == "" {
		matchedIndexHash = req.MatchedSkillIndexHash
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Skill Search Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_skill_search_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_search_status: `%s`\n", search.SearchStatus)
	fmt.Fprintf(&b, "- search_mode: `%s`\n", "repo-local-skill-metadata-lexical")
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
	fmt.Fprintf(&b, "- skill_search_id_sha256_12: `%s`\n", noneIfEmpty(searchIDHash))
	fmt.Fprintf(&b, "- skill_search_id_auto: `%t`\n", req.AutoSearchID)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", noneIfEmpty(queryHash))
	fmt.Fprintf(&b, "- query_terms: `%d`\n", search.QueryTerms)
	fmt.Fprintf(&b, "- query_bytes: `%d`\n", req.QueryBytes)
	fmt.Fprintf(&b, "- query_source: `%s`\n", noneIfEmpty(req.QuerySource))
	fmt.Fprintf(&b, "- max_results: `%d`\n", search.MaxResults)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", search.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", search.EnabledSkills)
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", search.MatchedSkills)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", search.ResultsReturned)
	fmt.Fprintf(&b, "- matched_skill_names_sha256_12: `%s`\n", noneIfEmpty(matchedNamesHash))
	fmt.Fprintf(&b, "- matched_skill_paths_sha256_12: `%s`\n", noneIfEmpty(matchedPathsHash))
	fmt.Fprintf(&b, "- matched_skill_index_sha256_12: `%s`\n", noneIfEmpty(matchedIndexHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(notificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", notificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", notificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- progressive_disclosure_enabled: `%t`\n", true)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_query_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_search_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_names_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_skill_search_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw searched repo-local skill metadata and queued a provider-facing skill recall result with names, paths, hashes, requirement counts, and match metadata. The source receipt keeps raw skill names, paths, descriptions, bodies, search queries, and channel bodies out of band. The action does not call a model, install or update skills, contact registries, run installers, mutate repository files, execute tools, or call provider APIs.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read skill-search results with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent skill-search results with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate skill-search notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func RenderChannelSkillSearchNotificationBody(opts ChannelSkillSearchOptions, report ChannelSkillSearchReport, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("GitClaw channel skill search\n\n")
	fmt.Fprintf(&b, "Search status: %s\n", report.SearchStatus)
	fmt.Fprintf(&b, "Query hash: %s\n", report.QueryHash)
	fmt.Fprintf(&b, "Query terms: %d\n", report.QueryTerms)
	fmt.Fprintf(&b, "Max results: %d\n", report.MaxResults)
	fmt.Fprintf(&b, "Available skills: %d\n", report.AvailableSkills)
	fmt.Fprintf(&b, "Enabled skills: %d\n", report.EnabledSkills)
	fmt.Fprintf(&b, "Matched skills: %d\n", report.MatchedSkills)
	fmt.Fprintf(&b, "Results returned: %d\n", report.ResultsReturned)
	fmt.Fprintf(&b, "Search id hash: %s\n", shortDocumentHash(opts.SearchID))
	b.WriteString("\nResults:\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			skill := result.Skill
			fmt.Fprintf(&b, "- skill_name=%s path=%s folder=%s score=%d match_fields=%s enabled=%t selected_for_this_turn=%t always=%t frontmatter=%t description_present=%t bytes=%d lines=%d sha256_12=%s requires_env=%d requires_bins=%d missing_env=%d missing_bins=%d\n",
				skill.Name,
				skill.Path,
				skillFolderName(skill.Path),
				result.Score,
				inlineListOrNone(result.MatchFields),
				skillIsEnabled(skill),
				skillSelectedForTurn(repoContext, skill),
				skill.Always,
				skill.FrontmatterPresent,
				strings.TrimSpace(skill.Description) != "",
				skill.Bytes,
				skill.Lines,
				skill.SHA,
				len(skill.RequiredEnv),
				len(skill.RequiredBins),
				len(skill.MissingEnv),
				len(skill.MissingBins),
			)
		}
	}
	b.WriteString("\nRaw skill bodies, skill descriptions, channel bodies, issue bodies, comment bodies, prompts, tool outputs, and raw search queries are not included. Skill install: not performed by this action. Skill update: not performed by this action. Registry contact: not performed by this action. Installer scripts: not run by this action. Model call: not performed by this action. Repository mutation: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func channelSkillSearchActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelSkillSearchActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelSkillSearchIssueTarget(ev Event, req *ChannelSkillSearchActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel skill search requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelSkillSearchOptions(opts ChannelSkillSearchOptions) ChannelSkillSearchOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SearchID = cleanChannelSkillSearchID(opts.SearchID)
	opts.Query = cleanChannelSkillSearchQuery(opts.Query)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxResults <= 0 {
		opts.MaxResults = defaultChannelSkillSearchMaxResults
	}
	return opts
}

func applyChannelSkillSearchRoute(cfg Config, opts ChannelSkillSearchOptions) (ChannelSkillSearchOptions, error) {
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
		Body:      "GitClaw channel skill search.",
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

func validateChannelSkillSearchOptions(opts ChannelSkillSearchOptions) error {
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
		return fmt.Errorf("missing skill search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid skill search id %q", opts.SearchID)
	}
	if cleanChannelSkillSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing skill search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel skill search max results must be between 1 and 25")
	}
	return nil
}

func validateChannelSkillSearchActionRequestOptions(opts ChannelSkillSearchOptions) error {
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
		return fmt.Errorf("missing skill search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid skill search id %q", opts.SearchID)
	}
	if cleanChannelSkillSearchQuery(opts.Query) == "" {
		return fmt.Errorf("missing skill search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel skill search max results must be between 1 and 25")
	}
	return nil
}

func cleanChannelSkillSearchSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelSkillSearchID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelSkillSearchQuery(value string) string {
	value = cleanSkillSearchQuery(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		value = strings.TrimSpace(value[:300])
	}
	return value
}

func parseChannelSkillSearchTrailingQuery(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "query:") || strings.HasPrefix(lower, "search:") || strings.HasPrefix(lower, "skill:") || strings.HasPrefix(lower, "skills:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelSkillSearchQuery(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelSkillSearchSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-skill-search-source-%s", eventID(ev))
}

func autoChannelSkillSearchID(ev Event, opts ChannelSkillSearchOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Query}, "|")
	return cleanChannelSkillSearchID(fmt.Sprintf("skill-search-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelSkillSearchNotifyMessageID(ev Event, searchID string) string {
	seed := strings.Join([]string{eventID(ev), searchID}, "|")
	return fmt.Sprintf("gitclaw-channel-skill-search-%s-%s", eventID(ev), shortDocumentHash(seed))
}

func channelSkillSearchResultNames(results []SkillSearchResult) []string {
	var names []string
	for _, result := range results {
		if strings.TrimSpace(result.Skill.Name) != "" {
			names = append(names, result.Skill.Name)
		}
	}
	return uniqueSortedStrings(names)
}

func channelSkillSearchResultPaths(results []SkillSearchResult) []string {
	var paths []string
	for _, result := range results {
		if strings.TrimSpace(result.Skill.Path) != "" {
			paths = append(paths, result.Skill.Path)
		}
	}
	return uniqueSortedStrings(paths)
}

func channelSkillSearchResultIndex(results []SkillSearchResult) string {
	var lines []string
	for _, result := range results {
		skill := result.Skill
		lines = append(lines, fmt.Sprintf("%s|%s|%s|%d|%s", skill.Name, skill.Path, skill.SHA, result.Score, inlineList(result.MatchFields)))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(uniqueSortedStrings(lines), "\n")
}
