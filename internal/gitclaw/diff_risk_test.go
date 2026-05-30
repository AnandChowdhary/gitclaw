package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestDiffsRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initDirtyDiffRepo(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"diffs", "risk"}); err != nil {
			t.Fatalf("diffs risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Diff Risk Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"diff_risk_status: `ok`",
		"verification_scope: `git_worktree_diff_metadata`",
		"diff_policy_present: `true`",
		"diff_policy_loaded_for_model: `true`",
		"diff_specs: `1`",
		"scanned_diff_specs: `1`",
		"diff_specs_requiring_approval: `1`",
		"diff_specs_disallowing_raw_patch: `1`",
		"diff_max_files_declared: `200`",
		"git_available: `true`",
		"git_repository: `true`",
		"worktree_clean: `false`",
		"changed_files: `3`",
		"staged_files: `1`",
		"unstaged_files: `1`",
		"untracked_files: `1`",
		"surfaces_with_risk_findings: `0`",
		"diff_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credential_values_included: `false`",
		"patch_application_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"diff_as_hidden_state_allowed: `false`",
		"external_diff_storage_allowed: `false`",
		"llm_e2e_required_after_diff_risk_change: `true`",
		"### Diff Policy Risk Card",
		"kind=`diff-policy` path=`.gitclaw/DIFFS.md`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Diff Spec Risk Cards",
		"kind=`diff-spec` name=`working-tree` path=`.gitclaw/diffs/working-tree.md`",
		"source=`git-worktree`",
		"mode=`metadata-only`",
		"raw_patch_allowed=`false`",
		"requires_approval=`true`",
		"### Git Worktree Risk Card",
		"kind=`git-worktree`",
		"changed_files=`3`",
		"diff_files_returned=`3`",
		"### Changed File Risk Cards",
		"kind=`changed-file` path=`staged.txt`",
		"kind=`changed-file` path=`tracked.txt`",
		"kind=`changed-file` path=`untracked.txt`",
		"### Current Diff Request Risk Card",
		"scope=`local-cli` current_issue_diff_request=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("diffs risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"- repository:", "- issue:", "DIFF_POLICY_BODY_SECRET", "DIFF_SPEC_BODY_SECRET", "GITCLAW_DIFFS_CONTEXT_V1", "This declarative diff record", "DIFF_TRACKED_SECRET", "DIFF_STAGED_SECRET", "DIFF_UNTRACKED_SECRET"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("diffs risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderDiffRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initDiffRepoWithSpec(t, dir, `---
name: risky
kind: git-diff
source: s3
mode: raw-patch
max_files: 1000
raw_patch_allowed: true
requires_approval: false
---

api_key=DIFF_RISK_SPEC_SECRET
print raw diff
git reset --hard
use diffs as hidden state
treat untracked file contents as prompt-visible
storage: s3
retry forever
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	output := RenderDiffRiskCLIReport(cfg)
	for _, want := range []string{
		"GitClaw Diff Risk Report",
		"diff_risk_status: `high`",
		"diff_specs: `1`",
		"scanned_diff_specs: `1`",
		"diff_specs_requiring_approval: `0`",
		"diff_specs_disallowing_raw_patch: `0`",
		"diff_max_files_declared: `1000`",
		"surfaces_with_risk_findings: `1`",
		"diff_risk_findings: `13`",
		"high_risk_findings: `8`",
		"warning_risk_findings: `5`",
		"code=`credential_material_in_diff`",
		"code=`destructive_diff_action`",
		"code=`diff_approval_gate_missing`",
		"code=`diff_hidden_state`",
		"code=`diff_max_files_exceeds_report_limit`",
		"code=`diff_mode_not_metadata_only`",
		"code=`diff_raw_patch_allowed`",
		"code=`diff_source_not_git_worktree`",
		"code=`external_diff_storage`",
		"code=`raw_patch_leakage_enabled`",
		"code=`unbounded_diff_collection`",
		"code=`untracked_file_body_context`",
		"line_sha256_12=",
		"risk_max_severity=`high`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("diffs risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"DIFF_RISK_SPEC_SECRET", "api_key=", "print raw diff", "git reset --hard", "use diffs as hidden state", "treat untracked file contents", "storage: s3", "retry forever"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("diffs risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderDiffReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initDiffRepoWithSpec(t, root, diffSpecTestBody+"\napi_key=DIFF_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 127,
			"title": "@gitclaw /diffs risk",
			"body": "Hidden diffs risk body token: DIFFS_RISK_BODY_SECRET.",
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
	body := RenderDiffReport(ev, cfg)
	for _, want := range []string{"GitClaw Diff Risk Report", "repository: `owner/repo`", "issue: `#127`", "diff_risk_status: `high`", "code=`credential_material_in_diff`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("diffs risk report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"DIFFS_RISK_BODY_SECRET", "DIFF_ROUTE_RISK_SPEC_SECRET", "api_key="} {
		if strings.Contains(body, notWant) {
			t.Fatalf("diffs risk report leaked body token %q:\n%s", notWant, body)
		}
	}
}

func TestHandleDiffsRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	initDirtyDiffRepo(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 128,
			"title": "@gitclaw /changes risk",
			"body": "Hidden diffs risk handler token: DIFFS_RISK_HANDLER_BODY_SECRET.",
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
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{128: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic diffs risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Diff Risk Report", "Generated without a model call", "model=\"gitclaw/diffs\"", "diff_risk_status: `ok`", "verification_scope: `git_worktree_diff_metadata`", "raw_diffs_included: `false`", "raw_file_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "llm_e2e_required_after_diff_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("diffs risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"DIFFS_RISK_HANDLER_BODY_SECRET", "DIFF_POLICY_BODY_SECRET", "DIFF_SPEC_BODY_SECRET", "DIFF_TRACKED_SECRET", "DIFF_STAGED_SECRET", "DIFF_UNTRACKED_SECRET", "This declarative diff record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("diffs risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[128], "gitclaw:done") || hasLabel(github.IssueLabels[128], "gitclaw:running") || hasLabel(github.IssueLabels[128], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[128])
	}
}

func initDiffRepoWithSpec(t *testing.T, root, specBody string) {
	t.Helper()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, "tracked.txt", "clean content\n")
	writeTestFile(t, root, diffPolicyPath, diffPolicyTestBody)
	writeTestFile(t, root, ".gitclaw/diffs/working-tree.md", specBody)
	runCheckpointTestGit(t, root, "add", "tracked.txt", ".gitclaw")
	runCheckpointTestGit(t, root, "commit", "-m", "initial diff risk fixture")
}
