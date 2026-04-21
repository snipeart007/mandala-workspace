# Mandala Workspace - GEMINI Context

This document provides architectural and operational context for the `mandala-workspace` project, a Go-based backend for a secure file and workspace management system.

## Project Overview

`mandala-workspace` implements a secure file storage and user management service. It uses a challenge-response authentication mechanism for devices and a granular bitmask-based permission system for folder-level access control.

### Main Technologies
- **Language:** Go 1.26.2
- **API Framework:** gRPC with Protobuf
- **Database:** SQLite 3 (using `github.com/mattn/go-sqlite3`)
- **Authentication:**
    - **PASETO:** Used for stateless session tokens.
    - **Argon2id:** Used for secure password hashing.
    - **Ed25519:** Used for device-based challenge-response authentication.
- **Permissions:** Custom bitmask-based system (`uint64`) allowing efficient permission checks.

## Architecture

The project follows a modular structure within the `internal/` directory:

- **`proto/mandala/v1/`**: Contains `.proto` definitions for the `UserService` and potentially other services.
- **`gen/`**: Contains generated Go code from protobuf definitions.
- **`internal/crypto/`**: Implements cryptographic primitives:
    - `argon2`: Password hashing and verification.
    - `ed25519`: Signature generation and verification for device authentication.
    - `paseto`: Token creation and validation.
- **`internal/db/`**: Handles database interactions:
    - `DBManager`: Manages the SQLite connection and basic CRUD operations.
    - `sql/InitializeDB.sql`: Defines the database schema (folders, users, devices, files, versions, permissions).
- **`internal/grpc/`**:
    - `user_service`: Implementation of the `UserService` (CreateUser, RegisterDevice, LoginUser, VerifyLoginSignature).
    - `interceptors`: Contains authentication middleware.
- **`internal/permission/`**: Defines the permission system:
    - `PermissionBitMask`: Bitmask definitions for `PermRead`, `PermWrite`, `PermUserCreate`, `PermAdmin`, etc.
    - `PermissionManager`: Logic for checking and generating permissions based on hierarchy.
- **`internal/types/`**: Domain models (User, Device, Folder, File, Version).

## Building and Running

The project includes a `Makefile` for common tasks:

- **Install Tools:** `make install-proto-tools`
- **Setup Environment:** `make setup` (installs tools, downloads deps, and generates proto)
- **Generate Proto Code:** `make proto`
- **Run Tests:** `make test`
- **Run Server:** `make run` (Note: Currently references `cmd/server/main.go`, which may need to be created or updated).
- **Build Binary:** `make build`

## Development Conventions

- **Surgical Updates:** Prefer targeted changes to specific files over broad refactoring.
- **Validation:** Always run tests (`make test`) after making changes to ensure core logic (especially authentication and permissions) remains intact.
- **Database Schema:** Any changes to the data model should be reflected in `internal/db/sql/InitializeDB.sql` and `Database Schema.md`.
- **Error Handling:** Use gRPC `status` codes for API-level errors.
- **Permission Checks:** All administrative actions (like `CreateUser` or `RegisterDevice`) must be guarded by permission checks using `PermissionManager`.

## Key Files
- `proto/mandala/v1/user_service.proto`: API contract.
- `internal/db/sql/InitializeDB.sql`: Source of truth for the database schema.
- `internal/permission/permission_bitmask.go`: Source of truth for available permissions.
- `internal/grpc/user_service/user_service.go`: Main service implementation logic.
