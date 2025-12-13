.PHONY: help clean step1 step1b step2 step3 step4 all run

help:
	@echo "GCP Workload Identity Federation POC"
	@echo ""
	@echo "Available targets:"
	@echo "  make step1    - Generate RSA key pair"
	@echo "  make step1b   - Convert public key to JWK format"
	@echo "  make step2    - Create and sign JWT token"
	@echo "  make step3    - Exchange JWT for GCP access token"
	@echo "  make step4    - List Pub/Sub topics"
	@echo "  make all      - Run all steps in sequence"
	@echo "  make run      - Same as 'make all'"
	@echo "  make clean    - Remove generated files"
	@echo ""
	@echo "Before running, ensure you have:"
	@echo "  1. Completed GCP setup (see GCP_SETUP.md)"
	@echo "  2. Created config.json from config.json.template"

step1:
	@echo "=== Running Step 1: Generate Keys ==="
	go run step1_generate_keys.go
	@echo ""
	@echo "=== Running Step 1b: Generate JWK ==="
	go run step1b_generate_jwk.go

step1b:
	@echo "=== Running Step 1b: Generate JWK ==="
	go run step1b_generate_jwk.go

step2:
	@echo "=== Running Step 2: Create JWT ==="
	go run step2_create_jwt.go

step3:
	@echo "=== Running Step 3: Exchange Token ==="
	go run step3_exchange_token.go

step4:
	@echo "=== Running Step 4: List Topics ==="
	go run step4_list_topics.go

all: step1 step2 step3 step4
	@echo ""
	@echo "✓ All steps completed successfully!"

run: all

clean:
	@echo "Removing generated files..."
	rm -f private_key.pem public_key.pem public_key.jwk public_key.jwks external_token.jwt gcp_access_token.txt
	@echo "✓ Cleaned"
