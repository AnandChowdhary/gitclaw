package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const heartbeatReportTestWorkflow = `name: GitClaw Heartbeat

on:
  workflow_dispatch:
    inputs:
      label:
        description: Issue label to scan
        required: false
        default: gitclaw:heartbeat
      slot:
        description: Idempotency slot override
        required: false
      limit:
        description: Maximum issues to scan
        required: false
        default: "3"
  schedule:
    - cron: "17 * * * *"

jobs:
  heartbeat:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
      models: read
    steps:
      - run: go run ./cmd/gitclaw heartbeat --repo "$GITHUB_REPOSITORY"
`

func TestRenderHeartbeatReportAuditsWorkflowWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, heartbeatWorkflowPath, heartbeatReportTestWorkflow)
	writeTestFile(t, dir, heartbeatContextPath, "Heartbeat context token HEARTBEAT_CONTEXT_SECRET.\n")
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 111,
			"title": "@gitclaw /heartbeat",
			"body": "Hidden heartbeat report body token: HEARTBEAT_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:heartbeat"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	comments := []Comment{
		{ID: 1, Body: RenderHeartbeatComment(HeartbeatMarker{RunID: "run-1", Slot: "slot-1"}, "HEARTBEAT_COMMENT_SECRET")},
	}
	report := RenderHeartbeatReport(ev, cfg, comments)
	for _, want := range []string{
		"GitClaw Heartbeat Report",
		"Generated without a model call",
		"heartbeat_report_status: `ok`",
		"heartbeat_label: `gitclaw:heartbeat`",
		"trigger_label: `gitclaw`",
		"disabled_label: `gitclaw:disabled`",
		"workflow_path: `.github/workflows/gitclaw-heartbeat.yml`",
		"workflow_present: `true`",
		"workflow_dispatch_trigger: `true`",
		"schedule_trigger: `true`",
		"schedule_entries: `1`",
		"permissions_contents_read: `true`",
		"permissions_issues_write: `true`",
		"permissions_models_read: `true`",
		"workflow_inputs: `3`",
		"heartbeat_context_path: `.gitclaw/HEARTBEAT.md`",
		"heartbeat_context_present: `true`",
		"default_limit: `3`",
		"slot_strategy: `utc-hour-or-explicit`",
		"idempotency_marker: `gitclaw:heartbeat`",
		"heartbeat_marker_model_telemetry: `true`",
		"heartbeat_marker_prompt_provenance: `true`",
		"heartbeat_marker_usage_telemetry: `true`",
		"quiet_response: `HEARTBEAT_OK`",
		"model_call_required: `false`",
		"runner_model_call_required: `true`",
		"repository_mutation_allowed: `false`",
		"issue_scan_performed: `false`",
		"raw_bodies_included: `false`",
		"raw_heartbeat_body_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"llm_e2e_required_after_heartbeat_marker_change: `true`",
		"heartbeat_label_present: `true`",
		"disabled_label_present: `false`",
		"heartbeat_comments_now: `1`",
		"gitclaw heartbeat --repo <owner/repo>",
		"gitclaw heartbeat status",
		"### Verification Findings",
		"- none",
		"sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("heartbeat report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"HEARTBEAT_REPORT_BODY_SECRET", "HEARTBEAT_CONTEXT_SECRET", "HEARTBEAT_COMMENT_SECRET"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("heartbeat report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestHeartbeatStatusCommandReportsWithoutTokenOrModel(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, heartbeatWorkflowPath, heartbeatReportTestWorkflow)
	writeTestFile(t, dir, heartbeatContextPath, "Heartbeat context token HEARTBEAT_STATUS_CLI_SECRET.\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"heartbeat", "status"}); err != nil {
			t.Fatalf("heartbeat status returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Heartbeat Report", "scope: `local-cli`", "Generated without a model call", "heartbeat_report_status: `ok`", "model_call_required: `false`", "runner_model_call_required: `true`", "heartbeat_marker_model_telemetry: `true`", "heartbeat_marker_prompt_provenance: `true`", "heartbeat_marker_usage_telemetry: `true`", "llm_e2e_required_after_heartbeat_marker_change: `true`", "workflow_dispatch_trigger: `true`", "schedule_trigger: `true`", "permissions_models_read: `true`", "heartbeat_context_present: `true`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("heartbeat status output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "HEARTBEAT_STATUS_CLI_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("heartbeat status leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleHeartbeatCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, heartbeatWorkflowPath, heartbeatReportTestWorkflow)
	writeTestFile(t, dir, heartbeatContextPath, "Heartbeat context token HEARTBEAT_HANDLER_CONTEXT_SECRET.\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 112,
			"title": "@gitclaw /heartbeat",
			"body": "Hidden heartbeat handler token: HEARTBEAT_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:heartbeat"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{
		112: {
			{ID: 1, Body: RenderHeartbeatComment(HeartbeatMarker{RunID: "run-1", Slot: "slot-1"}, "HEARTBEAT_HANDLER_COMMENT_SECRET")},
		},
	}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic heartbeat command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Heartbeat Report", "Generated without a model call", "model=\"gitclaw/heartbeat\"", "heartbeat_report_status: `ok`", "heartbeat_label_present: `true`", "heartbeat_comments_now: `1`", "heartbeat_marker_model_telemetry: `true`", "heartbeat_marker_prompt_provenance: `true`", "heartbeat_marker_usage_telemetry: `true`", "llm_e2e_required_after_heartbeat_marker_change: `true`", "model_call_required: `false`", "runner_model_call_required: `true`", "raw_bodies_included: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(body, want) {
			t.Fatalf("heartbeat handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"HEARTBEAT_HANDLER_BODY_SECRET", "HEARTBEAT_HANDLER_CONTEXT_SECRET", "HEARTBEAT_HANDLER_COMMENT_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("heartbeat handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[112], "gitclaw:done") || hasLabel(github.IssueLabels[112], "gitclaw:running") || hasLabel(github.IssueLabels[112], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[112])
	}
}
