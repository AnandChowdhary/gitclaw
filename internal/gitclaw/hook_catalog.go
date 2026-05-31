package gitclaw

import (
	"fmt"
	"strings"
)

type hookCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type hookCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderHookCatalogCLIReport(cfg Config) string {
	return renderHookCatalogReport(Event{}, cfg, false)
}

func renderHookCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectHookSurface(cfg.Workdir)
	findings := hookFindings(surface)
	risk := BuildHookRiskReport(cfg)
	provenance := BuildHookProvenanceReport(cfg)
	entries := hookCatalogEntries()
	layers := hookCatalogLayers(surface, provenance)

	var b strings.Builder
	b.WriteString("## GitClaw Hooks Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_hooks_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- hooks_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_hook_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- hook_catalog_status: `%s`\n", hookCatalogStatus(surface, findings, risk, provenance))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-event-hook-discovery")
	fmt.Fprintf(&b, "- catalog_scope: `%s`\n", "hook-policy-specs-events-provenance")
	fmt.Fprintf(&b, "- hook_surface_model: `%s`\n", "repo-reviewed-hooks-plus-github-actions")
	fmt.Fprintf(&b, "- hooks_status: `%s`\n", hookStatus(surface, findings))
	fmt.Fprintf(&b, "- hook_risk_status: `%s`\n", risk.Status)
	fmt.Fprintf(&b, "- hook_provenance_status: `%s`\n", provenance.Status)
	fmt.Fprintf(&b, "- hook_policy_path: `%s`\n", hookPolicyPath)
	fmt.Fprintf(&b, "- hook_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- hook_policy_loaded_for_model: `%t`\n", hookPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- hook_specs_dir: `%s`\n", hookSpecsDir)
	fmt.Fprintf(&b, "- hook_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- hook_specs_with_frontmatter: `%d`\n", hookSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- hook_events: `%d`\n", hookEventCount(surface.Specs))
	fmt.Fprintf(&b, "- hook_specs_requiring_approval: `%d`\n", hookSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- hook_specs_audit_only: `%d`\n", hookSpecsAuditOnly(surface.Specs))
	fmt.Fprintf(&b, "- executable_handlers_present: `%d`\n", len(surface.ExecutableHandlers))
	fmt.Fprintf(&b, "- git_tracked_hook_surfaces: `%d`\n", provenance.GitTrackedSurfaces)
	fmt.Fprintf(&b, "- working_tree_dirty_hook_surfaces: `%d`\n", provenance.WorkingTreeDirtySurfaces)
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- hook_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- hook_execution_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- hook_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- handler_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- provider_payload_ingest_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_hook_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_handler_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_provider_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_hook_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps GitClaw's declarative hook surface inspired by OpenClaw's hook discovery and Hermes' hook testing posture: event bindings remain repo-reviewed metadata, executable handlers stay disabled, provider payloads are not ingested, and reports expose only commands, layers, counts, hashes, and gates.\n\n")

	b.WriteString("### Catalog Entries\n")
	for _, entry := range entries {
		fmt.Fprintf(&b, "- command=`%s` issue_intent=`%s` local_command=`%s` execution=`%s` gate=`%s` raw_bodies_included=`%t` mutation_allowed=`%t`\n",
			entry.Name,
			entry.IssueIntent,
			entry.LocalCommand,
			entry.Execution,
			entry.Gate,
			entry.RawBodies,
			entry.MutationAllowed,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Hook Layers\n")
	for _, layer := range layers {
		fmt.Fprintf(&b, "- layer=`%s` store=`%s` source=`%s` gate=`%s` count=`%d` raw_bodies_included=`%t`\n",
			layer.Name,
			layer.Store,
			layer.Source,
			layer.Gate,
			layer.Count,
			layer.RawBodies,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Catalog Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", hookCatalogGate(hookStatus(surface, findings)))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", hookCatalogGate(risk.Status))
	fmt.Fprintf(&b, "- provenance_gate=`%s`\n", hookCatalogGate(provenance.Status))
	b.WriteString("- context_gate=`hook-policy-loaded-before-model`\n")
	b.WriteString("- event_gate=`declarative-events-only`\n")
	b.WriteString("- approval_gate=`side-effects-require-approval`\n")
	b.WriteString("- handler_gate=`disabled-not-executed`\n")
	b.WriteString("- provider_payload_gate=`not-ingested`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func hookCatalogEntries() []hookCatalogEntry {
	return []hookCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /hooks catalog", LocalCommand: "gitclaw hooks catalog", Execution: "metadata-only", Gate: "body-free-hook-command-map"},
		{Name: "list", IssueIntent: "@gitclaw /hooks", LocalCommand: "gitclaw hooks list", Execution: "metadata-only", Gate: "hook-policy-and-spec-inventory"},
		{Name: "verify", IssueIntent: "@gitclaw /hooks verify", LocalCommand: "gitclaw hooks verify", Execution: "metadata-only", Gate: "hook-policy-and-spec-inventory"},
		{Name: "risk", IssueIntent: "@gitclaw /hooks risk", LocalCommand: "gitclaw hooks risk", Execution: "risk-audit", Gate: "hook-safety-risk-audit"},
		{Name: "provenance", IssueIntent: "@gitclaw /hooks provenance", LocalCommand: "gitclaw hooks provenance", Execution: "git-history-audit", Gate: "body-free-hook-git-provenance"},
	}
}

func hookCatalogLayers(surface hookSurface, provenance HookProvenanceReport) []hookCatalogLayer {
	return []hookCatalogLayer{
		{Name: "policy", Store: hookPolicyPath, Source: "repo-context-allowlist", Gate: "loaded-before-model", Count: boolCount(surface.Policy.Present)},
		{Name: "specs", Store: hookSpecsDir + "/*.md", Source: "repo-reviewed-hook-specs", Gate: "frontmatter-and-hash-metadata", Count: len(surface.Specs)},
		{Name: "events", Store: "hook frontmatter events", Source: "declarative-hook-specs", Gate: "no-live-event-listener", Count: hookEventCount(surface.Specs)},
		{Name: "approval", Store: "requires_approval frontmatter", Source: "declarative-hook-specs", Gate: "side-effects-require-approval", Count: hookSpecsRequiringApproval(surface.Specs)},
		{Name: "handlers", Store: "executable-looking hook files", Source: "ignored-hook-files", Gate: "ignored-not-executed", Count: len(surface.ExecutableHandlers)},
		{Name: "provenance", Store: "git history", Source: "tracked-hook-files", Gate: "commit-subject-hashes-only", Count: provenance.ProvenanceSurfaces},
		{Name: "provider-payloads", Store: "unsupported external payloads", Source: "explicit-negative-capability", Gate: "body-free-reporting", Count: 0},
	}
}

func isHookCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/hooks" && command != "/hook" {
		return false
	}
	return strings.EqualFold(fields[1], "catalog") ||
		strings.EqualFold(fields[1], "commands") ||
		strings.EqualFold(fields[1], "index") ||
		strings.EqualFold(fields[1], "map")
}

func hookCatalogStatus(surface hookSurface, findings []hookFinding, risk HookRiskReport, provenance HookProvenanceReport) string {
	statuses := []string{hookStatus(surface, findings), risk.Status, provenance.Status}
	status := "ok"
	for _, value := range statuses {
		switch value {
		case "high", "error":
			return "high"
		case "warn", "dirty":
			status = "warn"
		}
	}
	return status
}

func hookCatalogGate(status string) string {
	switch status {
	case "ok":
		return "pass"
	case "warn", "dirty":
		return "warn"
	case "high", "error":
		return "block"
	default:
		return "unknown"
	}
}
