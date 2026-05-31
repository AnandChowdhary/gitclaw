package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

const (
	defaultToolDeferMode            = "auto"
	defaultToolDeferThresholdPct    = 10
	defaultToolDeferEntryBytes      = 360
	defaultToolDeferBridgeBytes     = 300
	defaultToolDeferBridgeToolCount = 3
	defaultToolDeferMinEntries      = 15
)

type ToolDeferPlanCard struct {
	Kind                string
	Name                string
	Source              string
	Path                string
	Mode                string
	ToolRefs            []string
	DirectCore          bool
	DeferrableCandidate bool
	PlannedDeferred     bool
	Enabled             bool
	Reason              string
	RiskCodes           []string
	SHA                 string
}

type ToolDeferPlanReport struct {
	Status                         string
	Mode                           string
	ThresholdPct                   int
	ThresholdBytes                 int
	ContextBudgetBytes             int
	EstimatedDirectBytes           int
	EstimatedDeferrableBytes       int
	EstimatedCatalogBytes          int
	EstimatedBridgeBytes           int
	ActivationDecision             string
	ActivationReason               string
	DirectCoreEntries              int
	EnabledCoreEntries             int
	DeferrableCandidateEntries     int
	ToolsetCatalogEntries          int
	MCPCatalogEntries              int
	PlannedDirectEntries           int
	PlannedDeferredEntries         int
	CandidateBridgeTools           int
	PlannedBridgeTools             int
	ToolsetsScanned                int
	MCPSpecsScanned                int
	ModelCallableStructuredTools   bool
	ToolSearchBridgeRuntimeEnabled bool
	DynamicMCPDiscoveryAllowed     bool
	MCPServerLaunchAllowed         bool
	ToolsetActivationSupported     bool
	RawToolSchemasIncluded         bool
	RawToolsetBodiesIncluded       bool
	RawToolsetInstructionsIncluded bool
	RawMCPBodiesIncluded           bool
	RawMCPCommandArgsIncluded      bool
	RawIssueBodiesIncluded         bool
	RawCommentBodiesIncluded       bool
	RawPromptBodiesIncluded        bool
	LLME2ERequiredAfterChange      bool
	ToolValidationStatus           string
	ToolValidationErrors           int
	ToolValidationWarnings         int
	Findings                       []ToolRiskFinding
	HighRiskFindings               int
	WarningRiskFindings            int
	InfoRiskFindings               int
	Cards                          []ToolDeferPlanCard
}

func RenderToolDeferPlanCLIReport(cfg Config, repoContext RepoContext) string {
	return renderToolDeferPlanReport(Event{}, cfg, repoContext, false)
}

func renderToolDeferPlanReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolDeferPlanReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tool Defer Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeToolDeferPlanSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report plans a future Hermes-style progressive-disclosure bridge for GitClaw's repo-reviewed tool catalog. It is advisory in v1: no structured tools are exposed, no MCP server is launched, no toolset is activated, and no raw schemas, toolset instructions, MCP specs, issue bodies, comments, prompts, or tool outputs are printed.\n\n")

	b.WriteString("### Catalog Cards\n")
	writeToolDeferPlanCards(&b, report.Cards)

	b.WriteString("\n### Findings\n")
	writeToolDeferPlanFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildToolDeferPlanReport(cfg Config, repoContext RepoContext) ToolDeferPlanReport {
	validation := ValidateTools(repoContext)
	toolsets := BuildToolsetStoreReport(cfg)
	mcps := BuildMCPReport(cfg)
	budget := cfg.MaxPromptBytes
	if budget <= 0 {
		budget = DefaultConfig().MaxPromptBytes
	}
	thresholdPct := defaultToolDeferThresholdPct
	thresholdBytes := budget * thresholdPct / 100
	report := ToolDeferPlanReport{
		Status:                         "ok",
		Mode:                           defaultToolDeferMode,
		ThresholdPct:                   thresholdPct,
		ThresholdBytes:                 thresholdBytes,
		ContextBudgetBytes:             budget,
		CandidateBridgeTools:           defaultToolDeferBridgeToolCount,
		ToolsetsScanned:                toolsets.Toolsets,
		MCPSpecsScanned:                mcps.Specs,
		ModelCallableStructuredTools:   false,
		ToolSearchBridgeRuntimeEnabled: false,
		DynamicMCPDiscoveryAllowed:     false,
		MCPServerLaunchAllowed:         false,
		ToolsetActivationSupported:     false,
		RawToolSchemasIncluded:         false,
		RawToolsetBodiesIncluded:       false,
		RawToolsetInstructionsIncluded: false,
		RawMCPBodiesIncluded:           false,
		RawMCPCommandArgsIncluded:      false,
		RawIssueBodiesIncluded:         false,
		RawCommentBodiesIncluded:       false,
		RawPromptBodiesIncluded:        false,
		LLME2ERequiredAfterChange:      true,
		ToolValidationStatus:           validation.Status,
		ToolValidationErrors:           validation.Errors,
		ToolValidationWarnings:         validation.Warnings,
	}
	for _, contract := range toolReportContracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		riskCodes := toolRiskCodes(scanToolContractRiskFindings(contract))
		if disabled {
			riskCodes = append(riskCodes, "disabled_by_config")
		}
		if blocked {
			riskCodes = append(riskCodes, "blocked_by_allowlist")
		}
		report.DirectCoreEntries++
		if enabled {
			report.EnabledCoreEntries++
		}
		report.Cards = append(report.Cards, ToolDeferPlanCard{
			Kind:       "builtin-contract",
			Name:       contract.Name,
			Source:     "builtin-gitclaw",
			Path:       "builtin",
			Mode:       contract.Mode,
			DirectCore: true,
			Enabled:    enabled,
			Reason:     "core_deterministic_tool",
			RiskCodes:  uniqueSortedStrings(riskCodes),
			SHA:        shortDocumentHash(strings.Join([]string{contract.Name, contract.Mode, contract.Trigger}, "\n")),
		})
	}
	for _, summary := range toolsets.Summaries {
		enabled := summary.ParseError == "" && len(summary.ResolvedTools) > 0 && len(summary.UnknownTools) == 0 && toolRiskMaxSeverity(summary.RiskFindings) != "high"
		riskCodes := toolRiskCodes(summary.RiskFindings)
		if len(summary.UnknownTools) > 0 {
			riskCodes = append(riskCodes, "unknown_tool_refs")
		}
		if len(summary.DisabledTools) > 0 {
			riskCodes = append(riskCodes, "disabled_tool_refs")
		}
		if len(summary.AllowlistBlockedTools) > 0 {
			riskCodes = append(riskCodes, "allowlist_blocked_tool_refs")
		}
		report.ToolsetCatalogEntries++
		if enabled {
			report.DeferrableCandidateEntries++
		}
		report.Cards = append(report.Cards, ToolDeferPlanCard{
			Kind:                "toolset-profile",
			Name:                summary.Name,
			Source:              "repo-reviewed-toolset",
			Path:                summary.Path,
			Mode:                summary.Mode,
			ToolRefs:            append([]string(nil), summary.ResolvedTools...),
			DeferrableCandidate: enabled,
			Enabled:             enabled,
			Reason:              "repo_reviewed_task_profile",
			RiskCodes:           uniqueSortedStrings(riskCodes),
			SHA:                 summary.SHA,
		})
	}
	for _, spec := range mcps.Cards {
		specBlocked := spec.Activation == "disabled" || pluginRiskMaxSeverity(spec.RiskFindings) == "high" || spec.ParseError != ""
		for _, tool := range spec.ToolAllowlist {
			name := spec.Name + "/" + strings.TrimSpace(tool)
			riskCodes := pluginRiskCodes(spec.RiskFindings)
			if specBlocked {
				riskCodes = append(riskCodes, "mcp_spec_not_deferrable")
			}
			enabled := !specBlocked
			report.MCPCatalogEntries++
			if enabled {
				report.DeferrableCandidateEntries++
			}
			report.Cards = append(report.Cards, ToolDeferPlanCard{
				Kind:                "mcp-tool",
				Name:                name,
				Source:              "repo-reviewed-mcp-metadata",
				Path:                spec.Path,
				Mode:                spec.Activation,
				DeferrableCandidate: enabled,
				Enabled:             enabled,
				Reason:              "mcp_tool_allowlist_ref",
				RiskCodes:           uniqueSortedStrings(riskCodes),
				SHA:                 shortDocumentHash(strings.Join([]string{spec.Path, spec.Name, tool, spec.Activation}, "\n")),
			})
		}
	}

	report.EstimatedDirectBytes = report.DirectCoreEntries * defaultToolDeferEntryBytes
	report.EstimatedDeferrableBytes = report.DeferrableCandidateEntries * defaultToolDeferEntryBytes
	report.EstimatedCatalogBytes = report.EstimatedDirectBytes + report.EstimatedDeferrableBytes
	report.EstimatedBridgeBytes = defaultToolDeferBridgeToolCount * defaultToolDeferBridgeBytes
	report.ActivationDecision = "direct"
	report.ActivationReason = "below_threshold"
	report.PlannedDirectEntries = report.DirectCoreEntries + report.DeferrableCandidateEntries
	if report.DeferrableCandidateEntries == 0 {
		report.ActivationReason = "no_deferrable_catalog_entries"
	}
	if report.DeferrableCandidateEntries >= defaultToolDeferMinEntries || report.EstimatedDeferrableBytes >= report.ThresholdBytes {
		report.ActivationDecision = "bridge_recommended"
		report.ActivationReason = "deferrable_catalog_over_threshold"
		report.PlannedDirectEntries = report.DirectCoreEntries
		report.PlannedDeferredEntries = report.DeferrableCandidateEntries
		report.PlannedBridgeTools = defaultToolDeferBridgeToolCount
	}
	for i := range report.Cards {
		if report.Cards[i].DeferrableCandidate && report.ActivationDecision == "bridge_recommended" {
			report.Cards[i].PlannedDeferred = true
		}
	}
	report.Findings = buildToolDeferPlanFindings(report)
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
	sort.Slice(report.Cards, func(i, j int) bool {
		if report.Cards[i].Kind != report.Cards[j].Kind {
			return report.Cards[i].Kind < report.Cards[j].Kind
		}
		return report.Cards[i].Name < report.Cards[j].Name
	})
	return report
}

func buildToolDeferPlanFindings(report ToolDeferPlanReport) []ToolRiskFinding {
	add := func(findings *[]ToolRiskFinding, severity, code, category, name, field, value string) {
		*findings = append(*findings, ToolRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Kind:     "tool-defer-plan",
			Name:     name,
			Field:    field,
			LineSHA:  shortDocumentHash(value),
		})
	}
	var findings []ToolRiskFinding
	add(&findings, "info", "hermes_progressive_disclosure_threshold_evaluated", "progressive-disclosure", "gitclaw", "threshold", fmt.Sprintf("%d:%d", report.ThresholdPct, report.ThresholdBytes))
	add(&findings, "info", "structured_model_tools_disabled", "tool-exposure", "gitclaw", "model_callable_structured_tools", "false")
	add(&findings, "info", "mcp_runtime_disabled", "runtime-extension", "gitclaw", "mcp_runtime", "disabled")
	if report.DeferrableCandidateEntries == 0 {
		add(&findings, "info", "no_deferrable_catalog_entries", "progressive-disclosure", "gitclaw", "deferrable_catalog_entries", "0")
	}
	if report.ActivationDecision == "bridge_recommended" {
		add(&findings, "warning", "bridge_recommended_for_large_catalog", "progressive-disclosure", "gitclaw", "activation_decision", report.ActivationDecision)
	}
	if report.ToolValidationErrors > 0 {
		add(&findings, "high", "tool_validation_errors_present", "tool-validation", "tools", "validation", report.ToolValidationStatus)
	}
	if report.ToolValidationWarnings > 0 {
		add(&findings, "warning", "tool_validation_warnings_present", "tool-validation", "tools", "validation", report.ToolValidationStatus)
	}
	return findings
}

func writeToolDeferPlanSummary(b *strings.Builder, report ToolDeferPlanReport) {
	fmt.Fprintf(b, "- tool_defer_plan_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- defer_mode: `%s`\n", report.Mode)
	fmt.Fprintf(b, "- threshold_pct: `%d`\n", report.ThresholdPct)
	fmt.Fprintf(b, "- context_budget_bytes: `%d`\n", report.ContextBudgetBytes)
	fmt.Fprintf(b, "- threshold_bytes: `%d`\n", report.ThresholdBytes)
	fmt.Fprintf(b, "- estimated_direct_catalog_bytes: `%d`\n", report.EstimatedDirectBytes)
	fmt.Fprintf(b, "- estimated_deferrable_catalog_bytes: `%d`\n", report.EstimatedDeferrableBytes)
	fmt.Fprintf(b, "- estimated_total_catalog_bytes: `%d`\n", report.EstimatedCatalogBytes)
	fmt.Fprintf(b, "- estimated_bridge_catalog_bytes: `%d`\n", report.EstimatedBridgeBytes)
	fmt.Fprintf(b, "- activation_decision: `%s`\n", report.ActivationDecision)
	fmt.Fprintf(b, "- activation_reason: `%s`\n", report.ActivationReason)
	fmt.Fprintf(b, "- direct_core_entries: `%d`\n", report.DirectCoreEntries)
	fmt.Fprintf(b, "- enabled_core_entries: `%d`\n", report.EnabledCoreEntries)
	fmt.Fprintf(b, "- deferrable_candidate_entries: `%d`\n", report.DeferrableCandidateEntries)
	fmt.Fprintf(b, "- toolset_catalog_entries: `%d`\n", report.ToolsetCatalogEntries)
	fmt.Fprintf(b, "- mcp_catalog_entries: `%d`\n", report.MCPCatalogEntries)
	fmt.Fprintf(b, "- planned_direct_entries: `%d`\n", report.PlannedDirectEntries)
	fmt.Fprintf(b, "- planned_deferred_entries: `%d`\n", report.PlannedDeferredEntries)
	fmt.Fprintf(b, "- candidate_bridge_tools: `%d`\n", report.CandidateBridgeTools)
	fmt.Fprintf(b, "- planned_bridge_tools: `%d`\n", report.PlannedBridgeTools)
	fmt.Fprintf(b, "- toolsets_scanned: `%d`\n", report.ToolsetsScanned)
	fmt.Fprintf(b, "- mcp_specs_scanned: `%d`\n", report.MCPSpecsScanned)
	fmt.Fprintf(b, "- model_callable_structured_tools: `%t`\n", report.ModelCallableStructuredTools)
	fmt.Fprintf(b, "- tool_search_bridge_runtime_enabled: `%t`\n", report.ToolSearchBridgeRuntimeEnabled)
	fmt.Fprintf(b, "- dynamic_mcp_discovery_allowed: `%t`\n", report.DynamicMCPDiscoveryAllowed)
	fmt.Fprintf(b, "- mcp_server_launch_allowed: `%t`\n", report.MCPServerLaunchAllowed)
	fmt.Fprintf(b, "- toolset_activation_supported: `%t`\n", report.ToolsetActivationSupported)
	fmt.Fprintf(b, "- raw_tool_schemas_included: `%t`\n", report.RawToolSchemasIncluded)
	fmt.Fprintf(b, "- raw_toolset_bodies_included: `%t`\n", report.RawToolsetBodiesIncluded)
	fmt.Fprintf(b, "- raw_toolset_instructions_included: `%t`\n", report.RawToolsetInstructionsIncluded)
	fmt.Fprintf(b, "- raw_mcp_bodies_included: `%t`\n", report.RawMCPBodiesIncluded)
	fmt.Fprintf(b, "- raw_mcp_command_args_included: `%t`\n", report.RawMCPCommandArgsIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_tool_defer_plan_change: `%t`\n", report.LLME2ERequiredAfterChange)
	fmt.Fprintf(b, "- tool_validation_status: `%s`\n", report.ToolValidationStatus)
	fmt.Fprintf(b, "- tool_validation_errors: `%d`\n", report.ToolValidationErrors)
	fmt.Fprintf(b, "- tool_validation_warnings: `%d`\n", report.ToolValidationWarnings)
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
}

func writeToolDeferPlanCards(b *strings.Builder, cards []ToolDeferPlanCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(b, "- kind=`%s` name=`%s` source=`%s` path=`%s` mode=`%s` direct_core=`%t` deferrable_candidate=`%t` planned_deferred=`%t` enabled=`%t` reason=`%s` tool_refs=`%s` risk_codes=`%s` sha256_12=`%s`\n",
			card.Kind,
			inlineCode(card.Name),
			inlineCode(card.Source),
			card.Path,
			inlineCode(card.Mode),
			card.DirectCore,
			card.DeferrableCandidate,
			card.PlannedDeferred,
			card.Enabled,
			inlineCode(card.Reason),
			inlineListOrNone(card.ToolRefs),
			inlineListOrNone(card.RiskCodes),
			card.SHA,
		)
	}
}

func writeToolDeferPlanFindings(b *strings.Builder, findings []ToolRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` name=`%s` field=`%s` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			inlineCode(finding.Name),
			finding.Field,
			finding.LineSHA,
		)
	}
}

func isToolDeferPlanRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/tools" {
		return false
	}
	if strings.EqualFold(fields[1], "defer-plan") ||
		strings.EqualFold(fields[1], "deferral") ||
		strings.EqualFold(fields[1], "defer") ||
		strings.EqualFold(fields[1], "tool-search-plan") ||
		strings.EqualFold(fields[1], "progressive-disclosure") {
		return true
	}
	return len(fields) >= 3 &&
		strings.EqualFold(fields[1], "tool-search") &&
		strings.EqualFold(fields[2], "plan")
}
