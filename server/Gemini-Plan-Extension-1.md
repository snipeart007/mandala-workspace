# Plan: Permission Caching, Synchronization, and Comprehensive API Expansion

## Objective
Implement a robust permission caching and synchronization mechanism and expand the Mandala Workspace API into a comprehensive, enterprise-grade suite. This includes a Redis-based server cache, a client-side polling mechanism (`PermUpdates`), and a full set of management, audit, and administrative RPCs.

## Scope & Impact
- **Proto Definitions**: Massive expansion of `user_service.proto`, `folder_service.proto`, and `file_service.proto`.
- **Permission Bitmask**: Introduce `PermSetPermissions` (bit 28) for granular management.
- **Server DB Layer**: Implement `CachedDBProvider` (wrapper) and `CacheManager` (Redis).
- **Server Services**: Implement ~40+ new RPCs across all three services with strict authorization.
- **Database Architecture**: **PostgreSQL is now the primary production database.** SQLite is deprecated and reserved strictly for integration testing. All metadata columns are now treated as `JSONB`.
- **System Dependencies**: **Redis is a hard dependency.** If Redis is down, the application will not function.

---

## 1. Proto Definitions Expansion

### A. User Service (`user_service.proto`)
#### Identity & Profile
- `GetMyProfile()`: Returns current user profile.
- `UpdateProfile(name, json_metadata)`: Updates current user info.
- `ChangePassword(old, new)`: Updates Argon2id hash.
#### Device & Session Management
- `ListMyDevices()`: Returns all devices for the user.
- `UpdateDeviceMetadata(device_id, name, json_metadata)`: Renames/updates device info.
- `ListUserSessions()`: Shows all active PASETO sessions.
- `TerminateSession(session_id)`: Invalidates a specific session.
#### Administration (Admin/Root Only)
- `ListAllUsers(pagination)`: Paginated user list.
- `GetUserByID(user_id)`: Detailed profile of any user.
- `UpdateUserStatus(user_id, status)`: Deactivate/Reactivate accounts.
- `InviteUser()`: **[TODO/Unimplemented]**
- `SetUserQuota(user_id, quota_bytes)`: Cap storage usage.
#### Security & Compliance
- `SetupMFA()` / `VerifyMFA()`: TOTP setup/validation.
- `GetAuditLogs(filter)`: **[TODO/Unimplemented]**
- `GetActivityFeed()`: **[TODO/Unimplemented]**
- `ExportMyData()`: GDPR-compliant data export.
#### Permission Sync & Management (Previously Planned)
- `GetAllPermissions()`: Initial sync.
- `PermUpdates()`: Incremental polling.
- `PermCheck(folder_id)`: Self-check.
- `SetUserPermission(target_user_id, folder_id, bitmask)`: Management.
- `BatchSetPermissions(folder_id, map<user_id, bitmask>)`: Bulk update.
- `RevokeUserPermissions(target_user_id, folder_id)`: Removal.
- `GetUserEffectivePermissionsForAudit(target_user_id, folder_id)`: Audit.

### B. Folder Service (`folder_service.proto`)
#### Navigation & Metadata
- `GetFolder(folder_id)`: Metadata for one folder (no children).
- `UpdateFolder(folder_id, name, json_metadata, inheritance)`: Update settings.
- `CalculateFolderSize(folder_id)`: Recursive size calculation.
#### Recycle Bin & Maintenance
- `ListDeletedFolders()`: View soft-deleted folders.
- `RestoreFolder(folder_id)`: Undelete.
- `PermanentDeleteFolder(folder_id)`: (Admin) Wipe from DB.
#### Batch Operations
- `BatchDeleteFolders(folder_ids)`: Bulk soft-delete.

### C. File Service (`file_service.proto`)
#### Metadata & Versioning
- `GetFileMetadata(file_id)`: Current state and hash.
- `RestoreFileVersion(version_id)`: Promote old version to active.
- `DeleteFileVersion(version_id)`: Purge specific history.
#### Recycle Bin & Recovery
- `RestoreFile(file_id)`: Recover from soft-delete.
- `PermanentDeleteFile(file_id)`: (Admin) Wipe from DB.
#### Content & Collaboration
- `CopyFile(file_id, target_folder_id)`: Duplicate file (CAS-efficient).
- `LockFile(file_id)`: Advisory locking (TTL: 30 minutes in Redis).
- `UnlockFile(file_id)`: Remove lock.
- `RefreshLock(file_id)`: Reset the 30-minute TTL.
- `VerifyFileIntegrity(file_id)`: Trigger server-side re-hash.
#### Search & Batch
- `Search(query, filters)`: Unified file/folder search using PostgreSQL `JSONB` indexing.
- `SearchAdvanced(complex_filters)`: Size/Date/Metadata filtering.
- `BatchMoveFiles(file_ids, target_folder_id)`: Bulk move.
- `BatchDeleteFiles(file_ids)`: Bulk soft-delete.
- `BatchDownload(file_ids, folder_ids)`: **[Streaming]** Server streams file metadata messages followed by the raw file chunks for each item.

---

## 2. Server Architecture & Authorization

### A. Redis CacheManager & Dependency
- **Hard Dependency**: The application will fail to start if Redis is unreachable.
- **Hierarchical Invalidation**:
    - Effective permissions will be cached with a versioned prefix: `eff_perm:{user_id}:{folder_id}:{v}`.
    - Each folder/user combination has a `version` key in Redis. Incrementing a parent's version key invalidates all cached descendant permissions for that user in $O(1)$.
- **Session Cache**: `session:{user_id}:{device_id}` with `LoginExpiry` TTL.
- **Advisory Locks**: `lock:file:{file_id}` with a 30-minute TTL.

### B. Database Logic
- **JSONB Implementation**: All `metadata` columns in PostgreSQL will be migrated to `JSONB` to support efficient indexing and searching.
- **Quota Enforcement**: A `user_quotas` table will store `used_bytes`. The `CachedDBProvider` (or DB triggers) will update this atomically during file creation and deletion.
- **Audit Logs**: An `audit_logs` table will be implemented with an auto-pruning policy for compliance tracking.

### C. Authorization Logic
For all sensitive RPCs, the server verifies:
1. **Management/Audit**: Caller has `PermSetPermissions` on `folder_id` **OR** `PermAdmin` on root (ID 1).
2. **Administrative**: Caller has `PermAdmin` on root (ID 1).
3. **Data Access**: Standard `PermRead`/`PermWrite` checks on target IDs.

---

## 3. Implementation Phases
1. **Proto & Generation**: Update all `.proto` files and run `make server-proto client-proto`.
2. **PostgreSQL Migration**: Migrate schema to Postgres, converting `BLOB` metadata to `JSONB`.
3. **Cache Infrastructure**: Setup Redis connection and `CacheManager`.
4. **DB Wrapper**: Implement `CachedDBProvider` for invalidation and quota updates.
5. **Service Implementation**:
   - Phase A: Permissions & Sync (Highest Priority).
   - Phase B: Recycle Bin & Restore Logic.
   - Phase C: Batch Operations (excluding BatchMoveFolders) & Admin Tools.
   - Phase D: Search & Compliance Features.
6. **Validation**: Test suite using SQLite for integration tests and Postgres for production readiness.
