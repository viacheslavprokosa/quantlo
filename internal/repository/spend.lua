-- KEYS[1] = Balance key (e.g., "balance:user123:api_tokens")
-- KEYS[2] = Idempotency key (e.g., "idem:req-uuid-456")
-- ARGV[1] = Deduction amount (e.g., 10)

-- 1. Check idempotency. If this request has already been processed, return status 0
if redis.call("EXISTS", KEYS[2]) == 1 then
    return {0, "ALREADY_PROCESSED"}
end

-- 2. Get the current balance
local current_balance = redis.call("GET", KEYS[1])
if not current_balance then
    return {-1, "BALANCE_NOT_FOUND"}
end

current_balance = tonumber(current_balance)
local deduct_amount = tonumber(ARGV[1])

-- 3. Check if there are enough tokens on the balance
if current_balance < deduct_amount then
    return {-2, "INSUFFICIENT_FUNDS"}
end

-- 4. Success! Deduct funds
local new_balance = redis.call("DECRBY", KEYS[1], deduct_amount)

-- 5. Store the idempotency key for 24 hours (86400 seconds) to prevent duplicates
redis.call("SET", KEYS[2], "1", "EX", 86400)

-- Return 1 (success) and the new balance
return {1, new_balance}