package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SandboxRiskReport struct {
	Status                               string
	VerificationScope                    string
	RuntimeBoundary                      string
	SandboxBackend                       string
	HostExecPolicy                       string
	ShellExecutionAllowed                bool
	RepositoryMutationAllowed            bool
	WriteMode                            string
	ApprovalMode                         string
	ApprovalStore                        string
	ElevatedModeAvailable                bool
	SkillCLIAutoAllow                    bool
	InlineEvalPolicy                     string
	NetworkEgressPolicy                  string
	AvailableTools                       int
	EnabledTools                         int
	DisabledTools                        int
	AllowlistBlockedTools                int
	ReadOnlyToolContracts                int
	MetadataOnlyToolContracts            int
	MutatingToolContracts                int
	ActiveToolOutputs                    int
	ToolValidationStatus                 string
	ToolValidationErrors                 int
	ToolValidationWarnings               int
	WorkflowPermissionStatus             string
	WorkflowPresent                      bool
	UnexpectedWritePermissions           int
	BackupWritePermissionScope           string
	BackupConcurrencyGroup               bool
	BackupConcurrencyCancelSafe          bool
	SkillRequiredBins                    int
	SkillMissingBins                     int
	SurfacesWithRiskFindings             int
	Findings                             []SandboxRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	RawPromptBodiesIncluded              bool
	RawWorkflowBodiesIncluded            bool
	RawToolOutputsIncluded               bool
	SecretsIncluded                      bool
	LLME2ERequiredAfterSandboxRiskChange bool
}

type SandboxRiskFinding struct {
	Severity string
	Code     string
	Category string
	Surface  string
	Field    string
	Evidence string
}

func BuildSandboxRiskReport(cfg Config, repoContext RepoContext) SandboxRiskReport {
	toolValidation := ValidateTools(repoContext)
	policyVerify := BuildPolicyVerifyReport(cfg, repoContext)
	requiredBins, missingBins := skillRequiredBinCounts(repoContext.SkillSummaries)
	report := SandboxRiskReport{
		Status:                               "ok",
		VerificationScope:                    "github_actions_sandbox_boundary",
		RuntimeBoundary:                      "github-actions-ephemeral-runner",
		SandboxBackend:                       "github-actions",
		HostExecPolicy:                       "deny",
		ShellExecutionAllowed:                false,
		RepositoryMutationAllowed:            false,
		WriteMode:                            "read-only",
		ApprovalMode:                         "not_applicable_no_exec_tool",
		ApprovalStore:                        "not_configured",
		ElevatedModeAvailable:                false,
		SkillCLIAutoAllow:                    false,
		InlineEvalPolicy:                     "not_applicable_no_exec_tool",
		NetworkEgressPolicy:                  "github-actions-default",
		AvailableTools:                       len(toolReportContracts),
		EnabledTools:                         enabledToolCount(repoContext),
		DisabledTools:                        disabledToolCount(repoContext),
		AllowlistBlockedTools:                allowlistBlockedToolCount(repoContext),
		ReadOnlyToolContracts:                readOnlyToolContractCount(),
		MetadataOnlyToolContracts:            metadataOnlyToolContractCount(),
		MutatingToolContracts:                mutatingToolContractCount(),
		ActiveToolOutputs:                    len(repoContext.ToolOutputs),
		ToolValidationStatus:                 toolValidation.Status,
		ToolValidationErrors:                 toolValidation.Errors,
		ToolValidationWarnings:               toolValidation.Warnings,
		WorkflowPermissionStatus:             policyVerify.Status,
		WorkflowPresent:                      policyVerify.WorkflowPresent,
		UnexpectedWritePermissions:           policyVerify.UnexpectedWritePermissions,
		BackupWritePermissionScope:           "backup-job-only",
		BackupConcurrencyGroup:               policyVerify.BackupConcurrencyGroup,
		BackupConcurrencyCancelSafe:          policyVerify.BackupConcurrencyCancelSafe,
		SkillRequiredBins:                    requiredBins,
		SkillMissingBins:                     missingBins,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		RawPromptBodiesIncluded:              false,
		RawWorkflowBodiesIncluded:            false,
		RawToolOutputsIncluded:               false,
		SecretsIncluded:                      false,
		LLME2ERequiredAfterSandboxRiskChange: true,
	}
	report.Findings = buildSandboxRiskFindings(report, toolValidation, policyVerify)
	sortSandboxRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = sandboxRiskSurfaceCount(report.Findings)
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

func RenderSandboxRiskReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSandboxRiskReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Sandbox Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
		fmt.Fprintf(&b, "- event_action: `%s`\n", ev.Action)
		fmt.Fprintf(&b, "- active_command: `%s`\n", activeSlashCommand(ev, cfg))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- sandbox_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(&b, "- runtime_boundary: `%s`\n", report.RuntimeBoundary)
	fmt.Fprintf(&b, "- sandbox_backend: `%s`\n", report.SandboxBackend)
	fmt.Fprintf(&b, "- host_exec_policy: `%s`\n", report.HostExecPolicy)
	fmt.Fprintf(&b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- write_mode: `%s`\n", report.WriteMode)
	fmt.Fprintf(&b, "- approval_mode: `%s`\n", report.ApprovalMode)
	fmt.Fprintf(&b, "- approval_store: `%s`\n", report.ApprovalStore)
	fmt.Fprintf(&b, "- elevated_mode_available: `%t`\n", report.ElevatedModeAvailable)
	fmt.Fprintf(&b, "- skill_cli_auto_allow: `%t`\n", report.SkillCLIAutoAllow)
	fmt.Fprintf(&b, "- inline_eval_policy: `%s`\n", report.InlineEvalPolicy)
	fmt.Fprintf(&b, "- network_egress_policy: `%s`\n", report.NetworkEgressPolicy)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", report.EnabledTools)
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", report.DisabledTools)
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", report.AllowlistBlockedTools)
	fmt.Fprintf(&b, "- read_only_tool_contracts: `%d`\n", report.ReadOnlyToolContracts)
	fmt.Fprintf(&b, "- metadata_only_tool_contracts: `%d`\n", report.MetadataOnlyToolContracts)
	fmt.Fprintf(&b, "- mutating_tool_contracts: `%d`\n", report.MutatingToolContracts)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", report.ToolValidationStatus)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", report.ToolValidationErrors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", report.ToolValidationWarnings)
	fmt.Fprintf(&b, "- workflow_permission_status: `%s`\n", report.WorkflowPermissionStatus)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", report.WorkflowPresent)
	fmt.Fprintf(&b, "- unexpected_write_permissions: `%d`\n", report.UnexpectedWritePermissions)
	fmt.Fprintf(&b, "- backup_write_permission_scope: `%s`\n", report.BackupWritePermissionScope)
	fmt.Fprintf(&b, "- backup_concurrency_group: `%t`\n", report.BackupConcurrencyGroup)
	fmt.Fprintf(&b, "- backup_concurrency_cancel_safe: `%t`\n", report.BackupConcurrencyCancelSafe)
	fmt.Fprintf(&b, "- skill_required_bins: `%d`\n", report.SkillRequiredBins)
	fmt.Fprintf(&b, "- skill_missing_bins: `%d`\n", report.SkillMissingBins)
	fmt.Fprintf(&b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(&b, "- sandbox_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- secrets_included: `%t`\n", report.SecretsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_sandbox_risk_change: `%t`\n", report.LLME2ERequiredAfterSandboxRiskChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits GitClaw's current execution boundary. It reports sandbox, tool, skill, and workflow-permission metadata only; issue bodies, comments, prompts, workflow bodies, tool output bodies, and secrets are not included.\n\n")

	b.WriteString("### Runtime Boundary Risk Card\n")
	writeSandboxRiskCard(&b, "runtime-boundary", report.Findings, func(f SandboxRiskFinding) bool { return f.Surface == "runtime-boundary" }, []string{
		fmt.Sprintf("runtime_boundary=`%s`", report.RuntimeBoundary),
		fmt.Sprintf("sandbox_backend=`%s`", report.SandboxBackend),
		fmt.Sprintf("host_exec_policy=`%s`", report.HostExecPolicy),
		fmt.Sprintf("shell_execution_allowed=`%t`", report.ShellExecutionAllowed),
		fmt.Sprintf("repository_mutation_allowed=`%t`", report.RepositoryMutationAllowed),
		fmt.Sprintf("elevated_mode_available=`%t`", report.ElevatedModeAvailable),
	})

	b.WriteString("\n### Tool Contract Risk Card\n")
	writeSandboxRiskCard(&b, "tool-contract", report.Findings, func(f SandboxRiskFinding) bool { return f.Surface == "tool-contract" }, []string{
		fmt.Sprintf("available_tools=`%d`", report.AvailableTools),
		fmt.Sprintf("enabled_tools=`%d`", report.EnabledTools),
		fmt.Sprintf("read_only_tool_contracts=`%d`", report.ReadOnlyToolContracts),
		fmt.Sprintf("metadata_only_tool_contracts=`%d`", report.MetadataOnlyToolContracts),
		fmt.Sprintf("mutating_tool_contracts=`%d`", report.MutatingToolContracts),
		fmt.Sprintf("active_tool_outputs=`%d`", report.ActiveToolOutputs),
		fmt.Sprintf("tool_validation_status=`%s`", report.ToolValidationStatus),
	})

	b.WriteString("\n### Workflow Permission Risk Card\n")
	writeSandboxRiskCard(&b, "workflow-permission", report.Findings, func(f SandboxRiskFinding) bool { return f.Surface == "workflow-permission" }, []string{
		fmt.Sprintf("workflow_present=`%t`", report.WorkflowPresent),
		fmt.Sprintf("workflow_permission_status=`%s`", report.WorkflowPermissionStatus),
		fmt.Sprintf("unexpected_write_permissions=`%d`", report.UnexpectedWritePermissions),
		fmt.Sprintf("backup_write_permission_scope=`%s`", report.BackupWritePermissionScope),
		fmt.Sprintf("backup_concurrency_group=`%t`", report.BackupConcurrencyGroup),
		fmt.Sprintf("backup_concurrency_cancel_safe=`%t`", report.BackupConcurrencyCancelSafe),
	})

	b.WriteString("\n### Skill Runtime Risk Card\n")
	writeSandboxRiskCard(&b, "skill-runtime", report.Findings, func(f SandboxRiskFinding) bool { return f.Surface == "skill-runtime" }, []string{
		fmt.Sprintf("skill_cli_auto_allow=`%t`", report.SkillCLIAutoAllow),
		fmt.Sprintf("skill_required_bins=`%d`", report.SkillRequiredBins),
		fmt.Sprintf("skill_missing_bins=`%d`", report.SkillMissingBins),
	})

	b.WriteString("\n### Risk Findings\n")
	writeSandboxRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func requestedSandboxRisk(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	switch fields[0] {
	case "/sandbox", "/sandboxes", "/exec-policy":
		return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
	default:
		return false
	}
}

func buildSandboxRiskFindings(report SandboxRiskReport, tools ToolValidationReport, policy policyVerifyReport) []SandboxRiskFinding {
	var findings []SandboxRiskFinding
	add := func(severity, code, category, surface, field, evidence string) {
		if evidence == "" {
			evidence = shortDocumentHash(surface + ":" + field + ":" + code)
		}
		findings = append(findings, SandboxRiskFinding{Severity: severity, Code: code, Category: category, Surface: surface, Field: field, Evidence: evidence})
	}
	if report.ShellExecutionAllowed {
		add("high", "shell_execution_enabled", "host-exec", "runtime-boundary", "shell_execution_allowed", "")
	}
	if report.RepositoryMutationAllowed {
		add("high", "repository_mutation_enabled", "mutation", "runtime-boundary", "repository_mutation_allowed", "")
	}
	if report.ElevatedModeAvailable {
		add("high", "elevated_mode_available", "host-exec", "runtime-boundary", "elevated_mode_available", "")
	}
	if report.MutatingToolContracts > 0 {
		add("high", "mutating_tool_contracts_present", "tool-policy", "tool-contract", "mutating_tool_contracts", "")
	}
	if tools.Errors > 0 {
		add("high", "tool_validation_errors", "tool-policy", "tool-contract", "tool_validation_errors", sandboxRiskEvidenceForToolFindings(tools.Findings, "error"))
	}
	if tools.Warnings > 0 {
		add("warning", "tool_validation_warnings", "tool-policy", "tool-contract", "tool_validation_warnings", sandboxRiskEvidenceForToolFindings(tools.Findings, "warning"))
	}
	if !report.WorkflowPresent {
		add("high", "workflow_missing", "workflow-permission", "workflow-permission", "workflow_present", "")
	}
	if policy.Status == "error" {
		add("high", "workflow_permission_contract_failed", "workflow-permission", "workflow-permission", "workflow_permission_status", sandboxRiskEvidenceForPolicyFindings(policy.Findings))
	}
	if report.UnexpectedWritePermissions > 0 {
		add("high", "unexpected_write_permissions", "workflow-permission", "workflow-permission", "unexpected_write_permissions", sandboxRiskEvidenceForPolicyFindings(policy.Findings))
	}
	if report.SkillMissingBins > 0 {
		add("warning", "skill_required_bins_missing", "skill-runtime", "skill-runtime", "skill_missing_bins", "")
	}
	return findings
}

func writeSandboxRiskCard(b *strings.Builder, kind string, findings []SandboxRiskFinding, match func(SandboxRiskFinding) bool, fields []string) {
	filtered := filterSandboxRiskFindings(findings, match)
	fmt.Fprintf(b, "- kind=`%s` %s risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` evidence_hashes=`%s`\n",
		kind,
		strings.Join(fields, " "),
		len(filtered),
		sandboxRiskMaxSeverity(filtered),
		inlineListOrNone(sandboxRiskCodes(filtered)),
		inlineListOrNone(sandboxRiskEvidenceHashes(filtered)),
	)
}

func writeSandboxRiskFindings(b *strings.Builder, findings []SandboxRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` surface=`%s` field=`%s` evidence_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Surface, finding.Field, finding.Evidence)
	}
}

func filterSandboxRiskFindings(findings []SandboxRiskFinding, match func(SandboxRiskFinding) bool) []SandboxRiskFinding {
	var filtered []SandboxRiskFinding
	for _, finding := range findings {
		if match(finding) {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func sandboxRiskSurfaceCount(findings []SandboxRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.Surface] = true
	}
	return len(seen)
}

func sandboxRiskCodes(findings []SandboxRiskFinding) []string {
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

func sandboxRiskEvidenceHashes(findings []SandboxRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.Evidence == "" || seen[finding.Evidence] {
			continue
		}
		seen[finding.Evidence] = true
		hashes = append(hashes, finding.Evidence)
	}
	sort.Strings(hashes)
	return hashes
}

func sandboxRiskMaxSeverity(findings []SandboxRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if sandboxRiskSeverityRank(finding.Severity) > sandboxRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func sandboxRiskSeverityRank(severity string) int {
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

func sortSandboxRiskFindings(findings []SandboxRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if sandboxRiskSeverityRank(findings[i].Severity) != sandboxRiskSeverityRank(findings[j].Severity) {
			return sandboxRiskSeverityRank(findings[i].Severity) > sandboxRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Surface != findings[j].Surface {
			return findings[i].Surface < findings[j].Surface
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Field < findings[j].Field
	})
}

func sandboxRiskEvidenceForToolFindings(findings []ToolValidationFinding, severity string) string {
	var parts []string
	for _, finding := range findings {
		if finding.Severity != severity {
			continue
		}
		parts = append(parts, finding.Code+":"+finding.Name)
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return shortDocumentHash(strings.Join(parts, "\n"))
}

func sandboxRiskEvidenceForPolicyFindings(findings []policyVerifyFinding) string {
	var parts []string
	for _, finding := range findings {
		parts = append(parts, finding.Code+":"+finding.Job+":"+finding.Permission)
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return shortDocumentHash(strings.Join(parts, "\n"))
}
