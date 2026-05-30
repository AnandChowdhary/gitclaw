package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Trigger       fileTriggerConfig       `yaml:"trigger"`
	Authorization fileAuthorizationConfig `yaml:"authorization"`
	Model         fileModelConfig         `yaml:"model"`
	Actions       fileActionsConfig       `yaml:"actions"`
	Skills        fileSkillsConfig        `yaml:"skills"`
	Tools         fileToolsConfig         `yaml:"tools"`
}

type fileTriggerConfig struct {
	Mode          string `yaml:"mode"`
	Label         string `yaml:"label"`
	Prefix        string `yaml:"prefix"`
	DisabledLabel string `yaml:"disabled_label"`
}

type fileAuthorizationConfig struct {
	AllowedAssociations []string `yaml:"allowed_associations"`
}

type fileModelConfig struct {
	Provider                  string   `yaml:"provider"`
	Model                     string   `yaml:"model"`
	Fallbacks                 []string `yaml:"fallbacks"`
	BaseURL                   string   `yaml:"base_url"`
	MaxPromptBytes            int      `yaml:"max_prompt_bytes"`
	MaxInputTokens            int      `yaml:"max_input_tokens"`
	MaxOutputTokens           int      `yaml:"max_output_tokens"`
	MaxTranscriptMessages     int      `yaml:"max_transcript_messages"`
	MaxTranscriptMessageBytes int      `yaml:"max_transcript_message_bytes"`
}

type fileActionsConfig struct {
	Mode string `yaml:"mode"`
}

type fileSkillsConfig struct {
	Allowed  []string `yaml:"allowed"`
	Disabled []string `yaml:"disabled"`
}

type fileToolsConfig struct {
	Allowed  []string `yaml:"allowed"`
	Disabled []string `yaml:"disabled"`
}

var knownAuthorAssociations = []string{
	"OWNER",
	"MEMBER",
	"COLLABORATOR",
	"CONTRIBUTOR",
	"FIRST_TIME_CONTRIBUTOR",
	"FIRST_TIMER",
	"MANNEQUIN",
	"NONE",
}

func LoadConfigFromWorkdir(cfg Config) (Config, error) {
	if cfg.ConfigSource == "" {
		cfg.ConfigSource = "defaults"
	}
	root := cfg.Workdir
	if root == "" {
		root = "."
	}
	path := filepath.Join(root, filepath.FromSlash(gitclawConfigPath))
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read %s: %w", gitclawConfigPath, err)
	}

	var file fileConfig
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", gitclawConfigPath, err)
	}
	if err := applyFileConfig(&cfg, file); err != nil {
		return cfg, err
	}
	cfg.ConfigSource = appendConfigSource(cfg.ConfigSource, "repo")
	return cfg, nil
}

func ApplyEnvConfig(cfg Config) Config {
	if workdir := os.Getenv("GITCLAW_WORKDIR"); workdir != "" {
		cfg.Workdir = workdir
		cfg.ConfigSource = appendConfigSource(cfg.ConfigSource, "environment")
	}
	if model := os.Getenv("GITCLAW_MODEL"); model != "" {
		cfg.Model = model
		cfg.ConfigSource = appendConfigSource(cfg.ConfigSource, "environment")
	}
	if fallbacks, ok := envModelFallbacks(); ok {
		cfg.ModelFallbacks = fallbacks
		cfg.ConfigSource = appendConfigSource(cfg.ConfigSource, "environment")
	}
	if baseURL := os.Getenv("GITCLAW_LLM_BASE_URL"); baseURL != "" {
		cfg.LLMBaseURL = baseURL
		cfg.ConfigSource = appendConfigSource(cfg.ConfigSource, "environment")
	}
	return cfg
}

func LoadEffectiveConfig() (Config, error) {
	cfg := DefaultConfig()
	envWorkdir := os.Getenv("GITCLAW_WORKDIR")
	if envWorkdir != "" {
		cfg.Workdir = envWorkdir
	}
	loaded, err := LoadConfigFromWorkdir(cfg)
	if err != nil {
		return loaded, err
	}
	if envWorkdir != "" {
		loaded.ConfigSource = appendConfigSource(loaded.ConfigSource, "environment")
	}
	if model := os.Getenv("GITCLAW_MODEL"); model != "" {
		loaded.Model = model
		loaded.ConfigSource = appendConfigSource(loaded.ConfigSource, "environment")
	}
	if fallbacks, ok := envModelFallbacks(); ok {
		loaded.ModelFallbacks = fallbacks
		loaded.ConfigSource = appendConfigSource(loaded.ConfigSource, "environment")
	}
	if baseURL := os.Getenv("GITCLAW_LLM_BASE_URL"); baseURL != "" {
		loaded.LLMBaseURL = baseURL
		loaded.ConfigSource = appendConfigSource(loaded.ConfigSource, "environment")
	}
	return loaded, nil
}

func applyFileConfig(cfg *Config, file fileConfig) error {
	if value := strings.TrimSpace(file.Trigger.Mode); value != "" {
		mode, err := normalizeTriggerMode(value)
		if err != nil {
			return err
		}
		cfg.TriggerMode = mode
	}
	if value := strings.TrimSpace(file.Trigger.Label); value != "" {
		if err := validateLabelValue("trigger.label", value); err != nil {
			return err
		}
		cfg.TriggerLabel = value
	}
	if value := strings.TrimSpace(file.Trigger.Prefix); value != "" {
		cfg.TriggerPrefix = value
	}
	if value := strings.TrimSpace(file.Trigger.DisabledLabel); value != "" {
		if err := validateLabelValue("trigger.disabled_label", value); err != nil {
			return err
		}
		cfg.DisabledLabel = value
	}
	if len(file.Authorization.AllowedAssociations) > 0 {
		associations := map[string]bool{}
		for _, association := range file.Authorization.AllowedAssociations {
			normalized := strings.ToUpper(strings.TrimSpace(association))
			if !slices.Contains(knownAuthorAssociations, normalized) {
				return fmt.Errorf("%s contains unknown author association %q", gitclawConfigPath, association)
			}
			associations[normalized] = true
		}
		cfg.AllowedAssociations = associations
	}
	if value := strings.TrimSpace(file.Model.Provider); value != "" {
		if value != "github-models" && value != "openai-compatible" {
			return fmt.Errorf("%s model.provider must be github-models or openai-compatible", gitclawConfigPath)
		}
		cfg.ModelProvider = value
	}
	if value := strings.TrimSpace(file.Model.Model); value != "" {
		cfg.Model = value
	}
	if len(file.Model.Fallbacks) > 0 {
		cfg.ModelFallbacks = normalizeModelFallbacks(file.Model.Fallbacks)
	}
	if value := strings.TrimSpace(file.Model.BaseURL); value != "" {
		cfg.LLMBaseURL = value
	}
	if file.Model.MaxInputTokens > 0 {
		cfg.MaxPromptBytes = file.Model.MaxInputTokens
	}
	if file.Model.MaxPromptBytes > 0 {
		cfg.MaxPromptBytes = file.Model.MaxPromptBytes
	}
	if file.Model.MaxOutputTokens > 0 {
		cfg.MaxOutputTokens = file.Model.MaxOutputTokens
	}
	if file.Model.MaxTranscriptMessages > 0 {
		cfg.MaxTranscriptMessages = file.Model.MaxTranscriptMessages
	}
	if file.Model.MaxTranscriptMessageBytes > 0 {
		cfg.MaxTranscriptMessageBytes = file.Model.MaxTranscriptMessageBytes
	}
	if value := strings.TrimSpace(file.Actions.Mode); value != "" && value != "read_only" {
		return fmt.Errorf("%s actions.mode must be read_only", gitclawConfigPath)
	}
	if len(file.Skills.Allowed) > 0 {
		allowed, err := normalizeConfiguredSkillSet("skills.allowed", file.Skills.Allowed)
		if err != nil {
			return err
		}
		cfg.AllowedSkills = allowed
	}
	if len(file.Skills.Disabled) > 0 {
		disabled, err := normalizeConfiguredSkillSet("skills.disabled", file.Skills.Disabled)
		if err != nil {
			return err
		}
		cfg.DisabledSkills = disabled
	}
	if len(file.Tools.Allowed) > 0 {
		allowed, err := normalizeConfiguredToolSet("tools.allowed", file.Tools.Allowed)
		if err != nil {
			return err
		}
		cfg.AllowedTools = allowed
	}
	if len(file.Tools.Disabled) > 0 {
		disabled, err := normalizeConfiguredToolSet("tools.disabled", file.Tools.Disabled)
		if err != nil {
			return err
		}
		cfg.DisabledTools = disabled
	}
	return validateConfig(*cfg)
}

func validateConfig(cfg Config) error {
	if _, err := normalizeTriggerMode(cfg.TriggerMode); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"trigger.label":          cfg.TriggerLabel,
		"trigger.disabled_label": cfg.DisabledLabel,
		"labels.running":         cfg.RunningLabel,
		"labels.done":            cfg.DoneLabel,
		"labels.error":           cfg.ErrorLabel,
		"labels.heartbeat":       cfg.HeartbeatLabel,
		"labels.channel":         cfg.ChannelLabel,
		"labels.proactive":       cfg.ProactiveLabel,
		"labels.write_requested": cfg.WriteRequestedLabel,
	} {
		if err := validateLabelValue(name, value); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.TriggerPrefix) == "" {
		return fmt.Errorf("%s trigger.prefix must not be empty", gitclawConfigPath)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("%s model.model must not be empty", gitclawConfigPath)
	}
	for _, fallback := range cfg.ModelFallbacks {
		if strings.TrimSpace(fallback) == "" {
			return fmt.Errorf("%s model.fallbacks must not contain empty model ids", gitclawConfigPath)
		}
	}
	if strings.TrimSpace(cfg.LLMBaseURL) == "" {
		return fmt.Errorf("%s model.base_url must not be empty", gitclawConfigPath)
	}
	if cfg.MaxPromptBytes <= 0 {
		return fmt.Errorf("%s model.max_prompt_bytes must be positive", gitclawConfigPath)
	}
	if cfg.MaxOutputTokens <= 0 {
		return fmt.Errorf("%s model.max_output_tokens must be positive", gitclawConfigPath)
	}
	if cfg.MaxTranscriptMessages <= 0 {
		return fmt.Errorf("%s model.max_transcript_messages must be positive", gitclawConfigPath)
	}
	if cfg.MaxTranscriptMessageBytes <= 0 {
		return fmt.Errorf("%s model.max_transcript_message_bytes must be positive", gitclawConfigPath)
	}
	if len(cfg.AllowedAssociations) == 0 {
		return fmt.Errorf("%s authorization.allowed_associations must not be empty", gitclawConfigPath)
	}
	return nil
}

func normalizeTriggerMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", TriggerModeLabelOrPrefix, "label_or_prefix", "label-prefix", "per-repo", "per_repo":
		return TriggerModeLabelOrPrefix, nil
	case TriggerModeLabelOnly, "label_only", "label":
		return TriggerModeLabelOnly, nil
	case TriggerModePrefixOnly, "prefix_only", "prefix":
		return TriggerModePrefixOnly, nil
	case TriggerModeInbox, "inbox-repo", "inbox_repo", "all":
		return TriggerModeInbox, nil
	default:
		return "", fmt.Errorf("%s trigger.mode must be one of %s, %s, %s, or %s", gitclawConfigPath, TriggerModeLabelOrPrefix, TriggerModeLabelOnly, TriggerModePrefixOnly, TriggerModeInbox)
	}
}

func envModelFallbacks() ([]string, bool) {
	raw, ok := os.LookupEnv("GITCLAW_MODEL_FALLBACKS")
	if !ok {
		return nil, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") || strings.EqualFold(raw, "false") || raw == "[]" {
		return nil, true
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
	return normalizeModelFallbacks(parts), true
}

func normalizeModelFallbacks(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeConfiguredSkillSet(field string, values []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, value := range values {
		name := strings.ToLower(cleanSkillLookupName(value))
		if name == "" {
			continue
		}
		if !skillNamePattern.MatchString(name) {
			return nil, fmt.Errorf("%s %s contains invalid skill name %q", gitclawConfigPath, field, value)
		}
		out[name] = true
	}
	return out, nil
}

func normalizeConfiguredToolSet(field string, values []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, value := range values {
		name := normalizeToolLookupName(value)
		if name == "" {
			continue
		}
		if len(matchingToolContracts(toolReportContracts, name)) != 1 {
			return nil, fmt.Errorf("%s %s contains unknown tool %q", gitclawConfigPath, field, value)
		}
		out[name] = true
	}
	return out, nil
}

func validateLabelValue(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s %s must not be empty", gitclawConfigPath, name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s %s must be a single-line label", gitclawConfigPath, name)
	}
	return nil
}

func appendConfigSource(source, part string) string {
	if source == "" {
		return part
	}
	for _, existing := range strings.Split(source, "+") {
		if existing == part {
			return source
		}
	}
	return source + "+" + part
}
