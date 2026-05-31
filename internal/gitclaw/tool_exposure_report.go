package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ToolExposureCard struct {
	Name                 string
	Mode                 string
	Trigger              string
	Enabled              bool
	DisabledByConfig     bool
	BlockedByAllowlist   bool
	ExposedForPrompt     bool
	Mutating             bool
	ActiveOutputs        int
	RiskFindings         []ToolRiskFinding
	ExposureFindingCodes []string
}

type ToolExposureReport struct {
	Status                            string
	ExposureStrategy                  string
	BridgeStrategy                    string
	AvailableTools                    int
	EnabledToolContracts              int
	DisabledToolContracts             int
	AllowlistBlockedToolContracts     int
	ExplicitAllowlistConfigured       bool
	AllowedToolNames                  []string
	DisabledToolNames                 []string
	AllowedToolNamesConfigured        int
	DisabledToolNamesConfigured       int
	ExposedReadOnlyContracts          int
	ExposedMetadataOnlyContracts      int
	MutatingToolContracts             int
	ActiveToolOutputs                 int
	KnownActiveToolOutputs            int
	UnknownActiveToolOutputs          int
	PromptVisibleToolOutputs          int
	ToolSearchBridgeTools             int
	DeferredToolSchemas               int
	ModelCallableStructuredTools      bool
	FailClosedRequired                bool
	ToolValidationStatus              string
	ToolValidationErrors              int
	ToolValidationWarnings            int
	ShellExecutionAllowed             bool
	RepositoryMutationAllowed         bool
	NetworkToolExecutionAllowed       bool
	RawToolSchemasIncluded            bool
	RawToolInputsIncluded             bool
	RawToolOutputsIncluded            bool
	RawIssueBodiesIncluded            bool
	LLME2ERequiredAfterExposureChange bool
	Findings                          []ToolRiskFinding
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	Cards                             []ToolExposureCard
}

func RenderToolExposureCLIReport(repoContext RepoContext) string {
	return renderToolExposureReport(Event{}, repoContext, false)
}

func RenderToolExposureRiskCLIReport(repoContext RepoContext) string {
	return renderToolExposureRiskReport(Event{}, repoContext, false)
}

func renderToolExposureReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolExposureReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tool Exposure Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolExposureHeader(&b, ev, includeIssue)
	writeToolExposureSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report explains which deterministic GitClaw tool contracts are prompt-visible for the current turn. GitClaw v1 does not expose model-callable structured tool schemas; it builds bounded read-only tool outputs before the model call and records hashes.\n\n")

	b.WriteString("### Exposure Cards\n")
	writeToolExposureCards(&b, report.Cards)
	return strings.TrimSpace(b.String())
}

func renderToolExposureRiskReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolExposureReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tool Exposure Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolExposureHeader(&b, ev, includeIssue)
	writeToolExposureSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits the prompt-visible tool boundary for explicit allowlists, denylist precedence, unknown active outputs, mutating contracts, and fail-closed conditions. It reports names, modes, counts, hashes, codes, and severities only.\n\n")

	b.WriteString("### Exposure Risk Cards\n")
	writeToolExposureCards(&b, report.Cards)

	b.WriteString("\n### Exposure Findings\n")
	writeToolExposureFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildToolExposureReport(repoContext RepoContext) ToolExposureReport {
	validation := ValidateTools(repoContext)
	activeCounts := toolActiveOutputCounts(repoContext.ToolOutputs)
	contractNames := toolContractNameSet()
	report := ToolExposureReport{
		Status:                            "ok",
		ExposureStrategy:                  "static-pre-model-context",
		BridgeStrategy:                    "not_enabled_in_v1",
		AvailableTools:                    len(toolReportContracts),
		ExplicitAllowlistConfigured:       len(repoContext.AllowedTools) > 0,
		AllowedToolNames:                  sortedToolMapKeys(repoContext.AllowedTools),
		DisabledToolNames:                 sortedToolMapKeys(repoContext.DisabledTools),
		AllowedToolNamesConfigured:        len(repoContext.AllowedTools),
		DisabledToolNamesConfigured:       len(repoContext.DisabledTools),
		ActiveToolOutputs:                 len(repoContext.ToolOutputs),
		PromptVisibleToolOutputs:          len(repoContext.ToolOutputs),
		ToolValidationStatus:              validation.Status,
		ToolValidationErrors:              validation.Errors,
		ToolValidationWarnings:            validation.Warnings,
		ModelCallableStructuredTools:      false,
		ToolSearchBridgeTools:             0,
		DeferredToolSchemas:               0,
		ShellExecutionAllowed:             false,
		RepositoryMutationAllowed:         false,
		NetworkToolExecutionAllowed:       false,
		RawToolSchemasIncluded:            false,
		RawToolInputsIncluded:             false,
		RawToolOutputsIncluded:            false,
		RawIssueBodiesIncluded:            false,
		LLME2ERequiredAfterExposureChange: true,
	}
	for _, output := range repoContext.ToolOutputs {
		if contractNames[output.Name] {
			report.KnownActiveToolOutputs++
		} else {
			report.UnknownActiveToolOutputs++
		}
	}
	for _, contract := range toolReportContracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		card := ToolExposureCard{
			Name:               contract.Name,
			Mode:               contract.Mode,
			Trigger:            contract.Trigger,
			Enabled:            enabled,
			DisabledByConfig:   disabled,
			BlockedByAllowlist: blocked,
			ExposedForPrompt:   enabled && !isMutatingToolContract(contract),
			Mutating:           isMutatingToolContract(contract),
			ActiveOutputs:      activeCounts[contract.Name],
			RiskFindings:       scanToolContractRiskFindings(contract),
		}
		card.ExposureFindingCodes = toolExposureCardFindingCodes(card)
		report.Cards = append(report.Cards, card)
		if enabled {
			report.EnabledToolContracts++
			switch contract.Mode {
			case "read-only":
				report.ExposedReadOnlyContracts++
			case "metadata-only":
				report.ExposedMetadataOnlyContracts++
			}
		}
		if disabled {
			report.DisabledToolContracts++
		}
		if blocked {
			report.AllowlistBlockedToolContracts++
		}
		if card.Mutating {
			report.MutatingToolContracts++
		}
		report.Findings = append(report.Findings, card.RiskFindings...)
	}
	report.FailClosedRequired = report.ExplicitAllowlistConfigured && report.EnabledToolContracts == 0
	report.Findings = append(report.Findings, toolExposureBoundaryFindings(report, validation)...)
	sortToolRiskFindings(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func toolExposureBoundaryFindings(report ToolExposureReport, validation ToolValidationReport) []ToolRiskFinding {
	var findings []ToolRiskFinding
	add := func(severity, code, category, name, field, value string) {
		findings = append(findings, ToolRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Kind:     "tool-exposure",
			Name:     name,
			Field:    field,
			Line:     0,
			LineSHA:  shortDocumentHash(value),
		})
	}
	add("info", "static_pre_model_tool_context", "tool-exposure", "gitclaw", "strategy", report.ExposureStrategy)
	add("info", "structured_model_tools_disabled", "tool-exposure", "gitclaw", "model_callable_structured_tools", "false")
	add("info", "hermes_tool_search_bridge_not_enabled", "progressive-disclosure", "gitclaw", "bridge_strategy", report.BridgeStrategy)
	if report.ExplicitAllowlistConfigured && report.EnabledToolContracts == 0 {
		add("high", "explicit_allowlist_resolved_zero", "fail-closed", "tools.allowed", "allowed_tools", strings.Join(report.AllowedToolNames, ","))
	}
	if report.EnabledToolContracts == 0 {
		add("warning", "no_enabled_tool_contracts", "tool-exposure", "gitclaw", "enabled_tools", "0")
	}
	if validation.Errors > 0 {
		add("high", "tool_validation_errors_present", "tool-validation", "tools", "validation", validation.Status)
	}
	if validation.Warnings > 0 {
		add("warning", "tool_validation_warnings_present", "tool-validation", "tools", "validation", validation.Status)
	}
	if report.UnknownActiveToolOutputs > 0 {
		add("high", "unknown_active_tool_outputs_present", "tool-provenance", "tool_outputs", "active_outputs", fmt.Sprintf("%d", report.UnknownActiveToolOutputs))
	}
	return findings
}

func toolExposureCardFindingCodes(card ToolExposureCard) []string {
	var codes []string
	if card.DisabledByConfig {
		codes = append(codes, "disabled_by_config")
	}
	if card.BlockedByAllowlist {
		codes = append(codes, "blocked_by_allowlist")
	}
	if card.Mutating {
		codes = append(codes, "mutating_tool_contract")
	}
	codes = append(codes, toolRiskCodes(card.RiskFindings)...)
	return uniqueSortedStrings(codes)
}

func writeToolExposureHeader(b *strings.Builder, ev Event, includeIssue bool) {
	if includeIssue {
		fmt.Fprintf(b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(b, "- scope: `%s`\n", "local-cli")
	}
}

func writeToolExposureSummary(b *strings.Builder, report ToolExposureReport) {
	fmt.Fprintf(b, "- tool_exposure_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- exposure_strategy: `%s`\n", report.ExposureStrategy)
	fmt.Fprintf(b, "- bridge_strategy: `%s`\n", report.BridgeStrategy)
	fmt.Fprintf(b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(b, "- enabled_tool_contracts: `%d`\n", report.EnabledToolContracts)
	fmt.Fprintf(b, "- disabled_tool_contracts: `%d`\n", report.DisabledToolContracts)
	fmt.Fprintf(b, "- allowlist_blocked_tool_contracts: `%d`\n", report.AllowlistBlockedToolContracts)
	fmt.Fprintf(b, "- explicit_allowlist_configured: `%t`\n", report.ExplicitAllowlistConfigured)
	fmt.Fprintf(b, "- allowed_tool_names_configured: `%d`\n", report.AllowedToolNamesConfigured)
	fmt.Fprintf(b, "- disabled_tool_names_configured: `%d`\n", report.DisabledToolNamesConfigured)
	fmt.Fprintf(b, "- allowed_tool_names: `%s`\n", inlineListOrNone(report.AllowedToolNames))
	fmt.Fprintf(b, "- disabled_tool_names: `%s`\n", inlineListOrNone(report.DisabledToolNames))
	fmt.Fprintf(b, "- exposed_read_only_contracts: `%d`\n", report.ExposedReadOnlyContracts)
	fmt.Fprintf(b, "- exposed_metadata_only_contracts: `%d`\n", report.ExposedMetadataOnlyContracts)
	fmt.Fprintf(b, "- mutating_tool_contracts: `%d`\n", report.MutatingToolContracts)
	fmt.Fprintf(b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(b, "- known_active_tool_outputs: `%d`\n", report.KnownActiveToolOutputs)
	fmt.Fprintf(b, "- unknown_active_tool_outputs: `%d`\n", report.UnknownActiveToolOutputs)
	fmt.Fprintf(b, "- prompt_visible_tool_outputs: `%d`\n", report.PromptVisibleToolOutputs)
	fmt.Fprintf(b, "- model_callable_structured_tools: `%t`\n", report.ModelCallableStructuredTools)
	fmt.Fprintf(b, "- deferred_tool_schemas: `%d`\n", report.DeferredToolSchemas)
	fmt.Fprintf(b, "- tool_search_bridge_tools: `%d`\n", report.ToolSearchBridgeTools)
	fmt.Fprintf(b, "- fail_closed_required: `%t`\n", report.FailClosedRequired)
	fmt.Fprintf(b, "- tool_validation_status: `%s`\n", report.ToolValidationStatus)
	fmt.Fprintf(b, "- tool_validation_errors: `%d`\n", report.ToolValidationErrors)
	fmt.Fprintf(b, "- tool_validation_warnings: `%d`\n", report.ToolValidationWarnings)
	fmt.Fprintf(b, "- tool_exposure_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- network_tool_execution_allowed: `%t`\n", report.NetworkToolExecutionAllowed)
	fmt.Fprintf(b, "- raw_tool_schemas_included: `%t`\n", report.RawToolSchemasIncluded)
	fmt.Fprintf(b, "- raw_tool_inputs_included: `%t`\n", report.RawToolInputsIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_tool_exposure_change: `%t`\n", report.LLME2ERequiredAfterExposureChange)
}

func writeToolExposureCards(b *strings.Builder, cards []ToolExposureCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(b, "- tool_name=`%s` mode=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` exposed_for_prompt=`%t` mutating=`%t` active_outputs=`%d` trigger=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` exposure_codes=`%s`\n",
			card.Name,
			card.Mode,
			card.Enabled,
			card.DisabledByConfig,
			card.BlockedByAllowlist,
			card.ExposedForPrompt,
			card.Mutating,
			card.ActiveOutputs,
			inlineCode(card.Trigger),
			len(card.RiskFindings),
			toolRiskMaxSeverity(card.RiskFindings),
			inlineListOrNone(toolRiskCodes(card.RiskFindings)),
			inlineListOrNone(card.ExposureFindingCodes),
		)
	}
}

func writeToolExposureFindings(b *strings.Builder, findings []ToolRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` field=`%s` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			inlineCode(finding.Name),
			finding.Field,
			finding.LineSHA,
		)
	}
}

func sortedToolMapKeys(values map[string]bool) []string {
	var keys []string
	for key, enabled := range values {
		if enabled {
			keys = append(keys, normalizeToolLookupName(key))
		}
	}
	sort.Strings(keys)
	return uniqueSortedStrings(keys)
}

func isToolExposureListRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "exposure") || strings.EqualFold(fields[1], "expose")) &&
		(len(fields) == 2 || strings.EqualFold(fields[2], "list"))
}

func isToolExposureRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "exposure") || strings.EqualFold(fields[1], "expose")) &&
		(strings.EqualFold(fields[2], "risk") || strings.EqualFold(fields[2], "risk-audit"))
}
