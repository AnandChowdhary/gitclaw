package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelResearchMapQueuesSequenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelResearchMapFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-research-map-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 911,
			"title": "GitClaw telegram thread chat-research-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-research-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91101,
			"body": "@gitclaw /channels research-map openclaw --message-id research-map-inbound-911 --notify-message-id research-map-notify-911 --map-id Research.Map.Secret.911\nNote: Keep upstream mapping reviewed\nDo not include this command hidden token in the receipt: CHANNEL_RESEARCH_MAP_COMMAND_MARKER.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"}
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	github := &FakeGitHub{
		Issues: []Issue{{
			Number: 911,
			Title:  "GitClaw telegram thread chat-research-map-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{911: {{
			ID: 91100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-research-map-123",
				MessageID: "research-map-inbound-911",
				Author:    "telegram",
				Body:      "Original mirrored research map command with CHANNEL_RESEARCH_MAP_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{911: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel research map action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("research map action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[911]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="research-map-notify-911"`,
		"GitClaw channel research map",
		"Map status: ok",
		"Source snapshot date: 2026-06-01",
		"Focus hash: ",
		"Focus terms: 1",
		"Reviewed sources: 10",
		"Pattern coverage: 11",
		"Rejected patterns: 5",
		"Matched items: ",
		"Candidate items: ",
		"Selected index: ",
		"Step count: 6",
		"Step hash: ",
		"Selection seed hash: ",
		"Selection hash: ",
		"Research map id hash: ",
		"Selected research item:",
		"kind=",
		"Research sequence:",
		"`@gitclaw /research catalog` - review the body-free source and pattern catalog",
		"`@gitclaw /channels research-spotlight openclaw --message-id <id> --notify-message-id <id>`",
		"`@gitclaw /channels compass openclaw --compass-id <id> --message-id <id> --notify-message-id <id>`",
		"`@gitclaw /channels coach research --coach-id <id> --message-id <id> --notify-message-id <id>`",
		"`@gitclaw /channels palette research --palette-id <id> --message-id <id> --notify-message-id <id>`",
		"Note: Keep upstream mapping reviewed",
		"Note hash: ",
		"Map source: reviewed static research catalog snapshot 2026-06-01.",
		"Source fetch: not performed by this action.",
		"Live source browse: not performed by this action.",
		"Model call: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("research map notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_RESEARCH_MAP_INGEST_MARKER", "CHANNEL_RESEARCH_MAP_COMMAND_MARKER", "CHANNEL_RESEARCH_MAP_DOC_SECRET", "Research.Map.Secret.911"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("research map notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Research Map Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels research-map`",
		"channel_research_map_status: `queued`",
		"research_map_status: `ok`",
		"research_map_mode: `static-research-catalog-safety-sequence`",
		"notification_target_issue: `#911`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"research_map_id_sha256_12: `",
		"research_map_id_auto: `false`",
		"research_focus_sha256_12: `",
		"research_focus_bytes: `8`",
		"research_focus_terms: `1`",
		"research_focus_source: `positional`",
		"research_map_note_sha256_12: `",
		"research_map_note_bytes: `30`",
		"research_map_note_lines: `1`",
		"research_map_note_source: `trailing-note`",
		"source_snapshot_date: `2026-06-01`",
		"reviewed_sources: `10`",
		"pattern_coverage: `11`",
		"rejected_patterns: `5`",
		"local_research_docs: `3`",
		"local_research_docs_present: `3`",
		"matched_research_items: `",
		"candidate_research_items: `",
		"selected_index: `",
		"selected_research_kind_sha256_12: `",
		"selected_research_id_sha256_12: `",
		"selected_research_system_sha256_12: `",
		"selected_research_url_sha256_12: `",
		"selected_research_pattern_sha256_12: `",
		"selected_research_surface_sha256_12: `",
		"selected_research_gate_sha256_12: `",
		"research_map_step_count: `6`",
		"research_map_step_sha256_12: `",
		"selection_seed_sha256_12: `",
		"selection_sha256_12: `",
		"notification_body_sha256_12: `",
		"deterministic_selection: `true`",
		"external_randomness_used: `false`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_focus_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_research_map_id_included: `false`",
		"raw_selection_seed_included: `false`",
		"raw_research_source_ids_included: `false`",
		"raw_research_source_urls_included: `false`",
		"raw_research_patterns_included: `false`",
		"raw_research_surfaces_included: `false`",
		"raw_research_map_steps_included: `false`",
		"raw_research_bodies_included: `false`",
		"raw_source_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_research_map_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel research map receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"openclaw", "CHANNEL_RESEARCH_MAP_INGEST_MARKER", "CHANNEL_RESEARCH_MAP_COMMAND_MARKER", "CHANNEL_RESEARCH_MAP_DOC_SECRET", "chat-research-map-123", "research-map-inbound-911", "research-map-notify-911", "Research.Map.Secret.911", "Keep upstream mapping reviewed"} {
		if strings.Contains(strings.ToLower(receipt), strings.ToLower(leaked)) {
			t.Fatalf("channel research map receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 911,
			"title": "GitClaw telegram thread chat-research-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-research-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91102,
			"body": "@gitclaw /channels research-path openclaw --message-id research-map-inbound-911 --notify-message-id research-map-notify-911 --map-id Research.Map.Secret.911\nNote: Keep upstream mapping reviewed\nDo not include duplicate hidden token CHANNEL_RESEARCH_MAP_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[911]); got != 4 {
		t.Fatalf("duplicate research map posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[911])
	}
	duplicateReceipt := github.CommentsByIssue[911][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels research-path`",
		"channel_research_map_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate research map receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"openclaw", "CHANNEL_RESEARCH_MAP_DUPLICATE_MARKER", "chat-research-map-123", "research-map-inbound-911", "research-map-notify-911", "Research.Map.Secret.911", "Keep upstream mapping reviewed"} {
		if strings.Contains(strings.ToLower(duplicateReceipt), strings.ToLower(leaked)) {
			t.Fatalf("duplicate research map receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelResearchMapActionRequestParsesRouteAliasAndTrailingNote(t *testing.T) {
	root := t.TempDir()
	writeChannelResearchMapFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel research map"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel hermes-map --route team-demo --message-id source-1 --notify-message-id notify-1 --id Research.Map.One --focus cron
Note: compare scheduled fresh sessions.`,
		},
	}
	req, err := BuildChannelResearchMapActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelResearchMapActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "hermes-map" || req.Options.Route != "team-demo" || req.Options.Focus != "cron" || req.Options.Note != "compare scheduled fresh sessions." || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MapID != "research-map-one" {
		t.Fatalf("unexpected channel research map parsing: %#v", req)
	}
	if req.FocusSource != "flag" || req.NoteSource != "trailing-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMapID {
		t.Fatalf("unexpected channel research map defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.MapIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" || req.Report.CandidateItems == 0 || req.Report.SelectedIndex < 0 || req.Report.StepCount != 6 || req.Report.StepHash == "" {
		t.Fatalf("expected route map hashes and selected research candidate: %#v", req)
	}
	if !IsChannelResearchMapActionRequest(ev, cfg) {
		t.Fatalf("expected channel hermes-map alias to be recognized")
	}
}

func writeChannelResearchMapFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "README.md", "Research map fixture with CHANNEL_RESEARCH_MAP_DOC_SECRET.\n")
	writeTestFile(t, root, "docs/spec-github-native-gitclaw.md", "Spec research map fixture. Follow-up: keep this body private.\n")
	writeTestFile(t, root, "docs/research-openclaw-hermes-landscape.md", "Landscape research map fixture. Follow-up: never print this note body.\n")
}
