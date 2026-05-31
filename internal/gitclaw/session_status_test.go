package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSessionStatusReportSummarizesLatestMessagesWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 150,
			"title": "@gitclaw session status test",
			"body": "SESSION_STATUS_ISSUE_BODY_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:done"}]
		},
		"comment": {
			"id": 52,
			"body": "@gitclaw /session status\nSESSION_STATUS_USER_COMMENT_SECRET",
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
	comments := []Comment{
		{
			ID: 51,
			Body: RenderAssistantComment(Marker{
				RunID:               "status-model-run",
				EventID:             "issue-149",
				Model:               "openai/gpt-5-nano",
				IdempotencyKey:      "SESSION_STATUS_IDEMPOTENCY_SECRET",
				RunURL:              "https://github.com/owner/repo/actions/runs/SESSION_STATUS_URL_SECRET",
				PromptContextSHA:    "statusabc1234",
				ContextDocuments:    4,
				SelectedSkills:      1,
				ToolOutputs:         2,
				PromptVisibleSkills: []string{"repo-reader"},
				PromptVisibleTools:  []string{"gitclaw.search_files", "gitclaw.list_files"},
			}, "SESSION_STATUS_ASSISTANT_BODY_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                52,
			Body:              "@gitclaw /session status\nSESSION_STATUS_USER_COMMENT_SECRET",
			User:              User{Login: "alice", Type: "User"},
			AuthorAssociation: "MEMBER",
			CreatedAt:         "2026-05-31T12:00:00Z",
			UpdatedAt:         "2026-05-31T12:00:00Z",
		},
	}
	transcript := BuildTranscript(ev, comments)

	body := RenderSessionStatusReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Session Status Report",
		"Generated without a model call",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#150`",
		"event_kind: `issue_comment`",
		"active_command: `/session status`",
		"session_status: `ok`",
		"label_names: `gitclaw, gitclaw:done`",
		"raw_comments: `2`",
		"transcript_messages: `3`",
		"user_messages: `2`",
		"assistant_messages: `1`",
		"assistant_turn_comments: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"latest_user_message_present: `true`",
		"latest_assistant_message_present: `true`",
		"latest_assistant_model: `openai/gpt-5-nano`",
		"latest_assistant_prompt_context_sha256_12: `statusabc1234`",
		"raw_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_session_status_change: `true`",
		"### Latest Message Hashes",
		"kind=`user` present=`true` source=`comment:52` actor=`alice` trusted=`true` edited=`false`",
		"kind=`assistant` present=`true` source=`comment:51` actor=`github-actions[bot]` trusted=`true` edited=`false`",
		"sha256_12=`",
		"### Latest Assistant Marker",
		"model=`openai/gpt-5-nano` prompt_context_sha256_12=`statusabc1234` context_documents=`4` selected_skills=`1` tool_outputs=`2` skills=`repo-reader` tools=`gitclaw.list_files, gitclaw.search_files`",
		"kind=`skill` name=`repo-reader` turns=`1`",
		"kind=`tool` name=`gitclaw.list_files` turns=`1`",
		"kind=`tool` name=`gitclaw.search_files` turns=`1`",
		"backup JSON can replay the same body-free status locally",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session status report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_STATUS_ISSUE_BODY_SECRET", "SESSION_STATUS_USER_COMMENT_SECRET", "SESSION_STATUS_ASSISTANT_BODY_SECRET", "SESSION_STATUS_IDEMPOTENCY_SECRET", "SESSION_STATUS_URL_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session status report leaked body token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesStatusCommand(t *testing.T) {
	ev := Event{
		Kind: "issue_comment",
		Repo: "owner/repo",
		Issue: Issue{
			Number: 10,
			Title:  "@gitclaw session status route",
			Labels: []string{"gitclaw"},
		},
		Comment: &Comment{ID: 2, Body: "@gitclaw /session status\nSESSION_STATUS_ROUTE_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"},
	}
	body := RenderSessionReport(ev, DefaultConfig(), []Comment{{ID: 2, Body: "@gitclaw /session status\nSESSION_STATUS_ROUTE_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"}}, BuildTranscript(ev, []Comment{{ID: 2, Body: "@gitclaw /session status\nSESSION_STATUS_ROUTE_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"}}))
	for _, want := range []string{
		"GitClaw Session Status Report",
		"active_command: `/session status`",
		"session_status: `ok`",
		"latest_user_message_present: `true`",
		"latest_assistant_message_present: `false`",
		"kind=`assistant` present=`false`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session status route missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_STATUS_ROUTE_SECRET") {
		t.Fatalf("session status route leaked body:\n%s", body)
	}
}

func TestSessionStatusCommandReportsBackupWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	backup := IssueBackup{
		Version:   1,
		Repo:      "owner/repo",
		EventName: "issues",
		Issue: IssueBackupIssue{
			Number:            151,
			Title:             "@gitclaw session status",
			Body:              "SESSION_STATUS_CLI_ISSUE_BODY_SECRET",
			Author:            "alice",
			AuthorAssociation: "MEMBER",
			Labels:            []string{"gitclaw", "gitclaw:done"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_STATUS_CLI_USER_TRANSCRIPT_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
			{Role: "assistant", Body: "SESSION_STATUS_CLI_ASSISTANT_TRANSCRIPT_SECRET", Actor: "github-actions[bot]", AuthorAssociation: "MEMBER", CommentID: 61, Trusted: true},
			{Role: "user", Body: "@gitclaw /session status\nSESSION_STATUS_CLI_STATUS_REQUEST_SECRET", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 62, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{
				ID: 61,
				Body: RenderAssistantComment(Marker{
					RunID:               "backup-status-run",
					EventID:             "issue-151",
					Model:               "openai/gpt-5-nano",
					IdempotencyKey:      "SESSION_STATUS_CLI_IDEMPOTENCY_SECRET",
					PromptContextSHA:    "backupstatus1",
					ContextDocuments:    5,
					SelectedSkills:      1,
					ToolOutputs:         2,
					PromptVisibleSkills: []string{"repo-reader"},
					PromptVisibleTools:  []string{"gitclaw.search_files"},
				}, "SESSION_STATUS_CLI_COMMENT_BODY_SECRET"),
				Author:            "github-actions[bot]",
				AuthorAssociation: "MEMBER",
			},
			{ID: 62, Body: "@gitclaw /session status\nSESSION_STATUS_CLI_STATUS_REQUEST_SECRET", Author: "alice", AuthorAssociation: "MEMBER"},
		},
	}
	writeBackupFixture(t, dir, backup)
	backupPath := issueBackupPath(dir, backup.Repo, backup.Issue.Number)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"session", "status", "--backup", backupPath}); err != nil {
			t.Fatalf("session status returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Session Status Report",
		"scope: `local-backup`",
		"backup_file: `",
		"repository: `owner/repo`",
		"issue: `#151`",
		"session_status: `ok`",
		"label_names: `gitclaw, gitclaw:done`",
		"raw_comments: `2`",
		"transcript_messages: `3`",
		"latest_user_message_present: `true`",
		"latest_assistant_message_present: `true`",
		"latest_assistant_model: `openai/gpt-5-nano`",
		"latest_assistant_prompt_context_sha256_12: `backupstatus1`",
		"kind=`user` present=`true` source=`comment:62`",
		"kind=`assistant` present=`true` source=`comment:61`",
		"raw_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_session_status_change: `true`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("session status output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"SESSION_STATUS_CLI_ISSUE_BODY_SECRET", "SESSION_STATUS_CLI_USER_TRANSCRIPT_SECRET", "SESSION_STATUS_CLI_ASSISTANT_TRANSCRIPT_SECRET", "SESSION_STATUS_CLI_STATUS_REQUEST_SECRET", "SESSION_STATUS_CLI_IDEMPOTENCY_SECRET", "SESSION_STATUS_CLI_COMMENT_BODY_SECRET"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("session status output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestHandleSessionStatusCommandPostsReportWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 152,
			"title": "GitClaw session status handler test",
			"body": "SESSION_STATUS_HANDLER_ISSUE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 72,
			"body": "@gitclaw /session status\nHidden comment token: SESSION_STATUS_HANDLER_COMMENT_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{152: {
		{
			ID: 71,
			Body: RenderAssistantComment(Marker{
				RunID:               "handler-status-run",
				EventID:             "issue-151",
				Model:               "openai/gpt-4.1-nano",
				IdempotencyKey:      "SESSION_STATUS_HANDLER_IDEMPOTENCY_SECRET",
				PromptContextSHA:    "handlerstat1",
				ContextDocuments:    5,
				SelectedSkills:      1,
				ToolOutputs:         2,
				PromptVisibleSkills: []string{"repo-reader"},
				PromptVisibleTools:  []string{"gitclaw.search_files"},
			}, "SESSION_STATUS_HANDLER_ASSISTANT_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                72,
			Body:              "@gitclaw /session status\nHidden comment token: SESSION_STATUS_HANDLER_COMMENT_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic session status command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Session Status Report",
		"Generated without a model call",
		"model=\"gitclaw/session\"",
		"repository: `owner/repo`",
		"issue: `#152`",
		"active_command: `/session status`",
		"session_status: `ok`",
		"raw_comments: `2`",
		"transcript_messages: `3`",
		"latest_user_message_present: `true`",
		"latest_assistant_message_present: `true`",
		"latest_assistant_model: `openai/gpt-4.1-nano`",
		"latest_assistant_prompt_context_sha256_12: `handlerstat1`",
		"kind=`tool` name=`gitclaw.search_files` turns=`1`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session status report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_STATUS_HANDLER_ISSUE_SECRET", "SESSION_STATUS_HANDLER_COMMENT_SECRET", "SESSION_STATUS_HANDLER_ASSISTANT_SECRET", "SESSION_STATUS_HANDLER_IDEMPOTENCY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session status report leaked body token %q:\n%s", leaked, body)
		}
	}
}
