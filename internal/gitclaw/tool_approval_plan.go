package gitclaw

import (
	"fmt"
	"strings"
)

type toolApprovalPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderToolApprovalPlanCLIReport(cfg Config, repoContext RepoContext, name string) string {
	return renderToolApprovalPlanReport(Event{}, cfg, repoContext, name, false)
}

func renderToolApprovalPlanReport(ev Event, cfg Config, repoContext RepoContext, name string, includeIssue bool) string {
	requested := cleanToolLookupName(name)
	normalized := normalizeToolLookupName(requested)
	matches := matchingToolContracts(toolReportContracts, requested)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	findings := toolApprovalPlanFindings(requested, matches, repoContext, validation)
	status := toolApprovalPlanStatus(findings)

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
	decision := toolApprovalPlanDecision(requested, matches, enabled, disabled, blocked, mutating, validation)

	var b strings.Builder
	b.WriteString("## GitClaw Tool Approval Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_approval_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- requested_tool_sha256_12: `%s`\n", shortDocumentHash(requested))
	fmt.Fprintf(&b, "- normalized_tool: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", len(activeOutputs))
	fmt.Fprintf(&b, "- tool_enabled: `%t`\n", enabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", disabled)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", blocked)
	fmt.Fprintf(&b, "- tool_mode: `%s`\n", mode)
	fmt.Fprintf(&b, "- tool_trigger: `%s`\n", inlineCode(trigger))
	fmt.Fprintf(&b, "- mutating_contract: `%t`\n", mutating)
	fmt.Fprintf(&b, "- approval_required: `%t`\n", approvalRequired)
	fmt.Fprintf(&b, "- approval_decision: `%s`\n", decision)
	fmt.Fprintf(&b, "- approval_store: `%s`\n", "github-issue-labels")
	fmt.Fprintf(&b, "- approval_scope: `%s`\n", "per-issue")
	fmt.Fprintf(&b, "- approval_label: `%s`\n", defaultApprovedLabel)
	fmt.Fprintf(&b, "- needs_human_label: `%s`\n", defaultNeedsHumanLabel)
	fmt.Fprintf(&b, "- write_requested_label: `%s`\n", cfg.WriteRequestedLabel)
	fmt.Fprintf(&b, "- approval_timeout_policy: `%s`\n", "not_applicable_no_exec_tool")
	fmt.Fprintf(&b, "- run_allowed_now: `%t`\n", runAllowedNow)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- write_actions_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- model_callable_structured_tools: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- network_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_approval_payloads_included: `%t`\n", false)
	writeToolsValidationSummary(&b, validation)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_approval_plan_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is an approval dry-run only. It models the approval boundary for one deterministic tool contract without approving, executing, calling a model, making network calls, mutating the repository, or dumping raw tool inputs, outputs, issue bodies, comments, prompts, approval payloads, credentials, or secret values.\n\n")

	b.WriteString("### Approval Gates\n")
	writeToolApprovalGates(&b, matches, enabled, disabled, blocked, mutating, approvalRequired)

	b.WriteString("\n### Contract\n")
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

	b.WriteString("\n### Review Steps\n")
	if len(matches) != 1 {
		b.WriteString("1. Choose one known GitClaw tool contract from `/tools` or `gitclaw tools list`.\n")
	} else if mutating {
		b.WriteString("1. Keep the tool disabled until a future write mode, approval label, and review workflow exist.\n")
		b.WriteString("2. Require `gitclaw:write-requested` and `gitclaw:approved` on the issue before any future mutation.\n")
		b.WriteString("3. Re-run `gitclaw tools verify`, `gitclaw tools risk`, and a live GitHub Models conversation E2E before enabling execution.\n")
	} else {
		b.WriteString("1. Confirm the tool is enabled by config and not blocked by the allowlist.\n")
		b.WriteString("2. Confirm the trigger is satisfied by bounded issue text or repo context.\n")
		b.WriteString("3. Treat active outputs as untrusted prompt-visible data and inspect hashes with `gitclaw tools provenance`.\n")
		b.WriteString("4. Use a live GitHub Models conversation E2E when changing tool behavior, not only deterministic reports.\n")
	}

	b.WriteString("\n### Findings\n")
	writeToolApprovalPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedToolApprovalPlanName(ev Event, cfg Config) string {
	firstCandidate := ""
	for _, line := range strings.Split(activeRequestText(ev), "\n") {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if len(fields) < 2 || fields[0] != "/tools" {
			continue
		}
		switch strings.ToLower(fields[1]) {
		case "approval-plan", "approval", "approve-plan", "approval-gate", "gate":
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

func toolApprovalPlanDecision(requested string, matches []toolContract, enabled, disabled, blocked, mutating bool, validation ToolValidationReport) string {
	if requested == "" || requested == "__missing__" {
		return "blocked_tool_missing"
	}
	if len(matches) == 0 {
		return "blocked_tool_not_found"
	}
	if len(matches) > 1 {
		return "blocked_tool_ambiguous"
	}
	if disabled {
		return "blocked_disabled_by_config"
	}
	if blocked {
		return "blocked_by_allowlist"
	}
	if validation.Errors > 0 {
		return "blocked_tool_validation_errors"
	}
	if mutating {
		return "approval_required_future_write_mode"
	}
	if !enabled {
		return "blocked_tool_not_enabled"
	}
	return "no_approval_required_read_only"
}

func toolApprovalPlanFindings(requested string, matches []toolContract, repoContext RepoContext, validation ToolValidationReport) []toolApprovalPlanFinding {
	var findings []toolApprovalPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, toolApprovalPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "openclaw_exec_approval_boundary_modeled", "tool execution is modeled as policy, allowlist, approval, and runtime gates before action")
	add("info", "hermes_tool_authorization_boundary_modeled", "tool contracts remain deterministic pre-model context builders rather than model-callable handles")
	add("info", "github_issue_approval_store_modeled", "future approval state is represented by per-issue labels, not raw approval payloads")
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
			add("error", "mutating_tool_requires_future_approval", "mutating tools require a future write mode and explicit issue approval")
		} else if enabled {
			add("info", "read_only_or_metadata_only_no_approval_required", "matched tool is read-only or metadata-only, so no approval label is required in GitClaw v1")
		}
	}
	if validation.Errors > 0 {
		add("error", "tool_validation_errors_present", "fix existing tool validation errors before relying on tool output")
	} else if validation.Warnings > 0 {
		add("warning", "tool_validation_warnings_present", "review existing tool validation warnings before relying on tool output")
	}
	return findings
}

func toolApprovalPlanStatus(findings []toolApprovalPlanFinding) string {
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

func writeToolApprovalGates(b *strings.Builder, matches []toolContract, enabled, disabled, blocked, mutating, approvalRequired bool) {
	contractStatus := "matched"
	if len(matches) == 0 {
		contractStatus = "not_found"
	} else if len(matches) > 1 {
		contractStatus = "ambiguous"
	}
	fmt.Fprintf(b, "- gate=`tool_contract` status=`%s` matched_tools=`%d`\n", contractStatus, len(matches))
	fmt.Fprintf(b, "- gate=`config_enabled` status=`%s` disabled_by_config=`%t`\n", gateStatus(enabled && !disabled), disabled)
	fmt.Fprintf(b, "- gate=`allowlist` status=`%s` blocked_by_allowlist=`%t`\n", gateStatus(!blocked), blocked)
	modeStatus := "not_applicable"
	if len(matches) == 1 {
		if mutating {
			modeStatus = "future_write_approval_required"
		} else {
			modeStatus = "read_only_or_metadata_only"
		}
	}
	fmt.Fprintf(b, "- gate=`tool_mode` status=`%s` mutating_contract=`%t`\n", modeStatus, mutating)
	fmt.Fprintf(b, "- gate=`approval_label` status=`%s` label=`%s`\n", toolApprovalLabelGateStatus(approvalRequired), defaultApprovedLabel)
	fmt.Fprintf(b, "- gate=`write_mode` status=`blocked` detail=`read_only_v1`\n")
	fmt.Fprintf(b, "- gate=`structured_model_tools` status=`disabled`\n")
	fmt.Fprintf(b, "- gate=`shell_exec` status=`disabled`\n")
	fmt.Fprintf(b, "- gate=`repository_mutation` status=`disabled`\n")
}

func toolApprovalLabelGateStatus(required bool) string {
	if required {
		return "required_for_future_write_mode"
	}
	return "not_required"
}

func writeToolApprovalPlanFindings(b *strings.Builder, findings []toolApprovalPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
