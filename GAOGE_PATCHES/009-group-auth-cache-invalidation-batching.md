# 009 Group Auth Cache Invalidation Batching

## Purpose

Keep group administration responsive when a group contains thousands of API keys and authentication snapshots are stored in shared Redis.

## Key Behavior

- Group and user cache invalidation still clears the current node's L1 entries immediately.
- L2 deletes and per-key cross-node invalidation publications are sent through Redis pipelines in batches of 256.
- Multi-node cache consistency is preserved; every affected key is still published to the existing invalidation channel.
- If the optional batch path fails, the service falls back to the existing per-key invalidation behavior.
- The OpenAI image-tool strip control is rendered as a single horizontal switch in create and edit forms.

## Test

- Verify a batch-capable cache receives one bulk call instead of per-key deletes.
- Verify Redis removes every requested auth snapshot and publishes every matching invalidation message.
- Verify the non-batch cache fallback remains compatible.
- Verify the OpenAI-only switch submits `strip_codex_image_generation_tool=true` and no checkbox is rendered.
