package gitclaw

import (
	"fmt"
	"strings"
)

type toolMapStep struct {
	IssueCommand string
	CLICommand   string
	Purpose      string
}

type toolMapFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderToolMapCLIReport(repoContext RepoContext, name string) string {
	return renderToolMapReport(Event{}, repoContext, name, false)
}

func renderToolMapReport(ev Event, repoContext RepoContext, name string, includeIssue bool) string {
	requested := cleanToolLookupName(name)
	normalized := normalizeToolLookupName(requested)
	matches := matchingToolContracts(toolReportContracts, requested)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	risk := BuildToolRiskReport(repoContext)
	findings := toolMapFindings(requested, matches, repoContext, validation)
	status := toolMapStatus(findings)
	steps := toolMapSteps(normalized)

	enabled, disabled, blocked := false, false, false
	mode := ""
	trigger := ""
	mutating := false
	if len(matches) == 1 {
		enabled, disabled, blocked = toolEnabledInRepoContext(matches[0].Name, repoContext)
		mode = matches[0].Mode
		trigger = matches[0].Trigger
		mutating = isMutatingToolContract(matches[0])
	}
	approvalRequired := len(matches) == 1 && mutating
	runAllowedNow := len(matches) == 1 && enabled && !mutating && validation.Errors == 0

	var b strings.Builder
	b.WriteString("## GitClaw Tool Map Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_map_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_map_mode: `%s`\n", "issue-native-safe-tool-sequence")
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", shortDocumentHash(requested))
	fmt.Fprintf(&b, "- normalized_tool: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", enabledToolCount(repoContext))
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", disabledToolCount(repoContext))
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", allowlistBlockedToolCount(repoContext))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", len(activeOutputs))
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", enabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", disabled)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", blocked)
	fmt.Fprintf(&b, "- tool_mode: `%s`\n", mode)
	fmt.Fprintf(&b, "- tool_trigger: `%s`\n", inlineCode(trigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", mutating)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", approvalRequired)
	fmt.Fprintf(&b, "- run_allowed_now: `%t`\n", runAllowedNow)
	fmt.Fprintf(&b, "- tool_map_step_count: `%d`\n", len(steps))
	fmt.Fprintf(&b, "- tool_map_steps_sha256_12: `%s`\n", shortDocumentHash(toolMapStepManifest(steps)))
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_launch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- network_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- rehearsal_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- run_request_issue_created: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comments_included: `%t`\n", false)
	writeToolsValidationSummary(&b, validation)
	writeToolRiskSummary(&b, risk)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_map_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps one deterministic GitClaw tool contract into a safe review sequence. It does not execute tools, launch MCP servers, create approval/rehearsal/run-request issues, call a model, make network calls, mutate workflows, mutate repository files, or dump raw tool inputs, outputs, issue bodies, comments, prompts, credentials, or secret values.\n\n")

	b.WriteString("### Matched Tool\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		activeCounts := toolActiveOutputCounts(repoContext.ToolOutputs)
		for _, contract := range matches {
			writeToolInfoContract(&b, contract, activeCounts[contract.Name], repoContext)
		}
	}

	b.WriteString("\n### Safe Tool Sequence\n")
	writeToolMapSteps(&b, steps)

	b.WriteString("\n### Active Outputs For Tool\n")
	writeToolInfoActiveOutputs(&b, activeOutputs)

	b.WriteString("\n### Safety Gates\n")
	writeToolMapSafetyGates(&b, len(matches), enabled, disabled, blocked, mutating, validation, risk)

	b.WriteString("\n### Findings\n")
	writeToolMapFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedToolMapName(ev Event, cfg Config) string {
	firstCandidate := ""
	for _, line := range strings.Split(activeRequestText(ev), "\n") {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if len(fields) < 2 || fields[0] != "/tools" {
			continue
		}
		switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
		case "map", "tool-map", "path", "runbook", "safe-tool", "sequence":
		default:
			continue
		}
		candidate := "__missing__"
		if len(fields) >= 3 {
			candidate = cleanToolLookupName(fields[2])
		}
		if firstCandidate == "" {
			firstCandidate = candidate
		}
		if candidate == "__missing__" {
			continue
		}
		if len(matchingToolContracts(toolReportContracts, candidate)) == 1 {
			return candidate
		}
	}
	return firstCandidate
}

func toolMapSteps(normalizedTool string) []toolMapStep {
	toolArg := "<tool>"
	if normalizedTool != "" {
		toolArg = normalizedTool
	}
	return []toolMapStep{
		{IssueCommand: "/tools list", CLICommand: "gitclaw tools list", Purpose: "inventory deterministic tool contracts and current enablement"},
		{IssueCommand: fmt.Sprintf("/tools search %s", toolArg), CLICommand: fmt.Sprintf("gitclaw tools search %s", toolArg), Purpose: "find the intended contract and related active-output metadata"},
		{IssueCommand: fmt.Sprintf("/tools info %s", toolArg), CLICommand: fmt.Sprintf("gitclaw tools info %s", toolArg), Purpose: "inspect the focused contract without raw tool inputs or outputs"},
		{IssueCommand: fmt.Sprintf("/tools approval-plan %s", toolArg), CLICommand: fmt.Sprintf("gitclaw tools approval-plan %s", toolArg), Purpose: "review approval, allowlist, mode, and validation gates"},
		{IssueCommand: fmt.Sprintf("/tools run-plan %s", toolArg), CLICommand: fmt.Sprintf("gitclaw tools run-plan %s", toolArg), Purpose: "dry-run the read-only execution boundary before any real model conversation"},
		{IssueCommand: fmt.Sprintf("/tools request-run %s --id <request-id>", toolArg), CLICommand: "issue-only reviewed request", Purpose: "open a reviewed GitHub request issue only when a human wants a durable request"},
	}
}

func toolMapStepManifest(steps []toolMapStep) string {
	var lines []string
	for _, step := range steps {
		lines = append(lines, strings.Join([]string{step.IssueCommand, step.CLICommand, step.Purpose}, "|"))
	}
	return strings.Join(lines, "\n")
}

func writeToolMapSteps(b *strings.Builder, steps []toolMapStep) {
	if len(steps) == 0 {
		b.WriteString("- none\n")
		return
	}
	for i, step := range steps {
		fmt.Fprintf(b, "%d. issue=`%s` cli=`%s` purpose=`%s`\n", i+1, inlineCode(step.IssueCommand), inlineCode(step.CLICommand), inlineCode(step.Purpose))
	}
}

func writeToolMapSafetyGates(b *strings.Builder, matched int, enabled, disabled, blocked, mutating bool, validation ToolValidationReport, risk ToolRiskReport) {
	contractStatus := "matched"
	if matched == 0 {
		contractStatus = "not_found"
	} else if matched > 1 {
		contractStatus = "ambiguous"
	}
	fmt.Fprintf(b, "- gate=`tool_contract` status=`%s` matched_tools=`%d`\n", contractStatus, matched)
	fmt.Fprintf(b, "- gate=`config_enabled` status=`%s` disabled_by_config=`%t`\n", gateStatus(enabled && !disabled), disabled)
	fmt.Fprintf(b, "- gate=`allowlist` status=`%s` blocked_by_allowlist=`%t`\n", gateStatus(!blocked), blocked)
	fmt.Fprintf(b, "- gate=`tool_mode` status=`%s` mutating_contract=`%t`\n", toolMapModeGateStatus(matched, mutating), mutating)
	fmt.Fprintf(b, "- gate=`validation` status=`%s` errors=`%d` warnings=`%d`\n", validation.Status, validation.Errors, validation.Warnings)
	fmt.Fprintf(b, "- gate=`risk` status=`%s` high=`%d` warning=`%d`\n", risk.Status, risk.HighRiskFindings, risk.WarningRiskFindings)
	fmt.Fprintf(b, "- gate=`model_call` status=`not_performed`\n")
	fmt.Fprintf(b, "- gate=`tool_execution` status=`not_performed`\n")
	fmt.Fprintf(b, "- gate=`shell_exec` status=`disabled`\n")
	fmt.Fprintf(b, "- gate=`mcp_launch` status=`disabled`\n")
	fmt.Fprintf(b, "- gate=`repository_mutation` status=`disabled`\n")
}

func toolMapModeGateStatus(matched int, mutating bool) string {
	if matched != 1 {
		return "not_applicable"
	}
	if mutating {
		return "blocked_mutating_contract"
	}
	return "read_only_or_metadata_only"
}

func toolMapFindings(requested string, matches []toolContract, repoContext RepoContext, validation ToolValidationReport) []toolMapFinding {
	var findings []toolMapFinding
	add := func(severity, code, detail string) {
		findings = append(findings, toolMapFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "openclaw_tool_sequence_mapped", "tool use is staged as discover, inspect, approve, dry-run, then optionally request")
	add("info", "hermes_issue_native_review_boundary", "durable tool requests stay in GitHub issues rather than sockets or hidden server state")
	add("info", "tool_execution_disabled", "this map does not execute tools, launch MCP servers, call models, or mutate repositories")
	if requested == "" || requested == "__missing__" {
		add("error", "tool_missing", "provide one GitClaw tool name")
	}
	if requested != "" && len(matches) == 0 {
		add("error", "tool_not_found", "requested tool does not match a known GitClaw tool contract")
	}
	if len(matches) > 1 {
		add("error", "tool_ambiguous", "requested tool matched multiple contracts")
	}
	if len(matches) == 1 {
		enabled, disabled, blocked := toolEnabledInRepoContext(matches[0].Name, repoContext)
		if disabled {
			add("error", "tool_disabled_by_config", "requested tool is disabled by repository configuration")
		}
		if blocked {
			add("error", "tool_blocked_by_allowlist", "requested tool is not in the configured allowlist")
		}
		if isMutatingToolContract(matches[0]) {
			add("error", "mutating_tool_contract", "mutating tool contracts are blocked in GitClaw v1")
		} else if enabled {
			add("info", "read_only_or_metadata_only", "matched tool can only provide read-only or metadata-only prompt context")
		}
	}
	if validation.Errors > 0 {
		add("error", "tool_validation_errors_present", "fix existing tool validation errors before relying on tool output")
	} else if validation.Warnings > 0 {
		add("warning", "tool_validation_warnings_present", "review existing tool validation warnings before relying on tool output")
	}
	return findings
}

func toolMapStatus(findings []toolMapFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "needs_review"
		}
	}
	return "ok"
}

func writeToolMapFindings(b *strings.Builder, findings []toolMapFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
