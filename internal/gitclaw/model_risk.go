package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ModelRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type ModelRiskReport struct {
	Status                                  string
	VerificationScope                       string
	Provider                                string
	Model                                   string
	FallbackModels                          []string
	FallbackModelsConfigured                int
	DefaultModelPolicy                      string
	CatalogEndpointHost                     string
	EndpointHost                            string
	TokenSource                             string
	OutputTokenParameter                    string
	RequestTimeoutSeconds                   int
	RetryMaxAttempts                        int
	RetryBaseDelaySeconds                   int
	RetryMaxDelaySeconds                    int
	RetryableStatuses                       string
	FallbackOnRetryableStatuses             bool
	FallbackPrimaryAttemptsBeforeFallback   int
	PromptArtifactEnabled                   bool
	ConfigFilePresent                       bool
	ConfigFilePath                          string
	GitHubModelsEndpoint                    bool
	OpenAICompatibleEndpoint                bool
	GitHubActionsTokenSupported             bool
	PrimaryModelSmallDefault                bool
	PrimaryModelKnownGitHubCatalogEntry     bool
	FallbackModelsKnownGitHubCatalogEntries int
	ModelCatalogProbePerformed              bool
	LiveInferenceProbePerformed             bool
	SurfacesWithRiskFindings                int
	Findings                                []ModelRiskFinding
	HighRiskFindings                        int
	WarningRiskFindings                     int
	InfoRiskFindings                        int
	RawModelConfigBodiesIncluded            bool
	RawIssueBodiesIncluded                  bool
	RawCommentBodiesIncluded                bool
	RawPromptBodiesIncluded                 bool
	RawProviderErrorBodiesIncluded          bool
	CredentialValuesIncluded                bool
	LLME2ERequiredAfterModelRiskChange      bool
}

type modelRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var modelConfigRiskRules = []modelRiskRule{
	{
		Severity: "high",
		Code:     "credential_material_in_model_config",
		Category: "credential-handling",
		Any: []string{
			"api_key:",
			"api_key=",
			"openai_api_key:",
			"openai_api_key=",
			"github_token:",
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"sk-",
			"xoxb-",
			"xapp-",
		},
		IgnoreAny: []string{
			"not included",
			"not printed",
			"without token",
			"without dumping",
			"placeholder",
			"secret name",
			"${{ secrets.",
		},
	},
	{
		Severity: "high",
		Code:     "raw_prompt_logging_enabled",
		Category: "prompt-leakage",
		Any: []string{
			"dump raw prompt",
			"log raw prompt",
			"print raw prompt",
			"upload raw prompt",
			"include full prompt",
		},
		IgnoreAny: []string{
			"do not",
			"not dump",
			"not printed",
			"without dumping",
			"redact",
		},
	},
	{
		Severity: "warning",
		Code:     "live_model_probe_required",
		Category: "quota-control",
		Any: []string{
			"probe model on every report",
			"live model probe required",
			"always call model during status",
			"always call model during report",
		},
		IgnoreAny: []string{
			"not required",
			"does not",
			"without calling",
		},
	},
	{
		Severity: "warning",
		Code:     "raw_provider_error_logging_enabled",
		Category: "provider-error-leakage",
		Any: []string{
			"dump provider error body",
			"log provider error body",
			"print provider error body",
			"include raw provider error",
		},
		IgnoreAny: []string{
			"not included",
			"not printed",
			"safe failure",
			"without dumping",
		},
	},
}

func BuildModelRiskReport(cfg Config) ModelRiskReport {
	baseURL := llmBaseURL(cfg)
	provider := llmProviderForReport(cfg, baseURL)
	configSurface := inspectConfigSurface(cfg.Workdir)
	report := ModelRiskReport{
		Status:                                "ok",
		VerificationScope:                     "github_models_control_plane",
		Provider:                              provider,
		Model:                                 cfg.Model,
		FallbackModels:                        normalizeModelFallbacks(cfg.ModelFallbacks),
		FallbackModelsConfigured:              len(normalizeModelFallbacks(cfg.ModelFallbacks)),
		DefaultModelPolicy:                    "smallest-openai-github-models-catalog-model",
		CatalogEndpointHost:                   llmEndpointHost(defaultGitHubModelsCatalogURL),
		EndpointHost:                          llmEndpointHost(baseURL),
		TokenSource:                           llmTokenSource(baseURL),
		OutputTokenParameter:                  llmOutputTokenParam(cfg.Model),
		RequestTimeoutSeconds:                 int(llmTimeout().Seconds()),
		RetryMaxAttempts:                      llmMaxAttempts(),
		RetryBaseDelaySeconds:                 int(llmRetryBaseDelay().Seconds()),
		RetryMaxDelaySeconds:                  int(llmRetryMaxDelay().Seconds()),
		RetryableStatuses:                     "429, 408, 5xx",
		FallbackOnRetryableStatuses:           len(normalizeModelFallbacks(cfg.ModelFallbacks)) > 0,
		FallbackPrimaryAttemptsBeforeFallback: llmPrimaryAttemptsBeforeFallback(llmMaxAttempts()),
		PromptArtifactEnabled:                 strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "",
		ConfigFilePresent:                     configSurface.ConfigFile.Present,
		ConfigFilePath:                        gitclawConfigPath,
		GitHubModelsEndpoint:                  strings.EqualFold(llmEndpointHost(baseURL), "models.github.ai"),
		OpenAICompatibleEndpoint:              true,
		GitHubActionsTokenSupported:           strings.Contains(baseURL, "models.github.ai"),
		PrimaryModelSmallDefault:              strings.EqualFold(strings.TrimSpace(cfg.Model), "openai/gpt-5-nano"),
		PrimaryModelKnownGitHubCatalogEntry:   isKnownGitHubModelsModel(cfg.Model),
		ModelCatalogProbePerformed:            false,
		LiveInferenceProbePerformed:           false,
		RawModelConfigBodiesIncluded:          false,
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		RawPromptBodiesIncluded:               false,
		RawProviderErrorBodiesIncluded:        false,
		CredentialValuesIncluded:              false,
		LLME2ERequiredAfterModelRiskChange:    true,
	}
	for _, fallback := range report.FallbackModels {
		if isKnownGitHubModelsModel(fallback) {
			report.FallbackModelsKnownGitHubCatalogEntries++
		}
	}
	report.Findings = append(report.Findings, modelMetadataRiskFindings(report, baseURL, cfg)...)
	if configSurface.ConfigFile.Present {
		report.Findings = append(report.Findings, scanModelConfigRiskText(configSurface.ConfigFile.Path, readModelRiskBody(cfg.Workdir, configSurface.ConfigFile.Path))...)
	}
	sortModelRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = modelRiskSurfaceCount(report.Findings)
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

func renderModelRiskReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildModelRiskReport(cfg)
	configFile := inspectConfigSurface(cfg.Workdir).ConfigFile
	var b strings.Builder
	b.WriteString("## GitClaw Model Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeModelRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits model/provider metadata inspired by OpenClaw's model status split and Hermes profile config boundaries. It does not call the GitHub Models catalog or inference endpoint. It reports provider, endpoint, retry, fallback, config metadata, finding codes, severities, and hashes only; model config bodies, issue bodies, comments, prompts, provider error bodies, credentials, and secret values are not included.\n\n")

	b.WriteString("### Provider Risk Card\n")
	writeModelProviderRiskCard(&b, report)

	b.WriteString("\n### Fallback Risk Card\n")
	writeModelFallbackRiskCard(&b, report)

	b.WriteString("\n### Retry Risk Card\n")
	writeModelRetryRiskCard(&b, report)

	b.WriteString("\n### Config Model Risk Card\n")
	writeModelConfigRiskCard(&b, cfg.Workdir, configFile)

	b.WriteString("\n### Current Model Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-model-request` current_issue_model_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-model-request` scope=`local-cli` current_issue_model_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeModelRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeModelRiskSummary(b *strings.Builder, report ModelRiskReport) {
	fmt.Fprintf(b, "- model_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- fallback_models: `%s`\n", inlineListOrNone(report.FallbackModels))
	fmt.Fprintf(b, "- fallback_models_configured: `%d`\n", report.FallbackModelsConfigured)
	fmt.Fprintf(b, "- default_model_policy: `%s`\n", report.DefaultModelPolicy)
	fmt.Fprintf(b, "- catalog_endpoint_host: `%s`\n", report.CatalogEndpointHost)
	fmt.Fprintf(b, "- endpoint_host: `%s`\n", report.EndpointHost)
	fmt.Fprintf(b, "- token_source: `%s`\n", report.TokenSource)
	fmt.Fprintf(b, "- output_token_parameter: `%s`\n", report.OutputTokenParameter)
	fmt.Fprintf(b, "- request_timeout_seconds: `%d`\n", report.RequestTimeoutSeconds)
	fmt.Fprintf(b, "- retry_max_attempts: `%d`\n", report.RetryMaxAttempts)
	fmt.Fprintf(b, "- retry_base_delay_seconds: `%d`\n", report.RetryBaseDelaySeconds)
	fmt.Fprintf(b, "- retry_max_delay_seconds: `%d`\n", report.RetryMaxDelaySeconds)
	fmt.Fprintf(b, "- retryable_statuses: `%s`\n", report.RetryableStatuses)
	fmt.Fprintf(b, "- fallback_on_retryable_statuses: `%t`\n", report.FallbackOnRetryableStatuses)
	fmt.Fprintf(b, "- fallback_primary_attempts_before_fallback: `%d`\n", report.FallbackPrimaryAttemptsBeforeFallback)
	fmt.Fprintf(b, "- prompt_artifact_enabled: `%t`\n", report.PromptArtifactEnabled)
	fmt.Fprintf(b, "- config_file_present: `%t`\n", report.ConfigFilePresent)
	fmt.Fprintf(b, "- config_file_path: `%s`\n", report.ConfigFilePath)
	fmt.Fprintf(b, "- github_models_endpoint: `%t`\n", report.GitHubModelsEndpoint)
	fmt.Fprintf(b, "- openai_compatible_endpoint: `%t`\n", report.OpenAICompatibleEndpoint)
	fmt.Fprintf(b, "- github_actions_token_supported: `%t`\n", report.GitHubActionsTokenSupported)
	fmt.Fprintf(b, "- primary_model_small_default: `%t`\n", report.PrimaryModelSmallDefault)
	fmt.Fprintf(b, "- primary_model_known_github_catalog_entry: `%t`\n", report.PrimaryModelKnownGitHubCatalogEntry)
	fmt.Fprintf(b, "- fallback_models_known_github_catalog_entries: `%d`\n", report.FallbackModelsKnownGitHubCatalogEntries)
	fmt.Fprintf(b, "- model_catalog_probe_performed: `%t`\n", report.ModelCatalogProbePerformed)
	fmt.Fprintf(b, "- live_inference_probe_performed: `%t`\n", report.LiveInferenceProbePerformed)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- model_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- raw_model_config_bodies_included: `%t`\n", report.RawModelConfigBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_provider_error_bodies_included: `%t`\n", report.RawProviderErrorBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_model_risk_change: `%t`\n", report.LLME2ERequiredAfterModelRiskChange)
}

func writeModelProviderRiskCard(b *strings.Builder, report ModelRiskReport) {
	findings := filterModelRiskFindings(report.Findings, "provider")
	fmt.Fprintf(
		b,
		"- kind=`provider` provider=`%s` model=`%s` endpoint_host=`%s` catalog_endpoint_host=`%s` token_source=`%s` github_models_endpoint=`%t` openai_compatible_endpoint=`%t` github_actions_token_supported=`%t` primary_model_small_default=`%t` primary_model_known_github_catalog_entry=`%t` output_token_parameter=`%s` model_catalog_probe_performed=`%t` live_inference_probe_performed=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.Provider,
		report.Model,
		report.EndpointHost,
		report.CatalogEndpointHost,
		report.TokenSource,
		report.GitHubModelsEndpoint,
		report.OpenAICompatibleEndpoint,
		report.GitHubActionsTokenSupported,
		report.PrimaryModelSmallDefault,
		report.PrimaryModelKnownGitHubCatalogEntry,
		report.OutputTokenParameter,
		report.ModelCatalogProbePerformed,
		report.LiveInferenceProbePerformed,
		len(findings),
		modelRiskMaxSeverity(findings),
		inlineListOrNone(modelRiskCodes(findings)),
		inlineListOrNone(modelRiskLineHashes(findings)),
	)
}

func writeModelFallbackRiskCard(b *strings.Builder, report ModelRiskReport) {
	findings := filterModelRiskFindings(report.Findings, "fallback")
	fmt.Fprintf(
		b,
		"- kind=`fallback` fallback_models=`%s` fallback_models_configured=`%d` fallback_on_retryable_statuses=`%t` fallback_primary_attempts_before_fallback=`%d` fallback_models_known_github_catalog_entries=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineListOrNone(report.FallbackModels),
		report.FallbackModelsConfigured,
		report.FallbackOnRetryableStatuses,
		report.FallbackPrimaryAttemptsBeforeFallback,
		report.FallbackModelsKnownGitHubCatalogEntries,
		len(findings),
		modelRiskMaxSeverity(findings),
		inlineListOrNone(modelRiskCodes(findings)),
		inlineListOrNone(modelRiskLineHashes(findings)),
	)
}

func writeModelRetryRiskCard(b *strings.Builder, report ModelRiskReport) {
	findings := filterModelRiskFindings(report.Findings, "retry")
	fmt.Fprintf(
		b,
		"- kind=`retry` request_timeout_seconds=`%d` retry_max_attempts=`%d` retry_base_delay_seconds=`%d` retry_max_delay_seconds=`%d` retryable_statuses=`%s` raw_provider_error_bodies_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.RequestTimeoutSeconds,
		report.RetryMaxAttempts,
		report.RetryBaseDelaySeconds,
		report.RetryMaxDelaySeconds,
		report.RetryableStatuses,
		report.RawProviderErrorBodiesIncluded,
		len(findings),
		modelRiskMaxSeverity(findings),
		inlineListOrNone(modelRiskCodes(findings)),
		inlineListOrNone(modelRiskLineHashes(findings)),
	)
}

func writeModelConfigRiskCard(b *strings.Builder, root string, file configSurfaceFile) {
	findings := []ModelRiskFinding(nil)
	if file.Present {
		findings = scanModelConfigRiskText(file.Path, readModelRiskBody(root, file.Path))
	}
	if !file.Present {
		fmt.Fprintf(b, "- kind=`model-config` path=`%s` present=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none` line_hashes=`none`\n", file.Path)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`model-config` path=`%s` present=`true` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		file.Path,
		file.Bytes,
		file.Lines,
		file.SHA,
		len(findings),
		modelRiskMaxSeverity(findings),
		inlineListOrNone(modelRiskCodes(findings)),
		inlineListOrNone(modelRiskLineHashes(findings)),
	)
}

func modelMetadataRiskFindings(report ModelRiskReport, baseURL string, cfg Config) []ModelRiskFinding {
	var findings []ModelRiskFinding
	if strings.TrimSpace(report.Model) == "" {
		findings = append(findings, modelRiskMetadataFinding("high", "model_id_missing", "model-selection", "provider", "model"))
	} else if strings.ContainsAny(strings.TrimSpace(report.Model), " \t\r\n") {
		findings = append(findings, modelRiskMetadataFinding("warning", "model_id_contains_whitespace", "model-selection", "provider", "model"))
	} else if !strings.Contains(report.Model, "/") {
		findings = append(findings, modelRiskMetadataFinding("info", "model_id_unqualified", "model-selection", "provider", "model"))
	}
	if report.Provider != "github-models" {
		findings = append(findings, modelRiskMetadataFinding("warning", "non_github_models_provider", "provider-boundary", "provider", "provider"))
	}
	if !report.GitHubModelsEndpoint {
		findings = append(findings, modelRiskMetadataFinding("warning", "non_github_models_endpoint", "provider-boundary", "provider", "endpoint_host"))
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(baseURL)), "http://") {
		findings = append(findings, modelRiskMetadataFinding("high", "insecure_model_endpoint", "transport-security", "provider", "endpoint_host"))
	}
	if !report.PrimaryModelSmallDefault {
		findings = append(findings, modelRiskMetadataFinding("info", "primary_model_not_small_default", "model-selection", "provider", "model"))
	}
	if report.Model != "" && report.GitHubModelsEndpoint && !report.PrimaryModelKnownGitHubCatalogEntry {
		findings = append(findings, modelRiskMetadataFinding("warning", "primary_model_not_in_known_catalog_snapshot", "model-selection", "provider", "model"))
	}
	if report.FallbackModelsConfigured == 0 {
		findings = append(findings, modelRiskMetadataFinding("info", "fallback_models_not_configured", "resilience", "fallback", "fallback_models"))
	}
	for _, fallback := range report.FallbackModels {
		if strings.EqualFold(fallback, report.Model) {
			findings = append(findings, modelRiskMetadataFinding("warning", "fallback_model_duplicates_primary", "resilience", "fallback", "fallback_models"))
		}
		if report.GitHubModelsEndpoint && !isKnownGitHubModelsModel(fallback) {
			findings = append(findings, modelRiskMetadataFinding("warning", "fallback_model_not_in_known_catalog_snapshot", "resilience", "fallback", "fallback_models"))
		}
	}
	if cfg.MaxPromptBytes <= 0 {
		findings = append(findings, modelRiskMetadataFinding("high", "max_prompt_bytes_not_positive", "prompt-budget", "provider", "max_prompt_bytes"))
	} else if cfg.MaxPromptBytes > 200000 {
		findings = append(findings, modelRiskMetadataFinding("warning", "max_prompt_bytes_exceeds_default_gpt5_context", "prompt-budget", "provider", "max_prompt_bytes"))
	}
	if cfg.MaxOutputTokens <= 0 {
		findings = append(findings, modelRiskMetadataFinding("high", "max_output_tokens_not_positive", "output-budget", "provider", "max_output_tokens"))
	} else if cfg.MaxOutputTokens > 100000 {
		findings = append(findings, modelRiskMetadataFinding("warning", "max_output_tokens_excessive", "output-budget", "provider", "max_output_tokens"))
	}
	if report.RequestTimeoutSeconds < 15 {
		findings = append(findings, modelRiskMetadataFinding("warning", "model_request_timeout_too_low", "retry-policy", "retry", "request_timeout_seconds"))
	}
	if report.RetryMaxAttempts < 1 {
		findings = append(findings, modelRiskMetadataFinding("high", "retry_max_attempts_not_positive", "retry-policy", "retry", "retry_max_attempts"))
	} else if report.RetryMaxAttempts > 10 {
		findings = append(findings, modelRiskMetadataFinding("warning", "retry_max_attempts_excessive", "retry-policy", "retry", "retry_max_attempts"))
	}
	if report.RetryMaxDelaySeconds > 300 {
		findings = append(findings, modelRiskMetadataFinding("warning", "retry_max_delay_excessive", "retry-policy", "retry", "retry_max_delay_seconds"))
	}
	sortModelRiskFindings(findings)
	return findings
}

func scanModelConfigRiskText(path, body string) []ModelRiskFinding {
	var findings []ModelRiskFinding
	lines := strings.Split(body, "\n")
	for lineNumber, line := range lines {
		lower := strings.ToLower(line)
		contextLower := strings.ToLower(modelRiskLineContext(lines, lineNumber))
		for _, rule := range modelConfigRiskRules {
			if !modelRiskRuleMatches(lower, contextLower, rule) {
				continue
			}
			findings = append(findings, ModelRiskFinding{Severity: rule.Severity, Code: rule.Code, Category: rule.Category, Kind: "model-config", Path: path, Field: "body", Line: lineNumber + 1, LineSHA: shortDocumentHash(line)})
		}
	}
	sortModelRiskFindings(findings)
	return findings
}

func modelRiskRuleMatches(lowerLine, lowerContext string, rule modelRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerContext, ignored) {
			return false
		}
	}
	for _, required := range rule.All {
		if !strings.Contains(lowerLine, required) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func modelRiskLineContext(lines []string, lineNumber int) string {
	var context []string
	start := lineNumber - 2
	if start < 0 {
		start = 0
	}
	end := lineNumber + 2
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		context = append(context, lines[i])
	}
	return strings.Join(context, " ")
}

func modelRiskMetadataFinding(severity, code, category, kind, field string) ModelRiskFinding {
	return ModelRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Path:     "model-config",
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + field + ":" + code),
	}
}

func filterModelRiskFindings(findings []ModelRiskFinding, kind string) []ModelRiskFinding {
	var filtered []ModelRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			filtered = append(filtered, finding)
		}
	}
	sortModelRiskFindings(filtered)
	return filtered
}

func readModelRiskBody(root, relPath string) string {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	if err != nil {
		return ""
	}
	return string(body)
}

func writeModelRiskFindings(b *strings.Builder, findings []ModelRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Path, finding.Field, finding.Line, finding.LineSHA)
	}
}

func modelRiskSurfaceCount(findings []ModelRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Path
		if key == "\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func modelRiskCodes(findings []ModelRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func modelRiskLineHashes(findings []ModelRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	sort.Strings(hashes)
	return hashes
}

func modelRiskMaxSeverity(findings []ModelRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if modelRiskSeverityRank(finding.Severity) > modelRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func modelRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortModelRiskFindings(findings []ModelRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return modelRiskSeverityRank(findings[i].Severity) > modelRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Field != findings[j].Field {
			return findings[i].Field < findings[j].Field
		}
		return findings[i].Line < findings[j].Line
	})
}

func isKnownGitHubModelsModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "openai/gpt-5-nano",
		"openai/gpt-5-mini",
		"openai/gpt-5",
		"openai/gpt-5-chat",
		"openai/gpt-4.1-nano",
		"openai/gpt-4.1-mini",
		"openai/gpt-4.1":
		return true
	default:
		return false
	}
}
