# Mandala Workspace: Granular Implementation Chronology

This document outlines the step-by-step roadmap for expanding the Mandala Workspace API and implementing the Permission Caching System.

---

## Phase 1: Foundational Infrastructure (The Bedrock)
*Goal: Prepare the codebase for massive expansion and integrate the caching layer.*

### Sub-Phase 1.1: Protocol Buffers Expansion
- [ ] Update `proto/mandala/v1/user_service.proto` with ~25 new messages/RPCs.
- [ ] Update `proto/mandala/v1/folder_service.proto` with ~10 new messages/RPCs.
- [ ] Update `proto/mandala/v1/file_service.proto` with ~15 new messages/RPCs.
- [ ] Run `make server-proto` and `make client-proto` to update generated Go/TS code.

### Sub-Phase 1.2: Bitmask & Schema Preparation
- [ ] Add `PermSetPermissions` (bit 28) to `server/internal/permission/permission_bitmask.go`.
- [ ] Ensure DB schemas (SQLite/Postgres) support soft-deletion timestamps (`deleted_at`) for all tables.

### Sub-Phase 1.3: Redis Cache Integration
- [ ] Create `server/internal/cache/cache_manager.go`.
- [ ] Implement Redis connection logic and basic Get/Set/Del/Hash operations.
- [ ] Update `ServerInstanceConfig` to include Redis connection strings.

### Sub-Phase 1.4: Database Wrapper (CachedDBProvider)
- [ ] Create `server/internal/db/cached_db.go`.
- [ ] Implement the `CachedDBProvider` struct that wraps `db.DBProvider`.
- [ ] Initialize this wrapper in `server/internal/server/server.go`.

---

## Phase 2: Permission Synchronization & Management (Critical)
*Goal: Establish the high-performance permission sync needed for the client.*

### Sub-Phase 2.1: Retrieval & Verification
- [ ] Implement `GetAllPermissions` (Full user permission dump).
- [ ] Implement `PermCheck` (Lightweight self-check for a single folder).

### Sub-Phase 2.2: Incremental Updates (Redis Polling)
- [ ] Implement `PermUpdates` in `UserService`.
- [ ] Logic: Fetch from Redis `perm_updates:{user_id}`, then clear the hash.

### Sub-Phase 2.3: Invalidation Logic
- [ ] Update `CachedDBProvider.SetUserPermission` to:
    1. Call underlying DB.
    2. Invalidate `eff_perm:{user_id}:{folder_id}` in Redis.
    3. Push updated bitmask to `perm_updates:{user_id}`.
- [ ] Implement recursive invalidation for child folders when parent inheritance/permissions change.

### Sub-Phase 2.4: Management RPCs
- [ ] Implement `SetUserPermission` with authorization checks (Caller has `PermSetPermissions` on folder OR `PermAdmin` on root).
- [ ] Implement `BatchSetPermissions` for efficient bulk updates.
- [ ] Implement `RevokeUserPermissions` (clears explicit entries).

### Sub-Phase 2.5: Audit Capabilities
- [ ] Implement `GetUserEffectivePermissionsForAudit` (Managers checking other users' rights).

---

## Phase 3: Identity & Device Lifecycle
*Goal: Provide users control over their accounts and security.*

### Sub-Phase 3.1: Profile & Password
- [ ] Implement `GetMyProfile` and `UpdateProfile`.
- [ ] Implement `ChangePassword` (verifying old password hash before update).

### Sub-Phase 3.2: Device Visibility
- [ ] Implement `ListMyDevices`.
- [ ] Implement `UpdateDeviceMetadata` (Rename "My Phone" to "Work iPhone").

### Sub-Phase 3.3: Session Control
- [ ] Implement `ListUserSessions` (Retrieving active PASETO tokens from session manager).
- [ ] Implement `TerminateSession` (Immediate invalidation of a specific token).

### Sub-Phase 3.4: Multi-Factor Auth (MFA)
- [ ] Implement `SetupMFA` (Generate TOTP secret/QR).
- [ ] Implement `VerifyMFA` (Validate 6-digit code).

---

## Phase 4: Data Recovery & Version Control
*Goal: Implement the "Recycle Bin" and granular history management.*

### Sub-Phase 4.1: Folder Recycle Bin
- [ ] Implement `ListDeletedFolders` (Filters folders where `deleted_at` is not null).
- [ ] Implement `RestoreFolder` (Sets `deleted_at` back to null).

### Sub-Phase 4.2: File Recycle Bin
- [ ] Implement `RestoreFile`.
- [ ] Implement `PermanentDeleteFolder` & `PermanentDeleteFile` (Strictly Admin only).

### Sub-Phase 4.3: Granular Versioning
- [ ] Implement `DeleteFileVersion` (Remove a specific content address).
- [ ] Implement `RestoreFileVersion` (Promote an old version ID to be the active one in the `files` table).

---

## Phase 5: Enhanced Navigation & Collaboration
*Goal: Add professional workflow tools for files and folders.*

### Sub-Phase 5.1: Metadata & Statistics
- [ ] Implement `GetFolder` (Single metadata object).
- [ ] Implement `UpdateFolder` (Rename/Toggle Inheritance).
- [ ] Implement `GetFileMetadata`.
- [ ] Implement `CalculateFolderSize` (Recursive sum of file sizes).

### Sub-Phase 5.2: Integrity & Copying
- [ ] Implement `CopyFile` (New DB record, same CAS hash).
- [ ] Implement `VerifyFileIntegrity` (Server-side re-hash and comparison).

### Sub-Phase 5.3: Collaboration Locks
- [ ] Implement `LockFile` & `UnlockFile` (Advisory bit in DB to prevent concurrent overwrites).

---

## Phase 6: Bulk Operations & Discovery
*Goal: Optimize for scale and large-scale data organization.*

### Sub-Phase 6.1: Batch Folder Operations
- [ ] Implement `BatchMoveFolders`.
- [ ] Implement `BatchDeleteFolders`.

### Sub-Phase 6.2: Batch File Operations
- [ ] Implement `BatchMoveFiles`.
- [ ] Implement `BatchDeleteFiles`.
- [ ] Implement `BatchDownload` (Generate server-side ZIP stream).

### Sub-Phase 6.3: Unified Search
- [ ] Implement `Search` (Basic name/path matching).
- [ ] Implement `SearchAdvanced` (Filtering by size, date ranges, and metadata keys).

---

## Phase 7: Compliance & Notifications
*Goal: Meeting regulatory requirements and improving user awareness.*

### Sub-Phase 7.1: Audit & Activity
- [ ] Implement `GetAuditLogs` (Admin view of access logs).
- [ ] Implement `GetActivityFeed` (User-centric stream of "What happened lately?").

### Sub-Phase 7.2: Data Privacy (GDPR)
- [ ] Implement `ExportMyData` (JSON dump of all user-related records).

### Sub-Phase 7.3: Change Notifications
- [ ] Implement `SubscribeToFolder` (Registering interest in events).

---

## Phase 8: System Administration & Governance (Admin Only)
*Goal: Tools for managing the entire instance at scale.*

### Sub-Phase 8.1: User Governance
- [ ] Implement `ListAllUsers` (Paginated).
- [ ] Implement `GetUserByID`.
- [ ] Implement `UpdateUserStatus` (Ban/Deactivate).
- [ ] Implement `InviteUser` (Pending creation flow).

### Sub-Phase 8.2: Resource Quotas
- [ ] Implement `SetUserQuota`.
- [ ] Implement `GetUsageStats`.

### Sub-Phase 8.3: Storage Orchestration
- [ ] Implement `ListStorageProviders`.
- [ ] Implement `MigrateStorage` (Background task to move blobs between S3/Local/etc.).

### Sub-Phase 8.4: System Maintenance
- [ ] Implement `RunMaintenance` (Triggering DB Vacuum or CAS Garbage Collection).
- [ ] Implement `GetSystemConfig` & `UpdateSystemConfig`.
- [ ] Implement `GetServerInfo` (Health, Version, and Resource consumption).
