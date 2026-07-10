# 007 Internal Backend Allowlist

## Purpose

Allow sub2 nodes to call software backend internal APIs while keeping those internal endpoints blocked from public access.

## Key Behavior

- Internal backend paths allow loopback and known server node IPs.
- Public access is denied.
- Backend URL failover order should prefer healthy domains.

## Test

- From each server node, call the internal subscription endpoint and expect 200.
- From public internet, direct internal endpoint access should be denied.

