package gitclaw

import (
	"fmt"
	"strings"
)

type migrationPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func IsMigrationReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/migrate" || command == "/migration"
}

func RenderMigrationReport(ev Event, cfg Config, repoContext RepoContext) string {
	source := requestedMigrationPlanSource(ev, cfg)
	if source == "__missing__" {
		source = ""
	}
	return renderMigrationPlanReport(ev, repoContext, source, true)
}

func RenderMigrationCLIReport(repoContext RepoContext, source string) string {
	return renderMigrationPlanReport(Event{}, repoContext, source, false)
}

func renderMigrationPlanReport(ev Event, repoContext RepoContext, source string, includeIssue bool) string {
	requested := cleanMigrationSource(source)
	normalized := normalizeMigrationSource(requested)
	supported := supportedMigrationSource(normalized)
	soulValidation := ValidateSoulContext(repoContext)
	skillValidation := ValidateSkillSummaries(repoContext.SkillSummaries)
	toolValidation := ValidateTools(repoContext)
	findings := migrationPlanFindings(requested, normalized, supported, soulValidation, skillValidation, toolValidation)
	status := migrationPlanStatus(findings)

	var b strings.Builder
	b.WriteString("## GitClaw Migration Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- migration_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- requested_source_sha256_12: `%s`\n", shortDocumentHash(requested))
	fmt.Fprintf(&b, "- normalized_source: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- supported_source: `%t`\n", supported)
	fmt.Fprintf(&b, "- plan_scope: `%s`\n", "repo-local-declarative-state")
	fmt.Fprintf(&b, "- source_scan_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- apply_supported: `%t`\n", false)
	fmt.Fprintf(&b, "- model_call_required: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- backup_required_before_apply: `%t`\n", true)
	fmt.Fprintf(&b, "- credentials_import_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- executable_state_import_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_body_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_secret_values_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	fmt.Fprintf(&b, "- existing_context_documents: `%d`\n", len(repoContext.Documents))
	fmt.Fprintf(&b, "- required_context_files: `%d`\n", soulValidation.RequiredFiles)
	fmt.Fprintf(&b, "- required_context_files_present: `%d`\n", soulValidation.PresentRequiredFiles)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", soulValidation.MemoryNotes)
	fmt.Fprintf(&b, "- tool_contracts: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", 1)
	writeSoulValidationSummary(&b, soulValidation)
	writeSkillValidationSummary(&b, skillValidation)
	writeToolsValidationSummary(&b, toolValidation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is a migration planner only. It maps known agent-state surfaces into GitClaw's repo-reviewed layout without scanning source homes, importing secrets, executing installers, mutating files, or calling a model.\n\n")

	b.WriteString("### Source Import Map\n")
	writeMigrationImportMap(&b, normalized, supported)

	b.WriteString("\n### Current GitClaw Target Inventory\n")
	writeMigrationTargetInventory(&b, repoContext)

	b.WriteString("\n### Review Steps\n")
	if !supported {
		b.WriteString("1. Choose one supported source: `openclaw`, `hermes`, `codex`, or `claude`.\n")
	} else {
		b.WriteString("1. Run the source system's own dry-run or export command and keep its report out of issue comments if it contains bodies or secrets.\n")
		b.WriteString("2. Verify the current GitClaw backup branch before applying any imported state.\n")
		b.WriteString("3. Copy only reviewed declarative files into `.gitclaw/`, `AGENTS.md`, or `.gitclaw/SKILLS/`; quarantine credentials, hooks, plugins, MCP definitions, caches, logs, sessions, and generated state for manual review.\n")
		b.WriteString("4. Re-run `/soul verify`, `/skills verify`, `/tools verify`, and a live GitHub Models conversation E2E that performs an actual model call after any migration change.\n")
	}

	b.WriteString("\n### Findings\n")
	writeMigrationPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func requestedMigrationPlanSource(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 || (fields[0] != "/migrate" && fields[0] != "/migration") {
		return ""
	}
	if len(fields) == 1 {
		return "__missing__"
	}
	if strings.EqualFold(fields[1], "plan") || strings.EqualFold(fields[1], "dry-run") || strings.EqualFold(fields[1], "from") {
		if len(fields) < 3 {
			return "__missing__"
		}
		return cleanMigrationSource(fields[2])
	}
	return cleanMigrationSource(fields[1])
}

func cleanMigrationSource(source string) string {
	return strings.Trim(strings.TrimSpace(source), " \t\r\n.,:;!?`\"'")
}

func normalizeMigrationSource(source string) string {
	source = strings.ToLower(cleanMigrationSource(source))
	switch source {
	case "openclaw", "claw", "clawdia":
		return "openclaw"
	case "hermes", "hermes-agent":
		return "hermes"
	case "codex", "codex-cli":
		return "codex"
	case "claude", "claude-code":
		return "claude"
	default:
		return source
	}
}

func supportedMigrationSource(source string) bool {
	switch source {
	case "openclaw", "hermes", "codex", "claude":
		return true
	default:
		return false
	}
}

func migrationPlanFindings(requested, normalized string, supported bool, soul SoulValidationReport, skills SkillValidationReport, tools ToolValidationReport) []migrationPlanFinding {
	var findings []migrationPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, migrationPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "preview_first", "migration planning is dry-run metadata only")
	add("info", "backup_first", "verify gitclaw-backups before applying any imported state")
	add("info", "credentials_manual", "credentials, tokens, auth profiles, cookies, and API keys are not imported by GitClaw migration planning")
	add("info", "executable_state_quarantined", "hooks, plugins, installers, MCP servers, caches, logs, raw sessions, and gateway state require manual review")
	if requested == "" || requested == "__missing__" {
		add("error", "source_missing", "provide one migration source")
	} else if !supported {
		add("error", "source_unsupported", "supported sources are openclaw, hermes, codex, and claude")
	} else {
		add("warning", "manual_review_required", fmt.Sprintf("%s migration requires human review before files are copied", normalized))
	}
	if soul.Errors > 0 {
		add("error", "soul_validation_errors_present", "fix current GitClaw soul validation errors before importing identity or memory")
	} else if soul.Warnings > 0 {
		add("warning", "soul_validation_warnings_present", "review current GitClaw soul validation warnings before importing identity or memory")
	}
	if skills.Errors > 0 {
		add("error", "skill_validation_errors_present", "fix current GitClaw skill validation errors before importing skills")
	} else if skills.Warnings > 0 {
		add("warning", "skill_validation_warnings_present", "review current GitClaw skill validation warnings before importing skills")
	}
	if tools.Errors > 0 {
		add("error", "tool_validation_errors_present", "fix current GitClaw tool validation errors before importing tool guidance")
	} else if tools.Warnings > 0 {
		add("warning", "tool_validation_warnings_present", "review current GitClaw tool validation warnings before importing tool guidance")
	}
	return findings
}

func migrationPlanStatus(findings []migrationPlanFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "needs_review"
		}
	}
	return "ok"
}

func writeMigrationImportMap(b *strings.Builder, source string, supported bool) {
	if !supported {
		b.WriteString("- none\n")
		return
	}
	for _, item := range migrationImportMap(source) {
		fmt.Fprintf(b, "- source_kind=`%s` target=`%s` action=`%s` review=`%s`\n",
			item[0],
			item[1],
			item[2],
			item[3],
		)
	}
}

func migrationImportMap(source string) [][4]string {
	switch source {
	case "hermes":
		return [][4]string{
			{"config.yaml providers", ".gitclaw/config.yml", "reviewed-merge", "model/provider config only"},
			{"SOUL.md", ".gitclaw/SOUL.md", "manual-copy", "identity/persona"},
			{"AGENTS.md", "AGENTS.md", "manual-copy", "workspace instructions"},
			{"memories/MEMORY.md", ".gitclaw/MEMORY.md", "reviewed-append", "long-term memory"},
			{"memories/USER.md", ".gitclaw/USER.md", "reviewed-append", "user profile"},
			{"skills/<name>/SKILL.md", ".gitclaw/SKILLS/<name>/SKILL.md", "manual-copy", "repo-local skills"},
			{"cron jobs", ".gitclaw/proactive/*.md", "manual-rewrite", "scheduled prompts only"},
			{"sessions/state.db", "gitclaw-backups archive", "archive-only", "raw sessions stay out of prompt context"},
			{"auth.json/.env", "manual secret setup", "skip", "credential import disabled"},
		}
	case "openclaw":
		return [][4]string{
			{"SOUL.md", ".gitclaw/SOUL.md", "manual-copy", "identity/persona"},
			{"USER.md", ".gitclaw/USER.md", "reviewed-merge", "user profile"},
			{"MEMORY.md", ".gitclaw/MEMORY.md", "reviewed-merge", "long-term memory"},
			{"memory/*.md", ".gitclaw/memory/YYYY-MM-DD.md", "manual-copy", "dated memory notes"},
			{"AGENTS.md", "AGENTS.md", "manual-copy", "workspace instructions"},
			{"skills/<name>/SKILL.md", ".gitclaw/SKILLS/<name>/SKILL.md", "manual-copy", "repo-local skills"},
			{"TOOLS.md/tool policy", ".gitclaw/TOOLS.md", "reviewed-merge", "tool guidance only"},
			{"sessions/logs", "gitclaw-backups archive", "archive-only", "raw state stays out of prompt context"},
			{"auth profiles/.env", "manual secret setup", "skip", "credential import disabled"},
		}
	case "codex":
		return [][4]string{
			{"$CODEX_HOME/skills", ".gitclaw/SKILLS/<name>/SKILL.md", "manual-copy", "exclude system/cache skills"},
			{"$HOME/.agents/skills", ".gitclaw/SKILLS/<name>/SKILL.md", "manual-copy", "personal AgentSkills only after review"},
			{"AGENTS.md", "AGENTS.md", "manual-copy", "workspace instructions"},
			{"config.toml", ".gitclaw/config.yml", "manual-rewrite", "model/profile settings only"},
			{"plugins/connectors", ".gitclaw/TOOLS.md", "manual-review", "do not activate automatically"},
			{"sessions/history", "gitclaw-backups archive", "archive-only", "raw sessions stay out of prompt context"},
			{"auth/tokens", "manual secret setup", "skip", "credential import disabled"},
		}
	case "claude":
		return [][4]string{
			{"CLAUDE.md", "AGENTS.md", "manual-copy", "workspace instructions"},
			{".claude/CLAUDE.md", ".gitclaw/GITCLAW.md", "manual-copy", "project instructions"},
			{"~/.claude/CLAUDE.md", ".gitclaw/USER.md", "reviewed-append", "user profile"},
			{"skills/<name>/SKILL.md", ".gitclaw/SKILLS/<name>/SKILL.md", "manual-copy", "repo-local skills"},
			{"commands/*.md", ".gitclaw/SKILLS/<name>/SKILL.md", "manual-rewrite", "manual invocation skills"},
			{"MCP config", ".gitclaw/TOOLS.md", "manual-review", "tool guidance only"},
			{"hooks/permissions/history", "gitclaw-backups archive", "archive-only", "executable state stays quarantined"},
			{"OAuth/Desktop credentials", "manual secret setup", "skip", "credential import disabled"},
		}
	default:
		return nil
	}
}

func writeMigrationTargetInventory(b *strings.Builder, repoContext RepoContext) {
	if len(repoContext.Documents) == 0 {
		b.WriteString("- context_documents=`0`\n")
	} else {
		for _, doc := range repoContext.Documents {
			fmt.Fprintf(b, "- kind=`context` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
		}
	}
	if len(repoContext.SkillSummaries) == 0 {
		b.WriteString("- skills=`0`\n")
	} else {
		for _, skill := range repoContext.SkillSummaries {
			fmt.Fprintf(b, "- kind=`skill` name=`%s` path=`%s` enabled=`%t` sha256_12=`%s`\n", inlineCode(skill.Name), skill.Path, skillIsEnabled(skill), skill.SHA)
		}
	}
	if len(repoContext.SkillBundles) == 0 {
		b.WriteString("- skill_bundles=`0`\n")
	} else {
		for _, bundle := range repoContext.SkillBundles {
			fmt.Fprintf(b, "- kind=`skill-bundle` name=`%s` path=`%s` resolved_skills=`%s` sha256_12=`%s`\n", inlineCode(bundle.Name), bundle.Path, inlineList(bundle.ResolvedSkills), bundle.SHA)
		}
	}
}

func writeMigrationPlanFindings(b *strings.Builder, findings []migrationPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
