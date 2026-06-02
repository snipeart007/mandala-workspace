# Mandala Backend Server

A high-performance, secure gRPC server managing the Mandala Workspace filesystem. It implements content-addressable storage (CAS), automated file versioning, inherited folder permissions, and a Redis-backed hierarchical cache invalidation engine.

---

## 🏗️ Architecture & Component Design

The server is structured into modular layers adhering to clean architecture principles:

```
                            [ gRPC API Client ]
                                     │
                        ┌────────────▼────────────┐
                        │   gRPC Service Layer    │
                        └────────────┬────────────┘
                                     │
                        ┌────────────▼────────────┐
                        │   Permission Manager    │
                        └────────────┬────────────┘
                                     │
                        ┌────────────▼────────────┐
                        │    CachedDBProvider     │
                        └─────┬──────────────┬────┘
                              │              │
                     ┌────────▼──────┐ ┌─────▼────────┐
                     │ CacheManager  │ │  DBProvider  │
                     │    (Redis)    │ │ (PostgreSQL) │
                     └───────────────┘ └──────────────┘
```

### Server Components
*   **[`cmd/server/`](file:///home/snipeart007/repos/mandala-workspace/server/cmd/server)**: Entry point that instantiates database providers, cache managers, and starts the gRPC listener.
*   **[`internal/crypto/`](file:///home/snipeart007/repos/mandala-workspace/server/internal/crypto)**: Implements security protocols (PASETO v2, Ed25519 signature verification, Argon2id password hashing).
*   **[`internal/db/`](file:///home/snipeart007/repos/mandala-workspace/server/internal/db)**: Contains the database access abstraction layer.
    *   [`interface.go`](file:///home/snipeart007/repos/mandala-workspace/server/internal/db/interface.go): The master `DBProvider` interface.
    *   `postgres`: SQL execution for production PostgreSQL.
    *   `sqlite`: In-memory and file-based execution for integration tests.
    *   `cached_db.go`: The database wrapper caching query results in Redis.
*   **[`internal/grpc/`](file:///home/snipeart007/repos/mandala-workspace/server/internal/grpc)**: Contains RPC endpoint implementations grouped by domain:
    *   `user_service`: Registration, authentication, profile administration, session management.
    *   `folder_service`: Navigation, directory management, permissions updates.
    *   `file_service`: Streamed file uploads and downloads, version control, recycle bin.
*   **[`internal/permission/`](file:///home/snipeart007/repos/mandala-workspace/server/internal/permission)**: Decodes and validates `uint64` bitmask operations.
*   **[`internal/storage/`](file:///home/snipeart007/repos/mandala-workspace/server/internal/storage)**: Content-Addressable Storage (CAS) implementing cryptographic integrity checks and two-level folder sharding.

---

## ⚡ Core Technical Features & Hard Dependencies

### 1. Redis Caching & O(1) Hierarchical Invalidation
*   **Redis is a hard dependency.** The server fails startup checks if Redis is unreachable.
*   **Effective Permissions Cache**: Effective permissions are cached in Redis under versioned keys: `eff_perm:{user_id}:{folder_id}:{v}`.
*   **Hierarchical Invalidation**: Folders maintain path prefixes (e.g., `1.3.12` representing Root -> Folder 3 -> Folder 12). To invalidate descendant permissions instantly when permissions are updated or inheritance is modified, the version token `v` is incremented. Descendant caches will suffer a cache-miss and reload from SQL, ensuring consistency with $O(1)$ invalidation complexity.

### 2. Multi-Engine SQL Design
*   **Production**: **PostgreSQL** is the required database for production, utilizing `JSONB` data types for arbitrary metadata blobs and JSON indexing.
*   **Testing**: **SQLite** is used in-memory during integration testing to guarantee clean states without database setup overhead.
*   Database schemas must be synced across:
    *   [SQLite Schema Script](file:///home/snipeart007/repos/mandala-workspace/server/internal/db/sql/InitializeDB.sql)
    *   [PostgreSQL Schema Script](file:///home/snipeart007/repos/mandala-workspace/server/internal/db/sql/InitializeDB.postgres.sql)

### 3. Asymmetric Challenge-Response Auth
To safeguard user files, the server does not store raw passwords or accept password-only login from client machines:
1.  **Register**: The user registers their username/email and a device public key (Ed25519).
2.  **Challenge**: The server issues a randomly generated payload.
3.  **Sign & Verify**: The client signs this challenge with the device's private key. The server validates the signature against the database public key.
4.  **Issue**: Upon successful validation, the server generates a PASETO v2 token cached in Redis for fast validation.

### 4. Sharded Content-Addressable Storage (CAS)
Files are stored based on their cryptographically computed SHA-256 hash.
*   **Directory Sharding**: Storage paths are generated using the first two levels of the hex hash (e.g. `storage/2f/4a/c30198de...`). This keeps directories balanced and avoids file-system count limits.
*   **Integrity Enforcement**: When retrieving a file, the server streams the file through a hash writer and checks it against the database hash. If a mismatch is detected, the operation aborts to prevent corrupt downloads.
*   **Retention Enforcement**: Folder records support a `version_retention` limit (`N`). Successive uploads prune the oldest file versions from DB records.

---

## 🚀 Build, Run, and Test

### Prerequisites
*   **Go** 1.20+
*   **Redis** (running locally on default port 6379, or configured via environment)
*   **PostgreSQL** (running locally)

### Commands

From the `server/` directory:

#### Install Protocol Tools & Generate Schemas
```bash
make setup
```

#### Rebuild Protobuf Bindings
```bash
make proto
```

#### Run Unit & Integration Tests
```bash
make test
```

#### Run Backend Server
```bash
make run
```

#### Build Production Binary
```bash
make build
```

---

## 🛡️ Access Control Bitmask Definitions

The permissions bitmask is an unsigned 64-bit integer (`uint64`):

| Bit | Permission Constant | Description |
| :--- | :--- | :--- |
| `0` | `PermRead` | Read files and list folder contents |
| `1` | `PermWrite` | Upload files and update metadata |
| `2` | `PermCreateFolder` | Create child subfolders |
| `3` | `PermMoveFolder` | Reposition folders within hierarchy |
| `4` | `PermDeleteFolder` | Delete or soft-delete folder nodes |
| `5` | `PermDeleteFile` | Remove file and version nodes |
| `28` | `PermSetPermissions` | Set permissions on target folder |
| `63` | `PermAdmin` | Super-admin access, bypasses all blocks |
