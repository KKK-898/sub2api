# 002 Balance Overdraft Protection

## Purpose

Prevent users from continuing to consume after balance is too low, avoiding large negative balances.

## Key Behavior

- Small in-flight overdraft can still happen on a single request.
- Large repeated overdraft must be blocked.
- Balance checks must use the cloud database state.

## Test

- Use an account with low balance.
- Send requests until the balance approaches zero.
- Confirm follow-up requests are rejected before large negative balance appears.

