package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelWhoamiQueuesIdentityStatusWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-whoami-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 815,
			"title": "GitClaw telegram thread chat-whoami-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-whoami-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 81501,
			"body": "@gitclaw /channels whoami --identity-id identity-1 --provider-user-id provider-secret-user --handle @secret-handle --role review-captain --message-id whoami-inbound-815 --notify-message-id whoami-notify-815\nDisplay name: Casey Whoami\nNotes:\nVisible identity note with CHANNEL_WHOAMI_NOTE_SECRET.",
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
			Number: 815,
			Title:  "GitClaw telegram thread chat-whoami-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{815: {{
			ID: 81500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-whoami-123",
				MessageID: "whoami-inbound-815",
				Author:    "telegram",
				Body:      "Original mirrored whoami request with CHANNEL_WHOAMI_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{815: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel whoami action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("whoami should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[815]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="whoami-notify-815"`,
		"GitClaw channel identity status.",
		"Display name: Casey Whoami",
		"Role: review-captain",
		"Identity record: pending GitHub review",
		"Contact card: not saved by this action",
		"Access: not granted or changed by this action.",
		"No access was granted by this action.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("whoami notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_WHOAMI_NOTE_SECRET", "CHANNEL_WHOAMI_INGEST_SECRET", "provider-secret-user", "@secret-handle"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("whoami notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Whoami Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels whoami`",
		"channel_whoami_status: `queued`",
		"identity_record_status: `pending-github-review`",
		"notification_target_issue: `#815`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"permission_grant_performed: `false`",
		"allowlist_mutation_performed: `false`",
		"pairing_code_issued: `false`",
		"contact_card_created: `false`",
		"access_review_created: `false`",
		"raw_identity_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_display_name_included: `false`",
		"raw_provider_user_id_included: `false`",
		"raw_provider_handle_included: `false`",
		"raw_role_included: `false`",
		"raw_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_whoami_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel whoami receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WHOAMI_INGEST_SECRET", "CHANNEL_WHOAMI_NOTE_SECRET", "Casey Whoami", "review-captain", "identity-1", "provider-secret-user", "@secret-handle", "chat-whoami-123", "whoami-inbound-815", "whoami-notify-815"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel whoami receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 815,
			"title": "GitClaw telegram thread chat-whoami-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-whoami-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 81502,
			"body": "@gitclaw /channels whoami --identity-id identity-1 --provider-user-id provider-secret-user --handle @secret-handle --role review-captain --message-id whoami-inbound-815 --notify-message-id whoami-notify-815\nDisplay name: Casey Whoami\nNotes:\nDo not leak duplicate token CHANNEL_WHOAMI_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate whoami created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[815]); got != 4 {
		t.Fatalf("duplicate whoami posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[815])
	}
	duplicateReceipt := github.CommentsByIssue[815][3].Body
	for _, want := range []string{
		"channel_whoami_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"permission_grant_performed: `false`",
		"allowlist_mutation_performed: `false`",
		"contact_card_created: `false`",
		"access_review_created: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate whoami receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_WHOAMI_DUPLICATE_SECRET", "Casey Whoami", "review-captain", "identity-1", "provider-secret-user", "@secret-handle"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate whoami receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelWhoamiActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel whoami"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel who --route team-demo --identity-id Identity.One --provider-user-id provider-user-1 --handle @route-user --role unrestricted --message-id source-1 --notify-message-id notify-1
Name: Route User
Notes:
Known participant from the team route.`,
		},
	}
	req, err := BuildChannelWhoamiActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelWhoamiActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "who" || req.Options.Route != "team-demo" || req.Options.IdentityID != "identity-one" || req.Options.ProviderUserID != "provider-user-1" || req.Options.ProviderHandle != "@route-user" || req.Options.Role != "unrestricted" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel whoami parsing: %#v", req)
	}
	if req.Options.DisplayName != "Route User" || !strings.Contains(req.Options.Notes, "Known participant") {
		t.Fatalf("unexpected whoami display name/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoIdentityID || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.DisplayNameSHA == "" || req.ProviderUserIDSHA == "" || req.ProviderHandleSHA == "" || req.RoleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route whoami hashes: %#v", req)
	}
}
