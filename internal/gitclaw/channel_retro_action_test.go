package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRetroCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-retro-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-retro-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-retro-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38401,
			"body": "@gitclaw /channels retro --retro-id retro-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness retro\nWent well:\nVisible went-well token CHANNEL_RETRO_WENT_WELL_SECRET.\nRough edges:\nVisible rough-edge token CHANNEL_RETRO_ROUGH_SECRET.\nNext:\nVisible next token CHANNEL_RETRO_NEXT_SECRET.",
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
			Number: 384,
			Title:  "GitClaw telegram thread chat-retro-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{384: {{
			ID: 38400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-retro-123",
				MessageID: "inbound-384",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_RETRO_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{384: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel retro action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one retro issue: %#v", len(github.Issues), github.Issues)
	}
	retro := github.Issues[1]
	if !HasChannelRetroMarker(retro.Body) || !strings.Contains(retro.Body, `retro_id="retro-1"`) {
		t.Fatalf("retro issue missing channel-retro marker:\n%s", retro.Body)
	}
	for _, want := range []string{
		"GitClaw channel retrospective",
		"retro_id: retro-1",
		"source_channel: telegram",
		"source_issue: #384",
		"source_message_id_sha256_12:",
		"retro_mode: github-issue-retro",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Title",
		"Launch readiness retro",
		"## Went Well",
		"Visible went-well token CHANNEL_RETRO_WENT_WELL_SECRET.",
		"## Rough Edges",
		"Visible rough-edge token CHANNEL_RETRO_ROUGH_SECRET.",
		"## Next",
		"Visible next token CHANNEL_RETRO_NEXT_SECRET.",
	} {
		if !strings.Contains(retro.Body, want) {
			t.Fatalf("retro issue missing %q:\n%s", want, retro.Body)
		}
	}
	if strings.Contains(retro.Body, "chat-retro-123") || strings.Contains(retro.Body, "inbound-384") || strings.Contains(retro.Body, "CHANNEL_RETRO_INGEST_SECRET") {
		t.Fatalf("retro issue leaked provider IDs or channel body:\n%s", retro.Body)
	}
	if !hasLabel(github.IssueLabels[retro.Number], "gitclaw") {
		t.Fatalf("retro issue missing gitclaw trigger label: %#v", github.IssueLabels[retro.Number])
	}

	sourceComments := github.CommentsByIssue[384]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-384"`,
		"GitClaw channel retro recorded.",
		"Retro: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Launch readiness retro",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("retro notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_RETRO_WENT_WELL_SECRET", "CHANNEL_RETRO_ROUGH_SECRET", "CHANNEL_RETRO_NEXT_SECRET", "CHANNEL_RETRO_INGEST_SECRET"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("retro notification leaked section or channel body %q:\n%s", leaked, outbound)
		}
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Retro Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels retro`",
		"channel_retro_status: `recorded`",
		"retro_issue: `#101`",
		"retro_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#384`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_retro_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_retro_title_included: `false`",
		"raw_retro_went_well_included: `false`",
		"raw_retro_rough_edges_included: `false`",
		"raw_retro_next_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_retro_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel retro receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_RETRO_INGEST_SECRET", "CHANNEL_RETRO_WENT_WELL_SECRET", "CHANNEL_RETRO_ROUGH_SECRET", "CHANNEL_RETRO_NEXT_SECRET", "Launch readiness retro", "retro-1", "chat-retro-123", "inbound-384", "notify-384"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel retro receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 384,
			"title": "GitClaw telegram thread chat-retro-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-retro-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 38402,
			"body": "@gitclaw /channels retro --retro-id retro-1 --message-id inbound-384 --notify-message-id notify-384\nTitle: Launch readiness retro\nWent well:\nDo not leak duplicate token CHANNEL_RETRO_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate retro created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[384]); got != 4 {
		t.Fatalf("duplicate retro posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[384])
	}
	duplicateReceipt := github.CommentsByIssue[384][3].Body
	for _, want := range []string{
		"channel_retro_status: `duplicate`",
		"retro_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate retro receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_RETRO_DUPLICATE_SECRET") {
		t.Fatalf("duplicate retro receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelRetroActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel retro"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel aar --route team-demo --retro-id Weekly.Retro --message-id source-1 --notify-message-id notify-1
Retro: The channel reached launch readiness
Wins:
- Design is stable.
Blockers:
- Release checklist moved late.
Actions:
- Follow-up moves to GitHub.`,
		},
	}
	req, err := BuildChannelRetroActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRetroActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "aar" || req.Options.Route != "team-demo" || req.Options.RetroID != "weekly-retro" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel retro parsing: %#v", req)
	}
	if req.Options.Title != "The channel reached launch readiness" || !strings.Contains(req.Options.WentWell, "Design is stable") || !strings.Contains(req.Options.RoughEdges, "Release checklist") || !strings.Contains(req.Options.Next, "Follow-up moves") {
		t.Fatalf("unexpected retro sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoRetroID || req.AutoNotifyMessageID || req.TitleSHA == "" || req.WentWellSHA == "" || req.RoughEdgesSHA == "" || req.NextSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route retro hashes: %#v", req)
	}
}

func TestIsChannelRetroActionFieldsKeepsDigestAliasesSeparate(t *testing.T) {
	if isChannelRetroActionFields([]string{"/channels", "summary"}) {
		t.Fatalf("summary should remain a digest alias, not a retro alias")
	}
	if !isChannelRetroActionFields([]string{"/channels", "retrospective"}) {
		t.Fatalf("retrospective should be accepted as a retro alias")
	}
}
