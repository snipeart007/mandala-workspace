.PHONY: server-test server-run server-proto client-test client-proto all test setup

all: server-proto client-proto server-test client-test

server-proto:
	$(MAKE) -C server proto

client-proto:
	@echo "Generating Go code for client from proto files..."
	cd client && mkdir -p gen && protoc -I.. --go_out=. --go-grpc_out=. ../proto/mandala/v1/*.proto

server-test:
	$(MAKE) -C server test

server-run:
	$(MAKE) -C server run

client-test:
	# Add client tests when available
	@echo "No client tests yet"

test: server-test client-test

setup:
	$(MAKE) -C server setup
	$(MAKE) client-proto
	@echo "Root setup complete"
