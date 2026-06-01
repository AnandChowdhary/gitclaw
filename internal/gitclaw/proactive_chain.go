package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type proactiveChainReport struct {
	Status                                  string
	Strategy                                string
	UpstreamPattern                         string
	PromptFiles                             int
	PromptFilesWithContextFrom              int
	ChainEdges                              int
	MissingContextSources                   int
	SelfReferences                          int
	CycleNodes                              int
	SkillBackedPromptFiles                  int
	PromptSkillHints                        int
	FreshIssueThreadPerNameSlot             bool
	ContextFromMetadataOnly                 bool
	RecursiveScheduleCreationAllowed        bool
	NoAgentModeSupported                    bool
	RawPromptBodiesIncluded                 bool
	RawWorkflowBodiesIncluded               bool
	RawIssueBodiesIncluded                  bool
	RawCommentBodiesIncluded                bool
	RawToolOutputsIncluded                  bool
	CredentialValuesIncluded                bool
	LLME2ERequiredAfterProactiveChainChange bool
	Cards                                   []proactiveChainPromptCard
	Edges                                   []proactiveChainEdge
	Findings                                []proactiveChainFinding
}

type proactiveChainPromptCard struct {
	Name                       string
	Path                       string
	Bytes                      int
	Lines                      int
	SHA                        string
	SkillHints                 []string
	ContextFromRefs            int
	ResolvedContextSources     []string
	MissingContextSourceHashes []string
	SelfReference              bool
	InCycle                    bool
}

type proactiveChainEdge struct {
	SourceName string
	SourcePath string
	SourceSHA  string
	TargetName string
	TargetPath string
	TargetSHA  string
}

type proactiveChainFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildProactiveChainReport(cfg Config) proactiveChainReport {
	surface := inspectProactiveSurface(cfg.Workdir)
	report := proactiveChainReport{
		Status:                                  "ok",
		Strategy:                                "github-actions-issue-thread-context-from",
		UpstreamPattern:                         "hermes-cron-context-from-openclaw-skill-backed-jobs",
		PromptFiles:                             len(surface.Prompts),
		SkillBackedPromptFiles:                  proactiveScheduleSkillBackedPrompts(surface.Prompts),
		PromptSkillHints:                        proactivePromptSkillHintCount(surface.Prompts),
		FreshIssueThreadPerNameSlot:             true,
		ContextFromMetadataOnly:                 true,
		RecursiveScheduleCreationAllowed:        false,
		NoAgentModeSupported:                    false,
		RawPromptBodiesIncluded:                 false,
		RawWorkflowBodiesIncluded:               false,
		RawIssueBodiesIncluded:                  false,
		RawCommentBodiesIncluded:                false,
		RawToolOutputsIncluded:                  false,
		CredentialValuesIncluded:                false,
		LLME2ERequiredAfterProactiveChainChange: true,
	}
	byName := map[string]proactivePrompt{}
	for _, prompt := range surface.Prompts {
		byName[prompt.Name] = prompt
	}

	cycleNames := map[string]bool{}
	var cards []proactiveChainPromptCard
	for _, prompt := range surface.Prompts {
		card := proactiveChainPromptCard{
			Name:            prompt.Name,
			Path:            prompt.Path,
			Bytes:           prompt.Bytes,
			Lines:           prompt.Lines,
			SHA:             prompt.SHA,
			SkillHints:      prompt.SkillHints,
			ContextFromRefs: len(prompt.ContextFrom),
		}
		if len(prompt.ContextFrom) > 0 {
			report.PromptFilesWithContextFrom++
		}
		for _, sourceName := range prompt.ContextFrom {
			if sourceName == prompt.Name {
				report.SelfReferences++
				card.SelfReference = true
				report.addFinding("warning", "proactive_context_self_reference", prompt.Path, "proactive job references itself as context source")
				continue
			}
			source, ok := byName[sourceName]
			if !ok {
				report.MissingContextSources++
				card.MissingContextSourceHashes = append(card.MissingContextSourceHashes, shortDocumentHash(sourceName))
				report.addFinding("warning", "proactive_context_source_missing", prompt.Path, "context source hash does not match a reviewed proactive prompt")
				continue
			}
			card.ResolvedContextSources = append(card.ResolvedContextSources, source.Name)
			report.Edges = append(report.Edges, proactiveChainEdge{
				SourceName: source.Name,
				SourcePath: source.Path,
				SourceSHA:  source.SHA,
				TargetName: prompt.Name,
				TargetPath: prompt.Path,
				TargetSHA:  prompt.SHA,
			})
		}
		cards = append(cards, card)
	}
	report.ChainEdges = len(report.Edges)
	for _, name := range proactiveChainCycleNames(report.Edges) {
		cycleNames[name] = true
	}
	for i := range cards {
		if cycleNames[cards[i].Name] {
			cards[i].InCycle = true
		}
	}
	report.CycleNodes = len(cycleNames)
	if report.CycleNodes > 0 {
		report.addFinding("warning", "proactive_context_cycle", "proactive-chain", "resolved context_from edges contain a cycle")
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Name != cards[j].Name {
			return cards[i].Name < cards[j].Name
		}
		return cards[i].Path < cards[j].Path
	})
	sort.Slice(report.Edges, func(i, j int) bool {
		if report.Edges[i].SourceName != report.Edges[j].SourceName {
			return report.Edges[i].SourceName < report.Edges[j].SourceName
		}
		return report.Edges[i].TargetName < report.Edges[j].TargetName
	})
	report.Cards = cards
	switch {
	case report.MissingContextSources > 0 || report.SelfReferences > 0 || report.CycleNodes > 0:
		report.Status = "warning"
	case report.ChainEdges == 0:
		report.Status = "no_chains"
	default:
		report.Status = "ok"
	}
	return report
}

func RenderProactiveChainCLIReport(cfg Config) string {
	return renderProactiveChainReport(Event{}, cfg, false)
}

func RenderProactiveChainReport(ev Event, cfg Config) string {
	return renderProactiveChainReport(ev, cfg, true)
}

func renderProactiveChainReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildProactiveChainReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Proactive Chain Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_proactive_command: `%s`\n", "chain")
		fmt.Fprintf(&b, "- proactive_command_status: `%s`\n", "ok")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- proactive_chain_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- chain_strategy: `%s`\n", report.Strategy)
	fmt.Fprintf(&b, "- upstream_pattern: `%s`\n", report.UpstreamPattern)
	fmt.Fprintf(&b, "- prompt_files: `%d`\n", report.PromptFiles)
	fmt.Fprintf(&b, "- prompt_files_with_context_from: `%d`\n", report.PromptFilesWithContextFrom)
	fmt.Fprintf(&b, "- chain_edges: `%d`\n", report.ChainEdges)
	fmt.Fprintf(&b, "- missing_context_sources: `%d`\n", report.MissingContextSources)
	fmt.Fprintf(&b, "- self_references: `%d`\n", report.SelfReferences)
	fmt.Fprintf(&b, "- cycle_nodes: `%d`\n", report.CycleNodes)
	fmt.Fprintf(&b, "- skill_backed_prompt_files: `%d`\n", report.SkillBackedPromptFiles)
	fmt.Fprintf(&b, "- prompt_skill_hints: `%d`\n", report.PromptSkillHints)
	fmt.Fprintf(&b, "- fresh_issue_thread_per_name_slot: `%t`\n", report.FreshIssueThreadPerNameSlot)
	fmt.Fprintf(&b, "- context_from_metadata_only: `%t`\n", report.ContextFromMetadataOnly)
	fmt.Fprintf(&b, "- recursive_schedule_creation_allowed: `%t`\n", report.RecursiveScheduleCreationAllowed)
	fmt.Fprintf(&b, "- no_agent_mode_supported: `%t`\n", report.NoAgentModeSupported)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_proactive_chain_change: `%t`\n", report.LLME2ERequiredAfterProactiveChainChange)
	if includeIssue {
		fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps reviewed proactive prompt dependency metadata. It adapts Hermes cron `context_from` chaining to GitHub issue-native proactive runs by reporting only prompt paths, hashes, skill hints, resolved job names, and missing-source hashes; prompt bodies, workflow bodies, issue/comment bodies, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Prompt Chain Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			fmt.Fprintf(&b, "- kind=`prompt-chain` name=`%s` path=`%s` bytes=`%d` lines=`%d` skill_hints=`%s` context_from_refs=`%d` resolved_context_sources=`%s` missing_context_source_hashes=`%s` self_reference=`%t` in_cycle=`%t` sha256_12=`%s` raw_prompt_body_included=`false`\n",
				card.Name,
				card.Path,
				card.Bytes,
				card.Lines,
				inlineListOrNone(card.SkillHints),
				card.ContextFromRefs,
				inlineListOrNone(card.ResolvedContextSources),
				inlineListOrNone(card.MissingContextSourceHashes),
				card.SelfReference,
				card.InCycle,
				card.SHA,
			)
		}
	}

	b.WriteString("\n### Chain Edges\n")
	if len(report.Edges) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, edge := range report.Edges {
			fmt.Fprintf(&b, "- kind=`chain-edge` from=`%s` to=`%s` source_path=`%s` target_path=`%s` source_sha256_12=`%s` target_sha256_12=`%s`\n",
				edge.SourceName,
				edge.TargetName,
				edge.SourcePath,
				edge.TargetPath,
				edge.SourceSHA,
				edge.TargetSHA,
			)
		}
	}

	b.WriteString("\n### Chain Gates\n")
	fmt.Fprintf(&b, "- context_from_gate=`%s`\n", proactiveChainContextGate(report))
	b.WriteString("- prompt_body_gate=`metadata-hashes-and-context-refs-only`\n")
	b.WriteString("- skill_hint_gate=`metadata-only`\n")
	b.WriteString("- issue_thread_gate=`one-proactive-run-issue-per-name-slot`\n")
	b.WriteString("- scheduler_gate=`github-actions-workflow-dispatch`\n")
	b.WriteString("- recursive_schedule_gate=`disabled-inside-proactive-run`\n")
	b.WriteString("- model_e2e_gate=`required`\n")

	b.WriteString("\n### Findings\n")
	writeProactiveChainFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func parseProactiveContextFrom(text string) []string {
	const marker = "gitclaw:proactive-context-from"
	var refs []string
	remaining := text
	for {
		start := strings.Index(remaining, marker)
		if start < 0 {
			break
		}
		after := remaining[start+len(marker):]
		end := strings.Index(after, "-->")
		if end < 0 {
			break
		}
		refs = append(refs, after[:end])
		remaining = after[end+len("-->"):]
	}
	return normalizeProactiveContextRefs(refs)
}

func normalizeProactiveContextRefs(refs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, raw := range refs {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\t'
		}) {
			ref := normalizeProactiveName(strings.TrimSpace(part))
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}

func proactiveChainContextGate(report proactiveChainReport) string {
	switch report.Status {
	case "ok":
		return "resolved"
	case "no_chains":
		return "no-chains"
	default:
		return "warn"
	}
}

func proactiveChainCycleNames(edges []proactiveChainEdge) []string {
	adj := map[string][]string{}
	nodes := map[string]bool{}
	for _, edge := range edges {
		adj[edge.SourceName] = append(adj[edge.SourceName], edge.TargetName)
		nodes[edge.SourceName] = true
		nodes[edge.TargetName] = true
	}
	for node := range adj {
		sort.Strings(adj[node])
	}
	state := map[string]int{}
	stackIndex := map[string]int{}
	var stack []string
	cycle := map[string]bool{}
	var visit func(string)
	visit = func(node string) {
		if state[node] == 2 {
			return
		}
		if state[node] == 1 {
			if idx, ok := stackIndex[node]; ok {
				for _, name := range stack[idx:] {
					cycle[name] = true
				}
				cycle[node] = true
			}
			return
		}
		state[node] = 1
		stackIndex[node] = len(stack)
		stack = append(stack, node)
		for _, next := range adj[node] {
			visit(next)
		}
		stack = stack[:len(stack)-1]
		delete(stackIndex, node)
		state[node] = 2
	}
	var names []string
	for node := range nodes {
		names = append(names, node)
	}
	sort.Strings(names)
	for _, name := range names {
		visit(name)
	}
	var out []string
	for name := range cycle {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func writeProactiveChainFindings(b *strings.Builder, findings []proactiveChainFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func (r *proactiveChainReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, proactiveChainFinding{Severity: severity, Code: code, Path: path, Detail: detail})
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return r.Findings[i].Severity < r.Findings[j].Severity
		}
		if r.Findings[i].Code != r.Findings[j].Code {
			return r.Findings[i].Code < r.Findings[j].Code
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}

func isProactiveChainRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || !isProactiveCommand(fields[0]) {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "chain", "chains", "dependencies", "dependency", "context-from", "context":
		return true
	default:
		return false
	}
}
