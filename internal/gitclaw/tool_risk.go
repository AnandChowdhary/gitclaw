package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ToolRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Name     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type ToolRiskReport struct {
	Status                        string
	AvailableTools                int
	ScannedContracts              int
	ActiveOutputs                 int
	ScannedActiveOutputs          int
	GuidanceFiles                 int
	SurfacesWithRiskFindings      int
	Findings                      []ToolRiskFinding
	HighRiskFindings              int
	WarningRiskFindings           int
	InfoRiskFindings              int
	RegistryVerification          string
	RuntimePermissionVerification string
	ShellExecutionAllowed         bool
	RepositoryMutationAllowed     bool
	RawBodiesIncluded             bool
	RawInputsIncluded             bool
	LLME2ERequiredAfterRiskChange bool
}

type toolRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var toolRiskRules = []toolRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
			"developer message",
		},
	},
	{
		Severity: "high",
		Code:     "secret_exfiltration_instruction",
		Category: "data-exfiltration",
		Any: []string{
			"exfiltrate",
			"leak secrets",
			"send secrets",
			"upload secrets",
			"steal secrets",
		},
	},
	{
		Severity: "high",
		Code:     "credential_material_exposed",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"api_key=",
			"private_key=",
			"-----begin private key-----",
			"-----begin openssh private key-----",
		},
	},
	{
		Severity: "high",
		Code:     "unreviewed_host_execution",
		Category: "host-execution",
		Any: []string{
			"rm -rf",
			"bash -c",
			"python -c",
			"curl http",
			"wget http",
			"execute shell command",
			"shell_execution_allowed: true",
			"shell_execution_allowed=true",
		},
	},
	{
		Severity: "high",
		Code:     "repository_mutation_enabled",
		Category: "repository-mutation",
		Any: []string{
			"repository_mutation_allowed: true",
			"repository_mutation_allowed=true",
			"contents: write",
			"commit and push without",
			"open pull request without",
			"write files without",
		},
	},
	{
		Severity: "warning",
		Code:     "remote_exfiltration_instruction",
		Category: "network-exfiltration",
		Any: []string{
			"send to webhook",
			"upload to http",
			"post to http",
			"attacker-controlled webhook",
			"send to telegram",
			"send to slack",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_tool_loop",
		Category: "token-or-tool-amplification",
		Any: []string{
			"retry forever",
			"loop forever",
			"infinite loop",
			"while true",
			"never stop",
			"continue indefinitely",
			"keep calling",
		},
	},
	{
		Severity: "info",
		Code:     "credential_reference",
		Category: "credential-handling",
		Any: []string{
			"api key",
			"private key",
			"github_token",
			"github token",
		},
		All: []string{
			"tool",
		},
	},
}

func scanToolRiskText(kind, name, path, field, body string) []ToolRiskFinding {
	var findings []ToolRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range toolRiskRules {
			if !toolRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, ToolRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Name:     name,
				Path:     path,
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortToolRiskFindings(findings)
	return findings
}

func toolRiskRuleMatches(lowerLine string, rule toolRiskRule) bool {
	for _, required := range rule.All {
		if !strings.Contains(lowerLine, required) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func BuildToolRiskReport(repoContext RepoContext) ToolRiskReport {
	report := ToolRiskReport{
		Status:                        "ok",
		AvailableTools:                len(toolReportContracts),
		ScannedContracts:              len(toolReportContracts),
		ActiveOutputs:                 len(repoContext.ToolOutputs),
		ScannedActiveOutputs:          len(repoContext.ToolOutputs),
		GuidanceFiles:                 toolGuidanceDocumentCount(repoContext.Documents),
		RegistryVerification:          "not_configured",
		RuntimePermissionVerification: "static_contracts_only",
		ShellExecutionAllowed:         false,
		RepositoryMutationAllowed:     false,
		RawBodiesIncluded:             false,
		RawInputsIncluded:             false,
		LLME2ERequiredAfterRiskChange: true,
	}
	contracts := toolContractNameSet()
	for _, contract := range toolReportContracts {
		report.Findings = append(report.Findings, scanToolContractRiskFindings(contract)...)
	}
	for _, doc := range repoContext.Documents {
		if !isToolGuidanceDocument(doc.Path) {
			continue
		}
		report.Findings = append(report.Findings, scanToolRiskText("guidance", doc.Path, doc.Path, "body", doc.Body)...)
	}
	for _, output := range repoContext.ToolOutputs {
		report.Findings = append(report.Findings, scanToolOutputRiskFindings(output, contracts[output.Name])...)
	}
	sortToolRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = toolRiskSurfaceCount(report.Findings)
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

func scanToolContractRiskFindings(contract toolContract) []ToolRiskFinding {
	var findings []ToolRiskFinding
	if isMutatingToolContract(contract) {
		findings = append(findings, ToolRiskFinding{
			Severity: "high",
			Code:     "mutating_tool_contract",
			Category: "repository-mutation",
			Kind:     "contract",
			Name:     contract.Name,
			Field:    "mode",
			Line:     2,
			LineSHA:  shortDocumentHash(contract.Mode),
		})
	}
	body := strings.Join([]string{contract.Name, contract.Mode, contract.Trigger}, "\n")
	findings = append(findings, scanToolRiskText("contract", contract.Name, "", "metadata", body)...)
	sortToolRiskFindings(findings)
	return findings
}

func scanToolOutputRiskFindings(output ToolOutput, contractKnown bool) []ToolRiskFinding {
	var findings []ToolRiskFinding
	if !contractKnown {
		findings = append(findings, ToolRiskFinding{
			Severity: "high",
			Code:     "unknown_tool_output",
			Category: "tool-provenance",
			Kind:     "active-output",
			Name:     output.Name,
			Field:    "contract",
			Line:     0,
			LineSHA:  shortDocumentHash(output.Name),
		})
	}
	if strings.TrimSpace(output.Input) != "" {
		findings = append(findings, scanToolRiskText("active-output", output.Name, "", "input", output.Input)...)
	}
	if strings.TrimSpace(output.Output) != "" {
		findings = append(findings, scanToolRiskText("active-output", output.Name, "", "output", output.Output)...)
	}
	sortToolRiskFindings(findings)
	return findings
}

func RenderToolRiskReport(repoContext RepoContext) string {
	return renderToolRiskReport(Event{}, repoContext, false)
}

func renderToolRiskReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildToolRiskReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeToolRiskSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report scans deterministic tool contracts, repo-local tool guidance, and active prompt-visible tool output metadata for body-free risk categories inspired by OpenClaw/Hermes tool, MCP, plugin, and permission safety boundaries. It reports only names, paths, counts, finding codes, severities, and hashes; raw tool outputs, tool inputs, file bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Tool Risk Cards\n")
	writeToolRiskCards(&b, repoContext)

	b.WriteString("\n### Risk Findings\n")
	writeToolRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeToolRiskSummary(b *strings.Builder, report ToolRiskReport) {
	fmt.Fprintf(b, "- tool_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(b, "- scanned_contracts: `%d`\n", report.ScannedContracts)
	fmt.Fprintf(b, "- active_tool_outputs: `%d`\n", report.ActiveOutputs)
	fmt.Fprintf(b, "- scanned_active_outputs: `%d`\n", report.ScannedActiveOutputs)
	fmt.Fprintf(b, "- tool_guidance_files: `%d`\n", report.GuidanceFiles)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- tool_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(b, "- runtime_permission_verification: `%s`\n", report.RuntimePermissionVerification)
	fmt.Fprintf(b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_tool_risk_change: `%t`\n", report.LLME2ERequiredAfterRiskChange)
}

func writeToolRiskCards(b *strings.Builder, repoContext RepoContext) {
	for _, contract := range toolReportContracts {
		writeToolContractRiskCard(b, contract, repoContext)
	}
	wroteGuidance := false
	for _, doc := range repoContext.Documents {
		if !isToolGuidanceDocument(doc.Path) {
			continue
		}
		wroteGuidance = true
		writeToolGuidanceRiskCard(b, doc)
	}
	if !wroteGuidance {
		b.WriteString("- kind=`guidance` none\n")
	}
	if len(repoContext.ToolOutputs) == 0 {
		b.WriteString("- kind=`active-output` none\n")
		return
	}
	contracts := toolContractNameSet()
	for _, output := range repoContext.ToolOutputs {
		writeToolOutputRiskCard(b, output, contracts[output.Name])
	}
}

func writeToolContractRiskCard(b *strings.Builder, contract toolContract, repoContext RepoContext) {
	findings := scanToolContractRiskFindings(contract)
	enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
	fmt.Fprintf(
		b,
		"- kind=`contract` name=`%s` source=`builtin-gitclaw` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` mode=`%s` mutating=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		contract.Name,
		enabled,
		disabled,
		blocked,
		contract.Mode,
		isMutatingToolContract(contract),
		len(findings),
		toolRiskMaxSeverity(findings),
		inlineListOrNone(toolRiskCodes(findings)),
		inlineListOrNone(toolRiskLineHashes(findings)),
	)
}

func writeToolGuidanceRiskCard(b *strings.Builder, doc ContextDocument) {
	findings := scanToolRiskText("guidance", doc.Path, doc.Path, "body", doc.Body)
	fmt.Fprintf(
		b,
		"- kind=`guidance` path=`%s` source=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		doc.Path,
		toolGuidanceSource(doc.Path),
		len(doc.Body),
		lineCount(doc.Body),
		shortDocumentHash(doc.Body),
		len(findings),
		toolRiskMaxSeverity(findings),
		inlineListOrNone(toolRiskCodes(findings)),
		inlineListOrNone(toolRiskLineHashes(findings)),
	)
}

func writeToolOutputRiskCard(b *strings.Builder, output ToolOutput, contractKnown bool) {
	findings := scanToolOutputRiskFindings(output, contractKnown)
	fmt.Fprintf(
		b,
		"- kind=`active-output` name=`%s` contract_known=`%t` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		output.Name,
		contractKnown,
		shortDocumentHash(output.Input),
		len(output.Output),
		lineCount(output.Output),
		shortDocumentHash(output.Output),
		len(findings),
		toolRiskMaxSeverity(findings),
		inlineListOrNone(toolRiskCodes(findings)),
		inlineListOrNone(toolRiskLineHashes(findings)),
	)
}

func writeToolRiskFindings(b *strings.Builder, findings []ToolRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			finding.Name,
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func toolRiskSurfaceCount(findings []ToolRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Name + "\x00" + finding.Path
		if key == "\x00\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func toolRiskCodes(findings []ToolRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func toolRiskLineHashes(findings []ToolRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	sort.Strings(hashes)
	return hashes
}

func toolRiskMaxSeverity(findings []ToolRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if toolRiskSeverityRank(finding.Severity) > toolRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func toolRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortToolRiskFindings(findings []ToolRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if toolRiskSeverityRank(findings[i].Severity) != toolRiskSeverityRank(findings[j].Severity) {
			return toolRiskSeverityRank(findings[i].Severity) > toolRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Name != findings[j].Name {
			return findings[i].Name < findings[j].Name
		}
		if findings[i].Field != findings[j].Field {
			return findings[i].Field < findings[j].Field
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Code < findings[j].Code
	})
}
