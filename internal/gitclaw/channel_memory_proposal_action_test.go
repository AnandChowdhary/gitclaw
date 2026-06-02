package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelMemoryProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Stable memory file.\nCHANNEL_MEMORY_PROPOSAL_ACTIVE_MEMORY_SECRET\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily memory file with CHANNEL_MEMORY_PROPOSAL_DAILY_SECRET.\n")
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-memory-proposal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 651,
			"title": "GitClaw telegram thread channel-memory-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-memory-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 65101,
			"body": "@gitclaw /channels propose-memory --target long-term --id weekly-ops-memory --message-id inbound-651 --notify-message-id notify-651\nCapture a durable channel memory proposal.\nCHANNEL_MEMORY_PROPOSAL_SOURCE_SECRET",
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
			Number: 651,
			Title:  "GitClaw telegram thread channel-memory-proposal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{651: {{
			ID: 65100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-memory-proposal-thread-123",
				MessageID: "inbound-651",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_MEMORY_PROPOSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{651: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel memory proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one memory proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposalIssue := github.Issues[1]
	if proposalIssue.Title != "GitClaw memory proposal: weekly-ops-memory" {
		t.Fatalf("unexpected memory proposal issue title: %q", proposalIssue.Title)
	}
	for _, want := range []string{
		"gitclaw:memory-proposal-issue",
		`id="weekly-ops-memory"`,
		`target_kind="long-term"`,
		"proposal_id: weekly-ops-memory",
		"target_kind: long-term",
		"target_path: .gitclaw/MEMORY.md",
		"source_issue: #651",
		"source_kind: channel_comment",
		"review_pr_required: true",
		"raw_source_body_included: false",
		"raw_candidate_memory_included: false",
		"raw_existing_memory_included: false",
		"memory_file_written: false",
	} {
		if !strings.Contains(proposalIssue.Body, want) {
			t.Fatalf("memory proposal issue missing %q:\n%s", want, proposalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_PROPOSAL_SOURCE_SECRET", "CHANNEL_MEMORY_PROPOSAL_INGEST_SECRET", "CHANNEL_MEMORY_PROPOSAL_ACTIVE_MEMORY_SECRET", "CHANNEL_MEMORY_PROPOSAL_DAILY_SECRET", "Capture a durable channel memory proposal"} {
		if strings.Contains(proposalIssue.Body, leaked) {
			t.Fatalf("memory proposal issue leaked %q:\n%s", leaked, proposalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[651]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-651"`,
		"GitClaw channel memory proposal",
		"Proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Proposal id: weekly-ops-memory",
		"Target kind: long-term",
		"Target path: .gitclaw/MEMORY.md",
		"Memory validation: ok",
		"Review PR required: true",
		"Memory file written: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel memory proposal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_PROPOSAL_SOURCE_SECRET", "CHANNEL_MEMORY_PROPOSAL_INGEST_SECRET", "CHANNEL_MEMORY_PROPOSAL_ACTIVE_MEMORY_SECRET", "CHANNEL_MEMORY_PROPOSAL_DAILY_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel memory proposal notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Memory Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-memory`",
		"channel_memory_proposal_status: `created`",
		"memory_proposal_issue: `#101`",
		"memory_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#651`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"memory_proposal_id_auto: `false`",
		"normalized_target_kind: `long-term`",
		"target_present: `true`",
		"proposal_store: `github-issue-to-git-reviewed-memory-file`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"candidate_memory_generation_allowed: `false`",
		"memory_file_written: `false`",
		"raw_memory_proposal_id_included: `false`",
		"raw_target_path_included: `false`",
		"raw_latest_memory_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_existing_memory_included: `false`",
		"raw_candidate_memory_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_memory_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel memory proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_PROPOSAL_SOURCE_SECRET", "CHANNEL_MEMORY_PROPOSAL_INGEST_SECRET", "CHANNEL_MEMORY_PROPOSAL_ACTIVE_MEMORY_SECRET", "CHANNEL_MEMORY_PROPOSAL_DAILY_SECRET", "Capture a durable channel memory proposal", "weekly-ops-memory", "channel-memory-proposal-thread-123", "inbound-651", "notify-651", ".gitclaw/MEMORY.md", ".gitclaw/memory/2026-05-29.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel memory proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 651,
			"title": "GitClaw telegram thread channel-memory-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-memory-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 65102,
			"body": "@gitclaw /channels propose-memory --target long-term --id weekly-ops-memory --message-id inbound-651 --notify-message-id notify-651\nDo not leak duplicate token CHANNEL_MEMORY_PROPOSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel memory proposal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[651]); got != 4 {
		t.Fatalf("duplicate channel memory proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[651])
	}
	duplicateReceipt := github.CommentsByIssue[651][3].Body
	for _, want := range []string{
		"channel_memory_proposal_status: `duplicate`",
		"memory_proposal_issue: `#101`",
		"memory_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel memory proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_MEMORY_PROPOSAL_DUPLICATE_SECRET", "weekly-ops-memory", "channel-memory-proposal-thread-123", "inbound-651", "notify-651"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel memory proposal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelMemoryProposalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "Stable memory file.\n")
	writeTestFile(t, root, ".gitclaw/memory/2026-05-29.md", "Daily memory file.\n")
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
		Issue:     Issue{Number: 39, Title: "Channel memory proposal"},
		Comment: &Comment{
			ID:   3901,
			Body: `@gitclaw /channel memory-proposal --target daily-note --id Daily-Memory --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelMemoryProposalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelMemoryProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "memory-proposal" || req.Options.Channel != "slack" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel memory proposal parsing: %#v", req)
	}
	if req.Proposal.ProposalID != "daily-memory" || req.Proposal.SourceKind != "channel_comment" || req.Proposal.Target.Kind != "dated-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoProposalID {
		t.Fatalf("unexpected channel memory proposal details: %#v", req)
	}
}
