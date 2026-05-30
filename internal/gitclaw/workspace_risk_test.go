package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestWorkspaceRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"workspace", "risk"}); err != nil {
			t.Fatalf("workspace risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Workspace Risk Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"workspace_risk_status: `ok`",
		"verification_scope: `github_actions_workspace_metadata`",
		"workspace_policy_present: `true`",
		"workspace_policy_loaded_for_model: `true`",
		"workspace_specs: `1`",
		"scanned_workspace_specs: `1`",
		"workspace_specs_requiring_approval: `1`",
		"git_available: `true`",
		"git_repository: `true`",
		"repo_file_list_limit: `240`",
		"context_documents_loaded: `1`",
		"workflow_files_present: `1`",
		"checkout_workflows: `1`",
		"checkout_steps: `1`",
		"setup_go_steps: `1`",
		"checkout_action_versions: `actions/checkout@v5`",
		"setup_go_action_versions: `actions/setup-go@v6`",
		"fetch_depth_configured: `true`",
		"sandbox_backend: `github-actions`",
		"durable_state_backend: `git-tracked-files-and-backup-branch`",
		"surfaces_with_risk_findings: `0`",
		"workspace_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"private_workspace_memory_supported: `false`",
		"external_workspace_mount_supported: `false`",
		"workspace_mutation_allowed: `false`",
		"workspace_daemon_supported: `false`",
		"long_running_socket_supported: `false`",
		"raw_workspace_bodies_included: `false`",
		"raw_file_bodies_included: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"credential_values_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_workspace_risk_change: `true`",
		"### Workspace Policy Risk Card",
		"kind=`workspace-policy` path=`.gitclaw/WORKSPACE.md`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Workspace Spec Risk Cards",
		"kind=`workspace-spec` name=`repository-checkout` path=`.gitclaw/workspaces/repository.md`",
		"runtime=`github-actions`",
		"storage=`repository-checkout`",
		"mode=`metadata-only`",
		"root=`.`",
		"isolation=`ephemeral-actions-runner`",
		"durable_state=`git-tracked-files-and-backup-branch`",
		"requires_approval=`true`",
		"### Workflow Workspace Risk Cards",
		"kind=`workspace-workflow` path=`.github/workflows/gitclaw.yml` present=`true`",
		"checkout_actions=`actions/checkout@v5`",
		"setup_go_actions=`actions/setup-go@v6`",
		"fetch_depth_configured=`true`",
		"### Git Workspace Risk Card",
		"kind=`git-workspace` root=`.` git_available=`true` git_repository=`true`",
		"### Repository Inventory Risk Card",
		"raw_paths_included=`false`",
		"raw_context_bodies_included=`false`",
		"### Current Workspace Request Risk Card",
		"scope=`local-cli` current_issue_workspace_request=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("workspace risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"- repository:", "- issue:", "WORKSPACE_POLICY_BODY_SECRET", "WORKSPACE_SPEC_BODY_SECRET", "GITCLAW_WORKSPACE_CONTEXT_V1", "This declarative workspace record"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("workspace risk output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestRenderWorkspaceRiskReportFlagsSpecRisksWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepoWithSpec(t, dir, `---
name: risky
kind: host-workspace
runtime: local-host
storage: s3
mode: read-write
root: /
isolation: shared-host
durable_state: runner-local-private-memory
requires_approval: false
---

api_key=WORKSPACE_RISK_SPEC_SECRET
private workspace memory is enabled
mount external paths from /Users
rm -rf workspace-cache
start daemon for websocket server
print raw file bodies
repo_file_list_limit: 0
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	output := RenderWorkspaceRiskCLIReport(cfg)
	for _, want := range []string{
		"GitClaw Workspace Risk Report",
		"workspace_risk_status: `high`",
		"workspace_specs: `1`",
		"scanned_workspace_specs: `1`",
		"workspace_specs_requiring_approval: `0`",
		"surfaces_with_risk_findings: `1`",
		"workspace_risk_findings: `16`",
		"high_risk_findings: `10`",
		"warning_risk_findings: `6`",
		"code=`credential_material_in_workspace`",
		"code=`destructive_workspace_mutation`",
		"code=`external_workspace_mount_enabled`",
		"code=`long_running_workspace_service`",
		"code=`private_workspace_memory_enabled`",
		"code=`raw_workspace_body_leakage`",
		"code=`unbounded_workspace_inventory`",
		"code=`workspace_approval_gate_missing`",
		"code=`workspace_durable_state_unreviewed`",
		"code=`workspace_isolation_not_ephemeral_runner`",
		"code=`workspace_kind_not_git_workspace`",
		"code=`workspace_mode_not_metadata_only`",
		"code=`workspace_root_not_repo_root`",
		"code=`workspace_runtime_not_github_actions`",
		"code=`workspace_storage_not_checkout`",
		"line_sha256_12=",
		"risk_max_severity=`high`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("workspace risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"WORKSPACE_RISK_SPEC_SECRET", "api_key=", "private workspace memory", "mount external paths", "rm -rf", "start daemon", "print raw file bodies", "repo_file_list_limit: 0"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("workspace risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderWorkspaceReportRoutesRiskAuditWithoutBodies(t *testing.T) {
	root := t.TempDir()
	initWorkspaceRepoWithSpec(t, root, workspaceSpecTestBody+"\napi_key=WORKSPACE_ROUTE_RISK_SPEC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 129,
			"title": "@gitclaw /workspace risk",
			"body": "Hidden workspace risk body token: WORKSPACE_RISK_BODY_SECRET.",
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
	body := RenderWorkspaceReport(ev, cfg)
	for _, want := range []string{"GitClaw Workspace Risk Report", "repository: `owner/repo`", "issue: `#129`", "workspace_risk_status: `high`", "code=`credential_material_in_workspace`", "issue_title_sha256_12:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("workspace risk report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"WORKSPACE_RISK_BODY_SECRET", "WORKSPACE_ROUTE_RISK_SPEC_SECRET", "api_key="} {
		if strings.Contains(body, notWant) {
			t.Fatalf("workspace risk report leaked body token %q:\n%s", notWant, body)
		}
	}
}

func TestHandleWorkspaceRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 130,
			"title": "@gitclaw /repo risk",
			"body": "Hidden workspace risk handler token: WORKSPACE_RISK_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{130: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic workspace risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Workspace Risk Report", "Generated without a model call", "model=\"gitclaw/workspace\"", "workspace_risk_status: `ok`", "verification_scope: `github_actions_workspace_metadata`", "raw_workspace_bodies_included: `false`", "raw_file_bodies_included: `false`", "raw_workflow_bodies_included: `false`", "raw_issue_bodies_included: `false`", "raw_comment_bodies_included: `false`", "llm_e2e_required_after_workspace_risk_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("workspace risk handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"WORKSPACE_RISK_HANDLER_BODY_SECRET", "WORKSPACE_POLICY_BODY_SECRET", "WORKSPACE_SPEC_BODY_SECRET", "This declarative workspace record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("workspace risk handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[130], "gitclaw:done") || hasLabel(github.IssueLabels[130], "gitclaw:running") || hasLabel(github.IssueLabels[130], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[130])
	}
}

func initWorkspaceRepoWithSpec(t *testing.T, root, specBody string) {
	t.Helper()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, workspacePolicyPath, workspacePolicyTestBody)
	writeTestFile(t, root, ".gitclaw/workspaces/repository.md", specBody)
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", workspaceWorkflowTestBody)
	writeTestFile(t, root, "README.md", "workspace fixture\n")
	runCheckpointTestGit(t, root, "add", ".")
	runCheckpointTestGit(t, root, "commit", "-m", "initial workspace risk fixture")
}
