package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillBundleRiskFinding struct {
	Severity string
	Code     string
	Category string
	Path     string
	Line     int
	LineSHA  string
}

type SkillBundleRiskReport struct {
	Status                         string
	Bundles                        int
	ScannedBundles                 int
	BundlesWithRiskFindings        int
	BundleSkillRefs                int
	ResolvedBundleSkills           int
	MissingBundleSkills            int
	BundlesWithInstruction         int
	Findings                       []SkillBundleRiskFinding
	HighRiskFindings               int
	WarningRiskFindings            int
	InfoRiskFindings               int
	ExternalRegistryVerification   string
	InstallerScriptsRun            bool
	AgentAuthoredMutationSupported bool
	RawBundleBodiesIncluded        bool
	RawBundleInstructionsIncluded  bool
	RawSkillBodiesIncluded         bool
	RawIssueBodiesIncluded         bool
	RawCommentBodiesIncluded       bool
	CredentialValuesIncluded       bool
	LLME2ERequiredAfterRisk        bool
}

type skillBundleRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var skillBundleRiskRules = []skillBundleRiskRule{
	{
		Severity: "high",
		Code:     "bundle_prompt_boundary_override",
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
		Code:     "bundle_secret_exfiltration_instruction",
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
		Code:     "bundle_unbounded_tool_loop",
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
		Code:     "bundle_unreviewed_shell_execution",
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
		Code:     "bundle_hidden_persistence_instruction",
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
		Severity: "warning",
		Code:     "bundle_external_delivery_instruction",
		Category: "external-delivery",
		Any: []string{
			"post to webhook",
			"send to webhook",
			"open a socket",
			"socket connection",
			"upload to http",
			"upload to https",
		},
	},
	{
		Severity: "warning",
		Code:     "bundle_remote_install_instruction",
		Category: "supply-chain",
		Any: []string{
			"install missing skill",
			"download missing skill",
			"install all dependencies",
			"curl | bash",
			"curl -fs",
		},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"send", "api key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"upload", "api key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"post", "api key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"copy", "api key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"send", "github token"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"upload", "github token"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"post", "github token"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"copy", "github token"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"send", "private key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"upload", "private key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"post", "private key"},
	},
	{
		Severity: "info",
		Code:     "bundle_credential_transfer_instruction",
		Category: "credential-handling",
		All:      []string{"copy", "private key"},
	},
}

func scanSkillBundleRiskFindings(path, body, parseError string) []SkillBundleRiskFinding {
	var findings []SkillBundleRiskFinding
	if strings.TrimSpace(parseError) != "" {
		findings = append(findings, SkillBundleRiskFinding{
			Severity: "warning",
			Code:     "bundle_yaml_parse_error",
			Category: "bundle-schema",
			Path:     path,
			Line:     0,
			LineSHA:  shortDocumentHash(parseError),
		})
	}
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range skillBundleRiskRules {
			if !skillBundleRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, SkillBundleRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Path:     path,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortSkillBundleRiskFindings(findings)
	return findings
}

func skillBundleRiskRuleMatches(lowerLine string, rule skillBundleRiskRule) bool {
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

func BuildSkillBundleRiskReport(bundles []SkillBundleSummary) SkillBundleRiskReport {
	report := SkillBundleRiskReport{
		Status:                         "ok",
		Bundles:                        len(bundles),
		ScannedBundles:                 len(bundles),
		BundleSkillRefs:                bundleSkillRefCount(bundles),
		ResolvedBundleSkills:           resolvedBundleSkillCount(bundles),
		MissingBundleSkills:            missingBundleSkillCount(bundles),
		BundlesWithInstruction:         bundlesWithInstructionCount(bundles),
		ExternalRegistryVerification:   "not_configured",
		RawBundleBodiesIncluded:        false,
		RawBundleInstructionsIncluded:  false,
		RawSkillBodiesIncluded:         false,
		RawIssueBodiesIncluded:         false,
		RawCommentBodiesIncluded:       false,
		CredentialValuesIncluded:       false,
		LLME2ERequiredAfterRisk:        true,
		AgentAuthoredMutationSupported: false,
	}
	pathsWithFindings := map[string]bool{}
	for _, bundle := range bundles {
		for _, finding := range bundle.RiskFindings {
			report.Findings = append(report.Findings, finding)
			pathsWithFindings[bundle.Path] = true
		}
		if len(bundle.Skills) == 0 {
			report.Findings = append(report.Findings, bundleMetadataRiskFinding("warning", "bundle_empty_skill_refs", "skill-resolution", bundle.Path, bundle.Name))
			pathsWithFindings[bundle.Path] = true
		}
		for _, missing := range bundle.MissingSkills {
			report.Findings = append(report.Findings, bundleMetadataRiskFinding("warning", "bundle_missing_skill_ref", "skill-resolution", bundle.Path, missing))
			pathsWithFindings[bundle.Path] = true
		}
	}
	for _, bundle := range duplicateSkillBundleSummaries(bundles) {
		report.Findings = append(report.Findings, bundleMetadataRiskFinding("warning", "duplicate_bundle_name", "bundle-schema", bundle.Path, bundle.Name))
		pathsWithFindings[bundle.Path] = true
	}
	sortSkillBundleRiskFindings(report.Findings)
	report.BundlesWithRiskFindings = len(pathsWithFindings)
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

func bundleMetadataRiskFinding(severity, code, category, path, value string) SkillBundleRiskFinding {
	return SkillBundleRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Path:     path,
		Line:     0,
		LineSHA:  shortDocumentHash(path + ":" + code + ":" + value),
	}
}

func duplicateSkillBundleSummaries(bundles []SkillBundleSummary) []SkillBundleSummary {
	counts := map[string]int{}
	for _, bundle := range bundles {
		counts[strings.ToLower(strings.TrimSpace(bundle.Name))]++
	}
	var duplicates []SkillBundleSummary
	for _, bundle := range bundles {
		if counts[strings.ToLower(strings.TrimSpace(bundle.Name))] > 1 {
			duplicates = append(duplicates, bundle)
		}
	}
	return duplicates
}

func RenderSkillBundlesRiskReport(repoContext RepoContext) string {
	return renderSkillBundlesRiskReport(Event{}, repoContext, false)
}

func renderSkillBundlesRiskReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillBundleRiskReport(repoContext.SkillBundles)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Bundle Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSkillBundleRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans repo-local skill bundle YAML and bundle instructions for body-free risk categories inspired by Hermes skill bundles and OpenClaw skill safety boundaries. It reports only paths, counts, finding codes, and hashes; bundle YAML bodies, bundle instructions, skill bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Bundle Risk Cards\n")
	if len(repoContext.SkillBundles) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, bundle := range repoContext.SkillBundles {
			writeSkillBundleRiskCard(&b, bundle, report.Findings)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeSkillBundleRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSkillBundleRiskSummary(b *strings.Builder, report SkillBundleRiskReport) {
	fmt.Fprintf(b, "- bundle_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- available_bundles: `%d`\n", report.Bundles)
	fmt.Fprintf(b, "- scanned_bundles: `%d`\n", report.ScannedBundles)
	fmt.Fprintf(b, "- bundles_with_risk_findings: `%d`\n", report.BundlesWithRiskFindings)
	fmt.Fprintf(b, "- bundle_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- bundle_skill_refs: `%d`\n", report.BundleSkillRefs)
	fmt.Fprintf(b, "- resolved_bundle_skills: `%d`\n", report.ResolvedBundleSkills)
	fmt.Fprintf(b, "- missing_bundle_skills: `%d`\n", report.MissingBundleSkills)
	fmt.Fprintf(b, "- bundles_with_instruction: `%d`\n", report.BundlesWithInstruction)
	fmt.Fprintf(b, "- external_registry_verification: `%s`\n", report.ExternalRegistryVerification)
	fmt.Fprintf(b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(b, "- agent_authored_bundle_mutation_supported: `%t`\n", report.AgentAuthoredMutationSupported)
	fmt.Fprintf(b, "- raw_bundle_bodies_included: `%t`\n", report.RawBundleBodiesIncluded)
	fmt.Fprintf(b, "- raw_bundle_instructions_included: `%t`\n", report.RawBundleInstructionsIncluded)
	fmt.Fprintf(b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_bundle_risk_change: `%t`\n", report.LLME2ERequiredAfterRisk)
}

func writeSkillBundleRiskCard(b *strings.Builder, bundle SkillBundleSummary, reportFindings []SkillBundleRiskFinding) {
	findings := skillBundleFindingsForPath(bundle.Path, reportFindings)
	fmt.Fprintf(
		b,
		"- bundle_name=`%s` path=`%s` selected_for_this_turn=`%t` instruction=`%t` skills=`%s` resolved_skills=`%s` missing_skills=`%s` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(bundle.Name),
		bundle.Path,
		bundle.Selected,
		bundle.InstructionPresent,
		inlineList(bundle.Skills),
		inlineList(bundle.ResolvedSkills),
		inlineList(bundle.MissingSkills),
		bundle.SHA,
		len(findings),
		skillBundleRiskMaxSeverity(findings),
		inlineListOrNone(skillBundleRiskCodes(findings)),
		inlineListOrNone(skillBundleRiskLineHashes(findings)),
	)
}

func writeSkillBundleRiskFindings(b *strings.Builder, findings []SkillBundleRiskFinding) {
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

func skillBundleFindingsForPath(path string, findings []SkillBundleRiskFinding) []SkillBundleRiskFinding {
	var matches []SkillBundleRiskFinding
	for _, finding := range findings {
		if finding.Path == path {
			matches = append(matches, finding)
		}
	}
	return matches
}

func skillBundleRiskCodes(findings []SkillBundleRiskFinding) []string {
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

func skillBundleRiskLineHashes(findings []SkillBundleRiskFinding) []string {
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

func skillBundleRiskMaxSeverity(findings []SkillBundleRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if skillBundleRiskSeverityRank(finding.Severity) > skillBundleRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func skillBundleRiskSeverityRank(severity string) int {
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

func sortSkillBundleRiskFindings(findings []SkillBundleRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if skillBundleRiskSeverityRank(findings[i].Severity) != skillBundleRiskSeverityRank(findings[j].Severity) {
			return skillBundleRiskSeverityRank(findings[i].Severity) > skillBundleRiskSeverityRank(findings[j].Severity)
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
