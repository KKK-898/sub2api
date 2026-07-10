# 005 Desktop Domain Management

## Purpose

Support dynamic desktop client domain management so primary, backup, and temporary domains can be replaced without hardcoding them into the client.

## Key Behavior

- Domain management is data/config driven.
- Client fallback domains should be replaceable from backend management.
- Future v2 work will move all server domain references into global variables/templates.

## Test

- Update domain config in backend.
- Confirm desktop client receives the updated primary/backup/temp domain list.

