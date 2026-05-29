package gitclaw

import (
	"fmt"
	"strings"
)

func IsContextReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/context"
}

func activeRequestText(ev Event) string {
	if ev.Comment != nil {
		return ev.Comment.Body
	}
	return strings.TrimSpace(ev.Issue.Title + "\n" + ev.Issue.Body)
}

func RenderContextReport(ev Event, cfg Config, transcript []TranscriptMessage, repoContext RepoContext) string {
	return renderContextReport(ev, cfg, transcript, repoContext, true)
}

func RenderContextCLIReport(cfg Config, repoContext RepoContext) string {
	return renderContextReport(Event{}, cfg, nil, repoContext, false)
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

func writeContextDocumentList(b *strings.Builder, docs []ContextDocument) {
	if len(docs) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, doc := range docs {
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
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
