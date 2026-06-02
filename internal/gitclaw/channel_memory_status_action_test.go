package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMemoryStatusQueuesSnapshotWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelMemoryStatusFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-memory-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 891,
			"title": "GitClaw telegram thread chat-memory-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-memory-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89101,
			"body": "@gitclaw /channels memory-status --message-id memory-status-inbound-891 --notify-message-id memory-status-notify-891 --status-id memory-status-secret-891\nDo not include this command hidden token in the receipt: CHANNEL_MEMORY_STATUS_COMMAND_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 891,
			Title:  "GitClaw telegram thread chat-memory-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{891: {{
			ID: 89100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-memory-status-123",
				MessageID: "memory-status-inbound-891",
				Author:    "telegram",
				Body:      "Original mirrored memory command with CHANNEL_MEMORY_STATUS_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{891: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel memory status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("memory status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[891]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="memory-status-notify-891"`,
		"GitClaw channel memory status.",
		"Memory status: ok",
		"Snapshot version: gitclaw-memory-snapshot-v1",
		"Snapshot scope: repo-local-durable-memory",
		"Snapshot hash: ",
		"Snapshot entries: 2",
		"Memory files: 2",
		"Long-term memory: present=true loaded=true",
		"Dated notes: 1 canonical=1 noncanonical=0 loaded=1 max_loaded=3",
		"Prompt-visible entries: 2",
		"Loaded memory entries: 2",
		"Omitted memory entries: 0",
		"Timeline: span_days=0 largest_gap_days=0 gaps_over_one_day=0",
		"Validation: ok (0 errors, 0 warnings, secret_findings=0)",
		"Risk: ok (0 findings, high=0 warning=0 info=0)",
		"Memory writes: disabled.",
		"Background promotion: disabled; git review required.",
		"External provider access: not configured and not performed.",
		"Embedding vectors: not included.",
		"Raw memory bodies: not included.",
		"Raw issue bodies: not included.",
		"Raw comment bodies: not included.",
		"Raw prompt bodies: not included.",
		"Raw session bodies: not included.",
		"Backup payloads: not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("memory-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_STATUS_INGEST_MARKER", "CHANNEL_MEMORY_STATUS_COMMAND_MARKER", "CHANNEL_MEMORY_STATUS_LONG_TERM_MARKER", "CHANNEL_MEMORY_STATUS_NOTE_MARKER", "memory-status-secret-891", ".gitclaw/MEMORY.md", ".gitclaw/memory/2026-05-29.md"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("memory-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Memory Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels memory-status`",
		"channel_memory_status_status: `queued`",
		"memory_snapshot_mode: `provider-facing-memory-status`",
		"notification_target_issue: `#891`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"memory_status: `ok`",
		"snapshot_version: `gitclaw-memory-snapshot-v1`",
		"snapshot_scope: `repo-local-durable-memory`",
		"snapshot_sha256_12: `",
		"snapshot_entries: `2`",
		"long_term_entries: `1`",
		"dated_note_entries: `1`",
		"memory_note_entries: `0`",
		"prompt_visible_entries: `2`",
		"loaded_memory_entries: `2`",
		"omitted_memory_entries: `0`",
		"memory_files: `2`",
		"long_term_memory_present: `true`",
		"long_term_memory_loaded: `true`",
		"dated_memory_notes: `1`",
		"canonical_dated_memory_notes: `1`",
		"noncanonical_dated_memory_notes: `0`",
		"loaded_memory_notes: `1`",
		"max_loaded_memory_notes: `3`",
		"first_memory_note_sha256_12: `",
		"latest_memory_note_sha256_12: `",
		"timeline_span_days: `0`",
		"largest_gap_days: `0`",
		"gaps_over_one_day: `0`",
		"memory_validation_status: `ok`",
		"memory_validation_errors: `0`",
		"memory_validation_warnings: `0`",
		"empty_memory_files: `0`",
		"memory_files_at_limit: `0`",
		"potential_secret_findings: `0`",
		"memory_risk_status: `ok`",
		"scanned_memory_files: `2`",
		"memory_files_with_risk_findings: `0`",
		"memory_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"memory_writes_allowed: `false`",
		"external_provider_accessed: `false`",
		"external_provider_verification: `not_configured`",
		"background_promotion_active: `false`",
		"background_promotion_review: `git_review_required`",
		"raw_memory_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_session_bodies_included: `false`",
		"embedding_vectors_included: `false`",
		"notification_body_sha256_12: `",
		"memory_write_performed: `false`",
		"background_promotion_performed: `false`",
		"external_provider_access_performed: `false`",
		"embedding_vector_access_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_memory_status_id_included: `false`",
		"raw_memory_file_paths_included: `false`",
		"raw_memory_bodies_included_in_notification: `false`",
		"raw_timeline_paths_included: `false`",
		"raw_issue_bodies_included_in_notification: `false`",
		"raw_comment_bodies_included_in_notification: `false`",
		"raw_prompt_bodies_included_in_notification: `false`",
		"raw_session_bodies_included_in_notification: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_memory_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel memory status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_STATUS_INGEST_MARKER", "CHANNEL_MEMORY_STATUS_COMMAND_MARKER", "CHANNEL_MEMORY_STATUS_LONG_TERM_MARKER", "CHANNEL_MEMORY_STATUS_NOTE_MARKER", "chat-memory-status-123", "memory-status-inbound-891", "memory-status-notify-891", "memory-status-secret-891", ".gitclaw/MEMORY.md", ".gitclaw/memory/2026-05-29.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel memory status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 891,
			"title": "GitClaw telegram thread chat-memory-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-memory-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89102,
			"body": "@gitclaw /channels recall-status --message-id memory-status-inbound-891 --notify-message-id memory-status-notify-891 --status-id memory-status-secret-891\nDo not leak duplicate token CHANNEL_MEMORY_STATUS_DUPLICATE_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate memory status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[891]); got != 4 {
		t.Fatalf("duplicate memory status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[891])
	}
	duplicateReceipt := github.CommentsByIssue[891][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels recall-status`",
		"channel_memory_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"memory_write_performed: `false`",
		"background_promotion_performed: `false`",
		"external_provider_access_performed: `false`",
		"embedding_vector_access_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate memory status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_STATUS_DUPLICATE_MARKER", "chat-memory-status-123", "memory-status-inbound-891", "memory-status-notify-891", "memory-status-secret-891", ".gitclaw/MEMORY.md", ".gitclaw/memory/2026-05-29.md"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate memory status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelMemoryStatusActionRequestParsesRouteAlias(t *testing.T) {
	root := t.TempDir()
	writeChannelMemoryStatusFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channels memory-status"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel memory status"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel memory-health --route team-demo --message-id source-1 --notify-message-id notify-1 --memory-status-id Memory.Status`,
		},
	}
	req, err := BuildChannelMemoryStatusActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelMemoryStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "memory-health" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "memory-status" {
		t.Fatalf("unexpected channel memory status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.MemoryStatus != "ok" || req.SnapshotEntries != 2 || req.LongTermEntries != 1 || req.DatedNoteEntries != 1 || req.MemoryFiles != 2 || req.SnapshotSHA == "" || req.ValidationStatus != "ok" || req.RiskStatus != "ok" || req.FirstMemoryNoteHash == "" || req.LatestMemoryNoteHash == "" {
		t.Fatalf("expected explicit route memory-status hashes and counts: %#v", req)
	}
}

func writeChannelMemoryStatusFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Long-term memory fixture. Marker CHANNEL_MEMORY_STATUS_LONG_TERM_MARKER.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Dated memory note fixture. Marker CHANNEL_MEMORY_STATUS_NOTE_MARKER.\n")
	writeTestFile(t, root, "README.md", "fixture repo\n")
}
