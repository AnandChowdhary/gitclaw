package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionTrajectoryReport struct {
	Scope                                 string
	BackupFile                            string
	Repo                                  string
	IssueNumber                           int
	EventKind                             string
	SessionTrajectoryStatus               string
	ExportFormat                          string
	RawComments                           int
	TranscriptMessages                    int
	UserMessages                          int
	AssistantMessages                     int
	AssistantTurnComments                 int
	TrajectoryTurns                       int
	ModelBackedAssistantTurns             int
	DeterministicAssistantTurns           int
	ModelNames                            []string
	PromptVisibleSkillNames               []string
	PromptVisibleToolNames                []string
	UniquePromptContextHashes             int
	AssistantTurnsWithPromptProvenance    int
	AssistantTurnsMissingPromptProvenance int
	RunMetadataTurns                      int
	UniqueRunIDHashes                     int
	ContextDocumentsTotal                 int
	SelectedSkillsTotal                   int
	ToolOutputsTotal                      int
	UsageBearingAssistantTurns            int
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
	RawProviderResponsesIncluded          bool
	RawToolOutputsIncluded                bool
	RawSearchQueriesIncluded              bool
	RepositoryMutationAllowed             bool
	LLME2ERequiredAfterSessionTrajectory  bool
	Turns                                 []SessionTrajectoryTurn
}

type SessionTrajectoryTurn struct {
	Index             int
	Source            string
	Model             string
	Deterministic     bool
	RunIDSHA          string
	EventIDSHA        string
	IdempotencyKeySHA string
	RunURLSHA         string
	PromptContextSHA  string
	ContextDocuments  int
	SelectedSkills    int
	ToolOutputs       int
	Skills            []string
	Tools             []string
	Usage             LLMUsage
	CommentSHA        string
	HasPromptEvidence bool
	HasRunMetadata    bool
}

func BuildSessionTrajectoryReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionTrajectoryReport {
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	turns := buildSessionTrajectoryTurns(comments)
	report := SessionTrajectoryReport{
		Scope:                                 scope,
		BackupFile:                            backupFile,
		Repo:                                  ev.Repo,
		IssueNumber:                           ev.Issue.Number,
		EventKind:                             ev.Kind,
		SessionTrajectoryStatus:               "ok",
		ExportFormat:                          "gitclaw.session-trajectory.v1",
		RawComments:                           len(comments),
		TranscriptMessages:                    len(transcript),
		UserMessages:                          countTranscriptRole(transcript, "user"),
		AssistantMessages:                     countTranscriptRole(transcript, "assistant"),
		AssistantTurnComments:                 countSessionMarkers(comments).AssistantTurns,
		TrajectoryTurns:                       len(turns),
		ModelBackedAssistantTurns:             modelBackedTurns,
		DeterministicAssistantTurns:           deterministicTurns,
		ModelNames:                            modelNames,
		PromptVisibleSkillNames:               provenance.PromptVisibleSkillNames,
		PromptVisibleToolNames:                provenance.PromptVisibleToolNames,
		UniquePromptContextHashes:             provenance.UniquePromptContextSHAs,
		AssistantTurnsWithPromptProvenance:    provenance.TurnsWithProvenance,
		AssistantTurnsMissingPromptProvenance: provenance.PromptContextHashMissing,
		RawBodiesIncluded:                     false,
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		RawAssistantRepliesIncluded:           false,
		RawPromptsIncluded:                    false,
		RawProviderResponsesIncluded:          false,
		RawToolOutputsIncluded:                false,
		RawSearchQueriesIncluded:              false,
		RepositoryMutationAllowed:             false,
		LLME2ERequiredAfterSessionTrajectory:  true,
		Turns:                                 turns,
	}
	runHashes := map[string]bool{}
	for _, turn := range turns {
		if turn.HasRunMetadata {
			report.RunMetadataTurns++
		}
		if turn.RunIDSHA != "" {
			runHashes[turn.RunIDSHA] = true
		}
		report.ContextDocumentsTotal += turn.ContextDocuments
		report.SelectedSkillsTotal += turn.SelectedSkills
		report.ToolOutputsTotal += turn.ToolOutputs
		if !turn.Usage.Present {
			continue
		}
		report.UsageBearingAssistantTurns++
		report.UsagePromptTokens += turn.Usage.PromptTokens
		report.UsageCompletionTokens += turn.Usage.CompletionTokens
		report.UsageTotalTokens += turn.Usage.TotalTokens
		report.UsageCacheReadTokens += turn.Usage.CacheReadTokens
		report.UsageCacheWriteTokens += turn.Usage.CacheWriteTokens
	}
	report.UniqueRunIDHashes = len(runHashes)
	if report.AssistantTurnComments == 0 {
		report.SessionTrajectoryStatus = "empty"
	} else if report.ModelBackedAssistantTurns == 0 || report.AssistantTurnsWithPromptProvenance == 0 || report.RunMetadataTurns == 0 {
		report.SessionTrajectoryStatus = "warn"
	}
	return report
}

func RenderSessionTrajectoryReport(report SessionTrajectoryReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Trajectory Report\n\n")
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
	fmt.Fprintf(&b, "- session_trajectory_status: `%s`\n", report.SessionTrajectoryStatus)
	fmt.Fprintf(&b, "- trajectory_scope: `%s`\n", "body-free-assistant-turn-manifest")
	fmt.Fprintf(&b, "- export_format: `%s`\n", report.ExportFormat)
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- trajectory_turns: `%d`\n", report.TrajectoryTurns)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- run_metadata_turns: `%d`\n", report.RunMetadataTurns)
	fmt.Fprintf(&b, "- unique_run_id_hashes: `%d`\n", report.UniqueRunIDHashes)
	fmt.Fprintf(&b, "- context_documents_total: `%d`\n", report.ContextDocumentsTotal)
	fmt.Fprintf(&b, "- selected_skills_total: `%d`\n", report.SelectedSkillsTotal)
	fmt.Fprintf(&b, "- tool_outputs_total: `%d`\n", report.ToolOutputsTotal)
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
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
	fmt.Fprintf(&b, "- raw_provider_responses_included: `%t`\n", report.RawProviderResponsesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_trajectory_change: `%t`\n\n", report.LLME2ERequiredAfterSessionTrajectory)
	b.WriteString("This OpenClaw trajectory-bundle and Hermes-session inspired report exports a body-free manifest of assistant turns. It records marker metadata, run/idempotency hashes, prompt-context hashes, model names, prompt-visible skills/tools, and usage counters; raw issue bodies, comment bodies, assistant replies, prompts, provider responses, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Trajectory Manifest\n")
	writeSessionTrajectoryTurns(&b, report.Turns)

	b.WriteString("\n### Trajectory Gates\n")
	fmt.Fprintf(&b, "- prompt_provenance_gate=`%s`\n", sessionTrajectoryPromptGate(report))
	fmt.Fprintf(&b, "- model_backed_gate=`%s`\n", sessionTrajectoryModelGate(report))
	fmt.Fprintf(&b, "- run_metadata_gate=`%s`\n", sessionTrajectoryRunMetadataGate(report))
	fmt.Fprintf(&b, "- usage_telemetry_gate=`%s`\n", sessionTrajectoryUsageGate(report))
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hashes-and-marker-attributes-only")
	fmt.Fprintf(&b, "- raw_provider_response_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func RenderSessionTrajectoryCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionTrajectoryReport(BuildSessionTrajectoryReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func buildSessionTrajectoryTurns(comments []Comment) []SessionTrajectoryTurn {
	var turns []SessionTrajectoryTurn
	for _, comment := range comments {
		turn, ok := parseSessionTrajectoryTurn(comment, len(turns)+1)
		if ok {
			turns = append(turns, turn)
		}
	}
	return turns
}

func parseSessionTrajectoryTurn(comment Comment, index int) (SessionTrajectoryTurn, bool) {
	match := markerPattern.FindStringSubmatch(comment.Body)
	if len(match) < 2 {
		return SessionTrajectoryTurn{}, false
	}
	attrs := match[1]
	model := strings.TrimSpace(markerAttribute(attrs, "model"))
	runID := markerAttribute(attrs, "run_id")
	eventID := markerAttribute(attrs, "event_id")
	idempotencyKey := markerAttribute(attrs, "idempotency_key")
	runURL := markerAttribute(attrs, "run_url")
	promptContextSHA := markerAttribute(attrs, "prompt_context_sha256_12")
	turn := SessionTrajectoryTurn{
		Index:             index,
		Source:            fmt.Sprintf("comment:%d", comment.ID),
		Model:             model,
		Deterministic:     strings.HasPrefix(model, "gitclaw/"),
		PromptContextSHA:  promptContextSHA,
		ContextDocuments:  markerAttributeInt(attrs, "context_documents"),
		SelectedSkills:    markerAttributeInt(attrs, "selected_skills"),
		ToolOutputs:       markerAttributeInt(attrs, "tool_outputs"),
		Skills:            markerAttributeList(attrs, "skills"),
		Tools:             markerAttributeList(attrs, "tools"),
		CommentSHA:        shortDocumentHash(comment.Body),
		HasPromptEvidence: promptContextSHA != "",
		HasRunMetadata:    runID != "" || eventID != "" || idempotencyKey != "" || runURL != "",
		Usage: LLMUsage{
			Present:          markerAttribute(attrs, "usage_total_tokens") != "",
			PromptTokens:     markerAttributeInt(attrs, "usage_prompt_tokens"),
			CompletionTokens: markerAttributeInt(attrs, "usage_completion_tokens"),
			TotalTokens:      markerAttributeInt(attrs, "usage_total_tokens"),
			CacheReadTokens:  markerAttributeInt(attrs, "usage_cache_read_tokens"),
			CacheWriteTokens: markerAttributeInt(attrs, "usage_cache_write_tokens"),
		},
	}
	sort.Strings(turn.Skills)
	sort.Strings(turn.Tools)
	if runID != "" {
		turn.RunIDSHA = shortDocumentHash(runID)
	}
	if eventID != "" {
		turn.EventIDSHA = shortDocumentHash(eventID)
	}
	if idempotencyKey != "" {
		turn.IdempotencyKeySHA = shortDocumentHash(idempotencyKey)
	}
	if runURL != "" {
		turn.RunURLSHA = shortDocumentHash(runURL)
	}
	return turn, true
}

func writeSessionTrajectoryTurns(b *strings.Builder, turns []SessionTrajectoryTurn) {
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
			"- turn=`%02d` source=`%s` model=`%s` deterministic=`%t` run_id_sha256_12=`%s` event_id_sha256_12=`%s` idempotency_key_sha256_12=`%s` run_url_sha256_12=`%s` prompt_context_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s` usage_present=`%t` usage_prompt_tokens=`%d` usage_completion_tokens=`%d` usage_total_tokens=`%d` usage_cache_read_tokens=`%d` usage_cache_write_tokens=`%d` assistant_comment_sha256_12=`%s`\n",
			turn.Index,
			turn.Source,
			inlineCode(turn.Model),
			turn.Deterministic,
			inlineCode(turn.RunIDSHA),
			inlineCode(turn.EventIDSHA),
			inlineCode(turn.IdempotencyKeySHA),
			inlineCode(turn.RunURLSHA),
			inlineCode(promptHash),
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
			turn.CommentSHA,
		)
	}
}

func sessionTrajectoryPromptGate(report SessionTrajectoryReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.AssistantTurnsWithPromptProvenance == 0 || report.AssistantTurnsMissingPromptProvenance > 0 {
		return "warn"
	}
	return "pass"
}

func sessionTrajectoryModelGate(report SessionTrajectoryReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.ModelBackedAssistantTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionTrajectoryRunMetadataGate(report SessionTrajectoryReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.RunMetadataTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionTrajectoryUsageGate(report SessionTrajectoryReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.UsageBearingAssistantTurns == 0 || report.UsageTotalTokens == 0 {
		return "warn"
	}
	return "pass"
}
