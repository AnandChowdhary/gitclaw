package gitclaw

import (
	"fmt"
	"strings"
)

type SecurityAuditReport struct {
	Status                            string
	VerificationScope                 string
	TrustModel                        string
	RuntimeBoundary                   string
	GatewayServerRequired             bool
	HostileMultiTenantSupported       bool
	SurfacesScanned                   int
	SurfacesWithFindings              int
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	ConfigRisk                        ConfigRiskReport
	PolicyRisk                        PolicyRiskReport
	SandboxRisk                       SandboxRiskReport
	ChannelRisk                       ChannelRiskReport
	ToolRisk                          ToolRiskReport
	SkillRisk                         SkillRiskReport
	PluginRisk                        PluginRiskReport
	SecretRisk                        SecretRiskReport
	RawConfigBodiesIncluded           bool
	RawWorkflowBodiesIncluded         bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	RawPromptBodiesIncluded           bool
	RawToolOutputsIncluded            bool
	CredentialValuesIncluded          bool
	RepositoryMutationAllowed         bool
	HostExecAllowed                   bool
	LLME2ERequiredAfterSecurityChange bool
}

type securitySurfaceSummary struct {
	Name     string
	Status   string
	High     int
	Warning  int
	Info     int
	Findings int
	Notes    []string
}

func IsSecurityReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/security" || command == "/sec"
}

func BuildSecurityAuditReport(cfg Config, repoContext RepoContext, comments []Comment) (SecurityAuditReport, error) {
	secretAudit, err := BuildSecretAuditReport(cfg.Workdir)
	if err != nil {
		return SecurityAuditReport{}, err
	}
	report := SecurityAuditReport{
		VerificationScope:                 "openclaw_personal_assistant_security_audit",
		TrustModel:                        "personal-assistant-single-operator",
		RuntimeBoundary:                   "github-actions-ephemeral-runner",
		GatewayServerRequired:             false,
		HostileMultiTenantSupported:       false,
		ConfigRisk:                        BuildConfigRiskReport(cfg),
		PolicyRisk:                        BuildPolicyRiskReport(cfg, repoContext),
		SandboxRisk:                       BuildSandboxRiskReport(cfg, repoContext),
		ChannelRisk:                       BuildChannelRiskReport(cfg, comments),
		ToolRisk:                          BuildToolRiskReport(repoContext),
		SkillRisk:                         BuildSkillRiskReport(repoContext.SkillSummaries),
		PluginRisk:                        BuildPluginRiskReport(cfg),
		SecretRisk:                        BuildSecretRiskReport(secretAudit),
		RawConfigBodiesIncluded:           false,
		RawWorkflowBodiesIncluded:         false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		RawPromptBodiesIncluded:           false,
		RawToolOutputsIncluded:            false,
		CredentialValuesIncluded:          false,
		RepositoryMutationAllowed:         false,
		HostExecAllowed:                   false,
		LLME2ERequiredAfterSecurityChange: true,
	}
	summaries := securitySurfaceSummaries(report)
	report.SurfacesScanned = len(summaries)
	for _, summary := range summaries {
		if summary.Findings > 0 || summary.Status != "ok" && summary.Status != "locked_down" {
			report.SurfacesWithFindings++
		}
		report.HighRiskFindings += summary.High
		report.WarningRiskFindings += summary.Warning
		report.InfoRiskFindings += summary.Info
	}
	report.Status = securityAuditStatus(report)
	return report, nil
}

func RenderSecurityReport(ev Event, cfg Config, repoContext RepoContext, comments []Comment) (string, error) {
	report, err := BuildSecurityAuditReport(cfg, repoContext, comments)
	if err != nil {
		return "", err
	}
	return renderSecurityAuditReport(ev, report, true), nil
}

func RenderSecurityCLIReport(cfg Config, repoContext RepoContext) (string, error) {
	report, err := BuildSecurityAuditReport(cfg, repoContext, nil)
	if err != nil {
		return "", err
	}
	return renderSecurityAuditReport(Event{}, report, false), nil
}

func renderSecurityAuditReport(ev Event, report SecurityAuditReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Security Audit Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actorAssociation(ev))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- security_audit_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(&b, "- trust_model: `%s`\n", report.TrustModel)
	fmt.Fprintf(&b, "- runtime_boundary: `%s`\n", report.RuntimeBoundary)
	fmt.Fprintf(&b, "- gateway_server_required: `%t`\n", report.GatewayServerRequired)
	fmt.Fprintf(&b, "- hostile_multi_tenant_supported: `%t`\n", report.HostileMultiTenantSupported)
	fmt.Fprintf(&b, "- surfaces_scanned: `%d`\n", report.SurfacesScanned)
	fmt.Fprintf(&b, "- surfaces_with_findings: `%d`\n", report.SurfacesWithFindings)
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(&b, "- config_risk_status: `%s`\n", report.ConfigRisk.Status)
	fmt.Fprintf(&b, "- policy_risk_status: `%s`\n", report.PolicyRisk.Status)
	fmt.Fprintf(&b, "- sandbox_risk_status: `%s`\n", report.SandboxRisk.Status)
	fmt.Fprintf(&b, "- channel_risk_status: `%s`\n", report.ChannelRisk.Status)
	fmt.Fprintf(&b, "- tool_risk_status: `%s`\n", report.ToolRisk.Status)
	fmt.Fprintf(&b, "- skill_risk_status: `%s`\n", report.SkillRisk.Status)
	fmt.Fprintf(&b, "- plugin_risk_status: `%s`\n", report.PluginRisk.Status)
	fmt.Fprintf(&b, "- secrets_risk_status: `%s`\n", report.SecretRisk.Status)
	fmt.Fprintf(&b, "- raw_config_bodies_included: `%t`\n", report.RawConfigBodiesIncluded)
	fmt.Fprintf(&b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- host_exec_allowed: `%t`\n", report.HostExecAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_security_audit_change: `%t`\n", report.LLME2ERequiredAfterSecurityChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report is the GitHub-native companion to OpenClaw-style security audit posture. It assumes one trusted operator boundary, aggregates repo-local config, policy, sandbox, channel, tool, skill, plugin, and secret risk metadata, and keeps issue bodies, comments, prompts, workflow bodies, tool outputs, credentials, and secret values out of the report.\n\n")

	b.WriteString("### Trust Boundary\n")
	fmt.Fprintf(&b, "- trust_model=`%s` hostile_multi_tenant_supported=`%t` gateway_server_required=`%t`\n", report.TrustModel, report.HostileMultiTenantSupported, report.GatewayServerRequired)
	fmt.Fprintf(&b, "- trusted_associations=`%s` broad_trusted_associations=`%s` trigger_mode=`%s`\n", inlineListOrNone(report.ConfigRisk.TrustedAssociations), inlineListOrNone(report.ConfigRisk.BroadTrustedAssociations), report.ConfigRisk.TriggerMode)
	b.WriteString("- session_keys_are_routing_not_authorization=`true`\n")
	b.WriteString("- split_trust_boundaries_for_untrusted_users=`true`\n")

	b.WriteString("\n### Surface Cards\n")
	for _, summary := range securitySurfaceSummaries(report) {
		fmt.Fprintf(&b, "- surface=`%s` status=`%s` findings=`%d` high=`%d` warning=`%d` info=`%d` notes=`%s`\n", summary.Name, summary.Status, summary.Findings, summary.High, summary.Warning, summary.Info, inlineListOrNone(summary.Notes))
	}

	b.WriteString("\n### Control Plane Gates\n")
	fmt.Fprintf(&b, "- workflow_files_present=`%d/%d` unexpected_write_permissions=`%d` backup_concurrency_cancel_safe=`%t`\n", report.ConfigRisk.WorkflowFilesPresent, report.ConfigRisk.WorkflowFilesExpected, report.PolicyRisk.UnexpectedWritePermissions, report.PolicyRisk.BackupConcurrencyCancelSafe)
	fmt.Fprintf(&b, "- shell_execution_allowed=`%t` repository_mutation_allowed=`%t` mutating_tool_contracts=`%d`\n", report.SandboxRisk.ShellExecutionAllowed, report.SandboxRisk.RepositoryMutationAllowed, report.SandboxRisk.MutatingToolContracts)
	fmt.Fprintf(&b, "- channel_workflows_present=`%d/%d` wake_strategy=`%s` gateway_runtime=`%s`\n", report.ChannelRisk.PresentWorkflows, report.ChannelRisk.ScannedWorkflows, report.ChannelRisk.WakeStrategy, report.ChannelRisk.GatewayRuntime)
	fmt.Fprintf(&b, "- plaintext_secret_findings=`%d` github_actions_secret_references=`%d` github_secret_values_resolved=`%t`\n", report.SecretRisk.PlaintextSecretFindings, report.SecretRisk.GitHubActionsSecretRefs, report.SecretRisk.GitHubSecretValuesResolved)

	b.WriteString("\n### Audit Boundaries\n")
	b.WriteString("- model_call_required=`false`\n")
	b.WriteString("- repository_mutation_allowed=`false`\n")
	b.WriteString("- raw_bodies_included=`false`\n")
	b.WriteString("- credential_values_included=`false`\n")
	b.WriteString("- exact_secrets_or_provider_tokens_included=`false`\n")
	return strings.TrimSpace(b.String())
}

func securitySurfaceSummaries(report SecurityAuditReport) []securitySurfaceSummary {
	return []securitySurfaceSummary{
		{
			Name:     "config",
			Status:   report.ConfigRisk.Status,
			High:     report.ConfigRisk.HighRiskFindings,
			Warning:  report.ConfigRisk.WarningRiskFindings,
			Info:     report.ConfigRisk.InfoRiskFindings,
			Findings: len(report.ConfigRisk.Findings),
			Notes:    []string{fmt.Sprintf("workflows=%d/%d", report.ConfigRisk.WorkflowFilesPresent, report.ConfigRisk.WorkflowFilesExpected), fmt.Sprintf("broad_trust=%d", report.ConfigRisk.BroadTrustedAssociationsConfigured)},
		},
		{
			Name:     "policy",
			Status:   report.PolicyRisk.Status,
			High:     report.PolicyRisk.HighRiskFindings,
			Warning:  report.PolicyRisk.WarningRiskFindings,
			Info:     report.PolicyRisk.InfoRiskFindings,
			Findings: len(report.PolicyRisk.Findings),
			Notes:    []string{fmt.Sprintf("unexpected_write=%d", report.PolicyRisk.UnexpectedWritePermissions), fmt.Sprintf("write_enabled=%t", report.PolicyRisk.WriteActionsEnabled)},
		},
		{
			Name:     "sandbox",
			Status:   report.SandboxRisk.Status,
			High:     report.SandboxRisk.HighRiskFindings,
			Warning:  report.SandboxRisk.WarningRiskFindings,
			Info:     report.SandboxRisk.InfoRiskFindings,
			Findings: len(report.SandboxRisk.Findings),
			Notes:    []string{fmt.Sprintf("host_exec=%t", report.SandboxRisk.ShellExecutionAllowed), fmt.Sprintf("mutating_tools=%d", report.SandboxRisk.MutatingToolContracts)},
		},
		{
			Name:     "channels",
			Status:   report.ChannelRisk.Status,
			High:     report.ChannelRisk.HighRiskFindings,
			Warning:  report.ChannelRisk.WarningRiskFindings,
			Info:     report.ChannelRisk.InfoRiskFindings,
			Findings: len(report.ChannelRisk.Findings),
			Notes:    []string{fmt.Sprintf("workflows=%d/%d", report.ChannelRisk.PresentWorkflows, report.ChannelRisk.ScannedWorkflows), report.ChannelRisk.WakeStrategy},
		},
		{
			Name:     "tools",
			Status:   report.ToolRisk.Status,
			High:     report.ToolRisk.HighRiskFindings,
			Warning:  report.ToolRisk.WarningRiskFindings,
			Info:     report.ToolRisk.InfoRiskFindings,
			Findings: len(report.ToolRisk.Findings),
			Notes:    []string{fmt.Sprintf("available=%d", report.ToolRisk.AvailableTools), fmt.Sprintf("shell=%t", report.ToolRisk.ShellExecutionAllowed)},
		},
		{
			Name:     "skills",
			Status:   report.SkillRisk.Status,
			High:     report.SkillRisk.HighRiskFindings,
			Warning:  report.SkillRisk.WarningRiskFindings,
			Info:     report.SkillRisk.InfoRiskFindings,
			Findings: len(report.SkillRisk.Findings),
			Notes:    []string{fmt.Sprintf("skills=%d", report.SkillRisk.Skills), fmt.Sprintf("installer_scripts=%t", report.SkillRisk.InstallerScriptsRun)},
		},
		{
			Name:     "plugins",
			Status:   report.PluginRisk.Status,
			High:     report.PluginRisk.HighRiskFindings,
			Warning:  report.PluginRisk.WarningRiskFindings,
			Info:     report.PluginRisk.InfoRiskFindings,
			Findings: len(report.PluginRisk.Findings),
			Notes:    []string{fmt.Sprintf("specs=%d", report.PluginRisk.PluginSpecs), fmt.Sprintf("execution_allowed=%t", report.PluginRisk.PluginExecutionAllowed)},
		},
		{
			Name:     "secrets",
			Status:   report.SecretRisk.Status,
			High:     report.SecretRisk.HighSeverityFindings,
			Warning:  report.SecretRisk.MediumSeverityFindings,
			Info:     0,
			Findings: report.SecretRisk.PlaintextSecretFindings,
			Notes:    []string{fmt.Sprintf("secret_refs=%d", report.SecretRisk.GitHubActionsSecretRefs), fmt.Sprintf("values_resolved=%t", report.SecretRisk.GitHubSecretValuesResolved)},
		},
	}
}

func securityAuditStatus(report SecurityAuditReport) string {
	if report.HighRiskFindings > 0 {
		return "high"
	}
	if report.WarningRiskFindings > 0 || report.SecretRisk.Status == "reference_review" {
		return "review"
	}
	if report.SurfacesWithFindings > 0 {
		return "review"
	}
	return "ok"
}
