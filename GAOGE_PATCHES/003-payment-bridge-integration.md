# 003 Payment Bridge Integration

## Purpose

Support payment bridge auto-redeem flow and keep sub2 balance synchronized with software online payments.

## Key Behavior

- Payment bridge writes order and redemption state to cloud PostgreSQL.
- sub2 balance should reflect redeemed card value.
- Payment success should not require users to manually copy card codes.

## Test

- Create a test order through payment bridge.
- Mark order paid or use provider test flow.
- Confirm card is redeemed and balance changes in sub2.

