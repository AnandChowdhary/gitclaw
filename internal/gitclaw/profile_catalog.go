package gitclaw

import (
	"fmt"
	"strings"
)

type profileCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type profileCatalogLayer struct {
	Name          string
	Store         string
	Source        string
	Gate          string
	Count         int
	PromptVisible bool
	RawBodies     bool
}

func RenderProfileCatalogReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderProfileCatalogReport(ev, cfg, repoContext, true)
}

func RenderProfileCatalogCLIReport(cfg Config, repoContext RepoContext) string {
	return renderProfileCatalogReport(Event{}, cfg, repoContext, false)
}

func renderProfileCatalogReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	soulValidation := ValidateSoulContext(repoContext)
	skillValidation := ValidateSkillSummaries(repoContext.SkillSummaries)
	toolValidation := ValidateTools(repoContext)
	profileDocs := profileDocuments(repoContext.Documents)
	entries := profileCatalogEntries()
	layers := profileCatalogLayers(cfg, repoContext, profileDocs)

	var b strings.Builder
	b.WriteString("## GitClaw Profile Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_profile_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- profile_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "repo_local_profile_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_catalog_status: `%s`\n", profileStatus(soulValidation, skillValidation, toolValidation))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-repo-local-profile-discovery")
	fmt.Fprintf(&b, "- profile_strategy: `%s`\n", "repo-local-git-profile")
	fmt.Fprintf(&b, "- profile_store: `%s`\n", ".gitclaw/")
	fmt.Fprintf(&b, "- profile_scope: `%s`\n", "repository")
	fmt.Fprintf(&b, "- profile_surface: `%s`\n", "identity, user, soul, memory, skills, bundles, tools, models, proactive, hooks, channels, backups, sessions")
	fmt.Fprintf(&b, "- provider: `%s`\n", cfg.ModelProvider)
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- profile_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", len(profileDocs))
	fmt.Fprintf(&b, "- profile_documents_cataloged: `%d`\n", len(profileDocs))
	fmt.Fprintf(&b, "- identity_policy_files: `%d`\n", soulIdentityDocumentCount(repoContext.Documents))
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", soulMemoryDocumentCount(repoContext.Documents))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", len(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_profile_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_config_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_profile_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps the repo-local GitClaw profile surface inspired by OpenClaw workspace files and Hermes profiles: it exposes commands, layers, and gates while keeping raw profile files, issue/comment bodies, prompts, tool outputs, credentials, sessions, and backup payloads out of the report.\n\n")

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

	b.WriteString("### Profile Layers\n")
	for _, layer := range layers {
		fmt.Fprintf(&b, "- layer=`%s` store=`%s` source=`%s` gate=`%s` count=`%d` prompt_visible=`%t` raw_bodies_included=`%t`\n",
			layer.Name,
			layer.Store,
			layer.Source,
			layer.Gate,
			layer.Count,
			layer.PromptVisible,
			layer.RawBodies,
		)
	}
	b.WriteByte('\n')

	b.WriteString("### Catalog Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", profileStatus(soulValidation, skillValidation, toolValidation))
	b.WriteString("- profile_store_gate=`repo-local-reviewed-files`\n")
	b.WriteString("- switching_gate=`unsupported-single-repository-profile`\n")
	b.WriteString("- export_gate=`plan-only`\n")
	b.WriteString("- import_gate=`unsupported`\n")
	b.WriteString("- credential_gate=`secret-names-only`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- session_gate=`github-issue-thread-plus-backup-json`\n")
	b.WriteString("- backup_gate=`gitclaw-backups-branch-metadata-only`\n")
	return strings.TrimSpace(b.String())
}

func profileCatalogEntries() []profileCatalogEntry {
	return []profileCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /profile catalog", LocalCommand: "gitclaw profile catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "show", IssueIntent: "@gitclaw /profile", LocalCommand: "gitclaw profile show", Execution: "repo-local-context", Gate: "body-free-profile-envelope"},
		{Name: "verify", IssueIntent: "@gitclaw /profile verify", LocalCommand: "gitclaw profile verify", Execution: "repo-local-validation", Gate: "soul-skill-tool-validation"},
		{Name: "provenance", IssueIntent: "@gitclaw /profile provenance", LocalCommand: "gitclaw profile provenance", Execution: "repo-local-git-history", Gate: "commit-subject-hashes-only"},
		{Name: "search", IssueIntent: "@gitclaw /profile search <query>", LocalCommand: "gitclaw profile search <query>", Execution: "body-free-profile-search", Gate: "query-hash-and-line-hashes"},
		{Name: "snapshot", IssueIntent: "@gitclaw /profile snapshot", LocalCommand: "gitclaw profile snapshot", Execution: "composite-profile-fingerprint", Gate: "body-free-snapshot"},
		{Name: "manifest", IssueIntent: "@gitclaw /profile manifest", LocalCommand: "gitclaw profile manifest", Execution: "dry-run-portability-manifest", Gate: "no-profile-export"},
		{Name: "export-plan", IssueIntent: "@gitclaw /profile export-plan", LocalCommand: "gitclaw profile export-plan", Execution: "dry-run-portability-plan", Gate: "no-profile-export"},
		{Name: "risk", IssueIntent: "@gitclaw /profile risk", LocalCommand: "gitclaw profile risk", Execution: "repo-local-risk-audit", Gate: "profile-isolation"},
	}
}

func profileCatalogLayers(cfg Config, repoContext RepoContext, profileDocs []ContextDocument) []profileCatalogLayer {
	proactivePrompts := 0
	if strings.TrimSpace(cfg.Workdir) != "" {
		proactivePrompts = len(inspectProactiveSurface(cfg.Workdir).Prompts)
	}
	return []profileCatalogLayer{
		{Name: "identity", Store: ".gitclaw/IDENTITY.md", Source: "repo-local-profile-document", Gate: "required-identity-policy", Count: profileDocumentCategoryCount(profileDocs, "identity"), PromptVisible: true},
		{Name: "user", Store: ".gitclaw/USER.md", Source: "repo-local-profile-document", Gate: "required-user-context", Count: profileDocumentCategoryCount(profileDocs, "user"), PromptVisible: true},
		{Name: "soul", Store: ".gitclaw/SOUL.md", Source: "repo-local-profile-document", Gate: "required-personality-boundary", Count: profileDocumentCategoryCount(profileDocs, "soul"), PromptVisible: true},
		{Name: "memory", Store: ".gitclaw/MEMORY.md + .gitclaw/memory/*.md", Source: "repo-local-reviewed-markdown", Gate: "bounded-memory-load", Count: profileDocumentCategoryCount(profileDocs, "memory") + profileDocumentCategoryCount(profileDocs, "memory-note"), PromptVisible: true},
		{Name: "skills", Store: ".gitclaw/SKILLS", Source: "repo-local-skill-metadata", Gate: "progressive-disclosure", Count: len(repoContext.SkillSummaries), PromptVisible: true},
		{Name: "skill-bundles", Store: ".gitclaw/skill-bundles", Source: "repo-local-bundle-specs", Gate: "selection-plan-only", Count: len(repoContext.SkillBundles), PromptVisible: false},
		{Name: "tools", Store: "runtime contracts + .gitclaw/TOOLS.md", Source: "deterministic-tool-contracts", Gate: "contract-only-tool-surface", Count: len(toolReportContracts), PromptVisible: true},
		{Name: "models", Store: ".gitclaw/config.yml model", Source: "effective-config", Gate: "github-models-actions-token", Count: 1, PromptVisible: true},
		{Name: "proactive", Store: ".gitclaw/proactive + .github/workflows", Source: "scheduled-workflow-prompts", Gate: "workflow-dispatch-issue-ingress", Count: proactivePrompts, PromptVisible: false},
		{Name: "hooks", Store: ".gitclaw/HOOKS.md", Source: "repo-local-policy", Gate: "event-policy-only", Count: profileDocumentCategoryCount(profileDocs, "hooks"), PromptVisible: true},
		{Name: "channels", Store: "workflow_dispatch + GitHub issues", Source: "external-provider-bridges", Gate: "canonical-github-issue-thread", Count: 1, PromptVisible: false},
		{Name: "backups", Store: "gitclaw-backups branch", Source: "post-turn-backup-workflow", Gate: "metadata-before-raw-recovery", Count: 1, PromptVisible: false},
		{Name: "sessions", Store: "GitHub issue thread + backup JSON", Source: "issue-native-session-store", Gate: "body-free-session-catalog", Count: 1, PromptVisible: false},
	}
}

func profileDocumentCategoryCount(docs []ContextDocument, category string) int {
	count := 0
	for _, doc := range docs {
		if profileDocumentCategory(doc.Path) == category {
			count++
		}
	}
	return count
}
