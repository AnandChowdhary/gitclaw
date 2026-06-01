package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeProfileDiffFixture(t *testing.T, root string) {
	t.Helper()
	writeProfileSnapshotFixture(t, root)
	writeTestFile(t, root, ".gitclaw/config.yml", "trigger:\n  label: gitclaw\n")
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "profile-diff@example.invalid")
	runTestGit(t, root, "config", "user.name", "Profile Diff Test")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add profile diff base PROFILE_DIFF_BASE_COMMIT_SECRET")

	writeTestFile(t, root, ".gitclaw/SOUL.md", "Changed soul body for profile diff with PROFILE_DIFF_CHANGED_SOUL_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/proactive/profile-diff.md", "Profile diff proactive prompt PROFILE_DIFF_PROACTIVE_SECRET.\n")
	runTestGit(t, root, "rm", ".gitclaw/HEARTBEAT.md")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Change profile diff fixture PROFILE_DIFF_HEAD_COMMIT_SECRET")
}

func TestRenderProfileDiffReportComparesProfileRefsWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileDiffFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile diff."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	body := RenderProfileDiffCLIReport(cfg, ctx, "HEAD~1")
	for _, want := range []string{
		"GitClaw Profile Diff Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"profile_diff_status: `ok`",
		"diff_scope: `repo-local-profile-files`",
		"base_ref_sha256_12:",
		"base_ref_resolved: `true`",
		"base_commit_sha256_12:",
		"head_commit_sha256_12:",
		"profile_diff_sha256_12:",
		"current_manifest_entries:",
		"current_profile_surfaces:",
		"changed_profile_files: `3`",
		"added_profile_files: `1`",
		"modified_profile_files: `1`",
		"deleted_profile_files: `1`",
		"renamed_profile_files: `0`",
		"binary_profile_files: `0`",
		"profile_file_limit: `50`",
		"profile_files_returned: `3`",
		"git_available: `true`",
		"git_repository: `true`",
		"raw_diffs_included: `false`",
		"raw_profile_bodies_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"raw_requested_refs_included: `false`",
		"profile_mutation_allowed: `false`",
		"llm_e2e_required_after_profile_diff_change: `true`",
		"### Profile Diff Cards",
		"path=`.gitclaw/HEARTBEAT.md` status=`deleted`",
		"category=`heartbeat`",
		"path=`.gitclaw/SOUL.md` status=`modified` kind=`profile-document` category=`soul`",
		"path=`.gitclaw/proactive/profile-diff.md` status=`added` kind=`proactive-prompt` category=`proactive`",
		"insertions=`",
		"deletions=`",
		"path_sha256_12=",
		"current_file_sha256_12=",
		"### Diff Gates",
		"base_ref_gate=`pass`",
		"raw_diff_gate=`numstat-and-status-only`",
		"raw_body_gate=`hashes-only`",
		"requested_ref_gate=`sha256_12_only`",
		"git_subject_gate=`excluded`",
		"mutation_gate=`disabled`",
		"llm_e2e_gate=`required`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile diff report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"HEAD~1",
		"PROFILE_DIFF_BASE_COMMIT_SECRET",
		"PROFILE_DIFF_HEAD_COMMIT_SECRET",
		"PROFILE_DIFF_CHANGED_SOUL_SECRET",
		"PROFILE_DIFF_PROACTIVE_SECRET",
		"profile-diff@example.invalid",
		"Profile Diff Test",
		"Changed soul body for profile diff",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile diff report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderProfileReportRoutesDiffWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileDiffFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile diff."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 196,
			Title:  "Profile diff route",
			Body:   "@gitclaw /profile diff HEAD~1 PROFILE_DIFF_ROUTE_REF_SECRET\nHidden profile diff issue token: PROFILE_DIFF_ROUTE_BODY_SECRET.",
		},
	}
	body := RenderProfileReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Diff Report",
		"repository: `owner/repo`",
		"issue: `#196`",
		"profile_diff_status: `ok`",
		"base_ref_sha256_12:",
		"base_ref_resolved: `true`",
		"changed_profile_files: `3`",
		"raw_requested_refs_included: `false`",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_profile_diff_change: `true`",
		"base_ref_gate=`pass`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile diff route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"HEAD~1", "PROFILE_DIFF_ROUTE_REF_SECRET", "PROFILE_DIFF_ROUTE_BODY_SECRET", "PROFILE_DIFF_CHANGED_SOUL_SECRET", "PROFILE_DIFF_HEAD_COMMIT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile diff route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestProfileDiffCommandReportsGitDiffWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileDiffFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"profile", "diff", "HEAD~1"}); err != nil {
			t.Fatalf("profile diff returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Profile Diff Report",
		"scope: `local-cli`",
		"profile_diff_status: `ok`",
		"changed_profile_files: `3`",
		"path=`.gitclaw/SOUL.md` status=`modified`",
		"path=`.gitclaw/proactive/profile-diff.md` status=`added`",
		"raw_diffs_included: `false`",
		"raw_requested_refs_included: `false`",
		"llm_e2e_required_after_profile_diff_change: `true`",
		"### Diff Gates",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("profile diff output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"HEAD~1", "PROFILE_DIFF_CHANGED_SOUL_SECRET", "PROFILE_DIFF_PROACTIVE_SECRET", "PROFILE_DIFF_HEAD_COMMIT_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("profile diff output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleProfileDiffCommandPostsDiffWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeProfileDiffFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 197,
			"title": "Profile diff handler",
			"body": "@gitclaw /profile diff HEAD~1 PROFILE_DIFF_HANDLER_REF_SECRET\nHidden profile diff body token: PROFILE_DIFF_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{197: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic profile diff command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Profile Diff Report",
		"Generated without a model call",
		"model=\"gitclaw/profile\"",
		"repository: `owner/repo`",
		"issue: `#197`",
		"profile_diff_status: `ok`",
		"changed_profile_files: `3`",
		"raw_diffs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_requested_refs_included: `false`",
		"llm_e2e_required_after_profile_diff_change: `true`",
		"path=`.gitclaw/SOUL.md` status=`modified`",
		"### Diff Gates",
		"requested_ref_gate=`sha256_12_only`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile diff handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"HEAD~1", "PROFILE_DIFF_HANDLER_REF_SECRET", "PROFILE_DIFF_HANDLER_BODY_SECRET", "PROFILE_DIFF_CHANGED_SOUL_SECRET", "PROFILE_DIFF_HEAD_COMMIT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile diff handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[197], "gitclaw:done") || hasLabel(github.IssueLabels[197], "gitclaw:running") || hasLabel(github.IssueLabels[197], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[197])
	}
}
