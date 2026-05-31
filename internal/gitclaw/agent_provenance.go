package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type AgentProvenanceReport struct {
	Status                                   string
	AgentStatus                              string
	Risk                                     AgentRiskReport
	AgentPolicyPresent                       bool
	AgentPolicyLoadedForModel                bool
	AgentSpecs                               int
	AgentSpecsWithFrontmatter                int
	AgentRoles                               int
	AgentToolsRequested                      int
	AgentSpecsRequiringApproval              int
	AgentSpecsSingleAssistant                int
	AgentValidationFindings                  int
	ProvenanceSurfaces                       int
	RepoLocalSurfaces                        int
	UnknownSourceSurfaces                    int
	GitTrackedSurfaces                       int
	UntrackedSurfaces                        int
	WorkingTreeDirtySurfaces                 int
	SurfacesWithCommits                      int
	SurfacesWithoutCommits                   int
	GitAvailable                             bool
	GitHistoryAvailable                      bool
	ActiveAgentRuntime                       string
	MultiAgentRoutingSupported               bool
	MultiAgentDelegationSupported            bool
	SubagentExecutionSupported               bool
	DelegateTaskSupported                    bool
	RemoteAgentProcessAllowed                bool
	AgentToAgentMessagingAllowed             bool
	RepositoryMutationAllowed                bool
	RawAgentBodiesIncluded                   bool
	RawIssueBodiesIncluded                   bool
	RawCommentBodiesIncluded                 bool
	RawPromptBodiesIncluded                  bool
	RawToolOutputsIncluded                   bool
	RawGitSubjectsIncluded                   bool
	AuthorIdentitiesIncluded                 bool
	CredentialValuesIncluded                 bool
	LLME2ERequiredAfterAgentProvenanceChange bool
	Cards                                    []AgentProvenanceCard
	Findings                                 []AgentProvenanceFinding
}

type AgentProvenanceCard struct {
	Kind               string
	Name               string
	Path               string
	Source             string
	Present            bool
	LoadedForModel     bool
	Frontmatter        bool
	Role               string
	Runtime            string
	Mode               string
	Tools              int
	RequiresApproval   bool
	Bytes              int
	Lines              int
	SHA                string
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	ValidationFindings int
	GitTracked         bool
	WorkingTreeDirty   bool
	LastCommitSHA12    string
	LastCommitShort    string
	LastCommitDate     string
	SubjectSHA12       string
	CommitAvailable    bool
}

type AgentProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildAgentProvenanceReport(cfg Config) AgentProvenanceReport {
	surface := inspectAgentSurface(cfg.Workdir)
	validationFindings := agentFindings(surface)
	agentStatusValue := agentStatus(surface, validationFindings)
	risk := BuildAgentRiskReport(cfg, false)
	report := AgentProvenanceReport{
		Status:                                   agentProvenanceBaseStatus(agentStatusValue, risk.Status),
		AgentStatus:                              agentStatusValue,
		Risk:                                     risk,
		AgentPolicyPresent:                       surface.Policy.Present,
		AgentPolicyLoadedForModel:                agentPolicyLoadedForModel(surface),
		AgentSpecs:                               len(surface.Specs),
		AgentSpecsWithFrontmatter:                agentSpecsWithFrontmatter(surface.Specs),
		AgentRoles:                               agentRoleCount(surface.Specs),
		AgentToolsRequested:                      agentToolCount(surface.Specs),
		AgentSpecsRequiringApproval:              agentSpecsRequiringApproval(surface.Specs),
		AgentSpecsSingleAssistant:                agentSpecsSingleAssistant(surface.Specs),
		AgentValidationFindings:                  len(validationFindings),
		GitAvailable:                             soulGitAvailable(),
		ActiveAgentRuntime:                       "github-actions",
		MultiAgentRoutingSupported:               false,
		MultiAgentDelegationSupported:            false,
		SubagentExecutionSupported:               false,
		DelegateTaskSupported:                    false,
		RemoteAgentProcessAllowed:                false,
		AgentToAgentMessagingAllowed:             false,
		RepositoryMutationAllowed:                false,
		RawAgentBodiesIncluded:                   false,
		RawIssueBodiesIncluded:                   false,
		RawCommentBodiesIncluded:                 false,
		RawPromptBodiesIncluded:                  false,
		RawToolOutputsIncluded:                   false,
		RawGitSubjectsIncluded:                   false,
		AuthorIdentitiesIncluded:                 false,
		CredentialValuesIncluded:                 false,
		LLME2ERequiredAfterAgentProvenanceChange: true,
	}
	validationByPath := agentValidationFindingCountByPath(validationFindings)
	if surface.Policy.Present {
		report.addCard(agentPolicyProvenanceCard(cfg, surface.Policy, risk, validationByPath))
	}
	for _, spec := range surface.Specs {
		report.addCard(agentSpecProvenanceCard(cfg, spec, risk, validationByPath))
	}
	sort.Slice(report.Cards, func(i, j int) bool {
		if report.Cards[i].Path != report.Cards[j].Path {
			return report.Cards[i].Path < report.Cards[j].Path
		}
		return report.Cards[i].Kind < report.Cards[j].Kind
	})
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for agent provenance checks")
	}
	if report.ProvenanceSurfaces > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local agent files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderAgentProvenanceCLIReport(cfg Config) string {
	return renderAgentProvenanceReport(Event{}, cfg, false)
}

func RenderAgentProvenanceReport(ev Event, cfg Config) string {
	return renderAgentProvenanceReport(ev, cfg, true)
}

func renderAgentProvenanceReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildAgentProvenanceReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Agent Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeAgentProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local agent policy and agent specs to body-free git provenance. It reports paths, sources, counts, hashes, tracked state, dirty state, last commit IDs/dates, risk metadata, validation counts, and commit-subject hashes only; agent bodies, issue bodies, comments, prompts, tool outputs, git subjects, author identities, channel payloads, and secret values are not included.\n\n")

	b.WriteString("### Agent Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeAgentProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- agent_validation_gate=`%s`\n", soulAnchorGate(report.AgentStatus))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", agentProvenanceGitGate(report))
	fmt.Fprintf(&b, "- runtime_gate=`%s`\n", "github-actions-only")
	fmt.Fprintf(&b, "- delegation_gate=`%s`\n", "disabled-single-assistant-v1")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")

	b.WriteString("\n### Findings\n")
	writeAgentProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeAgentProvenanceSummary(b *strings.Builder, report AgentProvenanceReport) {
	fmt.Fprintf(b, "- agent_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", "repo-local-agent-git-history")
	fmt.Fprintf(b, "- agents_status: `%s`\n", report.AgentStatus)
	fmt.Fprintf(b, "- agent_validation_status: `%s`\n", report.AgentStatus)
	fmt.Fprintf(b, "- agent_validation_findings: `%d`\n", report.AgentValidationFindings)
	fmt.Fprintf(b, "- agent_risk_status: `%s`\n", report.Risk.Status)
	fmt.Fprintf(b, "- agent_risk_findings: `%d`\n", len(report.Risk.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.Risk.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.Risk.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.Risk.InfoRiskFindings)
	fmt.Fprintf(b, "- agent_policy_present: `%t`\n", report.AgentPolicyPresent)
	fmt.Fprintf(b, "- agent_policy_loaded_for_model: `%t`\n", report.AgentPolicyLoadedForModel)
	fmt.Fprintf(b, "- agent_specs: `%d`\n", report.AgentSpecs)
	fmt.Fprintf(b, "- agent_specs_with_frontmatter: `%d`\n", report.AgentSpecsWithFrontmatter)
	fmt.Fprintf(b, "- agent_roles: `%d`\n", report.AgentRoles)
	fmt.Fprintf(b, "- agent_tools_requested: `%d`\n", report.AgentToolsRequested)
	fmt.Fprintf(b, "- agent_specs_requiring_approval: `%d`\n", report.AgentSpecsRequiringApproval)
	fmt.Fprintf(b, "- agent_specs_single_assistant: `%d`\n", report.AgentSpecsSingleAssistant)
	fmt.Fprintf(b, "- provenance_surfaces: `%d`\n", report.ProvenanceSurfaces)
	fmt.Fprintf(b, "- repo_local_surfaces: `%d`\n", report.RepoLocalSurfaces)
	fmt.Fprintf(b, "- unknown_source_surfaces: `%d`\n", report.UnknownSourceSurfaces)
	fmt.Fprintf(b, "- git_tracked_surfaces: `%d`\n", report.GitTrackedSurfaces)
	fmt.Fprintf(b, "- untracked_surfaces: `%d`\n", report.UntrackedSurfaces)
	fmt.Fprintf(b, "- working_tree_dirty_surfaces: `%d`\n", report.WorkingTreeDirtySurfaces)
	fmt.Fprintf(b, "- surfaces_with_commits: `%d`\n", report.SurfacesWithCommits)
	fmt.Fprintf(b, "- surfaces_without_commits: `%d`\n", report.SurfacesWithoutCommits)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(b, "- active_agent_runtime: `%s`\n", report.ActiveAgentRuntime)
	fmt.Fprintf(b, "- multi_agent_routing_supported: `%t`\n", report.MultiAgentRoutingSupported)
	fmt.Fprintf(b, "- multi_agent_delegation_supported: `%t`\n", report.MultiAgentDelegationSupported)
	fmt.Fprintf(b, "- subagent_execution_supported: `%t`\n", report.SubagentExecutionSupported)
	fmt.Fprintf(b, "- delegate_task_supported: `%t`\n", report.DelegateTaskSupported)
	fmt.Fprintf(b, "- remote_agent_process_allowed: `%t`\n", report.RemoteAgentProcessAllowed)
	fmt.Fprintf(b, "- agent_to_agent_messaging_allowed: `%t`\n", report.AgentToAgentMessagingAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_agent_bodies_included: `%t`\n", report.RawAgentBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_agent_provenance_change: `%t`\n", report.LLME2ERequiredAfterAgentProvenanceChange)
}

func writeAgentProvenanceCard(b *strings.Builder, card AgentProvenanceCard) {
	fmt.Fprintf(
		b,
		"- kind=`%s` name=`%s` path=`%s` source=`%s` present=`%t` loaded_for_model=`%t` frontmatter=`%t` role=`%s` runtime=`%s` mode=`%s` tools=`%d` requires_approval=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` validation_findings=`%d` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		card.Kind,
		inlineCode(card.Name),
		card.Path,
		card.Source,
		card.Present,
		card.LoadedForModel,
		card.Frontmatter,
		inlineCode(card.Role),
		inlineCode(card.Runtime),
		inlineCode(card.Mode),
		card.Tools,
		card.RequiresApproval,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.RiskFindings,
		card.RiskMaxSeverity,
		inlineListOrNone(card.RiskCodes),
		card.ValidationFindings,
		card.GitTracked,
		card.WorkingTreeDirty,
		card.CommitAvailable,
		card.LastCommitSHA12,
		card.LastCommitShort,
		card.LastCommitDate,
		card.SubjectSHA12,
	)
}

func writeAgentProvenanceFindings(b *strings.Builder, findings []AgentProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func agentPolicyProvenanceCard(cfg Config, policy configSurfaceFile, risk AgentRiskReport, validationByPath map[string]int) AgentProvenanceCard {
	card := AgentProvenanceCard{
		Kind:               "agent-policy",
		Name:               "agents-policy",
		Path:               policy.Path,
		Source:             agentTrustSource(policy.Path),
		Present:            policy.Present,
		LoadedForModel:     agentPolicyPathInContext(),
		Role:               "policy",
		Runtime:            "github-actions",
		Mode:               "single-assistant-boundary",
		Bytes:              policy.Bytes,
		Lines:              policy.Lines,
		SHA:                policy.SHA,
		ValidationFindings: validationByPath[policy.Path],
		LastCommitSHA12:    "none",
		LastCommitShort:    "none",
		LastCommitDate:     "none",
		SubjectSHA12:       "none",
		RiskMaxSeverity:    "none",
	}
	return withAgentProvenanceGit(cfg, card, agentRiskFindingsForPath(risk, policy.Path))
}

func agentSpecProvenanceCard(cfg Config, spec agentSpecCard, risk AgentRiskReport, validationByPath map[string]int) AgentProvenanceCard {
	card := AgentProvenanceCard{
		Kind:               "agent-spec",
		Name:               spec.Name,
		Path:               spec.Path,
		Source:             agentTrustSource(spec.Path),
		Present:            spec.Present,
		Frontmatter:        spec.Frontmatter,
		Role:               spec.Role,
		Runtime:            spec.Runtime,
		Mode:               spec.Mode,
		Tools:              len(spec.Tools),
		RequiresApproval:   spec.RequiresApproval,
		Bytes:              spec.Bytes,
		Lines:              spec.Lines,
		SHA:                spec.SHA,
		ValidationFindings: validationByPath[spec.Path],
		LastCommitSHA12:    "none",
		LastCommitShort:    "none",
		LastCommitDate:     "none",
		SubjectSHA12:       "none",
		RiskMaxSeverity:    "none",
	}
	return withAgentProvenanceGit(cfg, card, agentRiskFindingsForPath(risk, spec.Path))
}

func withAgentProvenanceGit(cfg Config, card AgentProvenanceCard, findings []AgentRiskFinding) AgentProvenanceCard {
	card.RiskFindings = len(findings)
	card.RiskMaxSeverity = agentRiskMaxSeverity(findings)
	card.RiskCodes = agentRiskCodes(findings)
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

func (r *AgentProvenanceReport) addCard(card AgentProvenanceCard) {
	r.Cards = append(r.Cards, card)
	r.ProvenanceSurfaces++
	switch card.Source {
	case "repo-local":
		r.RepoLocalSurfaces++
	default:
		r.UnknownSourceSurfaces++
		r.addFinding("warning", "unknown_agent_source", card.Path, "agent provenance surface is outside known repo-local agent roots")
	}
	if card.GitTracked {
		r.GitTrackedSurfaces++
		if card.WorkingTreeDirty {
			r.WorkingTreeDirtySurfaces++
			r.addFinding("warning", "dirty_agent_file", card.Path, "agent provenance surface has uncommitted working-tree changes")
		}
		if card.CommitAvailable {
			r.SurfacesWithCommits++
			r.GitHistoryAvailable = true
		} else {
			r.SurfacesWithoutCommits++
			r.addFinding("warning", "missing_git_history", card.Path, "no git commit was found for this agent provenance surface")
		}
		return
	}
	r.UntrackedSurfaces++
	r.addFinding("warning", "untracked_agent_file", card.Path, "agent provenance surface is not tracked by git")
}

func agentRiskFindingsForPath(report AgentRiskReport, path string) []AgentRiskFinding {
	var findings []AgentRiskFinding
	for _, finding := range report.Findings {
		if finding.Path == path {
			findings = append(findings, finding)
		}
	}
	return findings
}

func agentValidationFindingCountByPath(findings []agentFinding) map[string]int {
	counts := map[string]int{}
	for _, finding := range findings {
		counts[finding.Subject]++
	}
	return counts
}

func agentTrustSource(path string) string {
	if path == agentPolicyPath || strings.HasPrefix(path, agentSpecsDir+"/") {
		return "repo-local"
	}
	return "unknown"
}

func agentProvenanceBaseStatus(agentStatusValue, riskStatus string) string {
	if riskStatus == "high" {
		return "high"
	}
	if agentStatusValue == "error" {
		return "error"
	}
	if agentStatusValue == "warning" || riskStatus == "warn" {
		return "warn"
	}
	if agentStatusValue == "not_configured" {
		return "not_configured"
	}
	if agentStatusValue == "" && riskStatus == "" {
		return "unknown"
	}
	return "ok"
}

func agentProvenanceGitGate(report AgentProvenanceReport) string {
	if report.ProvenanceSurfaces == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedSurfaces > 0 || report.SurfacesWithoutCommits > 0 || report.WorkingTreeDirtySurfaces > 0 {
		return "warn"
	}
	return "pass"
}

func (r *AgentProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, AgentProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
	sort.Slice(r.Findings, func(i, j int) bool {
		if agentRiskSeverityRank(r.Findings[i].Severity) != agentRiskSeverityRank(r.Findings[j].Severity) {
			return agentRiskSeverityRank(r.Findings[i].Severity) > agentRiskSeverityRank(r.Findings[j].Severity)
		}
		if r.Findings[i].Code != r.Findings[j].Code {
			return r.Findings[i].Code < r.Findings[j].Code
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}

func isAgentProvenanceRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/agents" && command != "/agent" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "provenance", "history", "git-history":
		return true
	default:
		return false
	}
}
