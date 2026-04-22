.PHONY: proto generate test run clean

# Generate Go code from proto files
proto:
	@echo "Generating Go code from proto files..."
	protoc -I.. --go_out=. --go-grpc_out=. ../proto/mandala/v1/*.proto
	@echo "Proto code generated successfully"

# Install protoc plugins if not present
install-proto-tools:
	@echo "Installing protoc tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Download dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test ./...

# Run the server
run: proto
	go run cmd/server/main.go

# Build the binary
build: proto
	go build -o bin/mandala-server cmd/server/main.go

# Clean generated files
clean:
	find ./gen -type f -name '*.pb.go' -delete
	rm -f bin/mandala-server

# Setup development environment
setup: install-proto-tools deps proto
	@echo "Development environment ready"

# Development watcher (requires fswatch or similar)
watch:
	@echo "Watching for changes..."
	@while true; do \
		fswatch -r proto/ | xargs -n1 bash -c 'clear && make proto' || sleep 1; \
	done
