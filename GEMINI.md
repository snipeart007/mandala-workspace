# Mandala Workspace - GEMINI Context

This repository is organized as a monorepo containing two separate software solutions: the Server and the Client. It uses a Go workspace (`go.work`) to manage these components.

## Repository Structure

- **`proto/`**: Shared Protobuf definitions for the gRPC API.
- **`server/`**: The backend gRPC service (Go module `mandala-workspace`).
- **`client/`**: The desktop client implementation (Go module `mandala-workspace/client`).
- **`go.work`**: Go workspace configuration managing both components.

### Shared API & Code Generation
- Proto definitions are centralized in the root `proto/` directory.
- **Strict Separation:** Each module (Server and Client) is responsible for its own code generation. Generated code is stored within each module's `gen/` directory to avoid cross-module dependency on generated artifacts.

## Server Overview (`server/`)

The server implements a secure file storage and user management service. It uses a challenge-response authentication mechanism for devices and a granular bitmask-based permission system for folder-level access control.

### Main Technologies (Server)
- **Language:** Go 1.26.2
- **API Framework:** gRPC with Protobuf
- **Database:** Modular system supporting SQLite 3 and PostgreSQL.
- **Authentication:** PASETO, Argon2id, Ed25519.
- **Permissions:** Custom bitmask-based system (`uint64`).

### Server Architecture
- **`server/gen/`**: Contains generated Go code for the server.
- **`server/internal/`**: Implements core logic (crypto, db, grpc, permission, storage).
- **`server/cmd/server/`**: Entry point for the gRPC server.

## Client Overview (`client/`)

The client is a desktop application providing a native file-explorer-like interface for the Mandala Workspace.

### Main Technologies (Client)
- **Framework:** Wails (v2) with Go backend and React frontend.
- **Frontend Stack:** React, Material UI, Zustand.
- **Security:** Password-protected local keyring using Argon2id for KDF and AES-256-GCM for encryption.

### Client Architecture
- **`client/gen/`**: Contains generated Go code for the client.
- **`client/frontend/`**: React-based frontend application.
- **`client/app.go`**: Wails application entry point and bindings.

## Building and Running

The project includes a root `Makefile` that delegates to component-specific tasks:

- **Setup Environment:** `make setup`
- **Generate All Proto Code:** `make server-proto client-proto`
- **Run Server Tests:** `make server-test`
- **Run Server:** `make server-run`
- **Run All Tests:** `make test`

## Development Conventions

- **Surgical Updates:** Prefer targeted changes to specific files over broad refactoring.
- **Validation:** Always run tests (`make test`) after making changes.
- **Database Schema:** Data model changes must be reflected in both SQLite and PostgreSQL schemas within `server/internal/db/sql/`.
- **Modular DB:** All database operations must be implemented via the `DBProvider` interface.
