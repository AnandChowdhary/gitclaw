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
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- skills_with_frontmatter: `%d`\n", skillsWithFrontmatter(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skills_with_description: `%d`\n", skillsWithDescription(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skills_with_requirements: `%d`\n", skillsWithRequirements(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n\n", skillsMissingRequirements(repoContext.SkillSummaries))
	b.WriteString("GitClaw uses progressive disclosure: this report lists available skill metadata, while full `SKILL.md` bodies are loaded only when selected or marked always-on.\n\n")
	b.WriteString("Skill bodies are not included; hashes and requirement counts make local skills reviewable like code before they influence a model turn.\n\n")

	b.WriteString("### Available Skills\n")
	if len(repoContext.SkillSummaries) == 0 {
		index := skillIndexOutput(repoContext)
		if index != "" {
			b.WriteString(index)
			b.WriteByte('\n')
		} else {
			b.WriteString("- none\n")
		}
	} else {
		for _, skill := range repoContext.SkillSummaries {
			writeSkillSummary(&b, skill)
		}
	}

	b.WriteString("\n### Selected For This Turn\n")
	writeSelectedSkillList(&b, repoContext.Skills)

	return strings.TrimSpace(b.String())
}

func writeSkillSummary(b *strings.Builder, skill SkillSummary) {
	fmt.Fprintf(b, "- name=`%s` path=`%s` always=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d`",
		inlineCode(skill.Name),
		skill.Path,
		skill.Always,
		skill.FrontmatterPresent,
		strings.TrimSpace(skill.Description) != "",
		skill.Bytes,
		skill.Lines,
		skill.SHA,
		len(skill.RequiredEnv),
		len(skill.RequiredBins),
		len(skill.MissingEnv),
		len(skill.MissingBins),
	)
	if skill.Description != "" {
		fmt.Fprintf(b, " description=`%s`", inlineCode(skill.Description))
	}
	b.WriteByte('\n')
}

func writeSelectedSkillList(b *strings.Builder, docs []ContextDocument) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
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
	if len(repoContext.SkillSummaries) > 0 {
		return len(repoContext.SkillSummaries)
	}
	index := skillIndexOutput(repoContext)
	if index == "" {
		return 0
	}
	return lineCount(index)
}

func skillsWithFrontmatter(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if skill.FrontmatterPresent {
			count++
		}
	}
	return count
}

func skillsWithDescription(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if strings.TrimSpace(skill.Description) != "" {
			count++
		}
	}
	return count
}

func skillsWithRequirements(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if len(skill.RequiredEnv) > 0 || len(skill.RequiredBins) > 0 {
			count++
		}
	}
	return count
}

func skillsMissingRequirements(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if len(skill.MissingEnv) > 0 || len(skill.MissingBins) > 0 {
			count++
		}
	}
	return count
}
