package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type BackupProvenance struct {
	Root                                      string
	Repo                                      string
	RepoDir                                   string
	IndexPath                                 string
	ReadmePath                                string
	SchemaVersion                             int
	IndexGeneratedAt                          string
	BackupProvenanceStatus                    string
	BackupVerifyStatus                        string
	VerificationFailures                      int
	IssueCount                                int
	ControlFiles                              int
	IssuePayloadFiles                         int
	ProvenanceFiles                           int
	ReadableFiles                             int
	UnreadableFiles                           int
	GitAvailable                              bool
	GitCurrentBranch                          string
	GitHistoryAvailable                       bool
	GitTrackedFiles                           int
	UntrackedFiles                            int
	WorkingTreeDirtyFiles                     int
	FilesWithCommits                          int
	FilesWithoutCommits                       int
	RawBackupBodiesIncluded                   bool
	RawGitSubjectsIncluded                    bool
	AuthorIdentitiesIncluded                  bool
	RepositoryMutationAllowed                 bool
	LLME2ERequiredAfterBackupProvenanceChange bool
	Cards                                     []BackupProvenanceCard
	Findings                                  []BackupProvenanceFinding
}

type BackupProvenanceCard struct {
	Kind              string
	Path              string
	GitPath           string
	IssueNumber       int
	BackupGeneratedAt string
	EventName         string
	Readable          bool
	Bytes             int
	Lines             int
	SHA               string
	GitTracked        bool
	WorkingTreeDirty  bool
	CommitAvailable   bool
	LastCommitSHA     string
	LastCommitShort   string
	LastCommitDate    string
	SubjectSHA        string
}

type BackupProvenanceFinding struct {
	Severity  string
	Code      string
	Path      string
	DetailSHA string
}

func (p BackupProvenance) OK() bool {
	return p.BackupProvenanceStatus == "ok"
}

func BuildBackupProvenance(root, repo string) (BackupProvenance, error) {
	if err := validateRepoName(repo); err != nil {
		return BackupProvenance{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupProvenance{}, err
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupProvenance{}, err
	}

	report := BackupProvenance{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		BackupProvenanceStatus:    "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		IssueCount:                len(index.Issues),
		RawBackupBodiesIncluded:   false,
		RawGitSubjectsIncluded:    false,
		AuthorIdentitiesIncluded:  false,
		RepositoryMutationAllowed: false,
		LLME2ERequiredAfterBackupProvenanceChange: true,
	}
	if !verify.OK() {
		report.BackupVerifyStatus = "warn"
		for _, failure := range verify.VerificationFailures {
			report.addFinding("high", "backup_verification_failure", "", failure)
		}
	}

	gitRoot, err := backupProvenanceGitRoot(repoDir)
	if err != nil {
		report.addFinding("warning", "git_worktree_unavailable", "", err.Error())
	} else {
		report.GitAvailable = true
		if branch, err := backupProvenanceRunGit(gitRoot, "branch", "--show-current"); err == nil {
			report.GitCurrentBranch = strings.TrimSpace(branch)
		}
	}

	card, findings := backupProvenanceCard(repoDir, repoDir, report.GitAvailable, "index", "index.json", 0, "", "")
	report.addCard(card, findings...)
	card, findings = backupProvenanceCard(repoDir, repoDir, report.GitAvailable, "readme", "README.md", 0, "", "")
	report.addCard(card, findings...)
	for _, issue := range index.Issues {
		card, findings = backupProvenanceCard(repoDir, repoDir, report.GitAvailable, "issue-backup", issue.Path, issue.Number, issue.BackupGeneratedAt, issue.EventName)
		report.addCard(card, findings...)
	}

	report.GitHistoryAvailable = report.GitAvailable && report.ProvenanceFiles > 0 && report.FilesWithCommits == report.ProvenanceFiles
	if report.BackupVerifyStatus != "ok" || !report.GitAvailable || !report.GitHistoryAvailable || report.UntrackedFiles > 0 || report.WorkingTreeDirtyFiles > 0 || report.UnreadableFiles > 0 {
		report.BackupProvenanceStatus = "warn"
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		if report.Findings[i].Severity != report.Findings[j].Severity {
			return report.Findings[i].Severity < report.Findings[j].Severity
		}
		if report.Findings[i].Code != report.Findings[j].Code {
			return report.Findings[i].Code < report.Findings[j].Code
		}
		return report.Findings[i].Path < report.Findings[j].Path
	})
	return report, nil
}

func RenderBackupProvenance(report BackupProvenance) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Provenance Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", report.Repo)
	fmt.Fprintf(&b, "- backup_provenance_status: `%s`\n", report.BackupProvenanceStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", report.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", report.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", report.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", report.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", report.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", report.ReadmePath)
	fmt.Fprintf(&b, "- expected_backup_branch: `%s`\n", defaultBackupBranch)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", report.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", report.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", report.IssueCount)
	fmt.Fprintf(&b, "- control_files: `%d`\n", report.ControlFiles)
	fmt.Fprintf(&b, "- issue_payload_files: `%d`\n", report.IssuePayloadFiles)
	fmt.Fprintf(&b, "- provenance_files: `%d`\n", report.ProvenanceFiles)
	fmt.Fprintf(&b, "- readable_files: `%d`\n", report.ReadableFiles)
	fmt.Fprintf(&b, "- unreadable_files: `%d`\n", report.UnreadableFiles)
	fmt.Fprintf(&b, "- git_available: `%t`\n", report.GitAvailable)
	if strings.TrimSpace(report.GitCurrentBranch) != "" {
		fmt.Fprintf(&b, "- git_current_branch: `%s`\n", inlineCode(report.GitCurrentBranch))
	} else {
		b.WriteString("- git_current_branch: `unknown`\n")
	}
	fmt.Fprintf(&b, "- git_history_available: `%t`\n", report.GitHistoryAvailable)
	fmt.Fprintf(&b, "- git_tracked_files: `%d`\n", report.GitTrackedFiles)
	fmt.Fprintf(&b, "- untracked_files: `%d`\n", report.UntrackedFiles)
	fmt.Fprintf(&b, "- working_tree_dirty_files: `%d`\n", report.WorkingTreeDirtyFiles)
	fmt.Fprintf(&b, "- files_with_commits: `%d`\n", report.FilesWithCommits)
	fmt.Fprintf(&b, "- files_without_commits: `%d`\n", report.FilesWithoutCommits)
	fmt.Fprintf(&b, "- raw_backup_bodies_included: `%t`\n", report.RawBackupBodiesIncluded)
	fmt.Fprintf(&b, "- raw_git_subjects_included: `%t`\n", report.RawGitSubjectsIncluded)
	fmt.Fprintf(&b, "- author_identities_included: `%t`\n", report.AuthorIdentitiesIncluded)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_provenance_change: `%t`\n\n", report.LLME2ERequiredAfterBackupProvenanceChange)

	b.WriteString("This report verifies a fetched `gitclaw-backups` tree, then proves the backup index, README, and issue payload files are present in git history. It reports only paths, counts, booleans, timestamps, and hashes; raw issue titles, issue bodies, comment bodies, transcript messages, git commit subjects, and author identities are not included.\n\n")

	b.WriteString("### Provenance Gates\n")
	writeBackupProvenanceGate(&b, "verify_gate", report.BackupVerifyStatus == "ok")
	writeBackupProvenanceGate(&b, "git_history_gate", report.GitHistoryAvailable)
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("\n### Provenance Files\n")
	writeBackupProvenanceCards(&b, report.Cards)
	b.WriteString("\n### Provenance Findings\n")
	writeBackupProvenanceFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func (p *BackupProvenance) addCard(card BackupProvenanceCard, findings ...BackupProvenanceFinding) {
	p.Cards = append(p.Cards, card)
	p.Findings = append(p.Findings, findings...)
	p.ProvenanceFiles++
	if card.Kind == "issue-backup" {
		p.IssuePayloadFiles++
	} else {
		p.ControlFiles++
	}
	if card.Readable {
		p.ReadableFiles++
	} else {
		p.UnreadableFiles++
	}
	if p.GitAvailable {
		if card.GitTracked {
			p.GitTrackedFiles++
		} else {
			p.UntrackedFiles++
			p.addFinding("warning", "file_not_tracked_by_git", card.Path, card.GitPath)
		}
		if card.WorkingTreeDirty {
			p.WorkingTreeDirtyFiles++
			p.addFinding("warning", "file_dirty_in_worktree", card.Path, card.GitPath)
		}
		if card.CommitAvailable {
			p.FilesWithCommits++
		} else {
			p.FilesWithoutCommits++
			p.addFinding("warning", "file_missing_git_commit", card.Path, card.GitPath)
		}
	}
}

func (p *BackupProvenance) addFinding(severity, code, path, detail string) {
	p.Findings = append(p.Findings, BackupProvenanceFinding{
		Severity:  severity,
		Code:      code,
		Path:      filepath.ToSlash(path),
		DetailSHA: shortDocumentHash(detail),
	})
}

func backupProvenanceCard(repoDir, gitCwd string, gitAvailable bool, kind, relPath string, issueNumber int, generatedAt, eventName string) (BackupProvenanceCard, []BackupProvenanceFinding) {
	card := BackupProvenanceCard{
		Kind:              kind,
		Path:              filepath.ToSlash(relPath),
		IssueNumber:       issueNumber,
		BackupGeneratedAt: generatedAt,
		EventName:         eventName,
	}
	var findings []BackupProvenanceFinding
	absPath := filepath.Join(repoDir, filepath.FromSlash(relPath))
	data, err := os.ReadFile(absPath)
	if err != nil {
		findings = append(findings, BackupProvenanceFinding{
			Severity:  "high",
			Code:      "provenance_file_unreadable",
			Path:      card.Path,
			DetailSHA: shortDocumentHash(err.Error()),
		})
	} else {
		card.Readable = true
		card.Bytes = len(data)
		card.Lines = backupProvenanceLineCount(data)
		card.SHA = shortDocumentHash(string(data))
	}
	if gitAvailable {
		card.fillGitProvenance(gitCwd)
	}
	return card, findings
}

func (c *BackupProvenanceCard) fillGitProvenance(gitCwd string) {
	c.GitPath = c.Path
	if _, err := backupProvenanceRunGit(gitCwd, "ls-files", "--error-unmatch", "--", c.GitPath); err == nil {
		c.GitTracked = true
	}
	if status, err := backupProvenanceRunGit(gitCwd, "status", "--porcelain", "--", c.GitPath); err == nil && strings.TrimSpace(status) != "" && c.GitTracked {
		c.WorkingTreeDirty = true
	}
	log, err := backupProvenanceRunGit(gitCwd, "log", "-1", "--date=iso-strict", "--format=%H%x00%h%x00%aI%x00%s", "--", c.GitPath)
	if err != nil {
		return
	}
	log = strings.TrimRight(log, "\r\n")
	if strings.TrimSpace(log) == "" {
		return
	}
	parts := strings.SplitN(log, "\x00", 4)
	if len(parts) < 4 {
		return
	}
	c.CommitAvailable = true
	c.LastCommitSHA = shortDocumentHash(parts[0])
	c.LastCommitShort = parts[1]
	c.LastCommitDate = parts[2]
	c.SubjectSHA = shortDocumentHash(parts[3])
}

func backupProvenanceGitRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(string(out)))
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("git rev-parse returned an empty worktree root")
	}
	return filepath.Clean(root), nil
}

func backupProvenanceRunGit(root string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", root}, args...)
	out, err := exec.Command("git", fullArgs...).CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return string(out), nil
}

func backupProvenanceLineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		lines++
	}
	return lines
}

func writeBackupProvenanceGate(b *strings.Builder, name string, pass bool) {
	if pass {
		fmt.Fprintf(b, "- %s=`pass`\n", name)
		return
	}
	fmt.Fprintf(b, "- %s=`warn`\n", name)
}

func writeBackupProvenanceCards(b *strings.Builder, cards []BackupProvenanceCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		eventName := strings.TrimSpace(card.EventName)
		if eventName == "" {
			eventName = "none"
		}
		generatedAt := strings.TrimSpace(card.BackupGeneratedAt)
		if generatedAt == "" {
			generatedAt = "none"
		}
		issue := "none"
		if card.IssueNumber > 0 {
			issue = fmt.Sprintf("#%d", card.IssueNumber)
		}
		fmt.Fprintf(
			b,
			"- kind=`%s` issue=%s path=`%s` git_path=`%s` readable=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` backup_generated_at=`%s` event=`%s` git_tracked=`%t` working_tree_dirty=`%t` commit_available=`%t` last_commit_sha256_12=`%s` last_commit_short=`%s` last_commit_date=`%s` subject_sha256_12=`%s`\n",
			card.Kind,
			issue,
			card.Path,
			card.GitPath,
			card.Readable,
			card.Bytes,
			card.Lines,
			card.SHA,
			generatedAt,
			eventName,
			card.GitTracked,
			card.WorkingTreeDirty,
			card.CommitAvailable,
			card.LastCommitSHA,
			card.LastCommitShort,
			card.LastCommitDate,
			card.SubjectSHA,
		)
	}
}

func writeBackupProvenanceFindings(b *strings.Builder, findings []BackupProvenanceFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		path := strings.TrimSpace(finding.Path)
		if path == "" {
			path = "global"
		}
		fmt.Fprintf(
			b,
			"- severity=`%s` code=`%s` path=`%s` detail_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			path,
			finding.DetailSHA,
		)
	}
}
