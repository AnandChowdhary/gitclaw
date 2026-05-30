package gitclaw

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type PromptRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type PromptRiskReport struct {
	Status                              string
	VerificationScope                   string
	RunMode                             string
	Provider                            string
	Model                               string
	SystemPromptBytes                   int
	SystemPromptSHA                     string
	PromptBytes                         int
	PromptLines                         int
	PromptSHA                           string
	MaxPromptBytes                      int
	PromptBudgetPercent                 int
	MaxOutputTokens                     int
	MaxTranscriptMessages               int
	MaxTranscriptMessageBytes           int
	TranscriptMessages                  int
	BoundedTranscriptMessages           int
	OmittedOlderMessages                int
	TruncatedTranscriptBodies           int
	PromptContainsTruncation            bool
	ContextFiles                        int
	ContextFileBytes                    int
	ContextReferences                   int
	SelectedSkills                      int
	AvailableSkills                     int
	SkillBytes                          int
	ToolOutputs                         int
	ToolOutputBytes                     int
	PromptArtifactEnabled               bool
	PromptArtifactRedactionPatterns     int
	SurfacesWithRiskFindings            int
	Findings                            []PromptRiskFinding
	HighRiskFindings                    int
	WarningRiskFindings                 int
	InfoRiskFindings                    int
	PromptBodyIncluded                  bool
	ContextFileBodiesIncluded           bool
	ContextReferenceBodiesIncluded      bool
	SkillBodiesIncluded                 bool
	ToolOutputBodiesIncluded            bool
	RawIssueBodiesIncluded              bool
	RawCommentBodiesIncluded            bool
	RawInputsIncluded                   bool
	CredentialValuesIncluded            bool
	RepositoryMutationAllowed           bool
	HostExecAllowed                     bool
	LLME2ERequiredAfterPromptRiskChange bool
}

type promptRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var promptRiskRules = []promptRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
			"system prompt override",
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
		Code:     "unbounded_context_request",
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

func RenderPromptRiskReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderPromptRiskReport(ev, cfg, transcript, repoContext, true)
}

func RenderPromptRiskCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPromptRiskReport(Event{}, cfg, nil, repoContext, false)
}

func renderPromptRiskReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	report := BuildPromptRiskReport(ev, cfg, transcript, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Prompt Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writePromptRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report risk-audits GitClaw's assembled prompt envelope: prompt budget, transcript bounding, context contributors, selected skills, deterministic tool outputs, prompt artifact controls, and prompt-boundary text patterns. It reports metadata, counts, hashes, risk codes, and severities only; prompt bodies, issue bodies, comments, context bodies, skill bodies, tool outputs, raw tool inputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Prompt Budget Risk Card\n")
	writePromptBudgetRiskCard(&b, report)

	b.WriteString("\n### Transcript Risk Card\n")
	writePromptTranscriptRiskCard(&b, report)

	b.WriteString("\n### Context Contributor Risk Cards\n")
	writePromptDocumentRiskCards(&b, "context-file", repoContext.Documents, report)

	b.WriteString("\n### Selected Skill Risk Cards\n")
	writePromptDocumentRiskCards(&b, "selected-skill", repoContext.Skills, report)

	b.WriteString("\n### Tool Output Risk Cards\n")
	writePromptToolOutputRiskCards(&b, repoContext.ToolOutputs, report)

	b.WriteString("\n### Runtime Boundary Risk Card\n")
	writePromptRuntimeRiskCard(&b, report)

	b.WriteString("\n### Risk Findings\n")
	writePromptRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildPromptRiskReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) PromptRiskReport {
	req := LLMRequest{
		Event:      ev,
		Transcript: transcript,
		Context:    repoContext,
		Config:     cfg,
	}
	profile := buildPromptReportProfile(req)
	contextBytes, _ := contextDocumentTotals(repoContext.Documents)
	skillBytes := contextDocumentBytes(repoContext.Skills)
	toolBytes := toolOutputBytes(repoContext.ToolOutputs)
	report := PromptRiskReport{
		Status:                              "ok",
		VerificationScope:                   "prompt-budget-transcript-context-skills-tools-and-artifact-boundary",
		RunMode:                             "read-only",
		Provider:                            cfg.ModelProvider,
		Model:                               cfg.Model,
		SystemPromptBytes:                   len(systemPrompt),
		SystemPromptSHA:                     shortDocumentHash(systemPrompt),
		PromptBytes:                         len(profile.Prompt),
		PromptLines:                         lineCount(profile.Prompt),
		PromptSHA:                           shortDocumentHash(profile.Prompt),
		MaxPromptBytes:                      profile.Budget.MaxPromptBytes,
		PromptBudgetPercent:                 percentOf(len(profile.Prompt), profile.Budget.MaxPromptBytes),
		MaxOutputTokens:                     cfg.MaxOutputTokens,
		MaxTranscriptMessages:               profile.Budget.MaxTranscriptMessages,
		MaxTranscriptMessageBytes:           profile.Budget.MaxTranscriptMessageBytes,
		TranscriptMessages:                  len(transcript),
		BoundedTranscriptMessages:           profile.BoundedTranscriptMessages,
		OmittedOlderMessages:                profile.OmittedOlderMessages,
		TruncatedTranscriptBodies:           profile.TruncatedTranscriptBodies,
		PromptContainsTruncation:            profile.PromptContainsTruncation,
		ContextFiles:                        len(repoContext.Documents),
		ContextFileBytes:                    contextBytes,
		ContextReferences:                   len(repoContext.ContextReferences),
		SelectedSkills:                      len(repoContext.Skills),
		AvailableSkills:                     availableSkillCount(repoContext),
		SkillBytes:                          skillBytes,
		ToolOutputs:                         len(repoContext.ToolOutputs),
		ToolOutputBytes:                     toolBytes,
		PromptArtifactEnabled:               strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "",
		PromptArtifactRedactionPatterns:     len(promptArtifactRedactions),
		PromptBodyIncluded:                  false,
		ContextFileBodiesIncluded:           false,
		ContextReferenceBodiesIncluded:      false,
		SkillBodiesIncluded:                 false,
		ToolOutputBodiesIncluded:            false,
		RawIssueBodiesIncluded:              false,
		RawCommentBodiesIncluded:            false,
		RawInputsIncluded:                   false,
		CredentialValuesIncluded:            false,
		RepositoryMutationAllowed:           false,
		HostExecAllowed:                     false,
		LLME2ERequiredAfterPromptRiskChange: true,
	}
	report.Findings = append(report.Findings, promptRiskMetadataFindings(report)...)
	report.Findings = append(report.Findings, promptRiskTranscriptFindings(transcript, profile.Budget)...)
	report.Findings = append(report.Findings, promptRiskDocumentFindings("context-file", repoContext.Documents)...)
	report.Findings = append(report.Findings, promptRiskDocumentFindings("selected-skill", repoContext.Skills)...)
	report.Findings = append(report.Findings, promptRiskToolOutputFindings(repoContext.ToolOutputs)...)
	sortPromptRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = promptRiskSurfaceCount(report.Findings)
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

func promptRiskMetadataFindings(report PromptRiskReport) []PromptRiskFinding {
	var findings []PromptRiskFinding
	if report.MaxPromptBytes <= 0 {
		findings = append(findings, promptRiskMetadataFinding("high", "max_prompt_bytes_not_positive", "prompt-budget", "prompt-budget", "prompt", "max_prompt_bytes"))
	} else if report.PromptBytes > report.MaxPromptBytes {
		findings = append(findings, promptRiskMetadataFinding("high", "prompt_exceeds_budget", "prompt-budget", "prompt-budget", "prompt", "max_prompt_bytes"))
	} else if report.PromptBudgetPercent >= 80 {
		findings = append(findings, promptRiskMetadataFinding("warning", "prompt_budget_high", "prompt-budget", "prompt-budget", "prompt", "max_prompt_bytes"))
	}
	if report.MaxTranscriptMessages <= 0 {
		findings = append(findings, promptRiskMetadataFinding("high", "max_transcript_messages_not_positive", "transcript-budget", "transcript", "transcript", "max_transcript_messages"))
	}
	if report.MaxTranscriptMessageBytes <= 0 {
		findings = append(findings, promptRiskMetadataFinding("high", "max_transcript_message_bytes_not_positive", "transcript-budget", "transcript", "transcript", "max_transcript_message_bytes"))
	}
	if report.OmittedOlderMessages > 0 {
		findings = append(findings, promptRiskMetadataFinding("info", "older_transcript_messages_omitted", "transcript-budget", "transcript", "transcript", "omitted_older_messages"))
	}
	if report.TruncatedTranscriptBodies > 0 || report.PromptContainsTruncation {
		findings = append(findings, promptRiskMetadataFinding("warning", "transcript_body_truncated", "transcript-budget", "transcript", "transcript", "max_transcript_message_bytes"))
	}
	if report.SelectedSkills > 5 {
		findings = append(findings, promptRiskMetadataFinding("info", "many_selected_skills", "skill-context", "selected-skill", "skills", "selected_skills"))
	}
	sortPromptRiskFindings(findings)
	return findings
}

func writePromptRiskSummary(b *strings.Builder, report PromptRiskReport) {
	fmt.Fprintf(b, "- prompt_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- run_mode: `%s`\n", report.RunMode)
	fmt.Fprintf(b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(b, "- system_prompt_bytes: `%d`\n", report.SystemPromptBytes)
	fmt.Fprintf(b, "- system_prompt_sha256_12: `%s`\n", report.SystemPromptSHA)
	fmt.Fprintf(b, "- prompt_bytes: `%d`\n", report.PromptBytes)
	fmt.Fprintf(b, "- prompt_lines: `%d`\n", report.PromptLines)
	fmt.Fprintf(b, "- prompt_sha256_12: `%s`\n", report.PromptSHA)
	fmt.Fprintf(b, "- max_prompt_bytes: `%d`\n", report.MaxPromptBytes)
	fmt.Fprintf(b, "- prompt_budget_percent: `%d`\n", report.PromptBudgetPercent)
	fmt.Fprintf(b, "- max_output_tokens: `%d`\n", report.MaxOutputTokens)
	fmt.Fprintf(b, "- max_transcript_messages: `%d`\n", report.MaxTranscriptMessages)
	fmt.Fprintf(b, "- max_transcript_message_bytes: `%d`\n", report.MaxTranscriptMessageBytes)
	fmt.Fprintf(b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(b, "- bounded_transcript_messages: `%d`\n", report.BoundedTranscriptMessages)
	fmt.Fprintf(b, "- omitted_older_messages: `%d`\n", report.OmittedOlderMessages)
	fmt.Fprintf(b, "- truncated_transcript_bodies: `%d`\n", report.TruncatedTranscriptBodies)
	fmt.Fprintf(b, "- prompt_contains_truncation_marker: `%t`\n", report.PromptContainsTruncation)
	fmt.Fprintf(b, "- context_files: `%d`\n", report.ContextFiles)
	fmt.Fprintf(b, "- context_file_bytes: `%d`\n", report.ContextFileBytes)
	fmt.Fprintf(b, "- context_references: `%d`\n", report.ContextReferences)
	fmt.Fprintf(b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(b, "- skill_bytes: `%d`\n", report.SkillBytes)
	fmt.Fprintf(b, "- tool_outputs: `%d`\n", report.ToolOutputs)
	fmt.Fprintf(b, "- tool_output_bytes: `%d`\n", report.ToolOutputBytes)
	fmt.Fprintf(b, "- prompt_artifact_enabled: `%t`\n", report.PromptArtifactEnabled)
	fmt.Fprintf(b, "- prompt_artifact_redaction_patterns: `%d`\n", report.PromptArtifactRedactionPatterns)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- prompt_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- prompt_body_included: `%t`\n", report.PromptBodyIncluded)
	fmt.Fprintf(b, "- context_file_bodies_included: `%t`\n", report.ContextFileBodiesIncluded)
	fmt.Fprintf(b, "- context_reference_bodies_included: `%t`\n", report.ContextReferenceBodiesIncluded)
	fmt.Fprintf(b, "- skill_bodies_included: `%t`\n", report.SkillBodiesIncluded)
	fmt.Fprintf(b, "- tool_output_bodies_included: `%t`\n", report.ToolOutputBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_inputs_included: `%t`\n", report.RawInputsIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- host_exec_allowed: `%t`\n", report.HostExecAllowed)
	fmt.Fprintf(b, "- llm_e2e_required_after_prompt_risk_change: `%t`\n", report.LLME2ERequiredAfterPromptRiskChange)
}

func writePromptBudgetRiskCard(b *strings.Builder, report PromptRiskReport) {
	findings := promptRiskFindingsByKind(report.Findings, "prompt-budget")
	fmt.Fprintf(
		b,
		"- kind=`prompt-budget` prompt_bytes=`%d` max_prompt_bytes=`%d` prompt_budget_percent=`%d` system_prompt_bytes=`%d` max_output_tokens=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.PromptBytes,
		report.MaxPromptBytes,
		report.PromptBudgetPercent,
		report.SystemPromptBytes,
		report.MaxOutputTokens,
		len(findings),
		promptRiskMaxSeverity(findings),
		inlineListOrNone(promptRiskCodes(findings)),
		inlineListOrNone(promptRiskLineHashes(findings)),
	)
}

func writePromptTranscriptRiskCard(b *strings.Builder, report PromptRiskReport) {
	findings := promptRiskFindingsByKind(report.Findings, "transcript")
	fmt.Fprintf(
		b,
		"- kind=`transcript` transcript_messages=`%d` bounded_transcript_messages=`%d` omitted_older_messages=`%d` truncated_transcript_bodies=`%d` prompt_contains_truncation_marker=`%t` max_transcript_messages=`%d` max_transcript_message_bytes=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		report.TranscriptMessages,
		report.BoundedTranscriptMessages,
		report.OmittedOlderMessages,
		report.TruncatedTranscriptBodies,
		report.PromptContainsTruncation,
		report.MaxTranscriptMessages,
		report.MaxTranscriptMessageBytes,
		len(findings),
		promptRiskMaxSeverity(findings),
		inlineListOrNone(promptRiskCodes(findings)),
		inlineListOrNone(promptRiskLineHashes(findings)),
	)
}

func writePromptDocumentRiskCards(b *strings.Builder, kind string, docs []ContextDocument, report PromptRiskReport) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		findings := promptRiskFindingsByKindPath(report.Findings, kind, doc.Path)
		fmt.Fprintf(
			b,
			"- kind=`%s` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			kind,
			doc.Path,
			len(doc.Body),
			lineCount(doc.Body),
			shortDocumentHash(doc.Body),
			len(findings),
			promptRiskMaxSeverity(findings),
			inlineListOrNone(promptRiskCodes(findings)),
			inlineListOrNone(promptRiskLineHashes(findings)),
		)
	}
}

func writePromptToolOutputRiskCards(b *strings.Builder, outputs []ToolOutput, report PromptRiskReport) {
	if len(outputs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, output := range outputs {
		findings := promptRiskFindingsByKindPath(report.Findings, "tool-output", output.Name)
		fmt.Fprintf(
			b,
			"- kind=`tool-output` name=`%s` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` input_included=`false` output_body_included=`false` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
			output.Name,
			shortDocumentHash(output.Input),
			len(output.Output),
			lineCount(output.Output),
			shortDocumentHash(output.Output),
			len(findings),
			promptRiskMaxSeverity(findings),
			inlineListOrNone(promptRiskCodes(findings)),
			inlineListOrNone(promptRiskLineHashes(findings)),
		)
	}
}

func writePromptRuntimeRiskCard(b *strings.Builder, report PromptRiskReport) {
	fmt.Fprintf(
		b,
		"- kind=`runtime-boundary` prompt_artifact_enabled=`%t` prompt_artifact_redaction_patterns=`%d` prompt_body_included=`%t` context_file_bodies_included=`%t` context_reference_bodies_included=`%t` skill_bodies_included=`%t` tool_output_bodies_included=`%t` raw_issue_bodies_included=`%t` raw_comment_bodies_included=`%t` raw_inputs_included=`%t` credential_values_included=`%t` repository_mutation_allowed=`%t` host_exec_allowed=`%t` risk_findings=`0` risk_max_severity=`none` risk_codes=`none` line_hashes=`none`\n",
		report.PromptArtifactEnabled,
		report.PromptArtifactRedactionPatterns,
		report.PromptBodyIncluded,
		report.ContextFileBodiesIncluded,
		report.ContextReferenceBodiesIncluded,
		report.SkillBodiesIncluded,
		report.ToolOutputBodiesIncluded,
		report.RawIssueBodiesIncluded,
		report.RawCommentBodiesIncluded,
		report.RawInputsIncluded,
		report.CredentialValuesIncluded,
		report.RepositoryMutationAllowed,
		report.HostExecAllowed,
	)
}

func promptRiskTranscriptFindings(transcript []TranscriptMessage, budget Config) []PromptRiskFinding {
	bounded, _ := boundedTranscript(transcript, budget.MaxTranscriptMessages)
	var findings []PromptRiskFinding
	for index, msg := range bounded {
		body := truncatePromptText(msg.Body, budget.MaxTranscriptMessageBytes)
		findings = append(findings, scanPromptRiskText("transcript", fmt.Sprintf("message:%02d", index+1), "body", body, promptRiskRules)...)
	}
	sortPromptRiskFindings(findings)
	return findings
}

func promptRiskDocumentFindings(kind string, docs []ContextDocument) []PromptRiskFinding {
	var findings []PromptRiskFinding
	for _, doc := range docs {
		findings = append(findings, scanPromptRiskText(kind, doc.Path, "body", doc.Body, promptRiskRules)...)
	}
	sortPromptRiskFindings(findings)
	return findings
}

func promptRiskToolOutputFindings(outputs []ToolOutput) []PromptRiskFinding {
	var findings []PromptRiskFinding
	for _, output := range outputs {
		findings = append(findings, scanPromptRiskText("tool-output", output.Name, "output", output.Output, promptRiskRules)...)
	}
	sortPromptRiskFindings(findings)
	return findings
}

func scanPromptRiskText(kind, path, field, body string, rules []promptRiskRule) []PromptRiskFinding {
	var findings []PromptRiskFinding
	for lineNumber, line := range strings.Split(body, "\n") {
		lower := strings.ToLower(line)
		for _, rule := range rules {
			if !promptRiskRuleMatches(lower, rule) {
				continue
			}
			findings = append(findings, PromptRiskFinding{
				Severity: rule.Severity,
				Code:     rule.Code,
				Category: rule.Category,
				Kind:     kind,
				Path:     path,
				Field:    field,
				Line:     lineNumber + 1,
				LineSHA:  shortDocumentHash(line),
			})
		}
	}
	sortPromptRiskFindings(findings)
	return findings
}

func promptRiskRuleMatches(lowerLine string, rule promptRiskRule) bool {
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

func promptRiskMetadataFinding(severity, code, category, kind, path, field string) PromptRiskFinding {
	return PromptRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     kind,
		Path:     path,
		Field:    field,
		LineSHA:  shortDocumentHash(kind + ":" + path + ":" + field + ":" + code),
	}
}

func writePromptRiskFindings(b *strings.Builder, findings []PromptRiskFinding) {
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

func promptRiskFindingsByKind(findings []PromptRiskFinding, kind string) []PromptRiskFinding {
	var filtered []PromptRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			filtered = append(filtered, finding)
		}
	}
	sortPromptRiskFindings(filtered)
	return filtered
}

func promptRiskFindingsByKindPath(findings []PromptRiskFinding, kind, path string) []PromptRiskFinding {
	var filtered []PromptRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind && finding.Path == path {
			filtered = append(filtered, finding)
		}
	}
	sortPromptRiskFindings(filtered)
	return filtered
}

func promptRiskSurfaceCount(findings []PromptRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.Kind+"\x00"+finding.Path] = true
	}
	return len(seen)
}

func promptRiskCodes(findings []PromptRiskFinding) []string {
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

func promptRiskLineHashes(findings []PromptRiskFinding) []string {
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

func promptRiskMaxSeverity(findings []PromptRiskFinding) string {
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

func sortPromptRiskFindings(findings []PromptRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		if left.Severity != right.Severity {
			return promptRiskSeverityRank(left.Severity) < promptRiskSeverityRank(right.Severity)
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

func promptRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}
