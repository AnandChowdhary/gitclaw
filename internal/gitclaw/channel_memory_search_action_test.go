package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMemorySearchQueuesRecallWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelMemorySearchFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-memory-search-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-memory-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-memory-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90401,
			"body": "@gitclaw /channels memory-search deployment CHANNEL_MEMORY_SEARCH_QUERY_SECRET --message-id memory-search-inbound-904 --notify-message-id memory-search-notify-904 --search-id Memory.Search.Secret.904 --max-results 2\nDo not include this command hidden token in the receipt: CHANNEL_MEMORY_SEARCH_COMMAND_MARKER.",
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
			Number: 904,
			Title:  "GitClaw telegram thread chat-memory-search-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{904: {{
			ID: 90400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-memory-search-123",
				MessageID: "memory-search-inbound-904",
				Author:    "telegram",
				Body:      "Original mirrored memory search command with CHANNEL_MEMORY_SEARCH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{904: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel memory search action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("memory search action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[904]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="memory-search-notify-904"`,
		"GitClaw channel memory search",
		"Search status: ok",
		"Query hash: ",
		"Query terms: 2",
		"Max results: 2",
		"Files scanned: 2",
		"Matched files: 2",
		"Matched lines: 2",
		"Results returned: 2",
		"path=.gitclaw/MEMORY.md",
		"path=.gitclaw/memory/2026-05-29.md",
		"line_sha256_12=",
		"raw search queries are not included",
		"Model call: not performed by this action.",
		"Memory write: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"External memory provider access: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("memory search notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_SEARCH_QUERY_SECRET", "CHANNEL_MEMORY_SEARCH_LONG_TERM_SECRET", "CHANNEL_MEMORY_SEARCH_DATED_SECRET", "Memory.Search.Secret.904"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("memory search notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Memory Search Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels memory-search`",
		"channel_memory_search_status: `queued`",
		"memory_search_status: `ok`",
		"search_mode: `repo-local-memory-lexical`",
		"notification_target_issue: `#904`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"search_id_sha256_12: `",
		"search_id_auto: `false`",
		"query_sha256_12: `",
		"query_terms: `2`",
		"query_bytes: `45`",
		"query_source: `positional`",
		"max_results: `2`",
		"files_scanned: `2`",
		"matched_files: `2`",
		"matched_lines: `2`",
		"results_returned: `2`",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"memory_writes_performed: `false`",
		"external_memory_provider_accessed: `false`",
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
		"raw_memory_bodies_included: `false`",
		"raw_memory_paths_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_memory_search_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel memory search receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_SEARCH_QUERY_SECRET", "CHANNEL_MEMORY_SEARCH_LONG_TERM_SECRET", "CHANNEL_MEMORY_SEARCH_DATED_SECRET", "CHANNEL_MEMORY_SEARCH_INGEST_MARKER", "CHANNEL_MEMORY_SEARCH_COMMAND_MARKER", "chat-memory-search-123", "memory-search-inbound-904", "memory-search-notify-904", "Memory.Search.Secret.904", ".gitclaw/MEMORY.md", ".gitclaw/memory/2026-05-29.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel memory search receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-memory-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-memory-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90402,
			"body": "@gitclaw /channels recall-memory deployment CHANNEL_MEMORY_SEARCH_QUERY_SECRET --message-id memory-search-inbound-904 --notify-message-id memory-search-notify-904 --search-id Memory.Search.Secret.904 --max-results 2\nDo not include duplicate hidden token CHANNEL_MEMORY_SEARCH_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[904]); got != 4 {
		t.Fatalf("duplicate memory search posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[904])
	}
	duplicateReceipt := github.CommentsByIssue[904][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels recall-memory`",
		"channel_memory_search_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"memory_writes_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate memory search receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_SEARCH_QUERY_SECRET", "CHANNEL_MEMORY_SEARCH_DUPLICATE_MARKER", "chat-memory-search-123", "memory-search-inbound-904", "memory-search-notify-904", "Memory.Search.Secret.904"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate memory search receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelMemorySearchActionRequestParsesRouteAliasAndTrailingQuery(t *testing.T) {
	root := t.TempDir()
	writeChannelMemorySearchFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel memory search"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel search-memory --route team-demo --message-id source-1 --notify-message-id notify-1 --id Memory.Search.One --max-results 5
Query: deployment archive`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel search-memory"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelMemorySearchActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelMemorySearchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "search-memory" || req.Options.Route != "team-demo" || req.Options.Query != "deployment archive" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SearchID != "memory-search-one" || req.Options.MaxResults != 5 {
		t.Fatalf("unexpected channel memory search parsing: %#v", req)
	}
	if req.QuerySource != "trailing-query" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSearchID {
		t.Fatalf("unexpected channel memory search defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SearchIDHash == "" || req.QuerySHA == "" || req.NotificationBodySHA == "" || req.Search.ResultsReturned != 2 {
		t.Fatalf("expected route search hashes and result: %#v", req)
	}
	if !IsChannelMemorySearchActionRequest(ev, cfg) {
		t.Fatalf("expected channel search-memory alias to be recognized")
	}
}

func writeChannelMemorySearchFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "# Memory\n\n- Stable deployment recall context. Hidden body token CHANNEL_MEMORY_SEARCH_LONG_TERM_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "# 2026-05-29\n\n- Dated deployment archive context. Hidden body token CHANNEL_MEMORY_SEARCH_DATED_SECRET.\n")
}
