package gitclaw

import (
	"fmt"
	"strings"
)

const soulSnapshotVersion = "gitclaw-soul-snapshot-v1"

type SoulSnapshotReport struct {
	Status                                string
	AnchorReport                          SoulAnchorReport
	SnapshotVersion                       string
	SnapshotScope                         string
	SnapshotSHA                           string
	SnapshotEntries                       int
	LoadedSnapshotEntries                 int
	RequiredSnapshotEntries               int
	RequiredLoadedEntries                 int
	MissingRequiredEntries                int
	OptionalLoadedEntries                 int
	MemoryNoteEntries                     int
	PromptVisibleEntries                  int
	RegistryContactAllowed                bool
	ProfileExportAllowed                  bool
	SoulWritesAllowed                     bool
	RepositoryMutationAllowed             bool
	RawBodiesIncluded                     bool
	RawDescriptionsIncluded               bool
	LLME2ERequiredAfterSoulSnapshotChange bool
	Cards                                 []SoulSnapshotCard
}

type SoulSnapshotCard struct {
	Position         int
	Name             string
	Path             string
	Category         string
	Role             string
	Authority        string
	Source           string
	Required         bool
	Loaded           bool
	PromptVisible    bool
	Canonical        bool
	LatestMemoryNote bool
	LoadState        string
	Bytes            int
	Lines            int
	SHA              string
	RiskFindings     int
	RiskMaxSeverity  string
	RiskCodes        []string
}

func BuildSoulSnapshotReport(repoContext RepoContext) SoulSnapshotReport {
	anchors := BuildSoulAnchorReport(repoContext)
	report := SoulSnapshotReport{
		Status:                                anchors.Status,
		AnchorReport:                          anchors,
		SnapshotVersion:                       soulSnapshotVersion,
		SnapshotScope:                         "repo-local-high-authority-context",
		SnapshotEntries:                       anchors.AnchorCount,
		LoadedSnapshotEntries:                 anchors.LoadedAnchors,
		RequiredSnapshotEntries:               anchors.RequiredAnchors,
		RequiredLoadedEntries:                 anchors.RequiredAnchorsLoaded,
		MissingRequiredEntries:                anchors.RequiredAnchorsMissing,
		OptionalLoadedEntries:                 anchors.OptionalAnchorsLoaded,
		MemoryNoteEntries:                     anchors.MemoryNoteAnchors,
		PromptVisibleEntries:                  anchors.PromptVisibleAnchors,
		RegistryContactAllowed:                false,
		ProfileExportAllowed:                  false,
		SoulWritesAllowed:                     false,
		RepositoryMutationAllowed:             false,
		RawBodiesIncluded:                     false,
		RawDescriptionsIncluded:               false,
		LLME2ERequiredAfterSoulSnapshotChange: true,
	}
	for i, anchor := range anchors.Anchors {
		sha := anchor.SHA
		if sha == "" {
			sha = "none"
		}
		report.Cards = append(report.Cards, SoulSnapshotCard{
			Position:         i + 1,
			Name:             anchor.Name,
			Path:             anchor.Path,
			Category:         anchor.Category,
			Role:             anchor.Role,
			Authority:        anchor.Authority,
			Source:           anchor.Source,
			Required:         anchor.Required,
			Loaded:           anchor.Loaded,
			PromptVisible:    anchor.PromptVisible,
			Canonical:        anchor.Canonical,
			LatestMemoryNote: anchor.LatestMemoryNote,
			LoadState:        soulSnapshotLoadState(anchor),
			Bytes:            anchor.Bytes,
			Lines:            anchor.Lines,
			SHA:              sha,
			RiskFindings:     anchor.RiskFindings,
			RiskMaxSeverity:  noneIfEmpty(anchor.RiskMaxSeverity),
			RiskCodes:        anchor.RiskCodes,
		})
	}
	report.SnapshotSHA = soulSnapshotManifestHash(report.Cards)
	return report
}

func RenderSoulSnapshotCLIReport(repoContext RepoContext) string {
	return renderSoulSnapshotReport(Event{}, repoContext, false)
}

func renderSoulSnapshotReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSoulSnapshotReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Snapshot Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_snapshot_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", report.SnapshotVersion)
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", report.SnapshotScope)
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", report.SnapshotSHA)
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", report.SnapshotEntries)
	fmt.Fprintf(&b, "- loaded_snapshot_entries: `%d`\n", report.LoadedSnapshotEntries)
	fmt.Fprintf(&b, "- required_snapshot_entries: `%d`\n", report.RequiredSnapshotEntries)
	fmt.Fprintf(&b, "- required_loaded_entries: `%d`\n", report.RequiredLoadedEntries)
	fmt.Fprintf(&b, "- missing_required_entries: `%d`\n", report.MissingRequiredEntries)
	fmt.Fprintf(&b, "- optional_loaded_entries: `%d`\n", report.OptionalLoadedEntries)
	fmt.Fprintf(&b, "- memory_note_entries: `%d`\n", report.MemoryNoteEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", report.PromptVisibleEntries)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(&b, "- profile_export_allowed: `%t`\n", report.ProfileExportAllowed)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", report.SoulWritesAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_descriptions_included: `%t`\n", report.RawDescriptionsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_soul_snapshot_change: `%t`\n", report.LLME2ERequiredAfterSoulSnapshotChange)
	writeSoulValidationSummary(&b, report.AnchorReport.Validation)
	writeSoulRiskSummary(&b, report.AnchorReport.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report fingerprints GitClaw's repo-local high-authority context in the spirit of OpenClaw workspace files and Hermes profiles. It emits paths, categories, roles, load states, short hashes, and one composite snapshot hash only; raw soul, identity, user, memory, tool, heartbeat, issue, comment, prompt, and secret bodies are not included.\n\n")

	b.WriteString("### Snapshot Entries\n")
	writeSoulSnapshotCards(&b, report.Cards)

	b.WriteString("\n### Snapshot Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.AnchorReport.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.AnchorReport.Risk.Status))
	fmt.Fprintf(&b, "- registry_gate=`disabled`\n")
	fmt.Fprintf(&b, "- profile_export_gate=`disabled`\n")
	fmt.Fprintf(&b, "- mutation_gate=`disabled`\n")
	fmt.Fprintf(&b, "- body_hash_gate=`sha256_12`\n")
	fmt.Fprintf(&b, "- snapshot_hash_gate=`composite-sha256_12`\n")
	return strings.TrimSpace(b.String())
}

func writeSoulSnapshotCards(b *strings.Builder, cards []SoulSnapshotCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- position=`%d` name=`%s` path=`%s` category=`%s` role=`%s` authority=`%s` source=`%s` required=`%t` loaded=`%t` prompt_visible=`%t` canonical=`%t` latest_memory_note=`%t` load_state=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			card.Position,
			card.Name,
			card.Path,
			card.Category,
			card.Role,
			card.Authority,
			card.Source,
			card.Required,
			card.Loaded,
			card.PromptVisible,
			card.Canonical,
			card.LatestMemoryNote,
			card.LoadState,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.RiskFindings,
			card.RiskMaxSeverity,
			inlineListOrNone(card.RiskCodes),
		)
	}
}

func soulSnapshotManifestHash(cards []SoulSnapshotCard) string {
	var b strings.Builder
	b.WriteString(soulSnapshotVersion)
	b.WriteByte('\n')
	for _, card := range cards {
		fmt.Fprintf(&b, "%03d|%s|%s|%s|%s|%s|%s|%t|%t|%t|%t|%s|%d|%d|%s|%d|%s|%s\n",
			card.Position,
			card.Name,
			card.Path,
			card.Category,
			card.Role,
			card.Authority,
			card.Source,
			card.Required,
			card.Loaded,
			card.PromptVisible,
			card.Canonical,
			card.LoadState,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.RiskFindings,
			card.RiskMaxSeverity,
			strings.Join(card.RiskCodes, ","),
		)
	}
	return shortDocumentHash(b.String())
}

func soulSnapshotLoadState(anchor SoulAnchor) string {
	switch {
	case anchor.Required && anchor.Loaded:
		return "required-loaded"
	case anchor.Required && !anchor.Loaded:
		return "required-missing"
	case anchor.Loaded:
		return "optional-loaded"
	default:
		return "optional-missing"
	}
}

func isSoulSnapshotRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/soul" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return true
	default:
		return false
	}
}
