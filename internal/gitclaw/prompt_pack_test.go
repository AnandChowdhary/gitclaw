package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writePromptPackFixture(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, dir, ".gitclaw/config.yml", `model:
  max_prompt_bytes: 900
  max_transcript_messages: 3
  max_transcript_message_bytes: 120
`)
	writeTestFile(t, dir, ".gitclaw/SOUL.md", strings.Repeat("PROMPT_PACK_SOUL_SECRET ", 14))
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", strings.Repeat("PROMPT_PACK_MEMORY_SECRET ", 14))
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "PROMPT_PACK_TOOLS_SECRET: keep diagnostics body-free.\n")
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
always: true
---

PROMPT_PACK_SKILL_SECRET
`)
	writeTestFile(t, dir, "docs/search-fixture.md", "prompt pack unique search fixture phrase => GITCLAW_PROMPT_PACK_CONTEXT_V1\n")
}

func promptPackTranscript() []TranscriptMessage {
	return []TranscriptMessage{
		{Role: "user", Body: "Initial hidden prompt pack transcript token PROMPT_PACK_OLD_MESSAGE_SECRET.", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
		{Role: "assistant", Body: "Earlier assistant note.", Actor: "github-actions[bot]", AuthorAssociation: "NONE", Trusted: true},
		{Role: "user", Body: strings.Repeat("PROMPT_PACK_LONG_MESSAGE_SECRET ", 20), Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 11, Trusted: true},
		{Role: "user", Body: "Search for `prompt pack unique search fixture phrase`.", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 12, Trusted: true},
	}
}

func TestRenderPromptPackReportExplainsBudgetProjectionWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writePromptPackFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	cfg.MaxPromptBytes = 900
	cfg.MaxTranscriptMessages = 3
	cfg.MaxTranscriptMessageBytes = 120
	transcript := promptPackTranscript()
	repoContext, err := LoadRepoContextWithConfig(dir, transcript, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{Repo: "owner/repo", Kind: EventIssueOpened, EventName: "issues", Issue: Issue{Number: 44, Title: "@gitclaw /prompt pack"}}

	report := RenderPromptPackReport(ev, cfg, transcript, repoContext)
	for _, want := range []string{
		"GitClaw Prompt Pack Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#44`",
		"prompt_pack_status: `warn`",
		"pack_strategy: `fixed-order-head-tail-budgeted`",
		"compression_model: `openclaw-budget-snapshot+hermes-50-85-thresholds`",
		"max_prompt_bytes: `900`",
		"agent_compression_threshold_percent: `50`",
		"agent_compression_threshold_bytes: `450`",
		"gateway_hygiene_threshold_percent: `85`",
		"gateway_hygiene_threshold_bytes: `765`",
		"prompt_contains_truncation_marker: `true`",
		"pack_truncation_marker_bytes:",
		"pack_components:",
		"partial_components:",
		"omitted_components:",
		"context_files:",
		"selected_skills: `1`",
		"tool_outputs:",
		"transcript_messages: `4`",
		"bounded_transcript_messages: `3`",
		"omitted_older_messages: `1`",
		"truncated_transcript_bodies: `1`",
		"prompt_body_included: `false`",
		"context_file_bodies_included: `false`",
		"skill_bodies_included: `false`",
		"tool_output_bodies_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credential_values_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_prompt_pack_change: `true`",
		"### Pack Components",
		"kind=`run-header` name=`repository-and-issue`",
		"kind=`context-file` name=`.gitclaw/SOUL.md`",
		"kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"kind=`tool-output` name=`gitclaw.search_files`",
		"metadata=`input_sha256_12:",
		"kind=`transcript-message` name=`user-",
		"body_included=`false` input_included=`false`",
		"code=`prompt_pack_order_static`",
		"code=`openclaw_context_budget_snapshot`",
		"code=`hermes_compression_thresholds_evaluated`",
		"code=`prompt_pack_requires_final_truncation`",
		"code=`prompt_over_agent_compression_threshold`",
		"code=`prompt_over_gateway_hygiene_threshold`",
		"code=`older_transcript_messages_omitted`",
		"code=`transcript_message_bodies_truncated`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("prompt pack report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"PROMPT_PACK_SOUL_SECRET",
		"PROMPT_PACK_MEMORY_SECRET",
		"PROMPT_PACK_TOOLS_SECRET",
		"PROMPT_PACK_SKILL_SECRET",
		"PROMPT_PACK_OLD_MESSAGE_SECRET",
		"PROMPT_PACK_LONG_MESSAGE_SECRET",
		"GITCLAW_PROMPT_PACK_CONTEXT_V1",
		"prompt pack unique search fixture phrase",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("prompt pack report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestPromptPackCommandReportsLocalPackingWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writePromptPackFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"prompt", "pack"}); err != nil {
			t.Fatalf("prompt pack returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Prompt Pack Report",
		"scope: `local-cli`",
		"pack_strategy: `fixed-order-head-tail-budgeted`",
		"max_prompt_bytes: `900`",
		"agent_compression_threshold_percent: `50`",
		"gateway_hygiene_threshold_percent: `85`",
		"llm_e2e_required_after_prompt_pack_change: `true`",
		"### Pack Components",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt pack output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"PROMPT_PACK_SOUL_SECRET", "PROMPT_PACK_MEMORY_SECRET", "PROMPT_PACK_TOOLS_SECRET", "PROMPT_PACK_SKILL_SECRET", "GITCLAW_PROMPT_PACK_CONTEXT_V1"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("prompt pack output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandlePromptPackCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writePromptPackFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 129,
			"title": "@gitclaw /prompt pack",
			"body": "Hidden prompt pack handler token: PROMPT_PACK_HANDLER_BODY_SECRET. Search for prompt pack unique search fixture phrase.",
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
	cfg.MaxPromptBytes = 900
	cfg.MaxTranscriptMessages = 3
	cfg.MaxTranscriptMessageBytes = 120
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{129: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic prompt pack report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Prompt Pack Report",
		"Generated without a model call",
		"model=\"gitclaw/prompt\"",
		"repository: `owner/repo`",
		"issue: `#129`",
		"pack_strategy: `fixed-order-head-tail-budgeted`",
		"raw_issue_bodies_included: `false`",
		"llm_e2e_required_after_prompt_pack_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt pack handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROMPT_PACK_HANDLER_BODY_SECRET", "PROMPT_PACK_SOUL_SECRET", "PROMPT_PACK_SKILL_SECRET", "GITCLAW_PROMPT_PACK_CONTEXT_V1", "prompt pack unique search fixture phrase"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt pack handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[129], "gitclaw:done") || hasLabel(github.IssueLabels[129], "gitclaw:running") || hasLabel(github.IssueLabels[129], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[129])
	}
}
