# Solana High-Speed Indexer

A high-throughput Solana Devnet listener written in Go. The service subscribes to live transaction logs over WebSocket, processes events via a configurable worker pool, and stores transactions in PostgreSQL while caching recent items in Redis.

## Features

- Solana Devnet WebSocket listener (`LogsSubscribe`) using `github.com/gagliardetto/solana-go`
- Graceful shutdown with `context` + signal handling
- Configurable worker pool with `sync.WaitGroup`
- Storage abstraction via `TransactionStore` interface
- PostgreSQL persistence with `pgx/v5`
- Redis cache for latest transactions
- Dockerized app + Postgres + Redis stack

## Project Structure

```text
cmd/listener/main.go           # Application entrypoint
internal/solana/               # Solana WebSocket listener
internal/worker/               # Worker pool and transaction model
internal/storage/              # Storage interface + postgres + redis
db/migrations/                 # SQL migrations
```

## Requirements

- Go 1.25+
- Docker & Docker Compose (for containerized run)
- (Optional local run) PostgreSQL and Redis running locally

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_DSN` | `postgres://postgres:postgres@localhost:5432/solana_indexer?sslmode=disable` | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | empty | Redis password |

## Run Locally (without Docker)

1. Apply migration (example):

```bash
psql "postgres://postgres:postgres@localhost:5432/solana_indexer?sslmode=disable" \
  -f db/migrations/001_create_transactions.sql
```

2. Export environment variables if needed.
3. Start the app:

```bash
go run ./cmd/listener
```

## Run with Docker Compose

```bash
docker compose up --build -d
docker compose ps
docker compose logs -f indexer
```

Stop all services:

```bash
docker compose down
```

## Database Schema

Migration file:

- `db/migrations/001_create_transactions.sql`

Creates a `transactions` table with:

- `signature` (primary key)
- `slot`
- `observed_at`
- `created_at`

## Notes

- Listener connects to **Solana Devnet**: `wss://api.devnet.solana.com/`
- Incoming logs are normalized into transaction jobs (`Signature`, `Slot`, `Timestamp`)
- Workers process and persist each transaction to both PostgreSQL and Redis
