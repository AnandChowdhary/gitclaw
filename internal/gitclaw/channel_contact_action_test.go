package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelContactCreatesContactIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-contact-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 715,
			"title": "GitClaw telegram thread chat-contact-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-contact-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 71501,
			"body": "@gitclaw /channels contact --contact-id contact-1 --provider-user-id provider-secret-user --handle @secret-handle --role reviewer --message-id inbound-715 --notify-message-id notify-715\nDisplay name: Ada Lovelace\nNotes:\nVisible contact note with CHANNEL_CONTACT_NOTE_SECRET.",
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
			Number: 715,
			Title:  "GitClaw telegram thread chat-contact-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{715: {{
			ID: 71500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-contact-123",
				MessageID: "inbound-715",
				Author:    "telegram",
				Body:      "Original mirrored contact details with CHANNEL_CONTACT_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{715: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel contact action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one contact issue: %#v", len(github.Issues), github.Issues)
	}
	contact := github.Issues[1]
	if !HasChannelContactMarker(contact.Body) || !strings.Contains(contact.Body, `contact_id="contact-1"`) {
		t.Fatalf("contact issue missing channel-contact marker:\n%s", contact.Body)
	}
	for _, want := range []string{
		"GitClaw channel contact card",
		"contact_id: contact-1",
		"source_channel: telegram",
		"source_issue: #715",
		"source_message_id_sha256_12:",
		"display_name: Ada Lovelace",
		"provider_user_id_sha256_12:",
		"provider_handle_sha256_12:",
		"contact_role: reviewer",
		"contact_mode: github-issue-contact-card",
		"permission_grant_performed: false",
		"allowlist_mutation_performed: false",
		"pairing_code_issued: false",
		"raw_provider_user_id_included: false",
		"raw_provider_handle_included: false",
		"Visible contact note with CHANNEL_CONTACT_NOTE_SECRET.",
	} {
		if !strings.Contains(contact.Body, want) {
			t.Fatalf("contact issue missing %q:\n%s", want, contact.Body)
		}
	}
	for _, leaked := range []string{"chat-contact-123", "inbound-715", "provider-secret-user", "@secret-handle", "CHANNEL_CONTACT_INGEST_SECRET"} {
		if strings.Contains(contact.Body, leaked) {
			t.Fatalf("contact issue leaked provider IDs or channel body %q:\n%s", leaked, contact.Body)
		}
	}
	if !hasLabel(github.IssueLabels[contact.Number], "gitclaw") {
		t.Fatalf("contact issue missing gitclaw trigger label: %#v", github.IssueLabels[contact.Number])
	}

	sourceComments := github.CommentsByIssue[715]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-715"`,
		"GitClaw channel contact card saved.",
		"Contact card: #101",
		"https://github.com/owner/repo/issues/101",
		"Display name: Ada Lovelace",
		"Contact role: reviewer",
		"No access was granted by this action.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("contact notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_CONTACT_NOTE_SECRET", "CHANNEL_CONTACT_INGEST_SECRET", "provider-secret-user", "@secret-handle"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("contact notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Contact Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels contact`",
		"channel_contact_status: `opened`",
		"contact_issue: `#101`",
		"contact_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#715`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"permission_grant_performed: `false`",
		"allowlist_mutation_performed: `false`",
		"pairing_code_issued: `false`",
		"raw_contact_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_display_name_included: `false`",
		"raw_provider_user_id_included: `false`",
		"raw_provider_handle_included: `false`",
		"raw_contact_role_included: `false`",
		"raw_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_contact_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel contact receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CONTACT_INGEST_SECRET", "CHANNEL_CONTACT_NOTE_SECRET", "Ada Lovelace", "reviewer", "contact-1", "provider-secret-user", "@secret-handle", "chat-contact-123", "inbound-715", "notify-715"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel contact receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 715,
			"title": "GitClaw telegram thread chat-contact-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-contact-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 71502,
			"body": "@gitclaw /channels contact --contact-id contact-1 --provider-user-id provider-secret-user --handle @secret-handle --role reviewer --message-id inbound-715 --notify-message-id notify-715\nDisplay name: Ada Lovelace\nNotes:\nDo not leak duplicate token CHANNEL_CONTACT_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate contact created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[715]); got != 4 {
		t.Fatalf("duplicate contact posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[715])
	}
	duplicateReceipt := github.CommentsByIssue[715][3].Body
	for _, want := range []string{
		"channel_contact_status: `duplicate`",
		"contact_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"permission_grant_performed: `false`",
		"allowlist_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate contact receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_CONTACT_DUPLICATE_SECRET", "Ada Lovelace", "reviewer", "contact-1", "provider-secret-user", "@secret-handle"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate contact receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelContactActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel contact"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel profile --route team-demo --contact-id Contact.One --provider-user-id provider-user-1 --handle @route-user --role admin --message-id source-1 --notify-message-id notify-1
Name: Route User
Notes:
Known reviewer from the team route.`,
		},
	}
	req, err := BuildChannelContactActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelContactActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "profile" || req.Options.Route != "team-demo" || req.Options.ContactID != "contact-one" || req.Options.ProviderUserID != "provider-user-1" || req.Options.ProviderHandle != "@route-user" || req.Options.ContactRole != "admin" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel contact parsing: %#v", req)
	}
	if req.Options.DisplayName != "Route User" || !strings.Contains(req.Options.Notes, "Known reviewer") {
		t.Fatalf("unexpected contact display name/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoContactID || req.AutoNotifyMessageID || req.DisplayNameSHA == "" || req.ProviderUserIDSHA == "" || req.ProviderHandleSHA == "" || req.ContactRoleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route contact hashes: %#v", req)
	}
}
