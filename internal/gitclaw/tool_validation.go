package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

var allowedToolContractModes = map[string]bool{
	"read-only":     true,
	"metadata-only": true,
}

type ToolValidationReport struct {
	Status             string
	Errors             int
	Warnings           int
	Contracts          int
	ActiveOutputs      int
	GuidanceFiles      int
	UnknownOutputs     int
	UnsafeContracts    int
	OverLimitOutputs   int
	MissingGuidance    int
	DuplicateContracts int
	Findings           []ToolValidationFinding
}

type ToolValidationFinding struct {
	Severity string
	Code     string
	Name     string
	Detail   string
}

func ValidateTools(repoContext RepoContext) ToolValidationReport {
	return ValidateToolSurface(toolReportContracts, repoContext)
}

func ValidateToolSurface(contracts []toolContract, repoContext RepoContext) ToolValidationReport {
	report := ToolValidationReport{
		Status:        "ok",
		Contracts:     len(contracts),
		ActiveOutputs: len(repoContext.ToolOutputs),
		GuidanceFiles: toolGuidanceDocumentCount(repoContext.Documents),
	}
	contractByName := map[string]toolContract{}
	contractCounts := map[string]int{}
	for _, contract := range contracts {
		name := strings.TrimSpace(contract.Name)
		contractCounts[name]++
		if name == "" {
			report.addFinding("error", "missing_tool_name", "", "tool contract name is empty")
			continue
		}
		if !strings.HasPrefix(name, "gitclaw.") {
			report.addFinding("error", "invalid_tool_name", name, "tool contract name must use the gitclaw. namespace")
		}
		if !allowedToolContractModes[contract.Mode] {
			report.UnsafeContracts++
			report.addFinding("error", "unsafe_tool_mode", name, fmt.Sprintf("tool mode %q is not allowed for GitClaw v1", contract.Mode))
		}
		contractByName[name] = contract
	}
	for name, count := range contractCounts {
		if name == "" || count < 2 {
			continue
		}
		report.DuplicateContracts++
		report.addFinding("error", "duplicate_tool_contract", name, fmt.Sprintf("tool contract is declared %d times", count))
	}
	if report.GuidanceFiles == 0 {
		report.MissingGuidance = 1
		report.addFinding("warning", "missing_tool_guidance", ".gitclaw/TOOLS.md", "tool guidance file is not loaded")
	}
	for _, output := range repoContext.ToolOutputs {
		contract, ok := contractByName[output.Name]
		if !ok {
			report.UnknownOutputs++
			report.addFinding("error", "unknown_tool_output", output.Name, "active tool output has no declared contract")
			continue
		}
		if strings.TrimSpace(output.Output) == "" {
			report.addFinding("warning", "empty_tool_output", output.Name, "declared tool produced empty output")
		}
		if detail := toolOutputLimitFinding(contract, output); detail != "" {
			report.OverLimitOutputs++
			report.addFinding("warning", "tool_output_over_limit", output.Name, detail)
		}
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity < report.Findings[j].Severity
		}
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return report.Findings[i].Name < report.Findings[j].Name
	})
	if report.Errors > 0 {
		report.Status = "error"
	} else if report.Warnings > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderToolsValidationReport(repoContext RepoContext) string {
	validation := ValidateTools(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Validate Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolsValidationSummary(&b, validation)
	b.WriteString("\n### Findings\n")
	writeToolsValidationFindings(&b, validation)
	return strings.TrimSpace(b.String())
}

func writeToolsValidationSummary(b *strings.Builder, validation ToolValidationReport) {
	fmt.Fprintf(b, "- tool_validation_status: `%s`\n", validation.Status)
	fmt.Fprintf(b, "- tool_validation_errors: `%d`\n", validation.Errors)
	fmt.Fprintf(b, "- tool_validation_warnings: `%d`\n", validation.Warnings)
	fmt.Fprintf(b, "- tool_contracts: `%d`\n", validation.Contracts)
	fmt.Fprintf(b, "- tool_active_outputs: `%d`\n", validation.ActiveOutputs)
	fmt.Fprintf(b, "- tool_guidance_files: `%d`\n", validation.GuidanceFiles)
	fmt.Fprintf(b, "- tool_unknown_outputs: `%d`\n", validation.UnknownOutputs)
	fmt.Fprintf(b, "- tool_unsafe_contracts: `%d`\n", validation.UnsafeContracts)
	fmt.Fprintf(b, "- tool_over_limit_outputs: `%d`\n", validation.OverLimitOutputs)
	fmt.Fprintf(b, "- tool_missing_guidance: `%d`\n", validation.MissingGuidance)
	fmt.Fprintf(b, "- tool_duplicate_contracts: `%d`\n", validation.DuplicateContracts)
}

func writeToolsValidationFindings(b *strings.Builder, validation ToolValidationReport) {
	if len(validation.Findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` name=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Name, inlineCode(finding.Detail))
	}
}

func toolOutputLimitFinding(contract toolContract, output ToolOutput) string {
	switch contract.Name {
	case "gitclaw.list_files":
		if lineCount(output.Output) > maxRepoFilesListed {
			return fmt.Sprintf("list_files returned %d lines, max %d", lineCount(output.Output), maxRepoFilesListed)
		}
	case "gitclaw.search_files":
		matches := searchToolMatchLineCount(output.Output)
		if matches > maxSearchMatches {
			return fmt.Sprintf("search_files returned %d matches, max %d", matches, maxSearchMatches)
		}
	case "gitclaw.read_file":
		if len(output.Output) > maxToolReadBytes {
			return fmt.Sprintf("read_file returned %d bytes, max %d", len(output.Output), maxToolReadBytes)
		}
	case "gitclaw.skill_index":
		if len(output.Output) > maxContextDocumentBytes {
			return fmt.Sprintf("skill_index returned %d bytes, max %d", len(output.Output), maxContextDocumentBytes)
		}
	case "gitclaw.policy":
		if len(output.Output) > maxToolReadBytes {
			return fmt.Sprintf("policy returned %d bytes, max %d", len(output.Output), maxToolReadBytes)
		}
	}
	return ""
}

func searchToolMatchLineCount(output string) int {
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[query ") {
			continue
		}
		count++
	}
	return count
}

func (r *ToolValidationReport) addFinding(severity, code, name, detail string) {
	r.Findings = append(r.Findings, ToolValidationFinding{
		Severity: severity,
		Code:     code,
		Name:     name,
		Detail:   detail,
	})
	switch severity {
	case "error":
		r.Errors++
	case "warning":
		r.Warnings++
	}
}
