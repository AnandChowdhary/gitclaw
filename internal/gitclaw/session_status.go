package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionStatusReport struct {
	Scope                              string
	BackupFile                         string
	Repo                               string
	IssueNumber                        int
	EventKind                          string
	ActiveCommand                      string
	SessionStatus                      string
	LabelNames                         []string
	RawComments                        int
	TranscriptMessages                 int
	UserMessages                       int
	AssistantMessages                  int
	AssistantTurnComments              int
	ModelBackedAssistantTurns          int
	DeterministicAssistantTurns        int
	ModelNames                         []string
	PromptVisibleSkillNames            []string
	PromptVisibleToolNames             []string
	LatestUser                         SessionStatusMessage
	LatestAssistant                    SessionStatusMessage
	LatestAssistantModel               string
	LatestAssistantPromptContextSHA    string
	LatestAssistantContextDocuments    int
	LatestAssistantSelectedSkills      int
	LatestAssistantToolOutputs         int
	LatestAssistantPromptVisibleSkills []string
	LatestAssistantPromptVisibleTools  []string
	ChannelThreadIssue                 bool
	ProactiveRunIssue                  bool
	RawBodiesIncluded                  bool
	RawPromptsIncluded                 bool
	RawToolOutputsIncluded             bool
	LLME2ERequiredOnChange             bool
	SkillUsage                         []SessionStatusNameCount
	ToolUsage                          []SessionStatusNameCount
}

type SessionStatusMessage struct {
	Present bool
	Role    string
	Source  string
	Actor   string
	Trusted bool
	Edited  bool
	Bytes   int
	Lines   int
	SHA     string
}

type SessionStatusNameCount struct {
	Name  string
	Turns int
}

func requestedSessionStatus(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/session" {
		return false
	}
	subcommand := strings.Trim(strings.ToLower(fields[1]), " \t\r\n.,:;!?")
	return subcommand == "status" || subcommand == "readback"
}

func RenderSessionStatusReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) string {
	report := BuildSessionStatusReport("issue-thread", "", ev, comments, transcript)
	report.ActiveCommand = strings.Join(activeSlashCommandFields(ev, cfg), " ")
	return renderSessionStatusReport(report)
}

func RenderSessionStatusCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
			Labels: backup.Issue.Labels,
		},
	}
	return renderSessionStatusReport(BuildSessionStatusReport("local-backup", backupPath, ev, commentsFromBackup(backup.Comments), backup.Transcript))
}

func BuildSessionStatusReport(scope, backupFile string, ev Event, comments []Comment, transcript []TranscriptMessage) SessionStatusReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	report := SessionStatusReport{
		Scope:                       scope,
		BackupFile:                  backupFile,
		Repo:                        ev.Repo,
		IssueNumber:                 ev.Issue.Number,
		EventKind:                   ev.Kind,
		SessionStatus:               "ok",
		LabelNames:                  sortedSessionStatusStrings(ev.Issue.Labels),
		RawComments:                 len(comments),
		TranscriptMessages:          len(transcript),
		UserMessages:                countTranscriptRole(transcript, "user"),
		AssistantMessages:           countTranscriptRole(transcript, "assistant"),
		AssistantTurnComments:       counts.AssistantTurns,
		ModelBackedAssistantTurns:   modelBackedTurns,
		DeterministicAssistantTurns: deterministicTurns,
		ModelNames:                  modelNames,
		PromptVisibleSkillNames:     sortedSessionStatusStrings(provenance.PromptVisibleSkillNames),
		PromptVisibleToolNames:      sortedSessionStatusStrings(provenance.PromptVisibleToolNames),
		LatestUser:                  latestSessionStatusMessage(transcript, "user"),
		LatestAssistant:             latestSessionStatusMessage(transcript, "assistant"),
		ChannelThreadIssue:          HasChannelThreadMarker(ev.Issue.Body),
		ProactiveRunIssue:           HasProactiveRunMarker(ev.Issue.Body),
		RawBodiesIncluded:           false,
		RawPromptsIncluded:          false,
		RawToolOutputsIncluded:      false,
		LLME2ERequiredOnChange:      true,
		SkillUsage:                  sessionStatusNameCounts(provenance.Turns, "skills"),
		ToolUsage:                   sessionStatusNameCounts(provenance.Turns, "tools"),
	}
	if len(transcript) == 0 {
		report.SessionStatus = "empty"
	}
	if len(provenance.Turns) > 0 {
		turn := provenance.Turns[len(provenance.Turns)-1]
		report.LatestAssistantModel = turn.Model
		report.LatestAssistantPromptContextSHA = turn.PromptContextSHA
		report.LatestAssistantContextDocuments = turn.ContextDocuments
		report.LatestAssistantSelectedSkills = turn.SelectedSkills
		report.LatestAssistantToolOutputs = turn.ToolOutputs
		report.LatestAssistantPromptVisibleSkills = append([]string(nil), turn.Skills...)
		report.LatestAssistantPromptVisibleTools = append([]string(nil), turn.Tools...)
		sort.Strings(report.LatestAssistantPromptVisibleSkills)
		sort.Strings(report.LatestAssistantPromptVisibleTools)
	}
	return report
}

func renderSessionStatusReport(report SessionStatusReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Status Report\n\n")
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
	fmt.Fprintf(&b, "- active_command: `%s`\n", inlineCode(report.ActiveCommand))
	fmt.Fprintf(&b, "- session_status: `%s`\n", report.SessionStatus)
	fmt.Fprintf(&b, "- label_names: `%s`\n", inlineListOrNone(report.LabelNames))
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- latest_user_message_present: `%t`\n", report.LatestUser.Present)
	fmt.Fprintf(&b, "- latest_assistant_message_present: `%t`\n", report.LatestAssistant.Present)
	fmt.Fprintf(&b, "- latest_assistant_model: `%s`\n", inlineCode(report.LatestAssistantModel))
	fmt.Fprintf(&b, "- latest_assistant_prompt_context_sha256_12: `%s`\n", inlineCode(report.LatestAssistantPromptContextSHA))
	fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", report.ChannelThreadIssue)
	fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", report.ProactiveRunIssue)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_status_change: `%t`\n\n", report.LLME2ERequiredOnChange)
	b.WriteString("This Hermes-inspired status readback summarizes the current GitHub issue session without replaying bodies. It reports labels, counts, model names, latest-message hashes, and assistant-marker provenance only; issue bodies, comment bodies, assistant replies, prompts, search queries, and tool outputs are not included.\n\n")

	b.WriteString("### Latest Message Hashes\n")
	writeSessionStatusMessage(&b, "user", report.LatestUser)
	writeSessionStatusMessage(&b, "assistant", report.LatestAssistant)

	b.WriteString("\n### Latest Assistant Marker\n")
	if report.LatestAssistantModel == "" && report.LatestAssistantPromptContextSHA == "" {
		b.WriteString("- none\n")
	} else {
		fmt.Fprintf(&b, "- model=`%s` prompt_context_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s`\n",
			inlineCode(report.LatestAssistantModel),
			inlineCode(report.LatestAssistantPromptContextSHA),
			report.LatestAssistantContextDocuments,
			report.LatestAssistantSelectedSkills,
			report.LatestAssistantToolOutputs,
			inlineListOrNone(report.LatestAssistantPromptVisibleSkills),
			inlineListOrNone(report.LatestAssistantPromptVisibleTools),
		)
	}

	b.WriteString("\n### Prompt Surface Counts\n")
	writeSessionStatusNameCounts(&b, "skill", report.SkillUsage)
	writeSessionStatusNameCounts(&b, "tool", report.ToolUsage)

	b.WriteString("\n### Status Notes\n")
	b.WriteString("- issue comments remain the canonical session log\n")
	b.WriteString("- assistant markers remain the canonical model/provenance index\n")
	b.WriteString("- backup JSON can replay the same body-free status locally\n")
	b.WriteString("- raw issue bodies, comment bodies, assistant replies, prompts, and tool outputs are excluded\n")

	return strings.TrimSpace(b.String())
}

func latestSessionStatusMessage(transcript []TranscriptMessage, role string) SessionStatusMessage {
	for i := len(transcript) - 1; i >= 0; i-- {
		msg := transcript[i]
		if msg.Role != role {
			continue
		}
		source := "issue"
		if msg.CommentID != 0 {
			source = fmt.Sprintf("comment:%d", msg.CommentID)
		}
		return SessionStatusMessage{
			Present: true,
			Role:    msg.Role,
			Source:  source,
			Actor:   msg.Actor,
			Trusted: msg.Trusted,
			Edited:  msg.Edited,
			Bytes:   len(msg.Body),
			Lines:   lineCount(msg.Body),
			SHA:     shortDocumentHash(msg.Body),
		}
	}
	return SessionStatusMessage{Role: role}
}

func writeSessionStatusMessage(b *strings.Builder, kind string, msg SessionStatusMessage) {
	if !msg.Present {
		fmt.Fprintf(b, "- kind=`%s` present=`false`\n", kind)
		return
	}
	fmt.Fprintf(b, "- kind=`%s` present=`true` source=`%s` actor=`%s` trusted=`%t` edited=`%t` bytes=`%d` lines=`%d` sha256_12=`%s`\n",
		kind,
		msg.Source,
		inlineCode(msg.Actor),
		msg.Trusted,
		msg.Edited,
		msg.Bytes,
		msg.Lines,
		msg.SHA,
	)
}

func sessionStatusNameCounts(turns []sessionPromptProvenanceTurn, kind string) []SessionStatusNameCount {
	counts := map[string]int{}
	for _, turn := range turns {
		var names []string
		if kind == "skills" {
			names = turn.Skills
		} else {
			names = turn.Tools
		}
		for _, name := range names {
			if name != "" {
				counts[name]++
			}
		}
	}
	out := make([]SessionStatusNameCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, SessionStatusNameCount{Name: name, Turns: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Turns != out[j].Turns {
			return out[i].Turns > out[j].Turns
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func writeSessionStatusNameCounts(b *strings.Builder, kind string, counts []SessionStatusNameCount) {
	if len(counts) == 0 {
		fmt.Fprintf(b, "- kind=`%s` none\n", kind)
		return
	}
	for _, count := range counts {
		fmt.Fprintf(b, "- kind=`%s` name=`%s` turns=`%d`\n", kind, inlineCode(count.Name), count.Turns)
	}
}

func sortedSessionStatusStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
