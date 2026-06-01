package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionResumeReport struct {
	Scope                                 string
	BackupFile                            string
	Repo                                  string
	IssueNumber                           int
	EventKind                             string
	ActiveCommand                         string
	SessionResumeStatus                   string
	ResumeStrategy                        string
	ResumeKeySHA                          string
	RawComments                           int
	TranscriptMessages                    int
	UserMessages                          int
	AssistantMessages                     int
	LabelNames                            []string
	TriggerLabelPresent                   bool
	RunningLabelPresent                   bool
	DoneLabelPresent                      bool
	ErrorLabelPresent                     bool
	DisabledLabelPresent                  bool
	LatestUser                            SessionStatusMessage
	LatestAssistant                       SessionStatusMessage
	AssistantTurnComments                 int
	AssistantTurnsWithPromptProvenance    int
	AssistantTurnsMissingPromptProvenance int
	UniquePromptContextHashes             int
	ModelBackedAssistantTurns             int
	DeterministicAssistantTurns           int
	ModelNames                            []string
	PromptVisibleSkillNames               []string
	PromptVisibleToolNames                []string
	LatestAssistantModel                  string
	LatestAssistantPromptContextSHA       string
	LatestAssistantContextDocuments       int
	LatestAssistantSelectedSkills         int
	LatestAssistantToolOutputs            int
	LatestAssistantPromptVisibleSkills    []string
	LatestAssistantPromptVisibleTools     []string
	UsageBearingAssistantTurns            int
	UsagePromptTokens                     int
	UsageCompletionTokens                 int
	UsageTotalTokens                      int
	UsageCacheReadTokens                  int
	UsageCacheWriteTokens                 int
	NextIssueCommentResumesSession        bool
	GitHubActionsReentrySupported         bool
	WorkflowEvent                         string
	WorkflowDispatchRequired              bool
	ServerRequired                        bool
	SocketRequired                        bool
	ExternalSessionDBRequired             bool
	BackupBranchReplayPreferred           bool
	IssueThreadCanonicalStorage           bool
	ChannelThreadIssue                    bool
	ProactiveRunIssue                     bool
	RawBodiesIncluded                     bool
	RawIssueBodiesIncluded                bool
	RawCommentBodiesIncluded              bool
	RawAssistantRepliesIncluded           bool
	RawPromptsIncluded                    bool
	RawProviderResponsesIncluded          bool
	RawToolOutputsIncluded                bool
	RawSearchQueriesIncluded              bool
	RepositoryMutationAllowed             bool
	ResumeMutationAllowed                 bool
	LLME2ERequiredAfterSessionResume      bool
	Anchors                               []SessionResumeAnchor
}

type SessionResumeAnchor struct {
	Kind               string
	Source             string
	Model              string
	Trusted            bool
	Edited             bool
	Bytes              int
	Lines              int
	SHA                string
	PromptContextSHA   string
	ContextDocuments   int
	SelectedSkills     int
	ToolOutputs        int
	Skills             []string
	Tools              []string
	UsagePresent       bool
	UsageTotalTokens   int
	BodyIncluded       bool
	IdentifierIncluded bool
}

func requestedSessionResume(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || fields[0] != "/session" {
		return false
	}
	subcommand := strings.Trim(strings.ToLower(fields[1]), " \t\r\n.,:;!?")
	return subcommand == "resume" ||
		subcommand == "handoff" ||
		subcommand == "continuation" ||
		subcommand == "continue" ||
		subcommand == "yield"
}

func RenderSessionResumeReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) string {
	report := BuildSessionResumeReport("issue-thread", "", ev, cfg, comments, transcript)
	report.ActiveCommand = strings.Join(activeSlashCommandFields(ev, cfg), " ")
	return renderSessionResumeReport(report)
}

func RenderSessionResumeCLIReport(backupPath string, backup IssueBackup, cfg Config) string {
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
	return renderSessionResumeReport(BuildSessionResumeReport("local-backup", backupPath, ev, cfg, commentsFromBackup(backup.Comments), backup.Transcript))
}

func BuildSessionResumeReport(scope, backupFile string, ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) SessionResumeReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	modelNames, modelBackedTurns, deterministicTurns := sessionStatsModelSummary(provenance.Turns)
	report := SessionResumeReport{
		Scope:                                 scope,
		BackupFile:                            backupFile,
		Repo:                                  ev.Repo,
		IssueNumber:                           ev.Issue.Number,
		EventKind:                             ev.Kind,
		SessionResumeStatus:                   "ok",
		ResumeStrategy:                        "github-issue-comment-continuation",
		ResumeKeySHA:                          shortDocumentHash(fmt.Sprintf("%s#%d", ev.Repo, ev.Issue.Number)),
		RawComments:                           len(comments),
		TranscriptMessages:                    len(transcript),
		UserMessages:                          countTranscriptRole(transcript, "user"),
		AssistantMessages:                     countTranscriptRole(transcript, "assistant"),
		LabelNames:                            sortedSessionStatusStrings(ev.Issue.Labels),
		TriggerLabelPresent:                   hasLabel(ev.Issue.Labels, cfg.TriggerLabel),
		RunningLabelPresent:                   hasLabel(ev.Issue.Labels, cfg.RunningLabel),
		DoneLabelPresent:                      hasLabel(ev.Issue.Labels, cfg.DoneLabel),
		ErrorLabelPresent:                     hasLabel(ev.Issue.Labels, cfg.ErrorLabel),
		DisabledLabelPresent:                  hasLabel(ev.Issue.Labels, cfg.DisabledLabel),
		LatestUser:                            latestSessionStatusMessage(transcript, "user"),
		LatestAssistant:                       latestSessionStatusMessage(transcript, "assistant"),
		AssistantTurnComments:                 counts.AssistantTurns,
		AssistantTurnsWithPromptProvenance:    provenance.TurnsWithProvenance,
		AssistantTurnsMissingPromptProvenance: provenance.PromptContextHashMissing,
		UniquePromptContextHashes:             provenance.UniquePromptContextSHAs,
		ModelBackedAssistantTurns:             modelBackedTurns,
		DeterministicAssistantTurns:           deterministicTurns,
		ModelNames:                            modelNames,
		PromptVisibleSkillNames:               sortedSessionStatusStrings(provenance.PromptVisibleSkillNames),
		PromptVisibleToolNames:                sortedSessionStatusStrings(provenance.PromptVisibleToolNames),
		NextIssueCommentResumesSession:        true,
		GitHubActionsReentrySupported:         true,
		WorkflowEvent:                         "issue_comment",
		WorkflowDispatchRequired:              false,
		ServerRequired:                        false,
		SocketRequired:                        false,
		ExternalSessionDBRequired:             false,
		BackupBranchReplayPreferred:           true,
		IssueThreadCanonicalStorage:           true,
		ChannelThreadIssue:                    HasChannelThreadMarker(ev.Issue.Body),
		ProactiveRunIssue:                     HasProactiveRunMarker(ev.Issue.Body),
		RawBodiesIncluded:                     false,
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		RawAssistantRepliesIncluded:           false,
		RawPromptsIncluded:                    false,
		RawProviderResponsesIncluded:          false,
		RawToolOutputsIncluded:                false,
		RawSearchQueriesIncluded:              false,
		RepositoryMutationAllowed:             false,
		ResumeMutationAllowed:                 false,
		LLME2ERequiredAfterSessionResume:      true,
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
	report.UsagePromptTokens, report.UsageCompletionTokens, report.UsageTotalTokens, report.UsageCacheReadTokens, report.UsageCacheWriteTokens = sessionProvenanceUsageTotals(provenance.Turns)
	for _, turn := range provenance.Turns {
		if turn.Usage.Present {
			report.UsageBearingAssistantTurns++
		}
	}
	report.Anchors = buildSessionResumeAnchors(report, provenance.Turns)
	switch {
	case report.TranscriptMessages == 0:
		report.SessionResumeStatus = "empty"
	case report.DisabledLabelPresent || report.ErrorLabelPresent || report.RunningLabelPresent || !report.LatestUser.Present || !report.LatestAssistant.Present || report.ModelBackedAssistantTurns == 0:
		report.SessionResumeStatus = "warn"
	default:
		report.SessionResumeStatus = "ok"
	}
	return report
}

func renderSessionResumeReport(report SessionResumeReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Session Resume Report\n\n")
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
	fmt.Fprintf(&b, "- session_resume_status: `%s`\n", report.SessionResumeStatus)
	fmt.Fprintf(&b, "- resume_strategy: `%s`\n", report.ResumeStrategy)
	fmt.Fprintf(&b, "- resume_key_sha256_12: `%s`\n", report.ResumeKeySHA)
	fmt.Fprintf(&b, "- session_store: `%s`\n", "github-issue-thread")
	if report.BackupFile != "" {
		fmt.Fprintf(&b, "- local_backup_store: `%s`\n", "gitclaw-backups issue JSON")
	}
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- label_names: `%s`\n", inlineListOrNone(report.LabelNames))
	fmt.Fprintf(&b, "- trigger_label_present: `%t`\n", report.TriggerLabelPresent)
	fmt.Fprintf(&b, "- running_label_present: `%t`\n", report.RunningLabelPresent)
	fmt.Fprintf(&b, "- done_label_present: `%t`\n", report.DoneLabelPresent)
	fmt.Fprintf(&b, "- error_label_present: `%t`\n", report.ErrorLabelPresent)
	fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", report.DisabledLabelPresent)
	fmt.Fprintf(&b, "- latest_user_message_present: `%t`\n", report.LatestUser.Present)
	fmt.Fprintf(&b, "- latest_assistant_message_present: `%t`\n", report.LatestAssistant.Present)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- latest_assistant_model: `%s`\n", inlineCode(report.LatestAssistantModel))
	fmt.Fprintf(&b, "- latest_assistant_prompt_context_sha256_12: `%s`\n", inlineCode(report.LatestAssistantPromptContextSHA))
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(&b, "- usage_prompt_tokens: `%d`\n", report.UsagePromptTokens)
	fmt.Fprintf(&b, "- usage_completion_tokens: `%d`\n", report.UsageCompletionTokens)
	fmt.Fprintf(&b, "- usage_total_tokens: `%d`\n", report.UsageTotalTokens)
	fmt.Fprintf(&b, "- usage_cache_read_tokens: `%d`\n", report.UsageCacheReadTokens)
	fmt.Fprintf(&b, "- usage_cache_write_tokens: `%d`\n", report.UsageCacheWriteTokens)
	fmt.Fprintf(&b, "- next_issue_comment_resumes_session: `%t`\n", report.NextIssueCommentResumesSession)
	fmt.Fprintf(&b, "- github_actions_reentry_supported: `%t`\n", report.GitHubActionsReentrySupported)
	fmt.Fprintf(&b, "- workflow_event: `%s`\n", report.WorkflowEvent)
	fmt.Fprintf(&b, "- workflow_dispatch_required: `%t`\n", report.WorkflowDispatchRequired)
	fmt.Fprintf(&b, "- server_required: `%t`\n", report.ServerRequired)
	fmt.Fprintf(&b, "- socket_required: `%t`\n", report.SocketRequired)
	fmt.Fprintf(&b, "- external_session_db_required: `%t`\n", report.ExternalSessionDBRequired)
	fmt.Fprintf(&b, "- backup_branch_replay_preferred: `%t`\n", report.BackupBranchReplayPreferred)
	fmt.Fprintf(&b, "- issue_thread_canonical_storage: `%t`\n", report.IssueThreadCanonicalStorage)
	fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", report.ChannelThreadIssue)
	fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n", report.ProactiveRunIssue)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", report.RawAssistantRepliesIncluded)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(&b, "- raw_provider_responses_included: `%t`\n", report.RawProviderResponsesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- resume_mutation_allowed: `%t`\n", report.ResumeMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_resume_change: `%t`\n\n", report.LLME2ERequiredAfterSessionResume)
	b.WriteString("This Hermes-resume and OpenClaw-session inspired report proves how the current GitHub issue can continue as the same session. It reports the issue-thread resume key, labels, latest-message hashes, assistant-marker provenance, model and usage telemetry, and reentry gates only. GitClaw does not open a socket, require a server, create a hidden session database, mutate the repository, or print issue bodies, comments, assistant replies, prompts, search queries, provider responses, or tool outputs.\n\n")

	b.WriteString("### Resume Anchors\n")
	writeSessionResumeAnchors(&b, report.Anchors)

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

	b.WriteString("\n### Resume Gates\n")
	fmt.Fprintf(&b, "- continuation_gate=`%s`\n", sessionResumeContinuationGate(report))
	fmt.Fprintf(&b, "- issue_label_gate=`%s`\n", sessionResumeLabelGate(report))
	fmt.Fprintf(&b, "- latest_user_gate=`%s`\n", sessionResumeMessageGate(report.LatestUser))
	fmt.Fprintf(&b, "- latest_assistant_gate=`%s`\n", sessionResumeMessageGate(report.LatestAssistant))
	fmt.Fprintf(&b, "- model_backed_gate=`%s`\n", sessionResumeModelGate(report))
	fmt.Fprintf(&b, "- prompt_provenance_gate=`%s`\n", sessionResumePromptGate(report))
	fmt.Fprintf(&b, "- reentry_gate=`%s`\n", "issue-comment-action")
	fmt.Fprintf(&b, "- external_session_db_gate=`%s`\n", "disabled")
	fmt.Fprintf(&b, "- raw_body_gate=`%s`\n", "hashes-counts-and-marker-attributes-only")
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	return strings.TrimSpace(b.String())
}

func buildSessionResumeAnchors(report SessionResumeReport, turns []sessionPromptProvenanceTurn) []SessionResumeAnchor {
	anchors := []SessionResumeAnchor{{
		Kind:               "session-key",
		Source:             "github-issue",
		SHA:                report.ResumeKeySHA,
		BodyIncluded:       false,
		IdentifierIncluded: false,
	}}
	if report.LatestUser.Present {
		anchors = append(anchors, sessionResumeMessageAnchor("latest-user", report.LatestUser))
	}
	if report.LatestAssistant.Present {
		anchors = append(anchors, sessionResumeMessageAnchor("latest-assistant", report.LatestAssistant))
	}
	if len(turns) > 0 {
		turn := turns[len(turns)-1]
		skills := append([]string(nil), turn.Skills...)
		tools := append([]string(nil), turn.Tools...)
		sort.Strings(skills)
		sort.Strings(tools)
		anchors = append(anchors, SessionResumeAnchor{
			Kind:               "latest-model-turn",
			Source:             turn.Source,
			Model:              turn.Model,
			PromptContextSHA:   turn.PromptContextSHA,
			ContextDocuments:   turn.ContextDocuments,
			SelectedSkills:     turn.SelectedSkills,
			ToolOutputs:        turn.ToolOutputs,
			Skills:             skills,
			Tools:              tools,
			UsagePresent:       turn.Usage.Present,
			UsageTotalTokens:   turn.Usage.TotalTokens,
			BodyIncluded:       false,
			IdentifierIncluded: false,
		})
	}
	return anchors
}

func sessionResumeMessageAnchor(kind string, msg SessionStatusMessage) SessionResumeAnchor {
	return SessionResumeAnchor{
		Kind:               kind,
		Source:             msg.Source,
		Trusted:            msg.Trusted,
		Edited:             msg.Edited,
		Bytes:              msg.Bytes,
		Lines:              msg.Lines,
		SHA:                msg.SHA,
		BodyIncluded:       false,
		IdentifierIncluded: false,
	}
}

func writeSessionResumeAnchors(b *strings.Builder, anchors []SessionResumeAnchor) {
	if len(anchors) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, anchor := range anchors {
		fmt.Fprintf(b, "- kind=`%s` source=`%s`", anchor.Kind, anchor.Source)
		if anchor.Model != "" {
			fmt.Fprintf(b, " model=`%s`", inlineCode(anchor.Model))
		}
		if anchor.SHA != "" {
			fmt.Fprintf(b, " sha256_12=`%s`", anchor.SHA)
		}
		if anchor.PromptContextSHA != "" {
			fmt.Fprintf(b, " prompt_context_sha256_12=`%s`", anchor.PromptContextSHA)
		}
		if anchor.Kind == "latest-user" || anchor.Kind == "latest-assistant" {
			fmt.Fprintf(b, " trusted=`%t` edited=`%t` bytes=`%d` lines=`%d`", anchor.Trusted, anchor.Edited, anchor.Bytes, anchor.Lines)
		}
		if anchor.Kind == "latest-model-turn" {
			fmt.Fprintf(b, " context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s` usage_present=`%t` usage_total_tokens=`%d`",
				anchor.ContextDocuments,
				anchor.SelectedSkills,
				anchor.ToolOutputs,
				inlineListOrNone(anchor.Skills),
				inlineListOrNone(anchor.Tools),
				anchor.UsagePresent,
				anchor.UsageTotalTokens,
			)
		}
		fmt.Fprintf(b, " body_included=`%t` identifier_included=`%t`\n", anchor.BodyIncluded, anchor.IdentifierIncluded)
	}
}

func sessionResumeContinuationGate(report SessionResumeReport) string {
	if !report.NextIssueCommentResumesSession || !report.GitHubActionsReentrySupported {
		return "warn"
	}
	return "pass"
}

func sessionResumeLabelGate(report SessionResumeReport) string {
	switch {
	case report.DisabledLabelPresent:
		return "disabled"
	case report.ErrorLabelPresent || report.RunningLabelPresent:
		return "warn"
	default:
		return "pass"
	}
}

func sessionResumeMessageGate(msg SessionStatusMessage) string {
	if !msg.Present {
		return "missing"
	}
	return "pass"
}

func sessionResumeModelGate(report SessionResumeReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.ModelBackedAssistantTurns == 0 {
		return "warn"
	}
	return "pass"
}

func sessionResumePromptGate(report SessionResumeReport) string {
	if report.AssistantTurnComments == 0 {
		return "no-assistant-turns"
	}
	if report.AssistantTurnsWithPromptProvenance == 0 || report.AssistantTurnsMissingPromptProvenance > 0 {
		return "warn"
	}
	return "pass"
}
