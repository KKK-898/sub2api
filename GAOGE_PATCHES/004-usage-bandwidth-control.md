# 004 Usage Bandwidth Control

## Purpose

Reduce bandwidth pressure from old clients refreshing heavy usage detail endpoints.

## Key Behavior

- Dashboard summary refresh remains available.
- Heavy usage detail refresh can be rejected or rate-limited.
- gzip support and smaller responses should be preserved.

## Test

- Request dashboard stats and confirm success.
- Request heavy usage details with old-client behavior and confirm expected rejection or reduced traffic.

