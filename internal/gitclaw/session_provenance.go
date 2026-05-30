package gitclaw

import (
	"fmt"
	"strconv"
	"strings"
)

type sessionPromptProvenanceReport struct {
	Turns                    []sessionPromptProvenanceTurn
	TurnsWithProvenance      int
	UniquePromptContextSHAs  int
	PromptVisibleSkillNames  []string
	PromptVisibleToolNames   []string
	PromptContextHashMissing int
}

type sessionPromptProvenanceTurn struct {
	Source            string
	Model             string
	PromptContextSHA  string
	ContextDocuments  int
	SelectedSkills    int
	ToolOutputs       int
	Skills            []string
	Tools             []string
	HasPromptEvidence bool
}

func buildSessionPromptProvenanceReport(comments []Comment) sessionPromptProvenanceReport {
	report := sessionPromptProvenanceReport{}
	promptHashes := map[string]bool{}
	skillSeen := map[string]bool{}
	toolSeen := map[string]bool{}
	for _, comment := range comments {
		turn, ok := parseSessionPromptProvenanceTurn(comment)
		if !ok {
			continue
		}
		report.Turns = append(report.Turns, turn)
		if !turn.HasPromptEvidence {
			report.PromptContextHashMissing++
			continue
		}
		report.TurnsWithProvenance++
		if turn.PromptContextSHA != "" {
			promptHashes[turn.PromptContextSHA] = true
		}
		for _, skill := range turn.Skills {
			if skill == "" || skillSeen[skill] {
				continue
			}
			skillSeen[skill] = true
			report.PromptVisibleSkillNames = append(report.PromptVisibleSkillNames, skill)
		}
		for _, tool := range turn.Tools {
			if tool == "" || toolSeen[tool] {
				continue
			}
			toolSeen[tool] = true
			report.PromptVisibleToolNames = append(report.PromptVisibleToolNames, tool)
		}
	}
	report.UniquePromptContextSHAs = len(promptHashes)
	return report
}

func parseSessionPromptProvenanceTurn(comment Comment) (sessionPromptProvenanceTurn, bool) {
	match := markerPattern.FindStringSubmatch(comment.Body)
	if len(match) < 2 {
		return sessionPromptProvenanceTurn{}, false
	}
	attrs := match[1]
	turn := sessionPromptProvenanceTurn{
		Source:           fmt.Sprintf("comment:%d", comment.ID),
		Model:            markerAttribute(attrs, "model"),
		PromptContextSHA: markerAttribute(attrs, "prompt_context_sha256_12"),
		ContextDocuments: markerAttributeInt(attrs, "context_documents"),
		SelectedSkills:   markerAttributeInt(attrs, "selected_skills"),
		ToolOutputs:      markerAttributeInt(attrs, "tool_outputs"),
		Skills:           markerAttributeList(attrs, "skills"),
		Tools:            markerAttributeList(attrs, "tools"),
	}
	turn.HasPromptEvidence = turn.PromptContextSHA != ""
	return turn, true
}

func markerAttributeInt(attrs, key string) int {
	raw := strings.TrimSpace(markerAttribute(attrs, key))
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func markerAttributeList(attrs, key string) []string {
	raw := markerAttribute(attrs, key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}
