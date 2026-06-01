package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ChannelRiskFinding struct {
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

type ChannelRiskReport struct {
	Status                               string
	VerificationScope                    string
	SupportedProviders                   int
	ScannedProviders                     int
	ScannedWorkflows                     int
	PresentWorkflows                     int
	ChannelMessageComments               int
	ScannedChannelMessageComments        int
	SurfacesWithRiskFindings             int
	Findings                             []ChannelRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	WakeStrategy                         string
	StateStorage                         string
	GatewayRuntime                       string
	RawBodiesIncluded                    bool
	RawWorkflowBodiesIncluded            bool
	CredentialValuesIncluded             bool
	LLME2ERequiredAfterChannelRiskChange bool
}

type channelRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var channelRiskRules = []channelRiskRule{
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
		Code:     "credential_material_exposed",
		Category: "credential-handling",
		Any: []string{
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
		Code:     "channel_body_execution",
		Category: "host-execution",
		Any: []string{
			"eval \"$gitclaw_channel_body",
			"eval \"${{ inputs.body",
			"bash -c \"$gitclaw_channel_body",
			"bash -c \"${{ inputs.body",
			"sh -c \"$gitclaw_channel_body",
			"sh -c \"${{ inputs.body",
			"python -c \"$gitclaw_channel_body",
			"python -c \"${{ inputs.body",
		},
	},
	{
		Severity: "high",
		Code:     "raw_channel_body_logged",
		Category: "body-leakage",
		Any: []string{
			"echo \"$gitclaw_channel_body",
			"echo \"${{ inputs.body",
			"printf \"%s\" \"$gitclaw_channel_body",
			"printf '%s' \"$gitclaw_channel_body",
			"cat \"$gitclaw_channel_body",
		},
	},
	{
		Severity: "high",
		Code:     "credential_value_logged",
		Category: "credential-handling",
		Any: []string{
			"echo \"$telegram_bot_token",
			"echo \"$slack_bot_token",
			"echo \"$slack_app_token",
			"echo \"${{ secrets.",
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
		Code:     "unbounded_gateway_loop",
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

func renderChannelRiskReport(ev Event, cfg Config, comments []Comment, includeIssue bool) string {
	report := BuildChannelRiskReport(cfg, comments)
	surface := inspectChannelSurface(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Channel Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeChannelRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans the GitHub-native Slack/Telegram channel bridge for workflow-dispatch control risks and prompt-visible mirrored channel-message risk. It reports provider names, workflow metadata, comment IDs, counts, risk codes, severities, and hashes only; channel message bodies, issue bodies, workflow bodies, prompts, provider credentials, and secret values are not included.\n\n")

	b.WriteString("### Provider Risk Cards\n")
	for _, provider := range channelProviderCatalog {
		fmt.Fprintf(
			&b,
			"- kind=`provider` name=`%s` ingress=`%s` gateway=`%s` offset_key=`%s` thread_key=`%s` message_key=`%s` required_secret_names=`%s` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n",
			provider.Name,
			provider.IngressStrategy,
			provider.GatewayStrategy,
			provider.OffsetKey,
			provider.ThreadKey,
			provider.MessageKey,
			inlineList(provider.RequiredSecrets),
		)
	}

	b.WriteString("\n### Workflow Risk Cards\n")
	writeChannelWorkflowRiskCard(&b, "ingest", surface.IngestWorkflow, 5, true, true)
	writeChannelWorkflowRiskCard(&b, "state", surface.StateWorkflow, 4, false, true)
	writeChannelWorkflowRiskCard(&b, "gateway", surface.GatewayWorkflow, 6, true, true)
	writeChannelWorkflowRiskCard(&b, "delivery", surface.DeliveryWorkflow, 6, false, true)
	writeChannelWorkflowRiskCard(&b, "outbox", surface.OutboxWorkflow, 5, false, false)

	b.WriteString("\n### Channel Message Risk Cards\n")
	wroteMessage := false
	for _, comment := range comments {
		if !HasChannelMessageMarker(comment.Body) {
			continue
		}
		wroteMessage = true
		writeChannelMessageRiskCard(&b, comment)
	}
	if !wroteMessage {
		b.WriteString("- kind=`channel-message` none\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeChannelRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildChannelRiskReport(cfg Config, comments []Comment) ChannelRiskReport {
	surface := inspectChannelSurface(cfg.Workdir)
	report := ChannelRiskReport{
		Status:                               "ok",
		VerificationScope:                    "workflow_dispatch_channel_bridge",
		SupportedProviders:                   len(channelProviderCatalog),
		ScannedProviders:                     len(channelProviderCatalog),
		ScannedWorkflows:                     5,
		ChannelMessageComments:               countChannelMessages(comments),
		WakeStrategy:                         "workflow_dispatch",
		StateStorage:                         "gitclaw:channel-state issue",
		GatewayRuntime:                       "GitHub Actions workflow_dispatch",
		RawBodiesIncluded:                    false,
		RawWorkflowBodiesIncluded:            false,
		CredentialValuesIncluded:             false,
		LLME2ERequiredAfterChannelRiskChange: true,
	}
	workflows := []struct {
		name                 string
		workflow             channelWorkflow
		requiredInputs       int
		requiresActionsWrite bool
		requiresIssuesWrite  bool
	}{
		{name: "ingest", workflow: surface.IngestWorkflow, requiredInputs: 5, requiresActionsWrite: true, requiresIssuesWrite: true},
		{name: "state", workflow: surface.StateWorkflow, requiredInputs: 4, requiresActionsWrite: false, requiresIssuesWrite: true},
		{name: "gateway", workflow: surface.GatewayWorkflow, requiredInputs: 6, requiresActionsWrite: true, requiresIssuesWrite: true},
		{name: "delivery", workflow: surface.DeliveryWorkflow, requiredInputs: 6, requiresActionsWrite: false, requiresIssuesWrite: true},
		{name: "outbox", workflow: surface.OutboxWorkflow, requiredInputs: 5, requiresActionsWrite: false, requiresIssuesWrite: false},
	}
	for _, item := range workflows {
		if item.workflow.Present {
			report.PresentWorkflows++
		}
		report.Findings = append(report.Findings, scanChannelWorkflowRiskFindings(item.name, item.workflow, item.requiredInputs, item.requiresActionsWrite, item.requiresIssuesWrite)...)
	}
	for _, comment := range comments {
		if !HasChannelMessageMarker(comment.Body) {
			continue
		}
		report.ScannedChannelMessageComments++
		report.Findings = append(report.Findings, scanChannelMessageRiskFindings(comment)...)
	}
	sortChannelRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = channelRiskSurfaceCount(report.Findings)
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

func writeChannelRiskSummary(b *strings.Builder, report ChannelRiskReport) {
	fmt.Fprintf(b, "- channel_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- supported_providers: `%d`\n", report.SupportedProviders)
	fmt.Fprintf(b, "- scanned_providers: `%d`\n", report.ScannedProviders)
	fmt.Fprintf(b, "- scanned_workflows: `%d`\n", report.ScannedWorkflows)
	fmt.Fprintf(b, "- present_workflows: `%d`\n", report.PresentWorkflows)
	fmt.Fprintf(b, "- channel_message_comments: `%d`\n", report.ChannelMessageComments)
	fmt.Fprintf(b, "- channel_message_comments_scanned: `%d`\n", report.ScannedChannelMessageComments)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- channel_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- wake_strategy: `%s`\n", report.WakeStrategy)
	fmt.Fprintf(b, "- state_storage: `%s`\n", report.StateStorage)
	fmt.Fprintf(b, "- gateway_runtime: `%s`\n", report.GatewayRuntime)
	fmt.Fprintf(b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_channel_risk_change: `%t`\n", report.LLME2ERequiredAfterChannelRiskChange)
}

func writeChannelWorkflowRiskCard(b *strings.Builder, name string, workflow channelWorkflow, requiredInputs int, requiresActionsWrite, requiresIssuesWrite bool) {
	findings := scanChannelWorkflowRiskFindings(name, workflow, requiredInputs, requiresActionsWrite, requiresIssuesWrite)
	if !workflow.Present {
		fmt.Fprintf(
			b,
			"- kind=`workflow` name=`%s` path=`%s` present=`false` required_inputs=`%d` requires_actions_write=`%t` requires_issues_write=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			name,
			workflow.Path,
			requiredInputs,
			requiresActionsWrite,
			requiresIssuesWrite,
			len(findings),
			channelRiskMaxSeverity(findings),
			inlineListOrNone(channelRiskCodes(findings)),
			inlineListOrNone(channelRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`workflow` name=`%s` path=`%s` present=`true` workflow_dispatch=`%t` actions_write=`%t` issues_write=`%t` inputs=`%d` required_inputs=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		name,
		workflow.Path,
		workflow.WorkflowDispatch,
		workflow.ActionsWrite,
		workflow.IssuesWrite,
		workflow.Inputs,
		requiredInputs,
		workflow.SHA,
		len(findings),
		channelRiskMaxSeverity(findings),
		inlineListOrNone(channelRiskCodes(findings)),
		inlineListOrNone(channelRiskLineHashes(findings)),
	)
}

func writeChannelMessageRiskCard(b *strings.Builder, comment Comment) {
	channel, messageID := channelMessageMarkerFields(comment.Body)
	if channel == "" {
		channel = "none"
	}
	body := StripChannelMessageMarker(comment.Body)
	findings := scanChannelMessageRiskFindings(comment)
	fmt.Fprintf(
		b,
		"- kind=`channel-message` comment_id=`%d` channel=`%s` message_sha256_12=`%s` body_sha256_12=`%s` body_lines=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		comment.ID,
		channel,
		shortDocumentHash(messageID),
		shortDocumentHash(body),
		lineCount(body),
		len(findings),
		channelRiskMaxSeverity(findings),
		inlineListOrNone(channelRiskCodes(findings)),
		inlineListOrNone(channelRiskLineHashes(findings)),
	)
}

func scanChannelWorkflowRiskFindings(name string, workflow channelWorkflow, requiredInputs int, requiresActionsWrite, requiresIssuesWrite bool) []ChannelRiskFinding {
	var findings []ChannelRiskFinding
	if !workflow.Present {
		findings = append(findings, ChannelRiskFinding{
			Severity: "high",
			Code:     "channel_workflow_missing",
			Category: "workflow-dispatch",
			Kind:     "workflow",
			Name:     name,
			Path:     workflow.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(workflow.Path),
		})
		sortChannelRiskFindings(findings)
		return findings
	}
	if !workflow.WorkflowDispatch {
		findings = append(findings, channelWorkflowMetadataFinding("high", "workflow_dispatch_missing", "workflow-dispatch", name, workflow, "workflow_dispatch"))
	}
	if requiresActionsWrite && !workflow.ActionsWrite {
		findings = append(findings, channelWorkflowMetadataFinding("high", "actions_write_permission_missing", "workflow-permission", name, workflow, "actions"))
	}
	if requiresIssuesWrite && !workflow.IssuesWrite {
		findings = append(findings, channelWorkflowMetadataFinding("high", "issues_write_permission_missing", "workflow-permission", name, workflow, "issues"))
	}
	if workflow.Inputs < requiredInputs {
		findings = append(findings, channelWorkflowMetadataFinding("high", "required_workflow_inputs_missing", "workflow-inputs", name, workflow, "inputs"))
	}
	findings = append(findings, scanChannelRiskText("workflow", name, workflow.Path, "body", workflow.Body)...)
	sortChannelRiskFindings(findings)
	return findings
}

func channelWorkflowMetadataFinding(severity, code, category, name string, workflow channelWorkflow, field string) ChannelRiskFinding {
	return ChannelRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "workflow",
		Name:     name,
		Path:     workflow.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(workflow.Path + ":" + field),
	}
}

func scanChannelMessageRiskFindings(comment Comment) []ChannelRiskFinding {
	channel, _ := channelMessageMarkerFields(comment.Body)
	if channel == "" {
		channel = "none"
	}
	body := StripChannelMessageMarker(comment.Body)
	findings := scanChannelRiskText("channel-message", channel, fmt.Sprintf("comment:%d", comment.ID), "body", body)
	sortChannelRiskFindings(findings)
	return findings
}

func scanChannelRiskText(kind, name, path, field, body string) []ChannelRiskFinding {
	var findings []ChannelRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range channelRiskRules {
			if !channelRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, ChannelRiskFinding{
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
	sortChannelRiskFindings(findings)
	return findings
}

func channelRiskRuleMatches(lowerLine string, rule channelRiskRule) bool {
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

func channelMessageMarkerFields(body string) (string, string) {
	match := channelMessageMarkerPattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return "", ""
	}
	return markerAttribute(match[1], "channel"), markerAttribute(match[1], "message_id")
}

func writeChannelRiskFindings(b *strings.Builder, findings []ChannelRiskFinding) {
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

func channelRiskSurfaceCount(findings []ChannelRiskFinding) int {
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

func channelRiskCodes(findings []ChannelRiskFinding) []string {
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

func channelRiskLineHashes(findings []ChannelRiskFinding) []string {
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

func channelRiskMaxSeverity(findings []ChannelRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if channelRiskSeverityRank(finding.Severity) > channelRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func channelRiskSeverityRank(severity string) int {
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

func sortChannelRiskFindings(findings []ChannelRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if channelRiskSeverityRank(findings[i].Severity) != channelRiskSeverityRank(findings[j].Severity) {
			return channelRiskSeverityRank(findings[i].Severity) > channelRiskSeverityRank(findings[j].Severity)
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
