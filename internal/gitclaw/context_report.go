package gitclaw

import (
	"fmt"
	"strings"
)

func IsContextReportRequest(ev Event, cfg Config) bool {
	text := strings.TrimSpace(activeRequestText(ev))
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	prefix := strings.ToLower(cfg.TriggerPrefix)
	if strings.HasPrefix(lower, prefix) {
		text = strings.TrimSpace(text[len(cfg.TriggerPrefix):])
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	command := strings.Trim(strings.ToLower(fields[0]), " \t\r\n.,:;!?")
	return command == "/context"
}

func activeRequestText(ev Event) string {
	if ev.Comment != nil {
		return ev.Comment.Body
	}
	return strings.TrimSpace(ev.Issue.Title + "\n" + ev.Issue.Body)
}

func RenderContextReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("## GitClaw Context Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", len(transcript))
	fmt.Fprintf(&b, "- max_prompt_bytes: `%d`\n", cfg.MaxPromptBytes)
	fmt.Fprintf(&b, "- max_transcript_messages: `%d`\n", cfg.MaxTranscriptMessages)
	fmt.Fprintf(&b, "- max_transcript_message_bytes: `%d`\n\n", cfg.MaxTranscriptMessageBytes)

	b.WriteString("### Context Files\n")
	writeContextDocumentList(&b, repoContext.Documents)

	b.WriteString("\n### Selected Skills\n")
	writeContextDocumentList(&b, repoContext.Skills)

	b.WriteString("\n### Tool Outputs\n")
	if len(repoContext.ToolOutputs) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, output := range repoContext.ToolOutputs {
			fmt.Fprintf(&b, "- `%s` input=`%s` bytes=`%d` lines=`%d`\n", output.Name, inlineCode(output.Input), len(output.Output), lineCount(output.Output))
		}
	}

	return strings.TrimSpace(b.String())
}

func writeContextDocumentList(b *strings.Builder, docs []ContextDocument) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d`\n", doc.Path, len(doc.Body), lineCount(doc.Body))
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
