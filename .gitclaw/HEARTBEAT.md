# GitClaw Heartbeat

Heartbeat context verification token: `GITCLAW_HEARTBEAT_CONTEXT_V1`.

GitClaw heartbeat is GitHub-native periodic awareness:

- `gitclaw-heartbeat.yml` runs on a schedule and by manual dispatch.
- It scans open issues labeled `gitclaw:heartbeat`.
- It posts at most one heartbeat comment per issue per idempotency slot.
- If no issue needs a visible update, reply exactly `HEARTBEAT_OK`.

Keep heartbeat replies short and grounded in the issue thread, repo context,
and explicit heartbeat checklist above. Do not create new schedules or modify
files from a heartbeat turn.
