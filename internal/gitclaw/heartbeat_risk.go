package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type HeartbeatRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type HeartbeatRiskReport struct {
	Status                                 string
	VerificationScope                      string
	RunMode                                string
	Model                                  string
	WorkflowPath                           string
	WorkflowPresent                        bool
	WorkflowDispatchTrigger                bool
	ScheduleTrigger                        bool
	ScheduleEntries                        int
	TopOfHourScheduleEntries               int
	OffHourScheduleEntries                 int
	FastScheduleEntries                    int
	InvalidScheduleEntries                 int
	PermissionsContentsRead                bool
	PermissionsIssuesWrite                 bool
	PermissionsModelsRead                  bool
	PermissionsContentsWrite               bool
	PermissionsActionsWrite                bool
	WorkflowInputs                         int
	ConcurrencyGroup                       bool
	ConcurrencyCancelSafe                  bool
	HeartbeatContextPath                   string
	HeartbeatContextPresent                bool
	HeartbeatContextBytes                  int
	HeartbeatContextLines                  int
	SurfacesWithRiskFindings               int
	Findings                               []HeartbeatRiskFinding
	HighRiskFindings                       int
	WarningRiskFindings                    int
	InfoRiskFindings                       int
	SchedulerRuntime                       string
	WakeStrategy                           string
	SlotStrategy                           string
	IdempotencyMarker                      string
	QuietResponse                          string
	ModelCallRequired                      bool
	RunnerModelCallRequired                bool
	IssueScanPerformed                     bool
	RepositoryMutationAllowed              bool
	HeartbeatTurnHostExecAllowed           bool
	RawWorkflowBodyIncluded                bool
	RawHeartbeatBodyIncluded               bool
	RawIssueBodiesIncluded                 bool
	RawCommentBodiesIncluded               bool
	RawInputsIncluded                      bool
	CredentialValuesIncluded               bool
	LLME2ERequiredAfterHeartbeatRiskChange bool
}

type heartbeatRiskWorkflow struct {
	Workflow heartbeatWorkflow
	Body     string
}

type heartbeatRiskContext struct {
	File configSurfaceFile
	Body string
}

type heartbeatRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var heartbeatWorkflowRiskRules = []heartbeatRiskRule{
	{
		Severity: "high",
		Code:     "heartbeat_raw_input_logged",
		Category: "body-leakage",
		Any: []string{
			"echo \"${{ github.event.inputs",
			"printf \"%s\" \"${{ github.event.inputs",
			"printf '%s' \"${{ github.event.inputs",
		},
	},
	{
		Severity: "high",
		Code:     "heartbeat_credential_value_logged",
		Category: "credential-handling",
		Any: []string{
			"echo \"$github_token",
			"echo \"$gh_token",
			"echo \"${{ secrets.",
			"printenv",
		},
	},
	{
		Severity: "high",
		Code:     "heartbeat_self_dispatch_loop",
		Category: "runtime-amplification",
		Any: []string{
			"gh workflow run .github/workflows/gitclaw-heartbeat.yml",
			"gh workflow run gitclaw-heartbeat.yml",
			"workflow_dispatch",
		},
		All: []string{
			"gh workflow run",
		},
	},
	{
		Severity: "warning",
		Code:     "heartbeat_unbounded_workflow_loop",
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
	{
		Severity: "warning",
		Code:     "heartbeat_unreviewed_repository_write",
		Category: "write-authority",
		Any: []string{
			"git push",
			"git commit",
			"gh pr merge",
			"gh issue edit",
		},
		IgnoreAny: []string{
			"gitclaw heartbeat --repo",
		},
	},
}

var heartbeatContextRiskRules = []heartbeatRiskRule{
	{
		Severity: "high",
		Code:     "heartbeat_prompt_boundary_override",
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
		Code:     "heartbeat_secret_exfiltration",
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
		Severity: "warning",
		Code:     "heartbeat_unreviewed_persistent_state",
		Category: "persistence",
		Any: []string{
			"write to memory without review",
			"edit memory without review",
			"modify soul without review",
			"append to soul without review",
			"create new schedules",
			"modify files",
			"commit directly",
			"push directly",
		},
		IgnoreAny: []string{
			"do not",
			"never",
		},
	},
	{
		Severity: "warning",
		Code:     "heartbeat_unbounded_task_scope",
		Category: "runtime-amplification",
		Any: []string{
			"retry forever",
			"loop forever",
			"every minute",
			"every 1 minute",
			"never stop",
			"continue indefinitely",
			"read every issue",
			"scan all repositories",
		},
	},
	{
		Severity: "info",
		Code:     "heartbeat_credential_transfer",
		Category: "credential-handling",
		All: []string{
			"api key",
			"send",
		},
	},
}

func IsHeartbeatRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/heartbeat" && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}

func RenderHeartbeatRiskReport(ev Event, cfg Config, comments []Comment) string {
	return renderHeartbeatRiskReport(ev, cfg, comments, true)
}

func RenderHeartbeatRiskCLIReport(cfg Config) string {
	return renderHeartbeatRiskReport(Event{}, cfg, nil, false)
}

func renderHeartbeatRiskReport(ev Event, cfg Config, comments []Comment, includeIssue bool) string {
	report := BuildHeartbeatRiskReport(cfg)

	var b strings.Builder
	b.WriteString("## GitClaw Heartbeat Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeHeartbeatRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- heartbeat_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.HeartbeatLabel))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- heartbeat_comments_now: `%d`\n", countHeartbeatComments(comments))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans the scheduled heartbeat workflow and heartbeat-only context for body leakage, schedule-amplification, idempotency, permission, host-exec, prompt-boundary, persistent-state, and credential risks. It reports only metadata, counts, risk codes, severities, and hashes; workflow bodies, heartbeat context bodies, issue bodies, comments, raw inputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Workflow Risk Cards\n")
	writeHeartbeatWorkflowRiskCard(&b, loadHeartbeatRiskWorkflow(cfg.Workdir, heartbeatWorkflowPath))

	b.WriteString("\n### Heartbeat Context Risk Cards\n")
	writeHeartbeatContextRiskCard(&b, loadHeartbeatRiskContext(cfg.Workdir, heartbeatContextPath))

	b.WriteString("\n### Runtime Boundary Risk Card\n")
	writeHeartbeatRuntimeRiskCard(&b, report)

	b.WriteString("\n### Risk Findings\n")
	writeHeartbeatRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildHeartbeatRiskReport(cfg Config) HeartbeatRiskReport {
	surface := inspectHeartbeatSurface(cfg.Workdir)
	workflow := loadHeartbeatRiskWorkflow(cfg.Workdir, heartbeatWorkflowPath)
	context := loadHeartbeatRiskContext(cfg.Workdir, heartbeatContextPath)
	report := HeartbeatRiskReport{
		Status:                                 "ok",
		VerificationScope:                      "scheduled-heartbeat-workflow-context-and-idempotency",
		RunMode:                                "read-only",
		Model:                                  cfg.Model,
		WorkflowPath:                           heartbeatWorkflowPath,
		WorkflowPresent:                        surface.Workflow.Present,
		WorkflowDispatchTrigger:                surface.Workflow.WorkflowDispatch,
		ScheduleTrigger:                        surface.Workflow.Schedule,
		ScheduleEntries:                        surface.Workflow.ScheduleEntries,
		TopOfHourScheduleEntries:               countCronExpressions(workflow.Workflow.CronExpressions, cronMinuteIncludesTopOfHour),
		OffHourScheduleEntries:                 countCronExpressions(workflow.Workflow.CronExpressions, func(expr string) bool { return !cronMinuteIncludesTopOfHour(expr) && cronFieldCount(expr) == 5 }),
		FastScheduleEntries:                    countCronExpressions(workflow.Workflow.CronExpressions, cronLooksTooFrequent),
		InvalidScheduleEntries:                 countCronExpressions(workflow.Workflow.CronExpressions, func(expr string) bool { return cronFieldCount(expr) != 5 }),
		PermissionsContentsRead:                surface.Workflow.ContentsRead,
		PermissionsIssuesWrite:                 surface.Workflow.IssuesWrite,
		PermissionsModelsRead:                  surface.Workflow.ModelsRead,
		PermissionsContentsWrite:               surface.Workflow.ContentsWrite,
		PermissionsActionsWrite:                surface.Workflow.ActionsWrite,
		WorkflowInputs:                         surface.Workflow.Inputs,
		ConcurrencyGroup:                       surface.Workflow.ConcurrencyGroup,
		ConcurrencyCancelSafe:                  surface.Workflow.ConcurrencyCancelSafe,
		HeartbeatContextPath:                   heartbeatContextPath,
		HeartbeatContextPresent:                surface.Context.Present,
		HeartbeatContextBytes:                  surface.Context.Bytes,
		HeartbeatContextLines:                  surface.Context.Lines,
		SchedulerRuntime:                       "GitHub Actions schedule",
		WakeStrategy:                           "workflow_dispatch-or-schedule",
		SlotStrategy:                           "utc-hour-or-explicit",
		IdempotencyMarker:                      "gitclaw:heartbeat",
		QuietResponse:                          "HEARTBEAT_OK",
		ModelCallRequired:                      false,
		RunnerModelCallRequired:                true,
		IssueScanPerformed:                     false,
		RepositoryMutationAllowed:              false,
		HeartbeatTurnHostExecAllowed:           false,
		RawWorkflowBodyIncluded:                false,
		RawHeartbeatBodyIncluded:               false,
		RawIssueBodiesIncluded:                 false,
		RawCommentBodiesIncluded:               false,
		RawInputsIncluded:                      false,
		CredentialValuesIncluded:               false,
		LLME2ERequiredAfterHeartbeatRiskChange: true,
	}
	report.Findings = append(report.Findings, scanHeartbeatWorkflowRiskFindings(workflow)...)
	report.Findings = append(report.Findings, scanHeartbeatContextRiskFindings(context)...)
	sortHeartbeatRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = heartbeatRiskSurfaceCount(report.Findings)
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

func writeHeartbeatRiskSummary(b *strings.Builder, report HeartbeatRiskReport) {
	fmt.Fprintf(b, "- heartbeat_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- run_mode: `%s`\n", report.RunMode)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- workflow_path: `%s`\n", report.WorkflowPath)
	fmt.Fprintf(b, "- workflow_present: `%t`\n", report.WorkflowPresent)
	fmt.Fprintf(b, "- workflow_dispatch_trigger: `%t`\n", report.WorkflowDispatchTrigger)
	fmt.Fprintf(b, "- schedule_trigger: `%t`\n", report.ScheduleTrigger)
	fmt.Fprintf(b, "- schedule_entries: `%d`\n", report.ScheduleEntries)
	fmt.Fprintf(b, "- top_of_hour_schedule_entries: `%d`\n", report.TopOfHourScheduleEntries)
	fmt.Fprintf(b, "- off_hour_schedule_entries: `%d`\n", report.OffHourScheduleEntries)
	fmt.Fprintf(b, "- fast_schedule_entries: `%d`\n", report.FastScheduleEntries)
	fmt.Fprintf(b, "- invalid_schedule_entries: `%d`\n", report.InvalidScheduleEntries)
	fmt.Fprintf(b, "- permissions_contents_read: `%t`\n", report.PermissionsContentsRead)
	fmt.Fprintf(b, "- permissions_issues_write: `%t`\n", report.PermissionsIssuesWrite)
	fmt.Fprintf(b, "- permissions_models_read: `%t`\n", report.PermissionsModelsRead)
	fmt.Fprintf(b, "- permissions_contents_write: `%t`\n", report.PermissionsContentsWrite)
	fmt.Fprintf(b, "- permissions_actions_write: `%t`\n", report.PermissionsActionsWrite)
	fmt.Fprintf(b, "- workflow_inputs: `%d`\n", report.WorkflowInputs)
	fmt.Fprintf(b, "- concurrency_group: `%t`\n", report.ConcurrencyGroup)
	fmt.Fprintf(b, "- concurrency_cancel_safe: `%t`\n", report.ConcurrencyCancelSafe)
	fmt.Fprintf(b, "- heartbeat_context_path: `%s`\n", report.HeartbeatContextPath)
	fmt.Fprintf(b, "- heartbeat_context_present: `%t`\n", report.HeartbeatContextPresent)
	fmt.Fprintf(b, "- heartbeat_context_bytes: `%d`\n", report.HeartbeatContextBytes)
	fmt.Fprintf(b, "- heartbeat_context_lines: `%d`\n", report.HeartbeatContextLines)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- heartbeat_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- scheduler_runtime: `%s`\n", report.SchedulerRuntime)
	fmt.Fprintf(b, "- wake_strategy: `%s`\n", report.WakeStrategy)
	fmt.Fprintf(b, "- slot_strategy: `%s`\n", report.SlotStrategy)
	fmt.Fprintf(b, "- idempotency_marker: `%s`\n", report.IdempotencyMarker)
	fmt.Fprintf(b, "- quiet_response: `%s`\n", report.QuietResponse)
	fmt.Fprintf(b, "- model_call_required: `%t`\n", report.ModelCallRequired)
	fmt.Fprintf(b, "- runner_model_call_required: `%t`\n", report.RunnerModelCallRequired)
	fmt.Fprintf(b, "- issue_scan_performed: `%t`\n", report.IssueScanPerformed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- heartbeat_turn_host_exec_allowed: `%t`\n", report.HeartbeatTurnHostExecAllowed)
	fmt.Fprintf(b, "- raw_workflow_body_included: `%t`\n", report.RawWorkflowBodyIncluded)
	fmt.Fprintf(b, "- raw_heartbeat_body_included: `%t`\n", report.RawHeartbeatBodyIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_heartbeat_risk_change: `%t`\n", report.LLME2ERequiredAfterHeartbeatRiskChange)
}

func writeHeartbeatWorkflowRiskCard(b *strings.Builder, workflow heartbeatRiskWorkflow) {
	findings := scanHeartbeatWorkflowRiskFindings(workflow)
	if !workflow.Workflow.Present {
		fmt.Fprintf(
			b,
			"- kind=`workflow` path=`%s` present=`false` workflow_dispatch=`false` schedule=`false` schedule_entries=`0` top_of_hour_schedule_entries=`0` off_hour_schedule_entries=`0` fast_schedule_entries=`0` invalid_schedule_entries=`0` contents_read=`false` issues_write=`false` models_read=`false` contents_write=`false` actions_write=`false` concurrency_group=`false` concurrency_cancel_safe=`false` inputs=`0` sha256_12=`none` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			workflow.Workflow.Path,
			len(findings),
			heartbeatRiskMaxSeverity(findings),
			inlineListOrNone(heartbeatRiskCodes(findings)),
			inlineListOrNone(heartbeatRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`workflow` path=`%s` present=`true` workflow_dispatch=`%t` schedule=`%t` schedule_entries=`%d` top_of_hour_schedule_entries=`%d` off_hour_schedule_entries=`%d` fast_schedule_entries=`%d` invalid_schedule_entries=`%d` contents_read=`%t` issues_write=`%t` models_read=`%t` contents_write=`%t` actions_write=`%t` concurrency_group=`%t` concurrency_cancel_safe=`%t` inputs=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		workflow.Workflow.Path,
		workflow.Workflow.WorkflowDispatch,
		workflow.Workflow.Schedule,
		workflow.Workflow.ScheduleEntries,
		countCronExpressions(workflow.Workflow.CronExpressions, cronMinuteIncludesTopOfHour),
		countCronExpressions(workflow.Workflow.CronExpressions, func(expr string) bool { return !cronMinuteIncludesTopOfHour(expr) && cronFieldCount(expr) == 5 }),
		countCronExpressions(workflow.Workflow.CronExpressions, cronLooksTooFrequent),
		countCronExpressions(workflow.Workflow.CronExpressions, func(expr string) bool { return cronFieldCount(expr) != 5 }),
		workflow.Workflow.ContentsRead,
		workflow.Workflow.IssuesWrite,
		workflow.Workflow.ModelsRead,
		workflow.Workflow.ContentsWrite,
		workflow.Workflow.ActionsWrite,
		workflow.Workflow.ConcurrencyGroup,
		workflow.Workflow.ConcurrencyCancelSafe,
		workflow.Workflow.Inputs,
		workflow.Workflow.SHA,
		len(findings),
		heartbeatRiskMaxSeverity(findings),
		inlineListOrNone(heartbeatRiskCodes(findings)),
		inlineListOrNone(heartbeatRiskLineHashes(findings)),
	)
}

func writeHeartbeatContextRiskCard(b *strings.Builder, context heartbeatRiskContext) {
	findings := scanHeartbeatContextRiskFindings(context)
	if !context.File.Present {
		fmt.Fprintf(
			b,
			"- kind=`heartbeat-context` path=`%s` present=`false` body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			context.File.Path,
			len(findings),
			heartbeatRiskMaxSeverity(findings),
			inlineListOrNone(heartbeatRiskCodes(findings)),
			inlineListOrNone(heartbeatRiskLineHashes(findings)),
		)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`heartbeat-context` path=`%s` present=`true` bytes=`%d` lines=`%d` sha256_12=`%s` body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		context.File.Path,
		context.File.Bytes,
		context.File.Lines,
		context.File.SHA,
		len(findings),
		heartbeatRiskMaxSeverity(findings),
		inlineListOrNone(heartbeatRiskCodes(findings)),
		inlineListOrNone(heartbeatRiskLineHashes(findings)),
	)
}

func writeHeartbeatRuntimeRiskCard(b *strings.Builder, report HeartbeatRiskReport) {
	fmt.Fprintf(
		b,
		"- kind=`runtime-boundary` scheduler_runtime=`%s` wake_strategy=`%s` slot_strategy=`%s` idempotency_marker=`%s` quiet_response=`%s` model_call_required=`%t` runner_model_call_required=`%t` issue_scan_performed=`%t` repository_mutation_allowed=`%t` heartbeat_turn_host_exec_allowed=`%t` raw_workflow_body_included=`%t` raw_heartbeat_body_included=`%t` raw_issue_bodies_included=`%t` raw_comment_bodies_included=`%t` raw_inputs_included=`%t` credential_values_included=`%t` risk_findings=`0` risk_max_severity=`none` risk_codes=`none` line_hashes=`none`\n",
		report.SchedulerRuntime,
		report.WakeStrategy,
		report.SlotStrategy,
		report.IdempotencyMarker,
		report.QuietResponse,
		report.ModelCallRequired,
		report.RunnerModelCallRequired,
		report.IssueScanPerformed,
		report.RepositoryMutationAllowed,
		report.HeartbeatTurnHostExecAllowed,
		report.RawWorkflowBodyIncluded,
		report.RawHeartbeatBodyIncluded,
		report.RawIssueBodiesIncluded,
		report.RawCommentBodiesIncluded,
		report.RawInputsIncluded,
		report.CredentialValuesIncluded,
	)
}

func scanHeartbeatWorkflowRiskFindings(workflow heartbeatRiskWorkflow) []HeartbeatRiskFinding {
	var findings []HeartbeatRiskFinding
	if !workflow.Workflow.Present {
		findings = append(findings, heartbeatRiskMetadataFinding("high", "heartbeat_workflow_missing", "workflow-presence", "workflow", workflow.Workflow.Path, "present"))
		sortHeartbeatRiskFindings(findings)
		return findings
	}
	if !workflow.Workflow.WorkflowDispatch {
		findings = append(findings, heartbeatRiskMetadataFinding("high", "workflow_dispatch_missing", "wake-strategy", "workflow", workflow.Workflow.Path, "workflow_dispatch"))
	}
	if !workflow.Workflow.Schedule {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "schedule_trigger_missing", "scheduler", "workflow", workflow.Workflow.Path, "schedule"))
	}
	if !workflow.Workflow.ContentsRead {
		findings = append(findings, heartbeatRiskMetadataFinding("high", "contents_read_permission_missing", "workflow-permission", "workflow", workflow.Workflow.Path, "contents"))
	}
	if !workflow.Workflow.IssuesWrite {
		findings = append(findings, heartbeatRiskMetadataFinding("high", "issues_write_permission_missing", "workflow-permission", "workflow", workflow.Workflow.Path, "issues"))
	}
	if !workflow.Workflow.ModelsRead {
		findings = append(findings, heartbeatRiskMetadataFinding("high", "models_read_permission_missing", "workflow-permission", "workflow", workflow.Workflow.Path, "models"))
	}
	if workflow.Workflow.ContentsWrite {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "contents_write_permission_unexpected", "workflow-permission", "workflow", workflow.Workflow.Path, "contents"))
	}
	if workflow.Workflow.ActionsWrite {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "actions_write_permission_unexpected", "workflow-permission", "workflow", workflow.Workflow.Path, "actions"))
	}
	if workflow.Workflow.Inputs < 3 {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "workflow_inputs_incomplete", "workflow-dispatch", "workflow", workflow.Workflow.Path, "inputs"))
	}
	if !workflow.Workflow.ConcurrencyGroup {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "concurrency_group_missing", "idempotency", "workflow", workflow.Workflow.Path, "concurrency"))
	}
	if workflow.Workflow.ConcurrencyGroup && !workflow.Workflow.ConcurrencyCancelSafe {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "concurrency_cancel_policy_unsafe", "idempotency", "workflow", workflow.Workflow.Path, "cancel-in-progress"))
	}
	for _, cron := range workflow.Workflow.CronExpressions {
		if cronFieldCount(cron) != 5 {
			findings = append(findings, heartbeatRiskMetadataFinding("warning", "invalid_cron_expression", "scheduler", "workflow", workflow.Workflow.Path, "cron"))
			continue
		}
		if cronMinuteIncludesTopOfHour(cron) {
			findings = append(findings, heartbeatRiskMetadataFinding("warning", "top_of_hour_schedule", "scheduler-reliability", "workflow", workflow.Workflow.Path, "cron"))
		}
		if cronLooksTooFrequent(cron) {
			findings = append(findings, heartbeatRiskMetadataFinding("warning", "fast_heartbeat_schedule", "runtime-amplification", "workflow", workflow.Workflow.Path, "cron"))
		}
	}
	findings = append(findings, scanHeartbeatRiskText("workflow", workflow.Workflow.Path, "body", workflow.Body, heartbeatWorkflowRiskRules)...)
	sortHeartbeatRiskFindings(findings)
	return findings
}

func scanHeartbeatContextRiskFindings(context heartbeatRiskContext) []HeartbeatRiskFinding {
	var findings []HeartbeatRiskFinding
	if !context.File.Present {
		findings = append(findings, heartbeatRiskMetadataFinding("warning", "heartbeat_context_missing", "heartbeat-context", "heartbeat-context", context.File.Path, "present"))
		sortHeartbeatRiskFindings(findings)
		return findings
	}
	findings = append(findings, scanHeartbeatRiskText("heartbeat-context", context.File.Path, "body", context.Body, heartbeatContextRiskRules)...)
	sortHeartbeatRiskFindings(findings)
	return findings
}

func scanHeartbeatRiskText(kind, path, field, body string, rules []heartbeatRiskRule) []HeartbeatRiskFinding {
	var findings []HeartbeatRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range rules {
			if !heartbeatRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, HeartbeatRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Path:     path,
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortHeartbeatRiskFindings(findings)
	return findings
}

func heartbeatRiskRuleMatches(lowerLine string, rule heartbeatRiskRule) bool {
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

func heartbeatRiskMetadataFinding(severity, code, category, kind, path, field string) HeartbeatRiskFinding {
	return HeartbeatRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Path:     path,
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + path + ":" + field + ":" + code),
	}
}

func loadHeartbeatRiskWorkflow(root, relPath string) heartbeatRiskWorkflow {
	workflow := inspectHeartbeatWorkflow(absOrDot(root), relPath)
	body := readHeartbeatRiskBody(root, relPath)
	return heartbeatRiskWorkflow{Workflow: workflow, Body: body}
}

func loadHeartbeatRiskContext(root, relPath string) heartbeatRiskContext {
	return heartbeatRiskContext{
		File: inspectConfigSurfaceFile(absOrDot(root), relPath),
		Body: readHeartbeatRiskBody(root, relPath),
	}
}

func readHeartbeatRiskBody(root, relPath string) string {
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

func absOrDot(root string) string {
	if root == "" {
		return "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "."
	}
	return abs
}

func extractCronExpressions(text string) []string {
	var crons []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "cron:") {
			continue
		}
		parts := strings.SplitN(trimmed, "cron:", 2)
		if len(parts) != 2 {
			continue
		}
		expr := strings.TrimSpace(parts[1])
		expr = strings.Trim(expr, "\"'")
		if expr != "" {
			crons = append(crons, expr)
		}
	}
	return crons
}

func countCronExpressions(crons []string, fn func(string) bool) int {
	count := 0
	for _, cron := range crons {
		if fn(cron) {
			count++
		}
	}
	return count
}

func cronFieldCount(expr string) int {
	return len(strings.Fields(expr))
}

func cronMinuteIncludesTopOfHour(expr string) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	return cronMinuteTokenIncludes(fields[0], 0)
}

func cronLooksTooFrequent(expr string) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	minute := fields[0]
	if minute == "*" {
		return true
	}
	for _, part := range strings.Split(minute, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "/") {
			step := part[strings.LastIndex(part, "/")+1:]
			parsed, err := strconv.Atoi(step)
			if err == nil && parsed > 0 && parsed < 15 {
				return true
			}
		}
	}
	return false
}

func cronMinuteTokenIncludes(token string, value int) bool {
	for _, part := range strings.Split(token, ",") {
		part = strings.TrimSpace(part)
		if part == "*" {
			return true
		}
		if strings.Contains(part, "/") {
			part = part[:strings.Index(part, "/")]
		}
		if part == "*" {
			return true
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, startErr := strconv.Atoi(bounds[0])
			end, endErr := strconv.Atoi(bounds[1])
			if startErr == nil && endErr == nil && value >= start && value <= end {
				return true
			}
			continue
		}
		parsed, err := strconv.Atoi(part)
		if err == nil && parsed == value {
			return true
		}
	}
	return false
}

func writeHeartbeatRiskFindings(b *strings.Builder, findings []HeartbeatRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			inlineCode(finding.Path),
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func heartbeatRiskSurfaceCount(findings []HeartbeatRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.Kind+"\x00"+finding.Path] = true
	}
	return len(seen)
}

func heartbeatRiskCodes(findings []HeartbeatRiskFinding) []string {
	var codes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	return sortedStrings(codes)
}

func heartbeatRiskLineHashes(findings []HeartbeatRiskFinding) []string {
	var hashes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	return sortedStrings(hashes)
}

func heartbeatRiskMaxSeverity(findings []HeartbeatRiskFinding) string {
	if len(findings) == 0 {
		return "none"
	}
	max := "info"
	for _, finding := range findings {
		switch finding.Severity {
		case "high":
			return "high"
		case "warning":
			max = "warning"
		}
	}
	return max
}

func sortHeartbeatRiskFindings(findings []HeartbeatRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		if left.Severity != right.Severity {
			return heartbeatRiskSeverityRank(left.Severity) < heartbeatRiskSeverityRank(right.Severity)
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Field < right.Field
	})
}

func heartbeatRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}
