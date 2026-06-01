package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSecurityAuditReportAggregatesSurfacesWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSafeConfigRiskFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	body, err := RenderSecurityCLIReport(cfg, repoContext)
	if err != nil {
		t.Fatalf("RenderSecurityCLIReport returned error: %v", err)
	}
	for _, want := range []string{
		"GitClaw Security Audit Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"security_audit_status:",
		"verification_scope: `openclaw_personal_assistant_security_audit`",
		"trust_model: `personal-assistant-single-operator`",
		"runtime_boundary: `github-actions-ephemeral-runner`",
		"gateway_server_required: `false`",
		"hostile_multi_tenant_supported: `false`",
		"surfaces_scanned: `8`",
		"config_risk_status:",
		"policy_risk_status:",
		"sandbox_risk_status:",
		"channel_risk_status:",
		"tool_risk_status:",
		"skill_risk_status:",
		"plugin_risk_status:",
		"secrets_risk_status:",
		"raw_config_bodies_included: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"repository_mutation_allowed: `false`",
		"host_exec_allowed: `false`",
		"llm_e2e_required_after_security_audit_change: `true`",
		"### Trust Boundary",
		"split_trust_boundaries_for_untrusted_users=`true`",
		"### Surface Cards",
		"surface=`config`",
		"surface=`secrets`",
		"### Control Plane Gates",
		"### Audit Boundaries",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("security audit report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"permissions:", "workflow_dispatch:", "api_key", "GITCLAW_CONFIG_RISK_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("security audit leaked body text %q:\n%s", leaked, body)
		}
	}
}

func TestHandleSecurityCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeSafeConfigRiskFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 509,
			"title": "@gitclaw /security audit",
			"body": "Hidden security body token: SECURITY_AUDIT_HANDLER_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		t.Fatalf("LoadEffectiveConfig returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{509: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic security audit", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"model=\"gitclaw/security\"",
		"GitClaw Security Audit Report",
		"repository: `owner/repo`",
		"issue: `#509`",
		"actor_association: `MEMBER`",
		"trust_model: `personal-assistant-single-operator`",
		"surfaces_scanned: `8`",
		"raw_issue_bodies_included: `false`",
		"llm_e2e_required_after_security_audit_change: `true`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("security audit comment missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SECURITY_AUDIT_HANDLER_SECRET", "permissions:", "workflow_dispatch:"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("security audit comment leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[509], "gitclaw:done") || hasLabel(github.IssueLabels[509], "gitclaw:running") || hasLabel(github.IssueLabels[509], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[509])
	}
}
