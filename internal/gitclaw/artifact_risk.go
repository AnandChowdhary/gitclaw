package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ArtifactRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Name     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type ArtifactRiskReport struct {
	Status                                string
	VerificationScope                     string
	ArtifactPolicyPresent                 bool
	ArtifactPolicyLoadedForModel          bool
	ArtifactSpecs                         int
	ScannedArtifactSpecs                  int
	WorkflowArtifactUploaders             int
	ScannedWorkflowArtifactUploaders      int
	ArtifactSpecsRequiringApproval        int
	ArtifactSpecsRequiringRedaction       int
	ArtifactRetentionDaysDeclared         int
	PromptArtifactDefaultEnabled          bool
	PromptArtifactLabel                   string
	PromptArtifactEnvPathConfigured       bool
	ArtifactStorageBackend                string
	DurableBackupBackend                  string
	SurfacesWithRiskFindings              int
	Findings                              []ArtifactRiskFinding
	HighRiskFindings                      int
	WarningRiskFindings                   int
	InfoRiskFindings                      int
	ArtifactBodyPrintingAllowed           bool
	RawArtifactBodiesIncluded             bool
	RawIssueBodiesIncluded                bool
	RawCommentBodiesIncluded              bool
	CredentialValuesIncluded              bool
	ArtifactAsHiddenStateAllowed          bool
	ExternalArtifactStorageAllowed        bool
	RepositoryMutationAllowed             bool
	LLME2ERequiredAfterArtifactRiskChange bool
}

type artifactRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var artifactTextRiskRules = []artifactRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
			"developer message",
		},
	},
	{
		Severity: "high",
		Code:     "credential_material_in_artifact",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"telegram_bot_token=",
			"slack_bot_token=",
			"slack_app_token=",
			"xoxb-",
			"xapp-",
			"api_key=",
			"private_key=",
			"-----begin private key-----",
			"-----begin openssh private key-----",
		},
	},
	{
		Severity: "high",
		Code:     "unredacted_prompt_artifact",
		Category: "data-leakage",
		Any: []string{
			"redaction_required: false",
			"upload unredacted prompt",
			"upload raw prompt",
			"raw prompt artifact",
			"unredacted prompt artifact",
			"include full prompt body",
			"include raw tool output",
		},
		IgnoreAny: []string{
			"redaction_required: true",
			"do not upload unredacted",
			"must not upload unredacted",
		},
	},
	{
		Severity: "high",
		Code:     "raw_artifact_payload_logged",
		Category: "body-leakage",
		Any: []string{
			"cat $gitclaw_prompt_artifact_path",
			"cat \"$gitclaw_prompt_artifact_path",
			"cat ${gitclaw_prompt_artifact_path",
			"cat ${{ runner.temp }}/gitclaw-prompt-artifacts/prompt.md",
			"echo \"$prompt",
			"echo \"$issue_body",
			"echo \"${{ github.event.issue.body",
			"printf \"%s\" \"$prompt",
			"printf '%s' \"$prompt",
			"printenv",
		},
	},
	{
		Severity: "high",
		Code:     "artifact_hidden_state",
		Category: "state-boundary",
		Any: []string{
			"use artifacts as hidden state",
			"use artifact as hidden state",
			"artifacts are durable memory",
			"artifact is durable memory",
			"load prior artifact as context",
			"load artifact next turn",
			"conversation transcript artifact is source of truth",
		},
	},
	{
		Severity: "warning",
		Code:     "external_artifact_storage",
		Category: "storage-boundary",
		Any: []string{
			"storage: s3",
			"storage: gcs",
			"storage: supabase",
			"s3://",
			"gs://",
			"aws s3 cp",
			"curl -t ",
			"curl --upload-file",
			"scp ",
		},
	},
	{
		Severity: "warning",
		Code:     "unreviewed_repository_mutation",
		Category: "write-authority",
		Any: []string{
			"git push",
			"git commit",
			"gh issue edit",
			"gh workflow run",
			"write files without review",
			"modify files without review",
			"commit directly",
			"push directly",
		},
		IgnoreAny: []string{
			"back up gitclaw issue",
			"backup_branch",
			"gitclaw-backups",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_artifact_collection",
		Category: "runtime-amplification",
		Any: []string{
			"while true",
			"retry forever",
			"loop forever",
			"sleep infinity",
			"never stop",
			"continue indefinitely",
		},
	},
}

func renderArtifactRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectArtifactSurface(cfg.Workdir)
	report := BuildArtifactRiskReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Artifact Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeArtifactRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans artifact policy, artifact specs, and workflow upload metadata for prompt-boundary, credential, unredacted prompt, raw payload logging, hidden-state, external storage, repository mutation, retention, label-gate, missing-file, and unbounded-collection risks. It reports metadata, paths, risk codes, severities, and hashes only; artifact bodies, issue bodies, comments, uploaded files, prompt bodies, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Artifact Policy Risk Card\n")
	writeArtifactPolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Artifact Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`artifact-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeArtifactSpecRiskCard(&b, cfg.Workdir, spec, surface.Workflows)
		}
	}

	b.WriteString("\n### Workflow Artifact Risk Cards\n")
	if len(surface.Workflows) == 0 {
		b.WriteString("- kind=`artifact-workflow` none\n")
	} else {
		for _, workflow := range surface.Workflows {
			writeArtifactWorkflowRiskCard(&b, cfg.Workdir, workflow)
		}
	}

	b.WriteString("\n### Current Artifact Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-artifact-request` current_issue_artifact_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-artifact-request` scope=`local-cli` current_issue_artifact_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeArtifactRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildArtifactRiskReport(cfg Config) ArtifactRiskReport {
	surface := inspectArtifactSurface(cfg.Workdir)
	report := ArtifactRiskReport{
		Status:                                "ok",
		VerificationScope:                     "github_actions_artifact_metadata",
		ArtifactPolicyPresent:                 surface.Policy.Present,
		ArtifactPolicyLoadedForModel:          artifactPolicyLoadedForModel(surface),
		ArtifactSpecs:                         len(surface.Specs),
		WorkflowArtifactUploaders:             len(surface.Workflows),
		ArtifactSpecsRequiringApproval:        artifactSpecsRequiringApproval(surface.Specs),
		ArtifactSpecsRequiringRedaction:       artifactSpecsRequiringRedaction(surface.Specs),
		ArtifactRetentionDaysDeclared:         artifactRetentionDaysDeclared(surface.Specs),
		PromptArtifactDefaultEnabled:          false,
		PromptArtifactLabel:                   "gitclaw:e2e-prompt-artifact",
		PromptArtifactEnvPathConfigured:       strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "",
		ArtifactStorageBackend:                "github-actions-artifacts",
		DurableBackupBackend:                  "git-backup-branch",
		ArtifactBodyPrintingAllowed:           false,
		RawArtifactBodiesIncluded:             false,
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		CredentialValuesIncluded:              false,
		ArtifactAsHiddenStateAllowed:          false,
		ExternalArtifactStorageAllowed:        false,
		RepositoryMutationAllowed:             false,
		LLME2ERequiredAfterArtifactRiskChange: true,
	}
	report.Findings = append(report.Findings, scanArtifactPolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedArtifactSpecs++
		report.Findings = append(report.Findings, scanArtifactSpecRiskFindings(cfg.Workdir, spec, surface.Workflows)...)
	}
	for _, workflow := range surface.Workflows {
		report.ScannedWorkflowArtifactUploaders++
		report.Findings = append(report.Findings, scanArtifactWorkflowRiskFindings(cfg.Workdir, workflow)...)
	}
	sortArtifactRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = artifactRiskSurfaceCount(report.Findings)
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

func writeArtifactRiskSummary(b *strings.Builder, report ArtifactRiskReport) {
	fmt.Fprintf(b, "- artifact_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- artifact_policy_present: `%t`\n", report.ArtifactPolicyPresent)
	fmt.Fprintf(b, "- artifact_policy_loaded_for_model: `%t`\n", report.ArtifactPolicyLoadedForModel)
	fmt.Fprintf(b, "- artifact_specs: `%d`\n", report.ArtifactSpecs)
	fmt.Fprintf(b, "- scanned_artifact_specs: `%d`\n", report.ScannedArtifactSpecs)
	fmt.Fprintf(b, "- workflow_artifact_uploaders: `%d`\n", report.WorkflowArtifactUploaders)
	fmt.Fprintf(b, "- scanned_workflow_artifact_uploaders: `%d`\n", report.ScannedWorkflowArtifactUploaders)
	fmt.Fprintf(b, "- artifact_specs_requiring_approval: `%d`\n", report.ArtifactSpecsRequiringApproval)
	fmt.Fprintf(b, "- artifact_specs_requiring_redaction: `%d`\n", report.ArtifactSpecsRequiringRedaction)
	fmt.Fprintf(b, "- artifact_retention_days_declared: `%d`\n", report.ArtifactRetentionDaysDeclared)
	fmt.Fprintf(b, "- prompt_artifact_default_enabled: `%t`\n", report.PromptArtifactDefaultEnabled)
	fmt.Fprintf(b, "- prompt_artifact_label: `%s`\n", report.PromptArtifactLabel)
	fmt.Fprintf(b, "- prompt_artifact_env_path_configured: `%t`\n", report.PromptArtifactEnvPathConfigured)
	fmt.Fprintf(b, "- artifact_storage_backend: `%s`\n", report.ArtifactStorageBackend)
	fmt.Fprintf(b, "- durable_backup_backend: `%s`\n", report.DurableBackupBackend)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- artifact_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- artifact_body_printing_allowed: `%t`\n", report.ArtifactBodyPrintingAllowed)
	fmt.Fprintf(b, "- raw_artifact_bodies_included: `%t`\n", report.RawArtifactBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- artifact_as_hidden_state_allowed: `%t`\n", report.ArtifactAsHiddenStateAllowed)
	fmt.Fprintf(b, "- external_artifact_storage_allowed: `%t`\n", report.ExternalArtifactStorageAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_artifact_risk_change: `%t`\n", report.LLME2ERequiredAfterArtifactRiskChange)
}

func writeArtifactPolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanArtifactPolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`artifact-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			artifactRiskMaxSeverity(findings),
			inlineListOrNone(artifactRiskCodes(findings)),
			inlineListOrNone(artifactRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`artifact-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		artifactPolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		artifactRiskMaxSeverity(findings),
		inlineListOrNone(artifactRiskCodes(findings)),
		inlineListOrNone(artifactRiskLineHashes(findings)),
	)
}

func writeArtifactSpecRiskCard(b *strings.Builder, root string, spec artifactSpecCard, workflows []artifactWorkflowCard) {
	findings := scanArtifactSpecRiskFindings(root, spec, workflows)
	fmt.Fprintf(
		b,
		"- kind=`artifact-spec` name=`%s` path=`%s` frontmatter=`%t` artifact_kind=`%s` storage=`%s` filename=`%s` workflow=`%s` label=`%s` retention_days=`%d` redaction_required=`%t` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Kind),
		inlineCode(spec.Storage),
		inlineCode(spec.Filename),
		inlineCode(spec.Workflow),
		inlineCode(spec.Label),
		spec.RetentionDays,
		spec.RedactionRequired,
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		artifactRiskMaxSeverity(findings),
		inlineListOrNone(artifactRiskCodes(findings)),
		inlineListOrNone(artifactRiskLineHashes(findings)),
	)
}

func writeArtifactWorkflowRiskCard(b *strings.Builder, root string, workflow artifactWorkflowCard) {
	findings := scanArtifactWorkflowRiskFindings(root, workflow)
	fmt.Fprintf(
		b,
		"- kind=`artifact-workflow` path=`%s` upload_artifact_actions=`%s` retention_days=`%s` if_no_files_found_error=`%t` prompt_artifact_label_gate=`%t` prompt_artifact_path_env=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		workflow.Path,
		inlineCode(strings.Join(workflow.UploadArtifactActions, ", ")),
		inlineCode(joinInts(workflow.RetentionDays)),
		workflow.IfNoFilesFoundError,
		workflow.PromptArtifactLabelGate,
		workflow.PromptArtifactPathEnv,
		workflow.Bytes,
		workflow.Lines,
		workflow.SHA,
		len(findings),
		artifactRiskMaxSeverity(findings),
		inlineListOrNone(artifactRiskCodes(findings)),
		inlineListOrNone(artifactRiskLineHashes(findings)),
	)
}

func scanArtifactPolicyRiskFindings(root string, policy configSurfaceFile) []ArtifactRiskFinding {
	var findings []ArtifactRiskFinding
	if !policy.Present {
		findings = append(findings, ArtifactRiskFinding{
			Severity: "info",
			Code:     "artifact_policy_not_configured",
			Category: "policy",
			Kind:     "artifact-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !artifactPolicyPathInContext() {
		findings = append(findings, ArtifactRiskFinding{
			Severity: "high",
			Code:     "artifact_policy_not_loaded",
			Category: "context",
			Kind:     "artifact-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanArtifactRiskText("artifact-policy", "policy", policy.Path, "body", readArtifactRiskBody(root, policy.Path))...)
	sortArtifactRiskFindings(findings)
	return findings
}

func scanArtifactSpecRiskFindings(root string, spec artifactSpecCard, workflows []artifactWorkflowCard) []ArtifactRiskFinding {
	var findings []ArtifactRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if strings.TrimSpace(spec.Kind) == "" {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_kind_missing", "metadata", spec, "kind"))
	}
	if !strings.EqualFold(spec.Storage, "github-actions-artifact") {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_storage_not_actions", "storage-boundary", spec, "storage"))
	}
	if strings.TrimSpace(spec.Filename) == "" {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_filename_missing", "metadata", spec, "filename"))
	}
	if strings.TrimSpace(spec.Workflow) == "" {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_workflow_missing", "workflow", spec, "workflow"))
	} else if !artifactWorkflowUploads(workflows, spec.Workflow) {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_workflow_upload_missing", "workflow", spec, "workflow"))
	}
	if strings.TrimSpace(spec.Label) == "" {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_label_missing", "approval", spec, "label"))
	}
	if spec.RetentionDays <= 0 {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_retention_missing", "retention", spec, "retention_days"))
	} else if spec.RetentionDays > 30 {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_retention_too_long", "retention", spec, "retention_days"))
	}
	if !spec.RedactionRequired {
		findings = append(findings, artifactSpecMetadataRiskFinding("high", "artifact_redaction_not_required", "data-leakage", spec, "redaction_required"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, artifactSpecMetadataRiskFinding("warning", "artifact_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanArtifactRiskText("artifact-spec", spec.Name, spec.Path, "body", readArtifactRiskBody(root, spec.Path))...)
	sortArtifactRiskFindings(findings)
	return findings
}

func scanArtifactWorkflowRiskFindings(root string, workflow artifactWorkflowCard) []ArtifactRiskFinding {
	var findings []ArtifactRiskFinding
	if !artifactWorkflowUsesUploadV6(workflow) {
		findings = append(findings, artifactWorkflowMetadataRiskFinding("warning", "artifact_upload_action_not_v6", "workflow", workflow, "upload_artifact_actions"))
	}
	if len(workflow.RetentionDays) == 0 {
		findings = append(findings, artifactWorkflowMetadataRiskFinding("warning", "artifact_workflow_retention_missing", "retention", workflow, "retention_days"))
	} else {
		for _, days := range workflow.RetentionDays {
			if days > 30 {
				findings = append(findings, artifactWorkflowMetadataRiskFinding("warning", "artifact_workflow_retention_too_long", "retention", workflow, "retention_days"))
				break
			}
		}
	}
	if !workflow.IfNoFilesFoundError {
		findings = append(findings, artifactWorkflowMetadataRiskFinding("warning", "artifact_workflow_missing_if_no_files_found_error", "workflow", workflow, "if_no_files_found"))
	}
	if !workflow.PromptArtifactLabelGate {
		findings = append(findings, artifactWorkflowMetadataRiskFinding("high", "artifact_workflow_missing_label_gate", "approval", workflow, "prompt_artifact_label_gate"))
	}
	if !workflow.PromptArtifactPathEnv {
		findings = append(findings, artifactWorkflowMetadataRiskFinding("warning", "artifact_workflow_missing_prompt_path_env", "workflow", workflow, "prompt_artifact_path_env"))
	}
	findings = append(findings, scanArtifactRiskText("artifact-workflow", "workflow", workflow.Path, "body", readArtifactRiskBody(root, workflow.Path))...)
	sortArtifactRiskFindings(findings)
	return findings
}

func artifactSpecMetadataRiskFinding(severity, code, category string, spec artifactSpecCard, field string) ArtifactRiskFinding {
	return ArtifactRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "artifact-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func artifactWorkflowMetadataRiskFinding(severity, code, category string, workflow artifactWorkflowCard, field string) ArtifactRiskFinding {
	return ArtifactRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "artifact-workflow",
		Name:     "workflow",
		Path:     workflow.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(workflow.Path + ":" + field),
	}
}

func scanArtifactRiskText(kind, name, path, field, body string) []ArtifactRiskFinding {
	var findings []ArtifactRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range artifactTextRiskRules {
			if !artifactRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, ArtifactRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Name:     name,
				Path:     path,
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortArtifactRiskFindings(findings)
	return findings
}

func artifactRiskRuleMatches(lowerLine string, rule artifactRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerLine, ignored) {
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

func readArtifactRiskBody(root, relPath string) string {
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

func writeArtifactRiskFindings(b *strings.Builder, findings []ArtifactRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			finding.Name,
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func artifactRiskSurfaceCount(findings []ArtifactRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Name + "\x00" + finding.Path
		if key == "\x00\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func artifactRiskCodes(findings []ArtifactRiskFinding) []string {
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

func artifactRiskLineHashes(findings []ArtifactRiskFinding) []string {
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

func artifactRiskMaxSeverity(findings []ArtifactRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if artifactRiskSeverityRank(finding.Severity) > artifactRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func artifactRiskSeverityRank(severity string) int {
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

func sortArtifactRiskFindings(findings []ArtifactRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := artifactRiskSeverityRank(a.Severity), artifactRiskSeverityRank(b.Severity); rankA != rankB {
			return rankA > rankB
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.LineSHA < b.LineSHA
	})
}
