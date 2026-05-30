---
name: prompt-artifact
kind: prompt
storage: github-actions-artifact
filename: prompt.md
workflow: .github/workflows/gitclaw.yml
label: gitclaw:e2e-prompt-artifact
retention_days: 7
redaction_required: true
requires_approval: true
---

# Prompt Artifact

This declarative artifact record describes the opt-in redacted prompt artifact
used by live GitClaw E2E tests. The artifact report may expose this card's
frontmatter-derived metadata, file size, line count, and hash, but it must not
print this body text or any uploaded prompt body.
