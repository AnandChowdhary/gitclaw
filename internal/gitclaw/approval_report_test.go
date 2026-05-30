package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderApprovalReportShowsGatesWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /approvals",
			"body": "Please implement this without leaking APPROVAL_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:approved"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	transcript := BuildTranscript(ev, nil)
	report := RenderApprovalReport(ev, DefaultConfig(), Preflight(ev, DefaultConfig()), transcript, DetectWriteRequest(transcript))
	for _, want := range []string{
		"GitClaw Approvals Report",
		"Generated without a model call",
		"approval_status: `approved_but_write_mode_disabled`",
		"approval_decision: `proposal_only_approved_label_seen`",
		"approval_store: `github-issue-labels`",
		"approval_scope: `per-issue`",
		"approval_label: `gitclaw:approved`",
		"needs_human_label: `gitclaw:needs-human`",
		"write_requested_label: `gitclaw:write-requested`",
		"write_request_detected: `true`",
		"approved_label_present: `true`",
		"write_actions_enabled: `false`",
		"raw_bodies_included: `false`",
		"raw_approval_payloads_included: `false`",
		"gate=`trusted_actor` status=`passed`",
		"gate=`approval_label` status=`present`",
		"gate=`write_mode` status=`blocked`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("approval report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"APPROVAL_BODY_SECRET", "Please implement this"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("approval report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderApprovalRiskReportShowsBoundaryWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 127,
			"title": "@gitclaw /approvals risk",
			"body": "Please implement this without leaking APPROVAL_RISK_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:approved"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	transcript := BuildTranscript(ev, nil)
	report := RenderApprovalReport(ev, DefaultConfig(), Preflight(ev, DefaultConfig()), transcript, DetectWriteRequest(transcript))
	for _, want := range []string{
		"GitClaw Approvals Risk Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#127`",
		"event_kind: `issue_opened`",
		"preflight_allowed: `true`",
		"actor_association: `MEMBER`",
		"actor_trusted: `true`",
		"write_request_detected: `true`",
		"write_requested_label_present: `true`",
		"approved_label_present: `true`",
		"approval_risk_status: `ok`",
		"verification_scope: `approval-gates-labels-and-read-only-boundary`",
		"approval_store: `github-issue-labels`",
		"approval_scope: `per-issue`",
		"trusted_associations: `3`",
		"broad_trusted_associations: `0`",
		"approval_labels_configured: `3`",
		"duplicate_approval_labels: `0`",
		"approval_managed_label_collisions: `0`",
		"approval_risk_findings: `0`",
		"write_actions_supported: `false`",
		"write_actions_enabled: `false`",
		"repository_mutation_allowed: `false`",
		"host_exec_allowed: `false`",
		"approval_payloads_included: `false`",
		"raw_bodies_included: `false`",
		"llm_e2e_required_after_approval_risk_change: `true`",
		"### Approval Gate Risk Card",
		"kind=`approval-gates`",
		"write_request_detection=`heuristic-transcript-scan`",
		"### Trusted Association Risk Cards",
		"association=`OWNER` trusted=`true` broad=`false`",
		"### Approval Label Risk Cards",
		"role=`approved` label=`gitclaw:approved`",
		"### Runtime Boundary Risk Card",
		"kind=`runtime-boundary` write_actions_supported=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("approval risk report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"APPROVAL_RISK_BODY_SECRET", "Please implement this"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("approval risk report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestBuildApprovalRiskReportWarnsOnBroadTrust(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowedAssociations = map[string]bool{"OWNER": true, "CONTRIBUTOR": true}
	report := BuildApprovalRiskReport(cfg)
	for _, want := range []string{
		"approval_risk_status: `warn`",
		"broad_trusted_associations: `1`",
		"warning_risk_findings: `1`",
		"code=`broad_trusted_association`",
		"association=`CONTRIBUTOR` trusted=`true` broad=`true`",
	} {
		rendered := renderApprovalRiskReport(Event{}, cfg, PreflightDecision{}, nil, false, false)
		if !strings.Contains(rendered, want) {
			t.Fatalf("approval risk broad-trust report missing %q:\n%s", want, rendered)
		}
	}
	if report.Status != "warn" || report.WarningRiskFindings != 1 {
		t.Fatalf("unexpected broad-trust report: %#v", report)
	}
}
