package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelBoardCardCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-board-card-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 584,
			"title": "GitClaw telegram thread chat-board-card-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-board-card-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 58401,
			"body": "@gitclaw /channels board-card --card-id board-card-1 --lane doing --owner alice --message-id inbound-584 --notify-message-id notify-584\nCard: Ship channel-native board cards\nNotes:\nVisible board card note with CHANNEL_BOARD_CARD_NOTE_SECRET.",
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
			Number: 584,
			Title:  "GitClaw telegram thread chat-board-card-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{584: {{
			ID: 58400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-board-card-123",
				MessageID: "inbound-584",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_BOARD_CARD_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{584: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel board card action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one board card issue: %#v", len(github.Issues), github.Issues)
	}
	card := github.Issues[1]
	if !HasChannelBoardCardMarker(card.Body) || !strings.Contains(card.Body, `board_card_id="board-card-1"`) {
		t.Fatalf("board card issue missing channel-board-card marker:\n%s", card.Body)
	}
	for _, want := range []string{
		"GitClaw channel board card",
		"board_card_id: board-card-1",
		"lane: doing",
		"owner: alice",
		"source_channel: telegram",
		"source_issue: #584",
		"source_message_id_sha256_12:",
		"board_card_mode: github-issue-board-card",
		"repository_mutation_performed: false",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"## Card",
		"Ship channel-native board cards",
		"## Lane",
		"doing",
		"## Owner",
		"alice",
		"## Notes",
		"Visible board card note with CHANNEL_BOARD_CARD_NOTE_SECRET.",
	} {
		if !strings.Contains(card.Body, want) {
			t.Fatalf("board card issue missing %q:\n%s", want, card.Body)
		}
	}
	if strings.Contains(card.Body, "chat-board-card-123") || strings.Contains(card.Body, "inbound-584") || strings.Contains(card.Body, "CHANNEL_BOARD_CARD_INGEST_SECRET") {
		t.Fatalf("board card issue leaked provider IDs or channel body:\n%s", card.Body)
	}
	if !hasLabel(github.IssueLabels[card.Number], "gitclaw") {
		t.Fatalf("board card issue missing gitclaw trigger label: %#v", github.IssueLabels[card.Number])
	}

	sourceComments := github.CommentsByIssue[584]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="notify-584"`,
		"GitClaw channel board card captured.",
		"Board card: #101",
		"https://github.com/owner/repo/issues/101",
		"Title: Ship channel-native board cards",
		"Lane: doing",
		"Owner: alice",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("board card notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_BOARD_CARD_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_BOARD_CARD_INGEST_SECRET") {
		t.Fatalf("board card notification leaked notes or channel body:\n%s", outbound)
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Board Card Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels board-card`",
		"channel_board_card_status: `captured`",
		"board_card_issue: `#101`",
		"board_card_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#584`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"board_card_mode: `github-issue-board-card`",
		"repository_mutation_performed: `false`",
		"raw_board_card_id_included: `false`",
		"raw_board_card_lane_included: `false`",
		"raw_board_card_owner_included: `false`",
		"raw_board_card_title_included: `false`",
		"raw_board_card_notes_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_board_card_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel board card receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_BOARD_CARD_INGEST_SECRET", "CHANNEL_BOARD_CARD_NOTE_SECRET", "Ship channel-native", "board-card-1", "chat-board-card-123", "inbound-584", "notify-584", "doing", "alice"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel board card receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 584,
			"title": "GitClaw telegram thread chat-board-card-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-board-card-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 58402,
			"body": "@gitclaw /channels board-card --card-id board-card-1 --lane doing --owner alice --message-id inbound-584 --notify-message-id notify-584\nCard: Ship channel-native board cards\nNotes:\nDo not leak duplicate token CHANNEL_BOARD_CARD_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate board card created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[584]); got != 4 {
		t.Fatalf("duplicate board card posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[584])
	}
	duplicateReceipt := github.CommentsByIssue[584][3].Body
	for _, want := range []string{
		"channel_board_card_status: `duplicate`",
		"board_card_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate board card receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_BOARD_CARD_DUPLICATE_SECRET") {
		t.Fatalf("duplicate board card receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelBoardCardActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel board card"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel kanban --route team-demo --board-card-id Roadmap.Card --lane in-review --owner bob --message-id source-1 --notify-message-id notify-1
Card: Make channel messages spawn GitHub-native board cards
Context:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable board surface.`,
		},
	}
	req, err := BuildChannelBoardCardActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelBoardCardActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "kanban" || req.Options.Route != "team-demo" || req.Options.BoardCardID != "roadmap-card" || req.Options.Lane != "in-review" || req.Options.Owner != "bob" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel board card parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native board cards" || !strings.Contains(req.Options.Notes, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected board card sections: %#v", req)
	}
	if req.TargetFromIssue || req.AutoBoardCardID || req.AutoNotifyMessageID || req.LaneSHA == "" || req.OwnerSHA == "" || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route board card hashes: %#v", req)
	}
}

func TestIsChannelBoardCardActionFieldsKeepsIdeaAliasesSeparate(t *testing.T) {
	if isChannelBoardCardActionFields([]string{"/channels", "idea"}) {
		t.Fatalf("idea should remain an idea alias, not a board card alias")
	}
	if !isChannelBoardCardActionFields([]string{"/channels", "card"}) {
		t.Fatalf("card should be accepted as a board card alias")
	}
	if !isChannelBoardCardActionFields([]string{"/channels", "board-card"}) {
		t.Fatalf("board-card should be accepted as a board card alias")
	}
}
