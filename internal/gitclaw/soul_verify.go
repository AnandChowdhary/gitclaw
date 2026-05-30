package gitclaw

import (
	"fmt"
	"strings"
)

type SoulVerifyReport struct {
	Status                    string
	Validation                SoulValidationReport
	Risk                      SoulRiskReport
	Documents                 int
	RepoLocalDocuments        int
	UnknownSourceDocuments    int
	RequiredDocuments         int
	RequiredDocumentsPresent  int
	RequiredDocumentsMissing  int
	SoulFilePresent           bool
	SoulFrontmatterPresent    bool
	SoulDescriptionPresent    bool
	IdentityPolicyFiles       int
	MemoryNotes               int
	FilesWithHashes           int
	RegistryVerification      string
	ProfileExportVerification string
	RawBodiesIncluded         bool
}

func BuildSoulVerifyReport(repoContext RepoContext) SoulVerifyReport {
	validation := ValidateSoulContext(repoContext)
	risk := BuildSoulRiskReport(repoContext)
	report := SoulVerifyReport{
		Status:                    validation.Status,
		Validation:                validation,
		Risk:                      risk,
		Documents:                 len(repoContext.Documents),
		RequiredDocuments:         validation.RequiredFiles,
		RequiredDocumentsPresent:  validation.PresentRequiredFiles,
		RequiredDocumentsMissing:  validation.MissingRequiredFiles,
		MemoryNotes:               validation.MemoryNotes,
		RegistryVerification:      "not_configured",
		ProfileExportVerification: "not_configured",
		RawBodiesIncluded:         false,
	}
	for _, doc := range repoContext.Documents {
		switch soulTrustSource(doc.Path) {
		case "repo-local":
			report.RepoLocalDocuments++
		default:
			report.UnknownSourceDocuments++
		}
		if !isSoulMemoryNote(doc.Path) {
			report.IdentityPolicyFiles++
		}
		if strings.TrimSpace(doc.Body) != "" {
			report.FilesWithHashes++
		}
		if doc.Path == ".gitclaw/SOUL.md" {
			report.SoulFilePresent = true
			if fm, ok := frontmatter(doc.Body); ok {
				report.SoulFrontmatterPresent = true
				report.SoulDescriptionPresent = strings.TrimSpace(frontmatterValue(fm, "description")) != ""
			}
		}
	}
	if report.UnknownSourceDocuments > 0 && report.Status == "ok" {
		report.Status = "warn"
	}
	if report.Status != "error" {
		switch risk.Status {
		case "high":
			report.Status = "high"
		case "warn":
			if report.Status == "ok" {
				report.Status = "warn"
			}
		}
	}
	return report
}

func RenderSoulVerifyReport(repoContext RepoContext) string {
	return renderSoulVerifyReport(Event{}, repoContext, false)
}

func renderSoulVerifyReport(ev Event, repoContext RepoContext, includeIssue bool) string {
	report := BuildSoulVerifyReport(repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Soul Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	if includeIssue {
		fmt.Fprintf(&b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(&b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(&b, "- scope: `%s`\n", "local-cli")
	}
	fmt.Fprintf(&b, "- soul_verify_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "repo-local-high-authority-context")
	fmt.Fprintf(&b, "- context_documents: `%d`\n", report.Documents)
	fmt.Fprintf(&b, "- repo_local_documents: `%d`\n", report.RepoLocalDocuments)
	fmt.Fprintf(&b, "- unknown_source_documents: `%d`\n", report.UnknownSourceDocuments)
	fmt.Fprintf(&b, "- required_documents: `%d`\n", report.RequiredDocuments)
	fmt.Fprintf(&b, "- required_documents_present: `%d`\n", report.RequiredDocumentsPresent)
	fmt.Fprintf(&b, "- required_documents_missing: `%d`\n", report.RequiredDocumentsMissing)
	fmt.Fprintf(&b, "- soul_file_present: `%t`\n", report.SoulFilePresent)
	fmt.Fprintf(&b, "- soul_frontmatter_present: `%t`\n", report.SoulFrontmatterPresent)
	fmt.Fprintf(&b, "- soul_description_present: `%t`\n", report.SoulDescriptionPresent)
	fmt.Fprintf(&b, "- identity_policy_files: `%d`\n", report.IdentityPolicyFiles)
	fmt.Fprintf(&b, "- memory_notes: `%d`\n", report.MemoryNotes)
	fmt.Fprintf(&b, "- files_with_hashes: `%d`\n", report.FilesWithHashes)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(&b, "- profile_export_verification: `%s`\n", report.ProfileExportVerification)
	fmt.Fprintf(&b, "- raw_bodies_included: `%t`\n", report.RawBodiesIncluded)
	writeSoulValidationSummary(&b, report.Validation)
	writeSoulRiskSummary(&b, report.Risk)
	b.WriteByte('\n')
	b.WriteString("This report is GitClaw's local trust envelope for high-authority context. It verifies repo-local soul, identity, user, memory, tool, heartbeat, and dated memory-note metadata. It does not contact an external soul registry, export a Hermes/OpenClaw profile, or include raw context, issue, comment, prompt, or secret bodies.\n\n")

	b.WriteString("### Trust Cards\n")
	if len(repoContext.Documents) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, doc := range repoContext.Documents {
			writeSoulTrustCard(&b, doc)
		}
	}

	b.WriteString("\n### Verification Findings\n")
	writeSoulVerifyFindings(&b, report)
	return strings.TrimSpace(b.String())
}

func writeSoulTrustCard(b *strings.Builder, doc ContextDocument) {
	frontmatterPresent := false
	descriptionPresent := false
	riskFindings := scanSoulRiskFindings(doc.Path, doc.Body)
	if fm, ok := frontmatter(doc.Body); ok {
		frontmatterPresent = true
		descriptionPresent = strings.TrimSpace(frontmatterValue(fm, "description")) != ""
	}
	fmt.Fprintf(b, "- path=`%s` category=`%s` source=`%s` required=`%t` frontmatter=`%t` description=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
		doc.Path,
		soulDocumentCategory(doc.Path),
		soulTrustSource(doc.Path),
		isRequiredSoulDocument(doc.Path),
		frontmatterPresent,
		descriptionPresent,
		len(doc.Body),
		lineCount(doc.Body),
		shortDocumentHash(doc.Body),
		len(riskFindings),
		soulRiskMaxSeverity(riskFindings),
		inlineListOrNone(soulRiskCodes(riskFindings)),
	)
}

func writeSoulVerifyFindings(b *strings.Builder, report SoulVerifyReport) {
	wrote := false
	if report.RegistryVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`registry_verification_not_configured` detail=`external soul registry signatures are not part of GitClaw repo-local verification`\n")
		wrote = true
	}
	if report.ProfileExportVerification == "not_configured" {
		b.WriteString("- severity=`info` code=`profile_export_verification_not_configured` detail=`Hermes/OpenClaw profile export verification is outside the GitClaw MVP boundary`\n")
		wrote = true
	}
	if report.UnknownSourceDocuments > 0 {
		b.WriteString("- severity=`warning` code=`unknown_context_source` detail=`one or more high-authority context files are outside known repo-local roots`\n")
		wrote = true
	}
	for _, finding := range report.Validation.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` path=`%s` detail=`%s`\n", finding.Severity, finding.Code, finding.Path, inlineCode(finding.Detail))
		wrote = true
	}
	for _, finding := range report.Risk.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` path=`%s` line=`%d` line_sha256_12=`%s`\n", finding.Severity, finding.Code, finding.Category, finding.Path, finding.Line, finding.LineSHA)
		wrote = true
	}
	if !wrote {
		b.WriteString("- none\n")
	}
}

func soulTrustSource(path string) string {
	if strings.HasPrefix(path, ".gitclaw/") {
		return "repo-local"
	}
	return "unknown"
}

func isRequiredSoulDocument(path string) bool {
	for _, required := range requiredSoulDocumentPaths {
		if path == required {
			return true
		}
	}
	return false
}
