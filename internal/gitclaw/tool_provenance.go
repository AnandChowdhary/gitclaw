package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ToolProvenanceEntry struct {
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

type ToolProvenanceReport struct {
	Status                              string
	Validation                          ToolValidationReport
	Risk                                ToolRiskReport
	ProvenanceScope                     string
	ContextStrategy                     string
	AvailableTools                      int
	EnabledTools                        int
	DisabledTools                       int
	AllowlistBlockedTools               int
	ActiveToolOutputs                   int
	KnownToolOutputs                    int
	UnknownToolOutputs                  int
	PromptVisibleToolOutputs            int
	PromptVisibleToolNames              []string
	ReadOnlyOutputs                     int
	MetadataOnlyOutputs                 int
	ToolInputsHashed                    int
	ToolOutputsHashed                   int
	TotalOutputBytes                    int
	TotalOutputLines                    int
	MaxOutputBytes                      int
	RegistryVerification                string
	RuntimePermissionVerification       string
	ModelCallableStructuredTools        bool
	ShellExecutionAllowed               bool
	RepositoryMutationAllowed           bool
	RawInputsIncluded                   bool
	RawOutputsIncluded                  bool
	RawBodiesIncluded                   bool
	RawIssueBodiesIncluded              bool
	RawCommentBodiesIncluded            bool
	RawPromptBodiesIncluded             bool
	LLME2ERequiredAfterProvenanceChange bool
	Entries                             []ToolProvenanceEntry
}

func BuildToolProvenanceReport(repoContext RepoContext) ToolProvenanceReport {
	validation := ValidateTools(repoContext)
	risk := BuildToolRiskReport(repoContext)
	contractByName := toolContractMap()
	report := ToolProvenanceReport{
		Status:                              toolProvenanceStatus(validation, risk),
		Validation:                          validation,
		Risk:                                risk,
		ProvenanceScope:                     "pre_model_prompt_context",
		ContextStrategy:                     "deterministic-pre-model-outputs",
		AvailableTools:                      len(toolReportContracts),
		EnabledTools:                        enabledToolCount(repoContext),
		DisabledTools:                       disabledToolCount(repoContext),
		AllowlistBlockedTools:               allowlistBlockedToolCount(repoContext),
		ActiveToolOutputs:                   len(repoContext.ToolOutputs),
		KnownToolOutputs:                    len(repoContext.ToolOutputs) - validation.UnknownOutputs,
		UnknownToolOutputs:                  validation.UnknownOutputs,
		PromptVisibleToolOutputs:            len(repoContext.ToolOutputs),
		PromptVisibleToolNames:              uniqueSortedStrings(promptVisibleToolNames(repoContext.ToolOutputs)),
		RegistryVerification:                risk.RegistryVerification,
		RuntimePermissionVerification:       risk.RuntimePermissionVerification,
		ModelCallableStructuredTools:        false,
		ShellExecutionAllowed:               false,
		RepositoryMutationAllowed:           false,
		RawInputsIncluded:                   false,
		RawOutputsIncluded:                  false,
		RawBodiesIncluded:                   false,
		RawIssueBodiesIncluded:              false,
		RawCommentBodiesIncluded:            false,
		RawPromptBodiesIncluded:             false,
		LLME2ERequiredAfterProvenanceChange: true,
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
		entry := ToolProvenanceEntry{
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
		report.TotalOutputBytes += entry.OutputBytes
		report.TotalOutputLines += entry.OutputLines
		if entry.OutputBytes > report.MaxOutputBytes {
			report.MaxOutputBytes = entry.OutputBytes
		}
		report.Entries = append(report.Entries, entry)
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		if report.Entries[i].Name != report.Entries[j].Name {
			return report.Entries[i].Name < report.Entries[j].Name
		}
		if report.Entries[i].InputSHA != report.Entries[j].InputSHA {
			return report.Entries[i].InputSHA < report.Entries[j].InputSHA
		}
		return report.Entries[i].OutputSHA < report.Entries[j].OutputSHA
	})
	return report
}

func RenderToolProvenanceCLIReport(repoContext RepoContext) string {
	return renderToolProvenanceReport(Event{}, repoContext, false)
}

func renderToolProvenanceReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolProvenanceReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tool Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeToolProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps the deterministic tool outputs that are prompt-visible for the current turn. It reports names, modes, counts, sizes, hashes, and risk codes only; raw tool inputs, tool outputs, file bodies, issue bodies, comments, prompts, search results, and secrets are not included.\n\n")

	b.WriteString("### Prompt-Visible Tool Outputs\n")
	writeToolProvenanceEntries(&b, report.Entries)

	b.WriteString("\n### Provenance Gates\n")
	b.WriteString("- model_callable_structured_tools=`false`\n")
	b.WriteString("- raw_input_gate=`hash_only`\n")
	b.WriteString("- raw_output_gate=`hash_only`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- shell_gate=`disabled`\n")
	return strings.TrimSpace(b.String())
}

func writeToolProvenanceSummary(b *strings.Builder, report ToolProvenanceReport) {
	fmt.Fprintf(b, "- tool_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", report.ProvenanceScope)
	fmt.Fprintf(b, "- tool_context_strategy: `%s`\n", report.ContextStrategy)
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
	fmt.Fprintf(b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(b, "- runtime_permission_verification: `%s`\n", report.RuntimePermissionVerification)
	fmt.Fprintf(b, "- model_callable_structured_tools: `%t`\n", report.ModelCallableStructuredTools)
	fmt.Fprintf(b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	fmt.Fprintf(b, "- raw_outputs_included: `%t`\n", report.RawOutputsIncluded)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_tool_provenance_change: `%t`\n", report.LLME2ERequiredAfterProvenanceChange)
	writeToolsValidationSummary(b, report.Validation)
	writeToolRiskSummary(b, report.Risk)
}

func writeToolProvenanceEntries(b *strings.Builder, entries []ToolProvenanceEntry) {
	if len(entries) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, entry := range entries {
		fmt.Fprintf(
			b,
			"- name=`%s` contract_known=`%t` mode=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` prompt_visible=`%t` input_sha256_12=`%s` input_bytes=`%d` input_lines=`%d` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` trigger=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			entry.Name,
			entry.ContractKnown,
			entry.Mode,
			entry.Enabled,
			entry.DisabledByConfig,
			entry.BlockedByAllowlist,
			entry.PromptVisible,
			entry.InputSHA,
			entry.InputBytes,
			entry.InputLines,
			entry.OutputBytes,
			entry.OutputLines,
			entry.OutputSHA,
			inlineCode(entry.Trigger),
			len(entry.RiskFindings),
			entry.RiskMaxSeverity,
			inlineListOrNone(entry.RiskCodes),
			inlineListOrNone(entry.RiskLineHashes),
		)
	}
}

func toolProvenanceStatus(validation ToolValidationReport, risk ToolRiskReport) string {
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

func toolContractMap() map[string]toolContract {
	contracts := map[string]toolContract{}
	for _, contract := range toolReportContracts {
		contracts[contract.Name] = contract
	}
	return contracts
}

func isToolProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "provenance") ||
			strings.EqualFold(fields[1], "outputs") ||
			strings.EqualFold(fields[1], "trace") ||
			strings.EqualFold(fields[1], "lineage"))
}
