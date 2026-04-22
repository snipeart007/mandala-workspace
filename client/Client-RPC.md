# Mandala Workspace: Client RPC Implementation Guide

This guide provides a comprehensive overview for client-side developers integrating with the Mandala Workspace gRPC API.

## 1. Core Concepts

### Authentication & Sessions
Mandala uses a challenge-response authentication mechanism tied to registered devices.
1.  **Challenge:** The client requests a challenge from the server.
2.  **Signature:** The client signs the challenge using the device's private Ed25519 key.
3.  **Token:** The server verifies the signature and issues a stateless **PASETO** token.
4.  **Authorized Requests:** All subsequent requests must include this token in the `Authorization` header.

### Authorization (RBAC/ABAC)
Access is controlled by a bitmask-based permission system. Permissions are inherited down the folder hierarchy unless an inheritance break is explicitly set. Key permissions include `PermRead`, `PermWrite`, `PermCreateFolder`, `PermAdmin`, etc.

---

## 2. Authentication Flow

### Step 1: Request Challenge
**Service:** `UserService`
**Method:** `LoginUser`

- **Request:** `LoginUserRequest { user_id, device_id }`
- **Response:** `LoginUserChallenge { user_id, device_id, timestamp }`

### Step 2: Sign and Verify
1.  Marshal the `LoginUserChallenge` message using deterministic Protobuf encoding.
2.  Sign the resulting bytes with the device's Ed25519 private key.
3.  Send the signature back to the server.

**Service:** `UserService`
**Method:** `VerifyLoginSignature`

- **Request:** `LoginUserSignatureRequest { user_id, device_id, timestamp, signature }`
- **Response:** `LoginUserTokenResponse { token }`

### Step 3: Use the Token
Include the token in the gRPC metadata for all authenticated calls:
```text
Authorization: Bearer <token>
```

---

## 3. User & Device Management (`UserService`)

### `CreateUser`
- **Requirement:** `PermUserCreate` on the root folder (ID 1).
- **Usage:** Used by administrators to provision new users.

### `RegisterDevice`
- **Requirement:** `PermDeviceSetup` on the root folder.
- **Usage:** Associates an Ed25519 public key with a user account.

### `RevokeDevice`
- **Requirement:** `PermAdmin`.
- **Usage:** Immediately invalidates a device's ability to login and clears active sessions.

---

## 4. Workspace Hierarchy (`FolderService`)

### `CreateFolder`
- **Requirement:** `PermCreateFolder` on the parent.
- **Note:** You can toggle `inheritance` (default: true) to control permission flow.

### `ListFolder`
- **Requirement:** `PermRead`.
- **Response:** Returns immediate subfolders and files. Useful for building a tree view.

### `MoveFolder` / `DeleteFolder`
- **Requirement:** `PermMoveFolder` / `PermDeleteFolder`.
- **Note:** Operations are recursive. Moving or deleting a folder affects all descendants.

---

## 5. File Operations (`FileService`)

### Uploading a New File (`UploadFile`)
This is a **Client-to-Server Stream**.
1.  The first message **MUST** be `UploadMetadata`.
2.  Subsequent messages contain `bytes chunk`.
3.  The server aggregates the stream, calculates the content hash (SHA-256), and stores it in the CAS.

### Downloading a File (`DownloadFile`)
This is a **Server-to-Client Stream**.
- **Request:** Specify `file_id` and optionally `version_id` (defaults to latest).
- **Stream:** The server sends back `bytes chunk`.

### Versioning (`UploadVersion`)
- Similar to `UploadFile`, but targets an existing `file_id`.
- Every upload creates a new immutable version.

### Retention Policy (`SetRetentionPolicy`)
- **Usage:** Controls how many historical versions are kept for files within a folder.
- **Example:** Setting `last_n_versions = 5` will automatically prune the 6th oldest version upon a new upload.

---

## 6. Error Handling

Mandala uses standard gRPC status codes:

| Code | Meaning | Client Action |
| :--- | :--- | :--- |
| `OK` (0) | Success | Proceed. |
| `INVALID_ARGUMENT` (3) | Bad request parameters. | Fix request payload. |
| `UNAUTHENTICATED` (16) | Missing/invalid token or signature failure. | Re-run Login flow. |
| `PERMISSION_DENIED` (7) | Insufficient permissions for the resource. | Check user rights. |
| `NOT_FOUND` (5) | File, Folder, or Device does not exist. | Check IDs. |
| `INTERNAL` (13) | Server-side error (DB, Storage, etc.). | Retry with exponential backoff. |

---

## 7. Implementation Tips

- **Deterministic Marshaling:** When signing the login challenge, ensure your Protobuf library uses deterministic encoding to match the server's byte representation.
- **Chunk Size:** For streaming uploads/downloads, a chunk size of **64KB to 1MB** is recommended for optimal performance.
- **Context Deadlines:** Set reasonable gRPC deadlines (e.g., 30s for metadata, longer for large file streams) to prevent hanging connections.
