package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const workspacePolicyTestBody = `# Workspace

GITCLAW_WORKSPACE_CONTEXT_V1

WORKSPACE_POLICY_BODY_SECRET
`

const workspaceSpecTestBody = `---
name: repository-checkout
kind: git-workspace
runtime: github-actions
storage: repository-checkout
mode: metadata-only
root: .
isolation: ephemeral-actions-runner
durable_state: git-tracked-files-and-backup-branch
requires_approval: true
---

# Repository Checkout Workspace

This declarative workspace record must not be printed.

WORKSPACE_SPEC_BODY_SECRET
`

const workspaceWorkflowTestBody = `name: GitClaw

on:
  issues:
    types: [opened]

jobs:
  handle:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 1
      - uses: actions/setup-go@v6
`

func TestRenderWorkspaceReportAuditsWorkspaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 129,
			"title": "@gitclaw /workspace",
			"body": "Hidden workspace report body token: WORKSPACE_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderWorkspaceReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Workspace Report",
		"Generated without a model call",
		"workspace_status: `ok`",
		"workspace_policy_path: `.gitclaw/WORKSPACE.md`",
		"workspace_policy_present: `true`",
		"workspace_policy_loaded_for_model: `true`",
		"workspace_specs_dir: `.gitclaw/workspaces`",
		"workspace_specs: `1`",
		"workspace_specs_with_frontmatter: `1`",
		"workspace_specs_requiring_approval: `1`",
		"git_available: `true`",
		"git_repository: `true`",
		"worktree_root: `.`",
		"branch: `main`",
		"repo_file_list_limit: `240`",
		"context_documents_loaded: `1`",
		"workspace_context_policy_loaded: `true`",
		"workflow_files_present: `1`",
		"checkout_workflows: `1`",
		"checkout_steps: `1`",
		"checkout_action_versions: `actions/checkout@v5`",
		"setup_go_action_versions: `actions/setup-go@v6`",
		"fetch_depth_configured: `true`",
		"sandbox_backend: `github-actions`",
		"durable_state_backend: `git-tracked-files-and-backup-branch`",
		"private_workspace_memory_supported: `false`",
		"external_workspace_mount_supported: `false`",
		"workspace_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_file_bodies_included: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Workspace Policy",
		"`.gitclaw/WORKSPACE.md` loaded=`true` source=`contextDocumentPaths`",
		"### Workspace Specs",
		"name=`repository-checkout`",
		"path=`.gitclaw/workspaces/repository.md`",
		"frontmatter=`true`",
		"kind=`git-workspace`",
		"runtime=`github-actions`",
		"storage=`repository-checkout`",
		"mode=`metadata-only`",
		"root=`.`",
		"isolation=`ephemeral-actions-runner`",
		"durable_state=`git-tracked-files-and-backup-branch`",
		"requires_approval=`true`",
		"### Workflow Workspace Setup",
		"path=`.github/workflows/gitclaw.yml`",
		"checkout_actions=`actions/checkout@v5`",
		"setup_go_actions=`actions/setup-go@v6`",
		"### Repository Inventory",
		"raw_paths_included=`false`",
		"raw_context_bodies_included=`false`",
		"### Runtime Boundary",
		"`/workspace` is inspect-only",
		"future private workspace memory or external mounts require reviewed specs",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("workspace report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"WORKSPACE_POLICY_BODY_SECRET", "WORKSPACE_SPEC_BODY_SECRET", "WORKSPACE_REPORT_BODY_SECRET", "GITCLAW_WORKSPACE_CONTEXT_V1", "This declarative workspace record"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("workspace report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestRenderWorkspaceCatalogReportShowsCommandAndLayerSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 131,
			"title": "@gitclaw /workspace catalog",
			"body": "Hidden workspace catalog body token: WORKSPACE_CATALOG_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderWorkspaceReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Workspace Catalog Report",
		"Generated without a model call",
		"requested_workspace_command: `catalog`",
		"workspace_command_status: `ok`",
		"issue_side_execution: `github_actions_workspace_metadata`",
		"workspace_catalog_status: `ok`",
		"catalog_strategy: `compact-github-actions-workspace-discovery`",
		"workspace_model: `github-actions-checkout-plus-repo-reviewed-policy`",
		"workspace_scope: `repository-checkout`",
		"workspace_policy_path: `.gitclaw/WORKSPACE.md`",
		"workspace_policy_present: `true`",
		"workspace_policy_loaded_for_model: `true`",
		"workspace_specs: `1`",
		"workspace_specs_with_frontmatter: `1`",
		"workspace_specs_requiring_approval: `1`",
		"git_available: `true`",
		"git_repository: `true`",
		"workflow_files_present: `1`",
		"checkout_action_versions: `actions/checkout@v5`",
		"setup_go_action_versions: `actions/setup-go@v6`",
		"fetch_depth_configured: `true`",
		"sandbox_backend: `github-actions`",
		"durable_state_backend: `git-tracked-files-and-backup-branch`",
		"catalog_entries: `4`",
		"workspace_layers: `8`",
		"workspace_daemon_supported: `false`",
		"long_running_socket_supported: `false`",
		"raw_workspace_bodies_included: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_workspace_catalog_change: `true`",
		"### Catalog Entries",
		"command=`catalog` issue_intent=`@gitclaw /workspace catalog` local_command=`gitclaw workspace catalog` execution=`metadata-only` gate=`body-free-output`",
		"command=`summary` issue_intent=`@gitclaw /workspace` local_command=`gitclaw workspace summary`",
		"command=`risk` issue_intent=`@gitclaw /workspace risk` local_command=`gitclaw workspace risk`",
		"### Workspace Layers",
		"layer=`policy` store=`.gitclaw/WORKSPACE.md`",
		"layer=`specs` store=`.gitclaw/workspaces/*.md`",
		"layer=`git` store=`.git`",
		"layer=`runtime` store=`GitHub Actions runner`",
		"layer=`durable-state` store=`git tracked files + backup branch`",
		"### Catalog Gates",
		"workspace_policy_gate=`repo-reviewed-policy-file`",
		"sandbox_gate=`workspace-is-not-sandbox`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("workspace catalog report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"WORKSPACE_POLICY_BODY_SECRET", "WORKSPACE_SPEC_BODY_SECRET", "WORKSPACE_CATALOG_BODY_SECRET", "GITCLAW_WORKSPACE_CONTEXT_V1", "This declarative workspace record"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("workspace catalog report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestWorkspaceSummaryCommandReportsWorkspace(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"workspace", "summary"}); err != nil {
			t.Fatalf("workspace summary returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Workspace Report", "scope: `local-cli`", "workspace_status: `ok`", "workspace_policy_loaded_for_model: `true`", "workspace_specs: `1`", "workflow_files_present: `1`", "checkout_action_versions: `actions/checkout@v5`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("workspace summary output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "WORKSPACE_POLICY_BODY_SECRET") || strings.Contains(output, "WORKSPACE_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("workspace summary leaked body or issue metadata:\n%s", output)
	}

	verifyOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"workdir", "verify"}); err != nil {
			t.Fatalf("workdir verify returned error: %v", err)
		}
	})
	if !strings.Contains(verifyOutput, "GitClaw Workspace Report") || !strings.Contains(verifyOutput, "workspace_status: `ok`") {
		t.Fatalf("workdir verify did not render workspace report:\n%s", verifyOutput)
	}
}

func TestWorkspaceCatalogCommandReportsSurfaceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"workspace", "catalog"}); err != nil {
			t.Fatalf("workspace catalog returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Workspace Catalog Report", "scope: `local-cli`", "workspace_catalog_status: `ok`", "catalog_entries: `4`", "workspace_layers: `8`", "command=`catalog` issue_intent=`@gitclaw /workspace catalog` local_command=`gitclaw workspace catalog`", "layer=`workflow` store=`.github/workflows/*.yml`", "raw_workspace_bodies_included: `false`", "model_e2e_gate=`required`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("workspace catalog output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "WORKSPACE_POLICY_BODY_SECRET") || strings.Contains(output, "WORKSPACE_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("workspace catalog leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleWorkspaceCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 130,
			"title": "@gitclaw /repo",
			"body": "Hidden workspace handler token: WORKSPACE_HANDLER_BODY_SECRET.",
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
		t.Fatalf("LLM called %d times for deterministic workspace command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Workspace Report", "Generated without a model call", "model=\"gitclaw/workspace\"", "workspace_status: `ok`", "workspace_policy_loaded_for_model: `true`", "checkout_action_versions: `actions/checkout@v5`", "raw_file_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("workspace handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"WORKSPACE_HANDLER_BODY_SECRET", "WORKSPACE_POLICY_BODY_SECRET", "WORKSPACE_SPEC_BODY_SECRET", "This declarative workspace record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("workspace handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[130], "gitclaw:done") || hasLabel(github.IssueLabels[130], "gitclaw:running") || hasLabel(github.IssueLabels[130], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[130])
	}
}

func TestHandleWorkspaceCatalogCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	initWorkspaceRepo(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 132,
			"title": "@gitclaw /workdir commands",
			"body": "Hidden workspace catalog handler token: WORKSPACE_CATALOG_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{132: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic workspace catalog command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Workspace Catalog Report", "Generated without a model call", "model=\"gitclaw/workspace\"", "requested_workspace_command: `catalog`", "workspace_catalog_status: `ok`", "catalog_entries: `4`", "workspace_layers: `8`", "command=`catalog` issue_intent=`@gitclaw /workspace catalog`", "layer=`policy` store=`.gitclaw/WORKSPACE.md`", "raw_workspace_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("workspace catalog handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"WORKSPACE_CATALOG_HANDLER_BODY_SECRET", "WORKSPACE_POLICY_BODY_SECRET", "WORKSPACE_SPEC_BODY_SECRET", "This declarative workspace record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("workspace catalog handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[132], "gitclaw:done") || hasLabel(github.IssueLabels[132], "gitclaw:running") || hasLabel(github.IssueLabels[132], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[132])
	}
}

func TestLoadRepoContextIncludesWorkspacePolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, workspacePolicyPath, workspacePolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == workspacePolicyPath {
			found = true
			if !strings.Contains(doc.Body, "WORKSPACE_POLICY_BODY_SECRET") {
				t.Fatalf("workspace policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("workspace policy file was not loaded into context: %#v", ctx.Documents)
	}
}

func initWorkspaceRepo(t *testing.T, root string) {
	t.Helper()
	runCheckpointTestGit(t, root, "init")
	runCheckpointTestGit(t, root, "checkout", "-b", "main")
	runCheckpointTestGit(t, root, "config", "user.name", "GitClaw Test")
	runCheckpointTestGit(t, root, "config", "user.email", "gitclaw@example.com")
	runCheckpointTestGit(t, root, "config", "commit.gpgsign", "false")
	writeTestFile(t, root, workspacePolicyPath, workspacePolicyTestBody)
	writeTestFile(t, root, ".gitclaw/workspaces/repository.md", workspaceSpecTestBody)
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", workspaceWorkflowTestBody)
	writeTestFile(t, root, "README.md", "workspace fixture\n")
	runCheckpointTestGit(t, root, "add", ".")
	runCheckpointTestGit(t, root, "commit", "-m", "initial workspace fixture")
}
