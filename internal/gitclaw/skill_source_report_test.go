package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

const skillSourceSkillBody = `---
name: repo-reader
description: Read repository context.
---

# Repo Reader

SKILL_SOURCE_SKILL_BODY_SECRET
`

func writeSkillSourceFixture(t *testing.T, dir string) string {
	t.Helper()
	hash := shortDocumentHash(strings.TrimSpace(skillSourceSkillBody))
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", skillSourceSkillBody)
	writeTestFile(t, dir, ".gitclaw/skill-sources/repo-reader.yaml", fmt.Sprintf(`name: repo-reader
skill_path: .gitclaw/SKILLS/repo-reader/SKILL.md
source_kind: repo-local
source_ref: .gitclaw/SKILLS/repo-reader/SKILL.md
trust_level: repo-local
install_mode: manual-review
expected_sha256_12: %s
requires_approval: true
remote_fetch_allowed: false
`, hash))
	return hash
}

func writeSkillSourceProvenanceGitFixture(t *testing.T, dir string) string {
	t.Helper()
	hash := writeSkillSourceFixture(t, dir)
	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "skill-source-provenance@example.invalid")
	runTestGit(t, dir, "config", "user.name", "Skill Source Provenance Secret Author")
	runTestGit(t, dir, "add", ".gitclaw")
	runTestGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-m", "Add skill source provenance fixture SKILL_SOURCE_COMMIT_SUBJECT_SECRET")
	return hash
}

func TestRenderSkillSourcesReportAuditsPinsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	hash := writeSkillSourceFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderSkillSourcesCLIReport(cfg, repoContext)
	for _, want := range []string{
		"GitClaw Skill Sources Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"skill_source_status: `ok`",
		"skill_source_specs_dir: `.gitclaw/skill-sources`",
		"skill_source_specs: `1`",
		"parsed_skill_source_specs: `1`",
		"matched_skill_sources: `1`",
		"missing_skill_source_matches: `0`",
		"hash_pinned_skill_sources: `1`",
		"hash_matched_skill_sources: `1`",
		"hash_mismatched_skill_sources: `0`",
		"repo_local_source_refs: `1`",
		"remote_source_refs: `0`",
		"sources_requiring_approval: `1`",
		"remote_fetch_allowed_specs: `0`",
		"sources_with_risk_findings: `0`",
		"skill_source_risk_findings: `0`",
		"registry_contact_allowed: `false`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_source_bodies_included: `false`",
		"raw_source_refs_included: `false`",
		"raw_skill_bodies_included: `false`",
		"llm_e2e_required_after_skill_source_change: `true`",
		"source_name=`repo-reader`",
		"path=`.gitclaw/skill-sources/repo-reader.yaml`",
		"skill_path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"skill_matched=`true`",
		"source_kind=`repo-local`",
		"source_ref_present=`true`",
		"trust_level=`repo-local`",
		"install_mode=`manual-review`",
		"requires_approval=`true`",
		"remote_fetch_allowed=`false`",
		"hash_pinned=`true`",
		"expected_sha256_12=`" + hash + "`",
		"current_skill_sha256_12=`" + hash + "`",
		"hash_matched=`true`",
		"risk_findings=`0`",
		"risk_codes=`none`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill sources report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"SKILL_SOURCE_SKILL_BODY_SECRET", "repository:", "issue:", ".gitclaw/SKILLS/repo-reader/SKILL.md\n"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("skill sources report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestSkillsSourcesProvenanceCommandReportsGitHistoryWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	hash := writeSkillSourceProvenanceGitFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "sources", "provenance"}); err != nil {
			t.Fatalf("skills sources provenance returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Skill Source Provenance Report",
		"scope: `local-cli`",
		"Generated without a model call",
		"skill_source_provenance_status: `ok`",
		"provenance_scope: `repo-local-skill-source-git-history`",
		"skill_source_status: `ok`",
		"skill_source_specs_dir: `.gitclaw/skill-sources`",
		"skill_source_specs: `1`",
		"parsed_skill_source_specs: `1`",
		"matched_skill_sources: `1`",
		"missing_skill_source_matches: `0`",
		"hash_pinned_skill_sources: `1`",
		"hash_matched_skill_sources: `1`",
		"hash_mismatched_skill_sources: `0`",
		"repo_local_source_refs: `1`",
		"remote_source_refs: `0`",
		"sources_requiring_approval: `1`",
		"remote_fetch_allowed_specs: `0`",
		"sources_with_risk_findings: `0`",
		"skill_source_risk_findings: `0`",
		"provenance_surfaces: `1`",
		"repo_local_surfaces: `1`",
		"unknown_source_surfaces: `0`",
		"git_tracked_surfaces: `1`",
		"untracked_surfaces: `0`",
		"working_tree_dirty_surfaces: `0`",
		"surfaces_with_commits: `1`",
		"surfaces_without_commits: `0`",
		"git_available: `true`",
		"git_history_available: `true`",
		"registry_contact_allowed: `false`",
		"remote_fetch_allowed: `false`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"repository_mutation_allowed: `false`",
		"raw_source_bodies_included: `false`",
		"raw_source_refs_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"raw_git_subjects_included: `false`",
		"author_identities_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_skill_source_provenance_change: `true`",
		"### Skill Source Provenance Cards",
		"source_name=`repo-reader` path=`.gitclaw/skill-sources/repo-reader.yaml` source=`repo-local`",
		"skill_path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"skill_matched=`true`",
		"source_kind=`repo-local`",
		"source_ref_present=`true`",
		"trust_level=`repo-local`",
		"install_mode=`manual-review`",
		"requires_approval=`true`",
		"remote_fetch_allowed=`false`",
		"hash_pinned=`true`",
		"expected_sha256_12=`" + hash + "`",
		"current_skill_sha256_12=`" + hash + "`",
		"hash_matched=`true`",
		"hash_mismatched=`false`",
		"risk_findings=`0`",
		"risk_codes=`none`",
		"git_tracked=`true`",
		"working_tree_dirty=`false`",
		"commit_available=`true`",
		"last_commit_sha256_12=",
		"last_commit_short=",
		"last_commit_date=",
		"subject_sha256_12=",
		"### Provenance Gates",
		"risk_gate=`pass`",
		"git_history_gate=`pass`",
		"source_pin_gate=`repo-reviewed`",
		"registry_gate=`disabled`",
		"remote_fetch_gate=`disabled`",
		"installer_gate=`disabled`",
		"mutation_gate=`disabled`",
		"raw_body_gate=`hash_only`",
		"### Findings",
		"- none",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills sources provenance output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{
		"SKILL_SOURCE_SKILL_BODY_SECRET",
		"SKILL_SOURCE_COMMIT_SUBJECT_SECRET",
		"skill-source-provenance@example.invalid",
		"Skill Source Provenance Secret Author",
		"issue: `#0`",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("skills sources provenance leaked %q:\n%s", leaked, output)
		}
	}
}

func TestRenderSkillSourcesRiskReportFlagsUnsafePinsWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".gitclaw/SKILLS/repo-reader/SKILL.md", skillSourceSkillBody)
	writeTestFile(t, dir, ".gitclaw/skill-sources/risky.yaml", `name: repo-reader
skill_path: .gitclaw/SKILLS/repo-reader/SKILL.md
source_kind: http://example.invalid/risky
source_ref: https://example.invalid/SKILL_SOURCE_REMOTE_SECRET
trust_level: community
install_mode: auto-install
expected_sha256_12: deadbeef0000
requires_approval: false
remote_fetch_allowed: true
notes: npm install risky-skill
`)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}

	report := RenderSkillSourcesRiskCLIReport(cfg, repoContext)
	for _, want := range []string{
		"GitClaw Skill Sources Risk Report",
		"skill_source_status: `high`",
		"skill_source_specs: `1`",
		"matched_skill_sources: `1`",
		"hash_mismatched_skill_sources: `1`",
		"remote_source_refs: `1`",
		"remote_fetch_allowed_specs: `1`",
		"sources_with_risk_findings: `1`",
		"high_risk_findings:",
		"warning_risk_findings:",
		"source_name=`repo-reader`",
		"source_kind=`http-url`",
		"install_mode=`auto-install`",
		"remote_fetch_allowed=`true`",
		"hash_matched=`false`",
		"risk_max_severity=`high`",
		"code=`skill_source_yaml_parse_error`",
		"code=`skill_source_hash_mismatch`",
		"code=`skill_source_remote_fetch_allowed`",
		"code=`skill_source_install_mode_not_review_only`",
		"code=`skill_source_approval_gate_missing`",
		"code=`skill_source_kind_untrusted`",
		"code=`automatic_plugin_install`",
		"line_sha256_12=",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill sources risk report missing %q:\n%s", want, report)
		}
	}
	for _, notWant := range []string{"SKILL_SOURCE_REMOTE_SECRET", "SKILL_SOURCE_SKILL_BODY_SECRET", "npm install risky-skill", "https://example.invalid"} {
		if strings.Contains(report, notWant) {
			t.Fatalf("skill sources risk report leaked %q:\n%s", notWant, report)
		}
	}
}

func TestSkillsSourcesCommandsReportPins(t *testing.T) {
	dir := t.TempDir()
	writeSkillSourceFixture(t, dir)
	t.Setenv("GITCLAW_WORKDIR", dir)

	listOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "sources"}); err != nil {
			t.Fatalf("skills sources returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skill Sources Report", "scope: `local-cli`", "skill_source_status: `ok`", "skill_source_specs: `1`"} {
		if !strings.Contains(listOutput, want) {
			t.Fatalf("skills sources output missing %q:\n%s", want, listOutput)
		}
	}

	provenanceOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "sources", "provenance"}); err != nil {
			t.Fatalf("skills sources provenance returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skill Source Provenance Report", "skill_source_provenance_status:", "skill_source_specs: `1`", "raw_source_bodies_included: `false`"} {
		if !strings.Contains(provenanceOutput, want) {
			t.Fatalf("skills sources provenance output missing %q:\n%s", want, provenanceOutput)
		}
	}

	infoOutput := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"skills", "sources", "info", "repo-reader"}); err != nil {
			t.Fatalf("skills sources info returned error: %v", err)
		}
	})
	for _, want := range []string{"GitClaw Skill Source Info Report", "skill_source_info_status: `ok`", "matched_skill_sources: `1`", "source_name=`repo-reader`"} {
		if !strings.Contains(infoOutput, want) {
			t.Fatalf("skills sources info output missing %q:\n%s", want, infoOutput)
		}
	}
	if strings.Contains(listOutput, "SKILL_SOURCE_SKILL_BODY_SECRET") || strings.Contains(provenanceOutput, "SKILL_SOURCE_SKILL_BODY_SECRET") || strings.Contains(infoOutput, "SKILL_SOURCE_SKILL_BODY_SECRET") {
		t.Fatalf("skill source CLI leaked skill body:\nlist:\n%s\nprovenance:\n%s\ninfo:\n%s", listOutput, provenanceOutput, infoOutput)
	}
}

func TestRenderSkillsReportRoutesSourcesProvenanceWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeSkillSourceProvenanceGitFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 125,
			"title": "@gitclaw /skills sources provenance",
			"body": "Hidden skill source provenance route token: SKILL_SOURCE_PROVENANCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderSkillsReport(ev, cfg, repoContext)
	for _, want := range []string{"GitClaw Skill Source Provenance Report", "repository: `owner/repo`", "issue: `#125`", "skill_source_provenance_status: `ok`", "provenance_scope: `repo-local-skill-source-git-history`", "git_history_gate=`pass`", "issue_title_sha256_12:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill sources provenance routed report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"SKILL_SOURCE_PROVENANCE_ROUTE_BODY_SECRET", "SKILL_SOURCE_SKILL_BODY_SECRET", "SKILL_SOURCE_COMMIT_SUBJECT_SECRET"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("skill sources provenance routed report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestRenderSkillsReportRoutesSourcesRiskWithoutBodies(t *testing.T) {
	dir := t.TempDir()
	writeSkillSourceFixture(t, dir)
	cfg := DefaultConfig()
	cfg.Workdir = dir
	repoContext, err := LoadRepoContextWithConfig(dir, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 123,
			"title": "@gitclaw /skills sources risk",
			"body": "Hidden skill source route token: SKILL_SOURCE_ROUTE_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}

	report := RenderSkillsReport(ev, cfg, repoContext)
	for _, want := range []string{"GitClaw Skill Sources Risk Report", "repository: `owner/repo`", "issue: `#123`", "skill_source_status: `ok`", "issue_title_sha256_12:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("skill sources routed report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "SKILL_SOURCE_ROUTE_BODY_SECRET") || strings.Contains(report, "SKILL_SOURCE_SKILL_BODY_SECRET") {
		t.Fatalf("skill sources routed report leaked body text:\n%s", report)
	}
}

func TestHandleSkillsSourcesProvenanceCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeSkillSourceProvenanceGitFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 126,
			"title": "@gitclaw /skills sources history",
			"body": "Hidden skill source provenance handler token: SKILL_SOURCE_PROVENANCE_HANDLER_BODY_SECRET.",
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
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{126: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic skill sources provenance report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Skill Source Provenance Report", "Generated without a model call", "model=\"gitclaw/skills\"", "skill_source_provenance_status: `ok`", "skill_source_specs: `1`", "provenance_surfaces: `1`", "git_tracked_surfaces: `1`", "surfaces_with_commits: `1`", "raw_source_bodies_included: `false`", "raw_skill_bodies_included: `false`", "raw_git_subjects_included: `false`", "llm_e2e_required_after_skill_source_provenance_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("skill sources provenance handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"SKILL_SOURCE_PROVENANCE_HANDLER_BODY_SECRET", "SKILL_SOURCE_SKILL_BODY_SECRET", "SKILL_SOURCE_COMMIT_SUBJECT_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("skill sources provenance handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[126], "gitclaw:done") || hasLabel(github.IssueLabels[126], "gitclaw:running") || hasLabel(github.IssueLabels[126], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[126])
	}
}

func TestHandleSkillsSourcesRiskCommandPostsReportWithoutLLM(t *testing.T) {
	dir := t.TempDir()
	writeSkillSourceFixture(t, dir)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 124,
			"title": "@gitclaw /skills sources risk",
			"body": "Hidden skill source handler token: SKILL_SOURCE_HANDLER_BODY_SECRET.",
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
	cfg.Workdir = dir
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{124: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic skill sources report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{"GitClaw Skill Sources Risk Report", "Generated without a model call", "model=\"gitclaw/skills\"", "skill_source_status: `ok`", "skill_source_specs: `1`", "raw_source_bodies_included: `false`", "raw_skill_bodies_included: `false`", "llm_e2e_required_after_skill_source_change: `true`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("skill sources handler report missing %q:\n%s", want, body)
		}
	}
	for _, notWant := range []string{"SKILL_SOURCE_HANDLER_BODY_SECRET", "SKILL_SOURCE_SKILL_BODY_SECRET"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("skill sources handler report leaked %q:\n%s", notWant, body)
		}
	}
	if !hasLabel(github.IssueLabels[124], "gitclaw:done") || hasLabel(github.IssueLabels[124], "gitclaw:running") || hasLabel(github.IssueLabels[124], "gitclaw:error") {
		t.Fatalf("unexpected final labels: %#v", github.IssueLabels[124])
	}
}
