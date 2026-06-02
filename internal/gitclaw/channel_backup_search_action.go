package gitclaw

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ChannelBackupSearchOptions struct {
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

type ChannelBackupSearchResult struct {
	Notification        ChannelSendResult
	RouteName           string
	RouteHash           string
	Channel             string
	ThreadHash          string
	MessageHash         string
	NotifyHash          string
	SearchIDHash        string
	QueryHash           string
	Search              BackupSearchReport
	BackupFetchStatus   string
	BackupRootHash      string
	SearchErrorKind     string
	SearchErrorHash     string
	NotificationBodySHA string
	NotificationBytes   int
	NotificationLines   int
}

type ChannelBackupSearchActionRequest struct {
	Options             ChannelBackupSearchOptions
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
}

func IsChannelBackupSearchActionRequest(ev Event, cfg Config) bool {
	return isChannelBackupSearchActionFields(activeSlashCommandFields(ev, cfg))
}

func isChannelBackupSearchActionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/channel" && fields[0] != "/channels" {
		return false
	}
	switch cleanChannelBackupSearchSubcommand(fields[1]) {
	case "backup-search", "search-backup", "search-backups", "backup-recall", "recovery-search", "archive-search":
		return true
	default:
		return false
	}
}

func BuildChannelBackupSearchActionRequest(ev Event, cfg Config) (ChannelBackupSearchActionRequest, error) {
	fields, trailing, ok := channelBackupSearchActionFieldsAndTrailingBody(ev, cfg)
	if !ok {
		return ChannelBackupSearchActionRequest{}, fmt.Errorf("missing channel backup search command")
	}
	req := ChannelBackupSearchActionRequest{
		Options: ChannelBackupSearchOptions{
			Repo:              ev.Repo,
			SourceIssueNumber: ev.Issue.Number,
			MaxResults:        defaultBackupSearchMaxResults,
		},
		Command:    strings.ToLower(strings.Trim(fields[0], " \t\r\n.,:;!?")),
		Subcommand: cleanChannelBackupSearchSubcommand(fields[1]),
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
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("--route requires a value")
			}
			req.Options.Route = fields[i+1]
			i++
		case "--channel", "-c":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("--channel requires a value")
			}
			req.Options.Channel = fields[i+1]
			i++
		case "--thread-id", "--thread":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("--thread-id requires a value")
			}
			req.Options.ThreadID = fields[i+1]
			i++
		case "--message-id", "--source-message-id", "--target-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SourceMessageID = fields[i+1]
			i++
		case "--notify-message-id", "--notification-message-id":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.NotifyMessageID = fields[i+1]
			i++
		case "--search-id", "--backup-search-id", "--recall-id", "--id":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.Options.SearchID = cleanChannelBackupSearchID(fields[i+1])
			i++
		case "--query", "-q":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			queryParts = append(queryParts, fields[i+1])
			req.QuerySource = "flag"
			i++
		case "--max-results", "--limit":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("%s requires a value", field)
			}
			maxResults, err := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if err != nil {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("%s must be an integer from 1 to 25", field)
			}
			req.Options.MaxResults = maxResults
			i++
		case "--author":
			if i+1 >= len(fields) {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("--author requires a value")
			}
			req.Options.Author = fields[i+1]
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ChannelBackupSearchActionRequest{}, fmt.Errorf("unknown channel backup search argument %q", field)
			}
			queryParts = append(queryParts, field)
			if req.QuerySource == "" {
				req.QuerySource = "positional"
			}
		}
	}
	req.Options.Query = cleanChannelBackupSearchQuery(strings.Join(queryParts, " "))
	if req.Options.Query == "" {
		req.Options.Query = parseChannelBackupSearchTrailingQuery(trailing)
		if req.Options.Query != "" {
			req.QuerySource = "trailing-query"
		}
	}
	if err := applyChannelBackupSearchIssueTarget(ev, &req); err != nil {
		return ChannelBackupSearchActionRequest{}, err
	}
	if strings.TrimSpace(req.Options.SourceMessageID) == "" {
		req.Options.SourceMessageID = autoChannelBackupSearchSourceMessageID(ev)
		req.AutoSourceMessageID = true
	}
	if strings.TrimSpace(req.Options.SearchID) == "" {
		req.Options.SearchID = autoChannelBackupSearchID(ev, req.Options)
		req.AutoSearchID = true
	}
	if strings.TrimSpace(req.Options.NotifyMessageID) == "" {
		req.Options.NotifyMessageID = autoChannelBackupSearchNotifyMessageID(ev, req.Options.SearchID)
		req.AutoNotifyMessageID = true
	}
	req.Options = normalizeChannelBackupSearchOptions(req.Options)
	if err := validateChannelBackupSearchActionRequestOptions(req.Options); err != nil {
		return ChannelBackupSearchActionRequest{}, err
	}
	req.RequestedRouteHash = channelRouteHash(req.Options.Route)
	if req.Options.ThreadID != "" {
		req.RequestedThreadHash = shortDocumentHash(req.Options.ThreadID)
	}
	req.RequestedMsgHash = shortDocumentHash(req.Options.SourceMessageID)
	req.NotifyMessageHash = shortDocumentHash(req.Options.NotifyMessageID)
	req.SearchIDHash = shortDocumentHash(req.Options.SearchID)
	req.QuerySHA = shortDocumentHash(req.Options.Query)
	req.QueryBytes = len(req.Options.Query)
	req.QueryTerms = len(memorySearchTerms(req.Options.Query))
	return req, nil
}

func RunChannelBackupSearch(ctx context.Context, cfg Config, github ChannelSendGitHubClient, req ChannelBackupSearchActionRequest) (ChannelBackupSearchResult, error) {
	opts := normalizeChannelBackupSearchOptions(req.Options)
	var err error
	opts, err = applyChannelBackupSearchRoute(cfg, opts)
	if err != nil {
		return ChannelBackupSearchResult{}, err
	}
	if err := validateChannelBackupSearchOptions(opts); err != nil {
		return ChannelBackupSearchResult{}, err
	}
	search, backupRoot, fetchStatus, searchErr := loadChannelBackupSearchReport(ctx, cfg, opts)
	errorKind := ""
	errorHash := ""
	if searchErr != nil {
		errorKind = channelBackupSearchErrorKind(searchErr)
		errorHash = shortDocumentHash(searchErr.Error())
	}
	body := renderChannelBackupSearchNotificationBody(opts, search, fetchStatus, errorKind)
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
		return ChannelBackupSearchResult{}, fmt.Errorf("queue channel backup search notification: %w", err)
	}
	return ChannelBackupSearchResult{
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
		BackupFetchStatus:   fetchStatus,
		BackupRootHash:      shortDocumentHash(backupRoot),
		SearchErrorKind:     errorKind,
		SearchErrorHash:     errorHash,
		NotificationBodySHA: shortDocumentHash(body),
		NotificationBytes:   len(body),
		NotificationLines:   lineCount(body),
	}, nil
}

func RenderChannelBackupSearchActionReport(ev Event, req ChannelBackupSearchActionRequest, result ChannelBackupSearchResult) string {
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
	search := result.Search
	if search.QueryHash == "" {
		search = unavailableBackupSearchReport(req.Options.Repo, req.Options.Query, req.Options.MaxResults, "")
	}
	var b strings.Builder
	b.WriteString("## GitClaw Channel Backup Search Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- requested_channel_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- channel_backup_search_status: `%s`\n", status)
	fmt.Fprintf(&b, "- backup_search_status: `%s`\n", search.SearchStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", search.BackupVerifyStatus)
	fmt.Fprintf(&b, "- backup_fetch_status: `%s`\n", noneIfEmpty(result.BackupFetchStatus))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- search_mode: `%s`\n", "gitclaw-backups-local-lexical")
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
	fmt.Fprintf(&b, "- backup_root_sha256_12: `%s`\n", noneIfEmpty(result.BackupRootHash))
	fmt.Fprintf(&b, "- repo_backup_dir_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(search.RepoDir)))
	fmt.Fprintf(&b, "- index_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(search.IndexPath)))
	fmt.Fprintf(&b, "- readme_path_sha256_12: `%s`\n", noneIfEmpty(shortDocumentHash(search.ReadmePath)))
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", search.SchemaVersion)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", search.IssueCount)
	fmt.Fprintf(&b, "- issue_fields_searched: `%d`\n", search.IssueFieldsSearched)
	fmt.Fprintf(&b, "- comment_bodies_searched: `%d`\n", search.CommentBodiesSearched)
	fmt.Fprintf(&b, "- transcript_messages_searched: `%d`\n", search.TranscriptMessagesSearched)
	fmt.Fprintf(&b, "- matched_issues: `%d`\n", search.MatchedIssues)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", search.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", search.ResultsReturned)
	fmt.Fprintf(&b, "- search_error_kind: `%s`\n", noneIfEmpty(result.SearchErrorKind))
	fmt.Fprintf(&b, "- search_error_sha256_12: `%s`\n", noneIfEmpty(result.SearchErrorHash))
	fmt.Fprintf(&b, "- notification_body_sha256_12: `%s`\n", noneIfEmpty(result.NotificationBodySHA))
	fmt.Fprintf(&b, "- notification_body_bytes: `%d`\n", result.NotificationBytes)
	fmt.Fprintf(&b, "- notification_body_lines: `%d`\n", result.NotificationLines)
	fmt.Fprintf(&b, "- target_from_current_channel_issue: `%t`\n", req.TargetFromIssue)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_write_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_delivery_strategy: `%s`\n", "channel-outbox + channel-delivery")
	fmt.Fprintf(&b, "- raw_query_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_thread_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_notify_message_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_search_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_root_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_paths_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_backup_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_channel_message_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_transcript_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_channel_backup_search_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw searched the fetched gitclaw-backups archive and queued provider-facing recall metadata. This action may fetch the backup branch read-only when the local backup root is absent, but it does not write the backup branch, restore files, replay GitHub APIs, call a model, call provider APIs, or print raw backup payloads.\n\n")
	b.WriteString("### Follow-Up Delivery\n")
	b.WriteString("- provider gateways read backup-search results with `gitclaw channel-outbox --channel <provider> --account-id <account> --issue-number <issue> --out <file>`\n")
	b.WriteString("- provider gateways record sent backup-search results with `gitclaw channel-delivery --channel <provider> --account-id <account> --issue-number <issue> --comment-id <comment> --external-message-id <message>`\n")
	b.WriteString("- duplicate backup-search notifications are suppressed by `channel + notify_message_id`\n")
	return strings.TrimSpace(b.String())
}

func renderChannelBackupSearchNotificationBody(opts ChannelBackupSearchOptions, report BackupSearchReport, fetchStatus, errorKind string) string {
	var b strings.Builder
	b.WriteString("GitClaw channel backup search\n\n")
	fmt.Fprintf(&b, "Backup search status: %s\n", report.SearchStatus)
	fmt.Fprintf(&b, "Backup verify status: %s\n", report.BackupVerifyStatus)
	fmt.Fprintf(&b, "Backup branch: %s\n", defaultBackupBranch)
	fmt.Fprintf(&b, "Backup fetch status: %s\n", fetchStatus)
	if errorKind != "" {
		fmt.Fprintf(&b, "Search error kind: %s\n", errorKind)
	}
	fmt.Fprintf(&b, "Query hash: %s\n", report.QueryHash)
	fmt.Fprintf(&b, "Query terms: %d\n", report.QueryTerms)
	fmt.Fprintf(&b, "Max results: %d\n", report.MaxResults)
	fmt.Fprintf(&b, "Issue count: %d\n", report.IssueCount)
	fmt.Fprintf(&b, "Issue fields searched: %d\n", report.IssueFieldsSearched)
	fmt.Fprintf(&b, "Comment bodies searched: %d\n", report.CommentBodiesSearched)
	fmt.Fprintf(&b, "Transcript messages searched: %d\n", report.TranscriptMessagesSearched)
	fmt.Fprintf(&b, "Matched issues: %d\n", report.MatchedIssues)
	fmt.Fprintf(&b, "Matched lines: %d\n", report.MatchedLines)
	fmt.Fprintf(&b, "Results returned: %d\n", report.ResultsReturned)
	fmt.Fprintf(&b, "Search id hash: %s\n", shortDocumentHash(opts.SearchID))
	b.WriteString("\nResults:\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- issue=#%d path=%s source=%s role=%s trusted=%t line=%d score=%d matched_terms=%d body_bytes=%d body_lines=%d body_sha256_12=%s line_sha256_12=%s generated_at=%s event=%s\n",
				result.IssueNumber,
				result.Path,
				result.Source,
				result.Role,
				result.Trusted,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.BodyBytes,
				result.BodyLines,
				result.BodySHA,
				result.LineSHA,
				result.BackupGeneratedAt,
				result.EventName,
			)
		}
	}
	b.WriteString("\nRaw backup payloads, channel bodies, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw search queries are not included. Model call: not performed by this action. Repository mutation: not performed by this action. Backup branch write: not performed by this action. Provider delivery: queued through GitHub channel outbox.")
	return strings.TrimSpace(b.String())
}

func loadChannelBackupSearchReport(ctx context.Context, cfg Config, opts ChannelBackupSearchOptions) (BackupSearchReport, string, string, error) {
	localRoot := channelBackupSearchLocalRoot(cfg)
	if channelBackupSearchIndexExists(localRoot, opts.Repo) {
		report, err := BuildBackupSearch(localRoot, opts.Repo, opts.Query, opts.MaxResults)
		if err != nil {
			return unavailableBackupSearchReport(opts.Repo, opts.Query, opts.MaxResults, localRoot), localRoot, "local_error", err
		}
		return report, localRoot, "local", nil
	}
	worktree, cleanup, err := fetchChannelBackupSearchWorktree(ctx, cfg)
	if err != nil {
		return unavailableBackupSearchReport(opts.Repo, opts.Query, opts.MaxResults, localRoot), localRoot, "unavailable", err
	}
	defer cleanup()
	fetchedRoot := filepath.Join(worktree, defaultBackupRoot)
	report, err := BuildBackupSearch(fetchedRoot, opts.Repo, opts.Query, opts.MaxResults)
	if err != nil {
		return unavailableBackupSearchReport(opts.Repo, opts.Query, opts.MaxResults, fetchedRoot), fetchedRoot, "fetched_error", err
	}
	return report, fetchedRoot, "fetched", nil
}

func channelBackupSearchLocalRoot(cfg Config) string {
	root := defaultBackupRoot
	if strings.TrimSpace(cfg.Workdir) != "" {
		root = filepath.Join(cfg.Workdir, defaultBackupRoot)
	}
	return root
}

func channelBackupSearchIndexExists(root, repo string) bool {
	if root == "" || repo == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(backupRepoDir(root, repo), "index.json"))
	return err == nil
}

func fetchChannelBackupSearchWorktree(ctx context.Context, cfg Config) (string, func(), error) {
	workdir := strings.TrimSpace(cfg.Workdir)
	if workdir == "" {
		workdir = "."
	}
	tempDir, err := os.MkdirTemp("", "gitclaw-channel-backup-search-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp backup worktree: %w", err)
	}
	worktree := filepath.Join(tempDir, "backup-branch")
	cleanup := func() {
		_ = runChannelBackupSearchGit(context.Background(), workdir, "worktree", "remove", "--force", worktree)
		_ = os.RemoveAll(tempDir)
	}
	if err := runChannelBackupSearchGit(ctx, workdir, "fetch", "--depth=1", "origin", defaultBackupBranch); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("fetch backup branch: %w", err)
	}
	if err := runChannelBackupSearchGit(ctx, workdir, "worktree", "add", "--detach", worktree, "FETCH_HEAD"); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("create backup worktree: %w", err)
	}
	return worktree, cleanup, nil
}

func runChannelBackupSearchGit(ctx context.Context, workdir string, args ...string) error {
	cmdArgs := append([]string{"-C", workdir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

func unavailableBackupSearchReport(repo, query string, maxResults int, root string) BackupSearchReport {
	query = cleanChannelBackupSearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultBackupSearchMaxResults
	}
	return BackupSearchReport{
		Root:               filepath.ToSlash(root),
		Repo:               repo,
		QueryHash:          shortDocumentHash(query),
		QueryTerms:         len(memorySearchTerms(query)),
		SearchStatus:       "unavailable",
		MaxResults:         maxResults,
		BackupVerifyStatus: "unavailable",
		RawBodiesIncluded:  false,
	}
}

func channelBackupSearchErrorKind(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "fetch backup branch"):
		return "backup_branch_fetch_failed"
	case strings.Contains(text, "create backup worktree"):
		return "backup_worktree_failed"
	case strings.Contains(text, "read backup index"):
		return "backup_index_unavailable"
	case strings.Contains(text, "parse backup index"):
		return "backup_index_invalid"
	case strings.Contains(text, "backup index repo"):
		return "backup_index_repo_mismatch"
	default:
		return "backup_search_failed"
	}
}

func channelBackupSearchActionFieldsAndTrailingBody(ev Event, cfg Config) ([]string, string, bool) {
	lines := strings.Split(activeRequestText(ev), "\n")
	for i, line := range lines {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if !isChannelBackupSearchActionFields(fields) {
			continue
		}
		return fields, strings.Join(lines[i+1:], "\n"), true
	}
	return nil, "", false
}

func applyChannelBackupSearchIssueTarget(ev Event, req *ChannelBackupSearchActionRequest) error {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.Options.Route) != "" || strings.TrimSpace(req.Options.Channel) != "" || strings.TrimSpace(req.Options.ThreadID) != "" {
		return nil
	}
	channel, threadID := channelThreadMarkerFields(ev.Issue.Body)
	if channel == "" || threadID == "" {
		return fmt.Errorf("channel backup search requires a gitclaw:channel-thread issue or an explicit route/channel/thread target")
	}
	req.Options.Channel = channel
	req.Options.ThreadID = threadID
	req.TargetFromIssue = true
	return nil
}

func normalizeChannelBackupSearchOptions(opts ChannelBackupSearchOptions) ChannelBackupSearchOptions {
	opts.Repo = strings.TrimSpace(opts.Repo)
	opts.Route = cleanChannelRouteName(opts.Route)
	opts.Channel = strings.ToLower(strings.TrimSpace(opts.Channel))
	opts.ThreadID = strings.TrimSpace(opts.ThreadID)
	opts.SourceMessageID = strings.TrimSpace(opts.SourceMessageID)
	opts.NotifyMessageID = strings.TrimSpace(opts.NotifyMessageID)
	opts.SearchID = cleanChannelBackupSearchID(opts.SearchID)
	opts.Query = cleanChannelBackupSearchQuery(opts.Query)
	opts.Author = strings.TrimSpace(opts.Author)
	if opts.MaxResults <= 0 {
		opts.MaxResults = defaultBackupSearchMaxResults
	}
	return opts
}

func applyChannelBackupSearchRoute(cfg Config, opts ChannelBackupSearchOptions) (ChannelBackupSearchOptions, error) {
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
		Body:      "GitClaw channel backup search.",
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

func validateChannelBackupSearchOptions(opts ChannelBackupSearchOptions) error {
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
		return fmt.Errorf("missing backup search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid backup search id %q", opts.SearchID)
	}
	if opts.Query == "" {
		return fmt.Errorf("missing backup search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel backup search max results must be between 1 and 25")
	}
	return nil
}

func validateChannelBackupSearchActionRequestOptions(opts ChannelBackupSearchOptions) error {
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
		return fmt.Errorf("missing backup search id")
	}
	if !skillNamePattern.MatchString(opts.SearchID) {
		return fmt.Errorf("invalid backup search id %q", opts.SearchID)
	}
	if opts.Query == "" {
		return fmt.Errorf("missing backup search query")
	}
	if opts.MaxResults < 1 || opts.MaxResults > 25 {
		return fmt.Errorf("channel backup search max results must be between 1 and 25")
	}
	return nil
}

func cleanChannelBackupSearchSubcommand(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n.,:;!?`\"'"))
	return strings.ReplaceAll(value, "_", "-")
}

func cleanChannelBackupSearchID(value string) string {
	return cleanChannelHuddleID(value)
}

func cleanChannelBackupSearchQuery(value string) string {
	value = cleanMemorySearchQuery(value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		value = strings.TrimSpace(value[:300])
	}
	return value
}

func parseChannelBackupSearchTrailingQuery(trailing string) string {
	for _, line := range strings.Split(strings.TrimSpace(trailing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "query:") || strings.HasPrefix(lower, "search:") || strings.HasPrefix(lower, "recall:") {
			if idx := strings.Index(trimmed, ":"); idx >= 0 {
				return cleanChannelBackupSearchQuery(trimmed[idx+1:])
			}
		}
	}
	return ""
}

func autoChannelBackupSearchSourceMessageID(ev Event) string {
	return fmt.Sprintf("gitclaw-channel-backup-search-source-%s", eventID(ev))
}

func autoChannelBackupSearchID(ev Event, opts ChannelBackupSearchOptions) string {
	seed := strings.Join([]string{eventID(ev), opts.Route, opts.Channel, opts.ThreadID, opts.SourceMessageID, opts.Query}, "|")
	return cleanChannelBackupSearchID(fmt.Sprintf("backup-search-%s-%s", eventID(ev), shortDocumentHash(seed)))
}

func autoChannelBackupSearchNotifyMessageID(ev Event, searchID string) string {
	seed := strings.Join([]string{eventID(ev), searchID}, "|")
	return fmt.Sprintf("gitclaw-channel-backup-search-%s-%s", eventID(ev), shortDocumentHash(seed))
}
