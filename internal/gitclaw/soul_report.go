package gitclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func IsSoulReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/soul"
}

func RenderSoulReport(ev Event, repoContext RepoContext) string {
	var b strings.Builder
	b.WriteString("## GitClaw Soul Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- identity_policy_files: `%d`\n", soulIdentityDocumentCount(repoContext.Documents))
	fmt.Fprintf(&b, "- memory_notes: `%d`\n\n", soulMemoryDocumentCount(repoContext.Documents))
	b.WriteString("File bodies are not included; hashes let maintainers verify exactly which git-backed context was loaded.\n\n")

	b.WriteString("### Identity And Policy Files\n")
	writeSoulDocumentList(&b, repoContext.Documents, false)

	b.WriteString("\n### Memory Notes\n")
	writeSoulDocumentList(&b, repoContext.Documents, true)

	return strings.TrimSpace(b.String())
}

func writeSoulDocumentList(b *strings.Builder, docs []ContextDocument, memoryOnly bool) {
	wrote := false
	for _, doc := range docs {
		if isSoulMemoryNote(doc.Path) != memoryOnly {
			continue
		}
		wrote = true
		fmt.Fprintf(b, "- `%s` bytes=`%d` lines=`%d` sha256_12=`%s`\n", doc.Path, len(doc.Body), lineCount(doc.Body), shortDocumentHash(doc.Body))
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func soulIdentityDocumentCount(docs []ContextDocument) int {
	count := 0
	for _, doc := range docs {
		if !isSoulMemoryNote(doc.Path) {
			count++
		}
	}
	return count
}

func soulMemoryDocumentCount(docs []ContextDocument) int {
	count := 0
	for _, doc := range docs {
		if isSoulMemoryNote(doc.Path) {
			count++
		}
	}
	return count
}

func isSoulMemoryNote(path string) bool {
	return strings.HasPrefix(path, ".gitclaw/memory/")
}

func shortDocumentHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:12]
}
