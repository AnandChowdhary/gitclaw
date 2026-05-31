package gitclaw

import (
	"fmt"
	"strings"
)

type skillProposalPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func renderSkillProposalPlanReport(ev Event, repoContext RepoContext, requestedAction, target string, includeIssue bool) string {
	requestedAction = normalizeSkillProposalAction(requestedAction)
	targetInfo := classifySkillInstallTarget(target)
	matches := matchingInstallPlanSkillSummaries(repoContext.SkillSummaries, targetInfo)
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	plannedAction := plannedSkillProposalAction(requestedAction, len(matches))
	proposalPath := skillProposalPlanPath(targetInfo.Candidate)
	destinationPath := skillInstallDestinationPath(targetInfo.Candidate)
	sourceText := activeRequestText(ev)
	findings := skillProposalPlanFindings(requestedAction, targetInfo, matches, validation)
	status := skillProposalPlanStatus(findings)

	var b strings.Builder
	b.WriteString("## GitClaw Skill Proposal Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- proposal_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- operation: `%s`\n", "proposal-plan")
	fmt.Fprintf(&b, "- requested_action: `%s`\n", requestedAction)
	fmt.Fprintf(&b, "- planned_proposal_action: `%s`\n", plannedAction)
	fmt.Fprintf(&b, "- target_type: `%s`\n", targetInfo.Type)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", targetInfo.Hash)
	fmt.Fprintf(&b, "- target_terms: `%d`\n", targetInfo.Terms)
	fmt.Fprintf(&b, "- safe_name_candidate: `%s`\n", inlineCode(targetInfo.Candidate))
	fmt.Fprintf(&b, "- proposal_path: `%s`\n", proposalPath)
	fmt.Fprintf(&b, "- destination_path: `%s`\n", destinationPath)
	fmt.Fprintf(&b, "- existing_skill_matches: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- proposal_store: `%s`\n", "git-reviewed-proposal-file")
	fmt.Fprintf(&b, "- proposal_support_dirs: `%s`\n", "assets,examples,references,scripts,templates")
	fmt.Fprintf(&b, "- review_pr_required: `%t`\n", true)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- proposal_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- active_skill_write_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- autonomous_skill_creation: `%t`\n", false)
	fmt.Fprintf(&b, "- autonomous_skill_improvement: `%t`\n", false)
	fmt.Fprintf(&b, "- manual_review_required: `%t`\n", true)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	fmt.Fprintf(&b, "- proposal_source_sha256_12: `%s`\n", shortDocumentHash(sourceText))
	fmt.Fprintf(&b, "- proposal_source_bytes: `%d`\n", len(sourceText))
	fmt.Fprintf(&b, "- proposal_source_lines: `%d`\n", lineCount(sourceText))
	fmt.Fprintf(&b, "- raw_target_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_proposal_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_existing_skill_body_included: `%t`\n", false)
	writeSkillValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is a non-mutating proposal planner for reusable skill changes. It records the reviewed repo paths and request hashes without fetching remote sources, running installers, writing proposal files, updating active skills, or dumping issue, proposal, or skill bodies.\n\n")

	b.WriteString("### Existing Skill Matches\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range matches {
			writeSkillInfoSummary(&b, skill, skillSelectedForTurn(repoContext, skill))
		}
	}

	b.WriteString("\n### Proposal Review Steps\n")
	if proposalPath == "" {
		b.WriteString("1. Provide a safe lower hyphen-case skill name.\n")
	} else {
		fmt.Fprintf(&b, "1. Draft the proposal at `%s` on a reviewed branch, using proposal-only frontmatter.\n", proposalPath)
		b.WriteString("2. Keep support files under `assets/`, `examples/`, `references/`, `scripts/`, or `templates/`, and review scripts as code before use.\n")
		fmt.Fprintf(&b, "3. If approved, manually convert the proposal into `%s`; GitClaw does not auto-apply proposals.\n", destinationPath)
		b.WriteString("4. Run `gitclaw skills validate`, `gitclaw skills verify`, `gitclaw skills risk`, and a live GitHub Models conversation E2E before merging.\n")
	}

	b.WriteString("\n### Findings\n")
	writeSkillProposalPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func RenderSkillProposalPlanCLIReport(repoContext RepoContext, requestedAction, target string) string {
	return renderSkillProposalPlanReport(Event{ActiveText: "skills proposal-plan " + target}, repoContext, requestedAction, target, false)
}

func skillProposalPlanPath(candidate string) string {
	if candidate == "" {
		return ""
	}
	return ".gitclaw/skill-proposals/" + candidate + "/PROPOSAL.md"
}

func normalizeSkillProposalAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "propose-create":
		return "propose-create"
	case "propose-update":
		return "propose-update"
	default:
		return "auto"
	}
}

func plannedSkillProposalAction(requestedAction string, matches int) string {
	if requestedAction == "propose-create" || requestedAction == "propose-update" {
		return requestedAction
	}
	if matches > 0 {
		return "propose-update"
	}
	return "propose-create"
}

func skillProposalPlanFindings(requestedAction string, target skillInstallPlanTarget, matches []SkillSummary, validation SkillValidationReport) []skillProposalPlanFinding {
	var findings []skillProposalPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, skillProposalPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "manual_review_required", "skill proposals must be reviewed as repository code before they affect model context")
	add("info", "proposal_store_git_backed", "proposal paths are repo-local review artifacts, not hidden agent state")
	add("info", "autonomous_apply_disabled", "GitClaw does not autonomously create, update, apply, or improve skills")
	add("info", "repository_mutation_disabled", "proposal planning does not create, update, delete, commit, or push files")
	add("info", "live_llm_e2e_required", "accepted skill changes must pass a live GitHub Models conversation E2E")
	switch target.Type {
	case "empty":
		add("error", "target_empty", "provide a reusable skill proposal name")
	case "unsafe-path":
		add("error", "unsafe_target_path", "absolute paths, parent traversal, backslashes, and NUL bytes are rejected")
	case "http-url":
		add("error", "insecure_http_url", "HTTP proposal sources are blocked; use HTTPS and manual review")
	case "unsupported-url":
		add("error", "unsupported_url_scheme", "only HTTPS URLs and GitHub SSH shorthand can be classified")
	}
	if target.Remote {
		add("warning", "network_fetch_disabled", "remote proposal targets are classified only; the Actions job does not fetch skill code")
	}
	if target.Candidate == "" {
		add("error", "safe_name_candidate_empty", "could not derive a safe repo-local proposal folder name")
	} else if !skillNamePattern.MatchString(target.Candidate) {
		add("error", "safe_name_candidate_invalid", "safe repo-local skill names must match ^[a-z0-9][a-z0-9-]*$")
	}
	if len(matches) > 0 {
		add("warning", "existing_skill_update_review", "the proposal targets an existing repo-local skill and should be reviewed as an update")
	} else {
		add("info", "new_skill_proposal", "no existing repo-local skill matches the proposal candidate")
	}
	if requestedAction == "propose-update" && len(matches) == 0 {
		add("error", "proposal_update_target_missing", "explicit update proposals require an existing repo-local skill match")
	}
	if requestedAction == "propose-create" && len(matches) > 0 {
		add("warning", "proposal_create_existing_skill", "explicit create proposals should not overwrite an existing skill without update review")
	}
	if validation.Errors > 0 {
		add("error", "skill_validation_errors_present", "fix existing skill validation errors before adding proposal work")
	} else if validation.Warnings > 0 {
		add("warning", "skill_validation_warnings_present", "review existing skill validation warnings before adding proposal work")
	}
	return findings
}

func skillProposalPlanStatus(findings []skillProposalPlanFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	return "needs_review"
}

func writeSkillProposalPlanFindings(b *strings.Builder, findings []skillProposalPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
