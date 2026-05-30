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
	if isToolsValidateRequest(ev, cfg) {
		return renderToolsValidationReport(ev, repoContext, true)
	}
	if query := requestedToolSearchQuery(ev, cfg); query != "" {
		return RenderToolSearchReport(ev, repoContext, query, defaultToolSearchMaxResults)
	}
	return renderToolsListReport(ev, repoContext, true)
}

func RenderToolsCLIReport(repoContext RepoContext) string {
	return renderToolsListReport(Event{}, repoContext, false)
}

func renderToolsListReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	validation := ValidateTools(repoContext)
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

func requestedToolSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/tools" || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanMemorySearchQuery(strings.Join(fields[2:], " "))
}

func isToolsValidateRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/tools" && strings.EqualFold(fields[1], "validate")
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
		results = append(results, ToolSearchResult{
			Kind:        "contract",
			Name:        contract.Name,
			MatchFields: fields,
			Score:       score,
			Mode:        contract.Mode,
			Trigger:     contract.Trigger,
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
		fmt.Fprintf(b, "- kind=`%s` name=`%s` match_fields=`%s` score=`%d` mode=`%s` trigger=`%s`\n",
			result.Kind,
			result.Name,
			inlineList(result.MatchFields),
			result.Score,
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
