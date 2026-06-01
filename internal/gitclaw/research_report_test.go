package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestResearchCatalogReportMapsSourcesAndCoverageWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "GitClaw README body RESEARCH_README_SECRET\n")
	writeTestFile(t, root, "docs/spec-github-native-gitclaw.md", "GitClaw spec body RESEARCH_SPEC_SECRET\n")
	writeTestFile(t, root, "docs/research-openclaw-hermes-landscape.md", "OpenClaw notes follow-up Hermes notes follow-up RESEARCH_DOC_SECRET\n")
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository fixtures.
---

RESEARCH_SKILL_SECRET
`)

	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	report := RenderResearchCLIReport(cfg, repoContext)
	for _, want := range []string{
		"GitClaw Research Catalog Report",
		"scope: `local-cli`",
		"research_catalog_status: `ok`",
		"catalog_strategy: `primary-source-to-repo-native-design-map`",
		"research_scope: `openclaw, hermes-agent, nano-mini-claw-variants`",
		"source_snapshot_date: `2026-06-01`",
		"reviewed_sources: `10`",
		"primary_sources: `10`",
		"official_docs_sources: `8`",
		"official_repo_sources: `2`",
		"local_research_docs: `3`",
		"local_research_docs_present: `3`",
		"research_followups_indexed: `2`",
		"implemented_patterns: `6`",
		"adapted_patterns: `5`",
		"rejected_patterns: `5`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"raw_research_bodies_included: `false`",
		"raw_source_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompt_bodies_included: `false`",
		"credential_values_included: `false`",
		"llm_e2e_required_after_research_catalog_change: `true`",
		"### Reviewed Sources",
		"source_id=`openclaw-architecture`",
		"source_id=`hermes-skills`",
		"### Pattern Coverage",
		"pattern=`serverless-wakeup`",
		"pattern=`progressive-skills`",
		"### Rejected Patterns",
		"surface=`long-running-gateway-socket`",
		"surface=`agent-managed-skill-writes`",
		"runtime_fetch_gate=`disabled-static-reviewed-snapshot`",
		"model_e2e_gate=`required`",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("research catalog report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{"RESEARCH_README_SECRET", "RESEARCH_SPEC_SECRET", "RESEARCH_DOC_SECRET", "RESEARCH_SKILL_SECRET", "GitClaw README body", "OpenClaw notes follow-up"} {
		if strings.Contains(report, leaked) {
			t.Fatalf("research catalog report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestResearchCatalogCommandReportsSurfaceWithoutBodies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "README RESEARCH_CLI_SECRET\n")
	writeTestFile(t, root, "docs/spec-github-native-gitclaw.md", "Spec RESEARCH_CLI_SPEC_SECRET\n")
	writeTestFile(t, root, "docs/research-openclaw-hermes-landscape.md", "follow-up RESEARCH_CLI_DOC_SECRET\n")
	t.Setenv("GITCLAW_WORKDIR", root)

	output := captureStdout(t, func() {
		if err := RunCLI(context.Background(), []string{"research", "catalog"}); err != nil {
			t.Fatalf("research catalog returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Research Catalog Report",
		"scope: `local-cli`",
		"research_catalog_status: `ok`",
		"reviewed_sources: `10`",
		"local_research_docs_present: `3`",
		"gitclaw_surface=`/skills catalog/select-plan/runtime/sources`",
		"surface=`remote-skill-install`",
		"raw_body_gate=`paths-urls-counts-hashes-and-decisions-only`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("research catalog output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"RESEARCH_CLI_SECRET", "RESEARCH_CLI_SPEC_SECRET", "RESEARCH_CLI_DOC_SECRET"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("research catalog output leaked %q:\n%s", leaked, output)
		}
	}
}

func TestHandleResearchCatalogCommandPostsReportWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "README RESEARCH_HANDLE_SECRET\n")
	writeTestFile(t, root, "docs/spec-github-native-gitclaw.md", "Spec RESEARCH_HANDLE_SPEC_SECRET\n")
	writeTestFile(t, root, "docs/research-openclaw-hermes-landscape.md", "follow-up RESEARCH_HANDLE_DOC_SECRET\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 612,
			"title": "@gitclaw /research catalog",
			"body": "Hidden research body token: RESEARCH_ISSUE_BODY_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{612: nil}}
	llm := &FakeLLM{Response: "should not be used"}
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for deterministic research report", llm.Calls)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	body := github.Posted[0].Body
	for _, want := range []string{
		"GitClaw Research Catalog Report",
		"Generated without a model call",
		"model=\"gitclaw/research\"",
		"repository: `owner/repo`",
		"issue: `#612`",
		"requested_research_command: `catalog`",
		"research_command_status: `ok`",
		"research_catalog_status: `ok`",
		"reviewed_sources: `10`",
		"llm_e2e_required_after_research_catalog_change: `true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("research handler report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"RESEARCH_ISSUE_BODY_SECRET", "RESEARCH_HANDLE_SECRET", "RESEARCH_HANDLE_SPEC_SECRET", "RESEARCH_HANDLE_DOC_SECRET", "Hidden research body token"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("research handler report leaked %q:\n%s", leaked, body)
		}
	}
}
