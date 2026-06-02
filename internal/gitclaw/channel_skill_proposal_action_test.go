package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSkillProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", `---
name: repo-reader
description: Read repository files.
---

Use repo files.
CHANNEL_SKILL_PROPOSAL_ACTIVE_SKILL_SECRET
`)
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-skill-proposal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 647,
			"title": "GitClaw telegram thread channel-skill-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-skill-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64701,
			"body": "@gitclaw /channels propose-skill weekly-review --message-id inbound-647 --notify-message-id notify-647\nCapture a reusable weekly review workflow from this channel.\nCHANNEL_SKILL_PROPOSAL_SOURCE_SECRET",
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
			Number: 647,
			Title:  "GitClaw telegram thread channel-skill-proposal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{647: {{
			ID: 64700,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-skill-proposal-thread-123",
				MessageID: "inbound-647",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_SKILL_PROPOSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{647: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel skill proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one skill proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposalIssue := github.Issues[1]
	if proposalIssue.Title != "GitClaw skill proposal: weekly-review" {
		t.Fatalf("unexpected skill proposal issue title: %q", proposalIssue.Title)
	}
	for _, want := range []string{
		"gitclaw:skill-proposal-issue",
		`name="weekly-review"`,
		"proposal_name: weekly-review",
		"planned_action: propose-create",
		"requested_action: auto",
		"proposal_path: .gitclaw/skill-proposals/weekly-review/PROPOSAL.md",
		"destination_path: .gitclaw/SKILLS/weekly-review/SKILL.md",
		"source_issue: #647",
		"source_kind: channel_comment",
		"existing_skill_matches: 0",
		"available_skills: 1",
		"review_pr_required: true",
		"raw_source_body_included: false",
		"raw_proposal_body_included: false",
		"active_skill_write_performed: false",
	} {
		if !strings.Contains(proposalIssue.Body, want) {
			t.Fatalf("skill proposal issue missing %q:\n%s", want, proposalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_PROPOSAL_SOURCE_SECRET", "CHANNEL_SKILL_PROPOSAL_INGEST_SECRET", "CHANNEL_SKILL_PROPOSAL_ACTIVE_SKILL_SECRET", "Capture a reusable weekly review"} {
		if strings.Contains(proposalIssue.Body, leaked) {
			t.Fatalf("skill proposal issue leaked %q:\n%s", leaked, proposalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[647]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-647"`,
		"GitClaw channel skill proposal",
		"Proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Proposal name: weekly-review",
		"Planned action: propose-create",
		"Proposal path: .gitclaw/skill-proposals/weekly-review/PROPOSAL.md",
		"Destination path: .gitclaw/SKILLS/weekly-review/SKILL.md",
		"Review PR required: true",
		"Active skill written: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel skill proposal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_PROPOSAL_SOURCE_SECRET", "CHANNEL_SKILL_PROPOSAL_INGEST_SECRET", "CHANNEL_SKILL_PROPOSAL_ACTIVE_SKILL_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel skill proposal notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Skill Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-skill`",
		"channel_skill_proposal_status: `created`",
		"proposal_issue: `#101`",
		"proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#647`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"requested_action: `auto`",
		"planned_proposal_action: `propose-create`",
		"target_type: `registry-name`",
		"existing_skill_matches: `0`",
		"available_skills: `1`",
		"proposal_store: `github-issue-to-git-reviewed-proposal-file`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"skill_body_generated: `false`",
		"proposal_file_written: `false`",
		"active_skill_write_performed: `false`",
		"raw_proposal_name_included: `false`",
		"raw_proposal_path_included: `false`",
		"raw_destination_path_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_skill_body_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_skill_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel skill proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_PROPOSAL_SOURCE_SECRET", "CHANNEL_SKILL_PROPOSAL_INGEST_SECRET", "CHANNEL_SKILL_PROPOSAL_ACTIVE_SKILL_SECRET", "Capture a reusable weekly review", "weekly-review", "channel-skill-proposal-thread-123", "inbound-647", "notify-647", ".gitclaw/skill-proposals/weekly-review", ".gitclaw/SKILLS/weekly-review"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel skill proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 647,
			"title": "GitClaw telegram thread channel-skill-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-skill-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64702,
			"body": "@gitclaw /channels propose-skill weekly-review --message-id inbound-647 --notify-message-id notify-647\nDo not leak duplicate token CHANNEL_SKILL_PROPOSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel skill proposal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[647]); got != 4 {
		t.Fatalf("duplicate channel skill proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[647])
	}
	duplicateReceipt := github.CommentsByIssue[647][3].Body
	for _, want := range []string{
		"channel_skill_proposal_status: `duplicate`",
		"proposal_issue: `#101`",
		"proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel skill proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SKILL_PROPOSAL_DUPLICATE_SECRET", "weekly-review", "channel-skill-proposal-thread-123", "inbound-647", "notify-647"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel skill proposal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSkillProposalActionRequestParsesAlias(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, ".gitclaw/SKILLS/repo-reader/SKILL.md", "Use repo files.\n")
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
		Issue:     Issue{Number: 37, Title: "Channel skill proposal"},
		Comment: &Comment{
			ID:   3701,
			Body: `@gitclaw /channel skill-proposal --skill Repo.Reader --action update --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelSkillProposalActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSkillProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "skill-proposal" || req.Options.Channel != "slack" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel skill proposal parsing: %#v", req)
	}
	if req.Proposal.Target.Candidate != "repo-reader" || req.Proposal.SourceKind != "channel_comment" || req.Proposal.RequestedAction != "propose-update" || req.Proposal.PlannedAction != "propose-update" || req.TargetFromIssue || req.AutoNotifyMessageID {
		t.Fatalf("unexpected channel skill proposal details: %#v", req)
	}
}
