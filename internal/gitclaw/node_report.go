package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const nodePolicyPath = ".gitclaw/NODES.md"
const nodeSpecsDir = ".gitclaw/nodes"

type nodeSurface struct {
	Policy configSurfaceFile
	Specs  []nodeSpecCard
}

type nodeSpecCard struct {
	Name             string
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	Frontmatter      bool
	Role             string
	Runtime          string
	Mode             string
	Capabilities     []string
	RequiresApproval bool
}

type nodeSpecFrontmatter struct {
	Name             string   `yaml:"name"`
	Role             string   `yaml:"role"`
	Runtime          string   `yaml:"runtime"`
	Mode             string   `yaml:"mode"`
	Capabilities     []string `yaml:"capabilities"`
	RequiresApproval bool     `yaml:"requires_approval"`
}

type nodeFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsNodeReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/nodes" || command == "/node"
}

func RenderNodeReport(ev Event, cfg Config) string {
	if isNodeCatalogRequest(ev, cfg) {
		return RenderNodeCatalogReport(ev, cfg)
	}
	if isNodeRiskRequest(ev, cfg) {
		return renderNodeRiskReport(ev, cfg, true)
	}
	return renderNodeReport(ev, cfg, true)
}

func RenderNodeCLIReport(cfg Config) string {
	return renderNodeReport(Event{}, cfg, false)
}

func RenderNodeRiskCLIReport(cfg Config) string {
	return renderNodeRiskReport(Event{}, cfg, false)
}

func renderNodeReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectNodeSurface(cfg.Workdir)
	findings := nodeFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Nodes Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- nodes_status: `%s`\n", nodeStatus(surface, findings))
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
	fmt.Fprintf(&b, "- gateway_websocket_required: `%t`\n", false)
	fmt.Fprintf(&b, "- headless_node_host_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- node_pairing_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- node_rpc_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- node_command_invocation_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_node_exec_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- browser_proxy_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- media_device_capabilities_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- long_running_node_service_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_node_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Nodes describe reviewed execution-host intent. GitClaw treats node specs as metadata only: no node host is started, no Gateway WebSocket is opened, no device is paired, no remote command is invoked, and no node, issue, comment, channel, credential, or provider payload body is printed by this report.\n\n")

	b.WriteString("### Node Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if nodePolicyPathInContext() {
		b.WriteString("- `.gitclaw/NODES.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/NODES.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Node Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` role=`%s` runtime=`%s` mode=`%s` capabilities=`%d` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Role),
				inlineCode(spec.Runtime),
				inlineCode(spec.Mode),
				len(spec.Capabilities),
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- GitHub Actions jobs are the only active execution nodes in v1\n")
	b.WriteString("- workflow dispatch and scheduled workflows are the GitHub-native wake paths\n")
	b.WriteString("- node specs are not service definitions and do not create paired devices\n")
	b.WriteString("- future remote-node execution requires reviewed workflows, explicit permissions, approval gates, body-free audit cards, and live GitHub Models E2E coverage\n")

	b.WriteString("\n### Verification Findings\n")
	if len(findings) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, finding := range findings {
			fmt.Fprintf(&b, "- severity=`%s` code=`%s` subject=`%s` message=`%s`\n", finding.Severity, finding.Code, finding.Subject, finding.Message)
		}
	}

	return strings.TrimSpace(b.String())
}

func inspectNodeSurface(root string) nodeSurface {
	if root == "" {
		root = "."
	}
	surface := nodeSurface{
		Policy: inspectConfigSurfaceFile(root, nodePolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.Specs = inspectNodeSpecs(absRoot)
	return surface
}

func inspectNodeSpecs(absRoot string) []nodeSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "nodes", "*.md"))
	sort.Strings(matches)
	specs := make([]nodeSpecCard, 0, len(matches))
	for _, match := range matches {
		body, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		relPath := filepath.ToSlash(rel)
		text := string(body)
		spec := nodeSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseNodeFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Role = strings.TrimSpace(meta.Role)
			spec.Runtime = strings.TrimSpace(meta.Runtime)
			spec.Mode = strings.TrimSpace(meta.Mode)
			spec.Capabilities = cleanNodeList(meta.Capabilities)
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func parseNodeFrontmatter(text string) (nodeSpecFrontmatter, bool) {
	var meta nodeSpecFrontmatter
	if !strings.HasPrefix(text, "---\n") {
		return meta, false
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, false
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(rest[:end])))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil {
		return nodeSpecFrontmatter{}, false
	}
	return meta, true
}

func nodeFindings(surface nodeSurface) []nodeFinding {
	var findings []nodeFinding
	if !surface.Policy.Present {
		findings = append(findings, nodeFinding{"info", "node_policy_not_configured", nodePolicyPath, "no node policy file is configured"})
	}
	if surface.Policy.Present && !nodePolicyPathInContext() {
		findings = append(findings, nodeFinding{"error", "node_policy_not_loaded", nodePolicyPath, "node policy file is not in the model context allowlist"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, nodeFinding{"warning", "node_frontmatter_missing", spec.Path, "node spec should start with YAML frontmatter"})
		}
		if strings.TrimSpace(spec.Role) == "" {
			findings = append(findings, nodeFinding{"warning", "node_role_missing", spec.Path, "node spec should declare a role such as primary-runtime or channel-bridge"})
		}
		if !strings.EqualFold(spec.Runtime, "github-actions") {
			findings = append(findings, nodeFinding{"warning", "node_runtime_not_github_actions", spec.Path, "GitClaw v1 only supports GitHub Actions execution nodes"})
		}
		if !strings.EqualFold(spec.Mode, "ephemeral-job") {
			findings = append(findings, nodeFinding{"warning", "node_mode_not_ephemeral_job", spec.Path, "GitClaw v1 only supports ephemeral-job node specs"})
		}
		if len(spec.Capabilities) == 0 {
			findings = append(findings, nodeFinding{"warning", "node_capabilities_missing", spec.Path, "node spec should list reviewed GitHub-native capabilities"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, nodeFinding{"warning", "node_approval_gate_missing", spec.Path, "node spec should require approval before pairing, remote execution, or new host capabilities"})
		}
	}
	return findings
}

func nodeStatus(surface nodeSurface, findings []nodeFinding) string {
	if !surface.Policy.Present && len(surface.Specs) == 0 {
		return "not_configured"
	}
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "error"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "warning"
		}
	}
	return "ok"
}

func nodePolicyLoadedForModel(surface nodeSurface) bool {
	return surface.Policy.Present && nodePolicyPathInContext()
}

func nodePolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == nodePolicyPath {
			return true
		}
	}
	return false
}

func cleanNodeList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}

func nodeSpecsWithFrontmatter(specs []nodeSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func nodeRoleCount(specs []nodeSpecCard) int {
	roles := map[string]bool{}
	for _, spec := range specs {
		if strings.TrimSpace(spec.Role) != "" {
			roles[spec.Role] = true
		}
	}
	return len(roles)
}

func nodeCapabilityCount(specs []nodeSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.Capabilities)
	}
	return count
}

func nodeSpecsRequiringApproval(specs []nodeSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func nodeSpecsEphemeralJobs(specs []nodeSpecCard) int {
	count := 0
	for _, spec := range specs {
		if strings.EqualFold(spec.Mode, "ephemeral-job") {
			count++
		}
	}
	return count
}

func isNodeRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/nodes" && command != "/node" {
		return false
	}
	return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
}

func isNodeCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/nodes" && command != "/node" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "catalog", "commands", "capabilities", "index":
		return true
	default:
		return false
	}
}
