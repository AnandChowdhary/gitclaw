package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeMemoryProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_PROVENANCE_LONG_TERM_BODY_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "MEMORY_PROVENANCE_DATED_BODY_SECRET\n")
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "memory-provenance@example.invalid")
	runTestGit(t, root, "config", "user.name", "Memory Provenance Secret Author")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add memory provenance fixture MEMORY_COMMIT_SUBJECT_SECRET")
}

func TestMemoryProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeMemoryProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"memory", "provenance"}); err != nil {
			t.Fatalf("memory provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Memory Provenance Report",
		"scope: `local-cli`",
		"memory_provenance_status: `ok`",
		"provenance_scope: `repo-local-memory-git-history`",
		"memory_files: `2`",
		"long_term_memory_present: `true`",
		"long_term_memory_loaded: `true`",
		"dated_memory_notes: `1`",
		"canonical_dated_memory_notes: `1`",
		"noncanonical_dated_memory_notes: `0`",
		"loaded_memory_notes: `1`",
		"omitted_memory_notes: `0`",
		"first_memory_note: `.gitclaw/memory/2026-05-29.md`",
		"latest_memory_note: `.gitclaw/memory/2026-05-29.md`",
		"repo_local_memory_files: `2`",
		"unknown_memory_files: `0`",
		"git_tracked_memory_files: `2`",
		"untracked_memory_files: `0`",
		"working_tree_dirty_memory_files: `0`",
		"memory_files_with_commits: `2`",
		"memory_files_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"external_provider_accessed: `false`",
		"session_search_index_source: `github-issues-and-backups`",
		"background_promotion_active: `false`",
		"memory_writes_allowed: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_memory_provenance_change: `true`",
		"memory_validation_status: `ok`",
		"memory_risk_status: `ok`",
		"### Memory Provenance Cards",
		"position=`1` kind=`long-term` path=`.gitclaw/MEMORY.md` source=`repo-local` role=`stable-summary` date=`long-term`",
		"position=`2` kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md` source=`repo-local` role=`latest-daily-note` date=`2026-05-29`",
		"risk_codes=`none`",
		"validation_findings=`0`",
		"git_tracked=`true`",
		"working_tree_dirty=`false`",
		"commit_available=`true`",
		"last_commit_sha256_12=",
		"last_commit_short=",
		"last_commit_date=",
		"subject_sha256_12=",
		"### Provenance Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"git_history_gate=`pass`",
		"memory_write_gate=`disabled`",
		"external_provider_gate=`not_configured`",
		"session_search_gate=`github-issues-and-backups`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"MEMORY_PROVENANCE_LONG_TERM_BODY_SECRET",
		"MEMORY_PROVENANCE_DATED_BODY_SECRET",
		"MEMORY_COMMIT_SUBJECT_SECRET",
		"memory-provenance@example.invalid",
		"Memory Provenance Secret Author",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("memory provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderMemoryReportRoutesProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeMemoryProvenanceGitFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 172,
			"title": "@gitclaw /memory provenance",
			"body": "Hidden memory provenance body token: MEMORY_PROVENANCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	body := RenderMemoryReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Provenance Report",
		"repository: `owner/repo`",
		"issue: `#172`",
		"memory_provenance_status: `ok`",
		"issue_title_sha256_12:",
		"git_history_gate=`pass`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory provenance route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_PROVENANCE_ROUTE_BODY_SECRET", "MEMORY_PROVENANCE_LONG_TERM_BODY_SECRET", "MEMORY_PROVENANCE_DATED_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory provenance route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestHandleMemoryProvenanceCommandPostsGitHistoryReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeMemoryProvenanceGitFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 173,
			"title": "@gitclaw /memory provenance",
			"body": "Hidden memory provenance body token: MEMORY_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{173: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic memory provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Memory Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/memory\"",
		"repository: `owner/repo`",
		"issue: `#173`",
		"memory_provenance_status: `ok`",
		"provenance_scope: `repo-local-memory-git-history`",
		"memory_files: `2`",
		"git_tracked_memory_files: `2`",
		"memory_files_with_commits: `2`",
		"raw_memory_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_memory_provenance_change: `true`",
		"memory_validation_status: `ok`",
		"memory_risk_status: `ok`",
		"### Memory Provenance Cards",
		"kind=`long-term` path=`.gitclaw/MEMORY.md`",
		"kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md`",
		"git_tracked=`true`",
		"commit_available=`true`",
		"subject_sha256_12=",
		"### Provenance Gates",
		"git_history_gate=`pass`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_PROVENANCE_HANDLER_BODY_SECRET", "MEMORY_PROVENANCE_LONG_TERM_BODY_SECRET", "MEMORY_PROVENANCE_DATED_BODY_SECRET", "MEMORY_COMMIT_SUBJECT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory provenance report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[173], "gitclaw:done") || hasLabel(github.IssueLabels[173], "gitclaw:running") || hasLabel(github.IssueLabels[173], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[173])
	}
}
