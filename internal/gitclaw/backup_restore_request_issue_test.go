package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleBackupRestoreRequestCreatesReviewIssueWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 320,
			"title": "Request backup restore",
			"body": "@gitclaw /backup restore-request --id restore-lab-1\n\nDo not leak source token BACKUP_RESTORE_REQUEST_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{320: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for backup restore request action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one restore request issue: %#v", len(github.Issues), github.Issues)
	}
	restoreIssue := github.Issues[0]
	if restoreIssue.Title != "GitClaw backup restore request: #320" || !strings.Contains(restoreIssue.Body, backupRestoreRequestIssueMarker) {
		t.Fatalf("unexpected restore request issue: %#v", restoreIssue)
	}
	if !hasLabel(github.IssueLabels[restoreIssue.Number], cfg.TriggerLabel) {
		t.Fatalf("restore request issue missing gitclaw label: %#v", github.IssueLabels[restoreIssue.Number])
	}
	for _, want := range []string{
		"GitClaw backup restore request issue",
		"request_id: restore-lab-1",
		"restore_scope: issue-thread",
		"backup_issue: #320",
		"target_repository: owner/repo",
		"backup_branch: gitclaw-backups",
		"issue_backup_path: .gitclaw/backups/owner__repo/issues/000320.json",
		"approval_required: true",
		"restore_pr_required: true",
		"restore_mode: dry-run-first",
		"repository_mutation_allowed: false",
		"backup_branch_write_allowed: false",
		"github_api_replay_allowed: false",
		"raw_source_body_included: false",
		"raw_backup_bodies_included: false",
		"gitclaw backup verify --root .gitclaw/backups --repo owner/repo",
		"gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 320",
		"gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 320",
		"gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --target-repo owner/repo --issue 320",
		"gitclaw backup manifest --root .gitclaw/backups --repo owner/repo --issue 320",
	} {
		if !strings.Contains(restoreIssue.Body, want) {
			t.Fatalf("backup restore request issue missing %q:\n%s", want, restoreIssue.Body)
		}
	}
	for _, leaked := range []string{"BACKUP_RESTORE_REQUEST_SOURCE_SECRET", "Do not leak source token"} {
		if strings.Contains(restoreIssue.Body, leaked) {
			t.Fatalf("backup restore request issue leaked %q:\n%s", leaked, restoreIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[320]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want restore request receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Backup Restore Request Issue Action",
		"Generated without a model call",
		`model="gitclaw/backup"`,
		"requested_backup_command: `/backup restore-request`",
		"backup_restore_request_status: `created`",
		"restore_request_issue: `#100`",
		"restore_request_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"backup_issue: `#320`",
		"target_repository: `owner/repo`",
		"backup_branch: `gitclaw-backups`",
		"backup_root: `.gitclaw/backups`",
		"restore_request_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"approval_required: `true`",
		"restore_pr_required: `true`",
		"restore_mode: `dry-run-first`",
		"repository_mutation_allowed: `false`",
		"backup_branch_write_allowed: `false`",
		"github_api_replay_allowed: `false`",
		"raw_source_body_included: `false`",
		"raw_backup_bodies_included: `false`",
		"llm_e2e_required_after_backup_restore_request_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("backup restore request receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"BACKUP_RESTORE_REQUEST_SOURCE_SECRET", "Do not leak source token", "restore-lab-1", "owner__repo/issues/000320.json"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("backup restore request receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 320,
			"title": "Request backup restore",
			"body": "@gitclaw /backup restore-request --id restore-lab-1\n\nDo not leak source token BACKUP_RESTORE_REQUEST_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 32001,
			"body": "@gitclaw /backup restore-request --id restore-lab-1\n\nDo not leak duplicate token BACKUP_RESTORE_REQUEST_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate backup restore request created more issues: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[320][1].Body
	for _, want := range []string{
		"backup_restore_request_status: `existing`",
		"restore_request_issue_created: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate restore request receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"BACKUP_RESTORE_REQUEST_DUPLICATE_SECRET", "restore-lab-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup restore request receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestHandleBackupRestoreRequestQueuesChannelNotificationWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: backup-restore-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 321,
			"title": "Request backup restore notification",
			"body": "@gitclaw /backup restore-request --id restore-notify-1 --notify-route e2e-telegram-route\n\nOpen a restore review and notify recovery ops. Hidden token: BACKUP_RESTORE_NOTIFY_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{321: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for backup restore notify action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want restore request issue and channel issue: %#v", len(github.Issues), github.Issues)
	}
	restoreIssue := github.Issues[0]
	channelIssue := github.Issues[1]
	if !strings.Contains(restoreIssue.Body, backupRestoreRequestIssueMarker) || restoreIssue.Title != "GitClaw backup restore request: #321" {
		t.Fatalf("first issue should be restore request issue: %#v", restoreIssue)
	}
	if !hasLabel(github.IssueLabels[restoreIssue.Number], cfg.TriggerLabel) {
		t.Fatalf("restore request issue missing trigger label: %#v", github.IssueLabels[restoreIssue.Number])
	}
	if strings.Contains(restoreIssue.Body, "BACKUP_RESTORE_NOTIFY_SOURCE_SECRET") || strings.Contains(restoreIssue.Body, "notify recovery ops") || strings.Contains(restoreIssue.Body, "e2e-telegram-route") {
		t.Fatalf("restore request issue leaked source or route:\n%s", restoreIssue.Body)
	}
	if !HasChannelThreadMarker(channelIssue.Body) || !strings.Contains(channelIssue.Body, `channel="telegram"`) {
		t.Fatalf("second issue should be telegram channel issue: %#v", channelIssue)
	}
	if hasLabel(github.IssueLabels[channelIssue.Number], cfg.TriggerLabel) {
		t.Fatalf("channel issue should not carry trigger label: %#v", github.IssueLabels[channelIssue.Number])
	}
	if !hasLabel(github.IssueLabels[channelIssue.Number], cfg.ChannelLabel) {
		t.Fatalf("channel issue missing channel label: %#v", github.IssueLabels[channelIssue.Number])
	}
	channelComments := github.CommentsByIssue[channelIssue.Number]
	if len(channelComments) != 1 {
		t.Fatalf("channel issue comments = %d, want one notification: %#v", len(channelComments), channelComments)
	}
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`message_id="gitclaw-backup-restore-request-restore-notify-1"`,
		"GitClaw backup restore request",
		"Review issue: #100 https://github.com/owner/repo/issues/100",
		"Source issue: #321 https://github.com/owner/repo/issues/321",
		"Request id: restore-notify-1",
		"Backup issue: #321",
		"Target repository: owner/repo",
		"Backup branch: gitclaw-backups",
		"Restore PR required: true",
		"Restore mode: dry-run-first",
	} {
		if !strings.Contains(channelComments[0].Body, want) {
			t.Fatalf("channel notification missing %q:\n%s", want, channelComments[0].Body)
		}
	}
	if strings.Contains(channelComments[0].Body, "BACKUP_RESTORE_NOTIFY_SOURCE_SECRET") || strings.Contains(channelComments[0].Body, "notify recovery ops") {
		t.Fatalf("channel notification leaked source body:\n%s", channelComments[0].Body)
	}

	receipt := github.CommentsByIssue[321][0].Body
	for _, want := range []string{
		"GitClaw Backup Restore Request Issue Action",
		"backup_restore_request_status: `created`",
		"restore_request_issue: `#100`",
		"channel_notification_requested: `true`",
		"channel_notification_routes: `1`",
		"channel_notification_queued: `1`",
		"channel_notification_duplicates: `0`",
		"channel_notification_target_issues_created: `1`",
		"raw_channel_routes_included: `false`",
		"raw_channel_notification_body_included: `false`",
		"provider_delivery_performed: `false`",
		"destination=`01` target_issue=`#101`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("backup restore notify receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"BACKUP_RESTORE_NOTIFY_SOURCE_SECRET", "notify recovery ops", "e2e-telegram-route", "restore-notify-1", "gitclaw-backup-restore-request-restore-notify-1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("backup restore notify receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 321,
			"title": "Request backup restore notification",
			"body": "@gitclaw /backup restore-request --id restore-notify-1 --notify-route e2e-telegram-route",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 32101,
			"body": "@gitclaw /backup restore-request --id restore-notify-1 --notify-route e2e-telegram-route\n\nDuplicate hidden token: BACKUP_RESTORE_NOTIFY_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[channelIssue.Number]); got != 1 {
		t.Fatalf("duplicate posted another channel notification: %d", got)
	}
	duplicateReceipt := github.CommentsByIssue[321][1].Body
	for _, want := range []string{
		"backup_restore_request_status: `existing`",
		"duplicate_suppressed: `true`",
		"channel_notification_requested: `true`",
		"channel_notification_queued: `0`",
		"channel_notification_duplicates: `1`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate backup restore notify receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"BACKUP_RESTORE_NOTIFY_DUPLICATE_SECRET", "e2e-telegram-route", "restore-notify-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup restore notify receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildBackupRestoreRequestIssueRequestParsesIssueOption(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 33, Title: "Backup restore request"},
		Comment: &Comment{
			ID:   3301,
			Body: "@gitclaw /backup request-restore --issue #28 --id Restore.Request",
		},
	}
	req, err := BuildBackupRestoreRequestIssueRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildBackupRestoreRequestIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "request-restore" || req.BackupIssueNumber != 28 || req.RequestID != "restore-request" {
		t.Fatalf("unexpected backup restore request parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 3301 || req.SourceSHA == "" {
		t.Fatalf("unexpected backup restore request source metadata: %#v", req)
	}
	if req.IssueBackupPath != ".gitclaw/backups/owner__repo/issues/000028.json" || !strings.Contains(req.RestorePlanCmd, "--issue 28") || !strings.Contains(req.RestorePlanCmd, "--target-repo owner/repo") {
		t.Fatalf("unexpected backup restore request paths: %#v", req)
	}
}
