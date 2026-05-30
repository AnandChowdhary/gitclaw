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

func TestRenderSessionReportShowsAssistantTurnProvenanceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                51,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files,gitclaw.read_file\" -->\nSESSION_PROVENANCE_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_PROVENANCE_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 51,
		Trusted:   true,
	}}
	body := renderSessionReport(Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, comments, transcript, true, "")
	for _, want := range []string{
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files, gitclaw.read_file`",
		"### Assistant Turn Provenance",
		"source=`comment:51`",
		"model=`openai/gpt-4.1-nano`",
		"prompt_context_sha256_12=`abc123abc123`",
		"context_documents=`2`",
		"selected_skills=`1`",
		"tool_outputs=`2`",
		"skills=`repo-reader`",
		"tools=`gitclaw.search_files, gitclaw.read_file`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_PROVENANCE_ASSISTANT_SECRET") {
		t.Fatalf("session report leaked assistant body:\n%s", body)
	}
}

func TestBuildSessionPromptProvenanceReportCountsMissingMarkers(t *testing.T) {
	report := buildSessionPromptProvenanceReport([]Comment{{
		ID:   52,
		Body: "<!-- gitclaw:assistant-turn model=\"gitclaw/context\" -->\nold deterministic report",
	}})
	if report.TurnsWithProvenance != 0 || report.PromptContextHashMissing != 1 || len(report.Turns) != 1 {
		t.Fatalf("unexpected provenance report: %#v", report)
	}
}
