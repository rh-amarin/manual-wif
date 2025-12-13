package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config represents the configuration loaded from config.json
type Config struct {
	IssuerURL   string `json:"issuer_url"`
	JWTAudience string `json:"jwt_audience"`
	KeyID       string `json:"key_id"`
	Subject     string `json:"subject"`
	UserEmail   string `json:"user_email"`
	Environment string `json:"environment"`
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Step 2: Create and sign a JWT token
// This simulates the external identity provider issuing a token
func main() {
	fmt.Println("=== Step 2: Creating and Signing JWT Token ===")
	fmt.Println("This token represents an identity from the external provider")
	fmt.Println()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		fmt.Println("Make sure to create config.json - see GCP_SETUP.md")
		os.Exit(1)
	}

	// Load the private key
	privateKeyData, err := os.ReadFile("private_key.pem")
	if err != nil {
		fmt.Printf("Error reading private key: %v\n", err)
		fmt.Println("Make sure to run step1_generate_keys.go first!")
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
	// These claims identify who is requesting access
	now := time.Now()
	claims := jwt.MapClaims{
		// Standard claims
		"iss": config.IssuerURL,      // Issuer - identifies your external IdP
		"sub": config.Subject,        // Subject - the user/identity
		"aud": config.JWTAudience,    // Audience - must match the WIF provider configuration
		"iat": now.Unix(),            // Issued at
		"exp": now.Add(1 * time.Hour).Unix(), // Expiration (1 hour)

		// Custom claims (optional, can be used in attribute mappings)
		"email":       config.UserEmail,
		"environment": config.Environment,
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Set the key ID (kid) to match the JWK
	token.Header["kid"] = config.KeyID

	// Sign the token with the private key
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Printf("Error signing token: %v\n", err)
		os.Exit(1)
	}

	// Save the token to a file
	if err := os.WriteFile("external_token.jwt", []byte(tokenString), 0o644); err != nil {
		fmt.Printf("Error writing token file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Created and signed JWT token")
	fmt.Println()
	fmt.Println("Token claims:")
	claimsJSON, _ := json.MarshalIndent(claims, "  ", "  ")
	fmt.Printf("  %s\n", claimsJSON)
	fmt.Println()
	fmt.Println("Token saved to: external_token.jwt")
	fmt.Println()
	fmt.Println("Token preview (first 100 chars):")
	if len(tokenString) > 100 {
		fmt.Printf("  %s...\n", tokenString[:100])
	} else {
		fmt.Printf("  %s\n", tokenString)
	}
	fmt.Println()
	fmt.Println("Next step: Configure GCP Workload Identity Pool, then run step3_exchange_token.go")
}
