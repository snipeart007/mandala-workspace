-- 1. Set Journaling Mode
PRAGMA journal_mode = WAL;

-- 2. Create Tables
CREATE TABLE IF NOT EXISTS folders (
    folder_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    parent_folder_id INTEGER,
    path TEXT NOT NULL,
    metadata BLOB,
    merkle_root BLOB,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER,
    FOREIGN KEY(parent_folder_id) REFERENCES folders(folder_id)
);

CREATE TABLE IF NOT EXISTS users (
    user_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash BLOB NOT NULL,
    metadata BLOB,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS devices (
    device_id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    public_key BLOB NOT NULL,
    metadata BLOB,
    created_at INTEGER NOT NULL,
    revoked_at INTEGER,
    FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS files (
    file_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    folder_id INTEGER NOT NULL,
    metadata BLOB,
    path TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    location TEXT NOT NULL,
    -- Removed NOT NULL to allow initial creation before the first version exists
    version_id INTEGER, 
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER,
    FOREIGN KEY(folder_id) REFERENCES folders(folder_id),
    FOREIGN KEY(version_id) REFERENCES versions(version_id)
);

CREATE TABLE IF NOT EXISTS versions (
    version_id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER NOT NULL,
    version TEXT NOT NULL,
    hash BLOB NOT NULL,
    user_id INTEGER NOT NULL,
    metadata BLOB,
    version_comment TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files(file_id),
    FOREIGN KEY(user_id) REFERENCES users(user_id)
);

CREATE TABLE IF NOT EXISTS permissions (
    perm_id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    folder_id INTEGER NOT NULL,
    metadata BLOB,
    permissions INTEGER NOT NULL,
    UNIQUE(user_id, folder_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (folder_id) REFERENCES folders(folder_id) ON DELETE CASCADE
);

-- 3. Create Indexes (Stand-alone commands)
CREATE INDEX IF NOT EXISTS idx_files_folder_id ON files(folder_id);
CREATE INDEX IF NOT EXISTS idx_versions_file_id ON versions(file_id);

-- 4. Enable Foreign Key Enforcement at the end
PRAGMA foreign_keys = ON;