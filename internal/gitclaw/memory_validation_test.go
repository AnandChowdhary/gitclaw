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

func TestRenderMemoryVerifyReportShowsTrustEnvelopeWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Durable facts only. MEMORY_VERIFY_LONG_TERM_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily note with MEMORY_VERIFY_DATED_SECRET.\n")
	ctx, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	report := RenderMemoryVerifyReport(Event{}, cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Verify Report",
		"scope: `local-cli`",
		"memory_verify_status: `ok`",
		"verification_scope: `repo-local-memory-provenance`",
		"memory_files: `2`",
		"repo_local_memory_files: `2`",
		"unknown_memory_files: `0`",
		"long_term_memory_present: `true`",
		"long_term_memory_loaded: `true`",
		"dated_memory_notes: `1`",
		"canonical_dated_memory_notes: `1`",
		"noncanonical_dated_memory_notes: `0`",
		"loaded_memory_notes: `1`",
		"omitted_memory_notes: `0`",
		"max_loaded_memory_notes: `3`",
		"latest_memory_note: `.gitclaw/memory/2026-05-29.md`",
		"memory_files_hashed: `2`",
		"memory_files_at_limit: `0`",
		"potential_secret_findings: `0`",
		"external_provider_verification: `not_configured`",
		"session_search_index_verification: `not_configured`",
		"background_promotion_verification: `not_configured`",
		"memory_writes_allowed: `false`",
		"raw_bodies_included: `false`",
		"memory_validation_status: `ok`",
		"memory_validation_errors: `0`",
		"memory_validation_warnings: `0`",
		"### Trust Cards",
		"kind=`long-term` path=`.gitclaw/MEMORY.md` source=`repo-local` present=`true` canonical=`true`",
		"kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md` source=`repo-local` present=`true` canonical=`true` latest=`true` loaded_for_this_turn=`true`",
		"sha256_12=",
		"### Verification Findings",
		"code=`external_memory_provider_verification_not_configured`",
		"code=`session_search_index_verification_not_configured`",
		"code=`background_promotion_verification_not_configured`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("memory verify report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"MEMORY_VERIFY_LONG_TERM_SECRET", "MEMORY_VERIFY_DATED_SECRET", "Durable facts only", "Daily note with"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("memory verify report leaked body token %q:\n%s", leaked, report)
		}
	}
}

func TestRenderMemoryCatalogReportShowsCompactCatalogWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_CATALOG_LONG_TERM_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-27.md", "MEMORY_CATALOG_OLDER_NOTE_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "MEMORY_CATALOG_LATEST_NOTE_SECRET\n")
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "memory catalog"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	body := RenderMemoryCatalogCLIReport(cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Catalog Report",
		"scope: `local-cli`",
		"memory_catalog_status: `ok`",
		"catalog_strategy: `compact-durable-memory-discovery`",
		"catalog_scope: `repo-local-memory-notes-session-search`",
		"memory_model: `repo-local-reviewed-markdown`",
		"hermes_memory_layers: `durable-memory, procedural-skills, session-search`",
		"durable_memory_layer: `git-backed-markdown`",
		"procedural_memory_layer: `skills-catalog-separate`",
		"session_search_layer: `github-issues-and-backups`",
		"cataloged_entries: `3`",
		"long_term_entries: `1`",
		"dated_note_entries: `2`",
		"memory_note_entries: `0`",
		"prompt_visible_entries: `3`",
		"loaded_memory_entries: `3`",
		"omitted_memory_entries: `0`",
		"memory_files: `3`",
		"long_term_memory_present: `true`",
		"long_term_memory_loaded: `true`",
		"dated_memory_notes: `2`",
		"canonical_dated_memory_notes: `2`",
		"noncanonical_dated_memory_notes: `0`",
		"loaded_memory_notes: `2`",
		"first_memory_note: `.gitclaw/memory/2026-05-27.md`",
		"latest_memory_note: `.gitclaw/memory/2026-05-29.md`",
		"timeline_span_days: `2`",
		"largest_gap_days: `2`",
		"raw_memory_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_session_bodies_included: `false`",
		"embedding_vectors_included: `false`",
		"external_provider_accessed: `false`",
		"memory_writes_allowed: `false`",
		"background_promotion_active: `false`",
		"llm_e2e_required_after_memory_catalog_change: `true`",
		"memory_validation_status: `ok`",
		"memory_risk_status: `ok`",
		"### Memory Catalog Entries",
		"position=`1` kind=`long-term` path=`.gitclaw/MEMORY.md` memory_layer=`durable-memory`",
		"role=`stable-summary` date=`long-term`",
		"load_mode=`prompt-visible`",
		"reason_codes=`below_context_limit, canonical, durable_memory, loaded, long_term, no_risk_findings, no_validation_findings, not_latest, present, prompt_visible, stable_summary`",
		"position=`3` kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md`",
		"role=`latest-daily-note` date=`2026-05-29`",
		"reason_codes=`below_context_limit, canonical, dated_note, durable_memory, latest, latest_daily_note, loaded, no_risk_findings, no_validation_findings, present, prompt_visible`",
		"### Catalog Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"memory_write_gate=`disabled`",
		"external_provider_gate=`not_configured`",
		"session_search_gate=`github-issues-and-backups`",
		"background_promotion_gate=`disabled`",
		"body_hash_gate=`sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory catalog report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_CATALOG_LONG_TERM_SECRET", "MEMORY_CATALOG_OLDER_NOTE_SECRET", "MEMORY_CATALOG_LATEST_NOTE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory catalog report leaked body token %q:\n%s", leaked, body)
		}
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

func TestRenderMemoryInfoReportShowsOneMemoryFileWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Long-term memory with MEMORY_INFO_LONG_TERM_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily note with MEMORY_INFO_DATED_SECRET.\n")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "memory info .gitclaw/memory/2026-05-29.md"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 131,
			"title": "@gitclaw /memory info .gitclaw/memory/2026-05-29.md",
			"body": "Hidden memory info issue token: MEMORY_INFO_ISSUE_SECRET.",
			"author_association": "OWNER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderMemoryReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Info Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#131`",
		"requested_memory: `.gitclaw/memory/2026-05-29.md`",
		"normalized_memory_path: `.gitclaw/memory/2026-05-29.md`",
		"memory_info_status: `ok`",
		"matched_memory_files: `1`",
		"memory_mode: `read-only`",
		"raw_bodies_included: `false`",
		"memory_writes_allowed: `false`",
		"memory_validation_status: `ok`",
		"kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md` source=`repo-local` present=`true` canonical=`true` latest=`true` loaded_for_this_turn=`true`",
		"sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("memory info report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"MEMORY_INFO_LONG_TERM_SECRET", "MEMORY_INFO_DATED_SECRET", "MEMORY_INFO_ISSUE_SECRET", "Daily note with"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("memory info report leaked %q:\n%s", leaked, report)
		}
	}
}
