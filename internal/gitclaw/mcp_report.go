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

const mcpSpecsDir = ".gitclaw/mcp"

type mcpSpecDocument struct {
	Name            string
	Description     string
	Path            string
	Body            string
	Transport       string
	Source          string
	Activation      string
	Command         string
	Args            []string
	URL             string
	ToolAllowlist   []string
	ToolDenylist    []string
	RequiresSecrets []string
	EnvPassthrough  []string
	Resources       bool
	Prompts         bool
	ParseError      string
}

type MCPSpecCard struct {
	Name            string
	Path            string
	Description     bool
	Transport       string
	Source          string
	Activation      string
	CommandPresent  bool
	CommandSHA      string
	ArgsCount       int
	ArgsSHA         string
	URLPresent      bool
	URLSHA          string
	ToolAllowlist   []string
	ToolDenylist    []string
	RequiresSecrets []string
	EnvPassthrough  []string
	Resources       bool
	Prompts         bool
	Bytes           int
	Lines           int
	SHA             string
	ParseError      string
	RiskFindings    []PluginRiskFinding
}

type MCPReport struct {
	Status                       string
	Specs                        int
	ParsedSpecs                  int
	SpecsWithCommand             int
	SpecsWithURL                 int
	SpecsWithToolAllowlist       int
	ToolAllowlistRefs            int
	ToolDenylistRefs             int
	RequiredSecretRefs           int
	EnvPassthroughRefs           int
	SpecsWithResourcesEnabled    int
	SpecsWithPromptsEnabled      int
	SpecsWithRiskFindings        int
	Findings                     []PluginRiskFinding
	HighRiskFindings             int
	WarningRiskFindings          int
	InfoRiskFindings             int
	MCPConnectionSupported       bool
	MCPServerLaunchAllowed       bool
	MCPToolExposureAllowed       bool
	DynamicToolDiscoveryAllowed  bool
	RepositoryMutationAllowed    bool
	RawMCPBodiesIncluded         bool
	RawCommandArgsIncluded       bool
	CredentialValuesIncluded     bool
	LLME2ERequiredAfterMCPChange bool
	Cards                        []MCPSpecCard
}

func RenderMCPCLIReport(cfg Config) string {
	return renderMCPReport(Event{}, cfg, false)
}

func RenderMCPRiskCLIReport(cfg Config) string {
	return renderMCPRiskReport(Event{}, cfg, false)
}

func RenderMCPInfoCLIReport(cfg Config, name string) string {
	return renderMCPInfoReport(Event{}, cfg, name, false)
}

func isMCPProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 &&
		(fields[0] == "/plugins" || fields[0] == "/plugin") &&
		(strings.EqualFold(fields[1], "mcp") || strings.EqualFold(fields[1], "mcps")) &&
		(strings.EqualFold(fields[2], "provenance") ||
			strings.EqualFold(fields[2], "history") ||
			strings.EqualFold(fields[2], "timeline"))
}

func renderMCPReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildMCPReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw MCP Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeMCPHeader(&b, ev, includeIssue)
	writeMCPSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("MCP specs are repo-reviewed capability metadata. GitClaw v1 inventories them without launching servers, connecting clients, exposing MCP tools to the model, or printing raw spec bodies, command args, URLs, credentials, issue bodies, comments, prompts, or provider payloads.\n\n")

	b.WriteString("### MCP Specs\n")
	writeMCPCards(&b, report.Cards)
	return strings.TrimSpace(b.String())
}

func renderMCPRiskReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildMCPReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw MCP Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeMCPHeader(&b, ev, includeIssue)
	writeMCPSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans repo-local MCP server specs for over-broad tool exposure, unsafe activation, command/url launch surfaces, env passthrough, prompt/resource exposure, prompt-boundary overrides, credential material, host execution, repository mutation, remote exfiltration, and unbounded loops. It reports only metadata, counts, paths, risk codes, severities, and hashes.\n\n")

	b.WriteString("### MCP Risk Cards\n")
	writeMCPCards(&b, report.Cards)

	b.WriteString("\n### Risk Findings\n")
	writeMCPRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func renderMCPInfoReport(ev Event, cfg Config, name string, includeIssue bool) string {
	report := BuildMCPReport(cfg)
	normalized := normalizeMCPName(name)
	matches := matchingMCPCards(report.Cards, normalized)
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw MCP Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeMCPHeader(&b, ev, includeIssue)
	fmt.Fprintf(&b, "- requested_mcp_sha256_12: `%s`\n", shortDocumentHash(name))
	fmt.Fprintf(&b, "- normalized_mcp: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- mcp_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- mcp_specs: `%d`\n", report.Specs)
	fmt.Fprintf(&b, "- matched_mcp_specs: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- raw_requested_mcp_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_mcp_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_command_args_included: `%t`\n", false)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report focuses one MCP spec without launching it. It shows transport, activation, tool filters, secret-name refs, command/url hashes, and risk codes only.\n\n")

	b.WriteString("### Matches\n")
	writeMCPCards(&b, matches)

	b.WriteString("\n### Risk Findings For Matches\n")
	var findings []PluginRiskFinding
	for _, card := range matches {
		findings = append(findings, card.RiskFindings...)
	}
	sortPluginRiskFindings(findings)
	writeMCPRiskFindings(&b, findings)

	if len(matches) == 0 {
		b.WriteString("\n### Available MCP Specs\n")
		writeMCPCards(&b, report.Cards)
	}
	return strings.TrimSpace(b.String())
}

func BuildMCPReport(cfg Config) MCPReport {
	docs := discoverMCPSpecs(cfg.Workdir)
	cards := summarizeMCPSpecs(docs)
	report := MCPReport{
		Status:                       "ok",
		Specs:                        len(cards),
		MCPConnectionSupported:       false,
		MCPServerLaunchAllowed:       false,
		MCPToolExposureAllowed:       false,
		DynamicToolDiscoveryAllowed:  false,
		RepositoryMutationAllowed:    false,
		RawMCPBodiesIncluded:         false,
		RawCommandArgsIncluded:       false,
		CredentialValuesIncluded:     false,
		LLME2ERequiredAfterMCPChange: true,
		Cards:                        cards,
	}
	for _, card := range cards {
		if card.ParseError == "" {
			report.ParsedSpecs++
		}
		if card.CommandPresent {
			report.SpecsWithCommand++
		}
		if card.URLPresent {
			report.SpecsWithURL++
		}
		if len(card.ToolAllowlist) > 0 {
			report.SpecsWithToolAllowlist++
		}
		report.ToolAllowlistRefs += len(card.ToolAllowlist)
		report.ToolDenylistRefs += len(card.ToolDenylist)
		report.RequiredSecretRefs += len(card.RequiresSecrets)
		report.EnvPassthroughRefs += len(card.EnvPassthrough)
		if card.Resources {
			report.SpecsWithResourcesEnabled++
		}
		if card.Prompts {
			report.SpecsWithPromptsEnabled++
		}
		if len(card.RiskFindings) > 0 {
			report.SpecsWithRiskFindings++
		}
		report.Findings = append(report.Findings, card.RiskFindings...)
	}
	sortPluginRiskFindings(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func discoverMCPSpecs(root string) []mcpSpecDocument {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	var docs []mcpSpecDocument
	seen := map[string]bool{}
	for _, pattern := range []string{".gitclaw/mcp/*.yml", ".gitclaw/mcp/*.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(absRoot, filepath.FromSlash(pattern)))
		for _, match := range matches {
			realPath, err := filepath.EvalSymlinks(match)
			if err != nil {
				realPath = match
			}
			seenKey := strings.ToLower(realPath)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			rel, err := filepath.Rel(absRoot, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			body, err := os.ReadFile(match)
			if err != nil {
				continue
			}
			docs = append(docs, parseMCPSpecDocument(rel, string(body)))
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs
}

func parseMCPSpecDocument(path, body string) mcpSpecDocument {
	name := normalizeMCPName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	var file struct {
		Name            string   `yaml:"name"`
		Description     string   `yaml:"description"`
		Transport       string   `yaml:"transport"`
		Source          string   `yaml:"source"`
		Activation      string   `yaml:"activation"`
		Command         string   `yaml:"command"`
		Args            []string `yaml:"args"`
		URL             string   `yaml:"url"`
		ToolAllowlist   []string `yaml:"tool_allowlist"`
		ToolDenylist    []string `yaml:"tool_denylist"`
		RequiresSecrets []string `yaml:"requires_secrets"`
		EnvPassthrough  []string `yaml:"env_passthrough"`
		Resources       bool     `yaml:"resources_enabled"`
		Prompts         bool     `yaml:"prompts_enabled"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.KnownFields(true)
	parseError := ""
	if err := decoder.Decode(&file); err != nil {
		parseError = err.Error()
	}
	if value := normalizeMCPName(file.Name); value != "" {
		name = value
	}
	transport := strings.ToLower(strings.TrimSpace(file.Transport))
	if transport == "" {
		transport = "unspecified"
	}
	activation := strings.ToLower(strings.TrimSpace(file.Activation))
	if activation == "" {
		activation = "metadata-only"
	}
	return mcpSpecDocument{
		Name:            name,
		Description:     strings.TrimSpace(file.Description),
		Path:            path,
		Body:            body,
		Transport:       transport,
		Source:          strings.TrimSpace(file.Source),
		Activation:      activation,
		Command:         strings.TrimSpace(file.Command),
		Args:            cleanMCPList(file.Args),
		URL:             strings.TrimSpace(file.URL),
		ToolAllowlist:   cleanMCPList(file.ToolAllowlist),
		ToolDenylist:    cleanMCPList(file.ToolDenylist),
		RequiresSecrets: cleanMCPList(file.RequiresSecrets),
		EnvPassthrough:  cleanMCPList(file.EnvPassthrough),
		Resources:       file.Resources,
		Prompts:         file.Prompts,
		ParseError:      parseError,
	}
}

func summarizeMCPSpecs(docs []mcpSpecDocument) []MCPSpecCard {
	cards := make([]MCPSpecCard, 0, len(docs))
	for _, doc := range docs {
		findings := scanMCPSpecRiskFindings(doc)
		cards = append(cards, MCPSpecCard{
			Name:            doc.Name,
			Path:            doc.Path,
			Description:     doc.Description != "",
			Transport:       doc.Transport,
			Source:          doc.Source,
			Activation:      doc.Activation,
			CommandPresent:  doc.Command != "",
			CommandSHA:      shortDocumentHash(doc.Command),
			ArgsCount:       len(doc.Args),
			ArgsSHA:         shortDocumentHash(strings.Join(doc.Args, "\n")),
			URLPresent:      doc.URL != "",
			URLSHA:          shortDocumentHash(doc.URL),
			ToolAllowlist:   append([]string(nil), doc.ToolAllowlist...),
			ToolDenylist:    append([]string(nil), doc.ToolDenylist...),
			RequiresSecrets: append([]string(nil), doc.RequiresSecrets...),
			EnvPassthrough:  append([]string(nil), doc.EnvPassthrough...),
			Resources:       doc.Resources,
			Prompts:         doc.Prompts,
			Bytes:           len(doc.Body),
			Lines:           lineCount(doc.Body),
			SHA:             shortDocumentHash(doc.Body),
			ParseError:      doc.ParseError,
			RiskFindings:    findings,
		})
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Path < cards[j].Path })
	return cards
}

func scanMCPSpecRiskFindings(doc mcpSpecDocument) []PluginRiskFinding {
	findings := scanPluginRiskText("mcp-spec", doc.Name, doc.Path, "body", doc.Body)
	add := func(severity, code, category, field, value string) {
		findings = append(findings, PluginRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Kind:     "mcp-spec",
			Name:     doc.Name,
			Path:     doc.Path,
			Field:    field,
			Line:     0,
			LineSHA:  shortDocumentHash(value),
		})
	}
	if strings.TrimSpace(doc.ParseError) != "" {
		add("warning", "mcp_yaml_parse_error", "mcp-schema", "yaml", doc.ParseError)
	}
	if doc.Activation != "metadata-only" && doc.Activation != "disabled" {
		add("warning", "mcp_activation_not_metadata_only", "runtime-extension", "activation", doc.Activation)
	}
	if len(doc.ToolAllowlist) == 0 {
		add("warning", "mcp_missing_tool_allowlist", "tool-exposure", "tool_allowlist", doc.Path)
	}
	if doc.Command != "" {
		add("warning", "mcp_command_declared", "host-execution", "command", doc.Command)
	}
	if doc.URL != "" {
		add("warning", "mcp_remote_endpoint_declared", "network-exposure", "url", doc.URL)
	}
	if doc.Resources {
		add("warning", "mcp_resources_enabled", "prompt-surface", "resources_enabled", doc.Path)
	}
	if doc.Prompts {
		add("warning", "mcp_prompts_enabled", "prompt-surface", "prompts_enabled", doc.Path)
	}
	for _, env := range doc.EnvPassthrough {
		if env == "*" || strings.EqualFold(env, "all") {
			add("high", "mcp_unbounded_env_passthrough", "credential-handling", "env_passthrough", env)
		}
	}
	for _, tool := range doc.ToolAllowlist {
		if mcpToolLooksMutating(tool) {
			add("high", "mcp_mutating_tool_allowlisted", "tool-exposure", "tool_allowlist", tool)
		}
	}
	sortPluginRiskFindings(findings)
	return findings
}

func mcpToolLooksMutating(tool string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	for _, phrase := range []string{"write", "create", "update", "delete", "remove", "admin", "execute", "run", "shell", "deploy", "send", "post", "push", "merge"} {
		if strings.Contains(tool, phrase) {
			return true
		}
	}
	return false
}

func normalizeMCPName(value string) string {
	return normalizeSkillBundleName(value)
}

func cleanMCPList(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return uniqueSortedStrings(out)
}

func matchingMCPCards(cards []MCPSpecCard, name string) []MCPSpecCard {
	name = normalizeMCPName(name)
	if name == "" {
		return nil
	}
	var matches []MCPSpecCard
	for _, card := range cards {
		pathName := normalizeMCPName(strings.TrimSuffix(filepath.Base(card.Path), filepath.Ext(card.Path)))
		if normalizeMCPName(card.Name) == name || pathName == name {
			matches = append(matches, card)
		}
	}
	return matches
}

func writeMCPHeader(b *strings.Builder, ev Event, includeIssue bool) {
	if includeIssue {
		fmt.Fprintf(b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(b, "- scope: `%s`\n", "local-cli")
	}
}

func writeMCPSummary(b *strings.Builder, report MCPReport) {
	fmt.Fprintf(b, "- mcp_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- mcp_specs_dir: `%s`\n", mcpSpecsDir)
	fmt.Fprintf(b, "- mcp_specs: `%d`\n", report.Specs)
	fmt.Fprintf(b, "- parsed_mcp_specs: `%d`\n", report.ParsedSpecs)
	fmt.Fprintf(b, "- mcp_specs_with_command: `%d`\n", report.SpecsWithCommand)
	fmt.Fprintf(b, "- mcp_specs_with_url: `%d`\n", report.SpecsWithURL)
	fmt.Fprintf(b, "- mcp_specs_with_tool_allowlist: `%d`\n", report.SpecsWithToolAllowlist)
	fmt.Fprintf(b, "- mcp_tool_allowlist_refs: `%d`\n", report.ToolAllowlistRefs)
	fmt.Fprintf(b, "- mcp_tool_denylist_refs: `%d`\n", report.ToolDenylistRefs)
	fmt.Fprintf(b, "- mcp_required_secret_refs: `%d`\n", report.RequiredSecretRefs)
	fmt.Fprintf(b, "- mcp_env_passthrough_refs: `%d`\n", report.EnvPassthroughRefs)
	fmt.Fprintf(b, "- mcp_specs_with_resources_enabled: `%d`\n", report.SpecsWithResourcesEnabled)
	fmt.Fprintf(b, "- mcp_specs_with_prompts_enabled: `%d`\n", report.SpecsWithPromptsEnabled)
	fmt.Fprintf(b, "- mcp_specs_with_risk_findings: `%d`\n", report.SpecsWithRiskFindings)
	fmt.Fprintf(b, "- mcp_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- mcp_connection_supported: `%t`\n", report.MCPConnectionSupported)
	fmt.Fprintf(b, "- mcp_server_launch_allowed: `%t`\n", report.MCPServerLaunchAllowed)
	fmt.Fprintf(b, "- mcp_tool_exposure_allowed: `%t`\n", report.MCPToolExposureAllowed)
	fmt.Fprintf(b, "- dynamic_tool_discovery_allowed: `%t`\n", report.DynamicToolDiscoveryAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_mcp_bodies_included: `%t`\n", report.RawMCPBodiesIncluded)
	fmt.Fprintf(b, "- raw_command_args_included: `%t`\n", report.RawCommandArgsIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_mcp_change: `%t`\n", report.LLME2ERequiredAfterMCPChange)
}

func writeMCPCards(b *strings.Builder, cards []MCPSpecCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		commandSHA := "none"
		if card.CommandPresent {
			commandSHA = card.CommandSHA
		}
		argsSHA := "none"
		if card.ArgsCount > 0 {
			argsSHA = card.ArgsSHA
		}
		urlSHA := "none"
		if card.URLPresent {
			urlSHA = card.URLSHA
		}
		fmt.Fprintf(b, "- mcp_name=`%s` path=`%s` transport=`%s` source=`%s` activation=`%s` description=`%t` command_present=`%t` command_sha256_12=`%s` args_count=`%d` args_sha256_12=`%s` url_present=`%t` url_sha256_12=`%s` tool_allowlist=`%s` tool_denylist=`%s` requires_secrets=`%s` env_passthrough=`%s` resources_enabled=`%t` prompts_enabled=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			inlineCode(card.Name),
			card.Path,
			inlineCode(card.Transport),
			inlineCode(card.Source),
			inlineCode(card.Activation),
			card.Description,
			card.CommandPresent,
			commandSHA,
			card.ArgsCount,
			argsSHA,
			card.URLPresent,
			urlSHA,
			inlineListOrNone(card.ToolAllowlist),
			inlineListOrNone(card.ToolDenylist),
			inlineListOrNone(card.RequiresSecrets),
			inlineListOrNone(card.EnvPassthrough),
			card.Resources,
			card.Prompts,
			card.Bytes,
			card.Lines,
			card.SHA,
			len(card.RiskFindings),
			pluginRiskMaxSeverity(card.RiskFindings),
			inlineListOrNone(pluginRiskCodes(card.RiskFindings)),
		)
	}
}

func writeMCPRiskFindings(b *strings.Builder, findings []PluginRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` mcp=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			inlineCode(finding.Name),
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func isMCPListRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		(fields[0] == "/plugins" || fields[0] == "/plugin") &&
		(strings.EqualFold(fields[1], "mcp") || strings.EqualFold(fields[1], "mcps")) &&
		(len(fields) == 2 || strings.EqualFold(fields[2], "list"))
}

func isMCPRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 &&
		(fields[0] == "/plugins" || fields[0] == "/plugin") &&
		(strings.EqualFold(fields[1], "mcp") || strings.EqualFold(fields[1], "mcps")) &&
		(strings.EqualFold(fields[2], "risk") || strings.EqualFold(fields[2], "risk-audit"))
}

func requestedMCPInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 4 {
		return ""
	}
	if fields[0] != "/plugins" && fields[0] != "/plugin" {
		return ""
	}
	if !strings.EqualFold(fields[1], "mcp") && !strings.EqualFold(fields[1], "mcps") {
		return ""
	}
	if !strings.EqualFold(fields[2], "info") && !strings.EqualFold(fields[2], "show") {
		return ""
	}
	return normalizeMCPName(fields[3])
}
