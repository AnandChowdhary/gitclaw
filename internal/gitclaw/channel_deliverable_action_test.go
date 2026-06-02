package gitclaw

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelDeliverableQueuesProviderNativeDeliverableWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-deliverable-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 640,
			"title": "GitClaw telegram thread chat-deliverable-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-deliverable-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64001,
			"body": "@gitclaw /channels deliverable --deliverable-id deliverable-1 --message-id deliverable-msg-1 --filename launch-report.pdf --media-type application/pdf --bytes 4242 --sha256 abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd --url https://artifacts.example/CHANNEL_DELIVERABLE_URL_SECRET\nCaption:\nVisible deliverable caption with CHANNEL_DELIVERABLE_CAPTION_SECRET.",
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
			Number: 640,
			Title:  "GitClaw telegram thread chat-deliverable-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{640: {{
			ID: 64000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-deliverable-123",
				MessageID: "inbound-640",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_DELIVERABLE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{640: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel deliverable action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("channel deliverable should not create a new issue: %#v", github.Issues)
	}
	sourceComments := github.CommentsByIssue[640]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + deliverable + receipt: %#v", len(sourceComments), sourceComments)
	}
	deliverable := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-deliverable",
		`channel="telegram"`,
		`message_id="deliverable-msg-1"`,
		`deliverable_id="deliverable-1"`,
		"GitClaw channel deliverable queued.",
		"Filename: launch-report.pdf",
		"Media type: application/pdf",
		"Size: 4242 bytes",
		"SHA-256: abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"URL: https://artifacts.example/CHANNEL_DELIVERABLE_URL_SECRET",
		"Visible deliverable caption with CHANNEL_DELIVERABLE_CAPTION_SECRET.",
		"Provider upload performed: false",
		"Provider delivery performed: false",
	} {
		if !strings.Contains(deliverable, want) {
			t.Fatalf("deliverable comment missing %q:\n%s", want, deliverable)
		}
	}
	if strings.Contains(deliverable, "CHANNEL_DELIVERABLE_INGEST_SECRET") {
		t.Fatalf("deliverable comment leaked original channel body:\n%s", deliverable)
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Deliverable Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels deliverable`",
		"channel_deliverable_status: `queued`",
		"deliverable_target_issue: `#640`",
		"deliverable_comment_id: `9000`",
		"deliverable_queued: `true`",
		"duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"deliverable_mode: `channel-outbox-native-deliverable`",
		"provider_upload_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_deliverable_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_message_id_included: `false`",
		"raw_deliverable_filename_included: `false`",
		"raw_deliverable_caption_included: `false`",
		"raw_deliverable_url_included: `false`",
		"raw_file_checksum_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_strategy: `channel-outbox --include-body + channel-delivery`",
		"llm_e2e_required_after_channel_deliverable_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel deliverable receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{
		"CHANNEL_DELIVERABLE_INGEST_SECRET",
		"CHANNEL_DELIVERABLE_CAPTION_SECRET",
		"CHANNEL_DELIVERABLE_URL_SECRET",
		"Visible deliverable caption",
		"launch-report.pdf",
		"deliverable-1",
		"deliverable-msg-1",
		"chat-deliverable-123",
		"https://artifacts.example",
		"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel deliverable receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	outbox, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "telegram",
		AccountID:   "telegram-deliverable-account-secret",
		IssueNumber: 640,
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox returned error: %v", err)
	}
	if outbox.SourceDeliverableComments != 2 || outbox.SourceAssistantComments != 1 || outbox.PendingMessages != 2 {
		t.Fatalf("unexpected deliverable outbox: %#v", outbox)
	}
	var deliverableOutbox *ChannelOutboxMessage
	for i := range outbox.Messages {
		if outbox.Messages[i].Kind == "channel-deliverable" {
			deliverableOutbox = &outbox.Messages[i]
			break
		}
	}
	if deliverableOutbox == nil {
		t.Fatalf("outbox file missing channel deliverable message: %#v", outbox.Messages)
	}
	if deliverableOutbox.Body != "" || deliverableOutbox.MessageHash == "" {
		t.Fatalf("metadata-only outbox should omit body and hash message: %#v", deliverableOutbox)
	}

	bodyOutbox, err := RunChannelOutbox(context.Background(), cfg, github, ChannelOutboxOptions{
		Repo:        "owner/repo",
		Channel:     "telegram",
		AccountID:   "telegram-deliverable-account-secret",
		IssueNumber: 640,
		IncludeBody: true,
		OutPath:     filepath.Join(t.TempDir(), "outbox.json"),
	})
	if err != nil {
		t.Fatalf("RunChannelOutbox include body returned error: %v", err)
	}
	var bodyDeliverable *ChannelOutboxMessage
	for i := range bodyOutbox.Messages {
		if bodyOutbox.Messages[i].Kind == "channel-deliverable" {
			bodyDeliverable = &bodyOutbox.Messages[i]
			break
		}
	}
	if bodyDeliverable == nil || !strings.Contains(bodyDeliverable.Body, "CHANNEL_DELIVERABLE_URL_SECRET") || strings.Contains(bodyDeliverable.Body, "gitclaw:channel-deliverable") {
		t.Fatalf("include-body outbox should expose visible deliverable body only: %#v", bodyOutbox.Messages)
	}

	delivery, err := RunChannelDelivery(context.Background(), cfg, github, ChannelDeliveryOptions{
		Repo:              "owner/repo",
		Channel:           "telegram",
		AccountID:         "telegram-deliverable-account-secret",
		IssueNumber:       640,
		CommentID:         9000,
		ExternalMessageID: "provider-deliverable-1",
		GatewayRunID:      "gateway-run-1",
	})
	if err != nil {
		t.Fatalf("RunChannelDelivery returned error for deliverable comment: %v", err)
	}
	if !delivery.Delivered || delivery.StateIssueNumber == 0 || delivery.ExternalMessageHash == "provider-deliverable-1" {
		t.Fatalf("unexpected delivery result: %#v", delivery)
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 640,
			"title": "GitClaw telegram thread chat-deliverable-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-deliverable-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 64002,
			"body": "@gitclaw /channels deliverable --deliverable-id deliverable-1 --message-id deliverable-msg-1 --filename launch-report.pdf --media-type application/pdf --bytes 4242 --url https://artifacts.example/duplicate\nCaption:\nDo not leak duplicate token CHANNEL_DELIVERABLE_DUPLICATE_SECRET.",
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
	if got := len(github.CommentsByIssue[640]); got != 4 {
		t.Fatalf("duplicate deliverable should add only receipt: comments=%d %#v", got, github.CommentsByIssue[640])
	}
	duplicateReceipt := github.CommentsByIssue[640][3].Body
	for _, want := range []string{
		"channel_deliverable_status: `duplicate`",
		"deliverable_queued: `false`",
		"duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate deliverable receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_DELIVERABLE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate deliverable receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelDeliverableActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel deliverable"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel send-file --route Team-Demo --id Design.File --message-id deliver-1 --filename Chart.PNG --type IMAGE/PNG --bytes 2048 --checksum ff00 --url https://example.invalid/chart.png
Message:
Here is the chart.`,
		},
	}
	req, err := BuildChannelDeliverableActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelDeliverableActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "send-file" || req.Options.Route != "team-demo" || req.Options.DeliverableID != "design-file" || req.Options.MessageID != "deliver-1" {
		t.Fatalf("unexpected channel deliverable parsing: %#v", req)
	}
	if req.Options.Filename != "Chart.PNG" || req.Options.MediaType != "image/png" || req.Options.Bytes != 2048 || req.Options.FileSHA256 != "ff00" || req.Options.URL != "https://example.invalid/chart.png" || !strings.Contains(req.Options.Caption, "Here is the chart.") {
		t.Fatalf("unexpected deliverable metadata: %#v", req)
	}
	if req.TargetFromIssue || req.AutoDeliverableID || req.AutoMessageID || req.FilenameSHA == "" || req.CaptionSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route deliverable hashes: %#v", req)
	}
}
