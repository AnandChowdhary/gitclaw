package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleSoulRehearsalCreatesConversationIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		".gitclaw/SOUL.md":              "Voice: direct, warm, pragmatic.",
		".gitclaw/IDENTITY.md":          "Identity: GitClaw.",
		".gitclaw/USER.md":              "User facts.",
		".gitclaw/TOOLS.md":             "Tools.",
		".gitclaw/MEMORY.md":            "Memory.",
		".gitclaw/HEARTBEAT.md":         "Heartbeat.",
		".gitclaw/memory/2026-06-01.md": "Daily note.",
	}
	for path, body := range files {
		writeTestFile(t, root, path, body)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 333,
			"title": "@gitclaw /soul rehearse --target soul --id voice-lab-1",
			"body": "Create a current-soul rehearsal lane. Hidden soul rehearsal token: SOUL_REHEARSAL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{333: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for soul rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created soul rehearsal issues = %d, want 1: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[0]
	if rehearsalIssue.Title != "GitClaw soul rehearsal: .gitclaw/SOUL.md (voice-lab-1)" || !strings.Contains(rehearsalIssue.Body, soulRehearsalIssueMarker) {
		t.Fatalf("unexpected soul rehearsal issue: %#v", rehearsalIssue)
	}
	if !hasLabel(github.IssueLabels[rehearsalIssue.Number], cfg.TriggerLabel) {
		t.Fatalf("rehearsal issue missing gitclaw label: %#v", github.IssueLabels[rehearsalIssue.Number])
	}
	for _, want := range []string{
		"rehearsal_id: voice-lab-1",
		"target_path: .gitclaw/SOUL.md",
		"target_category: soul",
		"source_issue: #333",
		"rehearsal_mode: github-issue-conversation",
		"context_target_write_allowed: false",
		"candidate_soul_generation_allowed: false",
		"repository_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_target_body_included: false",
		"raw_candidate_soul_included: false",
		"Use this issue to rehearse the current `.gitclaw/SOUL.md` behavior",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("soul rehearsal issue body missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"SOUL_REHEARSAL_SOURCE_SECRET", "Create a current-soul rehearsal lane", "Voice: direct"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("soul rehearsal issue body leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	if len(github.CommentsByIssue[333]) != 1 {
		t.Fatalf("source comments = %d, want soul rehearsal receipt: %#v", len(github.CommentsByIssue[333]), github.CommentsByIssue[333])
	}
	receipt := github.CommentsByIssue[333][0].Body
	for _, want := range []string{
		"GitClaw Soul Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/soul"`,
		"requested_soul_command: `/soul rehearse`",
		"soul_rehearsal_status: `created`",
		"rehearsal_issue: `#100`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"normalized_soul_path: `.gitclaw/SOUL.md`",
		"target_category: `soul`",
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"context_target_write_allowed: `false`",
		"candidate_soul_generation_allowed: `false`",
		"soul_file_written: `false`",
		"repository_mutation_performed: `false`",
		"raw_source_body_included: `false`",
		"raw_target_body_included: `false`",
		"raw_candidate_soul_included: `false`",
		"llm_e2e_required_after_soul_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("soul rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"SOUL_REHEARSAL_SOURCE_SECRET", "Create a current-soul rehearsal lane", "voice-lab-1", "Voice: direct"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("soul rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	commentEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 333,
			"title": "@gitclaw /soul rehearse --target soul --id voice-lab-1",
			"body": "Create a current-soul rehearsal lane.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 91,
			"body": "@gitclaw /soul rehearse --target soul --id voice-lab-1\nDuplicate soul rehearsal hidden token: SOUL_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate soul rehearsal created another issue: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[333][1].Body
	for _, want := range []string{
		"soul_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"rehearsal_issue: `#100`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"SOUL_REHEARSAL_DUPLICATE_SECRET", "voice-lab-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soul rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildSoulRehearsalIssueRequestParsesTargetAndID(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "Soul policy.")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "Identity policy.")
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /soul tone-test --target Identity --id Voice.Lab"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 41, Title: "Soul rehearsal"},
		Comment: &Comment{
			ID:   4101,
			Body: "@gitclaw /soul tone-test --target Identity --id Voice.Lab",
		},
	}
	req, err := BuildSoulRehearsalIssueRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildSoulRehearsalIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "tone-test" || req.RehearsalID != "voice-lab" || req.TargetPath != ".gitclaw/IDENTITY.md" {
		t.Fatalf("unexpected soul rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 4101 || req.SourceSHA == "" {
		t.Fatalf("unexpected soul rehearsal source metadata: %#v", req)
	}
}
