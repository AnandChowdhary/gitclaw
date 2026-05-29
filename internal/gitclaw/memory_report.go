package gitclaw

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const longTermMemoryPath = ".gitclaw/MEMORY.md"

var datedMemoryNotePattern = regexp.MustCompile(`^\.gitclaw/memory/\d{4}-\d{2}-\d{2}\.md$`)

type memorySurface struct {
	LongTerm        configSurfaceFile
	DatedNotes      []configSurfaceFile
	LoadedLongTerm  bool
	LoadedNotePaths []string
}

func IsMemoryReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/memory" || command == "/memories"
}

func RenderMemoryReport(ev Event, cfg Config, repoContext RepoContext) string {
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	latest := latestMemoryNotePath(surface.DatedNotes)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	fmt.Fprintf(&b, "- memory_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- long_term_memory_present: `%t`\n", surface.LongTerm.Present)
	fmt.Fprintf(&b, "- long_term_memory_loaded: `%t`\n", surface.LoadedLongTerm)
	fmt.Fprintf(&b, "- dated_memory_notes: `%d`\n", len(surface.DatedNotes))
	fmt.Fprintf(&b, "- canonical_dated_memory_notes: `%d`\n", countCanonicalMemoryNotes(surface.DatedNotes))
	fmt.Fprintf(&b, "- noncanonical_dated_memory_notes: `%d`\n", countNoncanonicalMemoryNotes(surface.DatedNotes))
	fmt.Fprintf(&b, "- loaded_memory_notes: `%d`\n", len(surface.LoadedNotePaths))
	fmt.Fprintf(&b, "- max_loaded_memory_notes: `%d`\n", maxMemoryDocuments)
	fmt.Fprintf(&b, "- omitted_memory_notes: `%d`\n", omittedMemoryNotes(surface))
	fmt.Fprintf(&b, "- latest_memory_note: `%s`\n", latest)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n\n", shortDocumentHash(ev.Issue.Title))

	b.WriteString("GitClaw memory is repo-local Markdown loaded as read-only prompt context. This report never dumps memory bodies; hashes let maintainers verify exactly which git-backed memory files were present and loaded.\n\n")
	b.WriteString("Memory edits require normal reviewed git changes. GitClaw does not self-write `.gitclaw/MEMORY.md` or `.gitclaw/memory/*.md` during assistant turns.\n\n")

	b.WriteString("### Long-Term Memory\n")
	writeConfigSurfaceFile(&b, surface.LongTerm)

	b.WriteString("\n### Dated Memory Notes\n")
	writeMemorySurfaceFiles(&b, surface.DatedNotes)

	b.WriteString("\n### Loaded For This Turn\n")
	writeLoadedMemoryPaths(&b, surface)

	return strings.TrimSpace(b.String())
}

func inspectMemorySurface(root string, repoContext RepoContext) memorySurface {
	if root == "" {
		root = "."
	}
	surface := memorySurface{
		LongTerm: inspectConfigSurfaceFile(root, longTermMemoryPath),
	}
	absRoot, err := filepath.Abs(root)
	if err == nil {
		matches, _ := filepath.Glob(filepath.Join(absRoot, ".gitclaw", "memory", "*.md"))
		sort.Strings(matches)
		for _, match := range matches {
			rel, err := filepath.Rel(absRoot, match)
			if err != nil {
				continue
			}
			surface.DatedNotes = append(surface.DatedNotes, inspectConfigSurfaceFile(root, filepath.ToSlash(rel)))
		}
	}
	loadedNotes := map[string]bool{}
	for _, doc := range repoContext.Documents {
		if doc.Path == longTermMemoryPath {
			surface.LoadedLongTerm = true
			continue
		}
		if isSoulMemoryNote(doc.Path) {
			loadedNotes[doc.Path] = true
		}
	}
	for path := range loadedNotes {
		surface.LoadedNotePaths = append(surface.LoadedNotePaths, path)
	}
	sort.Strings(surface.LoadedNotePaths)
	return surface
}

func writeMemorySurfaceFiles(b *strings.Builder, files []configSurfaceFile) {
	if len(files) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, file := range files {
		writeConfigSurfaceFile(b, file)
	}
}

func writeLoadedMemoryPaths(b *strings.Builder, surface memorySurface) {
	wrote := false
	if surface.LoadedLongTerm {
		fmt.Fprintf(b, "- `%s`\n", longTermMemoryPath)
		wrote = true
	}
	for _, path := range surface.LoadedNotePaths {
		fmt.Fprintf(b, "- `%s`\n", path)
		wrote = true
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func countCanonicalMemoryNotes(files []configSurfaceFile) int {
	count := 0
	for _, file := range files {
		if datedMemoryNotePattern.MatchString(file.Path) {
			count++
		}
	}
	return count
}

func countNoncanonicalMemoryNotes(files []configSurfaceFile) int {
	return len(files) - countCanonicalMemoryNotes(files)
}

func omittedMemoryNotes(surface memorySurface) int {
	omitted := len(surface.DatedNotes) - len(surface.LoadedNotePaths)
	if omitted < 0 {
		return 0
	}
	return omitted
}

func latestMemoryNotePath(files []configSurfaceFile) string {
	for i := len(files) - 1; i >= 0; i-- {
		if datedMemoryNotePattern.MatchString(files[i].Path) {
			return files[i].Path
		}
	}
	return "none"
}
