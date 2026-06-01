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
	E2E                doctorE2ESurface
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

type doctorE2ESurface struct {
	Scripts                 []doctorE2EScript
	ScriptCount             int
	LiveIssueScripts        int
	CleanupScripts          int
	ModelCoverageScripts    int
	ModelFollowupScripts    int
	SessionCoverageScripts  int
	BackupGateScripts       int
	WorkflowDispatchScripts int
}

type doctorE2EScript struct {
	Path             string
	Bytes            int
	Lines            int
	SHA              string
	CreatesIssue     bool
	HasCleanup       bool
	ModelCoverage    bool
	ModelFollowup    bool
	SessionCoverage  bool
	BackupGate       bool
	WorkflowDispatch bool
}

func IsDoctorReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/doctor" || command == "/health"
}

func RenderDoctorReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderDoctorReport(ev, cfg, repoContext, true)
}

func RenderDoctorCLIReport(cfg Config, repoContext RepoContext) string {
	return renderDoctorReport(Event{}, cfg, repoContext, false)
}

func renderDoctorReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	surface := inspectDoctorSurface(cfg, repoContext)
	checks := doctorChecks(surface)
	var b strings.Builder
	b.WriteString("## GitClaw Doctor Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue && ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if includeIssue && ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else if !includeIssue {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
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
	fmt.Fprintf(&b, "- e2e_scripts: `%d`\n", surface.E2E.ScriptCount)
	fmt.Fprintf(&b, "- e2e_live_issue_scripts: `%d`\n", surface.E2E.LiveIssueScripts)
	fmt.Fprintf(&b, "- e2e_cleanup_scripts: `%d`\n", surface.E2E.CleanupScripts)
	fmt.Fprintf(&b, "- e2e_model_coverage_scripts: `%d`\n", surface.E2E.ModelCoverageScripts)
	fmt.Fprintf(&b, "- e2e_model_followup_scripts: `%d`\n", surface.E2E.ModelFollowupScripts)
	fmt.Fprintf(&b, "- e2e_session_coverage_scripts: `%d`\n", surface.E2E.SessionCoverageScripts)
	fmt.Fprintf(&b, "- e2e_backup_gate_scripts: `%d`\n", surface.E2E.BackupGateScripts)
	fmt.Fprintf(&b, "- e2e_workflow_dispatch_scripts: `%d`\n", surface.E2E.WorkflowDispatchScripts)
	fmt.Fprintf(&b, "- enabled_skills: `%d`\n", enabledSkillCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- disabled_skills: `%d`\n", disabledByConfigCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- allowlist_blocked_skills: `%d`\n", blockedByAllowlistCount(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- enabled_tools: `%d`\n", enabledToolCount(repoContext))
	fmt.Fprintf(&b, "- disabled_tools: `%d`\n", disabledToolCount(repoContext))
	fmt.Fprintf(&b, "- allowlist_blocked_tools: `%d`\n", allowlistBlockedToolCount(repoContext))
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
	if includeIssue && ev.Issue.Title != "" {
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

	b.WriteString("\n### E2E Harnesses\n")
	writeDoctorE2ESurface(&b, surface.E2E)

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
		E2E:                inspectDoctorE2ESurface(root),
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

func inspectDoctorE2ESurface(root string) doctorE2ESurface {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return doctorE2ESurface{}
	}
	matches, _ := filepath.Glob(filepath.Join(absRoot, "scripts", "e2e", "*.sh"))
	sort.Strings(matches)
	surface := doctorE2ESurface{Scripts: make([]doctorE2EScript, 0, len(matches))}
	for _, match := range matches {
		rel, err := filepath.Rel(absRoot, match)
		if err != nil {
			continue
		}
		body, err := os.ReadFile(match)
		if err != nil {
			continue
		}
		text := string(body)
		lower := strings.ToLower(text)
		script := doctorE2EScript{
			Path:             filepath.ToSlash(rel),
			Bytes:            len(body),
			Lines:            lineCount(text),
			SHA:              shortDocumentHash(text),
			CreatesIssue:     strings.Contains(text, "gh issue create") || strings.Contains(text, "gitclaw-doctor-live-issue"),
			HasCleanup:       strings.Contains(text, "trap cleanup EXIT") && strings.Contains(text, "gh issue close"),
			ModelCoverage:    doctorScriptHasModelCoverage(text),
			ModelFollowup:    doctorScriptHasModelFollowupCoverage(text),
			SessionCoverage:  strings.Contains(lower, "session coverage") || strings.Contains(text, "gitclaw session coverage"),
			BackupGate:       strings.Contains(text, "gitclaw-backups") || strings.Contains(text, "backup_checkout") || strings.Contains(text, "gitclaw backup coverage"),
			WorkflowDispatch: doctorScriptHasWorkflowDispatchCoverage(text),
		}
		surface.Scripts = append(surface.Scripts, script)
		surface.ScriptCount++
		if script.CreatesIssue {
			surface.LiveIssueScripts++
		}
		if script.HasCleanup {
			surface.CleanupScripts++
		}
		if script.ModelCoverage {
			surface.ModelCoverageScripts++
		}
		if script.ModelFollowup {
			surface.ModelFollowupScripts++
		}
		if script.SessionCoverage {
			surface.SessionCoverageScripts++
		}
		if script.BackupGate {
			surface.BackupGateScripts++
		}
		if script.WorkflowDispatch {
			surface.WorkflowDispatchScripts++
		}
	}
	return surface
}

func doctorScriptHasModelCoverage(text string) bool {
	return strings.Contains(text, "prompt_context_sha256_12") ||
		strings.Contains(text, `model="openai/`) ||
		strings.Contains(text, "GitHub Models") ||
		strings.Contains(text, "gitclaw.search_files")
}

func doctorScriptHasModelFollowupCoverage(text string) bool {
	return strings.Contains(text, "gh issue comment") &&
		strings.Contains(text, "issue_comment") &&
		doctorScriptWaitsForAssistantCount(text) &&
		strings.Contains(text, "prompt_context_sha256_12") &&
		strings.Contains(text, "gitclaw.search_files")
}

func doctorScriptWaitsForAssistantCount(text string) bool {
	for count := 1; count <= 9; count++ {
		if strings.Contains(text, fmt.Sprintf("wait_for_assistant_count %d", count)) {
			return true
		}
	}
	return false
}

func doctorScriptHasWorkflowDispatchCoverage(text string) bool {
	for _, marker := range []string{
		"--event workflow_dispatch",
		"gh workflow run",
		"workflow_dispatch_trigger",
		"workflow_dispatch_channel_bridge",
		"wake_strategy: `workflow_dispatch",
		"workflow has `workflow_dispatch`",
		"workflow_dispatch run",
		"GitHub Actions workflow_dispatch",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
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
			Name:   "e2e_harnesses",
			Status: doctorStatus(doctorE2EHealthy(surface.E2E)),
			Detail: fmt.Sprintf("%d script(s), %d model coverage, %d model follow-up, %d session coverage, %d backup gates", surface.E2E.ScriptCount, surface.E2E.ModelCoverageScripts, surface.E2E.ModelFollowupScripts, surface.E2E.SessionCoverageScripts, surface.E2E.BackupGateScripts),
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

func doctorE2EHealthy(surface doctorE2ESurface) bool {
	if surface.ScriptCount == 0 || surface.LiveIssueScripts == 0 {
		return false
	}
	if surface.CleanupScripts != surface.ScriptCount {
		return false
	}
	return surface.ModelCoverageScripts > 0 && surface.ModelFollowupScripts > 0 && surface.SessionCoverageScripts > 0 && surface.BackupGateScripts > 0
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

func writeDoctorE2ESurface(b *strings.Builder, surface doctorE2ESurface) {
	fmt.Fprintf(b, "- e2e_coverage_status=`%s` scripts=`%d` live_issue_scripts=`%d` cleanup_scripts=`%d` model_coverage_scripts=`%d` model_followup_scripts=`%d` session_coverage_scripts=`%d` backup_gate_scripts=`%d` workflow_dispatch_scripts=`%d`\n",
		doctorStatus(doctorE2EHealthy(surface)),
		surface.ScriptCount,
		surface.LiveIssueScripts,
		surface.CleanupScripts,
		surface.ModelCoverageScripts,
		surface.ModelFollowupScripts,
		surface.SessionCoverageScripts,
		surface.BackupGateScripts,
		surface.WorkflowDispatchScripts,
	)
	wrote := false
	for _, script := range surface.Scripts {
		if !script.ModelCoverage && !script.ModelFollowup && !script.SessionCoverage && !script.BackupGate && !script.WorkflowDispatch {
			continue
		}
		fmt.Fprintf(b, "- path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` live_issue=`%t` cleanup=`%t` model_coverage=`%t` model_followup=`%t` session_coverage=`%t` backup_gate=`%t` workflow_dispatch=`%t`\n",
			script.Path,
			script.Bytes,
			script.Lines,
			script.SHA,
			script.CreatesIssue,
			script.HasCleanup,
			script.ModelCoverage,
			script.ModelFollowup,
			script.SessionCoverage,
			script.BackupGate,
			script.WorkflowDispatch,
		)
		wrote = true
	}
	if !wrote {
		b.WriteString("- coverage_evidence_scripts=`none`\n")
	}
}

func PrintDoctorReport(cfg Config) {
	repoContext, _ := LoadRepoContextWithConfig(cfg.Workdir, nil, cfg)
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		fmt.Println(RenderDoctorReport(Event{Repo: repo}, cfg, repoContext))
		return
	}
	fmt.Println(RenderDoctorCLIReport(cfg, repoContext))
}
