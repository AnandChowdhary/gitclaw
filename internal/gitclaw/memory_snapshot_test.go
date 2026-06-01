package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func writeMemorySnapshotFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "MEMORY_SNAPSHOT_LONG_TERM_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-27.md", "MEMORY_SNAPSHOT_OLDER_NOTE_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "MEMORY_SNAPSHOT_LATEST_NOTE_SECRET\n")
}

func TestRenderMemorySnapshotReportFingerprintsMemoryWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeMemorySnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "memory snapshot"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	body := RenderMemorySnapshotCLIReport(cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Snapshot Report",
		"scope: `local-cli`",
		"memory_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-memory-snapshot-v1`",
		"snapshot_scope: `repo-local-durable-memory`",
		"snapshot_sha256_12:",
		"snapshot_entries: `3`",
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
		"max_loaded_memory_notes: `3`",
		"first_memory_note: `.gitclaw/memory/2026-05-27.md`",
		"latest_memory_note: `.gitclaw/memory/2026-05-29.md`",
		"timeline_span_days: `2`",
		"largest_gap_days: `2`",
		"gaps_over_one_day: `1`",
		"raw_memory_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_session_bodies_included: `false`",
		"embedding_vectors_included: `false`",
		"external_provider_accessed: `false`",
		"memory_writes_allowed: `false`",
		"background_promotion_active: `false`",
		"llm_e2e_required_after_memory_snapshot_change: `true`",
		"memory_validation_status: `ok`",
		"memory_risk_status: `ok`",
		"memory_risk_findings: `0`",
		"### Snapshot Entries",
		"position=`1` kind=`long-term` path=`.gitclaw/MEMORY.md`",
		"role=`stable-summary` date=`long-term`",
		"position=`2` kind=`dated-note` path=`.gitclaw/memory/2026-05-27.md`",
		"gap_days_since_previous_note=`first`",
		"position=`3` kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md`",
		"role=`latest-daily-note` date=`2026-05-29`",
		"gap_days_since_previous_note=`2`",
		"sha256_12=",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"validation_findings=`0`",
		"### Snapshot Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"memory_write_gate=`disabled`",
		"external_provider_gate=`not_configured`",
		"session_search_gate=`github-issues-and-backups`",
		"background_promotion_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory snapshot report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_SNAPSHOT_LONG_TERM_SECRET", "MEMORY_SNAPSHOT_OLDER_NOTE_SECRET", "MEMORY_SNAPSHOT_LATEST_NOTE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory snapshot leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderMemoryReportRoutesSnapshotWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeMemorySnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "memory snapshot"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 181,
			Title:  "@gitclaw /memory snapshot",
			Body:   "Hidden memory snapshot issue token: MEMORY_SNAPSHOT_ROUTE_BODY_SECRET.",
		},
	}
	body := RenderMemoryReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Memory Snapshot Report",
		"repository: `owner/repo`",
		"issue: `#181`",
		"memory_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-memory-snapshot-v1`",
		"snapshot_sha256_12:",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_memory_snapshot_change: `true`",
		"raw_memory_bodies_included: `false`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory snapshot route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_SNAPSHOT_ROUTE_BODY_SECRET", "MEMORY_SNAPSHOT_LONG_TERM_SECRET", "MEMORY_SNAPSHOT_OLDER_NOTE_SECRET", "MEMORY_SNAPSHOT_LATEST_NOTE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory snapshot route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestMemorySnapshotCommandReportsCompositeFingerprintWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeMemorySnapshotFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"memory", "snapshot"}); err != nil {
			t.Fatalf("memory snapshot returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Memory Snapshot Report",
		"scope: `local-cli`",
		"memory_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-memory-snapshot-v1`",
		"snapshot_entries: `3`",
		"long_term_entries: `1`",
		"dated_note_entries: `2`",
		"raw_memory_bodies_included: `false`",
		"llm_e2e_required_after_memory_snapshot_change: `true`",
		"### Snapshot Entries",
		"kind=`long-term` path=`.gitclaw/MEMORY.md`",
		"kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md`",
		"### Snapshot Gates",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory snapshot output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"MEMORY_SNAPSHOT_LONG_TERM_SECRET", "MEMORY_SNAPSHOT_OLDER_NOTE_SECRET", "MEMORY_SNAPSHOT_LATEST_NOTE_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("memory snapshot output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleMemorySnapshotCommandPostsCompositeFingerprintWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeMemorySnapshotFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 182,
			"title": "@gitclaw /memory snapshot",
			"body": "Hidden memory snapshot body token: MEMORY_SNAPSHOT_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{182: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic memory snapshot command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Memory Snapshot Report",
		"Generated without a model call",
		"model=\"gitclaw/memory\"",
		"repository: `owner/repo`",
		"issue: `#182`",
		"memory_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-memory-snapshot-v1`",
		"snapshot_scope: `repo-local-durable-memory`",
		"snapshot_sha256_12:",
		"snapshot_entries: `3`",
		"raw_memory_bodies_included: `false`",
		"llm_e2e_required_after_memory_snapshot_change: `true`",
		"memory_validation_status: `ok`",
		"memory_risk_status: `ok`",
		"### Snapshot Entries",
		"kind=`long-term` path=`.gitclaw/MEMORY.md`",
		"kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md`",
		"### Snapshot Gates",
		"raw_body_gate=`hash_only`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("memory snapshot handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"MEMORY_SNAPSHOT_HANDLER_BODY_SECRET", "MEMORY_SNAPSHOT_LONG_TERM_SECRET", "MEMORY_SNAPSHOT_OLDER_NOTE_SECRET", "MEMORY_SNAPSHOT_LATEST_NOTE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("memory snapshot handler leaked %q:\n%s", leaked, body)
		}
	}
}
