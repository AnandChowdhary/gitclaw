package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelOpenLoopCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-open-loop-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-open-loop-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-open-loop-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26301,
			"body": "@gitclaw /channels open-loop --loop-id loop-1 --message-id inbound-263 --notify-message-id notify-263\nTitle: Resolve channel launch question\nContext:\nVisible open-loop context with CHANNEL_OPEN_LOOP_CONTEXT_SECRET.\nNext step:\nVisible open-loop next step with CHANNEL_OPEN_LOOP_NEXT_SECRET.",
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
			Number: 263,
			Title:  "GitClaw telegram thread chat-open-loop-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{263: {{
			ID: 26300,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-open-loop-123",
				MessageID: "inbound-263",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_OPEN_LOOP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{263: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel open loop action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one open-loop issue: %#v", len(github.Issues), github.Issues)
	}
	openLoop := github.Issues[1]
	if !HasChannelOpenLoopMarker(openLoop.Body) || !strings.Contains(openLoop.Body, `loop_id="loop-1"`) {
		t.Fatalf("open-loop issue missing channel-open-loop marker:\n%s", openLoop.Body)
	}
	for _, want := range []string{
		"GitClaw channel open loop",
		"loop_id: loop-1",
		"source_channel: telegram",
		"source_issue: #263",
		"source_message_id_sha256_12:",
		"open_loop_mode: github-issue-open-loop",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Resolve channel launch question",
		"Visible open-loop context with CHANNEL_OPEN_LOOP_CONTEXT_SECRET.",
		"Visible open-loop next step with CHANNEL_OPEN_LOOP_NEXT_SECRET.",
	} {
		if !strings.Contains(openLoop.Body, want) {
			t.Fatalf("open-loop issue missing %q:\n%s", want, openLoop.Body)
		}
	}
	if strings.Contains(openLoop.Body, "chat-open-loop-123") || strings.Contains(openLoop.Body, "inbound-263") || strings.Contains(openLoop.Body, "CHANNEL_OPEN_LOOP_INGEST_SECRET") {
		t.Fatalf("open-loop issue leaked provider IDs or channel body:\n%s", openLoop.Body)
	}
	if !hasLabel(github.IssueLabels[openLoop.Number], "gitclaw") {
		t.Fatalf("open-loop issue missing gitclaw trigger label: %#v", github.IssueLabels[openLoop.Number])
	}

	sourceComments := github.CommentsByIssue[263]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-263"`,
		"GitClaw channel open loop captured.",
		"Open loop: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Resolve channel launch question",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("open_loop notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_OPEN_LOOP_CONTEXT_SECRET") || strings.Contains(outbound, "CHANNEL_OPEN_LOOP_NEXT_SECRET") || strings.Contains(outbound, "CHANNEL_OPEN_LOOP_INGEST_SECRET") {
		t.Fatalf("open_loop notification leaked context, next step, or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Open Loop Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels open-loop`",
		"channel_open_loop_status: `saved`",
		"open_loop_issue: `#101`",
		"open_loop_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#263`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_open_loop_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_open_loop_title_included: `false`",
		"open_loop_context_sha256_12:",
		"open_loop_next_step_sha256_12:",
		"raw_open_loop_context_included: `false`",
		"raw_open_loop_next_step_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_open_loop_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel open loop receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_OPEN_LOOP_INGEST_SECRET", "CHANNEL_OPEN_LOOP_CONTEXT_SECRET", "CHANNEL_OPEN_LOOP_NEXT_SECRET", "Resolve channel launch question", "loop-1", "chat-open-loop-123", "inbound-263", "notify-263"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel open loop receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 263,
			"title": "GitClaw telegram thread chat-open-loop-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-open-loop-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 26302,
			"body": "@gitclaw /channels open-loop --loop-id loop-1 --message-id inbound-263 --notify-message-id notify-263\nTitle: Resolve channel launch question\nContext:\nDo not leak duplicate token CHANNEL_OPEN_LOOP_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate open_loop created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[263]); got != 4 {
		t.Fatalf("duplicate open_loop posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[263])
	}
	duplicateReceipt := github.CommentsByIssue[263][3].Body
	for _, want := range []string{
		"channel_open_loop_status: `duplicate`",
		"open_loop_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate open_loop receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_OPEN_LOOP_DUPLICATE_SECRET", "Resolve channel launch question", "loop-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate open-loop receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelOpenLoopActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 30, Title: "Channel open loop"},
		Comment: &Comment{
			ID: 3001,
			Body: `@gitclaw /channel follow-up --route team-demo --loop-id Design.Loop --message-id source-1 --notify-message-id notify-1
Summary: Resolve routed open loop
Context:
Keep this provider thread handy.
Next step:
Ask the release owner for an answer.`,
		},
	}
	req, err := BuildChannelOpenLoopActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelOpenLoopActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "follow-up" || req.Options.Route != "team-demo" || req.Options.LoopID != "design-loop" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel open loop parsing: %#v", req)
	}
	if req.Options.Title != "Resolve routed open loop" || !strings.Contains(req.Options.Context, "Keep this provider thread handy.") || !strings.Contains(req.Options.NextStep, "Ask the release owner for an answer.") {
		t.Fatalf("unexpected open-loop fields: %#v", req)
	}
	if req.TargetFromIssue || req.AutoLoopID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.ContextSHA == "" || req.NextStepSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route open_loop hashes: %#v", req)
	}
}
