package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelResearchSpotlightQueuesCatalogCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelResearchSpotlightFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-research-spotlight-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 910,
			"title": "GitClaw telegram thread chat-research-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-research-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91001,
			"body": "@gitclaw /channels research-spotlight openclaw --message-id research-spotlight-inbound-910 --notify-message-id research-spotlight-notify-910 --spotlight-id Research.Spotlight.Secret.910\nDo not include this command hidden token in the receipt: CHANNEL_RESEARCH_SPOTLIGHT_COMMAND_MARKER.",
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
			Number: 910,
			Title:  "GitClaw telegram thread chat-research-spotlight-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{910: {{
			ID: 91000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-research-spotlight-123",
				MessageID: "research-spotlight-inbound-910",
				Author:    "telegram",
				Body:      "Original mirrored research spotlight command with CHANNEL_RESEARCH_SPOTLIGHT_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{910: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel research spotlight action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("research spotlight action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[910]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="research-spotlight-notify-910"`,
		"GitClaw channel research spotlight",
		"Spotlight status: ok",
		"Source snapshot date: 2026-06-01",
		"Focus hash: ",
		"Focus terms: 1",
		"Reviewed sources: 10",
		"Pattern coverage: 11",
		"Rejected patterns: 5",
		"Local research docs: 3",
		"Local research docs present: 3",
		"Research followups indexed: ",
		"Matched items: ",
		"Candidate items: ",
		"Selected index: ",
		"Selection seed hash: ",
		"Selection hash: ",
		"Research spotlight id hash: ",
		"Spotlight:",
		"kind=",
		"Try next:",
		"@gitclaw /research catalog",
		"Raw research notes, source bodies, channel bodies, issue bodies, comment bodies, prompts, tool outputs, raw focus text, raw notes, and raw spotlight ids are not included in the source receipt.",
		"Source fetch: not performed by this action.",
		"Live source browse: not performed by this action.",
		"Model call: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("research spotlight notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_RESEARCH_SPOTLIGHT_INGEST_MARKER", "CHANNEL_RESEARCH_SPOTLIGHT_COMMAND_MARKER", "CHANNEL_RESEARCH_SPOTLIGHT_DOC_SECRET", "Research.Spotlight.Secret.910"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("research spotlight notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Research Spotlight Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels research-spotlight`",
		"channel_research_spotlight_status: `queued`",
		"research_spotlight_status: `ok`",
		"spotlight_mode: `static-research-catalog-deterministic-draw`",
		"notification_target_issue: `#910`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"research_spotlight_id_sha256_12: `",
		"research_spotlight_id_auto: `false`",
		"spotlight_focus_sha256_12: `",
		"spotlight_focus_bytes: `8`",
		"spotlight_focus_terms: `1`",
		"spotlight_focus_source: `positional`",
		"spotlight_note_sha256_12: `",
		"source_snapshot_date: `2026-06-01`",
		"reviewed_sources: `10`",
		"pattern_coverage: `11`",
		"rejected_patterns: `5`",
		"local_research_docs: `3`",
		"local_research_docs_present: `3`",
		"research_followups_indexed: `",
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
		"selection_seed_sha256_12: `",
		"selection_sha256_12: `",
		"notification_body_sha256_12: `",
		"deterministic_selection: `true`",
		"external_randomness_used: `false`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_focus_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_research_spotlight_id_included: `false`",
		"raw_selection_seed_included: `false`",
		"raw_research_source_ids_included: `false`",
		"raw_research_source_urls_included: `false`",
		"raw_research_patterns_included: `false`",
		"raw_research_surfaces_included: `false`",
		"raw_research_bodies_included: `false`",
		"raw_source_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_research_spotlight_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel research spotlight receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"openclaw", "docs.openclaw.ai", "CHANNEL_RESEARCH_SPOTLIGHT_INGEST_MARKER", "CHANNEL_RESEARCH_SPOTLIGHT_COMMAND_MARKER", "CHANNEL_RESEARCH_SPOTLIGHT_DOC_SECRET", "chat-research-spotlight-123", "research-spotlight-inbound-910", "research-spotlight-notify-910", "Research.Spotlight.Secret.910"} {
		if strings.Contains(strings.ToLower(receipt), strings.ToLower(leaked)) {
			t.Fatalf("channel research spotlight receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 910,
			"title": "GitClaw telegram thread chat-research-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-research-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91002,
			"body": "@gitclaw /channels research-draw openclaw --message-id research-spotlight-inbound-910 --notify-message-id research-spotlight-notify-910 --spotlight-id Research.Spotlight.Secret.910\nDo not include duplicate hidden token CHANNEL_RESEARCH_SPOTLIGHT_DUPLICATE_MARKER.",
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
	if got := len(github.CommentsByIssue[910]); got != 4 {
		t.Fatalf("duplicate research spotlight posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[910])
	}
	duplicateReceipt := github.CommentsByIssue[910][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels research-draw`",
		"channel_research_spotlight_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"source_fetch_performed: `false`",
		"live_source_browse_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate research spotlight receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"openclaw", "CHANNEL_RESEARCH_SPOTLIGHT_DUPLICATE_MARKER", "chat-research-spotlight-123", "research-spotlight-inbound-910", "research-spotlight-notify-910", "Research.Spotlight.Secret.910"} {
		if strings.Contains(strings.ToLower(duplicateReceipt), strings.ToLower(leaked)) {
			t.Fatalf("duplicate research spotlight receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelResearchSpotlightActionRequestParsesRouteAliasAndTrailingNote(t *testing.T) {
	root := t.TempDir()
	writeChannelResearchSpotlightFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel research spotlight"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel hermes-spotlight --route team-demo --message-id source-1 --notify-message-id notify-1 --id Research.Spotlight.One --focus cron
Note: compare scheduled fresh sessions.`,
		},
	}
	req, err := BuildChannelResearchSpotlightActionRequest(ev, cfg)
	if err != nil {
		t.Fatalf("BuildChannelResearchSpotlightActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "hermes-spotlight" || req.Options.Route != "team-demo" || req.Options.Focus != "cron" || req.Options.Note != "compare scheduled fresh sessions." || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SpotlightID != "research-spotlight-one" {
		t.Fatalf("unexpected channel research spotlight parsing: %#v", req)
	}
	if req.FocusSource != "flag" || req.NoteSource != "trailing-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSpotlightID {
		t.Fatalf("unexpected channel research spotlight defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SpotlightIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" || req.Report.CandidateItems == 0 || req.Report.SelectedIndex < 0 {
		t.Fatalf("expected route spotlight hashes and selected research candidate: %#v", req)
	}
	if !IsChannelResearchSpotlightActionRequest(ev, cfg) {
		t.Fatalf("expected channel hermes-spotlight alias to be recognized")
	}
}

func writeChannelResearchSpotlightFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "README.md", "Research spotlight fixture with CHANNEL_RESEARCH_SPOTLIGHT_DOC_SECRET.\n")
	writeTestFile(t, root, "docs/spec-github-native-gitclaw.md", "Spec research fixture. Follow-up: keep this body private.\n")
	writeTestFile(t, root, "docs/research-openclaw-hermes-landscape.md", "Landscape research fixture. Follow-up: never print this note body.\n")
}
