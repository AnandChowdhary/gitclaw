package gitclaw

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	maxContextDocumentBytes  = 12000
	maxContextReferenceBytes = 12000
	maxContextFolderEntries  = 120
	maxContextGitCommits     = 10
	maxToolReadBytes         = 8000
	maxRepoFilesListed       = 240
	maxToolFilesRead         = 5
	maxMemoryDocuments       = 3
	maxSearchQueries         = 5
	maxSearchMatches         = 20
	maxSearchMatchesPerQuery = 5
	maxSearchFileBytes       = 64000
	maxSearchLineBytes       = 300
)

var searchIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.:-]{5,}`)

var contextDocumentPaths = []string{
	"AGENTS.md",
	".github/copilot-instructions.md",
	".gitclaw/GITCLAW.md",
	".gitclaw/SOUL.md",
	".gitclaw/IDENTITY.md",
	".gitclaw/USER.md",
	".gitclaw/TOOLS.md",
	".gitclaw/MEMORY.md",
	".gitclaw/HEARTBEAT.md",
	".gitclaw/STANDING_ORDERS.md",
}

func LoadRepoContext(root string, transcript []TranscriptMessage) (RepoContext, error) {
	return LoadRepoContextWithConfig(root, transcript, Config{})
}

func LoadRepoContextWithConfig(root string, transcript []TranscriptMessage, cfg Config) (RepoContext, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return RepoContext{}, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return RepoContext{}, err
	}
	if !info.IsDir() {
		return RepoContext{}, fmt.Errorf("workdir is not a directory: %s", root)
	}

	files, err := listRepoFiles(absRoot)
	if err != nil {
		return RepoContext{}, err
	}
	documents := loadContextDocuments(absRoot, contextDocumentPaths)
	documents = append(documents, loadMemoryDocuments(absRoot)...)
	referenceDocs, referenceSummaries := loadContextReferences(absRoot, transcript)
	documents = append(documents, referenceDocs...)
	skillSummaries, skillBundles, skills := loadSkillContext(absRoot, transcript, cfg)
	ctx := RepoContext{
		Documents:         documents,
		ContextReferences: referenceSummaries,
		Skills:            skills,
		SkillSummaries:    skillSummaries,
		SkillBundles:      skillBundles,
		AllowedTools:      cfg.AllowedTools,
		DisabledTools:     cfg.DisabledTools,
	}
	if toolEnabled, _, _ := toolEnabledByConfig("gitclaw.list_files", cfg); toolEnabled {
		ctx.ToolOutputs = append(ctx.ToolOutputs, ToolOutput{Name: "gitclaw.list_files", Input: ".", Output: strings.Join(files, "\n")})
	}
	if toolEnabled, _, _ := toolEnabledByConfig("gitclaw.skill_index", cfg); toolEnabled && len(skillSummaries) > 0 {
		ctx.ToolOutputs = append(ctx.ToolOutputs, ToolOutput{
			Name:   "gitclaw.skill_index",
			Input:  ".gitclaw/SKILLS",
			Output: renderSkillIndex(skillSummaries),
		})
	}
	searchQueries := searchQueriesFromTranscript(transcript)
	if toolEnabled, _, _ := toolEnabledByConfig("gitclaw.search_files", cfg); toolEnabled && len(searchQueries) > 0 {
		if output := searchRepoFiles(absRoot, files, searchQueries); output != "" {
			ctx.ToolOutputs = append(ctx.ToolOutputs, ToolOutput{
				Name:   "gitclaw.search_files",
				Input:  strings.Join(searchQueries, " | "),
				Output: output,
			})
		}
	}
	if toolEnabled, _, _ := toolEnabledByConfig("gitclaw.read_file", cfg); toolEnabled {
		for _, file := range mentionedRepoFiles(files, transcript) {
			body, err := readRepoTextFile(absRoot, file, maxToolReadBytes)
			if err != nil {
				continue
			}
			ctx.ToolOutputs = append(ctx.ToolOutputs, ToolOutput{
				Name:   "gitclaw.read_file",
				Input:  file,
				Output: body,
			})
		}
	}
	return ctx, nil
}

func loadMemoryDocuments(root string) []ContextDocument {
	matches, _ := filepath.Glob(filepath.Join(root, ".gitclaw", "memory", "*.md"))
	sort.Slice(matches, func(i, j int) bool {
		return filepath.Base(matches[i]) > filepath.Base(matches[j])
	})
	if len(matches) > maxMemoryDocuments {
		matches = matches[:maxMemoryDocuments]
	}
	docs := make([]ContextDocument, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(root, match)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		body, err := readRepoTextFile(root, rel, maxContextDocumentBytes)
		if err != nil {
			continue
		}
		docs = append(docs, ContextDocument{Path: rel, Body: body})
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs
}

func loadContextDocuments(root string, paths []string) []ContextDocument {
	docs := make([]ContextDocument, 0, len(paths))
	for _, path := range paths {
		body, err := readRepoTextFile(root, path, maxContextDocumentBytes)
		if err != nil {
			continue
		}
		docs = append(docs, ContextDocument{Path: path, Body: body})
	}
	return docs
}

type parsedContextReference struct {
	Kind      string
	Path      string
	LineRange string
	Count     int
	StartLine int
	EndLine   int
}

func loadContextReferences(root string, transcript []TranscriptMessage) ([]ContextDocument, []ContextReferenceSummary) {
	tokens := contextReferenceTokens(transcriptText(transcript))
	if len(tokens) == 0 {
		return nil, nil
	}
	docs := make([]ContextDocument, 0, len(tokens))
	summaries := make([]ContextReferenceSummary, 0, len(tokens))
	for _, token := range tokens {
		ref, ok := parseContextReference(token)
		if !ok {
			continue
		}
		switch ref.Kind {
		case "file":
			doc, summary := loadFileContextReference(root, ref)
			if summary.Status == "ok" {
				docs = append(docs, doc)
			}
			summaries = append(summaries, summary)
		case "folder":
			doc, summary := loadFolderContextReference(root, ref)
			if summary.Status == "ok" {
				docs = append(docs, doc)
			}
			summaries = append(summaries, summary)
		case "diff", "staged", "git":
			doc, summary := loadGitContextReference(root, ref)
			if summary.Status == "ok" {
				docs = append(docs, doc)
			}
			summaries = append(summaries, summary)
		}
	}
	return docs, summaries
}

func contextReferenceTokens(text string) []string {
	var tokens []string
	seen := map[string]bool{}
	for _, field := range strings.Fields(text) {
		token := cleanContextReferenceToken(field)
		if token == "" {
			continue
		}
		lower := strings.ToLower(token)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		tokens = append(tokens, token)
	}
	return tokens
}

func cleanContextReferenceToken(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, "`\"'()[]{}<>")
	token = strings.TrimRight(token, ".,;!?")
	if token == "@diff" || token == "@staged" || strings.HasPrefix(token, "@git:") || strings.HasPrefix(token, "@file:") || strings.HasPrefix(token, "@folder:") {
		return token
	}
	return ""
}

var contextLineRangePattern = regexp.MustCompile(`^(.+):([0-9]+)(?:-([0-9]+))?$`)

func parseContextReference(token string) (parsedContextReference, bool) {
	token = cleanContextReferenceToken(token)
	if strings.HasPrefix(token, "@file:") {
		path := strings.TrimSpace(strings.TrimPrefix(token, "@file:"))
		ref := parsedContextReference{Kind: "file"}
		if matches := contextLineRangePattern.FindStringSubmatch(path); len(matches) > 0 {
			start, end := atoiPositive(matches[2]), atoiPositive(matches[3])
			if end == 0 {
				end = start
			}
			if start > 0 && end >= start {
				path = matches[1]
				ref.StartLine = start
				ref.EndLine = end
				if start == end {
					ref.LineRange = fmt.Sprintf("%d", start)
				} else {
					ref.LineRange = fmt.Sprintf("%d-%d", start, end)
				}
			}
		}
		ref.Path = cleanContextLookupPath(path)
		return ref, ref.Path != ""
	}
	if strings.HasPrefix(token, "@folder:") {
		path := cleanContextLookupPath(strings.TrimSpace(strings.TrimPrefix(token, "@folder:")))
		return parsedContextReference{Kind: "folder", Path: path}, path != ""
	}
	if token == "@diff" {
		return parsedContextReference{Kind: "diff", Path: ".", LineRange: "none"}, true
	}
	if token == "@staged" {
		return parsedContextReference{Kind: "staged", Path: ".", LineRange: "none"}, true
	}
	if strings.HasPrefix(token, "@git:") {
		count := atoiPositive(strings.TrimSpace(strings.TrimPrefix(token, "@git:")))
		if count <= 0 {
			return parsedContextReference{}, false
		}
		if count > maxContextGitCommits {
			count = maxContextGitCommits
		}
		return parsedContextReference{Kind: "git", Path: "HEAD", LineRange: "none", Count: count}, true
	}
	return parsedContextReference{}, false
}

func atoiPositive(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func loadFileContextReference(root string, ref parsedContextReference) (ContextDocument, ContextReferenceSummary) {
	summary := ContextReferenceSummary{
		Kind:      "file",
		Path:      ref.Path,
		LineRange: lineRangeOrNone(ref.LineRange),
		Status:    "not_found",
	}
	if contextReferencePathBlocked(ref.Path) {
		summary.Status = "blocked"
		summary.Reason = "sensitive_path"
		return ContextDocument{}, summary
	}
	body, err := readRepoTextFile(root, ref.Path, maxContextReferenceBytes)
	if err != nil {
		summary.Reason = contextReferenceErrorReason(err)
		if summary.Reason == "path_outside_repository" {
			summary.Status = "blocked"
		}
		return ContextDocument{}, summary
	}
	if ref.StartLine > 0 {
		ranged, ok := fileLineRange(body, ref.StartLine, ref.EndLine)
		if ok {
			body = ranged
		}
	}
	docPath := ref.Path
	if ref.LineRange != "" {
		docPath += ":" + ref.LineRange
	}
	summary.Status = "ok"
	summary.Bytes = len(body)
	summary.Lines = lineCount(body)
	summary.SHA = shortDocumentHash(body)
	return ContextDocument{Path: docPath, Body: body}, summary
}

func loadFolderContextReference(root string, ref parsedContextReference) (ContextDocument, ContextReferenceSummary) {
	summary := ContextReferenceSummary{
		Kind:      "folder",
		Path:      ref.Path,
		LineRange: "none",
		Status:    "not_found",
	}
	if contextReferencePathBlocked(ref.Path) {
		summary.Status = "blocked"
		summary.Reason = "sensitive_path"
		return ContextDocument{}, summary
	}
	path, err := safeRepoPath(root, ref.Path)
	if err != nil {
		summary.Status = "blocked"
		summary.Reason = "path_outside_repository"
		return ContextDocument{}, summary
	}
	info, err := os.Lstat(path)
	if err != nil {
		summary.Reason = contextReferenceErrorReason(err)
		return ContextDocument{}, summary
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		summary.Status = "not_folder"
		summary.Reason = "not_directory"
		return ContextDocument{}, summary
	}
	body, entries := renderFolderContextReference(root, ref.Path)
	if body == "" {
		summary.Status = "empty"
		summary.Reason = "no_text_files"
		return ContextDocument{}, summary
	}
	docPath := "@folder:" + ref.Path
	summary.Status = "ok"
	summary.Bytes = len(body)
	summary.Lines = lineCount(body)
	summary.Entries = entries
	summary.SHA = shortDocumentHash(body)
	return ContextDocument{Path: docPath, Body: body}, summary
}

func loadGitContextReference(root string, ref parsedContextReference) (ContextDocument, ContextReferenceSummary) {
	summary := ContextReferenceSummary{
		Kind:      ref.Kind,
		Path:      ref.Path,
		LineRange: lineRangeOrNone(ref.LineRange),
		Count:     ref.Count,
		Status:    "not_found",
	}
	args := gitReferenceArgs(ref)
	if len(args) == 0 {
		summary.Status = "invalid"
		summary.Reason = "unsupported_reference"
		return ContextDocument{}, summary
	}
	body, err := runGitReferenceCommand(root, args)
	if err != nil {
		summary.Status = "error"
		summary.Reason = gitReferenceErrorReason(err)
		return ContextDocument{}, summary
	}
	if strings.TrimSpace(body) == "" {
		summary.Status = "empty"
		summary.Reason = gitReferenceEmptyReason(ref.Kind)
		return ContextDocument{}, summary
	}
	body = truncateContextReferenceText(body, maxContextReferenceBytes)
	docPath := "@" + ref.Kind
	if ref.Kind == "git" {
		docPath = fmt.Sprintf("@git:%d", ref.Count)
	}
	summary.Status = "ok"
	summary.Bytes = len(body)
	summary.Lines = lineCount(body)
	summary.SHA = shortDocumentHash(body)
	return ContextDocument{Path: docPath, Body: body}, summary
}

func gitReferenceArgs(ref parsedContextReference) []string {
	switch ref.Kind {
	case "diff":
		return []string{"diff", "--no-color", "--no-ext-diff", "--"}
	case "staged":
		return []string{"diff", "--cached", "--no-color", "--no-ext-diff", "--"}
	case "git":
		count := ref.Count
		if count <= 0 {
			count = 1
		}
		if count > maxContextGitCommits {
			count = maxContextGitCommits
		}
		return []string{"log", fmt.Sprintf("-n%d", count), "--patch", "--stat", "--no-color", "--no-ext-diff", "--date=iso-strict", "--format=format:commit=%H%nshort=%h%nauthor=%an <%ae>%ndate=%ad%nsubject=%s%n"}
	default:
		return nil
	}
}

func runGitReferenceCommand(root string, args []string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git_not_found: %w", err)
	}
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(data)))
	}
	return strings.TrimSpace(string(data)), nil
}

func truncateContextReferenceText(body string, limit int) string {
	body = strings.TrimSpace(body)
	if limit <= 0 || len(body) <= limit {
		return body
	}
	marker := fmt.Sprintf("\n...[gitclaw:context_reference_truncated omitted_bytes=%d]...\n", len(body)-limit)
	if limit <= len(marker)+20 {
		return body[:limit]
	}
	keep := limit - len(marker)
	head := keep / 2
	tail := keep - head
	return strings.TrimSpace(body[:head] + marker + body[len(body)-tail:])
}

func gitReferenceEmptyReason(kind string) string {
	switch kind {
	case "diff":
		return "no_working_tree_changes"
	case "staged":
		return "no_staged_changes"
	case "git":
		return "no_commits"
	default:
		return "empty_output"
	}
}

func gitReferenceErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "git_not_found"):
		return "git_not_found"
	case strings.Contains(msg, "not a git repository"):
		return "not_git_repository"
	default:
		return "git_command_failed"
	}
}

func renderFolderContextReference(root, rel string) (string, int) {
	dir, err := safeRepoPath(root, rel)
	if err != nil {
		return "", 0
	}
	var b strings.Builder
	entries := 0
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() && shouldSkipDir(name) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if entries >= maxContextFolderEntries {
			return filepath.SkipDir
		}
		if shouldSkipFile(name) {
			return nil
		}
		childRel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		childRel = filepath.ToSlash(childRel)
		if contextReferencePathBlocked(childRel) {
			return nil
		}
		body, err := readRepoTextFile(root, childRel, maxSearchFileBytes)
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "- path=%s bytes=%d lines=%d sha256_12=%s\n", childRel, len(body), lineCount(body), shortDocumentHash(body))
		entries++
		return nil
	})
	if entries >= maxContextFolderEntries {
		b.WriteString("- ...\n")
	}
	return strings.TrimSpace(b.String()), entries
}

func lineRangeOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func fileLineRange(body string, start, end int) (string, bool) {
	if start <= 0 || end < start {
		return "", false
	}
	lines := strings.Split(body, "\n")
	if start > len(lines) {
		return "", false
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.TrimSpace(strings.Join(lines[start-1:end], "\n")), true
}

func contextReferencePathBlocked(rel string) bool {
	clean := strings.ToLower(cleanContextLookupPath(rel))
	clean = strings.Trim(clean, "/")
	if clean == "" {
		return true
	}
	if clean == ".git" || strings.HasPrefix(clean, ".git/") {
		return true
	}
	blockedDirs := []string{".ssh", ".aws", ".gnupg", ".kube", ".gitclaw/skills/.hub"}
	for _, dir := range blockedDirs {
		if clean == dir || strings.HasPrefix(clean, dir+"/") {
			return true
		}
	}
	base := filepath.Base(filepath.FromSlash(clean))
	switch base {
	case ".env", ".env.local", ".envrc", ".netrc", ".pgpass", ".npmrc", ".pypirc", "id_rsa", "id_ed25519", "authorized_keys", "config":
		return true
	default:
		return false
	}
}

func contextReferenceErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "escapes repository"):
		return "path_outside_repository"
	case strings.Contains(msg, "binary file"):
		return "binary_file"
	case strings.Contains(msg, "not a regular file"):
		return "not_regular_file"
	default:
		return "not_found"
	}
}

func loadSkillContext(root string, transcript []TranscriptMessage, cfg Config) ([]SkillSummary, []SkillBundleSummary, []ContextDocument) {
	available := discoverSkills(root)
	bundles := discoverSkillBundles(root)
	if len(available) == 0 && len(bundles) == 0 {
		return nil, nil, nil
	}
	query := strings.ToLower(transcriptText(transcript))
	summaries := make([]SkillSummary, 0, len(available))
	selected := make([]ContextDocument, 0, len(available))
	selectedBundles := selectedSkillBundleSet(bundles, transcript, cfg)
	bundleSelectedSkills := map[string]bool{}
	for _, bundle := range bundles {
		if !selectedBundles[bundle.Name] {
			continue
		}
		for _, skill := range bundle.Skills {
			bundleSelectedSkills[strings.ToLower(strings.TrimSpace(skill))] = true
		}
		if strings.TrimSpace(bundle.Instruction) != "" {
			selected = append(selected, ContextDocument{
				Path: bundle.Path + "#instruction",
				Body: strings.TrimSpace(bundle.Instruction),
			})
		}
	}
	for _, skill := range available {
		enabled, disabledByConfig, blockedByAllowlist := skillEnabledByConfig(skill, cfg)
		summaries = append(summaries, SkillSummary{
			Name:               skill.Name,
			Description:        skill.Description,
			Path:               skill.Path,
			Always:             skill.Always,
			Enabled:            enabled,
			DisabledByConfig:   disabledByConfig,
			BlockedByAllowlist: blockedByAllowlist,
			FrontmatterPresent: skill.FrontmatterPresent,
			Bytes:              len(skill.Body),
			Lines:              lineCount(skill.Body),
			SHA:                shortDocumentHash(skill.Body),
			RequiredEnv:        append([]string(nil), skill.RequiredEnv...),
			RequiredBins:       append([]string(nil), skill.RequiredBins...),
			MissingEnv:         missingEnvVars(skill.RequiredEnv),
			MissingBins:        missingBins(skill.RequiredBins),
		})
		if enabled && (skill.Always || skillMatchesQuery(skill, query) || skillSelectedByBundle(skill, bundleSelectedSkills)) {
			selected = append(selected, ContextDocument{Path: skill.Path, Body: skill.Body})
		}
	}
	bundleSummaries := summarizeSkillBundles(bundles, summaries, selectedBundles)
	return summaries, bundleSummaries, selected
}

func skillEnabledByConfig(skill skillDocument, cfg Config) (enabled bool, disabledByConfig bool, blockedByAllowlist bool) {
	name := strings.ToLower(strings.TrimSpace(skill.Name))
	folder := strings.ToLower(skillFolderName(skill.Path))
	if skillNameInSet(cfg.DisabledSkills, name, folder) {
		return false, true, false
	}
	if len(cfg.AllowedSkills) > 0 && !skillNameInSet(cfg.AllowedSkills, name, folder) {
		return false, false, true
	}
	return true, false, false
}

func skillNameInSet(values map[string]bool, candidates ...string) bool {
	if len(values) == 0 {
		return false
	}
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" && values[candidate] {
			return true
		}
	}
	return false
}

type skillDocument struct {
	Name               string
	Description        string
	Path               string
	Body               string
	Always             bool
	FrontmatterPresent bool
	RequiredEnv        []string
	RequiredBins       []string
}

type skillBundleDocument struct {
	Name        string
	Description string
	Path        string
	Body        string
	Skills      []string
	Instruction string
	ParseError  string
}

func discoverSkills(root string) []skillDocument {
	var skills []skillDocument
	seen := map[string]bool{}
	for _, base := range []string{".gitclaw/SKILLS", ".gitclaw/skills"} {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(base), "*", "SKILL.md"))
		for _, match := range matches {
			realPath, err := filepath.EvalSymlinks(match)
			if err != nil {
				realPath = match
			}
			seenKey := strings.ToLower(realPath)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			rel, err := filepath.Rel(root, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			body, err := readRepoTextFile(root, rel, maxContextDocumentBytes)
			if err != nil {
				continue
			}
			skill := parseSkillDocument(rel, body)
			skills = append(skills, skill)
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Path < skills[j].Path })
	return skills
}

func discoverSkillBundles(root string) []skillBundleDocument {
	var bundles []skillBundleDocument
	seen := map[string]bool{}
	for _, pattern := range []string{".gitclaw/skill-bundles/*.yml", ".gitclaw/skill-bundles/*.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		for _, match := range matches {
			realPath, err := filepath.EvalSymlinks(match)
			if err != nil {
				realPath = match
			}
			seenKey := strings.ToLower(realPath)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			rel, err := filepath.Rel(root, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			body, err := readRepoTextFile(root, rel, maxContextDocumentBytes)
			if err != nil {
				continue
			}
			bundles = append(bundles, parseSkillBundleDocument(rel, body))
		}
	}
	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Path < bundles[j].Path })
	return bundles
}

func parseSkillBundleDocument(path, body string) skillBundleDocument {
	name := skillBundleNameFromPath(path)
	var file struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Skills      []string `yaml:"skills"`
		Instruction string   `yaml:"instruction"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.KnownFields(true)
	parseError := ""
	if err := decoder.Decode(&file); err != nil {
		parseError = err.Error()
	}
	if value := normalizeSkillBundleName(file.Name); value != "" {
		name = value
	}
	return skillBundleDocument{
		Name:        name,
		Description: strings.TrimSpace(file.Description),
		Path:        path,
		Body:        body,
		Skills:      normalizeSkillBundleSkills(file.Skills),
		Instruction: strings.TrimSpace(file.Instruction),
		ParseError:  parseError,
	}
}

func skillBundleNameFromPath(path string) string {
	base := filepath.Base(filepath.FromSlash(path))
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return normalizeSkillBundleName(base)
}

func parseSkillDocument(path, body string) skillDocument {
	name := filepath.Base(filepath.Dir(filepath.FromSlash(path)))
	description := ""
	always := false
	frontmatterPresent := false
	var requiredEnv []string
	var requiredBins []string
	if fm, ok := frontmatter(body); ok {
		frontmatterPresent = true
		if value := frontmatterValue(fm, "name"); value != "" {
			name = value
		}
		if value := frontmatterValue(fm, "description"); value != "" {
			description = value
		}
		if value := frontmatterValue(fm, "skillKey"); value != "" {
			name = value
		}
		always = frontmatterBool(fm, "always") || frontmatterBool(fm, "metadata.openclaw.always")
		requiredEnv = append(requiredEnv, frontmatterList(fm, "metadata.openclaw.requires.env")...)
		requiredEnv = append(requiredEnv, frontmatterList(fm, "metadata.openclaw.env")...)
		requiredBins = append(requiredBins, frontmatterList(fm, "metadata.openclaw.requires.bins")...)
	}
	return skillDocument{
		Name:               name,
		Description:        description,
		Path:               path,
		Body:               body,
		Always:             always,
		FrontmatterPresent: frontmatterPresent,
		RequiredEnv:        uniqueSortedStrings(requiredEnv),
		RequiredBins:       uniqueSortedStrings(requiredBins),
	}
}

func frontmatter(body string) ([]string, bool) {
	body = strings.TrimPrefix(body, "\ufeff")
	if !strings.HasPrefix(body, "---\n") && strings.TrimSpace(body) != "---" {
		return nil, false
	}
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return lines[1:i], true
		}
	}
	return nil, false
}

func frontmatterValue(lines []string, key string) string {
	parts := strings.Split(key, ".")
	for i, line := range lines {
		if !frontmatterPathMatches(lines, i, parts) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), parts[len(parts)-1]+":"))
		value = strings.Trim(value, `"'`)
		return value
	}
	return ""
}

func frontmatterBool(lines []string, key string) bool {
	switch strings.ToLower(frontmatterValue(lines, key)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

func frontmatterList(lines []string, key string) []string {
	parts := strings.Split(key, ".")
	for i, line := range lines {
		if !frontmatterPathMatches(lines, i, parts) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), parts[len(parts)-1]+":"))
		items := parseFrontmatterInlineList(value)
		items = append(items, frontmatterChildList(lines, i)...)
		return uniqueSortedStrings(items)
	}
	return nil
}

func frontmatterChildList(lines []string, index int) []string {
	parentIndent := leadingSpaces(lines[index])
	var items []string
	for i := index + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := leadingSpaces(line)
		if indent <= parentIndent {
			break
		}
		if strings.HasPrefix(trimmed, "- ") {
			items = append(items, strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), `"'`))
		}
	}
	return items
}

func parseFrontmatterInlineList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func frontmatterPathMatches(lines []string, index int, parts []string) bool {
	line := lines[index]
	if !strings.Contains(strings.TrimSpace(line), ":") {
		return false
	}
	key := strings.TrimSpace(strings.SplitN(strings.TrimSpace(line), ":", 2)[0])
	if key != parts[len(parts)-1] {
		return false
	}
	indent := leadingSpaces(line)
	parentIndent := indent
	for p := len(parts) - 2; p >= 0; p-- {
		found := false
		for j := index - 1; j >= 0; j-- {
			candidate := lines[j]
			if strings.TrimSpace(candidate) == "" || !strings.Contains(strings.TrimSpace(candidate), ":") {
				continue
			}
			candidateIndent := leadingSpaces(candidate)
			if candidateIndent >= parentIndent {
				continue
			}
			candidateKey := strings.TrimSpace(strings.SplitN(strings.TrimSpace(candidate), ":", 2)[0])
			if candidateKey == parts[p] {
				parentIndent = candidateIndent
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func leadingSpaces(value string) int {
	return len(value) - len(strings.TrimLeft(value, " "))
}

func skillMatchesQuery(skill skillDocument, query string) bool {
	if query == "" {
		return false
	}
	candidates := []string{
		skill.Name,
		filepath.Base(filepath.Dir(filepath.FromSlash(skill.Path))),
		skill.Path,
	}
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" && strings.Contains(query, candidate) {
			return true
		}
	}
	for _, word := range searchableWords(skill.Description) {
		if strings.Contains(query, word) {
			return true
		}
	}
	return false
}

func selectedSkillBundleSet(bundles []skillBundleDocument, transcript []TranscriptMessage, cfg Config) map[string]bool {
	selected := map[string]bool{}
	if len(bundles) == 0 {
		return selected
	}
	byName := map[string]bool{}
	for _, bundle := range bundles {
		byName[bundle.Name] = true
	}
	for _, msg := range transcript {
		if msg.Role != "user" {
			continue
		}
		for _, line := range strings.Split(msg.Body, "\n") {
			fields := slashCommandFieldsFromLine(line, cfg.TriggerPrefix)
			if len(fields) == 0 {
				continue
			}
			name := normalizeSkillBundleName(strings.TrimPrefix(fields[0], "/"))
			if byName[name] {
				selected[name] = true
			}
		}
	}
	return selected
}

func skillSelectedByBundle(skill skillDocument, selected map[string]bool) bool {
	if len(selected) == 0 {
		return false
	}
	return selected[strings.ToLower(strings.TrimSpace(skill.Name))] || selected[strings.ToLower(skillFolderName(skill.Path))]
}

func summarizeSkillBundles(bundles []skillBundleDocument, skills []SkillSummary, selected map[string]bool) []SkillBundleSummary {
	skillNames := map[string]bool{}
	for _, skill := range skills {
		skillNames[strings.ToLower(strings.TrimSpace(skill.Name))] = true
		skillNames[strings.ToLower(skillFolderName(skill.Path))] = true
	}
	summaries := make([]SkillBundleSummary, 0, len(bundles))
	for _, bundle := range bundles {
		var resolved []string
		var missing []string
		for _, skill := range bundle.Skills {
			normalized := strings.ToLower(strings.TrimSpace(skill))
			if skillNames[normalized] {
				resolved = append(resolved, normalized)
			} else {
				missing = append(missing, normalized)
			}
		}
		summaries = append(summaries, SkillBundleSummary{
			Name:               bundle.Name,
			Description:        bundle.Description,
			Path:               bundle.Path,
			Skills:             append([]string(nil), bundle.Skills...),
			ResolvedSkills:     uniqueSortedStrings(resolved),
			MissingSkills:      uniqueSortedStrings(missing),
			Selected:           selected[bundle.Name],
			InstructionPresent: strings.TrimSpace(bundle.Instruction) != "",
			Bytes:              len(bundle.Body),
			Lines:              lineCount(bundle.Body),
			SHA:                shortDocumentHash(bundle.Body),
		})
	}
	return summaries
}

func normalizeSkillBundleSkills(values []string) []string {
	var out []string
	for _, value := range values {
		normalized := normalizeSkillBundleName(value)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return uniqueSortedStrings(out)
}

func normalizeSkillBundleName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastHyphen := false
	for _, r := range value {
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if allowed {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func searchableWords(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
	})
	var words []string
	for _, field := range fields {
		if len(field) < 5 || isStopWord(field) {
			continue
		}
		words = append(words, field)
	}
	return words
}

func isStopWord(value string) bool {
	switch value {
	case "about", "after", "before", "should", "would", "could", "using", "there", "their", "which", "where", "skill", "skills":
		return true
	default:
		return false
	}
}

func renderSkillIndex(skills []SkillSummary) string {
	var b strings.Builder
	for _, skill := range skills {
		fmt.Fprintf(&b, "- name=%s path=%s enabled=%t disabled_by_config=%t blocked_by_allowlist=%t always=%t frontmatter=%t description=%t bytes=%d lines=%d sha256_12=%s requires_env=%d requires_bins=%d missing_env=%d missing_bins=%d",
			skill.Name,
			skill.Path,
			skill.Enabled,
			skill.DisabledByConfig,
			skill.BlockedByAllowlist,
			skill.Always,
			skill.FrontmatterPresent,
			strings.TrimSpace(skill.Description) != "",
			skill.Bytes,
			skill.Lines,
			skill.SHA,
			len(skill.RequiredEnv),
			len(skill.RequiredBins),
			len(skill.MissingEnv),
			len(skill.MissingBins),
		)
		if skill.Description != "" {
			fmt.Fprintf(&b, " description=%q", skill.Description)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func missingEnvVars(names []string) []string {
	var missing []string
	for _, name := range names {
		if strings.TrimSpace(name) != "" && os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	return uniqueSortedStrings(missing)
}

func missingBins(names []string) []string {
	var missing []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	return uniqueSortedStrings(missing)
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func searchQueriesFromTranscript(transcript []TranscriptMessage) []string {
	text := transcriptText(transcript)
	queries := uniqueStrings(append(delimitedSearchQueries(text), commandSearchQueries(text)...))
	for _, match := range searchIdentifierPattern.FindAllString(text, -1) {
		if len(queries) >= maxSearchQueries {
			break
		}
		query := sanitizeSearchQuery(match)
		if query == "" || looksLikeRepoPath(query) || !looksLikeSearchIdentifier(query) || containsStringFold(queries, query) {
			continue
		}
		if isStopWord(strings.ToLower(query)) {
			continue
		}
		queries = append(queries, query)
	}
	if len(queries) > maxSearchQueries {
		return queries[:maxSearchQueries]
	}
	return queries
}

func delimitedSearchQueries(text string) []string {
	var queries []string
	for _, delimiter := range []rune{'`', '"'} {
		in := false
		var b strings.Builder
		for _, r := range text {
			if r == delimiter {
				if in {
					if query := sanitizeSearchQuery(b.String()); query != "" {
						queries = append(queries, query)
					}
					b.Reset()
				}
				in = !in
				continue
			}
			if in {
				b.WriteRune(r)
			}
		}
	}
	return queries
}

func commandSearchQueries(text string) []string {
	lower := strings.ToLower(text)
	triggers := []string{"search for ", "search repository for ", "find "}
	var queries []string
	for _, trigger := range triggers {
		start := 0
		for {
			idx := strings.Index(lower[start:], trigger)
			if idx < 0 {
				break
			}
			begin := start + idx + len(trigger)
			for begin < len(text) && text[begin] == ' ' {
				begin++
			}
			if begin < len(text) && (text[begin] == '`' || text[begin] == '"') {
				delimiter := text[begin]
				end := begin + 1
				for end < len(text) && text[end] != delimiter {
					end++
				}
				if end < len(text) {
					if query := sanitizeSearchQuery(text[begin+1 : end]); query != "" {
						queries = append(queries, query)
					}
					start = end + 1
					continue
				}
			}
			end := begin
			for end < len(text) {
				switch text[end] {
				case '\n', '.', ',', ';', ':':
					goto done
				default:
					end++
				}
			}
		done:
			if query := sanitizeSearchQuery(trimCommandSearchTail(text[begin:end])); query != "" {
				queries = append(queries, query)
			}
			start = end
		}
	}
	return queries
}

func trimCommandSearchTail(value string) string {
	lower := strings.ToLower(value)
	cut := len(value)
	for _, marker := range []string{" and ", " without ", " from ", " in ", " with "} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return value[:cut]
}

func sanitizeSearchQuery(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"'()[]{}")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) < 4 || len(value) > 120 {
		return ""
	}
	return value
}

func searchRepoFiles(root string, files []string, queries []string) string {
	var b strings.Builder
	matches := 0
	for _, query := range queries {
		queryLower := strings.ToLower(query)
		queryHeaderWritten := false
		queryMatches := 0
		for _, file := range files {
			if matches >= maxSearchMatches || queryMatches >= maxSearchMatchesPerQuery {
				break
			}
			body, err := readRepoTextFile(root, file, maxSearchFileBytes)
			if err != nil {
				continue
			}
			lines := strings.Split(body, "\n")
			for i, line := range lines {
				if matches >= maxSearchMatches || queryMatches >= maxSearchMatchesPerQuery {
					break
				}
				if !strings.Contains(strings.ToLower(line), queryLower) {
					continue
				}
				if !queryHeaderWritten {
					fmt.Fprintf(&b, "[query %q]\n", query)
					queryHeaderWritten = true
				}
				fmt.Fprintf(&b, "%s:%d:%s\n", file, i+1, trimSearchLine(line))
				matches++
				queryMatches++
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func trimSearchLine(line string) string {
	line = strings.Join(strings.Fields(line), " ")
	if len(line) <= maxSearchLineBytes {
		return line
	}
	return line[:maxSearchLineBytes] + "..."
}

func uniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || containsStringFold(unique, value) {
			continue
		}
		unique = append(unique, value)
	}
	return unique
}

func containsStringFold(values []string, value string) bool {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return true
		}
	}
	return false
}

func looksLikeRepoPath(value string) bool {
	return strings.Contains(value, "/") || strings.Contains(value, "\\")
}

func looksLikeSearchIdentifier(value string) bool {
	if strings.ContainsAny(value, "_.:-") {
		return true
	}
	hasLower := false
	hasUpper := false
	hasDigit := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasDigit || (hasLower && hasUpper && !isTitleCaseWord(value))
}

func isTitleCaseWord(value string) bool {
	if len(value) < 2 {
		return false
	}
	runes := []rune(value)
	if !(runes[0] >= 'A' && runes[0] <= 'Z') {
		return false
	}
	for _, r := range runes[1:] {
		if !(r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}

func listRepoFiles(root string) ([]string, error) {
	files := make([]string, 0, maxRepoFilesListed)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() && shouldSkipDir(name) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if len(files) >= maxRepoFilesListed || shouldSkipFile(name) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", ".cache":
		return true
	default:
		return false
	}
}

func shouldSkipFile(name string) bool {
	return strings.HasSuffix(name, ".png") ||
		strings.HasSuffix(name, ".jpg") ||
		strings.HasSuffix(name, ".jpeg") ||
		strings.HasSuffix(name, ".gif") ||
		strings.HasSuffix(name, ".pdf") ||
		strings.HasSuffix(name, ".zip")
}

func mentionedRepoFiles(files []string, transcript []TranscriptMessage) []string {
	text := strings.ToLower(transcriptText(transcript))
	var mentioned []string
	for _, file := range files {
		if len(mentioned) >= maxToolFilesRead {
			break
		}
		if strings.Contains(text, strings.ToLower(file)) {
			mentioned = append(mentioned, file)
		}
	}
	return mentioned
}

func transcriptText(transcript []TranscriptMessage) string {
	var b strings.Builder
	for _, msg := range transcript {
		if msg.Role != "user" {
			continue
		}
		b.WriteString(msg.Body)
		b.WriteByte('\n')
	}
	return b.String()
}

func readRepoTextFile(root, rel string, limit int) (string, error) {
	path, err := safeRepoPath(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("not a regular file: %s", rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > limit {
		data = data[:limit]
	}
	if strings.ContainsRune(string(data), '\x00') {
		return "", fmt.Errorf("binary file: %s", rel)
	}
	return strings.TrimSpace(string(data)), nil
}

func safeRepoPath(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid repo path: %s", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path escapes repository: %s", rel)
	}
	path := filepath.Join(root, clean)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if absPath != absRoot && !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository: %s", rel)
	}
	return absPath, nil
}
