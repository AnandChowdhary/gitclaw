package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

func renderSoulCatalogReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSoulAnchorReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_catalog_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-authority-discovery")
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "soul-identity-memory-policy")
	fmt.Fprintf(&b, "- authority_model: `%s`\n", report.AuthorityModel)
	fmt.Fprintf(&b, "- profile_model: `%s`\n", "github-repo-profile")
	fmt.Fprintf(&b, "- cataloged_anchors: `%d`\n", report.AnchorCount)
	fmt.Fprintf(&b, "- loaded_anchors: `%d`\n", report.LoadedAnchors)
	fmt.Fprintf(&b, "- prompt_visible_anchors: `%d`\n", report.PromptVisibleAnchors)
	fmt.Fprintf(&b, "- required_anchors: `%d`\n", report.RequiredAnchors)
	fmt.Fprintf(&b, "- required_anchors_loaded: `%d`\n", report.RequiredAnchorsLoaded)
	fmt.Fprintf(&b, "- required_anchors_missing: `%d`\n", report.RequiredAnchorsMissing)
	fmt.Fprintf(&b, "- optional_anchors: `%d`\n", report.AnchorCount-report.RequiredAnchors)
	fmt.Fprintf(&b, "- optional_anchors_loaded: `%d`\n", report.OptionalAnchorsLoaded)
	fmt.Fprintf(&b, "- memory_note_anchors: `%d`\n", report.MemoryNoteAnchors)
	fmt.Fprintf(&b, "- authority_layers: `%d`\n", soulCatalogAuthorityLayerCount(report.Anchors))
	fmt.Fprintf(&b, "- authority_layer_names: `%s`\n", inlineList(soulCatalogAuthorityLayers(report.Anchors)))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_descriptions_included: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_soul_catalog_change: `%t`\n", true)
	writeSoulValidationSummary(&b, report.Validation)
	writeSoulRiskSummary(&b, report.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This compact catalog follows OpenClaw workspace-file and Hermes profile boundaries: high-authority context is visible as repo-reviewed metadata first, while raw soul, identity, user, memory, tool, heartbeat, issue, comment, prompt, description, and secret bodies stay out of deterministic reports.\n\n")

	b.WriteString("### Authority Catalog\n")
	if len(report.Anchors) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, anchor := range report.Anchors {
			writeSoulCatalogCard(&b, anchor)
		}
	}

	b.WriteString("\n### Catalog Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- external_registry_gate=`%s`\n", "not_configured")
	fmt.Fprintf(&b, "- profile_export_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- body_hash_gate=`%s`\n", "sha256_12")
	return strings.TrimSpace(b.String())
}

func writeSoulCatalogCard(b *strings.Builder, anchor SoulAnchor) {
	sha := anchor.SHA
	if sha == "" {
		sha = "none"
	}
	riskMax := anchor.RiskMaxSeverity
	if riskMax == "" {
		riskMax = "none"
	}
	fmt.Fprintf(
		b,
		"- name=`%s` path=`%s` category=`%s` authority=`%s` role=`%s` required=`%t` loaded=`%t` prompt_visible=`%t` load_mode=`%s` canonical=`%t` latest_memory_note=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` reason_codes=`%s`\n",
		anchor.Name,
		anchor.Path,
		anchor.Category,
		anchor.Authority,
		anchor.Role,
		anchor.Required,
		anchor.Loaded,
		anchor.PromptVisible,
		soulCatalogLoadMode(anchor),
		anchor.Canonical,
		anchor.LatestMemoryNote,
		anchor.Bytes,
		anchor.Lines,
		sha,
		anchor.RiskFindings,
		riskMax,
		inlineList(soulCatalogReasonCodes(anchor)),
	)
}

func soulCatalogLoadMode(anchor SoulAnchor) string {
	switch {
	case anchor.Required && anchor.Loaded:
		return "required-loaded"
	case anchor.Required:
		return "required-missing"
	case anchor.Loaded:
		return "optional-loaded"
	default:
		return "optional-missing"
	}
}

func soulCatalogReasonCodes(anchor SoulAnchor) []string {
	var reasons []string
	if anchor.Required {
		reasons = append(reasons, "required")
	} else {
		reasons = append(reasons, "optional")
	}
	if anchor.Loaded {
		reasons = append(reasons, "loaded")
	} else {
		reasons = append(reasons, "not_loaded")
	}
	if anchor.PromptVisible {
		reasons = append(reasons, "prompt_visible")
	} else {
		reasons = append(reasons, "not_prompt_visible")
	}
	if anchor.Canonical {
		reasons = append(reasons, "canonical")
	}
	if anchor.LatestMemoryNote {
		reasons = append(reasons, "latest_memory_note")
	}
	if anchor.AtContextLimit {
		reasons = append(reasons, "at_context_limit")
	}
	if anchor.RiskFindings > 0 {
		reasons = append(reasons, "risk_findings")
	}
	return uniqueSortedStrings(reasons)
}

func soulCatalogAuthorityLayerCount(anchors []SoulAnchor) int {
	layers := map[string]bool{}
	for _, anchor := range anchors {
		authority := strings.TrimSpace(anchor.Authority)
		if authority != "" {
			layers[authority] = true
		}
	}
	return len(layers)
}

func soulCatalogAuthorityLayers(anchors []SoulAnchor) []string {
	layers := map[string]bool{}
	for _, anchor := range anchors {
		authority := strings.TrimSpace(anchor.Authority)
		if authority != "" {
			layers[authority] = true
		}
	}
	out := make([]string, 0, len(layers))
	for layer := range layers {
		out = append(out, layer)
	}
	sort.Strings(out)
	return out
}
