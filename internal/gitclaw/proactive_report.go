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
	Path  string
	Bytes int
	Lines int
	SHA   string
}

func IsProactiveReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/proactive" || command == "/cron"
}

func RenderProactiveReport(ev Event, cfg Config) string {
	surface := inspectProactiveSurface(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Proactive Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- proactive_label: `%s`\n", cfg.ProactiveLabel)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- workflow_path: `%s`\n", proactiveWorkflowPath)
	fmt.Fprintf(&b, "- workflow_present: `%t`\n", surface.Workflow.Present)
	fmt.Fprintf(&b, "- workflow_dispatch_trigger: `%t`\n", surface.Workflow.WorkflowDispatch)
	fmt.Fprintf(&b, "- schedule_trigger: `%t`\n", surface.Workflow.Schedule)
	fmt.Fprintf(&b, "- prompt_files: `%d`\n", len(surface.Prompts))
	fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", HasProactiveRunMarker(ev.Issue.Body))
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n\n", shortDocumentHash(ev.Issue.Title))
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
			fmt.Fprintf(&b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", prompt.Path, prompt.Bytes, prompt.Lines, prompt.SHA)
		}
	}

	b.WriteString("\n### Enqueue Contract\n")
	b.WriteString("- `gitclaw proactive enqueue --name <name> --slot <slot> --prompt-file .gitclaw/proactive/<name>.md`\n")
	b.WriteString("- one issue per `name + slot`\n")
	b.WriteString("- dispatch id: `proactive-<name>-<slot>`\n")

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

	if body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(proactiveWorkflowPath))); err == nil {
		text := string(body)
		surface.Workflow.Present = true
		surface.Workflow.Bytes = len(body)
		surface.Workflow.Lines = lineCount(text)
		surface.Workflow.SHA = shortDocumentHash(text)
		surface.Workflow.WorkflowDispatch = strings.Contains(text, "workflow_dispatch:")
		surface.Workflow.Schedule = strings.Contains(text, "schedule:")
	}

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
		surface.Prompts = append(surface.Prompts, proactivePrompt{
			Path:  filepath.ToSlash(rel),
			Bytes: len(body),
			Lines: lineCount(text),
			SHA:   shortDocumentHash(text),
		})
	}
	return surface
}
