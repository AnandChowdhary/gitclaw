package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRenderSessionCatalogReportListsBodyFreeSessionSurface(t *testing.T) {
	ev := Event{
		Repo: "owner/repo",
		Issue: Issue{
			Number: 88,
			Title:  "@gitclaw /session catalog",
			Body:   "SESSION_CATALOG_BODY_SECRET",
		},
	}
	comments := []Comment{{
		ID:   21,
		Body: "<!-- gitclaw:assistant-turn idempotency_key=old -->\nSESSION_CATALOG_COMMENT_SECRET",
		User: User{Login: "github-actions[bot]", Type: "Bot"},
	}}
	transcript := []TranscriptMessage{
		{Role: "user", Body: "SESSION_CATALOG_TRANSCRIPT_SECRET"},
		{Role: "assistant", Body: "SESSION_CATALOG_ASSISTANT_SECRET"},
	}

	body := RenderSessionReport(ev, DefaultConfig(), comments, transcript)
	for _, want := range []string{
		"GitClaw Session Catalog Report",
		"requested_session_command: `catalog`",
		"session_command_status: `ok`",
		"session_catalog_status: `ok`",
		"catalog_strategy: `compact-issue-thread-session-discovery`",
		"session_model: `github-issue-thread-plus-backup-json`",
		"canonical_session_store: `github-issue-thread`",
		"local_backup_store: `gitclaw-backups issue JSON`",
		"catalog_entries: `9`",
		"issue_side_commands: `9`",
		"local_backup_commands: `8`",
		"raw_bodies_included: `false`",
		"raw_tool_outputs_included: `false`",
		"repository_mutation_allowed: `false`",
		"session_deletion_allowed: `false`",
		"session_export_allowed_issue_side: `false`",
		"llm_e2e_required_after_session_catalog_change: `true`",
		"command=`catalog` issue_intent=`@gitclaw /session catalog` local_command=`gitclaw session catalog` execution=`metadata-only` gate=`body-free-output` raw_bodies_included=`false` mutation_allowed=`false`",
		"command=`provenance` issue_intent=`@gitclaw /session provenance` local_command=`gitclaw session provenance --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`assistant-turn-marker-prompt-context`",
		"command=`tools` issue_intent=`@gitclaw /session tools` local_command=`gitclaw session tools --backup <issue.json>` execution=`current-issue-or-local-backup` gate=`assistant-turn-marker-tool-context`",
		"command=`search` issue_intent=`@gitclaw /session search <query>` local_command=`gitclaw session search <query> --backup <issue.json>`",
		"issue_thread_gate=`canonical-session-is-github-issue-thread`",
		"provenance_gate=`assistant-turn-marker-prompt-context`",
		"tools_gate=`assistant-turn-marker-tool-context`",
		"search_gate=`query-hash-and-line-hash-metadata`",
		"coverage_gate=`prompt-provenance-skill-tool-telemetry`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session catalog report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_CATALOG_BODY_SECRET", "SESSION_CATALOG_COMMENT_SECRET", "SESSION_CATALOG_TRANSCRIPT_SECRET", "SESSION_CATALOG_ASSISTANT_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session catalog report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionSearchReportFindsTranscriptWithoutBodies(t *testing.T) {
	transcript := []TranscriptMessage{
		{
			Role:              "user",
			Body:              "Please remember deployment window SESSION_SEARCH_ISSUE_SECRET.",
			Actor:             "alice",
			AuthorAssociation: "MEMBER",
			Trusted:           true,
		},
		{
			Role:              "assistant",
			Body:              "Deployment window noted with SESSION_SEARCH_ASSISTANT_SECRET.",
			Actor:             "github-actions[bot]",
			AuthorAssociation: "NONE",
			CommentID:         42,
			Trusted:           true,
		},
	}
	body := RenderSessionSearchReport(Event{}, transcript, "deployment SESSION_SEARCH_QUERY_SECRET", 1)
	for _, want := range []string{
		"GitClaw Session Search Report",
		"scope: `local-cli`",
		"session_search_status: `ok`",
		"query_sha256_12:",
		"query_terms:",
		"max_results: `1`",
		"transcript_messages: `2`",
		"matched_messages: `2`",
		"matched_lines: `2`",
		"results_returned: `1`",
		"raw_bodies_included: `false`",
		"message=`01`",
		"role=`user`",
		"source=`issue`",
		"trusted=`true`",
		"message_sha256_12=",
		"line_sha256_12=",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session search report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_SEARCH_ISSUE_SECRET", "SESSION_SEARCH_ASSISTANT_SECRET", "SESSION_SEARCH_QUERY_SECRET", "deployment SESSION_SEARCH_QUERY_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session search report leaked body/query token %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportShowsAssistantTurnProvenanceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                51,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files,gitclaw.read_file\" -->\nSESSION_PROVENANCE_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_PROVENANCE_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 51,
		Trusted:   true,
	}}
	body := renderSessionReport(Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, comments, transcript, true, "")
	for _, want := range []string{
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files, gitclaw.read_file`",
		"### Assistant Turn Provenance",
		"source=`comment:51`",
		"model=`openai/gpt-4.1-nano`",
		"prompt_context_sha256_12=`abc123abc123`",
		"context_documents=`2`",
		"selected_skills=`1`",
		"tool_outputs=`2`",
		"skills=`repo-reader`",
		"tools=`gitclaw.search_files, gitclaw.read_file`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_PROVENANCE_ASSISTANT_SECRET") {
		t.Fatalf("session report leaked assistant body:\n%s", body)
	}
}

func TestRenderSessionProvenanceReportShowsPromptEvidenceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                53,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"2\" skills=\"repo-reader\" tools=\"gitclaw.search_files,gitclaw.read_file\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" usage_cache_read_tokens=\"7\" usage_cache_write_tokens=\"0\" -->\nSESSION_PROVENANCE_NAMED_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_PROVENANCE_NAMED_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 53,
		Trusted:   true,
	}}
	body := RenderSessionProvenanceReport(BuildSessionProvenanceReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 6}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Provenance Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#6`",
		"event_kind: `issue_comment`",
		"session_provenance_status: `ok`",
		"provenance_scope: `assistant-turn-marker-prompt-context`",
		"session_store: `github-issue-thread`",
		"raw_comments: `1`",
		"transcript_messages: `1`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"assistant_turns_missing_prompt_provenance: `0`",
		"unique_prompt_context_hashes: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-4.1-nano`",
		"prompt_visible_skill_count: `1`",
		"prompt_visible_tool_count: `2`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.search_files, gitclaw.read_file`",
		"usage_prompt_tokens: `100`",
		"usage_completion_tokens: `9`",
		"usage_total_tokens: `109`",
		"usage_cache_read_tokens: `7`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_provenance_change: `true`",
		"### Assistant Turn Provenance",
		"source=`comment:53`",
		"prompt_context_sha256_12=`abc123abc123`",
		"usage_present=`true`",
		"usage_total_tokens=`109`",
		"### Provenance Gates",
		"prompt_provenance_gate=`pass`",
		"model_backed_gate=`pass`",
		"skill_tool_gate=`pass`",
		"usage_telemetry_gate=`pass`",
		"raw_body_gate=`hashes-and-marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session provenance report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_PROVENANCE_NAMED_ASSISTANT_SECRET") {
		t.Fatalf("session provenance report leaked assistant body:\n%s", body)
	}
}

func TestRenderSessionReportRoutesProvenanceCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   54,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_total_tokens=\"12\" -->\nSESSION_PROVENANCE_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 7, Title: "@gitclaw /session provenance", Body: "SESSION_PROVENANCE_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Provenance Report", "session_provenance_status: `ok`", "provenance_scope: `assistant-turn-marker-prompt-context`", "prompt_visible_skill_names: `repo-reader`", "prompt_visible_tool_names: `gitclaw.search_files`", "usage_total_tokens: `12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session provenance route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_PROVENANCE_ROUTE_SECRET", "SESSION_PROVENANCE_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session provenance route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionToolsReportShowsToolLedgerWithoutBodies(t *testing.T) {
	comments := []Comment{
		{
			ID:                55,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" usage_prompt_tokens=\"100\" usage_completion_tokens=\"9\" usage_total_tokens=\"109\" -->\nSESSION_TOOLS_MODEL_ASSISTANT_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
		{
			ID:                56,
			Body:              "<!-- gitclaw:assistant-turn run_id=\"run-2\" model=\"gitclaw/session\" prompt_context_sha256_12=\"def456def456\" context_documents=\"1\" selected_skills=\"0\" tool_outputs=\"1\" tools=\"gitclaw.search_files\" -->\nSESSION_TOOLS_DETERMINISTIC_SECRET",
			User:              User{Login: "github-actions[bot]", Type: "Bot"},
			AuthorAssociation: "NONE",
		},
	}
	transcript := []TranscriptMessage{
		{Role: "assistant", Body: "SESSION_TOOLS_MODEL_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 55, Trusted: true},
		{Role: "assistant", Body: "SESSION_TOOLS_DETERMINISTIC_SECRET", Actor: "github-actions[bot]", CommentID: 56, Trusted: true},
	}
	body := RenderSessionToolsReport(BuildSessionToolsReport("issue-thread", "", Event{Repo: "owner/repo", Kind: "issue_comment", Issue: Issue{Number: 8}}, comments, transcript))
	for _, want := range []string{
		"GitClaw Session Tools Report",
		"scope: `issue-thread`",
		"repository: `owner/repo`",
		"issue: `#8`",
		"event_kind: `issue_comment`",
		"session_tools_status: `ok`",
		"provenance_scope: `assistant-turn-marker-tool-context`",
		"session_store: `github-issue-thread`",
		"raw_comments: `2`",
		"transcript_messages: `2`",
		"assistant_turn_comments: `2`",
		"tool_backed_assistant_turns: `2`",
		"assistant_turns_missing_tool_context: `0`",
		"unique_prompt_visible_tools: `2`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"prompt_visible_tool_output_markers: `4`",
		"model_backed_tool_turns: `1`",
		"deterministic_tool_turns: `1`",
		"model_names: `gitclaw/session, openai/gpt-4.1-nano`",
		"usage_total_tokens: `109`",
		"raw_bodies_included: `false`",
		"raw_issue_bodies_included: `false`",
		"raw_comment_bodies_included: `false`",
		"raw_assistant_replies_included: `false`",
		"raw_prompts_included: `false`",
		"raw_tool_inputs_included: `false`",
		"raw_tool_outputs_included: `false`",
		"raw_search_queries_included: `false`",
		"repository_mutation_allowed: `false`",
		"llm_e2e_required_after_session_tools_change: `true`",
		"### Tool Ledger",
		"tool=`gitclaw.search_files` prompt_visible_turns=`2` model_backed_turns=`1` deterministic_turns=`1` first_source=`comment:55` last_source=`comment:56` models=`gitclaw/session, openai/gpt-4.1-nano` prompt_context_hashes=`2`",
		"tool=`gitclaw.list_files` prompt_visible_turns=`1` model_backed_turns=`1` deterministic_turns=`0` first_source=`comment:55` last_source=`comment:55` models=`openai/gpt-4.1-nano` prompt_context_hashes=`1`",
		"### Tool Turn Evidence",
		"source=`comment:55` model=`openai/gpt-4.1-nano` prompt_context_sha256_12=`abc123abc123` tool_outputs=`3` tools=`gitclaw.list_files, gitclaw.search_files` usage_present=`true` usage_total_tokens=`109`",
		"source=`comment:56` model=`gitclaw/session` prompt_context_sha256_12=`def456def456` tool_outputs=`1` tools=`gitclaw.search_files` usage_present=`false` usage_total_tokens=`0`",
		"### Tool Gates",
		"tool_context_gate=`pass`",
		"model_backed_tool_gate=`pass`",
		"usage_telemetry_gate=`pass`",
		"raw_tool_input_gate=`marker-attributes-only`",
		"raw_tool_output_gate=`marker-attributes-only`",
		"mutation_gate=`disabled`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session tools report missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_TOOLS_MODEL_ASSISTANT_SECRET", "SESSION_TOOLS_DETERMINISTIC_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session tools report leaked %q:\n%s", leaked, body)
		}
	}
}

func TestRenderSessionReportRoutesToolsCommandWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:   57,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" usage_total_tokens=\"12\" -->\nSESSION_TOOLS_ROUTE_SECRET",
	}}
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Kind:  "issue_comment",
		Issue: Issue{Number: 9, Title: "@gitclaw /session tools", Body: "SESSION_TOOLS_ROUTE_ISSUE_SECRET"},
	}, DefaultConfig(), comments, nil)
	for _, want := range []string{"GitClaw Session Tools Report", "session_tools_status: `ok`", "provenance_scope: `assistant-turn-marker-tool-context`", "tool_backed_assistant_turns: `1`", "prompt_visible_tool_names: `gitclaw.search_files`", "usage_total_tokens: `12`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session tools route missing %q:\n%s", want, body)
		}
	}
	for _, leaked := range []string{"SESSION_TOOLS_ROUTE_SECRET", "SESSION_TOOLS_ROUTE_ISSUE_SECRET"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("session tools route leaked %q:\n%s", leaked, body)
		}
	}
}

func TestBuildSessionPromptProvenanceReportCountsMissingMarkers(t *testing.T) {
	report := buildSessionPromptProvenanceReport([]Comment{{
		ID:   52,
		Body: "<!-- gitclaw:assistant-turn model=\"gitclaw/context\" -->\nold deterministic report",
	}})
	if report.TurnsWithProvenance != 0 || report.PromptContextHashMissing != 1 || len(report.Turns) != 1 {
		t.Fatalf("unexpected provenance report: %#v", report)
	}
}

func TestRenderSessionCoverageReportRequiresModelBackedProvenanceWithoutBodies(t *testing.T) {
	comments := []Comment{{
		ID:                61,
		Body:              "<!-- gitclaw:assistant-turn run_id=\"run-1\" model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"2\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.search_files\" -->\nSESSION_COVERAGE_ASSISTANT_SECRET",
		User:              User{Login: "github-actions[bot]", Type: "Bot"},
		AuthorAssociation: "NONE",
	}}
	transcript := []TranscriptMessage{{
		Role:      "assistant",
		Body:      "SESSION_COVERAGE_ASSISTANT_SECRET",
		Actor:     "github-actions[bot]",
		CommentID: 61,
		Trusted:   true,
	}}
	req := DefaultSessionCoverageRequirements()
	req.RequiredSkills = []string{"repo-reader"}
	req.RequiredTools = []string{"gitclaw.search_files"}
	report := BuildSessionCoverageReport("issue-thread", "", Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, comments, transcript, req)
	if !report.OK() {
		t.Fatalf("session coverage unexpectedly failed: %#v", report)
	}
	body := RenderSessionCoverageReport(report)
	for _, want := range []string{
		"GitClaw Session Coverage Report",
		"session_coverage_status: `ok`",
		"required_assistant_turns: `1`",
		"required_prompt_provenance_turns: `1`",
		"required_model_backed_turns: `1`",
		"required_skill_names: `repo-reader`",
		"required_tool_names: `gitclaw.search_files`",
		"assistant_turn_comments: `1`",
		"assistant_turns_with_prompt_provenance: `1`",
		"model_backed_assistant_turns: `1`",
		"deterministic_assistant_turns: `0`",
		"model_names: `openai/gpt-4.1-nano`",
		"prompt_visible_skill_names: `repo-reader`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.search_files`",
		"missing_required_skill_names: `none`",
		"missing_required_tool_names: `none`",
		"raw_bodies_included: `false`",
		"raw_prompts_included: `false`",
		"llm_e2e_required_after_session_coverage_change: `true`",
		"assistant_turns_met=`true`",
		"prompt_provenance_met=`true`",
		"model_backed_turns_met=`true`",
		"required_skills_met=`true`",
		"required_tools_met=`true`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("session coverage report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SESSION_COVERAGE_ASSISTANT_SECRET") {
		t.Fatalf("session coverage report leaked assistant body:\n%s", body)
	}
}

func TestRenderSessionCoverageReportWarnsWithoutModelBackedTurn(t *testing.T) {
	report := BuildSessionCoverageReport("issue-thread", "", Event{Repo: "owner/repo", Issue: Issue{Number: 5}}, []Comment{{
		ID:   62,
		Body: "<!-- gitclaw:assistant-turn model=\"gitclaw/session\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"0\" tool_outputs=\"0\" -->\ndeterministic body",
	}}, nil, DefaultSessionCoverageRequirements())
	if report.OK() || report.SessionCoverageStatus != "warn" || report.ModelBackedTurnsMet {
		t.Fatalf("session coverage unexpectedly passed: %#v", report)
	}
	body := RenderSessionCoverageReport(report)
	for _, want := range []string{"session_coverage_status: `warn`", "model_backed_assistant_turns: `0`", "deterministic_assistant_turns: `1`", "model_backed_turns_met=`false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("warning session coverage report missing %q:\n%s", want, body)
		}
	}
}

func TestRenderSessionReportRoutesCoverageCommand(t *testing.T) {
	body := RenderSessionReport(Event{
		Repo:  "owner/repo",
		Issue: Issue{Number: 5, Title: "@gitclaw /session coverage"},
	}, DefaultConfig(), []Comment{{
		ID:   63,
		Body: "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"abc123abc123\" context_documents=\"1\" selected_skills=\"1\" tool_outputs=\"1\" skills=\"repo-reader\" tools=\"gitclaw.search_files\" -->\ncoverage body secret",
	}}, nil)
	for _, want := range []string{"GitClaw Session Coverage Report", "session_coverage_status: `ok`", "model_backed_assistant_turns: `1`", "prompt_visible_skill_names: `repo-reader`", "prompt_visible_tool_names: `gitclaw.search_files`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("session coverage route missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "coverage body secret") {
		t.Fatalf("session coverage route leaked body:\n%s", body)
	}
}

func TestSessionCoverageCommandReportsBackupCoverageWithoutBodies(t *testing.T) {
	root := t.TempDir()
	backup := IssueBackup{
		Version:     1,
		GeneratedAt: "2026-05-31T00:00:00Z",
		Repo:        "owner/repo",
		EventName:   "issues",
		Issue: IssueBackupIssue{
			Number: 22,
			Title:  "@gitclaw coverage backup SESSION_COVERAGE_BACKUP_TITLE_SECRET",
			Body:   "SESSION_COVERAGE_BACKUP_BODY_SECRET",
		},
		Comments: []IssueBackupComment{{
			ID:                71,
			Body:              "<!-- gitclaw:assistant-turn model=\"openai/gpt-4.1-nano\" prompt_context_sha256_12=\"def456def456\" context_documents=\"4\" selected_skills=\"1\" tool_outputs=\"3\" skills=\"repo-reader\" tools=\"gitclaw.list_files,gitclaw.skill_index,gitclaw.search_files\" -->\nSESSION_COVERAGE_BACKUP_ASSISTANT_SECRET",
			Author:            "github-actions[bot]",
			AuthorAssociation: "NONE",
		}},
		Transcript: []TranscriptMessage{
			{Role: "user", Body: "SESSION_COVERAGE_BACKUP_USER_SECRET", Actor: "alice", Trusted: true},
			{Role: "assistant", Body: "SESSION_COVERAGE_BACKUP_ASSISTANT_SECRET", Actor: "github-actions[bot]", CommentID: 71, Trusted: true},
		},
	}
	writeBackupFixture(t, root, backup)
	backupPath := issueBackupPath(root, backup.Repo, backup.Issue.Number)

	output := captureStdout(t, func() {
		err := RunCLI(context.Background(), []string{
			"session", "coverage",
			"--backup", backupPath,
			"--require-skill", "repo-reader",
			"--require-tool", "gitclaw.search_files",
		})
		if err != nil {
			t.Fatalf("session coverage returned error: %v", err)
		}
	})
	for _, want := range []string{
		"GitClaw Session Coverage Report",
		"scope: `local-backup`",
		"backup_file:",
		"repository: `owner/repo`",
		"issue: `#22`",
		"session_coverage_status: `ok`",
		"model_backed_assistant_turns: `1`",
		"required_skill_names: `repo-reader`",
		"required_tool_names: `gitclaw.search_files`",
		"prompt_visible_tool_names: `gitclaw.list_files, gitclaw.skill_index, gitclaw.search_files`",
		"required_tools_met=`true`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("session coverage output missing %q:\n%s", want, output)
		}
	}
	for _, leaked := range []string{"SESSION_COVERAGE_BACKUP_TITLE_SECRET", "SESSION_COVERAGE_BACKUP_BODY_SECRET", "SESSION_COVERAGE_BACKUP_USER_SECRET", "SESSION_COVERAGE_BACKUP_ASSISTANT_SECRET", "@gitclaw coverage backup"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("session coverage leaked body/title token %q:\n%s", leaked, output)
		}
	}
}
