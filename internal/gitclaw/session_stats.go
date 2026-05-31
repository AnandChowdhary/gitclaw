package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionStatsReport struct {
	Scope                                 string
	BackupFile                            string
	Repo                                  string
	IssueNumber                           int
	EventKind                             string
	SessionStatsStatus                    string
	RawComments                           int
	TranscriptMessages                    int
	UserMessages                          int
	AssistantMessages                     int
	TrustedMessages                       int
	UntrustedMessages                     int
	EditedMessages                        int
	TranscriptBodyBytes                   int
	TranscriptBodyLines                   int
	AssistantTurnComments                 int
	AssistantTurnsWithPromptProvenance    int
	AssistantTurnsMissingPromptProvenance int
	UniquePromptContextHashes             int
	ModelBackedAssistantTurns             int
	DeterministicAssistantTurns           int
	ModelNames                            []string
	PromptVisibleSkillNames               []string
	PromptVisibleToolNames                []string
	PromptVisibleSkillCount               int
	PromptVisibleToolCount                int
	HeartbeatComments                     int
	ErrorMarkerComments                   int
	ChannelMessageComments                int
	ChannelThreadIssue                    bool
	ProactiveRunIssue                     bool
	RawBodiesIncluded                     bool
	RawPromptsIncluded                    bool
	RawToolOutputsIncluded                bool
}

func BuildSessionStatsReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionStatsReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	report := SessionStatsReport{
		Scope:                                 scope,
		BackupFile:                            backupFile,
		Repo:                                  ev.Repo,
		IssueNumber:                           ev.Issue.Number,
		EventKind:                             ev.Kind,
		SessionStatsStatus:                    "ok",
		RawComments:                           len(comments),
		TranscriptMessages:                    len(transcript),
		UserMessages:                          countTranscriptRole(transcript, "user"),
		AssistantMessages:                     countTranscriptRole(transcript, "assistant"),
		TrustedMessages:                       countTrustedTranscriptMessages(transcript, true),
		UntrustedMessages:                     countTrustedTranscriptMessages(transcript, false),
		EditedMessages:                        countEditedTranscriptMessages(transcript),
		TranscriptBodyBytes:                   sessionStatsTranscriptBytes(transcript),
		TranscriptBodyLines:                   sessionStatsTranscriptLines(transcript),
		AssistantTurnComments:                 counts.AssistantTurns,
		AssistantTurnsWithPromptProvenance:    provenance.TurnsWithProvenance,
		AssistantTurnsMissingPromptProvenance: provenance.PromptContextHashMissing,
		UniquePromptContextHashes:             provenance.UniquePromptContextSHAs,
		ModelBackedAssistantTurns:             modelBackedTurns,
		DeterministicAssistantTurns:           deterministicTurns,
		ModelNames:                            modelNames,
		PromptVisibleSkillNames:               provenance.PromptVisibleSkillNames,
		PromptVisibleToolNames:                provenance.PromptVisibleToolNames,
		PromptVisibleSkillCount:               len(provenance.PromptVisibleSkillNames),
		PromptVisibleToolCount:                len(provenance.PromptVisibleToolNames),
		HeartbeatComments:                     counts.Heartbeats,
		ErrorMarkerComments:                   counts.Errors,
		ChannelMessageComments:                counts.ChannelMessages,
		ChannelThreadIssue:                    HasChannelThreadMarker(ev.Issue.Body),
		ProactiveRunIssue:                     HasProactiveRunMarker(ev.Issue.Body),
		RawBodiesIncluded:                     false,
		RawPromptsIncluded:                    false,
		RawToolOutputsIncluded:                false,
	}
	if report.TranscriptMessages == 0 {
		report.SessionStatsStatus = "empty"
	}
	return report
}

func RenderSessionStatsReport(report SessionStatsReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Stats Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if report.Scope == "" {
		report.Scope = "issue-thread"
	}
	fmt.Fprintf(&b, "- scope: `%s`\n", report.Scope)
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(report.BackupFile))
	}
	if report.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", report.Repo)
	}
	if report.IssueNumber != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", report.IssueNumber)
	}
	fmt.Fprintf(&b, "- event_kind: `%s`\n", report.EventKind)
	fmt.Fprintf(&b, "- session_stats_status: `%s`\n", report.SessionStatsStatus)
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- trusted_messages: `%d`\n", report.TrustedMessages)
	fmt.Fprintf(&b, "- untrusted_messages: `%d`\n", report.UntrustedMessages)
	fmt.Fprintf(&b, "- edited_messages: `%d`\n", report.EditedMessages)
	fmt.Fprintf(&b, "- transcript_body_bytes: `%d`\n", report.TranscriptBodyBytes)
	fmt.Fprintf(&b, "- transcript_body_lines: `%d`\n", report.TranscriptBodyLines)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_count: `%d`\n", report.PromptVisibleSkillCount)
	fmt.Fprintf(&b, "- prompt_visible_tool_count: `%d`\n", report.PromptVisibleToolCount)
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- heartbeat_comments: `%d`\n", report.HeartbeatComments)
	fmt.Fprintf(&b, "- error_marker_comments: `%d`\n", report.ErrorMarkerComments)
	fmt.Fprintf(&b, "- channel_message_comments: `%d`\n", report.ChannelMessageComments)
	fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", report.ChannelThreadIssue)
	fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", report.ProactiveRunIssue)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n\n", report.RawToolOutputsIncluded)
	b.WriteString("This report is a compact, body-free session summary. It reports counts, model names, prompt-visible skill/tool names, and provenance totals only; issue bodies, comment bodies, assistant replies, prompts, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Stats Cards\n")
	fmt.Fprintf(&b, "- kind=`transcript-shape` transcript_messages=`%d` user_messages=`%d` assistant_messages=`%d` trusted_messages=`%d` untrusted_messages=`%d` edited_messages=`%d` body_bytes=`%d` body_lines=`%d`\n",
		report.TranscriptMessages,
		report.UserMessages,
		report.AssistantMessages,
		report.TrustedMessages,
		report.UntrustedMessages,
		report.EditedMessages,
		report.TranscriptBodyBytes,
		report.TranscriptBodyLines,
	)
	fmt.Fprintf(&b, "- kind=`assistant-provenance` assistant_turn_comments=`%d` with_prompt_provenance=`%d` missing_prompt_provenance=`%d` unique_prompt_context_hashes=`%d` model_backed_turns=`%d` deterministic_turns=`%d` model_names=`%s`\n",
		report.AssistantTurnComments,
		report.AssistantTurnsWithPromptProvenance,
		report.AssistantTurnsMissingPromptProvenance,
		report.UniquePromptContextHashes,
		report.ModelBackedAssistantTurns,
		report.DeterministicAssistantTurns,
		inlineListOrNone(report.ModelNames),
	)
	fmt.Fprintf(&b, "- kind=`prompt-surface` prompt_visible_skill_count=`%d` prompt_visible_tool_count=`%d` prompt_visible_skill_names=`%s` prompt_visible_tool_names=`%s`\n",
		report.PromptVisibleSkillCount,
		report.PromptVisibleToolCount,
		inlineListOrNone(report.PromptVisibleSkillNames),
		inlineListOrNone(report.PromptVisibleToolNames),
	)
	fmt.Fprintf(&b, "- kind=`session-markers` heartbeat_comments=`%d` error_marker_comments=`%d` channel_message_comments=`%d` channel_thread_issue=`%t` proactive_run_issue=`%t`\n",
		report.HeartbeatComments,
		report.ErrorMarkerComments,
		report.ChannelMessageComments,
		report.ChannelThreadIssue,
		report.ProactiveRunIssue,
	)
	return strings.TrimSpace(b.String())
}

func RenderSessionStatsCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return RenderSessionStatsReport(BuildSessionStatsReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func sessionStatsModelSummary(turns []sessionPromptProvenanceTurn) ([]string, int, int) {
	seen := map[string]bool{}
	var names []string
	modelBacked := 0
	deterministic := 0
	for _, turn := range turns {
		model := strings.TrimSpace(turn.Model)
		if model == "" {
			continue
		}
		if strings.HasPrefix(model, "gitclaw/") {
			deterministic++
		} else {
			modelBacked++
		}
		if !seen[model] {
			seen[model] = true
			names = append(names, model)
		}
	}
	sort.Strings(names)
	return names, modelBacked, deterministic
}

func sessionStatsTranscriptBytes(transcript []TranscriptMessage) int {
	total := 0
	for _, msg := range transcript {
		total += len(msg.Body)
	}
	return total
}

func sessionStatsTranscriptLines(transcript []TranscriptMessage) int {
	total := 0
	for _, msg := range transcript {
		total += lineCount(msg.Body)
	}
	return total
}
