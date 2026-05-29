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

func TestRenderMemorySearchReportFindsMemoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Durable deployment preference with MEMORY_SEARCH_LONG_TERM_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Today we debugged deployment rollout notes with MEMORY_SEARCH_DATED_SECRET.\nAnother unrelated note.\n")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "memory search deployment rollout"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 114,
			"title": "@gitclaw /memory search deployment rollout MEMORY_SEARCH_QUERY_SECRET",
			"body": "Hidden memory search issue token: MEMORY_SEARCH_ISSUE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	report := RenderMemoryReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Search Report",
		"Generated without a model call",
		"memory_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `10`",
		"files_scanned: `2`",
		"matched_files: `2`",
		"matched_lines: `2`",
		"results_returned: `2`",
		"raw_bodies_included: `false`",
		"### Results",
		"path=`.gitclaw/MEMORY.md`",
		"path=`.gitclaw/memory/2026-05-29.md`",
		"line=`1`",
		"score=`",
		"matched_terms=`",
		"loaded_for_this_turn=`true`",
		"file_sha256_12=",
		"line_sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("memory search report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"MEMORY_SEARCH_LONG_TERM_SECRET", "MEMORY_SEARCH_DATED_SECRET", "MEMORY_SEARCH_QUERY_SECRET", "MEMORY_SEARCH_ISSUE_SECRET", "deployment rollout"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("memory search report leaked %q:\n%s", leaked, report)
		}
	}
}
