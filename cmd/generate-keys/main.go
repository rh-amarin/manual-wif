package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
)

func main() {
	privateKeyPath := flag.String("private-key", "", "Path to save the private key (required)")
	publicKeyPath := flag.String("public-key", "", "Path to save the public key (required)")
	flag.Parse()

	if *privateKeyPath == "" || *publicKeyPath == "" {
		fmt.Println("Error: --private-key and --public-key are required")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./bin/generate-keys --private-key <PATH> --public-key <PATH>")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ./bin/generate-keys --private-key private_key.pem --public-key public_key.pem")
		os.Exit(1)
	}

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

	privateKeyFile, err := os.Create(*privateKeyPath)
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

	publicKeyFile, err := os.Create(*publicKeyPath)
	if err != nil {
		fmt.Printf("Error creating public key file: %v\n", err)
		os.Exit(1)
	}
	defer publicKeyFile.Close()

	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		fmt.Printf("Error writing public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated %s (keep this secret!)\n", *privateKeyPath)
	fmt.Printf("✓ Generated %s (you'll upload this to GCP)\n", *publicKeyPath)
	fmt.Println()
	fmt.Println("=== Next Step ===")
	fmt.Println("Run the following command to generate JWK format:")
	fmt.Println()
	fmt.Println("  ./bin/generate-jwk --key-id <YOUR_KEY_ID>")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  ./bin/generate-jwk --key-id key-1")
}
