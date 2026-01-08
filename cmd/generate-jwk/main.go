package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"os"
)

type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

func main() {
	keyID := flag.String("key-id", "", "Key ID for the JWK (required)")
	publicKeyPath := flag.String("public-key", "", "Path to the public key PEM file (required)")
	jwkPath := flag.String("jwk-output", "", "Path to save the JWK file (required)")
	jwksPath := flag.String("jwks-output", "", "Path to save the JWKS file (required)")
	flag.Parse()

	if *keyID == "" || *publicKeyPath == "" || *jwkPath == "" || *jwksPath == "" {
		fmt.Println("Error: --key-id, --public-key, --jwk-output, and --jwks-output are required")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./bin/generate-jwk --key-id <KEY_ID> --public-key <PATH> --jwk-output <PATH> --jwks-output <PATH>")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ./bin/generate-jwk --key-id key-1 --public-key public_key.pem --jwk-output public_key.jwk --jwks-output public_key.jwks")
		os.Exit(1)
	}

	fmt.Println("=== Step 1b: Converting Public Key to JWK Format ===")
	fmt.Println("GCP requires JWK format for JWT signature verification")
	fmt.Println()

	// Read the public key PEM file
	publicKeyPEM, err := os.ReadFile(*publicKeyPath)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", *publicKeyPath, err)
		fmt.Println("Run generate-keys first to generate the key pair")
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
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())

	jwk := JWK{
		Kty: "RSA",
		Use: "sig",
		Kid: *keyID,
		Alg: "RS256",
		N:   n,
		E:   e,
	}

	jwks := JWKS{
		Keys: []JWK{jwk},
	}

	// Write individual JWK file
	jwkJSON, err := json.MarshalIndent(jwk, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JWK: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*jwkPath, jwkJSON, 0644); err != nil {
		fmt.Printf("Error writing JWK file: %v\n", err)
		os.Exit(1)
	}

	// Write JWKS file
	jwksJSON, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JWKS: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*jwksPath, jwksJSON, 0644); err != nil {
		fmt.Printf("Error writing JWKS file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated %s (single JWK)\n", *jwkPath)
	fmt.Printf("✓ Generated %s (JWK Set - upload this to GCP)\n", *jwksPath)
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
	fmt.Println()
	fmt.Println("=== Next Step ===")
	fmt.Println("Run the following command to create a JWT token:")
	fmt.Println()
	fmt.Println("  ./bin/create-jwt --key-id <KEY_ID> --issuer <ISSUER_URL> --audience <AUDIENCE> --subject <SUBJECT> --email <EMAIL> --environment <ENV>")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Printf("  ./bin/create-jwt --key-id %s --issuer https://my-external-idp.example.com --audience gcp-workload-identity --subject external-user-123 --email user@example.com --environment production\n", *keyID)
}
