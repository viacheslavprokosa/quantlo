-- Current balances table.
-- Composite primary key ensures only one record per user and resource type.
CREATE TABLE balances (
    account_id VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    -- Use BIGINT to avoid float math issues.
    amount BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (account_id, resource_type)
);

-- Transaction history table
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    -- amount can be positive (replenishment) or negative (deduction)
    amount BIGINT NOT NULL, 
    
    -- CRITICALLY IMPORTANT: Unique index on idempotency key
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    
    -- JSONB is ideal for storing flexible metadata (e.g., {"ip": "1.1.1.1", "service": "llm"})
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast report generation for a specific user and resource
CREATE INDEX idx_tx_account_resource ON transactions(account_id, resource_type);