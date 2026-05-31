package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type TaskLedgerReport struct {
	Status                              string
	Source                              string
	BackupFile                          string
	Repository                          string
	IssueNumber                         int
	TaskPolicyPresent                   bool
	TaskPolicyLoadedForModel            bool
	TaskSpecs                           int
	TaskStorageBackend                  string
	CurrentIssueTask                    bool
	CurrentTaskStatus                   string
	CurrentTaskLabels                   int
	CommentsScanned                     int
	TranscriptMessages                  int
	UserComments                        int
	AssistantComments                   int
	AssistantTurns                      int
	ModelBackedTurns                    int
	DeterministicTurns                  int
	TurnsWithPromptProvenance           int
	ErrorMarkers                        int
	HeartbeatMarkers                    int
	ChannelThreadIssue                  bool
	ChannelMessageComments              int
	ProactiveRunIssue                   bool
	StatusHistoryAvailable              bool
	StatusTransitionSource              string
	TaskRiskStatus                      string
	TaskRiskFindings                    int
	RawTaskBodiesIncluded               bool
	RawIssueBodiesIncluded              bool
	RawCommentBodiesIncluded            bool
	RawAssistantRepliesIncluded         bool
	RawPromptsIncluded                  bool
	RawToolOutputsIncluded              bool
	LLME2ERequiredAfterTaskLedgerChange bool
	Entries                             []TaskLedgerEntry
}

type TaskLedgerEntry struct {
	Index             int
	Kind              string
	Source            string
	Status            string
	ActorSHA          string
	Association       string
	Labels            int
	BodySHA           string
	TitleSHA          string
	Model             string
	Deterministic     bool
	HasPromptEvidence bool
	PromptContextSHA  string
	ContextDocuments  int
	SelectedSkills    int
	ToolOutputs       int
	Skills            []string
	Tools             []string
	RunIDSHA          string
	EventIDSHA        string
	IdempotencyKeySHA string
	RunURLSHA         string
	MarkerSHA         string
}

func RenderTaskLedgerCLIReport(cfg Config) string {
	return renderTaskLedgerReport(BuildTaskLedgerReport("local-cli", "", cfg, Event{}, nil, nil, false), false)
}

func RenderTaskLedgerBackupCLIReport(cfg Config, backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind:      backup.EventName,
		EventName: backup.EventName,
		Repo:      backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
			User:   User{Login: backup.Issue.Author},
			Labels: append([]string(nil), backup.Issue.Labels...),
		},
	}
	return renderTaskLedgerReport(BuildTaskLedgerReport("local-backup", backupPath, cfg, ev, commentsFromBackup(backup.Comments), backup.Transcript, true), false)
}

func renderTaskLedgerReport(report TaskLedgerReport, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Task Ledger Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", report.Repository)
		fmt.Fprintf(&b, "- issue: `#%d`\n", report.IssueNumber)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", report.Source)
		if report.BackupFile != "" {
			fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(report.BackupFile))
			fmt.Fprintf(&b, "- backup_repo: `%s`\n", report.Repository)
			fmt.Fprintf(&b, "- backup_issue: `#%d`\n", report.IssueNumber)
		}
	}
	writeTaskLedgerSummary(&b, report)
	b.WriteByte('\n')
	b.WriteString("This report treats the GitHub issue as the durable task row and issue comments as the task handoff log. It reports status, marker counts, hashes, and provenance metadata only; task bodies, issue bodies, comments, assistant replies, prompts, and tool outputs are not included.\n\n")

	b.WriteString("### Ledger Entries\n")
	writeTaskLedgerEntries(&b, report.Entries)

	b.WriteString("\n### Ledger Gates\n")
	b.WriteString("- task_storage_backend=`github-issues`\n")
	b.WriteString("- detached_worker_supported=`false`\n")
	b.WriteString("- kanban_dispatcher_supported=`false`\n")
	b.WriteString("- task_flow_execution_supported=`false`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- status_history_gate=`current_labels_only`\n")
	return strings.TrimSpace(b.String())
}

func BuildTaskLedgerReport(source, backupFile string, cfg Config, ev Event, comments []Comment, transcript []TranscriptMessage, includeIssue bool) TaskLedgerReport {
	surface := inspectTaskSurface(cfg.Workdir)
	risk := BuildTaskRiskReport(cfg, comments, transcript, includeIssue)
	report := TaskLedgerReport{
		Source:                              source,
		BackupFile:                          backupFile,
		Repository:                          ev.Repo,
		IssueNumber:                         ev.Issue.Number,
		TaskPolicyPresent:                   surface.Policy.Present,
		TaskPolicyLoadedForModel:            taskPolicyLoadedForModel(surface),
		TaskSpecs:                           len(surface.Specs),
		TaskStorageBackend:                  "github-issues",
		CurrentIssueTask:                    includeIssue,
		CurrentTaskLabels:                   len(ev.Issue.Labels),
		CommentsScanned:                     len(comments),
		TranscriptMessages:                  len(transcript),
		ChannelThreadIssue:                  includeIssue && HasChannelThreadMarker(ev.Issue.Body),
		ProactiveRunIssue:                   includeIssue && HasProactiveRunMarker(ev.Issue.Body),
		StatusHistoryAvailable:              false,
		StatusTransitionSource:              "current-labels-and-markers",
		TaskRiskStatus:                      risk.Status,
		TaskRiskFindings:                    len(risk.Findings),
		LLME2ERequiredAfterTaskLedgerChange: true,
	}
	findings := taskFindings(surface)
	report.Status = taskLedgerStatus(taskStatus(surface, findings), risk.Status, includeIssue, comments)
	if includeIssue {
		report.CurrentTaskStatus = currentIssueTaskStatus(ev, cfg)
		report.Entries = append(report.Entries, TaskLedgerEntry{
			Index:       len(report.Entries) + 1,
			Kind:        "current-task",
			Source:      "issue",
			Status:      report.CurrentTaskStatus,
			ActorSHA:    shortDocumentHash(ev.Issue.User.Login),
			Association: ev.Issue.AuthorAssociation,
			Labels:      len(ev.Issue.Labels),
			TitleSHA:    shortDocumentHash(ev.Issue.Title),
			BodySHA:     shortDocumentHash(ev.Issue.Body),
		})
	}
	for _, comment := range comments {
		entry := taskLedgerEntryFromComment(comment, len(report.Entries)+1)
		report.Entries = append(report.Entries, entry)
		if entry.Kind == "assistant-turn" {
			report.AssistantTurns++
			if entry.HasPromptEvidence {
				report.TurnsWithPromptProvenance++
			}
			if entry.Deterministic {
				report.DeterministicTurns++
			} else if entry.Model != "" {
				report.ModelBackedTurns++
			}
		}
		if entry.Kind == "error" {
			report.ErrorMarkers++
		}
		if entry.Kind == "heartbeat" {
			report.HeartbeatMarkers++
		}
		if entry.Kind == "channel-message" {
			report.ChannelMessageComments++
		}
		if entry.Kind == "assistant-turn" || entry.Kind == "error" || comment.User.IsBot() {
			report.AssistantComments++
		} else {
			report.UserComments++
		}
	}
	sortTaskLedgerEntries(report.Entries)
	return report
}

func taskLedgerEntryFromComment(comment Comment, index int) TaskLedgerEntry {
	entry := TaskLedgerEntry{
		Index:       index,
		Kind:        "user-comment",
		Source:      fmt.Sprintf("comment:%d", comment.ID),
		ActorSHA:    shortDocumentHash(comment.User.Login),
		Association: comment.AuthorAssociation,
		BodySHA:     shortDocumentHash(comment.Body),
	}
	if runEntry, ok := parseRunHistoryEntry(comment, index); ok {
		entry.Kind = "assistant-turn"
		entry.Model = runEntry.Model
		entry.Deterministic = runEntry.Deterministic
		entry.HasPromptEvidence = runEntry.HasPromptEvidence
		entry.PromptContextSHA = runEntry.PromptContextSHA
		entry.ContextDocuments = runEntry.ContextDocuments
		entry.SelectedSkills = runEntry.SelectedSkills
		entry.ToolOutputs = runEntry.ToolOutputs
		entry.Skills = append([]string(nil), runEntry.Skills...)
		entry.Tools = append([]string(nil), runEntry.Tools...)
		entry.IdempotencyKeySHA = runEntry.IdempotencyKeySHA
		entry.RunURLSHA = runEntry.RunURLSHA
		entry.MarkerSHA = runEntry.CommentSHA
		if runEntry.RunID != "" {
			entry.RunIDSHA = shortDocumentHash(runEntry.RunID)
		}
		if runEntry.EventID != "" {
			entry.EventIDSHA = shortDocumentHash(runEntry.EventID)
		}
		return entry
	}
	if match := errorMarkerPattern.FindStringSubmatch(comment.Body); len(match) >= 2 {
		attrs := match[1]
		entry.Kind = "error"
		entry.Status = markerAttribute(attrs, "phase")
		entry.RunIDSHA = shortDocumentHash(markerAttribute(attrs, "run_id"))
		entry.EventIDSHA = shortDocumentHash(markerAttribute(attrs, "event_id"))
		entry.RunURLSHA = shortDocumentHash(markerAttribute(attrs, "run_url"))
		entry.MarkerSHA = shortDocumentHash(match[0])
		return entry
	}
	if match := heartbeatMarkerPattern.FindStringSubmatch(comment.Body); len(match) >= 2 {
		attrs := match[1]
		entry.Kind = "heartbeat"
		entry.Status = markerAttribute(attrs, "slot")
		entry.RunIDSHA = shortDocumentHash(markerAttribute(attrs, "run_id"))
		entry.RunURLSHA = shortDocumentHash(markerAttribute(attrs, "run_url"))
		entry.MarkerSHA = shortDocumentHash(match[0])
		return entry
	}
	if match := channelMessageMarkerPattern.FindStringSubmatch(comment.Body); len(match) >= 2 {
		attrs := match[1]
		entry.Kind = "channel-message"
		entry.Status = markerAttribute(attrs, "provider")
		entry.RunIDSHA = shortDocumentHash(markerAttribute(attrs, "dispatch_id"))
		entry.EventIDSHA = shortDocumentHash(markerAttribute(attrs, "message_id"))
		entry.MarkerSHA = shortDocumentHash(match[0])
		return entry
	}
	return entry
}

func writeTaskLedgerSummary(b *strings.Builder, report TaskLedgerReport) {
	fmt.Fprintf(b, "- task_ledger_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- ledger_source: `%s`\n", report.Source)
	fmt.Fprintf(b, "- task_policy_present: `%t`\n", report.TaskPolicyPresent)
	fmt.Fprintf(b, "- task_policy_loaded_for_model: `%t`\n", report.TaskPolicyLoadedForModel)
	fmt.Fprintf(b, "- task_specs: `%d`\n", report.TaskSpecs)
	fmt.Fprintf(b, "- task_storage_backend: `%s`\n", report.TaskStorageBackend)
	fmt.Fprintf(b, "- current_issue_task: `%t`\n", report.CurrentIssueTask)
	fmt.Fprintf(b, "- current_task_status: `%s`\n", inlineCode(report.CurrentTaskStatus))
	fmt.Fprintf(b, "- current_task_labels: `%d`\n", report.CurrentTaskLabels)
	fmt.Fprintf(b, "- comments_scanned: `%d`\n", report.CommentsScanned)
	fmt.Fprintf(b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(b, "- user_comments: `%d`\n", report.UserComments)
	fmt.Fprintf(b, "- assistant_comments: `%d`\n", report.AssistantComments)
	fmt.Fprintf(b, "- assistant_turns: `%d`\n", report.AssistantTurns)
	fmt.Fprintf(b, "- model_backed_turns: `%d`\n", report.ModelBackedTurns)
	fmt.Fprintf(b, "- deterministic_turns: `%d`\n", report.DeterministicTurns)
	fmt.Fprintf(b, "- turns_with_prompt_provenance: `%d`\n", report.TurnsWithPromptProvenance)
	fmt.Fprintf(b, "- error_markers: `%d`\n", report.ErrorMarkers)
	fmt.Fprintf(b, "- heartbeat_markers: `%d`\n", report.HeartbeatMarkers)
	fmt.Fprintf(b, "- channel_thread_issue: `%t`\n", report.ChannelThreadIssue)
	fmt.Fprintf(b, "- channel_message_comments: `%d`\n", report.ChannelMessageComments)
	fmt.Fprintf(b, "- proactive_run_issue: `%t`\n", report.ProactiveRunIssue)
	fmt.Fprintf(b, "- status_history_available: `%t`\n", report.StatusHistoryAvailable)
	fmt.Fprintf(b, "- status_transition_source: `%s`\n", report.StatusTransitionSource)
	fmt.Fprintf(b, "- task_risk_status: `%s`\n", report.TaskRiskStatus)
	fmt.Fprintf(b, "- task_risk_findings: `%d`\n", report.TaskRiskFindings)
	fmt.Fprintf(b, "- raw_task_bodies_included: `%t`\n", report.RawTaskBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_assistant_replies_included: `%t`\n", report.RawAssistantRepliesIncluded)
	fmt.Fprintf(b, "- raw_prompts_included: `%t`\n", report.RawPromptsIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_task_ledger_change: `%t`\n", report.LLME2ERequiredAfterTaskLedgerChange)
}

func writeTaskLedgerEntries(b *strings.Builder, entries []TaskLedgerEntry) {
	if len(entries) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, entry := range entries {
		fmt.Fprintf(
			b,
			"- index=`%d` kind=`%s` source=`%s` status=`%s` actor_sha256_12=`%s` association=`%s` labels=`%d` title_sha256_12=`%s` body_sha256_12=`%s` model=`%s` deterministic=`%t` has_prompt_evidence=`%t` prompt_context_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s` run_id_sha256_12=`%s` event_id_sha256_12=`%s` idempotency_key_sha256_12=`%s` run_url_sha256_12=`%s` marker_sha256_12=`%s`\n",
			entry.Index,
			entry.Kind,
			entry.Source,
			inlineCode(entry.Status),
			entry.ActorSHA,
			inlineCode(entry.Association),
			entry.Labels,
			entry.TitleSHA,
			entry.BodySHA,
			inlineCode(entry.Model),
			entry.Deterministic,
			entry.HasPromptEvidence,
			inlineCode(entry.PromptContextSHA),
			entry.ContextDocuments,
			entry.SelectedSkills,
			entry.ToolOutputs,
			inlineListOrNone(entry.Skills),
			inlineListOrNone(entry.Tools),
			entry.RunIDSHA,
			entry.EventIDSHA,
			entry.IdempotencyKeySHA,
			entry.RunURLSHA,
			entry.MarkerSHA,
		)
	}
}

func sortTaskLedgerEntries(entries []TaskLedgerEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Index < entries[j].Index
	})
}

func taskLedgerStatus(taskStatusValue, riskStatus string, includeIssue bool, comments []Comment) string {
	status := taskStatusValue
	if status == "" {
		status = "ok"
	}
	if status != "error" {
		switch riskStatus {
		case "high":
			status = "high"
		case "warn":
			if status == "ok" {
				status = "warn"
			}
		}
	}
	if !includeIssue && len(comments) == 0 && status == "ok" {
		return "local"
	}
	return status
}

func isTaskLedgerRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	command := fields[0]
	if command != "/tasks" && command != "/task" {
		return false
	}
	subcommand := strings.Trim(strings.ToLower(fields[1]), " \t\r\n.,:;!?")
	return subcommand == "ledger" || subcommand == "timeline" || subcommand == "history"
}
