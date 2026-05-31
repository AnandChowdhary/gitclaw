package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestCheckpointRiskCommandReportsCleanPostureWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"checkpoints", "risk"}); err != nil {
			t.Fatalf("checkpoints risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Checkpoint Risk Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"checkpoint_risk_status: `ok`",
		"verification_scope: `git_checkpoint_metadata`",
		"checkpoint_strategy: `git-history-plus-backup-branch`",
		"rollback_mode: `inspect-only`",
		"git_available: `true`",
		"git_repository: `true`",
		"branch: `main`",
		"commits_available: `1`",
		"recent_commits_returned: `1`",
		"recent_commit_limit: `5`",
		"worktree_clean: `true`",
		"staged_changes: `0`",
		"unstaged_changes: `0`",
		"untracked_files: `0`",
		"surfaces_with_risk_findings: `0`",
		"checkpoint_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credential_values_included: `false`",
		"restore_operations_enabled: `false`",
		"git_reset_allowed: `false`",
		"git_clean_allowed: `false`",
		"checkout_mutation_allowed: `false`",
		"shadow_store_path_included: `false`",
		"pre_restore_snapshot_required: `true`",
		"rollback_diff_preview_required: `true`",
		"backup_manifest_required_for_restore: `true`",
		"llm_e2e_required_after_checkpoint_risk_change: `true`",
		"### Git Checkpoint Risk Card",
		"kind=`git-checkpoint`",
		"### Worktree Change Risk Card",
		"kind=`checkpoint-worktree` worktree_clean=`true`",
		"### Backup Branch Risk Card",
		"kind=`checkpoint-backup`",
		"### Rollback Operation Risk Card",
		"kind=`checkpoint-operation` rollback_mode=`inspect-only`",
		"### Recent Commit Risk Cards",
		"kind=`checkpoint-commit`",
		"raw_subject_included=`false`",
		"raw_diff_included=`false`",
		"### Current Checkpoint Request Risk Card",
		"scope=`local-cli` current_issue_checkpoint_request=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("checkpoint risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"- repository:", "- issue:", "CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_FILE_SECRET", "initial "} {
		if strings.Contains(output, notWant) {
			t.Fatalf("checkpoint risk output leaked %q:\n%s", notWant, output)
		}
	}
}

func TestCheckpointRiskReportFlagsDirtyWorktreeWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	writeTestFile(t, root, "tracked.txt", "dirty CHECKPOINT_DIRTY_SECRET\n")
	writeTestFile(t, root, "untracked.txt", "new CHECKPOINT_UNTRACKED_SECRET\n")

	output := RenderCheckpointRiskCLIReport(BuildCheckpointReport(root))
	for _, want := range []string{
		"GitClaw Checkpoint Risk Report",
		"checkpoint_risk_status: `warn`",
		"worktree_clean: `false`",
		"unstaged_changes: `1`",
		"untracked_files: `1`",
		"surfaces_with_risk_findings: `1`",
		"checkpoint_risk_findings: `3`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `3`",
		"code=`checkpoint_worktree_dirty`",
		"code=`checkpoint_unstaged_changes_present`",
		"code=`checkpoint_untracked_files_present`",
		"line_sha256_12=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("dirty checkpoint risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_DIRTY_SECRET", "CHECKPOINT_UNTRACKED_SECRET", "dirty ", "new "} {
		if strings.Contains(output, notWant) {
			t.Fatalf("dirty checkpoint risk output leaked %q:\n%s", notWant, output)
		}
	}
}

func TestRenderCheckpointReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 131,
			"title": "@gitclaw /rollback risk",
			"body": "Hidden checkpoint risk body token: CHECKPOINT_RISK_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	body := RenderCheckpointReport(ev, BuildCheckpointReport(root))
	for _, want := range []string{"GitClaw Checkpoint Risk Report", "repository: `owner/repo`", "issue: `#131`", "checkpoint_risk_status: `ok`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("checkpoint risk report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_RISK_BODY_SECRET", "CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_FILE_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("checkpoint risk report leaked %q:\n%s", notWant, body)
		}
	}
}

func TestHandleCheckpointRiskCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 132,
			"title": "@gitclaw /checkpoints risk",
			"body": "Hidden checkpoint risk handler token: CHECKPOINT_RISK_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{132: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic checkpoint risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Checkpoint Risk Report", "Generated without a model call", "model=\"gitclaw/checkpoints\"", "checkpoint_risk_status: `ok`", "verification_scope: `git_checkpoint_metadata`", "restore_operations_enabled: `false`", "llm_e2e_required_after_checkpoint_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("checkpoint risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_RISK_HANDLER_BODY_SECRET", "CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_FILE_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("checkpoint risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[132], "gitclaw:done") || hasLabel(github.IssueLabels[132], "gitclaw:running") || hasLabel(github.IssueLabels[132], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[132])
	}
}

func TestHandleCheckpointCatalogCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	initCheckpointRiskRepo(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 134,
			"title": "@gitclaw /rollback catalog",
			"body": "Hidden checkpoint catalog handler token: CHECKPOINT_CATALOG_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{134: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic checkpoint catalog command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Checkpoints Catalog Report",
		"Generated without a model call",
		"model=\"gitclaw/checkpoints\"",
		"requested_checkpoints_command: `catalog`",
		"checkpoint_catalog_status: `ok`",
		"catalog_entries: `8`",
		"checkpoint_layers: `7`",
		"command=`rollback-catalog` issue_intent=`@gitclaw /rollback catalog`",
		"layer=`git-history` store=`repository .git metadata`",
		"restore_gate=`disabled-inspect-only-v1`",
		"llm_e2e_required_after_checkpoint_catalog_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("checkpoint catalog handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"CHECKPOINT_CATALOG_HANDLER_BODY_SECRET", "CHECKPOINT_COMMIT_SECRET", "CHECKPOINT_FILE_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("checkpoint catalog handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[134], "gitclaw:done") || hasLabel(github.IssueLabels[134], "gitclaw:running") || hasLabel(github.IssueLabels[134], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[134])
	}
}

func initCheckpointRiskRepo(t *testing.T, root string) {
	t.Helper()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, "tracked.txt", "clean CHECKPOINT_FILE_SECRET\n")
	runCheckpointTestGit(t, root, "add", "tracked.txt")
	runCheckpointTestGit(t, root, "commit", "-m", "initial CHECKPOINT_COMMIT_SECRET")
}
