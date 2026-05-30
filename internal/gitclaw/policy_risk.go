package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type PolicyRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Subject  string
	Field    string
	LineSHA  string
}

type PolicyRiskReport struct {
	Status                               string
	VerificationScope                    string
	RunMode                              string
	Model                                string
	TrustedAssociations                  int
	BroadTrustedAssociations             int
	ManagedLabelsConfigured              int
	DuplicateManagedLabels               int
	WorkflowPath                         string
	WorkflowPresent                      bool
	WorkflowBytes                        int
	WorkflowLines                        int
	WorkflowSHA                          string
	WorkflowVerifyStatus                 string
	ExpectedJobs                         int
	JobsPresent                          int
	ExpectedPermissions                  int
	ExpectedWritePermissions             int
	PermissionsPresent                   int
	MissingPermissions                   int
	UnexpectedWritePermissions           int
	BackupConcurrencyGroup               bool
	BackupConcurrencyCancelSafe          bool
	PolicyOutputsHashed                  int
	PolicyOutputPresent                  bool
	SurfacesWithRiskFindings             int
	Findings                             []PolicyRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	WriteRequestPolicyOutputBodyIncluded bool
	WriteActionsSupported                bool
	WriteActionsEnabled                  bool
	RepositoryMutationAllowed            bool
	HostExecAllowed                      bool
	RawBodiesIncluded                    bool
	RawInputsIncluded                    bool
	LLME2ERequiredAfterPolicyRiskChange  bool
	WorkflowJobs                         []policyVerifyJob
}

func renderPolicyRiskReport(ev Event, cfg Config, decision PreflightDecision, transcript []TranscriptMessage, repoContext RepoContext, writeRequested bool, includeIssue bool) string {
	report := BuildPolicyRiskReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Policy Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
		fmt.Fprintf(&b, "- preflight_allowed: `%t`\n", decision.Allowed)
		fmt.Fprintf(&b, "- preflight_code: `%s`\n", decision.Code)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actorAssociation(ev))
		fmt.Fprintf(&b, "- actor_trusted: `%t`\n", trustedAssociation(actorAssociation(ev), cfg))
		fmt.Fprintf(&b, "- triggered: `%t`\n", triggered(ev, cfg))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- pull_request: `%t`\n", ev.Issue.IsPullRequest)
		fmt.Fprintf(&b, "- write_request_detected: `%t`\n", writeRequested)
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writePolicyRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report risk-audits GitClaw's control-plane policy boundary: actor trust, managed labels, workflow permissions, backup concurrency, active write-request policy outputs, and the hard read-only runtime gate. It reports metadata, hashes, risk codes, and severities only; workflow bodies, issue bodies, comments, prompts, policy output bodies, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Trust Boundary Risk Card\n")
	writePolicyTrustRiskCard(&b, cfg, report)

	b.WriteString("\n### Managed Label Risk Card\n")
	writePolicyManagedLabelRiskCard(&b, cfg, report)

	b.WriteString("\n### Workflow Permission Risk Cards\n")
	writePolicyWorkflowRiskCards(&b, report)

	b.WriteString("\n### Policy Output Risk Cards\n")
	writePolicyOutputRiskCards(&b, repoContext, report)

	b.WriteString("\n### Runtime Boundary Risk Card\n")
	writePolicyRuntimeBoundaryRiskCard(&b, report)

	b.WriteString("\n### Risk Findings\n")
	writePolicyRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPolicyRiskReport(cfg Config, repoContext RepoContext) PolicyRiskReport {
	verify := BuildPolicyVerifyReport(cfg, repoContext)
	trusted := sortedAllowedAssociations(cfg)
	broad := broadTrustedAssociations(trusted)
	managedLabels := managedPolicyLabels(cfg)
	report := PolicyRiskReport{
		Status:                              "ok",
		VerificationScope:                   "policy-trust-labels-workflow-permissions-and-write-boundary",
		RunMode:                             "read-only",
		Model:                               cfg.Model,
		TrustedAssociations:                 len(trusted),
		BroadTrustedAssociations:            len(broad),
		ManagedLabelsConfigured:             len(managedLabels),
		DuplicateManagedLabels:              duplicateStringCount(managedLabels),
		WorkflowPath:                        verify.WorkflowPath,
		WorkflowPresent:                     verify.WorkflowPresent,
		WorkflowBytes:                       verify.WorkflowBytes,
		WorkflowLines:                       verify.WorkflowLines,
		WorkflowSHA:                         verify.WorkflowSHA,
		WorkflowVerifyStatus:                verify.Status,
		ExpectedJobs:                        verify.ExpectedJobs,
		JobsPresent:                         verify.JobsPresent,
		ExpectedPermissions:                 verify.ExpectedPermissions,
		ExpectedWritePermissions:            policyExpectedWritePermissions(),
		PermissionsPresent:                  verify.PermissionsPresent,
		MissingPermissions:                  verify.MissingPermissions,
		UnexpectedWritePermissions:          verify.UnexpectedWritePermissions,
		BackupConcurrencyGroup:              verify.BackupConcurrencyGroup,
		BackupConcurrencyCancelSafe:         verify.BackupConcurrencyCancelSafe,
		PolicyOutputsHashed:                 verify.PolicyOutputsHashed,
		PolicyOutputPresent:                 verify.PolicyOutputsHashed > 0,
		WriteActionsSupported:               false,
		WriteActionsEnabled:                 false,
		RepositoryMutationAllowed:           false,
		HostExecAllowed:                     false,
		RawBodiesIncluded:                   false,
		RawInputsIncluded:                   false,
		LLME2ERequiredAfterPolicyRiskChange: true,
		WorkflowJobs:                        verify.Jobs,
	}
	report.Findings = append(report.Findings, policyStaticRiskFindings(cfg, trusted, broad)...)
	report.Findings = append(report.Findings, policyWorkflowRiskFindings(verify.Findings)...)
	sortPolicyRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = policyRiskSurfaceCount(report.Findings)
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

func policyStaticRiskFindings(cfg Config, trusted, broad []string) []PolicyRiskFinding {
	var findings []PolicyRiskFinding
	if len(trusted) == 0 {
		findings = append(findings, policyRiskMetadataFinding("high", "no_trusted_actors_configured", "authorization", "trust-boundary", "none", "allowed_associations"))
	}
	for _, association := range broad {
		severity := "warning"
		if association == "NONE" {
			severity = "high"
		}
		findings = append(findings, policyRiskMetadataFinding(severity, "broad_trusted_association", "authorization", "trust-boundary", association, "allowed_associations"))
	}
	for _, label := range managedPolicyLabels(cfg) {
		if strings.TrimSpace(label) == "" {
			findings = append(findings, policyRiskMetadataFinding("high", "managed_label_empty", "label-boundary", "managed-label", "empty", "labels"))
		}
	}
	if duplicateStringCount(managedPolicyLabels(cfg)) > 0 {
		findings = append(findings, policyRiskMetadataFinding("high", "managed_label_collision", "label-boundary", "managed-label", "managed-labels", "labels"))
	}
	sortPolicyRiskFindings(findings)
	return findings
}

func policyWorkflowRiskFindings(findings []policyVerifyFinding) []PolicyRiskFinding {
	var riskFindings []PolicyRiskFinding
	for _, finding := range findings {
		subject := finding.Job
		if subject == "" {
			subject = "workflow"
		}
		field := finding.Permission
		if field == "" {
			field = finding.Code
		}
		riskFindings = append(riskFindings, PolicyRiskFinding{
			Severity: "high",
			Code:     finding.Code,
			Category: "workflow-permissions",
			Kind:     "workflow-permission",
			Subject:  subject,
			Field:    field,
			LineSHA:  shortDocumentHash(subject + ":" + field + ":" + finding.Detail),
		})
	}
	sortPolicyRiskFindings(riskFindings)
	return riskFindings
}

func policyRiskMetadataFinding(severity, code, category, kind, subject, field string) PolicyRiskFinding {
	return PolicyRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Subject:  subject,
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + subject + ":" + field),
	}
}

func writePolicyRiskSummary(b *strings.Builder, report PolicyRiskReport) {
	fmt.Fprintf(b, "- policy_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- run_mode: `%s`\n", report.RunMode)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- trusted_associations: `%d`\n", report.TrustedAssociations)
	fmt.Fprintf(b, "- broad_trusted_associations: `%d`\n", report.BroadTrustedAssociations)
	fmt.Fprintf(b, "- managed_labels_configured: `%d`\n", report.ManagedLabelsConfigured)
	fmt.Fprintf(b, "- duplicate_managed_labels: `%d`\n", report.DuplicateManagedLabels)
	fmt.Fprintf(b, "- workflow_path: `%s`\n", report.WorkflowPath)
	fmt.Fprintf(b, "- workflow_present: `%t`\n", report.WorkflowPresent)
	if report.WorkflowPresent {
		fmt.Fprintf(b, "- workflow_bytes: `%d`\n", report.WorkflowBytes)
		fmt.Fprintf(b, "- workflow_lines: `%d`\n", report.WorkflowLines)
		fmt.Fprintf(b, "- workflow_sha256_12: `%s`\n", report.WorkflowSHA)
	}
	fmt.Fprintf(b, "- workflow_verify_status: `%s`\n", report.WorkflowVerifyStatus)
	fmt.Fprintf(b, "- expected_jobs: `%d`\n", report.ExpectedJobs)
	fmt.Fprintf(b, "- jobs_present: `%d`\n", report.JobsPresent)
	fmt.Fprintf(b, "- expected_permissions: `%d`\n", report.ExpectedPermissions)
	fmt.Fprintf(b, "- expected_write_permissions: `%d`\n", report.ExpectedWritePermissions)
	fmt.Fprintf(b, "- permissions_present: `%d`\n", report.PermissionsPresent)
	fmt.Fprintf(b, "- missing_permissions: `%d`\n", report.MissingPermissions)
	fmt.Fprintf(b, "- unexpected_write_permissions: `%d`\n", report.UnexpectedWritePermissions)
	fmt.Fprintf(b, "- backup_concurrency_group: `%t`\n", report.BackupConcurrencyGroup)
	fmt.Fprintf(b, "- backup_concurrency_cancel_safe: `%t`\n", report.BackupConcurrencyCancelSafe)
	fmt.Fprintf(b, "- policy_outputs_hashed: `%d`\n", report.PolicyOutputsHashed)
	fmt.Fprintf(b, "- policy_output_present: `%t`\n", report.PolicyOutputPresent)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- policy_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- write_request_policy_output_body_included: `%t`\n", report.WriteRequestPolicyOutputBodyIncluded)
	fmt.Fprintf(b, "- write_actions_supported: `%t`\n", report.WriteActionsSupported)
	fmt.Fprintf(b, "- write_actions_enabled: `%t`\n", report.WriteActionsEnabled)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- host_exec_allowed: `%t`\n", report.HostExecAllowed)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_policy_risk_change: `%t`\n", report.LLME2ERequiredAfterPolicyRiskChange)
}

func writePolicyTrustRiskCard(b *strings.Builder, cfg Config, report PolicyRiskReport) {
	findings := policyRiskFindingsByKind(report.Findings, "trust-boundary")
	fmt.Fprintf(
		b,
		"- kind=`trust-boundary` trusted_associations=`%s` broad_trusted_associations=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineListOrNone(sortedAllowedAssociations(cfg)),
		inlineListOrNone(broadTrustedAssociations(sortedAllowedAssociations(cfg))),
		len(findings),
		policyRiskMaxSeverity(findings),
		inlineListOrNone(policyRiskCodes(findings)),
		inlineListOrNone(policyRiskLineHashes(findings)),
	)
}

func writePolicyManagedLabelRiskCard(b *strings.Builder, cfg Config, report PolicyRiskReport) {
	findings := policyRiskFindingsByKind(report.Findings, "managed-label")
	fmt.Fprintf(
		b,
		"- kind=`managed-labels` labels=`%s` managed_labels_configured=`%d` duplicate_managed_labels=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineList(managedPolicyLabels(cfg)),
		report.ManagedLabelsConfigured,
		report.DuplicateManagedLabels,
		len(findings),
		policyRiskMaxSeverity(findings),
		inlineListOrNone(policyRiskCodes(findings)),
		inlineListOrNone(policyRiskLineHashes(findings)),
	)
}

func writePolicyWorkflowRiskCards(b *strings.Builder, report PolicyRiskReport) {
	if len(report.WorkflowJobs) == 0 {
		findings := policyRiskFindingsByKind(report.Findings, "workflow-permission")
		fmt.Fprintf(b, "- kind=`workflow-permission` workflow_path=`%s` workflow_present=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n", report.WorkflowPath, report.WorkflowPresent, len(findings), policyRiskMaxSeverity(findings), inlineListOrNone(policyRiskCodes(findings)), inlineListOrNone(policyRiskLineHashes(findings)))
		return
	}
	for _, job := range report.WorkflowJobs {
		findings := policyRiskFindingsBySubject(report.Findings, "workflow-permission", job.Name)
		fmt.Fprintf(
			b,
			"- kind=`workflow-permission` job=`%s` present=`%t` expected=`%s` actual=`%s` matched=`%d` missing=`%s` unexpected_write=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			job.Name,
			job.Present,
			inlineList(job.Expected),
			inlineList(job.Actual),
			len(job.Matched),
			inlineListOrNone(job.Missing),
			inlineListOrNone(job.UnexpectedWrites),
			len(findings),
			policyRiskMaxSeverity(findings),
			inlineListOrNone(policyRiskCodes(findings)),
			inlineListOrNone(policyRiskLineHashes(findings)),
		)
	}
}

func writePolicyOutputRiskCards(b *strings.Builder, repoContext RepoContext, report PolicyRiskReport) {
	wrote := false
	for _, output := range repoContext.ToolOutputs {
		if output.Name != "gitclaw.policy" {
			continue
		}
		wrote = true
		fmt.Fprintf(
			b,
			"- kind=`policy-output` name=`%s` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` output_body_included=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none` line_hashes=`none`\n",
			output.Name,
			shortDocumentHash(output.Input),
			len(output.Output),
			lineCount(output.Output),
			shortDocumentHash(output.Output),
		)
	}
	if !wrote {
		fmt.Fprintf(b, "- kind=`policy-output` none policy_outputs_hashed=`%d` output_body_included=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n", report.PolicyOutputsHashed)
	}
}

func writePolicyRuntimeBoundaryRiskCard(b *strings.Builder, report PolicyRiskReport) {
	findings := policyRiskFindingsByKind(report.Findings, "runtime-boundary")
	fmt.Fprintf(
		b,
		"- kind=`runtime-boundary` run_mode=`%s` write_actions_supported=`%t` write_actions_enabled=`%t` repository_mutation_allowed=`%t` host_exec_allowed=`%t` raw_bodies_included=`%t` raw_inputs_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.RunMode,
		report.WriteActionsSupported,
		report.WriteActionsEnabled,
		report.RepositoryMutationAllowed,
		report.HostExecAllowed,
		report.RawBodiesIncluded,
		report.RawInputsIncluded,
		len(findings),
		policyRiskMaxSeverity(findings),
		inlineListOrNone(policyRiskCodes(findings)),
		inlineListOrNone(policyRiskLineHashes(findings)),
	)
}

func writePolicyRiskFindings(b *strings.Builder, findings []PolicyRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` subject=`%s` field=`%s` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			inlineCode(finding.Subject),
			finding.Field,
			finding.LineSHA,
		)
	}
}

func policyExpectedWritePermissions() int {
	count := 0
	for _, contract := range policyWorkflowPermissions {
		for _, permission := range contract.Permissions {
			if strings.HasSuffix(permission, ":write") {
				count++
			}
		}
	}
	return count
}

func policyRiskFindingsByKind(findings []PolicyRiskFinding, kind string) []PolicyRiskFinding {
	var filtered []PolicyRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			filtered = append(filtered, finding)
		}
	}
	sortPolicyRiskFindings(filtered)
	return filtered
}

func policyRiskFindingsBySubject(findings []PolicyRiskFinding, kind, subject string) []PolicyRiskFinding {
	var filtered []PolicyRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind && strings.EqualFold(finding.Subject, subject) {
			filtered = append(filtered, finding)
		}
	}
	sortPolicyRiskFindings(filtered)
	return filtered
}

func policyRiskSurfaceCount(findings []PolicyRiskFinding) int {
	surfaces := map[string]bool{}
	for _, finding := range findings {
		surfaces[finding.Kind+"\x00"+finding.Subject] = true
	}
	return len(surfaces)
}

func policyRiskCodes(findings []PolicyRiskFinding) []string {
	var codes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	return sortedStrings(codes)
}

func policyRiskLineHashes(findings []PolicyRiskFinding) []string {
	var hashes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	return sortedStrings(hashes)
}

func policyRiskMaxSeverity(findings []PolicyRiskFinding) string {
	if len(findings) == 0 {
		return "none"
	}
	max := "info"
	for _, finding := range findings {
		switch finding.Severity {
		case "high":
			return "high"
		case "warning":
			max = "warning"
		}
	}
	return max
}

func sortPolicyRiskFindings(findings []PolicyRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		if left.Severity != right.Severity {
			return policyRiskSeverityRank(left.Severity) < policyRiskSeverityRank(right.Severity)
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Subject != right.Subject {
			return left.Subject < right.Subject
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		return left.Field < right.Field
	})
}

func policyRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}
