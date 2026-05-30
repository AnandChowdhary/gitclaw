package gitclaw

import (
	"fmt"
	"strings"
)

type ToolVerifyReport struct {
	Status                        string
	Validation                    ToolValidationReport
	AvailableTools                int
	ReadOnlyContracts             int
	MetadataOnlyContracts         int
	MutatingContracts             int
	ActiveOutputs                 int
	KnownToolOutputs              int
	UnknownToolOutputs            int
	GuidanceFiles                 int
	RepoLocalGuidanceFiles        int
	UnknownGuidanceFiles          int
	ToolOutputsHashed             int
	ToolInputsHashed              int
	RegistryVerification          string
	RuntimePermissionVerification string
	ShellExecutionAllowed         bool
	RepositoryMutationAllowed     bool
	RawBodiesIncluded             bool
	RawInputsIncluded             bool
}

func BuildToolVerifyReport(repoContext RepoContext) ToolVerifyReport {
	validation := ValidateTools(repoContext)
	report := ToolVerifyReport{
		Status:                        validation.Status,
		Validation:                    validation,
		AvailableTools:                len(toolReportContracts),
		ActiveOutputs:                 len(repoContext.ToolOutputs),
		KnownToolOutputs:              len(repoContext.ToolOutputs) - validation.UnknownOutputs,
		UnknownToolOutputs:            validation.UnknownOutputs,
		GuidanceFiles:                 validation.GuidanceFiles,
		RegistryVerification:          "not_configured",
		RuntimePermissionVerification: "static_contracts_only",
		ShellExecutionAllowed:         false,
		RepositoryMutationAllowed:     false,
		RawBodiesIncluded:             false,
		RawInputsIncluded:             false,
	}
	for _, contract := range toolReportContracts {
		switch contract.Mode {
		case "read-only":
			report.ReadOnlyContracts++
		case "metadata-only":
			report.MetadataOnlyContracts++
		default:
			report.MutatingContracts++
		}
	}
	for _, doc := range repoContext.Documents {
		if !isToolGuidanceDocument(doc.Path) {
			continue
		}
		switch toolGuidanceSource(doc.Path) {
		case "repo-local":
			report.RepoLocalGuidanceFiles++
		default:
			report.UnknownGuidanceFiles++
		}
	}
	for _, output := range repoContext.ToolOutputs {
		if strings.TrimSpace(output.Output) != "" {
			report.ToolOutputsHashed++
		}
		if strings.TrimSpace(output.Input) != "" {
			report.ToolInputsHashed++
		}
	}
	if report.UnknownGuidanceFiles > 0 && report.Status == "ok" {
		report.Status = "warn"
	}
	return report
}

func RenderToolVerifyReport(repoContext RepoContext) string {
	return renderToolVerifyReport(Event{}, repoContext, false)
}

func renderToolVerifyReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolVerifyReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_verify_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "deterministic-tool-contracts")
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- read_only_contracts: `%d`\n", report.ReadOnlyContracts)
	fmt.Fprintf(&b, "- metadata_only_contracts: `%d`\n", report.MetadataOnlyContracts)
	fmt.Fprintf(&b, "- mutating_contracts: `%d`\n", report.MutatingContracts)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "- known_tool_outputs: `%d`\n", report.KnownToolOutputs)
	fmt.Fprintf(&b, "- unknown_tool_outputs: `%d`\n", report.UnknownToolOutputs)
	fmt.Fprintf(&b, "- tool_guidance_files: `%d`\n", report.GuidanceFiles)
	fmt.Fprintf(&b, "- repo_local_guidance_files: `%d`\n", report.RepoLocalGuidanceFiles)
	fmt.Fprintf(&b, "- unknown_guidance_files: `%d`\n", report.UnknownGuidanceFiles)
	fmt.Fprintf(&b, "- tool_outputs_hashed: `%d`\n", report.ToolOutputsHashed)
	fmt.Fprintf(&b, "- tool_inputs_hashed: `%d`\n", report.ToolInputsHashed)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(&b, "- runtime_permission_verification: `%s`\n", report.RuntimePermissionVerification)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	writeToolsValidationSummary(&b, report.Validation)
	b.WriteByte('\n')
	b.WriteString("This report verifies GitClaw's deterministic v1 tool contracts and active tool-output metadata. It reports built-in contract modes, guidance-file hashes, and active-output hashes only; raw tool outputs, tool inputs, file bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Trust Cards\n")
	writeToolContractTrustCards(&b, repoContext.ToolOutputs)
	writeToolGuidanceTrustCards(&b, repoContext.Documents)
	writeToolOutputTrustCards(&b, repoContext.ToolOutputs)

	b.WriteString("\n### Verification Findings\n")
	writeToolVerifyFindings(&b, report)
	return strings.TrimSpace(b.String())
}

func writeToolContractTrustCards(b *strings.Builder, outputs []ToolOutput) {
	activeCounts := map[string]int{}
	for _, output := range outputs {
		activeCounts[output.Name]++
	}
	for _, contract := range toolReportContracts {
		fmt.Fprintf(b, "- kind=`contract` name=`%s` source=`builtin-gitclaw` mode=`%s` mutating=`%t` trigger=`%s` active_outputs=`%d`\n",
			contract.Name,
			contract.Mode,
			isMutatingToolContract(contract),
			inlineCode(contract.Trigger),
			activeCounts[contract.Name],
		)
	}
}

func writeToolGuidanceTrustCards(b *strings.Builder, docs []ContextDocument) {
	wrote := false
	for _, doc := range docs {
		if !isToolGuidanceDocument(doc.Path) {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- kind=`guidance` path=`%s` source=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n",
			doc.Path,
			toolGuidanceSource(doc.Path),
			len(doc.Body),
			lineCount(doc.Body),
			shortDocumentHash(doc.Body),
		)
	}
	if !wrote {
		b.WriteString("- kind=`guidance` none\n")
	}
}

func writeToolOutputTrustCards(b *strings.Builder, outputs []ToolOutput) {
	if len(outputs) == 0 {
		b.WriteString("- kind=`active-output` none\n")
		return
	}
	contracts := toolContractNameSet()
	for _, output := range outputs {
		fmt.Fprintf(b, "- kind=`active-output` name=`%s` contract_known=`%t` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s`\n",
			output.Name,
			contracts[output.Name],
			shortDocumentHash(output.Input),
			len(output.Output),
			lineCount(output.Output),
			shortDocumentHash(output.Output),
		)
	}
}

func writeToolVerifyFindings(b *strings.Builder, report ToolVerifyReport) {
	wrote := false
	if report.RegistryVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`tool_registry_verification_not_configured` detail=`external tool registry signatures are not part of GitClaw v1 verification`\n")
		wrote = true
	}
	if report.RuntimePermissionVerification == "static_contracts_only" {
		b.WriteString("- severity=`info` code=`runtime_permission_verification_static_only` detail=`GitClaw verifies built-in deterministic contract modes and metadata, not external runtime sandbox attestations`\n")
		wrote = true
	}
	if report.UnknownGuidanceFiles > 0 {
		b.WriteString("- severity=`warning` code=`unknown_tool_guidance_source` detail=`one or more tool guidance files are outside known repo-local roots`\n")
		wrote = true
	}
	for _, finding := range report.Validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` name=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Name, inlineCode(finding.Detail))
		wrote = true
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func toolContractNameSet() map[string]bool {
	names := map[string]bool{}
	for _, contract := range toolReportContracts {
		names[contract.Name] = true
	}
	return names
}

func isMutatingToolContract(contract toolContract) bool {
	return contract.Mode != "read-only" && contract.Mode != "metadata-only"
}

func isToolGuidanceDocument(path string) bool {
	return path == ".gitclaw/TOOLS.md" || strings.HasSuffix(path, "/TOOLS.md")
}

func toolGuidanceSource(path string) string {
	if path == ".gitclaw/TOOLS.md" {
		return "repo-local"
	}
	return "unknown"
}
