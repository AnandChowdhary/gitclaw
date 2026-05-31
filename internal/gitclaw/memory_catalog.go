package gitclaw

import (
	"fmt"
	"strings"
)

type MemoryCatalogReport struct {
	Status                    string
	Timeline                  MemoryTimelineReport
	CatalogedEntries          int
	LongTermEntries           int
	DatedNoteEntries          int
	MemoryNoteEntries         int
	PromptVisibleEntries      int
	LoadedMemoryEntries       int
	OmittedMemoryEntries      int
	RawMemoryBodiesIncluded   bool
	RawIssueBodiesIncluded    bool
	RawCommentBodiesIncluded  bool
	RawPromptBodiesIncluded   bool
	RawSessionBodiesIncluded  bool
	EmbeddingVectorsIncluded  bool
	ExternalProviderAccessed  bool
	MemoryWritesAllowed       bool
	BackgroundPromotionActive bool
	LLME2ERequiredAfterChange bool
	Entries                   []MemoryCatalogEntry
}

type MemoryCatalogEntry struct {
	Position           int
	Kind               string
	Path               string
	MemoryLayer        string
	Source             string
	Role               string
	Date               string
	Present            bool
	Canonical          bool
	Latest             bool
	LoadedForThisTurn  bool
	PromptVisible      bool
	LoadMode           string
	Bytes              int
	Lines              int
	SHA                string
	AtContextLimit     bool
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	ValidationFindings int
	ReasonCodes        []string
}

func BuildMemoryCatalogReport(cfg Config, repoContext RepoContext) MemoryCatalogReport {
	timeline := BuildMemoryTimelineReport(cfg, repoContext)
	validationByPath := memoryValidationFindingCountByPath(timeline.Validation)
	report := MemoryCatalogReport{
		Status:                    timeline.Status,
		Timeline:                  timeline,
		OmittedMemoryEntries:      timeline.OmittedMemoryNotes,
		RawMemoryBodiesIncluded:   false,
		RawIssueBodiesIncluded:    false,
		RawCommentBodiesIncluded:  false,
		RawPromptBodiesIncluded:   false,
		RawSessionBodiesIncluded:  false,
		EmbeddingVectorsIncluded:  false,
		ExternalProviderAccessed:  false,
		MemoryWritesAllowed:       false,
		BackgroundPromotionActive: false,
		LLME2ERequiredAfterChange: true,
	}
	for _, entry := range timeline.TimelineEntries {
		catalogEntry := MemoryCatalogEntry{
			Position:           entry.Position,
			Kind:               entry.Kind,
			Path:               entry.Path,
			MemoryLayer:        memoryCatalogLayer(entry),
			Source:             entry.Source,
			Role:               memoryCatalogRole(entry),
			Date:               entry.Date,
			Present:            entry.Present,
			Canonical:          entry.Canonical,
			Latest:             entry.Latest,
			LoadedForThisTurn:  entry.LoadedForThisTurn,
			PromptVisible:      entry.PromptVisible,
			LoadMode:           memoryCatalogLoadMode(entry),
			Bytes:              entry.Bytes,
			Lines:              entry.Lines,
			SHA:                entry.SHA,
			AtContextLimit:     entry.AtContextLimit,
			RiskFindings:       entry.RiskFindings,
			RiskMaxSeverity:    entry.RiskMaxSeverity,
			RiskCodes:          append([]string(nil), entry.RiskCodes...),
			ValidationFindings: validationByPath[entry.Path],
		}
		catalogEntry.ReasonCodes = memoryCatalogReasonCodes(catalogEntry)
		report.Entries = append(report.Entries, catalogEntry)
		report.CatalogedEntries++
		if catalogEntry.Kind == "long-term" {
			report.LongTermEntries++
		} else if catalogEntry.Kind == "dated-note" {
			report.DatedNoteEntries++
		} else {
			report.MemoryNoteEntries++
		}
		if catalogEntry.PromptVisible {
			report.PromptVisibleEntries++
		}
		if catalogEntry.LoadedForThisTurn {
			report.LoadedMemoryEntries++
		}
	}
	return report
}

func RenderMemoryCatalogCLIReport(cfg Config, repoContext RepoContext) string {
	return renderMemoryCatalogReport(Event{}, cfg, repoContext, false)
}

func RenderMemoryCatalogReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderMemoryCatalogReport(ev, cfg, repoContext, true)
}

func renderMemoryCatalogReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildMemoryCatalogReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_catalog_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-durable-memory-discovery")
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "repo-local-memory-notes-session-search")
	fmt.Fprintf(&b, "- memory_model: `%s`\n", "repo-local-reviewed-markdown")
	fmt.Fprintf(&b, "- hermes_memory_layers: `%s`\n", "durable-memory, procedural-skills, session-search")
	fmt.Fprintf(&b, "- durable_memory_layer: `%s`\n", "git-backed-markdown")
	fmt.Fprintf(&b, "- procedural_memory_layer: `%s`\n", "skills-catalog-separate")
	fmt.Fprintf(&b, "- session_search_layer: `%s`\n", "github-issues-and-backups")
	fmt.Fprintf(&b, "- cataloged_entries: `%d`\n", report.CatalogedEntries)
	fmt.Fprintf(&b, "- long_term_entries: `%d`\n", report.LongTermEntries)
	fmt.Fprintf(&b, "- dated_note_entries: `%d`\n", report.DatedNoteEntries)
	fmt.Fprintf(&b, "- memory_note_entries: `%d`\n", report.MemoryNoteEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", report.PromptVisibleEntries)
	fmt.Fprintf(&b, "- loaded_memory_entries: `%d`\n", report.LoadedMemoryEntries)
	fmt.Fprintf(&b, "- omitted_memory_entries: `%d`\n", report.OmittedMemoryEntries)
	fmt.Fprintf(&b, "- memory_files: `%d`\n", report.Timeline.MemoryFiles)
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", report.Timeline.LongTermMemoryPresent)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", report.Timeline.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", report.Timeline.DatedMemoryNotes)
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", report.Timeline.CanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", report.Timeline.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", report.Timeline.LoadedMemoryNotes)
	fmt.Fprintf(&b, "- omitted_memory_notes: `%d`\n", report.Timeline.OmittedMemoryNotes)
	fmt.Fprintf(&b, "- max_loaded_memory_notes: `%d`\n", report.Timeline.MaxLoadedMemoryNotes)
	fmt.Fprintf(&b, "- first_memory_note: `%s`\n", report.Timeline.FirstMemoryNote)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", report.Timeline.LatestMemoryNote)
	fmt.Fprintf(&b, "- timeline_span_days: `%d`\n", report.Timeline.TimelineSpanDays)
	fmt.Fprintf(&b, "- largest_gap_days: `%d`\n", report.Timeline.LargestGapDays)
	fmt.Fprintf(&b, "- gaps_over_one_day: `%d`\n", report.Timeline.GapsOverOneDay)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", report.RawMemoryBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_session_bodies_included: `%t`\n", report.RawSessionBodiesIncluded)
	fmt.Fprintf(&b, "- embedding_vectors_included: `%t`\n", report.EmbeddingVectorsIncluded)
	fmt.Fprintf(&b, "- external_provider_accessed: `%t`\n", report.ExternalProviderAccessed)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", report.MemoryWritesAllowed)
	fmt.Fprintf(&b, "- background_promotion_active: `%t`\n", report.BackgroundPromotionActive)
	fmt.Fprintf(&b, "- llm_e2e_required_after_memory_catalog_change: `%t`\n", report.LLME2ERequiredAfterChange)
	writeMemoryValidationSummary(&b, report.Timeline.Validation)
	fmt.Fprintf(&b, "- memory_risk_status: `%s`\n", report.Timeline.Risk.Status)
	fmt.Fprintf(&b, "- memory_risk_findings: `%d`\n", len(report.Timeline.Risk.Findings))
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This compact catalog follows OpenClaw and Hermes memory-layer boundaries: durable facts live in reviewed repo-local Markdown, procedural memory stays in skills, and session recall stays in GitHub issues/backups. Raw memory bodies, issue bodies, comments, prompts, session transcripts, embedding vectors, credentials, and secret values are not included.\n\n")

	b.WriteString("### Memory Catalog Entries\n")
	if len(report.Entries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, entry := range report.Entries {
			writeMemoryCatalogEntry(&b, entry)
		}
	}

	b.WriteString("\n### Catalog Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Timeline.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Timeline.Risk.Status))
	fmt.Fprintf(&b, "- memory_write_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- external_provider_gate=`%s`\n", "not_configured")
	fmt.Fprintf(&b, "- session_search_gate=`%s`\n", "github-issues-and-backups")
	fmt.Fprintf(&b, "- background_promotion_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- body_hash_gate=`%s`\n", "sha256_12")
	return strings.TrimSpace(b.String())
}

func writeMemoryCatalogEntry(b *strings.Builder, entry MemoryCatalogEntry) {
	fmt.Fprintf(
		b,
		"- position=`%d` kind=`%s` path=`%s` memory_layer=`%s` source=`%s` role=`%s` date=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` prompt_visible=`%t` load_mode=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` at_context_limit=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` validation_findings=`%d` reason_codes=`%s`\n",
		entry.Position,
		entry.Kind,
		entry.Path,
		entry.MemoryLayer,
		entry.Source,
		entry.Role,
		entry.Date,
		entry.Present,
		entry.Canonical,
		entry.Latest,
		entry.LoadedForThisTurn,
		entry.PromptVisible,
		entry.LoadMode,
		entry.Bytes,
		entry.Lines,
		entry.SHA,
		entry.AtContextLimit,
		entry.RiskFindings,
		entry.RiskMaxSeverity,
		inlineListOrNone(entry.RiskCodes),
		entry.ValidationFindings,
		memoryCatalogReasonCodeList(entry.ReasonCodes),
	)
}

func memoryValidationFindingCountByPath(validation MemoryValidationReport) map[string]int {
	counts := map[string]int{}
	for _, finding := range validation.Findings {
		counts[finding.Path]++
	}
	return counts
}

func memoryCatalogLayer(entry MemoryTimelineEntry) string {
	switch entry.Kind {
	case "long-term", "dated-note", "memory-note":
		return "durable-memory"
	default:
		return "unknown"
	}
}

func memoryCatalogRole(entry MemoryTimelineEntry) string {
	switch entry.Kind {
	case "long-term":
		return "stable-summary"
	case "dated-note":
		if entry.Latest {
			return "latest-daily-note"
		}
		return "daily-note"
	case "memory-note":
		return "noncanonical-note"
	default:
		return "unknown"
	}
}

func memoryCatalogLoadMode(entry MemoryTimelineEntry) string {
	switch {
	case !entry.Present:
		return "missing"
	case entry.PromptVisible && entry.AtContextLimit:
		return "prompt-visible-at-limit"
	case entry.PromptVisible:
		return "prompt-visible"
	case entry.LoadedForThisTurn:
		return "loaded"
	default:
		return "not-loaded"
	}
}

func memoryCatalogReasonCodes(entry MemoryCatalogEntry) []string {
	reasons := []string{strings.ReplaceAll(entry.Kind, "-", "_"), strings.ReplaceAll(entry.MemoryLayer, "-", "_"), strings.ReplaceAll(entry.Role, "-", "_")}
	if entry.Present {
		reasons = append(reasons, "present")
	} else {
		reasons = append(reasons, "missing")
	}
	if entry.Canonical {
		reasons = append(reasons, "canonical")
	} else {
		reasons = append(reasons, "noncanonical")
	}
	if entry.Latest {
		reasons = append(reasons, "latest")
	} else {
		reasons = append(reasons, "not_latest")
	}
	if entry.LoadedForThisTurn {
		reasons = append(reasons, "loaded")
	} else {
		reasons = append(reasons, "not_loaded")
	}
	if entry.PromptVisible {
		reasons = append(reasons, "prompt_visible")
	} else {
		reasons = append(reasons, "not_prompt_visible")
	}
	if entry.AtContextLimit {
		reasons = append(reasons, "at_context_limit")
	} else {
		reasons = append(reasons, "below_context_limit")
	}
	if entry.RiskFindings > 0 {
		reasons = append(reasons, "risk_findings")
	} else {
		reasons = append(reasons, "no_risk_findings")
	}
	if entry.ValidationFindings > 0 {
		reasons = append(reasons, "validation_findings")
	} else {
		reasons = append(reasons, "no_validation_findings")
	}
	return uniqueSortedStrings(reasons)
}

func memoryCatalogReasonCodeList(reasons []string) string {
	if len(reasons) == 0 {
		return "none"
	}
	return strings.Join(reasons, ", ")
}
