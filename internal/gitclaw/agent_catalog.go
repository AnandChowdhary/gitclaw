package gitclaw

import (
	"fmt"
	"strings"
)

type agentCatalogEntry struct {
	Name            string
	IssueIntent     string
	LocalCommand    string
	Execution       string
	Gate            string
	RawBodies       bool
	MutationAllowed bool
}

type agentCatalogLayer struct {
	Name      string
	Store     string
	Source    string
	Gate      string
	Count     int
	RawBodies bool
}

func RenderAgentCatalogReport(ev Event, cfg Config) string {
	return renderAgentCatalogReport(ev, cfg, true)
}

func RenderAgentCatalogCLIReport(cfg Config) string {
	return renderAgentCatalogReport(Event{}, cfg, false)
}

func renderAgentCatalogReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectAgentSurface(cfg.Workdir)
	findings := agentFindings(surface)
	entries := agentCatalogEntries()
	layers := agentCatalogLayers(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Agents Catalog Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_agents_command: `%s`\n", "catalog")
		fmt.Fprintf(&b, "- agents_command_status: `%s`\n", "ok")
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
		fmt.Fprintf(&b, "- issue_side_execution: `%s`\n", "github_actions_agent_metadata")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- agents_catalog_status: `%s`\n", agentStatus(surface, findings))
	fmt.Fprintf(&b, "- catalog_strategy: `%s`\n", "compact-single-assistant-agent-discovery")
	fmt.Fprintf(&b, "- agent_model: `%s`\n", "github-actions-single-repo-assistant")
	fmt.Fprintf(&b, "- agent_scope: `%s`\n", "repository-issue-assistant")
	fmt.Fprintf(&b, "- agent_policy_path: `%s`\n", agentPolicyPath)
	fmt.Fprintf(&b, "- agent_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- agent_policy_loaded_for_model: `%t`\n", agentPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- agent_specs_dir: `%s`\n", agentSpecsDir)
	fmt.Fprintf(&b, "- agent_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- agent_specs_with_frontmatter: `%d`\n", agentSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- agent_roles: `%d`\n", agentRoleCount(surface.Specs))
	fmt.Fprintf(&b, "- agent_tools_requested: `%d`\n", agentToolCount(surface.Specs))
	fmt.Fprintf(&b, "- agent_specs_requiring_approval: `%d`\n", agentSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- agent_specs_single_assistant: `%d`\n", agentSpecsSingleAssistant(surface.Specs))
	fmt.Fprintf(&b, "- active_agent_runtime: `%s`\n", "github-actions")
	fmt.Fprintf(&b, "- catalog_entries: `%d`\n", len(entries))
	fmt.Fprintf(&b, "- agent_layers: `%d`\n", len(layers))
	fmt.Fprintf(&b, "- multi_agent_routing_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- multi_agent_delegation_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- subagent_execution_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- delegate_task_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_agent_process_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- agent_to_agent_messaging_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- shared_agent_memory_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- agent_gateway_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_agent_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_agents_catalog_change: `%t`\n", true)
	b.WriteByte('\n')

	b.WriteString("This catalog maps GitClaw's agent surface inspired by OpenClaw agent files and Hermes profiles/delegation boundaries: it exposes commands, layers, and gates while keeping agent policy/spec bodies, issue/comment bodies, prompts, tool outputs, credentials, channel payloads, and session bodies out of the report.\n\n")

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

	b.WriteString("### Agent Layers\n")
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
	fmt.Fprintf(&b, "- agent_validation_gate=`%s`\n", agentStatus(surface, findings))
	b.WriteString("- agent_policy_gate=`repo-reviewed-policy-file`\n")
	b.WriteString("- agent_spec_gate=`repo-reviewed-specs`\n")
	b.WriteString("- runtime_gate=`github-actions-only`\n")
	b.WriteString("- delegation_gate=`disabled-single-assistant-v1`\n")
	b.WriteString("- profile_gate=`single-repository-profile-only`\n")
	b.WriteString("- tool_gate=`repo-reviewed-tool-names-only`\n")
	b.WriteString("- approval_gate=`required-before-routing-or-side-effects`\n")
	b.WriteString("- raw_body_gate=`hashes-counts-and-metadata-only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func agentCatalogEntries() []agentCatalogEntry {
	return []agentCatalogEntry{
		{Name: "catalog", IssueIntent: "@gitclaw /agents catalog", LocalCommand: "gitclaw agents catalog", Execution: "metadata-only", Gate: "body-free-output"},
		{Name: "list", IssueIntent: "@gitclaw /agents", LocalCommand: "gitclaw agents list", Execution: "metadata-only", Gate: "body-free-agent-envelope"},
		{Name: "verify", IssueIntent: "@gitclaw /agents verify", LocalCommand: "gitclaw agents verify", Execution: "metadata-only", Gate: "policy-spec-validation"},
		{Name: "risk", IssueIntent: "@gitclaw /agents risk", LocalCommand: "gitclaw agents risk", Execution: "risk-audit", Gate: "single-assistant-boundary"},
	}
}

func agentCatalogLayers(surface agentSurface) []agentCatalogLayer {
	policyCount := 0
	if surface.Policy.Present {
		policyCount = 1
	}
	return []agentCatalogLayer{
		{Name: "policy", Store: agentPolicyPath, Source: "repo-reviewed-agent-policy", Gate: "context-allowlist", Count: policyCount},
		{Name: "specs", Store: agentSpecsDir + "/*.md", Source: "repo-reviewed-agent-specs", Gate: "reviewed-frontmatter", Count: len(surface.Specs)},
		{Name: "runtime", Store: "GitHub Actions workflow", Source: "issue-comment-runner", Gate: "github-actions-only", Count: 1},
		{Name: "conversation", Store: "GitHub issues and comments", Source: "canonical-session-boundary", Gate: "issue-native-thread", Count: 1},
		{Name: "tools", Store: "agent spec tool names", Source: "repo-reviewed-tool-intent", Gate: "tool-name-metadata-only", Count: agentToolCount(surface.Specs)},
		{Name: "approval", Store: "agent spec requires_approval", Source: "repo-reviewed-frontmatter", Gate: "routing-side-effect-approval", Count: agentSpecsRequiringApproval(surface.Specs)},
		{Name: "delegation", Store: "unsupported in v1", Source: "explicit-negative-capability", Gate: "disabled-single-assistant-v1", Count: 0},
	}
}
