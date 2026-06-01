package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleBackupRehearsalCreatesRecoveryIssueWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 300,
			"title": "Rehearse backup recovery",
			"body": "@gitclaw /backup rehearse --id backup-lab-1\n\nDo not leak source token BACKUP_REHEARSAL_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{300: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for backup rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsal := github.Issues[0]
	if !strings.Contains(rehearsal.Body, "gitclaw:backup-rehearsal-issue") || !strings.Contains(rehearsal.Body, `id="backup-lab-1"`) {
		t.Fatalf("rehearsal issue missing marker:\n%s", rehearsal.Body)
	}
	if !hasLabel(github.IssueLabels[rehearsal.Number], cfg.TriggerLabel) {
		t.Fatalf("rehearsal issue missing gitclaw label: %#v", github.IssueLabels[rehearsal.Number])
	}
	for _, want := range []string{
		"GitClaw backup recovery rehearsal issue",
		"rehearsal_id: backup-lab-1",
		"backup_issue: #300",
		"backup_branch: gitclaw-backups",
		"issue_backup_path: .gitclaw/backups/owner__repo/issues/000300.json",
		"rehearsal_mode: recovery-conversation",
		"restore_mode: dry-run",
		"repository_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_backup_bodies_included: false",
		"gitclaw backup coverage --root .gitclaw/backups --repo owner/repo --issue 300",
		"gitclaw backup drill --root .gitclaw/backups --repo owner/repo --issue 300",
		"gitclaw backup restore-plan --root .gitclaw/backups --repo owner/repo --issue 300",
	} {
		if !strings.Contains(rehearsal.Body, want) {
			t.Fatalf("backup rehearsal issue missing %q:\n%s", want, rehearsal.Body)
		}
	}
	for _, leaked := range []string{"BACKUP_REHEARSAL_SOURCE_SECRET", "Do not leak source token"} {
		if strings.Contains(rehearsal.Body, leaked) {
			t.Fatalf("backup rehearsal issue leaked %q:\n%s", leaked, rehearsal.Body)
		}
	}

	sourceComments := github.CommentsByIssue[300]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want rehearsal receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Backup Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/backup"`,
		"requested_backup_command: `/backup rehearse`",
		"backup_rehearsal_status: `created`",
		"rehearsal_issue: `#100`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"backup_issue: `#300`",
		"backup_branch: `gitclaw-backups`",
		"backup_root: `.gitclaw/backups`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"restore_mode: `dry-run`",
		"repository_mutation_allowed: `false`",
		"backup_branch_write_allowed: `false`",
		"github_api_replay_allowed: `false`",
		"raw_source_body_included: `false`",
		"raw_backup_bodies_included: `false`",
		"llm_e2e_required_after_backup_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("backup rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"BACKUP_REHEARSAL_SOURCE_SECRET", "Do not leak source token", "backup-lab-1", "owner__repo/issues/000300.json"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("backup rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 300,
			"title": "Rehearse backup recovery",
			"body": "@gitclaw /backup rehearse --id backup-lab-1\n\nDo not leak source token BACKUP_REHEARSAL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 30001,
			"body": "@gitclaw /backup rehearse --id backup-lab-1\n\nDo not leak duplicate token BACKUP_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate backup rehearsal created more issues: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[300][1].Body
	for _, want := range []string{
		"backup_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"BACKUP_REHEARSAL_DUPLICATE_SECRET", "backup-lab-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate backup rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildBackupRehearsalIssueRequestParsesIssueOption(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Backup rehearsal"},
		Comment: &Comment{
			ID:   3101,
			Body: "@gitclaw /backup recovery --issue #27 --id Restore.Lab",
		},
	}
	req, err := BuildBackupRehearsalIssueRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildBackupRehearsalIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "recovery" || req.BackupIssueNumber != 27 || req.RehearsalID != "restore-lab" {
		t.Fatalf("unexpected backup rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 3101 || req.SourceSHA == "" {
		t.Fatalf("unexpected backup rehearsal source metadata: %#v", req)
	}
	if req.IssueBackupPath != ".gitclaw/backups/owner__repo/issues/000027.json" || !strings.Contains(req.RestorePlanCmd, "--issue 27") {
		t.Fatalf("unexpected backup paths: %#v", req)
	}
}
