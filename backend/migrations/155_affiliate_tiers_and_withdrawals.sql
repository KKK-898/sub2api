-- Dynamic affiliate rebate tiers and cash-withdrawal workflow.
INSERT INTO settings (key, value, updated_at)
VALUES
    ('affiliate_rebate_tier_0_5', '5', NOW()),
    ('affiliate_rebate_tier_6_10', '10', NOW()),
    ('affiliate_rebate_tier_11_20', '15', NOW()),
    ('affiliate_rebate_tier_21_plus', '20', NOW())
ON CONFLICT (key) DO NOTHING;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS rebate_rate_percent DECIMAL(5,2) NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS active_invitee_count INTEGER NULL;

COMMENT ON COLUMN user_affiliate_ledger.rebate_rate_percent IS 'Rebate percentage snapshotted when the reward was accrued';
COMMENT ON COLUMN user_affiliate_ledger.active_invitee_count IS 'Active invitee count snapshotted when the reward was accrued';

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS inviter_bound_at TIMESTAMPTZ NULL;

UPDATE user_affiliates
SET inviter_bound_at = created_at
WHERE inviter_id IS NOT NULL
  AND inviter_bound_at IS NULL;

COMMENT ON COLUMN user_affiliates.inviter_bound_at IS 'Immutable timestamp when this user bound an inviter';

CREATE TABLE IF NOT EXISTS affiliate_withdrawals (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    quota_amount DECIMAL(20,8) NOT NULL CHECK (quota_amount > 0),
    cash_rate DECIMAL(20,8) NOT NULL CHECK (cash_rate > 0),
    cash_amount DECIMAL(20,8) NOT NULL CHECK (cash_amount > 0),
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'completed', 'cancelled')),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    processed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_affiliate_withdrawals_one_pending_per_user
    ON affiliate_withdrawals(user_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_affiliate_withdrawals_status_requested
    ON affiliate_withdrawals(status, requested_at DESC);

CREATE INDEX IF NOT EXISTS idx_affiliate_withdrawals_user_requested
    ON affiliate_withdrawals(user_id, requested_at DESC);

COMMENT ON TABLE affiliate_withdrawals IS 'Affiliate reward cash withdrawal requests';
COMMENT ON COLUMN affiliate_withdrawals.quota_amount IS 'Reward quota reserved by this request';
COMMENT ON COLUMN affiliate_withdrawals.cash_rate IS 'CNY cash amount per one quota unit, snapshotted at request time';
COMMENT ON COLUMN affiliate_withdrawals.cash_amount IS 'CNY cash amount snapshotted at request time';
