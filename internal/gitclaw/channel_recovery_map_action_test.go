package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelRecoveryMapQueuesSequenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, ".gitclaw", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "README.md"), []byte("Recovery map docs secret CHANNEL_RECOVERY_MAP_DOCS_SECRET.\nDo not print this body.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile backup README returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-recovery-map-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 889,
			"title": "GitClaw telegram thread chat-recovery-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-recovery-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88901,
			"body": "@gitclaw /channels recovery-map incident --message-id recovery-map-inbound-889 --notify-message-id recovery-map-notify-889 --map-id recovery-map-secret-889\nNote: Keep restore review explicit.\nDo not include this command hidden token in the receipt: CHANNEL_RECOVERY_MAP_COMMAND_SECRET.",
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
			Number: 889,
			Title:  "GitClaw telegram thread chat-recovery-map-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{889: {{
			ID: 88900,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-recovery-map-123",
				MessageID: "recovery-map-inbound-889",
				Author:    "telegram",
				Body:      "Original mirrored recovery map command with CHANNEL_RECOVERY_MAP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{889: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel recovery map action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("recovery map should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[889]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="recovery-map-notify-889"`,
		"GitClaw channel recovery map.",
		"Scope: incident",
		"Backup branch: gitclaw-backups",
		"Backup root: .gitclaw/backups",
		"Schema version: 1",
		"Backup docs: present",
		"Catalog commands: 18",
		"Fetched-branch inspection commands: 17",
		"Recovery sequence:",
		"`/channels backup --message-id <id> --notify-message-id <id>`",
		"`/channels backup-search <query> --message-id <id> --notify-message-id <id>`",
		"`/channels backup-info <issue> --message-id <id> --notify-message-id <id>`",
		"`/channels rehearse-backup --issue <issue> --id <id> --message-id <id>`",
		"`/channels restore-request --issue <issue> --id <id> --message-id <id>`",
		"Note: Keep restore review explicit.",
		"Note hash: ",
		"Recovery map hash: ",
		"Recovery step hash: ",
		"Map source: current GitHub Actions checkout backup catalog.",
		"Backup branch fetch: not performed by this action.",
		"Raw backup payloads: not read by this action.",
		"Restore: not performed by this action.",
		"Rehearsal issue creation: not performed by this action.",
		"Restore-request issue creation: not performed by this action.",
		"GitHub API replay: not performed by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("recovery-map notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_RECOVERY_MAP_INGEST_SECRET", "CHANNEL_RECOVERY_MAP_COMMAND_SECRET", "CHANNEL_RECOVERY_MAP_DOCS_SECRET", "recovery-map-secret-889"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("recovery-map notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Recovery Map Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels recovery-map`",
		"channel_recovery_map_status: `queued`",
		"recovery_map_mode: `provider-facing-recovery-sequence`",
		"notification_target_issue: `#889`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"recovery_map_id_sha256_12: `",
		"recovery_map_id_auto: `false`",
		"recovery_scope_sha256_12: `",
		"recovery_scope_bytes: `8`",
		"recovery_note_sha256_12: `",
		"recovery_note_bytes: `29`",
		"recovery_note_lines: `1`",
		"recovery_note_source: `trailing-note`",
		"recovery_step_count: `5`",
		"recovery_step_sha256_12: `",
		"recovery_snapshot_sha256_12: `",
		"backup_branch: `gitclaw-backups`",
		"backup_root: `.gitclaw/backups`",
		"backup_schema_version: `1`",
		"backup_docs_present: `true`",
		"catalog_entries: `18`",
		"fetched_branch_required_commands: `17`",
		"metadata_only_commands: `1`",
		"raw_recovery_commands: `1`",
		"provider_visible_backup_actions: `4`",
		"notification_body_sha256_12: `",
		"backup_branch_fetch_performed: `false`",
		"raw_backup_payloads_read: `false`",
		"restore_performed: `false`",
		"backup_branch_write_performed: `false`",
		"github_api_replay_performed: `false`",
		"rehearsal_issue_created: `false`",
		"restore_request_issue_created: `false`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_recovery_map_id_included: `false`",
		"raw_recovery_scope_included: `false`",
		"raw_recovery_note_included: `false`",
		"raw_recovery_steps_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_recovery_map_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel recovery map receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RECOVERY_MAP_INGEST_SECRET", "CHANNEL_RECOVERY_MAP_COMMAND_SECRET", "CHANNEL_RECOVERY_MAP_DOCS_SECRET", "chat-recovery-map-123", "recovery-map-inbound-889", "recovery-map-notify-889", "recovery-map-secret-889", "Keep restore review explicit.", "/channels backup-search <query>", "owner__repo"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel recovery map receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 889,
			"title": "GitClaw telegram thread chat-recovery-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-recovery-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88902,
			"body": "@gitclaw /channels restore-map incident --message-id recovery-map-inbound-889 --notify-message-id recovery-map-notify-889 --map-id recovery-map-secret-889\nNote: Keep restore review explicit.\nDo not leak duplicate token CHANNEL_RECOVERY_MAP_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate recovery map created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[889]); got != 4 {
		t.Fatalf("duplicate recovery map posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[889])
	}
	duplicateReceipt := github.CommentsByIssue[889][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels restore-map`",
		"channel_recovery_map_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"backup_branch_fetch_performed: `false`",
		"raw_backup_payloads_read: `false`",
		"restore_performed: `false`",
		"rehearsal_issue_created: `false`",
		"restore_request_issue_created: `false`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate recovery map receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RECOVERY_MAP_DUPLICATE_SECRET", "chat-recovery-map-123", "recovery-map-inbound-889", "recovery-map-notify-889", "recovery-map-secret-889", "Keep restore review explicit.", "/channels backup-search <query>"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate recovery map receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelRecoveryMapActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, ".gitclaw", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel recovery map"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel restore-flow --route team-demo --scope repository --message-id source-1 --notify-message-id notify-1 --map-id Recovery.Map.One`,
		},
	}
	req, err := BuildChannelRecoveryMapActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelRecoveryMapActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "restore-flow" || req.Options.Route != "team-demo" || req.Options.Scope != "repo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MapID != "recovery-map-one" || req.StepCount != 5 {
		t.Fatalf("unexpected channel recovery map parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMapID || req.RequestedRouteHash == "" || req.MapIDHash == "" || req.ScopeSHA == "" || req.StepSHA == "" || req.SnapshotSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route recovery-map hashes: %#v", req)
	}
}

func TestBuildChannelRecoveryMapActionRequestParsesPositionalRouteAndScope(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels backup-path team-demo incident-response --message-id source-2 --notify-message-id notify-2 --map-id Recovery.Map.Two",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelRecoveryMapActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRecoveryMapActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Scope != "incident" || req.StepCount != 5 || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel recovery map parsing: %#v", req)
	}
}
