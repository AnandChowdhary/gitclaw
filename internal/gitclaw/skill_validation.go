package gitclaw

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type SkillValidationReport struct {
	Skills       int
	Status       string
	Errors       int
	Warnings     int
	Duplicates   int
	InvalidNames int
	Mismatches   int
	Findings     []SkillValidationFinding
}

type SkillValidationFinding struct {
	Severity string
	Code     string
	Path     string
	Detail   string
}

func ValidateSkillSummaries(skills []SkillSummary) SkillValidationReport {
	report := SkillValidationReport{
		Skills: len(skills),
		Status: "ok",
	}
	byName := map[string][]SkillSummary{}
	for _, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		byName[name] = append(byName[name], skill)
		if !skill.FrontmatterPresent {
			report.addFinding("error", "missing_frontmatter", skill.Path, "SKILL.md should start with YAML frontmatter")
		}
		if name == "" {
			report.addFinding("error", "missing_name", skill.Path, "skill name is empty")
		} else if !skillNamePattern.MatchString(name) {
			report.InvalidNames++
			report.addFinding("error", "invalid_name", skill.Path, "skill name must match ^[a-z0-9][a-z0-9-]*$")
		}
		if strings.TrimSpace(skill.Description) == "" {
			report.addFinding("error", "missing_description", skill.Path, "frontmatter description is required")
		}
		if folder := skillFolderName(skill.Path); folder != "" && name != "" && folder != name {
			report.Mismatches++
			report.addFinding("warning", "name_folder_mismatch", skill.Path, fmt.Sprintf("folder %q does not match skill name %q", folder, name))
		}
		if len(skill.MissingEnv) > 0 || len(skill.MissingBins) > 0 {
			report.addFinding("warning", "missing_requirements", skill.Path, fmt.Sprintf("missing_env=%d missing_bins=%d", len(skill.MissingEnv), len(skill.MissingBins)))
		}
	}
	for name, matches := range byName {
		if name == "" || len(matches) < 2 {
			continue
		}
		report.Duplicates++
		paths := make([]string, 0, len(matches))
		for _, skill := range matches {
			paths = append(paths, skill.Path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			report.addFinding("warning", "duplicate_name", path, fmt.Sprintf("skill name %q is declared by %d SKILL.md files", name, len(paths)))
		}
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
	if report.Errors > 0 {
		report.Status = "error"
	} else if report.Warnings > 0 {
		report.Status = "warn"
	}
	return report
}

func RenderSkillsValidationReport(repoContext RepoContext) string {
	return renderSkillsValidationReport(Event{}, repoContext, false)
}

func renderSkillsValidationReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	var b strings.Builder
	b.WriteString("## GitClaw Skills Validate Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	writeSkillValidationSummary(&b, validation)
	b.WriteString("\nThis report validates local skill metadata without dumping full `SKILL.md` bodies, issue bodies, comments, prompts, or secret values.\n")
	b.WriteString("\n### Findings\n")
	writeSkillValidationFindings(&b, validation)
	return strings.TrimSpace(b.String())
}

func writeSkillValidationSummary(b *strings.Builder, validation SkillValidationReport) {
	fmt.Fprintf(b, "- skill_validation_status: `%s`\n", validation.Status)
	fmt.Fprintf(b, "- skill_validation_errors: `%d`\n", validation.Errors)
	fmt.Fprintf(b, "- skill_validation_warnings: `%d`\n", validation.Warnings)
	fmt.Fprintf(b, "- skill_duplicate_names: `%d`\n", validation.Duplicates)
	fmt.Fprintf(b, "- skill_invalid_names: `%d`\n", validation.InvalidNames)
	fmt.Fprintf(b, "- skill_name_folder_mismatches: `%d`\n", validation.Mismatches)
}

func writeSkillValidationFindings(b *strings.Builder, validation SkillValidationReport) {
	if len(validation.Findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
	}
}

func (r *SkillValidationReport) addFinding(severity, code, path, detail string) {
	r.Findings = append(r.Findings, SkillValidationFinding{
		Severity: severity,
		Code:     code,
		Path:     path,
		Detail:   detail,
	})
	switch severity {
	case "error":
		r.Errors++
	case "warning":
		r.Warnings++
	}
}

func skillFolderName(path string) string {
	dir := filepath.Dir(filepath.FromSlash(path))
	if dir == "." || dir == "/" {
		return ""
	}
	return filepath.Base(dir)
}
