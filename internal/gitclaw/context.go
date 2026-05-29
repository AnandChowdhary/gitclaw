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
)

var contextDocumentPaths = []string{
	"AGENTS.md",
	".github/copilot-instructions.md",
	".gitclaw/GITCLAW.md",
	".gitclaw/SOUL.md",
	".gitclaw/TOOLS.md",
	".gitclaw/MEMORY.md",
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
	ctx := RepoContext{
		Documents:   loadContextDocuments(absRoot, contextDocumentPaths),
		Skills:      loadSkillDocuments(absRoot),
		ToolOutputs: []ToolOutput{{Name: "gitclaw.list_files", Input: ".", Output: strings.Join(files, "\n")}},
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

func loadSkillDocuments(root string) []ContextDocument {
	var skills []ContextDocument
	for _, base := range []string{".gitclaw/SKILLS", ".gitclaw/skills"} {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(base), "*", "SKILL.md"))
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
			skills = append(skills, ContextDocument{Path: rel, Body: body})
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Path < skills[j].Path })
	return skills
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
