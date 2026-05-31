package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PromptCacheSegment struct {
	Kind         string
	Name         string
	Index        int
	CacheRegion  string
	BoundaryRole string
	Bytes        int
	Lines        int
	EstimatedTok int
	SHA          string
	BodyIncluded bool
	Metadata     []string
}

type PromptCacheReport struct {
	Status                               string
	CacheStrategy                        string
	CacheModel                           string
	Provider                             string
	Model                                string
	ProviderCacheMode                    string
	AutomaticPrefixCachePossible         bool
	CacheControlRequestFieldsForwarded   bool
	PromptCacheKeyForwarded              bool
	PromptCacheRetentionForwarded        bool
	CacheUsageCountersAvailable          bool
	CacheReadTokensObserved              bool
	CacheWriteTokensObserved             bool
	ContextPruningCacheTTLConfigured     bool
	HeartbeatKeepWarmWorkflowPresent     bool
	SystemPromptBytes                    int
	SystemPromptEstimatedTokens          int
	SystemPromptSHA                      string
	FullUserPromptBytes                  int
	PackedUserPromptBytes                int
	TotalModelInputBytes                 int
	EstimatedInputTokens                 int
	StableUserPrefixBytes                int
	StableUserPrefixEstimatedTokens      int
	StableModelPrefixBytes               int
	StableModelPrefixEstimatedTokens     int
	DynamicSuffixBytes                   int
	CacheablePrefixPercent               int
	BoundaryComponentIndex               int
	BoundaryComponentKind                string
	BoundaryComponentName                string
	BoundaryReason                       string
	ContextFiles                         int
	SelectedSkills                       int
	ToolOutputs                          int
	TranscriptMessages                   int
	BoundedTranscriptMessages            int
	OmittedOlderMessages                 int
	TruncatedTranscriptBodies            int
	PromptBodyIncluded                   bool
	ContextFileBodiesIncluded            bool
	SkillBodiesIncluded                  bool
	ToolOutputBodiesIncluded             bool
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	CredentialValuesIncluded             bool
	RepositoryMutationAllowed            bool
	LLME2ERequiredAfterPromptCacheChange bool
	Findings                             []PromptRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	Segments                             []PromptCacheSegment
}

func IsPromptCacheRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return isPromptCacheFields(fields)
}

func isPromptCacheFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/prompt" && fields[0] != "/budget" && fields[0] != "/prompt-budget" {
		return false
	}
	return strings.EqualFold(fields[1], "cache") ||
		strings.EqualFold(fields[1], "cache-plan") ||
		strings.EqualFold(fields[1], "cache-status") ||
		strings.EqualFold(fields[1], "cache-readiness")
}

func RenderPromptCacheReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderPromptCacheReport(ev, cfg, transcript, repoContext, true)
}

func RenderPromptCacheCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPromptCacheReport(Event{}, cfg, nil, repoContext, false)
}

func renderPromptCacheReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildPromptCacheReport(ev, cfg, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Prompt Cache Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writePromptCacheSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits prompt-cache readiness without enabling cache controls. It models the same-issue stable prefix, dynamic suffix, provider/cache telemetry gaps, heartbeat keep-warm surface, and cache-sensitive prompt boundaries. Prompt text, issue bodies, comments, context bodies, skill bodies, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Cache Segments\n")
	writePromptCacheSegments(&b, report.Segments)

	b.WriteString("\n### Findings\n")
	writePromptRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPromptCacheReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) PromptCacheReport {
	budget := promptBudgetConfig(cfg)
	components, fullPrompt := promptPackComponents(ev, budget, transcript, repoContext)
	components = clampPromptPackComponentRanges(components, len(fullPrompt))
	packedPrompt := truncatePromptText(fullPrompt, budget.MaxPromptBytes)
	segments, stableUserBytes, boundaryIndex, boundaryKind, boundaryName, boundaryReason := promptCacheSegments(components)
	stableModelBytes := len(systemPrompt) + stableUserBytes
	totalInputBytes := len(systemPrompt) + len(packedPrompt)
	bounded, omitted := boundedTranscript(transcript, budget.MaxTranscriptMessages)
	report := PromptCacheReport{
		Status:                               "ok",
		CacheStrategy:                        "same-issue-stable-prefix-audit",
		CacheModel:                           "openclaw-cache-boundary+hermes-cache-compression",
		Provider:                             cfg.ModelProvider,
		Model:                                cfg.Model,
		ProviderCacheMode:                    promptCacheProviderMode(cfg),
		AutomaticPrefixCachePossible:         promptCacheAutomaticPrefixPossible(cfg),
		CacheControlRequestFieldsForwarded:   false,
		PromptCacheKeyForwarded:              false,
		PromptCacheRetentionForwarded:        false,
		CacheUsageCountersAvailable:          false,
		CacheReadTokensObserved:              false,
		CacheWriteTokensObserved:             false,
		ContextPruningCacheTTLConfigured:     false,
		HeartbeatKeepWarmWorkflowPresent:     promptCacheHeartbeatWorkflowPresent(cfg.Workdir),
		SystemPromptBytes:                    len(systemPrompt),
		SystemPromptEstimatedTokens:          estimatePromptTokens(len(systemPrompt)),
		SystemPromptSHA:                      shortDocumentHash(systemPrompt),
		FullUserPromptBytes:                  len(fullPrompt),
		PackedUserPromptBytes:                len(packedPrompt),
		TotalModelInputBytes:                 totalInputBytes,
		EstimatedInputTokens:                 estimatePromptTokens(totalInputBytes),
		StableUserPrefixBytes:                stableUserBytes,
		StableUserPrefixEstimatedTokens:      estimatePromptTokens(stableUserBytes),
		StableModelPrefixBytes:               stableModelBytes,
		StableModelPrefixEstimatedTokens:     estimatePromptTokens(stableModelBytes),
		DynamicSuffixBytes:                   maxInt(0, totalInputBytes-stableModelBytes),
		CacheablePrefixPercent:               percentOf(stableModelBytes, totalInputBytes),
		BoundaryComponentIndex:               boundaryIndex,
		BoundaryComponentKind:                boundaryKind,
		BoundaryComponentName:                boundaryName,
		BoundaryReason:                       boundaryReason,
		ContextFiles:                         len(repoContext.Documents),
		SelectedSkills:                       len(repoContext.Skills),
		ToolOutputs:                          len(repoContext.ToolOutputs),
		TranscriptMessages:                   len(transcript),
		BoundedTranscriptMessages:            len(bounded),
		OmittedOlderMessages:                 omitted,
		PromptBodyIncluded:                   false,
		ContextFileBodiesIncluded:            false,
		SkillBodiesIncluded:                  false,
		ToolOutputBodiesIncluded:             false,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		CredentialValuesIncluded:             false,
		RepositoryMutationAllowed:            false,
		LLME2ERequiredAfterPromptCacheChange: true,
		Segments:                             segments,
	}
	for _, msg := range bounded {
		if len(msg.Body) > budget.MaxTranscriptMessageBytes {
			report.TruncatedTranscriptBodies++
		}
	}
	report.Findings = promptCacheFindings(report)
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

func promptCacheSegments(components []PromptPackComponent) ([]PromptCacheSegment, int, int, string, string, string) {
	segments := []PromptCacheSegment{{
		Kind:         "system-prompt",
		Name:         "gitclaw-system-prompt",
		Index:        0,
		CacheRegion:  "stable-prefix",
		BoundaryRole: "system-prefix",
		Bytes:        len(systemPrompt),
		Lines:        lineCount(systemPrompt),
		EstimatedTok: estimatePromptTokens(len(systemPrompt)),
		SHA:          shortDocumentHash(systemPrompt),
		BodyIncluded: false,
		Metadata:     []string{"source:compiled"},
	}}
	stableUserBytes := 0
	boundaryIndex := 0
	boundaryKind := "none"
	boundaryName := "none"
	boundaryReason := "no_dynamic_boundary"
	inStablePrefix := true
	for _, card := range components {
		if inStablePrefix && promptCacheStartsDynamicSuffix(card) {
			inStablePrefix = false
			boundaryIndex = card.Index
			boundaryKind = card.Kind
			boundaryName = card.Name
			boundaryReason = promptCacheBoundaryReason(card)
		}
		region := "dynamic-suffix"
		role := "volatile"
		if inStablePrefix {
			region = "stable-prefix"
			role = "same-issue-prefix"
			stableUserBytes += card.Bytes
		} else if card.Index == boundaryIndex {
			role = "boundary-start"
		}
		segments = append(segments, PromptCacheSegment{
			Kind:         card.Kind,
			Name:         card.Name,
			Index:        card.Index,
			CacheRegion:  region,
			BoundaryRole: role,
			Bytes:        card.Bytes,
			Lines:        card.Lines,
			EstimatedTok: estimatePromptTokens(card.Bytes),
			SHA:          card.SHA,
			BodyIncluded: false,
			Metadata:     card.Metadata,
		})
	}
	return segments, stableUserBytes, boundaryIndex, boundaryKind, boundaryName, boundaryReason
}

func promptCacheStartsDynamicSuffix(card PromptPackComponent) bool {
	if card.Kind == "tool-output" {
		return true
	}
	if strings.HasPrefix(card.Kind, "transcript") {
		return true
	}
	return card.Kind == "section-header" && card.Name == "transcript"
}

func promptCacheBoundaryReason(card PromptPackComponent) string {
	switch {
	case card.Kind == "tool-output":
		return "before_dynamic_tool_outputs"
	case strings.HasPrefix(card.Kind, "transcript") || (card.Kind == "section-header" && card.Name == "transcript"):
		return "before_transcript_history"
	default:
		return "before_dynamic_suffix"
	}
}

func promptCacheProviderMode(cfg Config) string {
	switch {
	case cfg.ModelProvider == "github-models" && strings.HasPrefix(cfg.Model, "openai/"):
		return "github-models-openai-compatible-observe-only"
	case strings.HasPrefix(cfg.Model, "openai/"):
		return "openai-compatible-observe-only"
	case strings.Contains(cfg.Model, "anthropic") || strings.Contains(cfg.Model, "claude"):
		return "anthropic-compatible-observe-only"
	default:
		return "provider-cache-unknown-observe-only"
	}
}

func promptCacheAutomaticPrefixPossible(cfg Config) bool {
	model := strings.ToLower(cfg.Model)
	return strings.HasPrefix(model, "openai/") ||
		strings.Contains(model, "gpt-") ||
		strings.Contains(model, "claude") ||
		strings.Contains(model, "anthropic") ||
		strings.Contains(model, "gemini")
}

func promptCacheHeartbeatWorkflowPresent(workdir string) bool {
	root := workdir
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	_, err := os.Stat(filepath.Join(root, ".github", "workflows", "gitclaw-heartbeat.yml"))
	return err == nil
}

func promptCacheFindings(report PromptCacheReport) []PromptRiskFinding {
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
	add("info", "openclaw_prompt_cache_boundary_modeled", "prompt-cache", "prompt-cache", "gitclaw", "cache_strategy", report.CacheStrategy)
	add("info", "hermes_cache_compression_interaction_modeled", "prompt-cache", "prompt-cache", "gitclaw", "cache_model", report.CacheModel)
	if report.AutomaticPrefixCachePossible {
		add("info", "provider_prefix_cache_possible", "prompt-cache", "provider", report.Provider, "provider_cache_mode", report.ProviderCacheMode)
	} else {
		add("warning", "provider_prefix_cache_unknown", "prompt-cache", "provider", report.Provider, "provider_cache_mode", report.ProviderCacheMode)
	}
	if !report.CacheControlRequestFieldsForwarded || !report.PromptCacheKeyForwarded || !report.PromptCacheRetentionForwarded {
		add("warning", "cache_request_controls_not_forwarded", "prompt-cache", "provider", report.Provider, "cache_controls", report.ProviderCacheMode)
	}
	if !report.CacheUsageCountersAvailable {
		add("warning", "cache_usage_counters_unavailable", "prompt-cache", "provider", report.Provider, "usage_counters", report.Model)
	}
	if report.ToolOutputs > 0 {
		add("warning", "dynamic_tool_outputs_limit_prefix_reuse", "prompt-cache", "tool-output", "prompt-visible-tools", "tool_outputs", fmt.Sprintf("%d", report.ToolOutputs))
	}
	if report.BoundaryComponentIndex > 0 {
		add("info", "cache_boundary_before_dynamic_suffix", "prompt-cache", "prompt-component", report.BoundaryComponentName, "boundary_reason", report.BoundaryReason)
	}
	if report.HeartbeatKeepWarmWorkflowPresent {
		add("info", "heartbeat_keepwarm_workflow_present", "prompt-cache", "workflow", ".github/workflows/gitclaw-heartbeat.yml", "keepwarm_surface", "present")
	} else {
		add("warning", "heartbeat_keepwarm_workflow_missing", "prompt-cache", "workflow", ".github/workflows/gitclaw-heartbeat.yml", "keepwarm_surface", "missing")
	}
	if report.ContextPruningCacheTTLConfigured {
		add("info", "cache_ttl_pruning_configured", "prompt-cache", "config", gitclawConfigPath, "context_pruning", "configured")
	} else {
		add("info", "cache_ttl_pruning_not_configured", "prompt-cache", "config", gitclawConfigPath, "context_pruning", "not_configured")
	}
	sortPromptRiskFindings(findings)
	return findings
}

func writePromptCacheSummary(b *strings.Builder, report PromptCacheReport) {
	fmt.Fprintf(b, "- prompt_cache_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- cache_strategy: `%s`\n", report.CacheStrategy)
	fmt.Fprintf(b, "- cache_model: `%s`\n", report.CacheModel)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- provider_cache_mode: `%s`\n", report.ProviderCacheMode)
	fmt.Fprintf(b, "- automatic_prefix_cache_possible: `%t`\n", report.AutomaticPrefixCachePossible)
	fmt.Fprintf(b, "- cache_control_request_fields_forwarded: `%t`\n", report.CacheControlRequestFieldsForwarded)
	fmt.Fprintf(b, "- prompt_cache_key_forwarded: `%t`\n", report.PromptCacheKeyForwarded)
	fmt.Fprintf(b, "- prompt_cache_retention_forwarded: `%t`\n", report.PromptCacheRetentionForwarded)
	fmt.Fprintf(b, "- cache_usage_counters_available: `%t`\n", report.CacheUsageCountersAvailable)
	fmt.Fprintf(b, "- cache_read_tokens_observed: `%t`\n", report.CacheReadTokensObserved)
	fmt.Fprintf(b, "- cache_write_tokens_observed: `%t`\n", report.CacheWriteTokensObserved)
	fmt.Fprintf(b, "- context_pruning_cache_ttl_configured: `%t`\n", report.ContextPruningCacheTTLConfigured)
	fmt.Fprintf(b, "- heartbeat_keepwarm_workflow_present: `%t`\n", report.HeartbeatKeepWarmWorkflowPresent)
	fmt.Fprintf(b, "- system_prompt_bytes: `%d`\n", report.SystemPromptBytes)
	fmt.Fprintf(b, "- system_prompt_estimated_tokens: `%d`\n", report.SystemPromptEstimatedTokens)
	fmt.Fprintf(b, "- system_prompt_sha256_12: `%s`\n", report.SystemPromptSHA)
	fmt.Fprintf(b, "- full_user_prompt_bytes: `%d`\n", report.FullUserPromptBytes)
	fmt.Fprintf(b, "- packed_user_prompt_bytes: `%d`\n", report.PackedUserPromptBytes)
	fmt.Fprintf(b, "- total_model_input_bytes: `%d`\n", report.TotalModelInputBytes)
	fmt.Fprintf(b, "- estimated_input_tokens: `%d`\n", report.EstimatedInputTokens)
	fmt.Fprintf(b, "- stable_user_prefix_bytes: `%d`\n", report.StableUserPrefixBytes)
	fmt.Fprintf(b, "- stable_user_prefix_estimated_tokens: `%d`\n", report.StableUserPrefixEstimatedTokens)
	fmt.Fprintf(b, "- stable_model_prefix_bytes: `%d`\n", report.StableModelPrefixBytes)
	fmt.Fprintf(b, "- stable_model_prefix_estimated_tokens: `%d`\n", report.StableModelPrefixEstimatedTokens)
	fmt.Fprintf(b, "- dynamic_suffix_bytes: `%d`\n", report.DynamicSuffixBytes)
	fmt.Fprintf(b, "- cacheable_prefix_percent: `%d`\n", report.CacheablePrefixPercent)
	fmt.Fprintf(b, "- boundary_component_index: `%d`\n", report.BoundaryComponentIndex)
	fmt.Fprintf(b, "- boundary_component_kind: `%s`\n", report.BoundaryComponentKind)
	fmt.Fprintf(b, "- boundary_component_name: `%s`\n", inlineCode(report.BoundaryComponentName))
	fmt.Fprintf(b, "- boundary_reason: `%s`\n", report.BoundaryReason)
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
	fmt.Fprintf(b, "- llm_e2e_required_after_prompt_cache_change: `%t`\n", report.LLME2ERequiredAfterPromptCacheChange)
}

func writePromptCacheSegments(b *strings.Builder, segments []PromptCacheSegment) {
	if len(segments) == 0 {
		b.WriteString("- none\n")
		return
	}
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].Index < segments[j].Index })
	for _, segment := range segments {
		fmt.Fprintf(
			b,
			"- index=`%d` kind=`%s` name=`%s` cache_region=`%s` boundary_role=`%s` bytes=`%d` lines=`%d` estimated_tokens=`%d` sha256_12=`%s` body_included=`%t` metadata=`%s`\n",
			segment.Index,
			segment.Kind,
			inlineCode(segment.Name),
			segment.CacheRegion,
			segment.BoundaryRole,
			segment.Bytes,
			segment.Lines,
			segment.EstimatedTok,
			segment.SHA,
			segment.BodyIncluded,
			inlineListOrNone(segment.Metadata),
		)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
