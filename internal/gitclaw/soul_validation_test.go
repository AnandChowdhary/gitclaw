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
	for _, want := range []string{"soul_validation_status: `ok`", "soul_validation_errors: `0`", "soul_validation_warnings: `0`", "- none"} {
		if !strings.Contains(body, want) {
			t.Fatalf("validation report missing %q:\n%s", want, body)
		}
	}
}
