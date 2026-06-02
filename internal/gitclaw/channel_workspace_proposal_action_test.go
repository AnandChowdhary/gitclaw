package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelWorkspaceProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-workspace-proposal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread channel-workspace-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-workspace-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels propose-workspace --workspace-id channel-workspace-review --target .gitclaw/workspaces/channel-review.md --message-id inbound-384 --notify-message-id notify-384\nTitle: Channel review workspace\nProposal:\nVisible proposal token CHANNEL_WORKSPACE_PROPOSAL_BODY_SECRET.\nRationale:\nVisible rationale token CHANNEL_WORKSPACE_PROPOSAL_RATIONALE_SECRET.",
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
			Title:  "GitClaw telegram thread channel-workspace-proposal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-workspace-proposal-thread-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_WORKSPACE_PROPOSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel workspace proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one workspace proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposal := github.Issues[1]
	if !HasChannelWorkspaceProposalMarker(proposal.Body) || !strings.Contains(proposal.Body, `workspace_proposal_id="channel-workspace-review"`) {
		t.Fatalf("workspace proposal issue missing channel-workspace-proposal marker:\n%s", proposal.Body)
	}
	for _, want := range []string{
		"GitClaw channel workspace proposal",
		"workspace_proposal_id: channel-workspace-review",
		"target_path: .gitclaw/workspaces/channel-review.md",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"workspace_proposal_mode: github-issue-workspace-proposal",
		"review_pr_required: true",
		"workspace_file_written: false",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Channel review workspace",
		"## Target Path",
		".gitclaw/workspaces/channel-review.md",
		"## Proposal",
		"Visible proposal token CHANNEL_WORKSPACE_PROPOSAL_BODY_SECRET.",
		"## Rationale",
		"Visible rationale token CHANNEL_WORKSPACE_PROPOSAL_RATIONALE_SECRET.",
	} {
		if !strings.Contains(proposal.Body, want) {
			t.Fatalf("workspace proposal issue missing %q:\n%s", want, proposal.Body)
		}
	}
	if strings.Contains(proposal.Body, "channel-workspace-proposal-thread-123") || strings.Contains(proposal.Body, "inbound-384") || strings.Contains(proposal.Body, "CHANNEL_WORKSPACE_PROPOSAL_INGEST_SECRET") {
		t.Fatalf("workspace proposal issue leaked provider IDs or channel body:\n%s", proposal.Body)
	}
	if !hasLabel(github.IssueLabels[proposal.Number], "gitclaw") {
		t.Fatalf("workspace proposal issue missing gitclaw trigger label: %#v", github.IssueLabels[proposal.Number])
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
		"GitClaw channel workspace proposal recorded.",
		"Workspace proposal: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Channel review workspace",
		"Target path: .gitclaw/workspaces/channel-review.md",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("workspace proposal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_WORKSPACE_PROPOSAL_BODY_SECRET", "CHANNEL_WORKSPACE_PROPOSAL_RATIONALE_SECRET", "CHANNEL_WORKSPACE_PROPOSAL_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("workspace proposal notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Workspace Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-workspace`",
		"channel_workspace_proposal_status: `recorded`",
		"workspace_proposal_issue: `#101`",
		"workspace_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"workspace_proposal_mode: `github-issue-workspace-proposal`",
		"review_pr_required: `true`",
		"workspace_file_written: `false`",
		"repository_mutation_performed: `false`",
		"raw_workspace_proposal_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_workspace_proposal_title_included: `false`",
		"raw_workspace_proposal_target_path_included: `false`",
		"raw_workspace_proposal_proposal_included: `false`",
		"raw_workspace_proposal_rationale_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_workspace_proposal_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel workspace proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WORKSPACE_PROPOSAL_INGEST_SECRET", "CHANNEL_WORKSPACE_PROPOSAL_BODY_SECRET", "CHANNEL_WORKSPACE_PROPOSAL_RATIONALE_SECRET", "Channel review workspace", "channel-workspace-review", "channel-workspace-proposal-thread-123", "inbound-384", "notify-384", ".gitclaw/workspaces/channel-review.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel workspace proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread channel-workspace-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-workspace-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels propose-workspace --workspace-id channel-workspace-review --target .gitclaw/workspaces/channel-review.md --message-id inbound-384 --notify-message-id notify-384\nTitle: Channel review workspace\nProposal:\nDo not leak duplicate token CHANNEL_WORKSPACE_PROPOSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate workspace proposal created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate workspace proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_workspace_proposal_status: `duplicate`",
		"workspace_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate workspace proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_WORKSPACE_PROPOSAL_DUPLICATE_SECRET") {
		t.Fatalf("duplicate workspace proposal receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelWorkspaceProposalActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel workspace proposal"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel context-proposal --route team-demo --proposal-id Workspace.Proposal --path .gitclaw/workspaces/team-demo.md --message-id source-1 --notify-message-id notify-1
Workspace: The channel needs a review workspace
Proposal:
- Load repo docs.
Rationale:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelWorkspaceProposalActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelWorkspaceProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "context-proposal" || req.Options.Route != "team-demo" || req.Options.WorkspaceProposalID != "workspace-proposal" || req.Options.TargetPath != ".gitclaw/workspaces/team-demo.md" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel workspace proposal parsing: %#v", req)
	}
	if req.Options.Title != "The channel needs a review workspace" || !strings.Contains(req.Options.Proposal, "Load repo docs") || !strings.Contains(req.Options.Rationale, "Follow-up moves") {
		t.Fatalf("unexpected workspace proposal sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoWorkspaceProposalID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.TargetPathSHA == "" || req.ProposalSHA == "" || req.RationaleSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route workspace proposal hashes: %#v", req)
	}
}

func TestIsChannelWorkspaceProposalActionFieldsKeepsOtherAliasesSeparate(t *testing.T) {
	if isChannelWorkspaceProposalActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a workspace proposal alias")
	}
	if isChannelWorkspaceProposalActionFields([]string{"/channels", "takeaway"}) {
		t.Fatalf("takeaway should remain an insight alias, not a workspace proposal alias")
	}
	if !isChannelWorkspaceProposalActionFields([]string{"/channels", "propose-workspace"}) {
		t.Fatalf("propose-workspace should be accepted as a workspace proposal alias")
	}
	if !isChannelWorkspaceProposalActionFields([]string{"/channels", "workspace-proposal"}) {
		t.Fatalf("workspace-proposal should be accepted as a workspace proposal alias")
	}
}
