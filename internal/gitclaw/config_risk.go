package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ConfigRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type ConfigRiskReport struct {
	Status                               string
	VerificationScope                    string
	ConfigSource                         string
	ConfigFilePresent                    bool
	ConfigFilePath                       string
	WorkflowFilesExpected                int
	WorkflowFilesPresent                 int
	WorkflowFilesMissing                 int
	TriggerMode                          string
	TriggerLabel                         string
	TriggerPrefix                        string
	DisabledLabel                        string
	TrustedAssociations                  []string
	TrustedAssociationsConfigured        int
	BroadTrustedAssociations             []string
	BroadTrustedAssociationsConfigured   int
	ManagedLabels                        []string
	ManagedLabelsConfigured              int
	DuplicateManagedLabels               int
	ModelProvider                        string
	Model                                string
	ModelFallbacks                       []string
	ModelFallbacksConfigured             int
	RunMode                              string
	MaxPromptBytes                       int
	MaxOutputTokens                      int
	MaxTranscriptMessages                int
	MaxTranscriptMessageBytes            int
	SkillsAllowedConfigured              int
	SkillsDisabledConfigured             int
	SkillGateConflicts                   int
	ToolsAllowedConfigured               int
	ToolsDisabledConfigured              int
	ToolGateConflicts                    int
	SlashCommands                        int
	SurfacesWithRiskFindings             int
	Findings                             []ConfigRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	RawConfigBodiesIncluded              bool
	RawWorkflowBodiesIncluded            bool
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	RawPromptBodiesIncluded              bool
	RawProviderErrorBodiesIncluded       bool
	CredentialValuesIncluded             bool
	RepositoryMutationAllowed            bool
	AgentAuthoredConfigMutationSupported bool
	LLME2ERequiredAfterConfigRiskChange  bool
}

type configRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Kind      string
	Any       []string
	All       []string
	IgnoreAny []string
}

var configFileRiskRules = []configRiskRule{
	{
		Severity: "high",
		Code:     "credential_material_in_config",
		Category: "credential-handling",
		Kind:     "config-file",
		Any: []string{
			"api_key:",
			"api_key=",
			"openai_api_key:",
			"openai_api_key=",
			"github_token:",
			"github_token=",
			"password:",
			"password=",
			"private_key:",
			"private_key=",
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
		Code:     "config_write_mode_enabled",
		Category: "mutation-boundary",
		Kind:     "config-file",
		Any: []string{
			"mode: write",
			"mode: write_enabled",
			"mode: full",
			"mode: mutate",
		},
	},
	{
		Severity: "warning",
		Code:     "raw_prompt_logging_enabled",
		Category: "prompt-leakage",
		Kind:     "config-file",
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
		Code:     "external_webhook_configured",
		Category: "ingress-boundary",
		Kind:     "config-file",
		Any: []string{
			"webhook_url:",
			"webhook:",
			"socket_url:",
			"websocket:",
			"daemon:",
		},
		IgnoreAny: []string{
			"workflow_dispatch",
			"no webhook",
			"without webhook",
			"webhook-free",
		},
	},
}

var workflowConfigRiskRules = []configRiskRule{
	{
		Severity: "high",
		Code:     "workflow_write_all_permissions",
		Category: "workflow-permissions",
		Kind:     "workflow-file",
		Any:      []string{"write-all"},
	},
	{
		Severity: "warning",
		Code:     "pull_request_target_trigger",
		Category: "workflow-ingress",
		Kind:     "workflow-file",
		Any:      []string{"pull_request_target:"},
	},
	{
		Severity: "warning",
		Code:     "workflow_raw_secret_echo",
		Category: "credential-handling",
		Kind:     "workflow-file",
		Any: []string{
			"echo $github_token",
			"echo $gh_token",
			"echo $openai_api_key",
			"printenv",
			"env |",
		},
	},
	{
		Severity: "warning",
		Code:     "workflow_unbounded_background_process",
		Category: "runtime-boundary",
		Kind:     "workflow-file",
		Any: []string{
			"nohup ",
			"while true",
			"sleep infinity",
			"tail -f",
		},
	},
}

func BuildConfigRiskReport(cfg Config) ConfigRiskReport {
	surface := inspectConfigSurface(cfg.Workdir)
	trusted := sortedAllowedAssociations(cfg)
	broadTrusted := broadTrustedAssociations(trusted)
	managedLabels := managedPolicyLabels(cfg)
	report := ConfigRiskReport{
		Status:                               "ok",
		VerificationScope:                    "repo_local_config_control_plane",
		ConfigSource:                         configSource(cfg),
		ConfigFilePresent:                    surface.ConfigFile.Present,
		ConfigFilePath:                       gitclawConfigPath,
		WorkflowFilesExpected:                len(surface.Workflows),
		WorkflowFilesPresent:                 countPresentConfigFiles(surface.Workflows),
		WorkflowFilesMissing:                 len(surface.Workflows) - countPresentConfigFiles(surface.Workflows),
		TriggerMode:                          cfg.TriggerMode,
		TriggerLabel:                         cfg.TriggerLabel,
		TriggerPrefix:                        cfg.TriggerPrefix,
		DisabledLabel:                        cfg.DisabledLabel,
		TrustedAssociations:                  trusted,
		TrustedAssociationsConfigured:        len(trusted),
		BroadTrustedAssociations:             broadTrusted,
		BroadTrustedAssociationsConfigured:   len(broadTrusted),
		ManagedLabels:                        managedLabels,
		ManagedLabelsConfigured:              len(managedLabels),
		DuplicateManagedLabels:               duplicateStringCount(managedLabels),
		ModelProvider:                        cfg.ModelProvider,
		Model:                                cfg.Model,
		ModelFallbacks:                       normalizeModelFallbacks(cfg.ModelFallbacks),
		ModelFallbacksConfigured:             len(normalizeModelFallbacks(cfg.ModelFallbacks)),
		RunMode:                              "read-only",
		MaxPromptBytes:                       cfg.MaxPromptBytes,
		MaxOutputTokens:                      cfg.MaxOutputTokens,
		MaxTranscriptMessages:                cfg.MaxTranscriptMessages,
		MaxTranscriptMessageBytes:            cfg.MaxTranscriptMessageBytes,
		SkillsAllowedConfigured:              trueMapCount(cfg.AllowedSkills),
		SkillsDisabledConfigured:             trueMapCount(cfg.DisabledSkills),
		SkillGateConflicts:                   setIntersectionCount(cfg.AllowedSkills, cfg.DisabledSkills),
		ToolsAllowedConfigured:               trueMapCount(cfg.AllowedTools),
		ToolsDisabledConfigured:              trueMapCount(cfg.DisabledTools),
		ToolGateConflicts:                    setIntersectionCount(cfg.AllowedTools, cfg.DisabledTools),
		SlashCommands:                        len(configSlashCommands),
		RawConfigBodiesIncluded:              false,
		RawWorkflowBodiesIncluded:            false,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		RawPromptBodiesIncluded:              false,
		RawProviderErrorBodiesIncluded:       false,
		CredentialValuesIncluded:             false,
		RepositoryMutationAllowed:            false,
		AgentAuthoredConfigMutationSupported: false,
		LLME2ERequiredAfterConfigRiskChange:  true,
	}
	report.Findings = buildConfigRiskFindings(report, cfg, surface)
	sortConfigRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = configRiskSurfaceCount(report.Findings)
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

func RenderConfigRiskCLIReport(cfg Config) string {
	return renderConfigRiskReport(Event{}, cfg, false)
}

func renderConfigRiskReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildConfigRiskReport(cfg)
	surface := inspectConfigSurface(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Config Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeConfigRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits GitClaw's repo-local config control plane inspired by OpenClaw config/schema checks and Hermes profile config boundaries. It reports config/workflow metadata, trigger and trust settings, model budgets, gate conflicts, finding codes, severities, and hashes only; config bodies, workflow bodies, issue bodies, comments, prompts, provider errors, credentials, and secret values are not included.\n\n")

	b.WriteString("### Config File Risk Card\n")
	writeConfigRiskFileCard(&b, report.Findings, surface.ConfigFile, "config-file")

	b.WriteString("\n### Workflow Risk Cards\n")
	for _, file := range surface.Workflows {
		writeConfigRiskFileCard(&b, report.Findings, file, "workflow-file")
	}

	b.WriteString("\n### Trigger And Trust Risk Card\n")
	writeConfigTriggerTrustRiskCard(&b, report)

	b.WriteString("\n### Model And Budget Risk Card\n")
	writeConfigModelBudgetRiskCard(&b, report)

	b.WriteString("\n### Gate Risk Card\n")
	writeConfigGateRiskCard(&b, report)

	b.WriteString("\n### Current Config Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-config-request` current_issue_config_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-config-request` scope=`local-cli` current_issue_config_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeConfigRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeConfigRiskSummary(b *strings.Builder, report ConfigRiskReport) {
	fmt.Fprintf(b, "- config_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- config_source: `%s`\n", report.ConfigSource)
	fmt.Fprintf(b, "- config_file_path: `%s`\n", report.ConfigFilePath)
	fmt.Fprintf(b, "- config_file_present: `%t`\n", report.ConfigFilePresent)
	fmt.Fprintf(b, "- workflow_files_expected: `%d`\n", report.WorkflowFilesExpected)
	fmt.Fprintf(b, "- workflow_files_present: `%d`\n", report.WorkflowFilesPresent)
	fmt.Fprintf(b, "- workflow_files_missing: `%d`\n", report.WorkflowFilesMissing)
	fmt.Fprintf(b, "- trigger_mode: `%s`\n", report.TriggerMode)
	fmt.Fprintf(b, "- trigger_label: `%s`\n", report.TriggerLabel)
	fmt.Fprintf(b, "- trigger_prefix: `%s`\n", report.TriggerPrefix)
	fmt.Fprintf(b, "- disabled_label: `%s`\n", report.DisabledLabel)
	fmt.Fprintf(b, "- trusted_associations: `%s`\n", inlineListOrNone(report.TrustedAssociations))
	fmt.Fprintf(b, "- trusted_associations_configured: `%d`\n", report.TrustedAssociationsConfigured)
	fmt.Fprintf(b, "- broad_trusted_associations: `%s`\n", inlineListOrNone(report.BroadTrustedAssociations))
	fmt.Fprintf(b, "- broad_trusted_associations_configured: `%d`\n", report.BroadTrustedAssociationsConfigured)
	fmt.Fprintf(b, "- managed_labels_configured: `%d`\n", report.ManagedLabelsConfigured)
	fmt.Fprintf(b, "- duplicate_managed_labels: `%d`\n", report.DuplicateManagedLabels)
	fmt.Fprintf(b, "- model_provider: `%s`\n", report.ModelProvider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- model_fallbacks: `%s`\n", inlineListOrNone(report.ModelFallbacks))
	fmt.Fprintf(b, "- model_fallbacks_configured: `%d`\n", report.ModelFallbacksConfigured)
	fmt.Fprintf(b, "- run_mode: `%s`\n", report.RunMode)
	fmt.Fprintf(b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(b, "- max_output_tokens: `%d`\n", report.MaxOutputTokens)
	fmt.Fprintf(b, "- max_transcript_messages: `%d`\n", report.MaxTranscriptMessages)
	fmt.Fprintf(b, "- max_transcript_message_bytes: `%d`\n", report.MaxTranscriptMessageBytes)
	fmt.Fprintf(b, "- skills_allowed_configured: `%d`\n", report.SkillsAllowedConfigured)
	fmt.Fprintf(b, "- skills_disabled_configured: `%d`\n", report.SkillsDisabledConfigured)
	fmt.Fprintf(b, "- skill_gate_conflicts: `%d`\n", report.SkillGateConflicts)
	fmt.Fprintf(b, "- tools_allowed_configured: `%d`\n", report.ToolsAllowedConfigured)
	fmt.Fprintf(b, "- tools_disabled_configured: `%d`\n", report.ToolsDisabledConfigured)
	fmt.Fprintf(b, "- tool_gate_conflicts: `%d`\n", report.ToolGateConflicts)
	fmt.Fprintf(b, "- slash_commands: `%d`\n", report.SlashCommands)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- config_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- raw_config_bodies_included: `%t`\n", report.RawConfigBodiesIncluded)
	fmt.Fprintf(b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_provider_error_bodies_included: `%t`\n", report.RawProviderErrorBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- agent_authored_config_mutation_supported: `%t`\n", report.AgentAuthoredConfigMutationSupported)
	fmt.Fprintf(b, "- llm_e2e_required_after_config_risk_change: `%t`\n", report.LLME2ERequiredAfterConfigRiskChange)
}

func writeConfigRiskFileCard(b *strings.Builder, findings []ConfigRiskFinding, file configSurfaceFile, kind string) {
	findings = filterConfigRiskFindings(findings, kind, file.Path)
	if !file.Present {
		fmt.Fprintf(b, "- kind=`%s` path=`%s` present=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n", kind, file.Path, len(findings), configRiskMaxSeverity(findings), inlineListOrNone(configRiskCodes(findings)), inlineListOrNone(configRiskLineHashes(findings)))
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`%s` path=`%s` present=`true` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		kind,
		file.Path,
		file.Bytes,
		file.Lines,
		file.SHA,
		len(findings),
		configRiskMaxSeverity(findings),
		inlineListOrNone(configRiskCodes(findings)),
		inlineListOrNone(configRiskLineHashes(findings)),
	)
}

func writeConfigTriggerTrustRiskCard(b *strings.Builder, report ConfigRiskReport) {
	findings := filterConfigRiskFindings(report.Findings, "trigger-trust", "config-metadata")
	fmt.Fprintf(
		b,
		"- kind=`trigger-trust` trigger_mode=`%s` trigger_label=`%s` trigger_prefix=`%s` disabled_label=`%s` trusted_associations=`%s` broad_trusted_associations=`%s` managed_labels_configured=`%d` duplicate_managed_labels=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.TriggerMode,
		report.TriggerLabel,
		report.TriggerPrefix,
		report.DisabledLabel,
		inlineListOrNone(report.TrustedAssociations),
		inlineListOrNone(report.BroadTrustedAssociations),
		report.ManagedLabelsConfigured,
		report.DuplicateManagedLabels,
		len(findings),
		configRiskMaxSeverity(findings),
		inlineListOrNone(configRiskCodes(findings)),
		inlineListOrNone(configRiskLineHashes(findings)),
	)
}

func writeConfigModelBudgetRiskCard(b *strings.Builder, report ConfigRiskReport) {
	findings := filterConfigRiskFindings(report.Findings, "model-budget", "config-metadata")
	fmt.Fprintf(
		b,
		"- kind=`model-budget` model_provider=`%s` model=`%s` model_fallbacks=`%s` model_fallbacks_configured=`%d` max_prompt_bytes=`%d` max_output_tokens=`%d` max_transcript_messages=`%d` max_transcript_message_bytes=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.ModelProvider,
		report.Model,
		inlineListOrNone(report.ModelFallbacks),
		report.ModelFallbacksConfigured,
		report.MaxPromptBytes,
		report.MaxOutputTokens,
		report.MaxTranscriptMessages,
		report.MaxTranscriptMessageBytes,
		len(findings),
		configRiskMaxSeverity(findings),
		inlineListOrNone(configRiskCodes(findings)),
		inlineListOrNone(configRiskLineHashes(findings)),
	)
}

func writeConfigGateRiskCard(b *strings.Builder, report ConfigRiskReport) {
	findings := filterConfigRiskFindings(report.Findings, "gate", "config-metadata")
	fmt.Fprintf(
		b,
		"- kind=`gate` skills_allowed_configured=`%d` skills_disabled_configured=`%d` skill_gate_conflicts=`%d` tools_allowed_configured=`%d` tools_disabled_configured=`%d` tool_gate_conflicts=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.SkillsAllowedConfigured,
		report.SkillsDisabledConfigured,
		report.SkillGateConflicts,
		report.ToolsAllowedConfigured,
		report.ToolsDisabledConfigured,
		report.ToolGateConflicts,
		len(findings),
		configRiskMaxSeverity(findings),
		inlineListOrNone(configRiskCodes(findings)),
		inlineListOrNone(configRiskLineHashes(findings)),
	)
}

func buildConfigRiskFindings(report ConfigRiskReport, cfg Config, surface configSurface) []ConfigRiskFinding {
	var findings []ConfigRiskFinding
	if !report.ConfigFilePresent {
		findings = append(findings, configRiskMetadataFinding("warning", "config_file_missing", "config-presence", "config-file", gitclawConfigPath, "present"))
	}
	for _, workflow := range surface.Workflows {
		if !workflow.Present {
			findings = append(findings, configRiskMetadataFinding("warning", "workflow_file_missing", "workflow-presence", "workflow-file", workflow.Path, "present"))
		}
	}
	if _, err := normalizeTriggerMode(report.TriggerMode); err != nil {
		findings = append(findings, configRiskMetadataFinding("high", "trigger_mode_invalid", "trigger-boundary", "trigger-trust", "config-metadata", "trigger_mode"))
	}
	if strings.TrimSpace(report.TriggerLabel) == "" {
		findings = append(findings, configRiskMetadataFinding("high", "trigger_label_empty", "trigger-boundary", "trigger-trust", "config-metadata", "trigger_label"))
	}
	if strings.TrimSpace(report.TriggerPrefix) == "" {
		findings = append(findings, configRiskMetadataFinding("high", "trigger_prefix_empty", "trigger-boundary", "trigger-trust", "config-metadata", "trigger_prefix"))
	}
	if strings.TrimSpace(report.DisabledLabel) == "" {
		findings = append(findings, configRiskMetadataFinding("high", "disabled_label_empty", "trigger-boundary", "trigger-trust", "config-metadata", "disabled_label"))
	}
	if strings.TrimSpace(report.TriggerLabel) != "" && strings.EqualFold(report.TriggerLabel, report.DisabledLabel) {
		findings = append(findings, configRiskMetadataFinding("high", "trigger_disabled_label_collision", "trigger-boundary", "trigger-trust", "config-metadata", "labels"))
	}
	if report.DuplicateManagedLabels > 0 {
		findings = append(findings, configRiskMetadataFinding("warning", "managed_label_collision", "label-boundary", "trigger-trust", "config-metadata", "managed_labels"))
	}
	if report.TrustedAssociationsConfigured == 0 {
		findings = append(findings, configRiskMetadataFinding("high", "trusted_associations_empty", "authorization", "trigger-trust", "config-metadata", "trusted_associations"))
	}
	for _, association := range report.BroadTrustedAssociations {
		findings = append(findings, configRiskMetadataFinding("warning", "broad_trusted_association", "authorization", "trigger-trust", "config-metadata", strings.ToLower(association)))
	}
	if strings.TrimSpace(report.Model) == "" {
		findings = append(findings, configRiskMetadataFinding("high", "model_id_missing", "model-selection", "model-budget", "config-metadata", "model"))
	}
	if report.ModelProvider != "github-models" {
		findings = append(findings, configRiskMetadataFinding("warning", "non_github_models_provider", "provider-boundary", "model-budget", "config-metadata", "model_provider"))
	}
	if report.ModelFallbacksConfigured == 0 {
		findings = append(findings, configRiskMetadataFinding("info", "model_fallbacks_not_configured", "resilience", "model-budget", "config-metadata", "model_fallbacks"))
	}
	if report.MaxPromptBytes <= 0 {
		findings = append(findings, configRiskMetadataFinding("high", "max_prompt_bytes_not_positive", "prompt-budget", "model-budget", "config-metadata", "max_prompt_bytes"))
	} else if report.MaxPromptBytes > 200000 {
		findings = append(findings, configRiskMetadataFinding("warning", "max_prompt_bytes_exceeds_default_gpt5_context", "prompt-budget", "model-budget", "config-metadata", "max_prompt_bytes"))
	}
	if report.MaxOutputTokens <= 0 {
		findings = append(findings, configRiskMetadataFinding("high", "max_output_tokens_not_positive", "output-budget", "model-budget", "config-metadata", "max_output_tokens"))
	} else if report.MaxOutputTokens > 100000 {
		findings = append(findings, configRiskMetadataFinding("warning", "max_output_tokens_excessive", "output-budget", "model-budget", "config-metadata", "max_output_tokens"))
	}
	if report.MaxTranscriptMessages <= 0 {
		findings = append(findings, configRiskMetadataFinding("high", "max_transcript_messages_not_positive", "transcript-budget", "model-budget", "config-metadata", "max_transcript_messages"))
	}
	if report.MaxTranscriptMessageBytes <= 0 {
		findings = append(findings, configRiskMetadataFinding("high", "max_transcript_message_bytes_not_positive", "transcript-budget", "model-budget", "config-metadata", "max_transcript_message_bytes"))
	}
	if report.SkillGateConflicts > 0 {
		findings = append(findings, configRiskMetadataFinding("warning", "skill_gate_conflict", "skill-gates", "gate", "config-metadata", "skill_gate_conflicts"))
	}
	if report.ToolGateConflicts > 0 {
		findings = append(findings, configRiskMetadataFinding("warning", "tool_gate_conflict", "tool-gates", "gate", "config-metadata", "tool_gate_conflicts"))
	}
	findings = append(findings, scanConfigRiskFile(cfg.Workdir, surface.ConfigFile, "config-file")...)
	for _, workflow := range surface.Workflows {
		findings = append(findings, scanConfigRiskFile(cfg.Workdir, workflow, "workflow-file")...)
	}
	sortConfigRiskFindings(findings)
	return findings
}

func scanConfigRiskFile(root string, file configSurfaceFile, kind string) []ConfigRiskFinding {
	if !file.Present {
		return nil
	}
	body := readConfigRiskBody(root, file.Path)
	rules := configFileRiskRules
	if kind == "workflow-file" {
		rules = workflowConfigRiskRules
	}
	return scanConfigRiskText(file.Path, body, kind, rules)
}

func scanConfigRiskText(path, body, kind string, rules []configRiskRule) []ConfigRiskFinding {
	var findings []ConfigRiskFinding
	lines := strings.Split(body, "\n")
	for lineNumber, line := range lines {
		lower := strings.ToLower(line)
		contextLower := strings.ToLower(configRiskLineContext(lines, lineNumber))
		for _, rule := range rules {
			if rule.Kind != "" && rule.Kind != kind {
				continue
			}
			if !configRiskRuleMatches(lower, contextLower, rule) {
				continue
			}
			findings = append(findings, ConfigRiskFinding{Severity: rule.Severity, Code: rule.Code, Category: rule.Category, Kind: kind, Path: path, Field: "body", Line: lineNumber + 1, LineSHA: shortDocumentHash(line)})
		}
	}
	sortConfigRiskFindings(findings)
	return findings
}

func configRiskRuleMatches(lowerLine, lowerContext string, rule configRiskRule) bool {
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

func configRiskLineContext(lines []string, lineNumber int) string {
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

func configRiskMetadataFinding(severity, code, category, kind, path, field string) ConfigRiskFinding {
	return ConfigRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Path:     path,
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + path + ":" + field + ":" + code),
	}
}

func readConfigRiskBody(root, relPath string) string {
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

func writeConfigRiskFindings(b *strings.Builder, findings []ConfigRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Path, finding.Field, finding.Line, finding.LineSHA)
	}
}

func filterConfigRiskFindings(findings []ConfigRiskFinding, kind, path string) []ConfigRiskFinding {
	var filtered []ConfigRiskFinding
	for _, finding := range findings {
		if finding.Kind != kind {
			continue
		}
		if path != "" && finding.Path != path {
			continue
		}
		filtered = append(filtered, finding)
	}
	sortConfigRiskFindings(filtered)
	return filtered
}

func configRiskSurfaceCount(findings []ConfigRiskFinding) int {
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

func configRiskCodes(findings []ConfigRiskFinding) []string {
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

func configRiskLineHashes(findings []ConfigRiskFinding) []string {
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

func configRiskMaxSeverity(findings []ConfigRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if configRiskSeverityRank(finding.Severity) > configRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func configRiskSeverityRank(severity string) int {
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

func sortConfigRiskFindings(findings []ConfigRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return configRiskSeverityRank(findings[i].Severity) > configRiskSeverityRank(findings[j].Severity)
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

func trueMapCount(values map[string]bool) int {
	count := 0
	for _, ok := range values {
		if ok {
			count++
		}
	}
	return count
}

func setIntersectionCount(left, right map[string]bool) int {
	count := 0
	for name, ok := range left {
		if ok && right[name] {
			count++
		}
	}
	return count
}

func duplicateStringCount(values []string) int {
	seen := map[string]int{}
	duplicates := 0
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		seen[value]++
		if seen[value] == 2 {
			duplicates++
		}
	}
	return duplicates
}

func broadTrustedAssociations(associations []string) []string {
	broad := map[string]bool{}
	for _, association := range associations {
		switch strings.ToUpper(strings.TrimSpace(association)) {
		case "CONTRIBUTOR", "FIRST_TIME_CONTRIBUTOR", "FIRST_TIMER", "MANNEQUIN", "NONE":
			broad[strings.ToUpper(strings.TrimSpace(association))] = true
		}
	}
	var out []string
	for association := range broad {
		out = append(out, association)
	}
	return sortedStrings(out)
}
