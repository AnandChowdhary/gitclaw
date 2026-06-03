package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBrowserStatusQueuesReadinessCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".github/workflows/gitclaw-channel-gateway.yml", "name: gateway\n")
	writeTestFile(t, root, ".github/workflows/gitclaw-channel-outbox.yml", "name: outbox\n")
	writeTestFile(t, root, ".gitclaw/mcp/browser-cdp.yaml", "secret browser body CHANNEL_BROWSER_STATUS_MCP_SECRET\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-browser-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 886,
			"title": "GitClaw telegram thread chat-browser-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-browser-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88601,
			"body": "@gitclaw /channels browser --message-id browser-inbound-886 --notify-message-id browser-notify-886 --status-id browser-status-secret-886\nDo not include this command hidden token in the receipt: CHANNEL_BROWSER_STATUS_COMMAND_SECRET.",
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
			Number: 886,
			Title:  "GitClaw telegram thread chat-browser-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{886: {{
			ID: 88600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-browser-123",
				MessageID: "browser-inbound-886",
				Author:    "telegram",
				Body:      "Original mirrored browser-status command with CHANNEL_BROWSER_STATUS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{886: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel browser status action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("browser status should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[886]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="browser-notify-886"`,
		"GitClaw channel browser status.",
		"Browser bridge: reviewed MCP spec present",
		"Browser MCP specs: 1",
		"MCP specs scanned: 1",
		"Channel gateway workflow: present",
		"Channel outbox workflow: present",
		"Run mode: read-only status card",
		"Browser session opened: not performed by this action.",
		"Browser navigation: not performed by this action.",
		"Browser screenshot: not performed by this action.",
		"Browser MCP server launch: not performed by this action.",
		"Model call: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("browser-status notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BROWSER_STATUS_INGEST_SECRET", "CHANNEL_BROWSER_STATUS_COMMAND_SECRET", "CHANNEL_BROWSER_STATUS_MCP_SECRET", "browser-status-secret-886"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("browser-status notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Browser Status Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels browser`",
		"channel_browser_status_status: `queued`",
		"browser_status_mode: `provider-facing-browser-readiness`",
		"notification_target_issue: `#886`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"mcp_specs_scanned: `1`",
		"browser_mcp_specs: `1`",
		"browser_bridge_reviewed: `true`",
		"channel_gateway_workflow_present: `true`",
		"channel_outbox_workflow_present: `true`",
		"run_mode: `read-only-status-card`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"browser_session_opened: `false`",
		"browser_navigation_performed: `false`",
		"browser_screenshot_performed: `false`",
		"browser_mcp_server_launch_performed: `false`",
		"workflow_mutation_performed: `false`",
		"provider_api_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_browser_status_id_included: `false`",
		"raw_mcp_spec_body_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_browser_status_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel browser status receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BROWSER_STATUS_INGEST_SECRET", "CHANNEL_BROWSER_STATUS_COMMAND_SECRET", "CHANNEL_BROWSER_STATUS_MCP_SECRET", "chat-browser-123", "browser-inbound-886", "browser-notify-886", "browser-status-secret-886"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel browser status receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 886,
			"title": "GitClaw telegram thread chat-browser-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-browser-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 88602,
			"body": "@gitclaw /channels browser-status --message-id browser-inbound-886 --notify-message-id browser-notify-886 --status-id browser-status-secret-886\nDo not leak duplicate token CHANNEL_BROWSER_STATUS_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate browser status created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[886]); got != 4 {
		t.Fatalf("duplicate browser status posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[886])
	}
	duplicateReceipt := github.CommentsByIssue[886][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels browser-status`",
		"channel_browser_status_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"browser_session_opened: `false`",
		"browser_navigation_performed: `false`",
		"browser_screenshot_performed: `false`",
		"browser_mcp_server_launch_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate browser status receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BROWSER_STATUS_DUPLICATE_SECRET", "chat-browser-123", "browser-inbound-886", "browser-notify-886", "browser-status-secret-886"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate browser status receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBrowserStatusActionRequestParsesRouteAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/mcp/github-read.yaml", "name: github-read\n")
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel browser"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel playwright-status --route team-demo --message-id source-1 --notify-message-id notify-1 --status-id browser.1`,
		},
	}
	req, err := BuildChannelBrowserStatusActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelBrowserStatusActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "playwright-status" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.StatusID != "browser-1" {
		t.Fatalf("unexpected channel browser status parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoStatusID || req.RequestedRouteHash == "" || req.StatusIDHash == "" || req.MCPSpecsScanned != 1 || req.BrowserMCPSpecs != 0 || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route browser-status hashes: %#v", req)
	}
}
