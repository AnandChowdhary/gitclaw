package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TaskRiskFinding struct {
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

type TaskRiskReport struct {
	Status                            string
	VerificationScope                 string
	TaskPolicyPresent                 bool
	TaskPolicyLoadedForModel          bool
	TaskSpecs                         int
	ScannedTaskSpecs                  int
	TaskStatusesDeclared              int
	TaskLabelsDeclared                int
	TaskSpecsRequiringApproval        int
	TaskSpecsIssueNative              int
	CurrentIssueTask                  bool
	CurrentTaskComments               int
	CurrentTaskTranscriptMessages     int
	SurfacesWithRiskFindings          int
	Findings                          []TaskRiskFinding
	HighRiskFindings                  int
	WarningRiskFindings               int
	InfoRiskFindings                  int
	TaskStorageBackend                string
	SQLiteTaskDBRequired              bool
	DetachedWorkerSupported           bool
	KanbanDispatcherSupported         bool
	TaskFlowExecutionSupported        bool
	RepositoryMutationAllowed         bool
	RawTaskBodiesIncluded             bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	CredentialValuesIncluded          bool
	LLME2ERequiredAfterTaskRiskChange bool
}

type taskRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var taskTextRiskRules = []taskRiskRule{
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
		Code:     "credential_material_in_task",
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
		Code:     "detached_worker_spawn",
		Category: "runtime-extension",
		Any: []string{
			"spawn detached worker",
			"start detached worker",
			"spawn subagent",
			"start subagent",
			"delegate_task",
			"start kanban dispatcher",
			"spawn kanban worker",
			"run worker automatically",
		},
		IgnoreAny: []string{
			"does not spawn detached worker",
			"doesn't spawn detached worker",
			"do not spawn detached worker",
			"not spawn detached worker",
			"without spawning detached worker",
			"does not start detached worker",
			"do not start detached worker",
			"without starting detached worker",
		},
	},
	{
		Severity: "high",
		Code:     "raw_task_payload_logged",
		Category: "body-leakage",
		Any: []string{
			"echo \"$gitclaw_task_payload",
			"echo \"$issue_body",
			"echo \"${{ github.event.issue.body",
			"printf \"%s\" \"$gitclaw_task_payload",
			"printf '%s' \"$gitclaw_task_payload",
			"printenv",
		},
	},
	{
		Severity: "warning",
		Code:     "external_task_database",
		Category: "state-storage",
		Any: []string{
			"requires sqlite task db",
			"sqlite_task_db_required: true",
			"sqlite task board required",
			"supabase",
			"postgres task",
			"external task database",
			"linear api",
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
		Code:     "unbounded_task_loop",
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

func renderTaskRiskReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, includeIssue bool) string {
	surface := inspectTaskSurface(cfg.Workdir)
	report := BuildTaskRiskReport(cfg, comments, transcript, includeIssue)
	var b strings.Builder
	b.WriteString("## GitClaw Task Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeTaskRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- current_task_status: `%s`\n", currentIssueTaskStatus(ev, cfg))
		fmt.Fprintf(&b, "- current_task_labels: `%d`\n", len(ev.Issue.Labels))
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans task policy and issue-native task specs for prompt-boundary, credential, host-execution, detached-worker, external state, raw payload logging, webhook, repository mutation, and unbounded-loop risks. It reports metadata, paths, risk codes, severities, and hashes only; task bodies, issue bodies, comments, transcript messages, worker outputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Task Policy Risk Card\n")
	writeTaskPolicyRiskCard(&b, cfg.Workdir, surface.Policy)

	b.WriteString("\n### Task Spec Risk Cards\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- kind=`task-spec` none\n")
	} else {
		for _, spec := range surface.Specs {
			writeTaskSpecRiskCard(&b, cfg.Workdir, spec)
		}
	}

	b.WriteString("\n### Current Task Thread Risk Card\n")
	if includeIssue {
		fmt.Fprintf(
			&b,
			"- kind=`current-task` status=`%s` labels=`%d` comments=`%d` transcript_messages=`%d` issue_body_scanned=`false` comment_bodies_scanned=`false` transcript_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n",
			currentIssueTaskStatus(ev, cfg),
			len(ev.Issue.Labels),
			len(comments),
			len(transcript),
		)
	} else {
		b.WriteString("- kind=`current-task` scope=`local-cli` current_issue_task=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeTaskRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildTaskRiskReport(cfg Config, comments []Comment, transcript []TranscriptMessage, includeIssue bool) TaskRiskReport {
	surface := inspectTaskSurface(cfg.Workdir)
	report := TaskRiskReport{
		Status:                            "ok",
		VerificationScope:                 "github_issue_task_metadata",
		TaskPolicyPresent:                 surface.Policy.Present,
		TaskPolicyLoadedForModel:          taskPolicyLoadedForModel(surface),
		TaskSpecs:                         len(surface.Specs),
		TaskStatusesDeclared:              taskStatusCount(surface.Specs),
		TaskLabelsDeclared:                taskLabelCount(surface.Specs),
		TaskSpecsRequiringApproval:        taskSpecsRequiringApproval(surface.Specs),
		TaskSpecsIssueNative:              taskSpecsIssueNative(surface.Specs),
		CurrentIssueTask:                  includeIssue,
		CurrentTaskComments:               len(comments),
		CurrentTaskTranscriptMessages:     len(transcript),
		TaskStorageBackend:                "github-issues",
		SQLiteTaskDBRequired:              false,
		DetachedWorkerSupported:           false,
		KanbanDispatcherSupported:         false,
		TaskFlowExecutionSupported:        false,
		RepositoryMutationAllowed:         false,
		RawTaskBodiesIncluded:             false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		CredentialValuesIncluded:          false,
		LLME2ERequiredAfterTaskRiskChange: true,
	}
	report.Findings = append(report.Findings, scanTaskPolicyRiskFindings(cfg.Workdir, surface.Policy)...)
	for _, spec := range surface.Specs {
		report.ScannedTaskSpecs++
		report.Findings = append(report.Findings, scanTaskSpecRiskFindings(cfg.Workdir, spec)...)
	}
	sortTaskRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = taskRiskSurfaceCount(report.Findings)
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

func writeTaskRiskSummary(b *strings.Builder, report TaskRiskReport) {
	fmt.Fprintf(b, "- task_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- task_policy_present: `%t`\n", report.TaskPolicyPresent)
	fmt.Fprintf(b, "- task_policy_loaded_for_model: `%t`\n", report.TaskPolicyLoadedForModel)
	fmt.Fprintf(b, "- task_specs: `%d`\n", report.TaskSpecs)
	fmt.Fprintf(b, "- scanned_task_specs: `%d`\n", report.ScannedTaskSpecs)
	fmt.Fprintf(b, "- task_statuses_declared: `%d`\n", report.TaskStatusesDeclared)
	fmt.Fprintf(b, "- task_labels_declared: `%d`\n", report.TaskLabelsDeclared)
	fmt.Fprintf(b, "- task_specs_requiring_approval: `%d`\n", report.TaskSpecsRequiringApproval)
	fmt.Fprintf(b, "- task_specs_issue_native: `%d`\n", report.TaskSpecsIssueNative)
	fmt.Fprintf(b, "- current_issue_task: `%t`\n", report.CurrentIssueTask)
	fmt.Fprintf(b, "- current_task_comments: `%d`\n", report.CurrentTaskComments)
	fmt.Fprintf(b, "- current_task_transcript_messages: `%d`\n", report.CurrentTaskTranscriptMessages)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- task_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- task_storage_backend: `%s`\n", report.TaskStorageBackend)
	fmt.Fprintf(b, "- sqlite_task_db_required: `%t`\n", report.SQLiteTaskDBRequired)
	fmt.Fprintf(b, "- detached_worker_supported: `%t`\n", report.DetachedWorkerSupported)
	fmt.Fprintf(b, "- kanban_dispatcher_supported: `%t`\n", report.KanbanDispatcherSupported)
	fmt.Fprintf(b, "- task_flow_execution_supported: `%t`\n", report.TaskFlowExecutionSupported)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_task_bodies_included: `%t`\n", report.RawTaskBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_task_risk_change: `%t`\n", report.LLME2ERequiredAfterTaskRiskChange)
}

func writeTaskPolicyRiskCard(b *strings.Builder, root string, policy configSurfaceFile) {
	findings := scanTaskPolicyRiskFindings(root, policy)
	if !policy.Present {
		fmt.Fprintf(
			b,
			"- kind=`task-policy` path=`%s` present=`false` loaded_for_model=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			policy.Path,
			len(findings),
			taskRiskMaxSeverity(findings),
			inlineListOrNone(taskRiskCodes(findings)),
			inlineListOrNone(taskRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`task-policy` path=`%s` present=`true` loaded_for_model=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		policy.Path,
		taskPolicyPathInContext(),
		policy.Bytes,
		policy.Lines,
		policy.SHA,
		len(findings),
		taskRiskMaxSeverity(findings),
		inlineListOrNone(taskRiskCodes(findings)),
		inlineListOrNone(taskRiskLineHashes(findings)),
	)
}

func writeTaskSpecRiskCard(b *strings.Builder, root string, spec taskSpecCard) {
	findings := scanTaskSpecRiskFindings(root, spec)
	fmt.Fprintf(
		b,
		"- kind=`task-spec` name=`%s` path=`%s` frontmatter=`%t` kind_field=`%s` mode=`%s` statuses=`%d` labels=`%d` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		inlineCode(spec.Name),
		spec.Path,
		spec.Frontmatter,
		inlineCode(spec.Kind),
		inlineCode(spec.Mode),
		len(spec.Statuses),
		len(spec.Labels),
		spec.RequiresApproval,
		spec.Bytes,
		spec.Lines,
		spec.SHA,
		len(findings),
		taskRiskMaxSeverity(findings),
		inlineListOrNone(taskRiskCodes(findings)),
		inlineListOrNone(taskRiskLineHashes(findings)),
	)
}

func scanTaskPolicyRiskFindings(root string, policy configSurfaceFile) []TaskRiskFinding {
	var findings []TaskRiskFinding
	if !policy.Present {
		findings = append(findings, TaskRiskFinding{
			Severity: "info",
			Code:     "task_policy_not_configured",
			Category: "policy",
			Kind:     "task-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "present",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":present"),
		})
		return findings
	}
	if !taskPolicyPathInContext() {
		findings = append(findings, TaskRiskFinding{
			Severity: "high",
			Code:     "task_policy_not_loaded",
			Category: "context",
			Kind:     "task-policy",
			Name:     "policy",
			Path:     policy.Path,
			Field:    "loaded_for_model",
			Line:     0,
			LineSHA:  shortDocumentHash(policy.Path + ":loaded_for_model"),
		})
	}
	findings = append(findings, scanTaskRiskText("task-policy", "policy", policy.Path, "body", readTaskRiskBody(root, policy.Path))...)
	sortTaskRiskFindings(findings)
	return findings
}

func scanTaskSpecRiskFindings(root string, spec taskSpecCard) []TaskRiskFinding {
	var findings []TaskRiskFinding
	if !spec.Frontmatter {
		findings = append(findings, taskSpecMetadataRiskFinding("warning", "task_frontmatter_missing", "metadata", spec, "frontmatter"))
	}
	if strings.TrimSpace(spec.Kind) == "" {
		findings = append(findings, taskSpecMetadataRiskFinding("warning", "task_kind_missing", "metadata", spec, "kind"))
	}
	if !strings.EqualFold(spec.Mode, "issue-native") {
		findings = append(findings, taskSpecMetadataRiskFinding("warning", "task_mode_not_issue_native", "runtime-boundary", spec, "mode"))
	}
	if len(spec.Statuses) == 0 {
		findings = append(findings, taskSpecMetadataRiskFinding("warning", "task_statuses_missing", "metadata", spec, "statuses"))
	}
	if len(spec.Labels) == 0 {
		findings = append(findings, taskSpecMetadataRiskFinding("warning", "task_labels_missing", "metadata", spec, "labels"))
	}
	if !spec.RequiresApproval {
		findings = append(findings, taskSpecMetadataRiskFinding("warning", "task_approval_gate_missing", "approval", spec, "requires_approval"))
	}
	findings = append(findings, scanTaskRiskText("task-spec", spec.Name, spec.Path, "body", readTaskRiskBody(root, spec.Path))...)
	sortTaskRiskFindings(findings)
	return findings
}

func taskSpecMetadataRiskFinding(severity, code, category string, spec taskSpecCard, field string) TaskRiskFinding {
	return TaskRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "task-spec",
		Name:     spec.Name,
		Path:     spec.Path,
		Field:    field,
		Line:     0,
		LineSHA:  shortDocumentHash(spec.Path + ":" + field),
	}
}

func scanTaskRiskText(kind, name, path, field, body string) []TaskRiskFinding {
	var findings []TaskRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range taskTextRiskRules {
			if !taskRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, TaskRiskFinding{
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
	sortTaskRiskFindings(findings)
	return findings
}

func taskRiskRuleMatches(lowerLine string, rule taskRiskRule) bool {
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

func readTaskRiskBody(root, relPath string) string {
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

func writeTaskRiskFindings(b *strings.Builder, findings []TaskRiskFinding) {
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

func taskRiskSurfaceCount(findings []TaskRiskFinding) int {
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

func taskRiskCodes(findings []TaskRiskFinding) []string {
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

func taskRiskLineHashes(findings []TaskRiskFinding) []string {
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

func taskRiskMaxSeverity(findings []TaskRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if taskRiskSeverityRank(finding.Severity) > taskRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func taskRiskSeverityRank(severity string) int {
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

func sortTaskRiskFindings(findings []TaskRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if taskRiskSeverityRank(findings[i].Severity) != taskRiskSeverityRank(findings[j].Severity) {
			return taskRiskSeverityRank(findings[i].Severity) > taskRiskSeverityRank(findings[j].Severity)
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
