package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestProactiveChainCommandReportsContextFromWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProactiveChainFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"proactive", "chain"}); err != nil {
			t.Fatalf("proactive chain returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Proactive Chain Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"proactive_chain_status: `ok`",
		"chain_strategy: `github-actions-issue-thread-context-from`",
		"upstream_pattern: `hermes-cron-context-from-openclaw-skill-backed-jobs`",
		"prompt_files: `2`",
		"prompt_files_with_context_from: `1`",
		"chain_edges: `1`",
		"missing_context_sources: `0`",
		"self_references: `0`",
		"cycle_nodes: `0`",
		"skill_backed_prompt_files: `2`",
		"prompt_skill_hints: `1`",
		"fresh_issue_thread_per_name_slot: `true`",
		"context_from_metadata_only: `true`",
		"recursive_schedule_creation_allowed: `false`",
		"no_agent_mode_supported: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_proactive_chain_change: `true`",
		"### Prompt Chain Cards",
		"kind=`prompt-chain` name=`daily-summary` path=`.gitclaw/proactive/daily-summary.md`",
		"skill_hints=`repo-reader`",
		"context_from_refs=`1`",
		"resolved_context_sources=`repo-hygiene`",
		"missing_context_source_hashes=`none`",
		"raw_prompt_body_included=`false`",
		"kind=`prompt-chain` name=`repo-hygiene` path=`.gitclaw/proactive/repo-hygiene.md`",
		"### Chain Edges",
		"kind=`chain-edge` from=`repo-hygiene` to=`daily-summary`",
		"source_path=`.gitclaw/proactive/repo-hygiene.md`",
		"target_path=`.gitclaw/proactive/daily-summary.md`",
		"### Chain Gates",
		"context_from_gate=`resolved`",
		"prompt_body_gate=`metadata-hashes-and-context-refs-only`",
		"skill_hint_gate=`metadata-only`",
		"issue_thread_gate=`one-proactive-run-issue-per-name-slot`",
		"scheduler_gate=`github-actions-workflow-dispatch`",
		"recursive_schedule_gate=`disabled-inside-proactive-run`",
		"model_e2e_gate=`required`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("proactive chain output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{
		"- repository:",
		"- issue:",
		"PROACTIVE_CHAIN_WORKFLOW_SECRET",
		"PROACTIVE_CHAIN_SOURCE_SECRET",
		"PROACTIVE_CHAIN_TARGET_SECRET",
		"Summarize the upstream secret result",
		"GITCLAW_PROACTIVE_PROMPT_FILE",
	} {
		if strings.Contains(output, notWant) {
			t.Fatalf("proactive chain output leaked %q:\n%s", notWant, output)
		}
	}
}

func TestHandleProactiveChainCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeProactiveChainFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 198,
			"title": "@gitclaw /proactive chain",
			"body": "Hidden proactive chain token: PROACTIVE_CHAIN_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{198: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic proactive chain command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Proactive Chain Report",
		"Generated without a model call",
		"model=\"gitclaw/proactive\"",
		"repository: `owner/repo`",
		"issue: `#198`",
		"requested_proactive_command: `chain`",
		"proactive_command_status: `ok`",
		"proactive_chain_status: `ok`",
		"chain_edges: `1`",
		"proactive_run_issue: `false`",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_proactive_chain_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("proactive chain handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"PROACTIVE_CHAIN_BODY_SECRET", "PROACTIVE_CHAIN_WORKFLOW_SECRET", "PROACTIVE_CHAIN_SOURCE_SECRET", "PROACTIVE_CHAIN_TARGET_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("proactive chain handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[198], "gitclaw:done") || hasLabel(github.IssueLabels[198], "gitclaw:running") || hasLabel(github.IssueLabels[198], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[198])
	}
}

func TestProactiveChainReportsMissingSourcesByHash(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/proactive/daily-summary.md", `<!-- gitclaw:proactive-context-from Missing Secret Source -->
Summarize the missing source.
PROACTIVE_CHAIN_MISSING_SOURCE_SECRET
`)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"proactive", "chain"}); err != nil {
			t.Fatalf("proactive chain returned error: %v", err)
		}
	})
	for _, want := range []string{
		"proactive_chain_status: `warning`",
		"prompt_files_with_context_from: `1`",
		"chain_edges: `0`",
		"missing_context_sources: `1`",
		"missing_context_source_hashes=",
		"context_from_gate=`warn`",
		"code=`proactive_context_source_missing`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("proactive chain missing-source output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"Missing Secret Source", "missing-secret-source", "PROACTIVE_CHAIN_MISSING_SOURCE_SECRET", "Summarize the missing source"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("proactive chain missing-source output leaked %q:\n%s", leaked, output)
		}
	}
}

func writeProactiveChainFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".github/workflows/gitclaw-proactive.yml", `name: GitClaw Proactive
on:
  workflow_dispatch:
  schedule:
    - cron: "23 8 * * 1"
jobs:
  enqueue:
    steps:
      - run: go run ./cmd/gitclaw proactive enqueue --prompt-file .gitclaw/proactive/daily-summary.md
# PROACTIVE_CHAIN_WORKFLOW_SECRET
`)
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", `<!-- gitclaw:proactive-skills repo-reader -->
Run the upstream secret repository hygiene job.
PROACTIVE_CHAIN_SOURCE_SECRET
`)
	writeTestFile(t, root, ".gitclaw/proactive/daily-summary.md", `<!-- gitclaw:proactive-skills repo-reader -->
<!-- gitclaw:proactive-context-from repo-hygiene -->
Summarize the upstream secret result.
PROACTIVE_CHAIN_TARGET_SECRET
`)
}
