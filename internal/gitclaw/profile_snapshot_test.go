package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeProfileSnapshotFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_PROFILE_SNAPSHOT_SECRET")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "IDENTITY_PROFILE_SNAPSHOT_SECRET")
	writeTestFile(t, root, ".gitclaw/USER.md", "USER_PROFILE_SNAPSHOT_SECRET")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_PROFILE_SNAPSHOT_SECRET")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_PROFILE_SNAPSHOT_SECRET")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_PROFILE_SNAPSHOT_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-30.md", "MEMORY_NOTE_PROFILE_SNAPSHOT_SECRET")
	writeSkillSnapshotFixture(t, root)
	writeTestFile(t, root, ".gitclaw/toolsets/repo-read.yaml", "name: repo-read\ntools:\n  - gitclaw.list_files\n")
}

func TestRenderProfileSnapshotReportFingerprintsProfileWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileSnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile snapshot."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	body := RenderProfileSnapshotCLIReport(cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Snapshot Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"profile_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-profile-snapshot-v1`",
		"snapshot_scope: `repo-local-profile-soul-memory-skills-tools`",
		"snapshot_sha256_12:",
		"snapshot_entries: `5`",
		"profile_documents_loaded:",
		"available_skills: `1`",
		"selected_skills: `1`",
		"skill_bundles: `1`",
		"available_tools: `5`",
		"manifest_entries:",
		"profile_manifest_sha256_12:",
		"soul_snapshot_sha256_12:",
		"memory_snapshot_sha256_12:",
		"skill_snapshot_sha256_12:",
		"tool_snapshot_sha256_12:",
		"profile_export_supported: `false`",
		"profile_import_supported: `false`",
		"profile_switching_supported: `false`",
		"profile_mutation_allowed: `false`",
		"raw_profile_bodies_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credentials_included: `false`",
		"sessions_included: `false`",
		"backup_payloads_included: `false`",
		"llm_e2e_required_after_profile_snapshot_change: `true`",
		"### Snapshot Components",
		"kind=`profile-manifest` name=`profile-manifest` status=`ok`",
		"kind=`soul-snapshot` name=`soul` status=`ok` version=`gitclaw-soul-snapshot-v1`",
		"kind=`memory-snapshot` name=`memory` status=`ok` version=`gitclaw-memory-snapshot-v1`",
		"kind=`skill-snapshot` name=`skills` status=`ok` version=`gitclaw-skill-snapshot-v1`",
		"kind=`tool-snapshot` name=`tools` status=`ok` version=`gitclaw-tool-snapshot-v1`",
		"raw_bodies_included=`false`",
		"mutation_allowed=`false`",
		"### Snapshot Gates",
		"manifest_gate=`pass`",
		"soul_gate=`pass`",
		"memory_gate=`pass`",
		"skills_gate=`pass`",
		"tools_gate=`pass`",
		"profile_export_gate=`disabled`",
		"profile_import_gate=`disabled`",
		"profile_switching_gate=`disabled`",
		"mutation_gate=`disabled`",
		"session_payload_gate=`excluded`",
		"backup_payload_gate=`excluded`",
		"raw_body_gate=`hash_only`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile snapshot report missing %q:\n%s", want, body)
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
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile snapshot report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderProfileReportRoutesSnapshotWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileSnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile snapshot."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 190,
			Title:  "@gitclaw /profile snapshot",
			Body:   "Hidden profile snapshot issue token: PROFILE_SNAPSHOT_ROUTE_BODY_SECRET. repo-reader",
		},
	}
	body := RenderProfileReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Snapshot Report",
		"repository: `owner/repo`",
		"issue: `#190`",
		"profile_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-profile-snapshot-v1`",
		"snapshot_sha256_12:",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_profile_snapshot_change: `true`",
		"kind=`skill-snapshot` name=`skills`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile snapshot route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROFILE_SNAPSHOT_ROUTE_BODY_SECRET", "SOUL_PROFILE_SNAPSHOT_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile snapshot route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestProfileSnapshotCommandReportsCompositeFingerprintWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeProfileSnapshotFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"profile", "snapshot"}); err != nil {
			t.Fatalf("profile snapshot returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Profile Snapshot Report",
		"scope: `local-cli`",
		"profile_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-profile-snapshot-v1`",
		"snapshot_entries: `5`",
		"profile_manifest_sha256_12:",
		"soul_snapshot_sha256_12:",
		"memory_snapshot_sha256_12:",
		"skill_snapshot_sha256_12:",
		"tool_snapshot_sha256_12:",
		"raw_profile_bodies_included: `false`",
		"llm_e2e_required_after_profile_snapshot_change: `true`",
		"### Snapshot Components",
		"kind=`profile-manifest`",
		"kind=`skill-snapshot`",
		"kind=`tool-snapshot`",
		"### Snapshot Gates",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("profile snapshot output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SOUL_PROFILE_SNAPSHOT_SECRET", "MEMORY_PROFILE_SNAPSHOT_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_SOURCE_REF_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("profile snapshot output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleProfileSnapshotCommandPostsCompositeFingerprintWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeProfileSnapshotFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 191,
			"title": "@gitclaw /profile snapshot",
			"body": "@gitclaw /profile snapshot\nrepo-reader\nHidden profile snapshot body token: PROFILE_SNAPSHOT_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{191: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic profile snapshot command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Profile Snapshot Report",
		"Generated without a model call",
		"model=\"gitclaw/profile\"",
		"repository: `owner/repo`",
		"issue: `#191`",
		"profile_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-profile-snapshot-v1`",
		"snapshot_scope: `repo-local-profile-soul-memory-skills-tools`",
		"snapshot_sha256_12:",
		"snapshot_entries: `5`",
		"profile_manifest_sha256_12:",
		"skill_snapshot_sha256_12:",
		"tool_snapshot_sha256_12:",
		"raw_issue_bodies_included: `false`",
		"llm_e2e_required_after_profile_snapshot_change: `true`",
		"issue_title_sha256_12:",
		"### Snapshot Components",
		"kind=`profile-manifest`",
		"kind=`skill-snapshot`",
		"### Snapshot Gates",
		"manifest_gate=`pass`",
		"skills_gate=`pass`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile snapshot handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"PROFILE_SNAPSHOT_HANDLER_BODY_SECRET", "SOUL_PROFILE_SNAPSHOT_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile snapshot handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[191], "gitclaw:done") || hasLabel(github.IssueLabels[191], "gitclaw:running") || hasLabel(github.IssueLabels[191], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[191])
	}
}
