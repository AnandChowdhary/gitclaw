package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func writeSkillSnapshotFixture(t *testing.T, root string) string {
	t.Helper()
	skillBody := `---
name: repo-reader
description: Fixture private description should stay hidden.
---
When a user asks about a repository file, use bounded repository search.
SKILL_SNAPSHOT_BODY_SECRET
`
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", skillBody)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", `name: repo-context
description: Bundle private description should stay hidden.
skills:
  - repo-reader
instruction: |
  SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET should stay hidden.
`)
	writeTestFile(t, root, ".gitclaw/skill-sources/repo-reader.yaml", fmt.Sprintf(`name: repo-reader
skill_path: .gitclaw/SKILLS/repo-reader/SKILL.md
source_kind: github
source_ref: https://example.com/gitclaw/SKILL_SNAPSHOT_SOURCE_REF_SECRET
trust_level: repo-local
install_mode: manual-review
expected_sha256_12: %s
requires_approval: true
remote_fetch_allowed: false
`, shortDocumentHash(strings.TrimSpace(skillBody))))
	return skillBody
}

func TestRenderSkillSnapshotReportFingerprintsSkillSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	skillBody := writeSkillSnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "repo-reader skills snapshot"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ctx.Skills = []ContextDocument{{Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Body: skillBody}}

	body := RenderSkillSnapshotCLIReport(cfg, ctx)
	for _, want := range []string{
		"GitClaw Skill Snapshot Report",
		"scope: `local-cli`",
		"skill_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-skill-snapshot-v1`",
		"snapshot_scope: `repo-local-skills-bundles-sources`",
		"snapshot_sha256_12:",
		"snapshot_entries: `4`",
		"skill_entries: `1`",
		"selected_skill_entries: `1`",
		"bundle_entries: `1`",
		"source_pin_entries: `1`",
		"prompt_visible_entries: `2`",
		"available_skills: `1`",
		"enabled_skills: `1`",
		"disabled_skills: `0`",
		"allowlist_blocked_skills: `0`",
		"always_on_skills: `0`",
		"skills_with_frontmatter: `1`",
		"skills_with_description: `1`",
		"skills_with_requirements: `0`",
		"skills_missing_requirements: `0`",
		"skill_bundles: `1`",
		"selected_bundles: `0`",
		"source_pins_scanned: `1`",
		"hash_pinned_skill_sources: `1`",
		"hash_matched_skill_sources: `1`",
		"hash_mismatched_skill_sources: `0`",
		"missing_skill_source_matches: `0`",
		"remote_source_refs: `1`",
		"remote_fetch_allowed_specs: `0`",
		"registry_contact_allowed: `false`",
		"remote_fetch_allowed: `false`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_skill_descriptions_included: `false`",
		"raw_source_bodies_included: `false`",
		"raw_source_refs_included: `false`",
		"raw_bundle_instructions_included: `false`",
		"llm_e2e_required_after_skill_snapshot_change: `true`",
		"skill_validation_status: `ok`",
		"skill_risk_status: `ok`",
		"### Snapshot Entries",
		"kind=`skill` name=`repo-reader` source=`repo-local-skill` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"prompt_visible=`true`",
		"selected_for_this_turn=`true`",
		"description_present=`true`",
		"description_sha256_12=",
		"kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md` source=`prompt-context`",
		"kind=`bundle` name=`repo-context` source=`repo-local-skill-bundle`",
		"bundle_skill_refs=`repo-reader`",
		"resolved_bundle_skills=`repo-reader`",
		"instruction_present=`true`",
		"instruction_sha256_12=",
		"kind=`source-pin` name=`repo-reader` source=`repo-local-skill-source`",
		"source_kind=`github`",
		"source_ref_present=`true`",
		"source_ref_sha256_12=",
		"trust_level=`repo-local`",
		"install_mode=`manual-review`",
		"hash_pinned=`true`",
		"hash_matched=`true`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"### Snapshot Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"source_gate=`pass`",
		"progressive_disclosure_gate=`enabled`",
		"registry_gate=`disabled`",
		"remote_fetch_gate=`disabled`",
		"installer_gate=`disabled`",
		"dependency_install_gate=`disabled`",
		"mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("skill snapshot report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{
		"SKILL_SNAPSHOT_BODY_SECRET",
		"Fixture private description should stay hidden",
		"When a user asks about a repository file",
		"SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET",
		"SKILL_SNAPSHOT_SOURCE_REF_SECRET",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("skill snapshot report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSkillsReportRoutesSnapshotInsteadOfRefreshPlan(t *testing.T) {
	root := t.TempDir()
	writeSkillSnapshotFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ctx, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: "repo-reader skills snapshot"}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 164,
			Title:  "@gitclaw /skills snapshot",
			Body:   "Hidden skill snapshot issue token: SKILL_SNAPSHOT_ROUTE_BODY_SECRET. repo-reader",
		},
	}
	body := RenderSkillsReport(ev, cfg, ctx)
	for _, want := range []string{
		"GitClaw Skill Snapshot Report",
		"repository: `owner/repo`",
		"issue: `#164`",
		"skill_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-skill-snapshot-v1`",
		"snapshot_sha256_12:",
		"issue_title_sha256_12:",
		"llm_e2e_required_after_skill_snapshot_change: `true`",
		"raw_skill_bodies_included: `false`",
		"kind=`skill` name=`repo-reader`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("skills snapshot route missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "GitClaw Skill Refresh Plan Report") {
		t.Fatalf("skills snapshot routed to refresh plan:\n%s", body)
	}
	for _, leaked := range []string{"SKILL_SNAPSHOT_ROUTE_BODY_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET", "SKILL_SNAPSHOT_SOURCE_REF_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("skills snapshot route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestSkillsSnapshotCommandReportsCompositeFingerprintWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeSkillSnapshotFixture(t, root)
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "snapshot"}); err != nil {
			t.Fatalf("skills snapshot returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Skill Snapshot Report",
		"scope: `local-cli`",
		"skill_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-skill-snapshot-v1`",
		"snapshot_entries: `3`",
		"skill_entries: `1`",
		"selected_skill_entries: `0`",
		"bundle_entries: `1`",
		"source_pin_entries: `1`",
		"raw_skill_bodies_included: `false`",
		"raw_bundle_instructions_included: `false`",
		"llm_e2e_required_after_skill_snapshot_change: `true`",
		"### Snapshot Entries",
		"kind=`skill` name=`repo-reader`",
		"kind=`bundle` name=`repo-context`",
		"kind=`source-pin` name=`repo-reader`",
		"### Snapshot Gates",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills snapshot output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SKILL_SNAPSHOT_BODY_SECRET", "Fixture private description should stay hidden", "SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET", "SKILL_SNAPSHOT_SOURCE_REF_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("skills snapshot output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleSkillsSnapshotCommandPostsCompositeFingerprintWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeSkillSnapshotFixture(t, root)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 165,
			"title": "@gitclaw /skills snapshot",
			"body": "@gitclaw /skills snapshot\nrepo-reader\nHidden skill snapshot body token: SKILL_SNAPSHOT_HANDLER_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{165: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic skills snapshot command", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Skill Snapshot Report",
		"Generated without a model call",
		"model=\"gitclaw/skills\"",
		"repository: `owner/repo`",
		"issue: `#165`",
		"skill_snapshot_status: `ok`",
		"snapshot_version: `gitclaw-skill-snapshot-v1`",
		"snapshot_scope: `repo-local-skills-bundles-sources`",
		"snapshot_sha256_12:",
		"skill_entries: `1`",
		"selected_skill_entries: `1`",
		"bundle_entries: `1`",
		"source_pin_entries: `1`",
		"raw_skill_bodies_included: `false`",
		"raw_source_refs_included: `false`",
		"llm_e2e_required_after_skill_snapshot_change: `true`",
		"issue_title_sha256_12:",
		"### Snapshot Entries",
		"kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"kind=`source-pin` name=`repo-reader`",
		"### Snapshot Gates",
		"validation_gate=`pass`",
		"risk_gate=`pass`",
		"source_gate=`pass`",
		"snapshot_hash_gate=`composite-sha256_12`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("skills snapshot handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SKILL_SNAPSHOT_HANDLER_BODY_SECRET", "SKILL_SNAPSHOT_BODY_SECRET", "SKILL_SNAPSHOT_BUNDLE_INSTRUCTION_SECRET", "SKILL_SNAPSHOT_SOURCE_REF_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("skills snapshot handler report leaked %q:\n%s", leaked, body)
		}
	}
	if !hasLabel(github.IssueLabels[165], "gitclaw:done") || hasLabel(github.IssueLabels[165], "gitclaw:running") || hasLabel(github.IssueLabels[165], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[165])
	}
}
