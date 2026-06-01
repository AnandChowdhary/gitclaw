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

func TestRenderProfileCatalogReportShowsCommandAndLayerSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "IDENTITY_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/USER.md", "USER_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-30.md", "MEMORY_NOTE_PROFILE_CATALOG_SECRET")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SKILL_PROFILE_CATALOG_SECRET`)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", "name: repo-context\nskills:\n  - repo-reader\n")
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", "PROACTIVE_PROFILE_CATALOG_SECRET")

	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile catalog."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 178,
			"title": "@gitclaw /profile catalog",
			"body": "Hidden profile catalog route token: PROFILE_CATALOG_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderProfileReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Catalog Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#178`",
		"requested_profile_command: `catalog`",
		"profile_command_status: `ok`",
		"profile_catalog_status: `ok`",
		"catalog_strategy: `compact-repo-local-profile-discovery`",
		"profile_strategy: `repo-local-git-profile`",
		"profile_surface: `identity, user, soul, memory, skills, bundles, tools, models, proactive, hooks, channels, backups, sessions`",
		"catalog_entries: `9`",
		"profile_layers: `13`",
		"profile_documents_loaded: `7`",
		"identity_policy_files: `6`",
		"memory_notes: `1`",
		"available_skills: `1`",
		"selected_skills: `1`",
		"skill_bundles: `1`",
		"available_tools: `5`",
		"raw_bodies_included: `false`",
		"raw_profile_payloads_included: `false`",
		"raw_config_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"profile_mutation_allowed: `false`",
		"profile_switching_supported: `false`",
		"profile_import_supported: `false`",
		"profile_export_supported: `false`",
		"llm_e2e_required_after_profile_catalog_change: `true`",
		"### Catalog Entries",
		"command=`catalog` issue_intent=`@gitclaw /profile catalog` local_command=`gitclaw profile catalog` execution=`metadata-only` gate=`body-free-output`",
		"command=`provenance` issue_intent=`@gitclaw /profile provenance` local_command=`gitclaw profile provenance` execution=`repo-local-git-history` gate=`commit-subject-hashes-only`",
		"command=`search` issue_intent=`@gitclaw /profile search <query>` local_command=`gitclaw profile search <query>` execution=`body-free-profile-search` gate=`query-hash-and-line-hashes`",
		"command=`snapshot` issue_intent=`@gitclaw /profile snapshot` local_command=`gitclaw profile snapshot` execution=`composite-profile-fingerprint` gate=`body-free-snapshot`",
		"command=`risk` issue_intent=`@gitclaw /profile risk` local_command=`gitclaw profile risk` execution=`repo-local-risk-audit` gate=`profile-isolation`",
		"### Profile Layers",
		"layer=`identity` store=`.gitclaw/IDENTITY.md`",
		"layer=`proactive` store=`.gitclaw/proactive + .github/workflows` source=`scheduled-workflow-prompts` gate=`workflow-dispatch-issue-ingress` count=`1`",
		"layer=`sessions` store=`GitHub issue thread + backup JSON`",
		"### Catalog Gates",
		"profile_store_gate=`repo-local-reviewed-files`",
		"switching_gate=`unsupported-single-repository-profile`",
		"raw_body_gate=`hashes-counts-and-metadata-only`",
		"session_gate=`github-issue-thread-plus-backup-json`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("profile catalog report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SOUL_PROFILE_CATALOG_SECRET", "IDENTITY_PROFILE_CATALOG_SECRET", "USER_PROFILE_CATALOG_SECRET", "TOOLS_PROFILE_CATALOG_SECRET", "MEMORY_PROFILE_CATALOG_SECRET", "HEARTBEAT_PROFILE_CATALOG_SECRET", "MEMORY_NOTE_PROFILE_CATALOG_SECRET", "SKILL_PROFILE_CATALOG_SECRET", "PROACTIVE_PROFILE_CATALOG_SECRET", "PROFILE_CATALOG_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("profile catalog report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderProfileManifestReportShowsPortabilityPlanWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", "model:\n  provider: github-models\n  name: openai/gpt-5-nano\n")
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "IDENTITY_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/USER.md", "USER_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "TOOLS_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-30.md", "MEMORY_NOTE_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SKILL_PROFILE_MANIFEST_SECRET`)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", "name: repo-context\nskills:\n  - repo-reader\n")
	writeTestFile(t, root, ".gitclaw/proactive/repo-hygiene.md", "PROACTIVE_PROFILE_MANIFEST_SECRET")
	writeTestFile(t, root, ".gitclaw/toolsets/repo-read.yaml", "name: repo-read\ntools:\n  - gitclaw.list_files\n")

	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Use repo-reader for profile manifest."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	report := renderProfileManifestReport(Event{}, cfg, ctx, false)
	for _, want := range []string{
		"GitClaw Profile Manifest Report",
		"Generated without a model call",
		"scope: `local-cli`",
		"profile_manifest_status: `ok`",
		"profile_strategy: `repo-local-git-profile`",
		"manifest_strategy: `dry-run-metadata-only`",
		"manifest_supported: `true`",
		"profile_export_supported: `false`",
		"profile_import_supported: `false`",
		"profile_switching_supported: `false`",
		"profile_distribution_install_supported: `false`",
		"profile_mutation_allowed: `false`",
		"profile_documents_loaded: `7`",
		"required_profile_documents: `6`",
		"required_profile_documents_present: `6`",
		"required_profile_documents_missing: `0`",
		"available_skills: `1`",
		"selected_skills: `1`",
		"skill_bundles: `1`",
		"available_tools: `5`",
		"config_file_present: `true`",
		"manifest_entries:",
		"manifest_sha256_12:",
		"credentials_included: `false`",
		"sessions_included: `false`",
		"backup_payloads_included: `false`",
		"raw_bodies_included: `false`",
		"raw_config_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"llm_e2e_required_after_profile_manifest_change: `true`",
		"### Manifest Entries",
		"kind=`profile-config` name=`config` path=`.gitclaw/config.yml` category=`config` source=`repo-local` include_policy=`metadata-only`",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md` category=`soul` source=`repo-local` include_policy=`repo-reviewed-source` portable=`true` required=`true` present=`true` selected=`true` enabled=`true` body_in_report=`false`",
		"kind=`profile-document` name=`memory-note` path=`.gitclaw/memory/2026-05-30.md`",
		"kind=`skill` name=`repo-reader` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"kind=`skill-bundle` name=`repo-context` path=`.gitclaw/skill-bundles/repo-context.yaml`",
		"kind=`proactive-prompt` name=`repo-hygiene` path=`.gitclaw/proactive/repo-hygiene.md`",
		"kind=`toolset-spec` name=`repo-read` path=`.gitclaw/toolsets/repo-read.yaml`",
		"kind=`tool-contract` name=`gitclaw.search_files` path=`tool:gitclaw.search_files` category=`tool` source=`runtime-contract` include_policy=`contract-only` portable=`false`",
		"### Excluded State",
		"kind=`credentials`",
		"kind=`sessions`",
		"kind=`backup-payloads`",
		"kind=`external-profile-home`",
		"kind=`profile-mutation`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("profile manifest report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SOUL_PROFILE_MANIFEST_SECRET", "IDENTITY_PROFILE_MANIFEST_SECRET", "USER_PROFILE_MANIFEST_SECRET", "TOOLS_PROFILE_MANIFEST_SECRET", "MEMORY_PROFILE_MANIFEST_SECRET", "HEARTBEAT_PROFILE_MANIFEST_SECRET", "MEMORY_NOTE_PROFILE_MANIFEST_SECRET", "SKILL_PROFILE_MANIFEST_SECRET", "PROACTIVE_PROFILE_MANIFEST_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("profile manifest leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderProfileReportRoutesManifestWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "SOUL_PROFILE_MANIFEST_ROUTE_SECRET")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity.")
	writeTestFile(t, root, ".gitclaw/USER.md", "USER_PROFILE_MANIFEST_ROUTE_SECRET")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tools.")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory.")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat.")
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 177,
			"title": "@gitclaw /profile manifest",
			"body": "Hidden profile manifest route token: PROFILE_MANIFEST_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	body := RenderProfileReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Profile Manifest Report",
		"repository: `owner/repo`",
		"issue: `#177`",
		"profile_manifest_status: `ok`",
		"issue_title_sha256_12:",
		"kind=`profile-document` name=`soul` path=`.gitclaw/SOUL.md`",
		"raw_bodies_included: `false`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("profile manifest route report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SOUL_PROFILE_MANIFEST_ROUTE_SECRET", "USER_PROFILE_MANIFEST_ROUTE_SECRET", "PROFILE_MANIFEST_ROUTE_BODY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("profile manifest route report leaked %q:\n%s", leaked, body)
		}
	}
}
