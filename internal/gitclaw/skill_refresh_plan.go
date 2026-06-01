package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type skillRefreshPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderSkillRefreshPlanCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillRefreshPlanReport(Event{}, cfg, repoContext, false)
}

func renderSkillRefreshPlanReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	findings := skillRefreshPlanFindings(validation)
	status := skillRefreshPlanStatus(findings)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Refresh Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- skill_refresh_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- refresh_strategy: `%s`\n", "github-actions-per-turn-discovery")
	fmt.Fprintf(&b, "- refresh_trigger: `%s`\n", "next-issue-comment-or-workflow-dispatch-run")
	fmt.Fprintf(&b, "- current_snapshot_scope: `%s`\n", "current-actions-checkout")
	fmt.Fprintf(&b, "- resident_skill_watcher: `%t`\n", false)
	fmt.Fprintf(&b, "- mid_run_hot_reload_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- session_snapshot_reused_across_runs: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_snapshot_persisted: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_node_refresh_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- remote_registry_polling_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- workflow_dispatch_refresh_supported: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- skill_update_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompt_included: `%t`\n", false)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", enabledSkillCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", disabledByConfigCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", blockedByAllowlistCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- selected_bundles: `%d`\n", selectedSkillBundleSummaryCount(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- skill_hashes: `%d`\n", skillRefreshHashCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skill_index_sha256_12: `%s`\n", skillRefreshIndexHash(repoContext.SkillSummaries, repoContext.SkillBundles))
	fmt.Fprintf(&b, "- config_allowed_skills: `%d`\n", len(cfg.AllowedSkills))
	fmt.Fprintf(&b, "- config_disabled_skills: `%d`\n", len(cfg.DisabledSkills))
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_refresh_change: `%t`\n", true)
	writeSkillValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report explains when skill changes become prompt-visible in GitClaw's serverless runtime. It reports current checkout metadata, validation state, and refresh boundaries only; skill bodies, issue bodies, comments, prompts, registry responses, credentials, and secret values are not included.\n\n")

	b.WriteString("### Refresh Boundary\n")
	fmt.Fprintf(&b, "- kind=`runtime` refresh_strategy=`github-actions-per-turn-discovery` current_snapshot_scope=`current-actions-checkout` resident_skill_watcher=`false` mid_run_hot_reload_supported=`false` session_snapshot_reused_across_runs=`false` workflow_dispatch_refresh_supported=`true`\n")
	fmt.Fprintf(&b, "- kind=`source` repo_review_required=`true` default_branch_checkout_required=`true` remote_registry_polling_allowed=`false` skill_install_allowed=`false` skill_update_allowed=`false` repository_mutation_allowed=`false`\n")
	fmt.Fprintf(&b, "- kind=`prompt` progressive_disclosure=`true` selected_skills=`%d` raw_skill_bodies_included=`false` raw_prompt_included=`false`\n", len(repoContext.Skills))

	b.WriteString("\n### Current Skill Snapshot\n")
	if len(repoContext.SkillSummaries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range repoContext.SkillSummaries {
			fmt.Fprintf(&b, "- name=`%s` path=`%s` enabled=`%t` selected_for_this_turn=`%t` always_on=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t` sha256_12=`%s`\n",
				skill.Name,
				skill.Path,
				skillIsEnabled(skill),
				skillSelectedForTurn(repoContext, skill),
				skill.Always,
				skill.DisabledByConfig,
				skill.BlockedByAllowlist,
				skill.SHA,
			)
		}
	}

	b.WriteString("\n### Refresh Steps\n")
	b.WriteString("1. Make skill or skill-config changes as reviewed repository edits.\n")
	b.WriteString("2. Run `gitclaw skills validate`, `gitclaw skills verify`, and `gitclaw skills risk` before relying on the changed skill.\n")
	b.WriteString("3. Merge or push the reviewed change to the branch used by the GitHub Actions checkout.\n")
	b.WriteString("4. Start a new issue/comment turn or dispatch the workflow for the target issue; that run rebuilds the skill index from the checkout.\n")
	b.WriteString("5. Run a live GitHub Models conversation E2E after skill refresh behavior changes, not only deterministic reports.\n")

	b.WriteString("\n### Findings\n")
	writeSkillRefreshPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func isSkillsRefreshPlanRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/skills" {
		return false
	}
	return strings.EqualFold(fields[1], "refresh-plan") ||
		strings.EqualFold(fields[1], "refresh") ||
		strings.EqualFold(fields[1], "reload-plan")
}

func skillRefreshPlanFindings(validation SkillValidationReport) []skillRefreshPlanFinding {
	findings := []skillRefreshPlanFinding{
		{Severity: "info", Code: "per_turn_discovery", Detail: "each GitHub Actions turn rebuilds the skill index from the checked-out repository"},
		{Severity: "info", Code: "resident_watcher_disabled", Detail: "GitClaw does not keep a long-running skill watcher or mid-run hot reload loop"},
		{Severity: "info", Code: "progressive_disclosure", Detail: "skill bodies are prompt-visible only when selected for the turn"},
		{Severity: "info", Code: "repository_mutation_disabled", Detail: "refresh planning does not install, update, delete, commit, push, or apply skill files"},
		{Severity: "warning", Code: "live_llm_e2e_required", Detail: "skill refresh behavior changes must be followed by a real GitHub Models conversation E2E"},
	}
	if validation.Errors > 0 {
		findings = append(findings, skillRefreshPlanFinding{Severity: "error", Code: "skill_validation_errors_present", Detail: "fix skill validation errors before relying on refresh behavior"})
	} else if validation.Warnings > 0 {
		findings = append(findings, skillRefreshPlanFinding{Severity: "warning", Code: "skill_validation_warnings_present", Detail: "review skill validation warnings before relying on refresh behavior"})
	}
	return findings
}

func skillRefreshPlanStatus(findings []skillRefreshPlanFinding) string {
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

func writeSkillRefreshPlanFindings(b *strings.Builder, findings []skillRefreshPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}

func skillRefreshHashCount(skills []SkillSummary) int {
	count := 0
	for _, skill := range skills {
		if strings.TrimSpace(skill.SHA) != "" {
			count++
		}
	}
	return count
}

func skillRefreshIndexHash(skills []SkillSummary, bundles []SkillBundleSummary) string {
	parts := make([]string, 0, len(skills)+len(bundles))
	for _, skill := range skills {
		parts = append(parts, "skill:"+skill.Path+":"+skill.SHA)
	}
	for _, bundle := range bundles {
		parts = append(parts, "bundle:"+bundle.Path+":"+bundle.SHA)
	}
	sort.Strings(parts)
	return shortDocumentHash(strings.Join(parts, "\n"))
}
