# 008 OpenAI Group Image Tool Stripping

## Purpose

Prevent ordinary Codex/Responses chats from being rejected when a client advertises image-generation tools in an OpenAI group that does not allow image generation.

## Configuration

- Text-only OpenAI group: `allow_image_generation=false` and `strip_codex_image_generation_tool=true`.
- Image-enabled OpenAI group: `allow_image_generation=true` and `strip_codex_image_generation_tool=false`.
- Shared accounts should use the account-level `inherit` or `disable injection` mode. Account-level `block` still applies across every group using that account.

## Key Behavior

- The strip flag is read from the API key's currently matched group. Other groups are unaffected.
- Group stripping or account-level blocking wins over account, channel, and global image-tool injection.
- The gateway removes top-level `image_generation`, the `image_gen` namespace form, Responses Lite `input.additional_tools`, and an image-specific `tool_choice` while preserving unrelated tools.
- Optional image tools are stripped before the group image gate is evaluated, so a normal text request does not produce a false 403.
- Explicit image endpoints, image models, and an image-specific `tool_choice` remain blocked when `allow_image_generation=false`.
- Natural-language image requests with only an automatic optional tool cannot be reliably classified as explicit intent; after stripping they continue as text and cannot generate an image.
- Standard HTTP, Responses-to-ChatCompletions fallback, passthrough, and Responses WebSocket ingress must preserve the same ordering.

## Compatibility and Update Notes

- The database field defaults to `false`, preserving existing groups after migration.
- The field must remain in the group DTOs, repository mapping, and API-key authentication cache snapshot.
- When merging official sub2 updates, re-check account-level Codex image policy changes and every early-return forwarding path before syncing to `apps/api-core`.

## Test

- In a text-only group with stripping enabled, send a normal chat that advertises image tools and expect a successful upstream text request with those image tools removed.
- Verify unrelated function tools remain unchanged.
- Send an explicit image model or image-specific `tool_choice` and expect a local 403 with no upstream request.
- In an image-enabled group with stripping disabled, verify image generation remains available.
- Repeat the strip test through standard HTTP, passthrough/fallback, and WebSocket ingress.
