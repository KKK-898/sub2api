# 012 Codex Model Manifest Token Selection

## Ownership

This is a Gaoge compatibility patch maintained on top of official Sub2API. It is not an official Wei-Shaw/Sub2API feature or fix.

## Purpose

Prevent Codex model-list refreshes from selecting an OpenAI API-key account that cannot authenticate to the ChatGPT Codex models endpoint.

## Root Cause

- Official Sub2API uses the generic OpenAI account selector for the live Codex manifest route.
- Mixed groups can contain both OAuth and API-key accounts.
- API-key accounts are valid for normal OpenAI API requests but do not have the OAuth access token required by `chatgpt.com/backend-api/codex/models`.
- Selecting one therefore returns `OPENAI_CODEX_MODELS_TOKEN_MISSING` as HTTP 502.

## Key Behavior

- Only the Codex model-manifest routes use the Gaoge token-aware selector.
- Schedulable candidates without a usable Codex OAuth access token are excluded and selection continues.
- The selected child/shadow account is preserved for proxy routing while its parent supplies the OAuth credential.
- If no token-capable account is available, the handler returns the existing HTTP 503 no-account response.
- Normal Responses, Chat Completions, WebSocket, and message account selection are unchanged.

## Test

- Verify a higher-priority API-key account is skipped and a lower-priority OAuth account is selected.
- Verify an API-key-only pool produces a no-available-account error instead of reaching the manifest fetch with a missing token.
- Re-run the existing manifest passthrough, cache, upstream-error, and missing-token tests.

## Upstream Update Rule

Re-check this patch whenever official code changes the Codex models handler, manifest service, account selector, or shadow-account credential resolution.
