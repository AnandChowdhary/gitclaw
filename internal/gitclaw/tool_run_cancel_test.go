package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleToolsCancelRunClosesRequestIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\n")
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 310,
			"title": "Tool request source",
			"body": "Source issue body TOOL_RUN_CANCEL_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 31001,
			"body": "@gitclaw /tools cancel-run --id review-search-tool\nDo not leak TOOL_RUN_CANCEL_COMMENT_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	request := ToolRunRequestIssueRequest{
		Repo:              "owner/repo",
		RequestID:         "review-search-tool",
		NormalizedTool:    "gitclaw.search_files",
		SourceIssueNumber: 310,
		SourceSHA:         "abc123",
		SourceKind:        "issue",
	}
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 99,
			Title:  "GitClaw tool run request: review-search-tool",
			Body:   RenderToolRunRequestIssueBody(request),
		}},
		CommentsByIssue: map[int][]Comment{310: nil, 99: nil},
	}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for tool run cancel action", llm.Calls)
	}
	if !github.ClosedIssues[99] {
		t.Fatalf("tool request issue was not closed: %#v", github.ClosedIssues)
	}
	requestComments := github.CommentsByIssue[99]
	if len(requestComments) != 1 {
		t.Fatalf("request issue comments = %d, want cancellation marker: %#v", len(requestComments), requestComments)
	}
	for _, want := range []string{
		"gitclaw:tool-run-cancel",
		`id="review-search-tool"`,
		`source_issue="310"`,
		`source_comment_id="31001"`,
		"tool_execution_performed: false",
		"approval_granted: false",
		"repository_mutation_performed: false",
		"raw_source_body_included: false",
	} {
		if !strings.Contains(requestComments[0].Body, want) {
			t.Fatalf("cancel comment missing %q:\n%s", want, requestComments[0].Body)
		}
	}

	sourceComments := github.CommentsByIssue[310]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want action receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Tool Run Cancel Action",
		"Generated without a model call",
		`model="gitclaw/tools"`,
		"requested_tool_command: `/tools cancel-run`",
		"tool_run_cancel_status: `cancelled`",
		"tool_run_request_issue: `#99`",
		"cancellation_comment_posted: `true`",
		"issue_closed: `true`",
		"request_not_found_or_closed: `false`",
		"request_id_sha256_12:",
		"cancellation_store: `github-issue-comment-plus-closed-state`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"approval_granted: `false`",
		"repository_mutation_performed: `false`",
		"raw_request_id_included: `false`",
		"raw_source_body_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_tool_run_cancel_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("tool run cancel receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"TOOL_RUN_CANCEL_SOURCE_SECRET", "TOOL_RUN_CANCEL_COMMENT_SECRET", "review-search-tool"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("tool run cancel receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 310,
			"title": "Tool request source",
			"body": "Source issue body.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 31002,
			"body": "@gitclaw /tools cancel-run --id review-search-tool\nDo not leak TOOL_RUN_CANCEL_DUPLICATE_SECRET.",
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
	if got := len(github.CommentsByIssue[99]); got != 1 {
		t.Fatalf("duplicate cancellation changed request issue comments: %d", got)
	}
	duplicateReceipt := github.CommentsByIssue[310][1].Body
	for _, want := range []string{
		"tool_run_cancel_status: `not_found_or_closed`",
		"tool_run_request_issue: `#0`",
		"cancellation_comment_posted: `false`",
		"issue_closed: `false`",
		"request_not_found_or_closed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool run cancel receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "TOOL_RUN_CANCEL_DUPLICATE_SECRET") || strings.Contains(duplicateReceipt, "review-search-tool") {
		t.Fatalf("duplicate tool run cancel receipt leaked raw content:\n%s", duplicateReceipt)
	}
}

func TestBuildToolRunCancelInfersIDFromRequestIssue(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 99,
			Title:  "GitClaw tool run request: inferred-tool-run",
			Body:   `<!-- gitclaw:tool-run-request-issue id="inferred-tool-run" normalized_tool="gitclaw.search_files" -->`,
		},
		Comment: &Comment{ID: 9901, Body: "@gitclaw /tools cancel-run"},
	}
	req, err := BuildToolRunCancelRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildToolRunCancelRequest returned error: %v", err)
	}
	if req.RequestID != "inferred-tool-run" || !req.RequestIDAuto || req.SourceKind != "comment" || req.SourceCommentID != 9901 {
		t.Fatalf("unexpected inferred cancel request: %#v", req)
	}
}
