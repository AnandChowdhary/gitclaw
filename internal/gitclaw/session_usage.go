package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionUsageReport struct {
	Scope                               string
	BackupFile                          string
	Repo                                string
	IssueNumber                         int
	EventKind                           string
	SessionUsageStatus                  string
	RawComments                         int
	TranscriptMessages                  int
	AssistantTurnComments               int
	UsageBearingAssistantTurns          int
	AssistantTurnsMissingUsageTelemetry int
	ModelBackedUsageTurns               int
	DeterministicUsageTurns             int
	ModelNames                          []string
	UsagePromptTokens                   int
	UsageCompletionTokens               int
	UsageTotalTokens                    int
	UsageCacheReadTokens                int
	UsageCacheWriteTokens               int
	LatestUsageSource                   string
	LatestUsageModel                    string
	LatestUsagePromptTokens             int
	LatestUsageCompletionTokens         int
	LatestUsageTotalTokens              int
	LatestUsageCacheReadTokens          int
	LatestUsageCacheWriteTokens         int
	RawBodiesIncluded                   bool
	RawIssueBodiesIncluded              bool
	RawCommentBodiesIncluded            bool
	RawAssistantRepliesIncluded         bool
	RawPromptsIncluded                  bool
	RawProviderUsageIncluded            bool
	RawProviderResponsesIncluded        bool
	RawToolOutputsIncluded              bool
	RawSearchQueriesIncluded            bool
	RepositoryMutationAllowed           bool
	LLME2ERequiredAfterSessionUsage     bool
	Ledger                              []SessionUsageLedgerEntry
	Turns                               []sessionPromptProvenanceTurn
}

type SessionUsageLedgerEntry struct {
	Model              string
	AssistantTurns     int
	UsageTurns         int
	ModelBackedTurns   int
	DeterministicTurns int
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	CacheReadTokens    int
	CacheWriteTokens   int
	FirstSource        string
	LastSource         string
	PromptContextSHAs  int
}

func BuildSessionUsageReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionUsageReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, _, _ := sessionStatsModelSummary(provenance.Turns)
	report := SessionUsageReport{
		Scope:                           scope,
		BackupFile:                      backupFile,
		Repo:                            ev.Repo,
		IssueNumber:                     ev.Issue.Number,
		EventKind:                       ev.Kind,
		SessionUsageStatus:              "ok",
		RawComments:                     len(comments),
		TranscriptMessages:              len(transcript),
		AssistantTurnComments:           counts.AssistantTurns,
		ModelNames:                      modelNames,
		RawBodiesIncluded:               false,
		RawIssueBodiesIncluded:          false,
		RawCommentBodiesIncluded:        false,
		RawAssistantRepliesIncluded:     false,
		RawPromptsIncluded:              false,
		RawProviderUsageIncluded:        false,
		RawProviderResponsesIncluded:    false,
		RawToolOutputsIncluded:          false,
		RawSearchQueriesIncluded:        false,
		RepositoryMutationAllowed:       false,
		LLME2ERequiredAfterSessionUsage: true,
		Turns:                           provenance.Turns,
	}
	report.Ledger = buildSessionUsageLedger(provenance.Turns)
	for _, turn := range provenance.Turns {
		if !turn.Usage.Present {
			report.AssistantTurnsMissingUsageTelemetry++
			continue
		}
		report.UsageBearingAssistantTurns++
		if strings.HasPrefix(strings.TrimSpace(turn.Model), "gitclaw/") {
			report.DeterministicUsageTurns++
		} else if strings.TrimSpace(turn.Model) != "" {
			report.ModelBackedUsageTurns++
		}
		report.UsagePromptTokens += turn.Usage.PromptTokens
		report.UsageCompletionTokens += turn.Usage.CompletionTokens
		report.UsageTotalTokens += turn.Usage.TotalTokens
		report.UsageCacheReadTokens += turn.Usage.CacheReadTokens
		report.UsageCacheWriteTokens += turn.Usage.CacheWriteTokens
		report.LatestUsageSource = turn.Source
		report.LatestUsageModel = turn.Model
		report.LatestUsagePromptTokens = turn.Usage.PromptTokens
		report.LatestUsageCompletionTokens = turn.Usage.CompletionTokens
		report.LatestUsageTotalTokens = turn.Usage.TotalTokens
		report.LatestUsageCacheReadTokens = turn.Usage.CacheReadTokens
		report.LatestUsageCacheWriteTokens = turn.Usage.CacheWriteTokens
	}
	if report.AssistantTurnComments == 0 {
		report.SessionUsageStatus = "empty"
	} else if report.UsageBearingAssistantTurns == 0 || report.ModelBackedUsageTurns == 0 || report.UsageTotalTokens == 0 {
		report.SessionUsageStatus = "warn"
	}
	return report
}

func RenderSessionUsageReport(report SessionUsageReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Usage Report\n\n")
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
	fmt.Fprintf(&b, "- session_usage_status: `%s`\n", report.SessionUsageStatus)
	fmt.Fprintf(&b, "- usage_scope: `%s`\n", "assistant-turn-marker-token-telemetry")
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(&b, "- assistant_turns_missing_usage_telemetry: `%d`\n", report.AssistantTurnsMissingUsageTelemetry)
	fmt.Fprintf(&b, "- model_backed_usage_turns: `%d`\n", report.ModelBackedUsageTurns)
	fmt.Fprintf(&b, "- deterministic_usage_turns: `%d`\n", report.DeterministicUsageTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- usage_prompt_tokens: `%d`\n", report.UsagePromptTokens)
	fmt.Fprintf(&b, "- usage_completion_tokens: `%d`\n", report.UsageCompletionTokens)
	fmt.Fprintf(&b, "- usage_total_tokens: `%d`\n", report.UsageTotalTokens)
	fmt.Fprintf(&b, "- usage_cache_read_tokens: `%d`\n", report.UsageCacheReadTokens)
	fmt.Fprintf(&b, "- usage_cache_write_tokens: `%d`\n", report.UsageCacheWriteTokens)
	fmt.Fprintf(&b, "- latest_usage_source: `%s`\n", inlineCode(report.LatestUsageSource))
	fmt.Fprintf(&b, "- latest_usage_model: `%s`\n", inlineCode(report.LatestUsageModel))
	fmt.Fprintf(&b, "- latest_usage_prompt_tokens: `%d`\n", report.LatestUsagePromptTokens)
	fmt.Fprintf(&b, "- latest_usage_completion_tokens: `%d`\n", report.LatestUsageCompletionTokens)
	fmt.Fprintf(&b, "- latest_usage_total_tokens: `%d`\n", report.LatestUsageTotalTokens)
	fmt.Fprintf(&b, "- latest_usage_cache_read_tokens: `%d`\n", report.LatestUsageCacheReadTokens)
	fmt.Fprintf(&b, "- latest_usage_cache_write_tokens: `%d`\n", report.LatestUsageCacheWriteTokens)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", report.RawAssistantRepliesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- raw_provider_usage_included: `%t`\n", report.RawProviderUsageIncluded)
	fmt.Fprintf(&b, "- raw_provider_responses_included: `%t`\n", report.RawProviderResponsesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_usage_change: `%t`\n\n", report.LLME2ERequiredAfterSessionUsage)
	b.WriteString("This OpenClaw/Hermes-inspired report audits token and cache telemetry across the current GitHub issue session through assistant-turn markers only. It reports normalized prompt, completion, total, cache-read, and cache-write token counts by model and by turn; raw issue bodies, comment bodies, assistant replies, prompts, provider responses, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Usage Ledger\n")
	writeSessionUsageLedger(&b, report.Ledger)

	b.WriteString("\n### Usage Turn Evidence\n")
	writeSessionUsageTurnEvidence(&b, report.Turns)

	b.WriteString("\n### Usage Gates\n")
	fmt.Fprintf(&b, "- usage_telemetry_gate=`%s`\n", sessionUsageTelemetryGate(report))
	fmt.Fprintf(&b, "- model_backed_usage_gate=`%s`\n", sessionUsageModelGate(report))
	fmt.Fprintf(&b, "- raw_provider_usage_gate=`%s`\n", "marker-attributes-only")
	fmt.Fprintf(&b, "- raw_provider_response_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_prompt_body_gate=`%s`\n", "hashes-and-marker-attributes-only")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func RenderSessionUsageCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionUsageReport(BuildSessionUsageReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func buildSessionUsageLedger(turns []sessionPromptProvenanceTurn) []SessionUsageLedgerEntry {
	type mutableEntry struct {
		SessionUsageLedgerEntry
		hashSeen map[string]bool
	}
	byModel := map[string]*mutableEntry{}
	var order []string
	for _, turn := range turns {
		model := strings.TrimSpace(turn.Model)
		if model == "" {
			model = "unknown"
		}
		entry := byModel[model]
		if entry == nil {
			entry = &mutableEntry{
				SessionUsageLedgerEntry: SessionUsageLedgerEntry{Model: model, FirstSource: turn.Source},
				hashSeen:                map[string]bool{},
			}
			byModel[model] = entry
			order = append(order, model)
		}
		entry.AssistantTurns++
		entry.LastSource = turn.Source
		if strings.HasPrefix(model, "gitclaw/") {
			entry.DeterministicTurns++
		} else if model != "unknown" {
			entry.ModelBackedTurns++
		}
		if turn.PromptContextSHA != "" && !entry.hashSeen[turn.PromptContextSHA] {
			entry.hashSeen[turn.PromptContextSHA] = true
			entry.PromptContextSHAs++
		}
		if !turn.Usage.Present {
			continue
		}
		entry.UsageTurns++
		entry.PromptTokens += turn.Usage.PromptTokens
		entry.CompletionTokens += turn.Usage.CompletionTokens
		entry.TotalTokens += turn.Usage.TotalTokens
		entry.CacheReadTokens += turn.Usage.CacheReadTokens
		entry.CacheWriteTokens += turn.Usage.CacheWriteTokens
	}
	sort.Strings(order)
	ledger := make([]SessionUsageLedgerEntry, 0, len(order))
	for _, model := range order {
		ledger = append(ledger, byModel[model].SessionUsageLedgerEntry)
	}
	return ledger
}

func writeSessionUsageLedger(b *strings.Builder, ledger []SessionUsageLedgerEntry) {
	if len(ledger) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, entry := range ledger {
		fmt.Fprintf(
			b,
			"- model=`%s` assistant_turns=`%d` usage_turns=`%d` model_backed_turns=`%d` deterministic_turns=`%d` prompt_tokens=`%d` completion_tokens=`%d` total_tokens=`%d` cache_read_tokens=`%d` cache_write_tokens=`%d` first_source=`%s` last_source=`%s` prompt_context_hashes=`%d`\n",
			inlineCode(entry.Model),
			entry.AssistantTurns,
			entry.UsageTurns,
			entry.ModelBackedTurns,
			entry.DeterministicTurns,
			entry.PromptTokens,
			entry.CompletionTokens,
			entry.TotalTokens,
			entry.CacheReadTokens,
			entry.CacheWriteTokens,
			entry.FirstSource,
			entry.LastSource,
			entry.PromptContextSHAs,
		)
	}
}

func writeSessionUsageTurnEvidence(b *strings.Builder, turns []sessionPromptProvenanceTurn) {
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
			"- source=`%s` model=`%s` prompt_context_sha256_12=`%s` usage_present=`%t` usage_prompt_tokens=`%d` usage_completion_tokens=`%d` usage_total_tokens=`%d` usage_cache_read_tokens=`%d` usage_cache_write_tokens=`%d`\n",
			turn.Source,
			inlineCode(turn.Model),
			promptHash,
			turn.Usage.Present,
			turn.Usage.PromptTokens,
			turn.Usage.CompletionTokens,
			turn.Usage.TotalTokens,
			turn.Usage.CacheReadTokens,
			turn.Usage.CacheWriteTokens,
		)
	}
}

func sessionUsageTelemetryGate(report SessionUsageReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.UsageBearingAssistantTurns == 0 || report.UsageTotalTokens == 0 {
		return "warn"
	}
	return "pass"
}

func sessionUsageModelGate(report SessionUsageReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.ModelBackedUsageTurns == 0 {
		return "warn"
	}
	return "pass"
}
