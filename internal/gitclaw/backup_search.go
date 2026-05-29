package gitclaw

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const defaultBackupSearchMaxResults = 10

type BackupSearchReport struct {
	Root                       string
	Repo                       string
	RepoDir                    string
	IndexPath                  string
	ReadmePath                 string
	SchemaVersion              int
	IndexGeneratedAt           string
	QueryHash                  string
	QueryTerms                 int
	SearchStatus               string
	MaxResults                 int
	BackupVerifyStatus         string
	VerificationFailures       int
	IssueCount                 int
	IssueFieldsSearched        int
	CommentBodiesSearched      int
	TranscriptMessagesSearched int
	MatchedIssues              int
	MatchedLines               int
	ResultsReturned            int
	RawBodiesIncluded          bool
	Results                    []BackupSearchResult
}

type BackupSearchResult struct {
	IssueNumber       int
	Path              string
	Source            string
	Role              string
	Actor             string
	AuthorAssociation string
	Trusted           bool
	Line              int
	Score             int
	MatchedTerms      int
	BodyBytes         int
	BodyLines         int
	BodySHA           string
	LineSHA           string
	BackupGeneratedAt string
	EventName         string
}

func BuildBackupSearch(root, repo, query string, maxResults int) (BackupSearchReport, error) {
	if err := validateRepoName(repo); err != nil {
		return BackupSearchReport{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	if maxResults <= 0 {
		maxResults = defaultBackupSearchMaxResults
	}
	query = cleanMemorySearchQuery(query)
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupSearchReport{}, err
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupSearchReport{}, err
	}
	report := BackupSearchReport{
		Root:                       filepath.ToSlash(root),
		Repo:                       repo,
		RepoDir:                    filepath.ToSlash(repoDir),
		IndexPath:                  filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                 filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:              index.Version,
		IndexGeneratedAt:           index.GeneratedAt,
		QueryHash:                  shortDocumentHash(query),
		QueryTerms:                 len(memorySearchTerms(query)),
		SearchStatus:               "ok",
		MaxResults:                 maxResults,
		BackupVerifyStatus:         "ok",
		VerificationFailures:       len(verify.VerificationFailures),
		IssueCount:                 len(index.Issues),
		IssueFieldsSearched:        len(index.Issues) * 2,
		RawBodiesIncluded:          false,
		CommentBodiesSearched:      0,
		TranscriptMessagesSearched: 0,
	}
	if !verify.OK() {
		report.BackupVerifyStatus = "warn"
	}
	terms := memorySearchTerms(query)
	if query == "" || len(terms) == 0 {
		report.SearchStatus = "no_query"
		return report, nil
	}

	matchedIssues := map[int]bool{}
	var results []BackupSearchResult
	for _, issue := range index.Issues {
		backup, err := readIndexedBackup(repoDir, repo, issue)
		if err != nil {
			return BackupSearchReport{}, err
		}
		appendBackupSearchResults(&results, matchedIssues, issue, backup, backup.Issue.Title, backupSearchSource{
			Name:              "issue.title",
			Role:              "user",
			Actor:             backup.Issue.Author,
			AuthorAssociation: backup.Issue.AuthorAssociation,
			Trusted:           trustedAssociation(backup.Issue.AuthorAssociation, DefaultConfig()),
		}, query, terms)
		appendBackupSearchResults(&results, matchedIssues, issue, backup, backup.Issue.Body, backupSearchSource{
			Name:              "issue.body",
			Role:              "user",
			Actor:             backup.Issue.Author,
			AuthorAssociation: backup.Issue.AuthorAssociation,
			Trusted:           trustedAssociation(backup.Issue.AuthorAssociation, DefaultConfig()),
		}, query, terms)
		report.CommentBodiesSearched += len(backup.Comments)
		for _, comment := range backup.Comments {
			role := "comment"
			trusted := trustedAssociation(comment.AuthorAssociation, DefaultConfig())
			if HasGitClawMarker(comment.Body) {
				role = "assistant"
				trusted = true
			}
			appendBackupSearchResults(&results, matchedIssues, issue, backup, comment.Body, backupSearchSource{
				Name:              fmt.Sprintf("comment:%d", comment.ID),
				Role:              role,
				Actor:             comment.Author,
				AuthorAssociation: comment.AuthorAssociation,
				Trusted:           trusted,
			}, query, terms)
		}
		report.TranscriptMessagesSearched += len(backup.Transcript)
		for index, msg := range backup.Transcript {
			appendBackupSearchResults(&results, matchedIssues, issue, backup, msg.Body, backupSearchSource{
				Name:              fmt.Sprintf("transcript:%02d", index+1),
				Role:              msg.Role,
				Actor:             msg.Actor,
				AuthorAssociation: msg.AuthorAssociation,
				Trusted:           msg.Trusted,
			}, query, terms)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].BackupGeneratedAt != results[j].BackupGeneratedAt {
			return results[i].BackupGeneratedAt > results[j].BackupGeneratedAt
		}
		if results[i].IssueNumber != results[j].IssueNumber {
			return results[i].IssueNumber > results[j].IssueNumber
		}
		if results[i].Source != results[j].Source {
			return results[i].Source < results[j].Source
		}
		return results[i].Line < results[j].Line
	})
	report.MatchedIssues = len(matchedIssues)
	report.MatchedLines = len(results)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	report.Results = results
	report.ResultsReturned = len(results)
	if report.MatchedLines == 0 {
		report.SearchStatus = "no_matches"
	}
	return report, nil
}

func RenderBackupSearchReport(report BackupSearchReport) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", report.Repo)
	fmt.Fprintf(&b, "- backup_search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", report.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", report.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", report.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", report.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", report.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", report.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", report.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", report.IndexGeneratedAt)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", report.IssueCount)
	fmt.Fprintf(&b, "- issue_fields_searched: `%d`\n", report.IssueFieldsSearched)
	fmt.Fprintf(&b, "- comment_bodies_searched: `%d`\n", report.CommentBodiesSearched)
	fmt.Fprintf(&b, "- transcript_messages_searched: `%d`\n", report.TranscriptMessagesSearched)
	fmt.Fprintf(&b, "- matched_issues: `%d`\n", report.MatchedIssues)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", report.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n\n", report.RawBodiesIncluded)
	b.WriteString("This report searches a fetched local backup tree with a lexical matcher. It reports issue paths, sources, trust metadata, line numbers, scores, and hashes only; raw issue titles, issue bodies, comment bodies, transcript messages, prompts, and raw search queries are not included.\n\n")

	b.WriteString("### Results\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- issue=`#%d` path=`%s` source=`%s` role=`%s` actor=`%s` association=`%s` trusted=`%t` line=`%d` score=`%d` matched_terms=`%d` body_bytes=`%d` body_lines=`%d` body_sha256_12=`%s` line_sha256_12=`%s` generated_at=`%s` event=`%s`\n",
				result.IssueNumber,
				result.Path,
				result.Source,
				result.Role,
				inlineCode(result.Actor),
				inlineCode(result.AuthorAssociation),
				result.Trusted,
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.BodyBytes,
				result.BodyLines,
				result.BodySHA,
				result.LineSHA,
				result.BackupGeneratedAt,
				inlineCode(result.EventName),
			)
		}
	}
	return strings.TrimSpace(b.String())
}

type backupSearchSource struct {
	Name              string
	Role              string
	Actor             string
	AuthorAssociation string
	Trusted           bool
}

func appendBackupSearchResults(results *[]BackupSearchResult, matchedIssues map[int]bool, issue BackupIndexIssue, backup IssueBackup, body string, source backupSearchSource, query string, terms []string) {
	lines := strings.Split(body, "\n")
	for lineIndex, line := range lines {
		score, matchedTerms := memoryLineSearchScore("", line, query, terms)
		if score == 0 {
			continue
		}
		matchedIssues[backup.Issue.Number] = true
		*results = append(*results, BackupSearchResult{
			IssueNumber:       backup.Issue.Number,
			Path:              filepath.ToSlash(issue.Path),
			Source:            source.Name,
			Role:              source.Role,
			Actor:             source.Actor,
			AuthorAssociation: source.AuthorAssociation,
			Trusted:           source.Trusted,
			Line:              lineIndex + 1,
			Score:             score,
			MatchedTerms:      matchedTerms,
			BodyBytes:         len(body),
			BodyLines:         lineCount(body),
			BodySHA:           shortDocumentHash(body),
			LineSHA:           shortDocumentHash(line),
			BackupGeneratedAt: backup.GeneratedAt,
			EventName:         backup.EventName,
		})
	}
}
