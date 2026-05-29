package gitclaw

import (
	"fmt"
	"strings"
)

type toolContract struct {
	Name    string
	Mode    string
	Trigger string
}

var toolReportContracts = []toolContract{
	{Name: "gitclaw.list_files", Mode: "read-only", Trigger: "always"},
	{Name: "gitclaw.search_files", Mode: "read-only", Trigger: "explicit quoted phrase or identifier"},
	{Name: "gitclaw.read_file", Mode: "read-only", Trigger: "explicit repository-relative path"},
	{Name: "gitclaw.skill_index", Mode: "metadata-only", Trigger: "local skills exist"},
	{Name: "gitclaw.policy", Mode: "metadata-only", Trigger: "write intent detected"},
}

func IsToolsReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/tools"
}

func RenderToolsReport(ev Event, repoContext RepoContext) string {
	validation := ValidateTools(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	writeToolsValidationSummary(&b, validation)
	b.WriteString("\n")
	b.WriteString("GitClaw v1 tools are deterministic pre-model context builders. They do not execute shell commands, mutate files, open pull requests, or modify repository state.\n\n")
	b.WriteString("Tool output bodies are not included; hashes let maintainers verify exactly which prompt-visible outputs were produced.\n\n")

	b.WriteString("### Available Tools\n")
	for _, contract := range toolReportContracts {
		fmt.Fprintf(&b, "- `%s` mode=`%s` trigger=`%s`\n", contract.Name, contract.Mode, contract.Trigger)
	}

	b.WriteString("\n### Tool Guidance Files\n")
	writeToolGuidanceDocumentList(&b, repoContext.Documents)

	b.WriteString("\n### Active Tool Outputs\n")
	writeToolOutputList(&b, repoContext.ToolOutputs)

	b.WriteString("\n### Validation\n")
	writeToolsValidationFindings(&b, validation)

	return strings.TrimSpace(b.String())
}

func writeToolGuidanceDocumentList(b *strings.Builder, docs []ContextDocument) {
	wrote := false
	for _, doc := range docs {
		if doc.Path != ".gitclaw/TOOLS.md" {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func writeToolOutputList(b *strings.Builder, outputs []ToolOutput) {
	if len(outputs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, output := range outputs {
		fmt.Fprintf(b, "- `%s` input=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", output.Name, inlineCode(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output))
	}
}

func toolGuidanceDocumentCount(docs []ContextDocument) int {
	count := 0
	for _, doc := range docs {
		if doc.Path == ".gitclaw/TOOLS.md" {
			count++
		}
	}
	return count
}
