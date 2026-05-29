package gitclaw

import (
	"strings"
	"testing"
)

func TestValidateSkillSummariesReportsProblemsWithoutBodies(t *testing.T) {
	skills := []SkillSummary{
		{
			Name:               "Bad Skill",
			Description:        "",
			Path:               ".gitclaw/SKILLS/bad-skill/SKILL.md",
			FrontmatterPresent: false,
			Bytes:              42,
			Lines:              3,
			SHA:                "abc",
			MissingEnv:         []string{"SECRET_TOKEN"},
			MissingBins:        []string{"missing-bin"},
		},
		{
			Name:               "dup",
			Description:        "First duplicate",
			Path:               ".gitclaw/SKILLS/dup/SKILL.md",
			FrontmatterPresent: true,
		},
		{
			Name:               "dup",
			Description:        "Second duplicate",
			Path:               ".gitclaw/SKILLS/dup-two/SKILL.md",
			FrontmatterPresent: true,
		},
	}
	report := ValidateSkillSummaries(skills)
	if report.Status != "error" || report.Errors != 3 || report.Warnings != 5 || report.Duplicates != 1 || report.InvalidNames != 1 || report.Mismatches != 2 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
	body := RenderSkillsValidationReport(RepoContext{SkillSummaries: skills})
	for _, want := range []string{
		"GitClaw Skills Validate Report",
		"skill_validation_status: `error`",
		"skill_validation_errors: `3`",
		"skill_validation_warnings: `5`",
		"skill_duplicate_names: `1`",
		"skill_invalid_names: `1`",
		"skill_name_folder_mismatches: `2`",
		"code=`missing_frontmatter`",
		"code=`missing_description`",
		"code=`invalid_name`",
		"code=`missing_requirements`",
		"code=`duplicate_name`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("validation report missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "SECRET_BODY_TOKEN") {
		t.Fatalf("validation report leaked skill body token:\n%s", body)
	}
}

func TestValidateSkillSummariesAcceptsCurrentSkillShape(t *testing.T) {
	report := ValidateSkillSummaries([]SkillSummary{{
		Name:               "repo-reader",
		Description:        "Use read-only repository context.",
		Path:               ".gitclaw/SKILLS/repo-reader/SKILL.md",
		FrontmatterPresent: true,
	}})
	if report.Status != "ok" || report.Errors != 0 || report.Warnings != 0 {
		t.Fatalf("unexpected validation report: %#v", report)
	}
}
