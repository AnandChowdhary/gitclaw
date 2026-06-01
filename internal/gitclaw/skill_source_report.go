package gitclaw

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillSourcesDir = ".gitclaw/skill-sources"
const externalClawHubLockPath = ".clawhub/lock.json"
const defaultSkillSourceSearchMaxResults = 10

type skillSourceDocument struct {
	Name               string
	Path               string
	Body               string
	SkillPath          string
	SourceKind         string
	SourceRef          string
	TrustLevel         string
	InstallMode        string
	ExpectedSHA        string
	RequiresApproval   bool
	RemoteFetchAllowed bool
	ParseError         string
}

type SkillSourceCard struct {
	Name               string
	Path               string
	SkillPath          string
	SkillMatched       bool
	SkillSHA           string
	SourceKind         string
	SourceRefPresent   bool
	SourceRefSHA       string
	TrustLevel         string
	InstallMode        string
	ExpectedSHA        string
	HashPinned         bool
	HashMatched        bool
	HashMismatched     bool
	RequiresApproval   bool
	RemoteFetchAllowed bool
	Bytes              int
	Lines              int
	SHA                string
	ParseError         string
	RiskFindings       []SkillSourceRiskFinding
}

type SkillSourceRiskFinding struct {
	Severity string
	Code     string
	Category string
	Name     string
	Path     string
	Field    string
	Line     int
	LineSHA  string
}

type SkillSourceSearchResult struct {
	Card        SkillSourceCard
	MatchFields []string
	Score       int
}

type SkillSourceSearchReport struct {
	Status                  string
	QueryHash               string
	QueryTerms              int
	MaxResults              int
	AvailableSourcePins     int
	MatchedSourcePins       int
	ResultsReturned         int
	RawQueryIncluded        bool
	RawSourceBodiesIncluded bool
	RawSourceRefsIncluded   bool
	RawSkillBodiesIncluded  bool
	Results                 []SkillSourceSearchResult
}

type SkillSourceLockReport struct {
	Status                    string
	Source                    SkillSourceReport
	LockEntries               int
	LockedSourcePins          int
	ReproducibleSourcePins    int
	StaleSourcePins           int
	UnpinnedSourcePins        int
	MissingSkillPins          int
	RemoteFetchAllowedPins    int
	RiskFindingPins           int
	ClawHubLockPresent        bool
	ClawHubLockHash           string
	LockManifestHash          string
	RegistryContactAllowed    bool
	RemoteFetchAllowed        bool
	InstallerScriptsRun       bool
	DependencyInstallAllowed  bool
	RepositoryMutationAllowed bool
	RawLockfileIncluded       bool
	RawSourceBodiesIncluded   bool
	RawSourceRefsIncluded     bool
	RawSkillBodiesIncluded    bool
}

type SkillSourceReport struct {
	Status                          string
	Specs                           int
	ParsedSpecs                     int
	MatchedSources                  int
	HashPinnedSources               int
	HashMatchedSources              int
	HashMismatchedSources           int
	MissingSkillMatches             int
	RepoLocalSourceRefs             int
	RemoteSourceRefs                int
	SourcesRequiringApproval        int
	RemoteFetchAllowedSpecs         int
	Findings                        []SkillSourceRiskFinding
	SourcesWithRiskFindings         int
	HighRiskFindings                int
	WarningRiskFindings             int
	InfoRiskFindings                int
	RegistryContactAllowed          bool
	InstallerScriptsRun             bool
	DependencyInstallAllowed        bool
	RepositoryMutationAllowed       bool
	RawSourceBodiesIncluded         bool
	RawSourceRefsIncluded           bool
	RawSkillBodiesIncluded          bool
	LLME2ERequiredAfterSourceChange bool
	Cards                           []SkillSourceCard
}

type SkillSourceVerifyReport struct {
	Status                     string
	Source                     SkillSourceReport
	SourcePinsHashed           int
	SourceRefsHashed           int
	CurrentSkillHashesObserved int
	RegistryVerification       string
	RemoteFetchVerification    string
	InstallVerification        string
	RegistryContactAllowed     bool
	RemoteFetchAllowed         bool
	InstallerScriptsRun        bool
	DependencyInstallAllowed   bool
	RepositoryMutationAllowed  bool
	RawSourceBodiesIncluded    bool
	RawSourceRefsIncluded      bool
	RawSkillBodiesIncluded     bool
}

func RenderSkillSourcesCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillSourcesReport(Event{}, cfg, repoContext, false)
}

func RenderSkillSourcesRiskCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillSourcesRiskReport(Event{}, cfg, repoContext, false)
}

func RenderSkillSourcesVerifyCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillSourcesVerifyReport(Event{}, cfg, repoContext, false)
}

func RenderSkillSourcesSearchCLIReport(cfg Config, repoContext RepoContext, query string) string {
	return renderSkillSourcesSearchReport(Event{}, cfg, repoContext, query, defaultSkillSourceSearchMaxResults, false)
}

func RenderSkillSourcesLockCLIReport(cfg Config, repoContext RepoContext) string {
	return renderSkillSourcesLockReport(Event{}, cfg, repoContext, false)
}

func RenderSkillSourceInfoCLIReport(cfg Config, repoContext RepoContext, name string) string {
	return renderSkillSourceInfoReport(Event{}, cfg, repoContext, name, false)
}

func renderSkillSourcesReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillSourceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Sources Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	writeSkillSourceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("Skill sources are repo-reviewed provenance pins for local skills. GitClaw inventories them without contacting registries, fetching remote sources, running installers, mutating `.gitclaw/SKILLS`, or printing raw source refs, source bodies, skill bodies, issue bodies, comments, prompts, or provider payloads.\n\n")

	b.WriteString("### Skill Sources\n")
	writeSkillSourceCards(&b, report.Cards)
	return strings.TrimSpace(b.String())
}

func renderSkillSourcesRiskReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillSourceReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Sources Risk Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	writeSkillSourceSummary(&b, report)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report checks repo-local skill source pins for missing skills, hash mismatches, unsafe remote-fetch gates, installer-like install modes, missing approval gates, untrusted source kinds, prompt-boundary overrides, credential material, host execution, repository mutation, remote exfiltration, and unbounded loops. It reports only metadata, counts, hashes, paths, risk codes, and severities.\n\n")

	b.WriteString("### Skill Source Risk Cards\n")
	writeSkillSourceCards(&b, report.Cards)

	b.WriteString("\n### Risk Findings\n")
	writeSkillSourceRiskFindings(&b, report.Findings)
	return strings.TrimSpace(b.String())
}

func BuildSkillSourceSearchReport(cfg Config, repoContext RepoContext, query string, maxResults int) SkillSourceSearchReport {
	query = cleanSkillSearchQuery(query)
	if maxResults <= 0 {
		maxResults = defaultSkillSourceSearchMaxResults
	}
	source := BuildSkillSourceReport(cfg, repoContext)
	report := SkillSourceSearchReport{
		Status:                  "ok",
		QueryHash:               shortDocumentHash(query),
		QueryTerms:              len(skillSearchTerms(query)),
		MaxResults:              maxResults,
		AvailableSourcePins:     len(source.Cards),
		RawQueryIncluded:        false,
		RawSourceBodiesIncluded: false,
		RawSourceRefsIncluded:   false,
		RawSkillBodiesIncluded:  false,
	}
	if query == "" {
		report.Status = "no_query"
		return report
	}
	terms := skillSearchTerms(query)
	if len(terms) == 0 {
		report.Status = "no_query"
		return report
	}
	for _, card := range source.Cards {
		score, fields := skillSourceSearchScore(card, query, terms)
		if score == 0 {
			continue
		}
		report.MatchedSourcePins++
		results := SkillSourceSearchResult{Card: card, MatchFields: fields, Score: score}
		report.Results = append(report.Results, results)
	}
	sort.Slice(report.Results, func(i, j int) bool {
		if report.Results[i].Score != report.Results[j].Score {
			return report.Results[i].Score > report.Results[j].Score
		}
		return report.Results[i].Card.Path < report.Results[j].Card.Path
	})
	if len(report.Results) > maxResults {
		report.Results = report.Results[:maxResults]
	}
	report.ResultsReturned = len(report.Results)
	if report.MatchedSourcePins == 0 {
		report.Status = "no_matches"
	}
	return report
}

func renderSkillSourcesSearchReport(ev Event, cfg Config, repoContext RepoContext, query string, maxResults int, includeIssue bool) string {
	report := BuildSkillSourceSearchReport(cfg, repoContext, query, maxResults)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Source Search Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	fmt.Fprintf(&b, "- skill_source_search_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- query_sha256_12: `%s`\n", report.QueryHash)
	fmt.Fprintf(&b, "- query_terms: `%d`\n", report.QueryTerms)
	fmt.Fprintf(&b, "- max_results: `%d`\n", report.MaxResults)
	fmt.Fprintf(&b, "- available_source_pins: `%d`\n", report.AvailableSourcePins)
	fmt.Fprintf(&b, "- matched_source_pins: `%d`\n", report.MatchedSourcePins)
	fmt.Fprintf(&b, "- results_returned: `%d`\n", report.ResultsReturned)
	fmt.Fprintf(&b, "- raw_query_included: `%t`\n", report.RawQueryIncluded)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", report.RawSourceBodiesIncluded)
	fmt.Fprintf(&b, "- raw_source_refs_included: `%t`\n", report.RawSourceRefsIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_source_search_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report searches reviewed skill source-pin metadata only: source name, path, skill path, source kind, trust level, install mode, expected/current hashes, and risk codes. Raw search queries, raw source refs, source YAML, skill bodies, issue bodies, comments, prompts, provider payloads, and secret values are not included.\n\n")

	b.WriteString("### Matches\n")
	writeSkillSourceSearchResults(&b, report.Results)

	if report.ResultsReturned == 0 {
		source := BuildSkillSourceReport(cfg, repoContext)
		b.WriteString("\n### Available Skill Sources\n")
		writeSkillSourceCards(&b, source.Cards)
	}
	return strings.TrimSpace(b.String())
}

func BuildSkillSourceLockReport(cfg Config, repoContext RepoContext) SkillSourceLockReport {
	source := BuildSkillSourceReport(cfg, repoContext)
	clawHubLockPresent, clawHubLockHash := inspectExternalClawHubLock(cfg.Workdir)
	report := SkillSourceLockReport{
		Status:                    source.Status,
		Source:                    source,
		LockEntries:               len(source.Cards),
		ClawHubLockPresent:        clawHubLockPresent,
		ClawHubLockHash:           clawHubLockHash,
		LockManifestHash:          skillSourceLockManifestHash(source.Cards),
		RegistryContactAllowed:    false,
		RemoteFetchAllowed:        false,
		InstallerScriptsRun:       false,
		DependencyInstallAllowed:  false,
		RepositoryMutationAllowed: false,
		RawLockfileIncluded:       false,
		RawSourceBodiesIncluded:   false,
		RawSourceRefsIncluded:     false,
		RawSkillBodiesIncluded:    false,
	}
	for _, card := range source.Cards {
		if card.SkillMatched && card.HashPinned && card.HashMatched {
			report.ReproducibleSourcePins++
		}
		if skillSourceLockState(card) == "locked" {
			report.LockedSourcePins++
		}
		if card.HashMismatched {
			report.StaleSourcePins++
		}
		if !card.HashPinned {
			report.UnpinnedSourcePins++
		}
		if !card.SkillMatched {
			report.MissingSkillPins++
		}
		if card.RemoteFetchAllowed {
			report.RemoteFetchAllowedPins++
		}
		if len(card.RiskFindings) > 0 {
			report.RiskFindingPins++
		}
	}
	if report.Status == "ok" && report.LockEntries > 0 && report.LockedSourcePins != report.LockEntries {
		report.Status = "warn"
	}
	return report
}

func renderSkillSourcesLockReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillSourceLockReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Source Lock Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	fmt.Fprintf(&b, "- skill_source_lock_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- lock_scope: `%s`\n", "repo-local-source-pin-reproducibility")
	writeSkillSourceSummary(&b, report.Source)
	fmt.Fprintf(&b, "- lock_entries: `%d`\n", report.LockEntries)
	fmt.Fprintf(&b, "- locked_source_pins: `%d`\n", report.LockedSourcePins)
	fmt.Fprintf(&b, "- reproducible_source_pins: `%d`\n", report.ReproducibleSourcePins)
	fmt.Fprintf(&b, "- stale_source_pins: `%d`\n", report.StaleSourcePins)
	fmt.Fprintf(&b, "- unpinned_source_pins: `%d`\n", report.UnpinnedSourcePins)
	fmt.Fprintf(&b, "- missing_skill_pins: `%d`\n", report.MissingSkillPins)
	fmt.Fprintf(&b, "- remote_fetch_allowed_pins: `%d`\n", report.RemoteFetchAllowedPins)
	fmt.Fprintf(&b, "- risk_finding_pins: `%d`\n", report.RiskFindingPins)
	fmt.Fprintf(&b, "- external_clawhub_lock_path: `%s`\n", externalClawHubLockPath)
	fmt.Fprintf(&b, "- external_clawhub_lock_present: `%t`\n", report.ClawHubLockPresent)
	fmt.Fprintf(&b, "- external_clawhub_lock_sha256_12: `%s`\n", noneIfEmpty(report.ClawHubLockHash))
	fmt.Fprintf(&b, "- lock_manifest_sha256_12: `%s`\n", report.LockManifestHash)
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(&b, "- remote_fetch_allowed: `%t`\n", report.RemoteFetchAllowed)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_lockfile_included: `%t`\n", report.RawLockfileIncluded)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", report.RawSourceBodiesIncluded)
	fmt.Fprintf(&b, "- raw_source_refs_included: `%t`\n", report.RawSourceRefsIncluded)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_source_lock_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report projects a reproducibility lock from reviewed repo-local source pins. It mirrors OpenClaw/ClawHub tracked-install pressure without loading registry state: source pins, source refs, and local skills are represented by hashes and gates only.\n\n")

	b.WriteString("### Lock Entries\n")
	writeSkillSourceLockCards(&b, report.Source.Cards)

	b.WriteString("\n### Lock Gates\n")
	fmt.Fprintf(&b, "- lockfile_gate=`%s`\n", "derived-from-reviewed-source-pins")
	b.WriteString("- external_clawhub_lock_gate=`presence-and-hash-only`\n")
	b.WriteString("- registry_gate=`disabled`\n")
	b.WriteString("- remote_fetch_gate=`disabled`\n")
	b.WriteString("- installer_gate=`disabled`\n")
	b.WriteString("- dependency_install_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")
	return strings.TrimSpace(b.String())
}

func BuildSkillSourceVerifyReport(cfg Config, repoContext RepoContext) SkillSourceVerifyReport {
	source := BuildSkillSourceReport(cfg, repoContext)
	report := SkillSourceVerifyReport{
		Status:                    source.Status,
		Source:                    source,
		RegistryVerification:      "not_configured",
		RemoteFetchVerification:   "static_source_pins_only",
		InstallVerification:       "disabled_gates_only",
		RegistryContactAllowed:    false,
		RemoteFetchAllowed:        false,
		InstallerScriptsRun:       false,
		DependencyInstallAllowed:  false,
		RepositoryMutationAllowed: false,
		RawSourceBodiesIncluded:   false,
		RawSourceRefsIncluded:     false,
		RawSkillBodiesIncluded:    false,
	}
	for _, card := range source.Cards {
		if card.SHA != "" {
			report.SourcePinsHashed++
		}
		if card.SourceRefPresent {
			report.SourceRefsHashed++
		}
		if card.SkillSHA != "" {
			report.CurrentSkillHashesObserved++
		}
	}
	return report
}

func renderSkillSourcesVerifyReport(ev Event, cfg Config, repoContext RepoContext, includeIssue bool) string {
	report := BuildSkillSourceVerifyReport(cfg, repoContext)
	var b strings.Builder
	b.WriteString("## GitClaw Skill Source Verify Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	fmt.Fprintf(&b, "- skill_source_verify_status: `%s`\n", report.Status)
	fmt.Fprintf(&b, "- verification_scope: `%s`\n", "repo-local-source-pin-trust")
	writeSkillSourceSummary(&b, report.Source)
	fmt.Fprintf(&b, "- source_pins_hashed: `%d`\n", report.SourcePinsHashed)
	fmt.Fprintf(&b, "- source_refs_hashed: `%d`\n", report.SourceRefsHashed)
	fmt.Fprintf(&b, "- current_skill_hashes_observed: `%d`\n", report.CurrentSkillHashesObserved)
	fmt.Fprintf(&b, "- registry_verification: `%s`\n", report.RegistryVerification)
	fmt.Fprintf(&b, "- remote_fetch_verification: `%s`\n", report.RemoteFetchVerification)
	fmt.Fprintf(&b, "- install_verification: `%s`\n", report.InstallVerification)
	fmt.Fprintf(&b, "- remote_fetch_runtime_allowed: `%t`\n", report.RemoteFetchAllowed)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_source_verify_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report verifies reviewed skill source pins as repo-local trust envelopes. It reports source-pin hashes, source-ref hashes, current skill hashes, approval/no-fetch/no-install gates, and risk findings without contacting registries, fetching remote sources, running installers, mutating skills, or printing raw source refs, source YAML, skill bodies, issue bodies, comments, prompts, provider payloads, or secret values.\n\n")

	b.WriteString("### Source Pin Trust Cards\n")
	writeSkillSourceCards(&b, report.Source.Cards)

	b.WriteString("\n### Verification Findings\n")
	writeSkillSourceVerifyFindings(&b, report)
	return strings.TrimSpace(b.String())
}

func renderSkillSourceInfoReport(ev Event, cfg Config, repoContext RepoContext, name string, includeIssue bool) string {
	report := BuildSkillSourceReport(cfg, repoContext)
	normalized := normalizeSkillSourceName(name)
	matches := matchingSkillSourceCards(report.Cards, normalized)
	status := "not_found"
	if len(matches) == 1 {
		status = "ok"
	} else if len(matches) > 1 {
		status = "ambiguous"
	}

	var b strings.Builder
	b.WriteString("## GitClaw Skill Source Info Report\n\n")
	b.WriteString("Generated without a model call.\n\n")
	writeSkillSourceHeader(&b, ev, includeIssue)
	fmt.Fprintf(&b, "- requested_source_sha256_12: `%s`\n", shortDocumentHash(name))
	fmt.Fprintf(&b, "- normalized_source: `%s`\n", inlineCode(normalized))
	fmt.Fprintf(&b, "- skill_source_info_status: `%s`\n", status)
	fmt.Fprintf(&b, "- skill_source_specs: `%d`\n", report.Specs)
	fmt.Fprintf(&b, "- matched_skill_sources: `%d`\n", len(matches))
	fmt.Fprintf(&b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(&b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(&b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(&b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(&b, "- raw_requested_source_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_source_refs_included: `%t`\n", false)
	fmt.Fprintf(&b, "- raw_skill_bodies_included: `%t`\n", false)
	fmt.Fprintf(&b, "- llm_e2e_required_after_skill_source_info_change: `%t`\n", true)
	if includeIssue {
		fmt.Fprintf(&b, "- issue_title_sha256_12: `%s`\n", shortDocumentHash(ev.Issue.Title))
	}
	b.WriteByte('\n')
	b.WriteString("This report focuses one skill source pin. It shows path, source kind, trust level, install mode, expected/current hashes, match state, and risk codes without fetching or printing source bodies.\n\n")

	b.WriteString("### Matches\n")
	writeSkillSourceCards(&b, matches)

	b.WriteString("\n### Risk Findings For Matches\n")
	var findings []SkillSourceRiskFinding
	for _, card := range matches {
		findings = append(findings, card.RiskFindings...)
	}
	sortSkillSourceRiskFindings(findings)
	writeSkillSourceRiskFindings(&b, findings)

	b.WriteString("\n### Info Gates\n")
	fmt.Fprintf(&b, "- skill_source_info_gate=`%s`\n", status)
	b.WriteString("- registry_contact_gate=`disabled`\n")
	b.WriteString("- remote_fetch_gate=`metadata-only-no-fetch`\n")
	b.WriteString("- installer_gate=`disabled`\n")
	b.WriteString("- dependency_install_gate=`disabled`\n")
	b.WriteString("- mutation_gate=`disabled`\n")
	b.WriteString("- raw_body_gate=`hash_only`\n")
	b.WriteString("- model_e2e_gate=`required`\n")

	if len(matches) == 0 {
		b.WriteString("\n### Available Skill Sources\n")
		writeSkillSourceCards(&b, report.Cards)
	}
	return strings.TrimSpace(b.String())
}

func BuildSkillSourceReport(cfg Config, repoContext RepoContext) SkillSourceReport {
	docs := discoverSkillSources(cfg.Workdir)
	cards := summarizeSkillSources(docs, repoContext.SkillSummaries)
	report := SkillSourceReport{
		Status:                          "ok",
		Specs:                           len(cards),
		RegistryContactAllowed:          false,
		InstallerScriptsRun:             false,
		DependencyInstallAllowed:        false,
		RepositoryMutationAllowed:       false,
		RawSourceBodiesIncluded:         false,
		RawSourceRefsIncluded:           false,
		RawSkillBodiesIncluded:          false,
		LLME2ERequiredAfterSourceChange: true,
		Cards:                           cards,
	}
	for _, card := range cards {
		if card.ParseError == "" {
			report.ParsedSpecs++
		}
		if card.SkillMatched {
			report.MatchedSources++
		} else {
			report.MissingSkillMatches++
		}
		if card.HashPinned {
			report.HashPinnedSources++
		}
		if card.HashMatched {
			report.HashMatchedSources++
		}
		if card.HashMismatched {
			report.HashMismatchedSources++
		}
		if card.SourceKind == "repo-local" {
			report.RepoLocalSourceRefs++
		} else {
			report.RemoteSourceRefs++
		}
		if card.RequiresApproval {
			report.SourcesRequiringApproval++
		}
		if card.RemoteFetchAllowed {
			report.RemoteFetchAllowedSpecs++
		}
		if len(card.RiskFindings) > 0 {
			report.SourcesWithRiskFindings++
		}
		report.Findings = append(report.Findings, card.RiskFindings...)
	}
	sortSkillSourceRiskFindings(report.Findings)
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.HighRiskFindings++
		case "warning":
			report.WarningRiskFindings++
		default:
			report.InfoRiskFindings++
		}
	}
	switch {
	case report.HighRiskFindings > 0:
		report.Status = "high"
	case report.WarningRiskFindings > 0:
		report.Status = "warn"
	default:
		report.Status = "ok"
	}
	return report
}

func discoverSkillSources(root string) []skillSourceDocument {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	var docs []skillSourceDocument
	seen := map[string]bool{}
	for _, pattern := range []string{".gitclaw/skill-sources/*.yml", ".gitclaw/skill-sources/*.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(absRoot, filepath.FromSlash(pattern)))
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
			rel, err := filepath.Rel(absRoot, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			body, err := os.ReadFile(match)
			if err != nil {
				continue
			}
			docs = append(docs, parseSkillSourceDocument(rel, string(body)))
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs
}

func parseSkillSourceDocument(path, body string) skillSourceDocument {
	name := normalizeSkillSourceName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	var file struct {
		Name               string `yaml:"name"`
		SkillPath          string `yaml:"skill_path"`
		SourceKind         string `yaml:"source_kind"`
		SourceRef          string `yaml:"source_ref"`
		TrustLevel         string `yaml:"trust_level"`
		InstallMode        string `yaml:"install_mode"`
		ExpectedSHA        string `yaml:"expected_sha256_12"`
		RequiresApproval   bool   `yaml:"requires_approval"`
		RemoteFetchAllowed bool   `yaml:"remote_fetch_allowed"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.KnownFields(true)
	parseError := ""
	if err := decoder.Decode(&file); err != nil {
		parseError = err.Error()
	}
	if value := normalizeSkillSourceName(file.Name); value != "" {
		name = value
	}
	skillPath := filepath.ToSlash(strings.TrimSpace(file.SkillPath))
	if skillPath == "" && name != "" {
		skillPath = ".gitclaw/SKILLS/" + name + "/SKILL.md"
	}
	sourceKind := normalizeSkillSourceKind(file.SourceKind)
	trustLevel := normalizeSkillSourceValue(file.TrustLevel)
	if trustLevel == "" {
		trustLevel = "unknown"
	}
	installMode := normalizeSkillSourceValue(file.InstallMode)
	if installMode == "" {
		installMode = "manual-review"
	}
	return skillSourceDocument{
		Name:               name,
		Path:               path,
		Body:               body,
		SkillPath:          skillPath,
		SourceKind:         sourceKind,
		SourceRef:          strings.TrimSpace(file.SourceRef),
		TrustLevel:         trustLevel,
		InstallMode:        installMode,
		ExpectedSHA:        strings.ToLower(strings.TrimSpace(file.ExpectedSHA)),
		RequiresApproval:   file.RequiresApproval,
		RemoteFetchAllowed: file.RemoteFetchAllowed,
		ParseError:         parseError,
	}
}

func summarizeSkillSources(docs []skillSourceDocument, skills []SkillSummary) []SkillSourceCard {
	cards := make([]SkillSourceCard, 0, len(docs))
	for _, doc := range docs {
		matchedSkill, matched := matchSkillSourceToSkill(doc, skills)
		card := SkillSourceCard{
			Name:               doc.Name,
			Path:               doc.Path,
			SkillPath:          doc.SkillPath,
			SkillMatched:       matched,
			SkillSHA:           matchedSkill.SHA,
			SourceKind:         doc.SourceKind,
			SourceRefPresent:   strings.TrimSpace(doc.SourceRef) != "",
			SourceRefSHA:       shortDocumentHash(doc.SourceRef),
			TrustLevel:         doc.TrustLevel,
			InstallMode:        doc.InstallMode,
			ExpectedSHA:        doc.ExpectedSHA,
			HashPinned:         doc.ExpectedSHA != "",
			RequiresApproval:   doc.RequiresApproval,
			RemoteFetchAllowed: doc.RemoteFetchAllowed,
			Bytes:              len(doc.Body),
			Lines:              lineCount(doc.Body),
			SHA:                shortDocumentHash(doc.Body),
			ParseError:         doc.ParseError,
		}
		if matched && doc.ExpectedSHA != "" {
			card.HashMatched = strings.EqualFold(doc.ExpectedSHA, matchedSkill.SHA)
			card.HashMismatched = !card.HashMatched
		}
		card.RiskFindings = scanSkillSourceRiskFindings(doc, card)
		cards = append(cards, card)
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Path < cards[j].Path })
	return cards
}

func matchSkillSourceToSkill(doc skillSourceDocument, skills []SkillSummary) (SkillSummary, bool) {
	for _, skill := range skills {
		if doc.SkillPath != "" && skill.Path == doc.SkillPath {
			return skill, true
		}
		if doc.Name != "" && strings.EqualFold(skill.Name, doc.Name) {
			return skill, true
		}
	}
	return SkillSummary{}, false
}

func scanSkillSourceRiskFindings(doc skillSourceDocument, card SkillSourceCard) []SkillSourceRiskFinding {
	var findings []SkillSourceRiskFinding
	add := func(severity, code, category, field, value string) {
		findings = append(findings, SkillSourceRiskFinding{
			Severity: severity,
			Code:     code,
			Category: category,
			Name:     doc.Name,
			Path:     doc.Path,
			Field:    field,
			Line:     0,
			LineSHA:  shortDocumentHash(value),
		})
	}
	if strings.TrimSpace(doc.ParseError) != "" {
		add("warning", "skill_source_yaml_parse_error", "skill-source-schema", "yaml", doc.ParseError)
	}
	if doc.SkillPath == "" {
		add("warning", "skill_source_missing_skill_path", "skill-provenance", "skill_path", doc.Path)
	}
	if !card.SkillMatched {
		add("warning", "skill_source_match_missing", "skill-provenance", "skill_path", doc.SkillPath)
	}
	if doc.ExpectedSHA == "" {
		add("warning", "skill_source_missing_expected_hash", "skill-integrity", "expected_sha256_12", doc.Path)
	}
	if card.HashMismatched {
		add("high", "skill_source_hash_mismatch", "skill-integrity", "expected_sha256_12", doc.ExpectedSHA)
	}
	if doc.RemoteFetchAllowed {
		add("high", "skill_source_remote_fetch_allowed", "supply-chain", "remote_fetch_allowed", doc.Path)
	}
	if doc.InstallMode != "manual-review" && doc.InstallMode != "proposal-only" {
		add("warning", "skill_source_install_mode_not_review_only", "supply-chain", "install_mode", doc.InstallMode)
	}
	if !doc.RequiresApproval {
		add("warning", "skill_source_approval_gate_missing", "approval", "requires_approval", doc.Path)
	}
	if !skillSourceKindAllowed(doc.SourceKind) {
		add("warning", "skill_source_kind_untrusted", "supply-chain", "source_kind", doc.SourceKind)
	}
	for _, finding := range scanPluginRiskText("skill-source", doc.Name, doc.Path, "body", doc.Body) {
		findings = append(findings, SkillSourceRiskFinding{
			Severity: finding.Severity,
			Code:     finding.Code,
			Category: finding.Category,
			Name:     doc.Name,
			Path:     doc.Path,
			Field:    finding.Field,
			Line:     finding.Line,
			LineSHA:  finding.LineSHA,
		})
	}
	sortSkillSourceRiskFindings(findings)
	return findings
}

func skillSourceKindAllowed(kind string) bool {
	switch kind {
	case "repo-local", "github", "clawhub", "hermes-hub", "skills-sh", "well-known", "https-url":
		return true
	default:
		return false
	}
}

func normalizeSkillSourceName(value string) string {
	return normalizeSkillBundleName(value)
}

func normalizeSkillSourceValue(value string) string {
	return normalizeSkillBundleName(value)
}

func normalizeSkillSourceKind(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	if strings.HasPrefix(value, "https://") {
		return "https-url"
	}
	if strings.HasPrefix(value, "http://") {
		return "http-url"
	}
	return normalizeSkillSourceValue(value)
}

func matchingSkillSourceCards(cards []SkillSourceCard, name string) []SkillSourceCard {
	name = normalizeSkillSourceName(name)
	if name == "" {
		return nil
	}
	var matches []SkillSourceCard
	for _, card := range cards {
		pathName := normalizeSkillSourceName(strings.TrimSuffix(filepath.Base(card.Path), filepath.Ext(card.Path)))
		if normalizeSkillSourceName(card.Name) == name || pathName == name {
			matches = append(matches, card)
		}
	}
	return matches
}

func writeSkillSourceHeader(b *strings.Builder, ev Event, includeIssue bool) {
	if includeIssue {
		fmt.Fprintf(b, "- repository: `%s`\n", ev.Repo)
		fmt.Fprintf(b, "- issue: `#%d`\n", ev.Issue.Number)
	} else {
		fmt.Fprintf(b, "- scope: `%s`\n", "local-cli")
	}
}

func writeSkillSourceSummary(b *strings.Builder, report SkillSourceReport) {
	fmt.Fprintf(b, "- skill_source_status: `%s`\n", report.Status)
	fmt.Fprintf(b, "- skill_source_specs_dir: `%s`\n", skillSourcesDir)
	fmt.Fprintf(b, "- skill_source_specs: `%d`\n", report.Specs)
	fmt.Fprintf(b, "- parsed_skill_source_specs: `%d`\n", report.ParsedSpecs)
	fmt.Fprintf(b, "- matched_skill_sources: `%d`\n", report.MatchedSources)
	fmt.Fprintf(b, "- missing_skill_source_matches: `%d`\n", report.MissingSkillMatches)
	fmt.Fprintf(b, "- hash_pinned_skill_sources: `%d`\n", report.HashPinnedSources)
	fmt.Fprintf(b, "- hash_matched_skill_sources: `%d`\n", report.HashMatchedSources)
	fmt.Fprintf(b, "- hash_mismatched_skill_sources: `%d`\n", report.HashMismatchedSources)
	fmt.Fprintf(b, "- repo_local_source_refs: `%d`\n", report.RepoLocalSourceRefs)
	fmt.Fprintf(b, "- remote_source_refs: `%d`\n", report.RemoteSourceRefs)
	fmt.Fprintf(b, "- sources_requiring_approval: `%d`\n", report.SourcesRequiringApproval)
	fmt.Fprintf(b, "- remote_fetch_allowed_specs: `%d`\n", report.RemoteFetchAllowedSpecs)
	fmt.Fprintf(b, "- sources_with_risk_findings: `%d`\n", report.SourcesWithRiskFindings)
	fmt.Fprintf(b, "- skill_source_risk_findings: `%d`\n", len(report.Findings))
	fmt.Fprintf(b, "- high_risk_findings: `%d`\n", report.HighRiskFindings)
	fmt.Fprintf(b, "- warning_risk_findings: `%d`\n", report.WarningRiskFindings)
	fmt.Fprintf(b, "- info_risk_findings: `%d`\n", report.InfoRiskFindings)
	fmt.Fprintf(b, "- registry_contact_allowed: `%t`\n", report.RegistryContactAllowed)
	fmt.Fprintf(b, "- installer_scripts_run: `%t`\n", report.InstallerScriptsRun)
	fmt.Fprintf(b, "- dependency_install_allowed: `%t`\n", report.DependencyInstallAllowed)
	fmt.Fprintf(b, "- repository_mutation_allowed: `%t`\n", report.RepositoryMutationAllowed)
	fmt.Fprintf(b, "- raw_source_bodies_included: `%t`\n", report.RawSourceBodiesIncluded)
	fmt.Fprintf(b, "- raw_source_refs_included: `%t`\n", report.RawSourceRefsIncluded)
	fmt.Fprintf(b, "- raw_skill_bodies_included: `%t`\n", report.RawSkillBodiesIncluded)
	fmt.Fprintf(b, "- llm_e2e_required_after_skill_source_change: `%t`\n", report.LLME2ERequiredAfterSourceChange)
}

func writeSkillSourceCards(b *strings.Builder, cards []SkillSourceCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		sourceRefSHA := "none"
		if card.SourceRefPresent {
			sourceRefSHA = card.SourceRefSHA
		}
		currentSHA := "none"
		if card.SkillSHA != "" {
			currentSHA = card.SkillSHA
		}
		expectedSHA := "none"
		if card.ExpectedSHA != "" {
			expectedSHA = card.ExpectedSHA
		}
		fmt.Fprintf(b, "- source_name=`%s` path=`%s` skill_path=`%s` skill_matched=`%t` source_kind=`%s` source_ref_present=`%t` source_ref_sha256_12=`%s` trust_level=`%s` install_mode=`%s` requires_approval=`%t` remote_fetch_allowed=`%t` hash_pinned=`%t` expected_sha256_12=`%s` current_skill_sha256_12=`%s` hash_matched=`%t` bytes=`%d` lines=`%d` sha256_12=`%s` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			inlineCode(card.Name),
			card.Path,
			card.SkillPath,
			card.SkillMatched,
			inlineCode(card.SourceKind),
			card.SourceRefPresent,
			sourceRefSHA,
			inlineCode(card.TrustLevel),
			inlineCode(card.InstallMode),
			card.RequiresApproval,
			card.RemoteFetchAllowed,
			card.HashPinned,
			expectedSHA,
			currentSHA,
			card.HashMatched,
			card.Bytes,
			card.Lines,
			card.SHA,
			len(card.RiskFindings),
			skillSourceRiskMaxSeverity(card.RiskFindings),
			inlineListOrNone(skillSourceRiskCodes(card.RiskFindings)),
		)
	}
}

func writeSkillSourceRiskFindings(b *strings.Builder, findings []SkillSourceRiskFinding) {
	if len(findings) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` source=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			inlineCode(finding.Name),
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func writeSkillSourceSearchResults(b *strings.Builder, results []SkillSourceSearchResult) {
	if len(results) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, result := range results {
		card := result.Card
		sourceRefSHA := "none"
		if card.SourceRefPresent {
			sourceRefSHA = card.SourceRefSHA
		}
		currentSHA := "none"
		if card.SkillSHA != "" {
			currentSHA = card.SkillSHA
		}
		expectedSHA := "none"
		if card.ExpectedSHA != "" {
			expectedSHA = card.ExpectedSHA
		}
		fmt.Fprintf(b, "- source_name=`%s` path=`%s` skill_path=`%s` match_fields=`%s` score=`%d` skill_matched=`%t` source_kind=`%s` source_ref_present=`%t` source_ref_sha256_12=`%s` trust_level=`%s` install_mode=`%s` requires_approval=`%t` remote_fetch_allowed=`%t` hash_pinned=`%t` expected_sha256_12=`%s` current_skill_sha256_12=`%s` hash_matched=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			inlineCode(card.Name),
			card.Path,
			card.SkillPath,
			inlineList(result.MatchFields),
			result.Score,
			card.SkillMatched,
			inlineCode(card.SourceKind),
			card.SourceRefPresent,
			sourceRefSHA,
			inlineCode(card.TrustLevel),
			inlineCode(card.InstallMode),
			card.RequiresApproval,
			card.RemoteFetchAllowed,
			card.HashPinned,
			expectedSHA,
			currentSHA,
			card.HashMatched,
			len(card.RiskFindings),
			skillSourceRiskMaxSeverity(card.RiskFindings),
			inlineListOrNone(skillSourceRiskCodes(card.RiskFindings)),
		)
	}
}

func writeSkillSourceLockCards(b *strings.Builder, cards []SkillSourceCard) {
	if len(cards) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, card := range cards {
		sourceRefSHA := "none"
		if card.SourceRefPresent {
			sourceRefSHA = card.SourceRefSHA
		}
		currentSHA := "none"
		if card.SkillSHA != "" {
			currentSHA = card.SkillSHA
		}
		expectedSHA := "none"
		if card.ExpectedSHA != "" {
			expectedSHA = card.ExpectedSHA
		}
		fmt.Fprintf(b, "- source_name=`%s` path=`%s` skill_path=`%s` lock_state=`%s` skill_matched=`%t` source_kind=`%s` source_ref_present=`%t` source_ref_sha256_12=`%s` trust_level=`%s` install_mode=`%s` requires_approval=`%t` remote_fetch_allowed=`%t` hash_pinned=`%t` expected_sha256_12=`%s` current_skill_sha256_12=`%s` hash_matched=`%t` risk_findings=`%d` risk_max_severity=`%s` risk_codes=`%s`\n",
			inlineCode(card.Name),
			card.Path,
			card.SkillPath,
			skillSourceLockState(card),
			card.SkillMatched,
			inlineCode(card.SourceKind),
			card.SourceRefPresent,
			sourceRefSHA,
			inlineCode(card.TrustLevel),
			inlineCode(card.InstallMode),
			card.RequiresApproval,
			card.RemoteFetchAllowed,
			card.HashPinned,
			expectedSHA,
			currentSHA,
			card.HashMatched,
			len(card.RiskFindings),
			skillSourceRiskMaxSeverity(card.RiskFindings),
			inlineListOrNone(skillSourceRiskCodes(card.RiskFindings)),
		)
	}
}

func writeSkillSourceVerifyFindings(b *strings.Builder, report SkillSourceVerifyReport) {
	b.WriteString("- severity=`info` code=`skill_source_registry_verification_not_configured` detail=`GitClaw v1 verifies reviewed repo-local source pins and hashes without contacting external skill registries`\n")
	b.WriteString("- severity=`info` code=`skill_source_remote_fetch_verification_static_only` detail=`remote source refs are not fetched; verification is limited to reviewed source-pin metadata and local skill hashes`\n")
	b.WriteString("- severity=`info` code=`skill_source_install_verification_disabled` detail=`skill source verification does not run installers or dependency managers`\n")
	for _, finding := range report.Source.Findings {
		fmt.Fprintf(b, "- severity=`%s` code=`%s` category=`%s` source=`%s` path=`%s` field=`%s` line=`%d` line_sha256_12=`%s`\n",
			finding.Severity,
			finding.Code,
			finding.Category,
			inlineCode(finding.Name),
			finding.Path,
			finding.Field,
			finding.Line,
			finding.LineSHA,
		)
	}
}

func skillSourceRiskCodes(findings []SkillSourceRiskFinding) []string {
	var codes []string
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Code != "" && !seen[finding.Code] {
			seen[finding.Code] = true
			codes = append(codes, finding.Code)
		}
	}
	sort.Strings(codes)
	return codes
}

func skillSourceRiskMaxSeverity(findings []SkillSourceRiskFinding) string {
	if len(findings) == 0 {
		return "none"
	}
	max := "info"
	for _, finding := range findings {
		if finding.Severity == "high" {
			return "high"
		}
		if finding.Severity == "warning" {
			max = "warning"
		}
	}
	return max
}

func sortSkillSourceRiskFindings(findings []SkillSourceRiskFinding) {
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity != b.Severity {
			return skillSourceSeverityRank(a.Severity) < skillSourceSeverityRank(b.Severity)
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.Line < b.Line
	})
}

func skillSourceSearchScore(card SkillSourceCard, query string, terms []string) (int, []string) {
	fields := map[string]string{
		"name":              card.Name,
		"path":              card.Path,
		"skill_path":        card.SkillPath,
		"source_kind":       card.SourceKind,
		"trust_level":       card.TrustLevel,
		"install_mode":      card.InstallMode,
		"expected_sha":      card.ExpectedSHA,
		"current_skill_sha": card.SkillSHA,
		"risk_codes":        strings.Join(skillSourceRiskCodes(card.RiskFindings), " "),
	}
	weights := map[string]int{
		"name":              80,
		"path":              35,
		"skill_path":        40,
		"source_kind":       55,
		"trust_level":       45,
		"install_mode":      45,
		"expected_sha":      8,
		"current_skill_sha": 8,
		"risk_codes":        50,
	}
	return scoreSearchFields(fields, weights, strings.ToLower(cleanSkillSearchQuery(query)), terms)
}

func skillSourceLockState(card SkillSourceCard) string {
	switch {
	case !card.SkillMatched:
		return "missing-skill"
	case !card.HashPinned:
		return "unpinned"
	case card.HashMismatched:
		return "stale"
	case card.RemoteFetchAllowed:
		return "remote-fetch-allowed"
	case len(card.RiskFindings) > 0:
		return "review-required"
	default:
		return "locked"
	}
}

func skillSourceLockManifestHash(cards []SkillSourceCard) string {
	if len(cards) == 0 {
		return shortDocumentHash("gitclaw-skill-source-lock-empty")
	}
	var lines []string
	for _, card := range cards {
		lines = append(lines, strings.Join([]string{
			card.Name,
			card.Path,
			card.SkillPath,
			card.SourceKind,
			card.TrustLevel,
			card.InstallMode,
			boolString(card.SkillMatched),
			boolString(card.SourceRefPresent),
			card.SourceRefSHA,
			boolString(card.RequiresApproval),
			boolString(card.RemoteFetchAllowed),
			boolString(card.HashPinned),
			card.ExpectedSHA,
			card.SkillSHA,
			boolString(card.HashMatched),
			boolString(card.HashMismatched),
			skillSourceLockState(card),
			strings.Join(skillSourceRiskCodes(card.RiskFindings), ","),
		}, "\t"))
	}
	sort.Strings(lines)
	return shortDocumentHash(strings.Join(lines, "\n"))
}

func inspectExternalClawHubLock(root string) (bool, string) {
	if root == "" {
		root = "."
	}
	path := filepath.Join(root, filepath.FromSlash(externalClawHubLockPath))
	body, err := os.ReadFile(path)
	if err != nil {
		return false, ""
	}
	return true, shortDocumentHash(string(body))
}

func noneIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func skillSourceSeverityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func isSkillSourcesListRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 2 &&
		fields[0] == "/skills" &&
		(strings.EqualFold(fields[1], "sources") || strings.EqualFold(fields[1], "source")) &&
		(len(fields) == 2 || strings.EqualFold(fields[2], "list"))
}

func isSkillSourcesRiskRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 &&
		fields[0] == "/skills" &&
		(strings.EqualFold(fields[1], "sources") || strings.EqualFold(fields[1], "source")) &&
		(strings.EqualFold(fields[2], "risk") || strings.EqualFold(fields[2], "risk-audit"))
}

func isSkillSourcesVerifyRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 &&
		fields[0] == "/skills" &&
		(strings.EqualFold(fields[1], "sources") || strings.EqualFold(fields[1], "source")) &&
		(strings.EqualFold(fields[2], "verify") || strings.EqualFold(fields[2], "check"))
}

func isSkillSourcesLockRequest(ev Event, cfg Config) bool {
	fields := activeSlashCommandFields(ev, cfg)
	return len(fields) >= 3 &&
		fields[0] == "/skills" &&
		(strings.EqualFold(fields[1], "sources") || strings.EqualFold(fields[1], "source")) &&
		(strings.EqualFold(fields[2], "lock") || strings.EqualFold(fields[2], "lockfile"))
}

func requestedSkillSourceSearchQuery(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 4 {
		return ""
	}
	if fields[0] != "/skills" {
		return ""
	}
	if !strings.EqualFold(fields[1], "sources") && !strings.EqualFold(fields[1], "source") {
		return ""
	}
	if !strings.EqualFold(fields[2], "search") && !strings.EqualFold(fields[2], "find") {
		return ""
	}
	return cleanSkillSearchQuery(strings.Join(fields[3:], " "))
}

func requestedSkillSourceInfoName(ev Event, cfg Config) string {
	fields := activeSlashCommandFields(ev, cfg)
	if len(fields) < 4 {
		return ""
	}
	if fields[0] != "/skills" {
		return ""
	}
	if !strings.EqualFold(fields[1], "sources") && !strings.EqualFold(fields[1], "source") {
		return ""
	}
	if !strings.EqualFold(fields[2], "info") && !strings.EqualFold(fields[2], "show") {
		return ""
	}
	return normalizeSkillSourceName(fields[3])
}
