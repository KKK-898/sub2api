# 001 Platform Subscription Billing

## Purpose

Connect sub2 API usage to the software platform subscription rules, so users can be billed by their platform subscription status and rule set.

## Key Behavior

- sub2 checks backend subscription rules before billing.
- Backend rule lookup supports multiple backend URLs for failover.
- Failure behavior must be reviewed carefully during every upstream merge.

## Config

- `PLATFORM_SUBSCRIPTION_BILLING_BACKEND_URL`
- Cloud PostgreSQL and Redis settings from production env.

## Test

- Call a model with a user covered by platform subscription rules.
- Confirm usage is billed according to platform rules.
- Confirm backend rule endpoint returns 200 from both Japan and US nodes.

