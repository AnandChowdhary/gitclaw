package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHooksRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/hooks/repo-snapshot.md", hookSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"hooks", "risk"}); err != nil {
			t.Fatalf("hooks risk returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Hook Risk Report", "scope: `local-cli`", "Generated without a model call", "hook_risk_status: `ok`", "verification_scope: `repo_reviewed_hook_metadata`", "hook_policy_present: `true`", "hook_policy_loaded_for_model: `true`", "hook_specs: `1`", "scanned_hook_specs: `1`", "hook_events: `2`", "hook_specs_requiring_approval: `1`", "hook_specs_audit_only: `1`", "executable_handlers_present: `0`", "scanned_executable_handlers: `0`", "surfaces_with_risk_findings: `0`", "hook_risk_findings: `0`", "hook_execution_supported: `false`", "hook_execution_allowed: `false`", "repository_mutation_allowed: `false`", "raw_hook_bodies_included: `false`", "raw_handler_bodies_included: `false`", "credential_values_included: `false`", "llm_e2e_required_after_hook_risk_change: `true`", "### Hook Policy Risk Card", "kind=`hook-policy` path=`.gitclaw/HOOKS.md`", "risk_findings=`0`", "risk_codes=`none`", "### Hook Spec Risk Cards", "kind=`hook-spec` name=`repo-snapshot` path=`.gitclaw/hooks/repo-snapshot.md`", "### Executable Handler Risk Cards", "kind=`handler` none", "### Risk Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("hooks risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "HOOK_POLICY_BODY_SECRET", "HOOK_SPEC_BODY_SECRET", "Repo Snapshot Hook"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("hooks risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderHookRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/hooks/risky.md", `---
name: risky
events:
  - issue:opened
mode: execute
delivery: webhook
requires_approval: false
---

api_key=HOOK_RISK_SPEC_SECRET
eval "$ISSUE_BODY"
retry forever
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	output := RenderHookRiskCLIReport(cfg)
	for _, want := range []string{"GitClaw Hook Risk Report", "hook_risk_status: `high`", "hook_specs: `1`", "scanned_hook_specs: `1`", "hook_specs_requiring_approval: `0`", "hook_specs_audit_only: `0`", "surfaces_with_risk_findings: `1`", "hook_risk_findings: `5`", "high_risk_findings: `2`", "warning_risk_findings: `3`", "code=`credential_material_in_hook`", "code=`untrusted_issue_body_execution`", "code=`hook_mode_not_audit_only`", "code=`hook_approval_gate_missing`", "code=`unbounded_hook_loop`", "line_sha256_12=", "risk_max_severity=`high`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("hooks risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"HOOK_RISK_SPEC_SECRET", "eval \"$ISSUE_BODY\"", "retry forever", "api_key="} {
		if strings.Contains(output, notWant) {
			t.Fatalf("hooks risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderHookReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/hooks/repo-snapshot.md", "api_key=HOOK_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 117,
			"title": "@gitclaw /hooks risk",
			"body": "Hidden hooks risk body token: HOOK_RISK_BODY_SECRET.",
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
	body := RenderHookReport(ev, cfg)
	for _, want := range []string{"GitClaw Hook Risk Report", "repository: `owner/repo`", "issue: `#117`", "hook_risk_status: `high`", "code=`credential_material_in_hook`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("hooks risk report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "HOOK_RISK_BODY_SECRET") || strings.Contains(body, "HOOK_ROUTE_RISK_SPEC_SECRET") {
		t.Fatalf("hooks risk report leaked body token:\n%s", body)
	}
}

func TestHandleHooksRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/hooks/repo-snapshot.md", hookSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 118,
			"title": "@gitclaw /hook risk",
			"body": "Hidden hooks risk handler token: HOOKS_RISK_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{118: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic hooks risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Hook Risk Report", "Generated without a model call", "model=\"gitclaw/hooks\"", "hook_risk_status: `ok`", "verification_scope: `repo_reviewed_hook_metadata`", "raw_hook_bodies_included: `false`", "raw_handler_bodies_included: `false`", "llm_e2e_required_after_hook_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("hooks risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"HOOKS_RISK_HANDLER_BODY_SECRET", "HOOK_POLICY_BODY_SECRET", "HOOK_SPEC_BODY_SECRET", "Repo Snapshot Hook"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("hooks risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[118], "gitclaw:done") || hasLabel(github.IssueLabels[118], "gitclaw:running") || hasLabel(github.IssueLabels[118], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[118])
	}
}
