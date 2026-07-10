# 010 Image Tool Strip Gate Precedence

## Purpose

Prevent a text-only OpenAI group from returning a false 403 when a client automatically sends an image-specific `tool_choice` together with its advertised image tool.

## Root Cause

- The gateway recorded explicit image intent before applying the effective account/group strip policy.
- The tool and `tool_choice` were then removed, but the pre-strip intent flag was retained.
- The group image gate evaluated that stale flag and returned `Image generation is not enabled for this group` without contacting upstream.

## Key Behavior

- The effective strip policy runs before the group image permission gate.
- The permission gate classifies the mutated request after image tools, Responses Lite `additional_tools`, and matching `tool_choice` values are removed.
- A client-advertised or client-selected image tool therefore becomes a normal text request in a group configured with `allow_image_generation=false` and `strip_codex_image_generation_tool=true`.
- Dedicated image endpoints and explicit image models remain hard-blocked locally with HTTP 403 and never reach upstream.
- Standard Responses HTTP, OpenAI passthrough, and Responses WebSocket ingress use the same post-strip classification rule.
- Handler preflight, image-concurrency classification, and scheduler intent use the group-aware effective request view described by patch 013.
- The behavior remains scoped to the API key's matched OpenAI group; groups without the strip flag and other platforms are unchanged.

## Compatibility Note

Some clients use the same image tool and `tool_choice` payload for ordinary chat and for a user-requested image. The server cannot reliably distinguish those cases. In a strip-enabled group both become text-only requests: no image is generated and no false 403 is returned.

## Test

- Verify standard HTTP removes an explicit `image_gen` namespace choice and forwards the remaining text/function tools.
- Verify OpenAI passthrough removes flat and namespace image tools plus the image-specific `tool_choice`.
- Verify WebSocket ingress forwards the stripped payload without a policy close.
- Verify an explicit image model remains locally blocked and the upstream recorder receives no request.
