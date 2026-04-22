# Mandala Workspace - GEMINI Context

This document provides architectural and operational context for the `mandala-workspace` project, a Go-based backend for a secure file and workspace management system.

## Project Overview

`mandala-workspace` implements a secure file storage and user management service. It uses a challenge-response authentication mechanism for devices and a granular bitmask-based permission system for folder-level access control.

### Main Technologies
- **Language:** Go 1.26.2
- **API Framework:** gRPC with Protobuf
- **Database:** Modular system supporting SQLite 3 and PostgreSQL.
    - **SQLite:** Uses `github.com/mattn/go-sqlite3`.
    - **PostgreSQL:** Uses `github.com/jackc/pgx/v5`.
- **Authentication:**
    - **PASETO:** Used for stateless session tokens.
    - **Argon2id:** Used for secure password hashing.
    - **Ed25519:** Used for device-based challenge-response authentication.
- **Permissions:** Custom bitmask-based system (`uint64`) allowing efficient permission checks.

## Architecture

The project follows a modular structure within the `internal/` directory:

- **`proto/mandala/v1/`**: Contains `.proto` definitions for the `UserService`, `FolderService`, and `FileService`.
- **`gen/`**: Contains generated Go code from protobuf definitions.
- **`internal/crypto/`**: Implements cryptographic primitives:
    - `argon2`: Password hashing and verification.
    - `ed25519`: Signature generation and verification for device authentication.
    - `paseto`: Token creation and validation.
- **`internal/db/`**: Modular database management:
    - `interface.go`: Defines the `DBProvider` interface for all database operations.
    - `models.go`: Shared database models (User, Device, Folder, File, Version).
    - `sqlite/`: SQLite-specific implementation of `DBProvider`.
    - `postgres/`: PostgreSQL-specific implementation of `DBProvider`.
    - `sql/`: Database schema definitions (`InitializeDB.sql` for SQLite, `InitializeDB.postgres.sql` for PostgreSQL).
- **`internal/grpc/`**:
    - `user_service`: Implementation of the `UserService`.
    - `folder_service`: Implementation of `FolderService`.
    - `file_service`: Implementation of `FileService`.
    - `interceptors`: Contains authentication middleware.
- **`internal/permission/`**: Defines the permission system:
    - `PermissionBitMask`: Bitmask definitions for `PermRead`, `PermWrite`, etc.
    - `PermissionManager`: Logic for evaluating permissions based on folder hierarchy and inheritance.
- **`internal/storage/`**: Implements the modular Content-Addressable Storage (CAS):
    - `interface.go`: Defines the `CASProvider` interface.
    - `local.go`: Sharded local disk implementation.
    - `s3.go`: AWS S3-based implementation.
    - `registry.go`: Routes storage requests based on URI schemes.
- **`internal/server/`**: Core server lifecycle management. `ServerInstance` coordinates all components and supports multiple database backends via configuration.

## Building and Running

The project includes a `Makefile` for common tasks:

- **Install Tools:** `make install-proto-tools`
- **Setup Environment:** `make setup`
- **Generate Proto Code:** `make proto`
- **Run Tests:** `make test`
- **Run Server:** `make run` (Starts the gRPC server using `cmd/server/main.go`).
- **Build Binary:** `make build`

## Development Conventions

- **Surgical Updates:** Prefer targeted changes to specific files over broad refactoring.
- **Validation:** Always run tests (`make test`) after making changes to ensure core logic (especially authentication and permissions) remains intact.
- **Database Schema:** Data model changes must be reflected in BOTH `internal/db/sql/InitializeDB.sql` and `internal/db/sql/InitializeDB.postgres.sql`.
- **Error Handling:** Use gRPC `status` codes for API-level errors.
- **Modular DB:** All database operations must be implemented via the `DBProvider` interface to maintain backend independence.

## Key Files
- `proto/mandala/v1/*.proto`: API contracts.
- `internal/db/interface.go`: Definition of the core database interface.
- `internal/db/sql/InitializeDB.sql`: SQLite schema source of truth.
- `internal/db/sql/InitializeDB.postgres.sql`: PostgreSQL schema source of truth.
- `internal/permission/permission_bitmask.go`: Source of truth for available permissions.
- `internal/storage/interface.go`: Defines core CAS capabilities.
- `internal/grpc/`: gRPC service implementations.
- `cmd/server/main.go`: Entry point for the gRPC server.
