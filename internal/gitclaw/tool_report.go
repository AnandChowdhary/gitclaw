package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type toolContract struct {
	Name    string
	Mode    string
	Trigger string
}

const defaultToolSearchMaxResults = 10

type ToolSearchReport struct {
	QueryHash         string
	QueryTerms        int
	SearchStatus      string
	MaxResults        int
	AvailableTools    int
	ActiveOutputs     int
	MatchedContracts  int
	MatchedOutputs    int
	ResultsReturned   int
	RawBodiesIncluded bool
	RawInputsIncluded bool
	Results           []ToolSearchResult
}

type ToolSearchResult struct {
	Kind        string
	Name        string
	MatchFields []string
	Score       int
	Mode        string
	Trigger     string
	Enabled     bool
	Disabled    bool
	Blocked     bool
	InputSHA    string
	OutputBytes int
	OutputLines int
	OutputSHA   string
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

func RenderToolsReport(ev Event, cfg Config, repoContext RepoContext) string {
	if isToolCatalogRequest(ev, cfg) {
		return renderToolCatalogReport(ev, cfg, repoContext, true)
	}
	if isToolSnapshotRequest(ev, cfg) {
		return renderToolSnapshotReport(ev, cfg, repoContext, true)
	}
	if isToolsVerifyRequest(ev, cfg) {
		return renderToolVerifyReport(ev, repoContext, true)
	}
	if isToolsRiskRequest(ev, cfg) {
		return renderToolRiskReport(ev, repoContext, true)
	}
	if isToolsValidateRequest(ev, cfg) {
		return renderToolsValidationReport(ev, repoContext, true)
	}
	if isToolExposureRiskRequest(ev, cfg) {
		return renderToolExposureRiskReport(ev, repoContext, true)
	}
	if isToolExposureListRequest(ev, cfg) {
		return renderToolExposureReport(ev, repoContext, true)
	}
	if isToolDeferPlanRequest(ev, cfg) {
		return renderToolDeferPlanReport(ev, cfg, repoContext, true)
	}
	if isToolBoundaryRequest(ev, cfg) {
		return renderToolBoundaryReport(ev, repoContext, true)
	}
	if isToolProvenanceRequest(ev, cfg) {
		return renderToolProvenanceReport(ev, repoContext, true)
	}
	if toolName := requestedToolApprovalPlanName(ev, cfg); toolName != "" {
		if toolName == "__missing__" {
			toolName = ""
		}
		return renderToolApprovalPlanReport(ev, cfg, repoContext, toolName, true)
	}
	if toolName := requestedToolReadinessName(ev, cfg); toolName != "" {
		if toolName == "__missing__" {
			toolName = ""
		}
		return renderToolReadinessReport(ev, repoContext, toolName, true)
	}
	if toolName := requestedToolMapName(ev, cfg); toolName != "" {
		if toolName == "__missing__" {
			toolName = ""
		}
		return renderToolMapReport(ev, repoContext, toolName, true)
	}
	if isToolsetsProvenanceRequest(ev, cfg) {
		return renderToolsetsProvenanceReport(ev, cfg, true)
	}
	if isToolsetsRiskRequest(ev, cfg) {
		return renderToolsetsRiskReport(ev, cfg, true)
	}
	if toolsetName := requestedToolsetInfoName(ev, cfg); toolsetName != "" {
		return renderToolsetInfoReport(ev, cfg, toolsetName, true)
	}
	if isToolsetsListRequest(ev, cfg) {
		return renderToolsetsReport(ev, cfg, true, "list")
	}
	if toolName := requestedToolRunPlanName(ev, cfg); toolName != "" {
		if toolName == "__missing__" {
			toolName = ""
		}
		return renderToolRunPlanReport(ev, repoContext, toolName, true)
	}
	if toolName := requestedToolInfoName(ev, cfg); toolName != "" {
		return renderToolInfoReport(ev, repoContext, toolName, true)
	}
	if query := requestedToolSearchQuery(ev, cfg); query != "" {
		return RenderToolSearchReport(ev, repoContext, query, defaultToolSearchMaxResults)
	}
	return renderToolsListReport(ev, repoContext, true)
}

func RenderToolsCLIReport(repoContext RepoContext) string {
	return renderToolsListReport(Event{}, repoContext, false)
}

func RenderToolInfoCLIReport(repoContext RepoContext, name string) string {
	return renderToolInfoReport(Event{}, repoContext, name, false)
}

func renderToolsListReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	validation := ValidateTools(repoContext)
	risk := BuildToolRiskReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", enabledToolCount(repoContext))
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", disabledToolCount(repoContext))
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", allowlistBlockedToolCount(repoContext))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- llm_e2e_required_after_tool_report_change: `%t`\n", true)
	writeToolsValidationSummary(&b, validation)
	writeToolRiskSummary(&b, risk)
	b.WriteString("\n")
	b.WriteString("GitClaw v1 tools are deterministic pre-model context builders. They do not execute shell commands, mutate files, open pull requests, or modify repository state.\n\n")
	b.WriteString("Tool output bodies are not included; hashes let maintainers verify exactly which prompt-visible outputs were produced.\n\n")

	b.WriteString("### Available Tools\n")
	for _, contract := range toolReportContracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		fmt.Fprintf(&b, "- `%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` mode=`%s` trigger=`%s`\n", contract.Name, enabled, disabled, blocked, contract.Mode, contract.Trigger)
	}

	b.WriteString("\n### Tool Guidance Files\n")
	writeToolGuidanceDocumentList(&b, repoContext.Documents)

	b.WriteString("\n### Active Tool Outputs\n")
	writeToolOutputList(&b, repoContext.ToolOutputs)

	b.WriteString("\n### Validation\n")
	writeToolsValidationFindings(&b, validation)

	return strings.TrimSpace(b.String())
}

func RenderToolSearchReport(ev Event, repoContext RepoContext, query string, maxResults int) string {
	report := BuildToolSearchReport(repoContext, query, maxResults)
	var b strings.Builder
	b.WriteString("## GitClaw Tools Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tool_search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveOutputs)
	fmt.Fprintf(&b, "- matched_contracts: `%d`\n", report.MatchedContracts)
	fmt.Fprintf(&b, "- matched_outputs: `%d`\n", report.MatchedOutputs)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n\n", report.RawInputsIncluded)
	b.WriteString("This report searches deterministic tool contracts and active tool-output metadata. It reports names, match fields, counts, sizes, and hashes only; raw tool outputs, tool inputs, issue bodies, comments, prompts, and raw search queries are not included.\n\n")

	b.WriteString("### Results\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			writeToolSearchResult(&b, result)
		}
	}
	return strings.TrimSpace(b.String())
}

func renderToolInfoReport(ev Event, repoContext RepoContext, name string, includeIssue bool) string {
	name = cleanToolLookupName(name)
	matches := matchingToolContracts(toolReportContracts, name)
	activeOutputs := matchingToolOutputs(repoContext.ToolOutputs, matches)
	validation := ValidateTools(repoContext)
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Tool Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_tool: `%s`\n", inlineCode(name))
	fmt.Fprintf(&b, "- tool_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- matched_tools: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- active_outputs_for_tool: `%d`\n", len(activeOutputs))
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n\n", false)
	b.WriteString("This report shows one deterministic tool contract plus active-output hashes for that tool. Raw tool inputs, tool output bodies, file bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Matches\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		activeCounts := toolActiveOutputCounts(repoContext.ToolOutputs)
		for _, contract := range matches {
			writeToolInfoContract(&b, contract, activeCounts[contract.Name], repoContext)
		}
	}

	b.WriteString("\n### Active Outputs For Tool\n")
	writeToolInfoActiveOutputs(&b, activeOutputs)

	b.WriteString("\n### Validation For Matches\n")
	writeToolInfoValidationFindings(&b, validation, matches)

	if len(matches) == 0 {
		b.WriteString("\n### Available Tools\n")
		for _, contract := range toolReportContracts {
			fmt.Fprintf(&b, "- `%s` mode=`%s`\n", contract.Name, contract.Mode)
		}
	}
	return strings.TrimSpace(b.String())
}

func requestedToolInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/tools" || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanToolLookupName(fields[2])
}

func requestedToolSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/tools" || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanMemorySearchQuery(strings.Join(fields[2:], " "))
}

func isToolCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/tools" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "catalog", "index", "tool-catalog", "discovery", "eligible":
		return true
	default:
		return false
	}
}

func isToolsValidateRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" && strings.EqualFold(fields[1], "validate")
}

func isToolsVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" && strings.EqualFold(fields[1], "verify")
}

func isToolsRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}

func isToolsetsListRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "toolsets") || strings.EqualFold(fields[1], "toolset")) &&
		(len(fields) == 2 || strings.EqualFold(fields[2], "list"))
}

func isToolsetsRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "toolsets") || strings.EqualFold(fields[1], "toolset")) &&
		(strings.EqualFold(fields[2], "risk") || strings.EqualFold(fields[2], "risk-audit"))
}

func isToolsetsProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 && fields[0] == "/tools" &&
		(strings.EqualFold(fields[1], "toolsets") || strings.EqualFold(fields[1], "toolset")) &&
		(strings.EqualFold(fields[2], "provenance") ||
			strings.EqualFold(fields[2], "history") ||
			strings.EqualFold(fields[2], "timeline"))
}

func requestedToolsetInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 4 || fields[0] != "/tools" {
		return ""
	}
	if !strings.EqualFold(fields[1], "toolsets") && !strings.EqualFold(fields[1], "toolset") {
		return ""
	}
	if !strings.EqualFold(fields[2], "info") && !strings.EqualFold(fields[2], "show") {
		return ""
	}
	return normalizeToolsetName(fields[3])
}

func cleanToolLookupName(name string) string {
	return strings.Trim(strings.TrimSpace(name), " \t\r\n.,:;!?`\"'")
}

func normalizeToolLookupName(name string) string {
	name = strings.ToLower(cleanToolLookupName(name))
	if name == "" {
		return ""
	}
	if !strings.Contains(name, ".") {
		name = "gitclaw." + name
	}
	return name
}

func matchingToolContracts(contracts []toolContract, name string) []toolContract {
	name = normalizeToolLookupName(name)
	if name == "" {
		return nil
	}
	matches := make([]toolContract, 0, 1)
	for _, contract := range contracts {
		contractName := strings.ToLower(contract.Name)
		if contractName == name {
			matches = append(matches, contract)
		}
	}
	return matches
}

func matchingToolOutputs(outputs []ToolOutput, contracts []toolContract) []ToolOutput {
	if len(outputs) == 0 || len(contracts) == 0 {
		return nil
	}
	names := map[string]bool{}
	for _, contract := range contracts {
		names[contract.Name] = true
	}
	matches := make([]ToolOutput, 0, len(outputs))
	for _, output := range outputs {
		if names[output.Name] {
			matches = append(matches, output)
		}
	}
	return matches
}

func BuildToolSearchReport(repoContext RepoContext, query string, maxResults int) ToolSearchReport {
	query = cleanMemorySearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultToolSearchMaxResults
	}
	report := ToolSearchReport{
		QueryHash:         shortDocumentHash(query),
		QueryTerms:        len(memorySearchTerms(query)),
		SearchStatus:      "ok",
		MaxResults:        maxResults,
		AvailableTools:    len(toolReportContracts),
		ActiveOutputs:     len(repoContext.ToolOutputs),
		RawBodiesIncluded: false,
		RawInputsIncluded: false,
	}
	if query == "" {
		report.SearchStatus = "no_query"
		return report
	}
	terms := memorySearchTerms(query)
	if len(terms) == 0 {
		report.SearchStatus = "no_query"
		return report
	}

	activeOutputCounts := map[string]int{}
	for _, output := range repoContext.ToolOutputs {
		activeOutputCounts[output.Name]++
	}

	var results []ToolSearchResult
	for _, contract := range toolReportContracts {
		score, fields := toolContractSearchScore(contract, query, terms)
		if score == 0 {
			continue
		}
		report.MatchedContracts++
		score += activeOutputCounts[contract.Name] * 2
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		results = append(results, ToolSearchResult{
			Kind:        "contract",
			Name:        contract.Name,
			MatchFields: fields,
			Score:       score,
			Mode:        contract.Mode,
			Trigger:     contract.Trigger,
			Enabled:     enabled,
			Disabled:    disabled,
			Blocked:     blocked,
		})
	}
	for _, output := range repoContext.ToolOutputs {
		score, fields := toolOutputSearchScore(output, query, terms)
		if score == 0 {
			continue
		}
		report.MatchedOutputs++
		results = append(results, ToolSearchResult{
			Kind:        "active-output",
			Name:        output.Name,
			MatchFields: fields,
			Score:       score,
			InputSHA:    shortDocumentHash(output.Input),
			OutputBytes: len(output.Output),
			OutputLines: lineCount(output.Output),
			OutputSHA:   shortDocumentHash(output.Output),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Kind != results[j].Kind {
			return results[i].Kind < results[j].Kind
		}
		return results[i].Name < results[j].Name
	})
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	report.Results = results
	report.ResultsReturned = len(results)
	if report.MatchedContracts == 0 && report.MatchedOutputs == 0 {
		report.SearchStatus = "no_matches"
	}
	return report
}

func toolActiveOutputCounts(outputs []ToolOutput) map[string]int {
	counts := map[string]int{}
	for _, output := range outputs {
		counts[output.Name]++
	}
	return counts
}

func toolContractSearchScore(contract toolContract, query string, terms []string) (int, []string) {
	fields := map[string]string{
		"name":    contract.Name,
		"mode":    contract.Mode,
		"trigger": contract.Trigger,
	}
	weights := map[string]int{
		"name":    80,
		"mode":    30,
		"trigger": 20,
	}
	return scoreSearchFields(fields, weights, query, terms)
}

func toolOutputSearchScore(output ToolOutput, query string, terms []string) (int, []string) {
	fields := map[string]string{
		"name":  output.Name,
		"input": output.Input,
	}
	weights := map[string]int{
		"name":  70,
		"input": 25,
	}
	return scoreSearchFields(fields, weights, query, terms)
}

func scoreSearchFields(fields map[string]string, weights map[string]int, query string, terms []string) (int, []string) {
	query = strings.ToLower(cleanMemorySearchQuery(query))
	if query == "" {
		return 0, nil
	}
	score := 0
	matchedFields := map[string]bool{}
	for field, value := range fields {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if value == query {
			score += weights[field] * 2
			matchedFields[field] = true
			continue
		}
		if strings.Contains(value, query) {
			score += weights[field]
			matchedFields[field] = true
		}
		for _, term := range terms {
			if strings.Contains(value, term) {
				score += weights[field] / 2
				matchedFields[field] = true
			}
		}
	}
	if score == 0 {
		return 0, nil
	}
	var out []string
	for field := range matchedFields {
		out = append(out, field)
	}
	sort.Strings(out)
	return score, out
}

func writeToolSearchResult(b *strings.Builder, result ToolSearchResult) {
	switch result.Kind {
	case "contract":
		fmt.Fprintf(b, "- kind=`%s` name=`%s` match_fields=`%s` score=`%d` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` mode=`%s` trigger=`%s`\n",
			result.Kind,
			result.Name,
			inlineList(result.MatchFields),
			result.Score,
			result.Enabled,
			result.Disabled,
			result.Blocked,
			result.Mode,
			inlineCode(result.Trigger),
		)
	default:
		fmt.Fprintf(b, "- kind=`%s` name=`%s` match_fields=`%s` score=`%d` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s`\n",
			result.Kind,
			result.Name,
			inlineList(result.MatchFields),
			result.Score,
			result.InputSHA,
			result.OutputBytes,
			result.OutputLines,
			result.OutputSHA,
		)
	}
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

func writeToolInfoContract(b *strings.Builder, contract toolContract, activeOutputs int, repoContext RepoContext) {
	enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
	fmt.Fprintf(b, "- name=`%s` source=`builtin-gitclaw` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` mode=`%s` mutating=`%t` trigger=`%s` active_outputs=`%d`\n",
		contract.Name,
		enabled,
		disabled,
		blocked,
		contract.Mode,
		isMutatingToolContract(contract),
		inlineCode(contract.Trigger),
		activeOutputs,
	)
}

func writeToolInfoActiveOutputs(b *strings.Builder, outputs []ToolOutput) {
	if len(outputs) == 0 {
		b.WriteString("- none\n")
		return
	}
	contracts := toolContractNameSet()
	for _, output := range outputs {
		fmt.Fprintf(b, "- name=`%s` contract_known=`%t` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s`\n",
			output.Name,
			contracts[output.Name],
			shortDocumentHash(output.Input),
			len(output.Output),
			lineCount(output.Output),
			shortDocumentHash(output.Output),
		)
	}
}

func toolEnabledByConfig(name string, cfg Config) (enabled bool, disabledByConfig bool, blockedByAllowlist bool) {
	return toolEnabledByMaps(name, cfg.AllowedTools, cfg.DisabledTools)
}

func toolEnabledInRepoContext(name string, repoContext RepoContext) (enabled bool, disabledByConfig bool, blockedByAllowlist bool) {
	return toolEnabledByMaps(name, repoContext.AllowedTools, repoContext.DisabledTools)
}

func toolEnabledByMaps(name string, allowed, disabled map[string]bool) (enabled bool, disabledByConfig bool, blockedByAllowlist bool) {
	name = normalizeToolLookupName(name)
	if name == "" {
		return true, false, false
	}
	if disabled[name] {
		return false, true, false
	}
	if len(allowed) > 0 && !allowed[name] {
		return false, false, true
	}
	return true, false, false
}

func enabledToolCount(repoContext RepoContext) int {
	count := 0
	for _, contract := range toolReportContracts {
		enabled, _, _ := toolEnabledInRepoContext(contract.Name, repoContext)
		if enabled {
			count++
		}
	}
	return count
}

func disabledToolCount(repoContext RepoContext) int {
	count := 0
	for _, contract := range toolReportContracts {
		_, disabled, _ := toolEnabledInRepoContext(contract.Name, repoContext)
		if disabled {
			count++
		}
	}
	return count
}

func allowlistBlockedToolCount(repoContext RepoContext) int {
	count := 0
	for _, contract := range toolReportContracts {
		_, _, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		if blocked {
			count++
		}
	}
	return count
}

func writeToolInfoValidationFindings(b *strings.Builder, validation ToolValidationReport, matches []toolContract) {
	if len(matches) == 0 {
		b.WriteString("- none\n")
		return
	}
	names := map[string]bool{}
	for _, contract := range matches {
		names[contract.Name] = true
	}
	wrote := false
	for _, finding := range validation.Findings {
		if !names[finding.Name] {
			continue
		}
		fmt.Fprintf(b, "- severity=`%s` code=`%s` name=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Name, inlineCode(finding.Detail))
		wrote = true
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
