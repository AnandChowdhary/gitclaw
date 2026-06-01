package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type proactiveScheduleSurface struct {
	Workflows []proactiveScheduleWorkflow
	Prompts   []proactivePrompt
}

type proactiveScheduleWorkflow struct {
	Workflow         proactiveWorkflow
	Name             string
	Crons            []string
	CronHashes       []string
	Cadences         []string
	NotBeforeSupport bool
	PromptFileRefs   int
}

func RenderProactiveScheduleReport(ev Event, cfg Config) string {
	return renderProactiveScheduleReport(ev, cfg, true)
}

func RenderProactiveScheduleCLIReport(cfg Config) string {
	return renderProactiveScheduleReport(Event{}, cfg, false)
}

func renderProactiveScheduleReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectProactiveScheduleSurface(cfg.Workdir)
	status := proactiveScheduleStatus(surface)

	var b strings.Builder
	b.WriteString("## GitClaw Proactive Schedule Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- requested_proactive_command: `%s`\n", "schedule")
		fmt.Fprintf(&b, "- proactive_command_status: `%s`\n", "ok")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- proactive_schedule_status: `%s`\n", status)
	fmt.Fprintf(&b, "- schedule_strategy: `%s`\n", "github-actions-cron-to-issue-dispatch")
	fmt.Fprintf(&b, "- upstream_pattern: `%s`\n", "openclaw-cron-hermes-cron-skill-backed-fresh-session")
	fmt.Fprintf(&b, "- scheduler_runtime: `%s`\n", "GitHub Actions schedule")
	fmt.Fprintf(&b, "- state_storage: `%s`\n", "gitclaw:proactive-run issues")
	fmt.Fprintf(&b, "- workflow_files_indexed: `%d`\n", len(surface.Workflows))
	fmt.Fprintf(&b, "- workflow_files_present: `%d`\n", proactiveSchedulePresentWorkflows(surface.Workflows))
	fmt.Fprintf(&b, "- scheduled_workflows: `%d`\n", proactiveScheduleScheduledWorkflows(surface.Workflows))
	fmt.Fprintf(&b, "- workflow_dispatch_workflows: `%d`\n", proactiveScheduleDispatchWorkflows(surface.Workflows))
	fmt.Fprintf(&b, "- cron_entries: `%d`\n", proactiveScheduleCronEntries(surface.Workflows))
	fmt.Fprintf(&b, "- cron_entries_valid: `%d`\n", proactiveScheduleValidCronEntries(surface.Workflows))
	fmt.Fprintf(&b, "- prompt_files: `%d`\n", len(surface.Prompts))
	fmt.Fprintf(&b, "- skill_backed_prompt_files: `%d`\n", proactiveScheduleSkillBackedPrompts(surface.Prompts))
	fmt.Fprintf(&b, "- prompt_skill_hints: `%d`\n", proactivePromptSkillHintCount(surface.Prompts))
	fmt.Fprintf(&b, "- not_before_supported_workflows: `%d`\n", proactiveScheduleNotBeforeWorkflows(surface.Workflows))
	fmt.Fprintf(&b, "- exact_timing_supported: `%t`\n", proactiveScheduleCronEntries(surface.Workflows) > 0)
	fmt.Fprintf(&b, "- heartbeat_is_approximate_channel: `%t`\n", true)
	fmt.Fprintf(&b, "- fresh_issue_thread_per_name_slot: `%t`\n", true)
	fmt.Fprintf(&b, "- recursive_schedule_creation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- no_agent_mode_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_workflow_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_proactive_schedule_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This schedule report maps GitClaw's reviewed GitHub Actions cron jobs to proactive issue threads. It adapts OpenClaw's exact cron-vs-heartbeat split and Hermes' skill-backed cron jobs while keeping workflow bodies, prompt bodies, issue/comment bodies, tool outputs, credentials, and secret values out of the report.\n\n")

	b.WriteString("### Workflow Schedule Entries\n")
	if len(surface.Workflows) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, workflow := range surface.Workflows {
			writeProactiveScheduleWorkflowCard(&b, workflow)
		}
	}

	b.WriteString("\n### Prompt Schedule Cards\n")
	if len(surface.Prompts) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, prompt := range surface.Prompts {
			fmt.Fprintf(&b, "- kind=`prompt-schedule` name=`%s` path=`%s` bytes=`%d` lines=`%d` skill_hints=`%s` sha256_12=`%s` raw_prompt_body_included=`false`\n",
				prompt.Name,
				prompt.Path,
				prompt.Bytes,
				prompt.Lines,
				inlineListOrNone(prompt.SkillHints),
				prompt.SHA,
			)
		}
	}

	b.WriteString("\n### Schedule Gates\n")
	fmt.Fprintf(&b, "- proactive_schedule_gate=`%s`\n", status)
	b.WriteString("- schedule_source_gate=`reviewed-github-workflow`\n")
	b.WriteString("- exact_timing_gate=`github-actions-cron`\n")
	b.WriteString("- heartbeat_boundary_gate=`heartbeat-is-approximate-monitoring-not-exact-schedule`\n")
	b.WriteString("- issue_thread_gate=`one-proactive-run-issue-per-name-slot`\n")
	b.WriteString("- dispatch_gate=`workflow-dispatch-to-main-handler`\n")
	b.WriteString("- prompt_body_gate=`metadata-hashes-and-skill-hints-only`\n")
	b.WriteString("- recursive_schedule_gate=`disabled-inside-proactive-run`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func inspectProactiveScheduleSurface(root string) proactiveScheduleSurface {
	if root == "" {
		root = "."
	}
	base := inspectProactiveSurface(root)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return proactiveScheduleSurface{Prompts: base.Prompts}
	}
	workflowPaths := proactiveScheduleWorkflowPaths(absRoot)
	workflows := make([]proactiveScheduleWorkflow, 0, len(workflowPaths))
	for _, relPath := range workflowPaths {
		workflows = append(workflows, inspectProactiveScheduleWorkflow(absRoot, relPath))
	}
	return proactiveScheduleSurface{Workflows: workflows, Prompts: base.Prompts}
}

func proactiveScheduleWorkflowPaths(absRoot string) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(rel string) {
		if rel == "" || seen[rel] {
			return
		}
		seen[rel] = true
		paths = append(paths, rel)
	}
	add(proactiveWorkflowPath)
	matches, _ := filepath.Glob(filepath.Join(absRoot, ".github", "workflows", "gitclaw-proactive*.yml"))
	for _, match := range matches {
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		add(filepath.ToSlash(rel))
	}
	sort.Strings(paths)
	return paths
}

func inspectProactiveScheduleWorkflow(absRoot, relPath string) proactiveScheduleWorkflow {
	body, _ := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	text := string(body)
	workflow := inspectProactiveWorkflowAt(absRoot, relPath)
	crons := parseProactiveCronEntries(text)
	cronHashes := make([]string, 0, len(crons))
	cadences := make([]string, 0, len(crons))
	for _, cron := range crons {
		cronHashes = append(cronHashes, shortDocumentHash(cron))
		cadences = append(cadences, proactiveCronCadence(cron))
	}
	return proactiveScheduleWorkflow{
		Workflow:         workflow,
		Name:             proactiveScheduleWorkflowName(relPath),
		Crons:            crons,
		CronHashes:       cronHashes,
		Cadences:         cadences,
		NotBeforeSupport: strings.Contains(text, "not_before") || strings.Contains(text, "GITCLAW_PROACTIVE_NOT_BEFORE"),
		PromptFileRefs:   strings.Count(text, ".gitclaw/proactive/"),
	}
}

func writeProactiveScheduleWorkflowCard(b *strings.Builder, workflow proactiveScheduleWorkflow) {
	if !workflow.Workflow.Present {
		fmt.Fprintf(b, "- kind=`workflow-schedule` name=`%s` path=`%s` present=`false` workflow_dispatch=`false` schedule=`false` cron_entries=`0` raw_workflow_body_included=`false`\n", workflow.Name, workflow.Workflow.Path)
		return
	}
	if len(workflow.Crons) == 0 {
		fmt.Fprintf(b, "- kind=`workflow-schedule` name=`%s` path=`%s` present=`true` workflow_dispatch=`%t` schedule=`%t` cron_entries=`0` prompt_file_refs=`%d` not_before_supported=`%t` sha256_12=`%s` raw_workflow_body_included=`false`\n",
			workflow.Name,
			workflow.Workflow.Path,
			workflow.Workflow.WorkflowDispatch,
			workflow.Workflow.Schedule,
			workflow.PromptFileRefs,
			workflow.NotBeforeSupport,
			workflow.Workflow.SHA,
		)
		return
	}
	for i, cron := range workflow.Crons {
		fmt.Fprintf(b, "- kind=`workflow-schedule` name=`%s` path=`%s` present=`true` workflow_dispatch=`%t` schedule=`%t` cron_index=`%d` cron=`%s` cron_sha256_12=`%s` cadence=`%s` prompt_file_refs=`%d` not_before_supported=`%t` sha256_12=`%s` raw_workflow_body_included=`false`\n",
			workflow.Name,
			workflow.Workflow.Path,
			workflow.Workflow.WorkflowDispatch,
			workflow.Workflow.Schedule,
			i+1,
			inlineCode(cron),
			workflow.CronHashes[i],
			workflow.Cadences[i],
			workflow.PromptFileRefs,
			workflow.NotBeforeSupport,
			workflow.Workflow.SHA,
		)
	}
}

func parseProactiveCronEntries(body string) []string {
	var crons []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		if !strings.HasPrefix(trimmed, "cron:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "cron:"))
		value = strings.Trim(value, `"'`)
		if value != "" {
			crons = append(crons, value)
		}
	}
	return crons
}

func proactiveCronCadence(cron string) string {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return "invalid"
	}
	minute, hour, dayOfMonth, month, dayOfWeek := fields[0], fields[1], fields[2], fields[3], fields[4]
	if strings.HasPrefix(minute, "*/") || minute == "*" {
		return "sub-hourly"
	}
	if hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		return "hourly"
	}
	if dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		return "daily"
	}
	if dayOfMonth == "*" && month == "*" && dayOfWeek != "*" {
		return "weekly"
	}
	if dayOfMonth != "*" && month == "*" {
		return "monthly"
	}
	return "custom"
}

func proactiveScheduleWorkflowName(path string) string {
	base := strings.TrimSuffix(filepath.Base(filepath.FromSlash(path)), filepath.Ext(path))
	base = strings.TrimPrefix(base, "gitclaw-proactive-")
	if base == "gitclaw-proactive" || base == "" {
		return "generic"
	}
	return normalizeProactiveName(base)
}

func proactiveScheduleStatus(surface proactiveScheduleSurface) string {
	if proactiveSchedulePresentWorkflows(surface.Workflows) == 0 || proactiveScheduleCronEntries(surface.Workflows) == 0 {
		return "warn"
	}
	if len(surface.Prompts) == 0 {
		return "warn"
	}
	return "ok"
}

func proactiveSchedulePresentWorkflows(workflows []proactiveScheduleWorkflow) int {
	count := 0
	for _, workflow := range workflows {
		if workflow.Workflow.Present {
			count++
		}
	}
	return count
}

func proactiveScheduleScheduledWorkflows(workflows []proactiveScheduleWorkflow) int {
	count := 0
	for _, workflow := range workflows {
		if workflow.Workflow.Schedule {
			count++
		}
	}
	return count
}

func proactiveScheduleDispatchWorkflows(workflows []proactiveScheduleWorkflow) int {
	count := 0
	for _, workflow := range workflows {
		if workflow.Workflow.WorkflowDispatch {
			count++
		}
	}
	return count
}

func proactiveScheduleCronEntries(workflows []proactiveScheduleWorkflow) int {
	count := 0
	for _, workflow := range workflows {
		count += len(workflow.Crons)
	}
	return count
}

func proactiveScheduleValidCronEntries(workflows []proactiveScheduleWorkflow) int {
	count := 0
	for _, workflow := range workflows {
		for _, cadence := range workflow.Cadences {
			if cadence != "invalid" {
				count++
			}
		}
	}
	return count
}

func proactiveScheduleNotBeforeWorkflows(workflows []proactiveScheduleWorkflow) int {
	count := 0
	for _, workflow := range workflows {
		if workflow.NotBeforeSupport {
			count++
		}
	}
	return count
}

func proactiveScheduleSkillBackedPrompts(prompts []proactivePrompt) int {
	count := 0
	for _, prompt := range prompts {
		if len(prompt.SkillHints) > 0 {
			count++
		}
	}
	return count
}

func isProactiveScheduleRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || !isProactiveCommand(fields[0]) {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "schedule", "schedules", "calendar", "cron":
		return true
	default:
		return false
	}
}
