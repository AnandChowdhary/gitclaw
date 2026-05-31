package gitclaw

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type SoulProvenanceReport struct {
	Status                                  string
	Validation                              SoulValidationReport
	Risk                                    SoulRiskReport
	ContextDocuments                        int
	IdentityPolicyFiles                     int
	MemoryNotes                             int
	RepoLocalDocuments                      int
	GitTrackedDocuments                     int
	UntrackedDocuments                      int
	WorkingTreeDirtyDocuments               int
	DocumentsWithCommits                    int
	DocumentsWithoutCommits                 int
	GitAvailable                            bool
	GitHistoryAvailable                     bool
	RawBodiesIncluded                       bool
	RawGitSubjectsIncluded                  bool
	AuthorIdentitiesIncluded                bool
	SoulWritesAllowed                       bool
	LLME2ERequiredAfterSoulProvenanceChange bool
	Cards                                   []SoulProvenanceCard
	Findings                                []SoulProvenanceFinding
}

type SoulProvenanceCard struct {
	Path              string
	Category          string
	Source            string
	Required          bool
	LoadedForThisTurn bool
	Bytes             int
	Lines             int
	SHA               string
	GitTracked        bool
	WorkingTreeDirty  bool
	LastCommitSHA12   string
	LastCommitShort   string
	LastCommitDate    string
	SubjectSHA12      string
	CommitAvailable   bool
}

type SoulProvenanceFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func BuildSoulProvenanceReport(cfg Config, repoContext RepoContext) SoulProvenanceReport {
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	report := SoulProvenanceReport{
		Status:                                  soulProvenanceBaseStatus(validation.Status, risk.Status),
		Validation:                              validation,
		Risk:                                    risk,
		ContextDocuments:                        len(repoContext.Documents),
		IdentityPolicyFiles:                     soulIdentityDocumentCount(repoContext.Documents),
		MemoryNotes:                             soulMemoryDocumentCount(repoContext.Documents),
		GitAvailable:                            soulGitAvailable(),
		RawBodiesIncluded:                       false,
		RawGitSubjectsIncluded:                  false,
		AuthorIdentitiesIncluded:                false,
		SoulWritesAllowed:                       false,
		LLME2ERequiredAfterSoulProvenanceChange: true,
	}
	docs := append([]ContextDocument(nil), repoContext.Documents...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	for _, doc := range docs {
		if soulTrustSource(doc.Path) == "repo-local" {
			report.RepoLocalDocuments++
		}
		card := SoulProvenanceCard{
			Path:              doc.Path,
			Category:          soulDocumentCategory(doc.Path),
			Source:            soulTrustSource(doc.Path),
			Required:          isRequiredSoulDocument(doc.Path),
			LoadedForThisTurn: true,
			Bytes:             len(doc.Body),
			Lines:             lineCount(doc.Body),
			SHA:               shortDocumentHash(doc.Body),
			LastCommitSHA12:   "none",
			LastCommitShort:   "none",
			LastCommitDate:    "none",
			SubjectSHA12:      "none",
		}
		tracked, trackErr := soulGitTracked(cfg.Workdir, doc.Path)
		card.GitTracked = tracked
		if tracked {
			report.GitTrackedDocuments++
			card.WorkingTreeDirty = soulGitDirty(cfg.Workdir, doc.Path)
			if card.WorkingTreeDirty {
				report.WorkingTreeDirtyDocuments++
				report.addFinding("warning", "dirty_soul_file", doc.Path, "high-authority context file has uncommitted working-tree changes")
			}
			info, ok := soulGitLastCommit(cfg.Workdir, doc.Path)
			if ok {
				card.CommitAvailable = true
				card.LastCommitSHA12 = shortSHA(info.FullSHA)
				card.LastCommitShort = info.ShortSHA
				card.LastCommitDate = info.Date
				card.SubjectSHA12 = shortDocumentHash(info.Subject)
				report.DocumentsWithCommits++
				report.GitHistoryAvailable = true
			} else {
				report.DocumentsWithoutCommits++
				report.addFinding("warning", "missing_git_history", doc.Path, "no git commit was found for this high-authority context file")
			}
		} else {
			report.UntrackedDocuments++
			detail := "high-authority context file is not tracked by git"
			if trackErr != "" {
				detail = "git tracking check failed"
			}
			report.addFinding("warning", "untracked_soul_file", doc.Path, detail)
		}
		report.Cards = append(report.Cards, card)
	}
	if !report.GitAvailable {
		report.addFinding("warning", "git_not_available", "git", "git executable was not available for provenance checks")
	}
	if !report.GitHistoryAvailable {
		report.addFinding("warning", "git_history_not_available", "git", "no commit history was available for loaded high-authority context files")
	}
	if report.Status == "ok" && len(report.Findings) > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderSoulProvenanceReport(ev Event, cfg Config, repoContext RepoContext) string {
	return renderSoulProvenanceReport(ev, cfg, repoContext, ev.Repo != "" || ev.Issue.Number != 0)
}

func RenderSoulProvenanceCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSoulProvenanceReport(Event{}, cfg, repoContext, false)
}

func renderSoulProvenanceReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSoulProvenanceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_provenance_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- provenance_scope: `%s`\n", "repo-local-git-history")
	fmt.Fprintf(&b, "- context_documents: `%d`\n", report.ContextDocuments)
	fmt.Fprintf(&b, "- identity_policy_files: `%d`\n", report.IdentityPolicyFiles)
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", report.MemoryNotes)
	fmt.Fprintf(&b, "- repo_local_documents: `%d`\n", report.RepoLocalDocuments)
	fmt.Fprintf(&b, "- git_tracked_documents: `%d`\n", report.GitTrackedDocuments)
	fmt.Fprintf(&b, "- untracked_documents: `%d`\n", report.UntrackedDocuments)
	fmt.Fprintf(&b, "- working_tree_dirty_documents: `%d`\n", report.WorkingTreeDirtyDocuments)
	fmt.Fprintf(&b, "- documents_with_commits: `%d`\n", report.DocumentsWithCommits)
	fmt.Fprintf(&b, "- documents_without_commits: `%d`\n", report.DocumentsWithoutCommits)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	fmt.Fprintf(&b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	fmt.Fprintf(&b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(&b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(&b, "- soul_writes_allowed: `%t`\n", report.SoulWritesAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_soul_provenance_change: `%t`\n", report.LLME2ERequiredAfterSoulProvenanceChange)
	writeSoulValidationSummary(&b, report.Validation)
	writeSoulRiskSummary(&b, report.Risk)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report maps loaded high-authority context files to body-free git provenance. It reports paths, categories, hashes, tracked state, last commit IDs/dates, and commit-subject hashes only; raw context bodies, issue bodies, comments, prompts, git subjects, author identities, and secret values are not included.\n\n")

	b.WriteString("### Provenance Cards\n")
	if len(report.Cards) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, card := range report.Cards {
			writeSoulProvenanceCard(&b, card)
		}
	}

	b.WriteString("\n### Provenance Gates\n")
	fmt.Fprintf(&b, "- validation_gate=`%s`\n", soulAnchorGate(report.Validation.Status))
	fmt.Fprintf(&b, "- risk_gate=`%s`\n", soulAnchorGate(report.Risk.Status))
	fmt.Fprintf(&b, "- git_history_gate=`%s`\n", soulProvenanceGitGate(report))
	fmt.Fprintf(&b, "- mutation_gate=`%s`\n", "disabled")

	b.WriteString("\n### Findings\n")
	writeSoulProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func writeSoulProvenanceCard(b *strings.Builder, card SoulProvenanceCard) {
	fmt.Fprintf(
		b,
		"- path=`%s` category=`%s` source=`%s` required=`%t` loaded_for_this_turn=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
		card.Path,
		card.Category,
		card.Source,
		card.Required,
		card.LoadedForThisTurn,
		card.Bytes,
		card.Lines,
		card.SHA,
		card.GitTracked,
		card.WorkingTreeDirty,
		card.CommitAvailable,
		card.LastCommitSHA12,
		card.LastCommitShort,
		card.LastCommitDate,
		card.SubjectSHA12,
	)
}

func writeSoulProvenanceFindings(b *strings.Builder, findings []SoulProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func soulProvenanceBaseStatus(validationStatus, riskStatus string) string {
	if riskStatus == "high" {
		return "high"
	}
	if validationStatus == "error" {
		return "error"
	}
	if validationStatus == "warn" || riskStatus == "warn" {
		return "warn"
	}
	if validationStatus == "" && riskStatus == "" {
		return "unknown"
	}
	return "ok"
}

func soulProvenanceGitGate(report SoulProvenanceReport) string {
	if !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedDocuments > 0 || report.DocumentsWithoutCommits > 0 || report.WorkingTreeDirtyDocuments > 0 {
		return "warn"
	}
	return "pass"
}

func (r *SoulProvenanceReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SoulProvenanceFinding{Severity: severity, Code: code, Path: path, Detail: detail})
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return r.Findings[i].Severity < r.Findings[j].Severity
		}
		if r.Findings[i].Code != r.Findings[j].Code {
			return r.Findings[i].Code < r.Findings[j].Code
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}

type soulGitCommitInfo struct {
	FullSHA  string
	ShortSHA string
	Date     string
	Subject  string
}

func soulGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func soulGitTracked(root, path string) (bool, string) {
	if !soulGitAvailable() {
		return false, "git_not_available"
	}
	out, err := runSoulGit(root, "ls-files", "--error-unmatch", "--", path)
	if err != nil {
		return false, strings.TrimSpace(out)
	}
	return true, ""
}

func soulGitLastCommit(root, path string) (soulGitCommitInfo, bool) {
	out, err := runSoulGit(root, "log", "-1", "--date=iso-strict", "--format=%H%x00%h%x00%aI%x00%s", "--", path)
	if err != nil || strings.TrimSpace(out) == "" {
		return soulGitCommitInfo{}, false
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\x00", 4)
	if len(parts) < 4 {
		return soulGitCommitInfo{}, false
	}
	return soulGitCommitInfo{FullSHA: parts[0], ShortSHA: parts[1], Date: parts[2], Subject: parts[3]}, true
}

func soulGitDirty(root, path string) bool {
	out, err := runSoulGit(root, "status", "--porcelain", "--", path)
	return err == nil && strings.TrimSpace(out) != ""
}

func runSoulGit(root string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", rootOrDot(root)}, args...)
	cmd := exec.Command("git", cmdArgs...)
	data, err := cmd.CombinedOutput()
	return string(data), err
}

func shortSHA(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
