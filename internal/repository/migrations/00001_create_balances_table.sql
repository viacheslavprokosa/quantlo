-- +goose Up
CREATE TABLE balances (
    account_id    VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50)  NOT NULL,
    amount        BIGINT       NOT NULL DEFAULT 0,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (account_id, resource_type)
);

-- +goose Down
DROP TABLE balances;