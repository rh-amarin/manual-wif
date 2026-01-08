package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	keyID := flag.String("key-id", "", "Key ID matching the JWK (required)")
	issuer := flag.String("issuer", "", "Issuer URL for the JWT (required)")
	audience := flag.String("audience", "", "Audience for the JWT (required)")
	subject := flag.String("subject", "", "Subject (user identifier) for the JWT (required)")
	email := flag.String("email", "", "User email address (optional)")
	environment := flag.String("environment", "", "Environment name (optional)")
	privateKeyPath := flag.String("private-key", "", "Path to the private key PEM file (required)")
	outputPath := flag.String("output", "", "Path to save the JWT token (required)")
	flag.Parse()

	if *keyID == "" || *issuer == "" || *audience == "" || *subject == "" || *privateKeyPath == "" || *outputPath == "" {
		fmt.Println("Error: Missing required parameters")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./bin/create-jwt --key-id <KEY_ID> --issuer <ISSUER_URL> --audience <AUDIENCE> --subject <SUBJECT> --private-key <PATH> --output <PATH> [--email <EMAIL>] [--environment <ENV>]")
		fmt.Println()
		fmt.Println("Required parameters:")
		fmt.Println("  --key-id       Key ID matching the JWK")
		fmt.Println("  --issuer       Issuer URL (e.g., https://my-external-idp.example.com)")
		fmt.Println("  --audience     JWT audience (must match WIF provider config)")
		fmt.Println("  --subject      Subject/user identifier")
		fmt.Println("  --private-key  Path to the private key PEM file")
		fmt.Println("  --output       Path to save the JWT token")
		fmt.Println()
		fmt.Println("Optional parameters:")
		fmt.Println("  --email        User email address")
		fmt.Println("  --environment  Environment name (e.g., production, staging)")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ./bin/create-jwt --key-id key-1 --issuer https://my-external-idp.example.com --audience gcp-workload-identity --subject external-user-123 --private-key private_key.pem --output external_token.jwt --email user@example.com --environment production")
		os.Exit(1)
	}

	fmt.Println("=== Step 2: Creating and Signing JWT Token ===")
	fmt.Println("This token represents an identity from the external provider")
	fmt.Println()

	// Load the private key
	privateKeyData, err := os.ReadFile(*privateKeyPath)
	if err != nil {
		fmt.Printf("Error reading private key: %v\n", err)
		fmt.Println("Make sure to run generate-keys first!")
		os.Exit(1)
	}

	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		fmt.Println("Failed to parse PEM block from private key")
		os.Exit(1)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		fmt.Printf("Error parsing private key: %v\n", err)
		os.Exit(1)
	}

	// Create JWT claims
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": *issuer,
		"sub": *subject,
		"aud": *audience,
		"iat": now.Unix(),
		"exp": now.Add(1 * time.Hour).Unix(),
	}

	// Add optional claims if provided
	if *email != "" {
		claims["email"] = *email
	}
	if *environment != "" {
		claims["environment"] = *environment
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = *keyID

	// Sign the token with the private key
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Printf("Error signing token: %v\n", err)
		os.Exit(1)
	}

	// Save the token to a file
	if err := os.WriteFile(*outputPath, []byte(tokenString), 0o644); err != nil {
		fmt.Printf("Error writing token file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Created and signed JWT token")
	fmt.Println()
	fmt.Println("Token claims:")
	claimsJSON, _ := json.MarshalIndent(claims, "  ", "  ")
	fmt.Printf("  %s\n", claimsJSON)
	fmt.Println()
	fmt.Printf("Token saved to: %s\n", *outputPath)
	fmt.Println()
	fmt.Println("Token preview (first 100 chars):")
	if len(tokenString) > 100 {
		fmt.Printf("  %s...\n", tokenString[:100])
	} else {
		fmt.Printf("  %s\n", tokenString)
	}
	fmt.Println()
	fmt.Println("=== Next Step ===")
	fmt.Println("Configure GCP Workload Identity Pool, then run:")
	fmt.Println()
	fmt.Println("  ./bin/exchange-token --project-number <PROJECT_NUMBER> --pool-id <POOL_ID> --provider-id <PROVIDER_ID> --service-account <SERVICE_ACCOUNT_EMAIL>")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  ./bin/exchange-token --project-number 123456789 --pool-id my-pool --provider-id my-provider --service-account my-sa@my-project.iam.gserviceaccount.com")
}
