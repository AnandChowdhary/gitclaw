# GitClaw Nodes

Node policy verification token: `GITCLAW_NODES_CONTEXT_V1`.

GitClaw v1 has one execution node class: ephemeral GitHub Actions jobs. Node
files describe reviewed runtime and worker intent; they do not pair devices,
open WebSockets, install services, or expose host capabilities.

Allowed node behavior:

- Use GitHub Actions jobs as the active execution node.
- Use scheduled workflows and `workflow_dispatch` as GitHub-native wake paths.
- Keep node specs in git for human review before automation relies on them.
- Hash node metadata in reports instead of printing runtime payloads.

Disallowed node behavior:

- Starting headless node hosts, gateway WebSocket clients, remote shell hosts,
  or long-running node services from an issue/comment turn.
- Pairing devices or auto-approving node scopes from untrusted issue text.
- Exposing camera, screen, location, SMS, notification, browser-proxy, or host
  exec capabilities without reviewed workflow code and explicit approval gates.
- Printing raw node spec bodies, issue bodies, comment bodies, or provider
  payloads in deterministic node reports.
