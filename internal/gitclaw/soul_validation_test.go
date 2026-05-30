package gitclaw

import (
	"strings"
	"testing"
)

func TestValidateSoulContextReportsProblemsWithoutBodies(t *testing.T) {
	repoContext := RepoContext{Documents: []ContextDocument{
		{Path: ".gitclaw/SOUL.md", Body: ""},
		{Path: ".gitclaw/USER.md", Body: "USER_PRIVATE_BODY_TOKEN"},
		{Path: ".gitclaw/memory/scratch.md", Body: "MEMORY_PRIVATE_BODY_TOKEN"},
	}}
	report := ValidateSoulContext(repoContext)
	if report.Status != "error" || report.Errors != 5 || report.Warnings != 1 || report.PresentRequiredFiles != 2 || report.MissingRequiredFiles != 4 || report.MemoryNotes != 1 || report.NoncanonicalMemoryNotes != 1 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	body := RenderSoulValidationReport(repoContext)
	for _, want := range []string{
		"GitClaw Soul Validate Report",
		"scope: `local-cli`",
		"soul_validation_status: `error`",
		"soul_validation_errors: `5`",
		"soul_validation_warnings: `1`",
		"soul_required_files: `6`",
		"soul_required_files_present: `2`",
		"soul_required_files_missing: `4`",
		"soul_memory_notes: `1`",
		"soul_noncanonical_memory_notes: `1`",
		"code=`empty_context_file`",
		"code=`missing_required_context_file`",
		"code=`noncanonical_memory_note`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("validation report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"USER_PRIVATE_BODY_TOKEN", "MEMORY_PRIVATE_BODY_TOKEN"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("validation report leaked body token %q:\n%s", leaked, body)
		}
	}
}

func TestValidateSoulContextAcceptsCurrentSoulShape(t *testing.T) {
	report := ValidateSoulContext(RepoContext{Documents: []ContextDocument{
		{Path: ".gitclaw/SOUL.md", Body: "Stay repo native."},
		{Path: ".gitclaw/IDENTITY.md", Body: "Identity: GitClaw."},
		{Path: ".gitclaw/USER.md", Body: "Maintainer preferences."},
		{Path: ".gitclaw/TOOLS.md", Body: "Read-only tools."},
		{Path: ".gitclaw/MEMORY.md", Body: "Long-term memory."},
		{Path: ".gitclaw/HEARTBEAT.md", Body: "Scheduled workflow notes."},
		{Path: ".gitclaw/memory/2026-05-29.md", Body: "Dated memory note."},
	}})
	if report.Status != "ok" || report.Errors != 0 || report.Warnings != 0 || report.PresentRequiredFiles != 6 || report.MemoryNotes != 1 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	body := RenderSoulValidationReport(RepoContext{Documents: []ContextDocument{
		{Path: ".gitclaw/SOUL.md", Body: "Stay repo native."},
		{Path: ".gitclaw/IDENTITY.md", Body: "Identity: GitClaw."},
		{Path: ".gitclaw/USER.md", Body: "Maintainer preferences."},
		{Path: ".gitclaw/TOOLS.md", Body: "Read-only tools."},
		{Path: ".gitclaw/MEMORY.md", Body: "Long-term memory."},
		{Path: ".gitclaw/HEARTBEAT.md", Body: "Scheduled workflow notes."},
		{Path: ".gitclaw/memory/2026-05-29.md", Body: "Dated memory note."},
	}})
	for _, want := range []string{"scope: `local-cli`", "soul_validation_status: `ok`", "soul_validation_errors: `0`", "soul_validation_warnings: `0`", "- none"} {
		if !strings.Contains(body, want) {
			t.Fatalf("validation report missing %q:\n%s", want, body)
		}
	}
}

func TestRenderSoulVerifyReportShowsTrustEnvelopeWithoutBodies(t *testing.T) {
	repoContext := RepoContext{Documents: []ContextDocument{
		{Path: ".gitclaw/SOUL.md", Body: "---\ndescription: Repo-local soul.\n---\nSOUL_VERIFY_BODY_TOKEN"},
		{Path: ".gitclaw/IDENTITY.md", Body: "Identity."},
		{Path: ".gitclaw/USER.md", Body: "USER_VERIFY_BODY_TOKEN"},
		{Path: ".gitclaw/TOOLS.md", Body: "Tools."},
		{Path: ".gitclaw/MEMORY.md", Body: "Memory."},
		{Path: ".gitclaw/HEARTBEAT.md", Body: "Heartbeat."},
		{Path: ".gitclaw/memory/2026-05-29.md", Body: "Memory note."},
	}}
	body := RenderSoulVerifyReport(repoContext)
	for _, want := range []string{
		"GitClaw Soul Verify Report",
		"scope: `local-cli`",
		"soul_verify_status: `ok`",
		"verification_scope: `repo-local-high-authority-context`",
		"context_documents: `7`",
		"repo_local_documents: `7`",
		"unknown_source_documents: `0`",
		"required_documents: `6`",
		"required_documents_present: `6`",
		"required_documents_missing: `0`",
		"soul_file_present: `true`",
		"soul_frontmatter_present: `true`",
		"soul_description_present: `true`",
		"identity_policy_files: `6`",
		"memory_notes: `1`",
		"files_with_hashes: `7`",
		"registry_verification: `not_configured`",
		"profile_export_verification: `not_configured`",
		"raw_bodies_included: `false`",
		"soul_validation_status: `ok`",
		"soul_validation_errors: `0`",
		"soul_validation_warnings: `0`",
		"soul_required_files_present: `6`",
		"soul_memory_notes: `1`",
		"### Trust Cards",
		"path=`.gitclaw/SOUL.md`",
		"category=`soul`",
		"source=`repo-local`",
		"required=`true`",
		"frontmatter=`true`",
		"description=`true`",
		"sha256_12=",
		"### Verification Findings",
		"code=`registry_verification_not_configured`",
		"code=`profile_export_verification_not_configured`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("soul verify report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SOUL_VERIFY_BODY_TOKEN", "USER_VERIFY_BODY_TOKEN"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("soul verify report leaked body token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSoulSearchReportFindsContextWithoutBodies(t *testing.T) {
	repoContext := RepoContext{Documents: []ContextDocument{
		{Path: ".gitclaw/SOUL.md", Body: "Repo-native operating guidance SOUL_SEARCH_BODY_TOKEN."},
		{Path: ".gitclaw/IDENTITY.md", Body: "Identity details."},
		{Path: ".gitclaw/USER.md", Body: "User operating preference USER_SEARCH_BODY_TOKEN."},
	}}
	body := RenderSoulSearchReport(Event{}, repoContext, "operating SOUL_SEARCH_QUERY_TOKEN", 2)
	for _, want := range []string{
		"GitClaw Soul Search Report",
		"scope: `local-cli`",
		"soul_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `2`",
		"files_scanned: `3`",
		"matched_files: `2`",
		"matched_lines: `2`",
		"results_returned: `2`",
		"raw_bodies_included: `false`",
		"path=`.gitclaw/SOUL.md`",
		"category=`soul`",
		"path=`.gitclaw/USER.md`",
		"category=`user`",
		"line_sha256_12=",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("soul search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SOUL_SEARCH_BODY_TOKEN", "USER_SEARCH_BODY_TOKEN", "SOUL_SEARCH_QUERY_TOKEN", "operating SOUL_SEARCH_QUERY_TOKEN"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("soul search report leaked body/query token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSoulInfoReportShowsOneContextFileWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul body with SOUL_INFO_BODY_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity body.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "User body.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tools body.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory body.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat body.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily body.\n")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "soul info soul"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 132,
			"title": "@gitclaw /soul info soul",
			"body": "Hidden soul info issue token: SOUL_INFO_ISSUE_SECRET.",
			"author_association": "OWNER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	body := RenderSoulReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Soul Info Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#132`",
		"requested_soul: `soul`",
		"normalized_soul_path: `.gitclaw/SOUL.md`",
		"soul_info_status: `ok`",
		"matched_soul_files: `1`",
		"run_mode: `read-only`",
		"raw_bodies_included: `false`",
		"soul_writes_allowed: `false`",
		"soul_validation_status: `ok`",
		"soul_validation_errors: `0`",
		"soul_validation_warnings: `0`",
		"category=`soul` path=`.gitclaw/SOUL.md` source=`repo-local` present=`true` required=`true` canonical=`true` latest=`false` loaded_for_this_turn=`true`",
		"sha256_12=",
		"at_context_limit=`false`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("soul info report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SOUL_INFO_BODY_SECRET", "SOUL_INFO_ISSUE_SECRET", "Soul body"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("soul info report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSoulEditPlanReportPlansHighAuthorityEditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul body with SOUL_EDIT_PLAN_BODY_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity body.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "User body.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tools body.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory body.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat body.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily body.\n")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "soul edit-plan soul"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 138,
			"title": "@gitclaw /soul edit-plan soul",
			"body": "Hidden soul edit plan issue token: SOUL_EDIT_PLAN_ISSUE_SECRET.",
			"author_association": "OWNER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	body := RenderSoulReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Soul Edit Plan Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#138`",
		"soul_edit_plan_status: `needs_review`",
		"target_sha256_12:",
		"target_allowed: `true`",
		"normalized_soul_path: `.gitclaw/SOUL.md`",
		"target_category: `soul`",
		"target_present: `true`",
		"target_required: `true`",
		"target_canonical: `true`",
		"target_loaded_for_this_turn: `true`",
		"matched_soul_files: `1`",
		"run_mode: `read-only`",
		"edit_operations_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"branch_creation_allowed: `false`",
		"commit_push_allowed: `false`",
		"model_self_modification_allowed: `false`",
		"manual_review_required: `true`",
		"llm_e2e_required_after_change: `true`",
		"raw_target_included: `false`",
		"raw_requested_change_included: `false`",
		"raw_bodies_included: `false`",
		"soul_writes_allowed: `false`",
		"soul_validation_status: `ok`",
		"### Current File Metadata",
		"category=`soul` path=`.gitclaw/SOUL.md`",
		"sha256_12=",
		"### Review Steps",
		"Run a live GitHub Models conversation E2E",
		"### Findings",
		"code=`manual_review_required`",
		"code=`repository_mutation_disabled`",
		"code=`model_self_modification_disabled`",
		"code=`high_authority_context_change`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("soul edit plan report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SOUL_EDIT_PLAN_BODY_SECRET", "SOUL_EDIT_PLAN_ISSUE_SECRET", "Soul body"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("soul edit plan report leaked %q:\n%s", leaked, body)
		}
	}
}
