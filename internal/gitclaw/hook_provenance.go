package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type HookProvenanceReport struct {
	Status                                  string
	HookStatus                              string
	Risk                                    HookRiskReport
	HookPolicyPresent                       bool
	HookPolicyLoadedForModel                bool
	HookSpecs                               int
	HookSpecsWithFrontmatter                int
	HookEvents                              int
	HookSpecsRequiringApproval              int
	HookSpecsAuditOnly                      int
	ExecutableHandlersPresent               int
	ProvenanceSurfaces                      int
	GitTrackedSurfaces                      int
	UntrackedSurfaces                       int
	WorkingTreeDirtySurfaces                int
	SurfacesWithCommits                     int
	SurfacesWithoutCommits                  int
	GitAvailable                            bool
	GitHistoryAvailable                     bool
	HookExecutionAllowed                    bool
	RepositoryMutationAllowed               bool
	RawHookBodiesIncluded                   bool
	RawHandlerBodiesIncluded                bool
	RawGitSubjectsIncluded                  bool
	AuthorIdentitiesIncluded                bool
	LLME2ERequiredAfterHookProvenanceChange bool
	Cards                                   []HookProvenanceCard
	Findings                                []HookProvenanceFinding
}

type HookProvenanceCard struct {
	Kind             string
	Name             string
	Path             string
	Present          bool
	LoadedForModel   bool
	Frontmatter      bool
	Events           int
	Mode             string
	Delivery         string
	RequiresApproval bool
	IgnoredHandler   bool
	Bytes            int
	Lines            int
	SHA              string
	RiskFindings     int
	RiskMaxSeverity  string
	RiskCodes        []string
	GitTracked       bool
	WorkingTreeDirty bool
	LastCommitSHA12  string
	LastCommitShort  string
	LastCommitDate   string
	SubjectSHA12     string
	CommitAvailable  bool
}

type HookProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildHookProvenanceReport(cfg Config) HookProvenanceReport {
	surface := inspectHookSurface(cfg.Workdir)
	validationFindings := hookFindings(surface)
	hookStatusValue := hookStatus(surface, validationFindings)
	risk := BuildHookRiskReport(cfg)
	report := HookProvenanceReport{
		Status:                                  hookProvenanceBaseStatus(hookStatusValue, risk.Status),
		HookStatus:                              hookStatusValue,
		Risk:                                    risk,
		HookPolicyPresent:                       surface.Policy.Present,
		HookPolicyLoadedForModel:                hookPolicyLoadedForModel(surface),
		HookSpecs:                               len(surface.Specs),
		HookSpecsWithFrontmatter:                hookSpecsWithFrontmatter(surface.Specs),
		HookEvents:                              hookEventCount(surface.Specs),
		HookSpecsRequiringApproval:              hookSpecsRequiringApproval(surface.Specs),
		HookSpecsAuditOnly:                      hookSpecsAuditOnly(surface.Specs),
		ExecutableHandlersPresent:               len(surface.ExecutableHandlers),
		GitAvailable:                            soulGitAvailable(),
		HookExecutionAllowed:                    false,
		RepositoryMutationAllowed:               false,
		RawHookBodiesIncluded:                   false,
		RawHandlerBodiesIncluded:                false,
		RawGitSubjectsIncluded:                  false,
		AuthorIdentitiesIncluded:                false,
		LLME2ERequiredAfterHookProvenanceChange: true,
	}
	if surface.Policy.Present {
		report.addCard(hookPolicyProvenanceCard(cfg, surface.Policy, risk))
	}
	for _, spec := range surface.Specs {
		report.addCard(hookSpecProvenanceCard(cfg, spec, risk))
	}
	for _, handler := range surface.ExecutableHandlers {
		report.addCard(hookHandlerProvenanceCard(cfg, handler, risk))
	}
	sort.Slice(report.Cards, func(i, j int) bool {
		if report.Cards[i].Path != report.Cards[j].Path {
			return report.Cards[i].Path < report.Cards[j].Path
		}
		return report.Cards[i].Kind < report.Cards[j].Kind
	})
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for hook provenance checks")
	}
	if report.ProvenanceSurfaces > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local hook files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderHookProvenanceCLIReport(cfg Config) string {
	return renderHookProvenanceReport(Event{}, cfg, false)
}

func renderHookProvenanceReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildHookProvenanceReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Hook Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeHookProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local hook policy, hook specs, and ignored executable-looking handlers to body-free git provenance. It reports paths, counts, hashes, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; hook bodies, handler bodies, issue bodies, comments, prompts, git subjects, author identities, provider payloads, and secret values are not included.\n\n")

	b.WriteString("### Hook Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeHookProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.HookStatus))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", hookProvenanceGitGate(report))
	fmt.Fprintf(&b, "- execution_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")

	b.WriteString("\n### Findings\n")
	writeHookProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeHookProvenanceSummary(b *strings.Builder, report HookProvenanceReport) {
	fmt.Fprintf(b, "- hook_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", "repo-local-hook-git-history")
	fmt.Fprintf(b, "- hooks_status: `%s`\n", report.HookStatus)
	fmt.Fprintf(b, "- hook_risk_status: `%s`\n", report.Risk.Status)
	fmt.Fprintf(b, "- hook_policy_present: `%t`\n", report.HookPolicyPresent)
	fmt.Fprintf(b, "- hook_policy_loaded_for_model: `%t`\n", report.HookPolicyLoadedForModel)
	fmt.Fprintf(b, "- hook_specs: `%d`\n", report.HookSpecs)
	fmt.Fprintf(b, "- hook_specs_with_frontmatter: `%d`\n", report.HookSpecsWithFrontmatter)
	fmt.Fprintf(b, "- hook_events: `%d`\n", report.HookEvents)
	fmt.Fprintf(b, "- hook_specs_requiring_approval: `%d`\n", report.HookSpecsRequiringApproval)
	fmt.Fprintf(b, "- hook_specs_audit_only: `%d`\n", report.HookSpecsAuditOnly)
	fmt.Fprintf(b, "- executable_handlers_present: `%d`\n", report.ExecutableHandlersPresent)
	fmt.Fprintf(b, "- provenance_surfaces: `%d`\n", report.ProvenanceSurfaces)
	fmt.Fprintf(b, "- git_tracked_surfaces: `%d`\n", report.GitTrackedSurfaces)
	fmt.Fprintf(b, "- untracked_surfaces: `%d`\n", report.UntrackedSurfaces)
	fmt.Fprintf(b, "- working_tree_dirty_surfaces: `%d`\n", report.WorkingTreeDirtySurfaces)
	fmt.Fprintf(b, "- surfaces_with_commits: `%d`\n", report.SurfacesWithCommits)
	fmt.Fprintf(b, "- surfaces_without_commits: `%d`\n", report.SurfacesWithoutCommits)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(b, "- hook_execution_allowed: `%t`\n", report.HookExecutionAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_hook_bodies_included: `%t`\n", report.RawHookBodiesIncluded)
	fmt.Fprintf(b, "- raw_handler_bodies_included: `%t`\n", report.RawHandlerBodiesIncluded)
	fmt.Fprintf(b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_hook_provenance_change: `%t`\n", report.LLME2ERequiredAfterHookProvenanceChange)
}

func writeHookProvenanceCard(b *strings.Builder, card HookProvenanceCard) {
	fmt.Fprintf(
		b,
		"- kind=`%s` name=`%s` path=`%s` present=`%t` loaded_for_model=`%t` frontmatter=`%t` events=`%d` mode=`%s` delivery=`%s` requires_approval=`%t` ignored_handler=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		card.Kind,
		inlineCode(card.Name),
		card.Path,
		card.Present,
		card.LoadedForModel,
		card.Frontmatter,
		card.Events,
		inlineCode(card.Mode),
		inlineCode(card.Delivery),
		card.RequiresApproval,
		card.IgnoredHandler,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.RiskFindings,
		card.RiskMaxSeverity,
		inlineListOrNone(card.RiskCodes),
		card.GitTracked,
		card.WorkingTreeDirty,
		card.CommitAvailable,
		card.LastCommitSHA12,
		card.LastCommitShort,
		card.LastCommitDate,
		card.SubjectSHA12,
	)
}

func writeHookProvenanceFindings(b *strings.Builder, findings []HookProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func hookPolicyProvenanceCard(cfg Config, policy configSurfaceFile, risk HookRiskReport) HookProvenanceCard {
	card := HookProvenanceCard{
		Kind:             "hook-policy",
		Name:             "hooks-policy",
		Path:             policy.Path,
		Present:          policy.Present,
		LoadedForModel:   hookPolicyPathInContext(),
		Mode:             "policy",
		Delivery:         "context",
		Bytes:            policy.Bytes,
		Lines:            policy.Lines,
		SHA:              policy.SHA,
		LastCommitSHA12:  "none",
		LastCommitShort:  "none",
		LastCommitDate:   "none",
		SubjectSHA12:     "none",
		RiskMaxSeverity:  "none",
		CommitAvailable:  false,
		GitTracked:       false,
		WorkingTreeDirty: false,
	}
	return withHookProvenanceGit(cfg, card, hookRiskFindingsForPath(risk, policy.Path))
}

func hookSpecProvenanceCard(cfg Config, spec hookSpecCard, risk HookRiskReport) HookProvenanceCard {
	card := HookProvenanceCard{
		Kind:             "hook-spec",
		Name:             spec.Name,
		Path:             spec.Path,
		Present:          spec.Present,
		Frontmatter:      spec.Frontmatter,
		Events:           len(spec.Events),
		Mode:             spec.Mode,
		Delivery:         spec.Delivery,
		RequiresApproval: spec.RequiresApproval,
		Bytes:            spec.Bytes,
		Lines:            spec.Lines,
		SHA:              spec.SHA,
		LastCommitSHA12:  "none",
		LastCommitShort:  "none",
		LastCommitDate:   "none",
		SubjectSHA12:     "none",
		RiskMaxSeverity:  "none",
		CommitAvailable:  false,
		GitTracked:       false,
		WorkingTreeDirty: false,
	}
	return withHookProvenanceGit(cfg, card, hookRiskFindingsForPath(risk, spec.Path))
}

func hookHandlerProvenanceCard(cfg Config, handler configSurfaceFile, risk HookRiskReport) HookProvenanceCard {
	card := HookProvenanceCard{
		Kind:             "handler",
		Name:             strings.TrimSuffix(strings.TrimPrefix(handler.Path, hookSpecsDir+"/"), ".md"),
		Path:             handler.Path,
		Present:          handler.Present,
		Mode:             "ignored",
		Delivery:         "none",
		IgnoredHandler:   true,
		Bytes:            handler.Bytes,
		Lines:            handler.Lines,
		SHA:              handler.SHA,
		LastCommitSHA12:  "none",
		LastCommitShort:  "none",
		LastCommitDate:   "none",
		SubjectSHA12:     "none",
		RiskMaxSeverity:  "none",
		CommitAvailable:  false,
		GitTracked:       false,
		WorkingTreeDirty: false,
	}
	return withHookProvenanceGit(cfg, card, hookRiskFindingsForPath(risk, handler.Path))
}

func withHookProvenanceGit(cfg Config, card HookProvenanceCard, findings []HookRiskFinding) HookProvenanceCard {
	card.RiskFindings = len(findings)
	card.RiskMaxSeverity = hookRiskMaxSeverity(findings)
	card.RiskCodes = hookRiskCodes(findings)
	if card.RiskMaxSeverity == "" {
		card.RiskMaxSeverity = "none"
	}
	tracked, _ := soulGitTracked(cfg.Workdir, card.Path)
	card.GitTracked = tracked
	if tracked {
		card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, card.Path)
		info, ok := soulGitLastCommit(cfg.Workdir, card.Path)
		if ok {
			card.CommitAvailable = true
			card.LastCommitSHA12 = shortSHA(info.FullSHA)
			card.LastCommitShort = info.ShortSHA
			card.LastCommitDate = info.Date
			card.SubjectSHA12 = shortDocumentHash(info.Subject)
		}
	}
	return card
}

func hookRiskFindingsForPath(report HookRiskReport, path string) []HookRiskFinding {
	var findings []HookRiskFinding
	for _, finding := range report.Findings {
		if finding.Path == path {
			findings = append(findings, finding)
		}
	}
	return findings
}

func (r *HookProvenanceReport) addCard(card HookProvenanceCard) {
	r.Cards = append(r.Cards, card)
	r.ProvenanceSurfaces++
	if card.GitTracked {
		r.GitTrackedSurfaces++
		if card.WorkingTreeDirty {
			r.WorkingTreeDirtySurfaces++
			r.addFinding("warning", "dirty_hook_file", card.Path, "hook provenance surface has uncommitted working-tree changes")
		}
		if card.CommitAvailable {
			r.SurfacesWithCommits++
			r.GitHistoryAvailable = true
		} else {
			r.SurfacesWithoutCommits++
			r.addFinding("warning", "missing_git_history", card.Path, "no git commit was found for this hook provenance surface")
		}
		return
	}
	r.UntrackedSurfaces++
	r.addFinding("warning", "untracked_hook_file", card.Path, "hook provenance surface is not tracked by git")
}

func hookProvenanceBaseStatus(hookStatusValue, riskStatus string) string {
	if riskStatus == "high" {
		return "high"
	}
	if hookStatusValue == "error" {
		return "error"
	}
	if hookStatusValue == "warning" || riskStatus == "warn" {
		return "warn"
	}
	if hookStatusValue == "not_configured" {
		return "not_configured"
	}
	if hookStatusValue == "" && riskStatus == "" {
		return "unknown"
	}
	return "ok"
}

func hookProvenanceGitGate(report HookProvenanceReport) string {
	if report.ProvenanceSurfaces == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedSurfaces > 0 || report.SurfacesWithoutCommits > 0 || report.WorkingTreeDirtySurfaces > 0 {
		return "warn"
	}
	return "pass"
}

func (r *HookProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, HookProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return r.Findings[i].Severity < r.Findings[j].Severity
		}
		if r.Findings[i].Code != r.Findings[j].Code {
			return r.Findings[i].Code < r.Findings[j].Code
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}
