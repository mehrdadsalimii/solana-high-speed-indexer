CREATE TABLE IF NOT EXISTS transactions (
    signature TEXT PRIMARY KEY,
    slot BIGINT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_slot ON transactions (slot DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_observed_at ON transactions (observed_at DESC);
