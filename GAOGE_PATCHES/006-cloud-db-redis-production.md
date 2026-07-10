# 006 Cloud DB Redis Production

## Purpose

Ensure production sub2 writes to cloud PostgreSQL and cloud Redis, not local Docker database containers.

## Required Production Values

- `DATABASE_HOST` must be the cloud PostgreSQL endpoint.
- `REDIS_HOST` must be the cloud Redis endpoint.
- Production compose must not default to local `postgres` or `redis` service names.

## Test

```bash
bash deploy/scripts/verify-production-config.sh
```

Confirm the running container environment points to cloud PostgreSQL and cloud Redis.

