package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillProvenanceReport struct {
	Status                                   string
	Validation                               SkillValidationReport
	Risk                                     SkillRiskReport
	AvailableSkills                          int
	EnabledSkills                            int
	DisabledSkills                           int
	AllowlistBlockedSkills                   int
	SelectedSkills                           int
	RepoLocalSkills                          int
	CompatRootSkills                         int
	UnknownSourceSkills                      int
	GitTrackedSkills                         int
	UntrackedSkills                          int
	WorkingTreeDirtySkills                   int
	SkillsWithCommits                        int
	SkillsWithoutCommits                     int
	GitAvailable                             bool
	GitHistoryAvailable                      bool
	InstallerScriptsRun                      bool
	RepositoryMutationAllowed                bool
	RawBodiesIncluded                        bool
	RawGitSubjectsIncluded                   bool
	AuthorIdentitiesIncluded                 bool
	LLME2ERequiredAfterSkillProvenanceChange bool
	Cards                                    []SkillProvenanceCard
	Findings                                 []SkillProvenanceFinding
}

type SkillProvenanceCard struct {
	Name             string
	Path             string
	Folder           string
	Source           string
	Enabled          bool
	SelectedForTurn  bool
	Frontmatter      bool
	Description      bool
	Bytes            int
	Lines            int
	SHA              string
	RequiresEnv      int
	RequiresBins     int
	MissingEnv       int
	MissingBins      int
	RiskFindings     int
	RiskMaxSeverity  string
	RiskCodes        []string
	GitTracked       bool
	WorkingTreeDirty bool
	LastCommitSHA12  string
	LastCommitShort  string
	LastCommitDate   string
	SubjectSHA12     string
	CommitAvailable  bool
}

type SkillProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildSkillProvenanceReport(cfg Config, repoContext RepoContext) SkillProvenanceReport {
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	risk := BuildSkillRiskReport(repoContext.SkillSummaries)
	report := SkillProvenanceReport{
		Status:                                   skillProvenanceBaseStatus(validation.Status, risk.Status),
		Validation:                               validation,
		Risk:                                     risk,
		AvailableSkills:                          len(repoContext.SkillSummaries),
		SelectedSkills:                           len(repoContext.Skills),
		GitAvailable:                             soulGitAvailable(),
		InstallerScriptsRun:                      false,
		RepositoryMutationAllowed:                false,
		RawBodiesIncluded:                        false,
		RawGitSubjectsIncluded:                   false,
		AuthorIdentitiesIncluded:                 false,
		LLME2ERequiredAfterSkillProvenanceChange: true,
	}
	skills := append([]SkillSummary(nil), repoContext.SkillSummaries...)
	sort.Slice(skills, func(i, j int) bool { return skills[i].Path < skills[j].Path })
	for _, skill := range skills {
		if skillIsEnabled(skill) {
			report.EnabledSkills++
		}
		if skill.DisabledByConfig {
			report.DisabledSkills++
		}
		if skill.BlockedByAllowlist {
			report.AllowlistBlockedSkills++
		}
		source := skillTrustSource(skill.Path)
		switch source {
		case "repo-local":
			report.RepoLocalSkills++
		case "repo-local-compat":
			report.CompatRootSkills++
		default:
			report.UnknownSourceSkills++
			report.addFinding("warning", "unknown_skill_source", skill.Path, "skill path is outside known repo-local skill roots")
		}
		card := SkillProvenanceCard{
			Name:             skill.Name,
			Path:             skill.Path,
			Folder:           skillFolderName(skill.Path),
			Source:           source,
			Enabled:          skillIsEnabled(skill),
			SelectedForTurn:  skillSelectedForTurn(repoContext, skill),
			Frontmatter:      skill.FrontmatterPresent,
			Description:      strings.TrimSpace(skill.Description) != "",
			Bytes:            skill.Bytes,
			Lines:            skill.Lines,
			SHA:              skill.SHA,
			RequiresEnv:      len(skill.RequiredEnv),
			RequiresBins:     len(skill.RequiredBins),
			MissingEnv:       len(skill.MissingEnv),
			MissingBins:      len(skill.MissingBins),
			RiskFindings:     len(skill.RiskFindings),
			RiskMaxSeverity:  skillRiskMaxSeverity(skill.RiskFindings),
			RiskCodes:        skillRiskCodes(skill.RiskFindings),
			LastCommitSHA12:  "none",
			LastCommitShort:  "none",
			LastCommitDate:   "none",
			SubjectSHA12:     "none",
			CommitAvailable:  false,
			GitTracked:       false,
			WorkingTreeDirty: false,
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, skill.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedSkills++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, skill.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtySkills++
				report.addFinding("warning", "dirty_skill_file", skill.Path, "skill file has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, skill.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.SkillsWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.SkillsWithoutCommits++
				report.addFinding("warning", "missing_git_history", skill.Path, "no git commit was found for this skill file")
			}
		} else {
			report.UntrackedSkills++
			detail := "skill file is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_skill_file", skill.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for skill provenance checks")
	}
	if report.AvailableSkills > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local skill files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderSkillProvenanceCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillProvenanceReport(Event{}, cfg, repoContext, false)
}

func RenderSkillProvenanceReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderSkillProvenanceReport(ev, cfg, repoContext, true)
}

func renderSkillProvenanceReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillProvenanceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- skill_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", "repo-local-skill-git-history")
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", report.EnabledSkills)
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", report.DisabledSkills)
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", report.AllowlistBlockedSkills)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(&b, "- repo_local_skills: `%d`\n", report.RepoLocalSkills)
	fmt.Fprintf(&b, "- compat_root_skills: `%d`\n", report.CompatRootSkills)
	fmt.Fprintf(&b, "- unknown_source_skills: `%d`\n", report.UnknownSourceSkills)
	fmt.Fprintf(&b, "- git_tracked_skills: `%d`\n", report.GitTrackedSkills)
	fmt.Fprintf(&b, "- untracked_skills: `%d`\n", report.UntrackedSkills)
	fmt.Fprintf(&b, "- working_tree_dirty_skills: `%d`\n", report.WorkingTreeDirtySkills)
	fmt.Fprintf(&b, "- skills_with_commits: `%d`\n", report.SkillsWithCommits)
	fmt.Fprintf(&b, "- skills_without_commits: `%d`\n", report.SkillsWithoutCommits)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(&b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_provenance_change: `%t`\n", report.LLME2ERequiredAfterSkillProvenanceChange)
	writeSkillValidationSummary(&b, report.Validation)
	writeSkillRiskSummary(&b, report.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local skills to body-free git provenance. It reports skill metadata, hashes, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; raw skill bodies, issue bodies, comments, prompts, git subjects, author identities, installer output, and secret values are not included.\n\n")

	b.WriteString("### Skill Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeSkillProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", skillProvenanceGitGate(report))
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- installer_gate=`%s`\n", "disabled")

	b.WriteString("\n### Findings\n")
	writeSkillProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSkillProvenanceCard(b *strings.Builder, card SkillProvenanceCard) {
	fmt.Fprintf(
		b,
		"- name=`%s` path=`%s` folder=`%s` source=`%s` enabled=`%t` selected_for_this_turn=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		inlineCode(card.Name),
		card.Path,
		inlineCode(card.Folder),
		card.Source,
		card.Enabled,
		card.SelectedForTurn,
		card.Frontmatter,
		card.Description,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.RequiresEnv,
		card.RequiresBins,
		card.MissingEnv,
		card.MissingBins,
		card.RiskFindings,
		card.RiskMaxSeverity,
		inlineListOrNone(card.RiskCodes),
		card.GitTracked,
		card.WorkingTreeDirty,
		card.CommitAvailable,
		card.LastCommitSHA12,
		card.LastCommitShort,
		card.LastCommitDate,
		card.SubjectSHA12,
	)
}

func writeSkillProvenanceFindings(b *strings.Builder, findings []SkillProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func skillProvenanceBaseStatus(validationStatus, riskStatus string) string {
	if riskStatus == "high" {
		return "high"
	}
	if validationStatus == "error" {
		return "error"
	}
	if validationStatus == "warn" || riskStatus == "warn" {
		return "warn"
	}
	if validationStatus == "" && riskStatus == "" {
		return "unknown"
	}
	return "ok"
}

func skillProvenanceGitGate(report SkillProvenanceReport) string {
	if report.AvailableSkills == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedSkills > 0 || report.SkillsWithoutCommits > 0 || report.WorkingTreeDirtySkills > 0 {
		return "warn"
	}
	return "pass"
}

func (r *SkillProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SkillProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return r.Findings[i].Severity < r.Findings[j].Severity
		}
		if r.Findings[i].Code != r.Findings[j].Code {
			return r.Findings[i].Code < r.Findings[j].Code
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}
