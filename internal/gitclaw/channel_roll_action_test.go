package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelRollQueuesDeterministicResultWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-roll-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 892,
			"title": "GitClaw telegram thread chat-roll-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-roll-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89201,
			"body": "@gitclaw /channels roll --dice 2d6+1 --message-id roll-inbound-892 --notify-message-id roll-notify-892 --roll-id roll-secret-892\nDo not include this command hidden token in the receipt: CHANNEL_ROLL_COMMAND_MARKER.",
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
			Number: 892,
			Title:  "GitClaw telegram thread chat-roll-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{892: {{
			ID: 89200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-roll-123",
				MessageID: "roll-inbound-892",
				Author:    "telegram",
				Body:      "Original mirrored roll command with CHANNEL_ROLL_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{892: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel roll action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("roll action should not create artifact issues: %#v", github.Issues)
	}

	outcome, err := BuildChannelRollOutcome(ChannelRollOptions{
		Repo:            "owner/repo",
		Channel:         "telegram",
		ThreadID:        "chat-roll-123",
		SourceMessageID: "roll-inbound-892",
		NotifyMessageID: "roll-notify-892",
		RollID:          "roll-secret-892",
		Expression:      "2d6+1",
	})
	if err != nil {
		t.Fatalf("BuildChannelRollOutcome returned error: %v", err)
	}

	sourceComments := github.CommentsByIssue[892]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="roll-notify-892"`,
		"GitClaw channel roll.",
		"Roll: 2d6+1",
		fmt.Sprintf("Result: %d", outcome.Total),
		fmt.Sprintf("Dice: %s", channelRollValuesString(outcome.Values)),
		"Modifier: +1",
		"Roll hash: ",
		"Seed hash: ",
		"Random source: deterministic GitHub channel action seed.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("roll notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROLL_INGEST_MARKER", "CHANNEL_ROLL_COMMAND_MARKER", "roll-secret-892"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("roll notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Roll Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels roll`",
		"channel_roll_status: `queued`",
		"roll_mode: `deterministic-channel-randomizer`",
		"notification_target_issue: `#892`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"roll_expression_sha256_12: `",
		"roll_expression_bytes: `5`",
		"roll_kind: `dice`",
		"dice_count: `2`",
		"dice_sides: `6`",
		"dice_modifier: `1`",
		fmt.Sprintf("roll_total: `%d`", outcome.Total),
		"roll_values_sha256_12: `",
		"roll_result_sha256_12: `",
		"roll_seed_sha256_12: `",
		"notification_body_sha256_12: `",
		"deterministic_rng_used: `true`",
		"external_randomness_used: `false`",
		"cryptographic_randomness_used: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_roll_id_included: `false`",
		"raw_roll_expression_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_roll_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel roll receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROLL_INGEST_MARKER", "CHANNEL_ROLL_COMMAND_MARKER", "chat-roll-123", "roll-inbound-892", "roll-notify-892", "roll-secret-892", "2d6+1"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel roll receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 892,
			"title": "GitClaw telegram thread chat-roll-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-roll-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89202,
			"body": "@gitclaw /channels random --dice 2d6+1 --message-id roll-inbound-892 --notify-message-id roll-notify-892 --roll-id roll-secret-892\nDo not leak duplicate token CHANNEL_ROLL_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate roll created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[892]); got != 4 {
		t.Fatalf("duplicate roll posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[892])
	}
	duplicateReceipt := github.CommentsByIssue[892][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels random`",
		"channel_roll_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"deterministic_rng_used: `true`",
		"external_randomness_used: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate roll receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROLL_DUPLICATE_MARKER", "chat-roll-123", "roll-inbound-892", "roll-notify-892", "roll-secret-892", "2d6+1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate roll receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelRollActionRequestParsesCoinRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel coin"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel flip --route team-demo --message-id source-1 --notify-message-id notify-1 --roll-id Fun.Roll`,
		},
	}
	req, err := BuildChannelRollActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRollActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "flip" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.RollID != "fun-roll" {
		t.Fatalf("unexpected channel roll parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoRollID || req.RequestedRouteHash == "" || req.RollIDHash == "" || req.Outcome.Kind != "coin" || req.Outcome.NormalizedExpression != "coin" || req.Outcome.Label == "" || req.Outcome.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route coin hashes and outcome: %#v", req)
	}
}
