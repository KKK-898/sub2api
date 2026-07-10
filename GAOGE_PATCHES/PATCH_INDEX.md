# Gaoge api-core Patch Index

This directory records all Gaoge custom patches applied on top of official sub2.

## Patches

- `001-platform-subscription-billing.md`: platform subscription billing and backend rule lookup.
- `002-balance-overdraft-protection.md`: balance overdraft and excessive negative balance protection.
- `003-payment-bridge-integration.md`: payment bridge and card auto-redeem integration.
- `004-usage-bandwidth-control.md`: usage detail bandwidth controls and dashboard refresh behavior.
- `005-desktop-domain-management.md`: desktop client domain management support.
- `006-cloud-db-redis-production.md`: cloud PostgreSQL and Redis production configuration.
- `007-internal-backend-allowlist.md`: backend internal API allowlist and failover domains.
- `008-openai-group-image-tool-stripping.md`: group-scoped Codex/Responses image-tool stripping and image-gate ordering.
- `009-group-auth-cache-invalidation-batching.md`: batched Redis invalidation for large-group admin updates.
- `010-image-tool-strip-gate-precedence.md`: evaluates image permissions after effective image-tool stripping across HTTP, passthrough, and WebSocket flows.
- `011-custom-build-self-update-guard.md`: blocks official in-place update and rollback paths in custom images.

## Update Rule

When official sub2 is updated in `workbench/sub2-upstream-sync`, each patch above must be checked and updated before syncing back to `apps/api-core`.
