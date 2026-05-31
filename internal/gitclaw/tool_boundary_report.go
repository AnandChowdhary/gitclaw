package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ToolBoundaryCard struct {
	Name               string
	Mode               string
	Trigger            string
	ContractKnown      bool
	Enabled            bool
	DisabledByConfig   bool
	BlockedByAllowlist bool
	PromptVisible      bool
	InputSHA           string
	InputBytes         int
	InputLines         int
	OutputSHA          string
	OutputBytes        int
	OutputLines        int
	RiskFindings       []ToolRiskFinding
	RiskMaxSeverity    string
	RiskCodes          []string
	RiskLineHashes     []string
}

type ToolBoundaryReport struct {
	Status                            string
	Validation                        ToolValidationReport
	Risk                              ToolRiskReport
	BoundaryScope                     string
	ContextStrategy                   string
	ToolOutputDelimiter               string
	PromptInjectionScan               string
	AvailableTools                    int
	EnabledTools                      int
	DisabledTools                     int
	AllowlistBlockedTools             int
	ActiveToolOutputs                 int
	KnownToolOutputs                  int
	UnknownToolOutputs                int
	PromptVisibleToolOutputs          int
	PromptVisibleToolNames            []string
	ReadOnlyOutputs                   int
	MetadataOnlyOutputs               int
	ToolInputsHashed                  int
	ToolOutputsHashed                 int
	TotalOutputBytes                  int
	TotalOutputLines                  int
	MaxOutputBytes                    int
	PromptBoundaryFindings            int
	PromptBoundaryHighFindings        int
	RegistryVerification              string
	RuntimePermissionVerification     string
	ModelCallableStructuredTools      bool
	ShellExecutionAllowed             bool
	RepositoryMutationAllowed         bool
	NetworkToolExecutionAllowed       bool
	RawToolInputsIncluded             bool
	RawToolOutputsIncluded            bool
	RawToolSchemasIncluded            bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	RawPromptBodiesIncluded           bool
	RawSearchQueriesIncluded          bool
	LLME2ERequiredAfterBoundaryChange bool
	Cards                             []ToolBoundaryCard
}

func BuildToolBoundaryReport(repoContext RepoContext) ToolBoundaryReport {
	validation := ValidateTools(repoContext)
	risk := BuildToolRiskReport(repoContext)
	contractByName := toolContractMap()
	report := ToolBoundaryReport{
		Status:                            toolBoundaryStatus(validation, risk),
		Validation:                        validation,
		Risk:                              risk,
		BoundaryScope:                     "prompt-visible-tool-output-boundary",
		ContextStrategy:                   "deterministic-pre-model-outputs",
		ToolOutputDelimiter:               "tool_output_blocks",
		PromptInjectionScan:               "enabled",
		AvailableTools:                    len(toolReportContracts),
		EnabledTools:                      enabledToolCount(repoContext),
		DisabledTools:                     disabledToolCount(repoContext),
		AllowlistBlockedTools:             allowlistBlockedToolCount(repoContext),
		ActiveToolOutputs:                 len(repoContext.ToolOutputs),
		KnownToolOutputs:                  len(repoContext.ToolOutputs) - validation.UnknownOutputs,
		UnknownToolOutputs:                validation.UnknownOutputs,
		PromptVisibleToolOutputs:          len(repoContext.ToolOutputs),
		PromptVisibleToolNames:            uniqueSortedStrings(promptVisibleToolNames(repoContext.ToolOutputs)),
		RegistryVerification:              risk.RegistryVerification,
		RuntimePermissionVerification:     risk.RuntimePermissionVerification,
		ModelCallableStructuredTools:      false,
		ShellExecutionAllowed:             false,
		RepositoryMutationAllowed:         false,
		NetworkToolExecutionAllowed:       false,
		RawToolInputsIncluded:             false,
		RawToolOutputsIncluded:            false,
		RawToolSchemasIncluded:            false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		RawPromptBodiesIncluded:           false,
		RawSearchQueriesIncluded:          false,
		LLME2ERequiredAfterBoundaryChange: true,
	}
	for _, finding := range risk.Findings {
		if finding.Category != "prompt-boundary" {
			continue
		}
		report.PromptBoundaryFindings++
		if finding.Severity == "high" {
			report.PromptBoundaryHighFindings++
		}
	}
	for _, output := range repoContext.ToolOutputs {
		contract, known := contractByName[output.Name]
		mode := "unknown"
		trigger := "unknown"
		enabled, disabled, blocked := false, false, false
		if known {
			mode = contract.Mode
			trigger = contract.Trigger
			enabled, disabled, blocked = toolEnabledInRepoContext(contract.Name, repoContext)
			switch contract.Mode {
			case "read-only":
				report.ReadOnlyOutputs++
			case "metadata-only":
				report.MetadataOnlyOutputs++
			}
		}
		findings := scanToolOutputRiskFindings(output, known)
		card := ToolBoundaryCard{
			Name:               output.Name,
			Mode:               mode,
			Trigger:            trigger,
			ContractKnown:      known,
			Enabled:            enabled,
			DisabledByConfig:   disabled,
			BlockedByAllowlist: blocked,
			PromptVisible:      true,
			InputSHA:           shortDocumentHash(output.Input),
			InputBytes:         len(output.Input),
			InputLines:         lineCount(output.Input),
			OutputSHA:          shortDocumentHash(output.Output),
			OutputBytes:        len(output.Output),
			OutputLines:        lineCount(output.Output),
			RiskFindings:       findings,
			RiskMaxSeverity:    toolRiskMaxSeverity(findings),
			RiskCodes:          toolRiskCodes(findings),
			RiskLineHashes:     toolRiskLineHashes(findings),
		}
		if strings.TrimSpace(output.Input) != "" {
			report.ToolInputsHashed++
		}
		if strings.TrimSpace(output.Output) != "" {
			report.ToolOutputsHashed++
		}
		report.TotalOutputBytes += card.OutputBytes
		report.TotalOutputLines += card.OutputLines
		if card.OutputBytes > report.MaxOutputBytes {
			report.MaxOutputBytes = card.OutputBytes
		}
		report.Cards = append(report.Cards, card)
	}
	sort.Slice(report.Cards, func(i, j int) bool {
		if report.Cards[i].Name != report.Cards[j].Name {
			return report.Cards[i].Name < report.Cards[j].Name
		}
		if report.Cards[i].InputSHA != report.Cards[j].InputSHA {
			return report.Cards[i].InputSHA < report.Cards[j].InputSHA
		}
		return report.Cards[i].OutputSHA < report.Cards[j].OutputSHA
	})
	return report
}

func RenderToolBoundaryCLIReport(repoContext RepoContext) string {
	return renderToolBoundaryReport(Event{}, repoContext, false)
}

func renderToolBoundaryReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolBoundaryReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tool Boundary Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeToolBoundarySummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits the prompt boundary around deterministic GitClaw tool outputs. It reports tool names, modes, counts, sizes, risk codes, and hashes only; raw tool inputs, tool outputs, search queries, issue bodies, comments, prompts, and secrets are not included.\n\n")

	b.WriteString("### Boundary Cards\n")
	writeToolBoundaryCards(&b, report.Cards)

	b.WriteString("\n### Boundary Gates\n")
	writeToolBoundaryGates(&b, report)

	b.WriteString("\n### Boundary Findings\n")
	writeToolRiskFindings(&b, report.Risk.Findings)
	return strings.TrimSpace(b.String())
}

func writeToolBoundarySummary(b *strings.Builder, report ToolBoundaryReport) {
	fmt.Fprintf(b, "- tool_boundary_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- boundary_scope: `%s`\n", report.BoundaryScope)
	fmt.Fprintf(b, "- tool_context_strategy: `%s`\n", report.ContextStrategy)
	fmt.Fprintf(b, "- tool_output_delimiter: `%s`\n", report.ToolOutputDelimiter)
	fmt.Fprintf(b, "- prompt_injection_scan: `%s`\n", report.PromptInjectionScan)
	fmt.Fprintf(b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(b, "- enabled_tools: `%d`\n", report.EnabledTools)
	fmt.Fprintf(b, "- disabled_tools: `%d`\n", report.DisabledTools)
	fmt.Fprintf(b, "- allowlist_blocked_tools: `%d`\n", report.AllowlistBlockedTools)
	fmt.Fprintf(b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(b, "- known_tool_outputs: `%d`\n", report.KnownToolOutputs)
	fmt.Fprintf(b, "- unknown_tool_outputs: `%d`\n", report.UnknownToolOutputs)
	fmt.Fprintf(b, "- prompt_visible_tool_outputs: `%d`\n", report.PromptVisibleToolOutputs)
	fmt.Fprintf(b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(b, "- read_only_outputs: `%d`\n", report.ReadOnlyOutputs)
	fmt.Fprintf(b, "- metadata_only_outputs: `%d`\n", report.MetadataOnlyOutputs)
	fmt.Fprintf(b, "- tool_inputs_hashed: `%d`\n", report.ToolInputsHashed)
	fmt.Fprintf(b, "- tool_outputs_hashed: `%d`\n", report.ToolOutputsHashed)
	fmt.Fprintf(b, "- total_tool_output_bytes: `%d`\n", report.TotalOutputBytes)
	fmt.Fprintf(b, "- total_tool_output_lines: `%d`\n", report.TotalOutputLines)
	fmt.Fprintf(b, "- max_tool_output_bytes: `%d`\n", report.MaxOutputBytes)
	fmt.Fprintf(b, "- prompt_boundary_findings: `%d`\n", report.PromptBoundaryFindings)
	fmt.Fprintf(b, "- prompt_boundary_high_findings: `%d`\n", report.PromptBoundaryHighFindings)
	fmt.Fprintf(b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(b, "- runtime_permission_verification: `%s`\n", report.RuntimePermissionVerification)
	fmt.Fprintf(b, "- model_callable_structured_tools: `%t`\n", report.ModelCallableStructuredTools)
	fmt.Fprintf(b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- network_tool_execution_allowed: `%t`\n", report.NetworkToolExecutionAllowed)
	fmt.Fprintf(b, "- raw_tool_inputs_included: `%t`\n", report.RawToolInputsIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- raw_tool_schemas_included: `%t`\n", report.RawToolSchemasIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_tool_boundary_change: `%t`\n", report.LLME2ERequiredAfterBoundaryChange)
	writeToolsValidationSummary(b, report.Validation)
	writeToolRiskSummary(b, report.Risk)
}

func writeToolBoundaryCards(b *strings.Builder, cards []ToolBoundaryCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- name=`%s` contract_known=`%t` mode=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` prompt_visible=`%t` input_sha256_12=`%s` input_bytes=`%d` input_lines=`%d` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` delimiter=`tool_output` trigger=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			card.Name,
			card.ContractKnown,
			card.Mode,
			card.Enabled,
			card.DisabledByConfig,
			card.BlockedByAllowlist,
			card.PromptVisible,
			card.InputSHA,
			card.InputBytes,
			card.InputLines,
			card.OutputBytes,
			card.OutputLines,
			card.OutputSHA,
			inlineCode(card.Trigger),
			len(card.RiskFindings),
			card.RiskMaxSeverity,
			inlineListOrNone(card.RiskCodes),
			inlineListOrNone(card.RiskLineHashes),
		)
	}
}

func writeToolBoundaryGates(b *strings.Builder, report ToolBoundaryReport) {
	fmt.Fprintf(b, "- gate=`structured_model_tools` state=`disabled` result=`pass`\n")
	fmt.Fprintf(b, "- gate=`tool_output_delimiters` state=`%s` result=`pass`\n", report.ToolOutputDelimiter)
	fmt.Fprintf(b, "- gate=`raw_tool_inputs` state=`hash_only` result=`pass`\n")
	fmt.Fprintf(b, "- gate=`raw_tool_outputs` state=`hash_only` result=`pass`\n")
	fmt.Fprintf(b, "- gate=`shell_execution` state=`disabled` result=`pass`\n")
	fmt.Fprintf(b, "- gate=`repository_mutation` state=`disabled` result=`pass`\n")
	fmt.Fprintf(b, "- gate=`network_tool_execution` state=`disabled` result=`pass`\n")
	fmt.Fprintf(b, "- gate=`unknown_outputs` state=`%d` result=`%s`\n", report.UnknownToolOutputs, toolBoundaryUnknownOutputGate(report))
	fmt.Fprintf(b, "- gate=`tool_validation` state=`%s` result=`%s`\n", report.Validation.Status, toolBoundaryValidationGate(report))
	fmt.Fprintf(b, "- gate=`prompt_injection` state=`%d` result=`%s`\n", report.PromptBoundaryFindings, toolBoundaryPromptInjectionGate(report))
}

func toolBoundaryStatus(validation ToolValidationReport, risk ToolRiskReport) string {
	status := validation.Status
	if status == "" {
		status = "ok"
	}
	if status != "error" {
		switch risk.Status {
		case "high":
			status = "high"
		case "warn":
			if status == "ok" {
				status = "warn"
			}
		}
	}
	return status
}

func toolBoundaryUnknownOutputGate(report ToolBoundaryReport) string {
	if report.UnknownToolOutputs > 0 {
		return "high"
	}
	return "pass"
}

func toolBoundaryValidationGate(report ToolBoundaryReport) string {
	switch report.Validation.Status {
	case "error":
		return "high"
	case "warn":
		return "warn"
	default:
		return "pass"
	}
}

func toolBoundaryPromptInjectionGate(report ToolBoundaryReport) string {
	if report.PromptBoundaryHighFindings > 0 {
		return "high"
	}
	if report.PromptBoundaryFindings > 0 {
		return "warn"
	}
	return "pass"
}

func isToolBoundaryRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "boundary") ||
			strings.EqualFold(fields[1], "prompt-boundary") ||
			strings.EqualFold(fields[1], "promptware"))
}
