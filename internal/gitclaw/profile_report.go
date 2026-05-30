package gitclaw

import (
	"fmt"
	"strings"
)

func IsProfileReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/profile" || command == "/profiles"
}

func RenderProfileReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderProfileReport(ev, cfg, repoContext, true)
}

func RenderProfileCLIReport(cfg Config, repoContext RepoContext) string {
	return renderProfileReport(Event{}, cfg, repoContext, false)
}

func renderProfileReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	soulValidation := ValidateSoulContext(repoContext)
	skillValidation := ValidateSkillSummaries(repoContext.SkillSummaries)
	toolValidation := ValidateTools(repoContext)
	profileDocs := profileDocuments(repoContext.Documents)

	var b strings.Builder
	b.WriteString("## GitClaw Profile Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_status: `%s`\n", profileStatus(soulValidation, skillValidation, toolValidation))
	fmt.Fprintf(&b, "- profile_strategy: `%s`\n", "repo-local-git-profile")
	fmt.Fprintf(&b, "- profile_store: `%s`\n", ".gitclaw/")
	fmt.Fprintf(&b, "- profile_scope: `%s`\n", "repository")
	fmt.Fprintf(&b, "- provider: `%s`\n", cfg.ModelProvider)
	fmt.Fprintf(&b, "- model: `%s`\n", cfg.Model)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- trigger_prefix: `%s`\n", cfg.TriggerPrefix)
	fmt.Fprintf(&b, "- trigger_label: `%s`\n", cfg.TriggerLabel)
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", len(profileDocs))
	fmt.Fprintf(&b, "- identity_policy_files: `%d`\n", soulIdentityDocumentCount(repoContext.Documents))
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", soulMemoryDocumentCount(repoContext.Documents))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", len(repoContext.SkillSummaries))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", len(repoContext.SkillBundles))
	fmt.Fprintf(&b, "- available_tools: `%d`\n", len(toolReportContracts))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_profile_payloads_included: `%t`\n", false)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report is GitClaw's repo-local profile envelope: identity, user context, soul, memory, skills, tools, model, and run gates. It does not dump profile files, skill bodies, tool outputs, issue/comment bodies, prompts, or secrets.\n\n")

	b.WriteString("### Profile Documents\n")
	writeProfileDocumentList(&b, profileDocs)

	b.WriteString("\n### Skills\n")
	writeProfileSkillList(&b, repoContext)

	b.WriteString("\n### Tool Surface\n")
	for _, contract := range toolReportContracts {
		enabled, disabled, blocked := toolEnabledInRepoContext(contract.Name, repoContext)
		fmt.Fprintf(&b, "- `%s` mode=`%s` enabled=`%t` disabled_by_config=`%t` blocked_by_allowlist=`%t`\n", contract.Name, contract.Mode, enabled, disabled, blocked)
	}

	b.WriteString("\n### Validation\n")
	fmt.Fprintf(&b, "- component=`soul` status=`%s` errors=`%d` warnings=`%d`\n", soulValidation.Status, soulValidation.Errors, soulValidation.Warnings)
	fmt.Fprintf(&b, "- component=`skills` status=`%s` errors=`%d` warnings=`%d`\n", skillValidation.Status, skillValidation.Errors, skillValidation.Warnings)
	fmt.Fprintf(&b, "- component=`tools` status=`%s` errors=`%d` warnings=`%d`\n", toolValidation.Status, toolValidation.Errors, toolValidation.Warnings)

	return strings.TrimSpace(b.String())
}

func profileStatus(soul SoulValidationReport, skills SkillValidationReport, tools ToolValidationReport) string {
	if soul.Errors > 0 || skills.Errors > 0 || tools.Errors > 0 {
		return "error"
	}
	if soul.Warnings > 0 || skills.Warnings > 0 || tools.Warnings > 0 {
		return "warn"
	}
	return "ok"
}

func profileDocuments(docs []ContextDocument) []ContextDocument {
	filtered := make([]ContextDocument, 0, len(docs))
	for _, doc := range docs {
		if !strings.HasPrefix(doc.Path, ".gitclaw/") {
			continue
		}
		filtered = append(filtered, doc)
	}
	return filtered
}

func writeProfileDocumentList(b *strings.Builder, docs []ContextDocument) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- `%s` category=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, profileDocumentCategory(doc.Path), len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
}

func profileDocumentCategory(path string) string {
	switch {
	case isSoulMemoryNote(path):
		return "memory-note"
	case path == ".gitclaw/SOUL.md":
		return "soul"
	case path == ".gitclaw/IDENTITY.md":
		return "identity"
	case path == ".gitclaw/USER.md":
		return "user"
	case path == ".gitclaw/TOOLS.md":
		return "tools"
	case path == ".gitclaw/MEMORY.md":
		return "memory"
	case path == ".gitclaw/HEARTBEAT.md":
		return "heartbeat"
	case path == standingOrdersPath:
		return "standing-orders"
	case path == hookPolicyPath:
		return "hooks"
	default:
		return "profile"
	}
}

func writeProfileSkillList(b *strings.Builder, repoContext RepoContext) {
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
		fmt.Fprintf(b, "- name=`%s` enabled=`%t` selected=`%t` always=`%t` path=`%s` sha256_12=`%s`\n", skill.Name, skill.Enabled, selected[skill.Name], skill.Always, skill.Path, skill.SHA)
	}
}
