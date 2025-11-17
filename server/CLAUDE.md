# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is "Alnitak", a Go-based video sharing platform server (similar to Bilibili/YouTube). It's a comprehensive backend service with video uploading, transcoding, user management, comments, and real-time features.

## Commands

### Development
```bash
# Run in development mode
go run cmd/main.go -env=dev

# Run in production mode (default)
go run cmd/main.go -env=prod

# Build the application
go build -o cmd.exe cmd/main.go

# Install dependencies
go mod tidy
```

### Testing
The project doesn't have a standard test command configured. Check for test files in individual packages before adding tests.

## Architecture

### Directory Structure
- `cmd/` - Application entry point (main.go)
- `internal/` - Private application code
  - `api/v1/` - HTTP API handlers
  - `routes/` - Route definitions and middleware setup
  - `service/` - Business logic layer
  - `global/` - Global variables and state
  - `config/` - Configuration management
  - `middleware/` - HTTP middleware
  - `domain/` - Domain models and DTOs
  - `initialize/` - Application initialization
  - `cron/` - Scheduled tasks
- `pkg/` - Reusable packages
  - `mysql/`, `redis/` - Database clients
  - `oss/` - Object storage interface
  - `logger/` - Logging utilities
  - `casbin/` - Authorization
  - `jwt/` - JWT authentication
- `conf/` - Configuration files (dev/prod YAML)
- `static/`, `upload/` - Static assets and uploads

### Key Components
1. **Web Framework**: Gin (HTTP router and middleware)
2. **Database**: GORM with MySQL, Redis for caching
3. **Authentication**: JWT tokens with Casbin RBAC
4. **Storage**: Multi-provider object storage (Minio, Aliyun OSS, Tencent COS)
5. **Real-time**: WebSocket support in `pkg/ws/`
6. **Video Processing**: Transcoding capabilities with GPU support
7. **ID Generation**: Snowflake algorithm for unique IDs

### Initialization Flow
The application initializes in this order (see `cmd/main.go:19-54`):
1. Config loading (dev/prod environment)
2. Logger setup
3. Captcha system
4. Object storage
5. Snowflake ID generator
6. MySQL database and tables
7. Video partition mapping
8. Redis cache
9. Casbin authorization
10. Background cron jobs
11. HTTP router (port 9000)

### Configuration
- Development: `conf/application.dev.yaml`
- Production: `conf/application.prod.yaml`
- Environment selected via `-env` flag (defaults to prod)

### API Structure
All endpoints are under `/api/v1/` with comprehensive route organization:
- Authentication, users, roles, permissions
- Video management, transcoding, partitions
- Comments, archives, collections
- Real-time messaging and online status
- File uploads and static serving

The server runs on port 9000 and serves static images at `/api/image/:file`.