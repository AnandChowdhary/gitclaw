package gitclaw

import (
	"fmt"
	"strings"
)

type MemoryVerifyReport struct {
	Status                          string
	Validation                      MemoryValidationReport
	MemoryFiles                     int
	RepoLocalMemoryFiles            int
	UnknownMemoryFiles              int
	LongTermMemoryPresent           bool
	LongTermMemoryLoaded            bool
	DatedMemoryNotes                int
	CanonicalDatedMemoryNotes       int
	NoncanonicalDatedMemoryNotes    int
	LoadedMemoryNotes               int
	OmittedMemoryNotes              int
	MaxLoadedMemoryNotes            int
	LatestMemoryNote                string
	MemoryFilesHashed               int
	MemoryFilesAtLimit              int
	PotentialSecretFindings         int
	ExternalProviderVerification    string
	SessionSearchIndexVerification  string
	BackgroundPromotionVerification string
	MemoryWritesAllowed             bool
	RawBodiesIncluded               bool
}

func BuildMemoryVerifyReport(cfg Config, repoContext RepoContext) MemoryVerifyReport {
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	files := memoryVerifyFiles(surface)
	report := MemoryVerifyReport{
		Status:                          validation.Status,
		Validation:                      validation,
		MemoryFiles:                     len(files),
		LongTermMemoryPresent:           surface.LongTerm.Present,
		LongTermMemoryLoaded:            surface.LoadedLongTerm,
		DatedMemoryNotes:                len(surface.DatedNotes),
		CanonicalDatedMemoryNotes:       countCanonicalMemoryNotes(surface.DatedNotes),
		NoncanonicalDatedMemoryNotes:    countNoncanonicalMemoryNotes(surface.DatedNotes),
		LoadedMemoryNotes:               len(surface.LoadedNotePaths),
		OmittedMemoryNotes:              omittedMemoryNotes(surface),
		MaxLoadedMemoryNotes:            maxMemoryDocuments,
		LatestMemoryNote:                latestMemoryNotePath(surface.DatedNotes),
		MemoryFilesAtLimit:              validation.MemoryFilesAtLimit,
		PotentialSecretFindings:         validation.PotentialSecretFindings,
		ExternalProviderVerification:    "not_configured",
		SessionSearchIndexVerification:  "not_configured",
		BackgroundPromotionVerification: "not_configured",
		MemoryWritesAllowed:             false,
		RawBodiesIncluded:               false,
	}
	for _, file := range files {
		switch memoryTrustSource(file.Path) {
		case "repo-local":
			report.RepoLocalMemoryFiles++
		default:
			report.UnknownMemoryFiles++
		}
		if file.Present && file.SHA != "" {
			report.MemoryFilesHashed++
		}
	}
	if report.UnknownMemoryFiles > 0 && report.Status == "ok" {
		report.Status = "warn"
	}
	return report
}

func RenderMemoryVerifyReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderMemoryVerifyReport(ev, cfg, repoContext, ev.Repo != "" || ev.Issue.Number != 0)
}

func renderMemoryVerifyReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildMemoryVerifyReport(cfg, repoContext)
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_verify_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "repo-local-memory-provenance")
	fmt.Fprintf(&b, "- memory_files: `%d`\n", report.MemoryFiles)
	fmt.Fprintf(&b, "- repo_local_memory_files: `%d`\n", report.RepoLocalMemoryFiles)
	fmt.Fprintf(&b, "- unknown_memory_files: `%d`\n", report.UnknownMemoryFiles)
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", report.LongTermMemoryPresent)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", report.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", report.DatedMemoryNotes)
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", report.CanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", report.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", report.LoadedMemoryNotes)
	fmt.Fprintf(&b, "- omitted_memory_notes: `%d`\n", report.OmittedMemoryNotes)
	fmt.Fprintf(&b, "- max_loaded_memory_notes: `%d`\n", report.MaxLoadedMemoryNotes)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", report.LatestMemoryNote)
	fmt.Fprintf(&b, "- memory_files_hashed: `%d`\n", report.MemoryFilesHashed)
	fmt.Fprintf(&b, "- memory_files_at_limit: `%d`\n", report.MemoryFilesAtLimit)
	fmt.Fprintf(&b, "- potential_secret_findings: `%d`\n", report.PotentialSecretFindings)
	fmt.Fprintf(&b, "- external_provider_verification: `%s`\n", report.ExternalProviderVerification)
	fmt.Fprintf(&b, "- session_search_index_verification: `%s`\n", report.SessionSearchIndexVerification)
	fmt.Fprintf(&b, "- background_promotion_verification: `%s`\n", report.BackgroundPromotionVerification)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", report.MemoryWritesAllowed)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	writeMemoryValidationSummary(&b, report.Validation)
	b.WriteByte('\n')
	b.WriteString("This report verifies GitClaw's repo-local memory provenance. It reports long-term memory, dated memory-note presence, load state, hashes, hygiene findings, and explicit non-goals only; raw memory, issue, comment, prompt, and secret bodies are not included.\n\n")

	b.WriteString("### Trust Cards\n")
	writeMemoryTrustCards(&b, surface)

	b.WriteString("\n### Verification Findings\n")
	writeMemoryVerifyFindings(&b, report)
	return strings.TrimSpace(b.String())
}

func writeMemoryTrustCards(b *strings.Builder, surface memorySurface) {
	loaded := loadedMemoryPathSet(surface)
	if surface.LongTerm.Present {
		writeMemoryTrustCard(b, surface.LongTerm, "long-term", true, loaded[surface.LongTerm.Path])
	} else {
		b.WriteString("- kind=`long-term` path=`.gitclaw/MEMORY.md` present=`false`\n")
	}
	if len(surface.DatedNotes) == 0 {
		b.WriteString("- kind=`dated-note` none\n")
		return
	}
	latest := latestMemoryNotePath(surface.DatedNotes)
	for _, file := range surface.DatedNotes {
		writeMemoryTrustCard(b, file, "dated-note", file.Path == latest, loaded[file.Path])
	}
}

func writeMemoryTrustCard(b *strings.Builder, file configSurfaceFile, kind string, latest bool, loaded bool) {
	fmt.Fprintf(b, "- kind=`%s` path=`%s` source=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` bytes=`%d` lines=`%d` sha256_12=`%s`\n",
		kind,
		file.Path,
		memoryTrustSource(file.Path),
		file.Present,
		isCanonicalMemoryPath(file.Path),
		latest,
		loaded,
		file.Bytes,
		file.Lines,
		file.SHA,
	)
}

func writeMemoryVerifyFindings(b *strings.Builder, report MemoryVerifyReport) {
	wrote := false
	if report.ExternalProviderVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`external_memory_provider_verification_not_configured` detail=`GitClaw v1 verifies repo-local Markdown memory files, not external memory providers`\n")
		wrote = true
	}
	if report.SessionSearchIndexVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`session_search_index_verification_not_configured` detail=`GitClaw uses GitHub issues and backup JSON for sessions; no separate SQLite/FTS memory index is verified`\n")
		wrote = true
	}
	if report.BackgroundPromotionVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`background_promotion_verification_not_configured` detail=`GitClaw does not auto-promote daily notes into long-term memory during assistant turns`\n")
		wrote = true
	}
	if report.UnknownMemoryFiles > 0 {
		b.WriteString("- severity=`warning` code=`unknown_memory_source` detail=`one or more memory files are outside known repo-local roots`\n")
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

func memoryVerifyFiles(surface memorySurface) []configSurfaceFile {
	files := make([]configSurfaceFile, 0, 1+len(surface.DatedNotes))
	files = append(files, surface.LongTerm)
	files = append(files, surface.DatedNotes...)
	return files
}

func memoryTrustSource(path string) string {
	if path == longTermMemoryPath || strings.HasPrefix(path, ".gitclaw/memory/") {
		return "repo-local"
	}
	return "unknown"
}

func isCanonicalMemoryPath(path string) bool {
	return path == longTermMemoryPath || datedMemoryNotePattern.MatchString(path)
}
