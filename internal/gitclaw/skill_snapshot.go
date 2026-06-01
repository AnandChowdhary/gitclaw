package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

const skillSnapshotVersion = "gitclaw-skill-snapshot-v1"

type SkillSnapshotReport struct {
	Status                            string
	SnapshotVersion                   string
	SnapshotScope                     string
	SnapshotSHA                       string
	SnapshotEntries                   int
	SkillEntries                      int
	SelectedSkillEntries              int
	BundleEntries                     int
	SourcePinEntries                  int
	PromptVisibleEntries              int
	AvailableSkills                   int
	EnabledSkills                     int
	DisabledSkills                    int
	AllowlistBlockedSkills            int
	AlwaysOnSkills                    int
	SkillsWithFrontmatter             int
	SkillsWithDescription             int
	SkillsWithRequirements            int
	SkillsMissingRequirements         int
	SkillBundles                      int
	SelectedBundles                   int
	SourcePinsScanned                 int
	HashPinnedSources                 int
	HashMatchedSources                int
	HashMismatchedSources             int
	MissingSkillMatches               int
	RemoteSourceRefs                  int
	RemoteFetchAllowedSpecs           int
	RegistryContactAllowed            bool
	RemoteFetchAllowed                bool
	InstallerScriptsRun               bool
	DependencyInstallAllowed          bool
	RepositoryMutationAllowed         bool
	RawSkillBodiesIncluded            bool
	RawSkillDescriptionsIncluded      bool
	RawSourceBodiesIncluded           bool
	RawSourceRefsIncluded             bool
	RawBundleInstructionsIncluded     bool
	LLME2ERequiredAfterSnapshotChange bool
	Validation                        SkillValidationReport
	Risk                              SkillRiskReport
	Source                            SkillSourceReport
	Cards                             []SkillSnapshotCard
}

type SkillSnapshotCard struct {
	Position             int
	Kind                 string
	Name                 string
	Source               string
	Path                 string
	Enabled              bool
	PromptVisible        bool
	SelectedForThisTurn  bool
	Always               bool
	Frontmatter          bool
	DescriptionPresent   bool
	DescriptionSHA       string
	Bytes                int
	Lines                int
	SHA                  string
	RequiredEnv          int
	RequiredBins         int
	MissingEnv           int
	MissingBins          int
	InstallSpecs         int
	InstallBins          int
	BundleSkillRefs      []string
	ResolvedBundleSkills []string
	MissingBundleSkills  []string
	InstructionPresent   bool
	InstructionSHA       string
	SkillMatched         bool
	SkillPath            string
	SkillSHA             string
	SourceKind           string
	SourceRefPresent     bool
	SourceRefSHA         string
	TrustLevel           string
	InstallMode          string
	HashPinned           bool
	HashMatched          bool
	HashMismatched       bool
	RequiresApproval     bool
	RemoteFetchAllowed   bool
	RiskFindings         int
	RiskMaxSeverity      string
	RiskCodes            []string
}

func BuildSkillSnapshotReport(cfg Config, repoContext RepoContext) SkillSnapshotReport {
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	risk := BuildSkillRiskReport(repoContext.SkillSummaries)
	source := BuildSkillSourceReport(cfg, repoContext)
	report := SkillSnapshotReport{
		Status:                            skillSnapshotStatus(validation.Status, risk.Status, source.Status),
		SnapshotVersion:                   skillSnapshotVersion,
		SnapshotScope:                     "repo-local-skills-bundles-sources",
		AvailableSkills:                   availableSkillCount(repoContext),
		EnabledSkills:                     enabledSkillCount(repoContext.SkillSummaries),
		DisabledSkills:                    disabledByConfigCount(repoContext.SkillSummaries),
		AllowlistBlockedSkills:            blockedByAllowlistCount(repoContext.SkillSummaries),
		AlwaysOnSkills:                    alwaysOnSkillCount(repoContext.SkillSummaries),
		SkillsWithFrontmatter:             skillsWithFrontmatter(repoContext.SkillSummaries),
		SkillsWithDescription:             skillsWithDescription(repoContext.SkillSummaries),
		SkillsWithRequirements:            skillsWithRequirements(repoContext.SkillSummaries),
		SkillsMissingRequirements:         skillsMissingRequirements(repoContext.SkillSummaries),
		SkillBundles:                      len(repoContext.SkillBundles),
		SelectedBundles:                   selectedSkillBundleCount(repoContext.SkillBundles),
		SourcePinsScanned:                 source.Specs,
		HashPinnedSources:                 source.HashPinnedSources,
		HashMatchedSources:                source.HashMatchedSources,
		HashMismatchedSources:             source.HashMismatchedSources,
		MissingSkillMatches:               source.MissingSkillMatches,
		RemoteSourceRefs:                  source.RemoteSourceRefs,
		RemoteFetchAllowedSpecs:           source.RemoteFetchAllowedSpecs,
		RegistryContactAllowed:            false,
		RemoteFetchAllowed:                false,
		InstallerScriptsRun:               false,
		DependencyInstallAllowed:          false,
		RepositoryMutationAllowed:         false,
		RawSkillBodiesIncluded:            false,
		RawSkillDescriptionsIncluded:      false,
		RawSourceBodiesIncluded:           false,
		RawSourceRefsIncluded:             false,
		RawBundleInstructionsIncluded:     false,
		LLME2ERequiredAfterSnapshotChange: true,
		Validation:                        validation,
		Risk:                              risk,
		Source:                            source,
	}

	skills := append([]SkillSummary(nil), repoContext.SkillSummaries...)
	sort.Slice(skills, func(i, j int) bool {
		return strings.ToLower(skills[i].Name)+"\x00"+skills[i].Path < strings.ToLower(skills[j].Name)+"\x00"+skills[j].Path
	})
	for _, skill := range skills {
		report.addCard(skillSnapshotCardForSkill(repoContext, skill))
	}

	selected := append([]ContextDocument(nil), repoContext.Skills...)
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	for _, doc := range selected {
		report.addCard(SkillSnapshotCard{
			Kind:            "selected-skill",
			Name:            doc.Path,
			Source:          "prompt-context",
			Path:            doc.Path,
			Enabled:         true,
			PromptVisible:   true,
			Bytes:           len(doc.Body),
			Lines:           lineCount(doc.Body),
			SHA:             shortDocumentHash(doc.Body),
			RiskFindings:    0,
			RiskMaxSeverity: "none",
		})
	}

	bundles := append([]SkillBundleSummary(nil), repoContext.SkillBundles...)
	sort.Slice(bundles, func(i, j int) bool {
		return strings.ToLower(bundles[i].Name)+"\x00"+bundles[i].Path < strings.ToLower(bundles[j].Name)+"\x00"+bundles[j].Path
	})
	for _, bundle := range bundles {
		report.addCard(skillSnapshotCardForBundle(bundle))
	}

	sourceCards := append([]SkillSourceCard(nil), source.Cards...)
	sort.Slice(sourceCards, func(i, j int) bool {
		return strings.ToLower(sourceCards[i].Name)+"\x00"+sourceCards[i].Path < strings.ToLower(sourceCards[j].Name)+"\x00"+sourceCards[j].Path
	})
	for _, card := range sourceCards {
		report.addCard(skillSnapshotCardForSource(card))
	}

	for i := range report.Cards {
		report.Cards[i].Position = i + 1
	}
	report.SnapshotEntries = len(report.Cards)
	report.SnapshotSHA = skillSnapshotManifestHash(report.Cards)
	return report
}

func RenderSkillSnapshotCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillSnapshotReport(Event{}, cfg, repoContext, false)
}

func renderSkillSnapshotReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillSnapshotReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Snapshot Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- skill_snapshot_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", report.SnapshotVersion)
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", report.SnapshotScope)
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", report.SnapshotSHA)
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", report.SnapshotEntries)
	fmt.Fprintf(&b, "- skill_entries: `%d`\n", report.SkillEntries)
	fmt.Fprintf(&b, "- selected_skill_entries: `%d`\n", report.SelectedSkillEntries)
	fmt.Fprintf(&b, "- bundle_entries: `%d`\n", report.BundleEntries)
	fmt.Fprintf(&b, "- source_pin_entries: `%d`\n", report.SourcePinEntries)
	fmt.Fprintf(&b, "- prompt_visible_entries: `%d`\n", report.PromptVisibleEntries)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", report.EnabledSkills)
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", report.DisabledSkills)
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", report.AllowlistBlockedSkills)
	fmt.Fprintf(&b, "- always_on_skills: `%d`\n", report.AlwaysOnSkills)
	fmt.Fprintf(&b, "- skills_with_frontmatter: `%d`\n", report.SkillsWithFrontmatter)
	fmt.Fprintf(&b, "- skills_with_description: `%d`\n", report.SkillsWithDescription)
	fmt.Fprintf(&b, "- skills_with_requirements: `%d`\n", report.SkillsWithRequirements)
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", report.SkillsMissingRequirements)
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", report.SkillBundles)
	fmt.Fprintf(&b, "- selected_bundles: `%d`\n", report.SelectedBundles)
	fmt.Fprintf(&b, "- source_pins_scanned: `%d`\n", report.SourcePinsScanned)
	fmt.Fprintf(&b, "- hash_pinned_skill_sources: `%d`\n", report.HashPinnedSources)
	fmt.Fprintf(&b, "- hash_matched_skill_sources: `%d`\n", report.HashMatchedSources)
	fmt.Fprintf(&b, "- hash_mismatched_skill_sources: `%d`\n", report.HashMismatchedSources)
	fmt.Fprintf(&b, "- missing_skill_source_matches: `%d`\n", report.MissingSkillMatches)
	fmt.Fprintf(&b, "- remote_source_refs: `%d`\n", report.RemoteSourceRefs)
	fmt.Fprintf(&b, "- remote_fetch_allowed_specs: `%d`\n", report.RemoteFetchAllowedSpecs)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", report.RemoteFetchAllowed)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_skill_descriptions_included: `%t`\n", report.RawSkillDescriptionsIncluded)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", report.RawSourceBodiesIncluded)
	fmt.Fprintf(&b, "- raw_source_refs_included: `%t`\n", report.RawSourceRefsIncluded)
	fmt.Fprintf(&b, "- raw_bundle_instructions_included: `%t`\n", report.RawBundleInstructionsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_snapshot_change: `%t`\n", report.LLME2ERequiredAfterSnapshotChange)
	writeSkillValidationSummary(&b, report.Validation)
	writeSkillRiskSummary(&b, report.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report fingerprints GitClaw's OpenClaw/Hermes-style skill surface as a body-free lockfile: local skills, prompt-visible selected skills, skill bundles, and reviewed source pins are represented by paths, counts, short hashes, and gates only. Raw skill bodies, raw descriptions, bundle instructions, source refs, issue bodies, comments, prompts, provider payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Snapshot Entries\n")
	writeSkillSnapshotCards(&b, report.Cards)

	b.WriteString("\n### Snapshot Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", skillSnapshotGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", skillSnapshotGate(report.Risk.Status))
	fmt.Fprintf(&b, "- source_gate=`%s`\n", skillSnapshotGate(report.Source.Status))
	b.WriteString("- progressive_disclosure_gate=`enabled`\n")
	b.WriteString("- registry_gate=`disabled`\n")
	b.WriteString("- remote_fetch_gate=`disabled`\n")
	b.WriteString("- installer_gate=`disabled`\n")
	b.WriteString("- dependency_install_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- snapshot_hash_gate=`composite-sha256_12`\n")
	return strings.TrimSpace(b.String())
}

func skillSnapshotCardForSkill(repoContext RepoContext, skill SkillSummary) SkillSnapshotCard {
	descriptionSHA := "none"
	if strings.TrimSpace(skill.Description) != "" {
		descriptionSHA = shortDocumentHash(skill.Description)
	}
	return SkillSnapshotCard{
		Kind:                "skill",
		Name:                skill.Name,
		Source:              "repo-local-skill",
		Path:                skill.Path,
		Enabled:             skillIsEnabled(skill),
		PromptVisible:       skill.Always || skillSelectedForTurn(repoContext, skill),
		SelectedForThisTurn: skillSelectedForTurn(repoContext, skill),
		Always:              skill.Always,
		Frontmatter:         skill.FrontmatterPresent,
		DescriptionPresent:  strings.TrimSpace(skill.Description) != "",
		DescriptionSHA:      descriptionSHA,
		Bytes:               skill.Bytes,
		Lines:               skill.Lines,
		SHA:                 skill.SHA,
		RequiredEnv:         len(skill.RequiredEnv),
		RequiredBins:        len(skill.RequiredBins),
		MissingEnv:          len(skill.MissingEnv),
		MissingBins:         len(skill.MissingBins),
		InstallSpecs:        len(skill.InstallSpecs),
		InstallBins:         skillInstallBinCount(skill.InstallSpecs),
		RiskFindings:        len(skill.RiskFindings),
		RiskMaxSeverity:     skillRiskMaxSeverity(skill.RiskFindings),
		RiskCodes:           skillRiskCodes(skill.RiskFindings),
	}
}

func skillSnapshotCardForBundle(bundle SkillBundleSummary) SkillSnapshotCard {
	return SkillSnapshotCard{
		Kind:                 "bundle",
		Name:                 bundle.Name,
		Source:               "repo-local-skill-bundle",
		Path:                 bundle.Path,
		Enabled:              true,
		PromptVisible:        bundle.Selected,
		SelectedForThisTurn:  bundle.Selected,
		Bytes:                bundle.Bytes,
		Lines:                bundle.Lines,
		SHA:                  bundle.SHA,
		BundleSkillRefs:      append([]string(nil), bundle.Skills...),
		ResolvedBundleSkills: append([]string(nil), bundle.ResolvedSkills...),
		MissingBundleSkills:  append([]string(nil), bundle.MissingSkills...),
		InstructionPresent:   bundle.InstructionPresent,
		InstructionSHA:       noneIfEmpty(bundle.InstructionSHA),
		RiskFindings:         len(bundle.RiskFindings),
		RiskMaxSeverity:      "none",
	}
}

func skillSnapshotCardForSource(card SkillSourceCard) SkillSnapshotCard {
	return SkillSnapshotCard{
		Kind:               "source-pin",
		Name:               card.Name,
		Source:             "repo-local-skill-source",
		Path:               card.Path,
		Enabled:            true,
		Bytes:              card.Bytes,
		Lines:              card.Lines,
		SHA:                card.SHA,
		SkillMatched:       card.SkillMatched,
		SkillPath:          card.SkillPath,
		SkillSHA:           noneIfEmpty(card.SkillSHA),
		SourceKind:         card.SourceKind,
		SourceRefPresent:   card.SourceRefPresent,
		SourceRefSHA:       noneIfEmpty(card.SourceRefSHA),
		TrustLevel:         card.TrustLevel,
		InstallMode:        card.InstallMode,
		HashPinned:         card.HashPinned,
		HashMatched:        card.HashMatched,
		HashMismatched:     card.HashMismatched,
		RequiresApproval:   card.RequiresApproval,
		RemoteFetchAllowed: card.RemoteFetchAllowed,
		RiskFindings:       len(card.RiskFindings),
		RiskMaxSeverity:    skillSourceRiskMaxSeverity(card.RiskFindings),
		RiskCodes:          skillSourceRiskCodes(card.RiskFindings),
	}
}

func writeSkillSnapshotCards(b *strings.Builder, cards []SkillSnapshotCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- position=`%d` kind=`%s` name=`%s` source=`%s` path=`%s` enabled=`%t` prompt_visible=`%t` selected_for_this_turn=`%t` always=`%t` frontmatter=`%t` description_present=`%t` description_sha256_12=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` required_env=`%d` required_bins=`%d` missing_env=`%d` missing_bins=`%d` install_specs=`%d` install_bins=`%d` bundle_skill_refs=`%s` resolved_bundle_skills=`%s` missing_bundle_skills=`%s` instruction_present=`%t` instruction_sha256_12=`%s` skill_matched=`%t` skill_path=`%s` skill_sha256_12=`%s` source_kind=`%s` source_ref_present=`%t` source_ref_sha256_12=`%s` trust_level=`%s` install_mode=`%s` hash_pinned=`%t` hash_matched=`%t` hash_mismatched=`%t` requires_approval=`%t` remote_fetch_allowed=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			card.Position,
			card.Kind,
			inlineCode(card.Name),
			inlineCode(card.Source),
			card.Path,
			card.Enabled,
			card.PromptVisible,
			card.SelectedForThisTurn,
			card.Always,
			card.Frontmatter,
			card.DescriptionPresent,
			noneIfEmpty(card.DescriptionSHA),
			card.Bytes,
			card.Lines,
			noneIfEmpty(card.SHA),
			card.RequiredEnv,
			card.RequiredBins,
			card.MissingEnv,
			card.MissingBins,
			card.InstallSpecs,
			card.InstallBins,
			inlineListOrNone(card.BundleSkillRefs),
			inlineListOrNone(card.ResolvedBundleSkills),
			inlineListOrNone(card.MissingBundleSkills),
			card.InstructionPresent,
			noneIfEmpty(card.InstructionSHA),
			card.SkillMatched,
			noneIfEmpty(card.SkillPath),
			noneIfEmpty(card.SkillSHA),
			noneIfEmpty(card.SourceKind),
			card.SourceRefPresent,
			noneIfEmpty(card.SourceRefSHA),
			noneIfEmpty(card.TrustLevel),
			noneIfEmpty(card.InstallMode),
			card.HashPinned,
			card.HashMatched,
			card.HashMismatched,
			card.RequiresApproval,
			card.RemoteFetchAllowed,
			card.RiskFindings,
			noneIfEmpty(card.RiskMaxSeverity),
			inlineListOrNone(card.RiskCodes),
		)
	}
}

func (r *SkillSnapshotReport) addCard(card SkillSnapshotCard) {
	if card.SHA == "" {
		card.SHA = "none"
	}
	if card.DescriptionSHA == "" {
		card.DescriptionSHA = "none"
	}
	if card.InstructionSHA == "" {
		card.InstructionSHA = "none"
	}
	if card.SkillSHA == "" {
		card.SkillSHA = "none"
	}
	if card.SourceRefSHA == "" {
		card.SourceRefSHA = "none"
	}
	if card.RiskMaxSeverity == "" {
		card.RiskMaxSeverity = "none"
	}
	r.Cards = append(r.Cards, card)
	switch card.Kind {
	case "skill":
		r.SkillEntries++
	case "selected-skill":
		r.SelectedSkillEntries++
	case "bundle":
		r.BundleEntries++
	case "source-pin":
		r.SourcePinEntries++
	}
	if card.PromptVisible {
		r.PromptVisibleEntries++
	}
}

func skillSnapshotManifestHash(cards []SkillSnapshotCard) string {
	var b strings.Builder
	b.WriteString(skillSnapshotVersion)
	b.WriteByte('\n')
	for _, card := range cards {
		fmt.Fprintf(&b, "%03d|%s|%s|%s|%s|%t|%t|%t|%t|%t|%t|%s|%d|%d|%s|%d|%d|%d|%d|%d|%d|%s|%s|%s|%t|%s|%t|%s|%s|%s|%t|%s|%s|%s|%t|%t|%t|%t|%t|%d|%s|%s\n",
			card.Position,
			card.Kind,
			card.Name,
			card.Source,
			card.Path,
			card.Enabled,
			card.PromptVisible,
			card.SelectedForThisTurn,
			card.Always,
			card.Frontmatter,
			card.DescriptionPresent,
			card.DescriptionSHA,
			card.Bytes,
			card.Lines,
			card.SHA,
			card.RequiredEnv,
			card.RequiredBins,
			card.MissingEnv,
			card.MissingBins,
			card.InstallSpecs,
			card.InstallBins,
			strings.Join(card.BundleSkillRefs, ","),
			strings.Join(card.ResolvedBundleSkills, ","),
			strings.Join(card.MissingBundleSkills, ","),
			card.InstructionPresent,
			card.InstructionSHA,
			card.SkillMatched,
			card.SkillPath,
			card.SkillSHA,
			card.SourceKind,
			card.SourceRefPresent,
			card.SourceRefSHA,
			card.TrustLevel,
			card.InstallMode,
			card.HashPinned,
			card.HashMatched,
			card.HashMismatched,
			card.RequiresApproval,
			card.RemoteFetchAllowed,
			card.RiskFindings,
			card.RiskMaxSeverity,
			strings.Join(card.RiskCodes, ","),
		)
	}
	return shortDocumentHash(b.String())
}

func skillSnapshotStatus(statuses ...string) string {
	best := "ok"
	bestRank := skillSnapshotStatusRank(best)
	for _, status := range statuses {
		rank := skillSnapshotStatusRank(status)
		if rank > bestRank {
			best = status
			bestRank = rank
		}
	}
	return best
}

func skillSnapshotStatusRank(status string) int {
	switch status {
	case "error":
		return 4
	case "high":
		return 3
	case "warn":
		return 2
	case "ok":
		return 1
	default:
		return 0
	}
}

func skillSnapshotGate(status string) string {
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

func isSkillsSnapshotRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/skills" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return true
	default:
		return false
	}
}
