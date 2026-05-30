package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type NodeRiskFinding struct {
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

type NodeRiskReport struct {
	Status                            string
	VerificationScope                 string
	NodePolicyPresent                 bool
	NodePolicyLoadedForModel          bool
	NodeSpecs                         int
	ScannedNodeSpecs                  int
	NodeRoles                         int
	NodeCapabilitiesDeclared          int
	NodeSpecsRequiringApproval        int
	NodeSpecsEphemeralJobs            int
	CurrentIssueNodeRequest           bool
	SurfacesWithRiskFindings          int
	Findings                          []NodeRiskFinding
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	ActiveNodeRuntime                 string
	NodeInventorySource               string
	GatewayWebSocketRequired          bool
	HeadlessNodeHostSupported         bool
	NodePairingSupported              bool
	NodeRPCSupported                  bool
	NodeCommandInvocationSupported    bool
	RemoteNodeExecSupported           bool
	BrowserProxySupported             bool
	MediaDeviceCapabilitiesSupported  bool
	LongRunningNodeServiceSupported   bool
	RepositoryMutationAllowed         bool
	RawNodeBodiesIncluded             bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	CredentialValuesIncluded          bool
	LLME2ERequiredAfterNodeRiskChange bool
}

type nodeRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var nodeTextRiskRules = []nodeRiskRule{
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
		Code:     "credential_material_in_node",
		Category: "credential-handling",
		Any: []string{
			"openclaw_gateway_token=",
			"openclaw_gateway_password=",
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
		Code:     "gateway_websocket_node_host",
		Category: "runtime-extension",
		Any: []string{
			"openclaw node run",
			"openclaw node install",
			"gateway_websocket_required: true",
			"headless_node_host_supported: true",
			"ws://",
			"wss://",
		},
		IgnoreAny: []string{
			"does not open websocket",
			"doesn't open websocket",
			"do not open websocket",
			"without opening websocket",
			"gateway_websocket_required: false",
			"headless_node_host_supported: false",
		},
	},
	{
		Severity: "high",
		Code:     "remote_node_exec_enabled",
		Category: "host-execution",
		Any: []string{
			"system.run",
			"system.which",
			"node.invoke",
			"host=node",
			"remote_node_exec_supported: true",
			"node_command_invocation_supported: true",
			"node_rpc_supported: true",
		},
		IgnoreAny: []string{
			"remote_node_exec_supported: false",
			"node_command_invocation_supported: false",
			"node_rpc_supported: false",
		},
	},
	{
		Severity: "high",
		Code:     "node_pairing_autoapprove",
		Category: "device-pairing",
		Any: []string{
			"autoapprovecidrs",
			"auto approve node",
			"auto-approve node",
			"devices approve",
			"node_pairing_supported: true",
			"operator.admin",
		},
		IgnoreAny: []string{
			"node_pairing_supported: false",
			"does not pair devices",
			"do not pair devices",
			"without pairing devices",
		},
	},
	{
		Severity: "warning",
		Code:     "browser_proxy_enabled",
		Category: "browser-surface",
		Any: []string{
			"browserproxy.enabled: true",
			"browser_proxy_supported: true",
			"browser proxy",
			"browser.enabled",
		},
		IgnoreAny: []string{
			"browser_proxy_supported: false",
		},
	},
	{
		Severity: "warning",
		Code:     "media_device_capability",
		Category: "device-capability",
		Any: []string{
			"camera.",
			"camera capture",
			"screen.",
			"screen capture",
			"location.",
			"device.",
			"notifications.",
			"sms.",
			"media_device_capabilities_supported: true",
		},
		IgnoreAny: []string{
			"media_device_capabilities_supported: false",
		},
	},
	{
		Severity: "warning",
		Code:     "external_worker_lane",
		Category: "runtime-extension",
		Any: []string{
			"hermes -p",
			"kanban worker",
			"external cli worker",
			"spawn_fn",
			"docker run",
			"ssh ",
			"systemd",
			"launchd",
		},
	},
	{
		Severity: "high",
		Code:     "raw_node_payload_logged",
		Category: "body-leakage",
		Any: []string{
			"cat .gitclaw/nodes",
			"cat .gitclaw/nodes.md",
			"echo \"$node",
			"echo \"$issue_body",
			"echo \"${{ github.event.issue.body",
			"printf \"%s\" \"$node",
			"printf '%s' \"$node",
			"printenv",
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
		Code:     "unbounded_node_loop",
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

func renderNodeRiskReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectNodeSurface(cfg.Workdir)
	report := BuildNodeRiskReport(cfg, includeIssue)
	var b strings.Builder
	b.WriteString("## GitClaw Node Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeNodeRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans node policy and ephemeral GitHub Actions node specs for prompt-boundary, credential, host-execution, gateway WebSocket, remote node execution, device pairing, browser proxy, media-device, external worker lane, raw payload logging, repository mutation, and unbounded-loop risks. It reports metadata, paths, risk codes, severities, and hashes only; node bodies, issue bodies, comments, channel payloads, worker outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Node Policy Risk Card\n")
	writeNodePolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Node Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`node-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeNodeSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Current Node Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-node-request` current_issue_node_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-node-request` scope=`local-cli` current_issue_node_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeNodeRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildNodeRiskReport(cfg Config, includeIssue bool) NodeRiskReport {
	surface := inspectNodeSurface(cfg.Workdir)
	report := NodeRiskReport{
		Status:                            "ok",
		VerificationScope:                 "github_actions_node_metadata",
		NodePolicyPresent:                 surface.Policy.Present,
		NodePolicyLoadedForModel:          nodePolicyLoadedForModel(surface),
		NodeSpecs:                         len(surface.Specs),
		NodeRoles:                         nodeRoleCount(surface.Specs),
		NodeCapabilitiesDeclared:          nodeCapabilityCount(surface.Specs),
		NodeSpecsRequiringApproval:        nodeSpecsRequiringApproval(surface.Specs),
		NodeSpecsEphemeralJobs:            nodeSpecsEphemeralJobs(surface.Specs),
		CurrentIssueNodeRequest:           includeIssue,
		ActiveNodeRuntime:                 "github-actions-ephemeral-job",
		NodeInventorySource:               "git-reviewed-metadata",
		GatewayWebSocketRequired:          false,
		HeadlessNodeHostSupported:         false,
		NodePairingSupported:              false,
		NodeRPCSupported:                  false,
		NodeCommandInvocationSupported:    false,
		RemoteNodeExecSupported:           false,
		BrowserProxySupported:             false,
		MediaDeviceCapabilitiesSupported:  false,
		LongRunningNodeServiceSupported:   false,
		RepositoryMutationAllowed:         false,
		RawNodeBodiesIncluded:             false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		CredentialValuesIncluded:          false,
		LLME2ERequiredAfterNodeRiskChange: true,
	}
	report.Findings = append(report.Findings, scanNodePolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedNodeSpecs++
		report.Findings = append(report.Findings, scanNodeSpecRiskFindings(cfg.Workdir, spec)...)
	}
	sortNodeRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = nodeRiskSurfaceCount(report.Findings)
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

func writeNodeRiskSummary(b *strings.Builder, report NodeRiskReport) {
	fmt.Fprintf(b, "- node_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- node_policy_present: `%t`\n", report.NodePolicyPresent)
	fmt.Fprintf(b, "- node_policy_loaded_for_model: `%t`\n", report.NodePolicyLoadedForModel)
	fmt.Fprintf(b, "- node_specs: `%d`\n", report.NodeSpecs)
	fmt.Fprintf(b, "- scanned_node_specs: `%d`\n", report.ScannedNodeSpecs)
	fmt.Fprintf(b, "- node_roles: `%d`\n", report.NodeRoles)
	fmt.Fprintf(b, "- node_capabilities_declared: `%d`\n", report.NodeCapabilitiesDeclared)
	fmt.Fprintf(b, "- node_specs_requiring_approval: `%d`\n", report.NodeSpecsRequiringApproval)
	fmt.Fprintf(b, "- node_specs_ephemeral_jobs: `%d`\n", report.NodeSpecsEphemeralJobs)
	fmt.Fprintf(b, "- current_issue_node_request: `%t`\n", report.CurrentIssueNodeRequest)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- node_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- active_node_runtime: `%s`\n", report.ActiveNodeRuntime)
	fmt.Fprintf(b, "- node_inventory_source: `%s`\n", report.NodeInventorySource)
	fmt.Fprintf(b, "- gateway_websocket_required: `%t`\n", report.GatewayWebSocketRequired)
	fmt.Fprintf(b, "- headless_node_host_supported: `%t`\n", report.HeadlessNodeHostSupported)
	fmt.Fprintf(b, "- node_pairing_supported: `%t`\n", report.NodePairingSupported)
	fmt.Fprintf(b, "- node_rpc_supported: `%t`\n", report.NodeRPCSupported)
	fmt.Fprintf(b, "- node_command_invocation_supported: `%t`\n", report.NodeCommandInvocationSupported)
	fmt.Fprintf(b, "- remote_node_exec_supported: `%t`\n", report.RemoteNodeExecSupported)
	fmt.Fprintf(b, "- browser_proxy_supported: `%t`\n", report.BrowserProxySupported)
	fmt.Fprintf(b, "- media_device_capabilities_supported: `%t`\n", report.MediaDeviceCapabilitiesSupported)
	fmt.Fprintf(b, "- long_running_node_service_supported: `%t`\n", report.LongRunningNodeServiceSupported)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_node_bodies_included: `%t`\n", report.RawNodeBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_node_risk_change: `%t`\n", report.LLME2ERequiredAfterNodeRiskChange)
}

func writeNodePolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanNodePolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`node-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			nodeRiskMaxSeverity(findings),
			inlineListOrNone(nodeRiskCodes(findings)),
			inlineListOrNone(nodeRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`node-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		nodePolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		nodeRiskMaxSeverity(findings),
		inlineListOrNone(nodeRiskCodes(findings)),
		inlineListOrNone(nodeRiskLineHashes(findings)),
	)
}

func writeNodeSpecRiskCard(b *strings.Builder, root string, spec nodeSpecCard) {
	findings := scanNodeSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`node-spec` name=`%s` path=`%s` frontmatter=`%t` role=`%s` runtime=`%s` mode=`%s` capabilities=`%d` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Role),
		inlineCode(spec.Runtime),
		inlineCode(spec.Mode),
		len(spec.Capabilities),
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		nodeRiskMaxSeverity(findings),
		inlineListOrNone(nodeRiskCodes(findings)),
		inlineListOrNone(nodeRiskLineHashes(findings)),
	)
}

func scanNodePolicyRiskFindings(root string, policy configSurfaceFile) []NodeRiskFinding {
	var findings []NodeRiskFinding
	if !policy.Present {
		findings = append(findings, NodeRiskFinding{
			Severity: "info",
			Code:     "node_policy_not_configured",
			Category: "policy",
			Kind:     "node-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !nodePolicyPathInContext() {
		findings = append(findings, NodeRiskFinding{
			Severity: "high",
			Code:     "node_policy_not_loaded",
			Category: "context",
			Kind:     "node-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanNodeRiskText("node-policy", "policy", policy.Path, "body", readNodeRiskBody(root, policy.Path))...)
	sortNodeRiskFindings(findings)
	return findings
}

func scanNodeSpecRiskFindings(root string, spec nodeSpecCard) []NodeRiskFinding {
	var findings []NodeRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, nodeSpecMetadataRiskFinding("warning", "node_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if strings.TrimSpace(spec.Role) == "" {
		findings = append(findings, nodeSpecMetadataRiskFinding("warning", "node_role_missing", "metadata", spec, "role"))
	}
	if !strings.EqualFold(spec.Runtime, "github-actions") {
		findings = append(findings, nodeSpecMetadataRiskFinding("warning", "node_runtime_not_github_actions", "runtime-boundary", spec, "runtime"))
	}
	if !strings.EqualFold(spec.Mode, "ephemeral-job") {
		findings = append(findings, nodeSpecMetadataRiskFinding("warning", "node_mode_not_ephemeral_job", "runtime-boundary", spec, "mode"))
	}
	if len(spec.Capabilities) == 0 {
		findings = append(findings, nodeSpecMetadataRiskFinding("warning", "node_capabilities_missing", "metadata", spec, "capabilities"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, nodeSpecMetadataRiskFinding("warning", "node_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanNodeRiskText("node-spec", spec.Name, spec.Path, "body", readNodeRiskBody(root, spec.Path))...)
	sortNodeRiskFindings(findings)
	return findings
}

func nodeSpecMetadataRiskFinding(severity, code, category string, spec nodeSpecCard, field string) NodeRiskFinding {
	return NodeRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "node-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func scanNodeRiskText(kind, name, path, field, body string) []NodeRiskFinding {
	var findings []NodeRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range nodeTextRiskRules {
			if !nodeRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, NodeRiskFinding{
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
	sortNodeRiskFindings(findings)
	return findings
}

func nodeRiskRuleMatches(lowerLine string, rule nodeRiskRule) bool {
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

func readNodeRiskBody(root, relPath string) string {
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

func writeNodeRiskFindings(b *strings.Builder, findings []NodeRiskFinding) {
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

func nodeRiskSurfaceCount(findings []NodeRiskFinding) int {
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

func nodeRiskCodes(findings []NodeRiskFinding) []string {
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

func nodeRiskLineHashes(findings []NodeRiskFinding) []string {
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

func nodeRiskMaxSeverity(findings []NodeRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if nodeRiskSeverityRank(finding.Severity) > nodeRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func nodeRiskSeverityRank(severity string) int {
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

func sortNodeRiskFindings(findings []NodeRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := nodeRiskSeverityRank(a.Severity), nodeRiskSeverityRank(b.Severity); rankA != rankB {
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
