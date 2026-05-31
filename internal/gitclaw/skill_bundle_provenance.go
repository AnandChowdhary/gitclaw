package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillBundleProvenanceReport struct {
	Status                                    string
	AvailableBundles                          int
	SelectedBundles                           int
	BundleSkillRefs                           int
	ResolvedBundleSkills                      int
	MissingBundleSkills                       int
	BundlesWithInstruction                    int
	GitTrackedBundles                         int
	UntrackedBundles                          int
	WorkingTreeDirtyBundles                   int
	BundlesWithCommits                        int
	BundlesWithoutCommits                     int
	GitAvailable                              bool
	GitHistoryAvailable                       bool
	RepositoryMutationAllowed                 bool
	AgentAuthoredBundleMutationSupported      bool
	RawBundleBodiesIncluded                   bool
	RawBundleInstructionsIncluded             bool
	RawSkillBodiesIncluded                    bool
	RawGitSubjectsIncluded                    bool
	AuthorIdentitiesIncluded                  bool
	LLME2ERequiredAfterBundleProvenanceChange bool
	Cards                                     []SkillBundleProvenanceCard
	Findings                                  []SkillBundleProvenanceFinding
}

type SkillBundleProvenanceCard struct {
	Name               string
	Path               string
	Skills             []string
	ResolvedSkills     []string
	MissingSkills      []string
	SelectedForTurn    bool
	InstructionPresent bool
	InstructionSHA12   string
	Bytes              int
	Lines              int
	SHA                string
	ParseError         string
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	GitTracked         bool
	WorkingTreeDirty   bool
	CommitAvailable    bool
	LastCommitSHA12    string
	LastCommitShort    string
	LastCommitDate     string
	SubjectSHA12       string
}

type SkillBundleProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildSkillBundleProvenanceReport(cfg Config, repoContext RepoContext) SkillBundleProvenanceReport {
	report := SkillBundleProvenanceReport{
		Status:                                    "ok",
		AvailableBundles:                          len(repoContext.SkillBundles),
		SelectedBundles:                           selectedSkillBundleCount(repoContext.SkillBundles),
		BundleSkillRefs:                           bundleSkillRefCount(repoContext.SkillBundles),
		ResolvedBundleSkills:                      resolvedBundleSkillCount(repoContext.SkillBundles),
		MissingBundleSkills:                       missingBundleSkillCount(repoContext.SkillBundles),
		BundlesWithInstruction:                    bundlesWithInstructionCount(repoContext.SkillBundles),
		GitAvailable:                              soulGitAvailable(),
		RepositoryMutationAllowed:                 false,
		AgentAuthoredBundleMutationSupported:      false,
		RawBundleBodiesIncluded:                   false,
		RawBundleInstructionsIncluded:             false,
		RawSkillBodiesIncluded:                    false,
		RawGitSubjectsIncluded:                    false,
		AuthorIdentitiesIncluded:                  false,
		LLME2ERequiredAfterBundleProvenanceChange: true,
	}
	bundles := append([]SkillBundleSummary(nil), repoContext.SkillBundles...)
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Path < bundles[j].Path })
	for _, bundle := range bundles {
		card := SkillBundleProvenanceCard{
			Name:               bundle.Name,
			Path:               bundle.Path,
			Skills:             append([]string(nil), bundle.Skills...),
			ResolvedSkills:     append([]string(nil), bundle.ResolvedSkills...),
			MissingSkills:      append([]string(nil), bundle.MissingSkills...),
			SelectedForTurn:    bundle.Selected,
			InstructionPresent: bundle.InstructionPresent,
			InstructionSHA12:   bundle.InstructionSHA,
			Bytes:              bundle.Bytes,
			Lines:              bundle.Lines,
			SHA:                bundle.SHA,
			ParseError:         bundle.ParseError,
			RiskFindings:       len(bundle.RiskFindings),
			RiskMaxSeverity:    skillBundleRiskMaxSeverity(bundle.RiskFindings),
			RiskCodes:          skillBundleRiskCodes(bundle.RiskFindings),
			LastCommitSHA12:    "none",
			LastCommitShort:    "none",
			LastCommitDate:     "none",
			SubjectSHA12:       "none",
		}
		if len(bundle.MissingSkills) > 0 {
			report.addFinding("warning", "missing_bundle_skill_refs", bundle.Path, fmt.Sprintf("missing_skill_refs=%d", len(bundle.MissingSkills)))
		}
		if bundle.ParseError != "" {
			report.addFinding("warning", "bundle_parse_error", bundle.Path, "bundle YAML parse error is present")
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, bundle.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedBundles++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, bundle.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtyBundles++
				report.addFinding("warning", "dirty_bundle_file", bundle.Path, "bundle file has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, bundle.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.BundlesWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.BundlesWithoutCommits++
				report.addFinding("warning", "missing_bundle_git_history", bundle.Path, "no git commit was found for this bundle file")
			}
		} else {
			report.UntrackedBundles++
			detail := "bundle file is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_bundle_file", bundle.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for bundle provenance checks")
	}
	if report.AvailableBundles > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "bundle_git_history_not_available", "git", "no commit history was available for repo-local bundle files")
	}
	if report.MissingBundleSkills > 0 || report.UntrackedBundles > 0 || report.WorkingTreeDirtyBundles > 0 || report.BundlesWithoutCommits > 0 || len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderSkillBundleProvenanceCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillBundleProvenanceReport(Event{}, cfg, repoContext, false)
}

func renderSkillBundleProvenanceReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillBundleProvenanceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Bundle Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSkillBundleProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local skill bundle YAML files to body-free git provenance. It reports bundle metadata, skill-ref counts, instruction hashes, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; raw bundle YAML, bundle instructions, skill bodies, issue bodies, comments, prompts, git subjects, author identities, provider payloads, and secret values are not included.\n\n")

	b.WriteString("### Bundle Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeSkillBundleProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", skillBundleProvenanceGitGate(report))
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- agent_authored_mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")

	b.WriteString("\n### Findings\n")
	writeSkillBundleProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSkillBundleProvenanceSummary(b *strings.Builder, report SkillBundleProvenanceReport) {
	fmt.Fprintf(b, "- bundle_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", "repo-local-skill-bundle-git-history")
	fmt.Fprintf(b, "- available_bundles: `%d`\n", report.AvailableBundles)
	fmt.Fprintf(b, "- selected_bundles: `%d`\n", report.SelectedBundles)
	fmt.Fprintf(b, "- bundle_skill_refs: `%d`\n", report.BundleSkillRefs)
	fmt.Fprintf(b, "- resolved_bundle_skills: `%d`\n", report.ResolvedBundleSkills)
	fmt.Fprintf(b, "- missing_bundle_skills: `%d`\n", report.MissingBundleSkills)
	fmt.Fprintf(b, "- bundles_with_instruction: `%d`\n", report.BundlesWithInstruction)
	fmt.Fprintf(b, "- git_tracked_bundles: `%d`\n", report.GitTrackedBundles)
	fmt.Fprintf(b, "- untracked_bundles: `%d`\n", report.UntrackedBundles)
	fmt.Fprintf(b, "- working_tree_dirty_bundles: `%d`\n", report.WorkingTreeDirtyBundles)
	fmt.Fprintf(b, "- bundles_with_commits: `%d`\n", report.BundlesWithCommits)
	fmt.Fprintf(b, "- bundles_without_commits: `%d`\n", report.BundlesWithoutCommits)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- agent_authored_bundle_mutation_supported: `%t`\n", report.AgentAuthoredBundleMutationSupported)
	fmt.Fprintf(b, "- raw_bundle_bodies_included: `%t`\n", report.RawBundleBodiesIncluded)
	fmt.Fprintf(b, "- raw_bundle_instructions_included: `%t`\n", report.RawBundleInstructionsIncluded)
	fmt.Fprintf(b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_bundle_provenance_change: `%t`\n", report.LLME2ERequiredAfterBundleProvenanceChange)
}

func writeSkillBundleProvenanceCard(b *strings.Builder, card SkillBundleProvenanceCard) {
	fmt.Fprintf(
		b,
		"- bundle_name=`%s` path=`%s` skills=`%s` resolved_skills=`%s` missing_skills=`%s` selected_for_this_turn=`%t` instruction=`%t` instruction_sha256_12=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` parse_error=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		inlineCode(card.Name),
		card.Path,
		inlineListOrNone(card.Skills),
		inlineListOrNone(card.ResolvedSkills),
		inlineListOrNone(card.MissingSkills),
		card.SelectedForTurn,
		card.InstructionPresent,
		card.InstructionSHA12,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.ParseError != "",
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

func writeSkillBundleProvenanceFindings(b *strings.Builder, findings []SkillBundleProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func skillBundleProvenanceGitGate(report SkillBundleProvenanceReport) string {
	if report.AvailableBundles == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedBundles > 0 || report.BundlesWithoutCommits > 0 || report.WorkingTreeDirtyBundles > 0 {
		return "warn"
	}
	return "pass"
}

func (r *SkillBundleProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SkillBundleProvenanceFinding{
		Severity: severity,
		Code:     code,
		Path:     path,
		Detail:   detail,
	})
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
