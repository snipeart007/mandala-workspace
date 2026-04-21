# Plan: Workspace Filesystem & Access Control (Gemini-Plan)

This plan outlines the server-side implementation for an automatically versioned filesystem with inherited access control for the Mandala workspace. We will implement this in small, incremental steps.

## Objective
Implement a gRPC-based workspace service that supports hierarchical folder management, content-addressable file versioning, and inherited permissions.

## Key Files & Context
- `proto/mandala/v1/workspace_service.proto`: (New) Service definitions for files and folders.
- `internal/permission/permission_manager.go`: Update to support path-based inheritance.
- `internal/storage/`: (New) Modular CAS storage system.
- `internal/grpc/workspace_service/`: (New) Service implementation.
- `internal/db/sql/InitializeDB.sql`: Schema reference for files, folders, and versions.

## Implementation Steps

### Phase 1: Storage Layer (Modular CAS) (Completed)
1. **Define `internal/storage/interface.go`**:
   - Create `CASProvider` interface with `Store`, `Retrieve`, `Exists`, `GetLocationType`, and `Delete`.
2. **Implement `CASRegistry` in `internal/storage/registry.go`**:
   - A central registry that routes requests to the correct implementation based on the `location` column in the database.
3. **Implement `LocalStorage` in `internal/storage/local.go`**:
   - Implements `CASProvider`.
   - Uses **two-level sharding** (e.g., `storage/ab/cd/hash_remnants`) to handle millions of files.
   - Performs **Integrity Checks** (re-hashing) on every `Retrieve`.

### Phase 2: Permission Inheritance & Break Logic (Completed)
1. **Update Database Schema**:
   - Add `inheritance BOOLEAN DEFAULT TRUE` to the `folders` table.
2. **Update `internal/db/db.go`**:
   - Implemented `GetUserEffectivePermissions(userID uint64, folderID uint64)` with path-based prefix search and inheritance break logic.
   - Added support for `PermAdmin` override (skips inheritance breaks).
   - Ensured all permission checks respect the `deleted_at IS NULL` soft-delete state.

### Phase 3: Folder Management (Completed)
1. **Define `folder_service.proto`**:
   - Defined `FolderService` with RPCs: `CreateFolder`, `ListFolder`, `MoveFolder`, `DeleteFolder`.
2. **Implement Database Operations in `internal/db/db.go`**:
   - **Soft-Delete**: All operations now include `deleted_at IS NULL` checks. `SoftDeleteFolder` recursively marks folders and files as deleted.
   - **Path Management**: `CreateFolder` computes hierarchical `path` strings; `MoveFolder` recursively updates path prefixes for all descendants.
   - **Data Models**: Added `FolderModel` and `FileModel` in `internal/db/models.go`.
   - **Helper Methods**: Added `ListFolders`, `ListFiles`, and `CreateFile` to facilitate workspace management and testing.
3. **Implement `FolderService` in `internal/grpc/folder_service/`**:
   - Integrated `PermissionManager` for granular access control (`PermCreateFolder`, `PermRead`, `PermMoveFolder`, `PermDeleteFolder`).
   - Implemented inheritance break handling in `CreateFolder`: if `inheritance=false`, creator's effective permissions from the parent are explicitly assigned to the new folder.

### Phase 4: Streaming File Service & Automated Versioning (Completed)
**Goal:** Implement a memory-efficient, streaming-based file service with automated history and retention.

1.  **Sub-Phase 1: Foundation & Schema Updates**
    - Define `proto/mandala/v1/file_service.proto` with streaming RPCs (`UploadFile` client-stream, `DownloadFile` server-stream).
    - Update `internal/db/sql/InitializeDB.sql` to add `version_retention` (INT, default 0) to the `folders` table.
    - Expand `internal/db` models to support fetching folder retention and managing `versions` records.

2.  **Sub-Phase 2: Streaming File Service Implementation**
    - Implement `UploadFile` gRPC handler to pipe the incoming request stream directly into `CASProvider.Store`.
    - Implement `DownloadFile` to pipe `io.ReadCloser` from `CASProvider.Retrieve` directly into the gRPC response stream.
    - Ensure all data movement uses `io.Copy` or buffered readers (e.g., 32KB chunks) to maintain constant memory overhead regardless of file size.

3.  **Sub-Phase 3: Automated Versioning & Metadata**
    - Implement atomic "Upsert" logic: if a filename exists in a folder, create a new `versions` entry and update the `files.version_id` pointer.
    - Implement `ListVersions` RPC to query file history (timestamps, hashes, authors).

4.  **Sub-Phase 4: Retention Policy Enforcement**
    - Add `SetRetentionPolicy` RPC to update the `version_retention` limit on a folder.
    - Implement post-upload "Pruning" logic: if versions > `N` (where `N > 0`), delete the oldest `versions` records from the database.
    - Added `Delete` method to `CASProvider` to support future physical storage deletion via background GC tasks.

5.  **Sub-Phase 5: Integration & Stress Testing**
    - Validate with 2GB+ files to ensure stable memory usage (monitored via `pprof`).
    - Verify strict enforcement of the "N" limit after successive uploads.
    - Test recovery from interrupted streams (network disconnects) to prevent partial/corrupt version state.

### Phase 5: Merkle Tree Integration (Advanced)
1. **Implement `UpdateFolderMerkleRoot(folderID uint64)`**:
   - Recursively computes a hash based on the hashes of all files and subfolders within.
   - Updates the `merkle_root` in the `folders` table.
   - Triggered on any file or folder modification to ensure organizational integrity.

## Cross-Cutting: Logging & Observability
- **Standardized Logging**: Integrated `log/slog` across all layers (DB, Storage, gRPC, Crypto).
- **Security Visibility**: Added warnings for permission denials, authentication failures, and invalid signature attempts.
- **Operational Tracing**: Implemented info-level logging for major state changes (user creation, folder moves, device revocations, file storage).
- **Error Diagnosis**: Comprehensive error logging for database and I/O failures with relevant context (IDs, paths, etc.).

## Verification & Testing
1. **Unit Tests**:
   - `LocalStorage` sharding and integrity checks.
   - `GetUserEffectivePermissions` with inheritance breaks and soft-delete scenarios.
   - `FolderOperations` in DB (Move, Delete, Create).
2. **Integration Tests**:
   - `FolderService` gRPC implementation with end-to-end permission checks and authenticated contexts.
   - `FileService` gRPC implementation (Phase 4).

