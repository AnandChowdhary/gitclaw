package gitclaw

import (
	"fmt"
	"os"
	"strings"
)

type artifactCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type artifactCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderArtifactCatalogReport(ev Event, cfg Config) string {
	return renderArtifactCatalogReport(ev, cfg, true)
}

func RenderArtifactCatalogCLIReport(cfg Config) string {
	return renderArtifactCatalogReport(Event{}, cfg, false)
}

func renderArtifactCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectArtifactSurface(cfg.Workdir)
	findings := artifactFindings(surface)
	entries := artifactCatalogEntries()
	layers := artifactCatalogLayers(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Artifacts Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_artifacts_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- artifacts_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_artifact_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- artifacts_catalog_status: `%s`\n", artifactStatus(surface, findings))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-github-actions-artifact-discovery")
	fmt.Fprintf(&b, "- artifact_model: `%s`\n", "github-actions-artifact-metadata")
	fmt.Fprintf(&b, "- artifact_scope: `%s`\n", "repository-run-evidence")
	fmt.Fprintf(&b, "- artifact_policy_path: `%s`\n", artifactPolicyPath)
	fmt.Fprintf(&b, "- artifact_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- artifact_policy_loaded_for_model: `%t`\n", artifactPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- artifact_specs_dir: `%s`\n", artifactSpecsDir)
	fmt.Fprintf(&b, "- artifact_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- artifact_specs_with_frontmatter: `%d`\n", artifactSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- artifact_specs_requiring_approval: `%d`\n", artifactSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- artifact_specs_requiring_redaction: `%d`\n", artifactSpecsRequiringRedaction(surface.Specs))
	fmt.Fprintf(&b, "- artifact_retention_days_declared: `%d`\n", artifactRetentionDaysDeclared(surface.Specs))
	fmt.Fprintf(&b, "- github_actions_artifact_uploaders: `%d`\n", len(surface.Workflows))
	fmt.Fprintf(&b, "- upload_artifact_versions: `%s`\n", inlineCode(strings.Join(artifactUploadVersions(surface.Workflows), ", ")))
	fmt.Fprintf(&b, "- prompt_artifact_default_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- prompt_artifact_label: `%s`\n", "gitclaw:e2e-prompt-artifact")
	fmt.Fprintf(&b, "- prompt_artifact_env_path_configured: `%t`\n", strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "")
	fmt.Fprintf(&b, "- artifact_storage_backend: `%s`\n", "github-actions-artifacts")
	fmt.Fprintf(&b, "- durable_backup_backend: `%s`\n", "git-backup-branch")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- artifact_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- artifact_body_printing_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- artifact_as_hidden_state_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- external_artifact_storage_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- long_term_artifact_memory_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- automatic_artifact_restore_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- unredacted_prompt_artifact_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_artifact_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_artifacts_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps GitClaw's artifact surface inspired by OpenClaw media/file evidence and Hermes session/checkpoint exports: it exposes commands, layers, and gates while keeping artifact payloads, prompt bodies, tool outputs, issue/comment bodies, credentials, channel payloads, backup payloads, and session bodies out of the report.\n\n")

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

	b.WriteString("### Artifact Layers\n")
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
	fmt.Fprintf(&b, "- artifact_validation_gate=`%s`\n", artifactStatus(surface, findings))
	b.WriteString("- artifact_policy_gate=`repo-reviewed-policy-file`\n")
	b.WriteString("- artifact_spec_gate=`repo-reviewed-specs`\n")
	b.WriteString("- workflow_upload_gate=`reviewed-github-actions-upload-step`\n")
	b.WriteString("- redaction_gate=`required-before-prompt-artifact-upload`\n")
	b.WriteString("- retention_gate=`explicit-short-lived-retention`\n")
	b.WriteString("- backup_gate=`durable-state-uses-git-backup-branch`\n")
	b.WriteString("- hidden_state_gate=`artifacts-not-agent-memory`\n")
	b.WriteString("- external_storage_gate=`disabled-github-actions-artifacts-only`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func artifactCatalogEntries() []artifactCatalogEntry {
	return []artifactCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /artifacts catalog", LocalCommand: "gitclaw artifacts catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "list", IssueIntent: "@gitclaw /artifacts", LocalCommand: "gitclaw artifacts list", Execution: "metadata-only", Gate: "body-free-artifact-envelope"},
		{Name: "verify", IssueIntent: "@gitclaw /artifacts verify", LocalCommand: "gitclaw artifacts verify", Execution: "metadata-only", Gate: "policy-spec-workflow-validation"},
		{Name: "risk", IssueIntent: "@gitclaw /artifacts risk", LocalCommand: "gitclaw artifacts risk", Execution: "risk-audit", Gate: "no-hidden-state-boundary"},
	}
}

func artifactCatalogLayers(surface artifactSurface) []artifactCatalogLayer {
	policyCount := 0
	if surface.Policy.Present {
		policyCount = 1
	}
	return []artifactCatalogLayer{
		{Name: "policy", Store: artifactPolicyPath, Source: "repo-reviewed-artifact-policy", Gate: "context-allowlist", Count: policyCount},
		{Name: "specs", Store: artifactSpecsDir + "/*.md", Source: "repo-reviewed-artifact-specs", Gate: "reviewed-frontmatter", Count: len(surface.Specs)},
		{Name: "workflow", Store: ".github/workflows/*.yml", Source: "reviewed-upload-artifact-steps", Gate: "github-actions-workflow-review", Count: len(surface.Workflows)},
		{Name: "storage", Store: "GitHub Actions artifacts", Source: "short-lived-run-evidence", Gate: "retention-and-label-gated", Count: len(surface.Workflows)},
		{Name: "redaction", Store: "artifact spec redaction_required", Source: "repo-reviewed-frontmatter", Gate: "redaction-before-upload", Count: artifactSpecsRequiringRedaction(surface.Specs)},
		{Name: "retention", Store: "artifact spec retention_days", Source: "repo-reviewed-frontmatter", Gate: "explicit-retention-days", Count: artifactRetentionDaysDeclared(surface.Specs)},
		{Name: "durable-backup", Store: "git backup branch", Source: "durable-transcript-backup", Gate: "not-actions-artifact-memory", Count: 1},
		{Name: "payloads", Store: "unsupported in reports", Source: "explicit-negative-capability", Gate: "body-free-reporting", Count: 0},
	}
}
