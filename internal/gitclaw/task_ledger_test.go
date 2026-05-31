package gitclaw

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTaskLedgerReportAuditsIssueThreadWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 131,
			"title": "@gitclaw /tasks ledger",
			"body": "Hidden tasks ledger issue token: TASK_LEDGER_ISSUE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:running"}]
		},
		"comment": {
			"id": 77,
			"body": "@gitclaw /tasks ledger\nHidden active comment token: TASK_LEDGER_ACTIVE_COMMENT_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	assistant := RenderAssistantComment(Marker{
		RunID:               "run-task-ledger-secret",
		EventID:             "event-task-ledger-secret",
		Model:               "openai/gpt-5-nano",
		IdempotencyKey:      "idempotency-task-ledger-secret",
		RunURL:              "https://github.com/owner/repo/actions/runs/123",
		PromptContextSHA:    "abcdef123456",
		ContextDocuments:    2,
		SelectedSkills:      1,
		ToolOutputs:         1,
		PromptVisibleSkills: []string{"repo-reader"},
		PromptVisibleTools:  []string{"gitclaw.search_files"},
	}, "TASK_LEDGER_ASSISTANT_BODY_SECRET")
	comments := []Comment{
		{ID: 77, Body: "TASK_LEDGER_COMMENT_BODY_SECRET", AuthorAssociation: "MEMBER", User: User{Login: "alice"}},
		{ID: 78, Body: assistant, AuthorAssociation: "NONE", User: User{Login: "github-actions[bot]", Type: "Bot"}},
	}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "TASK_LEDGER_ISSUE_SECRET", Actor: "alice", Trusted: true},
		{Role: "user", Body: "TASK_LEDGER_COMMENT_BODY_SECRET", Actor: "alice", Trusted: true, CommentID: 77},
		{Role: "assistant", Body: "TASK_LEDGER_ASSISTANT_BODY_SECRET", Actor: "github-actions[bot]", CommentID: 78},
	}

	report := RenderTaskReport(ev, cfg, comments, transcript)
	for _, want := range []string{
		"GitClaw Task Ledger Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#131`",
		"task_ledger_status: `ok`",
		"ledger_source: `issue-thread`",
		"task_policy_present: `true`",
		"task_policy_loaded_for_model: `true`",
		"task_specs: `1`",
		"task_storage_backend: `github-issues`",
		"current_issue_task: `true`",
		"current_task_status: `running`",
		"current_task_labels: `2`",
		"comments_scanned: `2`",
		"transcript_messages: `3`",
		"user_comments: `1`",
		"assistant_comments: `1`",
		"assistant_turns: `1`",
		"model_backed_turns: `1`",
		"deterministic_turns: `0`",
		"turns_with_prompt_provenance: `1`",
		"error_markers: `0`",
		"status_history_available: `false`",
		"status_transition_source: `current-labels-and-markers`",
		"task_risk_status: `ok`",
		"raw_task_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_task_ledger_change: `true`",
		"### Ledger Entries",
		"kind=`current-task` source=`issue` status=`running`",
		"kind=`user-comment` source=`comment:77`",
		"kind=`assistant-turn` source=`comment:78`",
		"model=`openai/gpt-5-nano`",
		"prompt_context_sha256_12=`abcdef123456`",
		"skills=`repo-reader`",
		"tools=`gitclaw.search_files`",
		"body_sha256_12=",
		"run_id_sha256_12=",
		"### Ledger Gates",
		"raw_body_gate=`hash_only`",
		"status_history_gate=`current_labels_only`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("task ledger report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"TASK_LEDGER_ISSUE_SECRET",
		"TASK_LEDGER_ACTIVE_COMMENT_SECRET",
		"TASK_LEDGER_COMMENT_BODY_SECRET",
		"TASK_LEDGER_ASSISTANT_BODY_SECRET",
		"run-task-ledger-secret",
		"event-task-ledger-secret",
		"idempotency-task-ledger-secret",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("task ledger report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestTasksLedgerCommandReadsBackupWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	assistant := RenderAssistantComment(Marker{
		RunID:               "backup-run-secret",
		EventID:             "backup-event-secret",
		Model:               "gitclaw/tasks",
		IdempotencyKey:      "backup-idempotency-secret",
		PromptContextSHA:    "",
		PromptVisibleSkills: nil,
		PromptVisibleTools:  nil,
	}, "TASK_LEDGER_BACKUP_ASSISTANT_SECRET")
	backup := IssueBackup{
		Version:   1,
		Repo:      "owner/repo",
		EventName: "issues",
		Issue: IssueBackupIssue{
			Number:            132,
			Title:             "@gitclaw /tasks ledger",
			Body:              "TASK_LEDGER_BACKUP_ISSUE_SECRET",
			Author:            "alice",
			AuthorAssociation: "MEMBER",
			Labels:            []string{"gitclaw", "gitclaw:done"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "TASK_LEDGER_BACKUP_TRANSCRIPT_SECRET"}},
		Comments: []IssueBackupComment{
			{ID: 90, Body: "TASK_LEDGER_BACKUP_COMMENT_SECRET", Author: "alice", AuthorAssociation: "MEMBER"},
			{ID: 91, Body: assistant, Author: "github-actions[bot]", AuthorAssociation: "NONE"},
		},
	}
	data, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	backupPath := dir + "/issue-132.json"
	writeTestFile(t, dir, "issue-132.json", string(data))
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tasks", "ledger", "--backup", backupPath}); err != nil {
			t.Fatalf("tasks ledger returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Task Ledger Report",
		"scope: `local-backup`",
		"backup_repo: `owner/repo`",
		"backup_issue: `#132`",
		"task_ledger_status: `ok`",
		"ledger_source: `local-backup`",
		"current_task_status: `done`",
		"comments_scanned: `2`",
		"transcript_messages: `1`",
		"user_comments: `1`",
		"assistant_comments: `1`",
		"assistant_turns: `1`",
		"deterministic_turns: `1`",
		"model=`gitclaw/tasks`",
		"kind=`assistant-turn` source=`comment:91`",
		"raw_comment_bodies_included: `false`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("tasks ledger output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"TASK_LEDGER_BACKUP_ISSUE_SECRET",
		"TASK_LEDGER_BACKUP_TRANSCRIPT_SECRET",
		"TASK_LEDGER_BACKUP_COMMENT_SECRET",
		"TASK_LEDGER_BACKUP_ASSISTANT_SECRET",
		"backup-run-secret",
		"backup-event-secret",
		"backup-idempotency-secret",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("tasks ledger output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleTasksLedgerCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 133,
			"title": "@gitclaw /tasks ledger",
			"body": "Hidden tasks ledger handler token: TASK_LEDGER_HANDLER_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{133: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tasks ledger command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Task Ledger Report", "Generated without a model call", "model=\"gitclaw/tasks\"", "task_ledger_status: `ok`", "ledger_source: `issue-thread`", "current_task_status: `ready`", "comments_scanned: `0`", "raw_issue_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tasks ledger handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TASK_LEDGER_HANDLER_SECRET", "TASK_POLICY_BODY_SECRET", "TASK_SPEC_BODY_SECRET", "Issue Native Board"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tasks ledger handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[133], "gitclaw:done") || hasLabel(github.IssueLabels[133], "gitclaw:running") || hasLabel(github.IssueLabels[133], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[133])
	}
}
