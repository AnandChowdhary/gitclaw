package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ToolsetProvenanceReport struct {
	Status                                     string
	Store                                      ToolsetStoreReport
	Toolsets                                   int
	ScannedToolsets                            int
	ToolsetToolRefs                            int
	ResolvedToolRefs                           int
	UnknownToolRefs                            int
	DisabledToolRefs                           int
	AllowlistBlockedToolRefs                   int
	ToolsetsWithInstruction                    int
	ToolsetsWithRiskFindings                   int
	GitTrackedToolsets                         int
	UntrackedToolsets                          int
	WorkingTreeDirtyToolsets                   int
	ToolsetsWithCommits                        int
	ToolsetsWithoutCommits                     int
	GitAvailable                               bool
	GitHistoryAvailable                        bool
	ToolsetActivationSupported                 bool
	RepositoryMutationAllowed                  bool
	ShellExecutionAllowed                      bool
	NetworkToolExecutionAllowed                bool
	RawToolsetBodiesIncluded                   bool
	RawToolsetInstructionsIncluded             bool
	RawToolOutputsIncluded                     bool
	RawGitSubjectsIncluded                     bool
	AuthorIdentitiesIncluded                   bool
	LLME2ERequiredAfterToolsetProvenanceChange bool
	Cards                                      []ToolsetProvenanceCard
	Findings                                   []ToolsetProvenanceFinding
}

type ToolsetProvenanceCard struct {
	Name                  string
	Path                  string
	Mode                  string
	Tools                 []string
	ResolvedTools         []string
	UnknownTools          []string
	DisabledTools         []string
	AllowlistBlockedTools []string
	Instruction           bool
	Description           bool
	Bytes                 int
	Lines                 int
	SHA                   string
	ParseError            bool
	ParseErrorSHA         string
	RiskFindings          int
	RiskMaxSeverity       string
	RiskCodes             []string
	GitTracked            bool
	WorkingTreeDirty      bool
	LastCommitSHA12       string
	LastCommitShort       string
	LastCommitDate        string
	SubjectSHA12          string
	CommitAvailable       bool
}

type ToolsetProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildToolsetProvenanceReport(cfg Config) ToolsetProvenanceReport {
	store := BuildToolsetStoreReport(cfg)
	report := ToolsetProvenanceReport{
		Status:                                     toolsetProvenanceBaseStatus(store.Status),
		Store:                                      store,
		Toolsets:                                   store.Toolsets,
		ScannedToolsets:                            store.ScannedToolsets,
		ToolsetToolRefs:                            store.ToolsetToolRefs,
		ResolvedToolRefs:                           store.ResolvedToolRefs,
		UnknownToolRefs:                            store.UnknownToolRefs,
		DisabledToolRefs:                           store.DisabledToolRefs,
		AllowlistBlockedToolRefs:                   store.AllowlistBlockedToolRefs,
		ToolsetsWithInstruction:                    store.ToolsetsWithInstruction,
		ToolsetsWithRiskFindings:                   store.ToolsetsWithRiskFindings,
		GitAvailable:                               soulGitAvailable(),
		ToolsetActivationSupported:                 false,
		RepositoryMutationAllowed:                  false,
		ShellExecutionAllowed:                      false,
		NetworkToolExecutionAllowed:                false,
		RawToolsetBodiesIncluded:                   false,
		RawToolsetInstructionsIncluded:             false,
		RawToolOutputsIncluded:                     false,
		RawGitSubjectsIncluded:                     false,
		AuthorIdentitiesIncluded:                   false,
		LLME2ERequiredAfterToolsetProvenanceChange: true,
	}
	for _, summary := range store.Summaries {
		card := ToolsetProvenanceCard{
			Name:                  summary.Name,
			Path:                  summary.Path,
			Mode:                  summary.Mode,
			Tools:                 append([]string(nil), summary.Tools...),
			ResolvedTools:         append([]string(nil), summary.ResolvedTools...),
			UnknownTools:          append([]string(nil), summary.UnknownTools...),
			DisabledTools:         append([]string(nil), summary.DisabledTools...),
			AllowlistBlockedTools: append([]string(nil), summary.AllowlistBlockedTools...),
			Instruction:           summary.InstructionPresent,
			Description:           summary.DescriptionPresent,
			Bytes:                 summary.Bytes,
			Lines:                 summary.Lines,
			SHA:                   summary.SHA,
			ParseError:            strings.TrimSpace(summary.ParseError) != "",
			ParseErrorSHA:         hashStringOrNone(summary.ParseError),
			RiskFindings:          len(summary.RiskFindings),
			RiskMaxSeverity:       toolRiskMaxSeverity(summary.RiskFindings),
			RiskCodes:             toolRiskCodes(summary.RiskFindings),
			LastCommitSHA12:       "none",
			LastCommitShort:       "none",
			LastCommitDate:        "none",
			SubjectSHA12:          "none",
			CommitAvailable:       false,
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, summary.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedToolsets++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, summary.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtyToolsets++
				report.addFinding("warning", "dirty_toolset_file", summary.Path, "toolset file has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, summary.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.ToolsetsWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.ToolsetsWithoutCommits++
				report.addFinding("warning", "missing_git_history", summary.Path, "no git commit was found for this toolset file")
			}
		} else {
			report.UntrackedToolsets++
			detail := "toolset file is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_toolset_file", summary.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	sort.Slice(report.Cards, func(i, j int) bool { return report.Cards[i].Path < report.Cards[j].Path })
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for toolset provenance checks")
	}
	if report.Toolsets > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local toolset files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderToolsetsProvenanceCLIReport(cfg Config) string {
	return renderToolsetsProvenanceReport(Event{}, cfg, false)
}

func renderToolsetsProvenanceReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildToolsetProvenanceReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw Toolsets Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeToolsetReportHeader(&b, ev, includeIssue)
	writeToolsetProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local toolset YAML files to body-free git provenance. It reports tool refs, config-gating counts, risk codes, hashes, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; raw toolset YAML, toolset instructions, tool outputs, issue bodies, comments, prompts, git subjects, author identities, provider payloads, and secret values are not included.\n\n")

	b.WriteString("### Toolset Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeToolsetProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Store.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", toolsetProvenanceGitGate(report))
	fmt.Fprintf(&b, "- activation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- shell_execution_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- network_execution_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")

	b.WriteString("\n### Findings\n")
	writeToolsetProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeToolsetProvenanceSummary(b *strings.Builder, report ToolsetProvenanceReport) {
	fmt.Fprintf(b, "- toolset_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", "repo-local-toolset-git-history")
	fmt.Fprintf(b, "- toolset_store_status: `%s`\n", report.Store.Status)
	fmt.Fprintf(b, "- toolset_store_path: `%s`\n", toolsetStorePath)
	fmt.Fprintf(b, "- toolset_files: `%d`\n", report.Toolsets)
	fmt.Fprintf(b, "- scanned_toolsets: `%d`\n", report.ScannedToolsets)
	fmt.Fprintf(b, "- toolset_tool_refs: `%d`\n", report.ToolsetToolRefs)
	fmt.Fprintf(b, "- resolved_tool_refs: `%d`\n", report.ResolvedToolRefs)
	fmt.Fprintf(b, "- unknown_tool_refs: `%d`\n", report.UnknownToolRefs)
	fmt.Fprintf(b, "- disabled_tool_refs: `%d`\n", report.DisabledToolRefs)
	fmt.Fprintf(b, "- allowlist_blocked_tool_refs: `%d`\n", report.AllowlistBlockedToolRefs)
	fmt.Fprintf(b, "- toolsets_with_instruction: `%d`\n", report.ToolsetsWithInstruction)
	fmt.Fprintf(b, "- toolsets_with_risk_findings: `%d`\n", report.ToolsetsWithRiskFindings)
	fmt.Fprintf(b, "- git_tracked_toolsets: `%d`\n", report.GitTrackedToolsets)
	fmt.Fprintf(b, "- untracked_toolsets: `%d`\n", report.UntrackedToolsets)
	fmt.Fprintf(b, "- working_tree_dirty_toolsets: `%d`\n", report.WorkingTreeDirtyToolsets)
	fmt.Fprintf(b, "- toolsets_with_commits: `%d`\n", report.ToolsetsWithCommits)
	fmt.Fprintf(b, "- toolsets_without_commits: `%d`\n", report.ToolsetsWithoutCommits)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(b, "- toolset_activation_supported: `%t`\n", report.ToolsetActivationSupported)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- shell_execution_allowed: `%t`\n", report.ShellExecutionAllowed)
	fmt.Fprintf(b, "- network_tool_execution_allowed: `%t`\n", report.NetworkToolExecutionAllowed)
	fmt.Fprintf(b, "- raw_toolset_bodies_included: `%t`\n", report.RawToolsetBodiesIncluded)
	fmt.Fprintf(b, "- raw_toolset_instructions_included: `%t`\n", report.RawToolsetInstructionsIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_toolset_provenance_change: `%t`\n", report.LLME2ERequiredAfterToolsetProvenanceChange)
}

func writeToolsetProvenanceCard(b *strings.Builder, card ToolsetProvenanceCard) {
	fmt.Fprintf(
		b,
		"- toolset_name=`%s` path=`%s` mode=`%s` tools=`%s` resolved_tools=`%s` unknown_tools=`%s` disabled_tools=`%s` allowlist_blocked_tools=`%s` instruction=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` parse_error=`%t` parse_error_sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		inlineCode(card.Name),
		card.Path,
		card.Mode,
		inlineListOrNone(card.Tools),
		inlineListOrNone(card.ResolvedTools),
		inlineListOrNone(card.UnknownTools),
		inlineListOrNone(card.DisabledTools),
		inlineListOrNone(card.AllowlistBlockedTools),
		card.Instruction,
		card.Description,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.ParseError,
		card.ParseErrorSHA,
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

func writeToolsetProvenanceFindings(b *strings.Builder, findings []ToolsetProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func toolsetProvenanceBaseStatus(storeStatus string) string {
	switch storeStatus {
	case "high":
		return "high"
	case "warn":
		return "warn"
	case "ok":
		return "ok"
	case "":
		return "unknown"
	default:
		return storeStatus
	}
}

func toolsetProvenanceGitGate(report ToolsetProvenanceReport) string {
	if report.Toolsets == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedToolsets > 0 || report.ToolsetsWithoutCommits > 0 || report.WorkingTreeDirtyToolsets > 0 {
		return "warn"
	}
	return "pass"
}

func (r *ToolsetProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, ToolsetProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
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
