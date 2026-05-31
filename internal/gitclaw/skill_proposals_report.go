package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const skillProposalStorePath = ".gitclaw/skill-proposals"

type SkillProposalStoreReport struct {
	Status                            string
	Root                              string
	StorePath                         string
	ProposalFiles                     []SkillProposalFile
	Findings                          []SkillRiskFinding
	ProposalFilesWithFindings         int
	FrontmatterFiles                  int
	ProposalStatusPending             int
	ProposalStatusApplied             int
	ProposalStatusRejected            int
	ProposalStatusQuarantined         int
	ProposalStatusUnknown             int
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	ReviewPRRequired                  bool
	ProposalApplySupported            bool
	ProposalMutationAllowed           bool
	ActiveSkillWriteAllowed           bool
	RepositoryMutationAllowed         bool
	AutonomousSkillCreation           bool
	AutonomousSkillImprovement        bool
	RemoteFetchAllowed                bool
	InstallerScriptsRun               bool
	DependencyInstallAllowed          bool
	RawProposalBodiesIncluded         bool
	RawSkillBodiesIncluded            bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	LLME2ERequiredAfterProposalChange bool
}

type SkillProposalFile struct {
	Name               string
	Path               string
	Folder             string
	Status             string
	Action             string
	TitlePresent       bool
	ReasonPresent      bool
	FrontmatterPresent bool
	Bytes              int
	Lines              int
	SHA                string
	RiskFindings       []SkillRiskFinding
}

type skillProposalRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var skillProposalRules = []skillProposalRule{
	{
		Severity: "high",
		Code:     "auto_apply_requested",
		Category: "proposal-apply-boundary",
		Any: []string{
			"approvalpolicy: auto",
			"approval policy: auto",
			"auto-apply",
			"auto apply",
			"apply without review",
			"merge without review",
		},
	},
	{
		Severity: "warning",
		Code:     "active_skill_write_requested",
		Category: "proposal-apply-boundary",
		Any: []string{
			".gitclaw/skills/",
			"write skill.md",
			"overwrite skill.md",
			"update active skill",
			"modify active skill",
		},
	},
	{
		Severity: "warning",
		Code:     "support_script_execution_requested",
		Category: "host-execution",
		Any: []string{
			"run installer",
			"execute setup",
			"curl | bash",
			"curl -fs",
			"npm install",
			"pip install",
		},
	},
	{
		Severity: "info",
		Code:     "proposal_lifecycle_reference",
		Category: "proposal-lifecycle",
		Any: []string{
			"quarantine",
			"reject",
			"applied",
			"pending approval",
		},
	},
}

func BuildSkillProposalStoreReport(root string) SkillProposalStoreReport {
	if root == "" {
		root = "."
	}
	report := SkillProposalStoreReport{
		Status:                            "ok",
		Root:                              filepath.ToSlash(root),
		StorePath:                         skillProposalStorePath,
		ReviewPRRequired:                  true,
		ProposalApplySupported:            false,
		ProposalMutationAllowed:           false,
		ActiveSkillWriteAllowed:           false,
		RepositoryMutationAllowed:         false,
		AutonomousSkillCreation:           false,
		AutonomousSkillImprovement:        false,
		RemoteFetchAllowed:                false,
		InstallerScriptsRun:               false,
		DependencyInstallAllowed:          false,
		RawProposalBodiesIncluded:         false,
		RawSkillBodiesIncluded:            false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		LLME2ERequiredAfterProposalChange: true,
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		report.Findings = append(report.Findings, proposalMetadataFinding("high", "proposal_root_unreadable", "proposal-store", "", err.Error()))
		return finalizeSkillProposalStoreReport(report)
	}
	matches, _ := filepath.Glob(filepath.Join(absRoot, filepath.FromSlash(skillProposalStorePath), "*", "PROPOSAL.md"))
	sort.Strings(matches)
	for _, match := range matches {
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			report.Findings = append(report.Findings, proposalMetadataFinding("high", "proposal_path_unreadable", "proposal-store", filepath.ToSlash(match), err.Error()))
			continue
		}
		proposal := inspectSkillProposalFile(absRoot, filepath.ToSlash(rel))
		report.ProposalFiles = append(report.ProposalFiles, proposal)
		report.Findings = append(report.Findings, proposal.RiskFindings...)
		if len(proposal.RiskFindings) > 0 {
			report.ProposalFilesWithFindings++
		}
		if proposal.FrontmatterPresent {
			report.FrontmatterFiles++
		}
		switch proposal.Status {
		case "pending":
			report.ProposalStatusPending++
		case "applied":
			report.ProposalStatusApplied++
		case "rejected":
			report.ProposalStatusRejected++
		case "quarantined":
			report.ProposalStatusQuarantined++
		default:
			report.ProposalStatusUnknown++
		}
	}
	return finalizeSkillProposalStoreReport(report)
}

func inspectSkillProposalFile(absRoot, relPath string) SkillProposalFile {
	folder := filepath.Base(filepath.Dir(filepath.FromSlash(relPath)))
	proposal := SkillProposalFile{
		Name:   folder,
		Path:   relPath,
		Folder: folder,
		Status: "unknown",
		Action: "unknown",
	}
	data, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	if err != nil {
		proposal.RiskFindings = append(proposal.RiskFindings, proposalMetadataFinding("high", "proposal_file_unreadable", "proposal-store", relPath, err.Error()))
		return proposal
	}
	body := string(data)
	proposal.Bytes = len(data)
	proposal.Lines = lineCount(body)
	proposal.SHA = shortDocumentHash(body)
	if fm, ok := frontmatter(body); ok {
		proposal.FrontmatterPresent = true
		if value := frontmatterValue(fm, "name"); value != "" {
			proposal.Name = normalizeSkillInstallCandidate(value)
		}
		if value := frontmatterValue(fm, "skillName"); value != "" {
			proposal.Name = normalizeSkillInstallCandidate(value)
		}
		if value := frontmatterValue(fm, "status"); value != "" {
			proposal.Status = normalizeSkillProposalStatus(value)
		}
		if value := frontmatterValue(fm, "action"); value != "" {
			proposal.Action = normalizeSkillProposalAction(value)
		}
		if value := frontmatterValue(fm, "operation"); value != "" {
			proposal.Action = normalizeSkillProposalAction(value)
		}
		proposal.TitlePresent = strings.TrimSpace(frontmatterValue(fm, "title")) != ""
		proposal.ReasonPresent = strings.TrimSpace(frontmatterValue(fm, "reason")) != ""
	}
	proposal.RiskFindings = append(proposal.RiskFindings, scanSkillProposalRiskFindings(relPath, body)...)
	if !skillNamePattern.MatchString(folder) {
		proposal.RiskFindings = append(proposal.RiskFindings, proposalMetadataFinding("high", "proposal_folder_invalid", "path-safety", relPath, folder))
	}
	if proposal.Name == "" || !skillNamePattern.MatchString(proposal.Name) {
		proposal.RiskFindings = append(proposal.RiskFindings, proposalMetadataFinding("high", "proposal_name_invalid", "metadata", relPath, proposal.Name))
	}
	if proposal.FrontmatterPresent && proposal.Status == "unknown" {
		proposal.RiskFindings = append(proposal.RiskFindings, proposalMetadataFinding("warning", "proposal_status_unknown", "proposal-lifecycle", relPath, proposal.Status))
	}
	sortSkillRiskFindings(proposal.RiskFindings)
	return proposal
}

func normalizeSkillProposalStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending", "proposal":
		return "pending"
	case "applied":
		return "applied"
	case "rejected":
		return "rejected"
	case "quarantined", "quarantine":
		return "quarantined"
	default:
		return "unknown"
	}
}

func scanSkillProposalRiskFindings(path, body string) []SkillRiskFinding {
	findings := scanSkillRiskFindings(path, body)
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range skillProposalRules {
			if !skillProposalRuleMatches(lower, rule) {
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

func skillProposalRuleMatches(lowerLine string, rule skillProposalRule) bool {
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

func proposalMetadataFinding(severity, code, category, path, detail string) SkillRiskFinding {
	return SkillRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Path:     filepath.ToSlash(path),
		Line:     0,
		LineSHA:  shortDocumentHash(detail),
	}
}

func finalizeSkillProposalStoreReport(report SkillProposalStoreReport) SkillProposalStoreReport {
	sort.Slice(report.ProposalFiles, func(i, j int) bool { return report.ProposalFiles[i].Path < report.ProposalFiles[j].Path })
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

func RenderSkillProposalsCLIReport(cfg Config) string {
	return renderSkillProposalsReport(Event{}, cfg, false)
}

func renderSkillProposalsReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildSkillProposalStoreReport(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Proposals Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- proposal_store_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- proposal_store_path: `%s`\n", report.StorePath)
	fmt.Fprintf(&b, "- proposal_files: `%d`\n", len(report.ProposalFiles))
	fmt.Fprintf(&b, "- proposal_frontmatter_files: `%d`\n", report.FrontmatterFiles)
	fmt.Fprintf(&b, "- proposal_status_pending: `%d`\n", report.ProposalStatusPending)
	fmt.Fprintf(&b, "- proposal_status_applied: `%d`\n", report.ProposalStatusApplied)
	fmt.Fprintf(&b, "- proposal_status_rejected: `%d`\n", report.ProposalStatusRejected)
	fmt.Fprintf(&b, "- proposal_status_quarantined: `%d`\n", report.ProposalStatusQuarantined)
	fmt.Fprintf(&b, "- proposal_status_unknown: `%d`\n", report.ProposalStatusUnknown)
	fmt.Fprintf(&b, "- proposal_files_with_findings: `%d`\n", report.ProposalFilesWithFindings)
	fmt.Fprintf(&b, "- proposal_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(&b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(&b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(&b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", report.ReviewPRRequired)
	fmt.Fprintf(&b, "- proposal_apply_supported: `%t`\n", report.ProposalApplySupported)
	fmt.Fprintf(&b, "- proposal_mutation_allowed: `%t`\n", report.ProposalMutationAllowed)
	fmt.Fprintf(&b, "- active_skill_write_allowed: `%t`\n", report.ActiveSkillWriteAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- autonomous_skill_creation: `%t`\n", report.AutonomousSkillCreation)
	fmt.Fprintf(&b, "- autonomous_skill_improvement: `%t`\n", report.AutonomousSkillImprovement)
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", report.RemoteFetchAllowed)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(&b, "- raw_proposal_bodies_included: `%t`\n", report.RawProposalBodiesIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_proposal_change: `%t`\n", report.LLME2ERequiredAfterProposalChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report inventories repo-reviewed skill proposal files without activating them. It reports proposal metadata, lifecycle counts, risk codes, and line hashes only; proposal bodies, skill bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Proposal Files\n")
	if len(report.ProposalFiles) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, proposal := range report.ProposalFiles {
			writeSkillProposalFileCard(&b, proposal)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeSkillRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSkillProposalFileCard(b *strings.Builder, proposal SkillProposalFile) {
	fmt.Fprintf(
		b,
		"- proposal_name=`%s` path=`%s` folder=`%s` status=`%s` action=`%s` frontmatter=`%t` title=`%t` reason=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(proposal.Name),
		proposal.Path,
		proposal.Folder,
		proposal.Status,
		proposal.Action,
		proposal.FrontmatterPresent,
		proposal.TitlePresent,
		proposal.ReasonPresent,
		proposal.Bytes,
		proposal.Lines,
		proposal.SHA,
		len(proposal.RiskFindings),
		skillRiskMaxSeverity(proposal.RiskFindings),
		inlineListOrNone(skillRiskCodes(proposal.RiskFindings)),
		inlineListOrNone(skillRiskLineHashes(proposal.RiskFindings)),
	)
}
