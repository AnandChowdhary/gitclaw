package gitclaw

import (
	"fmt"
	"os"
	"strings"
)

type ModelUsageReport struct {
	Status                              string
	VerificationScope                   string
	Provider                            string
	Model                               string
	FallbackModels                      []string
	DefaultModelPolicy                  string
	CatalogEndpointHost                 string
	EndpointHost                        string
	TokenSource                         string
	OutputTokenParameter                string
	GitHubModelsEndpoint                bool
	GitHubActionsTokenSupported         bool
	UsageResponseParsingEnabled         bool
	UsageMarkerPersistenceEnabled       bool
	LiveInferenceProbePerformed         bool
	BillingAPIProbePerformed            bool
	CostEstimationSupported             bool
	CostEstimationReason                string
	SystemPromptBytes                   int
	PackedPromptBytes                   int
	TotalModelInputBytes                int
	EstimatedInputTokens                int
	MaxPromptBytes                      int
	MaxOutputTokens                     int
	PromptContainsTruncation            bool
	ContextFiles                        int
	SelectedSkills                      int
	ToolOutputs                         int
	TranscriptMessages                  int
	BoundedTranscriptMessages           int
	OmittedOlderMessages                int
	AssistantTurnMarkers                int
	ModelBackedAssistantTurns           int
	DeterministicAssistantTurns         int
	UsageBearingAssistantTurns          int
	ModelNames                          []string
	RecordedPromptTokens                int
	RecordedCompletionTokens            int
	RecordedTotalTokens                 int
	RecordedCacheReadTokens             int
	RecordedCacheWriteTokens            int
	LatestUsageModel                    string
	LatestUsageTotalTokens              int
	LatestUsagePromptTokens             int
	LatestUsageCompletionTokens         int
	LatestUsageCacheReadTokens          int
	LatestUsageCacheWriteTokens         int
	RawProviderUsageIncluded            bool
	RawProviderResponseIncluded         bool
	RawIssueBodiesIncluded              bool
	RawCommentBodiesIncluded            bool
	RawPromptBodiesIncluded             bool
	RawToolOutputsIncluded              bool
	CredentialValuesIncluded            bool
	LLME2ERequiredAfterModelUsageChange bool
	Findings                            []ModelUsageFinding
}

type ModelUsageFinding struct {
	Severity string
	Code     string
	Detail   string
}

func IsModelUsageRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/model" && fields[0] != "/models" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "usage", "tokens", "token-use", "cost", "costs", "spend":
		return true
	default:
		return false
	}
}

func RenderModelUsageReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderModelUsageReport(ev, cfg, comments, transcript, repoContext, true)
}

func RenderModelUsageCLIReport(cfg Config, repoContext RepoContext) string {
	return renderModelUsageReport(Event{}, cfg, nil, nil, repoContext, false)
}

func renderModelUsageReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildModelUsageReport(ev, cfg, comments, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Model Usage Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeModelUsageSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps the OpenClaw-style token/cost surface onto GitClaw's GitHub Models runtime and Hermes-style session counters. It reports config, prompt projection, normalized marker usage, and telemetry gaps only. It does not perform a live inference probe, query billing APIs, or print provider responses, prompts, issue bodies, comments, tool outputs, credentials, or secret values.\n\n")

	b.WriteString("### Usage Telemetry Cards\n")
	writeModelUsageProviderCard(&b, report)
	writeModelUsagePromptCard(&b, report)
	writeModelUsageSessionCard(&b, report)
	writeModelUsageLatestCard(&b, report)

	b.WriteString("\n### Findings\n")
	writeModelUsageFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildModelUsageReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext) ModelUsageReport {
	baseURL := llmBaseURL(cfg)
	pack := BuildPromptPackReport(ev, cfg, transcript, repoContext)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	report := ModelUsageReport{
		Status:                              "ok",
		VerificationScope:                   "github_models_usage_surface",
		Provider:                            llmProviderForReport(cfg, baseURL),
		Model:                               cfg.Model,
		FallbackModels:                      normalizeModelFallbacks(cfg.ModelFallbacks),
		DefaultModelPolicy:                  "smallest-openai-github-models-catalog-model",
		CatalogEndpointHost:                 llmEndpointHost(defaultGitHubModelsCatalogURL),
		EndpointHost:                        llmEndpointHost(baseURL),
		TokenSource:                         llmTokenSource(baseURL),
		OutputTokenParameter:                llmOutputTokenParam(cfg.Model),
		GitHubModelsEndpoint:                strings.EqualFold(llmEndpointHost(baseURL), "models.github.ai"),
		GitHubActionsTokenSupported:         strings.Contains(baseURL, "models.github.ai"),
		UsageResponseParsingEnabled:         true,
		UsageMarkerPersistenceEnabled:       true,
		LiveInferenceProbePerformed:         false,
		BillingAPIProbePerformed:            false,
		CostEstimationSupported:             false,
		CostEstimationReason:                "pricing_catalog_not_configured",
		SystemPromptBytes:                   pack.SystemPromptBytes,
		PackedPromptBytes:                   pack.PackedUserPromptBytes,
		TotalModelInputBytes:                pack.TotalModelInputBytes,
		EstimatedInputTokens:                pack.EstimatedInputTokens,
		MaxPromptBytes:                      pack.MaxPromptBytes,
		MaxOutputTokens:                     pack.MaxOutputTokens,
		PromptContainsTruncation:            pack.PromptContainsTruncation,
		ContextFiles:                        pack.ContextFiles,
		SelectedSkills:                      pack.SelectedSkills,
		ToolOutputs:                         pack.ToolOutputs,
		TranscriptMessages:                  pack.TranscriptMessages,
		BoundedTranscriptMessages:           pack.BoundedTranscriptMessages,
		OmittedOlderMessages:                pack.OmittedOlderMessages,
		AssistantTurnMarkers:                countSessionMarkers(comments).AssistantTurns,
		ModelBackedAssistantTurns:           modelBackedTurns,
		DeterministicAssistantTurns:         deterministicTurns,
		ModelNames:                          modelNames,
		RawProviderUsageIncluded:            false,
		RawProviderResponseIncluded:         false,
		RawIssueBodiesIncluded:              false,
		RawCommentBodiesIncluded:            false,
		RawPromptBodiesIncluded:             false,
		RawToolOutputsIncluded:              false,
		CredentialValuesIncluded:            false,
		LLME2ERequiredAfterModelUsageChange: true,
	}
	report.RecordedPromptTokens, report.RecordedCompletionTokens, report.RecordedTotalTokens, report.RecordedCacheReadTokens, report.RecordedCacheWriteTokens = modelUsageTotals(provenance.Turns)
	report.UsageBearingAssistantTurns = modelUsageTurnCount(provenance.Turns)
	for i := len(provenance.Turns) - 1; i >= 0; i-- {
		turn := provenance.Turns[i]
		if !turn.Usage.Present {
			continue
		}
		report.LatestUsageModel = turn.Model
		report.LatestUsagePromptTokens = turn.Usage.PromptTokens
		report.LatestUsageCompletionTokens = turn.Usage.CompletionTokens
		report.LatestUsageTotalTokens = turn.Usage.TotalTokens
		report.LatestUsageCacheReadTokens = turn.Usage.CacheReadTokens
		report.LatestUsageCacheWriteTokens = turn.Usage.CacheWriteTokens
		break
	}
	report.Findings = modelUsageFindings(report)
	for _, finding := range report.Findings {
		if finding.Severity == "warning" {
			report.Status = "warn"
			break
		}
	}
	return report
}

func writeModelUsageSummary(b *strings.Builder, report ModelUsageReport) {
	fmt.Fprintf(b, "- model_usage_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- fallback_models: `%s`\n", inlineListOrNone(report.FallbackModels))
	fmt.Fprintf(b, "- default_model_policy: `%s`\n", report.DefaultModelPolicy)
	fmt.Fprintf(b, "- catalog_endpoint_host: `%s`\n", report.CatalogEndpointHost)
	fmt.Fprintf(b, "- endpoint_host: `%s`\n", report.EndpointHost)
	fmt.Fprintf(b, "- token_source: `%s`\n", report.TokenSource)
	fmt.Fprintf(b, "- output_token_parameter: `%s`\n", report.OutputTokenParameter)
	fmt.Fprintf(b, "- github_models_endpoint: `%t`\n", report.GitHubModelsEndpoint)
	fmt.Fprintf(b, "- github_actions_token_supported: `%t`\n", report.GitHubActionsTokenSupported)
	fmt.Fprintf(b, "- usage_response_parsing_enabled: `%t`\n", report.UsageResponseParsingEnabled)
	fmt.Fprintf(b, "- usage_marker_persistence_enabled: `%t`\n", report.UsageMarkerPersistenceEnabled)
	fmt.Fprintf(b, "- live_inference_probe_performed: `%t`\n", report.LiveInferenceProbePerformed)
	fmt.Fprintf(b, "- billing_api_probe_performed: `%t`\n", report.BillingAPIProbePerformed)
	fmt.Fprintf(b, "- cost_estimation_supported: `%t`\n", report.CostEstimationSupported)
	fmt.Fprintf(b, "- cost_estimation_reason: `%s`\n", report.CostEstimationReason)
	fmt.Fprintf(b, "- system_prompt_bytes: `%d`\n", report.SystemPromptBytes)
	fmt.Fprintf(b, "- packed_prompt_bytes: `%d`\n", report.PackedPromptBytes)
	fmt.Fprintf(b, "- total_model_input_bytes: `%d`\n", report.TotalModelInputBytes)
	fmt.Fprintf(b, "- estimated_input_tokens: `%d`\n", report.EstimatedInputTokens)
	fmt.Fprintf(b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(b, "- max_output_tokens: `%d`\n", report.MaxOutputTokens)
	fmt.Fprintf(b, "- prompt_contains_truncation_marker: `%t`\n", report.PromptContainsTruncation)
	fmt.Fprintf(b, "- context_files: `%d`\n", report.ContextFiles)
	fmt.Fprintf(b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(b, "- tool_outputs: `%d`\n", report.ToolOutputs)
	fmt.Fprintf(b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(b, "- bounded_transcript_messages: `%d`\n", report.BoundedTranscriptMessages)
	fmt.Fprintf(b, "- omitted_older_messages: `%d`\n", report.OmittedOlderMessages)
	fmt.Fprintf(b, "- assistant_turn_markers: `%d`\n", report.AssistantTurnMarkers)
	fmt.Fprintf(b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(b, "- recorded_prompt_tokens: `%d`\n", report.RecordedPromptTokens)
	fmt.Fprintf(b, "- recorded_completion_tokens: `%d`\n", report.RecordedCompletionTokens)
	fmt.Fprintf(b, "- recorded_total_tokens: `%d`\n", report.RecordedTotalTokens)
	fmt.Fprintf(b, "- recorded_cache_read_tokens: `%d`\n", report.RecordedCacheReadTokens)
	fmt.Fprintf(b, "- recorded_cache_write_tokens: `%d`\n", report.RecordedCacheWriteTokens)
	fmt.Fprintf(b, "- latest_usage_model: `%s`\n", inlineCode(report.LatestUsageModel))
	fmt.Fprintf(b, "- latest_usage_total_tokens: `%d`\n", report.LatestUsageTotalTokens)
	fmt.Fprintf(b, "- raw_provider_usage_included: `%t`\n", report.RawProviderUsageIncluded)
	fmt.Fprintf(b, "- raw_provider_response_included: `%t`\n", report.RawProviderResponseIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_model_usage_change: `%t`\n", report.LLME2ERequiredAfterModelUsageChange)
}

func writeModelUsageProviderCard(b *strings.Builder, report ModelUsageReport) {
	fmt.Fprintf(
		b,
		"- kind=`provider` provider=`%s` model=`%s` endpoint_host=`%s` token_source=`%s` github_models_endpoint=`%t` github_actions_token_supported=`%t` usage_response_parsing_enabled=`%t` usage_marker_persistence_enabled=`%t` live_inference_probe_performed=`%t` billing_api_probe_performed=`%t` cost_estimation_supported=`%t`\n",
		report.Provider,
		report.Model,
		report.EndpointHost,
		report.TokenSource,
		report.GitHubModelsEndpoint,
		report.GitHubActionsTokenSupported,
		report.UsageResponseParsingEnabled,
		report.UsageMarkerPersistenceEnabled,
		report.LiveInferenceProbePerformed,
		report.BillingAPIProbePerformed,
		report.CostEstimationSupported,
	)
}

func writeModelUsagePromptCard(b *strings.Builder, report ModelUsageReport) {
	fmt.Fprintf(
		b,
		"- kind=`prompt-projection` system_prompt_bytes=`%d` packed_prompt_bytes=`%d` total_model_input_bytes=`%d` estimated_input_tokens=`%d` max_prompt_bytes=`%d` max_output_tokens=`%d` prompt_contains_truncation_marker=`%t` context_files=`%d` selected_skills=`%d` tool_outputs=`%d` transcript_messages=`%d` bounded_transcript_messages=`%d`\n",
		report.SystemPromptBytes,
		report.PackedPromptBytes,
		report.TotalModelInputBytes,
		report.EstimatedInputTokens,
		report.MaxPromptBytes,
		report.MaxOutputTokens,
		report.PromptContainsTruncation,
		report.ContextFiles,
		report.SelectedSkills,
		report.ToolOutputs,
		report.TranscriptMessages,
		report.BoundedTranscriptMessages,
	)
}

func writeModelUsageSessionCard(b *strings.Builder, report ModelUsageReport) {
	fmt.Fprintf(
		b,
		"- kind=`session-usage` assistant_turn_markers=`%d` model_backed_assistant_turns=`%d` deterministic_assistant_turns=`%d` usage_bearing_assistant_turns=`%d` model_names=`%s` recorded_prompt_tokens=`%d` recorded_completion_tokens=`%d` recorded_total_tokens=`%d` recorded_cache_read_tokens=`%d` recorded_cache_write_tokens=`%d`\n",
		report.AssistantTurnMarkers,
		report.ModelBackedAssistantTurns,
		report.DeterministicAssistantTurns,
		report.UsageBearingAssistantTurns,
		inlineListOrNone(report.ModelNames),
		report.RecordedPromptTokens,
		report.RecordedCompletionTokens,
		report.RecordedTotalTokens,
		report.RecordedCacheReadTokens,
		report.RecordedCacheWriteTokens,
	)
}

func writeModelUsageLatestCard(b *strings.Builder, report ModelUsageReport) {
	if report.LatestUsageModel == "" {
		b.WriteString("- kind=`latest-usage` present=`false`\n")
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`latest-usage` present=`true` model=`%s` prompt_tokens=`%d` completion_tokens=`%d` total_tokens=`%d` cache_read_tokens=`%d` cache_write_tokens=`%d`\n",
		inlineCode(report.LatestUsageModel),
		report.LatestUsagePromptTokens,
		report.LatestUsageCompletionTokens,
		report.LatestUsageTotalTokens,
		report.LatestUsageCacheReadTokens,
		report.LatestUsageCacheWriteTokens,
	)
}

func modelUsageTotals(turns []sessionPromptProvenanceTurn) (promptTokens, completionTokens, totalTokens, cacheReadTokens, cacheWriteTokens int) {
	for _, turn := range turns {
		if !turn.Usage.Present {
			continue
		}
		promptTokens += turn.Usage.PromptTokens
		completionTokens += turn.Usage.CompletionTokens
		totalTokens += turn.Usage.TotalTokens
		cacheReadTokens += turn.Usage.CacheReadTokens
		cacheWriteTokens += turn.Usage.CacheWriteTokens
	}
	return promptTokens, completionTokens, totalTokens, cacheReadTokens, cacheWriteTokens
}

func modelUsageTurnCount(turns []sessionPromptProvenanceTurn) int {
	count := 0
	for _, turn := range turns {
		if turn.Usage.Present {
			count++
		}
	}
	return count
}

func modelUsageFindings(report ModelUsageReport) []ModelUsageFinding {
	findings := []ModelUsageFinding{
		{Severity: "info", Code: "openclaw_token_usage_surface_modeled", Detail: "status and usage concepts are represented without provider body dumps"},
		{Severity: "info", Code: "hermes_api_token_counts_modeled", Detail: "session counters distinguish estimated prompt projection from API-reported usage"},
		{Severity: "info", Code: "github_models_actions_token_boundary_modeled", Detail: "GitHub Actions token support is reported from endpoint configuration"},
		{Severity: "info", Code: "usage_marker_persistence_enabled", Detail: "future model-backed assistant turns can persist normalized usage attributes"},
		{Severity: "info", Code: "raw_usage_payload_excluded", Detail: "raw provider usage and response payloads are not printed"},
		{Severity: "info", Code: "cost_estimation_disabled_until_pricing_config", Detail: "token counts are recorded, but dollar estimates require a reviewed pricing catalog"},
	}
	if report.UsageBearingAssistantTurns == 0 {
		findings = append(findings, ModelUsageFinding{Severity: "warning", Code: "no_usage_markers_seen", Detail: "no previous assistant marker in this session contains provider usage counters yet"})
	}
	if !report.GitHubModelsEndpoint {
		findings = append(findings, ModelUsageFinding{Severity: "warning", Code: "non_github_models_endpoint", Detail: "usage semantics may vary for non-GitHub OpenAI-compatible endpoints"})
	}
	if strings.TrimSpace(os.Getenv("GITCLAW_LLM_BASE_URL")) != "" {
		findings = append(findings, ModelUsageFinding{Severity: "info", Code: "custom_model_endpoint_from_environment", Detail: "the endpoint host came from GITCLAW_LLM_BASE_URL"})
	}
	return findings
}

func writeModelUsageFindings(b *strings.Builder, findings []ModelUsageFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Detail)
	}
}
