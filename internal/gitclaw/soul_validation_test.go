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
