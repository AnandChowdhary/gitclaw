---
name: github-models-provider
kind: provider
source: repo-reviewed
activation: metadata-only
capabilities:
  - model:github-models
  - model-fallback:openai/gpt-4.1-nano
  - tool:search_files
optional_capabilities:
  - mcp:github
requires_approval: true
---

# GitHub Models Provider

This declarative plugin record documents the intended provider/tool boundary
for GitHub Models and related GitHub integrations. It is metadata only;
GitClaw does not install plugin packages or connect MCP servers in v1.

The primary model is `openai/gpt-5-nano`; `openai/gpt-4.1-nano` is a
repo-reviewed fallback for retryable hosted-provider failures.
