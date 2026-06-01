package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeProfileProvenanceFixture(t *testing.T, root string) {
	t.Helper()
	writeProfileSnapshotFixture(t, root)
	writeTestFile(t, root, ".gitclaw/config.yml", "trigger:\n  label: gitclaw\n")
	runTestGit(t, root, "init")
	runTestGit(t, root, "config", "user.email", "profile-provenance@example.invalid")
	runTestGit(t, root, "config", "user.name", "Profile Provenance Test")
	runTestGit(t, root, "add", ".gitclaw")
	runTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "Add profile provenance fixture PROFILE_PROVENANCE_COMMIT_SECRET")
}

func TestRenderProfileProvenanceReportMapsProfileGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileProvenanceFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile provenance."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	body := RenderProfileProvenanceCLIReport(cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Provenance Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"profile_provenance_status: `ok`",
		"provenance_scope: `repo-local-profile-git-history`",
		"provenance_sha256_12:",
		"profile_strategy: `repo-local-git-profile`",
		"profile_store: `.gitclaw/`",
		"profile_scope: `repository`",
		"profile_documents_loaded: `7`",
		"manifest_entries: `17`",
		"profile_surfaces: `12`",
		"repo_local_surfaces: `12`",
		"git_tracked_surfaces: `12`",
		"untracked_surfaces: `0`",
		"working_tree_dirty_surfaces: `0`",
		"surfaces_with_commits: `12`",
		"surfaces_without_commits: `0`",
		"available_skills: `1`",
		"skill_bundles: `1`",
		"available_tools: `5`",
		"manifest_sha256_12:",
		"profile_snapshot_sha256_12:",
		"git_available: `true`",
		"git_history_available: `true`",
		"external_profile_home_accessed: `false`",
		"profile_export_supported: `false`",
		"profile_import_supported: `false`",
		"profile_switching_supported: `false`",
		"profile_distribution_install_supported: `false`",
		"profile_mutation_allowed: `false`",
		"credentials_included: `false`",
		"sessions_included: `false`",
		"backup_payloads_included: `false`",
		"raw_profile_bodies_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"llm_e2e_required_after_profile_provenance_change: `true`",
		"### Profile Provenance Cards",
		"kind=`profile-config` name=`config` path=`.gitclaw/config.yml`",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md`",
		"kind=`profile-document` name=`memory-note` path=`.gitclaw/memory/2026-05-30.md`",
		"kind=`skill` name=`repo-reader` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"kind=`skill-bundle` name=`repo-context` path=`.gitclaw/skill-bundles/repo-context.yaml`",
		"kind=`skill-source` name=`repo-reader` path=`.gitclaw/skill-sources/repo-reader.yaml`",
		"kind=`toolset-spec` name=`repo-read` path=`.gitclaw/toolsets/repo-read.yaml`",
		"git_tracked=`true`",
		"working_tree_dirty=`false`",
		"commit_available=`true`",
		"last_commit_sha256_12=",
		"last_commit_short=",
		"last_commit_date=",
		"subject_sha256_12=",
		"### Provenance Gates",
		"manifest_gate=`pass`",
		"snapshot_gate=`pass`",
		"git_history_gate=`pass`",
		"profile_export_gate=`disabled`",
		"profile_import_gate=`disabled`",
		"profile_switching_gate=`disabled`",
		"mutation_gate=`disabled`",
		"external_profile_home_gate=`not_accessed`",
		"session_payload_gate=`excluded`",
		"backup_payload_gate=`excluded`",
		"raw_body_gate=`hash_only`",
		"git_subject_gate=`sha256_12_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile provenance report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"SOUL_PROFILE_SNAPSHOT_SECRET",
		"IDENTITY_PROFILE_SNAPSHOT_SECRET",
		"USER_PROFILE_SNAPSHOT_SECRET",
		"TOOLS_PROFILE_SNAPSHOT_SECRET",
		"MEMORY_PROFILE_SNAPSHOT_SECRET",
		"HEARTBEAT_PROFILE_SNAPSHOT_SECRET",
		"MEMORY_NOTE_PROFILE_SNAPSHOT_SECRET",
		"SKILL_SNAPSHOT_BODY_SECRET",
		"SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET",
		"SKILL_SNAPSHOT_SOURCE_REF_SECRET",
		"PROFILE_PROVENANCE_COMMIT_SECRET",
		"profile-provenance@example.invalid",
		"Profile Provenance Test",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile provenance report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderProfileReportRoutesProvenanceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileProvenanceFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile provenance."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 192,
			Title:  "@gitclaw /profile provenance",
			Body:   "Hidden profile provenance issue token: PROFILE_PROVENANCE_ROUTE_BODY_SECRET. repo-reader",
		},
	}
	body := RenderProfileReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Provenance Report",
		"repository: `owner/repo`",
		"issue: `#192`",
		"profile_provenance_status: `ok`",
		"provenance_scope: `repo-local-profile-git-history`",
		"profile_surfaces: `12`",
		"git_tracked_surfaces: `12`",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_profile_provenance_change: `true`",
		"kind=`skill` name=`repo-reader`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile provenance route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROFILE_PROVENANCE_ROUTE_BODY_SECRET", "SOUL_PROFILE_SNAPSHOT_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "PROFILE_PROVENANCE_COMMIT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile provenance route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestProfileProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileProvenanceFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"profile", "provenance"}); err != nil {
			t.Fatalf("profile provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Profile Provenance Report",
		"scope: `local-cli`",
		"profile_provenance_status: `ok`",
		"profile_surfaces: `12`",
		"git_tracked_surfaces: `12`",
		"surfaces_with_commits: `12`",
		"raw_profile_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"llm_e2e_required_after_profile_provenance_change: `true`",
		"kind=`profile-config` name=`config`",
		"kind=`toolset-spec` name=`repo-read`",
		"git_history_gate=`pass`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("profile provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SOUL_PROFILE_SNAPSHOT_SECRET", "MEMORY_PROFILE_SNAPSHOT_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_SOURCE_REF_SECRET", "PROFILE_PROVENANCE_COMMIT_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("profile provenance output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleProfileProvenanceCommandPostsGitHistoryWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeProfileProvenanceFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 193,
			"title": "@gitclaw /profile provenance",
			"body": "@gitclaw /profile provenance\nrepo-reader\nHidden profile provenance body token: PROFILE_PROVENANCE_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{193: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic profile provenance command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Profile Provenance Report",
		"Generated without a model call",
		"model=\"gitclaw/profile\"",
		"repository: `owner/repo`",
		"issue: `#193`",
		"profile_provenance_status: `ok`",
		"provenance_scope: `repo-local-profile-git-history`",
		"profile_surfaces: `12`",
		"git_tracked_surfaces: `12`",
		"surfaces_with_commits: `12`",
		"raw_issue_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"llm_e2e_required_after_profile_provenance_change: `true`",
		"issue_title_sha256_12:",
		"### Profile Provenance Cards",
		"kind=`profile-config`",
		"kind=`skill` name=`repo-reader`",
		"### Provenance Gates",
		"git_history_gate=`pass`",
		"git_subject_gate=`sha256_12_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile provenance handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROFILE_PROVENANCE_HANDLER_BODY_SECRET", "SOUL_PROFILE_SNAPSHOT_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET", "PROFILE_PROVENANCE_COMMIT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile provenance handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[193], "gitclaw:done") || hasLabel(github.IssueLabels[193], "gitclaw:running") || hasLabel(github.IssueLabels[193], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[193])
	}
}
