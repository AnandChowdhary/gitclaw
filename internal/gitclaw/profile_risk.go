package gitclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProfileRiskFinding struct {
	Severity string
	Code     string
	Category string
	Kind     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type ProfileRiskReport struct {
	Status                               string
	VerificationScope                    string
	ProfileStrategy                      string
	ProfileStore                         string
	ProfileScope                         string
	ProfileDocumentsLoaded               int
	ScannedProfileDocuments              int
	RequiredProfileDocuments             int
	RequiredProfileDocumentsPresent      int
	RequiredProfileDocumentsMissing      int
	IdentityPolicyFiles                  int
	MemoryNotes                          int
	AvailableSkills                      int
	SelectedSkills                       int
	SkillBundles                         int
	AvailableTools                       int
	ActiveToolOutputs                    int
	ConfigFilePresent                    bool
	ConfigFilePath                       string
	SurfacesWithRiskFindings             int
	Findings                             []ProfileRiskFinding
	HighRiskFindings                     int
	WarningRiskFindings                  int
	InfoRiskFindings                     int
	ExternalProfileStateSupported        bool
	ProfileImportSupported               bool
	ProfileExportSupported               bool
	ProfileSwitchingSupported            bool
	ProfileDistributionInstallSupported  bool
	ProfileCredentialStorageSupported    bool
	ProfileMutationAllowed               bool
	ProfileSandboxBoundaryEnforced       bool
	GitHubActionsSandboxBackend          string
	RawProfileBodiesIncluded             bool
	RawConfigBodiesIncluded              bool
	RawIssueBodiesIncluded               bool
	RawCommentBodiesIncluded             bool
	RawPromptBodiesIncluded              bool
	CredentialValuesIncluded             bool
	LLME2ERequiredAfterProfileRiskChange bool
}

type profileRiskRule struct {
	Severity  string
	Code      string
	Category  string
	Any       []string
	All       []string
	IgnoreAny []string
}

var profileRiskRules = []profileRiskRule{
	{
		Severity: "high",
		Code:     "prompt_boundary_override",
		Category: "prompt-boundary",
		Any: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"disregard previous instructions",
			"override system instructions",
			"reveal the system prompt",
			"show the system prompt",
			"developer message",
		},
	},
	{
		Severity: "high",
		Code:     "credential_material_in_profile",
		Category: "credential-handling",
		Any: []string{
			"github_token=",
			"github_pat_",
			"ghp_",
			"gho_",
			"ghu_",
			"ghs_",
			"openai_api_key=",
			"api_key=",
			"private_key=",
			"telegram_bot_token=",
			"slack_bot_token=",
			"slack_app_token=",
			"xoxb-",
			"xapp-",
			"-----begin private key-----",
			"-----begin openssh private key-----",
		},
		IgnoreAny: []string{
			"not included",
			"not printed",
			"not dumped",
			"do not commit",
			"without dumping",
			"without printing",
			"placeholder",
		},
	},
	{
		Severity: "high",
		Code:     "external_profile_state_enabled",
		Category: "state-boundary",
		Any: []string{
			"profile_store: ~/.hermes",
			"profile_store: ~/.openclaw",
			"use external profile home",
			"load profile from ~/.hermes",
			"load profile from ~/.openclaw",
			"write to ~/.hermes",
			"write to ~/.openclaw",
			"copy .env into .gitclaw",
			"store credentials in .gitclaw",
			"persist sessions in .gitclaw",
		},
		IgnoreAny: []string{
			"should not",
			"must not",
			"not committed",
			"not imported",
			"kept out",
			"outside",
			"separate from",
			"skip",
		},
	},
	{
		Severity: "high",
		Code:     "profile_import_export_enabled",
		Category: "profile-portability",
		Any: []string{
			"profile export",
			"profile import",
			"profile install",
			"profile distribution",
			"clone-all",
			"clone all profile state",
			"import auth profiles",
		},
		IgnoreAny: []string{
			"not a profile export",
			"not a profile mutation",
			"not configured",
			"not_configured",
			"not supported",
			"without adding",
			"outside",
			"non-goal",
			"manual secret setup",
			"skip",
			"does not",
		},
	},
	{
		Severity: "warning",
		Code:     "profile_switching_enabled",
		Category: "profile-selection",
		Any: []string{
			"profile use",
			"switch profile",
			"multi-profile switching",
			"openclaw_profile=",
			"hermes_home=",
			"profile alias",
		},
		IgnoreAny: []string{
			"without adding",
			"not supported",
			"not configured",
			"does not",
			"outside",
		},
	},
	{
		Severity: "high",
		Code:     "profile_mutation_enabled",
		Category: "profile-mutation",
		Any: []string{
			"mutate profile",
			"edit soul.md automatically",
			"edit user.md automatically",
			"overwrite soul.md",
			"overwrite user.md",
			"append to soul.md",
			"append to user.md",
			"write memory.md",
			"create profile",
			"delete profile",
		},
		IgnoreAny: []string{
			"not a profile mutation",
			"without adding",
			"must not",
			"do not",
			"does not",
			"not supported",
			"preview-first",
			"edit-plan",
		},
	},
	{
		Severity: "warning",
		Code:     "profile_claimed_sandbox_boundary",
		Category: "sandbox-boundary",
		Any: []string{
			"profile is a sandbox",
			"profile provides filesystem isolation",
			"profile enforces sandbox",
			"profile blocks access outside",
		},
		IgnoreAny: []string{
			"not a sandbox",
			"does not sandbox",
			"not enforce",
			"not reliable",
		},
	},
	{
		Severity: "high",
		Code:     "raw_profile_body_leakage",
		Category: "body-leakage",
		Any: []string{
			"dump profile files",
			"dump profile bodies",
			"print profile files",
			"print profile bodies",
			"include profile file bodies",
			"include full profile body",
			"cat .gitclaw",
		},
		IgnoreAny: []string{
			"does not dump",
			"never dumps",
			"not printed",
			"without dumping",
			"without printing",
			"not include",
		},
	},
}

func BuildProfileRiskReport(cfg Config, repoContext RepoContext) ProfileRiskReport {
	profileDocs := profileDocuments(repoContext.Documents)
	validation := ValidateSoulContext(repoContext)
	configSurface := inspectConfigSurface(cfg.Workdir)
	report := ProfileRiskReport{
		Status:                               "ok",
		VerificationScope:                    "repo_local_profile_isolation",
		ProfileStrategy:                      "repo-local-git-profile",
		ProfileStore:                         ".gitclaw/",
		ProfileScope:                         "repository",
		ProfileDocumentsLoaded:               len(profileDocs),
		ScannedProfileDocuments:              len(profileDocs),
		RequiredProfileDocuments:             len(requiredSoulDocumentPaths),
		RequiredProfileDocumentsPresent:      validation.PresentRequiredFiles,
		RequiredProfileDocumentsMissing:      validation.MissingRequiredFiles,
		IdentityPolicyFiles:                  soulIdentityDocumentCount(repoContext.Documents),
		MemoryNotes:                          soulMemoryDocumentCount(repoContext.Documents),
		AvailableSkills:                      len(repoContext.SkillSummaries),
		SelectedSkills:                       len(repoContext.Skills),
		SkillBundles:                         len(repoContext.SkillBundles),
		AvailableTools:                       len(toolReportContracts),
		ActiveToolOutputs:                    len(repoContext.ToolOutputs),
		ConfigFilePresent:                    configSurface.ConfigFile.Present,
		ConfigFilePath:                       gitclawConfigPath,
		ExternalProfileStateSupported:        false,
		ProfileImportSupported:               false,
		ProfileExportSupported:               false,
		ProfileSwitchingSupported:            false,
		ProfileDistributionInstallSupported:  false,
		ProfileCredentialStorageSupported:    false,
		ProfileMutationAllowed:               false,
		ProfileSandboxBoundaryEnforced:       false,
		GitHubActionsSandboxBackend:          "github-actions",
		RawProfileBodiesIncluded:             false,
		RawConfigBodiesIncluded:              false,
		RawIssueBodiesIncluded:               false,
		RawCommentBodiesIncluded:             false,
		RawPromptBodiesIncluded:              false,
		CredentialValuesIncluded:             false,
		LLME2ERequiredAfterProfileRiskChange: true,
	}

	for _, finding := range validation.Findings {
		report.Findings = append(report.Findings, profileRiskFindingFromSoulValidation(finding))
	}
	for _, doc := range profileDocs {
		report.Findings = append(report.Findings, scanProfileRiskText("profile-document", doc.Path, doc.Path, "body", doc.Body)...)
	}
	if configSurface.ConfigFile.Present {
		report.Findings = append(report.Findings, scanProfileRiskText("profile-config", gitclawConfigPath, gitclawConfigPath, "body", readProfileRiskBody(cfg.Workdir, gitclawConfigPath))...)
	}

	sortProfileRiskFindings(report.Findings)
	report.SurfacesWithRiskFindings = profileRiskSurfaceCount(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func renderProfileRiskReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildProfileRiskReport(cfg, repoContext)
	profileDocs := profileDocuments(repoContext.Documents)
	configFile := inspectConfigSurface(cfg.Workdir).ConfigFile
	var b strings.Builder
	b.WriteString("## GitClaw Profile Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeProfileRiskSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report scans the repo-local profile envelope for profile-isolation, credential, import/export, profile-switching, mutation, sandbox-boundary, and raw-body leakage risks inspired by Hermes profiles and OpenClaw workspace memory. It reports metadata, paths, categories, finding codes, severities, and line hashes only; profile bodies, config bodies, skill bodies, tool outputs, issue bodies, comments, prompts, credentials, and secret values are not included.\n\n")

	b.WriteString("### Profile Isolation Risk Card\n")
	writeProfileIsolationRiskCard(&b, report)

	b.WriteString("\n### Config Risk Card\n")
	writeProfileConfigRiskCard(&b, cfg.Workdir, configFile)

	b.WriteString("\n### Profile Document Risk Cards\n")
	if len(profileDocs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, doc := range profileDocs {
			writeProfileDocumentRiskCard(&b, doc)
		}
	}

	b.WriteString("\n### Skill Profile Risk Cards\n")
	writeProfileSkillRiskCards(&b, repoContext)

	b.WriteString("\n### Current Profile Request Risk Card\n")
	if includeIssue {
		b.WriteString("- kind=`current-profile-request` current_issue_profile_request=`true` issue_body_scanned=`false` comment_bodies_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n")
	} else {
		b.WriteString("- kind=`current-profile-request` scope=`local-cli` current_issue_profile_request=`false`\n")
	}

	b.WriteString("\n### Risk Findings\n")
	writeProfileRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeProfileRiskSummary(b *strings.Builder, report ProfileRiskReport) {
	fmt.Fprintf(b, "- profile_risk_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- verification_scope: `%s`\n", report.VerificationScope)
	fmt.Fprintf(b, "- profile_strategy: `%s`\n", report.ProfileStrategy)
	fmt.Fprintf(b, "- profile_store: `%s`\n", report.ProfileStore)
	fmt.Fprintf(b, "- profile_scope: `%s`\n", report.ProfileScope)
	fmt.Fprintf(b, "- profile_documents_loaded: `%d`\n", report.ProfileDocumentsLoaded)
	fmt.Fprintf(b, "- scanned_profile_documents: `%d`\n", report.ScannedProfileDocuments)
	fmt.Fprintf(b, "- required_profile_documents: `%d`\n", report.RequiredProfileDocuments)
	fmt.Fprintf(b, "- required_profile_documents_present: `%d`\n", report.RequiredProfileDocumentsPresent)
	fmt.Fprintf(b, "- required_profile_documents_missing: `%d`\n", report.RequiredProfileDocumentsMissing)
	fmt.Fprintf(b, "- identity_policy_files: `%d`\n", report.IdentityPolicyFiles)
	fmt.Fprintf(b, "- memory_notes: `%d`\n", report.MemoryNotes)
	fmt.Fprintf(b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(b, "- skill_bundles: `%d`\n", report.SkillBundles)
	fmt.Fprintf(b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(b, "- config_file_present: `%t`\n", report.ConfigFilePresent)
	fmt.Fprintf(b, "- config_file_path: `%s`\n", report.ConfigFilePath)
	fmt.Fprintf(b, "- surfaces_with_risk_findings: `%d`\n", report.SurfacesWithRiskFindings)
	fmt.Fprintf(b, "- profile_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- external_profile_state_supported: `%t`\n", report.ExternalProfileStateSupported)
	fmt.Fprintf(b, "- profile_import_supported: `%t`\n", report.ProfileImportSupported)
	fmt.Fprintf(b, "- profile_export_supported: `%t`\n", report.ProfileExportSupported)
	fmt.Fprintf(b, "- profile_switching_supported: `%t`\n", report.ProfileSwitchingSupported)
	fmt.Fprintf(b, "- profile_distribution_install_supported: `%t`\n", report.ProfileDistributionInstallSupported)
	fmt.Fprintf(b, "- profile_credential_storage_supported: `%t`\n", report.ProfileCredentialStorageSupported)
	fmt.Fprintf(b, "- profile_mutation_allowed: `%t`\n", report.ProfileMutationAllowed)
	fmt.Fprintf(b, "- profile_sandbox_boundary_enforced: `%t`\n", report.ProfileSandboxBoundaryEnforced)
	fmt.Fprintf(b, "- github_actions_sandbox_backend: `%s`\n", report.GitHubActionsSandboxBackend)
	fmt.Fprintf(b, "- raw_profile_bodies_included: `%t`\n", report.RawProfileBodiesIncluded)
	fmt.Fprintf(b, "- raw_config_bodies_included: `%t`\n", report.RawConfigBodiesIncluded)
	fmt.Fprintf(b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(b, "- credential_values_included: `%t`\n", report.CredentialValuesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_profile_risk_change: `%t`\n", report.LLME2ERequiredAfterProfileRiskChange)
}

func writeProfileIsolationRiskCard(b *strings.Builder, report ProfileRiskReport) {
	fmt.Fprintf(
		b,
		"- kind=`profile-isolation` strategy=`%s` store=`%s` scope=`%s` external_profile_state_supported=`%t` profile_import_supported=`%t` profile_export_supported=`%t` profile_switching_supported=`%t` profile_distribution_install_supported=`%t` profile_credential_storage_supported=`%t` profile_mutation_allowed=`%t` profile_sandbox_boundary_enforced=`%t` github_actions_sandbox_backend=`%s` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n",
		report.ProfileStrategy,
		report.ProfileStore,
		report.ProfileScope,
		report.ExternalProfileStateSupported,
		report.ProfileImportSupported,
		report.ProfileExportSupported,
		report.ProfileSwitchingSupported,
		report.ProfileDistributionInstallSupported,
		report.ProfileCredentialStorageSupported,
		report.ProfileMutationAllowed,
		report.ProfileSandboxBoundaryEnforced,
		report.GitHubActionsSandboxBackend,
	)
}

func writeProfileConfigRiskCard(b *strings.Builder, root string, file configSurfaceFile) {
	findings := []ProfileRiskFinding(nil)
	if file.Present {
		findings = scanProfileRiskText("profile-config", file.Path, file.Path, "body", readProfileRiskBody(root, file.Path))
	}
	if !file.Present {
		fmt.Fprintf(b, "- kind=`profile-config` path=`%s` present=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none` line_hashes=`none`\n", file.Path)
		return
	}
	fmt.Fprintf(
		b,
		"- kind=`profile-config` path=`%s` present=`true` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		file.Path,
		file.Bytes,
		file.Lines,
		file.SHA,
		len(findings),
		profileRiskMaxSeverity(findings),
		inlineListOrNone(profileRiskCodes(findings)),
		inlineListOrNone(profileRiskLineHashes(findings)),
	)
}

func writeProfileDocumentRiskCard(b *strings.Builder, doc ContextDocument) {
	findings := scanProfileRiskText("profile-document", doc.Path, doc.Path, "body", doc.Body)
	fmt.Fprintf(
		b,
		"- kind=`profile-document` path=`%s` category=`%s` required=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s` line_hashes=`%s`\n",
		doc.Path,
		profileDocumentCategory(doc.Path),
		isRequiredSoulDocument(doc.Path),
		len(doc.Body),
		lineCount(doc.Body),
		shortDocumentHash(doc.Body),
		len(findings),
		profileRiskMaxSeverity(findings),
		inlineListOrNone(profileRiskCodes(findings)),
		inlineListOrNone(profileRiskLineHashes(findings)),
	)
}

func writeProfileSkillRiskCards(b *strings.Builder, repoContext RepoContext) {
	if len(repoContext.SkillSummaries) == 0 {
		b.WriteString("- none\n")
		return
	}
	selected := map[string]bool{}
	for _, skill := range repoContext.Skills {
		for _, summary := range repoContext.SkillSummaries {
			if summary.Path == skill.Path {
				selected[summary.Name] = true
			}
		}
	}
	for _, skill := range repoContext.SkillSummaries {
		fmt.Fprintf(b, "- kind=`profile-skill` name=`%s` enabled=`%t` selected=`%t` always=`%t` path=`%s` sha256_12=`%s` body_scanned=`false` risk_findings=`0` risk_max_severity=`none` risk_codes=`none`\n", skill.Name, skill.Enabled, selected[skill.Name], skill.Always, skill.Path, skill.SHA)
	}
}

func scanProfileRiskText(kind, name, path, field, body string) []ProfileRiskFinding {
	var findings []ProfileRiskFinding
	lines := strings.Split(body, "\n")
	for lineNumber, line := range lines {
		lower := strings.ToLower(line)
		contextLower := strings.ToLower(profileRiskLineContext(lines, lineNumber))
		for _, rule := range profileRiskRules {
			if !profileRiskRuleMatches(lower, contextLower, rule) {
				continue
			}
			findings = append(findings, ProfileRiskFinding{Severity: rule.Severity, Code: rule.Code, Category: rule.Category, Kind: kind, Path: path, Field: field, Line: lineNumber + 1, LineSHA: shortDocumentHash(line)})
		}
	}
	sortProfileRiskFindings(findings)
	return findings
}

func profileRiskRuleMatches(lowerLine, lowerContext string, rule profileRiskRule) bool {
	for _, ignored := range rule.IgnoreAny {
		if strings.Contains(lowerContext, ignored) {
			return false
		}
	}
	for _, required := range rule.All {
		if !strings.Contains(lowerLine, required) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, phrase := range rule.Any {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	return false
}

func profileRiskLineContext(lines []string, lineNumber int) string {
	var context []string
	start := lineNumber - 2
	if start < 0 {
		start = 0
	}
	end := lineNumber + 2
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		context = append(context, lines[i])
	}
	return strings.Join(context, " ")
}

func profileRiskFindingFromSoulValidation(finding SoulValidationFinding) ProfileRiskFinding {
	severity := "info"
	switch finding.Severity {
	case "error":
		severity = "high"
	case "warning":
		severity = "warning"
	}
	code := finding.Code
	category := "profile-validation"
	if finding.Code == "missing_required_context_file" {
		code = "missing_required_profile_document"
		category = "required-profile-document"
	}
	if finding.Code == "empty_context_file" {
		code = "empty_profile_document"
	}
	if finding.Code == "context_file_at_limit" {
		code = "profile_document_at_context_limit"
	}
	return ProfileRiskFinding{
		Severity: severity,
		Code:     code,
		Category: category,
		Kind:     "profile-document",
		Path:     finding.Path,
		Field:    "validation",
		LineSHA:  shortDocumentHash(finding.Path + ":" + finding.Code),
	}
}

func readProfileRiskBody(root, relPath string) string {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(absRoot, filepath.FromSlash(relPath)))
	if err != nil {
		return ""
	}
	return string(body)
}

func writeProfileRiskFindings(b *strings.Builder, findings []ProfileRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` kind=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Kind, finding.Path, finding.Field, finding.Line, finding.LineSHA)
	}
}

func profileRiskSurfaceCount(findings []ProfileRiskFinding) int {
	seen := map[string]bool{}
	for _, finding := range findings {
		key := finding.Kind + "\x00" + finding.Path
		if key == "\x00" {
			continue
		}
		seen[key] = true
	}
	return len(seen)
}

func profileRiskCodes(findings []ProfileRiskFinding) []string {
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

func profileRiskLineHashes(findings []ProfileRiskFinding) []string {
	seen := map[string]bool{}
	var hashes []string
	for _, finding := range findings {
		if finding.LineSHA == "" || seen[finding.LineSHA] {
			continue
		}
		seen[finding.LineSHA] = true
		hashes = append(hashes, finding.LineSHA)
	}
	sort.Strings(hashes)
	return hashes
}

func profileRiskMaxSeverity(findings []ProfileRiskFinding) string {
	max := "none"
	for _, finding := range findings {
		if profileRiskSeverityRank(finding.Severity) > profileRiskSeverityRank(max) {
			max = finding.Severity
		}
	}
	return max
}

func profileRiskSeverityRank(severity string) int {
	switch severity {
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

func sortProfileRiskFindings(findings []ProfileRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return profileRiskSeverityRank(findings[i].Severity) > profileRiskSeverityRank(findings[j].Severity)
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Field != findings[j].Field {
			return findings[i].Field < findings[j].Field
		}
		return findings[i].Line < findings[j].Line
	})
}
