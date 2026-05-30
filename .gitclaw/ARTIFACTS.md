# GitClaw Artifacts

GITCLAW_ARTIFACTS_CONTEXT_V1

Artifacts are optional evidence bundles produced by reviewed GitHub Actions
workflows. They can make prompt construction, diagnostics, backups, and future
replay paths auditable without turning GitHub issue comments into body dumps.

## Policy

- Store bounded evidence in GitHub Actions artifacts when a reviewed workflow,
  label, or command explicitly enables it.
- Keep issue comments body-safe: comments may include artifact metadata, names,
  hashes, run links, retention settings, and verification findings, but not raw
  artifact contents.
- Redact prompt artifacts before upload and keep prompt artifact upload disabled
  by default.
- Use the git-backed backup branch for durable transcript backups. Use Actions
  artifacts for short-lived run evidence.
- Treat artifacts as review material, not hidden state. Future turns must rely on
  issue state, repo files, explicit context references, or reviewed backups
  unless a user explicitly points the agent at an artifact.

## Disallowed

- Do not print raw prompt, model, tool, backup, transcript, channel, secret, or
  artifact bodies into issue comments.
- Do not upload unredacted secrets or provider credentials.
- Do not enable prompt artifacts globally by default.
- Do not use artifacts to bypass approval gates, write-intent detection, or the
  GitHub issue conversation boundary.
