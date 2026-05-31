package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSkillRuntimeReportAuditsMetadataWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/runtime-audit/SKILL.md", `---
name: runtime-audit
description: Audit runtime metadata.
metadata:
  openclaw:
    requires:
      env:
        - GITCLAW_RUNTIME_REQUIRED_SECRET
      bins: [git]
    primaryEnv: GITCLAW_RUNTIME_REQUIRED_SECRET
    envVars:
      - name: GITCLAW_RUNTIME_OPTIONAL_SECRET
        required: false
      - name: GITCLAW_RUNTIME_DECLARED_SECRET
        required: true
    install:
      - kind: node
        package: dangerous-runtime-package
        bins: [runtime-cli]
      - kind: brew
        formula: jq
        bins: [jq]
---

# Runtime Audit
SECRET_SKILL_RUNTIME_BODY_TOKEN
`)
	t.Setenv("GITCLAW_RUNTIME_REQUIRED_SECRET", "present")
	t.Setenv("GITCLAW_RUNTIME_DECLARED_SECRET", "present")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /skills runtime"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 134,
			"title": "@gitclaw /skills runtime",
			"body": "Hidden skill runtime issue token: SKILL_RUNTIME_ISSUE_SECRET.",
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
		"GitClaw Skill Runtime Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#134`",
		"skill_runtime_status: `warn`",
		"runtime_metadata_scope: `repo-local-skill-frontmatter`",
		"available_skills: `1`",
		"skills_with_frontmatter: `1`",
		"skills_with_runtime_metadata: `1`",
		"skills_with_requirements: `1`",
		"skills_missing_requirements: `0`",
		"required_env_declarations: `2`",
		"optional_env_declarations: `1`",
		"primary_env_declarations: `1`",
		"primary_env_matched_declarations: `1`",
		"primary_env_mismatches: `0`",
		"required_bin_declarations: `1`",
		"install_specs: `2`",
		"install_bins: `3`",
		"skills_with_install_specs: `1`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"registry_contact_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_env_names_included: `false`",
		"raw_install_targets_included: `false`",
		"llm_e2e_required_after_skill_runtime_change: `true`",
		"name=`runtime-audit`",
		"path=`.gitclaw/SKILLS/runtime-audit/SKILL.md`",
		"runtime_metadata=`true`",
		"required_env=`2`",
		"optional_env=`1`",
		"required_env_sha256_12=",
		"optional_env_sha256_12=",
		"primary_env_present=`true`",
		"primary_env_declared=`true`",
		"required_bins=`1`",
		"install_kinds=`brew, node`",
		"install_targets_sha256_12=",
		"### Runtime Gates",
		"raw_metadata_gate=`hash_only`",
		"### Runtime Findings",
		"code=`declared_install_specs_inert`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill runtime report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"SECRET_SKILL_RUNTIME_BODY_TOKEN",
		"SKILL_RUNTIME_ISSUE_SECRET",
		"GITCLAW_RUNTIME_REQUIRED_SECRET",
		"GITCLAW_RUNTIME_OPTIONAL_SECRET",
		"GITCLAW_RUNTIME_DECLARED_SECRET",
		"dangerous-runtime-package",
		"runtime-cli",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill runtime report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestSkillsRuntimeCommandReportsCurrentRepoMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
metadata:
  openclaw:
    primaryEnv: GITCLAW_RUNTIME_CLI_ENV
    envVars:
      - name: GITCLAW_RUNTIME_CLI_ENV
        required: false
---

SECRET_SKILL_RUNTIME_CLI_BODY_TOKEN
`)
	t.Setenv("GITCLAW_WORKDIR", root)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "runtime"}); err != nil {
			t.Fatalf("skills runtime returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Skill Runtime Report",
		"scope: `local-cli`",
		"skill_runtime_status: `ok`",
		"available_skills: `1`",
		"skills_with_runtime_metadata: `1`",
		"optional_env_declarations: `1`",
		"primary_env_declarations: `1`",
		"primary_env_matched_declarations: `1`",
		"install_specs: `0`",
		"raw_env_names_included: `false`",
		"### Runtime Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills runtime output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_RUNTIME_CLI_BODY_TOKEN", "GITCLAW_RUNTIME_CLI_ENV"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("skills runtime output leaked %q:\n%s", leaked, output)
		}
	}
}
