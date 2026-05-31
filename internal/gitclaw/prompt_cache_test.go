package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writePromptCacheFixture(t *testing.T, dir string) {
	t.Helper()
	writePromptPackFixture(t, dir)
	writeTestFile(t, dir, ".github/workflows/gitclaw-heartbeat.yml", "name: GitClaw heartbeat\n")
	writeTestFile(t, dir, "docs/search-fixture.md", "prompt cache unique search fixture phrase => GITCLAW_PROMPT_CACHE_CONTEXT_V1\n")
}

func promptCacheTranscript() []TranscriptMessage {
	return []TranscriptMessage{
		{Role: "user", Body: "Initial hidden prompt cache transcript token PROMPT_CACHE_OLD_MESSAGE_SECRET.", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
		{Role: "assistant", Body: "Earlier cache note.", Actor: "github-actions[bot]", AuthorAssociation: "NONE", Trusted: true},
		{Role: "user", Body: strings.Repeat("PROMPT_CACHE_LONG_MESSAGE_SECRET ", 20), Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 21, Trusted: true},
		{Role: "user", Body: "Use the repo-reader skill and search for `prompt cache unique search fixture phrase`.", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 22, Trusted: true},
	}
}

func TestRenderPromptCacheReportAuditsReadinessWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writePromptCacheFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	cfg.MaxPromptBytes = 900
	cfg.MaxTranscriptMessages = 3
	cfg.MaxTranscriptMessageBytes = 120
	transcript := promptCacheTranscript()
	repoContext, err := LoadRepoContextWithConfig(dir, transcript, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{Repo: "owner/repo", Kind: EventIssueOpened, EventName: "issues", Issue: Issue{Number: 54, Title: "@gitclaw /prompt cache"}}

	report := RenderPromptCacheReport(ev, cfg, transcript, repoContext)
	for _, want := range []string{
		"GitClaw Prompt Cache Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#54`",
		"prompt_cache_status: `warn`",
		"cache_strategy: `same-issue-stable-prefix-audit`",
		"cache_model: `openclaw-cache-boundary+hermes-cache-compression`",
		"provider_cache_mode: `github-models-openai-compatible-observe-only`",
		"automatic_prefix_cache_possible: `true`",
		"cache_control_request_fields_forwarded: `false`",
		"prompt_cache_key_forwarded: `false`",
		"prompt_cache_retention_forwarded: `false`",
		"cache_usage_counters_available: `false`",
		"context_pruning_cache_ttl_configured: `false`",
		"heartbeat_keepwarm_workflow_present: `true`",
		"stable_user_prefix_bytes:",
		"stable_model_prefix_bytes:",
		"dynamic_suffix_bytes:",
		"cacheable_prefix_percent:",
		"boundary_component_kind: `tool-output`",
		"boundary_reason: `before_dynamic_tool_outputs`",
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
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credential_values_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_prompt_cache_change: `true`",
		"### Cache Segments",
		"kind=`system-prompt` name=`gitclaw-system-prompt` cache_region=`stable-prefix`",
		"kind=`context-file` name=`.gitclaw/SOUL.md` cache_region=`stable-prefix`",
		"kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md` cache_region=`stable-prefix`",
		"kind=`tool-output` name=`gitclaw.search_files` cache_region=`dynamic-suffix`",
		"kind=`transcript-message` name=`user-",
		"body_included=`false`",
		"code=`openclaw_prompt_cache_boundary_modeled`",
		"code=`hermes_cache_compression_interaction_modeled`",
		"code=`provider_prefix_cache_possible`",
		"code=`cache_request_controls_not_forwarded`",
		"code=`cache_usage_counters_unavailable`",
		"code=`dynamic_tool_outputs_limit_prefix_reuse`",
		"code=`heartbeat_keepwarm_workflow_present`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("prompt cache report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"PROMPT_PACK_SOUL_SECRET",
		"PROMPT_PACK_MEMORY_SECRET",
		"PROMPT_PACK_TOOLS_SECRET",
		"PROMPT_PACK_SKILL_SECRET",
		"PROMPT_CACHE_OLD_MESSAGE_SECRET",
		"PROMPT_CACHE_LONG_MESSAGE_SECRET",
		"GITCLAW_PROMPT_CACHE_CONTEXT_V1",
		"prompt cache unique search fixture phrase",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("prompt cache report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestPromptCacheCommandReportsLocalReadinessWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writePromptCacheFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"prompt", "cache"}); err != nil {
			t.Fatalf("prompt cache returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Prompt Cache Report",
		"scope: `local-cli`",
		"cache_strategy: `same-issue-stable-prefix-audit`",
		"provider_cache_mode: `github-models-openai-compatible-observe-only`",
		"heartbeat_keepwarm_workflow_present: `true`",
		"llm_e2e_required_after_prompt_cache_change: `true`",
		"### Cache Segments",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt cache output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"PROMPT_PACK_SOUL_SECRET", "PROMPT_PACK_MEMORY_SECRET", "PROMPT_PACK_TOOLS_SECRET", "PROMPT_PACK_SKILL_SECRET", "GITCLAW_PROMPT_CACHE_CONTEXT_V1"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("prompt cache output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandlePromptCacheCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writePromptCacheFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 130,
			"title": "@gitclaw /prompt cache",
			"body": "Hidden prompt cache handler token: PROMPT_CACHE_HANDLER_BODY_SECRET. Use the repo-reader skill and search for prompt cache unique search fixture phrase.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{130: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic prompt cache report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Prompt Cache Report",
		"Generated without a model call",
		"model=\"gitclaw/prompt\"",
		"repository: `owner/repo`",
		"issue: `#130`",
		"cache_strategy: `same-issue-stable-prefix-audit`",
		"cache_usage_counters_available: `false`",
		"raw_issue_bodies_included: `false`",
		"llm_e2e_required_after_prompt_cache_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt cache handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROMPT_CACHE_HANDLER_BODY_SECRET", "PROMPT_PACK_SOUL_SECRET", "PROMPT_PACK_SKILL_SECRET", "GITCLAW_PROMPT_CACHE_CONTEXT_V1", "prompt cache unique search fixture phrase"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt cache handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[130], "gitclaw:done") || hasLabel(github.IssueLabels[130], "gitclaw:running") || hasLabel(github.IssueLabels[130], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[130])
	}
}
