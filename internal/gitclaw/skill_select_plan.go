package gitclaw

import (
	"fmt"
	"strings"
)

type skillSelectPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderSkillSelectPlanCLIReport(repoContext RepoContext, name string) string {
	return renderSkillSelectPlanReport(Event{}, repoContext, name, "skills select-plan "+name, false)
}

func renderSkillSelectPlanReport(ev Event, repoContext RepoContext, name string, requestText string, includeIssue bool) string {
	requested := cleanSkillLookupName(name)
	matches := matchingSkillSummaries(repoContext.SkillSummaries, requested)
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	findings := skillSelectPlanFindings(requested, matches, repoContext, validation)
	status := skillSelectPlanStatus(findings)

	selected := false
	enabled := false
	disabled := false
	blocked := false
	always := false
	if len(matches) == 1 {
		selected = skillSelectedForTurn(repoContext, matches[0])
		enabled = skillIsEnabled(matches[0])
		disabled = matches[0].DisabledByConfig
		blocked = matches[0].BlockedByAllowlist
		always = matches[0].Always
	}

	var b strings.Builder
	b.WriteString("## GitClaw Skill Select Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- skill_select_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- requested_skill_sha256_12: `%s`\n", shortDocumentHash(requested))
	fmt.Fprintf(&b, "- request_text_sha256_12: `%s`\n", shortDocumentHash(requestText))
	fmt.Fprintf(&b, "- request_terms: `%d`\n", len(skillSearchTerms(requestText)))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", enabledSkillCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- matched_skills: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- selected_bundles: `%d`\n", selectedSkillBundleSummaryCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- selected_for_this_turn: `%t`\n", selected)
	fmt.Fprintf(&b, "- skill_enabled: `%t`\n", enabled)
	fmt.Fprintf(&b, "- disabled_by_config: `%t`\n", disabled)
	fmt.Fprintf(&b, "- blocked_by_allowlist: `%t`\n", blocked)
	fmt.Fprintf(&b, "- always_on: `%t`\n", always)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_skill_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_request_text_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	writeSkillValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report explains whether one repo-local skill is selected for the current turn. It uses only metadata, hashes, and gate state; full skill bodies, issue bodies, comments, prompts, and raw request text are not included.\n\n")

	b.WriteString("### Skill Match\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range matches {
			writeSkillInfoSummary(&b, skill, skillSelectedForTurn(repoContext, skill))
		}
	}

	b.WriteString("\n### Selection Reasons\n")
	writeSkillSelectionReasons(&b, repoContext, matches, requestText)

	b.WriteString("\n### Review Steps\n")
	if len(matches) != 1 {
		b.WriteString("1. Choose one known repo-local skill from `/skills` or `gitclaw skills list`.\n")
	} else {
		b.WriteString("1. Confirm the skill is enabled by config and not blocked by the allowlist.\n")
		b.WriteString("2. Confirm selection comes from `always`, a selected bundle, or bounded issue/comment text.\n")
		b.WriteString("3. Inspect the content hash and validation findings before changing skill behavior.\n")
		b.WriteString("4. Use a live GitHub Models conversation E2E when changing skill selection or skill contents, not only deterministic reports.\n")
	}

	b.WriteString("\n### Findings\n")
	writeSkillSelectPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedSkillSelectPlanName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/skills" {
		return ""
	}
	if !strings.EqualFold(fields[1], "select-plan") && !strings.EqualFold(fields[1], "selection-plan") {
		return ""
	}
	if len(fields) < 3 {
		return "__missing__"
	}
	return cleanSkillLookupName(fields[2])
}

func skillSelectPlanFindings(requested string, matches []SkillSummary, repoContext RepoContext, validation SkillValidationReport) []skillSelectPlanFinding {
	var findings []skillSelectPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, skillSelectPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "progressive_disclosure", "GitClaw loads skill bodies only when selected by always-on metadata, current turn text, or selected skill bundles")
	add("info", "repository_mutation_disabled", "skill selection planning does not create, update, delete, commit, push, or apply files")
	if requested == "" || requested == "__missing__" {
		add("error", "skill_missing", "provide one repo-local skill name")
	}
	if requested != "" && len(matches) == 0 {
		add("error", "skill_not_found", "requested skill does not match a known repo-local skill")
	}
	if len(matches) > 1 {
		add("error", "skill_ambiguous", "requested skill matched multiple repo-local skills")
	}
	if len(matches) == 1 {
		skill := matches[0]
		switch {
		case skill.DisabledByConfig:
			add("error", "skill_disabled_by_config", "matched skill is disabled by repo config")
		case skill.BlockedByAllowlist:
			add("error", "skill_blocked_by_allowlist", "matched skill is not in the repo-reviewed skill allowlist")
		case skillSelectedForTurn(repoContext, skill):
			add("info", "skill_selected_for_turn", "matched skill body is selected for this turn")
		default:
			add("warning", "skill_not_selected_for_turn", "matched skill is available but not selected for this turn")
		}
	}
	if validation.Errors > 0 {
		add("error", "skill_validation_errors_present", "fix skill validation errors before relying on skill selection")
	} else if validation.Warnings > 0 {
		add("warning", "skill_validation_warnings_present", "review skill validation warnings before relying on skill selection")
	}
	return findings
}

func skillSelectPlanStatus(findings []skillSelectPlanFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "needs_review"
		}
	}
	return "ok"
}

func writeSkillSelectionReasons(b *strings.Builder, repoContext RepoContext, matches []SkillSummary, requestText string) {
	if len(matches) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, skill := range matches {
		reasons := skillSelectionReasonCodes(repoContext, skill, requestText)
		if len(reasons) == 0 {
			reasons = []string{"not_selected"}
		}
		fmt.Fprintf(b, "- skill_name=`%s` selected_for_this_turn=`%t` reasons=`%s`\n",
			inlineCode(skill.Name),
			skillSelectedForTurn(repoContext, skill),
			inlineList(reasons),
		)
	}
}

func skillSelectionReasonCodes(repoContext RepoContext, skill SkillSummary, requestText string) []string {
	var reasons []string
	if skill.Always {
		reasons = append(reasons, "always")
	}
	if skillSelectedBySelectedBundleSummary(skill, repoContext.SkillBundles) {
		reasons = append(reasons, "selected_bundle")
	}
	if skillSummaryMatchesRequest(skill, requestText) {
		reasons = append(reasons, "request_metadata_match")
	}
	if !skillIsEnabled(skill) {
		if skill.DisabledByConfig {
			reasons = append(reasons, "disabled_by_config")
		}
		if skill.BlockedByAllowlist {
			reasons = append(reasons, "blocked_by_allowlist")
		}
	}
	return uniqueSortedStrings(reasons)
}

func skillSummaryMatchesRequest(skill SkillSummary, requestText string) bool {
	query := strings.ToLower(requestText)
	if query == "" {
		return false
	}
	for _, candidate := range []string{skill.Name, skillFolderName(skill.Path), skill.Path} {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" && strings.Contains(query, candidate) {
			return true
		}
	}
	for _, word := range searchableWords(skill.Description) {
		if strings.Contains(query, word) {
			return true
		}
	}
	return false
}

func skillSelectedBySelectedBundleSummary(skill SkillSummary, bundles []SkillBundleSummary) bool {
	name := strings.ToLower(strings.TrimSpace(skill.Name))
	folder := strings.ToLower(skillFolderName(skill.Path))
	for _, bundle := range bundles {
		if !bundle.Selected {
			continue
		}
		for _, ref := range bundle.ResolvedSkills {
			ref = strings.ToLower(strings.TrimSpace(ref))
			if ref == name || ref == folder {
				return true
			}
		}
	}
	return false
}

func selectedSkillBundleSummaryCount(bundles []SkillBundleSummary) int {
	count := 0
	for _, bundle := range bundles {
		if bundle.Selected {
			count++
		}
	}
	return count
}

func writeSkillSelectPlanFindings(b *strings.Builder, findings []skillSelectPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
