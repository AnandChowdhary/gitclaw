package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleCheckpointRehearsalCreatesRollbackIssueWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 301,
			"title": "Rehearse checkpoint rollback",
			"body": "@gitclaw /checkpoints rehearse --id rollback-lab-1 --target HEAD~1\n\nDo not leak source token CHECKPOINT_REHEARSAL_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{301: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for checkpoint rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsal := github.Issues[0]
	if !strings.Contains(rehearsal.Body, "gitclaw:checkpoint-rehearsal-issue") || !strings.Contains(rehearsal.Body, `id="rollback-lab-1"`) {
		t.Fatalf("rehearsal issue missing marker:\n%s", rehearsal.Body)
	}
	if !hasLabel(github.IssueLabels[rehearsal.Number], cfg.TriggerLabel) {
		t.Fatalf("rehearsal issue missing gitclaw label: %#v", github.IssueLabels[rehearsal.Number])
	}
	for _, want := range []string{
		"GitClaw checkpoint rollback rehearsal issue",
		"rehearsal_id: rollback-lab-1",
		"target_ref: HEAD~1",
		"target_ref_sha256_12:",
		"target_allowed: true",
		"rehearsal_mode: rollback-conversation",
		"restore_mode: rehearsal-only",
		"rollback_mode: inspect-only",
		"repository_mutation_allowed: false",
		"git_reset_allowed: false",
		"git_clean_allowed: false",
		"checkout_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_diffs_included: false",
		"raw_file_bodies_included: false",
		"gitclaw checkpoints status",
		"gitclaw checkpoints preview HEAD~1",
		"gitclaw checkpoints risk",
		"gitclaw rollback diff HEAD~1",
		"gitclaw rollback risk",
	} {
		if !strings.Contains(rehearsal.Body, want) {
			t.Fatalf("checkpoint rehearsal issue missing %q:\n%s", want, rehearsal.Body)
		}
	}
	for _, leaked := range []string{"CHECKPOINT_REHEARSAL_SOURCE_SECRET", "Do not leak source token"} {
		if strings.Contains(rehearsal.Body, leaked) {
			t.Fatalf("checkpoint rehearsal issue leaked %q:\n%s", leaked, rehearsal.Body)
		}
	}

	sourceComments := github.CommentsByIssue[301]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want rehearsal receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Checkpoint Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/checkpoints"`,
		"requested_checkpoints_command: `/checkpoints rehearse`",
		"checkpoint_rehearsal_status: `created`",
		"rehearsal_issue: `#100`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"target_ref_sha256_12:",
		"target_allowed: `true`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"restore_mode: `rehearsal-only`",
		"rollback_mode: `inspect-only`",
		"repository_mutation_allowed: `false`",
		"git_reset_allowed: `false`",
		"git_clean_allowed: `false`",
		"checkout_mutation_allowed: `false`",
		"raw_source_body_included: `false`",
		"raw_target_ref_included: `false`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"llm_e2e_required_after_checkpoint_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("checkpoint rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHECKPOINT_REHEARSAL_SOURCE_SECRET", "Do not leak source token", "rollback-lab-1", "HEAD~1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("checkpoint rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 301,
			"title": "Rehearse checkpoint rollback",
			"body": "@gitclaw /checkpoints rehearse --id rollback-lab-1 --target HEAD~1\n\nDo not leak source token CHECKPOINT_REHEARSAL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 30101,
			"body": "@gitclaw /rollback rehearsal --id rollback-lab-1 --target HEAD~1\n\nDo not leak duplicate token CHECKPOINT_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate checkpoint rehearsal created more issues: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[301][1].Body
	for _, want := range []string{
		"checkpoint_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate checkpoint rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHECKPOINT_REHEARSAL_DUPLICATE_SECRET", "rollback-lab-1", "HEAD~1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate checkpoint rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildCheckpointRehearsalIssueRequestParsesTargetOption(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 32, Title: "Checkpoint rehearsal"},
		Comment: &Comment{
			ID:   3201,
			Body: "@gitclaw /rollback drill --ref HEAD~2 --id Rollback.Lab",
		},
	}
	req, err := BuildCheckpointRehearsalIssueRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildCheckpointRehearsalIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "drill" || req.TargetRef != "HEAD~2" || req.RehearsalID != "rollback-lab" {
		t.Fatalf("unexpected checkpoint rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 3201 || req.SourceSHA == "" || req.TargetRefSHA == "" {
		t.Fatalf("unexpected checkpoint rehearsal source metadata: %#v", req)
	}
	if !strings.Contains(req.CheckpointPreviewCmd, "HEAD~2") || !strings.Contains(req.RollbackDiffCmd, "HEAD~2") {
		t.Fatalf("unexpected checkpoint dry-run commands: %#v", req)
	}
}
