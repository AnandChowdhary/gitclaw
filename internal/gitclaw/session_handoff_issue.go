package gitclaw

import (
	"context"
	"fmt"
	"strings"
)

const sessionHandoffIssueMarker = "gitclaw:session-handoff-issue"

type SessionHandoffIssueGitHubClient interface {
	CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error)
	ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error)
}

type SessionHandoffIssueRequest struct {
	Repo              string
	Command           string
	Subcommand        string
	HandoffID         string
	SourceIssueNumber int
	SourceCommentID   int64
	SourceSHA         string
	SourceBytes       int
	SourceLines       int
	SourceKind        string
	Resume            SessionResumeReport
}

type SessionHandoffIssueResult struct {
	IssueNumber int
	IssueURL    string
	Created     bool
	Duplicate   bool
}

func IsSessionHandoffIssueRequest(ev Event, cfg Config) bool {
	return isSessionHandoffIssueFields(activeSlashCommandFields(ev, cfg))
}

func isSessionHandoffIssueFields(fields []string) bool {
	if len(fields) < 2 || (fields[0] != "/session" && fields[0] != "/sessions") {
		return false
	}
	switch cleanSessionHandoffCommandName(fields[1]) {
	case "handoff", "fork", "transfer", "new-thread", "new-issue", "continue-here":
		return true
	default:
		return false
	}
}

func BuildSessionHandoffIssueRequest(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) (SessionHandoffIssueRequest, error) {
	fields := activeSlashCommandFields(ev, cfg)
	if !isSessionHandoffIssueFields(fields) {
		return SessionHandoffIssueRequest{}, fmt.Errorf("missing session handoff command")
	}
	sourceText := activeRequestText(ev)
	handoffID, err := parseSessionHandoffIssueArgs(fields[2:], ev.Issue.Number, sourceText)
	if err != nil {
		return SessionHandoffIssueRequest{}, err
	}
	if handoffID == "" {
		handoffID = cleanSessionHandoffID(fmt.Sprintf("session-handoff-%d-%s", ev.Issue.Number, shortDocumentHash(sourceText)))
	}
	if !skillNamePattern.MatchString(handoffID) {
		return SessionHandoffIssueRequest{}, fmt.Errorf("invalid session handoff id %q", handoffID)
	}
	resume := BuildSessionResumeReport("issue-thread", "", ev, cfg, comments, transcript)
	resume.ActiveCommand = strings.Join(fields, " ")
	req := SessionHandoffIssueRequest{
		Repo:              ev.Repo,
		Command:           fields[0],
		Subcommand:        cleanSessionHandoffCommandName(fields[1]),
		HandoffID:         handoffID,
		SourceIssueNumber: ev.Issue.Number,
		SourceSHA:         shortDocumentHash(sourceText),
		SourceBytes:       len(sourceText),
		SourceLines:       lineCount(sourceText),
		SourceKind:        "issue",
		Resume:            resume,
	}
	if ev.Comment != nil {
		req.SourceKind = "comment"
		req.SourceCommentID = ev.Comment.ID
	}
	return req, nil
}

func RunSessionHandoffIssue(ctx context.Context, cfg Config, github SessionHandoffIssueGitHubClient, req SessionHandoffIssueRequest) (SessionHandoffIssueResult, error) {
	if err := validateRepoName(req.Repo); err != nil {
		return SessionHandoffIssueResult{}, err
	}
	if req.HandoffID == "" {
		return SessionHandoffIssueResult{}, fmt.Errorf("missing session handoff id")
	}
	issues, err := github.ListOpenIssues(ctx, req.Repo, []string{cfg.TriggerLabel}, 300)
	if err != nil {
		return SessionHandoffIssueResult{}, fmt.Errorf("list session handoff issues: %w", err)
	}
	for _, issue := range issues {
		if issue.IsPullRequest {
			continue
		}
		if sessionHandoffIssueMatches(issue.Body, req.HandoffID) {
			return SessionHandoffIssueResult{
				IssueNumber: issue.Number,
				IssueURL:    issueURL(req.Repo, issue.Number),
				Duplicate:   true,
			}, nil
		}
	}
	issue, err := github.CreateIssue(ctx, req.Repo, sessionHandoffIssueTitle(req), RenderSessionHandoffIssueBody(req), []string{cfg.TriggerLabel})
	if err != nil {
		return SessionHandoffIssueResult{}, fmt.Errorf("create session handoff issue: %w", err)
	}
	return SessionHandoffIssueResult{
		IssueNumber: issue.Number,
		IssueURL:    issueURL(req.Repo, issue.Number),
		Created:     true,
	}, nil
}

func RenderSessionHandoffIssueBody(req SessionHandoffIssueRequest) string {
	report := req.Resume
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- %s id=\"%s\" source_issue=\"%d\" source_comment_id=\"%d\" source_sha256_12=\"%s\" -->\n", sessionHandoffIssueMarker, escapeMarkerValue(req.HandoffID), req.SourceIssueNumber, req.SourceCommentID, escapeMarkerValue(req.SourceSHA))
	b.WriteString("GitClaw session handoff issue.\n\n")
	fmt.Fprintf(&b, "- handoff_id: %s\n", req.HandoffID)
	fmt.Fprintf(&b, "- handoff_mode: %s\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- source_issue: #%d\n", req.SourceIssueNumber)
	fmt.Fprintf(&b, "- source_issue_url: %s\n", issueURL(req.Repo, req.SourceIssueNumber))
	fmt.Fprintf(&b, "- source_comment_id: %d\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- source_kind: %s\n", req.SourceKind)
	fmt.Fprintf(&b, "- source_sha256_12: %s\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_session_store: %s\n", "github-issue-thread")
	fmt.Fprintf(&b, "- transcript_messages: %d\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: %d\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: %d\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: %d\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: %d\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: %d\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: %d\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: %d\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: %d\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: %s\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: %s\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: %s\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- latest_user_message_sha256_12: %s\n", valueOrNone(report.LatestUser.SHA))
	fmt.Fprintf(&b, "- latest_assistant_message_sha256_12: %s\n", valueOrNone(report.LatestAssistant.SHA))
	fmt.Fprintf(&b, "- latest_assistant_prompt_context_sha256_12: %s\n", valueOrNone(report.LatestAssistantPromptContextSHA))
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: %d\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(&b, "- usage_total_tokens: %d\n", report.UsageTotalTokens)
	b.WriteString("- next_issue_comment_resumes_handoff: true\n")
	b.WriteString("- source_issue_continuation_supported: true\n")
	b.WriteString("- github_actions_reentry_supported: true\n")
	b.WriteString("- workflow_event: issue_comment\n")
	b.WriteString("- workflow_dispatch_required: false\n")
	b.WriteString("- server_required: false\n")
	b.WriteString("- socket_required: false\n")
	b.WriteString("- external_session_db_required: false\n")
	b.WriteString("- backup_branch_replay_preferred: true\n")
	b.WriteString("- repository_mutation_allowed: false\n")
	b.WriteString("- handoff_mutation_allowed: false\n")
	b.WriteString("- raw_source_body_included: false\n")
	b.WriteString("- raw_issue_bodies_included: false\n")
	b.WriteString("- raw_comment_bodies_included: false\n")
	b.WriteString("- raw_assistant_replies_included: false\n")
	b.WriteString("- raw_prompts_included: false\n")
	b.WriteString("- raw_tool_outputs_included: false\n\n")
	b.WriteString("Continue here with a normal `@gitclaw` message. This issue is a body-free handoff lane: it preserves source session counts, hashes, marker provenance, and reentry gates without copying source issue bodies, comments, prompts, assistant replies, or tool outputs.\n")
	return strings.TrimSpace(b.String())
}

func RenderSessionHandoffIssueActionReport(ev Event, req SessionHandoffIssueRequest, result SessionHandoffIssueResult) string {
	status := "created"
	if result.Duplicate {
		status = "existing"
	}
	report := req.Resume
	var b strings.Builder
	b.WriteString("## GitClaw Session Handoff Issue Action\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- source_issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- source_comment_id: `%d`\n", req.SourceCommentID)
	fmt.Fprintf(&b, "- requested_session_command: `%s %s`\n", req.Command, req.Subcommand)
	fmt.Fprintf(&b, "- session_handoff_status: `%s`\n", status)
	fmt.Fprintf(&b, "- handoff_issue: `#%d`\n", result.IssueNumber)
	fmt.Fprintf(&b, "- handoff_issue_url: `%s`\n", result.IssueURL)
	fmt.Fprintf(&b, "- handoff_issue_created: `%t`\n", result.Created)
	fmt.Fprintf(&b, "- duplicate_suppressed: `%t`\n", result.Duplicate)
	fmt.Fprintf(&b, "- handoff_id_sha256_12: `%s`\n", shortDocumentHash(req.HandoffID))
	fmt.Fprintf(&b, "- handoff_mode: `%s`\n", "github-issue-conversation")
	fmt.Fprintf(&b, "- handoff_issue_labeled_for_gitclaw: `%t`\n", true)
	fmt.Fprintf(&b, "- source_session_store: `%s`\n", "github-issue-thread")
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(&b, "- model_backed_assistant_turns: `%d`\n", report.ModelBackedAssistantTurns)
	fmt.Fprintf(&b, "- deterministic_assistant_turns: `%d`\n", report.DeterministicAssistantTurns)
	fmt.Fprintf(&b, "- model_names: `%s`\n", inlineListOrNone(report.ModelNames))
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- usage_bearing_assistant_turns: `%d`\n", report.UsageBearingAssistantTurns)
	fmt.Fprintf(&b, "- usage_total_tokens: `%d`\n", report.UsageTotalTokens)
	fmt.Fprintf(&b, "- next_issue_comment_resumes_handoff: `%t`\n", true)
	fmt.Fprintf(&b, "- source_issue_continuation_supported: `%t`\n", true)
	fmt.Fprintf(&b, "- github_actions_reentry_supported: `%t`\n", true)
	fmt.Fprintf(&b, "- workflow_event: `%s`\n", "issue_comment")
	fmt.Fprintf(&b, "- workflow_dispatch_required: `%t`\n", false)
	fmt.Fprintf(&b, "- server_required: `%t`\n", false)
	fmt.Fprintf(&b, "- socket_required: `%t`\n", false)
	fmt.Fprintf(&b, "- external_session_db_required: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_branch_replay_preferred: `%t`\n", true)
	fmt.Fprintf(&b, "- model_call_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_handoff_id_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_assistant_replies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_prompts_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- source_sha256_12: `%s`\n", req.SourceSHA)
	fmt.Fprintf(&b, "- source_bytes: `%d`\n", req.SourceBytes)
	fmt.Fprintf(&b, "- source_lines: `%d`\n", req.SourceLines)
	fmt.Fprintf(&b, "- llm_e2e_required_after_session_handoff_issue_change: `%t`\n", true)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	b.WriteByte('\n')
	b.WriteString("GitClaw opened or reused a labeled GitHub issue for continuing this session in a new lane. The action does not copy raw source bodies, comments, assistant replies, prompts, or tool outputs; the handoff issue carries counts, hashes, marker metadata, and reentry gates only.\n\n")
	b.WriteString("### Handoff Path\n")
	fmt.Fprintf(&b, "- continue on handoff issue: `#%d`\n", result.IssueNumber)
	b.WriteString("- ask a normal `@gitclaw` follow-up there to prove the new lane has model, skill, tool, and usage telemetry\n")
	b.WriteString("- use `/session resume`, `/session provenance`, or `/session coverage` on either issue to audit the source and handoff sessions\n")
	return strings.TrimSpace(b.String())
}

func parseSessionHandoffIssueArgs(args []string, defaultIssue int, sourceText string) (string, error) {
	handoffID := ""
	for i := 0; i < len(args); i++ {
		field := strings.TrimSpace(args[i])
		if field == "" {
			continue
		}
		switch field {
		case "--id", "--handoff-id":
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s requires a value", field)
			}
			handoffID = cleanSessionHandoffID(args[i])
		default:
			if strings.HasPrefix(field, "--") {
				return "", fmt.Errorf("unknown session handoff argument %q", field)
			}
			if handoffID == "" {
				handoffID = cleanSessionHandoffID(field)
			}
		}
	}
	if handoffID == "" {
		handoffID = cleanSessionHandoffID(fmt.Sprintf("session-handoff-%d-%s", defaultIssue, shortDocumentHash(sourceText)))
	}
	return handoffID, nil
}

func cleanSessionHandoffID(value string) string {
	return cleanSkillRehearsalID(value)
}

func cleanSessionHandoffCommandName(value string) string {
	value = strings.ToLower(strings.Trim(value, " \t\r\n.,:;!?"))
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "hand-off":
		return "handoff"
	default:
		return value
	}
}

func sessionHandoffIssueTitle(req SessionHandoffIssueRequest) string {
	title := fmt.Sprintf("GitClaw session handoff: issue #%d", req.SourceIssueNumber)
	if req.HandoffID != "" {
		title += " (" + req.HandoffID + ")"
	}
	if len(title) > 120 {
		title = strings.TrimSpace(title[:120])
	}
	return title
}

func sessionHandoffIssueMatches(body, handoffID string) bool {
	return strings.Contains(body, "<!-- "+sessionHandoffIssueMarker+" ") &&
		strings.Contains(body, fmt.Sprintf(`id="%s"`, escapeMarkerValue(cleanSessionHandoffID(handoffID))))
}
