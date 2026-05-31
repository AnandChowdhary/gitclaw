package gitclaw

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type ProfileManifestReport struct {
	Status                                   string
	ProfileStrategy                          string
	ProfileStore                             string
	ProfileScope                             string
	ManifestStrategy                         string
	ManifestSupported                        bool
	ProfileExportSupported                   bool
	ProfileImportSupported                   bool
	ProfileSwitchingSupported                bool
	ProfileDistributionInstallSupported      bool
	ProfileMutationAllowed                   bool
	CredentialsIncluded                      bool
	SessionsIncluded                         bool
	BackupPayloadsIncluded                   bool
	RawBodiesIncluded                        bool
	RawConfigBodiesIncluded                  bool
	RawIssueBodiesIncluded                   bool
	RawCommentBodiesIncluded                 bool
	LLME2ERequiredAfterProfileManifestChange bool
	ProfileDocumentsLoaded                   int
	RequiredProfileDocuments                 int
	RequiredProfileDocumentsPresent          int
	RequiredProfileDocumentsMissing          int
	AvailableSkills                          int
	SelectedSkills                           int
	SkillBundles                             int
	AvailableTools                           int
	ActiveToolOutputs                        int
	ConfigFilePresent                        bool
	ManifestEntries                          int
	RepoTrackedEntries                       int
	PortableEntries                          int
	MetadataOnlyEntries                      int
	ContractOnlyEntries                      int
	SelectedEntries                          int
	EnabledEntries                           int
	ExcludedStateClasses                     int
	ManifestSHA                              string
	Entries                                  []ProfileManifestEntry
	ExcludedState                            []ProfileManifestExclusion
}

type ProfileManifestEntry struct {
	Kind          string
	Name          string
	Path          string
	Category      string
	Source        string
	IncludePolicy string
	Portable      bool
	Required      bool
	Present       bool
	Selected      bool
	Enabled       bool
	BodyIncluded  bool
	Bytes         int
	Lines         int
	SHA           string
}

type ProfileManifestExclusion struct {
	Kind   string
	Source string
	Reason string
}

type profileManifestGlobSpec struct {
	Kind          string
	Category      string
	Pattern       string
	IncludePolicy string
}

var profileManifestGlobSpecs = []profileManifestGlobSpec{
	{Kind: "agent-spec", Category: "agents", Pattern: ".gitclaw/agents/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "artifact-spec", Category: "artifacts", Pattern: ".gitclaw/artifacts/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "diff-spec", Category: "diffs", Pattern: ".gitclaw/diffs/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "hook-spec", Category: "hooks", Pattern: ".gitclaw/hooks/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "mcp-spec", Category: "mcp", Pattern: ".gitclaw/mcp/*.yaml", IncludePolicy: "repo-reviewed-source"},
	{Kind: "node-spec", Category: "nodes", Pattern: ".gitclaw/nodes/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "proactive-prompt", Category: "proactive", Pattern: ".gitclaw/proactive/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "skill-source", Category: "skill-source", Pattern: ".gitclaw/skill-sources/*.yaml", IncludePolicy: "repo-reviewed-source"},
	{Kind: "skill-proposal", Category: "skill-proposal", Pattern: ".gitclaw/skill-proposals/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "task-spec", Category: "tasks", Pattern: ".gitclaw/tasks/*.md", IncludePolicy: "repo-reviewed-source"},
	{Kind: "toolset-spec", Category: "toolset", Pattern: ".gitclaw/toolsets/*.yaml", IncludePolicy: "repo-reviewed-source"},
	{Kind: "workspace-spec", Category: "workspace", Pattern: ".gitclaw/workspaces/*.md", IncludePolicy: "repo-reviewed-source"},
}

func BuildProfileManifestReport(cfg Config, repoContext RepoContext) ProfileManifestReport {
	soulValidation := ValidateSoulContext(repoContext)
	skillValidation := ValidateSkillSummaries(repoContext.SkillSummaries)
	toolValidation := ValidateTools(repoContext)
	configFile := inspectConfigSurfaceFile(cfg.Workdir, gitclawConfigPath)
	report := ProfileManifestReport{
		Status:                                   profileStatus(soulValidation, skillValidation, toolValidation),
		ProfileStrategy:                          "repo-local-git-profile",
		ProfileStore:                             ".gitclaw/",
		ProfileScope:                             "repository",
		ManifestStrategy:                         "dry-run-metadata-only",
		ManifestSupported:                        true,
		ProfileExportSupported:                   false,
		ProfileImportSupported:                   false,
		ProfileSwitchingSupported:                false,
		ProfileDistributionInstallSupported:      false,
		ProfileMutationAllowed:                   false,
		CredentialsIncluded:                      false,
		SessionsIncluded:                         false,
		BackupPayloadsIncluded:                   false,
		RawBodiesIncluded:                        false,
		RawConfigBodiesIncluded:                  false,
		RawIssueBodiesIncluded:                   false,
		RawCommentBodiesIncluded:                 false,
		LLME2ERequiredAfterProfileManifestChange: true,
		ProfileDocumentsLoaded:                   len(profileDocuments(repoContext.Documents)),
		RequiredProfileDocuments:                 len(requiredSoulDocumentPaths),
		RequiredProfileDocumentsPresent:          soulValidation.PresentRequiredFiles,
		RequiredProfileDocumentsMissing:          soulValidation.MissingRequiredFiles,
		AvailableSkills:                          len(repoContext.SkillSummaries),
		SelectedSkills:                           len(repoContext.Skills),
		SkillBundles:                             len(repoContext.SkillBundles),
		AvailableTools:                           len(toolReportContracts),
		ActiveToolOutputs:                        len(repoContext.ToolOutputs),
		ConfigFilePresent:                        configFile.Present,
	}

	if configFile.Present {
		report.addEntry(profileManifestEntryFromFile("profile-config", "config", configFile, "config", "repo-local", "metadata-only", false, false, false, true))
	} else {
		report.addEntry(ProfileManifestEntry{Kind: "profile-config", Name: "config", Path: gitclawConfigPath, Category: "config", Source: "repo-local", IncludePolicy: "metadata-only", Present: false})
	}
	seenPaths := map[string]bool{gitclawConfigPath: true}
	for _, doc := range profileDocuments(repoContext.Documents) {
		file := configSurfaceFile{Path: doc.Path, Present: true, Bytes: len(doc.Body), Lines: lineCount(doc.Body), SHA: shortDocumentHash(doc.Body)}
		report.addEntry(profileManifestEntryFromFile("profile-document", profileDocumentCategory(doc.Path), file, profileDocumentCategory(doc.Path), "repo-local", "repo-reviewed-source", true, isRequiredSoulDocument(doc.Path), true, true))
		seenPaths[doc.Path] = true
	}
	selectedSkills := selectedProfileSkillPaths(repoContext)
	for _, skill := range repoContext.SkillSummaries {
		entry := ProfileManifestEntry{
			Kind:          "skill",
			Name:          skill.Name,
			Path:          skill.Path,
			Category:      "skill",
			Source:        "repo-local",
			IncludePolicy: "repo-reviewed-source",
			Portable:      true,
			Present:       true,
			Selected:      selectedSkills[skill.Path],
			Enabled:       skill.Enabled,
			BodyIncluded:  false,
			Bytes:         skill.Bytes,
			Lines:         skill.Lines,
			SHA:           skill.SHA,
		}
		report.addEntry(entry)
		seenPaths[skill.Path] = true
	}
	for _, bundle := range repoContext.SkillBundles {
		entry := ProfileManifestEntry{
			Kind:          "skill-bundle",
			Name:          bundle.Name,
			Path:          bundle.Path,
			Category:      "skill-bundle",
			Source:        "repo-local",
			IncludePolicy: "repo-reviewed-source",
			Portable:      true,
			Present:       true,
			Selected:      bundle.Selected,
			Enabled:       bundle.ParseError == "",
			BodyIncluded:  false,
			Bytes:         bundle.Bytes,
			Lines:         bundle.Lines,
			SHA:           bundle.SHA,
		}
		report.addEntry(entry)
		seenPaths[bundle.Path] = true
	}
	for _, file := range inspectProfileManifestGlobs(cfg.Workdir, seenPaths) {
		report.addEntry(file)
	}
	for _, contract := range toolReportContracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		report.addEntry(ProfileManifestEntry{
			Kind:          "tool-contract",
			Name:          contract.Name,
			Path:          "tool:" + contract.Name,
			Category:      "tool",
			Source:        "runtime-contract",
			IncludePolicy: "contract-only",
			Portable:      false,
			Present:       true,
			Selected:      toolOutputActive(contract.Name, repoContext.ToolOutputs),
			Enabled:       enabled && !disabled && !blocked,
			BodyIncluded:  false,
		})
	}
	report.ExcludedState = []ProfileManifestExclusion{
		{Kind: "credentials", Source: ".env, GitHub secrets, provider tokens", Reason: "credentials stay outside repo-reviewed profile manifests"},
		{Kind: "sessions", Source: "GitHub issues, comments, Hermes sessions", Reason: "conversation history is backed up through the backup branch, not copied into profile metadata"},
		{Kind: "backup-payloads", Source: ".gitclaw/backups/** and gitclaw-backups", Reason: "raw transcript backups are inspected by backup/session commands only"},
		{Kind: "external-profile-home", Source: "~/.hermes and ~/.openclaw", Reason: "GitClaw does not read or package external agent homes"},
		{Kind: "profile-mutation", Source: "install, update, switch, import, apply", Reason: "profile manifest is read-only and does not mutate repository or external state"},
	}
	report.ExcludedStateClasses = len(report.ExcludedState)
	sort.Slice(report.Entries, func(i, j int) bool {
		if report.Entries[i].Kind == report.Entries[j].Kind {
			return report.Entries[i].Path < report.Entries[j].Path
		}
		return report.Entries[i].Kind < report.Entries[j].Kind
	})
	report.ManifestSHA = profileManifestSHA(report.Entries, report.ExcludedState)
	return report
}

func RenderProfileManifestCLIReport(cfg Config, repoContext RepoContext) string {
	return renderProfileManifestReport(Event{}, cfg, repoContext, false)
}

func renderProfileManifestReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildProfileManifestReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Profile Manifest Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_manifest_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- profile_strategy: `%s`\n", report.ProfileStrategy)
	fmt.Fprintf(&b, "- profile_store: `%s`\n", report.ProfileStore)
	fmt.Fprintf(&b, "- profile_scope: `%s`\n", report.ProfileScope)
	fmt.Fprintf(&b, "- manifest_strategy: `%s`\n", report.ManifestStrategy)
	fmt.Fprintf(&b, "- manifest_supported: `%t`\n", report.ManifestSupported)
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", report.ProfileExportSupported)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", report.ProfileImportSupported)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", report.ProfileSwitchingSupported)
	fmt.Fprintf(&b, "- profile_distribution_install_supported: `%t`\n", report.ProfileDistributionInstallSupported)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", report.ProfileMutationAllowed)
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", report.ProfileDocumentsLoaded)
	fmt.Fprintf(&b, "- required_profile_documents: `%d`\n", report.RequiredProfileDocuments)
	fmt.Fprintf(&b, "- required_profile_documents_present: `%d`\n", report.RequiredProfileDocumentsPresent)
	fmt.Fprintf(&b, "- required_profile_documents_missing: `%d`\n", report.RequiredProfileDocumentsMissing)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", report.SkillBundles)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(&b, "- config_file_present: `%t`\n", report.ConfigFilePresent)
	fmt.Fprintf(&b, "- manifest_entries: `%d`\n", report.ManifestEntries)
	fmt.Fprintf(&b, "- repo_tracked_entries: `%d`\n", report.RepoTrackedEntries)
	fmt.Fprintf(&b, "- portable_entries: `%d`\n", report.PortableEntries)
	fmt.Fprintf(&b, "- metadata_only_entries: `%d`\n", report.MetadataOnlyEntries)
	fmt.Fprintf(&b, "- contract_only_entries: `%d`\n", report.ContractOnlyEntries)
	fmt.Fprintf(&b, "- selected_entries: `%d`\n", report.SelectedEntries)
	fmt.Fprintf(&b, "- enabled_entries: `%d`\n", report.EnabledEntries)
	fmt.Fprintf(&b, "- excluded_state_classes: `%d`\n", report.ExcludedStateClasses)
	fmt.Fprintf(&b, "- manifest_sha256_12: `%s`\n", report.ManifestSHA)
	fmt.Fprintf(&b, "- credentials_included: `%t`\n", report.CredentialsIncluded)
	fmt.Fprintf(&b, "- sessions_included: `%t`\n", report.SessionsIncluded)
	fmt.Fprintf(&b, "- backup_payloads_included: `%t`\n", report.BackupPayloadsIncluded)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_config_bodies_included: `%t`\n", report.RawConfigBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_profile_manifest_change: `%t`\n", report.LLME2ERequiredAfterProfileManifestChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report is a body-free portability manifest for GitClaw's repo-local profile. It maps what is reviewed in git and what is deliberately excluded, but it does not package profile files, credentials, sessions, backup payloads, issue bodies, comments, prompts, or secrets.\n\n")

	b.WriteString("### Manifest Entries\n")
	if len(report.Entries) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, entry := range report.Entries {
			writeProfileManifestEntry(&b, entry)
		}
	}

	b.WriteString("\n### Excluded State\n")
	for _, excluded := range report.ExcludedState {
		fmt.Fprintf(&b, "- kind=`%s` source=`%s` reason=`%s`\n", excluded.Kind, excluded.Source, excluded.Reason)
	}
	return strings.TrimSpace(b.String())
}

func isProfileManifestRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 {
		return false
	}
	if fields[0] != "/profile" && fields[0] != "/profiles" {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "manifest", "portability", "portable", "export-plan", "export", "package-plan", "distribution":
		return true
	default:
		return false
	}
}

func (r *ProfileManifestReport) addEntry(entry ProfileManifestEntry) {
	r.Entries = append(r.Entries, entry)
	r.ManifestEntries++
	if strings.HasPrefix(entry.Path, ".gitclaw/") {
		r.RepoTrackedEntries++
	}
	if entry.Portable {
		r.PortableEntries++
	}
	switch entry.IncludePolicy {
	case "metadata-only":
		r.MetadataOnlyEntries++
	case "contract-only":
		r.ContractOnlyEntries++
	}
	if entry.Selected {
		r.SelectedEntries++
	}
	if entry.Enabled {
		r.EnabledEntries++
	}
}

func profileManifestEntryFromFile(kind, name string, file configSurfaceFile, category, source, includePolicy string, portable, required, selected, enabled bool) ProfileManifestEntry {
	return ProfileManifestEntry{
		Kind:          kind,
		Name:          name,
		Path:          file.Path,
		Category:      category,
		Source:        source,
		IncludePolicy: includePolicy,
		Portable:      portable,
		Required:      required,
		Present:       file.Present,
		Selected:      selected,
		Enabled:       enabled,
		BodyIncluded:  false,
		Bytes:         file.Bytes,
		Lines:         file.Lines,
		SHA:           file.SHA,
	}
}

func selectedProfileSkillPaths(repoContext RepoContext) map[string]bool {
	selected := map[string]bool{}
	for _, doc := range repoContext.Skills {
		selected[doc.Path] = true
	}
	return selected
}

func inspectProfileManifestGlobs(root string, seen map[string]bool) []ProfileManifestEntry {
	if root == "" {
		root = "."
	}
	var entries []ProfileManifestEntry
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	for _, spec := range profileManifestGlobSpecs {
		matches, _ := filepath.Glob(filepath.Join(absRoot, filepath.FromSlash(spec.Pattern)))
		sort.Strings(matches)
		for _, match := range matches {
			rel, err := filepath.Rel(absRoot, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if seen[rel] {
				continue
			}
			file := inspectConfigSurfaceFile(root, rel)
			if !file.Present {
				continue
			}
			entries = append(entries, profileManifestEntryFromFile(spec.Kind, profileManifestNameFromPath(rel), file, spec.Category, "repo-local", spec.IncludePolicy, true, false, false, true))
			seen[rel] = true
		}
	}
	return entries
}

func profileManifestNameFromPath(path string) string {
	base := filepath.Base(filepath.FromSlash(path))
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		return "unnamed"
	}
	return base
}

func toolOutputActive(name string, outputs []ToolOutput) bool {
	for _, output := range outputs {
		if output.Name == name {
			return true
		}
	}
	return false
}

func writeProfileManifestEntry(b *strings.Builder, entry ProfileManifestEntry) {
	sha := entry.SHA
	if sha == "" {
		sha = "none"
	}
	fmt.Fprintf(
		b,
		"- kind=`%s` name=`%s` path=`%s` category=`%s` source=`%s` include_policy=`%s` portable=`%t` required=`%t` present=`%t` selected=`%t` enabled=`%t` body_in_report=`%t` bytes=`%d` lines=`%d` sha256_12=`%s`\n",
		entry.Kind,
		entry.Name,
		entry.Path,
		entry.Category,
		entry.Source,
		entry.IncludePolicy,
		entry.Portable,
		entry.Required,
		entry.Present,
		entry.Selected,
		entry.Enabled,
		entry.BodyIncluded,
		entry.Bytes,
		entry.Lines,
		sha,
	)
}

func profileManifestSHA(entries []ProfileManifestEntry, excluded []ProfileManifestExclusion) string {
	var b strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&b, "%s|%s|%s|%s|%t|%t|%s\n", entry.Kind, entry.Name, entry.Path, entry.SHA, entry.Portable, entry.Present, entry.IncludePolicy)
	}
	for _, item := range excluded {
		fmt.Fprintf(&b, "excluded|%s|%s|%s\n", item.Kind, item.Source, item.Reason)
	}
	return shortDocumentHash(b.String())
}
