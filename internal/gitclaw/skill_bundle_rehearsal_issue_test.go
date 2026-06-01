package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleSkillBundleRehearsalCreatesConversationIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---
Use repository file tools for grounded answers.

BUNDLE_REHEARSAL_SKILL_BODY_SECRET
`)
	writeTestFile(t, root, ".gitclaw/skill-bundles/repo-context.yaml", `name: repo-context
description: Repository context questions.
skills:
  - repo-reader
instruction: |
  Prefer repository context and deterministic tool outputs before answering.
  BUNDLE_REHEARSAL_INSTRUCTION_SECRET
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 281,
			"title": "Try a bundle safely",
			"body": "@gitclaw /bundles rehearse repo-context --id bundle-rehearsal-1\n\nDo not leak source token BUNDLE_REHEARSAL_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{281: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for skill bundle rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsal := github.Issues[0]
	if !strings.Contains(rehearsal.Body, "gitclaw:bundle-rehearsal-issue") || !strings.Contains(rehearsal.Body, `id="bundle-rehearsal-1"`) || !strings.Contains(rehearsal.Body, `bundle="repo-context"`) {
		t.Fatalf("rehearsal issue missing marker:\n%s", rehearsal.Body)
	}
	if !hasLabel(github.IssueLabels[rehearsal.Number], "gitclaw") {
		t.Fatalf("rehearsal issue missing gitclaw label: %#v", github.IssueLabels[rehearsal.Number])
	}
	for _, want := range []string{
		"GitClaw skill bundle rehearsal issue",
		"rehearsal_id: bundle-rehearsal-1",
		"requested_bundle: repo-context",
		"matched_bundles: 1",
		"available_bundles: 1",
		"bundle_path: .gitclaw/skill-bundles/repo-context.yaml",
		"bundle_skill_refs: 1",
		"resolved_bundle_skills: 1",
		"resolved_skills: repo-reader",
		"missing_bundle_skills: 0",
		"instruction_present: true",
		"rehearsal_mode: github-issue-conversation",
		"bundle_update_allowed: false",
		"skill_install_allowed: false",
		"raw_source_body_included: false",
		"raw_bundle_body_included: false",
		"raw_bundle_instruction_included: false",
		"raw_skill_bodies_included: false",
		"Use the `repo-context` bundle",
	} {
		if !strings.Contains(rehearsal.Body, want) {
			t.Fatalf("rehearsal issue missing %q:\n%s", want, rehearsal.Body)
		}
	}
	for _, leaked := range []string{"BUNDLE_REHEARSAL_SOURCE_SECRET", "BUNDLE_REHEARSAL_SKILL_BODY_SECRET", "BUNDLE_REHEARSAL_INSTRUCTION_SECRET", "Do not leak source token", "Prefer repository context"} {
		if strings.Contains(rehearsal.Body, leaked) {
			t.Fatalf("rehearsal issue leaked %q:\n%s", leaked, rehearsal.Body)
		}
	}

	sourceComments := github.CommentsByIssue[281]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want rehearsal receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Skill Bundle Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/skills"`,
		"requested_bundle_command: `/bundles rehearse`",
		"bundle_rehearsal_status: `created`",
		"rehearsal_issue: `#100`",
		"rehearsal_issue_created: `true`",
		"requested_bundle: `repo-context`",
		"matched_bundles: `1`",
		"available_bundles: `1`",
		"available_skills: `1`",
		"bundle_path: `.gitclaw/skill-bundles/repo-context.yaml`",
		"bundle_skill_refs: `1`",
		"resolved_bundle_skills: `1`",
		"missing_bundle_skills: `0`",
		"instruction_present: `true`",
		"bundle_risk_findings: `0`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"bundle_update_allowed: `false`",
		"skill_install_allowed: `false`",
		"raw_source_body_included: `false`",
		"raw_bundle_body_included: `false`",
		"raw_bundle_instruction_included: `false`",
		"raw_skill_bodies_included: `false`",
		"llm_e2e_required_after_bundle_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"BUNDLE_REHEARSAL_SOURCE_SECRET", "BUNDLE_REHEARSAL_SKILL_BODY_SECRET", "BUNDLE_REHEARSAL_INSTRUCTION_SECRET", "bundle-rehearsal-1", "Prefer repository context"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 281,
			"title": "Try a bundle safely",
			"body": "@gitclaw /bundles rehearse repo-context --id bundle-rehearsal-1\n\nDo not leak source token BUNDLE_REHEARSAL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28101,
			"body": "@gitclaw /bundles rehearse repo-context --id bundle-rehearsal-1\n\nDo not leak duplicate token BUNDLE_REHEARSAL_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	if err := Handle(context.Background(), duplicateEv, cfg, github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate rehearsal created more issues: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[281][1].Body
	for _, want := range []string{
		"bundle_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "BUNDLE_REHEARSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate rehearsal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildSkillBundleRehearsalIssueRequestParsesBundleOption(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 28, Title: "Rehearse bundle"},
		Comment: &Comment{
			ID:   2801,
			Body: "@gitclaw /bundle practice --bundle Repo.Context --id Design.Bundle",
		},
	}
	req, err := BuildSkillBundleRehearsalIssueRequest(ev, DefaultConfig(), RepoContext{})
	if err != nil {
		t.Fatalf("BuildSkillBundleRehearsalIssueRequest returned error: %v", err)
	}
	if req.Command != "/bundle" || req.Subcommand != "practice" || req.RequestedBundle != "repo-context" || req.RehearsalID != "design-bundle" {
		t.Fatalf("unexpected skill bundle rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 2801 || req.SourceSHA == "" {
		t.Fatalf("unexpected skill bundle rehearsal source metadata: %#v", req)
	}
}
