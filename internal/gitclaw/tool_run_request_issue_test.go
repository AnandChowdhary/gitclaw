package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleToolsRequestRunCreatesReviewIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 210,
			"title": "@gitclaw /tools request-run search_files --id review-search-tool",
			"body": "Please queue a reviewed tool run request.\n\nTOOL_RUN_REQUEST_SOURCE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{210: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for tool run request action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one tool run request issue: %#v", len(github.Issues), github.Issues)
	}
	requestIssue := github.Issues[0]
	for _, want := range []string{
		"gitclaw:tool-run-request-issue",
		`id="review-search-tool"`,
		`normalized_tool="gitclaw.search_files"`,
		"review_decision: review_required_read_only_tool",
		"tool_execution_performed: false",
		"raw_source_body_included: false",
		"raw_tool_inputs_included: false",
		"raw_tool_outputs_included: false",
	} {
		if !strings.Contains(requestIssue.Body, want) {
			t.Fatalf("tool run request issue missing %q:\n%s", want, requestIssue.Body)
		}
	}
	if strings.Contains(requestIssue.Body, "TOOL_RUN_REQUEST_SOURCE_SECRET") || strings.Contains(requestIssue.Body, "Please queue") {
		t.Fatalf("tool run request issue leaked source body:\n%s", requestIssue.Body)
	}

	sourceComments := github.CommentsByIssue[210]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want action receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Tool Run Request Issue Action",
		"Generated without a model call",
		`model="gitclaw/tools"`,
		"requested_tool_command: `/tools request-run`",
		"tool_run_request_status: `created`",
		"tool_run_request_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"tool_run_request_id: `review-search-tool`",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tool: `gitclaw.search_files`",
		"review_decision: `review_required_read_only_tool`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"raw_source_body_included: `false`",
		"raw_tool_name_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_tool_run_request_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("tool run request receipt missing %q:\n%s", want, receipt)
		}
	}
	if strings.Contains(receipt, "TOOL_RUN_REQUEST_SOURCE_SECRET") || strings.Contains(receipt, "Please queue") {
		t.Fatalf("tool run request receipt leaked source body:\n%s", receipt)
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 210,
			"title": "@gitclaw /tools request-run search_files --id review-search-tool",
			"body": "Please queue a reviewed tool run request.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 21001,
			"body": "@gitclaw /tools request-run search_files --id review-search-tool\n\nTOOL_RUN_REQUEST_DUPLICATE_SECRET",
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
		t.Fatalf("duplicate created another request issue: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[210][1].Body
	for _, want := range []string{
		"tool_run_request_status: `existing`",
		"tool_run_request_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"tool_run_request_id: `review-search-tool`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool run request receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "TOOL_RUN_REQUEST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate receipt leaked source body:\n%s", duplicateReceipt)
	}
}
