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

const agentPolicyPath = ".gitclaw/AGENTS.md"
const agentSpecsDir = ".gitclaw/agents"

type agentSurface struct {
	Policy configSurfaceFile
	Specs  []agentSpecCard
}

type agentSpecCard struct {
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
	Tools            []string
	RequiresApproval bool
}

type agentSpecFrontmatter struct {
	Name             string   `yaml:"name"`
	Role             string   `yaml:"role"`
	Runtime          string   `yaml:"runtime"`
	Mode             string   `yaml:"mode"`
	Tools            []string `yaml:"tools"`
	RequiresApproval bool     `yaml:"requires_approval"`
}

type agentFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsAgentReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/agents" || command == "/agent"
}

func RenderAgentReport(ev Event, cfg Config) string {
	if isAgentCatalogRequest(ev, cfg) {
		return RenderAgentCatalogReport(ev, cfg)
	}
	if isAgentProvenanceRequest(ev, cfg) {
		return RenderAgentProvenanceReport(ev, cfg)
	}
	if isAgentRiskRequest(ev, cfg) {
		return renderAgentRiskReport(ev, cfg, true)
	}
	return renderAgentReport(ev, cfg, true)
}

func RenderAgentCLIReport(cfg Config) string {
	return renderAgentReport(Event{}, cfg, false)
}

func RenderAgentRiskCLIReport(cfg Config) string {
	return renderAgentRiskReport(Event{}, cfg, false)
}

func renderAgentReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectAgentSurface(cfg.Workdir)
	findings := agentFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Agents Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- agents_status: `%s`\n", agentStatus(surface, findings))
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
	fmt.Fprintf(&b, "- multi_agent_routing_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- multi_agent_delegation_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- subagent_execution_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- delegate_task_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_agent_process_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- agent_to_agent_messaging_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_agent_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Agents describe reviewed assistant identity and routing intent. GitClaw treats agent specs as metadata only: no subagent is spawned, no gateway or node runtime is started, no agent-to-agent message is sent, and no agent, issue, comment, channel, or credential body is printed by this report.\n\n")

	b.WriteString("### Agent Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if agentPolicyPathInContext() {
		b.WriteString("- `.gitclaw/AGENTS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/AGENTS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Agent Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` role=`%s` runtime=`%s` mode=`%s` tools=`%d` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Role),
				inlineCode(spec.Runtime),
				inlineCode(spec.Mode),
				len(spec.Tools),
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- GitHub Actions is the only active assistant runtime in v1\n")
	b.WriteString("- GitHub issues and comments are the conversation and handoff boundary\n")
	b.WriteString("- agent specs are not process definitions and do not create child workers\n")
	b.WriteString("- future multi-agent routing or delegation requires reviewed workflows, explicit permissions, approval gates, and live GitHub Models E2E coverage\n")

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

func inspectAgentSurface(root string) agentSurface {
	if root == "" {
		root = "."
	}
	surface := agentSurface{
		Policy: inspectConfigSurfaceFile(root, agentPolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.Specs = inspectAgentSpecs(absRoot)
	return surface
}

func inspectAgentSpecs(absRoot string) []agentSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "agents", "*.md"))
	sort.Strings(matches)
	specs := make([]agentSpecCard, 0, len(matches))
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
		spec := agentSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseAgentFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Role = strings.TrimSpace(meta.Role)
			spec.Runtime = strings.TrimSpace(meta.Runtime)
			spec.Mode = strings.TrimSpace(meta.Mode)
			spec.Tools = cleanAgentList(meta.Tools)
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func parseAgentFrontmatter(text string) (agentSpecFrontmatter, bool) {
	var meta agentSpecFrontmatter
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
		return agentSpecFrontmatter{}, false
	}
	return meta, true
}

func agentFindings(surface agentSurface) []agentFinding {
	var findings []agentFinding
	if !surface.Policy.Present {
		findings = append(findings, agentFinding{"info", "agent_policy_not_configured", agentPolicyPath, "no agent policy file is configured"})
	}
	if surface.Policy.Present && !agentPolicyPathInContext() {
		findings = append(findings, agentFinding{"error", "agent_policy_not_loaded", agentPolicyPath, "agent policy file is not in the model context allowlist"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, agentFinding{"warning", "agent_frontmatter_missing", spec.Path, "agent spec should start with YAML frontmatter"})
		}
		if strings.TrimSpace(spec.Role) == "" {
			findings = append(findings, agentFinding{"warning", "agent_role_missing", spec.Path, "agent spec should declare a role such as primary, reviewer, or operator"})
		}
		if !strings.EqualFold(spec.Runtime, "github-actions") {
			findings = append(findings, agentFinding{"warning", "agent_runtime_not_github_actions", spec.Path, "GitClaw v1 only supports the GitHub Actions assistant runtime"})
		}
		if !strings.EqualFold(spec.Mode, "single-assistant") {
			findings = append(findings, agentFinding{"warning", "agent_mode_not_single_assistant", spec.Path, "GitClaw v1 only supports single-assistant agent specs"})
		}
		if len(spec.Tools) == 0 {
			findings = append(findings, agentFinding{"warning", "agent_tools_missing", spec.Path, "agent spec should list reviewed tool names"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, agentFinding{"warning", "agent_approval_gate_missing", spec.Path, "agent spec should require approval before delegation, routing, or mutating side effects"})
		}
	}
	return findings
}

func agentStatus(surface agentSurface, findings []agentFinding) string {
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

func agentPolicyLoadedForModel(surface agentSurface) bool {
	return surface.Policy.Present && agentPolicyPathInContext()
}

func agentPolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == agentPolicyPath {
			return true
		}
	}
	return false
}

func cleanAgentList(values []string) []string {
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

func agentSpecsWithFrontmatter(specs []agentSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func agentRoleCount(specs []agentSpecCard) int {
	roles := map[string]bool{}
	for _, spec := range specs {
		if strings.TrimSpace(spec.Role) != "" {
			roles[spec.Role] = true
		}
	}
	return len(roles)
}

func agentToolCount(specs []agentSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.Tools)
	}
	return count
}

func agentSpecsRequiringApproval(specs []agentSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func agentSpecsSingleAssistant(specs []agentSpecCard) int {
	count := 0
	for _, spec := range specs {
		if strings.EqualFold(spec.Mode, "single-assistant") {
			count++
		}
	}
	return count
}

func isAgentRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/agents" && command != "/agent" {
		return false
	}
	return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
}

func isAgentCatalogRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/agents" && command != "/agent" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "catalog", "commands", "capabilities", "index":
		return true
	default:
		return false
	}
}
