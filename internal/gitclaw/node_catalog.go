package gitclaw

import (
	"fmt"
	"strings"
)

type nodeCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type nodeCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderNodeCatalogReport(ev Event, cfg Config) string {
	return renderNodeCatalogReport(ev, cfg, true)
}

func RenderNodeCatalogCLIReport(cfg Config) string {
	return renderNodeCatalogReport(Event{}, cfg, false)
}

func renderNodeCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectNodeSurface(cfg.Workdir)
	findings := nodeFindings(surface)
	entries := nodeCatalogEntries()
	layers := nodeCatalogLayers(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Nodes Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_nodes_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- nodes_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_node_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- nodes_catalog_status: `%s`\n", nodeStatus(surface, findings))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-github-actions-node-discovery")
	fmt.Fprintf(&b, "- node_model: `%s`\n", "github-actions-ephemeral-node-metadata")
	fmt.Fprintf(&b, "- node_scope: `%s`\n", "repository-execution-node")
	fmt.Fprintf(&b, "- node_policy_path: `%s`\n", nodePolicyPath)
	fmt.Fprintf(&b, "- node_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- node_policy_loaded_for_model: `%t`\n", nodePolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- node_specs_dir: `%s`\n", nodeSpecsDir)
	fmt.Fprintf(&b, "- node_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- node_specs_with_frontmatter: `%d`\n", nodeSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- node_roles: `%d`\n", nodeRoleCount(surface.Specs))
	fmt.Fprintf(&b, "- node_capabilities_declared: `%d`\n", nodeCapabilityCount(surface.Specs))
	fmt.Fprintf(&b, "- node_specs_requiring_approval: `%d`\n", nodeSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- node_specs_ephemeral_jobs: `%d`\n", nodeSpecsEphemeralJobs(surface.Specs))
	fmt.Fprintf(&b, "- active_node_runtime: `%s`\n", "github-actions-ephemeral-job")
	fmt.Fprintf(&b, "- node_inventory_source: `%s`\n", "git-reviewed-metadata")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- node_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- gateway_websocket_required: `%t`\n", false)
	fmt.Fprintf(&b, "- gateway_process_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- headless_node_host_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- node_pairing_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- node_rpc_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- node_command_invocation_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_node_exec_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- browser_proxy_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- media_device_capabilities_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- long_running_node_service_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- cross_node_routing_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_node_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_nodes_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps GitClaw's node surface inspired by OpenClaw paired nodes and Hermes worker/delegation boundaries: it exposes commands, layers, and gates while keeping node policy/spec bodies, issue/comment bodies, prompts, tool outputs, credentials, channel payloads, worker payloads, and session bodies out of the report.\n\n")

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

	b.WriteString("### Node Layers\n")
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
	fmt.Fprintf(&b, "- node_validation_gate=`%s`\n", nodeStatus(surface, findings))
	b.WriteString("- node_policy_gate=`repo-reviewed-policy-file`\n")
	b.WriteString("- node_spec_gate=`repo-reviewed-specs`\n")
	b.WriteString("- runtime_gate=`github-actions-ephemeral-job-only`\n")
	b.WriteString("- wake_gate=`issues-schedule-workflow-dispatch-only`\n")
	b.WriteString("- pairing_gate=`disabled-no-device-pairing-v1`\n")
	b.WriteString("- gateway_gate=`disabled-no-websocket-gateway-v1`\n")
	b.WriteString("- remote_exec_gate=`disabled-no-remote-node-exec-v1`\n")
	b.WriteString("- capability_gate=`repo-reviewed-capability-names-only`\n")
	b.WriteString("- approval_gate=`required-before-pairing-or-host-capabilities`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func nodeCatalogEntries() []nodeCatalogEntry {
	return []nodeCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /nodes catalog", LocalCommand: "gitclaw nodes catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "list", IssueIntent: "@gitclaw /nodes", LocalCommand: "gitclaw nodes list", Execution: "metadata-only", Gate: "body-free-node-envelope"},
		{Name: "verify", IssueIntent: "@gitclaw /nodes verify", LocalCommand: "gitclaw nodes verify", Execution: "metadata-only", Gate: "policy-spec-validation"},
		{Name: "risk", IssueIntent: "@gitclaw /nodes risk", LocalCommand: "gitclaw nodes risk", Execution: "risk-audit", Gate: "no-remote-exec-boundary"},
	}
}

func nodeCatalogLayers(surface nodeSurface) []nodeCatalogLayer {
	policyCount := 0
	if surface.Policy.Present {
		policyCount = 1
	}
	return []nodeCatalogLayer{
		{Name: "policy", Store: nodePolicyPath, Source: "repo-reviewed-node-policy", Gate: "context-allowlist", Count: policyCount},
		{Name: "specs", Store: nodeSpecsDir + "/*.md", Source: "repo-reviewed-node-specs", Gate: "reviewed-frontmatter", Count: len(surface.Specs)},
		{Name: "runtime", Store: "GitHub Actions workflow", Source: "issue-comment-runner", Gate: "github-actions-ephemeral-job", Count: 1},
		{Name: "wake", Store: "issues/schedule/workflow_dispatch", Source: "github-native-events", Gate: "github-native-wake-paths", Count: 3},
		{Name: "conversation", Store: "GitHub issues and comments", Source: "canonical-session-boundary", Gate: "issue-native-thread", Count: 1},
		{Name: "capabilities", Store: "node spec capability names", Source: "repo-reviewed-capability-intent", Gate: "capability-name-metadata-only", Count: nodeCapabilityCount(surface.Specs)},
		{Name: "approval", Store: "node spec requires_approval", Source: "repo-reviewed-frontmatter", Gate: "pairing-exec-side-effect-approval", Count: nodeSpecsRequiringApproval(surface.Specs)},
		{Name: "remote-exec", Store: "unsupported in v1", Source: "explicit-negative-capability", Gate: "disabled-no-node-host-v1", Count: 0},
	}
}
