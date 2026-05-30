package gitclaw

import (
	"strings"
	"testing"
)

func TestLoadConfigFromWorkdirAppliesRepoConfig(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `trigger:
  label: claw
  prefix: "@claw"
  disabled_label: claw:disabled

authorization:
  allowed_associations:
    - OWNER
    - contributor

model:
  provider: github-models
  model: openai/gpt-5-mini
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
	if loaded.TriggerLabel != "claw" || loaded.TriggerPrefix != "@claw" || loaded.DisabledLabel != "claw:disabled" {
		t.Fatalf("trigger config not applied: %#v", loaded)
	}
	if !loaded.AllowedAssociations["OWNER"] || !loaded.AllowedAssociations["CONTRIBUTOR"] || loaded.AllowedAssociations["MEMBER"] {
		t.Fatalf("allowed associations not replaced/normalized: %#v", loaded.AllowedAssociations)
	}
	if loaded.ModelProvider != "github-models" || loaded.LLMBaseURL != defaultGitHubModelsBaseURL {
		t.Fatalf("model provider/base_url config not applied: %#v", loaded)
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
}

func TestLoadEffectiveConfigAppliesEnvironmentAfterRepoConfig(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  model: openai/repo-model
`)
	t.Setenv("GITCLAW_WORKDIR", root)
	t.Setenv("GITCLAW_MODEL", "openai/env-model")

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
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/config.yml", `model:
  model: openai/gpt-5-mini
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
