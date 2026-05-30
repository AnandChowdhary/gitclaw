package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderMemoryPromotePlanReportsMetadataWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Durable memory body MEMORY_PROMOTE_PLAN_MEMORY_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily note body MEMORY_PROMOTE_PLAN_NOTE_SECRET.\n")
	ctx, err := LoadRepoContext(root, []TranscriptMessage{
		{Role: "user", Body: "Remember this durable project convention MEMORY_PROMOTE_PLAN_TRANSCRIPT_SECRET."},
	})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 141,
			"title": "@gitclaw /memory promote-plan long-term",
			"body": "Hidden memory promote body token: MEMORY_PROMOTE_PLAN_BODY_SECRET.",
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
	report := RenderMemoryReport(ev, cfg, ctx, []TranscriptMessage{
		{Role: "user", Body: "Remember this durable project convention MEMORY_PROMOTE_PLAN_TRANSCRIPT_SECRET."},
	})
	for _, want := range []string{
		"GitClaw Memory Promote Plan Report",
		"Generated without a model call",
		"memory_promote_plan_status: `needs_review`",
		"requested_target_sha256_12:",
		"request_text_sha256_12:",
		"source_scope: `issue-thread-transcript-metadata`",
		"normalized_target_kind: `long-term`",
		"normalized_target_path: `.gitclaw/MEMORY.md`",
		"supported_target: `true`",
		"target_present: `true`",
		"memory_budget_bytes: `12000`",
		"dated_memory_notes: `1`",
		"latest_memory_note: `.gitclaw/memory/2026-05-29.md`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"memory_writes_allowed: `false`",
		"candidate_generation_included: `false`",
		"manual_review_required: `true`",
		"llm_e2e_required_after_change: `true`",
		"raw_candidate_memory_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"memory_validation_status: `ok`",
		"### Target Memory File",
		".gitclaw/MEMORY.md",
		"### Promotion Boundaries",
		"route user-profile or communication-style changes through `/soul edit-plan user`",
		"### Review Steps",
		"actual model call",
		"### Findings",
		"code=`durable_memory_is_prompt_authority`",
		"code=`repository_mutation_disabled`",
		"code=`body_suppression_enabled`",
		"code=`manual_review_required`",
		"code=`compact_memory_required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("memory promote plan missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"MEMORY_PROMOTE_PLAN_MEMORY_SECRET",
		"MEMORY_PROMOTE_PLAN_NOTE_SECRET",
		"MEMORY_PROMOTE_PLAN_TRANSCRIPT_SECRET",
		"MEMORY_PROMOTE_PLAN_BODY_SECRET",
		"Hidden memory promote body token",
		"Remember this durable project convention",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("memory promote plan leaked %q:\n%s", leaked, report)
		}
	}
}
