.PHONY: server-test server-run server-proto client-test all test setup

all: server-proto server-test client-test

server-proto:
	$(MAKE) -C server proto

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
	@echo "Root setup complete"
