package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoulStatusQueuesSnapshotWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulStatusFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soul-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 890,
			"title": "GitClaw telegram thread chat-soul-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89001,
			"body": "@gitclaw /channels soul-status --message-id soul-status-inbound-890 --notify-message-id soul-status-notify-890 --status-id soul-status-secret-890\nDo not include this command hidden token in the receipt: CHANNEL_SOUL_STATUS_COMMAND_SECRET.",
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
			Number: 890,
			Title:  "GitClaw telegram thread chat-soul-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{890: {{
			ID: 89000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soul-status-123",
				MessageID: "soul-status-inbound-890",
				Author:    "telegram",
				Body:      "Original mirrored soul command with CHANNEL_SOUL_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{890: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("soul status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[890]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="soul-status-notify-890"`,
		"GitClaw channel soul status.",
		"Soul status: ok",
		"Snapshot version: gitclaw-soul-snapshot-v1",
		"Snapshot scope: repo-local-high-authority-context",
		"Snapshot hash: ",
		"Snapshot entries: 16",
		"Loaded entries: 6",
		"Required entries: 6/6 loaded",
		"Missing required entries: 0",
		"Optional loaded entries: 0",
		"Memory note entries: 0",
		"Prompt-visible entries: 6",
		"Validation: ok (0 errors, 0 warnings)",
		"Required files: 6/6 present",
		"Risk: ok (0 findings, high=0 warning=0 info=0)",
		"Registry contact: not performed by this action.",
		"Profile export: disabled.",
		"Soul writes: disabled.",
		"Repository mutation: not performed by this action.",
		"Raw soul bodies: not included.",
		"Raw identity bodies: not included.",
		"Raw user bodies: not included.",
		"Raw memory bodies: not included.",
		"Raw tool guidance bodies: not included.",
		"Raw heartbeat bodies: not included.",
		"Prompts, sessions, and backup payloads: not included.",
		"Model call: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soul-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_STATUS_INGEST_SECRET", "CHANNEL_SOUL_STATUS_COMMAND_SECRET", "CHANNEL_SOUL_STATUS_SOUL_SECRET", "CHANNEL_SOUL_STATUS_MEMORY_SECRET", "soul-status-secret-890"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("soul-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soul-status`",
		"channel_soul_status_status: `queued`",
		"soul_snapshot_mode: `provider-facing-soul-status`",
		"notification_target_issue: `#890`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"soul_status: `ok`",
		"snapshot_version: `gitclaw-soul-snapshot-v1`",
		"snapshot_scope: `repo-local-high-authority-context`",
		"snapshot_sha256_12: `",
		"snapshot_entries: `16`",
		"loaded_snapshot_entries: `6`",
		"required_snapshot_entries: `6`",
		"required_loaded_entries: `6`",
		"missing_required_entries: `0`",
		"optional_loaded_entries: `0`",
		"memory_note_entries: `0`",
		"prompt_visible_entries: `6`",
		"soul_validation_status: `ok`",
		"soul_validation_errors: `0`",
		"soul_validation_warnings: `0`",
		"soul_required_files: `6`",
		"soul_required_files_present: `6`",
		"soul_required_files_missing: `0`",
		"soul_noncanonical_memory_notes: `0`",
		"soul_risk_status: `ok`",
		"context_documents: `6`",
		"scanned_documents: `6`",
		"documents_with_risk_findings: `0`",
		"soul_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"registry_contact_allowed: `false`",
		"profile_export_allowed: `false`",
		"soul_writes_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_descriptions_included: `false`",
		"registry_verification: `not_configured`",
		"profile_export_verification: `not_configured`",
		"notification_body_sha256_12: `",
		"registry_contact_performed: `false`",
		"profile_export_performed: `false`",
		"soul_write_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_soul_status_id_included: `false`",
		"raw_soul_file_paths_included: `false`",
		"raw_soul_bodies_included: `false`",
		"raw_identity_bodies_included: `false`",
		"raw_user_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_tool_guidance_bodies_included: `false`",
		"raw_heartbeat_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_session_bodies_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_soul_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_STATUS_INGEST_SECRET", "CHANNEL_SOUL_STATUS_COMMAND_SECRET", "CHANNEL_SOUL_STATUS_SOUL_SECRET", "CHANNEL_SOUL_STATUS_MEMORY_SECRET", "chat-soul-status-123", "soul-status-inbound-890", "soul-status-notify-890", "soul-status-secret-890", ".gitclaw/SOUL.md", ".gitclaw/MEMORY.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 890,
			"title": "GitClaw telegram thread chat-soul-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89002,
			"body": "@gitclaw /channels authority-status --message-id soul-status-inbound-890 --notify-message-id soul-status-notify-890 --status-id soul-status-secret-890\nDo not leak duplicate token CHANNEL_SOUL_STATUS_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate soul status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[890]); got != 4 {
		t.Fatalf("duplicate soul status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[890])
	}
	duplicateReceipt := github.CommentsByIssue[890][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels authority-status`",
		"channel_soul_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"registry_contact_performed: `false`",
		"profile_export_performed: `false`",
		"soul_write_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_STATUS_DUPLICATE_SECRET", "chat-soul-status-123", "soul-status-inbound-890", "soul-status-notify-890", "soul-status-secret-890"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soul status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSoulStatusActionRequestParsesRouteAlias(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulStatusFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channels soul-status"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel soul status"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel soul-health --route team-demo --message-id source-1 --notify-message-id notify-1 --soul-status-id Soul.Status`,
		},
	}
	req, err := BuildChannelSoulStatusActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSoulStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "soul-health" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "soul-status" {
		t.Fatalf("unexpected channel soul status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.SoulStatus != "ok" || req.SnapshotEntries != 16 || req.LoadedSnapshotEntries != 6 || req.RequiredSnapshotEntries != 6 || req.RequiredLoadedEntries != 6 || req.SnapshotSHA == "" || req.ValidationStatus != "ok" || req.RiskStatus != "ok" {
		t.Fatalf("expected explicit route soul-status hashes and counts: %#v", req)
	}
}

func writeChannelSoulStatusFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul fixture. Hidden token CHANNEL_SOUL_STATUS_SOUL_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity fixture.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "User fixture.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "Tools fixture.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory fixture. Hidden token CHANNEL_SOUL_STATUS_MEMORY_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "Heartbeat fixture.\n")
	writeTestFile(t, root, "README.md", "fixture repo\n")
}
