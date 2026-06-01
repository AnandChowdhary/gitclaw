package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelProbeQueuesRouteTestWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, channelRoutesPath, `routes:
  - name: team-probe
    channel: slack
    thread_id_template: probe-thread-{message_id}
    author: gitclaw:e2e
`)
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 290,
			"title": "Probe channel route",
			"body": "@gitclaw /channels probe team-probe --message-id probe-1\n\nDo not leak source token CHANNEL_PROBE_SOURCE_SECRET.",
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
	github := &FakeGitHub{CommentsByIssue: map[int][]Comment{290: nil}}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel probe action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("created issues = %d, want one channel issue: %#v", len(github.Issues), github.Issues)
	}
	target := github.Issues[0]
	if !HasChannelThreadMarker(target.Body) || !strings.Contains(target.Body, `channel="slack"`) || !strings.Contains(target.Body, `thread_id="probe-thread-probe-1"`) {
		t.Fatalf("target issue missing channel thread marker:\n%s", target.Body)
	}
	if hasLabel(github.IssueLabels[target.Number], cfg.TriggerLabel) {
		t.Fatalf("probe target should not get model trigger label: %#v", github.IssueLabels[target.Number])
	}
	if !hasLabel(github.IssueLabels[target.Number], cfg.ChannelLabel) {
		t.Fatalf("probe target missing channel label: %#v", github.IssueLabels[target.Number])
	}
	targetComments := github.CommentsByIssue[target.Number]
	if len(targetComments) != 1 {
		t.Fatalf("target comments = %d, want one outbound probe: %#v", len(targetComments), targetComments)
	}
	outbound := targetComments[0].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="slack"`,
		`thread_id="probe-thread-probe-1"`,
		`message_id="probe-1"`,
		"GitClaw channel route probe",
		"source_issue: #290",
		"generated_without_model_call: true",
		"provider_delivery_strategy: channel-outbox + channel-delivery",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("outbound probe missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROBE_SOURCE_SECRET", "Do not leak source token"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("outbound probe leaked %q:\n%s", leaked, outbound)
		}
	}

	sourceComments := github.CommentsByIssue[290]
	if len(sourceComments) != 1 {
		t.Fatalf("source comments = %d, want probe receipt: %#v", len(sourceComments), sourceComments)
	}
	receipt := sourceComments[0].Body
	for _, want := range []string{
		"GitClaw Channel Probe Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels probe`",
		"channel_probe_status: `queued`",
		"target_issue_created: `true`",
		"duplicate_suppressed: `false`",
		"route_resolved: `true`",
		"channel: `slack`",
		"message_id_auto: `false`",
		"raw_route_names_included: `false`",
		"raw_thread_ids_included: `false`",
		"raw_message_ids_included: `false`",
		"raw_source_body_included: `false`",
		"raw_probe_body_included: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"llm_e2e_required_after_channel_probe_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("probe receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROBE_SOURCE_SECRET", "team-probe", "probe-1", "probe-thread-probe-1", "Do not leak source token"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("probe receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 290,
			"title": "Probe channel route",
			"body": "@gitclaw /channels probe team-probe --message-id probe-1\n\nDo not leak source token CHANNEL_PROBE_SOURCE_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}]
		},
		"comment": {
			"id": 29001,
			"body": "@gitclaw /channels probe team-probe --message-id probe-1\n\nDo not leak duplicate token CHANNEL_PROBE_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 1 || len(github.CommentsByIssue[target.Number]) != 1 {
		t.Fatalf("duplicate probe should not create another issue/comment: issues=%#v comments=%#v", github.Issues, github.CommentsByIssue[target.Number])
	}
	duplicateReceipt := github.CommentsByIssue[290][1].Body
	for _, want := range []string{
		"channel_probe_status: `duplicate`",
		"target_issue_created: `false`",
		"duplicate_suppressed: `true`",
		"raw_source_body_included: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate probe receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_PROBE_DUPLICATE_SECRET", "team-probe", "probe-1"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate probe receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelProbeActionRequestParsesRouteOptionAndAutoID(t *testing.T) {
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 31, Title: "Probe"},
		Comment: &Comment{
			ID:   3101,
			Body: "@gitclaw /channels ping --route Team-Probe\nOperator note that must stay hashed.",
		},
	}
	req, err := BuildChannelProbeActionRequest(ev, DefaultConfig())
	if err != nil {
		t.Fatalf("BuildChannelProbeActionRequest returned error: %v", err)
	}
	if req.Subcommand != "ping" || req.Options.Route != "team-probe" || !req.AutoMessageID {
		t.Fatalf("unexpected channel probe parsing: %#v", req)
	}
	if !strings.HasPrefix(req.Options.MessageID, "gitclaw-probe-comment-3101-") {
		t.Fatalf("unexpected auto message id: %q", req.Options.MessageID)
	}
	if req.OperatorNoteSHA == "" || req.OperatorNoteBytes == 0 || req.SourceKind != "comment" || req.SourceCommentID != 3101 {
		t.Fatalf("expected hashed note and comment metadata: %#v", req)
	}
	if strings.Contains(req.Options.Body, "Operator note") {
		t.Fatalf("probe body included operator note:\n%s", req.Options.Body)
	}
}
