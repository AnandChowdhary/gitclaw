package gitclaw

import (
	"fmt"
	"strings"
)

type toolRunPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderToolRunPlanCLIReport(repoContext RepoContext, name string) string {
	return renderToolRunPlanReport(Event{}, repoContext, name, false)
}

func renderToolRunPlanReport(ev Event, repoContext RepoContext, name string, includeIssue bool) string {
	requested := cleanToolLookupName(name)
	normalized := normalizeToolLookupName(requested)
	matches := matchingToolContracts(toolReportContracts, requested)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	findings := toolRunPlanFindings(requested, matches, validation)
	status := toolRunPlanStatus(findings)

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

	var b strings.Builder
	b.WriteString("## GitClaw Tool Run Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_run_plan_status: `%s`\n", status)
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
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- network_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_name_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outputs_included: `%t`\n", false)
	writeToolsValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is a dry-run planner only. It explains one deterministic tool contract and any already-active output hashes without executing shell commands, making network calls, mutating the repository, calling a model, or dumping raw tool inputs or outputs.\n\n")

	b.WriteString("### Contract\n")
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
	} else {
		b.WriteString("1. Confirm the tool is enabled by config and not blocked by the allowlist.\n")
		b.WriteString("2. Confirm the trigger is satisfied by bounded issue text or repo context.\n")
		b.WriteString("3. Inspect output hashes with `gitclaw tools verify` or `@gitclaw /tools verify` after the run.\n")
		b.WriteString("4. Use a live GitHub Models conversation E2E when changing tool behavior, not only deterministic reports.\n")
	}

	b.WriteString("\n### Findings\n")
	writeToolRunPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedToolRunPlanName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/tools" {
		return ""
	}
	if !strings.EqualFold(fields[1], "run-plan") && !strings.EqualFold(fields[1], "plan") {
		return ""
	}
	if len(fields) < 3 {
		return "__missing__"
	}
	return cleanToolLookupName(fields[2])
}

func toolRunPlanFindings(requested string, matches []toolContract, validation ToolValidationReport) []toolRunPlanFinding {
	var findings []toolRunPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, toolRunPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "deterministic_tool_contract", "GitClaw tools are pre-model context builders with static read-only or metadata-only contracts")
	add("info", "shell_execution_disabled", "tool planning never executes shell commands or host processes")
	add("info", "repository_mutation_disabled", "tool planning does not create, update, delete, commit, push, or apply files")
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
		if isMutatingToolContract(matches[0]) {
			add("error", "mutating_tool_contract", "mutating tool contracts are not allowed in GitClaw v1")
		} else {
			add("info", "read_only_or_metadata_only", "matched tool contract is read-only or metadata-only")
		}
	}
	if validation.Errors > 0 {
		add("error", "tool_validation_errors_present", "fix existing tool validation errors before relying on tool output")
	} else if validation.Warnings > 0 {
		add("warning", "tool_validation_warnings_present", "review existing tool validation warnings before relying on tool output")
	}
	return findings
}

func toolRunPlanStatus(findings []toolRunPlanFinding) string {
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

func writeToolRunPlanFindings(b *strings.Builder, findings []toolRunPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
