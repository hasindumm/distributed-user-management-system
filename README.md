# Distributed User Management System

A distributed User Management System built with Go and PostgreSQL, following microservice and event-driven architecture principles.

## Services

- **Gateway Service** — REST + WebSocket API, routes requests to User Service via NATS
- **User Service** — Core business logic, PostgreSQL persistence, NATS RPC + event publishing

## Tech Stack

| Concern | Technology |
|---|---|
| Language | Go 1.22.5 |
| Database | PostgreSQL |
| Messaging | NATS |
| Router | Chi |
| WebSocket | Gorilla WebSocket |
| SQL Generator | SQLC |
| Validation | go-playground/validator |
| Logging | log/slog |
| Linting | golangci-lint |

## Project Structure
```
distributed-user-management-system/
├── gateway-service/
│   ├── cmd/
│   └── go.mod
├── user-service/
│   ├── cmd/
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

- Go 1.22.5
- Docker + Docker Compose
- pre-commit

### Install pre-commit hooks
```bash
pip install pre-commit
pre-commit install
pre-commit install --hook-type commit-msg
```

This enforces:
- Go lint validation on commit
- Conventional Commit message format

### Start infrastructure (NATS + PostgreSQL)
```bash
docker compose up -d
```

## CI/CD

| Pipeline | Trigger | Steps |
|---|---|---|
| CI (`ci.yml`) | Push to any branch / PR | Lint + Tests |
| CD (`cd.yml`) | Push to `main` | Build + Docker image (stubbed) |

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

## Milestones

- [x] Milestone 0 — Repo, Tooling, Hooks, CI Skeleton, Base Compose
- [ ] Milestone 1 — Data Model & SQLC Foundation
- [ ] Milestone 2 — NATS RPC + Client Library Skeleton
- [ ] Milestone 3 — User Events + Client Subscribe/Unsubscribe
- [ ] Milestone 4 — Client Library Caching
- [ ] Milestone 5 — Gateway REST API + OpenAPI
- [ ] Milestone 6 — Gateway WebSocket API + AsyncAPI
- [ ] Milestone 7 — Containerization + Full System Compose
- [ ] Milestone 8 — Hardening & Release CI Complete