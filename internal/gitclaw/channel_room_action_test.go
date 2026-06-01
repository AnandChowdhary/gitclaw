package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelRoomCreatesDurableRoomAndInvitesRoutesWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: e2e-slack-route
    channel: slack
    thread_id_template: slack-room-{route}-{message_id}
    author: gitclaw:test
  - name: e2e-telegram-route
    channel: telegram
    thread_id_template: telegram-room-{route}-{message_id}
    author: gitclaw:test
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 285,
			"title": "Plan the channel room feature",
			"body": "@gitclaw /channels room e2e-slack-route,e2e-telegram-route --room-id room-1 --message-id room-msg-1\n\nTopic: Channel room launch\nNotes:\nVisible room notes with CHANNEL_ROOM_NOTES_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{285: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel room action", llm.Calls)
	}
	if len(github.Issues) != 3 {
		t.Fatalf("created issues = %d, want room plus two channel issues: %#v", len(github.Issues), github.Issues)
	}
	room := github.Issues[0]
	if !HasChannelRoomMarker(room.Body) || !strings.Contains(room.Body, `room_id="room-1"`) {
		t.Fatalf("room issue missing room marker:\n%s", room.Body)
	}
	for _, want := range []string{
		"GitClaw channel room",
		"Channel room launch",
		"Visible room notes with CHANNEL_ROOM_NOTES_SECRET",
		"source_issue: `#285`",
		"room_mode: `durable-issue-channel`",
		"raw_route_names_included: `false`",
	} {
		if !strings.Contains(room.Body, want) {
			t.Fatalf("room issue missing %q:\n%s", want, room.Body)
		}
	}
	if !hasLabel(github.IssueLabels[room.Number], "gitclaw") {
		t.Fatalf("room issue missing gitclaw trigger label: %#v", github.IssueLabels[room.Number])
	}

	for _, issue := range github.Issues[1:] {
		if !HasChannelThreadMarker(issue.Body) {
			t.Fatalf("room target issue missing channel thread marker: %#v", issue)
		}
		comments := github.CommentsByIssue[issue.Number]
		if len(comments) != 1 {
			t.Fatalf("target issue %d comments = %d, want one outbound: %#v", issue.Number, len(comments), comments)
		}
		for _, want := range []string{
			"gitclaw:channel-outbound",
			`message_id="room-msg-1"`,
			"GitClaw channel room",
			"Room: #100",
			"https://github.com/owner/repo/issues/100",
			"Channel room launch",
			"CHANNEL_ROOM_NOTES_SECRET",
		} {
			if !strings.Contains(comments[0].Body, want) {
				t.Fatalf("target issue %d missing outbound room invite %q:\n%s", issue.Number, want, comments[0].Body)
			}
		}
	}

	sourceComments := github.CommentsByIssue[285]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want room receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Room Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels room`",
		"channel_room_status: `queued`",
		"room_issue: `#100`",
		"room_issue_created: `true`",
		"room_id_auto: `false`",
		"message_id_auto: `false`",
		"room_routes: `2`",
		"room_invites_queued: `2`",
		"room_invite_duplicates: `0`",
		"target_issues_created: `2`",
		"raw_route_names_included: `false`",
		"raw_room_id_included: `false`",
		"raw_topic_included: `false`",
		"raw_notes_included: `false`",
		"raw_outbound_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_room_action_change: `true`",
		"channel=`slack`",
		"channel=`telegram`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("room receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_ROOM_NOTES_SECRET", "Channel room launch", "room-1", "room-msg-1", "e2e-slack-route", "e2e-telegram-route"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("room receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 285,
			"title": "Plan the channel room feature",
			"body": "@gitclaw /channels room e2e-slack-route,e2e-telegram-route --room-id room-1 --message-id room-msg-1\n\nTopic: Channel room launch\nNotes:\nVisible room notes with CHANNEL_ROOM_NOTES_SECRET",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 28501,
			"body": "@gitclaw /channels room e2e-slack-route,e2e-telegram-route --room-id room-1 --message-id room-msg-1\n\nTopic: Channel room launch\nNotes:\nDo not leak duplicate secret CHANNEL_ROOM_DUPLICATE_SECRET",
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
	if len(github.Issues) != 3 {
		t.Fatalf("duplicate room created more issues: %#v", github.Issues)
	}
	for _, issue := range github.Issues[1:] {
		if got := len(github.CommentsByIssue[issue.Number]); got != 1 {
			t.Fatalf("duplicate room posted another outbound comment on issue %d: %d", issue.Number, got)
		}
	}
	duplicateReceipt := github.CommentsByIssue[285][1].Body
	for _, want := range []string{
		"channel_room_status: `duplicate`",
		"room_issue_created: `false`",
		"room_invites_queued: `0`",
		"room_invite_duplicates: `2`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate room receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_ROOM_DUPLICATE_SECRET") {
		t.Fatalf("duplicate room receipt leaked body:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelRoomActionRequestParsesTopicAndNotes(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue: Issue{
			Number: 27,
			Title:  "Design review",
		},
		Comment: &Comment{
			ID: 2701,
			Body: `@gitclaw /channels space --route Team-Demo --room-id Design.Room --message-id room-27
Topic: Design review room
Notes:
Sketch the route UX.`,
		},
	}
	req, err := BuildChannelRoomActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelRoomActionRequest returned error: %v", err)
	}
	if req.Subcommand != "space" || req.Options.Routes[0] != "team-demo" || req.Options.RoomID != "design-room" || req.Options.MessageID != "room-27" {
		t.Fatalf("unexpected room target parsing: %#v", req)
	}
	if req.Options.Topic != "Design review room" || !strings.Contains(req.Options.Notes, "Sketch the route UX.") {
		t.Fatalf("unexpected room notes parsing: %#v", req)
	}
	if req.AutoRoomID || req.AutoMessageID || req.TopicSHA == "" || req.NotesSHA == "" || req.RoutesSHA == "" {
		t.Fatalf("expected room hashes and explicit ids: %#v", req)
	}
}
