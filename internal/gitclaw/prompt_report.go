package gitclaw

import (
	"fmt"
	"os"
	"strings"
)

type promptReportProfile struct {
	Prompt                    string
	Budget                    Config
	BoundedTranscriptMessages int
	OmittedOlderMessages      int
	TruncatedTranscriptBodies int
	PromptContainsTruncation  bool
}

func IsPromptReportRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 {
		return false
	}
	if fields[0] != "/prompt" && fields[0] != "/budget" && fields[0] != "/prompt-budget" {
		return false
	}
	return len(fields) < 2 || (!strings.EqualFold(fields[1], "risk") && !strings.EqualFold(fields[1], "risk-audit") && !isPromptPackFields(fields) && !isPromptCacheFields(fields) && !isPromptContextFields(fields))
}

func IsPromptRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		(fields[0] == "/prompt" || fields[0] == "/budget" || fields[0] == "/prompt-budget") &&
		(strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}

func RenderPromptReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderPromptReport(ev, cfg, transcript, repoContext, true)
}

func RenderPromptCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPromptReport(Event{}, cfg, nil, repoContext, false)
}

func renderPromptReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	profile := buildPromptReportProfile(LLMRequest{
		Event:      ev,
		Transcript: transcript,
		Context:    repoContext,
		Config:     cfg,
	})
	var b strings.Builder
	b.WriteString("## GitClaw Prompt Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- provider: `%s`\n", cfg.ModelProvider)
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- system_prompt_bytes: `%d`\n", len(systemPrompt))
	fmt.Fprintf(&b, "- system_prompt_sha256_12: `%s`\n", shortDocumentHash(systemPrompt))
	fmt.Fprintf(&b, "- prompt_bytes: `%d`\n", len(profile.Prompt))
	fmt.Fprintf(&b, "- prompt_lines: `%d`\n", lineCount(profile.Prompt))
	fmt.Fprintf(&b, "- prompt_sha256_12: `%s`\n", shortDocumentHash(profile.Prompt))
	fmt.Fprintf(&b, "- max_prompt_bytes: `%d`\n", profile.Budget.MaxPromptBytes)
	fmt.Fprintf(&b, "- max_output_tokens: `%d`\n", cfg.MaxOutputTokens)
	fmt.Fprintf(&b, "- max_transcript_messages: `%d`\n", profile.Budget.MaxTranscriptMessages)
	fmt.Fprintf(&b, "- max_transcript_message_bytes: `%d`\n", profile.Budget.MaxTranscriptMessageBytes)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- bounded_transcript_messages: `%d`\n", profile.BoundedTranscriptMessages)
	fmt.Fprintf(&b, "- omitted_older_messages: `%d`\n", profile.OmittedOlderMessages)
	fmt.Fprintf(&b, "- truncated_transcript_bodies: `%d`\n", profile.TruncatedTranscriptBodies)
	fmt.Fprintf(&b, "- prompt_contains_truncation_marker: `%t`\n", profile.PromptContainsTruncation)
	fmt.Fprintf(&b, "- prompt_artifact_enabled: `%t`\n", strings.TrimSpace(os.Getenv("GITCLAW_PROMPT_ARTIFACT_PATH")) != "")
	fmt.Fprintf(&b, "- prompt_artifact_redaction_patterns: `%d`\n", len(promptArtifactRedactions))
	fmt.Fprintf(&b, "- prompt_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_prompt_report_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Prompt text, issue/comment bodies, context file bodies, skill bodies, and tool output bodies are not included. Hashes and size metadata make the prompt construction auditable without turning a diagnostic issue into a prompt dump.\n\n")

	b.WriteString("### Prompt Inputs\n")
	fmt.Fprintf(&b, "- context_files: `%d`\n", len(repoContext.Documents))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- tool_outputs: `%d`\n", len(repoContext.ToolOutputs))

	b.WriteString("\n### Context Files\n")
	writePromptDocumentList(&b, repoContext.Documents)

	b.WriteString("\n### Selected Skills\n")
	writePromptDocumentList(&b, repoContext.Skills)

	b.WriteString("\n### Tool Outputs\n")
	writeToolOutputList(&b, repoContext.ToolOutputs)

	return strings.TrimSpace(b.String())
}

func buildPromptReportProfile(req LLMRequest) promptReportProfile {
	budget := promptBudgetConfig(req.Config)
	bounded, omitted := boundedTranscript(req.Transcript, budget.MaxTranscriptMessages)
	truncatedBodies := 0
	for _, msg := range bounded {
		if len(msg.Body) > budget.MaxTranscriptMessageBytes {
			truncatedBodies++
		}
	}
	prompt := BuildPrompt(req)
	return promptReportProfile{
		Prompt:                    prompt,
		Budget:                    budget,
		BoundedTranscriptMessages: len(bounded),
		OmittedOlderMessages:      omitted,
		TruncatedTranscriptBodies: truncatedBodies,
		PromptContainsTruncation:  strings.Contains(prompt, "[gitclaw:truncated"),
	}
}

func writePromptDocumentList(b *strings.Builder, docs []ContextDocument) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
}
