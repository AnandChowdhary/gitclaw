package gitclaw

import (
	"fmt"
	"strings"
)

func BuildTranscript(ev Event, comments []Comment) []TranscriptMessage {
	cfg := DefaultConfig()
	transcript := []TranscriptMessage{
		{
			Role:              "user",
			Body:              fmt.Sprintf("%s\n\n%s", ev.Issue.Title, ev.Issue.Body),
			Actor:             ev.Issue.User.Login,
			AuthorAssociation: ev.Issue.AuthorAssociation,
			Trusted:           trustedAssociation(ev.Issue.AuthorAssociation, cfg),
		},
	}
	for _, comment := range comments {
		if HasGitClawMarker(comment.Body) {
			transcript = append(transcript, TranscriptMessage{
				Role:              "assistant",
				Body:              StripMarker(comment.Body),
				Actor:             comment.User.Login,
				AuthorAssociation: comment.AuthorAssociation,
				CommentID:         comment.ID,
				Trusted:           true,
			})
			continue
		}
		if HasHeartbeatMarker(comment.Body) {
			transcript = append(transcript, TranscriptMessage{
				Role:              "assistant",
				Body:              strings.TrimSpace(heartbeatMarkerPattern.ReplaceAllString(comment.Body, "")),
				Actor:             comment.User.Login,
				AuthorAssociation: comment.AuthorAssociation,
				CommentID:         comment.ID,
				Trusted:           true,
			})
			continue
		}
		if HasGitClawErrorMarker(comment.Body) {
			continue
		}
		if comment.User.IsBot() {
			continue
		}
		transcript = append(transcript, TranscriptMessage{
			Role:              "user",
			Body:              comment.Body,
			Actor:             comment.User.Login,
			AuthorAssociation: comment.AuthorAssociation,
			CommentID:         comment.ID,
			Edited:            comment.UpdatedAt != "" && comment.UpdatedAt != comment.CreatedAt,
			Trusted:           trustedAssociation(comment.AuthorAssociation, cfg),
		})
	}
	return transcript
}
