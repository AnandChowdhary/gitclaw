package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderProfileReportShowsEnvelopeWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "IDENTITY_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/USER.md", "USER_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-30.md", "MEMORY_NOTE_PROFILE_SECRET")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SKILL_PROFILE_SECRET`)

	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile."}}, DefaultConfig())
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 127,
			"title": "@gitclaw /profile",
			"body": "Hidden issue token: PROFILE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderProfileReport(ev, DefaultConfig(), ctx)
	for _, want := range []string{
		"GitClaw Profile Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#127`",
		"profile_status: `ok`",
		"profile_strategy: `repo-local-git-profile`",
		"profile_store: `.gitclaw/`",
		"profile_scope: `repository`",
		"provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"run_mode: `read-only`",
		"profile_documents_loaded: `7`",
		"identity_policy_files: `6`",
		"memory_notes: `1`",
		"available_skills: `1`",
		"selected_skills: `1`",
		"skill_bundles: `0`",
		"available_tools: `5`",
		"raw_bodies_included: `false`",
		"raw_profile_payloads_included: `false`",
		"### Profile Documents",
		".gitclaw/SOUL.md",
		"category=`soul`",
		".gitclaw/memory/2026-05-30.md",
		"category=`memory-note`",
		"### Skills",
		"name=`repo-reader`",
		"selected=`true`",
		"### Tool Surface",
		"gitclaw.list_files",
		"### Validation",
		"component=`soul` status=`ok`",
		"component=`skills` status=`ok`",
		"component=`tools` status=`ok`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("profile report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SOUL_PROFILE_SECRET", "IDENTITY_PROFILE_SECRET", "USER_PROFILE_SECRET", "TOOLS_PROFILE_SECRET", "MEMORY_PROFILE_SECRET", "HEARTBEAT_PROFILE_SECRET", "MEMORY_NOTE_PROFILE_SECRET", "SKILL_PROFILE_SECRET", "PROFILE_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("profile report leaked %q:\n%s", leaked, report)
		}
	}
}
