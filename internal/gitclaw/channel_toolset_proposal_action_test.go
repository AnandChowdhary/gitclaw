package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelToolsetProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-toolset-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 704,
			"title": "GitClaw telegram thread chat-toolset-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-toolset-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 70401,
			"body": "@gitclaw /channels propose-toolset --toolset-id research-tools --message-id inbound-704 --notify-message-id notify-704\nName: Research channel toolset\nPurpose: Review a channel-origin bundle with CHANNEL_TOOLSET_PURPOSE_SECRET.\nTools:\n- gitclaw.search_files\n- gitclaw.list_files\nPolicy:\nRead-only only with CHANNEL_TOOLSET_POLICY_SECRET.\nNotes:\nVisible toolset note with CHANNEL_TOOLSET_NOTE_SECRET.",
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
			Number: 704,
			Title:  "GitClaw telegram thread chat-toolset-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{704: {{
			ID: 70400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-toolset-123",
				MessageID: "inbound-704",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_TOOLSET_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{704: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel toolset proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one toolset proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposal := github.Issues[1]
	for _, want := range []string{
		"gitclaw:channel-toolset-proposal",
		`toolset_id="research-tools"`,
		"GitClaw channel toolset proposal",
		"toolset_id: research-tools",
		"toolset_name: Research channel toolset",
		"tool_count: 2",
		"source_channel: telegram",
		"source_issue: #704",
		"source_message_id_sha256_12:",
		"proposal_store: github-issue-to-git-reviewed-toolset",
		"review_pr_required: true",
		"toolset_enabled: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Purpose",
		"CHANNEL_TOOLSET_PURPOSE_SECRET",
		"## Proposed Tools",
		"- gitclaw.search_files",
		"- gitclaw.list_files",
		"## Policy",
		"CHANNEL_TOOLSET_POLICY_SECRET",
		"## Notes",
		"CHANNEL_TOOLSET_NOTE_SECRET",
	} {
		if !strings.Contains(proposal.Body, want) {
			t.Fatalf("toolset proposal issue missing %q:\n%s", want, proposal.Body)
		}
	}
	if strings.Contains(proposal.Body, "chat-toolset-123") || strings.Contains(proposal.Body, "inbound-704") || strings.Contains(proposal.Body, "CHANNEL_TOOLSET_INGEST_SECRET") {
		t.Fatalf("toolset proposal issue leaked provider IDs or channel body:\n%s", proposal.Body)
	}
	if !hasLabel(github.IssueLabels[proposal.Number], "gitclaw") {
		t.Fatalf("toolset proposal issue missing gitclaw trigger label: %#v", github.IssueLabels[proposal.Number])
	}

	sourceComments := github.CommentsByIssue[704]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-704"`,
		"GitClaw channel toolset proposal captured.",
		"Toolset proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Name: Research channel toolset",
		"Tools: 2",
		"Review PR required: true",
		"Toolset enabled: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("toolset proposal notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_TOOLSET_PURPOSE_SECRET") || strings.Contains(outbound, "CHANNEL_TOOLSET_POLICY_SECRET") || strings.Contains(outbound, "CHANNEL_TOOLSET_NOTE_SECRET") || strings.Contains(outbound, "gitclaw.search_files") || strings.Contains(outbound, "CHANNEL_TOOLSET_INGEST_SECRET") {
		t.Fatalf("toolset proposal notification leaked proposal body or channel body:\n%s", outbound)
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Toolset Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-toolset`",
		"channel_toolset_proposal_status: `created`",
		"toolset_proposal_issue: `#101`",
		"toolset_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#704`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"tool_count: `2`",
		"proposal_store: `github-issue-to-git-reviewed-toolset`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"toolset_enabled: `false`",
		"tool_execution_performed: `false`",
		"active_tool_config_write_performed: `false`",
		"repository_mutation_performed: `false`",
		"raw_toolset_id_included: `false`",
		"raw_toolset_name_included: `false`",
		"raw_toolset_purpose_included: `false`",
		"raw_toolset_tools_included: `false`",
		"raw_toolset_policy_included: `false`",
		"raw_toolset_notes_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_toolset_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel toolset proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_TOOLSET_INGEST_SECRET", "CHANNEL_TOOLSET_PURPOSE_SECRET", "CHANNEL_TOOLSET_POLICY_SECRET", "CHANNEL_TOOLSET_NOTE_SECRET", "Research channel toolset", "research-tools", "gitclaw.search_files", "chat-toolset-123", "inbound-704", "notify-704"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel toolset proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 704,
			"title": "GitClaw telegram thread chat-toolset-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-toolset-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 70402,
			"body": "@gitclaw /channels propose-toolset --toolset-id research-tools --message-id inbound-704 --notify-message-id notify-704\nName: Research channel toolset\nTools:\n- Do not leak duplicate token CHANNEL_TOOLSET_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate toolset proposal created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[704]); got != 4 {
		t.Fatalf("duplicate toolset proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[704])
	}
	duplicateReceipt := github.CommentsByIssue[704][3].Body
	for _, want := range []string{
		"channel_toolset_proposal_status: `duplicate`",
		"toolset_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate toolset proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_TOOLSET_DUPLICATE_SECRET") {
		t.Fatalf("duplicate toolset proposal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelToolsetProposalActionRequestParsesAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel toolset"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel tool-bundle --channel slack --thread-id thread-1 --id research.bundle --message-id source-1 --notify-message-id notify-1
Name: GitHub research bundle
Purpose:
Make channel-origin research reusable without enabling tools automatically.
Capabilities:
1. gitclaw.search_files
2. gitclaw.list_files
Policy:
Read-only and PR-reviewed.
Notes:
Inspired by explicit toolsets.`,
		},
	}
	req, err := BuildChannelToolsetProposalActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelToolsetProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "tool-bundle" || req.Options.Channel != "slack" || req.Options.ThreadID != "thread-1" || req.Options.ToolsetID != "research-bundle" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel toolset parsing: %#v", req)
	}
	if req.Options.Name != "GitHub research bundle" || !strings.Contains(req.Options.Purpose, "reusable") || !strings.Contains(req.Options.Tools, "gitclaw.search_files") || !strings.Contains(req.Options.Policy, "Read-only") || !strings.Contains(req.Options.Notes, "Inspired") {
		t.Fatalf("unexpected toolset sections: %#v", req)
	}
	if req.ToolCount != 2 || req.TargetFromIssue || req.AutoToolsetID || req.AutoNotifyMessageID || req.NameSHA == "" || req.ToolsSHA == "" || req.PolicySHA == "" {
		t.Fatalf("expected explicit channel toolset hashes/counts: %#v", req)
	}
}

func TestIsChannelToolsetProposalActionFieldsKeepsAliasesSeparate(t *testing.T) {
	if isChannelToolsetProposalActionFields([]string{"/channels", "request-run"}) {
		t.Fatalf("request-run should remain a tool run request alias")
	}
	if isChannelToolsetProposalActionFields([]string{"/channels", "approval-plan"}) {
		t.Fatalf("approval-plan should remain a tool approval plan alias")
	}
	if !isChannelToolsetProposalActionFields([]string{"/channels", "propose-toolset"}) {
		t.Fatalf("propose-toolset should be accepted")
	}
	if !isChannelToolsetProposalActionFields([]string{"/channels", "tool-bundle"}) {
		t.Fatalf("tool-bundle should be accepted")
	}
}
