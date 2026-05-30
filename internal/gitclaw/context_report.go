package gitclaw

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type ContextInfoMatch struct {
	Kind        string
	Path        string
	Tool        string
	InputSHA    string
	Bytes       int
	Lines       int
	SHA         string
	Selected    bool
	MatchSource string
}

func IsContextReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/context"
}

func activeRequestText(ev Event) string {
	if strings.TrimSpace(ev.ActiveText) != "" {
		return ev.ActiveText
	}
	if ev.Comment != nil {
		return ev.Comment.Body
	}
	return strings.TrimSpace(ev.Issue.Title + "\n" + ev.Issue.Body)
}

func RenderContextReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	if isContextInfoRequest(ev, cfg) {
		return renderContextInfoReport(ev, cfg, repoContext, requestedContextInfoPath(ev, cfg), true)
	}
	return renderContextReport(ev, cfg, transcript, repoContext, true)
}

func RenderContextCLIReport(cfg Config, repoContext RepoContext) string {
	return renderContextReport(Event{}, cfg, nil, repoContext, false)
}

func RenderContextInfoCLIReport(cfg Config, repoContext RepoContext, path string) string {
	return renderContextInfoReport(Event{}, cfg, repoContext, path, false)
}

func renderContextReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext, includeIssue bool) string {
	var b strings.Builder
	b.WriteString("## GitClaw Context Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- max_prompt_bytes: `%d`\n", cfg.MaxPromptBytes)
	fmt.Fprintf(&b, "- max_transcript_messages: `%d`\n", cfg.MaxTranscriptMessages)
	fmt.Fprintf(&b, "- max_transcript_message_bytes: `%d`\n\n", cfg.MaxTranscriptMessageBytes)
	b.WriteString("Context file bodies, skill bodies, issue/comment bodies, and tool output bodies are not included.\n\n")

	b.WriteString("### Context Files\n")
	writeContextDocumentList(&b, repoContext.Documents)

	b.WriteString("\n### Selected Skills\n")
	writeContextDocumentList(&b, repoContext.Skills)

	b.WriteString("\n### Tool Outputs\n")
	if len(repoContext.ToolOutputs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, output := range repoContext.ToolOutputs {
			fmt.Fprintf(&b, "- `%s` input=`%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", output.Name, inlineCode(output.Input), len(output.Output), lineCount(output.Output), shortDocumentHash(output.Output))
		}
	}

	return strings.TrimSpace(b.String())
}

func renderContextInfoReport(ev Event, cfg Config, repoContext RepoContext, path string, includeIssue bool) string {
	path = cleanContextLookupPath(path)
	matches := BuildContextInfoMatches(repoContext, path)
	status := "not_found"
	if path == "" {
		status = "no_query"
	} else if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Context Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_context: `%s`\n", inlineCode(path))
	fmt.Fprintf(&b, "- context_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- matched_context_items: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- context_files_loaded: `%d`\n", len(repoContext.Documents))
	fmt.Fprintf(&b, "- selected_skills: `%d`\n", len(repoContext.Skills))
	fmt.Fprintf(&b, "- active_tool_outputs: `%d`\n", len(repoContext.ToolOutputs))
	fmt.Fprintf(&b, "- max_prompt_bytes: `%d`\n", cfg.MaxPromptBytes)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_inputs_included: `%t`\n\n", false)
	b.WriteString("This report shows one requested context item by metadata only. File bodies, skill bodies, tool output bodies, raw tool inputs, issue/comment bodies, prompts, and secret values are not included.\n\n")

	b.WriteString("### Matches\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, match := range matches {
			writeContextInfoMatch(&b, match)
		}
	}

	if len(matches) == 0 {
		b.WriteString("\n### Available Context Paths\n")
		writeContextInfoAvailablePaths(&b, repoContext)
	}
	return strings.TrimSpace(b.String())
}

func writeContextDocumentList(b *strings.Builder, docs []ContextDocument) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
}

func requestedContextInfoPath(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/context" || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanContextLookupPath(fields[2])
}

func isContextInfoRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/context" && strings.EqualFold(fields[1], "info")
}

func cleanContextLookupPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), " \t\r\n,:;!?`\"'")
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "./")
	if path == "" {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return path
	}
	return clean
}

func BuildContextInfoMatches(repoContext RepoContext, path string) []ContextInfoMatch {
	path = cleanContextLookupPath(path)
	if path == "" {
		return nil
	}
	matchedPaths := map[string]bool{}
	var matches []ContextInfoMatch

	for _, doc := range repoContext.Documents {
		if !contextPathMatches(doc.Path, path) {
			continue
		}
		normalized := strings.ToLower(cleanContextLookupPath(doc.Path))
		matchedPaths[normalized] = true
		matches = append(matches, ContextInfoMatch{
			Kind:        "context_file",
			Path:        doc.Path,
			Bytes:       len(doc.Body),
			Lines:       lineCount(doc.Body),
			SHA:         shortDocumentHash(doc.Body),
			MatchSource: "loaded_context_documents",
		})
	}

	for _, skill := range repoContext.Skills {
		if !contextPathMatches(skill.Path, path) {
			continue
		}
		normalized := strings.ToLower(cleanContextLookupPath(skill.Path))
		matchedPaths[normalized] = true
		matches = append(matches, ContextInfoMatch{
			Kind:        "selected_skill",
			Path:        skill.Path,
			Bytes:       len(skill.Body),
			Lines:       lineCount(skill.Body),
			SHA:         shortDocumentHash(skill.Body),
			Selected:    true,
			MatchSource: "selected_skill_documents",
		})
	}

	for _, output := range repoContext.ToolOutputs {
		inputPath := cleanContextLookupPath(output.Input)
		outputPathMatched := output.Name == "gitclaw.read_file" && inputPath != "" && contextPathMatches(inputPath, path)
		toolMatched := contextToolMatches(output.Name, path)
		if !outputPathMatched && !toolMatched {
			continue
		}
		if outputPathMatched && matchedPaths[strings.ToLower(inputPath)] {
			continue
		}
		matches = append(matches, ContextInfoMatch{
			Kind:        "tool_output",
			Path:        inputPath,
			Tool:        output.Name,
			InputSHA:    shortDocumentHash(output.Input),
			Bytes:       len(output.Output),
			Lines:       lineCount(output.Output),
			SHA:         shortDocumentHash(output.Output),
			MatchSource: "active_tool_outputs",
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Kind != matches[j].Kind {
			return contextInfoKindRank(matches[i].Kind) < contextInfoKindRank(matches[j].Kind)
		}
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Tool < matches[j].Tool
	})
	return matches
}

func contextPathMatches(candidate, requested string) bool {
	candidate = cleanContextLookupPath(candidate)
	requested = cleanContextLookupPath(requested)
	if candidate == "" || requested == "" {
		return false
	}
	if strings.EqualFold(candidate, requested) {
		return true
	}
	if !strings.Contains(requested, "/") && strings.EqualFold(filepath.Base(filepath.FromSlash(candidate)), requested) {
		return true
	}
	return false
}

func contextToolMatches(name, requested string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	requested = strings.ToLower(cleanContextLookupPath(requested))
	if name == "" || requested == "" {
		return false
	}
	return name == requested || strings.TrimPrefix(name, "gitclaw.") == requested
}

func contextInfoKindRank(kind string) int {
	switch kind {
	case "context_file":
		return 0
	case "selected_skill":
		return 1
	case "tool_output":
		return 2
	default:
		return 9
	}
}

func writeContextInfoMatch(b *strings.Builder, match ContextInfoMatch) {
	switch match.Kind {
	case "tool_output":
		fmt.Fprintf(b, "- kind=`%s` tool=`%s` path=`%s` input_sha256_12=`%s` output_bytes=`%d` output_lines=`%d` output_sha256_12=`%s` source=`%s`\n",
			match.Kind,
			match.Tool,
			inlineCode(match.Path),
			match.InputSHA,
			match.Bytes,
			match.Lines,
			match.SHA,
			match.MatchSource,
		)
	case "selected_skill":
		fmt.Fprintf(b, "- kind=`%s` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` selected=`%t` source=`%s`\n",
			match.Kind,
			inlineCode(match.Path),
			match.Bytes,
			match.Lines,
			match.SHA,
			match.Selected,
			match.MatchSource,
		)
	default:
		fmt.Fprintf(b, "- kind=`%s` path=`%s` bytes=`%d` lines=`%d` sha256_12=`%s` source=`%s`\n",
			match.Kind,
			inlineCode(match.Path),
			match.Bytes,
			match.Lines,
			match.SHA,
			match.MatchSource,
		)
	}
}

func writeContextInfoAvailablePaths(b *strings.Builder, repoContext RepoContext) {
	wrote := false
	for _, doc := range repoContext.Documents {
		wrote = true
		fmt.Fprintf(b, "- kind=`context_file` path=`%s`\n", inlineCode(doc.Path))
	}
	for _, skill := range repoContext.SkillSummaries {
		wrote = true
		fmt.Fprintf(b, "- kind=`skill` path=`%s` selected=`%t`\n", inlineCode(skill.Path), skillSelectedForTurn(repoContext, skill))
	}
	for _, output := range repoContext.ToolOutputs {
		if output.Name != "gitclaw.read_file" {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- kind=`tool_output` tool=`%s` path_sha256_12=`%s`\n", output.Name, shortDocumentHash(output.Input))
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func lineCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return strings.Count(value, "\n") + 1
}

func inlineCode(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "`", "'")
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) > 80 {
		return value[:77] + "..."
	}
	return value
}
