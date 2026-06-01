package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleSkillRehearsalCreatesConversationIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Use read-only repository context.
---
Use repository file tools for grounded answers.

SKILL_REHEARSAL_SKILL_BODY_SECRET
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 280,
			"title": "Try a skill safely",
			"body": "@gitclaw /skills rehearse repo-reader --id rehearsal-1\n\nDo not leak source token SKILL_REHEARSAL_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{280: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for skill rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsal := github.Issues[0]
	if !strings.Contains(rehearsal.Body, "gitclaw:skill-rehearsal-issue") || !strings.Contains(rehearsal.Body, `id="rehearsal-1"`) {
		t.Fatalf("rehearsal issue missing marker:\n%s", rehearsal.Body)
	}
	if !hasLabel(github.IssueLabels[rehearsal.Number], "gitclaw") {
		t.Fatalf("rehearsal issue missing gitclaw label: %#v", github.IssueLabels[rehearsal.Number])
	}
	for _, want := range []string{
		"GitClaw skill rehearsal issue",
		"rehearsal_id: rehearsal-1",
		"requested_skill: repo-reader",
		"matched_skills: 1",
		"enabled_matches: 1",
		"skill_install_allowed: false",
		"raw_source_body_included: false",
		"raw_skill_body_included: false",
		"Use the `repo-reader` skill",
	} {
		if !strings.Contains(rehearsal.Body, want) {
			t.Fatalf("rehearsal issue missing %q:\n%s", want, rehearsal.Body)
		}
	}
	for _, leaked := range []string{"SKILL_REHEARSAL_SOURCE_SECRET", "SKILL_REHEARSAL_SKILL_BODY_SECRET", "Do not leak source token"} {
		if strings.Contains(rehearsal.Body, leaked) {
			t.Fatalf("rehearsal issue leaked %q:\n%s", leaked, rehearsal.Body)
		}
	}

	sourceComments := github.CommentsByIssue[280]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want rehearsal receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Skill Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/skills"`,
		"requested_skill_command: `/skills rehearse`",
		"skill_rehearsal_status: `created`",
		"rehearsal_issue: `#100`",
		"rehearsal_issue_created: `true`",
		"requested_skill: `repo-reader`",
		"matched_skills: `1`",
		"enabled_matches: `1`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"skill_install_allowed: `false`",
		"active_skill_write_performed: `false`",
		"raw_source_body_included: `false`",
		"raw_skill_body_included: `false`",
		"llm_e2e_required_after_skill_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"SKILL_REHEARSAL_SOURCE_SECRET", "SKILL_REHEARSAL_SKILL_BODY_SECRET", "rehearsal-1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 280,
			"title": "Try a skill safely",
			"body": "@gitclaw /skills rehearse repo-reader --id rehearsal-1\n\nDo not leak source token SKILL_REHEARSAL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28001,
			"body": "@gitclaw /skills rehearse repo-reader --id rehearsal-1\n\nDo not leak duplicate token SKILL_REHEARSAL_DUPLICATE_SECRET.",
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
	duplicateReceipt := github.CommentsByIssue[280][1].Body
	for _, want := range []string{
		"skill_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "SKILL_REHEARSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate rehearsal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildSkillRehearsalIssueRequestParsesSkillOption(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 27, Title: "Rehearse"},
		Comment: &Comment{
			ID:   2701,
			Body: "@gitclaw /skills trial --skill Repo-Reader --id Design.Rehearsal",
		},
	}
	req, err := BuildSkillRehearsalIssueRequest(ev, DefaultConfig(), RepoContext{})
	if err != nil {
		t.Fatalf("BuildSkillRehearsalIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "trial" || req.RequestedSkill != "repo-reader" || req.RehearsalID != "design-rehearsal" {
		t.Fatalf("unexpected skill rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 2701 || req.SourceSHA == "" {
		t.Fatalf("unexpected skill rehearsal source metadata: %#v", req)
	}
}
