package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleChannelBackupContinuityQueuesContinuityCardWithoutLLM(t *testing.T) {
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
			Title:  "@gitclaw old continuity CHANNEL_BACKUP_CONTINUITY_OLD_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_CONTINUITY_OLD_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "CHANNEL_BACKUP_CONTINUITY_OLD_TRANSCRIPT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 11, Body: "CHANNEL_BACKUP_CONTINUITY_OLD_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw middle continuity CHANNEL_BACKUP_CONTINUITY_MIDDLE_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_CONTINUITY_MIDDLE_BODY_SECRET",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{{Role: "assistant", Body: "CHANNEL_BACKUP_CONTINUITY_MIDDLE_TRANSCRIPT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 12, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_CONTINUITY_MIDDLE_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: now.Add(-1 * time.Hour).Format(time.RFC3339),
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 9,
			Title:  "@gitclaw latest continuity CHANNEL_BACKUP_CONTINUITY_LATEST_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_CONTINUITY_LATEST_BODY_SECRET",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{{Role: "assistant", Body: "CHANNEL_BACKUP_CONTINUITY_LATEST_TRANSCRIPT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 13, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_CONTINUITY_LATEST_COMMENT_SECRET"}},
	})
	if _, err := WriteBackupIndex(backupRoot, "owner/repo", now); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-continuity-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 907,
			"title": "GitClaw telegram thread chat-backup-continuity-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-continuity-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90701,
			"body": "@gitclaw /channels backup-continuity --message-id backup-continuity-inbound-907 --notify-message-id backup-continuity-notify-907 --continuity-id Backup.Continuity.Secret.907 --max-gap-hours 72\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_CONTINUITY_COMMAND_MARKER.",
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
			Number: 907,
			Title:  "GitClaw telegram thread chat-backup-continuity-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{907: {{
			ID: 90700,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-continuity-123",
				MessageID: "backup-continuity-inbound-907",
				Author:    "telegram",
				Body:      "Original mirrored backup continuity command with CHANNEL_BACKUP_CONTINUITY_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{907: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup continuity action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup continuity action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[907]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-continuity-notify-907"`,
		"GitClaw channel backup continuity",
		"Backup continuity status: ok",
		"Backup verify status: ok",
		"Continuity gate: pass",
		"Backup branch: gitclaw-backups",
		"Backup fetch status: local",
		"Issue count: 3",
		"Points scanned: 3",
		"Timeline order: chronological",
		"Max gap hours: 72",
		"Max gap seconds: 259200",
		"Gaps over max: 0",
		"Gaps reported: 0",
		"First issue: #7",
		"Latest issue: #9",
		"Total span seconds: 7200",
		"Longest gap seconds: 3600",
		"Longest gap from issue: #7",
		"Longest gap to issue: #8",
		"Continuity id hash: ",
		"Gaps over threshold:",
		"- none",
		"Raw backup payloads, backup paths, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw continuity ids are not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"Restore: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup continuity notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range channelBackupContinuityLeakTokens() {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup continuity notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Continuity Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-continuity`",
		"channel_backup_continuity_status: `queued`",
		"backup_continuity_status: `ok`",
		"backup_verify_status: `ok`",
		"continuity_gate: `pass`",
		"backup_fetch_status: `local`",
		"backup_branch: `gitclaw-backups`",
		"continuity_mode: `gitclaw-backups-continuity-card`",
		"notification_target_issue: `#907`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"continuity_id_sha256_12: `",
		"continuity_id_auto: `false`",
		"max_gap_hours: `72`",
		"max_gap_seconds: `259200`",
		"max_gap_source: `flag`",
		"backup_root_sha256_12: `",
		"repo_backup_dir_sha256_12: `",
		"index_path_sha256_12: `",
		"readme_path_sha256_12: `",
		"backup_schema_version: `1`",
		"index_generated_at_sha256_12: `",
		"verification_failures: `0`",
		"issue_count: `3`",
		"points_scanned: `3`",
		"timeline_order: `chronological`",
		"gaps_over_max: `0`",
		"gaps_reported: `0`",
		"first_issue_sha256_12: `",
		"first_generated_at_sha256_12: `",
		"latest_issue_sha256_12: `",
		"latest_generated_at_sha256_12: `",
		"total_span_seconds: `7200`",
		"longest_gap_seconds: `3600`",
		"longest_gap_from_issue_sha256_12: `",
		"longest_gap_to_issue_sha256_12: `",
		"longest_gap_from_generated_at_sha256_12: `",
		"longest_gap_to_generated_at_sha256_12: `",
		"continuity_gaps_sha256_12: `none`",
		"continuity_error_kind: `none`",
		"continuity_error_sha256_12: `none`",
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
		"raw_continuity_id_included: `false`",
		"raw_backup_root_included: `false`",
		"raw_backup_paths_included: `false`",
		"raw_continuity_gaps_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_titles_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_backup_continuity_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup continuity receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range append(channelBackupContinuityLeakTokens(), "CHANNEL_BACKUP_CONTINUITY_INGEST_MARKER", "CHANNEL_BACKUP_CONTINUITY_COMMAND_MARKER", "chat-backup-continuity-123", "backup-continuity-inbound-907", "backup-continuity-notify-907", "Backup.Continuity.Secret.907", "owner__repo") {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup continuity receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 907,
			"title": "GitClaw telegram thread chat-backup-continuity-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-continuity-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90702,
			"body": "@gitclaw /channels archive-gaps --message-id backup-continuity-inbound-907 --notify-message-id backup-continuity-notify-907 --continuity-id Backup.Continuity.Secret.907 --max-gap-hours 72\nDo not leak duplicate hidden token CHANNEL_BACKUP_CONTINUITY_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[907]); got != 4 {
		t.Fatalf("duplicate backup continuity posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[907])
	}
	duplicateReceipt := github.CommentsByIssue[907][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels archive-gaps`",
		"channel_backup_continuity_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup continuity receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_CONTINUITY_DUPLICATE_MARKER", "chat-backup-continuity-123", "backup-continuity-inbound-907", "backup-continuity-notify-907", "Backup.Continuity.Secret.907"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup continuity receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupContinuityActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup continuity"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel recovery-continuity --route team-demo --message-id source-1 --notify-message-id notify-1 --id Backup.Continuity.One --max-gap-hours 36`,
		},
	}
	req, err := BuildChannelBackupContinuityActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupContinuityActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-continuity" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ContinuityID != "backup-continuity-one" || req.Options.MaxGapHours != 36 {
		t.Fatalf("unexpected channel backup continuity parsing: %#v", req)
	}
	if req.MaxGapSource != "flag" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoContinuityID {
		t.Fatalf("unexpected channel backup continuity defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.ContinuityIDHash == "" {
		t.Fatalf("expected route continuity hashes: %#v", req)
	}
	if !IsChannelBackupContinuityActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel recovery-continuity alias to be recognized")
	}
}

func channelBackupContinuityLeakTokens() []string {
	return []string{
		"CHANNEL_BACKUP_CONTINUITY_OLD_TITLE_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_OLD_BODY_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_OLD_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_OLD_COMMENT_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_MIDDLE_TITLE_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_MIDDLE_BODY_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_MIDDLE_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_MIDDLE_COMMENT_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_LATEST_TITLE_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_LATEST_BODY_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_LATEST_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_CONTINUITY_LATEST_COMMENT_SECRET",
		"@gitclaw old continuity",
		"@gitclaw middle continuity",
		"@gitclaw latest continuity",
	}
}
