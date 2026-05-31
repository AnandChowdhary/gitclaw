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

const pluginPolicyPath = ".gitclaw/PLUGINS.md"
const pluginSpecsDir = ".gitclaw/plugins"

type pluginSurface struct {
	Policy       configSurfaceFile
	Specs        []pluginSpecCard
	PackageFiles []configSurfaceFile
}

type pluginSpecCard struct {
	Name                 string
	Path                 string
	Present              bool
	Bytes                int
	Lines                int
	SHA                  string
	Frontmatter          bool
	Kind                 string
	Source               string
	Activation           string
	Capabilities         []string
	OptionalCapabilities []string
	RequiresApproval     bool
}

type pluginSpecFrontmatter struct {
	Name                 string   `yaml:"name"`
	Kind                 string   `yaml:"kind"`
	Source               string   `yaml:"source"`
	Activation           string   `yaml:"activation"`
	Capabilities         []string `yaml:"capabilities"`
	OptionalCapabilities []string `yaml:"optional_capabilities"`
	RequiresApproval     bool     `yaml:"requires_approval"`
}

type pluginFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsPluginReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/plugins" || command == "/plugin"
}

func RenderPluginReport(ev Event, cfg Config) string {
	if isMCPProvenanceRequest(ev, cfg) {
		return renderMCPProvenanceReport(ev, cfg, true)
	}
	if isMCPRiskRequest(ev, cfg) {
		return renderMCPRiskReport(ev, cfg, true)
	}
	if mcpName := requestedMCPInfoName(ev, cfg); mcpName != "" {
		return renderMCPInfoReport(ev, cfg, mcpName, true)
	}
	if isMCPListRequest(ev, cfg) {
		return renderMCPReport(ev, cfg, true)
	}
	if isPluginRiskRequest(ev, cfg) {
		return renderPluginRiskReport(ev, cfg, true)
	}
	return renderPluginReport(ev, cfg, true)
}

func RenderPluginCLIReport(cfg Config) string {
	return renderPluginReport(Event{}, cfg, false)
}

func RenderPluginRiskCLIReport(cfg Config) string {
	return renderPluginRiskReport(Event{}, cfg, false)
}

func renderPluginReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectPluginSurface(cfg.Workdir)
	findings := pluginFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Plugins Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- plugins_status: `%s`\n", pluginStatus(surface, findings))
	fmt.Fprintf(&b, "- plugin_policy_path: `%s`\n", pluginPolicyPath)
	fmt.Fprintf(&b, "- plugin_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- plugin_policy_loaded_for_model: `%t`\n", pluginPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- plugin_specs_dir: `%s`\n", pluginSpecsDir)
	fmt.Fprintf(&b, "- plugin_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- plugin_specs_with_frontmatter: `%d`\n", pluginSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- plugin_capabilities: `%d`\n", pluginCapabilityCount(surface.Specs))
	fmt.Fprintf(&b, "- plugin_optional_capabilities: `%d`\n", pluginOptionalCapabilityCount(surface.Specs))
	fmt.Fprintf(&b, "- plugin_specs_requiring_approval: `%d`\n", pluginSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- plugin_specs_metadata_only: `%d`\n", pluginSpecsMetadataOnly(surface.Specs))
	fmt.Fprintf(&b, "- plugin_package_files_present: `%d`\n", len(surface.PackageFiles))
	fmt.Fprintf(&b, "- plugin_install_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- plugin_execution_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- plugin_execution_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- mcp_connection_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_plugin_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Plugins describe runtime extension intent. GitClaw treats plugin specs as reviewed metadata only: no plugin is installed, no package manager is invoked, no MCP server is connected, and no plugin, issue, comment, config, or provider payload body is printed by this report.\n\n")

	b.WriteString("### Plugin Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if pluginPolicyPathInContext() {
		b.WriteString("- `.gitclaw/PLUGINS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/PLUGINS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Plugin Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` kind=`%s` source=`%s` activation=`%s` capabilities=`%d` optional_capabilities=`%d` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Kind),
				inlineCode(spec.Source),
				inlineCode(spec.Activation),
				len(spec.Capabilities),
				len(spec.OptionalCapabilities),
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- plugin packages are not installed by `gitclaw handle`\n")
	b.WriteString("- plugin runtimes, MCP servers, webhooks, and channel bridges are not started\n")
	b.WriteString("- executable-looking plugin files are reported as ignored metadata, not run\n")
	b.WriteString("- future executable plugins require reviewed workflows, explicit permissions, approval gates, and body-free audit cards\n")

	b.WriteString("\n### Package Or Runtime Files\n")
	if len(surface.PackageFiles) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, file := range surface.PackageFiles {
			writeConfigSurfaceFile(&b, file)
		}
	}

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

func inspectPluginSurface(root string) pluginSurface {
	if root == "" {
		root = "."
	}
	surface := pluginSurface{
		Policy: inspectConfigSurfaceFile(root, pluginPolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.Specs = inspectPluginSpecs(absRoot)
	surface.PackageFiles = inspectPluginPackageFiles(absRoot)
	return surface
}

func inspectPluginSpecs(absRoot string) []pluginSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "plugins", "*.md"))
	sort.Strings(matches)
	specs := make([]pluginSpecCard, 0, len(matches))
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
		spec := pluginSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parsePluginFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Kind = strings.TrimSpace(meta.Kind)
			spec.Source = strings.TrimSpace(meta.Source)
			spec.Activation = strings.TrimSpace(meta.Activation)
			spec.Capabilities = cleanPluginList(meta.Capabilities)
			spec.OptionalCapabilities = cleanPluginList(meta.OptionalCapabilities)
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func parsePluginFrontmatter(text string) (pluginSpecFrontmatter, bool) {
	var meta pluginSpecFrontmatter
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
		return pluginSpecFrontmatter{}, false
	}
	return meta, true
}

func inspectPluginPackageFiles(absRoot string) []configSurfaceFile {
	root := filepath.Join(absRoot, ".gitclaw", "plugins")
	var files []configSurfaceFile
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		relPath := filepath.ToSlash(rel)
		if strings.HasSuffix(relPath, ".md") {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		ext := strings.ToLower(filepath.Ext(path))
		if base == "package.json" || base == "openclaw.plugin.json" || base == "manifest.yaml" || base == "manifest.yml" ||
			strings.HasPrefix(base, "handler.") || strings.HasPrefix(base, "index.") ||
			ext == ".ts" || ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".sh" || ext == ".py" || ext == ".json" || ext == ".yaml" || ext == ".yml" {
			files = append(files, inspectConfigSurfaceFile(absRoot, relPath))
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files
}

func pluginFindings(surface pluginSurface) []pluginFinding {
	var findings []pluginFinding
	if !surface.Policy.Present {
		findings = append(findings, pluginFinding{"info", "plugin_policy_not_configured", pluginPolicyPath, "no plugin policy file is configured"})
	}
	if surface.Policy.Present && !pluginPolicyPathInContext() {
		findings = append(findings, pluginFinding{"error", "plugin_policy_not_loaded", pluginPolicyPath, "plugin policy file is not in the model context allowlist"})
	}
	if len(surface.PackageFiles) > 0 {
		findings = append(findings, pluginFinding{"warning", "plugin_package_files_ignored", pluginSpecsDir, "package or runtime-looking plugin files are present but ignored by GitClaw"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, pluginFinding{"warning", "plugin_frontmatter_missing", spec.Path, "plugin spec should start with YAML frontmatter"})
		}
		if strings.TrimSpace(spec.Kind) == "" {
			findings = append(findings, pluginFinding{"warning", "plugin_kind_missing", spec.Path, "plugin spec should declare a kind such as provider, tool, channel, hook, mcp, or bundle"})
		}
		if strings.TrimSpace(spec.Source) == "" {
			findings = append(findings, pluginFinding{"warning", "plugin_source_missing", spec.Path, "plugin spec should declare a reviewed source"})
		}
		if len(spec.Capabilities) == 0 {
			findings = append(findings, pluginFinding{"warning", "plugin_capabilities_missing", spec.Path, "plugin spec should declare at least one capability"})
		}
		if !strings.EqualFold(spec.Activation, "metadata-only") {
			findings = append(findings, pluginFinding{"warning", "plugin_activation_not_metadata_only", spec.Path, "GitClaw v1 only supports metadata-only plugin specs"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, pluginFinding{"warning", "plugin_approval_gate_missing", spec.Path, "plugin spec should require approval before side effects or new tool exposure"})
		}
	}
	return findings
}

func pluginStatus(surface pluginSurface, findings []pluginFinding) string {
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

func pluginPolicyLoadedForModel(surface pluginSurface) bool {
	return surface.Policy.Present && pluginPolicyPathInContext()
}

func pluginPolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == pluginPolicyPath {
			return true
		}
	}
	return false
}

func cleanPluginList(values []string) []string {
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

func pluginSpecsWithFrontmatter(specs []pluginSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func pluginCapabilityCount(specs []pluginSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.Capabilities)
	}
	return count
}

func pluginOptionalCapabilityCount(specs []pluginSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.OptionalCapabilities)
	}
	return count
}

func pluginSpecsRequiringApproval(specs []pluginSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func pluginSpecsMetadataOnly(specs []pluginSpecCard) int {
	count := 0
	for _, spec := range specs {
		if strings.EqualFold(spec.Activation, "metadata-only") {
			count++
		}
	}
	return count
}

func isPluginRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/plugins" && command != "/plugin" {
		return false
	}
	return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
}
