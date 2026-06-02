package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelBackupRestoreRequestCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-backup-restore-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 489,
			"title": "GitClaw telegram thread channel-backup-restore-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-backup-restore-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48901,
			"body": "@gitclaw /channels restore-request --id channel-backup-restore --message-id inbound-489 --notify-message-id notify-489\nPlease review this channel-origin restore request.\nCHANNEL_BACKUP_RESTORE_SOURCE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 489,
			Title:  "GitClaw telegram thread channel-backup-restore-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{489: {{
			ID: 48900,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-backup-restore-thread-123",
				MessageID: "inbound-489",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BACKUP_RESTORE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{489: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel backup restore request action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one backup restore request issue: %#v", len(github.Issues), github.Issues)
	}
	restoreIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:backup-restore-request-issue",
		`id="channel-backup-restore"`,
		`backup_issue="489"`,
		"GitClaw backup restore request issue",
		"request_id: channel-backup-restore",
		"backup_issue: #489",
		"target_repository: owner/repo",
		"backup_branch: gitclaw-backups",
		"backup_root: .gitclaw/backups",
		"issue_backup_path: .gitclaw/backups/owner__repo/issues/000489.json",
		"source_issue: #489",
		"source_kind: channel_comment",
		"approval_required: true",
		"restore_pr_required: true",
		"restore_mode: dry-run-first",
		"repository_mutation_allowed: false",
		"backup_branch_write_allowed: false",
		"github_api_replay_allowed: false",
		"raw_source_body_included: false",
		"raw_backup_bodies_included: false",
		"gitclaw backup verify --root .gitclaw/backups --repo owner/repo",
		"gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 489",
		"gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 489",
		"gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --target-repo owner/repo --issue 489",
		"gitclaw backup manifest --root .gitclaw/backups --repo owner/repo --issue 489",
	} {
		if !strings.Contains(restoreIssue.Body, want) {
			t.Fatalf("backup restore request issue missing %q:\n%s", want, restoreIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_RESTORE_SOURCE_SECRET", "CHANNEL_BACKUP_RESTORE_INGEST_SECRET", "Please review this channel-origin"} {
		if strings.Contains(restoreIssue.Body, leaked) {
			t.Fatalf("backup restore request issue leaked %q:\n%s", leaked, restoreIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[489]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-489"`,
		"GitClaw channel backup restore request",
		"Review issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Backup issue: #489",
		"Target repository: owner/repo",
		"Backup branch: gitclaw-backups",
		"Restore PR required: true",
		"Restore mode: dry-run-first",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel backup restore request notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_BACKUP_RESTORE_SOURCE_SECRET") || strings.Contains(outbound, "CHANNEL_BACKUP_RESTORE_INGEST_SECRET") {
		t.Fatalf("channel backup restore request notification leaked source:\n%s", outbound)
	}
	outboundParts := strings.SplitN(outbound, "\n", 2)
	if len(outboundParts) != 2 {
		t.Fatalf("channel backup restore request outbound missing provider body:\n%s", outbound)
	}
	notificationBody := strings.TrimSpace(outboundParts[1])
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Backup Restore Request Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels restore-request`",
		"channel_backup_restore_request_status: `created`",
		"restore_request_issue: `#101`",
		"restore_request_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#489`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"backup_issue: `#489`",
		"backup_branch: `gitclaw-backups`",
		"backup_root: `.gitclaw/backups`",
		"notification_body_sha256_12: `" + shortDocumentHash(notificationBody) + "`",
		fmt.Sprintf("notification_body_bytes: `%d`", len(notificationBody)),
		fmt.Sprintf("notification_body_lines: `%d`", lineCount(notificationBody)),
		"target_from_current_channel_issue: `true`",
		"restore_request_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"approval_required: `true`",
		"restore_pr_required: `true`",
		"restore_mode: `dry-run-first`",
		"repository_mutation_allowed: `false`",
		"backup_branch_write_allowed: `false`",
		"github_api_replay_allowed: `false`",
		"raw_request_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_source_body_included: `false`",
		"raw_backup_bodies_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_backup_restore_request_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel backup restore request receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_RESTORE_SOURCE_SECRET", "CHANNEL_BACKUP_RESTORE_INGEST_SECRET", "Please review this channel-origin", "channel-backup-restore", "channel-backup-restore-thread-123", "inbound-489", "notify-489", "owner__repo/issues/000489.json"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel backup restore request receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 489,
			"title": "GitClaw telegram thread channel-backup-restore-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-backup-restore-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48902,
			"body": "@gitclaw /channels restore-request --id channel-backup-restore --message-id inbound-489 --notify-message-id notify-489\nDo not leak duplicate token CHANNEL_BACKUP_RESTORE_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate channel backup restore request created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[489]); got != 4 {
		t.Fatalf("duplicate channel backup restore request posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[489])
	}
	duplicateReceipt := github.CommentsByIssue[489][3].Body
	for _, want := range []string{
		"channel_backup_restore_request_status: `duplicate`",
		"restore_request_issue: `#101`",
		"restore_request_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel backup restore request receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BACKUP_RESTORE_DUPLICATE_SECRET", "channel-backup-restore", "channel-backup-restore-thread-123", "inbound-489", "notify-489"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel backup restore request receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBackupRestoreRequestActionRequestParsesAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 34, Title: "Channel backup restore request"},
		Comment: &Comment{
			ID:   3401,
			Body: `@gitclaw /channel request-recovery #27 --id Channel.Backup.Restore --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelBackupRestoreRequestActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBackupRestoreRequestActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "request-recovery" || req.Options.Channel != "slack" || req.Options.BackupIssueNumber != 27 || req.Options.RequestID != "channel-backup-restore" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel backup restore request parsing: %#v", req)
	}
	if req.RestoreRequest.BackupIssueNumber != 27 || req.RestoreRequest.SourceKind != "channel_comment" || req.RestoreRequest.SourceCommentID != 3401 {
		t.Fatalf("unexpected channel backup restore request: %#v", req.RestoreRequest)
	}
	if req.RestoreRequest.IssueBackupPath != ".gitclaw/backups/owner__repo/issues/000027.json" || !strings.Contains(req.RestoreRequest.RestorePlanCmd, "--target-repo owner/repo --issue 27") {
		t.Fatalf("unexpected backup paths: %#v", req.RestoreRequest)
	}
}
