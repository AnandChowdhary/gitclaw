package gitclaw

import "testing"

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
