package gitclaw

import (
	"fmt"
	"strings"
)

type SessionCompactionReport struct {
	Scope                                 string
	BackupFile                            string
	Repo                                  string
	IssueNumber                           int
	EventKind                             string
	SessionCompactionStatus               string
	CompactionStrategy                    string
	CompressionModel                      string
	RawComments                           int
	TranscriptMessages                    int
	UserMessages                          int
	AssistantMessages                     int
	TrustedMessages                       int
	UntrustedMessages                     int
	EditedMessages                        int
	TranscriptBodyBytes                   int
	TranscriptBodyLines                   int
	EstimatedTranscriptTokens             int
	MaxPromptBytes                        int
	MaxTranscriptMessages                 int
	MaxTranscriptMessageBytes             int
	BoundedTranscriptMessages             int
	OmittedOlderMessages                  int
	TruncatedTranscriptBodies             int
	AssistantTurnComments                 int
	AssistantTurnsWithPromptProvenance    int
	AssistantTurnsMissingPromptProvenance int
	UniquePromptContextHashes             int
	ModelBackedAssistantTurns             int
	DeterministicAssistantTurns           int
	ModelNames                            []string
	PromptVisibleSkillNames               []string
	PromptVisibleToolNames                []string
	UsageBearingAssistantTurns            int
	UsagePromptTokens                     int
	UsageCompletionTokens                 int
	UsageTotalTokens                      int
	UsageCacheReadTokens                  int
	UsageCacheWriteTokens                 int
	AgentCompressionThresholdPercent      int
	AgentCompressionThresholdBytes        int
	GatewayHygieneThresholdPercent        int
	GatewayHygieneThresholdBytes          int
	AgentCompactionRecommended            bool
	GatewayHygieneRecommended             bool
	CompressionEngineConfigured           bool
	LossySummarySupported                 bool
	LosslessSessionSearchSupported        bool
	InLoopContextCompressionSupported     bool
	PreAgentGatewayHygieneSupported       bool
	SessionSplitSupported                 bool
	ExternalSessionDBRequired             bool
	IssueThreadCanonicalStorage           bool
	BackupBranchReplayPreferred           bool
	RawBodiesIncluded                     bool
	RawIssueBodiesIncluded                bool
	RawCommentBodiesIncluded              bool
	RawAssistantRepliesIncluded           bool
	RawPromptsIncluded                    bool
	RawProviderUsageIncluded              bool
	RawProviderResponsesIncluded          bool
	RawToolOutputsIncluded                bool
	RawSearchQueriesIncluded              bool
	RepositoryMutationAllowed             bool
	CompactionMutationAllowed             bool
	CompressionWritesMemoryAllowed        bool
	LLME2ERequiredAfterSessionCompaction  bool
	Cards                                 []SessionCompactionCard
}

type SessionCompactionCard struct {
	MessageIndex        int
	Role                string
	Source              string
	Actor               string
	AuthorAssociation   string
	Trusted             bool
	Edited              bool
	Bytes               int
	Lines               int
	EstimatedTokens     int
	SHA                 string
	InCurrentPromptPack bool
	OmittedByLimit      bool
	TruncatedByLimit    bool
	CompactionRegion    string
	CompactionAction    string
	BodyIncluded        bool
}

func BuildSessionCompactionReport(scope, backupFile string, ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) SessionCompactionReport {
	budget := promptBudgetConfig(cfg)
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	bounded, omitted := boundedTranscript(transcript, budget.MaxTranscriptMessages)
	report := SessionCompactionReport{
		Scope:                                 scope,
		BackupFile:                            backupFile,
		Repo:                                  ev.Repo,
		IssueNumber:                           ev.Issue.Number,
		EventKind:                             ev.Kind,
		SessionCompactionStatus:               "ok",
		CompactionStrategy:                    "github-issue-thread-body-free-compaction-readiness",
		CompressionModel:                      "hermes-dual-thresholds+openclaw-trajectory-pruning",
		RawComments:                           len(comments),
		TranscriptMessages:                    len(transcript),
		UserMessages:                          countTranscriptRole(transcript, "user"),
		AssistantMessages:                     countTranscriptRole(transcript, "assistant"),
		TrustedMessages:                       countTrustedTranscriptMessages(transcript, true),
		UntrustedMessages:                     countTrustedTranscriptMessages(transcript, false),
		EditedMessages:                        countEditedTranscriptMessages(transcript),
		TranscriptBodyBytes:                   sessionStatsTranscriptBytes(transcript),
		TranscriptBodyLines:                   sessionStatsTranscriptLines(transcript),
		MaxPromptBytes:                        budget.MaxPromptBytes,
		MaxTranscriptMessages:                 budget.MaxTranscriptMessages,
		MaxTranscriptMessageBytes:             budget.MaxTranscriptMessageBytes,
		BoundedTranscriptMessages:             len(bounded),
		OmittedOlderMessages:                  omitted,
		AssistantTurnComments:                 counts.AssistantTurns,
		AssistantTurnsWithPromptProvenance:    provenance.TurnsWithProvenance,
		AssistantTurnsMissingPromptProvenance: provenance.PromptContextHashMissing,
		UniquePromptContextHashes:             provenance.UniquePromptContextSHAs,
		ModelBackedAssistantTurns:             modelBackedTurns,
		DeterministicAssistantTurns:           deterministicTurns,
		ModelNames:                            modelNames,
		PromptVisibleSkillNames:               provenance.PromptVisibleSkillNames,
		PromptVisibleToolNames:                provenance.PromptVisibleToolNames,
		AgentCompressionThresholdPercent:      promptPackAgentCompressionPct,
		AgentCompressionThresholdBytes:        budget.MaxPromptBytes * promptPackAgentCompressionPct / 100,
		GatewayHygieneThresholdPercent:        promptPackGatewayHygienePct,
		GatewayHygieneThresholdBytes:          budget.MaxPromptBytes * promptPackGatewayHygienePct / 100,
		CompressionEngineConfigured:           false,
		LossySummarySupported:                 false,
		LosslessSessionSearchSupported:        true,
		InLoopContextCompressionSupported:     false,
		PreAgentGatewayHygieneSupported:       false,
		SessionSplitSupported:                 false,
		ExternalSessionDBRequired:             false,
		IssueThreadCanonicalStorage:           true,
		BackupBranchReplayPreferred:           true,
		RawBodiesIncluded:                     false,
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		RawAssistantRepliesIncluded:           false,
		RawPromptsIncluded:                    false,
		RawProviderUsageIncluded:              false,
		RawProviderResponsesIncluded:          false,
		RawToolOutputsIncluded:                false,
		RawSearchQueriesIncluded:              false,
		RepositoryMutationAllowed:             false,
		CompactionMutationAllowed:             false,
		CompressionWritesMemoryAllowed:        false,
		LLME2ERequiredAfterSessionCompaction:  true,
	}
	report.EstimatedTranscriptTokens = estimatePromptTokens(report.TranscriptBodyBytes)
	report.AgentCompactionRecommended = report.TranscriptBodyBytes >= report.AgentCompressionThresholdBytes && report.AgentCompressionThresholdBytes > 0
	report.GatewayHygieneRecommended = report.TranscriptBodyBytes >= report.GatewayHygieneThresholdBytes && report.GatewayHygieneThresholdBytes > 0
	for _, msg := range bounded {
		if len(msg.Body) > budget.MaxTranscriptMessageBytes {
			report.TruncatedTranscriptBodies++
		}
	}
	for _, turn := range provenance.Turns {
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
	report.Cards = buildSessionCompactionCards(transcript, budget)
	switch {
	case report.TranscriptMessages == 0:
		report.SessionCompactionStatus = "empty"
	case report.AgentCompactionRecommended ||
		report.GatewayHygieneRecommended ||
		report.OmittedOlderMessages > 0 ||
		report.TruncatedTranscriptBodies > 0 ||
		(report.AssistantTurnComments > 0 && report.ModelBackedAssistantTurns == 0):
		report.SessionCompactionStatus = "warn"
	default:
		report.SessionCompactionStatus = "ok"
	}
	return report
}

func RenderSessionCompactionReport(report SessionCompactionReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Compaction Report\n\n")
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
	fmt.Fprintf(&b, "- session_compaction_status: `%s`\n", report.SessionCompactionStatus)
	fmt.Fprintf(&b, "- compaction_scope: `%s`\n", "body-free-session-compaction-readiness")
	fmt.Fprintf(&b, "- compaction_strategy: `%s`\n", report.CompactionStrategy)
	fmt.Fprintf(&b, "- compression_model: `%s`\n", report.CompressionModel)
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- trusted_messages: `%d`\n", report.TrustedMessages)
	fmt.Fprintf(&b, "- untrusted_messages: `%d`\n", report.UntrustedMessages)
	fmt.Fprintf(&b, "- edited_messages: `%d`\n", report.EditedMessages)
	fmt.Fprintf(&b, "- transcript_body_bytes: `%d`\n", report.TranscriptBodyBytes)
	fmt.Fprintf(&b, "- transcript_body_lines: `%d`\n", report.TranscriptBodyLines)
	fmt.Fprintf(&b, "- estimated_transcript_tokens: `%d`\n", report.EstimatedTranscriptTokens)
	fmt.Fprintf(&b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(&b, "- max_transcript_messages: `%d`\n", report.MaxTranscriptMessages)
	fmt.Fprintf(&b, "- max_transcript_message_bytes: `%d`\n", report.MaxTranscriptMessageBytes)
	fmt.Fprintf(&b, "- bounded_transcript_messages: `%d`\n", report.BoundedTranscriptMessages)
	fmt.Fprintf(&b, "- omitted_older_messages: `%d`\n", report.OmittedOlderMessages)
	fmt.Fprintf(&b, "- truncated_transcript_bodies: `%d`\n", report.TruncatedTranscriptBodies)
	fmt.Fprintf(&b, "- agent_compression_threshold_percent: `%d`\n", report.AgentCompressionThresholdPercent)
	fmt.Fprintf(&b, "- agent_compression_threshold_bytes: `%d`\n", report.AgentCompressionThresholdBytes)
	fmt.Fprintf(&b, "- gateway_hygiene_threshold_percent: `%d`\n", report.GatewayHygieneThresholdPercent)
	fmt.Fprintf(&b, "- gateway_hygiene_threshold_bytes: `%d`\n", report.GatewayHygieneThresholdBytes)
	fmt.Fprintf(&b, "- agent_compaction_recommended: `%t`\n", report.AgentCompactionRecommended)
	fmt.Fprintf(&b, "- gateway_hygiene_recommended: `%t`\n", report.GatewayHygieneRecommended)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(&b, "- usage_prompt_tokens: `%d`\n", report.UsagePromptTokens)
	fmt.Fprintf(&b, "- usage_completion_tokens: `%d`\n", report.UsageCompletionTokens)
	fmt.Fprintf(&b, "- usage_total_tokens: `%d`\n", report.UsageTotalTokens)
	fmt.Fprintf(&b, "- usage_cache_read_tokens: `%d`\n", report.UsageCacheReadTokens)
	fmt.Fprintf(&b, "- usage_cache_write_tokens: `%d`\n", report.UsageCacheWriteTokens)
	fmt.Fprintf(&b, "- compression_engine_configured: `%t`\n", report.CompressionEngineConfigured)
	fmt.Fprintf(&b, "- lossy_summary_supported: `%t`\n", report.LossySummarySupported)
	fmt.Fprintf(&b, "- lossless_session_search_supported: `%t`\n", report.LosslessSessionSearchSupported)
	fmt.Fprintf(&b, "- in_loop_context_compression_supported: `%t`\n", report.InLoopContextCompressionSupported)
	fmt.Fprintf(&b, "- pre_agent_gateway_hygiene_supported: `%t`\n", report.PreAgentGatewayHygieneSupported)
	fmt.Fprintf(&b, "- session_split_supported: `%t`\n", report.SessionSplitSupported)
	fmt.Fprintf(&b, "- external_session_db_required: `%t`\n", report.ExternalSessionDBRequired)
	fmt.Fprintf(&b, "- issue_thread_canonical_storage: `%t`\n", report.IssueThreadCanonicalStorage)
	fmt.Fprintf(&b, "- backup_branch_replay_preferred: `%t`\n", report.BackupBranchReplayPreferred)
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
	fmt.Fprintf(&b, "- compaction_mutation_allowed: `%t`\n", report.CompactionMutationAllowed)
	fmt.Fprintf(&b, "- compression_writes_memory_allowed: `%t`\n", report.CompressionWritesMemoryAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_compaction_change: `%t`\n\n", report.LLME2ERequiredAfterSessionCompaction)
	b.WriteString("This OpenClaw/Hermes-inspired report audits whether the GitHub issue session is nearing a compaction boundary. It models Hermes-style 50% in-loop and 85% gateway-hygiene thresholds plus OpenClaw-style trajectory pruning, but it is advisory only: GitClaw does not summarize, split, mutate memory, or write compressed state from this report. Raw issue bodies, comment bodies, assistant replies, prompts, provider responses, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Compaction Cards\n")
	writeSessionCompactionCards(&b, report.Cards)

	b.WriteString("\n### Compaction Gates\n")
	fmt.Fprintf(&b, "- agent_compaction_gate=`%s`\n", sessionCompactionAgentGate(report))
	fmt.Fprintf(&b, "- gateway_hygiene_gate=`%s`\n", sessionCompactionGatewayGate(report))
	fmt.Fprintf(&b, "- model_backed_gate=`%s`\n", sessionCompactionModelGate(report))
	fmt.Fprintf(&b, "- lossless_recall_gate=`%s`\n", "backup-json-and-session-search")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hashes-counts-and-metadata-only")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func RenderSessionCompactionCLIReport(backupPath string, backup IssueBackup, cfg Config) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionCompactionReport(BuildSessionCompactionReport("local-backup", backupPath, ev, cfg, commentsFromBackup(backup.Comments), backup.Transcript))
}

func buildSessionCompactionCards(transcript []TranscriptMessage, budget Config) []SessionCompactionCard {
	cards := make([]SessionCompactionCard, 0, len(transcript))
	for index, msg := range transcript {
		inPack := sessionCompactionMessageInPromptPack(index, len(transcript), budget.MaxTranscriptMessages)
		truncated := len(msg.Body) > budget.MaxTranscriptMessageBytes && inPack
		card := SessionCompactionCard{
			MessageIndex:        index + 1,
			Role:                msg.Role,
			Source:              sessionMessageSource(msg),
			Actor:               msg.Actor,
			AuthorAssociation:   msg.AuthorAssociation,
			Trusted:             msg.Trusted,
			Edited:              msg.Edited,
			Bytes:               len(msg.Body),
			Lines:               lineCount(msg.Body),
			EstimatedTokens:     estimatePromptTokens(len(msg.Body)),
			SHA:                 shortDocumentHash(msg.Body),
			InCurrentPromptPack: inPack,
			OmittedByLimit:      !inPack,
			TruncatedByLimit:    truncated,
			CompactionRegion:    sessionCompactionRegion(index, len(transcript), inPack, truncated),
			CompactionAction:    sessionCompactionAction(inPack, truncated),
			BodyIncluded:        false,
		}
		cards = append(cards, card)
	}
	return cards
}

func writeSessionCompactionCards(b *strings.Builder, cards []SessionCompactionCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- message=`%02d` role=`%s` source=`%s` actor=`%s` association=`%s` trusted=`%t` edited=`%t` bytes=`%d` lines=`%d` estimated_tokens=`%d` sha256_12=`%s` in_current_prompt_pack=`%t` omitted_by_limit=`%t` truncated_by_limit=`%t` compaction_region=`%s` compaction_action=`%s` body_included=`%t`\n",
			card.MessageIndex,
			card.Role,
			card.Source,
			inlineCode(card.Actor),
			inlineCode(card.AuthorAssociation),
			card.Trusted,
			card.Edited,
			card.Bytes,
			card.Lines,
			card.EstimatedTokens,
			card.SHA,
			card.InCurrentPromptPack,
			card.OmittedByLimit,
			card.TruncatedByLimit,
			card.CompactionRegion,
			card.CompactionAction,
			card.BodyIncluded,
		)
	}
}

func sessionCompactionMessageInPromptPack(index, total, limit int) bool {
	if limit <= 0 || total <= limit {
		return true
	}
	if limit == 1 {
		return index == total-1
	}
	return index == 0 || index >= total-(limit-1)
}

func sessionCompactionRegion(index, total int, inPack, truncated bool) string {
	switch {
	case !inPack:
		return "older-middle-history"
	case index == 0:
		return "session-anchor"
	case index == total-1:
		return "latest-turn"
	case truncated:
		return "oversized-current-prompt-message"
	default:
		return "conversation-tail"
	}
}

func sessionCompactionAction(inPack, truncated bool) string {
	switch {
	case !inPack:
		return "keep-in-issue-and-backup-search"
	case truncated:
		return "truncate-before-prompt-pack"
	default:
		return "keep-body-out-of-report"
	}
}

func sessionCompactionAgentGate(report SessionCompactionReport) string {
	if report.TranscriptMessages == 0 {
		return "no-transcript"
	}
	if report.AgentCompactionRecommended || report.OmittedOlderMessages > 0 || report.TruncatedTranscriptBodies > 0 {
		return "warn"
	}
	return "pass"
}

func sessionCompactionGatewayGate(report SessionCompactionReport) string {
	if report.TranscriptMessages == 0 {
		return "no-transcript"
	}
	if report.GatewayHygieneRecommended {
		return "warn"
	}
	return "pass"
}

func sessionCompactionModelGate(report SessionCompactionReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.ModelBackedAssistantTurns == 0 {
		return "warn"
	}
	return "pass"
}
