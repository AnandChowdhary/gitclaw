package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSessionHandoffCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-session-handoff-thread-123",
	})
	commandBody := "@gitclaw /channels handoff --id channel-session-handoff --message-id inbound-491 --notify-message-id notify-491\nPlease hand off this channel-origin session.\nCHANNEL_SESSION_HANDOFF_SOURCE_SECRET"
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 491,
			"title": "GitClaw telegram thread channel-session-handoff-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-session-handoff-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49102,
			"body": "@gitclaw /channels handoff --id channel-session-handoff --message-id inbound-491 --notify-message-id notify-491\nPlease hand off this channel-origin session.\nCHANNEL_SESSION_HANDOFF_SOURCE_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	assistant := RenderAssistantComment(Marker{
		RunID:               "100",
		EventID:             "issue-491",
		Model:               "openai/gpt-5-nano",
		IdempotencyKey:      "CHANNEL_SESSION_HANDOFF_IDEMPOTENCY_SECRET",
		RunURL:              "https://github.com/owner/repo/actions/runs/CHANNEL_SESSION_HANDOFF_RUN_SECRET",
		PromptContextSHA:    "abc123abc123",
		ContextDocuments:    3,
		SelectedSkills:      1,
		ToolOutputs:         2,
		PromptVisibleSkills: []string{"repo-reader"},
		PromptVisibleTools:  []string{"gitclaw.search_files"},
		Usage:               LLMUsage{Present: true, PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110},
	}, "assistant body with CHANNEL_SESSION_HANDOFF_ASSISTANT_SECRET")
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 491,
			Title:  "GitClaw telegram thread channel-session-handoff-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{491: {
			{
				ID: 49100,
				Body: RenderChannelMessageComment(ChannelIngestOptions{
					Channel:   "telegram",
					ThreadID:  "channel-session-handoff-thread-123",
					MessageID: "inbound-491",
					Author:    "telegram",
					Body:      "Original mirrored message with CHANNEL_SESSION_HANDOFF_INGEST_SECRET.",
				}),
				User: User{Login: "telegram", Type: "User"},
			},
			{ID: 49101, Body: assistant, User: User{Login: "github-actions[bot]", Type: "Bot"}, AuthorAssociation: "MEMBER"},
			{
				ID:                49102,
				Body:              commandBody,
				User:              User{Login: "alice", Type: "User"},
				AuthorAssociation: "MEMBER",
			},
		}},
		IssueLabels: map[int][]string{491: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel session handoff action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one handoff issue: %#v", len(github.Issues), github.Issues)
	}
	handoffIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:session-handoff-issue",
		`id="channel-session-handoff"`,
		"GitClaw session handoff issue",
		"- handoff_id: channel-session-handoff",
		"- handoff_mode: github-issue-conversation",
		"- source_issue: #491",
		"- source_issue_url: https://github.com/owner/repo/issues/491",
		"- source_comment_id: 49102",
		"- source_kind: channel_comment",
		"- source_session_store: github-issue-thread",
		"- transcript_messages: 4",
		"- assistant_turn_comments: 1",
		"- assistant_turns_with_prompt_provenance: 1",
		"- model_backed_assistant_turns: 1",
		"- model_names: openai/gpt-5-nano",
		"- prompt_visible_skill_names: repo-reader",
		"- prompt_visible_tool_names: gitclaw.search_files",
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
		if !strings.Contains(handoffIssue.Body, want) {
			t.Fatalf("handoff issue missing %q:\n%s", want, handoffIssue.Body)
		}
	}
	if !hasLabel(github.IssueLabels[handoffIssue.Number], "gitclaw") {
		t.Fatalf("handoff issue missing gitclaw label: %#v", github.IssueLabels[handoffIssue.Number])
	}
	for _, leaked := range []string{
		"CHANNEL_SESSION_HANDOFF_SOURCE_SECRET",
		"CHANNEL_SESSION_HANDOFF_INGEST_SECRET",
		"CHANNEL_SESSION_HANDOFF_ASSISTANT_SECRET",
		"CHANNEL_SESSION_HANDOFF_IDEMPOTENCY_SECRET",
		"CHANNEL_SESSION_HANDOFF_RUN_SECRET",
		"Please hand off this channel-origin session",
	} {
		if strings.Contains(handoffIssue.Body, leaked) {
			t.Fatalf("handoff issue leaked %q:\n%s", leaked, handoffIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[491]
	if len(sourceComments) != 5 {
		t.Fatalf("source comments = %d, want message, assistant, command, outbound, receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[3].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-491"`,
		"GitClaw channel session handoff",
		"Handoff issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Transcript messages: 4",
		"Assistant turns: 1",
		"Model-backed turns: 1",
		"Usage-bearing turns: 1",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel session handoff notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SESSION_HANDOFF_SOURCE_SECRET", "CHANNEL_SESSION_HANDOFF_INGEST_SECRET", "CHANNEL_SESSION_HANDOFF_ASSISTANT_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel session handoff notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[4].Body
	for _, want := range []string{
		"GitClaw Channel Session Handoff Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels handoff`",
		"channel_session_handoff_status: `created`",
		"handoff_issue: `#101`",
		"handoff_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#491`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"handoff_mode: `github-issue-conversation`",
		"handoff_issue_labeled_for_gitclaw: `true`",
		"source_session_store: `github-issue-thread`",
		"transcript_messages: `4`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"model_backed_assistant_turns: `1`",
		"model_names: `openai/gpt-5-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files`",
		"usage_total_tokens: `110`",
		"next_issue_comment_resumes_handoff: `true`",
		"workflow_event: `issue_comment`",
		"server_required: `false`",
		"socket_required: `false`",
		"external_session_db_required: `false`",
		"model_call_performed: `false`",
		"raw_handoff_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_session_handoff_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel session handoff receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{
		"channel-session-handoff",
		"channel-session-handoff-thread-123",
		"inbound-491",
		"notify-491",
		"CHANNEL_SESSION_HANDOFF_SOURCE_SECRET",
		"CHANNEL_SESSION_HANDOFF_INGEST_SECRET",
		"CHANNEL_SESSION_HANDOFF_ASSISTANT_SECRET",
		"CHANNEL_SESSION_HANDOFF_IDEMPOTENCY_SECRET",
		"CHANNEL_SESSION_HANDOFF_RUN_SECRET",
		"Please hand off this channel-origin session",
	} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel session handoff receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 491,
			"title": "GitClaw telegram thread channel-session-handoff-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-session-handoff-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49103,
			"body": "@gitclaw /channels handoff --id channel-session-handoff --message-id inbound-491 --notify-message-id notify-491\nDo not leak duplicate token CHANNEL_SESSION_HANDOFF_DUPLICATE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent duplicate returned error: %v", err)
	}
	github.CommentsByIssue[491] = append(github.CommentsByIssue[491], Comment{
		ID:                49103,
		Body:              "@gitclaw /channels handoff --id channel-session-handoff --message-id inbound-491 --notify-message-id notify-491\nDo not leak duplicate token CHANNEL_SESSION_HANDOFF_DUPLICATE_SECRET.",
		User:              User{Login: "alice", Type: "User"},
		AuthorAssociation: "MEMBER",
	})
	if err := Handle(context.Background(), duplicateEv, DefaultConfig(), github, llm); err != nil {
		t.Fatalf("Handle duplicate returned error: %v", err)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate channel session handoff created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[491]); got != 7 {
		t.Fatalf("duplicate channel session handoff posted unexpected comments: comments=%d %#v", got, github.CommentsByIssue[491])
	}
	duplicateReceipt := github.CommentsByIssue[491][6].Body
	for _, want := range []string{
		"channel_session_handoff_status: `duplicate`",
		"handoff_issue: `#101`",
		"handoff_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"raw_handoff_id_included: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel session handoff receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"channel-session-handoff", "CHANNEL_SESSION_HANDOFF_DUPLICATE_SECRET"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel session handoff receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSessionHandoffActionRequestParsesAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 35, Title: "Channel session handoff"},
		Comment: &Comment{
			ID:   3501,
			Body: `@gitclaw /channel session-fork --handoff-id Channel.Session.Handoff --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelSessionHandoffActionRequest(ev, DefaultConfig(), nil, nil)
	if err != nil {
		t.Fatalf("BuildChannelSessionHandoffActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "session-fork" || req.Options.Channel != "slack" || req.Options.HandoffID != "channel-session-handoff" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel session handoff parsing: %#v", req)
	}
	if req.Handoff.Command != "/session" || req.Handoff.Subcommand != "handoff" || req.Handoff.SourceKind != "channel_comment" || req.TargetFromIssue || req.AutoHandoffID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected channel session handoff details: %#v", req)
	}
	if !IsChannelSessionHandoffActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel session-fork alias to be recognized")
	}
}
