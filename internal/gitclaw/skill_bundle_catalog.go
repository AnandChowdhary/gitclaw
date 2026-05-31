package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillBundleCatalogReport struct {
	Status                             string
	AvailableBundles                   int
	CatalogedEntries                   int
	SelectedBundles                    int
	AvailableSkills                    int
	BundleSkillRefs                    int
	ResolvedBundleSkills               int
	MissingBundleSkills                int
	BundlesWithInstruction             int
	PromptVisibleInstructions          int
	MetadataOnlyInstructions           int
	EntriesWithRiskFindings            int
	BundleRiskFindings                 int
	ExternalRegistryVerification       string
	InstallerScriptsRun                bool
	AgentAuthoredBundleMutationAllowed bool
	RawBundleBodiesIncluded            bool
	RawBundleInstructionsIncluded      bool
	RawSkillBodiesIncluded             bool
	RawIssueBodiesIncluded             bool
	RawCommentBodiesIncluded           bool
	RawPromptBodiesIncluded            bool
	CredentialValuesIncluded           bool
	LLME2ERequiredAfterBundleCatalog   bool
	Entries                            []SkillBundleCatalogEntry
}

type SkillBundleCatalogEntry struct {
	Position            int
	Name                string
	Path                string
	OrchestrationLayer  string
	Source              string
	Role                string
	SelectedForThisTurn bool
	PromptVisible       bool
	InstructionPresent  bool
	InstructionLoadMode string
	InstructionSHA      string
	Skills              []string
	ResolvedSkills      []string
	MissingSkills       []string
	SkillRefs           int
	ResolvedSkillRefs   int
	MissingSkillRefs    int
	Bytes               int
	Lines               int
	SHA                 string
	ParseError          bool
	RiskFindings        int
	RiskMaxSeverity     string
	RiskCodes           []string
	ReasonCodes         []string
}

func BuildSkillBundleCatalogReport(repoContext RepoContext) SkillBundleCatalogReport {
	risk := BuildSkillBundleRiskReport(repoContext.SkillBundles)
	report := SkillBundleCatalogReport{
		Status:                             risk.Status,
		AvailableBundles:                   len(repoContext.SkillBundles),
		SelectedBundles:                    selectedSkillBundleCount(repoContext.SkillBundles),
		AvailableSkills:                    availableSkillCount(repoContext),
		BundleSkillRefs:                    bundleSkillRefCount(repoContext.SkillBundles),
		ResolvedBundleSkills:               resolvedBundleSkillCount(repoContext.SkillBundles),
		MissingBundleSkills:                missingBundleSkillCount(repoContext.SkillBundles),
		BundlesWithInstruction:             bundlesWithInstructionCount(repoContext.SkillBundles),
		BundleRiskFindings:                 len(risk.Findings),
		ExternalRegistryVerification:       "not_configured",
		InstallerScriptsRun:                false,
		AgentAuthoredBundleMutationAllowed: false,
		RawBundleBodiesIncluded:            false,
		RawBundleInstructionsIncluded:      false,
		RawSkillBodiesIncluded:             false,
		RawIssueBodiesIncluded:             false,
		RawCommentBodiesIncluded:           false,
		RawPromptBodiesIncluded:            false,
		CredentialValuesIncluded:           false,
		LLME2ERequiredAfterBundleCatalog:   true,
	}
	bundles := append([]SkillBundleSummary(nil), repoContext.SkillBundles...)
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Path < bundles[j].Path })
	for idx, bundle := range bundles {
		findings := skillBundleFindingsForPath(bundle.Path, risk.Findings)
		entry := SkillBundleCatalogEntry{
			Position:            idx + 1,
			Name:                bundle.Name,
			Path:                bundle.Path,
			OrchestrationLayer:  "procedural-memory",
			Source:              "repo-local-yaml",
			Role:                skillBundleCatalogRole(bundle),
			SelectedForThisTurn: bundle.Selected,
			PromptVisible:       bundle.Selected,
			InstructionPresent:  bundle.InstructionPresent,
			InstructionLoadMode: skillBundleCatalogInstructionLoadMode(bundle),
			InstructionSHA:      bundle.InstructionSHA,
			Skills:              append([]string(nil), bundle.Skills...),
			ResolvedSkills:      append([]string(nil), bundle.ResolvedSkills...),
			MissingSkills:       append([]string(nil), bundle.MissingSkills...),
			SkillRefs:           len(bundle.Skills),
			ResolvedSkillRefs:   len(bundle.ResolvedSkills),
			MissingSkillRefs:    len(bundle.MissingSkills),
			Bytes:               bundle.Bytes,
			Lines:               bundle.Lines,
			SHA:                 bundle.SHA,
			ParseError:          bundle.ParseError != "",
			RiskFindings:        len(findings),
			RiskMaxSeverity:     skillBundleRiskMaxSeverity(findings),
			RiskCodes:           skillBundleRiskCodes(findings),
		}
		entry.ReasonCodes = skillBundleCatalogReasonCodes(entry)
		report.Entries = append(report.Entries, entry)
		report.CatalogedEntries++
		if entry.InstructionPresent && entry.PromptVisible {
			report.PromptVisibleInstructions++
		} else if entry.InstructionPresent {
			report.MetadataOnlyInstructions++
		}
		if entry.RiskFindings > 0 {
			report.EntriesWithRiskFindings++
		}
	}
	return report
}

func RenderSkillBundleCatalogCLIReport(repoContext RepoContext) string {
	return renderSkillBundleCatalogReport(Event{}, repoContext, false)
}

func renderSkillBundleCatalogReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillBundleCatalogReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Bundle Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- bundle_catalog_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-bundle-orchestration-discovery")
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "repo-local-skill-bundles-procedural-memory")
	fmt.Fprintf(&b, "- bundle_model: `%s`\n", "repo-local-reviewed-yaml")
	fmt.Fprintf(&b, "- hermes_bundle_boundary: `%s`\n", "task-profile-loads-existing-skills")
	fmt.Fprintf(&b, "- openclaw_skill_boundary: `%s`\n", "skills-install-separately-review-first")
	fmt.Fprintf(&b, "- available_bundles: `%d`\n", report.AvailableBundles)
	fmt.Fprintf(&b, "- cataloged_entries: `%d`\n", report.CatalogedEntries)
	fmt.Fprintf(&b, "- selected_bundles: `%d`\n", report.SelectedBundles)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- bundle_skill_refs: `%d`\n", report.BundleSkillRefs)
	fmt.Fprintf(&b, "- resolved_bundle_skills: `%d`\n", report.ResolvedBundleSkills)
	fmt.Fprintf(&b, "- missing_bundle_skills: `%d`\n", report.MissingBundleSkills)
	fmt.Fprintf(&b, "- bundles_with_instruction: `%d`\n", report.BundlesWithInstruction)
	fmt.Fprintf(&b, "- prompt_visible_instructions: `%d`\n", report.PromptVisibleInstructions)
	fmt.Fprintf(&b, "- metadata_only_instructions: `%d`\n", report.MetadataOnlyInstructions)
	fmt.Fprintf(&b, "- entries_with_risk_findings: `%d`\n", report.EntriesWithRiskFindings)
	fmt.Fprintf(&b, "- bundle_risk_findings: `%d`\n", report.BundleRiskFindings)
	fmt.Fprintf(&b, "- external_registry_verification: `%s`\n", report.ExternalRegistryVerification)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- agent_authored_bundle_mutation_allowed: `%t`\n", report.AgentAuthoredBundleMutationAllowed)
	fmt.Fprintf(&b, "- raw_bundle_bodies_included: `%t`\n", report.RawBundleBodiesIncluded)
	fmt.Fprintf(&b, "- raw_bundle_instructions_included: `%t`\n", report.RawBundleInstructionsIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_bundle_catalog_change: `%t`\n", report.LLME2ERequiredAfterBundleCatalog)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This compact catalog treats skill bundles as Hermes-style task profiles over existing reviewed skills. It reports procedural-memory orchestration metadata, selected/load state, skill-ref resolution, instruction hashes, risk rollups, and gates only; raw bundle YAML, bundle instructions, skill bodies, issue bodies, comments, prompts, credentials, and secret values are not included.\n\n")

	b.WriteString("### Bundle Catalog Entries\n")
	if len(report.Entries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, entry := range report.Entries {
			writeSkillBundleCatalogEntry(&b, entry)
		}
	}

	b.WriteString("\n### Catalog Gates\n")
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Status))
	fmt.Fprintf(&b, "- skill_ref_gate=`%s`\n", skillBundleCatalogSkillRefGate(report))
	fmt.Fprintf(&b, "- instruction_body_gate=`%s`\n", "sha256_12")
	fmt.Fprintf(&b, "- external_registry_gate=`%s`\n", report.ExternalRegistryVerification)
	fmt.Fprintf(&b, "- installer_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- agent_authored_mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")
	return strings.TrimSpace(b.String())
}

func writeSkillBundleCatalogEntry(b *strings.Builder, entry SkillBundleCatalogEntry) {
	fmt.Fprintf(
		b,
		"- position=`%d` bundle_name=`%s` path=`%s` orchestration_layer=`%s` source=`%s` role=`%s` selected_for_this_turn=`%t` prompt_visible=`%t` instruction=`%t` instruction_load_mode=`%s` instruction_sha256_12=`%s` skills=`%s` resolved_skills=`%s` missing_skills=`%s` skill_refs=`%d` resolved_skill_refs=`%d` missing_skill_refs=`%d` bytes=`%d` lines=`%d` sha256_12=`%s` parse_error=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` reason_codes=`%s`\n",
		entry.Position,
		inlineCode(entry.Name),
		entry.Path,
		entry.OrchestrationLayer,
		entry.Source,
		entry.Role,
		entry.SelectedForThisTurn,
		entry.PromptVisible,
		entry.InstructionPresent,
		entry.InstructionLoadMode,
		entry.InstructionSHA,
		inlineList(entry.Skills),
		inlineList(entry.ResolvedSkills),
		inlineList(entry.MissingSkills),
		entry.SkillRefs,
		entry.ResolvedSkillRefs,
		entry.MissingSkillRefs,
		entry.Bytes,
		entry.Lines,
		entry.SHA,
		entry.ParseError,
		entry.RiskFindings,
		entry.RiskMaxSeverity,
		inlineListOrNone(entry.RiskCodes),
		skillBundleCatalogReasonCodeList(entry.ReasonCodes),
	)
}

func skillBundleCatalogRole(bundle SkillBundleSummary) string {
	if bundle.Selected {
		return "selected-task-profile"
	}
	return "available-task-profile"
}

func skillBundleCatalogInstructionLoadMode(bundle SkillBundleSummary) string {
	switch {
	case !bundle.InstructionPresent:
		return "none"
	case bundle.Selected:
		return "prompt-visible"
	default:
		return "metadata-only"
	}
}

func skillBundleCatalogReasonCodes(entry SkillBundleCatalogEntry) []string {
	reasons := []string{
		strings.ReplaceAll(entry.OrchestrationLayer, "-", "_"),
		strings.ReplaceAll(entry.Source, "-", "_"),
		strings.ReplaceAll(entry.Role, "-", "_"),
	}
	if entry.SelectedForThisTurn {
		reasons = append(reasons, "selected")
	} else {
		reasons = append(reasons, "not_selected")
	}
	if entry.PromptVisible {
		reasons = append(reasons, "prompt_visible")
	} else {
		reasons = append(reasons, "not_prompt_visible")
	}
	if entry.InstructionPresent {
		reasons = append(reasons, "instruction_present")
	} else {
		reasons = append(reasons, "instruction_absent")
	}
	if entry.InstructionLoadMode != "" {
		reasons = append(reasons, strings.ReplaceAll(entry.InstructionLoadMode, "-", "_"))
	}
	if entry.MissingSkillRefs > 0 {
		reasons = append(reasons, "missing_skill_refs")
	} else {
		reasons = append(reasons, "all_skill_refs_resolved")
	}
	if entry.ParseError {
		reasons = append(reasons, "parse_error")
	} else {
		reasons = append(reasons, "parse_ok")
	}
	if entry.RiskFindings > 0 {
		reasons = append(reasons, "risk_findings")
	} else {
		reasons = append(reasons, "no_risk_findings")
	}
	return uniqueSortedStrings(reasons)
}

func skillBundleCatalogReasonCodeList(reasons []string) string {
	if len(reasons) == 0 {
		return "none"
	}
	return strings.Join(reasons, ", ")
}

func skillBundleCatalogSkillRefGate(report SkillBundleCatalogReport) string {
	if report.MissingBundleSkills > 0 {
		return "warn"
	}
	return "pass"
}
