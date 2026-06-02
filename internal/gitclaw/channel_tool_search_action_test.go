package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolSearchQueuesCapabilitySearchWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelToolSearchFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-search-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 906,
			"title": "GitClaw telegram thread chat-tool-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90601,
			"body": "@gitclaw /channels tool-search read_file --message-id tool-search-inbound-906 --notify-message-id tool-search-notify-906 --search-id Tool.Search.Secret.906 --max-results 1\nDo not include this command hidden token in the receipt: CHANNEL_TOOL_SEARCH_COMMAND_MARKER.",
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
			Number: 906,
			Title:  "GitClaw telegram thread chat-tool-search-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{906: {{
			ID: 90600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-search-123",
				MessageID: "tool-search-inbound-906",
				Author:    "telegram",
				Body:      "Original mirrored tool search command with CHANNEL_TOOL_SEARCH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{906: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool search action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("tool search action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[906]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="tool-search-notify-906"`,
		"GitClaw channel tool search",
		"Search status: ok",
		"Query hash: ",
		"Query terms: 1",
		"Max results: 1",
		"Available tools: 5",
		"Active tool outputs: ",
		"Matched contracts: 1",
		"Results returned: 1",
		"Search id hash: ",
		"kind=contract",
		"name=gitclaw.read_file",
		"match_fields=",
		"enabled=true",
		"disabled_by_config=false",
		"blocked_by_allowlist=false",
		"mode=read-only",
		"trigger_sha256_12=",
		"Raw tool inputs, tool output bodies, tool schemas, channel bodies, issue bodies, comment bodies, prompts, and raw search queries are not included.",
		"Tool execution: not performed by this action.",
		"Shell execution: not performed by this action.",
		"MCP server launch: not performed by this action.",
		"Toolset activation: not performed by this action.",
		"Model call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool search notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_SEARCH_INGEST_MARKER", "CHANNEL_TOOL_SEARCH_COMMAND_MARKER", "CHANNEL_TOOL_SEARCH_FILE_SECRET", "Tool.Search.Secret.906", "explicit repository-relative path"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("tool search notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Search Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tool-search`",
		"channel_tool_search_status: `queued`",
		"tool_search_status: `ok`",
		"search_mode: `deterministic-tool-contracts-local-lexical`",
		"notification_target_issue: `#906`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_search_id_sha256_12: `",
		"tool_search_id_auto: `false`",
		"query_sha256_12: `",
		"query_terms: `1`",
		"query_bytes: `9`",
		"query_source: `positional`",
		"max_results: `1`",
		"available_tools: `5`",
		"active_tool_outputs: `",
		"matched_contracts: `1`",
		"results_returned: `1`",
		"matched_tool_names_sha256_12: `",
		"matched_tool_index_sha256_12: `",
		"notification_body_sha256_12: `",
		"notification_body_bytes: `",
		"notification_body_lines: `",
		"tool_execution_allowed: `false`",
		"tool_execution_performed: `false`",
		"shell_execution_allowed: `false`",
		"mcp_server_launch_allowed: `false`",
		"toolset_activation_allowed: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_query_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_tool_search_id_included: `false`",
		"raw_tool_names_included: `false`",
		"raw_tool_triggers_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_tool_schemas_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"llm_e2e_required_after_channel_tool_search_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool search receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"read_file", "gitclaw.read_file", "explicit repository-relative path", "CHANNEL_TOOL_SEARCH_INGEST_MARKER", "CHANNEL_TOOL_SEARCH_COMMAND_MARKER", "CHANNEL_TOOL_SEARCH_FILE_SECRET", "chat-tool-search-123", "tool-search-inbound-906", "tool-search-notify-906", "Tool.Search.Secret.906"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool search receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 906,
			"title": "GitClaw telegram thread chat-tool-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90602,
			"body": "@gitclaw /channels search-tools read_file --message-id tool-search-inbound-906 --notify-message-id tool-search-notify-906 --search-id Tool.Search.Secret.906 --max-results 1\nDo not include duplicate hidden token CHANNEL_TOOL_SEARCH_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[906]); got != 4 {
		t.Fatalf("duplicate tool search posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[906])
	}
	duplicateReceipt := github.CommentsByIssue[906][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels search-tools`",
		"channel_tool_search_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"tool_execution_allowed: `false`",
		"tool_execution_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool search receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"read_file", "gitclaw.read_file", "CHANNEL_TOOL_SEARCH_DUPLICATE_MARKER", "chat-tool-search-123", "tool-search-inbound-906", "tool-search-notify-906", "Tool.Search.Secret.906"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate tool search receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelToolSearchActionRequestParsesRouteAliasAndTrailingQuery(t *testing.T) {
	root := t.TempDir()
	writeChannelToolSearchFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel tool search"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel tool-capability-search --route team-demo --message-id source-1 --notify-message-id notify-1 --id Tool.Search.One --max-results 5
Tool: read_file`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel tool-capability-search"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelToolSearchActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolSearchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-capability-search" || req.Options.Route != "team-demo" || req.Options.Query != "read_file" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SearchID != "tool-search-one" || req.Options.MaxResults != 5 {
		t.Fatalf("unexpected channel tool search parsing: %#v", req)
	}
	if req.QuerySource != "trailing-query" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSearchID {
		t.Fatalf("unexpected channel tool search defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SearchIDHash == "" || req.QuerySHA == "" || req.NotificationBodySHA == "" || req.Search.ResultsReturned != 1 {
		t.Fatalf("expected route search hashes and result: %#v", req)
	}
	if !IsChannelToolSearchActionRequest(ev, cfg) {
		t.Fatalf("expected channel tool-capability-search alias to be recognized")
	}
}

func writeChannelToolSearchFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "README.md", "Channel tool search fixture with CHANNEL_TOOL_SEARCH_FILE_SECRET.\n")
}
