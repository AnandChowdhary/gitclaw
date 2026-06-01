package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const researchSnapshotDate = "2026-06-01"

var researchDocumentPaths = []string{
	"docs/research-openclaw-hermes-landscape.md",
	"docs/spec-github-native-gitclaw.md",
	"README.md",
}

type researchSource struct {
	ID       string
	System   string
	Kind     string
	URL      string
	Pattern  string
	Decision string
}

type researchPattern struct {
	Name     string
	Upstream string
	Surface  string
	Status   string
	Gate     string
}

type researchRejection struct {
	Surface  string
	Upstream string
	Decision string
	Gate     string
}

type researchSurface struct {
	Documents        []configSurfaceFile
	DocumentsPresent int
	Followups        int
}

func IsResearchReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/research" || command == "/landscape"
}

func RenderResearchReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderResearchReport(ev, cfg, repoContext, true)
}

func RenderResearchCLIReport(cfg Config, repoContext RepoContext) string {
	return renderResearchReport(Event{}, cfg, repoContext, false)
}

func renderResearchReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	surface := inspectResearchSurface(cfg.Workdir)
	sources := researchSources()
	patterns := researchPatterns()
	rejections := researchRejections()

	var b strings.Builder
	b.WriteString("## GitClaw Research Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_research_command: `%s`\n", researchRequestedCommand(ev, cfg))
		fmt.Fprintf(&b, "- research_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "repo_local_research_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- research_catalog_status: `%s`\n", researchStatus(surface))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "primary-source-to-repo-native-design-map")
	fmt.Fprintf(&b, "- research_scope: `%s`\n", "openclaw, hermes-agent, nano-mini-claw-variants")
	fmt.Fprintf(&b, "- source_snapshot_date: `%s`\n", researchSnapshotDate)
	fmt.Fprintf(&b, "- reviewed_sources: `%d`\n", len(sources))
	fmt.Fprintf(&b, "- primary_sources: `%d`\n", countResearchSourcesByKind(sources, "primary"))
	fmt.Fprintf(&b, "- official_docs_sources: `%d`\n", countResearchSourcesByKind(sources, "official-docs"))
	fmt.Fprintf(&b, "- official_repo_sources: `%d`\n", countResearchSourcesByKind(sources, "primary-repo"))
	fmt.Fprintf(&b, "- local_research_docs: `%d`\n", len(surface.Documents))
	fmt.Fprintf(&b, "- local_research_docs_present: `%d`\n", surface.DocumentsPresent)
	fmt.Fprintf(&b, "- research_followups_indexed: `%d`\n", surface.Followups)
	fmt.Fprintf(&b, "- implemented_patterns: `%d`\n", countResearchPatternsByStatus(patterns, "implemented"))
	fmt.Fprintf(&b, "- adapted_patterns: `%d`\n", countResearchPatternsByStatus(patterns, "adapted"))
	fmt.Fprintf(&b, "- rejected_patterns: `%d`\n", len(rejections))
	fmt.Fprintf(&b, "- command_catalog_entries: `%d`\n", len(commandCatalog))
	fmt.Fprintf(&b, "- e2e_scripts_indexed: `%d`\n", inspectDoctorE2ESurface(cfg.Workdir).ScriptCount)
	fmt.Fprintf(&b, "- repo_context_documents_loaded: `%d`\n", len(repoContext.Documents))
	fmt.Fprintf(&b, "- repo_local_skills: `%d`\n", len(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- repo_local_skill_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- source_fetch_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- live_source_browse_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_research_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_research_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog turns the OpenClaw/Hermes research notes into an issue-visible design map. It reports reviewed source IDs, URLs, local research-file hashes, adopted/rejected pattern coverage, and gates only; raw research notes, source bodies, issue bodies, comments, prompts, tool outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Local Research Files\n")
	for _, doc := range surface.Documents {
		writeConfigSurfaceFile(&b, doc)
	}
	b.WriteByte('\n')

	b.WriteString("### Reviewed Sources\n")
	for _, source := range sources {
		fmt.Fprintf(&b, "- source_id=`%s` system=`%s` kind=`%s` url=`%s` pattern=`%s` decision=`%s`\n",
			source.ID,
			source.System,
			source.Kind,
			source.URL,
			source.Pattern,
			source.Decision,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Pattern Coverage\n")
	for _, pattern := range patterns {
		fmt.Fprintf(&b, "- pattern=`%s` upstream=`%s` gitclaw_surface=`%s` status=`%s` gate=`%s`\n",
			pattern.Name,
			pattern.Upstream,
			pattern.Surface,
			pattern.Status,
			pattern.Gate,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Rejected Patterns\n")
	for _, rejection := range rejections {
		fmt.Fprintf(&b, "- surface=`%s` upstream=`%s` decision=`%s` gate=`%s`\n",
			rejection.Surface,
			rejection.Upstream,
			rejection.Decision,
			rejection.Gate,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Catalog Gates\n")
	fmt.Fprintf(&b, "- local_research_gate=`%s`\n", researchStatus(surface))
	b.WriteString("- primary_source_gate=`official-docs-and-repos-reviewed`\n")
	b.WriteString("- runtime_fetch_gate=`disabled-static-reviewed-snapshot`\n")
	b.WriteString("- design_scope_gate=`github-native-serverless-core`\n")
	b.WriteString("- mini_variant_gate=`comparative-not-cloned`\n")
	b.WriteString("- self_improvement_gate=`proposal-only-no-agent-managed-writes`\n")
	b.WriteString("- raw_body_gate=`paths-urls-counts-hashes-and-decisions-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func researchRequestedCommand(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return "catalog"
	}
	switch strings.ToLower(fields[1]) {
	case "catalog", "sources", "source", "coverage", "map", "verify", "list":
		return strings.ToLower(fields[1])
	default:
		return "catalog"
	}
}

func inspectResearchSurface(root string) researchSurface {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	surface := researchSurface{
		Documents: make([]configSurfaceFile, 0, len(researchDocumentPaths)),
	}
	for _, path := range researchDocumentPaths {
		file := inspectConfigSurfaceFile(root, path)
		if file.Present {
			surface.DocumentsPresent++
		}
		surface.Documents = append(surface.Documents, file)
	}
	surface.Followups = countResearchFollowups(root)
	return surface
}

func countResearchFollowups(root string) int {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("docs/research-openclaw-hermes-landscape.md")))
	if err != nil {
		return 0
	}
	return strings.Count(strings.ToLower(string(body)), "follow-up")
}

func researchStatus(surface researchSurface) string {
	if surface.DocumentsPresent != len(surface.Documents) {
		return "warning"
	}
	return "ok"
}

func researchSources() []researchSource {
	return []researchSource{
		{ID: "openclaw-repo-readme", System: "openclaw", Kind: "primary-repo", URL: "https://github.com/openclaw/openclaw", Pattern: "local-first personal assistant gateway", Decision: "adapt-serverless"},
		{ID: "openclaw-architecture", System: "openclaw", Kind: "official-docs", URL: "https://docs.openclaw.ai/concepts/architecture", Pattern: "long-lived gateway, channels, nodes, websocket control plane", Decision: "replace-with-github-actions-issues"},
		{ID: "openclaw-skills", System: "openclaw", Kind: "official-docs", URL: "https://docs.openclaw.ai/tools/skills", Pattern: "markdown skills, loading order, allowlists, skill workshop", Decision: "repo-local-progressive-disclosure"},
		{ID: "openclaw-channels", System: "openclaw", Kind: "official-docs", URL: "https://docs.openclaw.ai/channels", Pattern: "multi-channel assistant inbox", Decision: "workflow-dispatch-bridge"},
		{ID: "openclaw-security", System: "openclaw", Kind: "official-docs", URL: "https://docs.openclaw.ai/gateway/security", Pattern: "untrusted inbound messages, pairing, sandboxing", Decision: "body-free-reports-and-label-gates"},
		{ID: "hermes-repo-readme", System: "hermes-agent", Kind: "primary-repo", URL: "https://github.com/NousResearch/hermes-agent", Pattern: "self-improving runtime, gateway, skills, memory, cron", Decision: "adapt-observability-reject-self-writes"},
		{ID: "hermes-skills", System: "hermes-agent", Kind: "official-docs", URL: "https://hermes-agent.nousresearch.com/docs/guides/work-with-skills/", Pattern: "skills_list and skill_view progressive disclosure", Decision: "repo-reader-skill-index"},
		{ID: "hermes-memory", System: "hermes-agent", Kind: "official-docs", URL: "https://hermes-agent.nousresearch.com/docs/user-guide/features/memory/", Pattern: "bounded MEMORY.md and USER.md prompt snapshot", Decision: "repo-local-soul-memory"},
		{ID: "hermes-cron", System: "hermes-agent", Kind: "official-docs", URL: "https://hermes-agent.nousresearch.com/docs/user-guide/features/cron/", Pattern: "scheduled fresh sessions and no-agent jobs", Decision: "github-actions-proactive-workflows"},
		{ID: "hermes-checkpoints", System: "hermes-agent", Kind: "official-docs", URL: "https://hermes-agent.nousresearch.com/docs/user-guide/checkpoints-and-rollback", Pattern: "checkpoint and rollback preview before restore", Decision: "inspect-only-checkpoint-reports"},
	}
}

func researchPatterns() []researchPattern {
	return []researchPattern{
		{Name: "issue-native-session", Upstream: "openclaw/hermes sessions", Surface: "/session plus GitHub issue comments", Status: "implemented", Gate: "canonical-github-issue-thread"},
		{Name: "serverless-wakeup", Upstream: "gateway daemon and cron", Surface: "GitHub Actions issues, comments, schedule, workflow_dispatch", Status: "adapted", Gate: "no-always-on-server"},
		{Name: "multi-channel-ingress", Upstream: "OpenClaw channels and Hermes gateway", Surface: "/channels plus channel-ingest/state/gateway/delivery workflows", Status: "adapted", Gate: "workflow-dispatch-canonical-issue"},
		{Name: "repo-local-soul", Upstream: "SOUL.md and profile files", Surface: "/soul, /profile, .gitclaw/SOUL.md", Status: "implemented", Gate: "reviewed-git-files"},
		{Name: "bounded-memory", Upstream: "Hermes MEMORY.md and USER.md", Surface: "/memory catalog/provenance/timeline", Status: "implemented", Gate: "body-free-memory-metadata"},
		{Name: "progressive-skills", Upstream: "OpenClaw skills and Hermes skill_view", Surface: "/skills catalog/select-plan/runtime/sources", Status: "implemented", Gate: "repo-local-no-install"},
		{Name: "tool-boundary", Upstream: "OpenClaw tools and Hermes toolsets", Surface: "/tools catalog/verify/risk/boundary", Status: "implemented", Gate: "deterministic-contracts"},
		{Name: "proactive-work", Upstream: "Hermes cron and OpenClaw automation", Surface: "/proactive plus scheduled Actions", Status: "adapted", Gate: "issue-opening-scheduled-work"},
		{Name: "checkpoint-readiness", Upstream: "Hermes checkpoints and rollback", Surface: "/checkpoints and /rollback inspect-only", Status: "adapted", Gate: "no-restore-no-reset"},
		{Name: "backup-durability", Upstream: "session export and durable state", Surface: "/backup plus gitclaw-backups branch", Status: "implemented", Gate: "post-turn-backup-branch"},
		{Name: "model-provider", Upstream: "multi-provider agent runtimes", Surface: "/models GitHub Models catalog/usage/cost/risk", Status: "adapted", Gate: "actions-token-github-models"},
	}
}

func researchRejections() []researchRejection {
	return []researchRejection{
		{Surface: "long-running-gateway-socket", Upstream: "OpenClaw gateway and Hermes gateway", Decision: "rejected-core-loop", Gate: "github-actions-only"},
		{Surface: "agent-managed-skill-writes", Upstream: "Hermes self-improving skills", Decision: "proposal-plan-only", Gate: "human-reviewed-git-diff"},
		{Surface: "remote-skill-install", Upstream: "ClawHub and Hermes Hub installs", Decision: "dry-run-install-plan", Gate: "no-registry-fetch-in-actions"},
		{Surface: "delegation-subagents", Upstream: "Hermes delegates and OpenClaw multi-agent routing", Decision: "out-of-scope-v1", Gate: "single-assistant-boundary"},
		{Surface: "destructive-rollback", Upstream: "Hermes rollback restore", Decision: "inspect-only-v1", Gate: "no-reset-clean-checkout"},
	}
}

func countResearchSourcesByKind(sources []researchSource, kind string) int {
	count := 0
	for _, source := range sources {
		if source.Kind == kind || (kind == "primary" && strings.HasPrefix(source.Kind, "primary")) || (kind == "primary" && source.Kind == "official-docs") {
			count++
		}
	}
	return count
}

func countResearchPatternsByStatus(patterns []researchPattern, status string) int {
	count := 0
	for _, pattern := range patterns {
		if pattern.Status == status {
			count++
		}
	}
	return count
}
