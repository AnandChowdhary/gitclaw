package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderMigrationPlanReportMapsOpenClawWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul policy.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity policy.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "User profile.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tool policy.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat.\n")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

MIGRATION_SKILL_BODY_SECRET
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "migrate plan openclaw"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 139,
			"title": "@gitclaw /migrate plan openclaw",
			"body": "Hidden migration body token: MIGRATION_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderMigrationReport(ev, DefaultConfig(), ctx)
	for _, want := range []string{
		"GitClaw Migration Plan Report",
		"Generated without a model call",
		"migration_plan_status: `needs_review`",
		"requested_source_sha256_12:",
		"normalized_source: `openclaw`",
		"supported_source: `true`",
		"plan_scope: `repo-local-declarative-state`",
		"source_scan_allowed: `false`",
		"apply_supported: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"backup_required_before_apply: `true`",
		"credentials_import_allowed: `false`",
		"executable_state_import_allowed: `false`",
		"raw_source_body_included: `false`",
		"raw_secret_values_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"required_context_files_present: `6`",
		"available_skills: `1`",
		"tool_contracts: `5`",
		"backup_branch: `gitclaw-backups`",
		"soul_validation_status: `ok`",
		"skill_validation_status: `ok`",
		"tool_validation_status: `ok`",
		"### Source Import Map",
		"source_kind=`SOUL.md` target=`.gitclaw/SOUL.md` action=`manual-copy`",
		"source_kind=`skills/<name>/SKILL.md` target=`.gitclaw/SKILLS/<name>/SKILL.md` action=`manual-copy`",
		"source_kind=`auth profiles/.env` target=`manual secret setup` action=`skip`",
		"### Current GitClaw Target Inventory",
		"kind=`context` path=`.gitclaw/SOUL.md`",
		"kind=`skill` name=`repo-reader`",
		"### Review Steps",
		"Verify the current GitClaw backup branch",
		"### Findings",
		"code=`preview_first`",
		"code=`backup_first`",
		"code=`credentials_manual`",
		"code=`executable_state_quarantined`",
		"code=`manual_review_required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("migration report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"MIGRATION_SKILL_BODY_SECRET", "MIGRATION_BODY_SECRET", "Hidden migration body token"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("migration report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderMigrationRiskReportAuditsHermesBoundaryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul policy.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity policy.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "User profile.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tool policy.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat.\n")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

MIGRATION_RISK_SKILL_BODY_SECRET
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "migrate risk hermes"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 141,
			"title": "@gitclaw /migrate risk hermes",
			"body": "Hidden migration risk body token: MIGRATION_RISK_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderMigrationReport(ev, DefaultConfig(), ctx)
	for _, want := range []string{
		"GitClaw Migration Risk Report",
		"Generated without a model call",
		"migration_risk_status: `needs_review`",
		"verification_scope: `agent_state_migration_boundary`",
		"requested_source_sha256_12:",
		"normalized_source: `hermes`",
		"supported_source: `true`",
		"provider_import_items: `10`",
		"manual_review_items: `1`",
		"archive_only_items: `1`",
		"skipped_items: `1`",
		"credential_items: `1`",
		"executable_state_items: `2`",
		"memory_items: `2`",
		"skill_items: `1`",
		"session_archive_items: `1`",
		"source_scan_allowed: `false`",
		"source_home_read: `false`",
		"source_paths_printed: `false`",
		"apply_supported: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"credentials_import_allowed: `false`",
		"executable_state_import_allowed: `false`",
		"installer_execution_allowed: `false`",
		"mcp_autoload_allowed: `false`",
		"raw_source_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_secret_values_included: `false`",
		"backup_required_before_apply: `true`",
		"human_review_required: `true`",
		"quarantine_required: `true`",
		"soul_validation_status: `ok`",
		"skill_validation_status: `ok`",
		"tool_validation_status: `ok`",
		"llm_e2e_required_after_migration_risk_change: `true`",
		"### Source Boundary Risk Card",
		"### Apply Boundary Risk Card",
		"### Current Target Validation Cards",
		"### Provider Import Risk Cards",
		"source_kind=`mcp_servers` target=`.gitclaw/TOOLS.md` action=`manual-review`",
		"source_kind=`auth.json/.env` target=`manual secret setup` action=`skip`",
		"source_kind=`sessions/state.db` target=`gitclaw-backups archive` action=`archive-only`",
		"code=`credential_import_disabled`",
		"code=`executable_state_quarantined`",
		"code=`raw_state_archive_only`",
		"code=`skill_manual_review_required`",
		"code=`memory_review_required`",
		"code=`manual_review_required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("migration risk report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"MIGRATION_RISK_SKILL_BODY_SECRET", "MIGRATION_RISK_BODY_SECRET", "Hidden migration risk body token"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("migration risk report leaked %q:\n%s", leaked, report)
		}
	}
}
