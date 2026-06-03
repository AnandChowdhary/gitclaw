package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleChannelBackupFreshnessQueuesFreshnessCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, defaultBackupRoot)
	now := time.Now().UTC().Truncate(time.Second)
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: now.Add(-3 * time.Hour).Format(time.RFC3339),
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw old freshness CHANNEL_BACKUP_FRESHNESS_OLD_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_FRESHNESS_OLD_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "CHANNEL_BACKUP_FRESHNESS_OLD_TRANSCRIPT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 11, Body: "CHANNEL_BACKUP_FRESHNESS_OLD_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: now.Add(-1 * time.Hour).Format(time.RFC3339),
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 9,
			Title:  "@gitclaw latest freshness CHANNEL_BACKUP_FRESHNESS_LATEST_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_FRESHNESS_LATEST_BODY_SECRET",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "CHANNEL_BACKUP_FRESHNESS_LATEST_TRANSCRIPT_SECRET"},
			{Role: "assistant", Body: "CHANNEL_BACKUP_FRESHNESS_LATEST_ASSISTANT_SECRET"},
		},
		Comments: []IssueBackupComment{{ID: 13, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_FRESHNESS_LATEST_COMMENT_SECRET"}},
	})
	if _, err := WriteBackupIndex(backupRoot, "owner/repo", now); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-freshness-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-backup-freshness-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-freshness-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90501,
			"body": "@gitclaw /channels backup-freshness --message-id backup-freshness-inbound-905 --notify-message-id backup-freshness-notify-905 --freshness-id Backup.Freshness.Secret.905 --max-age-hours 72\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_FRESHNESS_COMMAND_MARKER.",
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
			Number: 905,
			Title:  "GitClaw telegram thread chat-backup-freshness-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{905: {{
			ID: 90500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-freshness-123",
				MessageID: "backup-freshness-inbound-905",
				Author:    "telegram",
				Body:      "Original mirrored backup freshness command with CHANNEL_BACKUP_FRESHNESS_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{905: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup freshness action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup freshness action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[905]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-freshness-notify-905"`,
		"GitClaw channel backup freshness",
		"Backup freshness status: ok",
		"Backup verify status: ok",
		"Freshness gate: pass",
		"Backup branch: gitclaw-backups",
		"Backup fetch status: local",
		"Issue count: 2",
		"Max age hours: 72",
		"Max age seconds: 259200",
		"Latest issue: #9",
		"Latest backup generated at: ",
		"Latest age seconds: ",
		"Clock skew seconds: 0",
		"Latest payload bytes: ",
		"Latest payload sha256_12: ",
		"Latest event hash: ",
		"Latest issue title hash: ",
		"Freshness id hash: ",
		"Raw backup payloads, backup paths, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw freshness ids are not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"Restore: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup freshness notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range channelBackupFreshnessLeakTokens() {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup freshness notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Freshness Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-freshness`",
		"channel_backup_freshness_status: `queued`",
		"backup_freshness_status: `ok`",
		"backup_verify_status: `ok`",
		"freshness_gate: `pass`",
		"backup_fetch_status: `local`",
		"backup_branch: `gitclaw-backups`",
		"freshness_mode: `gitclaw-backups-freshness-card`",
		"notification_target_issue: `#905`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"freshness_id_sha256_12: `",
		"freshness_id_auto: `false`",
		"max_age_hours: `72`",
		"max_age_seconds: `259200`",
		"max_age_source: `flag`",
		"backup_root_sha256_12: `",
		"repo_backup_dir_sha256_12: `",
		"index_path_sha256_12: `",
		"readme_path_sha256_12: `",
		"backup_schema_version: `1`",
		"index_generated_at_sha256_12: `",
		"as_of_sha256_12: `",
		"verification_failures: `0`",
		"issue_count: `2`",
		"latest_issue_sha256_12: `",
		"latest_generated_at_sha256_12: `",
		"latest_age_seconds: `",
		"clock_skew_seconds: `0`",
		"latest_payload_bytes: `",
		"latest_payload_sha256_12: `",
		"latest_event_name_sha256_12: `",
		"latest_issue_title_sha256_12: `",
		"freshness_error_kind: `none`",
		"freshness_error_sha256_12: `none`",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"backup_branch_fetch_performed: `false`",
		"raw_backup_payloads_read: `true`",
		"restore_performed: `false`",
		"backup_branch_write_performed: `false`",
		"github_api_replay_performed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_freshness_id_included: `false`",
		"raw_backup_root_included: `false`",
		"raw_backup_paths_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_titles_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_backup_freshness_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup freshness receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range append(channelBackupFreshnessLeakTokens(), "CHANNEL_BACKUP_FRESHNESS_INGEST_MARKER", "CHANNEL_BACKUP_FRESHNESS_COMMAND_MARKER", "chat-backup-freshness-123", "backup-freshness-inbound-905", "backup-freshness-notify-905", "Backup.Freshness.Secret.905", "owner__repo") {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup freshness receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 905,
			"title": "GitClaw telegram thread chat-backup-freshness-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-freshness-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90502,
			"body": "@gitclaw /channels archive-health --message-id backup-freshness-inbound-905 --notify-message-id backup-freshness-notify-905 --freshness-id Backup.Freshness.Secret.905 --max-age-hours 72\nDo not leak duplicate hidden token CHANNEL_BACKUP_FRESHNESS_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[905]); got != 4 {
		t.Fatalf("duplicate backup freshness posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[905])
	}
	duplicateReceipt := github.CommentsByIssue[905][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels archive-health`",
		"channel_backup_freshness_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup freshness receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_FRESHNESS_DUPLICATE_MARKER", "chat-backup-freshness-123", "backup-freshness-inbound-905", "backup-freshness-notify-905", "Backup.Freshness.Secret.905"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup freshness receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupFreshnessActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup freshness"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel backup-staleness --route team-demo --message-id source-1 --notify-message-id notify-1 --id Backup.Freshness.One --max-age-hours 36`,
		},
	}
	req, err := BuildChannelBackupFreshnessActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupFreshnessActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "backup-staleness" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.FreshnessID != "backup-freshness-one" || req.Options.MaxAgeHours != 36 {
		t.Fatalf("unexpected channel backup freshness parsing: %#v", req)
	}
	if req.MaxAgeSource != "flag" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoFreshnessID {
		t.Fatalf("unexpected channel backup freshness defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.FreshnessIDHash == "" {
		t.Fatalf("expected route freshness hashes: %#v", req)
	}
	if !IsChannelBackupFreshnessActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel backup-staleness alias to be recognized")
	}
}

func channelBackupFreshnessLeakTokens() []string {
	return []string{
		"CHANNEL_BACKUP_FRESHNESS_OLD_TITLE_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_OLD_BODY_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_OLD_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_OLD_COMMENT_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_LATEST_TITLE_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_LATEST_BODY_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_LATEST_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_LATEST_ASSISTANT_SECRET",
		"CHANNEL_BACKUP_FRESHNESS_LATEST_COMMENT_SECRET",
		"@gitclaw old freshness",
		"@gitclaw latest freshness",
	}
}
