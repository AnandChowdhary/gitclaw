package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolApprovalPlanCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "GitClaw tools are deterministic and reviewed.\nCHANNEL_TOOL_APPROVAL_TOOL_BODY_SECRET\n")
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-tool-approval-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 645,
			"title": "GitClaw telegram thread channel-tool-approval-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-tool-approval-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64501,
			"body": "@gitclaw /channels approval-plan search_files --id channel-tool-approval --message-id inbound-645 --notify-message-id notify-645\nPlease review this channel-origin tool approval boundary.\nCHANNEL_TOOL_APPROVAL_SOURCE_SECRET",
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
			Number: 645,
			Title:  "GitClaw telegram thread channel-tool-approval-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{645: {{
			ID: 64500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-tool-approval-thread-123",
				MessageID: "inbound-645",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TOOL_APPROVAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{645: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel tool approval plan action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one tool approval plan issue: %#v", len(github.Issues), github.Issues)
	}
	approvalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:tool-approval-plan-issue",
		`id="channel-tool-approval"`,
		`normalized_tool="gitclaw.search_files"`,
		"approval_plan_id: channel-tool-approval",
		"normalized_tool: gitclaw.search_files",
		"matched_tool: gitclaw.search_files",
		"source_issue: #645",
		"source_kind: channel_comment",
		"approval_mode: github-issue-dry-run",
		"approval_required: false",
		"approval_decision: no_approval_required_read_only",
		"run_allowed_now: true",
		"model_call_performed: false",
		"tool_execution_performed: false",
		"approval_granted: false",
		"repository_mutation_performed: false",
		"raw_source_body_included: false",
		"raw_tool_inputs_included: false",
		"raw_tool_outputs_included: false",
		"raw_approval_payloads_included: false",
		"GitClaw Tool Approval Plan Report",
	} {
		if !strings.Contains(approvalIssue.Body, want) {
			t.Fatalf("tool approval plan issue missing %q:\n%s", want, approvalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_APPROVAL_SOURCE_SECRET", "CHANNEL_TOOL_APPROVAL_INGEST_SECRET", "CHANNEL_TOOL_APPROVAL_TOOL_BODY_SECRET", "Please review this channel-origin"} {
		if strings.Contains(approvalIssue.Body, leaked) {
			t.Fatalf("tool approval plan issue leaked %q:\n%s", leaked, approvalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[645]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-645"`,
		"GitClaw channel tool approval plan",
		"Approval plan issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Normalized tool: gitclaw.search_files",
		"Matched tool: gitclaw.search_files",
		"Tool enabled: true",
		"Tool mode: read-only",
		"Approval required: false",
		"Run allowed now: true",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel tool approval notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_APPROVAL_SOURCE_SECRET", "CHANNEL_TOOL_APPROVAL_INGEST_SECRET", "CHANNEL_TOOL_APPROVAL_TOOL_BODY_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel tool approval notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Tool Approval Plan Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels approval-plan`",
		"channel_tool_approval_plan_status: `created`",
		"approval_plan_issue: `#101`",
		"approval_plan_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#645`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"tool_enabled: `true`",
		"mutating_contract: `false`",
		"approval_required: `false`",
		"approval_decision: `no_approval_required_read_only`",
		"run_allowed_now: `true`",
		"approval_mode: `github-issue-dry-run`",
		"approval_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"approval_granted: `false`",
		"repository_mutation_performed: `false`",
		"raw_approval_plan_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_requested_tool_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_approval_payloads_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_tool_approval_plan_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel tool approval plan receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOL_APPROVAL_SOURCE_SECRET", "CHANNEL_TOOL_APPROVAL_INGEST_SECRET", "CHANNEL_TOOL_APPROVAL_TOOL_BODY_SECRET", "Please review this channel-origin", "channel-tool-approval", "channel-tool-approval-thread-123", "inbound-645", "notify-645", "search_files", "GitClaw tools are deterministic"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel tool approval plan receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 645,
			"title": "GitClaw telegram thread channel-tool-approval-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-tool-approval-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64502,
			"body": "@gitclaw /channels approval-plan search_files --id channel-tool-approval --message-id inbound-645 --notify-message-id notify-645\nDo not leak duplicate token CHANNEL_TOOL_APPROVAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel tool approval plan created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[645]); got != 4 {
		t.Fatalf("duplicate channel tool approval plan posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[645])
	}
	duplicateReceipt := github.CommentsByIssue[645][3].Body
	for _, want := range []string{
		"channel_tool_approval_plan_status: `duplicate`",
		"approval_plan_issue: `#101`",
		"approval_plan_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel tool approval plan receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TOOL_APPROVAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate channel tool approval plan receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelToolApprovalPlanActionRequestParsesAlias(t *testing.T) {
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
		Issue:     Issue{Number: 36, Title: "Channel tool approval"},
		Comment: &Comment{
			ID:   3601,
			Body: `@gitclaw /channel tool-gate --tool Read_File --id Channel.Tool.Approval --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelToolApprovalPlanActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelToolApprovalPlanActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-gate" || req.Options.Channel != "slack" || req.Options.ApprovalPlanID != "channel-tool-approval" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel tool approval parsing: %#v", req)
	}
	if req.ApprovalPlan.NormalizedTool != "gitclaw.read_file" || req.ApprovalPlan.SourceKind != "channel_comment" || req.TargetFromIssue || req.AutoApprovalPlanID || req.AutoNotifyMessageID {
		t.Fatalf("unexpected tool approval details: %#v", req)
	}
}
