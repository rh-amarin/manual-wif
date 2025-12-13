package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Step 4: Use the GCP access token to call Pub/Sub API
// This demonstrates using the federated identity to access GCP resources
func main() {
	fmt.Println("=== Step 4: Listing Pub/Sub Topics ===")
	fmt.Println("Using the access token to call GCP Pub/Sub API")
	fmt.Println()

	// Load the access token
	accessTokenBytes, err := os.ReadFile("gcp_access_token.txt")
	if err != nil {
		fmt.Printf("Error reading access token: %v\n", err)
		fmt.Println("Make sure to run step3_exchange_token.go first!")
		os.Exit(1)
	}
	accessToken := strings.TrimSpace(string(accessTokenBytes))

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Call Pub/Sub API to list topics
	topics, err := listPubSubTopics(config.ProjectID, accessToken)
	if err != nil {
		fmt.Printf("Error listing topics: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Successfully called Pub/Sub API!")
	fmt.Println()

	if len(topics.Topics) == 0 {
		fmt.Println("No topics found in project.")
		fmt.Println("You can create a test topic with:")
		fmt.Printf("  gcloud pubsub topics create test-topic --project=%s\n", config.ProjectID)
	} else {
		fmt.Printf("Found %d topic(s):\n", len(topics.Topics))
		for i, topic := range topics.Topics {
			fmt.Printf("  %d. %s\n", i+1, topic.Name)
		}
	}
	fmt.Println()
	fmt.Println("SUCCESS! Workload Identity Federation is working correctly.")
	fmt.Println("You've successfully:")
	fmt.Println("  1. Created a JWT token from an external identity")
	fmt.Println("  2. Exchanged it for a GCP federated token")
	fmt.Println("  3. Exchanged the federated token for an access token")
	fmt.Println("  4. Used the access token to call GCP APIs")
}

type Config struct {
	ProjectID string `json:"project_id"`
}

type PubSubTopicsResponse struct {
	Topics []struct {
		Name string `json:"name"`
	} `json:"topics"`
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

func listPubSubTopics(projectID, accessToken string) (*PubSubTopicsResponse, error) {
	// Construct the Pub/Sub API URL
	url := fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics", projectID)

	fmt.Printf("Calling Pub/Sub API:\n")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Method: GET\n")
	fmt.Printf("  Authorization: Bearer <access_token>\n")
	fmt.Println()

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var topicsResp PubSubTopicsResponse
	if err := json.Unmarshal(body, &topicsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &topicsResp, nil
}
