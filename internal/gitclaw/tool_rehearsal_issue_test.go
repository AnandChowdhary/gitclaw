package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleToolsRehearseCreatesConversationIssueWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\n")
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 212,
			"title": "@gitclaw /tools rehearse search_files --id search-tool-lab",
			"body": "Please open a tool rehearsal lane.\n\nTOOL_REHEARSAL_SOURCE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{212: nil}}
	llm := &FakeLLM{Response: "should not be called"}
	cfg := DefaultConfig()
	cfg.Workdir = root

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for tool rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one tool rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[0]
	for _, want := range []string{
		"gitclaw:tool-rehearsal-issue",
		`id="search-tool-lab"`,
		`normalized_tool="gitclaw.search_files"`,
		"GitClaw tool rehearsal issue",
		"rehearsal_id: search-tool-lab",
		"normalized_tool: gitclaw.search_files",
		"matched_tool: gitclaw.search_files",
		"rehearsal_mode: github-issue-conversation",
		"tool_execution_performed: false",
		"tool_inputs_generated: false",
		"tool_run_request_created: false",
		"raw_source_body_included: false",
		"raw_tool_inputs_included: false",
		"raw_tool_outputs_included: false",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("tool rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	if !hasLabel(github.IssueLabels[rehearsalIssue.Number], "gitclaw") {
		t.Fatalf("tool rehearsal issue missing gitclaw trigger label: %#v", github.IssueLabels[rehearsalIssue.Number])
	}
	if strings.Contains(rehearsalIssue.Body, "TOOL_REHEARSAL_SOURCE_SECRET") || strings.Contains(rehearsalIssue.Body, "Please open") {
		t.Fatalf("tool rehearsal issue leaked source body:\n%s", rehearsalIssue.Body)
	}

	sourceComments := github.CommentsByIssue[212]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want action receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Tool Rehearsal Issue Action",
		"Generated without a model call",
		`model="gitclaw/tools"`,
		"requested_tool_command: `/tools rehearse`",
		"tool_rehearsal_status: `created`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"tool_rehearsal_id_sha256_12:",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tool: `gitclaw.search_files`",
		"rehearsal_mode: `github-issue-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"tool_inputs_generated: `false`",
		"tool_run_request_created: `false`",
		"raw_source_body_included: `false`",
		"raw_tool_name_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_tool_rehearsal_issue_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("tool rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"TOOL_REHEARSAL_SOURCE_SECRET", "Please open", "search-tool-lab"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("tool rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 212,
			"title": "@gitclaw /tools rehearse search_files --id search-tool-lab",
			"body": "Please open a tool rehearsal lane.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 21201,
			"body": "@gitclaw /tools rehearse search_files --id search-tool-lab\n\nTOOL_REHEARSAL_DUPLICATE_SECRET",
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
		t.Fatalf("duplicate created another rehearsal issue: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[212][1].Body
	for _, want := range []string{
		"tool_rehearsal_status: `existing`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"rehearsal_issue: `#100`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"TOOL_REHEARSAL_DUPLICATE_SECRET", "search-tool-lab"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate tool rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildToolRehearsalIssueRequestParsesToolAndID(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 27,
			Title:  "Tool lab",
		},
		Comment: &Comment{
			ID:   2701,
			Body: "@gitclaw /tools lab --tool read_file --id tool-lab-27",
		},
	}
	req, err := BuildToolRehearsalIssueRequest(ev, DefaultConfig(), RepoContext{})
	if err != nil {
		t.Fatalf("BuildToolRehearsalIssueRequest returned error: %v", err)
	}
	if req.Subcommand != "lab" || req.RehearsalID != "tool-lab-27" || req.NormalizedTool != "gitclaw.read_file" || req.MatchedTool != "gitclaw.read_file" {
		t.Fatalf("unexpected tool rehearsal parsing: %#v", req)
	}
	if req.SourceKind != "comment" || req.SourceCommentID != 2701 || req.RequestedToolSHA == "" || req.RequestedToolTerms == 0 {
		t.Fatalf("expected tool rehearsal source metadata: %#v", req)
	}
}
