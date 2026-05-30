package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type SessionRiskFinding struct {
	Severity     string
	Code         string
	Category     string
	Kind         string
	Source       string
	Field        string
	CommentID    int64
	MessageIndex int
	BodySHA      string
}

type SessionRiskReport struct {
	Status                                string
	VerificationScope                     string
	EventKind                             string
	RawComments                           int
	TranscriptMessages                    int
	UserMessages                          int
	AssistantMessages                     int
	TrustedMessages                       int
	UntrustedMessages                     int
	EditedMessages                        int
	AssistantTurnComments                 int
	AssistantTurnsWithPromptProvenance    int
	AssistantTurnsMissingPromptProvenance int
	UniquePromptContextHashes             int
	PromptVisibleSkillNames               []string
	PromptVisibleToolNames                []string
	HeartbeatComments                     int
	ErrorMarkerComments                   int
	ChannelMessageComments                int
	ChannelThreadIssue                    bool
	ProactiveRunIssue                     bool
	SurfacesWithRiskFindings              int
	Findings                              []SessionRiskFinding
	HighRiskFindings                      int
	WarningRiskFindings                   int
	InfoRiskFindings                      int
	RawIssueBodiesIncluded                bool
	RawCommentBodiesIncluded              bool
	RawAssistantBodiesIncluded            bool
	RawPromptBodiesIncluded               bool
	RawToolOutputsIncluded                bool
	RawSearchQueriesIncluded              bool
	CredentialValuesIncluded              bool
	LLME2ERequiredAfterSessionRiskChange  bool
}

func BuildSessionRiskReport(ev Event, comments []Comment, transcript []TranscriptMessage) SessionRiskReport {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	report := SessionRiskReport{
		Status:                                "ok",
		VerificationScope:                     "github_issue_session_provenance",
		EventKind:                             ev.Kind,
		RawComments:                           len(comments),
		TranscriptMessages:                    len(transcript),
		UserMessages:                          countTranscriptRole(transcript, "user"),
		AssistantMessages:                     countTranscriptRole(transcript, "assistant"),
		TrustedMessages:                       countTrustedTranscriptMessages(transcript, true),
		UntrustedMessages:                     countTrustedTranscriptMessages(transcript, false),
		EditedMessages:                        countEditedTranscriptMessages(transcript),
		AssistantTurnComments:                 counts.AssistantTurns,
		AssistantTurnsWithPromptProvenance:    provenance.TurnsWithProvenance,
		AssistantTurnsMissingPromptProvenance: provenance.PromptContextHashMissing,
		UniquePromptContextHashes:             provenance.UniquePromptContextSHAs,
		PromptVisibleSkillNames:               provenance.PromptVisibleSkillNames,
		PromptVisibleToolNames:                provenance.PromptVisibleToolNames,
		HeartbeatComments:                     counts.Heartbeats,
		ErrorMarkerComments:                   counts.Errors,
		ChannelMessageComments:                counts.ChannelMessages,
		ChannelThreadIssue:                    HasChannelThreadMarker(ev.Issue.Body),
		ProactiveRunIssue:                     HasProactiveRunMarker(ev.Issue.Body),
		RawIssueBodiesIncluded:                false,
		RawCommentBodiesIncluded:              false,
		RawAssistantBodiesIncluded:            false,
		RawPromptBodiesIncluded:               false,
		RawToolOutputsIncluded:                false,
		RawSearchQueriesIncluded:              false,
		CredentialValuesIncluded:              false,
		LLME2ERequiredAfterSessionRiskChange:  true,
	}
	report.Findings = buildSessionRiskFindings(report, comments, transcript, provenance)
	sortSessionRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = sessionRiskSurfaceCount(report.Findings)
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

func RenderSessionRiskReport(ev Event, comments []Comment, transcript []TranscriptMessage) string {
	return renderSessionRiskReport(ev, comments, transcript, true, "")
}

func RenderSessionRiskCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return renderSessionRiskReport(ev, commentsFromBackup(backup.Comments), backup.Transcript, false, backupPath)
}

func renderSessionRiskReport(ev Event, comments []Comment, transcript []TranscriptMessage, includeIssue bool, backupPath string) string {
	report := BuildSessionRiskReport(ev, comments, transcript)
	var b strings.Builder
	b.WriteString("## GitClaw Session Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-backup")
		fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(backupPath))
		fmt.Fprintf(&b, "- backup_repo: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- backup_issue: `#%d`\n", ev.Issue.Number)
	}
	writeSessionRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report audits the GitHub issue session boundary inspired by OpenClaw's transcript/session commands and Hermes' searchable saved sessions. It reports counts, marker/provenance metadata, trust state, finding codes, and hashes only; issue bodies, comment bodies, assistant replies, prompts, tool outputs, search queries, credentials, and secret values are not included.\n\n")

	b.WriteString("### Transcript Trust Risk Card\n")
	writeSessionTranscriptRiskCard(&b, report)

	b.WriteString("\n### Assistant Provenance Risk Card\n")
	writeSessionProvenanceRiskCard(&b, report)

	b.WriteString("\n### Marker Risk Card\n")
	writeSessionMarkerRiskCard(&b, report)

	b.WriteString("\n### Current Session Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-session-request` current_issue_session_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-session-request` scope=`local-backup` current_issue_session_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeSessionRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSessionRiskSummary(b *strings.Builder, report SessionRiskReport) {
	fmt.Fprintf(b, "- session_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- event_kind: `%s`\n", report.EventKind)
	fmt.Fprintf(b, "- raw_comments: `%d`\n", report.RawComments)
	fmt.Fprintf(b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(b, "- user_messages: `%d`\n", report.UserMessages)
	fmt.Fprintf(b, "- assistant_messages: `%d`\n", report.AssistantMessages)
	fmt.Fprintf(b, "- trusted_messages: `%d`\n", report.TrustedMessages)
	fmt.Fprintf(b, "- untrusted_messages: `%d`\n", report.UntrustedMessages)
	fmt.Fprintf(b, "- edited_messages: `%d`\n", report.EditedMessages)
	fmt.Fprintf(b, "- assistant_turn_comments: `%d`\n", report.AssistantTurnComments)
	fmt.Fprintf(b, "- assistant_turns_with_prompt_provenance: `%d`\n", report.AssistantTurnsWithPromptProvenance)
	fmt.Fprintf(b, "- assistant_turns_missing_prompt_provenance: `%d`\n", report.AssistantTurnsMissingPromptProvenance)
	fmt.Fprintf(b, "- unique_prompt_context_hashes: `%d`\n", report.UniquePromptContextHashes)
	fmt.Fprintf(b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(report.PromptVisibleSkillNames))
	fmt.Fprintf(b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(report.PromptVisibleToolNames))
	fmt.Fprintf(b, "- heartbeat_comments: `%d`\n", report.HeartbeatComments)
	fmt.Fprintf(b, "- error_marker_comments: `%d`\n", report.ErrorMarkerComments)
	fmt.Fprintf(b, "- channel_message_comments: `%d`\n", report.ChannelMessageComments)
	fmt.Fprintf(b, "- channel_thread_issue: `%t`\n", report.ChannelThreadIssue)
	fmt.Fprintf(b, "- proactive_run_issue: `%t`\n", report.ProactiveRunIssue)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- session_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_assistant_bodies_included: `%t`\n", report.RawAssistantBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(b, "- raw_search_queries_included: `%t`\n", report.RawSearchQueriesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_session_risk_change: `%t`\n", report.LLME2ERequiredAfterSessionRiskChange)
}

func writeSessionTranscriptRiskCard(b *strings.Builder, report SessionRiskReport) {
	findings := filterSessionRiskFindings(report.Findings, "session-transcript")
	fmt.Fprintf(
		b,
		"- kind=`session-transcript` transcript_messages=`%d` user_messages=`%d` assistant_messages=`%d` trusted_messages=`%d` untrusted_messages=`%d` edited_messages=`%d` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` evidence_hashes=`%s`\n",
		report.TranscriptMessages,
		report.UserMessages,
		report.AssistantMessages,
		report.TrustedMessages,
		report.UntrustedMessages,
		report.EditedMessages,
		len(findings),
		sessionRiskMaxSeverity(findings),
		inlineListOrNone(sessionRiskCodes(findings)),
		inlineListOrNone(sessionRiskEvidenceHashes(findings)),
	)
}

func writeSessionProvenanceRiskCard(b *strings.Builder, report SessionRiskReport) {
	findings := filterSessionRiskFindings(report.Findings, "session-provenance")
	fmt.Fprintf(
		b,
		"- kind=`session-provenance` assistant_turn_comments=`%d` assistant_turns_with_prompt_provenance=`%d` assistant_turns_missing_prompt_provenance=`%d` unique_prompt_context_hashes=`%d` skills=`%s` tools=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` evidence_hashes=`%s`\n",
		report.AssistantTurnComments,
		report.AssistantTurnsWithPromptProvenance,
		report.AssistantTurnsMissingPromptProvenance,
		report.UniquePromptContextHashes,
		inlineListOrNone(report.PromptVisibleSkillNames),
		inlineListOrNone(report.PromptVisibleToolNames),
		len(findings),
		sessionRiskMaxSeverity(findings),
		inlineListOrNone(sessionRiskCodes(findings)),
		inlineListOrNone(sessionRiskEvidenceHashes(findings)),
	)
}

func writeSessionMarkerRiskCard(b *strings.Builder, report SessionRiskReport) {
	findings := filterSessionRiskFindings(report.Findings, "session-marker")
	fmt.Fprintf(
		b,
		"- kind=`session-marker` heartbeat_comments=`%d` error_marker_comments=`%d` channel_message_comments=`%d` channel_thread_issue=`%t` proactive_run_issue=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` evidence_hashes=`%s`\n",
		report.HeartbeatComments,
		report.ErrorMarkerComments,
		report.ChannelMessageComments,
		report.ChannelThreadIssue,
		report.ProactiveRunIssue,
		len(findings),
		sessionRiskMaxSeverity(findings),
		inlineListOrNone(sessionRiskCodes(findings)),
		inlineListOrNone(sessionRiskEvidenceHashes(findings)),
	)
}

func buildSessionRiskFindings(report SessionRiskReport, comments []Comment, transcript []TranscriptMessage, provenance sessionPromptProvenanceReport) []SessionRiskFinding {
	var findings []SessionRiskFinding
	if report.TranscriptMessages == 0 {
		findings = append(findings, newSessionRiskFinding("high", "session_transcript_empty", "session-reconstruction", "session-transcript", "transcript", "messages", 0, 0, ""))
	}
	if report.AssistantTurnComments == 0 {
		findings = append(findings, newSessionRiskFinding("info", "assistant_turns_not_seen_yet", "session-provenance", "session-provenance", "assistant-turns", "assistant_turn_comments", 0, 0, ""))
	}
	if report.AssistantTurnsMissingPromptProvenance > 0 {
		findings = append(findings, newSessionRiskFinding("warning", "assistant_prompt_provenance_missing", "prompt-provenance", "session-provenance", "assistant-turns", "prompt_context_sha256_12", 0, 0, ""))
	}
	if report.AssistantTurnsWithPromptProvenance > 1 && report.UniquePromptContextHashes < report.AssistantTurnsWithPromptProvenance {
		findings = append(findings, newSessionRiskFinding("info", "prompt_context_hash_reused", "prompt-provenance", "session-provenance", "assistant-turns", "unique_prompt_context_hashes", 0, 0, ""))
	}
	if report.ErrorMarkerComments > 0 {
		findings = append(findings, newSessionRiskFinding("warning", "error_marker_comments_present", "session-health", "session-marker", "comments", "error_marker_comments", 0, 0, ""))
	}
	if report.HeartbeatComments > 0 {
		findings = append(findings, newSessionRiskFinding("info", "heartbeat_comments_present", "session-origin", "session-marker", "comments", "heartbeat_comments", 0, 0, ""))
	}
	if report.ChannelMessageComments > 0 {
		findings = append(findings, newSessionRiskFinding("info", "channel_message_markers_present", "session-origin", "session-marker", "comments", "channel_message_comments", 0, 0, ""))
	}
	if report.ChannelThreadIssue {
		findings = append(findings, newSessionRiskFinding("info", "channel_thread_issue", "session-origin", "session-marker", "issue", "channel_thread_issue", 0, 0, shortDocumentHash("channel-thread:"+report.EventKind)))
	}
	if report.ProactiveRunIssue {
		findings = append(findings, newSessionRiskFinding("info", "proactive_run_issue", "session-origin", "session-marker", "issue", "proactive_run_issue", 0, 0, shortDocumentHash("proactive-run:"+report.EventKind)))
	}
	for index, msg := range transcript {
		source := sessionMessageSource(msg)
		bodySHA := shortDocumentHash(msg.Body)
		if !msg.Trusted {
			findings = append(findings, newSessionRiskFinding("warning", "untrusted_session_message_visible", "trust-boundary", "session-transcript", source, "trusted", msg.CommentID, index+1, bodySHA))
		}
		if msg.Edited {
			findings = append(findings, newSessionRiskFinding("info", "edited_session_message_present", "session-mutation", "session-transcript", source, "edited", msg.CommentID, index+1, bodySHA))
		}
	}
	for _, turn := range provenance.Turns {
		if turn.HasPromptEvidence {
			continue
		}
		findings = append(findings, newSessionRiskFinding("warning", "assistant_turn_marker_without_prompt_context_hash", "prompt-provenance", "session-provenance", turn.Source, "prompt_context_sha256_12", 0, 0, shortDocumentHash(turn.Source+":missing-prompt-context")))
	}
	for _, comment := range comments {
		if HasGitClawErrorMarker(comment.Body) {
			findings = append(findings, newSessionRiskFinding("warning", "error_marker_comment", "session-health", "session-marker", fmt.Sprintf("comment:%d", comment.ID), "body", comment.ID, 0, shortDocumentHash(comment.Body)))
		}
	}
	sortSessionRiskFindings(findings)
	return findings
}

func newSessionRiskFinding(severity, code, category, kind, source, field string, commentID int64, messageIndex int, bodySHA string) SessionRiskFinding {
	if bodySHA == "" {
		bodySHA = shortDocumentHash(kind + ":" + source + ":" + field + ":" + code)
	}
	return SessionRiskFinding{
		Severity:     severity,
		Code:         code,
		Category:     category,
		Kind:         kind,
		Source:       source,
		Field:        field,
		CommentID:    commentID,
		MessageIndex: messageIndex,
		BodySHA:      bodySHA,
	}
}

func writeSessionRiskFindings(b *strings.Builder, findings []SessionRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` source=`%s` field=`%s` comment_id=`%d` message=`%02d` evidence_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Source, finding.Field, finding.CommentID, finding.MessageIndex, finding.BodySHA)
	}
}

func countEditedTranscriptMessages(transcript []TranscriptMessage) int {
	count := 0
	for _, msg := range transcript {
		if msg.Edited {
			count++
		}
	}
	return count
}

func filterSessionRiskFindings(findings []SessionRiskFinding, kind string) []SessionRiskFinding {
	var filtered []SessionRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			filtered = append(filtered, finding)
		}
	}
	sortSessionRiskFindings(filtered)
	return filtered
}

func sessionRiskSurfaceCount(findings []SessionRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Source
		if key == "\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func sessionRiskCodes(findings []SessionRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func sessionRiskEvidenceHashes(findings []SessionRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.BodySHA == "" || seen[finding.BodySHA] {
			continue
		}
		seen[finding.BodySHA] = true
		hashes = append(hashes, finding.BodySHA)
	}
	sort.Strings(hashes)
	return hashes
}

func sessionRiskMaxSeverity(findings []SessionRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if sessionRiskSeverityRank(finding.Severity) > sessionRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func sessionRiskSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortSessionRiskFindings(findings []SessionRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return sessionRiskSeverityRank(findings[i].Severity) > sessionRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Source != findings[j].Source {
			return findings[i].Source < findings[j].Source
		}
		if findings[i].Field != findings[j].Field {
			return findings[i].Field < findings[j].Field
		}
		return findings[i].MessageIndex < findings[j].MessageIndex
	})
}
