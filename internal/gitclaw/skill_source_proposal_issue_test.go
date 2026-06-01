package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleSkillSourceProposeCommandCreatesConversationIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeSkillSourceFixture(t, root)
	sourceRef := "github:example/weekly-review-source/.gitclaw/SKILLS/weekly-review/SKILL.md"
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 146,
			"title": "@gitclaw /skills sources propose weekly-review-source --source github:example/weekly-review-source/.gitclaw/SKILLS/weekly-review/SKILL.md --skill-path .gitclaw/SKILLS/weekly-review-source/SKILL.md --id weekly-review-source-review",
			"body": "Review this external skill source. Hidden source proposal token: SKILL_SOURCE_PROPOSE_ACTION_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{146: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for skills source propose action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created skill source proposal issues = %d, want 1: %#v", len(github.Issues), github.Issues)
	}
	proposalIssue := github.Issues[0]
	if proposalIssue.Title != "GitClaw skill source proposal: weekly-review-source" || !strings.Contains(proposalIssue.Body, skillSourceProposalIssueMarker) {
		t.Fatalf("unexpected proposal issue: %#v", proposalIssue)
	}
	if !hasLabel(github.IssueLabels[proposalIssue.Number], cfg.TriggerLabel) {
		t.Fatalf("proposal issue missing gitclaw trigger label: %#v", github.IssueLabels[proposalIssue.Number])
	}
	sourceRefSHA := shortDocumentHash(sourceRef)
	for _, want := range []string{
		"proposal_id: weekly-review-source-review",
		"source_name: weekly-review-source",
		"source_pin_path: .gitclaw/skill-sources/weekly-review-source.yaml",
		"proposed_skill_path: .gitclaw/SKILLS/weekly-review-source/SKILL.md",
		"source_kind: github",
		"source_ref_sha256_12: " + sourceRefSHA,
		"trust_level: review-pending",
		"install_mode: proposal-only",
		"requires_approval: true",
		"remote_fetch_allowed: false",
		"existing_source_pins: 1",
		"source_issue: #146",
		"raw_source_ref_included: false",
		"raw_source_body_included: false",
		"source_pin_file_written: false",
		"active_skill_write_performed: false",
	} {
		if !strings.Contains(proposalIssue.Body, want) {
			t.Fatalf("proposal issue body missing %q:\n%s", want, proposalIssue.Body)
		}
	}
	for _, leaked := range []string{sourceRef, "SKILL_SOURCE_PROPOSE_ACTION_SECRET", "Review this external skill source"} {
		if strings.Contains(proposalIssue.Body, leaked) {
			t.Fatalf("proposal issue body leaked %q:\n%s", leaked, proposalIssue.Body)
		}
	}
	if len(github.CommentsByIssue[146]) != 1 {
		t.Fatalf("source comments = %d, want proposal receipt: %#v", len(github.CommentsByIssue[146]), github.CommentsByIssue[146])
	}
	receipt := github.CommentsByIssue[146][0].Body
	for _, want := range []string{
		"GitClaw Skill Source Proposal Issue Action",
		"Generated without a model call",
		`model="gitclaw/skills"`,
		"requested_skill_command: `/skills sources propose`",
		"skill_source_proposal_status: `created`",
		"skill_source_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"source_name: `weekly-review-source`",
		"source_pin_path: `.gitclaw/skill-sources/weekly-review-source.yaml`",
		"proposed_skill_path: `.gitclaw/SKILLS/weekly-review-source/SKILL.md`",
		"source_ref_sha256_12: `" + sourceRefSHA + "`",
		"proposal_store: `github-issue-to-git-reviewed-skill-source-pin`",
		"proposal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"raw_source_ref_included: `false`",
		"raw_source_body_included: `false`",
		"source_pin_file_written: `false`",
		"active_skill_write_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_skill_source_proposal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("skills source propose receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{sourceRef, "SKILL_SOURCE_PROPOSE_ACTION_SECRET", "Review this external skill source"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("skills source propose receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	commentEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 146,
			"title": "@gitclaw /skills sources propose weekly-review-source --source github:example/weekly-review-source/.gitclaw/SKILLS/weekly-review/SKILL.md --skill-path .gitclaw/SKILLS/weekly-review-source/SKILL.md --id weekly-review-source-review",
			"body": "Review this external skill source.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 89,
			"body": "@gitclaw /skills sources propose weekly-review-source --source github:example/weekly-review-source/.gitclaw/SKILLS/weekly-review/SKILL.md --id weekly-review-source-review\nDuplicate request hidden token: SKILL_SOURCE_PROPOSE_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent comment returned error: %v", err)
	}
	if err := Handle(context.Background(), commentEv, cfg, github, llm); err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate source proposal created another issue: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[146][1].Body
	for _, want := range []string{
		"skill_source_proposal_status: `existing`",
		"skill_source_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate skills source propose receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{sourceRef, "SKILL_SOURCE_PROPOSE_DUPLICATE_SECRET"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate skills source propose receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildSkillSourceProposalIssueRequestSupportsSourceOnlyTarget(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 147,
			"title": "@gitclaw /skills sources propose https://github.com/example/focus-skill/blob/main/SKILL.md",
			"body": "Review a source-only target.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	repoContext, err := LoadRepoContextWithConfig(root, nil, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	req, err := BuildSkillSourceProposalIssueRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildSkillSourceProposalIssueRequest returned error: %v", err)
	}
	if req.SourceName != "focus-skill" || req.SourceKind != "github" || req.SkillPath != ".gitclaw/SKILLS/focus-skill/SKILL.md" {
		t.Fatalf("unexpected source-only request: %#v", req)
	}
	if req.SourceRefSHA != shortDocumentHash("https://github.com/example/focus-skill/blob/main/SKILL.md") {
		t.Fatalf("unexpected source ref hash: %#v", req)
	}
}
