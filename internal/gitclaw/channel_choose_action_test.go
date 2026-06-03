package gitclaw

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHandleChannelChooseQueuesDeterministicChoiceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-choose-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 893,
			"title": "GitClaw telegram thread chat-choose-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-choose-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89301,
			"body": "@gitclaw /channels choose --message-id choose-inbound-893 --notify-message-id choose-notify-893 --choose-id choose-secret-893\nOptions:\n- Red Team\n- Blue Team\n- Green Team\nDo not include this command hidden token in the receipt: CHANNEL_CHOOSE_COMMAND_MARKER.",
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
			Number: 893,
			Title:  "GitClaw telegram thread chat-choose-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{893: {{
			ID: 89300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-choose-123",
				MessageID: "choose-inbound-893",
				Author:    "telegram",
				Body:      "Original mirrored choice command with CHANNEL_CHOOSE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{893: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel choose action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("choose action should not create artifact issues: %#v", github.Issues)
	}

	outcome, err := BuildChannelChooseOutcome(ChannelChooseOptions{
		Repo:            "owner/repo",
		Channel:         "telegram",
		ThreadID:        "chat-choose-123",
		SourceMessageID: "choose-inbound-893",
		NotifyMessageID: "choose-notify-893",
		ChooseID:        "choose-secret-893",
		Choices:         []string{"Red Team", "Blue Team", "Green Team"},
	})
	if err != nil {
		t.Fatalf("BuildChannelChooseOutcome returned error: %v", err)
	}

	sourceComments := github.CommentsByIssue[893]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="choose-notify-893"`,
		"GitClaw channel choice.",
		"Choices: 3",
		fmt.Sprintf("Picked: #%d", outcome.ChoiceIndex),
		fmt.Sprintf("Choice: %s", outcome.Choice),
		"Choice hash: ",
		"Seed hash: ",
		"Selection source: deterministic GitHub channel action seed.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("choice notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHOOSE_INGEST_MARKER", "CHANNEL_CHOOSE_COMMAND_MARKER", "choose-secret-893"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("choice notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Choose Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels choose`",
		"channel_choose_status: `queued`",
		"choose_mode: `deterministic-channel-option-picker`",
		"notification_target_issue: `#893`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"choose_id_sha256_12: `",
		"choose_id_auto: `false`",
		"choices_count: `3`",
		"choices_sha256_12: `",
		"choices_bytes: `29`",
		fmt.Sprintf("selected_choice_index: `%d`", outcome.ChoiceIndex),
		"selected_choice_sha256_12: `",
		"selection_sha256_12: `",
		"choice_seed_sha256_12: `",
		"notification_body_sha256_12: `",
		"deterministic_picker_used: `true`",
		"external_randomness_used: `false`",
		"cryptographic_randomness_used: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_choose_id_included: `false`",
		"raw_choices_included: `false`",
		"raw_selected_choice_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_choose_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel choose receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHOOSE_INGEST_MARKER", "CHANNEL_CHOOSE_COMMAND_MARKER", "chat-choose-123", "choose-inbound-893", "choose-notify-893", "choose-secret-893", "Red Team", "Blue Team", "Green Team"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel choose receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 893,
			"title": "GitClaw telegram thread chat-choose-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-choose-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89302,
			"body": "@gitclaw /channels pick --message-id choose-inbound-893 --notify-message-id choose-notify-893 --choose-id choose-secret-893\nOptions:\n- Red Team\n- Blue Team\n- Green Team\nDo not leak duplicate token CHANNEL_CHOOSE_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate choose created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[893]); got != 4 {
		t.Fatalf("duplicate choose posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[893])
	}
	duplicateReceipt := github.CommentsByIssue[893][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels pick`",
		"channel_choose_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"deterministic_picker_used: `true`",
		"external_randomness_used: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate choose receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CHOOSE_DUPLICATE_MARKER", "chat-choose-123", "choose-inbound-893", "choose-notify-893", "choose-secret-893", "Red Team", "Blue Team", "Green Team"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate choose receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelChooseActionRequestParsesRouteAliasAndInlineChoices(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel choice"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel pick --route team-demo --option alpha --option beta --message-id source-1 --notify-message-id notify-1 --choose-id Fun.Choice`,
		},
	}
	req, err := BuildChannelChooseActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelChooseActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "pick" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.ChooseID != "fun-choice" {
		t.Fatalf("unexpected channel choose parsing: %#v", req)
	}
	if got := strings.Join(req.Options.Choices, ","); got != "alpha,beta" {
		t.Fatalf("choices = %q, want alpha,beta", got)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoChooseID || req.RequestedRouteHash == "" || req.ChooseIDHash == "" || req.Outcome.SelectionMode != "deterministic-channel-option-picker" || req.Outcome.ChoiceIndex < 1 || req.Outcome.Choice == "" || req.Outcome.SeedSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route choose hashes and outcome: %#v", req)
	}
}

func TestHandleChannelOracleQueuesBoundedAnswerWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-oracle-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 894,
			"title": "GitClaw telegram thread chat-oracle-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-oracle-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 89401,
			"body": "@gitclaw /channels oracle --message-id oracle-inbound-894 --notify-message-id oracle-notify-894 --choose-id oracle-secret-894\nQuestion: Should we ship the tiny channel feature?\nDo not include this command hidden token in the receipt: CHANNEL_ORACLE_COMMAND_MARKER.",
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
			Number: 894,
			Title:  "GitClaw telegram thread chat-oracle-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{894: {{
			ID: 89400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-oracle-123",
				MessageID: "oracle-inbound-894",
				Author:    "telegram",
				Body:      "Original mirrored oracle command with CHANNEL_ORACLE_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{894: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel oracle action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("oracle action should not create artifact issues: %#v", github.Issues)
	}

	outcome, err := BuildChannelChooseOutcome(ChannelChooseOptions{
		Repo:            "owner/repo",
		Channel:         "telegram",
		ThreadID:        "chat-oracle-123",
		SourceMessageID: "oracle-inbound-894",
		NotifyMessageID: "oracle-notify-894",
		ChooseID:        "oracle-secret-894",
		Mode:            "oracle",
		Question:        "Should we ship the tiny channel feature?",
	})
	if err != nil {
		t.Fatalf("BuildChannelChooseOutcome returned error: %v", err)
	}
	if outcome.SelectionMode != "deterministic-channel-oracle" || len(outcome.Choices) != len(defaultChannelOracleChoices()) || outcome.Choice == "" {
		t.Fatalf("unexpected oracle outcome: %#v", outcome)
	}

	sourceComments := github.CommentsByIssue[894]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="oracle-notify-894"`,
		"GitClaw channel oracle.",
		"Question: Should we ship the tiny channel feature?",
		fmt.Sprintf("Answer: %s", outcome.Choice),
		"Answer hash: ",
		"Oracle hash: ",
		"Seed hash: ",
		"Selection source: deterministic GitHub channel action seed.",
		"Oracle deck: bounded static GitClaw answer deck.",
		"Model call: not performed by this action.",
		"External randomness: not used.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("oracle notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_ORACLE_INGEST_MARKER", "CHANNEL_ORACLE_COMMAND_MARKER", "oracle-secret-894"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("oracle notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Choose Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels oracle`",
		"channel_choose_status: `queued`",
		"choose_mode: `deterministic-channel-oracle`",
		"notification_target_issue: `#894`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"choose_id_sha256_12: `",
		"question_sha256_12: `",
		"question_bytes: `40`",
		"oracle_default_deck_used: `true`",
		"choices_count: `12`",
		"selected_choice_index: `",
		"selected_choice_sha256_12: `",
		"selection_sha256_12: `",
		"choice_seed_sha256_12: `",
		"deterministic_picker_used: `true`",
		"external_randomness_used: `false`",
		"cryptographic_randomness_used: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_question_included: `false`",
		"raw_choices_included: `false`",
		"raw_selected_choice_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_choose_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel oracle receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ORACLE_INGEST_MARKER", "CHANNEL_ORACLE_COMMAND_MARKER", "chat-oracle-123", "oracle-inbound-894", "oracle-notify-894", "oracle-secret-894", "Should we ship", outcome.Choice} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel oracle receipt leaked %q:\n%s", leaked, receipt)
		}
	}
}
