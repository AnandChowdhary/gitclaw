package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderSkillInfoReportListsOneSkillWithoutBody(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
metadata:
  openclaw:
    requires:
      env:
        - GITCLAW_SKILL_INFO_ENV
      bins: [git]
---

# Repo Reader
SECRET_SKILL_INFO_BODY_TOKEN
`)
	t.Setenv("GITCLAW_SKILL_INFO_ENV", "present")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for skills info."}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 111,
			"title": "@gitclaw /skills info repo-reader",
			"body": "Hidden skill info token: SKILL_INFO_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderSkillsReport(ev, DefaultConfig(), ctx)
	for _, want := range []string{
		"GitClaw Skill Info Report",
		"Generated without a model call",
		"requested_skill: `repo-reader`",
		"skill_info_status: `ok`",
		"available_skills: `1`",
		"matched_skills: `1`",
		"skill_name=`repo-reader`",
		"path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"selected_for_this_turn=`true`",
		"frontmatter=`true`",
		"description=`true`",
		"requires_env=`1`",
		"requires_bins=`1`",
		"missing_env=`0`",
		"missing_bins=`0`",
		"required_env=`GITCLAW_SKILL_INFO_ENV`",
		"required_bins=`git`",
		"missing_env=`none`",
		"### Validation For Matches",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill info report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_INFO_BODY_TOKEN", "SKILL_INFO_BODY_SECRET", "present"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill info report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRequestedSkillInfoNameRequiresInfoSubcommand(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 112,
			"title": "@gitclaw /skills e2e repo-reader",
			"body": "",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	if got := requestedSkillInfoName(ev, DefaultConfig()); got != "" {
		t.Fatalf("requestedSkillInfoName() = %q, want empty without info subcommand", got)
	}
}
