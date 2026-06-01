package gitclaw

import (
	"fmt"
	"sort"
	"strings"
)

const defaultProfileSearchMaxResults = 10

type ProfileSearchReport struct {
	QueryHash                              string
	QueryTerms                             int
	SearchStatus                           string
	SearchScope                            string
	MaxResults                             int
	ProfileDocumentsLoaded                 int
	ManifestEntries                        int
	FilesScanned                           int
	MatchedFiles                           int
	MatchedLines                           int
	ResultsReturned                        int
	RawBodiesIncluded                      bool
	RawProfileBodiesIncluded               bool
	RawSkillBodiesIncluded                 bool
	RawToolOutputsIncluded                 bool
	RawIssueBodiesIncluded                 bool
	RawCommentBodiesIncluded               bool
	RawPromptBodiesIncluded                bool
	RawQueriesIncluded                     bool
	ProfileMutationAllowed                 bool
	LLME2ERequiredAfterProfileSearchChange bool
	ExternalProfileHomeAccessed            bool
	ProfileExportSupported                 bool
	ProfileImportSupported                 bool
	ProfileSwitchingSupported              bool
	ProfileDistributionInstallSupported    bool
	Results                                []ProfileSearchResult
}

type ProfileSearchResult struct {
	Kind          string
	Name          string
	Path          string
	Category      string
	Source        string
	IncludePolicy string
	Line          int
	Score         int
	MatchedTerms  int
	Selected      bool
	Enabled       bool
	FileSHA       string
	LineSHA       string
}

func BuildProfileSearchReport(cfg Config, repoContext RepoContext, query string, maxResults int) ProfileSearchReport {
	if maxResults <= 0 {
		maxResults = defaultProfileSearchMaxResults
	}
	query = cleanMemorySearchQuery(query)
	manifest := BuildProfileManifestReport(cfg, repoContext)
	report := ProfileSearchReport{
		QueryHash:                              shortDocumentHash(query),
		SearchStatus:                           "ok",
		SearchScope:                            "repo-local-profile-files",
		MaxResults:                             maxResults,
		ProfileDocumentsLoaded:                 manifest.ProfileDocumentsLoaded,
		ManifestEntries:                        manifest.ManifestEntries,
		RawBodiesIncluded:                      false,
		RawProfileBodiesIncluded:               false,
		RawSkillBodiesIncluded:                 false,
		RawToolOutputsIncluded:                 false,
		RawIssueBodiesIncluded:                 false,
		RawCommentBodiesIncluded:               false,
		RawPromptBodiesIncluded:                false,
		RawQueriesIncluded:                     false,
		ProfileMutationAllowed:                 false,
		LLME2ERequiredAfterProfileSearchChange: true,
		ExternalProfileHomeAccessed:            false,
		ProfileExportSupported:                 false,
		ProfileImportSupported:                 false,
		ProfileSwitchingSupported:              false,
		ProfileDistributionInstallSupported:    false,
	}
	terms := memorySearchTerms(query)
	report.QueryTerms = len(terms)
	if query == "" || len(terms) == 0 {
		report.SearchStatus = "no_query"
		return report
	}

	entries := profileSearchEntries(manifest.Entries)
	report.FilesScanned = len(entries)
	var results []ProfileSearchResult
	matchedFiles := map[string]bool{}
	for _, entry := range entries {
		body, err := readRepoTextFile(rootOrDot(cfg.Workdir), entry.Path, maxContextDocumentBytes)
		if err != nil {
			continue
		}
		lines := strings.Split(body, "\n")
		fileSHA := entry.SHA
		if fileSHA == "" {
			fileSHA = shortDocumentHash(body)
		}
		for i, line := range lines {
			score, matchedTerms := profileLineSearchScore(entry, line, query, terms)
			if score == 0 {
				continue
			}
			matchedFiles[entry.Path] = true
			results = append(results, ProfileSearchResult{
				Kind:          entry.Kind,
				Name:          entry.Name,
				Path:          entry.Path,
				Category:      entry.Category,
				Source:        entry.Source,
				IncludePolicy: entry.IncludePolicy,
				Line:          i + 1,
				Score:         score,
				MatchedTerms:  matchedTerms,
				Selected:      entry.Selected,
				Enabled:       entry.Enabled,
				FileSHA:       fileSHA,
				LineSHA:       shortDocumentHash(line),
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

func RenderProfileSearchCLIReport(cfg Config, repoContext RepoContext, query string) string {
	return renderProfileSearchReport(Event{}, cfg, repoContext, query, defaultProfileSearchMaxResults, false)
}

func renderProfileSearchReport(ev Event, cfg Config, repoContext RepoContext, query string, maxResults int, includeIssue bool) string {
	report := BuildProfileSearchReport(cfg, repoContext, query, maxResults)
	var b strings.Builder
	b.WriteString("## GitClaw Profile Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- profile_search_status: `%s`\n", report.SearchStatus)
	fmt.Fprintf(&b, "- search_scope: `%s`\n", report.SearchScope)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- profile_documents_loaded: `%d`\n", report.ProfileDocumentsLoaded)
	fmt.Fprintf(&b, "- manifest_entries: `%d`\n", report.ManifestEntries)
	fmt.Fprintf(&b, "- files_scanned: `%d`\n", report.FilesScanned)
	fmt.Fprintf(&b, "- matched_files: `%d`\n", report.MatchedFiles)
	fmt.Fprintf(&b, "- matched_lines: `%d`\n", report.MatchedLines)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_profile_bodies_included: `%t`\n", report.RawProfileBodiesIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- raw_tool_outputs_included: `%t`\n", report.RawToolOutputsIncluded)
	fmt.Fprintf(&b, "- raw_issue_bodies_included: `%t`\n", report.RawIssueBodiesIncluded)
	fmt.Fprintf(&b, "- raw_comment_bodies_included: `%t`\n", report.RawCommentBodiesIncluded)
	fmt.Fprintf(&b, "- raw_prompt_bodies_included: `%t`\n", report.RawPromptBodiesIncluded)
	fmt.Fprintf(&b, "- raw_queries_included: `%t`\n", report.RawQueriesIncluded)
	fmt.Fprintf(&b, "- external_profile_home_accessed: `%t`\n", report.ExternalProfileHomeAccessed)
	fmt.Fprintf(&b, "- profile_export_supported: `%t`\n", report.ProfileExportSupported)
	fmt.Fprintf(&b, "- profile_import_supported: `%t`\n", report.ProfileImportSupported)
	fmt.Fprintf(&b, "- profile_switching_supported: `%t`\n", report.ProfileSwitchingSupported)
	fmt.Fprintf(&b, "- profile_distribution_install_supported: `%t`\n", report.ProfileDistributionInstallSupported)
	fmt.Fprintf(&b, "- profile_mutation_allowed: `%t`\n", report.ProfileMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_profile_search_change: `%t`\n", report.LLME2ERequiredAfterProfileSearchChange)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report searches GitClaw's repo-local `.gitclaw/` profile envelope with a local lexical matcher. It reports only paths, categories, line numbers, scores, and hashes; raw profile files, skill bodies, memory bodies, tool outputs, issue/comment bodies, prompts, raw search queries, sessions, backup payloads, credentials, and secret values are not included.\n\n")

	b.WriteString("### Results\n")
	if len(report.Results) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, result := range report.Results {
			fmt.Fprintf(&b, "- kind=`%s` name=`%s` path=`%s` category=`%s` source=`%s` include_policy=`%s` line=`%d` score=`%d` matched_terms=`%d` selected=`%t` enabled=`%t` file_sha256_12=`%s` line_sha256_12=`%s`\n",
				inlineCode(result.Kind),
				inlineCode(result.Name),
				result.Path,
				inlineCode(result.Category),
				inlineCode(result.Source),
				inlineCode(result.IncludePolicy),
				result.Line,
				result.Score,
				result.MatchedTerms,
				result.Selected,
				result.Enabled,
				noneIfEmpty(result.FileSHA),
				result.LineSHA,
			)
		}
	}
	b.WriteString("\n### Search Gates\n")
	b.WriteString("- query_gate=`sha256_12_only`\n")
	b.WriteString("- raw_body_gate=`hashes-and-line-hashes-only`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- external_profile_home_gate=`not_accessed`\n")
	b.WriteString("- session_payload_gate=`excluded`\n")
	b.WriteString("- backup_payload_gate=`excluded`\n")
	b.WriteString("- llm_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func profileSearchEntries(entries []ProfileManifestEntry) []ProfileManifestEntry {
	seen := map[string]bool{}
	filtered := make([]ProfileManifestEntry, 0, len(entries))
	for _, entry := range entries {
		if !profileSearchEntryEligible(entry) || seen[entry.Path] {
			continue
		}
		seen[entry.Path] = true
		filtered = append(filtered, entry)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Path != filtered[j].Path {
			return filtered[i].Path < filtered[j].Path
		}
		return filtered[i].Kind < filtered[j].Kind
	})
	return filtered
}

func profileSearchEntryEligible(entry ProfileManifestEntry) bool {
	return profileProvenanceEntryEligible(entry)
}

func profileLineSearchScore(entry ProfileManifestEntry, line, query string, terms []string) (int, int) {
	score, matchedTerms := memoryLineSearchScore(entry.Path, line, query, terms)
	if score == 0 {
		return 0, 0
	}
	if entry.Kind == "profile-document" {
		score += 2
	}
	if entry.Selected {
		score += 2
	}
	if entry.Enabled {
		score++
	}
	return score, matchedTerms
}

func isProfileSearchRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 2 || (fields[0] != "/profile" && fields[0] != "/profiles") {
		return false
	}
	return strings.EqualFold(fields[1], "search")
}

func requestedProfileSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 3 || (fields[0] != "/profile" && fields[0] != "/profiles") || !strings.EqualFold(fields[1], "search") {
		return ""
	}
	return cleanMemorySearchQuery(strings.Join(fields[2:], " "))
}
