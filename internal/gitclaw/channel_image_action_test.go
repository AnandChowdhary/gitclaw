package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelImageCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-image-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 504,
			"title": "GitClaw telegram thread chat-image-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-image-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 50401,
			"body": "@gitclaw /channels image --image-id image-1 --width 1280 --height 720 --media-type image/png --url https://media.example.invalid/image-secret.png --message-id inbound-504 --notify-message-id notify-504\nImage: Capture channel-native screenshots\nDescription:\nVisible image description with CHANNEL_IMAGE_DESCRIPTION_SECRET.",
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
			Number: 504,
			Title:  "GitClaw telegram thread chat-image-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{504: {{
			ID: 50400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-image-123",
				MessageID: "inbound-504",
				Author:    "telegram",
				Body:      "Original mirrored image caption with CHANNEL_IMAGE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{504: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel image action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one image issue: %#v", len(github.Issues), github.Issues)
	}
	image := github.Issues[1]
	if !HasChannelImageMarker(image.Body) || !strings.Contains(image.Body, `image_id="image-1"`) {
		t.Fatalf("image issue missing channel-image marker:\n%s", image.Body)
	}
	for _, want := range []string{
		"GitClaw channel image note",
		"image_id: image-1",
		"source_channel: telegram",
		"source_issue: #504",
		"source_message_id_sha256_12:",
		"width: 1280",
		"height: 720",
		"media_type_sha256_12:",
		"source_url_sha256_12:",
		"image_mode: github-issue-visual-context",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"raw_source_url_included: false",
		"Capture channel-native screenshots",
		"Visible image description with CHANNEL_IMAGE_DESCRIPTION_SECRET.",
	} {
		if !strings.Contains(image.Body, want) {
			t.Fatalf("image issue missing %q:\n%s", want, image.Body)
		}
	}
	for _, leaked := range []string{"chat-image-123", "inbound-504", "https://media.example.invalid/image-secret.png", "image/png", "CHANNEL_IMAGE_INGEST_SECRET"} {
		if strings.Contains(image.Body, leaked) {
			t.Fatalf("image issue leaked provider IDs, media metadata, or channel body %q:\n%s", leaked, image.Body)
		}
	}
	if !hasLabel(github.IssueLabels[image.Number], "gitclaw") {
		t.Fatalf("image issue missing gitclaw trigger label: %#v", github.IssueLabels[image.Number])
	}

	sourceComments := github.CommentsByIssue[504]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-504"`,
		"GitClaw channel image note captured.",
		"Image note: #101",
		"https://github.com/owner/repo/issues/101",
		"Dimensions: 1280x720",
		"Title: Capture channel-native screenshots",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("image notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_IMAGE_DESCRIPTION_SECRET", "CHANNEL_IMAGE_INGEST_SECRET", "https://media.example.invalid/image-secret.png", "image/png"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("image notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Image Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels image`",
		"channel_image_status: `captured`",
		"image_issue: `#101`",
		"image_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#504`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"image_width: `1280`",
		"image_height: `720`",
		"image_dimensions_known: `true`",
		"target_from_current_channel_issue: `true`",
		"raw_image_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_image_title_included: `false`",
		"raw_image_description_included: `false`",
		"raw_media_type_included: `false`",
		"raw_source_url_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_image_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel image receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_IMAGE_INGEST_SECRET", "CHANNEL_IMAGE_DESCRIPTION_SECRET", "Capture channel-native", "image-1", "chat-image-123", "inbound-504", "notify-504", "image/png", "https://media.example.invalid/image-secret.png"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel image receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 504,
			"title": "GitClaw telegram thread chat-image-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-image-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 50402,
			"body": "@gitclaw /channels image --image-id image-1 --width 1280 --height 720 --media-type image/png --url https://media.example.invalid/image-secret.png --message-id inbound-504 --notify-message-id notify-504\nImage: Capture channel-native screenshots\nDescription:\nDo not leak duplicate token CHANNEL_IMAGE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate image created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[504]); got != 4 {
		t.Fatalf("duplicate image posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[504])
	}
	duplicateReceipt := github.CommentsByIssue[504][3].Body
	for _, want := range []string{
		"channel_image_status: `duplicate`",
		"image_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate image receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_IMAGE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate image receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelImageActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel image"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel screenshot --route team-demo --image-id Roadmap.Image --width 1440px --height 900px --media-type IMAGE/PNG --url https://media.example.invalid/demo.png --message-id source-1 --notify-message-id notify-1
Title: Make screenshots durable GitHub visual notes
Description:
- Keep Slack/Telegram lightweight.
- Let GitHub become the searchable visual-note surface.`,
		},
	}
	req, err := BuildChannelImageActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelImageActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "screenshot" || req.Options.Route != "team-demo" || req.Options.ImageID != "roadmap-image" || req.Options.Width != 1440 || req.Options.Height != 900 || req.Options.MediaType != "image/png" || req.Options.SourceURL != "https://media.example.invalid/demo.png" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel image parsing: %#v", req)
	}
	if req.Options.Title != "Make screenshots durable GitHub visual notes" || !strings.Contains(req.Options.Description, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected title/description: %#v", req)
	}
	if req.TargetFromIssue || req.AutoImageID || req.AutoNotifyMessageID || !req.DimensionsKnown || req.TitleSHA == "" || req.DescriptionSHA == "" || req.MediaTypeSHA == "" || req.SourceURLSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route image hashes: %#v", req)
	}
}
