package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type MemoryProvenanceReport struct {
	Status                                    string
	Timeline                                  MemoryTimelineReport
	MemoryFiles                               int
	LongTermMemoryPresent                     bool
	LongTermMemoryLoaded                      bool
	DatedMemoryNotes                          int
	CanonicalDatedMemoryNotes                 int
	NoncanonicalDatedMemoryNotes              int
	LoadedMemoryNotes                         int
	OmittedMemoryNotes                        int
	FirstMemoryNote                           string
	LatestMemoryNote                          string
	TimelineSpanDays                          int
	RepoLocalMemoryFiles                      int
	UnknownMemoryFiles                        int
	GitTrackedMemoryFiles                     int
	UntrackedMemoryFiles                      int
	WorkingTreeDirtyMemoryFiles               int
	MemoryFilesWithCommits                    int
	MemoryFilesWithoutCommits                 int
	GitAvailable                              bool
	GitHistoryAvailable                       bool
	ExternalProviderAccessed                  bool
	SessionSearchIndexSource                  string
	BackgroundPromotionActive                 bool
	MemoryWritesAllowed                       bool
	RawMemoryBodiesIncluded                   bool
	RawIssueBodiesIncluded                    bool
	RawCommentBodiesIncluded                  bool
	RawPromptBodiesIncluded                   bool
	RawGitSubjectsIncluded                    bool
	AuthorIdentitiesIncluded                  bool
	LLME2ERequiredAfterMemoryProvenanceChange bool
	Cards                                     []MemoryProvenanceCard
	Findings                                  []MemoryProvenanceFinding
}

type MemoryProvenanceCard struct {
	Position           int
	Kind               string
	Path               string
	Source             string
	Role               string
	Date               string
	Present            bool
	Canonical          bool
	Latest             bool
	LoadedForThisTurn  bool
	PromptVisible      bool
	Bytes              int
	Lines              int
	SHA                string
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	ValidationFindings int
	GitTracked         bool
	WorkingTreeDirty   bool
	LastCommitSHA12    string
	LastCommitShort    string
	LastCommitDate     string
	SubjectSHA12       string
	CommitAvailable    bool
}

type MemoryProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildMemoryProvenanceReport(cfg Config, repoContext RepoContext) MemoryProvenanceReport {
	timeline := BuildMemoryTimelineReport(cfg, repoContext)
	validationByPath := memoryValidationFindingCountByPath(timeline.Validation)
	report := MemoryProvenanceReport{
		Status:                                    memoryProvenanceBaseStatus(timeline.Validation.Status, timeline.Risk.Status),
		Timeline:                                  timeline,
		MemoryFiles:                               timeline.MemoryFiles,
		LongTermMemoryPresent:                     timeline.LongTermMemoryPresent,
		LongTermMemoryLoaded:                      timeline.LongTermMemoryLoaded,
		DatedMemoryNotes:                          timeline.DatedMemoryNotes,
		CanonicalDatedMemoryNotes:                 timeline.CanonicalDatedMemoryNotes,
		NoncanonicalDatedMemoryNotes:              timeline.NoncanonicalDatedMemoryNotes,
		LoadedMemoryNotes:                         timeline.LoadedMemoryNotes,
		OmittedMemoryNotes:                        timeline.OmittedMemoryNotes,
		FirstMemoryNote:                           timeline.FirstMemoryNote,
		LatestMemoryNote:                          timeline.LatestMemoryNote,
		TimelineSpanDays:                          timeline.TimelineSpanDays,
		GitAvailable:                              soulGitAvailable(),
		ExternalProviderAccessed:                  false,
		SessionSearchIndexSource:                  "github-issues-and-backups",
		BackgroundPromotionActive:                 false,
		MemoryWritesAllowed:                       false,
		RawMemoryBodiesIncluded:                   false,
		RawIssueBodiesIncluded:                    false,
		RawCommentBodiesIncluded:                  false,
		RawPromptBodiesIncluded:                   false,
		RawGitSubjectsIncluded:                    false,
		AuthorIdentitiesIncluded:                  false,
		LLME2ERequiredAfterMemoryProvenanceChange: true,
	}
	for _, entry := range timeline.TimelineEntries {
		switch entry.Source {
		case "repo-local":
			report.RepoLocalMemoryFiles++
		default:
			report.UnknownMemoryFiles++
			report.addFinding("warning", "unknown_memory_source", entry.Path, "memory file is outside known repo-local memory roots")
		}
		card := MemoryProvenanceCard{
			Position:           entry.Position,
			Kind:               entry.Kind,
			Path:               entry.Path,
			Source:             entry.Source,
			Role:               memoryCatalogRole(entry),
			Date:               entry.Date,
			Present:            entry.Present,
			Canonical:          entry.Canonical,
			Latest:             entry.Latest,
			LoadedForThisTurn:  entry.LoadedForThisTurn,
			PromptVisible:      entry.PromptVisible,
			Bytes:              entry.Bytes,
			Lines:              entry.Lines,
			SHA:                entry.SHA,
			RiskFindings:       entry.RiskFindings,
			RiskMaxSeverity:    entry.RiskMaxSeverity,
			RiskCodes:          append([]string(nil), entry.RiskCodes...),
			ValidationFindings: validationByPath[entry.Path],
			LastCommitSHA12:    "none",
			LastCommitShort:    "none",
			LastCommitDate:     "none",
			SubjectSHA12:       "none",
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, entry.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedMemoryFiles++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, entry.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtyMemoryFiles++
				report.addFinding("warning", "dirty_memory_file", entry.Path, "memory file has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, entry.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.MemoryFilesWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.MemoryFilesWithoutCommits++
				report.addFinding("warning", "missing_git_history", entry.Path, "no git commit was found for this memory file")
			}
		} else {
			report.UntrackedMemoryFiles++
			detail := "memory file is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_memory_file", entry.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for memory provenance checks")
	}
	if report.MemoryFiles > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local memory files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderMemoryProvenanceCLIReport(cfg Config, repoContext RepoContext) string {
	return renderMemoryProvenanceReport(Event{}, cfg, repoContext, false)
}

func RenderMemoryProvenanceReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderMemoryProvenanceReport(ev, cfg, repoContext, ev.Repo != "" || ev.Issue.Number != 0)
}

func renderMemoryProvenanceReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildMemoryProvenanceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", "repo-local-memory-git-history")
	fmt.Fprintf(&b, "- memory_files: `%d`\n", report.MemoryFiles)
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", report.LongTermMemoryPresent)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", report.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", report.DatedMemoryNotes)
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", report.CanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", report.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", report.LoadedMemoryNotes)
	fmt.Fprintf(&b, "- omitted_memory_notes: `%d`\n", report.OmittedMemoryNotes)
	fmt.Fprintf(&b, "- first_memory_note: `%s`\n", report.FirstMemoryNote)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", report.LatestMemoryNote)
	fmt.Fprintf(&b, "- timeline_span_days: `%d`\n", report.TimelineSpanDays)
	fmt.Fprintf(&b, "- repo_local_memory_files: `%d`\n", report.RepoLocalMemoryFiles)
	fmt.Fprintf(&b, "- unknown_memory_files: `%d`\n", report.UnknownMemoryFiles)
	fmt.Fprintf(&b, "- git_tracked_memory_files: `%d`\n", report.GitTrackedMemoryFiles)
	fmt.Fprintf(&b, "- untracked_memory_files: `%d`\n", report.UntrackedMemoryFiles)
	fmt.Fprintf(&b, "- working_tree_dirty_memory_files: `%d`\n", report.WorkingTreeDirtyMemoryFiles)
	fmt.Fprintf(&b, "- memory_files_with_commits: `%d`\n", report.MemoryFilesWithCommits)
	fmt.Fprintf(&b, "- memory_files_without_commits: `%d`\n", report.MemoryFilesWithoutCommits)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(&b, "- external_provider_accessed: `%t`\n", report.ExternalProviderAccessed)
	fmt.Fprintf(&b, "- session_search_index_source: `%s`\n", report.SessionSearchIndexSource)
	fmt.Fprintf(&b, "- background_promotion_active: `%t`\n", report.BackgroundPromotionActive)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", report.MemoryWritesAllowed)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", report.RawMemoryBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(&b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_memory_provenance_change: `%t`\n", report.LLME2ERequiredAfterMemoryProvenanceChange)
	writeMemoryValidationSummary(&b, report.Timeline.Validation)
	fmt.Fprintf(&b, "- memory_risk_status: `%s`\n", report.Timeline.Risk.Status)
	fmt.Fprintf(&b, "- memory_risk_findings: `%d`\n", len(report.Timeline.Risk.Findings))
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local memory files to body-free git provenance. It reports memory paths, load state, validation/risk metadata, hashes, tracked state, last commit IDs/dates, and commit-subject hashes only; raw memory bodies, issue bodies, comments, prompts, git subjects, author identities, provider payloads, and secret values are not included.\n\n")

	b.WriteString("### Memory Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeMemoryProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Timeline.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Timeline.Risk.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", memoryProvenanceGitGate(report))
	fmt.Fprintf(&b, "- memory_write_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- external_provider_gate=`%s`\n", "not_configured")
	fmt.Fprintf(&b, "- session_search_gate=`%s`\n", report.SessionSearchIndexSource)
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")

	b.WriteString("\n### Findings\n")
	writeMemoryProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeMemoryProvenanceCard(b *strings.Builder, card MemoryProvenanceCard) {
	fmt.Fprintf(
		b,
		"- position=`%d` kind=`%s` path=`%s` source=`%s` role=`%s` date=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` prompt_visible=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` validation_findings=`%d` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		card.Position,
		card.Kind,
		card.Path,
		card.Source,
		card.Role,
		card.Date,
		card.Present,
		card.Canonical,
		card.Latest,
		card.LoadedForThisTurn,
		card.PromptVisible,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.RiskFindings,
		card.RiskMaxSeverity,
		inlineListOrNone(card.RiskCodes),
		card.ValidationFindings,
		card.GitTracked,
		card.WorkingTreeDirty,
		card.CommitAvailable,
		card.LastCommitSHA12,
		card.LastCommitShort,
		card.LastCommitDate,
		card.SubjectSHA12,
	)
}

func writeMemoryProvenanceFindings(b *strings.Builder, findings []MemoryProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func isMemoryProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/memory" || fields[0] == "/memories") && (strings.EqualFold(fields[1], "provenance") || strings.EqualFold(fields[1], "git-history"))
}

func memoryProvenanceBaseStatus(validationStatus, riskStatus string) string {
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

func memoryProvenanceGitGate(report MemoryProvenanceReport) string {
	if report.MemoryFiles == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedMemoryFiles > 0 || report.MemoryFilesWithoutCommits > 0 || report.WorkingTreeDirtyMemoryFiles > 0 {
		return "warn"
	}
	return "pass"
}

func (r *MemoryProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, MemoryProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
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
