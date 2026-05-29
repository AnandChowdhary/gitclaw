package gitclaw

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreflightCommandWritesOutputsWithoutLLMSecret(t *testing.T) {
	dir := t.TempDir()
	eventPath := filepath.Join(dir, "event.json")
	outputPath := filepath.Join(dir, "output")
	eventJSON := `{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 42,
			"title": "@gitclaw explain auth",
			"body": "How does auth work?",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": []
		},
		"sender": {"login": "alice", "type": "User"}
	}`
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_EVENT_NAME", "issues")
	t.Setenv("GITHUB_OUTPUT", outputPath)
	t.Setenv("GITCLAW_LLM_API_KEY", "")

	if err := RunCLI(context.Background(), []string{"preflight", "--event", eventPath}); err != nil {
		t.Fatalf("preflight returned error: %v", err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "allowed=true") {
		t.Fatalf("GITHUB_OUTPUT missing allowed=true: %s", output)
	}
}

func TestSkillsValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SECRET_SKILL_BODY_TOKEN
`)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "validate"}); err != nil {
			t.Fatalf("skills validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skills Validate Report", "skill_validation_status: `ok`", "skill_validation_errors: `0`", "skill_validation_warnings: `0`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills validate output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "SECRET_SKILL_BODY_TOKEN") {
		t.Fatalf("skills validate leaked skill body:\n%s", output)
	}
}

func TestSkillsInfoCommandReportsOneSkill(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SECRET_SKILLS_INFO_CLI_BODY
`)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "info", "repo-reader"}); err != nil {
			t.Fatalf("skills info returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skill Info Report", "scope: `local-cli`", "requested_skill: `repo-reader`", "skill_info_status: `ok`", "matched_skills: `1`", "skill_name=`repo-reader`", "selected_for_this_turn=`true`", ".gitclaw/SKILLS/repo-reader/SKILL.md", "missing_env=`0`", "missing_bins=`0`", "### Validation For Matches", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills info output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "SECRET_SKILLS_INFO_CLI_BODY") {
		t.Fatalf("skills info leaked skill body:\n%s", output)
	}
}

func TestSkillsSearchCommandReportsMetadataMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context and deterministic tool outputs.
---

SECRET_SKILLS_SEARCH_CLI_BODY
`)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "search", "repository", "context", "CLI_QUERY_SECRET"}); err != nil {
			t.Fatalf("skills search returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skills Search Report", "scope: `local-cli`", "skill_search_status: `ok`", "query_sha256_12:", "available_skills: `1`", "matched_skills: `1`", "raw_bodies_included: `false`", "skill_name=`repo-reader`", "match_fields=`description`", ".gitclaw/SKILLS/repo-reader/SKILL.md", "selected_for_this_turn=`true`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills search output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SECRET_SKILLS_SEARCH_CLI_BODY", "CLI_QUERY_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("skills search leaked %q:\n%s", leaked, output)
		}
	}
}

func TestSkillsListCommandReportsInventoryWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

SECRET_SKILLS_LIST_CLI_BODY
`)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "list"}); err != nil {
			t.Fatalf("skills list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skills Report", "scope: `local-cli`", "available_skills: `1`", "selected_skills: `0`", "skills_with_frontmatter: `1`", "skills_with_description: `1`", "skill_validation_status: `ok`", "### Available Skills", "name=`repo-reader`", ".gitclaw/SKILLS/repo-reader/SKILL.md", "description=`Use read-only repository context.`", "sha256_12=", "### Selected For This Turn", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "SECRET_SKILLS_LIST_CLI_BODY") {
		t.Fatalf("skills list leaked skill body:\n%s", output)
	}
}

func TestSoulValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "SOUL_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/IDENTITY.md", "IDENTITY_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/USER.md", "USER_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "MEMORY_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/memory/2026-05-29.md", "DATED_MEMORY_BODY_TOKEN")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"soul", "validate"}); err != nil {
			t.Fatalf("soul validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Soul Validate Report", "soul_validation_status: `ok`", "soul_validation_errors: `0`", "soul_validation_warnings: `0`", "soul_required_files_present: `6`", "soul_memory_notes: `1`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("soul validate output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SOUL_BODY_TOKEN", "USER_BODY_TOKEN", "MEMORY_BODY_TOKEN", "DATED_MEMORY_BODY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("soul validate leaked body token %q:\n%s", leaked, output)
		}
	}
}

func TestSoulListCommandReportsInventoryWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "SOUL_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/IDENTITY.md", "IDENTITY_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/USER.md", "USER_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "MEMORY_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/HEARTBEAT.md", "HEARTBEAT_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/memory/2026-05-29.md", "DATED_MEMORY_LIST_BODY_TOKEN")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"soul", "list"}); err != nil {
			t.Fatalf("soul list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Soul Report", "scope: `local-cli`", "identity_policy_files: `6`", "memory_notes: `1`", "soul_validation_status: `ok`", "soul_required_files_present: `6`", "soul_memory_notes: `1`", "### Identity And Policy Files", ".gitclaw/SOUL.md", ".gitclaw/IDENTITY.md", ".gitclaw/USER.md", ".gitclaw/TOOLS.md", ".gitclaw/MEMORY.md", ".gitclaw/HEARTBEAT.md", "### Memory Notes", ".gitclaw/memory/2026-05-29.md", "sha256_12=", "### Validation", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("soul list output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SOUL_LIST_BODY_TOKEN", "USER_LIST_BODY_TOKEN", "MEMORY_LIST_BODY_TOKEN", "DATED_MEMORY_LIST_BODY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("soul list leaked body token %q:\n%s", leaked, output)
		}
	}
}

func TestSoulSearchCommandReportsHashedMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "Repo-native operating boundary CLI_SOUL_SEARCH_BODY_TOKEN.\n")
	writeTestFile(t, dir, ".gitclaw/USER.md", "User operating preference CLI_SOUL_SEARCH_USER_TOKEN.\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"soul", "search", "ignored", "--query", "operating CLI_SOUL_SEARCH_QUERY_TOKEN", "--max-results", "1"}); err != nil {
			t.Fatalf("soul search returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Soul Search Report", "scope: `local-cli`", "soul_search_status: `ok`", "query_sha256_12:", "query_terms:", "max_results: `1`", "files_scanned: `2`", "matched_lines: `2`", "results_returned: `1`", "raw_bodies_included: `false`", "path=`.gitclaw/SOUL.md`", "category=`soul`", "line_sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("soul search output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"CLI_SOUL_SEARCH_BODY_TOKEN", "CLI_SOUL_SEARCH_USER_TOKEN", "CLI_SOUL_SEARCH_QUERY_TOKEN", "operating CLI_SOUL_SEARCH_QUERY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("soul search leaked %q:\n%s", leaked, output)
		}
	}
}

func TestMemoryValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "MEMORY_VALIDATE_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/memory/2026-05-29.md", "DATED_MEMORY_VALIDATE_BODY_TOKEN")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"memory", "validate"}); err != nil {
			t.Fatalf("memory validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Memory Validate Report", "scope: `local-cli`", "memory_validation_status: `ok`", "memory_validation_errors: `0`", "memory_validation_warnings: `0`", "long_term_memory_present: `true`", "long_term_memory_loaded: `true`", "dated_memory_notes: `1`", "canonical_dated_memory_notes: `1`", "noncanonical_dated_memory_notes: `0`", "loaded_memory_notes: `1`", "empty_memory_files: `0`", "potential_secret_findings: `0`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory validate output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"MEMORY_VALIDATE_BODY_TOKEN", "DATED_MEMORY_VALIDATE_BODY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("memory validate leaked body token %q:\n%s", leaked, output)
		}
	}
}

func TestMemoryListCommandReportsInventoryWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "MEMORY_LIST_BODY_TOKEN")
	writeTestFile(t, dir, ".gitclaw/memory/2026-05-29.md", "DATED_MEMORY_LIST_BODY_TOKEN")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"memory", "list"}); err != nil {
			t.Fatalf("memory list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Memory Report", "scope: `local-cli`", "memory_mode: `read-only`", "long_term_memory_present: `true`", "long_term_memory_loaded: `true`", "dated_memory_notes: `1`", "canonical_dated_memory_notes: `1`", "noncanonical_dated_memory_notes: `0`", "loaded_memory_notes: `1`", "latest_memory_note: `.gitclaw/memory/2026-05-29.md`", "memory_validation_status: `ok`", "memory_files_at_limit: `0`", "### Long-Term Memory", ".gitclaw/MEMORY.md", "### Dated Memory Notes", ".gitclaw/memory/2026-05-29.md", "sha256_12=", "### Validation", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory list output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"MEMORY_LIST_BODY_TOKEN", "DATED_MEMORY_LIST_BODY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("memory list leaked body token %q:\n%s", leaked, output)
		}
	}
}

func TestMemorySearchCommandReportsHashedMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "Repository deployment preference CLI_MEMORY_SEARCH_BODY_TOKEN.\n")
	writeTestFile(t, dir, ".gitclaw/memory/2026-05-29.md", "Deployment rollout note CLI_MEMORY_SEARCH_NOTE_TOKEN.\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"memory", "search", "ignored", "--query", "deployment CLI_MEMORY_SEARCH_QUERY_TOKEN", "--max-results", "1"}); err != nil {
			t.Fatalf("memory search returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Memory Search Report", "scope: `local-cli`", "memory_search_status: `ok`", "query_sha256_12:", "query_terms:", "max_results: `1`", "files_scanned: `2`", "matched_lines: `2`", "results_returned: `1`", "raw_bodies_included: `false`", "path=`.gitclaw/MEMORY.md`", "line_sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory search output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"CLI_MEMORY_SEARCH_BODY_TOKEN", "CLI_MEMORY_SEARCH_NOTE_TOKEN", "CLI_MEMORY_SEARCH_QUERY_TOKEN", "deployment CLI_MEMORY_SEARCH_QUERY_TOKEN"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("memory search leaked %q:\n%s", leaked, output)
		}
	}
}

func TestToolsValidateCommandReportsCurrentRepoShape(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_BODY_TOKEN")
	writeTestFile(t, dir, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "validate"}); err != nil {
			t.Fatalf("tools validate returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tools Validate Report", "tool_validation_status: `ok`", "tool_validation_errors: `0`", "tool_validation_warnings: `0`", "tool_contracts: `5`", "tool_active_outputs: `1`", "tool_guidance_files: `1`", "tool_missing_guidance: `0`", "tool_duplicate_contracts: `0`", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools validate output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "TOOLS_BODY_TOKEN") || strings.Contains(output, "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("tools validate leaked body/output token:\n%s", output)
	}
}

func TestToolsListCommandReportsInventoryWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_LIST_BODY_TOKEN")
	writeTestFile(t, dir, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "list"}); err != nil {
			t.Fatalf("tools list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tools Report", "scope: `local-cli`", "available_tools: `5`", "active_tool_outputs: `1`", "tool_validation_status: `ok`", "tool_contracts: `5`", "tool_active_outputs: `1`", "tool_guidance_files: `1`", "tool_missing_guidance: `0`", "### Available Tools", "gitclaw.list_files", "gitclaw.search_files", "gitclaw.read_file", "gitclaw.skill_index", "gitclaw.policy", "### Tool Guidance Files", ".gitclaw/TOOLS.md", "### Active Tool Outputs", "input=`.`", "sha256_12=", "### Validation", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools list output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "TOOLS_LIST_BODY_TOKEN") || strings.Contains(output, "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("tools list leaked body/output token:\n%s", output)
	}
}

func TestToolsSearchCommandReportsHashedMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "TOOLS_SEARCH_BODY_TOKEN")
	writeTestFile(t, dir, "go.mod", "module github.com/AnandChowdhary/gitclaw\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"tools", "search", "ignored", "--query", "read_file CLI_TOOLS_SEARCH_QUERY_TOKEN", "--max-results", "2"}); err != nil {
			t.Fatalf("tools search returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Tools Search Report", "scope: `local-cli`", "tool_search_status: `ok`", "query_sha256_12:", "query_terms:", "max_results: `2`", "available_tools: `5`", "active_tool_outputs:", "matched_contracts: `1`", "results_returned:", "raw_bodies_included: `false`", "raw_inputs_included: `false`", "kind=`contract` name=`gitclaw.read_file`", "match_fields=`name`", "mode=`read-only`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("tools search output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"TOOLS_SEARCH_BODY_TOKEN", "CLI_TOOLS_SEARCH_QUERY_TOKEN", "read_file CLI_TOOLS_SEARCH_QUERY_TOKEN", "module github.com/AnandChowdhary/gitclaw"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("tools search leaked %q:\n%s", leaked, output)
		}
	}
}

func TestChannelsListCommandReportsWorkflowDispatchBridge(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".github/workflows/gitclaw-channel-ingest.yml", `name: GitClaw Channel Ingest
on:
  workflow_dispatch:
    inputs:
      channel:
        required: true
      thread_id:
        required: true
      message_id:
        required: true
      author:
        required: false
      body:
        required: true
permissions:
  actions: write
  issues: write
jobs:
  ingest:
    steps:
      - run: echo CHANNEL_WORKFLOW_BODY_TOKEN
`)
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"channels", "list"}); err != nil {
			t.Fatalf("channels list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Channel Report", "scope: `local-cli`", "channel_label: `gitclaw:channel`", "trigger_label: `gitclaw`", "workflow_path: `.github/workflows/gitclaw-channel-ingest.yml`", "workflow_present: `true`", "workflow_dispatch_trigger: `true`", "permissions_actions_write: `true`", "permissions_issues_write: `true`", "workflow_inputs: `5`", "supported_providers: `telegram, slack, generic`", "wake_strategy: `workflow_dispatch`", "telegram", "slack", "generic", "gitclaw channel-ingest", "dispatch id: `<channel>-<message_id>`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("channels list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "channel_thread_issue:", "channel_message_comments_now:", "CHANNEL_WORKFLOW_BODY_TOKEN"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("channels list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestModelsListCommandReportsProviderWithoutCallingModel(t *testing.T) {
	t.Setenv("GITCLAW_WORKDIR", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "MODELS_LIST_SECRET_TOKEN")
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"models", "list"}); err != nil {
			t.Fatalf("models list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Model Report", "scope: `local-cli`", "Generated without a model call", "provider: `github-models`", "model: `openai/gpt-5-mini`", "endpoint_host: `models.github.ai`", "token_source: `GITHUB_TOKEN`", "request_timeout_seconds: `60`", "retry_max_attempts: `5`", "retry_base_delay_seconds: `5`", "retry_max_delay_seconds: `60`", "retryable_statuses: `429, 408, 5xx`", "prompt_artifact_enabled: `false`", "GITCLAW_MODEL", "GITCLAW_LLM_BASE_URL"} {
		if !strings.Contains(output, want) {
			t.Fatalf("models list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "issue_title_sha256_12", "MODELS_LIST_SECRET_TOKEN"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("models list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestConfigListCommandReportsEffectiveConfigWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/config.yml", `trigger:
  label: gitclaw
  prefix: "@gitclaw"
model:
  model: openai/gpt-5-mini
  max_prompt_bytes: 60000
`)
	writeTestFile(t, dir, ".github/workflows/gitclaw.yml", "name: GitClaw\n# CONFIG_LIST_WORKFLOW_BODY\n")
	writeTestFile(t, dir, ".github/workflows/gitclaw-heartbeat.yml", "name: GitClaw Heartbeat\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"config", "list"}); err != nil {
			t.Fatalf("config list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Config Report", "scope: `local-cli`", "Generated without a model call", "config_source: `defaults+repo+environment`", "config_file_path: `.gitclaw/config.yml`", "config_file_present: `true`", "trigger_label: `gitclaw`", "trigger_prefix: `@gitclaw`", "disabled_label: `gitclaw:disabled`", "model: `openai/gpt-5-mini`", "run_mode: `read-only`", "max_prompt_bytes: `60000`", "max_output_tokens: `4000`", "max_transcript_messages: `40`", "max_transcript_message_bytes: `8000`", "workflows_present: `2`", "slash_commands: `15`", "OWNER", "COLLABORATOR", "gitclaw:disabled", "/channels", "/config", "/models", ".gitclaw/config.yml", ".github/workflows/gitclaw.yml", ".github/workflows/gitclaw-heartbeat.yml", "sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("config list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "issue_title_sha256_12", "CONFIG_LIST_WORKFLOW_BODY"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("config list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestPolicyListCommandReportsStaticPolicyWithoutIssueFields(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", "module example.com/policy-list\nPOLICY_LIST_REPO_TOKEN\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"policy", "list"}); err != nil {
			t.Fatalf("policy list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Policy Report", "scope: `local-cli`", "Generated without a model call", "run_mode: `read-only`", "model: `openai/gpt-5-mini`", "### Trusted Associations", "OWNER", "MEMBER", "COLLABORATOR", "### Managed Labels", "gitclaw:disabled", "gitclaw:write-requested", "gitclaw:heartbeat", "gitclaw:channel", "gitclaw:proactive", "### Expected Workflow Permissions", "`preflight`: `contents:read`, `issues:read`", "`handle`: `contents:read`, `issues:write`, `models:read`", "`backup`: `contents:write`, `issues:read`", "### Active Policy Outputs", "- none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("policy list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "event_kind:", "preflight_allowed:", "actor_association:", "write_request_detected:", "Event Labels", "POLICY_LIST_REPO_TOKEN"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("policy list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestContextListCommandReportsRepoContextWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "CONTEXT_LIST_SOUL_BODY")
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "CONTEXT_LIST_MEMORY_BODY")
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "CONTEXT_LIST_TOOLS_BODY")
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---

CONTEXT_LIST_SKILL_BODY
`)
	writeTestFile(t, dir, "go.mod", "module example.com/context-list\nCONTEXT_LIST_REPO_TOKEN\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"context", "list"}); err != nil {
			t.Fatalf("context list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Context Report", "scope: `local-cli`", "Generated without a model call", "transcript_messages: `0`", "max_prompt_bytes: `60000`", "max_transcript_messages: `40`", "max_transcript_message_bytes: `8000`", "### Context Files", ".gitclaw/SOUL.md", ".gitclaw/MEMORY.md", ".gitclaw/TOOLS.md", "### Selected Skills", "- none", "### Tool Outputs", "gitclaw.list_files", "gitclaw.skill_index", "sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("context list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "CONTEXT_LIST_SOUL_BODY", "CONTEXT_LIST_MEMORY_BODY", "CONTEXT_LIST_TOOLS_BODY", "CONTEXT_LIST_SKILL_BODY", "CONTEXT_LIST_REPO_TOKEN", "module example.com/context-list"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("context list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestPromptListCommandReportsPromptBudgetWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SOUL.md", "PROMPT_LIST_SOUL_BODY")
	writeTestFile(t, dir, ".gitclaw/MEMORY.md", "PROMPT_LIST_MEMORY_BODY")
	writeTestFile(t, dir, ".gitclaw/TOOLS.md", "PROMPT_LIST_TOOLS_BODY")
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
always: true
---

PROMPT_LIST_SKILL_BODY
`)
	writeTestFile(t, dir, "go.mod", "module example.com/prompt-list\nPROMPT_LIST_REPO_TOKEN\n")
	t.Setenv("GITCLAW_WORKDIR", dir)
	t.Setenv("GITCLAW_PROMPT_ARTIFACT_PATH", "")
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"prompt", "list"}); err != nil {
			t.Fatalf("prompt list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Prompt Report", "scope: `local-cli`", "Generated without a model call", "provider: `github-models`", "model: `openai/gpt-5-mini`", "system_prompt_sha256_12:", "prompt_bytes:", "prompt_lines:", "prompt_sha256_12:", "max_prompt_bytes: `60000`", "max_output_tokens: `4000`", "max_transcript_messages: `40`", "max_transcript_message_bytes: `8000`", "transcript_messages: `0`", "bounded_transcript_messages: `0`", "omitted_older_messages: `0`", "truncated_transcript_bodies: `0`", "prompt_contains_truncation_marker: `false`", "prompt_artifact_enabled: `false`", "prompt_body_included: `false`", "### Prompt Inputs", "context_files:", "selected_skills: `1`", "available_skills: `1`", "tool_outputs:", "### Context Files", ".gitclaw/SOUL.md", ".gitclaw/MEMORY.md", ".gitclaw/TOOLS.md", "### Selected Skills", ".gitclaw/SKILLS/repo-reader/SKILL.md", "### Tool Outputs", "gitclaw.list_files", "gitclaw.skill_index", "sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"repository:", "issue:", "issue_title_sha256_12", "PROMPT_LIST_SOUL_BODY", "PROMPT_LIST_MEMORY_BODY", "PROMPT_LIST_TOOLS_BODY", "PROMPT_LIST_SKILL_BODY", "PROMPT_LIST_REPO_TOKEN", "module example.com/prompt-list"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("prompt list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestSessionListCommandReportsBackupTranscriptWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	backup := IssueBackup{
		Version:   1,
		Repo:      "owner/repo",
		EventName: "issue_comment",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw session list",
			Body:   "SESSION_LIST_ISSUE_BODY_TOKEN",
		},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_LIST_ISSUE_TRANSCRIPT_TOKEN", Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true},
			{Role: "assistant", Body: "SESSION_LIST_ASSISTANT_TRANSCRIPT_TOKEN", Actor: "github-actions[bot]", AuthorAssociation: "NONE", CommentID: 21, Trusted: true},
			{Role: "user", Body: "SESSION_LIST_COMMENT_TRANSCRIPT_TOKEN", Actor: "alice", AuthorAssociation: "MEMBER", CommentID: 22, Trusted: true},
		},
		Comments: []IssueBackupComment{
			{ID: 21, Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nSESSION_LIST_ASSISTANT_COMMENT_TOKEN", Author: "github-actions[bot]", AuthorAssociation: "NONE"},
			{ID: 22, Body: "@gitclaw /session list\nSESSION_LIST_USER_COMMENT_TOKEN", Author: "alice", AuthorAssociation: "MEMBER"},
		},
	}
	writeBackupFixture(t, dir, backup)
	backupPath := issueBackupPath(dir, "owner/repo", 7)
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"session", "list", "--backup", backupPath}); err != nil {
			t.Fatalf("session list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Session Report", "scope: `local-backup`", "backup_file:", "backup_repo: `owner/repo`", "backup_issue: `#7`", "event_kind: `issue_comment`", "raw_comments: `2`", "transcript_messages: `3`", "user_messages: `2`", "assistant_messages: `1`", "trusted_messages: `3`", "untrusted_messages: `0`", "assistant_turn_comments: `1`", "heartbeat_comments: `0`", "error_marker_comments: `0`", "channel_message_comments: `0`", "channel_thread_issue: `false`", "proactive_run_issue: `false`", "### Transcript Messages", "source=`issue`", "source=`comment:21`", "source=`comment:22`", "sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("session list output missing %q:\n%s", want, output)
		}
	}
	for _, notWant := range []string{"- repository:", "- issue:", "SESSION_LIST_ISSUE_BODY_TOKEN", "SESSION_LIST_ISSUE_TRANSCRIPT_TOKEN", "SESSION_LIST_ASSISTANT_TRANSCRIPT_TOKEN", "SESSION_LIST_COMMENT_TRANSCRIPT_TOKEN", "SESSION_LIST_ASSISTANT_COMMENT_TOKEN", "SESSION_LIST_USER_COMMENT_TOKEN"} {
		if strings.Contains(output, notWant) {
			t.Fatalf("session list output unexpectedly included %q:\n%s", notWant, output)
		}
	}
}

func TestCommandsCommandReportsCatalog(t *testing.T) {
	t.Setenv("GITCLAW_WORKDIR", t.TempDir())
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"commands"}); err != nil {
			t.Fatalf("commands returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Commands Report", "scope: `local-cli`", "commands: `15`", "aliases: `7`", "local_cli_helpers: `32`", "`/help` model=`gitclaw/commands`", "aliases=`/commands`", "`gitclaw commands` command=`/help`", "`gitclaw channels list` command=`/channels`", "`gitclaw config list` command=`/config`", "`gitclaw context list` command=`/context`", "`gitclaw prompt list` command=`/prompt`", "`gitclaw session list --backup <issue.json>` command=`/session`", "`gitclaw models list` command=`/models`", "`gitclaw policy list` command=`/policy`", "`gitclaw backup list` command=`/backup`", "`gitclaw backup stats` command=`/backup`", "`gitclaw backup search <query>` command=`/backup`", "`gitclaw backup retention-plan` command=`/backup`", "`gitclaw memory validate` command=`/memory`", "`gitclaw memory list` command=`/memory`", "`gitclaw memory search <query>` command=`/memory`", "`gitclaw soul list` command=`/soul`", "`gitclaw soul search <query>` command=`/soul`", "`gitclaw skills list` command=`/skills`", "`gitclaw skills info <name>` command=`/skills`", "`gitclaw skills search <query>` command=`/skills`", "`gitclaw tools list` command=`/tools`", "`gitclaw tools search <query>` command=`/tools`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("commands output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "issue: `#0`") {
		t.Fatalf("commands output should not include synthetic issue metadata:\n%s", output)
	}
}

func TestBackupListCommandReportsFetchedBackupTree(t *testing.T) {
	dir := t.TempDir()
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 7,
			Title:  "@gitclaw cli backup list old CLI_BACKUP_LIST_OLD_TITLE",
			Body:   "CLI_BACKUP_LIST_OLD_BODY",
			Labels: []string{"gitclaw"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "CLI_BACKUP_LIST_OLD_TRANSCRIPT"}},
	})
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue: IssueBackupIssue{
			Number: 8,
			Title:  "@gitclaw cli backup list new CLI_BACKUP_LIST_NEW_TITLE",
			Body:   "CLI_BACKUP_LIST_NEW_BODY",
			Labels: []string{"gitclaw", "gitclaw:e2e"},
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "CLI_BACKUP_LIST_NEW_TRANSCRIPT"}, {Role: "assistant", Body: "CLI_BACKUP_LIST_ASSISTANT"}},
		Comments:   []IssueBackupComment{{ID: 12, Body: "CLI_BACKUP_LIST_COMMENT"}},
	})
	if _, err := WriteBackupIndex(dir, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"backup", "list", "--root", dir, "--repo", "owner/repo", "--limit", "1"}); err != nil {
			t.Fatalf("backup list returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Backup List Report", "backup_list_status: `ok`", "backup_verify_status: `ok`", "issue_count: `2`", "limit: `1`", "backups_returned: `1`", "raw_bodies_included: `false`", "issue=#8 path=`issues/000008.json`", "labels=`2`", "comments=`1`", "transcript_messages=`2`", "title_sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup list output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"CLI_BACKUP_LIST_OLD_TITLE", "CLI_BACKUP_LIST_OLD_BODY", "CLI_BACKUP_LIST_OLD_TRANSCRIPT", "CLI_BACKUP_LIST_NEW_TITLE", "CLI_BACKUP_LIST_NEW_BODY", "CLI_BACKUP_LIST_NEW_TRANSCRIPT", "CLI_BACKUP_LIST_ASSISTANT", "CLI_BACKUP_LIST_COMMENT", "@gitclaw cli backup list"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("backup list leaked body/title token %q:\n%s", leaked, output)
		}
	}
}

func TestBackupStatsCommandReportsFetchedBackupTree(t *testing.T) {
	dir := t.TempDir()
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 7, Title: "@gitclaw cli stats", Body: "CLI_STATS_BODY_TOKEN"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "CLI_STATS_TRANSCRIPT_TOKEN"}},
	})
	if _, err := WriteBackupIndex(dir, "owner/repo", time.Date(2026, 5, 29, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"backup", "stats", "--root", dir, "--repo", "owner/repo"}); err != nil {
			t.Fatalf("backup stats returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Backup Stats Report", "backup_stats_status: `ok`", "backup_verify_status: `ok`", "issue_count: `1`", "transcript_messages: `1`", "latest_issue: `#7`", "raw_bodies_included: `false`"} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup stats output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "CLI_STATS_BODY_TOKEN") || strings.Contains(output, "CLI_STATS_TRANSCRIPT_TOKEN") || strings.Contains(output, "@gitclaw cli stats") {
		t.Fatalf("backup stats leaked body/title token:\n%s", output)
	}
}

func TestBackupSearchCommandReportsFetchedBackupMatches(t *testing.T) {
	dir := t.TempDir()
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number:            7,
			Title:             "@gitclaw cli backup search CLI_BACKUP_SEARCH_TITLE_TOKEN",
			Body:              "CLI backup search retrieval body CLI_BACKUP_SEARCH_BODY_TOKEN",
			Author:            "alice",
			AuthorAssociation: "OWNER",
		},
		Transcript: []TranscriptMessage{{Role: "user", Body: "retrieval transcript CLI_BACKUP_SEARCH_TRANSCRIPT_TOKEN", Actor: "alice", AuthorAssociation: "OWNER", Trusted: true}},
	})
	if _, err := WriteBackupIndex(dir, "owner/repo", time.Date(2026, 5, 29, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"backup", "search", "--root", dir, "--repo", "owner/repo", "--query", "retrieval CLI_BACKUP_SEARCH_QUERY_TOKEN", "--max-results", "1"}); err != nil {
			t.Fatalf("backup search returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Backup Search Report", "backup_search_status: `ok`", "backup_verify_status: `ok`", "query_sha256_12:", "max_results: `1`", "issue_count: `1`", "matched_issues: `1`", "matched_lines: `2`", "results_returned: `1`", "raw_bodies_included: `false`", "issue=`#7` path=`issues/000007.json`", "line_sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup search output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"CLI_BACKUP_SEARCH_TITLE_TOKEN", "CLI_BACKUP_SEARCH_BODY_TOKEN", "CLI_BACKUP_SEARCH_TRANSCRIPT_TOKEN", "CLI_BACKUP_SEARCH_QUERY_TOKEN", "retrieval CLI_BACKUP_SEARCH_QUERY_TOKEN", "@gitclaw cli backup search"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("backup search leaked body/title/query token %q:\n%s", leaked, output)
		}
	}
}

func TestBackupRetentionPlanCommandReportsDryRun(t *testing.T) {
	dir := t.TempDir()
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T12:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue:       IssueBackupIssue{Number: 7, Title: "@gitclaw cli retention old", Body: "CLI_RETENTION_OLD_BODY_TOKEN"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "CLI_RETENTION_OLD_TRANSCRIPT_TOKEN"}},
		Comments:    []IssueBackupComment{{ID: 11, Body: "CLI_RETENTION_OLD_COMMENT_TOKEN"}},
	})
	writeBackupFixture(t, dir, IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-29T13:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issue_comment",
		Issue:       IssueBackupIssue{Number: 8, Title: "@gitclaw cli retention new", Body: "CLI_RETENTION_NEW_BODY_TOKEN"},
		Transcript:  []TranscriptMessage{{Role: "user", Body: "CLI_RETENTION_NEW_TRANSCRIPT_TOKEN"}},
	})
	if _, err := WriteBackupIndex(dir, "owner/repo", time.Date(2026, 5, 29, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("WriteBackupIndex returned error: %v", err)
	}
	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"backup", "retention-plan", "--root", dir, "--repo", "owner/repo", "--keep-latest", "1"}); err != nil {
			t.Fatalf("backup retention-plan returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Backup Retention Plan", "retention_mode: `dry-run`", "backup_retention_status: `ok`", "backup_verify_status: `ok`", "keep_latest: `1`", "issue_count: `2`", "keep_count: `1`", "prune_candidate_count: `1`", "newest_kept_issue: `#8`", "oldest_kept_issue: `#8`", "raw_bodies_included: `false`", "### Kept Backups", "issue=#8 path=`issues/000008.json`", "### Prune Candidates", "issue=#7 path=`issues/000007.json`", "title_sha256_12="} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup retention-plan output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"CLI_RETENTION_OLD_BODY_TOKEN", "CLI_RETENTION_OLD_TRANSCRIPT_TOKEN", "CLI_RETENTION_OLD_COMMENT_TOKEN", "CLI_RETENTION_NEW_BODY_TOKEN", "CLI_RETENTION_NEW_TRANSCRIPT_TOKEN", "@gitclaw cli retention old", "@gitclaw cli retention new"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("backup retention-plan leaked body/title token %q:\n%s", leaked, output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
