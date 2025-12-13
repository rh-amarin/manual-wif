package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// Step 3: Exchange external JWT for GCP access token
// This demonstrates the token exchange flow using GCP STS API
func main() {
	fmt.Println("=== Step 3: Exchanging JWT for GCP Access Token ===")
	fmt.Println("This uses GCP's Security Token Service (STS) API")
	fmt.Println()

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		fmt.Println("Make sure to create config.json - see GCP_SETUP.md")
		os.Exit(1)
	}

	// Load the external JWT token
	externalToken, err := os.ReadFile("external_token.jwt")
	if err != nil {
		fmt.Printf("Error reading external token: %v\n", err)
		fmt.Println("Make sure to run step2_create_jwt.go first!")
		os.Exit(1)
	}

	fmt.Println("Step 3a: Exchange external JWT for federated token")
	fmt.Println("Calling GCP STS token endpoint...")
	fmt.Println()

	// Step 3a: Exchange external token for federated token
	federatedToken, err := exchangeForFederatedToken(string(externalToken), config)
	if err != nil {
		fmt.Printf("Error exchanging for federated token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Received federated token from GCP STS")
	fmt.Printf("  Token type: %s\n", federatedToken.TokenType)
	fmt.Printf("  Expires in: %d seconds\n", federatedToken.ExpiresIn)
	fmt.Println()

	fmt.Println("Step 3b: Exchange federated token for access token")
	fmt.Println("Calling GCP STS token endpoint again with service account impersonation...")
	fmt.Println()

	// Step 3b: Exchange federated token for access token with service account impersonation
	accessToken, err := exchangeForAccessToken(federatedToken.AccessToken, config)
	if err != nil {
		fmt.Printf("Error exchanging for access token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Received GCP access token")
	fmt.Printf("  Token type: %s\n", accessToken.TokenType)
	fmt.Printf("  Expires in: %d seconds\n", accessToken.ExpiresIn)
	fmt.Println()

	// Save the access token
	if err := os.WriteFile("gcp_access_token.txt", []byte(accessToken.AccessToken), 0600); err != nil {
		fmt.Printf("Error writing access token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Access token saved to: gcp_access_token.txt")
	fmt.Println()
	fmt.Println("Next step: Run step4_list_topics.go to use this token to list Pub/Sub topics")
}

type Config struct {
	ProjectID         string `json:"project_id"`
	ProjectNumber     string `json:"project_number"`
	PoolID            string `json:"pool_id"`
	ProviderID        string `json:"provider_id"`
	ServiceAccountEmail string `json:"service_account_email"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
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

func exchangeForFederatedToken(externalToken string, config *Config) (*TokenResponse, error) {
	// GCP STS endpoint
	stsURL := "https://sts.googleapis.com/v1/token"

	// Construct the audience - this identifies your workload identity pool and provider
	audience := fmt.Sprintf(
		"//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
		config.ProjectNumber,
		config.PoolID,
		config.ProviderID,
	)

	// Prepare the request body
	requestBody := map[string]string{
		"grant_type":          "urn:ietf:params:oauth:grant-type:token-exchange",
		"audience":            audience,
		"requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"subject_token_type":  "urn:ietf:params:oauth:token-type:jwt",
		"subject_token":       externalToken,
		"scope":               "https://www.googleapis.com/auth/cloud-platform",
	}

	fmt.Println("  Request details:")
	fmt.Printf("    Endpoint: %s\n", stsURL)
	fmt.Printf("    Audience: %s\n", audience)
	fmt.Printf("    Grant type: token-exchange\n")
	fmt.Printf("    Subject token type: JWT\n")
	fmt.Println()

	return callSTSEndpoint(stsURL, requestBody)
}

func exchangeForAccessToken(federatedToken string, config *Config) (*TokenResponse, error) {
	// Use Service Account Credentials API for impersonation
	url := fmt.Sprintf(
		"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
		config.ServiceAccountEmail,
	)

	// Prepare the request body
	requestBodyJSON := map[string]interface{}{
		"scope": []string{"https://www.googleapis.com/auth/cloud-platform"},
	}

	jsonData, err := json.Marshal(requestBodyJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Println("  Request details:")
	fmt.Printf("    Endpoint: %s\n", url)
	fmt.Printf("    Method: POST\n")
	fmt.Printf("    Service Account: %s\n", config.ServiceAccountEmail)
	fmt.Println()

	// Make the HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request creation failed: %w", err)
	}

	// Add authorization header with the federated token
	req.Header.Set("Authorization", "Bearer "+federatedToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IAM API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response (different format than STS)
	var saResp struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}
	if err := json.Unmarshal(body, &saResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to TokenResponse format
	return &TokenResponse{
		AccessToken: saResp.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600, // IAM doesn't return expires_in, typically 1 hour
	}, nil
}

func callSTSEndpoint(endpoint string, requestBody map[string]string) (*TokenResponse, error) {
	// Encode request body as form data
	formData := url.Values{}
	for key, value := range requestBody {
		formData.Set(key, value)
	}

	// Make the HTTP request
	resp, err := http.Post(
		endpoint,
		"application/x-www-form-urlencoded",
		bytes.NewBufferString(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("STS API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tokenResp, nil
}
