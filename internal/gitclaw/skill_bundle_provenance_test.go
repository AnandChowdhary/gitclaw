package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeSkillBundleProvenanceGitFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files.
---

# Repo Reader
SKILL_BUNDLE_PROVENANCE_SKILL_BODY_SECRET
`)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", `name: repo-context
description: BUNDLE_PROVENANCE_DESCRIPTION_SECRET
skills:
  - repo-reader
instruction: |
  BUNDLE_PROVENANCE_INSTRUCTION_SECRET
  Prefer deterministic repository search.
`)
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "test@example.com")
	runTestGit(t, root, "config", "user.name", "Test User")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add bundle provenance fixture BUNDLE_COMMIT_SUBJECT_SECRET")
}

func TestBundlesProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSkillBundleProvenanceGitFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"bundles", "provenance"}); err != nil {
			t.Fatalf("bundles provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Skill Bundle Provenance Report",
		"scope: `local-cli`",
		"bundle_provenance_status: `ok`",
		"provenance_scope: `repo-local-skill-bundle-git-history`",
		"available_bundles: `1`",
		"selected_bundles: `0`",
		"bundle_skill_refs: `1`",
		"resolved_bundle_skills: `1`",
		"missing_bundle_skills: `0`",
		"bundles_with_instruction: `1`",
		"git_tracked_bundles: `1`",
		"untracked_bundles: `0`",
		"working_tree_dirty_bundles: `0`",
		"bundles_with_commits: `1`",
		"bundles_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"repository_mutation_allowed: `false`",
		"agent_authored_bundle_mutation_supported: `false`",
		"raw_bundle_bodies_included: `false`",
		"raw_bundle_instructions_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_bundle_provenance_change: `true`",
		"### Bundle Provenance Cards",
		"bundle_name=`repo-context` path=`.gitclaw/skill-bundles/repo-context.yaml`",
		"skills=`repo-reader`",
		"resolved_skills=`repo-reader`",
		"missing_skills=`none`",
		"instruction=`true`",
		"instruction_sha256_12=",
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
		"git_history_gate=`pass`",
		"mutation_gate=`disabled`",
		"agent_authored_mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("bundle provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"SKILL_BUNDLE_PROVENANCE_SKILL_BODY_SECRET",
		"BUNDLE_PROVENANCE_DESCRIPTION_SECRET",
		"BUNDLE_PROVENANCE_INSTRUCTION_SECRET",
		"Prefer deterministic repository search",
		"BUNDLE_COMMIT_SUBJECT_SECRET",
		"test@example.com",
		"Test User",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("bundle provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderSkillsReportRoutesBundleProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSkillBundleProvenanceGitFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /bundles provenance"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 166,
			"title": "@gitclaw /bundles provenance",
			"body": "Hidden bundle provenance body token: BUNDLE_PROVENANCE_ROUTE_BODY_SECRET.",
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
		"GitClaw Skill Bundle Provenance Report",
		"repository: `owner/repo`",
		"issue: `#166`",
		"bundle_provenance_status: `ok`",
		"available_bundles: `1`",
		"git_tracked_bundles: `1`",
		"bundles_with_commits: `1`",
		"issue_title_sha256_12:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("bundle provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"BUNDLE_PROVENANCE_ROUTE_BODY_SECRET", "BUNDLE_PROVENANCE_INSTRUCTION_SECRET", "BUNDLE_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("bundle provenance routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestHandleBundlesProvenanceCommandPostsGitHistoryReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeSkillBundleProvenanceGitFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 167,
			"title": "@gitclaw /bundles provenance",
			"body": "Hidden bundle provenance body token: BUNDLE_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{167: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic bundle provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Skill Bundle Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/skills\"",
		"repository: `owner/repo`",
		"issue: `#167`",
		"bundle_provenance_status: `ok`",
		"available_bundles: `1`",
		"git_tracked_bundles: `1`",
		"working_tree_dirty_bundles: `0`",
		"bundles_with_commits: `1`",
		"raw_bundle_bodies_included: `false`",
		"raw_bundle_instructions_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_bundle_provenance_change: `true`",
		"bundle_name=`repo-context` path=`.gitclaw/skill-bundles/repo-context.yaml`",
		"git_history_gate=`pass`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("bundle provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"BUNDLE_PROVENANCE_HANDLER_BODY_SECRET", "BUNDLE_PROVENANCE_INSTRUCTION_SECRET", "BUNDLE_COMMIT_SUBJECT_SECRET", "test@example.com", "Test User"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("bundle provenance report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[167], "gitclaw:done") || hasLabel(github.IssueLabels[167], "gitclaw:running") || hasLabel(github.IssueLabels[167], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[167])
	}
}
