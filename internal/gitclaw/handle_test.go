package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleDryRunPostsExactlyOneIdempotentComment(t *testing.T) {
	eventJSON := []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 42,
			"title": "@gitclaw explain auth",
			"body": "How does auth work?",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": []
		},
		"sender": {"login": "alice", "type": "User"},
		"after": "abc123"
	}`)
	ev, err := ParseEvent("issues", eventJSON)
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{
		CommentsByIssue: map[int][]Comment{42: nil},
	}
	llm := &FakeLLM{Response: "Auth is handled by the repo code."}
	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("posted %d comments, want 1", len(github.Posted))
	}
	if !strings.Contains(github.Posted[0].Body, "Auth is handled") {
		t.Fatalf("posted body missing LLM response: %s", github.Posted[0].Body)
	}

	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}
	if len(github.Posted) != 1 {
		t.Fatalf("idempotent retry posted %d comments, want still 1", len(github.Posted))
	}
	if llm.Calls != 1 {
		t.Fatalf("LLM called %d times, want 1 due existing idempotency marker", llm.Calls)
	}
}

func TestHandleSkipsUntrustedBeforeLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 2,
			"title": "@gitclaw explain",
			"body": "",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 55,
			"body": "run this",
			"author_association": "CONTRIBUTOR",
			"user": {"login": "mallory", "type": "User"}
		},
		"sender": {"login": "mallory", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{2: nil}}
	llm := &FakeLLM{Response: "should not be used"}
	err = Handle(context.Background(), ev, DefaultConfig(), github, llm)
	if err == nil {
		t.Fatalf("Handle should return preflight rejection")
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called for untrusted actor")
	}
	if len(github.Posted) != 0 {
		t.Fatalf("posted comments for untrusted actor: %#v", github.Posted)
	}
}

func TestHandlePassesRepoContextToLLM(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/AnandChowdhary/gitclaw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gitclaw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitclaw", "SOUL.md"), []byte("Be repo-native."), 0o600); err != nil {
		t.Fatal(err)
	}
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 88,
			"title": "@gitclaw inspect go.mod",
			"body": "What module path is in go.mod?",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{88: nil}}
	llm := &FakeLLM{Response: "module path found"}
	cfg := DefaultConfig()
	cfg.Workdir = root
	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !hasContextDoc(llm.LastRequest.Context.Documents, ".gitclaw/SOUL.md", "repo-native") {
		t.Fatalf("LLM request missing soul context: %#v", llm.LastRequest.Context.Documents)
	}
	if !hasToolOutput(llm.LastRequest.Context.ToolOutputs, "gitclaw.read_file", "go.mod", "module github.com/AnandChowdhary/gitclaw") {
		t.Fatalf("LLM request missing read_file tool output: %#v", llm.LastRequest.Context.ToolOutputs)
	}
}

type FakeGitHub struct {
	Issues          []Issue
	CommentsByIssue map[int][]Comment
	Posted          []PostedComment
}

func (f *FakeGitHub) ListOpenIssues(ctx context.Context, repo string, labels []string, limit int) ([]Issue, error) {
	var issues []Issue
	for _, issue := range f.Issues {
		if !issueHasAllLabels(issue, labels) {
			continue
		}
		issues = append(issues, issue)
		if limit > 0 && len(issues) >= limit {
			break
		}
	}
	return issues, nil
}

func (f *FakeGitHub) ListIssueComments(ctx context.Context, repo string, issueNumber int) ([]Comment, error) {
	return append([]Comment(nil), f.CommentsByIssue[issueNumber]...), nil
}

func (f *FakeGitHub) PostIssueComment(ctx context.Context, repo string, issueNumber int, body string) (PostedComment, error) {
	posted := PostedComment{ID: int64(9000 + len(f.Posted)), Body: body}
	f.Posted = append(f.Posted, posted)
	f.CommentsByIssue[issueNumber] = append(f.CommentsByIssue[issueNumber], Comment{
		ID:   posted.ID,
		Body: body,
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	})
	return posted, nil
}

type FakeLLM struct {
	Response    string
	Calls       int
	LastRequest LLMRequest
}

func (f *FakeLLM) Complete(ctx context.Context, req LLMRequest) (string, error) {
	f.Calls++
	f.LastRequest = req
	return f.Response, nil
}

func issueHasAllLabels(issue Issue, labels []string) bool {
	for _, label := range labels {
		if !hasLabel(issue.Labels, label) {
			return false
		}
	}
	return true
}
