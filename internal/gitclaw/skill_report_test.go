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

func TestRenderSkillsReportRoutesRiskAuditWithoutBody(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

SECRET_SKILL_RISK_ROUTE_BODY_TOKEN
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /skills risk"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 117,
			"title": "@gitclaw /skills risk",
			"body": "Hidden skill risk route token: SKILL_RISK_ROUTE_BODY_SECRET.",
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
		"GitClaw Skills Risk Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#117`",
		"skill_risk_status: `ok`",
		"available_skills: `1`",
		"raw_bodies_included: `false`",
		"### Skill Risk Cards",
		"name=`repo-reader`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skills risk route report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_RISK_ROUTE_BODY_TOKEN", "SKILL_RISK_ROUTE_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skills risk route leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillSelectPlanReportExplainsSelectionWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

# Repo Reader
SECRET_SKILL_SELECT_PLAN_BODY_TOKEN
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "Please use repo-reader for this turn."}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 116,
			"title": "@gitclaw /skills select-plan repo-reader",
			"body": "Hidden skill select plan token: SKILL_SELECT_PLAN_BODY_SECRET.",
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
		"GitClaw Skill Select Plan Report",
		"Generated without a model call",
		"skill_select_plan_status: `ok`",
		"requested_skill_sha256_12:",
		"request_text_sha256_12:",
		"available_skills: `1`",
		"matched_skills: `1`",
		"selected_skills: `1`",
		"selected_for_this_turn: `true`",
		"skill_enabled: `true`",
		"disabled_by_config: `false`",
		"blocked_by_allowlist: `false`",
		"always_on: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_requested_skill_included: `false`",
		"raw_request_text_included: `false`",
		"raw_skill_body_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"skill_validation_status: `ok`",
		"### Skill Match",
		"skill_name=`repo-reader`",
		"path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"### Selection Reasons",
		"reasons=`request_metadata_match`",
		"### Review Steps",
		"Use a live GitHub Models conversation E2E",
		"### Findings",
		"code=`progressive_disclosure`",
		"code=`repository_mutation_disabled`",
		"code=`skill_selected_for_turn`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill select plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_SELECT_PLAN_BODY_TOKEN", "SKILL_SELECT_PLAN_BODY_SECRET", "Please use repo-reader"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill select plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillRefreshPlanReportExplainsPerTurnDiscoveryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

# Repo Reader
SECRET_SKILL_REFRESH_PLAN_BODY_TOKEN
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg.AllowedSkills = map[string]bool{"repo-reader": true}
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "Please use repo-reader while checking skill refresh."}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 117,
			"title": "@gitclaw /skills refresh-plan",
			"body": "Hidden skill refresh plan token: SKILL_REFRESH_PLAN_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderSkillsReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Skill Refresh Plan Report",
		"Generated without a model call",
		"skill_refresh_plan_status: `needs_review`",
		"refresh_strategy: `github-actions-per-turn-discovery`",
		"refresh_trigger: `next-issue-comment-or-workflow-dispatch-run`",
		"current_snapshot_scope: `current-actions-checkout`",
		"resident_skill_watcher: `false`",
		"mid_run_hot_reload_supported: `false`",
		"session_snapshot_reused_across_runs: `false`",
		"skill_snapshot_persisted: `false`",
		"remote_node_refresh_supported: `false`",
		"remote_registry_polling_allowed: `false`",
		"workflow_dispatch_refresh_supported: `true`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_included: `false`",
		"available_skills: `1`",
		"enabled_skills: `1`",
		"selected_skills: `1`",
		"skill_hashes: `1`",
		"skill_index_sha256_12:",
		"config_allowed_skills: `1`",
		"config_disabled_skills: `0`",
		"llm_e2e_required_after_skill_refresh_change: `true`",
		"skill_validation_status: `ok`",
		"### Refresh Boundary",
		"kind=`runtime` refresh_strategy=`github-actions-per-turn-discovery`",
		"kind=`source` repo_review_required=`true`",
		"kind=`prompt` progressive_disclosure=`true`",
		"### Current Skill Snapshot",
		"name=`repo-reader` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"selected_for_this_turn=`true`",
		"### Refresh Steps",
		"Start a new issue/comment turn or dispatch the workflow",
		"### Findings",
		"code=`per_turn_discovery`",
		"code=`resident_watcher_disabled`",
		"code=`progressive_disclosure`",
		"code=`repository_mutation_disabled`",
		"code=`live_llm_e2e_required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill refresh plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_REFRESH_PLAN_BODY_TOKEN", "SKILL_REFRESH_PLAN_BODY_SECRET", "Please use repo-reader"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill refresh plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillInstallPlanReportPlansExistingSkillWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
metadata:
  openclaw:
    requires:
      bins: [git]
---

# Repo Reader
SECRET_SKILL_INSTALL_BODY_TOKEN
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "skills install-plan repo-reader"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 114,
			"title": "@gitclaw /skills install-plan repo-reader e2e",
			"body": "Hidden skill install plan token: SKILL_INSTALL_PLAN_BODY_SECRET.",
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
		"GitClaw Skill Install Plan Report",
		"Generated without a model call",
		"install_plan_status: `needs_review`",
		"operation: `install-plan`",
		"target_type: `registry-name`",
		"target_sha256_12:",
		"target_terms: `1`",
		"safe_name_candidate: `repo-reader`",
		"destination_path: `.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"destination_exists: `true`",
		"existing_skill_matches: `1`",
		"available_skills: `1`",
		"run_mode: `read-only`",
		"remote_fetch_allowed: `false`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"manual_review_required: `true`",
		"llm_e2e_required_after_change: `true`",
		"raw_target_included: `false`",
		"raw_manifest_included: `false`",
		"raw_skill_body_included: `false`",
		"skill_validation_status: `ok`",
		"### Existing Skill Matches",
		"skill_name=`repo-reader`",
		"selected_for_this_turn=`true`",
		"### Review Steps",
		"Run a live GitHub Models conversation E2E",
		"### Findings",
		"code=`manual_review_required`",
		"code=`installer_scripts_disabled`",
		"code=`repository_mutation_disabled`",
		"code=`existing_skill_found`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill install plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_INSTALL_BODY_TOKEN", "SKILL_INSTALL_PLAN_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill install plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillProposalPlanReportPlansReviewedUpdateWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository files.
---

# Repo Reader
SECRET_SKILL_PROPOSAL_EXISTING_BODY
`)
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "skills proposal-plan repo-reader"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 116,
			"title": "@gitclaw /skills proposal-plan repo-reader e2e",
			"body": "Draft reusable repo-reader improvement. Hidden proposal token: SKILL_PROPOSAL_BODY_SECRET.",
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
		"GitClaw Skill Proposal Plan Report",
		"Generated without a model call",
		"proposal_plan_status: `needs_review`",
		"operation: `proposal-plan`",
		"requested_action: `auto`",
		"planned_proposal_action: `propose-update`",
		"target_type: `registry-name`",
		"target_sha256_12:",
		"target_terms: `1`",
		"safe_name_candidate: `repo-reader`",
		"proposal_path: `.gitclaw/skill-proposals/repo-reader/PROPOSAL.md`",
		"destination_path: `.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"existing_skill_matches: `1`",
		"available_skills: `1`",
		"proposal_store: `git-reviewed-proposal-file`",
		"proposal_support_dirs: `assets,examples,references,scripts,templates`",
		"review_pr_required: `true`",
		"remote_fetch_allowed: `false`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"proposal_mutation_allowed: `false`",
		"active_skill_write_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"autonomous_skill_creation: `false`",
		"autonomous_skill_improvement: `false`",
		"manual_review_required: `true`",
		"llm_e2e_required_after_change: `true`",
		"proposal_source_sha256_12:",
		"raw_target_included: `false`",
		"raw_proposal_body_included: `false`",
		"raw_skill_body_included: `false`",
		"raw_existing_skill_body_included: `false`",
		"skill_validation_status: `ok`",
		"### Existing Skill Matches",
		"skill_name=`repo-reader`",
		"selected_for_this_turn=`true`",
		"### Proposal Review Steps",
		"GitClaw does not auto-apply proposals",
		"### Findings",
		"code=`manual_review_required`",
		"code=`proposal_store_git_backed`",
		"code=`autonomous_apply_disabled`",
		"code=`repository_mutation_disabled`",
		"code=`live_llm_e2e_required`",
		"code=`existing_skill_update_review`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill proposal plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_SKILL_PROPOSAL_EXISTING_BODY", "SKILL_PROPOSAL_BODY_SECRET", "Draft reusable repo-reader improvement"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill proposal plan report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillInstallPlanReportDoesNotLeakRemoteURLSecrets(t *testing.T) {
	root := t.TempDir()
	ctx, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 115,
			"title": "@gitclaw /skills install-plan https://github.com/example/repo-reader?token=REMOTE_URL_SECRET",
			"body": "Hidden remote install body token: REMOTE_INSTALL_BODY_SECRET.",
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
		"GitClaw Skill Install Plan Report",
		"install_plan_status: `needs_review`",
		"target_type: `github-url`",
		"safe_name_candidate: `repo-reader`",
		"destination_path: `.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"destination_exists: `false`",
		"existing_skill_matches: `0`",
		"remote_fetch_allowed: `false`",
		"raw_target_included: `false`",
		"raw_manifest_included: `false`",
		"raw_skill_body_included: `false`",
		"code=`network_fetch_disabled`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("remote skill install plan report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"REMOTE_URL_SECRET", "REMOTE_INSTALL_BODY_SECRET", "https://github.com/example/repo-reader"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("remote skill install plan report leaked %q:\n%s", leaked, report)
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

func TestRenderSkillBundlesReportListsRepoBundlesWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SECRET_BUNDLE_SKILL_BODY_TOKEN`)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", `name: repo-context
description: Repository context workflow.
skills:
  - repo-reader
  - missing-skill
instruction: |
  SECRET_BUNDLE_INSTRUCTION_TOKEN
`)
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /repo-context inspect go.mod"}}, DefaultConfig())
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 134,
			"title": "@gitclaw /bundles",
			"body": "Hidden bundle list token: BUNDLE_LIST_BODY_SECRET.",
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
		"GitClaw Skill Bundles Report",
		"Generated without a model call",
		"available_bundles: `1`",
		"selected_bundles: `1`",
		"available_skills: `1`",
		"bundle_skill_refs: `2`",
		"resolved_bundle_skills: `1`",
		"missing_bundle_skills: `1`",
		"bundles_with_instruction: `1`",
		"raw_bodies_included: `false`",
		"bundle_name=`repo-context`",
		"path=`.gitclaw/skill-bundles/repo-context.yaml`",
		"skills=`missing-skill, repo-reader`",
		"resolved_skills=`repo-reader`",
		"missing_skills=`missing-skill`",
		"selected_for_this_turn=`true`",
		"instruction=`true`",
		"sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill bundles report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_BUNDLE_SKILL_BODY_TOKEN", "SECRET_BUNDLE_INSTRUCTION_TOKEN", "BUNDLE_LIST_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill bundles report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillBundleInfoReportShowsOneBundleWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SECRET_BUNDLE_INFO_SKILL_BODY_TOKEN`)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", `name: repo-context
description: Repository context workflow.
skills:
  - repo-reader
instruction: |
  SECRET_BUNDLE_INFO_INSTRUCTION_TOKEN
`)
	ctx, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 135,
			"title": "@gitclaw /bundles info repo-context",
			"body": "Hidden bundle info token: BUNDLE_INFO_BODY_SECRET.",
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
		"GitClaw Skill Bundle Info Report",
		"Generated without a model call",
		"requested_bundle: `repo-context`",
		"skill_bundle_info_status: `ok`",
		"available_bundles: `1`",
		"matched_bundles: `1`",
		"available_skills: `1`",
		"raw_bodies_included: `false`",
		"bundle_name=`repo-context`",
		"path=`.gitclaw/skill-bundles/repo-context.yaml`",
		"skills=`repo-reader`",
		"resolved_skills=`repo-reader`",
		"missing_skills=`none`",
		"selected_for_this_turn=`false`",
		"instruction=`true`",
		"sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill bundle info report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SECRET_BUNDLE_INFO_SKILL_BODY_TOKEN", "SECRET_BUNDLE_INFO_INSTRUCTION_TOKEN", "BUNDLE_INFO_BODY_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill bundle info report leaked %q:\n%s", leaked, report)
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
