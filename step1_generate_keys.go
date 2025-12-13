package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Step 1: Generate RSA key pair for signing JWT tokens
// This simulates an external identity provider's signing key
func main() {
	fmt.Println("=== Step 1: Generating RSA Key Pair ===")
	fmt.Println("This key pair will be used to sign JWT tokens from our 'external' identity provider")
	fmt.Println()

	// Generate 2048-bit RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Error generating key: %v\n", err)
		os.Exit(1)
	}

	// Export private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	privateKeyFile, err := os.Create("private_key.pem")
	if err != nil {
		fmt.Printf("Error creating private key file: %v\n", err)
		os.Exit(1)
	}
	defer privateKeyFile.Close()

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		fmt.Printf("Error writing private key: %v\n", err)
		os.Exit(1)
	}

	// Export public key to PEM format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		fmt.Printf("Error marshaling public key: %v\n", err)
		os.Exit(1)
	}

	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	publicKeyFile, err := os.Create("public_key.pem")
	if err != nil {
		fmt.Printf("Error creating public key file: %v\n", err)
		os.Exit(1)
	}
	defer publicKeyFile.Close()

	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		fmt.Printf("Error writing public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Generated private_key.pem (keep this secret!)")
	fmt.Println("✓ Generated public_key.pem (you'll upload this to GCP)")
	fmt.Println()
	fmt.Println("Next step: Run step2_create_jwt.go to create a signed JWT token")
}
