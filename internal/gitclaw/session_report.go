package gitclaw

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type sessionMarkerCounts struct {
	AssistantTurns  int
	Heartbeats      int
	Errors          int
	ChannelMessages int
}

const defaultSessionSearchMaxResults = 10

type SessionSearchReport struct {
	QueryHash          string
	QueryTerms         int
	SearchStatus       string
	MaxResults         int
	TranscriptMessages int
	MatchedMessages    int
	MatchedLines       int
	ResultsReturned    int
	RawBodiesIncluded  bool
	Results            []SessionSearchResult
}

type SessionSearchResult struct {
	MessageIndex      int
	Role              string
	Source            string
	Actor             string
	AuthorAssociation string
	Trusted           bool
	Edited            bool
	Line              int
	Score             int
	MatchedTerms      int
	MessageBytes      int
	MessageLines      int
	MessageSHA        string
	LineSHA           string
}

func IsSessionReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/session"
}

func RenderSessionReport(ev Event, cfg Config, comments []Comment, transcript []TranscriptMessage) string {
	if requestedSessionCoverage(ev, cfg) {
		report := BuildSessionCoverageReport("issue-thread", "", ev, comments, transcript, DefaultSessionCoverageRequirements())
		return RenderSessionCoverageReport(report)
	}
	if requestedSessionStats(ev, cfg) {
		return RenderSessionStatsReport(BuildSessionStatsReport("issue-thread", "", ev, comments, transcript))
	}
	if requestedSessionRisk(ev, cfg) {
		return RenderSessionRiskReport(ev, comments, transcript)
	}
	if query := requestedSessionSearchQuery(ev, cfg); query != "" {
		return RenderSessionSearchReport(ev, transcript, query, defaultSessionSearchMaxResults)
	}
	return renderSessionReport(ev, comments, transcript, true, "")
}

func RenderSessionCLIReport(backupPath string, backup IssueBackup) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return renderSessionReport(ev, commentsFromBackup(backup.Comments), backup.Transcript, false, backupPath)
}

func renderSessionReport(ev Event, comments []Comment, transcript []TranscriptMessage, includeIssue bool, backupPath string) string {
	counts := countSessionMarkers(comments)
	provenance := buildSessionPromptProvenanceReport(comments)
	var b strings.Builder
	b.WriteString("## GitClaw Session Report\n\n")
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
	fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", len(comments))
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- user_messages: `%d`\n", countTranscriptRole(transcript, "user"))
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", countTranscriptRole(transcript, "assistant"))
	fmt.Fprintf(&b, "- trusted_messages: `%d`\n", countTrustedTranscriptMessages(transcript, true))
	fmt.Fprintf(&b, "- untrusted_messages: `%d`\n", countTrustedTranscriptMessages(transcript, false))
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", counts.AssistantTurns)
	fmt.Fprintf(&b, "- assistant_turns_with_prompt_provenance: `%d`\n", provenance.TurnsWithProvenance)
	fmt.Fprintf(&b, "- assistant_turns_missing_prompt_provenance: `%d`\n", provenance.PromptContextHashMissing)
	fmt.Fprintf(&b, "- unique_prompt_context_hashes: `%d`\n", provenance.UniquePromptContextSHAs)
	fmt.Fprintf(&b, "- prompt_visible_skill_names: `%s`\n", inlineListOrNone(provenance.PromptVisibleSkillNames))
	fmt.Fprintf(&b, "- prompt_visible_tool_names: `%s`\n", inlineListOrNone(provenance.PromptVisibleToolNames))
	fmt.Fprintf(&b, "- heartbeat_comments: `%d`\n", counts.Heartbeats)
	fmt.Fprintf(&b, "- error_marker_comments: `%d`\n", counts.Errors)
	fmt.Fprintf(&b, "- channel_message_comments: `%d`\n", counts.ChannelMessages)
	fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
	fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n\n", HasProactiveRunMarker(ev.Issue.Body))
	b.WriteString("Message bodies are not included; hashes let maintainers verify exactly which issue-thread messages were loaded.\n\n")

	b.WriteString("### Transcript Messages\n")
	writeTranscriptMessageList(&b, transcript)

	b.WriteString("\n### Assistant Turn Provenance\n")
	writeSessionPromptProvenanceList(&b, provenance.Turns)

	return strings.TrimSpace(b.String())
}

func ReadIssueBackupFile(path string) (IssueBackup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IssueBackup{}, fmt.Errorf("read issue backup %s: %w", path, err)
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return IssueBackup{}, fmt.Errorf("parse issue backup %s: %w", path, err)
	}
	if backup.Repo == "" || backup.Issue.Number == 0 {
		return IssueBackup{}, fmt.Errorf("issue backup %s is missing repo or issue number", path)
	}
	return backup, nil
}

func commentsFromBackup(comments []IssueBackupComment) []Comment {
	out := make([]Comment, 0, len(comments))
	for _, comment := range comments {
		out = append(out, Comment{
			ID:                comment.ID,
			Body:              comment.Body,
			AuthorAssociation: comment.AuthorAssociation,
			User:              User{Login: comment.Author},
			CreatedAt:         comment.CreatedAt,
			UpdatedAt:         comment.UpdatedAt,
		})
	}
	return out
}

func RenderSessionSearchReport(ev Event, transcript []TranscriptMessage, query string, maxResults int) string {
	return renderSessionSearchReport(ev, transcript, query, maxResults, true, "")
}

func RenderSessionSearchCLIReport(backupPath string, backup IssueBackup, query string, maxResults int) string {
	ev := Event{
		Kind: backup.EventName,
		Repo: backup.Repo,
		Issue: Issue{
			Number: backup.Issue.Number,
			Title:  backup.Issue.Title,
			Body:   backup.Issue.Body,
		},
	}
	return renderSessionSearchReport(ev, backup.Transcript, query, maxResults, false, backupPath)
}

func renderSessionSearchReport(ev Event, transcript []TranscriptMessage, query string, maxResults int, includeIssue bool, backupPath string) string {
	report := BuildSessionSearchReport(transcript, query, maxResults)
	var b strings.Builder
	b.WriteString("## GitClaw Session Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue && ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if includeIssue && ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else if includeIssue {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-backup")
		fmt.Fprintf(&b, "- backup_file: `%s`\n", inlineCode(backupPath))
		fmt.Fprintf(&b, "- backup_repo: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- backup_issue: `#%d`\n", ev.Issue.Number)
	}
	fmt.Fprintf(&b, "- session_search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", report.TranscriptMessages)
	fmt.Fprintf(&b, "- matched_messages: `%d`\n", report.MatchedMessages)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", report.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", report.RawBodiesIncluded)
	b.WriteString("This report searches the reconstructed GitHub issue transcript with a local lexical matcher. It reports message indexes, sources, trust metadata, line numbers, scores, and hashes only; raw issue bodies, comment bodies, assistant replies, prompts, and raw search queries are not included.\n\n")

	b.WriteString("### Results\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- message=`%02d` role=`%s` source=`%s` actor=`%s` association=`%s` trusted=`%t` edited=`%t` line=`%d` score=`%d` matched_terms=`%d` message_bytes=`%d` message_lines=`%d` message_sha256_12=`%s` line_sha256_12=`%s`\n",
				result.MessageIndex,
				result.Role,
				result.Source,
				inlineCode(result.Actor),
				inlineCode(result.AuthorAssociation),
				result.Trusted,
				result.Edited,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.MessageBytes,
				result.MessageLines,
				result.MessageSHA,
				result.LineSHA,
			)
		}
	}
	return strings.TrimSpace(b.String())
}

func requestedSessionSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/session" || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanMemorySearchQuery(strings.Join(fields[2:], " "))
}

func requestedSessionRisk(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/session" && (strings.EqualFold(fields[1], "risk") || strings.EqualFold(fields[1], "risk-audit"))
}

func requestedSessionCoverage(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/session" && (strings.EqualFold(fields[1], "coverage") || strings.EqualFold(fields[1], "covered"))
}

func requestedSessionStats(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/session" && (strings.EqualFold(fields[1], "stats") || strings.EqualFold(fields[1], "summary"))
}

func BuildSessionSearchReport(transcript []TranscriptMessage, query string, maxResults int) SessionSearchReport {
	query = cleanMemorySearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultSessionSearchMaxResults
	}
	report := SessionSearchReport{
		QueryHash:          shortDocumentHash(query),
		QueryTerms:         len(memorySearchTerms(query)),
		SearchStatus:       "ok",
		MaxResults:         maxResults,
		TranscriptMessages: len(transcript),
		RawBodiesIncluded:  false,
	}
	if query == "" {
		report.SearchStatus = "no_query"
		return report
	}
	terms := memorySearchTerms(query)
	if len(terms) == 0 {
		report.SearchStatus = "no_query"
		return report
	}
	matchedMessages := map[int]bool{}
	var results []SessionSearchResult
	for index, msg := range transcript {
		lines := strings.Split(msg.Body, "\n")
		for lineIndex, line := range lines {
			score, matchedTerms := memoryLineSearchScore(sessionMessageSource(msg), line, query, terms)
			if score == 0 {
				continue
			}
			messageIndex := index + 1
			matchedMessages[messageIndex] = true
			results = append(results, SessionSearchResult{
				MessageIndex:      messageIndex,
				Role:              msg.Role,
				Source:            sessionMessageSource(msg),
				Actor:             msg.Actor,
				AuthorAssociation: msg.AuthorAssociation,
				Trusted:           msg.Trusted,
				Edited:            msg.Edited,
				Line:              lineIndex + 1,
				Score:             score,
				MatchedTerms:      matchedTerms,
				MessageBytes:      len(msg.Body),
				MessageLines:      lineCount(msg.Body),
				MessageSHA:        shortDocumentHash(msg.Body),
				LineSHA:           shortDocumentHash(line),
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].MessageIndex != results[j].MessageIndex {
			return results[i].MessageIndex < results[j].MessageIndex
		}
		return results[i].Line < results[j].Line
	})
	report.MatchedMessages = len(matchedMessages)
	report.MatchedLines = len(results)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	report.Results = results
	report.ResultsReturned = len(results)
	if report.MatchedLines == 0 {
		report.SearchStatus = "no_matches"
	}
	return report
}

func sessionMessageSource(msg TranscriptMessage) string {
	if msg.CommentID != 0 {
		return fmt.Sprintf("comment:%d", msg.CommentID)
	}
	return "issue"
}

func countSessionMarkers(comments []Comment) sessionMarkerCounts {
	var counts sessionMarkerCounts
	for _, comment := range comments {
		if HasGitClawMarker(comment.Body) {
			counts.AssistantTurns++
		}
		if HasHeartbeatMarker(comment.Body) {
			counts.Heartbeats++
		}
		if HasGitClawErrorMarker(comment.Body) {
			counts.Errors++
		}
		if HasChannelMessageMarker(comment.Body) {
			counts.ChannelMessages++
		}
	}
	return counts
}

func writeTranscriptMessageList(b *strings.Builder, transcript []TranscriptMessage) {
	if len(transcript) == 0 {
		b.WriteString("- none\n")
		return
	}
	for i, msg := range transcript {
		fmt.Fprintf(
			b,
			"- `%02d` role=`%s` source=`%s` actor=`%s` association=`%s` trusted=`%t` edited=`%t` bytes=`%d` lines=`%d` sha256_12=`%s`\n",
			i+1,
			msg.Role,
			sessionMessageSource(msg),
			inlineCode(msg.Actor),
			inlineCode(msg.AuthorAssociation),
			msg.Trusted,
			msg.Edited,
			len(msg.Body),
			lineCount(msg.Body),
			shortDocumentHash(msg.Body),
		)
	}
}

func writeSessionPromptProvenanceList(b *strings.Builder, turns []sessionPromptProvenanceTurn) {
	if len(turns) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, turn := range turns {
		promptHash := turn.PromptContextSHA
		if promptHash == "" {
			promptHash = "none"
		}
		fmt.Fprintf(
			b,
			"- source=`%s` model=`%s` prompt_context_sha256_12=`%s` context_documents=`%d` selected_skills=`%d` tool_outputs=`%d` skills=`%s` tools=`%s`\n",
			turn.Source,
			inlineCode(turn.Model),
			promptHash,
			turn.ContextDocuments,
			turn.SelectedSkills,
			turn.ToolOutputs,
			inlineListOrNone(turn.Skills),
			inlineListOrNone(turn.Tools),
		)
	}
}

func countTranscriptRole(transcript []TranscriptMessage, role string) int {
	count := 0
	for _, msg := range transcript {
		if msg.Role == role {
			count++
		}
	}
	return count
}

func countTrustedTranscriptMessages(transcript []TranscriptMessage, trusted bool) int {
	count := 0
	for _, msg := range transcript {
		if msg.Trusted == trusted {
			count++
		}
	}
	return count
}
