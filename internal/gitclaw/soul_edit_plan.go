package gitclaw

import (
	"fmt"
	"strings"
)

type soulEditPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func RenderSoulEditPlanCLIReport(cfg Config, repoContext RepoContext, target string) string {
	return renderSoulEditPlanReport(Event{}, cfg, repoContext, target, false)
}

func renderSoulEditPlanReport(ev Event, cfg Config, repoContext RepoContext, target string, includeIssue bool) string {
	target = cleanSoulInfoPath(target)
	normalized := normalizeSoulInfoPath(target, cfg, repoContext)
	allowed := normalized != "" && soulInfoAllowedPath(normalized)
	displayPath := ""
	var match soulInfoMatchResult
	matched := 0
	if allowed {
		displayPath = normalized
		if result, ok := soulInfoMatch(cfg.Workdir, repoContext, normalized); ok {
			match = result
			matched = 1
		}
	}
	validation := ValidateSoulContext(repoContext)
	findings := soulEditPlanFindings(target, normalized, allowed, matched, validation)
	status := soulEditPlanStatus(findings)

	var b strings.Builder
	b.WriteString("## GitClaw Soul Edit Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_edit_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", shortDocumentHash(target))
	fmt.Fprintf(&b, "- target_terms: `%d`\n", len(memorySearchTerms(target)))
	fmt.Fprintf(&b, "- target_allowed: `%t`\n", allowed)
	fmt.Fprintf(&b, "- normalized_soul_path: `%s`\n", inlineCode(displayPath))
	fmt.Fprintf(&b, "- target_category: `%s`\n", soulDocumentCategory(displayPath))
	fmt.Fprintf(&b, "- target_present: `%t`\n", allowed && match.Present)
	fmt.Fprintf(&b, "- target_required: `%t`\n", allowed && match.Required)
	fmt.Fprintf(&b, "- target_canonical: `%t`\n", allowed && match.Canonical)
	fmt.Fprintf(&b, "- target_loaded_for_this_turn: `%t`\n", allowed && match.LoadedForThisTurn)
	fmt.Fprintf(&b, "- matched_soul_files: `%d`\n", matched)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- edit_operations_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- branch_creation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- commit_push_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- model_self_modification_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- manual_review_required: `%t`\n", true)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	fmt.Fprintf(&b, "- raw_target_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_requested_change_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", false)
	writeSoulValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is a dry-run planner only. It identifies the high-authority context file that would be reviewed, but it does not write files, create branches, commit, push, apply patches, call a model, or include raw requested changes or context bodies.\n\n")

	b.WriteString("### Current File Metadata\n")
	if !allowed || matched == 0 {
		b.WriteString("- none\n")
	} else {
		writeSoulInfoMatch(&b, match)
	}

	b.WriteString("\n### Review Steps\n")
	if !allowed {
		b.WriteString("1. Choose a supported target such as `soul`, `identity`, `user`, `tools`, `memory`, `heartbeat`, or a dated memory note.\n")
	} else {
		fmt.Fprintf(&b, "1. Review the proposed change outside the Actions job and keep `%s` as reviewed repository code.\n", displayPath)
		b.WriteString("2. Make the edit on a branch with a human-readable diff.\n")
		b.WriteString("3. Run `gitclaw soul validate`, `gitclaw soul verify`, and `gitclaw profile verify` before merging.\n")
		b.WriteString("4. Run a live GitHub Models conversation E2E after the soul change, not only deterministic report tests.\n")
	}

	b.WriteString("\n### Findings\n")
	writeSoulEditPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedSoulEditPlanPath(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/soul" {
		return ""
	}
	if !strings.EqualFold(fields[1], "edit-plan") && !strings.EqualFold(fields[1], "plan") {
		return ""
	}
	if len(fields) < 3 {
		return "__missing__"
	}
	return cleanSoulInfoPath(fields[2])
}

func soulEditPlanFindings(target, normalized string, allowed bool, matched int, validation SoulValidationReport) []soulEditPlanFinding {
	var findings []soulEditPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, soulEditPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "manual_review_required", "high-authority context changes must be reviewed as repository code before they affect model context")
	add("info", "repository_mutation_disabled", "edit planning does not create, update, delete, commit, push, or apply files")
	add("info", "model_self_modification_disabled", "the model is not allowed to rewrite its own soul, identity, memory, tools, or heartbeat files")
	if target == "" || target == "__missing__" {
		add("error", "target_missing", "provide a high-authority context target")
	}
	if normalized != "" && !allowed {
		add("error", "unsupported_soul_target", "target must resolve to a required .gitclaw context file or dated memory note")
	}
	if allowed && matched == 0 {
		add("warning", "target_metadata_unavailable", "target is allowed but current file metadata could not be resolved")
	}
	if allowed && matched == 1 {
		add("warning", "high_authority_context_change", "soul edits change prompt-visible identity or policy context and require a reviewed diff")
	}
	if validation.Errors > 0 {
		add("error", "soul_validation_errors_present", "fix existing soul validation errors before editing high-authority context")
	} else if validation.Warnings > 0 {
		add("warning", "soul_validation_warnings_present", "review existing soul validation warnings before editing high-authority context")
	}
	return findings
}

func soulEditPlanStatus(findings []soulEditPlanFinding) string {
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

func writeSoulEditPlanFindings(b *strings.Builder, findings []soulEditPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
