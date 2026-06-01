package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestProactiveScheduleCommandReportsWorkflowCalendarWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProactiveScheduleFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"proactive", "schedule"}); err != nil {
			t.Fatalf("proactive schedule returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Proactive Schedule Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"proactive_schedule_status: `ok`",
		"schedule_strategy: `github-actions-cron-to-issue-dispatch`",
		"upstream_pattern: `openclaw-cron-hermes-cron-skill-backed-fresh-session`",
		"scheduler_runtime: `GitHub Actions schedule`",
		"state_storage: `gitclaw:proactive-run issues`",
		"workflow_files_indexed: `1`",
		"workflow_files_present: `1`",
		"scheduled_workflows: `1`",
		"workflow_dispatch_workflows: `1`",
		"cron_entries: `1`",
		"cron_entries_valid: `1`",
		"prompt_files: `1`",
		"skill_backed_prompt_files: `1`",
		"prompt_skill_hints: `1`",
		"not_before_supported_workflows: `1`",
		"exact_timing_supported: `true`",
		"heartbeat_is_approximate_channel: `true`",
		"fresh_issue_thread_per_name_slot: `true`",
		"recursive_schedule_creation_allowed: `false`",
		"no_agent_mode_supported: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_proactive_schedule_change: `true`",
		"### Workflow Schedule Entries",
		"kind=`workflow-schedule` name=`generic` path=`.github/workflows/gitclaw-proactive.yml`",
		"cron=`23 8 * * 1`",
		"cadence=`weekly`",
		"prompt_file_refs=`1`",
		"not_before_supported=`true`",
		"raw_workflow_body_included=`false`",
		"### Prompt Schedule Cards",
		"kind=`prompt-schedule` name=`repo-hygiene` path=`.gitclaw/proactive/repo-hygiene.md`",
		"skill_hints=`repo-reader`",
		"raw_prompt_body_included=`false`",
		"### Schedule Gates",
		"schedule_source_gate=`reviewed-github-workflow`",
		"heartbeat_boundary_gate=`heartbeat-is-approximate-monitoring-not-exact-schedule`",
		"recursive_schedule_gate=`disabled-inside-proactive-run`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("proactive schedule output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{
		"- repository:",
		"- issue:",
		"PROACTIVE_SCHEDULE_WORKFLOW_SECRET",
		"PROACTIVE_SCHEDULE_PROMPT_SECRET",
		"Run the secret scheduled task",
		"GITCLAW_PROACTIVE_PROMPT_FILE",
	} {
		if strings.Contains(output, notWant) {
			t.Fatalf("proactive schedule output leaked %q:\n%s", notWant, output)
		}
	}
}

func TestHandleProactiveScheduleCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeProactiveScheduleFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 136,
			"title": "@gitclaw /proactive schedule",
			"body": "Hidden proactive schedule token: PROACTIVE_SCHEDULE_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{136: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic proactive schedule command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Proactive Schedule Report",
		"Generated without a model call",
		"model=\"gitclaw/proactive\"",
		"repository: `owner/repo`",
		"issue: `#136`",
		"requested_proactive_command: `schedule`",
		"proactive_command_status: `ok`",
		"proactive_schedule_status: `ok`",
		"cron_entries: `1`",
		"skill_backed_prompt_files: `1`",
		"proactive_run_issue: `false`",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_proactive_schedule_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("proactive schedule handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"PROACTIVE_SCHEDULE_BODY_SECRET", "PROACTIVE_SCHEDULE_WORKFLOW_SECRET", "PROACTIVE_SCHEDULE_PROMPT_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("proactive schedule handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[136], "gitclaw:done") || hasLabel(github.IssueLabels[136], "gitclaw:running") || hasLabel(github.IssueLabels[136], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[136])
	}
}

func writeProactiveScheduleFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".github/workflows/gitclaw-proactive.yml", `name: GitClaw Proactive
on:
  workflow_dispatch:
    inputs:
      not_before:
        required: false
  schedule:
    - cron: "23 8 * * 1"
jobs:
  enqueue:
    steps:
      - run: go run ./cmd/gitclaw proactive enqueue --prompt-file .gitclaw/proactive/repo-hygiene.md
# PROACTIVE_SCHEDULE_WORKFLOW_SECRET
`)
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", `<!-- gitclaw:proactive-skills repo-reader -->
Run the secret scheduled task.
PROACTIVE_SCHEDULE_PROMPT_SECRET
`)
}
