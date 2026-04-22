Device:
1. device_id - int64
2. user - User
3. public_key - bytes
4. metadata - bytes
5. created_at - int64
6. revoked_at - int64

File:
1. file_id - int64
2. name - string
3. folder - Folder
4. metadata - bytes

5. path - string
   (12.435.3443)
6. storage_path - string
7. location - string
   (S3/Drive/Disk/etc.)

8. version - Version
9. created_at - int64
10. updated_at - int64
11. deleted_at - int64

Folder:
1. folder_id - int64
2. name - string
3. parent folder - Folder
4. path - string
   (12.435)
5. inheritance - boolean
6. version_retention - int32
7. metadata - bytes
8. merkle_root - bytes
9. created_at - int64
10. deleted_at - int64

Permissions:
1. perm_id - int64
2. user_id - int64
3. folder_id - int64
4. metadata - bytes
NOTE: UNIQUE (user_id, folder_id)
5. permissions - uint64 (bitmask)

User:
1. user_id - int64
2. name - string
3. email - string
4. password_hash - bytes
5. metadata - bytes
6. created_at - int64

Version:
1. version_id - int64
2. file_id - File
3. version - string
4. hash - bytes
5. user - User
6. metadata - bytes
7. version_comment - string
8. created_at - int64