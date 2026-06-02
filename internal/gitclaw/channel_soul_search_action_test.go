package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoulSearchQueuesRecallWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulSearchFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soul-search-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-soul-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90401,
			"body": "@gitclaw /channels soul-search operating CHANNEL_SOUL_SEARCH_QUERY_SECRET --message-id soul-search-inbound-904 --notify-message-id soul-search-notify-904 --search-id Soul.Search.Secret.904 --max-results 2\nDo not include this command hidden token in the receipt: CHANNEL_SOUL_SEARCH_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-soul-search-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{904: {{
			ID: 90400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soul-search-123",
				MessageID: "soul-search-inbound-904",
				Author:    "telegram",
				Body:      "Original mirrored soul search command with CHANNEL_SOUL_SEARCH_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{904: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul search action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("soul search action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[904]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="soul-search-notify-904"`,
		"GitClaw channel soul search",
		"Search status: ok",
		"Query hash: ",
		"Query terms: 2",
		"Max results: 2",
		"Files scanned: 2",
		"Matched files: 2",
		"Matched lines: 2",
		"Results returned: 2",
		"path=.gitclaw/SOUL.md",
		"category=soul",
		"path=.gitclaw/USER.md",
		"category=user",
		"line_sha256_12=",
		"Raw search queries are not included",
		"Model call: not performed by this action.",
		"Soul write: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Profile export: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soul search notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_SEARCH_QUERY_SECRET", "CHANNEL_SOUL_SEARCH_SOUL_SECRET", "CHANNEL_SOUL_SEARCH_USER_SECRET", "Soul.Search.Secret.904"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("soul search notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Search Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soul-search`",
		"channel_soul_search_status: `queued`",
		"soul_search_status: `ok`",
		"search_mode: `repo-local-high-authority-lexical`",
		"notification_target_issue: `#904`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"search_id_sha256_12: `",
		"search_id_auto: `false`",
		"query_sha256_12: `",
		"query_terms: `2`",
		"query_bytes: `",
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
		"soul_writes_performed: `false`",
		"profile_export_performed: `false`",
		"registry_contact_performed: `false`",
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
		"raw_soul_bodies_included: `false`",
		"raw_identity_bodies_included: `false`",
		"raw_user_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_tool_guidance_bodies_included: `false`",
		"raw_heartbeat_bodies_included: `false`",
		"raw_soul_file_paths_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_soul_search_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul search receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_SEARCH_QUERY_SECRET", "CHANNEL_SOUL_SEARCH_SOUL_SECRET", "CHANNEL_SOUL_SEARCH_USER_SECRET", "CHANNEL_SOUL_SEARCH_INGEST_MARKER", "CHANNEL_SOUL_SEARCH_COMMAND_MARKER", "chat-soul-search-123", "soul-search-inbound-904", "soul-search-notify-904", "Soul.Search.Secret.904", ".gitclaw/SOUL.md", ".gitclaw/USER.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul search receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-soul-search-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-search-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90402,
			"body": "@gitclaw /channels authority-search operating CHANNEL_SOUL_SEARCH_QUERY_SECRET --message-id soul-search-inbound-904 --notify-message-id soul-search-notify-904 --search-id Soul.Search.Secret.904 --max-results 2\nDo not include duplicate hidden token CHANNEL_SOUL_SEARCH_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate soul search posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[904])
	}
	duplicateReceipt := github.CommentsByIssue[904][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels authority-search`",
		"channel_soul_search_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul search receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_SEARCH_QUERY_SECRET", "CHANNEL_SOUL_SEARCH_DUPLICATE_MARKER", "chat-soul-search-123", "soul-search-inbound-904", "soul-search-notify-904", "Soul.Search.Secret.904"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soul search receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSoulSearchActionRequestParsesRouteAliasAndTrailingQuery(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulSearchFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel soul search"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel search-soul --route team-demo --message-id source-1 --notify-message-id notify-1 --id Soul.Search.One --max-results 5
Query: operating boundary`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel search-soul"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelSoulSearchActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSoulSearchActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "search-soul" || req.Options.Route != "team-demo" || req.Options.Query != "operating boundary" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SearchID != "soul-search-one" || req.Options.MaxResults != 5 {
		t.Fatalf("unexpected channel soul search parsing: %#v", req)
	}
	if req.QuerySource != "trailing-query" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSearchID {
		t.Fatalf("unexpected channel soul search defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SearchIDHash == "" || req.QuerySHA == "" || req.NotificationBodySHA == "" || req.Search.ResultsReturned != 2 {
		t.Fatalf("expected route search hashes and result: %#v", req)
	}
	if !IsChannelSoulSearchActionRequest(ev, cfg) {
		t.Fatalf("expected channel search-soul alias to be recognized")
	}
}

func writeChannelSoulSearchFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "# Soul\n\n- Repo-native operating boundary. Hidden body token CHANNEL_SOUL_SEARCH_SOUL_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "# User\n\n- User operating preference. Hidden body token CHANNEL_SOUL_SEARCH_USER_SECRET.\n")
}
