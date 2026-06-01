package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderPromptContextReportShowsManifestWithoutBodies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTranscriptMessages = 2
	cfg.MaxTranscriptMessageBytes = 12
	ev := Event{
		Kind:      "issue_comment",
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 180, Title: "@gitclaw /prompt context", Body: "PROMPT_CONTEXT_ISSUE_SECRET"},
		Comment:   &Comment{ID: 77, Body: "@gitclaw /prompt context\nPROMPT_CONTEXT_COMMENT_SECRET", User: User{Login: "alice"}, AuthorAssociation: "MEMBER"},
	}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "older PROMPT_CONTEXT_OLDER_SECRET"},
		{Role: "assistant", Body: "assistant PROMPT_CONTEXT_ASSISTANT_SECRET"},
		{Role: "user", Body: "newer PROMPT_CONTEXT_NEWER_SECRET"},
	}
	repoContext := RepoContext{
		Documents: []ContextDocument{
			{Path: ".gitclaw/SOUL.md", Body: "PROMPT_CONTEXT_SOUL_SECRET\n"},
			{Path: ".gitclaw/MEMORY.md", Body: "PROMPT_CONTEXT_MEMORY_SECRET\n"},
		},
		Skills: []ContextDocument{
			{Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Body: "PROMPT_CONTEXT_SKILL_SECRET\n"},
		},
		SkillSummaries: []SkillSummary{{Name: "repo-reader", Enabled: true}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.search_files", Input: "PROMPT_CONTEXT_TOOL_INPUT_SECRET", Output: "PROMPT_CONTEXT_TOOL_OUTPUT_SECRET\n"},
			{Name: "gitclaw.list_files", Input: ".", Output: "README.md\n"},
		},
	}

	body := RenderPromptContextReport(ev, cfg, transcript, repoContext)
	for _, want := range []string{
		"GitClaw Prompt Context Manifest",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#180`",
		"event_kind: `issue_comment`",
		"event_name: `issue_comment`",
		"prompt_context_status: `ok`",
		"manifest_format: `gitclaw.prompt-context.v1`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"prompt_context_sha256_12: `",
		"system_prompt_sha256_12:",
		"context_files: `2`",
		"selected_skills: `1`",
		"available_skills: `1`",
		"tool_outputs: `2`",
		"prompt_visible_skills: `repo-reader`",
		"prompt_visible_tools: `gitclaw.list_files, gitclaw.search_files`",
		"tool_input_hashes: `2`",
		"transcript_messages: `3`",
		"bounded_transcript_messages: `2`",
		"omitted_older_messages: `1`",
		"truncated_transcript_bodies: `2`",
		"prompt_bodies_included: `false`",
		"context_file_bodies_included: `false`",
		"skill_bodies_included: `false`",
		"tool_output_bodies_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"credential_values_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_prompt_context_change: `true`",
		"### Context Cards",
		"kind=`context-file` name=`.gitclaw/SOUL.md`",
		"kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"kind=`tool-output` name=`gitclaw.search_files`",
		"input_sha256_12=`",
		"body_included=`false` input_included=`false`",
		"### Context Gates",
		"manifest_gate=`pass`",
		"skill_snapshot_gate=`pass`",
		"tool_snapshot_gate=`pass`",
		"raw_body_gate=`hashes-counts-and-paths-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt context report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"PROMPT_CONTEXT_ISSUE_SECRET",
		"PROMPT_CONTEXT_COMMENT_SECRET",
		"PROMPT_CONTEXT_OLDER_SECRET",
		"PROMPT_CONTEXT_ASSISTANT_SECRET",
		"PROMPT_CONTEXT_NEWER_SECRET",
		"PROMPT_CONTEXT_SOUL_SECRET",
		"PROMPT_CONTEXT_MEMORY_SECRET",
		"PROMPT_CONTEXT_SKILL_SECRET",
		"PROMPT_CONTEXT_TOOL_INPUT_SECRET",
		"PROMPT_CONTEXT_TOOL_OUTPUT_SECRET",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt context report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestPromptContextCommandReportsLocalManifest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITCLAW_WORKDIR", dir)
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "PROMPT_CONTEXT_CLI_SOUL_SECRET\n")
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "PROMPT_CONTEXT_CLI_MEMORY_SECRET\n")
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", "---\nname: repo-reader\ndescription: Read repository files.\n---\nPROMPT_CONTEXT_CLI_SKILL_SECRET\n")
	writeTestFile(t, dir, "README.md", "PROMPT_CONTEXT_CLI_README_SECRET\n")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"prompt", "context"}); err != nil {
			t.Fatalf("prompt context returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Prompt Context Manifest",
		"scope: `local-cli`",
		"prompt_context_status: `ok`",
		"manifest_format: `gitclaw.prompt-context.v1`",
		"prompt_context_sha256_12:",
		"context_files:",
		"selected_skills: `0`",
		"available_skills: `1`",
		"tool_outputs:",
		"prompt_visible_skills: `none`",
		"gitclaw.list_files",
		"gitclaw.skill_index",
		"prompt_bodies_included: `false`",
		"raw_tool_inputs_included: `false`",
		"llm_e2e_required_after_prompt_context_change: `true`",
		"manifest_gate=`pass`",
		"skill_snapshot_gate=`not-selected`",
		"tool_snapshot_gate=`pass`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt context CLI output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"PROMPT_CONTEXT_CLI_SOUL_SECRET", "PROMPT_CONTEXT_CLI_MEMORY_SECRET", "PROMPT_CONTEXT_CLI_SKILL_SECRET", "PROMPT_CONTEXT_CLI_README_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("prompt context CLI output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandlePromptContextCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITCLAW_WORKDIR", dir)
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "PROMPT_CONTEXT_HANDLER_SOUL_SECRET\n")
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", "---\nname: repo-reader\ndescription: Read repository files.\n---\nPROMPT_CONTEXT_HANDLER_SKILL_SECRET\n")
	writeTestFile(t, dir, "docs/search-fixture.md", "handler prompt context fixture\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 181,
			"title": "@gitclaw /prompt context",
			"body": "@gitclaw /prompt context\nUse the repo-reader skill and search for handler prompt context fixture.\nHidden: PROMPT_CONTEXT_HANDLER_ISSUE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{181: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = dir
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic prompt context command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Prompt Context Manifest",
		"Generated without a model call",
		"model=\"gitclaw/prompt\"",
		"prompt_context_status: `ok`",
		"manifest_format: `gitclaw.prompt-context.v1`",
		"repository: `owner/repo`",
		"issue: `#181`",
		"selected_skills: `1`",
		"prompt_visible_skills: `repo-reader`",
		"prompt_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"llm_e2e_required_after_prompt_context_change: `true`",
		"manifest_gate=`pass`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt context handler output missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROMPT_CONTEXT_HANDLER_ISSUE_SECRET", "PROMPT_CONTEXT_HANDLER_SOUL_SECRET", "PROMPT_CONTEXT_HANDLER_SKILL_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt context handler output leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[181], "gitclaw:done") || hasLabel(github.IssueLabels[181], "gitclaw:running") || hasLabel(github.IssueLabels[181], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[181])
	}
}
