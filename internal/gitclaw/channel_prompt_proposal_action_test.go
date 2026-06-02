package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPromptProposalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	root := t.TempDir()
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-prompt-proposal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 732,
			"title": "GitClaw telegram thread channel-prompt-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-prompt-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 73201,
			"body": "@gitclaw /channels propose-prompt --prompt-id weekly-context-brief --message-id inbound-732 --notify-message-id notify-732\nName: Weekly context brief\nPurpose:\nCapture a reusable prompt proposal with CHANNEL_PROMPT_PROPOSAL_PURPOSE_TOKEN.\nPrompt:\nSummarize the current repo context and mention CHANNEL_PROMPT_PROPOSAL_DRAFT_TOKEN.\nInputs:\n- repository state CHANNEL_PROMPT_PROPOSAL_INPUT_TOKEN\nPolicy:\nRead-only only with CHANNEL_PROMPT_PROPOSAL_POLICY_TOKEN.\nNotes:\nVisible note CHANNEL_PROMPT_PROPOSAL_NOTES_TOKEN.",
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
			Number: 732,
			Title:  "GitClaw telegram thread channel-prompt-proposal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{732: {{
			ID: 73200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-prompt-proposal-thread-123",
				MessageID: "inbound-732",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_PROMPT_PROPOSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{732: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel prompt proposal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one prompt proposal issue: %#v", len(github.Issues), github.Issues)
	}
	proposalIssue := github.Issues[1]
	if proposalIssue.Title != "GitClaw channel prompt proposal: Weekly context brief" {
		t.Fatalf("unexpected prompt proposal issue title: %q", proposalIssue.Title)
	}
	for _, want := range []string{
		"gitclaw:channel-prompt-proposal",
		`prompt_id="weekly-context-brief"`,
		"prompt_id: weekly-context-brief",
		"prompt_name: Weekly context brief",
		"source_channel: telegram",
		"source_issue: #732",
		"proposal_store: github-issue-to-git-reviewed-prompt-pack",
		"review_pr_required: true",
		"prompt_enabled: false",
		"prompt_pack_write_performed: false",
		"repository_mutation_performed: false",
		"raw_source_message_id_included: false",
		"## Purpose",
		"CHANNEL_PROMPT_PROPOSAL_PURPOSE_TOKEN",
		"## Prompt Draft",
		"CHANNEL_PROMPT_PROPOSAL_DRAFT_TOKEN",
		"## Inputs",
		"CHANNEL_PROMPT_PROPOSAL_INPUT_TOKEN",
		"## Policy",
		"CHANNEL_PROMPT_PROPOSAL_POLICY_TOKEN",
		"## Notes",
		"CHANNEL_PROMPT_PROPOSAL_NOTES_TOKEN",
	} {
		if !strings.Contains(proposalIssue.Body, want) {
			t.Fatalf("prompt proposal issue missing %q:\n%s", want, proposalIssue.Body)
		}
	}
	for _, leaked := range []string{"channel-prompt-proposal-thread-123", "inbound-732", "CHANNEL_PROMPT_PROPOSAL_INGEST_SECRET"} {
		if strings.Contains(proposalIssue.Body, leaked) {
			t.Fatalf("prompt proposal issue leaked %q:\n%s", leaked, proposalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[732]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-732"`,
		"GitClaw channel prompt proposal captured.",
		"Prompt proposal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Name: Weekly context brief",
		"Review PR required: true",
		"Prompt enabled: false",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel prompt proposal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROMPT_PROPOSAL_SOURCE_SECRET", "CHANNEL_PROMPT_PROPOSAL_INGEST_SECRET", "CHANNEL_PROMPT_PROPOSAL_PURPOSE_TOKEN", "CHANNEL_PROMPT_PROPOSAL_DRAFT_TOKEN", "CHANNEL_PROMPT_PROPOSAL_INPUT_TOKEN", "CHANNEL_PROMPT_PROPOSAL_POLICY_TOKEN", "CHANNEL_PROMPT_PROPOSAL_NOTES_TOKEN"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel prompt proposal notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Prompt Proposal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels propose-prompt`",
		"channel_prompt_proposal_status: `created`",
		"prompt_proposal_issue: `#101`",
		"prompt_proposal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#732`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"proposal_store: `github-issue-to-git-reviewed-prompt-pack`",
		"review_pr_required: `true`",
		"model_call_performed: `false`",
		"prompt_enabled: `false`",
		"prompt_test_run_performed: `false`",
		"prompt_pack_write_performed: `false`",
		"repository_mutation_performed: `false`",
		"raw_prompt_id_included: `false`",
		"raw_prompt_name_included: `false`",
		"raw_prompt_purpose_included: `false`",
		"raw_prompt_draft_included: `false`",
		"raw_prompt_inputs_included: `false`",
		"raw_prompt_policy_included: `false`",
		"raw_prompt_notes_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_prompt_proposal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel prompt proposal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROMPT_PROPOSAL_SOURCE_SECRET", "CHANNEL_PROMPT_PROPOSAL_INGEST_SECRET", "CHANNEL_PROMPT_PROPOSAL_PURPOSE_TOKEN", "CHANNEL_PROMPT_PROPOSAL_DRAFT_TOKEN", "CHANNEL_PROMPT_PROPOSAL_INPUT_TOKEN", "CHANNEL_PROMPT_PROPOSAL_POLICY_TOKEN", "CHANNEL_PROMPT_PROPOSAL_NOTES_TOKEN", "weekly-context-brief", "Weekly context brief", "channel-prompt-proposal-thread-123", "inbound-732", "notify-732"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel prompt proposal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 732,
			"title": "GitClaw telegram thread channel-prompt-proposal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-prompt-proposal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 73202,
			"body": "@gitclaw /channels prompt-proposal --prompt-id weekly-context-brief --message-id inbound-732 --notify-message-id notify-732\nName: Weekly context brief\nPrompt:\nDo not leak duplicate token CHANNEL_PROMPT_PROPOSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel prompt proposal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[732]); got != 4 {
		t.Fatalf("duplicate channel prompt proposal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[732])
	}
	duplicateReceipt := github.CommentsByIssue[732][3].Body
	for _, want := range []string{
		"channel_prompt_proposal_status: `duplicate`",
		"prompt_proposal_issue: `#101`",
		"prompt_proposal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"raw_prompt_draft_included: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel prompt proposal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROMPT_PROPOSAL_DUPLICATE_SECRET", "weekly-context-brief", "channel-prompt-proposal-thread-123", "inbound-732", "notify-732"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel prompt proposal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelPromptProposalActionRequestParsesAlias(t *testing.T) {
	cfg := DefaultConfig()
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 37,
			Title:  "Channel prompt proposal",
			Body:   `<!-- gitclaw:channel-thread channel="slack" thread_id="thread-from-issue" -->`,
		},
		Comment: &Comment{
			ID: 3701,
			Body: `@gitclaw /channel prompt-pack-proposal --id Repo.Context.Brief --message-id source-1 --notify-message-id notify-1
Name: Repo Context Brief
Purpose: Review it.
Prompt:
Summarize the repository state.`,
		},
	}
	req, err := BuildChannelPromptProposalActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelPromptProposalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "prompt-pack-proposal" || req.Options.Channel != "slack" || req.Options.ThreadID != "thread-from-issue" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel prompt proposal parsing: %#v", req)
	}
	if req.Options.PromptID != "repo-context-brief" || req.Options.Name != "Repo Context Brief" || !strings.Contains(req.Options.Prompt, "Summarize the repository state.") {
		t.Fatalf("unexpected prompt proposal target fields: %#v", req.Options)
	}
	if !req.TargetFromIssue {
		t.Fatalf("expected prompt proposal target to come from channel issue")
	}
}
