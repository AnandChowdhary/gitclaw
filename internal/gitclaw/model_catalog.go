package gitclaw

import (
	"fmt"
	"strings"
)

const (
	githubModelsCatalogAPIVersion = "2026-03-10"
	modelCatalogSnapshotDate      = "2026-06-01"
	modelCatalogSourceURL         = "https://docs.github.com/en/rest/models/catalog"
	modelInferenceSourceURL       = "https://docs.github.com/en/rest/models/inference"
	defaultGitHubOpenAIModel      = "openai/gpt-5-nano"
)

type ModelCatalogEntry struct {
	ID               string
	Name             string
	Publisher        string
	Family           string
	SizeClass        string
	Version          string
	MaxInputTokens   int
	MaxOutputTokens  int
	DefaultCandidate bool
}

type ModelCatalogReport struct {
	Status                                 string
	Provider                               string
	Model                                  string
	FallbackModels                         []string
	DefaultModelPolicy                     string
	CatalogSource                          string
	CatalogSourceURL                       string
	InferenceSourceURL                     string
	CatalogAPIVersion                      string
	CatalogEndpointHost                    string
	EndpointHost                           string
	TokenSource                            string
	SnapshotDate                           string
	ReviewedCatalogEntries                 int
	ReviewedOpenAIEntries                  int
	ReviewedGPT5Entries                    int
	ConfiguredModelCatalogEntryPresent     bool
	FallbackModelsConfigured               int
	FallbackModelsCatalogEntries           int
	DefaultCandidate                       string
	DefaultCandidateCatalogEntryPresent    bool
	ConfiguredModelMatchesDefaultCandidate bool
	GPT54MiniCatalogEntryPresent           bool
	NewerSmallModelCandidatePresent        bool
	ModelCatalogProbePerformed             bool
	LiveInferenceProbePerformed            bool
	RawCatalogResponseIncluded             bool
	RawProviderResponseIncluded            bool
	RawIssueBodiesIncluded                 bool
	RawCommentBodiesIncluded               bool
	RawPromptBodiesIncluded                bool
	CredentialValuesIncluded               bool
	LLME2ERequiredAfterModelCatalogChange  bool
	Cards                                  []ModelCatalogEntry
	Findings                               []ModelCatalogFinding
}

type ModelCatalogFinding struct {
	Severity string
	Code     string
	Detail   string
}

var githubModelsReviewedCatalog = []ModelCatalogEntry{
	{ID: "openai/gpt-5-nano", Name: "OpenAI GPT-5 nano", Publisher: "OpenAI", Family: "gpt-5", SizeClass: "nano", Version: "reviewed", MaxInputTokens: 400000, MaxOutputTokens: 128000, DefaultCandidate: true},
	{ID: "openai/gpt-5-mini", Name: "OpenAI GPT-5 mini", Publisher: "OpenAI", Family: "gpt-5", SizeClass: "mini", Version: "reviewed", MaxInputTokens: 400000, MaxOutputTokens: 128000},
	{ID: "openai/gpt-5", Name: "OpenAI GPT-5", Publisher: "OpenAI", Family: "gpt-5", SizeClass: "full", Version: "reviewed", MaxInputTokens: 400000, MaxOutputTokens: 128000},
	{ID: "openai/gpt-5-chat", Name: "OpenAI GPT-5 Chat", Publisher: "OpenAI", Family: "gpt-5", SizeClass: "chat", Version: "reviewed", MaxInputTokens: 400000, MaxOutputTokens: 128000},
	{ID: "openai/gpt-4.1-nano", Name: "OpenAI GPT-4.1 nano", Publisher: "OpenAI", Family: "gpt-4.1", SizeClass: "nano", Version: "2025-04-14", MaxInputTokens: 1048576, MaxOutputTokens: 32768},
	{ID: "openai/gpt-4.1-mini", Name: "OpenAI GPT-4.1 mini", Publisher: "OpenAI", Family: "gpt-4.1", SizeClass: "mini", Version: "2025-04-14", MaxInputTokens: 1048576, MaxOutputTokens: 32768},
	{ID: "openai/gpt-4.1", Name: "OpenAI GPT-4.1", Publisher: "OpenAI", Family: "gpt-4.1", SizeClass: "full", Version: "2025-04-14", MaxInputTokens: 1048576, MaxOutputTokens: 32768},
	{ID: "openai/gpt-4o-mini", Name: "OpenAI GPT-4o mini", Publisher: "OpenAI", Family: "gpt-4o", SizeClass: "mini", Version: "reviewed", MaxInputTokens: 128000, MaxOutputTokens: 16384},
	{ID: "openai/gpt-4o", Name: "OpenAI GPT-4o", Publisher: "OpenAI", Family: "gpt-4o", SizeClass: "full", Version: "reviewed", MaxInputTokens: 128000, MaxOutputTokens: 16384},
}

func IsModelCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return isModelCatalogFields(fields)
}

func isModelCatalogFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/model" && fields[0] != "/models" {
		return false
	}
	switch strings.ToLower(strings.Trim(fields[1], " \t\r\n.,:;!?")) {
	case "catalog", "default", "defaults", "selection", "select", "available":
		return true
	default:
		return false
	}
}

func RenderModelCatalogReport(ev Event, cfg Config) string {
	return renderModelCatalogReport(ev, cfg, true)
}

func RenderModelCatalogCLIReport(cfg Config) string {
	return renderModelCatalogReport(Event{}, cfg, false)
}

func renderModelCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildModelCatalogReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Model Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeModelCatalogSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps GitClaw's configured model onto a reviewed GitHub Models catalog snapshot and default-selection policy. It does not call the catalog or inference endpoints. It reports model IDs, names, versions, limits, hashes, and gates only; raw catalog responses, provider responses, issue bodies, comments, prompts, credentials, and secret values are not included.\n\n")

	b.WriteString("### Catalog Cards\n")
	writeModelCatalogCards(&b, report.Cards)

	b.WriteString("\n### Catalog Gates\n")
	fmt.Fprintf(&b, "- configured_model_gate=`%s`\n", modelCatalogConfiguredGate(report))
	fmt.Fprintf(&b, "- fallback_model_gate=`%s`\n", modelCatalogFallbackGate(report))
	fmt.Fprintf(&b, "- default_candidate_gate=`%s`\n", modelCatalogDefaultGate(report))
	fmt.Fprintf(&b, "- gpt_5_4_mini_gate=`%s`\n", modelCatalogGPT54Gate(report))
	fmt.Fprintf(&b, "- live_probe_gate=`disabled-for-deterministic-report`\n")
	fmt.Fprintf(&b, "- raw_body_gate=`ids-metadata-and-hashes-only`\n")

	b.WriteString("\n### Findings\n")
	writeModelCatalogFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildModelCatalogReport(cfg Config) ModelCatalogReport {
	baseURL := llmBaseURL(cfg)
	entries := append([]ModelCatalogEntry(nil), githubModelsReviewedCatalog...)
	fallbacks := normalizeModelFallbacks(cfg.ModelFallbacks)
	report := ModelCatalogReport{
		Status:                                 "ok",
		Provider:                               llmProviderForReport(cfg, baseURL),
		Model:                                  strings.TrimSpace(cfg.Model),
		FallbackModels:                         fallbacks,
		DefaultModelPolicy:                     "smallest-openai-gpt5-github-models-catalog-model",
		CatalogSource:                          "reviewed-github-models-catalog-snapshot",
		CatalogSourceURL:                       modelCatalogSourceURL,
		InferenceSourceURL:                     modelInferenceSourceURL,
		CatalogAPIVersion:                      githubModelsCatalogAPIVersion,
		CatalogEndpointHost:                    llmEndpointHost(defaultGitHubModelsCatalogURL),
		EndpointHost:                           llmEndpointHost(baseURL),
		TokenSource:                            llmTokenSource(baseURL),
		SnapshotDate:                           modelCatalogSnapshotDate,
		ReviewedCatalogEntries:                 len(entries),
		ReviewedOpenAIEntries:                  modelCatalogOpenAIEntries(entries),
		ReviewedGPT5Entries:                    modelCatalogFamilyEntries(entries, "gpt-5"),
		ConfiguredModelCatalogEntryPresent:     lookupReviewedModelCatalogEntry(cfg.Model).ID != "",
		FallbackModelsConfigured:               len(fallbacks),
		DefaultCandidate:                       defaultGitHubOpenAIModel,
		DefaultCandidateCatalogEntryPresent:    lookupReviewedModelCatalogEntry(defaultGitHubOpenAIModel).ID != "",
		ConfiguredModelMatchesDefaultCandidate: strings.EqualFold(strings.TrimSpace(cfg.Model), defaultGitHubOpenAIModel),
		GPT54MiniCatalogEntryPresent:           lookupReviewedModelCatalogEntry("openai/gpt-5.4-mini").ID != "",
		NewerSmallModelCandidatePresent:        false,
		ModelCatalogProbePerformed:             false,
		LiveInferenceProbePerformed:            false,
		RawCatalogResponseIncluded:             false,
		RawProviderResponseIncluded:            false,
		RawIssueBodiesIncluded:                 false,
		RawCommentBodiesIncluded:               false,
		RawPromptBodiesIncluded:                false,
		CredentialValuesIncluded:               false,
		LLME2ERequiredAfterModelCatalogChange:  true,
		Cards:                                  entries,
	}
	for _, fallback := range fallbacks {
		if lookupReviewedModelCatalogEntry(fallback).ID != "" {
			report.FallbackModelsCatalogEntries++
		}
	}
	report.Findings = modelCatalogFindings(report)
	for _, finding := range report.Findings {
		if finding.Severity == "warning" && report.Status == "ok" {
			report.Status = "warn"
		}
		if finding.Severity == "high" {
			report.Status = "high"
		}
	}
	return report
}

func writeModelCatalogSummary(b *strings.Builder, report ModelCatalogReport) {
	fmt.Fprintf(b, "- model_catalog_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- fallback_models: `%s`\n", inlineListOrNone(report.FallbackModels))
	fmt.Fprintf(b, "- default_model_policy: `%s`\n", report.DefaultModelPolicy)
	fmt.Fprintf(b, "- catalog_source: `%s`\n", report.CatalogSource)
	fmt.Fprintf(b, "- catalog_source_url: `%s`\n", report.CatalogSourceURL)
	fmt.Fprintf(b, "- inference_source_url: `%s`\n", report.InferenceSourceURL)
	fmt.Fprintf(b, "- catalog_api_version: `%s`\n", report.CatalogAPIVersion)
	fmt.Fprintf(b, "- catalog_endpoint_host: `%s`\n", report.CatalogEndpointHost)
	fmt.Fprintf(b, "- endpoint_host: `%s`\n", report.EndpointHost)
	fmt.Fprintf(b, "- token_source: `%s`\n", report.TokenSource)
	fmt.Fprintf(b, "- catalog_snapshot_date: `%s`\n", report.SnapshotDate)
	fmt.Fprintf(b, "- reviewed_catalog_entries: `%d`\n", report.ReviewedCatalogEntries)
	fmt.Fprintf(b, "- reviewed_openai_entries: `%d`\n", report.ReviewedOpenAIEntries)
	fmt.Fprintf(b, "- reviewed_gpt5_entries: `%d`\n", report.ReviewedGPT5Entries)
	fmt.Fprintf(b, "- configured_model_catalog_entry_present: `%t`\n", report.ConfiguredModelCatalogEntryPresent)
	fmt.Fprintf(b, "- fallback_models_configured: `%d`\n", report.FallbackModelsConfigured)
	fmt.Fprintf(b, "- fallback_models_catalog_entries: `%d`\n", report.FallbackModelsCatalogEntries)
	fmt.Fprintf(b, "- default_candidate: `%s`\n", report.DefaultCandidate)
	fmt.Fprintf(b, "- default_candidate_catalog_entry_present: `%t`\n", report.DefaultCandidateCatalogEntryPresent)
	fmt.Fprintf(b, "- configured_model_matches_default_candidate: `%t`\n", report.ConfiguredModelMatchesDefaultCandidate)
	fmt.Fprintf(b, "- gpt_5_4_mini_catalog_entry_present: `%t`\n", report.GPT54MiniCatalogEntryPresent)
	fmt.Fprintf(b, "- newer_small_model_candidate_present: `%t`\n", report.NewerSmallModelCandidatePresent)
	fmt.Fprintf(b, "- model_catalog_probe_performed: `%t`\n", report.ModelCatalogProbePerformed)
	fmt.Fprintf(b, "- live_inference_probe_performed: `%t`\n", report.LiveInferenceProbePerformed)
	fmt.Fprintf(b, "- raw_catalog_response_included: `%t`\n", report.RawCatalogResponseIncluded)
	fmt.Fprintf(b, "- raw_provider_response_included: `%t`\n", report.RawProviderResponseIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_model_catalog_change: `%t`\n", report.LLME2ERequiredAfterModelCatalogChange)
}

func writeModelCatalogCards(b *strings.Builder, entries []ModelCatalogEntry) {
	if len(entries) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, entry := range entries {
		fmt.Fprintf(b, "- model_id=`%s` name_sha256_12=`%s` publisher=`%s` family=`%s` size_class=`%s` version=`%s` max_input_tokens=`%d` max_output_tokens=`%d` default_candidate=`%t`\n",
			entry.ID,
			shortDocumentHash(entry.Name),
			entry.Publisher,
			entry.Family,
			entry.SizeClass,
			entry.Version,
			entry.MaxInputTokens,
			entry.MaxOutputTokens,
			entry.DefaultCandidate,
		)
	}
}

func writeModelCatalogFindings(b *strings.Builder, findings []ModelCatalogFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}

func modelCatalogFindings(report ModelCatalogReport) []ModelCatalogFinding {
	var findings []ModelCatalogFinding
	if !report.ConfiguredModelCatalogEntryPresent {
		findings = append(findings, ModelCatalogFinding{Severity: "warning", Code: "configured_model_not_in_reviewed_catalog", Detail: "configured model is not present in the reviewed GitHub Models catalog snapshot"})
	}
	if !report.ConfiguredModelMatchesDefaultCandidate {
		findings = append(findings, ModelCatalogFinding{Severity: "warning", Code: "configured_model_not_default_candidate", Detail: "configured model does not match the reviewed smallest GPT-5-family OpenAI default candidate"})
	}
	if report.FallbackModelsConfigured > report.FallbackModelsCatalogEntries {
		findings = append(findings, ModelCatalogFinding{Severity: "warning", Code: "fallback_model_not_in_reviewed_catalog", Detail: "one or more fallback models are absent from the reviewed GitHub Models catalog snapshot"})
	}
	if !report.GPT54MiniCatalogEntryPresent {
		findings = append(findings, ModelCatalogFinding{Severity: "info", Code: "gpt_5_4_mini_not_in_reviewed_catalog", Detail: "the reviewed snapshot does not include openai/gpt-5.4-mini, so openai/gpt-5-nano remains the configured small default"})
	}
	if !report.ModelCatalogProbePerformed {
		findings = append(findings, ModelCatalogFinding{Severity: "info", Code: "live_catalog_probe_not_performed", Detail: "deterministic reports do not call the GitHub Models catalog endpoint"})
	}
	return findings
}

func lookupReviewedModelCatalogEntry(model string) ModelCatalogEntry {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, entry := range githubModelsReviewedCatalog {
		if strings.EqualFold(entry.ID, model) {
			return entry
		}
	}
	return ModelCatalogEntry{}
}

func modelCatalogOpenAIEntries(entries []ModelCatalogEntry) int {
	count := 0
	for _, entry := range entries {
		if strings.EqualFold(entry.Publisher, "OpenAI") {
			count++
		}
	}
	return count
}

func modelCatalogFamilyEntries(entries []ModelCatalogEntry, family string) int {
	count := 0
	for _, entry := range entries {
		if strings.EqualFold(entry.Family, family) {
			count++
		}
	}
	return count
}

func modelCatalogConfiguredGate(report ModelCatalogReport) string {
	if report.ConfiguredModelCatalogEntryPresent {
		return "pass"
	}
	return "warn"
}

func modelCatalogFallbackGate(report ModelCatalogReport) string {
	if report.FallbackModelsConfigured == 0 {
		return "none"
	}
	if report.FallbackModelsCatalogEntries == report.FallbackModelsConfigured {
		return "pass"
	}
	return "warn"
}

func modelCatalogDefaultGate(report ModelCatalogReport) string {
	if report.ConfiguredModelMatchesDefaultCandidate && report.DefaultCandidateCatalogEntryPresent {
		return "pass"
	}
	return "warn"
}

func modelCatalogGPT54Gate(report ModelCatalogReport) string {
	if report.GPT54MiniCatalogEntryPresent {
		return "review-needed"
	}
	return "not-present"
}
