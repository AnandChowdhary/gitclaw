package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type PromptCompressionSegment struct {
	Kind              string
	Name              string
	Index             int
	CompressionRegion string
	CompressionAction string
	PackStatus        string
	Bytes             int
	Lines             int
	EstimatedTok      int
	SHA               string
	BodyIncluded      bool
	Metadata          []string
}

type PromptCompressionReport struct {
	Status                                     string
	CompressionStrategy                        string
	CompressionModel                           string
	Provider                                   string
	Model                                      string
	MaxPromptBytes                             int
	MaxOutputTokens                            int
	SystemPromptBytes                          int
	SystemPromptEstimatedTokens                int
	SystemPromptSHA                            string
	FullUserPromptBytes                        int
	PackedUserPromptBytes                      int
	TotalModelInputBytes                       int
	EstimatedInputTokens                       int
	UserPromptBudgetPercent                    int
	AgentCompressionThresholdPercent           int
	AgentCompressionThresholdBytes             int
	GatewayHygieneThresholdPercent             int
	GatewayHygieneThresholdBytes               int
	AgentCompressionRecommended                bool
	GatewayHygieneRecommended                  bool
	FinalPackTruncationActive                  bool
	CompressionEngineConfigured                bool
	LossySummarySupported                      bool
	LosslessSessionSearchSupported             bool
	PreAgentGatewayHygieneSupported            bool
	InLoopContextCompressionSupported          bool
	CompressionWritesMemoryAllowed             bool
	SessionSplitSupported                      bool
	ExternalSessionDBRequired                  bool
	IssueThreadCanonicalStorage                bool
	BackupBranchReplayPreferred                bool
	ContextFiles                               int
	SelectedSkills                             int
	ToolOutputs                                int
	TranscriptMessages                         int
	BoundedTranscriptMessages                  int
	OmittedOlderMessages                       int
	TruncatedTranscriptBodies                  int
	PromptBodyIncluded                         bool
	ContextFileBodiesIncluded                  bool
	SkillBodiesIncluded                        bool
	ToolOutputBodiesIncluded                   bool
	RawIssueBodiesIncluded                     bool
	RawCommentBodiesIncluded                   bool
	CredentialValuesIncluded                   bool
	RepositoryMutationAllowed                  bool
	LLME2ERequiredAfterPromptCompressionChange bool
	Findings                                   []PromptRiskFinding
	HighRiskFindings                           int
	WarningRiskFindings                        int
	InfoRiskFindings                           int
	Segments                                   []PromptCompressionSegment
}

func IsPromptCompressionRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return isPromptCompressionFields(fields)
}

func isPromptCompressionFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/prompt" && fields[0] != "/budget" && fields[0] != "/prompt-budget" {
		return false
	}
	return strings.EqualFold(fields[1], "compression") ||
		strings.EqualFold(fields[1], "compress") ||
		strings.EqualFold(fields[1], "compaction") ||
		strings.EqualFold(fields[1], "compact") ||
		strings.EqualFold(fields[1], "summarization") ||
		strings.EqualFold(fields[1], "summary")
}

func RenderPromptCompressionReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderPromptCompressionReport(ev, cfg, transcript, repoContext, true)
}

func RenderPromptCompressionCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPromptCompressionReport(Event{}, cfg, nil, repoContext, false)
}

func renderPromptCompressionReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildPromptCompressionReport(ev, cfg, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Prompt Compression Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writePromptCompressionSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits whether the current prompt envelope would benefit from compression or compaction. It models Hermes-style gateway and in-loop thresholds plus OpenClaw-style session pruning as read-only metadata only; GitClaw does not create lossy summaries, split sessions, mutate memory, or write compressed state from this report. Prompt text, issue/comment bodies, context bodies, skill bodies, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Compression Segments\n")
	writePromptCompressionSegments(&b, report.Segments)

	b.WriteString("\n### Findings\n")
	writePromptRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPromptCompressionReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) PromptCompressionReport {
	budget := promptBudgetConfig(cfg)
	components, fullPrompt := promptPackComponents(ev, budget, transcript, repoContext)
	components = clampPromptPackComponentRanges(components, len(fullPrompt))
	packedPrompt := truncatePromptText(fullPrompt, budget.MaxPromptBytes)
	projected := applyPromptPackProjection(components, len(fullPrompt), budget.MaxPromptBytes)
	bounded, omitted := boundedTranscript(transcript, budget.MaxTranscriptMessages)
	report := PromptCompressionReport{
		Status:                                     "ok",
		CompressionStrategy:                        "stateless-github-issue-bounded-prompt-audit",
		CompressionModel:                           "hermes-dual-thresholds+openclaw-session-pruning",
		Provider:                                   cfg.ModelProvider,
		Model:                                      cfg.Model,
		MaxPromptBytes:                             budget.MaxPromptBytes,
		MaxOutputTokens:                            cfg.MaxOutputTokens,
		SystemPromptBytes:                          len(systemPrompt),
		SystemPromptEstimatedTokens:                estimatePromptTokens(len(systemPrompt)),
		SystemPromptSHA:                            shortDocumentHash(systemPrompt),
		FullUserPromptBytes:                        len(fullPrompt),
		PackedUserPromptBytes:                      len(packedPrompt),
		TotalModelInputBytes:                       len(systemPrompt) + len(packedPrompt),
		EstimatedInputTokens:                       estimatePromptTokens(len(systemPrompt) + len(packedPrompt)),
		UserPromptBudgetPercent:                    percentOf(len(packedPrompt), budget.MaxPromptBytes),
		AgentCompressionThresholdPercent:           promptPackAgentCompressionPct,
		AgentCompressionThresholdBytes:             budget.MaxPromptBytes * promptPackAgentCompressionPct / 100,
		GatewayHygieneThresholdPercent:             promptPackGatewayHygienePct,
		GatewayHygieneThresholdBytes:               budget.MaxPromptBytes * promptPackGatewayHygienePct / 100,
		CompressionEngineConfigured:                false,
		LossySummarySupported:                      false,
		LosslessSessionSearchSupported:             true,
		PreAgentGatewayHygieneSupported:            false,
		InLoopContextCompressionSupported:          false,
		CompressionWritesMemoryAllowed:             false,
		SessionSplitSupported:                      false,
		ExternalSessionDBRequired:                  false,
		IssueThreadCanonicalStorage:                true,
		BackupBranchReplayPreferred:                true,
		ContextFiles:                               len(repoContext.Documents),
		SelectedSkills:                             len(repoContext.Skills),
		ToolOutputs:                                len(repoContext.ToolOutputs),
		TranscriptMessages:                         len(transcript),
		BoundedTranscriptMessages:                  len(bounded),
		OmittedOlderMessages:                       omitted,
		PromptBodyIncluded:                         false,
		ContextFileBodiesIncluded:                  false,
		SkillBodiesIncluded:                        false,
		ToolOutputBodiesIncluded:                   false,
		RawIssueBodiesIncluded:                     false,
		RawCommentBodiesIncluded:                   false,
		CredentialValuesIncluded:                   false,
		RepositoryMutationAllowed:                  false,
		LLME2ERequiredAfterPromptCompressionChange: true,
	}
	report.AgentCompressionRecommended = report.FullUserPromptBytes >= report.AgentCompressionThresholdBytes && report.AgentCompressionThresholdBytes > 0
	report.GatewayHygieneRecommended = report.FullUserPromptBytes >= report.GatewayHygieneThresholdBytes && report.GatewayHygieneThresholdBytes > 0
	report.FinalPackTruncationActive = len(fullPrompt) > budget.MaxPromptBytes || strings.Contains(packedPrompt, "[gitclaw:truncated")
	for _, msg := range bounded {
		if len(msg.Body) > budget.MaxTranscriptMessageBytes {
			report.TruncatedTranscriptBodies++
		}
	}
	report.Segments = promptCompressionSegments(projected)
	report.Findings = promptCompressionFindings(report)
	sortPromptRiskFindings(report.Findings)
	for _, finding := range report.Findings {
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

func promptCompressionSegments(cards []PromptPackComponent) []PromptCompressionSegment {
	segments := []PromptCompressionSegment{{
		Kind:              "system-prompt",
		Name:              "gitclaw-system-prompt",
		Index:             0,
		CompressionRegion: "stable-system-prefix",
		CompressionAction: "keep",
		PackStatus:        "separate",
		Bytes:             len(systemPrompt),
		Lines:             lineCount(systemPrompt),
		EstimatedTok:      estimatePromptTokens(len(systemPrompt)),
		SHA:               shortDocumentHash(systemPrompt),
		BodyIncluded:      false,
		Metadata:          []string{"source:compiled"},
	}}
	for _, card := range cards {
		segments = append(segments, PromptCompressionSegment{
			Kind:              card.Kind,
			Name:              card.Name,
			Index:             card.Index,
			CompressionRegion: promptCompressionRegion(card),
			CompressionAction: promptCompressionAction(card),
			PackStatus:        card.PackStatus,
			Bytes:             card.Bytes,
			Lines:             card.Lines,
			EstimatedTok:      estimatePromptTokens(card.Bytes),
			SHA:               card.SHA,
			BodyIncluded:      false,
			Metadata:          card.Metadata,
		})
	}
	return segments
}

func promptCompressionRegion(card PromptPackComponent) string {
	switch {
	case card.Kind == "tool-output":
		return "dynamic-tool-context"
	case card.Kind == "transcript-message":
		return "conversation-tail"
	case card.Kind == "transcript-omission-marker":
		return "conversation-omission"
	case strings.HasPrefix(card.Kind, "transcript") || (card.Kind == "section-header" && card.Name == "transcript"):
		return "conversation-boundary"
	case card.Kind == "context-file" || card.Kind == "selected-skill":
		return "stable-context-prefix"
	default:
		return "prompt-structure"
	}
}

func promptCompressionAction(card PromptPackComponent) string {
	if card.PackStatus == "partial" {
		return "partial-after-head-tail-truncation"
	}
	if card.PackStatus == "omitted" {
		return "omitted-by-head-tail-truncation"
	}
	switch card.Kind {
	case "transcript-message":
		return "keep-bounded-message"
	case "transcript-omission-marker":
		return "keep-omission-marker"
	case "tool-output":
		return "keep-prompt-visible-tool-output"
	default:
		return "keep"
	}
}

func promptCompressionFindings(report PromptCompressionReport) []PromptRiskFinding {
	var findings []PromptRiskFinding
	add := func(severity, code, category, kind, path, field, value string) {
		findings = append(findings, PromptRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Kind:     kind,
			Path:     path,
			Field:    field,
			LineSHA:  shortDocumentHash(value),
		})
	}
	add("info", "hermes_dual_compression_thresholds_modeled", "context-compression", "prompt-compression", "gitclaw", "compression_model", report.CompressionModel)
	add("info", "openclaw_session_pruning_boundary_modeled", "session-pruning", "prompt-compression", "gitclaw", "compression_strategy", report.CompressionStrategy)
	add("warning", "lossy_compression_engine_disabled", "context-compression", "prompt-compression", "gitclaw", "compression_engine_configured", fmt.Sprintf("%t", report.CompressionEngineConfigured))
	add("info", "github_issue_thread_is_canonical_storage", "session-storage", "github-issue", "gitclaw", "issue_thread_canonical_storage", fmt.Sprintf("%t", report.IssueThreadCanonicalStorage))
	if report.AgentCompressionRecommended {
		add("warning", "agent_compression_threshold_crossed", "context-compression", "prompt-compression", "gitclaw", "agent_threshold", fmt.Sprintf("%d", report.AgentCompressionThresholdBytes))
	}
	if report.GatewayHygieneRecommended {
		add("warning", "gateway_hygiene_threshold_crossed", "context-compression", "prompt-compression", "gitclaw", "gateway_threshold", fmt.Sprintf("%d", report.GatewayHygieneThresholdBytes))
	}
	if report.FinalPackTruncationActive {
		add("warning", "final_prompt_pack_truncation_active", "prompt-budget", "prompt-pack", "gitclaw", "max_prompt_bytes", fmt.Sprintf("%d", report.MaxPromptBytes))
	}
	if report.OmittedOlderMessages > 0 {
		add("info", "older_transcript_messages_omitted", "transcript-budget", "transcript", "transcript", "omitted_older_messages", fmt.Sprintf("%d", report.OmittedOlderMessages))
	}
	if report.TruncatedTranscriptBodies > 0 {
		add("warning", "transcript_message_bodies_truncated", "transcript-budget", "transcript", "transcript", "truncated_transcript_bodies", fmt.Sprintf("%d", report.TruncatedTranscriptBodies))
	}
	if report.BackupBranchReplayPreferred {
		add("info", "backup_branch_replay_preferred", "session-storage", "backup", "gitclaw-backups", "backup_branch_replay", "preferred")
	}
	sortPromptRiskFindings(findings)
	return findings
}

func writePromptCompressionSummary(b *strings.Builder, report PromptCompressionReport) {
	fmt.Fprintf(b, "- prompt_compression_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- compression_strategy: `%s`\n", report.CompressionStrategy)
	fmt.Fprintf(b, "- compression_model: `%s`\n", report.CompressionModel)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(b, "- max_output_tokens: `%d`\n", report.MaxOutputTokens)
	fmt.Fprintf(b, "- system_prompt_bytes: `%d`\n", report.SystemPromptBytes)
	fmt.Fprintf(b, "- system_prompt_estimated_tokens: `%d`\n", report.SystemPromptEstimatedTokens)
	fmt.Fprintf(b, "- system_prompt_sha256_12: `%s`\n", report.SystemPromptSHA)
	fmt.Fprintf(b, "- full_user_prompt_bytes: `%d`\n", report.FullUserPromptBytes)
	fmt.Fprintf(b, "- packed_user_prompt_bytes: `%d`\n", report.PackedUserPromptBytes)
	fmt.Fprintf(b, "- total_model_input_bytes: `%d`\n", report.TotalModelInputBytes)
	fmt.Fprintf(b, "- estimated_input_tokens: `%d`\n", report.EstimatedInputTokens)
	fmt.Fprintf(b, "- user_prompt_budget_percent: `%d`\n", report.UserPromptBudgetPercent)
	fmt.Fprintf(b, "- agent_compression_threshold_percent: `%d`\n", report.AgentCompressionThresholdPercent)
	fmt.Fprintf(b, "- agent_compression_threshold_bytes: `%d`\n", report.AgentCompressionThresholdBytes)
	fmt.Fprintf(b, "- gateway_hygiene_threshold_percent: `%d`\n", report.GatewayHygieneThresholdPercent)
	fmt.Fprintf(b, "- gateway_hygiene_threshold_bytes: `%d`\n", report.GatewayHygieneThresholdBytes)
	fmt.Fprintf(b, "- agent_compression_recommended: `%t`\n", report.AgentCompressionRecommended)
	fmt.Fprintf(b, "- gateway_hygiene_recommended: `%t`\n", report.GatewayHygieneRecommended)
	fmt.Fprintf(b, "- final_pack_truncation_active: `%t`\n", report.FinalPackTruncationActive)
	fmt.Fprintf(b, "- compression_engine_configured: `%t`\n", report.CompressionEngineConfigured)
	fmt.Fprintf(b, "- lossy_summary_supported: `%t`\n", report.LossySummarySupported)
	fmt.Fprintf(b, "- lossless_session_search_supported: `%t`\n", report.LosslessSessionSearchSupported)
	fmt.Fprintf(b, "- pre_agent_gateway_hygiene_supported: `%t`\n", report.PreAgentGatewayHygieneSupported)
	fmt.Fprintf(b, "- in_loop_context_compression_supported: `%t`\n", report.InLoopContextCompressionSupported)
	fmt.Fprintf(b, "- compression_writes_memory_allowed: `%t`\n", report.CompressionWritesMemoryAllowed)
	fmt.Fprintf(b, "- session_split_supported: `%t`\n", report.SessionSplitSupported)
	fmt.Fprintf(b, "- external_session_db_required: `%t`\n", report.ExternalSessionDBRequired)
	fmt.Fprintf(b, "- issue_thread_canonical_storage: `%t`\n", report.IssueThreadCanonicalStorage)
	fmt.Fprintf(b, "- backup_branch_replay_preferred: `%t`\n", report.BackupBranchReplayPreferred)
	fmt.Fprintf(b, "- context_files: `%d`\n", report.ContextFiles)
	fmt.Fprintf(b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(b, "- tool_outputs: `%d`\n", report.ToolOutputs)
	fmt.Fprintf(b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(b, "- bounded_transcript_messages: `%d`\n", report.BoundedTranscriptMessages)
	fmt.Fprintf(b, "- omitted_older_messages: `%d`\n", report.OmittedOlderMessages)
	fmt.Fprintf(b, "- truncated_transcript_bodies: `%d`\n", report.TruncatedTranscriptBodies)
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- prompt_body_included: `%t`\n", report.PromptBodyIncluded)
	fmt.Fprintf(b, "- context_file_bodies_included: `%t`\n", report.ContextFileBodiesIncluded)
	fmt.Fprintf(b, "- skill_bodies_included: `%t`\n", report.SkillBodiesIncluded)
	fmt.Fprintf(b, "- tool_output_bodies_included: `%t`\n", report.ToolOutputBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_prompt_compression_change: `%t`\n", report.LLME2ERequiredAfterPromptCompressionChange)
}

func writePromptCompressionSegments(b *strings.Builder, segments []PromptCompressionSegment) {
	if len(segments) == 0 {
		b.WriteString("- none\n")
		return
	}
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].Index < segments[j].Index })
	for _, segment := range segments {
		fmt.Fprintf(
			b,
			"- index=`%d` kind=`%s` name=`%s` compression_region=`%s` compression_action=`%s` pack_status=`%s` bytes=`%d` lines=`%d` estimated_tokens=`%d` sha256_12=`%s` body_included=`%t` metadata=`%s`\n",
			segment.Index,
			segment.Kind,
			inlineCode(segment.Name),
			segment.CompressionRegion,
			segment.CompressionAction,
			segment.PackStatus,
			segment.Bytes,
			segment.Lines,
			segment.EstimatedTok,
			segment.SHA,
			segment.BodyIncluded,
			inlineListOrNone(segment.Metadata),
		)
	}
}
