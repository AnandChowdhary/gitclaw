package gitclaw

import (
	"fmt"
	"strings"
)

type toolReadinessFinding struct {
	Severity string
	Code     string
	Detail   string
}

type toolReadinessGate struct {
	Name   string
	Status string
	Detail string
}

func RenderToolReadinessCLIReport(repoContext RepoContext, name string) string {
	return renderToolReadinessReport(Event{}, repoContext, name, false)
}

func renderToolReadinessReport(ev Event, repoContext RepoContext, name string, includeIssue bool) string {
	requested := cleanToolLookupName(name)
	normalized := normalizeToolLookupName(requested)
	matches := matchingToolContracts(toolReportContracts, requested)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	risk := BuildToolRiskReport(repoContext)
	findings := toolReadinessFindings(requested, matches, repoContext, validation, risk)
	status := toolReadinessStatus(findings)

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
	promptVisibleReady := len(matches) == 1 &&
		enabled &&
		!disabled &&
		!blocked &&
		!mutating &&
		validation.Errors == 0 &&
		validation.Warnings == 0 &&
		risk.HighRiskFindings == 0 &&
		risk.WarningRiskFindings == 0
	modelContextAllowed := promptVisibleReady
	executionAllowedNow := false
	approvalRequired := len(matches) == 1 && mutating
	gates := toolReadinessGates(len(matches), enabled, disabled, blocked, mutating, len(activeOutputs), validation, risk)

	var b strings.Builder
	b.WriteString("## GitClaw Tool Readiness Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_readiness_status: `%s`\n", status)
	fmt.Fprintf(&b, "- tool_readiness_mode: `%s`\n", "body-free-tool-gate-checklist")
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
	fmt.Fprintf(&b, "- prompt_visible_ready: `%t`\n", promptVisibleReady)
	fmt.Fprintf(&b, "- model_context_allowed: `%t`\n", modelContextAllowed)
	fmt.Fprintf(&b, "- execution_allowed_now: `%t`\n", executionAllowedNow)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", approvalRequired)
	fmt.Fprintf(&b, "- readiness_gate_count: `%d`\n", len(gates))
	fmt.Fprintf(&b, "- readiness_gates_sha256_12: `%s`\n", shortDocumentHash(toolReadinessGateManifest(gates)))
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- tool_execution_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_launch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- network_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comments_included: `%t`\n", false)
	writeToolsValidationSummary(&b, validation)
	writeToolRiskSummary(&b, risk)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_readiness_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report checks whether one deterministic GitClaw tool contract is ready to be exposed as prompt-visible context. It does not execute tools, launch MCP servers, call a model, create approval/rehearsal/run-request issues, make network calls, mutate workflows, mutate repository files, or dump raw tool inputs, outputs, issue bodies, comments, prompts, credentials, or secret values.\n\n")

	b.WriteString("### Matched Tool\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		activeCounts := toolActiveOutputCounts(repoContext.ToolOutputs)
		for _, contract := range matches {
			writeToolInfoContract(&b, contract, activeCounts[contract.Name], repoContext)
		}
	}

	b.WriteString("\n### Active Outputs For Tool\n")
	writeToolInfoActiveOutputs(&b, activeOutputs)

	b.WriteString("\n### Readiness Gates\n")
	writeToolReadinessGates(&b, gates)

	b.WriteString("\n### Findings\n")
	writeToolReadinessFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedToolReadinessName(ev Event, cfg Config) string {
	firstCandidate := ""
	for _, line := range strings.Split(activeRequestText(ev), "\n") {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if len(fields) < 2 || fields[0] != "/tools" {
			continue
		}
		switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
		case "readiness", "ready", "readiness-check", "gate-check", "tool-readiness", "prompt-ready", "prompt-readiness":
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

func toolReadinessGates(matched int, enabled, disabled, blocked, mutating bool, activeOutputs int, validation ToolValidationReport, risk ToolRiskReport) []toolReadinessGate {
	contractStatus := "matched"
	if matched == 0 {
		contractStatus = "not_found"
	} else if matched > 1 {
		contractStatus = "ambiguous"
	}
	return []toolReadinessGate{
		{Name: "tool_contract", Status: contractStatus, Detail: fmt.Sprintf("matched_tools=%d", matched)},
		{Name: "config_enabled", Status: gateStatus(enabled && !disabled), Detail: fmt.Sprintf("disabled_by_config=%t", disabled)},
		{Name: "allowlist", Status: gateStatus(!blocked), Detail: fmt.Sprintf("blocked_by_allowlist=%t", blocked)},
		{Name: "tool_mode", Status: toolMapModeGateStatus(matched, mutating), Detail: fmt.Sprintf("mutating_contract=%t", mutating)},
		{Name: "validation", Status: validation.Status, Detail: fmt.Sprintf("errors=%d warnings=%d", validation.Errors, validation.Warnings)},
		{Name: "risk", Status: risk.Status, Detail: fmt.Sprintf("high=%d warning=%d", risk.HighRiskFindings, risk.WarningRiskFindings)},
		{Name: "active_outputs", Status: "hashes_only", Detail: fmt.Sprintf("outputs=%d", activeOutputs)},
		{Name: "model_call", Status: "not_performed", Detail: "readiness report is deterministic"},
		{Name: "tool_execution", Status: "disabled", Detail: "readiness never executes tools"},
		{Name: "shell_exec", Status: "disabled", Detail: "host process execution is outside this report"},
		{Name: "mcp_launch", Status: "disabled", Detail: "MCP servers are not launched by readiness"},
		{Name: "repository_mutation", Status: "disabled", Detail: "repo files are not changed"},
	}
}

func toolReadinessGateManifest(gates []toolReadinessGate) string {
	var lines []string
	for _, gate := range gates {
		lines = append(lines, strings.Join([]string{gate.Name, gate.Status, gate.Detail}, "|"))
	}
	return strings.Join(lines, "\n")
}

func writeToolReadinessGates(b *strings.Builder, gates []toolReadinessGate) {
	if len(gates) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, gate := range gates {
		fmt.Fprintf(b, "- gate=`%s` status=`%s` detail=`%s`\n", gate.Name, gate.Status, inlineCode(gate.Detail))
	}
}

func toolReadinessFindings(requested string, matches []toolContract, repoContext RepoContext, validation ToolValidationReport, risk ToolRiskReport) []toolReadinessFinding {
	var findings []toolReadinessFinding
	add := func(severity, code, detail string) {
		findings = append(findings, toolReadinessFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "openclaw_prompt_tool_exposure_checked", "tool exposure is checked before the model sees tool context")
	add("info", "hermes_tool_boundary_kept_issue_native", "readiness is reported in the GitHub issue instead of hidden socket or gateway state")
	add("info", "tool_execution_disabled", "readiness does not execute tools, launch MCP servers, call models, or mutate repositories")
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
			add("error", "mutating_tool_contract", "mutating tool contracts cannot become prompt-visible without a separate approval design")
		} else if enabled {
			add("info", "prompt_visible_read_only_or_metadata_only", "matched tool is eligible for prompt-visible context when validation and risk gates pass")
		}
	}
	if validation.Errors > 0 {
		add("error", "tool_validation_errors_present", "fix existing tool validation errors before exposing tool context")
	} else if validation.Warnings > 0 {
		add("warning", "tool_validation_warnings_present", "review existing tool validation warnings before exposing tool context")
	}
	if risk.HighRiskFindings > 0 {
		add("error", "tool_high_risk_findings_present", "fix high-risk tool findings before exposing tool context")
	} else if risk.WarningRiskFindings > 0 {
		add("warning", "tool_risk_warnings_present", "review tool risk warnings before exposing tool context")
	}
	return findings
}

func toolReadinessStatus(findings []toolReadinessFinding) string {
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

func writeToolReadinessFindings(b *strings.Builder, findings []toolReadinessFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
