package gitclaw

import (
	"fmt"
	"strings"
)

type SkillVerifyReport struct {
	Status                    string
	Skills                    int
	Validation                SkillValidationReport
	RepoLocalSkills           int
	CompatRootSkills          int
	UnknownSourceSkills       int
	SkillsWithHashes          int
	SkillsWithRequirements    int
	SkillsMissingRequirements int
	RegistryVerification      string
	InstallerScriptsRun       bool
	RawBodiesIncluded         bool
}

func BuildSkillVerifyReport(skills []SkillSummary) SkillVerifyReport {
	validation := ValidateSkillSummaries(skills)
	report := SkillVerifyReport{
		Status:               validation.Status,
		Skills:               len(skills),
		Validation:           validation,
		RegistryVerification: "not_configured",
		RawBodiesIncluded:    false,
	}
	for _, skill := range skills {
		switch skillTrustSource(skill.Path) {
		case "repo-local":
			report.RepoLocalSkills++
		case "repo-local-compat":
			report.CompatRootSkills++
		default:
			report.UnknownSourceSkills++
		}
		if strings.TrimSpace(skill.SHA) != "" {
			report.SkillsWithHashes++
		}
		if len(skill.RequiredEnv) > 0 || len(skill.RequiredBins) > 0 {
			report.SkillsWithRequirements++
		}
		if len(skill.MissingEnv) > 0 || len(skill.MissingBins) > 0 {
			report.SkillsMissingRequirements++
		}
	}
	if report.UnknownSourceSkills > 0 && report.Status == "ok" {
		report.Status = "warn"
	}
	return report
}

func RenderSkillsVerifyReport(repoContext RepoContext) string {
	return renderSkillsVerifyReport(Event{}, repoContext, false)
}

func renderSkillsVerifyReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillVerifyReport(repoContext.SkillSummaries)
	var b strings.Builder
	b.WriteString("## GitClaw Skills Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- skill_verify_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "repo-local-metadata")
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.Skills)
	fmt.Fprintf(&b, "- repo_local_skills: `%d`\n", report.RepoLocalSkills)
	fmt.Fprintf(&b, "- compat_root_skills: `%d`\n", report.CompatRootSkills)
	fmt.Fprintf(&b, "- unknown_source_skills: `%d`\n", report.UnknownSourceSkills)
	fmt.Fprintf(&b, "- skills_with_hashes: `%d`\n", report.SkillsWithHashes)
	fmt.Fprintf(&b, "- skills_with_requirements: `%d`\n", report.SkillsWithRequirements)
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", report.SkillsMissingRequirements)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	writeSkillValidationSummary(&b, report.Validation)
	b.WriteByte('\n')
	b.WriteString("This report is GitClaw's local trust envelope for skills. It verifies repository-scoped skill metadata, source roots, requirement declarations, and body hashes. It does not contact an external registry, execute installers, dump full `SKILL.md` bodies, or include issue/comment text.\n\n")

	b.WriteString("### Trust Cards\n")
	if len(repoContext.SkillSummaries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range repoContext.SkillSummaries {
			writeSkillTrustCard(&b, skill)
		}
	}

	b.WriteString("\n### Verification Findings\n")
	writeSkillVerifyFindings(&b, report)
	return strings.TrimSpace(b.String())
}

func writeSkillTrustCard(b *strings.Builder, skill SkillSummary) {
	fmt.Fprintf(b, "- name=`%s` path=`%s` source=`%s` frontmatter=`%t` description=`%t` sha256_12=`%s` requirements=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d`\n",
		inlineCode(skill.Name),
		skill.Path,
		skillTrustSource(skill.Path),
		skill.FrontmatterPresent,
		strings.TrimSpace(skill.Description) != "",
		skill.SHA,
		skillRequirementStatus(skill),
		len(skill.RequiredEnv),
		len(skill.RequiredBins),
		len(skill.MissingEnv),
		len(skill.MissingBins),
	)
}

func writeSkillVerifyFindings(b *strings.Builder, report SkillVerifyReport) {
	wrote := false
	if report.RegistryVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`registry_verification_not_configured` detail=`external registry signatures are not part of GitClaw repo-local verification`\n")
		wrote = true
	}
	if report.UnknownSourceSkills > 0 {
		b.WriteString("- severity=`warning` code=`unknown_skill_source` detail=`one or more skill paths are outside known repo-local skill roots`\n")
		wrote = true
	}
	for _, finding := range report.Validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
		wrote = true
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func skillTrustSource(path string) string {
	switch {
	case strings.HasPrefix(path, ".gitclaw/SKILLS/"):
		return "repo-local"
	case strings.HasPrefix(path, ".gitclaw/skills/"):
		return "repo-local-compat"
	default:
		return "unknown"
	}
}

func skillRequirementStatus(skill SkillSummary) string {
	if len(skill.MissingEnv) > 0 || len(skill.MissingBins) > 0 {
		return "missing"
	}
	if len(skill.RequiredEnv) > 0 || len(skill.RequiredBins) > 0 {
		return "declared-ok"
	}
	return "none"
}
