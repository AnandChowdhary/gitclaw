package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type ContextRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type ContextRiskReport struct {
	Status                               string
	VerificationScope                    string
	RunMode                              string
	Model                                string
	ContextFilesLoaded                   int
	ContextFileBytes                     int
	ContextFileLines                     int
	ContextReferences                    int
	LoadedContextReferences              int
	BlockedContextReferences             int
	FailedContextReferences              int
	FileContextReferences                int
	FolderContextReferences              int
	GitContextReferences                 int
	UnsupportedURLReferences             int
	SelectedSkills                       int
	SkillSummaries                       int
	SkillBundles                         int
	ActiveToolOutputs                    int
	PromptVisibleContextBytes            int
	MaxPromptBytes                       int
	ContextBudgetPercent                 int
	MaxTranscriptMessages                int
	MaxTranscriptMessageBytes            int
	MaxContextReferenceBytes             int
	MaxContextFolderEntries              int
	MaxContextGitCommits                 int
	MaxToolReadBytes                     int
	MaxRepoFilesListed                   int
	MaxSearchQueries                     int
	MaxSearchMatches                     int
	MaxSearchMatchesPerQuery             int
	SurfacesWithRiskFindings             int
	Findings                             []ContextRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	ContextFileBodiesIncluded            bool
	ContextReferenceBodiesIncluded       bool
	SkillBodiesIncluded                  bool
	ToolOutputBodiesIncluded             bool
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	RawInputsIncluded                    bool
	ExternalURLFetchSupported            bool
	RepositoryMutationAllowed            bool
	HostExecAllowed                      bool
	LLME2ERequiredAfterContextRiskChange bool
}

type contextRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var contextRiskRules = []contextRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"system prompt override",
			"reveal the system prompt",
			"show the system prompt",
			"developer message",
		},
	},
	{
		Severity: "high",
		Code:     "credential_exfiltration_instruction",
		Category: "data-exfiltration",
		Any: []string{
			"exfiltrate",
			"leak secrets",
			"send secrets",
			"upload secrets",
			"steal secrets",
			"cat .env",
		},
	},
	{
		Severity: "warning",
		Code:     "hidden_instruction",
		Category: "prompt-boundary",
		Any: []string{
			"do not tell the user",
			"display:none",
			"visibility:hidden",
			"<!-- ignore",
			"<!-- system",
		},
	},
	{
		Severity: "warning",
		Code:     "unbounded_context_growth",
		Category: "context-budget",
		Any: []string{
			"load everything",
			"read every file",
			"entire repository",
			"no context limit",
			"ignore context limit",
		},
	},
	{
		Severity: "warning",
		Code:     "host_execution_instruction",
		Category: "host-execution",
		Any: []string{
			"rm -rf",
			"bash -c",
			"python -c",
			"curl ",
			"wget ",
			"execute shell",
			"run shell",
		},
		IgnoreAny: []string{
			"do not claim",
		},
	},
	{
		Severity: "info",
		Code:     "credential_transfer_instruction",
		Category: "credential-handling",
		Any: []string{
			"api key",
			"private key",
			"github_token",
			"github token",
		},
		All: []string{
			"send",
			"upload",
			"post",
			"copy",
		},
	},
}

func renderContextRiskReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildContextRiskReport(cfg, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Context Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeContextRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report risk-audits GitClaw's prompt-visible context boundary: repo-local context files, explicit context references, selected skills, deterministic tool outputs, prompt budgets, and external-fetch/write/exec gates. It reports metadata, counts, hashes, risk codes, and severities only; context bodies, reference bodies, skill bodies, tool outputs, issue bodies, comments, prompts, raw tool inputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Context Budget Risk Card\n")
	writeContextBudgetRiskCard(&b, report)

	b.WriteString("\n### Context File Risk Cards\n")
	writeContextFileRiskCards(&b, repoContext.Documents, report)

	b.WriteString("\n### Context Reference Risk Cards\n")
	writeContextReferenceRiskCards(&b, repoContext.ContextReferences, report)

	b.WriteString("\n### Selected Skill Risk Cards\n")
	writeContextSkillRiskCards(&b, repoContext.Skills, report)

	b.WriteString("\n### Tool Output Risk Cards\n")
	writeContextToolOutputRiskCards(&b, repoContext.ToolOutputs, report)

	b.WriteString("\n### Runtime Boundary Risk Card\n")
	writeContextRuntimeBoundaryRiskCard(&b, report)

	b.WriteString("\n### Risk Findings\n")
	writeContextRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildContextRiskReport(cfg Config, transcript []TranscriptMessage, repoContext RepoContext) ContextRiskReport {
	contextBytes, contextLines := contextDocumentTotals(repoContext.Documents)
	promptVisibleBytes := contextBytes + contextDocumentBytes(repoContext.Skills) + toolOutputBytes(repoContext.ToolOutputs)
	report := ContextRiskReport{
		Status:                               "ok",
		VerificationScope:                    "context-files-references-skills-tools-and-prompt-boundary",
		RunMode:                              "read-only",
		Model:                                cfg.Model,
		ContextFilesLoaded:                   len(repoContext.Documents),
		ContextFileBytes:                     contextBytes,
		ContextFileLines:                     contextLines,
		ContextReferences:                    len(repoContext.ContextReferences),
		LoadedContextReferences:              loadedContextReferenceCount(repoContext.ContextReferences),
		BlockedContextReferences:             contextReferenceStatusCount(repoContext.ContextReferences, "blocked"),
		FailedContextReferences:              failedContextReferenceCount(repoContext.ContextReferences),
		FileContextReferences:                contextReferenceKindCount(repoContext.ContextReferences, "file"),
		FolderContextReferences:              contextReferenceKindCount(repoContext.ContextReferences, "folder"),
		GitContextReferences:                 contextReferenceKindCount(repoContext.ContextReferences, "git") + contextReferenceKindCount(repoContext.ContextReferences, "diff") + contextReferenceKindCount(repoContext.ContextReferences, "staged"),
		UnsupportedURLReferences:             unsupportedURLReferenceCount(transcript),
		SelectedSkills:                       len(repoContext.Skills),
		SkillSummaries:                       len(repoContext.SkillSummaries),
		SkillBundles:                         len(repoContext.SkillBundles),
		ActiveToolOutputs:                    len(repoContext.ToolOutputs),
		PromptVisibleContextBytes:            promptVisibleBytes,
		MaxPromptBytes:                       cfg.MaxPromptBytes,
		ContextBudgetPercent:                 percentOf(promptVisibleBytes, cfg.MaxPromptBytes),
		MaxTranscriptMessages:                cfg.MaxTranscriptMessages,
		MaxTranscriptMessageBytes:            cfg.MaxTranscriptMessageBytes,
		MaxContextReferenceBytes:             maxContextReferenceBytes,
		MaxContextFolderEntries:              maxContextFolderEntries,
		MaxContextGitCommits:                 maxContextGitCommits,
		MaxToolReadBytes:                     maxToolReadBytes,
		MaxRepoFilesListed:                   maxRepoFilesListed,
		MaxSearchQueries:                     maxSearchQueries,
		MaxSearchMatches:                     maxSearchMatches,
		MaxSearchMatchesPerQuery:             maxSearchMatchesPerQuery,
		ContextFileBodiesIncluded:            false,
		ContextReferenceBodiesIncluded:       false,
		SkillBodiesIncluded:                  false,
		ToolOutputBodiesIncluded:             false,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		RawInputsIncluded:                    false,
		ExternalURLFetchSupported:            false,
		RepositoryMutationAllowed:            false,
		HostExecAllowed:                      false,
		LLME2ERequiredAfterContextRiskChange: true,
	}
	report.Findings = buildContextRiskFindings(report, repoContext)
	sortContextRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = contextRiskSurfaceCount(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func buildContextRiskFindings(report ContextRiskReport, repoContext RepoContext) []ContextRiskFinding {
	var findings []ContextRiskFinding
	if report.ContextFilesLoaded == 0 {
		findings = append(findings, contextRiskMetadataFinding("warning", "context_files_missing", "context-presence", "context-budget", "context", "documents"))
	}
	if report.MaxPromptBytes <= 0 {
		findings = append(findings, contextRiskMetadataFinding("high", "max_prompt_bytes_not_positive", "context-budget", "context-budget", "context", "max_prompt_bytes"))
	} else if report.PromptVisibleContextBytes > report.MaxPromptBytes {
		findings = append(findings, contextRiskMetadataFinding("high", "prompt_visible_context_exceeds_budget", "context-budget", "context-budget", "context", "max_prompt_bytes"))
	} else if report.ContextBudgetPercent >= 80 {
		findings = append(findings, contextRiskMetadataFinding("warning", "prompt_visible_context_budget_high", "context-budget", "context-budget", "context", "max_prompt_bytes"))
	}
	if report.MaxTranscriptMessages <= 0 {
		findings = append(findings, contextRiskMetadataFinding("high", "max_transcript_messages_not_positive", "transcript-budget", "context-budget", "context", "max_transcript_messages"))
	}
	if report.MaxTranscriptMessageBytes <= 0 {
		findings = append(findings, contextRiskMetadataFinding("high", "max_transcript_message_bytes_not_positive", "transcript-budget", "context-budget", "context", "max_transcript_message_bytes"))
	}
	if report.UnsupportedURLReferences > 0 {
		findings = append(findings, contextRiskMetadataFinding("info", "url_context_reference_unsupported", "external-fetch", "runtime-boundary", "context", "external_url_fetch"))
	}
	for _, ref := range repoContext.ContextReferences {
		switch ref.Status {
		case "ok":
		case "blocked":
			findings = append(findings, contextRiskFindingForReference("info", "context_reference_blocked", "reference-boundary", ref))
		case "empty":
			findings = append(findings, contextRiskFindingForReference("info", "context_reference_empty", "reference-boundary", ref))
		default:
			findings = append(findings, contextRiskFindingForReference("warning", "context_reference_unloaded", "reference-boundary", ref))
		}
	}
	for _, doc := range repoContext.Documents {
		findings = append(findings, scanContextRiskText("context-file", doc.Path, doc.Body)...)
	}
	for _, skill := range repoContext.Skills {
		findings = append(findings, scanContextRiskText("selected-skill", skill.Path, skill.Body)...)
	}
	for _, output := range repoContext.ToolOutputs {
		findings = append(findings, scanContextRiskText("tool-output", output.Name, output.Output)...)
	}
	sortContextRiskFindings(findings)
	return findings
}

func scanContextRiskText(kind, path, body string) []ContextRiskFinding {
	var findings []ContextRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range contextRiskRules {
			if !contextRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, ContextRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Path:     path,
				Field:    "body",
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortContextRiskFindings(findings)
	return findings
}

func contextRiskRuleMatches(lowerLine string, rule contextRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerLine, ignored) {
			return false
		}
	}
	for _, required := range rule.All {
		if !strings.Contains(lowerLine, required) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func contextRiskMetadataFinding(severity, code, category, kind, path, field string) ContextRiskFinding {
	return ContextRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Path:     path,
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + path + ":" + field + ":" + code),
	}
}

func contextRiskFindingForReference(severity, code, category string, ref ContextReferenceSummary) ContextRiskFinding {
	field := ref.Status
	if ref.Reason != "" {
		field += ":" + ref.Reason
	}
	return ContextRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "context-reference",
		Path:     ref.Kind + ":" + ref.Path,
		Field:    field,
		LineSHA:  shortDocumentHash(ref.Kind + ":" + ref.Path + ":" + field),
	}
}

func writeContextRiskSummary(b *strings.Builder, report ContextRiskReport) {
	fmt.Fprintf(b, "- context_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- run_mode: `%s`\n", report.RunMode)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- context_files_loaded: `%d`\n", report.ContextFilesLoaded)
	fmt.Fprintf(b, "- context_file_bytes: `%d`\n", report.ContextFileBytes)
	fmt.Fprintf(b, "- context_file_lines: `%d`\n", report.ContextFileLines)
	fmt.Fprintf(b, "- context_references: `%d`\n", report.ContextReferences)
	fmt.Fprintf(b, "- loaded_context_references: `%d`\n", report.LoadedContextReferences)
	fmt.Fprintf(b, "- blocked_context_references: `%d`\n", report.BlockedContextReferences)
	fmt.Fprintf(b, "- failed_context_references: `%d`\n", report.FailedContextReferences)
	fmt.Fprintf(b, "- file_context_references: `%d`\n", report.FileContextReferences)
	fmt.Fprintf(b, "- folder_context_references: `%d`\n", report.FolderContextReferences)
	fmt.Fprintf(b, "- git_context_references: `%d`\n", report.GitContextReferences)
	fmt.Fprintf(b, "- unsupported_url_references: `%d`\n", report.UnsupportedURLReferences)
	fmt.Fprintf(b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(b, "- skill_summaries: `%d`\n", report.SkillSummaries)
	fmt.Fprintf(b, "- skill_bundles: `%d`\n", report.SkillBundles)
	fmt.Fprintf(b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(b, "- prompt_visible_context_bytes: `%d`\n", report.PromptVisibleContextBytes)
	fmt.Fprintf(b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(b, "- context_budget_percent: `%d`\n", report.ContextBudgetPercent)
	fmt.Fprintf(b, "- max_transcript_messages: `%d`\n", report.MaxTranscriptMessages)
	fmt.Fprintf(b, "- max_transcript_message_bytes: `%d`\n", report.MaxTranscriptMessageBytes)
	fmt.Fprintf(b, "- max_context_reference_bytes: `%d`\n", report.MaxContextReferenceBytes)
	fmt.Fprintf(b, "- max_context_folder_entries: `%d`\n", report.MaxContextFolderEntries)
	fmt.Fprintf(b, "- max_context_git_commits: `%d`\n", report.MaxContextGitCommits)
	fmt.Fprintf(b, "- max_tool_read_bytes: `%d`\n", report.MaxToolReadBytes)
	fmt.Fprintf(b, "- max_repo_files_listed: `%d`\n", report.MaxRepoFilesListed)
	fmt.Fprintf(b, "- max_search_queries: `%d`\n", report.MaxSearchQueries)
	fmt.Fprintf(b, "- max_search_matches: `%d`\n", report.MaxSearchMatches)
	fmt.Fprintf(b, "- max_search_matches_per_query: `%d`\n", report.MaxSearchMatchesPerQuery)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- context_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- context_file_bodies_included: `%t`\n", report.ContextFileBodiesIncluded)
	fmt.Fprintf(b, "- context_reference_bodies_included: `%t`\n", report.ContextReferenceBodiesIncluded)
	fmt.Fprintf(b, "- skill_bodies_included: `%t`\n", report.SkillBodiesIncluded)
	fmt.Fprintf(b, "- tool_output_bodies_included: `%t`\n", report.ToolOutputBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	fmt.Fprintf(b, "- external_url_fetch_supported: `%t`\n", report.ExternalURLFetchSupported)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- host_exec_allowed: `%t`\n", report.HostExecAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_context_risk_change: `%t`\n", report.LLME2ERequiredAfterContextRiskChange)
}

func writeContextBudgetRiskCard(b *strings.Builder, report ContextRiskReport) {
	findings := contextRiskFindingsByKind(report.Findings, "context-budget")
	fmt.Fprintf(
		b,
		"- kind=`context-budget` prompt_visible_context_bytes=`%d` max_prompt_bytes=`%d` context_budget_percent=`%d` max_transcript_messages=`%d` max_transcript_message_bytes=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.PromptVisibleContextBytes,
		report.MaxPromptBytes,
		report.ContextBudgetPercent,
		report.MaxTranscriptMessages,
		report.MaxTranscriptMessageBytes,
		len(findings),
		contextRiskMaxSeverity(findings),
		inlineListOrNone(contextRiskCodes(findings)),
		inlineListOrNone(contextRiskLineHashes(findings)),
	)
}

func writeContextFileRiskCards(b *strings.Builder, docs []ContextDocument, report ContextRiskReport) {
	if len(docs) == 0 {
		findings := contextRiskFindingsByKind(report.Findings, "context-file")
		fmt.Fprintf(b, "- kind=`context-file` none risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n", len(findings), contextRiskMaxSeverity(findings), inlineListOrNone(contextRiskCodes(findings)), inlineListOrNone(contextRiskLineHashes(findings)))
		return
	}
	for _, doc := range docs {
		findings := contextRiskFindingsByKindPath(report.Findings, "context-file", doc.Path)
		fmt.Fprintf(
			b,
			"- kind=`context-file` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			doc.Path,
			len(doc.Body),
			lineCount(doc.Body),
			shortDocumentHash(doc.Body),
			len(findings),
			contextRiskMaxSeverity(findings),
			inlineListOrNone(contextRiskCodes(findings)),
			inlineListOrNone(contextRiskLineHashes(findings)),
		)
	}
}

func writeContextReferenceRiskCards(b *strings.Builder, refs []ContextReferenceSummary, report ContextRiskReport) {
	if len(refs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, ref := range refs {
		findings := contextRiskFindingsByKindPath(report.Findings, "context-reference", ref.Kind+":"+ref.Path)
		fmt.Fprintf(
			b,
			"- kind=`context-reference` ref_kind=`%s` path=`%s` range=`%s` count=`%d` status=`%s` reason=`%s` bytes=`%d` lines=`%d` entries=`%d` sha256_12=`%s` body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			ref.Kind,
			ref.Path,
			inlineCode(ref.LineRange),
			ref.Count,
			ref.Status,
			inlineCode(ref.Reason),
			ref.Bytes,
			ref.Lines,
			ref.Entries,
			ref.SHA,
			len(findings),
			contextRiskMaxSeverity(findings),
			inlineListOrNone(contextRiskCodes(findings)),
			inlineListOrNone(contextRiskLineHashes(findings)),
		)
	}
}

func writeContextSkillRiskCards(b *strings.Builder, skills []ContextDocument, report ContextRiskReport) {
	if len(skills) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, skill := range skills {
		findings := contextRiskFindingsByKindPath(report.Findings, "selected-skill", skill.Path)
		fmt.Fprintf(
			b,
			"- kind=`selected-skill` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			skill.Path,
			len(skill.Body),
			lineCount(skill.Body),
			shortDocumentHash(skill.Body),
			len(findings),
			contextRiskMaxSeverity(findings),
			inlineListOrNone(contextRiskCodes(findings)),
			inlineListOrNone(contextRiskLineHashes(findings)),
		)
	}
}

func writeContextToolOutputRiskCards(b *strings.Builder, outputs []ToolOutput, report ContextRiskReport) {
	if len(outputs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, output := range outputs {
		findings := contextRiskFindingsByKindPath(report.Findings, "tool-output", output.Name)
		fmt.Fprintf(
			b,
			"- kind=`tool-output` name=`%s` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` input_included=`false` output_body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			output.Name,
			shortDocumentHash(output.Input),
			len(output.Output),
			lineCount(output.Output),
			shortDocumentHash(output.Output),
			len(findings),
			contextRiskMaxSeverity(findings),
			inlineListOrNone(contextRiskCodes(findings)),
			inlineListOrNone(contextRiskLineHashes(findings)),
		)
	}
}

func writeContextRuntimeBoundaryRiskCard(b *strings.Builder, report ContextRiskReport) {
	findings := contextRiskFindingsByKind(report.Findings, "runtime-boundary")
	fmt.Fprintf(
		b,
		"- kind=`runtime-boundary` external_url_fetch_supported=`%t` repository_mutation_allowed=`%t` host_exec_allowed=`%t` context_file_bodies_included=`%t` context_reference_bodies_included=`%t` skill_bodies_included=`%t` tool_output_bodies_included=`%t` raw_issue_bodies_included=`%t` raw_comment_bodies_included=`%t` raw_inputs_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.ExternalURLFetchSupported,
		report.RepositoryMutationAllowed,
		report.HostExecAllowed,
		report.ContextFileBodiesIncluded,
		report.ContextReferenceBodiesIncluded,
		report.SkillBodiesIncluded,
		report.ToolOutputBodiesIncluded,
		report.RawIssueBodiesIncluded,
		report.RawCommentBodiesIncluded,
		report.RawInputsIncluded,
		len(findings),
		contextRiskMaxSeverity(findings),
		inlineListOrNone(contextRiskCodes(findings)),
		inlineListOrNone(contextRiskLineHashes(findings)),
	)
}

func writeContextRiskFindings(b *strings.Builder, findings []ContextRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` category=`%s` kind=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			finding.Kind,
			inlineCode(finding.Path),
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func contextDocumentTotals(docs []ContextDocument) (bytes int, lines int) {
	for _, doc := range docs {
		bytes += len(doc.Body)
		lines += lineCount(doc.Body)
	}
	return bytes, lines
}

func contextDocumentBytes(docs []ContextDocument) int {
	total := 0
	for _, doc := range docs {
		total += len(doc.Body)
	}
	return total
}

func toolOutputBytes(outputs []ToolOutput) int {
	total := 0
	for _, output := range outputs {
		total += len(output.Output)
	}
	return total
}

func percentOf(value, total int) int {
	if total <= 0 {
		return 0
	}
	return (value * 100) / total
}

func contextReferenceStatusCount(refs []ContextReferenceSummary, status string) int {
	count := 0
	for _, ref := range refs {
		if ref.Status == status {
			count++
		}
	}
	return count
}

func failedContextReferenceCount(refs []ContextReferenceSummary) int {
	count := 0
	for _, ref := range refs {
		switch ref.Status {
		case "ok", "blocked", "empty":
		default:
			count++
		}
	}
	return count
}

func contextReferenceKindCount(refs []ContextReferenceSummary, kind string) int {
	count := 0
	for _, ref := range refs {
		if ref.Kind == kind {
			count++
		}
	}
	return count
}

func unsupportedURLReferenceCount(transcript []TranscriptMessage) int {
	count := 0
	seen := map[string]bool{}
	for _, field := range strings.Fields(transcriptText(transcript)) {
		token := strings.Trim(strings.TrimSpace(field), "`\"'()[]{}<>")
		token = strings.TrimRight(token, ".,;!?")
		if !strings.HasPrefix(strings.ToLower(token), "@url:") {
			continue
		}
		lower := strings.ToLower(token)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		count++
	}
	return count
}

func contextRiskFindingsByKind(findings []ContextRiskFinding, kind string) []ContextRiskFinding {
	var filtered []ContextRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			filtered = append(filtered, finding)
		}
	}
	sortContextRiskFindings(filtered)
	return filtered
}

func contextRiskFindingsByKindPath(findings []ContextRiskFinding, kind, path string) []ContextRiskFinding {
	var filtered []ContextRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind && finding.Path == path {
			filtered = append(filtered, finding)
		}
	}
	sortContextRiskFindings(filtered)
	return filtered
}

func contextRiskSurfaceCount(findings []ContextRiskFinding) int {
	surfaces := map[string]bool{}
	for _, finding := range findings {
		surfaces[finding.Kind+"\x00"+finding.Path] = true
	}
	return len(surfaces)
}

func contextRiskCodes(findings []ContextRiskFinding) []string {
	var codes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	return sortedStrings(codes)
}

func contextRiskLineHashes(findings []ContextRiskFinding) []string {
	var hashes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	return sortedStrings(hashes)
}

func contextRiskMaxSeverity(findings []ContextRiskFinding) string {
	if len(findings) == 0 {
		return "none"
	}
	max := "info"
	for _, finding := range findings {
		switch finding.Severity {
		case "high":
			return "high"
		case "warning":
			max = "warning"
		}
	}
	return max
}

func sortContextRiskFindings(findings []ContextRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		if left.Severity != right.Severity {
			return contextRiskSeverityRank(left.Severity) < contextRiskSeverityRank(right.Severity)
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Field < right.Field
	})
}

func contextRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}
