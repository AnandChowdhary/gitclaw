package gitclaw

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const longTermMemoryPath = ".gitclaw/MEMORY.md"
const defaultMemorySearchMaxResults = 10

var datedMemoryNotePattern = regexp.MustCompile(`^\.gitclaw/memory/\d{4}-\d{2}-\d{2}\.md$`)

type memorySurface struct {
	LongTerm        configSurfaceFile
	DatedNotes      []configSurfaceFile
	LoadedLongTerm  bool
	LoadedNotePaths []string
}

type MemorySearchReport struct {
	QueryHash         string
	QueryTerms        int
	SearchStatus      string
	MaxResults        int
	FilesScanned      int
	MatchedFiles      int
	MatchedLines      int
	ResultsReturned   int
	RawBodiesIncluded bool
	Results           []MemorySearchResult
}

type MemorySearchResult struct {
	Path              string
	Line              int
	Score             int
	MatchedTerms      int
	LoadedForThisTurn bool
	FileSHA           string
	LineSHA           string
}

func IsMemoryReportRequest(ev Event, cfg Config) bool {
	command := activeSlashCommand(ev, cfg)
	return command == "/memory" || command == "/memories"
}

func RenderMemoryReport(ev Event, cfg Config, repoContext RepoContext, transcript ...[]TranscriptMessage) string {
	var messages []TranscriptMessage
	if len(transcript) > 0 {
		messages = transcript[0]
	}
	if isMemoryVerifyRequest(ev, cfg) {
		return RenderMemoryVerifyReport(ev, cfg, repoContext)
	}
	if isMemoryValidateRequest(ev, cfg) {
		return RenderMemoryValidationReport(ev, cfg, repoContext)
	}
	if isMemoryPromotePlanRequest(ev, cfg) {
		return renderMemoryPromotePlanReport(ev, cfg, repoContext, messages, requestedMemoryPromoteTarget(ev, cfg), true)
	}
	if path := requestedMemoryInfoPath(ev, cfg); path != "" {
		return RenderMemoryInfoReport(ev, cfg, repoContext, path)
	}
	if query := requestedMemorySearchQuery(ev, cfg); query != "" {
		return RenderMemorySearchReport(ev, cfg, repoContext, query, defaultMemorySearchMaxResults)
	}
	return renderMemoryListReport(ev, cfg, repoContext, true)
}

func RenderMemoryCLIReport(cfg Config, repoContext RepoContext) string {
	return renderMemoryListReport(Event{}, cfg, repoContext, false)
}

func RenderMemoryInfoCLIReport(cfg Config, repoContext RepoContext, path string) string {
	return renderMemoryInfoReport(Event{}, cfg, repoContext, path, false)
}

func renderMemoryListReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	latest := latestMemoryNotePath(surface.DatedNotes)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
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
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	writeMemoryValidationSummary(&b, validation)
	b.WriteByte('\n')

	b.WriteString("GitClaw memory is repo-local Markdown loaded as read-only prompt context. This report never dumps memory bodies; hashes let maintainers verify exactly which git-backed memory files were present and loaded.\n\n")
	b.WriteString("Memory edits require normal reviewed git changes. GitClaw does not self-write `.gitclaw/MEMORY.md` or `.gitclaw/memory/*.md` during assistant turns.\n\n")

	b.WriteString("### Long-Term Memory\n")
	writeConfigSurfaceFile(&b, surface.LongTerm)

	b.WriteString("\n### Dated Memory Notes\n")
	writeMemorySurfaceFiles(&b, surface.DatedNotes)

	b.WriteString("\n### Loaded For This Turn\n")
	writeLoadedMemoryPaths(&b, surface)

	b.WriteString("\n### Validation\n")
	writeMemoryValidationFindings(&b, validation)

	return strings.TrimSpace(b.String())
}

func RenderMemorySearchReport(ev Event, cfg Config, repoContext RepoContext, query string, maxResults int) string {
	report := BuildMemorySearchReport(cfg.Workdir, repoContext, query, maxResults)
	var b strings.Builder
	b.WriteString("## GitClaw Memory Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- memory_search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- files_scanned: `%d`\n", report.FilesScanned)
	fmt.Fprintf(&b, "- matched_files: `%d`\n", report.MatchedFiles)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", report.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", report.RawBodiesIncluded)
	b.WriteString("This report searches git-backed memory files with a local lexical matcher. It reports paths, line numbers, scores, and hashes only; raw memory bodies, issue bodies, comments, prompts, and raw search queries are not included.\n\n")

	b.WriteString("### Results\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- path=`%s` line=`%d` score=`%d` matched_terms=`%d` loaded_for_this_turn=`%t` file_sha256_12=`%s` line_sha256_12=`%s`\n",
				result.Path,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.LoadedForThisTurn,
				result.FileSHA,
				result.LineSHA,
			)
		}
	}
	return strings.TrimSpace(b.String())
}

func RenderMemoryInfoReport(ev Event, cfg Config, repoContext RepoContext, path string) string {
	return renderMemoryInfoReport(ev, cfg, repoContext, path, true)
}

func renderMemoryInfoReport(ev Event, cfg Config, repoContext RepoContext, path string, includeIssue bool) string {
	surface := inspectMemorySurface(cfg.Workdir, repoContext)
	validation := ValidateMemory(cfg.Workdir, repoContext)
	normalized := normalizeMemoryInfoPath(path, surface)
	match, ok := memoryInfoMatch(surface, normalized)
	status := "not_found"
	if ok && match.File.Present {
		status = "ok"
	} else if ok {
		status = "missing"
	}
	if normalized == "" {
		status = "missing_path"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Memory Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_memory: `%s`\n", inlineCode(path))
	fmt.Fprintf(&b, "- normalized_memory_path: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- memory_info_status: `%s`\n", status)
	matched := 0
	if ok {
		matched = 1
	}
	fmt.Fprintf(&b, "- matched_memory_files: `%d`\n", matched)
	fmt.Fprintf(&b, "- memory_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- memory_writes_allowed: `%t`\n", false)
	writeMemoryValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report shows one repo-local memory file's metadata. Raw memory bodies, issue bodies, comments, prompts, and secret values are not included.\n\n")

	b.WriteString("### Match\n")
	if !ok {
		b.WriteString("- none\n")
	} else {
		writeMemoryInfoMatch(&b, match)
	}

	return strings.TrimSpace(b.String())
}

func isMemoryValidateRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/memory" || fields[0] == "/memories") && strings.EqualFold(fields[1], "validate")
}

func isMemoryVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && (fields[0] == "/memory" || fields[0] == "/memories") && strings.EqualFold(fields[1], "verify")
}

func requestedMemoryInfoPath(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || (fields[0] != "/memory" && fields[0] != "/memories") || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanMemoryInfoPath(strings.Join(fields[2:], " "))
}

func requestedMemorySearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || (fields[0] != "/memory" && fields[0] != "/memories") || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanMemorySearchQuery(strings.Join(fields[2:], " "))
}

func BuildMemorySearchReport(root string, repoContext RepoContext, query string, maxResults int) MemorySearchReport {
	query = cleanMemorySearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultMemorySearchMaxResults
	}
	report := MemorySearchReport{
		QueryHash:         shortDocumentHash(query),
		QueryTerms:        len(memorySearchTerms(query)),
		SearchStatus:      "ok",
		MaxResults:        maxResults,
		RawBodiesIncluded: false,
	}
	if query == "" {
		report.SearchStatus = "no_query"
		return report
	}
	terms := memorySearchTerms(query)
	if len(terms) == 0 {
		report.SearchStatus = "no_query"
		return report
	}
	surface := inspectMemorySurface(root, repoContext)
	files := memorySearchFiles(surface)
	report.FilesScanned = len(files)
	loaded := loadedMemoryPathSet(surface)
	var results []MemorySearchResult
	matchedFiles := map[string]bool{}
	for _, file := range files {
		if !file.Present {
			continue
		}
		body, err := readRepoTextFile(rootOrDot(root), file.Path, maxContextDocumentBytes)
		if err != nil {
			continue
		}
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			score, matchedTerms := memoryLineSearchScore(file.Path, line, query, terms)
			if score == 0 {
				continue
			}
			matchedFiles[file.Path] = true
			results = append(results, MemorySearchResult{
				Path:              file.Path,
				Line:              i + 1,
				Score:             score,
				MatchedTerms:      matchedTerms,
				LoadedForThisTurn: loaded[file.Path],
				FileSHA:           file.SHA,
				LineSHA:           shortDocumentHash(line),
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].Line < results[j].Line
	})
	report.MatchedFiles = len(matchedFiles)
	report.MatchedLines = len(results)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	report.Results = results
	report.ResultsReturned = len(results)
	if report.MatchedLines == 0 {
		report.SearchStatus = "no_matches"
	}
	return report
}

func memorySearchFiles(surface memorySurface) []configSurfaceFile {
	files := make([]configSurfaceFile, 0, 1+len(surface.DatedNotes))
	if surface.LongTerm.Present {
		files = append(files, surface.LongTerm)
	}
	files = append(files, surface.DatedNotes...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func loadedMemoryPathSet(surface memorySurface) map[string]bool {
	loaded := map[string]bool{}
	if surface.LoadedLongTerm {
		loaded[longTermMemoryPath] = true
	}
	for _, path := range surface.LoadedNotePaths {
		loaded[path] = true
	}
	return loaded
}

func memoryLineSearchScore(path, line, query string, terms []string) (int, int) {
	lowerLine := strings.ToLower(line)
	lowerPath := strings.ToLower(path)
	lowerQuery := strings.ToLower(query)
	score := 0
	if strings.Contains(lowerLine, lowerQuery) {
		score += 40
	}
	matchedTerms := 0
	for _, term := range terms {
		lineMatches := strings.Count(lowerLine, term)
		pathMatches := strings.Count(lowerPath, term)
		if lineMatches > 0 || pathMatches > 0 {
			matchedTerms++
			score += 10 + lineMatches*3 + pathMatches
		}
	}
	if path == longTermMemoryPath && score > 0 {
		score += 2
	}
	return score, matchedTerms
}

func memorySearchTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(cleanMemorySearchQuery(query)), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	})
	var terms []string
	for _, field := range fields {
		field = strings.Trim(field, "-_")
		if len(field) < 2 || isStopWord(field) {
			continue
		}
		if !containsStringFold(terms, field) {
			terms = append(terms, field)
		}
	}
	return terms
}

func cleanMemorySearchQuery(query string) string {
	return strings.Trim(strings.TrimSpace(query), " \t\r\n.,:;!?`\"'")
}

func cleanMemoryInfoPath(path string) string {
	return strings.Trim(strings.TrimSpace(path), " \t\r\n,;`\"'")
}

type memoryInfoMatchResult struct {
	File              configSurfaceFile
	Kind              string
	Source            string
	Canonical         bool
	Latest            bool
	LoadedForThisTurn bool
	AtContextLimit    bool
}

func normalizeMemoryInfoPath(raw string, surface memorySurface) string {
	value := cleanMemoryInfoPath(raw)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "./")
	lower := strings.ToLower(value)
	switch lower {
	case "long-term", "longterm", "memory", "memory.md", "long-term-memory", ".gitclaw/memory.md":
		return longTermMemoryPath
	case "latest", "latest-note", "latest-memory-note":
		return latestMemoryNotePath(surface.DatedNotes)
	}
	if datedMemoryDatePattern.MatchString(value) {
		return ".gitclaw/memory/" + value + ".md"
	}
	if datedMemoryBasenamePattern.MatchString(value) {
		return ".gitclaw/memory/" + value
	}
	if strings.HasPrefix(value, "memory/") {
		return ".gitclaw/" + value
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
}

var (
	datedMemoryDatePattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	datedMemoryBasenamePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.md$`)
)

func memoryInfoMatch(surface memorySurface, path string) (memoryInfoMatchResult, bool) {
	loaded := loadedMemoryPathSet(surface)
	latest := latestMemoryNotePath(surface.DatedNotes)
	for _, file := range memorySearchFiles(surface) {
		if file.Path != path {
			continue
		}
		return memoryInfoMatchResult{
			File:              file,
			Kind:              memoryFileKind(file.Path),
			Source:            memoryTrustSource(file.Path),
			Canonical:         memoryFileCanonical(file.Path),
			Latest:            file.Path == latest,
			LoadedForThisTurn: loaded[file.Path],
			AtContextLimit:    file.Bytes >= maxContextDocumentBytes,
		}, true
	}
	return memoryInfoMatchResult{}, false
}

func memoryFileKind(path string) string {
	if path == longTermMemoryPath {
		return "long-term"
	}
	if datedMemoryNotePattern.MatchString(path) {
		return "dated-note"
	}
	if strings.HasPrefix(path, ".gitclaw/memory/") {
		return "memory-note"
	}
	return "unknown"
}

func memoryFileCanonical(path string) bool {
	return path == longTermMemoryPath || datedMemoryNotePattern.MatchString(path)
}

func writeMemoryInfoMatch(b *strings.Builder, match memoryInfoMatchResult) {
	fmt.Fprintf(b, "- kind=`%s` path=`%s` source=`%s` present=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` at_context_limit=`%t`\n",
		match.Kind,
		match.File.Path,
		match.Source,
		match.File.Present,
		match.Canonical,
		match.Latest,
		match.LoadedForThisTurn,
		match.File.Bytes,
		match.File.Lines,
		match.File.SHA,
		match.AtContextLimit,
	)
}

func rootOrDot(root string) string {
	if root == "" {
		return "."
	}
	return root
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
