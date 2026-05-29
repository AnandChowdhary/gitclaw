# GitClaw Backups

`gitclaw backup` writes canonical issue transcript backups here by default:

```text
.gitclaw/backups/<owner>__<repo>/issues/<issue-number>.json
```

Backups are normal repository files. Projects can choose whether to commit
them on the default branch, a dedicated backup branch, or in a private mirror.
