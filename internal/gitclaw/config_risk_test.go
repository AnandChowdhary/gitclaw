package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestConfigRiskCommandReportsCurrentSafeSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSafeConfigRiskFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"config", "risk"}); err != nil {
			t.Fatalf("config risk returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Config Risk Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"config_risk_status: `ok`",
		"verification_scope: `repo_local_config_control_plane`",
		"config_source: `defaults+repo+environment`",
		"config_file_path: `.gitclaw/config.yml`",
		"config_file_present: `true`",
		"workflow_files_expected: `7`",
		"workflow_files_present: `7`",
		"workflow_files_missing: `0`",
		"trigger_mode: `label-or-prefix`",
		"trigger_label: `gitclaw`",
		"trigger_prefix: `@gitclaw`",
		"disabled_label: `gitclaw:disabled`",
		"trusted_associations: `COLLABORATOR, MEMBER, OWNER`",
		"trusted_associations_configured: `3`",
		"broad_trusted_associations: `none`",
		"broad_trusted_associations_configured: `0`",
		"managed_labels_configured: `9`",
		"duplicate_managed_labels: `0`",
		"model_provider: `github-models`",
		"model: `openai/gpt-5-nano`",
		"model_fallbacks: `openai/gpt-4.1-nano`",
		"model_fallbacks_configured: `1`",
		"run_mode: `read-only`",
		"max_prompt_bytes: `60000`",
		"max_output_tokens: `4000`",
		"max_transcript_messages: `40`",
		"max_transcript_message_bytes: `8000`",
		"skills_allowed_configured: `0`",
		"skills_disabled_configured: `0`",
		"skill_gate_conflicts: `0`",
		"tools_allowed_configured: `0`",
		"tools_disabled_configured: `0`",
		"tool_gate_conflicts: `0`",
		"slash_commands: `34`",
		"surfaces_with_risk_findings: `0`",
		"config_risk_findings: `0`",
		"high_risk_findings: `0`",
		"warning_risk_findings: `0`",
		"info_risk_findings: `0`",
		"raw_config_bodies_included: `false`",
		"raw_workflow_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_provider_error_bodies_included: `false`",
		"credential_values_included: `false`",
		"repository_mutation_allowed: `false`",
		"agent_authored_config_mutation_supported: `false`",
		"llm_e2e_required_after_config_risk_change: `true`",
		"### Config File Risk Card",
		"kind=`config-file` path=`.gitclaw/config.yml` present=`true`",
		"### Workflow Risk Cards",
		"kind=`workflow-file` path=`.github/workflows/gitclaw.yml` present=`true`",
		"### Trigger And Trust Risk Card",
		"kind=`trigger-trust` trigger_mode=`label-or-prefix` trigger_label=`gitclaw` trigger_prefix=`@gitclaw` disabled_label=`gitclaw:disabled`",
		"### Model And Budget Risk Card",
		"kind=`model-budget` model_provider=`github-models` model=`openai/gpt-5-nano`",
		"### Gate Risk Card",
		"kind=`gate` skills_allowed_configured=`0` skills_disabled_configured=`0`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Current Config Request Risk Card",
		"scope=`local-cli` current_issue_config_request=`false`",
		"### Risk Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("config risk output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"api_key", "GITCLAW_CONFIG_RISK_SECRET", "permissions:", "contents: read", "workflow_dispatch:"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("config risk output leaked body text %q:\n%s", notWant, output)
		}
	}
}

func TestRenderConfigRiskReportFlagsUnsafeConfigWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  label: gitclaw
  disabled_label: gitclaw
model:
  provider: openai-compatible
  model: custom-model
  max_prompt_bytes: 0
  max_output_tokens: 0
  api_key: GITCLAW_CONFIG_RISK_SECRET
actions:
  mode: write
webhook_url: https://example.test/hook
`)
	writeTestFile(t, root, ".github/workflows/gitclaw.yml", `name: GitClaw
on:
  pull_request_target:
permissions: write-all
jobs:
  risky:
    runs-on: ubuntu-latest
    steps:
      - run: |
          printenv
          while true; do sleep 1; done
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg.ConfigSource = "defaults+repo"
	cfg.TriggerLabel = "gitclaw"
	cfg.DisabledLabel = "gitclaw"
	cfg.AllowedAssociations = map[string]bool{"FIRST_TIMER": true, "NONE": true}
	cfg.ModelProvider = "openai-compatible"
	cfg.Model = "custom-model"
	cfg.ModelFallbacks = nil
	cfg.MaxPromptBytes = 0
	cfg.MaxOutputTokens = 0
	cfg.MaxTranscriptMessages = 0
	cfg.MaxTranscriptMessageBytes = 0
	cfg.AllowedSkills = map[string]bool{"repo-reader": true}
	cfg.DisabledSkills = map[string]bool{"repo-reader": true}
	cfg.AllowedTools = map[string]bool{"gitclaw.search_files": true}
	cfg.DisabledTools = map[string]bool{"gitclaw.search_files": true}

	output := RenderConfigRiskCLIReport(cfg)
	for _, want := range []string{
		"GitClaw Config Risk Report",
		"config_risk_status: `high`",
		"workflow_files_expected: `7`",
		"workflow_files_present: `1`",
		"workflow_files_missing: `6`",
		"trigger_mode: `label-or-prefix`",
		"trusted_associations: `FIRST_TIMER, NONE`",
		"broad_trusted_associations: `FIRST_TIMER, NONE`",
		"model_provider: `openai-compatible`",
		"model_fallbacks: `none`",
		"skill_gate_conflicts: `1`",
		"tool_gate_conflicts: `1`",
		"high_risk_findings: `8`",
		"warning_risk_findings: `16`",
		"info_risk_findings: `1`",
		"config_risk_findings: `25`",
		"code=`broad_trusted_association`",
		"code=`config_write_mode_enabled`",
		"code=`credential_material_in_config`",
		"code=`external_webhook_configured`",
		"code=`max_output_tokens_not_positive`",
		"code=`max_prompt_bytes_not_positive`",
		"code=`max_transcript_message_bytes_not_positive`",
		"code=`max_transcript_messages_not_positive`",
		"code=`managed_label_collision`",
		"code=`model_fallbacks_not_configured`",
		"code=`non_github_models_provider`",
		"code=`pull_request_target_trigger`",
		"code=`skill_gate_conflict`",
		"code=`tool_gate_conflict`",
		"code=`trigger_disabled_label_collision`",
		"code=`workflow_file_missing`",
		"code=`workflow_raw_secret_echo`",
		"code=`workflow_unbounded_background_process`",
		"code=`workflow_write_all_permissions`",
		"line_sha256_12=",
		"risk_max_severity=`high`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("config risk output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"GITCLAW_CONFIG_RISK_SECRET", "api_key:", "webhook_url:", "permissions: write-all", "printenv", "while true"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("config risk output leaked body text %q:\n%s", leaked, output)
		}
	}
}

func TestRenderConfigRiskReportFlagsInvalidTriggerMode(t *testing.T) {
	root := t.TempDir()
	writeSafeConfigRiskFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	cfg.TriggerMode = "socket-daemon"

	output := RenderConfigRiskCLIReport(cfg)
	for _, want := range []string{
		"config_risk_status: `high`",
		"trigger_mode: `socket-daemon`",
		"code=`trigger_mode_invalid`",
		"field=`trigger_mode`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("config risk output missing %q:\n%s", want, output)
		}
	}
}

func TestHandleConfigRiskCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeSafeConfigRiskFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 164,
			"title": "@gitclaw /config risk",
			"body": "Hidden config risk body token: CONFIG_RISK_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg, err := LoadEffectiveConfig()
	if err != nil {
		t.Fatalf("LoadEffectiveConfig returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{164: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic config risk command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Config Risk Report",
		"Generated without a model call",
		"model=\"gitclaw/config\"",
		"config_risk_status: `ok`",
		"verification_scope: `repo_local_config_control_plane`",
		"workflow_files_present: `7`",
		"workflow_files_missing: `0`",
		"trigger_mode: `label-or-prefix`",
		"trusted_associations: `COLLABORATOR, MEMBER, OWNER`",
		"model_fallbacks: `openai/gpt-4.1-nano`",
		"config_risk_findings: `0`",
		"current_issue_config_request=`true`",
		"issue_body_scanned=`false`",
		"comment_bodies_scanned=`false`",
		"llm_e2e_required_after_config_risk_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("config risk report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"CONFIG_RISK_HANDLER_BODY_SECRET", "contents: read", "workflow_dispatch:"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("config risk report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[164], "gitclaw:done") || hasLabel(github.IssueLabels[164], "gitclaw:running") || hasLabel(github.IssueLabels[164], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[164])
	}
}

func writeSafeConfigRiskFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  mode: label-or-prefix
  label: gitclaw
  prefix: "@gitclaw"
  disabled_label: gitclaw:disabled

authorization:
  allowed_associations:
    - OWNER
    - MEMBER
    - COLLABORATOR

model:
  provider: github-models
  model: openai/gpt-5-nano
  fallbacks:
    - openai/gpt-4.1-nano
  base_url: https://models.github.ai/inference/chat/completions
  max_prompt_bytes: 60000
  max_output_tokens: 4000
  max_transcript_messages: 40
  max_transcript_message_bytes: 8000

actions:
  mode: read_only
`)
	for _, path := range configWorkflowPaths {
		writeTestFile(t, root, path, `name: GitClaw
on:
  workflow_dispatch:
permissions:
  contents: read
  issues: write
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`)
	}
}
