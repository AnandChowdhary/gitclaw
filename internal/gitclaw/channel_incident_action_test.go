package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelIncidentCreatesIssueAndNotifiesThreadWithoutLLM(t *testing.T) {
	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-incident-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-incident-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-incident-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48401,
			"body": "@gitclaw /channels incident --incident-id incident-1 --severity sev1 --message-id inbound-484 --notify-message-id notify-484\nIncident: Investigate channel-native incident intake\nNotes:\nVisible incident note with CHANNEL_INCIDENT_NOTE_SECRET.",
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
			Title:  "GitClaw telegram thread chat-incident-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{484: {{
			ID: 48400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-incident-123",
				MessageID: "inbound-484",
				Author:    "telegram",
				Body:      "Original mirrored message with CHANNEL_INCIDENT_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{484: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel incident action", llm.Calls)
	}
	if len(github.Issues) != 2 {
		t.Fatalf("created issues = %d, want one incident issue: %#v", len(github.Issues), github.Issues)
	}
	incident := github.Issues[1]
	if !HasChannelIncidentMarker(incident.Body) || !strings.Contains(incident.Body, `incident_id="incident-1"`) {
		t.Fatalf("incident issue missing channel-incident marker:\n%s", incident.Body)
	}
	for _, want := range []string{
		"GitClaw channel incident",
		"incident_id: incident-1",
		"severity: sev1",
		"source_channel: telegram",
		"source_issue: #484",
		"source_message_id_sha256_12:",
		"incident_mode: github-issue-incident",
		"raw_thread_id_included: false",
		"raw_source_message_id_included: false",
		"Investigate channel-native incident intake",
		"Visible incident note with CHANNEL_INCIDENT_NOTE_SECRET.",
	} {
		if !strings.Contains(incident.Body, want) {
			t.Fatalf("incident issue missing %q:\n%s", want, incident.Body)
		}
	}
	if strings.Contains(incident.Body, "chat-incident-123") || strings.Contains(incident.Body, "inbound-484") || strings.Contains(incident.Body, "CHANNEL_INCIDENT_INGEST_SECRET") {
		t.Fatalf("incident issue leaked provider IDs or channel body:\n%s", incident.Body)
	}
	if !hasLabel(github.IssueLabels[incident.Number], "gitclaw") {
		t.Fatalf("incident issue missing gitclaw trigger label: %#v", github.IssueLabels[incident.Number])
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
		"GitClaw channel incident captured.",
		"Incident: #101",
		"https://github.com/owner/repo/issues/101",
		"Severity: sev1",
		"Title: Investigate channel-native incident intake",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("incident notification missing %q:\n%s", want, outbound)
		}
	}
	if strings.Contains(outbound, "CHANNEL_INCIDENT_NOTE_SECRET") || strings.Contains(outbound, "CHANNEL_INCIDENT_INGEST_SECRET") {
		t.Fatalf("incident notification leaked notes or channel body:\n%s", outbound)
	}
	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Incident Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels incident`",
		"channel_incident_status: `captured`",
		"incident_issue: `#101`",
		"incident_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"notification_target_issue: `#484`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"raw_incident_id_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_incident_severity_included: `false`",
		"raw_incident_title_included: `false`",
		"raw_incident_notes_included: `false`",
		"raw_channel_message_body_included: `false`",
		"provider_delivery_performed: `false`",
		"llm_e2e_required_after_channel_incident_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel incident receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_INCIDENT_INGEST_SECRET", "CHANNEL_INCIDENT_NOTE_SECRET", "Investigate channel-native", "incident-1", "sev1", "chat-incident-123", "inbound-484", "notify-484"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel incident receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 484,
			"title": "GitClaw telegram thread chat-incident-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-incident-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 48402,
			"body": "@gitclaw /channels incident --incident-id incident-1 --severity sev1 --message-id inbound-484 --notify-message-id notify-484\nIncident: Investigate channel-native incident intake\nNotes:\nDo not leak duplicate token CHANNEL_INCIDENT_DUPLICATE_SECRET.",
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
		t.Fatalf("duplicate incident created more issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[484]); got != 4 {
		t.Fatalf("duplicate incident posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[484])
	}
	duplicateReceipt := github.CommentsByIssue[484][3].Body
	for _, want := range []string{
		"channel_incident_status: `duplicate`",
		"incident_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate incident receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	if strings.Contains(duplicateReceipt, "CHANNEL_INCIDENT_DUPLICATE_SECRET") {
		t.Fatalf("duplicate incident receipt leaked source:\n%s", duplicateReceipt)
	}
}

func TestBuildChannelIncidentActionRequestParsesRouteAlias(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Channel incident"},
		Comment: &Comment{
			ID: 3101,
			Body: `@gitclaw /channel escalate --route team-demo --incident-id Roadmap.Spark --severity SEV2 --message-id source-1 --notify-message-id notify-1
Title: Make channel messages spawn GitHub-native incident rooms
Context:
- Keep Slack/Telegram lightweight.
- Let GitHub become the durable shaping surface.`,
		},
	}
	req, err := BuildChannelIncidentActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelIncidentActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "escalate" || req.Options.Route != "team-demo" || req.Options.IncidentID != "roadmap-spark" || req.Options.Severity != "sev2" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" {
		t.Fatalf("unexpected channel incident parsing: %#v", req)
	}
	if req.Options.Title != "Make channel messages spawn GitHub-native incident rooms" || !strings.Contains(req.Options.Notes, "Keep Slack/Telegram lightweight") {
		t.Fatalf("unexpected title/notes: %#v", req)
	}
	if req.TargetFromIssue || req.AutoIncidentID || req.AutoNotifyMessageID || req.SeveritySHA == "" || req.TitleSHA == "" || req.NotesSHA == "" || req.RequestedRouteHash == "" {
		t.Fatalf("expected explicit route incident hashes: %#v", req)
	}
}
