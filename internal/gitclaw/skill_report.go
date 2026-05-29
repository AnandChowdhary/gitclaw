package gitclaw

import (
	"fmt"
	"strings"
)

func IsSkillsReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/skills"
}

func activeSlashCommand(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func activeSlashCommandFields(ev Event, cfg Config) []string {
	for _, line := range strings.Split(activeRequestText(ev), "\n") {
		fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
		if len(fields) > 0 {
			return fields
		}
	}
	return nil
}

func slashCommandFromLine(line, triggerPrefix string) string {
	fields := slashCommandFieldsFromLine(line, triggerPrefix)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func slashCommandFieldsFromLine(line, triggerPrefix string) []string {
	text := strings.TrimSpace(line)
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	prefix := strings.ToLower(triggerPrefix)
	if strings.HasPrefix(lower, prefix) {
		text = strings.TrimSpace(text[len(triggerPrefix):])
	} else if !strings.HasPrefix(text, "/") {
		return nil
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	command := strings.Trim(strings.ToLower(fields[0]), " \t\r\n.,:;!?")
	if !strings.HasPrefix(command, "/") {
		return nil
	}
	fields[0] = command
	return fields
}

func RenderSkillsReport(ev Event, cfg Config, repoContext RepoContext) string {
	if skillName := requestedSkillInfoName(ev, cfg); skillName != "" {
		return renderSkillInfoReport(ev, repoContext, skillName, true)
	}

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
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", skillsMissingRequirements(repoContext.SkillSummaries))
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	writeSkillValidationSummary(&b, validation)
	b.WriteByte('\n')
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

	b.WriteString("\n### Validation\n")
	writeSkillValidationFindings(&b, validation)

	return strings.TrimSpace(b.String())
}

func RenderSkillInfoCLIReport(repoContext RepoContext, name string) string {
	return renderSkillInfoReport(Event{}, repoContext, name, false)
}

func renderSkillInfoReport(ev Event, repoContext RepoContext, name string, includeIssue bool) string {
	name = cleanSkillLookupName(name)
	matches := matchingSkillSummaries(repoContext.SkillSummaries, name)
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Skill Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_skill: `%s`\n", inlineCode(name))
	fmt.Fprintf(&b, "- skill_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- run_mode: `%s`\n\n", "read-only")
	b.WriteString("This report shows metadata for one local skill. Full `SKILL.md` bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Matches\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range matches {
			writeSkillInfoSummary(&b, skill, skillSelectedForTurn(repoContext, skill))
		}
	}

	b.WriteString("\n### Validation For Matches\n")
	writeSkillInfoValidationFindings(&b, validation, matches)

	if len(matches) == 0 {
		b.WriteString("\n### Available Skills\n")
		if len(repoContext.SkillSummaries) == 0 {
			b.WriteString("- none\n")
		} else {
			for _, skill := range repoContext.SkillSummaries {
				fmt.Fprintf(&b, "- `%s` path=`%s`\n", inlineCode(skill.Name), skill.Path)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func requestedSkillInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/skills" || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanSkillLookupName(fields[2])
}

func cleanSkillLookupName(name string) string {
	return strings.Trim(strings.TrimSpace(name), " \t\r\n.,:;!?`\"'")
}

func matchingSkillSummaries(skills []SkillSummary, name string) []SkillSummary {
	name = strings.ToLower(cleanSkillLookupName(name))
	if name == "" {
		return nil
	}
	matches := make([]SkillSummary, 0, 1)
	for _, skill := range skills {
		if strings.EqualFold(skill.Name, name) || strings.EqualFold(skillFolderName(skill.Path), name) {
			matches = append(matches, skill)
		}
	}
	return matches
}

func skillSelectedForTurn(repoContext RepoContext, skill SkillSummary) bool {
	for _, doc := range repoContext.Skills {
		if doc.Path == skill.Path {
			return true
		}
	}
	return false
}

func writeSkillInfoSummary(b *strings.Builder, skill SkillSummary, selected bool) {
	fmt.Fprintf(b, "- skill_name=`%s` path=`%s` folder=`%s` selected_for_this_turn=`%t` always=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d`",
		inlineCode(skill.Name),
		skill.Path,
		skillFolderName(skill.Path),
		selected,
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
	writeSkillInfoList(b, "required_env", skill.RequiredEnv)
	writeSkillInfoList(b, "required_bins", skill.RequiredBins)
	writeSkillInfoList(b, "missing_env", skill.MissingEnv)
	writeSkillInfoList(b, "missing_bins", skill.MissingBins)
}

func writeSkillInfoList(b *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		fmt.Fprintf(b, "  - %s=`none`\n", label)
		return
	}
	fmt.Fprintf(b, "  - %s=`%s`\n", label, inlineList(values))
}

func writeSkillInfoValidationFindings(b *strings.Builder, validation SkillValidationReport, matches []SkillSummary) {
	if len(matches) == 0 {
		b.WriteString("- none\n")
		return
	}
	paths := map[string]bool{}
	for _, match := range matches {
		paths[match.Path] = true
	}
	wrote := false
	for _, finding := range validation.Findings {
		if !paths[finding.Path] {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
	if !wrote {
		b.WriteString("- none\n")
	}
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
