package gitclaw

import (
	"fmt"
	"strings"
)

type workspaceCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type workspaceCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderWorkspaceCatalogReport(ev Event, cfg Config) string {
	return renderWorkspaceCatalogReport(ev, cfg, true)
}

func RenderWorkspaceCatalogCLIReport(cfg Config) string {
	return renderWorkspaceCatalogReport(Event{}, cfg, false)
}

func renderWorkspaceCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectWorkspaceSurface(cfg.Workdir)
	findings := workspaceFindings(surface)
	entries := workspaceCatalogEntries()
	layers := workspaceCatalogLayers(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Workspace Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_workspace_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- workspace_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_workspace_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- workspace_catalog_status: `%s`\n", workspaceStatus(findings))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-github-actions-workspace-discovery")
	fmt.Fprintf(&b, "- workspace_model: `%s`\n", "github-actions-checkout-plus-repo-reviewed-policy")
	fmt.Fprintf(&b, "- workspace_scope: `%s`\n", "repository-checkout")
	fmt.Fprintf(&b, "- workspace_policy_path: `%s`\n", workspacePolicyPath)
	fmt.Fprintf(&b, "- workspace_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- workspace_policy_loaded_for_model: `%t`\n", workspacePolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- workspace_specs_dir: `%s`\n", workspaceSpecsDir)
	fmt.Fprintf(&b, "- workspace_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- workspace_specs_with_frontmatter: `%d`\n", workspaceSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- workspace_specs_requiring_approval: `%d`\n", workspaceSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- git_available: `%t`\n", surface.Git.GitAvailable)
	fmt.Fprintf(&b, "- git_repository: `%t`\n", surface.Git.GitRepository)
	fmt.Fprintf(&b, "- worktree_root: `%s`\n", inlineCode(surface.Git.Root))
	fmt.Fprintf(&b, "- branch: `%s`\n", inlineCode(surface.Git.Branch))
	fmt.Fprintf(&b, "- head_commit: `%s`\n", surface.Git.HeadShortSHA)
	fmt.Fprintf(&b, "- repo_files_listed: `%d`\n", surface.RepoFilesListed)
	fmt.Fprintf(&b, "- repo_file_list_limit: `%d`\n", surface.RepoFileListLimit)
	fmt.Fprintf(&b, "- context_documents_loaded: `%d`\n", surface.ContextDocumentsLoaded)
	fmt.Fprintf(&b, "- context_allowlist_entries: `%d`\n", surface.ContextAllowlist)
	fmt.Fprintf(&b, "- workflow_files_present: `%d`\n", workspaceWorkflowsPresent(surface.Workflows))
	fmt.Fprintf(&b, "- checkout_workflows: `%d`\n", workspaceCheckoutWorkflows(surface.Workflows))
	fmt.Fprintf(&b, "- checkout_steps: `%d`\n", workspaceCheckoutSteps(surface.Workflows))
	fmt.Fprintf(&b, "- setup_go_steps: `%d`\n", workspaceSetupGoSteps(surface.Workflows))
	fmt.Fprintf(&b, "- checkout_action_versions: `%s`\n", inlineCode(joinOrNone(workspaceCheckoutVersions(surface.Workflows))))
	fmt.Fprintf(&b, "- setup_go_action_versions: `%s`\n", inlineCode(joinOrNone(workspaceSetupGoVersions(surface.Workflows))))
	fmt.Fprintf(&b, "- fetch_depth_configured: `%t`\n", workspaceFetchDepthConfigured(surface.Workflows))
	fmt.Fprintf(&b, "- sandbox_backend: `%s`\n", "github-actions")
	fmt.Fprintf(&b, "- durable_state_backend: `%s`\n", "git-tracked-files-and-backup-branch")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- workspace_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- private_workspace_memory_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- external_workspace_mount_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- workspace_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- workspace_daemon_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- long_running_socket_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workspace_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_file_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workflow_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_workspace_catalog_change: `%t`\n", true)
	if surface.Git.ErrorReason != "" {
		fmt.Fprintf(&b, "- git_error_reason: `%s`\n", surface.Git.ErrorReason)
	}
	if surface.RepoFileListError != "" {
		fmt.Fprintf(&b, "- repo_file_list_error: `%s`\n", surface.RepoFileListError)
	}
	b.WriteByte('\n')

	b.WriteString("This catalog maps the GitHub Actions workspace surface inspired by OpenClaw workspace files and Hermes worktree separation: it exposes commands, layers, and gates while keeping file bodies, workflow bodies, issue/comment bodies, prompts, tool outputs, credentials, and workspace payloads out of the report.\n\n")

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

	b.WriteString("### Workspace Layers\n")
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
	fmt.Fprintf(&b, "- workspace_validation_gate=`%s`\n", workspaceStatus(findings))
	b.WriteString("- workspace_policy_gate=`repo-reviewed-policy-file`\n")
	b.WriteString("- workspace_spec_gate=`repo-reviewed-specs`\n")
	b.WriteString("- checkout_gate=`actions-checkout-metadata`\n")
	b.WriteString("- sandbox_gate=`workspace-is-not-sandbox`\n")
	b.WriteString("- durability_gate=`git-tracked-files-and-backup-branch`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func workspaceCatalogEntries() []workspaceCatalogEntry {
	return []workspaceCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /workspace catalog", LocalCommand: "gitclaw workspace catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "summary", IssueIntent: "@gitclaw /workspace", LocalCommand: "gitclaw workspace summary", Execution: "metadata-only", Gate: "body-free-workspace-envelope"},
		{Name: "verify", IssueIntent: "@gitclaw /workspace verify", LocalCommand: "gitclaw workspace verify", Execution: "metadata-only", Gate: "workspace-policy-spec-validation"},
		{Name: "risk", IssueIntent: "@gitclaw /workspace risk", LocalCommand: "gitclaw workspace risk", Execution: "risk-audit", Gate: "workspace-isolation"},
	}
}

func workspaceCatalogLayers(surface workspaceSurface) []workspaceCatalogLayer {
	return []workspaceCatalogLayer{
		{Name: "policy", Store: workspacePolicyPath, Source: "repo-reviewed-workspace-policy", Gate: "context-allowlist", Count: boolCount(surface.Policy.Present)},
		{Name: "specs", Store: workspaceSpecsDir + "/*.md", Source: "repo-reviewed-workspace-specs", Gate: "reviewed-frontmatter", Count: len(surface.Specs)},
		{Name: "git", Store: ".git", Source: "checked-out-repository-metadata", Gate: "read-only-metadata", Count: boolCount(surface.Git.GitRepository)},
		{Name: "workflow", Store: ".github/workflows/*.yml", Source: "GitHub Actions workflow metadata", Gate: "checkout-and-setup-metadata", Count: workspaceWorkflowsPresent(surface.Workflows)},
		{Name: "context", Store: "contextDocumentPaths", Source: "repo-context-loader", Gate: "bounded-context-allowlist", Count: surface.ContextDocumentsLoaded},
		{Name: "repository-inventory", Store: "git ls-files/listRepoFiles", Source: "bounded-repository-inventory", Gate: "bounded-file-list", Count: surface.RepoFilesListed},
		{Name: "runtime", Store: "GitHub Actions runner", Source: "ephemeral-runner", Gate: "ephemeral-runner-not-durable-memory", Count: 1},
		{Name: "durable-state", Store: "git tracked files + backup branch", Source: "reviewed-repository-state", Gate: "reviewed-files-and-post-turn-backups", Count: 2},
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
