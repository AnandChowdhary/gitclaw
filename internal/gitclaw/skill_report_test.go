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

func TestRenderSkillSearchReportSearchesMetadataWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context and deterministic tool outputs.
---

# Repo Reader
SECRET_SKILL_SEARCH_BODY_TOKEN
`)
	writeTestFile(t, root, ".gitclaw/SKILLS/deploy-helper/SKILL.md", `---
name: deploy-helper
description: Prepare release deployment notes.
---

# Deploy Helper
OTHER_SKILL_SEARCH_BODY_TOKEN
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "Search for repository context skills."}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 113,
			"title": "@gitclaw /skills search repository context SEARCH_QUERY_SECRET",
			"body": "Hidden skill search body token: SKILL_SEARCH_BODY_SECRET.",
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
		"GitClaw Skills Search Report",
		"Generated without a model call",
		"skill_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"available_skills: `2`",
		"matched_skills: `1`",
		"run_mode: `read-only`",
		"raw_bodies_included: `false`",
		"searches only skill metadata",
		"### Matches",
		"skill_name=`repo-reader`",
		"path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"match_fields=`description`",
		"selected_for_this_turn=`true`",
		"sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill search report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_SEARCH_BODY_TOKEN", "OTHER_SKILL_SEARCH_BODY_TOKEN", "SKILL_SEARCH_BODY_SECRET", "SEARCH_QUERY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill search report leaked %q:\n%s", leaked, report)
		}
	}
	if strings.Contains(report, "deploy-helper") {
		t.Fatalf("skill search should not include nonmatching skill:\n%s", report)
	}
}

func TestRenderSkillsVerifyReportShowsTrustEnvelopeWithoutBodies(t *testing.T) {
	ctx := RepoContext{SkillSummaries: []SkillSummary{
		{
			Name:               "repo-reader",
			Description:        "Use read-only repository context.",
			Path:               ".gitclaw/SKILLS/repo-reader/SKILL.md",
			FrontmatterPresent: true,
			Bytes:              120,
			Lines:              8,
			SHA:                "abc123repo",
			RequiredBins:       []string{"git"},
		},
		{
			Name:               "legacy-helper",
			Description:        "Compatibility root helper.",
			Path:               ".gitclaw/skills/legacy-helper/SKILL.md",
			FrontmatterPresent: true,
			Bytes:              90,
			Lines:              7,
			SHA:                "def456legacy",
			RequiredEnv:        []string{"GITCLAW_SKILL_VERIFY_MISSING_ENV"},
			MissingEnv:         []string{"GITCLAW_SKILL_VERIFY_MISSING_ENV"},
		},
	}}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 114,
			"title": "@gitclaw /skills verify",
			"body": "Hidden skills verify body token: SKILL_VERIFY_BODY_SECRET.",
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
		"GitClaw Skills Verify Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#114`",
		"skill_verify_status: `warn`",
		"verification_scope: `repo-local-metadata`",
		"available_skills: `2`",
		"repo_local_skills: `1`",
		"compat_root_skills: `1`",
		"unknown_source_skills: `0`",
		"skills_with_hashes: `2`",
		"skills_with_requirements: `2`",
		"skills_missing_requirements: `1`",
		"registry_verification: `not_configured`",
		"installer_scripts_run: `false`",
		"raw_bodies_included: `false`",
		"skill_validation_status: `warn`",
		"### Trust Cards",
		"name=`repo-reader`",
		"path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"source=`repo-local`",
		"requirements=`declared-ok`",
		"name=`legacy-helper`",
		"path=`.gitclaw/skills/legacy-helper/SKILL.md`",
		"source=`repo-local-compat`",
		"requirements=`missing`",
		"### Verification Findings",
		"code=`registry_verification_not_configured`",
		"code=`missing_requirements`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skills verify report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_VERIFY_BODY_TOKEN", "LEGACY_VERIFY_BODY_TOKEN", "SKILL_VERIFY_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skills verify report leaked %q:\n%s", leaked, report)
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
