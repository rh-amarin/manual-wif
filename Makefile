.PHONY: all build clean test help

BINDIR := bin
CMDS := generate-keys generate-jwk create-jwt exchange-token list-topics

all: build

build:
	@echo "Building all commands..."
	@mkdir -p $(BINDIR)
	@for cmd in $(CMDS); do \
		echo "  Building $$cmd..."; \
		go build -o $(BINDIR)/$$cmd ./cmd/$$cmd; \
	done
	@echo "Done! Binaries are in $(BINDIR)/"

clean:
	@echo "Cleaning up..."
	@rm -rf $(BINDIR)
	@rm -f private_key.pem public_key.pem public_key.jwk public_key.jwks external_token.jwt gcp_access_token.txt
	@echo "Done!"

test:
	go test ./...

help:
	@echo "GCP Workload Identity Federation POC"
	@echo ""
	@echo "Available targets:"
	@echo "  make build  - Build all command binaries"
	@echo "  make clean  - Remove binaries and generated files"
	@echo "  make test   - Run tests"
	@echo "  make help   - Show this help message"
	@echo ""
	@echo "Commands (run in order):"
	@echo "  1. ./bin/generate-keys"
	@echo "  2. ./bin/generate-jwk --key-id <KEY_ID>"
	@echo "  3. ./bin/create-jwt --key-id <KEY_ID> --issuer <URL> --audience <AUD> --subject <SUB> [--email <EMAIL>] [--environment <ENV>]"
	@echo "  4. ./bin/exchange-token --project-number <NUM> --pool-id <POOL> --provider-id <PROVIDER> --service-account <SA_EMAIL>"
	@echo "  5. ./bin/list-topics --project-id <PROJECT_ID>"
