# Mandala Workspace

A Go-based microservices workspace with distributed architecture, client-server communication, and protocol buffers.

## Overview

Mandala Workspace is a distributed system project built with Go that implements a client-server architecture with protocol buffer definitions. It's organized as a monorepo using Go workspaces for managing multiple related services.

## Features

- **Go Workspaces** - Unified dependency management for multiple services
- **Microservices Architecture** - Separate client and server modules
- **Protocol Buffers** - Efficient data serialization
- **Makefile Build System** - Simple build and run commands
- **Comprehensive Documentation** - GEMINI.md for context and guidance

## Project Structure

```
mandala-workspace/
├── server/            # Server implementation
├── client/            # Client implementation
├── proto/             # Protocol buffer definitions
├── go.work           # Go workspace configuration
├── go.work.sum       # Workspace dependencies
├── Makefile          # Build commands
├── GEMINI.md         # Project documentation
├── LICENSE.md        # License information
└── .gitignore
```

## Prerequisites

- Go 1.18+ (required for Go workspaces)
- Protocol buffer compiler (protoc)

## Building

### Build all services:
```bash
make build
```

### Run the project:
```bash
make run
```

### Clean build artifacts:
```bash
make clean
```

## Architecture

### Server
- Main service implementation
- Protocol buffer service definitions

### Client
- Client application for server communication
- Uses protocol buffers for serialization

### Protocol Buffers
- Message definitions in `proto/` directory
- Enables efficient communication between services

## Development

### Generate Protocol Buffers

```bash
protoc --go_out=. --go-grpc_out=. proto/*.proto
```

### Format Code

```bash
go fmt ./...
```

### Lint

```bash
golangci-lint run ./...
```

## Documentation

Refer to `GEMINI.md` for detailed project documentation and architectural decisions.

## License

See LICENSE.md for license information.

## Author

Created by snipeart007
