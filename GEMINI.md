# Mandala Workspace - GEMINI Context

This repository is organized as a monorepo containing two separate software solutions: the Server and the Client. It uses a Go workspace (`go.work`) to manage these components.

## Repository Structure

- **`proto/`**: Shared Protobuf definitions for the gRPC API.
- **`server/`**: The backend gRPC service (Go module `mandala-workspace`).
- **`client/`**: The desktop client implementation (Go module `mandala-workspace/client`).
- **`go.work`**: Go workspace configuration managing both components.

### Shared API & Code Generation
- Proto definitions are centralized in the root `proto/` directory.
- **Strict Separation:** Each module (Server and Client) is responsible for its own code generation. Generated code is stored within each module's `gen/` directory to avoid cross-module dependency on generated artifacts.

## Server Overview (`server/`)

The server implements a secure file storage and user management service. It uses a challenge-response authentication mechanism for devices and a granular bitmask-based permission system for folder-level access control.

### Main Technologies (Server)
- **Language:** Go 1.26.2
- **API Framework:** gRPC with Protobuf
- **Database:** PostgreSQL (Primary Production). SQLite is deprecated and used only for integration testing.
- **Caching:** Redis (Hard Dependency for sessions and permissions).
- **Authentication:** PASETO, Argon2id, Ed25519.
- **Permissions:** Custom bitmask-based system (`uint64`).
- **High-Performance Caching:** Uses Redis to cache effective permissions with hierarchical versioning (`eff_perm:{user_id}:{folder_id}:{v}`) and an incremental update queue (`perm_updates:{user_id}`).

## Implementation Roadmap
Detailed plans for the expansion of the API and the caching system can be found in:
- `server/Gemini-Plan-Extension-1.md`: Detailed RPC and logic definitions.
- `server/Gemini-Plan-Chronology.md`: Granular phase-by-phase implementation roadmap.

### Server Architecture
- **`server/gen/`**: Contains generated Go code for the server.
- **`server/internal/`**: Implements core logic (crypto, db, grpc, permission, storage).
- **`server/cmd/server/`**: Entry point for the gRPC server.

## Client Overview (`client/`)

The client is a desktop application providing a native file-explorer-like interface for the Mandala Workspace.

### Main Technologies (Client)
- **Framework:** Wails (v2) with Go backend and React frontend.
- **Frontend Stack:** React, Material UI, Zustand.
- **Security:** Password-protected local keyring using Argon2id for KDF and AES-256-GCM for encryption.

### Client Architecture
- **`client/gen/`**: Contains generated Go code for the client.
- **`client/frontend/`**: React-based frontend application.
- **`client/app.go`**: Wails application entry point and bindings.

## Building and Running

The project includes a root `Makefile` that delegates to component-specific tasks:

- **Setup Environment:** `make setup`
- **Generate All Proto Code:** `make server-proto client-proto`
- **Run Server Tests:** `make server-test`
- **Run Server:** `make server-run`
- **Run All Tests:** `make test`

## Development Conventions

- **Surgical Updates:** Prefer targeted changes to specific files over broad refactoring.
- **Validation:** Always run tests (`make test`) after making changes.
- **Database Schema:** Data model changes must be reflected in both SQLite and PostgreSQL schemas within `server/internal/db/sql/`.
- **Modular DB:** All database operations must be implemented via the `DBProvider` interface.
# The gopls MCP server

These instructions describe how to efficiently work in the Go programming language using the gopls MCP server. You can load this file directly into a session where the gopls MCP server is connected.

## Detecting a Go workspace

At the start of every session, you MUST use the `go_workspace` tool to learn about the Go workspace. ONLY if you are in a Go workspace, you MUST run `go_vulncheck` immediately afterwards to identify any existing security risks. The rest of these instructions apply whenever that tool indicates that the user is in a Go workspace.

## Go programming workflows

These guidelines MUST be followed whenever working in a Go workspace. There are two workflows described below: the 'Read Workflow' must be followed when the user asks a question about a Go workspace. The 'Edit Workflow' must be followed when the user edits a Go workspace.

You may re-do parts of each workflow as necessary to recover from errors. However, you must not skip any steps.

### Read workflow

The goal of the read workflow is to understand the codebase.

1. **Understand the workspace layout**: Start by using `go_workspace` to understand the overall structure of the workspace, such as whether it's a module, a workspace, or a GOPATH project.

2. **Find relevant symbols**: If you're looking for a specific type, function, or variable, use `go_search`. This is a fuzzy search that will help you locate symbols even if you don't know the exact name or location.
   EXAMPLE: search for the 'Server' type: `go_search({"query":"server"})`

3. **Understand a file and its intra-package dependencies**: When you have a file path and want to understand its contents and how it connects to other files *in the same package*, use `go_file_context`. This tool will show you a summary of the declarations from other files in the same package that are used by the current file. `go_file_context` MUST be used immediately after reading any Go file for the first time, and MAY be re-used if dependencies have changed.
   EXAMPLE: to understand `server.go`'s dependencies on other files in its package: `go_file_context({"file":"/path/to/server.go"})`

4. **Understand a package's public API**: When you need to understand what a package provides to external code (i.e., its public API), use `go_package_api`. This is especially useful for understanding third-party dependencies or other packages in the same monorepo.
   EXAMPLE: to see the API of the `storage` package: `go_package_api({"packagePaths":["example.com/internal/storage"]})`

### Editing workflow

The editing workflow is iterative. You should cycle through these steps until the task is complete.

1. **Read first**: Before making any edits, follow the Read Workflow to understand the user's request and the relevant code.

2. **Find references**: Before modifying the definition of any symbol, use the `go_symbol_references` tool to find all references to that identifier. This is critical for understanding the impact of your change. Read the files containing references to evaluate if any further edits are required.
   EXAMPLE: `go_symbol_references({"file":"/path/to/server.go","symbol":"Server.Run"})`

3. **Make edits**: Make the required edits, including edits to references you identified in the previous step. Don't proceed to the next step until all planned edits are complete.

4. **Check for errors**: After every code modification, you MUST call the `go_diagnostics` tool. Pass the paths of the files you have edited. This tool will report any build or analysis errors.
   EXAMPLE: `go_diagnostics({"files":["/path/to/server.go"]})`

5. **Fix errors**: If `go_diagnostics` reports any errors, fix them. The tool may provide suggested quick fixes in the form of diffs. You should review these diffs and apply them if they are correct. Once you've applied a fix, re-run `go_diagnostics` to confirm that the issue is resolved. It is OK to ignore 'hint' or 'info' diagnostics if they are not relevant to the current task. Note that Go diagnostic messages may contain a summary of the source code, which may not match its exact text.

6. **Check for vulnerabilities**: If your edits involved adding or updating dependencies in the go.mod file, you MUST run a vulnerability check on the entire workspace. This ensures that the new dependencies do not introduce any security risks. This step should be performed after all build errors are resolved. EXAMPLE: `go_vulncheck({"pattern":"./..."})`

7. **Run tests**: Once `go_diagnostics` reports no errors (and ONLY once there are no errors), run the tests for the packages you have changed. You can do this with `go test [packagePath...]`. Don't run `go test ./...` unless the user explicitly requests it, as doing so may slow down the iteration loop.


