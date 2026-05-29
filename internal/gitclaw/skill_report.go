package gitclaw

import (
	"fmt"
	"strings"
)

func IsSkillsReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/skills"
}

func activeSlashCommand(ev Event, cfg Config) string {
	text := strings.TrimSpace(activeRequestText(ev))
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	prefix := strings.ToLower(cfg.TriggerPrefix)
	if strings.HasPrefix(lower, prefix) {
		text = strings.TrimSpace(text[len(cfg.TriggerPrefix):])
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(strings.ToLower(fields[0]), " \t\r\n.,:;!?")
}

func RenderSkillsReport(ev Event, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("## GitClaw Skills Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n\n", len(repoContext.Skills))
	b.WriteString("GitClaw uses progressive disclosure: this report lists available skill metadata, while full `SKILL.md` bodies are loaded only when selected or marked always-on.\n\n")

	b.WriteString("### Available Skills\n")
	index := skillIndexOutput(repoContext)
	if index == "" {
		b.WriteString("- none\n")
	} else {
		b.WriteString(index)
		b.WriteByte('\n')
	}

	b.WriteString("\n### Selected For This Turn\n")
	writeContextDocumentList(&b, repoContext.Skills)

	return strings.TrimSpace(b.String())
}

func skillIndexOutput(repoContext RepoContext) string {
	for _, output := range repoContext.ToolOutputs {
		if output.Name == "gitclaw.skill_index" {
			return strings.TrimSpace(output.Output)
		}
	}
	return ""
}

func availableSkillCount(repoContext RepoContext) int {
	index := skillIndexOutput(repoContext)
	if index == "" {
		return 0
	}
	return lineCount(index)
}
