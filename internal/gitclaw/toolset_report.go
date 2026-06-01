package gitclaw

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const toolsetStorePath = ".gitclaw/toolsets"

type toolsetDocument struct {
	Name        string
	Description string
	Path        string
	Body        string
	Tools       []string
	Instruction string
	Mode        string
	ParseError  string
}

type ToolsetSummary struct {
	Name                  string
	DescriptionPresent    bool
	Path                  string
	Tools                 []string
	ResolvedTools         []string
	UnknownTools          []string
	DisabledTools         []string
	AllowlistBlockedTools []string
	InstructionPresent    bool
	Mode                  string
	Bytes                 int
	Lines                 int
	SHA                   string
	ParseError            string
	RiskFindings          []ToolRiskFinding
}

type ToolsetStoreReport struct {
	Status                         string
	Toolsets                       int
	ScannedToolsets                int
	ToolsetsWithInstruction        int
	ToolsetToolRefs                int
	ResolvedToolRefs               int
	UnknownToolRefs                int
	DisabledToolRefs               int
	AllowlistBlockedToolRefs       int
	ToolsetsWithRiskFindings       int
	RiskFindings                   []ToolRiskFinding
	HighRiskFindings               int
	WarningRiskFindings            int
	InfoRiskFindings               int
	RuntimeToolsetSelection        string
	RegistryVerification           string
	ToolsetActivationSupported     bool
	RepositoryMutationAllowed      bool
	ShellExecutionAllowed          bool
	NetworkToolExecutionAllowed    bool
	RawToolsetBodiesIncluded       bool
	RawToolsetInstructionsIncluded bool
	RawToolOutputsIncluded         bool
	LLME2ERequiredAfterChange      bool
	Summaries                      []ToolsetSummary
}

func RenderToolsetsCLIReport(cfg Config) string {
	return renderToolsetsReport(Event{}, cfg, false, "")
}

func RenderToolsetsRiskCLIReport(cfg Config) string {
	return renderToolsetsRiskReport(Event{}, cfg, false)
}

func RenderToolsetInfoCLIReport(cfg Config, name string) string {
	return renderToolsetInfoReport(Event{}, cfg, name, false)
}

func renderToolsetsReport(ev Event, cfg Config, includeIssue bool, explicitMode string) string {
	report := BuildToolsetStoreReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Toolsets Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolsetReportHeader(&b, ev, includeIssue)
	writeToolsetReportSummary(&b, report)
	if explicitMode != "" {
		fmt.Fprintf(&b, "- requested_toolset_mode: `%s`\n", explicitMode)
	}
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Toolsets are repo-reviewed task profiles for deterministic tool exposure. GitClaw v1 inventories and audits them, but does not dynamically activate toolsets or grant additional runtime capabilities from these files.\n\n")

	b.WriteString("### Toolsets\n")
	writeToolsetCards(&b, report.Summaries)
	return strings.TrimSpace(b.String())
}

func renderToolsetsRiskReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildToolsetStoreReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Toolsets Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolsetReportHeader(&b, ev, includeIssue)
	writeToolsetReportSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans repo-local toolset profiles for unsafe references and risky instructions without activating tools, executing shell commands, calling provider APIs, mutating repository state, or printing raw toolset bodies, instructions, tool outputs, issue bodies, comments, prompts, credentials, or secret values.\n\n")

	b.WriteString("### Toolset Risk Cards\n")
	writeToolsetCards(&b, report.Summaries)

	b.WriteString("\n### Risk Findings\n")
	writeToolsetRiskFindings(&b, report.RiskFindings)
	return strings.TrimSpace(b.String())
}

func renderToolsetInfoReport(ev Event, cfg Config, name string, includeIssue bool) string {
	report := BuildToolsetStoreReport(cfg)
	normalized := normalizeToolsetName(name)
	matches := matchingToolsetSummaries(report.Summaries, normalized)
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Toolset Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolsetReportHeader(&b, ev, includeIssue)
	fmt.Fprintf(&b, "- requested_toolset_sha256_12: `%s`\n", shortDocumentHash(name))
	fmt.Fprintf(&b, "- normalized_toolset: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- toolset_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- available_toolsets: `%d`\n", report.Toolsets)
	fmt.Fprintf(&b, "- matched_toolsets: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- runtime_toolset_selection: `%s`\n", report.RuntimeToolsetSelection)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(&b, "- toolset_activation_supported: `%t`\n", report.ToolsetActivationSupported)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(&b, "- network_tool_execution_allowed: `%t`\n", report.NetworkToolExecutionAllowed)
	fmt.Fprintf(&b, "- raw_requested_toolset_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_toolset_instructions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_toolset_info_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report focuses one repo-reviewed toolset profile. It shows normalized tool refs, config gating, hashes, and risk codes only; raw toolset instructions, issue bodies, comments, prompts, and tool outputs are not included.\n\n")

	b.WriteString("### Matches\n")
	writeToolsetCards(&b, matches)

	b.WriteString("\n### Risk Findings For Matches\n")
	var findings []ToolRiskFinding
	for _, summary := range matches {
		findings = append(findings, summary.RiskFindings...)
	}
	sortToolRiskFindings(findings)
	writeToolsetRiskFindings(&b, findings)

	b.WriteString("\n### Info Gates\n")
	fmt.Fprintf(&b, "- toolset_info_gate=`%s`\n", status)
	b.WriteString("- activation_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- shell_execution_gate=`disabled`\n")
	b.WriteString("- network_execution_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")

	if len(matches) == 0 {
		b.WriteString("\n### Available Toolsets\n")
		writeToolsetCards(&b, report.Summaries)
	}
	return strings.TrimSpace(b.String())
}

func BuildToolsetStoreReport(cfg Config) ToolsetStoreReport {
	docs := discoverToolsets(cfg.Workdir)
	summaries := summarizeToolsets(docs, cfg)
	report := ToolsetStoreReport{
		Status:                         "ok",
		Toolsets:                       len(summaries),
		ScannedToolsets:                len(summaries),
		RuntimeToolsetSelection:        "not_active_in_v1",
		RegistryVerification:           "static_builtin_contracts_only",
		ToolsetActivationSupported:     false,
		RepositoryMutationAllowed:      false,
		ShellExecutionAllowed:          false,
		NetworkToolExecutionAllowed:    false,
		RawToolsetBodiesIncluded:       false,
		RawToolsetInstructionsIncluded: false,
		RawToolOutputsIncluded:         false,
		LLME2ERequiredAfterChange:      true,
		Summaries:                      summaries,
	}
	for _, summary := range summaries {
		if summary.InstructionPresent {
			report.ToolsetsWithInstruction++
		}
		report.ToolsetToolRefs += len(summary.Tools)
		report.ResolvedToolRefs += len(summary.ResolvedTools)
		report.UnknownToolRefs += len(summary.UnknownTools)
		report.DisabledToolRefs += len(summary.DisabledTools)
		report.AllowlistBlockedToolRefs += len(summary.AllowlistBlockedTools)
		if len(summary.RiskFindings) > 0 {
			report.ToolsetsWithRiskFindings++
		}
		report.RiskFindings = append(report.RiskFindings, summary.RiskFindings...)
	}
	sortToolRiskFindings(report.RiskFindings)
	for _, finding := range report.RiskFindings {
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

func discoverToolsets(root string) []toolsetDocument {
	if root == "" {
		root = "."
	}
	var docs []toolsetDocument
	seen := map[string]bool{}
	for _, pattern := range []string{".gitclaw/toolsets/*.yml", ".gitclaw/toolsets/*.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		for _, match := range matches {
			realPath, err := filepath.EvalSymlinks(match)
			if err != nil {
				realPath = match
			}
			seenKey := strings.ToLower(realPath)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			rel, err := filepath.Rel(root, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			body, err := readRepoTextFile(root, rel, maxContextDocumentBytes)
			if err != nil {
				continue
			}
			docs = append(docs, parseToolsetDocument(rel, body))
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs
}

func parseToolsetDocument(path, body string) toolsetDocument {
	name := toolsetNameFromPath(path)
	var file struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Mode        string   `yaml:"mode"`
		Tools       []string `yaml:"tools"`
		Instruction string   `yaml:"instruction"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.KnownFields(true)
	parseError := ""
	if err := decoder.Decode(&file); err != nil {
		parseError = err.Error()
	}
	if value := normalizeToolsetName(file.Name); value != "" {
		name = value
	}
	mode := strings.ToLower(strings.TrimSpace(file.Mode))
	if mode == "" {
		mode = "read-only"
	}
	return toolsetDocument{
		Name:        name,
		Description: strings.TrimSpace(file.Description),
		Path:        path,
		Body:        body,
		Tools:       normalizeToolsetTools(file.Tools),
		Instruction: strings.TrimSpace(file.Instruction),
		Mode:        mode,
		ParseError:  parseError,
	}
}

func summarizeToolsets(docs []toolsetDocument, cfg Config) []ToolsetSummary {
	contracts := toolContractNameSet()
	summaries := make([]ToolsetSummary, 0, len(docs))
	for _, doc := range docs {
		var resolved, unknown, disabled, blocked []string
		for _, tool := range doc.Tools {
			normalized := normalizeToolLookupName(tool)
			if !contracts[normalized] {
				unknown = append(unknown, normalized)
				continue
			}
			resolved = append(resolved, normalized)
			enabled, disabledByConfig, blockedByAllowlist := toolEnabledByConfig(normalized, cfg)
			if !enabled && disabledByConfig {
				disabled = append(disabled, normalized)
			}
			if !enabled && blockedByAllowlist {
				blocked = append(blocked, normalized)
			}
		}
		findings := scanToolsetRiskFindings(doc, cfg, contracts)
		summaries = append(summaries, ToolsetSummary{
			Name:                  doc.Name,
			DescriptionPresent:    doc.Description != "",
			Path:                  doc.Path,
			Tools:                 append([]string(nil), doc.Tools...),
			ResolvedTools:         uniqueSortedStrings(resolved),
			UnknownTools:          uniqueSortedStrings(unknown),
			DisabledTools:         uniqueSortedStrings(disabled),
			AllowlistBlockedTools: uniqueSortedStrings(blocked),
			InstructionPresent:    doc.Instruction != "",
			Mode:                  doc.Mode,
			Bytes:                 len(doc.Body),
			Lines:                 lineCount(doc.Body),
			SHA:                   shortDocumentHash(doc.Body),
			ParseError:            doc.ParseError,
			RiskFindings:          findings,
		})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Path < summaries[j].Path })
	return summaries
}

func scanToolsetRiskFindings(doc toolsetDocument, cfg Config, contracts map[string]bool) []ToolRiskFinding {
	findings := scanToolRiskText("toolset", doc.Name, doc.Path, "body", doc.Body)
	add := func(severity, code, category, field string, line int, value string) {
		findings = append(findings, ToolRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Kind:     "toolset",
			Name:     doc.Name,
			Path:     doc.Path,
			Field:    field,
			Line:     line,
			LineSHA:  shortDocumentHash(value),
		})
	}
	if strings.TrimSpace(doc.ParseError) != "" {
		add("warning", "toolset_yaml_parse_error", "toolset-schema", "yaml", 0, doc.ParseError)
	}
	if len(doc.Tools) == 0 {
		add("warning", "toolset_empty_tool_refs", "toolset-schema", "tools", 0, doc.Path)
	}
	if doc.Mode != "read-only" && doc.Mode != "metadata-only" {
		add("warning", "toolset_unrecognized_mode", "toolset-schema", "mode", 0, doc.Mode)
	}
	for _, tool := range doc.Tools {
		normalized := normalizeToolLookupName(tool)
		if !contracts[normalized] {
			add("warning", "toolset_unknown_tool_ref", "tool-contract", "tools", 0, normalized)
			continue
		}
		matches := matchingToolContracts(toolReportContracts, normalized)
		if len(matches) == 1 && isMutatingToolContract(matches[0]) {
			add("high", "toolset_mutating_tool_ref", "repository-mutation", "tools", 0, normalized)
		}
		enabled, disabledByConfig, blockedByAllowlist := toolEnabledByConfig(normalized, cfg)
		if !enabled && disabledByConfig {
			add("warning", "toolset_disabled_tool_ref", "config-gating", "tools", 0, normalized)
		}
		if !enabled && blockedByAllowlist {
			add("warning", "toolset_allowlist_blocked_tool_ref", "config-gating", "tools", 0, normalized)
		}
	}
	sortToolRiskFindings(findings)
	return findings
}

func toolsetNameFromPath(path string) string {
	base := filepath.Base(filepath.FromSlash(path))
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return normalizeToolsetName(base)
}

func normalizeToolsetName(value string) string {
	return normalizeSkillBundleName(value)
}

func normalizeToolsetTools(values []string) []string {
	var out []string
	for _, value := range values {
		normalized := normalizeToolLookupName(value)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return uniqueSortedStrings(out)
}

func matchingToolsetSummaries(summaries []ToolsetSummary, name string) []ToolsetSummary {
	name = normalizeToolsetName(name)
	if name == "" {
		return nil
	}
	var matches []ToolsetSummary
	for _, summary := range summaries {
		if normalizeToolsetName(summary.Name) == name || normalizeToolsetName(filepath.Base(strings.TrimSuffix(summary.Path, filepath.Ext(summary.Path)))) == name {
			matches = append(matches, summary)
		}
	}
	return matches
}

func writeToolsetReportHeader(b *strings.Builder, ev Event, includeIssue bool) {
	if includeIssue {
		fmt.Fprintf(b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(b, "- scope: `%s`\n", "local-cli")
	}
}

func writeToolsetReportSummary(b *strings.Builder, report ToolsetStoreReport) {
	fmt.Fprintf(b, "- toolset_store_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- toolset_store_path: `%s`\n", toolsetStorePath)
	fmt.Fprintf(b, "- toolset_files: `%d`\n", report.Toolsets)
	fmt.Fprintf(b, "- scanned_toolsets: `%d`\n", report.ScannedToolsets)
	fmt.Fprintf(b, "- toolset_tool_refs: `%d`\n", report.ToolsetToolRefs)
	fmt.Fprintf(b, "- resolved_tool_refs: `%d`\n", report.ResolvedToolRefs)
	fmt.Fprintf(b, "- unknown_tool_refs: `%d`\n", report.UnknownToolRefs)
	fmt.Fprintf(b, "- disabled_tool_refs: `%d`\n", report.DisabledToolRefs)
	fmt.Fprintf(b, "- allowlist_blocked_tool_refs: `%d`\n", report.AllowlistBlockedToolRefs)
	fmt.Fprintf(b, "- toolsets_with_instruction: `%d`\n", report.ToolsetsWithInstruction)
	fmt.Fprintf(b, "- toolsets_with_risk_findings: `%d`\n", report.ToolsetsWithRiskFindings)
	fmt.Fprintf(b, "- toolset_risk_findings: `%d`\n", len(report.RiskFindings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- runtime_toolset_selection: `%s`\n", report.RuntimeToolsetSelection)
	fmt.Fprintf(b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(b, "- toolset_activation_supported: `%t`\n", report.ToolsetActivationSupported)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(b, "- network_tool_execution_allowed: `%t`\n", report.NetworkToolExecutionAllowed)
	fmt.Fprintf(b, "- raw_toolset_bodies_included: `%t`\n", report.RawToolsetBodiesIncluded)
	fmt.Fprintf(b, "- raw_toolset_instructions_included: `%t`\n", report.RawToolsetInstructionsIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_toolset_change: `%t`\n", report.LLME2ERequiredAfterChange)
}

func writeToolsetCards(b *strings.Builder, summaries []ToolsetSummary) {
	if len(summaries) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, summary := range summaries {
		fmt.Fprintf(b, "- toolset_name=`%s` path=`%s` mode=`%s` tools=`%s` resolved_tools=`%s` unknown_tools=`%s` disabled_tools=`%s` allowlist_blocked_tools=`%s` instruction=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			inlineCode(summary.Name),
			summary.Path,
			summary.Mode,
			inlineListOrNone(summary.Tools),
			inlineListOrNone(summary.ResolvedTools),
			inlineListOrNone(summary.UnknownTools),
			inlineListOrNone(summary.DisabledTools),
			inlineListOrNone(summary.AllowlistBlockedTools),
			summary.InstructionPresent,
			summary.DescriptionPresent,
			summary.Bytes,
			summary.Lines,
			summary.SHA,
			len(summary.RiskFindings),
			toolRiskMaxSeverity(summary.RiskFindings),
			inlineListOrNone(toolRiskCodes(summary.RiskFindings)),
		)
	}
}

func writeToolsetRiskFindings(b *strings.Builder, findings []ToolRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` toolset=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			inlineCode(finding.Name),
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}
