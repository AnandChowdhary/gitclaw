package gitclaw

import (
	"fmt"
	"strings"
)

func RenderToolCatalogCLIReport(cfg Config, repoContext RepoContext) string {
	return renderToolCatalogReport(Event{}, cfg, repoContext, false)
}

func renderToolCatalogReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolDeferPlanReport(cfg, repoContext)
	risk := BuildToolRiskReport(repoContext)
	activeCounts := toolActiveOutputCounts(repoContext.ToolOutputs)

	var b strings.Builder
	b.WriteString("## GitClaw Tools Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_catalog_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-progressive-disclosure")
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "deterministic-tools-toolsets-mcp")
	fmt.Fprintf(&b, "- cataloged_entries: `%d`\n", len(report.Cards))
	fmt.Fprintf(&b, "- direct_core_entries: `%d`\n", report.DirectCoreEntries)
	fmt.Fprintf(&b, "- enabled_core_entries: `%d`\n", report.EnabledCoreEntries)
	fmt.Fprintf(&b, "- deferrable_candidate_entries: `%d`\n", report.DeferrableCandidateEntries)
	fmt.Fprintf(&b, "- toolset_catalog_entries: `%d`\n", report.ToolsetCatalogEntries)
	fmt.Fprintf(&b, "- mcp_catalog_entries: `%d`\n", report.MCPCatalogEntries)
	fmt.Fprintf(&b, "- planned_direct_entries: `%d`\n", report.PlannedDirectEntries)
	fmt.Fprintf(&b, "- planned_deferred_entries: `%d`\n", report.PlannedDeferredEntries)
	fmt.Fprintf(&b, "- candidate_bridge_tools: `%d`\n", report.CandidateBridgeTools)
	fmt.Fprintf(&b, "- planned_bridge_tools: `%d`\n", report.PlannedBridgeTools)
	fmt.Fprintf(&b, "- activation_decision: `%s`\n", report.ActivationDecision)
	fmt.Fprintf(&b, "- activation_reason: `%s`\n", report.ActivationReason)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", enabledToolCount(repoContext))
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", disabledToolCount(repoContext))
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", allowlistBlockedToolCount(repoContext))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- model_callable_structured_tools: `%t`\n", report.ModelCallableStructuredTools)
	fmt.Fprintf(&b, "- tool_search_bridge_runtime_enabled: `%t`\n", report.ToolSearchBridgeRuntimeEnabled)
	fmt.Fprintf(&b, "- schema_describe_required: `%t`\n", report.PlannedDeferredEntries > 0)
	fmt.Fprintf(&b, "- dynamic_mcp_discovery_allowed: `%t`\n", report.DynamicMCPDiscoveryAllowed)
	fmt.Fprintf(&b, "- mcp_server_launch_allowed: `%t`\n", report.MCPServerLaunchAllowed)
	fmt.Fprintf(&b, "- toolset_activation_supported: `%t`\n", report.ToolsetActivationSupported)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", report.RawToolSchemasIncluded)
	fmt.Fprintf(&b, "- raw_toolset_bodies_included: `%t`\n", report.RawToolsetBodiesIncluded)
	fmt.Fprintf(&b, "- raw_toolset_instructions_included: `%t`\n", report.RawToolsetInstructionsIncluded)
	fmt.Fprintf(&b, "- raw_mcp_bodies_included: `%t`\n", report.RawMCPBodiesIncluded)
	fmt.Fprintf(&b, "- raw_mcp_command_args_included: `%t`\n", report.RawMCPCommandArgsIncluded)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_catalog_change: `%t`\n", true)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", report.ToolValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", report.ToolValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", report.ToolValidationWarnings)
	writeToolRiskSummary(&b, risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This compact catalog follows OpenClaw and Hermes progressive-disclosure boundaries: core GitClaw tools stay directly visible, repo-reviewed toolsets and MCP allowlists are cataloged as metadata, and full schemas or instructions would be loaded only through a future reviewed bridge. Raw tool schemas, toolset instructions, MCP command args, tool inputs, tool outputs, issue bodies, comments, prompts, credentials, and secret values are not included.\n\n")

	b.WriteString("### Catalog Entries\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeToolCatalogEntry(&b, card, activeCounts[card.Name])
		}
	}

	b.WriteString("\n### Catalog Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", toolCatalogGate(report.ToolValidationStatus))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", toolCatalogGate(risk.Status))
	fmt.Fprintf(&b, "- activation_gate=`%s`\n", report.ActivationDecision)
	fmt.Fprintf(&b, "- tool_search_bridge_gate=`%s`\n", boolGate(report.ToolSearchBridgeRuntimeEnabled))
	fmt.Fprintf(&b, "- structured_tool_gate=`%s`\n", boolGate(report.ModelCallableStructuredTools))
	fmt.Fprintf(&b, "- mcp_runtime_gate=`%s`\n", boolGate(report.MCPServerLaunchAllowed))
	fmt.Fprintf(&b, "- toolset_activation_gate=`%s`\n", boolGate(report.ToolsetActivationSupported))
	fmt.Fprintf(&b, "- schema_body_gate=`%s`\n", "sha256_12")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	return strings.TrimSpace(b.String())
}

func writeToolCatalogEntry(b *strings.Builder, card ToolDeferPlanCard, activeOutputs int) {
	fmt.Fprintf(
		b,
		"- kind=`%s` name=`%s` source=`%s` path=`%s` mode=`%s` enabled=`%t` direct_core=`%t` deferrable_candidate=`%t` planned_deferred=`%t` catalog_mode=`%s` schema_visibility=`%s` active_outputs=`%d` tool_refs_count=`%d` reason=`%s` risk_codes=`%s` reason_codes=`%s` sha256_12=`%s`\n",
		card.Kind,
		inlineCode(card.Name),
		inlineCode(card.Source),
		card.Path,
		inlineCode(card.Mode),
		card.Enabled,
		card.DirectCore,
		card.DeferrableCandidate,
		card.PlannedDeferred,
		toolCatalogMode(card),
		toolCatalogSchemaVisibility(card),
		activeOutputs,
		len(card.ToolRefs),
		inlineCode(card.Reason),
		inlineListOrNone(card.RiskCodes),
		toolCatalogReasonCodeList(toolCatalogReasonCodes(card, activeOutputs)),
		card.SHA,
	)
}

func toolCatalogMode(card ToolDeferPlanCard) string {
	switch {
	case card.DirectCore:
		return "direct-core"
	case card.PlannedDeferred:
		return "deferred-candidate"
	case card.DeferrableCandidate:
		return "deferrable-direct"
	default:
		return "metadata-only"
	}
}

func toolCatalogSchemaVisibility(card ToolDeferPlanCard) string {
	switch {
	case card.DirectCore:
		return "direct-contract"
	case card.PlannedDeferred:
		return "describe-on-demand"
	default:
		return "compact-metadata-only"
	}
}

func toolCatalogReasonCodes(card ToolDeferPlanCard, activeOutputs int) []string {
	var reasons []string
	reasons = append(reasons, strings.ReplaceAll(card.Kind, "-", "_"))
	if card.Enabled {
		reasons = append(reasons, "enabled")
	} else {
		reasons = append(reasons, "not_enabled")
	}
	if card.DirectCore {
		reasons = append(reasons, "direct_core")
	}
	if card.DeferrableCandidate {
		reasons = append(reasons, "deferrable_candidate")
	} else {
		reasons = append(reasons, "not_deferrable")
	}
	if card.PlannedDeferred {
		reasons = append(reasons, "planned_deferred")
	} else {
		reasons = append(reasons, "planned_direct")
	}
	if activeOutputs > 0 {
		reasons = append(reasons, "active_outputs")
	} else {
		reasons = append(reasons, "no_active_outputs")
	}
	if len(card.RiskCodes) > 0 {
		reasons = append(reasons, "risk_codes")
	}
	return uniqueSortedStrings(reasons)
}

func toolCatalogReasonCodeList(reasons []string) string {
	if len(reasons) == 0 {
		return "none"
	}
	return strings.Join(reasons, ", ")
}

func toolCatalogGate(status string) string {
	switch status {
	case "ok":
		return "pass"
	case "warn":
		return "warn"
	case "high", "error":
		return "block"
	default:
		return "unknown"
	}
}

func boolGate(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
