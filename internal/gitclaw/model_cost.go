package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

const (
	githubModelsCostSourceURL    = "https://docs.github.com/en/billing/reference/costs-for-github-models"
	githubModelsCostSnapshotDate = "2026-05-31"
	githubModelsTokenUnitPrice   = 0.00001
)

type ModelCostCatalogEntry struct {
	ModelName             string
	ModelIDs              []string
	InputMultiplier       float64
	CachedInputMultiplier float64
	HasCachedInput        bool
	OutputMultiplier      float64
}

type ModelCostUsageLine struct {
	Source              string
	Model               string
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	CacheReadTokens     int
	CacheWriteTokens    int
	CatalogEntryPresent bool
	CatalogModelName    string
	TokenUnits          float64
	EstimatedUSD        float64
}

type ModelCostReport struct {
	Status                               string
	VerificationScope                    string
	Provider                             string
	Model                                string
	FallbackModels                       []string
	EndpointHost                         string
	TokenSource                          string
	PricingSource                        string
	PricingSourceURL                     string
	PricingSnapshotDate                  string
	TokenUnitPriceUSD                    float64
	CatalogEntries                       int
	CurrentModelCatalogEntryPresent      bool
	CurrentModelCatalogName              string
	CurrentModelInputMultiplier          float64
	CurrentModelCachedInputMultiplier    float64
	CurrentModelCachedInputMultiplierSet bool
	CurrentModelOutputMultiplier         float64
	CurrentModelCostEstimationSupported  bool
	ProjectedInputTokens                 int
	ProjectedOutputTokens                int
	ProjectedTokenUnits                  float64
	ProjectedUSD                         float64
	RecordedUsageCostEstimationSupported bool
	AssistantTurnMarkers                 int
	ModelBackedAssistantTurns            int
	DeterministicAssistantTurns          int
	UsageBearingAssistantTurns           int
	CostedUsageTurns                     int
	UncostedUsageTurns                   int
	ModelNames                           []string
	UncostedModelNames                   []string
	RecordedPromptTokens                 int
	RecordedCompletionTokens             int
	RecordedTotalTokens                  int
	RecordedCacheReadTokens              int
	RecordedCacheWriteTokens             int
	RecordedTokenUnits                   float64
	RecordedEstimatedUSD                 float64
	BillingAPIProbePerformed             bool
	LiveInferenceProbePerformed          bool
	BillingAccountStateKnown             bool
	PaidUsageOptInStateKnown             bool
	GitHubBudgetStateKnown               bool
	RawProviderUsageIncluded             bool
	RawProviderResponseIncluded          bool
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	RawPromptBodiesIncluded              bool
	RawToolOutputsIncluded               bool
	CredentialValuesIncluded             bool
	LLME2ERequiredAfterModelCostChange   bool
	UsageLines                           []ModelCostUsageLine
	Findings                             []ModelCostFinding
}

type ModelCostFinding struct {
	Severity string
	Code     string
	Detail   string
}

var githubModelsCostCatalog = []ModelCostCatalogEntry{
	{ModelName: "OpenAI GPT-4o", ModelIDs: []string{"openai/gpt-4o", "gpt-4o"}, InputMultiplier: 0.25, CachedInputMultiplier: 0.125, HasCachedInput: true, OutputMultiplier: 1.0},
	{ModelName: "OpenAI GPT-4o mini", ModelIDs: []string{"openai/gpt-4o-mini", "gpt-4o-mini"}, InputMultiplier: 0.015, CachedInputMultiplier: 0.0075, HasCachedInput: true, OutputMultiplier: 0.06},
	{ModelName: "OpenAI GPT-4.1-mini", ModelIDs: []string{"openai/gpt-4.1-mini", "gpt-4.1-mini"}, InputMultiplier: 0.04, CachedInputMultiplier: 0.01, HasCachedInput: true, OutputMultiplier: 0.16},
	{ModelName: "OpenAI GPT-4.1", ModelIDs: []string{"openai/gpt-4.1", "gpt-4.1"}, InputMultiplier: 0.2, CachedInputMultiplier: 0.05, HasCachedInput: true, OutputMultiplier: 0.8},
	{ModelName: "Phi-4", ModelIDs: []string{"phi-4"}, InputMultiplier: 0.0125, OutputMultiplier: 0.05},
	{ModelName: "Phi-4-mini-instruct", ModelIDs: []string{"phi-4-mini-instruct"}, InputMultiplier: 0.0075, OutputMultiplier: 0.03},
	{ModelName: "Phi-4-multimodal-instruct", ModelIDs: []string{"phi-4-multimodal-instruct"}, InputMultiplier: 0.008, OutputMultiplier: 0.032},
	{ModelName: "DeepSeek-R1", ModelIDs: []string{"deepseek-r1"}, InputMultiplier: 0.135, OutputMultiplier: 0.54},
	{ModelName: "DeepSeek-R1-0528", ModelIDs: []string{"deepseek-r1-0528"}, InputMultiplier: 0.135, OutputMultiplier: 0.54},
	{ModelName: "DeepSeek-V3-0324", ModelIDs: []string{"deepseek-v3-0324"}, InputMultiplier: 0.114, OutputMultiplier: 0.456},
	{ModelName: "MAI-DS-R1", ModelIDs: []string{"mai-ds-r1"}, InputMultiplier: 0.135, OutputMultiplier: 0.54},
	{ModelName: "Grok 3 Mini", ModelIDs: []string{"grok-3-mini", "xai/grok-3-mini"}, InputMultiplier: 0.025, OutputMultiplier: 0.127},
	{ModelName: "Grok 3", ModelIDs: []string{"grok-3", "xai/grok-3"}, InputMultiplier: 0.3, OutputMultiplier: 1.5},
	{ModelName: "Llama 4 Maverick 17B Instruct FP8", ModelIDs: []string{"llama-4-maverick-17b-instruct-fp8", "meta/llama-4-maverick-17b-instruct-fp8"}, InputMultiplier: 0.025, OutputMultiplier: 0.1},
	{ModelName: "Llama-3.3-70B-Instruct", ModelIDs: []string{"llama-3.3-70b-instruct", "meta/llama-3.3-70b-instruct"}, InputMultiplier: 0.071, OutputMultiplier: 0.071},
}

func IsModelCostRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/model" && fields[0] != "/models" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "cost", "costs", "spend", "billing", "bill", "budget":
		return true
	default:
		return false
	}
}

func RenderModelCostReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderModelCostReport(ev, cfg, comments, transcript, repoContext, true)
}

func RenderModelCostCLIReport(cfg Config, repoContext RepoContext) string {
	return renderModelCostReport(Event{}, cfg, nil, nil, repoContext, false)
}

func renderModelCostReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildModelCostReport(ev, cfg, comments, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Model Cost Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeModelCostSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps normalized assistant-marker usage onto GitHub Models' direct-use token-unit billing model. It estimates costs only for models present in GitClaw's reviewed GitHub Models multiplier snapshot. It does not perform a live inference probe, query billing APIs, inspect account budgets, or print provider responses, prompts, issue bodies, comments, tool outputs, credentials, or secret values.\n\n")

	b.WriteString("### Cost Cards\n")
	writeModelCostCurrentCard(&b, report)
	writeModelCostRecordedCard(&b, report)
	writeModelCostUsageLines(&b, report.UsageLines)

	b.WriteString("\n### Findings\n")
	writeModelCostFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildModelCostReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext) ModelCostReport {
	baseURL := llmBaseURL(cfg)
	pack := BuildPromptPackReport(ev, cfg, transcript, repoContext)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	currentEntry, currentKnown := lookupModelCostCatalogEntry(cfg.Model)
	report := ModelCostReport{
		Status:                               "ok",
		VerificationScope:                    "github_models_direct_cost_catalog",
		Provider:                             llmProviderForReport(cfg, baseURL),
		Model:                                cfg.Model,
		FallbackModels:                       normalizeModelFallbacks(cfg.ModelFallbacks),
		EndpointHost:                         llmEndpointHost(baseURL),
		TokenSource:                          llmTokenSource(baseURL),
		PricingSource:                        "github_models_direct_costs_snapshot",
		PricingSourceURL:                     githubModelsCostSourceURL,
		PricingSnapshotDate:                  githubModelsCostSnapshotDate,
		TokenUnitPriceUSD:                    githubModelsTokenUnitPrice,
		CatalogEntries:                       len(githubModelsCostCatalog),
		CurrentModelCatalogEntryPresent:      currentKnown,
		CurrentModelCatalogName:              currentEntry.ModelName,
		CurrentModelInputMultiplier:          currentEntry.InputMultiplier,
		CurrentModelCachedInputMultiplier:    currentEntry.CachedInputMultiplier,
		CurrentModelCachedInputMultiplierSet: currentEntry.HasCachedInput,
		CurrentModelOutputMultiplier:         currentEntry.OutputMultiplier,
		CurrentModelCostEstimationSupported:  currentKnown,
		ProjectedInputTokens:                 pack.EstimatedInputTokens,
		ProjectedOutputTokens:                cfg.MaxOutputTokens,
		AssistantTurnMarkers:                 countSessionMarkers(comments).AssistantTurns,
		ModelBackedAssistantTurns:            modelBackedTurns,
		DeterministicAssistantTurns:          deterministicTurns,
		ModelNames:                           modelNames,
		BillingAPIProbePerformed:             false,
		LiveInferenceProbePerformed:          false,
		BillingAccountStateKnown:             false,
		PaidUsageOptInStateKnown:             false,
		GitHubBudgetStateKnown:               false,
		RawProviderUsageIncluded:             false,
		RawProviderResponseIncluded:          false,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		RawPromptBodiesIncluded:              false,
		RawToolOutputsIncluded:               false,
		CredentialValuesIncluded:             false,
		LLME2ERequiredAfterModelCostChange:   true,
	}
	if currentKnown {
		report.ProjectedTokenUnits = modelCostTokenUnits(LLMUsage{Present: true, PromptTokens: report.ProjectedInputTokens, CompletionTokens: report.ProjectedOutputTokens}, currentEntry)
		report.ProjectedUSD = report.ProjectedTokenUnits * githubModelsTokenUnitPrice
	}
	report.UsageLines = modelCostUsageLines(provenance.Turns)
	for _, line := range report.UsageLines {
		report.UsageBearingAssistantTurns++
		report.RecordedPromptTokens += line.PromptTokens
		report.RecordedCompletionTokens += line.CompletionTokens
		report.RecordedTotalTokens += line.TotalTokens
		report.RecordedCacheReadTokens += line.CacheReadTokens
		report.RecordedCacheWriteTokens += line.CacheWriteTokens
		if line.CatalogEntryPresent {
			report.CostedUsageTurns++
			report.RecordedTokenUnits += line.TokenUnits
			report.RecordedEstimatedUSD += line.EstimatedUSD
		} else {
			report.UncostedUsageTurns++
		}
	}
	report.RecordedUsageCostEstimationSupported = report.CostedUsageTurns > 0 && report.UncostedUsageTurns == 0
	report.UncostedModelNames = modelCostUncostedModelNames(report.UsageLines)
	report.Findings = modelCostFindings(report)
	for _, finding := range report.Findings {
		if finding.Severity == "warning" {
			report.Status = "warn"
			break
		}
	}
	return report
}

func writeModelCostSummary(b *strings.Builder, report ModelCostReport) {
	fmt.Fprintf(b, "- model_cost_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- fallback_models: `%s`\n", inlineListOrNone(report.FallbackModels))
	fmt.Fprintf(b, "- endpoint_host: `%s`\n", report.EndpointHost)
	fmt.Fprintf(b, "- token_source: `%s`\n", report.TokenSource)
	fmt.Fprintf(b, "- pricing_source: `%s`\n", report.PricingSource)
	fmt.Fprintf(b, "- pricing_source_url: `%s`\n", report.PricingSourceURL)
	fmt.Fprintf(b, "- pricing_snapshot_date: `%s`\n", report.PricingSnapshotDate)
	fmt.Fprintf(b, "- token_unit_price_usd: `%s`\n", modelCostDecimal(report.TokenUnitPriceUSD))
	fmt.Fprintf(b, "- catalog_entries: `%d`\n", report.CatalogEntries)
	fmt.Fprintf(b, "- current_model_catalog_entry_present: `%t`\n", report.CurrentModelCatalogEntryPresent)
	fmt.Fprintf(b, "- current_model_catalog_name: `%s`\n", inlineCode(report.CurrentModelCatalogName))
	fmt.Fprintf(b, "- current_model_input_multiplier: `%s`\n", modelCostDecimal(report.CurrentModelInputMultiplier))
	fmt.Fprintf(b, "- current_model_cached_input_multiplier: `%s`\n", modelCostDecimal(report.CurrentModelCachedInputMultiplier))
	fmt.Fprintf(b, "- current_model_cached_input_multiplier_set: `%t`\n", report.CurrentModelCachedInputMultiplierSet)
	fmt.Fprintf(b, "- current_model_output_multiplier: `%s`\n", modelCostDecimal(report.CurrentModelOutputMultiplier))
	fmt.Fprintf(b, "- current_model_cost_estimation_supported: `%t`\n", report.CurrentModelCostEstimationSupported)
	fmt.Fprintf(b, "- projected_input_tokens: `%d`\n", report.ProjectedInputTokens)
	fmt.Fprintf(b, "- projected_output_tokens: `%d`\n", report.ProjectedOutputTokens)
	fmt.Fprintf(b, "- projected_token_units: `%s`\n", modelCostDecimal(report.ProjectedTokenUnits))
	fmt.Fprintf(b, "- projected_usd: `%s`\n", modelCostMaybeUSD(report.ProjectedUSD, report.CurrentModelCostEstimationSupported))
	fmt.Fprintf(b, "- recorded_usage_cost_estimation_supported: `%t`\n", report.RecordedUsageCostEstimationSupported)
	fmt.Fprintf(b, "- assistant_turn_markers: `%d`\n", report.AssistantTurnMarkers)
	fmt.Fprintf(b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(b, "- costed_usage_turns: `%d`\n", report.CostedUsageTurns)
	fmt.Fprintf(b, "- uncosted_usage_turns: `%d`\n", report.UncostedUsageTurns)
	fmt.Fprintf(b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(b, "- uncosted_model_names: `%s`\n", inlineListOrNone(report.UncostedModelNames))
	fmt.Fprintf(b, "- recorded_prompt_tokens: `%d`\n", report.RecordedPromptTokens)
	fmt.Fprintf(b, "- recorded_completion_tokens: `%d`\n", report.RecordedCompletionTokens)
	fmt.Fprintf(b, "- recorded_total_tokens: `%d`\n", report.RecordedTotalTokens)
	fmt.Fprintf(b, "- recorded_cache_read_tokens: `%d`\n", report.RecordedCacheReadTokens)
	fmt.Fprintf(b, "- recorded_cache_write_tokens: `%d`\n", report.RecordedCacheWriteTokens)
	fmt.Fprintf(b, "- recorded_token_units: `%s`\n", modelCostDecimal(report.RecordedTokenUnits))
	fmt.Fprintf(b, "- recorded_estimated_usd: `%s`\n", modelCostMaybeUSD(report.RecordedEstimatedUSD, report.CostedUsageTurns > 0))
	fmt.Fprintf(b, "- billing_api_probe_performed: `%t`\n", report.BillingAPIProbePerformed)
	fmt.Fprintf(b, "- live_inference_probe_performed: `%t`\n", report.LiveInferenceProbePerformed)
	fmt.Fprintf(b, "- billing_account_state_known: `%t`\n", report.BillingAccountStateKnown)
	fmt.Fprintf(b, "- paid_usage_opt_in_state_known: `%t`\n", report.PaidUsageOptInStateKnown)
	fmt.Fprintf(b, "- github_budget_state_known: `%t`\n", report.GitHubBudgetStateKnown)
	fmt.Fprintf(b, "- raw_provider_usage_included: `%t`\n", report.RawProviderUsageIncluded)
	fmt.Fprintf(b, "- raw_provider_response_included: `%t`\n", report.RawProviderResponseIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_model_cost_change: `%t`\n", report.LLME2ERequiredAfterModelCostChange)
}

func writeModelCostCurrentCard(b *strings.Builder, report ModelCostReport) {
	fmt.Fprintf(
		b,
		"- kind=`current-model-cost` model=`%s` catalog_entry_present=`%t` catalog_model_name=`%s` input_multiplier=`%s` cached_input_multiplier=`%s` cached_input_multiplier_set=`%t` output_multiplier=`%s` projected_input_tokens=`%d` projected_output_tokens=`%d` projected_token_units=`%s` projected_usd=`%s`\n",
		inlineCode(report.Model),
		report.CurrentModelCatalogEntryPresent,
		inlineCode(report.CurrentModelCatalogName),
		modelCostDecimal(report.CurrentModelInputMultiplier),
		modelCostDecimal(report.CurrentModelCachedInputMultiplier),
		report.CurrentModelCachedInputMultiplierSet,
		modelCostDecimal(report.CurrentModelOutputMultiplier),
		report.ProjectedInputTokens,
		report.ProjectedOutputTokens,
		modelCostDecimal(report.ProjectedTokenUnits),
		modelCostMaybeUSD(report.ProjectedUSD, report.CurrentModelCostEstimationSupported),
	)
}

func writeModelCostRecordedCard(b *strings.Builder, report ModelCostReport) {
	fmt.Fprintf(
		b,
		"- kind=`recorded-usage-cost` usage_bearing_assistant_turns=`%d` costed_usage_turns=`%d` uncosted_usage_turns=`%d` recorded_prompt_tokens=`%d` recorded_completion_tokens=`%d` recorded_total_tokens=`%d` recorded_cache_read_tokens=`%d` recorded_token_units=`%s` recorded_estimated_usd=`%s` uncosted_model_names=`%s`\n",
		report.UsageBearingAssistantTurns,
		report.CostedUsageTurns,
		report.UncostedUsageTurns,
		report.RecordedPromptTokens,
		report.RecordedCompletionTokens,
		report.RecordedTotalTokens,
		report.RecordedCacheReadTokens,
		modelCostDecimal(report.RecordedTokenUnits),
		modelCostMaybeUSD(report.RecordedEstimatedUSD, report.CostedUsageTurns > 0),
		inlineListOrNone(report.UncostedModelNames),
	)
}

func writeModelCostUsageLines(b *strings.Builder, lines []ModelCostUsageLine) {
	b.WriteString("\n### Usage Cost Lines\n")
	if len(lines) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, line := range lines {
		fmt.Fprintf(
			b,
			"- source=`%s` model=`%s` prompt_tokens=`%d` completion_tokens=`%d` total_tokens=`%d` cache_read_tokens=`%d` cache_write_tokens=`%d` catalog_entry_present=`%t` catalog_model_name=`%s` token_units=`%s` estimated_usd=`%s`\n",
			line.Source,
			inlineCode(line.Model),
			line.PromptTokens,
			line.CompletionTokens,
			line.TotalTokens,
			line.CacheReadTokens,
			line.CacheWriteTokens,
			line.CatalogEntryPresent,
			inlineCode(line.CatalogModelName),
			modelCostDecimal(line.TokenUnits),
			modelCostMaybeUSD(line.EstimatedUSD, line.CatalogEntryPresent),
		)
	}
}

func modelCostUsageLines(turns []sessionPromptProvenanceTurn) []ModelCostUsageLine {
	lines := make([]ModelCostUsageLine, 0, len(turns))
	for _, turn := range turns {
		if !turn.Usage.Present {
			continue
		}
		entry, ok := lookupModelCostCatalogEntry(turn.Model)
		line := ModelCostUsageLine{
			Source:              turn.Source,
			Model:               turn.Model,
			PromptTokens:        turn.Usage.PromptTokens,
			CompletionTokens:    turn.Usage.CompletionTokens,
			TotalTokens:         turn.Usage.TotalTokens,
			CacheReadTokens:     turn.Usage.CacheReadTokens,
			CacheWriteTokens:    turn.Usage.CacheWriteTokens,
			CatalogEntryPresent: ok,
			CatalogModelName:    entry.ModelName,
		}
		if ok {
			line.TokenUnits = modelCostTokenUnits(turn.Usage, entry)
			line.EstimatedUSD = line.TokenUnits * githubModelsTokenUnitPrice
		}
		lines = append(lines, line)
	}
	return lines
}

func modelCostTokenUnits(usage LLMUsage, entry ModelCostCatalogEntry) float64 {
	cacheRead := usage.CacheReadTokens
	if cacheRead > usage.PromptTokens {
		cacheRead = usage.PromptTokens
	}
	uncachedInput := usage.PromptTokens - cacheRead
	cachedMultiplier := entry.InputMultiplier
	if entry.HasCachedInput {
		cachedMultiplier = entry.CachedInputMultiplier
	}
	return float64(uncachedInput)*entry.InputMultiplier + float64(cacheRead)*cachedMultiplier + float64(usage.CompletionTokens)*entry.OutputMultiplier
}

func lookupModelCostCatalogEntry(model string) (ModelCostCatalogEntry, bool) {
	key := normalizeModelCostID(model)
	if key == "" {
		return ModelCostCatalogEntry{}, false
	}
	for _, entry := range githubModelsCostCatalog {
		for _, id := range entry.ModelIDs {
			if normalizeModelCostID(id) == key {
				return entry, true
			}
		}
	}
	return ModelCostCatalogEntry{}, false
}

func normalizeModelCostID(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

func modelCostUncostedModelNames(lines []ModelCostUsageLine) []string {
	seen := map[string]bool{}
	var names []string
	for _, line := range lines {
		model := strings.TrimSpace(line.Model)
		if model == "" || line.CatalogEntryPresent || seen[model] {
			continue
		}
		seen[model] = true
		names = append(names, model)
	}
	sort.Strings(names)
	return names
}

func modelCostFindings(report ModelCostReport) []ModelCostFinding {
	findings := []ModelCostFinding{
		{Severity: "info", Code: "github_models_token_unit_pricing_modeled", Detail: "direct GitHub Models pricing uses token units, fixed unit price, and model multipliers"},
		{Severity: "info", Code: "openclaw_usage_cost_surface_modeled", Detail: "token usage and cost diagnostics are separated from raw prompts and transcripts"},
		{Severity: "info", Code: "hermes_api_token_count_boundary_modeled", Detail: "only provider-reported usage markers are treated as recorded token evidence"},
		{Severity: "info", Code: "billing_api_not_queried", Detail: "account billing, paid opt-in, and budget state are not queried by this report"},
	}
	if !report.CurrentModelCatalogEntryPresent {
		findings = append(findings, ModelCostFinding{Severity: "warning", Code: "current_model_multiplier_unknown", Detail: "current model is not present in the reviewed GitHub Models multiplier snapshot"})
	}
	if report.UsageBearingAssistantTurns == 0 {
		findings = append(findings, ModelCostFinding{Severity: "warning", Code: "no_usage_markers_seen", Detail: "no previous assistant marker in this session contains provider usage counters"})
	}
	if report.UncostedUsageTurns > 0 {
		findings = append(findings, ModelCostFinding{Severity: "warning", Code: "uncosted_usage_markers_seen", Detail: "some usage-bearing assistant turns used models without reviewed cost multipliers"})
	}
	if report.CostedUsageTurns > 0 {
		findings = append(findings, ModelCostFinding{Severity: "info", Code: "recorded_usage_cost_estimated", Detail: "at least one prior usage marker was converted to GitHub Models token units"})
	}
	return findings
}

func writeModelCostFindings(b *strings.Builder, findings []ModelCostFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Detail)
	}
}

func modelCostMaybeUSD(value float64, available bool) string {
	if !available {
		return "unavailable"
	}
	return modelCostDecimal(value)
}

func modelCostDecimal(value float64) string {
	if value == 0 {
		return "0"
	}
	out := fmt.Sprintf("%.6f", value)
	out = strings.TrimRight(out, "0")
	out = strings.TrimRight(out, ".")
	if out == "" || out == "-0" {
		return "0"
	}
	return out
}
