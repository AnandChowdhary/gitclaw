package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type StandingOrderRiskFinding struct {
	Severity string
	Code     string
	Category string
	Path     string
	Subject  string
	Line     int
	LineSHA  string
}

type StandingOrderRiskReport struct {
	Status                        string
	StandingOrdersPresent         bool
	StandingOrdersLoadedForModel  bool
	StandingOrderPrograms         int
	CompletePrograms              int
	ProgramsWithAuthority         int
	ProgramsWithTrigger           int
	ProgramsWithApprovalGate      int
	ProgramsWithEscalation        int
	ProactivePromptFiles          int
	ProactiveWorkflowPresent      bool
	ProactiveScheduleTrigger      bool
	Findings                      []StandingOrderRiskFinding
	SurfacesWithRiskFindings      int
	HighRiskFindings              int
	WarningRiskFindings           int
	InfoRiskFindings              int
	EnforcementStrategy           string
	ModelCallRequired             bool
	RepositoryMutationAllowed     bool
	AgentAuthoredOrderMutation    bool
	RawBodiesIncluded             bool
	RawOrdersBodyIncluded         bool
	RawProactiveBodiesIncluded    bool
	RawIssueBodiesIncluded        bool
	RawCommentBodiesIncluded      bool
	CredentialValuesIncluded      bool
	LLME2ERequiredAfterOrdersRisk bool
}

type standingOrderRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var standingOrderRiskRules = []standingOrderRiskRule{
	{
		Severity: "high",
		Code:     "standing_order_prompt_boundary_override",
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
		Code:     "standing_order_unbounded_authority",
		Category: "authority-boundary",
		Any: []string{
			"do whatever you think is best",
			"unlimited authority",
			"full autonomy",
			"no limits",
			"act without any approval",
			"skip approval",
			"do not ask for approval",
		},
	},
	{
		Severity: "high",
		Code:     "standing_order_secret_exfiltration",
		Category: "data-exfiltration",
		Any: []string{
			"exfiltrate",
			"leak secrets",
			"send secrets",
			"upload secrets",
			"steal secrets",
		},
	},
	{
		Severity: "warning",
		Code:     "standing_order_unreviewed_external_delivery",
		Category: "external-delivery",
		Any: []string{
			"post publicly",
			"publish publicly",
			"send to external parties",
			"email customers",
			"message customers",
			"wire money",
			"transfer funds",
		},
	},
	{
		Severity: "warning",
		Code:     "standing_order_unreviewed_host_execution",
		Category: "host-execution",
		Any: []string{
			"rm -rf",
			"force push",
			"git push --force",
			"delete repository",
			"wipe database",
			"drop table",
			"run shell",
			"execute shell",
			"bash -c",
			"curl | bash",
		},
	},
	{
		Severity: "warning",
		Code:     "standing_order_hidden_persistence",
		Category: "persistent-state",
		Any: []string{
			"silently persist",
			"persist without review",
			"write to memory without review",
			"edit memory without review",
			"modify soul without review",
			"append to soul without review",
		},
	},
	{
		Severity: "warning",
		Code:     "standing_order_unbounded_retry",
		Category: "execution-control",
		Any: []string{
			"retry forever",
			"loop forever",
			"infinite loop",
			"never stop",
			"continue indefinitely",
		},
	},
	{
		Severity: "warning",
		Code:     "standing_order_skip_verification",
		Category: "verification",
		Any: []string{
			"skip tests",
			"skip e2e",
			"skip verification",
			"no need to verify",
			"do not report",
			"silently fail",
		},
	},
	{
		Severity: "info",
		Code:     "standing_order_credential_transfer",
		Category: "credential-handling",
		All:      []string{"send", "api key"},
	},
	{
		Severity: "info",
		Code:     "standing_order_credential_transfer",
		Category: "credential-handling",
		All:      []string{"upload", "api key"},
	},
	{
		Severity: "info",
		Code:     "standing_order_credential_transfer",
		Category: "credential-handling",
		All:      []string{"post", "api key"},
	},
	{
		Severity: "info",
		Code:     "standing_order_credential_transfer",
		Category: "credential-handling",
		All:      []string{"send", "github token"},
	},
	{
		Severity: "info",
		Code:     "standing_order_credential_transfer",
		Category: "credential-handling",
		All:      []string{"upload", "github token"},
	},
	{
		Severity: "info",
		Code:     "standing_order_credential_transfer",
		Category: "credential-handling",
		All:      []string{"post", "github token"},
	},
}

func BuildStandingOrderRiskReport(root string) StandingOrderRiskReport {
	surface := inspectStandingOrderSurface(root)
	report := StandingOrderRiskReport{
		Status:                        "ok",
		StandingOrdersPresent:         surface.Orders.Present,
		StandingOrdersLoadedForModel:  standingOrdersLoadedForModel(surface),
		StandingOrderPrograms:         len(surface.Programs),
		CompletePrograms:              countStandingPrograms(surface.Programs, standingOrderProgramComplete),
		ProgramsWithAuthority:         countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.Authority }),
		ProgramsWithTrigger:           countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.Trigger }),
		ProgramsWithApprovalGate:      countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.ApprovalGate }),
		ProgramsWithEscalation:        countStandingPrograms(surface.Programs, func(p standingOrderProgram) bool { return p.Escalation }),
		ProactivePromptFiles:          len(surface.Proactive.Prompts),
		ProactiveWorkflowPresent:      surface.Proactive.Workflow.Present,
		ProactiveScheduleTrigger:      surface.Proactive.Workflow.Schedule,
		EnforcementStrategy:           "repo-reviewed-proactive-workflows-or-manual-trigger",
		RawBodiesIncluded:             false,
		RawOrdersBodyIncluded:         false,
		RawProactiveBodiesIncluded:    false,
		RawIssueBodiesIncluded:        false,
		RawCommentBodiesIncluded:      false,
		CredentialValuesIncluded:      false,
		LLME2ERequiredAfterOrdersRisk: true,
	}

	for _, finding := range standingOrderFindings(surface) {
		report.Findings = append(report.Findings, standingOrderVerificationRiskFinding(finding))
	}
	report.Findings = append(report.Findings, scanStandingOrderRiskFindings(root, surface.Orders.Path)...)
	if surface.Orders.Present && !surface.Proactive.Workflow.Present {
		report.Findings = append(report.Findings, standingOrderMetadataRiskFinding("warning", "proactive_workflow_missing", "enforcement", proactiveWorkflowPath, "workflow", "standing orders need an explicit GitHub Actions enforcement surface"))
	}
	if surface.Orders.Present && surface.Proactive.Workflow.Present && !surface.Proactive.Workflow.Schedule {
		report.Findings = append(report.Findings, standingOrderMetadataRiskFinding("warning", "proactive_schedule_missing", "enforcement", proactiveWorkflowPath, "workflow", "standing orders without a schedule are manual-only"))
	}
	sortStandingOrderRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = countStandingOrderRiskSurfaces(report.Findings)
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

func standingOrderVerificationRiskFinding(finding standingOrderFinding) StandingOrderRiskFinding {
	severity := "warning"
	if finding.Severity == "error" {
		severity = "high"
	} else if finding.Severity == "info" {
		severity = "info"
	}
	return standingOrderMetadataRiskFinding(severity, finding.Code, "standing-order-structure", standingOrdersPath, finding.Subject, finding.Message)
}

func standingOrderMetadataRiskFinding(severity, code, category, path, subject, detail string) StandingOrderRiskFinding {
	return StandingOrderRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Path:     path,
		Subject:  subject,
		Line:     0,
		LineSHA:  shortDocumentHash(path + ":" + code + ":" + subject + ":" + detail),
	}
}

func scanStandingOrderRiskFindings(root, relPath string) []StandingOrderRiskFinding {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	if err != nil {
		return nil
	}
	var findings []StandingOrderRiskFinding
	for lineNumber, line := range strings.Split(string(body), "\n") {
		lower := strings.ToLower(line)
		seenLineCodes := map[string]bool{}
		for _, rule := range standingOrderRiskRules {
			if !standingOrderRiskRuleMatches(lower, rule) {
				continue
			}
			if seenLineCodes[rule.Code] {
				continue
			}
			seenLineCodes[rule.Code] = true
			findings = append(findings, StandingOrderRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Path:     relPath,
				Subject:  "line",
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortStandingOrderRiskFindings(findings)
	return findings
}

func standingOrderRiskRuleMatches(lowerLine string, rule standingOrderRiskRule) bool {
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

func renderStandingOrdersRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectStandingOrderSurface(cfg.Workdir)
	report := BuildStandingOrderRiskReport(cfg.Workdir)

	var b strings.Builder
	b.WriteString("## GitClaw Standing Orders Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeStandingOrderRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans repo-reviewed standing orders for body-free authority, enforcement, escalation, persistence, external-delivery, credential, and verification risks inspired by OpenClaw standing orders and Hermes scheduled goals. It reports only metadata, counts, finding codes, severities, paths, and hashes; standing-order bodies, proactive prompt bodies, issue bodies, comments, prompts, credentials, and secret values are not included.\n\n")

	b.WriteString("### Standing Orders Risk Card\n")
	writeStandingOrdersFileRiskCard(&b, surface.Orders, report.Findings)

	b.WriteString("\n### Program Risk Cards\n")
	if len(surface.Programs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, program := range surface.Programs {
			findings := standingOrderFindingsForSubject(fmt.Sprintf("program:%02d", program.Index), report.Findings)
			fmt.Fprintf(
				&b,
				"- program=`%02d` title_sha256_12=`%s` lines=`%d` authority=`%t` trigger=`%t` approval_gate=`%t` escalation=`%t` complete=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
				program.Index,
				program.TitleSHA,
				program.Lines,
				program.Authority,
				program.Trigger,
				program.ApprovalGate,
				program.Escalation,
				standingOrderProgramComplete(program),
				len(findings),
				standingOrderRiskMaxSeverity(findings),
				inlineListOrNone(standingOrderRiskCodes(findings)),
				inlineListOrNone(standingOrderRiskLineHashes(findings)),
			)
		}
	}

	b.WriteString("\n### Enforcement Risk Card\n")
	enforcementFindings := standingOrderFindingsForPath(proactiveWorkflowPath, report.Findings)
	fmt.Fprintf(
		&b,
		"- path=`%s` present=`%t` workflow_dispatch=`%t` schedule=`%t` proactive_prompt_files=`%d` enforcement_strategy=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		proactiveWorkflowPath,
		surface.Proactive.Workflow.Present,
		surface.Proactive.Workflow.WorkflowDispatch,
		surface.Proactive.Workflow.Schedule,
		len(surface.Proactive.Prompts),
		report.EnforcementStrategy,
		len(enforcementFindings),
		standingOrderRiskMaxSeverity(enforcementFindings),
		inlineListOrNone(standingOrderRiskCodes(enforcementFindings)),
		inlineListOrNone(standingOrderRiskLineHashes(enforcementFindings)),
	)

	b.WriteString("\n### Risk Findings\n")
	writeStandingOrderRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeStandingOrderRiskSummary(b *strings.Builder, report StandingOrderRiskReport) {
	fmt.Fprintf(b, "- standing_order_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- standing_orders_present: `%t`\n", report.StandingOrdersPresent)
	fmt.Fprintf(b, "- standing_orders_loaded_for_model: `%t`\n", report.StandingOrdersLoadedForModel)
	fmt.Fprintf(b, "- standing_order_programs: `%d`\n", report.StandingOrderPrograms)
	fmt.Fprintf(b, "- complete_programs: `%d`\n", report.CompletePrograms)
	fmt.Fprintf(b, "- programs_with_authority: `%d`\n", report.ProgramsWithAuthority)
	fmt.Fprintf(b, "- programs_with_trigger: `%d`\n", report.ProgramsWithTrigger)
	fmt.Fprintf(b, "- programs_with_approval_gate: `%d`\n", report.ProgramsWithApprovalGate)
	fmt.Fprintf(b, "- programs_with_escalation: `%d`\n", report.ProgramsWithEscalation)
	fmt.Fprintf(b, "- proactive_prompt_files: `%d`\n", report.ProactivePromptFiles)
	fmt.Fprintf(b, "- proactive_workflow_present: `%t`\n", report.ProactiveWorkflowPresent)
	fmt.Fprintf(b, "- proactive_schedule_trigger: `%t`\n", report.ProactiveScheduleTrigger)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- standing_order_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- enforcement_strategy: `%s`\n", report.EnforcementStrategy)
	fmt.Fprintf(b, "- model_call_required: `%t`\n", report.ModelCallRequired)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- agent_authored_order_mutation_supported: `%t`\n", report.AgentAuthoredOrderMutation)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- raw_orders_body_included: `%t`\n", report.RawOrdersBodyIncluded)
	fmt.Fprintf(b, "- raw_proactive_bodies_included: `%t`\n", report.RawProactiveBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_orders_risk_change: `%t`\n", report.LLME2ERequiredAfterOrdersRisk)
}

func writeStandingOrdersFileRiskCard(b *strings.Builder, file configSurfaceFile, findings []StandingOrderRiskFinding) {
	fileFindings := standingOrderFindingsForPath(file.Path, findings)
	if !file.Present {
		fmt.Fprintf(
			b,
			"- path=`%s` present=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			file.Path,
			len(fileFindings),
			standingOrderRiskMaxSeverity(fileFindings),
			inlineListOrNone(standingOrderRiskCodes(fileFindings)),
			inlineListOrNone(standingOrderRiskLineHashes(fileFindings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- path=`%s` present=`true` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		file.Path,
		file.Bytes,
		file.Lines,
		file.SHA,
		len(fileFindings),
		standingOrderRiskMaxSeverity(fileFindings),
		inlineListOrNone(standingOrderRiskCodes(fileFindings)),
		inlineListOrNone(standingOrderRiskLineHashes(fileFindings)),
	)
}

func writeStandingOrderRiskFindings(b *strings.Builder, findings []StandingOrderRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` path=`%s` subject=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Path,
			finding.Subject,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func standingOrderFindingsForPath(path string, findings []StandingOrderRiskFinding) []StandingOrderRiskFinding {
	var matches []StandingOrderRiskFinding
	for _, finding := range findings {
		if finding.Path == path {
			matches = append(matches, finding)
		}
	}
	return matches
}

func standingOrderFindingsForSubject(subject string, findings []StandingOrderRiskFinding) []StandingOrderRiskFinding {
	var matches []StandingOrderRiskFinding
	for _, finding := range findings {
		if finding.Subject == subject {
			matches = append(matches, finding)
		}
	}
	return matches
}

func standingOrderRiskCodes(findings []StandingOrderRiskFinding) []string {
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

func standingOrderRiskLineHashes(findings []StandingOrderRiskFinding) []string {
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

func standingOrderRiskMaxSeverity(findings []StandingOrderRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if standingOrderRiskSeverityRank(finding.Severity) > standingOrderRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func standingOrderRiskSeverityRank(severity string) int {
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

func countStandingOrderRiskSurfaces(findings []StandingOrderRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Path
		if finding.Subject != "" {
			key += "#" + finding.Subject
		}
		seen[key] = true
	}
	return len(seen)
}

func sortStandingOrderRiskFindings(findings []StandingOrderRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if standingOrderRiskSeverityRank(findings[i].Severity) != standingOrderRiskSeverityRank(findings[j].Severity) {
			return standingOrderRiskSeverityRank(findings[i].Severity) > standingOrderRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Subject != findings[j].Subject {
			return findings[i].Subject < findings[j].Subject
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Code < findings[j].Code
	})
}
