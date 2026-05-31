package gitclaw

import (
	"fmt"
	"strings"
	"time"
)

type MemoryTimelineReport struct {
	Status                                  string
	Validation                              MemoryValidationReport
	Risk                                    MemoryRiskReport
	MemoryFiles                             int
	LongTermMemoryPresent                   bool
	LongTermMemoryLoaded                    bool
	DatedMemoryNotes                        int
	CanonicalDatedMemoryNotes               int
	NoncanonicalDatedMemoryNotes            int
	LoadedMemoryNotes                       int
	OmittedMemoryNotes                      int
	MaxLoadedMemoryNotes                    int
	LatestMemoryNote                        string
	FirstMemoryNote                         string
	TimelineEntries                         []MemoryTimelineEntry
	TimelineSpanDays                        int
	LargestGapDays                          int
	GapsOverOneDay                          int
	RawBodiesIncluded                       bool
	MemoryWritesAllowed                     bool
	LLME2ERequiredAfterMemoryTimelineChange bool
}

type MemoryTimelineEntry struct {
	Position                 int
	Kind                     string
	Path                     string
	Source                   string
	Date                     string
	Present                  bool
	Canonical                bool
	Latest                   bool
	LoadedForThisTurn        bool
	PromptVisible            bool
	Bytes                    int
	Lines                    int
	SHA                      string
	AtContextLimit           bool
	GapDaysSincePreviousNote string
	RiskFindings             int
	RiskMaxSeverity          string
	RiskCodes                []string
}

func BuildMemoryTimelineReport(cfg Config, repoContext RepoContext) MemoryTimelineReport {
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	risk := BuildMemoryRiskReport(cfg.Workdir, repoContext)
	files := memorySearchFiles(surface)
	report := MemoryTimelineReport{
		Status:                                  memoryTimelineStatus(validation.Status, risk.Status),
		Validation:                              validation,
		Risk:                                    risk,
		MemoryFiles:                             len(files),
		LongTermMemoryPresent:                   surface.LongTerm.Present,
		LongTermMemoryLoaded:                    surface.LoadedLongTerm,
		DatedMemoryNotes:                        len(surface.DatedNotes),
		CanonicalDatedMemoryNotes:               countCanonicalMemoryNotes(surface.DatedNotes),
		NoncanonicalDatedMemoryNotes:            countNoncanonicalMemoryNotes(surface.DatedNotes),
		LoadedMemoryNotes:                       len(surface.LoadedNotePaths),
		OmittedMemoryNotes:                      omittedMemoryNotes(surface),
		MaxLoadedMemoryNotes:                    maxMemoryDocuments,
		LatestMemoryNote:                        latestMemoryNotePath(surface.DatedNotes),
		RawBodiesIncluded:                       false,
		MemoryWritesAllowed:                     false,
		LLME2ERequiredAfterMemoryTimelineChange: true,
	}
	riskByPath := map[string]MemoryRiskFile{}
	for _, file := range risk.Files {
		riskByPath[file.Path] = file
	}
	loaded := loadedMemoryPathSet(surface)
	var firstDate, latestDate *time.Time
	var previousDate *time.Time
	for _, file := range files {
		dateLabel, parsedDate := memoryTimelineDate(file.Path)
		gap := "n/a"
		if parsedDate != nil {
			if firstDate == nil {
				firstDate = parsedDate
			}
			latestDate = parsedDate
			if previousDate == nil {
				gap = "first"
			} else {
				days := int(parsedDate.Sub(*previousDate).Hours() / 24)
				if days < 0 {
					days = 0
				}
				gap = fmt.Sprintf("%d", days)
				if days > report.LargestGapDays {
					report.LargestGapDays = days
				}
				if days > 1 {
					report.GapsOverOneDay++
				}
			}
			previousDate = parsedDate
		}
		riskFile := riskByPath[file.Path]
		report.TimelineEntries = append(report.TimelineEntries, MemoryTimelineEntry{
			Position:                 len(report.TimelineEntries) + 1,
			Kind:                     memoryFileKind(file.Path),
			Path:                     file.Path,
			Source:                   memoryTrustSource(file.Path),
			Date:                     dateLabel,
			Present:                  file.Present,
			Canonical:                memoryFileCanonical(file.Path),
			Latest:                   file.Path == report.LatestMemoryNote,
			LoadedForThisTurn:        loaded[file.Path],
			PromptVisible:            loaded[file.Path],
			Bytes:                    file.Bytes,
			Lines:                    file.Lines,
			SHA:                      file.SHA,
			AtContextLimit:           file.Bytes >= maxContextDocumentBytes,
			GapDaysSincePreviousNote: gap,
			RiskFindings:             len(riskFile.Findings),
			RiskMaxSeverity:          memoryRiskMaxSeverity(riskFile.Findings),
			RiskCodes:                memoryRiskCodes(riskFile.Findings),
		})
	}
	if firstDate != nil {
		report.FirstMemoryNote = ".gitclaw/memory/" + firstDate.Format("2006-01-02") + ".md"
	}
	if report.FirstMemoryNote == "" {
		report.FirstMemoryNote = "none"
	}
	if firstDate != nil && latestDate != nil {
		report.TimelineSpanDays = int(latestDate.Sub(*firstDate).Hours() / 24)
	}
	return report
}

func RenderMemoryTimelineReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderMemoryTimelineReport(ev, cfg, repoContext, ev.Repo != "" || ev.Issue.Number != 0)
}

func RenderMemoryTimelineCLIReport(cfg Config, repoContext RepoContext) string {
	return renderMemoryTimelineReport(Event{}, cfg, repoContext, false)
}

func renderMemoryTimelineReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildMemoryTimelineReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Timeline Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_timeline_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- memory_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- authority_model: `%s`\n", "repo-local-reviewed-markdown")
	fmt.Fprintf(&b, "- memory_files: `%d`\n", report.MemoryFiles)
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", report.LongTermMemoryPresent)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", report.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", report.DatedMemoryNotes)
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", report.CanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", report.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", report.LoadedMemoryNotes)
	fmt.Fprintf(&b, "- omitted_memory_notes: `%d`\n", report.OmittedMemoryNotes)
	fmt.Fprintf(&b, "- max_loaded_memory_notes: `%d`\n", report.MaxLoadedMemoryNotes)
	fmt.Fprintf(&b, "- first_memory_note: `%s`\n", report.FirstMemoryNote)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", report.LatestMemoryNote)
	fmt.Fprintf(&b, "- timeline_entries: `%d`\n", len(report.TimelineEntries))
	fmt.Fprintf(&b, "- timeline_span_days: `%d`\n", report.TimelineSpanDays)
	fmt.Fprintf(&b, "- largest_gap_days: `%d`\n", report.LargestGapDays)
	fmt.Fprintf(&b, "- gaps_over_one_day: `%d`\n", report.GapsOverOneDay)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", report.MemoryWritesAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_memory_timeline_change: `%t`\n", report.LLME2ERequiredAfterMemoryTimelineChange)
	writeMemoryValidationSummary(&b, report.Validation)
	fmt.Fprintf(&b, "- memory_risk_status: `%s`\n", report.Risk.Status)
	fmt.Fprintf(&b, "- memory_risk_findings: `%d`\n", len(report.Risk.Findings))
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report turns repo-local memory into a body-free chronology. It shows long-term and dated memory files, prompt-visible load state, validation/risk gates, hashes, and note gaps only; raw memory bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Timeline Entries\n")
	if len(report.TimelineEntries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, entry := range report.TimelineEntries {
			writeMemoryTimelineEntry(&b, entry)
		}
	}

	b.WriteString("\n### Timeline Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	return strings.TrimSpace(b.String())
}

func writeMemoryTimelineEntry(b *strings.Builder, entry MemoryTimelineEntry) {
	fmt.Fprintf(
		b,
		"- position=`%d` kind=`%s` path=`%s` source=`%s` date=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` prompt_visible=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` at_context_limit=`%t` gap_days_since_previous_note=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
		entry.Position,
		entry.Kind,
		entry.Path,
		entry.Source,
		entry.Date,
		entry.Present,
		entry.Canonical,
		entry.Latest,
		entry.LoadedForThisTurn,
		entry.PromptVisible,
		entry.Bytes,
		entry.Lines,
		entry.SHA,
		entry.AtContextLimit,
		entry.GapDaysSincePreviousNote,
		entry.RiskFindings,
		entry.RiskMaxSeverity,
		inlineListOrNone(entry.RiskCodes),
	)
}

func memoryTimelineStatus(validationStatus, riskStatus string) string {
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

func memoryTimelineDate(path string) (string, *time.Time) {
	if path == longTermMemoryPath {
		return "long-term", nil
	}
	if datedMemoryNotePattern.MatchString(path) {
		value := strings.TrimSuffix(strings.TrimPrefix(path, ".gitclaw/memory/"), ".md")
		parsed, err := time.Parse("2006-01-02", value)
		if err == nil {
			return value, &parsed
		}
	}
	if strings.HasPrefix(path, ".gitclaw/memory/") {
		return "noncanonical", nil
	}
	return "unknown", nil
}
