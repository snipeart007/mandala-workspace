-- PostgreSQL Schema for Mandala Workspace

-- 1. Create Tables
CREATE TABLE IF NOT EXISTS folders (
    folder_id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    parent_folder_id INTEGER REFERENCES folders(folder_id),
    path TEXT NOT NULL,
    inheritance BOOLEAN DEFAULT TRUE,
    version_retention INTEGER DEFAULT 0,
    metadata BYTEA,
    merkle_root BYTEA,
    created_at BIGINT NOT NULL,
    deleted_at BIGINT
);

CREATE TABLE IF NOT EXISTS users (
    user_id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash BYTEA NOT NULL,
    metadata BYTEA,
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS devices (
    device_id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    public_key BYTEA NOT NULL,
    metadata BYTEA,
    created_at BIGINT NOT NULL,
    revoked_at BIGINT
);

CREATE TABLE IF NOT EXISTS files (
    file_id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    folder_id INTEGER NOT NULL REFERENCES folders(folder_id),
    metadata BYTEA,
    path TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    location TEXT NOT NULL,
    version_id INTEGER, 
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT
);

CREATE TABLE IF NOT EXISTS versions (
    version_id SERIAL PRIMARY KEY,
    file_id INTEGER NOT NULL REFERENCES files(file_id),
    version TEXT NOT NULL,
    hash BYTEA NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    metadata BYTEA,
    version_comment TEXT,
    created_at BIGINT NOT NULL
);

-- Add foreign key constraint to files now that versions table exists
ALTER TABLE files ADD CONSTRAINT fk_files_version_id FOREIGN KEY (version_id) REFERENCES versions(version_id);

CREATE TABLE IF NOT EXISTS permissions (
    perm_id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    folder_id INTEGER NOT NULL REFERENCES folders(folder_id) ON DELETE CASCADE,
    metadata BYTEA,
    permissions BIGINT NOT NULL,
    UNIQUE(user_id, folder_id)
);

-- 2. Create Indexes
CREATE INDEX IF NOT EXISTS idx_files_folder_id ON files(folder_id);
CREATE INDEX IF NOT EXISTS idx_versions_file_id ON versions(file_id);
