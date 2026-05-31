package gitclaw

import (
	"context"
	"strings"
	"testing"
)

func TestRunHeartbeatPostsOncePerSlot(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number:            7,
			Title:             "@gitclaw heartbeat check",
			Body:              "On heartbeat include token HEARTBEAT_TEST_TOKEN.",
			AuthorAssociation: "MEMBER",
			User:              User{Login: "alice", Type: "User"},
			Labels:            []string{"gitclaw", "gitclaw:heartbeat"},
		}},
		CommentsByIssue: map[int][]Comment{7: nil},
	}
	llm := &FakeLLM{
		Response:          "Heartbeat noted: HEARTBEAT_TEST_TOKEN.",
		SelectedModelName: "openai/gpt-5-nano",
		Usage:             LLMUsage{Present: true, PromptTokens: 120, CompletionTokens: 30, TotalTokens: 150},
	}

	result, err := RunHeartbeat(context.Background(), cfg, github, llm, HeartbeatOptions{
		Repo:  "owner/repo",
		Label: "gitclaw:heartbeat",
		Slot:  "slot-1",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("RunHeartbeat returned error: %v", err)
	}
	if result.Posted != 1 || len(github.Posted) != 1 {
		t.Fatalf("posted result = %+v, posted comments = %#v", result, github.Posted)
	}
	if !HasHeartbeatMarker(github.Posted[0].Body) || !ContainsHeartbeatSlot(github.Posted[0].Body, "slot-1") {
		t.Fatalf("heartbeat marker missing: %s", github.Posted[0].Body)
	}
	if !strings.Contains(github.Posted[0].Body, "HEARTBEAT_TEST_TOKEN") {
		t.Fatalf("heartbeat response missing token: %s", github.Posted[0].Body)
	}
	for _, want := range []string{`model="openai/gpt-5-nano"`, `prompt_context_sha256_12="`, `context_documents="`, `selected_skills="`, `tool_outputs="`, `usage_prompt_tokens="120"`, `usage_completion_tokens="30"`, `usage_total_tokens="150"`} {
		if !strings.Contains(github.Posted[0].Body, want) {
			t.Fatalf("heartbeat marker missing %q: %s", want, github.Posted[0].Body)
		}
	}

	result, err = RunHeartbeat(context.Background(), cfg, github, llm, HeartbeatOptions{
		Repo:  "owner/repo",
		Label: "gitclaw:heartbeat",
		Slot:  "slot-1",
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("second RunHeartbeat returned error: %v", err)
	}
	if result.Posted != 0 || result.Skipped != 1 || len(github.Posted) != 1 {
		t.Fatalf("second run should be idempotent: result=%+v posted=%#v", result, github.Posted)
	}
}

func TestRunHeartbeatSkipsOKResponse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workdir = t.TempDir()
	github := &FakeGitHub{
		Issues: []Issue{{
			Number:            8,
			Title:             "@gitclaw heartbeat quiet",
			Body:              "No visible update needed.",
			AuthorAssociation: "MEMBER",
			User:              User{Login: "alice", Type: "User"},
			Labels:            []string{"gitclaw:heartbeat"},
		}},
		CommentsByIssue: map[int][]Comment{8: nil},
	}
	llm := &FakeLLM{Response: "HEARTBEAT_OK"}
	result, err := RunHeartbeat(context.Background(), cfg, github, llm, HeartbeatOptions{
		Repo:  "owner/repo",
		Label: "gitclaw:heartbeat",
		Slot:  "slot-quiet",
	})
	if err != nil {
		t.Fatalf("RunHeartbeat returned error: %v", err)
	}
	if result.Posted != 0 || result.Skipped != 1 || len(github.Posted) != 0 {
		t.Fatalf("HEARTBEAT_OK should not post: result=%+v posted=%#v", result, github.Posted)
	}
}

func TestHeartbeatInstructionUsesSlot(t *testing.T) {
	instruction := heartbeatInstruction("slot-abc")
	if !strings.Contains(instruction, "slot-abc") || !strings.Contains(instruction, "HEARTBEAT_OK") {
		t.Fatalf("heartbeat instruction missing expected terms: %s", instruction)
	}
}
