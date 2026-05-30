package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SkillSearchResult struct {
	Skill       SkillSummary
	MatchFields []string
	Score       int
}

func IsSkillsReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/skills" || command == "/bundles"
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
	if isSkillBundlesRiskRequest(ev, cfg) {
		return renderSkillBundlesRiskReport(ev, repoContext, true)
	}
	if isSkillBundlesListRequest(ev, cfg) {
		return renderSkillBundlesReport(ev, repoContext, true)
	}
	if bundleName := requestedSkillBundleInfoName(ev, cfg); bundleName != "" {
		return renderSkillBundleInfoReport(ev, repoContext, bundleName, true)
	}
	if skillName := requestedSkillSelectPlanName(ev, cfg); skillName != "" {
		if skillName == "__missing__" {
			skillName = ""
		}
		return renderSkillSelectPlanReport(ev, repoContext, skillName, activeRequestText(ev), true)
	}
	if operation, target, ok := requestedSkillInstallPlan(ev, cfg); ok {
		return renderSkillInstallPlanReport(ev, repoContext, operation, target, true)
	}
	if isSkillsVerifyRequest(ev, cfg) {
		return renderSkillsVerifyReport(ev, repoContext, true)
	}
	if isSkillsRiskRequest(ev, cfg) {
		return renderSkillsRiskReport(ev, repoContext, true)
	}
	if isSkillsValidateRequest(ev, cfg) {
		return renderSkillsValidationReport(ev, repoContext, true)
	}
	if skillName := requestedSkillInfoName(ev, cfg); skillName != "" {
		return renderSkillInfoReport(ev, repoContext, skillName, true)
	}
	if query := requestedSkillSearchQuery(ev, cfg); query != "" {
		return renderSkillSearchReport(ev, repoContext, query, true)
	}
	return renderSkillsListReport(ev, repoContext, true)
}

func RenderSkillsCLIReport(repoContext RepoContext) string {
	return renderSkillsListReport(Event{}, repoContext, false)
}

func renderSkillsListReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Skills Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", enabledSkillCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", disabledByConfigCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", blockedByAllowlistCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- skills_with_frontmatter: `%d`\n", skillsWithFrontmatter(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skills_with_description: `%d`\n", skillsWithDescription(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skills_with_requirements: `%d`\n", skillsWithRequirements(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skills_missing_requirements: `%d`\n", skillsMissingRequirements(repoContext.SkillSummaries))
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	risk := BuildSkillRiskReport(repoContext.SkillSummaries)
	writeSkillValidationSummary(&b, validation)
	writeSkillRiskSummary(&b, risk)
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

func RenderSkillSearchCLIReport(repoContext RepoContext, query string) string {
	return renderSkillSearchReport(Event{}, repoContext, query, false)
}

func RenderSkillBundlesCLIReport(repoContext RepoContext) string {
	return renderSkillBundlesReport(Event{}, repoContext, false)
}

func RenderSkillBundlesRiskCLIReport(repoContext RepoContext) string {
	return renderSkillBundlesRiskReport(Event{}, repoContext, false)
}

func RenderSkillBundleInfoCLIReport(repoContext RepoContext, name string) string {
	return renderSkillBundleInfoReport(Event{}, repoContext, name, false)
}

func RenderSkillInstallPlanCLIReport(repoContext RepoContext, operation, target string) string {
	return renderSkillInstallPlanReport(Event{}, repoContext, operation, target, false)
}

func renderSkillBundlesReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Skill Bundles Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- available_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- selected_bundles: `%d`\n", selectedSkillBundleCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- bundle_skill_refs: `%d`\n", bundleSkillRefCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- resolved_bundle_skills: `%d`\n", resolvedBundleSkillCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- missing_bundle_skills: `%d`\n", missingBundleSkillCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- bundles_with_instruction: `%d`\n", bundlesWithInstructionCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Skill bundles are repo-local YAML task profiles. They group existing skills under a slash command without installing skills, mutating the system prompt, or dumping bundle instructions.\n\n")

	b.WriteString("### Bundles\n")
	if len(repoContext.SkillBundles) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, bundle := range repoContext.SkillBundles {
			writeSkillBundleSummary(&b, bundle)
		}
	}
	return strings.TrimSpace(b.String())
}

func renderSkillBundleInfoReport(ev Event, repoContext RepoContext, name string, includeIssue bool) string {
	name = cleanSkillLookupName(name)
	matches := matchingSkillBundleSummaries(repoContext.SkillBundles, name)
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Skill Bundle Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_bundle: `%s`\n", inlineCode(name))
	fmt.Fprintf(&b, "- skill_bundle_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- available_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- matched_bundles: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report shows one repo-local skill bundle by metadata only. Bundle YAML bodies, bundle instructions, skill bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Matches\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, bundle := range matches {
			writeSkillBundleSummary(&b, bundle)
		}
	}
	if len(matches) == 0 && len(repoContext.SkillBundles) > 0 {
		b.WriteString("\n### Available Bundles\n")
		for _, bundle := range repoContext.SkillBundles {
			fmt.Fprintf(&b, "- `%s` path=`%s`\n", inlineCode(bundle.Name), bundle.Path)
		}
	}
	return strings.TrimSpace(b.String())
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
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", enabledSkillCount(repoContext.SkillSummaries))
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

func renderSkillSearchReport(ev Event, repoContext RepoContext, query string, includeIssue bool) string {
	query = cleanSkillSearchQuery(query)
	results := searchSkillSummaries(repoContext.SkillSummaries, query)
	status := "ok"
	if query == "" {
		status = "no_query"
	} else if len(results) == 0 {
		status = "no_matches"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Skills Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- skill_search_status: `%s`\n", status)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", shortDocumentHash(query))
	fmt.Fprintf(&b, "- query_terms: `%d`\n", len(skillSearchTerms(query)))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", enabledSkillCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", len(results))
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", false)
	b.WriteString("This report searches only skill metadata: name, folder, path, and frontmatter description. Full `SKILL.md` bodies, issue bodies, comments, prompts, and raw search queries are not included.\n\n")

	b.WriteString("### Matches\n")
	if len(results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range results {
			writeSkillSearchResult(&b, result, skillSelectedForTurn(repoContext, result.Skill))
		}
	}

	if len(results) == 0 && len(repoContext.SkillSummaries) > 0 {
		b.WriteString("\n### Available Skills\n")
		for _, skill := range repoContext.SkillSummaries {
			fmt.Fprintf(&b, "- `%s` path=`%s`\n", inlineCode(skill.Name), skill.Path)
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

func requestedSkillBundleInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return ""
	}
	if fields[0] == "/bundles" {
		if len(fields) >= 3 && (strings.EqualFold(fields[1], "info") || strings.EqualFold(fields[1], "show")) {
			return cleanSkillLookupName(fields[2])
		}
		return ""
	}
	if fields[0] == "/skills" && len(fields) >= 3 && (strings.EqualFold(fields[1], "bundle") || strings.EqualFold(fields[1], "bundle-info")) {
		return cleanSkillLookupName(fields[2])
	}
	return ""
}

func requestedSkillSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/skills" || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanSkillSearchQuery(strings.Join(fields[2:], " "))
}

func requestedSkillInstallPlan(ev Event, cfg Config) (operation string, target string, ok bool) {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/skills" {
		return "", "", false
	}
	switch strings.ToLower(fields[1]) {
	case "install-plan", "plan":
		operation = "install-plan"
	case "upgrade-plan":
		operation = "upgrade-plan"
	default:
		return "", "", false
	}
	if len(fields) >= 3 {
		target = cleanSkillInstallTarget(fields[2])
	}
	return operation, target, true
}

func isSkillsValidateRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/skills" && (strings.EqualFold(fields[1], "validate") || strings.EqualFold(fields[1], "check"))
}

func isSkillsVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/skills" && strings.EqualFold(fields[1], "verify")
}

func isSkillsRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/skills" && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}

func isSkillBundlesListRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "/bundles" {
		return len(fields) == 1 || strings.EqualFold(fields[1], "list")
	}
	return (len(fields) == 2 && fields[0] == "/skills" && strings.EqualFold(fields[1], "bundles")) ||
		(len(fields) == 2 && fields[0] == "/skills" && strings.EqualFold(fields[1], "bundle-list"))
}

func isSkillBundlesRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "/bundles" {
		return len(fields) >= 2 && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
	}
	if fields[0] != "/skills" || len(fields) < 2 {
		return false
	}
	if strings.EqualFold(fields[1], "bundle-risk") || strings.EqualFold(fields[1], "bundles-risk") {
		return true
	}
	return len(fields) >= 3 &&
		(strings.EqualFold(fields[1], "bundles") || strings.EqualFold(fields[1], "bundle-list")) &&
		(strings.EqualFold(fields[2], "risk") || strings.EqualFold(fields[2], "risk-audit"))
}

func cleanSkillLookupName(name string) string {
	return strings.Trim(strings.TrimSpace(name), " \t\r\n.,:;!?`\"'")
}

func cleanSkillSearchQuery(query string) string {
	return strings.Trim(strings.TrimSpace(query), " \t\r\n.,:;!?`\"'")
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

func matchingSkillBundleSummaries(bundles []SkillBundleSummary, name string) []SkillBundleSummary {
	name = normalizeSkillBundleName(cleanSkillLookupName(name))
	if name == "" {
		return nil
	}
	matches := make([]SkillBundleSummary, 0, 1)
	for _, bundle := range bundles {
		if strings.EqualFold(bundle.Name, name) || strings.EqualFold(skillBundleNameFromPath(bundle.Path), name) {
			matches = append(matches, bundle)
		}
	}
	return matches
}

func searchSkillSummaries(skills []SkillSummary, query string) []SkillSearchResult {
	query = strings.ToLower(cleanSkillSearchQuery(query))
	if query == "" {
		return nil
	}
	terms := skillSearchTerms(query)
	var results []SkillSearchResult
	for _, skill := range skills {
		score, fields := skillSearchScore(skill, query, terms)
		if score == 0 {
			continue
		}
		results = append(results, SkillSearchResult{
			Skill:       skill,
			MatchFields: fields,
			Score:       score,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Skill.Path < results[j].Skill.Path
	})
	return results
}

func skillSearchScore(skill SkillSummary, query string, terms []string) (int, []string) {
	fields := map[string]string{
		"name":        skill.Name,
		"folder":      skillFolderName(skill.Path),
		"path":        skill.Path,
		"description": skill.Description,
	}
	weights := map[string]int{
		"name":        80,
		"folder":      70,
		"path":        30,
		"description": 20,
	}
	score := 0
	matchedFields := map[string]bool{}
	for field, value := range fields {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if value == query {
			score += weights[field] * 2
			matchedFields[field] = true
			continue
		}
		if strings.Contains(value, query) {
			score += weights[field]
			matchedFields[field] = true
		}
		for _, term := range terms {
			if strings.Contains(value, term) {
				score += weights[field] / 2
				matchedFields[field] = true
			}
		}
	}
	if score == 0 {
		return 0, nil
	}
	var out []string
	for field := range matchedFields {
		out = append(out, field)
	}
	sort.Strings(out)
	return score, out
}

func skillSearchTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(cleanSkillSearchQuery(query)), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	})
	var terms []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 2 || isStopWord(field) {
			continue
		}
		if !containsStringFold(terms, field) {
			terms = append(terms, field)
		}
	}
	return terms
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
	fmt.Fprintf(b, "- skill_name=`%s` path=`%s` folder=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` selected_for_this_turn=`%t` always=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d`",
		inlineCode(skill.Name),
		skill.Path,
		skillFolderName(skill.Path),
		skillIsEnabled(skill),
		skill.DisabledByConfig,
		skill.BlockedByAllowlist,
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

func writeSkillSearchResult(b *strings.Builder, result SkillSearchResult, selected bool) {
	skill := result.Skill
	fmt.Fprintf(b, "- skill_name=`%s` path=`%s` folder=`%s` match_fields=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` selected_for_this_turn=`%t` always=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d`\n",
		inlineCode(skill.Name),
		skill.Path,
		skillFolderName(skill.Path),
		inlineList(result.MatchFields),
		skillIsEnabled(skill),
		skill.DisabledByConfig,
		skill.BlockedByAllowlist,
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
	fmt.Fprintf(b, "- name=`%s` path=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` always=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` requires_env=`%d` requires_bins=`%d` missing_env=`%d` missing_bins=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`",
		inlineCode(skill.Name),
		skill.Path,
		skillIsEnabled(skill),
		skill.DisabledByConfig,
		skill.BlockedByAllowlist,
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
		len(skill.RiskFindings),
		skillRiskMaxSeverity(skill.RiskFindings),
		inlineListOrNone(skillRiskCodes(skill.RiskFindings)),
	)
	if skill.Description != "" {
		fmt.Fprintf(b, " description=`%s`", inlineCode(skill.Description))
	}
	b.WriteByte('\n')
}

func writeSkillBundleSummary(b *strings.Builder, bundle SkillBundleSummary) {
	fmt.Fprintf(b, "- bundle_name=`%s` path=`%s` skills=`%s` resolved_skills=`%s` missing_skills=`%s` selected_for_this_turn=`%t` instruction=`%t` bytes=`%d` lines=`%d` sha256_12=`%s`",
		inlineCode(bundle.Name),
		bundle.Path,
		inlineList(bundle.Skills),
		inlineList(bundle.ResolvedSkills),
		inlineList(bundle.MissingSkills),
		bundle.Selected,
		bundle.InstructionPresent,
		bundle.Bytes,
		bundle.Lines,
		bundle.SHA,
	)
	if bundle.Description != "" {
		fmt.Fprintf(b, " description=`%s`", inlineCode(bundle.Description))
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

func enabledSkillCount(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if skillIsEnabled(skill) {
			count++
		}
	}
	return count
}

func skillIsEnabled(skill SkillSummary) bool {
	return skill.Enabled || (!skill.DisabledByConfig && !skill.BlockedByAllowlist)
}

func disabledByConfigCount(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if skill.DisabledByConfig {
			count++
		}
	}
	return count
}

func blockedByAllowlistCount(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if skill.BlockedByAllowlist {
			count++
		}
	}
	return count
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
		if skillIsEnabled(skill) && (len(skill.MissingEnv) > 0 || len(skill.MissingBins) > 0) {
			count++
		}
	}
	return count
}

func selectedSkillBundleCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		if bundle.Selected {
			count++
		}
	}
	return count
}

func bundleSkillRefCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		count += len(bundle.Skills)
	}
	return count
}

func resolvedBundleSkillCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		count += len(bundle.ResolvedSkills)
	}
	return count
}

func missingBundleSkillCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		count += len(bundle.MissingSkills)
	}
	return count
}

func bundlesWithInstructionCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		if bundle.InstructionPresent {
			count++
		}
	}
	return count
}
