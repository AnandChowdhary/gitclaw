package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const heartbeatWorkflowPath = ".github/workflows/gitclaw-heartbeat.yml"
const heartbeatContextPath = ".gitclaw/HEARTBEAT.md"

type heartbeatSurface struct {
	Workflow heartbeatWorkflow
	Context  configSurfaceFile
}

type heartbeatWorkflow struct {
	Path                  string
	Present               bool
	Bytes                 int
	Lines                 int
	SHA                   string
	WorkflowDispatch      bool
	Schedule              bool
	ScheduleEntries       int
	CronExpressions       []string
	ContentsRead          bool
	IssuesWrite           bool
	ModelsRead            bool
	ContentsWrite         bool
	ActionsWrite          bool
	ConcurrencyGroup      bool
	ConcurrencyCancelSafe bool
	Inputs                int
}

type heartbeatFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsHeartbeatReportRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 || fields[0] != "/heartbeat" {
		return false
	}
	return len(fields) < 2 || (!strings.EqualFold(fields[1], "risk") && !strings.EqualFold(fields[1], "risk-audit"))
}

func RenderHeartbeatReport(ev Event, cfg Config, comments []Comment) string {
	return renderHeartbeatReport(ev, cfg, comments, true)
}

func RenderHeartbeatCLIReport(cfg Config) string {
	return renderHeartbeatReport(Event{}, cfg, nil, false)
}

func renderHeartbeatReport(ev Event, cfg Config, comments []Comment, includeIssue bool) string {
	surface := inspectHeartbeatSurface(cfg.Workdir)
	findings := heartbeatFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Heartbeat Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- heartbeat_report_status: `%s`\n", heartbeatStatus(findings))
	fmt.Fprintf(&b, "- heartbeat_label: `%s`\n", cfg.HeartbeatLabel)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- disabled_label: `%s`\n", cfg.DisabledLabel)
	fmt.Fprintf(&b, "- workflow_path: `%s`\n", heartbeatWorkflowPath)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.Workflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.Workflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- schedule_trigger: `%t`\n", surface.Workflow.Schedule)
	fmt.Fprintf(&b, "- schedule_entries: `%d`\n", surface.Workflow.ScheduleEntries)
	fmt.Fprintf(&b, "- permissions_contents_read: `%t`\n", surface.Workflow.ContentsRead)
	fmt.Fprintf(&b, "- permissions_issues_write: `%t`\n", surface.Workflow.IssuesWrite)
	fmt.Fprintf(&b, "- permissions_models_read: `%t`\n", surface.Workflow.ModelsRead)
	fmt.Fprintf(&b, "- workflow_inputs: `%d`\n", surface.Workflow.Inputs)
	fmt.Fprintf(&b, "- heartbeat_context_path: `%s`\n", heartbeatContextPath)
	fmt.Fprintf(&b, "- heartbeat_context_present: `%t`\n", surface.Context.Present)
	fmt.Fprintf(&b, "- default_limit: `%d`\n", 3)
	fmt.Fprintf(&b, "- slot_strategy: `%s`\n", "utc-hour-or-explicit")
	fmt.Fprintf(&b, "- idempotency_marker: `%s`\n", "gitclaw:heartbeat")
	fmt.Fprintf(&b, "- quiet_response: `%s`\n", "HEARTBEAT_OK")
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- runner_model_call_required: `%t`\n", true)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- issue_scan_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_heartbeat_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- heartbeat_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.HeartbeatLabel))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- heartbeat_comments_now: `%d`\n", countHeartbeatComments(comments))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits the checked-in heartbeat surface. It does not scan heartbeat issues or call a model; the scheduled runner does both through GitHub Actions and GitHub Models. Workflow bodies, issue bodies, comments, and heartbeat context bodies are not included.\n\n")

	b.WriteString("### Workflow\n")
	writeHeartbeatWorkflowInfo(&b, surface.Workflow)

	b.WriteString("\n### Heartbeat Context\n")
	writeConfigSurfaceFile(&b, surface.Context)

	b.WriteString("\n### Runtime Contract\n")
	b.WriteString("- `gitclaw heartbeat --repo <owner/repo>` runs the actual heartbeat scanner and may call the configured model\n")
	b.WriteString("- `gitclaw heartbeat status` renders this read-only report and never calls the model\n")
	b.WriteString("- `.github/workflows/gitclaw-heartbeat.yml` supports manual `workflow_dispatch` and scheduled UTC cron dispatch\n")
	b.WriteString("- the runner lists open issues with the heartbeat label, skips disabled issues and pull requests, and writes at most one heartbeat comment per slot\n")
	b.WriteString("- model output `HEARTBEAT_OK` suppresses visible comments for quiet turns\n")

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

func inspectHeartbeatSurface(root string) heartbeatSurface {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return heartbeatSurface{
			Workflow: heartbeatWorkflow{Path: heartbeatWorkflowPath},
			Context:  configSurfaceFile{Path: heartbeatContextPath},
		}
	}
	return heartbeatSurface{
		Workflow: inspectHeartbeatWorkflow(absRoot, heartbeatWorkflowPath),
		Context:  inspectConfigSurfaceFile(absRoot, heartbeatContextPath),
	}
}

func inspectHeartbeatWorkflow(absRoot, rel string) heartbeatWorkflow {
	workflow := heartbeatWorkflow{Path: rel}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(rel)))
	if err != nil {
		return workflow
	}
	text := string(body)
	workflow.Present = true
	workflow.Bytes = len(body)
	workflow.Lines = lineCount(text)
	workflow.SHA = shortDocumentHash(text)
	workflow.WorkflowDispatch = strings.Contains(text, "workflow_dispatch:")
	workflow.Schedule = strings.Contains(text, "schedule:")
	workflow.CronExpressions = extractCronExpressions(text)
	workflow.ScheduleEntries = len(workflow.CronExpressions)
	workflow.ContentsRead = strings.Contains(text, "contents: read")
	workflow.IssuesWrite = strings.Contains(text, "issues: write")
	workflow.ModelsRead = strings.Contains(text, "models: read")
	workflow.ContentsWrite = strings.Contains(text, "contents: write")
	workflow.ActionsWrite = strings.Contains(text, "actions: write")
	workflow.ConcurrencyGroup = strings.Contains(text, "group: gitclaw-heartbeat")
	workflow.ConcurrencyCancelSafe = strings.Contains(text, "cancel-in-progress: false")
	workflow.Inputs = countWorkflowInputKeys(text)
	return workflow
}

func writeHeartbeatWorkflowInfo(b *strings.Builder, workflow heartbeatWorkflow) {
	if !workflow.Present {
		fmt.Fprintf(b, "- `%s` present=`false`\n", workflow.Path)
		return
	}
	fmt.Fprintf(
		b,
		"- `%s` present=`true` bytes=`%d` lines=`%d` workflow_dispatch=`%t` schedule=`%t` schedule_entries=`%d` contents_read=`%t` issues_write=`%t` models_read=`%t` inputs=`%d` sha256_12=`%s`\n",
		workflow.Path,
		workflow.Bytes,
		workflow.Lines,
		workflow.WorkflowDispatch,
		workflow.Schedule,
		workflow.ScheduleEntries,
		workflow.ContentsRead,
		workflow.IssuesWrite,
		workflow.ModelsRead,
		workflow.Inputs,
		workflow.SHA,
	)
}

func heartbeatFindings(surface heartbeatSurface) []heartbeatFinding {
	var findings []heartbeatFinding
	if !surface.Workflow.Present {
		findings = append(findings, heartbeatFinding{"error", "workflow_missing", heartbeatWorkflowPath, "heartbeat workflow is missing"})
		return findings
	}
	if !surface.Workflow.WorkflowDispatch {
		findings = append(findings, heartbeatFinding{"error", "workflow_dispatch_missing", heartbeatWorkflowPath, "heartbeat workflow cannot be run manually"})
	}
	if !surface.Workflow.Schedule {
		findings = append(findings, heartbeatFinding{"error", "schedule_missing", heartbeatWorkflowPath, "heartbeat workflow has no scheduled trigger"})
	}
	if !surface.Workflow.ContentsRead {
		findings = append(findings, heartbeatFinding{"error", "contents_read_missing", heartbeatWorkflowPath, "heartbeat workflow cannot read repo context"})
	}
	if !surface.Workflow.IssuesWrite {
		findings = append(findings, heartbeatFinding{"error", "issues_write_missing", heartbeatWorkflowPath, "heartbeat workflow cannot post heartbeat comments"})
	}
	if !surface.Workflow.ModelsRead {
		findings = append(findings, heartbeatFinding{"error", "models_read_missing", heartbeatWorkflowPath, "heartbeat workflow cannot call GitHub Models"})
	}
	if surface.Workflow.Inputs < 3 {
		findings = append(findings, heartbeatFinding{"warning", "workflow_inputs_incomplete", heartbeatWorkflowPath, "expected label, slot, and limit inputs"})
	}
	if !surface.Context.Present {
		findings = append(findings, heartbeatFinding{"warning", "heartbeat_context_missing", heartbeatContextPath, "heartbeat prompt context is missing"})
	}
	return findings
}

func heartbeatStatus(findings []heartbeatFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "error"
		}
	}
	if len(findings) > 0 {
		return "warning"
	}
	return "ok"
}

func countHeartbeatComments(comments []Comment) int {
	count := 0
	for _, comment := range comments {
		if HasHeartbeatMarker(comment.Body) {
			count++
		}
	}
	return count
}
