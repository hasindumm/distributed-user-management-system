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
# User Service — reads config.yaml by default (override with env vars)
cd user-service
go run cmd/main.go

# Override config via environment variables
DATABASE_URL="postgres://user:pass@localhost:5432/userdb?sslmode=disable" \
NATS_URL="nats://localhost:4222" \
go run cmd/main.go

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

## Internal NATS Communication

All inter-service communication happens over NATS. There are two distinct patterns:

### 1. RPC (Request / Reply)

Used when the caller needs a response — Gateway asks User Service to perform an operation and waits for the result.

| Subject | Operation | Description |
|---|---|---|
| `users.v1.create` | Create | Create a new user |
| `users.v1.get.by_id` | Read | Fetch a user by UUID |
| `users.v1.get.by_email` | Read | Fetch a user by email address |
| `users.v1.list` | Read | List users with optional status filter and pagination |
| `users.v1.update` | Update | Replace all mutable fields of a user |
| `users.v1.delete` | Delete | Soft-delete a user by UUID |

**Request/Response envelope:**

Every RPC response has the same shape — either a result or an error, never both:

```json
{ "user": { ...UserDTO... }, "error": null }
{ "user": null, "error": { "code": "NOT_FOUND", "message": "user not found" } }
```

**Error codes:**

| Code | Meaning |
|---|---|
| `NOT_FOUND` | No user matched the given ID or email |
| `ALREADY_EXISTS` | Email is already taken |
| `VALIDATION_ERROR` | Request payload failed validation |
| `INTERNAL_ERROR` | Unexpected server-side failure |

**Timeout:** callers apply a 5-second timeout by default (configurable via `userclient.Config.Timeout`).

---

### 2. Events (Publish / Subscribe)

Published by User Service after every successful mutation. Fire-and-forget — the RPC reply is sent first, then the event is published. No acknowledgement is expected.

| Subject | Trigger | Payload |
|---|---|---|
| `users.v1.events.created` | User successfully created | Full `UserDTO` |
| `users.v1.events.updated` | User successfully updated | Full `UserDTO` |
| `users.v1.events.deleted` | User successfully deleted | `{ "user_id": "<uuid>" }` |

**Created / Updated payload:**
```json
{
  "user": {
    "user_id": "a7f5b6e2-...",
    "first_name": "Jane",
    "last_name": "Doe",
    "email": "jane@example.com",
    "phone": "+1234567890",
    "age": 30,
    "status": "ACTIVE",
    "created_at": "2026-03-31T10:00:00Z",
    "updated_at": "2026-03-31T10:00:00Z"
  }
}
```

**Deleted payload:**
```json
{ "user_id": "a7f5b6e2-..." }
```

Only the ID is included on deletion because the user record no longer exists.

---

### Client Library (`user-service/pkg/userclient`)

The client library abstracts all NATS details. Consumers never interact with NATS subjects, JSON encoding, or subscription management directly.

**CRUD operations:**
```go
client, _ := userclient.New(userclient.Config{NATSURL: "nats://localhost:4222"}, logger)

user, err := client.CreateUser(ctx, userclient.CreateUserRequest{...})
user, err := client.GetUserByID(ctx, id)
user, err := client.GetUserByEmail(ctx, email)
users, err := client.ListUsers(ctx, userclient.ListUsersRequest{Limit: 50})
user, err := client.UpdateUser(ctx, userclient.UpdateUserRequest{...})
err        := client.DeleteUser(ctx, id)
```

**Event subscriptions:**
```go
sub, err := client.Subscribe(userclient.EventHandlers{
    OnCreated: func(e userclient.UserCreatedEvent) { /* handle */ },
    OnUpdated: func(e userclient.UserUpdatedEvent) { /* handle */ },
    OnDeleted: func(e userclient.UserDeletedEvent) { /* handle */ },
})

// Stop receiving events
sub.Unsubscribe()
```

All three handlers are optional — omit any field to skip that event type.

---

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

