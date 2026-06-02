# Mandala Workspace

A secure, distributed, automatically-versioned hierarchical filesystem and workspace system. It is organized as a Go-based monorepo employing **Go Workspaces (`go.work`)** to manage the decoupled **Server** and **Desktop Client** components.

---

## 🏗️ Repository Architecture

This repository is structured as a monorepo consisting of two principal software solutions communicating via high-performance **gRPC**:

*   **[`proto/`](file:///home/snipeart007/repos/mandala-workspace/proto)**: Centralized Protocol Buffers definitions containing API contracts and service definitions for client-server interaction.
*   **[`server/`](file:///home/snipeart007/repos/mandala-workspace/server)**: The backend gRPC service (Go module `mandala-workspace`) implementing storage, access control, database interfaces, and cryptography.
*   **[`client/`](file:///home/snipeart007/repos/mandala-workspace/client)**: A native desktop application (Go module `mandala-workspace/client`) utilizing the Wails v2 framework with a Go backend and a React/TypeScript frontend.
*   **[`go.work`](file:///home/snipeart007/repos/mandala-workspace/go.work)**: Go workspace configuration managing unified dependency resolution.
*   **[`Makefile`](file:///home/snipeart007/repos/mandala-workspace/Makefile)**: Master build, setup, and test execution tasks.
*   **[`GEMINI.md`](file:///home/snipeart007/repos/mandala-workspace/GEMINI.md)**: AI context file and development guidelines.

---

## 🛠️ Global Technology Stack & Capabilities

| Component | Stack & Technologies | Key Capabilities |
| :--- | :--- | :--- |
| **Shared API** | Protobuf v3, gRPC | Schema-first, high-throughput RPCs, client & server streaming |
| **Server Backend** | Go (1.26.2), PostgreSQL, SQLite, Redis | CAS sharded storage, custom bitmask access control, Postgres JSONB indexing, Redis hierarchical invalidation |
| **Desktop Client** | Wails v2, React, Material UI (MUI), Zustand, TypeScript | Cross-platform desktop interface, native file execution flow, password-protected local identity keyring |
| **Security Layer** | PASETO (v2), Ed25519, Argon2id, AES-256-GCM | Challenge-response authentication, KDF secure keyring, transport encryption integrity checks |

---

## 🔑 Core Features & Cryptographic Architecture

### 1. Challenge-Response Authentication & PASETO Sessions
Instead of transmitting credentials, the workspace utilizes asymmetric cryptography:
*   **Device Keypair**: Devices register their public key (Ed25519) on the server.
*   **Login Challenge**: The server presents a randomized cryptographic challenge. The client signs it locally with the private key from its secure keyring.
*   **PASETO Tokens**: After validation, the server issues a PASETO session token containing cryptographic expirations.

### 2. Hierarchical & Granular Access Control
*   **Permissions Bitmask**: Custom `uint64` permissions bitmask providing folder-level access control (Read, Write, Admin, Create Folder, Delete, Move, Set Permissions).
*   **Path-based Inheritance**: Permissions default to inheriting from parent folders (leveraging prefix-based path indexing like `1.12.34`).
*   **Inheritance Break**: Folders can disable inheritance, isolating child directories. If inheritance is disabled, explicit creator permissions from the parent are snapshot-assigned.

### 3. High-Performance Caching & Synchronization
*   **Redis Dependency**: Redis is a hard dependency. If Redis is down, the server will not function.
*   **Hierarchical Invalidation**: Effective permissions are cached in Redis under versioned keys: `eff_perm:{user_id}:{folder_id}:{v}`. Incrementing a folder/user version key invalidates cached descendant permissions in $O(1)$ time.
*   **Permissions Queue**: An incremental updates queue (`perm_updates:{user_id}`) processes cached permission changes and pushes them down to the client.

### 4. Content-Addressable Storage (CAS)
*   **Two-Level Sharding**: Files are stored based on their content hash (e.g., `storage/ab/cd/hash_remnants`).
*   **Integrity Hash Checks**: Content is re-hashed using SHA-256 on every retrieve operation to ensure no file degradation.
*   **Automated Versioning**: File uploads with identical names are atomically versioned, adhering to custom retention policies per folder.

### 5. Secure Identity Vault (Client Keyring)
*   The desktop client encrypts Ed25519 identity keys and session tokens.
*   Uses **Argon2id** for Key Derivation (KDF) from a user master password.
*   Performs symmetric encryption/decryption in memory using **AES-256-GCM**.

---

## ⚡ Development & Getting Started

### Prerequisites
*   **Go** (1.20+ recommended)
*   **Node.js** (v16+ for desktop frontend)
*   **Protocol Buffer Compiler (`protoc`)**
*   **Redis** (running locally or accessible via config)
*   **PostgreSQL** (production DB) / **SQLite** (used for integration tests)

### Build and Setup Workflow

To initialize the monorepo workspace, install the necessary tooling, generate Go bindings, and run tests.

> [!IMPORTANT]
> Since the Protobuf compiler `protoc` invokes code generation plugins installed via `go install`, your system's `PATH` must contain the Go bin directory (typically `~/go/bin`). If you receive plugin errors, prepend this directory to your execution path as shown below.

```bash
# 1. Setup dev tools, download dependencies, and generate protobuf files
export PATH=$PATH:$(go env GOPATH)/bin
make setup

# 2. Re-generate Protobuf sources for server and client (if changed)
make server-proto client-proto

# 3. Run all server and client unit & integration tests
make test

# 4. Start the server locally
make server-run
```

---

## 📐 Development Conventions

*   **Strict Code Generation Separation**: To prevent circular dependencies, each module is responsible for its own code generation. Output must reside in each module's local `gen/` directory (e.g. `server/gen/` and `client/gen/`). Do not cross-import generated sources.
*   **Database Dual-Schema Alignment**: PostgreSQL is the primary production database, while SQLite is utilized for testing. If you make data model changes, you **must** update the schemas in both:
    *   PostgreSQL schema: [`server/internal/db/sql/InitializeDB.postgres.sql`](file:///home/snipeart007/repos/mandala-workspace/server/internal/db/sql/InitializeDB.postgres.sql)
    *   SQLite schema: [`server/internal/db/sql/InitializeDB.sql`](file:///home/snipeart007/repos/mandala-workspace/server/internal/db/sql/InitializeDB.sql)
*   **Modular DB Operations**: Implement all database queries and interactions through the [`DBProvider`](file:///home/snipeart007/repos/mandala-workspace/server/internal/db/interface.go) interface.
*   **Structured Logging**: Utilize Go's structured logging library `log/slog` for all application layers.
