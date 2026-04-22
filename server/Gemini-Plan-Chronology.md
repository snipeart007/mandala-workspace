# Mandala Workspace: Granular Implementation Chronology

This document outlines the step-by-step roadmap for expanding the Mandala Workspace API and implementing the Permission Caching System.

---

## Phase 1: Foundational Infrastructure (The Bedrock)
*Goal: Prepare the codebase for massive expansion and integrate the caching layer.*

### Sub-Phase 1.1: Protocol Buffers Expansion
- [ ] Update `proto/mandala/v1/user_service.proto` with new RPCs (InviteUser as stub).
- [ ] Update `proto/mandala/v1/folder_service.proto` (Remove BatchMoveFolders).
- [ ] Update `proto/mandala/v1/file_service.proto` (Add BatchDownload streaming support).
- [ ] Run `make server-proto` and `make client-proto`.

### Sub-Phase 1.2: PostgreSQL Migration & Schema
- [ ] Migrate `InitializeDB.sql` to standard PostgreSQL (jsonb metadata).
- [ ] Implement `user_quotas` table for atomic tracking.
- [ ] Implement `audit_logs` table (stubbed/empty for now).
- [ ] Deprecate SQLite in production config.

### Sub-Phase 1.3: Redis Core (Hard Dependency)
- [ ] Implement `server/internal/cache/cache_manager.go`.
- [ ] Setup Redis connection (Server fails to start if connection fails).
- [ ] Implement hierarchical versioning logic for $O(1)$ invalidation.
- [ ] Implement Redis-backed Session Management (TTL supported).

### Sub-Phase 1.4: Database Wrapper (CachedDBProvider)
- [ ] Create `server/internal/db/cached_db.go`.
- [ ] Implement versioned cache lookups for effective permissions.
- [ ] Implement atomic quota updates in the wrapper.

---

## Phase 2: Permission Synchronization & Management (Critical)
*Goal: Establish the high-performance permission sync needed for the client.*

### Sub-Phase 2.1: Retrieval & Sync
- [ ] Implement `GetAllPermissions`.
- [ ] Implement `PermUpdates` (Redis hash polling).
- [ ] Implement `PermCheck` (Self-check).

### Sub-Phase 2.2: Management RPCs
- [ ] Implement `SetUserPermission` (with hierarchical version increment).
- [ ] Implement `BatchSetPermissions`.
- [ ] Implement `RevokeUserPermissions`.
- [ ] Implement `GetUserEffectivePermissionsForAudit`.

---

## Phase 3: Identity & Security
*Goal: Secure user lifecycle and MFA.*

### Sub-Phase 3.1: Profile & MFA
- [ ] Implement `GetMyProfile` and `UpdateProfile`.
- [ ] Implement `ChangePassword`.
- [ ] Implement `SetupMFA` and `VerifyMFA` (TOTP).

### Sub-Phase 3.2: Device & Session Control
- [ ] Implement `ListMyDevices` and `UpdateDeviceMetadata`.
- [ ] Implement `ListUserSessions` and `TerminateSession`.

---

## Phase 4: Data Recovery & Version Control
*Goal: Implement the "Recycle Bin".*

### Sub-Phase 4.1: Recycle Bin
- [ ] Implement `ListDeletedFolders`.
- [ ] Implement `RestoreFolder` & `RestoreFile`.
- [ ] Implement `PermanentDeleteFolder` & `PermanentDeleteFile` (Admin only).

### Sub-Phase 4.2: Versioning
- [ ] Implement `DeleteFileVersion` & `RestoreFileVersion`.

---

## Phase 5: Navigation, Collaboration & Search
*Goal: Professional workflow tools.*

### Sub-Phase 5.1: Folder/File Metadata
- [ ] Implement `GetFolder` & `GetFileMetadata`.
- [ ] Implement `UpdateFolder` (Rename/Inheritance).
- [ ] Implement `CalculateFolderSize` (Recursive sum).

### Sub-Phase 5.2: Collaboration & Locking
- [ ] Implement `LockFile`, `UnlockFile`, and `RefreshLock` (Redis TTL: 30m).
- [ ] Implement `CopyFile` & `VerifyFileIntegrity`.

### Sub-Phase 5.3: Discovery
- [ ] Implement `Search` and `SearchAdvanced` (Leveraging Postgres JSONB indexes).

---

## Phase 6: Bulk Operations & Compliance
*Goal: Optimize for scale.*

### Sub-Phase 6.1: Batch Operations
- [ ] Implement `BatchDeleteFolders`.
- [ ] Implement `BatchMoveFiles` & `BatchDeleteFiles`.
- [ ] Implement `BatchDownload` (ZIP Streaming).

### Sub-Phase 6.2: Compliance & GDPR
- [ ] Implement `ExportMyData`.
- [ ] Implement Audit/Activity stubs (marked TODO).

---

## Phase 7: System Administration (Admin Only)
*Goal: Instance-wide management.*

### Sub-Phase 7.1: User Governance
- [ ] Implement `ListAllUsers`, `GetUserByID`, `UpdateUserStatus`.
- [ ] Implement `InviteUser` (Stub).
- [ ] Implement `SetUserQuota` & `GetUsageStats`.

### Sub-Phase 7.2: Orchestration & Maintenance
- [ ] Implement `ListStorageProviders` & `MigrateStorage`.
- [ ] Implement `RunMaintenance` (Triggering DB Vacuum/GC).
- [ ] Implement `GetSystemConfig` & `UpdateSystemConfig`.
- [ ] Implement `GetServerInfo`.
