package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillRiskFinding struct {
	Severity string
	Code     string
	Category string
	Path     string
	Line     int
	LineSHA  string
}

type SkillRiskReport struct {
	Status                  string
	Skills                  int
	ScannedSkills           int
	SkillsWithRiskFindings  int
	Findings                []SkillRiskFinding
	HighRiskFindings        int
	WarningRiskFindings     int
	InfoRiskFindings        int
	RegistryVerification    string
	InstallerScriptsRun     bool
	RawBodiesIncluded       bool
	LLME2ERequiredAfterRisk bool
}

type skillRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var skillRiskRules = []skillRiskRule{
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
		Severity: "warning",
		Code:     "unbounded_tool_loop",
		Category: "token-or-tool-amplification",
		Any: []string{
			"retry forever",
			"loop forever",
			"infinite loop",
			"while true",
			"never stop",
			"continue indefinitely",
			"keep calling",
		},
	},
	{
		Severity: "warning",
		Code:     "unreviewed_shell_execution",
		Category: "host-execution",
		Any: []string{
			"rm -rf",
			"bash -c",
			"python -c",
			"curl ",
			"wget ",
			"subprocess",
			"execute shell",
			"run shell",
		},
	},
	{
		Severity: "warning",
		Code:     "hidden_persistence_instruction",
		Category: "persistent-state",
		Any: []string{
			"silently persist",
			"persist without review",
			"write to memory",
			"edit memory",
			"modify soul",
			"append to soul",
		},
	},
	{
		Severity: "info",
		Code:     "credential_transfer_instruction",
		Category: "credential-handling",
		Any: []string{
			"api key",
			"private key",
			"github_token",
			"github token",
		},
		All: []string{
			"send",
			"upload",
			"post",
			"copy",
		},
	},
}

func scanSkillRiskFindings(path, body string) []SkillRiskFinding {
	var findings []SkillRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range skillRiskRules {
			if !skillRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, SkillRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Path:     path,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortSkillRiskFindings(findings)
	return findings
}

func skillRiskRuleMatches(lowerLine string, rule skillRiskRule) bool {
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

func BuildSkillRiskReport(skills []SkillSummary) SkillRiskReport {
	report := SkillRiskReport{
		Status:                  "ok",
		Skills:                  len(skills),
		ScannedSkills:           len(skills),
		RegistryVerification:    "not_configured",
		RawBodiesIncluded:       false,
		LLME2ERequiredAfterRisk: true,
	}
	for _, skill := range skills {
		if len(skill.RiskFindings) > 0 {
			report.SkillsWithRiskFindings++
		}
		report.Findings = append(report.Findings, skill.RiskFindings...)
	}
	sortSkillRiskFindings(report.Findings)
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

func RenderSkillsRiskReport(repoContext RepoContext) string {
	return renderSkillsRiskReport(Event{}, repoContext, false)
}

func renderSkillsRiskReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillRiskReport(repoContext.SkillSummaries)
	var b strings.Builder
	b.WriteString("## GitClaw Skills Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSkillRiskSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report scans repo-local skill instruction text for body-free risk categories inspired by OpenClaw/Hermes skill and toolset safety boundaries. It reports only paths, counts, finding codes, and line hashes; full `SKILL.md` bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Skill Risk Cards\n")
	if len(repoContext.SkillSummaries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range repoContext.SkillSummaries {
			writeSkillRiskCard(&b, skill)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeSkillRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSkillRiskSummary(b *strings.Builder, report SkillRiskReport) {
	fmt.Fprintf(b, "- skill_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- available_skills: `%d`\n", report.Skills)
	fmt.Fprintf(b, "- scanned_skills: `%d`\n", report.ScannedSkills)
	fmt.Fprintf(b, "- skills_with_risk_findings: `%d`\n", report.SkillsWithRiskFindings)
	fmt.Fprintf(b, "- skill_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_skill_risk_change: `%t`\n", report.LLME2ERequiredAfterRisk)
}

func writeSkillRiskCard(b *strings.Builder, skill SkillSummary) {
	fmt.Fprintf(
		b,
		"- name=`%s` path=`%s` enabled=`%t` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(skill.Name),
		skill.Path,
		skillIsEnabled(skill),
		skill.SHA,
		len(skill.RiskFindings),
		skillRiskMaxSeverity(skill.RiskFindings),
		inlineListOrNone(skillRiskCodes(skill.RiskFindings)),
		inlineListOrNone(skillRiskLineHashes(skill.RiskFindings)),
	)
}

func writeSkillRiskFindings(b *strings.Builder, findings []SkillRiskFinding) {
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

func skillRiskCodes(findings []SkillRiskFinding) []string {
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

func skillRiskLineHashes(findings []SkillRiskFinding) []string {
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

func skillRiskMaxSeverity(findings []SkillRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if skillRiskSeverityRank(finding.Severity) > skillRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func skillRiskSeverityRank(severity string) int {
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

func sortSkillRiskFindings(findings []SkillRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if skillRiskSeverityRank(findings[i].Severity) != skillRiskSeverityRank(findings[j].Severity) {
			return skillRiskSeverityRank(findings[i].Severity) > skillRiskSeverityRank(findings[j].Severity)
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
