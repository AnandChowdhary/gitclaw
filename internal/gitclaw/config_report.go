package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const gitclawConfigPath = ".gitclaw/config.yml"

var configWorkflowPaths = []string{
	".github/workflows/gitclaw.yml",
	".github/workflows/gitclaw-heartbeat.yml",
	".github/workflows/gitclaw-proactive.yml",
	".github/workflows/gitclaw-channel-ingest.yml",
	".github/workflows/gitclaw-channel-state.yml",
	".github/workflows/gitclaw-channel-gateway.yml",
	".github/workflows/gitclaw-channel-delivery.yml",
}

var configSlashCommands = commandCatalogNames()

type configSurfaceFile struct {
	Path    string
	Present bool
	Bytes   int
	Lines   int
	SHA     string
}

type configSurface struct {
	ConfigFile configSurfaceFile
	Workflows  []configSurfaceFile
}

func IsConfigReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/config" || command == "/configuration"
}

func RenderConfigReport(ev Event, cfg Config) string {
	return renderConfigReport(ev, cfg, true)
}

func RenderConfigCLIReport(cfg Config) string {
	return renderConfigReport(Event{}, cfg, false)
}

func renderConfigReport(ev Event, cfg Config, includeIssue bool) string {
	surface := inspectConfigSurface(cfg.Workdir)
	var b strings.Builder
	b.WriteString("## GitClaw Config Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- config_source: `%s`\n", configSource(cfg))
	fmt.Fprintf(&b, "- config_file_path: `%s`\n", gitclawConfigPath)
	fmt.Fprintf(&b, "- config_file_present: `%t`\n", surface.ConfigFile.Present)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- trigger_prefix: `%s`\n", cfg.TriggerPrefix)
	fmt.Fprintf(&b, "- disabled_label: `%s`\n", cfg.DisabledLabel)
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- workdir: `%s`\n", inlineCode(cfg.Workdir))
	fmt.Fprintf(&b, "- max_prompt_bytes: `%d`\n", cfg.MaxPromptBytes)
	fmt.Fprintf(&b, "- max_output_tokens: `%d`\n", cfg.MaxOutputTokens)
	fmt.Fprintf(&b, "- max_transcript_messages: `%d`\n", cfg.MaxTranscriptMessages)
	fmt.Fprintf(&b, "- max_transcript_message_bytes: `%d`\n", cfg.MaxTranscriptMessageBytes)
	fmt.Fprintf(&b, "- workflows_present: `%d`\n", countPresentConfigFiles(surface.Workflows))
	fmt.Fprintf(&b, "- slash_commands: `%d`\n", len(configSlashCommands))
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report shows effective GitClaw control-plane settings and file metadata. Config, workflow, issue, and comment bodies are not included.\n\n")

	b.WriteString("### Trusted Associations\n")
	for _, association := range sortedAllowedAssociations(cfg) {
		fmt.Fprintf(&b, "- `%s`\n", association)
	}

	b.WriteString("\n### Managed Labels\n")
	for _, label := range managedPolicyLabels(cfg) {
		fmt.Fprintf(&b, "- `%s`\n", label)
	}

	b.WriteString("\n### Slash Commands\n")
	for _, command := range configSlashCommands {
		fmt.Fprintf(&b, "- `%s`\n", command)
	}

	b.WriteString("\n### Config File\n")
	writeConfigSurfaceFile(&b, surface.ConfigFile)

	b.WriteString("\n### Workflow Files\n")
	for _, file := range surface.Workflows {
		writeConfigSurfaceFile(&b, file)
	}

	return strings.TrimSpace(b.String())
}

func configSource(cfg Config) string {
	if cfg.ConfigSource == "" {
		return "defaults"
	}
	return cfg.ConfigSource
}

func inspectConfigSurface(root string) configSurface {
	if root == "" {
		root = "."
	}
	surface := configSurface{
		ConfigFile: inspectConfigSurfaceFile(root, gitclawConfigPath),
		Workflows:  make([]configSurfaceFile, 0, len(configWorkflowPaths)),
	}
	for _, path := range configWorkflowPaths {
		surface.Workflows = append(surface.Workflows, inspectConfigSurfaceFile(root, path))
	}
	sort.Slice(surface.Workflows, func(i, j int) bool {
		return surface.Workflows[i].Path < surface.Workflows[j].Path
	})
	return surface
}

func inspectConfigSurfaceFile(root, rel string) configSurfaceFile {
	file := configSurfaceFile{Path: rel}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return file
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(rel)))
	if err != nil {
		return file
	}
	text := string(body)
	file.Present = true
	file.Bytes = len(body)
	file.Lines = lineCount(text)
	file.SHA = shortDocumentHash(text)
	return file
}

func writeConfigSurfaceFile(b *strings.Builder, file configSurfaceFile) {
	if !file.Present {
		fmt.Fprintf(b, "- `%s` present=`false`\n", file.Path)
		return
	}
	fmt.Fprintf(b, "- `%s` present=`true` bytes=`%d` lines=`%d` sha256_12=`%s`\n", file.Path, file.Bytes, file.Lines, file.SHA)
}

func countPresentConfigFiles(files []configSurfaceFile) int {
	count := 0
	for _, file := range files {
		if file.Present {
			count++
		}
	}
	return count
}
