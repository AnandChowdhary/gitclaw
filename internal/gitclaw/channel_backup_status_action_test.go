package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelBackupStatusQueuesSnapshotWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, ".gitclaw", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "README.md"), []byte("Backup docs secret CHANNEL_BACKUP_STATUS_DOCS_SECRET.\nDo not print this body.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile backup README returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-status-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 888,
			"title": "GitClaw telegram thread chat-backup-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88801,
			"body": "@gitclaw /channels backup --message-id backup-status-inbound-888 --notify-message-id backup-status-notify-888 --status-id backup-status-secret-888\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_STATUS_COMMAND_SECRET.",
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
			Number: 888,
			Title:  "GitClaw telegram thread chat-backup-status-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{888: {{
			ID: 88800,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-status-123",
				MessageID: "backup-status-inbound-888",
				Author:    "telegram",
				Body:      "Original mirrored backup command with CHANNEL_BACKUP_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{888: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[888]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-status-notify-888"`,
		"GitClaw channel backup status.",
		"Backup branch: gitclaw-backups",
		"Backup root: .gitclaw/backups",
		"Schema version: 1",
		"Catalog commands: 18",
		"Fetched-branch inspection commands: 17",
		"Metadata-only commands: 1",
		"Raw recovery commands: 1",
		"Channel backup actions: status, rehearse-backup, restore-request",
		"Backup docs: present",
		"Latest backup freshness: requires fetched backup branch",
		"Raw backup payloads: not read by this action.",
		"Backup branch fetch: not performed by this action.",
		"Restore: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"GitHub API replay: not performed by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_STATUS_INGEST_SECRET", "CHANNEL_BACKUP_STATUS_COMMAND_SECRET", "CHANNEL_BACKUP_STATUS_DOCS_SECRET", "backup-status-secret-888"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup`",
		"channel_backup_status_status: `queued`",
		"backup_snapshot_mode: `provider-facing-backup-status`",
		"notification_target_issue: `#888`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"backup_branch: `gitclaw-backups`",
		"backup_root: `.gitclaw/backups`",
		"backup_schema_version: `1`",
		"repo_backup_dir_sha256_12: `",
		"index_path_sha256_12: `",
		"readme_path_sha256_12: `",
		"backup_docs_path_sha256_12: `",
		"backup_docs_present: `true`",
		"backup_docs_bytes: `",
		"backup_docs_lines: `2`",
		"backup_docs_sha256_12: `",
		"catalog_entries: `18`",
		"fetched_branch_required_commands: `17`",
		"metadata_only_commands: `1`",
		"raw_recovery_commands: `1`",
		"provider_visible_backup_actions: `3`",
		"catalog_command_names_sha256_12: `",
		"backup_status_snapshot_sha256_12: `",
		"notification_body_sha256_12: `",
		"backup_branch_fetch_performed: `false`",
		"raw_backup_payloads_read: `false`",
		"restore_performed: `false`",
		"backup_branch_write_performed: `false`",
		"github_api_replay_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_backup_status_id_included: `false`",
		"raw_repo_backup_dir_included: `false`",
		"raw_index_path_included: `false`",
		"raw_readme_path_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_backup_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_STATUS_INGEST_SECRET", "CHANNEL_BACKUP_STATUS_COMMAND_SECRET", "CHANNEL_BACKUP_STATUS_DOCS_SECRET", "chat-backup-status-123", "backup-status-inbound-888", "backup-status-notify-888", "backup-status-secret-888", "owner__repo"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 888,
			"title": "GitClaw telegram thread chat-backup-status-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-status-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88802,
			"body": "@gitclaw /channels backup-status --message-id backup-status-inbound-888 --notify-message-id backup-status-notify-888 --status-id backup-status-secret-888\nDo not leak duplicate token CHANNEL_BACKUP_STATUS_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate backup status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[888]); got != 4 {
		t.Fatalf("duplicate backup status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[888])
	}
	duplicateReceipt := github.CommentsByIssue[888][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels backup-status`",
		"channel_backup_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"backup_branch_fetch_performed: `false`",
		"raw_backup_payloads_read: `false`",
		"restore_performed: `false`",
		"backup_branch_write_performed: `false`",
		"github_api_replay_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_STATUS_DUPLICATE_SECRET", "chat-backup-status-123", "backup-status-inbound-888", "backup-status-notify-888", "backup-status-secret-888"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupStatusActionRequestParsesRouteAlias(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, ".gitclaw", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "README.md"), []byte("backup status route docs\n"), 0o644); err != nil {
		t.Fatalf("WriteFile backup README returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup status"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel recovery-health --route team-demo --message-id source-1 --notify-message-id notify-1 --backup-status-id Backup.Status`,
		},
	}
	req, err := BuildChannelBackupStatusActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelBackupStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-health" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "backup-status" {
		t.Fatalf("unexpected channel backup status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.CatalogEntries != 18 || req.FetchedBranchRequiredCommands != 17 || req.MetadataOnlyCommands != 1 || req.RawRecoveryCommands != 1 || req.ProviderVisibleActions != 3 || req.CatalogCommandNamesHash == "" || req.BackupStatusSnapshotHash == "" {
		t.Fatalf("expected explicit route backup-status hashes and counts: %#v", req)
	}
}
