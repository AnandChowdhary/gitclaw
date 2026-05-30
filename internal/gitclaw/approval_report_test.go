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
