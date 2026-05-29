package gitclaw

import (
	"fmt"
	"strings"
)

type sessionMarkerCounts struct {
	AssistantTurns  int
	Heartbeats      int
	Errors          int
	ChannelMessages int
}

func IsSessionReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/session"
}

func RenderSessionReport(ev Event, comments []Comment, transcript []TranscriptMessage) string {
	counts := countSessionMarkers(comments)
	var b strings.Builder
	b.WriteString("## GitClaw Session Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- event_kind: `%s`\n", ev.Kind)
	fmt.Fprintf(&b, "- raw_comments: `%d`\n", len(comments))
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- user_messages: `%d`\n", countTranscriptRole(transcript, "user"))
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", countTranscriptRole(transcript, "assistant"))
	fmt.Fprintf(&b, "- trusted_messages: `%d`\n", countTrustedTranscriptMessages(transcript, true))
	fmt.Fprintf(&b, "- untrusted_messages: `%d`\n", countTrustedTranscriptMessages(transcript, false))
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", counts.AssistantTurns)
	fmt.Fprintf(&b, "- heartbeat_comments: `%d`\n", counts.Heartbeats)
	fmt.Fprintf(&b, "- error_marker_comments: `%d`\n", counts.Errors)
	fmt.Fprintf(&b, "- channel_message_comments: `%d`\n", counts.ChannelMessages)
	fmt.Fprintf(&b, "- channel_thread_issue: `%t`\n", HasChannelThreadMarker(ev.Issue.Body))
	fmt.Fprintf(&b, "- proactive_run_issue: `%t`\n\n", HasProactiveRunMarker(ev.Issue.Body))
	b.WriteString("Message bodies are not included; hashes let maintainers verify exactly which issue-thread messages were loaded.\n\n")

	b.WriteString("### Transcript Messages\n")
	writeTranscriptMessageList(&b, transcript)

	return strings.TrimSpace(b.String())
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
		source := "issue"
		if msg.CommentID != 0 {
			source = fmt.Sprintf("comment:%d", msg.CommentID)
		}
		fmt.Fprintf(
			b,
			"- `%02d` role=`%s` source=`%s` actor=`%s` association=`%s` trusted=`%t` edited=`%t` bytes=`%d` lines=`%d` sha256_12=`%s`\n",
			i+1,
			msg.Role,
			source,
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
