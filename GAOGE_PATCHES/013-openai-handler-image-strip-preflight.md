# 013 OpenAI Handler Image Strip Preflight

## Purpose

Allow a text-only OpenAI group to reach the service-layer image-tool stripping policy instead of being rejected by an earlier Handler preflight check.

## Root Cause

- Patch 010 fixed the service-layer permission gate to classify the stripped request.
- The OpenAI HTTP Handler and WebSocket first-frame Handler still classified the original request before service mutation.
- With `allow_image_generation=false`, any advertised image tool was therefore rejected with HTTP 403 or a WebSocket policy close before stripping could run.
- The generic Responses scheduler path also marked the raw request as image generation, causing unnecessary image-account routing even when the group would strip the tool.

## Key Behavior

- Handler preflight uses `IsImageGenerationIntentForGroupGate`.
- For an OpenAI group with `strip_codex_image_generation_tool=true`, tool declarations, Responses Lite `additional_tools`, and image-specific `tool_choice` values do not count as image intent at Handler preflight.
- The service layer still removes those fields before forwarding upstream.
- Explicit image models and dedicated image endpoints remain image intent and are locally blocked when `allow_image_generation=false`.
- HTTP permission checks, HTTP image-concurrency classification, generic Responses scheduler intent, and WebSocket first-frame permission checks now use the same group-aware rule.
- Groups without the strip flag and non-OpenAI platforms preserve their previous behavior.

## Production Evidence

- A group update to `allow_image_generation=false` and `strip_codex_image_generation_tool=true` caused immediate 403 responses before any upstream call.
- Restoring `allow_image_generation=true` stopped the Handler rejection, proving that the early gate ran before service-layer stripping.
- A controlled real-key baseline with stripping disabled returned text for `tool_choice=auto` and generated one image for an explicit image choice.

## Test

- Verify group-aware classification ignores flat, namespace, and `additional_tools` image declarations for a strip-enabled OpenAI group.
- Verify image models and dedicated image endpoints remain classified as image generation.
- Verify the HTTP Handler neither returns 403 nor consumes the image concurrency slot for a strippable tool request.
- Verify the HTTP Handler still returns 403 for an image model in the same group.
- Re-run service HTTP, passthrough, and WebSocket strip regression tests.
