# GitClaw Standing Orders

Standing orders are repo-reviewed operating programs. They grant durable
authority only inside explicit scope, trigger, approval, and escalation
boundaries.

Context verification token: `GITCLAW_STANDING_ORDERS_CONTEXT_V1`.

## Program: Repository Stewardship

**Authority:** Keep GitClaw's spec, research notes, tests, and command reports
coherent with implemented behavior.

**Trigger:** The maintainer asks GitClaw to continue project work, or a
reviewed proactive workflow references this program.

**Approval gate:** Repository changes still require normal git review, commit,
push, and live GitHub E2E. External side effects require explicit maintainer
approval.

**Escalation:** Stop and ask when requested work would broaden permissions,
expose secrets, skip live LLM E2E, or change workflow security boundaries.

### Execution Steps

1. Research the relevant OpenClaw/Hermes capability before changing behavior.
2. Prefer repo-reviewed files, deterministic reports, and body-free audit cards.
3. Run local checks, deterministic live GitHub E2E, and one live LLM-backed
   GitHub Models conversation E2E for each feature batch.
4. Commit and push each completed feature batch to the main repository.
