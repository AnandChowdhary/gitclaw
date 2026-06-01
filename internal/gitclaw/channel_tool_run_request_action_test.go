package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolRunRequestCreatesReviewIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\n")
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-request-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 485,
			"title": "GitClaw telegram thread chat-tool-request-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-request-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48501,
			"body": "@gitclaw /channels request-run search_files --id channel-tool-review --message-id inbound-485 --notify-message-id notify-485\nPlease review this channel-origin tool request.\nCHANNEL_TOOL_REQUEST_SOURCE_SECRET",
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
			Number: 485,
			Title:  "GitClaw telegram thread chat-tool-request-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{485: {{
			ID: 48500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-request-123",
				MessageID: "inbound-485",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TOOL_REQUEST_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{485: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool request action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one tool run request issue: %#v", len(github.Issues), github.Issues)
	}
	requestIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:tool-run-request-issue",
		`id="channel-tool-review"`,
		`normalized_tool="gitclaw.search_files"`,
		"request_id: channel-tool-review",
		"normalized_tool: gitclaw.search_files",
		"matched_tool: gitclaw.search_files",
		"review_decision: review_required_read_only_tool",
		"source_issue: #485",
		"source_kind: channel_comment",
		"tool_execution_performed: false",
		"model_call_performed: false",
		"raw_source_body_included: false",
		"raw_tool_inputs_included: false",
		"raw_tool_outputs_included: false",
	} {
		if !strings.Contains(requestIssue.Body, want) {
			t.Fatalf("tool run request issue missing %q:\n%s", want, requestIssue.Body)
		}
	}
	if strings.Contains(requestIssue.Body, "CHANNEL_TOOL_REQUEST_SOURCE_SECRET") || strings.Contains(requestIssue.Body, "CHANNEL_TOOL_REQUEST_INGEST_SECRET") || strings.Contains(requestIssue.Body, "Please review this channel-origin") {
		t.Fatalf("tool run request issue leaked source body:\n%s", requestIssue.Body)
	}

	sourceComments := github.CommentsByIssue[485]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-485"`,
		"GitClaw channel tool run request",
		"Review issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Normalized tool: gitclaw.search_files",
		"Review decision: review_required_read_only_tool",
		"Run allowed now: true",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel tool request notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_TOOL_REQUEST_SOURCE_SECRET") || strings.Contains(outbound, "CHANNEL_TOOL_REQUEST_INGEST_SECRET") {
		t.Fatalf("channel tool request notification leaked source:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Run Request Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels request-run`",
		"channel_tool_run_request_status: `created`",
		"tool_run_request_issue: `#101`",
		"tool_run_request_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#485`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"normalized_tool: `gitclaw.search_files`",
		"matched_tool: `gitclaw.search_files`",
		"review_decision: `review_required_read_only_tool`",
		"request_store: `github-issue-to-reviewed-tool-run`",
		"review_required: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"raw_request_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_requested_tool_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_tool_run_request_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool request receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_REQUEST_SOURCE_SECRET", "CHANNEL_TOOL_REQUEST_INGEST_SECRET", "Please review this channel-origin", "channel-tool-review", "chat-tool-request-123", "inbound-485", "notify-485"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool request receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 485,
			"title": "GitClaw telegram thread chat-tool-request-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-request-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48502,
			"body": "@gitclaw /channels request-run search_files --id channel-tool-review --message-id inbound-485 --notify-message-id notify-485\nDo not leak duplicate token CHANNEL_TOOL_REQUEST_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 2 {
		t.Fatalf("duplicate channel tool request created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[485]); got != 4 {
		t.Fatalf("duplicate channel tool request posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[485])
	}
	duplicateReceipt := github.CommentsByIssue[485][3].Body
	for _, want := range []string{
		"channel_tool_run_request_status: `duplicate`",
		"tool_run_request_issue: `#101`",
		"tool_run_request_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel tool request receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TOOL_REQUEST_DUPLICATE_SECRET") {
		t.Fatalf("duplicate channel tool request receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelToolRunRequestActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\n")
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext, err := LoadRepoContext(root, nil)
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 32, Title: "Channel tool request"},
		Comment: &Comment{
			ID:   3201,
			Body: `@gitclaw /channel tool-run --tool search_files --id Channel.Tool.Review --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelToolRunRequestActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolRunRequestActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-run" || req.Options.Channel != "slack" || req.Options.RequestID != "channel-tool-review" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel tool request parsing: %#v", req)
	}
	if req.ToolRequest.NormalizedTool != "gitclaw.search_files" || req.ToolRequest.ReviewDecision != "review_required_read_only_tool" || req.TargetFromIssue || req.AutoRequestID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected tool request details: %#v", req)
	}
}
