package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillRuntimeReport struct {
	Status                                string
	Skills                                int
	SkillsWithFrontmatter                 int
	SkillsWithRuntimeMetadata             int
	SkillsWithRequirements                int
	SkillsMissingRequirements             int
	RequiredEnvDeclarations               int
	OptionalEnvDeclarations               int
	PrimaryEnvDeclarations                int
	PrimaryEnvMatchedDeclarations         int
	PrimaryEnvMismatches                  int
	RequiredBinDeclarations               int
	InstallSpecs                          int
	InstallBins                           int
	SkillsWithInstallSpecs                int
	InstallerScriptsRun                   bool
	DependencyInstallAllowed              bool
	RegistryContactAllowed                bool
	RepositoryMutationAllowed             bool
	RawSkillBodiesIncluded                bool
	RawEnvNamesIncluded                   bool
	RawInstallTargetsIncluded             bool
	LLME2ERequiredAfterSkillRuntimeChange bool
	Findings                              []SkillRuntimeFinding
}

type SkillRuntimeFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func RenderSkillRuntimeCLIReport(repoContext RepoContext) string {
	return renderSkillRuntimeReport(Event{}, repoContext, false)
}

func renderSkillRuntimeReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillRuntimeReport(repoContext.SkillSummaries)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Runtime Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSkillRuntimeSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report audits OpenClaw-compatible skill runtime metadata from repo-local `SKILL.md` frontmatter. It records env/bin/install declarations as counts, hashes, and gates only; GitClaw does not fetch registries, run installers, install dependencies, mutate skills, or print raw skill bodies, env names, install targets, issue bodies, comments, prompts, or tool outputs.\n\n")

	b.WriteString("### Runtime Cards\n")
	if len(repoContext.SkillSummaries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range repoContext.SkillSummaries {
			writeSkillRuntimeCard(&b, skill)
		}
	}

	b.WriteString("\n### Runtime Gates\n")
	b.WriteString("- registry_contact_allowed=`false`\n")
	b.WriteString("- installer_scripts_run=`false`\n")
	b.WriteString("- dependency_install_allowed=`false`\n")
	b.WriteString("- repository_mutation_allowed=`false`\n")
	b.WriteString("- raw_metadata_gate=`hash_only`\n")

	b.WriteString("\n### Runtime Findings\n")
	writeSkillRuntimeFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildSkillRuntimeReport(skills []SkillSummary) SkillRuntimeReport {
	report := SkillRuntimeReport{
		Status:                                "ok",
		Skills:                                len(skills),
		LLME2ERequiredAfterSkillRuntimeChange: true,
	}
	for _, skill := range skills {
		if skill.FrontmatterPresent {
			report.SkillsWithFrontmatter++
		}
		if skillRuntimeMetadataPresent(skill) {
			report.SkillsWithRuntimeMetadata++
		}
		if len(skill.RequiredEnv) > 0 || len(skill.RequiredBins) > 0 {
			report.SkillsWithRequirements++
		}
		if skillIsEnabled(skill) && (len(skill.MissingEnv) > 0 || len(skill.MissingBins) > 0) {
			report.SkillsMissingRequirements++
			report.addFinding("warning", "missing_runtime_requirements", skill.Path, fmt.Sprintf("missing_env=%d missing_bins=%d", len(skill.MissingEnv), len(skill.MissingBins)))
		}
		report.RequiredEnvDeclarations += len(skill.RequiredEnv)
		report.OptionalEnvDeclarations += len(skill.OptionalEnv)
		report.RequiredBinDeclarations += len(skill.RequiredBins)
		if strings.TrimSpace(skill.PrimaryEnv) != "" {
			report.PrimaryEnvDeclarations++
			if skillPrimaryEnvDeclared(skill) {
				report.PrimaryEnvMatchedDeclarations++
			} else {
				report.PrimaryEnvMismatches++
				report.addFinding("warning", "primary_env_not_declared", skill.Path, "primaryEnv should also be declared in required or optional env metadata")
			}
		}
		if len(skill.InstallSpecs) > 0 {
			report.SkillsWithInstallSpecs++
			report.InstallSpecs += len(skill.InstallSpecs)
			report.InstallBins += skillInstallBinCount(skill.InstallSpecs)
			report.addFinding("warning", "declared_install_specs_inert", skill.Path, "install specs are metadata only; GitClaw does not run dependency installers")
		}
	}
	if report.SkillsMissingRequirements > 0 || report.PrimaryEnvMismatches > 0 || report.SkillsWithInstallSpecs > 0 {
		report.Status = "warn"
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity < report.Findings[j].Severity
		}
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return report.Findings[i].Path < report.Findings[j].Path
	})
	return report
}

func writeSkillRuntimeSummary(b *strings.Builder, report SkillRuntimeReport) {
	fmt.Fprintf(b, "- skill_runtime_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- runtime_metadata_scope: `%s`\n", "repo-local-skill-frontmatter")
	fmt.Fprintf(b, "- available_skills: `%d`\n", report.Skills)
	fmt.Fprintf(b, "- skills_with_frontmatter: `%d`\n", report.SkillsWithFrontmatter)
	fmt.Fprintf(b, "- skills_with_runtime_metadata: `%d`\n", report.SkillsWithRuntimeMetadata)
	fmt.Fprintf(b, "- skills_with_requirements: `%d`\n", report.SkillsWithRequirements)
	fmt.Fprintf(b, "- skills_missing_requirements: `%d`\n", report.SkillsMissingRequirements)
	fmt.Fprintf(b, "- required_env_declarations: `%d`\n", report.RequiredEnvDeclarations)
	fmt.Fprintf(b, "- optional_env_declarations: `%d`\n", report.OptionalEnvDeclarations)
	fmt.Fprintf(b, "- primary_env_declarations: `%d`\n", report.PrimaryEnvDeclarations)
	fmt.Fprintf(b, "- primary_env_matched_declarations: `%d`\n", report.PrimaryEnvMatchedDeclarations)
	fmt.Fprintf(b, "- primary_env_mismatches: `%d`\n", report.PrimaryEnvMismatches)
	fmt.Fprintf(b, "- required_bin_declarations: `%d`\n", report.RequiredBinDeclarations)
	fmt.Fprintf(b, "- install_specs: `%d`\n", report.InstallSpecs)
	fmt.Fprintf(b, "- install_bins: `%d`\n", report.InstallBins)
	fmt.Fprintf(b, "- skills_with_install_specs: `%d`\n", report.SkillsWithInstallSpecs)
	fmt.Fprintf(b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(b, "- raw_env_names_included: `%t`\n", report.RawEnvNamesIncluded)
	fmt.Fprintf(b, "- raw_install_targets_included: `%t`\n", report.RawInstallTargetsIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_skill_runtime_change: `%t`\n", report.LLME2ERequiredAfterSkillRuntimeChange)
}

func writeSkillRuntimeCard(b *strings.Builder, skill SkillSummary) {
	fmt.Fprintf(
		b,
		"- name=`%s` path=`%s` enabled=`%t` frontmatter=`%t` runtime_metadata=`%t` always=`%t` required_env=`%d` optional_env=`%d` required_env_sha256_12=`%s` optional_env_sha256_12=`%s` primary_env_present=`%t` primary_env_declared=`%t` primary_env_sha256_12=`%s` required_bins=`%d` required_bins_sha256_12=`%s` missing_env=`%d` missing_bins=`%d` install_specs=`%d` install_kinds=`%s` install_bins=`%d` install_targets_sha256_12=`%s` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
		inlineCode(skill.Name),
		skill.Path,
		skillIsEnabled(skill),
		skill.FrontmatterPresent,
		skillRuntimeMetadataPresent(skill),
		skill.Always,
		len(skill.RequiredEnv),
		len(skill.OptionalEnv),
		hashStringList(skill.RequiredEnv),
		hashStringList(skill.OptionalEnv),
		strings.TrimSpace(skill.PrimaryEnv) != "",
		skillPrimaryEnvDeclared(skill),
		hashStringOrNone(skill.PrimaryEnv),
		len(skill.RequiredBins),
		hashStringList(skill.RequiredBins),
		len(skill.MissingEnv),
		len(skill.MissingBins),
		len(skill.InstallSpecs),
		inlineListOrNone(skillInstallKinds(skill.InstallSpecs)),
		skillInstallBinCount(skill.InstallSpecs),
		hashStringList(skillInstallTargets(skill.InstallSpecs)),
		skill.SHA,
		len(skill.RiskFindings),
		skillRiskMaxSeverity(skill.RiskFindings),
		inlineListOrNone(skillRiskCodes(skill.RiskFindings)),
	)
}

func writeSkillRuntimeFindings(b *strings.Builder, findings []SkillRuntimeFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func (r *SkillRuntimeReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SkillRuntimeFinding{
		Severity: severity,
		Code:     code,
		Path:     path,
		Detail:   detail,
	})
}

func skillRuntimeMetadataPresent(skill SkillSummary) bool {
	return len(skill.RequiredEnv) > 0 ||
		len(skill.RequiredBins) > 0 ||
		len(skill.OptionalEnv) > 0 ||
		strings.TrimSpace(skill.PrimaryEnv) != "" ||
		len(skill.InstallSpecs) > 0 ||
		skill.Always
}

func skillPrimaryEnvDeclared(skill SkillSummary) bool {
	primary := strings.TrimSpace(skill.PrimaryEnv)
	if primary == "" {
		return false
	}
	return containsStringFold(skill.RequiredEnv, primary) || containsStringFold(skill.OptionalEnv, primary)
}

func skillInstallKinds(specs []SkillInstallSpec) []string {
	kinds := make([]string, 0, len(specs))
	for _, spec := range specs {
		kinds = append(kinds, spec.Kind)
	}
	return uniqueSortedStrings(kinds)
}

func skillInstallTargets(specs []SkillInstallSpec) []string {
	var targets []string
	for _, spec := range specs {
		targets = append(targets, spec.Bins...)
	}
	return uniqueSortedStrings(targets)
}

func skillInstallBinCount(specs []SkillInstallSpec) int {
	return len(skillInstallTargets(specs))
}

func hashStringList(values []string) string {
	values = uniqueSortedStrings(values)
	if len(values) == 0 {
		return "none"
	}
	return shortDocumentHash(strings.Join(values, "\n"))
}

func hashStringOrNone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	return shortDocumentHash(value)
}

func isSkillsRuntimeRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/skills" {
		return false
	}
	subcommand := strings.Trim(strings.ToLower(fields[1]), " \t\r\n.,:;!?")
	return subcommand == "runtime" || subcommand == "requirements" || subcommand == "metadata"
}
