package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ApprovalRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Subject  string
	Field    string
	LineSHA  string
}

type ApprovalRiskReport struct {
	Status                                string
	VerificationScope                     string
	ApprovalStore                         string
	ApprovalScope                         string
	TrustedAssociations                   int
	BroadTrustedAssociations              int
	ApprovalLabelsConfigured              int
	DuplicateApprovalLabels               int
	ApprovalManagedLabelCollisions        int
	ManagedLabelsConfigured               int
	SurfacesWithRiskFindings              int
	Findings                              []ApprovalRiskFinding
	HighRiskFindings                      int
	WarningRiskFindings                   int
	InfoRiskFindings                      int
	WriteActionsSupported                 bool
	WriteActionsEnabled                   bool
	RepositoryMutationAllowed             bool
	HostExecAllowed                       bool
	ApprovalPayloadsIncluded              bool
	RawBodiesIncluded                     bool
	LLME2ERequiredAfterApprovalRiskChange bool
}

func RenderApprovalRiskCLIReport(cfg Config) string {
	return renderApprovalRiskReport(Event{}, cfg, PreflightDecision{}, nil, false, false)
}

func renderApprovalRiskReport(ev Event, cfg Config, decision PreflightDecision, transcript []TranscriptMessage, writeRequested bool, includeIssue bool) string {
	report := BuildApprovalRiskReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Approvals Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		approvedLabelPresent := hasLabel(ev.Issue.Labels, defaultApprovedLabel)
		needsHumanLabelPresent := hasLabel(ev.Issue.Labels, defaultNeedsHumanLabel)
		writeRequestedLabelPresent := hasLabel(ev.Issue.Labels, cfg.WriteRequestedLabel) || writeRequested
		actor := actorAssociation(ev)
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- preflight_allowed: `%t`\n", decision.Allowed)
		fmt.Fprintf(&b, "- preflight_code: `%s`\n", decision.Code)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actor)
		fmt.Fprintf(&b, "- actor_trusted: `%t`\n", trustedAssociation(actor, cfg))
		fmt.Fprintf(&b, "- triggered: `%t`\n", triggered(ev, cfg))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- write_request_detected: `%t`\n", writeRequested)
		fmt.Fprintf(&b, "- write_requested_label_present: `%t`\n", writeRequestedLabelPresent)
		fmt.Fprintf(&b, "- approved_label_present: `%t`\n", approvedLabelPresent)
		fmt.Fprintf(&b, "- needs_human_label_present: `%t`\n", needsHumanLabelPresent)
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeApprovalRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits the approval boundary GitClaw would rely on before future write-capable work: trusted associations, write-request labeling, per-issue approval labels, managed-label collisions, and the hard read-only runtime gate. It reports metadata, counts, labels, risk codes, and hashes only; issue bodies, comments, prompts, approval payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Approval Gate Risk Card\n")
	writeApprovalGateRiskCard(&b, report)

	b.WriteString("\n### Trusted Association Risk Cards\n")
	writeApprovalTrustedAssociationRiskCards(&b, cfg, report)

	b.WriteString("\n### Approval Label Risk Cards\n")
	writeApprovalLabelRiskCards(&b, cfg, report)

	b.WriteString("\n### Runtime Boundary Risk Card\n")
	writeApprovalRuntimeBoundaryRiskCard(&b, report)

	b.WriteString("\n### Risk Findings\n")
	writeApprovalRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildApprovalRiskReport(cfg Config) ApprovalRiskReport {
	trusted := sortedAllowedAssociations(cfg)
	broad := broadTrustedAssociations(trusted)
	approvalLabels := approvalRiskLabels(cfg)
	report := ApprovalRiskReport{
		Status:                                "ok",
		VerificationScope:                     "approval-gates-labels-and-read-only-boundary",
		ApprovalStore:                         "github-issue-labels",
		ApprovalScope:                         "per-issue",
		TrustedAssociations:                   len(trusted),
		BroadTrustedAssociations:              len(broad),
		ApprovalLabelsConfigured:              len(approvalLabels),
		DuplicateApprovalLabels:               duplicateStringCount(approvalLabels),
		ApprovalManagedLabelCollisions:        duplicateStringCount(append(managedPolicyLabels(cfg), defaultApprovedLabel, defaultNeedsHumanLabel)),
		ManagedLabelsConfigured:               len(managedPolicyLabels(cfg)),
		WriteActionsSupported:                 false,
		WriteActionsEnabled:                   false,
		RepositoryMutationAllowed:             false,
		HostExecAllowed:                       false,
		ApprovalPayloadsIncluded:              false,
		RawBodiesIncluded:                     false,
		LLME2ERequiredAfterApprovalRiskChange: true,
	}
	report.Findings = append(report.Findings, approvalStaticRiskFindings(cfg, trusted, broad)...)
	sortApprovalRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = approvalRiskSurfaceCount(report.Findings)
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

func approvalStaticRiskFindings(cfg Config, trusted, broad []string) []ApprovalRiskFinding {
	var findings []ApprovalRiskFinding
	if len(trusted) == 0 {
		findings = append(findings, approvalRiskMetadataFinding("high", "no_trusted_actors_configured", "authorization", "trusted-association", "none", "allowed_associations"))
	}
	for _, association := range broad {
		severity := "warning"
		if association == "NONE" {
			severity = "high"
		}
		findings = append(findings, approvalRiskMetadataFinding(severity, "broad_trusted_association", "authorization", "trusted-association", association, "allowed_associations"))
	}
	for _, label := range approvalRiskLabels(cfg) {
		if strings.TrimSpace(label) == "" {
			findings = append(findings, approvalRiskMetadataFinding("high", "approval_label_empty", "label-boundary", "approval-label", "empty", "labels"))
		}
	}
	if duplicateStringCount(approvalRiskLabels(cfg)) > 0 {
		findings = append(findings, approvalRiskMetadataFinding("high", "approval_label_collision", "label-boundary", "approval-label", "approval-labels", "labels"))
	}
	if duplicateStringCount(append(managedPolicyLabels(cfg), defaultApprovedLabel, defaultNeedsHumanLabel)) > 0 {
		findings = append(findings, approvalRiskMetadataFinding("high", "approval_managed_label_collision", "label-boundary", "approval-label", "managed-labels", "labels"))
	}
	sortApprovalRiskFindings(findings)
	return findings
}

func approvalRiskMetadataFinding(severity, code, category, kind, subject, field string) ApprovalRiskFinding {
	return ApprovalRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Subject:  subject,
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + subject + ":" + field),
	}
}

func writeApprovalRiskSummary(b *strings.Builder, report ApprovalRiskReport) {
	fmt.Fprintf(b, "- approval_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- approval_store: `%s`\n", report.ApprovalStore)
	fmt.Fprintf(b, "- approval_scope: `%s`\n", report.ApprovalScope)
	fmt.Fprintf(b, "- trusted_associations: `%d`\n", report.TrustedAssociations)
	fmt.Fprintf(b, "- broad_trusted_associations: `%d`\n", report.BroadTrustedAssociations)
	fmt.Fprintf(b, "- approval_labels_configured: `%d`\n", report.ApprovalLabelsConfigured)
	fmt.Fprintf(b, "- duplicate_approval_labels: `%d`\n", report.DuplicateApprovalLabels)
	fmt.Fprintf(b, "- approval_managed_label_collisions: `%d`\n", report.ApprovalManagedLabelCollisions)
	fmt.Fprintf(b, "- managed_labels_configured: `%d`\n", report.ManagedLabelsConfigured)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- approval_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- write_actions_supported: `%t`\n", report.WriteActionsSupported)
	fmt.Fprintf(b, "- write_actions_enabled: `%t`\n", report.WriteActionsEnabled)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- host_exec_allowed: `%t`\n", report.HostExecAllowed)
	fmt.Fprintf(b, "- approval_payloads_included: `%t`\n", report.ApprovalPayloadsIncluded)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_approval_risk_change: `%t`\n", report.LLME2ERequiredAfterApprovalRiskChange)
}

func writeApprovalGateRiskCard(b *strings.Builder, report ApprovalRiskReport) {
	findings := approvalRiskFindingsByKind(report.Findings, "approval-gates")
	fmt.Fprintf(
		b,
		"- kind=`approval-gates` store=`%s` scope=`%s` write_request_detection=`heuristic-transcript-scan` approval_required_for_future_write=`true` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.ApprovalStore,
		report.ApprovalScope,
		len(findings),
		approvalRiskMaxSeverity(findings),
		inlineListOrNone(approvalRiskCodes(findings)),
		inlineListOrNone(approvalRiskLineHashes(findings)),
	)
}

func writeApprovalTrustedAssociationRiskCards(b *strings.Builder, cfg Config, report ApprovalRiskReport) {
	associations := sortedAllowedAssociations(cfg)
	if len(associations) == 0 {
		findings := approvalRiskFindingsByKind(report.Findings, "trusted-association")
		fmt.Fprintf(b, "- kind=`trusted-association` association=`none` trusted=`false` broad=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n", len(findings), approvalRiskMaxSeverity(findings), inlineListOrNone(approvalRiskCodes(findings)), inlineListOrNone(approvalRiskLineHashes(findings)))
		return
	}
	broad := map[string]bool{}
	for _, association := range broadTrustedAssociations(associations) {
		broad[association] = true
	}
	for _, association := range associations {
		findings := approvalRiskFindingsBySubject(report.Findings, "trusted-association", association)
		fmt.Fprintf(
			b,
			"- kind=`trusted-association` association=`%s` trusted=`true` broad=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			association,
			broad[association],
			len(findings),
			approvalRiskMaxSeverity(findings),
			inlineListOrNone(approvalRiskCodes(findings)),
			inlineListOrNone(approvalRiskLineHashes(findings)),
		)
	}
}

func writeApprovalLabelRiskCards(b *strings.Builder, cfg Config, report ApprovalRiskReport) {
	managed := map[string]bool{}
	for _, label := range managedPolicyLabels(cfg) {
		managed[strings.ToLower(strings.TrimSpace(label))] = true
	}
	for _, card := range []struct {
		role  string
		label string
	}{
		{role: "write-requested", label: cfg.WriteRequestedLabel},
		{role: "approved", label: defaultApprovedLabel},
		{role: "needs-human", label: defaultNeedsHumanLabel},
	} {
		findings := approvalRiskFindingsBySubject(report.Findings, "approval-label", card.role)
		if len(findings) == 0 && strings.TrimSpace(card.label) == "" {
			findings = approvalRiskFindingsBySubject(report.Findings, "approval-label", "empty")
		}
		fmt.Fprintf(
			b,
			"- kind=`approval-label` role=`%s` label=`%s` managed=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			card.role,
			inlineCode(card.label),
			managed[strings.ToLower(strings.TrimSpace(card.label))],
			len(findings),
			approvalRiskMaxSeverity(findings),
			inlineListOrNone(approvalRiskCodes(findings)),
			inlineListOrNone(approvalRiskLineHashes(findings)),
		)
	}
}

func writeApprovalRuntimeBoundaryRiskCard(b *strings.Builder, report ApprovalRiskReport) {
	findings := approvalRiskFindingsByKind(report.Findings, "runtime-boundary")
	fmt.Fprintf(
		b,
		"- kind=`runtime-boundary` write_actions_supported=`%t` write_actions_enabled=`%t` repository_mutation_allowed=`%t` host_exec_allowed=`%t` approval_payloads_included=`%t` raw_bodies_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.WriteActionsSupported,
		report.WriteActionsEnabled,
		report.RepositoryMutationAllowed,
		report.HostExecAllowed,
		report.ApprovalPayloadsIncluded,
		report.RawBodiesIncluded,
		len(findings),
		approvalRiskMaxSeverity(findings),
		inlineListOrNone(approvalRiskCodes(findings)),
		inlineListOrNone(approvalRiskLineHashes(findings)),
	)
}

func writeApprovalRiskFindings(b *strings.Builder, findings []ApprovalRiskFinding) {
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

func approvalRiskLabels(cfg Config) []string {
	return []string{cfg.WriteRequestedLabel, defaultApprovedLabel, defaultNeedsHumanLabel}
}

func approvalRiskFindingsByKind(findings []ApprovalRiskFinding, kind string) []ApprovalRiskFinding {
	var filtered []ApprovalRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			filtered = append(filtered, finding)
		}
	}
	sortApprovalRiskFindings(filtered)
	return filtered
}

func approvalRiskFindingsBySubject(findings []ApprovalRiskFinding, kind, subject string) []ApprovalRiskFinding {
	var filtered []ApprovalRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind && strings.EqualFold(finding.Subject, subject) {
			filtered = append(filtered, finding)
		}
	}
	sortApprovalRiskFindings(filtered)
	return filtered
}

func approvalRiskSurfaceCount(findings []ApprovalRiskFinding) int {
	surfaces := map[string]bool{}
	for _, finding := range findings {
		surfaces[finding.Kind+"\x00"+finding.Subject] = true
	}
	return len(surfaces)
}

func approvalRiskCodes(findings []ApprovalRiskFinding) []string {
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

func approvalRiskLineHashes(findings []ApprovalRiskFinding) []string {
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

func approvalRiskMaxSeverity(findings []ApprovalRiskFinding) string {
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

func sortApprovalRiskFindings(findings []ApprovalRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		if left.Severity != right.Severity {
			return approvalRiskSeverityRank(left.Severity) < approvalRiskSeverityRank(right.Severity)
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

func approvalRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func isApprovalRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		(fields[0] == "/approvals" || fields[0] == "/approval") &&
		(strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}
