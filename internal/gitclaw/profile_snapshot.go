package gitclaw

import (
	"fmt"
	"strings"
)

const profileSnapshotVersion = "gitclaw-profile-snapshot-v1"

type ProfileSnapshotReport struct {
	Status                            string
	SnapshotVersion                   string
	SnapshotScope                     string
	SnapshotSHA                       string
	SnapshotEntries                   int
	ProfileDocumentsLoaded            int
	AvailableSkills                   int
	SelectedSkills                    int
	SkillBundles                      int
	AvailableTools                    int
	ActiveToolOutputs                 int
	ManifestEntries                   int
	ProfileExportSupported            bool
	ProfileImportSupported            bool
	ProfileSwitchingSupported         bool
	ProfileMutationAllowed            bool
	RawProfileBodiesIncluded          bool
	RawSkillBodiesIncluded            bool
	RawToolOutputsIncluded            bool
	RawMemoryBodiesIncluded           bool
	RawIssueBodiesIncluded            bool
	RawCommentBodiesIncluded          bool
	CredentialsIncluded               bool
	SessionsIncluded                  bool
	BackupPayloadsIncluded            bool
	LLME2ERequiredAfterSnapshotChange bool
	Manifest                          ProfileManifestReport
	Soul                              SoulSnapshotReport
	Memory                            MemorySnapshotReport
	Skills                            SkillSnapshotReport
	Tools                             ToolSnapshotReport
	Cards                             []ProfileSnapshotCard
}

type ProfileSnapshotCard struct {
	Position             int
	Kind                 string
	Name                 string
	Status               string
	Version              string
	Scope                string
	SHA                  string
	Entries              int
	PromptVisibleEntries int
	PortableEntries      int
	SelectedEntries      int
	EnabledEntries       int
	Source               string
	RawBodiesIncluded    bool
	MutationAllowed      bool
}

func BuildProfileSnapshotReport(cfg Config, repoContext RepoContext) ProfileSnapshotReport {
	manifest := BuildProfileManifestReport(cfg, repoContext)
	soul := BuildSoulSnapshotReport(repoContext)
	memory := BuildMemorySnapshotReport(cfg, repoContext)
	skills := BuildSkillSnapshotReport(cfg, repoContext)
	tools := BuildToolSnapshotReport(cfg, repoContext)
	report := ProfileSnapshotReport{
		Status:                            profileSnapshotStatus(manifest.Status, soul.Status, memory.Status, skills.Status, tools.Status),
		SnapshotVersion:                   profileSnapshotVersion,
		SnapshotScope:                     "repo-local-profile-soul-memory-skills-tools",
		ProfileDocumentsLoaded:            manifest.ProfileDocumentsLoaded,
		AvailableSkills:                   manifest.AvailableSkills,
		SelectedSkills:                    manifest.SelectedSkills,
		SkillBundles:                      manifest.SkillBundles,
		AvailableTools:                    manifest.AvailableTools,
		ActiveToolOutputs:                 manifest.ActiveToolOutputs,
		ManifestEntries:                   manifest.ManifestEntries,
		ProfileExportSupported:            false,
		ProfileImportSupported:            false,
		ProfileSwitchingSupported:         false,
		ProfileMutationAllowed:            false,
		RawProfileBodiesIncluded:          false,
		RawSkillBodiesIncluded:            false,
		RawToolOutputsIncluded:            false,
		RawMemoryBodiesIncluded:           false,
		RawIssueBodiesIncluded:            false,
		RawCommentBodiesIncluded:          false,
		CredentialsIncluded:               false,
		SessionsIncluded:                  false,
		BackupPayloadsIncluded:            false,
		LLME2ERequiredAfterSnapshotChange: true,
		Manifest:                          manifest,
		Soul:                              soul,
		Memory:                            memory,
		Skills:                            skills,
		Tools:                             tools,
	}
	report.addCard(ProfileSnapshotCard{
		Kind:              "profile-manifest",
		Name:              "profile-manifest",
		Status:            manifest.Status,
		Version:           "profile-manifest",
		Scope:             manifest.ProfileScope,
		SHA:               manifest.ManifestSHA,
		Entries:           manifest.ManifestEntries,
		PortableEntries:   manifest.PortableEntries,
		SelectedEntries:   manifest.SelectedEntries,
		EnabledEntries:    manifest.EnabledEntries,
		Source:            "repo-local-profile",
		RawBodiesIncluded: manifest.RawBodiesIncluded,
		MutationAllowed:   manifest.ProfileMutationAllowed,
	})
	report.addCard(ProfileSnapshotCard{
		Kind:                 "soul-snapshot",
		Name:                 "soul",
		Status:               soul.Status,
		Version:              soul.SnapshotVersion,
		Scope:                soul.SnapshotScope,
		SHA:                  soul.SnapshotSHA,
		Entries:              soul.SnapshotEntries,
		PromptVisibleEntries: soul.PromptVisibleEntries,
		Source:               "repo-local-soul",
		RawBodiesIncluded:    soul.RawBodiesIncluded,
		MutationAllowed:      soul.RepositoryMutationAllowed,
	})
	report.addCard(ProfileSnapshotCard{
		Kind:                 "memory-snapshot",
		Name:                 "memory",
		Status:               memory.Status,
		Version:              memory.SnapshotVersion,
		Scope:                memory.SnapshotScope,
		SHA:                  memory.SnapshotSHA,
		Entries:              memory.SnapshotEntries,
		PromptVisibleEntries: memory.PromptVisibleEntries,
		Source:               "repo-local-memory",
		RawBodiesIncluded:    memory.RawMemoryBodiesIncluded,
		MutationAllowed:      memory.MemoryWritesAllowed,
	})
	report.addCard(ProfileSnapshotCard{
		Kind:                 "skill-snapshot",
		Name:                 "skills",
		Status:               skills.Status,
		Version:              skills.SnapshotVersion,
		Scope:                skills.SnapshotScope,
		SHA:                  skills.SnapshotSHA,
		Entries:              skills.SnapshotEntries,
		PromptVisibleEntries: skills.PromptVisibleEntries,
		Source:               "repo-local-skills",
		RawBodiesIncluded:    skills.RawSkillBodiesIncluded,
		MutationAllowed:      skills.RepositoryMutationAllowed,
	})
	report.addCard(ProfileSnapshotCard{
		Kind:                 "tool-snapshot",
		Name:                 "tools",
		Status:               tools.Status,
		Version:              tools.SnapshotVersion,
		Scope:                tools.SnapshotScope,
		SHA:                  tools.SnapshotSHA,
		Entries:              tools.SnapshotEntries,
		PromptVisibleEntries: tools.PromptVisibleEntries,
		Source:               "deterministic-tool-surface",
		RawBodiesIncluded:    tools.RawToolOutputsIncluded || tools.RawToolSchemasIncluded || tools.RawToolsetBodiesIncluded,
		MutationAllowed:      tools.RepositoryMutationAllowed,
	})
	for i := range report.Cards {
		report.Cards[i].Position = i + 1
	}
	report.SnapshotEntries = len(report.Cards)
	report.SnapshotSHA = profileSnapshotManifestHash(report.Cards)
	return report
}

func RenderProfileSnapshotCLIReport(cfg Config, repoContext RepoContext) string {
	return renderProfileSnapshotReport(Event{}, cfg, repoContext, false)
}

func renderProfileSnapshotReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildProfileSnapshotReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Profile Snapshot Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_snapshot_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- snapshot_version: `%s`\n", report.SnapshotVersion)
	fmt.Fprintf(&b, "- snapshot_scope: `%s`\n", report.SnapshotScope)
	fmt.Fprintf(&b, "- snapshot_sha256_12: `%s`\n", report.SnapshotSHA)
	fmt.Fprintf(&b, "- snapshot_entries: `%d`\n", report.SnapshotEntries)
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", report.ProfileDocumentsLoaded)
	fmt.Fprintf(&b, "- available_skills: `%d`\n", report.AvailableSkills)
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", report.SelectedSkills)
	fmt.Fprintf(&b, "- skill_bundles: `%d`\n", report.SkillBundles)
	fmt.Fprintf(&b, "- available_tools: `%d`\n", report.AvailableTools)
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", report.ActiveToolOutputs)
	fmt.Fprintf(&b, "- manifest_entries: `%d`\n", report.ManifestEntries)
	fmt.Fprintf(&b, "- profile_manifest_sha256_12: `%s`\n", report.Manifest.ManifestSHA)
	fmt.Fprintf(&b, "- soul_snapshot_sha256_12: `%s`\n", report.Soul.SnapshotSHA)
	fmt.Fprintf(&b, "- memory_snapshot_sha256_12: `%s`\n", report.Memory.SnapshotSHA)
	fmt.Fprintf(&b, "- skill_snapshot_sha256_12: `%s`\n", report.Skills.SnapshotSHA)
	fmt.Fprintf(&b, "- tool_snapshot_sha256_12: `%s`\n", report.Tools.SnapshotSHA)
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", report.ProfileExportSupported)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", report.ProfileImportSupported)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", report.ProfileSwitchingSupported)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", report.ProfileMutationAllowed)
	fmt.Fprintf(&b, "- raw_profile_bodies_included: `%t`\n", report.RawProfileBodiesIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_memory_bodies_included: `%t`\n", report.RawMemoryBodiesIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- credentials_included: `%t`\n", report.CredentialsIncluded)
	fmt.Fprintf(&b, "- sessions_included: `%t`\n", report.SessionsIncluded)
	fmt.Fprintf(&b, "- backup_payloads_included: `%t`\n", report.BackupPayloadsIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_profile_snapshot_change: `%t`\n", report.LLME2ERequiredAfterSnapshotChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report is a composite body-free fingerprint for GitClaw's repo-local profile envelope. It ties the profile manifest, soul snapshot, memory snapshot, skill snapshot, and tool snapshot together with component hashes and one profile snapshot hash; raw profile files, skill bodies, memory bodies, tool outputs, issue/comment bodies, prompts, sessions, backup payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Snapshot Components\n")
	writeProfileSnapshotCards(&b, report.Cards)

	b.WriteString("\n### Snapshot Gates\n")
	fmt.Fprintf(&b, "- manifest_gate=`%s`\n", profileSnapshotGate(report.Manifest.Status))
	fmt.Fprintf(&b, "- soul_gate=`%s`\n", profileSnapshotGate(report.Soul.Status))
	fmt.Fprintf(&b, "- memory_gate=`%s`\n", profileSnapshotGate(report.Memory.Status))
	fmt.Fprintf(&b, "- skills_gate=`%s`\n", profileSnapshotGate(report.Skills.Status))
	fmt.Fprintf(&b, "- tools_gate=`%s`\n", profileSnapshotGate(report.Tools.Status))
	b.WriteString("- profile_export_gate=`disabled`\n")
	b.WriteString("- profile_import_gate=`disabled`\n")
	b.WriteString("- profile_switching_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- session_payload_gate=`excluded`\n")
	b.WriteString("- backup_payload_gate=`excluded`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- snapshot_hash_gate=`composite-sha256_12`\n")
	return strings.TrimSpace(b.String())
}

func writeProfileSnapshotCards(b *strings.Builder, cards []ProfileSnapshotCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		fmt.Fprintf(
			b,
			"- position=`%d` kind=`%s` name=`%s` status=`%s` version=`%s` scope=`%s` sha256_12=`%s` entries=`%d` prompt_visible_entries=`%d` portable_entries=`%d` selected_entries=`%d` enabled_entries=`%d` source=`%s` raw_bodies_included=`%t` mutation_allowed=`%t`\n",
			card.Position,
			card.Kind,
			card.Name,
			card.Status,
			card.Version,
			card.Scope,
			noneIfEmpty(card.SHA),
			card.Entries,
			card.PromptVisibleEntries,
			card.PortableEntries,
			card.SelectedEntries,
			card.EnabledEntries,
			card.Source,
			card.RawBodiesIncluded,
			card.MutationAllowed,
		)
	}
}

func (r *ProfileSnapshotReport) addCard(card ProfileSnapshotCard) {
	if card.SHA == "" {
		card.SHA = "none"
	}
	r.Cards = append(r.Cards, card)
}

func profileSnapshotManifestHash(cards []ProfileSnapshotCard) string {
	var b strings.Builder
	b.WriteString(profileSnapshotVersion)
	b.WriteByte('\n')
	for _, card := range cards {
		fmt.Fprintf(&b, "%03d|%s|%s|%s|%s|%s|%s|%d|%d|%d|%d|%d|%s|%t|%t\n",
			card.Position,
			card.Kind,
			card.Name,
			card.Status,
			card.Version,
			card.Scope,
			card.SHA,
			card.Entries,
			card.PromptVisibleEntries,
			card.PortableEntries,
			card.SelectedEntries,
			card.EnabledEntries,
			card.Source,
			card.RawBodiesIncluded,
			card.MutationAllowed,
		)
	}
	return shortDocumentHash(b.String())
}

func profileSnapshotStatus(statuses ...string) string {
	best := "ok"
	bestRank := profileSnapshotStatusRank(best)
	for _, status := range statuses {
		rank := profileSnapshotStatusRank(status)
		if rank > bestRank {
			best = status
			bestRank = rank
		}
	}
	return best
}

func profileSnapshotStatusRank(status string) int {
	switch status {
	case "error":
		return 4
	case "high":
		return 3
	case "warn":
		return 2
	case "ok":
		return 1
	default:
		return 0
	}
}

func profileSnapshotGate(status string) string {
	switch status {
	case "ok":
		return "pass"
	case "warn":
		return "warn"
	case "high", "error":
		return "fail"
	default:
		return status
	}
}

func isProfileSnapshotRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || (fields[0] != "/profile" && fields[0] != "/profiles") {
		return false
	}
	switch strings.ToLower(fields[1]) {
	case "snapshot", "snapshots", "fingerprint", "fingerprints", "lock", "lockfile":
		return true
	default:
		return false
	}
}
