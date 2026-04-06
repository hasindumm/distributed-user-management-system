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
│   ├── internal/
│   │   ├── adapters/
│   │   │   ├── httphandler/    # REST inbound adapter
│   │   │   ├── userclientadapter/ # outbound adapter (wraps userclient)
│   │   │   └── wshandler/      # WebSocket inbound adapter
│   │   ├── app/                # core service (ports only, no userclient)
│   │   ├── dto/                # HTTP/WS data shapes + validation tags
│   │   ├── middleware/         # request logging
│   │   └── ports/              # UserService, UserClient, Subscription, events
│   ├── docs/
│   │   ├── openapi.yaml        # REST API spec (OpenAPI 3.0.3)
│   │   └── asyncapi.yaml       # WebSocket API spec (AsyncAPI 2.6.0)
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
│   ├── openapi.yaml        # REST API spec (OpenAPI 3.0.3)
│   └── asyncapi.yaml       # WebSocket API spec (AsyncAPI 2.6.0)
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

---

## Run the Full System (Docker)

The entire system — PostgreSQL, NATS, user-service, and gateway — runs with a single command.

### Start everything

```bash
docker-compose up
```

Docker Compose will:
1. Start **postgres** and wait until it is healthy (accepting connections)
2. Start **nats** and wait until it is healthy (monitoring endpoint responds)
3. Start **user-service** — connects to postgres, runs migrations, subscribes to NATS
4. Start **gateway** — connects to NATS, starts HTTP server on port 8080

On the first run, Docker will build the images automatically. This takes a few minutes.

### Start in the background

```bash
docker-compose up -d
```

### View logs

```bash
# All services
docker-compose logs -f

# One service only
docker-compose logs -f user-service
docker-compose logs -f gateway
```

### Stop everything

```bash
docker-compose down
```

### Rebuild images after code changes

```bash
docker-compose build
docker-compose up
```

Or in one step:

```bash
docker-compose up --build
```

### Verify the system is running

```bash
curl http://localhost:8080/api/v1/health
```

Expected response: `{"status":"ok"}`

### Create a user

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"first_name":"Jane","last_name":"Doe","email":"jane@example.com","status":"ACTIVE"}'
```

### List users

```bash
curl http://localhost:8080/api/v1/users
```

---

## Run Services Locally (without Docker)

For local development, run postgres and NATS via Docker, then the services directly with Go.

### Start infrastructure only

```bash
docker-compose up postgres nats -d
```

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

### Run services locally

```bash
# User Service — reads config.yaml by default (override with env vars)
cd user-service
go run cmd/main.go

# Override config via environment variables
DATABASE_URL="postgres://user:pass@localhost:5432/users?sslmode=disable" \
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

## Gateway REST API

Full spec: `gateway-service/docs/openapi.yaml` (OpenAPI 3.0.3).

Base URL: `http://localhost:8080`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/health` | Health check |
| `POST` | `/api/v1/users` | Create user |
| `GET` | `/api/v1/users` | List users (query: `status`, `limit`, `offset`) |
| `GET` | `/api/v1/users/{id}` | Get user by UUID |
| `PUT` | `/api/v1/users/{id}` | Update user |
| `DELETE` | `/api/v1/users/{id}` | Delete user |
| `GET` | `/api/v1/users/email/{email}` | Get user by email |

---

## Gateway WebSocket API

Full spec: `gateway-service/docs/asyncapi.yaml` (AsyncAPI 2.6.0).

Connect: `ws://localhost:8080/api/v1/ws`

### Protocol

Every message is a JSON object with an `action` field. Optionally include a `request_id` — it is echoed back in the reply so you can match responses to requests.

**Request envelope (client → server):**
```json
{
  "action":     "user.create",
  "request_id": "req-001",
  "payload":    { ... }
}
```

**Response envelope (server → client):**
```json
{ "action": "user.create", "request_id": "req-001", "success": true,  "payload": { ... } }
{ "action": "user.create", "request_id": "req-001", "success": false, "error": { "code": "...", "message": "..." } }
```

### Actions

| Action | Payload fields | Description |
|--------|----------------|-------------|
| `user.create` | `first_name`, `last_name`, `email`, `phone`?, `age`?, `status`? | Create a user |
| `user.get_by_id` | `user_id` | Get user by UUID |
| `user.get_by_email` | `email` | Get user by email |
| `user.list` | `status`?, `limit`?, `offset`? | List users |
| `user.update` | `user_id`, `first_name`, `last_name`, `email`, `status`, `phone`?, `age`? | Update a user |
| `user.delete` | `user_id` | Delete a user |
| `user.subscribe` | _(none)_ | Start receiving real-time push events |
| `user.unsubscribe` | _(none)_ | Stop receiving push events |

### Error codes

| Code | Meaning |
|------|---------|
| `BAD_REQUEST` | Missing required field or malformed JSON |
| `NOT_FOUND` | No user matched the given ID or email |
| `ALREADY_EXISTS` | Email is already taken |
| `VALIDATION_ERROR` | Field validation failed |
| `INTERNAL_ERROR` | Unexpected server-side failure |
| `UNKNOWN_ACTION` | Unrecognised action string |

### Real-time push events

After sending `user.subscribe`, the server pushes these messages without being asked:

| Action | Trigger | Payload |
|--------|---------|---------|
| `user.created` | Any user is created | Full user object |
| `user.updated` | Any user is updated | Full user object |
| `user.deleted` | Any user is deleted | `{ "user_id": "..." }` |

Push events have **no `request_id`** — they are not replies to any request.

### Quick example (wscat)

```bash
# Install: npm install -g wscat
wscat -c ws://localhost:8080/api/v1/ws

# Create a user
{"action":"user.create","request_id":"1","payload":{"first_name":"John","last_name":"Doe","email":"john@example.com","status":"ACTIVE"}}

# Subscribe to live events
{"action":"user.subscribe","request_id":"2"}

# Now any create/update/delete anywhere will push an event to this connection
```

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

