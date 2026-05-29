package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var doctorContextPaths = []string{
	".gitclaw/SOUL.md",
	".gitclaw/IDENTITY.md",
	".gitclaw/USER.md",
	".gitclaw/TOOLS.md",
	".gitclaw/MEMORY.md",
	".gitclaw/HEARTBEAT.md",
}

type doctorSurface struct {
	Config             configSurfaceFile
	Workflows          []configSurfaceFile
	ContextFiles       []configSurfaceFile
	MemoryNotes        []configSurfaceFile
	SkillFiles         []configSurfaceFile
	Proactive          proactiveSurface
	SkillValidation    SkillValidationReport
	SoulValidation     SoulValidationReport
	ToolValidation     ToolValidationReport
	MemoryValidation   MemoryValidationReport
	ConfigValid        bool
	ConfigError        string
	ModelHost          string
	ConfigSource       string
	ManagedLabels      []string
	ValidationErrors   int
	ValidationWarnings int
}

func IsDoctorReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/doctor" || command == "/health"
}

func RenderDoctorReport(ev Event, cfg Config, repoContext RepoContext) string {
	surface := inspectDoctorSurface(cfg, repoContext)
	checks := doctorChecks(surface)
	var b strings.Builder
	b.WriteString("## GitClaw Doctor Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	}
	fmt.Fprintf(&b, "- health_status: `%s`\n", doctorHealthStatus(checks))
	fmt.Fprintf(&b, "- config_source: `%s`\n", surface.ConfigSource)
	fmt.Fprintf(&b, "- config_valid: `%t`\n", surface.ConfigValid)
	fmt.Fprintf(&b, "- config_file_present: `%t`\n", surface.Config.Present)
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- model_provider: `%s`\n", llmProviderForReport(cfg, llmBaseURL(cfg)))
	fmt.Fprintf(&b, "- model_endpoint_host: `%s`\n", surface.ModelHost)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- workflows_present: `%d`\n", countPresentConfigFiles(surface.Workflows))
	fmt.Fprintf(&b, "- context_files_present: `%d`\n", countPresentConfigFiles(surface.ContextFiles))
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", len(surface.MemoryNotes))
	fmt.Fprintf(&b, "- skill_files: `%d`\n", len(surface.SkillFiles))
	fmt.Fprintf(&b, "- proactive_prompt_files: `%d`\n", len(surface.Proactive.Prompts))
	fmt.Fprintf(&b, "- managed_labels: `%d`\n", len(surface.ManagedLabels))
	fmt.Fprintf(&b, "- validation_errors: `%d`\n", surface.ValidationErrors)
	fmt.Fprintf(&b, "- validation_warnings: `%d`\n", surface.ValidationWarnings)
	fmt.Fprintf(&b, "- skill_validation_status: `%s`\n", surface.SkillValidation.Status)
	fmt.Fprintf(&b, "- skill_validation_errors: `%d`\n", surface.SkillValidation.Errors)
	fmt.Fprintf(&b, "- skill_validation_warnings: `%d`\n", surface.SkillValidation.Warnings)
	fmt.Fprintf(&b, "- soul_validation_status: `%s`\n", surface.SoulValidation.Status)
	fmt.Fprintf(&b, "- soul_validation_errors: `%d`\n", surface.SoulValidation.Errors)
	fmt.Fprintf(&b, "- soul_validation_warnings: `%d`\n", surface.SoulValidation.Warnings)
	fmt.Fprintf(&b, "- memory_validation_status: `%s`\n", surface.MemoryValidation.Status)
	fmt.Fprintf(&b, "- memory_validation_errors: `%d`\n", surface.MemoryValidation.Errors)
	fmt.Fprintf(&b, "- memory_validation_warnings: `%d`\n", surface.MemoryValidation.Warnings)
	fmt.Fprintf(&b, "- tool_validation_status: `%s`\n", surface.ToolValidation.Status)
	fmt.Fprintf(&b, "- tool_validation_errors: `%d`\n", surface.ToolValidation.Errors)
	fmt.Fprintf(&b, "- tool_validation_warnings: `%d`\n", surface.ToolValidation.Warnings)
	if ev.Issue.Title != "" {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteString("\nThis report checks the GitClaw control plane from the checked-out repository. File bodies, issue bodies, comments, prompts, and secrets are not included.\n\n")

	b.WriteString("### Checks\n")
	for _, check := range checks {
		fmt.Fprintf(&b, "- `%s`: `%s`", check.Name, check.Status)
		if check.Detail != "" {
			fmt.Fprintf(&b, " - %s", check.Detail)
		}
		b.WriteByte('\n')
	}

	b.WriteString("\n### Config File\n")
	writeConfigSurfaceFile(&b, surface.Config)
	if surface.ConfigError != "" {
		fmt.Fprintf(&b, "- validation_error_sha256_12=`%s`\n", shortDocumentHash(surface.ConfigError))
	}

	b.WriteString("\n### Workflow Files\n")
	for _, file := range surface.Workflows {
		writeConfigSurfaceFile(&b, file)
	}

	b.WriteString("\n### Context Files\n")
	for _, file := range surface.ContextFiles {
		writeConfigSurfaceFile(&b, file)
	}

	b.WriteString("\n### Memory Notes\n")
	writeDoctorFileList(&b, surface.MemoryNotes)

	b.WriteString("\n### Skills\n")
	writeDoctorFileList(&b, surface.SkillFiles)

	b.WriteString("\n### Proactive Prompts\n")
	if len(surface.Proactive.Prompts) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, prompt := range surface.Proactive.Prompts {
			fmt.Fprintf(&b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", prompt.Path, prompt.Bytes, prompt.Lines, prompt.SHA)
		}
	}

	return strings.TrimSpace(b.String())
}

type doctorCheck struct {
	Name   string
	Status string
	Detail string
}

func inspectDoctorSurface(cfg Config, repoContext RepoContext) doctorSurface {
	root := cfg.Workdir
	if root == "" {
		root = "."
	}
	skillValidation := ValidateSkillSummaries(repoContext.SkillSummaries)
	soulValidation := ValidateSoulContext(repoContext)
	toolValidation := ValidateTools(repoContext)
	memoryValidation := ValidateMemory(root, repoContext)
	surface := doctorSurface{
		Config:             inspectConfigSurfaceFile(root, gitclawConfigPath),
		Workflows:          make([]configSurfaceFile, 0, len(configWorkflowPaths)),
		ContextFiles:       make([]configSurfaceFile, 0, len(doctorContextPaths)),
		Proactive:          inspectProactiveSurface(root),
		SkillValidation:    skillValidation,
		SoulValidation:     soulValidation,
		ToolValidation:     toolValidation,
		MemoryValidation:   memoryValidation,
		ConfigSource:       configSource(cfg),
		ModelHost:          llmEndpointHost(llmBaseURL(cfg)),
		ManagedLabels:      managedPolicyLabels(cfg),
		ValidationErrors:   skillValidation.Errors + soulValidation.Errors + memoryValidation.Errors + toolValidation.Errors,
		ValidationWarnings: skillValidation.Warnings + soulValidation.Warnings + memoryValidation.Warnings + toolValidation.Warnings,
	}
	configCheck := DefaultConfig()
	configCheck.Workdir = root
	if _, err := LoadConfigFromWorkdir(configCheck); err != nil {
		surface.ConfigValid = false
		surface.ConfigError = err.Error()
	} else {
		surface.ConfigValid = true
	}
	for _, path := range configWorkflowPaths {
		surface.Workflows = append(surface.Workflows, inspectConfigSurfaceFile(root, path))
	}
	sort.Slice(surface.Workflows, func(i, j int) bool {
		return surface.Workflows[i].Path < surface.Workflows[j].Path
	})
	for _, path := range doctorContextPaths {
		surface.ContextFiles = append(surface.ContextFiles, inspectConfigSurfaceFile(root, path))
	}
	surface.MemoryNotes = inspectDoctorGlob(root, ".gitclaw/memory/*.md")
	surface.SkillFiles = inspectDoctorGlob(root, ".gitclaw/SKILLS/*/SKILL.md")
	return surface
}

func inspectDoctorGlob(root, pattern string) []configSurfaceFile {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(absRoot, filepath.FromSlash(pattern)))
	sort.Strings(matches)
	files := make([]configSurfaceFile, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		files = append(files, inspectConfigSurfaceFile(root, filepath.ToSlash(rel)))
	}
	return files
}

func doctorChecks(surface doctorSurface) []doctorCheck {
	return []doctorCheck{
		{
			Name:   "config_file",
			Status: doctorStatus(surface.Config.Present),
			Detail: doctorBoolDetail(surface.Config.Present, "present", "missing"),
		},
		{
			Name:   "config_validation",
			Status: doctorStatus(surface.ConfigValid),
			Detail: doctorBoolDetail(surface.ConfigValid, "schema accepted", "schema rejected"),
		},
		{
			Name:   "main_workflow",
			Status: doctorStatus(configWorkflowPresent(surface.Workflows, ".github/workflows/gitclaw.yml")),
			Detail: ".github/workflows/gitclaw.yml",
		},
		{
			Name:   "workflow_set",
			Status: doctorStatus(countPresentConfigFiles(surface.Workflows) == len(surface.Workflows)),
			Detail: fmt.Sprintf("%d/%d present", countPresentConfigFiles(surface.Workflows), len(surface.Workflows)),
		},
		{
			Name:   "identity_context",
			Status: doctorStatus(configFilePresent(surface.ContextFiles, ".gitclaw/SOUL.md") && configFilePresent(surface.ContextFiles, ".gitclaw/IDENTITY.md")),
			Detail: "SOUL.md and IDENTITY.md",
		},
		{
			Name:   "local_skills",
			Status: doctorStatus(len(surface.SkillFiles) > 0),
			Detail: fmt.Sprintf("%d SKILL.md file(s)", len(surface.SkillFiles)),
		},
		{
			Name:   "proactive_prompt",
			Status: doctorStatus(len(surface.Proactive.Prompts) > 0),
			Detail: fmt.Sprintf("%d prompt file(s)", len(surface.Proactive.Prompts)),
		},
		{
			Name:   "skill_validation",
			Status: surface.SkillValidation.Status,
			Detail: doctorValidationDetail(surface.SkillValidation.Errors, surface.SkillValidation.Warnings),
		},
		{
			Name:   "soul_validation",
			Status: surface.SoulValidation.Status,
			Detail: doctorValidationDetail(surface.SoulValidation.Errors, surface.SoulValidation.Warnings),
		},
		{
			Name:   "memory_validation",
			Status: surface.MemoryValidation.Status,
			Detail: doctorValidationDetail(surface.MemoryValidation.Errors, surface.MemoryValidation.Warnings),
		},
		{
			Name:   "tool_validation",
			Status: surface.ToolValidation.Status,
			Detail: doctorValidationDetail(surface.ToolValidation.Errors, surface.ToolValidation.Warnings),
		},
	}
}

func doctorHealthStatus(checks []doctorCheck) string {
	for _, check := range checks {
		if check.Status != "ok" {
			return "warn"
		}
	}
	return "ok"
}

func doctorStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "warn"
}

func doctorBoolDetail(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func doctorValidationDetail(errors, warnings int) string {
	return fmt.Sprintf("%d error(s), %d warning(s)", errors, warnings)
}

func configWorkflowPresent(files []configSurfaceFile, path string) bool {
	return configFilePresent(files, path)
}

func configFilePresent(files []configSurfaceFile, path string) bool {
	for _, file := range files {
		if file.Path == path && file.Present {
			return true
		}
	}
	return false
}

func writeDoctorFileList(b *strings.Builder, files []configSurfaceFile) {
	if len(files) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, file := range files {
		writeConfigSurfaceFile(b, file)
	}
}

func PrintDoctorReport(cfg Config) {
	repo := os.Getenv("GITHUB_REPOSITORY")
	repoContext, _ := LoadRepoContext(cfg.Workdir, nil)
	fmt.Println(RenderDoctorReport(Event{Repo: repo}, cfg, repoContext))
}
