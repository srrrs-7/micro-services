# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

This is a Go-based microservices architecture with the following services:
- **audit**: Audit service with API and worker components
- **auth**: Authentication service 
- **queue**: Queue service with gRPC API
- **shared**: Shared utilities and logging

Each service is containerized with Docker and orchestrated via Docker Compose. Services communicate via gRPC protocols.

## Common Commands

### Development Environment
- `make gopher` - Enter interactive Go development container
- `make test` - Run all tests across all modules
- `make tidy` - Run go mod tidy on all modules  
- `make vet` - Run go vet on all modules

### Service Management
- `make audit` - Build and start audit service (API + worker + database + queue)
- `make auth` - Build and start auth service (API + database)
- `make audit-migrate` - Run database migrations for audit service
- `make auth-migrate` - Run database migrations for auth service

### Database Migrations
- `make new-migrate MODULE=<service_name>` - Create new migration for specified module
- Migrations are stored in `modules/<service>/database/migration/`

### Container Management
- `make rmi` - Remove dangling Docker images
- `make rmv` - Prune Docker volumes

## Project Structure

```
modules/
├── audit/src/          # Audit service (Go 1.24.4)
│   ├── cmd/api/        # API server entry point
│   ├── cmd/worker/     # Worker process entry point
│   ├── domain/         # Business logic
│   └── driver/         # Database/external adapters
├── auth/src/           # Auth service (Go 1.24.4)
│   ├── cmd/api/        # API server entry point
│   ├── domain/token/   # Token management
│   └── driver/         # Database/external adapters
├── queue/src/          # Queue service (Go 1.24.4)
│   ├── cmd/api/        # gRPC API server
│   ├── routes/grpc/    # gRPC protocol definitions
│   └── domain/         # Business logic
└── shared/src/         # Shared utilities (Go 1.24.4)
    └── logging/        # Common logging utilities
```

## Development Notes

- All services use Go 1.24.4
- PostgreSQL databases run on ports 5432 (audit) and 5433 (auth)  
- gRPC protocol definitions are in `modules/queue/src/routes/grpc/queue.proto`
- Database migrations use Atlas migration tool via the migrator container
- Tests run via the gopher container across all modules simultaneously