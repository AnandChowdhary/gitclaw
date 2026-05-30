package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PluginRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Name     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type PluginRiskReport struct {
	Status                              string
	VerificationScope                   string
	PluginPolicyPresent                 bool
	PluginPolicyLoadedForModel          bool
	PluginSpecs                         int
	ScannedPluginSpecs                  int
	PluginCapabilities                  int
	PluginOptionalCapabilities          int
	PluginSpecsRequiringApproval        int
	PluginSpecsMetadataOnly             int
	PackageFilesPresent                 int
	ScannedPackageFiles                 int
	SurfacesWithRiskFindings            int
	Findings                            []PluginRiskFinding
	HighRiskFindings                    int
	WarningRiskFindings                 int
	InfoRiskFindings                    int
	PluginInstallSupported              bool
	PluginExecutionSupported            bool
	PluginExecutionAllowed              bool
	MCPConnectionAllowed                bool
	RepositoryMutationAllowed           bool
	RawPluginBodiesIncluded             bool
	RawPackageBodiesIncluded            bool
	CredentialValuesIncluded            bool
	LLME2ERequiredAfterPluginRiskChange bool
}

type pluginRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var pluginTextRiskRules = []pluginRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
			"developer message",
		},
	},
	{
		Severity: "high",
		Code:     "secret_exfiltration_instruction",
		Category: "data-exfiltration",
		Any: []string{
			"exfiltrate",
			"leak secrets",
			"send secrets",
			"upload secrets",
			"steal secrets",
		},
	},
	{
		Severity: "high",
		Code:     "credential_material_in_plugin",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"telegram_bot_token=",
			"slack_bot_token=",
			"slack_app_token=",
			"xoxb-",
			"xapp-",
			"api_key=",
			"private_key=",
			"-----begin private key-----",
			"-----begin openssh private key-----",
		},
	},
	{
		Severity: "high",
		Code:     "untrusted_issue_body_execution",
		Category: "host-execution",
		Any: []string{
			"eval \"$issue_body",
			"eval \"${{ github.event.issue.body",
			"bash -c \"$issue_body",
			"bash -c \"${{ github.event.issue.body",
			"sh -c \"$issue_body",
			"sh -c \"${{ github.event.issue.body",
			"python -c \"$issue_body",
			"python -c \"${{ github.event.issue.body",
			"node -e \"$issue_body",
			"node -e \"${{ github.event.issue.body",
		},
	},
	{
		Severity: "high",
		Code:     "automatic_plugin_install",
		Category: "supply-chain",
		Any: []string{
			"npm install",
			"pip install",
			"curl | sh",
			"curl -fs",
			"wget | sh",
			"git clone",
			"openclaw plugins install",
			"clawhub install",
			"clawhub:",
		},
	},
	{
		Severity: "high",
		Code:     "mcp_runtime_connection",
		Category: "runtime-extension",
		Any: []string{
			"connect to mcp",
			"start mcp",
			"stdio mcp",
			"websocket mcp",
		},
	},
	{
		Severity: "high",
		Code:     "raw_payload_logged",
		Category: "body-leakage",
		Any: []string{
			"echo \"$gitclaw_plugin_payload",
			"echo \"$issue_body",
			"echo \"${{ github.event.issue.body",
			"printf \"%s\" \"$gitclaw_plugin_payload",
			"printf '%s' \"$gitclaw_plugin_payload",
			"printenv",
		},
	},
	{
		Severity: "warning",
		Code:     "external_webhook_bridge",
		Category: "network-exposure",
		Any: []string{
			"webhook.site",
			"requestbin",
			"ngrok",
			"public webhook",
			"unauthenticated webhook",
		},
	},
	{
		Severity: "warning",
		Code:     "unreviewed_repository_mutation",
		Category: "write-authority",
		Any: []string{
			"git push",
			"git commit",
			"gh issue edit",
			"gh workflow run",
			"write files without review",
			"modify files without review",
			"commit directly",
			"push directly",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_plugin_loop",
		Category: "runtime-amplification",
		Any: []string{
			"while true",
			"retry forever",
			"loop forever",
			"sleep infinity",
			"never stop",
			"continue indefinitely",
		},
	},
}

func renderPluginRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectPluginSurface(cfg.Workdir)
	report := BuildPluginRiskReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Plugin Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writePluginRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans declarative plugin policy/spec files and ignored package/runtime files for prompt-boundary, credential, supply-chain install, MCP/runtime connection, host-execution, raw payload logging, external webhook, repository mutation, and unbounded-loop risks. It reports metadata, paths, risk codes, severities, and hashes only; plugin bodies, package bodies, issue bodies, comments, provider payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Plugin Policy Risk Card\n")
	writePluginPolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Plugin Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`plugin-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writePluginSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Package Runtime Risk Cards\n")
	if len(surface.PackageFiles) == 0 {
		b.WriteString("- kind=`package-runtime` none\n")
	} else {
		for _, file := range surface.PackageFiles {
			writePluginPackageRiskCard(&b, cfg.Workdir, file)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writePluginRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPluginRiskReport(cfg Config) PluginRiskReport {
	surface := inspectPluginSurface(cfg.Workdir)
	report := PluginRiskReport{
		Status:                              "ok",
		VerificationScope:                   "repo_reviewed_plugin_metadata",
		PluginPolicyPresent:                 surface.Policy.Present,
		PluginPolicyLoadedForModel:          pluginPolicyLoadedForModel(surface),
		PluginSpecs:                         len(surface.Specs),
		PluginCapabilities:                  pluginCapabilityCount(surface.Specs),
		PluginOptionalCapabilities:          pluginOptionalCapabilityCount(surface.Specs),
		PluginSpecsRequiringApproval:        pluginSpecsRequiringApproval(surface.Specs),
		PluginSpecsMetadataOnly:             pluginSpecsMetadataOnly(surface.Specs),
		PackageFilesPresent:                 len(surface.PackageFiles),
		PluginInstallSupported:              false,
		PluginExecutionSupported:            false,
		PluginExecutionAllowed:              false,
		MCPConnectionAllowed:                false,
		RepositoryMutationAllowed:           false,
		RawPluginBodiesIncluded:             false,
		RawPackageBodiesIncluded:            false,
		CredentialValuesIncluded:            false,
		LLME2ERequiredAfterPluginRiskChange: true,
	}
	report.Findings = append(report.Findings, scanPluginPolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedPluginSpecs++
		report.Findings = append(report.Findings, scanPluginSpecRiskFindings(cfg.Workdir, spec)...)
	}
	for _, file := range surface.PackageFiles {
		report.ScannedPackageFiles++
		report.Findings = append(report.Findings, scanPluginPackageRiskFindings(cfg.Workdir, file)...)
	}
	sortPluginRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = pluginRiskSurfaceCount(report.Findings)
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

func writePluginRiskSummary(b *strings.Builder, report PluginRiskReport) {
	fmt.Fprintf(b, "- plugin_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- plugin_policy_present: `%t`\n", report.PluginPolicyPresent)
	fmt.Fprintf(b, "- plugin_policy_loaded_for_model: `%t`\n", report.PluginPolicyLoadedForModel)
	fmt.Fprintf(b, "- plugin_specs: `%d`\n", report.PluginSpecs)
	fmt.Fprintf(b, "- scanned_plugin_specs: `%d`\n", report.ScannedPluginSpecs)
	fmt.Fprintf(b, "- plugin_capabilities: `%d`\n", report.PluginCapabilities)
	fmt.Fprintf(b, "- plugin_optional_capabilities: `%d`\n", report.PluginOptionalCapabilities)
	fmt.Fprintf(b, "- plugin_specs_requiring_approval: `%d`\n", report.PluginSpecsRequiringApproval)
	fmt.Fprintf(b, "- plugin_specs_metadata_only: `%d`\n", report.PluginSpecsMetadataOnly)
	fmt.Fprintf(b, "- package_files_present: `%d`\n", report.PackageFilesPresent)
	fmt.Fprintf(b, "- scanned_package_files: `%d`\n", report.ScannedPackageFiles)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- plugin_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- plugin_install_supported: `%t`\n", report.PluginInstallSupported)
	fmt.Fprintf(b, "- plugin_execution_supported: `%t`\n", report.PluginExecutionSupported)
	fmt.Fprintf(b, "- plugin_execution_allowed: `%t`\n", report.PluginExecutionAllowed)
	fmt.Fprintf(b, "- mcp_connection_allowed: `%t`\n", report.MCPConnectionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_plugin_bodies_included: `%t`\n", report.RawPluginBodiesIncluded)
	fmt.Fprintf(b, "- raw_package_bodies_included: `%t`\n", report.RawPackageBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_plugin_risk_change: `%t`\n", report.LLME2ERequiredAfterPluginRiskChange)
}

func writePluginPolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanPluginPolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`plugin-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			pluginRiskMaxSeverity(findings),
			inlineListOrNone(pluginRiskCodes(findings)),
			inlineListOrNone(pluginRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`plugin-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		pluginPolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		pluginRiskMaxSeverity(findings),
		inlineListOrNone(pluginRiskCodes(findings)),
		inlineListOrNone(pluginRiskLineHashes(findings)),
	)
}

func writePluginSpecRiskCard(b *strings.Builder, root string, spec pluginSpecCard) {
	findings := scanPluginSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`plugin-spec` name=`%s` path=`%s` frontmatter=`%t` kind_field=`%s` source=`%s` activation=`%s` capabilities=`%d` optional_capabilities=`%d` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Kind),
		inlineCode(spec.Source),
		inlineCode(spec.Activation),
		len(spec.Capabilities),
		len(spec.OptionalCapabilities),
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		pluginRiskMaxSeverity(findings),
		inlineListOrNone(pluginRiskCodes(findings)),
		inlineListOrNone(pluginRiskLineHashes(findings)),
	)
}

func writePluginPackageRiskCard(b *strings.Builder, root string, file configSurfaceFile) {
	findings := scanPluginPackageRiskFindings(root, file)
	fmt.Fprintf(
		b,
		"- kind=`package-runtime` path=`%s` present=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		file.Path,
		file.Present,
		file.Bytes,
		file.Lines,
		file.SHA,
		len(findings),
		pluginRiskMaxSeverity(findings),
		inlineListOrNone(pluginRiskCodes(findings)),
		inlineListOrNone(pluginRiskLineHashes(findings)),
	)
}

func scanPluginPolicyRiskFindings(root string, policy configSurfaceFile) []PluginRiskFinding {
	var findings []PluginRiskFinding
	if !policy.Present {
		findings = append(findings, PluginRiskFinding{
			Severity: "info",
			Code:     "plugin_policy_not_configured",
			Category: "policy",
			Kind:     "plugin-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !pluginPolicyPathInContext() {
		findings = append(findings, PluginRiskFinding{
			Severity: "high",
			Code:     "plugin_policy_not_loaded",
			Category: "context",
			Kind:     "plugin-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanPluginRiskText("plugin-policy", "policy", policy.Path, "body", readPluginRiskBody(root, policy.Path))...)
	sortPluginRiskFindings(findings)
	return findings
}

func scanPluginSpecRiskFindings(root string, spec pluginSpecCard) []PluginRiskFinding {
	var findings []PluginRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, pluginSpecMetadataRiskFinding("warning", "plugin_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if strings.TrimSpace(spec.Kind) == "" {
		findings = append(findings, pluginSpecMetadataRiskFinding("warning", "plugin_kind_missing", "metadata", spec, "kind"))
	}
	if strings.TrimSpace(spec.Source) == "" {
		findings = append(findings, pluginSpecMetadataRiskFinding("warning", "plugin_source_missing", "metadata", spec, "source"))
	}
	if len(spec.Capabilities) == 0 {
		findings = append(findings, pluginSpecMetadataRiskFinding("warning", "plugin_capabilities_missing", "metadata", spec, "capabilities"))
	}
	if !strings.EqualFold(spec.Activation, "metadata-only") {
		findings = append(findings, pluginSpecMetadataRiskFinding("warning", "plugin_activation_not_metadata_only", "runtime-boundary", spec, "activation"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, pluginSpecMetadataRiskFinding("warning", "plugin_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanPluginRiskText("plugin-spec", spec.Name, spec.Path, "body", readPluginRiskBody(root, spec.Path))...)
	sortPluginRiskFindings(findings)
	return findings
}

func scanPluginPackageRiskFindings(root string, file configSurfaceFile) []PluginRiskFinding {
	findings := []PluginRiskFinding{{
		Severity: "warning",
		Code:     "plugin_package_file_present",
		Category: "runtime-boundary",
		Kind:     "package-runtime",
		Name:     filepath.Base(filepath.FromSlash(file.Path)),
		Path:     file.Path,
		Field:    "present",
		Line:     0,
		LineSHA:  shortDocumentHash(file.Path + ":package-runtime"),
	}}
	findings = append(findings, scanPluginRiskText("package-runtime", filepath.Base(filepath.FromSlash(file.Path)), file.Path, "body", readPluginRiskBody(root, file.Path))...)
	sortPluginRiskFindings(findings)
	return findings
}

func pluginSpecMetadataRiskFinding(severity, code, category string, spec pluginSpecCard, field string) PluginRiskFinding {
	return PluginRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "plugin-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func scanPluginRiskText(kind, name, path, field, body string) []PluginRiskFinding {
	var findings []PluginRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range pluginTextRiskRules {
			if !pluginRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, PluginRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Name:     name,
				Path:     path,
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortPluginRiskFindings(findings)
	return findings
}

func pluginRiskRuleMatches(lowerLine string, rule pluginRiskRule) bool {
	for _, required := range rule.All {
		if !strings.Contains(lowerLine, required) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func readPluginRiskBody(root, relPath string) string {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	if err != nil {
		return ""
	}
	return string(body)
}

func writePluginRiskFindings(b *strings.Builder, findings []PluginRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` name=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			finding.Name,
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func pluginRiskSurfaceCount(findings []PluginRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Name + "\x00" + finding.Path
		if key == "\x00\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func pluginRiskCodes(findings []PluginRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func pluginRiskLineHashes(findings []PluginRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	sort.Strings(hashes)
	return hashes
}

func pluginRiskMaxSeverity(findings []PluginRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if pluginRiskSeverityRank(finding.Severity) > pluginRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func pluginRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortPluginRiskFindings(findings []PluginRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if pluginRiskSeverityRank(findings[i].Severity) != pluginRiskSeverityRank(findings[j].Severity) {
			return pluginRiskSeverityRank(findings[i].Severity) > pluginRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Name != findings[j].Name {
			return findings[i].Name < findings[j].Name
		}
		if findings[i].Field != findings[j].Field {
			return findings[i].Field < findings[j].Field
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Code < findings[j].Code
	})
}
