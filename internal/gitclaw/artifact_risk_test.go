package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestArtifactsRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/artifacts/prompt-artifact.md", artifactSpecTestBody)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", artifactWorkflowTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"artifacts", "risk"}); err != nil {
			t.Fatalf("artifacts risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Artifact Risk Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"artifact_risk_status: `ok`",
		"verification_scope: `github_actions_artifact_metadata`",
		"artifact_policy_present: `true`",
		"artifact_policy_loaded_for_model: `true`",
		"artifact_specs: `1`",
		"scanned_artifact_specs: `1`",
		"workflow_artifact_uploaders: `1`",
		"scanned_workflow_artifact_uploaders: `1`",
		"artifact_specs_requiring_approval: `1`",
		"artifact_specs_requiring_redaction: `1`",
		"artifact_retention_days_declared: `7`",
		"prompt_artifact_default_enabled: `false`",
		"prompt_artifact_label: `gitclaw:e2e-prompt-artifact`",
		"prompt_artifact_env_path_configured: `false`",
		"artifact_storage_backend: `github-actions-artifacts`",
		"durable_backup_backend: `git-backup-branch`",
		"surfaces_with_risk_findings: `0`",
		"artifact_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"artifact_body_printing_allowed: `false`",
		"raw_artifact_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credential_values_included: `false`",
		"artifact_as_hidden_state_allowed: `false`",
		"external_artifact_storage_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_artifact_risk_change: `true`",
		"### Artifact Policy Risk Card",
		"kind=`artifact-policy` path=`.gitclaw/ARTIFACTS.md`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Artifact Spec Risk Cards",
		"kind=`artifact-spec` name=`prompt-artifact` path=`.gitclaw/artifacts/prompt-artifact.md`",
		"storage=`github-actions-artifact`",
		"label=`gitclaw:e2e-prompt-artifact`",
		"redaction_required=`true`",
		"requires_approval=`true`",
		"### Workflow Artifact Risk Cards",
		"kind=`artifact-workflow` path=`.github/workflows/gitclaw.yml`",
		"upload_artifact_actions=`actions/upload-artifact@v6`",
		"retention_days=`7`",
		"if_no_files_found_error=`true`",
		"prompt_artifact_label_gate=`true`",
		"prompt_artifact_path_env=`true`",
		"### Current Artifact Request Risk Card",
		"scope=`local-cli` current_issue_artifact_request=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("artifacts risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "ARTIFACT_POLICY_BODY_SECRET", "ARTIFACT_SPEC_BODY_SECRET", "GITCLAW_ARTIFACTS_CONTEXT_V1", "This declarative artifact record"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("artifacts risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderArtifactRiskReportFlagsSpecAndWorkflowRisksWithoutBodies(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/artifacts/risky.md", `---
name: risky
kind: prompt
storage: s3
filename: prompt.md
workflow: .github/workflows/gitclaw.yml
label:
retention_days: 90
redaction_required: false
requires_approval: false
---

api_key=ARTIFACT_RISK_SPEC_SECRET
upload raw prompt
use artifacts as hidden state
retry forever
`)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", `name: Risky

on:
  workflow_dispatch:

jobs:
  upload:
    runs-on: ubuntu-latest
    steps:
      - run: cat "$GITCLAW_PROMPT_ARTIFACT_PATH"
      - uses: actions/upload-artifact@v4
        with:
          name: risky
          path: prompt.md
          retention-days: 90
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	output := RenderArtifactRiskCLIReport(cfg)
	for _, want := range []string{
		"GitClaw Artifact Risk Report",
		"artifact_risk_status: `high`",
		"artifact_specs: `1`",
		"scanned_artifact_specs: `1`",
		"workflow_artifact_uploaders: `1`",
		"scanned_workflow_artifact_uploaders: `1`",
		"artifact_specs_requiring_approval: `0`",
		"artifact_specs_requiring_redaction: `0`",
		"artifact_retention_days_declared: `90`",
		"surfaces_with_risk_findings: `2`",
		"artifact_risk_findings: `16`",
		"high_risk_findings: `7`",
		"warning_risk_findings: `9`",
		"code=`artifact_redaction_not_required`",
		"code=`credential_material_in_artifact`",
		"code=`raw_artifact_payload_logged`",
		"code=`unredacted_prompt_artifact`",
		"code=`artifact_hidden_state`",
		"code=`artifact_storage_not_actions`",
		"code=`external_artifact_storage`",
		"code=`artifact_label_missing`",
		"code=`artifact_retention_too_long`",
		"code=`artifact_approval_gate_missing`",
		"code=`artifact_upload_action_not_v6`",
		"code=`artifact_workflow_missing_if_no_files_found_error`",
		"code=`artifact_workflow_missing_label_gate`",
		"code=`artifact_workflow_retention_too_long`",
		"code=`unbounded_artifact_collection`",
		"line_sha256_12=",
		"risk_max_severity=`high`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("artifacts risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"ARTIFACT_RISK_SPEC_SECRET", "api_key=", "upload raw prompt", "use artifacts as hidden state", "retry forever", "cat \"$GITCLAW_PROMPT_ARTIFACT_PATH\""} {
		if strings.Contains(output, notWant) {
			t.Fatalf("artifacts risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderArtifactReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/artifacts/prompt-artifact.md", artifactSpecTestBody+"\napi_key=ARTIFACT_ROUTE_RISK_SPEC_SECRET\n")
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", artifactWorkflowTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /artifacts risk",
			"body": "Hidden artifacts risk body token: ARTIFACTS_RISK_BODY_SECRET.",
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
	body := RenderArtifactReport(ev, cfg)
	for _, want := range []string{"GitClaw Artifact Risk Report", "repository: `owner/repo`", "issue: `#125`", "artifact_risk_status: `high`", "code=`credential_material_in_artifact`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("artifacts risk report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"ARTIFACTS_RISK_BODY_SECRET", "ARTIFACT_ROUTE_RISK_SPEC_SECRET", "api_key="} {
		if strings.Contains(body, notWant) {
			t.Fatalf("artifacts risk report leaked body token %q:\n%s", notWant, body)
		}
	}
}

func TestHandleArtifactsRiskCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/artifacts/prompt-artifact.md", artifactSpecTestBody)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", artifactWorkflowTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /artifact risk",
			"body": "Hidden artifacts risk handler token: ARTIFACTS_RISK_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{126: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic artifacts risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Artifact Risk Report", "Generated without a model call", "model=\"gitclaw/artifacts\"", "artifact_risk_status: `ok`", "verification_scope: `github_actions_artifact_metadata`", "raw_artifact_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "llm_e2e_required_after_artifact_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("artifacts risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"ARTIFACTS_RISK_HANDLER_BODY_SECRET", "ARTIFACT_POLICY_BODY_SECRET", "ARTIFACT_SPEC_BODY_SECRET", "GITCLAW_ARTIFACTS_CONTEXT_V1", "This declarative artifact record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("artifacts risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}
