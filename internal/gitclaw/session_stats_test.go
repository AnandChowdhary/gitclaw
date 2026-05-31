package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSessionStatsReportSummarizesWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                81,
		Body:              "<!-- gitclaw:assistant-turn model=\"openai/gpt-5-nano\" prompt_context_sha256_12=\"feedface1234\" context_documents=\"5\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files,gitclaw.read_file\" -->\nSESSION_STATS_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "SESSION_STATS_USER_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
		{Role: "assistant", Body: "SESSION_STATS_ASSISTANT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 81, Trusted: true},
	}
	report := BuildSessionStatsReport("issue-thread", "", Event{
		Kind:  "issue_comment",
		Repo:  "owner/repo",
		Issue: Issue{Number: 9, Body: "SESSION_STATS_ISSUE_BODY_SECRET"},
	}, comments, transcript)
	body := RenderSessionStatsReport(report)
	for _, want := range []string{
		"GitClaw Session Stats Report",
		"Generated without a model call",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#9`",
		"event_kind: `issue_comment`",
		"session_stats_status: `ok`",
		"raw_comments: `1`",
		"transcript_messages: `2`",
		"user_messages: `1`",
		"assistant_messages: `1`",
		"trusted_messages: `2`",
		"untrusted_messages: `0`",
		"edited_messages: `0`",
		"transcript_body_bytes:",
		"transcript_body_lines:",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_count: `1`",
		"prompt_visible_tool_count: `2`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files, gitclaw.read_file`",
		"raw_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"### Stats Cards",
		"kind=`transcript-shape`",
		"kind=`assistant-provenance`",
		"kind=`prompt-surface`",
		"kind=`session-markers`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session stats report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_STATS_USER_SECRET", "SESSION_STATS_ASSISTANT_SECRET", "SESSION_STATS_ISSUE_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session stats report leaked body token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesStatsCommand(t *testing.T) {
	body := RenderSessionReport(Event{
		Kind:  "issue_comment",
		Repo:  "owner/repo",
		Issue: Issue{Number: 10, Title: "@gitclaw /session stats"},
	}, DefaultConfig(), []Comment{{
		ID:                82,
		Body:              "<!-- gitclaw:assistant-turn model=\"openai/gpt-5-nano\" prompt_context_sha256_12=\"abcabcabcabc\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" -->\nstats route secret",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}, []TranscriptMessage{{
		Role:      "assistant",
		Body:      "stats route secret",
		Actor:     "github-actions[bot]",
		CommentID: 82,
		Trusted:   true,
	}})
	for _, want := range []string{"GitClaw Session Stats Report", "session_stats_status: `ok`", "model_backed_assistant_turns: `1`", "model_names: `openai/gpt-5-nano`", "prompt_visible_skill_names: `repo-reader`", "prompt_visible_tool_names: `gitclaw.search_files`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session stats route missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "stats route secret") {
		t.Fatalf("session stats route leaked body:\n%s", body)
	}
}

func TestSessionStatsCommandReportsBackupTranscriptWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	backup := IssueBackup{
		Version:   1,
		Repo:      "owner/repo",
		EventName: "issue_comment",
		Issue: IssueBackupIssue{
			Number: 11,
			Title:  "@gitclaw session stats",
			Body:   "SESSION_STATS_CLI_ISSUE_BODY_TOKEN",
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_STATS_CLI_ISSUE_TRANSCRIPT_TOKEN", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
			{Role: "assistant", Body: "SESSION_STATS_CLI_ASSISTANT_TRANSCRIPT_TOKEN", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 91, Trusted: true},
			{Role: "user", Body: "SESSION_STATS_CLI_COMMENT_TRANSCRIPT_TOKEN", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 92, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 91, Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-5-nano\" prompt_context_sha256_12=\"dadada123456\" context_documents=\"3\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" -->\nSESSION_STATS_CLI_ASSISTANT_COMMENT_TOKEN", Author: "github-actions[bot]", AuthorAssociation: "NONE"},
			{ID: 92, Body: "@gitclaw /session stats\nSESSION_STATS_CLI_USER_COMMENT_TOKEN", Author: "alice", AuthorAssociation: "MEMBER"},
		},
	}
	writeBackupFixture(t, dir, backup)
	backupPath := issueBackupPath(dir, "owner/repo", 11)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"session", "stats", "--backup", backupPath}); err != nil {
			t.Fatalf("session stats returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Session Stats Report",
		"scope: `local-backup`",
		"backup_file:",
		"repository: `owner/repo`",
		"issue: `#11`",
		"event_kind: `issue_comment`",
		"session_stats_status: `ok`",
		"raw_comments: `2`",
		"transcript_messages: `3`",
		"user_messages: `2`",
		"assistant_messages: `1`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"model_backed_assistant_turns: `1`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("session stats output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"@gitclaw session stats", "SESSION_STATS_CLI_ISSUE_BODY_TOKEN", "SESSION_STATS_CLI_ISSUE_TRANSCRIPT_TOKEN", "SESSION_STATS_CLI_ASSISTANT_TRANSCRIPT_TOKEN", "SESSION_STATS_CLI_COMMENT_TRANSCRIPT_TOKEN", "SESSION_STATS_CLI_ASSISTANT_COMMENT_TOKEN", "SESSION_STATS_CLI_USER_COMMENT_TOKEN"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("session stats output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestHandleSessionStatsCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 119,
			"title": "GitClaw session stats handler test",
			"body": "Initial stats body token: SESSION_STATS_HANDLER_ISSUE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 94,
			"body": "@gitclaw /session stats\nHidden comment token: SESSION_STATS_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2026-05-31T12:00:00Z",
			"updated_at": "2026-05-31T12:00:00Z"
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{119: {
		{
			ID:                93,
			Body:              "<!-- gitclaw:assistant-turn idempotency_key=\"old\" model=\"openai/gpt-5-nano\" prompt_context_sha256_12=\"abcdef123456\" context_documents=\"7\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" -->\nAssistant body token: SESSION_STATS_HANDLER_ASSISTANT_SECRET.",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                94,
			Body:              "@gitclaw /session stats\nHidden comment token: SESSION_STATS_HANDLER_BODY_SECRET.",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			CreatedAt:         "2026-05-31T12:00:00Z",
			UpdatedAt:         "2026-05-31T12:00:00Z",
		},
	}}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic session stats command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Session Stats Report", "Generated without a model call", "model=\"gitclaw/session\"", "session_stats_status: `ok`", "raw_comments: `2`", "transcript_messages: `3`", "user_messages: `2`", "assistant_messages: `1`", "assistant_turn_comments: `1`", "assistant_turns_with_prompt_provenance: `1`", "assistant_turns_missing_prompt_provenance: `0`", "unique_prompt_context_hashes: `1`", "model_backed_assistant_turns: `1`", "model_names: `openai/gpt-5-nano`", "prompt_visible_skill_names: `repo-reader`", "prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`", "raw_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session stats report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_STATS_HANDLER_ISSUE_SECRET", "SESSION_STATS_HANDLER_BODY_SECRET", "SESSION_STATS_HANDLER_ASSISTANT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session stats report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[119], "gitclaw:done") || hasLabel(github.IssueLabels[119], "gitclaw:running") || hasLabel(github.IssueLabels[119], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[119])
	}
}
