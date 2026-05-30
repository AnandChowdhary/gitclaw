package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type HookRiskFinding struct {
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

type HookRiskReport struct {
	Status                            string
	VerificationScope                 string
	HookPolicyPresent                 bool
	HookPolicyLoadedForModel          bool
	HookSpecs                         int
	ScannedHookSpecs                  int
	HookEvents                        int
	HookSpecsRequiringApproval        int
	HookSpecsAuditOnly                int
	ExecutableHandlersPresent         int
	ScannedExecutableHandlers         int
	SurfacesWithRiskFindings          int
	Findings                          []HookRiskFinding
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	HookExecutionSupported            bool
	HookExecutionAllowed              bool
	RepositoryMutationAllowed         bool
	RawHookBodiesIncluded             bool
	RawHandlerBodiesIncluded          bool
	CredentialValuesIncluded          bool
	LLME2ERequiredAfterHookRiskChange bool
}

type hookRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var hookTextRiskRules = []hookRiskRule{
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
		Code:     "credential_material_in_hook",
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
		Code:     "raw_payload_logged",
		Category: "body-leakage",
		Any: []string{
			"echo \"$gitclaw_hook_payload",
			"echo \"$issue_body",
			"echo \"${{ github.event.issue.body",
			"printf \"%s\" \"$gitclaw_hook_payload",
			"printf '%s' \"$gitclaw_hook_payload",
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
		Code:     "unbounded_hook_loop",
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

func renderHookRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectHookSurface(cfg.Workdir)
	report := BuildHookRiskReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Hook Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeHookRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans declarative hook policy/spec files and ignored executable-looking hook handlers for prompt-boundary, credential, host-execution, raw payload logging, external webhook, repository mutation, and unbounded-loop risks. It reports metadata, paths, risk codes, severities, and hashes only; hook bodies, handler bodies, issue bodies, comments, provider payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Hook Policy Risk Card\n")
	writeHookPolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Hook Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`hook-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeHookSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Executable Handler Risk Cards\n")
	if len(surface.ExecutableHandlers) == 0 {
		b.WriteString("- kind=`handler` none\n")
	} else {
		for _, handler := range surface.ExecutableHandlers {
			writeHookHandlerRiskCard(&b, cfg.Workdir, handler)
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeHookRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildHookRiskReport(cfg Config) HookRiskReport {
	surface := inspectHookSurface(cfg.Workdir)
	report := HookRiskReport{
		Status:                            "ok",
		VerificationScope:                 "repo_reviewed_hook_metadata",
		HookPolicyPresent:                 surface.Policy.Present,
		HookPolicyLoadedForModel:          hookPolicyLoadedForModel(surface),
		HookSpecs:                         len(surface.Specs),
		HookEvents:                        hookEventCount(surface.Specs),
		HookSpecsRequiringApproval:        hookSpecsRequiringApproval(surface.Specs),
		HookSpecsAuditOnly:                hookSpecsAuditOnly(surface.Specs),
		ExecutableHandlersPresent:         len(surface.ExecutableHandlers),
		HookExecutionSupported:            false,
		HookExecutionAllowed:              false,
		RepositoryMutationAllowed:         false,
		RawHookBodiesIncluded:             false,
		RawHandlerBodiesIncluded:          false,
		CredentialValuesIncluded:          false,
		LLME2ERequiredAfterHookRiskChange: true,
	}
	report.Findings = append(report.Findings, scanHookPolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedHookSpecs++
		report.Findings = append(report.Findings, scanHookSpecRiskFindings(cfg.Workdir, spec)...)
	}
	for _, handler := range surface.ExecutableHandlers {
		report.ScannedExecutableHandlers++
		report.Findings = append(report.Findings, scanHookHandlerRiskFindings(cfg.Workdir, handler)...)
	}
	sortHookRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = hookRiskSurfaceCount(report.Findings)
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

func writeHookRiskSummary(b *strings.Builder, report HookRiskReport) {
	fmt.Fprintf(b, "- hook_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- hook_policy_present: `%t`\n", report.HookPolicyPresent)
	fmt.Fprintf(b, "- hook_policy_loaded_for_model: `%t`\n", report.HookPolicyLoadedForModel)
	fmt.Fprintf(b, "- hook_specs: `%d`\n", report.HookSpecs)
	fmt.Fprintf(b, "- scanned_hook_specs: `%d`\n", report.ScannedHookSpecs)
	fmt.Fprintf(b, "- hook_events: `%d`\n", report.HookEvents)
	fmt.Fprintf(b, "- hook_specs_requiring_approval: `%d`\n", report.HookSpecsRequiringApproval)
	fmt.Fprintf(b, "- hook_specs_audit_only: `%d`\n", report.HookSpecsAuditOnly)
	fmt.Fprintf(b, "- executable_handlers_present: `%d`\n", report.ExecutableHandlersPresent)
	fmt.Fprintf(b, "- scanned_executable_handlers: `%d`\n", report.ScannedExecutableHandlers)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- hook_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- hook_execution_supported: `%t`\n", report.HookExecutionSupported)
	fmt.Fprintf(b, "- hook_execution_allowed: `%t`\n", report.HookExecutionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_hook_bodies_included: `%t`\n", report.RawHookBodiesIncluded)
	fmt.Fprintf(b, "- raw_handler_bodies_included: `%t`\n", report.RawHandlerBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_hook_risk_change: `%t`\n", report.LLME2ERequiredAfterHookRiskChange)
}

func writeHookPolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanHookPolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`hook-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			hookRiskMaxSeverity(findings),
			inlineListOrNone(hookRiskCodes(findings)),
			inlineListOrNone(hookRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`hook-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		hookPolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		hookRiskMaxSeverity(findings),
		inlineListOrNone(hookRiskCodes(findings)),
		inlineListOrNone(hookRiskLineHashes(findings)),
	)
}

func writeHookSpecRiskCard(b *strings.Builder, root string, spec hookSpecCard) {
	findings := scanHookSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`hook-spec` name=`%s` path=`%s` frontmatter=`%t` events=`%d` mode=`%s` delivery=`%s` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		len(spec.Events),
		inlineCode(spec.Mode),
		inlineCode(spec.Delivery),
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		hookRiskMaxSeverity(findings),
		inlineListOrNone(hookRiskCodes(findings)),
		inlineListOrNone(hookRiskLineHashes(findings)),
	)
}

func writeHookHandlerRiskCard(b *strings.Builder, root string, handler configSurfaceFile) {
	findings := scanHookHandlerRiskFindings(root, handler)
	fmt.Fprintf(
		b,
		"- kind=`handler` path=`%s` present=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		handler.Path,
		handler.Present,
		handler.Bytes,
		handler.Lines,
		handler.SHA,
		len(findings),
		hookRiskMaxSeverity(findings),
		inlineListOrNone(hookRiskCodes(findings)),
		inlineListOrNone(hookRiskLineHashes(findings)),
	)
}

func scanHookPolicyRiskFindings(root string, policy configSurfaceFile) []HookRiskFinding {
	var findings []HookRiskFinding
	if !policy.Present {
		findings = append(findings, HookRiskFinding{
			Severity: "info",
			Code:     "hook_policy_not_configured",
			Category: "policy",
			Kind:     "hook-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !hookPolicyPathInContext() {
		findings = append(findings, HookRiskFinding{
			Severity: "high",
			Code:     "hook_policy_not_loaded",
			Category: "context",
			Kind:     "hook-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanHookRiskText("hook-policy", "policy", policy.Path, "body", readHookRiskBody(root, policy.Path))...)
	sortHookRiskFindings(findings)
	return findings
}

func scanHookSpecRiskFindings(root string, spec hookSpecCard) []HookRiskFinding {
	var findings []HookRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, hookSpecMetadataRiskFinding("warning", "hook_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if len(spec.Events) == 0 {
		findings = append(findings, hookSpecMetadataRiskFinding("warning", "hook_events_missing", "metadata", spec, "events"))
	}
	if !strings.EqualFold(spec.Mode, "audit-only") {
		findings = append(findings, hookSpecMetadataRiskFinding("warning", "hook_mode_not_audit_only", "runtime-boundary", spec, "mode"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, hookSpecMetadataRiskFinding("warning", "hook_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanHookRiskText("hook-spec", spec.Name, spec.Path, "body", readHookRiskBody(root, spec.Path))...)
	sortHookRiskFindings(findings)
	return findings
}

func scanHookHandlerRiskFindings(root string, handler configSurfaceFile) []HookRiskFinding {
	findings := []HookRiskFinding{{
		Severity: "warning",
		Code:     "executable_handler_present",
		Category: "runtime-boundary",
		Kind:     "handler",
		Name:     filepath.Base(filepath.FromSlash(handler.Path)),
		Path:     handler.Path,
		Field:    "present",
		Line:     0,
		LineSHA:  shortDocumentHash(handler.Path + ":handler"),
	}}
	findings = append(findings, scanHookRiskText("handler", filepath.Base(filepath.FromSlash(handler.Path)), handler.Path, "body", readHookRiskBody(root, handler.Path))...)
	sortHookRiskFindings(findings)
	return findings
}

func hookSpecMetadataRiskFinding(severity, code, category string, spec hookSpecCard, field string) HookRiskFinding {
	return HookRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "hook-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func scanHookRiskText(kind, name, path, field, body string) []HookRiskFinding {
	var findings []HookRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range hookTextRiskRules {
			if !hookRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, HookRiskFinding{
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
	sortHookRiskFindings(findings)
	return findings
}

func hookRiskRuleMatches(lowerLine string, rule hookRiskRule) bool {
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

func readHookRiskBody(root, relPath string) string {
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

func writeHookRiskFindings(b *strings.Builder, findings []HookRiskFinding) {
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

func hookRiskSurfaceCount(findings []HookRiskFinding) int {
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

func hookRiskCodes(findings []HookRiskFinding) []string {
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

func hookRiskLineHashes(findings []HookRiskFinding) []string {
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

func hookRiskMaxSeverity(findings []HookRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if hookRiskSeverityRank(finding.Severity) > hookRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func hookRiskSeverityRank(severity string) int {
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

func sortHookRiskFindings(findings []HookRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if hookRiskSeverityRank(findings[i].Severity) != hookRiskSeverityRank(findings[j].Severity) {
			return hookRiskSeverityRank(findings[i].Severity) > hookRiskSeverityRank(findings[j].Severity)
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
