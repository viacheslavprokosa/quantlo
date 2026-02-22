# ==========================================
# Stage 1: Builder 
# ==========================================
FROM golang:1.26-bookworm AS builder

WORKDIR /app

COPY go.mod ./
# COPY go.sum ./ 
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o qantlo-api ./cmd/api/main.go

# ==========================================
# Stage 2: Production
# ==========================================
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/qantlo-api .

EXPOSE 8080

CMD ["./qantlo-api"]