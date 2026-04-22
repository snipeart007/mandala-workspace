# Plan: Permission Caching, Synchronization, and Comprehensive API Expansion

## Objective
Implement a robust permission caching and synchronization mechanism and expand the Mandala Workspace API into a comprehensive, enterprise-grade suite. This includes a Redis-based server cache, a client-side polling mechanism (`PermUpdates`), and a full set of management, audit, and administrative RPCs.

## Scope & Impact
- **Proto Definitions**: Massive expansion of `user_service.proto`, `folder_service.proto`, and `file_service.proto`.
- **Permission Bitmask**: Introduce `PermSetPermissions` (bit 28) for granular management.
- **Server DB Layer**: Implement `CachedDBProvider` (wrapper) and `CacheManager` (Redis).
- **Server Services**: Implement ~40+ new RPCs across all three services with strict authorization.
- **Client**: Update synchronization logic and local caching for permissions.

---

## 1. Proto Definitions Expansion

### A. User Service (`user_service.proto`)
#### Identity & Profile
- `GetMyProfile()`: Returns current user profile.
- `UpdateProfile(name, metadata)`: Updates current user info.
- `ChangePassword(old, new)`: Updates Argon2id hash.
#### Device & Session Management
- `ListMyDevices()`: Returns all devices for the user.
- `UpdateDeviceMetadata(device_id, name, metadata)`: Renames/updates device info.
- `ListUserSessions()`: Shows all active PASETO sessions.
- `TerminateSession(session_id)`: Invalidates a specific session.
#### Administration (Admin/Root Only)
- `ListAllUsers(pagination)`: Paginated user list.
- `GetUserByID(user_id)`: Detailed profile of any user.
- `UpdateUserStatus(user_id, status)`: Deactivate/Reactivate accounts.
- `InviteUser(email, name, role)`: Create pending account.
- `SetUserQuota(user_id, quota_bytes)`: Cap storage usage.
#### Security & Compliance
- `SetupMFA()` / `VerifyMFA()`: TOTP setup/validation.
- `GetAuditLogs(filter)`: Access trail for files/folders.
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
- `UpdateFolder(folder_id, name, metadata, inheritance)`: Update settings.
- `CalculateFolderSize(folder_id)`: Recursive size calculation.
#### Recycle Bin & Maintenance
- `ListDeletedFolders()`: View soft-deleted folders.
- `RestoreFolder(folder_id)`: Undelete.
- `PermanentDeleteFolder(folder_id)`: (Admin) Wipe from DB.
#### Batch Operations
- `BatchMoveFolders(folder_ids, target_parent_id)`: Bulk move.
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
- `LockFile(file_id)` / `UnlockFile(file_id)`: Advisory locking.
- `VerifyFileIntegrity(file_id)`: Trigger server-side re-hash.
#### Search & Batch
- `Search(query, filters)`: Unified file/folder search.
- `SearchAdvanced(complex_filters)`: Size/Date/Metadata filtering.
- `BatchMoveFiles(file_ids, target_folder_id)`: Bulk move.
- `BatchDeleteFiles(file_ids)`: Bulk soft-delete.
- `BatchDownload(file_ids, folder_ids)`: Request server-side ZIP/Tar.

### D. System & Utility
- `GetUsageStats()`: Storage breakdown for caller.
- `GetServerInfo()`: Health, version, and disk stats.
- `ListStorageProviders()`: (Admin) Backend visibility.
- `MigrateStorage(filter, target_provider)`: (Admin) Backend migration.
- `RunMaintenance(task_type)`: (Admin) GC or Merkle check.
- `GetSystemConfig()` / `UpdateSystemConfig()`: (Admin) Global limits.
- `GetActivityFeed(pagination)`: Event stream of modifications.
- `SubscribeToFolder(folder_id)`: Register for change notifications.

---

## 2. Server Architecture & Authorization

### A. Redis CacheManager
- `eff_perm:{user_id}:{folder_id}`: Effective permission cache.
- `perm_updates:{user_id}`: Hash of pending changes for polling.

### B. Authorization Logic
For all sensitive RPCs, the server verifies:
1. **Management/Audit**: Caller has `PermSetPermissions` on `folder_id` **OR** `PermAdmin` on root (ID 1).
2. **Administrative**: Caller has `PermAdmin` on root (ID 1).
3. **Data Access**: Standard `PermRead`/`PermWrite` checks on target IDs.

---

## 3. Implementation Phases
1. **Proto & Generation**: Update all `.proto` files and run `make server-proto client-proto`.
2. **Cache Infrastructure**: Setup Redis connection and `CacheManager`.
3. **DB Wrapper**: Implement `CachedDBProvider` to handle invalidation logic.
4. **Service Implementation**:
   - Phase A: Permissions & Sync (Highest Priority).
   - Phase B: Recycle Bin & Restore Logic.
   - Phase C: Batch Operations & Administrative Tools.
   - Phase D: Search & Compliance Features.
5. **Validation**: Test suite for each new RPC, ensuring authorization boundaries are impenetrable.

### 5. Client Integration
- Implement an initial fetch via `GetAllPermissions` upon login.
- Store permissions in a client-side cache (e.g., in `Zustand` or an internal Go map in the Wails backend).
- Setup a background worker to call `PermUpdates` every N seconds, applying the received changes to the local cache.

### 6. Documentation Updates
- **`Gemini.md`**: Add architecture notes about Redis and the CachedDBProvider wrapper pattern.
- **`Client-RPC.md` & `Client-Plan.md`**: Detail the polling strategy for `PermUpdates` and the structure of the local cache.
- **`Gemini-Plan.md`**: Outline the Redis implementation steps.

## Migration & Rollback
- Since Redis acts purely as a cache, the system can gracefully fallback to direct DB queries if Redis is unavailable or if cache inconsistencies occur.
- Add configuration for Redis connection string in `ServerInstanceConfig`. If empty, the caching layer can act as a pass-through or use a simple in-memory map.
