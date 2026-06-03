package gitclaw

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleChannelSourceMapQueuesSafeSequenceWithoutLLM(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".gitclaw", "SKILLS", "repo-reader")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skill dir returned error: %v", err)
	}
	skillBody := []byte(`---
name: repo-reader
description: Use read-only repository context.
---
Full skill body secret CHANNEL_SOURCE_MAP_SKILL_BODY_SECRET.
`)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skillBody, 0o644); err != nil {
		t.Fatalf("WriteFile skill returned error: %v", err)
	}
	sourceDir := filepath.Join(root, ".gitclaw", "skill-sources")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll source dir returned error: %v", err)
	}
	sourceBody := `name: repo-reader
skill_path: .gitclaw/SKILLS/repo-reader/SKILL.md
source_kind: repo-local
source_ref: .gitclaw/SKILLS/repo-reader/SKILL.md
trust_level: repo-local
install_mode: manual-review
expected_sha256_12: stale1234567
requires_approval: true
remote_fetch_allowed: false
`
	if err := os.WriteFile(filepath.Join(sourceDir, "repo-reader.yaml"), []byte(sourceBody), 0o644); err != nil {
		t.Fatalf("WriteFile source returned error: %v", err)
	}

	threadBody := RenderChannelThreadBody(ChannelIngestOptions{
		Channel:  "telegram",
		ThreadID: "chat-source-map-123",
	})
	ev, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-source-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-source-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90401,
			"body": "@gitclaw /channels source-map repo-reader --message-id source-map-inbound-904 --notify-message-id source-map-notify-904 --map-id source-map-secret-904\nNote: Keep source pins reviewed\nDo not include this command hidden token in the receipt: CHANNEL_SOURCE_MAP_COMMAND_SECRET.",
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
			Number: 904,
			Title:  "GitClaw telegram thread chat-source-map-123",
			Body:   threadBody,
			Labels: []string{"gitclaw", "gitclaw:channel"},
		}},
		CommentsByIssue: map[int][]Comment{904: {{
			ID: 90400,
			Body: RenderChannelMessageComment(ChannelIngestOptions{
				Channel:   "telegram",
				ThreadID:  "chat-source-map-123",
				MessageID: "source-map-inbound-904",
				Author:    "telegram",
				Body:      "Original mirrored source-map command with CHANNEL_SOURCE_MAP_INGEST_SECRET.",
			}),
		}}},
		IssueLabels: map[int][]string{904: []string{"gitclaw", "gitclaw:channel"}},
	}
	llm := &FakeLLM{Response: "should not be called"}

	if err := Handle(context.Background(), ev, cfg, github, llm); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if llm.Calls != 0 {
		t.Fatalf("LLM called %d times for channel source map action", llm.Calls)
	}
	if len(github.Issues) != 1 {
		t.Fatalf("source map should not create artifact issues: %#v", github.Issues)
	}

	sourceComments := github.CommentsByIssue[904]
	if len(sourceComments) != 3 {
		t.Fatalf("source comments = %d, want message + outbound + receipt: %#v", len(sourceComments), sourceComments)
	}
	outbound := sourceComments[1].Body
	for _, want := range []string{
		"gitclaw:channel-outbound",
		`channel="telegram"`,
		`message_id="source-map-notify-904"`,
		"GitClaw channel skill source map.",
		"Requested source: repo-reader",
		"Skill source status: high",
		"Available source pins: 1",
		"Parsed source pins: 1",
		"Matched source pins: 1",
		"Hash pinned sources: 1",
		"Hash matched sources: 0",
		"Hash mismatched sources: 1",
		"Remote fetch allowed pins: 0",
		"Sources requiring approval: 1",
		"Sources with risk findings: 1",
		"Matched source: repo-reader",
		"Source kind: repo-local",
		"Source ref present: true",
		"Skill matched: true",
		"Trust level: repo-local",
		"Install mode: manual-review",
		"Requires approval: true",
		"Remote fetch allowed: false",
		"Hash pinned: true",
		"Hash matched: false",
		"Hash mismatched: true",
		"Risk findings: 1",
		"Source sequence:",
		"`/skills sources`",
		"`/skills sources info repo-reader`",
		"`/skills sources verify`",
		"`/skills sources lock`",
		"`/skills sources update-plan`",
		"`/skills sources propose repo-reader --source <ref> --id <proposal-id>`",
		"Note: Keep source pins reviewed",
		"Source map hash: ",
		"Source step hash: ",
		"Map source: current GitHub Actions checkout skill-source metadata.",
		"Raw source refs: not included.",
		"Full source bodies: not included.",
		"Full skill bodies: not included.",
		"Registry contact: not performed by this action.",
		"Remote fetch: not performed by this action.",
		"Skill install: not performed by this action.",
		"Skill update: not performed by this action.",
		"Source pin write: not performed by this action.",
		"Installer scripts: not run by this action.",
		"Dependency install: not performed by this action.",
		"Source proposal issue creation: not performed by this action.",
		"Model call: not performed by this action.",
		"Provider API call: not performed by this action.",
		"Workflow mutation: not performed by this action.",
		"Repository mutation: not performed by this action.",
		"Provider delivery: queued through GitHub channel outbox.",
	} {
		if !strings.Contains(outbound, want) {
			t.Fatalf("source-map notification missing %q:\n%s", want, outbound)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOURCE_MAP_INGEST_SECRET", "CHANNEL_SOURCE_MAP_COMMAND_SECRET", "CHANNEL_SOURCE_MAP_SKILL_BODY_SECRET", "source-map-secret-904"} {
		if strings.Contains(outbound, leaked) {
			t.Fatalf("source-map notification leaked %q:\n%s", leaked, outbound)
		}
	}

	receipt := sourceComments[2].Body
	for _, want := range []string{
		"GitClaw Channel Source Map Action",
		"Generated without a model call",
		`model="gitclaw/channels"`,
		"requested_channel_command: `/channels source-map`",
		"channel_source_map_status: `queued`",
		"source_map_mode: `provider-facing-skill-source-sequence`",
		"notification_target_issue: `#904`",
		"notification_comment_id: `9000`",
		"notification_queued: `true`",
		"notification_duplicate_suppressed: `false`",
		"target_from_current_channel_issue: `true`",
		"source_map_id_sha256_12: `",
		"source_map_id_auto: `false`",
		"requested_source_sha256_12: `",
		"normalized_source_sha256_12: `",
		"requested_source_bytes: `11`",
		"requested_source_terms: `1`",
		"source_pin_source: `positional`",
		"source_map_note_sha256_12: `",
		"source_map_note_bytes: `25`",
		"source_map_note_lines: `1`",
		"source_map_note_source: `trailing-note`",
		"source_map_step_count: `6`",
		"source_map_step_sha256_12: `",
		"source_map_snapshot_sha256_12: `",
		"skill_source_status: `high`",
		"available_source_pins: `1`",
		"parsed_source_pins: `1`",
		"matched_source_pins: `1`",
		"missing_skill_matches: `0`",
		"hash_pinned_sources: `1`",
		"hash_matched_sources: `0`",
		"hash_mismatched_sources: `1`",
		"repo_local_source_refs: `1`",
		"remote_source_refs: `0`",
		"sources_requiring_approval: `1`",
		"remote_fetch_allowed_specs: `0`",
		"sources_with_risk_findings: `1`",
		"high_risk_findings: `1`",
		"selected_source_pins: `1`",
		"selected_skill_matched: `1`",
		"selected_hash_pinned: `1`",
		"selected_hash_matched: `0`",
		"selected_hash_mismatched: `1`",
		"selected_requires_approval: `1`",
		"selected_remote_fetch_allowed: `0`",
		"selected_risk_findings: `1`",
		"registry_contact_allowed: `false`",
		"remote_fetch_allowed: `false`",
		"skill_install_allowed: `false`",
		"skill_update_allowed: `false`",
		"source_pin_write_allowed: `false`",
		"installer_scripts_run: `false`",
		"dependency_install_allowed: `false`",
		"source_proposal_issue_created: `false`",
		"model_call_performed: `false`",
		"provider_api_call_performed: `false`",
		"workflow_mutation_performed: `false`",
		"repository_mutation_performed: `false`",
		"provider_delivery_performed: `false`",
		"raw_thread_id_included: `false`",
		"raw_source_message_id_included: `false`",
		"raw_notify_message_id_included: `false`",
		"raw_source_map_id_included: `false`",
		"raw_requested_source_included: `false`",
		"raw_source_map_note_included: `false`",
		"raw_source_map_steps_included: `false`",
		"raw_source_names_included: `false`",
		"raw_source_paths_included: `false`",
		"raw_source_refs_included: `false`",
		"raw_source_bodies_included: `false`",
		"raw_skill_paths_included: `false`",
		"raw_skill_bodies_included: `false`",
		"raw_channel_message_body_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"llm_e2e_required_after_channel_source_map_action_change: `true`",
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("channel source map receipt missing %q:\n%s", want, receipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOURCE_MAP_INGEST_SECRET", "CHANNEL_SOURCE_MAP_COMMAND_SECRET", "CHANNEL_SOURCE_MAP_SKILL_BODY_SECRET", "chat-source-map-123", "source-map-inbound-904", "source-map-notify-904", "source-map-secret-904", "repo-reader", "Keep source pins reviewed", ".gitclaw/SKILLS/repo-reader/SKILL.md"} {
		if strings.Contains(receipt, leaked) {
			t.Fatalf("channel source map receipt leaked %q:\n%s", leaked, receipt)
		}
	}

	duplicateEv, err := ParseEvent("issue_comment", []byte(`{
		"action": "created",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 904,
			"title": "GitClaw telegram thread chat-source-map-123",
			"body": "<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"chat-source-map-123\" -->\nGitClaw channel bridge thread.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:channel"}]
		},
		"comment": {
			"id": 90402,
			"body": "@gitclaw /channels source-path repo-reader --message-id source-map-inbound-904 --notify-message-id source-map-notify-904 --map-id source-map-secret-904\nNote: Keep source pins reviewed\nDo not leak duplicate token CHANNEL_SOURCE_MAP_DUPLICATE_SECRET.",
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
	if len(github.Issues) != 1 {
		t.Fatalf("duplicate source map created artifact issues: %#v", github.Issues)
	}
	if got := len(github.CommentsByIssue[904]); got != 4 {
		t.Fatalf("duplicate source map posted another outbound comment: comments=%d %#v", got, github.CommentsByIssue[904])
	}
	duplicateReceipt := github.CommentsByIssue[904][3].Body
	for _, want := range []string{
		"requested_channel_command: `/channels source-path`",
		"channel_source_map_status: `duplicate`",
		"notification_queued: `false`",
		"notification_duplicate_suppressed: `true`",
		"registry_contact_allowed: `false`",
		"remote_fetch_allowed: `false`",
		"skill_install_allowed: `false`",
		"source_pin_write_allowed: `false`",
		"source_proposal_issue_created: `false`",
		"model_call_performed: `false`",
		"repository_mutation_performed: `false`",
	} {
		if !strings.Contains(duplicateReceipt, want) {
			t.Fatalf("duplicate source map receipt missing %q:\n%s", want, duplicateReceipt)
		}
	}
	for _, leaked := range []string{"CHANNEL_SOURCE_MAP_DUPLICATE_SECRET", "chat-source-map-123", "source-map-inbound-904", "source-map-notify-904", "source-map-secret-904", "repo-reader", "Keep source pins reviewed", ".gitclaw/SKILLS/repo-reader/SKILL.md"} {
		if strings.Contains(duplicateReceipt, leaked) {
			t.Fatalf("duplicate source map receipt leaked %q:\n%s", leaked, duplicateReceipt)
		}
	}
}

func TestBuildChannelSourceMapActionRequestParsesRouteAlias(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, ".gitclaw", "skill-sources")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll source dir returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "repo-reader.yaml"), []byte(`name: repo-reader
skill_path: .gitclaw/SKILLS/repo-reader/SKILL.md
source_kind: repo-local
source_ref: .gitclaw/SKILLS/repo-reader/SKILL.md
trust_level: repo-local
install_mode: manual-review
expected_sha256_12: abc123
requires_approval: true
remote_fetch_allowed: false
`), 0o644); err != nil {
		t.Fatalf("WriteFile source returned error: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Workdir = root
	repoContext := RepoContext{SkillSummaries: []SkillSummary{{
		Name: "repo-reader",
		Path: ".gitclaw/SKILLS/repo-reader/SKILL.md",
		SHA:  "abc123",
	}}}
	ev := Event{
		Kind:      EventIssueComment,
		EventName: "issue_comment",
		Repo:      "owner/repo",
		Issue:     Issue{Number: 42, Title: "Channel source map"},
		Comment: &Comment{
			ID:   4201,
			Body: `@gitclaw /channel source-runbook --route team-demo --source repo-reader --message-id source-1 --notify-message-id notify-1 --map-id map-1 --note reviewed-only`,
		},
	}
	req, err := BuildChannelSourceMapActionRequest(ev, cfg, repoContext)
	if err != nil {
		t.Fatalf("BuildChannelSourceMapActionRequest returned error: %v", err)
	}
	if req.Command != "/channel" || req.Subcommand != "source-runbook" || req.Options.Route != "team-demo" || req.Options.RequestedSource != "repo-reader" || req.Options.SourceMessageID != "source-1" || req.Options.NotifyMessageID != "notify-1" || req.Options.MapID != "map-1" || req.Options.Note != "reviewed-only" {
		t.Fatalf("unexpected channel source map parsing: %#v", req)
	}
	if req.TargetFromIssue || req.AutoNotifyMessageID || req.AutoSourceMessageID || req.AutoMapID || req.RequestedRouteHash == "" || req.MapIDHash == "" || req.RequestedSourceHash == "" || req.StepSHA == "" || req.SnapshotSHA == "" {
		t.Fatalf("expected explicit route source-map hashes: %#v", req)
	}
}
