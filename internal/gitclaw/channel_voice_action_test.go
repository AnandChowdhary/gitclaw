package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelVoiceCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-voice-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 494,
			"title": "GitClaw telegram thread chat-voice-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-voice-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49401,
			"body": "@gitclaw /channels voice --voice-id voice-1 --duration 47s --media-type audio/ogg --url https://media.example.invalid/voice-secret.ogg --message-id inbound-494 --notify-message-id notify-494\nVoice: Capture channel-native voice notes\nTranscript:\nVisible transcript with CHANNEL_VOICE_TRANSCRIPT_SECRET.",
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
			Number: 494,
			Title:  "GitClaw telegram thread chat-voice-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{494: {{
			ID: 49400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-voice-123",
				MessageID: "inbound-494",
				Author:    "telegram",
				Body:      "Original mirrored voice caption with CHANNEL_VOICE_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{494: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel voice action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one voice issue: %#v", len(github.Issues), github.Issues)
	}
	voice := github.Issues[1]
	if !HasChannelVoiceMarker(voice.Body) || !strings.Contains(voice.Body, `voice_id="voice-1"`) {
		t.Fatalf("voice issue missing channel-voice marker:\n%s", voice.Body)
	}
	for _, want := range []string{
		"GitClaw channel voice note",
		"voice_id: voice-1",
		"source_channel: telegram",
		"source_issue: #494",
		"source_message_id_sha256_12:",
		"duration_seconds: 47",
		"media_type_sha256_12:",
		"audio_url_sha256_12:",
		"voice_mode: github-issue-transcript",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"raw_audio_url_included: false",
		"Capture channel-native voice notes",
		"Visible transcript with CHANNEL_VOICE_TRANSCRIPT_SECRET.",
	} {
		if !strings.Contains(voice.Body, want) {
			t.Fatalf("voice issue missing %q:\n%s", want, voice.Body)
		}
	}
	for _, leaked := range []string{"chat-voice-123", "inbound-494", "https://media.example.invalid/voice-secret.ogg", "audio/ogg", "CHANNEL_VOICE_INGEST_SECRET"} {
		if strings.Contains(voice.Body, leaked) {
			t.Fatalf("voice issue leaked provider IDs, media metadata, or channel body %q:\n%s", leaked, voice.Body)
		}
	}
	if !hasLabel(github.IssueLabels[voice.Number], "gitclaw") {
		t.Fatalf("voice issue missing gitclaw trigger label: %#v", github.IssueLabels[voice.Number])
	}

	sourceComments := github.CommentsByIssue[494]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-494"`,
		"GitClaw channel voice note captured.",
		"Voice note: #101",
		"https://github.com/owner/repo/issues/101",
		"Duration: 47s",
		"Title: Capture channel-native voice notes",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("voice notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_VOICE_TRANSCRIPT_SECRET", "CHANNEL_VOICE_INGEST_SECRET", "https://media.example.invalid/voice-secret.ogg", "audio/ogg"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("voice notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Voice Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels voice`",
		"channel_voice_status: `captured`",
		"voice_issue: `#101`",
		"voice_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#494`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"duration_seconds: `47`",
		"duration_seconds_known: `true`",
		"target_from_current_channel_issue: `true`",
		"raw_voice_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_voice_title_included: `false`",
		"raw_transcript_included: `false`",
		"raw_media_type_included: `false`",
		"raw_audio_url_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_voice_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel voice receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_VOICE_INGEST_SECRET", "CHANNEL_VOICE_TRANSCRIPT_SECRET", "Capture channel-native", "voice-1", "chat-voice-123", "inbound-494", "notify-494", "audio/ogg", "https://media.example.invalid/voice-secret.ogg"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel voice receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 494,
			"title": "GitClaw telegram thread chat-voice-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-voice-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 49402,
			"body": "@gitclaw /channels voice --voice-id voice-1 --duration 47s --media-type audio/ogg --url https://media.example.invalid/voice-secret.ogg --message-id inbound-494 --notify-message-id notify-494\nVoice: Capture channel-native voice notes\nTranscript:\nDo not leak duplicate token CHANNEL_VOICE_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate voice created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[494]); got != 4 {
		t.Fatalf("duplicate voice posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[494])
	}
	duplicateReceipt := github.CommentsByIssue[494][3].Body
	for _, want := range []string{
		"channel_voice_status: `duplicate`",
		"voice_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate voice receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_VOICE_DUPLICATE_SECRET") {
		t.Fatalf("duplicate voice receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelVoiceActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel voice"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel audio --route team-demo --voice-id Roadmap.Voice --duration 12 --media-type AUDIO/OGG --url https://media.example.invalid/demo.ogg --message-id source-1 --notify-message-id notify-1
Title: Make voice notes durable GitHub transcripts
Transcript:
- Keep Slack/Telegram lightweight.
- Let GitHub become the searchable voice-note surface.`,
		},
	}
	req, err := BuildChannelVoiceActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelVoiceActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "audio" || req.Options.Route != "team-demo" || req.Options.VoiceID != "roadmap-voice" || req.Options.DurationSeconds != 12 || req.Options.MediaType != "audio/ogg" || req.Options.AudioURL != "https://media.example.invalid/demo.ogg" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel voice parsing: %#v", req)
	}
	if req.Options.Title != "Make voice notes durable GitHub transcripts" || !strings.Contains(req.Options.Transcript, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected title/transcript: %#v", req)
	}
	if req.TargetFromIssue || req.AutoVoiceID || req.AutoNotifyMessageID || !req.DurationSecondsKnown || req.TitleSHA == "" || req.TranscriptSHA == "" || req.MediaTypeSHA == "" || req.AudioURLSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route voice hashes: %#v", req)
	}
}
