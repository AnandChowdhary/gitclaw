package gitclaw

import (
	"strings"
	"testing"
)

func TestLoadConfigFromWorkdirAppliesRepoConfig(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  mode: inbox
  label: claw
  prefix: "@claw"
  disabled_label: claw:disabled

authorization:
  allowed_associations:
    - OWNER
    - contributor

model:
  provider: github-models
  model: openai/gpt-5-nano
  fallbacks:
    - openai/gpt-4.1-nano
    - openai/gpt-5-nano
    - openai/gpt-4.1-nano
  base_url: https://models.github.ai/inference/chat/completions
  max_prompt_bytes: 12345
  max_output_tokens: 678
  max_transcript_messages: 12
  max_transcript_message_bytes: 3456

actions:
  mode: read_only

skills:
  allowed:
    - repo-reader
    - deploy-helper
  disabled:
    - deploy-helper

tools:
  allowed:
    - list_files
    - gitclaw.read_file
  disabled:
    - search_files
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	loaded, err := LoadConfigFromWorkdir(cfg)
	if err != nil {
		t.Fatalf("LoadConfigFromWorkdir returned error: %v", err)
	}
	if loaded.ConfigSource != "defaults+repo" {
		t.Fatalf("ConfigSource = %q, want defaults+repo", loaded.ConfigSource)
	}
	if loaded.TriggerMode != TriggerModeInbox || loaded.TriggerLabel != "claw" || loaded.TriggerPrefix != "@claw" || loaded.DisabledLabel != "claw:disabled" {
		t.Fatalf("trigger config not applied: %#v", loaded)
	}
	if !loaded.AllowedAssociations["OWNER"] || !loaded.AllowedAssociations["CONTRIBUTOR"] || loaded.AllowedAssociations["MEMBER"] {
		t.Fatalf("allowed associations not replaced/normalized: %#v", loaded.AllowedAssociations)
	}
	if loaded.ModelProvider != "github-models" || loaded.LLMBaseURL != defaultGitHubModelsBaseURL {
		t.Fatalf("model provider/base_url config not applied: %#v", loaded)
	}
	if len(loaded.ModelFallbacks) != 2 || loaded.ModelFallbacks[0] != "openai/gpt-4.1-nano" || loaded.ModelFallbacks[1] != "openai/gpt-5-nano" {
		t.Fatalf("model fallbacks not normalized: %#v", loaded.ModelFallbacks)
	}
	if loaded.MaxPromptBytes != 12345 || loaded.MaxOutputTokens != 678 || loaded.MaxTranscriptMessages != 12 || loaded.MaxTranscriptMessageBytes != 3456 {
		t.Fatalf("prompt budget config not applied: %#v", loaded)
	}
	if !loaded.AllowedSkills["repo-reader"] || !loaded.AllowedSkills["deploy-helper"] || len(loaded.AllowedSkills) != 2 {
		t.Fatalf("skills.allowed config not applied: %#v", loaded.AllowedSkills)
	}
	if !loaded.DisabledSkills["deploy-helper"] || len(loaded.DisabledSkills) != 1 {
		t.Fatalf("skills.disabled config not applied: %#v", loaded.DisabledSkills)
	}
	if !loaded.AllowedTools["gitclaw.list_files"] || !loaded.AllowedTools["gitclaw.read_file"] || len(loaded.AllowedTools) != 2 {
		t.Fatalf("tools.allowed config not applied: %#v", loaded.AllowedTools)
	}
	if !loaded.DisabledTools["gitclaw.search_files"] || len(loaded.DisabledTools) != 1 {
		t.Fatalf("tools.disabled config not applied: %#v", loaded.DisabledTools)
	}
}

func TestLoadEffectiveConfigAppliesEnvironmentAfterRepoConfig(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  model: openai/repo-model
`)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITCLAW_MODEL", "openai/env-model")
	t.Setenv("GITCLAW_MODEL_FALLBACKS", "openai/env-fallback, openai/env-fallback-2")

	loaded, err := LoadEffectiveConfig()
	if err != nil {
		t.Fatalf("LoadEffectiveConfig returned error: %v", err)
	}
	if loaded.ConfigSource != "defaults+repo+environment" {
		t.Fatalf("ConfigSource = %q, want defaults+repo+environment", loaded.ConfigSource)
	}
	if loaded.Model != "openai/env-model" {
		t.Fatalf("Model = %q, want env override", loaded.Model)
	}
	if len(loaded.ModelFallbacks) != 2 || loaded.ModelFallbacks[0] != "openai/env-fallback" || loaded.ModelFallbacks[1] != "openai/env-fallback-2" {
		t.Fatalf("ModelFallbacks = %#v, want env override", loaded.ModelFallbacks)
	}
}

func TestEnvModelFallbacksCanDisableRepoFallbacks(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  model: openai/repo-model
  fallbacks:
    - openai/repo-fallback
`)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITCLAW_MODEL_FALLBACKS", "none")

	loaded, err := LoadEffectiveConfig()
	if err != nil {
		t.Fatalf("LoadEffectiveConfig returned error: %v", err)
	}
	if len(loaded.ModelFallbacks) != 0 {
		t.Fatalf("ModelFallbacks = %#v, want disabled", loaded.ModelFallbacks)
	}
}

func TestLoadConfigRejectsInvalidTriggerMode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  mode: socket-daemon
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	_, err := LoadConfigFromWorkdir(cfg)
	if err == nil {
		t.Fatalf("LoadConfigFromWorkdir should reject invalid trigger mode")
	}
	if !strings.Contains(err.Error(), "trigger.mode") {
		t.Fatalf("error should mention trigger.mode, got %v", err)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  model: openai/gpt-5-nano
  api_key: SHOULD_NOT_BE_ACCEPTED
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	_, err := LoadConfigFromWorkdir(cfg)
	if err == nil {
		t.Fatalf("LoadConfigFromWorkdir should reject unknown fields")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("error should mention unknown field, got %v", err)
	}
}

func TestLoadConfigRejectsInvalidSkillGateNames(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `skills:
  allowed:
    - Repo Reader
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	_, err := LoadConfigFromWorkdir(cfg)
	if err == nil {
		t.Fatalf("LoadConfigFromWorkdir should reject invalid skill names")
	}
	if !strings.Contains(err.Error(), "skills.allowed") {
		t.Fatalf("error should mention skills.allowed, got %v", err)
	}
}

func TestLoadConfigRejectsUnknownToolGateNames(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `tools:
  disabled:
    - shell_exec
`)
	cfg := DefaultConfig()
	cfg.Workdir = root
	_, err := LoadConfigFromWorkdir(cfg)
	if err == nil {
		t.Fatalf("LoadConfigFromWorkdir should reject unknown tool names")
	}
	if !strings.Contains(err.Error(), "tools.disabled") {
		t.Fatalf("error should mention tools.disabled, got %v", err)
	}
}
