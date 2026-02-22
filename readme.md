Qantlo

High-concurrency agnostic microservice for atomic resource metering and real-time balancing.

Qantlo is a specialized ledger engine designed to handle massive streams of consumption requests. It provides atomic "spend" operations, idempotent transaction processing, and precise balance tracking for any digital or physical unit (API credits, GPU-hours, traffic, or even milliliters).

✨ Key Features
⚡ High Concurrency: Engineered to handle thousands of simultaneous decrement requests to the same account balance without race conditions.

💎 Atomic Operations: Ensures data integrity using ACID transactions or atomic scripts (Redis/Lua), preventing "double-spending" or negative balances.

🔗 Agnostic Metering: Works with any units. Define your own resource types: tokens, liters, seconds, requests, etc.

🛡️ Idempotency Built-in: Native support for Idempotency-Key to safely retry transactions in unstable network conditions.

📊 Real-time Analytics: Instant access to current balances and historical usage reports.

🚀 Quick Start

1. Deposit Tokens (Top-up)
Add units to a specific account.

Bash
curl -X POST <https://api.qantlo.io/v1/deposit> \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user_42",
    "resource_type": "api_credits",
    "amount": 1000,
    "metadata": { "reason": "subscription_refill" }
  }'
2. Spend Tokens (The Hot Path)
The main endpoint for high-frequency consumption.

Bash
curl -X POST <https://api.qantlo.io/v1/spend> \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: uuid-string-123" \
  -d '{
    "account_id": "user_42",
    "resource_type": "api_credits",
    "amount": 1,
    "metadata": { "service": "llm-inference" }
  }'
3. Check Balance
Get the current state of a specific resource.

Bash
curl <https://api.qantlo.io/v1/balance/user_42/api_credits>
🛠 Architecture Overview
Qantlo is designed to be a "middleman" between your business logic and your data persistence.

API Layer: Fast HTTP/gRPC interface.

Concurrency Control: Uses distributed locking or atomic counters to manage hot-keys (popular accounts).

Persistence: Records every transaction into a relational database (PostgreSQL) or a specialized OLAP store (ClickHouse) for heavy reporting.

📈 Use Cases
SaaS Billing: Track API usage and enforce rate limits based on pre-paid credits.

Cloud Infrastructure: Metering CPU/RAM seconds for serverless functions.

IoT Platforms: Monitoring resource consumption (water, electricity, data) across millions of devices.

Gaming: Managing in-game currencies and energy points.
