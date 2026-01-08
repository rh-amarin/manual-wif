package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type PubSubTopicsResponse struct {
	Topics []struct {
		Name string `json:"name"`
	} `json:"topics"`
}

func main() {
	projectID := flag.String("project-id", "", "GCP project ID (required)")
	tokenPath := flag.String("token-input", "", "Path to the GCP access token file (required)")
	flag.Parse()

	if *projectID == "" || *tokenPath == "" {
		fmt.Println("Error: Missing required parameters")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./bin/list-topics --project-id <PROJECT_ID> --token-input <PATH>")
		fmt.Println()
		fmt.Println("Required parameters:")
		fmt.Println("  --project-id   GCP project ID (not project number)")
		fmt.Println("  --token-input  Path to the GCP access token file")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  ./bin/list-topics --project-id my-project --token-input gcp_access_token.txt")
		os.Exit(1)
	}

	fmt.Println("=== Step 4: Listing Pub/Sub Topics ===")
	fmt.Println("Using the access token to call GCP Pub/Sub API")
	fmt.Println()

	// Load the access token
	accessTokenBytes, err := os.ReadFile(*tokenPath)
	if err != nil {
		fmt.Printf("Error reading access token: %v\n", err)
		fmt.Println("Make sure to run exchange-token first!")
		os.Exit(1)
	}
	accessToken := strings.TrimSpace(string(accessTokenBytes))

	// Call Pub/Sub API to list topics
	topics, err := listPubSubTopics(*projectID, accessToken)
	if err != nil {
		fmt.Printf("Error listing topics: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Successfully called Pub/Sub API!")
	fmt.Println()

	if len(topics.Topics) == 0 {
		fmt.Println("No topics found in project.")
		fmt.Println("You can create a test topic with:")
		fmt.Printf("  gcloud pubsub topics create test-topic --project=%s\n", *projectID)
	} else {
		fmt.Printf("Found %d topic(s):\n", len(topics.Topics))
		for i, topic := range topics.Topics {
			fmt.Printf("  %d. %s\n", i+1, topic.Name)
		}
	}
	fmt.Println()
	fmt.Println("=== SUCCESS ===")
	fmt.Println("Workload Identity Federation is working correctly!")
	fmt.Println()
	fmt.Println("You've successfully:")
	fmt.Println("  1. Created a JWT token from an external identity")
	fmt.Println("  2. Exchanged it for a GCP federated token")
	fmt.Println("  3. Exchanged the federated token for an access token")
	fmt.Println("  4. Used the access token to call GCP APIs")
	fmt.Println()
	fmt.Println("The access token in gcp_access_token.txt can be used to call other GCP APIs.")
	fmt.Println("To start over, run: ./bin/generate-keys")
}

func listPubSubTopics(projectID, accessToken string) (*PubSubTopicsResponse, error) {
	url := fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics", projectID)

	fmt.Printf("Calling Pub/Sub API:\n")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Method: GET\n")
	fmt.Printf("  Authorization: Bearer <access_token>\n")
	fmt.Println()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
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
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var topicsResp PubSubTopicsResponse
	if err := json.Unmarshal(body, &topicsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &topicsResp, nil
}
