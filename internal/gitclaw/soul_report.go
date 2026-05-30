package gitclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	if isSoulValidateRequest(ev, cfg) {
		return renderSoulValidationReport(ev, repoContext, true)
	}
	if query := requestedSoulSearchQuery(ev, cfg); query != "" {
		return RenderSoulSearchReport(ev, repoContext, query, defaultSoulSearchMaxResults)
	}
	return renderSoulListReport(ev, repoContext, true)
}

func RenderSoulCLIReport(repoContext RepoContext) string {
	return renderSoulListReport(Event{}, repoContext, false)
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
