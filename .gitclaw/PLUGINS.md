# GitClaw Plugins

Plugin policy verification token: `GITCLAW_PLUGINS_CONTEXT_V1`.

GitClaw plugins are declarative, repo-reviewed capability records in v1. They
describe integration intent, ownership, and safety gates, but GitClaw does not
install, import, execute, or connect plugin runtimes during an assistant turn.

Allowed plugin behavior:

- Declare capability metadata that future reviewed workflows may implement.
- Require explicit approval before external side effects or new tool exposure.
- Prefer existing deterministic GitClaw tools and workflow-dispatch boundaries.
- Keep plugin config, credentials, manifests, and provider payloads out of
  issue-visible diagnostics.

Disallowed plugin behavior:

- Installing ClawHub, npm, pip, git, archive, or MCP packages automatically.
- Starting plugin-owned servers, webhooks, channel bridges, or MCP connections
  from an issue/comment turn.
- Exposing new model-visible tools without reviewed tool policy and approval.
- Treating untrusted issue/comment text as plugin configuration or code.

MCP specs live in `.gitclaw/mcp/*.yaml` and are metadata-only in v1. They may
declare reviewed server intent, transport, source, tool filters, and secret-name
references, but GitClaw must not launch MCP servers, connect clients, expose MCP
tools to the model, pass raw env, or print command/URL/arg bodies.
