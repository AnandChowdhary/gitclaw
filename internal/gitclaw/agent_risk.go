package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AgentRiskFinding struct {
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

type AgentRiskReport struct {
	Status                             string
	VerificationScope                  string
	AgentPolicyPresent                 bool
	AgentPolicyLoadedForModel          bool
	AgentSpecs                         int
	ScannedAgentSpecs                  int
	AgentRoles                         int
	AgentToolsRequested                int
	AgentSpecsRequiringApproval        int
	AgentSpecsSingleAssistant          int
	CurrentIssueAgentRequest           bool
	SurfacesWithRiskFindings           int
	Findings                           []AgentRiskFinding
	HighRiskFindings                   int
	WarningRiskFindings                int
	InfoRiskFindings                   int
	ActiveAgentRuntime                 string
	MultiAgentRoutingSupported         bool
	MultiAgentDelegationSupported      bool
	SubagentExecutionSupported         bool
	DelegateTaskSupported              bool
	RemoteAgentProcessAllowed          bool
	AgentToAgentMessagingAllowed       bool
	RepositoryMutationAllowed          bool
	RawAgentBodiesIncluded             bool
	RawIssueBodiesIncluded             bool
	RawCommentBodiesIncluded           bool
	CredentialValuesIncluded           bool
	LLME2ERequiredAfterAgentRiskChange bool
}

type agentRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var agentTextRiskRules = []agentRiskRule{
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
		Code:     "credential_material_in_agent",
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
		Code:     "subagent_delegation_enabled",
		Category: "runtime-extension",
		Any: []string{
			"sessions_spawn",
			"delegate_task(",
			"delegate_task ",
			"spawn subagent",
			"spawn sub-agent",
			"start subagent",
			"start sub-agent",
			"agents.defaults.subagents",
			"multi_agent_delegation_supported: true",
			"subagent_execution_supported: true",
		},
		IgnoreAny: []string{
			"does not spawn subagent",
			"does not spawn sub-agent",
			"doesn't spawn subagent",
			"doesn't spawn sub-agent",
			"do not spawn subagent",
			"do not spawn sub-agent",
			"not spawn subagent",
			"not spawn sub-agent",
			"without spawning subagents",
			"without spawning sub-agents",
			"subagent_execution_supported: false",
			"multi_agent_delegation_supported: false",
		},
	},
	{
		Severity: "high",
		Code:     "external_agent_process",
		Category: "runtime-extension",
		Any: []string{
			"gateway start",
			"gateway install",
			"start gateway",
			"runtime: acp",
			"runtime=\"acp\"",
			"acpx",
			"remote agent process",
			"docker run",
			"ssh ",
		},
	},
	{
		Severity: "high",
		Code:     "raw_agent_payload_logged",
		Category: "body-leakage",
		Any: []string{
			"cat .gitclaw/agents",
			"cat .gitclaw/agents.md",
			"echo \"$agent",
			"echo \"$issue_body",
			"echo \"${{ github.event.issue.body",
			"printf \"%s\" \"$agent",
			"printf '%s' \"$agent",
			"printenv",
		},
	},
	{
		Severity: "warning",
		Code:     "shared_agent_secret_state",
		Category: "isolation-boundary",
		Any: []string{
			"share credentials",
			"share sessions",
			"share memory",
			"same bot token",
			"copy .env",
			"shared session database",
			"shared memory database",
		},
	},
	{
		Severity: "warning",
		Code:     "external_agent_webhook_bridge",
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
		Code:     "unbounded_agent_loop",
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

func renderAgentRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectAgentSurface(cfg.Workdir)
	report := BuildAgentRiskReport(cfg, includeIssue)
	var b strings.Builder
	b.WriteString("## GitClaw Agent Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeAgentRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans agent policy and single-assistant agent specs for prompt-boundary, credential, host-execution, subagent/delegation, external process, shared-state, raw payload logging, webhook, repository mutation, and unbounded-loop risks. It reports metadata, paths, risk codes, severities, and hashes only; agent bodies, issue bodies, comments, channel payloads, worker outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Agent Policy Risk Card\n")
	writeAgentPolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Agent Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`agent-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeAgentSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Current Agent Request Risk Card\n")
	if includeIssue {
		fmt.Fprintf(
			&b,
			"- kind=`current-agent-request` current_issue_agent_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n",
		)
	} else {
		b.WriteString("- kind=`current-agent-request` scope=`local-cli` current_issue_agent_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeAgentRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildAgentRiskReport(cfg Config, includeIssue bool) AgentRiskReport {
	surface := inspectAgentSurface(cfg.Workdir)
	report := AgentRiskReport{
		Status:                             "ok",
		VerificationScope:                  "github_actions_agent_metadata",
		AgentPolicyPresent:                 surface.Policy.Present,
		AgentPolicyLoadedForModel:          agentPolicyLoadedForModel(surface),
		AgentSpecs:                         len(surface.Specs),
		AgentRoles:                         agentRoleCount(surface.Specs),
		AgentToolsRequested:                agentToolCount(surface.Specs),
		AgentSpecsRequiringApproval:        agentSpecsRequiringApproval(surface.Specs),
		AgentSpecsSingleAssistant:          agentSpecsSingleAssistant(surface.Specs),
		CurrentIssueAgentRequest:           includeIssue,
		ActiveAgentRuntime:                 "github-actions",
		MultiAgentRoutingSupported:         false,
		MultiAgentDelegationSupported:      false,
		SubagentExecutionSupported:         false,
		DelegateTaskSupported:              false,
		RemoteAgentProcessAllowed:          false,
		AgentToAgentMessagingAllowed:       false,
		RepositoryMutationAllowed:          false,
		RawAgentBodiesIncluded:             false,
		RawIssueBodiesIncluded:             false,
		RawCommentBodiesIncluded:           false,
		CredentialValuesIncluded:           false,
		LLME2ERequiredAfterAgentRiskChange: true,
	}
	report.Findings = append(report.Findings, scanAgentPolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedAgentSpecs++
		report.Findings = append(report.Findings, scanAgentSpecRiskFindings(cfg.Workdir, spec)...)
	}
	sortAgentRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = agentRiskSurfaceCount(report.Findings)
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

func writeAgentRiskSummary(b *strings.Builder, report AgentRiskReport) {
	fmt.Fprintf(b, "- agent_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- agent_policy_present: `%t`\n", report.AgentPolicyPresent)
	fmt.Fprintf(b, "- agent_policy_loaded_for_model: `%t`\n", report.AgentPolicyLoadedForModel)
	fmt.Fprintf(b, "- agent_specs: `%d`\n", report.AgentSpecs)
	fmt.Fprintf(b, "- scanned_agent_specs: `%d`\n", report.ScannedAgentSpecs)
	fmt.Fprintf(b, "- agent_roles: `%d`\n", report.AgentRoles)
	fmt.Fprintf(b, "- agent_tools_requested: `%d`\n", report.AgentToolsRequested)
	fmt.Fprintf(b, "- agent_specs_requiring_approval: `%d`\n", report.AgentSpecsRequiringApproval)
	fmt.Fprintf(b, "- agent_specs_single_assistant: `%d`\n", report.AgentSpecsSingleAssistant)
	fmt.Fprintf(b, "- current_issue_agent_request: `%t`\n", report.CurrentIssueAgentRequest)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- agent_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- active_agent_runtime: `%s`\n", report.ActiveAgentRuntime)
	fmt.Fprintf(b, "- multi_agent_routing_supported: `%t`\n", report.MultiAgentRoutingSupported)
	fmt.Fprintf(b, "- multi_agent_delegation_supported: `%t`\n", report.MultiAgentDelegationSupported)
	fmt.Fprintf(b, "- subagent_execution_supported: `%t`\n", report.SubagentExecutionSupported)
	fmt.Fprintf(b, "- delegate_task_supported: `%t`\n", report.DelegateTaskSupported)
	fmt.Fprintf(b, "- remote_agent_process_allowed: `%t`\n", report.RemoteAgentProcessAllowed)
	fmt.Fprintf(b, "- agent_to_agent_messaging_allowed: `%t`\n", report.AgentToAgentMessagingAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_agent_bodies_included: `%t`\n", report.RawAgentBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_agent_risk_change: `%t`\n", report.LLME2ERequiredAfterAgentRiskChange)
}

func writeAgentPolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanAgentPolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`agent-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			agentRiskMaxSeverity(findings),
			inlineListOrNone(agentRiskCodes(findings)),
			inlineListOrNone(agentRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`agent-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		agentPolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		agentRiskMaxSeverity(findings),
		inlineListOrNone(agentRiskCodes(findings)),
		inlineListOrNone(agentRiskLineHashes(findings)),
	)
}

func writeAgentSpecRiskCard(b *strings.Builder, root string, spec agentSpecCard) {
	findings := scanAgentSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`agent-spec` name=`%s` path=`%s` frontmatter=`%t` role=`%s` runtime=`%s` mode=`%s` tools=`%d` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Role),
		inlineCode(spec.Runtime),
		inlineCode(spec.Mode),
		len(spec.Tools),
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		agentRiskMaxSeverity(findings),
		inlineListOrNone(agentRiskCodes(findings)),
		inlineListOrNone(agentRiskLineHashes(findings)),
	)
}

func scanAgentPolicyRiskFindings(root string, policy configSurfaceFile) []AgentRiskFinding {
	var findings []AgentRiskFinding
	if !policy.Present {
		findings = append(findings, AgentRiskFinding{
			Severity: "info",
			Code:     "agent_policy_not_configured",
			Category: "policy",
			Kind:     "agent-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !agentPolicyPathInContext() {
		findings = append(findings, AgentRiskFinding{
			Severity: "high",
			Code:     "agent_policy_not_loaded",
			Category: "context",
			Kind:     "agent-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanAgentRiskText("agent-policy", "policy", policy.Path, "body", readAgentRiskBody(root, policy.Path))...)
	sortAgentRiskFindings(findings)
	return findings
}

func scanAgentSpecRiskFindings(root string, spec agentSpecCard) []AgentRiskFinding {
	var findings []AgentRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, agentSpecMetadataRiskFinding("warning", "agent_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if strings.TrimSpace(spec.Role) == "" {
		findings = append(findings, agentSpecMetadataRiskFinding("warning", "agent_role_missing", "metadata", spec, "role"))
	}
	if !strings.EqualFold(spec.Runtime, "github-actions") {
		findings = append(findings, agentSpecMetadataRiskFinding("warning", "agent_runtime_not_github_actions", "runtime-boundary", spec, "runtime"))
	}
	if !strings.EqualFold(spec.Mode, "single-assistant") {
		findings = append(findings, agentSpecMetadataRiskFinding("warning", "agent_mode_not_single_assistant", "runtime-boundary", spec, "mode"))
	}
	if len(spec.Tools) == 0 {
		findings = append(findings, agentSpecMetadataRiskFinding("warning", "agent_tools_missing", "metadata", spec, "tools"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, agentSpecMetadataRiskFinding("warning", "agent_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanAgentRiskText("agent-spec", spec.Name, spec.Path, "body", readAgentRiskBody(root, spec.Path))...)
	sortAgentRiskFindings(findings)
	return findings
}

func agentSpecMetadataRiskFinding(severity, code, category string, spec agentSpecCard, field string) AgentRiskFinding {
	return AgentRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "agent-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func scanAgentRiskText(kind, name, path, field, body string) []AgentRiskFinding {
	var findings []AgentRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range agentTextRiskRules {
			if !agentRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, AgentRiskFinding{
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
	sortAgentRiskFindings(findings)
	return findings
}

func agentRiskRuleMatches(lowerLine string, rule agentRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerLine, ignored) {
			return false
		}
	}
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

func readAgentRiskBody(root, relPath string) string {
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

func writeAgentRiskFindings(b *strings.Builder, findings []AgentRiskFinding) {
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

func agentRiskSurfaceCount(findings []AgentRiskFinding) int {
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

func agentRiskCodes(findings []AgentRiskFinding) []string {
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

func agentRiskLineHashes(findings []AgentRiskFinding) []string {
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

func agentRiskMaxSeverity(findings []AgentRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if agentRiskSeverityRank(finding.Severity) > agentRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func agentRiskSeverityRank(severity string) int {
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

func sortAgentRiskFindings(findings []AgentRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := agentRiskSeverityRank(a.Severity), agentRiskSeverityRank(b.Severity); rankA != rankB {
			return rankA > rankB
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.LineSHA < b.LineSHA
	})
}
