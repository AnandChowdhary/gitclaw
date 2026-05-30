package gitclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const defaultSoulSearchMaxResults = 10

type SoulSearchReport struct {
	QueryHash         string
	QueryTerms        int
	SearchStatus      string
	MaxResults        int
	FilesScanned      int
	MatchedFiles      int
	MatchedLines      int
	ResultsReturned   int
	RawBodiesIncluded bool
	Results           []SoulSearchResult
}

type SoulSearchResult struct {
	Path         string
	Category     string
	Line         int
	Score        int
	MatchedTerms int
	FileSHA      string
	LineSHA      string
}

func IsSoulReportRequest(ev Event, cfg Config) bool {
	return activeSlashCommand(ev, cfg) == "/soul"
}

func RenderSoulReport(ev Event, cfg Config, repoContext RepoContext) string {
	if isSoulVerifyRequest(ev, cfg) {
		return renderSoulVerifyReport(ev, repoContext, true)
	}
	if isSoulValidateRequest(ev, cfg) {
		return renderSoulValidationReport(ev, repoContext, true)
	}
	if target := requestedSoulEditPlanPath(ev, cfg); target != "" {
		if target == "__missing__" {
			target = ""
		}
		return renderSoulEditPlanReport(ev, cfg, repoContext, target, true)
	}
	if path := requestedSoulInfoPath(ev, cfg); path != "" {
		return RenderSoulInfoReport(ev, cfg, repoContext, path)
	}
	if query := requestedSoulSearchQuery(ev, cfg); query != "" {
		return RenderSoulSearchReport(ev, repoContext, query, defaultSoulSearchMaxResults)
	}
	return renderSoulListReport(ev, repoContext, true)
}

func RenderSoulCLIReport(repoContext RepoContext) string {
	return renderSoulListReport(Event{}, repoContext, false)
}

func RenderSoulInfoCLIReport(cfg Config, repoContext RepoContext, path string) string {
	return renderSoulInfoReport(Event{}, cfg, repoContext, path, false)
}

func renderSoulListReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	validation := ValidateSoulContext(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- identity_policy_files: `%d`\n", soulIdentityDocumentCount(repoContext.Documents))
	fmt.Fprintf(&b, "- memory_notes: `%d`\n\n", soulMemoryDocumentCount(repoContext.Documents))
	writeSoulValidationSummary(&b, validation)
	b.WriteString("\n")
	b.WriteString("File bodies are not included; hashes let maintainers verify exactly which git-backed context was loaded.\n\n")

	b.WriteString("### Identity And Policy Files\n")
	writeSoulDocumentList(&b, repoContext.Documents, false)

	b.WriteString("\n### Memory Notes\n")
	writeSoulDocumentList(&b, repoContext.Documents, true)

	b.WriteString("\n### Validation\n")
	writeSoulValidationFindings(&b, validation)

	return strings.TrimSpace(b.String())
}

func RenderSoulSearchReport(ev Event, repoContext RepoContext, query string, maxResults int) string {
	report := BuildSoulSearchReport(repoContext, query, maxResults)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if ev.Repo != "" {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
	}
	if ev.Issue.Number != 0 {
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- files_scanned: `%d`\n", report.FilesScanned)
	fmt.Fprintf(&b, "- matched_files: `%d`\n", report.MatchedFiles)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", report.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", report.RawBodiesIncluded)
	b.WriteString("This report searches loaded high-authority GitClaw context files with a local lexical matcher. It reports paths, categories, line numbers, scores, and hashes only; raw soul, user, memory, tool, heartbeat, issue, comment, prompt, and raw search query bodies are not included.\n\n")

	b.WriteString("### Results\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- path=`%s` category=`%s` line=`%d` score=`%d` matched_terms=`%d` file_sha256_12=`%s` line_sha256_12=`%s`\n",
				result.Path,
				result.Category,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.FileSHA,
				result.LineSHA,
			)
		}
	}
	return strings.TrimSpace(b.String())
}

func RenderSoulInfoReport(ev Event, cfg Config, repoContext RepoContext, path string) string {
	return renderSoulInfoReport(ev, cfg, repoContext, path, true)
}

func renderSoulInfoReport(ev Event, cfg Config, repoContext RepoContext, path string, includeIssue bool) string {
	normalized := normalizeSoulInfoPath(path, cfg, repoContext)
	match, ok := soulInfoMatch(cfg.Workdir, repoContext, normalized)
	status := "not_found"
	if ok && match.Present {
		status = "ok"
	} else if ok {
		status = "missing"
	}
	if normalized == "" {
		status = "missing_path"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Soul Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- requested_soul: `%s`\n", inlineCode(path))
	fmt.Fprintf(&b, "- normalized_soul_path: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- soul_info_status: `%s`\n", status)
	matched := 0
	if ok {
		matched = 1
	}
	fmt.Fprintf(&b, "- matched_soul_files: `%d`\n", matched)
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", false)
	writeSoulValidationSummary(&b, ValidateSoulContext(repoContext))
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report shows one repo-local high-authority context file's metadata. Raw soul, identity, user, memory, tools, heartbeat, issue, comment, prompt, and secret bodies are not included.\n\n")

	b.WriteString("### Match\n")
	if !ok {
		b.WriteString("- none\n")
	} else {
		writeSoulInfoMatch(&b, match)
	}

	return strings.TrimSpace(b.String())
}

func requestedSoulInfoPath(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/soul" || !strings.EqualFold(fields[1], "info") {
		return ""
	}
	return cleanSoulInfoPath(strings.Join(fields[2:], " "))
}

func requestedSoulSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || fields[0] != "/soul" || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanMemorySearchQuery(strings.Join(fields[2:], " "))
}

func isSoulValidateRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/soul" && strings.EqualFold(fields[1], "validate")
}

func isSoulVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 && fields[0] == "/soul" && strings.EqualFold(fields[1], "verify")
}

func BuildSoulSearchReport(repoContext RepoContext, query string, maxResults int) SoulSearchReport {
	query = cleanMemorySearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultSoulSearchMaxResults
	}
	report := SoulSearchReport{
		QueryHash:         shortDocumentHash(query),
		QueryTerms:        len(memorySearchTerms(query)),
		SearchStatus:      "ok",
		MaxResults:        maxResults,
		FilesScanned:      len(repoContext.Documents),
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
	docs := append([]ContextDocument(nil), repoContext.Documents...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })

	var results []SoulSearchResult
	matchedFiles := map[string]bool{}
	for _, doc := range docs {
		lines := strings.Split(doc.Body, "\n")
		for i, line := range lines {
			score, matchedTerms := soulLineSearchScore(doc.Path, line, query, terms)
			if score == 0 {
				continue
			}
			matchedFiles[doc.Path] = true
			results = append(results, SoulSearchResult{
				Path:         doc.Path,
				Category:     soulDocumentCategory(doc.Path),
				Line:         i + 1,
				Score:        score,
				MatchedTerms: matchedTerms,
				FileSHA:      shortDocumentHash(doc.Body),
				LineSHA:      shortDocumentHash(line),
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

type soulInfoMatchResult struct {
	Path              string
	Category          string
	Source            string
	Present           bool
	Required          bool
	Canonical         bool
	Latest            bool
	LoadedForThisTurn bool
	Bytes             int
	Lines             int
	SHA               string
	AtContextLimit    bool
}

func cleanSoulInfoPath(path string) string {
	return strings.Trim(strings.TrimSpace(path), " \t\r\n,;`\"'")
}

func normalizeSoulInfoPath(raw string, cfg Config, repoContext RepoContext) string {
	value := cleanSoulInfoPath(raw)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "./")
	lower := strings.ToLower(value)
	switch lower {
	case "soul", "soul.md", ".gitclaw/soul.md":
		return ".gitclaw/SOUL.md"
	case "identity", "identity.md", ".gitclaw/identity.md":
		return ".gitclaw/IDENTITY.md"
	case "user", "user.md", ".gitclaw/user.md":
		return ".gitclaw/USER.md"
	case "tools", "tools.md", ".gitclaw/tools.md":
		return ".gitclaw/TOOLS.md"
	case "memory", "memory.md", ".gitclaw/memory.md":
		return ".gitclaw/MEMORY.md"
	case "heartbeat", "heartbeat.md", ".gitclaw/heartbeat.md":
		return ".gitclaw/HEARTBEAT.md"
	case "latest", "latest-memory", "latest-memory-note":
		surface := inspectMemorySurface(cfg.Workdir, repoContext)
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

func soulInfoMatch(root string, repoContext RepoContext, path string) (soulInfoMatchResult, bool) {
	if path == "" || !soulInfoAllowedPath(path) {
		return soulInfoMatchResult{}, false
	}
	body, err := readRepoTextFile(rootOrDot(root), path, maxContextDocumentBytes)
	present := err == nil
	loaded := false
	for _, doc := range repoContext.Documents {
		if doc.Path == path {
			loaded = true
			if !present {
				body = doc.Body
				present = true
			}
			break
		}
	}
	latest := latestMemoryNotePath(inspectMemorySurface(root, repoContext).DatedNotes)
	return soulInfoMatchResult{
		Path:              path,
		Category:          soulDocumentCategory(path),
		Source:            soulTrustSource(path),
		Present:           present,
		Required:          soulRequiredPath(path),
		Canonical:         soulInfoCanonicalPath(path),
		Latest:            path == latest,
		LoadedForThisTurn: loaded,
		Bytes:             len(body),
		Lines:             lineCount(body),
		SHA:               shortDocumentHash(body),
		AtContextLimit:    len(body) >= maxContextDocumentBytes,
	}, true
}

func soulInfoAllowedPath(path string) bool {
	if soulRequiredPath(path) {
		return true
	}
	return strings.HasPrefix(path, ".gitclaw/memory/") && strings.HasSuffix(path, ".md")
}

func soulRequiredPath(path string) bool {
	for _, required := range requiredSoulDocumentPaths {
		if path == required {
			return true
		}
	}
	return false
}

func soulInfoCanonicalPath(path string) bool {
	return soulRequiredPath(path) || datedMemoryNotePattern.MatchString(path)
}

func writeSoulInfoMatch(b *strings.Builder, match soulInfoMatchResult) {
	fmt.Fprintf(b, "- category=`%s` path=`%s` source=`%s` present=`%t` required=`%t` canonical=`%t` latest=`%t` loaded_for_this_turn=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` at_context_limit=`%t`\n",
		match.Category,
		match.Path,
		match.Source,
		match.Present,
		match.Required,
		match.Canonical,
		match.Latest,
		match.LoadedForThisTurn,
		match.Bytes,
		match.Lines,
		match.SHA,
		match.AtContextLimit,
	)
}

func soulLineSearchScore(path, line, query string, terms []string) (int, int) {
	score, matchedTerms := memoryLineSearchScore(path, line, query, terms)
	if score == 0 {
		return 0, 0
	}
	switch path {
	case ".gitclaw/SOUL.md":
		score += 4
	case ".gitclaw/IDENTITY.md", ".gitclaw/USER.md":
		score += 2
	}
	return score, matchedTerms
}

func soulDocumentCategory(path string) string {
	switch {
	case path == ".gitclaw/SOUL.md":
		return "soul"
	case path == ".gitclaw/IDENTITY.md":
		return "identity"
	case path == ".gitclaw/USER.md":
		return "user"
	case path == ".gitclaw/TOOLS.md":
		return "tools"
	case path == ".gitclaw/MEMORY.md":
		return "memory"
	case path == ".gitclaw/HEARTBEAT.md":
		return "heartbeat"
	case isSoulMemoryNote(path):
		return "memory-note"
	default:
		return "context"
	}
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
