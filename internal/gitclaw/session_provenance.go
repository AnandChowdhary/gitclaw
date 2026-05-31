package gitclaw

import (
	"fmt"
	"strconv"
	"strings"
)

type sessionPromptProvenanceReport struct {
	Turns                    []sessionPromptProvenanceTurn
	TurnsWithProvenance      int
	UniquePromptContextSHAs  int
	PromptVisibleSkillNames  []string
	PromptVisibleToolNames   []string
	PromptContextHashMissing int
}

type sessionPromptProvenanceTurn struct {
	Source            string
	Model             string
	PromptContextSHA  string
	ContextDocuments  int
	SelectedSkills    int
	ToolOutputs       int
	Skills            []string
	Tools             []string
	Usage             LLMUsage
	HasPromptEvidence bool
}

type SessionProvenanceReport struct {
	Scope                                 string
	BackupFile                            string
	Repo                                  string
	IssueNumber                           int
	EventKind                             string
	SessionProvenanceStatus               string
	RawComments                           int
	TranscriptMessages                    int
	AssistantTurnComments                 int
	AssistantTurnsWithPromptProvenance    int
	AssistantTurnsMissingPromptProvenance int
	UniquePromptContextHashes             int
	ModelBackedAssistantTurns             int
	DeterministicAssistantTurns           int
	ModelNames                            []string
	PromptVisibleSkillNames               []string
	PromptVisibleToolNames                []string
	PromptVisibleSkillCount               int
	PromptVisibleToolCount                int
	UsagePromptTokens                     int
	UsageCompletionTokens                 int
	UsageTotalTokens                      int
	UsageCacheReadTokens                  int
	UsageCacheWriteTokens                 int
	RawBodiesIncluded                     bool
	RawIssueBodiesIncluded                bool
	RawCommentBodiesIncluded              bool
	RawAssistantRepliesIncluded           bool
	RawPromptsIncluded                    bool
	RawToolOutputsIncluded                bool
	RawSearchQueriesIncluded              bool
	RepositoryMutationAllowed             bool
	LLME2ERequiredAfterSessionProvenance  bool
	Turns                                 []sessionPromptProvenanceTurn
}

func BuildSessionProvenanceReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionProvenanceReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	report := SessionProvenanceReport{
		Scope:                                 scope,
		BackupFile:                            backupFile,
		Repo:                                  ev.Repo,
		IssueNumber:                           ev.Issue.Number,
		EventKind:                             ev.Kind,
		SessionProvenanceStatus:               "ok",
		RawComments:                           len(comments),
		TranscriptMessages:                    len(transcript),
		AssistantTurnComments:                 counts.AssistantTurns,
		AssistantTurnsWithPromptProvenance:    provenance.TurnsWithProvenance,
		AssistantTurnsMissingPromptProvenance: provenance.PromptContextHashMissing,
		UniquePromptContextHashes:             provenance.UniquePromptContextSHAs,
		ModelBackedAssistantTurns:             modelBackedTurns,
		DeterministicAssistantTurns:           deterministicTurns,
		ModelNames:                            modelNames,
		PromptVisibleSkillNames:               provenance.PromptVisibleSkillNames,
		PromptVisibleToolNames:                provenance.PromptVisibleToolNames,
		PromptVisibleSkillCount:               len(provenance.PromptVisibleSkillNames),
		PromptVisibleToolCount:                len(provenance.PromptVisibleToolNames),
		RawBodiesIncluded:                     false,
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		RawAssistantRepliesIncluded:           false,
		RawPromptsIncluded:                    false,
		RawToolOutputsIncluded:                false,
		RawSearchQueriesIncluded:              false,
		RepositoryMutationAllowed:             false,
		LLME2ERequiredAfterSessionProvenance:  true,
		Turns:                                 provenance.Turns,
	}
	report.UsagePromptTokens, report.UsageCompletionTokens, report.UsageTotalTokens, report.UsageCacheReadTokens, report.UsageCacheWriteTokens = sessionProvenanceUsageTotals(provenance.Turns)
	if report.AssistantTurnComments == 0 {
		report.SessionProvenanceStatus = "empty"
	} else if report.AssistantTurnsMissingPromptProvenance > 0 || report.ModelBackedAssistantTurns == 0 {
		report.SessionProvenanceStatus = "warn"
	}
	return report
}

func RenderSessionProvenanceReport(report SessionProvenanceReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Provenance Report\n\n")
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
	fmt.Fprintf(&b, "- session_provenance_status: `%s`\n", report.SessionProvenanceStatus)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", "assistant-turn-marker-prompt-context")
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_count: `%d`\n", report.PromptVisibleSkillCount)
	fmt.Fprintf(&b, "- prompt_visible_tool_count: `%d`\n", report.PromptVisibleToolCount)
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
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
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_provenance_change: `%t`\n\n", report.LLME2ERequiredAfterSessionProvenance)
	b.WriteString("This OpenClaw/Hermes-inspired provenance report audits the current GitHub issue session through assistant-turn markers only. It reports model names, prompt-context hashes, prompt-visible skill/tool names, tool-output counts, and token usage telemetry; raw issue bodies, comment bodies, assistant replies, prompts, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Assistant Turn Provenance\n")
	writeSessionPromptProvenanceUsageList(&b, report.Turns)

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- prompt_provenance_gate=`%s`\n", sessionProvenancePromptGate(report))
	fmt.Fprintf(&b, "- model_backed_gate=`%s`\n", sessionProvenanceModelGate(report))
	fmt.Fprintf(&b, "- skill_tool_gate=`%s`\n", sessionProvenanceSkillToolGate(report))
	fmt.Fprintf(&b, "- usage_telemetry_gate=`%s`\n", sessionProvenanceUsageGate(report))
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hashes-and-marker-attributes-only")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func RenderSessionProvenanceCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionProvenanceReport(BuildSessionProvenanceReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func buildSessionPromptProvenanceReport(comments []Comment) sessionPromptProvenanceReport {
	report := sessionPromptProvenanceReport{}
	promptHashes := map[string]bool{}
	skillSeen := map[string]bool{}
	toolSeen := map[string]bool{}
	for _, comment := range comments {
		turn, ok := parseSessionPromptProvenanceTurn(comment)
		if !ok {
			continue
		}
		report.Turns = append(report.Turns, turn)
		if !turn.HasPromptEvidence {
			report.PromptContextHashMissing++
			continue
		}
		report.TurnsWithProvenance++
		if turn.PromptContextSHA != "" {
			promptHashes[turn.PromptContextSHA] = true
		}
		for _, skill := range turn.Skills {
			if skill == "" || skillSeen[skill] {
				continue
			}
			skillSeen[skill] = true
			report.PromptVisibleSkillNames = append(report.PromptVisibleSkillNames, skill)
		}
		for _, tool := range turn.Tools {
			if tool == "" || toolSeen[tool] {
				continue
			}
			toolSeen[tool] = true
			report.PromptVisibleToolNames = append(report.PromptVisibleToolNames, tool)
		}
	}
	report.UniquePromptContextSHAs = len(promptHashes)
	return report
}

func writeSessionPromptProvenanceUsageList(b *strings.Builder, turns []sessionPromptProvenanceTurn) {
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
			"- source=`%s` model=`%s` prompt_context_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s` usage_present=`%t` usage_prompt_tokens=`%d` usage_completion_tokens=`%d` usage_total_tokens=`%d` usage_cache_read_tokens=`%d` usage_cache_write_tokens=`%d`\n",
			turn.Source,
			inlineCode(turn.Model),
			promptHash,
			turn.ContextDocuments,
			turn.SelectedSkills,
			turn.ToolOutputs,
			inlineListOrNone(turn.Skills),
			inlineListOrNone(turn.Tools),
			turn.Usage.Present,
			turn.Usage.PromptTokens,
			turn.Usage.CompletionTokens,
			turn.Usage.TotalTokens,
			turn.Usage.CacheReadTokens,
			turn.Usage.CacheWriteTokens,
		)
	}
}

func sessionProvenanceUsageTotals(turns []sessionPromptProvenanceTurn) (prompt, completion, total, cacheRead, cacheWrite int) {
	for _, turn := range turns {
		if !turn.Usage.Present {
			continue
		}
		prompt += turn.Usage.PromptTokens
		completion += turn.Usage.CompletionTokens
		total += turn.Usage.TotalTokens
		cacheRead += turn.Usage.CacheReadTokens
		cacheWrite += turn.Usage.CacheWriteTokens
	}
	return prompt, completion, total, cacheRead, cacheWrite
}

func sessionProvenancePromptGate(report SessionProvenanceReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.AssistantTurnsMissingPromptProvenance > 0 || report.AssistantTurnsWithPromptProvenance == 0 {
		return "warn"
	}
	return "pass"
}

func sessionProvenanceModelGate(report SessionProvenanceReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.ModelBackedAssistantTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionProvenanceSkillToolGate(report SessionProvenanceReport) string {
	if report.PromptVisibleSkillCount == 0 || report.PromptVisibleToolCount == 0 {
		return "warn"
	}
	return "pass"
}

func sessionProvenanceUsageGate(report SessionProvenanceReport) string {
	if report.ModelBackedAssistantTurns == 0 {
		return "warn"
	}
	if report.UsageTotalTokens == 0 {
		return "warn"
	}
	return "pass"
}

func parseSessionPromptProvenanceTurn(comment Comment) (sessionPromptProvenanceTurn, bool) {
	match := markerPattern.FindStringSubmatch(comment.Body)
	if len(match) < 2 {
		return sessionPromptProvenanceTurn{}, false
	}
	attrs := match[1]
	turn := sessionPromptProvenanceTurn{
		Source:           fmt.Sprintf("comment:%d", comment.ID),
		Model:            markerAttribute(attrs, "model"),
		PromptContextSHA: markerAttribute(attrs, "prompt_context_sha256_12"),
		ContextDocuments: markerAttributeInt(attrs, "context_documents"),
		SelectedSkills:   markerAttributeInt(attrs, "selected_skills"),
		ToolOutputs:      markerAttributeInt(attrs, "tool_outputs"),
		Skills:           markerAttributeList(attrs, "skills"),
		Tools:            markerAttributeList(attrs, "tools"),
		Usage: LLMUsage{
			Present:          markerAttribute(attrs, "usage_total_tokens") != "",
			PromptTokens:     markerAttributeInt(attrs, "usage_prompt_tokens"),
			CompletionTokens: markerAttributeInt(attrs, "usage_completion_tokens"),
			TotalTokens:      markerAttributeInt(attrs, "usage_total_tokens"),
			CacheReadTokens:  markerAttributeInt(attrs, "usage_cache_read_tokens"),
			CacheWriteTokens: markerAttributeInt(attrs, "usage_cache_write_tokens"),
		},
	}
	turn.HasPromptEvidence = turn.PromptContextSHA != ""
	return turn, true
}

func markerAttributeInt(attrs, key string) int {
	raw := strings.TrimSpace(markerAttribute(attrs, key))
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func markerAttributeList(attrs, key string) []string {
	raw := markerAttribute(attrs, key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}
