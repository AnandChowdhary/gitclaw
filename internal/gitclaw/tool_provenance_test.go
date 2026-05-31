package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeToolProvenanceFixture(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "Use deterministic read-only GitClaw tools as bounded prompt context.\n")
	writeTestFile(t, dir, ".gitclaw/skills/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files for tests.
---
Use bounded repository context only.
`)
	writeTestFile(t, dir, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	writeTestFile(t, dir, "docs/search.md", "bounded repository search fixture phrase -> GITCLAW_SEARCH_CONTEXT_V1\n")
}

func TestRenderToolProvenanceReportShowsPromptVisibleOutputsWithoutBodies(t *testing.T) {
	repoContext := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/TOOLS.md", Body: "TOOL_PROVENANCE_GUIDANCE_SECRET"}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: ". TOOL_PROVENANCE_LIST_INPUT_SECRET", Output: "go.mod\nREADME.md\nTOOL_PROVENANCE_LIST_OUTPUT_SECRET"},
			{Name: "gitclaw.read_file", Input: "go.mod TOOL_PROVENANCE_READ_INPUT_SECRET", Output: "module github.com/AnandChowdhary/gitclaw\nTOOL_PROVENANCE_READ_OUTPUT_SECRET"},
			{Name: "gitclaw.search_files", Input: "bounded phrase TOOL_PROVENANCE_SEARCH_INPUT_SECRET", Output: "docs/search.md:1:GITCLAW_SEARCH_CONTEXT_V1 TOOL_PROVENANCE_SEARCH_OUTPUT_SECRET"},
			{Name: "gitclaw.skill_index", Input: ".gitclaw/SKILLS TOOL_PROVENANCE_SKILL_INPUT_SECRET", Output: "repo-reader sha256_12=abcdef123456 TOOL_PROVENANCE_SKILL_OUTPUT_SECRET"},
		},
	}
	body := RenderToolProvenanceCLIReport(repoContext)
	for _, want := range []string{
		"GitClaw Tool Provenance Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"tool_provenance_status: `ok`",
		"provenance_scope: `pre_model_prompt_context`",
		"tool_context_strategy: `deterministic-pre-model-outputs`",
		"available_tools: `5`",
		"enabled_tools: `5`",
		"disabled_tools: `0`",
		"allowlist_blocked_tools: `0`",
		"active_tool_outputs: `4`",
		"known_tool_outputs: `4`",
		"unknown_tool_outputs: `0`",
		"prompt_visible_tool_outputs: `4`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.read_file, gitclaw.search_files, gitclaw.skill_index`",
		"read_only_outputs: `3`",
		"metadata_only_outputs: `1`",
		"tool_inputs_hashed: `4`",
		"tool_outputs_hashed: `4`",
		"registry_verification: `not_configured`",
		"runtime_permission_verification: `static_contracts_only`",
		"model_callable_structured_tools: `false`",
		"shell_execution_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_inputs_included: `false`",
		"raw_outputs_included: `false`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"llm_e2e_required_after_tool_provenance_change: `true`",
		"tool_validation_status: `ok`",
		"tool_risk_status: `ok`",
		"### Prompt-Visible Tool Outputs",
		"name=`gitclaw.list_files` contract_known=`true` mode=`read-only`",
		"name=`gitclaw.read_file` contract_known=`true` mode=`read-only`",
		"name=`gitclaw.search_files` contract_known=`true` mode=`read-only`",
		"name=`gitclaw.skill_index` contract_known=`true` mode=`metadata-only`",
		"input_sha256_12=",
		"output_sha256_12=",
		"risk_codes=`none`",
		"line_hashes=`none`",
		"### Provenance Gates",
		"raw_input_gate=`hash_only`",
		"raw_output_gate=`hash_only`",
		"mutation_gate=`disabled`",
		"shell_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tool provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"TOOL_PROVENANCE_GUIDANCE_SECRET",
		"TOOL_PROVENANCE_LIST_INPUT_SECRET",
		"TOOL_PROVENANCE_LIST_OUTPUT_SECRET",
		"TOOL_PROVENANCE_READ_INPUT_SECRET",
		"TOOL_PROVENANCE_READ_OUTPUT_SECRET",
		"TOOL_PROVENANCE_SEARCH_INPUT_SECRET",
		"TOOL_PROVENANCE_SEARCH_OUTPUT_SECRET",
		"TOOL_PROVENANCE_SKILL_INPUT_SECRET",
		"TOOL_PROVENANCE_SKILL_OUTPUT_SECRET",
		"GITCLAW_SEARCH_CONTEXT_V1",
		"bounded phrase",
		"module github.com/AnandChowdhary/gitclaw",
		"go.mod TOOL_PROVENANCE_READ_INPUT_SECRET",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tool provenance report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestToolsProvenanceCommandReportsHashesWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeToolProvenanceFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "provenance", "go.mod", "\"bounded repository search fixture phrase\""}); err != nil {
			t.Fatalf("tools provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Tool Provenance Report",
		"scope: `local-cli`",
		"tool_provenance_status: `ok`",
		"active_tool_outputs: `4`",
		"prompt_visible_tool_outputs: `4`",
		"read_only_outputs: `3`",
		"metadata_only_outputs: `1`",
		"name=`gitclaw.read_file`",
		"name=`gitclaw.search_files`",
		"name=`gitclaw.skill_index`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"module github.com/AnandChowdhary/gitclaw", "bounded repository search fixture phrase", "GITCLAW_SEARCH_CONTEXT_V1", "Use bounded repository context only"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("tools provenance CLI leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderToolsReportRoutesProvenanceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeToolProvenanceFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 127,
			"title": "@gitclaw /tools provenance",
			"body": "Inspect `+"`go.mod`"+` and search for `+"`bounded repository search fixture phrase`"+`. Hidden token: TOOL_PROVENANCE_ROUTE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	repoContext, err := LoadRepoContextWithConfig(dir, []TranscriptMessage{{Role: "user", Body: ev.Issue.Body}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderToolsReport(ev, cfg, repoContext)
	for _, want := range []string{
		"GitClaw Tool Provenance Report",
		"repository: `owner/repo`",
		"issue: `#127`",
		"tool_provenance_status: `ok`",
		"active_tool_outputs: `4`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("tools provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"TOOL_PROVENANCE_ROUTE_SECRET", "module github.com/AnandChowdhary/gitclaw", "GITCLAW_SEARCH_CONTEXT_V1", "bounded repository search fixture phrase"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("tools provenance routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestHandleToolsProvenanceCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeToolProvenanceFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 128,
			"title": "@gitclaw /tools provenance",
			"body": "Inspect `+"`go.mod`"+` and search for `+"`bounded repository search fixture phrase`"+`. Hidden token: TOOL_PROVENANCE_HANDLER_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{128: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic tool provenance report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Tool Provenance Report", "Generated without a model call", "model=\"gitclaw/tools\"", "tool_provenance_status: `ok`", "prompt_visible_tool_outputs: `4`", "llm_e2e_required_after_tool_provenance_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools provenance handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"TOOL_PROVENANCE_HANDLER_SECRET", "module github.com/AnandChowdhary/gitclaw", "GITCLAW_SEARCH_CONTEXT_V1", "bounded repository search fixture phrase"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("tools provenance handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[128], "gitclaw:done") || hasLabel(github.IssueLabels[128], "gitclaw:running") || hasLabel(github.IssueLabels[128], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[128])
	}
}
