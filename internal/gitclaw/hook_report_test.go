package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const hookPolicyTestBody = `# Hooks

HOOK_POLICY_BODY_SECRET
`

const hookSpecTestBody = `---
name: repo-snapshot
events:
  - issue:opened
  - message:sent
mode: audit-only
delivery: issue-comment
requires_approval: true
---

# Repo Snapshot Hook

HOOK_SPEC_BODY_SECRET
`

func TestRenderHookReportAuditsHooksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/hooks/repo-snapshot.md", hookSpecTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 115,
			"title": "@gitclaw /hooks",
			"body": "Hidden hooks report body token: HOOKS_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderHookReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Hooks Report",
		"Generated without a model call",
		"hooks_status: `ok`",
		"hook_policy_path: `.gitclaw/HOOKS.md`",
		"hook_policy_present: `true`",
		"hook_policy_loaded_for_model: `true`",
		"hook_specs_dir: `.gitclaw/hooks`",
		"hook_specs: `1`",
		"hook_specs_with_frontmatter: `1`",
		"hook_events: `2`",
		"hook_specs_requiring_approval: `1`",
		"hook_specs_audit_only: `1`",
		"executable_handlers_present: `0`",
		"hook_execution_supported: `false`",
		"hook_execution_allowed: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_hook_bodies_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Hook Specs",
		"name=`repo-snapshot`",
		"path=`.gitclaw/hooks/repo-snapshot.md`",
		"frontmatter=`true`",
		"events=`2`",
		"mode=`audit-only`",
		"delivery=`issue-comment`",
		"requires_approval=`true`",
		"### Runtime Boundary",
		"hook handlers are not executed",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("hook report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"HOOK_POLICY_BODY_SECRET", "HOOK_SPEC_BODY_SECRET", "HOOKS_REPORT_BODY_SECRET", "Repo Snapshot Hook"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("hook report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestHooksListCommandReportsHooks(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/hooks/repo-snapshot.md", hookSpecTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"hooks", "list"}); err != nil {
			t.Fatalf("hooks list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Hooks Report", "scope: `local-cli`", "hooks_status: `ok`", "hook_policy_loaded_for_model: `true`", "hook_specs: `1`", "hook_events: `2`", "hook_execution_allowed: `false`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("hooks list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "HOOK_POLICY_BODY_SECRET") || strings.Contains(output, "HOOK_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("hooks list leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleHooksCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/hooks/repo-snapshot.md", hookSpecTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 116,
			"title": "@gitclaw /hook",
			"body": "Hidden hooks handler token: HOOKS_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{116: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic hooks command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Hooks Report", "Generated without a model call", "model=\"gitclaw/hooks\"", "hooks_status: `ok`", "hook_policy_loaded_for_model: `true`", "hook_specs: `1`", "hook_execution_allowed: `false`", "raw_hook_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("hooks handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"HOOKS_HANDLER_BODY_SECRET", "HOOK_POLICY_BODY_SECRET", "HOOK_SPEC_BODY_SECRET", "Repo Snapshot Hook"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("hooks handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[116], "gitclaw:done") || hasLabel(github.IssueLabels[116], "gitclaw:running") || hasLabel(github.IssueLabels[116], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[116])
	}
}

func TestLoadRepoContextIncludesHookPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, hookPolicyPath, hookPolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == hookPolicyPath {
			found = true
			if !strings.Contains(doc.Body, "HOOK_POLICY_BODY_SECRET") {
				t.Fatalf("hook policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("hook policy file was not loaded into context: %#v", ctx.Documents)
	}
}
