package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type RunHistoryReport struct {
	Status                              string
	Source                              string
	BackupFile                          string
	Repository                          string
	IssueNumber                         int
	CommentsScanned                     int
	AssistantTurns                      int
	ModelBackedTurns                    int
	DeterministicTurns                  int
	TurnsWithPromptProvenance           int
	TurnsMissingPromptProvenance        int
	UniqueRunIDs                        int
	UniqueModels                        []string
	PromptVisibleSkillNames             []string
	PromptVisibleToolNames              []string
	RawBodiesIncluded                   bool
	RawRunPayloadsIncluded              bool
	RawToolOutputsIncluded              bool
	RawPromptsIncluded                  bool
	LLME2ERequiredAfterRunHistoryChange bool
	Entries                             []RunHistoryEntry
}

type RunHistoryEntry struct {
	Index             int
	Source            string
	RunID             string
	EventID           string
	Model             string
	IdempotencyKeySHA string
	RunURLSHA         string
	PromptContextSHA  string
	ContextDocuments  int
	SelectedSkills    int
	ToolOutputs       int
	Skills            []string
	Tools             []string
	HasPromptEvidence bool
	Deterministic     bool
	CommentSHA        string
}

func IsRunHistoryRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/runs" && command != "/run" && command != "/ledger" {
		return false
	}
	subcommand := strings.Trim(strings.ToLower(fields[1]), " \t\r\n.,:;!?")
	return subcommand == "history" || subcommand == "timeline"
}

func RenderRunHistoryReport(ev Event, cfg Config, comments []Comment) string {
	_ = cfg
	return renderRunHistoryReport(BuildRunHistoryReport("issue-thread", "", ev, comments), true)
}

func RenderRunHistoryCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return renderRunHistoryReport(BuildRunHistoryReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments)), false)
}

func BuildRunHistoryReport(source, backupFile string, ev Event, comments []Comment) RunHistoryReport {
	report := RunHistoryReport{
		Status:                              "empty",
		Source:                              source,
		BackupFile:                          backupFile,
		Repository:                          ev.Repo,
		IssueNumber:                         ev.Issue.Number,
		CommentsScanned:                     len(comments),
		LLME2ERequiredAfterRunHistoryChange: true,
	}
	runIDs := map[string]bool{}
	models := map[string]bool{}
	skills := map[string]bool{}
	tools := map[string]bool{}
	for _, comment := range comments {
		entry, ok := parseRunHistoryEntry(comment, len(report.Entries)+1)
		if !ok {
			continue
		}
		report.Entries = append(report.Entries, entry)
		report.AssistantTurns++
		if entry.Model != "" {
			models[entry.Model] = true
		}
		if entry.RunID != "" {
			runIDs[entry.RunID] = true
		}
		if entry.Deterministic {
			report.DeterministicTurns++
		} else if entry.Model != "" {
			report.ModelBackedTurns++
		}
		if entry.HasPromptEvidence {
			report.TurnsWithPromptProvenance++
		} else {
			report.TurnsMissingPromptProvenance++
		}
		for _, skill := range entry.Skills {
			if skill != "" {
				skills[skill] = true
			}
		}
		for _, tool := range entry.Tools {
			if tool != "" {
				tools[tool] = true
			}
		}
	}
	if report.AssistantTurns > 0 {
		report.Status = "ok"
	}
	report.UniqueRunIDs = len(runIDs)
	report.UniqueModels = runHistorySortedKeys(models)
	report.PromptVisibleSkillNames = runHistorySortedKeys(skills)
	report.PromptVisibleToolNames = runHistorySortedKeys(tools)
	return report
}

func parseRunHistoryEntry(comment Comment, index int) (RunHistoryEntry, bool) {
	match := markerPattern.FindStringSubmatch(comment.Body)
	if len(match) < 2 {
		return RunHistoryEntry{}, false
	}
	attrs := match[1]
	model := markerAttribute(attrs, "model")
	idempotencyKey := markerAttribute(attrs, "idempotency_key")
	runURL := markerAttribute(attrs, "run_url")
	promptContextSHA := markerAttribute(attrs, "prompt_context_sha256_12")
	entry := RunHistoryEntry{
		Index:             index,
		Source:            fmt.Sprintf("comment:%d", comment.ID),
		RunID:             markerAttribute(attrs, "run_id"),
		EventID:           markerAttribute(attrs, "event_id"),
		Model:             model,
		PromptContextSHA:  promptContextSHA,
		ContextDocuments:  markerAttributeInt(attrs, "context_documents"),
		SelectedSkills:    markerAttributeInt(attrs, "selected_skills"),
		ToolOutputs:       markerAttributeInt(attrs, "tool_outputs"),
		Skills:            markerAttributeList(attrs, "skills"),
		Tools:             markerAttributeList(attrs, "tools"),
		HasPromptEvidence: promptContextSHA != "",
		Deterministic:     strings.HasPrefix(model, "gitclaw/"),
		CommentSHA:        shortDocumentHash(comment.Body),
	}
	sort.Strings(entry.Skills)
	sort.Strings(entry.Tools)
	if idempotencyKey != "" {
		entry.IdempotencyKeySHA = shortDocumentHash(idempotencyKey)
	}
	if runURL != "" {
		entry.RunURLSHA = shortDocumentHash(runURL)
	}
	return entry, true
}

func renderRunHistoryReport(report RunHistoryReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Run History Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", report.Repository)
		fmt.Fprintf(&b, "- issue: `#%d`\n", report.IssueNumber)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", report.Source)
		fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(report.BackupFile))
		fmt.Fprintf(&b, "- backup_repo: `%s`\n", report.Repository)
		fmt.Fprintf(&b, "- backup_issue: `#%d`\n", report.IssueNumber)
	}
	fmt.Fprintf(&b, "- run_history_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- history_source: `%s`\n", report.Source)
	fmt.Fprintf(&b, "- comments_scanned: `%d`\n", report.CommentsScanned)
	fmt.Fprintf(&b, "- assistant_turns: `%d`\n", report.AssistantTurns)
	fmt.Fprintf(&b, "- model_backed_turns: `%d`\n", report.ModelBackedTurns)
	fmt.Fprintf(&b, "- deterministic_turns: `%d`\n", report.DeterministicTurns)
	fmt.Fprintf(&b, "- turns_with_prompt_provenance: `%d`\n", report.TurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- turns_missing_prompt_provenance: `%d`\n", report.TurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_run_ids: `%d`\n", report.UniqueRunIDs)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.UniqueModels))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_run_payloads_included: `%t`\n", report.RawRunPayloadsIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_run_history_change: `%t`\n\n", report.LLME2ERequiredAfterRunHistoryChange)
	b.WriteString("This report reconstructs a body-free run timeline from GitClaw assistant markers. It reports model names, marker counts, and hashes only; assistant replies, prompts, tool outputs, issue bodies, and workflow payloads are excluded.\n\n")

	b.WriteString("### Run History Entries\n")
	if len(report.Entries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, entry := range report.Entries {
			fmt.Fprintf(&b, "- index=`%d` source=`%s` run_id=`%s` event_id=`%s` model=`%s` deterministic=`%t` has_prompt_evidence=`%t` prompt_context_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s` idempotency_key_sha256_12=`%s` run_url_sha256_12=`%s` comment_sha256_12=`%s`\n",
				entry.Index,
				entry.Source,
				inlineCode(entry.RunID),
				inlineCode(entry.EventID),
				inlineCode(entry.Model),
				entry.Deterministic,
				entry.HasPromptEvidence,
				inlineCode(entry.PromptContextSHA),
				entry.ContextDocuments,
				entry.SelectedSkills,
				entry.ToolOutputs,
				inlineListOrNone(entry.Skills),
				inlineListOrNone(entry.Tools),
				entry.IdempotencyKeySHA,
				entry.RunURLSHA,
				entry.CommentSHA,
			)
		}
	}

	b.WriteString("\n### History Notes\n")
	b.WriteString("- issue comments remain the canonical run history\n")
	b.WriteString("- assistant markers provide the replayable turn index\n")
	b.WriteString("- backup JSON can replay the same body-free history locally\n")
	b.WriteString("- raw assistant replies, prompts, tool outputs, issue bodies, and workflow payloads are excluded\n")

	return strings.TrimSpace(b.String())
}

func runHistorySortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
