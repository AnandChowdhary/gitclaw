package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSessionSearchQueuesRecallWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-session-search-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-session-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-session-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90201,
			"body": "@gitclaw /channels session-search CHANNEL_SESSION_SEARCH_QUERY_SECRET --message-id search-inbound-902 --notify-message-id search-notify-902 --search-id Search.Secret.902 --max-results 3\nDo not include this command hidden token in the receipt: CHANNEL_SESSION_SEARCH_COMMAND_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 902,
			Title:  "GitClaw telegram thread chat-session-search-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{902: {{
			ID: 90200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-session-search-123",
				MessageID: "search-inbound-902",
				Author:    "telegram",
				Body:      "Earlier mirrored context contains CHANNEL_SESSION_SEARCH_QUERY_SECRET and hidden body token CHANNEL_SESSION_SEARCH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{902: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel session search action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("session search action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[902]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="search-notify-902"`,
		"GitClaw channel session search",
		"Search status: ok",
		"Query hash: ",
		"Query terms: 1",
		"Max results: 3",
		"Transcript messages: 2",
		"Matched messages: 1",
		"Matched lines: 1",
		"Results returned: 1",
		"message=02 role=user source=comment:90200",
		"message_sha256_12=",
		"line_sha256_12=",
		"raw search queries are not included",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("session search notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SESSION_SEARCH_QUERY_SECRET", "CHANNEL_SESSION_SEARCH_INGEST_MARKER", "CHANNEL_SESSION_SEARCH_COMMAND_MARKER", "Search.Secret.902"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("session search notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Session Search Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels session-search`",
		"channel_session_search_status: `queued`",
		"session_search_status: `ok`",
		"search_mode: `github-issue-transcript-local-lexical`",
		"notification_target_issue: `#902`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"search_id_sha256_12: `",
		"search_id_auto: `false`",
		"query_sha256_12: `",
		"query_terms: `1`",
		"query_bytes: `35`",
		"query_source: `positional`",
		"max_results: `3`",
		"transcript_messages: `2`",
		"matched_messages: `1`",
		"matched_lines: `1`",
		"results_returned: `1`",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_query_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_search_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_session_search_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel session search receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SESSION_SEARCH_QUERY_SECRET", "CHANNEL_SESSION_SEARCH_INGEST_MARKER", "CHANNEL_SESSION_SEARCH_COMMAND_MARKER", "chat-session-search-123", "search-inbound-902", "search-notify-902", "Search.Secret.902"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel session search receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 902,
			"title": "GitClaw telegram thread chat-session-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-session-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90202,
			"body": "@gitclaw /channels recall-session CHANNEL_SESSION_SEARCH_QUERY_SECRET --message-id search-inbound-902 --notify-message-id search-notify-902 --search-id Search.Secret.902 --max-results 3\nDo not include duplicate hidden token CHANNEL_SESSION_SEARCH_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[902]); got != 4 {
		t.Fatalf("duplicate session search posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[902])
	}
	duplicateReceipt := github.CommentsByIssue[902][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels recall-session`",
		"channel_session_search_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate session search receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SESSION_SEARCH_QUERY_SECRET", "CHANNEL_SESSION_SEARCH_DUPLICATE_MARKER", "chat-session-search-123", "search-inbound-902", "search-notify-902", "Search.Secret.902"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate session search receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSessionSearchActionRequestParsesRouteAliasAndTrailingQuery(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel session search"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel thread-search --route team-demo --message-id source-1 --notify-message-id notify-1 --id Session.Search.One --max-results 5
Query: deployment outage`,
		},
	}
	transcript := []TranscriptMessage{{
		Role:              "user",
		Body:              "deployment outage notes",
		Actor:             "alice",
		AuthorAssociation: "MEMBER",
		Trusted:           true,
	}}
	req, err := BuildChannelSessionSearchActionRequest(ev, DefaultConfig(), transcript)
	if err != nil {
		t.Fatalf("BuildChannelSessionSearchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "thread-search" || req.Options.Route != "team-demo" || req.Options.Query != "deployment outage" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SearchID != "session-search-one" || req.Options.MaxResults != 5 {
		t.Fatalf("unexpected channel session search parsing: %#v", req)
	}
	if req.QuerySource != "trailing-query" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSearchID {
		t.Fatalf("unexpected channel session search defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SearchIDHash == "" || req.QuerySHA == "" || req.NotificationBodySHA == "" || req.Search.ResultsReturned != 1 {
		t.Fatalf("expected route search hashes and result: %#v", req)
	}
	if !IsChannelSessionSearchActionRequest(ev, DefaultConfig()) {
		t.Fatalf("expected channel thread-search alias to be recognized")
	}
}
