ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS platform_subscription_action VARCHAR(20),
    ADD COLUMN IF NOT EXISTS platform_subscription_rule_id TEXT,
    ADD COLUMN IF NOT EXISTS platform_subscription_grant_id TEXT,
    ADD COLUMN IF NOT EXISTS platform_subscription_grant_date TEXT;

CREATE INDEX IF NOT EXISTS idx_usage_logs_platform_sub_action_user_created
    ON usage_logs (platform_subscription_action, user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_usage_logs_platform_sub_grant
    ON usage_logs (platform_subscription_grant_id);
