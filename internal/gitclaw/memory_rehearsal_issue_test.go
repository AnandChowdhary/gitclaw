package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleMemoryRehearsalCreatesConversationIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory: durable facts for tests.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-06-01.md", "Daily note for tests.\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 281,
			"title": "@gitclaw /memory rehearse --target long-term --id memory-lab-1",
			"body": "Do not leak source token MEMORY_REHEARSAL_SOURCE_SECRET.",
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
		t.Fatalf("LLM called %d times for memory rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsal := github.Issues[0]
	if rehearsal.Title != "GitClaw memory rehearsal: .gitclaw/MEMORY.md (memory-lab-1)" {
		t.Fatalf("unexpected rehearsal title: %q", rehearsal.Title)
	}
	if !hasLabel(github.IssueLabels[rehearsal.Number], "gitclaw") {
		t.Fatalf("rehearsal issue missing gitclaw trigger label: %#v", github.IssueLabels[rehearsal.Number])
	}
	for _, want := range []string{
		"gitclaw:memory-rehearsal-issue",
		`id="memory-lab-1"`,
		"GitClaw memory rehearsal issue.",
		"rehearsal_id: memory-lab-1",
		"target_kind: long-term",
		"target_path: .gitclaw/MEMORY.md",
		"source_issue: #281",
		"memory_validation_status: ok",
		"rehearsal_mode: github-issue-conversation",
		"memory_write_allowed: false",
		"candidate_memory_generation_allowed: false",
		"repository_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_target_memory_included: false",
		"raw_candidate_memory_included: false",
	} {
		if !strings.Contains(rehearsal.Body, want) {
			t.Fatalf("rehearsal issue body missing %q:\n%s", want, rehearsal.Body)
		}
	}
	for _, leaked := range []string{"MEMORY_REHEARSAL_SOURCE_SECRET", "Memory: durable facts"} {
		if strings.Contains(rehearsal.Body, leaked) {
			t.Fatalf("rehearsal issue body leaked %q:\n%s", leaked, rehearsal.Body)
		}
	}

	comments := github.CommentsByIssue[281]
	if len(comments) != 1 {
		t.Fatalf("source comments = %d, want memory rehearsal receipt: %#v", len(comments), comments)
	}
	receipt := comments[0].Body
	for _, want := range []string{
		"GitClaw Memory Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/memory"`,
		"requested_memory_command: `/memory rehearse`",
		"memory_rehearsal_status: `created`",
		"rehearsal_issue: `#100`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"normalized_target_kind: `long-term`",
		"normalized_target_path: `.gitclaw/MEMORY.md`",
		"memory_validation_status: `ok`",
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"memory_write_allowed: `false`",
		"candidate_memory_generation_allowed: `false`",
		"memory_file_written: `false`",
		"repository_mutation_performed: `false`",
		"raw_source_body_included: `false`",
		"raw_target_memory_included: `false`",
		"raw_candidate_memory_included: `false`",
		"llm_e2e_required_after_memory_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("memory rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"MEMORY_REHEARSAL_SOURCE_SECRET", "Do not leak source token", "memory-lab-1", "Memory: durable facts"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("memory rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 281,
			"title": "@gitclaw /memory rehearse --target long-term --id memory-lab-1",
			"body": "Do not leak source token MEMORY_REHEARSAL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28101,
			"body": "@gitclaw /memory rehearse --target long-term --id memory-lab-1\n\nDo not leak duplicate token MEMORY_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate memory rehearsal created more issues: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[281][1].Body
	for _, want := range []string{
		"memory_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"rehearsal_issue: `#100`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate memory rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"MEMORY_REHEARSAL_DUPLICATE_SECRET", "memory-lab-1", "Do not leak duplicate token"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate memory rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildMemoryRehearsalIssueRequestParsesTargetAndID(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Memory: durable facts for tests.\n")
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 28,
			Title:  "Memory rehearsal",
		},
		Comment: &Comment{
			ID:   2801,
			Body: "@gitclaw /memories recall-test --target memory --id Recall.Lab",
		},
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContextWithConfig(root, []TranscriptMessage{{Role: "user", Body: ev.Comment.Body, Actor: "alice", AuthorAssociation: "MEMBER", Trusted: true}}, cfg)
	if err != nil {
		t.Fatalf("LoadRepoContextWithConfig returned error: %v", err)
	}
	req, err := BuildMemoryRehearsalIssueRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildMemoryRehearsalIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "recall-test" || req.RehearsalID != "recall-lab" || req.Target.Kind != "long-term" || req.Target.Path != ".gitclaw/MEMORY.md" {
		t.Fatalf("unexpected memory rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 2801 || req.TargetSHA == "" {
		t.Fatalf("unexpected source or target metadata: %#v", req)
	}
}
