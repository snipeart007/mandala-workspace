# Mandala Desktop Client

A modern desktop file explorer and workspace manager for the Mandala Workspace. It is built on the **Wails v2** framework, combining a high-performance **Go** backend engine with a sleek **React / TypeScript / Material UI** desktop interface.

---

## 🎨 Technology Stack & Architecture

```
                                  [ Desktop Window ]
                                           │
                             ┌─────────────▼─────────────┐
                             │    React / TS Frontend    │
                             └─────────────┬─────────────┘
                                           │ (Wails IPC Bindings)
                             ┌─────────────▼─────────────┐
                             │      Go Wails Backend     │
                             └─────────────┬─────────────┘
                                           │ (gRPC client connections)
                                           ▼
                                    [ Server Node ]
```

### Frontend (User Interface)
*   **Framework**: React (Vite-based build setup).
*   **Styling**: Material UI (MUI) components & layout, Outfit/Inter typography.
*   **State Management**: Zustand stores (`useAuthStore`, `useWorkspaceStore`).
*   **Routing**: React Router.

### Backend (System Bindings & Security)
*   **Framework Core**: Wails (v2) exposing Go service adapters directly to JavaScript runtime.
*   **API Protocol**: gRPC (Go clients compiled from shared protobuf schema definitions).
*   **Local Identity Storage**: Argon2id KDF key derivation + AES-256-GCM encryption vault.
*   **OS Integrations**: Cross-platform system commands (via `os/exec`) to open files locally.

---

## 📂 Codebase Structure

*   **[`frontend/`](file:///home/snipeart007/repos/mandala-workspace/client/frontend)**: React application, source files, styles, components, and state stores.
*   **[`internal/services/`](file:///home/snipeart007/repos/mandala-workspace/client/internal/services)**: Wails binding implementations representing frontend-accessible adapters:
    *   [`auth.go`](file:///home/snipeart007/repos/mandala-workspace/client/internal/services/auth.go): Integrates challenge-response device signing.
    *   [`workspace.go`](file:///home/snipeart007/repos/mandala-workspace/client/internal/services/workspace.go): Exposes directory structures, creation, deletion, and movement.
*   **[`internal/grpcclient/`](file:///home/snipeart007/repos/mandala-workspace/client/internal/grpcclient)**:
    *   [`connection.go`](file:///home/snipeart007/repos/mandala-workspace/client/internal/grpcclient/connection.go): Manages connection state, TLS setup, and metadata-injecting interceptors.
*   **[`pkg/`](file:///home/snipeart007/repos/mandala-workspace/client/pkg)**: Shared utilities:
    *   `auth`: Secure local password-protected credentials keyring.
    *   `config`: System configurations (endpoints, directories).
    *   `logger`: Structured log configurations.
    *   `sysutils`: Native system launchers (`OpenFile` via `xdg-open`/`start`/`open`) and directory path resolvers.
*   **[`wails.json`](file:///home/snipeart007/repos/mandala-workspace/client/wails.json)**: Wails configuration file.

---

## 🔑 Crucial Features & Architecture Decisions

### 1. Zero IPC File Streaming
To prevent clogging the Wails IPC bridge, file content streams (uploads and downloads) are kept within the Go environment:
1.  **For Uploads**: The frontend opens a native file dialog, resolves the local absolute path, and passes *only* the string path to Go. The Go backend opens the file descriptor directly and streams chunks to the server via gRPC.
2.  **For Downloads**: The Go backend streams chunks directly from gRPC and writes them to a temporary system directory, returning only the local path to the frontend.

### 2. Password-Protected Keyring Security
The user’s device private key (Ed25519) and session tokens are encrypted at-rest:
*   Upon launch, the user inputs a master password.
*   The master password is ran through a high-cost **Argon2id** key derivation function.
*   The derived key encrypts the keyring store using **AES-256-GCM**.
*   The unlocked keys reside only in-memory and are zeroed out on session termination.

### 3. Native File Launch Flow
To support opening files seamlessly in their corresponding native applications:
1.  Double-clicking a file in the UI triggers a download call to the backend.
2.  The backend streams the file to a cached file path in `os.TempDir()`.
3.  The backend triggers `sysutils.OpenFile(path)`, which spawns the OS handler:
    *   **Windows**: `cmd /c start`
    *   **macOS**: `open`
    *   **Linux**: `xdg-open`

---

## 🚀 Running & Building

### Prerequisites
1.  Wails CLI installed:
    ```bash
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    ```
2.  Node.js & npm/yarn installed.

### Commands

From the `client/` subdirectory:

#### Live Development Mode
Runs a live hot-reloading desktop window, as well as a local web dev server on http://localhost:34115:
```bash
wails dev
```

#### Production Build
Builds a platform-native, optimized application bundle (e.g. `.exe` on Windows, `.app` on macOS, or binary executable on Linux):
```bash
wails build
```

---

## 📐 Development Guidelines

*   **Generate Bindings**: Every time you modify Go bindings in `app.go` or services, run `wails dev` or `wails generate module` to regenerate the TypeScript bindings in `frontend/src/api`.
*   **Strict Generated Code Scoping**: Do not manually import backend structures directly into React; always utilize the generated TypeScript bindings to keep components modular.
