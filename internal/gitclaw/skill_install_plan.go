package gitclaw

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

type skillInstallPlanTarget struct {
	Raw       string
	Type      string
	Candidate string
	Hash      string
	Terms     int
	Remote    bool
}

type skillInstallPlanFinding struct {
	Severity string
	Code     string
	Detail   string
}

func renderSkillInstallPlanReport(ev Event, repoContext RepoContext, operation, target string, includeIssue bool) string {
	operation = normalizeSkillInstallOperation(operation)
	targetInfo := classifySkillInstallTarget(target)
	matches := matchingInstallPlanSkillSummaries(repoContext.SkillSummaries, targetInfo)
	validation := ValidateSkillSummaries(repoContext.SkillSummaries)
	destinationPath := skillInstallDestinationPath(targetInfo.Candidate)
	destinationExists := len(matches) > 0
	findings := skillInstallPlanFindings(operation, targetInfo, matches, validation)
	status := skillInstallPlanStatus(findings)

	var b strings.Builder
	b.WriteString("## GitClaw Skill Install Plan Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- install_plan_status: `%s`\n", status)
	fmt.Fprintf(&b, "- operation: `%s`\n", operation)
	fmt.Fprintf(&b, "- target_type: `%s`\n", targetInfo.Type)
	fmt.Fprintf(&b, "- target_sha256_12: `%s`\n", targetInfo.Hash)
	fmt.Fprintf(&b, "- target_terms: `%d`\n", targetInfo.Terms)
	fmt.Fprintf(&b, "- safe_name_candidate: `%s`\n", inlineCode(targetInfo.Candidate))
	fmt.Fprintf(&b, "- destination_path: `%s`\n", destinationPath)
	fmt.Fprintf(&b, "- destination_exists: `%t`\n", destinationExists)
	fmt.Fprintf(&b, "- existing_skill_matches: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- available_skills: `%d`\n", availableSkillCount(repoContext))
	fmt.Fprintf(&b, "- run_mode: `%s`\n", "read-only")
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", false)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", false)
	fmt.Fprintf(&b, "- manual_review_required: `%t`\n", true)
	fmt.Fprintf(&b, "- llm_e2e_required_after_change: `%t`\n", true)
	fmt.Fprintf(&b, "- raw_target_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_manifest_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_body_included: `%t`\n", false)
	writeSkillValidationSummary(&b, validation)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This is a dry-run planner only. It classifies the requested skill source and shows the repo-local review path without fetching registries, running installers, installing dependencies, mutating `.gitclaw/SKILLS`, or dumping skill bodies.\n\n")

	b.WriteString("### Existing Skill Matches\n")
	if len(matches) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, skill := range matches {
			writeSkillInfoSummary(&b, skill, skillSelectedForTurn(repoContext, skill))
		}
	}

	b.WriteString("\n### Review Steps\n")
	if destinationPath == "" {
		b.WriteString("1. Provide a safe skill name, local skill path, or HTTPS/GitHub source.\n")
	} else {
		b.WriteString("1. Review the source outside the Actions job and ignore installer scripts by default.\n")
		fmt.Fprintf(&b, "2. If accepted, add or update `%s` on a reviewed branch.\n", destinationPath)
		b.WriteString("3. Run `gitclaw skills validate` and `gitclaw skills verify` before merging.\n")
		b.WriteString("4. Run a live GitHub Models conversation E2E after the skill change, not only deterministic report tests.\n")
	}

	b.WriteString("\n### Findings\n")
	writeSkillInstallPlanFindings(&b, findings)
	return strings.TrimSpace(b.String())
}

func cleanSkillInstallTarget(target string) string {
	return strings.Trim(strings.TrimSpace(target), "`\"'")
}

func normalizeSkillInstallOperation(operation string) string {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "upgrade-plan":
		return "upgrade-plan"
	default:
		return "install-plan"
	}
}

func classifySkillInstallTarget(target string) skillInstallPlanTarget {
	target = cleanSkillInstallTarget(target)
	info := skillInstallPlanTarget{
		Raw:   target,
		Type:  "registry-name",
		Hash:  shortDocumentHash(target),
		Terms: len(skillSearchTerms(target)),
	}
	if target == "" {
		info.Type = "empty"
		return info
	}
	lower := strings.ToLower(target)
	if strings.Contains(target, "\x00") || strings.Contains(target, "\\") || strings.HasPrefix(target, "/") || strings.Contains(target, "..") {
		info.Type = "unsafe-path"
		info.Candidate = normalizeSkillInstallCandidate(target)
		return info
	}
	if u, err := url.Parse(target); err == nil && u.Scheme != "" {
		switch {
		case strings.EqualFold(u.Scheme, "https") && strings.EqualFold(u.Hostname(), "github.com"):
			info.Type = "github-url"
			info.Remote = true
		case strings.EqualFold(u.Scheme, "https"):
			info.Type = "https-url"
			info.Remote = true
		case strings.EqualFold(u.Scheme, "http"):
			info.Type = "http-url"
			info.Remote = true
		default:
			info.Type = "unsupported-url"
			info.Remote = true
		}
		info.Candidate = skillInstallCandidateFromURL(u)
		return info
	}
	if strings.HasPrefix(lower, "git@github.com:") {
		info.Type = "github-ssh-url"
		info.Remote = true
		remotePath := strings.TrimPrefix(target, "git@github.com:")
		parts := strings.Split(remotePath, "/")
		if len(parts) >= 2 {
			info.Candidate = normalizeSkillInstallCandidate(parts[len(parts)-1])
		} else {
			info.Candidate = normalizeSkillInstallCandidate(remotePath)
		}
		return info
	}
	if strings.Contains(target, "://") {
		info.Type = "unsupported-url"
		info.Remote = true
		info.Candidate = normalizeSkillInstallCandidate(target)
		return info
	}
	if strings.Contains(target, "/") {
		if strings.HasPrefix(lower, ".gitclaw/") || strings.HasSuffix(lower, "skill.md") {
			info.Type = "local-path"
			info.Candidate = skillInstallCandidateFromPath(target)
			return info
		}
		if isLikelyGitHubRepoShorthand(target) {
			info.Type = "github-shorthand"
			info.Remote = true
			parts := strings.Split(strings.TrimSuffix(target, ".git"), "/")
			if len(parts) >= 2 {
				info.Candidate = normalizeSkillInstallCandidate(parts[1])
			}
			return info
		}
		info.Type = "local-path"
		info.Candidate = skillInstallCandidateFromPath(target)
		return info
	}
	info.Candidate = normalizeSkillInstallCandidate(target)
	return info
}

func skillInstallCandidateFromURL(u *url.URL) string {
	cleanPath := path.Clean(u.EscapedPath())
	if cleanPath == "." || cleanPath == "/" {
		return ""
	}
	base := path.Base(cleanPath)
	if strings.EqualFold(base, "SKILL.md") {
		base = path.Base(path.Dir(cleanPath))
	}
	base = strings.TrimSuffix(base, ".git")
	return normalizeSkillInstallCandidate(base)
}

func skillInstallCandidateFromPath(value string) string {
	cleanPath := filepath.ToSlash(filepath.Clean(value))
	base := path.Base(cleanPath)
	if strings.EqualFold(base, "SKILL.md") {
		base = path.Base(path.Dir(cleanPath))
	}
	return normalizeSkillInstallCandidate(base)
}

func normalizeSkillInstallCandidate(value string) string {
	value = strings.TrimSuffix(strings.TrimSpace(strings.ToLower(value)), ".git")
	var b strings.Builder
	lastHyphen := false
	for _, r := range value {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 64 {
		out = strings.Trim(out[:64], "-")
	}
	return out
}

func skillInstallDestinationPath(candidate string) string {
	if candidate == "" {
		return ""
	}
	return ".gitclaw/SKILLS/" + candidate + "/SKILL.md"
}

func isLikelyGitHubRepoShorthand(target string) bool {
	parts := strings.Split(target, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	return normalizeSkillInstallCandidate(parts[0]) != "" && normalizeSkillInstallCandidate(strings.TrimSuffix(parts[1], ".git")) != ""
}

func matchingInstallPlanSkillSummaries(skills []SkillSummary, target skillInstallPlanTarget) []SkillSummary {
	seen := map[string]bool{}
	var matches []SkillSummary
	add := func(skill SkillSummary) {
		if seen[skill.Path] {
			return
		}
		seen[skill.Path] = true
		matches = append(matches, skill)
	}
	for _, match := range matchingSkillSummaries(skills, target.Candidate) {
		add(match)
	}
	for _, skill := range skills {
		if strings.EqualFold(skill.Path, target.Raw) || strings.EqualFold(skill.Path, filepath.ToSlash(filepath.Clean(target.Raw))) {
			add(skill)
		}
	}
	return matches
}

func skillInstallPlanFindings(operation string, target skillInstallPlanTarget, matches []SkillSummary, validation SkillValidationReport) []skillInstallPlanFinding {
	var findings []skillInstallPlanFinding
	add := func(severity, code, detail string) {
		findings = append(findings, skillInstallPlanFinding{Severity: severity, Code: code, Detail: detail})
	}
	add("info", "manual_review_required", "skill changes must be reviewed as repository code before they affect model context")
	add("info", "installer_scripts_disabled", "install planning never runs remote scripts or local installer hooks")
	add("info", "repository_mutation_disabled", "install planning does not create, update, delete, commit, or push files")
	switch target.Type {
	case "empty":
		add("error", "target_empty", "provide a skill name, local skill path, GitHub shorthand, or HTTPS URL")
	case "unsafe-path":
		add("error", "unsafe_target_path", "absolute paths, parent traversal, backslashes, and NUL bytes are rejected")
	case "http-url":
		add("error", "insecure_http_url", "HTTP skill sources are blocked; use HTTPS and manual review")
	case "unsupported-url":
		add("error", "unsupported_url_scheme", "only HTTPS URLs and GitHub SSH shorthand can be planned")
	}
	if target.Remote {
		add("warning", "network_fetch_disabled", "remote targets are classified only; the Actions job does not fetch skill code")
	}
	if target.Candidate == "" {
		add("error", "safe_name_candidate_empty", "could not derive a safe repo-local skill folder name")
	} else if !skillNamePattern.MatchString(target.Candidate) {
		add("error", "safe_name_candidate_invalid", "safe repo-local skill folder name must match ^[a-z0-9][a-z0-9-]*$")
	}
	if len(matches) > 0 {
		add("warning", "existing_skill_found", "the candidate already maps to a repo-local skill; review overwrite/upgrade intent")
	}
	if operation == "upgrade-plan" && len(matches) == 0 {
		add("error", "upgrade_target_missing", "upgrade plans require an existing repo-local skill match")
	}
	if validation.Errors > 0 {
		add("error", "skill_validation_errors_present", "fix existing skill validation errors before installing or upgrading skills")
	} else if validation.Warnings > 0 {
		add("warning", "skill_validation_warnings_present", "review existing skill validation warnings before installing or upgrading skills")
	}
	return findings
}

func skillInstallPlanStatus(findings []skillInstallPlanFinding) string {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return "blocked"
		}
	}
	for _, finding := range findings {
		if finding.Severity == "warning" {
			return "needs_review"
		}
	}
	return "ok"
}

func writeSkillInstallPlanFindings(b *strings.Builder, findings []skillInstallPlanFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` detail=`%s`\n", finding.Severity, finding.Code, inlineCode(finding.Detail))
	}
}
