# api-core

Production source of truth for the customized sub2 core API and admin UI.

## Runtime

- Container: `sub2api`
- Internal port: `127.0.0.1:8080`
- Public entry: Nginx reverse proxy
- Data: cloud PostgreSQL and cloud Redis

## Build

```bash
docker build -t gaoge-sub2api:niubi999-v1 .
```

## Deploy

```bash
bash deploy/scripts/deploy-api-core.sh
```

## Health Check

```bash
curl -fsS http://127.0.0.1:8080/health
```

## Patch Docs

All Gaoge custom patches must be documented in `GAOGE_PATCHES/`.

## Rollback

Restore `apps/api-core` from the previous Git tag and redeploy only `api-core`.

