package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
)

// JWK represents a JSON Web Key for RSA
type JWK struct {
	Kty string `json:"kty"` // Key Type
	Use string `json:"use"` // Public Key Use
	Kid string `json:"kid"` // Key ID
	Alg string `json:"alg"` // Algorithm
	N   string `json:"n"`   // Modulus
	E   string `json:"e"`   // Exponent
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// Config represents the configuration loaded from config.json
type Config struct {
	KeyID string `json:"key_id"`
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

// Step 1b: Convert PEM public key to JWK format for GCP
func main() {
	fmt.Println("=== Step 1b: Converting Public Key to JWK Format ===")
	fmt.Println("GCP requires JWK format for JWT signature verification")
	fmt.Println()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		fmt.Println("Make sure to create config.json - see GCP_SETUP.md")
		os.Exit(1)
	}

	// Read the public key PEM file
	publicKeyPEM, err := os.ReadFile("public_key.pem")
	if err != nil {
		fmt.Printf("Error reading public_key.pem: %v\n", err)
		fmt.Println("Run step1_generate_keys.go first to generate the key pair")
		os.Exit(1)
	}

	// Decode PEM block
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		fmt.Println("Error: Failed to decode PEM block")
		os.Exit(1)
	}

	// Parse the public key
	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		fmt.Printf("Error parsing public key: %v\n", err)
		os.Exit(1)
	}

	// Assert it's an RSA public key
	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		fmt.Println("Error: Key is not an RSA public key")
		os.Exit(1)
	}

	// Convert to JWK format
	// The modulus (N) and exponent (E) need to be base64url encoded
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())

	jwk := JWK{
		Kty: "RSA",
		Use: "sig",            // Signature use
		Kid: config.KeyID,     // Key ID - loaded from config.json
		Alg: "RS256",          // Algorithm
		N:   n,                // Modulus
		E:   e,                // Exponent
	}

	// Create JWKS (JSON Web Key Set) - GCP expects this format
	jwks := JWKS{
		Keys: []JWK{jwk},
	}

	// Write individual JWK file
	jwkJSON, err := json.MarshalIndent(jwk, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JWK: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("public_key.jwk", jwkJSON, 0644); err != nil {
		fmt.Printf("Error writing JWK file: %v\n", err)
		os.Exit(1)
	}

	// Write JWKS file (this is what GCP typically expects)
	jwksJSON, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JWKS: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("public_key.jwks", jwksJSON, 0644); err != nil {
		fmt.Printf("Error writing JWKS file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Generated public_key.jwk (single JWK)")
	fmt.Println("✓ Generated public_key.jwks (JWK Set - upload this to GCP)")
	fmt.Println()
	fmt.Println("JWK content:")
	fmt.Println(string(jwkJSON))
	fmt.Println()
	fmt.Println("JWKS content:")
	fmt.Println(string(jwksJSON))
	fmt.Println()
	fmt.Println("To use with GCP Workload Identity Federation:")
	fmt.Println("1. Host the public_key.jwks file at a publicly accessible HTTPS URL")
	fmt.Println("2. Configure the provider with --jwk-json-path=<URL to your JWKS>")
	fmt.Println("   OR")
	fmt.Println("   Use the gcloud command to upload directly (see GCP_SETUP.md)")
}
