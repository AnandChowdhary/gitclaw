package gitclaw

import (
	"strings"
	"testing"
)

func TestBuildTranscriptOrdersUserAndAssistantMessages(t *testing.T) {
	ev := Event{
		Issue: Issue{
			Number: 1,
			Title:  "@gitclaw explain",
			Body:   "Initial question",
			User:   User{Login: "alice", Type: "User"},
		},
	}
	comments := []Comment{
		{
			ID:                10,
			Body:              "<!-- gitclaw:assistant-turn idempotency_key=old -->\nFirst answer",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                11,
			Body:              "Follow up",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			UpdatedAt:         "2026-05-29T12:00:00Z",
		},
		{
			ID:   12,
			Body: "unrelated bot noise",
			User: User{Login: "dependabot[bot]", Type: "Bot"},
		},
		{
			ID:   13,
			Body: RenderErrorComment(ErrorMarker{RunID: "run-1", EventID: "issue-1", Phase: "model"}, "model provider request failed"),
			User: User{Login: "github-actions[bot]", Type: "Bot"},
		},
	}
	transcript := BuildTranscript(ev, comments)
	if len(transcript) != 3 {
		t.Fatalf("len(transcript) = %d, want 3: %#v", len(transcript), transcript)
	}
	if transcript[0].Role != "user" || transcript[1].Role != "assistant" || transcript[2].Role != "user" {
		t.Fatalf("roles = %#v", transcript)
	}
	if !transcript[2].Edited {
		t.Fatalf("updated user comment should be marked edited")
	}
}

func TestBuildTranscriptMarksOriginalIssueTrustFromAuthorAssociation(t *testing.T) {
	ev := Event{
		Issue: Issue{
			Number:            1,
			Title:             "@gitclaw explain",
			Body:              "Initial question",
			User:              User{Login: "mallory", Type: "User"},
			AuthorAssociation: "CONTRIBUTOR",
		},
	}

	transcript := BuildTranscript(ev, nil)
	if len(transcript) != 1 {
		t.Fatalf("len(transcript) = %d, want 1", len(transcript))
	}
	if transcript[0].Trusted {
		t.Fatalf("original issue from CONTRIBUTOR should be marked untrusted")
	}
}

func TestBuildTranscriptIncludesChannelMessageMarkerFromBot(t *testing.T) {
	ev := Event{
		Issue: Issue{
			Number:            1,
			Title:             "Mirrored Telegram session",
			Body:              "Channel bridge root issue.",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	comments := []Comment{
		{
			ID:                21,
			Body:              "<!-- gitclaw:channel-message channel=\"telegram\" message_id=\"123\" -->\nPlease answer with CHANNEL_MARKER_TOKEN.",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := BuildTranscript(ev, comments)
	if len(transcript) != 2 {
		t.Fatalf("len(transcript) = %d, want 2: %#v", len(transcript), transcript)
	}
	msg := transcript[1]
	if msg.Role != "user" || !strings.Contains(msg.Body, "CHANNEL_MARKER_TOKEN") {
		t.Fatalf("channel message was not reconstructed as user input: %#v", msg)
	}
	if strings.Contains(msg.Body, "gitclaw:channel-message") {
		t.Fatalf("channel marker should be stripped from prompt body: %q", msg.Body)
	}
	if msg.Trusted {
		t.Fatalf("channel message content should remain untrusted input")
	}
}
