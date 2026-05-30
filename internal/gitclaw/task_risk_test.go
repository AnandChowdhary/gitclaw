package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestTasksRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tasks", "risk"}); err != nil {
			t.Fatalf("tasks risk returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Task Risk Report", "scope: `local-cli`", "Generated without a model call", "task_risk_status: `ok`", "verification_scope: `github_issue_task_metadata`", "task_policy_present: `true`", "task_policy_loaded_for_model: `true`", "task_specs: `1`", "scanned_task_specs: `1`", "task_statuses_declared: `4`", "task_labels_declared: `4`", "task_specs_requiring_approval: `1`", "task_specs_issue_native: `1`", "current_issue_task: `false`", "current_task_comments: `0`", "current_task_transcript_messages: `0`", "surfaces_with_risk_findings: `0`", "task_risk_findings: `0`", "task_storage_backend: `github-issues`", "sqlite_task_db_required: `false`", "detached_worker_supported: `false`", "kanban_dispatcher_supported: `false`", "task_flow_execution_supported: `false`", "repository_mutation_allowed: `false`", "raw_task_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "credential_values_included: `false`", "llm_e2e_required_after_task_risk_change: `true`", "### Task Policy Risk Card", "kind=`task-policy` path=`.gitclaw/TASKS.md`", "risk_findings=`0`", "risk_codes=`none`", "### Task Spec Risk Cards", "kind=`task-spec` name=`issue-native-board` path=`.gitclaw/tasks/issue-native-board.md`", "### Current Task Thread Risk Card", "scope=`local-cli` current_issue_task=`false`", "### Risk Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tasks risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "TASK_POLICY_BODY_SECRET", "TASK_SPEC_BODY_SECRET", "Issue Native Board"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("tasks risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderTaskRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/risky.md", `---
name: risky
kind: flow
mode: detached-worker
statuses:
  - ready
labels:
  - gitclaw
requires_approval: false
---

api_key=TASK_RISK_SPEC_SECRET
spawn detached worker
requires sqlite task db
retry forever
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	output := RenderTaskRiskCLIReport(cfg)
	for _, want := range []string{"GitClaw Task Risk Report", "task_risk_status: `high`", "task_specs: `1`", "scanned_task_specs: `1`", "task_specs_requiring_approval: `0`", "task_specs_issue_native: `0`", "surfaces_with_risk_findings: `1`", "task_risk_findings: `6`", "high_risk_findings: `2`", "warning_risk_findings: `4`", "code=`credential_material_in_task`", "code=`detached_worker_spawn`", "code=`task_mode_not_issue_native`", "code=`task_approval_gate_missing`", "code=`external_task_database`", "code=`unbounded_task_loop`", "line_sha256_12=", "risk_max_severity=`high`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tasks risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"TASK_RISK_SPEC_SECRET", "spawn detached worker", "requires sqlite task db", "retry forever", "api_key="} {
		if strings.Contains(output, notWant) {
			t.Fatalf("tasks risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderTaskReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/tasks/issue-native-board.md", "api_key=TASK_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 121,
			"title": "@gitclaw /tasks risk",
			"body": "Hidden tasks risk body token: TASK_RISK_BODY_SECRET.",
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
	cfg.Workdir = root
	body := RenderTaskReport(ev, cfg, nil, nil)
	for _, want := range []string{"GitClaw Task Risk Report", "repository: `owner/repo`", "issue: `#121`", "task_risk_status: `high`", "code=`credential_material_in_task`", "current_task_status: `ready`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tasks risk report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "TASK_RISK_BODY_SECRET") || strings.Contains(body, "TASK_ROUTE_RISK_SPEC_SECRET") {
		t.Fatalf("tasks risk report leaked body token:\n%s", body)
	}
}

func TestHandleTasksRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, taskPolicyPath, taskPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/tasks/issue-native-board.md", taskSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 122,
			"title": "@gitclaw /task risk",
			"body": "Hidden tasks risk handler token: TASKS_RISK_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{122: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tasks risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Task Risk Report", "Generated without a model call", "model=\"gitclaw/tasks\"", "task_risk_status: `ok`", "verification_scope: `github_issue_task_metadata`", "raw_task_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "llm_e2e_required_after_task_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tasks risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"TASKS_RISK_HANDLER_BODY_SECRET", "TASK_POLICY_BODY_SECRET", "TASK_SPEC_BODY_SECRET", "Issue Native Board"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("tasks risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[122], "gitclaw:done") || hasLabel(github.IssueLabels[122], "gitclaw:running") || hasLabel(github.IssueLabels[122], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[122])
	}
}
