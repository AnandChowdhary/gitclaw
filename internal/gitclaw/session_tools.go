package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionToolsReport struct {
	Scope                            string
	BackupFile                       string
	Repo                             string
	IssueNumber                      int
	EventKind                        string
	SessionToolsStatus               string
	RawComments                      int
	TranscriptMessages               int
	AssistantTurnComments            int
	ToolBackedAssistantTurns         int
	AssistantTurnsMissingToolContext int
	UniquePromptVisibleTools         int
	PromptVisibleToolNames           []string
	PromptVisibleToolOutputMarkers   int
	ModelBackedToolTurns             int
	DeterministicToolTurns           int
	ModelNames                       []string
	UsagePromptTokens                int
	UsageCompletionTokens            int
	UsageTotalTokens                 int
	UsageCacheReadTokens             int
	UsageCacheWriteTokens            int
	RawBodiesIncluded                bool
	RawIssueBodiesIncluded           bool
	RawCommentBodiesIncluded         bool
	RawAssistantRepliesIncluded      bool
	RawPromptsIncluded               bool
	RawToolInputsIncluded            bool
	RawToolOutputsIncluded           bool
	RawSearchQueriesIncluded         bool
	RepositoryMutationAllowed        bool
	LLME2ERequiredAfterSessionTools  bool
	Ledger                           []SessionToolLedgerEntry
	Turns                            []sessionPromptProvenanceTurn
}

type SessionToolLedgerEntry struct {
	Name               string
	PromptVisibleTurns int
	ModelBackedTurns   int
	DeterministicTurns int
	FirstSource        string
	LastSource         string
	Models             []string
	PromptContextSHAs  int
}

func BuildSessionToolsReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionToolsReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, _, _ := sessionStatsModelSummary(provenance.Turns)
	report := SessionToolsReport{
		Scope:                           scope,
		BackupFile:                      backupFile,
		Repo:                            ev.Repo,
		IssueNumber:                     ev.Issue.Number,
		EventKind:                       ev.Kind,
		SessionToolsStatus:              "ok",
		RawComments:                     len(comments),
		TranscriptMessages:              len(transcript),
		AssistantTurnComments:           counts.AssistantTurns,
		ModelNames:                      modelNames,
		PromptVisibleToolNames:          provenance.PromptVisibleToolNames,
		UniquePromptVisibleTools:        len(provenance.PromptVisibleToolNames),
		RawBodiesIncluded:               false,
		RawIssueBodiesIncluded:          false,
		RawCommentBodiesIncluded:        false,
		RawAssistantRepliesIncluded:     false,
		RawPromptsIncluded:              false,
		RawToolInputsIncluded:           false,
		RawToolOutputsIncluded:          false,
		RawSearchQueriesIncluded:        false,
		RepositoryMutationAllowed:       false,
		LLME2ERequiredAfterSessionTools: true,
		Turns:                           provenance.Turns,
	}
	report.Ledger = buildSessionToolLedger(provenance.Turns)
	for _, turn := range provenance.Turns {
		if len(turn.Tools) == 0 {
			report.AssistantTurnsMissingToolContext++
			continue
		}
		report.ToolBackedAssistantTurns++
		report.PromptVisibleToolOutputMarkers += turn.ToolOutputs
		if strings.HasPrefix(strings.TrimSpace(turn.Model), "gitclaw/") {
			report.DeterministicToolTurns++
		} else if strings.TrimSpace(turn.Model) != "" {
			report.ModelBackedToolTurns++
		}
	}
	report.UsagePromptTokens, report.UsageCompletionTokens, report.UsageTotalTokens, report.UsageCacheReadTokens, report.UsageCacheWriteTokens = sessionProvenanceUsageTotals(provenance.Turns)
	if report.AssistantTurnComments == 0 {
		report.SessionToolsStatus = "empty"
	} else if report.ToolBackedAssistantTurns == 0 || report.ModelBackedToolTurns == 0 {
		report.SessionToolsStatus = "warn"
	}
	return report
}

func RenderSessionToolsReport(report SessionToolsReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Tools Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if report.Scope == "" {
		report.Scope = "issue-thread"
	}
	fmt.Fprintf(&b, "- scope: `%s`\n", report.Scope)
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(report.BackupFile))
	}
	if report.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", report.Repo)
	}
	if report.IssueNumber != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", report.IssueNumber)
	}
	fmt.Fprintf(&b, "- event_kind: `%s`\n", report.EventKind)
	fmt.Fprintf(&b, "- session_tools_status: `%s`\n", report.SessionToolsStatus)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", "assistant-turn-marker-tool-context")
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- tool_backed_assistant_turns: `%d`\n", report.ToolBackedAssistantTurns)
	fmt.Fprintf(&b, "- assistant_turns_missing_tool_context: `%d`\n", report.AssistantTurnsMissingToolContext)
	fmt.Fprintf(&b, "- unique_prompt_visible_tools: `%d`\n", report.UniquePromptVisibleTools)
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_output_markers: `%d`\n", report.PromptVisibleToolOutputMarkers)
	fmt.Fprintf(&b, "- model_backed_tool_turns: `%d`\n", report.ModelBackedToolTurns)
	fmt.Fprintf(&b, "- deterministic_tool_turns: `%d`\n", report.DeterministicToolTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- usage_prompt_tokens: `%d`\n", report.UsagePromptTokens)
	fmt.Fprintf(&b, "- usage_completion_tokens: `%d`\n", report.UsageCompletionTokens)
	fmt.Fprintf(&b, "- usage_total_tokens: `%d`\n", report.UsageTotalTokens)
	fmt.Fprintf(&b, "- usage_cache_read_tokens: `%d`\n", report.UsageCacheReadTokens)
	fmt.Fprintf(&b, "- usage_cache_write_tokens: `%d`\n", report.UsageCacheWriteTokens)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", report.RawAssistantRepliesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", report.RawToolInputsIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_tools_change: `%t`\n\n", report.LLME2ERequiredAfterSessionTools)
	b.WriteString("This OpenClaw/Hermes-inspired report audits tool use across the current GitHub issue session through assistant-turn markers only. It reports prompt-visible tool names, model-backed tool turns, prompt-context hashes, and token usage telemetry; raw issue bodies, comment bodies, assistant replies, prompts, tool inputs, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Tool Ledger\n")
	writeSessionToolLedger(&b, report.Ledger)

	b.WriteString("\n### Tool Turn Evidence\n")
	writeSessionToolTurnEvidence(&b, report.Turns)

	b.WriteString("\n### Tool Gates\n")
	fmt.Fprintf(&b, "- tool_context_gate=`%s`\n", sessionToolsContextGate(report))
	fmt.Fprintf(&b, "- model_backed_tool_gate=`%s`\n", sessionToolsModelGate(report))
	fmt.Fprintf(&b, "- usage_telemetry_gate=`%s`\n", sessionToolsUsageGate(report))
	fmt.Fprintf(&b, "- raw_tool_input_gate=`%s`\n", "marker-attributes-only")
	fmt.Fprintf(&b, "- raw_tool_output_gate=`%s`\n", "marker-attributes-only")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func RenderSessionToolsCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionToolsReport(BuildSessionToolsReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func buildSessionToolLedger(turns []sessionPromptProvenanceTurn) []SessionToolLedgerEntry {
	type mutableEntry struct {
		SessionToolLedgerEntry
		modelSeen map[string]bool
		hashSeen  map[string]bool
	}
	byName := map[string]*mutableEntry{}
	var order []string
	for _, turn := range turns {
		if len(turn.Tools) == 0 {
			continue
		}
		model := strings.TrimSpace(turn.Model)
		for _, tool := range turn.Tools {
			if tool == "" {
				continue
			}
			entry := byName[tool]
			if entry == nil {
				entry = &mutableEntry{
					SessionToolLedgerEntry: SessionToolLedgerEntry{Name: tool, FirstSource: turn.Source},
					modelSeen:              map[string]bool{},
					hashSeen:               map[string]bool{},
				}
				byName[tool] = entry
				order = append(order, tool)
			}
			entry.PromptVisibleTurns++
			entry.LastSource = turn.Source
			if strings.HasPrefix(model, "gitclaw/") {
				entry.DeterministicTurns++
			} else if model != "" {
				entry.ModelBackedTurns++
			}
			if model != "" && !entry.modelSeen[model] {
				entry.modelSeen[model] = true
				entry.Models = append(entry.Models, model)
			}
			if turn.PromptContextSHA != "" && !entry.hashSeen[turn.PromptContextSHA] {
				entry.hashSeen[turn.PromptContextSHA] = true
				entry.PromptContextSHAs++
			}
		}
	}
	sort.Strings(order)
	ledger := make([]SessionToolLedgerEntry, 0, len(order))
	for _, name := range order {
		entry := byName[name]
		sort.Strings(entry.Models)
		ledger = append(ledger, entry.SessionToolLedgerEntry)
	}
	return ledger
}

func writeSessionToolLedger(b *strings.Builder, ledger []SessionToolLedgerEntry) {
	if len(ledger) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, entry := range ledger {
		fmt.Fprintf(
			b,
			"- tool=`%s` prompt_visible_turns=`%d` model_backed_turns=`%d` deterministic_turns=`%d` first_source=`%s` last_source=`%s` models=`%s` prompt_context_hashes=`%d`\n",
			entry.Name,
			entry.PromptVisibleTurns,
			entry.ModelBackedTurns,
			entry.DeterministicTurns,
			entry.FirstSource,
			entry.LastSource,
			inlineListOrNone(entry.Models),
			entry.PromptContextSHAs,
		)
	}
}

func writeSessionToolTurnEvidence(b *strings.Builder, turns []sessionPromptProvenanceTurn) {
	if len(turns) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, turn := range turns {
		promptHash := turn.PromptContextSHA
		if promptHash == "" {
			promptHash = "none"
		}
		fmt.Fprintf(
			b,
			"- source=`%s` model=`%s` prompt_context_sha256_12=`%s` tool_outputs=`%d` tools=`%s` usage_present=`%t` usage_total_tokens=`%d`\n",
			turn.Source,
			inlineCode(turn.Model),
			promptHash,
			turn.ToolOutputs,
			inlineListOrNone(turn.Tools),
			turn.Usage.Present,
			turn.Usage.TotalTokens,
		)
	}
}

func sessionToolsContextGate(report SessionToolsReport) string {
	if report.ToolBackedAssistantTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionToolsModelGate(report SessionToolsReport) string {
	if report.ModelBackedToolTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionToolsUsageGate(report SessionToolsReport) string {
	if report.ModelBackedToolTurns == 0 || report.UsageTotalTokens == 0 {
		return "warn"
	}
	return "pass"
}
