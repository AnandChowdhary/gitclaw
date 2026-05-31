package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writePromptCompressionFixture(t *testing.T, dir string) {
	t.Helper()
	writePromptPackFixture(t, dir)
	writeTestFile(t, dir, "docs/search-fixture.md", "prompt compression unique search fixture phrase => GITCLAW_PROMPT_COMPRESSION_CONTEXT_V1\n")
}

func promptCompressionTranscript() []TranscriptMessage {
	return []TranscriptMessage{
		{Role: "user", Body: "Initial hidden prompt compression transcript token PROMPT_COMPRESSION_OLD_MESSAGE_SECRET.", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
		{Role: "assistant", Body: "Earlier compression note.", Actor: "github-actions[bot]", AuthorAssociation: "NONE", Trusted: true},
		{Role: "user", Body: strings.Repeat("PROMPT_COMPRESSION_LONG_MESSAGE_SECRET ", 20), Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 31, Trusted: true},
		{Role: "user", Body: "Use the repo-reader skill and search for `prompt compression unique search fixture phrase`.", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 32, Trusted: true},
	}
}

func TestRenderPromptCompressionReportAuditsReadinessWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writePromptCompressionFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	cfg.MaxPromptBytes = 900
	cfg.MaxTranscriptMessages = 3
	cfg.MaxTranscriptMessageBytes = 120
	transcript := promptCompressionTranscript()
	repoContext, err := LoadRepoContextWithConfig(dir, transcript, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{Repo: "owner/repo", Kind: EventIssueOpened, EventName: "issues", Issue: Issue{Number: 55, Title: "@gitclaw /prompt compression"}}

	report := RenderPromptCompressionReport(ev, cfg, transcript, repoContext)
	for _, want := range []string{
		"GitClaw Prompt Compression Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#55`",
		"prompt_compression_status: `warn`",
		"compression_strategy: `stateless-github-issue-bounded-prompt-audit`",
		"compression_model: `hermes-dual-thresholds+openclaw-session-pruning`",
		"max_prompt_bytes: `900`",
		"agent_compression_threshold_percent: `50`",
		"gateway_hygiene_threshold_percent: `85`",
		"agent_compression_recommended: `true`",
		"gateway_hygiene_recommended: `true`",
		"final_pack_truncation_active: `true`",
		"compression_engine_configured: `false`",
		"lossy_summary_supported: `false`",
		"lossless_session_search_supported: `true`",
		"pre_agent_gateway_hygiene_supported: `false`",
		"in_loop_context_compression_supported: `false`",
		"compression_writes_memory_allowed: `false`",
		"session_split_supported: `false`",
		"external_session_db_required: `false`",
		"issue_thread_canonical_storage: `true`",
		"backup_branch_replay_preferred: `true`",
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
		"llm_e2e_required_after_prompt_compression_change: `true`",
		"### Compression Segments",
		"kind=`system-prompt` name=`gitclaw-system-prompt` compression_region=`stable-system-prefix`",
		"kind=`context-file` name=`.gitclaw/SOUL.md` compression_region=`stable-context-prefix`",
		"kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md` compression_region=`stable-context-prefix`",
		"kind=`tool-output` name=`gitclaw.search_files` compression_region=`dynamic-tool-context`",
		"kind=`transcript-message` name=`user-",
		"body_included=`false`",
		"code=`hermes_dual_compression_thresholds_modeled`",
		"code=`openclaw_session_pruning_boundary_modeled`",
		"code=`lossy_compression_engine_disabled`",
		"code=`agent_compression_threshold_crossed`",
		"code=`gateway_hygiene_threshold_crossed`",
		"code=`final_prompt_pack_truncation_active`",
		"code=`older_transcript_messages_omitted`",
		"code=`transcript_message_bodies_truncated`",
		"code=`backup_branch_replay_preferred`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("prompt compression report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"PROMPT_PACK_SOUL_SECRET",
		"PROMPT_PACK_MEMORY_SECRET",
		"PROMPT_PACK_TOOLS_SECRET",
		"PROMPT_PACK_SKILL_SECRET",
		"PROMPT_COMPRESSION_OLD_MESSAGE_SECRET",
		"PROMPT_COMPRESSION_LONG_MESSAGE_SECRET",
		"GITCLAW_PROMPT_COMPRESSION_CONTEXT_V1",
		"prompt compression unique search fixture phrase",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("prompt compression report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestPromptCompressionCommandReportsLocalReadinessWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writePromptCompressionFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"prompt", "compression"}); err != nil {
			t.Fatalf("prompt compression returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Prompt Compression Report",
		"scope: `local-cli`",
		"compression_strategy: `stateless-github-issue-bounded-prompt-audit`",
		"compression_model: `hermes-dual-thresholds+openclaw-session-pruning`",
		"compression_engine_configured: `false`",
		"lossless_session_search_supported: `true`",
		"llm_e2e_required_after_prompt_compression_change: `true`",
		"### Compression Segments",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt compression output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"PROMPT_PACK_SOUL_SECRET", "PROMPT_PACK_MEMORY_SECRET", "PROMPT_PACK_TOOLS_SECRET", "PROMPT_PACK_SKILL_SECRET", "GITCLAW_PROMPT_COMPRESSION_CONTEXT_V1"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("prompt compression output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandlePromptCompressionCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writePromptCompressionFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 131,
			"title": "@gitclaw /prompt compression",
			"body": "Hidden prompt compression handler token: PROMPT_COMPRESSION_HANDLER_BODY_SECRET. Use the repo-reader skill and search for prompt compression unique search fixture phrase.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{131: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic prompt compression report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Prompt Compression Report",
		"Generated without a model call",
		"model=\"gitclaw/prompt\"",
		"repository: `owner/repo`",
		"issue: `#131`",
		"compression_strategy: `stateless-github-issue-bounded-prompt-audit`",
		"compression_engine_configured: `false`",
		"raw_issue_bodies_included: `false`",
		"llm_e2e_required_after_prompt_compression_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt compression handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROMPT_COMPRESSION_HANDLER_BODY_SECRET", "PROMPT_PACK_SOUL_SECRET", "PROMPT_PACK_SKILL_SECRET", "GITCLAW_PROMPT_COMPRESSION_CONTEXT_V1", "prompt compression unique search fixture phrase"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt compression handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[131], "gitclaw:done") || hasLabel(github.IssueLabels[131], "gitclaw:running") || hasLabel(github.IssueLabels[131], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[131])
	}
}
