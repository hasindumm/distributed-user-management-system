# Distributed User Management System

A distributed User Management System built with Go and PostgreSQL, following microservice and event-driven architecture principles.

## Services

- **Gateway Service** — REST + WebSocket API, routes requests to User Service via NATS
- **User Service** — Core business logic, PostgreSQL persistence, NATS RPC + event publishing

## Tech Stack

| Concern        | Technology                  |
|----------------|-----------------------------|
| Language       | Go 1.25.0                   |
| Database       | PostgreSQL                  |
| Messaging      | NATS                        |
| Router         | Chi                         |
| WebSocket      | Gorilla WebSocket           |
| SQL Generator  | SQLC                        |
| Validation     | go-playground/validator     |
| Logging        | log/slog                    |
| Linting        | golangci-lint v1.64.8       |
| Testing        | testify + testcontainers-go |

## Architecture

Both services follow **Hexagonal Architecture (Ports & Adapters)**:

- `internal/domain/` — pure business entities, no external dependencies
- `internal/ports/` — interfaces defining boundaries (repository, publisher)
- `internal/service/` — core business logic, depends only on ports
- `internal/adapters/` — external implementations (PostgreSQL, NATS)
- `pkg/` — public client library importable by other services
- `cmd/` — entry point, wires everything together

## Project Structure

```
distributed-user-management-system/
├── gateway-service/
│   ├── cmd/
│   │   └── main.go
│   └── go.mod
├── user-service/
│   ├── cmd/
│   │   └── main.go
│   ├── db/
│   │   ├── migrations/
│   │   ├── queries/
│   │   └── sqlc.yaml
│   ├── internal/
│   │   ├── domain/
│   │   ├── ports/
│   │   ├── service/
│   │   └── adapters/
│   │       └── postgresadaptor/
│   └── go.mod
├── docs/
│   ├── openapi.yaml        # (upcoming)
│   └── asyncapi.yaml       # (upcoming)
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── cd.yml
├── .golangci.yml
├── .pre-commit-config.yaml
└── docker-compose.yml
```

## Getting Started

### Prerequisites

- Go 1.25.0
- Docker + Docker Compose
- pre-commit
- golangci-lint v1.64.8
- sqlc

### Install pre-commit hooks

```bash
pip install pre-commit
pre-commit install
pre-commit install --hook-type commit-msg
```

This enforces:
- Go lint validation on commit
- Conventional Commit message format

### Install sqlc

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### Start infrastructure (NATS + PostgreSQL)

```bash
docker compose up -d
```

### Run services locally

```bash
# User Service
cd user-service
DATABASE_URL="postgres://user:password@localhost:5432/dbname?sslmode=disable" go run cmd/main.go

# Gateway Service
cd gateway-service
go run cmd/main.go
```

### Run tests

```bash
# User Service (requires Docker for testcontainers)
cd user-service
go test ./...

# Gateway Service
cd gateway-service
go test ./...
```

## Pre-commit Checks

Before every commit the following run automatically:
- `golangci-lint` on both services
- Conventional Commit message validation

To run manually:

```bash
pre-commit run --all-files
```

## CI/CD

| Pipeline      | Trigger                 | Steps           |
|---------------|-------------------------|-----------------|
| CI (`ci.yml`) | Push to any branch / PR | Lint + Tests    |
| CD (`cd.yml`) | Push to `main`          | Build binaries  |

## Git Conventions

This project follows [Conventional Commits](https://www.conventionalcommits.org/):

```
feat:     new feature
fix:      bug fix
chore:    tooling, config changes
docs:     documentation
test:     adding or updating tests
refactor: code restructure without behavior change
```

## Domain Model

| Field     | Type    | Description              |
|-----------|---------|--------------------------|
| userId    | UUID    | Primary key, auto-generated |
| firstName | String  | Required                 |
| lastName  | String  | Required                 |
| email     | String  | Required, unique         |
| phone     | String  | Optional                 |
| age       | Integer | Optional                 |
| status    | Enum    | Optional, default Active |

Status values: `ACTIVE`, `INACTIVE`, `SUSPENDED`

## Milestones

- ✅ Milestone 0 — Repo, Tooling, Hooks, CI Skeleton, Base Compose
- ✅ Milestone 1 — Data Model & SQLC Foundation
- ⬜ Milestone 2 — NATS RPC + Client Library Skeleton
- ⬜ Milestone 3 — User Events + Client Subscribe/Unsubscribe
- ⬜ Milestone 4 — Client Library Caching
- ⬜ Milestone 5 — Gateway REST API + OpenAPI
- ⬜ Milestone 6 — Gateway WebSocket API + AsyncAPI
- ⬜ Milestone 7 — Containerization + Full System Compose
- ⬜ Milestone 8 — Hardening & Release CI Complete