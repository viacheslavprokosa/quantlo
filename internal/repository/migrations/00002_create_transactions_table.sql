-- +goose Up
CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      VARCHAR(255) NOT NULL,
    resource_type   VARCHAR(50)  NOT NULL,
    amount          BIGINT       NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_tx_account_resource ON transactions(account_id, resource_type);

-- +goose Down
DROP TABLE transactions;