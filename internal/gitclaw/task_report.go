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

const taskPolicyPath = ".gitclaw/TASKS.md"
const taskSpecsDir = ".gitclaw/tasks"

type taskSurface struct {
	Policy configSurfaceFile
	Specs  []taskSpecCard
}

type taskSpecCard struct {
	Name             string
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	Frontmatter      bool
	Kind             string
	Mode             string
	Statuses         []string
	Labels           []string
	RequiresApproval bool
}

type taskSpecFrontmatter struct {
	Name             string   `yaml:"name"`
	Kind             string   `yaml:"kind"`
	Mode             string   `yaml:"mode"`
	Statuses         []string `yaml:"statuses"`
	Labels           []string `yaml:"labels"`
	RequiresApproval bool     `yaml:"requires_approval"`
}

type taskFinding struct {
	Severity string
	Code     string
	Subject  string
	Message  string
}

func IsTaskReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/tasks" || command == "/task"
}

func RenderTaskReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) string {
	if isTaskRiskRequest(ev, cfg) {
		return renderTaskRiskReport(ev, cfg, comments, transcript, true)
	}
	return renderTaskReport(ev, cfg, comments, transcript, true)
}

func RenderTaskCLIReport(cfg Config) string {
	return renderTaskReport(Event{}, cfg, nil, nil, false)
}

func RenderTaskRiskCLIReport(cfg Config) string {
	return renderTaskRiskReport(Event{}, cfg, nil, nil, false)
}

func renderTaskReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage, includeIssue bool) string {
	surface := inspectTaskSurface(cfg.Workdir)
	findings := taskFindings(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Tasks Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- tasks_status: `%s`\n", taskStatus(surface, findings))
	fmt.Fprintf(&b, "- task_policy_path: `%s`\n", taskPolicyPath)
	fmt.Fprintf(&b, "- task_policy_present: `%t`\n", surface.Policy.Present)
	fmt.Fprintf(&b, "- task_policy_loaded_for_model: `%t`\n", taskPolicyLoadedForModel(surface))
	fmt.Fprintf(&b, "- task_specs_dir: `%s`\n", taskSpecsDir)
	fmt.Fprintf(&b, "- task_specs: `%d`\n", len(surface.Specs))
	fmt.Fprintf(&b, "- task_specs_with_frontmatter: `%d`\n", taskSpecsWithFrontmatter(surface.Specs))
	fmt.Fprintf(&b, "- task_statuses_declared: `%d`\n", taskStatusCount(surface.Specs))
	fmt.Fprintf(&b, "- task_labels_declared: `%d`\n", taskLabelCount(surface.Specs))
	fmt.Fprintf(&b, "- task_specs_requiring_approval: `%d`\n", taskSpecsRequiringApproval(surface.Specs))
	fmt.Fprintf(&b, "- task_specs_issue_native: `%d`\n", taskSpecsIssueNative(surface.Specs))
	fmt.Fprintf(&b, "- task_storage_backend: `%s`\n", "github-issues")
	fmt.Fprintf(&b, "- sqlite_task_db_required: `%t`\n", false)
	fmt.Fprintf(&b, "- detached_worker_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- kanban_dispatcher_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- task_flow_execution_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_task_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- current_issue_task: `%t`\n", true)
		fmt.Fprintf(&b, "- current_task_status: `%s`\n", currentIssueTaskStatus(ev, cfg))
		fmt.Fprintf(&b, "- current_task_labels: `%d`\n", len(ev.Issue.Labels))
		fmt.Fprintf(&b, "- current_task_comments: `%d`\n", len(comments))
		fmt.Fprintf(&b, "- current_task_transcript_messages: `%d`\n", len(transcript))
		fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Tasks describe durable work tracking. GitClaw treats GitHub issues as the task ledger and task specs as reviewed metadata only: no detached worker is spawned, no Kanban dispatcher is started, no SQLite task board is opened, and no issue, comment, task, flow, or worker output body is printed by this report.\n\n")

	b.WriteString("### Task Policy\n")
	writeConfigSurfaceFile(&b, surface.Policy)
	if taskPolicyPathInContext() {
		b.WriteString("- `.gitclaw/TASKS.md` loaded=`true` source=`contextDocumentPaths`\n")
	} else {
		b.WriteString("- `.gitclaw/TASKS.md` loaded=`false` source=`contextDocumentPaths`\n")
	}

	b.WriteString("\n### Task Specs\n")
	if len(surface.Specs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, spec := range surface.Specs {
			fmt.Fprintf(
				&b,
				"- name=`%s` path=`%s` bytes=`%d` lines=`%d` frontmatter=`%t` kind=`%s` mode=`%s` statuses=`%d` labels=`%d` requires_approval=`%t` sha256_12=`%s`\n",
				inlineCode(spec.Name),
				spec.Path,
				spec.Bytes,
				spec.Lines,
				spec.Frontmatter,
				inlineCode(spec.Kind),
				inlineCode(spec.Mode),
				len(spec.Statuses),
				len(spec.Labels),
				spec.RequiresApproval,
				spec.SHA,
			)
		}
	}

	b.WriteString("\n### Current Issue Task\n")
	if includeIssue {
		fmt.Fprintf(&b, "- status=`%s` labels=`%d` comments=`%d` transcript_messages=`%d`\n", currentIssueTaskStatus(ev, cfg), len(ev.Issue.Labels), len(comments), len(transcript))
		fmt.Fprintf(&b, "- trigger_label_present=`%t` running_label_present=`%t` done_label_present=`%t` error_label_present=`%t` disabled_label_present=`%t`\n", hasLabel(ev.Issue.Labels, cfg.TriggerLabel), hasLabel(ev.Issue.Labels, cfg.RunningLabel), hasLabel(ev.Issue.Labels, cfg.DoneLabel), hasLabel(ev.Issue.Labels, cfg.ErrorLabel), hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- needs_human_label_present=`%t` write_requested_label_present=`%t`\n", hasLabel(ev.Issue.Labels, defaultNeedsHumanLabel), hasLabel(ev.Issue.Labels, cfg.WriteRequestedLabel))
	} else {
		b.WriteString("- scope=`local-cli` current_issue_task=`false`\n")
	}

	b.WriteString("\n### Runtime Boundary\n")
	b.WriteString("- GitHub issues are the durable task rows for GitClaw v1\n")
	b.WriteString("- labels are the issue-native state machine; comments are the handoff log\n")
	b.WriteString("- task flow specs are audited as metadata and do not start background workers\n")
	b.WriteString("- future workers require reviewed workflows, explicit permissions, approval gates, and body-free audit cards\n")

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

func inspectTaskSurface(root string) taskSurface {
	if root == "" {
		root = "."
	}
	surface := taskSurface{
		Policy: inspectConfigSurfaceFile(root, taskPolicyPath),
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}
	surface.Specs = inspectTaskSpecs(absRoot)
	return surface
}

func inspectTaskSpecs(absRoot string) []taskSpecCard {
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "tasks", "*.md"))
	sort.Strings(matches)
	specs := make([]taskSpecCard, 0, len(matches))
	for _, match := range matches {
		body, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		relPath := filepath.ToSlash(rel)
		text := string(body)
		spec := taskSpecCard{
			Name:    strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)),
			Path:    relPath,
			Present: true,
			Bytes:   len(body),
			Lines:   lineCount(text),
			SHA:     shortDocumentHash(text),
		}
		if meta, ok := parseTaskFrontmatter(text); ok {
			spec.Frontmatter = true
			if strings.TrimSpace(meta.Name) != "" {
				spec.Name = strings.TrimSpace(meta.Name)
			}
			spec.Kind = strings.TrimSpace(meta.Kind)
			spec.Mode = strings.TrimSpace(meta.Mode)
			spec.Statuses = cleanTaskList(meta.Statuses)
			spec.Labels = cleanTaskList(meta.Labels)
			spec.RequiresApproval = meta.RequiresApproval
		}
		specs = append(specs, spec)
	}
	return specs
}

func parseTaskFrontmatter(text string) (taskSpecFrontmatter, bool) {
	var meta taskSpecFrontmatter
	if !strings.HasPrefix(text, "---\n") {
		return meta, false
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, false
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(rest[:end])))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil {
		return taskSpecFrontmatter{}, false
	}
	return meta, true
}

func taskFindings(surface taskSurface) []taskFinding {
	var findings []taskFinding
	if !surface.Policy.Present {
		findings = append(findings, taskFinding{"info", "task_policy_not_configured", taskPolicyPath, "no task policy file is configured"})
	}
	if surface.Policy.Present && !taskPolicyPathInContext() {
		findings = append(findings, taskFinding{"error", "task_policy_not_loaded", taskPolicyPath, "task policy file is not in the model context allowlist"})
	}
	for _, spec := range surface.Specs {
		if !spec.Frontmatter {
			findings = append(findings, taskFinding{"warning", "task_frontmatter_missing", spec.Path, "task spec should start with YAML frontmatter"})
		}
		if strings.TrimSpace(spec.Kind) == "" {
			findings = append(findings, taskFinding{"warning", "task_kind_missing", spec.Path, "task spec should declare a kind such as board, queue, or flow"})
		}
		if !strings.EqualFold(spec.Mode, "issue-native") {
			findings = append(findings, taskFinding{"warning", "task_mode_not_issue_native", spec.Path, "GitClaw v1 only supports issue-native task specs"})
		}
		if len(spec.Statuses) == 0 {
			findings = append(findings, taskFinding{"warning", "task_statuses_missing", spec.Path, "task spec should declare issue-native statuses"})
		}
		if len(spec.Labels) == 0 {
			findings = append(findings, taskFinding{"warning", "task_labels_missing", spec.Path, "task spec should declare GitHub labels that represent task state"})
		}
		if !spec.RequiresApproval {
			findings = append(findings, taskFinding{"warning", "task_approval_gate_missing", spec.Path, "task spec should require approval before workers or mutating side effects"})
		}
	}
	return findings
}

func taskStatus(surface taskSurface, findings []taskFinding) string {
	if !surface.Policy.Present && len(surface.Specs) == 0 {
		return "not_configured"
	}
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "error"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "warning"
		}
	}
	return "ok"
}

func currentIssueTaskStatus(ev Event, cfg Config) string {
	switch {
	case hasLabel(ev.Issue.Labels, cfg.DisabledLabel):
		return "disabled"
	case hasLabel(ev.Issue.Labels, cfg.ErrorLabel):
		return "error"
	case hasLabel(ev.Issue.Labels, cfg.RunningLabel):
		return "running"
	case hasLabel(ev.Issue.Labels, cfg.DoneLabel):
		return "done"
	case hasLabel(ev.Issue.Labels, defaultNeedsHumanLabel):
		return "blocked"
	case hasLabel(ev.Issue.Labels, cfg.WriteRequestedLabel):
		return "awaiting_approval"
	case hasLabel(ev.Issue.Labels, cfg.TriggerLabel):
		return "ready"
	default:
		return "triage"
	}
}

func taskPolicyLoadedForModel(surface taskSurface) bool {
	return surface.Policy.Present && taskPolicyPathInContext()
}

func taskPolicyPathInContext() bool {
	for _, path := range contextDocumentPaths {
		if path == taskPolicyPath {
			return true
		}
	}
	return false
}

func cleanTaskList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}

func taskSpecsWithFrontmatter(specs []taskSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.Frontmatter {
			count++
		}
	}
	return count
}

func taskStatusCount(specs []taskSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.Statuses)
	}
	return count
}

func taskLabelCount(specs []taskSpecCard) int {
	count := 0
	for _, spec := range specs {
		count += len(spec.Labels)
	}
	return count
}

func taskSpecsRequiringApproval(specs []taskSpecCard) int {
	count := 0
	for _, spec := range specs {
		if spec.RequiresApproval {
			count++
		}
	}
	return count
}

func taskSpecsIssueNative(specs []taskSpecCard) int {
	count := 0
	for _, spec := range specs {
		if strings.EqualFold(spec.Mode, "issue-native") {
			count++
		}
	}
	return count
}

func isTaskRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/tasks" && command != "/task" {
		return false
	}
	return strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit")
}
