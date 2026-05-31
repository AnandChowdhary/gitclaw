package gitclaw

import (
	"fmt"
	"strings"
)

type approvalCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type approvalCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderApprovalCatalogReport(ev Event, cfg Config) string {
	return renderApprovalCatalogReport(ev, cfg, true)
}

func RenderApprovalCatalogCLIReport(cfg Config) string {
	return renderApprovalCatalogReport(Event{}, cfg, false)
}

func renderApprovalCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	risk := BuildApprovalRiskReport(cfg)
	entries := approvalCatalogEntries()
	layers := approvalCatalogLayers(cfg)

	var b strings.Builder
	b.WriteString("## GitClaw Approvals Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_approvals_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- approvals_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_approval_metadata")
		fmt.Fprintf(&b, "- current_issue_labels_available: `%t`\n", true)
		fmt.Fprintf(&b, "- current_issue_labels: `%d`\n", len(ev.Issue.Labels))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
		fmt.Fprintf(&b, "- current_issue_labels_available: `%t`\n", false)
	}
	fmt.Fprintf(&b, "- approvals_catalog_status: `%s`\n", risk.Status)
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-github-issue-approval-discovery")
	fmt.Fprintf(&b, "- approval_model: `%s`\n", "github-actions-issue-label-approval-boundary")
	fmt.Fprintf(&b, "- approval_store: `%s`\n", risk.ApprovalStore)
	fmt.Fprintf(&b, "- approval_scope: `%s`\n", risk.ApprovalScope)
	fmt.Fprintf(&b, "- trusted_associations: `%d`\n", risk.TrustedAssociations)
	fmt.Fprintf(&b, "- broad_trusted_associations: `%d`\n", risk.BroadTrustedAssociations)
	fmt.Fprintf(&b, "- approval_labels_configured: `%d`\n", risk.ApprovalLabelsConfigured)
	fmt.Fprintf(&b, "- managed_labels_configured: `%d`\n", risk.ManagedLabelsConfigured)
	fmt.Fprintf(&b, "- duplicate_approval_labels: `%d`\n", risk.DuplicateApprovalLabels)
	fmt.Fprintf(&b, "- approval_managed_label_collisions: `%d`\n", risk.ApprovalManagedLabelCollisions)
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- approval_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- write_actions_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- write_actions_enabled: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- host_exec_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- approval_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_approvals_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps GitClaw's approval surface inspired by OpenClaw exec approval policy/allowlist/user-decision gates and Hermes dangerous-command approval boundaries: it exposes commands, layers, and gates while keeping approval payloads, issue/comment bodies, prompts, tool outputs, credentials, and secret values out of the report.\n\n")

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

	b.WriteString("### Approval Layers\n")
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
	fmt.Fprintf(&b, "- approval_catalog_gate=`%s`\n", risk.Status)
	b.WriteString("- preflight_gate=`trusted-association-required`\n")
	b.WriteString("- write_request_gate=`heuristic-plus-label-evidence`\n")
	b.WriteString("- approval_label_gate=`per-issue-github-label`\n")
	b.WriteString("- provenance_gate=`assistant-turn-marker-hashes`\n")
	b.WriteString("- risk_gate=`collision-and-broad-trust-audit`\n")
	b.WriteString("- write_mode_gate=`blocked-read-only-v1`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-labels-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func approvalCatalogEntries() []approvalCatalogEntry {
	return []approvalCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /approvals catalog", LocalCommand: "gitclaw approvals catalog", Execution: "metadata-only", Gate: "body-free-approval-command-map"},
		{Name: "list", IssueIntent: "@gitclaw /approvals", LocalCommand: "gitclaw approvals list", Execution: "metadata-only", Gate: "approval-readiness-inventory"},
		{Name: "verify", IssueIntent: "@gitclaw /approvals verify", LocalCommand: "gitclaw approvals verify", Execution: "metadata-only", Gate: "approval-readiness-inventory"},
		{Name: "provenance", IssueIntent: "@gitclaw /approvals provenance", LocalCommand: "gitclaw approvals provenance", Execution: "metadata-only", Gate: "body-free-evidence-chain"},
		{Name: "risk", IssueIntent: "@gitclaw /approvals risk", LocalCommand: "gitclaw approvals risk", Execution: "risk-audit", Gate: "approval-boundary-risk-audit"},
	}
}

func approvalCatalogLayers(cfg Config) []approvalCatalogLayer {
	return []approvalCatalogLayer{
		{Name: "authorization", Store: "authorization.allowed_associations", Source: "repo-config", Gate: "trusted-association-preflight", Count: len(sortedAllowedAssociations(cfg))},
		{Name: "write-request", Store: cfg.WriteRequestedLabel, Source: "transcript-heuristic-or-label", Gate: "write-intent-labeling", Count: 1},
		{Name: "approval-labels", Store: defaultApprovedLabel + "/" + defaultNeedsHumanLabel, Source: "github-issue-labels", Gate: "per-issue-approval-state", Count: len(approvalRiskLabels(cfg))},
		{Name: "managed-labels", Store: "GitClaw managed labels", Source: "repo-config", Gate: "label-collision-audit", Count: len(managedPolicyLabels(cfg))},
		{Name: "evidence", Store: "assistant-turn markers", Source: "issue-comments", Gate: "body-free-provenance", Count: 1},
		{Name: "runtime", Store: "GitHub Actions workflow", Source: "issue-comment-runner", Gate: "read-only-v1", Count: 1},
		{Name: "payloads", Store: "unsupported in reports", Source: "explicit-negative-capability", Gate: "body-free-reporting", Count: 0},
	}
}

func isApprovalCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		(fields[0] == "/approvals" || fields[0] == "/approval") &&
		(strings.EqualFold(fields[1], "catalog") || strings.EqualFold(fields[1], "commands") || strings.EqualFold(fields[1], "gates") || strings.EqualFold(fields[1], "index"))
}
