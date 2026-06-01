package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

const toolSnapshotVersion = "gitclaw-tool-snapshot-v1"

type ToolSnapshotReport struct {
	Status                            string
	SnapshotVersion                   string
	SnapshotScope                     string
	SnapshotSHA                       string
	SnapshotEntries                   int
	CatalogEntries                    int
	BuiltinContractEntries            int
	ToolsetProfileEntries             int
	MCPToolEntries                    int
	GuidanceEntries                   int
	ActiveOutputEntries               int
	PromptVisibleEntries              int
	AvailableTools                    int
	EnabledTools                      int
	DisabledTools                     int
	AllowlistBlockedTools             int
	ActiveToolOutputs                 int
	KnownToolOutputs                  int
	UnknownToolOutputs                int
	ToolsetsScanned                   int
	MCPSpecsScanned                   int
	RegistryContactAllowed            bool
	DynamicMCPDiscoveryAllowed        bool
	MCPServerLaunchAllowed            bool
	ToolsetActivationSupported        bool
	ModelCallableStructuredTools      bool
	ShellExecutionAllowed             bool
	RepositoryMutationAllowed         bool
	RawToolSchemasIncluded            bool
	RawToolsetBodiesIncluded          bool
	RawToolsetInstructionsIncluded    bool
	RawMCPBodiesIncluded              bool
	RawMCPCommandArgsIncluded         bool
	RawToolOutputsIncluded            bool
	RawToolInputsIncluded             bool
	LLME2ERequiredAfterSnapshotChange bool
	Validation                        ToolValidationReport
	Risk                              ToolRiskReport
	Cards                             []ToolSnapshotCard
}

type ToolSnapshotCard struct {
	Position            int
	Kind                string
	Name                string
	Source              string
	Path                string
	Mode                string
	Enabled             bool
	PromptVisible       bool
	DirectCore          bool
	DeferrableCandidate bool
	PlannedDeferred     bool
	ActiveOutputs       int
	ToolRefs            int
	InputSHA            string
	OutputBytes         int
	OutputLines         int
	OutputSHA           string
	Bytes               int
	Lines               int
	SHA                 string
	RiskFindings        int
	RiskMaxSeverity     string
	RiskCodes           []string
}

func BuildToolSnapshotReport(cfg Config, repoContext RepoContext) ToolSnapshotReport {
	deferPlan := BuildToolDeferPlanReport(cfg, repoContext)
	verify := BuildToolVerifyReport(repoContext)
	toolsets := BuildToolsetStoreReport(cfg)
	report := ToolSnapshotReport{
		Status:                            toolSnapshotStatus(verify.Status, deferPlan.Status, toolsets.Status),
		SnapshotVersion:                   toolSnapshotVersion,
		SnapshotScope:                     "deterministic-tools-toolsets-mcp-outputs",
		CatalogEntries:                    len(deferPlan.Cards),
		AvailableTools:                    verify.AvailableTools,
		EnabledTools:                      verify.EnabledTools,
		DisabledTools:                     verify.DisabledTools,
		AllowlistBlockedTools:             verify.AllowlistBlockedTools,
		ActiveToolOutputs:                 verify.ActiveOutputs,
		KnownToolOutputs:                  verify.KnownToolOutputs,
		UnknownToolOutputs:                verify.UnknownToolOutputs,
		ToolsetsScanned:                   deferPlan.ToolsetsScanned,
		MCPSpecsScanned:                   deferPlan.MCPSpecsScanned,
		RegistryContactAllowed:            false,
		DynamicMCPDiscoveryAllowed:        deferPlan.DynamicMCPDiscoveryAllowed,
		MCPServerLaunchAllowed:            deferPlan.MCPServerLaunchAllowed,
		ToolsetActivationSupported:        deferPlan.ToolsetActivationSupported,
		ModelCallableStructuredTools:      deferPlan.ModelCallableStructuredTools,
		ShellExecutionAllowed:             false,
		RepositoryMutationAllowed:         false,
		RawToolSchemasIncluded:            deferPlan.RawToolSchemasIncluded,
		RawToolsetBodiesIncluded:          deferPlan.RawToolsetBodiesIncluded,
		RawToolsetInstructionsIncluded:    deferPlan.RawToolsetInstructionsIncluded,
		RawMCPBodiesIncluded:              deferPlan.RawMCPBodiesIncluded,
		RawMCPCommandArgsIncluded:         deferPlan.RawMCPCommandArgsIncluded,
		RawToolOutputsIncluded:            false,
		RawToolInputsIncluded:             false,
		LLME2ERequiredAfterSnapshotChange: true,
		Validation:                        verify.Validation,
		Risk:                              verify.Risk,
	}

	activeCounts := toolActiveOutputCounts(repoContext.ToolOutputs)
	for _, card := range deferPlan.Cards {
		riskCodes := append([]string(nil), card.RiskCodes...)
		report.addCard(ToolSnapshotCard{
			Kind:                card.Kind,
			Name:                card.Name,
			Source:              card.Source,
			Path:                card.Path,
			Mode:                card.Mode,
			Enabled:             card.Enabled,
			PromptVisible:       card.DirectCore,
			DirectCore:          card.DirectCore,
			DeferrableCandidate: card.DeferrableCandidate,
			PlannedDeferred:     card.PlannedDeferred,
			ActiveOutputs:       activeCounts[card.Name],
			ToolRefs:            len(card.ToolRefs),
			SHA:                 noneIfEmpty(card.SHA),
			RiskMaxSeverity:     "none",
			RiskCodes:           uniqueSortedStrings(riskCodes),
		})
	}

	guidanceDocs := append([]ContextDocument(nil), repoContext.Documents...)
	sort.Slice(guidanceDocs, func(i, j int) bool { return guidanceDocs[i].Path < guidanceDocs[j].Path })
	for _, doc := range guidanceDocs {
		if !isToolGuidanceDocument(doc.Path) {
			continue
		}
		findings := scanToolRiskText("guidance", doc.Path, doc.Path, "body", doc.Body)
		report.addCard(ToolSnapshotCard{
			Kind:            "guidance",
			Name:            doc.Path,
			Source:          toolGuidanceSource(doc.Path),
			Path:            doc.Path,
			Mode:            "metadata-only",
			Enabled:         true,
			PromptVisible:   true,
			Bytes:           len(doc.Body),
			Lines:           lineCount(doc.Body),
			SHA:             shortDocumentHash(doc.Body),
			RiskFindings:    len(findings),
			RiskMaxSeverity: toolRiskMaxSeverity(findings),
			RiskCodes:       toolRiskCodes(findings),
		})
	}

	outputs := append([]ToolOutput(nil), repoContext.ToolOutputs...)
	sort.Slice(outputs, func(i, j int) bool {
		left := strings.Join([]string{outputs[i].Name, shortDocumentHash(outputs[i].Input), shortDocumentHash(outputs[i].Output)}, "\x00")
		right := strings.Join([]string{outputs[j].Name, shortDocumentHash(outputs[j].Input), shortDocumentHash(outputs[j].Output)}, "\x00")
		return left < right
	})
	knownContracts := toolContractNameSet()
	for _, output := range outputs {
		findings := scanToolOutputRiskFindings(output, knownContracts[output.Name])
		report.addCard(ToolSnapshotCard{
			Kind:            "active-output",
			Name:            output.Name,
			Source:          "prompt-context",
			Path:            "prompt-context",
			Mode:            "metadata-only",
			Enabled:         knownContracts[output.Name],
			PromptVisible:   true,
			InputSHA:        shortDocumentHash(output.Input),
			OutputBytes:     len(output.Output),
			OutputLines:     lineCount(output.Output),
			OutputSHA:       shortDocumentHash(output.Output),
			SHA:             shortDocumentHash(strings.Join([]string{output.Name, output.Input, output.Output}, "\n")),
			RiskFindings:    len(findings),
			RiskMaxSeverity: toolRiskMaxSeverity(findings),
			RiskCodes:       toolRiskCodes(findings),
		})
	}

	for i := range report.Cards {
		report.Cards[i].Position = i + 1
	}
	report.SnapshotEntries = len(report.Cards)
	report.SnapshotSHA = toolSnapshotManifestHash(report.Cards)
	return report
}

func RenderToolSnapshotCLIReport(cfg Config, repoContext RepoContext) string {
	return renderToolSnapshotReport(Event{}, cfg, repoContext, false)
}

func renderToolSnapshotReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolSnapshotReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Snapshot Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_snapshot_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", report.SnapshotVersion)
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", report.SnapshotScope)
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", report.SnapshotSHA)
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", report.SnapshotEntries)
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", report.CatalogEntries)
	fmt.Fprintf(&b, "- builtin_contract_entries: `%d`\n", report.BuiltinContractEntries)
	fmt.Fprintf(&b, "- toolset_profile_entries: `%d`\n", report.ToolsetProfileEntries)
	fmt.Fprintf(&b, "- mcp_tool_entries: `%d`\n", report.MCPToolEntries)
	fmt.Fprintf(&b, "- guidance_entries: `%d`\n", report.GuidanceEntries)
	fmt.Fprintf(&b, "- active_output_entries: `%d`\n", report.ActiveOutputEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", report.PromptVisibleEntries)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", report.EnabledTools)
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", report.DisabledTools)
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", report.AllowlistBlockedTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(&b, "- known_tool_outputs: `%d`\n", report.KnownToolOutputs)
	fmt.Fprintf(&b, "- unknown_tool_outputs: `%d`\n", report.UnknownToolOutputs)
	fmt.Fprintf(&b, "- toolsets_scanned: `%d`\n", report.ToolsetsScanned)
	fmt.Fprintf(&b, "- mcp_specs_scanned: `%d`\n", report.MCPSpecsScanned)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(&b, "- dynamic_mcp_discovery_allowed: `%t`\n", report.DynamicMCPDiscoveryAllowed)
	fmt.Fprintf(&b, "- mcp_server_launch_allowed: `%t`\n", report.MCPServerLaunchAllowed)
	fmt.Fprintf(&b, "- toolset_activation_supported: `%t`\n", report.ToolsetActivationSupported)
	fmt.Fprintf(&b, "- model_callable_structured_tools: `%t`\n", report.ModelCallableStructuredTools)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_tool_schemas_included: `%t`\n", report.RawToolSchemasIncluded)
	fmt.Fprintf(&b, "- raw_toolset_bodies_included: `%t`\n", report.RawToolsetBodiesIncluded)
	fmt.Fprintf(&b, "- raw_toolset_instructions_included: `%t`\n", report.RawToolsetInstructionsIncluded)
	fmt.Fprintf(&b, "- raw_mcp_bodies_included: `%t`\n", report.RawMCPBodiesIncluded)
	fmt.Fprintf(&b, "- raw_mcp_command_args_included: `%t`\n", report.RawMCPCommandArgsIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", report.RawToolInputsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_snapshot_change: `%t`\n", report.LLME2ERequiredAfterSnapshotChange)
	writeToolsValidationSummary(&b, report.Validation)
	writeToolRiskSummary(&b, report.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report fingerprints GitClaw's deterministic tool surface in the spirit of OpenClaw tool policies and Hermes tool/profile catalogs. It emits contract, toolset, MCP allowlist, guidance, and active-output metadata plus one composite snapshot hash only; raw schemas, instructions, MCP command args, tool inputs, tool outputs, issue bodies, comments, prompts, credentials, and secret values are not included.\n\n")

	b.WriteString("### Snapshot Entries\n")
	writeToolSnapshotCards(&b, report.Cards)

	b.WriteString("\n### Snapshot Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", toolCatalogGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", toolCatalogGate(report.Risk.Status))
	b.WriteString("- registry_gate=`disabled`\n")
	b.WriteString("- dynamic_mcp_discovery_gate=`disabled`\n")
	b.WriteString("- mcp_runtime_gate=`disabled`\n")
	b.WriteString("- toolset_activation_gate=`disabled`\n")
	b.WriteString("- structured_tool_gate=`disabled`\n")
	b.WriteString("- shell_execution_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- snapshot_hash_gate=`composite-sha256_12`\n")
	return strings.TrimSpace(b.String())
}

func writeToolSnapshotCards(b *strings.Builder, cards []ToolSnapshotCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		inputSHA := card.InputSHA
		if inputSHA == "" {
			inputSHA = "none"
		}
		outputSHA := card.OutputSHA
		if outputSHA == "" {
			outputSHA = "none"
		}
		fmt.Fprintf(
			b,
			"- position=`%d` kind=`%s` name=`%s` source=`%s` path=`%s` mode=`%s` enabled=`%t` prompt_visible=`%t` direct_core=`%t` deferrable_candidate=`%t` planned_deferred=`%t` active_outputs=`%d` tool_refs=`%d` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			card.Position,
			card.Kind,
			inlineCode(card.Name),
			inlineCode(card.Source),
			card.Path,
			inlineCode(card.Mode),
			card.Enabled,
			card.PromptVisible,
			card.DirectCore,
			card.DeferrableCandidate,
			card.PlannedDeferred,
			card.ActiveOutputs,
			card.ToolRefs,
			inputSHA,
			card.OutputBytes,
			card.OutputLines,
			outputSHA,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.RiskFindings,
			noneIfEmpty(card.RiskMaxSeverity),
			inlineListOrNone(card.RiskCodes),
		)
	}
}

func (r *ToolSnapshotReport) addCard(card ToolSnapshotCard) {
	if card.SHA == "" {
		card.SHA = "none"
	}
	if card.RiskMaxSeverity == "" {
		card.RiskMaxSeverity = "none"
	}
	r.Cards = append(r.Cards, card)
	switch card.Kind {
	case "builtin-contract":
		r.BuiltinContractEntries++
	case "toolset-profile":
		r.ToolsetProfileEntries++
	case "mcp-tool":
		r.MCPToolEntries++
	case "guidance":
		r.GuidanceEntries++
	case "active-output":
		r.ActiveOutputEntries++
	}
	if card.PromptVisible {
		r.PromptVisibleEntries++
	}
}

func toolSnapshotManifestHash(cards []ToolSnapshotCard) string {
	var b strings.Builder
	b.WriteString(toolSnapshotVersion)
	b.WriteByte('\n')
	for _, card := range cards {
		fmt.Fprintf(&b, "%03d|%s|%s|%s|%s|%s|%t|%t|%t|%t|%t|%d|%d|%s|%s|%d|%d|%s|%d|%d|%s|%d|%s|%s\n",
			card.Position,
			card.Kind,
			card.Name,
			card.Source,
			card.Path,
			card.Mode,
			card.Enabled,
			card.PromptVisible,
			card.DirectCore,
			card.DeferrableCandidate,
			card.PlannedDeferred,
			card.ActiveOutputs,
			card.ToolRefs,
			card.InputSHA,
			card.OutputSHA,
			card.OutputBytes,
			card.OutputLines,
			card.SHA,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.RiskFindings,
			card.RiskMaxSeverity,
			strings.Join(card.RiskCodes, ","),
		)
	}
	return shortDocumentHash(b.String())
}

func toolSnapshotStatus(statuses ...string) string {
	best := "ok"
	bestRank := toolSnapshotStatusRank(best)
	for _, status := range statuses {
		rank := toolSnapshotStatusRank(status)
		if rank > bestRank {
			best = status
			bestRank = rank
		}
	}
	return best
}

func toolSnapshotStatusRank(status string) int {
	switch status {
	case "error":
		return 4
	case "high":
		return 3
	case "warn":
		return 2
	case "ok":
		return 1
	default:
		return 0
	}
}

func isToolSnapshotRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/tools" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return true
	default:
		return false
	}
}
