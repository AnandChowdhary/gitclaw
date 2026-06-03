package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelPostcardQueuesSceneCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "fixture repo\n")

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-postcard-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-postcard-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-postcard-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90101,
			"body": "@gitclaw /channels postcard launch-ready --message-id postcard-inbound-901 --notify-message-id postcard-notify-901 --postcard-id postcard-secret-901 --tone bright\nCaption: Release handoff is steady.\nDo not include this command hidden token in the receipt: CHANNEL_POSTCARD_COMMAND_MARKER.",
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
			Number: 901,
			Title:  "GitClaw telegram thread chat-postcard-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{901: {{
			ID: 90100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-postcard-123",
				MessageID: "postcard-inbound-901",
				Author:    "telegram",
				Body:      "Original mirrored postcard command with CHANNEL_POSTCARD_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{901: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel postcard action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("postcard action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[901]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="postcard-notify-901"`,
		"GitClaw channel postcard.",
		"Postcard: launch-ready",
		"Tone: bright",
		"Caption: Release handoff is steady.",
		"Postcard hash: ",
		"Caption hash: ",
		"Postcard source: GitHub channel action.",
		"Model call: not performed by this action.",
		"Image generation: not performed by this action.",
		"Media fetch: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("postcard notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_POSTCARD_INGEST_MARKER", "CHANNEL_POSTCARD_COMMAND_MARKER", "postcard-secret-901"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("postcard notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Postcard Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels postcard`",
		"channel_postcard_status: `queued`",
		"postcard_mode: `provider-facing-scene-card`",
		"notification_target_issue: `#901`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"postcard_id_sha256_12: `",
		"postcard_id_auto: `false`",
		"postcard_title_sha256_12: `",
		"postcard_title_bytes: `12`",
		"postcard_title_lines: `1`",
		"postcard_title_source: `positional-title`",
		"postcard_caption_sha256_12: `",
		"postcard_caption_bytes: `26`",
		"postcard_caption_lines: `1`",
		"postcard_caption_source: `trailing-caption`",
		"postcard_tone_sha256_12: `",
		"postcard_tone_bytes: `6`",
		"notification_body_sha256_12: `",
		"model_call_performed: `false`",
		"image_generation_performed: `false`",
		"media_fetch_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_postcard_id_included: `false`",
		"raw_postcard_title_included: `false`",
		"raw_postcard_caption_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_postcard_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel postcard receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_POSTCARD_INGEST_MARKER", "CHANNEL_POSTCARD_COMMAND_MARKER", "chat-postcard-123", "postcard-inbound-901", "postcard-notify-901", "postcard-secret-901", "launch-ready", "Release handoff is steady."} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel postcard receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 901,
			"title": "GitClaw telegram thread chat-postcard-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-postcard-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90102,
			"body": "@gitclaw /channels scene-card launch-ready --message-id postcard-inbound-901 --notify-message-id postcard-notify-901 --postcard-id postcard-secret-901 --tone bright\nCaption: Release handoff is steady.\nDo not leak duplicate token CHANNEL_POSTCARD_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate postcard created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[901]); got != 4 {
		t.Fatalf("duplicate postcard posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[901])
	}
	duplicateReceipt := github.CommentsByIssue[901][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels scene-card`",
		"channel_postcard_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"image_generation_performed: `false`",
		"media_fetch_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate postcard receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_POSTCARD_DUPLICATE_MARKER", "chat-postcard-123", "postcard-inbound-901", "postcard-notify-901", "postcard-secret-901", "launch-ready", "Release handoff is steady."} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate postcard receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelPostcardActionRequestParsesRouteAliasAndDefaults(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel postcard"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel field-note --route team-demo --message-id source-1 --notify-message-id notify-1 --postcard-id Postcard.One --tone observant
Postcard: Release crossed the line.
Caption: Tests are green.`,
		},
	}
	req, err := BuildChannelPostcardActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPostcardActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "field-note" || req.Options.Route != "team-demo" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.PostcardID != "postcard-one" || req.Options.Title != "Release crossed the line." || req.Options.Caption != "Tests are green." || req.Options.Tone != "observant" {
		t.Fatalf("unexpected channel postcard parsing: %#v", req)
	}
	if req.TitleSource != "trailing-title" || req.CaptionSource != "trailing-caption" {
		t.Fatalf("unexpected channel postcard text source parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoPostcardID || req.RequestedRouteHash == "" || req.PostcardIDHash == "" || req.TitleSHA == "" || req.CaptionSHA == "" || req.ToneSHA == "" || req.NotificationBodySHA == "" {
		t.Fatalf("expected explicit route postcard hashes: %#v", req)
	}
}

func TestBuildChannelPostcardActionRequestParsesPositionalRouteAndTitle(t *testing.T) {
	ev := Event{
		Kind:      EventIssueOpened,
		EventName: "issues",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 43,
			Title:  "@gitclaw /channels postcards team-demo launch-complete --message-id source-2 --notify-message-id notify-2 --postcard-id Postcard.Two --tone warm",
			Body:   "No channel marker on this issue.",
		},
	}
	req, err := BuildChannelPostcardActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelPostcardActionRequest returned error: %v", err)
	}
	if req.Options.Route != "team-demo" || req.Options.Title != "launch-complete" || req.Options.Tone != "warm" || req.TargetFromIssue {
		t.Fatalf("unexpected positional channel postcard parsing: %#v", req)
	}
}
