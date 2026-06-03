package gitclaw

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const toolRunCancelMarker = "gitclaw:tool-run-cancel"

var toolRunRequestIssueMarkerPattern = regexp.MustCompile(`<!--\s*gitclaw:tool-run-request-issue\s+([^>]*)-->`)

type ToolRunCancelGitHubClient interface {
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
	ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error)
	PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error)
	CloseIssue(ctx context.Context, repo string, issueNumber int) error
}

type ToolRunCancelRequest struct {
	Repo              string
	Command           string
	Subcommand        string
	RequestID         string
	RequestIDAuto     bool
	SourceIssueNumber int
	SourceCommentID   int64
	SourceKind        string
	SourceSHA         string
	SourceBytes       int
	SourceLines       int
}

type ToolRunCancelResult struct {
	RequestIssueNumber int
	RequestIssueURL    string
	CancelCommentID    int64
	Cancelled          bool
	Closed             bool
	Duplicate          bool
	NotFoundOrClosed   bool
}

func IsToolRunCancelRequest(ev Event, cfg Config) bool {
	return isToolRunCancelFields(activeSlashCommandFields(ev, cfg))
}

func isToolRunCancelFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/tools" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "cancel-run", "cancel-request", "close-run", "close-request", "close-run-request", "reject-run", "reject-request":
		return true
	default:
		return false
	}
}

func BuildToolRunCancelRequest(ev Event, cfg Config) (ToolRunCancelRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isToolRunCancelFields(fields) {
		return ToolRunCancelRequest{}, fmt.Errorf("missing tool run cancel command")
	}
	req := ToolRunCancelRequest{
		Repo:              ev.Repo,
		Command:           fields[0],
		Subcommand:        strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		SourceIssueNumber: ev.Issue.Number,
		SourceKind:        "issue",
		SourceSHA:         shortDocumentHash(activeRequestText(ev)),
		SourceBytes:       len(activeRequestText(ev)),
		SourceLines:       lineCount(activeRequestText(ev)),
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	for i := 2; i < len(fields); i++ {
		field := strings.TrimSpace(fields[i])
		switch field {
		case "--id", "--request-id":
			if i+1 >= len(fields) {
				return ToolRunCancelRequest{}, fmt.Errorf("%s requires a value", field)
			}
			req.RequestID = cleanToolRunRequestID(fields[i+1])
			i++
		default:
			if strings.HasPrefix(field, "--") {
				return ToolRunCancelRequest{}, fmt.Errorf("unknown tool run cancel argument %q", field)
			}
			if req.RequestID == "" {
				req.RequestID = cleanToolRunRequestID(field)
				continue
			}
			return ToolRunCancelRequest{}, fmt.Errorf("unexpected tool run cancel argument %q", field)
		}
	}
	if req.RequestID == "" {
		req.RequestID = cleanToolRunRequestID(toolRunRequestIssueID(ev.Issue.Body))
		req.RequestIDAuto = req.RequestID != ""
	}
	if req.RequestID == "" {
		return ToolRunCancelRequest{}, fmt.Errorf("missing tool run request id")
	}
	if !skillNamePattern.MatchString(req.RequestID) {
		return ToolRunCancelRequest{}, fmt.Errorf("invalid tool run request id %q", req.RequestID)
	}
	return req, nil
}

func RunToolRunCancel(ctx context.Context, github ToolRunCancelGitHubClient, req ToolRunCancelRequest) (ToolRunCancelResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return ToolRunCancelResult{}, err
	}
	if req.RequestID == "" {
		return ToolRunCancelResult{}, fmt.Errorf("missing tool run request id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, nil, 300)
	if err != nil {
		return ToolRunCancelResult{}, fmt.Errorf("list tool run request issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest || !toolRunRequestIssueMatches(issue.Body, req.RequestID) {
			continue
		}
		result := ToolRunCancelResult{
			RequestIssueNumber: issue.Number,
			RequestIssueURL:    issueURL(req.Repo, issue.Number),
		}
		comments, err := github.ListIssueComments(ctx, req.Repo, issue.Number)
		if err != nil {
			return ToolRunCancelResult{}, fmt.Errorf("list tool run request comments: %w", err)
		}
		for _, comment := range comments {
			if toolRunCancelMatches(comment.Body, req.RequestID) {
				result.Duplicate = true
				if err := github.CloseIssue(ctx, req.Repo, issue.Number); err != nil {
					return ToolRunCancelResult{}, fmt.Errorf("close already-cancelled tool run request issue: %w", err)
				}
				result.Closed = true
				return result, nil
			}
		}
		posted, err := github.PostIssueComment(ctx, req.Repo, issue.Number, RenderToolRunCancelComment(req))
		if err != nil {
			return ToolRunCancelResult{}, fmt.Errorf("post tool run cancel comment: %w", err)
		}
		if err := github.CloseIssue(ctx, req.Repo, issue.Number); err != nil {
			return ToolRunCancelResult{}, fmt.Errorf("close tool run request issue: %w", err)
		}
		result.CancelCommentID = posted.ID
		result.Cancelled = true
		result.Closed = true
		return result, nil
	}
	return ToolRunCancelResult{NotFoundOrClosed: true}, nil
}

func RenderToolRunCancelComment(req ToolRunCancelRequest) string {
	return fmt.Sprintf(`<!-- %s id="%s" source_issue="%d" source_comment_id="%d" source_sha256_12="%s" -->
GitClaw tool run request cancellation.

- source_issue: #%d
- source_comment_id: %d
- source_kind: %s
- source_sha256_12: %s
- model_call_performed: false
- tool_execution_performed: false
- approval_granted: false
- repository_mutation_performed: false
- raw_source_body_included: false
- raw_tool_inputs_included: false
- raw_tool_outputs_included: false`, toolRunCancelMarker, escapeMarkerValue(req.RequestID), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA), req.SourceIssueNumber, req.SourceCommentID, req.SourceKind, req.SourceSHA)
}

func RenderToolRunCancelActionReport(ev Event, req ToolRunCancelRequest, result ToolRunCancelResult) string {
	status := "cancelled"
	if result.Duplicate {
		status = "already_cancelled"
	}
	if result.NotFoundOrClosed {
		status = "not_found_or_closed"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Tool Run Cancel Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_tool_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- tool_run_cancel_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_run_request_issue: `#%d`\n", result.RequestIssueNumber)
	fmt.Fprintf(&b, "- tool_run_request_issue_url: `%s`\n", result.RequestIssueURL)
	fmt.Fprintf(&b, "- cancel_comment_id: `%d`\n", result.CancelCommentID)
	fmt.Fprintf(&b, "- cancellation_comment_posted: `%t`\n", result.CancelCommentID != 0)
	fmt.Fprintf(&b, "- issue_closed: `%t`\n", result.Closed)
	fmt.Fprintf(&b, "- cancellation_duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- request_not_found_or_closed: `%t`\n", result.NotFoundOrClosed)
	fmt.Fprintf(&b, "- request_id_sha256_12: `%s`\n", shortDocumentHash(req.RequestID))
	fmt.Fprintf(&b, "- request_id_auto_from_current_issue: `%t`\n", req.RequestIDAuto)
	fmt.Fprintf(&b, "- cancellation_store: `%s`\n", "github-issue-comment-plus-closed-state")
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_granted: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_request_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_run_cancel_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw cancelled an open reviewed tool-run request issue when one was found. This does not approve or execute the requested tool, call a model, copy raw source text, or mutate repository files.\n\n")
	b.WriteString("### Review Path\n")
	b.WriteString("- cancellation is durable as a request-issue comment plus closed GitHub issue state\n")
	b.WriteString("- a missing request means the request id was not open in the current GitHub issue store\n")
	return strings.TrimSpace(b.String())
}

func toolRunCancelMatches(body, requestID string) bool {
	return strings.Contains(body, "<!-- "+toolRunCancelMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(requestID)))
}

func toolRunRequestIssueID(body string) string {
	match := toolRunRequestIssueMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return markerAttribute(match[1], "id")
}
