package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBundleProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-bundle-proposal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 746,
			"title": "GitClaw telegram thread channel-bundle-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-bundle-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 74601,
			"body": "@gitclaw /channels propose-bundle --bundle-id weekly-review-bundle --message-id inbound-746 --notify-message-id notify-746\nName: Weekly review bundle\nPurpose:\nReview a channel-origin bundle with CHANNEL_BUNDLE_PROPOSAL_PURPOSE_TOKEN.\nSkills:\n- repo-reader\n- self-grill\nInstruction:\nUse the selected skills together and mention CHANNEL_BUNDLE_PROPOSAL_INSTRUCTION_TOKEN only in the proposal issue.\nPolicy:\nRead-only review with CHANNEL_BUNDLE_PROPOSAL_POLICY_TOKEN.\nNotes:\nVisible note CHANNEL_BUNDLE_PROPOSAL_NOTES_TOKEN.",
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
			Number: 746,
			Title:  "GitClaw telegram thread channel-bundle-proposal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{746: {{
			ID: 74600,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-bundle-proposal-thread-123",
				MessageID: "inbound-746",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BUNDLE_PROPOSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{746: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel bundle proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one bundle proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposalIssue := github.Issues[1]
	if proposalIssue.Title != "GitClaw channel bundle proposal: Weekly review bundle" {
		t.Fatalf("unexpected bundle proposal issue title: %q", proposalIssue.Title)
	}
	for _, want := range []string{
		"gitclaw:channel-bundle-proposal",
		`bundle_id="weekly-review-bundle"`,
		"bundle_id: weekly-review-bundle",
		"bundle_name: Weekly review bundle",
		"skill_count: 2",
		"source_channel: telegram",
		"source_issue: #746",
		"proposal_store: github-issue-to-git-reviewed-skill-bundle",
		"review_pr_required: true",
		"bundle_enabled: false",
		"skill_install_performed: false",
		"bundle_yaml_write_performed: false",
		"repository_mutation_performed: false",
		"raw_source_message_id_included: false",
		"## Purpose",
		"CHANNEL_BUNDLE_PROPOSAL_PURPOSE_TOKEN",
		"## Skills",
		"repo-reader",
		"self-grill",
		"## Bundle Instruction",
		"CHANNEL_BUNDLE_PROPOSAL_INSTRUCTION_TOKEN",
		"## Policy",
		"CHANNEL_BUNDLE_PROPOSAL_POLICY_TOKEN",
		"## Notes",
		"CHANNEL_BUNDLE_PROPOSAL_NOTES_TOKEN",
	} {
		if !strings.Contains(proposalIssue.Body, want) {
			t.Fatalf("bundle proposal issue missing %q:\n%s", want, proposalIssue.Body)
		}
	}
	for _, leaked := range []string{"channel-bundle-proposal-thread-123", "inbound-746", "CHANNEL_BUNDLE_PROPOSAL_INGEST_SECRET"} {
		if strings.Contains(proposalIssue.Body, leaked) {
			t.Fatalf("bundle proposal issue leaked %q:\n%s", leaked, proposalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[746]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-746"`,
		"GitClaw channel skill bundle proposal captured.",
		"Bundle proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Name: Weekly review bundle",
		"Skills: 2",
		"Review PR required: true",
		"Bundle enabled: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel bundle proposal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_BUNDLE_PROPOSAL_SOURCE_SECRET", "CHANNEL_BUNDLE_PROPOSAL_INGEST_SECRET", "CHANNEL_BUNDLE_PROPOSAL_PURPOSE_TOKEN", "CHANNEL_BUNDLE_PROPOSAL_INSTRUCTION_TOKEN", "CHANNEL_BUNDLE_PROPOSAL_POLICY_TOKEN", "CHANNEL_BUNDLE_PROPOSAL_NOTES_TOKEN", "repo-reader", "self-grill"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel bundle proposal notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Bundle Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-bundle`",
		"channel_bundle_proposal_status: `created`",
		"bundle_proposal_issue: `#101`",
		"bundle_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#746`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"proposal_store: `github-issue-to-git-reviewed-skill-bundle`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"bundle_enabled: `false`",
		"skill_install_performed: `false`",
		"bundle_yaml_write_performed: `false`",
		"repository_mutation_performed: `false`",
		"raw_bundle_id_included: `false`",
		"raw_bundle_name_included: `false`",
		"raw_bundle_purpose_included: `false`",
		"raw_bundle_skills_included: `false`",
		"raw_bundle_instruction_included: `false`",
		"raw_bundle_policy_included: `false`",
		"raw_bundle_notes_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_bundle_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel bundle proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BUNDLE_PROPOSAL_SOURCE_SECRET", "CHANNEL_BUNDLE_PROPOSAL_INGEST_SECRET", "CHANNEL_BUNDLE_PROPOSAL_PURPOSE_TOKEN", "CHANNEL_BUNDLE_PROPOSAL_INSTRUCTION_TOKEN", "CHANNEL_BUNDLE_PROPOSAL_POLICY_TOKEN", "CHANNEL_BUNDLE_PROPOSAL_NOTES_TOKEN", "weekly-review-bundle", "Weekly review bundle", "repo-reader", "self-grill", "channel-bundle-proposal-thread-123", "inbound-746", "notify-746"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel bundle proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 746,
			"title": "GitClaw telegram thread channel-bundle-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-bundle-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 74602,
			"body": "@gitclaw /channels skill-bundle-proposal --bundle-id weekly-review-bundle --message-id inbound-746 --notify-message-id notify-746\nName: Weekly review bundle\nSkills:\n- repo-reader\nInstruction:\nDo not leak duplicate token CHANNEL_BUNDLE_PROPOSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel bundle proposal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[746]); got != 4 {
		t.Fatalf("duplicate channel bundle proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[746])
	}
	duplicateReceipt := github.CommentsByIssue[746][3].Body
	for _, want := range []string{
		"channel_bundle_proposal_status: `duplicate`",
		"bundle_proposal_issue: `#101`",
		"bundle_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"raw_bundle_instruction_included: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel bundle proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BUNDLE_PROPOSAL_DUPLICATE_SECRET", "weekly-review-bundle", "repo-reader", "channel-bundle-proposal-thread-123", "inbound-746", "notify-746"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel bundle proposal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelBundleProposalActionRequestParsesAlias(t *testing.T) {
	cfg := DefaultConfig()
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 37,
			Title:  "Channel bundle proposal",
			Body:   `<!-- gitclaw:channel-thread channel="slack" thread_id="thread-from-issue" -->`,
		},
		Comment: &Comment{
			ID: 3701,
			Body: `@gitclaw /channel bundle --id Repo.Context.Bundle --message-id source-1 --notify-message-id notify-1
Name: Repo Context Bundle
Purpose: Review it.
Skills:
- repo-reader
- self-grill
Instruction:
Use both skills before producing the plan.`,
		},
	}
	req, err := BuildChannelBundleProposalActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelBundleProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "bundle" || req.Options.Channel != "slack" || req.Options.ThreadID != "thread-from-issue" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel bundle proposal parsing: %#v", req)
	}
	if req.Options.BundleID != "repo-context-bundle" || req.Options.Name != "Repo Context Bundle" || !strings.Contains(req.Options.Instruction, "Use both skills") || req.SkillCount != 2 {
		t.Fatalf("unexpected bundle proposal target fields: %#v", req.Options)
	}
	if !req.TargetFromIssue {
		t.Fatalf("expected bundle proposal target to come from channel issue")
	}
}
