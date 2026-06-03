package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleChannelBackupTimelineQueuesChronologyWithoutLLM(t *testing.T) {
	root := t.TempDir()
	backupRoot := filepath.Join(root, defaultBackupRoot)
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw old backup CHANNEL_BACKUP_TIMELINE_OLD_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_TIMELINE_OLD_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "CHANNEL_BACKUP_TIMELINE_OLD_TRANSCRIPT_SECRET"}},
		Comments:   []IssueBackupComment{{ID: 11, Body: "CHANNEL_BACKUP_TIMELINE_OLD_COMMENT_SECRET"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T14:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw middle backup CHANNEL_BACKUP_TIMELINE_MIDDLE_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_TIMELINE_MIDDLE_BODY_SECRET",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "CHANNEL_BACKUP_TIMELINE_MIDDLE_TRANSCRIPT_SECRET"},
			{Role: "assistant", Body: "CHANNEL_BACKUP_TIMELINE_MIDDLE_ASSISTANT_SECRET"},
		},
		Comments: []IssueBackupComment{{ID: 12, Body: "<!-- gitclaw:error -->\nCHANNEL_BACKUP_TIMELINE_MIDDLE_ERROR_SECRET"}},
	})
	writeBackupFixture(t, backupRoot, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T15:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 9,
			Title:  "@gitclaw latest backup CHANNEL_BACKUP_TIMELINE_LATEST_TITLE_SECRET",
			Body:   "CHANNEL_BACKUP_TIMELINE_LATEST_BODY_SECRET",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "CHANNEL_BACKUP_TIMELINE_LATEST_TRANSCRIPT_SECRET"},
			{Role: "assistant", Body: "CHANNEL_BACKUP_TIMELINE_LATEST_ASSISTANT_SECRET"},
		},
		Comments: []IssueBackupComment{{ID: 13, Body: "<!-- gitclaw:assistant-turn -->\nCHANNEL_BACKUP_TIMELINE_LATEST_COMMENT_SECRET"}},
	})
	if _, err := WriteBackupIndex(backupRoot, "owner/repo", time.Date(2026, 5, 29, 16, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-backup-timeline-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-backup-timeline-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-timeline-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90401,
			"body": "@gitclaw /channels backup-timeline --message-id backup-timeline-inbound-904 --notify-message-id backup-timeline-notify-904 --timeline-id Backup.Timeline.Secret.904 --limit 2\nDo not include this command hidden token in the receipt: CHANNEL_BACKUP_TIMELINE_COMMAND_MARKER.",
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
			Number: 904,
			Title:  "GitClaw telegram thread chat-backup-timeline-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{904: {{
			ID: 90400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-backup-timeline-123",
				MessageID: "backup-timeline-inbound-904",
				Author:    "telegram",
				Body:      "Original mirrored backup timeline command with CHANNEL_BACKUP_TIMELINE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{904: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup timeline action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("backup timeline action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[904]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="backup-timeline-notify-904"`,
		"GitClaw channel backup timeline",
		"Backup timeline status: ok",
		"Backup verify status: ok",
		"Backup branch: gitclaw-backups",
		"Backup fetch status: local",
		"Issue count: 3",
		"Limit: 2",
		"Timeline points: 2",
		"Timeline order: chronological",
		"Timeline window: latest_by_backup_generated_at",
		"First issue: #8",
		"Latest issue: #9",
		"Total span seconds: 3600",
		"Timeline id hash: ",
		"Timeline points:",
		"issue=#8 path=issues/000008.json",
		"gap_seconds_since_previous=0",
		"error_comments=1",
		"issue=#9 path=issues/000009.json",
		"gap_seconds_since_previous=3600",
		"assistant_turn_comments=1",
		"payload_sha256_12=",
		"title_sha256_12=",
		"Raw backup payloads, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw timeline ids are not included.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Backup branch write: not performed by this action.",
		"Restore: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("backup timeline notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range channelBackupTimelineLeakTokens() {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("backup timeline notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Timeline Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels backup-timeline`",
		"channel_backup_timeline_status: `queued`",
		"backup_timeline_status: `ok`",
		"backup_verify_status: `ok`",
		"backup_fetch_status: `local`",
		"backup_branch: `gitclaw-backups`",
		"timeline_mode: `gitclaw-backups-chronology-card`",
		"notification_target_issue: `#904`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"timeline_id_sha256_12: `",
		"timeline_id_auto: `false`",
		"limit: `2`",
		"limit_source: `flag`",
		"backup_root_sha256_12: `",
		"repo_backup_dir_sha256_12: `",
		"index_path_sha256_12: `",
		"readme_path_sha256_12: `",
		"backup_schema_version: `1`",
		"index_generated_at_sha256_12: `",
		"verification_failures: `0`",
		"issue_count: `3`",
		"timeline_points: `2`",
		"timeline_order: `chronological`",
		"timeline_window: `latest_by_backup_generated_at`",
		"first_issue_sha256_12: `",
		"latest_issue_sha256_12: `",
		"total_span_seconds: `3600`",
		"timeline_points_sha256_12: `",
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
		"raw_timeline_id_included: `false`",
		"raw_backup_root_included: `false`",
		"raw_backup_paths_included: `false`",
		"raw_timeline_points_included: `false`",
		"raw_backup_payloads_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_titles_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_transcript_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_backup_timeline_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup timeline receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range append(channelBackupTimelineLeakTokens(), "CHANNEL_BACKUP_TIMELINE_INGEST_MARKER", "CHANNEL_BACKUP_TIMELINE_COMMAND_MARKER", "chat-backup-timeline-123", "backup-timeline-inbound-904", "backup-timeline-notify-904", "Backup.Timeline.Secret.904", "owner__repo") {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup timeline receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-backup-timeline-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-backup-timeline-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90402,
			"body": "@gitclaw /channels archive-timeline --message-id backup-timeline-inbound-904 --notify-message-id backup-timeline-notify-904 --timeline-id Backup.Timeline.Secret.904 --limit 2\nDo not leak duplicate hidden token CHANNEL_BACKUP_TIMELINE_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[904]); got != 4 {
		t.Fatalf("duplicate backup timeline posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[904])
	}
	duplicateReceipt := github.CommentsByIssue[904][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels archive-timeline`",
		"channel_backup_timeline_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"backup_branch_write_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup timeline receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_TIMELINE_DUPLICATE_MARKER", "chat-backup-timeline-123", "backup-timeline-inbound-904", "backup-timeline-notify-904", "Backup.Timeline.Secret.904"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup timeline receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupTimelineActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel backup timeline"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel recovery-timeline --route team-demo --message-id source-1 --notify-message-id notify-1 --id Backup.Timeline.One --limit 7`,
		},
	}
	req, err := BuildChannelBackupTimelineActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupTimelineActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "recovery-timeline" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.TimelineID != "backup-timeline-one" || req.Options.Limit != 7 {
		t.Fatalf("unexpected channel backup timeline parsing: %#v", req)
	}
	if req.LimitSource != "flag" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoTimelineID {
		t.Fatalf("unexpected channel backup timeline defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.TimelineIDHash == "" {
		t.Fatalf("expected route timeline hashes: %#v", req)
	}
	if !IsChannelBackupTimelineActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel recovery-timeline alias to be recognized")
	}
}

func channelBackupTimelineLeakTokens() []string {
	return []string{
		"CHANNEL_BACKUP_TIMELINE_OLD_TITLE_SECRET",
		"CHANNEL_BACKUP_TIMELINE_OLD_BODY_SECRET",
		"CHANNEL_BACKUP_TIMELINE_OLD_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_TIMELINE_OLD_COMMENT_SECRET",
		"CHANNEL_BACKUP_TIMELINE_MIDDLE_TITLE_SECRET",
		"CHANNEL_BACKUP_TIMELINE_MIDDLE_BODY_SECRET",
		"CHANNEL_BACKUP_TIMELINE_MIDDLE_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_TIMELINE_MIDDLE_ASSISTANT_SECRET",
		"CHANNEL_BACKUP_TIMELINE_MIDDLE_ERROR_SECRET",
		"CHANNEL_BACKUP_TIMELINE_LATEST_TITLE_SECRET",
		"CHANNEL_BACKUP_TIMELINE_LATEST_BODY_SECRET",
		"CHANNEL_BACKUP_TIMELINE_LATEST_TRANSCRIPT_SECRET",
		"CHANNEL_BACKUP_TIMELINE_LATEST_ASSISTANT_SECRET",
		"CHANNEL_BACKUP_TIMELINE_LATEST_COMMENT_SECRET",
		"@gitclaw old backup",
		"@gitclaw middle backup",
		"@gitclaw latest backup",
	}
}
