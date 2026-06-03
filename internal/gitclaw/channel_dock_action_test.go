package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelDockCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-dock-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-dock-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-dock-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels dock design-review --dock-id dock-1 --message-id inbound-484 --notify-message-id notify-484\nReason:\nVisible dock reason with CHANNEL_DOCK_REASON_SECRET.",
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
			Number: 484,
			Title:  "GitClaw telegram thread chat-dock-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-dock-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_DOCK_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel dock action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one dock issue: %#v", len(github.Issues), github.Issues)
	}
	dock := github.Issues[1]
	if !HasChannelDockMarker(dock.Body) || !strings.Contains(dock.Body, `dock_id="dock-1"`) {
		t.Fatalf("dock issue missing channel-dock marker:\n%s", dock.Body)
	}
	for _, want := range []string{
		"GitClaw channel dock",
		"dock_id: dock-1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"target_route: design-review",
		"dock_mode: github-issue-dock-request",
		"provider_route_change_performed: false",
		"session_route_persistence_performed: false",
		"routebook_mutation_performed: false",
		"workflow_mutation_performed: false",
		"provider_api_call_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Visible dock reason with CHANNEL_DOCK_REASON_SECRET.",
	} {
		if !strings.Contains(dock.Body, want) {
			t.Fatalf("dock issue missing %q:\n%s", want, dock.Body)
		}
	}
	if strings.Contains(dock.Body, "chat-dock-123") || strings.Contains(dock.Body, "inbound-484") || strings.Contains(dock.Body, "CHANNEL_DOCK_INGEST_SECRET") {
		t.Fatalf("dock issue leaked provider IDs or channel body:\n%s", dock.Body)
	}
	if !hasLabel(github.IssueLabels[dock.Number], "gitclaw") {
		t.Fatalf("dock issue missing gitclaw trigger label: %#v", github.IssueLabels[dock.Number])
	}

	sourceComments := github.CommentsByIssue[484]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-484"`,
		"GitClaw channel dock captured.",
		"Dock: #101",
		"https://github.com/owner/repo/issues/101",
		"Target route: design-review",
		"No provider route change has been performed.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("dock notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_DOCK_REASON_SECRET") || strings.Contains(outbound, "CHANNEL_DOCK_INGEST_SECRET") {
		t.Fatalf("dock notification leaked reason or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Dock Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels dock`",
		"channel_dock_status: `captured`",
		"dock_issue: `#101`",
		"dock_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"target_route_sha256_12: `",
		"provider_route_change_performed: `false`",
		"session_route_persistence_performed: `false`",
		"routebook_mutation_performed: `false`",
		"workflow_mutation_performed: `false`",
		"provider_api_call_performed: `false`",
		"raw_dock_id_included: `false`",
		"raw_target_route_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_reason_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_dock_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel dock receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_DOCK_INGEST_SECRET", "CHANNEL_DOCK_REASON_SECRET", "design-review", "dock-1", "chat-dock-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel dock receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-dock-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-dock-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels dock design-review --dock-id dock-1 --message-id inbound-484 --notify-message-id notify-484\nReason:\nDo not leak duplicate token CHANNEL_DOCK_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate dock created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate dock posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_dock_status: `duplicate`",
		"dock_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate dock receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_DOCK_DUPLICATE_SECRET") {
		t.Fatalf("duplicate dock receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelDockActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel dock"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel dock design-review --route team-demo --dock-id Roadmap.Spark --message-id source-1 --notify-message-id notify-1
Context:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable shaping surface.`,
		},
	}
	req, err := BuildChannelDockActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelDockActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "dock" || req.Options.TargetRoute != "design-review" || req.Options.Route != "team-demo" || req.Options.DockID != "roadmap-spark" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel dock parsing: %#v", req)
	}
	if !strings.Contains(req.Options.Reason, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected reason: %#v", req)
	}
	if req.TargetFromIssue || req.AutoDockID || req.AutoNotifyMessageID || req.TargetRouteSHA == "" || req.ReasonSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route dock hashes: %#v", req)
	}
}
