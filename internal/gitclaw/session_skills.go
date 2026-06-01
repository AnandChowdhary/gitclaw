package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionSkillsReport struct {
	Scope                             string
	BackupFile                        string
	Repo                              string
	IssueNumber                       int
	EventKind                         string
	SessionSkillsStatus               string
	RawComments                       int
	TranscriptMessages                int
	AssistantTurnComments             int
	SkillBackedAssistantTurns         int
	AssistantTurnsMissingSkillContext int
	UniquePromptVisibleSkills         int
	PromptVisibleSkillNames           []string
	SelectedSkillMarkers              int
	ModelBackedSkillTurns             int
	DeterministicSkillTurns           int
	ModelNames                        []string
	UsagePromptTokens                 int
	UsageCompletionTokens             int
	UsageTotalTokens                  int
	UsageCacheReadTokens              int
	UsageCacheWriteTokens             int
	RawBodiesIncluded                 bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	RawAssistantRepliesIncluded       bool
	RawPromptsIncluded                bool
	RawSkillBodiesIncluded            bool
	RawToolOutputsIncluded            bool
	RawSearchQueriesIncluded          bool
	RepositoryMutationAllowed         bool
	LLME2ERequiredAfterSessionSkills  bool
	Ledger                            []SessionSkillLedgerEntry
	Turns                             []sessionPromptProvenanceTurn
}

type SessionSkillLedgerEntry struct {
	Name               string
	PromptVisibleTurns int
	ModelBackedTurns   int
	DeterministicTurns int
	FirstSource        string
	LastSource         string
	Models             []string
	PromptContextSHAs  int
}

func BuildSessionSkillsReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionSkillsReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, _, _ := sessionStatsModelSummary(provenance.Turns)
	report := SessionSkillsReport{
		Scope:                            scope,
		BackupFile:                       backupFile,
		Repo:                             ev.Repo,
		IssueNumber:                      ev.Issue.Number,
		EventKind:                        ev.Kind,
		SessionSkillsStatus:              "ok",
		RawComments:                      len(comments),
		TranscriptMessages:               len(transcript),
		AssistantTurnComments:            counts.AssistantTurns,
		ModelNames:                       modelNames,
		PromptVisibleSkillNames:          provenance.PromptVisibleSkillNames,
		UniquePromptVisibleSkills:        len(provenance.PromptVisibleSkillNames),
		RawBodiesIncluded:                false,
		RawIssueBodiesIncluded:           false,
		RawCommentBodiesIncluded:         false,
		RawAssistantRepliesIncluded:      false,
		RawPromptsIncluded:               false,
		RawSkillBodiesIncluded:           false,
		RawToolOutputsIncluded:           false,
		RawSearchQueriesIncluded:         false,
		RepositoryMutationAllowed:        false,
		LLME2ERequiredAfterSessionSkills: true,
		Turns:                            provenance.Turns,
	}
	report.Ledger = buildSessionSkillLedger(provenance.Turns)
	for _, turn := range provenance.Turns {
		if len(turn.Skills) == 0 {
			report.AssistantTurnsMissingSkillContext++
			continue
		}
		report.SkillBackedAssistantTurns++
		report.SelectedSkillMarkers += turn.SelectedSkills
		if strings.HasPrefix(strings.TrimSpace(turn.Model), "gitclaw/") {
			report.DeterministicSkillTurns++
		} else if strings.TrimSpace(turn.Model) != "" {
			report.ModelBackedSkillTurns++
		}
	}
	report.UsagePromptTokens, report.UsageCompletionTokens, report.UsageTotalTokens, report.UsageCacheReadTokens, report.UsageCacheWriteTokens = sessionProvenanceUsageTotals(provenance.Turns)
	if report.AssistantTurnComments == 0 {
		report.SessionSkillsStatus = "empty"
	} else if report.SkillBackedAssistantTurns == 0 || report.ModelBackedSkillTurns == 0 {
		report.SessionSkillsStatus = "warn"
	}
	return report
}

func RenderSessionSkillsReport(report SessionSkillsReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Skills Report\n\n")
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
	fmt.Fprintf(&b, "- session_skills_status: `%s`\n", report.SessionSkillsStatus)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", "assistant-turn-marker-skill-context")
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- skill_backed_assistant_turns: `%d`\n", report.SkillBackedAssistantTurns)
	fmt.Fprintf(&b, "- assistant_turns_missing_skill_context: `%d`\n", report.AssistantTurnsMissingSkillContext)
	fmt.Fprintf(&b, "- unique_prompt_visible_skills: `%d`\n", report.UniquePromptVisibleSkills)
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- selected_skill_markers: `%d`\n", report.SelectedSkillMarkers)
	fmt.Fprintf(&b, "- model_backed_skill_turns: `%d`\n", report.ModelBackedSkillTurns)
	fmt.Fprintf(&b, "- deterministic_skill_turns: `%d`\n", report.DeterministicSkillTurns)
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
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_skills_change: `%t`\n\n", report.LLME2ERequiredAfterSessionSkills)
	b.WriteString("This OpenClaw/Hermes-inspired report audits skill use across the current GitHub issue session through assistant-turn markers only. It reports prompt-visible skill names, model-backed skill turns, prompt-context hashes, and token usage telemetry; raw issue bodies, comment bodies, assistant replies, prompts, skill bodies, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Skill Ledger\n")
	writeSessionSkillLedger(&b, report.Ledger)

	b.WriteString("\n### Skill Turn Evidence\n")
	writeSessionSkillTurnEvidence(&b, report.Turns)

	b.WriteString("\n### Skill Gates\n")
	fmt.Fprintf(&b, "- skill_context_gate=`%s`\n", sessionSkillsContextGate(report))
	fmt.Fprintf(&b, "- model_backed_skill_gate=`%s`\n", sessionSkillsModelGate(report))
	fmt.Fprintf(&b, "- usage_telemetry_gate=`%s`\n", sessionSkillsUsageGate(report))
	fmt.Fprintf(&b, "- raw_skill_body_gate=`%s`\n", "marker-attributes-only")
	fmt.Fprintf(&b, "- raw_tool_output_gate=`%s`\n", "marker-attributes-only")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func RenderSessionSkillsCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionSkillsReport(BuildSessionSkillsReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func buildSessionSkillLedger(turns []sessionPromptProvenanceTurn) []SessionSkillLedgerEntry {
	type mutableEntry struct {
		SessionSkillLedgerEntry
		modelSeen map[string]bool
		hashSeen  map[string]bool
	}
	byName := map[string]*mutableEntry{}
	var order []string
	for _, turn := range turns {
		if len(turn.Skills) == 0 {
			continue
		}
		model := strings.TrimSpace(turn.Model)
		for _, skill := range turn.Skills {
			if skill == "" {
				continue
			}
			entry := byName[skill]
			if entry == nil {
				entry = &mutableEntry{
					SessionSkillLedgerEntry: SessionSkillLedgerEntry{Name: skill, FirstSource: turn.Source},
					modelSeen:               map[string]bool{},
					hashSeen:                map[string]bool{},
				}
				byName[skill] = entry
				order = append(order, skill)
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
	ledger := make([]SessionSkillLedgerEntry, 0, len(order))
	for _, name := range order {
		entry := byName[name]
		sort.Strings(entry.Models)
		ledger = append(ledger, entry.SessionSkillLedgerEntry)
	}
	return ledger
}

func writeSessionSkillLedger(b *strings.Builder, ledger []SessionSkillLedgerEntry) {
	if len(ledger) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, entry := range ledger {
		fmt.Fprintf(
			b,
			"- skill=`%s` prompt_visible_turns=`%d` model_backed_turns=`%d` deterministic_turns=`%d` first_source=`%s` last_source=`%s` models=`%s` prompt_context_hashes=`%d`\n",
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

func writeSessionSkillTurnEvidence(b *strings.Builder, turns []sessionPromptProvenanceTurn) {
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
			"- source=`%s` model=`%s` prompt_context_sha256_12=`%s` selected_skills=`%d` skills=`%s` usage_present=`%t` usage_total_tokens=`%d`\n",
			turn.Source,
			inlineCode(turn.Model),
			promptHash,
			turn.SelectedSkills,
			inlineListOrNone(turn.Skills),
			turn.Usage.Present,
			turn.Usage.TotalTokens,
		)
	}
}

func sessionSkillsContextGate(report SessionSkillsReport) string {
	if report.SkillBackedAssistantTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionSkillsModelGate(report SessionSkillsReport) string {
	if report.ModelBackedSkillTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionSkillsUsageGate(report SessionSkillsReport) string {
	if report.ModelBackedSkillTurns == 0 || report.UsageTotalTokens == 0 {
		return "warn"
	}
	return "pass"
}
