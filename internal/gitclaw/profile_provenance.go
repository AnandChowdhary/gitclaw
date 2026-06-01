package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ProfileProvenanceReport struct {
	Status                                     string
	ProvenanceScope                            string
	ProvenanceSHA                              string
	Manifest                                   ProfileManifestReport
	Snapshot                                   ProfileSnapshotReport
	ProfileDocumentsLoaded                     int
	ManifestEntries                            int
	ProfileSurfaces                            int
	RepoLocalSurfaces                          int
	PortableSurfaces                           int
	SelectedSurfaces                           int
	EnabledSurfaces                            int
	GitTrackedSurfaces                         int
	UntrackedSurfaces                          int
	WorkingTreeDirtySurfaces                   int
	SurfacesWithCommits                        int
	SurfacesWithoutCommits                     int
	AvailableSkills                            int
	SkillBundles                               int
	AvailableTools                             int
	GitAvailable                               bool
	GitHistoryAvailable                        bool
	ExternalProfileHomeAccessed                bool
	ProfileExportSupported                     bool
	ProfileImportSupported                     bool
	ProfileSwitchingSupported                  bool
	ProfileDistributionInstallSupported        bool
	ProfileMutationAllowed                     bool
	CredentialsIncluded                        bool
	SessionsIncluded                           bool
	BackupPayloadsIncluded                     bool
	RawProfileBodiesIncluded                   bool
	RawSkillBodiesIncluded                     bool
	RawToolOutputsIncluded                     bool
	RawIssueBodiesIncluded                     bool
	RawCommentBodiesIncluded                   bool
	RawPromptBodiesIncluded                    bool
	RawGitSubjectsIncluded                     bool
	AuthorIdentitiesIncluded                   bool
	LLME2ERequiredAfterProfileProvenanceChange bool
	Cards                                      []ProfileProvenanceCard
	Findings                                   []ProfileProvenanceFinding
}

type ProfileProvenanceCard struct {
	Position         int
	Kind             string
	Name             string
	Path             string
	Category         string
	Source           string
	IncludePolicy    string
	Portable         bool
	Required         bool
	Present          bool
	Selected         bool
	Enabled          bool
	Bytes            int
	Lines            int
	SHA              string
	GitTracked       bool
	WorkingTreeDirty bool
	CommitAvailable  bool
	LastCommitSHA12  string
	LastCommitShort  string
	LastCommitDate   string
	SubjectSHA12     string
}

type ProfileProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildProfileProvenanceReport(cfg Config, repoContext RepoContext) ProfileProvenanceReport {
	manifest := BuildProfileManifestReport(cfg, repoContext)
	snapshot := BuildProfileSnapshotReport(cfg, repoContext)
	report := ProfileProvenanceReport{
		Status:                                     manifest.Status,
		ProvenanceScope:                            "repo-local-profile-git-history",
		Manifest:                                   manifest,
		Snapshot:                                   snapshot,
		ProfileDocumentsLoaded:                     manifest.ProfileDocumentsLoaded,
		ManifestEntries:                            manifest.ManifestEntries,
		AvailableSkills:                            manifest.AvailableSkills,
		SkillBundles:                               manifest.SkillBundles,
		AvailableTools:                             manifest.AvailableTools,
		GitAvailable:                               soulGitAvailable(),
		ExternalProfileHomeAccessed:                false,
		ProfileExportSupported:                     false,
		ProfileImportSupported:                     false,
		ProfileSwitchingSupported:                  false,
		ProfileDistributionInstallSupported:        false,
		ProfileMutationAllowed:                     false,
		CredentialsIncluded:                        false,
		SessionsIncluded:                           false,
		BackupPayloadsIncluded:                     false,
		RawProfileBodiesIncluded:                   false,
		RawSkillBodiesIncluded:                     false,
		RawToolOutputsIncluded:                     false,
		RawIssueBodiesIncluded:                     false,
		RawCommentBodiesIncluded:                   false,
		RawPromptBodiesIncluded:                    false,
		RawGitSubjectsIncluded:                     false,
		AuthorIdentitiesIncluded:                   false,
		LLME2ERequiredAfterProfileProvenanceChange: true,
	}
	for _, entry := range manifest.Entries {
		if !profileProvenanceEntryEligible(entry) {
			continue
		}
		card := ProfileProvenanceCard{
			Position:        len(report.Cards) + 1,
			Kind:            entry.Kind,
			Name:            entry.Name,
			Path:            entry.Path,
			Category:        entry.Category,
			Source:          entry.Source,
			IncludePolicy:   entry.IncludePolicy,
			Portable:        entry.Portable,
			Required:        entry.Required,
			Present:         entry.Present,
			Selected:        entry.Selected,
			Enabled:         entry.Enabled,
			Bytes:           entry.Bytes,
			Lines:           entry.Lines,
			SHA:             entry.SHA,
			LastCommitSHA12: "none",
			LastCommitShort: "none",
			LastCommitDate:  "none",
			SubjectSHA12:    "none",
		}
		report.ProfileSurfaces++
		if entry.Source == "repo-local" {
			report.RepoLocalSurfaces++
		}
		if entry.Portable {
			report.PortableSurfaces++
		}
		if entry.Selected {
			report.SelectedSurfaces++
		}
		if entry.Enabled {
			report.EnabledSurfaces++
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, entry.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedSurfaces++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, entry.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtySurfaces++
				report.addFinding("warning", "dirty_profile_surface", entry.Path, "profile surface has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, entry.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.SurfacesWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.SurfacesWithoutCommits++
				report.addFinding("warning", "missing_git_history", entry.Path, "no git commit was found for this profile surface")
			}
		} else {
			report.UntrackedSurfaces++
			detail := "profile surface is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_profile_surface", entry.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for profile provenance checks")
	}
	if report.ProfileSurfaces > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local profile surfaces")
	}
	report.ProvenanceSHA = profileProvenanceSHA(report.Cards, manifest.ManifestSHA, snapshot.SnapshotSHA)
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderProfileProvenanceCLIReport(cfg Config, repoContext RepoContext) string {
	return renderProfileProvenanceReport(Event{}, cfg, repoContext, false)
}

func renderProfileProvenanceReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildProfileProvenanceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Profile Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", report.ProvenanceScope)
	fmt.Fprintf(&b, "- provenance_sha256_12: `%s`\n", report.ProvenanceSHA)
	fmt.Fprintf(&b, "- profile_strategy: `%s`\n", report.Manifest.ProfileStrategy)
	fmt.Fprintf(&b, "- profile_store: `%s`\n", report.Manifest.ProfileStore)
	fmt.Fprintf(&b, "- profile_scope: `%s`\n", report.Manifest.ProfileScope)
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", report.ProfileDocumentsLoaded)
	fmt.Fprintf(&b, "- manifest_entries: `%d`\n", report.ManifestEntries)
	fmt.Fprintf(&b, "- profile_surfaces: `%d`\n", report.ProfileSurfaces)
	fmt.Fprintf(&b, "- repo_local_surfaces: `%d`\n", report.RepoLocalSurfaces)
	fmt.Fprintf(&b, "- portable_surfaces: `%d`\n", report.PortableSurfaces)
	fmt.Fprintf(&b, "- selected_surfaces: `%d`\n", report.SelectedSurfaces)
	fmt.Fprintf(&b, "- enabled_surfaces: `%d`\n", report.EnabledSurfaces)
	fmt.Fprintf(&b, "- git_tracked_surfaces: `%d`\n", report.GitTrackedSurfaces)
	fmt.Fprintf(&b, "- untracked_surfaces: `%d`\n", report.UntrackedSurfaces)
	fmt.Fprintf(&b, "- working_tree_dirty_surfaces: `%d`\n", report.WorkingTreeDirtySurfaces)
	fmt.Fprintf(&b, "- surfaces_with_commits: `%d`\n", report.SurfacesWithCommits)
	fmt.Fprintf(&b, "- surfaces_without_commits: `%d`\n", report.SurfacesWithoutCommits)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", report.SkillBundles)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- manifest_sha256_12: `%s`\n", report.Manifest.ManifestSHA)
	fmt.Fprintf(&b, "- profile_snapshot_sha256_12: `%s`\n", report.Snapshot.SnapshotSHA)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(&b, "- external_profile_home_accessed: `%t`\n", report.ExternalProfileHomeAccessed)
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", report.ProfileExportSupported)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", report.ProfileImportSupported)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", report.ProfileSwitchingSupported)
	fmt.Fprintf(&b, "- profile_distribution_install_supported: `%t`\n", report.ProfileDistributionInstallSupported)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", report.ProfileMutationAllowed)
	fmt.Fprintf(&b, "- credentials_included: `%t`\n", report.CredentialsIncluded)
	fmt.Fprintf(&b, "- sessions_included: `%t`\n", report.SessionsIncluded)
	fmt.Fprintf(&b, "- backup_payloads_included: `%t`\n", report.BackupPayloadsIncluded)
	fmt.Fprintf(&b, "- raw_profile_bodies_included: `%t`\n", report.RawProfileBodiesIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(&b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_profile_provenance_change: `%t`\n", report.LLME2ERequiredAfterProfileProvenanceChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps GitClaw's repo-local profile envelope to body-free git provenance. It reports profile paths, categories, hashes, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; raw profile files, skill bodies, memory bodies, tool outputs, issue/comment bodies, prompts, git subjects, author identities, sessions, backup payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Profile Provenance Cards\n")
	writeProfileProvenanceCards(&b, report.Cards)

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- manifest_gate=`%s`\n", profileProvenanceGate(report.Manifest.Status))
	fmt.Fprintf(&b, "- snapshot_gate=`%s`\n", profileProvenanceGate(report.Snapshot.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", profileProvenanceGitGate(report))
	b.WriteString("- profile_export_gate=`disabled`\n")
	b.WriteString("- profile_import_gate=`disabled`\n")
	b.WriteString("- profile_switching_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- external_profile_home_gate=`not_accessed`\n")
	b.WriteString("- session_payload_gate=`excluded`\n")
	b.WriteString("- backup_payload_gate=`excluded`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- git_subject_gate=`sha256_12_only`\n")

	b.WriteString("\n### Findings\n")
	writeProfileProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeProfileProvenanceCards(b *strings.Builder, cards []ProfileProvenanceCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- position=`%d` kind=`%s` name=`%s` path=`%s` category=`%s` source=`%s` include_policy=`%s` portable=`%t` required=`%t` present=`%t` selected=`%t` enabled=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
			card.Position,
			inlineCode(card.Kind),
			inlineCode(card.Name),
			card.Path,
			inlineCode(card.Category),
			inlineCode(card.Source),
			inlineCode(card.IncludePolicy),
			card.Portable,
			card.Required,
			card.Present,
			card.Selected,
			card.Enabled,
			card.Bytes,
			card.Lines,
			noneIfEmpty(card.SHA),
			card.GitTracked,
			card.WorkingTreeDirty,
			card.CommitAvailable,
			card.LastCommitSHA12,
			card.LastCommitShort,
			card.LastCommitDate,
			card.SubjectSHA12,
		)
	}
}

func writeProfileProvenanceFindings(b *strings.Builder, findings []ProfileProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func profileProvenanceEntryEligible(entry ProfileManifestEntry) bool {
	if !entry.Present {
		return false
	}
	if strings.HasPrefix(entry.Path, "tool:") {
		return false
	}
	if entry.Path == "" {
		return false
	}
	return strings.HasPrefix(entry.Path, ".gitclaw/")
}

func profileProvenanceSHA(cards []ProfileProvenanceCard, manifestSHA, snapshotSHA string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "gitclaw-profile-provenance-v1|manifest=%s|snapshot=%s\n", manifestSHA, snapshotSHA)
	for _, card := range cards {
		fmt.Fprintf(&b, "%03d|%s|%s|%s|%s|%s|%s|%t|%t|%t|%t|%t|%d|%d|%s|%t|%t|%t|%s|%s|%s|%s\n",
			card.Position,
			card.Kind,
			card.Name,
			card.Path,
			card.Category,
			card.Source,
			card.IncludePolicy,
			card.Portable,
			card.Required,
			card.Present,
			card.Selected,
			card.Enabled,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.GitTracked,
			card.WorkingTreeDirty,
			card.CommitAvailable,
			card.LastCommitSHA12,
			card.LastCommitShort,
			card.LastCommitDate,
			card.SubjectSHA12,
		)
	}
	return shortDocumentHash(b.String())
}

func profileProvenanceGate(status string) string {
	switch status {
	case "ok":
		return "pass"
	case "warn":
		return "warn"
	case "high", "error":
		return "fail"
	default:
		return status
	}
}

func profileProvenanceGitGate(report ProfileProvenanceReport) string {
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedSurfaces > 0 || report.SurfacesWithoutCommits > 0 || report.WorkingTreeDirtySurfaces > 0 {
		return "warn"
	}
	return "pass"
}

func (r *ProfileProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, ProfileProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
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

func isProfileProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || (fields[0] != "/profile" && fields[0] != "/profiles") {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "provenance", "history", "git", "git-history":
		return true
	default:
		return false
	}
}
