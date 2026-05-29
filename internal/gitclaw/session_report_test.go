package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderSessionSearchReportFindsTranscriptWithoutBodies(t *testing.T) {
	transcript := []TranscriptMessage{
		{
			Role:              "user",
			Body:              "Please remember deployment window SESSION_SEARCH_ISSUE_SECRET.",
			Actor:             "alice",
			AuthorAssociation: "MEMBER",
			Trusted:           true,
		},
		{
			Role:              "assistant",
			Body:              "Deployment window noted with SESSION_SEARCH_ASSISTANT_SECRET.",
			Actor:             "github-actions[bot]",
			AuthorAssociation: "NONE",
			CommentID:         42,
			Trusted:           true,
		},
	}
	body := RenderSessionSearchReport(Event{}, transcript, "deployment SESSION_SEARCH_QUERY_SECRET", 1)
	for _, want := range []string{
		"GitClaw Session Search Report",
		"scope: `local-cli`",
		"session_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `1`",
		"transcript_messages: `2`",
		"matched_messages: `2`",
		"matched_lines: `2`",
		"results_returned: `1`",
		"raw_bodies_included: `false`",
		"message=`01`",
		"role=`user`",
		"source=`issue`",
		"trusted=`true`",
		"message_sha256_12=",
		"line_sha256_12=",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_SEARCH_ISSUE_SECRET", "SESSION_SEARCH_ASSISTANT_SECRET", "SESSION_SEARCH_QUERY_SECRET", "deployment SESSION_SEARCH_QUERY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session search report leaked body/query token %q:\n%s", leaked, body)
		}
	}
}
