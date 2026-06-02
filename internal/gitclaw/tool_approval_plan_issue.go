package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const toolApprovalPlanIssueMarker = "gitclaw:tool-approval-plan-issue"

type ToolApprovalPlanIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type ToolApprovalPlanIssueRequest struct {
	Repo                 string
	Command              string
	Subcommand           string
	ApprovalPlanID       string
	RequestedTool        string
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
	ApprovalRequired     bool
	RunAllowedNow        bool
	ApprovalDecision     string
	PlanStatus           string
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

type ToolApprovalPlanIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func BuildToolApprovalPlanIssueRequest(ev Event, cfg Config, repoContext RepoContext, toolName, approvalPlanID, sourceKind string) (ToolApprovalPlanIssueRequest, error) {
	requestedTool := cleanToolLookupName(toolName)
	if requestedTool == "" {
		return ToolApprovalPlanIssueRequest{}, fmt.Errorf("missing tool approval plan target")
	}
	sourceText := activeRequestText(ev)
	approvalPlanID = cleanToolApprovalPlanID(approvalPlanID)
	if approvalPlanID == "" {
		approvalPlanID = fmt.Sprintf("tool-approval-%s", shortDocumentHash(sourceText))
	}
	if !skillNamePattern.MatchString(approvalPlanID) {
		return ToolApprovalPlanIssueRequest{}, fmt.Errorf("invalid tool approval plan id %q", approvalPlanID)
	}

	normalized := normalizeToolLookupName(requestedTool)
	matches := matchingToolContracts(toolReportContracts, requestedTool)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	findings := toolApprovalPlanFindings(requestedTool, matches, repoContext, validation)
	enabled, disabled, blocked := false, false, false
	mode := ""
	trigger := ""
	mutating := false
	matchedTool := ""
	if len(matches) == 1 {
		match := matches[0]
		matchedTool = match.Name
		mode = match.Mode
		trigger = match.Trigger
		mutating = isMutatingToolContract(match)
		enabled, disabled, blocked = toolEnabledInRepoContext(match.Name, repoContext)
	}
	if strings.TrimSpace(sourceKind) == "" {
		sourceKind = "issue"
	}
	req := ToolApprovalPlanIssueRequest{
		Repo:                 ev.Repo,
		Command:              "/tools",
		Subcommand:           "approval-plan",
		ApprovalPlanID:       approvalPlanID,
		RequestedTool:        requestedTool,
		RequestedToolSHA:     shortDocumentHash(requestedTool),
		RequestedToolTerms:   len(memorySearchTerms(requestedTool)),
		NormalizedTool:       normalized,
		MatchedTool:          matchedTool,
		MatchedTools:         len(matches),
		ActiveOutputsForTool: len(activeOutputs),
		AvailableTools:       len(toolReportContracts),
		ToolEnabled:          enabled,
		DisabledByConfig:     disabled,
		BlockedByAllowlist:   blocked,
		ToolMode:             mode,
		ToolTrigger:          trigger,
		MutatingContract:     mutating,
		ApprovalRequired:     len(matches) == 1 && mutating,
		RunAllowedNow:        len(matches) == 1 && enabled && !mutating && validation.Errors == 0,
		ApprovalDecision:     toolApprovalPlanDecision(requestedTool, matches, enabled, disabled, blocked, mutating, validation),
		PlanStatus:           toolApprovalPlanStatus(findings),
		ValidationStatus:     validation.Status,
		ValidationErrors:     validation.Errors,
		ValidationWarnings:   validation.Warnings,
		SourceIssueNumber:    ev.Issue.Number,
		SourceSHA:            shortDocumentHash(sourceText),
		SourceBytes:          len(sourceText),
		SourceLines:          lineCount(sourceText),
		SourceKind:           sourceKind,
	}
	if ev.Comment != nil {
		req.SourceCommentID = ev.Comment.ID
	}
	return req, nil
}

func RunToolApprovalPlanIssue(ctx context.Context, cfg Config, github ToolApprovalPlanIssueGitHubClient, req ToolApprovalPlanIssueRequest, repoContext RepoContext) (ToolApprovalPlanIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return ToolApprovalPlanIssueResult{}, err
	}
	if req.ApprovalPlanID == "" {
		return ToolApprovalPlanIssueResult{}, fmt.Errorf("missing tool approval plan id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return ToolApprovalPlanIssueResult{}, fmt.Errorf("list tool approval plan issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if toolApprovalPlanIssueMatches(issue.Body, req.ApprovalPlanID) {
			return ToolApprovalPlanIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, toolApprovalPlanIssueTitle(req), RenderToolApprovalPlanIssueBody(cfg, repoContext, req), []string{cfg.TriggerLabel})
	if err != nil {
		return ToolApprovalPlanIssueResult{}, fmt.Errorf("create tool approval plan issue: %w", err)
	}
	return ToolApprovalPlanIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderToolApprovalPlanIssueBody(cfg Config, repoContext RepoContext, req ToolApprovalPlanIssueRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" normalized_tool=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", toolApprovalPlanIssueMarker, escapeMarkerValue(req.ApprovalPlanID), escapeMarkerValue(req.NormalizedTool), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw tool approval plan issue.\n\n")
	fmt.Fprintf(&b, "- approval_plan_id: %s\n", req.ApprovalPlanID)
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
	fmt.Fprintf(&b, "- approval_required: %t\n", req.ApprovalRequired)
	fmt.Fprintf(&b, "- approval_decision: %s\n", req.ApprovalDecision)
	fmt.Fprintf(&b, "- run_allowed_now: %t\n", req.RunAllowedNow)
	fmt.Fprintf(&b, "- approval_plan_status: %s\n", req.PlanStatus)
	fmt.Fprintf(&b, "- tool_validation_status: %s\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	b.WriteString("- approval_mode: github-issue-dry-run\n")
	b.WriteString("- model_call_performed: false\n")
	b.WriteString("- tool_execution_performed: false\n")
	b.WriteString("- approval_granted: false\n")
	b.WriteString("- repository_mutation_performed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_tool_inputs_included: false\n")
	b.WriteString("- raw_tool_outputs_included: false\n")
	b.WriteString("- raw_approval_payloads_included: false\n\n")
	b.WriteString("Continue here to review the approval boundary in a normal GitClaw conversation. This issue records the dry-run gates only; it does not approve, execute, or mutate anything.\n\n")
	b.WriteString("### Approval Plan Snapshot\n\n")
	planEvent := Event{
		Repo: req.Repo,
		Issue: Issue{
			Number: req.SourceIssueNumber,
			Title:  "GitClaw channel tool approval source " + req.SourceSHA,
		},
	}
	b.WriteString(renderToolApprovalPlanReport(planEvent, cfg, repoContext, req.RequestedTool, true))
	return strings.TrimSpace(b.String())
}

func RenderToolApprovalPlanIssueActionReport(ev Event, req ToolApprovalPlanIssueRequest, result ToolApprovalPlanIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	var b strings.Builder
	b.WriteString("## GitClaw Tool Approval Plan Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_tool_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- tool_approval_plan_issue_status: `%s`\n", status)
	fmt.Fprintf(&b, "- approval_plan_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- approval_plan_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- approval_plan_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- approval_plan_id_sha256_12: `%s`\n", shortDocumentHash(req.ApprovalPlanID))
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", req.RequestedToolSHA)
	fmt.Fprintf(&b, "- requested_tool_terms: `%d`\n", req.RequestedToolTerms)
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", req.MatchedTools)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", req.AvailableTools)
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", req.ActiveOutputsForTool)
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", req.ToolEnabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", req.DisabledByConfig)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", req.BlockedByAllowlist)
	fmt.Fprintf(&b, "- tool_mode_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.ToolMode))
	fmt.Fprintf(&b, "- tool_trigger_sha256_12: `%s`\n", toolRunRequestHashOrNone(req.ToolTrigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", req.MutatingContract)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", req.ApprovalRequired)
	fmt.Fprintf(&b, "- approval_decision: `%s`\n", req.ApprovalDecision)
	fmt.Fprintf(&b, "- run_allowed_now: `%t`\n", req.RunAllowedNow)
	fmt.Fprintf(&b, "- approval_plan_status: `%s`\n", req.PlanStatus)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", req.ValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", req.ValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", req.ValidationWarnings)
	fmt.Fprintf(&b, "- approval_mode: `%s`\n", "github-issue-dry-run")
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_granted: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_approval_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_approval_plan_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a GitHub issue for reviewing one tool approval dry-run. The action does not approve, execute tools, call a model, or mutate the repository.\n\n")
	b.WriteString("### Approval Review Path\n")
	fmt.Fprintf(&b, "- continue on approval plan issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up to inspect the repo-backed tool boundary\n")
	return strings.TrimSpace(b.String())
}

func cleanToolApprovalPlanID(id string) string {
	return cleanToolRehearsalID(id)
}

func toolApprovalPlanIssueMatches(body, approvalPlanID string) bool {
	return strings.Contains(body, "<!-- "+toolApprovalPlanIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanToolApprovalPlanID(approvalPlanID))))
}

func toolApprovalPlanIssueTitle(req ToolApprovalPlanIssueRequest) string {
	toolName := valueOrNone(req.NormalizedTool)
	if req.MatchedTool != "" {
		toolName = req.MatchedTool
	}
	title := "GitClaw tool approval plan: " + toolName
	if req.ApprovalPlanID != "" {
		title += " (" + req.ApprovalPlanID + ")"
	}
	if len(title) > 120 {
		title = title[:120]
	}
	return title
}
