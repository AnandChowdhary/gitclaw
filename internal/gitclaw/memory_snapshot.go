package gitclaw

import (
	"fmt"
	"strings"
)

const memorySnapshotVersion = "gitclaw-memory-snapshot-v1"

type MemorySnapshotReport struct {
	Status                                  string
	SnapshotVersion                         string
	SnapshotScope                           string
	SnapshotSHA                             string
	SnapshotEntries                         int
	LongTermEntries                         int
	DatedNoteEntries                        int
	MemoryNoteEntries                       int
	PromptVisibleEntries                    int
	LoadedMemoryEntries                     int
	OmittedMemoryEntries                    int
	MemoryFiles                             int
	LongTermMemoryPresent                   bool
	LongTermMemoryLoaded                    bool
	DatedMemoryNotes                        int
	CanonicalDatedMemoryNotes               int
	NoncanonicalDatedMemoryNotes            int
	LoadedMemoryNotes                       int
	MaxLoadedMemoryNotes                    int
	FirstMemoryNote                         string
	LatestMemoryNote                        string
	TimelineSpanDays                        int
	LargestGapDays                          int
	GapsOverOneDay                          int
	RawMemoryBodiesIncluded                 bool
	RawIssueBodiesIncluded                  bool
	RawCommentBodiesIncluded                bool
	RawPromptBodiesIncluded                 bool
	RawSessionBodiesIncluded                bool
	EmbeddingVectorsIncluded                bool
	ExternalProviderAccessed                bool
	MemoryWritesAllowed                     bool
	BackgroundPromotionActive               bool
	LLME2ERequiredAfterMemorySnapshotChange bool
	Validation                              MemoryValidationReport
	Risk                                    MemoryRiskReport
	Cards                                   []MemorySnapshotCard
}

type MemorySnapshotCard struct {
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
	AtContextLimit     bool
	GapDays            string
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	ValidationFindings int
}

func BuildMemorySnapshotReport(cfg Config, repoContext RepoContext) MemorySnapshotReport {
	timeline := BuildMemoryTimelineReport(cfg, repoContext)
	validationByPath := memoryValidationFindingCountByPath(timeline.Validation)
	report := MemorySnapshotReport{
		Status:                                  timeline.Status,
		SnapshotVersion:                         memorySnapshotVersion,
		SnapshotScope:                           "repo-local-durable-memory",
		OmittedMemoryEntries:                    timeline.OmittedMemoryNotes,
		MemoryFiles:                             timeline.MemoryFiles,
		LongTermMemoryPresent:                   timeline.LongTermMemoryPresent,
		LongTermMemoryLoaded:                    timeline.LongTermMemoryLoaded,
		DatedMemoryNotes:                        timeline.DatedMemoryNotes,
		CanonicalDatedMemoryNotes:               timeline.CanonicalDatedMemoryNotes,
		NoncanonicalDatedMemoryNotes:            timeline.NoncanonicalDatedMemoryNotes,
		LoadedMemoryNotes:                       timeline.LoadedMemoryNotes,
		MaxLoadedMemoryNotes:                    timeline.MaxLoadedMemoryNotes,
		FirstMemoryNote:                         timeline.FirstMemoryNote,
		LatestMemoryNote:                        timeline.LatestMemoryNote,
		TimelineSpanDays:                        timeline.TimelineSpanDays,
		LargestGapDays:                          timeline.LargestGapDays,
		GapsOverOneDay:                          timeline.GapsOverOneDay,
		RawMemoryBodiesIncluded:                 false,
		RawIssueBodiesIncluded:                  false,
		RawCommentBodiesIncluded:                false,
		RawPromptBodiesIncluded:                 false,
		RawSessionBodiesIncluded:                false,
		EmbeddingVectorsIncluded:                false,
		ExternalProviderAccessed:                false,
		MemoryWritesAllowed:                     false,
		BackgroundPromotionActive:               false,
		LLME2ERequiredAfterMemorySnapshotChange: true,
		Validation:                              timeline.Validation,
		Risk:                                    timeline.Risk,
	}
	for _, entry := range timeline.TimelineEntries {
		card := MemorySnapshotCard{
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
			AtContextLimit:     entry.AtContextLimit,
			GapDays:            entry.GapDaysSincePreviousNote,
			RiskFindings:       entry.RiskFindings,
			RiskMaxSeverity:    entry.RiskMaxSeverity,
			RiskCodes:          append([]string(nil), entry.RiskCodes...),
			ValidationFindings: validationByPath[entry.Path],
		}
		report.addCard(card)
	}
	report.SnapshotEntries = len(report.Cards)
	report.SnapshotSHA = memorySnapshotManifestHash(report.Cards)
	return report
}

func RenderMemorySnapshotCLIReport(cfg Config, repoContext RepoContext) string {
	return renderMemorySnapshotReport(Event{}, cfg, repoContext, false)
}

func RenderMemorySnapshotReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderMemorySnapshotReport(ev, cfg, repoContext, true)
}

func renderMemorySnapshotReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildMemorySnapshotReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Snapshot Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_snapshot_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", report.SnapshotVersion)
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", report.SnapshotScope)
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", report.SnapshotSHA)
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", report.SnapshotEntries)
	fmt.Fprintf(&b, "- long_term_entries: `%d`\n", report.LongTermEntries)
	fmt.Fprintf(&b, "- dated_note_entries: `%d`\n", report.DatedNoteEntries)
	fmt.Fprintf(&b, "- memory_note_entries: `%d`\n", report.MemoryNoteEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", report.PromptVisibleEntries)
	fmt.Fprintf(&b, "- loaded_memory_entries: `%d`\n", report.LoadedMemoryEntries)
	fmt.Fprintf(&b, "- omitted_memory_entries: `%d`\n", report.OmittedMemoryEntries)
	fmt.Fprintf(&b, "- memory_files: `%d`\n", report.MemoryFiles)
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", report.LongTermMemoryPresent)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", report.LongTermMemoryLoaded)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", report.DatedMemoryNotes)
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", report.CanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", report.NoncanonicalDatedMemoryNotes)
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", report.LoadedMemoryNotes)
	fmt.Fprintf(&b, "- max_loaded_memory_notes: `%d`\n", report.MaxLoadedMemoryNotes)
	fmt.Fprintf(&b, "- first_memory_note: `%s`\n", report.FirstMemoryNote)
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", report.LatestMemoryNote)
	fmt.Fprintf(&b, "- timeline_span_days: `%d`\n", report.TimelineSpanDays)
	fmt.Fprintf(&b, "- largest_gap_days: `%d`\n", report.LargestGapDays)
	fmt.Fprintf(&b, "- gaps_over_one_day: `%d`\n", report.GapsOverOneDay)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", report.RawMemoryBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_session_bodies_included: `%t`\n", report.RawSessionBodiesIncluded)
	fmt.Fprintf(&b, "- embedding_vectors_included: `%t`\n", report.EmbeddingVectorsIncluded)
	fmt.Fprintf(&b, "- external_provider_accessed: `%t`\n", report.ExternalProviderAccessed)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", report.MemoryWritesAllowed)
	fmt.Fprintf(&b, "- background_promotion_active: `%t`\n", report.BackgroundPromotionActive)
	fmt.Fprintf(&b, "- llm_e2e_required_after_memory_snapshot_change: `%t`\n", report.LLME2ERequiredAfterMemorySnapshotChange)
	writeMemoryValidationSummary(&b, report.Validation)
	fmt.Fprintf(&b, "- memory_risk_status: `%s`\n", report.Risk.Status)
	fmt.Fprintf(&b, "- memory_risk_findings: `%d`\n", len(report.Risk.Findings))
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report fingerprints GitClaw's durable repo-local memory in the spirit of OpenClaw Markdown memory and Hermes profile/session boundaries. It emits per-memory-file metadata plus one composite snapshot hash only; raw memory bodies, issue bodies, comments, prompts, session transcripts, embedding vectors, credentials, and secret values are not included.\n\n")

	b.WriteString("### Snapshot Entries\n")
	writeMemorySnapshotCards(&b, report.Cards)

	b.WriteString("\n### Snapshot Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	b.WriteString("- memory_write_gate=`disabled`\n")
	b.WriteString("- external_provider_gate=`not_configured`\n")
	b.WriteString("- session_search_gate=`github-issues-and-backups`\n")
	b.WriteString("- background_promotion_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- snapshot_hash_gate=`composite-sha256_12`\n")
	return strings.TrimSpace(b.String())
}

func writeMemorySnapshotCards(b *strings.Builder, cards []MemorySnapshotCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- position=`%d` kind=`%s` path=`%s` source=`%s` role=`%s` date=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` prompt_visible=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` at_context_limit=`%t` gap_days_since_previous_note=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` validation_findings=`%d`\n",
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
			card.AtContextLimit,
			card.GapDays,
			card.RiskFindings,
			noneIfEmpty(card.RiskMaxSeverity),
			inlineListOrNone(card.RiskCodes),
			card.ValidationFindings,
		)
	}
}

func (r *MemorySnapshotReport) addCard(card MemorySnapshotCard) {
	if card.SHA == "" {
		card.SHA = "none"
	}
	if card.RiskMaxSeverity == "" {
		card.RiskMaxSeverity = "none"
	}
	r.Cards = append(r.Cards, card)
	switch card.Kind {
	case "long-term":
		r.LongTermEntries++
	case "dated-note":
		r.DatedNoteEntries++
	case "memory-note":
		r.MemoryNoteEntries++
	}
	if card.PromptVisible {
		r.PromptVisibleEntries++
	}
	if card.LoadedForThisTurn {
		r.LoadedMemoryEntries++
	}
}

func memorySnapshotManifestHash(cards []MemorySnapshotCard) string {
	var b strings.Builder
	b.WriteString(memorySnapshotVersion)
	b.WriteByte('\n')
	for _, card := range cards {
		fmt.Fprintf(&b, "%03d|%s|%s|%s|%s|%s|%t|%t|%t|%t|%t|%d|%d|%s|%t|%s|%d|%s|%s|%d\n",
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
			card.AtContextLimit,
			card.GapDays,
			card.RiskFindings,
			card.RiskMaxSeverity,
			strings.Join(card.RiskCodes, ","),
			card.ValidationFindings,
		)
	}
	return shortDocumentHash(b.String())
}

func isMemorySnapshotRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || (fields[0] != "/memory" && fields[0] != "/memories") {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return true
	default:
		return false
	}
}
