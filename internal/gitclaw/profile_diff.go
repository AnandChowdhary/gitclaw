package gitclaw

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const defaultProfileDiffBaseRef = "HEAD~1"
const maxProfileDiffFilesReturned = 50

type ProfileDiffReport struct {
	Status                               string
	DiffScope                            string
	BaseRefHash                          string
	BaseCommitSHA12                      string
	HeadCommitSHA12                      string
	DiffSHA                              string
	CurrentManifestEntries               int
	CurrentProfileSurfaces               int
	ChangedProfileFiles                  int
	AddedProfileFiles                    int
	ModifiedProfileFiles                 int
	DeletedProfileFiles                  int
	RenamedProfileFiles                  int
	BinaryProfileFiles                   int
	ProfileFilesReturned                 int
	ProfileFileLimit                     int
	GitAvailable                         bool
	GitRepository                        bool
	BaseRefResolved                      bool
	RawDiffsIncluded                     bool
	RawProfileBodiesIncluded             bool
	RawSkillBodiesIncluded               bool
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	RawPromptBodiesIncluded              bool
	RawGitSubjectsIncluded               bool
	AuthorIdentitiesIncluded             bool
	RawRequestedRefsIncluded             bool
	ExternalProfileHomeAccessed          bool
	ProfileMutationAllowed               bool
	ProfileExportSupported               bool
	ProfileImportSupported               bool
	ProfileSwitchingSupported            bool
	LLME2ERequiredAfterProfileDiffChange bool
	ErrorReason                          string
	Cards                                []ProfileDiffCard
	Findings                             []ProfileDiffFinding
}

type ProfileDiffCard struct {
	Path              string
	OldPath           string
	Status            string
	Kind              string
	Category          string
	InCurrentManifest bool
	Selected          bool
	Enabled           bool
	Insertions        int
	Deletions         int
	Binary            bool
	PathSHA           string
	OldPathSHA        string
	CurrentFileSHA    string
	CurrentLineCount  int
}

type ProfileDiffFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildProfileDiffReport(cfg Config, repoContext RepoContext, baseRef string) ProfileDiffReport {
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		baseRef = defaultProfileDiffBaseRef
	}
	manifest := BuildProfileManifestReport(cfg, repoContext)
	report := ProfileDiffReport{
		Status:                               "ok",
		DiffScope:                            "repo-local-profile-files",
		BaseRefHash:                          shortDocumentHash(baseRef),
		CurrentManifestEntries:               manifest.ManifestEntries,
		CurrentProfileSurfaces:               len(profileSearchEntries(manifest.Entries)),
		ProfileFileLimit:                     maxProfileDiffFilesReturned,
		RawDiffsIncluded:                     false,
		RawProfileBodiesIncluded:             false,
		RawSkillBodiesIncluded:               false,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		RawPromptBodiesIncluded:              false,
		RawGitSubjectsIncluded:               false,
		AuthorIdentitiesIncluded:             false,
		RawRequestedRefsIncluded:             false,
		ExternalProfileHomeAccessed:          false,
		ProfileMutationAllowed:               false,
		ProfileExportSupported:               false,
		ProfileImportSupported:               false,
		ProfileSwitchingSupported:            false,
		LLME2ERequiredAfterProfileDiffChange: true,
	}
	if !soulGitAvailable() {
		report.Status = "unavailable"
		report.ErrorReason = "git_not_found"
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for profile diff checks")
		return report
	}
	report.GitAvailable = true
	if inside, err := runSoulGit(cfg.Workdir, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(inside) != "true" {
		report.Status = "unavailable"
		report.ErrorReason = "not_git_repository"
		report.addFinding("warning", "not_git_repository", "git", "workdir is not inside a git repository")
		return report
	}
	report.GitRepository = true
	if head, err := runSoulGit(cfg.Workdir, "rev-parse", "--short=12", "HEAD"); err == nil {
		report.HeadCommitSHA12 = strings.TrimSpace(head)
	}
	baseCommit, err := runSoulGit(cfg.Workdir, "rev-parse", "--verify", baseRef+"^{commit}")
	if err != nil || strings.TrimSpace(baseCommit) == "" {
		report.Status = "unavailable"
		report.ErrorReason = "base_ref_unresolved"
		report.addFinding("warning", "base_ref_unresolved", "git", "base ref could not be resolved to a commit")
		return report
	}
	report.BaseRefResolved = true
	report.BaseCommitSHA12 = shortSHA(strings.TrimSpace(baseCommit))
	currentEntries := profileDiffCurrentEntryMap(manifest.Entries)
	cards := profileDiffCards(cfg.Workdir, baseRef, currentEntries)
	report.ChangedProfileFiles = len(cards)
	for _, card := range cards {
		switch {
		case card.Binary:
			report.BinaryProfileFiles++
		}
		switch card.Status {
		case "added":
			report.AddedProfileFiles++
		case "deleted":
			report.DeletedProfileFiles++
		case "renamed":
			report.RenamedProfileFiles++
		default:
			report.ModifiedProfileFiles++
		}
	}
	if len(cards) > maxProfileDiffFilesReturned {
		report.addFinding("warning", "profile_diff_file_limit", "git", "profile diff had more files than the report limit")
		cards = cards[:maxProfileDiffFilesReturned]
	}
	report.Cards = cards
	report.ProfileFilesReturned = len(cards)
	if report.ChangedProfileFiles == 0 {
		report.Status = "no_changes"
	}
	report.DiffSHA = profileDiffSHA(report)
	return report
}

func RenderProfileDiffCLIReport(cfg Config, repoContext RepoContext, baseRef string) string {
	return renderProfileDiffReport(Event{}, cfg, repoContext, baseRef, false)
}

func renderProfileDiffReport(ev Event, cfg Config, repoContext RepoContext, baseRef string, includeIssue bool) string {
	report := BuildProfileDiffReport(cfg, repoContext, baseRef)
	var b strings.Builder
	b.WriteString("## GitClaw Profile Diff Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_diff_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- diff_scope: `%s`\n", report.DiffScope)
	fmt.Fprintf(&b, "- base_ref_sha256_12: `%s`\n", report.BaseRefHash)
	fmt.Fprintf(&b, "- base_ref_resolved: `%t`\n", report.BaseRefResolved)
	fmt.Fprintf(&b, "- base_commit_sha256_12: `%s`\n", noneIfEmpty(report.BaseCommitSHA12))
	fmt.Fprintf(&b, "- head_commit_sha256_12: `%s`\n", noneIfEmpty(report.HeadCommitSHA12))
	fmt.Fprintf(&b, "- profile_diff_sha256_12: `%s`\n", noneIfEmpty(report.DiffSHA))
	fmt.Fprintf(&b, "- current_manifest_entries: `%d`\n", report.CurrentManifestEntries)
	fmt.Fprintf(&b, "- current_profile_surfaces: `%d`\n", report.CurrentProfileSurfaces)
	fmt.Fprintf(&b, "- changed_profile_files: `%d`\n", report.ChangedProfileFiles)
	fmt.Fprintf(&b, "- added_profile_files: `%d`\n", report.AddedProfileFiles)
	fmt.Fprintf(&b, "- modified_profile_files: `%d`\n", report.ModifiedProfileFiles)
	fmt.Fprintf(&b, "- deleted_profile_files: `%d`\n", report.DeletedProfileFiles)
	fmt.Fprintf(&b, "- renamed_profile_files: `%d`\n", report.RenamedProfileFiles)
	fmt.Fprintf(&b, "- binary_profile_files: `%d`\n", report.BinaryProfileFiles)
	fmt.Fprintf(&b, "- profile_file_limit: `%d`\n", report.ProfileFileLimit)
	fmt.Fprintf(&b, "- profile_files_returned: `%d`\n", report.ProfileFilesReturned)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", report.GitRepository)
	fmt.Fprintf(&b, "- raw_diffs_included: `%t`\n", report.RawDiffsIncluded)
	fmt.Fprintf(&b, "- raw_profile_bodies_included: `%t`\n", report.RawProfileBodiesIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(&b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(&b, "- raw_requested_refs_included: `%t`\n", report.RawRequestedRefsIncluded)
	fmt.Fprintf(&b, "- external_profile_home_accessed: `%t`\n", report.ExternalProfileHomeAccessed)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", report.ProfileMutationAllowed)
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", report.ProfileExportSupported)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", report.ProfileImportSupported)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", report.ProfileSwitchingSupported)
	fmt.Fprintf(&b, "- llm_e2e_required_after_profile_diff_change: `%t`\n", report.LLME2ERequiredAfterProfileDiffChange)
	if report.ErrorReason != "" {
		fmt.Fprintf(&b, "- error_reason: `%s`\n", report.ErrorReason)
	}
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report compares repo-local `.gitclaw/` profile files against a git base ref using metadata-only diff commands. It reports path metadata, statuses, numstat counts, and hashes only; raw patches, profile files, skill bodies, issue/comment bodies, prompts, requested ref text, git subjects, author identities, sessions, backup payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Profile Diff Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeProfileDiffCard(&b, card)
		}
	}

	b.WriteString("\n### Diff Gates\n")
	fmt.Fprintf(&b, "- base_ref_gate=`%s`\n", profileDiffBaseRefGate(report))
	fmt.Fprintf(&b, "- raw_diff_gate=`%s`\n", "numstat-and-status-only")
	b.WriteString("- raw_body_gate=`hashes-only`\n")
	b.WriteString("- requested_ref_gate=`sha256_12_only`\n")
	b.WriteString("- git_subject_gate=`excluded`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- external_profile_home_gate=`not_accessed`\n")
	b.WriteString("- session_payload_gate=`excluded`\n")
	b.WriteString("- backup_payload_gate=`excluded`\n")
	b.WriteString("- llm_e2e_gate=`required`\n")

	b.WriteString("\n### Findings\n")
	writeProfileDiffFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func profileDiffCards(root, baseRef string, current map[string]ProfileManifestEntry) []ProfileDiffCard {
	statuses := parseProfileDiffNameStatus(mustRunSoulGit(root, "diff", "--name-status", "--find-renames", baseRef+"..HEAD", "--", ".gitclaw"))
	numstats := parseProfileDiffNumstat(mustRunSoulGit(root, "diff", "--numstat", "--find-renames", baseRef+"..HEAD", "--", ".gitclaw"))
	for path, card := range statuses {
		if stat, ok := numstats[path]; ok {
			card.Insertions = stat.Insertions
			card.Deletions = stat.Deletions
			card.Binary = stat.Binary
		}
		entry, ok := current[path]
		if !ok && card.OldPath != "" {
			entry, ok = current[card.OldPath]
		}
		if ok {
			card.Kind = entry.Kind
			card.Category = entry.Category
			card.InCurrentManifest = current[path].Path != "" || current[card.OldPath].Path != ""
			card.Selected = entry.Selected
			card.Enabled = entry.Enabled
			card.CurrentFileSHA = entry.SHA
			card.CurrentLineCount = entry.Lines
		} else {
			card.Kind = "profile-surface"
			card.Category = profileDiffCategory(path)
		}
		statuses[path] = card
	}
	paths := make([]string, 0, len(statuses))
	for path := range statuses {
		if !profileDiffPathEligible(path) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	cards := make([]ProfileDiffCard, 0, len(paths))
	for _, path := range paths {
		cards = append(cards, statuses[path])
	}
	return cards
}

type profileDiffNumstat struct {
	Insertions int
	Deletions  int
	Binary     bool
}

func parseProfileDiffNameStatus(out string) map[string]ProfileDiffCard {
	cards := map[string]ProfileDiffCard{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		statusCode := parts[0]
		path := parts[len(parts)-1]
		if !profileDiffPathEligible(path) {
			continue
		}
		card := ProfileDiffCard{
			Path:    path,
			Status:  profileDiffStatus(statusCode),
			PathSHA: shortDocumentHash(path),
		}
		if strings.HasPrefix(statusCode, "R") && len(parts) >= 3 {
			card.OldPath = parts[1]
			card.OldPathSHA = shortDocumentHash(card.OldPath)
		}
		cards[path] = card
	}
	return cards
}

func parseProfileDiffNumstat(out string) map[string]profileDiffNumstat {
	stats := map[string]profileDiffNumstat{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		path := parts[len(parts)-1]
		if !profileDiffPathEligible(path) {
			continue
		}
		stat := profileDiffNumstat{}
		if parts[0] == "-" || parts[1] == "-" {
			stat.Binary = true
		} else {
			stat.Insertions, _ = strconv.Atoi(parts[0])
			stat.Deletions, _ = strconv.Atoi(parts[1])
		}
		stats[path] = stat
	}
	return stats
}

func profileDiffCurrentEntryMap(entries []ProfileManifestEntry) map[string]ProfileManifestEntry {
	out := map[string]ProfileManifestEntry{}
	for _, entry := range profileSearchEntries(entries) {
		out[entry.Path] = entry
	}
	return out
}

func profileDiffStatus(code string) string {
	switch {
	case strings.HasPrefix(code, "A"):
		return "added"
	case strings.HasPrefix(code, "D"):
		return "deleted"
	case strings.HasPrefix(code, "R"):
		return "renamed"
	case strings.HasPrefix(code, "C"):
		return "copied"
	default:
		return "modified"
	}
}

func profileDiffCategory(path string) string {
	if path == "" {
		return "unknown"
	}
	entry := ProfileManifestEntry{Path: path, Present: true}
	if profileSearchEntryEligible(entry) {
		return profileDocumentCategory(path)
	}
	return "excluded"
}

func profileDiffPathEligible(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, ".gitclaw/") {
		return false
	}
	if strings.HasPrefix(path, ".gitclaw/backups/") {
		return false
	}
	return true
}

func writeProfileDiffCard(b *strings.Builder, card ProfileDiffCard) {
	fmt.Fprintf(
		b,
		"- path=`%s` status=`%s` kind=`%s` category=`%s` in_current_manifest=`%t` selected=`%t` enabled=`%t` insertions=`%d` deletions=`%d` binary=`%t` path_sha256_12=`%s` old_path_sha256_12=`%s` current_file_sha256_12=`%s` current_lines=`%d`\n",
		card.Path,
		card.Status,
		inlineCode(card.Kind),
		inlineCode(card.Category),
		card.InCurrentManifest,
		card.Selected,
		card.Enabled,
		card.Insertions,
		card.Deletions,
		card.Binary,
		card.PathSHA,
		noneIfEmpty(card.OldPathSHA),
		noneIfEmpty(card.CurrentFileSHA),
		card.CurrentLineCount,
	)
}

func writeProfileDiffFindings(b *strings.Builder, findings []ProfileDiffFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func profileDiffBaseRefGate(report ProfileDiffReport) string {
	if report.BaseRefResolved {
		return "pass"
	}
	return "warn"
}

func profileDiffSHA(report ProfileDiffReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "gitclaw-profile-diff-v1|base=%s|head=%s|status=%s\n", report.BaseCommitSHA12, report.HeadCommitSHA12, report.Status)
	for _, card := range report.Cards {
		fmt.Fprintf(&b, "%s|%s|%s|%s|%t|%t|%d|%d|%t|%s|%s\n", card.Path, card.Status, card.Kind, card.Category, card.Selected, card.Enabled, card.Insertions, card.Deletions, card.Binary, card.PathSHA, card.CurrentFileSHA)
	}
	return shortDocumentHash(b.String())
}

func (r *ProfileDiffReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, ProfileDiffFinding{Severity: severity, Code: code, Path: path, Detail: detail})
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

func isProfileDiffRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || (fields[0] != "/profile" && fields[0] != "/profiles") {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "diff", "changes", "compare":
		return true
	default:
		return false
	}
}

func requestedProfileDiffBaseRef(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || (fields[0] != "/profile" && fields[0] != "/profiles") {
		return defaultProfileDiffBaseRef
	}
	switch strings.ToLower(fields[1]) {
	case "diff", "changes", "compare":
		return strings.TrimSpace(fields[2])
	default:
		return defaultProfileDiffBaseRef
	}
}

func mustRunSoulGit(root string, args ...string) string {
	out, err := runSoulGit(root, args...)
	if err != nil {
		return ""
	}
	return out
}
