package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleSessionHandoffIssueCreatesNewLaneWithoutLLM(t *testing.T) {
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 290,
			"title": "Original session thread",
			"body": "@gitclaw Initial source issue with SESSION_HANDOFF_ISSUE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 29001,
			"body": "@gitclaw /session handoff --id handoff-1\n\nDo not leak SESSION_HANDOFF_COMMAND_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"created_at": "2026-06-01T12:00:00Z",
			"updated_at": "2026-06-01T12:00:00Z"
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	assistant := RenderAssistantComment(Marker{
		RunID:               "100",
		EventID:             "issue-290",
		Model:               "openai/gpt-5-nano",
		IdempotencyKey:      "SESSION_HANDOFF_IDEMPOTENCY_SECRET",
		RunURL:              "https://github.com/owner/repo/actions/runs/SESSION_HANDOFF_RUN_SECRET",
		PromptContextSHA:    "abc123abc123",
		ContextDocuments:    3,
		SelectedSkills:      1,
		ToolOutputs:         2,
		PromptVisibleSkills: []string{"repo-reader"},
		PromptVisibleTools:  []string{"gitclaw.list_files", "gitclaw.search_files"},
		Usage:               LLMUsage{Present: true, PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110},
	}, "assistant body with SESSION_HANDOFF_ASSISTANT_SECRET")
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{
		290: {
			{ID: 29000, Body: assistant, User: User{Login: "github-actions[bot]", Type: "Bot"}, AuthorAssociation: "MEMBER"},
			{
				ID:                29001,
				Body:              "@gitclaw /session handoff --id handoff-1\n\nDo not leak SESSION_HANDOFF_COMMAND_SECRET",
				User:              User{Login: "alice", Type: "User"},
				AuthorAssociation: "MEMBER",
				CreatedAt:         "2026-06-01T12:00:00Z",
				UpdatedAt:         "2026-06-01T12:00:00Z",
			},
		},
	}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for session handoff action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one handoff issue: %#v", len(github.Issues), github.Issues)
	}
	handoff := github.Issues[0]
	for _, want := range []string{
		"gitclaw:session-handoff-issue",
		`id="handoff-1"`,
		"GitClaw session handoff issue",
		"- handoff_id: handoff-1",
		"- handoff_mode: github-issue-conversation",
		"- source_issue: #290",
		"- source_issue_url: https://github.com/owner/repo/issues/290",
		"- source_comment_id: 29001",
		"- source_kind: comment",
		"- source_session_store: github-issue-thread",
		"- transcript_messages: 3",
		"- assistant_turn_comments: 1",
		"- assistant_turns_with_prompt_provenance: 1",
		"- model_backed_assistant_turns: 1",
		"- model_names: openai/gpt-5-nano",
		"- prompt_visible_skill_names: repo-reader",
		"- prompt_visible_tool_names: gitclaw.list_files, gitclaw.search_files",
		"- usage_total_tokens: 110",
		"- next_issue_comment_resumes_handoff: true",
		"- workflow_event: issue_comment",
		"- server_required: false",
		"- socket_required: false",
		"- external_session_db_required: false",
		"- raw_source_body_included: false",
		"- raw_comment_bodies_included: false",
		"- raw_assistant_replies_included: false",
		"- raw_prompts_included: false",
		"- raw_tool_outputs_included: false",
	} {
		if !strings.Contains(handoff.Body, want) {
			t.Fatalf("handoff issue missing %q:\n%s", want, handoff.Body)
		}
	}
	if !hasLabel(github.IssueLabels[handoff.Number], "gitclaw") {
		t.Fatalf("handoff issue missing gitclaw label: %#v", github.IssueLabels[handoff.Number])
	}
	for _, leaked := range []string{"SESSION_HANDOFF_ISSUE_SECRET", "SESSION_HANDOFF_COMMAND_SECRET", "SESSION_HANDOFF_ASSISTANT_SECRET", "SESSION_HANDOFF_IDEMPOTENCY_SECRET", "SESSION_HANDOFF_RUN_SECRET"} {
		if strings.Contains(handoff.Body, leaked) {
			t.Fatalf("handoff issue leaked %q:\n%s", leaked, handoff.Body)
		}
	}

	sourceComments := github.CommentsByIssue[290]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want original assistant, command, receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Session Handoff Issue Action",
		"Generated without a model call",
		`model="gitclaw/session"`,
		"requested_session_command: `/session handoff`",
		"session_handoff_status: `created`",
		"handoff_issue: `#100`",
		"handoff_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"handoff_issue_labeled_for_gitclaw: `true`",
		"transcript_messages: `3`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"model_backed_assistant_turns: `1`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"usage_total_tokens: `110`",
		"next_issue_comment_resumes_handoff: `true`",
		"workflow_event: `issue_comment`",
		"server_required: `false`",
		"socket_required: `false`",
		"external_session_db_required: `false`",
		"model_call_performed: `false`",
		"raw_handoff_id_included: `false`",
		"raw_source_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_session_handoff_issue_change: `true`",
		"### Handoff Path",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("session handoff receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"handoff-1", "SESSION_HANDOFF_ISSUE_SECRET", "SESSION_HANDOFF_COMMAND_SECRET", "SESSION_HANDOFF_ASSISTANT_SECRET", "SESSION_HANDOFF_IDEMPOTENCY_SECRET", "SESSION_HANDOFF_RUN_SECRET"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("session handoff receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 290,
			"title": "Original session thread",
			"body": "@gitclaw Initial source issue with SESSION_HANDOFF_ISSUE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 29002,
			"body": "@gitclaw /session handoff --id handoff-1\n\nDo not leak SESSION_HANDOFF_DUPLICATE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	github.CommentsByIssue[290] = append(github.CommentsByIssue[290], Comment{
		ID:                29002,
		Body:              "@gitclaw /session handoff --id handoff-1\n\nDo not leak SESSION_HANDOFF_DUPLICATE_SECRET",
		User:              User{Login: "alice", Type: "User"},
		AuthorAssociation: "MEMBER",
	})
	if err := Handle(context.Background(), duplicateEv, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate created another handoff issue: %#v", github.Issues)
	}
	duplicateReceipt := github.CommentsByIssue[290][len(github.CommentsByIssue[290])-1].Body
	for _, want := range []string{
		"session_handoff_status: `existing`",
		"handoff_issue: `#100`",
		"handoff_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"raw_handoff_id_included: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate session handoff receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"handoff-1", "SESSION_HANDOFF_DUPLICATE_SECRET"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate session handoff receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildSessionHandoffIssueRequestParsesAliases(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 291, Title: "handoff aliases"},
		Comment:   &Comment{ID: 29101, Body: "@gitclaw /sessions fork --handoff-id Design.Handoff"},
	}
	req, err := BuildSessionHandoffIssueRequest(ev, DefaultConfig(), nil, nil)
	if err != nil {
		t.Fatalf("BuildSessionHandoffIssueRequest returned error: %v", err)
	}
	if req.Command != "/sessions" || req.Subcommand != "fork" || req.HandoffID != "design-handoff" || req.SourceCommentID != 29101 {
		t.Fatalf("unexpected parsed handoff request: %#v", req)
	}
	if !IsSessionHandoffIssueRequest(ev, DefaultConfig()) {
		t.Fatalf("expected sessions fork alias to be recognized")
	}
}
