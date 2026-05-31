package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type MCPProvenanceReport struct {
	Status                                 string
	MCP                                    MCPReport
	Specs                                  int
	ParsedSpecs                            int
	SpecsWithCommand                       int
	SpecsWithURL                           int
	SpecsWithToolAllowlist                 int
	ToolAllowlistRefs                      int
	ToolDenylistRefs                       int
	RequiredSecretRefs                     int
	EnvPassthroughRefs                     int
	SpecsWithResourcesEnabled              int
	SpecsWithPromptsEnabled                int
	SpecsWithRiskFindings                  int
	RiskFindings                           int
	HighRiskFindings                       int
	WarningRiskFindings                    int
	InfoRiskFindings                       int
	GitTrackedSpecs                        int
	UntrackedSpecs                         int
	WorkingTreeDirtySpecs                  int
	SpecsWithCommits                       int
	SpecsWithoutCommits                    int
	GitAvailable                           bool
	GitHistoryAvailable                    bool
	MCPConnectionSupported                 bool
	MCPServerLaunchAllowed                 bool
	MCPToolExposureAllowed                 bool
	DynamicToolDiscoveryAllowed            bool
	RepositoryMutationAllowed              bool
	RawMCPBodiesIncluded                   bool
	RawCommandArgsIncluded                 bool
	RawURLsIncluded                        bool
	RawGitSubjectsIncluded                 bool
	AuthorIdentitiesIncluded               bool
	CredentialValuesIncluded               bool
	EnvValuesIncluded                      bool
	LLME2ERequiredAfterMCPProvenanceChange bool
	Cards                                  []MCPProvenanceCard
	Findings                               []MCPProvenanceFinding
}

type MCPProvenanceCard struct {
	Name               string
	Path               string
	Description        bool
	Transport          string
	Activation         string
	SourcePresent      bool
	SourceSHA          string
	CommandPresent     bool
	CommandSHA         string
	ArgsCount          int
	ArgsSHA            string
	URLPresent         bool
	URLSHA             string
	ToolAllowlist      []string
	ToolDenylist       []string
	RequiresSecretRefs int
	RequiresSecretsSHA string
	EnvPassthroughRefs int
	EnvPassthroughSHA  string
	Resources          bool
	Prompts            bool
	Bytes              int
	Lines              int
	SHA                string
	ParseError         bool
	ParseErrorSHA      string
	RiskFindings       int
	RiskMaxSeverity    string
	RiskCodes          []string
	GitTracked         bool
	WorkingTreeDirty   bool
	LastCommitSHA12    string
	LastCommitShort    string
	LastCommitDate     string
	SubjectSHA12       string
	CommitAvailable    bool
}

type MCPProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildMCPProvenanceReport(cfg Config) MCPProvenanceReport {
	mcp := BuildMCPReport(cfg)
	report := MCPProvenanceReport{
		Status:                                 mcpProvenanceBaseStatus(mcp.Status),
		MCP:                                    mcp,
		Specs:                                  mcp.Specs,
		ParsedSpecs:                            mcp.ParsedSpecs,
		SpecsWithCommand:                       mcp.SpecsWithCommand,
		SpecsWithURL:                           mcp.SpecsWithURL,
		SpecsWithToolAllowlist:                 mcp.SpecsWithToolAllowlist,
		ToolAllowlistRefs:                      mcp.ToolAllowlistRefs,
		ToolDenylistRefs:                       mcp.ToolDenylistRefs,
		RequiredSecretRefs:                     mcp.RequiredSecretRefs,
		EnvPassthroughRefs:                     mcp.EnvPassthroughRefs,
		SpecsWithResourcesEnabled:              mcp.SpecsWithResourcesEnabled,
		SpecsWithPromptsEnabled:                mcp.SpecsWithPromptsEnabled,
		SpecsWithRiskFindings:                  mcp.SpecsWithRiskFindings,
		RiskFindings:                           len(mcp.Findings),
		HighRiskFindings:                       mcp.HighRiskFindings,
		WarningRiskFindings:                    mcp.WarningRiskFindings,
		InfoRiskFindings:                       mcp.InfoRiskFindings,
		GitAvailable:                           soulGitAvailable(),
		MCPConnectionSupported:                 false,
		MCPServerLaunchAllowed:                 false,
		MCPToolExposureAllowed:                 false,
		DynamicToolDiscoveryAllowed:            false,
		RepositoryMutationAllowed:              false,
		RawMCPBodiesIncluded:                   false,
		RawCommandArgsIncluded:                 false,
		RawURLsIncluded:                        false,
		RawGitSubjectsIncluded:                 false,
		AuthorIdentitiesIncluded:               false,
		CredentialValuesIncluded:               false,
		EnvValuesIncluded:                      false,
		LLME2ERequiredAfterMCPProvenanceChange: true,
	}
	for _, summary := range mcp.Cards {
		card := MCPProvenanceCard{
			Name:               summary.Name,
			Path:               summary.Path,
			Description:        summary.Description,
			Transport:          summary.Transport,
			Activation:         summary.Activation,
			SourcePresent:      strings.TrimSpace(summary.Source) != "",
			SourceSHA:          hashStringOrNone(summary.Source),
			CommandPresent:     summary.CommandPresent,
			CommandSHA:         mcpProvenanceHashIfPresent(summary.CommandPresent, summary.CommandSHA),
			ArgsCount:          summary.ArgsCount,
			ArgsSHA:            mcpProvenanceHashIfCount(summary.ArgsCount, summary.ArgsSHA),
			URLPresent:         summary.URLPresent,
			URLSHA:             mcpProvenanceHashIfPresent(summary.URLPresent, summary.URLSHA),
			ToolAllowlist:      append([]string(nil), summary.ToolAllowlist...),
			ToolDenylist:       append([]string(nil), summary.ToolDenylist...),
			RequiresSecretRefs: len(summary.RequiresSecrets),
			RequiresSecretsSHA: hashStringList(summary.RequiresSecrets),
			EnvPassthroughRefs: len(summary.EnvPassthrough),
			EnvPassthroughSHA:  hashStringList(summary.EnvPassthrough),
			Resources:          summary.Resources,
			Prompts:            summary.Prompts,
			Bytes:              summary.Bytes,
			Lines:              summary.Lines,
			SHA:                summary.SHA,
			ParseError:         strings.TrimSpace(summary.ParseError) != "",
			ParseErrorSHA:      hashStringOrNone(summary.ParseError),
			RiskFindings:       len(summary.RiskFindings),
			RiskMaxSeverity:    pluginRiskMaxSeverity(summary.RiskFindings),
			RiskCodes:          pluginRiskCodes(summary.RiskFindings),
			LastCommitSHA12:    "none",
			LastCommitShort:    "none",
			LastCommitDate:     "none",
			SubjectSHA12:       "none",
			CommitAvailable:    false,
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, summary.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedSpecs++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, summary.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtySpecs++
				report.addFinding("warning", "dirty_mcp_spec_file", summary.Path, "MCP spec file has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, summary.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.SpecsWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.SpecsWithoutCommits++
				report.addFinding("warning", "missing_git_history", summary.Path, "no git commit was found for this MCP spec file")
			}
		} else {
			report.UntrackedSpecs++
			detail := "MCP spec file is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_mcp_spec_file", summary.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	sort.Slice(report.Cards, func(i, j int) bool { return report.Cards[i].Path < report.Cards[j].Path })
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for MCP provenance checks")
	}
	if report.Specs > 0 && !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for repo-local MCP spec files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderMCPProvenanceCLIReport(cfg Config) string {
	return renderMCPProvenanceReport(Event{}, cfg, false)
}

func renderMCPProvenanceReport(ev Event, cfg Config, includeIssue bool) string {
	report := BuildMCPProvenanceReport(cfg)
	var b strings.Builder
	b.WriteString("## GitClaw MCP Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeMCPHeader(&b, ev, includeIssue)
	writeMCPProvenanceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps repo-local MCP spec YAML files to body-free git provenance. It reports tool filters, launch-surface hashes, risk codes, tracked state, dirty state, last commit IDs/dates, and commit-subject hashes only; raw MCP YAML, commands, args, URLs, env values, credential values, issue bodies, comments, prompts, git subjects, author identities, provider payloads, and tool output bodies are not included.\n\n")

	b.WriteString("### MCP Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeMCPProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.MCP.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", mcpProvenanceGitGate(report))
	fmt.Fprintf(&b, "- connection_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- server_launch_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- tool_exposure_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- dynamic_discovery_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hash_only")
	fmt.Fprintf(&b, "- raw_command_args_gate=`%s`\n", "hash_only")
	fmt.Fprintf(&b, "- raw_url_gate=`%s`\n", "hash_only")
	fmt.Fprintf(&b, "- credential_value_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- env_value_gate=`%s`\n", "disabled")

	b.WriteString("\n### Findings\n")
	writeMCPProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeMCPProvenanceSummary(b *strings.Builder, report MCPProvenanceReport) {
	fmt.Fprintf(b, "- mcp_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- provenance_scope: `%s`\n", "repo-local-mcp-git-history")
	fmt.Fprintf(b, "- mcp_status: `%s`\n", report.MCP.Status)
	fmt.Fprintf(b, "- mcp_specs_dir: `%s`\n", mcpSpecsDir)
	fmt.Fprintf(b, "- mcp_specs: `%d`\n", report.Specs)
	fmt.Fprintf(b, "- parsed_mcp_specs: `%d`\n", report.ParsedSpecs)
	fmt.Fprintf(b, "- mcp_specs_with_command: `%d`\n", report.SpecsWithCommand)
	fmt.Fprintf(b, "- mcp_specs_with_url: `%d`\n", report.SpecsWithURL)
	fmt.Fprintf(b, "- mcp_specs_with_tool_allowlist: `%d`\n", report.SpecsWithToolAllowlist)
	fmt.Fprintf(b, "- mcp_tool_allowlist_refs: `%d`\n", report.ToolAllowlistRefs)
	fmt.Fprintf(b, "- mcp_tool_denylist_refs: `%d`\n", report.ToolDenylistRefs)
	fmt.Fprintf(b, "- mcp_required_secret_refs: `%d`\n", report.RequiredSecretRefs)
	fmt.Fprintf(b, "- mcp_env_passthrough_refs: `%d`\n", report.EnvPassthroughRefs)
	fmt.Fprintf(b, "- mcp_specs_with_resources_enabled: `%d`\n", report.SpecsWithResourcesEnabled)
	fmt.Fprintf(b, "- mcp_specs_with_prompts_enabled: `%d`\n", report.SpecsWithPromptsEnabled)
	fmt.Fprintf(b, "- mcp_specs_with_risk_findings: `%d`\n", report.SpecsWithRiskFindings)
	fmt.Fprintf(b, "- mcp_risk_findings: `%d`\n", report.RiskFindings)
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- git_tracked_mcp_specs: `%d`\n", report.GitTrackedSpecs)
	fmt.Fprintf(b, "- untracked_mcp_specs: `%d`\n", report.UntrackedSpecs)
	fmt.Fprintf(b, "- working_tree_dirty_mcp_specs: `%d`\n", report.WorkingTreeDirtySpecs)
	fmt.Fprintf(b, "- mcp_specs_with_commits: `%d`\n", report.SpecsWithCommits)
	fmt.Fprintf(b, "- mcp_specs_without_commits: `%d`\n", report.SpecsWithoutCommits)
	fmt.Fprintf(b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(b, "- mcp_connection_supported: `%t`\n", report.MCPConnectionSupported)
	fmt.Fprintf(b, "- mcp_server_launch_allowed: `%t`\n", report.MCPServerLaunchAllowed)
	fmt.Fprintf(b, "- mcp_tool_exposure_allowed: `%t`\n", report.MCPToolExposureAllowed)
	fmt.Fprintf(b, "- dynamic_tool_discovery_allowed: `%t`\n", report.DynamicToolDiscoveryAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_mcp_bodies_included: `%t`\n", report.RawMCPBodiesIncluded)
	fmt.Fprintf(b, "- raw_command_args_included: `%t`\n", report.RawCommandArgsIncluded)
	fmt.Fprintf(b, "- raw_urls_included: `%t`\n", report.RawURLsIncluded)
	fmt.Fprintf(b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- env_values_included: `%t`\n", report.EnvValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_mcp_provenance_change: `%t`\n", report.LLME2ERequiredAfterMCPProvenanceChange)
}

func writeMCPProvenanceCard(b *strings.Builder, card MCPProvenanceCard) {
	fmt.Fprintf(
		b,
		"- mcp_name=`%s` path=`%s` transport=`%s` activation=`%s` description=`%t` source_present=`%t` source_sha256_12=`%s` command_present=`%t` command_sha256_12=`%s` args_count=`%d` args_sha256_12=`%s` url_present=`%t` url_sha256_12=`%s` tool_allowlist=`%s` tool_denylist=`%s` requires_secret_refs=`%d` requires_secrets_sha256_12=`%s` env_passthrough_refs=`%d` env_passthrough_sha256_12=`%s` resources_enabled=`%t` prompts_enabled=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` parse_error=`%t` parse_error_sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		inlineCode(card.Name),
		card.Path,
		inlineCode(card.Transport),
		inlineCode(card.Activation),
		card.Description,
		card.SourcePresent,
		card.SourceSHA,
		card.CommandPresent,
		card.CommandSHA,
		card.ArgsCount,
		card.ArgsSHA,
		card.URLPresent,
		card.URLSHA,
		inlineListOrNone(card.ToolAllowlist),
		inlineListOrNone(card.ToolDenylist),
		card.RequiresSecretRefs,
		card.RequiresSecretsSHA,
		card.EnvPassthroughRefs,
		card.EnvPassthroughSHA,
		card.Resources,
		card.Prompts,
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

func writeMCPProvenanceFindings(b *strings.Builder, findings []MCPProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func mcpProvenanceHashIfPresent(present bool, hash string) string {
	if !present {
		return "none"
	}
	if strings.TrimSpace(hash) == "" {
		return "none"
	}
	return hash
}

func mcpProvenanceHashIfCount(count int, hash string) string {
	if count == 0 {
		return "none"
	}
	if strings.TrimSpace(hash) == "" {
		return "none"
	}
	return hash
}

func mcpProvenanceBaseStatus(mcpStatus string) string {
	switch mcpStatus {
	case "high":
		return "high"
	case "warn":
		return "warn"
	case "ok":
		return "ok"
	case "":
		return "unknown"
	default:
		return mcpStatus
	}
}

func mcpProvenanceGitGate(report MCPProvenanceReport) string {
	if report.Specs == 0 {
		return "pass"
	}
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedSpecs > 0 || report.SpecsWithoutCommits > 0 || report.WorkingTreeDirtySpecs > 0 {
		return "warn"
	}
	return "pass"
}

func (r *MCPProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, MCPProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
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
