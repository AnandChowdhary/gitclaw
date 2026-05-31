package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeSkillProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files for provenance tests.
always: true
metadata:
  openclaw:
    requires:
      env:
        - GITCLAW_SKILL_PROVENANCE_ENV
      bins: [git]
---

# Repo Reader
SKILL_PROVENANCE_BODY_SECRET
`)
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "test@example.com")
	runTestGit(t, root, "config", "user.name", "Test User")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add skill provenance fixture SKILL_COMMIT_SUBJECT_SECRET")
}

func TestSkillsProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSkillProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITCLAW_SKILL_PROVENANCE_ENV", "present-secret-value")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "provenance"}); err != nil {
			t.Fatalf("skills provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Skill Provenance Report",
		"scope: `local-cli`",
		"skill_provenance_status: `ok`",
		"provenance_scope: `repo-local-skill-git-history`",
		"available_skills: `1`",
		"enabled_skills: `1`",
		"disabled_skills: `0`",
		"allowlist_blocked_skills: `0`",
		"selected_skills: `1`",
		"repo_local_skills: `1`",
		"compat_root_skills: `0`",
		"unknown_source_skills: `0`",
		"git_tracked_skills: `1`",
		"untracked_skills: `0`",
		"working_tree_dirty_skills: `0`",
		"skills_with_commits: `1`",
		"skills_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"installer_scripts_run: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_skill_provenance_change: `true`",
		"skill_validation_status: `ok`",
		"skill_risk_status: `ok`",
		"### Skill Provenance Cards",
		"name=`repo-reader` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"folder=`repo-reader`",
		"source=`repo-local`",
		"enabled=`true`",
		"selected_for_this_turn=`true`",
		"frontmatter=`true`",
		"description=`true`",
		"requires_env=`1`",
		"requires_bins=`1`",
		"missing_env=`0`",
		"missing_bins=`0`",
		"risk_findings=`0`",
		"risk_codes=`none`",
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
		"mutation_gate=`disabled`",
		"installer_gate=`disabled`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"SKILL_PROVENANCE_BODY_SECRET",
		"SKILL_COMMIT_SUBJECT_SECRET",
		"Read repository files for provenance tests",
		"GITCLAW_SKILL_PROVENANCE_ENV",
		"present-secret-value",
		"test@example.com",
		"Test User",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("skills provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderSkillsReportRoutesProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSkillProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_SKILL_PROVENANCE_ENV", "present-secret-value")
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /skills provenance"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 156,
			"title": "@gitclaw /skills provenance",
			"body": "Hidden skill provenance body token: SKILL_PROVENANCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderSkillsReport(ev, cfg, repoContext)
	for _, want := range []string{
		"GitClaw Skill Provenance Report",
		"repository: `owner/repo`",
		"issue: `#156`",
		"skill_provenance_status: `ok`",
		"available_skills: `1`",
		"git_tracked_skills: `1`",
		"skills_with_commits: `1`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skills provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SKILL_PROVENANCE_ROUTE_BODY_SECRET", "SKILL_PROVENANCE_BODY_SECRET", "SKILL_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skills provenance routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestHandleSkillsProvenanceCommandPostsGitHistoryReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeSkillProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_SKILL_PROVENANCE_ENV", "present-secret-value")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 157,
			"title": "@gitclaw /skills provenance",
			"body": "Hidden skill provenance body token: SKILL_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{157: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic skills provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Skill Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/skills\"",
		"repository: `owner/repo`",
		"issue: `#157`",
		"skill_provenance_status: `ok`",
		"available_skills: `1`",
		"repo_local_skills: `1`",
		"git_tracked_skills: `1`",
		"working_tree_dirty_skills: `0`",
		"skills_with_commits: `1`",
		"raw_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_skill_provenance_change: `true`",
		"name=`repo-reader` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"git_history_gate=`pass`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("skills provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SKILL_PROVENANCE_HANDLER_BODY_SECRET", "SKILL_PROVENANCE_BODY_SECRET", "SKILL_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("skills provenance report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[157], "gitclaw:done") || hasLabel(github.IssueLabels[157], "gitclaw:running") || hasLabel(github.IssueLabels[157], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[157])
	}
}
