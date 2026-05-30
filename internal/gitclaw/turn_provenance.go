package gitclaw

import (
	"fmt"
	"path/filepath"
	"strings"
)

func withPromptProvenance(marker Marker, repoContext RepoContext) Marker {
	marker.PromptContextSHA = promptContextHash(repoContext)
	marker.ContextDocuments = len(repoContext.Documents)
	marker.SelectedSkills = len(repoContext.Skills)
	marker.ToolOutputs = len(repoContext.ToolOutputs)
	marker.PromptVisibleSkills = promptVisibleSkillNames(repoContext.Skills)
	marker.PromptVisibleTools = promptVisibleToolNames(repoContext.ToolOutputs)
	return marker
}

func promptContextHash(repoContext RepoContext) string {
	var b strings.Builder
	for _, doc := range repoContext.Documents {
		fmt.Fprintf(&b, "context\t%s\t%d\t%d\t%s\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
	for _, skill := range repoContext.Skills {
		fmt.Fprintf(&b, "skill\t%s\t%d\t%d\t%s\n", skill.Path, len(skill.Body), lineCount(skill.Body), shortDocumentHash(skill.Body))
	}
	for _, output := range repoContext.ToolOutputs {
		fmt.Fprintf(&b, "tool\t%s\t%s\t%d\t%d\t%s\n", output.Name, shortDocumentHash(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output))
	}
	return shortDocumentHash(b.String())
}

func promptVisibleSkillNames(skills []ContextDocument) []string {
	names := make([]string, 0, len(skills))
	seen := map[string]bool{}
	for _, skill := range skills {
		name := skillNameFromPath(skill.Path)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func skillNameFromPath(path string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	if normalized == "" {
		return ""
	}
	if strings.HasSuffix(normalized, "/SKILL.md") {
		name := filepath.Base(filepath.Dir(normalized))
		if name != "." && name != "/" {
			return name
		}
	}
	return normalized
}

func promptVisibleToolNames(outputs []ToolOutput) []string {
	names := make([]string, 0, len(outputs))
	seen := map[string]bool{}
	for _, output := range outputs {
		name := strings.TrimSpace(output.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
