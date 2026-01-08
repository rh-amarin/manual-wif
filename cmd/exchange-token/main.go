package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func main() {
	projectNumber := flag.String("project-number", "", "GCP project number (required)")
	poolID := flag.String("pool-id", "", "Workload Identity Pool ID (required)")
	providerID := flag.String("provider-id", "", "Workload Identity Provider ID (required)")
	serviceAccount := flag.String("service-account", "", "Service account email to impersonate (required)")
	tokenPath := flag.String("token-input", "", "Path to the external JWT token file (required)")
	outputPath := flag.String("output", "", "Path to save the GCP access token (required)")
	flag.Parse()

	if *projectNumber == "" || *poolID == "" || *providerID == "" || *serviceAccount == "" || *tokenPath == "" || *outputPath == "" {
		fmt.Println("Error: Missing required parameters")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./bin/exchange-token --project-number <PROJECT_NUMBER> --pool-id <POOL_ID> --provider-id <PROVIDER_ID> --service-account <SERVICE_ACCOUNT_EMAIL> --token-input <PATH> --output <PATH>")
		fmt.Println()
		fmt.Println("Required parameters:")
		fmt.Println("  --project-number   GCP project number (not project ID)")
		fmt.Println("  --pool-id          Workload Identity Pool ID")
		fmt.Println("  --provider-id      Workload Identity Provider ID")
		fmt.Println("  --service-account  Service account email to impersonate")
		fmt.Println("  --token-input      Path to the external JWT token file")
		fmt.Println("  --output           Path to save the GCP access token")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ./bin/exchange-token --project-number 123456789 --pool-id my-pool --provider-id my-provider --service-account my-sa@my-project.iam.gserviceaccount.com --token-input external_token.jwt --output gcp_access_token.txt")
		os.Exit(1)
	}

	fmt.Println("=== Step 3: Exchanging JWT for GCP Access Token ===")
	fmt.Println("This uses GCP's Security Token Service (STS) API")
	fmt.Println()

	// Load the external JWT token
	externalToken, err := os.ReadFile(*tokenPath)
	if err != nil {
		fmt.Printf("Error reading external token: %v\n", err)
		fmt.Println("Make sure to run create-jwt first!")
		os.Exit(1)
	}

	fmt.Println("Step 3a: Exchange external JWT for federated token")
	fmt.Println("Calling GCP STS token endpoint...")
	fmt.Println()

	// Step 3a: Exchange external token for federated token
	federatedToken, err := exchangeForFederatedToken(string(externalToken), *projectNumber, *poolID, *providerID)
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
	accessToken, err := exchangeForAccessToken(federatedToken.AccessToken, *serviceAccount)
	if err != nil {
		fmt.Printf("Error exchanging for access token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Received GCP access token")
	fmt.Printf("  Token type: %s\n", accessToken.TokenType)
	fmt.Printf("  Expires in: %d seconds\n", accessToken.ExpiresIn)
	fmt.Println()

	// Save the access token
	if err := os.WriteFile(*outputPath, []byte(accessToken.AccessToken), 0600); err != nil {
		fmt.Printf("Error writing access token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Access token saved to: %s\n", *outputPath)
	fmt.Println()
	fmt.Println("=== Next Step ===")
	fmt.Println("Use the access token to call GCP APIs:")
	fmt.Println()
	fmt.Println("  ./bin/list-topics --project-id <PROJECT_ID>")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  ./bin/list-topics --project-id my-project")
}

func exchangeForFederatedToken(externalToken, projectNumber, poolID, providerID string) (*TokenResponse, error) {
	stsURL := "https://sts.googleapis.com/v1/token"

	audience := fmt.Sprintf(
		"//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
		projectNumber,
		poolID,
		providerID,
	)

	requestBody := map[string]string{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"audience":             audience,
		"requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"subject_token_type":   "urn:ietf:params:oauth:token-type:jwt",
		"subject_token":        externalToken,
		"scope":                "https://www.googleapis.com/auth/cloud-platform",
	}

	fmt.Println("  Request details:")
	fmt.Printf("    Endpoint: %s\n", stsURL)
	fmt.Printf("    Audience: %s\n", audience)
	fmt.Printf("    Grant type: token-exchange\n")
	fmt.Printf("    Subject token type: JWT\n")
	fmt.Println()

	return callSTSEndpoint(stsURL, requestBody)
}

func exchangeForAccessToken(federatedToken, serviceAccountEmail string) (*TokenResponse, error) {
	url := fmt.Sprintf(
		"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
		serviceAccountEmail,
	)

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
	fmt.Printf("    Service Account: %s\n", serviceAccountEmail)
	fmt.Println()

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request creation failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+federatedToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IAM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var saResp struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}
	if err := json.Unmarshal(body, &saResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &TokenResponse{
		AccessToken: saResp.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}, nil
}

func callSTSEndpoint(endpoint string, requestBody map[string]string) (*TokenResponse, error) {
	formData := url.Values{}
	for key, value := range requestBody {
		formData.Set(key, value)
	}

	resp, err := http.Post(
		endpoint,
		"application/x-www-form-urlencoded",
		bytes.NewBufferString(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("STS API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tokenResp, nil
}
