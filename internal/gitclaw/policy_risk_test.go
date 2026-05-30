package gitclaw

import (
	"strings"
	"testing"
)

const policyRiskWorkflowBody = `name: GitClaw
on:
  issues:
    types: [opened]
jobs:
  preflight:
    permissions:
      contents: read
      issues: read
  handle:
    permissions:
      contents: read
      issues: write
      models: read
  backup:
    concurrency:
      group: gitclaw-backups-${{ github.repository }}
      cancel-in-progress: false
    permissions:
      contents: write
      issues: read
`

func TestRenderPolicyRiskReportShowsBoundaryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", policyRiskWorkflowBody+"POLICY_RISK_WORKFLOW_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 128,
			"title": "@gitclaw /policy risk",
			"body": "Please implement this without leaking POLICY_RISK_BODY_SECRET.",
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
	transcript := BuildTranscript(ev, nil)
	repoContext := RepoContext{ToolOutputs: []ToolOutput{WriteRequestPolicyOutput()}}
	report := RenderPolicyReport(ev, cfg, Preflight(ev, cfg), transcript, repoContext, DetectWriteRequest(transcript))
	for _, want := range []string{
		"GitClaw Policy Risk Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#128`",
		"event_kind: `issue_opened`",
		"preflight_allowed: `true`",
		"actor_association: `MEMBER`",
		"actor_trusted: `true`",
		"write_request_detected: `true`",
		"policy_risk_status: `ok`",
		"verification_scope: `policy-trust-labels-workflow-permissions-and-write-boundary`",
		"run_mode: `read-only`",
		"trusted_associations: `3`",
		"broad_trusted_associations: `0`",
		"managed_labels_configured: `9`",
		"duplicate_managed_labels: `0`",
		"workflow_verify_status: `ok`",
		"workflow_present: `true`",
		"expected_jobs: `3`",
		"jobs_present: `3`",
		"expected_permissions: `7`",
		"expected_write_permissions: `2`",
		"permissions_present: `7`",
		"missing_permissions: `0`",
		"unexpected_write_permissions: `0`",
		"backup_concurrency_group: `true`",
		"backup_concurrency_cancel_safe: `true`",
		"policy_outputs_hashed: `1`",
		"policy_output_present: `true`",
		"policy_risk_findings: `0`",
		"write_request_policy_output_body_included: `false`",
		"write_actions_supported: `false`",
		"write_actions_enabled: `false`",
		"repository_mutation_allowed: `false`",
		"host_exec_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_inputs_included: `false`",
		"llm_e2e_required_after_policy_risk_change: `true`",
		"### Trust Boundary Risk Card",
		"kind=`trust-boundary`",
		"### Managed Label Risk Card",
		"kind=`managed-labels`",
		"### Workflow Permission Risk Cards",
		"job=`handle` present=`true`",
		"expected=`contents:read, issues:write, models:read`",
		"### Policy Output Risk Cards",
		"kind=`policy-output` name=`gitclaw.policy`",
		"input_sha256_12=",
		"output_sha256_12=",
		"output_body_included=`false`",
		"### Runtime Boundary Risk Card",
		"kind=`runtime-boundary` run_mode=`read-only`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("policy risk report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"POLICY_RISK_BODY_SECRET", "POLICY_RISK_WORKFLOW_SECRET", "Please implement this", "Current GitClaw mode is read-only", "input=`write-request`"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("policy risk report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestBuildPolicyRiskReportFindsBroadTrustAndWorkflowRisk(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", `name: GitClaw
jobs:
  preflight:
    permissions:
      contents: write
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg.AllowedAssociations = map[string]bool{"OWNER": true, "NONE": true}
	report := BuildPolicyRiskReport(cfg, RepoContext{})
	rendered := renderPolicyRiskReport(Event{}, cfg, PreflightDecision{}, nil, RepoContext{}, false, false)
	for _, want := range []string{
		"policy_risk_status: `high`",
		"broad_trusted_associations: `1`",
		"workflow_verify_status: `error`",
		"unexpected_write_permissions:",
		"policy_risk_findings:",
		"code=`broad_trusted_association`",
		"code=`workflow_permission_missing`",
		"code=`workflow_job_missing`",
		"code=`backup_concurrency_missing`",
		"code=`backup_concurrency_cancel_unsafe`",
		"line_sha256_12=",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("policy risk failure report missing %q:\n%s", want, rendered)
		}
	}
	if report.Status != "high" || report.HighRiskFindings == 0 {
		t.Fatalf("unexpected risk report: %#v", report)
	}
}
