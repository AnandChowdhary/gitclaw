package gitclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IssueBackup struct {
	Version     int                  `json:"version"`
	GeneratedAt string               `json:"generated_at"`
	Repo        string               `json:"repo"`
	EventName   string               `json:"event_name"`
	Issue       IssueBackupIssue     `json:"issue"`
	Transcript  []TranscriptMessage  `json:"transcript"`
	Comments    []IssueBackupComment `json:"comments"`
}

type IssueBackupIssue struct {
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	Body              string   `json:"body"`
	Author            string   `json:"author"`
	AuthorAssociation string   `json:"author_association"`
	Labels            []string `json:"labels"`
}

type IssueBackupComment struct {
	ID                int64  `json:"id"`
	Body              string `json:"body"`
	Author            string `json:"author"`
	AuthorAssociation string `json:"author_association"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

type BackupIndex struct {
	Version     int                `json:"version"`
	GeneratedAt string             `json:"generated_at"`
	Repo        string             `json:"repo"`
	Count       int                `json:"count"`
	Issues      []BackupIndexIssue `json:"issues"`
}

type BackupIndexIssue struct {
	Number             int      `json:"number"`
	Title              string   `json:"title"`
	Path               string   `json:"path"`
	BackupGeneratedAt  string   `json:"backup_generated_at"`
	EventName          string   `json:"event_name"`
	Labels             []string `json:"labels,omitempty"`
	CommentCount       int      `json:"comment_count"`
	TranscriptMessages int      `json:"transcript_messages"`
}

type BackupVerifyResult struct {
	Root                 string
	Repo                 string
	RepoDir              string
	IndexPath            string
	ReadmePath           string
	IssuesChecked        int
	CommentsChecked      int
	TranscriptMessages   int
	UnindexedIssueFiles  int
	VerificationFailures []string
}

type BackupJSONLRecord struct {
	Schema            string `json:"schema"`
	Repo              string `json:"repo"`
	IssueNumber       int    `json:"issue_number"`
	IssueTitle        string `json:"issue_title"`
	BackupGeneratedAt string `json:"backup_generated_at"`
	EventName         string `json:"event_name"`
	Sequence          int    `json:"sequence"`
	Source            string `json:"source"`
	Role              string `json:"role"`
	Actor             string `json:"actor"`
	AuthorAssociation string `json:"author_association"`
	CommentID         int64  `json:"comment_id,omitempty"`
	Edited            bool   `json:"edited"`
	Trusted           bool   `json:"trusted"`
	BodySHA           string `json:"body_sha256_12"`
	Body              string `json:"body"`
}

type BackupRestorePlan struct {
	Root                      string
	Repo                      string
	TargetRepo                string
	IssuePath                 string
	IssueNumber               int
	BackupGeneratedAt         string
	EventName                 string
	SchemaVersion             int
	Labels                    []string
	Comments                  int
	TranscriptMessages        int
	AssistantTurns            int
	ErrorComments             int
	UserMessages              int
	AssistantMessages         int
	IssueTitleSHA             string
	IssueBodySHA              string
	CommentBodySHAs           []string
	TranscriptMessageSHAs     []string
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
}

type BackupManifest struct {
	Root                      string
	Repo                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	IssueFilter               int
	IssuePayloads             []BackupManifestPayload
	ControlFiles              []BackupManifestFile
	TotalPayloadBytes         int
	TotalComments             int
	TotalTranscript           int
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
}

type BackupManifestFile struct {
	Path  string
	Bytes int
	SHA   string
}

type BackupManifestPayload struct {
	BackupManifestFile
	IssueNumber        int
	BackupGeneratedAt  string
	EventName          string
	SchemaVersion      int
	Comments           int
	TranscriptMessages int
}

type BackupStats struct {
	Root                      string
	Repo                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	BackupStatsStatus         string
	BackupVerifyStatus        string
	VerificationFailures      int
	IssueCount                int
	CommentCount              int
	TranscriptMessages        int
	UserMessages              int
	AssistantMessages         int
	AssistantTurns            int
	ErrorComments             int
	TotalPayloadBytes         int
	EventCounts               []BackupStatsEvent
	LatestIssueNumber         int
	LatestIssuePath           string
	LatestGeneratedAt         string
	LatestEventName           string
	LatestIssueTitleSHA       string
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
}

type BackupStatsEvent struct {
	Name  string
	Count int
}

type BackupList struct {
	Root                      string
	Repo                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	BackupListStatus          string
	BackupVerifyStatus        string
	VerificationFailures      int
	IssueCount                int
	Limit                     int
	BackupsReturned           int
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
	Issues                    []BackupListIssue
}

type BackupListIssue struct {
	IssueNumber        int
	Path               string
	BackupGeneratedAt  string
	EventName          string
	Labels             int
	Comments           int
	TranscriptMessages int
	IssueTitleSHA      string
}

type BackupTimeline struct {
	Root                      string
	Repo                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	BackupTimelineStatus      string
	BackupVerifyStatus        string
	VerificationFailures      int
	IssueCount                int
	Limit                     int
	TimelinePoints            int
	TimelineOrder             string
	TimelineWindow            string
	FirstIssueNumber          int
	FirstGeneratedAt          string
	LatestIssueNumber         int
	LatestGeneratedAt         string
	TotalSpanSeconds          int64
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
	Points                    []BackupTimelinePoint
}

type BackupTimelinePoint struct {
	IssueNumber             int
	Path                    string
	BackupGeneratedAt       string
	EventName               string
	GapSecondsSincePrevious int64
	PayloadBytes            int
	PayloadSHA              string
	Comments                int
	TranscriptMessages      int
	UserMessages            int
	AssistantMessages       int
	AssistantTurns          int
	ErrorComments           int
	IssueTitleSHA           string
}

type BackupInfo struct {
	Root                      string
	Repo                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	BackupInfoStatus          string
	BackupVerifyStatus        string
	VerificationFailures      int
	IssueNumber               int
	IssuePath                 string
	BackupGeneratedAt         string
	EventName                 string
	PayloadBytes              int
	PayloadSHA                string
	Labels                    []string
	Comments                  int
	TranscriptMessages        int
	UserMessages              int
	AssistantMessages         int
	AssistantTurns            int
	ErrorComments             int
	IssueTitleSHA             string
	IssueBodySHA              string
	CommentBodySHAs           []string
	TranscriptBodySHAs        []string
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
}

type BackupCoverage struct {
	Root                       string
	Repo                       string
	RepoDir                    string
	IndexPath                  string
	ReadmePath                 string
	SchemaVersion              int
	IndexGeneratedAt           string
	BackupCoverageStatus       string
	BackupVerifyStatus         string
	VerificationFailures       int
	IssueNumber                int
	IssueIndexed               bool
	IssueBackupPathExpected    string
	IssuePath                  string
	IssueBackupPathCanonical   bool
	IssueBackupPayloadReadable bool
	IssueBackupPayloadBytes    int
	IssueBackupPayloadSHA      string
	BackupGeneratedAt          string
	EventName                  string
	Labels                     int
	Comments                   int
	TranscriptMessages         int
	UserMessages               int
	AssistantMessages          int
	AssistantTurns             int
	ErrorComments              int
	IssueTitleSHA              string
	IssueBodySHA               string
	RawBodiesIncluded          bool
	LLME2ERequiredAfterChange  bool
}

type BackupDrill struct {
	Root                       string
	Repo                       string
	TargetRepo                 string
	RepoDir                    string
	IndexPath                  string
	ReadmePath                 string
	SchemaVersion              int
	IndexGeneratedAt           string
	BackupDrillStatus          string
	BackupVerifyStatus         string
	BackupCoverageStatus       string
	RestorePlanStatus          string
	VerificationFailures       int
	IssueNumber                int
	IssueIndexed               bool
	IssueBackupPathExpected    string
	IssuePath                  string
	IssueBackupPathCanonical   bool
	IssueBackupPayloadReadable bool
	IssueBackupPayloadBytes    int
	IssueBackupPayloadSHA      string
	BackupGeneratedAt          string
	EventName                  string
	Labels                     int
	Comments                   int
	TranscriptMessages         int
	UserMessages               int
	AssistantMessages          int
	AssistantTurns             int
	ErrorComments              int
	IssueTitleSHA              string
	IssueBodySHA               string
	CommentBodySHAs            []string
	TranscriptMessageSHAs      []string
	RestorePlanAvailable       bool
	RawBodiesIncluded          bool
	LLME2ERequiredAfterChange  bool
}

type BackupRetentionPlan struct {
	Root                      string
	Repo                      string
	RepoDir                   string
	IndexPath                 string
	ReadmePath                string
	SchemaVersion             int
	IndexGeneratedAt          string
	KeepLatest                int
	IssueCount                int
	KeepCount                 int
	PruneCandidateCount       int
	RetentionPlanStatus       string
	BackupVerifyStatus        string
	VerificationFailures      int
	OldestKeptIssueNumber     int
	OldestKeptGeneratedAt     string
	NewestKeptIssueNumber     int
	NewestKeptGeneratedAt     string
	RawBodiesIncluded         bool
	LLME2ERequiredAfterChange bool
	Kept                      []BackupRetentionIssue
	PruneCandidates           []BackupRetentionIssue
}

type BackupRetentionIssue struct {
	IssueNumber        int
	Path               string
	BackupGeneratedAt  string
	EventName          string
	Comments           int
	TranscriptMessages int
	IssueTitleSHA      string
}

func (c BackupCoverage) OK() bool {
	return c.BackupCoverageStatus == "ok"
}

func (d BackupDrill) OK() bool {
	return d.BackupDrillStatus == "ok"
}

func (r BackupVerifyResult) OK() bool {
	return len(r.VerificationFailures) == 0
}

func BackupIssue(ctx context.Context, ev Event, github GitHubClient, outDir string, generatedAt time.Time) (string, error) {
	if err := validateRepoName(ev.Repo); err != nil {
		return "", err
	}
	if outDir == "" {
		outDir = filepath.Join(".gitclaw", "backups")
	}
	comments, err := github.ListIssueComments(ctx, ev.Repo, ev.Issue.Number)
	if err != nil {
		return "", fmt.Errorf("list issue comments: %w", err)
	}
	backup := IssueBackup{
		Version:     1,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Repo:        ev.Repo,
		EventName:   ev.EventName,
		Issue: IssueBackupIssue{
			Number:            ev.Issue.Number,
			Title:             ev.Issue.Title,
			Body:              ev.Issue.Body,
			Author:            ev.Issue.User.Login,
			AuthorAssociation: ev.Issue.AuthorAssociation,
			Labels:            append([]string(nil), ev.Issue.Labels...),
		},
		Transcript: BuildTranscript(ev, comments),
		Comments:   backupComments(comments),
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	path := issueBackupPath(outDir, ev.Repo, ev.Issue.Number)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func VerifyBackupTree(root, repo string) (BackupVerifyResult, error) {
	if err := validateRepoName(repo); err != nil {
		return BackupVerifyResult{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	result := BackupVerifyResult{
		Root:       filepath.ToSlash(root),
		Repo:       repo,
		RepoDir:    filepath.ToSlash(repoDir),
		IndexPath:  filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath: filepath.ToSlash(filepath.Join(repoDir, "README.md")),
	}

	indexData, err := os.ReadFile(filepath.Join(repoDir, "index.json"))
	if err != nil {
		result.addFailure("index_json_readable", err.Error())
		return result, nil
	}
	var index BackupIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		result.addFailure("index_json_valid", err.Error())
		return result, nil
	}
	if index.Version != 1 {
		result.addFailure("index_version", fmt.Sprintf("got %d, want 1", index.Version))
	}
	if index.Repo != repo {
		result.addFailure("index_repo", fmt.Sprintf("got %q, want %q", index.Repo, repo))
	}
	if index.Count != len(index.Issues) {
		result.addFailure("index_count", fmt.Sprintf("got %d, want %d", index.Count, len(index.Issues)))
	}
	if _, err := time.Parse(time.RFC3339, index.GeneratedAt); err != nil {
		result.addFailure("index_generated_at", "not RFC3339")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "README.md")); err != nil {
		result.addFailure("readme_present", err.Error())
	}

	seenNumbers := map[int]bool{}
	seenPaths := map[string]bool{}
	lastNumber := -1
	for _, issue := range index.Issues {
		result.verifyIndexIssue(repoDir, repo, issue, seenNumbers, seenPaths, &lastNumber)
	}
	result.verifyNoUnindexedIssues(repoDir, seenPaths)
	return result, nil
}

func WriteBackupIndex(root, repo string, generatedAt time.Time) (string, error) {
	if err := validateRepoName(repo); err != nil {
		return "", err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	matches, err := filepath.Glob(filepath.Join(repoDir, "issues", "*.json"))
	if err != nil {
		return "", err
	}
	issues := make([]BackupIndexIssue, 0, len(matches))
	for _, match := range matches {
		issue, err := readBackupIndexIssue(repoDir, match)
		if err != nil {
			continue
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	index := BackupIndex{
		Version:     1,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Repo:        repo,
		Count:       len(issues),
		Issues:      issues,
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return "", err
	}
	indexPath := filepath.Join(repoDir, "index.json")
	if err := os.WriteFile(indexPath, data, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte(RenderBackupIndexReadme(index)), 0o600); err != nil {
		return "", err
	}
	return indexPath, nil
}

func ExportBackupJSONL(root, repo string, issueNumber int) (string, error) {
	if err := validateRepoName(repo); err != nil {
		return "", err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	matched := 0
	for _, issue := range index.Issues {
		if issueNumber > 0 && issue.Number != issueNumber {
			continue
		}
		backup, err := readIndexedBackup(repoDir, repo, issue)
		if err != nil {
			return "", err
		}
		matched++
		for i, msg := range backup.Transcript {
			line, err := json.Marshal(backupJSONLRecord(backup, msg, i+1))
			if err != nil {
				return "", err
			}
			b.Write(line)
			b.WriteByte('\n')
		}
	}
	if issueNumber > 0 && matched == 0 {
		return "", fmt.Errorf("issue #%d not found in backup index", issueNumber)
	}
	return b.String(), nil
}

func PlanBackupRestore(root, repo string, issueNumber int, targetRepo string) (BackupRestorePlan, error) {
	if issueNumber <= 0 {
		return BackupRestorePlan{}, fmt.Errorf("missing positive issue number")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupRestorePlan{}, err
	}
	if targetRepo == "" {
		targetRepo = repo
	}
	if err := validateRepoName(targetRepo); err != nil {
		return BackupRestorePlan{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupRestorePlan{}, err
	}
	for _, issue := range index.Issues {
		if issue.Number != issueNumber {
			continue
		}
		backup, err := readIndexedBackup(repoDir, repo, issue)
		if err != nil {
			return BackupRestorePlan{}, err
		}
		return buildBackupRestorePlan(root, repo, targetRepo, issue, backup), nil
	}
	return BackupRestorePlan{}, fmt.Errorf("issue #%d not found in backup index", issueNumber)
}

func BuildBackupManifest(root, repo string, issueNumber int) (BackupManifest, error) {
	if issueNumber < 0 {
		return BackupManifest{}, fmt.Errorf("invalid issue number %d", issueNumber)
	}
	if err := validateRepoName(repo); err != nil {
		return BackupManifest{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupManifest{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	manifest := BackupManifest{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		IssueFilter:               issueNumber,
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}

	for _, path := range []string{filepath.Join(repoDir, "index.json"), filepath.Join(repoDir, "README.md")} {
		file, err := manifestFile(repoDir, path)
		if err != nil {
			return BackupManifest{}, err
		}
		manifest.ControlFiles = append(manifest.ControlFiles, file)
	}

	matched := 0
	for _, issue := range index.Issues {
		if issueNumber > 0 && issue.Number != issueNumber {
			continue
		}
		backup, payload, err := manifestPayload(repoDir, repo, issue)
		if err != nil {
			return BackupManifest{}, err
		}
		matched++
		manifest.IssuePayloads = append(manifest.IssuePayloads, payload)
		manifest.TotalPayloadBytes += payload.Bytes
		manifest.TotalComments += len(backup.Comments)
		manifest.TotalTranscript += len(backup.Transcript)
	}
	if issueNumber > 0 && matched == 0 {
		return BackupManifest{}, fmt.Errorf("issue #%d not found in backup index", issueNumber)
	}
	return manifest, nil
}

func BuildBackupStats(root, repo string) (BackupStats, error) {
	if err := validateRepoName(repo); err != nil {
		return BackupStats{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupStats{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupStats{}, err
	}
	stats := BackupStats{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		BackupStatsStatus:         "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		IssueCount:                len(index.Issues),
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	if !verify.OK() {
		stats.BackupStatsStatus = "warn"
		stats.BackupVerifyStatus = "warn"
	}
	eventCounts := map[string]int{}
	var latest time.Time
	for _, issue := range index.Issues {
		backup, payload, err := manifestPayload(repoDir, repo, issue)
		if err != nil {
			return BackupStats{}, err
		}
		stats.TotalPayloadBytes += payload.Bytes
		stats.CommentCount += len(backup.Comments)
		stats.TranscriptMessages += len(backup.Transcript)
		eventName := strings.TrimSpace(backup.EventName)
		if eventName == "" {
			eventName = "unknown"
		}
		eventCounts[eventName]++
		for _, comment := range backup.Comments {
			if HasGitClawMarker(comment.Body) {
				stats.AssistantTurns++
			}
			if HasGitClawErrorMarker(comment.Body) {
				stats.ErrorComments++
			}
		}
		for _, msg := range backup.Transcript {
			switch msg.Role {
			case "assistant":
				stats.AssistantMessages++
			default:
				stats.UserMessages++
			}
		}
		generatedAt, err := time.Parse(time.RFC3339, backup.GeneratedAt)
		if err == nil && (stats.LatestIssueNumber == 0 || generatedAt.After(latest)) {
			latest = generatedAt
			stats.LatestIssueNumber = backup.Issue.Number
			stats.LatestIssuePath = issue.Path
			stats.LatestGeneratedAt = backup.GeneratedAt
			stats.LatestEventName = eventName
			stats.LatestIssueTitleSHA = shortDocumentHash(backup.Issue.Title)
		}
	}
	for name, count := range eventCounts {
		stats.EventCounts = append(stats.EventCounts, BackupStatsEvent{Name: name, Count: count})
	}
	sort.Slice(stats.EventCounts, func(i, j int) bool { return stats.EventCounts[i].Name < stats.EventCounts[j].Name })
	return stats, nil
}

func BuildBackupList(root, repo string, limit int) (BackupList, error) {
	if limit <= 0 {
		return BackupList{}, fmt.Errorf("limit must be positive")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupList{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupList{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupList{}, err
	}
	list := BackupList{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		BackupListStatus:          "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		IssueCount:                len(index.Issues),
		Limit:                     limit,
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	if !verify.OK() {
		list.BackupListStatus = "warn"
		list.BackupVerifyStatus = "warn"
	}

	type backupListCandidate struct {
		issue       BackupListIssue
		generatedAt time.Time
	}
	candidates := make([]backupListCandidate, 0, len(index.Issues))
	for _, issue := range index.Issues {
		generatedAt, err := time.Parse(time.RFC3339, issue.BackupGeneratedAt)
		if err != nil {
			return BackupList{}, fmt.Errorf("parse backup timestamp for issue #%d: %w", issue.Number, err)
		}
		candidates = append(candidates, backupListCandidate{
			generatedAt: generatedAt,
			issue: BackupListIssue{
				IssueNumber:        issue.Number,
				Path:               filepath.ToSlash(issue.Path),
				BackupGeneratedAt:  issue.BackupGeneratedAt,
				EventName:          issue.EventName,
				Labels:             len(issue.Labels),
				Comments:           issue.CommentCount,
				TranscriptMessages: issue.TranscriptMessages,
				IssueTitleSHA:      shortDocumentHash(issue.Title),
			},
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].generatedAt.Equal(candidates[j].generatedAt) {
			return candidates[i].generatedAt.After(candidates[j].generatedAt)
		}
		return candidates[i].issue.IssueNumber > candidates[j].issue.IssueNumber
	})
	for i, candidate := range candidates {
		if i >= limit {
			break
		}
		list.Issues = append(list.Issues, candidate.issue)
	}
	list.BackupsReturned = len(list.Issues)
	return list, nil
}

func BuildBackupTimeline(root, repo string, limit int) (BackupTimeline, error) {
	if limit <= 0 {
		return BackupTimeline{}, fmt.Errorf("limit must be positive")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupTimeline{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupTimeline{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupTimeline{}, err
	}
	timeline := BackupTimeline{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		BackupTimelineStatus:      "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		IssueCount:                len(index.Issues),
		Limit:                     limit,
		TimelineOrder:             "chronological",
		TimelineWindow:            "latest_by_backup_generated_at",
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	if !verify.OK() {
		timeline.BackupTimelineStatus = "warn"
		timeline.BackupVerifyStatus = "warn"
	}

	type timelineCandidate struct {
		point       BackupTimelinePoint
		generatedAt time.Time
	}
	candidates := make([]timelineCandidate, 0, len(index.Issues))
	for _, issue := range index.Issues {
		backup, payload, err := manifestPayload(repoDir, repo, issue)
		if err != nil {
			return BackupTimeline{}, err
		}
		generatedAt, err := time.Parse(time.RFC3339, backup.GeneratedAt)
		if err != nil {
			return BackupTimeline{}, fmt.Errorf("parse backup timestamp for issue #%d: %w", backup.Issue.Number, err)
		}
		point := BackupTimelinePoint{
			IssueNumber:        backup.Issue.Number,
			Path:               filepath.ToSlash(issue.Path),
			BackupGeneratedAt:  backup.GeneratedAt,
			EventName:          backup.EventName,
			PayloadBytes:       payload.Bytes,
			PayloadSHA:         payload.SHA,
			Comments:           len(backup.Comments),
			TranscriptMessages: len(backup.Transcript),
			IssueTitleSHA:      shortDocumentHash(backup.Issue.Title),
		}
		for _, comment := range backup.Comments {
			if HasGitClawMarker(comment.Body) {
				point.AssistantTurns++
			}
			if HasGitClawErrorMarker(comment.Body) {
				point.ErrorComments++
			}
		}
		for _, msg := range backup.Transcript {
			switch msg.Role {
			case "assistant":
				point.AssistantMessages++
			default:
				point.UserMessages++
			}
		}
		candidates = append(candidates, timelineCandidate{
			point:       point,
			generatedAt: generatedAt,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].generatedAt.Equal(candidates[j].generatedAt) {
			return candidates[i].generatedAt.After(candidates[j].generatedAt)
		}
		return candidates[i].point.IssueNumber > candidates[j].point.IssueNumber
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].generatedAt.Equal(candidates[j].generatedAt) {
			return candidates[i].generatedAt.Before(candidates[j].generatedAt)
		}
		return candidates[i].point.IssueNumber < candidates[j].point.IssueNumber
	})
	for i, candidate := range candidates {
		if i > 0 {
			candidate.point.GapSecondsSincePrevious = int64(candidate.generatedAt.Sub(candidates[i-1].generatedAt).Seconds())
		}
		timeline.Points = append(timeline.Points, candidate.point)
	}
	timeline.TimelinePoints = len(timeline.Points)
	if len(candidates) > 0 {
		first := candidates[0]
		latest := candidates[len(candidates)-1]
		timeline.FirstIssueNumber = first.point.IssueNumber
		timeline.FirstGeneratedAt = first.point.BackupGeneratedAt
		timeline.LatestIssueNumber = latest.point.IssueNumber
		timeline.LatestGeneratedAt = latest.point.BackupGeneratedAt
		timeline.TotalSpanSeconds = int64(latest.generatedAt.Sub(first.generatedAt).Seconds())
	}
	return timeline, nil
}

func BuildBackupInfo(root, repo string, issueNumber int) (BackupInfo, error) {
	if issueNumber <= 0 {
		return BackupInfo{}, fmt.Errorf("missing positive issue number")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupInfo{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupInfo{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupInfo{}, err
	}
	info := BackupInfo{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		BackupInfoStatus:          "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		IssueNumber:               issueNumber,
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	if !verify.OK() {
		info.BackupInfoStatus = "warn"
		info.BackupVerifyStatus = "warn"
	}

	for _, issue := range index.Issues {
		if issue.Number != issueNumber {
			continue
		}
		backup, payload, err := manifestPayload(repoDir, repo, issue)
		if err != nil {
			return BackupInfo{}, err
		}
		info.IssuePath = filepath.ToSlash(issue.Path)
		info.BackupGeneratedAt = backup.GeneratedAt
		info.EventName = backup.EventName
		info.PayloadBytes = payload.Bytes
		info.PayloadSHA = payload.SHA
		info.Labels = append([]string(nil), backup.Issue.Labels...)
		sort.Strings(info.Labels)
		info.Comments = len(backup.Comments)
		info.TranscriptMessages = len(backup.Transcript)
		info.IssueTitleSHA = shortDocumentHash(backup.Issue.Title)
		info.IssueBodySHA = shortDocumentHash(backup.Issue.Body)
		info.CommentBodySHAs = make([]string, 0, len(backup.Comments))
		for _, comment := range backup.Comments {
			info.CommentBodySHAs = append(info.CommentBodySHAs, shortDocumentHash(comment.Body))
			if HasGitClawMarker(comment.Body) {
				info.AssistantTurns++
			}
			if HasGitClawErrorMarker(comment.Body) {
				info.ErrorComments++
			}
		}
		info.TranscriptBodySHAs = make([]string, 0, len(backup.Transcript))
		for _, msg := range backup.Transcript {
			info.TranscriptBodySHAs = append(info.TranscriptBodySHAs, shortDocumentHash(msg.Body))
			switch msg.Role {
			case "assistant":
				info.AssistantMessages++
			default:
				info.UserMessages++
			}
		}
		return info, nil
	}
	return BackupInfo{}, fmt.Errorf("issue #%d not found in backup index", issueNumber)
}

func BuildBackupCoverage(root, repo string, issueNumber int) (BackupCoverage, error) {
	if issueNumber <= 0 {
		return BackupCoverage{}, fmt.Errorf("missing positive issue number")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupCoverage{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupCoverage{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupCoverage{}, err
	}
	expectedPath := filepath.ToSlash(filepath.Join("issues", fmt.Sprintf("%06d.json", issueNumber)))
	coverage := BackupCoverage{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		BackupCoverageStatus:      "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		IssueNumber:               issueNumber,
		IssueBackupPathExpected:   expectedPath,
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	if !verify.OK() {
		coverage.BackupCoverageStatus = "warn"
		coverage.BackupVerifyStatus = "warn"
	}

	for _, issue := range index.Issues {
		if issue.Number != issueNumber {
			continue
		}
		coverage.IssueIndexed = true
		coverage.IssuePath = filepath.ToSlash(issue.Path)
		coverage.IssueBackupPathCanonical = coverage.IssuePath == expectedPath
		if !coverage.IssueBackupPathCanonical {
			coverage.BackupCoverageStatus = "warn"
		}
		backup, payload, err := manifestPayload(repoDir, repo, issue)
		if err != nil {
			coverage.BackupCoverageStatus = "warn"
			return coverage, nil
		}
		coverage.IssueBackupPayloadReadable = true
		coverage.IssueBackupPayloadBytes = payload.Bytes
		coverage.IssueBackupPayloadSHA = payload.SHA
		coverage.BackupGeneratedAt = backup.GeneratedAt
		coverage.EventName = backup.EventName
		coverage.Labels = len(backup.Issue.Labels)
		coverage.Comments = len(backup.Comments)
		coverage.TranscriptMessages = len(backup.Transcript)
		coverage.IssueTitleSHA = shortDocumentHash(backup.Issue.Title)
		coverage.IssueBodySHA = shortDocumentHash(backup.Issue.Body)
		for _, comment := range backup.Comments {
			if HasGitClawMarker(comment.Body) {
				coverage.AssistantTurns++
			}
			if HasGitClawErrorMarker(comment.Body) {
				coverage.ErrorComments++
			}
		}
		for _, msg := range backup.Transcript {
			switch msg.Role {
			case "assistant":
				coverage.AssistantMessages++
			default:
				coverage.UserMessages++
			}
		}
		return coverage, nil
	}
	coverage.BackupCoverageStatus = "missing"
	return coverage, nil
}

func BuildBackupDrill(root, repo string, issueNumber int, targetRepo string) (BackupDrill, error) {
	if issueNumber <= 0 {
		return BackupDrill{}, fmt.Errorf("missing positive issue number")
	}
	if strings.TrimSpace(targetRepo) == "" {
		targetRepo = repo
	}
	if err := validateRepoName(targetRepo); err != nil {
		return BackupDrill{}, err
	}
	coverage, err := BuildBackupCoverage(root, repo, issueNumber)
	if err != nil {
		return BackupDrill{}, err
	}
	drill := BackupDrill{
		Root:                       coverage.Root,
		Repo:                       coverage.Repo,
		TargetRepo:                 targetRepo,
		RepoDir:                    coverage.RepoDir,
		IndexPath:                  coverage.IndexPath,
		ReadmePath:                 coverage.ReadmePath,
		SchemaVersion:              coverage.SchemaVersion,
		IndexGeneratedAt:           coverage.IndexGeneratedAt,
		BackupDrillStatus:          "ok",
		BackupVerifyStatus:         coverage.BackupVerifyStatus,
		BackupCoverageStatus:       coverage.BackupCoverageStatus,
		RestorePlanStatus:          "ok",
		VerificationFailures:       coverage.VerificationFailures,
		IssueNumber:                coverage.IssueNumber,
		IssueIndexed:               coverage.IssueIndexed,
		IssueBackupPathExpected:    coverage.IssueBackupPathExpected,
		IssuePath:                  coverage.IssuePath,
		IssueBackupPathCanonical:   coverage.IssueBackupPathCanonical,
		IssueBackupPayloadReadable: coverage.IssueBackupPayloadReadable,
		IssueBackupPayloadBytes:    coverage.IssueBackupPayloadBytes,
		IssueBackupPayloadSHA:      coverage.IssueBackupPayloadSHA,
		BackupGeneratedAt:          coverage.BackupGeneratedAt,
		EventName:                  coverage.EventName,
		Labels:                     coverage.Labels,
		Comments:                   coverage.Comments,
		TranscriptMessages:         coverage.TranscriptMessages,
		UserMessages:               coverage.UserMessages,
		AssistantMessages:          coverage.AssistantMessages,
		AssistantTurns:             coverage.AssistantTurns,
		ErrorComments:              coverage.ErrorComments,
		IssueTitleSHA:              coverage.IssueTitleSHA,
		IssueBodySHA:               coverage.IssueBodySHA,
		RawBodiesIncluded:          false,
		LLME2ERequiredAfterChange:  true,
	}
	if !coverage.OK() {
		drill.BackupDrillStatus = coverage.BackupCoverageStatus
		drill.RestorePlanStatus = "skipped"
		return drill, nil
	}
	plan, err := PlanBackupRestore(root, repo, issueNumber, targetRepo)
	if err != nil {
		return BackupDrill{}, err
	}
	drill.RestorePlanAvailable = true
	drill.CommentBodySHAs = append([]string(nil), plan.CommentBodySHAs...)
	drill.TranscriptMessageSHAs = append([]string(nil), plan.TranscriptMessageSHAs...)
	if plan.Comments != drill.Comments || plan.TranscriptMessages != drill.TranscriptMessages || plan.AssistantTurns != drill.AssistantTurns || plan.ErrorComments != drill.ErrorComments {
		drill.BackupDrillStatus = "warn"
		drill.RestorePlanStatus = "warn"
	}
	return drill, nil
}

func BuildBackupRetentionPlan(root, repo string, keepLatest int) (BackupRetentionPlan, error) {
	if keepLatest <= 0 {
		return BackupRetentionPlan{}, fmt.Errorf("keep latest must be positive")
	}
	if err := validateRepoName(repo); err != nil {
		return BackupRetentionPlan{}, err
	}
	repoDir, index, err := readBackupIndex(root, repo)
	if err != nil {
		return BackupRetentionPlan{}, err
	}
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	verify, err := VerifyBackupTree(root, repo)
	if err != nil {
		return BackupRetentionPlan{}, err
	}
	plan := BackupRetentionPlan{
		Root:                      filepath.ToSlash(root),
		Repo:                      repo,
		RepoDir:                   filepath.ToSlash(repoDir),
		IndexPath:                 filepath.ToSlash(filepath.Join(repoDir, "index.json")),
		ReadmePath:                filepath.ToSlash(filepath.Join(repoDir, "README.md")),
		SchemaVersion:             index.Version,
		IndexGeneratedAt:          index.GeneratedAt,
		KeepLatest:                keepLatest,
		IssueCount:                len(index.Issues),
		RetentionPlanStatus:       "ok",
		BackupVerifyStatus:        "ok",
		VerificationFailures:      len(verify.VerificationFailures),
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	if !verify.OK() {
		plan.RetentionPlanStatus = "warn"
		plan.BackupVerifyStatus = "warn"
	}

	type retentionCandidate struct {
		issue       BackupRetentionIssue
		generatedAt time.Time
	}
	candidates := make([]retentionCandidate, 0, len(index.Issues))
	for _, issue := range index.Issues {
		backup, err := readIndexedBackup(repoDir, repo, issue)
		if err != nil {
			return BackupRetentionPlan{}, err
		}
		generatedAt, err := time.Parse(time.RFC3339, backup.GeneratedAt)
		if err != nil {
			return BackupRetentionPlan{}, fmt.Errorf("parse backup timestamp for issue #%d: %w", backup.Issue.Number, err)
		}
		candidates = append(candidates, retentionCandidate{
			generatedAt: generatedAt,
			issue: BackupRetentionIssue{
				IssueNumber:        backup.Issue.Number,
				Path:               filepath.ToSlash(issue.Path),
				BackupGeneratedAt:  backup.GeneratedAt,
				EventName:          backup.EventName,
				Comments:           len(backup.Comments),
				TranscriptMessages: len(backup.Transcript),
				IssueTitleSHA:      shortDocumentHash(backup.Issue.Title),
			},
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].generatedAt.Equal(candidates[j].generatedAt) {
			return candidates[i].generatedAt.After(candidates[j].generatedAt)
		}
		return candidates[i].issue.IssueNumber > candidates[j].issue.IssueNumber
	})
	for i, candidate := range candidates {
		if i < keepLatest {
			plan.Kept = append(plan.Kept, candidate.issue)
		} else {
			plan.PruneCandidates = append(plan.PruneCandidates, candidate.issue)
		}
	}
	plan.KeepCount = len(plan.Kept)
	plan.PruneCandidateCount = len(plan.PruneCandidates)
	if len(plan.Kept) > 0 {
		newest := plan.Kept[0]
		oldest := plan.Kept[len(plan.Kept)-1]
		plan.NewestKeptIssueNumber = newest.IssueNumber
		plan.NewestKeptGeneratedAt = newest.BackupGeneratedAt
		plan.OldestKeptIssueNumber = oldest.IssueNumber
		plan.OldestKeptGeneratedAt = oldest.BackupGeneratedAt
	}
	return plan, nil
}

func RenderBackupRestorePlan(plan BackupRestorePlan) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Restore Plan\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run")
	fmt.Fprintf(&b, "- source_repository: `%s`\n", plan.Repo)
	fmt.Fprintf(&b, "- target_repository: `%s`\n", plan.TargetRepo)
	fmt.Fprintf(&b, "- issue: `#%d`\n", plan.IssueNumber)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", filepath.ToSlash(plan.Root))
	fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", plan.IssuePath)
	fmt.Fprintf(&b, "- backup_generated_at: `%s`\n", plan.BackupGeneratedAt)
	fmt.Fprintf(&b, "- backup_event_name: `%s`\n", plan.EventName)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", plan.SchemaVersion)
	fmt.Fprintf(&b, "- labels: `%d`\n", len(plan.Labels))
	fmt.Fprintf(&b, "- comments: `%d`\n", plan.Comments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", plan.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", plan.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", plan.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", plan.AssistantTurns)
	fmt.Fprintf(&b, "- error_comments: `%d`\n", plan.ErrorComments)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", plan.IssueTitleSHA)
	fmt.Fprintf(&b, "- issue_body_sha256_12: `%s`\n", plan.IssueBodySHA)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", plan.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_restore_plan_change: `%t`\n\n", plan.LLME2ERequiredAfterChange)
	b.WriteString("This is a non-mutating recovery plan. It reads the local backup tree only; it does not create issues, post comments, apply labels, or call GitHub APIs.\n\n")
	b.WriteString("### Planned Restore Actions\n")
	b.WriteString("- create one new issue in the target repository using the backed-up title and body\n")
	b.WriteString("- apply backed-up labels that still exist or can be created by a future approved restore command\n")
	b.WriteString("- replay backed-up comments in backup order as the restoring actor, preserving original author/comment metadata in comment headers\n")
	b.WriteString("- preserve assistant-turn and error marker bodies from the raw backup if a future approved restore command is allowed to post them\n")
	b.WriteString("- verify the new issue transcript against the hashes below before considering restore complete\n\n")
	b.WriteString("### Labels\n")
	if len(plan.Labels) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, label := range plan.Labels {
			fmt.Fprintf(&b, "- `%s`\n", inlineCode(label))
		}
	}
	b.WriteString("\n### Comment Body Hashes\n")
	writeHashList(&b, "comment", plan.CommentBodySHAs)
	b.WriteString("\n### Transcript Body Hashes\n")
	writeHashList(&b, "message", plan.TranscriptMessageSHAs)
	return strings.TrimSpace(b.String())
}

func RenderBackupRetentionPlan(plan BackupRetentionPlan) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Retention Plan\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", plan.Repo)
	fmt.Fprintf(&b, "- retention_mode: `%s`\n", "dry-run")
	fmt.Fprintf(&b, "- backup_retention_status: `%s`\n", plan.RetentionPlanStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", plan.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", plan.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", plan.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", plan.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", plan.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", plan.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", plan.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", plan.IndexGeneratedAt)
	fmt.Fprintf(&b, "- keep_latest: `%d`\n", plan.KeepLatest)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", plan.IssueCount)
	fmt.Fprintf(&b, "- keep_count: `%d`\n", plan.KeepCount)
	fmt.Fprintf(&b, "- prune_candidate_count: `%d`\n", plan.PruneCandidateCount)
	if plan.NewestKeptIssueNumber > 0 {
		fmt.Fprintf(&b, "- newest_kept_issue: `#%d`\n", plan.NewestKeptIssueNumber)
		fmt.Fprintf(&b, "- newest_kept_generated_at: `%s`\n", plan.NewestKeptGeneratedAt)
		fmt.Fprintf(&b, "- oldest_kept_issue: `#%d`\n", plan.OldestKeptIssueNumber)
		fmt.Fprintf(&b, "- oldest_kept_generated_at: `%s`\n", plan.OldestKeptGeneratedAt)
	} else {
		b.WriteString("- newest_kept_issue: `none`\n")
		b.WriteString("- oldest_kept_issue: `none`\n")
	}
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", plan.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_retention_plan_change: `%t`\n\n", plan.LLME2ERequiredAfterChange)

	b.WriteString("This is a non-mutating retention plan. It reads the local backup tree only; it does not delete files, delete branches, edit issues, post comments, or call GitHub APIs.\n\n")
	b.WriteString("Issue and comment bodies are not included. Titles are represented by short hashes so retention can be audited without exposing transcript contents.\n\n")

	b.WriteString("### Kept Backups\n")
	writeRetentionIssueList(&b, plan.Kept)
	b.WriteString("\n### Prune Candidates\n")
	writeRetentionIssueList(&b, plan.PruneCandidates)
	return strings.TrimSpace(b.String())
}

func RenderBackupManifest(manifest BackupManifest) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Manifest\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", manifest.Repo)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", manifest.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", manifest.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", manifest.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", manifest.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", manifest.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", manifest.IndexGeneratedAt)
	if manifest.IssueFilter > 0 {
		fmt.Fprintf(&b, "- issue_filter: `#%d`\n", manifest.IssueFilter)
	} else {
		b.WriteString("- issue_filter: `all`\n")
	}
	fmt.Fprintf(&b, "- control_files: `%d`\n", len(manifest.ControlFiles))
	fmt.Fprintf(&b, "- issue_payload_files: `%d`\n", len(manifest.IssuePayloads))
	fmt.Fprintf(&b, "- total_payload_bytes: `%d`\n", manifest.TotalPayloadBytes)
	fmt.Fprintf(&b, "- total_comments: `%d`\n", manifest.TotalComments)
	fmt.Fprintf(&b, "- total_transcript_messages: `%d`\n", manifest.TotalTranscript)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", manifest.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_manifest_change: `%t`\n\n", manifest.LLME2ERequiredAfterChange)
	b.WriteString("This manifest reads a fetched backup tree and reports file-level provenance, counts, and hashes. It does not print raw issue, comment, or transcript bodies.\n\n")

	b.WriteString("### Control Files\n")
	for _, file := range manifest.ControlFiles {
		fmt.Fprintf(&b, "- `%s` bytes=`%d` sha256_12=`%s`\n", file.Path, file.Bytes, file.SHA)
	}

	b.WriteString("\n### Issue Payloads\n")
	if len(manifest.IssuePayloads) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, payload := range manifest.IssuePayloads {
			fmt.Fprintf(
				&b,
				"- issue=`#%d` path=`%s` bytes=`%d` sha256_12=`%s` schema=`%d` event=`%s` generated_at=`%s` comments=`%d` transcript_messages=`%d`\n",
				payload.IssueNumber,
				payload.Path,
				payload.Bytes,
				payload.SHA,
				payload.SchemaVersion,
				payload.EventName,
				payload.BackupGeneratedAt,
				payload.Comments,
				payload.TranscriptMessages,
			)
		}
	}
	return strings.TrimSpace(b.String())
}

func RenderBackupStats(stats BackupStats) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Stats Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", stats.Repo)
	fmt.Fprintf(&b, "- backup_stats_status: `%s`\n", stats.BackupStatsStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", stats.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", stats.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", stats.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", stats.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", stats.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", stats.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", stats.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", stats.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", stats.IssueCount)
	fmt.Fprintf(&b, "- comment_count: `%d`\n", stats.CommentCount)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", stats.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", stats.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", stats.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", stats.AssistantTurns)
	fmt.Fprintf(&b, "- error_comments: `%d`\n", stats.ErrorComments)
	fmt.Fprintf(&b, "- event_types: `%d`\n", len(stats.EventCounts))
	fmt.Fprintf(&b, "- total_payload_bytes: `%d`\n", stats.TotalPayloadBytes)
	if stats.LatestIssueNumber > 0 {
		fmt.Fprintf(&b, "- latest_issue: `#%d`\n", stats.LatestIssueNumber)
		fmt.Fprintf(&b, "- latest_issue_path: `%s`\n", stats.LatestIssuePath)
		fmt.Fprintf(&b, "- latest_backup_generated_at: `%s`\n", stats.LatestGeneratedAt)
		fmt.Fprintf(&b, "- latest_event_name: `%s`\n", stats.LatestEventName)
		fmt.Fprintf(&b, "- latest_issue_title_sha256_12: `%s`\n", stats.LatestIssueTitleSHA)
	} else {
		b.WriteString("- latest_issue: `none`\n")
	}
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", stats.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_stats_change: `%t`\n\n", stats.LLME2ERequiredAfterChange)
	b.WriteString("This report summarizes a fetched backup branch index and payload metadata. It does not print raw issue, comment, or transcript bodies.\n\n")

	b.WriteString("### Event Types\n")
	if len(stats.EventCounts) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, event := range stats.EventCounts {
			fmt.Fprintf(&b, "- `%s`: `%d`\n", event.Name, event.Count)
		}
	}
	return strings.TrimSpace(b.String())
}

func RenderBackupList(list BackupList) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup List Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", list.Repo)
	fmt.Fprintf(&b, "- backup_list_status: `%s`\n", list.BackupListStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", list.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", list.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", list.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", list.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", list.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", list.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", list.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", list.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", list.IssueCount)
	fmt.Fprintf(&b, "- limit: `%d`\n", list.Limit)
	fmt.Fprintf(&b, "- backups_returned: `%d`\n", list.BackupsReturned)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", list.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_list_change: `%t`\n\n", list.LLME2ERequiredAfterChange)
	b.WriteString("This report lists indexed backup conversation metadata from a fetched backup tree. It does not print raw issue titles, issue bodies, comments, or transcript bodies.\n\n")

	b.WriteString("### Indexed Backups\n")
	writeBackupListIssues(&b, list.Issues)
	return strings.TrimSpace(b.String())
}

func RenderBackupTimeline(timeline BackupTimeline) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Timeline Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", timeline.Repo)
	fmt.Fprintf(&b, "- backup_timeline_status: `%s`\n", timeline.BackupTimelineStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", timeline.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", timeline.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", timeline.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", timeline.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", timeline.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", timeline.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", timeline.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", timeline.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue_count: `%d`\n", timeline.IssueCount)
	fmt.Fprintf(&b, "- limit: `%d`\n", timeline.Limit)
	fmt.Fprintf(&b, "- timeline_points: `%d`\n", timeline.TimelinePoints)
	fmt.Fprintf(&b, "- timeline_order: `%s`\n", timeline.TimelineOrder)
	fmt.Fprintf(&b, "- timeline_window: `%s`\n", timeline.TimelineWindow)
	if timeline.TimelinePoints > 0 {
		fmt.Fprintf(&b, "- first_issue: `#%d`\n", timeline.FirstIssueNumber)
		fmt.Fprintf(&b, "- first_generated_at: `%s`\n", timeline.FirstGeneratedAt)
		fmt.Fprintf(&b, "- latest_issue: `#%d`\n", timeline.LatestIssueNumber)
		fmt.Fprintf(&b, "- latest_generated_at: `%s`\n", timeline.LatestGeneratedAt)
		fmt.Fprintf(&b, "- total_span_seconds: `%d`\n", timeline.TotalSpanSeconds)
	} else {
		b.WriteString("- first_issue: `none`\n")
		b.WriteString("- latest_issue: `none`\n")
		b.WriteString("- total_span_seconds: `0`\n")
	}
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", timeline.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_timeline_change: `%t`\n\n", timeline.LLME2ERequiredAfterChange)

	b.WriteString("This report turns the fetched `gitclaw-backups` branch into a body-free conversation backup timeline. It reports ordering, gaps, counts, hashes, and assistant-turn markers only; raw issue titles, issue bodies, comment bodies, transcript messages, prompts, search queries, and tool outputs are not included.\n\n")
	b.WriteString("### Timeline Points\n")
	writeBackupTimelinePoints(&b, timeline.Points)
	return strings.TrimSpace(b.String())
}

func RenderBackupInfo(info BackupInfo) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", info.Repo)
	fmt.Fprintf(&b, "- backup_info_status: `%s`\n", info.BackupInfoStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", info.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", info.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", info.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", info.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", info.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", info.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", info.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", info.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue: `#%d`\n", info.IssueNumber)
	fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", info.IssuePath)
	fmt.Fprintf(&b, "- backup_generated_at: `%s`\n", info.BackupGeneratedAt)
	fmt.Fprintf(&b, "- backup_event_name: `%s`\n", info.EventName)
	fmt.Fprintf(&b, "- payload_bytes: `%d`\n", info.PayloadBytes)
	fmt.Fprintf(&b, "- payload_sha256_12: `%s`\n", info.PayloadSHA)
	fmt.Fprintf(&b, "- labels: `%d`\n", len(info.Labels))
	fmt.Fprintf(&b, "- comments: `%d`\n", info.Comments)
	fmt.Fprintf(&b, "- transcript_messages: `%d`\n", info.TranscriptMessages)
	fmt.Fprintf(&b, "- user_messages: `%d`\n", info.UserMessages)
	fmt.Fprintf(&b, "- assistant_messages: `%d`\n", info.AssistantMessages)
	fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", info.AssistantTurns)
	fmt.Fprintf(&b, "- error_comments: `%d`\n", info.ErrorComments)
	fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", info.IssueTitleSHA)
	fmt.Fprintf(&b, "- issue_body_sha256_12: `%s`\n", info.IssueBodySHA)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", info.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_info_change: `%t`\n\n", info.LLME2ERequiredAfterChange)
	b.WriteString("This report inspects one fetched backup payload from the local backup tree. It does not print raw issue titles, issue bodies, comments, transcript messages, prompts, or restored content.\n\n")

	b.WriteString("### Labels\n")
	if len(info.Labels) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, label := range info.Labels {
			fmt.Fprintf(&b, "- `%s`\n", inlineCode(label))
		}
	}
	b.WriteString("\n### Comment Body Hashes\n")
	writeHashList(&b, "comment", info.CommentBodySHAs)
	b.WriteString("\n### Transcript Body Hashes\n")
	writeHashList(&b, "message", info.TranscriptBodySHAs)
	return strings.TrimSpace(b.String())
}

func RenderBackupCoverage(coverage BackupCoverage) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Coverage Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", coverage.Repo)
	fmt.Fprintf(&b, "- backup_coverage_status: `%s`\n", coverage.BackupCoverageStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", coverage.BackupVerifyStatus)
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", coverage.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", coverage.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", coverage.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", coverage.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", coverage.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", coverage.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", coverage.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue: `#%d`\n", coverage.IssueNumber)
	fmt.Fprintf(&b, "- issue_indexed: `%t`\n", coverage.IssueIndexed)
	fmt.Fprintf(&b, "- expected_issue_backup_path: `%s`\n", coverage.IssueBackupPathExpected)
	if coverage.IssuePath != "" {
		fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", coverage.IssuePath)
	} else {
		b.WriteString("- issue_backup_path: `none`\n")
	}
	fmt.Fprintf(&b, "- issue_backup_path_canonical: `%t`\n", coverage.IssueBackupPathCanonical)
	fmt.Fprintf(&b, "- issue_backup_payload_readable: `%t`\n", coverage.IssueBackupPayloadReadable)
	if coverage.IssueBackupPayloadReadable {
		fmt.Fprintf(&b, "- payload_bytes: `%d`\n", coverage.IssueBackupPayloadBytes)
		fmt.Fprintf(&b, "- payload_sha256_12: `%s`\n", coverage.IssueBackupPayloadSHA)
		fmt.Fprintf(&b, "- backup_generated_at: `%s`\n", coverage.BackupGeneratedAt)
		fmt.Fprintf(&b, "- backup_event_name: `%s`\n", coverage.EventName)
		fmt.Fprintf(&b, "- labels: `%d`\n", coverage.Labels)
		fmt.Fprintf(&b, "- comments: `%d`\n", coverage.Comments)
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", coverage.TranscriptMessages)
		fmt.Fprintf(&b, "- user_messages: `%d`\n", coverage.UserMessages)
		fmt.Fprintf(&b, "- assistant_messages: `%d`\n", coverage.AssistantMessages)
		fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", coverage.AssistantTurns)
		fmt.Fprintf(&b, "- error_comments: `%d`\n", coverage.ErrorComments)
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", coverage.IssueTitleSHA)
		fmt.Fprintf(&b, "- issue_body_sha256_12: `%s`\n", coverage.IssueBodySHA)
	}
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", coverage.RawBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_coverage_change: `%t`\n\n", coverage.LLME2ERequiredAfterChange)

	b.WriteString("This report verifies whether one requested issue has a canonical, readable entry in a fetched `gitclaw-backups` tree. It reports paths, counts, timestamps, and hashes only; raw issue titles, issue bodies, comments, transcript messages, prompts, and restored content are not included.\n\n")
	b.WriteString("### Coverage Evidence\n")
	if !coverage.IssueIndexed {
		b.WriteString("- index_entry=`missing`\n")
		b.WriteString("- payload_read=`skipped`\n")
	} else {
		b.WriteString("- index_entry=`present`\n")
		fmt.Fprintf(&b, "- canonical_path=`%t`\n", coverage.IssueBackupPathCanonical)
		fmt.Fprintf(&b, "- payload_read=`%t`\n", coverage.IssueBackupPayloadReadable)
	}
	b.WriteString("- mutation_performed=`false`\n")
	b.WriteString("- github_api_calls_performed=`false`\n")
	return strings.TrimSpace(b.String())
}

func RenderBackupDrill(drill BackupDrill) string {
	var b strings.Builder
	b.WriteString("## GitClaw Backup Drill Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	fmt.Fprintf(&b, "- repository: `%s`\n", drill.Repo)
	fmt.Fprintf(&b, "- target_repository: `%s`\n", drill.TargetRepo)
	fmt.Fprintf(&b, "- backup_drill_status: `%s`\n", drill.BackupDrillStatus)
	fmt.Fprintf(&b, "- backup_verify_status: `%s`\n", drill.BackupVerifyStatus)
	fmt.Fprintf(&b, "- backup_coverage_status: `%s`\n", drill.BackupCoverageStatus)
	fmt.Fprintf(&b, "- restore_plan_status: `%s`\n", drill.RestorePlanStatus)
	fmt.Fprintf(&b, "- restore_mode: `%s`\n", "dry-run")
	fmt.Fprintf(&b, "- verification_failures: `%d`\n", drill.VerificationFailures)
	fmt.Fprintf(&b, "- backup_root: `%s`\n", drill.Root)
	fmt.Fprintf(&b, "- repo_backup_dir: `%s`\n", drill.RepoDir)
	fmt.Fprintf(&b, "- index_path: `%s`\n", drill.IndexPath)
	fmt.Fprintf(&b, "- readme_path: `%s`\n", drill.ReadmePath)
	fmt.Fprintf(&b, "- backup_schema_version: `%d`\n", drill.SchemaVersion)
	fmt.Fprintf(&b, "- index_generated_at: `%s`\n", drill.IndexGeneratedAt)
	fmt.Fprintf(&b, "- issue: `#%d`\n", drill.IssueNumber)
	fmt.Fprintf(&b, "- issue_indexed: `%t`\n", drill.IssueIndexed)
	fmt.Fprintf(&b, "- expected_issue_backup_path: `%s`\n", drill.IssueBackupPathExpected)
	if drill.IssuePath != "" {
		fmt.Fprintf(&b, "- issue_backup_path: `%s`\n", drill.IssuePath)
	} else {
		b.WriteString("- issue_backup_path: `none`\n")
	}
	fmt.Fprintf(&b, "- issue_backup_path_canonical: `%t`\n", drill.IssueBackupPathCanonical)
	fmt.Fprintf(&b, "- issue_backup_payload_readable: `%t`\n", drill.IssueBackupPayloadReadable)
	if drill.IssueBackupPayloadReadable {
		fmt.Fprintf(&b, "- payload_bytes: `%d`\n", drill.IssueBackupPayloadBytes)
		fmt.Fprintf(&b, "- payload_sha256_12: `%s`\n", drill.IssueBackupPayloadSHA)
		fmt.Fprintf(&b, "- backup_generated_at: `%s`\n", drill.BackupGeneratedAt)
		fmt.Fprintf(&b, "- backup_event_name: `%s`\n", drill.EventName)
		fmt.Fprintf(&b, "- labels: `%d`\n", drill.Labels)
		fmt.Fprintf(&b, "- comments: `%d`\n", drill.Comments)
		fmt.Fprintf(&b, "- transcript_messages: `%d`\n", drill.TranscriptMessages)
		fmt.Fprintf(&b, "- user_messages: `%d`\n", drill.UserMessages)
		fmt.Fprintf(&b, "- assistant_messages: `%d`\n", drill.AssistantMessages)
		fmt.Fprintf(&b, "- assistant_turn_comments: `%d`\n", drill.AssistantTurns)
		fmt.Fprintf(&b, "- error_comments: `%d`\n", drill.ErrorComments)
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", drill.IssueTitleSHA)
		fmt.Fprintf(&b, "- issue_body_sha256_12: `%s`\n", drill.IssueBodySHA)
	}
	fmt.Fprintf(&b, "- restore_plan_available: `%t`\n", drill.RestorePlanAvailable)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", drill.RawBodiesIncluded)
	fmt.Fprintf(&b, "- mutation_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- github_api_calls_performed: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_backup_drill_change: `%t`\n\n", drill.LLME2ERequiredAfterChange)

	b.WriteString("This restore-readiness drill composes backup verification, single-issue coverage, and a dry-run restore plan against a fetched `gitclaw-backups` tree. It reports only paths, counts, timestamps, gate statuses, and hashes; raw issue titles, issue bodies, comments, transcript messages, prompts, and restored content are not included.\n\n")
	b.WriteString("### Drill Gates\n")
	writeBackupDrillGate(&b, "verify_gate", drill.BackupVerifyStatus)
	writeBackupDrillGate(&b, "coverage_gate", drill.BackupCoverageStatus)
	writeBackupDrillGate(&b, "restore_plan_gate", drill.RestorePlanStatus)
	b.WriteString("- backup_branch_fetch_required=`true`\n")
	b.WriteString("- issue_side_intent_supported=`true`\n")
	b.WriteString("\n### Comment Body Hashes\n")
	writeHashList(&b, "comment", drill.CommentBodySHAs)
	b.WriteString("\n### Transcript Body Hashes\n")
	writeHashList(&b, "message", drill.TranscriptMessageSHAs)
	return strings.TrimSpace(b.String())
}

func writeBackupListIssues(b *strings.Builder, issues []BackupListIssue) {
	if len(issues) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, issue := range issues {
		eventName := strings.TrimSpace(issue.EventName)
		if eventName == "" {
			eventName = "unknown"
		}
		fmt.Fprintf(
			b,
			"- issue=#%d path=`%s` generated_at=`%s` event=`%s` labels=`%d` comments=`%d` transcript_messages=`%d` title_sha256_12=`%s`\n",
			issue.IssueNumber,
			issue.Path,
			issue.BackupGeneratedAt,
			eventName,
			issue.Labels,
			issue.Comments,
			issue.TranscriptMessages,
			issue.IssueTitleSHA,
		)
	}
}

func writeBackupTimelinePoints(b *strings.Builder, points []BackupTimelinePoint) {
	if len(points) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, point := range points {
		eventName := strings.TrimSpace(point.EventName)
		if eventName == "" {
			eventName = "unknown"
		}
		fmt.Fprintf(
			b,
			"- issue=#%d path=`%s` generated_at=`%s` event=`%s` gap_seconds_since_previous=`%d` payload_bytes=`%d` payload_sha256_12=`%s` comments=`%d` transcript_messages=`%d` user_messages=`%d` assistant_messages=`%d` assistant_turn_comments=`%d` error_comments=`%d` title_sha256_12=`%s`\n",
			point.IssueNumber,
			point.Path,
			point.BackupGeneratedAt,
			eventName,
			point.GapSecondsSincePrevious,
			point.PayloadBytes,
			point.PayloadSHA,
			point.Comments,
			point.TranscriptMessages,
			point.UserMessages,
			point.AssistantMessages,
			point.AssistantTurns,
			point.ErrorComments,
			point.IssueTitleSHA,
		)
	}
}

func writeRetentionIssueList(b *strings.Builder, issues []BackupRetentionIssue) {
	if len(issues) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, issue := range issues {
		eventName := strings.TrimSpace(issue.EventName)
		if eventName == "" {
			eventName = "unknown"
		}
		fmt.Fprintf(
			b,
			"- issue=#%d path=`%s` generated_at=`%s` event=`%s` comments=`%d` transcript_messages=`%d` title_sha256_12=`%s`\n",
			issue.IssueNumber,
			issue.Path,
			issue.BackupGeneratedAt,
			eventName,
			issue.Comments,
			issue.TranscriptMessages,
			issue.IssueTitleSHA,
		)
	}
}

func readBackupIndex(root, repo string) (string, BackupIndex, error) {
	if root == "" {
		root = filepath.Join(".gitclaw", "backups")
	}
	repoDir := backupRepoDir(root, repo)
	indexData, err := os.ReadFile(filepath.Join(repoDir, "index.json"))
	if err != nil {
		return "", BackupIndex{}, fmt.Errorf("read backup index: %w", err)
	}
	var index BackupIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return "", BackupIndex{}, fmt.Errorf("parse backup index: %w", err)
	}
	if index.Repo != repo {
		return "", BackupIndex{}, fmt.Errorf("backup index repo %q does not match %q", index.Repo, repo)
	}
	return repoDir, index, nil
}

func manifestPayload(repoDir, repo string, issue BackupIndexIssue) (IssueBackup, BackupManifestPayload, error) {
	absPath, err := safeBackupPayloadPath(repoDir, issue.Path)
	if err != nil {
		return IssueBackup{}, BackupManifestPayload{}, err
	}
	file, err := manifestFile(repoDir, absPath)
	if err != nil {
		return IssueBackup{}, BackupManifestPayload{}, err
	}
	backup, err := readIndexedBackup(repoDir, repo, issue)
	if err != nil {
		return IssueBackup{}, BackupManifestPayload{}, err
	}
	return backup, BackupManifestPayload{
		BackupManifestFile: file,
		IssueNumber:        backup.Issue.Number,
		BackupGeneratedAt:  backup.GeneratedAt,
		EventName:          backup.EventName,
		SchemaVersion:      backup.Version,
		Comments:           len(backup.Comments),
		TranscriptMessages: len(backup.Transcript),
	}, nil
}

func manifestFile(repoDir, absPath string) (BackupManifestFile, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return BackupManifestFile{}, err
	}
	rel, err := filepath.Rel(repoDir, absPath)
	if err != nil {
		return BackupManifestFile{}, err
	}
	return BackupManifestFile{
		Path:  filepath.ToSlash(rel),
		Bytes: len(data),
		SHA:   shortDocumentHash(string(data)),
	}, nil
}

func readIndexedBackup(repoDir, repo string, issue BackupIndexIssue) (IssueBackup, error) {
	absPath, err := safeBackupPayloadPath(repoDir, issue.Path)
	if err != nil {
		return IssueBackup{}, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return IssueBackup{}, fmt.Errorf("read issue backup %s: %w", issue.Path, err)
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return IssueBackup{}, fmt.Errorf("parse issue backup %s: %w", issue.Path, err)
	}
	if backup.Repo != repo {
		return IssueBackup{}, fmt.Errorf("issue backup %s repo %q does not match %q", issue.Path, backup.Repo, repo)
	}
	if backup.Issue.Number != issue.Number {
		return IssueBackup{}, fmt.Errorf("issue backup %s number %d does not match index number %d", issue.Path, backup.Issue.Number, issue.Number)
	}
	return backup, nil
}

func buildBackupRestorePlan(root, repo, targetRepo string, issue BackupIndexIssue, backup IssueBackup) BackupRestorePlan {
	plan := BackupRestorePlan{
		Root:                      root,
		Repo:                      repo,
		TargetRepo:                targetRepo,
		IssuePath:                 issue.Path,
		IssueNumber:               backup.Issue.Number,
		BackupGeneratedAt:         backup.GeneratedAt,
		EventName:                 backup.EventName,
		SchemaVersion:             backup.Version,
		Labels:                    append([]string(nil), backup.Issue.Labels...),
		Comments:                  len(backup.Comments),
		TranscriptMessages:        len(backup.Transcript),
		IssueTitleSHA:             shortDocumentHash(backup.Issue.Title),
		IssueBodySHA:              shortDocumentHash(backup.Issue.Body),
		CommentBodySHAs:           make([]string, 0, len(backup.Comments)),
		RawBodiesIncluded:         false,
		LLME2ERequiredAfterChange: true,
	}
	for _, comment := range backup.Comments {
		plan.CommentBodySHAs = append(plan.CommentBodySHAs, shortDocumentHash(comment.Body))
		if HasGitClawMarker(comment.Body) {
			plan.AssistantTurns++
		}
		if HasGitClawErrorMarker(comment.Body) {
			plan.ErrorComments++
		}
	}
	plan.TranscriptMessageSHAs = make([]string, 0, len(backup.Transcript))
	for _, msg := range backup.Transcript {
		plan.TranscriptMessageSHAs = append(plan.TranscriptMessageSHAs, shortDocumentHash(msg.Body))
		switch msg.Role {
		case "assistant":
			plan.AssistantMessages++
		default:
			plan.UserMessages++
		}
	}
	return plan
}

func writeHashList(b *strings.Builder, prefix string, hashes []string) {
	if len(hashes) == 0 {
		b.WriteString("- none\n")
		return
	}
	for i, hash := range hashes {
		fmt.Fprintf(b, "- %s_%d_sha256_12: `%s`\n", prefix, i+1, hash)
	}
}

func writeBackupDrillGate(b *strings.Builder, name, status string) {
	gate := "warn"
	switch status {
	case "ok":
		gate = "pass"
	case "skipped":
		gate = "skipped"
	case "missing":
		gate = "missing"
	}
	fmt.Fprintf(b, "- %s=`%s`\n", name, gate)
}

func backupJSONLRecord(backup IssueBackup, msg TranscriptMessage, sequence int) BackupJSONLRecord {
	source := "issue"
	if msg.CommentID > 0 {
		source = fmt.Sprintf("comment:%d", msg.CommentID)
	}
	return BackupJSONLRecord{
		Schema:            "gitclaw.backup.transcript.v1",
		Repo:              backup.Repo,
		IssueNumber:       backup.Issue.Number,
		IssueTitle:        backup.Issue.Title,
		BackupGeneratedAt: backup.GeneratedAt,
		EventName:         backup.EventName,
		Sequence:          sequence,
		Source:            source,
		Role:              msg.Role,
		Actor:             msg.Actor,
		AuthorAssociation: msg.AuthorAssociation,
		CommentID:         msg.CommentID,
		Edited:            msg.Edited,
		Trusted:           msg.Trusted,
		BodySHA:           shortDocumentHash(msg.Body),
		Body:              msg.Body,
	}
}

func (r *BackupVerifyResult) verifyIndexIssue(repoDir, repo string, issue BackupIndexIssue, seenNumbers map[int]bool, seenPaths map[string]bool, lastNumber *int) {
	r.IssuesChecked++
	if issue.Number <= 0 {
		r.addFailure("issue_number", fmt.Sprintf("invalid issue number %d", issue.Number))
	}
	if seenNumbers[issue.Number] {
		r.addFailure("issue_unique", fmt.Sprintf("duplicate issue #%d", issue.Number))
	}
	seenNumbers[issue.Number] = true
	if issue.Number < *lastNumber {
		r.addFailure("index_sorted", fmt.Sprintf("issue #%d appears after #%d", issue.Number, *lastNumber))
	}
	*lastNumber = issue.Number

	wantPath := filepath.ToSlash(filepath.Join("issues", fmt.Sprintf("%06d.json", issue.Number)))
	if issue.Path != wantPath {
		r.addFailure("issue_path_canonical", fmt.Sprintf("issue #%d path %q, want %q", issue.Number, issue.Path, wantPath))
	}
	absPath, err := safeBackupPayloadPath(repoDir, issue.Path)
	if err != nil {
		r.addFailure("issue_path_safe", err.Error())
		return
	}
	seenPaths[filepath.ToSlash(issue.Path)] = true
	data, err := os.ReadFile(absPath)
	if err != nil {
		r.addFailure("issue_payload_readable", fmt.Sprintf("%s: %v", issue.Path, err))
		return
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		r.addFailure("issue_payload_json", fmt.Sprintf("%s: %v", issue.Path, err))
		return
	}
	r.CommentsChecked += len(backup.Comments)
	r.TranscriptMessages += len(backup.Transcript)
	if backup.Version != 1 {
		r.addFailure("issue_payload_version", fmt.Sprintf("%s version %d, want 1", issue.Path, backup.Version))
	}
	if backup.Repo != repo {
		r.addFailure("issue_payload_repo", fmt.Sprintf("%s repo %q, want %q", issue.Path, backup.Repo, repo))
	}
	if backup.Issue.Number != issue.Number {
		r.addFailure("issue_payload_number", fmt.Sprintf("%s number %d, want %d", issue.Path, backup.Issue.Number, issue.Number))
	}
	if backup.Issue.Title != issue.Title {
		r.addFailure("issue_payload_title", fmt.Sprintf("issue #%d title hash mismatch", issue.Number))
	}
	if len(backup.Comments) != issue.CommentCount {
		r.addFailure("issue_comment_count", fmt.Sprintf("issue #%d got %d, want %d", issue.Number, len(backup.Comments), issue.CommentCount))
	}
	if len(backup.Transcript) != issue.TranscriptMessages {
		r.addFailure("issue_transcript_count", fmt.Sprintf("issue #%d got %d, want %d", issue.Number, len(backup.Transcript), issue.TranscriptMessages))
	}
	if _, err := time.Parse(time.RFC3339, backup.GeneratedAt); err != nil {
		r.addFailure("issue_generated_at", fmt.Sprintf("issue #%d timestamp is not RFC3339", issue.Number))
	}
}

func (r *BackupVerifyResult) verifyNoUnindexedIssues(repoDir string, seenPaths map[string]bool) {
	matches, err := filepath.Glob(filepath.Join(repoDir, "issues", "*.json"))
	if err != nil {
		r.addFailure("issue_glob", err.Error())
		return
	}
	for _, match := range matches {
		rel, err := filepath.Rel(repoDir, match)
		if err != nil {
			r.addFailure("issue_relpath", err.Error())
			continue
		}
		rel = filepath.ToSlash(rel)
		if !seenPaths[rel] {
			r.UnindexedIssueFiles++
			r.addFailure("issue_indexed", fmt.Sprintf("%s is not listed in index.json", rel))
		}
	}
}

func safeBackupPayloadPath(repoDir, relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("empty backup payload path")
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe backup payload path %q", relPath)
	}
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(filepath.Join(absRepoDir, clean))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRepoDir, absPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe backup payload path %q", relPath)
	}
	return absPath, nil
}

func (r *BackupVerifyResult) addFailure(name, detail string) {
	r.VerificationFailures = append(r.VerificationFailures, fmt.Sprintf("%s: %s", name, detail))
}

func readBackupIndexIssue(repoDir, path string) (BackupIndexIssue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BackupIndexIssue{}, err
	}
	var backup IssueBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return BackupIndexIssue{}, err
	}
	rel, err := filepath.Rel(repoDir, path)
	if err != nil {
		return BackupIndexIssue{}, err
	}
	return BackupIndexIssue{
		Number:             backup.Issue.Number,
		Title:              backup.Issue.Title,
		Path:               filepath.ToSlash(rel),
		BackupGeneratedAt:  backup.GeneratedAt,
		EventName:          backup.EventName,
		Labels:             append([]string(nil), backup.Issue.Labels...),
		CommentCount:       len(backup.Comments),
		TranscriptMessages: len(backup.Transcript),
	}, nil
}

func RenderBackupIndexReadme(index BackupIndex) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GitClaw Backups for `%s`\n\n", index.Repo)
	fmt.Fprintf(&b, "- generated_at: `%s`\n", index.GeneratedAt)
	fmt.Fprintf(&b, "- issue_count: `%d`\n\n", index.Count)
	b.WriteString("| Issue | Title | Generated | Comments | Transcript | Path |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | --- |\n")
	for _, issue := range index.Issues {
		fmt.Fprintf(&b, "| #%d | %s | `%s` | %d | %d | `%s` |\n",
			issue.Number,
			escapeMarkdownTableCell(issue.Title),
			issue.BackupGeneratedAt,
			issue.CommentCount,
			issue.TranscriptMessages,
			issue.Path,
		)
	}
	return b.String()
}

func escapeMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}

func backupComments(comments []Comment) []IssueBackupComment {
	out := make([]IssueBackupComment, 0, len(comments))
	for _, comment := range comments {
		out = append(out, IssueBackupComment{
			ID:                comment.ID,
			Body:              comment.Body,
			Author:            comment.User.Login,
			AuthorAssociation: comment.AuthorAssociation,
			CreatedAt:         comment.CreatedAt,
			UpdatedAt:         comment.UpdatedAt,
		})
	}
	return out
}

func issueBackupPath(outDir, repo string, issueNumber int) string {
	return filepath.Join(backupRepoDir(outDir, repo), "issues", fmt.Sprintf("%06d.json", issueNumber))
}

func backupRepoDir(outDir, repo string) string {
	return filepath.Join(outDir, strings.ReplaceAll(repo, "/", "__"))
}
