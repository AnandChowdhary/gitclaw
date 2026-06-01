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
