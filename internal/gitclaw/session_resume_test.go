package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSessionResumeReportShowsContinuationReadinessWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 170,
			"title": "@gitclaw session resume test",
			"body": "SESSION_RESUME_ISSUE_BODY_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:done"}]
		},
		"comment": {
			"id": 82,
			"body": "@gitclaw /session resume\nSESSION_RESUME_USER_COMMENT_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2026-06-01T12:00:00Z",
			"updated_at": "2026-06-01T12:00:00Z"
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	comments := []Comment{
		{
			ID: 81,
			Body: RenderAssistantComment(Marker{
				RunID:               "resume-model-run",
				EventID:             "issue-170",
				Model:               "openai/gpt-5-nano",
				IdempotencyKey:      "SESSION_RESUME_IDEMPOTENCY_SECRET",
				RunURL:              "https://github.com/owner/repo/actions/runs/SESSION_RESUME_URL_SECRET",
				PromptContextSHA:    "resumeabc123",
				ContextDocuments:    4,
				SelectedSkills:      1,
				ToolOutputs:         2,
				PromptVisibleSkills: []string{"repo-reader"},
				PromptVisibleTools:  []string{"gitclaw.search_files", "gitclaw.list_files"},
				Usage:               LLMUsage{Present: true, PromptTokens: 100, CompletionTokens: 9, TotalTokens: 109, CacheReadTokens: 7, CacheWriteTokens: 2},
			}, "SESSION_RESUME_ASSISTANT_BODY_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                82,
			Body:              "@gitclaw /session resume\nSESSION_RESUME_USER_COMMENT_SECRET",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			CreatedAt:         "2026-06-01T12:00:00Z",
			UpdatedAt:         "2026-06-01T12:00:00Z",
		},
	}
	transcript := BuildTranscript(ev, comments)

	body := RenderSessionResumeReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Session Resume Report",
		"Generated without a model call",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#170`",
		"event_kind: `issue_comment`",
		"active_command: `/session resume`",
		"session_resume_status: `ok`",
		"resume_strategy: `github-issue-comment-continuation`",
		"resume_key_sha256_12: `",
		"session_store: `github-issue-thread`",
		"raw_comments: `2`",
		"transcript_messages: `3`",
		"user_messages: `2`",
		"assistant_messages: `1`",
		"label_names: `gitclaw, gitclaw:done`",
		"trigger_label_present: `true`",
		"done_label_present: `true`",
		"error_label_present: `false`",
		"disabled_label_present: `false`",
		"latest_user_message_present: `true`",
		"latest_assistant_message_present: `true`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"latest_assistant_model: `openai/gpt-5-nano`",
		"latest_assistant_prompt_context_sha256_12: `resumeabc123`",
		"usage_bearing_assistant_turns: `1`",
		"usage_prompt_tokens: `100`",
		"usage_completion_tokens: `9`",
		"usage_total_tokens: `109`",
		"usage_cache_read_tokens: `7`",
		"usage_cache_write_tokens: `2`",
		"next_issue_comment_resumes_session: `true`",
		"github_actions_reentry_supported: `true`",
		"workflow_event: `issue_comment`",
		"workflow_dispatch_required: `false`",
		"server_required: `false`",
		"socket_required: `false`",
		"external_session_db_required: `false`",
		"backup_branch_replay_preferred: `true`",
		"issue_thread_canonical_storage: `true`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_provider_responses_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"resume_mutation_allowed: `false`",
		"llm_e2e_required_after_session_resume_change: `true`",
		"### Resume Anchors",
		"kind=`session-key` source=`github-issue`",
		"kind=`latest-user` source=`comment:82`",
		"kind=`latest-assistant` source=`comment:81`",
		"kind=`latest-model-turn` source=`comment:81` model=`openai/gpt-5-nano`",
		"prompt_context_sha256_12=`resumeabc123`",
		"skills=`repo-reader` tools=`gitclaw.list_files, gitclaw.search_files` usage_present=`true` usage_total_tokens=`109`",
		"body_included=`false` identifier_included=`false`",
		"### Latest Assistant Marker",
		"model=`openai/gpt-5-nano` prompt_context_sha256_12=`resumeabc123` context_documents=`4` selected_skills=`1` tool_outputs=`2` skills=`repo-reader` tools=`gitclaw.list_files, gitclaw.search_files`",
		"### Resume Gates",
		"continuation_gate=`pass`",
		"issue_label_gate=`pass`",
		"latest_user_gate=`pass`",
		"latest_assistant_gate=`pass`",
		"model_backed_gate=`pass`",
		"prompt_provenance_gate=`pass`",
		"reentry_gate=`issue-comment-action`",
		"external_session_db_gate=`disabled`",
		"raw_body_gate=`hashes-counts-and-marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session resume report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_RESUME_ISSUE_BODY_SECRET", "SESSION_RESUME_USER_COMMENT_SECRET", "SESSION_RESUME_ASSISTANT_BODY_SECRET", "SESSION_RESUME_IDEMPOTENCY_SECRET", "SESSION_RESUME_URL_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session resume report leaked body token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesResumeCommand(t *testing.T) {
	ev := Event{
		Kind: "issue_comment",
		Repo: "owner/repo",
		Issue: Issue{
			Number: 171,
			Title:  "@gitclaw session resume route",
			Labels: []string{"gitclaw"},
		},
		Comment: &Comment{ID: 2, Body: "@gitclaw /session resume\nSESSION_RESUME_ROUTE_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"},
	}
	comments := []Comment{{ID: 2, Body: "@gitclaw /session resume\nSESSION_RESUME_ROUTE_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"}}
	body := RenderSessionReport(ev, DefaultConfig(), comments, BuildTranscript(ev, comments))
	for _, want := range []string{
		"GitClaw Session Resume Report",
		"active_command: `/session resume`",
		"session_resume_status: `warn`",
		"next_issue_comment_resumes_session: `true`",
		"latest_user_gate=`pass`",
		"latest_assistant_gate=`missing`",
		"model_backed_gate=`no-assistant-turns`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session resume route missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_RESUME_ROUTE_SECRET") {
		t.Fatalf("session resume route leaked body:\n%s", body)
	}
}

func TestSessionResumeCommandReportsBackupWithoutBodies(t *testing.T) {
	t.Setenv("GITCLAW_WORKDIR", t.TempDir())
	dir := t.TempDir()
	backup := IssueBackup{
		Version:   1,
		Repo:      "owner/repo",
		EventName: "issues",
		Issue: IssueBackupIssue{
			Number:            172,
			Title:             "@gitclaw session resume",
			Body:              "SESSION_RESUME_CLI_ISSUE_BODY_SECRET",
			Author:            "alice",
			AuthorAssociation: "MEMBER",
			Labels:            []string{"gitclaw", "gitclaw:done"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_RESUME_CLI_USER_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
			{Role: "assistant", Body: "SESSION_RESUME_CLI_ASSISTANT_TRANSCRIPT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "MEMBER", CommentID: 91, Trusted: true},
			{Role: "user", Body: "@gitclaw /session resume\nSESSION_RESUME_CLI_RESUME_REQUEST_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 92, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{
				ID: 91,
				Body: RenderAssistantComment(Marker{
					RunID:               "backup-resume-run",
					EventID:             "issue-172",
					Model:               "openai/gpt-5-nano",
					IdempotencyKey:      "SESSION_RESUME_CLI_IDEMPOTENCY_SECRET",
					PromptContextSHA:    "backupresume1",
					ContextDocuments:    5,
					SelectedSkills:      1,
					ToolOutputs:         2,
					PromptVisibleSkills: []string{"repo-reader"},
					PromptVisibleTools:  []string{"gitclaw.search_files"},
					Usage:               LLMUsage{Present: true, PromptTokens: 100, CompletionTokens: 9, TotalTokens: 109},
				}, "SESSION_RESUME_CLI_COMMENT_BODY_SECRET"),
				Author:            "github-actions[bot]",
				AuthorAssociation: "MEMBER",
			},
			{ID: 92, Body: "@gitclaw /session resume\nSESSION_RESUME_CLI_RESUME_REQUEST_SECRET", Author: "alice", AuthorAssociation: "MEMBER"},
		},
	}
	writeBackupFixture(t, dir, backup)
	backupPath := issueBackupPath(dir, backup.Repo, backup.Issue.Number)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"session", "resume", "--backup", backupPath}); err != nil {
			t.Fatalf("session resume returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Session Resume Report",
		"scope: `local-backup`",
		"backup_file: `",
		"repository: `owner/repo`",
		"issue: `#172`",
		"session_resume_status: `ok`",
		"resume_strategy: `github-issue-comment-continuation`",
		"local_backup_store: `gitclaw-backups issue JSON`",
		"label_names: `gitclaw, gitclaw:done`",
		"latest_user_message_present: `true`",
		"latest_assistant_message_present: `true`",
		"latest_assistant_model: `openai/gpt-5-nano`",
		"latest_assistant_prompt_context_sha256_12: `backupresume1`",
		"usage_total_tokens: `109`",
		"next_issue_comment_resumes_session: `true`",
		"server_required: `false`",
		"socket_required: `false`",
		"external_session_db_required: `false`",
		"raw_bodies_included: `false`",
		"resume_mutation_allowed: `false`",
		"llm_e2e_required_after_session_resume_change: `true`",
		"kind=`latest-user` source=`comment:92`",
		"kind=`latest-assistant` source=`comment:91`",
		"kind=`latest-model-turn` source=`comment:91` model=`openai/gpt-5-nano`",
		"continuation_gate=`pass`",
		"issue_label_gate=`pass`",
		"model_backed_gate=`pass`",
		"prompt_provenance_gate=`pass`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("session resume output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"SESSION_RESUME_CLI_ISSUE_BODY_SECRET", "SESSION_RESUME_CLI_USER_TRANSCRIPT_SECRET", "SESSION_RESUME_CLI_ASSISTANT_TRANSCRIPT_SECRET", "SESSION_RESUME_CLI_RESUME_REQUEST_SECRET", "SESSION_RESUME_CLI_IDEMPOTENCY_SECRET", "SESSION_RESUME_CLI_COMMENT_BODY_SECRET"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("session resume output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestHandleSessionResumeCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 173,
			"title": "GitClaw session resume handler test",
			"body": "SESSION_RESUME_HANDLER_ISSUE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:done"}]
		},
		"comment": {
			"id": 102,
			"body": "@gitclaw /session resume\nHidden comment token: SESSION_RESUME_HANDLER_COMMENT_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2026-06-01T12:00:00Z",
			"updated_at": "2026-06-01T12:00:00Z"
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{173: {
		{
			ID: 101,
			Body: RenderAssistantComment(Marker{
				RunID:               "resume-handler-run",
				EventID:             "issue-173",
				Model:               "openai/gpt-5-nano",
				IdempotencyKey:      "SESSION_RESUME_HANDLER_IDEMPOTENCY_SECRET",
				PromptContextSHA:    "handlerresume",
				ContextDocuments:    5,
				SelectedSkills:      1,
				ToolOutputs:         2,
				PromptVisibleSkills: []string{"repo-reader"},
				PromptVisibleTools:  []string{"gitclaw.search_files"},
				Usage:               LLMUsage{Present: true, PromptTokens: 100, CompletionTokens: 9, TotalTokens: 109},
			}, "SESSION_RESUME_HANDLER_ASSISTANT_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                102,
			Body:              "@gitclaw /session resume\nHidden comment token: SESSION_RESUME_HANDLER_COMMENT_SECRET.",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			CreatedAt:         "2026-06-01T12:00:00Z",
			UpdatedAt:         "2026-06-01T12:00:00Z",
		},
	}}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic session resume command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Session Resume Report",
		"Generated without a model call",
		"model=\"gitclaw/session\"",
		"session_resume_status: `ok`",
		"resume_strategy: `github-issue-comment-continuation`",
		"next_issue_comment_resumes_session: `true`",
		"github_actions_reentry_supported: `true`",
		"workflow_event: `issue_comment`",
		"workflow_dispatch_required: `false`",
		"server_required: `false`",
		"socket_required: `false`",
		"external_session_db_required: `false`",
		"model_backed_assistant_turns: `1`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files`",
		"usage_total_tokens: `109`",
		"raw_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"resume_mutation_allowed: `false`",
		"llm_e2e_required_after_session_resume_change: `true`",
		"continuation_gate=`pass`",
		"issue_label_gate=`pass`",
		"model_backed_gate=`pass`",
		"prompt_provenance_gate=`pass`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session resume report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_RESUME_HANDLER_ISSUE_SECRET", "SESSION_RESUME_HANDLER_ASSISTANT_SECRET", "SESSION_RESUME_HANDLER_COMMENT_SECRET", "SESSION_RESUME_HANDLER_IDEMPOTENCY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session resume report leaked body token %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[173], "gitclaw:done") || hasLabel(github.IssueLabels[173], "gitclaw:running") || hasLabel(github.IssueLabels[173], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[173])
	}
}
