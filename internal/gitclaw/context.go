package gitclaw

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxContextDocumentBytes = 12000
	maxToolReadBytes        = 8000
	maxRepoFilesListed      = 240
	maxToolFilesRead        = 5
	maxMemoryDocuments      = 3
)

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
}

func LoadRepoContext(root string, transcript []TranscriptMessage) (RepoContext, error) {
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
	skillSummaries, skills := loadSkillContext(absRoot, transcript)
	ctx := RepoContext{
		Documents:   documents,
		Skills:      skills,
		ToolOutputs: []ToolOutput{{Name: "gitclaw.list_files", Input: ".", Output: strings.Join(files, "\n")}},
	}
	if len(skillSummaries) > 0 {
		ctx.ToolOutputs = append(ctx.ToolOutputs, ToolOutput{
			Name:   "gitclaw.skill_index",
			Input:  ".gitclaw/SKILLS",
			Output: renderSkillIndex(skillSummaries),
		})
	}
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

func loadSkillContext(root string, transcript []TranscriptMessage) ([]SkillSummary, []ContextDocument) {
	available := discoverSkills(root)
	if len(available) == 0 {
		return nil, nil
	}
	query := strings.ToLower(transcriptText(transcript))
	summaries := make([]SkillSummary, 0, len(available))
	selected := make([]ContextDocument, 0, len(available))
	for _, skill := range available {
		summaries = append(summaries, SkillSummary{
			Name:        skill.Name,
			Description: skill.Description,
			Path:        skill.Path,
			Always:      skill.Always,
		})
		if skill.Always || skillMatchesQuery(skill, query) {
			selected = append(selected, ContextDocument{Path: skill.Path, Body: skill.Body})
		}
	}
	return summaries, selected
}

type skillDocument struct {
	Name        string
	Description string
	Path        string
	Body        string
	Always      bool
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

func parseSkillDocument(path, body string) skillDocument {
	name := filepath.Base(filepath.Dir(filepath.FromSlash(path)))
	description := ""
	always := false
	if fm, ok := frontmatter(body); ok {
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
	}
	return skillDocument{
		Name:        name,
		Description: description,
		Path:        path,
		Body:        body,
		Always:      always,
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
		fmt.Fprintf(&b, "- name=%s path=%s always=%t", skill.Name, skill.Path, skill.Always)
		if skill.Description != "" {
			fmt.Fprintf(&b, " description=%q", skill.Description)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
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
