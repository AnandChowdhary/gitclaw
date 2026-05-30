package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const standingOrdersReportTestBody = `# Standing Orders

STANDING_ORDERS_BODY_SECRET

## Program: Release Stewardship

**Authority:** Keep release notes and checks coherent.

**Trigger:** Weekly schedule or maintainer continuation request.

**Approval gate:** Ask before external side effects.

**Escalation:** Stop on missing credentials or permission broadening.
`

func TestRenderStandingOrdersReportAuditsOrdersWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, standingOrdersPath, standingOrdersReportTestBody)
	writeTestFile(t, dir, "AGENTS.md", "Reference .gitclaw/STANDING_ORDERS.md without leaking body.\nAGENTS_BODY_SECRET\n")
	writeTestFile(t, dir, proactiveWorkflowPath, "on:\n  workflow_dispatch:\n  schedule:\n    - cron: \"13 7 * * 1\"\n")
	writeTestFile(t, dir, ".gitclaw/proactive/release.md", "PROACTIVE_ORDER_BODY_SECRET\n")
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 113,
			"title": "@gitclaw /orders",
			"body": "Hidden orders report body token: ORDERS_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderStandingOrdersReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Standing Orders Report",
		"Generated without a model call",
		"standing_orders_status: `ok`",
		"standing_orders_path: `.gitclaw/STANDING_ORDERS.md`",
		"standing_orders_present: `true`",
		"standing_orders_loaded_for_model: `true`",
		"agents_path: `AGENTS.md`",
		"agents_present: `true`",
		"agents_mentions_standing_orders: `true`",
		"standing_order_programs: `1`",
		"programs_with_authority: `1`",
		"programs_with_trigger: `1`",
		"programs_with_approval_gate: `1`",
		"programs_with_escalation: `1`",
		"complete_programs: `1`",
		"proactive_prompt_files: `1`",
		"proactive_workflow_present: `true`",
		"proactive_schedule_trigger: `true`",
		"enforcement_strategy: `repo-reviewed-proactive-workflows-or-manual-trigger`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_orders_body_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Program Cards",
		"program=`01`",
		"title_sha256_12=",
		"complete=`true`",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("standing orders report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"STANDING_ORDERS_BODY_SECRET", "AGENTS_BODY_SECRET", "PROACTIVE_ORDER_BODY_SECRET", "ORDERS_REPORT_BODY_SECRET", "Release Stewardship"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("standing orders report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestOrdersListCommandReportsStandingOrders(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, standingOrdersPath, standingOrdersReportTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"orders", "list"}); err != nil {
			t.Fatalf("orders list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Standing Orders Report", "scope: `local-cli`", "standing_orders_status: `ok`", "standing_orders_loaded_for_model: `true`", "standing_order_programs: `1`", "complete_programs: `1`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("orders list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "STANDING_ORDERS_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("orders list leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleOrdersCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, standingOrdersPath, standingOrdersReportTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 114,
			"title": "@gitclaw /standing-orders",
			"body": "Hidden standing orders handler token: ORDERS_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{114: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic orders command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Standing Orders Report", "Generated without a model call", "model=\"gitclaw/orders\"", "standing_orders_status: `ok`", "standing_orders_loaded_for_model: `true`", "complete_programs: `1`", "model_call_required: `false`", "raw_orders_body_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("orders handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"ORDERS_HANDLER_BODY_SECRET", "STANDING_ORDERS_BODY_SECRET", "Release Stewardship"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("orders handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[114], "gitclaw:done") || hasLabel(github.IssueLabels[114], "gitclaw:running") || hasLabel(github.IssueLabels[114], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[114])
	}
}

func TestLoadRepoContextIncludesStandingOrders(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, standingOrdersPath, standingOrdersReportTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == standingOrdersPath {
			found = true
			if !strings.Contains(doc.Body, "STANDING_ORDERS_BODY_SECRET") {
				t.Fatalf("standing orders body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("standing orders file was not loaded into context: %#v", ctx.Documents)
	}
}
