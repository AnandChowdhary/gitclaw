package gitclaw

import (
	"context"
	"strings"
	"testing"
)

const artifactPolicyTestBody = `# Artifacts

GITCLAW_ARTIFACTS_CONTEXT_V1

ARTIFACT_POLICY_BODY_SECRET
`

const artifactSpecTestBody = `---
name: prompt-artifact
kind: prompt
storage: github-actions-artifact
filename: prompt.md
workflow: .github/workflows/gitclaw.yml
label: gitclaw:e2e-prompt-artifact
retention_days: 7
redaction_required: true
requires_approval: true
---

# Prompt Artifact

This declarative artifact record must not be printed.

ARTIFACT_SPEC_BODY_SECRET
`

const artifactWorkflowTestBody = `name: GitClaw

on:
  issues:
    types: [opened]

jobs:
  handle:
    runs-on: ubuntu-latest
    steps:
      - run: |
          if [[ "${GITCLAW_PROMPT_ARTIFACT_ENABLED}" == "true" ]]; then
            export GITCLAW_PROMPT_ARTIFACT_PATH="${RUNNER_TEMP}/gitclaw-prompt-artifacts/prompt.md"
          fi
        env:
          GITCLAW_PROMPT_ARTIFACT_ENABLED: ${{ contains(github.event.issue.labels.*.name, 'gitclaw:e2e-prompt-artifact') }}
      - if: always() && contains(github.event.issue.labels.*.name, 'gitclaw:e2e-prompt-artifact')
        uses: actions/upload-artifact@v6
        with:
          name: gitclaw-issue-${{ github.event.issue.number }}-run-${{ github.run_id }}-prompt
          path: ${{ runner.temp }}/gitclaw-prompt-artifacts/prompt.md
          if-no-files-found: error
          retention-days: 7
`

func TestRenderArtifactReportAuditsArtifactSpecsWithoutBodies(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/artifacts/prompt-artifact.md", artifactSpecTestBody)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", artifactWorkflowTestBody)
	cfg := DefaultConfig()
	cfg.Workdir = dir

	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /artifacts",
			"body": "Hidden artifacts report body token: ARTIFACTS_REPORT_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	report := RenderArtifactReport(ev, cfg)
	for _, want := range []string{
		"GitClaw Artifacts Report",
		"Generated without a model call",
		"artifacts_status: `ok`",
		"artifact_policy_path: `.gitclaw/ARTIFACTS.md`",
		"artifact_policy_present: `true`",
		"artifact_policy_loaded_for_model: `true`",
		"artifact_specs_dir: `.gitclaw/artifacts`",
		"artifact_specs: `1`",
		"artifact_specs_with_frontmatter: `1`",
		"artifact_specs_requiring_approval: `1`",
		"artifact_specs_requiring_redaction: `1`",
		"artifact_retention_days_declared: `7`",
		"github_actions_artifact_uploaders: `1`",
		"upload_artifact_versions: `actions/upload-artifact@v6`",
		"prompt_artifact_default_enabled: `false`",
		"prompt_artifact_label: `gitclaw:e2e-prompt-artifact`",
		"prompt_artifact_env_path_configured: `false`",
		"artifact_storage_backend: `github-actions-artifacts`",
		"durable_backup_backend: `git-backup-branch`",
		"artifact_body_printing_allowed: `false`",
		"model_call_required: `false`",
		"repository_mutation_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_artifact_bodies_included: `false`",
		"llm_e2e_required_after_change: `true`",
		"### Artifact Policy",
		"`.gitclaw/ARTIFACTS.md` loaded=`true` source=`contextDocumentPaths`",
		"### Artifact Specs",
		"name=`prompt-artifact`",
		"path=`.gitclaw/artifacts/prompt-artifact.md`",
		"frontmatter=`true`",
		"kind=`prompt`",
		"storage=`github-actions-artifact`",
		"filename=`prompt.md`",
		"workflow=`.github/workflows/gitclaw.yml`",
		"label=`gitclaw:e2e-prompt-artifact`",
		"retention_days=`7`",
		"redaction_required=`true`",
		"requires_approval=`true`",
		"### Workflow Artifact Uploads",
		"path=`.github/workflows/gitclaw.yml`",
		"upload_artifact_actions=`actions/upload-artifact@v6`",
		"if_no_files_found_error=`true`",
		"prompt_artifact_label_gate=`true`",
		"prompt_artifact_path_env=`true`",
		"### Runtime Boundary",
		"GitHub Actions artifacts are short-lived evidence bundles",
		"future artifact types require body-free audit cards",
		"### Verification Findings",
		"- none",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("artifact report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"ARTIFACT_POLICY_BODY_SECRET", "ARTIFACT_SPEC_BODY_SECRET", "ARTIFACTS_REPORT_BODY_SECRET", "GITCLAW_ARTIFACTS_CONTEXT_V1", "This declarative artifact record"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("artifact report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestArtifactsListCommandReportsArtifacts(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/artifacts/prompt-artifact.md", artifactSpecTestBody)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", artifactWorkflowTestBody)
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"artifacts", "list"}); err != nil {
			t.Fatalf("artifacts list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Artifacts Report", "scope: `local-cli`", "artifacts_status: `ok`", "artifact_policy_loaded_for_model: `true`", "artifact_specs: `1`", "github_actions_artifact_uploaders: `1`", "upload_artifact_versions: `actions/upload-artifact@v6`", "artifact_body_printing_allowed: `false`", "model_call_required: `false`", "### Verification Findings", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("artifacts list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "ARTIFACT_POLICY_BODY_SECRET") || strings.Contains(output, "ARTIFACT_SPEC_BODY_SECRET") || strings.Contains(output, "issue: `#0`") {
		t.Fatalf("artifacts list leaked body or issue metadata:\n%s", output)
	}
}

func TestHandleArtifactsCommandPostsReportWithoutLLM(t *testing.T) {
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	writeTestFile(t, dir, ".gitclaw/artifacts/prompt-artifact.md", artifactSpecTestBody)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", artifactWorkflowTestBody)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /artifact",
			"body": "Hidden artifacts handler token: ARTIFACTS_HANDLER_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{126: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic artifacts command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Artifacts Report", "Generated without a model call", "model=\"gitclaw/artifacts\"", "artifacts_status: `ok`", "artifact_policy_loaded_for_model: `true`", "artifact_specs: `1`", "github_actions_artifact_uploaders: `1`", "raw_artifact_bodies_included: `false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("artifacts handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"ARTIFACTS_HANDLER_BODY_SECRET", "ARTIFACT_POLICY_BODY_SECRET", "ARTIFACT_SPEC_BODY_SECRET", "This declarative artifact record"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("artifacts handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}

func TestLoadRepoContextIncludesArtifactPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, artifactPolicyPath, artifactPolicyTestBody)
	ctx, err := LoadRepoContext(dir, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	found := false
	for _, doc := range ctx.Documents {
		if doc.Path == artifactPolicyPath {
			found = true
			if !strings.Contains(doc.Body, "ARTIFACT_POLICY_BODY_SECRET") {
				t.Fatalf("artifact policy body was not loaded into context: %#v", doc)
			}
		}
	}
	if !found {
		t.Fatalf("artifact policy file was not loaded into context: %#v", ctx.Documents)
	}
}
