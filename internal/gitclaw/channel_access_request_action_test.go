package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelAccessRequestCreatesReviewIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-access-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 615,
			"title": "GitClaw telegram thread chat-access-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-access-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 61501,
			"body": "@gitclaw /channels access-request --access-id access-1 --provider-user-id provider-secret-user --handle @secret-handle --scope project-alpha --role user --message-id inbound-615 --notify-message-id notify-615\nRequester: Ada Lovelace\nReason:\nVisible access reason with CHANNEL_ACCESS_REASON_SECRET.",
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
			Number: 615,
			Title:  "GitClaw telegram thread chat-access-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{615: {{
			ID: 61500,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-access-123",
				MessageID: "inbound-615",
				Author:    "telegram",
				Body:      "Original mirrored access request with CHANNEL_ACCESS_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{615: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel access request action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one access request issue: %#v", len(github.Issues), github.Issues)
	}
	access := github.Issues[1]
	if !HasChannelAccessRequestMarker(access.Body) || !strings.Contains(access.Body, `access_id="access-1"`) {
		t.Fatalf("access request issue missing channel-access-request marker:\n%s", access.Body)
	}
	for _, want := range []string{
		"GitClaw channel access request",
		"access_id: access-1",
		"source_channel: telegram",
		"source_issue: #615",
		"source_message_id_sha256_12:",
		"requester: Ada Lovelace",
		"provider_user_id_sha256_12:",
		"provider_handle_sha256_12:",
		"scope: project-alpha",
		"requested_role: user",
		"access_mode: github-issue-access-review",
		"permission_grant_performed: false",
		"allowlist_mutation_performed: false",
		"pairing_code_issued: false",
		"raw_provider_user_id_included: false",
		"raw_provider_handle_included: false",
		"Visible access reason with CHANNEL_ACCESS_REASON_SECRET.",
	} {
		if !strings.Contains(access.Body, want) {
			t.Fatalf("access request issue missing %q:\n%s", want, access.Body)
		}
	}
	for _, leaked := range []string{"chat-access-123", "inbound-615", "provider-secret-user", "@secret-handle", "CHANNEL_ACCESS_INGEST_SECRET"} {
		if strings.Contains(access.Body, leaked) {
			t.Fatalf("access request issue leaked provider IDs or channel body %q:\n%s", leaked, access.Body)
		}
	}
	if !hasLabel(github.IssueLabels[access.Number], "gitclaw") {
		t.Fatalf("access request issue missing gitclaw trigger label: %#v", github.IssueLabels[access.Number])
	}

	sourceComments := github.CommentsByIssue[615]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-615"`,
		"GitClaw channel access request opened.",
		"Access review: #101",
		"https://github.com/owner/repo/issues/101",
		"Requester: Ada Lovelace",
		"Scope: project-alpha",
		"Requested role: user",
		"No access was granted by this action.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("access request notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_ACCESS_REASON_SECRET", "CHANNEL_ACCESS_INGEST_SECRET", "provider-secret-user", "@secret-handle"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("access request notification leaked %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Access Request Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels access-request`",
		"channel_access_request_status: `opened`",
		"access_request_issue: `#101`",
		"access_request_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#615`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"permission_grant_performed: `false`",
		"allowlist_mutation_performed: `false`",
		"pairing_code_issued: `false`",
		"raw_access_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_requester_included: `false`",
		"raw_provider_user_id_included: `false`",
		"raw_provider_handle_included: `false`",
		"raw_scope_included: `false`",
		"raw_requested_role_included: `false`",
		"raw_reason_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_access_request_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel access request receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ACCESS_INGEST_SECRET", "CHANNEL_ACCESS_REASON_SECRET", "Ada Lovelace", "project-alpha", "access-1", "provider-secret-user", "@secret-handle", "chat-access-123", "inbound-615", "notify-615"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel access request receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 615,
			"title": "GitClaw telegram thread chat-access-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-access-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 61502,
			"body": "@gitclaw /channels access-request --access-id access-1 --provider-user-id provider-secret-user --handle @secret-handle --scope project-alpha --role user --message-id inbound-615 --notify-message-id notify-615\nRequester: Ada Lovelace\nReason:\nDo not leak duplicate token CHANNEL_ACCESS_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate access request created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[615]); got != 4 {
		t.Fatalf("duplicate access request posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[615])
	}
	duplicateReceipt := github.CommentsByIssue[615][3].Body
	for _, want := range []string{
		"channel_access_request_status: `duplicate`",
		"access_request_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"permission_grant_performed: `false`",
		"allowlist_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate access request receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_ACCESS_DUPLICATE_SECRET") {
		t.Fatalf("duplicate access request receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelAccessRequestActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel access request"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel pairing --route team-demo --access-id Pair.Access --provider-user-id provider-user-1 --handle @route-user --scope team-demo --role admin --message-id source-1 --notify-message-id notify-1
Requester: Route User
Reason:
Needs temporary route access.`,
		},
	}
	req, err := BuildChannelAccessRequestActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelAccessRequestActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "pairing" || req.Options.Route != "team-demo" || req.Options.AccessID != "pair-access" || req.Options.ProviderUserID != "provider-user-1" || req.Options.ProviderHandle != "@route-user" || req.Options.Scope != "team-demo" || req.Options.RequestedRole != "admin" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel access request parsing: %#v", req)
	}
	if req.Options.Requester != "Route User" || !strings.Contains(req.Options.Reason, "Needs temporary route access.") {
		t.Fatalf("unexpected access requester/reason: %#v", req)
	}
	if req.TargetFromIssue || req.AutoAccessID || req.AutoNotifyMessageID || req.RequesterSHA == "" || req.ProviderUserIDSHA == "" || req.ProviderHandleSHA == "" || req.ScopeSHA == "" || req.RequestedRoleSHA == "" || req.ReasonSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route access hashes: %#v", req)
	}
}
