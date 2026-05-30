package gitclaw

import (
	"strings"
	"testing"
)

func TestWithPromptProvenanceAddsBodyFreePromptVisibleEvidence(t *testing.T) {
	ctx := RepoContext{
		Documents: []ContextDocument{{Path: ".gitclaw/SOUL.md", Body: "SOUL_PROVENANCE_SECRET"}},
		Skills: []ContextDocument{{
			Path: ".gitclaw/SKILLS/repo-reader/SKILL.md",
			Body: "SKILL_PROVENANCE_SECRET",
		}},
		ToolOutputs: []ToolOutput{
			{Name: "gitclaw.search_files", Input: "secret query", Output: "TOOL_PROVENANCE_SECRET"},
			{Name: "gitclaw.search_files", Input: "secret query", Output: "duplicate name"},
			{Name: "gitclaw.read_file", Input: "go.mod", Output: "module example.com/repo"},
		},
	}

	marker := withPromptProvenance(Marker{RunID: "run"}, ctx)
	if marker.PromptContextSHA == "" {
		t.Fatalf("PromptContextSHA is empty")
	}
	if marker.ContextDocuments != 1 || marker.SelectedSkills != 1 || marker.ToolOutputs != 3 {
		t.Fatalf("unexpected counts: %#v", marker)
	}
	if len(marker.PromptVisibleSkills) != 1 || marker.PromptVisibleSkills[0] != "repo-reader" {
		t.Fatalf("PromptVisibleSkills = %#v, want repo-reader", marker.PromptVisibleSkills)
	}
	if len(marker.PromptVisibleTools) != 2 || marker.PromptVisibleTools[0] != "gitclaw.search_files" || marker.PromptVisibleTools[1] != "gitclaw.read_file" {
		t.Fatalf("PromptVisibleTools = %#v, want deduped tool order", marker.PromptVisibleTools)
	}
	body := RenderAssistantComment(marker, "ok")
	for _, leaked := range []string{"SOUL_PROVENANCE_SECRET", "SKILL_PROVENANCE_SECRET", "TOOL_PROVENANCE_SECRET", "secret query", "module example.com/repo"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("prompt provenance marker leaked %q:\n%s", leaked, body)
		}
	}
}

func TestSkillNameFromPathFallsBackToPath(t *testing.T) {
	if got := skillNameFromPath(".gitclaw/SKILLS/repo-reader/SKILL.md"); got != "repo-reader" {
		t.Fatalf("skillNameFromPath() = %q, want repo-reader", got)
	}
	if got := skillNameFromPath(".gitclaw/custom.md"); got != ".gitclaw/custom.md" {
		t.Fatalf("skillNameFromPath() = %q, want original path", got)
	}
}
