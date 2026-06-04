package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestHandleChannelSoulSpotlightQueuesDeterministicCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulSpotlightFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soul-spotlight-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 910,
			"title": "GitClaw telegram thread chat-soul-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91001,
			"body": "@gitclaw /channels soul-spotlight SOUL --message-id soul-spotlight-inbound-910 --notify-message-id soul-spotlight-notify-910 --spotlight-id Soul.Spotlight.Secret.910\nDo not include this command hidden token in the receipt: CHANNEL_SOUL_SPOTLIGHT_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-soul-spotlight-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{910: {{
			ID: 91000,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soul-spotlight-123",
				MessageID: "soul-spotlight-inbound-910",
				Author:    "telegram",
				Body:      "Original mirrored soul spotlight command with CHANNEL_SOUL_SPOTLIGHT_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{910: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul spotlight action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("soul spotlight action should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[910]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="soul-spotlight-notify-910"`,
		"GitClaw channel soul spotlight",
		"Spotlight status: ok",
		"Focus hash: ",
		"Focus terms: 1",
		"Available soul files: 6",
		"Present soul files: 6",
		"Eligible soul files: 6",
		"Matched soul files: 1",
		"Candidate soul files: 1",
		"Selected index: 0",
		"Selection seed hash: ",
		"Selection hash: ",
		"Validation status: ok",
		"Validation errors: 0",
		"Validation warnings: 0",
		"Risk status: ok",
		"Risk findings: 0",
		"Soul spotlight id hash: ",
		"Spotlight:",
		"path=.gitclaw/SOUL.md",
		"category=soul",
		"source=repo-local",
		"present=true",
		"required=true",
		"canonical=true",
		"loaded_for_this_turn=true",
		"sha256_12=",
		"Try next:",
		"@gitclaw /channels soul-info .gitclaw/SOUL.md",
		"@gitclaw /channels soul-search soul",
		"Raw soul, identity, user, memory, tool guidance, heartbeat, channel, issue, comment, prompt, and tool output bodies are not included in the source receipt.",
		"Raw focus text, raw notes, raw spotlight ids, and raw selected paths are not included in the source receipt.",
		"Model call: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Soul write: not performed by this action.",
		"Memory write: not performed by this action.",
		"Registry contact: not performed by this action.",
		"Profile export: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soul spotlight notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_SPOTLIGHT_SOUL_SECRET", "CHANNEL_SOUL_SPOTLIGHT_COMMAND_MARKER", "Soul.Spotlight.Secret.910"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("soul spotlight notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Spotlight Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soul-spotlight`",
		"channel_soul_spotlight_status: `queued`",
		"soul_spotlight_status: `ok`",
		"spotlight_mode: `repo-local-high-authority-deterministic-draw`",
		"notification_target_issue: `#910`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"soul_spotlight_id_sha256_12: `",
		"soul_spotlight_id_auto: `false`",
		"spotlight_focus_sha256_12: `",
		"spotlight_focus_bytes: `4`",
		"spotlight_focus_terms: `1`",
		"spotlight_focus_source: `positional`",
		"spotlight_note_sha256_12: `",
		"spotlight_note_bytes: `0`",
		"available_soul_files: `6`",
		"present_soul_files: `6`",
		"eligible_soul_files: `6`",
		"matched_soul_files: `1`",
		"candidate_soul_files: `1`",
		"selected_index: `0`",
		"selected_soul_path_sha256_12: `",
		"selected_soul_category_sha256_12: `",
		"selected_soul_source_sha256_12: `",
		"selection_seed_sha256_12: `",
		"selection_sha256_12: `",
		"validation_status: `ok`",
		"validation_errors: `0`",
		"validation_warnings: `0`",
		"required_files: `6`",
		"present_required_files: `6`",
		"missing_required_files: `0`",
		"memory_notes: `0`",
		"risk_status: `ok`",
		"risk_findings: `0`",
		"notification_body_sha256_12: `",
		"progressive_disclosure_enabled: `true`",
		"deterministic_selection: `true`",
		"external_randomness_used: `false`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
		"memory_writes_performed: `false`",
		"profile_export_performed: `false`",
		"registry_contact_performed: `false`",
		"provider_delivery_performed: `false`",
		"provider_delivery_strategy: `channel-outbox + channel-delivery`",
		"raw_focus_included: `false`",
		"raw_note_included: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_soul_spotlight_id_included: `false`",
		"raw_selection_seed_included: `false`",
		"raw_soul_file_paths_included: `false`",
		"raw_soul_bodies_included: `false`",
		"raw_identity_bodies_included: `false`",
		"raw_user_bodies_included: `false`",
		"raw_memory_bodies_included: `false`",
		"raw_tool_guidance_bodies_included: `false`",
		"raw_heartbeat_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_soul_spotlight_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul spotlight receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"SOUL", ".gitclaw/SOUL.md", "CHANNEL_SOUL_SPOTLIGHT_SOUL_SECRET", "CHANNEL_SOUL_SPOTLIGHT_INGEST_MARKER", "CHANNEL_SOUL_SPOTLIGHT_COMMAND_MARKER", "chat-soul-spotlight-123", "soul-spotlight-inbound-910", "soul-spotlight-notify-910", "Soul.Spotlight.Secret.910"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul spotlight receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 910,
			"title": "GitClaw telegram thread chat-soul-spotlight-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-spotlight-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91002,
			"body": "@gitclaw /channels soul-draw SOUL --message-id soul-spotlight-inbound-910 --notify-message-id soul-spotlight-notify-910 --spotlight-id Soul.Spotlight.Secret.910\nDo not include duplicate hidden token CHANNEL_SOUL_SPOTLIGHT_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate soul spotlight posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[910])
	}
	duplicateReceipt := github.CommentsByIssue[910][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels soul-draw`",
		"channel_soul_spotlight_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul spotlight receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"SOUL", ".gitclaw/SOUL.md", "CHANNEL_SOUL_SPOTLIGHT_DUPLICATE_MARKER", "chat-soul-spotlight-123", "soul-spotlight-inbound-910", "soul-spotlight-notify-910", "Soul.Spotlight.Secret.910"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soul spotlight receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestHandleChannelSoulDrillQueuesPracticeCardWithoutLLM(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulSpotlightFixture(t, root)

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-soul-drill-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 911,
			"title": "GitClaw telegram thread chat-soul-drill-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-drill-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91101,
			"body": "@gitclaw /channels soul-drill SOUL --message-id soul-drill-inbound-911 --notify-message-id soul-drill-notify-911 --drill-id Soul.Drill.Secret.911\nHidden command token CHANNEL_SOUL_DRILL_COMMAND_MARKER.",
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
			Title:  "GitClaw telegram thread chat-soul-drill-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{911: {{
			ID: 91100,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-soul-drill-123",
				MessageID: "soul-drill-inbound-911",
				Author:    "telegram",
				Body:      "Original mirrored soul drill command with CHANNEL_SOUL_DRILL_INGEST_MARKER.",
			}),
		}}},
		IssueLabels: map[int][]string{911: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel soul drill action", llm.Calls)
	}
	sourceComments := github.CommentsByIssue[911]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`message_id="soul-drill-notify-911"`,
		"GitClaw channel soul drill",
		"Drill status: ok",
		"Focus terms: 1",
		"Candidate soul files: 1",
		"Selection hash: ",
		"Risk status: ok",
		"Soul drill id hash: ",
		"Drill:",
		"path=.gitclaw/SOUL.md",
		"category=soul",
		"notice: name the operating boundary this file should protect.",
		"practice: ask GitClaw a normal question that should use this context.",
		"verify: check the assistant marker for context documents and prompt-visible tools.",
		"next: open a soul rehearsal issue if the context needs more than one turn.",
		"@gitclaw /channels soul-info .gitclaw/SOUL.md",
		"@gitclaw /channels rehearse-soul .gitclaw/SOUL.md",
		"Raw focus text, raw notes, raw drill ids, and raw selected paths are not included in the source receipt.",
		"Model call: not performed by this action.",
		"Tool execution: not performed by this action.",
		"Soul write: not performed by this action.",
		"Memory write: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("soul drill notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOUL_DRILL_SOUL_SECRET", "CHANNEL_SOUL_DRILL_COMMAND_MARKER", "Soul.Drill.Secret.911"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("soul drill notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Soul Drill Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels soul-drill`",
		"channel_soul_spotlight_status: `queued`",
		"soul_spotlight_status: `ok`",
		"soul_card_mode: `repo-local-soul-drill`",
		"soul_card_mode_sha256_12: `",
		"soul_card_mode_bytes: `5`",
		"drill_step_count: `4`",
		"target_from_current_channel_issue: `true`",
		"model_call_performed: `false`",
		"tool_execution_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
		"memory_writes_performed: `false`",
		"raw_soul_card_mode_included: `false`",
		"raw_soul_file_paths_included: `false`",
		"raw_soul_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"llm_e2e_required_after_channel_soul_spotlight_change: `true`",
		"llm_e2e_required_after_channel_soul_drill_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel soul drill receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{".gitclaw/SOUL.md", "CHANNEL_SOUL_DRILL_SOUL_SECRET", "CHANNEL_SOUL_DRILL_INGEST_MARKER", "CHANNEL_SOUL_DRILL_COMMAND_MARKER", "chat-soul-drill-123", "soul-drill-inbound-911", "soul-drill-notify-911", "Soul.Drill.Secret.911"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel soul drill receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 911,
			"title": "GitClaw telegram thread chat-soul-drill-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-soul-drill-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 91102,
			"body": "@gitclaw /channels soul-practice SOUL --message-id soul-drill-inbound-911 --notify-message-id soul-drill-notify-911 --practice-id Soul.Drill.Secret.911\nDo not include duplicate hidden token CHANNEL_SOUL_DRILL_DUPLICATE_MARKER.",
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
		t.Fatalf("duplicate soul drill posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[911])
	}
	duplicateReceipt := github.CommentsByIssue[911][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels soul-practice`",
		"channel_soul_spotlight_status: `duplicate`",
		"soul_card_mode: `repo-local-soul-drill`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
		"soul_writes_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate soul drill receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{".gitclaw/SOUL.md", "CHANNEL_SOUL_DRILL_DUPLICATE_MARKER", "chat-soul-drill-123", "soul-drill-inbound-911", "soul-drill-notify-911", "Soul.Drill.Secret.911"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate soul drill receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSoulSpotlightActionRequestParsesRouteAliasAndTrailingNote(t *testing.T) {
	root := t.TempDir()
	writeChannelSoulSpotlightFixture(t, root)
	cfg := DefaultConfig()
	cfg.Workdir = root
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel soul spotlight"},
		Comment: &Comment{
			ID: 4201,
			Body: `@gitclaw /channel context-draw --route team-demo --message-id source-1 --notify-message-id notify-1 --id Soul.Spotlight.One --focus identity
Note: try identity context.`,
		},
	}
	repoContext, err := LoadRepoContext(root, []TranscriptMessage{{Role: "user", Body: "@gitclaw /channel context-draw"}})
	if err != nil {
		t.Fatalf("LoadRepoContext returned error: %v", err)
	}
	req, err := BuildChannelSoulSpotlightActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSoulSpotlightActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "context-draw" || req.Options.Route != "team-demo" || req.Options.Focus != "identity" || req.Options.Note != "try identity context." || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.SpotlightID != "soul-spotlight-one" {
		t.Fatalf("unexpected channel soul spotlight parsing: %#v", req)
	}
	if req.FocusSource != "flag" || req.NoteSource != "trailing-note" || req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoSpotlightID {
		t.Fatalf("unexpected channel soul spotlight defaults: %#v", req)
	}
	if req.RequestedRouteHash == "" || req.SpotlightIDHash == "" || req.FocusSHA == "" || req.NoteSHA == "" || req.NotificationBodySHA == "" || req.Report.CandidateSoulFiles != 1 || req.Report.SelectedIndex != 0 || req.Report.SelectedMatch.Path != ".gitclaw/IDENTITY.md" {
		t.Fatalf("expected route soul spotlight hashes and selected context: %#v", req)
	}
	if !IsChannelSoulSpotlightActionRequest(ev, cfg) {
		t.Fatalf("expected channel context-draw alias to be recognized")
	}
}

func writeChannelSoulSpotlightFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, ".gitclaw/SOUL.md", "# Soul\n\n- Repo-native operating boundary. Hidden body token CHANNEL_SOUL_SPOTLIGHT_SOUL_SECRET.\n")
	writeTestFile(t, root, ".gitclaw/IDENTITY.md", "# Identity\n\n- Identity context.\n")
	writeTestFile(t, root, ".gitclaw/USER.md", "# User\n\n- User preference.\n")
	writeTestFile(t, root, ".gitclaw/TOOLS.md", "# Tools\n\n- Tool guidance.\n")
	writeTestFile(t, root, ".gitclaw/MEMORY.md", "# Memory\n\n- Durable memory index.\n")
	writeTestFile(t, root, ".gitclaw/HEARTBEAT.md", "# Heartbeat\n\n- Heartbeat notes.\n")
}
