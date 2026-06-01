package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeProfileSearchFixture(t *testing.T, root string) {
	t.Helper()
	writeProfileSnapshotFixture(t, root)
	writeTestFile(t, root, ".gitclaw/SOUL.md", "GitClaw profile search fixture marker profile-search-marker with PROFILE_SEARCH_SOUL_BODY_SECRET.\n")
}

func TestRenderProfileSearchReportFindsProfileWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileSearchFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile search."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	body := RenderProfileSearchCLIReport(cfg, ctx, "profile-search-marker PROFILE_SEARCH_QUERY_SECRET")
	for _, want := range []string{
		"GitClaw Profile Search Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"profile_search_status: `ok`",
		"search_scope: `repo-local-profile-files`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `10`",
		"profile_documents_loaded:",
		"manifest_entries:",
		"files_scanned:",
		"matched_files:",
		"matched_lines:",
		"results_returned:",
		"raw_bodies_included: `false`",
		"raw_profile_bodies_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_queries_included: `false`",
		"profile_mutation_allowed: `false`",
		"llm_e2e_required_after_profile_search_change: `true`",
		"### Results",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md`",
		"line=`1`",
		"score=`",
		"matched_terms=`",
		"file_sha256_12=",
		"line_sha256_12=",
		"### Search Gates",
		"query_gate=`sha256_12_only`",
		"raw_body_gate=`hashes-and-line-hashes-only`",
		"mutation_gate=`disabled`",
		"llm_e2e_gate=`required`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"PROFILE_SEARCH_SOUL_BODY_SECRET",
		"PROFILE_SEARCH_QUERY_SECRET",
		"profile-search-marker",
		"GitClaw profile search fixture marker",
		"SKILL_SNAPSHOT_BODY_SECRET",
		"SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET",
		"SKILL_SNAPSHOT_SOURCE_REF_SECRET",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile search report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderProfileReportRoutesSearchWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileSearchFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile search."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 194,
			Title:  "@gitclaw /profile search profile-search-marker PROFILE_SEARCH_ROUTE_QUERY_SECRET",
			Body:   "Hidden profile search issue token: PROFILE_SEARCH_ROUTE_BODY_SECRET.",
		},
	}
	body := RenderProfileReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Search Report",
		"repository: `owner/repo`",
		"issue: `#194`",
		"profile_search_status: `ok`",
		"search_scope: `repo-local-profile-files`",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_profile_search_change: `true`",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md`",
		"query_gate=`sha256_12_only`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile search route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROFILE_SEARCH_ROUTE_BODY_SECRET", "PROFILE_SEARCH_ROUTE_QUERY_SECRET", "PROFILE_SEARCH_SOUL_BODY_SECRET", "profile-search-marker"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile search route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestProfileSearchCommandReportsMatchesWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileSearchFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"profile", "search", "profile-search-marker", "PROFILE_SEARCH_CLI_QUERY_SECRET"}); err != nil {
			t.Fatalf("profile search returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Profile Search Report",
		"scope: `local-cli`",
		"profile_search_status: `ok`",
		"search_scope: `repo-local-profile-files`",
		"query_sha256_12:",
		"raw_profile_bodies_included: `false`",
		"raw_queries_included: `false`",
		"llm_e2e_required_after_profile_search_change: `true`",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md`",
		"line_sha256_12=",
		"### Search Gates",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("profile search output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"PROFILE_SEARCH_CLI_QUERY_SECRET", "PROFILE_SEARCH_SOUL_BODY_SECRET", "profile-search-marker", "SKILL_SNAPSHOT_BODY_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("profile search output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleProfileSearchCommandPostsSearchWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeProfileSearchFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 195,
			"title": "@gitclaw /profile search profile-search-marker PROFILE_SEARCH_HANDLER_QUERY_SECRET",
			"body": "@gitclaw /profile search profile-search-marker PROFILE_SEARCH_HANDLER_QUERY_SECRET\nHidden profile search body token: PROFILE_SEARCH_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{195: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic profile search command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Profile Search Report",
		"Generated without a model call",
		"model=\"gitclaw/profile\"",
		"repository: `owner/repo`",
		"issue: `#195`",
		"profile_search_status: `ok`",
		"search_scope: `repo-local-profile-files`",
		"raw_issue_bodies_included: `false`",
		"raw_queries_included: `false`",
		"llm_e2e_required_after_profile_search_change: `true`",
		"issue_title_sha256_12:",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md`",
		"### Search Gates",
		"query_gate=`sha256_12_only`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile search handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROFILE_SEARCH_HANDLER_BODY_SECRET", "PROFILE_SEARCH_HANDLER_QUERY_SECRET", "PROFILE_SEARCH_SOUL_BODY_SECRET", "profile-search-marker", "SKILL_SNAPSHOT_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile search handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[195], "gitclaw:done") || hasLabel(github.IssueLabels[195], "gitclaw:running") || hasLabel(github.IssueLabels[195], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[195])
	}
}
