package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

type MigrationRiskFinding struct {
	Severity   string
	Code       string
	Category   string
	Kind       string
	SourceKind string
	Target     string
	Action     string
	Review     string
	ItemSHA    string
}

type MigrationRiskReport struct {
	Status                                 string
	VerificationScope                      string
	RequestedSourceHash                    string
	NormalizedSource                       string
	SupportedSource                        bool
	ProviderImportItems                    int
	ManualCopyItems                        int
	ReviewedMergeItems                     int
	ReviewedAppendItems                    int
	ManualRewriteItems                     int
	ManualReviewItems                      int
	ArchiveOnlyItems                       int
	SkippedItems                           int
	CredentialItems                        int
	ExecutableStateItems                   int
	MemoryItems                            int
	SkillItems                             int
	SessionArchiveItems                    int
	SourceScanAllowed                      bool
	SourceHomeRead                         bool
	SourcePathsPrinted                     bool
	ApplySupported                         bool
	ModelCallRequired                      bool
	RepositoryMutationAllowed              bool
	CredentialsImportAllowed               bool
	ExecutableStateImportAllowed           bool
	InstallerExecutionAllowed              bool
	MCPAutoloadAllowed                     bool
	RawSourceBodyIncluded                  bool
	RawIssueBodiesIncluded                 bool
	RawCommentBodiesIncluded               bool
	RawSecretValuesIncluded                bool
	BackupRequiredBeforeApply              bool
	HumanReviewRequired                    bool
	QuarantineRequired                     bool
	SoulValidationStatus                   string
	SkillValidationStatus                  string
	ToolValidationStatus                   string
	SoulValidationErrors                   int
	SkillValidationErrors                  int
	ToolValidationErrors                   int
	SoulValidationWarnings                 int
	SkillValidationWarnings                int
	ToolValidationWarnings                 int
	SurfacesWithRiskFindings               int
	HighRiskFindings                       int
	WarningRiskFindings                    int
	InfoRiskFindings                       int
	LLME2ERequiredAfterMigrationRiskChange bool
	Items                                  []migrationRiskItem
	Findings                               []MigrationRiskFinding
}

type migrationRiskItem struct {
	SourceKind string
	Target     string
	Action     string
	Review     string
	Category   string
	Findings   []MigrationRiskFinding
}

func IsMigrationRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/migrate" && fields[0] != "/migration" {
		return false
	}
	if isMigrationRiskWord(fields[1]) {
		return true
	}
	return len(fields) >= 3 && isMigrationRiskWord(fields[2])
}

func RenderMigrationRiskReport(ev Event, cfg Config, repoContext RepoContext) string {
	source := requestedMigrationRiskSource(ev, cfg)
	if source == "__missing__" {
		source = ""
	}
	return renderMigrationRiskReport(ev, repoContext, source, true)
}

func RenderMigrationRiskCLIReport(repoContext RepoContext, source string) string {
	return renderMigrationRiskReport(Event{}, repoContext, source, false)
}

func renderMigrationRiskReport(ev Event, repoContext RepoContext, source string, includeIssue bool) string {
	report := BuildMigrationRiskReport(source, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Migration Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeMigrationRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report checks the migration boundary before importing OpenClaw, Hermes, Codex, or Claude state. It reports provider-map metadata, review categories, risk codes, and hashes only; source homes, source files, issue bodies, comments, prompts, skill bodies, credentials, and secret values are not included. It does not apply migrations, execute installers, load MCP servers, import credentials, dispatch workflows, or mutate the repository.\n\n")

	b.WriteString("### Source Boundary Risk Card\n")
	writeMigrationSourceRiskCard(&b, report)

	b.WriteString("\n### Apply Boundary Risk Card\n")
	writeMigrationApplyRiskCard(&b, report)

	b.WriteString("\n### Current Target Validation Cards\n")
	writeMigrationTargetRiskCards(&b, report)

	b.WriteString("\n### Provider Import Risk Cards\n")
	writeMigrationImportRiskCards(&b, report.Items)

	b.WriteString("\n### Risk Findings\n")
	writeMigrationRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildMigrationRiskReport(source string, repoContext RepoContext) MigrationRiskReport {
	requested := cleanMigrationSource(source)
	normalized := normalizeMigrationSource(requested)
	supported := supportedMigrationSource(normalized)
	soulValidation := ValidateSoulContext(repoContext)
	skillValidation := ValidateSkillSummaries(repoContext.SkillSummaries)
	toolValidation := ValidateTools(repoContext)
	report := MigrationRiskReport{
		Status:                                 "ok",
		VerificationScope:                      "agent_state_migration_boundary",
		RequestedSourceHash:                    shortDocumentHash(requested),
		NormalizedSource:                       normalized,
		SupportedSource:                        supported,
		SourceScanAllowed:                      false,
		SourceHomeRead:                         false,
		SourcePathsPrinted:                     false,
		ApplySupported:                         false,
		ModelCallRequired:                      false,
		RepositoryMutationAllowed:              false,
		CredentialsImportAllowed:               false,
		ExecutableStateImportAllowed:           false,
		InstallerExecutionAllowed:              false,
		MCPAutoloadAllowed:                     false,
		RawSourceBodyIncluded:                  false,
		RawIssueBodiesIncluded:                 false,
		RawCommentBodiesIncluded:               false,
		RawSecretValuesIncluded:                false,
		BackupRequiredBeforeApply:              true,
		HumanReviewRequired:                    true,
		QuarantineRequired:                     true,
		SoulValidationStatus:                   soulValidation.Status,
		SkillValidationStatus:                  skillValidation.Status,
		ToolValidationStatus:                   toolValidation.Status,
		SoulValidationErrors:                   soulValidation.Errors,
		SkillValidationErrors:                  skillValidation.Errors,
		ToolValidationErrors:                   toolValidation.Errors,
		SoulValidationWarnings:                 soulValidation.Warnings,
		SkillValidationWarnings:                skillValidation.Warnings,
		ToolValidationWarnings:                 toolValidation.Warnings,
		LLME2ERequiredAfterMigrationRiskChange: true,
	}
	if requested == "" || requested == "__missing__" {
		report.Findings = append(report.Findings, migrationRiskFinding("error", "source_missing", "source", "source-boundary", "", "", "", "", "provide one migration source"))
	} else if !supported {
		report.Findings = append(report.Findings, migrationRiskFinding("error", "source_unsupported", "source", "source-boundary", normalized, "", "", "", "supported sources are openclaw, hermes, codex, and claude"))
	} else {
		report.Findings = append(report.Findings,
			migrationRiskFinding("info", "preview_first", "planning", "source-boundary", normalized, "", "", "", "migration risk audit is dry-run metadata only"),
			migrationRiskFinding("info", "backup_first", "recovery", "apply-boundary", normalized, "", "", "", "verify backups before applying migrated state"),
			migrationRiskFinding("warning", "manual_review_required", "review", "apply-boundary", normalized, "", "", "", "human review is required before copying migrated state"),
		)
		report.Items = buildMigrationRiskItems(normalized)
		for _, item := range report.Items {
			report.Findings = append(report.Findings, item.Findings...)
			report.ProviderImportItems++
			switch item.Action {
			case "manual-copy":
				report.ManualCopyItems++
			case "reviewed-merge":
				report.ReviewedMergeItems++
			case "reviewed-append":
				report.ReviewedAppendItems++
			case "manual-rewrite":
				report.ManualRewriteItems++
			case "manual-review":
				report.ManualReviewItems++
			case "archive-only":
				report.ArchiveOnlyItems++
			case "skip":
				report.SkippedItems++
			}
			if item.Category == "credentials" {
				report.CredentialItems++
			}
			if item.Category == "executable-state" {
				report.ExecutableStateItems++
			}
			if item.Category == "memory" {
				report.MemoryItems++
			}
			if item.Category == "skills" {
				report.SkillItems++
			}
			if item.Category == "session-archive" {
				report.SessionArchiveItems++
			}
		}
	}
	if soulValidation.Errors > 0 {
		report.Findings = append(report.Findings, migrationRiskFinding("error", "target_soul_validation_errors", "target-validation", "target-validation", normalized, ".gitclaw/SOUL.md", "validate", "fix before migration", "current soul validation has errors"))
	} else if soulValidation.Warnings > 0 {
		report.Findings = append(report.Findings, migrationRiskFinding("warning", "target_soul_validation_warnings", "target-validation", "target-validation", normalized, ".gitclaw/SOUL.md", "validate", "review before migration", "current soul validation has warnings"))
	}
	if skillValidation.Errors > 0 {
		report.Findings = append(report.Findings, migrationRiskFinding("error", "target_skill_validation_errors", "target-validation", "target-validation", normalized, ".gitclaw/SKILLS", "validate", "fix before migration", "current skill validation has errors"))
	} else if skillValidation.Warnings > 0 {
		report.Findings = append(report.Findings, migrationRiskFinding("warning", "target_skill_validation_warnings", "target-validation", "target-validation", normalized, ".gitclaw/SKILLS", "validate", "review before migration", "current skill validation has warnings"))
	}
	if toolValidation.Errors > 0 {
		report.Findings = append(report.Findings, migrationRiskFinding("error", "target_tool_validation_errors", "target-validation", "target-validation", normalized, ".gitclaw/TOOLS.md", "validate", "fix before migration", "current tool validation has errors"))
	} else if toolValidation.Warnings > 0 {
		report.Findings = append(report.Findings, migrationRiskFinding("warning", "target_tool_validation_warnings", "target-validation", "target-validation", normalized, ".gitclaw/TOOLS.md", "validate", "review before migration", "current tool validation has warnings"))
	}
	sortMigrationRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = migrationRiskSurfaceCount(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high", "error":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case migrationRiskHasSeverity(report.Findings, "error"):
		report.Status = "blocked"
	case migrationRiskHasSeverity(report.Findings, "high"):
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "needs_review"
	default:
		report.Status = "ok"
	}
	return report
}

func buildMigrationRiskItems(source string) []migrationRiskItem {
	imports := migrationImportMap(source)
	items := make([]migrationRiskItem, 0, len(imports))
	for _, raw := range imports {
		item := migrationRiskItem{
			SourceKind: raw[0],
			Target:     raw[1],
			Action:     raw[2],
			Review:     raw[3],
		}
		item.Category = migrationRiskItemCategory(item)
		item.Findings = migrationRiskItemFindings(item)
		items = append(items, item)
	}
	return items
}

func migrationRiskItemCategory(item migrationRiskItem) string {
	lower := strings.ToLower(strings.Join([]string{item.SourceKind, item.Target, item.Action, item.Review}, " "))
	switch {
	case strings.Contains(lower, "auth") || strings.Contains(lower, "credential") || strings.Contains(lower, "secret") || strings.Contains(lower, ".env") || strings.Contains(lower, "token") || strings.Contains(lower, "cookie"):
		return "credentials"
	case strings.Contains(lower, "hook") || strings.Contains(lower, "plugin") || strings.Contains(lower, "mcp") || strings.Contains(lower, "installer") || strings.Contains(lower, "cron") || strings.Contains(lower, "command"):
		return "executable-state"
	case strings.Contains(lower, "session") || strings.Contains(lower, "logs") || strings.Contains(lower, "state.db") || strings.Contains(lower, "cache") || strings.Contains(lower, "history"):
		return "session-archive"
	case strings.Contains(lower, "skill"):
		return "skills"
	case strings.Contains(lower, "memory") || strings.Contains(lower, "memories") || strings.Contains(lower, "user.md"):
		return "memory"
	case strings.Contains(lower, "soul") || strings.Contains(lower, "agents.md"):
		return "identity"
	case strings.Contains(lower, "config") || strings.Contains(lower, "provider"):
		return "config"
	case strings.Contains(lower, "tool"):
		return "tools"
	default:
		return "declarative-state"
	}
}

func migrationRiskItemFindings(item migrationRiskItem) []MigrationRiskFinding {
	var findings []MigrationRiskFinding
	add := func(severity, code, category string) {
		findings = append(findings, migrationRiskFinding(severity, code, category, "provider-import", item.SourceKind, item.Target, item.Action, item.Review, ""))
	}
	switch item.Category {
	case "credentials":
		add("warning", "credential_import_disabled", "credentials")
	case "executable-state":
		add("warning", "executable_state_quarantined", "executable-state")
	case "session-archive":
		add("warning", "raw_state_archive_only", "session-archive")
	case "skills":
		add("warning", "skill_manual_review_required", "skills")
	case "memory":
		add("warning", "memory_review_required", "memory")
	}
	switch item.Action {
	case "skip":
		add("info", "import_item_skipped", "review")
	case "archive-only":
		add("info", "import_item_archive_only", "review")
	case "manual-review":
		add("warning", "manual_review_gate_required", "review")
	case "manual-copy", "reviewed-merge", "reviewed-append", "manual-rewrite":
		add("info", "review_gate_required", "review")
	}
	return findings
}

func writeMigrationRiskSummary(b *strings.Builder, report MigrationRiskReport) {
	fmt.Fprintf(b, "- migration_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- requested_source_sha256_12: `%s`\n", report.RequestedSourceHash)
	fmt.Fprintf(b, "- normalized_source: `%s`\n", inlineCode(report.NormalizedSource))
	fmt.Fprintf(b, "- supported_source: `%t`\n", report.SupportedSource)
	fmt.Fprintf(b, "- provider_import_items: `%d`\n", report.ProviderImportItems)
	fmt.Fprintf(b, "- manual_copy_items: `%d`\n", report.ManualCopyItems)
	fmt.Fprintf(b, "- reviewed_merge_items: `%d`\n", report.ReviewedMergeItems)
	fmt.Fprintf(b, "- reviewed_append_items: `%d`\n", report.ReviewedAppendItems)
	fmt.Fprintf(b, "- manual_rewrite_items: `%d`\n", report.ManualRewriteItems)
	fmt.Fprintf(b, "- manual_review_items: `%d`\n", report.ManualReviewItems)
	fmt.Fprintf(b, "- archive_only_items: `%d`\n", report.ArchiveOnlyItems)
	fmt.Fprintf(b, "- skipped_items: `%d`\n", report.SkippedItems)
	fmt.Fprintf(b, "- credential_items: `%d`\n", report.CredentialItems)
	fmt.Fprintf(b, "- executable_state_items: `%d`\n", report.ExecutableStateItems)
	fmt.Fprintf(b, "- memory_items: `%d`\n", report.MemoryItems)
	fmt.Fprintf(b, "- skill_items: `%d`\n", report.SkillItems)
	fmt.Fprintf(b, "- session_archive_items: `%d`\n", report.SessionArchiveItems)
	fmt.Fprintf(b, "- source_scan_allowed: `%t`\n", report.SourceScanAllowed)
	fmt.Fprintf(b, "- source_home_read: `%t`\n", report.SourceHomeRead)
	fmt.Fprintf(b, "- source_paths_printed: `%t`\n", report.SourcePathsPrinted)
	fmt.Fprintf(b, "- apply_supported: `%t`\n", report.ApplySupported)
	fmt.Fprintf(b, "- model_call_required: `%t`\n", report.ModelCallRequired)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- credentials_import_allowed: `%t`\n", report.CredentialsImportAllowed)
	fmt.Fprintf(b, "- executable_state_import_allowed: `%t`\n", report.ExecutableStateImportAllowed)
	fmt.Fprintf(b, "- installer_execution_allowed: `%t`\n", report.InstallerExecutionAllowed)
	fmt.Fprintf(b, "- mcp_autoload_allowed: `%t`\n", report.MCPAutoloadAllowed)
	fmt.Fprintf(b, "- raw_source_body_included: `%t`\n", report.RawSourceBodyIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_secret_values_included: `%t`\n", report.RawSecretValuesIncluded)
	fmt.Fprintf(b, "- backup_required_before_apply: `%t`\n", report.BackupRequiredBeforeApply)
	fmt.Fprintf(b, "- human_review_required: `%t`\n", report.HumanReviewRequired)
	fmt.Fprintf(b, "- quarantine_required: `%t`\n", report.QuarantineRequired)
	fmt.Fprintf(b, "- soul_validation_status: `%s`\n", report.SoulValidationStatus)
	fmt.Fprintf(b, "- skill_validation_status: `%s`\n", report.SkillValidationStatus)
	fmt.Fprintf(b, "- tool_validation_status: `%s`\n", report.ToolValidationStatus)
	fmt.Fprintf(b, "- soul_validation_errors: `%d`\n", report.SoulValidationErrors)
	fmt.Fprintf(b, "- skill_validation_errors: `%d`\n", report.SkillValidationErrors)
	fmt.Fprintf(b, "- tool_validation_errors: `%d`\n", report.ToolValidationErrors)
	fmt.Fprintf(b, "- soul_validation_warnings: `%d`\n", report.SoulValidationWarnings)
	fmt.Fprintf(b, "- skill_validation_warnings: `%d`\n", report.SkillValidationWarnings)
	fmt.Fprintf(b, "- tool_validation_warnings: `%d`\n", report.ToolValidationWarnings)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- migration_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- llm_e2e_required_after_migration_risk_change: `%t`\n", report.LLME2ERequiredAfterMigrationRiskChange)
}

func writeMigrationSourceRiskCard(b *strings.Builder, report MigrationRiskReport) {
	findings := migrationRiskFindingsForKind(report.Findings, "source-boundary")
	fmt.Fprintf(b, "- kind=`source-boundary` normalized_source=`%s` supported=`%t` source_scan_allowed=`%t` source_home_read=`%t` source_paths_printed=`%t` raw_source_body_included=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", inlineCode(report.NormalizedSource), report.SupportedSource, report.SourceScanAllowed, report.SourceHomeRead, report.SourcePathsPrinted, report.RawSourceBodyIncluded, len(findings), migrationRiskMaxSeverity(findings), inlineListOrNone(migrationRiskCodes(findings)))
}

func writeMigrationApplyRiskCard(b *strings.Builder, report MigrationRiskReport) {
	findings := migrationRiskFindingsForKind(report.Findings, "apply-boundary")
	fmt.Fprintf(b, "- kind=`apply-boundary` apply_supported=`%t` repository_mutation_allowed=`%t` credentials_import_allowed=`%t` executable_state_import_allowed=`%t` installer_execution_allowed=`%t` mcp_autoload_allowed=`%t` backup_required_before_apply=`%t` human_review_required=`%t` quarantine_required=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", report.ApplySupported, report.RepositoryMutationAllowed, report.CredentialsImportAllowed, report.ExecutableStateImportAllowed, report.InstallerExecutionAllowed, report.MCPAutoloadAllowed, report.BackupRequiredBeforeApply, report.HumanReviewRequired, report.QuarantineRequired, len(findings), migrationRiskMaxSeverity(findings), inlineListOrNone(migrationRiskCodes(findings)))
}

func writeMigrationTargetRiskCards(b *strings.Builder, report MigrationRiskReport) {
	findings := migrationRiskFindingsForKind(report.Findings, "target-validation")
	fmt.Fprintf(b, "- kind=`target-validation` component=`soul` status=`%s` errors=`%d` warnings=`%d`\n", report.SoulValidationStatus, report.SoulValidationErrors, report.SoulValidationWarnings)
	fmt.Fprintf(b, "- kind=`target-validation` component=`skills` status=`%s` errors=`%d` warnings=`%d`\n", report.SkillValidationStatus, report.SkillValidationErrors, report.SkillValidationWarnings)
	fmt.Fprintf(b, "- kind=`target-validation` component=`tools` status=`%s` errors=`%d` warnings=`%d`\n", report.ToolValidationStatus, report.ToolValidationErrors, report.ToolValidationWarnings)
	fmt.Fprintf(b, "- kind=`target-validation-rollup` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", len(findings), migrationRiskMaxSeverity(findings), inlineListOrNone(migrationRiskCodes(findings)))
}

func writeMigrationImportRiskCards(b *strings.Builder, items []migrationRiskItem) {
	if len(items) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "- kind=`provider-import` source_kind=`%s` target=`%s` action=`%s` review=`%s` category=`%s` raw_body_included=`false` credential_values_included=`false` installer_execution_allowed=`false` item_sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n", item.SourceKind, item.Target, item.Action, item.Review, item.Category, migrationRiskItemHash(item), len(item.Findings), migrationRiskMaxSeverity(item.Findings), inlineListOrNone(migrationRiskCodes(item.Findings)))
	}
}

func writeMigrationRiskFindings(b *strings.Builder, findings []MigrationRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` source_kind=`%s` target=`%s` action=`%s` review=`%s` item_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.SourceKind, finding.Target, finding.Action, finding.Review, finding.ItemSHA)
	}
}

func requestedMigrationRiskSource(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) == 0 || (fields[0] != "/migrate" && fields[0] != "/migration") {
		return ""
	}
	if len(fields) == 1 {
		return "__missing__"
	}
	if isMigrationRiskWord(fields[1]) {
		if len(fields) < 3 {
			return "__missing__"
		}
		return cleanMigrationSource(fields[2])
	}
	if len(fields) >= 3 && isMigrationRiskWord(fields[2]) {
		return cleanMigrationSource(fields[1])
	}
	return "__missing__"
}

func isMigrationRiskWord(value string) bool {
	return strings.EqualFold(value, "risk") || strings.EqualFold(value, "risk-audit")
}

func migrationRiskFinding(severity, code, category, kind, sourceKind, target, action, review, detail string) MigrationRiskFinding {
	itemSHA := shortDocumentHash(strings.Join([]string{severity, code, category, kind, sourceKind, target, action, review, detail}, "\x00"))
	return MigrationRiskFinding{
		Severity:   severity,
		Code:       code,
		Category:   category,
		Kind:       kind,
		SourceKind: sourceKind,
		Target:     target,
		Action:     action,
		Review:     review,
		ItemSHA:    itemSHA,
	}
}

func migrationRiskItemHash(item migrationRiskItem) string {
	return shortDocumentHash(strings.Join([]string{item.SourceKind, item.Target, item.Action, item.Review, item.Category}, "\x00"))
}

func migrationRiskFindingsForKind(findings []MigrationRiskFinding, kind string) []MigrationRiskFinding {
	var selected []MigrationRiskFinding
	for _, finding := range findings {
		if finding.Kind == kind {
			selected = append(selected, finding)
		}
	}
	return selected
}

func migrationRiskSurfaceCount(findings []MigrationRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.SourceKind + "\x00" + finding.Target
		if key == "\x00\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func migrationRiskCodes(findings []MigrationRiskFinding) []string {
	seen := map[string]bool{}
	var codes []string
	for _, finding := range findings {
		if finding.Code == "" || seen[finding.Code] {
			continue
		}
		seen[finding.Code] = true
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func migrationRiskMaxSeverity(findings []MigrationRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if migrationRiskSeverityRank(finding.Severity) > migrationRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	if max == "error" {
		return "high"
	}
	return max
}

func migrationRiskHasSeverity(findings []MigrationRiskFinding, severity string) bool {
	for _, finding := range findings {
		if finding.Severity == severity {
			return true
		}
	}
	return false
}

func migrationRiskSeverityRank(severity string) int {
	switch severity {
	case "error":
		return 4
	case "high":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func sortMigrationRiskFindings(findings []MigrationRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a := findings[i]
		b := findings[j]
		if rankA, rankB := migrationRiskSeverityRank(a.Severity), migrationRiskSeverityRank(b.Severity); rankA != rankB {
			return rankA > rankB
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.SourceKind != b.SourceKind {
			return a.SourceKind < b.SourceKind
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Target < b.Target
	})
}
