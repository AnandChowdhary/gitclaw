package gitclaw

import (
	"strings"
	"testing"
)

func TestValidateMemoryReportsHygieneWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Durable facts only.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily note with api_key=abcdefghijklmnop\n")
	writeTestFile(t, root, ".gitclaw/memory/scratch.md", "")

	ctx, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	validation := ValidateMemory(root, ctx)
	if validation.Status != "error" || validation.Errors != 2 || validation.Warnings != 1 || validation.EmptyMemoryFiles != 1 || validation.PotentialSecretFindings != 1 || validation.NoncanonicalDatedNotes != 1 {
		t.Fatalf("unexpected validation report: %#v", validation)
	}
	report := RenderMemoryValidationReport(Event{}, Config{Workdir: root}, ctx)
	for _, want := range []string{
		"GitClaw Memory Validate Report",
		"memory_validation_status: `error`",
		"memory_validation_errors: `2`",
		"memory_validation_warnings: `1`",
		"long_term_memory_present: `true`",
		"dated_memory_notes: `2`",
		"canonical_dated_memory_notes: `1`",
		"noncanonical_dated_memory_notes: `1`",
		"empty_memory_files: `1`",
		"potential_secret_findings: `1`",
		"code=`empty_memory_file`",
		"code=`potential_secret`",
		"code=`noncanonical_memory_note`",
		".gitclaw/memory/2026-05-29.md",
		".gitclaw/memory/scratch.md",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("memory validation report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"api_key=abcdefghijklmnop", "Durable facts only"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("memory validation leaked body token %q:\n%s", leaked, report)
		}
	}
}

func TestValidateMemoryAcceptsCanonicalMemoryShape(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Durable facts only.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "A dated working note.\n")
	ctx, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	validation := ValidateMemory(root, ctx)
	if validation.Status != "ok" || validation.Errors != 0 || validation.Warnings != 0 {
		t.Fatalf("unexpected validation report: %#v", validation)
	}
}
