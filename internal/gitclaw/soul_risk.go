package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SoulRiskFinding struct {
	Severity string
	Code     string
	Category string
	Path     string
	Line     int
	LineSHA  string
}

type SoulRiskReport struct {
	Status                        string
	Documents                     int
	ScannedDocuments              int
	DocumentsWithRiskFindings     int
	Findings                      []SoulRiskFinding
	HighRiskFindings              int
	WarningRiskFindings           int
	InfoRiskFindings              int
	RegistryVerification          string
	ProfileExportVerification     string
	RawBodiesIncluded             bool
	LLME2ERequiredAfterRiskChange bool
}

type soulRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var soulRiskRules = []soulRiskRule{
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
		Code:     "secret_exfiltration_instruction",
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
		Severity: "high",
		Code:     "persistent_state_backdoor",
		Category: "persistent-state",
		Any: []string{
			"modify soul.md",
			"overwrite soul.md",
			"edit soul.md",
			"append to soul.md",
			"write memory.md",
			"overwrite memory.md",
			"re-inject",
			"install backdoor",
			"command-and-control",
			"c2 implant",
		},
	},
	{
		Severity: "warning",
		Code:     "channel_takeover_instruction",
		Category: "channel-control",
		Any: []string{
			"add telegram bot",
			"attacker-controlled telegram",
			"add whatsapp channel",
			"attacker-controlled webhook",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_automation_instruction",
		Category: "automation-amplification",
		Any: []string{
			"retry forever",
			"loop forever",
			"infinite loop",
			"while true",
			"never stop",
			"continue indefinitely",
			"every 2 minutes",
		},
	},
	{
		Severity: "warning",
		Code:     "unreviewed_host_execution",
		Category: "host-execution",
		Any: []string{
			"rm -rf",
			"bash -c",
			"python -c",
			"curl http",
			"wget http",
			"execute shell command",
		},
	},
	{
		Severity: "info",
		Code:     "credential_persistence_instruction",
		Category: "credential-handling",
		Any: []string{
			"api key",
			"private key",
			"github_token",
			"github token",
		},
		All: []string{
			"memory",
		},
	},
}

func scanSoulRiskFindings(path, body string) []SoulRiskFinding {
	var findings []SoulRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range soulRiskRules {
			if !soulRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, SoulRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Path:     path,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortSoulRiskFindings(findings)
	return findings
}

func soulRiskRuleMatches(lowerLine string, rule soulRiskRule) bool {
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

func BuildSoulRiskReport(repoContext RepoContext) SoulRiskReport {
	report := SoulRiskReport{
		Status:                        "ok",
		Documents:                     len(repoContext.Documents),
		ScannedDocuments:              len(repoContext.Documents),
		RegistryVerification:          "not_configured",
		ProfileExportVerification:     "not_configured",
		RawBodiesIncluded:             false,
		LLME2ERequiredAfterRiskChange: true,
	}
	for _, doc := range repoContext.Documents {
		findings := scanSoulRiskFindings(doc.Path, doc.Body)
		if len(findings) > 0 {
			report.DocumentsWithRiskFindings++
		}
		report.Findings = append(report.Findings, findings...)
	}
	sortSoulRiskFindings(report.Findings)
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

func RenderSoulRiskReport(repoContext RepoContext) string {
	return renderSoulRiskReport(Event{}, repoContext, false)
}

func renderSoulRiskReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSoulRiskReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSoulRiskSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report scans loaded high-authority context files for body-free persistent-state risk categories inspired by OpenClaw/Hermes SOUL, memory, profile, and toolset safety boundaries. It reports only paths, categories, finding codes, and line hashes; raw soul, user, memory, tool, heartbeat, issue, comment, prompt, and secret bodies are not included.\n\n")

	b.WriteString("### Soul Risk Cards\n")
	if len(repoContext.Documents) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, doc := range repoContext.Documents {
			writeSoulRiskCard(&b, doc)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeSoulRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSoulRiskSummary(b *strings.Builder, report SoulRiskReport) {
	fmt.Fprintf(b, "- soul_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- context_documents: `%d`\n", report.Documents)
	fmt.Fprintf(b, "- scanned_documents: `%d`\n", report.ScannedDocuments)
	fmt.Fprintf(b, "- documents_with_risk_findings: `%d`\n", report.DocumentsWithRiskFindings)
	fmt.Fprintf(b, "- soul_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(b, "- profile_export_verification: `%s`\n", report.ProfileExportVerification)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_soul_risk_change: `%t`\n", report.LLME2ERequiredAfterRiskChange)
}

func writeSoulRiskCard(b *strings.Builder, doc ContextDocument) {
	findings := scanSoulRiskFindings(doc.Path, doc.Body)
	fmt.Fprintf(
		b,
		"- path=`%s` category=`%s` source=`%s` required=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		doc.Path,
		soulDocumentCategory(doc.Path),
		soulTrustSource(doc.Path),
		isRequiredSoulDocument(doc.Path),
		len(doc.Body),
		lineCount(doc.Body),
		shortDocumentHash(doc.Body),
		len(findings),
		soulRiskMaxSeverity(findings),
		inlineListOrNone(soulRiskCodes(findings)),
		inlineListOrNone(soulRiskLineHashes(findings)),
	)
}

func writeSoulRiskFindings(b *strings.Builder, findings []SoulRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` path=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Path,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func soulRiskCodes(findings []SoulRiskFinding) []string {
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

func soulRiskLineHashes(findings []SoulRiskFinding) []string {
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

func soulRiskMaxSeverity(findings []SoulRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if soulRiskSeverityRank(finding.Severity) > soulRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func soulRiskSeverityRank(severity string) int {
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

func sortSoulRiskFindings(findings []SoulRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if soulRiskSeverityRank(findings[i].Severity) != soulRiskSeverityRank(findings[j].Severity) {
			return soulRiskSeverityRank(findings[i].Severity) > soulRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Code < findings[j].Code
	})
}
