# Quantlo

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue.svg)](https://go.dev)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/viacheslavprokosa/quantlo/actions)

**High-concurrency microservice for atomic resource metering and real-time ledger balancing.**

Quantlo is a high-performance ledger engine designed to handle massive streams of consumption requests with sub-millisecond latency. It provides atomic balance operations, built-in idempotency, and a provider-agnostic architecture for any digital or physical unit (API credits, GPU-hours, liters, or IoT data).

---

## ⚡ Key Features

- 💎 **ACID Atomicity**: Ensures data integrity using Redis Lua scripts for the "hot path", preventing "double-spending" or negative balances even under extreme concurrency.
- 🛡️ **Native Idempotency**: Built-in support for `Idempotency-Key` to safely retry transactions in unstable network conditions without double counting.
- 🚀 **High Throughput**: Engineered as a non-blocking service. Asynchronous database synchronization ensures the API remains responsive while maintaining a persistent audit trail.
- 🔗 **Agnostic Metering**: Define your own resource types: tokens, liters, seconds, or requests.
- 🛠️ **Pluggable Infrastructure**: Choose between **NATS** or **gRPC** for internal event distribution and transaction synchronization.

---

## 🏗 Architecture Overview

Quantlo acts as a high-performance "middleman" between your business logic and persistent storage.

- **API Layer**: Fast HTTP/gRPC interfaces designed for low-latency response times.
- **Cache Layer (Hot Path)**: Every balance operation happens in Redis via atomic Lua scripts.
- **Event Bus**: Successful transactions are published to an internal bus (NATS or gRPC).
- **Worker (Persistence)**: A separate worker subscribes to the bus and persists transactions into **PostgreSQL** to ensure durability and consistency.

---

## 🚀 Getting Started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- [k6](https://k6.io/) (for load testing)

### Installation & Run

1. **Clone the repository**:

   ```bash
   git clone https://github.com/viacheslavprokosa/quantlo.git
   cd quantlo
   ```

2. **Start Infrastructure**:

   ```bash
   docker-compose up -d
   ```

3. **Run Migrations**:

   ```bash
   go run cmd/migrate/main.go up
   ```

4. **Start the API**:

   ```bash
   make run
   ```

---

## 📡 API Reference

### 1. Create Account

```bash
curl -X POST http://localhost:8080/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user_42",
    "resource_type": "api_credits",
    "initial_amount": 5000
  }'
```

### 2. Spend Resources (The Hot Path)

Includes mandatory idempotency to prevent duplicate charges.

```bash
curl -X POST http://localhost:8080/spend \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user_42",
    "resource_type": "api_credits",
    "amount": 10,
    "idempotency_key": "req-uuid-123"
  }'
```

### 3. Check Balance

```bash
curl "http://localhost:8080/balance?account_id=user_42&resource_type=api_credits"
```

---

## ⚙️ Configuration Providers

Quantlo allows you to switch between transport and worker providers via environment variables:

| Variable | Values | Description |
| :--- | :--- | :--- |
| `QANTLO_BUS_PROVIDER` | `nats`, `grpc` | Transport for internal event distribution. |
| `QANTLO_WORKER_PROVIDER` | `nats`, `grpc` | Transport for the DB sync worker. |
| `QANTLO_BUS_BUFFER_SIZE` | `int` | Internal buffer size for async gRPC publishing. |

---

## 🧪 Testing & Verification

### Unit Tests

```bash
go test ./...
```

### Load Testing

To verify system performance under high concurrency:

```bash
k6 run scripts/load_test.js
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
