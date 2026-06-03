package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolResultCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-tool-result-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-tool-result-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-result-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels tool-result --result-id result-1 --tool gitclaw.search_files --status success --exit-code 0 --recorded-at 2037-07-19T10:11:12Z --message-id inbound-384 --notify-message-id notify-384\nSummary: Search tool found the channel fixture\nResult:\nVisible external result with CHANNEL_TOOL_RESULT_DETAIL_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 384,
			Title:  "GitClaw telegram thread chat-tool-result-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-tool-result-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TOOL_RESULT_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool-result action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one tool-result issue: %#v", len(github.Issues), github.Issues)
	}
	resultIssue := github.Issues[1]
	if !HasChannelToolResultMarker(resultIssue.Body) || !strings.Contains(resultIssue.Body, `result_id="result-1"`) {
		t.Fatalf("tool-result issue missing channel-tool-result marker:\n%s", resultIssue.Body)
	}
	for _, want := range []string{
		"GitClaw channel tool result",
		"result_id: result-1",
		"tool_name: gitclaw.search_files",
		"status: success",
		"exit_code: 0",
		"recorded_at: 2037-07-19T10:11:12Z",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"tool_result_mode: github-issue-tool-result",
		"tool_execution_performed: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Tool",
		"## Status",
		"## Recorded At",
		"## Result Details",
		"Search tool found the channel fixture",
		"Visible external result with CHANNEL_TOOL_RESULT_DETAIL_SECRET.",
	} {
		if !strings.Contains(resultIssue.Body, want) {
			t.Fatalf("tool-result issue missing %q:\n%s", want, resultIssue.Body)
		}
	}
	if strings.Contains(resultIssue.Body, "chat-tool-result-123") || strings.Contains(resultIssue.Body, "inbound-384") || strings.Contains(resultIssue.Body, "CHANNEL_TOOL_RESULT_INGEST_SECRET") {
		t.Fatalf("tool-result issue leaked provider IDs or channel body:\n%s", resultIssue.Body)
	}
	if !hasLabel(github.IssueLabels[resultIssue.Number], "gitclaw") {
		t.Fatalf("tool-result issue missing gitclaw trigger label: %#v", github.IssueLabels[resultIssue.Number])
	}

	sourceComments := github.CommentsByIssue[384]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-384"`,
		"GitClaw channel tool result recorded.",
		"Tool result: #101",
		"https://github.com/owner/repo/issues/101",
		"Tool: gitclaw.search_files",
		"Status: success",
		"Summary: Search tool found the channel fixture",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("tool-result notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_TOOL_RESULT_DETAIL_SECRET") || strings.Contains(outbound, "CHANNEL_TOOL_RESULT_INGEST_SECRET") {
		t.Fatalf("tool-result notification leaked details or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Result Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels tool-result`",
		"channel_tool_result_status: `recorded`",
		"tool_result_issue: `#101`",
		"tool_result_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_name_sha256_12:",
		"tool_status_sha256_12:",
		"tool_exit_code_sha256_12:",
		"recorded_at_sha256_12:",
		"tool_result_summary_sha256_12:",
		"tool_result_details_sha256_12:",
		"raw_tool_result_id_included: `false`",
		"raw_tool_name_included: `false`",
		"raw_tool_status_included: `false`",
		"raw_tool_exit_code_included: `false`",
		"raw_recorded_at_included: `false`",
		"raw_tool_result_summary_included: `false`",
		"raw_tool_result_details_included: `false`",
		"raw_channel_message_body_included: `false`",
		"tool_execution_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_tool_result_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool-result receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_RESULT_INGEST_SECRET", "CHANNEL_TOOL_RESULT_DETAIL_SECRET", "Search tool found", "2037-07-19", "result-1", "gitclaw.search_files", "success", "chat-tool-result-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool-result receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-tool-result-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-tool-result-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels tool-result --result-id result-1 --tool gitclaw.search_files --status success --exit-code 0 --recorded-at 2037-07-19T10:11:12Z --message-id inbound-384 --notify-message-id notify-384\nSummary: Search tool found the channel fixture\nResult:\nDo not leak duplicate token CHANNEL_TOOL_RESULT_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate tool-result created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate tool-result posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_tool_result_status: `duplicate`",
		"tool_result_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate tool-result receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TOOL_RESULT_DUPLICATE_SECRET") {
		t.Fatalf("duplicate tool-result receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelToolResultActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel tool result"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel tool-output gitclaw.search_files --route team-demo --result-id Weekly.Tool.Result --status success --recorded-at 2037-07-19T10:11:12Z --message-id source-1 --notify-message-id notify-1
Summary: The external gateway returned the expected token
Exit code: 0
Output:
- Tool output is stable.
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelToolResultActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelToolResultActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-output" || req.Options.Route != "team-demo" || req.Options.ToolName != "gitclaw.search_files" || req.Options.ResultID != "weekly-tool-result" || req.Options.Status != "success" || req.Options.ExitCode != "0" || req.Options.RecordedAt != "2037-07-19T10:11:12Z" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel tool-result parsing: %#v", req)
	}
	if req.Options.Summary != "The external gateway returned the expected token" || !strings.Contains(req.Options.Details, "Tool output is stable") {
		t.Fatalf("unexpected summary/details: %#v", req)
	}
	if req.TargetFromIssue || req.AutoResultID || req.AutoNotifyMessageID || req.ToolSHA == "" || req.StatusSHA == "" || req.RecordedAtSHA == "" || req.SummarySHA == "" || req.DetailsSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route tool-result hashes: %#v", req)
	}
}
