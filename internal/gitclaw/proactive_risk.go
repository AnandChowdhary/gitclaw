package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProactiveRiskFinding struct {
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

type ProactiveRiskReport struct {
	Status                                 string
	VerificationScope                      string
	PromptFiles                            int
	ScannedPromptFiles                     int
	WorkflowFiles                          int
	ScannedWorkflows                       int
	PresentWorkflows                       int
	PromptSkillHints                       int
	SurfacesWithRiskFindings               int
	Findings                               []ProactiveRiskFinding
	HighRiskFindings                       int
	WarningRiskFindings                    int
	InfoRiskFindings                       int
	WakeStrategy                           string
	SchedulerRuntime                       string
	StateStorage                           string
	RawPromptBodiesIncluded                bool
	RawWorkflowBodiesIncluded              bool
	CredentialValuesIncluded               bool
	LLME2ERequiredAfterProactiveRiskChange bool
}

type proactiveRiskWorkflow struct {
	Workflow proactiveWorkflow
	Body     string
}

type proactiveRiskPrompt struct {
	Prompt proactivePrompt
	Body   string
}

type proactiveRiskRule struct {
	Severity string
	Code     string
	Category string
	Any      []string
	All      []string
}

var proactivePromptRiskRules = []proactiveRiskRule{
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
		Code:     "credential_material_in_prompt",
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
		Code:     "persistent_state_backdoor",
		Category: "persistence",
		Any: []string{
			"silently persist",
			"persist without review",
			"write to memory without review",
			"edit soul without review",
			"install backdoor",
			"command-and-control",
			"c2 implant",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_schedule_instruction",
		Category: "runtime-amplification",
		Any: []string{
			"retry forever",
			"loop forever",
			"while true",
			"every minute",
			"every 1 minute",
			"every 2 minutes",
			"sleep infinity",
			"never stop",
			"continue indefinitely",
		},
	},
	{
		Severity: "warning",
		Code:     "unreviewed_write_instruction",
		Category: "write-authority",
		Any: []string{
			"commit directly",
			"push directly",
			"merge without review",
			"write files without review",
			"modify files without review",
			"create branches without review",
			"open pull requests without review",
		},
	},
}

var proactiveWorkflowRiskRules = []proactiveRiskRule{
	{
		Severity: "high",
		Code:     "prompt_body_execution",
		Category: "host-execution",
		Any: []string{
			"eval \"$gitclaw_proactive_prompt",
			"eval \"${{ github.event.inputs.prompt",
			"bash -c \"$gitclaw_proactive_prompt",
			"bash -c \"${{ github.event.inputs.prompt",
			"sh -c \"$gitclaw_proactive_prompt",
			"sh -c \"${{ github.event.inputs.prompt",
			"python -c \"$gitclaw_proactive_prompt",
			"python -c \"${{ github.event.inputs.prompt",
		},
	},
	{
		Severity: "high",
		Code:     "raw_prompt_logged",
		Category: "body-leakage",
		Any: []string{
			"echo \"$gitclaw_proactive_prompt",
			"echo \"${{ github.event.inputs.prompt",
			"printf \"%s\" \"$gitclaw_proactive_prompt",
			"printf '%s' \"$gitclaw_proactive_prompt",
		},
	},
	{
		Severity: "high",
		Code:     "credential_value_logged",
		Category: "credential-handling",
		Any: []string{
			"echo \"$github_token",
			"echo \"$gh_token",
			"echo \"${{ secrets.",
			"printenv",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_workflow_loop",
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

func renderProactiveRiskReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildProactiveRiskReport(cfg)
	surface := inspectProactiveSurface(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Proactive Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeProactiveRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans scheduled proactive prompts and workflow-dispatch wakeups for body leakage, host execution, credential exposure, unbounded scheduling, and write-authority risks. It reports paths, metadata, risk codes, severities, and hashes only; proactive prompt bodies, workflow bodies, issue bodies, comments, credentials, and secret values are not included.\n\n")

	b.WriteString("### Workflow Risk Cards\n")
	writeProactiveWorkflowRiskCard(&b, loadProactiveRiskWorkflow(cfg.Workdir, proactiveWorkflowPath))

	b.WriteString("\n### Prompt Risk Cards\n")
	if len(surface.Prompts) == 0 {
		b.WriteString("- kind=`prompt` none\n")
	} else {
		for _, prompt := range surface.Prompts {
			writeProactivePromptRiskCard(&b, loadProactiveRiskPrompt(cfg.Workdir, prompt))
		}
	}

	b.WriteString("\n### Risk Findings\n")
	writeProactiveRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildProactiveRiskReport(cfg Config) ProactiveRiskReport {
	surface := inspectProactiveSurface(cfg.Workdir)
	report := ProactiveRiskReport{
		Status:                                 "ok",
		VerificationScope:                      "scheduled_issue_workflow",
		PromptFiles:                            len(surface.Prompts),
		WorkflowFiles:                          1,
		ScannedWorkflows:                       1,
		PromptSkillHints:                       proactivePromptSkillHintCount(surface.Prompts),
		WakeStrategy:                           "workflow_dispatch",
		SchedulerRuntime:                       "GitHub Actions schedule",
		StateStorage:                           "gitclaw:proactive-run issues",
		RawPromptBodiesIncluded:                false,
		RawWorkflowBodiesIncluded:              false,
		CredentialValuesIncluded:               false,
		LLME2ERequiredAfterProactiveRiskChange: true,
	}
	workflow := loadProactiveRiskWorkflow(cfg.Workdir, proactiveWorkflowPath)
	if workflow.Workflow.Present {
		report.PresentWorkflows++
	}
	report.Findings = append(report.Findings, scanProactiveWorkflowRiskFindings(workflow)...)
	for _, prompt := range surface.Prompts {
		report.ScannedPromptFiles++
		report.Findings = append(report.Findings, scanProactivePromptRiskFindings(loadProactiveRiskPrompt(cfg.Workdir, prompt))...)
	}
	sortProactiveRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = proactiveRiskSurfaceCount(report.Findings)
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

func writeProactiveRiskSummary(b *strings.Builder, report ProactiveRiskReport) {
	fmt.Fprintf(b, "- proactive_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- prompt_files: `%d`\n", report.PromptFiles)
	fmt.Fprintf(b, "- scanned_prompt_files: `%d`\n", report.ScannedPromptFiles)
	fmt.Fprintf(b, "- workflow_files: `%d`\n", report.WorkflowFiles)
	fmt.Fprintf(b, "- scanned_workflows: `%d`\n", report.ScannedWorkflows)
	fmt.Fprintf(b, "- present_workflows: `%d`\n", report.PresentWorkflows)
	fmt.Fprintf(b, "- prompt_skill_hints: `%d`\n", report.PromptSkillHints)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- proactive_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- wake_strategy: `%s`\n", report.WakeStrategy)
	fmt.Fprintf(b, "- scheduler_runtime: `%s`\n", report.SchedulerRuntime)
	fmt.Fprintf(b, "- state_storage: `%s`\n", report.StateStorage)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_workflow_bodies_included: `%t`\n", report.RawWorkflowBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_proactive_risk_change: `%t`\n", report.LLME2ERequiredAfterProactiveRiskChange)
}

func writeProactiveWorkflowRiskCard(b *strings.Builder, workflow proactiveRiskWorkflow) {
	findings := scanProactiveWorkflowRiskFindings(workflow)
	if !workflow.Workflow.Present {
		fmt.Fprintf(
			b,
			"- kind=`workflow` name=`generic` path=`%s` present=`false` workflow_dispatch=`false` schedule=`false` actions_write=`false` issues_write=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			workflow.Workflow.Path,
			len(findings),
			proactiveRiskMaxSeverity(findings),
			inlineListOrNone(proactiveRiskCodes(findings)),
			inlineListOrNone(proactiveRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`workflow` name=`generic` path=`%s` present=`true` workflow_dispatch=`%t` schedule=`%t` actions_write=`%t` issues_write=`%t` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		workflow.Workflow.Path,
		workflow.Workflow.WorkflowDispatch,
		workflow.Workflow.Schedule,
		proactiveWorkflowHasPermission(workflow.Body, "actions", "write"),
		proactiveWorkflowHasPermission(workflow.Body, "issues", "write"),
		workflow.Workflow.SHA,
		len(findings),
		proactiveRiskMaxSeverity(findings),
		inlineListOrNone(proactiveRiskCodes(findings)),
		inlineListOrNone(proactiveRiskLineHashes(findings)),
	)
}

func writeProactivePromptRiskCard(b *strings.Builder, prompt proactiveRiskPrompt) {
	findings := scanProactivePromptRiskFindings(prompt)
	fmt.Fprintf(
		b,
		"- kind=`prompt` name=`%s` path=`%s` bytes=`%d` lines=`%d` skill_hints=`%s` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		prompt.Prompt.Name,
		prompt.Prompt.Path,
		prompt.Prompt.Bytes,
		prompt.Prompt.Lines,
		inlineListOrNone(prompt.Prompt.SkillHints),
		prompt.Prompt.SHA,
		len(findings),
		proactiveRiskMaxSeverity(findings),
		inlineListOrNone(proactiveRiskCodes(findings)),
		inlineListOrNone(proactiveRiskLineHashes(findings)),
	)
}

func scanProactiveWorkflowRiskFindings(workflow proactiveRiskWorkflow) []ProactiveRiskFinding {
	var findings []ProactiveRiskFinding
	if !workflow.Workflow.Present {
		findings = append(findings, proactiveWorkflowMetadataFinding("high", "proactive_workflow_missing", "workflow-dispatch", workflow.Workflow, "present"))
		sortProactiveRiskFindings(findings)
		return findings
	}
	if !workflow.Workflow.WorkflowDispatch {
		findings = append(findings, proactiveWorkflowMetadataFinding("high", "workflow_dispatch_missing", "workflow-dispatch", workflow.Workflow, "workflow_dispatch"))
	}
	if !workflow.Workflow.Schedule {
		findings = append(findings, proactiveWorkflowMetadataFinding("warning", "schedule_trigger_missing", "scheduler", workflow.Workflow, "schedule"))
	}
	if !proactiveWorkflowHasPermission(workflow.Body, "actions", "write") {
		findings = append(findings, proactiveWorkflowMetadataFinding("high", "actions_write_permission_missing", "workflow-permission", workflow.Workflow, "actions"))
	}
	if !proactiveWorkflowHasPermission(workflow.Body, "issues", "write") {
		findings = append(findings, proactiveWorkflowMetadataFinding("high", "issues_write_permission_missing", "workflow-permission", workflow.Workflow, "issues"))
	}
	findings = append(findings, scanProactiveRiskText("workflow", "generic", workflow.Workflow.Path, "body", workflow.Body, proactiveWorkflowRiskRules)...)
	sortProactiveRiskFindings(findings)
	return findings
}

func scanProactivePromptRiskFindings(prompt proactiveRiskPrompt) []ProactiveRiskFinding {
	findings := scanProactiveRiskText("prompt", prompt.Prompt.Name, prompt.Prompt.Path, "body", prompt.Body, proactivePromptRiskRules)
	sortProactiveRiskFindings(findings)
	return findings
}

func scanProactiveRiskText(kind, name, path, field, body string, rules []proactiveRiskRule) []ProactiveRiskFinding {
	var findings []ProactiveRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range rules {
			if !proactiveRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, ProactiveRiskFinding{
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
	sortProactiveRiskFindings(findings)
	return findings
}

func proactiveRiskRuleMatches(lowerLine string, rule proactiveRiskRule) bool {
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

func proactiveWorkflowMetadataFinding(severity, code, category string, workflow proactiveWorkflow, field string) ProactiveRiskFinding {
	return ProactiveRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "workflow",
		Name:     "generic",
		Path:     workflow.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(workflow.Path + ":" + field),
	}
}

func loadProactiveRiskWorkflow(root, relPath string) proactiveRiskWorkflow {
	workflow := inspectProactiveWorkflow(root, relPath)
	body := readProactiveRiskBody(root, relPath)
	return proactiveRiskWorkflow{Workflow: workflow, Body: body}
}

func loadProactiveRiskPrompt(root string, prompt proactivePrompt) proactiveRiskPrompt {
	return proactiveRiskPrompt{Prompt: prompt, Body: readProactiveRiskBody(root, prompt.Path)}
}

func readProactiveRiskBody(root, relPath string) string {
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

func proactiveWorkflowHasPermission(body, name, value string) bool {
	want := strings.ToLower(name) + ": " + strings.ToLower(value)
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(strings.ToLower(strings.TrimSpace(line)), want) {
			return true
		}
	}
	return false
}

func writeProactiveRiskFindings(b *strings.Builder, findings []ProactiveRiskFinding) {
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

func proactiveRiskSurfaceCount(findings []ProactiveRiskFinding) int {
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

func proactiveRiskCodes(findings []ProactiveRiskFinding) []string {
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

func proactiveRiskLineHashes(findings []ProactiveRiskFinding) []string {
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

func proactiveRiskMaxSeverity(findings []ProactiveRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if proactiveRiskSeverityRank(finding.Severity) > proactiveRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func proactiveRiskSeverityRank(severity string) int {
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

func sortProactiveRiskFindings(findings []ProactiveRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if proactiveRiskSeverityRank(findings[i].Severity) != proactiveRiskSeverityRank(findings[j].Severity) {
			return proactiveRiskSeverityRank(findings[i].Severity) > proactiveRiskSeverityRank(findings[j].Severity)
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
