package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const proactiveWorkflowPath = ".github/workflows/gitclaw-proactive.yml"

type proactiveSurface struct {
	Workflow proactiveWorkflow
	Prompts  []proactivePrompt
}

type proactiveWorkflow struct {
	Path             string
	Present          bool
	Bytes            int
	Lines            int
	SHA              string
	WorkflowDispatch bool
	Schedule         bool
}

type proactivePrompt struct {
	Name       string
	Path       string
	Bytes      int
	Lines      int
	SHA        string
	SkillHints []string
}

func IsProactiveReportRequest(ev Event, cfg Config) bool {
	return isProactiveCommand(activeSlashCommand(ev, cfg))
}

func RenderProactiveReport(ev Event, cfg Config) string {
	if name := requestedProactiveInfoName(ev, cfg); name != "" {
		return renderProactiveInfoReport(ev, cfg, name, true)
	}
	return renderProactiveReport(ev, cfg, true)
}

func RenderProactiveCLIReport(cfg Config) string {
	return renderProactiveReport(Event{}, cfg, false)
}

func RenderProactiveInfoCLIReport(cfg Config, name string) string {
	return renderProactiveInfoReport(Event{}, cfg, name, false)
}

func renderProactiveReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectProactiveSurface(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Proactive Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- proactive_label: `%s`\n", cfg.ProactiveLabel)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- workflow_path: `%s`\n", proactiveWorkflowPath)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.Workflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.Workflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- schedule_trigger: `%t`\n", surface.Workflow.Schedule)
	fmt.Fprintf(&b, "- prompt_files: `%d`\n", len(surface.Prompts))
	fmt.Fprintf(&b, "- prompt_skill_hints: `%d`\n", proactivePromptSkillHintCount(surface.Prompts))
	if includeIssue {
		fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Proactive jobs create or reuse visible GitHub issues, then wake the normal issue handler with `workflow_dispatch`. Prompt bodies and issue bodies are not included in this report.\n\n")

	b.WriteString("### Workflow\n")
	if !surface.Workflow.Present {
		b.WriteString("- none\n")
	} else {
		fmt.Fprintf(
			&b,
			"- `%s` bytes=`%d` lines=`%d` workflow_dispatch=`%t` schedule=`%t` sha256_12=`%s`\n",
			surface.Workflow.Path,
			surface.Workflow.Bytes,
			surface.Workflow.Lines,
			surface.Workflow.WorkflowDispatch,
			surface.Workflow.Schedule,
			surface.Workflow.SHA,
		)
	}

	b.WriteString("\n### Prompt Files\n")
	if len(surface.Prompts) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, prompt := range surface.Prompts {
			fmt.Fprintf(&b, "- `%s` bytes=`%d` lines=`%d` skill_hints=`%d` sha256_12=`%s`\n", prompt.Path, prompt.Bytes, prompt.Lines, len(prompt.SkillHints), prompt.SHA)
		}
	}

	b.WriteString("\n### Enqueue Contract\n")
	b.WriteString("- `gitclaw proactive enqueue --name <name> --slot <slot> --prompt-file .gitclaw/proactive/<name>.md [--not-before <rfc3339-or-date>]`\n")
	b.WriteString("- `--not-before` skips issue creation until the due gate has passed, which supports reminder-style scheduled jobs\n")
	b.WriteString("- one issue per `name + slot`\n")
	b.WriteString("- dispatch id: `proactive-<name>-<slot>`\n")

	return strings.TrimSpace(b.String())
}

func renderProactiveInfoReport(ev Event, cfg Config, name string, includeIssue bool) string {
	name = cleanProactiveLookupName(name)
	surface := inspectProactiveSurface(cfg.Workdir)
	matches := matchingProactivePrompts(surface.Prompts, name)
	generatedWorkflow := inspectProactiveWorkflow(cfg.Workdir, proactiveGeneratedWorkflowPath(name))
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	promptPath := ".gitclaw/proactive/" + name + ".md"
	if len(matches) == 1 {
		promptPath = matches[0].Path
	}

	var b strings.Builder
	b.WriteString("## GitClaw Proactive Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_proactive: `%s`\n", inlineCode(name))
	fmt.Fprintf(&b, "- proactive_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- prompt_matches: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- prompt_skill_hints: `%d`\n", proactivePromptSkillHintCount(matches))
	fmt.Fprintf(&b, "- skill_hints: `%s`\n", inlineList(proactivePromptSkillHintNames(matches)))
	fmt.Fprintf(&b, "- generic_workflow_path: `%s`\n", proactiveWorkflowPath)
	fmt.Fprintf(&b, "- generic_workflow_present: `%t`\n", surface.Workflow.Present)
	fmt.Fprintf(&b, "- generic_workflow_dispatch_trigger: `%t`\n", surface.Workflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- generic_schedule_trigger: `%t`\n", surface.Workflow.Schedule)
	fmt.Fprintf(&b, "- generated_workflow_path: `%s`\n", generatedWorkflow.Path)
	fmt.Fprintf(&b, "- generated_workflow_present: `%t`\n", generatedWorkflow.Present)
	fmt.Fprintf(&b, "- generated_workflow_dispatch_trigger: `%t`\n", generatedWorkflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- generated_schedule_trigger: `%t`\n", generatedWorkflow.Schedule)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report shows metadata for one proactive job definition. Prompt bodies, workflow bodies, issue bodies, comments, and secret values are not included.\n\n")

	b.WriteString("### Prompt Match\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, prompt := range matches {
			fmt.Fprintf(&b, "- `%s` name=`%s` bytes=`%d` lines=`%d` skill_hints=`%s` sha256_12=`%s`\n", prompt.Path, prompt.Name, prompt.Bytes, prompt.Lines, inlineList(prompt.SkillHints), prompt.SHA)
		}
	}

	b.WriteString("\n### Generic Workflow\n")
	writeProactiveWorkflowInfo(&b, surface.Workflow)

	b.WriteString("\n### Generated Workflow Candidate\n")
	writeProactiveWorkflowInfo(&b, generatedWorkflow)

	b.WriteString("\n### Enqueue Contract\n")
	fmt.Fprintf(&b, "- `gitclaw proactive enqueue --name %s --slot <slot> --prompt-file %s [--not-before <rfc3339-or-date>]`\n", name, promptPath)
	fmt.Fprintf(&b, "- dispatch id: `proactive-%s-<slot>`\n", name)
	b.WriteString("- scheduled workflows should dispatch `.github/workflows/gitclaw.yml` only after enqueue returns a nonzero issue number\n")

	if len(matches) == 0 && len(surface.Prompts) > 0 {
		b.WriteString("\n### Available Proactive Jobs\n")
		for _, prompt := range surface.Prompts {
			fmt.Fprintf(&b, "- `%s` path=`%s`\n", prompt.Name, prompt.Path)
		}
	}
	return strings.TrimSpace(b.String())
}

func inspectProactiveSurface(root string) proactiveSurface {
	if root == "" {
		root = "."
	}
	surface := proactiveSurface{
		Workflow: proactiveWorkflow{Path: proactiveWorkflowPath},
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return surface
	}

	surface.Workflow = inspectProactiveWorkflowAt(absRoot, proactiveWorkflowPath)

	matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "proactive", "*.md"))
	sort.Strings(matches)
	for _, match := range matches {
		body, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		text := string(body)
		relPath := filepath.ToSlash(rel)
		surface.Prompts = append(surface.Prompts, proactivePrompt{
			Name:       proactivePromptName(relPath),
			Path:       relPath,
			Bytes:      len(body),
			Lines:      lineCount(text),
			SHA:        shortDocumentHash(text),
			SkillHints: parseProactiveSkillHints(text),
		})
	}
	return surface
}

func inspectProactiveWorkflow(root, relPath string) proactiveWorkflow {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return proactiveWorkflow{Path: relPath}
	}
	return inspectProactiveWorkflowAt(absRoot, relPath)
}

func inspectProactiveWorkflowAt(absRoot, relPath string) proactiveWorkflow {
	workflow := proactiveWorkflow{Path: relPath}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
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
	return workflow
}

func writeProactiveWorkflowInfo(b *strings.Builder, workflow proactiveWorkflow) {
	if !workflow.Present {
		fmt.Fprintf(b, "- `%s` present=`false`\n", workflow.Path)
		return
	}
	fmt.Fprintf(
		b,
		"- `%s` present=`true` bytes=`%d` lines=`%d` workflow_dispatch=`%t` schedule=`%t` sha256_12=`%s`\n",
		workflow.Path,
		workflow.Bytes,
		workflow.Lines,
		workflow.WorkflowDispatch,
		workflow.Schedule,
		workflow.SHA,
	)
}

func requestedProactiveInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || !isProactiveCommand(fields[0]) || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanProactiveLookupName(fields[2])
}

func matchingProactivePrompts(prompts []proactivePrompt, name string) []proactivePrompt {
	name = cleanProactiveLookupName(name)
	if name == "" {
		return nil
	}
	var matches []proactivePrompt
	for _, prompt := range prompts {
		if prompt.Name == name || cleanProactiveLookupName(prompt.Path) == name {
			matches = append(matches, prompt)
		}
	}
	return matches
}

func cleanProactiveLookupName(name string) string {
	return normalizeProactiveName(strings.Trim(strings.TrimSpace(name), " \t\r\n.,:;!?`\"'"))
}

func proactivePromptName(path string) string {
	base := filepath.Base(filepath.FromSlash(path))
	return cleanProactiveLookupName(strings.TrimSuffix(base, filepath.Ext(base)))
}

func proactiveGeneratedWorkflowPath(name string) string {
	return ".github/workflows/gitclaw-proactive-" + cleanProactiveLookupName(name) + ".yml"
}

func isProactiveCommand(command string) bool {
	return command == "/proactive" || command == "/cron"
}

func parseProactiveSkillHints(text string) []string {
	const marker = "gitclaw:proactive-skills"
	var hints []string
	remaining := text
	for {
		start := strings.Index(remaining, marker)
		if start < 0 {
			break
		}
		after := remaining[start+len(marker):]
		end := strings.Index(after, "-->")
		if end < 0 {
			break
		}
		hints = append(hints, after[:end])
		remaining = after[end+len("-->"):]
	}
	return normalizeProactiveSkillHints(hints)
}

func proactivePromptSkillHintCount(prompts []proactivePrompt) int {
	return len(proactivePromptSkillHintNames(prompts))
}

func proactivePromptSkillHintNames(prompts []proactivePrompt) []string {
	var hints []string
	for _, prompt := range prompts {
		hints = append(hints, prompt.SkillHints...)
	}
	return normalizeProactiveSkillHints(hints)
}
