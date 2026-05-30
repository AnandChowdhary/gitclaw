package gitclaw

import (
	"strings"
	"testing"
)

func TestRenderRunReportShowsCurrentTurnProvenanceWithoutBodies(t *testing.T) {
	ev, err := ParseEvent("issues", []byte(`{
		"action": "opened",
		"repository": {"full_name": "owner/repo", "default_branch": "main"},
		"issue": {
			"number": 128,
			"title": "@gitclaw /runs",
			"body": "Hidden run body token: RUN_BODY_SECRET.",
			"author_association": "MEMBER",
			"user": {"login": "alice", "type": "User"},
			"labels": [{"name": "gitclaw"}, {"name": "gitclaw:heartbeat"}]
		},
		"sender": {"login": "alice", "type": "User"}
	}`))
	if err != nil {
		t.Fatalf("ParseEvent returned error: %v", err)
	}
	comments := []Comment{
		{
			ID:                10,
			Body:              RenderAssistantComment(Marker{RunID: "old", EventID: "issue-126", Model: "openai/gpt-5-nano", IdempotencyKey: "old"}, "RUN_ASSISTANT_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                11,
			Body:              RenderHeartbeatComment(HeartbeatMarker{RunID: "heartbeat", Slot: "2026-05-30T10:00Z"}, "RUN_HEARTBEAT_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                12,
			Body:              RenderErrorComment(ErrorMarker{RunID: "error", EventID: "issue-127", Phase: "model"}, "RUN_ERROR_SECRET"),
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "MEMBER",
		},
		{
			ID:                13,
			Body:              "<!-- gitclaw:channel-message channel=\"telegram\" message_id=\"123\" -->\nRUN_CHANNEL_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := BuildTranscript(ev, comments)
	repoContext := RepoContext{
		Documents: []ContextDocument{
			{Path: ".gitclaw/SOUL.md", Body: "RUN_SOUL_SECRET"},
		},
		Skills: []ContextDocument{
			{Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Body: "RUN_SKILL_SECRET"},
		},
		SkillSummaries: []SkillSummary{
			{Name: "repo-reader", Path: ".gitclaw/SKILLS/repo-reader/SKILL.md", Enabled: true},
		},
		SkillBundles: []SkillBundleSummary{
			{Name: "repo-context", Path: ".gitclaw/skills.yml"},
		},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.list_files", Input: "RUN_INPUT_SECRET", Output: "RUN_OUTPUT_SECRET"},
		},
	}

	report := RenderRunReport(ev, DefaultConfig(), Preflight(ev, DefaultConfig()), comments, transcript, repoContext, false)
	for _, want := range []string{
		"GitClaw Run Ledger Report",
		"Generated without a model call",
		"repository: `owner/repo`",
		"issue: `#128`",
		"event_kind: `issue_opened`",
		"event_name: `issues`",
		"event_action: `opened`",
		"event_id: `issue-128`",
		"active_command: `/runs`",
		"idempotency_key: `",
		"run_id: `local`",
		"run_attempt: `0`",
		"run_environment_sha256_12: `",
		"run_url_present: `false`",
		"run_url_sha256_12: `",
		"event_sha256_12: `",
		"preflight_allowed: `true`",
		"preflight_code: `allowed`",
		"actor_association: `MEMBER`",
		"actor_trusted: `true`",
		"triggered: `true`",
		"disabled_label_present: `false`",
		"write_request_detected: `false`",
		"raw_comments_before_turn: `4`",
		"transcript_messages: `4`",
		"user_messages: `2`",
		"assistant_messages: `2`",
		"assistant_turn_comments_before_turn: `1`",
		"heartbeat_comments_before_turn: `1`",
		"error_marker_comments_before_turn: `1`",
		"channel_message_comments_before_turn: `1`",
		"context_documents: `1`",
		"selected_skills: `1`",
		"available_skills: `1`",
		"skill_bundles: `1`",
		"active_tool_outputs: `1`",
		"run_ledger_store: `github-issue-comments+actions-run`",
		"backup_branch: `gitclaw-backups`",
		"run_ledger_writes_allowed: `false`",
		"raw_bodies_included: `false`",
		"raw_run_payloads_included: `false`",
		"issue_title_sha256_12: `",
		"### Label State",
		"`gitclaw` present=`true`",
		"`gitclaw:disabled` present=`false`",
		"`gitclaw:heartbeat` present=`true`",
		"### Prompt-Visible Inputs",
		"kind=`context` path=`.gitclaw/SOUL.md`",
		"kind=`skill` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`",
		"### Tool Outputs",
		"name=`gitclaw.list_files` input_sha256_12=`",
		"output_sha256_12=`",
		"### Ledger Notes",
		"issue comments remain the canonical conversation log",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("run report missing %q:\n%s", want, report)
		}
	}
	for _, leaked := range []string{
		"RUN_BODY_SECRET",
		"RUN_ASSISTANT_SECRET",
		"RUN_HEARTBEAT_SECRET",
		"RUN_ERROR_SECRET",
		"RUN_CHANNEL_SECRET",
		"RUN_SOUL_SECRET",
		"RUN_SKILL_SECRET",
		"RUN_INPUT_SECRET",
		"RUN_OUTPUT_SECRET",
		"Hidden run body token",
	} {
		if strings.Contains(report, leaked) {
			t.Fatalf("run report leaked %q:\n%s", leaked, report)
		}
	}
}

func TestIsRunReportRequestAcceptsAliases(t *testing.T) {
	for _, command := range []string{"/runs", "/run", "/ledger"} {
		ev, err := ParseEvent("issues", []byte(`{
			"action": "opened",
			"repository": {"full_name": "owner/repo", "default_branch": "main"},
			"issue": {
				"number": 130,
				"title": "@gitclaw `+command+`",
				"body": "",
				"author_association": "MEMBER",
				"user": {"login": "alice", "type": "User"},
				"labels": [{"name": "gitclaw"}]
			},
			"sender": {"login": "alice", "type": "User"}
		}`))
		if err != nil {
			t.Fatalf("ParseEvent(%s) returned error: %v", command, err)
		}
		if !IsRunReportRequest(ev, DefaultConfig()) {
			t.Fatalf("%s should be accepted as a run report command", command)
		}
	}
}
