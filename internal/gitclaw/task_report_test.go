package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const taskPolicyTestBody = `# Tasks

TASK_POLICY_BODY_SECRET
`

const taskSpecTestBody = `---
name: issue-native-board
kind: board
mode: issue-native
statuses:
  - ready
  - running
  - blocked
  - done
labels:
  - gitclaw
  - gitclaw:running
  - gitclaw:done
  - gitclaw:needs-human
requires_approval: true
---

# Issue Native Board

This board does not spawn detached workers.
TASK_SPEC_BODY_SECRET
`

func TestRenderTaskReportAuditsCurrentIssueWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 119,
			"title": "@gitclaw /tasks",
			"body": "Hidden tasks report body token: TASKS_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:needs-human"}]
		},
		"comment": {
			"id": 77,
			"body": "@gitclaw /tasks\nHidden comment body token: TASKS_COMMENT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	comments := []Comment{{ID: 77, Body: "TASKS_COMMENT_BODY_SECRET", AuthorAssociation: "MEMBER", User: User{Login: "alice"}}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "TASKS_REPORT_BODY_SECRET", Actor: "alice", Trusted: true},
		{Role: "user", Body: "TASKS_COMMENT_BODY_SECRET", Actor: "alice", Trusted: true, CommentID: 77},
	}
	report := RenderTaskReport(ev, cfg, comments, transcript)
	for _, want := range []string{
		"GitClaw Tasks Report",
		"Generated without a model call",
		"tasks_status: `ok`",
		"task_policy_path: `.gitclaw/TASKS.md`",
		"task_policy_present: `true`",
		"task_policy_loaded_for_model: `true`",
		"task_specs_dir: `.gitclaw/tasks`",
		"task_specs: `1`",
		"task_specs_with_frontmatter: `1`",
		"task_statuses_declared: `4`",
		"task_labels_declared: `4`",
		"task_specs_requiring_approval: `1`",
		"task_specs_issue_native: `1`",
		"task_storage_backend: `github-issues`",
		"sqlite_task_db_required: `false`",
		"detached_worker_supported: `false`",
		"kanban_dispatcher_supported: `false`",
		"task_flow_execution_supported: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_task_bodies_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"current_issue_task: `true`",
		"current_task_status: `blocked`",
		"current_task_labels: `2`",
		"current_task_comments: `1`",
		"current_task_transcript_messages: `2`",
		"### Task Specs",
		"name=`issue-native-board`",
		"path=`.gitclaw/tasks/issue-native-board.md`",
		"frontmatter=`true`",
		"kind=`board`",
		"mode=`issue-native`",
		"statuses=`4`",
		"labels=`4`",
		"requires_approval=`true`",
		"### Current Issue Task",
		"status=`blocked`",
		"needs_human_label_present=`true`",
		"write_requested_label_present=`false`",
		"### Runtime Boundary",
		"GitHub issues are the durable task rows",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("task report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"TASK_POLICY_BODY_SECRET", "TASK_SPEC_BODY_SECRET", "TASKS_REPORT_BODY_SECRET", "TASKS_COMMENT_BODY_SECRET", "Issue Native Board"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("task report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestTasksListCommandReportsTasks(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tasks", "list"}); err != nil {
			t.Fatalf("tasks list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tasks Report", "scope: `local-cli`", "tasks_status: `ok`", "task_policy_loaded_for_model: `true`", "task_specs: `1`", "task_storage_backend: `github-issues`", "detached_worker_supported: `false`", "model_call_required: `false`", "current_issue_task=`false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tasks list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "TASK_POLICY_BODY_SECRET") || strings.Contains(output, "TASK_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("tasks list leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleTasksCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 120,
			"title": "@gitclaw /task",
			"body": "Hidden tasks handler token: TASKS_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{120: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tasks command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Tasks Report", "Generated without a model call", "model=\"gitclaw/tasks\"", "tasks_status: `ok`", "task_policy_loaded_for_model: `true`", "task_specs: `1`", "current_task_status: `ready`", "raw_task_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tasks handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"TASKS_HANDLER_BODY_SECRET", "TASK_POLICY_BODY_SECRET", "TASK_SPEC_BODY_SECRET", "Issue Native Board"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("tasks handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[120], "gitclaw:done") || hasLabel(github.IssueLabels[120], "gitclaw:running") || hasLabel(github.IssueLabels[120], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[120])
	}
}

func TestLoadRepoContextIncludesTaskPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == taskPolicyPath {
			found = true
			if !strings.Contains(doc.Body, "TASK_POLICY_BODY_SECRET") {
				t.Fatalf("task policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("task policy file was not loaded into context: %#v", ctx.Documents)
	}
}
