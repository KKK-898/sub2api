# 011 Custom Build Self-Update Guard

## Purpose

Prevent a Gaoge custom image from replacing its patched binary with an official Wei-Shaw release through the admin update or rollback APIs.

## Key Behavior

- Custom Docker and GoReleaser builds set `BuildType=custom`.
- Version information returns `self_update_enabled=false` without querying the official release API.
- Binary update, local binary rollback, version-list lookup, and version-specific rollback return HTTP 403 with reason `SELF_UPDATE_DISABLED`.
- The admin UI displays that the custom image is managed through the internal release process and does not render official update, rollback, release, or Docker commands.
- The runtime version remains the pure official baseline, currently `0.1.152`; customization stays in build metadata and the image tag.

## Test

- Call the update and rollback endpoints directly and confirm HTTP 403 with `SELF_UPDATE_DISABLED`.
- Confirm a custom build reports `build_type=custom`, `self_update_enabled=false`, and `has_update=false`.
- Confirm the version dropdown contains only the internal-release notice.
- Build with the custom Dockerfile and confirm the runtime reports version `0.1.152`.
