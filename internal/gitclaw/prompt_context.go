package gitclaw

import (
	"fmt"
	"strings"
)

type PromptContextReport struct {
	Scope                                  string
	Repo                                   string
	IssueNumber                            int
	EventKind                              string
	EventName                              string
	Provider                               string
	Model                                  string
	Status                                 string
	ManifestFormat                         string
	PromptContextSHA                       string
	SystemPromptBytes                      int
	SystemPromptSHA                        string
	ContextFiles                           int
	SelectedSkills                         int
	AvailableSkills                        int
	ToolOutputs                            int
	PromptVisibleSkills                    []string
	PromptVisibleTools                     []string
	ContextFileBytes                       int
	ContextFileLines                       int
	SelectedSkillBytes                     int
	SelectedSkillLines                     int
	ToolOutputBytes                        int
	ToolOutputLines                        int
	ToolInputHashes                        int
	TranscriptMessages                     int
	BoundedTranscriptMessages              int
	OmittedOlderMessages                   int
	TruncatedTranscriptBodies              int
	MaxTranscriptMessages                  int
	MaxTranscriptMessageBytes              int
	PromptBodiesIncluded                   bool
	ContextFileBodiesIncluded              bool
	SkillBodiesIncluded                    bool
	ToolOutputBodiesIncluded               bool
	RawToolInputsIncluded                  bool
	RawIssueBodiesIncluded                 bool
	RawCommentBodiesIncluded               bool
	RawPromptsIncluded                     bool
	CredentialValuesIncluded               bool
	RepositoryMutationAllowed              bool
	LLME2ERequiredAfterPromptContextChange bool
	Cards                                  []PromptContextCard
}

type PromptContextCard struct {
	Kind          string
	Name          string
	Index         int
	Bytes         int
	Lines         int
	SHA           string
	InputSHA      string
	BodyIncluded  bool
	InputIncluded bool
}

func IsPromptContextRequest(ev Event, cfg Config) bool {
	return isPromptContextFields(activeSlashCommandFields(ev, cfg))
}

func isPromptContextFields(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/prompt" && fields[0] != "/budget" && fields[0] != "/prompt-budget" {
		return false
	}
	return strings.EqualFold(fields[1], "context") ||
		strings.EqualFold(fields[1], "manifest") ||
		strings.EqualFold(fields[1], "snapshot") ||
		strings.EqualFold(fields[1], "inputs")
}

func RenderPromptContextReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderPromptContextReport(BuildPromptContextReport("issue-thread", ev, cfg, transcript, repoContext), true)
}

func RenderPromptContextCLIReport(cfg Config, repoContext RepoContext) string {
	return renderPromptContextReport(BuildPromptContextReport("local-cli", Event{}, cfg, nil, repoContext), false)
}

func BuildPromptContextReport(scope string, ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) PromptContextReport {
	budget := promptBudgetConfig(cfg)
	bounded, omitted := boundedTranscript(transcript, budget.MaxTranscriptMessages)
	report := PromptContextReport{
		Scope:                                  scope,
		Repo:                                   ev.Repo,
		IssueNumber:                            ev.Issue.Number,
		EventKind:                              ev.Kind,
		EventName:                              ev.EventName,
		Provider:                               cfg.ModelProvider,
		Model:                                  cfg.Model,
		Status:                                 "ok",
		ManifestFormat:                         "gitclaw.prompt-context.v1",
		PromptContextSHA:                       promptContextHash(repoContext),
		SystemPromptBytes:                      len(systemPrompt),
		SystemPromptSHA:                        shortDocumentHash(systemPrompt),
		ContextFiles:                           len(repoContext.Documents),
		SelectedSkills:                         len(repoContext.Skills),
		AvailableSkills:                        availableSkillCount(repoContext),
		ToolOutputs:                            len(repoContext.ToolOutputs),
		PromptVisibleSkills:                    uniqueSortedStrings(promptVisibleSkillNames(repoContext.Skills)),
		PromptVisibleTools:                     uniqueSortedStrings(promptVisibleToolNames(repoContext.ToolOutputs)),
		TranscriptMessages:                     len(transcript),
		BoundedTranscriptMessages:              len(bounded),
		OmittedOlderMessages:                   omitted,
		MaxTranscriptMessages:                  budget.MaxTranscriptMessages,
		MaxTranscriptMessageBytes:              budget.MaxTranscriptMessageBytes,
		PromptBodiesIncluded:                   false,
		ContextFileBodiesIncluded:              false,
		SkillBodiesIncluded:                    false,
		ToolOutputBodiesIncluded:               false,
		RawToolInputsIncluded:                  false,
		RawIssueBodiesIncluded:                 false,
		RawCommentBodiesIncluded:               false,
		RawPromptsIncluded:                     false,
		CredentialValuesIncluded:               false,
		RepositoryMutationAllowed:              false,
		LLME2ERequiredAfterPromptContextChange: true,
	}
	for _, msg := range bounded {
		if len(msg.Body) > budget.MaxTranscriptMessageBytes {
			report.TruncatedTranscriptBodies++
		}
	}
	for _, doc := range repoContext.Documents {
		report.ContextFileBytes += len(doc.Body)
		report.ContextFileLines += lineCount(doc.Body)
		report.Cards = append(report.Cards, PromptContextCard{
			Kind:          "context-file",
			Name:          doc.Path,
			Index:         len(report.Cards) + 1,
			Bytes:         len(doc.Body),
			Lines:         lineCount(doc.Body),
			SHA:           shortDocumentHash(doc.Body),
			BodyIncluded:  false,
			InputIncluded: false,
		})
	}
	for _, skill := range repoContext.Skills {
		report.SelectedSkillBytes += len(skill.Body)
		report.SelectedSkillLines += lineCount(skill.Body)
		report.Cards = append(report.Cards, PromptContextCard{
			Kind:          "selected-skill",
			Name:          skill.Path,
			Index:         len(report.Cards) + 1,
			Bytes:         len(skill.Body),
			Lines:         lineCount(skill.Body),
			SHA:           shortDocumentHash(skill.Body),
			BodyIncluded:  false,
			InputIncluded: false,
		})
	}
	for _, output := range repoContext.ToolOutputs {
		report.ToolOutputBytes += len(output.Output)
		report.ToolOutputLines += lineCount(output.Output)
		inputSHA := shortDocumentHash(output.Input)
		if strings.TrimSpace(output.Input) != "" {
			report.ToolInputHashes++
		}
		report.Cards = append(report.Cards, PromptContextCard{
			Kind:          "tool-output",
			Name:          output.Name,
			Index:         len(report.Cards) + 1,
			Bytes:         len(output.Output),
			Lines:         lineCount(output.Output),
			SHA:           shortDocumentHash(output.Output),
			InputSHA:      inputSHA,
			BodyIncluded:  false,
			InputIncluded: false,
		})
	}
	if report.ContextFiles == 0 && report.SelectedSkills == 0 && report.ToolOutputs == 0 {
		report.Status = "empty"
	}
	return report
}

func renderPromptContextReport(report PromptContextReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Prompt Context Manifest\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", report.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", report.IssueNumber)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", report.EventKind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", report.EventName)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", report.Scope)
	}
	fmt.Fprintf(&b, "- prompt_context_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- manifest_format: `%s`\n", report.ManifestFormat)
	fmt.Fprintf(&b, "- provider: `%s`\n", report.Provider)
	fmt.Fprintf(&b, "- model: `%s`\n", report.Model)
	fmt.Fprintf(&b, "- prompt_context_sha256_12: `%s`\n", report.PromptContextSHA)
	fmt.Fprintf(&b, "- system_prompt_bytes: `%d`\n", report.SystemPromptBytes)
	fmt.Fprintf(&b, "- system_prompt_sha256_12: `%s`\n", report.SystemPromptSHA)
	fmt.Fprintf(&b, "- context_files: `%d`\n", report.ContextFiles)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- tool_outputs: `%d`\n", report.ToolOutputs)
	fmt.Fprintf(&b, "- prompt_visible_skills: `%s`\n", inlineListOrNone(report.PromptVisibleSkills))
	fmt.Fprintf(&b, "- prompt_visible_tools: `%s`\n", inlineListOrNone(report.PromptVisibleTools))
	fmt.Fprintf(&b, "- context_file_bytes: `%d`\n", report.ContextFileBytes)
	fmt.Fprintf(&b, "- context_file_lines: `%d`\n", report.ContextFileLines)
	fmt.Fprintf(&b, "- selected_skill_bytes: `%d`\n", report.SelectedSkillBytes)
	fmt.Fprintf(&b, "- selected_skill_lines: `%d`\n", report.SelectedSkillLines)
	fmt.Fprintf(&b, "- tool_output_bytes: `%d`\n", report.ToolOutputBytes)
	fmt.Fprintf(&b, "- tool_output_lines: `%d`\n", report.ToolOutputLines)
	fmt.Fprintf(&b, "- tool_input_hashes: `%d`\n", report.ToolInputHashes)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- bounded_transcript_messages: `%d`\n", report.BoundedTranscriptMessages)
	fmt.Fprintf(&b, "- omitted_older_messages: `%d`\n", report.OmittedOlderMessages)
	fmt.Fprintf(&b, "- truncated_transcript_bodies: `%d`\n", report.TruncatedTranscriptBodies)
	fmt.Fprintf(&b, "- max_transcript_messages: `%d`\n", report.MaxTranscriptMessages)
	fmt.Fprintf(&b, "- max_transcript_message_bytes: `%d`\n", report.MaxTranscriptMessageBytes)
	fmt.Fprintf(&b, "- prompt_bodies_included: `%t`\n", report.PromptBodiesIncluded)
	fmt.Fprintf(&b, "- context_file_bodies_included: `%t`\n", report.ContextFileBodiesIncluded)
	fmt.Fprintf(&b, "- skill_bodies_included: `%t`\n", report.SkillBodiesIncluded)
	fmt.Fprintf(&b, "- tool_output_bodies_included: `%t`\n", report.ToolOutputBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_inputs_included: `%t`\n", report.RawToolInputsIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_prompt_context_change: `%t`\n\n", report.LLME2ERequiredAfterPromptContextChange)
	b.WriteString("This OpenClaw/Hermes-inspired manifest shows the exact prompt-visible context inventory that normal model turns stamp into assistant markers. It reports paths, names, hashes, sizes, counts, and prompt-context identity only; prompt text, issue/comment bodies, context bodies, skill bodies, tool outputs, raw tool inputs, credentials, and secret values are not included.\n\n")

	b.WriteString("### Context Cards\n")
	writePromptContextCards(&b, report.Cards)

	b.WriteString("\n### Context Gates\n")
	fmt.Fprintf(&b, "- manifest_gate=`%s`\n", promptContextManifestGate(report))
	fmt.Fprintf(&b, "- skill_snapshot_gate=`%s`\n", promptContextSkillGate(report))
	fmt.Fprintf(&b, "- tool_snapshot_gate=`%s`\n", promptContextToolGate(report))
	fmt.Fprintf(&b, "- raw_body_gate=`hashes-counts-and-paths-only`\n")
	fmt.Fprintf(&b, "- mutation_gate=`disabled`")
	return strings.TrimSpace(b.String())
}

func writePromptContextCards(b *strings.Builder, cards []PromptContextCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(b, "- card=`%02d` kind=`%s` name=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`",
			card.Index,
			card.Kind,
			inlineCode(card.Name),
			card.Bytes,
			card.Lines,
			card.SHA,
		)
		if card.Kind == "tool-output" {
			fmt.Fprintf(b, " input_sha256_12=`%s`", card.InputSHA)
		}
		fmt.Fprintf(b, " body_included=`%t` input_included=`%t`\n", card.BodyIncluded, card.InputIncluded)
	}
}

func promptContextManifestGate(report PromptContextReport) string {
	if report.PromptContextSHA == "" {
		return "missing"
	}
	return "pass"
}

func promptContextSkillGate(report PromptContextReport) string {
	if report.AvailableSkills == 0 {
		return "none"
	}
	if report.SelectedSkills == 0 {
		return "not-selected"
	}
	return "pass"
}

func promptContextToolGate(report PromptContextReport) string {
	if report.ToolOutputs == 0 {
		return "none"
	}
	return "pass"
}
