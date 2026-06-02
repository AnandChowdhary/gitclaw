package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelCheckpointRehearsalCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "channel-checkpoint-rehearsal-thread-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 491,
			"title": "GitClaw telegram thread channel-checkpoint-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-checkpoint-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49101,
			"body": "@gitclaw /channels rehearse-checkpoint --id channel-checkpoint-rehearsal --target HEAD~1 --message-id inbound-491 --notify-message-id notify-491\nPlease review this channel-origin checkpoint rollback.\nCHANNEL_CHECKPOINT_REHEARSAL_SOURCE_SECRET",
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
			Number: 491,
			Title:  "GitClaw telegram thread channel-checkpoint-rehearsal-thread-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{491: {{
			ID: 49100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "channel-checkpoint-rehearsal-thread-123",
				MessageID: "inbound-491",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_CHECKPOINT_REHEARSAL_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{491: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel checkpoint rehearsal action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one checkpoint rehearsal issue: %#v", len(github.Issues), github.Issues)
	}
	rehearsalIssue := github.Issues[1]
	for _, want := range []string{
		"gitclaw:checkpoint-rehearsal-issue",
		`id="channel-checkpoint-rehearsal"`,
		"GitClaw checkpoint rollback rehearsal issue",
		"rehearsal_id: channel-checkpoint-rehearsal",
		"target_ref: HEAD~1",
		"target_ref_sha256_12:",
		"target_allowed: true",
		"source_issue: #491",
		"source_kind: channel_comment",
		"rehearsal_mode: rollback-conversation",
		"restore_mode: rehearsal-only",
		"rollback_mode: inspect-only",
		"repository_mutation_allowed: false",
		"git_reset_allowed: false",
		"git_clean_allowed: false",
		"checkout_mutation_allowed: false",
		"raw_source_body_included: false",
		"raw_diffs_included: false",
		"raw_file_bodies_included: false",
		"gitclaw checkpoints status",
		"gitclaw checkpoints preview HEAD~1",
		"gitclaw checkpoints risk",
		"gitclaw rollback diff HEAD~1",
		"gitclaw rollback risk",
	} {
		if !strings.Contains(rehearsalIssue.Body, want) {
			t.Fatalf("checkpoint rehearsal issue missing %q:\n%s", want, rehearsalIssue.Body)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHECKPOINT_REHEARSAL_SOURCE_SECRET", "CHANNEL_CHECKPOINT_REHEARSAL_INGEST_SECRET", "Please review this channel-origin"} {
		if strings.Contains(rehearsalIssue.Body, leaked) {
			t.Fatalf("checkpoint rehearsal issue leaked %q:\n%s", leaked, rehearsalIssue.Body)
		}
	}

	sourceComments := github.CommentsByIssue[491]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-491"`,
		"GitClaw channel checkpoint rehearsal",
		"Rehearsal issue: #101",
		"https://github.com/owner/repo/issues/101",
		"Target ref hash: " + shortDocumentHash("HEAD~1"),
		"Rollback mode: inspect-only",
		"Restore mode: rehearsal-only",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("channel checkpoint rehearsal notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHECKPOINT_REHEARSAL_SOURCE_SECRET", "CHANNEL_CHECKPOINT_REHEARSAL_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("channel checkpoint rehearsal notification leaked %q:\n%s", leaked, outbound)
		}
	}
	outboundParts := strings.SplitN(outbound, "\n", 2)
	if len(outboundParts) != 2 {
		t.Fatalf("channel checkpoint rehearsal outbound missing provider body:\n%s", outbound)
	}
	notificationBody := strings.TrimSpace(outboundParts[1])
	for _, leaked := range []string{"HEAD~1", "channel-checkpoint-rehearsal"} {
		if strings.Contains(notificationBody, leaked) {
			t.Fatalf("channel checkpoint rehearsal provider body leaked %q:\n%s", leaked, notificationBody)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Checkpoint Rehearsal Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels rehearse-checkpoint`",
		"channel_checkpoint_rehearsal_status: `created`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#491`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"source_kind: `channel_comment`",
		"target_ref_sha256_12: `" + shortDocumentHash("HEAD~1") + "`",
		"target_allowed: `true`",
		"notification_body_sha256_12: `" + shortDocumentHash(notificationBody) + "`",
		fmt.Sprintf("notification_body_bytes: `%d`", len(notificationBody)),
		fmt.Sprintf("notification_body_lines: `%d`", lineCount(notificationBody)),
		"target_from_current_channel_issue: `true`",
		"rehearsal_mode: `rollback-conversation`",
		"rehearsal_issue_labeled_for_gitclaw: `true`",
		"model_call_performed: `false`",
		"restore_mode: `rehearsal-only`",
		"rollback_mode: `inspect-only`",
		"repository_mutation_allowed: `false`",
		"git_reset_allowed: `false`",
		"git_clean_allowed: `false`",
		"checkout_mutation_allowed: `false`",
		"raw_rehearsal_id_included: `false`",
		"raw_target_ref_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_source_body_included: `false`",
		"raw_diffs_included: `false`",
		"raw_file_bodies_included: `false`",
		"provider_delivery_performed: `false`",
		"repository_mutation_performed: `false`",
		"llm_e2e_required_after_channel_checkpoint_rehearsal_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel checkpoint rehearsal receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHECKPOINT_REHEARSAL_SOURCE_SECRET", "CHANNEL_CHECKPOINT_REHEARSAL_INGEST_SECRET", "Please review this channel-origin", "channel-checkpoint-rehearsal", "channel-checkpoint-rehearsal-thread-123", "inbound-491", "notify-491", "HEAD~1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel checkpoint rehearsal receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 491,
			"title": "GitClaw telegram thread channel-checkpoint-rehearsal-thread-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"channel-checkpoint-rehearsal-thread-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49102,
			"body": "@gitclaw /channels rollback-rehearsal --id channel-checkpoint-rehearsal --target HEAD~1 --message-id inbound-491 --notify-message-id notify-491\nDo not leak duplicate token CHANNEL_CHECKPOINT_REHEARSAL_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate channel checkpoint rehearsal created another issue: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[491]); got != 4 {
		t.Fatalf("duplicate channel checkpoint rehearsal posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[491])
	}
	duplicateReceipt := github.CommentsByIssue[491][3].Body
	for _, want := range []string{
		"channel_checkpoint_rehearsal_status: `duplicate`",
		"rehearsal_issue: `#101`",
		"rehearsal_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate channel checkpoint rehearsal receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHECKPOINT_REHEARSAL_DUPLICATE_SECRET", "channel-checkpoint-rehearsal", "channel-checkpoint-rehearsal-thread-123", "inbound-491", "notify-491", "HEAD~1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate channel checkpoint rehearsal receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelCheckpointRehearsalActionRequestParsesAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 34, Title: "Channel checkpoint rehearsal"},
		Comment: &Comment{
			ID:   3401,
			Body: `@gitclaw /channel rollback-drill --ref HEAD~2 --id Checkpoint.Lab --channel slack --thread-id thread-1 --message-id source-1 --notify-message-id notify-1`,
		},
	}
	req, err := BuildChannelCheckpointRehearsalActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelCheckpointRehearsalActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "rollback-drill" || req.Options.Channel != "slack" || req.Options.TargetRef != "HEAD~2" || req.Options.RehearsalID != "checkpoint-lab" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel checkpoint rehearsal parsing: %#v", req)
	}
	if req.Rehearsal.TargetRef != "HEAD~2" || req.Rehearsal.SourceKind != "channel_comment" || req.Rehearsal.SourceCommentID != 3401 {
		t.Fatalf("unexpected channel checkpoint rehearsal request: %#v", req.Rehearsal)
	}
	if !strings.Contains(req.Rehearsal.CheckpointPreviewCmd, "HEAD~2") || !strings.Contains(req.Rehearsal.RollbackDiffCmd, "HEAD~2") {
		t.Fatalf("unexpected checkpoint commands: %#v", req.Rehearsal)
	}
}
