package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const toolRehearsalIssueMarker = "gitclaw:tool-rehearsal-issue"

type ToolRehearsalIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type ToolRehearsalIssueRequest struct {
	Repo                 string
	Command              string
	Subcommand           string
	RehearsalID          string
	RequestedToolSHA     string
	RequestedToolTerms   int
	NormalizedTool       string
	MatchedTool          string
	MatchedTools         int
	ActiveOutputsForTool int
	AvailableTools       int
	ToolEnabled          bool
	DisabledByConfig     bool
	BlockedByAllowlist   bool
	ToolMode             string
	ToolTrigger          string
	MutatingContract     bool
	ValidationStatus     string
	ValidationErrors     int
	ValidationWarnings   int
	SourceIssueNumber    int
	SourceCommentID      int64
	SourceSHA            string
	SourceBytes          int
	SourceLines          int
	SourceKind           string
}

type ToolRehearsalIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsToolRehearsalIssueRequest(ev Event, cfg Config) bool {
	return isToolRehearsalIssueFields(activeSlashCommandFields(ev, cfg))
}

func isToolRehearsalIssueFields(fields []string) bool {
	if len(fields) < 2 || fields[0] != "/tools" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "rehearse", "rehearsal", "try", "trial", "practice", "lab", "sandbox":
		return true
	default:
		return false
	}
}

func BuildToolRehearsalIssueRequest(ev Event, cfg Config, repoContext RepoContext) (ToolRehearsalIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isToolRehearsalIssueFields(fields) {
		return ToolRehearsalIssueRequest{}, fmt.Errorf("missing tool rehearsal command")
	}
	sourceText := activeRequestText(ev)
	toolName, rehearsalID, err := parseToolRehearsalIssueArgs(fields[2:], sourceText)
	if err != nil {
		return ToolRehearsalIssueRequest{}, err
	}
	requestedTool := cleanToolLookupName(toolName)
	if requestedTool == "" {
		return ToolRehearsalIssueRequest{}, fmt.Errorf("missing tool rehearsal target")
	}
	if rehearsalID == "" {
		rehearsalID = fmt.Sprintf("tool-rehearsal-%s", shortDocumentHash(sourceText))
	}
	if !skillNamePattern.MatchString(rehearsalID) {
		return ToolRehearsalIssueRequest{}, fmt.Errorf("invalid tool rehearsal id %q", rehearsalID)
	}

	normalized := normalizeToolLookupName(requestedTool)
	matches := matchingToolContracts(toolReportContracts, requestedTool)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)

	req := ToolRehearsalIssueRequest{
		Repo:                 ev.Repo,
		Command:              fields[0],
		Subcommand:           strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")),
		RehearsalID:          rehearsalID,
		RequestedToolSHA:     shortDocumentHash(requestedTool),
		RequestedToolTerms:   len(memorySearchTerms(requestedTool)),
		NormalizedTool:       normalized,
		MatchedTools:         len(matches),
		ActiveOutputsForTool: len(activeOutputs),
		AvailableTools:       len(toolReportContracts),
		ValidationStatus:     validation.Status,
		ValidationErrors:     validation.Errors,
		ValidationWarnings:   validation.Warnings,
		SourceIssueNumber:    ev.Issue.Number,
		SourceSHA:            shortDocumentHash(sourceText),
		SourceBytes:          len(sourceText),
		SourceLines:          lineCount(sourceText),
		SourceKind:           "issue",
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	if len(matches) == 1 {
		match := matches[0]
		req.MatchedTool = match.Name
		req.ToolMode = match.Mode
		req.ToolTrigger = match.Trigger
		req.MutatingContract = isMutatingToolContract(match)
		req.ToolEnabled, req.DisabledByConfig, req.BlockedByAllowlist = toolEnabledInRepoContext(match.Name, repoContext)
	}
	return req, nil
}

func RunToolRehearsalIssue(ctx context.Context, cfg Config, github ToolRehearsalIssueGitHubClient, req ToolRehearsalIssueRequest) (ToolRehearsalIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return ToolRehearsalIssueResult{}, err
	}
	if req.RehearsalID == "" {
		return ToolRehearsalIssueResult{}, fmt.Errorf("missing tool rehearsal id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return ToolRehearsalIssueResult{}, fmt.Errorf("list tool rehearsal issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if toolRehearsalIssueMatches(issue.Body, req.RehearsalID) {
			return ToolRehearsalIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, toolRehearsalIssueTitle(req), RenderToolRehearsalIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return ToolRehearsalIssueResult{}, fmt.Errorf("create tool rehearsal issue: %w", err)
	}
	return ToolRehearsalIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderToolRehearsalIssueBody(req ToolRehearsalIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" normalized_tool=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", toolRehearsalIssueMarker, escapeMarkerValue(req.RehearsalID), escapeMarkerValue(req.NormalizedTool), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw tool rehearsal issue.\n\n")
	fmt.Fprintf(&b, "- rehearsal_id: %s\n", req.RehearsalID)
	fmt.Fprintf(&b, "- normalized_tool: %s\n", valueOrNone(req.NormalizedTool))
	fmt.Fprintf(&b, "- matched_tool: %s\n", valueOrNone(req.MatchedTool))
	fmt.Fprintf(&b, "- matched_tools: %d\n", req.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: %d\n", req.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: %d\n", req.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: %t\n", req.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: %t\n", req.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: %t\n", req.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode: %s\n", valueOrNone(req.ToolMode))
	fmt.Fprintf(&b, "- mutating_contract: %t\n", req.MutatingContract)
	fmt.Fprintf(&b, "- tool_validation_status: %s\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	b.WriteString("- rehearsal_mode: github-issue-conversation\n")
	b.WriteString("- tool_execution_performed: false\n")
	b.WriteString("- tool_inputs_generated: false\n")
	b.WriteString("- tool_run_request_created: false\n")
	b.WriteString("- repository_mutation_performed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_tool_inputs_included: false\n")
	b.WriteString("- raw_tool_outputs_included: false\n\n")
	b.WriteString("Use this issue to rehearse the current tool contract in a normal GitClaw conversation. If the discussion turns into an execution request, open a reviewed `/tools request-run` issue instead; this issue is only for trying the conversation boundary.\n")
	return strings.TrimSpace(b.String())
}

func RenderToolRehearsalIssueActionReport(ev Event, req ToolRehearsalIssueRequest, result ToolRehearsalIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Tool Rehearsal Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_tool_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- tool_rehearsal_status: `%s`\n", status)
	fmt.Fprintf(&b, "- rehearsal_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- rehearsal_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- tool_rehearsal_id_sha256_12: `%s`\n", shortDocumentHash(req.RehearsalID))
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", req.RequestedToolSHA)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- normalized_tool: `%s`\n", valueOrNone(req.NormalizedTool))
	fmt.Fprintf(&b, "- matched_tool: `%s`\n", valueOrNone(req.MatchedTool))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", req.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", req.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", req.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", req.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", req.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", req.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode: `%s`\n", valueOrNone(req.ToolMode))
	fmt.Fprintf(&b, "- tool_trigger_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.ToolTrigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", req.MutatingContract)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", req.ValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", req.ValidationWarnings)
	fmt.Fprintf(&b, "- rehearsal_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- rehearsal_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_inputs_generated: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_run_request_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_rehearsal_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for trying a reviewed tool contract in a normal conversation. The action does not execute tools, generate tool inputs, create a run request, call a model, or mutate the repository; continue on the rehearsal issue to exercise prompt-visible tool behavior with GitHub Models.\n\n")
	b.WriteString("### Rehearsal Path\n")
	fmt.Fprintf(&b, "- continue on rehearsal issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up that mentions the tool contract or asks for a bounded repository search\n")
	b.WriteString("- verify the follow-up assistant marker includes prompt context, selected skill, prompt-visible tools, and usage telemetry\n")
	return strings.TrimSpace(b.String())
}

func parseToolRehearsalIssueArgs(args []string, sourceText string) (string, string, error) {
	toolName := ""
	rehearsalID := ""
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--tool", "-t":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--tool requires a value")
			}
			toolName = cleanToolLookupName(args[i])
		case "--id":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--id requires a value")
			}
			rehearsalID = cleanToolRehearsalID(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", "", fmt.Errorf("unknown tool rehearsal argument %q", field)
			}
			if toolName == "" {
				toolName = cleanToolLookupName(field)
			}
		}
	}
	if rehearsalID == "" {
		rehearsalID = fmt.Sprintf("tool-rehearsal-%s", shortDocumentHash(sourceText))
	}
	return toolName, rehearsalID, nil
}

func cleanToolRehearsalID(id string) string {
	id = strings.ToLower(strings.Trim(strings.TrimSpace(id), " \t\r\n.,:;!?`\"'"))
	id = strings.ReplaceAll(id, "_", "-")
	return id
}

func toolRehearsalIssueTitle(req ToolRehearsalIssueRequest) string {
	toolName := valueOrNone(req.NormalizedTool)
	if req.MatchedTool != "" {
		toolName = req.MatchedTool
	}
	title := "GitClaw tool rehearsal: " + toolName
	if req.RehearsalID != "" {
		title += " (" + req.RehearsalID + ")"
	}
	if len(title) > 120 {
		return strings.TrimSpace(title[:120])
	}
	return title
}

func toolRehearsalIssueMatches(body, rehearsalID string) bool {
	return strings.Contains(body, "<!-- "+toolRehearsalIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanToolRehearsalID(rehearsalID))))
}
