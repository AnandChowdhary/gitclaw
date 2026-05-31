package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SoulAnchorReport struct {
	Status                    string
	Validation                SoulValidationReport
	Risk                      SoulRiskReport
	AuthorityModel            string
	AnchorCount               int
	LoadedAnchors             int
	RequiredAnchors           int
	RequiredAnchorsLoaded     int
	RequiredAnchorsMissing    int
	OptionalAnchorsLoaded     int
	MemoryNoteAnchors         int
	PromptVisibleAnchors      int
	RawBodiesIncluded         bool
	SoulWritesAllowed         bool
	LLME2ERequiredAfterChange bool
	Anchors                   []SoulAnchor
}

type SoulAnchor struct {
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
	Bytes            int
	Lines            int
	SHA              string
	RiskFindings     int
	RiskMaxSeverity  string
	RiskCodes        []string
	AtContextLimit   bool
	Frontmatter      bool
	Description      bool
}

type soulAnchorSpec struct {
	Name      string
	Path      string
	Role      string
	Authority string
	Required  bool
}

var soulAnchorSpecs = []soulAnchorSpec{
	{Name: "agent-instructions", Path: "AGENTS.md", Role: "operating-instructions", Authority: "workspace", Required: false},
	{Name: "soul", Path: ".gitclaw/SOUL.md", Role: "persona-boundaries", Authority: "core", Required: true},
	{Name: "identity", Path: ".gitclaw/IDENTITY.md", Role: "agent-identity", Authority: "core", Required: true},
	{Name: "user", Path: ".gitclaw/USER.md", Role: "user-profile", Authority: "core", Required: true},
	{Name: "tools", Path: ".gitclaw/TOOLS.md", Role: "tool-guidance", Authority: "operational", Required: true},
	{Name: "memory", Path: ".gitclaw/MEMORY.md", Role: "curated-memory", Authority: "memory", Required: true},
	{Name: "heartbeat", Path: ".gitclaw/HEARTBEAT.md", Role: "proactive-checklist", Authority: "proactive", Required: true},
	{Name: "standing-orders", Path: ".gitclaw/STANDING_ORDERS.md", Role: "durable-operating-authority", Authority: "policy", Required: false},
	{Name: "repo-agents", Path: ".gitclaw/AGENTS.md", Role: "repo-agent-policy", Authority: "policy", Required: false},
	{Name: "hooks", Path: ".gitclaw/HOOKS.md", Role: "event-hook-policy", Authority: "automation", Required: false},
	{Name: "plugins", Path: ".gitclaw/PLUGINS.md", Role: "plugin-policy", Authority: "extension", Required: false},
	{Name: "tasks", Path: ".gitclaw/TASKS.md", Role: "task-policy", Authority: "workflow", Required: false},
	{Name: "nodes", Path: ".gitclaw/NODES.md", Role: "node-policy", Authority: "runtime", Required: false},
	{Name: "artifacts", Path: ".gitclaw/ARTIFACTS.md", Role: "artifact-policy", Authority: "workflow", Required: false},
	{Name: "diffs", Path: ".gitclaw/DIFFS.md", Role: "diff-policy", Authority: "workflow", Required: false},
	{Name: "workspace", Path: ".gitclaw/WORKSPACE.md", Role: "workspace-policy", Authority: "workspace", Required: false},
}

func BuildSoulAnchorReport(repoContext RepoContext) SoulAnchorReport {
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	report := SoulAnchorReport{
		Status:                    validation.Status,
		Validation:                validation,
		Risk:                      risk,
		AuthorityModel:            "repo-local-workspace-files",
		RequiredAnchors:           validation.RequiredFiles,
		RawBodiesIncluded:         false,
		SoulWritesAllowed:         false,
		LLME2ERequiredAfterChange: true,
	}
	if report.Status != "error" {
		switch risk.Status {
		case "high":
			report.Status = "high"
		case "warn":
			if report.Status == "ok" {
				report.Status = "warn"
			}
		}
	}

	docsByPath := map[string]ContextDocument{}
	for _, doc := range repoContext.Documents {
		docsByPath[doc.Path] = doc
	}
	seen := map[string]bool{}
	latestMemory := latestMemoryAnchorPath(repoContext.Documents)
	for _, spec := range soulAnchorSpecs {
		doc, loaded := docsByPath[spec.Path]
		anchor := buildSoulAnchor(spec, doc, loaded, latestMemory)
		report.addAnchor(anchor)
		seen[spec.Path] = true
	}
	var memoryDocs []ContextDocument
	for _, doc := range repoContext.Documents {
		if seen[doc.Path] || !isSoulMemoryNote(doc.Path) {
			continue
		}
		memoryDocs = append(memoryDocs, doc)
	}
	sort.Slice(memoryDocs, func(i, j int) bool { return memoryDocs[i].Path < memoryDocs[j].Path })
	for _, doc := range memoryDocs {
		spec := soulAnchorSpec{
			Name:      "memory-note",
			Path:      doc.Path,
			Role:      "working-memory-note",
			Authority: "memory",
		}
		report.addAnchor(buildSoulAnchor(spec, doc, true, latestMemory))
	}
	return report
}

func RenderSoulAnchorsReport(repoContext RepoContext) string {
	return renderSoulAnchorsReport(Event{}, repoContext, false)
}

func renderSoulAnchorsReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSoulAnchorReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Anchors Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_anchors_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- authority_model: `%s`\n", report.AuthorityModel)
	fmt.Fprintf(&b, "- anchor_count: `%d`\n", report.AnchorCount)
	fmt.Fprintf(&b, "- loaded_anchors: `%d`\n", report.LoadedAnchors)
	fmt.Fprintf(&b, "- required_anchors: `%d`\n", report.RequiredAnchors)
	fmt.Fprintf(&b, "- required_anchors_loaded: `%d`\n", report.RequiredAnchorsLoaded)
	fmt.Fprintf(&b, "- required_anchors_missing: `%d`\n", report.RequiredAnchorsMissing)
	fmt.Fprintf(&b, "- optional_anchors_loaded: `%d`\n", report.OptionalAnchorsLoaded)
	fmt.Fprintf(&b, "- memory_note_anchors: `%d`\n", report.MemoryNoteAnchors)
	fmt.Fprintf(&b, "- prompt_visible_anchors: `%d`\n", report.PromptVisibleAnchors)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", report.SoulWritesAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_soul_anchors_change: `%t`\n", report.LLME2ERequiredAfterChange)
	writeSoulValidationSummary(&b, report.Validation)
	writeSoulRiskSummary(&b, report.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps GitClaw's high-authority repo-local context hierarchy inspired by OpenClaw workspace files and Hermes profiles. It reports loaded anchors, roles, authority layers, paths, counts, hashes, validation gates, and risk gates only; raw soul, identity, user, memory, tool, heartbeat, issue, comment, prompt, and secret bodies are not included.\n\n")

	b.WriteString("### Authority Anchors\n")
	if len(report.Anchors) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, anchor := range report.Anchors {
			writeSoulAnchorCard(&b, anchor)
		}
	}

	b.WriteString("\n### Anchor Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- external_registry_gate=`%s`\n", "not_configured")
	fmt.Fprintf(&b, "- profile_export_gate=`%s`\n", "not_configured")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	return strings.TrimSpace(b.String())
}

func isSoulAnchorsRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/soul" && (strings.EqualFold(fields[1], "anchors") || strings.EqualFold(fields[1], "anchor") || strings.EqualFold(fields[1], "authority") || strings.EqualFold(fields[1], "map"))
}

func buildSoulAnchor(spec soulAnchorSpec, doc ContextDocument, loaded bool, latestMemory string) SoulAnchor {
	anchor := SoulAnchor{
		Name:          spec.Name,
		Path:          spec.Path,
		Category:      soulDocumentCategory(spec.Path),
		Role:          spec.Role,
		Authority:     spec.Authority,
		Source:        soulAnchorSource(spec.Path),
		Required:      spec.Required,
		Loaded:        loaded,
		PromptVisible: loaded,
		Canonical:     soulAnchorCanonical(spec.Path),
	}
	if !loaded {
		return anchor
	}
	findings := scanSoulRiskFindings(doc.Path, doc.Body)
	anchor.Bytes = len(doc.Body)
	anchor.Lines = lineCount(doc.Body)
	anchor.SHA = shortDocumentHash(doc.Body)
	anchor.RiskFindings = len(findings)
	anchor.RiskMaxSeverity = soulRiskMaxSeverity(findings)
	anchor.RiskCodes = soulRiskCodes(findings)
	anchor.AtContextLimit = len(doc.Body) >= maxContextDocumentBytes
	anchor.LatestMemoryNote = doc.Path == latestMemory
	if fm, ok := frontmatter(doc.Body); ok {
		anchor.Frontmatter = true
		anchor.Description = strings.TrimSpace(frontmatterValue(fm, "description")) != ""
	}
	return anchor
}

func (r *SoulAnchorReport) addAnchor(anchor SoulAnchor) {
	r.Anchors = append(r.Anchors, anchor)
	r.AnchorCount++
	if anchor.Loaded {
		r.LoadedAnchors++
		if anchor.Required {
			r.RequiredAnchorsLoaded++
		} else {
			r.OptionalAnchorsLoaded++
		}
	}
	if anchor.Required && !anchor.Loaded {
		r.RequiredAnchorsMissing++
	}
	if anchor.PromptVisible {
		r.PromptVisibleAnchors++
	}
	if anchor.Category == "memory-note" {
		r.MemoryNoteAnchors++
	}
}

func writeSoulAnchorCard(b *strings.Builder, anchor SoulAnchor) {
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
		"- name=`%s` path=`%s` category=`%s` role=`%s` authority=`%s` source=`%s` required=`%t` loaded=`%t` prompt_visible=`%t` canonical=`%t` latest_memory_note=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` at_context_limit=`%t` frontmatter=`%t` description=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
		anchor.Name,
		anchor.Path,
		anchor.Category,
		anchor.Role,
		anchor.Authority,
		anchor.Source,
		anchor.Required,
		anchor.Loaded,
		anchor.PromptVisible,
		anchor.Canonical,
		anchor.LatestMemoryNote,
		anchor.Bytes,
		anchor.Lines,
		sha,
		anchor.AtContextLimit,
		anchor.Frontmatter,
		anchor.Description,
		anchor.RiskFindings,
		riskMax,
		inlineListOrNone(anchor.RiskCodes),
	)
}

func latestMemoryAnchorPath(docs []ContextDocument) string {
	var paths []string
	for _, doc := range docs {
		if isSoulMemoryNote(doc.Path) {
			paths = append(paths, doc.Path)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return ""
	}
	return paths[len(paths)-1]
}

func soulAnchorSource(path string) string {
	if path == "AGENTS.md" || strings.HasPrefix(path, ".github/") {
		return "repo-root"
	}
	return soulTrustSource(path)
}

func soulAnchorCanonical(path string) bool {
	if path == "AGENTS.md" {
		return true
	}
	for _, spec := range soulAnchorSpecs {
		if path == spec.Path {
			return true
		}
	}
	return soulInfoCanonicalPath(path)
}

func soulAnchorGate(status string) string {
	switch status {
	case "ok":
		return "pass"
	case "warn":
		return "warn"
	case "high":
		return "high"
	case "error":
		return "fail"
	default:
		if status == "" {
			return "unknown"
		}
		return status
	}
}
