package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelAttachmentCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-attachment-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 492,
			"title": "GitClaw telegram thread chat-attachment-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-attachment-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49201,
			"body": "@gitclaw /channels attachment --attachment-id attachment-1 --message-id inbound-492 --notify-message-id notify-492 --filename launch-brief.pdf --media-type application/pdf --bytes 4242 --sha256 abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd --source-url https://signed.example/CHANNEL_ATTACHMENT_URL_SECRET\nCaption:\nVisible attachment caption with CHANNEL_ATTACHMENT_CAPTION_SECRET.",
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
			Number: 492,
			Title:  "GitClaw telegram thread chat-attachment-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{492: {{
			ID: 49200,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-attachment-123",
				MessageID: "inbound-492",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_ATTACHMENT_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{492: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel attachment action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one attachment issue: %#v", len(github.Issues), github.Issues)
	}
	attachment := github.Issues[1]
	if !HasChannelAttachmentMarker(attachment.Body) || !strings.Contains(attachment.Body, `attachment_id="attachment-1"`) {
		t.Fatalf("attachment issue missing channel-attachment marker:\n%s", attachment.Body)
	}
	for _, want := range []string{
		"GitClaw channel attachment",
		"attachment_id: attachment-1",
		"source_channel: telegram",
		"source_issue: #492",
		"source_message_id_sha256_12:",
		"filename: launch-brief.pdf",
		"media_type: application/pdf",
		"attachment_bytes: 4242",
		"file_sha256: abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"source_url_sha256_12:",
		"attachment_mode: github-issue-attachment-metadata",
		"attachment_bytes_included: false",
		"provider_fetch_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"raw_source_url_included: false",
		"raw_attachment_bytes_included: false",
		"Visible attachment caption with CHANNEL_ATTACHMENT_CAPTION_SECRET.",
	} {
		if !strings.Contains(attachment.Body, want) {
			t.Fatalf("attachment issue missing %q:\n%s", want, attachment.Body)
		}
	}
	for _, leaked := range []string{"chat-attachment-123", "inbound-492", "CHANNEL_ATTACHMENT_INGEST_SECRET", "CHANNEL_ATTACHMENT_URL_SECRET"} {
		if strings.Contains(attachment.Body, leaked) {
			t.Fatalf("attachment issue leaked %q:\n%s", leaked, attachment.Body)
		}
	}
	if !hasLabel(github.IssueLabels[attachment.Number], "gitclaw") {
		t.Fatalf("attachment issue missing gitclaw trigger label: %#v", github.IssueLabels[attachment.Number])
	}

	sourceComments := github.CommentsByIssue[492]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-492"`,
		"GitClaw channel attachment recorded.",
		"Attachment: #101",
		"https://github.com/owner/repo/issues/101",
		"Filename: launch-brief.pdf",
		"Media type: application/pdf",
		"Size: 4242 bytes",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("attachment notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_ATTACHMENT_CAPTION_SECRET", "CHANNEL_ATTACHMENT_INGEST_SECRET", "CHANNEL_ATTACHMENT_URL_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("attachment notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Attachment Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels attachment`",
		"channel_attachment_status: `recorded`",
		"attachment_issue: `#101`",
		"attachment_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#492`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"attachment_mode: `github-issue-attachment-metadata`",
		"attachment_issue_labeled_for_gitclaw: `true`",
		"provider_fetch_performed: `false`",
		"raw_attachment_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_attachment_filename_included: `false`",
		"raw_attachment_caption_included: `false`",
		"raw_source_url_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_attachment_bytes_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_attachment_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel attachment receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{
		"CHANNEL_ATTACHMENT_INGEST_SECRET",
		"CHANNEL_ATTACHMENT_CAPTION_SECRET",
		"CHANNEL_ATTACHMENT_URL_SECRET",
		"Visible attachment caption",
		"launch-brief.pdf",
		"attachment-1",
		"chat-attachment-123",
		"inbound-492",
		"notify-492",
		"https://signed.example",
		"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel attachment receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 492,
			"title": "GitClaw telegram thread chat-attachment-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-attachment-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49202,
			"body": "@gitclaw /channels attachment --attachment-id attachment-1 --message-id inbound-492 --notify-message-id notify-492 --filename launch-brief.pdf --media-type application/pdf --bytes 4242\nCaption:\nDo not leak duplicate token CHANNEL_ATTACHMENT_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate attachment created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[492]); got != 4 {
		t.Fatalf("duplicate attachment posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[492])
	}
	duplicateReceipt := github.CommentsByIssue[492][3].Body
	for _, want := range []string{
		"channel_attachment_status: `duplicate`",
		"attachment_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate attachment receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_ATTACHMENT_DUPLICATE_SECRET") {
		t.Fatalf("duplicate attachment receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelAttachmentActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel attachment"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel media --route Team-Demo --file-id Design.Attachment --message-id source-1 --notify-message-id notify-1 --name Diagram.png --type IMAGE/PNG --bytes 2048 --checksum ff00
Description:
Keep this media reference handy.`,
		},
	}
	req, err := BuildChannelAttachmentActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelAttachmentActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "media" || req.Options.Route != "team-demo" || req.Options.AttachmentID != "design-attachment" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel attachment parsing: %#v", req)
	}
	if req.Options.Filename != "Diagram.png" || req.Options.MediaType != "image/png" || req.Options.Bytes != 2048 || req.Options.FileSHA256 != "ff00" || !strings.Contains(req.Options.Caption, "Keep this media reference handy.") {
		t.Fatalf("unexpected attachment metadata: %#v", req)
	}
	if req.TargetFromIssue || req.AutoAttachmentID || req.AutoNotifyMessageID || req.FilenameSHA == "" || req.CaptionSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route attachment hashes: %#v", req)
	}
}
