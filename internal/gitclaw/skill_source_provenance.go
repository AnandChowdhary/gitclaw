package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillSourceProvenanceReport struct {
	Status                                         string
	Sources                                        SkillSourceReport
	ProvenanceSurfaces                             int
	RepoLocalSurfaces                              int
	UnknownSourceSurfaces                          int
	GitTrackedSurfaces                             int
	UntrackedSurfaces                              int
	WorkingTreeDirtySurfaces                       int
	SurfacesWithCommits                            int
	SurfacesWithoutCommits                         int
	GitAvailable                                   bool
	GitHistoryAvailable                            bool
	RegistryContactAllowed                         bool
	RemoteFetchAllowed                             bool
	InstallerScriptsRun                            bool
	DependencyInstallAllowed                       bool
	RepositoryMutationAllowed                      bool
	RawSourceBodiesIncluded                        bool
	RawSourceRefsIncluded                          bool
	RawSkillBodiesIncluded                         bool
	RawIssueBodiesIncluded                         bool
	RawCommentBodiesIncluded                       bool
	RawPromptBodiesIncluded                        bool
	RawGitSubjectsIncluded                         bool
	AuthorIdentitiesIncluded                       bool
	CredentialValuesIncluded                       bool
	LLME2ERequiredAfterSkillSourceProvenanceChange bool
	Cards                                          []SkillSourceProvenanceCard
	Findings                                       []SkillSourceProvenanceFinding
}

type SkillSourceProvenanceCard struct {
	SourceName         string
	Path               string
	SkillPath          string
	Source             string
	SkillMatched       bool
	SkillSHA           string
	SourceKind         string
	SourceRefPresent   bool
	SourceRefSHA       string
	TrustLevel         string
	InstallMode        string
	ExpectedSHA        string
	HashPinned         bool
	HashMatched        bool
	HashMismatched     bool
	RequiresApproval   bool
	RemoteFetchAllowed bool
	Bytes              int
	Lines              int
	SHA                string
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	GitTracked         bool
	WorkingTreeDirty   bool
	LastCommitSHA12    string
	LastCommitShort    string
	LastCommitDate     string
	SubjectSHA12       string
	CommitAvailable    bool
}

type SkillSourceProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildSkillSourceProvenanceReport(cfg Config, repoContext RepoContext) SkillSourceProvenanceReport {
	sources := BuildSkillSourceReport(cfg, repoContext)
	report := SkillSourceProvenanceReport{
		Status:                                         skillSourceProvenanceBaseStatus(sources),
		Sources:                                        sources,
		GitAvailable:                                   soulGitAvailable(),
		RegistryContactAllowed:                         false,
		RemoteFetchAllowed:                             false,
		InstallerScriptsRun:                            false,
		DependencyInstallAllowed:                       false,
		RepositoryMutationAllowed:                      false,
		RawSourceBodiesIncluded:                        false,
		RawSourceRefsIncluded:                          false,
		RawSkillBodiesIncluded:                         false,
		RawIssueBodiesIncluded:                         false,
		RawCommentBodiesIncluded:                       false,
		RawPromptBodiesIncluded:                        false,
		RawGitSubjectsIncluded:                         false,
		AuthorIdentitiesIncluded:                       false,
		CredentialValuesIncluded:                       false,
		LLME2ERequiredAfterSkillSourceProvenanceChange: true,
	}
	for _, source := range sources.Cards {
		report.addCard(skillSourceProvenanceCard(cfg, source))
	}
	sort.Slice(report.Cards, func(i, j int) bool {
		if report.Cards[i].Path != report.Cards[j].Path {
			return report.Cards[i].Path < report.Cards[j].Path
		}
		return report.Cards[i].SourceName < report.Cards[j].SourceName
	})
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for skill source provenance checks")
	}
	if report.ProvenanceSurfaces > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local skill source pins")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderSkillSourceProvenanceCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillSourceProvenanceReport(Event{}, cfg, repoContext, false)
}

func renderSkillSourceProvenanceReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillSourceProvenanceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Source Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	writeSkillSourceProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-reviewed skill source pins to body-free git provenance. It reports source-pin paths, source kind, trust level, install mode, match/hash state, risk metadata, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; raw source-pin YAML, raw source refs, skill bodies, issue bodies, comments, prompts, git subjects, author identities, provider payloads, installer output, and secret values are not included.\n\n")

	b.WriteString("### Skill Source Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeSkillSourceProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Sources.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", skillSourceProvenanceGitGate(report))
	fmt.Fprintf(&b, "- source_pin_gate=`%s`\n", "repo-reviewed")
	fmt.Fprintf(&b, "- registry_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- remote_fetch_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- installer_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")

	b.WriteString("\n### Findings\n")
	writeSkillSourceProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSkillSourceProvenanceSummary(b *strings.Builder, report SkillSourceProvenanceReport) {
	fmt.Fprintf(b, "- skill_source_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", "repo-local-skill-source-git-history")
	fmt.Fprintf(b, "- skill_source_status: `%s`\n", report.Sources.Status)
	fmt.Fprintf(b, "- skill_source_specs_dir: `%s`\n", skillSourcesDir)
	fmt.Fprintf(b, "- skill_source_specs: `%d`\n", report.Sources.Specs)
	fmt.Fprintf(b, "- parsed_skill_source_specs: `%d`\n", report.Sources.ParsedSpecs)
	fmt.Fprintf(b, "- matched_skill_sources: `%d`\n", report.Sources.MatchedSources)
	fmt.Fprintf(b, "- missing_skill_source_matches: `%d`\n", report.Sources.MissingSkillMatches)
	fmt.Fprintf(b, "- hash_pinned_skill_sources: `%d`\n", report.Sources.HashPinnedSources)
	fmt.Fprintf(b, "- hash_matched_skill_sources: `%d`\n", report.Sources.HashMatchedSources)
	fmt.Fprintf(b, "- hash_mismatched_skill_sources: `%d`\n", report.Sources.HashMismatchedSources)
	fmt.Fprintf(b, "- repo_local_source_refs: `%d`\n", report.Sources.RepoLocalSourceRefs)
	fmt.Fprintf(b, "- remote_source_refs: `%d`\n", report.Sources.RemoteSourceRefs)
	fmt.Fprintf(b, "- sources_requiring_approval: `%d`\n", report.Sources.SourcesRequiringApproval)
	fmt.Fprintf(b, "- remote_fetch_allowed_specs: `%d`\n", report.Sources.RemoteFetchAllowedSpecs)
	fmt.Fprintf(b, "- sources_with_risk_findings: `%d`\n", report.Sources.SourcesWithRiskFindings)
	fmt.Fprintf(b, "- skill_source_risk_findings: `%d`\n", len(report.Sources.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.Sources.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.Sources.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.Sources.InfoRiskFindings)
	fmt.Fprintf(b, "- provenance_surfaces: `%d`\n", report.ProvenanceSurfaces)
	fmt.Fprintf(b, "- repo_local_surfaces: `%d`\n", report.RepoLocalSurfaces)
	fmt.Fprintf(b, "- unknown_source_surfaces: `%d`\n", report.UnknownSourceSurfaces)
	fmt.Fprintf(b, "- git_tracked_surfaces: `%d`\n", report.GitTrackedSurfaces)
	fmt.Fprintf(b, "- untracked_surfaces: `%d`\n", report.UntrackedSurfaces)
	fmt.Fprintf(b, "- working_tree_dirty_surfaces: `%d`\n", report.WorkingTreeDirtySurfaces)
	fmt.Fprintf(b, "- surfaces_with_commits: `%d`\n", report.SurfacesWithCommits)
	fmt.Fprintf(b, "- surfaces_without_commits: `%d`\n", report.SurfacesWithoutCommits)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(b, "- remote_fetch_allowed: `%t`\n", report.RemoteFetchAllowed)
	fmt.Fprintf(b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_source_bodies_included: `%t`\n", report.RawSourceBodiesIncluded)
	fmt.Fprintf(b, "- raw_source_refs_included: `%t`\n", report.RawSourceRefsIncluded)
	fmt.Fprintf(b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_skill_source_provenance_change: `%t`\n", report.LLME2ERequiredAfterSkillSourceProvenanceChange)
}

func writeSkillSourceProvenanceCard(b *strings.Builder, card SkillSourceProvenanceCard) {
	sourceRefSHA := "none"
	if card.SourceRefPresent {
		sourceRefSHA = card.SourceRefSHA
	}
	currentSHA := "none"
	if card.SkillSHA != "" {
		currentSHA = card.SkillSHA
	}
	expectedSHA := "none"
	if card.ExpectedSHA != "" {
		expectedSHA = card.ExpectedSHA
	}
	fmt.Fprintf(
		b,
		"- source_name=`%s` path=`%s` source=`%s` skill_path=`%s` skill_matched=`%t` source_kind=`%s` source_ref_present=`%t` source_ref_sha256_12=`%s` trust_level=`%s` install_mode=`%s` requires_approval=`%t` remote_fetch_allowed=`%t` hash_pinned=`%t` expected_sha256_12=`%s` current_skill_sha256_12=`%s` hash_matched=`%t` hash_mismatched=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		inlineCode(card.SourceName),
		card.Path,
		card.Source,
		card.SkillPath,
		card.SkillMatched,
		inlineCode(card.SourceKind),
		card.SourceRefPresent,
		sourceRefSHA,
		inlineCode(card.TrustLevel),
		inlineCode(card.InstallMode),
		card.RequiresApproval,
		card.RemoteFetchAllowed,
		card.HashPinned,
		expectedSHA,
		currentSHA,
		card.HashMatched,
		card.HashMismatched,
		card.Bytes,
		card.Lines,
		card.SHA,
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

func writeSkillSourceProvenanceFindings(b *strings.Builder, findings []SkillSourceProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func skillSourceProvenanceCard(cfg Config, source SkillSourceCard) SkillSourceProvenanceCard {
	card := SkillSourceProvenanceCard{
		SourceName:         source.Name,
		Path:               source.Path,
		SkillPath:          source.SkillPath,
		Source:             skillSourceTrustSource(source.Path),
		SkillMatched:       source.SkillMatched,
		SkillSHA:           source.SkillSHA,
		SourceKind:         source.SourceKind,
		SourceRefPresent:   source.SourceRefPresent,
		SourceRefSHA:       source.SourceRefSHA,
		TrustLevel:         source.TrustLevel,
		InstallMode:        source.InstallMode,
		ExpectedSHA:        source.ExpectedSHA,
		HashPinned:         source.HashPinned,
		HashMatched:        source.HashMatched,
		HashMismatched:     source.HashMismatched,
		RequiresApproval:   source.RequiresApproval,
		RemoteFetchAllowed: source.RemoteFetchAllowed,
		Bytes:              source.Bytes,
		Lines:              source.Lines,
		SHA:                source.SHA,
		RiskFindings:       len(source.RiskFindings),
		RiskMaxSeverity:    skillSourceRiskMaxSeverity(source.RiskFindings),
		RiskCodes:          skillSourceRiskCodes(source.RiskFindings),
		LastCommitSHA12:    "none",
		LastCommitShort:    "none",
		LastCommitDate:     "none",
		SubjectSHA12:       "none",
	}
	tracked, _ := soulGitTracked(cfg.Workdir, card.Path)
	card.GitTracked = tracked
	if tracked {
		card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, card.Path)
		info, ok := soulGitLastCommit(cfg.Workdir, card.Path)
		if ok {
			card.CommitAvailable = true
			card.LastCommitSHA12 = shortSHA(info.FullSHA)
			card.LastCommitShort = info.ShortSHA
			card.LastCommitDate = info.Date
			card.SubjectSHA12 = shortDocumentHash(info.Subject)
		}
	}
	return card
}

func (r *SkillSourceProvenanceReport) addCard(card SkillSourceProvenanceCard) {
	r.Cards = append(r.Cards, card)
	r.ProvenanceSurfaces++
	switch card.Source {
	case "repo-local":
		r.RepoLocalSurfaces++
	default:
		r.UnknownSourceSurfaces++
		r.addFinding("warning", "unknown_skill_source_pin_root", card.Path, "skill source pin is outside known repo-local source roots")
	}
	if card.GitTracked {
		r.GitTrackedSurfaces++
		if card.WorkingTreeDirty {
			r.WorkingTreeDirtySurfaces++
			r.addFinding("warning", "dirty_skill_source_pin", card.Path, "skill source pin has uncommitted working-tree changes")
		}
		if card.CommitAvailable {
			r.SurfacesWithCommits++
			r.GitHistoryAvailable = true
		} else {
			r.SurfacesWithoutCommits++
			r.addFinding("warning", "missing_git_history", card.Path, "no git commit was found for this skill source pin")
		}
		return
	}
	r.UntrackedSurfaces++
	r.addFinding("warning", "untracked_skill_source_pin", card.Path, "skill source pin is not tracked by git")
}

func skillSourceTrustSource(path string) string {
	if strings.HasPrefix(path, skillSourcesDir+"/") {
		return "repo-local"
	}
	return "unknown"
}

func skillSourceProvenanceBaseStatus(sources SkillSourceReport) string {
	if sources.Status == "high" {
		return "high"
	}
	if sources.Status == "warn" {
		return "warn"
	}
	if sources.Specs == 0 {
		return "not_configured"
	}
	if sources.Status == "" {
		return "unknown"
	}
	return "ok"
}

func skillSourceProvenanceGitGate(report SkillSourceProvenanceReport) string {
	if report.ProvenanceSurfaces == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedSurfaces > 0 || report.SurfacesWithoutCommits > 0 || report.WorkingTreeDirtySurfaces > 0 {
		return "warn"
	}
	return "pass"
}

func (r *SkillSourceProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SkillSourceProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
	sort.Slice(r.Findings, func(i, j int) bool {
		if skillSourceSeverityRank(r.Findings[i].Severity) != skillSourceSeverityRank(r.Findings[j].Severity) {
			return skillSourceSeverityRank(r.Findings[i].Severity) < skillSourceSeverityRank(r.Findings[j].Severity)
		}
		if r.Findings[i].Code != r.Findings[j].Code {
			return r.Findings[i].Code < r.Findings[j].Code
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}

func isSkillSourcesProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 &&
		fields[0] == "/skills" &&
		(strings.EqualFold(fields[1], "sources") || strings.EqualFold(fields[1], "source")) &&
		(strings.EqualFold(fields[2], "provenance") || strings.EqualFold(fields[2], "history") || strings.EqualFold(fields[2], "timeline"))
}
