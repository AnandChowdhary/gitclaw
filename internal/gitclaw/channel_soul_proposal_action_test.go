package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoulProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		".gitclaw/SOUL.md":              "Soul policy.\nCHANNEL_SOUL_PROPOSAL_ACTIVE_SOUL_SECRET\n",
		".gitclaw/IDENTITY.md":          "Identity: GitClaw.\n",
		".gitclaw/USER.md":              "User facts.\n",
		".gitclaw/TOOLS.md":             "Tools.\n",
		".gitclaw/MEMORY.md":            "Memory.\n",
		".gitclaw/HEARTBEAT.md":         "Heartbeat.\n",
		".gitclaw/memory/2026-05-29.md": "Daily note.\n",
	} {
		writeTestFile(t, root, path, body)
	}
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-soul-proposal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 649,
			"title": "GitClaw telegram thread channel-soul-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-soul-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64901,
			"body": "@gitclaw /channels propose-soul --target soul --id warm-tone-soul --message-id inbound-649 --notify-message-id notify-649\nCapture a high-authority tone proposal from this channel.\nCHANNEL_SOUL_PROPOSAL_SOURCE_SECRET",
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
			Number: 649,
			Title:  "GitClaw telegram thread channel-soul-proposal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{649: {{
			ID: 64900,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-soul-proposal-thread-123",
				MessageID: "inbound-649",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_SOUL_PROPOSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{649: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one soul proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposalIssue := github.Issues[1]
	if proposalIssue.Title != "GitClaw soul proposal: warm-tone-soul" {
		t.Fatalf("unexpected soul proposal issue title: %q", proposalIssue.Title)
	}
	for _, want := range []string{
		"gitclaw:soul-proposal-issue",
		`id="warm-tone-soul"`,
		`target_path=".gitclaw/SOUL.md"`,
		"proposal_id: warm-tone-soul",
		"requested_target: soul",
		"target_path: .gitclaw/SOUL.md",
		"target_category: soul",
		"source_issue: #649",
		"source_kind: channel_comment",
		"review_pr_required: true",
		"raw_source_body_included: false",
		"raw_candidate_soul_included: false",
		"raw_existing_soul_included: false",
		"soul_file_written: false",
	} {
		if !strings.Contains(proposalIssue.Body, want) {
			t.Fatalf("soul proposal issue missing %q:\n%s", want, proposalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_PROPOSAL_SOURCE_SECRET", "CHANNEL_SOUL_PROPOSAL_INGEST_SECRET", "CHANNEL_SOUL_PROPOSAL_ACTIVE_SOUL_SECRET", "Capture a high-authority tone proposal"} {
		if strings.Contains(proposalIssue.Body, leaked) {
			t.Fatalf("soul proposal issue leaked %q:\n%s", leaked, proposalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[649]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-649"`,
		"GitClaw channel soul proposal",
		"Proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Proposal id: warm-tone-soul",
		"Target path: .gitclaw/SOUL.md",
		"Target category: soul",
		"Review PR required: true",
		"Soul file written: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel soul proposal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_PROPOSAL_SOURCE_SECRET", "CHANNEL_SOUL_PROPOSAL_INGEST_SECRET", "CHANNEL_SOUL_PROPOSAL_ACTIVE_SOUL_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel soul proposal notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-soul`",
		"channel_soul_proposal_status: `created`",
		"soul_proposal_issue: `#101`",
		"soul_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#649`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"soul_proposal_id_auto: `false`",
		"target_category: `soul`",
		"target_present: `true`",
		"target_required: `true`",
		"target_canonical: `true`",
		"proposal_store: `github-issue-to-git-reviewed-soul-file`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"candidate_soul_generation_allowed: `false`",
		"soul_file_written: `false`",
		"raw_soul_proposal_id_included: `false`",
		"raw_target_path_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_existing_soul_included: `false`",
		"raw_candidate_soul_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_soul_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_PROPOSAL_SOURCE_SECRET", "CHANNEL_SOUL_PROPOSAL_INGEST_SECRET", "CHANNEL_SOUL_PROPOSAL_ACTIVE_SOUL_SECRET", "Capture a high-authority tone proposal", "warm-tone-soul", "channel-soul-proposal-thread-123", "inbound-649", "notify-649", ".gitclaw/SOUL.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 649,
			"title": "GitClaw telegram thread channel-soul-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-soul-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64902,
			"body": "@gitclaw /channels propose-soul --target soul --id warm-tone-soul --message-id inbound-649 --notify-message-id notify-649\nDo not leak duplicate token CHANNEL_SOUL_PROPOSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel soul proposal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[649]); got != 4 {
		t.Fatalf("duplicate channel soul proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[649])
	}
	duplicateReceipt := github.CommentsByIssue[649][3].Body
	for _, want := range []string{
		"channel_soul_proposal_status: `duplicate`",
		"soul_proposal_issue: `#101`",
		"soul_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel soul proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_PROPOSAL_DUPLICATE_SECRET", "warm-tone-soul", "channel-soul-proposal-thread-123", "inbound-649", "notify-649"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel soul proposal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSoulProposalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		".gitclaw/SOUL.md":              "Soul policy.\n",
		".gitclaw/IDENTITY.md":          "Identity: GitClaw.\n",
		".gitclaw/USER.md":              "User facts.\n",
		".gitclaw/TOOLS.md":             "Tools.\n",
		".gitclaw/MEMORY.md":            "Memory.\n",
		".gitclaw/HEARTBEAT.md":         "Heartbeat.\n",
		".gitclaw/memory/2026-05-29.md": "Daily note.\n",
	} {
		writeTestFile(t, root, path, body)
	}
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
		Issue:     Issue{Number: 38, Title: "Channel soul proposal"},
		Comment: &Comment{
			ID:   3801,
			Body: `@gitclaw /channel soul-proposal --target user --id User-Voice --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelSoulProposalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSoulProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "soul-proposal" || req.Options.Channel != "slack" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel soul proposal parsing: %#v", req)
	}
	if req.Proposal.ProposalID != "user-voice" || req.Proposal.SourceKind != "channel_comment" || req.Proposal.TargetPath != ".gitclaw/USER.md" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoProposalID {
		t.Fatalf("unexpected channel soul proposal details: %#v", req)
	}
}
