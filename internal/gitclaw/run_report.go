package gitclaw

import (
	"fmt"
	"os"
	"strings"
)

func IsRunReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/runs" || command == "/run" || command == "/ledger"
}

func RenderRunReport(ev Event, cfg Config, decision PreflightDecision, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext, writeRequested bool) string {
	return renderRunReport(ev, cfg, decision, comments, transcript, repoContext, writeRequested, true)
}

func RenderRunCLIReport(cfg Config, repoContext RepoContext) string {
	return renderRunReport(Event{}, cfg, PreflightDecision{}, nil, nil, repoContext, false, false)
}

func renderRunReport(ev Event, cfg Config, decision PreflightDecision, comments []Comment, transcript []TranscriptMessage, repoContext RepoContext, writeRequested bool, includeIssue bool) string {
	counts := countSessionMarkers(comments)
	runID := "local"
	runAttempt := "0"
	runURL := ""
	if includeIssue {
		runID = envFirst("GITHUB_RUN_ID", runID)
		runAttempt = envFirst("GITHUB_RUN_ATTEMPT", runAttempt)
		runURL = actionRunURL(ev)
	}

	var b strings.Builder
	b.WriteString("## GitClaw Run Ledger Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
		fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
		fmt.Fprintf(&b, "- event_name: `%s`\n", ev.EventName)
		fmt.Fprintf(&b, "- event_action: `%s`\n", ev.Action)
		fmt.Fprintf(&b, "- event_id: `%s`\n", eventID(ev))
		fmt.Fprintf(&b, "- active_command: `%s`\n", activeSlashCommand(ev, cfg))
		fmt.Fprintf(&b, "- idempotency_key: `%s`\n", IdempotencyKey(ev))
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- run_id: `%s`\n", inlineCode(runID))
	fmt.Fprintf(&b, "- run_attempt: `%s`\n", inlineCode(runAttempt))
	fmt.Fprintf(&b, "- run_environment_sha256_12: `%s`\n", currentRunEnvironmentHash())
	fmt.Fprintf(&b, "- run_url_present: `%t`\n", runURL != "")
	if includeIssue {
		fmt.Fprintf(&b, "- run_url_sha256_12: `%s`\n", shortDocumentHash(runURL))
		fmt.Fprintf(&b, "- event_sha256_12: `%s`\n", shortDocumentHash(ev.SHA))
		fmt.Fprintf(&b, "- preflight_allowed: `%t`\n", decision.Allowed)
		fmt.Fprintf(&b, "- preflight_code: `%s`\n", decision.Code)
		actor := actorAssociation(ev)
		fmt.Fprintf(&b, "- actor_association: `%s`\n", actor)
		fmt.Fprintf(&b, "- actor_trusted: `%t`\n", trustedAssociation(actor, cfg))
		fmt.Fprintf(&b, "- triggered: `%t`\n", triggered(ev, cfg))
		fmt.Fprintf(&b, "- disabled_label_present: `%t`\n", hasLabel(ev.Issue.Labels, cfg.DisabledLabel))
		fmt.Fprintf(&b, "- write_request_detected: `%t`\n", writeRequested)
	}
	fmt.Fprintf(&b, "- raw_comments_before_turn: `%d`\n", len(comments))
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- user_messages: `%d`\n", countTranscriptRole(transcript, "user"))
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", countTranscriptRole(transcript, "assistant"))
	fmt.Fprintf(&b, "- assistant_turn_comments_before_turn: `%d`\n", counts.AssistantTurns)
	fmt.Fprintf(&b, "- heartbeat_comments_before_turn: `%d`\n", counts.Heartbeats)
	fmt.Fprintf(&b, "- error_marker_comments_before_turn: `%d`\n", counts.Errors)
	fmt.Fprintf(&b, "- channel_message_comments_before_turn: `%d`\n", counts.ChannelMessages)
	fmt.Fprintf(&b, "- context_documents: `%d`\n", len(repoContext.Documents))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", len(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- run_ledger_store: `%s`\n", "github-issue-comments+actions-run")
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- run_ledger_writes_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_run_payloads_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_runs_report_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report summarizes the current GitHub issue turn and Actions run provenance. It does not print issue bodies, comments, prompts, tool outputs, context file bodies, workflow payloads, or secrets.\n\n")

	b.WriteString("### Label State\n")
	if includeIssue {
		for _, label := range runLedgerLabels(cfg) {
			fmt.Fprintf(&b, "- `%s` present=`%t`\n", label, hasLabel(ev.Issue.Labels, label))
		}
	} else {
		b.WriteString("- issue labels unavailable in local CLI mode\n")
	}

	b.WriteString("\n### Prompt-Visible Inputs\n")
	writeRunLedgerDocumentList(&b, "context", repoContext.Documents)
	writeRunLedgerDocumentList(&b, "skill", repoContext.Skills)

	b.WriteString("\n### Tool Outputs\n")
	if len(repoContext.ToolOutputs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, output := range repoContext.ToolOutputs {
			fmt.Fprintf(&b, "- name=`%s` input_sha256_12=`%s` bytes=`%d` lines=`%d` output_sha256_12=`%s`\n", output.Name, shortDocumentHash(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output))
		}
	}

	b.WriteString("\n### Ledger Notes\n")
	b.WriteString("- issue comments remain the canonical conversation log\n")
	b.WriteString("- GitHub Actions remains the canonical execution trace\n")
	b.WriteString("- gitclaw-backups remains the canonical post-turn backup branch when enabled\n")

	return strings.TrimSpace(b.String())
}

func runLedgerLabels(cfg Config) []string {
	return []string{
		cfg.TriggerLabel,
		cfg.RunningLabel,
		cfg.DoneLabel,
		cfg.ErrorLabel,
		cfg.DisabledLabel,
		cfg.WriteRequestedLabel,
		cfg.HeartbeatLabel,
		cfg.ChannelLabel,
		cfg.ProactiveLabel,
	}
}

func writeRunLedgerDocumentList(b *strings.Builder, kind string, docs []ContextDocument) {
	if len(docs) == 0 {
		fmt.Fprintf(b, "- kind=`%s` none\n", kind)
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- kind=`%s` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", kind, doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
}

func currentRunEnvironmentHash() string {
	return shortDocumentHash(strings.Join([]string{
		os.Getenv("GITHUB_RUN_ID"),
		os.Getenv("GITHUB_RUN_ATTEMPT"),
		os.Getenv("GITHUB_WORKFLOW"),
		os.Getenv("GITHUB_JOB"),
	}, "\n"))
}
